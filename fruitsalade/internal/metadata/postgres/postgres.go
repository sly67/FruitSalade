// Package postgres provides a PostgreSQL-backed metadata store with metrics.
package postgres

import (
	"context"
	"crypto/sha256"
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
		 FROM files WHERE deleted_at IS NULL ORDER BY path`)
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
		 FROM files WHERE parent_path = $1 AND deleted_at IS NULL ORDER BY name`, path)
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
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM files WHERE deleted_at IS NULL`).Scan(&count)
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
		 WHERE f.is_dir = FALSE AND f.deleted_at IS NULL
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

// ─── Trash (Soft Delete) ─────────────────────────────────────────────────────

// TrashRow holds a trashed file's information.
type TrashRow struct {
	ID            string
	Name          string
	OriginalPath  string
	Size          int64
	IsDir         bool
	DeletedAt     time.Time
	DeletedByName string
}

// SoftDeleteFile marks a file (or directory tree) as deleted.
func (s *Store) SoftDeleteFile(ctx context.Context, path string, userID int) error {
	start := time.Now()
	defer func() { metrics.RecordDBQuery("soft_delete_file", time.Since(start)) }()

	path = normalizePath(path)
	_, err := s.db.ExecContext(ctx,
		`UPDATE files SET deleted_at = NOW(), deleted_by = $2, original_path = path
		 WHERE (path = $1 OR path LIKE $1 || '/%') AND deleted_at IS NULL`,
		path, userID)
	if err != nil {
		return fmt.Errorf("soft delete: %w", err)
	}
	logging.Debug("soft-deleted file", zap.String("path", path), zap.Int("user_id", userID))
	return nil
}

// ListTrash returns all soft-deleted files. If userID is non-nil, filters by deleted_by.
func (s *Store) ListTrash(ctx context.Context, userID *int) ([]TrashRow, error) {
	start := time.Now()
	defer func() { metrics.RecordDBQuery("list_trash", time.Since(start)) }()

	var query string
	var args []interface{}
	if userID != nil {
		query = `SELECT f.id, f.name, f.original_path, f.size, f.is_dir, f.deleted_at,
		         COALESCE(u.username, '') AS deleted_by_name
		         FROM files f LEFT JOIN users u ON u.id = f.deleted_by
		         WHERE f.deleted_at IS NOT NULL AND f.deleted_by = $1
		         ORDER BY f.deleted_at DESC`
		args = []interface{}{*userID}
	} else {
		query = `SELECT f.id, f.name, f.original_path, f.size, f.is_dir, f.deleted_at,
		         COALESCE(u.username, '') AS deleted_by_name
		         FROM files f LEFT JOIN users u ON u.id = f.deleted_by
		         WHERE f.deleted_at IS NOT NULL
		         ORDER BY f.deleted_at DESC`
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list trash: %w", err)
	}
	defer rows.Close()

	var items []TrashRow
	for rows.Next() {
		var t TrashRow
		if err := rows.Scan(&t.ID, &t.Name, &t.OriginalPath, &t.Size, &t.IsDir,
			&t.DeletedAt, &t.DeletedByName); err != nil {
			return nil, fmt.Errorf("scan trash: %w", err)
		}
		items = append(items, t)
	}
	return items, rows.Err()
}

// RestoreFile restores a soft-deleted file by its original path.
func (s *Store) RestoreFile(ctx context.Context, originalPath string) error {
	start := time.Now()
	defer func() { metrics.RecordDBQuery("restore_file", time.Since(start)) }()

	originalPath = normalizePath(originalPath)
	result, err := s.db.ExecContext(ctx,
		`UPDATE files SET deleted_at = NULL, deleted_by = NULL
		 WHERE original_path = $1 AND deleted_at IS NOT NULL`,
		originalPath)
	if err != nil {
		return fmt.Errorf("restore file: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("not found in trash: %s", originalPath)
	}
	logging.Debug("restored file", zap.String("path", originalPath), zap.Int64("rows", rows))
	return nil
}

// PurgeFileRow holds info needed to clean up storage after purge.
type PurgeFileRow struct {
	S3Key        string
	StorageLocID *int
	GroupID      *int
}

// PurgeFile permanently deletes a trashed file. Returns storage info for cleanup.
func (s *Store) PurgeFile(ctx context.Context, originalPath string) ([]PurgeFileRow, error) {
	start := time.Now()
	defer func() { metrics.RecordDBQuery("purge_file", time.Since(start)) }()

	originalPath = normalizePath(originalPath)
	rows, err := s.db.QueryContext(ctx,
		`DELETE FROM files WHERE original_path = $1 AND deleted_at IS NOT NULL
		 RETURNING s3_key, storage_location_id, group_id`,
		originalPath)
	if err != nil {
		return nil, fmt.Errorf("purge file: %w", err)
	}
	defer rows.Close()

	var purged []PurgeFileRow
	for rows.Next() {
		var p PurgeFileRow
		var slid, gid sql.NullInt64
		if err := rows.Scan(&p.S3Key, &slid, &gid); err != nil {
			return nil, fmt.Errorf("scan purge: %w", err)
		}
		if slid.Valid {
			id := int(slid.Int64)
			p.StorageLocID = &id
		}
		if gid.Valid {
			id := int(gid.Int64)
			p.GroupID = &id
		}
		purged = append(purged, p)
	}
	return purged, rows.Err()
}

// PurgeAllTrash permanently deletes all trashed files. Returns storage info for cleanup.
func (s *Store) PurgeAllTrash(ctx context.Context) ([]PurgeFileRow, error) {
	start := time.Now()
	defer func() { metrics.RecordDBQuery("purge_all_trash", time.Since(start)) }()

	rows, err := s.db.QueryContext(ctx,
		`DELETE FROM files WHERE deleted_at IS NOT NULL
		 RETURNING s3_key, storage_location_id, group_id`)
	if err != nil {
		return nil, fmt.Errorf("purge all trash: %w", err)
	}
	defer rows.Close()

	var purged []PurgeFileRow
	for rows.Next() {
		var p PurgeFileRow
		var slid, gid sql.NullInt64
		if err := rows.Scan(&p.S3Key, &slid, &gid); err != nil {
			return nil, fmt.Errorf("scan purge: %w", err)
		}
		if slid.Valid {
			id := int(slid.Int64)
			p.StorageLocID = &id
		}
		if gid.Valid {
			id := int(gid.Int64)
			p.GroupID = &id
		}
		purged = append(purged, p)
	}
	return purged, rows.Err()
}

// PurgeExpiredTrash permanently deletes trash items older than maxAge. Returns storage info.
func (s *Store) PurgeExpiredTrash(ctx context.Context, maxAge time.Duration) ([]PurgeFileRow, error) {
	start := time.Now()
	defer func() { metrics.RecordDBQuery("purge_expired_trash", time.Since(start)) }()

	cutoff := time.Now().Add(-maxAge)
	rows, err := s.db.QueryContext(ctx,
		`DELETE FROM files WHERE deleted_at IS NOT NULL AND deleted_at < $1
		 RETURNING s3_key, storage_location_id, group_id`, cutoff)
	if err != nil {
		return nil, fmt.Errorf("purge expired trash: %w", err)
	}
	defer rows.Close()

	var purged []PurgeFileRow
	for rows.Next() {
		var p PurgeFileRow
		var slid, gid sql.NullInt64
		if err := rows.Scan(&p.S3Key, &slid, &gid); err != nil {
			return nil, fmt.Errorf("scan purge: %w", err)
		}
		if slid.Valid {
			id := int(slid.Int64)
			p.StorageLocID = &id
		}
		if gid.Valid {
			id := int(gid.Int64)
			p.GroupID = &id
		}
		purged = append(purged, p)
	}
	return purged, rows.Err()
}

// ─── Favorites ───────────────────────────────────────────────────────────────

// AddFavorite adds a file to the user's favorites.
func (s *Store) AddFavorite(ctx context.Context, userID int, filePath string) error {
	start := time.Now()
	defer func() { metrics.RecordDBQuery("add_favorite", time.Since(start)) }()

	filePath = normalizePath(filePath)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO user_favorites (user_id, file_path) VALUES ($1, $2)
		 ON CONFLICT (user_id, file_path) DO NOTHING`,
		userID, filePath)
	if err != nil {
		return fmt.Errorf("add favorite: %w", err)
	}
	return nil
}

