// Package winclient provides the backend-agnostic core for the Windows/cross-platform client.
package winclient

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fruitsalade/fruitsalade/shared/pkg/cache"
	"github.com/fruitsalade/fruitsalade/shared/pkg/client"
	"github.com/fruitsalade/fruitsalade/shared/pkg/logger"
	"github.com/fruitsalade/fruitsalade/shared/pkg/models"
)

// CoreConfig holds configuration for the ClientCore.
type CoreConfig struct {
	ServerURL         string
	AuthToken         string
	CacheDir          string
	SyncRoot          string
	MaxCacheSize      int64
	RefreshInterval   time.Duration
	HealthCheckPeriod time.Duration
	WatchSSE          bool
	VerifyHash        bool
}

// CoreStats holds client statistics.
type CoreStats struct {
	MetadataFetches atomic.Int64
	ContentFetches  atomic.Int64
	CacheHits       atomic.Int64
	CacheMisses     atomic.Int64
	RangeReads      atomic.Int64
	BytesDownloaded atomic.Int64
	BytesFromCache  atomic.Int64
	FailedFetches   atomic.Int64
	OfflineErrors   atomic.Int64
	BytesUploaded   atomic.Int64
	FilesCreated    atomic.Int64
	DirsCreated     atomic.Int64
	FilesDeleted    atomic.Int64
	DirsDeleted     atomic.Int64
	Renames         atomic.Int64
}

// MetadataDiff represents changes between two metadata trees.
type MetadataDiff struct {
	Added   []*models.FileNode
	Removed []*models.FileNode
	Changed []*models.FileNode // nodes where size, hash, or modtime changed
}

// ClientCore is the backend-agnostic core used by both CfAPI and cgofuse backends.
type ClientCore struct {
	Client    *client.Client
	SSEClient *client.SSEClient
	Cache     *cache.Cache
	Config    CoreConfig
	Stats     CoreStats

	mu       sync.RWMutex
	metadata *models.FileNode

	refreshTicker *time.Ticker
	refreshStop   chan struct{}
	sseCancel     context.CancelFunc
	healthCancel  context.CancelFunc
}

// NewClientCore creates a new ClientCore.
func NewClientCore(cfg CoreConfig) (*ClientCore, error) {
	if cfg.MaxCacheSize == 0 {
		cfg.MaxCacheSize = 1 << 30 // 1GB
	}

	c, err := cache.New(cfg.CacheDir, cfg.MaxCacheSize)
	if err != nil {
		return nil, fmt.Errorf("create cache: %w", err)
	}

	clientCfg := client.Config{
		BaseURL:   strings.TrimSuffix(cfg.ServerURL, "/"),
		Timeout:   60 * time.Second,
		AuthToken: cfg.AuthToken,
	}

	core := &ClientCore{
		Client:      client.New(clientCfg),
		Cache:       c,
		Config:      cfg,
		refreshStop: make(chan struct{}),
	}

	if cfg.WatchSSE {
		core.SSEClient = client.NewSSEClient(cfg.ServerURL)
		if cfg.AuthToken != "" {
			core.SSEClient.SetAuthToken(cfg.AuthToken)
		}
	}

	return core, nil
}

// Metadata returns the current metadata tree (read-locked).
func (c *ClientCore) Metadata() *models.FileNode {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.metadata
}

// FetchMetadata fetches the full metadata tree from the server.
func (c *ClientCore) FetchMetadata(ctx context.Context) error {
	logger.Info("Fetching metadata from %s", c.Config.ServerURL)

	tree, err := c.Client.FetchMetadata(ctx)
	if err != nil {
		return fmt.Errorf("fetch metadata: %w", err)
	}

	c.mu.Lock()
	c.metadata = tree
	c.mu.Unlock()

	c.Stats.MetadataFetches.Add(1)
	logger.Info("Metadata loaded: %d items", countNodes(tree))
	return nil
}

