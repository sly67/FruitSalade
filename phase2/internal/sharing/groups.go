package sharing

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/fruitsalade/fruitsalade/shared/pkg/protocol"
)

// Group represents a user group.
type Group struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	ParentID    *int      `json:"parent_id,omitempty"`
	ParentName  string    `json:"parent_name,omitempty"`
	CreatedBy   *int      `json:"created_by,omitempty"`
	CreatorName string    `json:"creator_name,omitempty"`
	MemberCount int       `json:"member_count"`
	CreatedAt   time.Time `json:"created_at"`
}

// GroupMember represents a member of a group.
type GroupMember struct {
	UserID   int       `json:"user_id"`
	Username string    `json:"username"`
	Role     string    `json:"role"` // "admin", "editor", "viewer"
	AddedAt  time.Time `json:"added_at"`
}

// GroupPermission represents a permission entry for a group.
type GroupPermission struct {
	ID         int    `json:"id"`
	GroupID    int    `json:"group_id"`
	GroupName  string `json:"group_name,omitempty"`
	Path       string `json:"path"`
	Permission string `json:"permission"`
}

// GroupStore manages user groups, membership, and group permissions.
type GroupStore struct {
	db *sql.DB
}

// NewGroupStore creates a new group store.
func NewGroupStore(db *sql.DB) *GroupStore {
	return &GroupStore{db: db}
}

// DB returns the underlying database connection.
func (s *GroupStore) DB() *sql.DB {
	return s.db
}

// CreateGroup creates a new group with optional parent.
func (s *GroupStore) CreateGroup(ctx context.Context, name, description string, parentID *int, createdBy int) (*Group, error) {
	var g Group
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO groups (name, description, parent_id, created_by)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, name, description, parent_id, created_by, created_at`,
		name, description, parentID, createdBy).Scan(&g.ID, &g.Name, &g.Description, &g.ParentID, &g.CreatedBy, &g.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create group: %w", err)
	}
	return &g, nil
}

// DeleteGroup deletes a group (CASCADE removes members, permissions, and children).
func (s *GroupStore) DeleteGroup(ctx context.Context, groupID int) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM groups WHERE id = $1`, groupID)
	if err != nil {
		return fmt.Errorf("delete group: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("group not found")
	}
	return nil
}

// ListGroups returns all groups with member count, creator name, and parent info.
func (s *GroupStore) ListGroups(ctx context.Context) ([]Group, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT g.id, g.name, g.description, g.parent_id, p.name,
		        g.created_by, u.username,
		        (SELECT COUNT(*) FROM group_members gm WHERE gm.group_id = g.id) AS member_count,
		        g.created_at
		 FROM groups g
		 LEFT JOIN users u ON u.id = g.created_by
		 LEFT JOIN groups p ON p.id = g.parent_id
		 ORDER BY g.name`)
	if err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}
	defer rows.Close()

	var groups []Group
	for rows.Next() {
		var g Group
		var creatorName, parentName sql.NullString
		if err := rows.Scan(&g.ID, &g.Name, &g.Description, &g.ParentID, &parentName,
			&g.CreatedBy, &creatorName, &g.MemberCount, &g.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan group: %w", err)
		}
		if creatorName.Valid {
			g.CreatorName = creatorName.String
		}
		if parentName.Valid {
			g.ParentName = parentName.String
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}

// GetGroup returns a single group by ID.
func (s *GroupStore) GetGroup(ctx context.Context, groupID int) (*Group, error) {
	var g Group
	var creatorName, parentName sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT g.id, g.name, g.description, g.parent_id, p.name,
		        g.created_by, u.username,
		        (SELECT COUNT(*) FROM group_members gm WHERE gm.group_id = g.id) AS member_count,
		        g.created_at
		 FROM groups g
		 LEFT JOIN users u ON u.id = g.created_by
		 LEFT JOIN groups p ON p.id = g.parent_id
		 WHERE g.id = $1`, groupID).Scan(&g.ID, &g.Name, &g.Description, &g.ParentID, &parentName,
		&g.CreatedBy, &creatorName, &g.MemberCount, &g.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("group not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get group: %w", err)
	}
	if creatorName.Valid {
		g.CreatorName = creatorName.String
	}
	if parentName.Valid {
		g.ParentName = parentName.String
	}
	return &g, nil
}

