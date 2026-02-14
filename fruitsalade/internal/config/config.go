// Package config loads configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all Phase 2 server configuration.
type Config struct {
	// Server
	ListenAddr  string
	MetricsAddr string

	// Logging
	LogLevel  string
	LogFormat string

	// Database
	DatabaseURL string

	// S3 storage
	S3Endpoint  string
	S3Bucket    string
	S3AccessKey string
	S3SecretKey string
	S3Region    string
	S3UseSSL    bool

	// TLS (optional — if both set, server uses HTTPS)
	TLSCertFile string
	TLSKeyFile  string

	// Auth
	JWTSecret string

	// OIDC (optional)
	OIDCIssuerURL    string
	OIDCClientID     string
	OIDCClientSecret string
	OIDCAdminClaim   string
	OIDCAdminValue   string

	// Storage backend ("local" or "s3", default: "local")
	StorageBackend   string
	LocalStoragePath string

	// Uploads
	MaxUploadSize int64

	// Quotas (defaults for new users)
	DefaultMaxStorage    int64
	DefaultMaxBandwidth  int64
	DefaultRequestsPerMin int
}

// Load reads configuration from environment variables with defaults.
func Load() (*Config, error) {
	cfg := &Config{
		ListenAddr:    envOr("LISTEN_ADDR", ":8080"),
		MetricsAddr:   envOr("METRICS_ADDR", ":9090"),
		LogLevel:      envOr("LOG_LEVEL", "info"),
		LogFormat:     envOr("LOG_FORMAT", "json"),
		DatabaseURL:   envOr("DATABASE_URL", ""),
		S3Endpoint:    envOr("S3_ENDPOINT", "http://localhost:9000"),
		S3Bucket:      envOr("S3_BUCKET", "fruitsalade"),
		S3AccessKey:   envOr("S3_ACCESS_KEY", "minioadmin"),
		S3SecretKey:   envOr("S3_SECRET_KEY", "minioadmin"),
		S3Region:      envOr("S3_REGION", "us-east-1"),
		S3UseSSL:      envBool("S3_USE_SSL", false),
		TLSCertFile:   envOr("TLS_CERT_FILE", ""),
		TLSKeyFile:    envOr("TLS_KEY_FILE", ""),
		JWTSecret:     envOr("JWT_SECRET", ""),
		OIDCIssuerURL:    envOr("OIDC_ISSUER_URL", ""),
		OIDCClientID:     envOr("OIDC_CLIENT_ID", ""),
		OIDCClientSecret: envOr("OIDC_CLIENT_SECRET", ""),
		OIDCAdminClaim:   envOr("OIDC_ADMIN_CLAIM", "is_admin"),
		OIDCAdminValue:   envOr("OIDC_ADMIN_VALUE", "true"),
		StorageBackend:       envOr("STORAGE_BACKEND", "local"),
		LocalStoragePath:     envOr("LOCAL_STORAGE_PATH", "/data/storage"),
		MaxUploadSize:        envInt64("MAX_UPLOAD_SIZE", 100*1024*1024), // 100MB default
		DefaultMaxStorage:    envInt64("DEFAULT_MAX_STORAGE", 0),        // 0 = unlimited
		DefaultMaxBandwidth:  envInt64("DEFAULT_MAX_BANDWIDTH", 0),      // 0 = unlimited
		DefaultRequestsPerMin: envInt("DEFAULT_REQUESTS_PER_MINUTE", 0), // 0 = unlimited
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

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return i
}

func envInt64(key string, fallback int64) int64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	i, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return fallback
	}
	return i
}
