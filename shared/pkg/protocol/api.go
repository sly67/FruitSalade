// Package protocol defines the API request/response types.
package protocol

import (
	"time"

	"github.com/fruitsalade/fruitsalade/shared/pkg/models"
)

// TreeResponse is returned by GET /api/v1/tree
type TreeResponse struct {
	Root *models.FileNode `json:"root"`
}

// ErrorResponse is returned on API errors.
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Details string `json:"details,omitempty"`
}

// ContentRequest parameters for GET /api/v1/content/{id}
// Range header: "bytes=start-end"
type ContentRequest struct {
	FileID string
	Offset int64 // From Range header
	Length int64 // From Range header, -1 for full file
}

// VersionInfo describes a single version of a file.
type VersionInfo struct {
	Version   int       `json:"version"`
	Size      int64     `json:"size"`
	Hash      string    `json:"hash"`
	CreatedAt time.Time `json:"created_at"`
}

// VersionListResponse is returned by GET /api/v1/versions/{path}
type VersionListResponse struct {
	Path           string        `json:"path"`
	CurrentVersion int           `json:"current_version"`
	Versions       []VersionInfo `json:"versions"`
}

// RollbackRequest is the body for POST /api/v1/versions/{path}/rollback
type RollbackRequest struct {
	Version int `json:"version"`
}

// ConflictResponse is returned when a write conflicts with the current state.
type ConflictResponse struct {
	Error           string `json:"error"`
	Path            string `json:"path"`
	ExpectedVersion int    `json:"expected_version"`
	CurrentVersion  int    `json:"current_version"`
	CurrentHash     string `json:"current_hash"`
}

// SSEEvent represents a server-sent event for real-time sync.
type SSEEvent struct {
	Type      string `json:"type"`
	Path      string `json:"path"`
	Version   int    `json:"version,omitempty"`
	Hash      string `json:"hash,omitempty"`
	Size      int64  `json:"size,omitempty"`
	Timestamp int64  `json:"timestamp"`
}

// PermissionRequest is the body for PUT /api/v1/permissions/{path}.
type PermissionRequest struct {
	UserID     int    `json:"user_id"`
	Permission string `json:"permission"` // "read", "write", "owner"
}

// PermissionResponse describes a single permission entry.
type PermissionResponse struct {
	UserID     int    `json:"user_id"`
	Username   string `json:"username,omitempty"`
	Path       string `json:"path"`
	Permission string `json:"permission"`
}

// PermissionListResponse is returned by GET /api/v1/permissions/{path}.
type PermissionListResponse struct {
	Path        string               `json:"path"`
	Permissions []PermissionResponse `json:"permissions"`
}

// ShareLinkRequest is the body for POST /api/v1/share/{path}.
type ShareLinkRequest struct {
	Password     string `json:"password,omitempty"`
	ExpiresInSec int64  `json:"expires_in_sec,omitempty"` // 0 = no expiry
	MaxDownloads int    `json:"max_downloads,omitempty"`  // 0 = unlimited
}

// ShareLinkResponse is returned when creating a share link.
type ShareLinkResponse struct {
	ID           string     `json:"id"`
	Path         string     `json:"path"`
	URL          string     `json:"url"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	MaxDownloads int        `json:"max_downloads"`
	CreatedAt    time.Time  `json:"created_at"`
}

// ShareInfoResponse is returned by GET /api/v1/share/{token}/info.
type ShareInfoResponse struct {
	FileName    string     `json:"file_name"`
	FileSize    int64      `json:"file_size"`
	HasPassword bool       `json:"has_password"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	Valid       bool       `json:"valid"`
	Error       string     `json:"error,omitempty"`
}

// UserQuotaResponse describes a user's quota settings.
type UserQuotaResponse struct {
	UserID              int   `json:"user_id"`
	MaxStorageBytes     int64 `json:"max_storage_bytes"`
	MaxBandwidthPerDay  int64 `json:"max_bandwidth_per_day"`
	MaxRequestsPerMin   int   `json:"max_requests_per_minute"`
	MaxUploadSizeBytes  int64 `json:"max_upload_size_bytes"`
}

// SetQuotaRequest is the body for PUT /api/v1/admin/quotas/{userID}.
type SetQuotaRequest struct {
	MaxStorageBytes     *int64 `json:"max_storage_bytes,omitempty"`
	MaxBandwidthPerDay  *int64 `json:"max_bandwidth_per_day,omitempty"`
	MaxRequestsPerMin   *int   `json:"max_requests_per_minute,omitempty"`
	MaxUploadSizeBytes  *int64 `json:"max_upload_size_bytes,omitempty"`
}

