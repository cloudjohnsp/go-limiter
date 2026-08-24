package main

import (
	"os"
	"log"
	"net/http"
	"time"
	"syscall"
	"signal"
	"atomic"

	"go-limiter/internal/api/handlers"
	"go-limiter/internal/middleware"
	"go-limiter/internal/redis"
)

const (
	_shutdownPeriod      = 15 * time.Second
	_shutdownHardPeriod  = 3 * time.Second
	_readinessDrainDelay = 5 * time.Second
)

var isShuttingDown atomic.Bool

func main() {
	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	ongoingCtx, stopOngoingGracefully := context.WithCancel(context.Background())

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
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       500 * time.Millisecond,
		WriteTimeout:      500 * time.Millisecond,
		IdleTimeout:       60 * time.Second
	}

	
	go func() {
		if err := server.ListenAndServe(ctx); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start API server: %v", err)
		}
	}()	
	
	<-rootCtx.Done()
	stop()
	isShuttingDown.Store(true)
	log.Println("Received shutdown signal, shutting down.")

	time.Sleep(_readinessDrainDelay)
	log.Println("Readiness check propagated, now waiting for ongoing requests to finish.")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), _shutdownPeriod)
	defer cancel()

	stopOngoingGracefully()
	if err != nil {
		log.Println("Failed to wait for ongoing requests to finish, waiting for forced cancellation.")
		time.Sleep(_shutdownHardPeriod)
	}

	log.Println("Server shut down gracefully.")
}
