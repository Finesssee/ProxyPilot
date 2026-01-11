// Package cmd provides CLI command implementations for ProxyPilot.
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/logging"
)

// DefaultLogLines is the default number of log lines to show
const DefaultLogLines = 50

// LogEntryOutput represents a log entry for JSON output
type LogEntryOutput struct {
	Timestamp string                 `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Source    string                 `json:"source,omitempty"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

// LogsOutput represents the JSON output structure for logs
type LogsOutput struct {
	Count   int              `json:"count"`
	Entries []LogEntryOutput `json:"entries"`
}

// ShowLogs displays recent proxy log entries
func ShowLogs(n int, jsonOutput bool) error {
	if n <= 0 {
		n = DefaultLogLines
	}

	entries := logging.GetRecentGlobalEntries(n)

	if jsonOutput {
		return outputLogsJSON(entries)
	}

	return outputLogsTable(entries)
}

func outputLogsJSON(entries []logging.LogEntry) error {
	output := LogsOutput{
		Count:   len(entries),
		Entries: make([]LogEntryOutput, len(entries)),
	}

	for i, entry := range entries {
		output.Entries[i] = LogEntryOutput{
			Timestamp: entry.Timestamp.Format("2006-01-02T15:04:05.000Z07:00"),
			Level:     entry.Level,
			Message:   entry.Message,
			Source:    entry.Source,
			Fields:    entry.Fields,
		}
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}

func outputLogsTable(entries []logging.LogEntry) error {
	if len(entries) == 0 {
		fmt.Printf("%sNo log entries available%s\n", colorYellow, colorReset)
		fmt.Printf("%sLogs are captured during proxy operation.%s\n", colorDim, colorReset)
		return nil
	}

	fmt.Printf("\n%s%sProxyPilot Logs%s (%d entries)\n", colorBold, colorCyan, colorReset, len(entries))
	fmt.Printf("%s───────────────────────────────────────────────────────────────────────────────%s\n\n", colorDim, colorReset)

	for _, entry := range entries {
		timestamp := entry.Timestamp.Format("15:04:05")
		level := formatLogLevel(entry.Level)
		message := entry.Message

		// Truncate very long messages
		if len(message) > 100 {
			message = message[:97] + "..."
		}

		// Remove trailing newlines
		message = strings.TrimRight(message, "\r\n")

		fmt.Printf("%s%s%s %s %s\n", colorDim, timestamp, colorReset, level, message)
	}

	fmt.Printf("\n%s───────────────────────────────────────────────────────────────────────────────%s\n", colorDim, colorReset)

	return nil
}

func formatLogLevel(level string) string {
	switch strings.ToLower(level) {
	case "debug":
		return colorDim + "[DEBUG]" + colorReset
	case "info":
		return colorBlue + "[INFO] " + colorReset
	case "warn", "warning":
		return colorYellow + "[WARN] " + colorReset
	case "error":
		return colorRed + "[ERROR]" + colorReset
	case "fatal":
		return colorRed + colorBold + "[FATAL]" + colorReset
	default:
		return "[" + strings.ToUpper(level) + "]"
	}
}
