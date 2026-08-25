package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Server    ServerConfig
	Redis     RedisConfig
	RateLimit RateLimitConfig
	Shutdown  ShutdownConfig
}

type ServerConfig struct {
	Addr              string
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
}

type RedisConfig struct {
	Addr         string
	Password     string
	DB           int
	PoolSize     int
	MinIdleConns int
	PoolTimeout  time.Duration
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	PingTimeout  time.Duration
	MaxRetries   int
	TLS          bool
}

type RateLimitConfig struct {
	Capacity       int64
	RefillRate     int64
	RefillInterval time.Duration
	RequestTimeout time.Duration
}

type ShutdownConfig struct {
	Period              time.Duration
	ReadinessDrainDelay time.Duration
}

// Load reads configuration from environment variables and applies safe local defaults.
func Load() (Config, error) {
	var err error
	cfg := Config{
		Server: ServerConfig{
			Addr:              stringValue("SERVER_ADDR", ":8080"),
			ReadHeaderTimeout: durationValue("SERVER_READ_HEADER_TIMEOUT", 5*time.Second),
			ReadTimeout:       durationValue("SERVER_READ_TIMEOUT", 500*time.Millisecond),
			WriteTimeout:      durationValue("SERVER_WRITE_TIMEOUT", 500*time.Millisecond),
			IdleTimeout:       durationValue("SERVER_IDLE_TIMEOUT", 60*time.Second),
		},
		Redis: RedisConfig{
			Addr:         stringValue("REDIS_ADDR", "localhost:6379"),
			Password:     os.Getenv("REDIS_PASSWORD"),
			DB:           intValue("REDIS_DB", 0),
			PoolSize:     intValue("REDIS_POOL_SIZE", 20),
			MinIdleConns: intValue("REDIS_MIN_IDLE_CONNS", 5),
			PoolTimeout:  durationValue("REDIS_POOL_TIMEOUT", time.Second),
			DialTimeout:  durationValue("REDIS_DIAL_TIMEOUT", 500*time.Millisecond),
			ReadTimeout:  durationValue("REDIS_READ_TIMEOUT", 500*time.Millisecond),
			WriteTimeout: durationValue("REDIS_WRITE_TIMEOUT", 500*time.Millisecond),
			PingTimeout:  durationValue("REDIS_PING_TIMEOUT", 500*time.Millisecond),
			MaxRetries:   intValue("REDIS_MAX_RETRIES", 0),
			TLS:          boolValue("REDIS_TLS", false),
		},
		RateLimit: RateLimitConfig{
			Capacity:       int64(intValue("RATE_LIMIT_CAPACITY", 30)),
			RefillRate:     int64(intValue("RATE_LIMIT_REFILL_RATE", 1)),
			RefillInterval: durationValue("RATE_LIMIT_REFILL_INTERVAL", time.Second),
			RequestTimeout: durationValue("RATE_LIMIT_REQUEST_TIMEOUT", 300*time.Millisecond),
		},
		Shutdown: ShutdownConfig{
			Period:              durationValue("SHUTDOWN_PERIOD", 15*time.Second),
			ReadinessDrainDelay: durationValue("READINESS_DRAIN_DELAY", 5*time.Second),
		},
	}

	if err = validate(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func validate(cfg Config) error {
	if cfg.Server.Addr == "" || cfg.Redis.Addr == "" {
		return fmt.Errorf("SERVER_ADDR and REDIS_ADDR must not be empty")
	}
	if cfg.Redis.PoolSize <= 0 || cfg.Redis.MinIdleConns < 0 || cfg.Redis.MaxRetries < 0 {
		return fmt.Errorf("Redis pool settings must be non-negative, with REDIS_POOL_SIZE greater than zero")
	}
	if cfg.RateLimit.Capacity <= 0 || cfg.RateLimit.RefillRate <= 0 {
		return fmt.Errorf("RATE_LIMIT_CAPACITY and RATE_LIMIT_REFILL_RATE must be greater than zero")
	}
	for name, value := range map[string]time.Duration{
		"SERVER_READ_HEADER_TIMEOUT": cfg.Server.ReadHeaderTimeout,
		"SERVER_READ_TIMEOUT":        cfg.Server.ReadTimeout,
		"SERVER_WRITE_TIMEOUT":       cfg.Server.WriteTimeout,
		"SERVER_IDLE_TIMEOUT":        cfg.Server.IdleTimeout,
		"REDIS_POOL_TIMEOUT":         cfg.Redis.PoolTimeout,
		"REDIS_DIAL_TIMEOUT":         cfg.Redis.DialTimeout,
		"REDIS_READ_TIMEOUT":         cfg.Redis.ReadTimeout,
		"REDIS_WRITE_TIMEOUT":        cfg.Redis.WriteTimeout,
		"REDIS_PING_TIMEOUT":         cfg.Redis.PingTimeout,
		"RATE_LIMIT_REFILL_INTERVAL": cfg.RateLimit.RefillInterval,
		"RATE_LIMIT_REQUEST_TIMEOUT": cfg.RateLimit.RequestTimeout,
		"SHUTDOWN_PERIOD":            cfg.Shutdown.Period,
		"READINESS_DRAIN_DELAY":      cfg.Shutdown.ReadinessDrainDelay,
	} {
		if value <= 0 {
			return fmt.Errorf("%s must be greater than zero", name)
		}
	}
	return nil
}

func stringValue(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func intValue(name string, fallback int) int {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func boolValue(name string, fallback bool) bool {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func durationValue(name string, fallback time.Duration) time.Duration {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}
