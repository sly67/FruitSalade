// Package fuse provides the FUSE filesystem implementation.
package fuse

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
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	gofuse "github.com/hanwen/go-fuse/v2/fuse"

	"github.com/fruitsalade/fruitsalade/shared/pkg/cache"
	"github.com/fruitsalade/fruitsalade/shared/pkg/client"
	"github.com/fruitsalade/fruitsalade/shared/pkg/logger"
	"github.com/fruitsalade/fruitsalade/shared/pkg/models"
)

// FruitFS is the main FUSE filesystem.
type FruitFS struct {
	fs.Inode

	client    *client.Client
	sseClient *client.SSEClient
	cache     *cache.Cache
	cfg       Config

	mu       sync.RWMutex
	metadata *models.FileNode

	refreshTicker *time.Ticker
	refreshStop   chan struct{}

	sseCancel    context.CancelFunc
	healthCancel context.CancelFunc

	stats Stats
}

// Stats holds filesystem statistics.
type Stats struct {
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

// FruitNode represents a file or directory in the filesystem.
type FruitNode struct {
	fs.Inode

	fsys     *FruitFS
	metadata *models.FileNode
}

// Config holds FUSE filesystem configuration.
type Config struct {
	ServerURL         string
	CacheDir          string
	MaxCacheSize      int64
	RefreshInterval   time.Duration
	VerifyHash        bool
	WatchSSE          bool
	HealthCheckPeriod time.Duration
}

// NewFruitFS creates a new FUSE filesystem.
func NewFruitFS(cfg Config) (*FruitFS, error) {
	if cfg.MaxCacheSize == 0 {
		cfg.MaxCacheSize = 1 << 30 // 1GB
	}

	c, err := cache.New(cfg.CacheDir, cfg.MaxCacheSize)
	if err != nil {
		return nil, fmt.Errorf("create cache: %w", err)
	}

	clientCfg := client.Config{
		BaseURL: strings.TrimSuffix(cfg.ServerURL, "/"),
		Timeout: 60 * time.Second,
	}

	f := &FruitFS{
		client:      client.New(clientCfg),
		cache:       c,
		cfg:         cfg,
		refreshStop: make(chan struct{}),
	}

	if cfg.WatchSSE {
		f.sseClient = client.NewSSEClient(cfg.ServerURL)
	}

	return f, nil
}

// SetAuthToken sets the JWT auth token on both the HTTP client and SSE client.
func (f *FruitFS) SetAuthToken(token string) {
	f.client.SetAuthToken(token)
	if f.sseClient != nil {
		f.sseClient.SetAuthToken(token)
	}
}

// FetchMetadata fetches the metadata tree from the server.
func (f *FruitFS) FetchMetadata(ctx context.Context) error {
	logger.Info("Fetching metadata from %s", f.cfg.ServerURL)

	tree, err := f.client.FetchMetadata(ctx)
	if err != nil {
		return fmt.Errorf("fetch metadata: %w", err)
	}

	f.mu.Lock()
	f.metadata = tree
	f.mu.Unlock()

	f.stats.MetadataFetches.Add(1)
	logger.Info("Metadata loaded: %d items", countNodes(tree))
	return nil
}

// RefreshMetadata refreshes the metadata tree.
func (f *FruitFS) RefreshMetadata(ctx context.Context) error {
	logger.Debug("Refreshing metadata...")

	tree, err := f.client.FetchMetadata(ctx)
	if err != nil {
		logger.Error("Metadata refresh failed: %v", err)
		return err
	}

	f.mu.Lock()
	oldCount := countNodes(f.metadata)
	f.metadata = tree
	newCount := countNodes(tree)
	f.mu.Unlock()

	f.stats.MetadataFetches.Add(1)

	if oldCount != newCount {
		logger.Info("Metadata refreshed: %d -> %d items", oldCount, newCount)
	} else {
		logger.Debug("Metadata refreshed: %d items (unchanged)", newCount)
	}

	return nil
}

// StartRefreshLoop starts periodic metadata refresh.
func (f *FruitFS) StartRefreshLoop(ctx context.Context) {
	if f.cfg.RefreshInterval <= 0 {
		return
	}

	f.refreshTicker = time.NewTicker(f.cfg.RefreshInterval)

	go func() {
		for {
			select {
			case <-f.refreshTicker.C:
				f.RefreshMetadata(ctx)
			case <-f.refreshStop:
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	logger.Info("Metadata refresh enabled: every %v", f.cfg.RefreshInterval)
}

// StopRefreshLoop stops the metadata refresh loop.
func (f *FruitFS) StopRefreshLoop() {
	if f.refreshTicker != nil {
		f.refreshTicker.Stop()
		close(f.refreshStop)
	}
}

// StartSSEWatch starts watching for server-sent events.
func (f *FruitFS) StartSSEWatch(ctx context.Context) {
	if f.sseClient == nil {
		return
	}

	sseCtx, cancel := context.WithCancel(ctx)
	f.sseCancel = cancel

	events, errors := f.sseClient.Subscribe(sseCtx)

	go func() {
		for {
			select {
			case event, ok := <-events:
				if !ok {
					return
				}
				logger.Debug("SSE event: %s %s", event.Type, event.Path)
				f.stats.MetadataFetches.Add(1)

				if err := f.RefreshMetadata(ctx); err != nil {
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

// StopSSEWatch stops the SSE event watcher.
func (f *FruitFS) StopSSEWatch() {
	if f.sseCancel != nil {
		f.sseCancel()
		f.sseCancel = nil
	}
}

// StartHealthCheck starts background health checking.
func (f *FruitFS) StartHealthCheck(ctx context.Context) {
	if f.cfg.HealthCheckPeriod <= 0 {
		return
	}

	healthCtx, cancel := context.WithCancel(ctx)
	f.healthCancel = cancel

	go func() {
		ticker := time.NewTicker(f.cfg.HealthCheckPeriod)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				wasOnline := f.client.IsOnline()
				err := f.client.Ping(healthCtx)

				if err == nil && !wasOnline {
					logger.Info("Server is back online, refreshing metadata...")
					if refreshErr := f.RefreshMetadata(healthCtx); refreshErr != nil {
						logger.Error("Failed to refresh metadata: %v", refreshErr)
					}
				}
			case <-healthCtx.Done():
				return
			}
		}
	}()

	logger.Info("Health check enabled: every %v", f.cfg.HealthCheckPeriod)
}

// StopHealthCheck stops the health check loop.
func (f *FruitFS) StopHealthCheck() {
	if f.healthCancel != nil {
		f.healthCancel()
		f.healthCancel = nil
	}
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

// Mount mounts the filesystem at the given path.
func (f *FruitFS) Mount(mountPoint string) (*gofuse.Server, error) {
	if err := os.MkdirAll(mountPoint, 0755); err != nil {
		return nil, fmt.Errorf("create mount point: %w", err)
	}

	root := &FruitNode{
		fsys:     f,
		metadata: f.metadata,
	}

	opts := &fs.Options{
		MountOptions: gofuse.MountOptions{
			AllowOther: false,
			Debug:      false,
			FsName:     "fruitsalade",
			Name:       "fruitsalade",
		},
		UID: uint32(os.Getuid()),
		GID: uint32(os.Getgid()),
	}

	server, err := fs.Mount(mountPoint, root, opts)
	if err != nil {
		return nil, fmt.Errorf("mount: %w", err)
	}

	return server, nil
}

// CacheStats returns cache statistics.
func (f *FruitFS) CacheStats() (used, max int64, count int) {
	return f.cache.Stats()
}

// GetStats returns filesystem statistics.
func (f *FruitFS) GetStats() Stats {
	return f.stats
}

// IsOnline returns true if the server is reachable.
func (f *FruitFS) IsOnline() bool {
	return f.client.IsOnline()
}

// Ensure FruitNode implements the required interfaces
var _ fs.InodeEmbedder = (*FruitNode)(nil)
var _ fs.NodeGetattrer = (*FruitNode)(nil)
var _ fs.NodeLookuper = (*FruitNode)(nil)
var _ fs.NodeReaddirer = (*FruitNode)(nil)
var _ fs.NodeOpener = (*FruitNode)(nil)
var _ fs.NodeReader = (*FruitNode)(nil)
var _ fs.NodeGetxattrer = (*FruitNode)(nil)
var _ fs.NodeListxattrer = (*FruitNode)(nil)
var _ fs.NodeCreater = (*FruitNode)(nil)
var _ fs.NodeMkdirer = (*FruitNode)(nil)
var _ fs.NodeUnlinker = (*FruitNode)(nil)
var _ fs.NodeRmdirer = (*FruitNode)(nil)
var _ fs.NodeSetattrer = (*FruitNode)(nil)
var _ fs.NodeRenamer = (*FruitNode)(nil)

// Getattr returns file attributes.
// CRITICAL: This must NEVER trigger a content download.
func (n *FruitNode) Getattr(ctx context.Context, fh fs.FileHandle, out *gofuse.AttrOut) syscall.Errno {
	if n.metadata == nil {
		return syscall.ENOENT
	}

	out.Mode = 0644
	if n.metadata.IsDir {
		out.Mode = 0755 | syscall.S_IFDIR
	} else {
		out.Mode = 0644 | syscall.S_IFREG
	}

	out.Size = uint64(n.metadata.Size)
	out.Mtime = uint64(n.metadata.ModTime.Unix())
	out.Atime = out.Mtime
	out.Ctime = out.Mtime
	out.Uid = uint32(os.Getuid())
	out.Gid = uint32(os.Getgid())

	return 0
}

// Lookup finds a child by name.
func (n *FruitNode) Lookup(ctx context.Context, name string, out *gofuse.EntryOut) (*fs.Inode, syscall.Errno) {
	if n.metadata == nil || !n.metadata.IsDir {
		return nil, syscall.ENOENT
	}

	var childMeta *models.FileNode
	for _, child := range n.metadata.Children {
		if child.Name == name {
			childMeta = child
			break
		}
	}

	if childMeta == nil {
		return nil, syscall.ENOENT
	}

	child := &FruitNode{
		fsys:     n.fsys,
		metadata: childMeta,
	}

	out.Mode = 0644
	if childMeta.IsDir {
		out.Mode = 0755 | syscall.S_IFDIR
	} else {
		out.Mode = 0644 | syscall.S_IFREG
	}
	out.Size = uint64(childMeta.Size)
	out.Mtime = uint64(childMeta.ModTime.Unix())
	out.Atime = out.Mtime
	out.Ctime = out.Mtime
	out.Uid = uint32(os.Getuid())
	out.Gid = uint32(os.Getgid())

	stableAttr := fs.StableAttr{Mode: out.Mode}
	return n.NewInode(ctx, child, stableAttr), 0
}

// Readdir lists directory contents.
func (n *FruitNode) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	if n.metadata == nil || !n.metadata.IsDir {
		return nil, syscall.ENOTDIR
	}

	entries := make([]gofuse.DirEntry, 0, len(n.metadata.Children))
	for _, child := range n.metadata.Children {
		mode := uint32(syscall.S_IFREG)
		if child.IsDir {
			mode = syscall.S_IFDIR
		}
		entries = append(entries, gofuse.DirEntry{
			Name: child.Name,
			Mode: mode,
		})
	}

	return fs.NewListDirStream(entries), 0
}

// Open prepares a file for reading or writing.
func (n *FruitNode) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	if n.metadata == nil || n.metadata.IsDir {
		return nil, 0, syscall.EISDIR
	}

	if flags&(syscall.O_WRONLY|syscall.O_RDWR) != 0 {
		return n.openForWrite(ctx, flags&syscall.O_TRUNC != 0)
	}

	fileID := n.getFileID()

	if cachePath, ok := n.fsys.cache.Get(fileID); ok {
		logger.Debug("Cache hit: %s", n.metadata.Path)
		n.fsys.stats.CacheHits.Add(1)
		return &FileHandle{
			node:      n,
			cachePath: cachePath,
			cached:    true,
		}, gofuse.FOPEN_KEEP_CACHE, 0
	}

	n.fsys.stats.CacheMisses.Add(1)

	if !n.fsys.client.IsOnline() {
		logger.Error("Cannot open %s: server offline (file not cached)", n.metadata.Path)
		n.fsys.stats.OfflineErrors.Add(1)
		return nil, 0, syscall.ENETUNREACH
	}

	const smallFileThreshold = 1 << 20

	if n.metadata.Size < smallFileThreshold {
		logger.Debug("Fetching small file: %s (%d bytes)", n.metadata.Path, n.metadata.Size)
		cachePath, err := n.fetchFullContent(ctx)
		if err != nil {
			logger.Error("Fetch error: %v", err)
			n.fsys.stats.FailedFetches.Add(1)
			return nil, 0, syscall.EIO
		}
		n.fsys.stats.ContentFetches.Add(1)
		return &FileHandle{
			node:      n,
			cachePath: cachePath,
			cached:    true,
		}, gofuse.FOPEN_KEEP_CACHE, 0
	}

	logger.Debug("Opening large file for range reads: %s (%d bytes)", n.metadata.Path, n.metadata.Size)
	return &FileHandle{
		node:      n,
		cachePath: "",
		cached:    false,
	}, 0, 0
}

// Read reads file content.
func (n *FruitNode) Read(ctx context.Context, fh fs.FileHandle, dest []byte, off int64) (gofuse.ReadResult, syscall.Errno) {
	handle, ok := fh.(*FileHandle)
	if !ok {
		return nil, syscall.EIO
	}

	if handle.writable && handle.tmpFile != nil {
		handle.mu.Lock()
		bytesRead, err := handle.tmpFile.ReadAt(dest, off)
		handle.mu.Unlock()
		if err != nil && err != io.EOF {
			return nil, syscall.EIO
		}
		return gofuse.ReadResultData(dest[:bytesRead]), 0
	}

	if handle.cached && handle.cachePath != "" {
		result, errno := n.readFromCache(handle.cachePath, dest, off)
		if errno == 0 {
			n.fsys.stats.BytesFromCache.Add(int64(len(dest)))
		}
		return result, errno
	}

	return n.readRange(ctx, dest, off)
}

// Getxattr returns extended attribute value.
func (n *FruitNode) Getxattr(ctx context.Context, attr string, dest []byte) (uint32, syscall.Errno) {
	var value string

	switch attr {
	case "user.fruitsalade.cached":
		fileID := n.getFileID()
		if _, ok := n.fsys.cache.Get(fileID); ok {
			value = "true"
		} else {
			value = "false"
		}
	case "user.fruitsalade.size":
		value = fmt.Sprintf("%d", n.metadata.Size)
	case "user.fruitsalade.path":
		value = n.metadata.Path
	case "user.fruitsalade.id":
		value = n.metadata.ID
	case "user.fruitsalade.hash":
		value = n.metadata.Hash
	case "user.fruitsalade.online":
		if n.fsys.client.IsOnline() {
			value = "true"
		} else {
			value = "false"
		}
	default:
		return 0, syscall.ENODATA
	}

	if len(dest) == 0 {
		return uint32(len(value)), 0
	}

	if len(dest) < len(value) {
		return 0, syscall.ERANGE
	}

	copy(dest, value)
	return uint32(len(value)), 0
}

// Listxattr lists extended attributes.
func (n *FruitNode) Listxattr(ctx context.Context, dest []byte) (uint32, syscall.Errno) {
	attrs := []string{
		"user.fruitsalade.cached",
		"user.fruitsalade.size",
		"user.fruitsalade.path",
		"user.fruitsalade.id",
		"user.fruitsalade.hash",
		"user.fruitsalade.online",
	}

	var total int
	for _, attr := range attrs {
		total += len(attr) + 1
	}

	if len(dest) == 0 {
		return uint32(total), 0
	}

	if len(dest) < total {
		return 0, syscall.ERANGE
	}

	offset := 0
	for _, attr := range attrs {
		copy(dest[offset:], attr)
		offset += len(attr)
		dest[offset] = 0
		offset++
	}

	return uint32(total), 0
}

func (n *FruitNode) readFromCache(cachePath string, dest []byte, off int64) (gofuse.ReadResult, syscall.Errno) {
	f, err := os.Open(cachePath)
	if err != nil {
		return nil, syscall.EIO
	}
	defer f.Close()

	_, err = f.Seek(off, io.SeekStart)
	if err != nil {
		return nil, syscall.EIO
	}

	bytesRead, err := f.Read(dest)
	if err != nil && err != io.EOF {
		return nil, syscall.EIO
	}

	return gofuse.ReadResultData(dest[:bytesRead]), 0
}

func (n *FruitNode) readRange(ctx context.Context, dest []byte, off int64) (gofuse.ReadResult, syscall.Errno) {
	if !n.fsys.client.IsOnline() {
		logger.Error("Cannot read %s: server offline (range read)", n.metadata.Path)
		n.fsys.stats.OfflineErrors.Add(1)
		return nil, syscall.ENETUNREACH
	}

	fileID := strings.TrimPrefix(n.metadata.ID, "/")

	end := off + int64(len(dest)) - 1
	if end >= n.metadata.Size {
		end = n.metadata.Size - 1
	}
	length := end - off + 1

	logger.Debug("Range read: %s bytes=%d-%d", n.metadata.Path, off, end)

	reader, _, err := n.fsys.client.FetchContent(ctx, fileID, off, length)
	if err != nil {
		logger.Error("Range read error: %v", err)
		n.fsys.stats.FailedFetches.Add(1)
		return nil, syscall.EIO
	}
	defer reader.Close()

	bytesRead, err := io.ReadFull(reader, dest)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		logger.Error("Range read error: %v", err)
		return nil, syscall.EIO
	}

	n.fsys.stats.RangeReads.Add(1)
	n.fsys.stats.BytesDownloaded.Add(int64(bytesRead))

	return gofuse.ReadResultData(dest[:bytesRead]), 0
}

func (n *FruitNode) getFileID() string {
	return strings.ReplaceAll(n.metadata.ID, "/", "_")
}

func (n *FruitNode) fetchFullContent(ctx context.Context) (string, error) {
	fileID := strings.TrimPrefix(n.metadata.ID, "/")

	reader, _, err := n.fsys.client.FetchContentFull(ctx, fileID)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	var hashReader io.Reader = reader
	var hasher hash.Hash
	if n.fsys.cfg.VerifyHash && n.metadata.Hash != "" {
		hasher = sha256.New()
		hashReader = io.TeeReader(reader, hasher)
	}

	cacheID := n.getFileID()
	cachePath, err := n.fsys.cache.Put(cacheID, hashReader, n.metadata.Size)
	if err != nil {
		return "", err
	}

	if hasher != nil && n.metadata.Hash != "" {
		actualHash := hex.EncodeToString(hasher.Sum(nil))
		if actualHash != n.metadata.Hash {
			n.fsys.cache.Evict(cacheID)
			return "", fmt.Errorf("hash mismatch: expected %s, got %s", n.metadata.Hash, actualHash)
		}
		logger.Debug("Hash verified: %s", n.metadata.Path)
	}

	n.fsys.stats.BytesDownloaded.Add(n.metadata.Size)

	return cachePath, nil
}

// FileHandle represents an open file.
type FileHandle struct {
	node      *FruitNode
	cachePath string
	cached    bool

	// Write support
	mu       sync.Mutex
	writable bool
	dirty    bool
	tmpFile  *os.File
	size     int64
}

var _ fs.FileHandle = (*FileHandle)(nil)
var _ fs.FileWriter = (*FileHandle)(nil)
var _ fs.FileFlusher = (*FileHandle)(nil)
var _ fs.FileReleaser = (*FileHandle)(nil)

// openForWrite prepares a file for writing with a temp file buffer.
func (n *FruitNode) openForWrite(ctx context.Context, truncate bool) (fs.FileHandle, uint32, syscall.Errno) {
	tmpFile, err := os.CreateTemp(n.fsys.cfg.CacheDir, "fruitsalade-write-*")
	if err != nil {
		logger.Error("Failed to create temp file: %v", err)
		return nil, 0, syscall.EIO
	}

	var size int64

	// If not truncating, pre-load existing content
	if !truncate && n.metadata.Size > 0 {
		fileID := n.getFileID()
		if cachePath, ok := n.fsys.cache.Get(fileID); ok {
			src, err := os.Open(cachePath)
			if err == nil {
				size, _ = io.Copy(tmpFile, src)
				src.Close()
				tmpFile.Seek(0, io.SeekStart)
			}
		} else if n.fsys.client.IsOnline() {
			serverID := strings.TrimPrefix(n.metadata.ID, "/")
			reader, _, err := n.fsys.client.FetchContentFull(ctx, serverID)
			if err == nil {
				size, _ = io.Copy(tmpFile, reader)
				reader.Close()
				tmpFile.Seek(0, io.SeekStart)
			}
		}
	}

	return &FileHandle{
		node:     n,
		writable: true,
		tmpFile:  tmpFile,
		size:     size,
	}, 0, 0
}

// Create creates a new file.
func (n *FruitNode) Create(ctx context.Context, name string, flags uint32, mode uint32, out *gofuse.EntryOut) (*fs.Inode, fs.FileHandle, uint32, syscall.Errno) {
	if n.metadata == nil || !n.metadata.IsDir {
		return nil, nil, 0, syscall.ENOTDIR
	}

	n.fsys.mu.RLock()
	for _, child := range n.metadata.Children {
		if child.Name == name {
			n.fsys.mu.RUnlock()
			return nil, nil, 0, syscall.EEXIST
		}
	}
	n.fsys.mu.RUnlock()

	now := time.Now()
	path := buildChildPath(n.metadata.Path, name)

	childMeta := &models.FileNode{
		ID:      path,
		Name:    name,
		Path:    path,
		Size:    0,
		ModTime: now,
		IsDir:   false,
	}

	tmpFile, err := os.CreateTemp(n.fsys.cfg.CacheDir, "fruitsalade-write-*")
	if err != nil {
		logger.Error("Failed to create temp file: %v", err)
		return nil, nil, 0, syscall.EIO
	}

	n.fsys.mu.Lock()
	n.metadata.Children = append(n.metadata.Children, childMeta)
	n.fsys.mu.Unlock()

	childNode := &FruitNode{
		fsys:     n.fsys,
		metadata: childMeta,
	}

	out.Mode = 0644 | syscall.S_IFREG
	out.Size = 0
	out.Mtime = uint64(now.Unix())
	out.Atime = out.Mtime
	out.Ctime = out.Mtime
	out.Uid = uint32(os.Getuid())
	out.Gid = uint32(os.Getgid())

	stableAttr := fs.StableAttr{Mode: out.Mode}
	inode := n.NewInode(ctx, childNode, stableAttr)

	fh := &FileHandle{
		node:     childNode,
		writable: true,
		dirty:    true,
		tmpFile:  tmpFile,
	}

	n.fsys.stats.FilesCreated.Add(1)
	logger.Info("Created file: %s", path)

	return inode, fh, 0, 0
}

// Mkdir creates a new directory.
func (n *FruitNode) Mkdir(ctx context.Context, name string, mode uint32, out *gofuse.EntryOut) (*fs.Inode, syscall.Errno) {
	if n.metadata == nil || !n.metadata.IsDir {
		return nil, syscall.ENOTDIR
	}

	n.fsys.mu.RLock()
	for _, child := range n.metadata.Children {
		if child.Name == name {
			n.fsys.mu.RUnlock()
			return nil, syscall.EEXIST
		}
	}
	n.fsys.mu.RUnlock()

	path := buildChildPath(n.metadata.Path, name)
	serverPath := strings.TrimPrefix(path, "/")

	if err := n.fsys.client.CreateDirectory(ctx, serverPath); err != nil {
		logger.Error("Mkdir failed for %s: %v", path, err)
		return nil, syscall.EIO
	}

	now := time.Now()
	childMeta := &models.FileNode{
		ID:      path,
		Name:    name,
		Path:    path,
		IsDir:   true,
		ModTime: now,
	}

	n.fsys.mu.Lock()
	n.metadata.Children = append(n.metadata.Children, childMeta)
	n.fsys.mu.Unlock()

	childNode := &FruitNode{
		fsys:     n.fsys,
		metadata: childMeta,
	}

	out.Mode = 0755 | syscall.S_IFDIR
	out.Mtime = uint64(now.Unix())
	out.Atime = out.Mtime
	out.Ctime = out.Mtime
	out.Uid = uint32(os.Getuid())
	out.Gid = uint32(os.Getgid())

	stableAttr := fs.StableAttr{Mode: out.Mode}
	n.fsys.stats.DirsCreated.Add(1)
	logger.Info("Created directory: %s", path)

	return n.NewInode(ctx, childNode, stableAttr), 0
}

// Unlink removes a file.
func (n *FruitNode) Unlink(ctx context.Context, name string) syscall.Errno {
	if n.metadata == nil || !n.metadata.IsDir {
		return syscall.ENOTDIR
	}

	n.fsys.mu.RLock()
	var target *models.FileNode
	for _, child := range n.metadata.Children {
		if child.Name == name {
			target = child
			break
		}
	}
	n.fsys.mu.RUnlock()

	if target == nil {
		return syscall.ENOENT
	}
	if target.IsDir {
		return syscall.EISDIR
	}

	serverPath := strings.TrimPrefix(target.Path, "/")
	if err := n.fsys.client.DeletePath(ctx, serverPath); err != nil {
		logger.Error("Delete failed for %s: %v", target.Path, err)
		return syscall.EIO
	}

	cacheID := strings.ReplaceAll(target.ID, "/", "_")
	n.fsys.cache.Evict(cacheID)

	n.fsys.mu.Lock()
	n.removeChildLocked(name)
	n.fsys.mu.Unlock()

	n.fsys.stats.FilesDeleted.Add(1)
	logger.Info("Deleted file: %s", target.Path)
	return 0
}

// Rmdir removes an empty directory.
func (n *FruitNode) Rmdir(ctx context.Context, name string) syscall.Errno {
	if n.metadata == nil || !n.metadata.IsDir {
		return syscall.ENOTDIR
	}

	n.fsys.mu.RLock()
	var target *models.FileNode
	for _, child := range n.metadata.Children {
		if child.Name == name {
			target = child
			break
		}
	}
	n.fsys.mu.RUnlock()

	if target == nil {
		return syscall.ENOENT
	}
	if !target.IsDir {
		return syscall.ENOTDIR
	}
	if len(target.Children) > 0 {
		return syscall.ENOTEMPTY
	}

	serverPath := strings.TrimPrefix(target.Path, "/")
	if err := n.fsys.client.DeletePath(ctx, serverPath); err != nil {
		logger.Error("Rmdir failed for %s: %v", target.Path, err)
		return syscall.EIO
	}

	n.fsys.mu.Lock()
	n.removeChildLocked(name)
	n.fsys.mu.Unlock()

	n.fsys.stats.DirsDeleted.Add(1)
	logger.Info("Removed directory: %s", target.Path)
	return 0
}

// Setattr sets file attributes (handles truncate and mtime changes).
func (n *FruitNode) Setattr(ctx context.Context, f fs.FileHandle, in *gofuse.SetAttrIn, out *gofuse.AttrOut) syscall.Errno {
	if n.metadata == nil {
		return syscall.ENOENT
	}

	if sz, ok := in.GetSize(); ok {
		n.fsys.mu.Lock()
		n.metadata.Size = int64(sz)
		n.fsys.mu.Unlock()

		if fh, ok := f.(*FileHandle); ok && fh.tmpFile != nil {
			fh.mu.Lock()
			fh.tmpFile.Truncate(int64(sz))
			fh.size = int64(sz)
			fh.dirty = true
			fh.mu.Unlock()
		}
	}

	if mtime, ok := in.GetMTime(); ok {
		n.fsys.mu.Lock()
		n.metadata.ModTime = mtime
		n.fsys.mu.Unlock()
	}

	return n.Getattr(ctx, f, out)
}

// Rename moves a file or directory.
func (n *FruitNode) Rename(ctx context.Context, name string, newParent fs.InodeEmbedder, newName string, flags uint32) syscall.Errno {
	n.fsys.mu.RLock()
	var source *models.FileNode
	for _, child := range n.metadata.Children {
		if child.Name == name {
			source = child
			break
		}
	}
	n.fsys.mu.RUnlock()

	if source == nil {
		return syscall.ENOENT
	}

	newParentNode, ok := newParent.(*FruitNode)
	if !ok {
		return syscall.EIO
	}

	// Check RENAME_NOREPLACE
	if flags&1 != 0 {
		n.fsys.mu.RLock()
		for _, child := range newParentNode.metadata.Children {
			if child.Name == newName {
				n.fsys.mu.RUnlock()
				return syscall.EEXIST
			}
		}
		n.fsys.mu.RUnlock()
	}

	newPath := buildChildPath(newParentNode.metadata.Path, newName)

	if source.IsDir {
		if len(source.Children) > 0 {
			logger.Error("Rename of non-empty directory not supported: %s", source.Path)
			return syscall.ENOTSUP
		}
		serverNewPath := strings.TrimPrefix(newPath, "/")
		if err := n.fsys.client.CreateDirectory(ctx, serverNewPath); err != nil {
			logger.Error("Rename create dir failed: %v", err)
			return syscall.EIO
		}
		serverOldPath := strings.TrimPrefix(source.Path, "/")
		n.fsys.client.DeletePath(ctx, serverOldPath)
	} else {
		// For files: read content, upload under new path, delete old
		var content io.ReadCloser
		var size int64

		cacheID := strings.ReplaceAll(source.ID, "/", "_")
		if cachePath, ok := n.fsys.cache.Get(cacheID); ok {
			f, err := os.Open(cachePath)
			if err != nil {
				return syscall.EIO
			}
			info, _ := f.Stat()
			content = f
			size = info.Size()
		} else if n.fsys.client.IsOnline() {
			serverID := strings.TrimPrefix(source.ID, "/")
			var err error
			content, size, err = n.fsys.client.FetchContentFull(ctx, serverID)
			if err != nil {
				logger.Error("Rename fetch failed: %v", err)
				return syscall.EIO
			}
		} else {
			return syscall.ENETUNREACH
		}
		defer content.Close()

		serverNewPath := strings.TrimPrefix(newPath, "/")
		if _, err := n.fsys.client.UploadFile(ctx, serverNewPath, content, size); err != nil {
			logger.Error("Rename upload failed: %v", err)
			return syscall.EIO
		}

		serverOldPath := strings.TrimPrefix(source.Path, "/")
		n.fsys.client.DeletePath(ctx, serverOldPath)
		n.fsys.cache.Evict(cacheID)
	}

	// Update local metadata tree
	n.fsys.mu.Lock()
	newParentNode.removeChildLocked(newName) // Remove existing target if any
	n.removeChildLocked(name)
	source.Name = newName
	source.Path = newPath
	source.ID = newPath
	newParentNode.metadata.Children = append(newParentNode.metadata.Children, source)
	n.fsys.mu.Unlock()

	n.fsys.stats.Renames.Add(1)
	logger.Info("Renamed: %s -> %s", name, newPath)
	return 0
}

// Write writes data to the file buffer.
func (fh *FileHandle) Write(ctx context.Context, data []byte, off int64) (uint32, syscall.Errno) {
	fh.mu.Lock()
	defer fh.mu.Unlock()

	if fh.tmpFile == nil {
		return 0, syscall.EIO
	}

	n, err := fh.tmpFile.WriteAt(data, off)
	if err != nil {
		logger.Error("Write error at offset %d: %v", off, err)
		return 0, syscall.EIO
	}

	end := off + int64(n)
	if end > fh.size {
		fh.size = end
	}
	fh.dirty = true

	return uint32(n), 0
}

// Flush uploads buffered content to the server.
func (fh *FileHandle) Flush(ctx context.Context) syscall.Errno {
	fh.mu.Lock()
	defer fh.mu.Unlock()

	if !fh.dirty || fh.tmpFile == nil {
		return 0
	}

	if _, err := fh.tmpFile.Seek(0, io.SeekStart); err != nil {
		logger.Error("Flush seek error: %v", err)
		return syscall.EIO
	}

	path := strings.TrimPrefix(fh.node.metadata.Path, "/")
	resp, err := fh.node.fsys.client.UploadFile(ctx, path, fh.tmpFile, fh.size)
	if err != nil {
		logger.Error("Upload failed for %s: %v", fh.node.metadata.Path, err)
		return syscall.EIO
	}

	// Update metadata with server response
	fh.node.fsys.mu.Lock()
	fh.node.metadata.Size = resp.Size
	fh.node.metadata.Hash = resp.Hash
	fh.node.metadata.ModTime = time.Now()
	fh.node.fsys.mu.Unlock()

	// Update cache with the written content
	if _, err := fh.tmpFile.Seek(0, io.SeekStart); err == nil {
		cacheID := fh.node.getFileID()
		if cachePath, err := fh.node.fsys.cache.Put(cacheID, fh.tmpFile, fh.size); err == nil {
			fh.cachePath = cachePath
			fh.cached = true
		}
	}

	fh.dirty = false
	fh.node.fsys.stats.BytesUploaded.Add(fh.size)
	logger.Info("Uploaded: %s (%d bytes)", fh.node.metadata.Path, fh.size)

	return 0
}

// Release cleans up the file handle.
func (fh *FileHandle) Release(ctx context.Context) syscall.Errno {
	fh.mu.Lock()
	defer fh.mu.Unlock()

	if fh.tmpFile != nil {
		name := fh.tmpFile.Name()
		fh.tmpFile.Close()
		os.Remove(name)
		fh.tmpFile = nil
	}

	return 0
}

// removeChildLocked removes a child by name. Must be called with fsys.mu held.
func (n *FruitNode) removeChildLocked(name string) {
	children := n.metadata.Children
	for i, child := range children {
		if child.Name == name {
			n.metadata.Children = append(children[:i], children[i+1:]...)
			return
		}
	}
}

func buildChildPath(parentPath, name string) string {
	if parentPath == "/" {
		return "/" + name
	}
	return parentPath + "/" + name
}
