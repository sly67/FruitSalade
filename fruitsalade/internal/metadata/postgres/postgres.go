// Package postgres provides a PostgreSQL-backed metadata store with metrics.
package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/lib/pq"

	"github.com/fruitsalade/fruitsalade/fruitsalade/internal/logging"
	"github.com/fruitsalade/fruitsalade/fruitsalade/internal/metrics"
	"github.com/fruitsalade/fruitsalade/shared/pkg/models"
	"go.uber.org/zap"
)

// Store is a PostgreSQL metadata store.
type Store struct {
	db *sql.DB
}

// FileRow maps to the files table.
type FileRow struct {
	ID           string
	Name         string
	Path         string
	ParentPath   string
	Size         int64
	ModTime      time.Time
	IsDir        bool
	Hash         string
	S3Key        string
	Version      int
	OwnerID      *int
	Visibility   string // "public", "group", "private"
	GroupID      *int
	StorageLocID *int // Storage location ID (NULL = default)
}

// New creates a new PostgreSQL metadata store.
func New(databaseURL string) (*Store, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &Store{db: db}, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// DB returns the underlying database connection for use by other packages.
func (s *Store) DB() *sql.DB {
	return s.db
}

// UpdateConnectionMetrics updates the database connection metrics.
func (s *Store) UpdateConnectionMetrics() {
	stats := s.db.Stats()
	metrics.SetDBConnectionsOpen(stats.OpenConnections)
}

// Migrate runs SQL migration files.
func (s *Store) Migrate(migrationsDir string) error {
	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.up.sql"))
	if err != nil {
		return fmt.Errorf("glob migrations: %w", err)
	}

	for _, f := range files {
		logging.Info("running migration", zap.String("file", filepath.Base(f)))
		content, err := os.ReadFile(f)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", f, err)
		}
		if _, err := s.db.Exec(string(content)); err != nil {
			return fmt.Errorf("exec migration %s: %w", f, err)
		}
	}

	return nil
}

// BuildTree builds the full metadata tree from the database.
func (s *Store) BuildTree(ctx context.Context) (*models.FileNode, error) {
	start := time.Now()
	defer func() {
		metrics.RecordDBQuery("build_tree", time.Since(start))
		metrics.RecordMetadataRefresh(time.Since(start))
	}()

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, path, parent_path, size, mod_time, is_dir, hash, s3_key, version, owner_id, visibility, group_id, storage_location_id
		 FROM files ORDER BY path`)
	if err != nil {
		return nil, fmt.Errorf("query files: %w", err)
	}
	defer rows.Close()

	// Build a map of path -> node
	nodeMap := make(map[string]*models.FileNode)
	var allRows []FileRow

	for rows.Next() {
		var r FileRow
		var ownerID, groupID, storageLocID sql.NullInt64
		var visibility sql.NullString
		if err := rows.Scan(&r.ID, &r.Name, &r.Path, &r.ParentPath,
			&r.Size, &r.ModTime, &r.IsDir, &r.Hash, &r.S3Key, &r.Version, &ownerID, &visibility, &groupID, &storageLocID); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		if ownerID.Valid {
			oid := int(ownerID.Int64)
			r.OwnerID = &oid
		}
		if visibility.Valid {
			r.Visibility = visibility.String
		} else {
			r.Visibility = "public"
		}
		if groupID.Valid {
			gid := int(groupID.Int64)
			r.GroupID = &gid
		}
		if storageLocID.Valid {
			slid := int(storageLocID.Int64)
			r.StorageLocID = &slid
		}
		allRows = append(allRows, r)
		nodeMap[r.Path] = rowToNode(&r)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	// Update tree size metric
	metrics.SetMetadataTreeSize(int64(len(allRows)))

	// Build parent-child relationships
	var root *models.FileNode
	for _, r := range allRows {
		node := nodeMap[r.Path]
		if r.Path == "/" {
			root = node
			continue
		}
		parent, ok := nodeMap[r.ParentPath]
		if ok {
			parent.Children = append(parent.Children, node)
		}
	}

	// If no root in DB, create a virtual root
	if root == nil {
		root = &models.FileNode{
			ID:    "root",
			Name:  "root",
			Path:  "/",
			IsDir: true,
		}
		// Attach orphan top-level nodes
		for _, r := range allRows {
			if r.ParentPath == "/" {
				root.Children = append(root.Children, nodeMap[r.Path])
			}
		}
	}

	logging.Debug("built metadata tree", zap.Int("nodes", len(allRows)))
	return root, nil
}

// GetMetadata returns metadata for a single path.
func (s *Store) GetMetadata(ctx context.Context, path string) (*models.FileNode, error) {
	start := time.Now()
	defer func() { metrics.RecordDBQuery("get_metadata", time.Since(start)) }()

	path = normalizePath(path)
	var r FileRow
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, path, parent_path, size, mod_time, is_dir, hash, s3_key
		 FROM files WHERE path = $1`, path).
		Scan(&r.ID, &r.Name, &r.Path, &r.ParentPath,
			&r.Size, &r.ModTime, &r.IsDir, &r.Hash, &r.S3Key)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("not found: %s", path)
	}
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	return rowToNode(&r), nil
}

