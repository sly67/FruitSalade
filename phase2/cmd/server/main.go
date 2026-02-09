// Phase 2 Server - Production Features
//
// Extends Phase 1 with:
// - Prometheus metrics & structured logging (zap)
// - File upload/create/delete endpoints
// - File versioning & conflict detection
// - SSE real-time sync
// - File sharing (ACLs + share links)
// - Rate limiting & user quotas
package main

import (
	"context"
	"crypto/tls"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/fruitsalade/fruitsalade/phase2/internal/api"
	"github.com/fruitsalade/fruitsalade/phase2/internal/auth"
	"github.com/fruitsalade/fruitsalade/phase2/internal/config"
	"github.com/fruitsalade/fruitsalade/phase2/internal/events"
	"github.com/fruitsalade/fruitsalade/phase2/internal/logging"
	"github.com/fruitsalade/fruitsalade/phase2/internal/metadata/postgres"
	"github.com/fruitsalade/fruitsalade/phase2/internal/metrics"
	"github.com/fruitsalade/fruitsalade/phase2/internal/quota"
	"github.com/fruitsalade/fruitsalade/phase2/internal/sharing"
	s3storage "github.com/fruitsalade/fruitsalade/phase2/internal/storage/s3"
	"go.uber.org/zap"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		// Can't use structured logging yet
		panic("configuration error: " + err.Error())
	}

	// Initialize structured logging
	if err := logging.Init(logging.Config{
		Level:  cfg.LogLevel,
		Format: cfg.LogFormat,
	}); err != nil {
		panic("logging init error: " + err.Error())
	}
	defer logging.Sync()

	logging.Info("FruitSalade Phase 2 Server starting...",
		zap.String("listen", cfg.ListenAddr),
		zap.String("metrics", cfg.MetricsAddr))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize PostgreSQL
	logging.Info("connecting to PostgreSQL...")
	metaStore, err := postgres.New(cfg.DatabaseURL)
	if err != nil {
		logging.Fatal("database connection failed", zap.Error(err))
	}
	defer metaStore.Close()

	// Run migrations
	migrationsDir := findMigrationsDir()
	if migrationsDir != "" {
		logging.Info("running migrations...", zap.String("dir", migrationsDir))
		if err := metaStore.Migrate(migrationsDir); err != nil {
			logging.Fatal("migration failed", zap.Error(err))
		}
	}

	// Initialize S3 storage
	logging.Info("connecting to S3...", zap.String("endpoint", cfg.S3Endpoint))
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
		logging.Fatal("S3 storage init failed", zap.Error(err))
	}

	// Initialize auth
	authHandler := auth.New(metaStore.DB(), cfg.JWTSecret)
	if err := authHandler.EnsureDefaultAdmin(ctx); err != nil {
		logging.Error("failed to ensure default admin", zap.Error(err))
	}

	// Initialize OIDC provider (optional)
	if cfg.OIDCIssuerURL != "" {
		oidcProvider, err := auth.NewOIDCProvider(ctx, auth.OIDCConfig{
			IssuerURL:    cfg.OIDCIssuerURL,
			ClientID:     cfg.OIDCClientID,
			ClientSecret: cfg.OIDCClientSecret,
			AdminClaim:   cfg.OIDCAdminClaim,
			AdminValue:   cfg.OIDCAdminValue,
		}, authHandler)
		if err != nil {
			logging.Fatal("OIDC provider init failed", zap.Error(err))
		}
		if oidcProvider != nil {
			authHandler.SetOIDCProvider(oidcProvider)
		}
	}

	// Initialize SSE broadcaster
	broadcaster := events.NewBroadcaster()
	logging.Info("SSE broadcaster initialized")

	// Initialize sharing stores
	db := metaStore.DB()
	permissionStore := sharing.NewPermissionStore(db)
	shareLinkStore := sharing.NewShareLinkStore(db)
	logging.Info("sharing stores initialized")

	// Initialize quota store and rate limiter
	quotaStore := quota.NewQuotaStore(db)
	rateLimiter := quota.NewRateLimiter(quotaStore)
	logging.Info("quota and rate limiter initialized")

	// Create API server
	srv := api.NewServer(
		storage, authHandler, cfg.MaxUploadSize,
		broadcaster, permissionStore, shareLinkStore,
		quotaStore, rateLimiter,
	)
	if err := srv.Init(ctx); err != nil {
		logging.Fatal("server init failed", zap.Error(err))
	}

	// Start metrics server
	metricsServer := &http.Server{
		Addr:    cfg.MetricsAddr,
		Handler: metrics.Handler(),
	}
	go func() {
		logging.Info("metrics server listening", zap.String("addr", cfg.MetricsAddr))
		if err := metricsServer.ListenAndServe(); err != http.ErrServerClosed {
			logging.Error("metrics server error", zap.Error(err))
		}
	}()

	// Start HTTP(S) server
	httpServer := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: srv.Handler(),
	}

	useTLS := cfg.TLSCertFile != "" && cfg.TLSKeyFile != ""
	if useTLS {
		httpServer.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS13,
		}
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		logging.Info("shutting down...")
		cancel()
		httpServer.Close()
		metricsServer.Close()
	}()

	// Start periodic metrics update
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				metaStore.UpdateConnectionMetrics()
			}
		}
	}()

	if useTLS {
		logging.Info("server listening (TLS 1.3)",
			zap.String("addr", cfg.ListenAddr),
			zap.String("cert", cfg.TLSCertFile))
		if err := httpServer.ListenAndServeTLS(cfg.TLSCertFile, cfg.TLSKeyFile); err != http.ErrServerClosed {
			logging.Fatal("server error", zap.Error(err))
		}
	} else {
		logging.Info("server listening (HTTP)", zap.String("addr", cfg.ListenAddr))
		if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
			logging.Fatal("server error", zap.Error(err))
		}
	}
}

func findMigrationsDir() string {
	candidates := []string{
		"migrations",
		"phase2/migrations",
		"../migrations",
		"../phase1/migrations", // Phase 2 can use Phase 1 migrations
	}

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
