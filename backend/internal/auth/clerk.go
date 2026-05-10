package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/clerk/clerk-sdk-go/v2/jwt"
)

// Verifier validates Clerk JWTs. The interface lets handlers/tests swap a
// fake without dragging in the real SDK.
type Verifier interface {
	Verify(ctx context.Context, token string) (userID string, err error)
}

// ClerkVerifier wraps the Clerk SDK's jwt.Verify call. It lazy-fetches the
// JWKS keyed by `kid` on the first call and caches it internally.
type ClerkVerifier struct{}

// NewClerkVerifier configures the global Clerk SDK with the secret key and
// returns a verifier. The SDK uses package-level state (clerk.SetKey), which
// we accept as a one-time init at composition root.
func NewClerkVerifier(secretKey string) *ClerkVerifier {
	clerk.SetKey(secretKey)
	return &ClerkVerifier{}
}

// Verify validates the JWT signature, issuer, and expiry. Returns the
// Clerk user ID (Subject claim) on success.
func (v *ClerkVerifier) Verify(ctx context.Context, token string) (string, error) {
	if token == "" {
		return "", errors.New("empty token")
	}
	claims, err := jwt.Verify(ctx, &jwt.VerifyParams{Token: token})
	if err != nil {
		return "", fmt.Errorf("clerk verify: %w", err)
	}
	if claims.Subject == "" {
		return "", errors.New("clerk claims missing subject")
	}
	return claims.Subject, nil
}
