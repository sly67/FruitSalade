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

// galleryPermFilter builds a PermFilter from the authenticated user's claims.
// Returns nil for admins (no filtering needed).
func (s *Server) galleryPermFilter(ctx context.Context, claims *auth.Claims) *gallery.PermFilter {
	if claims.IsAdmin {
		return nil
	}
	userGroups, _ := s.groups.GetUserGroupsMap(ctx, claims.UserID)
	var groupIDs []int
	for gid := range userGroups {
		groupIDs = append(groupIDs, gid)
	}
	userPerms, _ := s.permissions.GetUserPermissionsMap(ctx, claims.UserID)
	var permPaths []string
	for path := range userPerms {
		permPaths = append(permPaths, path)
	}
	return gallery.BuildPermFilter(1, claims.UserID, groupIDs, permPaths, false)
}

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
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		s.sendError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	pf := s.galleryPermFilter(r.Context(), claims)

	rows, err := s.galleryStore.GetAlbumsByDate(r.Context(), pf)
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
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		s.sendError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	pf := s.galleryPermFilter(r.Context(), claims)

	rows, err := s.galleryStore.GetAlbumsByLocation(r.Context(), pf)
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
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		s.sendError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	pf := s.galleryPermFilter(r.Context(), claims)

	rows, err := s.galleryStore.GetAlbumsByCamera(r.Context(), pf)
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
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		s.sendError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	pf := s.galleryPermFilter(r.Context(), claims)

	tags, err := s.galleryStore.ListAllTags(r.Context(), pf)
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
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		s.sendError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	pf := s.galleryPermFilter(r.Context(), claims)

	stats, err := s.galleryStore.GetStats(r.Context(), pf)
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

// ─── Map Points ─────────────────────────────────────────────────────────────

func (s *Server) handleGalleryMapPoints(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		s.sendError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	pf := s.galleryPermFilter(r.Context(), claims)

	points, err := s.galleryStore.GetMapPoints(r.Context(), pf)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to get map points: "+err.Error())
		return
	}

	resp := make([]protocol.MapPointResponse, 0, len(points))
	for _, p := range points {
		resp = append(resp, protocol.MapPointResponse{
			FilePath:     p.FilePath,
			FileName:     p.FileName,
			Latitude:     p.Latitude,
			Longitude:    p.Longitude,
			HasThumbnail: p.HasThumbnail,
			DateTaken:    p.DateTaken,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
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

// ─── Custom Albums ──────────────────────────────────────────────────────────

func (s *Server) handleListUserAlbums(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		s.sendError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	albums, err := s.galleryStore.ListAlbums(r.Context(), claims.UserID)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to list albums: "+err.Error())
		return
	}

	resp := make([]protocol.AlbumResponse, 0, len(albums))
	for _, a := range albums {
		resp = append(resp, protocol.AlbumResponse{
			ID:          a.ID,
			Name:        a.Name,
			Description: a.Description,
			CoverPath:   a.CoverPath,
			ImageCount:  a.ImageCount,
			CreatedAt:   a.CreatedAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleCreateAlbum(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		s.sendError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req protocol.AlbumRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		s.sendError(w, http.StatusBadRequest, "name is required")
		return
	}

	album, err := s.galleryStore.CreateAlbum(r.Context(), claims.UserID, req.Name, req.Description)
	if err != nil {
		if err == gallery.ErrAlbumExists {
			s.sendError(w, http.StatusConflict, "an album with this name already exists")
			return
		}
		s.sendError(w, http.StatusInternalServerError, "failed to create album: "+err.Error())
		return
	}

	logging.Info("album created", zap.String("name", album.Name), zap.Int("user_id", claims.UserID))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(protocol.AlbumResponse{
		ID:          album.ID,
		Name:        album.Name,
		Description: album.Description,
		CreatedAt:   album.CreatedAt,
	})
}

func (s *Server) handleUpdateAlbum(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		s.sendError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid album ID")
		return
	}

	album, err := s.galleryStore.GetAlbum(r.Context(), id)
	if err != nil || album == nil {
		s.sendError(w, http.StatusNotFound, "album not found")
		return
	}
	if album.UserID != claims.UserID && !claims.IsAdmin {
		s.sendError(w, http.StatusForbidden, "access denied")
		return
	}

	var req protocol.AlbumRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		s.sendError(w, http.StatusBadRequest, "name is required")
		return
	}

	if err := s.galleryStore.UpdateAlbum(r.Context(), id, req.Name, req.Description); err != nil {
		if err == gallery.ErrAlbumExists {
			s.sendError(w, http.StatusConflict, "an album with this name already exists")
			return
		}
		s.sendError(w, http.StatusInternalServerError, "failed to update album: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"id": id, "updated": true})
}

func (s *Server) handleDeleteAlbum(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		s.sendError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid album ID")
		return
	}

	album, err := s.galleryStore.GetAlbum(r.Context(), id)
	if err != nil || album == nil {
		s.sendError(w, http.StatusNotFound, "album not found")
		return
	}
	if album.UserID != claims.UserID && !claims.IsAdmin {
		s.sendError(w, http.StatusForbidden, "access denied")
		return
	}

	if err := s.galleryStore.DeleteAlbum(r.Context(), id); err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to delete album: "+err.Error())
		return
	}

	logging.Info("album deleted", zap.Int("id", id))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"id": id, "deleted": true})
}

func (s *Server) handleGetAlbumImages(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		s.sendError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid album ID")
		return
	}

	album, err := s.galleryStore.GetAlbum(r.Context(), id)
	if err != nil || album == nil {
		s.sendError(w, http.StatusNotFound, "album not found")
		return
	}
	if album.UserID != claims.UserID && !claims.IsAdmin {
		s.sendError(w, http.StatusForbidden, "access denied")
		return
	}

	paths, err := s.galleryStore.GetAlbumImages(r.Context(), id)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to get album images: "+err.Error())
		return
	}
	if paths == nil {
		paths = []string{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(paths)
}

func (s *Server) handleAddImageToAlbum(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		s.sendError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid album ID")
		return
	}

	album, err := s.galleryStore.GetAlbum(r.Context(), id)
	if err != nil || album == nil {
		s.sendError(w, http.StatusNotFound, "album not found")
		return
	}
	if album.UserID != claims.UserID && !claims.IsAdmin {
		s.sendError(w, http.StatusForbidden, "access denied")
		return
	}

	var req protocol.AlbumImageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.FilePath == "" {
		s.sendError(w, http.StatusBadRequest, "file_path is required")
		return
	}

	// Verify the user has read access to the file being added
	if !s.permissions.CheckAccess(r.Context(), claims.UserID, req.FilePath, "read", claims.IsAdmin) {
		s.sendError(w, http.StatusForbidden, "no read access to this file")
		return
	}

	if err := s.galleryStore.AddImageToAlbum(r.Context(), id, req.FilePath); err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to add image: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{"album_id": id, "file_path": req.FilePath, "added": true})
}

