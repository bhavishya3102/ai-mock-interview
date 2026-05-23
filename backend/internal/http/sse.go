package http

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// sseWriter is the minimal SSE encoder used by streaming endpoints. Headers
// are written lazily on the first event so handlers can fall back to a
// regular JSON error response if pre-stream validation fails (status code
// can only be set once, before the first byte goes out).
//
// Each call to WriteEvent emits `event: <name>\ndata: <json>\n\n` and
// flushes immediately so proxies and the browser surface the chunk without
// buffering. WriteEvent returns the underlying write error so the caller
// can detect client disconnects and abort early.
//
// Flush is performed through http.NewResponseController so middleware
// wrappers (statusRecorder, recoverer) that don't themselves implement
// http.Flusher are unwrapped automatically — a direct `w.(http.Flusher)`
// assertion would fail against the chi+statusRecorder middleware chain.
type sseWriter struct {
	w           http.ResponseWriter
	rc          *http.ResponseController
	headersSent bool
}

func newSSEWriter(w http.ResponseWriter) *sseWriter {
	return &sseWriter{w: w, rc: http.NewResponseController(w)}
}

// writeHeaders sets the SSE response headers and writes the 200 status.
// Idempotent — subsequent calls are no-ops so WriteEvent can call it
// safely from every event without bookkeeping at the call site.
func (s *sseWriter) writeHeaders() {
	if s.headersSent {
		return
	}
	h := s.w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	// Nginx-specific hint: do not buffer this response. Harmless on other
	// proxies and on the local dev setup; necessary in production when a
	// reverse proxy sits in front of the server.
	h.Set("X-Accel-Buffering", "no")
	s.w.WriteHeader(http.StatusOK)
	s.headersSent = true
}

// WriteEvent serialises data as JSON and emits an SSE event under the
// given name. The headers are written lazily on the first call.
func (s *sseWriter) WriteEvent(event string, data any) error {
	s.writeHeaders()

	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("sse marshal: %w", err)
	}
	if _, err := fmt.Fprintf(s.w, "event: %s\ndata: %s\n\n", event, payload); err != nil {
		return fmt.Errorf("sse write: %w", err)
	}
	// Flush failure is rare in production but possible if the underlying
	// writer doesn't support it (e.g. an exotic test recorder). Surface so
	// the handler can stop generating chunks rather than buffer them
	// indefinitely.
	if err := s.rc.Flush(); err != nil {
		return fmt.Errorf("sse flush: %w", err)
	}
	return nil
}

// HeadersSent reports whether any SSE bytes have been written. Streaming
// handlers consult this to decide whether a mid-flow error should be sent
// as an SSE `event: error` (true) or as a normal JSON error envelope on a
// non-200 status (false).
func (s *sseWriter) HeadersSent() bool { return s.headersSent }