// RefreshMetadata refreshes the metadata and returns a diff of changes.
func (c *ClientCore) RefreshMetadata(ctx context.Context) (*MetadataDiff, error) {
	logger.Debug("Refreshing metadata...")

	tree, err := c.Client.FetchMetadata(ctx)
	if err != nil {
		logger.Error("Metadata refresh failed: %v", err)
		return nil, err
	}

	c.mu.Lock()
	oldTree := c.metadata
	c.metadata = tree
	c.mu.Unlock()

	c.Stats.MetadataFetches.Add(1)

	diff := DiffMetadata(oldTree, tree)

	oldCount := countNodes(oldTree)
	newCount := countNodes(tree)
	if oldCount != newCount {
		logger.Info("Metadata refreshed: %d -> %d items (+%d/-%d/~%d)",
			oldCount, newCount, len(diff.Added), len(diff.Removed), len(diff.Changed))
	} else {
		logger.Debug("Metadata refreshed: %d items", newCount)
	}

	return diff, nil
}

// FindByPath resolves a path in the metadata tree.
func (c *ClientCore) FindByPath(path string) *models.FileNode {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return findByPath(c.metadata, path)
}

// FindByID finds a node by its ID.
func (c *ClientCore) FindByID(id string) *models.FileNode {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return findByID(c.metadata, id)
}

// FetchContent fetches a file's content, using cache if available.
// Returns the local cache path.
func (c *ClientCore) FetchContent(ctx context.Context, node *models.FileNode) (string, error) {
	fileID := cacheID(node.ID)

	if cachePath, ok := c.Cache.Get(fileID); ok {
		logger.Debug("Cache hit: %s", node.Path)
		c.Stats.CacheHits.Add(1)
		return cachePath, nil
	}

	c.Stats.CacheMisses.Add(1)

	if !c.Client.IsOnline() {
		logger.Error("Cannot fetch %s: server offline (not cached)", node.Path)
		c.Stats.OfflineErrors.Add(1)
		return "", fmt.Errorf("server offline, file not cached: %s", node.Path)
	}

	serverID := strings.TrimPrefix(node.ID, "/")

	reader, _, err := c.Client.FetchContentFull(ctx, serverID)
	if err != nil {
		c.Stats.FailedFetches.Add(1)
		return "", fmt.Errorf("fetch content %s: %w", node.Path, err)
	}
	defer reader.Close()

	var hashReader io.Reader = reader
	var hasher hash.Hash
	if c.Config.VerifyHash && node.Hash != "" {
		hasher = sha256.New()
		hashReader = io.TeeReader(reader, hasher)
	}

	cachePath, err := c.Cache.Put(fileID, hashReader, node.Size)
	if err != nil {
		c.Stats.FailedFetches.Add(1)
		return "", fmt.Errorf("cache put %s: %w", node.Path, err)
	}

	if hasher != nil && node.Hash != "" {
		actualHash := hex.EncodeToString(hasher.Sum(nil))
		if actualHash != node.Hash {
			c.Cache.Evict(fileID)
			return "", fmt.Errorf("hash mismatch for %s: expected %s, got %s", node.Path, node.Hash, actualHash)
		}
		logger.Debug("Hash verified: %s", node.Path)
	}

	c.Stats.ContentFetches.Add(1)
	c.Stats.BytesDownloaded.Add(node.Size)
	return cachePath, nil
}

