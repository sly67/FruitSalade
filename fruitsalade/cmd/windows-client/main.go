// FruitSalade Windows/cross-platform client.
//
// Supports two backends:
//   - cfapi: Windows Cloud Files API (native Explorer integration, placeholders)
//   - fuse:  cgofuse via WinFSP (cross-platform, works on Linux/macOS/Windows)
//
// Usage:
//
//	fruitsalade-winclient -server http://host:48000 -token TOKEN -sync-root /path
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/fruitsalade/fruitsalade/fruitsalade/internal/winclient"
	"github.com/fruitsalade/fruitsalade/shared/pkg/logger"
)

func main() {
	mode := flag.String("mode", "auto", "Backend mode: auto, cfapi, fuse")
	syncRoot := flag.String("sync-root", defaultSyncRoot(), "Sync root directory")
	server := flag.String("server", "http://localhost:48000", "Server URL")
	token := flag.String("token", "", "Auth token (JWT)")
	cacheDir := flag.String("cache", defaultCacheDir(), "Cache directory")
	maxCache := flag.Int64("max-cache", 1<<30, "Max cache size in bytes")
	refresh := flag.Duration("refresh", 30*time.Second, "Metadata refresh interval (0 to disable)")
	watchSSE := flag.Bool("watch", true, "Watch for SSE events")
	healthCheck := flag.Duration("health-check", 15*time.Second, "Health check period (0 to disable)")
	verifyHash := flag.Bool("verify-hash", false, "Verify file hashes after download")
	verbose := flag.Bool("v", false, "Verbose (debug) logging")
	installService := flag.Bool("install-service", false, "Install as Windows service")
	uninstallService := flag.Bool("uninstall-service", false, "Uninstall Windows service")

	flag.Parse()

	if *verbose {
		logger.SetLevel(logger.LevelDebug)
	}

	// Handle service install/uninstall
	if *installService {
		doInstallService()
		return
	}
	if *uninstallService {
		doUninstallService()
		return
	}

	// Check if running as Windows service
	if isWindowsService() {
		runAsService(*mode, *syncRoot, *server, *token, *cacheDir, *maxCache,
			*refresh, *watchSSE, *healthCheck, *verifyHash)
		return
	}

	cfg := winclient.CoreConfig{
		ServerURL:         *server,
		AuthToken:         *token,
		CacheDir:          *cacheDir,
		SyncRoot:          *syncRoot,
		MaxCacheSize:      *maxCache,
		RefreshInterval:   *refresh,
		HealthCheckPeriod: *healthCheck,
		WatchSSE:          *watchSSE,
		VerifyHash:        *verifyHash,
	}

	core, err := winclient.NewClientCore(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Fetch initial metadata
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := core.FetchMetadata(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to fetch metadata: %v\n", err)
		os.Exit(1)
	}

	// Select backend
	backend := selectBackend(*mode, *syncRoot)
	logger.Info("Using backend: %s", backend.Name())
	logger.Info("Sync root: %s", *syncRoot)
	logger.Info("Server: %s", *server)

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		logger.Info("Shutting down...")
		cancel()
		backend.Stop()
	}()

	// Start backend (blocks)
	if err := backend.Start(ctx, core); err != nil {
		if ctx.Err() != nil {
			// Clean shutdown
			logger.Info("Stopped")
			return
		}
		fmt.Fprintf(os.Stderr, "Backend error: %v\n", err)
		os.Exit(1)
	}
}

func selectBackend(mode, syncRoot string) winclient.Backend {
	switch mode {
	case "cfapi":
		return winclient.NewCfAPIBackend(syncRoot)
	case "fuse":
		return winclient.NewCgoFuseBackend(syncRoot)
	case "auto":
		if runtime.GOOS == "windows" {
			return winclient.NewCfAPIBackend(syncRoot)
		}
		return winclient.NewCgoFuseBackend(syncRoot)
	default:
		fmt.Fprintf(os.Stderr, "Unknown mode: %s (use auto, cfapi, or fuse)\n", mode)
		os.Exit(1)
		return nil
	}
}

func defaultSyncRoot() string {
	if runtime.GOOS == "windows" {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "FruitSalade")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "FruitSalade")
}

func defaultCacheDir() string {
	if runtime.GOOS == "windows" {
		cacheDir, _ := os.UserCacheDir()
		return filepath.Join(cacheDir, "FruitSalade", "cache")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache", "fruitsalade")
}
