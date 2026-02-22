// Package api provides the HTTP server and handlers.
package api

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/fruitsalade/fruitsalade/fruitsalade/internal/auth"
	"github.com/fruitsalade/fruitsalade/fruitsalade/internal/config"
	"github.com/fruitsalade/fruitsalade/fruitsalade/internal/events"
	"github.com/fruitsalade/fruitsalade/fruitsalade/internal/gallery"
	"github.com/fruitsalade/fruitsalade/fruitsalade/internal/logging"
	"github.com/fruitsalade/fruitsalade/fruitsalade/internal/metadata/postgres"
	"github.com/fruitsalade/fruitsalade/fruitsalade/internal/metrics"
	"github.com/fruitsalade/fruitsalade/fruitsalade/internal/quota"
	"github.com/fruitsalade/fruitsalade/fruitsalade/internal/sharing"
	"github.com/fruitsalade/fruitsalade/fruitsalade/internal/storage"
	davpkg "github.com/fruitsalade/fruitsalade/fruitsalade/internal/webdav"
	"github.com/fruitsalade/fruitsalade/fruitsalade/webapp"
	"github.com/fruitsalade/fruitsalade/shared/pkg/models"
	"github.com/fruitsalade/fruitsalade/shared/pkg/protocol"
)

// Package-level compiled regex for Range header parsing.
var rangeRegex = regexp.MustCompile(`bytes=(\d*)-(\d*)`)

// Pool gzip writers to reduce allocations on tree/subtree endpoints.
var gzipPool = sync.Pool{
	New: func() any { return gzip.NewWriter(nil) },
}

// Server is the HTTP server.
type Server struct {
	metadata      *postgres.Store
	storageRouter *storage.Router
	auth          *auth.Auth
	tree          *models.FileNode
	maxUploadSize int64
	config        *config.Config

	// SSE
	broadcaster *events.Broadcaster

	// Sharing
	permissions  *sharing.PermissionStore
	shareLinks   *sharing.ShareLinkStore
	groups       *sharing.GroupStore
	provisioner  *sharing.Provisioner

	// Quotas
	quotaStore  *quota.QuotaStore
	rateLimiter *quota.RateLimiter

	// Storage admin
	locationStore *storage.LocationStore

	// Gallery
	galleryStore *gallery.GalleryStore
	processor    *gallery.Processor
	pluginCaller *gallery.PluginCaller
}

// GalleryDeps bundles the gallery subsystem dependencies.
type GalleryDeps struct {
	Store        *gallery.GalleryStore
	Processor    *gallery.Processor
	PluginCaller *gallery.PluginCaller
}

// NewServer creates a new server.
func NewServer(
	metadata *postgres.Store,
	storageRouter *storage.Router,
	authHandler *auth.Auth,
	maxUploadSize int64,
	broadcaster *events.Broadcaster,
	permissions *sharing.PermissionStore,
	shareLinks *sharing.ShareLinkStore,
	quotaStore *quota.QuotaStore,
	rateLimiter *quota.RateLimiter,
	groups *sharing.GroupStore,
	cfg *config.Config,
	provisioner *sharing.Provisioner,
	locationStore *storage.LocationStore,
	galleryDeps *GalleryDeps,
) *Server {
	s := &Server{
		metadata:      metadata,
		storageRouter: storageRouter,
		auth:          authHandler,
		maxUploadSize: maxUploadSize,
		broadcaster:   broadcaster,
		permissions:   permissions,
		shareLinks:    shareLinks,
		groups:        groups,
		provisioner:   provisioner,
		quotaStore:    quotaStore,
		rateLimiter:   rateLimiter,
		config:        cfg,
		locationStore: locationStore,
	}
	if galleryDeps != nil {
		s.galleryStore = galleryDeps.Store
		s.processor = galleryDeps.Processor
		s.pluginCaller = galleryDeps.PluginCaller
	}
	return s
}

// Init initializes the server by building the metadata tree.
func (s *Server) Init(ctx context.Context) error {
	logging.Info("building metadata tree from database...")
	tree, err := s.metadata.BuildTree(ctx)
	if err != nil {
		return fmt.Errorf("build tree: %w", err)
	}
	s.tree = tree
	count := countNodes(tree)
	metrics.SetMetadataTreeSize(int64(count))
	logging.Info("metadata tree built", zap.Int("items", count))
	return nil
}

// RefreshTree rebuilds the metadata tree.
func (s *Server) RefreshTree(ctx context.Context) error {
	tree, err := s.metadata.BuildTree(ctx)
	if err != nil {
		return err
	}
	s.tree = tree
	metrics.SetMetadataTreeSize(int64(countNodes(tree)))
	return nil
}

func countNodes(node *models.FileNode) int {
	if node == nil {
		return 0
	}
	count := 1
	for _, child := range node.Children {
		count += countNodes(child)
	}
	return count
}

