package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/fruitsalade/fruitsalade/fruitsalade/internal/auth"
	"github.com/fruitsalade/fruitsalade/fruitsalade/internal/events"
	"github.com/fruitsalade/fruitsalade/fruitsalade/internal/gallery"
	"github.com/fruitsalade/fruitsalade/fruitsalade/internal/logging"
	"github.com/fruitsalade/fruitsalade/fruitsalade/internal/metadata/postgres"
	"github.com/fruitsalade/fruitsalade/fruitsalade/internal/metrics"
	"go.uber.org/zap"
)

const (
	defaultChunkSize   = 5 * 1024 * 1024 // 5 MB
	defaultUploadExpiry = 24 * time.Hour
	cleanupInterval     = 15 * time.Minute
)

// ChunkedUploadManager handles chunked file uploads with resume support.
type ChunkedUploadManager struct {
	tempDir   string
	db        *sql.DB
	chunkSize int
	expiry    time.Duration
	server    *Server // back-reference for shared upload logic
}

// NewChunkedUploadManager creates a new chunked upload manager.
func NewChunkedUploadManager(db *sql.DB, tempDir string, server *Server) *ChunkedUploadManager {
	m := &ChunkedUploadManager{
		tempDir:   tempDir,
		db:        db,
		chunkSize: defaultChunkSize,
		expiry:    defaultUploadExpiry,
		server:    server,
	}
	return m
}

// StartCleanup starts the background goroutine that cleans up expired uploads.
func (m *ChunkedUploadManager) StartCleanup(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(cleanupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.cleanupExpired(ctx)
			}
		}
	}()
}

// tempPath returns the temp file path for an upload.
func (m *ChunkedUploadManager) tempPath(uploadID string) string {
	return filepath.Join(m.tempDir, uploadID+".part")
}

// ─── Init Upload ────────────────────────────────────────────────────────────

type initUploadRequest struct {
	Path     string `json:"path"`
	FileName string `json:"fileName"`
	FileSize int64  `json:"fileSize"`
}

type initUploadResponse struct {
	UploadID    string `json:"uploadId"`
	ChunkSize   int    `json:"chunkSize"`
	TotalChunks int    `json:"totalChunks"`
}

func (m *ChunkedUploadManager) handleInitUpload(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		m.sendError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req initUploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		m.sendError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Path == "" || req.FileName == "" || req.FileSize <= 0 {
		m.sendError(w, http.StatusBadRequest, "path, fileName, and fileSize are required")
		return
	}

	// Normalize path
	path := req.Path
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// Check write permission
	if !m.server.permissions.CheckAccess(r.Context(), claims.UserID, path, "write", claims.IsAdmin) {
		m.sendError(w, http.StatusForbidden, "write access denied")
		return
	}

	// Chunked uploads have no per-file size limit — the whole point
	// is to handle arbitrarily large files. Storage quota still applies.

	// Check storage quota
	ok, err := m.server.quotaStore.CheckStorageQuota(r.Context(), claims.UserID, req.FileSize)
	if err == nil && !ok {
		metrics.RecordQuotaExceeded("storage")
		m.sendError(w, http.StatusRequestEntityTooLarge, "storage quota exceeded")
		return
	}

	// Calculate chunks
	totalChunks := int((req.FileSize + int64(m.chunkSize) - 1) / int64(m.chunkSize))

	uploadID := generateUploadID()
	expiresAt := time.Now().Add(m.expiry)

	// Create DB record
	_, err = m.db.ExecContext(r.Context(),
		`INSERT INTO chunked_uploads (id, user_id, path, file_name, file_size, chunk_size, total_chunks, status, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, 'active', $8)`,
		uploadID, claims.UserID, path, req.FileName, req.FileSize, m.chunkSize, totalChunks, expiresAt)
	if err != nil {
		m.sendError(w, http.StatusInternalServerError, "failed to create upload record: "+err.Error())
		return
	}

	// Ensure temp dir exists
	if err := os.MkdirAll(m.tempDir, 0o755); err != nil {
		m.sendError(w, http.StatusInternalServerError, "failed to create temp directory")
		return
	}

	// Create sparse temp file
	f, err := os.Create(m.tempPath(uploadID))
	if err != nil {
		m.sendError(w, http.StatusInternalServerError, "failed to create temp file")
		return
	}
	// Pre-allocate the file to the expected size
	if err := f.Truncate(req.FileSize); err != nil {
		f.Close()
		m.sendError(w, http.StatusInternalServerError, "failed to allocate temp file")
		return
	}
	f.Close()

	logging.Info("chunked upload initiated",
		zap.String("upload_id", uploadID),
		zap.String("path", path),
		zap.Int64("size", req.FileSize),
		zap.Int("chunks", totalChunks))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(initUploadResponse{
		UploadID:    uploadID,
		ChunkSize:   m.chunkSize,
		TotalChunks: totalChunks,
	})
}

