package auth

import (
	"context"
	"net/http"
)

type contextKey string

const claimsKey = contextKey("claims")

// AddClaimsToContext adds JWT claims to the request context
func AddClaimsToContext(ctx context.Context, claims *Claims) context.Context {
	return context.WithValue(ctx, claimsKey, claims)
}

// GetClaimsFromContext retrieves JWT claims from the request context
func GetClaimsFromContext(ctx context.Context) *Claims {
	claims, ok := ctx.Value(claimsKey).(*Claims)
	if !ok {
		return nil
	}
	return claims
}

// GetUsernameFromContext retrieves the username from the request context
func GetUsernameFromContext(ctx context.Context) string {
	claims := GetClaimsFromContext(ctx)
	if claims == nil {
		return ""
	}
	return claims.Username
}

// HasRoleInContext checks if the user has a specific role
func HasRoleInContext(ctx context.Context, role string) bool {
	claims := GetClaimsFromContext(ctx)
	if claims == nil {
		return false
	}
	for _, r := range claims.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// RequireRole is an HTTP middleware that requires a specific role
func RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !HasRoleInContext(r.Context(), role) {
				http.Error(w, "Forbidden: insufficient permissions", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
