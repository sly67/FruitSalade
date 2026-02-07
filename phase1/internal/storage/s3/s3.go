// Package s3 provides an S3-compatible storage backend.
package s3

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/fruitsalade/fruitsalade/phase1/internal/metadata/postgres"
	"github.com/fruitsalade/fruitsalade/shared/pkg/logger"
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
		logger.Error("Bucket check failed: %v", err)
	}

	return store, nil
}

func (s *Storage) ensureBucket(ctx context.Context) error {
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(s.bucket),
	})
	if err != nil {
		// Try to create
		_, createErr := s.client.CreateBucket(ctx, &s3.CreateBucketInput{
			Bucket: aws.String(s.bucket),
		})
		if createErr != nil {
			return fmt.Errorf("bucket %s does not exist and cannot create: %w", s.bucket, createErr)
		}
		logger.Info("Created S3 bucket: %s", s.bucket)
	}
	return nil
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
	// Look up S3 key from metadata
	s3Key, err := s.metadata.GetS3Key(ctx, fileID)
	if err != nil {
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
		return nil, 0, fmt.Errorf("get object %s: %w", s3Key, err)
	}

	totalSize := result.ContentLength
	if totalSize == nil {
		size := int64(0)
		totalSize = &size
	}

	return result.Body, *totalSize, nil
}

// GetContentSize returns the total file size from metadata.
func (s *Storage) GetContentSize(ctx context.Context, fileID string) (int64, error) {
	return s.metadata.GetFileSize(ctx, fileID)
}

// PutObject uploads content to S3 and records metadata.
func (s *Storage) PutObject(ctx context.Context, key string, body io.Reader, size int64) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          body,
		ContentLength: aws.Int64(size),
	})
	if err != nil {
		return fmt.Errorf("put object %s: %w", key, err)
	}
	return nil
}

// DeleteObject removes an object from S3.
func (s *Storage) DeleteObject(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	return err
}
