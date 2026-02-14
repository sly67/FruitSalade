// Phase 2 FUSE Client - Production
//
// Full-featured client with read + write support:
// - JWT authentication
// - LRU cache with configurable size
// - Metadata refresh and SSE watch
// - Health check for offline recovery
// - File creation, modification, deletion via FUSE
// - Extended attributes for file status
//
// Sub-commands:
//
//	fruitsalade-fuse mount [flags]    Mount filesystem (default)
//	fruitsalade-fuse pin <file-id>    Pin a cached file
//	fruitsalade-fuse unpin <file-id>  Unpin a cached file
//	fruitsalade-fuse pinned           List pinned files
//	fruitsalade-fuse status           Show cache status
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fruitsalade/fruitsalade/shared/pkg/cache"
	"github.com/fruitsalade/fruitsalade/shared/pkg/fuse"
	"github.com/fruitsalade/fruitsalade/shared/pkg/logger"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "pin":
			cmdPin(os.Args[2:])
			return
		case "unpin":
			cmdUnpin(os.Args[2:])
			return
		case "pinned":
			cmdPinned(os.Args[2:])
			return
		case "status":
			cmdStatus(os.Args[2:])
			return
		case "mount":
			// Strip "mount" from args and fall through to normal parsing
			os.Args = append(os.Args[:1], os.Args[2:]...)
		}
	}

	cmdMount()
}

func cmdMount() {
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
		*token = os.Getenv("FRUITSALADE_TOKEN")
	}

	if *token == "" {
		fmt.Fprintf(os.Stderr, "Error: -token or FRUITSALADE_TOKEN environment variable required\n")
		os.Exit(1)
	}

	logger.Info("FruitSalade Phase 2 FUSE Client (read/write)")
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

	fruitFS.SetAuthToken(*token)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger.Info("Fetching metadata...")
	if err := fruitFS.FetchMetadata(ctx); err != nil {
		logger.Error("Failed to fetch metadata: %v", err)
		os.Exit(1)
	}

	server, err := fruitFS.Mount(*mountPoint)
	if err != nil {
		logger.Error("Mount failed: %v", err)
		os.Exit(1)
	}

	fruitFS.StartRefreshLoop(ctx)
	fruitFS.StartSSEWatch(ctx)
	fruitFS.StartHealthCheck(ctx)

	logger.Info("Filesystem mounted at %s (read/write)", *mountPoint)
	logger.Info("Press Ctrl+C to unmount and exit")

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

func openCache(args []string) (*cache.Cache, *flag.FlagSet) {
	fs := flag.NewFlagSet("", flag.ExitOnError)
	cacheDir := fs.String("cache", "/tmp/fruitsalade-cache", "Cache directory")
	fs.Parse(args)
	c, err := cache.New(*cacheDir, 0) // size 0 = we're not writing, just managing
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening cache: %v\n", err)
		os.Exit(1)
	}
	return c, fs
}

func cmdPin(args []string) {
	fs := flag.NewFlagSet("pin", flag.ExitOnError)
	cacheDir := fs.String("cache", "/tmp/fruitsalade-cache", "Cache directory")
	fs.Parse(args)

	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "Usage: fruitsalade-fuse pin [-cache dir] <file-id>\n")
		os.Exit(1)
	}

	fileID := fs.Arg(0)
	c, err := cache.New(*cacheDir, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := c.Pin(fileID); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if err := c.SavePins(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to persist pins: %v\n", err)
	}
	fmt.Printf("Pinned: %s\n", fileID)
}

func cmdUnpin(args []string) {
	fs := flag.NewFlagSet("unpin", flag.ExitOnError)
	cacheDir := fs.String("cache", "/tmp/fruitsalade-cache", "Cache directory")
	fs.Parse(args)

	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "Usage: fruitsalade-fuse unpin [-cache dir] <file-id>\n")
		os.Exit(1)
	}

	fileID := fs.Arg(0)
	c, err := cache.New(*cacheDir, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := c.Unpin(fileID); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if err := c.SavePins(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to persist pins: %v\n", err)
	}
	fmt.Printf("Unpinned: %s\n", fileID)
}

func cmdPinned(args []string) {
	fs := flag.NewFlagSet("pinned", flag.ExitOnError)
	cacheDir := fs.String("cache", "/tmp/fruitsalade-cache", "Cache directory")
	fs.Parse(args)

	c, err := cache.New(*cacheDir, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	c.LoadPins()

	pinned := c.Pinned()
	if len(pinned) == 0 {
		fmt.Println("No pinned files.")
		return
	}

	fmt.Printf("%-40s  %10s  %s\n", "FILE ID", "SIZE", "PATH")
	for _, e := range pinned {
		fmt.Printf("%-40s  %10d  %s\n", e.FileID, e.Size, e.LocalPath)
	}
}

func cmdStatus(args []string) {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	cacheDir := fs.String("cache", "/tmp/fruitsalade-cache", "Cache directory")
	fs.Parse(args)

	c, err := cache.New(*cacheDir, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	size, maxSize, count := c.Stats()
	pinned := c.Pinned()

	fmt.Printf("Cache directory: %s\n", *cacheDir)
	fmt.Printf("Cached files:    %d\n", count)
	fmt.Printf("Cache size:      %d bytes\n", size)
	fmt.Printf("Max size:        %d bytes\n", maxSize)
	fmt.Printf("Pinned files:    %d\n", len(pinned))
}
