package winclient

import (
	"context"
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fruitsalade/fruitsalade/shared/pkg/logger"
	"github.com/fruitsalade/fruitsalade/shared/pkg/models"
	"github.com/fruitsalade/fruitsalade/shared/pkg/tree"
	"github.com/winfsp/cgofuse/fuse"
)

// CgoFuseBackend implements Backend using cgofuse (cross-platform FUSE via WinFSP).
type CgoFuseBackend struct {
	core      *ClientCore
	host      *fuse.FileSystemHost
	mountPath string
	ctx       context.Context
	cancel    context.CancelFunc

	mu      sync.Mutex
	handles map[uint64]*openHandle
	nextFh  atomic.Uint64
}

type openHandle struct {
	node     *models.FileNode
	cached   bool
	path     string // cache path or temp file path
	writable bool
	dirty    bool
	tmpFile  *os.File
	size     int64
	mu       sync.Mutex
}

// NewCgoFuseBackend creates a new cgofuse backend.
func NewCgoFuseBackend(mountPath string) *CgoFuseBackend {
	ctx, cancel := context.WithCancel(context.Background())
	return &CgoFuseBackend{
		mountPath: mountPath,
		handles:   make(map[uint64]*openHandle),
		ctx:       ctx,
		cancel:    cancel,
	}
}

func (b *CgoFuseBackend) Name() string {
	return "cgofuse"
}

func (b *CgoFuseBackend) Start(ctx context.Context, core *ClientCore) error {
	b.core = core

	if err := os.MkdirAll(b.mountPath, 0755); err != nil {
		return err
	}

	b.host = fuse.NewFileSystemHost(b)
	b.host.SetCapReaddirPlus(false)

	core.StartBackgroundLoops(ctx)

	logger.Info("Mounting cgofuse filesystem at %s", b.mountPath)

	// Mount in a goroutine; host.Mount blocks until unmounted
	errCh := make(chan error, 1)
	go func() {
		ok := b.host.Mount(b.mountPath, nil)
		if !ok {
			errCh <- fuse.Error(-1)
		} else {
			errCh <- nil
		}
	}()

	select {
	case err := <-errCh:
		core.StopBackgroundLoops()
		return err
	case <-ctx.Done():
		b.host.Unmount()
		core.StopBackgroundLoops()
		return ctx.Err()
	}
}

func (b *CgoFuseBackend) Stop() error {
	if b.host != nil {
		b.host.Unmount()
	}
	if b.core != nil {
		b.core.StopBackgroundLoops()
	}
	return nil
}

// allocFh allocates a file handle.
func (b *CgoFuseBackend) allocFh(h *openHandle) uint64 {
	fh := b.nextFh.Add(1)
	b.mu.Lock()
	b.handles[fh] = h
	b.mu.Unlock()
	return fh
}

func (b *CgoFuseBackend) getFh(fh uint64) *openHandle {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.handles[fh]
}

func (b *CgoFuseBackend) freeFh(fh uint64) *openHandle {
	b.mu.Lock()
	h := b.handles[fh]
	delete(b.handles, fh)
	b.mu.Unlock()
	return h
}

// resolvePath converts a FUSE path to the metadata path.
func resolvePath(fusePathRaw string) string {
	if fusePathRaw == "/" {
		return "/"
	}
	return fusePathRaw
}

func nodeToStat(node *models.FileNode, stat *fuse.Stat_t) {
	stat.Size = node.Size
	mt := fuse.NewTimespec(node.ModTime)
	stat.Mtim = mt
	stat.Atim = mt
	stat.Ctim = mt
	if node.IsDir {
		stat.Mode = fuse.S_IFDIR | 0755
		stat.Nlink = 2
	} else {
		stat.Mode = fuse.S_IFREG | 0644
		stat.Nlink = 1
	}
	stat.Uid = uint32(os.Getuid())
	stat.Gid = uint32(os.Getgid())
}

// --- fuse.FileSystemInterface implementation ---

func (b *CgoFuseBackend) Init() {
	logger.Info("cgofuse: Init")
}

