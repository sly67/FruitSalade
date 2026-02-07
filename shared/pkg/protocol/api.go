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
