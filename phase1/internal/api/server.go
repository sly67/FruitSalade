// Package api provides the HTTP server and handlers for Phase 1.
package api

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/fruitsalade/fruitsalade/phase1/internal/auth"
	s3storage "github.com/fruitsalade/fruitsalade/phase1/internal/storage/s3"
	"github.com/fruitsalade/fruitsalade/shared/pkg/logger"
	"github.com/fruitsalade/fruitsalade/shared/pkg/models"
	"github.com/fruitsalade/fruitsalade/shared/pkg/protocol"
)

// Server is the Phase 1 HTTP server.
type Server struct {
	storage *s3storage.Storage
	auth    *auth.Auth
	tree    *models.FileNode
}

// NewServer creates a new Phase 1 server.
func NewServer(storage *s3storage.Storage, authHandler *auth.Auth) *Server {
	return &Server{
		storage: storage,
		auth:    authHandler,
	}
}

// Init initializes the server by building the metadata tree.
func (s *Server) Init(ctx context.Context) error {
	logger.Info("Building metadata tree from database...")
	tree, err := s.storage.BuildTree(ctx)
	if err != nil {
		return fmt.Errorf("build tree: %w", err)
	}
	s.tree = tree
	logger.Info("Metadata tree built: %d items", countNodes(tree))
	return nil
}

// RefreshTree rebuilds the metadata tree.
func (s *Server) RefreshTree(ctx context.Context) error {
	tree, err := s.storage.BuildTree(ctx)
	if err != nil {
		return err
	}
	s.tree = tree
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

// Handler returns the HTTP handler with auth middleware.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Public endpoints (no auth required)
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("POST /api/v1/auth/token", s.auth.HandleLogin)

	// Protected endpoints
	protected := http.NewServeMux()
	protected.HandleFunc("GET /api/v1/tree", s.handleTree)
	protected.HandleFunc("GET /api/v1/tree/{path...}", s.handleSubtree)
	protected.HandleFunc("GET /api/v1/content/{path...}", s.handleContent)

	// Wrap protected routes with auth middleware
	mux.Handle("/api/v1/", s.auth.Middleware(protected))

	return loggingMiddleware(mux)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.Debug("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
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

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "version": "phase1"})
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
		s.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer reader.Close()

	if hasRange {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", offset, offset+length-1, totalSize))
		w.Header().Set("Content-Length", strconv.FormatInt(length, 10))
		w.WriteHeader(http.StatusPartialContent)
	} else {
		w.Header().Set("Content-Length", strconv.FormatInt(totalSize, 10))
		w.WriteHeader(http.StatusOK)
	}

	io.Copy(w, reader)
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

func (s *Server) sendError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(protocol.ErrorResponse{
		Error: message,
		Code:  code,
	})
}
