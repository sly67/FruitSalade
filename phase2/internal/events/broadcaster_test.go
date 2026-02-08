package events

import (
	"testing"
	"time"
)

func TestBroadcasterSubscribeUnsubscribe(t *testing.T) {
	b := NewBroadcaster()

	ch1 := b.Subscribe()
	ch2 := b.Subscribe()

	if b.Count() != 2 {
		t.Fatalf("expected 2 subscribers, got %d", b.Count())
	}

	b.Unsubscribe(ch1)
	if b.Count() != 1 {
		t.Fatalf("expected 1 subscriber after unsubscribe, got %d", b.Count())
	}

	b.Unsubscribe(ch2)
	if b.Count() != 0 {
		t.Fatalf("expected 0 subscribers, got %d", b.Count())
	}
}

func TestBroadcasterPublish(t *testing.T) {
	b := NewBroadcaster()
	ch := b.Subscribe()
	defer b.Unsubscribe(ch)

	event := Event{
		Type: EventCreate,
		Path: "/test/file.txt",
		Size: 100,
	}
	b.Publish(event)

	select {
	case received := <-ch:
		if received.Type != EventCreate {
			t.Errorf("expected type %s, got %s", EventCreate, received.Type)
		}
		if received.Path != "/test/file.txt" {
			t.Errorf("expected path /test/file.txt, got %s", received.Path)
		}
		if received.Timestamp == 0 {
			t.Error("expected non-zero timestamp")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestBroadcasterMultipleSubscribers(t *testing.T) {
	b := NewBroadcaster()
	ch1 := b.Subscribe()
	ch2 := b.Subscribe()
	defer b.Unsubscribe(ch1)
	defer b.Unsubscribe(ch2)

	event := Event{Type: EventModify, Path: "/shared.txt"}
	b.Publish(event)

	for i, ch := range []chan Event{ch1, ch2} {
		select {
		case received := <-ch:
			if received.Path != "/shared.txt" {
				t.Errorf("subscriber %d: expected /shared.txt, got %s", i, received.Path)
			}
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d: timed out", i)
		}
	}
}

func TestBroadcasterDropsForSlowConsumer(t *testing.T) {
	b := NewBroadcaster()
	ch := b.Subscribe()
	defer b.Unsubscribe(ch)

	// Fill the channel buffer (64)
	for i := 0; i < 100; i++ {
		b.Publish(Event{Type: EventCreate, Path: "/overflow.txt"})
	}

	// Should not block or panic
	count := 0
	for {
		select {
		case <-ch:
			count++
		default:
			goto done
		}
	}
done:
	if count != 64 {
		t.Errorf("expected 64 buffered events, got %d", count)
	}
}

func TestMarshalEvent(t *testing.T) {
	e := Event{
		Type:      EventDelete,
		Path:      "/deleted.txt",
		Timestamp: 1234567890,
	}
	data, err := MarshalEvent(e)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty JSON")
	}
}
