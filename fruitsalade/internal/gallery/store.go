// Package gallery provides image metadata, tagging, thumbnail generation,
// and auto-tagging plugin support for the photo gallery feature.
package gallery

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// ImageMetadata represents a row in the image_metadata table.
type ImageMetadata struct {
	ID              int        `json:"id"`
	FilePath        string     `json:"file_path"`
	Width           int        `json:"width"`
	Height          int        `json:"height"`
	CameraMake      string     `json:"camera_make"`
	CameraModel     string     `json:"camera_model"`
	LensModel       string     `json:"lens_model"`
	FocalLength     float32    `json:"focal_length"`
	Aperture        float32    `json:"aperture"`
	ShutterSpeed    string     `json:"shutter_speed"`
	ISO             int        `json:"iso"`
	Flash           bool       `json:"flash"`
	DateTaken       *time.Time `json:"date_taken"`
	Latitude        *float64   `json:"latitude"`
	Longitude       *float64   `json:"longitude"`
	Altitude        *float32   `json:"altitude"`
	LocationCountry string     `json:"location_country"`
	LocationCity    string     `json:"location_city"`
	LocationName    string     `json:"location_name"`
	Orientation     int        `json:"orientation"`
	HasThumbnail    bool       `json:"has_thumbnail"`
	ThumbS3Key      string     `json:"thumb_s3_key"`
	Status          string     `json:"status"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// ImageTag represents a row in the image_tags table.
type ImageTag struct {
	ID         int       `json:"id"`
	FilePath   string    `json:"file_path"`
	Tag        string    `json:"tag"`
	Confidence float32   `json:"confidence"`
	Source     string    `json:"source"`
	CreatedAt  time.Time `json:"created_at"`
}

// Plugin represents a row in the tagging_plugins table.
type Plugin struct {
	ID         int                    `json:"id"`
	Name       string                 `json:"name"`
	WebhookURL string                 `json:"webhook_url"`
	Enabled    bool                   `json:"enabled"`
	Config     map[string]interface{} `json:"config,omitempty"`
	LastHealth *time.Time             `json:"last_health,omitempty"`
	LastError  string                 `json:"last_error,omitempty"`
	CreatedAt  time.Time              `json:"created_at"`
	UpdatedAt  time.Time              `json:"updated_at"`
}

// GalleryStore provides CRUD for image_metadata, image_tags, and tagging_plugins.
type GalleryStore struct {
	db *sql.DB
}

// NewGalleryStore creates a new GalleryStore.
func NewGalleryStore(db *sql.DB) *GalleryStore {
	return &GalleryStore{db: db}
}

// DB returns the underlying database connection.
func (s *GalleryStore) DB() *sql.DB {
	return s.db
}

// ─── Image Metadata ─────────────────────────────────────────────────────────

// UpsertMetadata inserts or updates image metadata for a file.
func (s *GalleryStore) UpsertMetadata(ctx context.Context, m *ImageMetadata) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO image_metadata (
			file_path, width, height, camera_make, camera_model, lens_model,
			focal_length, aperture, shutter_speed, iso, flash,
			date_taken, latitude, longitude, altitude,
			location_country, location_city, location_name,
			orientation, has_thumbnail, thumb_s3_key, status, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,NOW())
		ON CONFLICT (file_path) DO UPDATE SET
			width=$2, height=$3, camera_make=$4, camera_model=$5, lens_model=$6,
			focal_length=$7, aperture=$8, shutter_speed=$9, iso=$10, flash=$11,
			date_taken=$12, latitude=$13, longitude=$14, altitude=$15,
			location_country=$16, location_city=$17, location_name=$18,
			orientation=$19, has_thumbnail=$20, thumb_s3_key=$21, status=$22,
			updated_at=NOW()`,
		m.FilePath, m.Width, m.Height, m.CameraMake, m.CameraModel, m.LensModel,
		m.FocalLength, m.Aperture, m.ShutterSpeed, m.ISO, m.Flash,
		m.DateTaken, m.Latitude, m.Longitude, m.Altitude,
		m.LocationCountry, m.LocationCity, m.LocationName,
		m.Orientation, m.HasThumbnail, m.ThumbS3Key, m.Status,
	)
	return err
}