// GetFileRow returns the full file row for a path (including S3Key and version).
func (s *Store) GetFileRow(ctx context.Context, path string) (*FileRow, error) {
	start := time.Now()
	defer func() { metrics.RecordDBQuery("get_file_row", time.Since(start)) }()

	path = normalizePath(path)
	var r FileRow
	var ownerID, groupID, storageLocID sql.NullInt64
	var visibility sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, path, parent_path, size, mod_time, is_dir, hash, s3_key, version, owner_id, visibility, group_id, storage_location_id
		 FROM files WHERE path = $1`, path).
		Scan(&r.ID, &r.Name, &r.Path, &r.ParentPath,
			&r.Size, &r.ModTime, &r.IsDir, &r.Hash, &r.S3Key, &r.Version, &ownerID, &visibility, &groupID, &storageLocID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	if ownerID.Valid {
		oid := int(ownerID.Int64)
		r.OwnerID = &oid
	}
	if visibility.Valid {
		r.Visibility = visibility.String
	} else {
		r.Visibility = "public"
	}
	if groupID.Valid {
		gid := int(groupID.Int64)
		r.GroupID = &gid
	}
	if storageLocID.Valid {
		slid := int(storageLocID.Int64)
		r.StorageLocID = &slid
	}
	return &r, nil
}

// ListDir returns children of a directory.
func (s *Store) ListDir(ctx context.Context, path string) ([]*models.FileNode, error) {
	start := time.Now()
	defer func() { metrics.RecordDBQuery("list_dir", time.Since(start)) }()

	path = normalizePath(path)
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, path, parent_path, size, mod_time, is_dir, hash, s3_key
		 FROM files WHERE parent_path = $1 ORDER BY name`, path)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	var nodes []*models.FileNode
	for rows.Next() {
		var r FileRow
		if err := rows.Scan(&r.ID, &r.Name, &r.Path, &r.ParentPath,
			&r.Size, &r.ModTime, &r.IsDir, &r.Hash, &r.S3Key); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		nodes = append(nodes, rowToNode(&r))
	}

	return nodes, rows.Err()
}

// GetS3Key returns the S3 object key for a file ID.
func (s *Store) GetS3Key(ctx context.Context, fileID string) (string, error) {
	start := time.Now()
	defer func() { metrics.RecordDBQuery("get_s3_key", time.Since(start)) }()

	var s3Key string
	err := s.db.QueryRowContext(ctx,
		`SELECT s3_key FROM files WHERE id = $1`, fileID).Scan(&s3Key)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("file not found: %s", fileID)
	}
	if err != nil {
		return "", fmt.Errorf("query: %w", err)
	}
	return s3Key, nil
}

// GetFileSize returns the size of a file by ID.
func (s *Store) GetFileSize(ctx context.Context, fileID string) (int64, error) {
	start := time.Now()
	defer func() { metrics.RecordDBQuery("get_file_size", time.Since(start)) }()

	var size int64
	err := s.db.QueryRowContext(ctx,
		`SELECT size FROM files WHERE id = $1`, fileID).Scan(&size)
	if err == sql.ErrNoRows {
		return 0, fmt.Errorf("file not found: %s", fileID)
	}
	if err != nil {
		return 0, fmt.Errorf("query: %w", err)
	}
	return size, nil
}

