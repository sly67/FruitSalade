package winclient

import (
	"testing"
	"time"

	"github.com/fruitsalade/fruitsalade/shared/pkg/models"
	"github.com/fruitsalade/fruitsalade/shared/pkg/tree"
)

func TestDiffMetadata(t *testing.T) {
	now := time.Now()

	old := &models.FileNode{
		Path: "/", IsDir: true,
		Children: []*models.FileNode{
			{Path: "/a.txt", Name: "a.txt", Size: 100, Hash: "aaa", ModTime: now},
			{Path: "/b.txt", Name: "b.txt", Size: 200, Hash: "bbb", ModTime: now},
			{Path: "/c.txt", Name: "c.txt", Size: 300, Hash: "ccc", ModTime: now},
		},
	}

	new := &models.FileNode{
		Path: "/", IsDir: true,
		Children: []*models.FileNode{
			{Path: "/a.txt", Name: "a.txt", Size: 100, Hash: "aaa", ModTime: now}, // unchanged
			{Path: "/b.txt", Name: "b.txt", Size: 250, Hash: "bbb2", ModTime: now.Add(time.Second)}, // changed
			// c.txt removed
			{Path: "/d.txt", Name: "d.txt", Size: 400, Hash: "ddd", ModTime: now}, // added
		},
	}

	diff := DiffMetadata(old, new)

	if len(diff.Added) != 1 || diff.Added[0].Path != "/d.txt" {
		t.Errorf("Added = %v, want [/d.txt]", pathsOf(diff.Added))
	}
	if len(diff.Removed) != 1 || diff.Removed[0].Path != "/c.txt" {
		t.Errorf("Removed = %v, want [/c.txt]", pathsOf(diff.Removed))
	}
	if len(diff.Changed) != 1 || diff.Changed[0].Path != "/b.txt" {
		t.Errorf("Changed = %v, want [/b.txt]", pathsOf(diff.Changed))
	}
}

func TestDiffMetadataNilTrees(t *testing.T) {
	node := &models.FileNode{Path: "/", IsDir: true}

	diff := DiffMetadata(nil, node)
	if len(diff.Added) != 1 {
		t.Errorf("nil->node: Added = %d, want 1", len(diff.Added))
	}

	diff = DiffMetadata(node, nil)
	if len(diff.Removed) != 1 {
		t.Errorf("node->nil: Removed = %d, want 1", len(diff.Removed))
	}

	diff = DiffMetadata(nil, nil)
	if len(diff.Added)+len(diff.Removed)+len(diff.Changed) != 0 {
		t.Error("nil->nil: expected empty diff")
	}
}

func TestBuildChildPath(t *testing.T) {
	tests := []struct {
		parent, name, want string
	}{
		{"/", "file.txt", "/file.txt"},
		{"/dir", "file.txt", "/dir/file.txt"},
		{"/a/b", "c", "/a/b/c"},
	}
	for _, tt := range tests {
		got := tree.BuildChildPath(tt.parent, tt.name)
		if got != tt.want {
			t.Errorf("BuildChildPath(%q, %q) = %q, want %q", tt.parent, tt.name, got, tt.want)
		}
	}
}

func TestNewClientCore(t *testing.T) {
	dir := t.TempDir()
	cfg := CoreConfig{
		ServerURL:    "http://localhost:48000",
		CacheDir:     dir,
		MaxCacheSize: 1 << 20,
	}

	core, err := NewClientCore(cfg)
	if err != nil {
		t.Fatalf("NewClientCore: %v", err)
	}
	if core.Client == nil {
		t.Error("Client is nil")
	}
	if core.Cache == nil {
		t.Error("Cache is nil")
	}
	if core.SSEClient != nil {
		t.Error("SSEClient should be nil when WatchSSE=false")
	}
}

func TestNewClientCoreWithSSE(t *testing.T) {
	dir := t.TempDir()
	cfg := CoreConfig{
		ServerURL:    "http://localhost:48000",
		AuthToken:    "test-token",
		CacheDir:     dir,
		MaxCacheSize: 1 << 20,
		WatchSSE:     true,
	}

	core, err := NewClientCore(cfg)
	if err != nil {
		t.Fatalf("NewClientCore: %v", err)
	}
	if core.SSEClient == nil {
		t.Error("SSEClient should not be nil when WatchSSE=true")
	}
}

func TestMetadataMethods(t *testing.T) {
	dir := t.TempDir()
	cfg := CoreConfig{
		ServerURL:    "http://localhost:48000",
		CacheDir:     dir,
		MaxCacheSize: 1 << 20,
	}

	core, err := NewClientCore(cfg)
	if err != nil {
		t.Fatalf("NewClientCore: %v", err)
	}

	// Initially nil
	if core.Metadata() != nil {
		t.Error("expected nil metadata initially")
	}
	if core.FindByPath("/") != nil {
		t.Error("FindByPath on nil tree should return nil")
	}

	// Set metadata directly for testing
	root := &models.FileNode{
		Path: "/", IsDir: true,
		Children: []*models.FileNode{
			{Path: "/docs", Name: "docs", IsDir: true},
		},
	}
	core.mu.Lock()
	core.metadata = root
	core.mu.Unlock()

	if node := core.FindByPath("/docs"); node == nil {
		t.Error("FindByPath(/docs) should find node")
	}

	// Test AddMetadataChild
	child := &models.FileNode{Path: "/docs/readme.txt", Name: "readme.txt"}
	core.AddMetadataChild("/docs", child)
	if node := core.FindByPath("/docs/readme.txt"); node == nil {
		t.Error("AddMetadataChild: node not found")
	}

	// Test UpdateMetadataNode
	now := time.Now()
	core.UpdateMetadataNode("/docs/readme.txt", 999, "newhash", now, 1)
	if node := core.FindByPath("/docs/readme.txt"); node.Size != 999 || node.Hash != "newhash" {
		t.Errorf("UpdateMetadataNode: size=%d hash=%s", node.Size, node.Hash)
	}

	// Test RemoveMetadataChild
	core.RemoveMetadataChild("/docs", "readme.txt")
	if node := core.FindByPath("/docs/readme.txt"); node != nil {
		t.Error("RemoveMetadataChild: node still found")
	}
}

func pathsOf(nodes []*models.FileNode) []string {
	var paths []string
	for _, n := range nodes {
		paths = append(paths, n.Path)
	}
	return paths
}
