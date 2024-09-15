# Load ".env" if file exists.
ifneq (,$(wildcard ./.env))
    include .env
    export
endif

# GO COMMANDS
run:
	go run cmd/todolist/main.go
update:
	go get -u -t ./...
	go mod tidy

# DB MIGRATION COMMANDS
DB_STRING=postgres://$(PG_USER):$(PG_PASSWORD)@$(PG_HOST):$(PG_PORT)/$(PG_DATABASE)?sslmode=$(PG_SSL_MODE)
db-up:
	goose -dir migration postgres "$(DB_STRING)" up
db-down:
	goose -dir migration postgres "$(DB_STRING)" down
db-reset:
	goose -dir migration postgres "$(DB_STRING)" reset