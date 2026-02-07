package cache

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCache_PutAndGet(t *testing.T) {
	dir := t.TempDir()
	c, err := New(dir, 1<<20)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	content := []byte("hello world")
	path, err := c.Put("test1", bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	if path == "" {
		t.Fatal("Put returned empty path")
	}

	// Verify file was written
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.Equal(data, content) {
		t.Errorf("content mismatch: got %q, want %q", data, content)
	}

	// Get should return the path
	gotPath, ok := c.Get("test1")
	if !ok {
		t.Fatal("Get returned not ok")
	}
	if gotPath != path {
		t.Errorf("Get path mismatch: got %q, want %q", gotPath, path)
	}
}

func TestCache_IsCached(t *testing.T) {
	dir := t.TempDir()
	c, err := New(dir, 1<<20)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if c.IsCached("nonexistent") {
		t.Error("IsCached returned true for nonexistent file")
	}

	content := []byte("test")
	_, err = c.Put("exists", bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	if !c.IsCached("exists") {
		t.Error("IsCached returned false for existing file")
	}
}

func TestCache_Evict(t *testing.T) {
	dir := t.TempDir()
	c, err := New(dir, 1<<20)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	content := []byte("test")
	path, _ := c.Put("evictme", bytes.NewReader(content), int64(len(content)))

	if err := c.Evict("evictme"); err != nil {
		t.Fatalf("Evict: %v", err)
	}

	if c.IsCached("evictme") {
		t.Error("file still cached after evict")
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("file still exists on disk after evict")
	}
}

func TestCache_PinUnpin(t *testing.T) {
	dir := t.TempDir()
	c, err := New(dir, 1<<20)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	content := []byte("pinme")
	c.Put("pinned", bytes.NewReader(content), int64(len(content)))

	if err := c.Pin("pinned"); err != nil {
		t.Fatalf("Pin: %v", err)
	}

	if !c.IsPinned("pinned") {
		t.Error("IsPinned returned false after Pin")
	}

	// Cannot evict pinned file
	if err := c.Evict("pinned"); err == nil {
		t.Error("Evict succeeded on pinned file")
	}

	if err := c.Unpin("pinned"); err != nil {
		t.Fatalf("Unpin: %v", err)
	}

	if c.IsPinned("pinned") {
		t.Error("IsPinned returned true after Unpin")
	}
}

func TestCache_LRUEviction(t *testing.T) {
	dir := t.TempDir()
	c, err := New(dir, 100) // Only 100 bytes
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Add file1 (30 bytes)
	content1 := make([]byte, 30)
	c.Put("file1", bytes.NewReader(content1), 30)
	time.Sleep(10 * time.Millisecond)

	// Add file2 (30 bytes) - total: 60
	content2 := make([]byte, 30)
	c.Put("file2", bytes.NewReader(content2), 30)
	time.Sleep(10 * time.Millisecond)

	// Access file1 to make it more recent
	c.Get("file1")

	// Add file3 (50 bytes) - total would be 110, should evict file2 (LRU)
	// After eviction: 30 + 50 = 80 <= 100
	content3 := make([]byte, 50)
	c.Put("file3", bytes.NewReader(content3), 50)

	// file2 should be evicted (oldest, not accessed recently)
	if c.IsCached("file2") {
		t.Error("file2 should have been evicted")
	}

	// file1 should still exist (accessed more recently)
	if !c.IsCached("file1") {
		t.Error("file1 should not have been evicted")
	}

	// file3 should exist
	if !c.IsCached("file3") {
		t.Error("file3 should be cached")
	}
}

func TestCache_Stats(t *testing.T) {
	dir := t.TempDir()
	maxSize := int64(1 << 20)
	c, err := New(dir, maxSize)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	size, max, count := c.Stats()
	if size != 0 || count != 0 {
		t.Errorf("initial stats wrong: size=%d, count=%d", size, count)
	}
	if max != maxSize {
		t.Errorf("max size wrong: got %d, want %d", max, maxSize)
	}

	content := make([]byte, 100)
	c.Put("stats1", bytes.NewReader(content), 100)

	size, _, count = c.Stats()
	if size != 100 || count != 1 {
		t.Errorf("after Put stats wrong: size=%d, count=%d", size, count)
	}
}

func TestCache_List(t *testing.T) {
	dir := t.TempDir()
	c, err := New(dir, 1<<20)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	c.Put("a", bytes.NewReader([]byte("a")), 1)
	c.Put("b", bytes.NewReader([]byte("bb")), 2)
	c.Put("c", bytes.NewReader([]byte("ccc")), 3)

	entries := c.List()
	if len(entries) != 3 {
		t.Errorf("List returned %d entries, want 3", len(entries))
	}
}

func TestCache_Clear(t *testing.T) {
	dir := t.TempDir()
	c, err := New(dir, 1<<20)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	c.Put("a", bytes.NewReader([]byte("a")), 1)
	c.Put("b", bytes.NewReader([]byte("b")), 1)
	c.Pin("b") // b is pinned

	cleared := c.Clear()
	if cleared != 1 {
		t.Errorf("Clear returned %d, want 1", cleared)
	}

	if c.IsCached("a") {
		t.Error("unpinned file not cleared")
	}
	if !c.IsCached("b") {
		t.Error("pinned file was cleared")
	}
}

func TestCache_Dir(t *testing.T) {
	dir := t.TempDir()
	c, err := New(dir, 1<<20)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if c.Dir() != dir {
		t.Errorf("Dir() = %q, want %q", c.Dir(), dir)
	}
}

func TestCache_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	c, err := New(dir, 1<<20)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Write a file
	content := []byte("atomic content")
	path, err := c.Put("atomic", bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	// Verify no .tmp file remains
	tmpPath := path + ".tmp"
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Error(".tmp file should not exist after Put")
	}

	// Verify the final file exists
	if _, err := os.Stat(path); err != nil {
		t.Errorf("final file should exist: %v", err)
	}
}

func TestCache_Pinned(t *testing.T) {
	dir := t.TempDir()
	c, err := New(dir, 1<<20)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	c.Put("a", bytes.NewReader([]byte("a")), 1)
	c.Put("b", bytes.NewReader([]byte("b")), 1)
	c.Put("c", bytes.NewReader([]byte("c")), 1)
	c.Pin("a")
	c.Pin("c")

	pinned := c.Pinned()
	if len(pinned) != 2 {
		t.Errorf("Pinned returned %d entries, want 2", len(pinned))
	}

	// Verify only pinned files are returned
	ids := make(map[string]bool)
	for _, e := range pinned {
		ids[e.FileID] = true
	}
	if !ids["a"] || !ids["c"] {
		t.Error("Pinned should return files a and c")
	}
	if ids["b"] {
		t.Error("Pinned should not return file b")
	}
}

func TestNew_CreatesDir(t *testing.T) {
	base := t.TempDir()
	cacheDir := filepath.Join(base, "subdir", "cache")

	c, err := New(cacheDir, 1<<20)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if c.Dir() != cacheDir {
		t.Errorf("Dir() = %q, want %q", c.Dir(), cacheDir)
	}

	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		t.Error("cache directory was not created")
	}
}
