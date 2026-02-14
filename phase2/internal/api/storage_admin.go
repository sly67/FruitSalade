package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"go.uber.org/zap"

	"github.com/fruitsalade/fruitsalade/phase2/internal/logging"
	"github.com/fruitsalade/fruitsalade/phase2/internal/storage"
)

// ─── Admin: Storage Locations ──────────────────────────────────────────────────

func (s *Server) handleListStorageLocations(w http.ResponseWriter, r *http.Request) {
	if s.requireAdmin(w, r) == nil {
		return
	}

	locs, err := s.locationStore.List(r.Context())
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to list storage locations: "+err.Error())
		return
	}

	// Redact secrets in configs
	resp := make([]map[string]interface{}, 0, len(locs))
	for _, loc := range locs {
		resp = append(resp, redactedLocationMap(loc))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleGetStorageLocation(w http.ResponseWriter, r *http.Request) {
	if s.requireAdmin(w, r) == nil {
		return
	}

	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid storage location ID")
		return
	}

	loc, err := s.locationStore.Get(r.Context(), id)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to get storage location: "+err.Error())
		return
	}
	if loc == nil {
		s.sendError(w, http.StatusNotFound, "storage location not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(redactedLocationMap(*loc))
}

func (s *Server) handleCreateStorageLocation(w http.ResponseWriter, r *http.Request) {
	if s.requireAdmin(w, r) == nil {
		return
	}

	var req struct {
		Name        string          `json:"name"`
		GroupID     *int            `json:"group_id"`
		BackendType string          `json:"backend_type"`
		Config      json.RawMessage `json:"config"`
		Priority    int             `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		s.sendError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.BackendType == "" {
		s.sendError(w, http.StatusBadRequest, "backend_type is required")
		return
	}

	row := &storage.LocationRow{
		Name:        req.Name,
		GroupID:     req.GroupID,
		BackendType: req.BackendType,
		Config:      req.Config,
		Priority:    req.Priority,
	}

	created, err := s.locationStore.Create(r.Context(), row)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to create storage location: "+err.Error())
		return
	}

	// Reload router to pick up the new location
	if err := s.storageRouter.Reload(r.Context()); err != nil {
		logging.Error("failed to reload storage router after create", zap.Error(err))
	}

	logging.Info("storage location created",
		zap.Int("id", created.ID),
		zap.String("name", created.Name),
		zap.String("type", created.BackendType))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(redactedLocationMap(*created))
}

func (s *Server) handleUpdateStorageLocation(w http.ResponseWriter, r *http.Request) {
	if s.requireAdmin(w, r) == nil {
		return
	}

	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid storage location ID")
		return
	}

	existing, err := s.locationStore.Get(r.Context(), id)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to get storage location: "+err.Error())
		return
	}
	if existing == nil {
		s.sendError(w, http.StatusNotFound, "storage location not found")
		return
	}

	var req struct {
		Name        *string         `json:"name"`
		GroupID     *int            `json:"group_id"`
		BackendType *string         `json:"backend_type"`
		Config      json.RawMessage `json:"config"`
		Priority    *int            `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.GroupID != nil {
		existing.GroupID = req.GroupID
	}
	if req.BackendType != nil {
		existing.BackendType = *req.BackendType
	}
	if req.Config != nil {
		existing.Config = req.Config
	}
	if req.Priority != nil {
		existing.Priority = *req.Priority
	}

	if err := s.locationStore.Update(r.Context(), existing); err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to update storage location: "+err.Error())
		return
	}

	// Reload router to pick up config changes
	if err := s.storageRouter.Reload(r.Context()); err != nil {
		logging.Error("failed to reload storage router after update", zap.Error(err))
	}

	logging.Info("storage location updated",
		zap.Int("id", id),
		zap.String("name", existing.Name))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(redactedLocationMap(*existing))
}

func (s *Server) handleDeleteStorageLocation(w http.ResponseWriter, r *http.Request) {
	if s.requireAdmin(w, r) == nil {
		return
	}

	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid storage location ID")
		return
	}

	if err := s.locationStore.Delete(r.Context(), id); err != nil {
		s.sendError(w, http.StatusConflict, err.Error())
		return
	}

	// Reload router
	if err := s.storageRouter.Reload(r.Context()); err != nil {
		logging.Error("failed to reload storage router after delete", zap.Error(err))
	}

	logging.Info("storage location deleted", zap.Int("id", id))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":      id,
		"deleted": true,
	})
}

