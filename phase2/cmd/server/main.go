// Phase 2 Server - Production Features
//
// Extends Phase 1 with:
// - Prometheus metrics & structured logging (zap)
// - File upload/create/delete endpoints
// - File versioning & conflict detection
// - SSE real-time sync
// - File sharing (ACLs + share links)
// - Rate limiting & user quotas
// - Multi-backend storage (S3, local, SMB)
package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
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
	"github.com/fruitsalade/fruitsalade/phase2/internal/storage"
	"github.com/fruitsalade/fruitsalade/phase2/internal/storage/local"
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

	// Initialize auth
	db := metaStore.DB()
	authHandler := auth.New(db, cfg.JWTSecret)
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
	permissionStore := sharing.NewPermissionStore(db)
	shareLinkStore := sharing.NewShareLinkStore(db)
	groupStore := sharing.NewGroupStore(db)
	permissionStore.SetGroupStore(groupStore)
	logging.Info("sharing stores initialized (with groups)")

	// Initialize quota store and rate limiter
	quotaStore := quota.NewQuotaStore(db)
	rateLimiter := quota.NewRateLimiter(quotaStore)
	logging.Info("quota and rate limiter initialized")

	// Create provisioner for auto-provisioning group folders and home directories
	provisioner := sharing.NewProvisioner(groupStore, metaStore, permissionStore)
	logging.Info("provisioner initialized")

	// Initialize storage location store and router
	locationStore := storage.NewLocationStore(db)

	storageRouter, err := storage.NewRouter(ctx, locationStore, groupStore)
	if err != nil {
		logging.Fatal("storage router init failed", zap.Error(err))
	}
	defer storageRouter.Close()

	// Auto-create default storage location on first run (if no locations exist)
	if storageRouter.DefaultLocation() == nil {
		var locName, backendType string
		var backendConfig json.RawMessage

		if cfg.StorageBackend == "s3" {
			locName = "Default S3"
			backendType = "s3"
			backendConfig, _ = json.Marshal(s3storage.BackendConfig{
				Endpoint:  cfg.S3Endpoint,
				Bucket:    cfg.S3Bucket,
				AccessKey: cfg.S3AccessKey,
				SecretKey: cfg.S3SecretKey,
				Region:    cfg.S3Region,
				UseSSL:    cfg.S3UseSSL,
			})
		} else {
			locName = "Default Local"
			backendType = "local"
			backendConfig, _ = json.Marshal(local.Config{
				RootPath:   cfg.LocalStoragePath,
				CreateDirs: true,
			})
		}

		_, err := locationStore.Create(ctx, &storage.LocationRow{
			Name:        locName,
			BackendType: backendType,
			Config:      backendConfig,
			IsDefault:   true,
		})
		if err != nil {
			logging.Fatal("failed to create default storage location", zap.Error(err))
		}
		if err := storageRouter.Reload(ctx); err != nil {
			logging.Fatal("failed to reload storage router", zap.Error(err))
		}
		logging.Info("auto-created default storage location",
			zap.String("backend", backendType),
			zap.String("name", locName))
	}

	// Create API server
	srv := api.NewServer(
		metaStore, storageRouter, authHandler, cfg.MaxUploadSize,
		broadcaster, permissionStore, shareLinkStore,
		quotaStore, rateLimiter, groupStore, cfg,
		provisioner, locationStore,
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
