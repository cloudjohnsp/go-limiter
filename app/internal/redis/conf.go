package redis

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"

	"go-limiter/internal/config"

	goredis "github.com/redis/go-redis/v9"
)

func NewRedisClient(cfg config.RedisConfig) (*goredis.Client, error) {
	log.Printf("Starting Redis on: %s", cfg.Addr)

	var tlsConfig *tls.Config
	if cfg.TLS {
		tlsConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}

	rdb := goredis.NewClient(&goredis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		PoolTimeout:  cfg.PoolTimeout,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		MaxRetries:   cfg.MaxRetries,
		TLSConfig:    tlsConfig,
	})

	ctx, cancel := context.WithTimeout(context.Background(), cfg.PingTimeout)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		return nil, fmt.Errorf("redis ping %s: %w", cfg.Addr, err)
	}
	log.Printf("Redis is running on: %s", cfg.Addr)
	return rdb, nil
}