func (s *Server) handleTestStorageLocation(w http.ResponseWriter, r *http.Request) {
	if s.requireAdmin(w, r) == nil {
		return
	}

	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid storage location ID")
		return
	}

	loc, err := s.locationStore.Get(r.Context(), id)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to get storage location: "+err.Error())
		return
	}
	if loc == nil {
		s.sendError(w, http.StatusNotFound, "storage location not found")
		return
	}

	// Create a temporary backend to test connectivity
	ctx := r.Context()
	backend, err := storage.NewBackendFromConfig(ctx, loc.BackendType, loc.Config)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "failed to create backend: " + err.Error(),
		})
		return
	}
	defer backend.Close()

	// Try write + read + delete of a test file
	testKey := "_fruitsalade_test_probe"
	testData := []byte("fruitsalade-connectivity-test")

	if err := backend.PutObject(ctx, testKey, strings.NewReader(string(testData)), int64(len(testData))); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "write test failed: " + err.Error(),
		})
		return
	}

	rc, _, err := backend.GetObject(ctx, testKey, 0, 0)
	if err != nil {
		backend.DeleteObject(ctx, testKey)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "read test failed: " + err.Error(),
		})
		return
	}
	rc.Close()

	backend.DeleteObject(ctx, testKey)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":      true,
		"backend_type": loc.BackendType,
	})
}

func (s *Server) handleSetDefaultStorage(w http.ResponseWriter, r *http.Request) {
	if s.requireAdmin(w, r) == nil {
		return
	}

	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid storage location ID")
		return
	}

	if err := s.locationStore.SetDefault(r.Context(), id); err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to set default: "+err.Error())
		return
	}

	// Reload router
	if err := s.storageRouter.Reload(r.Context()); err != nil {
		logging.Error("failed to reload storage router after set default", zap.Error(err))
	}

	logging.Info("storage location set as default", zap.Int("id", id))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":      id,
		"default": true,
	})
}

func (s *Server) handleStorageStats(w http.ResponseWriter, r *http.Request) {
	if s.requireAdmin(w, r) == nil {
		return
	}

	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid storage location ID")
		return
	}

	fileCount, totalSize, err := s.locationStore.Stats(r.Context(), id)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to get stats: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"location_id": id,
		"file_count":  fileCount,
		"total_size":  totalSize,
	})
}

// redactedLocationMap converts a LocationRow to a JSON-friendly map with secrets redacted.
func redactedLocationMap(loc storage.LocationRow) map[string]interface{} {
	m := map[string]interface{}{
		"id":           loc.ID,
		"name":         loc.Name,
		"group_id":     loc.GroupID,
		"backend_type": loc.BackendType,
		"priority":     loc.Priority,
		"is_default":   loc.IsDefault,
		"created_at":   loc.CreatedAt,
		"updated_at":   loc.UpdatedAt,
	}

	// Parse config and redact secrets
	var cfg map[string]interface{}
	if err := json.Unmarshal(loc.Config, &cfg); err == nil {
		for k := range cfg {
			lower := strings.ToLower(k)
			if strings.Contains(lower, "secret") || strings.Contains(lower, "password") {
				if s, ok := cfg[k].(string); ok && s != "" {
					cfg[k] = "***"
				}
			}
		}
		m["config"] = cfg
	} else {
		m["config"] = loc.Config
	}

	return m
}
