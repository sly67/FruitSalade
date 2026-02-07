// Package api provides the HTTP server and handlers for Phase 2.
package api

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/fruitsalade/fruitsalade/phase2/internal/auth"
	"github.com/fruitsalade/fruitsalade/phase2/internal/logging"
	"github.com/fruitsalade/fruitsalade/phase2/internal/metadata/postgres"
	"github.com/fruitsalade/fruitsalade/phase2/internal/metrics"
	s3storage "github.com/fruitsalade/fruitsalade/phase2/internal/storage/s3"
	"github.com/fruitsalade/fruitsalade/shared/pkg/models"
	"github.com/fruitsalade/fruitsalade/shared/pkg/protocol"
)

// Server is the Phase 2 HTTP server.
type Server struct {
	storage       *s3storage.Storage
	auth          *auth.Auth
	tree          *models.FileNode
	maxUploadSize int64
}

// NewServer creates a new Phase 2 server.
func NewServer(storage *s3storage.Storage, authHandler *auth.Auth, maxUploadSize int64) *Server {
	return &Server{
		storage:       storage,
		auth:          authHandler,
		maxUploadSize: maxUploadSize,
	}
}

// Init initializes the server by building the metadata tree.
func (s *Server) Init(ctx context.Context) error {
	logging.Info("building metadata tree from database...")
	tree, err := s.storage.BuildTree(ctx)
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
	tree, err := s.storage.BuildTree(ctx)
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

	// Protected endpoints
	protected := http.NewServeMux()

	// Read endpoints
	protected.HandleFunc("GET /api/v1/tree", s.handleTree)
	protected.HandleFunc("GET /api/v1/tree/{path...}", s.handleSubtree)
	protected.HandleFunc("GET /api/v1/content/{path...}", s.handleContent)

	// Write endpoints (Phase 2)
	protected.HandleFunc("POST /api/v1/content/{path...}", s.handleUpload)
	protected.HandleFunc("PUT /api/v1/tree/{path...}", s.handleCreateOrUpdate)
	protected.HandleFunc("DELETE /api/v1/tree/{path...}", s.handleDelete)

	// Version endpoints (Phase 2)
	// GET  /api/v1/versions/{path} → list versions
	// GET  /api/v1/versions/{path}?v=N → download version content
	// POST /api/v1/versions/{path} → rollback (body: {"version": N})
	protected.HandleFunc("GET /api/v1/versions/{path...}", s.handleVersions)
	protected.HandleFunc("POST /api/v1/versions/{path...}", s.handleRollback)

	// Wrap protected routes with auth middleware
	mux.Handle("/api/v1/", s.auth.Middleware(protected))

	// Apply logging and metrics middleware
	return metrics.Middleware(logging.Middleware(mux))
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "version": "phase2"})
}

