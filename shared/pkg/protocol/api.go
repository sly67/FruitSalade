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