func (b *CgoFuseBackend) Destroy() {
	logger.Info("cgofuse: Destroy")
	b.cancel()
}

func (b *CgoFuseBackend) Getattr(path string, stat *fuse.Stat_t, fh uint64) int {
	node := b.core.FindByPath(resolvePath(path))
	if node == nil {
		return -fuse.ENOENT
	}
	nodeToStat(node, stat)
	return 0
}

func (b *CgoFuseBackend) Readdir(path string, fill func(name string, stat *fuse.Stat_t, ofst int64) bool, ofst int64, fh uint64) int {
	node := b.core.FindByPath(resolvePath(path))
	if node == nil {
		return -fuse.ENOENT
	}
	if !node.IsDir {
		return -fuse.ENOTDIR
	}

	fill(".", nil, 0)
	fill("..", nil, 0)
	for _, child := range node.Children {
		var st fuse.Stat_t
		nodeToStat(child, &st)
		if !fill(child.Name, &st, 0) {
			break
		}
	}
	return 0
}

func (b *CgoFuseBackend) Open(path string, flags int) (int, uint64) {
	node := b.core.FindByPath(resolvePath(path))
	if node == nil {
		return -fuse.ENOENT, ^uint64(0)
	}
	if node.IsDir {
		return -fuse.EISDIR, ^uint64(0)
	}

	// Write mode
	if flags&(os.O_WRONLY|os.O_RDWR) != 0 {
		return b.openForWrite(node, flags&os.O_TRUNC != 0)
	}

	// Read mode: fetch content via core
	ctx := b.ctx
	cachePath, err := b.core.FetchContent(ctx, node)
	if err != nil {
		logger.Error("Open fetch failed for %s: %v", node.Path, err)
		if !b.core.IsOnline() {
			return -fuse.ENETUNREACH, ^uint64(0)
		}
		return -fuse.EIO, ^uint64(0)
	}

	fh := b.allocFh(&openHandle{
		node:   node,
		cached: true,
		path:   cachePath,
	})
	return 0, fh
}

func (b *CgoFuseBackend) openForWrite(node *models.FileNode, truncate bool) (int, uint64) {
	tmpFile, err := os.CreateTemp(b.core.Config.CacheDir, "fruitsalade-write-*")
	if err != nil {
		logger.Error("Failed to create temp file: %v", err)
		return -fuse.EIO, ^uint64(0)
	}

	var size int64
	if !truncate && node.Size > 0 {
		fileID := tree.CacheID(node.ID)
		if cachePath, ok := b.core.Cache.Get(fileID); ok {
			src, err := os.Open(cachePath)
			if err == nil {
				size, _ = io.Copy(tmpFile, src)
				src.Close()
				tmpFile.Seek(0, io.SeekStart)
			}
		} else if b.core.IsOnline() {
			ctx := b.ctx
			serverID := strings.TrimPrefix(node.ID, "/")
			reader, _, err := b.core.Client.FetchContentFull(ctx, serverID)
			if err == nil {
				size, _ = io.Copy(tmpFile, reader)
				reader.Close()
				tmpFile.Seek(0, io.SeekStart)
			}
		}
	}

	fh := b.allocFh(&openHandle{
		node:     node,
		writable: true,
		tmpFile:  tmpFile,
		size:     size,
	})
	return 0, fh
}

func (b *CgoFuseBackend) Read(path string, buff []byte, ofst int64, fh uint64) int {
	h := b.getFh(fh)
	if h == nil {
		return -fuse.EIO
	}

	if h.writable && h.tmpFile != nil {
		h.mu.Lock()
		n, err := h.tmpFile.ReadAt(buff, ofst)
		h.mu.Unlock()
		if err != nil && err != io.EOF {
			return -fuse.EIO
		}
		return n
	}

	if h.cached && h.path != "" {
		f, err := os.Open(h.path)
		if err != nil {
			return -fuse.EIO
		}
		defer f.Close()

		n, err := f.ReadAt(buff, ofst)
		if err != nil && err != io.EOF {
			return -fuse.EIO
		}
		b.core.Stats.BytesFromCache.Add(int64(n))
		return n
	}

	// Range read for large uncached files
	ctx := b.ctx
	node := b.core.FindByPath(resolvePath(path))
	if node == nil {
		return -fuse.ENOENT
	}

	end := ofst + int64(len(buff)) - 1
	if end >= node.Size {
		end = node.Size - 1
	}
	length := end - ofst + 1
	if length <= 0 {
		return 0
	}

	data, err := b.core.FetchContentRange(ctx, node.ID, ofst, length)
	if err != nil {
		logger.Error("Range read error: %v", err)
		return -fuse.EIO
	}

	copy(buff, data)
	return len(data)
}

