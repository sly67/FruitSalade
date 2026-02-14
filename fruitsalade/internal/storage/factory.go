package storage

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/fruitsalade/fruitsalade/fruitsalade/internal/storage/local"
	s3backend "github.com/fruitsalade/fruitsalade/fruitsalade/internal/storage/s3"
	"github.com/fruitsalade/fruitsalade/fruitsalade/internal/storage/smb"
)

// NewBackendFromConfig creates a Backend from a backend type string and JSON config.
func NewBackendFromConfig(ctx context.Context, backendType string, config json.RawMessage) (Backend, error) {
	switch backendType {
	case "s3":
		return s3backend.NewBackendFromJSON(ctx, config)
	case "local":
		return local.NewFromJSON(config)
	case "smb":
		return smb.NewFromJSON(config)
	default:
		return nil, fmt.Errorf("unknown backend type: %s", backendType)
	}
}
