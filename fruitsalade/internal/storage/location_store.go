package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// LocationRow maps to the storage_locations table.
type LocationRow struct {
	ID          int             `json:"id"`
	Name        string          `json:"name"`
	GroupID     *int            `json:"group_id"`
	BackendType string          `json:"backend_type"`
	Config      json.RawMessage `json:"config"`
	Priority    int             `json:"priority"`
	IsDefault   bool            `json:"is_default"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// LocationStore provides CRUD operations for storage_locations.
type LocationStore struct {
	db *sql.DB
}

// NewLocationStore creates a new LocationStore.
func NewLocationStore(db *sql.DB) *LocationStore {
	return &LocationStore{db: db}
}

// List returns all storage locations.
func (s *LocationStore) List(ctx context.Context) ([]LocationRow, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, group_id, backend_type, config, priority, is_default, created_at, updated_at
		 FROM storage_locations ORDER BY priority DESC, name`)
	if err != nil {
		return nil, fmt.Errorf("list storage locations: %w", err)
	}
	defer rows.Close()

	var locs []LocationRow
	for rows.Next() {
		var loc LocationRow
		var groupID sql.NullInt64
		if err := rows.Scan(&loc.ID, &loc.Name, &groupID, &loc.BackendType,
			&loc.Config, &loc.Priority, &loc.IsDefault, &loc.CreatedAt, &loc.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan storage location: %w", err)
		}
		if groupID.Valid {
			gid := int(groupID.Int64)
			loc.GroupID = &gid
		}
		locs = append(locs, loc)
	}
	return locs, rows.Err()
}

// Get returns a storage location by ID.
func (s *LocationStore) Get(ctx context.Context, id int) (*LocationRow, error) {
	var loc LocationRow
	var groupID sql.NullInt64
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, group_id, backend_type, config, priority, is_default, created_at, updated_at
		 FROM storage_locations WHERE id = $1`, id).
		Scan(&loc.ID, &loc.Name, &groupID, &loc.BackendType,
			&loc.Config, &loc.Priority, &loc.IsDefault, &loc.CreatedAt, &loc.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get storage location: %w", err)
	}
	if groupID.Valid {
		gid := int(groupID.Int64)
		loc.GroupID = &gid
	}
	return &loc, nil
}

// GetByGroupID returns storage locations for a group, ordered by priority desc.
func (s *LocationStore) GetByGroupID(ctx context.Context, groupID int) ([]LocationRow, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, group_id, backend_type, config, priority, is_default, created_at, updated_at
		 FROM storage_locations WHERE group_id = $1 ORDER BY priority DESC`, groupID)
	if err != nil {
		return nil, fmt.Errorf("get by group: %w", err)
	}
	defer rows.Close()

	var locs []LocationRow
	for rows.Next() {
		var loc LocationRow
		var gid sql.NullInt64
		if err := rows.Scan(&loc.ID, &loc.Name, &gid, &loc.BackendType,
			&loc.Config, &loc.Priority, &loc.IsDefault, &loc.CreatedAt, &loc.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		if gid.Valid {
			g := int(gid.Int64)
			loc.GroupID = &g
		}
		locs = append(locs, loc)
	}
	return locs, rows.Err()
}

// GetDefault returns the default storage location.
func (s *LocationStore) GetDefault(ctx context.Context) (*LocationRow, error) {
	var loc LocationRow
	var groupID sql.NullInt64
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, group_id, backend_type, config, priority, is_default, created_at, updated_at
		 FROM storage_locations WHERE is_default = TRUE LIMIT 1`).
		Scan(&loc.ID, &loc.Name, &groupID, &loc.BackendType,
			&loc.Config, &loc.Priority, &loc.IsDefault, &loc.CreatedAt, &loc.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get default: %w", err)
	}
	if groupID.Valid {
		gid := int(groupID.Int64)
		loc.GroupID = &gid
	}
	return &loc, nil
}

// Create inserts a new storage location and returns it with the generated ID.
func (s *LocationStore) Create(ctx context.Context, loc *LocationRow) (*LocationRow, error) {
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO storage_locations (name, group_id, backend_type, config, priority, is_default)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, created_at, updated_at`,
		loc.Name, loc.GroupID, loc.BackendType, loc.Config, loc.Priority, loc.IsDefault).
		Scan(&loc.ID, &loc.CreatedAt, &loc.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create storage location: %w", err)
	}
	return loc, nil
}

// Update modifies an existing storage location.
func (s *LocationStore) Update(ctx context.Context, loc *LocationRow) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE storage_locations
		 SET name = $2, group_id = $3, backend_type = $4, config = $5, priority = $6, updated_at = NOW()
		 WHERE id = $1`,
		loc.ID, loc.Name, loc.GroupID, loc.BackendType, loc.Config, loc.Priority)
	if err != nil {
		return fmt.Errorf("update storage location: %w", err)
	}
	return nil
}

// Delete removes a storage location. Fails if files still reference it.
func (s *LocationStore) Delete(ctx context.Context, id int) error {
	var count int64
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM files WHERE storage_location_id = $1`, id).Scan(&count)
	if err != nil {
		return fmt.Errorf("check files: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("cannot delete: %d files still use this storage location", count)
	}

	_, err = s.db.ExecContext(ctx,
		`DELETE FROM storage_locations WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete storage location: %w", err)
	}
	return nil
}

// SetDefault sets a location as the default (clears previous default).
func (s *LocationStore) SetDefault(ctx context.Context, id int) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx,
		`UPDATE storage_locations SET is_default = FALSE WHERE is_default = TRUE`)
	if err != nil {
		return fmt.Errorf("clear default: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		`UPDATE storage_locations SET is_default = TRUE, updated_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("set default: %w", err)
	}

	return tx.Commit()
}

// Stats returns file count and total size for a storage location.
func (s *LocationStore) Stats(ctx context.Context, id int) (fileCount int64, totalSize int64, err error) {
	err = s.db.QueryRowContext(ctx,
		`SELECT COUNT(*), COALESCE(SUM(size), 0)
		 FROM files WHERE storage_location_id = $1 AND is_dir = FALSE`, id).
		Scan(&fileCount, &totalSize)
	if err != nil {
		return 0, 0, fmt.Errorf("stats: %w", err)
	}
	return fileCount, totalSize, nil
}
