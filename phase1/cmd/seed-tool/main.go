// seed-tool populates PostgreSQL and S3 with test data.
//
// It walks a local directory (-data flag or /testdata default),
// uploads each file to S3, and records metadata in PostgreSQL.
// Designed to run once as an init container.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	_ "github.com/lib/pq"

	"github.com/fruitsalade/fruitsalade/phase1/internal/config"
	"github.com/fruitsalade/fruitsalade/phase1/internal/metadata/postgres"
	"github.com/fruitsalade/fruitsalade/shared/pkg/logger"
)

func main() {
	dataDir := flag.String("data", "/testdata", "Directory with seed files")
	migrationsDir := flag.String("migrations", "/app/migrations", "Migrations directory")
	flag.Parse()

	logger.SetLevel(logger.LevelInfo)
	logger.Info("FruitSalade seed-tool starting...")

	cfg, err := config.Load()
	if err != nil {
		logger.Error("Config error: %v", err)
		os.Exit(1)
	}

	// Connect to PostgreSQL with retries
	var store *postgres.Store
	for i := 0; i < 15; i++ {
		store, err = postgres.New(cfg.DatabaseURL)
		if err == nil {
			break
		}
		logger.Info("Waiting for PostgreSQL (%d/15): %v", i+1, err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		logger.Error("Failed to connect to PostgreSQL: %v", err)
		os.Exit(1)
	}
	defer store.Close()

	// Run migrations
	logger.Info("Running migrations from %s...", *migrationsDir)
	if err := store.Migrate(*migrationsDir); err != nil {
		logger.Error("Migration failed: %v", err)
		os.Exit(1)
	}

	// Connect to S3
	ctx := context.Background()
	s3Client, err := newS3Client(ctx, cfg)
	if err != nil {
		logger.Error("S3 client init failed: %v", err)
		os.Exit(1)
	}

	// Upsert root directory
	root := &postgres.FileRow{
		ID:         fileID("/"),
		Name:       "root",
		Path:       "/",
		ParentPath: "",
		IsDir:      true,
		ModTime:    time.Now(),
	}
	if err := store.UpsertFile(ctx, root); err != nil {
		logger.Error("Failed to upsert root: %v", err)
		os.Exit(1)
	}

	// Track directories we've created
	createdDirs := map[string]bool{"/": true}

	// Walk data directory
	logger.Info("Seeding files from %s...", *dataDir)
	err = filepath.Walk(*dataDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		// Compute virtual path (relative to data dir, with leading /)
		relPath, _ := filepath.Rel(*dataDir, path)
		if relPath == "." {
			return nil // skip root itself
		}
		virtualPath := "/" + filepath.ToSlash(relPath)

		if info.IsDir() {
			return seedDir(ctx, store, virtualPath, info, createdDirs)
		}
		return seedFile(ctx, store, s3Client, cfg.S3Bucket, path, virtualPath, info, createdDirs)
	})
	if err != nil {
		logger.Error("Walk failed: %v", err)
		os.Exit(1)
	}

	// Count total files
	total, _ := store.FileCount(ctx)
	logger.Info("Seeding complete. %d entries in database.", total)
}

func fileID(virtualPath string) string {
	h := sha256.Sum256([]byte(virtualPath))
	return fmt.Sprintf("%x", h[:8]) // first 16 hex chars
}

func ensureParentDirs(ctx context.Context, store *postgres.Store, virtualPath string, createdDirs map[string]bool) error {
	dir := filepath.Dir(virtualPath)
	if dir == "." || dir == "/" {
		return nil
	}
	dir = filepath.ToSlash(dir)
	if createdDirs[dir] {
		return nil
	}
	// Recursively ensure parent exists
	if err := ensureParentDirs(ctx, store, dir, createdDirs); err != nil {
		return err
	}
	return seedDir(ctx, store, dir, nil, createdDirs)
}

func seedDir(ctx context.Context, store *postgres.Store, virtualPath string, info os.FileInfo, createdDirs map[string]bool) error {
	if createdDirs[virtualPath] {
		return nil
	}
	parentPath := filepath.ToSlash(filepath.Dir(virtualPath))
	if parentPath == "." {
		parentPath = "/"
	}
	name := filepath.Base(virtualPath)
	modTime := time.Now()
	if info != nil {
		modTime = info.ModTime()
	}

	row := &postgres.FileRow{
		ID:         fileID(virtualPath),
		Name:       name,
		Path:       virtualPath,
		ParentPath: parentPath,
		IsDir:      true,
		ModTime:    modTime,
	}
	if err := store.UpsertFile(ctx, row); err != nil {
		return fmt.Errorf("upsert dir %s: %w", virtualPath, err)
	}
	createdDirs[virtualPath] = true
	logger.Info("  DIR  %s", virtualPath)
	return nil
}

func seedFile(ctx context.Context, store *postgres.Store, s3Client *s3.Client, bucket, localPath, virtualPath string, info os.FileInfo, createdDirs map[string]bool) error {
	// Ensure parent directories exist
	if err := ensureParentDirs(ctx, store, virtualPath, createdDirs); err != nil {
		return err
	}

	// Read file content
	data, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", localPath, err)
	}

	// Compute hash
	hash := sha256.Sum256(data)
	hashStr := fmt.Sprintf("%x", hash)

	// S3 key is the virtual path without leading /
	s3Key := strings.TrimPrefix(virtualPath, "/")

	// Upload to S3
	_, err = s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(bucket),
		Key:           aws.String(s3Key),
		Body:          bytes.NewReader(data),
		ContentLength: aws.Int64(int64(len(data))),
	})
	if err != nil {
		return fmt.Errorf("upload %s to S3: %w", s3Key, err)
	}

	// Record metadata
	parentPath := filepath.ToSlash(filepath.Dir(virtualPath))
	if parentPath == "." {
		parentPath = "/"
	}
	row := &postgres.FileRow{
		ID:         fileID(virtualPath),
		Name:       filepath.Base(virtualPath),
		Path:       virtualPath,
		ParentPath: parentPath,
		Size:       int64(len(data)),
		ModTime:    info.ModTime(),
		IsDir:      false,
		Hash:       hashStr,
		S3Key:      s3Key,
	}
	if err := store.UpsertFile(ctx, row); err != nil {
		return fmt.Errorf("upsert file %s: %w", virtualPath, err)
	}

	logger.Info("  FILE %s (%d bytes)", virtualPath, len(data))
	return nil
}

func newS3Client(ctx context.Context, cfg *config.Config) (*s3.Client, error) {
	resolver := aws.EndpointResolverWithOptionsFunc(
		func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL:               cfg.S3Endpoint,
				HostnameImmutable: true,
			}, nil
		},
	)

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(cfg.S3Region),
		awsconfig.WithEndpointResolverWithOptions(resolver),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.S3AccessKey, cfg.S3SecretKey, ""),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	return s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = true
	}), nil
}
