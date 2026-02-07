// Package postgres provides a PostgreSQL-backed metadata store.
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

	"github.com/fruitsalade/fruitsalade/shared/pkg/logger"
	"github.com/fruitsalade/fruitsalade/shared/pkg/models"
)

// Store is a PostgreSQL metadata store.
type Store struct {
	db *sql.DB
}

// FileRow maps to the files table.
type FileRow struct {
	ID         string
	Name       string
	Path       string
	ParentPath string
	Size       int64
	ModTime    time.Time
	IsDir      bool
	Hash       string
	S3Key      string
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

// Migrate runs SQL migration files.
func (s *Store) Migrate(migrationsDir string) error {
	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.up.sql"))
	if err != nil {
		return fmt.Errorf("glob migrations: %w", err)
	}

	for _, f := range files {
		logger.Info("Running migration: %s", filepath.Base(f))
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
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, path, parent_path, size, mod_time, is_dir, hash, s3_key
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
		if err := rows.Scan(&r.ID, &r.Name, &r.Path, &r.ParentPath,
			&r.Size, &r.ModTime, &r.IsDir, &r.Hash, &r.S3Key); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		allRows = append(allRows, r)
		nodeMap[r.Path] = rowToNode(&r)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

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

	return root, nil
}

// GetMetadata returns metadata for a single path.
func (s *Store) GetMetadata(ctx context.Context, path string) (*models.FileNode, error) {
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

// ListDir returns children of a directory.
func (s *Store) ListDir(ctx context.Context, path string) ([]*models.FileNode, error) {
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
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO files (id, name, path, parent_path, size, mod_time, is_dir, hash, s3_key, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
		 ON CONFLICT (path) DO UPDATE SET
			name = EXCLUDED.name,
			size = EXCLUDED.size,
			mod_time = EXCLUDED.mod_time,
			hash = EXCLUDED.hash,
			s3_key = EXCLUDED.s3_key,
			updated_at = NOW()`,
		f.ID, f.Name, f.Path, f.ParentPath, f.Size, f.ModTime, f.IsDir, f.Hash, f.S3Key)
	if err != nil {
		return fmt.Errorf("upsert: %w", err)
	}
	return nil
}

// DeleteFile removes a file entry.
func (s *Store) DeleteFile(ctx context.Context, path string) error {
	path = normalizePath(path)
	_, err := s.db.ExecContext(ctx, `DELETE FROM files WHERE path = $1`, path)
	return err
}

// FileCount returns the total number of file entries.
func (s *Store) FileCount(ctx context.Context) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM files`).Scan(&count)
	return count, err
}

func rowToNode(r *FileRow) *models.FileNode {
	return &models.FileNode{
		ID:      r.ID,
		Name:    r.Name,
		Path:    r.Path,
		Size:    r.Size,
		ModTime: r.ModTime,
		IsDir:   r.IsDir,
		Hash:    r.Hash,
	}
}

func normalizePath(path string) string {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return strings.TrimSuffix(path, "/")
}