// Handler returns the HTTP handler with auth and metrics middleware.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Public endpoints (no auth required)
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("POST /api/v1/auth/token", s.auth.HandleLogin)
	mux.HandleFunc("POST /api/v1/auth/device-code", s.handleDeviceCodeInit)
	mux.HandleFunc("POST /api/v1/auth/device-token", s.handleDeviceCodePoll)
	mux.HandleFunc("POST /api/v1/auth/totp/verify", s.handleTOTPVerify)

	// Public share link endpoints (no auth)
	mux.HandleFunc("GET /api/v1/share/{token}/info", s.handleShareInfo)
	mux.HandleFunc("GET /api/v1/share/{token}", s.handleShareDownload)

	// Web app (no auth — the app handles login via API)
	// WEBAPP_DIR overrides embedded assets for live-reload during development
	var appHandler http.Handler
	if dir := os.Getenv("WEBAPP_DIR"); dir != "" {
		log.Printf("[webapp] serving from disk: %s", dir)
		appHandler = http.StripPrefix("/app/", http.FileServer(http.Dir(dir)))
	} else {
		appFS, _ := fs.Sub(webapp.Assets, ".")
		appHandler = http.StripPrefix("/app/", http.FileServer(http.FS(appFS)))
	}
	mux.Handle("/app/", appHandler)
	mux.HandleFunc("GET /app", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/app/", http.StatusMovedPermanently)
	})

	// Redirect root to /app/
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/app/", http.StatusMovedPermanently)
	})

	// Redirect old /admin/ to /app/
	mux.HandleFunc("/admin/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/app/", http.StatusMovedPermanently)
	})
	mux.HandleFunc("GET /admin", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/app/", http.StatusMovedPermanently)
	})

	// WebDAV endpoint (has its own auth middleware)
	davHandler := davpkg.NewHandler(s.metadata, s.storageRouter, s.auth)
	mux.Handle("/webdav/", davHandler)
	mux.Handle("/webdav", davHandler)

	// Protected endpoints
	protected := http.NewServeMux()

	// Read endpoints
	protected.HandleFunc("GET /api/v1/tree", s.handleTree)
	protected.HandleFunc("GET /api/v1/tree/{path...}", s.handleSubtree)
	protected.HandleFunc("GET /api/v1/content/{path...}", s.handleContent)

	// Write endpoints
	protected.HandleFunc("POST /api/v1/content/{path...}", s.handleUpload)
	protected.HandleFunc("PUT /api/v1/tree/{path...}", s.handleCreateOrUpdate)
	protected.HandleFunc("DELETE /api/v1/tree/{path...}", s.handleDelete)

	// Version endpoints
	protected.HandleFunc("GET /api/v1/versions", s.handleVersionedFiles)
	protected.HandleFunc("GET /api/v1/versions/{path...}", s.handleVersions)
	protected.HandleFunc("POST /api/v1/versions/{path...}", s.handleRollback)

	// SSE endpoint
	protected.HandleFunc("GET /api/v1/events", s.handleEvents)

	// Permission endpoints
	protected.HandleFunc("PUT /api/v1/permissions/{path...}", s.handleSetPermission)
	protected.HandleFunc("GET /api/v1/permissions/{path...}", s.handleListPermissions)
	protected.HandleFunc("DELETE /api/v1/permissions/{path...}", s.handleDeletePermission)

	// Share link management endpoints
	protected.HandleFunc("GET /api/v1/shares", s.handleListUserShares)
	protected.HandleFunc("POST /api/v1/share/{path...}", s.handleCreateShareLink)
	protected.HandleFunc("DELETE /api/v1/share/{id}", s.handleRevokeShareLink)

	// Admin quota endpoints
	protected.HandleFunc("GET /api/v1/admin/quotas/{userID}", s.handleGetQuota)
	protected.HandleFunc("PUT /api/v1/admin/quotas/{userID}", s.handleSetQuota)

	// Admin UI endpoints
	protected.HandleFunc("GET /api/v1/admin/users", s.handleListUsers)
	protected.HandleFunc("POST /api/v1/admin/users", s.handleCreateUser)
	protected.HandleFunc("DELETE /api/v1/admin/users/{userID}", s.handleDeleteUser)
	protected.HandleFunc("PUT /api/v1/admin/users/{userID}/password", s.handleChangePassword)
	protected.HandleFunc("GET /api/v1/admin/users/{userID}/groups", s.handleUserGroups)
	protected.HandleFunc("GET /api/v1/admin/sharelinks", s.handleListShareLinks)
	protected.HandleFunc("GET /api/v1/admin/stats", s.handleDashboardStats)
	protected.HandleFunc("GET /api/v1/admin/storage-dashboard", s.handleStorageDashboard)
	protected.HandleFunc("GET /api/v1/admin/config", s.handleGetConfig)
	protected.HandleFunc("PUT /api/v1/admin/config", s.handleUpdateConfig)

	// Admin group endpoints
	protected.HandleFunc("GET /api/v1/admin/groups", s.handleListGroups)
	protected.HandleFunc("POST /api/v1/admin/groups", s.handleCreateGroup)
	protected.HandleFunc("GET /api/v1/admin/groups/tree", s.handleGroupTree)
	protected.HandleFunc("GET /api/v1/admin/groups/{groupID}", s.handleGetGroup)
	protected.HandleFunc("DELETE /api/v1/admin/groups/{groupID}", s.handleDeleteGroup)
	protected.HandleFunc("PUT /api/v1/admin/groups/{groupID}/parent", s.handleMoveGroup)
	protected.HandleFunc("GET /api/v1/admin/groups/{groupID}/members", s.handleListGroupMembers)
	protected.HandleFunc("POST /api/v1/admin/groups/{groupID}/members", s.handleAddGroupMember)
	protected.HandleFunc("PUT /api/v1/admin/groups/{groupID}/members/{userID}/role", s.handleUpdateMemberRole)
	protected.HandleFunc("DELETE /api/v1/admin/groups/{groupID}/members/{userID}", s.handleRemoveGroupMember)
	protected.HandleFunc("GET /api/v1/admin/groups/{groupID}/permissions", s.handleListGroupPermissions)
	protected.HandleFunc("PUT /api/v1/admin/groups/{groupID}/permissions/{path...}", s.handleSetGroupPermission)
	protected.HandleFunc("DELETE /api/v1/admin/groups/{groupID}/permissions/{path...}", s.handleDeleteGroupPermission)

	// Admin storage endpoints
	protected.HandleFunc("GET /api/v1/admin/storage", s.handleListStorageLocations)
	protected.HandleFunc("GET /api/v1/admin/storage/{id}", s.handleGetStorageLocation)
	protected.HandleFunc("POST /api/v1/admin/storage", s.handleCreateStorageLocation)
	protected.HandleFunc("PUT /api/v1/admin/storage/{id}", s.handleUpdateStorageLocation)
	protected.HandleFunc("DELETE /api/v1/admin/storage/{id}", s.handleDeleteStorageLocation)
	protected.HandleFunc("POST /api/v1/admin/storage/{id}/test", s.handleTestStorageLocation)
	protected.HandleFunc("POST /api/v1/admin/storage/{id}/default", s.handleSetDefaultStorage)
	protected.HandleFunc("GET /api/v1/admin/storage/{id}/stats", s.handleStorageStats)

	// Gallery endpoints
	if s.galleryStore != nil {
		protected.HandleFunc("GET /api/v1/gallery/search", s.handleGallerySearch)
		protected.HandleFunc("GET /api/v1/gallery/thumb/{path...}", s.handleGalleryThumb)
		protected.HandleFunc("GET /api/v1/gallery/metadata/{path...}", s.handleGalleryMetadata)
		protected.HandleFunc("GET /api/v1/gallery/albums/date", s.handleAlbumsByDate)
		protected.HandleFunc("GET /api/v1/gallery/albums/location", s.handleAlbumsByLocation)
		protected.HandleFunc("GET /api/v1/gallery/albums/camera", s.handleAlbumsByCamera)
		protected.HandleFunc("POST /api/v1/gallery/tags/{path...}", s.handleAddTag)
		protected.HandleFunc("DELETE /api/v1/gallery/tags/{path...}", s.handleRemoveTag)
		protected.HandleFunc("GET /api/v1/gallery/tags", s.handleListTags)
		protected.HandleFunc("GET /api/v1/gallery/stats", s.handleGalleryStats)
		protected.HandleFunc("GET /api/v1/gallery/map/points", s.handleGalleryMapPoints)

		// Custom album endpoints
		protected.HandleFunc("GET /api/v1/gallery/albums", s.handleListUserAlbums)
		protected.HandleFunc("POST /api/v1/gallery/albums", s.handleCreateAlbum)
		protected.HandleFunc("PUT /api/v1/gallery/albums/{id}", s.handleUpdateAlbum)
		protected.HandleFunc("DELETE /api/v1/gallery/albums/{id}", s.handleDeleteAlbum)
		protected.HandleFunc("GET /api/v1/gallery/albums/{id}/images", s.handleGetAlbumImages)
		protected.HandleFunc("POST /api/v1/gallery/albums/{id}/images", s.handleAddImageToAlbum)
		protected.HandleFunc("DELETE /api/v1/gallery/albums/{id}/images", s.handleRemoveImageFromAlbum)
		protected.HandleFunc("PUT /api/v1/gallery/albums/{id}/cover", s.handleSetAlbumCover)
		protected.HandleFunc("GET /api/v1/gallery/image-albums/{path...}", s.handleGetAlbumsForImage)

		// Per-user tag management
		protected.HandleFunc("DELETE /api/v1/gallery/user-tags/{tag}", s.handleDeleteUserTag)
		protected.HandleFunc("PUT /api/v1/gallery/user-tags/{tag}", s.handleRenameUserTag)

		// Admin gallery plugin endpoints
		protected.HandleFunc("GET /api/v1/admin/gallery/plugins", s.handleListPlugins)
		protected.HandleFunc("POST /api/v1/admin/gallery/plugins", s.handleCreatePlugin)
		protected.HandleFunc("PUT /api/v1/admin/gallery/plugins/{id}", s.handleUpdatePlugin)
		protected.HandleFunc("DELETE /api/v1/admin/gallery/plugins/{id}", s.handleDeletePlugin)
		protected.HandleFunc("POST /api/v1/admin/gallery/plugins/{id}/test", s.handleTestPlugin)
		protected.HandleFunc("POST /api/v1/admin/gallery/reprocess", s.handleReprocessGallery)

		// Admin global tag management
		protected.HandleFunc("DELETE /api/v1/admin/gallery/tags/{tag}", s.handleDeleteTagGlobal)
		protected.HandleFunc("PUT /api/v1/admin/gallery/tags/{tag}", s.handleRenameTagGlobal)
	}

	// Trash endpoints
	protected.HandleFunc("GET /api/v1/trash", s.handleTrashList)
	protected.HandleFunc("POST /api/v1/trash/restore", s.handleTrashRestore)
	protected.HandleFunc("DELETE /api/v1/trash/{path...}", s.handleTrashPurge)
	protected.HandleFunc("DELETE /api/v1/trash", s.handleTrashEmpty)

	// Favorites endpoints
	protected.HandleFunc("GET /api/v1/favorites", s.handleListFavorites)
	protected.HandleFunc("GET /api/v1/favorites/paths", s.handleListFavoritePaths)
	protected.HandleFunc("PUT /api/v1/favorites/{path...}", s.handleAddFavorite)
	protected.HandleFunc("DELETE /api/v1/favorites/{path...}", s.handleRemoveFavorite)

	// Search endpoint
	protected.HandleFunc("GET /api/v1/search", s.handleSearch)

	// Bulk operation endpoints
	protected.HandleFunc("POST /api/v1/bulk/move", s.handleBulkMove)
	protected.HandleFunc("POST /api/v1/bulk/copy", s.handleBulkCopy)
	protected.HandleFunc("POST /api/v1/bulk/share", s.handleBulkShare)
	protected.HandleFunc("POST /api/v1/bulk/tag", s.handleBulkTag)

	// File properties endpoint
	protected.HandleFunc("GET /api/v1/properties/{path...}", s.handleFileProperties)

	// Visibility endpoints
	protected.HandleFunc("GET /api/v1/visibility/{path...}", s.handleGetVisibility)
	protected.HandleFunc("PUT /api/v1/visibility/{path...}", s.handleSetVisibility)

	// Token management endpoints (user-facing)
	protected.HandleFunc("DELETE /api/v1/auth/token", s.handleRevokeCurrentToken)
	protected.HandleFunc("POST /api/v1/auth/refresh", s.handleRefreshToken)
	protected.HandleFunc("GET /api/v1/auth/sessions", s.handleListSessions)
	protected.HandleFunc("DELETE /api/v1/auth/sessions/{tokenID}", s.handleRevokeSession)

	// TOTP 2FA endpoints (user-facing, protected)
	protected.HandleFunc("GET /api/v1/auth/totp/status", s.handleTOTPStatus)
	protected.HandleFunc("POST /api/v1/auth/totp/setup", s.handleTOTPSetup)
	protected.HandleFunc("POST /api/v1/auth/totp/enable", s.handleTOTPEnable)
	protected.HandleFunc("POST /api/v1/auth/totp/disable", s.handleTOTPDisable)
	protected.HandleFunc("POST /api/v1/auth/totp/backup", s.handleTOTPBackup)

	// User usage endpoint
	protected.HandleFunc("GET /api/v1/usage", s.handleGetUsage)

	// User dashboard endpoint
	protected.HandleFunc("GET /api/v1/user/dashboard", s.handleUserDashboard)

	// Wrap protected routes with auth then rate limiter
	// Use OIDC-aware middleware if OIDC is configured
	var authed http.Handler
	if s.auth.HasOIDC() {
		authed = s.auth.MiddlewareWithOIDC(protected)
	} else {
		authed = s.auth.Middleware(protected)
	}
	getUserInfo := func(ctx context.Context) (int, int, bool) {
		claims := auth.GetClaims(ctx)
		if claims == nil {
			return 0, 0, false
		}
		return claims.UserID, 0, true
	}
	rateLimited := quota.RateLimitMiddleware(s.rateLimiter, s.quotaStore, getUserInfo)(authed)
	mux.Handle("/api/v1/", rateLimited)

	// Apply logging and metrics middleware
	return metrics.Middleware(logging.Middleware(mux))
}

