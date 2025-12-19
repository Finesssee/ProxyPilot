package errors

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// AppError represents a structured application error.
type AppError struct {
	// HTTPStatusCode is the HTTP status code to return.
	HTTPStatusCode int `json:"-"`
	// Code is an internal error code string.
	Code string `json:"code"`
	// Message is the user-facing error message.
	Message string `json:"message"`
	// Details provides additional error context (optional).
	Details map[string]interface{} `json:"details,omitempty"`
	// Err is the underlying error (not marshaled to JSON).
	Err error `json:"-"`
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

// Unwrap returns the underlying error.
func (e *AppError) Unwrap() error {
	return e.Err
}

// ToJSON returns the JSON byte representation of the error.
func (e *AppError) ToJSON() []byte {
	b, _ := json.Marshal(e)
	return b
}

// New creates a new AppError.
func New(statusCode int, code, message string, err error) *AppError {
	return &AppError{
		HTTPStatusCode: statusCode,
		Code:           code,
		Message:        message,
		Err:            err,
	}
}

// Common error constructors

func BadRequest(message string, err error) *AppError {
	return New(http.StatusBadRequest, "bad_request", message, err)
}

func Unauthorized(message string, err error) *AppError {
	return New(http.StatusUnauthorized, "unauthorized", message, err)
}

func Forbidden(message string, err error) *AppError {
	return New(http.StatusForbidden, "forbidden", message, err)
}

func NotFound(message string, err error) *AppError {
	return New(http.StatusNotFound, "not_found", message, err)
}

func InternalServerError(message string, err error) *AppError {
	return New(http.StatusInternalServerError, "internal_error", message, err)
}
