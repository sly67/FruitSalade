// Package events provides an SSE event broadcaster for real-time file sync.
package events

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/fruitsalade/fruitsalade/fruitsalade/internal/metrics"
)

const (
	EventCreate  = "create"
	EventModify  = "modify"
	EventDelete  = "delete"
	EventVersion = "version"
)

// Event represents a file system change event.
type Event struct {
	Type      string `json:"type"`
	Path      string `json:"path"`
	Version   int    `json:"version,omitempty"`
	Hash      string `json:"hash,omitempty"`
	Size      int64  `json:"size,omitempty"`
	Timestamp int64  `json:"timestamp"`
}

// Broadcaster manages SSE subscribers and publishes events.
type Broadcaster struct {
	mu          sync.RWMutex
	subscribers map[chan Event]struct{}
}

// NewBroadcaster creates a new event broadcaster.
func NewBroadcaster() *Broadcaster {
	return &Broadcaster{
		subscribers: make(map[chan Event]struct{}),
	}
}

// Subscribe adds a new subscriber and returns its event channel.
// The caller must call Unsubscribe when done.
func (b *Broadcaster) Subscribe() chan Event {
	ch := make(chan Event, 64)
	b.mu.Lock()
	b.subscribers[ch] = struct{}{}
	b.mu.Unlock()
	metrics.SetSSEConnectionsActive(int64(b.Count()))
	return ch
}

// Unsubscribe removes a subscriber and closes its channel.
func (b *Broadcaster) Unsubscribe(ch chan Event) {
	b.mu.Lock()
	delete(b.subscribers, ch)
	close(ch)
	b.mu.Unlock()
	metrics.SetSSEConnectionsActive(int64(b.Count()))
}

// Publish sends an event to all subscribers. Non-blocking: drops events
// for slow consumers.
func (b *Broadcaster) Publish(event Event) {
	if event.Timestamp == 0 {
		event.Timestamp = time.Now().Unix()
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.subscribers {
		select {
		case ch <- event:
		default:
			// Drop event for slow consumer
		}
	}
	metrics.RecordSSEEvent(event.Type)
}

// Count returns the current number of subscribers.
func (b *Broadcaster) Count() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subscribers)
}

// MarshalEvent serializes an event to JSON.
func MarshalEvent(e Event) ([]byte, error) {
	return json.Marshal(e)
}
