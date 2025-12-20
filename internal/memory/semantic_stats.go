package memory

import "sync/atomic"

// SemanticQueueStats captures background embedding queue stats.
type SemanticQueueStats struct {
	Queued    uint64 `json:"queued"`
	Dropped   uint64 `json:"dropped"`
	Processed uint64 `json:"processed"`
	Failed    uint64 `json:"failed"`
}

var (
	semanticQueued    uint64
	semanticDropped   uint64
	semanticProcessed uint64
	semanticFailed    uint64
)

func IncSemanticQueued(n int) {
	if n <= 0 {
		return
	}
	atomic.AddUint64(&semanticQueued, uint64(n))
}

func IncSemanticDropped(n int) {
	if n <= 0 {
		return
	}
	atomic.AddUint64(&semanticDropped, uint64(n))
}

func IncSemanticProcessed(n int) {
	if n <= 0 {
		return
	}
	atomic.AddUint64(&semanticProcessed, uint64(n))
}

func IncSemanticFailed(n int) {
	if n <= 0 {
		return
	}
	atomic.AddUint64(&semanticFailed, uint64(n))
}

func GetSemanticQueueStats() SemanticQueueStats {
	return SemanticQueueStats{
		Queued:    atomic.LoadUint64(&semanticQueued),
		Dropped:   atomic.LoadUint64(&semanticDropped),
		Processed: atomic.LoadUint64(&semanticProcessed),
		Failed:    atomic.LoadUint64(&semanticFailed),
	}
}
