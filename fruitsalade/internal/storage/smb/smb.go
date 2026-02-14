// Package smb provides an SMB/CIFS network share storage backend.
// The SMB share must be pre-mounted on the OS (via mount.cifs or fstab).
// This backend delegates to the local filesystem backend at the mount path.
package smb

import (
	"encoding/json"
	"fmt"

	"github.com/fruitsalade/fruitsalade/fruitsalade/internal/storage/local"
)

// Config holds SMB backend settings.
// Server/Username/Password/Domain are stored for admin reference and documentation.
// Actual I/O uses the MountPath where the share is pre-mounted.
type Config struct {
	Server    string `json:"server"`     // SMB server path (e.g., //server/share)
	Username  string `json:"username"`   // SMB credentials
	Password  string `json:"password"`   // SMB password
	Domain    string `json:"domain"`     // SMB domain
	MountPath string `json:"mount_path"` // Local mount point where share is mounted
}

// SMBBackend wraps a LocalBackend at the SMB mount point.
type SMBBackend struct {
	*local.LocalBackend
	config Config
}

// New creates a new SMB backend from the given config.
func New(cfg Config) (*SMBBackend, error) {
	if cfg.MountPath == "" {
		return nil, fmt.Errorf("mount_path is required")
	}

	lb, err := local.New(local.Config{
		RootPath:   cfg.MountPath,
		CreateDirs: true,
	})
	if err != nil {
		return nil, fmt.Errorf("smb backend at %s: %w", cfg.MountPath, err)
	}

	return &SMBBackend{
		LocalBackend: lb,
		config:       cfg,
	}, nil
}

// NewFromJSON creates an SMBBackend from raw JSON config.
func NewFromJSON(raw json.RawMessage) (*SMBBackend, error) {
	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parse smb config: %w", err)
	}
	return New(cfg)
}

// Type returns "smb".
func (b *SMBBackend) Type() string { return "smb" }