// GetMetadata retrieves image metadata for a file path.
func (s *GalleryStore) GetMetadata(ctx context.Context, filePath string) (*ImageMetadata, error) {
	m := &ImageMetadata{}
	err := s.db.QueryRowContext(ctx, `
		SELECT id, file_path, width, height, camera_make, camera_model, lens_model,
			focal_length, aperture, shutter_speed, iso, flash,
			date_taken, latitude, longitude, altitude,
			location_country, location_city, location_name,
			orientation, has_thumbnail, thumb_s3_key, status, created_at, updated_at
		FROM image_metadata WHERE file_path = $1`, filePath,
	).Scan(
		&m.ID, &m.FilePath, &m.Width, &m.Height, &m.CameraMake, &m.CameraModel, &m.LensModel,
		&m.FocalLength, &m.Aperture, &m.ShutterSpeed, &m.ISO, &m.Flash,
		&m.DateTaken, &m.Latitude, &m.Longitude, &m.Altitude,
		&m.LocationCountry, &m.LocationCity, &m.LocationName,
		&m.Orientation, &m.HasThumbnail, &m.ThumbS3Key, &m.Status, &m.CreatedAt, &m.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return m, err
}

// ListPendingProcessing returns file paths of images with status 'pending'.
func (s *GalleryStore) ListPendingProcessing(ctx context.Context, limit int) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT file_path FROM image_metadata WHERE status = 'pending' ORDER BY created_at LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		paths = append(paths, p)
	}
	return paths, rows.Err()
}

// SetStatus updates the processing status of an image.
func (s *GalleryStore) SetStatus(ctx context.Context, filePath, status string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE image_metadata SET status = $1, updated_at = NOW() WHERE file_path = $2`,
		status, filePath)
	return err
}

// DeleteMetadata removes image metadata for a file.
func (s *GalleryStore) DeleteMetadata(ctx context.Context, filePath string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM image_metadata WHERE file_path = $1`, filePath)
	return err
}

// GetThumbKey returns the thumbnail S3 key for a file, or empty string if none.
func (s *GalleryStore) GetThumbKey(ctx context.Context, filePath string) string {
	var key string
	s.db.QueryRowContext(ctx,
		`SELECT thumb_s3_key FROM image_metadata WHERE file_path = $1 AND has_thumbnail = TRUE`, filePath,
	).Scan(&key)
	return key
}

// ─── Tags ───────────────────────────────────────────────────────────────────

// AddTag adds a tag to an image. Returns error on conflict.
func (s *GalleryStore) AddTag(ctx context.Context, filePath, tag, source string, confidence float32) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO image_tags (file_path, tag, confidence, source)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (file_path, tag, source) DO UPDATE SET confidence = $3`,
		filePath, tag, confidence, source)
	return err
}

// RemoveTag removes a tag from an image (manual source only).
func (s *GalleryStore) RemoveTag(ctx context.Context, filePath, tag string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM image_tags WHERE file_path = $1 AND tag = $2`, filePath, tag)
	return err
}