// ─── Health ─────────────────────────────────────────────────────────────────

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "version": "1.0"})
}

// ─── SSE Events ─────────────────────────────────────────────────────────────

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		s.sendError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ch := s.broadcaster.Subscribe()
	defer s.broadcaster.Unsubscribe(ch)

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-ch:
			if !ok {
				return
			}
			data, err := events.MarshalEvent(event)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, data)
			flusher.Flush()
		}
	}
}

// publishEvent publishes an event to the broadcaster if available.
func (s *Server) publishEvent(eventType, path string, version int, hash string, size int64) {
	if s.broadcaster == nil {
		return
	}
	s.broadcaster.Publish(events.Event{
		Type:    eventType,
		Path:    path,
		Version: version,
		Hash:    hash,
		Size:    size,
	})
}

// ─── Tree ───────────────────────────────────────────────────────────────────

func (s *Server) handleTree(w http.ResponseWriter, r *http.Request) {
	if s.tree == nil {
		s.sendError(w, http.StatusInternalServerError, "metadata not initialized")
		return
	}

	// Filter tree by user permissions
	claims := auth.GetClaims(r.Context())
	filtered := s.filterTree(r.Context(), s.tree, claims)

	resp := protocol.TreeResponse{Root: filtered}

	if acceptsGzip(r) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Encoding", "gzip")
		gw := gzipPool.Get().(*gzip.Writer)
		gw.Reset(w)
		json.NewEncoder(gw).Encode(resp)
		gw.Close()
		gzipPool.Put(gw)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleSubtree(w http.ResponseWriter, r *http.Request) {
	path := r.PathValue("path")
	if path == "" {
		s.handleTree(w, r)
		return
	}

	node := s.findNode(s.tree, "/"+path)
	if node == nil {
		s.sendError(w, http.StatusNotFound, "path not found: "+path)
		return
	}

	// Check read permission
	claims := auth.GetClaims(r.Context())
	if claims != nil && !s.permissions.CheckAccess(r.Context(), claims.UserID, "/"+path, "read", claims.IsAdmin) {
		s.sendError(w, http.StatusForbidden, "access denied")
		return
	}

	filtered := s.filterTree(r.Context(), node, claims)
	resp := protocol.TreeResponse{Root: filtered}

	if acceptsGzip(r) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Encoding", "gzip")
		gw := gzipPool.Get().(*gzip.Writer)
		gw.Reset(w)
		json.NewEncoder(gw).Encode(resp)
		gw.Close()
		gzipPool.Put(gw)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// filterTree returns a copy of the tree with only nodes the user can read.
// Admins see everything. Uses pre-loaded maps for performance.
func (s *Server) filterTree(ctx context.Context, node *models.FileNode, claims *auth.Claims) *models.FileNode {
	if node == nil || claims == nil {
		return node
	}
	if claims.IsAdmin {
		return node
	}

	// Pre-load maps once for the entire tree walk
	userGroups, _ := s.groups.GetUserGroupsMap(ctx, claims.UserID)
	if userGroups == nil {
		userGroups = make(map[int]string)
	}
	userPerms, _ := s.permissions.GetUserPermissionsMap(ctx, claims.UserID)
	if userPerms == nil {
		userPerms = make(map[string]string)
	}

	return s.filterNodeRecursive(ctx, node, claims, userGroups, userPerms)
}

// filterNodeRecursive filters a single node using pre-loaded permission/group maps.
func (s *Server) filterNodeRecursive(ctx context.Context, node *models.FileNode, claims *auth.Claims, userGroups map[int]string, userPerms map[string]string) *models.FileNode {
	if node == nil {
		return nil
	}

	// 1. Visibility gate
	if !s.permissions.CheckVisibility(node, claims.UserID, false, userGroups) {
		return nil
	}

	// 2. Permission gate for files
	if !node.IsDir && !s.checkAccessFast(node, claims, userGroups, userPerms) {
		return nil
	}

	if !node.IsDir {
		return copyNode(node)
	}

	// 3. For directories, filter children recursively
	filtered := copyNode(node)
	filtered.Children = nil

	for _, child := range node.Children {
		fc := s.filterNodeRecursive(ctx, child, claims, userGroups, userPerms)
		if fc != nil {
			filtered.Children = append(filtered.Children, fc)
		}
	}

	return filtered
}

// checkAccessFast checks access using pre-loaded maps (no DB queries in the hot path).
func (s *Server) checkAccessFast(node *models.FileNode, claims *auth.Claims, userGroups map[int]string, userPerms map[string]string) bool {
	// Owner always has access
	if node.OwnerID > 0 && node.OwnerID == claims.UserID {
		return true
	}

	// Check user file_permissions with path inheritance
	segments := sharing.PathSegments(node.Path)
	for _, seg := range segments {
		if perm, ok := userPerms[seg]; ok {
			if sharing.PermissionSatisfies(perm, "read") {
				return true
			}
		}
	}

	// Check group role-based access for files with group_id
	if node.GroupID > 0 {
		if role, ok := userGroups[node.GroupID]; ok {
			mappedPerm := sharing.RoleToPermission(role)
			if sharing.PermissionSatisfies(mappedPerm, "read") {
				return true
			}
		}
	}

	// Fall back to DB-based CheckAccess for group_permissions path inheritance
	return s.permissions.CheckAccess(context.Background(), claims.UserID, node.Path, "read", false)
}

// copyNode creates a shallow copy of a FileNode (without children).
func copyNode(node *models.FileNode) *models.FileNode {
	return &models.FileNode{
		ID:         node.ID,
		Name:       node.Name,
		Path:       node.Path,
		Size:       node.Size,
		ModTime:    node.ModTime,
		IsDir:      node.IsDir,
		Hash:       node.Hash,
		Version:    node.Version,
		OwnerID:    node.OwnerID,
		Visibility: node.Visibility,
		GroupID:    node.GroupID,
	}
}

func (s *Server) findNode(root *models.FileNode, path string) *models.FileNode {
	if root == nil {
		return nil
	}

	path = strings.TrimSuffix(path, "/")
	rootPath := strings.TrimSuffix(root.Path, "/")

	if rootPath == path || (path == "/" && rootPath == "/") {
		return root
	}

	for _, child := range root.Children {
		if found := s.findNode(child, path); found != nil {
			return found
		}
	}
	return nil
}

// ─── Content ────────────────────────────────────────────────────────────────

func (s *Server) handleContent(w http.ResponseWriter, r *http.Request) {
	pathParam := r.PathValue("path")
	if pathParam == "" {
		s.sendError(w, http.StatusBadRequest, "file path required")
		return
	}

	// Look up file - try by path first, then by ID (for FUSE client compat)
	fullPath := "/" + pathParam
	fileRow, _ := s.metadata.GetFileRow(r.Context(), fullPath)

	// Check read permission
	claims := auth.GetClaims(r.Context())
	if fileRow != nil && claims != nil {
		if !s.permissions.CheckAccess(r.Context(), claims.UserID, fullPath, "read", claims.IsAdmin) {
			s.sendError(w, http.StatusForbidden, "access denied")
			return
		}
	}

	var totalSize int64
	var lookupKey string
	var storageLocID *int
	var groupID *int

	if fileRow != nil {
		totalSize = fileRow.Size
		lookupKey = fileRow.S3Key
		storageLocID = fileRow.StorageLocID
		groupID = fileRow.GroupID
		// Set version/ETag headers
		if fileRow.Hash != "" {
			w.Header().Set("ETag", `"`+fileRow.Hash+`"`)
		}
		if fileRow.Version > 0 {
			w.Header().Set("X-Version", strconv.Itoa(fileRow.Version))
		}
	} else {
		// Fall back to ID-based lookup (FUSE client passes file ID)
		lookupID := pathParam
		var err error
		totalSize, err = s.metadata.GetFileSize(r.Context(), lookupID)
		if err != nil {
			s.sendError(w, http.StatusNotFound, "file not found: "+pathParam)
			return
		}
		// Need to get the s3 key and storage location
		s3Key, _ := s.metadata.GetS3Key(r.Context(), lookupID)
		lookupKey = s3Key
	}

	// Parse Range header
	offset, length, hasRange := parseRangeHeader(r.Header.Get("Range"), totalSize)

	// Resolve backend
	backend, _, err := s.storageRouter.ResolveForFile(r.Context(), storageLocID, groupID)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "no storage backend: "+err.Error())
		return
	}

	// Get content from backend
	reader, _, err := backend.GetObject(r.Context(), lookupKey, offset, length)
	if err != nil {
		metrics.RecordContentDownload(0, false)
		s.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer reader.Close()

	// Set Content-Type based on file extension
	ct := mime.TypeByExtension(filepath.Ext(fullPath))
	if ct == "" {
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)

	if hasRange {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", offset, offset+length-1, totalSize))
		w.Header().Set("Content-Length", strconv.FormatInt(length, 10))
		w.WriteHeader(http.StatusPartialContent)
	} else {
		w.Header().Set("Content-Length", strconv.FormatInt(totalSize, 10))
		w.WriteHeader(http.StatusOK)
	}

	n, err := io.Copy(w, reader)
	if err != nil {
		logging.Warn("content transfer error", zap.String("path", r.URL.Path), zap.Error(err))
	}
	metrics.RecordContentDownload(n, err == nil)

	// Track bandwidth
	if claims != nil {
		s.quotaStore.TrackBandwidth(r.Context(), claims.UserID, 0, n)
	}
}

// ─── Upload ─────────────────────────────────────────────────────────────────

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	path := "/" + r.PathValue("path")
	if path == "/" {
		s.sendError(w, http.StatusBadRequest, "path required")
		return
	}

	claims := auth.GetClaims(r.Context())

	// Check write permission
	if claims != nil && !s.permissions.CheckAccess(r.Context(), claims.UserID, path, "write", claims.IsAdmin) {
		s.sendError(w, http.StatusForbidden, "write access denied")
		return
	}

	// Determine effective upload size limit (per-user override or global)
	effectiveMaxUpload := s.maxUploadSize
	if claims != nil {
		userLimit, err := s.quotaStore.GetUploadSizeLimit(r.Context(), claims.UserID)
		if err == nil && userLimit > 0 {
			effectiveMaxUpload = userLimit
		}
	}

	// Check content length
	if r.ContentLength > effectiveMaxUpload {
		s.sendError(w, http.StatusRequestEntityTooLarge,
			fmt.Sprintf("file too large: max %d bytes", effectiveMaxUpload))
		return
	}

	// Limit reader to max upload size
	limitedReader := io.LimitReader(r.Body, effectiveMaxUpload+1)

	// Read content and compute hash
	content, err := io.ReadAll(limitedReader)
	if err != nil {
		metrics.RecordContentUpload(0, false)
		s.sendError(w, http.StatusInternalServerError, "failed to read content")
		return
	}

	if int64(len(content)) > effectiveMaxUpload {
		metrics.RecordContentUpload(0, false)
		s.sendError(w, http.StatusRequestEntityTooLarge,
			fmt.Sprintf("file too large: max %d bytes", effectiveMaxUpload))
		return
	}

	// Check storage quota
	if claims != nil {
		ok, err := s.quotaStore.CheckStorageQuota(r.Context(), claims.UserID, int64(len(content)))
		if err == nil && !ok {
			metrics.RecordQuotaExceeded("storage")
			s.sendError(w, http.StatusRequestEntityTooLarge, "storage quota exceeded")
			return
		}
	}

	hash := sha256.Sum256(content)
	hashStr := fmt.Sprintf("%x", hash)

	// S3 key is the path without leading /
	s3Key := strings.TrimPrefix(path, "/")

	// Check if file already exists (for versioning and conflict detection)
	newVersion := 1
	existingRow, _ := s.metadata.GetFileRow(r.Context(), path)

	// Conflict detection: check X-Expected-Version and If-Match headers
	if existingRow != nil && !existingRow.IsDir {
		if evStr := r.Header.Get("X-Expected-Version"); evStr != "" {
			expectedVersion, _ := strconv.Atoi(evStr)
			if expectedVersion > 0 && expectedVersion != existingRow.Version {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusConflict)
				json.NewEncoder(w).Encode(protocol.ConflictResponse{
					Error:           "version conflict",
					Path:            path,
					ExpectedVersion: expectedVersion,
					CurrentVersion:  existingRow.Version,
					CurrentHash:     existingRow.Hash,
				})
				return
			}
		}
		if ifMatch := r.Header.Get("If-Match"); ifMatch != "" {
			etag := strings.Trim(ifMatch, "\"")
			if etag != existingRow.Hash {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusConflict)
				json.NewEncoder(w).Encode(protocol.ConflictResponse{
					Error:           "content conflict (hash mismatch)",
					Path:            path,
					ExpectedVersion: existingRow.Version,
					CurrentVersion:  existingRow.Version,
					CurrentHash:     existingRow.Hash,
				})
				return
			}
		}
	}

	if existingRow != nil && !existingRow.IsDir && existingRow.Size > 0 {
		// Save current state as a version before overwriting
		if err := s.metadata.SaveVersion(r.Context(), path); err != nil {
			logging.Warn("failed to save version", zap.String("path", path), zap.Error(err))
		}

		// Resolve existing file's backend for version backup
		existBackend, _, _ := s.storageRouter.ResolveForFile(r.Context(), existingRow.StorageLocID, existingRow.GroupID)
		if existBackend != nil {
			versionKey := fmt.Sprintf("_versions/%s/%d", s3Key, existingRow.Version)
			if err := existBackend.CopyObject(r.Context(), existingRow.S3Key, versionKey); err != nil {
				logging.Warn("failed to backup version content", zap.String("path", path), zap.Error(err))
			}
		}

		newVersion = existingRow.Version + 1
	}

	// Resolve backend for upload
	var groupID *int
	if existingRow != nil {
		groupID = existingRow.GroupID
	}
	backend, loc, err := s.storageRouter.ResolveForUpload(r.Context(), path, groupID)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "no storage backend: "+err.Error())
		return
	}

	// Upload to backend
	if err := backend.PutObject(r.Context(), s3Key, strings.NewReader(string(content)), int64(len(content))); err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to upload: "+err.Error())
		return
	}

	// Ensure parent directories exist
	if err := s.ensureParentDirs(r.Context(), path); err != nil {
		logging.Error("failed to ensure parent dirs", zap.Error(err))
	}

	// Create/update metadata
	parentPath := filepath.Dir(path)
	if parentPath == "." {
		parentPath = "/"
	}

	storageLocID := &loc.ID
	fileRow := &postgres.FileRow{
		ID:           fileID(path),
		Name:         filepath.Base(path),
		Path:         path,
		ParentPath:   parentPath,
		Size:         int64(len(content)),
		ModTime:      time.Now(),
		IsDir:        false,
		Hash:         hashStr,
		S3Key:        s3Key,
		Version:      newVersion,
		StorageLocID: storageLocID,
	}

	// Set owner on first upload
	if claims != nil && existingRow == nil {
		ownerID := claims.UserID
		fileRow.OwnerID = &ownerID
	}

	if err := s.metadata.UpsertFile(r.Context(), fileRow); err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to save metadata: "+err.Error())
		return
	}

	// Refresh tree
	s.RefreshTree(r.Context())

	// Track bandwidth
	if claims != nil {
		s.quotaStore.TrackBandwidth(r.Context(), claims.UserID, int64(len(content)), 0)
	}

	logging.Info("file uploaded",
		zap.String("path", path),
		zap.Int("size", len(content)),
		zap.String("hash", hashStr[:16]),
		zap.Int("version", newVersion))

	// Publish SSE event
	eventType := events.EventCreate
	if existingRow != nil {
		eventType = events.EventModify
	}
	s.publishEvent(eventType, path, newVersion, hashStr, int64(len(content)))

	// Gallery: enqueue image processing if applicable
	if s.processor != nil && gallery.IsImageFile(path) {
		s.processor.Enqueue(path)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"path":    path,
		"size":    len(content),
		"hash":    hashStr,
		"version": newVersion,
	})
}

