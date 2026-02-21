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
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/fruitsalade/fruitsalade/shared/pkg/cache"
	"github.com/fruitsalade/fruitsalade/shared/pkg/client"
	"github.com/fruitsalade/fruitsalade/shared/pkg/fuse"
	"github.com/fruitsalade/fruitsalade/shared/pkg/logger"
	"golang.org/x/term"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "login":
			cmdLogin(os.Args[2:])
			return
		case "logout":
			cmdLogout(os.Args[2:])
			return
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

	// Auto-load from token file if no token provided
	var tokenFile *client.TokenFile
	if *token == "" {
		tf, err := client.LoadToken()
		if err == nil {
			if tf.IsExpired(0) {
				fmt.Fprintf(os.Stderr, "Error: saved token has expired. Run 'fruitsalade-fuse login' to authenticate.\n")
				os.Exit(1)
			}
			*token = tf.Token
			tokenFile = tf
			logger.Info("Using saved token for %s@%s", tf.Username, tf.Server)
		}
	}

	if *token == "" {
		fmt.Fprintf(os.Stderr, "Error: no token available. Use -token, FRUITSALADE_TOKEN, or run 'fruitsalade-fuse login'\n")
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

	// Start token refresh loop if using a saved token file
	if tokenFile != nil {
		fruitFS.Client().StartTokenRefreshLoop(ctx, tokenFile)
	}

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

func cmdLogin(args []string) {
	fs := flag.NewFlagSet("login", flag.ExitOnError)
	serverURL := fs.String("server", "http://localhost:8080", "Server URL")
	useOIDC := fs.Bool("oidc", false, "Use OIDC device code flow")
	deviceName := fs.String("device", "", "Device name (default: hostname)")
	fs.Parse(args)

	if *deviceName == "" {
		name, _ := os.Hostname()
		*deviceName = name
	}

	cfg := client.Config{
		BaseURL: strings.TrimSuffix(*serverURL, "/"),
		Timeout: 30 * time.Second,
	}
	c := client.New(cfg)
	ctx := context.Background()

	if *useOIDC {
		resp, err := c.DeviceCodeAuth(ctx, *deviceName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		tf := &client.TokenFile{
			Token:     resp.Token,
			ExpiresAt: resp.ExpiresAt,
			Server:    *serverURL,
			Username:  resp.User.Username,
		}
		if err := client.SaveToken(tf); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to save token: %v\n", err)
		}
		fmt.Printf("Login successful! Token saved to %s\n", client.TokenFilePath())
		return
	}

	// Interactive username/password login
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Username: ")
	username, _ := reader.ReadString('\n')
	username = strings.TrimSpace(username)

	fmt.Print("Password: ")
	passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading password: %v\n", err)
		os.Exit(1)
	}
	password := string(passwordBytes)

	resp, err := c.Login(ctx, username, password, *deviceName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	tf := &client.TokenFile{
		Token:     resp.Token,
		ExpiresAt: resp.ExpiresAt,
		Server:    *serverURL,
		Username:  resp.User.Username,
	}
	if err := client.SaveToken(tf); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to save token: %v\n", err)
	}
	fmt.Printf("Login successful! Logged in as %s. Token saved to %s\n", resp.User.Username, client.TokenFilePath())
}

func cmdLogout(args []string) {
	fs := flag.NewFlagSet("logout", flag.ExitOnError)
	fs.Parse(args)

	tf, err := client.LoadToken()
	if err != nil {
		fmt.Fprintf(os.Stderr, "No saved token found.\n")
		os.Exit(1)
	}

	cfg := client.Config{
		BaseURL:   strings.TrimSuffix(tf.Server, "/"),
		Timeout:   10 * time.Second,
		AuthToken: tf.Token,
	}
	c := client.New(cfg)

	if err := c.Logout(context.Background()); err != nil {
		logger.Debug("Server logout failed (token may already be expired): %v", err)
	}

	if err := client.DeleteToken(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to delete token file: %v\n", err)
	}
	fmt.Println("Logged out successfully.")
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
