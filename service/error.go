package service

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/google/uuid"
	"github.com/gorilla/schema"
)

type APIError struct {
	StatusCode int
	Message    string
	Data       any
}

// Error implements the `error` interface.
func (e *APIError) Error() string {
	return fmt.Sprintf("API error %d: %s", e.StatusCode, e.Message)
}

func Error(statusCode int, message string) error {
	return &APIError{StatusCode: statusCode, Message: message}
}

func Errorf(statusCode int, format string, args ...any) error {
	return &APIError{StatusCode: statusCode, Message: fmt.Sprintf(format, args...)}
}

func ErrorStatusCode(err error) int {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode
	}
	return http.StatusInternalServerError
}

// ErrInvalidID is used when the request param is not a valid ID.
func ErrInvalidID(id string) error {
	return Errorf(http.StatusBadRequest, "Invalid ID '%s'.", id)
}

// ErrInvalidURLQuery is used when `gorilla/schema` failed to parse the URL query.
func ErrInvalidURLQuery(errs schema.MultiError) error {
	keys := make([]string, 0, len(errs))
	for k := range errs {
		keys = append(keys, k)
	}
	return Errorf(http.StatusBadRequest, "Invalid URL query: %s.", strings.Join(keys, ", "))
}

// ErrInvalidJSON is used when JSON decoder failed to parse the request body.
func ErrInvalidJSON() error {
	return Error(http.StatusBadRequest, "Invalid JSON request body.")
}

// ErrValidation is used when validation by `ozzo-validation` returns an error.
// Error value from `ozzo-validation` can be marshaled into key-value JSON object.
func ErrValidation(errs validation.Errors) error {
	return &APIError{
		StatusCode: http.StatusBadRequest,
		Message:    "Invalid input value.",
		Data:       errs,
	}
}

// ErrPermission is used when the user does not have enough authorization to do the request.
func ErrPermission() error {
	return Error(http.StatusForbidden, "Permission denied.")
}

func ErrNotFound(id uuid.UUID) error {
	return Errorf(http.StatusNotFound, "ID '%s' not found.", id)
}

func ErrConflict(key string, value string) error {
	return Errorf(http.StatusConflict, "%s '%s' already exists.", key, value)
}