// UsageResponse describes a user's current resource usage.
type UsageResponse struct {
	UserID          int   `json:"user_id"`
	StorageUsed     int64 `json:"storage_used"`
	BandwidthToday  int64 `json:"bandwidth_today"`
	Quota           UserQuotaResponse `json:"quota"`
}

// GroupRequest is the body for POST /api/v1/admin/groups.
type GroupRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	ParentID    *int   `json:"parent_id,omitempty"`
}

// GroupMemberRequest is the body for POST /api/v1/admin/groups/{id}/members.
type GroupMemberRequest struct {
	UserID int    `json:"user_id"`
	Role   string `json:"role"` // "admin"|"editor"|"viewer", default "viewer"
}

// SetVisibilityRequest is the body for PUT /api/v1/visibility/{path}.
type SetVisibilityRequest struct {
	Visibility string `json:"visibility"` // "public"|"group"|"private"
}

// GroupTreeNode represents a group in a nested tree.
type GroupTreeNode struct {
	ID          int             `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	MemberCount int             `json:"member_count"`
	Children    []GroupTreeNode `json:"children,omitempty"`
}

// UpdateRoleRequest is the body for PUT /api/v1/admin/groups/{id}/members/{uid}/role.
type UpdateRoleRequest struct {
	Role string `json:"role"` // "admin"|"editor"|"viewer"
}

// MoveGroupRequest is the body for PUT /api/v1/admin/groups/{id}/parent.
type MoveGroupRequest struct {
	ParentID *int `json:"parent_id"` // nil = make top-level
}

// GroupPermissionRequest is the body for PUT /api/v1/admin/groups/{id}/permissions/{path}.
type GroupPermissionRequest struct {
	Permission string `json:"permission"`
}

// FilePropertiesResponse is returned by GET /api/v1/properties/{path}.
type FilePropertiesResponse struct {
	// Core metadata
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Path     string    `json:"path"`
	Size     int64     `json:"size"`
	ModTime  time.Time `json:"mod_time"`
	IsDir    bool      `json:"is_dir"`
	Hash     string    `json:"hash,omitempty"`
	Version  int       `json:"version"`

	// Ownership
	OwnerID   int    `json:"owner_id,omitempty"`
	OwnerName string `json:"owner_name,omitempty"`

	// Group
	GroupID   int    `json:"group_id,omitempty"`
	GroupName string `json:"group_name,omitempty"`

	// Visibility
	Visibility string `json:"visibility"`

	// Permissions
	Permissions []PermissionResponse `json:"permissions,omitempty"`

	// Share links
	ShareLinks []ShareLinkInfo `json:"share_links,omitempty"`

	// Versions
	VersionCount int `json:"version_count"`
}

// UserDashboardResponse is returned by GET /api/v1/user/dashboard.
type UserDashboardResponse struct {
	UserID         int                   `json:"user_id"`
	Username       string                `json:"username"`
	StorageUsed    int64                 `json:"storage_used"`
	BandwidthToday int64                `json:"bandwidth_today"`
	Quota          UserQuotaResponse     `json:"quota"`
	Groups         []UserGroupInfo       `json:"groups"`
	FileCount      int                   `json:"file_count"`
	ShareLinkCount int                   `json:"share_link_count"`
}

// UserGroupInfo is a group membership entry for the user dashboard.
type UserGroupInfo struct {
	GroupID   int    `json:"group_id"`
	GroupName string `json:"group_name"`
	Role      string `json:"role"`
}

// ShareLinkInfo is a summary of an active share link for properties display.
type ShareLinkInfo struct {
	ID            string     `json:"id"`
	CreatedBy     string     `json:"created_by"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	MaxDownloads  int        `json:"max_downloads"`
	DownloadCount int        `json:"download_count"`
	CreatedAt     time.Time  `json:"created_at"`
}

// StorageLocationRequest is the body for POST/PUT /api/v1/admin/storage.
type StorageLocationRequest struct {
	Name        string                 `json:"name"`
	GroupID     *int                   `json:"group_id,omitempty"`
	BackendType string                 `json:"backend_type"`
	Config      map[string]interface{} `json:"config"`
	Priority    int                    `json:"priority"`
}

