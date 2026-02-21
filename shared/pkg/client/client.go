// Package client provides HTTP client with retry, offline support, and auth.
package client

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/fruitsalade/fruitsalade/shared/pkg/logger"
	"github.com/fruitsalade/fruitsalade/shared/pkg/models"
	"github.com/fruitsalade/fruitsalade/shared/pkg/protocol"
	"github.com/fruitsalade/fruitsalade/shared/pkg/retry"

	"encoding/json"
)

// Client provides HTTP client with retry, offline support, and auth.
type Client struct {
	baseURL     string
	httpClient  *http.Client
	retryConfig retry.Config

	mu        sync.RWMutex
	online    bool
	lastPing  time.Time
	authToken string
}

// Config holds client configuration.
type Config struct {
	BaseURL     string
	Timeout     time.Duration
	RetryConfig retry.Config
	AuthToken   string
}

// New creates a new client.
func New(cfg Config) *Client {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.RetryConfig.MaxAttempts == 0 {
		cfg.RetryConfig = retry.DefaultConfig()
	}

	return &Client{
		baseURL: cfg.BaseURL,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout:   10 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				MaxIdleConns:        100,
				IdleConnTimeout:     90 * time.Second,
				DisableCompression:  false,
				TLSHandshakeTimeout: 10 * time.Second,
			},
		},
		retryConfig: cfg.RetryConfig,
		online:      true,
		authToken:   cfg.AuthToken,
	}
}

// SetAuthToken sets the JWT auth token for requests.
func (c *Client) SetAuthToken(token string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.authToken = token
}

// applyAuth adds the auth header to a request if a token is set.
func (c *Client) applyAuth(req *http.Request) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}
}

// IsOnline returns true if the server is reachable.
func (c *Client) IsOnline() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.online
}

func (c *Client) setOnline(online bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.online != online {
		if online {
			logger.Info("Server is back online")
		} else {
			logger.Error("Server is offline")
		}
	}
	c.online = online
	c.lastPing = time.Now()
}

// Ping checks if the server is reachable.
func (c *Client) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/health", nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.setOnline(false)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.setOnline(false)
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}

	c.setOnline(true)
	return nil
}

// FetchMetadata fetches the metadata tree from the server.
func (c *Client) FetchMetadata(ctx context.Context) (*models.FileNode, error) {
	var result *models.FileNode

	err := retry.Do(ctx, c.retryConfig, func() error {
		req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/v1/tree", nil)
		if err != nil {
			return err
		}
		req.Header.Set("Accept-Encoding", "gzip")
		c.applyAuth(req)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			c.setOnline(false)
			return retry.Retryable(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			c.setOnline(false)
			if resp.StatusCode >= 500 {
				return retry.Retryable(fmt.Errorf("server error: %d", resp.StatusCode))
			}
			return fmt.Errorf("server returned %d", resp.StatusCode)
		}

		c.setOnline(true)

		var reader io.Reader = resp.Body
		if resp.Header.Get("Content-Encoding") == "gzip" {
			gr, err := gzip.NewReader(resp.Body)
			if err != nil {
				return err
			}
			defer gr.Close()
			reader = gr
		}

		var treeResp protocol.TreeResponse
		if err := json.NewDecoder(reader).Decode(&treeResp); err != nil {
			return err
		}

		result = treeResp.Root
		return nil
	})

	return result, err
}

