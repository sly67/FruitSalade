// Phase 1 FUSE Client - MVP
//
// Production-ready client with:
// - JWT authentication
// - Full LRU cache with configurable size
// - Pin/Unpin support
// - Metadata refresh and SSE watch
// - Health check for offline recovery
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fruitsalade/fruitsalade/shared/pkg/fuse"
	"github.com/fruitsalade/fruitsalade/shared/pkg/logger"
)

func main() {
	mountPoint := flag.String("mount", "", "Mount point for virtual filesystem (required)")
	serverURL := flag.String("server", "http://localhost:8080", "Server URL")
	cacheDir := flag.String("cache", "/tmp/fruitsalade-cache", "Cache directory")
	maxCacheSize := flag.Int64("max-cache", 1<<30, "Maximum cache size in bytes (default 1GB)")
	refreshInterval := flag.Duration("refresh", 30*time.Second, "Metadata refresh interval (0 to disable)")
	verifyHash := flag.Bool("verify-hash", false, "Verify file hashes after download")
	watchSSE := flag.Bool("watch", false, "Subscribe to server events for real-time updates")
	healthCheck := flag.Duration("health-check", 30*time.Second, "Health check interval for offline recovery")
	token := flag.String("token", "", "JWT authentication token")
	verbosity := flag.Int("v", 1, "Verbosity level: 0=quiet, 1=info, 2=debug")

	flag.Parse()

	// Set log level
	switch *verbosity {
	case 0:
		logger.SetLevel(logger.LevelQuiet)
	case 1:
		logger.SetLevel(logger.LevelInfo)
	default:
		logger.SetLevel(logger.LevelDebug)
	}

	if *mountPoint == "" {
		fmt.Fprintf(os.Stderr, "Error: -mount is required\n")
		flag.Usage()
		os.Exit(1)
	}

	if *token == "" {
		// Check environment variable
		*token = os.Getenv("FRUITSALADE_TOKEN")
	}

	if *token == "" {
		fmt.Fprintf(os.Stderr, "Error: -token or FRUITSALADE_TOKEN environment variable required\n")
		os.Exit(1)
	}

	logger.Info("FruitSalade Phase 1 FUSE Client")
	logger.Info("  Server:     %s", *serverURL)
	logger.Info("  Mount:      %s", *mountPoint)
	logger.Info("  Cache:      %s (max %d MB)", *cacheDir, *maxCacheSize/(1<<20))

	cfg := fuse.Config{
		ServerURL:         *serverURL,
		CacheDir:          *cacheDir,
		MaxCacheSize:      *maxCacheSize,
		RefreshInterval:   *refreshInterval,
		VerifyHash:        *verifyHash,
		WatchSSE:          *watchSSE,
		HealthCheckPeriod: *healthCheck,
	}

	fruitFS, err := fuse.NewFruitFS(cfg)
	if err != nil {
		logger.Error("Failed to create filesystem: %v", err)
		os.Exit(1)
	}

	// Set auth token on the underlying HTTP client
	fruitFS.SetAuthToken(*token)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Fetch initial metadata
	logger.Info("Fetching metadata...")
	if err := fruitFS.FetchMetadata(ctx); err != nil {
		logger.Error("Failed to fetch metadata: %v", err)
		os.Exit(1)
	}

	// Mount filesystem
	server, err := fruitFS.Mount(*mountPoint)
	if err != nil {
		logger.Error("Mount failed: %v", err)
		os.Exit(1)
	}

	// Start background tasks
	fruitFS.StartRefreshLoop(ctx)
	fruitFS.StartSSEWatch(ctx)
	fruitFS.StartHealthCheck(ctx)

	logger.Info("Filesystem mounted at %s", *mountPoint)
	logger.Info("Press Ctrl+C to unmount and exit")

	// Wait for signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	logger.Info("Unmounting...")
	fruitFS.StopRefreshLoop()
	fruitFS.StopSSEWatch()
	fruitFS.StopHealthCheck()
	server.Unmount()
	logger.Info("Done")
}
