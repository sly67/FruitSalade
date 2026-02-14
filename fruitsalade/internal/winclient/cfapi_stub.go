//go:build !windows

package winclient

import (
	"context"
	"fmt"
)

// CfAPIBackend is a stub for non-Windows platforms.
type CfAPIBackend struct {
	syncRoot string
}

// NewCfAPIBackend creates a CfAPI backend stub.
func NewCfAPIBackend(syncRoot string) *CfAPIBackend {
	return &CfAPIBackend{syncRoot: syncRoot}
}

func (b *CfAPIBackend) Name() string {
	return "cfapi"
}

func (b *CfAPIBackend) Start(ctx context.Context, core *ClientCore) error {
	return fmt.Errorf("CfAPI is only available on Windows")
}

func (b *CfAPIBackend) Stop() error {
	return fmt.Errorf("CfAPI is only available on Windows")
}
