package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/fruitsalade/fruitsalade/fruitsalade/internal/auth"
	"github.com/fruitsalade/fruitsalade/fruitsalade/internal/gallery"
	"github.com/fruitsalade/fruitsalade/fruitsalade/internal/logging"
	"github.com/fruitsalade/fruitsalade/shared/pkg/protocol"
)

// ─── Gallery Search ─────────────────────────────────────────────────────────

func (s *Server) handleGallerySearch(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		s.sendError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	q := r.URL.Query()

	// Parse search params
	params := &gallery.SearchParams{
		Query:       q.Get("query"),
		CameraMake:  q.Get("camera_make"),
		CameraModel: q.Get("camera_model"),
		Country:     q.Get("country"),
		City:        q.Get("city"),
		SortBy:      q.Get("sort_by"),
		SortOrder:   q.Get("sort_order"),
		UserID:      claims.UserID,
		IsAdmin:     claims.IsAdmin,
	}

	if limit, err := strconv.Atoi(q.Get("limit")); err == nil {
		params.Limit = limit
	} else {
		params.Limit = 50
	}
	if offset, err := strconv.Atoi(q.Get("offset")); err == nil {
		params.Offset = offset
	}

	if df := q.Get("date_from"); df != "" {
		if t, err := time.Parse("2006-01-02", df); err == nil {
			params.DateFrom = &t
		}
	}
	if dt := q.Get("date_to"); dt != "" {
		if t, err := time.Parse("2006-01-02", dt); err == nil {
			params.DateTo = &t
		}
	}

	if tags := q.Get("tags"); tags != "" {
		params.Tags = strings.Split(tags, ",")
	}

	// Load user groups for permission filtering
	if !claims.IsAdmin {
		userGroups, _ := s.groups.GetUserGroupsMap(r.Context(), claims.UserID)
		for gid := range userGroups {
			params.UserGroupIDs = append(params.UserGroupIDs, gid)
		}
		userPerms, _ := s.permissions.GetUserPermissionsMap(r.Context(), claims.UserID)
		for path := range userPerms {
			params.UserPermPaths = append(params.UserPermPaths, path)
		}
	}

	results, total, err := s.galleryStore.Search(r.Context(), params)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "search failed: "+err.Error())
		return
	}

	// Batch-load tags for results
	var paths []string
	for _, r := range results {
		paths = append(paths, r.FilePath)
	}
	tagMap, _ := s.galleryStore.GetTagsForFiles(r.Context(), paths)

	// Build response
	resp := protocol.GallerySearchResponse{
		Total:   total,
		Offset:  params.Offset,
		Limit:   params.Limit,
		HasMore: params.Offset+len(results) < total,
	}

	for _, r := range results {
		item := protocol.GalleryItem{
			FilePath:     r.FilePath,
			FileName:     r.FileName,
			Size:         r.Size,
			ModTime:      r.ModTime,
			Hash:         r.Hash,
			Width:        r.Width,
			Height:       r.Height,
			CameraMake:   r.CameraMake,
			CameraModel:  r.CameraModel,
			DateTaken:    r.DateTaken,
			Latitude:     r.Latitude,
			Longitude:    r.Longitude,
			LocationCity: r.LocationCity,
			Country:      r.LocationCountry,
			HasThumbnail: r.HasThumbnail,
			Tags:         tagMap[r.FilePath],
		}
		resp.Items = append(resp.Items, item)
	}

	if resp.Items == nil {
		resp.Items = []protocol.GalleryItem{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ─── Gallery Thumbnail ──────────────────────────────────────────────────────

func (s *Server) handleGalleryThumb(w http.ResponseWriter, r *http.Request) {
	pathParam := r.PathValue("path")
	if pathParam == "" {
		s.sendError(w, http.StatusBadRequest, "path required")
		return
	}

	filePath := "/" + pathParam

	// Check read permission
	claims := auth.GetClaims(r.Context())
	if claims != nil && !s.permissions.CheckAccess(r.Context(), claims.UserID, filePath, "read", claims.IsAdmin) {
		s.sendError(w, http.StatusForbidden, "access denied")
		return
	}

	thumbKey := s.galleryStore.GetThumbKey(r.Context(), filePath)
	if thumbKey == "" {
		s.sendError(w, http.StatusNotFound, "no thumbnail")
		return
	}

	backend, _, err := s.storageRouter.GetDefault()
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "no storage backend")
		return
	}

	reader, size, err := backend.GetObject(r.Context(), thumbKey, 0, 0)
	if err != nil {
		s.sendError(w, http.StatusNotFound, "thumbnail not found")
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	w.Header().Set("Cache-Control", "public, max-age=86400")
	io.Copy(w, reader)
}

// ─── Gallery Metadata ───────────────────────────────────────────────────────

func (s *Server) handleGalleryMetadata(w http.ResponseWriter, r *http.Request) {
	pathParam := r.PathValue("path")
	if pathParam == "" {
		s.sendError(w, http.StatusBadRequest, "path required")
		return
	}

	filePath := "/" + pathParam

	claims := auth.GetClaims(r.Context())
	if claims != nil && !s.permissions.CheckAccess(r.Context(), claims.UserID, filePath, "read", claims.IsAdmin) {
		s.sendError(w, http.StatusForbidden, "access denied")
		return
	}

	meta, err := s.galleryStore.GetMetadata(r.Context(), filePath)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to get metadata: "+err.Error())
		return
	}
	if meta == nil {
		s.sendError(w, http.StatusNotFound, "no gallery metadata for this file")
		return
	}

	tags, _ := s.galleryStore.GetTagsForFile(r.Context(), filePath)

	resp := protocol.GalleryMetadataResponse{
		FilePath:        meta.FilePath,
		FileName:        strings.TrimPrefix(meta.FilePath, "/"),
		Width:           meta.Width,
		Height:          meta.Height,
		CameraMake:      meta.CameraMake,
		CameraModel:     meta.CameraModel,
		LensModel:       meta.LensModel,
		FocalLength:     meta.FocalLength,
		Aperture:        meta.Aperture,
		ShutterSpeed:    meta.ShutterSpeed,
		ISO:             meta.ISO,
		Flash:           meta.Flash,
		DateTaken:       meta.DateTaken,
		Latitude:        meta.Latitude,
		Longitude:       meta.Longitude,
		Altitude:        meta.Altitude,
		LocationCountry: meta.LocationCountry,
		LocationCity:    meta.LocationCity,
		LocationName:    meta.LocationName,
		Orientation:     meta.Orientation,
		HasThumbnail:    meta.HasThumbnail,
		Status:          meta.Status,
	}

	// Get file size from the tree
	node := s.findNode(s.tree, filePath)
	if node != nil {
		resp.Size = node.Size
		resp.FileName = node.Name
	}

	for _, t := range tags {
		resp.Tags = append(resp.Tags, protocol.TagInfo{
			Tag:        t.Tag,
			Confidence: t.Confidence,
			Source:     t.Source,
		})
	}
	if resp.Tags == nil {
		resp.Tags = []protocol.TagInfo{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ─── Albums ─────────────────────────────────────────────────────────────────

func (s *Server) handleAlbumsByDate(w http.ResponseWriter, r *http.Request) {
	rows, err := s.galleryStore.GetAlbumsByDate(r.Context())
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to get date albums: "+err.Error())
		return
	}

	// Group by year
	albumMap := make(map[int][]protocol.MonthCount)
	var years []int
	seen := make(map[int]bool)
	for _, r := range rows {
		if !seen[r.Year] {
			years = append(years, r.Year)
			seen[r.Year] = true
		}
		albumMap[r.Year] = append(albumMap[r.Year], protocol.MonthCount{
			Month: r.Month,
			Count: r.Count,
		})
	}

	var albums []protocol.DateAlbum
	for _, y := range years {
		albums = append(albums, protocol.DateAlbum{
			Year:   y,
			Months: albumMap[y],
		})
	}
	if albums == nil {
		albums = []protocol.DateAlbum{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(albums)
}

func (s *Server) handleAlbumsByLocation(w http.ResponseWriter, r *http.Request) {
	rows, err := s.galleryStore.GetAlbumsByLocation(r.Context())
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to get location albums: "+err.Error())
		return
	}

	// Group by country
	albumMap := make(map[string][]protocol.CityCount)
	var countries []string
	seen := make(map[string]bool)
	for _, r := range rows {
		if !seen[r.Country] {
			countries = append(countries, r.Country)
			seen[r.Country] = true
		}
		albumMap[r.Country] = append(albumMap[r.Country], protocol.CityCount{
			City:  r.City,
			Count: r.Count,
		})
	}

	var albums []protocol.LocationAlbum
	for _, c := range countries {
		albums = append(albums, protocol.LocationAlbum{
			Country: c,
			Cities:  albumMap[c],
		})
	}
	if albums == nil {
		albums = []protocol.LocationAlbum{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(albums)
}

func (s *Server) handleAlbumsByCamera(w http.ResponseWriter, r *http.Request) {
	rows, err := s.galleryStore.GetAlbumsByCamera(r.Context())
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to get camera albums: "+err.Error())
		return
	}

	// Group by make
	albumMap := make(map[string][]protocol.ModelCount)
	var makes []string
	seen := make(map[string]bool)
	for _, r := range rows {
		if !seen[r.Make] {
			makes = append(makes, r.Make)
			seen[r.Make] = true
		}
		albumMap[r.Make] = append(albumMap[r.Make], protocol.ModelCount{
			Model: r.Model,
			Count: r.Count,
		})
	}

	var albums []protocol.CameraAlbum
	for _, m := range makes {
		albums = append(albums, protocol.CameraAlbum{
			Make:   m,
			Models: albumMap[m],
		})
	}
	if albums == nil {
		albums = []protocol.CameraAlbum{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(albums)
}

// ─── Tags ───────────────────────────────────────────────────────────────────

func (s *Server) handleAddTag(w http.ResponseWriter, r *http.Request) {
	pathParam := r.PathValue("path")
	if pathParam == "" {
		s.sendError(w, http.StatusBadRequest, "path required")
		return
	}
	filePath := "/" + pathParam

	claims := auth.GetClaims(r.Context())
	if claims != nil && !s.permissions.CheckAccess(r.Context(), claims.UserID, filePath, "write", claims.IsAdmin) {
		s.sendError(w, http.StatusForbidden, "write access denied")
		return
	}

	var req protocol.TagRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Tag == "" {
		s.sendError(w, http.StatusBadRequest, "tag is required")
		return
	}

	if err := s.galleryStore.AddTag(r.Context(), filePath, req.Tag, "manual", 1.0); err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to add tag: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"file_path": filePath,
		"tag":       req.Tag,
	})
}

func (s *Server) handleRemoveTag(w http.ResponseWriter, r *http.Request) {
	pathParam := r.PathValue("path")
	if pathParam == "" {
		s.sendError(w, http.StatusBadRequest, "path required")
		return
	}
	filePath := "/" + pathParam

	claims := auth.GetClaims(r.Context())
	if claims != nil && !s.permissions.CheckAccess(r.Context(), claims.UserID, filePath, "write", claims.IsAdmin) {
		s.sendError(w, http.StatusForbidden, "write access denied")
		return
	}

	tag := r.URL.Query().Get("tag")
	if tag == "" {
		s.sendError(w, http.StatusBadRequest, "tag query parameter required")
		return
	}

	if err := s.galleryStore.RemoveTag(r.Context(), filePath, tag); err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to remove tag: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"file_path": filePath,
		"tag":       tag,
		"removed":   true,
	})
}

func (s *Server) handleListTags(w http.ResponseWriter, r *http.Request) {
	tags, err := s.galleryStore.ListAllTags(r.Context())
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to list tags: "+err.Error())
		return
	}

	resp := make([]protocol.TagCountResponse, 0, len(tags))
	for _, t := range tags {
		resp = append(resp, protocol.TagCountResponse{
			Tag:   t.Tag,
			Count: t.Count,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ─── Stats ──────────────────────────────────────────────────────────────────

func (s *Server) handleGalleryStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.galleryStore.GetStats(r.Context())
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to get stats: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(protocol.GalleryStatsResponse{
		TotalImages: stats.TotalImages,
		WithGPS:     stats.WithGPS,
		WithTags:    stats.WithTags,
		Processed:   stats.Processed,
		Pending:     stats.Pending,
	})
}

// ─── Admin: Plugins ─────────────────────────────────────────────────────────

func (s *Server) handleListPlugins(w http.ResponseWriter, r *http.Request) {
	if s.requireAdmin(w, r) == nil {
		return
	}

	plugins, err := s.galleryStore.ListPlugins(r.Context())
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to list plugins: "+err.Error())
		return
	}

	resp := make([]protocol.PluginResponse, 0, len(plugins))
	for _, p := range plugins {
		resp = append(resp, pluginToResponse(p))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleCreatePlugin(w http.ResponseWriter, r *http.Request) {
	if s.requireAdmin(w, r) == nil {
		return
	}

	var req protocol.PluginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" || req.WebhookURL == "" {
		s.sendError(w, http.StatusBadRequest, "name and webhook_url are required")
		return
	}

	plugin := &gallery.Plugin{
		Name:       req.Name,
		WebhookURL: req.WebhookURL,
		Enabled:    req.Enabled,
		Config:     req.Config,
	}

	created, err := s.galleryStore.CreatePlugin(r.Context(), plugin)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to create plugin: "+err.Error())
		return
	}

	logging.Info("gallery plugin created", zap.String("name", created.Name))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(pluginToResponse(*created))
}

func (s *Server) handleUpdatePlugin(w http.ResponseWriter, r *http.Request) {
	if s.requireAdmin(w, r) == nil {
		return
	}

	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid plugin ID")
		return
	}

	existing, err := s.galleryStore.GetPlugin(r.Context(), id)
	if err != nil || existing == nil {
		s.sendError(w, http.StatusNotFound, "plugin not found")
		return
	}

	var req protocol.PluginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	existing.Name = req.Name
	existing.WebhookURL = req.WebhookURL
	existing.Enabled = req.Enabled
	if req.Config != nil {
		existing.Config = req.Config
	}

	if err := s.galleryStore.UpdatePlugin(r.Context(), existing); err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to update plugin: "+err.Error())
		return
	}

	logging.Info("gallery plugin updated", zap.String("name", existing.Name))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pluginToResponse(*existing))
}

