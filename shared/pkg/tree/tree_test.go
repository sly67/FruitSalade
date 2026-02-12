package tree

import (
	"testing"

	"github.com/fruitsalade/fruitsalade/shared/pkg/models"
)

func TestFindByPath(t *testing.T) {
	root := &models.FileNode{
		Path: "/", IsDir: true,
		Children: []*models.FileNode{
			{Path: "/a.txt", Name: "a.txt"},
			{Path: "/dir", Name: "dir", IsDir: true, Children: []*models.FileNode{
				{Path: "/dir/b.txt", Name: "b.txt"},
			}},
		},
	}

	tests := []struct {
		path  string
		found bool
	}{
		{"/", true},
		{"/a.txt", true},
		{"/dir", true},
		{"/dir/b.txt", true},
		{"/nonexistent", false},
	}

	for _, tt := range tests {
		node := FindByPath(root, tt.path)
		if (node != nil) != tt.found {
			t.Errorf("FindByPath(%q) found=%v, want %v", tt.path, node != nil, tt.found)
		}
		if node != nil && node.Path != tt.path {
			t.Errorf("FindByPath(%q).Path = %q", tt.path, node.Path)
		}
	}

	if FindByPath(nil, "/") != nil {
		t.Error("FindByPath(nil, /) should return nil")
	}
}

func TestFindByID(t *testing.T) {
	root := &models.FileNode{
		ID: "root", Path: "/", IsDir: true,
		Children: []*models.FileNode{
			{ID: "id-a", Path: "/a.txt", Name: "a.txt"},
		},
	}

	if node := FindByID(root, "id-a"); node == nil || node.Path != "/a.txt" {
		t.Errorf("FindByID(id-a) failed")
	}
	if node := FindByID(root, "nonexistent"); node != nil {
		t.Errorf("FindByID(nonexistent) should return nil")
	}
	if FindByID(nil, "x") != nil {
		t.Error("FindByID(nil, x) should return nil")
	}
}

func TestCacheID(t *testing.T) {
	tests := []struct {
		id, want string
	}{
		{"/path/to/file.txt", "_path_to_file.txt"},
		{"plain", "plain"},
		{"", ""},
	}
	for _, tt := range tests {
		got := CacheID(tt.id)
		if got != tt.want {
			t.Errorf("CacheID(%q) = %q, want %q", tt.id, got, tt.want)
		}
	}
}

func TestCountNodes(t *testing.T) {
	root := &models.FileNode{
		Path: "/", IsDir: true,
		Children: []*models.FileNode{
			{Path: "/a.txt", Name: "a.txt"},
			{Path: "/dir", Name: "dir", IsDir: true, Children: []*models.FileNode{
				{Path: "/dir/b.txt", Name: "b.txt"},
			}},
		},
	}
	if got := CountNodes(root); got != 4 {
		t.Errorf("CountNodes = %d, want 4", got)
	}
	if got := CountNodes(nil); got != 0 {
		t.Errorf("CountNodes(nil) = %d, want 0", got)
	}
}

func TestRemoveChild(t *testing.T) {
	parent := &models.FileNode{
		Path: "/", IsDir: true,
		Children: []*models.FileNode{
			{Name: "a", Path: "/a"},
			{Name: "b", Path: "/b"},
			{Name: "c", Path: "/c"},
		},
	}

	RemoveChild(parent, "b")
	if len(parent.Children) != 2 {
		t.Errorf("got %d children, want 2", len(parent.Children))
	}
	if parent.Children[0].Name != "a" || parent.Children[1].Name != "c" {
		t.Error("unexpected children after remove")
	}

	// Remove nonexistent: no-op
	RemoveChild(parent, "z")
	if len(parent.Children) != 2 {
		t.Errorf("remove nonexistent changed count: %d", len(parent.Children))
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
		got := BuildChildPath(tt.parent, tt.name)
		if got != tt.want {
			t.Errorf("BuildChildPath(%q, %q) = %q, want %q", tt.parent, tt.name, got, tt.want)
		}
	}
}

func TestFlatten(t *testing.T) {
	root := &models.FileNode{
		Path: "/", IsDir: true,
		Children: []*models.FileNode{
			{Path: "/a.txt", Name: "a.txt"},
			{Path: "/dir", Name: "dir", IsDir: true, Children: []*models.FileNode{
				{Path: "/dir/b.txt", Name: "b.txt"},
			}},
		},
	}

	flat := Flatten(root)
	if len(flat) != 4 {
		t.Errorf("Flatten returned %d nodes, want 4", len(flat))
	}
	for _, path := range []string{"/", "/a.txt", "/dir", "/dir/b.txt"} {
		if _, ok := flat[path]; !ok {
			t.Errorf("Flatten missing path %q", path)
		}
	}

	// Nil tree
	if len(Flatten(nil)) != 0 {
		t.Error("Flatten(nil) should return empty map")
	}
}
