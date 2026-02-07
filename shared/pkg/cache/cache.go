// Package cache provides client-side file caching.
package cache

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fruitsalade/fruitsalade/shared/pkg/models"
)

// Cache manages locally cached files.
type Cache struct {
	dir     string
	maxSize int64 // Maximum cache size in bytes

	mu      sync.RWMutex
	entries map[string]*models.CacheEntry
	size    int64
}

// New creates a new cache.
func New(dir string, maxSize int64) (*Cache, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create cache dir: %w", err)
	}
	return &Cache{
		dir:     dir,
		maxSize: maxSize,
		entries: make(map[string]*models.CacheEntry),
	}, nil
}

// Get returns the local path if the file is cached.
func (c *Cache) Get(fileID string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[fileID]
	if !ok {
		return "", false
	}

	// Update last access time
	entry.LastAccess = time.Now()
	return entry.LocalPath, true
}

// Put stores a file in the cache.
// Content is written atomically (temp file then rename).
func (c *Cache) Put(fileID string, r io.Reader, size int64) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict if needed
	for c.size+size > c.maxSize {
		if !c.evictOldest() {
			break // Nothing to evict
		}
	}

	// Write to temp file
	localPath := filepath.Join(c.dir, fileID)
	tempPath := localPath + ".tmp"

	f, err := os.Create(tempPath)
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}

	written, err := io.Copy(f, r)
	f.Close()
	if err != nil {
		os.Remove(tempPath)
		return "", fmt.Errorf("write content: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempPath, localPath); err != nil {
		os.Remove(tempPath)
		return "", fmt.Errorf("rename temp file: %w", err)
	}

	c.entries[fileID] = &models.CacheEntry{
		FileID:     fileID,
		LocalPath:  localPath,
		Size:       written,
		LastAccess: time.Now(),
		Pinned:     false,
	}
	c.size += written

	return localPath, nil
}

// Evict removes a file from the cache.
func (c *Cache) Evict(fileID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[fileID]
	if !ok {
		return nil
	}

	if entry.Pinned {
		return fmt.Errorf("cannot evict pinned file: %s", fileID)
	}

	os.Remove(entry.LocalPath)
	c.size -= entry.Size
	delete(c.entries, fileID)
	return nil
}

// Pin marks a file to never be evicted.
func (c *Cache) Pin(fileID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[fileID]
	if !ok {
		return fmt.Errorf("file not cached: %s", fileID)
	}
	entry.Pinned = true
	return nil
}

// Unpin allows a file to be evicted.
func (c *Cache) Unpin(fileID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[fileID]
	if !ok {
		return fmt.Errorf("file not cached: %s", fileID)
	}
	entry.Pinned = false
	return nil
}

// evictOldest removes the oldest non-pinned file.
// Must be called with lock held.
func (c *Cache) evictOldest() bool {
	var oldest *models.CacheEntry
	var oldestID string

	for id, entry := range c.entries {
		if entry.Pinned {
			continue
		}
		if oldest == nil || entry.LastAccess.Before(oldest.LastAccess) {
			oldest = entry
			oldestID = id
		}
	}

	if oldest == nil {
		return false
	}

	os.Remove(oldest.LocalPath)
	c.size -= oldest.Size
	delete(c.entries, oldestID)
	return true
}

// Stats returns cache statistics.
func (c *Cache) Stats() (size, maxSize int64, count int) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.size, c.maxSize, len(c.entries)
}

// List returns all cached entries.
func (c *Cache) List() []*models.CacheEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entries := make([]*models.CacheEntry, 0, len(c.entries))
	for _, entry := range c.entries {
		entries = append(entries, entry)
	}
	return entries
}

// Pinned returns all pinned entries.
func (c *Cache) Pinned() []*models.CacheEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entries := make([]*models.CacheEntry, 0)
	for _, entry := range c.entries {
		if entry.Pinned {
			entries = append(entries, entry)
		}
	}
	return entries
}

// Clear removes all non-pinned files from the cache.
func (c *Cache) Clear() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	count := 0
	for id, entry := range c.entries {
		if entry.Pinned {
			continue
		}
		os.Remove(entry.LocalPath)
		c.size -= entry.Size
		delete(c.entries, id)
		count++
	}
	return count
}

// Dir returns the cache directory path.
func (c *Cache) Dir() string {
	return c.dir
}

// IsCached returns true if the file is cached.
func (c *Cache) IsCached(fileID string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.entries[fileID]
	return ok
}

// IsPinned returns true if the file is pinned.
func (c *Cache) IsPinned(fileID string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.entries[fileID]
	return ok && entry.Pinned
}
