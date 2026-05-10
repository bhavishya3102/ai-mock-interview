// Package domain holds pure types and sentinel errors shared across layers.
// It must not import http, store, or llm — those layers depend on domain.
package domain

import "errors"

// Sentinel errors. Wrap with fmt.Errorf("...: %w", err) and inspect with
// errors.Is at the HTTP boundary to map to status codes. Messages are
// low-cardinality so APM tools (Datadog, Sentry) group them properly —
// variable data goes into structured log attributes, never into the message.
var (
	ErrNotFound       = errors.New("resource not found")
	ErrUnauthorized   = errors.New("unauthorized")
	ErrValidation     = errors.New("validation failed")
	ErrConflict       = errors.New("conflict")
	ErrLLM            = errors.New("llm failure")
	ErrNotImplemented = errors.New("not implemented")
)