// ─── Upload Chunk ───────────────────────────────────────────────────────────

func (m *ChunkedUploadManager) handleUploadChunk(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		m.sendError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	uploadID := r.PathValue("uploadId")
	chunkIndexStr := r.PathValue("chunkIndex")
	chunkIndex, err := strconv.Atoi(chunkIndexStr)
	if err != nil || chunkIndex < 0 {
		m.sendError(w, http.StatusBadRequest, "invalid chunk index")
		return
	}

	// Verify upload exists and belongs to user
	var userID int
	var fileSize int64
	var chunkSize int
	var totalChunks int
	var status string
	err = m.db.QueryRowContext(r.Context(),
		`SELECT user_id, file_size, chunk_size, total_chunks, status FROM chunked_uploads WHERE id = $1`,
		uploadID).Scan(&userID, &fileSize, &chunkSize, &totalChunks, &status)
	if err == sql.ErrNoRows {
		m.sendError(w, http.StatusNotFound, "upload not found")
		return
	}
	if err != nil {
		m.sendError(w, http.StatusInternalServerError, "failed to look up upload")
		return
	}
	if userID != claims.UserID && !claims.IsAdmin {
		m.sendError(w, http.StatusForbidden, "access denied")
		return
	}
	if status != "active" {
		m.sendError(w, http.StatusConflict, "upload is not active")
		return
	}
	if chunkIndex >= totalChunks {
		m.sendError(w, http.StatusBadRequest, fmt.Sprintf("chunk index %d >= total chunks %d", chunkIndex, totalChunks))
		return
	}

	// Calculate write offset and expected size
	offset := int64(chunkIndex) * int64(chunkSize)
	expectedSize := int64(chunkSize)
	if chunkIndex == totalChunks-1 {
		// Last chunk may be smaller
		expectedSize = fileSize - offset
	}

	// Open temp file and write at offset
	f, err := os.OpenFile(m.tempPath(uploadID), os.O_WRONLY, 0o644)
	if err != nil {
		m.sendError(w, http.StatusInternalServerError, "temp file not accessible")
		return
	}
	defer f.Close()

	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		m.sendError(w, http.StatusInternalServerError, "failed to seek in temp file")
		return
	}

	// Read chunk data from request body (limited to expected size + 1 to detect over-size)
	n, err := io.Copy(f, io.LimitReader(r.Body, expectedSize+1))
	if err != nil {
		m.sendError(w, http.StatusInternalServerError, "failed to write chunk")
		return
	}
	if n > expectedSize {
		m.sendError(w, http.StatusBadRequest, "chunk data exceeds expected size")
		return
	}

	// Record chunk in DB (upsert)
	_, err = m.db.ExecContext(r.Context(),
		`INSERT INTO upload_chunks (upload_id, chunk_index, size, received_at)
		 VALUES ($1, $2, $3, NOW())
		 ON CONFLICT (upload_id, chunk_index) DO UPDATE SET size = $3, received_at = NOW()`,
		uploadID, chunkIndex, int(n))
	if err != nil {
		m.sendError(w, http.StatusInternalServerError, "failed to record chunk")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}

// ─── Complete Upload ────────────────────────────────────────────────────────

