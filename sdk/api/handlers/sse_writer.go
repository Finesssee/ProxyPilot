package handlers

import (
	"bytes"
	"io"
	"sync"
)

var sseBufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

var (
	sseDataPrefix    = []byte("data: ")
	sseErrorPrefix   = []byte("event: error\ndata: ")
	sseErrorPrefixNL = []byte("\nevent: error\ndata: ")
	sseSuffix        = []byte("\n\n")
	sseDone          = []byte("data: [DONE]\n\n")
)

// WriteSSEData writes a standard SSE "data" frame.
func WriteSSEData(w io.Writer, data []byte) {
	if w == nil || len(data) == 0 {
		return
	}
	buf := sseBufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	buf.Grow(len(sseDataPrefix) + len(data) + len(sseSuffix))
	_, _ = buf.Write(sseDataPrefix)
	_, _ = buf.Write(data)
	_, _ = buf.Write(sseSuffix)
	_, _ = w.Write(buf.Bytes())
	buf.Reset()
	sseBufferPool.Put(buf)
}

// WriteSSEError writes an SSE error event. If leadingNewline is true, a newline is prefixed.
func WriteSSEError(w io.Writer, data []byte, leadingNewline bool) {
	if w == nil || len(data) == 0 {
		return
	}
	prefix := sseErrorPrefix
	if leadingNewline {
		prefix = sseErrorPrefixNL
	}
	buf := sseBufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	buf.Grow(len(prefix) + len(data) + len(sseSuffix))
	_, _ = buf.Write(prefix)
	_, _ = buf.Write(data)
	_, _ = buf.Write(sseSuffix)
	_, _ = w.Write(buf.Bytes())
	buf.Reset()
	sseBufferPool.Put(buf)
}

// WriteSSEDone writes the standard SSE done marker.
func WriteSSEDone(w io.Writer) {
	if w == nil {
		return
	}
	_, _ = w.Write(sseDone)
}