// ─── Create/Update ──────────────────────────────────────────────────────────

func (s *Server) handleCreateOrUpdate(w http.ResponseWriter, r *http.Request) {
	path := "/" + r.PathValue("path")
	if path == "/" {
		s.sendError(w, http.StatusBadRequest, "cannot modify root")
		return
	}

	claims := auth.GetClaims(r.Context())
	if claims != nil && !s.permissions.CheckAccess(r.Context(), claims.UserID, path, "write", claims.IsAdmin) {
		s.sendError(w, http.StatusForbidden, "write access denied")
		return
	}

	// Check if this is a directory creation
	isDir := r.URL.Query().Get("type") == "dir"

	if isDir {
		// Create directory
		if err := s.ensureParentDirs(r.Context(), path); err != nil {
			s.sendError(w, http.StatusInternalServerError, "failed to create parent dirs: "+err.Error())
			return
		}

		parentPath := filepath.Dir(path)
		if parentPath == "." {
			parentPath = "/"
		}

		fileRow := &postgres.FileRow{
			ID:         fileID(path),
			Name:       filepath.Base(path),
			Path:       path,
			ParentPath: parentPath,
			IsDir:      true,
			ModTime:    time.Now(),
		}

		if claims != nil {
			ownerID := claims.UserID
			fileRow.OwnerID = &ownerID
		}

		if err := s.metadata.UpsertFile(r.Context(), fileRow); err != nil {
			s.sendError(w, http.StatusInternalServerError, "failed to create directory: "+err.Error())
			return
		}

		s.RefreshTree(r.Context())

		logging.Info("directory created", zap.String("path", path))

		s.publishEvent(events.EventCreate, path, 0, "", 0)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"path":  path,
			"isDir": true,
		})
		return
	}

	// For files, this is metadata update only (content via POST /content)
	s.sendError(w, http.StatusBadRequest, "use POST /api/v1/content/{path} to upload files")
}

