package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"go.uber.org/zap"

	"github.com/fruitsalade/fruitsalade/fruitsalade/internal/auth"
	"github.com/fruitsalade/fruitsalade/fruitsalade/internal/logging"
	"github.com/fruitsalade/fruitsalade/shared/pkg/protocol"
)

// ─── Auth helpers ────────────────────────────────────────────────────────────

// requireGroupAdmin allows global admin OR user with "admin" role in the group.
func (s *Server) requireGroupAdmin(w http.ResponseWriter, r *http.Request, groupID int) *auth.Claims {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		s.sendError(w, http.StatusUnauthorized, "authentication required")
		return nil
	}
	if claims.IsAdmin {
		return claims
	}

	// Verify group exists before checking admin status
	_, err := s.groups.GetGroup(r.Context(), groupID)
	if err != nil {
		s.sendError(w, http.StatusNotFound, "group not found")
		return nil
	}

	isGA, err := s.groups.IsGroupAdmin(r.Context(), claims.UserID, groupID)
	if err != nil || !isGA {
		s.sendError(w, http.StatusForbidden, "admin or group admin access required")
		return nil
	}
	return claims
}

// ─── Admin: Groups ──────────────────────────────────────────────────────────

func (s *Server) handleListGroups(w http.ResponseWriter, r *http.Request) {
	if s.requireAdmin(w, r) == nil {
		return
	}

	groups, err := s.groups.ListGroups(r.Context())
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to list groups: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(groups)
}

func (s *Server) handleGroupTree(w http.ResponseWriter, r *http.Request) {
	if s.requireAdmin(w, r) == nil {
		return
	}

	tree, err := s.groups.GetGroupTree(r.Context())
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to get group tree: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tree)
}

func (s *Server) handleCreateGroup(w http.ResponseWriter, r *http.Request) {
	claims := s.requireAdmin(w, r)
	if claims == nil {
		return
	}

	var req protocol.GroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		s.sendError(w, http.StatusBadRequest, "group name required")
		return
	}

	group, err := s.groups.CreateGroup(r.Context(), req.Name, req.Description, req.ParentID, claims.UserID)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to create group: "+err.Error())
		return
	}

	// Auto-provision group folders
	if s.provisioner != nil {
		if err := s.provisioner.ProvisionGroupFolders(r.Context(), group); err != nil {
			logging.Warn("failed to provision group folders",
				zap.Int("group_id", group.ID), zap.Error(err))
		} else {
			s.RefreshTree(r.Context())
		}
	}

	logging.Info("group created", zap.String("name", req.Name), zap.Int("id", group.ID))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(group)
}

func (s *Server) handleGetGroup(w http.ResponseWriter, r *http.Request) {
	if s.requireAdmin(w, r) == nil {
		return
	}

	groupID, err := strconv.Atoi(r.PathValue("groupID"))
	if err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid group ID")
		return
	}

	group, err := s.groups.GetGroup(r.Context(), groupID)
	if err != nil {
		s.sendError(w, http.StatusNotFound, "group not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(group)
}

func (s *Server) handleDeleteGroup(w http.ResponseWriter, r *http.Request) {
	if s.requireAdmin(w, r) == nil {
		return
	}

	groupID, err := strconv.Atoi(r.PathValue("groupID"))
	if err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid group ID")
		return
	}

	if err := s.groups.DeleteGroup(r.Context(), groupID); err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to delete group: "+err.Error())
		return
	}

	logging.Info("group deleted", zap.Int("group_id", groupID))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"group_id": groupID,
		"deleted":  true,
	})
}

func (s *Server) handleMoveGroup(w http.ResponseWriter, r *http.Request) {
	if s.requireAdmin(w, r) == nil {
		return
	}

	groupID, err := strconv.Atoi(r.PathValue("groupID"))
	if err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid group ID")
		return
	}

	var req protocol.MoveGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := s.groups.MoveGroup(r.Context(), groupID, req.ParentID); err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to move group: "+err.Error())
		return
	}

	logging.Info("group moved", zap.Int("group_id", groupID))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"group_id":      groupID,
		"new_parent_id": req.ParentID,
		"moved":         true,
	})
}

