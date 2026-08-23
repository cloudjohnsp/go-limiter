package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
	"errors"

	"go-limiter/internal/redis"
"go-limiter/cmd/api/handlers"

	goredis "github.com/redis/go-redis/v9"
)

func main() {
	mux := http.NewServeMux()

	rdb, err := redis.NewRedisClient()
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	mux.HandleFunc("/healthz", handlers.HealthHandler)
	mux.Handle("/api", rateLimit(rdb, http.HandlerFunc(apiHandler)))

	addr := ":8080"
	log.Printf("Starting API server on: %s", addr)
	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Failed to start API server: %v", err)
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte("OK"))
	if err != nil {
		log.Printf("Failed to write response: %v", err)
	}
}

func rateLimit(rdb *goredis.Client, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = rdb

		requestApikey, err := getRequestApiKey(r)

		if err != nil {
			http.Error(w, "Failed to retrieve key", http.StatusBadRequest)
			return
		}

		key := "rl:apikey:" + requestApikey
		log.Printf("key: %s", key)

		ctx, cancel := context.WithTimeout(r.Context(), 300*time.Millisecond)
		defer cancel()
		// Executes an atomic Lua script.
		limiter := redis.NewTokenBucket(redis.TokenBucketConfig{
			Client:         rdb,
			Capacity:       30,          // Maximum burst size
			RefillRate:     1,           // Add 1 token per interval
			RefillInterval: time.Second, // Every 1 second
		})

		allowed, remaining, err := limiter.Allow(ctx, key)

		if err != nil {
			// Choose intentionally: fail open or fail closed.
			http.Error(w, "Rate limiter unavailable", http.StatusServiceUnavailable)
			return
		}

		if !allowed {
			w.Header().Set("Retry-After", "60")
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		log.Printf("Request Allowed. %d tokens remaining. \n", int64(remaining))
		next.ServeHTTP(w, r)
	})
}

func getClientIP(r *http.Request) string {
	// Prefer X-Real-IP: set by trusted edge nginx to $remote_addr.
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return realIP
	}

	// X-Forwarded-For: nginx overwrites this with $remote_addr at the edge.
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		ips := strings.Split(fwd, ",")
		return strings.TrimSpace(ips[0])
	}

	// Fallback when not behind a proxy (direct :8080 access)
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

func getRequestApiKey(r *http.Request) (string, error) {
	apiKey := r.Header.Get("X-API-KEY");
	if apiKey != "" {
		apiKey = apiKey
	} else {
		log.Printf("Failed to retrieve a valid api-key")
		return "", errors.New("Failed to retrieve a valid api-key")
	}
	return apiKey, nil
}

func apiHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte("OK"))
	if err != nil {
		log.Printf("Failed to write response: %v", err)
	}
}