// RemoveFavorite removes a file from the user's favorites.
func (s *Store) RemoveFavorite(ctx context.Context, userID int, filePath string) error {
	start := time.Now()
	defer func() { metrics.RecordDBQuery("remove_favorite", time.Since(start)) }()

	filePath = normalizePath(filePath)
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM user_favorites WHERE user_id = $1 AND file_path = $2`,
		userID, filePath)
	if err != nil {
		return fmt.Errorf("remove favorite: %w", err)
	}
	return nil
}

// FavoriteRow holds favorite info joined with file metadata.
type FavoriteRow struct {
	FilePath string
	FileName string
	Size     int64
	IsDir    bool
	ModTime  time.Time
}

// ListFavorites returns all favorites for a user, joined with file metadata.
func (s *Store) ListFavorites(ctx context.Context, userID int) ([]FavoriteRow, error) {
	start := time.Now()
	defer func() { metrics.RecordDBQuery("list_favorites", time.Since(start)) }()

	rows, err := s.db.QueryContext(ctx,
		`SELECT uf.file_path, COALESCE(f.name, ''), COALESCE(f.size, 0),
		        COALESCE(f.is_dir, FALSE), COALESCE(f.mod_time, NOW())
		 FROM user_favorites uf
		 LEFT JOIN files f ON f.path = uf.file_path AND f.deleted_at IS NULL
		 WHERE uf.user_id = $1
		 ORDER BY uf.created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list favorites: %w", err)
	}
	defer rows.Close()

	var items []FavoriteRow
	for rows.Next() {
		var f FavoriteRow
		if err := rows.Scan(&f.FilePath, &f.FileName, &f.Size, &f.IsDir, &f.ModTime); err != nil {
			return nil, fmt.Errorf("scan favorite: %w", err)
		}
		items = append(items, f)
	}
	return items, rows.Err()
}

