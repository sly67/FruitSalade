package sharing

import (
	"testing"

	"github.com/fruitsalade/fruitsalade/shared/pkg/models"
)

func TestPathSegmentsExtended(t *testing.T) {
	tests := []struct {
		path     string
		expected []string
	}{
		{"/docs/sub/file.txt", []string{"/docs/sub/file.txt", "/docs/sub", "/docs", "/"}},
		{"/a", []string{"/a", "/"}},
	}
	for _, tt := range tests {
		got := PathSegments(tt.path)
		if len(got) != len(tt.expected) {
			t.Errorf("PathSegments(%q) = %v (len %d), want %v (len %d)", tt.path, got, len(got), tt.expected, len(tt.expected))
			continue
		}
		for i := range got {
			if got[i] != tt.expected[i] {
				t.Errorf("PathSegments(%q)[%d] = %q, want %q", tt.path, i, got[i], tt.expected[i])
			}
		}
	}
}

func TestPermissionSatisfiesExtended(t *testing.T) {
	tests := []struct {
		has      string
		required string
		expected bool
	}{
		// Edge cases not covered in sharing_test.go
		{"", "read", false},
		{"read", "", true},
		{"", "", true},
		{"unknown", "read", false},
		{"read", "unknown", true}, // levels["unknown"]=0, levels["read"]=1, so 1>=0
	}
	for _, tt := range tests {
		got := PermissionSatisfies(tt.has, tt.required)
		if got != tt.expected {
			t.Errorf("PermissionSatisfies(%q, %q) = %v, want %v", tt.has, tt.required, got, tt.expected)
		}
	}
}

func TestCheckVisibility(t *testing.T) {
	store := &PermissionStore{}

	// Public node - everyone can see
	publicNode := &models.FileNode{Visibility: "public", OwnerID: 1}
	if !store.CheckVisibility(publicNode, 2, false, nil) {
		t.Error("public node should be visible to everyone")
	}

	// Empty visibility defaults to public
	emptyVisNode := &models.FileNode{Visibility: "", OwnerID: 1}
	if !store.CheckVisibility(emptyVisNode, 2, false, nil) {
		t.Error("empty visibility should default to public")
	}

	// Private node - only owner can see
	privateNode := &models.FileNode{Visibility: "private", OwnerID: 1}
	if !store.CheckVisibility(privateNode, 1, false, nil) {
		t.Error("private node should be visible to owner")
	}
	if store.CheckVisibility(privateNode, 2, false, nil) {
		t.Error("private node should not be visible to non-owner")
	}

	// Private node - admin override
	if !store.CheckVisibility(privateNode, 2, true, nil) {
		t.Error("private node should be visible to admin")
	}

	// Group node - only group members can see
	groupNode := &models.FileNode{Visibility: "group", GroupID: 5, OwnerID: 1}
	memberGroups := map[int]string{5: "viewer"}
	nonMemberGroups := map[int]string{99: "viewer"}

	if !store.CheckVisibility(groupNode, 2, false, memberGroups) {
		t.Error("group node should be visible to group members")
	}
	if store.CheckVisibility(groupNode, 2, false, nonMemberGroups) {
		t.Error("group node should not be visible to non-members")
	}
	if store.CheckVisibility(groupNode, 2, false, nil) {
		t.Error("group node should not be visible without group membership")
	}

	// Group node with no group_id set - treat as public
	groupNoIDNode := &models.FileNode{Visibility: "group", GroupID: 0}
	if !store.CheckVisibility(groupNoIDNode, 2, false, nil) {
		t.Error("group node with no group_id should be treated as public")
	}

	// Admin sees everything
	if !store.CheckVisibility(groupNode, 99, true, nil) {
		t.Error("admin should see group node regardless of membership")
	}
}