// FetchContent fetches file content with optional range.
func (c *Client) FetchContent(ctx context.Context, fileID string, offset, length int64) (io.ReadCloser, int64, error) {
	var reader io.ReadCloser
	var totalSize int64

	err := retry.Do(ctx, c.retryConfig, func() error {
		url := c.baseURL + "/api/v1/content/" + fileID
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return err
		}

		if offset > 0 || length > 0 {
			end := ""
			if length > 0 {
				end = fmt.Sprintf("%d", offset+length-1)
			}
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-%s", offset, end))
		}

		req.Header.Set("Accept-Encoding", "gzip")
		c.applyAuth(req)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			c.setOnline(false)
			return retry.Retryable(err)
		}

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
			resp.Body.Close()
			c.setOnline(false)
			if resp.StatusCode >= 500 {
				return retry.Retryable(fmt.Errorf("server error: %d", resp.StatusCode))
			}
			return fmt.Errorf("server returned %d", resp.StatusCode)
		}

		c.setOnline(true)

		if cl := resp.Header.Get("Content-Length"); cl != "" {
			fmt.Sscanf(cl, "%d", &totalSize)
		}

		if resp.Header.Get("Content-Encoding") == "gzip" {
			gr, err := gzip.NewReader(resp.Body)
			if err != nil {
				resp.Body.Close()
				return err
			}
			reader = &gzipReadCloser{gr: gr, body: resp.Body}
		} else {
			reader = resp.Body
		}

		return nil
	})

	return reader, totalSize, err
}

// FetchContentFull fetches the entire file content.
func (c *Client) FetchContentFull(ctx context.Context, fileID string) (io.ReadCloser, int64, error) {
	return c.FetchContent(ctx, fileID, 0, -1)
}

// ErrOffline is returned when the server is offline.
var ErrOffline = errors.New("server is offline")

// FetchResult holds the result of a concurrent file fetch.
type FetchResult struct {
	FileID string
	Reader io.ReadCloser
	Size   int64
	Err    error
}

// FetchContentConcurrent fetches multiple files concurrently.
func (c *Client) FetchContentConcurrent(ctx context.Context, fileIDs []string, maxConcurrent int) <-chan FetchResult {
	results := make(chan FetchResult, len(fileIDs))

	if maxConcurrent <= 0 {
		maxConcurrent = 10
	}

	go func() {
		defer close(results)

		sem := make(chan struct{}, maxConcurrent)
		var wg sync.WaitGroup

		for _, fileID := range fileIDs {
			select {
			case <-ctx.Done():
				results <- FetchResult{FileID: fileID, Err: ctx.Err()}
				continue
			default:
			}

			wg.Add(1)
			sem <- struct{}{}

			go func(id string) {
				defer wg.Done()
				defer func() { <-sem }()

				reader, size, err := c.FetchContentFull(ctx, id)
				results <- FetchResult{
					FileID: id,
					Reader: reader,
					Size:   size,
					Err:    err,
				}
			}(fileID)
		}

		wg.Wait()
	}()

	return results
}

// PrefetchFiles prefetches multiple files to a cache directory concurrently.
func (c *Client) PrefetchFiles(ctx context.Context, fileIDs []string, maxConcurrent int, saveFn func(fileID string, r io.Reader, size int64) error) <-chan error {
	errs := make(chan error, len(fileIDs))

	go func() {
		defer close(errs)

		results := c.FetchContentConcurrent(ctx, fileIDs, maxConcurrent)
		for result := range results {
			if result.Err != nil {
				errs <- fmt.Errorf("fetch %s: %w", result.FileID, result.Err)
				continue
			}

			if err := saveFn(result.FileID, result.Reader, result.Size); err != nil {
				result.Reader.Close()
				errs <- fmt.Errorf("save %s: %w", result.FileID, err)
				continue
			}

			result.Reader.Close()
			errs <- nil
		}
	}()

	return errs
}

type gzipReadCloser struct {
	gr   *gzip.Reader
	body io.ReadCloser
}

func (g *gzipReadCloser) Read(p []byte) (int, error) {
	return g.gr.Read(p)
}

func (g *gzipReadCloser) Close() error {
	g.gr.Close()
	return g.body.Close()
}

// UploadResponse holds the response from an upload operation.
type UploadResponse struct {
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	Hash    string `json:"hash"`
	Version int    `json:"version"`
}

// ConflictError is returned when an upload conflicts with the current server version.
type ConflictError struct {
	Path            string
	ExpectedVersion int
	CurrentVersion  int
	CurrentHash     string
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("conflict on %s: expected version %d, server has %d",
		e.Path, e.ExpectedVersion, e.CurrentVersion)
}

// AsConflict checks if an error is a ConflictError and returns it.
func AsConflict(err error) (*ConflictError, bool) {
	var ce *ConflictError
	if errors.As(err, &ce) {
		return ce, true
	}
	return nil, false
}

