package s3

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"go.uber.org/zap"

	"github.com/fruitsalade/fruitsalade/phase2/internal/logging"
	"github.com/fruitsalade/fruitsalade/phase2/internal/metrics"
)

// BackendConfig is a JSON-serializable config for S3 backends stored in the database.
type BackendConfig struct {
	Endpoint  string `json:"endpoint"`
	Bucket    string `json:"bucket"`
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
	Region    string `json:"region"`
	UseSSL    bool   `json:"use_ssl"`
}

// S3Backend implements storage.Backend using S3/MinIO.
type S3Backend struct {
	client *s3.Client
	bucket string
}

// NewBackend creates a new S3 backend from a BackendConfig.
func NewBackend(ctx context.Context, cfg BackendConfig) (*S3Backend, error) {
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
		o.UsePathStyle = true
	})

	backend := &S3Backend{
		client: client,
		bucket: cfg.Bucket,
	}

	// Verify bucket exists
	if err := backend.ensureBucket(ctx); err != nil {
		logging.Error("bucket check failed", zap.Error(err))
	}

	return backend, nil
}

// NewBackendFromJSON creates an S3Backend from raw JSON config.
func NewBackendFromJSON(ctx context.Context, raw json.RawMessage) (*S3Backend, error) {
	var cfg BackendConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parse s3 config: %w", err)
	}
	return NewBackend(ctx, cfg)
}

func (b *S3Backend) ensureBucket(ctx context.Context) error {
	start := time.Now()
	_, err := b.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(b.bucket),
	})
	if err != nil {
		_, createErr := b.client.CreateBucket(ctx, &s3.CreateBucketInput{
			Bucket: aws.String(b.bucket),
		})
		if createErr != nil {
			metrics.RecordS3Operation("create_bucket", time.Since(start), false)
			return fmt.Errorf("bucket %s does not exist and cannot create: %w", b.bucket, createErr)
		}
		metrics.RecordS3Operation("create_bucket", time.Since(start), true)
		logging.Info("created S3 bucket", zap.String("bucket", b.bucket))
	}
	return nil
}

// GetObject retrieves an object from S3 with range support.
func (b *S3Backend) GetObject(ctx context.Context, key string, offset, length int64) (io.ReadCloser, int64, error) {
	start := time.Now()

	input := &s3.GetObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(key),
	}

	if offset > 0 || length > 0 {
		var rangeStr string
		if length > 0 {
			rangeStr = fmt.Sprintf("bytes=%d-%d", offset, offset+length-1)
		} else {
			rangeStr = fmt.Sprintf("bytes=%d-", offset)
		}
		input.Range = aws.String(rangeStr)
	}

	result, err := b.client.GetObject(ctx, input)
	if err != nil {
		metrics.RecordS3Operation("get_object", time.Since(start), false)
		return nil, 0, fmt.Errorf("get object %s: %w", key, err)
	}

	metrics.RecordS3Operation("get_object", time.Since(start), true)

	totalSize := int64(0)
	if result.ContentLength != nil {
		totalSize = *result.ContentLength
	}

	return result.Body, totalSize, nil
}

// PutObject uploads content to S3.
func (b *S3Backend) PutObject(ctx context.Context, key string, body io.Reader, size int64) error {
	start := time.Now()

	_, err := b.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(b.bucket),
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
func (b *S3Backend) DeleteObject(ctx context.Context, key string) error {
	start := time.Now()

	_, err := b.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(b.bucket),
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

// CopyObject copies an S3 object from srcKey to dstKey.
func (b *S3Backend) CopyObject(ctx context.Context, srcKey, dstKey string) error {
	start := time.Now()

	_, err := b.client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(b.bucket),
		Key:        aws.String(dstKey),
		CopySource: aws.String(b.bucket + "/" + srcKey),
	})
	if err != nil {
		metrics.RecordS3Operation("copy_object", time.Since(start), false)
		return fmt.Errorf("copy %s -> %s: %w", srcKey, dstKey, err)
	}

	metrics.RecordS3Operation("copy_object", time.Since(start), true)
	logging.Debug("S3 copy object", zap.String("src", srcKey), zap.String("dst", dstKey))
	return nil
}

// ObjectExists checks if an object exists in S3.
func (b *S3Backend) ObjectExists(ctx context.Context, key string) (bool, error) {
	start := time.Now()

	_, err := b.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		metrics.RecordS3Operation("head_object", time.Since(start), false)
		return false, nil
	}

	metrics.RecordS3Operation("head_object", time.Since(start), true)
	return true, nil
}

// Type returns "s3".
func (b *S3Backend) Type() string { return "s3" }

// Close is a no-op for S3 backends.
func (b *S3Backend) Close() error { return nil }
