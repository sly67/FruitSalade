package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"go.uber.org/zap"

	"github.com/fruitsalade/fruitsalade/phase2/internal/auth"
	"github.com/fruitsalade/fruitsalade/phase2/internal/logging"
)

// requireAdmin checks that the request is from an admin user.
// Returns the claims if valid, nil otherwise (and writes error response).
func (s *Server) requireAdmin(w http.ResponseWriter, r *http.Request) *auth.Claims {
	claims := auth.GetClaims(r.Context())
	if claims == nil || !claims.IsAdmin {
		s.sendError(w, http.StatusForbidden, "admin access required")
		return nil
	}
	return claims
}

// ─── Admin: Users ───────────────────────────────────────────────────────────

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	if s.requireAdmin(w, r) == nil {
		return
	}

	users, err := s.auth.ListUsers(r.Context())
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to list users: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	if s.requireAdmin(w, r) == nil {
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		IsAdmin  bool   `json:"is_admin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Username == "" || req.Password == "" {
		s.sendError(w, http.StatusBadRequest, "username and password required")
		return
	}

	if err := s.auth.CreateUser(r.Context(), req.Username, req.Password, req.IsAdmin); err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to create user: "+err.Error())
		return
	}

	logging.Info("admin created user", zap.String("username", req.Username))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"username": req.Username,
		"is_admin": req.IsAdmin,
	})
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	claims := s.requireAdmin(w, r)
	if claims == nil {
		return
	}

	userID, err := strconv.Atoi(r.PathValue("userID"))
	if err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	if userID == claims.UserID {
		s.sendError(w, http.StatusBadRequest, "cannot delete yourself")
		return
	}

	if err := s.auth.DeleteUser(r.Context(), userID); err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to delete user: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"user_id": userID,
		"deleted": true,
	})
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	if s.requireAdmin(w, r) == nil {
		return
	}

	userID, err := strconv.Atoi(r.PathValue("userID"))
	if err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Password == "" {
		s.sendError(w, http.StatusBadRequest, "password required")
		return
	}

	if err := s.auth.ChangePassword(r.Context(), userID, req.Password); err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to change password: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"user_id": userID,
		"changed": true,
	})
}

// ─── Admin: Share Links ─────────────────────────────────────────────────────

func (s *Server) handleListShareLinks(w http.ResponseWriter, r *http.Request) {
	if s.requireAdmin(w, r) == nil {
		return
	}

	activeOnly := r.URL.Query().Get("active") == "true"

	links, err := s.shareLinks.ListAll(r.Context(), activeOnly)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to list share links: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(links)
}

// ─── Admin: Dashboard Stats ─────────────────────────────────────────────────

func (s *Server) handleDashboardStats(w http.ResponseWriter, r *http.Request) {
	if s.requireAdmin(w, r) == nil {
		return
	}

	ctx := r.Context()

	userCount, _ := s.auth.UserCount(ctx)
	sessionCount, _ := s.auth.ActiveSessionCount(ctx)
	fileCount, _ := s.storage.Metadata().FileCount(ctx)

	// Total storage: sum of all file sizes
	var totalStorage int64
	db := s.auth.DB()
	db.QueryRowContext(ctx, `SELECT COALESCE(SUM(size), 0) FROM files WHERE is_dir = FALSE`).Scan(&totalStorage)

	// Share link counts
	var activeShareLinks, totalShareLinks int64
	db.QueryRowContext(ctx, `SELECT COUNT(*) FROM share_links WHERE is_active = TRUE`).Scan(&activeShareLinks)
	db.QueryRowContext(ctx, `SELECT COUNT(*) FROM share_links`).Scan(&totalShareLinks)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"users":              userCount,
		"active_sessions":    sessionCount,
		"files":              fileCount,
		"total_storage":      totalStorage,
		"active_share_links": activeShareLinks,
		"total_share_links":  totalShareLinks,
	})
}