// FetchContentRange fetches a byte range of a file's content.
func (c *ClientCore) FetchContentRange(ctx context.Context, fileID string, offset, length int64) ([]byte, error) {
	if !c.Client.IsOnline() {
		c.Stats.OfflineErrors.Add(1)
		return nil, fmt.Errorf("server offline")
	}

	serverID := strings.TrimPrefix(fileID, "/")

	reader, _, err := c.Client.FetchContent(ctx, serverID, offset, length)
	if err != nil {
		c.Stats.FailedFetches.Add(1)
		return nil, err
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	c.Stats.RangeReads.Add(1)
	c.Stats.BytesDownloaded.Add(int64(len(data)))
	return data, nil
}

// UploadFile uploads a local file to the server.
func (c *ClientCore) UploadFile(ctx context.Context, serverPath, localPath string) (*client.UploadResponse, error) {
	f, err := os.Open(localPath)
	if err != nil {
		return nil, fmt.Errorf("open local file: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat local file: %w", err)
	}

	// Use SectionReader so HTTP client doesn't close our file
	reader := io.NewSectionReader(f, 0, info.Size())

	resp, err := c.Client.UploadFile(ctx, serverPath, reader, info.Size())
	if err != nil {
		return nil, err
	}

	c.Stats.BytesUploaded.Add(info.Size())
	return resp, nil
}

// UploadReader uploads content from a reader to the server.
func (c *ClientCore) UploadReader(ctx context.Context, serverPath string, r io.Reader, size int64) (*client.UploadResponse, error) {
	resp, err := c.Client.UploadFile(ctx, serverPath, r, size)
	if err != nil {
		return nil, err
	}
	c.Stats.BytesUploaded.Add(size)
	return resp, nil
}

// DeletePath deletes a file or directory on the server.
func (c *ClientCore) DeletePath(ctx context.Context, serverPath string) error {
	return c.Client.DeletePath(ctx, serverPath)
}

// CreateDirectory creates a directory on the server.
func (c *ClientCore) CreateDirectory(ctx context.Context, serverPath string) error {
	return c.Client.CreateDirectory(ctx, serverPath)
}

// IsOnline returns true if the server is reachable.
func (c *ClientCore) IsOnline() bool {
	return c.Client.IsOnline()
}

// CacheStats returns cache statistics.
func (c *ClientCore) CacheStats() (used, max int64, count int) {
	return c.Cache.Stats()
}

// AddMetadataChild adds a child node to a parent in the metadata tree.
func (c *ClientCore) AddMetadataChild(parentPath string, child *models.FileNode) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if parent := findByPath(c.metadata, parentPath); parent != nil {
		parent.Children = append(parent.Children, child)
	}
}

// RemoveMetadataChild removes a child node from a parent in the metadata tree.
func (c *ClientCore) RemoveMetadataChild(parentPath, childName string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if parent := findByPath(c.metadata, parentPath); parent != nil {
		removeChildFromNode(parent, childName)
	}
}

// UpdateMetadataNode updates a node's metadata in the tree.
func (c *ClientCore) UpdateMetadataNode(path string, size int64, hash string, modTime time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if node := findByPath(c.metadata, path); node != nil {
		node.Size = size
		node.Hash = hash
		node.ModTime = modTime
	}
}

// StartBackgroundLoops starts the refresh, SSE, and health check loops.
func (c *ClientCore) StartBackgroundLoops(ctx context.Context) {
	c.startRefreshLoop(ctx)
	c.startSSEWatch(ctx)
	c.startHealthCheck(ctx)
}

// StopBackgroundLoops stops all background loops.
func (c *ClientCore) StopBackgroundLoops() {
	c.stopRefreshLoop()
	c.stopSSEWatch()
	c.stopHealthCheck()
}

