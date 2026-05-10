// Package http hosts the transport layer: chi router, handlers, error
// mapping, and request middleware. Handlers depend on the service via a
// consumer-defined interface so tests can swap a fake.
package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/bhavisachdeva/ai-mock-interview-v2/backend/internal/domain"
	"github.com/go-playground/validator/v10"
)

// validate is shared across handlers — validator caches struct reflection,
// so creating one per request would defeat its purpose.
var validate = validator.New()

// errorBody is the JSON envelope shape: {"error": {"code": "...", "message": "..."}}.
type errorBody struct {
	Error errorPayload `json:"error"`
}

type errorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, log *slog.Logger, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if body == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Warn("encode response", slog.String("error", err.Error()))
	}
}

func writeError(w http.ResponseWriter, log *slog.Logger, status int, code, message string) {
	writeJSON(w, log, status, errorBody{Error: errorPayload{Code: code, Message: message}})
}

// decodeAndValidate parses the request body into dst and runs validator tags.
// On any failure returns a wrapped domain.ErrValidation suitable for the
// errors.Is mapping in mapErrToHTTP.
func decodeAndValidate(r *http.Request, dst any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20)) // 1 MiB
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return fmt.Errorf("empty body: %w", domain.ErrValidation)
		}
		return fmt.Errorf("decode body: %w", domain.ErrValidation)
	}
	if err := validate.Struct(dst); err != nil {
		return fmt.Errorf("validate body: %w", domain.ErrValidation)
	}
	return nil
}
