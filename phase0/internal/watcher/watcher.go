// Package watcher provides file system watching and SSE event streaming.
package watcher

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Event types.
const (
	EventCreate = "create"
	EventModify = "modify"
	EventDelete = "delete"
)

// Event represents a file system change.
type Event struct {
	Type string `json:"type"`
	Path string `json:"path"`
	Time int64  `json:"time"`
}

// Watcher watches a directory for changes.
type Watcher struct {
	root     string
	interval time.Duration

	mu       sync.RWMutex
	state    map[string]int64 // path -> mtime
	subs     map[chan Event]struct{}
	done     chan struct{}
}

// New creates a new file watcher.
func New(root string, interval time.Duration) *Watcher {
	if interval == 0 {
		interval = 5 * time.Second
	}
	return &Watcher{
		root:     root,
		interval: interval,
		state:    make(map[string]int64),
		subs:     make(map[chan Event]struct{}),
		done:     make(chan struct{}),
	}
}

// Start begins watching the directory.
func (w *Watcher) Start(ctx context.Context) error {
	// Build initial state
	if err := w.scan(); err != nil {
		return err
	}

	go w.watchLoop(ctx)
	return nil
}

// Stop stops the watcher.
func (w *Watcher) Stop() {
	close(w.done)
}

// Subscribe returns a channel that receives events.
func (w *Watcher) Subscribe() chan Event {
	ch := make(chan Event, 100)
	w.mu.Lock()
	w.subs[ch] = struct{}{}
	w.mu.Unlock()
	return ch
}

// Unsubscribe removes a subscriber.
func (w *Watcher) Unsubscribe(ch chan Event) {
	w.mu.Lock()
	delete(w.subs, ch)
	close(ch)
	w.mu.Unlock()
}

func (w *Watcher) watchLoop(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.checkChanges()
		case <-w.done:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (w *Watcher) scan() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	return filepath.Walk(w.root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if info.IsDir() {
			return nil
		}

		relPath, _ := filepath.Rel(w.root, path)
		w.state[relPath] = info.ModTime().UnixNano()
		return nil
	})
}

func (w *Watcher) checkChanges() {
	newState := make(map[string]int64)
	var events []Event

	filepath.Walk(w.root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		relPath, _ := filepath.Rel(w.root, path)
		mtime := info.ModTime().UnixNano()
		newState[relPath] = mtime

		w.mu.RLock()
		oldMtime, exists := w.state[relPath]
		w.mu.RUnlock()

		if !exists {
			events = append(events, Event{
				Type: EventCreate,
				Path: "/" + relPath,
				Time: time.Now().Unix(),
			})
		} else if mtime != oldMtime {
			events = append(events, Event{
				Type: EventModify,
				Path: "/" + relPath,
				Time: time.Now().Unix(),
			})
		}
		return nil
	})

	// Check for deletions
	w.mu.RLock()
	for path := range w.state {
		if _, exists := newState[path]; !exists {
			events = append(events, Event{
				Type: EventDelete,
				Path: "/" + path,
				Time: time.Now().Unix(),
			})
		}
	}
	w.mu.RUnlock()

	// Update state
	w.mu.Lock()
	w.state = newState
	w.mu.Unlock()

	// Broadcast events
	if len(events) > 0 {
		w.broadcast(events)
	}
}

func (w *Watcher) broadcast(events []Event) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	for ch := range w.subs {
		for _, event := range events {
			select {
			case ch <- event:
			default:
				log.Printf("Dropping event for slow subscriber: %v", event)
			}
		}
	}
}
