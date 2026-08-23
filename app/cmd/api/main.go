package main

import (
	"log"
	"net/http"
	"time"

	"go-limiter/internal/api/handlers"
	"go-limiter/internal/middleware"
	"go-limiter/internal/redis"
)

func main() {
	mux := http.NewServeMux()

	rdb, err := redis.NewRedisClient()
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	limiter := redis.NewTokenBucket(redis.TokenBucketConfig{
		Client:         rdb,
		Capacity:       30,          // Maximum burst size
		RefillRate:     1,           // Add 1 token per interval
		RefillInterval: time.Second, // Every 1 second
	})

	mux.HandleFunc("/healthz", handlers.ApiHandler)
	mux.Handle("/api/", middleware.RateLimit(limiter, http.HandlerFunc(handlers.ApiHandler)))

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