// ─── Delete ─────────────────────────────────────────────────────────────────

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	path := "/" + r.PathValue("path")
	if path == "/" {
		s.sendError(w, http.StatusBadRequest, "cannot delete root")
		return
	}

	// Check ownership or admin
	claims := auth.GetClaims(r.Context())
	if claims != nil && !claims.IsAdmin {
		ownerID, hasOwner := s.permissions.GetOwnerID(r.Context(), path)
		if hasOwner && ownerID != claims.UserID {
			if !s.permissions.CheckAccess(r.Context(), claims.UserID, path, "owner", false) {
				s.sendError(w, http.StatusForbidden, "only the owner or admin can delete")
				return
			}
		}
	}

	// Check if path exists
	fileRow, err := s.metadata.GetFileRow(r.Context(), path)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if fileRow == nil {
		s.sendError(w, http.StatusNotFound, "path not found: "+path)
		return
	}

	// Soft-delete: move to trash instead of permanent delete
	userID := 0
	if claims != nil {
		userID = claims.UserID
	}
	if err := s.metadata.SoftDeleteFile(r.Context(), path, userID); err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to delete: "+err.Error())
		return
	}

	s.RefreshTree(r.Context())

	logging.Info("file moved to trash", zap.String("path", path))

	s.publishEvent(events.EventDelete, path, 0, "", 0)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"path":    path,
		"trashed": true,
	})
}

// ─── Versions ───────────────────────────────────────────────────────────────