// ─── Admin: Group Members ───────────────────────────────────────────────────

func (s *Server) handleListGroupMembers(w http.ResponseWriter, r *http.Request) {
	groupID, err := strconv.Atoi(r.PathValue("groupID"))
	if err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid group ID")
		return
	}

	if s.requireGroupAdmin(w, r, groupID) == nil {
		return
	}

	members, err := s.groups.ListMembers(r.Context(), groupID)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to list members: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(members)
}

func (s *Server) handleAddGroupMember(w http.ResponseWriter, r *http.Request) {
	groupID, err := strconv.Atoi(r.PathValue("groupID"))
	if err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid group ID")
		return
	}

	if s.requireGroupAdmin(w, r, groupID) == nil {
		return
	}

	var req protocol.GroupMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.UserID == 0 {
		s.sendError(w, http.StatusBadRequest, "user_id required")
		return
	}

	role := req.Role
	if role == "" {
		role = "viewer"
	}
	if role != "admin" && role != "editor" && role != "viewer" {
		s.sendError(w, http.StatusBadRequest, "role must be 'admin', 'editor', or 'viewer'")
		return
	}

	if err := s.groups.AddMember(r.Context(), groupID, req.UserID, role); err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to add member: "+err.Error())
		return
	}

	// Auto-provision home directory for top-level group membership
	if s.provisioner != nil {
		if err := s.provisioner.ProvisionUserHome(r.Context(), req.UserID, groupID); err != nil {
			logging.Warn("failed to provision user home",
				zap.Int("user_id", req.UserID), zap.Int("group_id", groupID), zap.Error(err))
		} else {
			s.RefreshTree(r.Context())
		}
	}

	logging.Info("member added to group",
		zap.Int("group_id", groupID), zap.Int("user_id", req.UserID), zap.String("role", role))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"group_id": groupID,
		"user_id":  req.UserID,
		"role":     role,
		"added":    true,
	})
}

func (s *Server) handleUpdateMemberRole(w http.ResponseWriter, r *http.Request) {
	groupID, err := strconv.Atoi(r.PathValue("groupID"))
	if err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid group ID")
		return
	}

	if s.requireGroupAdmin(w, r, groupID) == nil {
		return
	}

	userID, err := strconv.Atoi(r.PathValue("userID"))
	if err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	var req protocol.UpdateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Role != "admin" && req.Role != "editor" && req.Role != "viewer" {
		s.sendError(w, http.StatusBadRequest, "role must be 'admin', 'editor', or 'viewer'")
		return
	}

	if err := s.groups.UpdateMemberRole(r.Context(), groupID, userID, req.Role); err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to update role: "+err.Error())
		return
	}

	logging.Info("member role updated",
		zap.Int("group_id", groupID), zap.Int("user_id", userID), zap.String("role", req.Role))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"group_id": groupID,
		"user_id":  userID,
		"role":     req.Role,
		"updated":  true,
	})
}

func (s *Server) handleRemoveGroupMember(w http.ResponseWriter, r *http.Request) {
	groupID, err := strconv.Atoi(r.PathValue("groupID"))
	if err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid group ID")
		return
	}

	if s.requireGroupAdmin(w, r, groupID) == nil {
		return
	}

	userID, err := strconv.Atoi(r.PathValue("userID"))
	if err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	if err := s.groups.RemoveMember(r.Context(), groupID, userID); err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to remove member: "+err.Error())
		return
	}

	// Deprovision home directory
	if s.provisioner != nil {
		_ = s.provisioner.DeprovisionUserHome(r.Context(), userID, groupID)
	}

	logging.Info("member removed from group", zap.Int("group_id", groupID), zap.Int("user_id", userID))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"group_id": groupID,
		"user_id":  userID,
		"removed":  true,
	})
}

// ─── Admin: Group Permissions ───────────────────────────────────────────────

