package http

import (
	"errors"
	"net/http"

	"github.com/bhavishya3102/ai-mock-interview/backend/internal/domain"
)

// httpError pairs a status code with a low-cardinality user message + code.
// Variable detail (mock IDs, user IDs, paths) belong in the slog attributes
// at the log site, NOT in the message — APM tools group on this string.
type httpError struct {
	Status  int
	Code    string
	Message string
}

// mapErrToHTTP translates a domain error into the public HTTP shape. Caller
// is responsible for logging the technical err once.
func mapErrToHTTP(err error) httpError {
	switch {
	case err == nil:
		return httpError{Status: http.StatusOK, Code: "ok", Message: ""}
	case errors.Is(err, domain.ErrValidation):
		return httpError{Status: http.StatusBadRequest, Code: "validation_failed", Message: "request validation failed"}
	case errors.Is(err, domain.ErrUnauthorized):
		return httpError{Status: http.StatusUnauthorized, Code: "unauthorized", Message: "unauthorized"}
	case errors.Is(err, domain.ErrNotFound):
		return httpError{Status: http.StatusNotFound, Code: "not_found", Message: "resource not found"}
	case errors.Is(err, domain.ErrConflict):
		return httpError{Status: http.StatusConflict, Code: "conflict", Message: "resource already exists"}
	case errors.Is(err, domain.ErrLLM):
		return httpError{Status: http.StatusBadGateway, Code: "llm_failure", Message: "upstream model error"}
	case errors.Is(err, domain.ErrNotImplemented):
		return httpError{Status: http.StatusNotImplemented, Code: "not_implemented", Message: "feature not implemented"}
	default:
		return httpError{Status: http.StatusInternalServerError, Code: "internal_error", Message: "internal error"}
	}
}
