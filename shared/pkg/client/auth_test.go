package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fruitsalade/fruitsalade/shared/pkg/retry"
)

func testAuthClient(handler http.Handler) (*Client, *httptest.Server) {
	ts := httptest.NewServer(handler)
	c := New(Config{
		BaseURL:     ts.URL,
		RetryConfig: retry.Config{MaxAttempts: 1, InitialWait: time.Millisecond, MaxWait: time.Millisecond},
	})
	return c, ts
}

func TestLogin_Success(t *testing.T) {
	c, ts := testAuthClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/api/v1/auth/token" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var req map[string]string
		json.NewDecoder(r.Body).Decode(&req)
		if req["username"] != "alice" {
			t.Errorf("expected username alice, got %s", req["username"])
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"token":      "jwt-token-123",
			"expires_at": time.Now().Add(24 * time.Hour),
			"user": map[string]interface{}{
				"id": 1, "username": "alice", "is_admin": false,
			},
		})
	}))
	defer ts.Close()

	resp, err := c.Login(context.Background(), "alice", "pass123", "test-device")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Token != "jwt-token-123" {
		t.Errorf("expected token jwt-token-123, got %s", resp.Token)
	}
	if resp.User.Username != "alice" {
		t.Errorf("expected user alice, got %s", resp.User.Username)
	}
}

func TestLogin_Failure(t *testing.T) {
	c, ts := testAuthClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid credentials"}`))
	}))
	defer ts.Close()

	_, err := c.Login(context.Background(), "alice", "wrong", "device")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("expected 401 in error, got: %v", err)
	}
}

func TestRefreshToken_Success(t *testing.T) {
	c, ts := testAuthClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/api/v1/auth/refresh" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			t.Errorf("expected Bearer auth, got %q", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"token":      "new-jwt-token",
			"expires_at": time.Now().Add(30 * 24 * time.Hour),
		})
	}))
	defer ts.Close()

	c.SetAuthToken("old-token")
	resp, err := c.RefreshToken(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Token != "new-jwt-token" {
		t.Errorf("expected new-jwt-token, got %s", resp.Token)
	}
}

func TestTokenFile_SaveLoadRoundTrip(t *testing.T) {
	// Use a temp dir to avoid interfering with real token file
	tmpDir := t.TempDir()
	tokenPath := filepath.Join(tmpDir, "token.json")

	original := &TokenFile{
		Token:     "test-token-abc",
		ExpiresAt: time.Now().Add(24 * time.Hour).Truncate(time.Millisecond),
		Server:    "http://localhost:8080",
		Username:  "testuser",
	}

	// Write manually to temp path
	data, err := json.MarshalIndent(original, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tokenPath, data, 0600); err != nil {
		t.Fatal(err)
	}

	// Read back
	readData, err := os.ReadFile(tokenPath)
	if err != nil {
		t.Fatal(err)
	}
	var loaded TokenFile
	if err := json.Unmarshal(readData, &loaded); err != nil {
		t.Fatal(err)
	}

	if loaded.Token != original.Token {
		t.Errorf("token mismatch: %s vs %s", loaded.Token, original.Token)
	}
	if loaded.Server != original.Server {
		t.Errorf("server mismatch: %s vs %s", loaded.Server, original.Server)
	}
	if loaded.Username != original.Username {
		t.Errorf("username mismatch: %s vs %s", loaded.Username, original.Username)
	}
}

func TestIsExpired(t *testing.T) {
	tests := []struct {
		name    string
		expires time.Time
		margin  time.Duration
		want    bool
	}{
		{"future token", time.Now().Add(24 * time.Hour), 0, false},
		{"past token", time.Now().Add(-1 * time.Hour), 0, true},
		{"expires within margin", time.Now().Add(30 * time.Minute), 1 * time.Hour, true},
		{"expires after margin", time.Now().Add(2 * time.Hour), 1 * time.Hour, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tf := &TokenFile{ExpiresAt: tt.expires}
			if got := tf.IsExpired(tt.margin); got != tt.want {
				t.Errorf("IsExpired(%v) = %v, want %v", tt.margin, got, tt.want)
			}
		})
	}
}