func (s *Server) handleListGroupPermissions(w http.ResponseWriter, r *http.Request) {
	groupID, err := strconv.Atoi(r.PathValue("groupID"))
	if err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid group ID")
		return
	}

	if s.requireGroupAdmin(w, r, groupID) == nil {
		return
	}

	perms, err := s.groups.ListPermissionsByGroup(r.Context(), groupID)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to list permissions: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(perms)
}

func (s *Server) handleSetGroupPermission(w http.ResponseWriter, r *http.Request) {
	groupID, err := strconv.Atoi(r.PathValue("groupID"))
	if err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid group ID")
		return
	}

	if s.requireGroupAdmin(w, r, groupID) == nil {
		return
	}

	path := "/" + r.PathValue("path")

	var req protocol.GroupPermissionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Permission != "read" && req.Permission != "write" && req.Permission != "owner" {
		s.sendError(w, http.StatusBadRequest, "permission must be 'read', 'write', or 'owner'")
		return
	}

	if err := s.groups.SetPermission(r.Context(), groupID, path, req.Permission); err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to set permission: "+err.Error())
		return
	}

	logging.Info("group permission set",
		zap.Int("group_id", groupID),
		zap.String("path", path),
		zap.String("permission", req.Permission))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"group_id":   groupID,
		"path":       path,
		"permission": req.Permission,
	})
}

func (s *Server) handleDeleteGroupPermission(w http.ResponseWriter, r *http.Request) {
	groupID, err := strconv.Atoi(r.PathValue("groupID"))
	if err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid group ID")
		return
	}

	if s.requireGroupAdmin(w, r, groupID) == nil {
		return
	}

	path := "/" + r.PathValue("path")

	if err := s.groups.RemovePermission(r.Context(), groupID, path); err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to remove permission: "+err.Error())
		return
	}

	logging.Info("group permission removed", zap.Int("group_id", groupID), zap.String("path", path))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"group_id": groupID,
		"path":     path,
		"removed":  true,
	})
}

// ─── Visibility ─────────────────────────────────────────────────────────────

func (s *Server) handleGetVisibility(w http.ResponseWriter, r *http.Request) {
	path := "/" + r.PathValue("path")

	claims := auth.GetClaims(r.Context())
	if claims == nil {
		s.sendError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	// Owner or admin can view visibility
	if !claims.IsAdmin {
		ownerID, hasOwner := s.permissions.GetOwnerID(r.Context(), path)
		if !hasOwner || ownerID != claims.UserID {
			s.sendError(w, http.StatusForbidden, "only the owner or admin can view visibility")
			return
		}
	}

	vis, err := s.permissions.GetVisibility(r.Context(), path)
	if err != nil {
		s.sendError(w, http.StatusNotFound, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"path":       path,
		"visibility": vis,
	})
}

func (s *Server) handleSetVisibility(w http.ResponseWriter, r *http.Request) {
	path := "/" + r.PathValue("path")

	claims := auth.GetClaims(r.Context())
	if claims == nil {
		s.sendError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	// Owner or admin can set visibility
	if !claims.IsAdmin {
		ownerID, hasOwner := s.permissions.GetOwnerID(r.Context(), path)
		if !hasOwner || ownerID != claims.UserID {
			s.sendError(w, http.StatusForbidden, "only the owner or admin can set visibility")
			return
		}
	}

	var req protocol.SetVisibilityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Visibility != "public" && req.Visibility != "group" && req.Visibility != "private" {
		s.sendError(w, http.StatusBadRequest, "visibility must be 'public', 'group', or 'private'")
		return
	}

	if err := s.permissions.SetVisibility(r.Context(), path, req.Visibility); err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to set visibility: "+err.Error())
		return
	}

	// Refresh tree to pick up visibility change
	s.RefreshTree(r.Context())

	logging.Info("visibility set", zap.String("path", path), zap.String("visibility", req.Visibility))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"path":       path,
		"visibility": req.Visibility,
	})
}
