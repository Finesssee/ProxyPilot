package translator

import (
	"bytes"
	"testing"
)

func TestBufferPool_BasicOperations(t *testing.T) {
	pool := NewBufferPool()

	// Get a buffer
	buf := pool.GetBuffer(100)
	if cap(buf) < 100 {
		t.Errorf("buffer capacity should be at least 100, got %d", cap(buf))
	}
	if len(buf) != 0 {
		t.Errorf("buffer length should be 0, got %d", len(buf))
	}

	// Use the buffer
	buf = append(buf, []byte("hello world")...)

	// Return it
	pool.PutBuffer(buf)

	stats := pool.GetStats()
	if stats.Reused != 1 {
		t.Errorf("expected 1 reused, got %d", stats.Reused)
	}
}

func TestBufferPool_BucketSelection(t *testing.T) {
	pool := NewBufferPool()

	// Request exact bucket size
	buf64 := pool.GetBuffer(64)
	if cap(buf64) != 64 {
		t.Errorf("expected capacity 64, got %d", cap(buf64))
	}

	// Request size between buckets
	buf100 := pool.GetBuffer(100)
	if cap(buf100) != 256 { // Should get 256 bucket
		t.Errorf("expected capacity 256, got %d", cap(buf100))
	}

	// Request very large size
	bufLarge := pool.GetBuffer(2000000)
	if cap(bufLarge) < 2000000 {
		t.Errorf("expected capacity >= 2000000, got %d", cap(bufLarge))
	}

	pool.PutBuffer(buf64)
	pool.PutBuffer(buf100)
	pool.PutBuffer(bufLarge) // This won't be pooled (too large)
}

func TestBufferPool_Reuse(t *testing.T) {
	pool := NewBufferPool()

	// Get and return a buffer
	buf1 := pool.GetBuffer(64)
	buf1 = append(buf1, []byte("test data")...)
	pool.PutBuffer(buf1)

	// Get another buffer of same size - should reuse
	buf2 := pool.GetBuffer(64)
	if cap(buf2) != 64 {
		t.Errorf("expected capacity 64, got %d", cap(buf2))
	}
	if len(buf2) != 0 {
		t.Errorf("reused buffer should have length 0, got %d", len(buf2))
	}

	stats := pool.GetStats()
	if stats.Reused < 2 {
		t.Errorf("expected at least 2 reuses, got %d", stats.Reused)
	}
}

func TestBufferPool_ResetStats(t *testing.T) {
	pool := NewBufferPool()

	// Generate some stats
	for i := 0; i < 5; i++ {
		buf := pool.GetBuffer(100)
		pool.PutBuffer(buf)
	}

	stats := pool.GetStats()
	if stats.Reused == 0 {
		t.Error("expected some reuses")
	}

	pool.ResetStats()
	stats = pool.GetStats()
	if stats.Reused != 0 || stats.Allocated != 0 {
		t.Error("expected stats to be reset")
	}
}

func TestBufferWriter_BasicOperations(t *testing.T) {
	pool := NewBufferPool()
	writer := NewBufferWriter(pool, 64)
	defer writer.Release()

	// Write data
	n, err := writer.Write([]byte("hello "))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if n != 6 {
		t.Errorf("expected 6 bytes written, got %d", n)
	}

	// Write string
	n, err = writer.WriteString("world")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if n != 5 {
		t.Errorf("expected 5 bytes written, got %d", n)
	}

	// Write byte
	err = writer.WriteByte('!')
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if writer.String() != "hello world!" {
		t.Errorf("unexpected content: %s", writer.String())
	}
	if writer.Len() != 12 {
		t.Errorf("expected length 12, got %d", writer.Len())
	}
}

func TestBufferWriter_Growth(t *testing.T) {
	pool := NewBufferPool()
	writer := NewBufferWriter(pool, 16)
	defer writer.Release()

	// Write more than initial capacity
	data := bytes.Repeat([]byte("x"), 100)
	writer.Write(data)

	if writer.Len() != 100 {
		t.Errorf("expected length 100, got %d", writer.Len())
	}
	if !bytes.Equal(writer.Bytes(), data) {
		t.Error("data mismatch after growth")
	}
}

func TestBufferWriter_Reset(t *testing.T) {
	pool := NewBufferPool()
	writer := NewBufferWriter(pool, 64)
	defer writer.Release()

	writer.WriteString("hello world")
	if writer.Len() != 11 {
		t.Errorf("expected length 11, got %d", writer.Len())
	}

	writer.Reset()
	if writer.Len() != 0 {
		t.Errorf("expected length 0 after reset, got %d", writer.Len())
	}

	// Should still be usable
	writer.WriteString("new content")
	if writer.String() != "new content" {
		t.Errorf("unexpected content: %s", writer.String())
	}
}

func TestBufferWriter_Copy(t *testing.T) {
	pool := NewBufferPool()
	writer := NewBufferWriter(pool, 64)
	defer writer.Release()

	writer.WriteString("original")
	cp := writer.Copy()

	// Modify original
	writer.WriteString(" modified")

	// Copy should be unchanged
	if string(cp) != "original" {
		t.Errorf("copy should be 'original', got '%s'", string(cp))
	}
}

func TestDefaultPoolFunctions(t *testing.T) {
	// Test package-level functions
	buf := GetBuffer(100)
	if cap(buf) < 100 {
		t.Errorf("buffer capacity should be at least 100, got %d", cap(buf))
	}

	PutBuffer(buf)

	stats := GetPoolStats()
	if stats.Reused < 1 {
		t.Errorf("expected at least 1 reuse, got %d", stats.Reused)
	}

	ResetPoolStats()
	stats = GetPoolStats()
	if stats.Reused != 0 {
		t.Error("expected stats reset")
	}
}

func TestBufferPoolWithSizes_CustomBuckets(t *testing.T) {
	customSizes := []int{32, 128, 512}
	pool := NewBufferPoolWithSizes(customSizes)

	buf32 := pool.GetBuffer(32)
	if cap(buf32) != 32 {
		t.Errorf("expected capacity 32, got %d", cap(buf32))
	}

	buf100 := pool.GetBuffer(100)
	if cap(buf100) != 128 { // Should get 128 bucket
		t.Errorf("expected capacity 128, got %d", cap(buf100))
	}

	buf200 := pool.GetBuffer(200)
	if cap(buf200) != 512 { // Should get 512 bucket
		t.Errorf("expected capacity 512, got %d", cap(buf200))
	}

	pool.PutBuffer(buf32)
	pool.PutBuffer(buf100)
	pool.PutBuffer(buf200)
}
