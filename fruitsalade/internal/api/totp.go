package api

import (
	"encoding/json"
	"net/http"

	"go.uber.org/zap"

	"github.com/fruitsalade/fruitsalade/fruitsalade/internal/auth"
	"github.com/fruitsalade/fruitsalade/fruitsalade/internal/logging"
	"github.com/fruitsalade/fruitsalade/fruitsalade/internal/metrics"
)

// handleTOTPVerify handles POST /api/v1/auth/totp/verify (public).
// Validates the temp token + TOTP code, then issues a full JWT.
func (s *Server) handleTOTPVerify(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TOTPToken  string `json:"totp_token"`
		Code       string `json:"code"`
		DeviceName string `json:"device_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.TOTPToken == "" || req.Code == "" {
		s.sendError(w, http.StatusBadRequest, "totp_token and code required")
		return
	}

	// Validate temp token
	claims, err := s.auth.ValidateTOTPTempToken(req.TOTPToken)
	if err != nil {
		metrics.RecordAuthAttempt(false)
		s.sendError(w, http.StatusUnauthorized, "invalid or expired TOTP token")
		return
	}

	// Validate TOTP code
	if err := s.auth.ValidateTOTP(r.Context(), claims.UserID, req.Code); err != nil {
		metrics.RecordAuthAttempt(false)
		s.sendError(w, http.StatusUnauthorized, "invalid TOTP code")
		return
	}

	// Issue full JWT (same flow as normal login completion)
	tokenStr, expiresAt, err := s.auth.IssueToken(r.Context(), claims.UserID, claims.Username, claims.IsAdmin, req.DeviceName)
	if err != nil {
		logging.Error("failed to issue token after TOTP", zap.Error(err))
		s.sendError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	metrics.RecordAuthAttempt(true)
	logging.Info("TOTP login successful", zap.String("username", claims.Username))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token":      tokenStr,
		"expires_at": expiresAt,
		"user": map[string]interface{}{
			"id":       claims.UserID,
			"username": claims.Username,
			"is_admin": claims.IsAdmin,
		},
	})
}

// handleTOTPStatus handles GET /api/v1/auth/totp/status (protected).
func (s *Server) handleTOTPStatus(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		s.sendError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	enabled, err := s.auth.IsTOTPEnabled(r.Context(), claims.UserID)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to check TOTP status")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"enabled": enabled,
	})
}

// handleTOTPSetup handles POST /api/v1/auth/totp/setup (protected).
func (s *Server) handleTOTPSetup(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		s.sendError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	result, err := s.auth.GenerateTOTPSetup(r.Context(), claims.UserID, claims.Username)
	if err != nil {
		logging.Error("TOTP setup failed", zap.Error(err))
		s.sendError(w, http.StatusInternalServerError, "failed to generate TOTP setup")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// handleTOTPEnable handles POST /api/v1/auth/totp/enable (protected).
func (s *Server) handleTOTPEnable(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		s.sendError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req struct {
		Secret string `json:"secret"`
		Code   string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Secret == "" || req.Code == "" {
		s.sendError(w, http.StatusBadRequest, "secret and code required")
		return
	}

	backupCodes, err := s.auth.EnableTOTP(r.Context(), claims.UserID, req.Secret, req.Code)
	if err != nil {
		s.sendError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"enabled":      true,
		"backup_codes": backupCodes,
	})
}

// handleTOTPDisable handles POST /api/v1/auth/totp/disable (protected).
func (s *Server) handleTOTPDisable(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		s.sendError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req struct {
		Password string `json:"password"`
		Code     string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Password == "" || req.Code == "" {
		s.sendError(w, http.StatusBadRequest, "password and code required")
		return
	}

	if err := s.auth.DisableTOTP(r.Context(), claims.UserID, req.Password, req.Code); err != nil {
		s.sendError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"enabled": false,
	})
}

// handleTOTPBackup handles POST /api/v1/auth/totp/backup (protected).
func (s *Server) handleTOTPBackup(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		s.sendError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	codes, err := s.auth.RegenerateBackupCodes(r.Context(), claims.UserID)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "failed to regenerate backup codes")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"backup_codes": codes,
	})
}

