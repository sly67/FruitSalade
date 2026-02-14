// Phase 2 seed-tool populates PostgreSQL and storage with test data.
//
// Backend-agnostic: uses the storage.Backend interface via the storage router,
// so it works with S3, local filesystem, or SMB backends.
//
// It walks a local directory (-data flag or /testdata default),
// uploads each file via the configured backend, and records metadata in PostgreSQL.
// Designed to run once as an init container.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/lib/pq"

	"github.com/fruitsalade/fruitsalade/phase2/internal/config"
	"github.com/fruitsalade/fruitsalade/phase2/internal/logging"
	"github.com/fruitsalade/fruitsalade/phase2/internal/metadata/postgres"
	"github.com/fruitsalade/fruitsalade/phase2/internal/sharing"
	"github.com/fruitsalade/fruitsalade/phase2/internal/storage"
	"github.com/fruitsalade/fruitsalade/phase2/internal/storage/local"
	s3storage "github.com/fruitsalade/fruitsalade/phase2/internal/storage/s3"
	"go.uber.org/zap"
)

func main() {
	dataDir := flag.String("data", "/testdata", "Directory with seed files")
	migrationsDir := flag.String("migrations", "/app/migrations", "Migrations directory")
	flag.Parse()

	// Initialize logging
	if err := logging.Init(logging.Config{Level: "info", Format: "console"}); err != nil {
		panic("logging init: " + err.Error())
	}
	defer logging.Sync()

	logging.Info("FruitSalade Phase 2 seed-tool starting...")

	cfg, err := config.Load()
	if err != nil {
		logging.Fatal("config error", zap.Error(err))
	}

	ctx := context.Background()

	// Connect to PostgreSQL with retries
	var metaStore *postgres.Store
	for i := 0; i < 15; i++ {
		metaStore, err = postgres.New(cfg.DatabaseURL)
		if err == nil {
			break
		}
		logging.Info("waiting for PostgreSQL",
			zap.Int("attempt", i+1),
			zap.Error(err))
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		logging.Fatal("failed to connect to PostgreSQL", zap.Error(err))
	}
	defer metaStore.Close()

	// Run migrations
	logging.Info("running migrations...", zap.String("dir", *migrationsDir))
	if err := metaStore.Migrate(*migrationsDir); err != nil {
		logging.Fatal("migration failed", zap.Error(err))
	}

	// Initialize storage router
	db := metaStore.DB()
	groupStore := sharing.NewGroupStore(db)
	locationStore := storage.NewLocationStore(db)

	storageRouter, err := storage.NewRouter(ctx, locationStore, groupStore)
	if err != nil {
		logging.Fatal("storage router init failed", zap.Error(err))
	}
	defer storageRouter.Close()

	// Auto-create default storage location if none exists
	if storageRouter.DefaultLocation() == nil {
		var locName, backendType string
		var backendConfig json.RawMessage

		if cfg.StorageBackend == "s3" {
			locName = "Default S3"
			backendType = "s3"
			backendConfig, _ = json.Marshal(s3storage.BackendConfig{
				Endpoint:  cfg.S3Endpoint,
				Bucket:    cfg.S3Bucket,
				AccessKey: cfg.S3AccessKey,
				SecretKey: cfg.S3SecretKey,
				Region:    cfg.S3Region,
				UseSSL:    cfg.S3UseSSL,
			})
		} else {
			locName = "Default Local"
			backendType = "local"
			backendConfig, _ = json.Marshal(local.Config{
				RootPath:   cfg.LocalStoragePath,
				CreateDirs: true,
			})
		}

		_, err := locationStore.Create(ctx, &storage.LocationRow{
			Name:        locName,
			BackendType: backendType,
			Config:      backendConfig,
			IsDefault:   true,
		})
		if err != nil {
			logging.Fatal("failed to create default storage location", zap.Error(err))
		}
		if err := storageRouter.Reload(ctx); err != nil {
			logging.Fatal("failed to reload storage router", zap.Error(err))
		}
		logging.Info("auto-created default storage location",
			zap.String("backend", backendType),
			zap.String("name", locName))
	}

	// Get default backend
	backend, _, err := storageRouter.GetDefault()
	if err != nil {
		logging.Fatal("no default storage backend", zap.Error(err))
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
	if err := metaStore.UpsertFile(ctx, root); err != nil {
		logging.Fatal("failed to upsert root", zap.Error(err))
	}

	// Track directories we've created
	createdDirs := map[string]bool{"/": true}

	// Walk data directory
	logging.Info("seeding files...", zap.String("dir", *dataDir))
	err = filepath.Walk(*dataDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relPath, _ := filepath.Rel(*dataDir, path)
		if relPath == "." {
			return nil
		}
		virtualPath := "/" + filepath.ToSlash(relPath)

		if info.IsDir() {
			return seedDir(ctx, metaStore, virtualPath, info, createdDirs)
		}
		return seedFile(ctx, metaStore, backend, path, virtualPath, info, createdDirs)
	})
	if err != nil {
		logging.Fatal("walk failed", zap.Error(err))
	}

	total, _ := metaStore.FileCount(ctx)
	logging.Info("seeding complete", zap.Int64("entries", total))
}

func fileID(virtualPath string) string {
	h := sha256.Sum256([]byte(virtualPath))
	return fmt.Sprintf("%x", h[:8])
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
	logging.Info("  DIR", zap.String("path", virtualPath))
	return nil
}

func seedFile(ctx context.Context, store *postgres.Store, backend storage.Backend, localPath, virtualPath string, info os.FileInfo, createdDirs map[string]bool) error {
	if err := ensureParentDirs(ctx, store, virtualPath, createdDirs); err != nil {
		return err
	}

	data, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", localPath, err)
	}

	hash := sha256.Sum256(data)
	hashStr := fmt.Sprintf("%x", hash)

	// Storage key is the virtual path without leading /
	key := strings.TrimPrefix(virtualPath, "/")

	// Upload via backend interface
	if err := backend.PutObject(ctx, key, bytes.NewReader(data), int64(len(data))); err != nil {
		return fmt.Errorf("upload %s: %w", key, err)
	}

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
		S3Key:      key,
	}
	if err := store.UpsertFile(ctx, row); err != nil {
		return fmt.Errorf("upsert file %s: %w", virtualPath, err)
	}

	logging.Info("  FILE", zap.String("path", virtualPath), zap.Int("bytes", len(data)))
	return nil
}
