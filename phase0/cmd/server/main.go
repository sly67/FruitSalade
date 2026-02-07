// Phase 0 Server - Proof of Concept
//
// Minimal server with:
// - In-memory metadata (built from directory tree)
// - Local filesystem storage backend
// - HTTP Range support for partial downloads
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fruitsalade/fruitsalade/phase0/internal/api"
	"github.com/fruitsalade/fruitsalade/phase0/internal/storage"
	"github.com/fruitsalade/fruitsalade/phase0/internal/watcher"
)

func main() {
	port := flag.Int("port", 8080, "Server port")
	dataDir := flag.String("data", "./testdata", "Directory containing files to serve")
	watchEnabled := flag.Bool("watch", false, "Enable file watching with SSE events")
	watchInterval := flag.Duration("watch-interval", 5*time.Second, "File watch polling interval")
	flag.Parse()

	fmt.Println("╔═══════════════════════════════════════╗")
	fmt.Println("║   FruitSalade Phase 0 Server          ║")
	fmt.Println("╚═══════════════════════════════════════╝")
	fmt.Printf("  Port:     %d\n", *port)
	fmt.Printf("  Data dir: %s\n", *dataDir)
	fmt.Println()

	// Check data directory
	if _, err := os.Stat(*dataDir); os.IsNotExist(err) {
		log.Printf("Creating data directory: %s", *dataDir)
		if err := os.MkdirAll(*dataDir, 0755); err != nil {
			log.Fatalf("Failed to create data directory: %v", err)
		}
		// Create a sample file
		sampleFile := *dataDir + "/hello.txt"
		if err := os.WriteFile(sampleFile, []byte("Hello from FruitSalade!\n"), 0644); err != nil {
			log.Printf("Warning: couldn't create sample file: %v", err)
		} else {
			log.Printf("Created sample file: %s", sampleFile)
		}
	}

	// Initialize storage
	store, err := storage.NewLocalStorage(*dataDir)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}

	// Initialize server
	srv := api.NewServer(store)
	if err := srv.Init(context.Background()); err != nil {
		log.Fatalf("Failed to initialize server: %v", err)
	}

	// Initialize file watcher if enabled
	var fileWatcher *watcher.Watcher
	if *watchEnabled {
		fileWatcher = watcher.New(*dataDir, *watchInterval)
		if err := fileWatcher.Start(context.Background()); err != nil {
			log.Fatalf("Failed to start file watcher: %v", err)
		}
		srv.SetWatcher(fileWatcher)
		log.Printf("File watching enabled (interval: %s)", *watchInterval)
	}

	// Create HTTP server
	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", *port),
		Handler:      srv.Handler(),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Server listening on http://localhost:%d", *port)
		log.Println()
		log.Println("Endpoints:")
		log.Println("  GET /health                - Health check")
		log.Println("  GET /api/v1/tree           - Full metadata tree")
		log.Println("  GET /api/v1/tree/{path}    - Subtree for path")
		log.Println("  GET /api/v1/content/{path} - File content (Range supported)")
		log.Println("  GET /api/v1/events         - SSE file change events (-watch)")
		log.Println()

		if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Stop file watcher
	if fileWatcher != nil {
		fileWatcher.Stop()
		log.Println("File watcher stopped.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	log.Println("Server stopped.")
}
