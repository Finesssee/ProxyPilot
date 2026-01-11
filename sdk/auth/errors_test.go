package auth

import "testing"

func TestEmailRequiredError_Error(t *testing.T) {
	tests := []struct {
		name    string
		err     *EmailRequiredError
		wantMsg string
	}{
		{
			name:    "nil error",
			err:     nil,
			wantMsg: "cliproxy auth: email is required",
		},
		{
			name:    "empty prompt",
			err:     &EmailRequiredError{Prompt: ""},
			wantMsg: "cliproxy auth: email is required",
		},
		{
			name:    "custom prompt",
			err:     &EmailRequiredError{Prompt: "Please provide your email address"},
			wantMsg: "Please provide your email address",
		},
		{
			name:    "prompt with whitespace",
			err:     &EmailRequiredError{Prompt: "   "},
			wantMsg: "   ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.wantMsg {
				t.Errorf("Error() = %q, want %q", got, tt.wantMsg)
			}
		})
	}
}

func TestEmailRequiredError_ImplementsError(t *testing.T) {
	var _ error = (*EmailRequiredError)(nil)
	var _ error = &EmailRequiredError{Prompt: "test"}
}
