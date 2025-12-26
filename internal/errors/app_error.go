package errors

import (
	"encoding/json"
	"fmt"
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