func (m *ChunkedUploadManager) handleCompleteUpload(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		m.sendError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	uploadID := r.PathValue("uploadId")

	// Load upload record
	var userID int
	var path, fileName string
	var fileSize int64
	var totalChunks int
	var status string
	err := m.db.QueryRowContext(r.Context(),
		`SELECT user_id, path, file_name, file_size, total_chunks, status FROM chunked_uploads WHERE id = $1`,
		uploadID).Scan(&userID, &path, &fileName, &fileSize, &totalChunks, &status)
	if err == sql.ErrNoRows {
		m.sendError(w, http.StatusNotFound, "upload not found")
		return
	}
	if err != nil {
		m.sendError(w, http.StatusInternalServerError, "failed to look up upload")
		return
	}
	if userID != claims.UserID && !claims.IsAdmin {
		m.sendError(w, http.StatusForbidden, "access denied")
		return
	}
	if status != "active" {
		m.sendError(w, http.StatusConflict, "upload is not active (status: "+status+")")
		return
	}

	// Verify all chunks received
	var receivedCount int
	err = m.db.QueryRowContext(r.Context(),
		`SELECT COUNT(*) FROM upload_chunks WHERE upload_id = $1`, uploadID).Scan(&receivedCount)
	if err != nil {
		m.sendError(w, http.StatusInternalServerError, "failed to count chunks")
		return
	}
	if receivedCount != totalChunks {
		m.sendError(w, http.StatusBadRequest,
			fmt.Sprintf("incomplete upload: received %d/%d chunks", receivedCount, totalChunks))
		return
	}

	// Compute SHA-256 hash of the assembled file (streaming)
	tmpFile := m.tempPath(uploadID)
	f, err := os.Open(tmpFile)
	if err != nil {
		m.sendError(w, http.StatusInternalServerError, "failed to open assembled file")
		return
	}

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		f.Close()
		m.sendError(w, http.StatusInternalServerError, "failed to compute hash")
		return
	}
	hashStr := fmt.Sprintf("%x", hasher.Sum(nil))

	// Seek back to beginning for upload to backend
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		f.Close()
		m.sendError(w, http.StatusInternalServerError, "failed to seek temp file")
		return
	}

	// S3 key
	s3Key := strings.TrimPrefix(path, "/")

	// Check existing file for versioning
	newVersion := 1
	existingRow, _ := m.server.metadata.GetFileRow(r.Context(), path)

	if existingRow != nil && !existingRow.IsDir && existingRow.Size > 0 {
		// Save current version
		if err := m.server.metadata.SaveVersion(r.Context(), path); err != nil {
			logging.Warn("failed to save version", zap.String("path", path), zap.Error(err))
		}

		existBackend, _, _ := m.server.storageRouter.ResolveForFile(r.Context(), existingRow.StorageLocID, existingRow.GroupID)
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
	backend, loc, err := m.server.storageRouter.ResolveForUpload(r.Context(), path, groupID)
	if err != nil {
		f.Close()
		m.sendError(w, http.StatusInternalServerError, "no storage backend: "+err.Error())
		return
	}

	// Stream temp file to backend
	if err := backend.PutObject(r.Context(), s3Key, f, fileSize); err != nil {
		f.Close()
		m.sendError(w, http.StatusInternalServerError, "failed to upload to storage: "+err.Error())
		return
	}
	f.Close()

	// Ensure parent directories exist
	if err := m.server.ensureParentDirs(r.Context(), path); err != nil {
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
		Name:         fileName,
		Path:         path,
		ParentPath:   parentPath,
		Size:         fileSize,
		ModTime:      time.Now(),
		IsDir:        false,
		Hash:         hashStr,
		S3Key:        s3Key,
		Version:      newVersion,
		StorageLocID: storageLocID,
	}

	if claims != nil && existingRow == nil {
		ownerID := claims.UserID
		fileRow.OwnerID = &ownerID
	}

	if err := m.server.metadata.UpsertFile(r.Context(), fileRow); err != nil {
		m.sendError(w, http.StatusInternalServerError, "failed to save metadata: "+err.Error())
		return
	}

	// Refresh tree
	m.server.RefreshTree(r.Context())

	// Track bandwidth
	m.server.quotaStore.TrackBandwidth(r.Context(), claims.UserID, fileSize, 0)

	logging.Info("chunked upload completed",
		zap.String("path", path),
		zap.Int64("size", fileSize),
		zap.String("hash", hashStr[:16]),
		zap.Int("version", newVersion))

	// SSE event
	eventType := events.EventCreate
	if existingRow != nil {
		eventType = events.EventModify
	}
	m.server.publishEvent(eventType, path, newVersion, hashStr, fileSize, claims.UserID, claims.Username)

	// Gallery processing
	if m.server.processor != nil && gallery.IsImageFile(path) {
		m.server.processor.Enqueue(path)
	}

	// Mark upload as completed and clean up
	m.db.ExecContext(r.Context(),
		`UPDATE chunked_uploads SET status = 'completed' WHERE id = $1`, uploadID)
	m.db.ExecContext(r.Context(),
		`DELETE FROM upload_chunks WHERE upload_id = $1`, uploadID)
	os.Remove(tmpFile)

	metrics.RecordContentUpload(fileSize, true)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"path":    path,
		"size":    fileSize,
		"hash":    hashStr,
		"version": newVersion,
	})
}

