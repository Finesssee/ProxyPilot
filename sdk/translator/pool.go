package translator

import (
	"sync"
	"sync/atomic"
)

// BufferPool manages reusable byte slices to reduce GC pressure.
type BufferPool struct {
	pools     []*sync.Pool
	sizes     []int
	allocated atomic.Int64
	reused    atomic.Int64
}

// PoolStats contains statistics about buffer pool usage.
type PoolStats struct {
	Allocated int64 `json:"allocated"`
	Reused    int64 `json:"reused"`
	ReuseRate float64 `json:"reuse_rate"`
}

// predefined bucket sizes for the pool (powers of 2)
var defaultBucketSizes = []int{
	64,     // 64 bytes
	256,    // 256 bytes
	1024,   // 1 KB
	4096,   // 4 KB
	16384,  // 16 KB
	65536,  // 64 KB
	262144, // 256 KB
	1048576, // 1 MB
}

// NewBufferPool creates a new buffer pool with default bucket sizes.
func NewBufferPool() *BufferPool {
	return NewBufferPoolWithSizes(defaultBucketSizes)
}

// NewBufferPoolWithSizes creates a buffer pool with custom bucket sizes.
func NewBufferPoolWithSizes(sizes []int) *BufferPool {
	bp := &BufferPool{
		pools: make([]*sync.Pool, len(sizes)),
		sizes: make([]int, len(sizes)),
	}
	copy(bp.sizes, sizes)

	for i, size := range sizes {
		s := size // capture for closure
		bp.pools[i] = &sync.Pool{
			New: func() any {
				return make([]byte, 0, s)
			},
		}
	}

	return bp
}

// GetBuffer retrieves a byte slice from the pool with at least the requested capacity.
func (bp *BufferPool) GetBuffer(size int) []byte {
	idx := bp.findBucket(size)
	if idx >= 0 {
		buf := bp.pools[idx].Get().([]byte)
		bp.reused.Add(1)
		return buf[:0] // Reset length to 0 while keeping capacity
	}
	// Size larger than any bucket, allocate directly
	bp.allocated.Add(1)
	return make([]byte, 0, size)
}

// PutBuffer returns a byte slice to the pool for reuse.
func (bp *BufferPool) PutBuffer(buf []byte) {
	if buf == nil {
		return
	}
	cap := cap(buf)
	idx := bp.findBucketExact(cap)
	if idx >= 0 {
		// Clear buffer content for security/safety
		for i := range buf {
			buf[i] = 0
		}
		bp.pools[idx].Put(buf[:0])
	}
	// If cap doesn't match any bucket, let GC handle it
}

// findBucket returns the index of the smallest bucket that can fit size.
func (bp *BufferPool) findBucket(size int) int {
	for i, s := range bp.sizes {
		if s >= size {
			return i
		}
	}
	return -1
}

// findBucketExact returns the index of the bucket with exact capacity match.
func (bp *BufferPool) findBucketExact(cap int) int {
	for i, s := range bp.sizes {
		if s == cap {
			return i
		}
	}
	return -1
}

// GetStats returns usage statistics for the pool.
func (bp *BufferPool) GetStats() PoolStats {
	allocated := bp.allocated.Load()
	reused := bp.reused.Load()
	total := allocated + reused
	var rate float64
	if total > 0 {
		rate = float64(reused) / float64(total)
	}
	return PoolStats{
		Allocated: allocated,
		Reused:    reused,
		ReuseRate: rate,
	}
}

// ResetStats resets the pool statistics.
func (bp *BufferPool) ResetStats() {
	bp.allocated.Store(0)
	bp.reused.Store(0)
}

// defaultPool is the package-level buffer pool.
var defaultPool = NewBufferPool()

// DefaultPool returns the package-level buffer pool.
func DefaultPool() *BufferPool {
	return defaultPool
}

// GetBuffer retrieves a buffer from the default pool.
func GetBuffer(size int) []byte {
	return defaultPool.GetBuffer(size)
}

// PutBuffer returns a buffer to the default pool.
func PutBuffer(buf []byte) {
	defaultPool.PutBuffer(buf)
}

// GetPoolStats returns statistics for the default pool.
func GetPoolStats() PoolStats {
	return defaultPool.GetStats()
}

// ResetPoolStats resets statistics for the default pool.
func ResetPoolStats() {
	defaultPool.ResetStats()
}

// BufferWriter is a helper that wraps buffer pool operations for writing.
type BufferWriter struct {
	pool *BufferPool
	buf  []byte
}

// NewBufferWriter creates a new BufferWriter with initial capacity.
func NewBufferWriter(pool *BufferPool, initialSize int) *BufferWriter {
	if pool == nil {
		pool = defaultPool
	}
	return &BufferWriter{
		pool: pool,
		buf:  pool.GetBuffer(initialSize),
	}
}

// Write appends data to the buffer, growing if necessary.
func (bw *BufferWriter) Write(p []byte) (n int, err error) {
	bw.grow(len(p))
	n = copy(bw.buf[len(bw.buf):cap(bw.buf)], p)
	bw.buf = bw.buf[:len(bw.buf)+n]
	return n, nil
}

// WriteByte appends a single byte.
func (bw *BufferWriter) WriteByte(c byte) error {
	bw.grow(1)
	bw.buf = append(bw.buf, c)
	return nil
}

// WriteString appends a string.
func (bw *BufferWriter) WriteString(s string) (n int, err error) {
	bw.grow(len(s))
	n = copy(bw.buf[len(bw.buf):cap(bw.buf)], s)
	bw.buf = bw.buf[:len(bw.buf)+n]
	return n, nil
}

// grow ensures there's enough capacity for n more bytes.
func (bw *BufferWriter) grow(n int) {
	if cap(bw.buf)-len(bw.buf) >= n {
		return
	}
	// Need to grow
	newSize := (cap(bw.buf) + n) * 2
	newBuf := bw.pool.GetBuffer(newSize)
	newBuf = append(newBuf, bw.buf...)
	bw.pool.PutBuffer(bw.buf)
	bw.buf = newBuf
}

// Bytes returns the accumulated bytes.
func (bw *BufferWriter) Bytes() []byte {
	return bw.buf
}

// String returns the accumulated bytes as a string.
func (bw *BufferWriter) String() string {
	return string(bw.buf)
}

// Len returns the current length.
func (bw *BufferWriter) Len() int {
	return len(bw.buf)
}

// Reset clears the buffer without releasing it back to the pool.
func (bw *BufferWriter) Reset() {
	bw.buf = bw.buf[:0]
}

// Release returns the buffer to the pool. The writer should not be used after this.
func (bw *BufferWriter) Release() {
	if bw.buf != nil {
		bw.pool.PutBuffer(bw.buf)
		bw.buf = nil
	}
}

// Copy returns a copy of the current buffer contents.
func (bw *BufferWriter) Copy() []byte {
	if bw.buf == nil {
		return nil
	}
	cp := make([]byte, len(bw.buf))
	copy(cp, bw.buf)
	return cp
}