func (b *CgoFuseBackend) Write(path string, buff []byte, ofst int64, fh uint64) int {
	h := b.getFh(fh)
	if h == nil || h.tmpFile == nil {
		return -fuse.EIO
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	n, err := h.tmpFile.WriteAt(buff, ofst)
	if err != nil {
		logger.Error("Write error at offset %d: %v", ofst, err)
		return -fuse.EIO
	}

	end := ofst + int64(n)
	if end > h.size {
		h.size = end
	}
	h.dirty = true
	return n
}

func (b *CgoFuseBackend) Flush(path string, fh uint64) int {
	h := b.getFh(fh)
	if h == nil {
		return 0
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.dirty || h.tmpFile == nil {
		return 0
	}

	reader := io.NewSectionReader(h.tmpFile, 0, h.size)
	serverPath := strings.TrimPrefix(h.node.Path, "/")

	ctx := b.ctx
	resp, err := b.core.UploadReader(ctx, serverPath, reader, h.size)
	if err != nil {
		logger.Error("Upload failed for %s: %v", h.node.Path, err)
		return -fuse.EIO
	}

	b.core.UpdateMetadataNode(h.node.Path, resp.Size, resp.Hash, time.Now())

	// Update cache
	cacheReader := io.NewSectionReader(h.tmpFile, 0, h.size)
	fileID := tree.CacheID(h.node.ID)
	b.core.Cache.Put(fileID, cacheReader, h.size)

	h.dirty = false
	logger.Info("Uploaded: %s (%d bytes)", h.node.Path, h.size)
	return 0
}

func (b *CgoFuseBackend) Release(path string, fh uint64) int {
	h := b.freeFh(fh)
	if h == nil {
		return 0
	}

	if h.tmpFile != nil {
		name := h.tmpFile.Name()
		h.tmpFile.Close()
		os.Remove(name)
	}
	return 0
}

func (b *CgoFuseBackend) Create(path string, flags int, mode uint32) (int, uint64) {
	dir, name := splitPath(path)
	parent := b.core.FindByPath(resolvePath(dir))
	if parent == nil || !parent.IsDir {
		return -fuse.ENOENT, ^uint64(0)
	}

	now := time.Now()
	childPath := resolvePath(path)

	childMeta := &models.FileNode{
		ID:      childPath,
		Name:    name,
		Path:    childPath,
		Size:    0,
		ModTime: now,
		IsDir:   false,
	}

	tmpFile, err := os.CreateTemp(b.core.Config.CacheDir, "fruitsalade-write-*")
	if err != nil {
		return -fuse.EIO, ^uint64(0)
	}

	b.core.AddMetadataChild(resolvePath(dir), childMeta)

	fh := b.allocFh(&openHandle{
		node:     childMeta,
		writable: true,
		dirty:    false,
		tmpFile:  tmpFile,
	})

	b.core.Stats.FilesCreated.Add(1)
	logger.Info("Created file: %s", childPath)
	return 0, fh
}

func (b *CgoFuseBackend) Mkdir(path string, mode uint32) int {
	dir, name := splitPath(path)
	parent := b.core.FindByPath(resolvePath(dir))
	if parent == nil || !parent.IsDir {
		return -fuse.ENOENT
	}

	serverPath := strings.TrimPrefix(resolvePath(path), "/")
	ctx := b.ctx
	if err := b.core.CreateDirectory(ctx, serverPath); err != nil {
		logger.Error("Mkdir failed for %s: %v", path, err)
		return -fuse.EIO
	}

	now := time.Now()
	childMeta := &models.FileNode{
		ID:      resolvePath(path),
		Name:    name,
		Path:    resolvePath(path),
		IsDir:   true,
		ModTime: now,
	}

	b.core.AddMetadataChild(resolvePath(dir), childMeta)
	b.core.Stats.DirsCreated.Add(1)
	logger.Info("Created directory: %s", path)
	return 0
}

func (b *CgoFuseBackend) Unlink(path string) int {
	node := b.core.FindByPath(resolvePath(path))
	if node == nil {
		return -fuse.ENOENT
	}
	if node.IsDir {
		return -fuse.EISDIR
	}

	serverPath := strings.TrimPrefix(node.Path, "/")
	ctx := b.ctx
	if err := b.core.DeletePath(ctx, serverPath); err != nil {
		logger.Error("Delete failed for %s: %v", path, err)
		return -fuse.EIO
	}

	b.core.Cache.Evict(tree.CacheID(node.ID))

	dir, name := splitPath(path)
	b.core.RemoveMetadataChild(resolvePath(dir), name)
	b.core.Stats.FilesDeleted.Add(1)
	logger.Info("Deleted file: %s", path)
	return 0
}

func (b *CgoFuseBackend) Rmdir(path string) int {
	node := b.core.FindByPath(resolvePath(path))
	if node == nil {
		return -fuse.ENOENT
	}
	if !node.IsDir {
		return -fuse.ENOTDIR
	}
	if len(node.Children) > 0 {
		return -fuse.ENOTEMPTY
	}

	serverPath := strings.TrimPrefix(node.Path, "/")
	ctx := b.ctx
	if err := b.core.DeletePath(ctx, serverPath); err != nil {
		logger.Error("Rmdir failed for %s: %v", path, err)
		return -fuse.EIO
	}

	dir, name := splitPath(path)
	b.core.RemoveMetadataChild(resolvePath(dir), name)
	b.core.Stats.DirsDeleted.Add(1)
	logger.Info("Removed directory: %s", path)
	return 0
}

func (b *CgoFuseBackend) Rename(oldpath string, newpath string) int {
	oldNode := b.core.FindByPath(resolvePath(oldpath))
	if oldNode == nil {
		return -fuse.ENOENT
	}

	ctx := b.ctx
	newResolved := resolvePath(newpath)

	if oldNode.IsDir {
		if len(oldNode.Children) > 0 {
			logger.Error("Rename of non-empty directory not supported: %s", oldpath)
			return -fuse.ENOTSUP
		}
		serverNewPath := strings.TrimPrefix(newResolved, "/")
		if err := b.core.CreateDirectory(ctx, serverNewPath); err != nil {
			return -fuse.EIO
		}
		serverOldPath := strings.TrimPrefix(oldNode.Path, "/")
		b.core.DeletePath(ctx, serverOldPath)
	} else {
		// Fetch content, upload under new path, delete old
		cachePath, err := b.core.FetchContent(ctx, oldNode)
		if err != nil {
			return -fuse.EIO
		}

		serverNewPath := strings.TrimPrefix(newResolved, "/")
		if _, err := b.core.UploadFile(ctx, serverNewPath, cachePath); err != nil {
			return -fuse.EIO
		}

		serverOldPath := strings.TrimPrefix(oldNode.Path, "/")
		b.core.DeletePath(ctx, serverOldPath)
		b.core.Cache.Evict(tree.CacheID(oldNode.ID))
	}

	// Update metadata tree
	oldDir, oldName := splitPath(oldpath)
	newDir, newName := splitPath(newpath)

	b.core.RemoveMetadataChild(resolvePath(newDir), newName) // remove target if exists
	b.core.RemoveMetadataChild(resolvePath(oldDir), oldName)

	oldNode.Name = newName
	oldNode.Path = newResolved
	oldNode.ID = newResolved
	b.core.AddMetadataChild(resolvePath(newDir), oldNode)

	b.core.Stats.Renames.Add(1)
	logger.Info("Renamed: %s -> %s", oldpath, newpath)
	return 0
}

func (b *CgoFuseBackend) Truncate(path string, size int64, fh uint64) int {
	node := b.core.FindByPath(resolvePath(path))
	if node == nil {
		return -fuse.ENOENT
	}

	b.core.UpdateMetadataNode(node.Path, size, node.Hash, time.Now())

	if fh != ^uint64(0) {
		h := b.getFh(fh)
		if h != nil && h.tmpFile != nil {
			h.mu.Lock()
			h.tmpFile.Truncate(size)
			h.size = size
			h.dirty = true
			h.mu.Unlock()
		}
	}
	return 0
}

func (b *CgoFuseBackend) Utimens(path string, tmsp []fuse.Timespec) int {
	node := b.core.FindByPath(resolvePath(path))
	if node == nil {
		return -fuse.ENOENT
	}
	if len(tmsp) >= 2 {
		mt := time.Unix(tmsp[1].Sec, tmsp[1].Nsec)
		b.core.UpdateMetadataNode(node.Path, node.Size, node.Hash, mt)
	}
	return 0
}

func (b *CgoFuseBackend) Chmod(path string, mode uint32) int {
	return 0 // no-op
}

func (b *CgoFuseBackend) Chown(path string, uid uint32, gid uint32) int {
	return 0 // no-op
}

func (b *CgoFuseBackend) Access(path string, mask uint32) int {
	node := b.core.FindByPath(resolvePath(path))
	if node == nil {
		return -fuse.ENOENT
	}
	return 0
}

func (b *CgoFuseBackend) Opendir(path string) (int, uint64) {
	node := b.core.FindByPath(resolvePath(path))
	if node == nil {
		return -fuse.ENOENT, ^uint64(0)
	}
	if !node.IsDir {
		return -fuse.ENOTDIR, ^uint64(0)
	}
	return 0, 0
}

func (b *CgoFuseBackend) Releasedir(path string, fh uint64) int {
	return 0
}

func (b *CgoFuseBackend) Statfs(path string, stat *fuse.Statfs_t) int {
	// Report reasonable defaults
	stat.Bsize = 4096
	stat.Frsize = 4096
	stat.Blocks = uint64(b.core.Config.MaxCacheSize) / 4096
	stat.Bfree = stat.Blocks / 2
	stat.Bavail = stat.Bfree
	stat.Files = 1000000
	stat.Ffree = 999000
	stat.Namemax = 255
	return 0
}

func (b *CgoFuseBackend) Mknod(path string, mode uint32, dev uint64) int {
	return -fuse.ENOSYS
}

func (b *CgoFuseBackend) Link(oldpath string, newpath string) int {
	return -fuse.ENOSYS
}

func (b *CgoFuseBackend) Symlink(target string, newpath string) int {
	return -fuse.ENOSYS
}

func (b *CgoFuseBackend) Readlink(path string) (int, string) {
	return -fuse.ENOSYS, ""
}

func (b *CgoFuseBackend) Fsync(path string, datasync bool, fh uint64) int {
	return 0
}

func (b *CgoFuseBackend) Fsyncdir(path string, datasync bool, fh uint64) int {
	return 0
}

func (b *CgoFuseBackend) Setxattr(path string, name string, value []byte, flags int) int {
	return -fuse.ENOSYS
}

func (b *CgoFuseBackend) Getxattr(path string, name string) (int, []byte) {
	return -fuse.ENODATA, nil
}

func (b *CgoFuseBackend) Removexattr(path string, name string) int {
	return -fuse.ENOSYS
}

func (b *CgoFuseBackend) Listxattr(path string, fill func(name string) bool) int {
	return 0
}

// splitPath splits a path into directory and basename.
func splitPath(path string) (string, string) {
	if path == "/" {
		return "/", ""
	}
	path = strings.TrimSuffix(path, "/")
	idx := strings.LastIndex(path, "/")
	if idx <= 0 {
		return "/", path[1:]
	}
	return path[:idx], path[idx+1:]
}