func (s *Server) handleVersionedFiles(w http.ResponseWriter, r *http.Request) {
	files, err := s.metadata.ListVersionedFiles(r.Context())
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to list versioned files: "+err.Error())
		return
	}
	if files == nil {
		files = []postgres.VersionedFileSummary{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(files)
}

func (s *Server) handleVersions(w http.ResponseWriter, r *http.Request) {
	path := "/" + r.PathValue("path")
	if path == "/" {
		s.sendError(w, http.StatusBadRequest, "path required")
		return
	}

	if vStr := r.URL.Query().Get("v"); vStr != "" {
		s.handleVersionContent(w, r, path, vStr)
		return
	}

	versions, currentVersion, err := s.metadata.ListVersions(r.Context(), path)
	if err != nil {
		s.sendError(w, http.StatusNotFound, "file not found or no versions: "+err.Error())
		return
	}

	resp := protocol.VersionListResponse{
		Path:           path,
		CurrentVersion: currentVersion,
	}
	for _, v := range versions {
		resp.Versions = append(resp.Versions, protocol.VersionInfo{
			Version:   v.Version,
			Size:      v.Size,
			Hash:      v.Hash,
			CreatedAt: v.CreatedAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleVersionContent(w http.ResponseWriter, r *http.Request, path, vStr string) {
	version, err := strconv.Atoi(vStr)
	if err != nil || version < 1 {
		s.sendError(w, http.StatusBadRequest, "invalid version number")
		return
	}

	vRecord, err := s.metadata.GetVersion(r.Context(), path, version)
	if err != nil {
		s.sendError(w, http.StatusNotFound, "version not found: "+err.Error())
		return
	}

	// Resolve backend from version record
	backend, _, err := s.storageRouter.ResolveForFile(r.Context(), vRecord.StorageLocID, nil)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "no storage backend: "+err.Error())
		return
	}

	versionS3Key := fmt.Sprintf("_versions/%s/%d", strings.TrimPrefix(path, "/"), version)
	reader, size, err := backend.GetObject(r.Context(), versionS3Key, 0, 0)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to retrieve version content: "+err.Error())
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	w.Header().Set("X-Version", strconv.Itoa(vRecord.Version))
	w.Header().Set("X-Version-Hash", vRecord.Hash)

	n, err := io.Copy(w, reader)
	if err != nil {
		logging.Warn("version content transfer error", zap.String("path", path), zap.Error(err))
	}
	metrics.RecordContentDownload(n, err == nil)
}

func (s *Server) handleRollback(w http.ResponseWriter, r *http.Request) {
	path := "/" + r.PathValue("path")
	if path == "/" {
		s.sendError(w, http.StatusBadRequest, "path required")
		return
	}

	var req protocol.RollbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Version < 1 {
		s.sendError(w, http.StatusBadRequest, "version must be >= 1")
		return
	}

	currentRow, err := s.metadata.GetFileRow(r.Context(), path)
	if err != nil || currentRow == nil {
		s.sendError(w, http.StatusNotFound, "file not found: "+path)
		return
	}

	// Resolve backend for this file
	backend, _, err := s.storageRouter.ResolveForFile(r.Context(), currentRow.StorageLocID, currentRow.GroupID)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "no storage backend: "+err.Error())
		return
	}

	// Save current state as a version before rollback
	if err := s.metadata.SaveVersion(r.Context(), path); err != nil {
		logging.Warn("failed to save pre-rollback version", zap.Error(err))
	}
	versionKey := fmt.Sprintf("_versions/%s/%d", strings.TrimPrefix(path, "/"), currentRow.Version)
	if err := backend.CopyObject(r.Context(), currentRow.S3Key, versionKey); err != nil {
		logging.Warn("failed to backup pre-rollback content", zap.Error(err))
	}

	// Copy version content back to current S3 key
	srcKey := fmt.Sprintf("_versions/%s/%d", strings.TrimPrefix(path, "/"), req.Version)
	if err := backend.CopyObject(r.Context(), srcKey, currentRow.S3Key); err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to restore content: "+err.Error())
		return
	}

	newVersion := currentRow.Version + 1
	if err := s.metadata.RestoreVersion(r.Context(), path, req.Version, newVersion, currentRow.S3Key); err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to restore metadata: "+err.Error())
		return
	}

	s.RefreshTree(r.Context())

	logging.Info("file rolled back",
		zap.String("path", path),
		zap.Int("to_version", req.Version),
		zap.Int("new_version", newVersion))

	s.publishEvent(events.EventVersion, path, newVersion, "", 0)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"path":             path,
		"restored_version": req.Version,
		"new_version":      newVersion,
	})
}

// ─── Permissions ────────────────────────────────────────────────────────────

func (s *Server) handleSetPermission(w http.ResponseWriter, r *http.Request) {
	path := "/" + r.PathValue("path")

	claims := auth.GetClaims(r.Context())
	if claims == nil {
		s.sendError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	// Only owner or admin can set permissions
	if !claims.IsAdmin {
		ownerID, hasOwner := s.permissions.GetOwnerID(r.Context(), path)
		if !hasOwner || ownerID != claims.UserID {
			s.sendError(w, http.StatusForbidden, "only the owner or admin can manage permissions")
			return
		}
	}

	var req protocol.PermissionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.UserID == 0 {
		s.sendError(w, http.StatusBadRequest, "user_id required")
		return
	}
	if req.Permission != "read" && req.Permission != "write" && req.Permission != "owner" {
		s.sendError(w, http.StatusBadRequest, "permission must be 'read', 'write', or 'owner'")
		return
	}

	if err := s.permissions.SetPermission(r.Context(), req.UserID, path, req.Permission); err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to set permission: "+err.Error())
		return
	}

	logging.Info("permission set",
		zap.String("path", path),
		zap.Int("user_id", req.UserID),
		zap.String("permission", req.Permission))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"path":       path,
		"user_id":    req.UserID,
		"permission": req.Permission,
	})
}

func (s *Server) handleListPermissions(w http.ResponseWriter, r *http.Request) {
	path := "/" + r.PathValue("path")

	claims := auth.GetClaims(r.Context())
	if claims == nil {
		s.sendError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	// Only owner or admin can list permissions
	if !claims.IsAdmin {
		ownerID, hasOwner := s.permissions.GetOwnerID(r.Context(), path)
		if !hasOwner || ownerID != claims.UserID {
			s.sendError(w, http.StatusForbidden, "only the owner or admin can view permissions")
			return
		}
	}

	perms, err := s.permissions.ListPermissions(r.Context(), path)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to list permissions: "+err.Error())
		return
	}

	resp := protocol.PermissionListResponse{Path: path}
	for _, p := range perms {
		resp.Permissions = append(resp.Permissions, protocol.PermissionResponse{
			UserID:     p.UserID,
			Username:   p.Username,
			Path:       p.Path,
			Permission: p.Permission,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleDeletePermission(w http.ResponseWriter, r *http.Request) {
	path := "/" + r.PathValue("path")

	claims := auth.GetClaims(r.Context())
	if claims == nil {
		s.sendError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	if !claims.IsAdmin {
		ownerID, hasOwner := s.permissions.GetOwnerID(r.Context(), path)
		if !hasOwner || ownerID != claims.UserID {
			s.sendError(w, http.StatusForbidden, "only the owner or admin can manage permissions")
			return
		}
	}

	userIDStr := r.URL.Query().Get("user_id")
	if userIDStr == "" {
		s.sendError(w, http.StatusBadRequest, "user_id query parameter required")
		return
	}
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid user_id")
		return
	}

	if err := s.permissions.RemovePermission(r.Context(), userID, path); err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to remove permission: "+err.Error())
		return
	}

	logging.Info("permission removed", zap.String("path", path), zap.Int("user_id", userID))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"path":    path,
		"user_id": userID,
		"removed": true,
	})
}

// ─── Share Links ────────────────────────────────────────────────────────────

func (s *Server) handleListUserShares(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		s.sendError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	links, err := s.shareLinks.ListByUser(r.Context(), claims.UserID)
	if err != nil {
		logging.Error("list user shares", zap.Error(err))
		s.sendError(w, http.StatusInternalServerError, "failed to list share links")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(links)
}

func (s *Server) handleCreateShareLink(w http.ResponseWriter, r *http.Request) {
	path := "/" + r.PathValue("path")

	claims := auth.GetClaims(r.Context())
	if claims == nil {
		s.sendError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	// Need at least read access to share
	if !s.permissions.CheckAccess(r.Context(), claims.UserID, path, "read", claims.IsAdmin) {
		s.sendError(w, http.StatusForbidden, "access denied")
		return
	}

	var req protocol.ShareLinkRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.sendError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}

	link, err := s.shareLinks.Create(r.Context(), path, claims.UserID, req.Password, req.ExpiresInSec, req.MaxDownloads)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to create share link: "+err.Error())
		return
	}

	// Build share URL pointing to the web app landing page
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	shareURL := fmt.Sprintf("%s://%s/app/#share/%s", scheme, r.Host, link.ID)
	if req.Password != "" {
		shareURL += "/" + req.Password
	}

	logging.Info("share link created",
		zap.String("path", path),
		zap.String("link_id", link.ID))

	resp := protocol.ShareLinkResponse{
		ID:           link.ID,
		Path:         link.Path,
		URL:          shareURL,
		ExpiresAt:    link.ExpiresAt,
		MaxDownloads: link.MaxDownloads,
		CreatedAt:    link.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleShareDownload(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		s.sendError(w, http.StatusBadRequest, "share token required")
		return
	}

	password := r.URL.Query().Get("password")

	link, err := s.shareLinks.Validate(r.Context(), token, password)
	if err != nil {
		s.sendError(w, http.StatusForbidden, err.Error())
		return
	}

	// Get file metadata
	fileRow, err := s.metadata.GetFileRow(r.Context(), link.Path)
	if err != nil || fileRow == nil {
		s.sendError(w, http.StatusNotFound, "shared file not found")
		return
	}

	// Resolve backend for this file
	backend, _, err := s.storageRouter.ResolveForFile(r.Context(), fileRow.StorageLocID, fileRow.GroupID)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "no storage backend: "+err.Error())
		return
	}

	// Get content
	reader, size, err := backend.GetObject(r.Context(), fileRow.S3Key, 0, 0)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to retrieve content: "+err.Error())
		return
	}
	defer reader.Close()

	// Increment download count
	s.shareLinks.IncrementDownloads(r.Context(), token)

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filepath.Base(link.Path)))
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	w.WriteHeader(http.StatusOK)

	n, err := io.Copy(w, reader)
	if err != nil {
		logging.Warn("share link transfer error", zap.String("token", token), zap.Error(err))
	}
	metrics.RecordContentDownload(n, err == nil)
}

