package auth

import "context"

type ctxKey int

const identityKey ctxKey = 0

// IdentityFromContext returns the authenticated user's identity, if present.
func IdentityFromContext(ctx context.Context) (string, bool) {
	identity, ok := ctx.Value(identityKey).(string)
	return identity, ok
}

// WithIdentity returns a new context with the authenticated user's identity.
func WithIdentity(ctx context.Context, identity string) context.Context {
	return context.WithValue(ctx, identityKey, identity)
}