// UpsertFile inserts or updates a file metadata entry.
func (s *Store) UpsertFile(ctx context.Context, f *FileRow) error {
	start := time.Now()
	defer func() { metrics.RecordDBQuery("upsert_file", time.Since(start)) }()

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO files (id, name, path, parent_path, size, mod_time, is_dir, hash, s3_key, version, owner_id, visibility, group_id, storage_location_id, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, NOW())
		 ON CONFLICT (path) DO UPDATE SET
			name = EXCLUDED.name,
			size = EXCLUDED.size,
			mod_time = EXCLUDED.mod_time,
			hash = EXCLUDED.hash,
			s3_key = EXCLUDED.s3_key,
			version = EXCLUDED.version,
			owner_id = COALESCE(files.owner_id, EXCLUDED.owner_id),
			visibility = COALESCE(NULLIF(EXCLUDED.visibility, ''), files.visibility),
			group_id = COALESCE(EXCLUDED.group_id, files.group_id),
			storage_location_id = COALESCE(EXCLUDED.storage_location_id, files.storage_location_id),
			updated_at = NOW()`,
		f.ID, f.Name, f.Path, f.ParentPath, f.Size, f.ModTime, f.IsDir, f.Hash, f.S3Key, f.Version, f.OwnerID, f.Visibility, f.GroupID, f.StorageLocID)
	if err != nil {
		return fmt.Errorf("upsert: %w", err)
	}

	logging.Debug("upserted file",
		zap.String("path", f.Path),
		zap.Bool("is_dir", f.IsDir),
		zap.Int64("size", f.Size))
	return nil
}

// DeleteFile removes a file entry.
func (s *Store) DeleteFile(ctx context.Context, path string) error {
	start := time.Now()
	defer func() { metrics.RecordDBQuery("delete_file", time.Since(start)) }()

	path = normalizePath(path)
	result, err := s.db.ExecContext(ctx, `DELETE FROM files WHERE path = $1`, path)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	logging.Debug("deleted file", zap.String("path", path), zap.Int64("rows", rows))
	return nil
}

// DeleteTree removes a directory and all its children.
func (s *Store) DeleteTree(ctx context.Context, path string) (int64, error) {
	start := time.Now()
	defer func() { metrics.RecordDBQuery("delete_tree", time.Since(start)) }()

	path = normalizePath(path)
	result, err := s.db.ExecContext(ctx,
		`DELETE FROM files WHERE path = $1 OR path LIKE $2`,
		path, path+"/%")
	if err != nil {
		return 0, err
	}
	rows, _ := result.RowsAffected()
	logging.Debug("deleted tree", zap.String("path", path), zap.Int64("rows", rows))
	return rows, nil
}

// FileCount returns the total number of file entries.
func (s *Store) FileCount(ctx context.Context) (int64, error) {
	start := time.Now()
	defer func() { metrics.RecordDBQuery("file_count", time.Since(start)) }()

	var count int64
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM files`).Scan(&count)
	return count, err
}

// PathExists checks if a path exists in the database.
func (s *Store) PathExists(ctx context.Context, path string) (bool, error) {
	path = normalizePath(path)
	var exists bool
	err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM files WHERE path = $1)`, path).Scan(&exists)
	return exists, err
}

func rowToNode(r *FileRow) *models.FileNode {
	node := &models.FileNode{
		ID:         r.ID,
		Name:       r.Name,
		Path:       r.Path,
		Size:       r.Size,
		ModTime:    r.ModTime,
		IsDir:      r.IsDir,
		Hash:       r.Hash,
		Version:    r.Version,
		Visibility: r.Visibility,
	}
	if r.OwnerID != nil {
		node.OwnerID = *r.OwnerID
	}
	if r.GroupID != nil {
		node.GroupID = *r.GroupID
	}
	return node
}

// VersionRecord holds a file version from the file_versions table.
type VersionRecord struct {
	FileID       string
	Path         string
	Version      int
	Size         int64
	Hash         string
	S3Key        string
	StorageLocID *int
	CreatedAt    time.Time
}

// SaveVersion saves the current file state as a version record.
func (s *Store) SaveVersion(ctx context.Context, path string) error {
	start := time.Now()
	defer func() { metrics.RecordDBQuery("save_version", time.Since(start)) }()

	path = normalizePath(path)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO file_versions (file_id, path, version, size, hash, s3_key, storage_location_id)
		 SELECT id, path, version, size, hash, s3_key, storage_location_id FROM files WHERE path = $1
		 ON CONFLICT (path, version) DO NOTHING`,
		path)
	if err != nil {
		return fmt.Errorf("save version: %w", err)
	}

	logging.Debug("saved version", zap.String("path", path))
	return nil
}

