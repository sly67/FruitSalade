package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"go.uber.org/zap"

	"github.com/fruitsalade/fruitsalade/fruitsalade/internal/auth"
	"github.com/fruitsalade/fruitsalade/fruitsalade/internal/logging"
	"github.com/fruitsalade/fruitsalade/shared/pkg/protocol"
)

// ─── Trash Handlers ─────────────────────────────────────────────────────────

func (s *Server) handleTrashList(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		s.sendError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var userID *int
	if !claims.IsAdmin {
		userID = &claims.UserID
	}

	items, err := s.metadata.ListTrash(r.Context(), userID)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to list trash: "+err.Error())
		return
	}

	var resp []protocol.TrashItem
	for _, t := range items {
		resp = append(resp, protocol.TrashItem{
			ID:            t.ID,
			Name:          t.Name,
			OriginalPath:  t.OriginalPath,
			Size:          t.Size,
			IsDir:         t.IsDir,
			DeletedAt:     t.DeletedAt,
			DeletedByName: t.DeletedByName,
		})
	}
	if resp == nil {
		resp = []protocol.TrashItem{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleTrashRestore(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		s.sendError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req protocol.TrashRestoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Path == "" {
		s.sendError(w, http.StatusBadRequest, "path required")
		return
	}

	if err := s.metadata.RestoreFile(r.Context(), req.Path); err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to restore: "+err.Error())
		return
	}

	s.RefreshTree(r.Context())

	logging.Info("file restored from trash", zap.String("path", req.Path))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"path":     req.Path,
		"restored": true,
	})
}

func (s *Server) handleTrashPurge(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		s.sendError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	path := "/" + r.PathValue("path")
	if path == "/" {
		s.sendError(w, http.StatusBadRequest, "path required")
		return
	}

	purged, err := s.metadata.PurgeFile(r.Context(), path)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to purge: "+err.Error())
		return
	}

	// Delete from storage
	for _, p := range purged {
		if p.S3Key == "" {
			continue
		}
		backend, _, err := s.storageRouter.ResolveForFile(r.Context(), p.StorageLocID, p.GroupID)
		if err == nil && backend != nil {
			backend.DeleteObject(r.Context(), p.S3Key)
		}
	}

	s.RefreshTree(r.Context())

	logging.Info("file purged from trash", zap.String("path", path), zap.Int("count", len(purged)))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"path":   path,
		"purged": len(purged),
	})
}

func (s *Server) handleTrashEmpty(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil || !claims.IsAdmin {
		s.sendError(w, http.StatusForbidden, "admin access required")
		return
	}

	purged, err := s.metadata.PurgeAllTrash(r.Context())
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to empty trash: "+err.Error())
		return
	}

	// Delete from storage
	for _, p := range purged {
		if p.S3Key == "" {
			continue
		}
		backend, _, err := s.storageRouter.ResolveForFile(r.Context(), p.StorageLocID, p.GroupID)
		if err == nil && backend != nil {
			backend.DeleteObject(r.Context(), p.S3Key)
		}
	}

	s.RefreshTree(r.Context())

	logging.Info("trash emptied", zap.Int("count", len(purged)))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"purged": len(purged),
	})
}

// ─── Favorites Handlers ─────────────────────────────────────────────────────

func (s *Server) handleAddFavorite(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		s.sendError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	path := "/" + r.PathValue("path")
	if path == "/" {
		s.sendError(w, http.StatusBadRequest, "path required")
		return
	}

	if err := s.metadata.AddFavorite(r.Context(), claims.UserID, path); err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to add favorite: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"path":    path,
		"starred": true,
	})
}

func (s *Server) handleRemoveFavorite(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		s.sendError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	path := "/" + r.PathValue("path")
	if path == "/" {
		s.sendError(w, http.StatusBadRequest, "path required")
		return
	}

	if err := s.metadata.RemoveFavorite(r.Context(), claims.UserID, path); err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to remove favorite: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"path":    path,
		"starred": false,
	})
}