// ListFavoritePaths returns just the paths of a user's favorites (for star rendering).
func (s *Store) ListFavoritePaths(ctx context.Context, userID int) ([]string, error) {
	start := time.Now()
	defer func() { metrics.RecordDBQuery("list_favorite_paths", time.Since(start)) }()

	rows, err := s.db.QueryContext(ctx,
		`SELECT file_path FROM user_favorites WHERE user_id = $1`, userID)
	if err != nil {
		return nil, fmt.Errorf("list favorite paths: %w", err)
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, fmt.Errorf("scan favorite path: %w", err)
		}
		paths = append(paths, p)
	}
	return paths, rows.Err()
}

// ─── Search ──────────────────────────────────────────────────────────────────

// SearchResultRow holds a file search result.
type SearchResultRow struct {
	ID      string
	Name    string
	Path    string
	Size    int64
	IsDir   bool
	ModTime time.Time
	Tags    []string
}

// SearchFiles searches files by name, path, or tags.
func (s *Store) SearchFiles(ctx context.Context, query, typeFilter string, limit int) ([]SearchResultRow, error) {
	start := time.Now()
	defer func() { metrics.RecordDBQuery("search_files", time.Since(start)) }()

	if limit <= 0 || limit > 200 {
		limit = 200
	}

	baseQuery := `SELECT DISTINCT f.id, f.name, f.path, f.size, f.is_dir, f.mod_time
	              FROM files f
	              LEFT JOIN image_tags it ON it.file_path = f.path
	              WHERE f.deleted_at IS NULL
	              AND (f.name ILIKE '%' || $1 || '%' OR f.path ILIKE '%' || $1 || '%' OR it.tag ILIKE '%' || $1 || '%')`

	switch typeFilter {
	case "files":
		baseQuery += ` AND NOT f.is_dir`
	case "dirs":
		baseQuery += ` AND f.is_dir`
	case "images":
		baseQuery += ` AND lower(f.name) ~ '\.(jpg|jpeg|png|gif|webp|bmp|svg)$'`
	}

	baseQuery += ` ORDER BY f.mod_time DESC LIMIT $2`

	rows, err := s.db.QueryContext(ctx, baseQuery, query, limit)
	if err != nil {
		return nil, fmt.Errorf("search files: %w", err)
	}
	defer rows.Close()

	var results []SearchResultRow
	for rows.Next() {
		var r SearchResultRow
		if err := rows.Scan(&r.ID, &r.Name, &r.Path, &r.Size, &r.IsDir, &r.ModTime); err != nil {
			return nil, fmt.Errorf("scan search: %w", err)
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// ─── Move & Copy ─────────────────────────────────────────────────────────────

// MoveFile moves a file or directory to a new path, updating children paths for directories.
func (s *Store) MoveFile(ctx context.Context, oldPath, newPath string) error {
	start := time.Now()
	defer func() { metrics.RecordDBQuery("move_file", time.Since(start)) }()

	oldPath = normalizePath(oldPath)
	newPath = normalizePath(newPath)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	newParent := filepath.Dir(newPath)
	if newParent == "." {
		newParent = "/"
	}
	newName := filepath.Base(newPath)

	// Update the file/directory itself
	_, err = tx.ExecContext(ctx,
		`UPDATE files SET path = $1, parent_path = $2, name = $3, id = $4, updated_at = NOW()
		 WHERE path = $5 AND deleted_at IS NULL`,
		newPath, newParent, newName, fileID(newPath), oldPath)
	if err != nil {
		return fmt.Errorf("move file: %w", err)
	}

	// Update all children paths (for directories)
	_, err = tx.ExecContext(ctx,
		`UPDATE files SET
		   path = $1 || substring(path from length($2) + 1),
		   parent_path = CASE
		     WHEN parent_path = $2 THEN $1
		     ELSE $1 || substring(parent_path from length($2) + 1)
		   END,
		   id = encode(sha256(($1 || substring(path from length($2) + 1))::bytea), 'hex')::varchar(16),
		   updated_at = NOW()
		 WHERE path LIKE $2 || '/%' AND deleted_at IS NULL`,
		newPath, oldPath)
	if err != nil {
		return fmt.Errorf("move children: %w", err)
	}

	return tx.Commit()
}

// CopyFileRow copies a file's metadata to a new path (does not copy storage objects).
func (s *Store) CopyFileRow(ctx context.Context, srcPath, dstPath string) error {
	start := time.Now()
	defer func() { metrics.RecordDBQuery("copy_file_row", time.Since(start)) }()

	srcPath = normalizePath(srcPath)
	dstPath = normalizePath(dstPath)

	dstParent := filepath.Dir(dstPath)
	if dstParent == "." {
		dstParent = "/"
	}
	dstName := filepath.Base(dstPath)
	dstS3Key := strings.TrimPrefix(dstPath, "/")

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO files (id, name, path, parent_path, size, mod_time, is_dir, hash, s3_key, version, owner_id, visibility, group_id, storage_location_id, created_at, updated_at)
		 SELECT $1, $2, $3, $4, size, NOW(), is_dir, hash, $5, 1, owner_id, visibility, group_id, storage_location_id, NOW(), NOW()
		 FROM files WHERE path = $6 AND deleted_at IS NULL
		 ON CONFLICT (path) DO NOTHING`,
		fileID(dstPath), dstName, dstPath, dstParent, dstS3Key, srcPath)
	if err != nil {
		return fmt.Errorf("copy file: %w", err)
	}
	return nil
}

// ─── Storage Dashboard Analytics ─────────────────────────────────────────

// UserStorageBreakdown is storage usage for a single user.
type UserStorageBreakdown struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	Size     int64  `json:"size"`
	Count    int    `json:"count"`
}

// GroupStorageBreakdown is storage usage for a single group.
type GroupStorageBreakdown struct {
	GroupID   int    `json:"group_id"`
	GroupName string `json:"group_name"`
	Size      int64  `json:"size"`
	Count     int    `json:"count"`
}

// TypeStorageBreakdown is storage usage for a file extension category.
type TypeStorageBreakdown struct {
	Extension string `json:"extension"`
	Category  string `json:"category"`
	Size      int64  `json:"size"`
	Count     int    `json:"count"`
}

// LocationStorageBreakdown is storage usage for a storage location.
type LocationStorageBreakdown struct {
	LocationID  int    `json:"location_id"`
	Name        string `json:"name"`
	BackendType string `json:"backend_type"`
	Size        int64  `json:"size"`
	Count       int    `json:"count"`
}

// VisibilityStorageBreakdown is storage usage by visibility setting.
type VisibilityStorageBreakdown struct {
	Visibility string `json:"visibility"`
	Size       int64  `json:"size"`
	Count      int    `json:"count"`
}

// StorageGrowthPoint is a daily data point for cumulative storage growth.
type StorageGrowthPoint struct {
	Date       string `json:"date"`
	TotalSize  int64  `json:"total_size"`
	TotalFiles int    `json:"total_files"`
}

// StorageByUser returns storage breakdown by user.
func (s *Store) StorageByUser(ctx context.Context) ([]UserStorageBreakdown, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT COALESCE(f.owner_id, 0), COALESCE(u.username, 'unknown'),
		        COALESCE(SUM(f.size), 0), COUNT(*)
		 FROM files f
		 LEFT JOIN users u ON u.id = f.owner_id
		 WHERE f.deleted_at IS NULL AND f.is_dir = FALSE
		 GROUP BY f.owner_id, u.username
		 ORDER BY SUM(f.size) DESC`)
	if err != nil {
		return nil, fmt.Errorf("storage by user: %w", err)
	}
	defer rows.Close()

	var result []UserStorageBreakdown
	for rows.Next() {
		var b UserStorageBreakdown
		if err := rows.Scan(&b.UserID, &b.Username, &b.Size, &b.Count); err != nil {
			return nil, err
		}
		result = append(result, b)
	}
	return result, rows.Err()
}

// StorageByGroup returns storage breakdown by group.
func (s *Store) StorageByGroup(ctx context.Context) ([]GroupStorageBreakdown, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT COALESCE(f.group_id, 0), COALESCE(g.name, 'No Group'),
		        COALESCE(SUM(f.size), 0), COUNT(*)
		 FROM files f
		 LEFT JOIN groups g ON g.id = f.group_id
		 WHERE f.deleted_at IS NULL AND f.is_dir = FALSE
		 GROUP BY f.group_id, g.name
		 ORDER BY SUM(f.size) DESC`)
	if err != nil {
		return nil, fmt.Errorf("storage by group: %w", err)
	}
	defer rows.Close()

	var result []GroupStorageBreakdown
	for rows.Next() {
		var b GroupStorageBreakdown
		if err := rows.Scan(&b.GroupID, &b.GroupName, &b.Size, &b.Count); err != nil {
			return nil, err
		}
		result = append(result, b)
	}
	return result, rows.Err()
}

// StorageByFileType returns storage breakdown by file extension category.
func (s *Store) StorageByFileType(ctx context.Context) ([]TypeStorageBreakdown, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT LOWER(COALESCE(NULLIF(
		        SUBSTRING(name FROM '\.([^.]+)$'), ''), 'none')),
		        COALESCE(SUM(size), 0), COUNT(*)
		 FROM files
		 WHERE deleted_at IS NULL AND is_dir = FALSE
		 GROUP BY 1
		 ORDER BY SUM(size) DESC`)
	if err != nil {
		return nil, fmt.Errorf("storage by type: %w", err)
	}
	defer rows.Close()

	var result []TypeStorageBreakdown
	for rows.Next() {
		var b TypeStorageBreakdown
		if err := rows.Scan(&b.Extension, &b.Size, &b.Count); err != nil {
			return nil, err
		}
		b.Category = extensionCategory(b.Extension)
		result = append(result, b)
	}
	return result, rows.Err()
}

// StorageByLocation returns storage breakdown by storage location.
func (s *Store) StorageByLocation(ctx context.Context) ([]LocationStorageBreakdown, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT COALESCE(f.storage_location_id, 0),
		        COALESCE(sl.name, 'Default'),
		        COALESCE(sl.backend_type, 'local'),
		        COALESCE(SUM(f.size), 0), COUNT(*)
		 FROM files f
		 LEFT JOIN storage_locations sl ON sl.id = f.storage_location_id
		 WHERE f.deleted_at IS NULL AND f.is_dir = FALSE
		 GROUP BY f.storage_location_id, sl.name, sl.backend_type
		 ORDER BY SUM(f.size) DESC`)
	if err != nil {
		return nil, fmt.Errorf("storage by location: %w", err)
	}
	defer rows.Close()

	var result []LocationStorageBreakdown
	for rows.Next() {
		var b LocationStorageBreakdown
		if err := rows.Scan(&b.LocationID, &b.Name, &b.BackendType, &b.Size, &b.Count); err != nil {
			return nil, err
		}
		result = append(result, b)
	}
	return result, rows.Err()
}

// StorageByVisibility returns storage breakdown by visibility setting.
func (s *Store) StorageByVisibility(ctx context.Context) ([]VisibilityStorageBreakdown, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT COALESCE(NULLIF(visibility, ''), 'public'),
		        COALESCE(SUM(size), 0), COUNT(*)
		 FROM files
		 WHERE deleted_at IS NULL AND is_dir = FALSE
		 GROUP BY 1
		 ORDER BY SUM(size) DESC`)
	if err != nil {
		return nil, fmt.Errorf("storage by visibility: %w", err)
	}
	defer rows.Close()

	var result []VisibilityStorageBreakdown
	for rows.Next() {
		var b VisibilityStorageBreakdown
		if err := rows.Scan(&b.Visibility, &b.Size, &b.Count); err != nil {
			return nil, err
		}
		result = append(result, b)
	}
	return result, rows.Err()
}