// AddMember adds a user to a group with a role.
func (s *GroupStore) AddMember(ctx context.Context, groupID, userID int, role string) error {
	if role == "" {
		role = "viewer"
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO group_members (group_id, user_id, role)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (group_id, user_id) DO UPDATE SET role = EXCLUDED.role`,
		groupID, userID, role)
	if err != nil {
		return fmt.Errorf("add member: %w", err)
	}
	return nil
}

// RemoveMember removes a user from a group.
func (s *GroupStore) RemoveMember(ctx context.Context, groupID, userID int) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM group_members WHERE group_id = $1 AND user_id = $2`,
		groupID, userID)
	if err != nil {
		return fmt.Errorf("remove member: %w", err)
	}
	return nil
}

// ListMembers returns all members of a group with roles.
func (s *GroupStore) ListMembers(ctx context.Context, groupID int) ([]GroupMember, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT gm.user_id, u.username, gm.role, gm.added_at
		 FROM group_members gm
		 JOIN users u ON u.id = gm.user_id
		 WHERE gm.group_id = $1
		 ORDER BY u.username`, groupID)
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}
	defer rows.Close()

	var members []GroupMember
	for rows.Next() {
		var m GroupMember
		if err := rows.Scan(&m.UserID, &m.Username, &m.Role, &m.AddedAt); err != nil {
			return nil, fmt.Errorf("scan member: %w", err)
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

// GetUserGroups returns all groups a user belongs to.
func (s *GroupStore) GetUserGroups(ctx context.Context, userID int) ([]Group, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT g.id, g.name, g.description, g.parent_id,
		        (SELECT COUNT(*) FROM group_members gm2 WHERE gm2.group_id = g.id) AS member_count,
		        g.created_at
		 FROM groups g
		 JOIN group_members gm ON gm.group_id = g.id
		 WHERE gm.user_id = $1
		 ORDER BY g.name`, userID)
	if err != nil {
		return nil, fmt.Errorf("get user groups: %w", err)
	}
	defer rows.Close()

	var groups []Group
	for rows.Next() {
		var g Group
		if err := rows.Scan(&g.ID, &g.Name, &g.Description, &g.ParentID, &g.MemberCount, &g.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan group: %w", err)
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}

// UserGroupMembership represents a group a user belongs to, with their role.
type UserGroupMembership struct {
	GroupID   int    `json:"group_id"`
	GroupName string `json:"group_name"`
	Role      string `json:"role"`
}

// GetUserGroupsWithRoles returns the groups a user belongs to, with their role in each.
func (s *GroupStore) GetUserGroupsWithRoles(ctx context.Context, userID int) ([]UserGroupMembership, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT g.id, g.name, gm.role
		 FROM groups g
		 JOIN group_members gm ON gm.group_id = g.id
		 WHERE gm.user_id = $1
		 ORDER BY g.name`, userID)
	if err != nil {
		return nil, fmt.Errorf("get user groups with roles: %w", err)
	}
	defer rows.Close()

	var memberships []UserGroupMembership
	for rows.Next() {
		var m UserGroupMembership
		if err := rows.Scan(&m.GroupID, &m.GroupName, &m.Role); err != nil {
			return nil, fmt.Errorf("scan membership: %w", err)
		}
		memberships = append(memberships, m)
	}
	return memberships, rows.Err()
}

// ─── Hierarchy Methods ──────────────────────────────────────────────────────

// GetGroupTree returns the full nested group tree for admin UI.
func (s *GroupStore) GetGroupTree(ctx context.Context) ([]protocol.GroupTreeNode, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT g.id, g.name, g.description, g.parent_id,
		        (SELECT COUNT(*) FROM group_members gm WHERE gm.group_id = g.id) AS member_count
		 FROM groups g
		 ORDER BY g.name`)
	if err != nil {
		return nil, fmt.Errorf("get group tree: %w", err)
	}
	defer rows.Close()

	type flatGroup struct {
		ID          int
		Name        string
		Description string
		ParentID    *int
		MemberCount int
	}

	var all []flatGroup
	for rows.Next() {
		var fg flatGroup
		if err := rows.Scan(&fg.ID, &fg.Name, &fg.Description, &fg.ParentID, &fg.MemberCount); err != nil {
			return nil, fmt.Errorf("scan group tree: %w", err)
		}
		all = append(all, fg)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Build tree from flat list
	childrenMap := make(map[int][]int) // parentID -> child indices
	idxMap := make(map[int]int)        // groupID -> index in all
	for i, fg := range all {
		idxMap[fg.ID] = i
		pid := 0
		if fg.ParentID != nil {
			pid = *fg.ParentID
		}
		childrenMap[pid] = append(childrenMap[pid], i)
	}

	var buildTree func(parentID int) []protocol.GroupTreeNode
	buildTree = func(parentID int) []protocol.GroupTreeNode {
		indices := childrenMap[parentID]
		if len(indices) == 0 {
			return nil
		}
		var nodes []protocol.GroupTreeNode
		for _, idx := range indices {
			fg := all[idx]
			node := protocol.GroupTreeNode{
				ID:          fg.ID,
				Name:        fg.Name,
				Description: fg.Description,
				MemberCount: fg.MemberCount,
				Children:    buildTree(fg.ID),
			}
			nodes = append(nodes, node)
		}
		return nodes
	}

	return buildTree(0), nil
}

// GetDescendantGroupIDs returns all child/grandchild group IDs (recursive).
func (s *GroupStore) GetDescendantGroupIDs(ctx context.Context, groupID int) ([]int, error) {
	rows, err := s.db.QueryContext(ctx,
		`WITH RECURSIVE descendants AS (
			SELECT id FROM groups WHERE parent_id = $1
			UNION ALL
			SELECT g.id FROM groups g JOIN descendants d ON g.parent_id = d.id
		)
		SELECT id FROM descendants`, groupID)
	if err != nil {
		return nil, fmt.Errorf("get descendants: %w", err)
	}
	defer rows.Close()

	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// GetAncestorGroupIDs returns all parent/grandparent group IDs (recursive).
func (s *GroupStore) GetAncestorGroupIDs(ctx context.Context, groupID int) ([]int, error) {
	rows, err := s.db.QueryContext(ctx,
		`WITH RECURSIVE ancestors AS (
			SELECT parent_id FROM groups WHERE id = $1 AND parent_id IS NOT NULL
			UNION ALL
			SELECT g.parent_id FROM groups g JOIN ancestors a ON g.id = a.parent_id WHERE g.parent_id IS NOT NULL
		)
		SELECT parent_id FROM ancestors`, groupID)
	if err != nil {
		return nil, fmt.Errorf("get ancestors: %w", err)
	}
	defer rows.Close()

	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// GetTopLevelGroup walks up to find the root group for a given group.
func (s *GroupStore) GetTopLevelGroup(ctx context.Context, groupID int) (*Group, error) {
	var topID int
	err := s.db.QueryRowContext(ctx,
		`WITH RECURSIVE chain AS (
			SELECT id, parent_id FROM groups WHERE id = $1
			UNION ALL
			SELECT g.id, g.parent_id FROM groups g JOIN chain c ON g.id = c.parent_id
		)
		SELECT id FROM chain WHERE parent_id IS NULL`, groupID).Scan(&topID)
	if err != nil {
		return nil, fmt.Errorf("get top-level group: %w", err)
	}
	return s.GetGroup(ctx, topID)
}

// GetUserRoleInGroup returns the user's direct role in a group.
func (s *GroupStore) GetUserRoleInGroup(ctx context.Context, userID, groupID int) (string, error) {
	var role string
	err := s.db.QueryRowContext(ctx,
		`SELECT role FROM group_members WHERE user_id = $1 AND group_id = $2`,
		userID, groupID).Scan(&role)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get user role: %w", err)
	}
	return role, nil
}

// GetUserEffectiveRole returns the most permissive role across a group and all its ancestors.
func (s *GroupStore) GetUserEffectiveRole(ctx context.Context, userID, groupID int) (string, error) {
	// Check direct membership
	bestRole := ""
	directRole, err := s.GetUserRoleInGroup(ctx, userID, groupID)
	if err != nil {
		return "", err
	}
	if directRole != "" {
		bestRole = directRole
	}

	// Check ancestor groups
	ancestors, err := s.GetAncestorGroupIDs(ctx, groupID)
	if err != nil {
		return bestRole, nil // fallback to direct role
	}

	for _, aid := range ancestors {
		role, err := s.GetUserRoleInGroup(ctx, userID, aid)
		if err != nil || role == "" {
			continue
		}
		if roleLevel(role) > roleLevel(bestRole) {
			bestRole = role
		}
	}

	return bestRole, nil
}

// UpdateMemberRole updates a member's role in a group.
func (s *GroupStore) UpdateMemberRole(ctx context.Context, groupID, userID int, role string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE group_members SET role = $1 WHERE group_id = $2 AND user_id = $3`,
		role, groupID, userID)
	if err != nil {
		return fmt.Errorf("update member role: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("member not found in group")
	}
	return nil
}

// IsGroupAdmin checks if a user is admin in the group or any ancestor group.
func (s *GroupStore) IsGroupAdmin(ctx context.Context, userID, groupID int) (bool, error) {
	role, err := s.GetUserEffectiveRole(ctx, userID, groupID)
	if err != nil {
		return false, err
	}
	return role == "admin", nil
}

// GetUserGroupsMap returns all group IDs -> role for a user (for efficient bulk checks).
func (s *GroupStore) GetUserGroupsMap(ctx context.Context, userID int) (map[int]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT group_id, role FROM group_members WHERE user_id = $1`, userID)
	if err != nil {
		return nil, fmt.Errorf("get user groups map: %w", err)
	}
	defer rows.Close()

	result := make(map[int]string)
	for rows.Next() {
		var gid int
		var role string
		if err := rows.Scan(&gid, &role); err != nil {
			return nil, err
		}
		result[gid] = role
	}
	return result, rows.Err()
}

// MoveGroup changes a group's parent (DB trigger prevents cycles).
func (s *GroupStore) MoveGroup(ctx context.Context, groupID int, newParentID *int) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE groups SET parent_id = $1 WHERE id = $2`,
		newParentID, groupID)
	if err != nil {
		return fmt.Errorf("move group: %w", err)
	}
	return nil
}

// GroupCount returns the total number of groups.
func (s *GroupStore) GroupCount(ctx context.Context) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM groups`).Scan(&count)
	return count, err
}