func (s *Server) handleTree(w http.ResponseWriter, r *http.Request) {
	if s.tree == nil {
		s.sendError(w, http.StatusInternalServerError, "metadata not initialized")
		return
	}

	resp := protocol.TreeResponse{Root: s.tree}

	if acceptsGzip(r) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Encoding", "gzip")
		gw := gzip.NewWriter(w)
		defer gw.Close()
		json.NewEncoder(gw).Encode(resp)
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

	resp := protocol.TreeResponse{Root: node}

	if acceptsGzip(r) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Encoding", "gzip")
		gw := gzip.NewWriter(w)
		defer gw.Close()
		json.NewEncoder(gw).Encode(resp)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
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

func (s *Server) handleContent(w http.ResponseWriter, r *http.Request) {
	fileID := r.PathValue("path")
	if fileID == "" {
		s.sendError(w, http.StatusBadRequest, "file ID required")
		return
	}

	// Get total file size
	totalSize, err := s.storage.GetContentSize(r.Context(), fileID)
	if err != nil {
		s.sendError(w, http.StatusNotFound, "file not found: "+fileID)
		return
	}

	// Parse Range header
	offset, length, hasRange := parseRangeHeader(r.Header.Get("Range"), totalSize)

	// Get content from S3
	reader, _, err := s.storage.GetContent(r.Context(), fileID, offset, length)
	if err != nil {
		metrics.RecordContentDownload(0, false)
		s.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer reader.Close()

	// Add version/ETag headers for conflict detection
	if fileRow, err := s.storage.Metadata().GetFileRow(r.Context(), "/"+fileID); err == nil && fileRow != nil {
		if fileRow.Hash != "" {
			w.Header().Set("ETag", `"`+fileRow.Hash+`"`)
		}
		if fileRow.Version > 0 {
			w.Header().Set("X-Version", strconv.Itoa(fileRow.Version))
		}
	}

	if hasRange {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", offset, offset+length-1, totalSize))
		w.Header().Set("Content-Length", strconv.FormatInt(length, 10))
		w.WriteHeader(http.StatusPartialContent)
	} else {
		w.Header().Set("Content-Length", strconv.FormatInt(totalSize, 10))
		w.WriteHeader(http.StatusOK)
	}

	n, _ := io.Copy(w, reader)
	metrics.RecordContentDownload(n, true)
}

// handleUpload handles POST /api/v1/content/{path}
// Uploads file content to S3 and creates/updates metadata.
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	path := "/" + r.PathValue("path")
	if path == "/" {
		s.sendError(w, http.StatusBadRequest, "path required")
		return
	}

	// Check content length
	if r.ContentLength > s.maxUploadSize {
		s.sendError(w, http.StatusRequestEntityTooLarge,
			fmt.Sprintf("file too large: max %d bytes", s.maxUploadSize))
		return
	}

	// Limit reader to max upload size
	limitedReader := io.LimitReader(r.Body, s.maxUploadSize+1)

	// Read content and compute hash
	content, err := io.ReadAll(limitedReader)
	if err != nil {
		metrics.RecordContentUpload(0, false)
		s.sendError(w, http.StatusInternalServerError, "failed to read content")
		return
	}

	if int64(len(content)) > s.maxUploadSize {
		metrics.RecordContentUpload(0, false)
		s.sendError(w, http.StatusRequestEntityTooLarge,
			fmt.Sprintf("file too large: max %d bytes", s.maxUploadSize))
		return
	}

	hash := sha256.Sum256(content)
	hashStr := fmt.Sprintf("%x", hash)

	// S3 key is the path without leading /
	s3Key := strings.TrimPrefix(path, "/")

	// Check if file already exists (for versioning and conflict detection)
	newVersion := 1
	existingRow, _ := s.storage.Metadata().GetFileRow(r.Context(), path)

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
		if err := s.storage.Metadata().SaveVersion(r.Context(), path); err != nil {
			logging.Warn("failed to save version", zap.String("path", path), zap.Error(err))
		}

		// Copy current S3 content to version backup key
		versionKey := fmt.Sprintf("_versions/%s/%d", s3Key, existingRow.Version)
		if err := s.storage.CopyObject(r.Context(), existingRow.S3Key, versionKey); err != nil {
			logging.Warn("failed to backup version content", zap.String("path", path), zap.Error(err))
		}

		newVersion = existingRow.Version + 1
	}

	// Upload to S3
	if err := s.storage.PutObject(r.Context(), s3Key, strings.NewReader(string(content)), int64(len(content))); err != nil {
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

	fileRow := &postgres.FileRow{
		ID:         fileID(path),
		Name:       filepath.Base(path),
		Path:       path,
		ParentPath: parentPath,
		Size:       int64(len(content)),
		ModTime:    time.Now(),
		IsDir:      false,
		Hash:       hashStr,
		S3Key:      s3Key,
		Version:    newVersion,
	}

	if err := s.storage.Metadata().UpsertFile(r.Context(), fileRow); err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to save metadata: "+err.Error())
		return
	}

	// Refresh tree
	s.RefreshTree(r.Context())

	logging.Info("file uploaded",
		zap.String("path", path),
		zap.Int("size", len(content)),
		zap.String("hash", hashStr[:16]),
		zap.Int("version", newVersion))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"path":    path,
		"size":    len(content),
		"hash":    hashStr,
		"version": newVersion,
	})
}

// handleCreateOrUpdate handles PUT /api/v1/tree/{path}
// Creates a directory or updates file metadata.
func (s *Server) handleCreateOrUpdate(w http.ResponseWriter, r *http.Request) {
	path := "/" + r.PathValue("path")
	if path == "/" {
		s.sendError(w, http.StatusBadRequest, "cannot modify root")
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

		if err := s.storage.Metadata().UpsertFile(r.Context(), fileRow); err != nil {
			s.sendError(w, http.StatusInternalServerError, "failed to create directory: "+err.Error())
			return
		}

		s.RefreshTree(r.Context())

		logging.Info("directory created", zap.String("path", path))

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

// handleDelete handles DELETE /api/v1/tree/{path}
// Deletes a file or directory (and all children).
func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	path := "/" + r.PathValue("path")
	if path == "/" {
		s.sendError(w, http.StatusBadRequest, "cannot delete root")
		return
	}

	// Check if path exists
	fileRow, err := s.storage.Metadata().GetFileRow(r.Context(), path)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if fileRow == nil {
		s.sendError(w, http.StatusNotFound, "path not found: "+path)
		return
	}

	if fileRow.IsDir {
		// Delete directory and all children from S3
		// First, list all files under this path
		rows, err := s.storage.Metadata().ListDir(r.Context(), path)
		if err == nil {
			for _, row := range rows {
				if !row.IsDir {
					s3Key := strings.TrimPrefix(row.Path, "/")
					s.storage.DeleteObject(r.Context(), s3Key)
				}
			}
		}

		// Delete from metadata (cascading delete)
		deleted, err := s.storage.Metadata().DeleteTree(r.Context(), path)
		if err != nil {
			s.sendError(w, http.StatusInternalServerError, "failed to delete: "+err.Error())
			return
		}

		s.RefreshTree(r.Context())

		logging.Info("directory deleted", zap.String("path", path), zap.Int64("items", deleted))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"path":    path,
			"deleted": deleted,
		})
	} else {
		// Delete file from S3
		s3Key := strings.TrimPrefix(path, "/")
		if err := s.storage.DeleteObject(r.Context(), s3Key); err != nil {
			logging.Warn("failed to delete from S3", zap.String("key", s3Key), zap.Error(err))
		}

		// Delete from metadata
		if err := s.storage.Metadata().DeleteFile(r.Context(), path); err != nil {
			s.sendError(w, http.StatusInternalServerError, "failed to delete: "+err.Error())
			return
		}

		s.RefreshTree(r.Context())

		logging.Info("file deleted", zap.String("path", path))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"path":    path,
			"deleted": 1,
		})
	}
}

