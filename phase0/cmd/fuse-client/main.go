// Phase 0 FUSE Client - Proof of Concept
//
// Features:
// - Metadata fetched from server on mount
// - On-demand content fetch on file open/read
// - Range reads for large files (>1MB)
// - LRU cache with configurable size limit
// - Periodic metadata refresh
// - Hash verification
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
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
	cacheDir := flag.String("cache", "", "Cache directory (default: ~/.cache/fruitsalade)")
	maxCacheMB := flag.Int64("max-cache", 1024, "Maximum cache size in MB (default: 1024)")
	refreshInterval := flag.Duration("refresh", 0, "Metadata refresh interval (e.g., 30s, 5m). 0 = disabled")
	watchSSE := flag.Bool("watch", false, "Subscribe to server events for real-time updates (requires server -watch)")
	healthCheck := flag.Duration("health-check", 30*time.Second, "Health check interval for offline recovery (0 = disabled)")
	verifyHash := flag.Bool("verify-hash", false, "Verify file hashes after download")
	verbose := flag.Bool("verbose", false, "Enable verbose/debug logging")
	quiet := flag.Bool("quiet", false, "Suppress all output except errors")
	logLevel := flag.String("log-level", "info", "Log level: quiet, error, info, debug")
	flag.Parse()

	// Set log level
	if *quiet {
		logger.SetLevel(logger.LevelQuiet)
	} else if *verbose {
		logger.SetLevel(logger.LevelDebug)
	} else {
		logger.SetLevel(logger.ParseLevel(*logLevel))
	}

	if *mountPoint == "" {
		fmt.Fprintln(os.Stderr, "Error: -mount is required")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Usage: fuse-client -mount <path> [options]")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Options:")
		fmt.Fprintln(os.Stderr, "  -server <url>       Server URL (default: http://localhost:8080)")
		fmt.Fprintln(os.Stderr, "  -cache <dir>        Cache directory (default: ~/.cache/fruitsalade)")
		fmt.Fprintln(os.Stderr, "  -max-cache <MB>     Max cache size in MB (default: 1024)")
		fmt.Fprintln(os.Stderr, "  -refresh <duration> Metadata refresh interval (e.g., 30s, 5m)")
		fmt.Fprintln(os.Stderr, "  -watch              Subscribe to server events (real-time updates)")
		fmt.Fprintln(os.Stderr, "  -health-check <dur> Health check interval when offline (default: 30s)")
		fmt.Fprintln(os.Stderr, "  -verify-hash        Verify file hashes after download")
		fmt.Fprintln(os.Stderr, "  -verbose            Enable verbose/debug logging")
		fmt.Fprintln(os.Stderr, "  -quiet              Suppress all output except errors")
		fmt.Fprintln(os.Stderr, "  -log-level <level>  Log level: quiet, error, info, debug")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Example:")
		fmt.Fprintln(os.Stderr, "  sudo ./fuse-client -mount /mnt/fruitsalade -server http://localhost:8080")
		fmt.Fprintln(os.Stderr, "  sudo ./fuse-client -mount /mnt/fs -watch -verbose  # real-time sync")
		os.Exit(1)
	}

	if *cacheDir == "" {
		home, _ := os.UserHomeDir()
		*cacheDir = home + "/.cache/fruitsalade"
	}

	maxCacheBytes := *maxCacheMB * 1024 * 1024

	if !*quiet {
		fmt.Println("╔═══════════════════════════════════════╗")
		fmt.Println("║   FruitSalade Phase 0 FUSE Client     ║")
		fmt.Println("╚═══════════════════════════════════════╝")
		fmt.Printf("  Mount:       %s\n", *mountPoint)
		fmt.Printf("  Server:      %s\n", *serverURL)
		fmt.Printf("  Cache:       %s\n", *cacheDir)
		fmt.Printf("  Max cache:   %d MB\n", *maxCacheMB)
		if *refreshInterval > 0 {
			fmt.Printf("  Refresh:     %s\n", *refreshInterval)
		}
		if *watchSSE {
			fmt.Printf("  Watch:       enabled (SSE)\n")
		}
		if *verifyHash {
			fmt.Printf("  Verify hash: enabled\n")
		}
		fmt.Println()
	}

	// Create filesystem
	cfg := fuse.Config{
		ServerURL:         *serverURL,
		CacheDir:          *cacheDir,
		MaxCacheSize:      maxCacheBytes,
		RefreshInterval:   *refreshInterval,
		WatchSSE:          *watchSSE,
		HealthCheckPeriod: *healthCheck,
		VerifyHash:        *verifyHash,
	}

	fsys, err := fuse.NewFruitFS(cfg)
	if err != nil {
		log.Fatalf("Failed to create filesystem: %v", err)
	}

	// Fetch metadata
	ctx := context.Background()
	if err := fsys.FetchMetadata(ctx); err != nil {
		log.Fatalf("Failed to fetch metadata: %v", err)
	}

	// Mount filesystem
	server, err := fsys.Mount(*mountPoint)
	if err != nil {
		log.Fatalf("Failed to mount: %v", err)
	}

	// Start metadata refresh loop if enabled
	fsys.StartRefreshLoop(ctx)

	// Start SSE watch if enabled
	fsys.StartSSEWatch(ctx)

	// Start health check for offline recovery
	fsys.StartHealthCheck(ctx)

	logger.Info("Mounted at %s", *mountPoint)
	if !*quiet {
		log.Println()
		log.Println("Features:")
		log.Println("  - Small files (<1MB): fetched fully and cached")
		log.Println("  - Large files (>=1MB): range reads on demand")
		log.Println("  - LRU eviction when cache exceeds limit")
		if *refreshInterval > 0 {
			log.Printf("  - Metadata refresh: every %s", *refreshInterval)
		}
		if *watchSSE {
			log.Println("  - Real-time sync: SSE events")
		}
		if *healthCheck > 0 {
			log.Printf("  - Offline recovery: health check every %s", *healthCheck)
		}
		if *verifyHash {
			log.Println("  - Hash verification: enabled")
		}
		log.Println()
		log.Println("Try these commands in another terminal:")
		log.Printf("  ls %s", *mountPoint)
		log.Printf("  cat %s/<filename>", *mountPoint)
		log.Printf("  head -c 100 %s/large.bin  # range read", *mountPoint)
		log.Println()
		log.Println("Press Ctrl+C to unmount and exit")
	}

	// Handle signals for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Wait for signal or server exit
	go func() {
		server.Wait()
	}()

	<-quit

	// Stop watchers
	fsys.StopHealthCheck()
	fsys.StopSSEWatch()
	fsys.StopRefreshLoop()

	// Print cache stats before exit
	used, max, count := fsys.CacheStats()
	stats := fsys.GetStats()

	if !*quiet {
		log.Println()
		log.Printf("Cache stats: %d files, %d MB / %d MB used", count, used/(1024*1024), max/(1024*1024))
		log.Printf("Session stats: %d fetches, %d cache hits, %d misses, %d range reads",
			stats.ContentFetches.Load(), stats.CacheHits.Load(), stats.CacheMisses.Load(), stats.RangeReads.Load())
		log.Printf("Downloaded: %d MB, From cache: %d MB",
			stats.BytesDownloaded.Load()/(1024*1024), stats.BytesFromCache.Load()/(1024*1024))
		if stats.OfflineErrors.Load() > 0 {
			log.Printf("Offline errors: %d", stats.OfflineErrors.Load())
		}
	}

	logger.Info("Unmounting...")
	if err := server.Unmount(); err != nil {
		logger.Error("Unmount error: %v", err)
	}
	logger.Info("Done.")
}
