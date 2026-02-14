// Package local provides a local filesystem storage backend.
package local

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Config holds local filesystem backend settings.
type Config struct {
	RootPath   string `json:"root_path"`
	CreateDirs bool   `json:"create_dirs"`
}

// LocalBackend implements storage.Backend using the local filesystem.
type LocalBackend struct {
	rootPath   string
	createDirs bool
}

// New creates a new local filesystem backend.
func New(cfg Config) (*LocalBackend, error) {
	if cfg.RootPath == "" {
		return nil, fmt.Errorf("root_path is required")
	}

	// Ensure root exists
	info, err := os.Stat(cfg.RootPath)
	if err != nil {
		if os.IsNotExist(err) && cfg.CreateDirs {
			if mkErr := os.MkdirAll(cfg.RootPath, 0755); mkErr != nil {
				return nil, fmt.Errorf("create root path %s: %w", cfg.RootPath, mkErr)
			}
		} else {
			return nil, fmt.Errorf("stat root path %s: %w", cfg.RootPath, err)
		}
	} else if !info.IsDir() {
		return nil, fmt.Errorf("root path %s is not a directory", cfg.RootPath)
	}

	return &LocalBackend{
		rootPath:   cfg.RootPath,
		createDirs: cfg.CreateDirs,
	}, nil
}

// NewFromJSON creates a LocalBackend from raw JSON config.
func NewFromJSON(raw json.RawMessage) (*LocalBackend, error) {
	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parse local config: %w", err)
	}
	return New(cfg)
}

func (b *LocalBackend) fullPath(key string) string {
	return filepath.Join(b.rootPath, filepath.FromSlash(key))
}

// GetObject reads a file from the local filesystem with range support.
func (b *LocalBackend) GetObject(_ context.Context, key string, offset, length int64) (io.ReadCloser, int64, error) {
	path := b.fullPath(key)
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("open %s: %w", key, err)
	}

	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, 0, fmt.Errorf("stat %s: %w", key, err)
	}

	totalSize := info.Size()

	if offset > 0 {
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			f.Close()
			return nil, 0, fmt.Errorf("seek %s: %w", key, err)
		}
	}

	if length > 0 {
		return &limitedReadCloser{
			Reader: io.LimitReader(f, length),
			Closer: f,
		}, length, nil
	}

	returnSize := totalSize - offset
	if returnSize < 0 {
		returnSize = 0
	}
	return f, returnSize, nil
}

// PutObject writes content to the local filesystem atomically.
func (b *LocalBackend) PutObject(_ context.Context, key string, body io.Reader, size int64) error {
	path := b.fullPath(key)
	dir := filepath.Dir(path)

	if b.createDirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create dirs for %s: %w", key, err)
		}
	}

	// Write to temp file then rename for atomicity
	tmp, err := os.CreateTemp(dir, ".fruitsalade-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp for %s: %w", key, err)
	}
	tmpName := tmp.Name()

	if _, err := io.Copy(tmp, body); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write %s: %w", key, err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close temp for %s: %w", key, err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename temp to %s: %w", key, err)
	}

	return nil
}

// DeleteObject removes a file from the local filesystem.
func (b *LocalBackend) DeleteObject(_ context.Context, key string) error {
	path := b.fullPath(key)
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete %s: %w", key, err)
	}
	return nil
}

// CopyObject copies a file on the local filesystem.
func (b *LocalBackend) CopyObject(_ context.Context, srcKey, dstKey string) error {
	srcPath := b.fullPath(srcKey)
	dstPath := b.fullPath(dstKey)

	if b.createDirs {
		if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
			return fmt.Errorf("create dirs for %s: %w", dstKey, err)
		}
	}

	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open src %s: %w", srcKey, err)
	}
	defer src.Close()

	// Atomic write via temp + rename
	tmp, err := os.CreateTemp(filepath.Dir(dstPath), ".fruitsalade-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp for %s: %w", dstKey, err)
	}
	tmpName := tmp.Name()

	if _, err := io.Copy(tmp, src); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("copy %s -> %s: %w", srcKey, dstKey, err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close temp for %s: %w", dstKey, err)
	}

	if err := os.Rename(tmpName, dstPath); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename temp to %s: %w", dstKey, err)
	}

	return nil
}

// ObjectExists checks if a file exists on the local filesystem.
func (b *LocalBackend) ObjectExists(_ context.Context, key string) (bool, error) {
	path := b.fullPath(key)
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("stat %s: %w", key, err)
	}
	return true, nil
}

// Type returns "local".
func (b *LocalBackend) Type() string { return "local" }

// Close is a no-op for local backends.
func (b *LocalBackend) Close() error { return nil }

// limitedReadCloser wraps a LimitReader with a separate Closer.
type limitedReadCloser struct {
	io.Reader
	io.Closer
}
