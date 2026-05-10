// Package auth handles JWT verification (Clerk) and request-scoped user
// identity propagation. The middleware here injects the verified Clerk user
// ID into ctx; downstream layers retrieve it via UserIDFromContext.
package auth

import "context"

// ctxKey is unexported so other packages cannot accidentally collide with the
// same key type. The empty struct is the canonical Go pattern.
type ctxKey struct{}

var userIDKey = ctxKey{}

// WithUserID returns ctx annotated with the clerk user ID.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// UserIDFromContext extracts the clerk user ID, returning ("", false) if
// absent — handlers MUST treat the false case as 401.
func UserIDFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(userIDKey).(string)
	if !ok || v == "" {
		return "", false
	}
	return v, true
}
