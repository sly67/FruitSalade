package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fruitsalade/fruitsalade/shared/pkg/protocol"
	"github.com/fruitsalade/fruitsalade/shared/pkg/retry"
)

func testClient(handler http.Handler) (*Client, *httptest.Server) {
	ts := httptest.NewServer(handler)
	c := New(Config{
		BaseURL: ts.URL,
		RetryConfig: retry.Config{
			MaxAttempts: 3,
			InitialWait: time.Millisecond,
			MaxWait:     time.Millisecond,
		},
	})
	return c, ts
}

func TestUploadFile_Success(t *testing.T) {
	var gotHeader string
	c, ts := testClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("X-Expected-Version")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"path":    "/test.txt",
			"size":    5,
			"hash":    "abc123",
			"version": 2,
		})
	}))
	defer ts.Close()

	resp, err := c.UploadFile(context.Background(), "test.txt", strings.NewReader("hello"), 5, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Version != 2 {
		t.Errorf("expected version 2, got %d", resp.Version)
	}
	if resp.Hash != "abc123" {
		t.Errorf("expected hash abc123, got %s", resp.Hash)
	}
	if gotHeader != "1" {
		t.Errorf("expected X-Expected-Version=1, got %q", gotHeader)
	}
}

func TestUploadFile_Conflict(t *testing.T) {
	c, ts := testClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(protocol.ConflictResponse{
			Error:           "version conflict",
			Path:            "/test.txt",
			ExpectedVersion: 1,
			CurrentVersion:  3,
			CurrentHash:     "def456",
		})
	}))
	defer ts.Close()

	_, err := c.UploadFile(context.Background(), "test.txt", strings.NewReader("hello"), 5, 1)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	ce, ok := AsConflict(err)
	if !ok {
		t.Fatalf("expected ConflictError, got %T: %v", err, err)
	}
	if ce.Path != "/test.txt" {
		t.Errorf("expected path /test.txt, got %s", ce.Path)
	}
	if ce.CurrentVersion != 3 {
		t.Errorf("expected current version 3, got %d", ce.CurrentVersion)
	}
	if ce.CurrentHash != "def456" {
		t.Errorf("expected hash def456, got %s", ce.CurrentHash)
	}
}

func TestUploadFile_NoVersionHeader(t *testing.T) {
	var gotHeader string
	c, ts := testClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("X-Expected-Version")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"path": "/test.txt", "size": 5, "hash": "abc", "version": 1,
		})
	}))
	defer ts.Close()

	_, err := c.UploadFile(context.Background(), "test.txt", strings.NewReader("hello"), 5, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotHeader != "" {
		t.Errorf("expected no X-Expected-Version header, got %q", gotHeader)
	}
}

func TestUploadFile_ServerError_Retry(t *testing.T) {
	var attempts atomic.Int32
	c, ts := testClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"path": "/test.txt", "size": 5, "hash": "abc", "version": 1,
		})
	}))
	defer ts.Close()

	_, err := c.UploadFile(context.Background(), "test.txt", strings.NewReader("hello"), 5, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if attempts.Load() < 3 {
		t.Errorf("expected at least 3 attempts, got %d", attempts.Load())
	}
}

func TestUploadFile_ConflictNotRetried(t *testing.T) {
	var attempts atomic.Int32
	c, ts := testClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(protocol.ConflictResponse{
			Error:           "version conflict",
			Path:            "/test.txt",
			ExpectedVersion: 1,
			CurrentVersion:  2,
		})
	}))
	defer ts.Close()

	_, err := c.UploadFile(context.Background(), "test.txt", strings.NewReader("hello"), 5, 1)
	if err == nil {
		t.Fatal("expected error")
	}
	if _, ok := AsConflict(err); !ok {
		t.Fatalf("expected ConflictError, got %T", err)
	}
	if attempts.Load() != 1 {
		t.Errorf("expected exactly 1 attempt (no retries), got %d", attempts.Load())
	}
}

func TestOnlineStatusAfterConflict(t *testing.T) {
	c, ts := testClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(protocol.ConflictResponse{
			Error: "version conflict", Path: "/test.txt",
			ExpectedVersion: 1, CurrentVersion: 2,
		})
	}))
	defer ts.Close()

	c.UploadFile(context.Background(), "test.txt", strings.NewReader("hello"), 5, 1)

	if !c.IsOnline() {
		t.Error("client should remain online after a 409 conflict")
	}
}
