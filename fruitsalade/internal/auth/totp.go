package auth

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	"image/png"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/lib/pq"
	"github.com/pquerna/otp/totp"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"github.com/fruitsalade/fruitsalade/fruitsalade/internal/logging"
)

// TOTPSetupResult holds the data returned when initiating TOTP setup.
type TOTPSetupResult struct {
	Secret  string `json:"secret"`
	QRPNG   string `json:"qr_png"`   // base64 PNG
	OTPAuth string `json:"otpauth"`   // otpauth:// URI
}

// GenerateTOTPSetup generates a new TOTP secret and QR code for the user.
func (a *Auth) GenerateTOTPSetup(ctx context.Context, userID int, username string) (*TOTPSetupResult, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "FruitSalade",
		AccountName: username,
	})
	if err != nil {
		return nil, fmt.Errorf("generate TOTP key: %w", err)
	}

	// Generate QR code image
	img, err := key.Image(256, 256)
	if err != nil {
		return nil, fmt.Errorf("generate QR image: %w", err)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("encode QR PNG: %w", err)
	}

	return &TOTPSetupResult{
		Secret:  key.Secret(),
		QRPNG:   base64.StdEncoding.EncodeToString(buf.Bytes()),
		OTPAuth: key.URL(),
	}, nil
}

// EnableTOTP verifies the code against the secret, stores it, and generates backup codes.
// Returns the plaintext backup codes.
func (a *Auth) EnableTOTP(ctx context.Context, userID int, secret, code string) ([]string, error) {
	if !totp.Validate(code, secret) {
		return nil, fmt.Errorf("invalid TOTP code")
	}

	// Generate backup codes
	plainCodes, hashedCodes, err := generateBackupCodes(10)
	if err != nil {
		return nil, fmt.Errorf("generate backup codes: %w", err)
	}

	_, err = a.db.ExecContext(ctx,
		`UPDATE users SET totp_secret = $1, totp_enabled = TRUE, totp_backup_codes = $2 WHERE id = $3`,
		secret, pq.Array(hashedCodes), userID)
	if err != nil {
		return nil, fmt.Errorf("enable TOTP: %w", err)
	}

	logging.Info("TOTP enabled", zap.Int("user_id", userID))
	return plainCodes, nil
}

// DisableTOTP disables TOTP for a user after verifying password and TOTP code.
func (a *Auth) DisableTOTP(ctx context.Context, userID int, password, code string) error {
	// Verify password
	var hashedPassword string
	var totpSecret string
	err := a.db.QueryRowContext(ctx,
		`SELECT password, COALESCE(totp_secret, '') FROM users WHERE id = $1`, userID).
		Scan(&hashedPassword, &totpSecret)
	if err != nil {
		return fmt.Errorf("user not found")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password)); err != nil {
		return fmt.Errorf("invalid password")
	}

	if !totp.Validate(code, totpSecret) {
		return fmt.Errorf("invalid TOTP code")
	}

	_, err = a.db.ExecContext(ctx,
		`UPDATE users SET totp_secret = NULL, totp_enabled = FALSE, totp_backup_codes = NULL WHERE id = $1`,
		userID)
	if err != nil {
		return fmt.Errorf("disable TOTP: %w", err)
	}

	logging.Info("TOTP disabled", zap.Int("user_id", userID))
	return nil
}

// ValidateTOTP validates a TOTP code or a backup code for the user.
func (a *Auth) ValidateTOTP(ctx context.Context, userID int, code string) error {
	var totpSecret string
	var backupCodes []string
	err := a.db.QueryRowContext(ctx,
		`SELECT COALESCE(totp_secret, ''), COALESCE(totp_backup_codes, '{}') FROM users WHERE id = $1`, userID).
		Scan(&totpSecret, pq.Array(&backupCodes))
	if err != nil {
		return fmt.Errorf("user not found")
	}

	// Try TOTP first
	if totp.Validate(code, totpSecret) {
		return nil
	}

	// Try backup codes
	for i, hashed := range backupCodes {
		if bcrypt.CompareHashAndPassword([]byte(hashed), []byte(code)) == nil {
			// Consume the backup code (remove from array)
			remaining := make([]string, 0, len(backupCodes)-1)
			remaining = append(remaining, backupCodes[:i]...)
			remaining = append(remaining, backupCodes[i+1:]...)
			a.db.ExecContext(ctx,
				`UPDATE users SET totp_backup_codes = $1 WHERE id = $2`,
				pq.Array(remaining), userID)
			logging.Info("backup code used", zap.Int("user_id", userID), zap.Int("remaining", len(remaining)))
			return nil
		}
	}

	return fmt.Errorf("invalid code")
}

// IsTOTPEnabled checks whether the user has TOTP enabled.
func (a *Auth) IsTOTPEnabled(ctx context.Context, userID int) (bool, error) {
	var enabled bool
	err := a.db.QueryRowContext(ctx,
		`SELECT totp_enabled FROM users WHERE id = $1`, userID).Scan(&enabled)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return enabled, err
}

// RegenerateBackupCodes generates new backup codes, replacing existing ones.
// Returns the plaintext codes.
func (a *Auth) RegenerateBackupCodes(ctx context.Context, userID int) ([]string, error) {
	plainCodes, hashedCodes, err := generateBackupCodes(10)
	if err != nil {
		return nil, fmt.Errorf("generate backup codes: %w", err)
	}

	_, err = a.db.ExecContext(ctx,
		`UPDATE users SET totp_backup_codes = $1 WHERE id = $2`,
		pq.Array(hashedCodes), userID)
	if err != nil {
		return nil, fmt.Errorf("save backup codes: %w", err)
	}

	logging.Info("backup codes regenerated", zap.Int("user_id", userID))
	return plainCodes, nil
}

// GenerateTOTPTempToken generates a short-lived JWT for the 2FA verification step.
func (a *Auth) GenerateTOTPTempToken(userID int, username string, isAdmin bool) (string, error) {
	now := time.Now()
	claims := &Claims{
		UserID:   userID,
		Username: username,
		IsAdmin:  isAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    "fruitsalade-totp",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(a.secret)
}

// ValidateTOTPTempToken validates a temp token issued for TOTP verification.
func (a *Auth) ValidateTOTPTempToken(tokenStr string) (*Claims, error) {
	claims, err := a.validateToken(tokenStr)
	if err != nil {
		return nil, err
	}
	if claims.Issuer != "fruitsalade-totp" {
		return nil, fmt.Errorf("not a TOTP temp token")
	}
	return claims, nil
}

// generateBackupCodes creates n random 8-char alphanumeric codes.
// Returns (plaintext, bcrypt-hashed).
func generateBackupCodes(n int) ([]string, []string, error) {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	plain := make([]string, n)
	hashed := make([]string, n)

	for i := 0; i < n; i++ {
		b := make([]byte, 8)
		if _, err := rand.Read(b); err != nil {
			return nil, nil, err
		}
		code := make([]byte, 8)
		for j := range code {
			code[j] = chars[int(b[j])%len(chars)]
		}
		plain[i] = string(code)

		h, err := bcrypt.GenerateFromPassword(code, bcrypt.DefaultCost)
		if err != nil {
			return nil, nil, err
		}
		hashed[i] = string(h)
	}

	return plain, hashed, nil
}
