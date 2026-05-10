package auth

import (
	"log/slog"
	"net/http"
	"strings"
)

// Middleware returns an HTTP middleware that requires a valid Clerk JWT in the
// Authorization header. On success, the verified user ID is injected into ctx.
// On failure, a low-cardinality 401 JSON envelope is written and the chain is
// short-circuited; the technical error is logged via slog.
func Middleware(v Verifier, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := bearerToken(r)
			if !ok {
				writeUnauthorized(w, log, "missing or malformed authorization header")
				return
			}

			userID, err := v.Verify(r.Context(), token)
			if err != nil {
				log.WarnContext(r.Context(), "auth verify failed", slog.String("error", err.Error()))
				writeUnauthorized(w, log, "invalid token")
				return
			}

			ctx := WithUserID(r.Context(), userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func bearerToken(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	if h == "" {
		return "", false
	}
	const prefix = "Bearer "
	if len(h) <= len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return "", false
	}
	tok := strings.TrimSpace(h[len(prefix):])
	if tok == "" {
		return "", false
	}
	return tok, true
}

func writeUnauthorized(w http.ResponseWriter, log *slog.Logger, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	body := `{"error":{"code":"unauthorized","message":"` + msg + `"}}`
	if _, err := w.Write([]byte(body)); err != nil {
		log.Warn("write 401 body", slog.String("error", err.Error()))
	}
}