func (c *ClientCore) startRefreshLoop(ctx context.Context) {
	if c.Config.RefreshInterval <= 0 {
		return
	}

	c.refreshTicker = time.NewTicker(c.Config.RefreshInterval)

	go func() {
		for {
			select {
			case <-c.refreshTicker.C:
				c.RefreshMetadata(ctx)
			case <-c.refreshStop:
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	logger.Info("Metadata refresh enabled: every %v", c.Config.RefreshInterval)
}

func (c *ClientCore) stopRefreshLoop() {
	if c.refreshTicker != nil {
		c.refreshTicker.Stop()
		close(c.refreshStop)
	}
}

func (c *ClientCore) startSSEWatch(ctx context.Context) {
	if c.SSEClient == nil {
		return
	}

	sseCtx, cancel := context.WithCancel(ctx)
	c.sseCancel = cancel

	events, errors := c.SSEClient.Subscribe(sseCtx)

	go func() {
		for {
			select {
			case _, ok := <-events:
				if !ok {
					return
				}
				if _, err := c.RefreshMetadata(ctx); err != nil {
					logger.Error("SSE refresh failed: %v", err)
				}
			case err, ok := <-errors:
				if !ok {
					return
				}
				if err != nil {
					logger.Error("SSE error: %v", err)
				}
			case <-sseCtx.Done():
				return
			}
		}
	}()

	logger.Info("SSE watch enabled")
}

func (c *ClientCore) stopSSEWatch() {
	if c.sseCancel != nil {
		c.sseCancel()
		c.sseCancel = nil
	}
}

func (c *ClientCore) startHealthCheck(ctx context.Context) {
	if c.Config.HealthCheckPeriod <= 0 {
		return
	}

	healthCtx, cancel := context.WithCancel(ctx)
	c.healthCancel = cancel

	go func() {
		ticker := time.NewTicker(c.Config.HealthCheckPeriod)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				wasOnline := c.Client.IsOnline()
				err := c.Client.Ping(healthCtx)

				if err == nil && !wasOnline {
					logger.Info("Server is back online, refreshing metadata...")
					if _, refreshErr := c.RefreshMetadata(healthCtx); refreshErr != nil {
						logger.Error("Failed to refresh metadata: %v", refreshErr)
					}
				}
			case <-healthCtx.Done():
				return
			}
		}
	}()

	logger.Info("Health check enabled: every %v", c.Config.HealthCheckPeriod)
}

func (c *ClientCore) stopHealthCheck() {
	if c.healthCancel != nil {
		c.healthCancel()
		c.healthCancel = nil
	}
}

// DiffMetadata computes the difference between two metadata trees.
func DiffMetadata(oldTree, newTree *models.FileNode) *MetadataDiff {
	diff := &MetadataDiff{}

	oldMap := flattenTree(oldTree)
	newMap := flattenTree(newTree)

	for path, newNode := range newMap {
		oldNode, exists := oldMap[path]
		if !exists {
			diff.Added = append(diff.Added, newNode)
		} else if nodeChanged(oldNode, newNode) {
			diff.Changed = append(diff.Changed, newNode)
		}
	}

	for path, oldNode := range oldMap {
		if _, exists := newMap[path]; !exists {
			diff.Removed = append(diff.Removed, oldNode)
		}
	}

	return diff
}

func flattenTree(root *models.FileNode) map[string]*models.FileNode {
	result := make(map[string]*models.FileNode)
	if root == nil {
		return result
	}
	flattenTreeRecursive(root, result)
	return result
}

func flattenTreeRecursive(node *models.FileNode, result map[string]*models.FileNode) {
	result[node.Path] = node
	for _, child := range node.Children {
		flattenTreeRecursive(child, result)
	}
}

func nodeChanged(old, new *models.FileNode) bool {
	return old.Size != new.Size ||
		old.Hash != new.Hash ||
		!old.ModTime.Equal(new.ModTime) ||
		old.IsDir != new.IsDir
}

func countNodes(node *models.FileNode) int {
	if node == nil {
		return 0
	}
	count := 1
	for _, child := range node.Children {
		count += countNodes(child)
	}
	return count
}

func findByPath(root *models.FileNode, path string) *models.FileNode {
	if root == nil {
		return nil
	}
	if root.Path == path {
		return root
	}
	for _, child := range root.Children {
		if found := findByPath(child, path); found != nil {
			return found
		}
	}
	return nil
}

func findByID(root *models.FileNode, id string) *models.FileNode {
	if root == nil {
		return nil
	}
	if root.ID == id {
		return root
	}
	for _, child := range root.Children {
		if found := findByID(child, id); found != nil {
			return found
		}
	}
	return nil
}

func removeChildFromNode(parent *models.FileNode, name string) {
	for i, child := range parent.Children {
		if child.Name == name {
			parent.Children = append(parent.Children[:i], parent.Children[i+1:]...)
			return
		}
	}
}

// cacheID converts a file ID to a cache-safe key.
func cacheID(id string) string {
	return strings.ReplaceAll(id, "/", "_")
}

// BuildChildPath constructs a child path from parent + name.
func BuildChildPath(parentPath, name string) string {
	if parentPath == "/" {
		return "/" + name
	}
	return parentPath + "/" + name
}
