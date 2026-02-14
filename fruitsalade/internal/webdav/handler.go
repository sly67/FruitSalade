package webdav

import (
	"net/http"

	"golang.org/x/net/webdav"

	"github.com/fruitsalade/fruitsalade/fruitsalade/internal/auth"
	"github.com/fruitsalade/fruitsalade/fruitsalade/internal/metadata/postgres"
	"github.com/fruitsalade/fruitsalade/fruitsalade/internal/storage"
)

// NewHandler creates a WebDAV HTTP handler with authentication.
func NewHandler(metadata *postgres.Store, storageRouter *storage.Router, authHandler *auth.Auth) http.Handler {
	davHandler := &webdav.Handler{
		FileSystem: &FruitFS{metadata: metadata, storageRouter: storageRouter},
		LockSystem: webdav.NewMemLS(),
		Prefix:     "/webdav",
	}
	return BasicAuthMiddleware(authHandler)(davHandler)
}