func (s *Server) handleListFavorites(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		s.sendError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	items, err := s.metadata.ListFavorites(r.Context(), claims.UserID)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to list favorites: "+err.Error())
		return
	}

	var resp []protocol.FavoriteItem
	for _, f := range items {
		resp = append(resp, protocol.FavoriteItem{
			FilePath: f.FilePath,
			FileName: f.FileName,
			Size:     f.Size,
			IsDir:    f.IsDir,
			ModTime:  f.ModTime,
		})
	}
	if resp == nil {
		resp = []protocol.FavoriteItem{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleListFavoritePaths(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		s.sendError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	paths, err := s.metadata.ListFavoritePaths(r.Context(), claims.UserID)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to list favorite paths: "+err.Error())
		return
	}
	if paths == nil {
		paths = []string{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(paths)
}

// ─── Search Handler ─────────────────────────────────────────────────────────

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		s.sendError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		s.sendError(w, http.StatusBadRequest, "query parameter 'q' required")
		return
	}

	typeFilter := r.URL.Query().Get("type") // "all", "files", "dirs", "images"

	results, err := s.metadata.SearchFiles(r.Context(), query, typeFilter, 200)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "search failed: "+err.Error())
		return
	}

	var resp []protocol.SearchResult
	for _, r := range results {
		resp = append(resp, protocol.SearchResult{
			ID:      r.ID,
			Name:    r.Name,
			Path:    r.Path,
			Size:    r.Size,
			IsDir:   r.IsDir,
			ModTime: r.ModTime,
		})
	}
	if resp == nil {
		resp = []protocol.SearchResult{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ─── Bulk Operation Handlers ────────────────────────────────────────────────

func (s *Server) handleBulkMove(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		s.sendError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req protocol.BulkMoveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.Paths) == 0 || req.Destination == "" {
		s.sendError(w, http.StatusBadRequest, "paths and destination required")
		return
	}

	// Ensure destination directory exists
	if err := s.ensureParentDirs(r.Context(), req.Destination+"/placeholder"); err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to create destination: "+err.Error())
		return
	}

	resp := protocol.BulkResponse{}
	for _, path := range req.Paths {
		// Check permission
		if !claims.IsAdmin && !s.permissions.CheckAccess(r.Context(), claims.UserID, path, "write", false) {
			resp.Failed++
			resp.Errors = append(resp.Errors, "access denied: "+path)
			continue
		}

		baseName := path[strings.LastIndex(path, "/")+1:]
		newPath := strings.TrimSuffix(req.Destination, "/") + "/" + baseName

		if err := s.metadata.MoveFile(r.Context(), path, newPath); err != nil {
			resp.Failed++
			resp.Errors = append(resp.Errors, path+": "+err.Error())
		} else {
			// Move storage object
			oldKey := strings.TrimPrefix(path, "/")
			newKey := strings.TrimPrefix(newPath, "/")
			fileRow, _ := s.metadata.GetFileRow(r.Context(), newPath)
			if fileRow != nil && !fileRow.IsDir {
				backend, _, _ := s.storageRouter.ResolveForFile(r.Context(), fileRow.StorageLocID, fileRow.GroupID)
				if backend != nil {
					backend.CopyObject(r.Context(), oldKey, newKey)
					backend.DeleteObject(r.Context(), oldKey)
				}
			}
			resp.Succeeded++
		}
	}

	s.RefreshTree(r.Context())

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleBulkCopy(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		s.sendError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req protocol.BulkCopyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.Paths) == 0 || req.Destination == "" {
		s.sendError(w, http.StatusBadRequest, "paths and destination required")
		return
	}

	// Ensure destination directory exists
	if err := s.ensureParentDirs(r.Context(), req.Destination+"/placeholder"); err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to create destination: "+err.Error())
		return
	}

	resp := protocol.BulkResponse{}
	for _, path := range req.Paths {
		// Check permission
		if !claims.IsAdmin && !s.permissions.CheckAccess(r.Context(), claims.UserID, path, "read", false) {
			resp.Failed++
			resp.Errors = append(resp.Errors, "access denied: "+path)
			continue
		}

		baseName := path[strings.LastIndex(path, "/")+1:]
		newPath := strings.TrimSuffix(req.Destination, "/") + "/" + baseName

		// Copy metadata
		if err := s.metadata.CopyFileRow(r.Context(), path, newPath); err != nil {
			resp.Failed++
			resp.Errors = append(resp.Errors, path+": "+err.Error())
			continue
		}

		// Copy storage object
		srcRow, _ := s.metadata.GetFileRow(r.Context(), path)
		if srcRow != nil && !srcRow.IsDir {
			srcKey := strings.TrimPrefix(path, "/")
			dstKey := strings.TrimPrefix(newPath, "/")
			backend, _, _ := s.storageRouter.ResolveForFile(r.Context(), srcRow.StorageLocID, srcRow.GroupID)
			if backend != nil {
				if err := backend.CopyObject(r.Context(), srcKey, dstKey); err != nil {
					logging.Warn("bulk copy storage failed", zap.String("src", srcKey), zap.Error(err))
				}
			}
		}
		resp.Succeeded++
	}

	s.RefreshTree(r.Context())

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleBulkShare(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		s.sendError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req protocol.BulkShareRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.Paths) == 0 {
		s.sendError(w, http.StatusBadRequest, "paths required")
		return
	}

	resp := protocol.BulkResponse{}
	for _, path := range req.Paths {
		if !claims.IsAdmin && !s.permissions.CheckAccess(r.Context(), claims.UserID, path, "read", false) {
			resp.Failed++
			resp.Errors = append(resp.Errors, "access denied: "+path)
			continue
		}

		_, err := s.shareLinks.Create(r.Context(), path, claims.UserID, req.Password, req.ExpiresInSec, req.MaxDownloads)
		if err != nil {
			resp.Failed++
			resp.Errors = append(resp.Errors, path+": "+err.Error())
		} else {
			resp.Succeeded++
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleBulkAlbumAdd(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		s.sendError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req protocol.BulkAlbumAddRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.Paths) == 0 || req.AlbumID == 0 {
		s.sendError(w, http.StatusBadRequest, "paths and album_id required")
		return
	}

	if s.galleryStore == nil {
		s.sendError(w, http.StatusNotImplemented, "gallery not enabled")
		return
	}

	// Verify album ownership
	album, err := s.galleryStore.GetAlbum(r.Context(), req.AlbumID)
	if err != nil {
		s.sendError(w, http.StatusNotFound, "album not found")
		return
	}
	if album.UserID != claims.UserID && !claims.IsAdmin {
		s.sendError(w, http.StatusForbidden, "album not owned by user")
		return
	}

	resp := protocol.BulkResponse{}
	for _, path := range req.Paths {
		if err := s.galleryStore.AddImageToAlbum(r.Context(), req.AlbumID, path); err != nil {
			resp.Failed++
			resp.Errors = append(resp.Errors, path+": "+err.Error())
		} else {
			resp.Succeeded++
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleBulkTag(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		s.sendError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req protocol.BulkTagRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.Paths) == 0 || len(req.Tags) == 0 {
		s.sendError(w, http.StatusBadRequest, "paths and tags required")
		return
	}

	if s.galleryStore == nil {
		s.sendError(w, http.StatusNotImplemented, "gallery not enabled")
		return
	}

	resp := protocol.BulkResponse{}
	for _, path := range req.Paths {
		for _, tag := range req.Tags {
			if err := s.galleryStore.AddTag(r.Context(), path, tag, "manual", 1.0); err != nil {
				resp.Failed++
				resp.Errors = append(resp.Errors, path+": "+err.Error())
			} else {
				resp.Succeeded++
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
