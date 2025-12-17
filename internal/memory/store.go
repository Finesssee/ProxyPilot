package memory

import "time"

type Event struct {
	TS   time.Time         `json:"ts"`
	Kind string            `json:"kind"`
	Role string            `json:"role,omitempty"`
	Type string            `json:"type,omitempty"`
	Text string            `json:"text,omitempty"`
	Meta map[string]string `json:"meta,omitempty"`
}

type Store interface {
	Append(session string, events []Event) error
	Search(session string, query string, maxChars int, maxSnippets int) ([]string, error)
}