func (s *Server) handleShareInfo(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		s.sendError(w, http.StatusBadRequest, "share token required")
		return
	}

	info, err := s.shareLinks.GetInfo(r.Context(), token)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(protocol.ShareInfoResponse{
			Valid: false,
			Error: "share link not found",
		})
		return
	}

	resp := protocol.ShareInfoResponse{
		HasPassword: info.HasPassword,
		ExpiresAt:   info.ExpiresAt,
		Valid:       info.Valid,
		Error:       info.Error,
	}

	// Look up file metadata for name and size
	if info.Valid {
		fileRow, err := s.metadata.GetFileRow(r.Context(), info.Path)
		if err == nil && fileRow != nil {
			resp.FileName = fileRow.Name
			resp.FileSize = fileRow.Size
		} else {
			resp.FileName = filepath.Base(info.Path)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleRevokeShareLink(w http.ResponseWriter, r *http.Request) {
	linkID := r.PathValue("id")
	if linkID == "" {
		s.sendError(w, http.StatusBadRequest, "share link ID required")
		return
	}

	claims := auth.GetClaims(r.Context())
	if claims == nil {
		s.sendError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	// Check ownership
	link, err := s.shareLinks.GetByID(r.Context(), linkID)
	if err != nil {
		s.sendError(w, http.StatusNotFound, "share link not found")
		return
	}
	if !claims.IsAdmin && link.CreatedBy != claims.UserID {
		s.sendError(w, http.StatusForbidden, "only the creator or admin can revoke")
		return
	}

	if err := s.shareLinks.Revoke(r.Context(), linkID); err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to revoke: "+err.Error())
		return
	}

	logging.Info("share link revoked", zap.String("link_id", linkID))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":      linkID,
		"revoked": true,
	})
}

// ─── Quotas ─────────────────────────────────────────────────────────────────

func (s *Server) handleGetQuota(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil || !claims.IsAdmin {
		s.sendError(w, http.StatusForbidden, "admin access required")
		return
	}

	userID, err := strconv.Atoi(r.PathValue("userID"))
	if err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	q, err := s.quotaStore.GetQuota(r.Context(), userID)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to get quota: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(protocol.UserQuotaResponse{
		UserID:             q.UserID,
		MaxStorageBytes:    q.MaxStorageBytes,
		MaxBandwidthPerDay: q.MaxBandwidthPerDay,
		MaxRequestsPerMin:  q.MaxRequestsPerMin,
		MaxUploadSizeBytes: q.MaxUploadSizeBytes,
	})
}

func (s *Server) handleSetQuota(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil || !claims.IsAdmin {
		s.sendError(w, http.StatusForbidden, "admin access required")
		return
	}

	userID, err := strconv.Atoi(r.PathValue("userID"))
	if err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	var req protocol.SetQuotaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Get current quota to merge
	current, err := s.quotaStore.GetQuota(r.Context(), userID)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to get current quota: "+err.Error())
		return
	}

	if req.MaxStorageBytes != nil {
		current.MaxStorageBytes = *req.MaxStorageBytes
	}
	if req.MaxBandwidthPerDay != nil {
		current.MaxBandwidthPerDay = *req.MaxBandwidthPerDay
	}
	if req.MaxRequestsPerMin != nil {
		current.MaxRequestsPerMin = *req.MaxRequestsPerMin
	}
	if req.MaxUploadSizeBytes != nil {
		current.MaxUploadSizeBytes = *req.MaxUploadSizeBytes
	}

	if err := s.quotaStore.SetQuota(r.Context(), current); err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to set quota: "+err.Error())
		return
	}

	logging.Info("quota set", zap.Int("user_id", userID))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(protocol.UserQuotaResponse{
		UserID:             current.UserID,
		MaxStorageBytes:    current.MaxStorageBytes,
		MaxBandwidthPerDay: current.MaxBandwidthPerDay,
		MaxRequestsPerMin:  current.MaxRequestsPerMin,
		MaxUploadSizeBytes: current.MaxUploadSizeBytes,
	})
}

func (s *Server) handleGetUsage(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		s.sendError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	q, err := s.quotaStore.GetQuota(r.Context(), claims.UserID)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to get quota: "+err.Error())
		return
	}

	storageUsed, err := s.quotaStore.GetStorageUsed(r.Context(), claims.UserID)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to get storage usage: "+err.Error())
		return
	}

	bIn, bOut, err := s.quotaStore.GetBandwidthToday(r.Context(), claims.UserID)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to get bandwidth: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(protocol.UsageResponse{
		UserID:         claims.UserID,
		StorageUsed:    storageUsed,
		BandwidthToday: bIn + bOut,
		Quota: protocol.UserQuotaResponse{
			UserID:             q.UserID,
			MaxStorageBytes:    q.MaxStorageBytes,
			MaxBandwidthPerDay: q.MaxBandwidthPerDay,
			MaxRequestsPerMin:  q.MaxRequestsPerMin,
			MaxUploadSizeBytes: q.MaxUploadSizeBytes,
		},
	})
}

// ─── User Dashboard ─────────────────────────────────────────────────────────

