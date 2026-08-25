package middleware

import (
	"context"
	"log"
	"net/http"
	"time"

	"go-limiter/internal/helpers"
	"go-limiter/internal/redis"
)

func RateLimit(limiter *redis.TokenBucket, requestTimeout time.Duration, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestApikey, err := helpers.GetRequestApiKey(r)

		if err != nil {
			http.Error(w, "Failed to retrieve key", http.StatusBadRequest)
			return
		}

		key := "rl:apikey:" + requestApikey
		log.Printf("key: %s", key)

		ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
		defer cancel()

		allowed, remaining, err := limiter.Allow(ctx, key)

		if err != nil {
			// Choose intentionally: fail open or fail closed.
			http.Error(w, "Rate limiter unavailable", http.StatusServiceUnavailable)
			return
		}

		if !allowed {
			w.Header().Set("Retry-After", "1")
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		log.Printf("Request Allowed. %d tokens remaining. \n", int64(remaining))
		next.ServeHTTP(w, r)
	})
}
