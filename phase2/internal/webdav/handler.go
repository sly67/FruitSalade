package webdav

import (
	"net/http"

	"golang.org/x/net/webdav"

	"github.com/fruitsalade/fruitsalade/phase2/internal/auth"
	s3storage "github.com/fruitsalade/fruitsalade/phase2/internal/storage/s3"
)

// NewHandler creates a WebDAV HTTP handler with authentication.
func NewHandler(storage *s3storage.Storage, authHandler *auth.Auth) http.Handler {
	davHandler := &webdav.Handler{
		FileSystem: &FruitFS{storage: storage},
		LockSystem: webdav.NewMemLS(),
		Prefix:     "/webdav",
	}
	return BasicAuthMiddleware(authHandler)(davHandler)
}
