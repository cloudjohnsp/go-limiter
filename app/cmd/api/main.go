package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"go-limiter/internal/redis"

	goredis "github.com/redis/go-redis/v9"
)

func main() {
	mux := http.NewServeMux()

	rdb, err := redis.NewRedisClient()
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	mux.Handle("/api/", rateLimit(rdb, http.HandlerFunc(apiHandler)))
	mux.HandleFunc("/healthz", healthHandler)

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

		key := "rl:ip:" + getClientIP(r)
		log.Printf("key: %s", key)

		_, cancel := context.WithTimeout(r.Context(), 300*time.Millisecond)
		defer cancel()

		// result, err := rdb.EvalSha(ctx, scriptSHA, []string{key}, args...).Result()
		// if err != nil {
		// 	// Choose intentionally: fail open or fail closed.
		// 	http.Error(w, "rate limiter unavailable", http.StatusServiceUnavailable)
		// 	return
		// }

		// allowed := result.(int64) == 1
		// if !allowed {
		// 	w.Header().Set("Retry-After", "60") // Ideally return this from Lua.
		// 	http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
		// 	return
		// }

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

func apiHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte("OK"))
	if err != nil {
		log.Printf("Failed to write response: %v", err)
	}
}