// GetGroupNameByID returns a group's name by ID.
func (s *GroupStore) GetGroupNameByID(ctx context.Context, groupID int) (string, error) {
	var name string
	err := s.db.QueryRowContext(ctx,
		`SELECT name FROM groups WHERE id = $1`, groupID).Scan(&name)
	if err != nil {
		return "", fmt.Errorf("get group name: %w", err)
	}
	return name, nil
}

// GetUsernameByID returns a username by user ID.
func (s *GroupStore) GetUsernameByID(ctx context.Context, userID int) (string, error) {
	var name string
	err := s.db.QueryRowContext(ctx,
		`SELECT username FROM users WHERE id = $1`, userID).Scan(&name)
	if err != nil {
		return "", fmt.Errorf("get username: %w", err)
	}
	return name, nil
}

// ─── Permissions ────────────────────────────────────────────────────────────

// SetPermission sets a permission for a group on a path (upsert).
func (s *GroupStore) SetPermission(ctx context.Context, groupID int, path, permission string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO group_permissions (group_id, path, permission)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (group_id, path) DO UPDATE SET permission = EXCLUDED.permission`,
		groupID, path, permission)
	if err != nil {
		return fmt.Errorf("set group permission: %w", err)
	}
	return nil
}

// RemovePermission removes a group's permission on a path.
func (s *GroupStore) RemovePermission(ctx context.Context, groupID int, path string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM group_permissions WHERE group_id = $1 AND path = $2`,
		groupID, path)
	if err != nil {
		return fmt.Errorf("remove group permission: %w", err)
	}
	return nil
}

