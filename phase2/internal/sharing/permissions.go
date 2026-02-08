// Package sharing provides file permission and share link management.
package sharing

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/fruitsalade/fruitsalade/phase2/internal/metrics"
)

// PermissionStore manages file access permissions.
type PermissionStore struct {
	db *sql.DB
}

// NewPermissionStore creates a new permission store.
func NewPermissionStore(db *sql.DB) *PermissionStore {
	return &PermissionStore{db: db}
}

// Permission represents a file permission entry.
type Permission struct {
	ID         int
	UserID     int
	Username   string
	Path       string
	Permission string // "owner", "read", "write"
}

// SetPermission grants a permission for a user on a path.
func (s *PermissionStore) SetPermission(ctx context.Context, userID int, path, permission string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO file_permissions (user_id, path, permission)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (user_id, path) DO UPDATE SET permission = EXCLUDED.permission`,
		userID, path, permission)
	if err != nil {
		return fmt.Errorf("set permission: %w", err)
	}
	return nil
}

// RemovePermission removes a user's permission on a path.
func (s *PermissionStore) RemovePermission(ctx context.Context, userID int, path string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM file_permissions WHERE user_id = $1 AND path = $2`,
		userID, path)
	if err != nil {
		return fmt.Errorf("remove permission: %w", err)
	}
	return nil
}

// ListPermissions returns all permissions for a path.
func (s *PermissionStore) ListPermissions(ctx context.Context, path string) ([]Permission, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT fp.id, fp.user_id, u.username, fp.path, fp.permission
		 FROM file_permissions fp
		 JOIN users u ON u.id = fp.user_id
		 WHERE fp.path = $1
		 ORDER BY u.username`, path)
	if err != nil {
		return nil, fmt.Errorf("list permissions: %w", err)
	}
	defer rows.Close()

	var perms []Permission
	for rows.Next() {
		var p Permission
		if err := rows.Scan(&p.ID, &p.UserID, &p.Username, &p.Path, &p.Permission); err != nil {
			return nil, fmt.Errorf("scan permission: %w", err)
		}
		perms = append(perms, p)
	}
	return perms, rows.Err()
}

// CheckAccess checks if a user has at least the given permission on a path.
// Supports path inheritance: permission on "/docs" grants access to "/docs/readme.md".
// Admins always have access. File owners always have access.
func (s *PermissionStore) CheckAccess(ctx context.Context, userID int, path string, requiredPerm string, isAdmin bool) bool {
	// Admins bypass all checks
	if isAdmin {
		metrics.RecordPermissionCheck(true)
		return true
	}

	// Check if user is the file owner
	var ownerID sql.NullInt64
	err := s.db.QueryRowContext(ctx,
		`SELECT owner_id FROM files WHERE path = $1`, path).Scan(&ownerID)
	if err == nil && ownerID.Valid && int(ownerID.Int64) == userID {
		metrics.RecordPermissionCheck(true)
		return true
	}

	// Check direct and inherited permissions
	// Build path segments: /a/b/c -> ["/a/b/c", "/a/b", "/a", "/"]
	segments := pathSegments(path)

	for _, seg := range segments {
		var perm string
		err := s.db.QueryRowContext(ctx,
			`SELECT permission FROM file_permissions
			 WHERE user_id = $1 AND path = $2`,
			userID, seg).Scan(&perm)
		if err != nil {
			continue
		}

		if permissionSatisfies(perm, requiredPerm) {
			metrics.RecordPermissionCheck(true)
			return true
		}
	}

	metrics.RecordPermissionCheck(false)
	return false
}

// GetOwnerID returns the owner_id for a file path.
func (s *PermissionStore) GetOwnerID(ctx context.Context, path string) (int, bool) {
	var ownerID sql.NullInt64
	err := s.db.QueryRowContext(ctx,
		`SELECT owner_id FROM files WHERE path = $1`, path).Scan(&ownerID)
	if err != nil || !ownerID.Valid {
		return 0, false
	}
	return int(ownerID.Int64), true
}

// pathSegments returns all path prefixes from most specific to least.
// "/a/b/c" -> ["/a/b/c", "/a/b", "/a", "/"]
func pathSegments(path string) []string {
	segments := []string{path}
	for {
		idx := strings.LastIndex(path, "/")
		if idx <= 0 {
			if path != "/" {
				segments = append(segments, "/")
			}
			break
		}
		path = path[:idx]
		segments = append(segments, path)
	}
	return segments
}

// permissionSatisfies checks if `has` satisfies `required`.
// owner > write > read
func permissionSatisfies(has, required string) bool {
	levels := map[string]int{"read": 1, "write": 2, "owner": 3}
	return levels[has] >= levels[required]
}
