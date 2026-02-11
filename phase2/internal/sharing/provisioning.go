package sharing

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/fruitsalade/fruitsalade/phase2/internal/logging"
	"github.com/fruitsalade/fruitsalade/phase2/internal/metadata/postgres"
)

// Provisioner handles auto-provisioning of group folders and user home directories.
type Provisioner struct {
	groups *GroupStore
	meta   *postgres.Store
	perms  *PermissionStore
}

// NewProvisioner creates a new Provisioner.
func NewProvisioner(groups *GroupStore, meta *postgres.Store, perms *PermissionStore) *Provisioner {
	return &Provisioner{
		groups: groups,
		meta:   meta,
		perms:  perms,
	}
}

// ProvisionGroupFolders creates the group directory and shared/ subdirectory.
// For top-level groups: /{group_name}/ and /{group_name}/shared/
// For subgroups: resolves full path from top-level group down.
func (p *Provisioner) ProvisionGroupFolders(ctx context.Context, group *Group) error {
	groupPath, err := p.resolveGroupPath(ctx, group)
	if err != nil {
		return fmt.Errorf("resolve group path: %w", err)
	}

	// Create group directory
	if err := p.ensureDir(ctx, groupPath, 0, group.ID); err != nil {
		return fmt.Errorf("create group dir: %w", err)
	}

	// Create shared/ subdirectory with group visibility
	sharedPath := groupPath + "/shared"
	if err := p.ensureDirWithVisibility(ctx, sharedPath, 0, group.ID, "group"); err != nil {
		return fmt.Errorf("create shared dir: %w", err)
	}

	return nil
}

// ProvisionUserHome creates a user's home directory within a top-level group.
// Creates /{group_name}/home/{username}/ with private visibility.
func (p *Provisioner) ProvisionUserHome(ctx context.Context, userID, groupID int) error {
	// Get the top-level group for this group
	topGroup, err := p.groups.GetTopLevelGroup(ctx, groupID)
	if err != nil {
		return fmt.Errorf("get top-level group: %w", err)
	}

	// Get the username
	username, err := p.groups.GetUsernameByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("get username: %w", err)
	}

	// Create /{group_name}/home/ if it doesn't exist
	groupPath := "/" + topGroup.Name
	homePath := groupPath + "/home"
	if err := p.ensureDir(ctx, homePath, 0, topGroup.ID); err != nil {
		return fmt.Errorf("create home dir: %w", err)
	}

	// Create /{group_name}/home/{username}/ with private visibility
	userHomePath := homePath + "/" + username
	if err := p.ensureDirWithVisibility(ctx, userHomePath, userID, topGroup.ID, "private"); err != nil {
		return fmt.Errorf("create user home dir: %w", err)
	}

	// Grant write permission on home directory
	if err := p.perms.SetPermission(ctx, userID, userHomePath, "write"); err != nil {
		return fmt.Errorf("set home permission: %w", err)
	}

	return nil
}

// DeprovisionUserHome removes permissions on a user's home directory.
// Does NOT delete files (data preservation).
func (p *Provisioner) DeprovisionUserHome(ctx context.Context, userID, groupID int) error {
	topGroup, err := p.groups.GetTopLevelGroup(ctx, groupID)
	if err != nil {
		logging.Debug("deprovision: group not found (may be deleted)", zap.Int("group_id", groupID), zap.Error(err))
		return nil
	}

	username, err := p.groups.GetUsernameByID(ctx, userID)
	if err != nil {
		logging.Debug("deprovision: user not found (may be deleted)", zap.Int("user_id", userID), zap.Error(err))
		return nil
	}

	userHomePath := "/" + topGroup.Name + "/home/" + username
	_ = p.perms.RemovePermission(ctx, userID, userHomePath)

	return nil
}

// resolveGroupPath builds the full path for a group by walking up the hierarchy.
func (p *Provisioner) resolveGroupPath(ctx context.Context, group *Group) (string, error) {
	if group.ParentID == nil {
		return "/" + group.Name, nil
	}

	// Walk up to build path segments
	var segments []string
	segments = append(segments, group.Name)

	currentID := group.ParentID
	for currentID != nil {
		parent, err := p.groups.GetGroup(ctx, *currentID)
		if err != nil {
			return "", err
		}
		segments = append(segments, parent.Name)
		currentID = parent.ParentID
	}

	// Reverse to get top-down order
	path := ""
	for i := len(segments) - 1; i >= 0; i-- {
		path += "/" + segments[i]
	}
	return path, nil
}

// ensureDir creates a directory entry if it doesn't exist.
func (p *Provisioner) ensureDir(ctx context.Context, path string, ownerID, groupID int) error {
	exists, err := p.meta.PathExists(ctx, path)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	// Ensure parent exists first
	parentPath := parentOf(path)
	if parentPath != "/" {
		if err := p.ensureDir(ctx, parentPath, 0, groupID); err != nil {
			return err
		}
	}

	row := &postgres.FileRow{
		ID:         pathID(path),
		Name:       baseName(path),
		Path:       path,
		ParentPath: parentPath,
		IsDir:      true,
		ModTime:    time.Now(),
	}
	if ownerID > 0 {
		row.OwnerID = &ownerID
	}
	if groupID > 0 {
		row.GroupID = &groupID
	}

	return p.meta.UpsertFile(ctx, row)
}

// ensureDirWithVisibility creates a directory entry with specific visibility.
func (p *Provisioner) ensureDirWithVisibility(ctx context.Context, path string, ownerID, groupID int, visibility string) error {
	exists, err := p.meta.PathExists(ctx, path)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	parentPath := parentOf(path)
	if parentPath != "/" {
		if err := p.ensureDir(ctx, parentPath, 0, groupID); err != nil {
			return err
		}
	}

	row := &postgres.FileRow{
		ID:         pathID(path),
		Name:       baseName(path),
		Path:       path,
		ParentPath: parentPath,
		IsDir:      true,
		ModTime:    time.Now(),
		Visibility: visibility,
	}
	if ownerID > 0 {
		row.OwnerID = &ownerID
	}
	if groupID > 0 {
		row.GroupID = &groupID
	}

	return p.meta.UpsertFile(ctx, row)
}

func pathID(path string) string {
	h := sha256.Sum256([]byte(path))
	return fmt.Sprintf("%x", h[:8])
}

func parentOf(path string) string {
	for i := len(path) - 1; i > 0; i-- {
		if path[i] == '/' {
			return path[:i]
		}
	}
	return "/"
}

func baseName(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[i+1:]
		}
	}
	return path
}