// GetTagsForFile returns all tags for a file.
func (s *GalleryStore) GetTagsForFile(ctx context.Context, filePath string) ([]ImageTag, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, file_path, tag, confidence, source, created_at
		FROM image_tags WHERE file_path = $1 ORDER BY tag`, filePath)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []ImageTag
	for rows.Next() {
		var t ImageTag
		if err := rows.Scan(&t.ID, &t.FilePath, &t.Tag, &t.Confidence, &t.Source, &t.CreatedAt); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

// ListAllTags returns all distinct tags with counts.
func (s *GalleryStore) ListAllTags(ctx context.Context) ([]TagCount, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT tag, COUNT(*) as cnt FROM image_tags GROUP BY tag ORDER BY cnt DESC, tag`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []TagCount
	for rows.Next() {
		var t TagCount
		if err := rows.Scan(&t.Tag, &t.Count); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

// TagCount holds a tag and its usage count.
type TagCount struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

// ─── Stats ──────────────────────────────────────────────────────────────────

// Stats holds aggregate gallery statistics.
type Stats struct {
	TotalImages int `json:"total_images"`
	WithGPS     int `json:"with_gps"`
	WithTags    int `json:"with_tags"`
	Processed   int `json:"processed"`
	Pending     int `json:"pending"`
}

// GetStats returns gallery-wide statistics.
func (s *GalleryStore) GetStats(ctx context.Context) (*Stats, error) {
	st := &Stats{}
	err := s.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*),
			COUNT(*) FILTER (WHERE latitude IS NOT NULL AND longitude IS NOT NULL),
			(SELECT COUNT(DISTINCT file_path) FROM image_tags),
			COUNT(*) FILTER (WHERE status = 'done'),
			COUNT(*) FILTER (WHERE status = 'pending')
		FROM image_metadata`).Scan(
		&st.TotalImages, &st.WithGPS, &st.WithTags, &st.Processed, &st.Pending)
	if err != nil {
		return nil, err
	}
	return st, nil
}

// ─── Plugins ────────────────────────────────────────────────────────────────

// CreatePlugin inserts a new tagging plugin.
func (s *GalleryStore) CreatePlugin(ctx context.Context, p *Plugin) (*Plugin, error) {
	cfgJSON, err := json.Marshal(p.Config)
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}

	err = s.db.QueryRowContext(ctx, `
		INSERT INTO tagging_plugins (name, webhook_url, enabled, config)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, updated_at`,
		p.Name, p.WebhookURL, p.Enabled, cfgJSON,
	).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// GetPlugin retrieves a plugin by ID.
func (s *GalleryStore) GetPlugin(ctx context.Context, id int) (*Plugin, error) {
	p := &Plugin{}
	var cfgJSON []byte
	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, webhook_url, enabled, config, last_health, last_error, created_at, updated_at
		FROM tagging_plugins WHERE id = $1`, id,
	).Scan(&p.ID, &p.Name, &p.WebhookURL, &p.Enabled, &cfgJSON,
		&p.LastHealth, &p.LastError, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(cfgJSON) > 0 {
		json.Unmarshal(cfgJSON, &p.Config)
	}
	return p, nil
}

// ListPlugins returns all tagging plugins.
func (s *GalleryStore) ListPlugins(ctx context.Context) ([]Plugin, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, webhook_url, enabled, config, last_health, last_error, created_at, updated_at
		FROM tagging_plugins ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var plugins []Plugin
	for rows.Next() {
		var p Plugin
		var cfgJSON []byte
		if err := rows.Scan(&p.ID, &p.Name, &p.WebhookURL, &p.Enabled, &cfgJSON,
			&p.LastHealth, &p.LastError, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		if len(cfgJSON) > 0 {
			json.Unmarshal(cfgJSON, &p.Config)
		}
		plugins = append(plugins, p)
	}
	return plugins, rows.Err()
}

// ListEnabledPlugins returns only enabled plugins.
func (s *GalleryStore) ListEnabledPlugins(ctx context.Context) ([]Plugin, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, webhook_url, enabled, config, last_health, last_error, created_at, updated_at
		FROM tagging_plugins WHERE enabled = TRUE ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var plugins []Plugin
	for rows.Next() {
		var p Plugin
		var cfgJSON []byte
		if err := rows.Scan(&p.ID, &p.Name, &p.WebhookURL, &p.Enabled, &cfgJSON,
			&p.LastHealth, &p.LastError, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		if len(cfgJSON) > 0 {
			json.Unmarshal(cfgJSON, &p.Config)
		}
		plugins = append(plugins, p)
	}
	return plugins, rows.Err()
}

// UpdatePlugin updates a tagging plugin.
func (s *GalleryStore) UpdatePlugin(ctx context.Context, p *Plugin) error {
	cfgJSON, err := json.Marshal(p.Config)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		UPDATE tagging_plugins SET
			name = $1, webhook_url = $2, enabled = $3, config = $4, updated_at = NOW()
		WHERE id = $5`,
		p.Name, p.WebhookURL, p.Enabled, cfgJSON, p.ID)
	return err
}

// DeletePlugin removes a tagging plugin.
func (s *GalleryStore) DeletePlugin(ctx context.Context, id int) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM tagging_plugins WHERE id = $1`, id)
	return err
}

// UpdatePluginHealth records the result of a plugin health check.
func (s *GalleryStore) UpdatePluginHealth(ctx context.Context, id int, lastError string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE tagging_plugins SET last_health = NOW(), last_error = $1, updated_at = NOW()
		WHERE id = $2`, lastError, id)
	return err
}
