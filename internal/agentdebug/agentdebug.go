package agentdebug

import (
	"encoding/json"
	"os"
	"runtime"
	"sync/atomic"
	"time"
)

const defaultWindowsLogPath = `d:\code\ProxyPilot\.cursor\debug.log`

var seq uint64

type entry struct {
	ID           string         `json:"id,omitempty"`
	Timestamp    int64          `json:"timestamp"`
	Location     string         `json:"location"`
	Message      string         `json:"message"`
	Data         map[string]any `json:"data,omitempty"`
	SessionID    string         `json:"sessionId"`
	RunID        string         `json:"runId"`
	HypothesisID string         `json:"hypothesisId"`
}

func Log(hypothesisID, location, message string, data map[string]any) {
	logPath := os.Getenv("PROXYPILOT_AGENT_DEBUG_LOG_PATH")
	if logPath == "" {
		if runtime.GOOS != "windows" {
			// Debug session is Windows-scoped; avoid hard-failing on other OS.
			return
		}
		logPath = defaultWindowsLogPath
	}

	runID := os.Getenv("PROXYPILOT_AGENT_DEBUG_RUN_ID")
	if runID == "" {
		runID = "pre-fix"
	}

	now := time.Now()
	id := atomic.AddUint64(&seq, 1)

	b, err := json.Marshal(entry{
		ID:           "log_" + now.Format("20060102T150405.000") + "_" + fmtUint(id),
		Timestamp:    now.UnixMilli(),
		Location:     location,
		Message:      message,
		Data:         data,
		SessionID:    "debug-session",
		RunID:        runID,
		HypothesisID: hypothesisID,
	})
	if err != nil {
		return
	}

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	_, _ = f.Write(append(b, '\n'))
	_ = f.Close()
}

func fmtUint(v uint64) string {
	// tiny, allocation-light formatting; no need for strconv import in hot paths.
	const digits = "0123456789"
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = digits[v%10]
		v /= 10
	}
	return string(buf[i:])
}