func (s *Server) handleDeletePlugin(w http.ResponseWriter, r *http.Request) {
	if s.requireAdmin(w, r) == nil {
		return
	}

	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid plugin ID")
		return
	}

	if err := s.galleryStore.DeletePlugin(r.Context(), id); err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to delete plugin: "+err.Error())
		return
	}

	logging.Info("gallery plugin deleted", zap.Int("id", id))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":      id,
		"deleted": true,
	})
}

func (s *Server) handleTestPlugin(w http.ResponseWriter, r *http.Request) {
	if s.requireAdmin(w, r) == nil {
		return
	}

	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid plugin ID")
		return
	}

	plugin, err := s.galleryStore.GetPlugin(r.Context(), id)
	if err != nil || plugin == nil {
		s.sendError(w, http.StatusNotFound, "plugin not found")
		return
	}

	resp, err := s.pluginCaller.TestPlugin(r.Context(), *plugin)
	if err != nil {
		s.galleryStore.UpdatePluginHealth(r.Context(), id, err.Error())
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	s.galleryStore.UpdatePluginHealth(r.Context(), id, "")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"tags":    resp.Tags,
	})
}

func (s *Server) handleReprocessGallery(w http.ResponseWriter, r *http.Request) {
	if s.requireAdmin(w, r) == nil {
		return
	}

	// Reset all to pending and re-enqueue
	db := s.auth.DB()
	_, err := db.ExecContext(r.Context(),
		`UPDATE image_metadata SET status = 'pending', updated_at = NOW()`)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to reset: "+err.Error())
		return
	}

	go s.processor.ProcessExisting(context.Background())

	logging.Info("gallery: reprocess triggered by admin")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "reprocessing started",
	})
}

func pluginToResponse(p gallery.Plugin) protocol.PluginResponse {
	return protocol.PluginResponse{
		ID:         p.ID,
		Name:       p.Name,
		WebhookURL: p.WebhookURL,
		Enabled:    p.Enabled,
		Config:     p.Config,
		LastHealth: p.LastHealth,
		LastError:  p.LastError,
		CreatedAt:  p.CreatedAt,
		UpdatedAt:  p.UpdatedAt,
	}
}
