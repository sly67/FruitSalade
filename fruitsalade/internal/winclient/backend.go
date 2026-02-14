package winclient

import "context"

// Backend is the interface that CfAPI and cgofuse backends implement.
type Backend interface {
	// Start starts the backend (mounts FUSE or registers CfAPI sync root).
	// It blocks until ctx is cancelled or an error occurs.
	Start(ctx context.Context, core *ClientCore) error

	// Stop cleanly shuts down the backend.
	Stop() error

	// Name returns a human-readable name for the backend.
	Name() string
}
