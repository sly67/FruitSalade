package sharing

import (
	"testing"
)

func TestPathSegments(t *testing.T) {
	tests := []struct {
		path     string
		expected []string
	}{
		{"/a/b/c", []string{"/a/b/c", "/a/b", "/a", "/"}},
		{"/docs", []string{"/docs", "/"}},
		{"/", []string{"/"}},
		{"/a/b/c/d/e", []string{"/a/b/c/d/e", "/a/b/c/d", "/a/b/c", "/a/b", "/a", "/"}},
	}

	for _, tt := range tests {
		result := pathSegments(tt.path)
		if len(result) != len(tt.expected) {
			t.Errorf("pathSegments(%s): got %d segments, want %d: %v", tt.path, len(result), len(tt.expected), result)
			continue
		}
		for i, seg := range result {
			if seg != tt.expected[i] {
				t.Errorf("pathSegments(%s)[%d]: got %s, want %s", tt.path, i, seg, tt.expected[i])
			}
		}
	}
}

func TestPermissionSatisfies(t *testing.T) {
	tests := []struct {
		has      string
		required string
		expect   bool
	}{
		{"owner", "read", true},
		{"owner", "write", true},
		{"owner", "owner", true},
		{"write", "read", true},
		{"write", "write", true},
		{"write", "owner", false},
		{"read", "read", true},
		{"read", "write", false},
		{"read", "owner", false},
	}

	for _, tt := range tests {
		result := permissionSatisfies(tt.has, tt.required)
		if result != tt.expect {
			t.Errorf("permissionSatisfies(%s, %s) = %v, want %v", tt.has, tt.required, result, tt.expect)
		}
	}
}
