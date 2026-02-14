package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"go.uber.org/zap"

	"github.com/fruitsalade/fruitsalade/phase2/internal/auth"
	"github.com/fruitsalade/fruitsalade/phase2/internal/logging"
	"github.com/fruitsalade/fruitsalade/phase2/internal/sharing"
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

// ─── Admin: User Groups ─────────────────────────────────────────────────────

func (s *Server) handleUserGroups(w http.ResponseWriter, r *http.Request) {
	if s.requireAdmin(w, r) == nil {
		return
	}

	userID, err := strconv.Atoi(r.PathValue("userID"))
	if err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	memberships, err := s.groups.GetUserGroupsWithRoles(r.Context(), userID)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to get user groups: "+err.Error())
		return
	}

	if memberships == nil {
		memberships = []sharing.UserGroupMembership{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(memberships)
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
	fileCount, _ := s.metadata.FileCount(ctx)

	// Total storage: sum of all file sizes
	var totalStorage int64
	db := s.auth.DB()
	db.QueryRowContext(ctx, `SELECT COALESCE(SUM(size), 0) FROM files WHERE is_dir = FALSE`).Scan(&totalStorage)

	// Share link counts
	var activeShareLinks, totalShareLinks int64
	db.QueryRowContext(ctx, `SELECT COUNT(*) FROM share_links WHERE is_active = TRUE`).Scan(&activeShareLinks)
	db.QueryRowContext(ctx, `SELECT COUNT(*) FROM share_links`).Scan(&totalShareLinks)

	// Group count
	groupCount, _ := s.groups.GroupCount(ctx)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"users":              userCount,
		"active_sessions":    sessionCount,
		"files":              fileCount,
		"total_storage":      totalStorage,
		"active_share_links": activeShareLinks,
		"total_share_links":  totalShareLinks,
		"groups":             groupCount,
	})
}

// ─── Token Management (user-facing, not admin-only) ─────────────────────────

func (s *Server) handleRevokeCurrentToken(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		s.sendError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	tokenStr := extractBearerToken(r)
	if tokenStr == "" {
		s.sendError(w, http.StatusBadRequest, "no token to revoke")
		return
	}

	if err := s.auth.RevokeToken(r.Context(), tokenStr); err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to revoke token: "+err.Error())
		return
	}

	logging.Info("token revoked", zap.Int("user_id", claims.UserID))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"revoked": true})
}

func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		s.sendError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	sessions, err := s.auth.ListSessions(r.Context(), claims.UserID)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to list sessions: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sessions)
}

func (s *Server) handleRevokeSession(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		s.sendError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	tokenID, err := strconv.Atoi(r.PathValue("tokenID"))
	if err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid token ID")
		return
	}

	if err := s.auth.RevokeSession(r.Context(), claims.UserID, tokenID); err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to revoke session: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"token_id": tokenID, "revoked": true})
}

func (s *Server) handleRefreshToken(w http.ResponseWriter, r *http.Request) {
	tokenStr := extractBearerToken(r)
	if tokenStr == "" {
		s.sendError(w, http.StatusUnauthorized, "bearer token required")
		return
	}

	newToken, expiresAt, err := s.auth.RefreshToken(r.Context(), tokenStr)
	if err != nil {
		s.sendError(w, http.StatusUnauthorized, "refresh failed: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token":      newToken,
		"expires_at": expiresAt,
	})
}

func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}

// ─── Admin: Config ──────────────────────────────────────────────────────────

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	if s.requireAdmin(w, r) == nil {
		return
	}

	if s.config == nil {
		s.sendError(w, http.StatusInternalServerError, "configuration not available")
		return
	}

	cfg := s.config
	resp := map[string]interface{}{
		"server": map[string]interface{}{
			"listen_addr":  cfg.ListenAddr,
			"metrics_addr": cfg.MetricsAddr,
		},
		"storage": map[string]interface{}{
			"s3_endpoint": cfg.S3Endpoint,
			"s3_bucket":   cfg.S3Bucket,
			"s3_region":   cfg.S3Region,
			"s3_use_ssl":  cfg.S3UseSSL,
		},
		"database": map[string]interface{}{
			"connected": true,
		},
		"auth": map[string]interface{}{
			"jwt_configured": cfg.JWTSecret != "",
			"oidc_issuer":    cfg.OIDCIssuerURL,
		},
		"tls": map[string]interface{}{
			"enabled":   cfg.TLSCertFile != "" && cfg.TLSKeyFile != "",
			"cert_file": cfg.TLSCertFile,
		},
		"runtime": map[string]interface{}{
			"log_level":             cfg.LogLevel,
			"max_upload_size":       cfg.MaxUploadSize,
			"default_max_storage":   cfg.DefaultMaxStorage,
			"default_max_bandwidth": cfg.DefaultMaxBandwidth,
			"default_requests_per_min": cfg.DefaultRequestsPerMin,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	if s.requireAdmin(w, r) == nil {
		return
	}

	if s.config == nil {
		s.sendError(w, http.StatusInternalServerError, "configuration not available")
		return
	}

	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	cfg := s.config

	if v, ok := req["log_level"].(string); ok {
		cfg.LogLevel = v
		logging.SetLevel(v)
		logging.Info("log level changed", zap.String("level", v))
	}

	if v, ok := req["max_upload_size"].(float64); ok {
		cfg.MaxUploadSize = int64(v)
		s.maxUploadSize = int64(v)
		logging.Info("max upload size changed", zap.Int64("size", int64(v)))
	}

	if v, ok := req["default_max_storage"].(float64); ok {
		cfg.DefaultMaxStorage = int64(v)
	}

	if v, ok := req["default_max_bandwidth"].(float64); ok {
		cfg.DefaultMaxBandwidth = int64(v)
	}

	if v, ok := req["default_requests_per_min"].(float64); ok {
		cfg.DefaultRequestsPerMin = int(v)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"updated": true,
		"runtime": map[string]interface{}{
			"log_level":             cfg.LogLevel,
			"max_upload_size":       cfg.MaxUploadSize,
			"default_max_storage":   cfg.DefaultMaxStorage,
			"default_max_bandwidth": cfg.DefaultMaxBandwidth,
			"default_requests_per_min": cfg.DefaultRequestsPerMin,
		},
	})
}