// ensureParentDirs creates all parent directories for a path.
func (s *Server) ensureParentDirs(ctx context.Context, path string) error {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) <= 1 {
		return nil
	}

	currentPath := ""
	for i := 0; i < len(parts)-1; i++ {
		currentPath += "/" + parts[i]

		exists, err := s.storage.Metadata().PathExists(ctx, currentPath)
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

		if err := s.storage.Metadata().UpsertFile(ctx, fileRow); err != nil {
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

func acceptsGzip(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept-Encoding"), "gzip")
}

func parseRangeHeader(rangeHeader string, totalSize int64) (offset, length int64, hasRange bool) {
	if rangeHeader == "" {
		return 0, totalSize, false
	}

	re := regexp.MustCompile(`bytes=(\d*)-(\d*)`)
	matches := re.FindStringSubmatch(rangeHeader)
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

// handleVersions handles GET /api/v1/versions/{path}
// Without ?v= query: lists all versions.
// With ?v=N query: downloads version N content.
func (s *Server) handleVersions(w http.ResponseWriter, r *http.Request) {
	path := "/" + r.PathValue("path")
	if path == "/" {
		s.sendError(w, http.StatusBadRequest, "path required")
		return
	}

	// Check if requesting specific version content
	if vStr := r.URL.Query().Get("v"); vStr != "" {
		s.handleVersionContent(w, r, path, vStr)
		return
	}

	// List all versions
	versions, currentVersion, err := s.storage.Metadata().ListVersions(r.Context(), path)
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

// handleVersionContent serves the content of a specific version.
func (s *Server) handleVersionContent(w http.ResponseWriter, r *http.Request, path, vStr string) {
	version, err := strconv.Atoi(vStr)
	if err != nil || version < 1 {
		s.sendError(w, http.StatusBadRequest, "invalid version number")
		return
	}

	vRecord, err := s.storage.Metadata().GetVersion(r.Context(), path, version)
	if err != nil {
		s.sendError(w, http.StatusNotFound, "version not found: "+err.Error())
		return
	}

	// The version content is stored at the backup key
	versionS3Key := fmt.Sprintf("_versions/%s/%d", strings.TrimPrefix(path, "/"), version)
	reader, size, err := s.storage.GetContentByS3Key(r.Context(), versionS3Key)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to retrieve version content: "+err.Error())
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	w.Header().Set("X-Version", strconv.Itoa(vRecord.Version))
	w.Header().Set("X-Version-Hash", vRecord.Hash)

	n, _ := io.Copy(w, reader)
	metrics.RecordContentDownload(n, true)
}

// handleRollback handles POST /api/v1/versions/{path}
// Restores a file to a previous version.
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

	// Get current file state
	currentRow, err := s.storage.Metadata().GetFileRow(r.Context(), path)
	if err != nil || currentRow == nil {
		s.sendError(w, http.StatusNotFound, "file not found: "+path)
		return
	}

	// Save current state as a version before rollback
	if err := s.storage.Metadata().SaveVersion(r.Context(), path); err != nil {
		logging.Warn("failed to save pre-rollback version", zap.Error(err))
	}
	versionKey := fmt.Sprintf("_versions/%s/%d", strings.TrimPrefix(path, "/"), currentRow.Version)
	if err := s.storage.CopyObject(r.Context(), currentRow.S3Key, versionKey); err != nil {
		logging.Warn("failed to backup pre-rollback content", zap.Error(err))
	}

	// Copy version content back to current S3 key
	srcKey := fmt.Sprintf("_versions/%s/%d", strings.TrimPrefix(path, "/"), req.Version)
	if err := s.storage.CopyObject(r.Context(), srcKey, currentRow.S3Key); err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to restore content: "+err.Error())
		return
	}

	// Update file metadata with the version's info
	newVersion := currentRow.Version + 1
	if err := s.storage.Metadata().RestoreVersion(r.Context(), path, req.Version, newVersion, currentRow.S3Key); err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to restore metadata: "+err.Error())
		return
	}

	// Refresh tree
	s.RefreshTree(r.Context())

	logging.Info("file rolled back",
		zap.String("path", path),
		zap.Int("to_version", req.Version),
		zap.Int("new_version", newVersion))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"path":            path,
		"restored_version": req.Version,
		"new_version":     newVersion,
	})
}

func (s *Server) sendError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(protocol.ErrorResponse{
		Error: message,
		Code:  code,
	})
}
