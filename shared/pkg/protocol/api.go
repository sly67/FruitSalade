// Package protocol defines the API request/response types.
package protocol

import "github.com/fruitsalade/fruitsalade/shared/pkg/models"

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