// ListPermissionsByGroup returns all permissions for a group.
func (s *GroupStore) ListPermissionsByGroup(ctx context.Context, groupID int) ([]GroupPermission, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT gp.id, gp.group_id, g.name, gp.path, gp.permission
		 FROM group_permissions gp
		 JOIN groups g ON g.id = gp.group_id
		 WHERE gp.group_id = $1
		 ORDER BY gp.path`, groupID)
	if err != nil {
		return nil, fmt.Errorf("list group permissions: %w", err)
	}
	defer rows.Close()

	var perms []GroupPermission
	for rows.Next() {
		var p GroupPermission
		if err := rows.Scan(&p.ID, &p.GroupID, &p.GroupName, &p.Path, &p.Permission); err != nil {
			return nil, fmt.Errorf("scan permission: %w", err)
		}
		perms = append(perms, p)
	}
	return perms, rows.Err()
}

// ListPermissionsByPath returns all group permissions for a given path.
func (s *GroupStore) ListPermissionsByPath(ctx context.Context, path string) ([]GroupPermission, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT gp.id, gp.group_id, g.name, gp.path, gp.permission
		 FROM group_permissions gp
		 JOIN groups g ON g.id = gp.group_id
		 WHERE gp.path = $1
		 ORDER BY g.name`, path)
	if err != nil {
		return nil, fmt.Errorf("list permissions by path: %w", err)
	}
	defer rows.Close()

	var perms []GroupPermission
	for rows.Next() {
		var p GroupPermission
		if err := rows.Scan(&p.ID, &p.GroupID, &p.GroupName, &p.Path, &p.Permission); err != nil {
			return nil, fmt.Errorf("scan permission: %w", err)
		}
		perms = append(perms, p)
	}
	return perms, rows.Err()
}

