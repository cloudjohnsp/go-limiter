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
	"go-limiter/internal/config"
	"go-limiter/internal/middleware"
	"go-limiter/internal/redis"
)

var isShuttingDown atomic.Bool

func main() {
	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	mux := http.NewServeMux()
	rdb, err := redis.NewRedisClient(cfg.Redis)
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
		Capacity:       cfg.RateLimit.Capacity,
		RefillRate:     cfg.RateLimit.RefillRate,
		RefillInterval: cfg.RateLimit.RefillInterval,
	})

	mux.HandleFunc("/healthz", healthHandler)
	mux.Handle("/api/", middleware.RateLimit(limiter, cfg.RateLimit.RequestTimeout, http.HandlerFunc(handlers.ApiHandler)))

	server := &http.Server{
		Addr:              cfg.Server.Addr,
		Handler:           mux,
		ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout,
		ReadTimeout:       cfg.Server.ReadTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout,
		IdleTimeout:       cfg.Server.IdleTimeout,
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
	time.Sleep(cfg.Shutdown.ReadinessDrainDelay)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Shutdown.Period)
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