// StorageGrowth returns cumulative storage growth over the given number of days.
func (s *Store) StorageGrowth(ctx context.Context, days int) ([]StorageGrowthPoint, error) {
	rows, err := s.db.QueryContext(ctx,
		`WITH daily AS (
		    SELECT DATE(created_at) AS d,
		           COALESCE(SUM(size), 0) AS day_size,
		           COUNT(*) AS day_files
		    FROM files
		    WHERE deleted_at IS NULL AND is_dir = FALSE
		      AND created_at >= NOW() - ($1 || ' days')::INTERVAL
		    GROUP BY DATE(created_at)
		 )
		 SELECT d::TEXT,
		        SUM(day_size) OVER (ORDER BY d) AS total_size,
		        SUM(day_files) OVER (ORDER BY d)::INT AS total_files
		 FROM daily
		 ORDER BY d`, days)
	if err != nil {
		return nil, fmt.Errorf("storage growth: %w", err)
	}
	defer rows.Close()

	var result []StorageGrowthPoint
	for rows.Next() {
		var p StorageGrowthPoint
		if err := rows.Scan(&p.Date, &p.TotalSize, &p.TotalFiles); err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

// TrashStats returns total size and count of trashed files.
func (s *Store) TrashStats(ctx context.Context) (int64, int, error) {
	var totalSize int64
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(size), 0), COUNT(*)
		 FROM files WHERE deleted_at IS NOT NULL AND is_dir = FALSE`).
		Scan(&totalSize, &count)
	return totalSize, count, err
}

// extensionCategory maps a file extension to a broad category.
func extensionCategory(ext string) string {
	switch ext {
	case "jpg", "jpeg", "png", "gif", "webp", "svg", "bmp", "tiff", "ico", "heic", "heif", "raw", "cr2", "nef":
		return "Images"
	case "mp4", "mov", "avi", "mkv", "webm", "flv", "wmv", "m4v":
		return "Videos"
	case "mp3", "wav", "flac", "aac", "ogg", "wma", "m4a":
		return "Audio"
	case "pdf", "doc", "docx", "xls", "xlsx", "ppt", "pptx", "odt", "ods", "txt", "rtf", "csv":
		return "Documents"
	case "zip", "tar", "gz", "bz2", "7z", "rar", "xz", "zst":
		return "Archives"
	case "go", "js", "ts", "py", "java", "c", "cpp", "h", "rs", "rb", "php", "html", "css", "json", "xml", "yaml", "yml", "toml", "sh", "sql":
		return "Code"
	default:
		return "Other"
	}
}

func fileID(path string) string {
	h := sha256.Sum256([]byte(path))
	return fmt.Sprintf("%x", h[:8])
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

// ─── Activity Log ───────────────────────────────────────────────────────────

// ActivityEntry represents a single activity log entry.
type ActivityEntry struct {
	ID           int64     `json:"id"`
	UserID       int       `json:"user_id"`
	Username     string    `json:"username"`
	Action       string    `json:"action"`
	ResourcePath string    `json:"resource_path"`
	Details      string    `json:"details"`
	CreatedAt    time.Time `json:"created_at"`
}

// GetActivity returns recent activity entries (all users, for admins).
func (s *Store) GetActivity(ctx context.Context, limit int, before *time.Time) ([]ActivityEntry, error) {
	var query string
	var args []interface{}

	if before != nil {
		query = `SELECT id, COALESCE(user_id, 0), username, action, resource_path, COALESCE(details::text, '{}'), created_at
		         FROM activity_log WHERE created_at < $1 ORDER BY created_at DESC LIMIT $2`
		args = []interface{}{*before, limit}
	} else {
		query = `SELECT id, COALESCE(user_id, 0), username, action, resource_path, COALESCE(details::text, '{}'), created_at
		         FROM activity_log ORDER BY created_at DESC LIMIT $1`
		args = []interface{}{limit}
	}

	return s.queryActivity(ctx, query, args...)
}

// GetUserActivity returns recent activity entries for a specific user.
func (s *Store) GetUserActivity(ctx context.Context, userID, limit int, before *time.Time) ([]ActivityEntry, error) {
	var query string
	var args []interface{}

	if before != nil {
		query = `SELECT id, COALESCE(user_id, 0), username, action, resource_path, COALESCE(details::text, '{}'), created_at
		         FROM activity_log WHERE user_id = $1 AND created_at < $2 ORDER BY created_at DESC LIMIT $3`
		args = []interface{}{userID, *before, limit}
	} else {
		query = `SELECT id, COALESCE(user_id, 0), username, action, resource_path, COALESCE(details::text, '{}'), created_at
		         FROM activity_log WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2`
		args = []interface{}{userID, limit}
	}

	return s.queryActivity(ctx, query, args...)
}

func (s *Store) queryActivity(ctx context.Context, query string, args ...interface{}) ([]ActivityEntry, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query activity: %w", err)
	}
	defer rows.Close()

	var entries []ActivityEntry
	for rows.Next() {
		var e ActivityEntry
		if err := rows.Scan(&e.ID, &e.UserID, &e.Username, &e.Action, &e.ResourcePath, &e.Details, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