func (s *Server) handleRemoveImageFromAlbum(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		s.sendError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid album ID")
		return
	}

	album, err := s.galleryStore.GetAlbum(r.Context(), id)
	if err != nil || album == nil {
		s.sendError(w, http.StatusNotFound, "album not found")
		return
	}
	if album.UserID != claims.UserID && !claims.IsAdmin {
		s.sendError(w, http.StatusForbidden, "access denied")
		return
	}

	var req protocol.AlbumImageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.FilePath == "" {
		s.sendError(w, http.StatusBadRequest, "file_path is required")
		return
	}

	if err := s.galleryStore.RemoveImageFromAlbum(r.Context(), id, req.FilePath); err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to remove image: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"album_id": id, "file_path": req.FilePath, "removed": true})
}

func (s *Server) handleSetAlbumCover(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		s.sendError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid album ID")
		return
	}

	album, err := s.galleryStore.GetAlbum(r.Context(), id)
	if err != nil || album == nil {
		s.sendError(w, http.StatusNotFound, "album not found")
		return
	}
	if album.UserID != claims.UserID && !claims.IsAdmin {
		s.sendError(w, http.StatusForbidden, "access denied")
		return
	}

	var req protocol.AlbumCoverRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.CoverPath == "" {
		s.sendError(w, http.StatusBadRequest, "cover_path is required")
		return
	}

	if err := s.galleryStore.SetAlbumCover(r.Context(), id, req.CoverPath); err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to set cover: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"album_id": id, "cover_path": req.CoverPath})
}

func (s *Server) handleGetAlbumsForImage(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		s.sendError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	pathParam := r.PathValue("path")
	if pathParam == "" {
		s.sendError(w, http.StatusBadRequest, "path required")
		return
	}
	filePath := "/" + pathParam

	albums, err := s.galleryStore.GetAlbumsForImage(r.Context(), filePath)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to get albums: "+err.Error())
		return
	}

	resp := make([]protocol.AlbumResponse, 0, len(albums))
	for _, a := range albums {
		resp = append(resp, protocol.AlbumResponse{
			ID:          a.ID,
			Name:        a.Name,
			Description: a.Description,
			CoverPath:   a.CoverPath,
			ImageCount:  a.ImageCount,
			CreatedAt:   a.CreatedAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ─── Admin: Global Tag Management ───────────────────────────────────────────

func (s *Server) handleDeleteTagGlobal(w http.ResponseWriter, r *http.Request) {
	if s.requireAdmin(w, r) == nil {
		return
	}

	tag := r.PathValue("tag")
	if tag == "" {
		s.sendError(w, http.StatusBadRequest, "tag is required")
		return
	}

	count, err := s.galleryStore.DeleteTagGlobal(r.Context(), tag)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to delete tag: "+err.Error())
		return
	}

	logging.Info("global tag deleted", zap.String("tag", tag), zap.Int64("affected", count))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tag":      tag,
		"deleted":  true,
		"affected": count,
	})
}

func (s *Server) handleRenameTagGlobal(w http.ResponseWriter, r *http.Request) {
	if s.requireAdmin(w, r) == nil {
		return
	}

	tag := r.PathValue("tag")
	if tag == "" {
		s.sendError(w, http.StatusBadRequest, "tag is required")
		return
	}

	var req protocol.GlobalTagActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.NewTag == "" {
		s.sendError(w, http.StatusBadRequest, "new_tag is required")
		return
	}

	count, err := s.galleryStore.RenameTagGlobal(r.Context(), tag, req.NewTag)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to rename tag: "+err.Error())
		return
	}

	logging.Info("global tag renamed", zap.String("from", tag), zap.String("to", req.NewTag), zap.Int64("affected", count))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"old_tag":  tag,
		"new_tag":  req.NewTag,
		"affected": count,
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