// ListVersions returns all versions for a file path, ordered by version descending.
func (s *Store) ListVersions(ctx context.Context, path string) ([]VersionRecord, int, error) {
	start := time.Now()
	defer func() { metrics.RecordDBQuery("list_versions", time.Since(start)) }()

	path = normalizePath(path)

	// Get current version
	var currentVersion int
	err := s.db.QueryRowContext(ctx,
		`SELECT version FROM files WHERE path = $1`, path).Scan(&currentVersion)
	if err != nil {
		return nil, 0, fmt.Errorf("get current version: %w", err)
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT file_id, path, version, size, hash, s3_key, storage_location_id, created_at
		 FROM file_versions WHERE path = $1 ORDER BY version DESC`, path)
	if err != nil {
		return nil, 0, fmt.Errorf("query versions: %w", err)
	}
	defer rows.Close()

	var versions []VersionRecord
	for rows.Next() {
		var v VersionRecord
		var slid sql.NullInt64
		if err := rows.Scan(&v.FileID, &v.Path, &v.Version, &v.Size, &v.Hash, &v.S3Key, &slid, &v.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan version: %w", err)
		}
		if slid.Valid {
			id := int(slid.Int64)
			v.StorageLocID = &id
		}
		versions = append(versions, v)
	}

	return versions, currentVersion, rows.Err()
}

// GetVersion returns a specific version record.
func (s *Store) GetVersion(ctx context.Context, path string, version int) (*VersionRecord, error) {
	start := time.Now()
	defer func() { metrics.RecordDBQuery("get_version", time.Since(start)) }()

	path = normalizePath(path)
	var v VersionRecord
	var slid sql.NullInt64
	err := s.db.QueryRowContext(ctx,
		`SELECT file_id, path, version, size, hash, s3_key, storage_location_id, created_at
		 FROM file_versions WHERE path = $1 AND version = $2`, path, version).
		Scan(&v.FileID, &v.Path, &v.Version, &v.Size, &v.Hash, &v.S3Key, &slid, &v.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get version %d for %s: %w", version, path, err)
	}
	if slid.Valid {
		id := int(slid.Int64)
		v.StorageLocID = &id
	}
	return &v, nil
}

// RestoreVersion restores a file to a previous version's metadata.
func (s *Store) RestoreVersion(ctx context.Context, path string, version int, newVersion int, s3Key string) error {
	start := time.Now()
	defer func() { metrics.RecordDBQuery("restore_version", time.Since(start)) }()

	path = normalizePath(path)
	var v VersionRecord
	err := s.db.QueryRowContext(ctx,
		`SELECT size, hash FROM file_versions WHERE path = $1 AND version = $2`,
		path, version).Scan(&v.Size, &v.Hash)
	if err != nil {
		return fmt.Errorf("get version: %w", err)
	}

	_, err = s.db.ExecContext(ctx,
		`UPDATE files SET size = $1, hash = $2, s3_key = $3, version = $4,
		 mod_time = NOW(), updated_at = NOW() WHERE path = $5`,
		v.Size, v.Hash, s3Key, newVersion, path)
	if err != nil {
		return fmt.Errorf("restore version: %w", err)
	}

	logging.Info("restored version",
		zap.String("path", path),
		zap.Int("from_version", version),
		zap.Int("to_version", newVersion))
	return nil
}

// VersionedFileSummary holds summary info about a file with version history.
type VersionedFileSummary struct {
	Path           string    `json:"path"`
	Name           string    `json:"name"`
	CurrentVersion int       `json:"current_version"`
	VersionCount   int       `json:"version_count"`
	Size           int64     `json:"size"`
	LatestChange   time.Time `json:"latest_change"`
}

// ListVersionedFiles returns all files that have at least one entry in file_versions.
func (s *Store) ListVersionedFiles(ctx context.Context) ([]VersionedFileSummary, error) {
	start := time.Now()
	defer func() { metrics.RecordDBQuery("list_versioned_files", time.Since(start)) }()

	rows, err := s.db.QueryContext(ctx,
		`SELECT f.path, f.name, f.version, COUNT(fv.id) AS version_count, f.size,
		        COALESCE(MAX(fv.created_at), f.mod_time) AS latest_change
		 FROM files f
		 JOIN file_versions fv ON fv.path = f.path
		 WHERE f.is_dir = FALSE
		 GROUP BY f.path, f.name, f.version, f.size, f.mod_time
		 ORDER BY latest_change DESC`)
	if err != nil {
		return nil, fmt.Errorf("list versioned files: %w", err)
	}
	defer rows.Close()

	var results []VersionedFileSummary
	for rows.Next() {
		var v VersionedFileSummary
		if err := rows.Scan(&v.Path, &v.Name, &v.CurrentVersion, &v.VersionCount, &v.Size, &v.LatestChange); err != nil {
			return nil, fmt.Errorf("scan versioned file: %w", err)
		}
		results = append(results, v)
	}
	return results, rows.Err()
}

func normalizePath(path string) string {
	if path == "" {
		return "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	// Don't trim trailing slash from root
	if path == "/" {
		return path
	}
	return strings.TrimSuffix(path, "/")
}
