//go:build windows

package winclient

/*
#cgo CFLAGS: -I${SRCDIR}/../../../windows
#cgo LDFLAGS: -lcldapi -lole32

#include "cfapi_shim.h"
*/
import "C"

import (
	"context"
	"fmt"
	"os"
	"sync"
	"unsafe"

	"github.com/fruitsalade/fruitsalade/shared/pkg/logger"
	"github.com/fruitsalade/fruitsalade/shared/pkg/models"
)

// CfAPIBackend implements Backend using Windows Cloud Files API.
type CfAPIBackend struct {
	syncRoot string
	core     *ClientCore
	connKey  C.CF_CONNECTION_KEY

	mu        sync.Mutex
	connected bool
}

// NewCfAPIBackend creates a CfAPI backend.
func NewCfAPIBackend(syncRoot string) *CfAPIBackend {
	return &CfAPIBackend{syncRoot: syncRoot}
}

func (b *CfAPIBackend) Name() string {
	return "cfapi"
}

// globalCfAPIBackend is used by the CGO callback to route hydration requests.
var globalCfAPIBackend *CfAPIBackend

func (b *CfAPIBackend) Start(ctx context.Context, core *ClientCore) error {
	b.core = core
	globalCfAPIBackend = b

	if err := os.MkdirAll(b.syncRoot, 0755); err != nil {
		return fmt.Errorf("create sync root: %w", err)
	}

	// Initialize CfAPI
	hr := C.cfapi_init()
	if hr != 0 {
		return fmt.Errorf("cfapi_init failed: HRESULT 0x%08x", uint32(hr))
	}

	// Register sync root
	cRoot := C.CString(b.syncRoot)
	defer C.free(unsafe.Pointer(cRoot))
	cName := C.CString("FruitSalade")
	defer C.free(unsafe.Pointer(cName))
	cVersion := C.CString("2.0")
	defer C.free(unsafe.Pointer(cVersion))

	hr = C.cfapi_register_sync_root(cRoot, cName, cVersion)
	if hr != 0 {
		return fmt.Errorf("cfapi_register_sync_root failed: HRESULT 0x%08x", uint32(hr))
	}

	// Connect sync root
	hr = C.cfapi_connect_sync_root(cRoot, &b.connKey)
	if hr != 0 {
		return fmt.Errorf("cfapi_connect_sync_root failed: HRESULT 0x%08x", uint32(hr))
	}
	b.connected = true

	// Fetch metadata and create placeholders
	if err := core.FetchMetadata(ctx); err != nil {
		return fmt.Errorf("initial metadata fetch: %w", err)
	}

	tree := core.Metadata()
	if tree != nil {
		b.createPlaceholdersRecursive(tree, b.syncRoot)
	}

	core.StartBackgroundLoops(ctx)

	// Watch for metadata changes in background
	go b.watchMetadataChanges(ctx)

	logger.Info("CfAPI backend started at %s", b.syncRoot)

	// Block until context is cancelled
	<-ctx.Done()

	return b.Stop()
}

func (b *CfAPIBackend) Stop() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.core != nil {
		b.core.StopBackgroundLoops()
	}

	if b.connected {
		cRoot := C.CString(b.syncRoot)
		defer C.free(unsafe.Pointer(cRoot))
		C.cfapi_disconnect_sync_root(b.connKey)
		b.connected = false
	}

	return nil
}

func (b *CfAPIBackend) createPlaceholdersRecursive(node *models.FileNode, localDir string) {
	for _, child := range node.Children {
		localPath := localDir + string(os.PathSeparator) + child.Name

		cPath := C.CString(localDir)
		cName := C.CString(child.Name)
		cID := C.CString(child.ID)
		isDir := C.int(0)
		if child.IsDir {
			isDir = 1
		}

		C.cfapi_create_placeholder(cPath, cName, cID,
			C.longlong(child.Size), C.longlong(child.ModTime.Unix()), isDir)

		C.free(unsafe.Pointer(cPath))
		C.free(unsafe.Pointer(cName))
		C.free(unsafe.Pointer(cID))

		if child.IsDir {
			os.MkdirAll(localPath, 0755)
			b.createPlaceholdersRecursive(child, localPath)
		}
	}
}

func (b *CfAPIBackend) watchMetadataChanges(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// The refresh loop already updates metadata. We periodically check for diffs
		// and update placeholders. In practice, SSE or refresh loop triggers RefreshMetadata
		// which returns a diff, but we keep this simple polling approach for robustness.
		diff, err := b.core.RefreshMetadata(ctx)
		if err != nil || diff == nil {
			select {
			case <-ctx.Done():
				return
			default:
			}
			continue
		}

		for _, node := range diff.Added {
			dir := b.syncRoot + string(os.PathSeparator) + dirOf(node.Path)
			b.createPlaceholderSingle(dir, node)
		}

		for _, node := range diff.Changed {
			localPath := b.syncRoot + string(os.PathSeparator) + node.Path
			cPath := C.CString(localPath)
			cID := C.CString(node.ID)
			C.cfapi_update_placeholder(cPath, cID,
				C.longlong(node.Size), C.longlong(node.ModTime.Unix()))
			C.free(unsafe.Pointer(cPath))
			C.free(unsafe.Pointer(cID))
		}

		// Removed files: the OS handles deletion via the sync root
	}
}

func (b *CfAPIBackend) createPlaceholderSingle(localDir string, node *models.FileNode) {
	cPath := C.CString(localDir)
	cName := C.CString(node.Name)
	cID := C.CString(node.ID)
	isDir := C.int(0)
	if node.IsDir {
		isDir = 1
	}
	C.cfapi_create_placeholder(cPath, cName, cID,
		C.longlong(node.Size), C.longlong(node.ModTime.Unix()), isDir)
	C.free(unsafe.Pointer(cPath))
	C.free(unsafe.Pointer(cName))
	C.free(unsafe.Pointer(cID))
}

func dirOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			if i == 0 {
				return ""
			}
			return path[1:i] // strip leading /
		}
	}
	return ""
}

//export goHydrationCallback
func goHydrationCallback(fileIdentity *C.char, fileIdentityLen C.int,
	offset C.longlong, length C.longlong,
	transferKey C.CF_TRANSFER_KEY) {

	if globalCfAPIBackend == nil || globalCfAPIBackend.core == nil {
		C.cfapi_transfer_error(globalCfAPIBackend.connKey, transferKey,
			C.longlong(offset), C.long(0x80004005)) // E_FAIL
		return
	}

	fileID := C.GoStringN(fileIdentity, fileIdentityLen)
	ctx := context.Background()

	data, err := globalCfAPIBackend.core.FetchContentRange(ctx, fileID, int64(offset), int64(length))
	if err != nil {
		logger.Error("Hydration failed for %s: %v", fileID, err)
		C.cfapi_transfer_error(globalCfAPIBackend.connKey, transferKey,
			C.longlong(offset), C.long(0x80004005))
		return
	}

	C.cfapi_transfer_data(globalCfAPIBackend.connKey, transferKey,
		unsafe.Pointer(&data[0]), C.longlong(offset), C.longlong(len(data)))
}
