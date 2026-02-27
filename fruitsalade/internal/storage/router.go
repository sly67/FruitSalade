package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"go.uber.org/zap"

	"github.com/fruitsalade/fruitsalade/fruitsalade/internal/logging"
	"github.com/fruitsalade/fruitsalade/fruitsalade/internal/sharing"
)

// ErrReadOnlyStorage is returned when a write operation targets a read-only storage location.
var ErrReadOnlyStorage = errors.New("storage location is read-only")

// StorageLocation pairs a LocationRow with its instantiated Backend.
type StorageLocation struct {
	LocationRow
	Backend Backend
}

// Router resolves which storage backend to use for a given file or upload.
type Router struct {
	mu         sync.RWMutex
	locations  map[int]*StorageLocation   // id -> location
	groupMap   map[int][]*StorageLocation // root_group_id -> locations (by priority)
	defaultLoc *StorageLocation
	locStore   *LocationStore
	groupStore *sharing.GroupStore
}

// NewRouter creates a Router and loads all configured storage locations.
func NewRouter(ctx context.Context, locStore *LocationStore, groupStore *sharing.GroupStore) (*Router, error) {
	r := &Router{
		locations:  make(map[int]*StorageLocation),
		groupMap:   make(map[int][]*StorageLocation),
		locStore:   locStore,
		groupStore: groupStore,
	}

	if err := r.Reload(ctx); err != nil {
		return nil, fmt.Errorf("initial load: %w", err)
	}

	return r, nil
}

// Reload re-reads all storage locations from the database and re-instantiates backends.
func (r *Router) Reload(ctx context.Context) error {
	rows, err := r.locStore.List(ctx)
	if err != nil {
		return err
	}

	newLocations := make(map[int]*StorageLocation, len(rows))
	newGroupMap := make(map[int][]*StorageLocation)
	var newDefault *StorageLocation

	for _, row := range rows {
		row := row

		// Reuse existing backend if config hasn't changed
		r.mu.RLock()
		existing := r.locations[row.ID]
		r.mu.RUnlock()

		var backend Backend
		if existing != nil && string(existing.Config) == string(row.Config) && existing.BackendType == row.BackendType {
			backend = existing.Backend
		} else {
			backend, err = NewBackendFromConfig(ctx, row.BackendType, row.Config)
			if err != nil {
				logging.Error("failed to initialize storage backend",
					zap.Int("location_id", row.ID),
					zap.String("name", row.Name),
					zap.Error(err))
				continue
			}
			// Close old backend if replaced
			if existing != nil && existing.Backend != nil {
				existing.Backend.Close()
			}
		}

		loc := &StorageLocation{
			LocationRow: row,
			Backend:     backend,
		}

		newLocations[row.ID] = loc

		if row.IsDefault {
			newDefault = loc
		}

		if row.GroupID != nil {
			// Find root group
			rootGroup, err := r.groupStore.GetTopLevelGroup(ctx, *row.GroupID)
			if err == nil && rootGroup != nil {
				newGroupMap[rootGroup.ID] = append(newGroupMap[rootGroup.ID], loc)
			} else {
				// Use the group itself as root
				newGroupMap[*row.GroupID] = append(newGroupMap[*row.GroupID], loc)
			}
		}
	}

	r.mu.Lock()
	r.locations = newLocations
	r.groupMap = newGroupMap
	r.defaultLoc = newDefault
	r.mu.Unlock()

	logging.Info("storage router reloaded",
		zap.Int("locations", len(newLocations)),
		zap.Int("group_mappings", len(newGroupMap)),
		zap.Bool("has_default", newDefault != nil))

	return nil
}

// ResolveForFile resolves which backend holds an existing file's content.
// Priority: storageLocID (explicit) > groupID (walk to root) > default.
func (r *Router) ResolveForFile(ctx context.Context, storageLocID *int, groupID *int) (Backend, *StorageLocation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 1. Explicit storage location
	if storageLocID != nil {
		if loc, ok := r.locations[*storageLocID]; ok {
			return loc.Backend, loc, nil
		}
	}

	// 2. Group-based resolution
	if groupID != nil && *groupID > 0 {
		loc := r.resolveByGroup(ctx, *groupID)
		if loc != nil {
			return loc.Backend, loc, nil
		}
	}

	// 3. Default
	if r.defaultLoc != nil {
		return r.defaultLoc.Backend, r.defaultLoc, nil
	}

	return nil, nil, fmt.Errorf("no storage backend available")
}

