// Package sharing provides file permission and share link management.
package sharing

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/fruitsalade/fruitsalade/phase2/internal/metrics"
	"github.com/fruitsalade/fruitsalade/shared/pkg/models"
)

// PermissionStore manages file access permissions.
type PermissionStore struct {
	db     *sql.DB
	groups *GroupStore
}

// SetGroupStore sets the group store for group-based permission checks.
func (s *PermissionStore) SetGroupStore(gs *GroupStore) {
	s.groups = gs
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
// Now also checks group role-based access for files with group_id.
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
	segments := PathSegments(path)

	for _, seg := range segments {
		var perm string
		err := s.db.QueryRowContext(ctx,
			`SELECT permission FROM file_permissions
			 WHERE user_id = $1 AND path = $2`,
			userID, seg).Scan(&perm)
		if err != nil {
			continue
		}

		if PermissionSatisfies(perm, requiredPerm) {
			metrics.RecordPermissionCheck(true)
			return true
		}
	}

	// Check group role-based access for files with group_id
	if s.groups != nil {
		var groupID sql.NullInt64
		err := s.db.QueryRowContext(ctx,
			`SELECT group_id FROM files WHERE path = $1`, path).Scan(&groupID)
		if err == nil && groupID.Valid {
			role, err := s.groups.GetUserEffectiveRole(ctx, userID, int(groupID.Int64))
			if err == nil && role != "" {
				mappedPerm := RoleToPermission(role)
				if PermissionSatisfies(mappedPerm, requiredPerm) {
					metrics.RecordPermissionCheck(true)
					return true
				}
			}
		}
	}

	// Check group permissions (explicit path-based)
	if s.groups != nil {
		hasGroupAccess, err := s.groups.CheckGroupAccess(ctx, userID, path, requiredPerm)
		if err == nil && hasGroupAccess {
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

// ─── Visibility ─────────────────────────────────────────────────────────────

// CheckVisibility returns true if the user can see this node based on visibility.
func (s *PermissionStore) CheckVisibility(node *models.FileNode, userID int, isAdmin bool, userGroups map[int]string) bool {
	if isAdmin {
		return true
	}

	vis := node.Visibility
	if vis == "" || vis == "public" {
		return true
	}

	if vis == "private" {
		return node.OwnerID == userID
	}

	if vis == "group" {
		if node.GroupID == 0 {
			return true // no group_id set, treat as public
		}
		_, isMember := userGroups[node.GroupID]
		return isMember
	}

	return true
}

// SetVisibility sets the visibility of a file/folder.
func (s *PermissionStore) SetVisibility(ctx context.Context, path, visibility string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE files SET visibility = $2 WHERE path = $1`,
		path, visibility)
	if err != nil {
		return fmt.Errorf("set visibility: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("file not found: %s", path)
	}
	return nil
}

// GetVisibility returns the visibility of a file/folder.
func (s *PermissionStore) GetVisibility(ctx context.Context, path string) (string, error) {
	var vis string
	err := s.db.QueryRowContext(ctx,
		`SELECT visibility FROM files WHERE path = $1`, path).Scan(&vis)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("file not found: %s", path)
	}
	if err != nil {
		return "", fmt.Errorf("get visibility: %w", err)
	}
	return vis, nil
}

// GetUserPermissionsMap returns all file permissions for a user as a map[path]permission.
func (s *PermissionStore) GetUserPermissionsMap(ctx context.Context, userID int) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT path, permission FROM file_permissions WHERE user_id = $1`, userID)
	if err != nil {
		return nil, fmt.Errorf("get user permissions map: %w", err)
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var path, perm string
		if err := rows.Scan(&path, &perm); err != nil {
			return nil, err
		}
		result[path] = perm
	}
	return result, rows.Err()
}

// ─── Helpers ────────────────────────────────────────────────────────────────

// PathSegments returns all path prefixes from most specific to least.
// "/a/b/c" -> ["/a/b/c", "/a/b", "/a", "/"]
func PathSegments(path string) []string {
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

// PermissionSatisfies checks if `has` satisfies `required`.
// owner > write > read
func PermissionSatisfies(has, required string) bool {
	levels := map[string]int{"read": 1, "write": 2, "owner": 3}
	return levels[has] >= levels[required]
}
