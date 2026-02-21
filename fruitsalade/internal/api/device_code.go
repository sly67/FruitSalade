package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// handleDeviceCodeInit proxies a device authorization request to the OIDC provider.
// POST /api/v1/auth/device-code
func (s *Server) handleDeviceCodeInit(w http.ResponseWriter, r *http.Request) {
	oidcCfg := s.auth.OIDCConfig()
	if oidcCfg == nil || oidcCfg.DeviceAuthEndpoint == "" {
		s.sendError(w, http.StatusNotImplemented, "OIDC device code flow not configured")
		return
	}

	// Forward to OIDC provider's device_authorization_endpoint
	data := url.Values{
		"client_id": {oidcCfg.ClientID},
		"scope":     {"openid profile email"},
	}
	if oidcCfg.ClientSecret != "" {
		data.Set("client_secret", oidcCfg.ClientSecret)
	}

	resp, err := http.Post(oidcCfg.DeviceAuthEndpoint, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
	if err != nil {
		s.sendError(w, http.StatusBadGateway, "failed to contact OIDC provider: "+err.Error())
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// handleDeviceCodePoll proxies a device token poll request to the OIDC provider.
// POST /api/v1/auth/device-token
func (s *Server) handleDeviceCodePoll(w http.ResponseWriter, r *http.Request) {
	oidcCfg := s.auth.OIDCConfig()
	if oidcCfg == nil || oidcCfg.TokenEndpoint == "" {
		s.sendError(w, http.StatusNotImplemented, "OIDC device code flow not configured")
		return
	}

	// Read the device_code from the request body
	var req struct {
		DeviceCode string `json:"device_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Forward to OIDC provider's token_endpoint
	data := url.Values{
		"client_id":   {oidcCfg.ClientID},
		"device_code": {req.DeviceCode},
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
	}
	if oidcCfg.ClientSecret != "" {
		data.Set("client_secret", oidcCfg.ClientSecret)
	}

	resp, err := http.Post(oidcCfg.TokenEndpoint, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
	if err != nil {
		s.sendError(w, http.StatusBadGateway, "failed to contact OIDC provider: "+err.Error())
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