// ResolveForUpload resolves which backend to use for a new file upload.
// Priority: groupID (walk to root) > path-based group match > default.
// Returns ErrReadOnlyStorage if the resolved location is read-only.
func (r *Router) ResolveForUpload(ctx context.Context, path string, groupID *int) (Backend, *StorageLocation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 1. Known group
	if groupID != nil && *groupID > 0 {
		loc := r.resolveByGroup(ctx, *groupID)
		if loc != nil {
			if loc.ReadOnly {
				return nil, loc, ErrReadOnlyStorage
			}
			return loc.Backend, loc, nil
		}
	}

	// 2. Path-based: extract first segment and try to match a root group
	if path != "" && path != "/" {
		firstSeg := extractFirstSegment(path)
		if firstSeg != "" {
			loc := r.resolveByGroupName(ctx, firstSeg)
			if loc != nil {
				if loc.ReadOnly {
					return nil, loc, ErrReadOnlyStorage
				}
				return loc.Backend, loc, nil
			}
		}
	}

	// 3. Default
	if r.defaultLoc != nil {
		if r.defaultLoc.ReadOnly {
			return nil, r.defaultLoc, ErrReadOnlyStorage
		}
		return r.defaultLoc.Backend, r.defaultLoc, nil
	}

	return nil, nil, fmt.Errorf("no storage backend available")
}

// IsReadOnly returns whether a storage location is read-only.
func (r *Router) IsReadOnly(locID int) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if loc, ok := r.locations[locID]; ok {
		return loc.ReadOnly
	}
	return false
}

// GetDefault returns the default backend and location.
func (r *Router) GetDefault() (Backend, *StorageLocation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.defaultLoc != nil {
		return r.defaultLoc.Backend, r.defaultLoc, nil
	}
	return nil, nil, fmt.Errorf("no default storage backend configured")
}

// DefaultLocation returns the default StorageLocation or nil.
func (r *Router) DefaultLocation() *StorageLocation {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.defaultLoc
}

// GetLocation returns a location by ID.
func (r *Router) GetLocation(id int) *StorageLocation {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.locations[id]
}

// resolveByGroup finds the highest-priority location for a group's root group.
// Must be called with r.mu held.
func (r *Router) resolveByGroup(ctx context.Context, groupID int) *StorageLocation {
	// Walk up to root group
	rootGroup, err := r.groupStore.GetTopLevelGroup(ctx, groupID)
	var rootID int
	if err == nil && rootGroup != nil {
		rootID = rootGroup.ID
	} else {
		rootID = groupID
	}

	locs := r.groupMap[rootID]
	if len(locs) > 0 {
		return locs[0] // Already sorted by priority desc
	}
	return nil
}

// resolveByGroupName tries to match a path segment to a root group name.
// Must be called with r.mu held.
func (r *Router) resolveByGroupName(ctx context.Context, name string) *StorageLocation {
	// Search all group mappings for a matching group name
	for gid, locs := range r.groupMap {
		g, err := r.groupStore.GetGroup(ctx, gid)
		if err == nil && g != nil && strings.EqualFold(g.Name, name) {
			if len(locs) > 0 {
				return locs[0]
			}
		}
	}
	return nil
}

// extractFirstSegment returns the first path component (without leading slash).
func extractFirstSegment(path string) string {
	path = strings.TrimPrefix(path, "/")
	if idx := strings.IndexByte(path, '/'); idx > 0 {
		return path[:idx]
	}
	return path
}

// Close closes all backend connections.
func (r *Router) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, loc := range r.locations {
		if loc.Backend != nil {
			loc.Backend.Close()
		}
	}
	return nil
}
