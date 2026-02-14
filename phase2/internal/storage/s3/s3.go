// Package s3 provides an S3-compatible storage backend with metrics.
package s3

import (
	"context"
	"fmt"
	"io"

	"go.uber.org/zap"

	"github.com/fruitsalade/fruitsalade/phase2/internal/logging"
	"github.com/fruitsalade/fruitsalade/phase2/internal/metadata/postgres"
	"github.com/fruitsalade/fruitsalade/shared/pkg/models"
)

// Config holds S3 connection settings.
type Config struct {
	Endpoint  string
	Bucket    string
	AccessKey string
	SecretKey string
	Region    string
	UseSSL    bool
}

// Storage implements the storage backend using S3/MinIO.
// It wraps S3Backend (content I/O) and postgres.Store (metadata).
type Storage struct {
	*S3Backend
	metadata *postgres.Store
}

// New creates a new S3 storage backend.
func New(ctx context.Context, cfg Config, metadata *postgres.Store) (*Storage, error) {
	backend, err := NewBackend(ctx, BackendConfig{
		Endpoint:  cfg.Endpoint,
		Bucket:    cfg.Bucket,
		AccessKey: cfg.AccessKey,
		SecretKey: cfg.SecretKey,
		Region:    cfg.Region,
		UseSSL:    cfg.UseSSL,
	})
	if err != nil {
		return nil, err
	}

	return &Storage{
		S3Backend: backend,
		metadata:  metadata,
	}, nil
}

// Metadata returns the underlying metadata store.
func (s *Storage) Metadata() *postgres.Store {
	return s.metadata
}

// BuildTree builds the metadata tree from PostgreSQL.
func (s *Storage) BuildTree(ctx context.Context) (*models.FileNode, error) {
	return s.metadata.BuildTree(ctx)
}

// GetMetadata returns metadata for a path.
func (s *Storage) GetMetadata(ctx context.Context, path string) (*models.FileNode, error) {
	return s.metadata.GetMetadata(ctx, path)
}

// ListDir returns children of a directory.
func (s *Storage) ListDir(ctx context.Context, path string) ([]*models.FileNode, error) {
	return s.metadata.ListDir(ctx, path)
}

// GetContent returns file content from S3 with range support.
// Looks up the S3 key from metadata by file ID, then delegates to S3Backend.
func (s *Storage) GetContent(ctx context.Context, fileID string, offset, length int64) (io.ReadCloser, int64, error) {
	s3Key, err := s.metadata.GetS3Key(ctx, fileID)
	if err != nil {
		return nil, 0, fmt.Errorf("get s3 key: %w", err)
	}

	reader, size, err := s.S3Backend.GetObject(ctx, s3Key, offset, length)
	if err != nil {
		return nil, 0, err
	}

	logging.Debug("S3 get object",
		zap.String("key", s3Key),
		zap.Int64("offset", offset),
		zap.Int64("length", length),
		zap.Int64("size", size))

	return reader, size, nil
}

// GetContentSize returns the total file size from metadata.
func (s *Storage) GetContentSize(ctx context.Context, fileID string) (int64, error) {
	return s.metadata.GetFileSize(ctx, fileID)
}

// GetContentByS3Key gets content directly by S3 key (for version content).
func (s *Storage) GetContentByS3Key(ctx context.Context, s3Key string) (io.ReadCloser, int64, error) {
	return s.S3Backend.GetObject(ctx, s3Key, 0, 0)
}
