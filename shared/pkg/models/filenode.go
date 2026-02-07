// Package models contains shared data types used across all phases.
package models

import "time"

// FileNode represents a file or directory in the virtual filesystem.
type FileNode struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Path     string      `json:"path"`
	Size     int64       `json:"size"`
	ModTime  time.Time   `json:"mtime"`
	IsDir    bool        `json:"is_dir"`
	Hash     string      `json:"hash,omitempty"`
	Children []*FileNode `json:"children,omitempty"`
}

// CacheEntry represents a cached file on the client.
type CacheEntry struct {
	FileID     string    `json:"file_id"`
	LocalPath  string    `json:"local_path"`
	Size       int64     `json:"size"`
	LastAccess time.Time `json:"last_access"`
	Pinned     bool      `json:"pinned"`
}
