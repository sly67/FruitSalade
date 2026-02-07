// Package config loads configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all Phase 1 server configuration.
type Config struct {
	// Server
	ListenAddr string

	// Database
	DatabaseURL string

	// S3 storage
	S3Endpoint  string
	S3Bucket    string
	S3AccessKey string
	S3SecretKey string
	S3Region    string
	S3UseSSL    bool

	// Auth
	JWTSecret string
}

// Load reads configuration from environment variables with defaults.
func Load() (*Config, error) {
	cfg := &Config{
		ListenAddr:  envOr("LISTEN_ADDR", ":8080"),
		DatabaseURL: envOr("DATABASE_URL", ""),
		S3Endpoint:  envOr("S3_ENDPOINT", "http://localhost:9000"),
		S3Bucket:    envOr("S3_BUCKET", "fruitsalade"),
		S3AccessKey: envOr("S3_ACCESS_KEY", "minioadmin"),
		S3SecretKey: envOr("S3_SECRET_KEY", "minioadmin"),
		S3Region:    envOr("S3_REGION", "us-east-1"),
		S3UseSSL:    envBool("S3_USE_SSL", false),
		JWTSecret:   envOr("JWT_SECRET", ""),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}

	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}
