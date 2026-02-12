package client

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/fruitsalade/fruitsalade/shared/pkg/logger"
)

// SSEEvent represents a Server-Sent Event.
type SSEEvent struct {
	Type string          `json:"type"`
	Path string          `json:"path"`
	Time int64           `json:"time"`
	Raw  json.RawMessage `json:"-"`
}

// SSEClient handles Server-Sent Events from the server.
type SSEClient struct {
	baseURL      string
	httpClient   *http.Client
	reconnectMin time.Duration
	reconnectMax time.Duration
	mu           sync.RWMutex
	authToken    string
}

// NewSSEClient creates a new SSE client.
func NewSSEClient(baseURL string) *SSEClient {
	return &SSEClient{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 0, // No timeout for SSE
		},
		reconnectMin: 1 * time.Second,
		reconnectMax: 30 * time.Second,
	}
}

// SetAuthToken sets the JWT auth token for SSE requests.
func (c *SSEClient) SetAuthToken(token string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.authToken = token
}

// Subscribe connects to the SSE endpoint and returns a channel of events.
func (c *SSEClient) Subscribe(ctx context.Context) (<-chan SSEEvent, <-chan error) {
	events := make(chan SSEEvent, 100)
	errors := make(chan error, 1)

	go c.subscribeLoop(ctx, events, errors)

	return events, errors
}

func (c *SSEClient) subscribeLoop(ctx context.Context, events chan<- SSEEvent, errors chan<- error) {
	defer close(events)
	defer close(errors)

	reconnectDelay := c.reconnectMin

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		err := c.connect(ctx, events)
		if err != nil {
			if ctx.Err() != nil {
				return
			}

			logger.Error("SSE connection error: %v (reconnecting in %s)", err, reconnectDelay)

			select {
			case <-ctx.Done():
				return
			case <-time.After(reconnectDelay):
			}

			reconnectDelay *= 2
			if reconnectDelay > c.reconnectMax {
				reconnectDelay = c.reconnectMax
			}
			continue
		}

		reconnectDelay = c.reconnectMin
	}
}

func (c *SSEClient) connect(ctx context.Context, events chan<- SSEEvent) error {
	url := c.baseURL + "/api/v1/events"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	c.mu.RLock()
	token := c.authToken
	c.mu.RUnlock()
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}

	logger.Info("SSE connected to %s", url)

	scanner := bufio.NewScanner(resp.Body)
	var eventType string
	var data string

	for scanner.Scan() {
		line := scanner.Text()

		select {
		case <-ctx.Done():
			return nil
		default:
		}

		if line == "" {
			if data != "" {
				event := SSEEvent{
					Type: eventType,
					Raw:  json.RawMessage(data),
				}
				json.Unmarshal([]byte(data), &event)

				select {
				case events <- event:
				default:
					logger.Debug("SSE event dropped (channel full)")
				}
			}
			eventType = ""
			data = ""
			continue
		}

		if strings.HasPrefix(line, ":") {
			continue
		}

		if strings.HasPrefix(line, "event:") {
			eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "data:") {
			data = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read: %w", err)
	}

	return fmt.Errorf("connection closed")
}
