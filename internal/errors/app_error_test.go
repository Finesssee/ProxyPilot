package errors

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestAppError_Error(t *testing.T) {
	tests := []struct {
		name    string
		appErr  *AppError
		wantMsg string
	}{
		{
			name: "message only",
			appErr: &AppError{
				Message: "something went wrong",
			},
			wantMsg: "something went wrong",
		},
		{
			name: "message with wrapped error",
			appErr: &AppError{
				Message: "request failed",
				Err:     errors.New("connection refused"),
			},
			wantMsg: "request failed: connection refused",
		},
		{
			name: "empty message with error",
			appErr: &AppError{
				Message: "",
				Err:     errors.New("underlying"),
			},
			wantMsg: ": underlying",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.appErr.Error()
			if got != tt.wantMsg {
				t.Errorf("Error() = %q, want %q", got, tt.wantMsg)
			}
		})
	}
}

func TestAppError_Unwrap(t *testing.T) {
	underlying := errors.New("root cause")
	appErr := &AppError{
		Message: "wrapper",
		Err:     underlying,
	}

	if got := appErr.Unwrap(); got != underlying {
		t.Errorf("Unwrap() = %v, want %v", got, underlying)
	}

	// nil wrapped error
	appErrNil := &AppError{Message: "no wrap"}
	if got := appErrNil.Unwrap(); got != nil {
		t.Errorf("Unwrap() on nil Err = %v, want nil", got)
	}
}

func TestAppError_ToJSON(t *testing.T) {
	appErr := &AppError{
		HTTPStatusCode: 400,
		Code:           "INVALID_REQUEST",
		Message:        "bad input",
		Details:        map[string]interface{}{"field": "email"},
	}

	b := appErr.ToJSON()

	var parsed map[string]interface{}
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatalf("ToJSON() produced invalid JSON: %v", err)
	}

	if parsed["code"] != "INVALID_REQUEST" {
		t.Errorf("code = %v, want INVALID_REQUEST", parsed["code"])
	}
	if parsed["message"] != "bad input" {
		t.Errorf("message = %v, want bad input", parsed["message"])
	}
	// HTTPStatusCode should not be in JSON
	if _, exists := parsed["http_status_code"]; exists {
		t.Error("HTTPStatusCode should not be in JSON output")
	}
	// Details should be present
	details, ok := parsed["details"].(map[string]interface{})
	if !ok {
		t.Fatal("details should be a map")
	}
	if details["field"] != "email" {
		t.Errorf("details.field = %v, want email", details["field"])
	}
}

func TestAppError_ToJSON_OmitsEmptyDetails(t *testing.T) {
	appErr := &AppError{
		Code:    "ERROR",
		Message: "msg",
	}

	b := appErr.ToJSON()

	var parsed map[string]interface{}
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatalf("ToJSON() produced invalid JSON: %v", err)
	}

	if _, exists := parsed["details"]; exists {
		t.Error("details should be omitted when empty")
	}
}

func TestNew(t *testing.T) {
	underlying := errors.New("cause")
	appErr := New(500, "INTERNAL", "server error", underlying)

	if appErr.HTTPStatusCode != 500 {
		t.Errorf("HTTPStatusCode = %d, want 500", appErr.HTTPStatusCode)
	}
	if appErr.Code != "INTERNAL" {
		t.Errorf("Code = %s, want INTERNAL", appErr.Code)
	}
	if appErr.Message != "server error" {
		t.Errorf("Message = %s, want server error", appErr.Message)
	}
	if appErr.Err != underlying {
		t.Errorf("Err = %v, want %v", appErr.Err, underlying)
	}
}

func TestNew_NilError(t *testing.T) {
	appErr := New(404, "NOT_FOUND", "resource missing", nil)

	if appErr.Err != nil {
		t.Errorf("Err = %v, want nil", appErr.Err)
	}
	if appErr.Error() != "resource missing" {
		t.Errorf("Error() = %s, want resource missing", appErr.Error())
	}
}
