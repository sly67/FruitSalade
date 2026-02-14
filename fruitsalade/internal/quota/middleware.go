package quota

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/fruitsalade/fruitsalade/fruitsalade/internal/metrics"
	"github.com/fruitsalade/fruitsalade/shared/pkg/protocol"
)

// UserIDFromContext extracts the user ID from the request context.
// This function type allows decoupling from the auth package.
type UserIDFromContext func(ctx context.Context) (userID int, rpm int, ok bool)

// RateLimitMiddleware returns middleware that enforces per-user rate limits.
func RateLimitMiddleware(limiter *RateLimiter, store *QuotaStore, getUserInfo UserIDFromContext) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, _, ok := getUserInfo(r.Context())
			if !ok {
				// No user context (unauthenticated request) - let it pass
				next.ServeHTTP(w, r)
				return
			}

			// Get user's rate limit from quota
			q, err := store.GetQuota(r.Context(), userID)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			rpm := q.MaxRequestsPerMin
			if !limiter.Allow(userID, rpm) {
				metrics.RecordRateLimitHit()
				retryAfter := limiter.RetryAfter(userID, rpm)
				w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				json.NewEncoder(w).Encode(protocol.ErrorResponse{
					Error: "rate limit exceeded",
					Code:  http.StatusTooManyRequests,
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