// ─── Upload Status ──────────────────────────────────────────────────────────

func (m *ChunkedUploadManager) handleUploadStatus(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		m.sendError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	uploadID := r.PathValue("uploadId")

	var userID int
	var totalChunks int
	var status string
	err := m.db.QueryRowContext(r.Context(),
		`SELECT user_id, total_chunks, status FROM chunked_uploads WHERE id = $1`,
		uploadID).Scan(&userID, &totalChunks, &status)
	if err == sql.ErrNoRows {
		m.sendError(w, http.StatusNotFound, "upload not found")
		return
	}
	if err != nil {
		m.sendError(w, http.StatusInternalServerError, "failed to look up upload")
		return
	}
	if userID != claims.UserID && !claims.IsAdmin {
		m.sendError(w, http.StatusForbidden, "access denied")
		return
	}

	// Get received chunk indices
	rows, err := m.db.QueryContext(r.Context(),
		`SELECT chunk_index FROM upload_chunks WHERE upload_id = $1 ORDER BY chunk_index`, uploadID)
	if err != nil {
		m.sendError(w, http.StatusInternalServerError, "failed to query chunks")
		return
	}
	defer rows.Close()

	var received []int
	for rows.Next() {
		var idx int
		if err := rows.Scan(&idx); err != nil {
			continue
		}
		received = append(received, idx)
	}
	if received == nil {
		received = []int{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"totalChunks": totalChunks,
		"received":    received,
		"status":      status,
	})
}

// ─── Abort Upload ───────────────────────────────────────────────────────────

func (m *ChunkedUploadManager) handleAbortUpload(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		m.sendError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	uploadID := r.PathValue("uploadId")

	var userID int
	err := m.db.QueryRowContext(r.Context(),
		`SELECT user_id FROM chunked_uploads WHERE id = $1`, uploadID).Scan(&userID)
	if err == sql.ErrNoRows {
		m.sendError(w, http.StatusNotFound, "upload not found")
		return
	}
	if err != nil {
		m.sendError(w, http.StatusInternalServerError, "failed to look up upload")
		return
	}
	if userID != claims.UserID && !claims.IsAdmin {
		m.sendError(w, http.StatusForbidden, "access denied")
		return
	}

	// Clean up
	m.db.ExecContext(r.Context(), `DELETE FROM upload_chunks WHERE upload_id = $1`, uploadID)
	m.db.ExecContext(r.Context(), `DELETE FROM chunked_uploads WHERE id = $1`, uploadID)
	os.Remove(m.tempPath(uploadID))

	logging.Info("chunked upload aborted", zap.String("upload_id", uploadID))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"aborted": true})
}

// ─── Cleanup ────────────────────────────────────────────────────────────────

func (m *ChunkedUploadManager) cleanupExpired(ctx context.Context) {
	rows, err := m.db.QueryContext(ctx,
		`SELECT id FROM chunked_uploads WHERE status = 'active' AND expires_at < NOW()`)
	if err != nil {
		logging.Warn("chunked upload cleanup query failed", zap.Error(err))
		return
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		ids = append(ids, id)
	}

	for _, id := range ids {
		m.db.ExecContext(ctx, `DELETE FROM upload_chunks WHERE upload_id = $1`, id)
		m.db.ExecContext(ctx, `DELETE FROM chunked_uploads WHERE id = $1`, id)
		os.Remove(m.tempPath(id))
		logging.Info("cleaned up expired chunked upload", zap.String("upload_id", id))
	}
}

// ─── Helpers ────────────────────────────────────────────────────────────────

func (m *ChunkedUploadManager) sendError(w http.ResponseWriter, code int, message string) {
	m.server.sendError(w, code, message)
}

func generateUploadID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