// StorageLocationResponse is returned by storage admin endpoints.
type StorageLocationResponse struct {
	ID          int                    `json:"id"`
	Name        string                 `json:"name"`
	GroupID     *int                   `json:"group_id,omitempty"`
	BackendType string                 `json:"backend_type"`
	Config      map[string]interface{} `json:"config"`
	Priority    int                    `json:"priority"`
	IsDefault   bool                   `json:"is_default"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// StorageStatsResponse is returned by GET /api/v1/admin/storage/{id}/stats.
type StorageStatsResponse struct {
	LocationID int   `json:"location_id"`
	FileCount  int64 `json:"file_count"`
	TotalSize  int64 `json:"total_size"`
}

// StorageTestResponse is returned by POST /api/v1/admin/storage/{id}/test.
type StorageTestResponse struct {
	Success     bool   `json:"success"`
	BackendType string `json:"backend_type,omitempty"`
	Error       string `json:"error,omitempty"`
}

// ─── Gallery Types ──────────────────────────────────────────────────────────

// GallerySearchResponse is returned by GET /api/v1/gallery/search.
type GallerySearchResponse struct {
	Items      []GalleryItem `json:"items"`
	Total      int           `json:"total"`
	Offset     int           `json:"offset"`
	Limit      int           `json:"limit"`
	HasMore    bool          `json:"has_more"`
}

// GalleryItem represents a single image in gallery search results.
type GalleryItem struct {
	FilePath     string     `json:"file_path"`
	FileName     string     `json:"file_name"`
	Size         int64      `json:"size"`
	ModTime      time.Time  `json:"mod_time"`
	Hash         string     `json:"hash,omitempty"`
	Width        int        `json:"width,omitempty"`
	Height       int        `json:"height,omitempty"`
	CameraMake   string     `json:"camera_make,omitempty"`
	CameraModel  string     `json:"camera_model,omitempty"`
	DateTaken    *time.Time `json:"date_taken,omitempty"`
	Latitude     *float64   `json:"latitude,omitempty"`
	Longitude    *float64   `json:"longitude,omitempty"`
	LocationCity string     `json:"location_city,omitempty"`
	Country      string     `json:"location_country,omitempty"`
	HasThumbnail bool       `json:"has_thumbnail"`
	Tags         []string   `json:"tags,omitempty"`
}

// GalleryMetadataResponse is returned by GET /api/v1/gallery/metadata/{path}.
type GalleryMetadataResponse struct {
	FilePath        string     `json:"file_path"`
	FileName        string     `json:"file_name"`
	Size            int64      `json:"size"`
	Width           int        `json:"width,omitempty"`
	Height          int        `json:"height,omitempty"`
	CameraMake      string     `json:"camera_make,omitempty"`
	CameraModel     string     `json:"camera_model,omitempty"`
	LensModel       string     `json:"lens_model,omitempty"`
	FocalLength     float32    `json:"focal_length,omitempty"`
	Aperture        float32    `json:"aperture,omitempty"`
	ShutterSpeed    string     `json:"shutter_speed,omitempty"`
	ISO             int        `json:"iso,omitempty"`
	Flash           bool       `json:"flash,omitempty"`
	DateTaken       *time.Time `json:"date_taken,omitempty"`
	Latitude        *float64   `json:"latitude,omitempty"`
	Longitude       *float64   `json:"longitude,omitempty"`
	Altitude        *float32   `json:"altitude,omitempty"`
	LocationCountry string     `json:"location_country,omitempty"`
	LocationCity    string     `json:"location_city,omitempty"`
	LocationName    string     `json:"location_name,omitempty"`
	Orientation     int        `json:"orientation"`
	HasThumbnail    bool       `json:"has_thumbnail"`
	Status          string     `json:"status"`
	Tags            []TagInfo  `json:"tags"`
}

// TagInfo describes a tag with its source and confidence.
type TagInfo struct {
	Tag        string  `json:"tag"`
	Confidence float32 `json:"confidence"`
	Source     string  `json:"source"`
}

// DateAlbum groups images by year and month.
type DateAlbum struct {
	Year   int          `json:"year"`
	Months []MonthCount `json:"months"`
}

// MonthCount is a month with an image count.
type MonthCount struct {
	Month int `json:"month"`
	Count int `json:"count"`
}

// LocationAlbum groups images by country and city.
type LocationAlbum struct {
	Country string      `json:"country"`
	Cities  []CityCount `json:"cities"`
}

// CityCount is a city with an image count.
type CityCount struct {
	City  string `json:"city"`
	Count int    `json:"count"`
}

// CameraAlbum groups images by camera make and model.
type CameraAlbum struct {
	Make   string       `json:"make"`
	Models []ModelCount `json:"models"`
}

// ModelCount is a camera model with an image count.
type ModelCount struct {
	Model string `json:"model"`
	Count int    `json:"count"`
}

// TagCountResponse is returned by GET /api/v1/gallery/tags.
type TagCountResponse struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

// GalleryStatsResponse is returned by GET /api/v1/gallery/stats.
type GalleryStatsResponse struct {
	TotalImages int `json:"total_images"`
	WithGPS     int `json:"with_gps"`
	WithTags    int `json:"with_tags"`
	Processed   int `json:"processed"`
	Pending     int `json:"pending"`
}

// MapPointResponse is a minimal representation of a geolocated image for map display.
type MapPointResponse struct {
	FilePath     string     `json:"file_path"`
	FileName     string     `json:"file_name"`
	Latitude     float64    `json:"latitude"`
	Longitude    float64    `json:"longitude"`
	HasThumbnail bool       `json:"has_thumbnail"`
	DateTaken    *time.Time `json:"date_taken,omitempty"`
}

// TagRequest is the body for POST /api/v1/gallery/tags/{path}.
type TagRequest struct {
	Tag string `json:"tag"`
}

// ─── Trash Types ────────────────────────────────────────────────────────────

// TrashItem represents a soft-deleted file in the trash.
type TrashItem struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	OriginalPath  string    `json:"original_path"`
	Size          int64     `json:"size"`
	IsDir         bool      `json:"is_dir"`
	DeletedAt     time.Time `json:"deleted_at"`
	DeletedByName string    `json:"deleted_by_name,omitempty"`
}

// TrashRestoreRequest is the body for POST /api/v1/trash/restore.
type TrashRestoreRequest struct {
	Path string `json:"path"`
}

// ─── Favorites Types ────────────────────────────────────────────────────────

// FavoriteItem represents a user's bookmarked file.
type FavoriteItem struct {
	FilePath string    `json:"file_path"`
	FileName string    `json:"file_name"`
	Size     int64     `json:"size"`
	IsDir    bool      `json:"is_dir"`
	ModTime  time.Time `json:"mod_time,omitempty"`
}

// ─── Search Types ───────────────────────────────────────────────────────────

// SearchResult represents a file search result.
type SearchResult struct {
	ID      string    `json:"id"`
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	Size    int64     `json:"size"`
	IsDir   bool      `json:"is_dir"`
	ModTime time.Time `json:"mod_time"`
	Tags    []string  `json:"tags,omitempty"`
}

// ─── Bulk Operation Types ───────────────────────────────────────────────────

// BulkMoveRequest is the body for POST /api/v1/bulk/move.
type BulkMoveRequest struct {
	Paths       []string `json:"paths"`
	Destination string   `json:"destination"`
}

// BulkCopyRequest is the body for POST /api/v1/bulk/copy.
type BulkCopyRequest struct {
	Paths       []string `json:"paths"`
	Destination string   `json:"destination"`
}

// BulkShareRequest is the body for POST /api/v1/bulk/share.
type BulkShareRequest struct {
	Paths        []string `json:"paths"`
	Password     string   `json:"password,omitempty"`
	ExpiresInSec int64    `json:"expires_in_sec,omitempty"`
	MaxDownloads int      `json:"max_downloads,omitempty"`
}

// BulkTagRequest is the body for POST /api/v1/bulk/tag.
type BulkTagRequest struct {
	Paths []string `json:"paths"`
	Tags  []string `json:"tags"`
}

// BulkResponse is the response for bulk operations.
type BulkResponse struct {
	Succeeded int      `json:"succeeded"`
	Failed    int      `json:"failed"`
	Errors    []string `json:"errors,omitempty"`
}

// AlbumRequest is the body for POST/PUT /api/v1/gallery/albums.
type AlbumRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// AlbumResponse is returned by album endpoints.
type AlbumResponse struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CoverPath   *string   `json:"cover_path,omitempty"`
	ImageCount  int       `json:"image_count"`
	CreatedAt   time.Time `json:"created_at"`
}

// AlbumCoverRequest is the body for PUT /api/v1/gallery/albums/{id}/cover.
type AlbumCoverRequest struct {
	CoverPath string `json:"cover_path"`
}

// AlbumImageRequest is the body for POST/DELETE /api/v1/gallery/albums/{id}/images.
type AlbumImageRequest struct {
	FilePath string `json:"file_path"`
}

// GlobalTagActionRequest is the body for PUT /api/v1/admin/gallery/tags/{tag}.
type GlobalTagActionRequest struct {
	NewTag string `json:"new_tag"`
}

// PluginRequest is the body for POST/PUT /api/v1/admin/gallery/plugins.
type PluginRequest struct {
	Name       string                 `json:"name"`
	WebhookURL string                `json:"webhook_url"`
	Enabled    bool                   `json:"enabled"`
	Config     map[string]interface{} `json:"config,omitempty"`
}

// PluginResponse is returned by plugin admin endpoints.
type PluginResponse struct {
	ID         int                    `json:"id"`
	Name       string                 `json:"name"`
	WebhookURL string                `json:"webhook_url"`
	Enabled    bool                   `json:"enabled"`
	Config     map[string]interface{} `json:"config,omitempty"`
	LastHealth *time.Time             `json:"last_health,omitempty"`
	LastError  string                 `json:"last_error,omitempty"`
	CreatedAt  time.Time              `json:"created_at"`
	UpdatedAt  time.Time              `json:"updated_at"`
}