func (s *Server) handleUserDashboard(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		s.sendError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	ctx := r.Context()

	// Quota and usage
	q, err := s.quotaStore.GetQuota(ctx, claims.UserID)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to get quota: "+err.Error())
		return
	}

	var storageUsed int64
	if claims.IsAdmin {
		db := s.auth.DB()
		db.QueryRowContext(ctx,
			`SELECT COALESCE(SUM(size), 0) FROM files WHERE is_dir = FALSE AND deleted_at IS NULL`,
		).Scan(&storageUsed)
	} else {
		storageUsed, _ = s.quotaStore.GetStorageUsed(ctx, claims.UserID)
	}
	bIn, bOut, _ := s.quotaStore.GetBandwidthToday(ctx, claims.UserID)

	// Groups
	memberships, _ := s.groups.GetUserGroupsWithRoles(ctx, claims.UserID)
	var groups []protocol.UserGroupInfo
	for _, m := range memberships {
		groups = append(groups, protocol.UserGroupInfo{
			GroupID:   m.GroupID,
			GroupName: m.GroupName,
			Role:      m.Role,
		})
	}
	if groups == nil {
		groups = []protocol.UserGroupInfo{}
	}

	// File count (accessible to user)
	var fileCount int
	db := s.auth.DB()
	if claims.IsAdmin {
		db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM files WHERE is_dir = FALSE AND deleted_at IS NULL`,
		).Scan(&fileCount)
	} else {
		db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM files WHERE (owner_id = $1 OR visibility = 'public') AND is_dir = FALSE AND deleted_at IS NULL`,
			claims.UserID).Scan(&fileCount)
	}

	// Active share link count
	var shareLinkCount int
	db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM share_links WHERE created_by = $1 AND is_active = TRUE`,
		claims.UserID).Scan(&shareLinkCount)

	resp := protocol.UserDashboardResponse{
		UserID:         claims.UserID,
		Username:       claims.Username,
		StorageUsed:    storageUsed,
		BandwidthToday: bIn + bOut,
		Quota: protocol.UserQuotaResponse{
			UserID:             q.UserID,
			MaxStorageBytes:    q.MaxStorageBytes,
			MaxBandwidthPerDay: q.MaxBandwidthPerDay,
			MaxRequestsPerMin:  q.MaxRequestsPerMin,
			MaxUploadSizeBytes: q.MaxUploadSizeBytes,
		},
		Groups:         groups,
		FileCount:      fileCount,
		ShareLinkCount: shareLinkCount,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ─── File Properties ────────────────────────────────────────────────────────

func (s *Server) handleFileProperties(w http.ResponseWriter, r *http.Request) {
	path := "/" + r.PathValue("path")
	if path == "/" {
		s.sendError(w, http.StatusBadRequest, "path required")
		return
	}

	claims := auth.GetClaims(r.Context())
	if claims == nil {
		s.sendError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	// Check read permission
	if !s.permissions.CheckAccess(r.Context(), claims.UserID, path, "read", claims.IsAdmin) {
		s.sendError(w, http.StatusForbidden, "access denied")
		return
	}

	// Get file metadata
	node := s.findNode(s.tree, path)
	if node == nil {
		s.sendError(w, http.StatusNotFound, "path not found: "+path)
		return
	}

	resp := protocol.FilePropertiesResponse{
		ID:         node.ID,
		Name:       node.Name,
		Path:       node.Path,
		Size:       node.Size,
		ModTime:    node.ModTime,
		IsDir:      node.IsDir,
		Hash:       node.Hash,
		Version:    node.Version,
		OwnerID:    node.OwnerID,
		GroupID:    node.GroupID,
		Visibility: node.Visibility,
	}
	if resp.Visibility == "" {
		resp.Visibility = "public"
	}

	// Resolve owner name
	if node.OwnerID > 0 {
		if name, err := s.groups.GetUsernameByID(r.Context(), node.OwnerID); err == nil {
			resp.OwnerName = name
		} else {
			logging.Debug("properties: failed to resolve owner name", zap.Int("owner_id", node.OwnerID), zap.Error(err))
		}
	}

	// Resolve group name
	if node.GroupID > 0 {
		if name, err := s.groups.GetGroupNameByID(r.Context(), node.GroupID); err == nil {
			resp.GroupName = name
		} else {
			logging.Debug("properties: failed to resolve group name", zap.Int("group_id", node.GroupID), zap.Error(err))
		}
	}

	// Get permissions (only if owner or admin)
	isOwner := node.OwnerID > 0 && node.OwnerID == claims.UserID
	if claims.IsAdmin || isOwner {
		if perms, err := s.permissions.ListPermissions(r.Context(), path); err == nil {
			for _, p := range perms {
				resp.Permissions = append(resp.Permissions, protocol.PermissionResponse{
					UserID:     p.UserID,
					Username:   p.Username,
					Path:       p.Path,
					Permission: p.Permission,
				})
			}
		} else {
			logging.Debug("properties: failed to list permissions", zap.String("path", path), zap.Error(err))
		}

		// Get share links
		if links, err := s.shareLinks.ListByPath(r.Context(), path); err == nil {
			for _, l := range links {
				resp.ShareLinks = append(resp.ShareLinks, protocol.ShareLinkInfo{
					ID:            l.ID,
					CreatedBy:     l.CreatedByUser,
					ExpiresAt:     l.ExpiresAt,
					MaxDownloads:  l.MaxDownloads,
					DownloadCount: l.DownloadCount,
					CreatedAt:     l.CreatedAt,
				})
			}
		} else {
			logging.Debug("properties: failed to list share links", zap.String("path", path), zap.Error(err))
		}
	}

	// Get version count
	if !node.IsDir {
		if versions, _, err := s.metadata.ListVersions(r.Context(), path); err == nil {
			resp.VersionCount = len(versions)
		} else {
			logging.Debug("properties: failed to list versions", zap.String("path", path), zap.Error(err))
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ─── Helpers ────────────────────────────────────────────────────────────────

func (s *Server) ensureParentDirs(ctx context.Context, path string) error {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) <= 1 {
		return nil
	}

	currentPath := ""
	for i := 0; i < len(parts)-1; i++ {
		currentPath += "/" + parts[i]

		exists, err := s.metadata.PathExists(ctx, currentPath)
		if err != nil {
			return err
		}
		if exists {
			continue
		}

		parentPath := filepath.Dir(currentPath)
		if parentPath == "." {
			parentPath = "/"
		}

		fileRow := &postgres.FileRow{
			ID:         fileID(currentPath),
			Name:       parts[i],
			Path:       currentPath,
			ParentPath: parentPath,
			IsDir:      true,
			ModTime:    time.Now(),
		}

		if err := s.metadata.UpsertFile(ctx, fileRow); err != nil {
			return err
		}
	}

	return nil
}

func fileID(path string) string {
	h := sha256.Sum256([]byte(path))
	return fmt.Sprintf("%x", h[:8])
}

type gzipResponseWriter struct {
	http.ResponseWriter
	gw *gzip.Writer
}

func (g *gzipResponseWriter) Write(data []byte) (int, error) {
	return g.gw.Write(data)
}

func (g *gzipResponseWriter) Flush() {
	g.gw.Flush()
	if f, ok := g.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func acceptsGzip(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept-Encoding"), "gzip")
}

func parseRangeHeader(rangeHeader string, totalSize int64) (offset, length int64, hasRange bool) {
	if rangeHeader == "" {
		return 0, totalSize, false
	}

	matches := rangeRegex.FindStringSubmatch(rangeHeader)
	if matches == nil {
		return 0, totalSize, false
	}

	startStr, endStr := matches[1], matches[2]

	if startStr == "" && endStr != "" {
		suffix, _ := strconv.ParseInt(endStr, 10, 64)
		offset = totalSize - suffix
		if offset < 0 {
			offset = 0
		}
		length = totalSize - offset
		return offset, length, true
	}

	if startStr != "" {
		offset, _ = strconv.ParseInt(startStr, 10, 64)
	}

	if endStr != "" {
		end, _ := strconv.ParseInt(endStr, 10, 64)
		length = end - offset + 1
	} else {
		length = totalSize - offset
	}

	if offset >= totalSize {
		offset = totalSize - 1
	}
	if offset+length > totalSize {
		length = totalSize - offset
	}

	return offset, length, true
}

func (s *Server) sendError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(protocol.ErrorResponse{
		Error: message,
		Code:  code,
	})
}
