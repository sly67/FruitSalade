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
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/fruitsalade/fruitsalade/fruitsalade/internal/winclient"
	"github.com/fruitsalade/fruitsalade/shared/pkg/client"
	"github.com/fruitsalade/fruitsalade/shared/pkg/logger"
	"golang.org/x/term"
)

func main() {
	// Handle subcommands before flag.Parse()
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "login":
			cmdLogin(os.Args[2:])
			return
		case "logout":
			cmdLogout(os.Args[2:])
			return
		}
	}

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

	// Auto-load token from file if not provided via flag or env
	if *token == "" {
		*token = os.Getenv("FRUITSALADE_TOKEN")
	}
	if *token == "" {
		if tf, err := client.LoadToken(); err == nil {
			if tf.IsExpired(0) {
				fmt.Fprintf(os.Stderr, "Error: saved token has expired. Run 'login' to authenticate.\n")
				os.Exit(1)
			}
			*token = tf.Token
			logger.Info("Using saved token for %s@%s", tf.Username, tf.Server)
		}
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

func cmdLogin(args []string) {
	fs := flag.NewFlagSet("login", flag.ExitOnError)
	serverURL := fs.String("server", "http://localhost:48000", "Server URL")
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
