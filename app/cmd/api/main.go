package main

import (
	"log"
	"net/http"
	"go-limiter/internal/redis"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("OK"))
		if err != nil {
			log.Printf("Failed to write response: %v", err)
		}
	})

	addr := ":8080"
	log.Printf("Starting API server on: %s", addr)
	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	if _, err := redis.NewRedisClient(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Failed to start API server: %v", err)
	}
}