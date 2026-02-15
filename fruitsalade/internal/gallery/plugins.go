package gallery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/fruitsalade/fruitsalade/fruitsalade/internal/logging"
)

const pluginTimeout = 30 * time.Second
const minConfidence = 0.5

// PluginCaller calls auto-tagging plugin webhooks.
type PluginCaller struct {
	client *http.Client
}

// NewPluginCaller creates a new PluginCaller with a timeout.
func NewPluginCaller() *PluginCaller {
	return &PluginCaller{
		client: &http.Client{Timeout: pluginTimeout},
	}
}

// PluginWebhookRequest is sent to plugin webhooks.
type PluginWebhookRequest struct {
	FilePath    string `json:"file_path"`
	FileName    string `json:"file_name"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
	ImageURL    string `json:"image_url"`
}

// PluginWebhookResponse is the expected response from plugin webhooks.
type PluginWebhookResponse struct {
	Tags []PluginTag `json:"tags"`
}

// PluginTag is a single tag from a plugin response.
type PluginTag struct {
	Tag        string  `json:"tag"`
	Confidence float32 `json:"confidence"`
}

// CallPlugins calls all enabled plugins for a processed image and stores resulting tags.
func (pc *PluginCaller) CallPlugins(ctx context.Context, store *GalleryStore, filePath, fileName string, size int64) {
	plugins, err := store.ListEnabledPlugins(ctx)
	if err != nil {
		logging.Warn("gallery: failed to list plugins", zap.Error(err))
		return
	}

	if len(plugins) == 0 {
		return
	}

	req := PluginWebhookRequest{
		FilePath:    filePath,
		FileName:    fileName,
		ContentType: contentTypeForExt(filePath),
		Size:        size,
		ImageURL:    fmt.Sprintf("/api/v1/content/%s", filePath[1:]), // relative URL
	}

	for _, plugin := range plugins {
		tags, err := pc.callPlugin(ctx, plugin, req)
		if err != nil {
			logging.Warn("gallery: plugin call failed",
				zap.String("plugin", plugin.Name),
				zap.String("path", filePath),
				zap.Error(err))
			store.UpdatePluginHealth(ctx, plugin.ID, err.Error())
			continue
		}

		store.UpdatePluginHealth(ctx, plugin.ID, "")

		for _, t := range tags {
			if t.Confidence >= minConfidence {
				store.AddTag(ctx, filePath, t.Tag, "plugin:"+plugin.Name, t.Confidence)
			}
		}
	}
}

// TestPlugin sends a test request to a plugin webhook and returns the response.
func (pc *PluginCaller) TestPlugin(ctx context.Context, plugin Plugin) (*PluginWebhookResponse, error) {
	req := PluginWebhookRequest{
		FilePath:    "/test/image.jpg",
		FileName:    "image.jpg",
		ContentType: "image/jpeg",
		Size:        1024,
		ImageURL:    "/api/v1/content/test/image.jpg",
	}

	tags, err := pc.callPlugin(ctx, plugin, req)
	if err != nil {
		return nil, err
	}

	return &PluginWebhookResponse{Tags: tags}, nil
}

func (pc *PluginCaller) callPlugin(ctx context.Context, plugin Plugin, req PluginWebhookRequest) ([]PluginTag, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, plugin.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := pc.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("webhook call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("webhook returned %d", resp.StatusCode)
	}

	var webhookResp PluginWebhookResponse
	if err := json.NewDecoder(resp.Body).Decode(&webhookResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return webhookResp.Tags, nil
}

func contentTypeForExt(path string) string {
	switch {
	case hasAnySuffix(path, ".jpg", ".jpeg"):
		return "image/jpeg"
	case hasAnySuffix(path, ".png"):
		return "image/png"
	case hasAnySuffix(path, ".gif"):
		return "image/gif"
	case hasAnySuffix(path, ".webp"):
		return "image/webp"
	case hasAnySuffix(path, ".bmp"):
		return "image/bmp"
	case hasAnySuffix(path, ".tiff", ".tif"):
		return "image/tiff"
	case hasAnySuffix(path, ".heic", ".heif"):
		return "image/heic"
	default:
		return "application/octet-stream"
	}
}

func hasAnySuffix(s string, suffixes ...string) bool {
	lower := bytes.ToLower([]byte(s))
	for _, suffix := range suffixes {
		if bytes.HasSuffix(lower, []byte(suffix)) {
			return true
		}
	}
	return false
}
