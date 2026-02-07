// Package api provides the HTTP server and handlers for Phase 0.
package api

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/fruitsalade/fruitsalade/phase0/internal/watcher"
	"github.com/fruitsalade/fruitsalade/shared/pkg/models"
	"github.com/fruitsalade/fruitsalade/shared/pkg/protocol"
)

// Storage interface for the server.
type Storage interface {
	BuildTree(ctx context.Context) (*models.FileNode, error)
	GetMetadata(ctx context.Context, path string) (*models.FileNode, error)
	ListDir(ctx context.Context, path string) ([]*models.FileNode, error)
	GetContent(ctx context.Context, id string, offset, length int64) (io.ReadCloser, int64, error)
	GetContentSize(ctx context.Context, id string) (int64, error)
}

// Server is the Phase 0 HTTP server.
type Server struct {
	storage Storage
	tree    *models.FileNode // Cached metadata tree
	watcher *watcher.Watcher // File system watcher (optional)
}

// SetWatcher sets the file system watcher for SSE events.
func (s *Server) SetWatcher(w *watcher.Watcher) {
	s.watcher = w
}

// NewServer creates a new Phase 0 server.
func NewServer(storage Storage) *Server {
	return &Server{
		storage: storage,
	}
}

// Init initializes the server by building the metadata tree.
func (s *Server) Init(ctx context.Context) error {
	log.Println("Building metadata tree...")
	tree, err := s.storage.BuildTree(ctx)
	if err != nil {
		return fmt.Errorf("build tree: %w", err)
	}
	s.tree = tree
	log.Printf("Metadata tree built: %d items", countNodes(tree))
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

// Handler returns the HTTP handler for the server.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("GET /health", s.handleHealth)

	// Metadata API
	mux.HandleFunc("GET /api/v1/tree", s.handleTree)
	mux.HandleFunc("GET /api/v1/tree/{path...}", s.handleSubtree)

	// Content API
	mux.HandleFunc("GET /api/v1/content/{path...}", s.handleContent)

	// Events API (SSE)
	mux.HandleFunc("GET /api/v1/events", s.handleEvents)

	// Wrap with logging middleware
	return loggingMiddleware(mux)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

// gzipResponseWriter wraps http.ResponseWriter with gzip compression.
type gzipResponseWriter struct {
	http.ResponseWriter
	gw *gzip.Writer
}

func (g *gzipResponseWriter) Write(data []byte) (int, error) {
	return g.gw.Write(data)
}

func (g *gzipResponseWriter) Close() error {
	return g.gw.Close()
}

// acceptsGzip returns true if the client accepts gzip encoding.
func acceptsGzip(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept-Encoding"), "gzip")
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleTree(w http.ResponseWriter, r *http.Request) {
	if s.tree == nil {
		s.sendError(w, http.StatusInternalServerError, "metadata not initialized")
		return
	}

	resp := protocol.TreeResponse{Root: s.tree}

	// Use gzip if client accepts it
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

	// Find node in tree
	node := s.findNode(s.tree, "/"+path)
	if node == nil {
		s.sendError(w, http.StatusNotFound, "path not found: "+path)
		return
	}

	resp := protocol.TreeResponse{Root: node}

	// Use gzip if client accepts it
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

	// Normalize paths
	path = strings.TrimSuffix(path, "/")
	rootPath := strings.TrimSuffix(root.Path, "/")

	if rootPath == path || (path == "/" && rootPath == "/") {
		return root
	}

	// Search children
	for _, child := range root.Children {
		if found := s.findNode(child, path); found != nil {
			return found
		}
	}

	return nil
}

func (s *Server) handleContent(w http.ResponseWriter, r *http.Request) {
	path := r.PathValue("path")
	if path == "" {
		s.sendError(w, http.StatusBadRequest, "path required")
		return
	}

	// Get total file size first
	totalSize, err := s.storage.GetContentSize(r.Context(), path)
	if err != nil {
		s.sendError(w, http.StatusNotFound, "file not found: "+path)
		return
	}

	// Parse Range header
	offset, length, hasRange := parseRangeHeader(r.Header.Get("Range"), totalSize)

	// Get content
	reader, _, err := s.storage.GetContent(r.Context(), path, offset, length)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer reader.Close()

	// Set headers
	if hasRange {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", offset, offset+length-1, totalSize))
		w.Header().Set("Content-Length", strconv.FormatInt(length, 10))
		w.WriteHeader(http.StatusPartialContent)
	} else {
		w.Header().Set("Content-Length", strconv.FormatInt(totalSize, 10))
		w.WriteHeader(http.StatusOK)
	}

	// Stream content
	io.Copy(w, reader)
}

// parseRangeHeader parses the Range header and returns offset, length, and whether a range was specified.
// Supports: "bytes=start-end", "bytes=start-", "bytes=-suffix"
func parseRangeHeader(rangeHeader string, totalSize int64) (offset, length int64, hasRange bool) {
	if rangeHeader == "" {
		return 0, totalSize, false
	}

	// Match "bytes=start-end" or "bytes=start-" or "bytes=-suffix"
	re := regexp.MustCompile(`bytes=(\d*)-(\d*)`)
	matches := re.FindStringSubmatch(rangeHeader)
	if matches == nil {
		return 0, totalSize, false
	}

	startStr, endStr := matches[1], matches[2]

	if startStr == "" && endStr != "" {
		// Suffix range: bytes=-500 (last 500 bytes)
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
		// Open-ended: bytes=500-
		length = totalSize - offset
	}

	// Clamp to file size
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

// handleEvents handles Server-Sent Events for file system changes.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if s.watcher == nil {
		s.sendError(w, http.StatusServiceUnavailable, "file watching not enabled")
		return
	}

	// Check if client supports SSE
	flusher, ok := w.(http.Flusher)
	if !ok {
		s.sendError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Subscribe to watcher events
	eventCh := s.watcher.Subscribe()
	defer s.watcher.Unsubscribe(eventCh)

	log.Printf("SSE client connected: %s", r.RemoteAddr)

	// Send initial keepalive
	fmt.Fprintf(w, ": connected\n\n")
	flusher.Flush()

	// Stream events until client disconnects
	for {
		select {
		case <-r.Context().Done():
			log.Printf("SSE client disconnected: %s", r.RemoteAddr)
			return
		case event, ok := <-eventCh:
			if !ok {
				return
			}
			// Encode event as JSON
			data, err := json.Marshal(event)
			if err != nil {
				log.Printf("Failed to marshal event: %v", err)
				continue
			}
			// Send SSE formatted event
			fmt.Fprintf(w, "event: %s\n", event.Type)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}
