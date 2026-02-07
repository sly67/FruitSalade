// Package storage provides the storage backend interface and implementations.
package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/fruitsalade/fruitsalade/shared/pkg/models"
)

// LocalStorage implements Storage using the local filesystem.
type LocalStorage struct {
	rootDir string
}

// NewLocalStorage creates a new local storage backend.
func NewLocalStorage(rootDir string) (*LocalStorage, error) {
	absPath, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("root directory error: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", absPath)
	}
	return &LocalStorage{rootDir: absPath}, nil
}

// BuildTree builds the complete metadata tree from the root directory.
func (s *LocalStorage) BuildTree(ctx context.Context) (*models.FileNode, error) {
	return s.buildNode(ctx, "")
}

func (s *LocalStorage) buildNode(ctx context.Context, relPath string) (*models.FileNode, error) {
	fullPath := filepath.Join(s.rootDir, relPath)
	info, err := os.Stat(fullPath)
	if err != nil {
		return nil, err
	}

	node := s.infoToNode(relPath, info)

	if info.IsDir() {
		entries, err := os.ReadDir(fullPath)
		if err != nil {
			return nil, err
		}

		node.Children = make([]*models.FileNode, 0, len(entries))
		for _, entry := range entries {
			// Skip hidden files
			if strings.HasPrefix(entry.Name(), ".") {
				continue
			}

			childPath := filepath.Join(relPath, entry.Name())
			childNode, err := s.buildNode(ctx, childPath)
			if err != nil {
				continue // Skip files we can't read
			}
			node.Children = append(node.Children, childNode)
		}
	}

	return node, nil
}

// GetMetadata returns metadata for a file or directory.
func (s *LocalStorage) GetMetadata(ctx context.Context, path string) (*models.FileNode, error) {
	// Normalize path
	path = strings.TrimPrefix(path, "/")
	fullPath := filepath.Join(s.rootDir, path)

	info, err := os.Stat(fullPath)
	if err != nil {
		return nil, err
	}
	return s.infoToNode(path, info), nil
}

// ListDir returns the contents of a directory.
func (s *LocalStorage) ListDir(ctx context.Context, path string) ([]*models.FileNode, error) {
	path = strings.TrimPrefix(path, "/")
	fullPath := filepath.Join(s.rootDir, path)

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, err
	}

	nodes := make([]*models.FileNode, 0, len(entries))
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}
		childPath := filepath.Join(path, entry.Name())
		nodes = append(nodes, s.infoToNode(childPath, info))
	}
	return nodes, nil
}

// GetContent returns a reader for file content with range support.
// Returns: reader, total file size, error
func (s *LocalStorage) GetContent(ctx context.Context, id string, offset, length int64) (io.ReadCloser, int64, error) {
	// ID is the path for Phase 0
	id = strings.TrimPrefix(id, "/")
	fullPath := filepath.Join(s.rootDir, id)

	f, err := os.Open(fullPath)
	if err != nil {
		return nil, 0, err
	}

	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, 0, err
	}

	if info.IsDir() {
		f.Close()
		return nil, 0, fmt.Errorf("cannot read directory")
	}

	totalSize := info.Size()

	// Seek to offset
	if offset > 0 {
		if offset >= totalSize {
			f.Close()
			return nil, totalSize, fmt.Errorf("offset beyond file size")
		}
		_, err = f.Seek(offset, io.SeekStart)
		if err != nil {
			f.Close()
			return nil, 0, err
		}
	}

	// Calculate read length
	if length < 0 {
		length = totalSize - offset
	} else if offset+length > totalSize {
		length = totalSize - offset
	}

	return &limitedReadCloser{f: f, remaining: length}, totalSize, nil
}

// GetContentSize returns the total size of a file.
func (s *LocalStorage) GetContentSize(ctx context.Context, id string) (int64, error) {
	id = strings.TrimPrefix(id, "/")
	fullPath := filepath.Join(s.rootDir, id)

	info, err := os.Stat(fullPath)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

func (s *LocalStorage) infoToNode(path string, info os.FileInfo) *models.FileNode {
	name := info.Name()
	if path == "" {
		name = "root"
	}

	return &models.FileNode{
		ID:      path, // Using path as ID for Phase 0
		Name:    name,
		Path:    "/" + path,
		Size:    info.Size(),
		ModTime: info.ModTime(),
		IsDir:   info.IsDir(),
		Hash:    "", // Computed on demand if needed
	}
}

// PathToID generates a stable ID from a path.
func PathToID(path string) string {
	h := sha256.Sum256([]byte(path))
	return hex.EncodeToString(h[:8])
}

// limitedReadCloser wraps a file with a read limit.
type limitedReadCloser struct {
	f         *os.File
	remaining int64
}

func (l *limitedReadCloser) Read(p []byte) (int, error) {
	if l.remaining <= 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > l.remaining {
		p = p[:l.remaining]
	}
	n, err := l.f.Read(p)
	l.remaining -= int64(n)
	return n, err
}

func (l *limitedReadCloser) Close() error {
	return l.f.Close()
}
