package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/fruitsalade/fruitsalade/shared/pkg/logger"
)

// TokenFile holds a saved authentication token.
type TokenFile struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	Server    string    `json:"server"`
	Username  string    `json:"username"`
}

// IsExpired returns true if the token has expired (with optional margin).
func (t *TokenFile) IsExpired(margin time.Duration) bool {
	return time.Now().Add(margin).After(t.ExpiresAt)
}

// LoginResponse is the response from POST /api/v1/auth/token.
type LoginResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	User      struct {
		ID       int    `json:"id"`
		Username string `json:"username"`
		IsAdmin  bool   `json:"is_admin"`
	} `json:"user"`
}

// RefreshResponse is the response from POST /api/v1/auth/refresh.
type RefreshResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// DeviceCodeResponse is the response from POST /api/v1/auth/device-code.
type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// DeviceTokenResponse is the response from POST /api/v1/auth/device-token.
type DeviceTokenResponse struct {
	AccessToken string `json:"access_token"`
	IDToken     string `json:"id_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	Error       string `json:"error,omitempty"`
}

// Login authenticates with username/password and returns a token.
func (c *Client) Login(ctx context.Context, username, password, deviceName string) (*LoginResponse, error) {
	body, _ := json.Marshal(map[string]string{
		"username":    username,
		"password":    password,
		"device_name": deviceName,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/v1/auth/token", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("login request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("login failed (%d): %s", resp.StatusCode, string(data))
	}

	var result LoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("parse login response: %w", err)
	}

	c.SetAuthToken(result.Token)
	return &result, nil
}

// RefreshToken refreshes the current token. Uses the current bearer token.
func (c *Client) RefreshToken(ctx context.Context) (*RefreshResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/v1/auth/refresh", nil)
	if err != nil {
		return nil, err
	}
	c.applyAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("refresh request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("refresh failed (%d): %s", resp.StatusCode, string(data))
	}

	var result RefreshResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("parse refresh response: %w", err)
	}

	c.SetAuthToken(result.Token)
	return &result, nil
}

// Logout revokes the current token on the server.
func (c *Client) Logout(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "DELETE", c.baseURL+"/api/v1/auth/token", nil)
	if err != nil {
		return err
	}
	c.applyAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("logout request failed: %w", err)
	}
	defer resp.Body.Close()

	c.SetAuthToken("")
	return nil
}

// DeviceCodeAuth performs the OAuth2 device code flow.
// It prints the verification URL and user code, then polls until authentication completes.
func (c *Client) DeviceCodeAuth(ctx context.Context, deviceName string) (*LoginResponse, error) {
	// Step 1: Request device code
	body, _ := json.Marshal(map[string]string{
		"device_name": deviceName,
	})
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/v1/auth/device-code", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("device code request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotImplemented {
		return nil, fmt.Errorf("OIDC device code flow not configured on server")
	}
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("device code request failed (%d): %s", resp.StatusCode, string(data))
	}

	var dcResp DeviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&dcResp); err != nil {
		return nil, fmt.Errorf("parse device code response: %w", err)
	}

	// Step 2: Print instructions
	fmt.Printf("\nTo sign in, open: %s\n", dcResp.VerificationURI)
	fmt.Printf("Enter code: %s\n\n", dcResp.UserCode)

	// Step 3: Poll for token
	interval := time.Duration(dcResp.Interval) * time.Second
	if interval < 5*time.Second {
		interval = 5 * time.Second
	}
	deadline := time.Now().Add(time.Duration(dcResp.ExpiresIn) * time.Second)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}

		pollBody, _ := json.Marshal(map[string]string{
			"device_code": dcResp.DeviceCode,
		})
		pollReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/v1/auth/device-token", bytes.NewReader(pollBody))
		if err != nil {
			return nil, err
		}
		pollReq.Header.Set("Content-Type", "application/json")

		pollResp, err := c.httpClient.Do(pollReq)
		if err != nil {
			continue
		}

		var tokenResp DeviceTokenResponse
		json.NewDecoder(pollResp.Body).Decode(&tokenResp)
		pollResp.Body.Close()

		if tokenResp.Error == "authorization_pending" || tokenResp.Error == "slow_down" {
			if tokenResp.Error == "slow_down" {
				interval += 5 * time.Second
			}
			continue
		}

		if tokenResp.Error != "" {
			return nil, fmt.Errorf("device auth failed: %s", tokenResp.Error)
		}

		// Use whichever token is available (ID token for OIDC, access token as fallback)
		token := tokenResp.IDToken
		if token == "" {
			token = tokenResp.AccessToken
		}
		if token == "" {
			continue
		}

		c.SetAuthToken(token)
		return &LoginResponse{
			Token:     token,
			ExpiresAt: time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
		}, nil
	}

	return nil, fmt.Errorf("device code expired, please try again")
}

// StartTokenRefreshLoop starts a goroutine that refreshes the token before it expires.
func (c *Client) StartTokenRefreshLoop(ctx context.Context, tf *TokenFile) {
	go func() {
		ticker := time.NewTicker(15 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Refresh if token expires within 1 hour
				if tf.IsExpired(1 * time.Hour) {
					logger.Info("Token expiring soon, refreshing...")
					refreshResp, err := c.RefreshToken(ctx)
					if err != nil {
						logger.Error("Token refresh failed: %v", err)
						continue
					}
					tf.Token = refreshResp.Token
					tf.ExpiresAt = refreshResp.ExpiresAt
					if err := SaveToken(tf); err != nil {
						logger.Error("Failed to save refreshed token: %v", err)
					} else {
						logger.Info("Token refreshed, expires %s", tf.ExpiresAt.Format(time.RFC3339))
					}
				}
			}
		}
	}()
}

// TokenFilePath returns the default path for the token file.
func TokenFilePath() string {
	if runtime.GOOS == "windows" {
		appData := os.Getenv("APPDATA")
		if appData == "" {
			home, _ := os.UserHomeDir()
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(appData, "FruitSalade", "token.json")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "fruitsalade", "token.json")
}

// SaveToken saves a token file to the default location.
func SaveToken(tf *TokenFile) error {
	path := TokenFilePath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(tf, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// LoadToken loads a token file from the default location.
func LoadToken() (*TokenFile, error) {
	data, err := os.ReadFile(TokenFilePath())
	if err != nil {
		return nil, err
	}
	var tf TokenFile
	if err := json.Unmarshal(data, &tf); err != nil {
		return nil, err
	}
	return &tf, nil
}

// DeleteToken removes the saved token file.
func DeleteToken() error {
	return os.Remove(TokenFilePath())
}
