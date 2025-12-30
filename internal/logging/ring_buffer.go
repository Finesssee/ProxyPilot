package logging

import (
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

// DefaultBufferSize is the default capacity of the ring buffer.
const DefaultBufferSize = 1000

// LogEntry represents a single log entry stored in the ring buffer.
// This struct matches what the TUI expects for displaying logs.
type LogEntry struct {
	Timestamp time.Time
	Level     string
	Message   string
	Source    string                 // Source file:line if available
	Fields    map[string]interface{} // Additional structured fields
}

// RingBuffer is a thread-safe circular buffer for storing log entries.
// It implements logrus.Hook to capture logs from the logging system.
type RingBuffer struct {
	mu       sync.RWMutex
	entries  []LogEntry
	capacity int
	head     int  // Index where the next entry will be written
	count    int  // Number of entries currently in the buffer
	full     bool // Whether the buffer has wrapped around
}

// NewRingBuffer creates a new ring buffer with the specified capacity.
// If capacity is 0 or negative, DefaultBufferSize is used.
func NewRingBuffer(capacity int) *RingBuffer {
	if capacity <= 0 {
		capacity = DefaultBufferSize
	}
	return &RingBuffer{
		entries:  make([]LogEntry, capacity),
		capacity: capacity,
		head:     0,
		count:    0,
		full:     false,
	}
}

// Levels returns the log levels that this hook should be fired for.
// We capture all levels to provide complete log visibility in the TUI.
func (rb *RingBuffer) Levels() []log.Level {
	return log.AllLevels
}

// Fire is called by logrus when a log entry is made.
// This method implements the logrus.Hook interface.
func (rb *RingBuffer) Fire(entry *log.Entry) error {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	// Build source string from caller info if available
	source := ""
	if entry.Caller != nil {
		source = entry.Caller.File + ":" + string(rune(entry.Caller.Line+'0'))
		// Proper formatting for line number
		source = formatSource(entry.Caller.File, entry.Caller.Line)
	}

	// Normalize level string
	level := entry.Level.String()
	if level == "warning" {
		level = "warn"
	}

	// Copy fields to avoid race conditions
	fields := make(map[string]interface{}, len(entry.Data))
	for k, v := range entry.Data {
		fields[k] = v
	}

	logEntry := LogEntry{
		Timestamp: entry.Time,
		Level:     level,
		Message:   entry.Message,
		Source:    source,
		Fields:    fields,
	}

	// Write to the current position and advance head
	rb.entries[rb.head] = logEntry
	rb.head = (rb.head + 1) % rb.capacity

	if rb.count < rb.capacity {
		rb.count++
	} else {
		rb.full = true
	}

	return nil
}

// formatSource formats caller file and line into a source string.
func formatSource(file string, line int) string {
	// Extract just the filename, not the full path
	short := file
	for i := len(file) - 1; i > 0; i-- {
		if file[i] == '/' || file[i] == '\\' {
			short = file[i+1:]
			break
		}
	}
	// Format line number properly
	return short + ":" + itoa(line)
}

// itoa converts an integer to a string without importing strconv.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	pos := len(b)
	neg := i < 0
	if neg {
		i = -i
	}
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		b[pos] = '-'
	}
	return string(b[pos:])
}

// Write writes a log entry directly to the buffer.
// This can be used as an alternative to the Hook interface.
func (rb *RingBuffer) Write(entry LogEntry) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.entries[rb.head] = entry
	rb.head = (rb.head + 1) % rb.capacity

	if rb.count < rb.capacity {
		rb.count++
	} else {
		rb.full = true
	}
}

// GetEntries returns a COPY of all entries in the buffer, oldest first.
// This method is safe to call concurrently and the returned slice
// can be freely modified by the caller without causing data races.
func (rb *RingBuffer) GetEntries() []LogEntry {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if rb.count == 0 {
		return []LogEntry{}
	}

	// Create a new slice with the exact size needed
	result := make([]LogEntry, rb.count)

	if rb.full {
		// Buffer has wrapped; oldest entry is at head position
		// Copy from head to end of array
		copied := copy(result, rb.entries[rb.head:])
		// Copy from start of array to head
		copy(result[copied:], rb.entries[:rb.head])
	} else {
		// Buffer hasn't wrapped; entries are from 0 to head-1
		copy(result, rb.entries[:rb.count])
	}

	// Deep copy the Fields maps to prevent data races
	for i := range result {
		if result[i].Fields != nil {
			fieldsCopy := make(map[string]interface{}, len(result[i].Fields))
			for k, v := range result[i].Fields {
				fieldsCopy[k] = v
			}
			result[i].Fields = fieldsCopy
		}
	}

	return result
}

// GetRecentEntries returns a COPY of the N most recent entries, oldest first.
// If n is greater than the number of entries, all entries are returned.
func (rb *RingBuffer) GetRecentEntries(n int) []LogEntry {
	entries := rb.GetEntries()
	if n <= 0 || n >= len(entries) {
		return entries
	}
	return entries[len(entries)-n:]
}

// Len returns the current number of entries in the buffer.
func (rb *RingBuffer) Len() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.count
}

// Cap returns the capacity of the buffer.
func (rb *RingBuffer) Cap() int {
	return rb.capacity
}

// Clear removes all entries from the buffer.
func (rb *RingBuffer) Clear() {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.head = 0
	rb.count = 0
	rb.full = false
	// Zero out the entries slice for garbage collection
	for i := range rb.entries {
		rb.entries[i] = LogEntry{}
	}
}

// GlobalBuffer is the global ring buffer instance used to capture all logs.
// It is automatically initialized with DefaultBufferSize capacity.
var GlobalBuffer = NewRingBuffer(DefaultBufferSize)

// GetGlobalEntries returns a copy of all log entries from the global buffer.
// This is a convenience function for use by the TUI.
func GetGlobalEntries() []LogEntry {
	return GlobalBuffer.GetEntries()
}

// GetRecentGlobalEntries returns a copy of the N most recent log entries.
// This is a convenience function for use by the TUI.
func GetRecentGlobalEntries(n int) []LogEntry {
	return GlobalBuffer.GetRecentEntries(n)
}
