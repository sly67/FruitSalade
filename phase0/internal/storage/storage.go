// Package storage provides the storage backend interface and implementations.
package storage

import (
	"context"
	"io"

	"github.com/fruitsalade/fruitsalade/shared/pkg/models"
)

// Storage is the interface for file storage backends.
type Storage interface {
	// GetMetadata returns metadata for a file or directory.
	GetMetadata(ctx context.Context, path string) (*models.FileNode, error)

	// ListDir returns the contents of a directory.
	ListDir(ctx context.Context, path string) ([]*models.FileNode, error)

	// GetContent returns a reader for file content.
	// offset and length specify the byte range (-1 for length means full file).
	GetContent(ctx context.Context, id string, offset, length int64) (io.ReadCloser, int64, error)

	// GetContentSize returns the total size of a file.
	GetContentSize(ctx context.Context, id string) (int64, error)
}
