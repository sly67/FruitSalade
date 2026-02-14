// Package storage defines the Backend interface for content storage
// and provides multi-backend routing with per-group storage locations.
package storage

import (
	"context"
	"io"
)

// Backend is the interface for content storage backends.
// Implementations handle raw object I/O (S3, local filesystem, SMB mounts).
// Metadata (file tree, versions) is handled separately by postgres.Store.
type Backend interface {
	// GetObject retrieves an object by key with optional range support.
	// If offset=0 and length=0, the entire object is returned.
	GetObject(ctx context.Context, key string, offset, length int64) (io.ReadCloser, int64, error)

	// PutObject uploads content to the given key.
	PutObject(ctx context.Context, key string, body io.Reader, size int64) error

	// DeleteObject removes an object by key.
	DeleteObject(ctx context.Context, key string) error

	// CopyObject copies an object from srcKey to dstKey.
	CopyObject(ctx context.Context, srcKey, dstKey string) error

	// ObjectExists checks if an object exists at the given key.
	ObjectExists(ctx context.Context, key string) (bool, error)

	// Type returns the backend type identifier ("s3", "local", "smb").
	Type() string

	// Close releases any resources held by the backend.
	Close() error
}
