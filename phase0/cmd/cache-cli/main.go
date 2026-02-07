// Package main provides a CLI tool for managing the FruitSalade cache.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/fruitsalade/fruitsalade/shared/pkg/cache"
	"github.com/fruitsalade/fruitsalade/shared/pkg/client"
	"github.com/fruitsalade/fruitsalade/shared/pkg/models"
)

func main() {
	cacheDir := flag.String("cache", "/tmp/fruitsalade-cache", "Cache directory")
	maxSize := flag.Int64("max-size", 1<<30, "Maximum cache size (bytes)")
	serverURL := flag.String("server", "http://localhost:8080", "Server URL (for prefetch)")
	concurrent := flag.Int("concurrent", 5, "Concurrent downloads (for prefetch)")
	pinAfter := flag.Bool("pin", false, "Pin files after prefetch")

	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}

	c, err := cache.New(*cacheDir, *maxSize)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening cache: %v\n", err)
		os.Exit(1)
	}

	cmd := args[0]
	cmdArgs := args[1:]

	switch cmd {
	case "list", "ls":
		cmdList(c)
	case "stats":
		cmdStats(c)
	case "clear":
		cmdClear(c)
	case "pin":
		cmdPin(c, cmdArgs)
	case "unpin":
		cmdUnpin(c, cmdArgs)
	case "pinned":
		cmdPinned(c)
	case "evict", "rm":
		cmdEvict(c, cmdArgs)
	case "json":
		cmdJSON(c)
	case "prefetch":
		cmdPrefetch(c, *serverURL, *concurrent, *pinAfter, cmdArgs)
	case "help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`FruitSalade Cache CLI

Usage: cache-cli [flags] <command> [args]

Flags:
  -cache <dir>       Cache directory (default: /tmp/fruitsalade-cache)
  -max-size <bytes>  Maximum cache size (default: 1GB)
  -server <url>      Server URL for prefetch (default: http://localhost:8080)
  -concurrent <n>    Concurrent downloads for prefetch (default: 5)
  -pin               Pin files after prefetch

Commands:
  list, ls           List all cached files
  stats              Show cache statistics
  clear              Clear all non-pinned files
  pin <fileID>       Pin a file (prevent eviction)
  unpin <fileID>     Unpin a file (allow eviction)
  pinned             List pinned files
  evict, rm <fileID> Remove a file from cache
  prefetch [pattern] Prefetch files from server (pattern: *.txt, all, or paths)
  json               Export cache info as JSON
  help               Show this help message

Examples:
  cache-cli -cache /var/cache/fruitsalade list
  cache-cli stats
  cache-cli pin hello.txt
  cache-cli clear
  cache-cli -server http://localhost:8080 prefetch all
  cache-cli prefetch "*.txt"
  cache-cli -pin prefetch subdir/`)
}

func cmdList(c *cache.Cache) {
	entries := c.List()
	if len(entries) == 0 {
		fmt.Println("Cache is empty")
		return
	}

	// Sort by last access time
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].LastAccess.After(entries[j].LastAccess)
	})

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "FILE ID\tSIZE\tPINNED\tLAST ACCESS")
	fmt.Fprintln(w, "-------\t----\t------\t-----------")

	for _, entry := range entries {
		pinned := ""
		if entry.Pinned {
			pinned = "yes"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			entry.FileID,
			formatSize(entry.Size),
			pinned,
			formatTime(entry.LastAccess))
	}
	w.Flush()
}

func cmdStats(c *cache.Cache) {
	size, maxSize, count := c.Stats()
	pinned := len(c.Pinned())

	fmt.Println("Cache Statistics")
	fmt.Println("----------------")
	fmt.Printf("Directory:    %s\n", c.Dir())
	fmt.Printf("Files:        %d\n", count)
	fmt.Printf("Pinned:       %d\n", pinned)
	fmt.Printf("Used:         %s\n", formatSize(size))
	fmt.Printf("Max:          %s\n", formatSize(maxSize))
	fmt.Printf("Usage:        %.1f%%\n", float64(size)/float64(maxSize)*100)
}

func cmdClear(c *cache.Cache) {
	count := c.Clear()
	fmt.Printf("Cleared %d files from cache\n", count)
}

func cmdPin(c *cache.Cache, args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: cache-cli pin <fileID>")
		os.Exit(1)
	}

	fileID := normalizeFileID(args[0])
	if err := c.Pin(fileID); err != nil {
		fmt.Fprintf(os.Stderr, "Error pinning file: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Pinned: %s\n", fileID)
}

func cmdUnpin(c *cache.Cache, args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: cache-cli unpin <fileID>")
		os.Exit(1)
	}

	fileID := normalizeFileID(args[0])
	if err := c.Unpin(fileID); err != nil {
		fmt.Fprintf(os.Stderr, "Error unpinning file: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Unpinned: %s\n", fileID)
}

func cmdPinned(c *cache.Cache) {
	entries := c.Pinned()
	if len(entries) == 0 {
		fmt.Println("No pinned files")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "FILE ID\tSIZE\tLAST ACCESS")
	fmt.Fprintln(w, "-------\t----\t-----------")

	for _, entry := range entries {
		fmt.Fprintf(w, "%s\t%s\t%s\n",
			entry.FileID,
			formatSize(entry.Size),
			formatTime(entry.LastAccess))
	}
	w.Flush()
}

func cmdEvict(c *cache.Cache, args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: cache-cli evict <fileID>")
		os.Exit(1)
	}

	fileID := normalizeFileID(args[0])
	if err := c.Evict(fileID); err != nil {
		fmt.Fprintf(os.Stderr, "Error evicting file: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Evicted: %s\n", fileID)
}

func cmdJSON(c *cache.Cache) {
	entries := c.List()
	size, maxSize, count := c.Stats()

	data := struct {
		Directory string `json:"directory"`
		Size      int64  `json:"size"`
		MaxSize   int64  `json:"max_size"`
		Count     int    `json:"count"`
		Entries   []struct {
			FileID     string `json:"file_id"`
			LocalPath  string `json:"local_path"`
			Size       int64  `json:"size"`
			Pinned     bool   `json:"pinned"`
			LastAccess string `json:"last_access"`
		} `json:"entries"`
	}{
		Directory: c.Dir(),
		Size:      size,
		MaxSize:   maxSize,
		Count:     count,
	}

	for _, entry := range entries {
		data.Entries = append(data.Entries, struct {
			FileID     string `json:"file_id"`
			LocalPath  string `json:"local_path"`
			Size       int64  `json:"size"`
			Pinned     bool   `json:"pinned"`
			LastAccess string `json:"last_access"`
		}{
			FileID:     entry.FileID,
			LocalPath:  entry.LocalPath,
			Size:       entry.Size,
			Pinned:     entry.Pinned,
			LastAccess: entry.LastAccess.Format(time.RFC3339),
		})
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(data)
}

func normalizeFileID(id string) string {
	return strings.ReplaceAll(id, "/", "_")
}

func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	return t.Format("2006-01-02 15:04:05")
}

func cmdPrefetch(c *cache.Cache, serverURL string, concurrent int, pinAfter bool, args []string) {
	pattern := "all"
	if len(args) > 0 {
		pattern = args[0]
	}

	fmt.Printf("Connecting to %s...\n", serverURL)

	// Create client
	cl := client.New(client.Config{
		BaseURL: serverURL,
		Timeout: 60 * time.Second,
	})

	// Fetch metadata
	ctx := context.Background()
	tree, err := cl.FetchMetadata(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching metadata: %v\n", err)
		os.Exit(1)
	}

	// Collect files to prefetch
	var files []*models.FileNode
	collectFiles(tree, pattern, &files)

	if len(files) == 0 {
		fmt.Println("No files match the pattern")
		return
	}

	fmt.Printf("Found %d files to prefetch\n", len(files))

	// Calculate total size
	var totalSize int64
	for _, f := range files {
		totalSize += f.Size
	}
	fmt.Printf("Total size: %s\n", formatSize(totalSize))

	// Prefetch files
	fileIDs := make([]string, len(files))
	fileMap := make(map[string]*models.FileNode)
	for i, f := range files {
		fileID := strings.TrimPrefix(f.ID, "/")
		fileIDs[i] = fileID
		fileMap[fileID] = f
	}

	fmt.Printf("Prefetching with %d concurrent downloads...\n\n", concurrent)

	successCount := 0
	failCount := 0
	var downloadedSize int64

	results := cl.FetchContentConcurrent(ctx, fileIDs, concurrent)
	for result := range results {
		if result.Err != nil {
			fmt.Printf("  ✗ %s: %v\n", result.FileID, result.Err)
			failCount++
			continue
		}

		// Save to cache
		cacheID := normalizeFileID(result.FileID)
		node := fileMap[result.FileID]
		size := node.Size
		if result.Size > 0 {
			size = result.Size
		}

		cachePath, err := c.Put(cacheID, result.Reader, size)
		result.Reader.Close()

		if err != nil {
			fmt.Printf("  ✗ %s: cache error: %v\n", result.FileID, err)
			failCount++
			continue
		}

		// Pin if requested
		if pinAfter {
			c.Pin(cacheID)
		}

		fmt.Printf("  ✓ %s (%s) -> %s\n", result.FileID, formatSize(size), filepath.Base(cachePath))
		successCount++
		downloadedSize += size
	}

	fmt.Println()
	fmt.Printf("Prefetch complete: %d success, %d failed\n", successCount, failCount)
	fmt.Printf("Downloaded: %s\n", formatSize(downloadedSize))

	if pinAfter && successCount > 0 {
		fmt.Printf("Pinned: %d files\n", successCount)
	}
}

// collectFiles collects files matching the pattern from the tree.
func collectFiles(node *models.FileNode, pattern string, files *[]*models.FileNode) {
	if node == nil {
		return
	}

	if !node.IsDir {
		// Check if file matches pattern
		if matchesPattern(node, pattern) {
			*files = append(*files, node)
		}
		return
	}

	// Recurse into children
	for _, child := range node.Children {
		collectFiles(child, pattern, files)
	}
}

// matchesPattern checks if a file matches the prefetch pattern.
func matchesPattern(node *models.FileNode, pattern string) bool {
	if pattern == "all" || pattern == "*" {
		return true
	}

	// Path prefix match (e.g., "subdir/")
	if strings.HasSuffix(pattern, "/") {
		return strings.HasPrefix(node.Path, pattern) || strings.HasPrefix(node.Path, "/"+pattern)
	}

	// Glob pattern match (e.g., "*.txt")
	if strings.Contains(pattern, "*") {
		matched, _ := filepath.Match(pattern, node.Name)
		return matched
	}

	// Exact name or path match
	return node.Name == pattern || node.Path == pattern || node.Path == "/"+pattern
}
