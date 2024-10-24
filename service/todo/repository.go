package todo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nathansiegfrid/todolist-go/service"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db}
}

func (r *Repository) GetAll(ctx context.Context, filter *TodoFilter) ([]*Todo, error) {
	// Translate filter into WHERE conditions and args.
	where, args, argIndex := []string{"TRUE"}, []any{}, 1
	if v := filter.ID; v.Defined {
		where = append(where, fmt.Sprintf("id = $%d", argIndex))
		args = append(args, v.Value)
		argIndex++
	}
	if v := filter.UserID; v.Defined {
		where = append(where, fmt.Sprintf("user_id = $%d", argIndex))
		args = append(args, v.Value)
		argIndex++
	}
	if v := filter.Priority; v.Defined {
		where = append(where, fmt.Sprintf("priority = $%d", argIndex))
		args = append(args, v.Value)
		argIndex++
	}
	if v := filter.DueDate; v.Defined {
		if v.Value == nil {
			where = append(where, "due_date IS NULL")
		} else {
			where = append(where, fmt.Sprintf("due_date::date = $%d::date", argIndex))
			args = append(args, v.Value)
			argIndex++
		}
	}
	if v := filter.Completed; v.Defined {
		where = append(where, fmt.Sprintf("completed = $%d", argIndex))
		args = append(args, v.Value)
		argIndex++
	}

	var limit, offset string
	if filter.Limit > 0 {
		limit = fmt.Sprintf(" LIMIT %d ", filter.Limit)
	}
	if filter.Offset > 0 {
		offset = fmt.Sprintf(" OFFSET %d ", filter.Offset)
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, user_id, subject, description, priority, due_date, completed, created_at, updated_at
		FROM todo
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY description ASC`+
		limit+offset,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var todos []*Todo
	for rows.Next() {
		todo := &Todo{}
		err := rows.Scan(
			&todo.ID,
			&todo.UserID,
			&todo.Subject,
			&todo.Description,
			&todo.Priority,
			&todo.DueDate,
			&todo.Completed,
			&todo.CreatedAt,
			&todo.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		todos = append(todos, todo)
	}
	return todos, nil
}

func (r *Repository) Get(ctx context.Context, id uuid.UUID) (*Todo, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, subject, description, priority, due_date, completed, created_at, updated_at
		FROM todo
		WHERE id = $1`,
		id,
	)

	todo := &Todo{}
	err := row.Scan(
		&todo.ID,
		&todo.UserID,
		&todo.Subject,
		&todo.Description,
		&todo.Priority,
		&todo.DueDate,
		&todo.Completed,
		&todo.CreatedAt,
		&todo.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrNotFound(id)
		}
		return nil, err
	}
	return todo, nil
}

func (r *Repository) Create(ctx context.Context, todo *Todo) error {
	todo.ID = uuid.New()
	todo.UserID = service.UserIDFromContext(ctx)
	todo.CreatedAt = time.Now()
	todo.UpdatedAt = todo.CreatedAt

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO todo (id, user_id, subject, description, priority, due_date, completed, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		todo.ID,
		todo.UserID,
		todo.Subject,
		todo.Description,
		todo.Priority,
		todo.DueDate,
		todo.Completed,
		todo.CreatedAt,
		todo.UpdatedAt,
	)
	return err
}

func (r *Repository) Update(ctx context.Context, id uuid.UUID, update *TodoUpdate) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	err = updateTodo(ctx, tx, id, update)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	err = deleteTodo(ctx, tx, id)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func getTodoForUpdate(ctx context.Context, tx *sql.Tx, id uuid.UUID) (*Todo, error) {
	// FOR UPDATE will lock selected row, which prevents new writes and locks to the same row
	// before current Tx is done.
	row := tx.QueryRowContext(ctx, `
		SELECT id, user_id, subject, description, priority, due_date, completed, created_at, updated_at
		FROM todo
		WHERE id = $1
		FOR UPDATE`,
		id,
	)

	todo := &Todo{}
	err := row.Scan(
		&todo.ID,
		&todo.UserID,
		&todo.Subject,
		&todo.Description,
		&todo.Priority,
		&todo.DueDate,
		&todo.Completed,
		&todo.CreatedAt,
		&todo.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrNotFound(id)
		}
		return nil, err
	}
	return todo, nil
}

func updateTodo(ctx context.Context, tx *sql.Tx, id uuid.UUID, update *TodoUpdate) error {
	todo, err := getTodoForUpdate(ctx, tx, id)
	if err != nil {
		return err
	}

	// Check if resource is owned by user.
	if todo.UserID != service.UserIDFromContext(ctx) {
		return service.ErrPermission()
	}

	var updated bool
	if v := update.Subject; v.Defined {
		todo.Subject = v.Value
		updated = true
	}
	if v := update.Description; v.Defined {
		todo.Description = v.Value
		updated = true
	}
	if v := update.Priority; v.Defined {
		todo.Priority = v.Value
		updated = true
	}
	if v := update.DueDate; v.Defined {
		todo.DueDate = v.Value
		updated = true
	}
	if v := update.Completed; v.Defined {
		todo.Completed = v.Value
		updated = true
	}
	if !updated {
		return nil
	}
	todo.UpdatedAt = time.Now()

	result, err := tx.ExecContext(ctx, `
		UPDATE todo
		SET subject = $1, description = $2, priority = $3, due_date = $4, completed = $5, updated_at = $6
		WHERE id = $7`,
		todo.Subject,
		todo.Description,
		todo.Priority,
		todo.DueDate,
		todo.Completed,
		todo.UpdatedAt,
		id,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return service.ErrNotFound(id)
	}
	return nil
}

func deleteTodo(ctx context.Context, tx *sql.Tx, id uuid.UUID) error {
	todo, err := getTodoForUpdate(ctx, tx, id)
	if err != nil {
		return err
	}

	// Check if resource is owned by user.
	if todo.UserID != service.UserIDFromContext(ctx) {
		return service.ErrPermission()
	}

	result, err := tx.ExecContext(ctx, "DELETE FROM todo WHERE id = $1", id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return service.ErrNotFound(id)
	}
	return nil
}
