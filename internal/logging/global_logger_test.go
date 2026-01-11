package logging

import (
	"testing"

	log "github.com/sirupsen/logrus"
)

func TestSetLogLevel(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected log.Level
	}{
		// Debug level
		{"debug lowercase", "debug", log.DebugLevel},
		{"debug uppercase", "DEBUG", log.DebugLevel},
		{"debug mixed case", "Debug", log.DebugLevel},
		{"verbose lowercase", "verbose", log.DebugLevel},
		{"verbose uppercase", "VERBOSE", log.DebugLevel},
		{"verbose mixed case", "Verbose", log.DebugLevel},

		// Info level
		{"info lowercase", "info", log.InfoLevel},
		{"info uppercase", "INFO", log.InfoLevel},
		{"info mixed case", "Info", log.InfoLevel},

		// Warn level
		{"warn lowercase", "warn", log.WarnLevel},
		{"warn uppercase", "WARN", log.WarnLevel},
		{"warning lowercase", "warning", log.WarnLevel},
		{"warning uppercase", "WARNING", log.WarnLevel},
		{"warning mixed case", "Warning", log.WarnLevel},

		// Error level
		{"error lowercase", "error", log.ErrorLevel},
		{"error uppercase", "ERROR", log.ErrorLevel},
		{"error mixed case", "Error", log.ErrorLevel},

		// Fatal level (quiet/silent)
		{"quiet lowercase", "quiet", log.FatalLevel},
		{"quiet uppercase", "QUIET", log.FatalLevel},
		{"silent lowercase", "silent", log.FatalLevel},
		{"silent uppercase", "SILENT", log.FatalLevel},

		// Default (unknown) -> InfoLevel
		{"unknown string", "unknown", log.InfoLevel},
		{"empty string", "", log.InfoLevel},
		{"random string", "foobar", log.InfoLevel},
		{"numeric string", "123", log.InfoLevel},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset to a known state before each test
			log.SetLevel(log.PanicLevel)

			SetLogLevel(tt.input)

			got := log.GetLevel()
			if got != tt.expected {
				t.Errorf("SetLogLevel(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}