// UploadFile uploads file content to the server.
// If expectedVersion > 0, the X-Expected-Version header is sent for conflict detection.
func (c *Client) UploadFile(ctx context.Context, path string, content io.Reader, size int64, expectedVersion int) (*UploadResponse, error) {
	var result *UploadResponse

	err := retry.Do(ctx, c.retryConfig, func() error {
		url := c.baseURL + "/api/v1/content/" + path
		req, err := http.NewRequestWithContext(ctx, "POST", url, content)
		if err != nil {
			return err
		}

		req.ContentLength = size
		req.Header.Set("Content-Type", "application/octet-stream")
		if expectedVersion > 0 {
			req.Header.Set("X-Expected-Version", strconv.Itoa(expectedVersion))
		}
		c.applyAuth(req)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			c.setOnline(false)
			return retry.Retryable(err)
		}
		defer resp.Body.Close()

		// Handle 409 Conflict — NOT retryable
		if resp.StatusCode == http.StatusConflict {
			c.setOnline(true)
			var cr protocol.ConflictResponse
			if json.NewDecoder(resp.Body).Decode(&cr) == nil {
				return &ConflictError{
					Path:            cr.Path,
					ExpectedVersion: cr.ExpectedVersion,
					CurrentVersion:  cr.CurrentVersion,
					CurrentHash:     cr.CurrentHash,
				}
			}
			return &ConflictError{Path: path, ExpectedVersion: expectedVersion}
		}

		if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
			c.setOnline(false)
			if resp.StatusCode >= 500 {
				return retry.Retryable(fmt.Errorf("server error: %d", resp.StatusCode))
			}
			// Try to read error message
			var errResp protocol.ErrorResponse
			if json.NewDecoder(resp.Body).Decode(&errResp) == nil {
				return fmt.Errorf("upload failed: %s", errResp.Error)
			}
			return fmt.Errorf("upload failed: %d", resp.StatusCode)
		}

		c.setOnline(true)

		result = &UploadResponse{}
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return err
		}

		return nil
	})

	return result, err
}

// CreateDirectory creates a directory on the server.
func (c *Client) CreateDirectory(ctx context.Context, path string) error {
	err := retry.Do(ctx, c.retryConfig, func() error {
		url := c.baseURL + "/api/v1/tree/" + path + "?type=dir"
		req, err := http.NewRequestWithContext(ctx, "PUT", url, nil)
		if err != nil {
			return err
		}
		c.applyAuth(req)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			c.setOnline(false)
			return retry.Retryable(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
			c.setOnline(false)
			if resp.StatusCode >= 500 {
				return retry.Retryable(fmt.Errorf("server error: %d", resp.StatusCode))
			}
			var errResp protocol.ErrorResponse
			if json.NewDecoder(resp.Body).Decode(&errResp) == nil {
				return fmt.Errorf("mkdir failed: %s", errResp.Error)
			}
			return fmt.Errorf("mkdir failed: %d", resp.StatusCode)
		}

		c.setOnline(true)
		return nil
	})

	return err
}

// DeletePath deletes a file or directory on the server.
func (c *Client) DeletePath(ctx context.Context, path string) error {
	err := retry.Do(ctx, c.retryConfig, func() error {
		url := c.baseURL + "/api/v1/tree/" + path
		req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
		if err != nil {
			return err
		}
		c.applyAuth(req)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			c.setOnline(false)
			return retry.Retryable(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			c.setOnline(false)
			if resp.StatusCode == http.StatusNotFound {
				return nil // Already deleted
			}
			if resp.StatusCode >= 500 {
				return retry.Retryable(fmt.Errorf("server error: %d", resp.StatusCode))
			}
			var errResp protocol.ErrorResponse
			if json.NewDecoder(resp.Body).Decode(&errResp) == nil {
				return fmt.Errorf("delete failed: %s", errResp.Error)
			}
			return fmt.Errorf("delete failed: %d", resp.StatusCode)
		}

		c.setOnline(true)
		return nil
	})

	return err
}
