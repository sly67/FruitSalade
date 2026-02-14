package sharing

import (
	"testing"
)

func TestRoleToPermission(t *testing.T) {
	tests := []struct {
		role     string
		expected string
	}{
		{"admin", "write"},
		{"editor", "write"},
		{"viewer", "read"},
		{"", ""},
		{"unknown", ""},
	}
	for _, tt := range tests {
		got := RoleToPermission(tt.role)
		if got != tt.expected {
			t.Errorf("RoleToPermission(%q) = %q, want %q", tt.role, got, tt.expected)
		}
	}
}

func TestRoleLevel(t *testing.T) {
	tests := []struct {
		role     string
		expected int
	}{
		{"admin", 3},
		{"editor", 2},
		{"viewer", 1},
		{"", 0},
		{"unknown", 0},
	}
	for _, tt := range tests {
		got := roleLevel(tt.role)
		if got != tt.expected {
			t.Errorf("roleLevel(%q) = %d, want %d", tt.role, got, tt.expected)
		}
	}
}

func TestRoleLevelOrdering(t *testing.T) {
	if roleLevel("viewer") >= roleLevel("editor") {
		t.Error("viewer should be less than editor")
	}
	if roleLevel("editor") >= roleLevel("admin") {
		t.Error("editor should be less than admin")
	}
}
