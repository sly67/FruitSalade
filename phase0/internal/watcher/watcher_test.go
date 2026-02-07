package watcher

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatcher_CreateEvent(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "watcher-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create initial file
	initialFile := filepath.Join(tmpDir, "initial.txt")
	if err := os.WriteFile(initialFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("Failed to create initial file: %v", err)
	}

	// Create watcher with short interval
	w := New(tmpDir, 100*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := w.Start(ctx); err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}
	defer w.Stop()

	// Subscribe to events
	events := w.Subscribe()
	defer w.Unsubscribe(events)

	// Create a new file
	newFile := filepath.Join(tmpDir, "newfile.txt")
	if err := os.WriteFile(newFile, []byte("world"), 0644); err != nil {
		t.Fatalf("Failed to create new file: %v", err)
	}

	// Wait for event
	select {
	case event := <-events:
		if event.Type != EventCreate {
			t.Errorf("Expected create event, got %s", event.Type)
		}
		if event.Path != "/newfile.txt" {
			t.Errorf("Expected path /newfile.txt, got %s", event.Path)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for create event")
	}
}

func TestWatcher_ModifyEvent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "watcher-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create initial file
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	w := New(tmpDir, 100*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := w.Start(ctx); err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}
	defer w.Stop()

	events := w.Subscribe()
	defer w.Unsubscribe(events)

	// Modify the file
	time.Sleep(150 * time.Millisecond) // Wait for initial scan
	if err := os.WriteFile(testFile, []byte("hello world"), 0644); err != nil {
		t.Fatalf("Failed to modify file: %v", err)
	}

	select {
	case event := <-events:
		if event.Type != EventModify {
			t.Errorf("Expected modify event, got %s", event.Type)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for modify event")
	}
}

func TestWatcher_DeleteEvent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "watcher-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create initial file
	testFile := filepath.Join(tmpDir, "todelete.txt")
	if err := os.WriteFile(testFile, []byte("bye"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	w := New(tmpDir, 100*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := w.Start(ctx); err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}
	defer w.Stop()

	events := w.Subscribe()
	defer w.Unsubscribe(events)

	// Delete the file
	time.Sleep(150 * time.Millisecond)
	if err := os.Remove(testFile); err != nil {
		t.Fatalf("Failed to delete file: %v", err)
	}

	select {
	case event := <-events:
		if event.Type != EventDelete {
			t.Errorf("Expected delete event, got %s", event.Type)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for delete event")
	}
}

func TestWatcher_MultipleSubscribers(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "watcher-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	w := New(tmpDir, 100*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := w.Start(ctx); err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}
	defer w.Stop()

	// Create multiple subscribers
	sub1 := w.Subscribe()
	sub2 := w.Subscribe()
	defer w.Unsubscribe(sub1)
	defer w.Unsubscribe(sub2)

	// Create a file
	newFile := filepath.Join(tmpDir, "multi.txt")
	if err := os.WriteFile(newFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	// Both should receive the event
	received := 0
	timeout := time.After(2 * time.Second)

	for received < 2 {
		select {
		case <-sub1:
			received++
		case <-sub2:
			received++
		case <-timeout:
			t.Fatalf("Timeout: only %d subscribers received event", received)
		}
	}
}
