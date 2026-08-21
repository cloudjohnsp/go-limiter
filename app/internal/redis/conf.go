package redis

import (
	"log"
	"context"
	"fmt"
	"os"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

func NewRedisClient() (*goredis.Client, error) {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}

	log.Printf("Starting Redis on: %s", addr)

	rdb := goredis.NewClient(&goredis.Options{
		Addr:     addr,
		Password: "",
		DB:       0,

		PoolSize: 20,
		MinIdleConns: 5,

		PoolTimeout:  1 * time.Second,
		DialTimeout:  500 * time.Millisecond,
		ReadTimeout:  500 * time.Millisecond,
		WriteTimeout: 500 * time.Millisecond,

		MaxRetries:      0,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		return nil, fmt.Errorf("redis ping %s: %w", addr, err)
	}
	log.Printf("Redis is running on: %s", addr)
	return rdb, nil
}