// CheckGroupAccess checks if any of the user's groups grant the required permission
// on the given path, using path inheritance.
func (s *GroupStore) CheckGroupAccess(ctx context.Context, userID int, path string, requiredPerm string) (bool, error) {
	// Get all group IDs for this user
	rows, err := s.db.QueryContext(ctx,
		`SELECT group_id FROM group_members WHERE user_id = $1`, userID)
	if err != nil {
		return false, fmt.Errorf("get user groups: %w", err)
	}
	defer rows.Close()

	var groupIDs []int
	for rows.Next() {
		var gid int
		if err := rows.Scan(&gid); err != nil {
			return false, err
		}
		groupIDs = append(groupIDs, gid)
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	if len(groupIDs) == 0 {
		return false, nil
	}

	// Check each path segment for any group permission
	segments := PathSegments(path)
	for _, seg := range segments {
		for _, gid := range groupIDs {
			var perm string
			err := s.db.QueryRowContext(ctx,
				`SELECT permission FROM group_permissions
				 WHERE group_id = $1 AND path = $2`,
				gid, seg).Scan(&perm)
			if err != nil {
				continue
			}
			if PermissionSatisfies(perm, requiredPerm) {
				return true, nil
			}
		}
	}

	return false, nil
}

// ─── Role helpers ───────────────────────────────────────────────────────────

// roleLevel returns the numeric level of a role for comparison.
func roleLevel(role string) int {
	switch role {
	case "admin":
		return 3
	case "editor":
		return 2
	case "viewer":
		return 1
	default:
		return 0
	}
}

// RoleToPermission maps a group role to file permission level.
func RoleToPermission(role string) string {
	switch role {
	case "admin":
		return "write"
	case "editor":
		return "write"
	case "viewer":
		return "read"
	default:
		return ""
	}
}
