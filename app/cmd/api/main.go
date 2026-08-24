package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"go-limiter/internal/api/handlers"
	"go-limiter/internal/middleware"
	"go-limiter/internal/redis"
)

const (
	shutdownPeriod      = 15 * time.Second
	readinessDrainDelay = 5 * time.Second
)

var isShuttingDown atomic.Bool

func main() {
	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	mux := http.NewServeMux()
	rdb, err := redis.NewRedisClient()
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer func() {
		if err := rdb.Close(); err != nil {
			log.Printf("Failed to close Redis client: %v", err)
		}
	}()

	limiter := redis.NewTokenBucket(redis.TokenBucketConfig{
		Client:         rdb,
		Capacity:       30,
		RefillRate:     1,
		RefillInterval: time.Second,
	})

	mux.HandleFunc("/healthz", healthHandler)
	mux.Handle("/api/", middleware.RateLimit(limiter, http.HandlerFunc(handlers.ApiHandler)))

	server := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       500 * time.Millisecond,
		WriteTimeout:      500 * time.Millisecond,
		IdleTimeout:       60 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("Starting API server on: %s", server.Addr)
		serverErr <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Failed to start API server: %v", err)
		}
		return
	case <-rootCtx.Done():
		log.Println("Received shutdown signal; starting graceful shutdown.")
	}

	isShuttingDown.Store(true)
	server.SetKeepAlivesEnabled(false)
	time.Sleep(readinessDrainDelay)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownPeriod)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Graceful shutdown timed out: %v", err)
		if closeErr := server.Close(); closeErr != nil && !errors.Is(closeErr, http.ErrServerClosed) {
			log.Printf("Failed to force-close server: %v", closeErr)
		}
		return
	}

	log.Println("Server shut down gracefully.")
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	if isShuttingDown.Load() {
		http.Error(w, "shutting down", http.StatusServiceUnavailable)
		return
	}
	handlers.ApiHandler(w, r)
}
