// Package s3 provides an S3-compatible storage backend with metrics.
package s3

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"go.uber.org/zap"

	"github.com/fruitsalade/fruitsalade/phase2/internal/logging"
	"github.com/fruitsalade/fruitsalade/phase2/internal/metadata/postgres"
	"github.com/fruitsalade/fruitsalade/phase2/internal/metrics"
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
type Storage struct {
	client   *s3.Client
	bucket   string
	metadata *postgres.Store
}

// New creates a new S3 storage backend.
func New(ctx context.Context, cfg Config, metadata *postgres.Store) (*Storage, error) {
	resolver := aws.EndpointResolverWithOptionsFunc(
		func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL:               cfg.Endpoint,
				HostnameImmutable: true,
			}, nil
		},
	)

	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(cfg.Region),
		config.WithEndpointResolverWithOptions(resolver),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = true // Required for MinIO
	})

	store := &Storage{
		client:   client,
		bucket:   cfg.Bucket,
		metadata: metadata,
	}

	// Verify bucket exists
	if err := store.ensureBucket(ctx); err != nil {
		logging.Error("bucket check failed", zap.Error(err))
	}

	return store, nil
}

func (s *Storage) ensureBucket(ctx context.Context) error {
	start := time.Now()
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(s.bucket),
	})
	if err != nil {
		// Try to create
		_, createErr := s.client.CreateBucket(ctx, &s3.CreateBucketInput{
			Bucket: aws.String(s.bucket),
		})
		if createErr != nil {
			metrics.RecordS3Operation("create_bucket", time.Since(start), false)
			return fmt.Errorf("bucket %s does not exist and cannot create: %w", s.bucket, createErr)
		}
		metrics.RecordS3Operation("create_bucket", time.Since(start), true)
		logging.Info("created S3 bucket", zap.String("bucket", s.bucket))
	}
	return nil
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
func (s *Storage) GetContent(ctx context.Context, fileID string, offset, length int64) (io.ReadCloser, int64, error) {
	start := time.Now()

	// Look up S3 key from metadata
	s3Key, err := s.metadata.GetS3Key(ctx, fileID)
	if err != nil {
		metrics.RecordS3Operation("get_object", time.Since(start), false)
		return nil, 0, fmt.Errorf("get s3 key: %w", err)
	}

	input := &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s3Key),
	}

	// Set range header if needed
	if offset > 0 || length > 0 {
		var rangeStr string
		if length > 0 {
			rangeStr = fmt.Sprintf("bytes=%d-%d", offset, offset+length-1)
		} else {
			rangeStr = fmt.Sprintf("bytes=%d-", offset)
		}
		input.Range = aws.String(rangeStr)
	}

	result, err := s.client.GetObject(ctx, input)
	if err != nil {
		metrics.RecordS3Operation("get_object", time.Since(start), false)
		return nil, 0, fmt.Errorf("get object %s: %w", s3Key, err)
	}

	metrics.RecordS3Operation("get_object", time.Since(start), true)

	totalSize := result.ContentLength
	if totalSize == nil {
		size := int64(0)
		totalSize = &size
	}

	logging.Debug("S3 get object",
		zap.String("key", s3Key),
		zap.Int64("offset", offset),
		zap.Int64("length", length),
		zap.Int64("size", *totalSize))

	return result.Body, *totalSize, nil
}

// GetContentSize returns the total file size from metadata.
func (s *Storage) GetContentSize(ctx context.Context, fileID string) (int64, error) {
	return s.metadata.GetFileSize(ctx, fileID)
}

// PutObject uploads content to S3.
func (s *Storage) PutObject(ctx context.Context, key string, body io.Reader, size int64) error {
	start := time.Now()

	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          body,
		ContentLength: aws.Int64(size),
	})
	if err != nil {
		metrics.RecordS3Operation("put_object", time.Since(start), false)
		metrics.RecordContentUpload(0, false)
		return fmt.Errorf("put object %s: %w", key, err)
	}

	metrics.RecordS3Operation("put_object", time.Since(start), true)
	metrics.RecordContentUpload(size, true)

	logging.Debug("S3 put object", zap.String("key", key), zap.Int64("size", size))
	return nil
}

// DeleteObject removes an object from S3.
func (s *Storage) DeleteObject(ctx context.Context, key string) error {
	start := time.Now()

	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		metrics.RecordS3Operation("delete_object", time.Since(start), false)
		return err
	}

	metrics.RecordS3Operation("delete_object", time.Since(start), true)
	logging.Debug("S3 delete object", zap.String("key", key))
	return nil
}

// ObjectExists checks if an object exists in S3.
func (s *Storage) ObjectExists(ctx context.Context, key string) (bool, error) {
	start := time.Now()

	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		// Check if it's a "not found" error
		metrics.RecordS3Operation("head_object", time.Since(start), false)
		return false, nil
	}

	metrics.RecordS3Operation("head_object", time.Since(start), true)
	return true, nil
}
