package webdav

import (
	"net/http"
	"strings"

	"github.com/fruitsalade/fruitsalade/phase2/internal/auth"
	"github.com/fruitsalade/fruitsalade/phase2/internal/logging"
	"go.uber.org/zap"
)

// BasicAuthMiddleware returns middleware that authenticates via Basic Auth
// or Bearer token (for programmatic access).
func BasicAuthMiddleware(a *auth.Auth) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Try Bearer token first (Authorization: Bearer <jwt>)
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				// Delegate to existing JWT middleware for this single request
				a.Middleware(next).ServeHTTP(w, r)
				return
			}

			// Fall back to HTTP Basic Auth
			username, password, ok := r.BasicAuth()
			if !ok {
				w.Header().Set("WWW-Authenticate", `Basic realm="FruitSalade"`)
				http.Error(w, "Authentication required", http.StatusUnauthorized)
				return
			}

			claims, err := a.ValidateCredentials(r.Context(), username, password)
			if err != nil {
				logging.Warn("webdav auth failed",
					zap.String("username", username),
					zap.Error(err))
				w.Header().Set("WWW-Authenticate", `Basic realm="FruitSalade"`)
				http.Error(w, "Invalid credentials", http.StatusUnauthorized)
				return
			}

			ctx := auth.WithClaims(r.Context(), claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
