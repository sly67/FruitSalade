// Phase 1 Server - MVP
//
// Production-ready server with:
// - PostgreSQL metadata storage
// - S3/MinIO storage backend
// - JWT authentication
// - Docker deployment
package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/fruitsalade/fruitsalade/phase1/internal/api"
	"github.com/fruitsalade/fruitsalade/phase1/internal/auth"
	"github.com/fruitsalade/fruitsalade/phase1/internal/config"
	"github.com/fruitsalade/fruitsalade/phase1/internal/metadata/postgres"
	s3storage "github.com/fruitsalade/fruitsalade/phase1/internal/storage/s3"
	"github.com/fruitsalade/fruitsalade/shared/pkg/logger"
)

func main() {
	logger.SetLevel(logger.LevelInfo)
	logger.Info("FruitSalade Phase 1 Server starting...")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Error("Configuration error: %v", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize PostgreSQL
	logger.Info("Connecting to PostgreSQL...")
	metaStore, err := postgres.New(cfg.DatabaseURL)
	if err != nil {
		logger.Error("Database connection failed: %v", err)
		os.Exit(1)
	}
	defer metaStore.Close()

	// Run migrations
	migrationsDir := findMigrationsDir()
	if migrationsDir != "" {
		logger.Info("Running migrations from %s...", migrationsDir)
		if err := metaStore.Migrate(migrationsDir); err != nil {
			logger.Error("Migration failed: %v", err)
			os.Exit(1)
		}
	}

	// Initialize S3 storage
	logger.Info("Connecting to S3 at %s...", cfg.S3Endpoint)
	s3Cfg := s3storage.Config{
		Endpoint:  cfg.S3Endpoint,
		Bucket:    cfg.S3Bucket,
		AccessKey: cfg.S3AccessKey,
		SecretKey: cfg.S3SecretKey,
		Region:    cfg.S3Region,
		UseSSL:    cfg.S3UseSSL,
	}
	storage, err := s3storage.New(ctx, s3Cfg, metaStore)
	if err != nil {
		logger.Error("S3 storage init failed: %v", err)
		os.Exit(1)
	}

	// Initialize auth
	authHandler := auth.New(metaStore.DB(), cfg.JWTSecret)
	if err := authHandler.EnsureDefaultAdmin(ctx); err != nil {
		logger.Error("Failed to ensure default admin: %v", err)
	}

	// Create API server
	srv := api.NewServer(storage, authHandler)
	if err := srv.Init(ctx); err != nil {
		logger.Error("Server init failed: %v", err)
		os.Exit(1)
	}

	// Start HTTP server
	httpServer := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: srv.Handler(),
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		logger.Info("Shutting down...")
		cancel()
		httpServer.Close()
	}()

	logger.Info("Server listening on %s", cfg.ListenAddr)
	if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
		logger.Error("Server error: %v", err)
		os.Exit(1)
	}
}

func findMigrationsDir() string {
	// Look in common locations
	candidates := []string{
		"migrations",
		"phase1/migrations",
		"../migrations",
	}

	// Also check relative to executable
	exe, _ := os.Executable()
	if exe != "" {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "migrations"))
	}

	for _, dir := range candidates {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
	}
	return ""
}
