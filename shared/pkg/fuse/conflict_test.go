package fuse

import (
	"strings"
	"testing"
	"time"
)

func TestConflictCopyPath(t *testing.T) {
	// Use a fixed date for reproducible tests
	stamp := time.Now().Format("2006-01-02")

	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "normal file",
			path: "/docs/report.txt",
			want: "/docs/report (conflict " + stamp + ").txt",
		},
		{
			name: "no extension",
			path: "/docs/README",
			want: "/docs/README (conflict " + stamp + ")",
		},
		{
			name: "nested directory",
			path: "/a/b/c/file.pdf",
			want: "/a/b/c/file (conflict " + stamp + ").pdf",
		},
		{
			name: "root file",
			path: "/test.txt",
			want: "/test (conflict " + stamp + ").txt",
		},
		{
			name: "double extension",
			path: "/data/archive.tar.gz",
			want: "/data/archive.tar (conflict " + stamp + ").gz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := conflictCopyPath(tt.path)
			// Normalize path separators for cross-platform testing
			got = strings.ReplaceAll(got, "\\", "/")
			if got != tt.want {
				t.Errorf("conflictCopyPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}
