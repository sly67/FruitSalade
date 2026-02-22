// Package auth provides JWT-based authentication middleware with metrics.
package auth

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"github.com/fruitsalade/fruitsalade/fruitsalade/internal/logging"
	"github.com/fruitsalade/fruitsalade/fruitsalade/internal/metrics"
	"github.com/fruitsalade/fruitsalade/shared/pkg/protocol"
)

type contextKey string

const (
	userContextKey contextKey = "user"
)

// Claims holds JWT token claims.
type Claims struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	IsAdmin  bool   `json:"is_admin"`
	jwt.RegisteredClaims
}

// Auth handles JWT authentication.
type Auth struct {
	db     *sql.DB
	secret []byte
	oidc   *OIDCProvider
}

// New creates a new Auth handler.
func New(db *sql.DB, jwtSecret string) *Auth {
	return &Auth{
		db:     db,
		secret: []byte(jwtSecret),
	}
}

// Middleware returns HTTP middleware that validates JWT tokens.
func (a *Auth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenStr := extractToken(r)
		if tokenStr == "" {
			metrics.RecordAuthAttempt(false)
			sendAuthError(w, http.StatusUnauthorized, "missing authentication token")
			return
		}

		claims, err := a.validateToken(tokenStr)
		if err != nil {
			metrics.RecordAuthAttempt(false)
			sendAuthError(w, http.StatusUnauthorized, "invalid token: "+err.Error())
			return
		}

		// Check if token is revoked
		revoked, err := a.isTokenRevoked(r.Context(), tokenStr)
		if err != nil {
			logging.Error("token revocation check failed", zap.Error(err))
		}
		if revoked {
			metrics.RecordAuthAttempt(false)
			sendAuthError(w, http.StatusUnauthorized, "token has been revoked")
			return
		}

		// Store claims in context
		ctx := context.WithValue(r.Context(), userContextKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetClaims extracts claims from the request context.
func GetClaims(ctx context.Context) *Claims {
	claims, _ := ctx.Value(userContextKey).(*Claims)
	return claims
}

// HandleLogin handles POST /api/v1/auth/token
func (a *Auth) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username   string `json:"username"`
		Password   string `json:"password"`
		DeviceName string `json:"device_name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		metrics.RecordAuthAttempt(false)
		sendAuthError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Username == "" || req.Password == "" {
		metrics.RecordAuthAttempt(false)
		sendAuthError(w, http.StatusBadRequest, "username and password required")
		return
	}

	// Look up user
	var userID int
	var hashedPassword string
	var isAdmin bool
	err := a.db.QueryRowContext(r.Context(),
		`SELECT id, password, is_admin FROM users WHERE username = $1`,
		req.Username).Scan(&userID, &hashedPassword, &isAdmin)
	if err == sql.ErrNoRows {
		metrics.RecordAuthAttempt(false)
		logging.Warn("login failed: unknown user", zap.String("username", req.Username))
		sendAuthError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if err != nil {
		metrics.RecordAuthAttempt(false)
		logging.Error("login database error", zap.Error(err))
		sendAuthError(w, http.StatusInternalServerError, "database error")
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(req.Password)); err != nil {
		metrics.RecordAuthAttempt(false)
		logging.Warn("login failed: invalid password", zap.String("username", req.Username))
		sendAuthError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	// Check if TOTP is enabled — if so, return a temp token for 2FA verification
	totpEnabled, _ := a.IsTOTPEnabled(r.Context(), userID)
	if totpEnabled {
		tempToken, err := a.GenerateTOTPTempToken(userID, req.Username, isAdmin)
		if err != nil {
			logging.Error("failed to generate TOTP temp token", zap.Error(err))
			sendAuthError(w, http.StatusInternalServerError, "failed to generate token")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"requires_2fa": true,
			"totp_token":   tempToken,
		})
		return
	}

	// Generate JWT
	now := time.Now()
	claims := &Claims{
		UserID:   userID,
		Username: req.Username,
		IsAdmin:  isAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(30 * 24 * time.Hour)), // 30 days
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    "fruitsalade",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString(a.secret)
	if err != nil {
		metrics.RecordAuthAttempt(false)
		logging.Error("failed to sign token", zap.Error(err))
		sendAuthError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	// Record device token
	deviceName := req.DeviceName
	if deviceName == "" {
		deviceName = "unknown"
	}
	tokenHash := hashToken(tokenStr)
	_, err = a.db.ExecContext(r.Context(),
		`INSERT INTO device_tokens (user_id, device_name, token_hash) VALUES ($1, $2, $3)`,
		userID, deviceName, tokenHash)
	if err != nil {
		logging.Error("failed to record device token", zap.Error(err))
	}

	metrics.RecordAuthAttempt(true)
	logging.Info("login successful",
		zap.String("username", req.Username),
		zap.String("device", deviceName))

	// Update active token count
	a.updateActiveTokenCount(r.Context())

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token":      tokenStr,
		"expires_at": claims.ExpiresAt.Time,
		"user": map[string]interface{}{
			"id":       userID,
			"username": req.Username,
			"is_admin": isAdmin,
		},
	})
}

// CreateUser creates a new user (admin only, or first user).
func (a *Auth) CreateUser(ctx context.Context, username, password string, isAdmin bool) error {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	_, err = a.db.ExecContext(ctx,
		`INSERT INTO users (username, password, is_admin) VALUES ($1, $2, $3)`,
		username, string(hashed), isAdmin)
	if err != nil {
		return fmt.Errorf("insert user: %w", err)
	}

	logging.Info("user created", zap.String("username", username), zap.Bool("is_admin", isAdmin))
	return nil
}

// EnsureDefaultAdmin creates a default admin user if no users exist.
func (a *Auth) EnsureDefaultAdmin(ctx context.Context) error {
	var count int
	err := a.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count)
	if err != nil {
		return fmt.Errorf("count users: %w", err)
	}

	if count == 0 {
		logging.Warn("no users found, creating default admin (admin/admin)")
		logging.Warn("** change the default password immediately! **")
		return a.CreateUser(ctx, "admin", "admin", true)
	}
	return nil
}

// IssueToken generates a full JWT and records a device token entry.
// Used by TOTP verify after 2FA validation completes.
func (a *Auth) IssueToken(ctx context.Context, userID int, username string, isAdmin bool, deviceName string) (string, time.Time, error) {
	now := time.Now()
	claims := &Claims{
		UserID:   userID,
		Username: username,
		IsAdmin:  isAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(30 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    "fruitsalade",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString(a.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign token: %w", err)
	}

	if deviceName == "" {
		deviceName = "unknown"
	}
	tokenHash := hashToken(tokenStr)
	_, err = a.db.ExecContext(ctx,
		`INSERT INTO device_tokens (user_id, device_name, token_hash) VALUES ($1, $2, $3)`,
		userID, deviceName, tokenHash)
	if err != nil {
		logging.Error("failed to record device token", zap.Error(err))
	}

	a.updateActiveTokenCount(ctx)

	return tokenStr, claims.ExpiresAt.Time, nil
}

func (a *Auth) validateToken(tokenStr string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return a.secret, nil
	})

	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}

func (a *Auth) isTokenRevoked(ctx context.Context, tokenStr string) (bool, error) {
	h := hashToken(tokenStr)
	var revoked bool
	err := a.db.QueryRowContext(ctx,
		`SELECT revoked FROM device_tokens WHERE token_hash = $1`, h).Scan(&revoked)
	if err == sql.ErrNoRows {
		return false, nil // Token not tracked = not revoked
	}
	if err != nil {
		return false, err
	}
	return revoked, nil
}

func (a *Auth) updateActiveTokenCount(ctx context.Context) {
	var count int64
	err := a.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM device_tokens WHERE revoked = false`).Scan(&count)
	if err == nil {
		metrics.SetActiveTokens(count)
	}
}

func extractToken(r *http.Request) string {
	// Bearer token from Authorization header
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	// Query parameter fallback
	return r.URL.Query().Get("token")
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// User represents a user account.
type User struct {
	ID        int       `json:"id"`
	Username  string    `json:"username"`
	IsAdmin   bool      `json:"is_admin"`
	CreatedAt time.Time `json:"created_at"`
}

// HasOIDC returns true if an OIDC provider is configured.
func (a *Auth) HasOIDC() bool {
	return a.oidc != nil
}

// OIDCConfig returns the OIDC configuration, or nil if OIDC is not configured.
func (a *Auth) OIDCConfig() *OIDCConfig {
	if a.oidc == nil {
		return nil
	}
	cfg := a.oidc.config
	return &cfg
}

// DB returns the underlying database connection.
func (a *Auth) DB() *sql.DB {
	return a.db
}

// ListUsers returns all users ordered by ID.
func (a *Auth) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := a.db.QueryContext(ctx,
		`SELECT id, username, is_admin, created_at FROM users ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.IsAdmin, &u.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// DeleteUser deletes a user by ID.
func (a *Auth) DeleteUser(ctx context.Context, userID int) error {
	result, err := a.db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("user not found")
	}
	logging.Info("user deleted", zap.Int("user_id", userID))
	return nil
}

// ChangePassword changes the password for a user.
func (a *Auth) ChangePassword(ctx context.Context, userID int, newPassword string) error {
	hashed, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	result, err := a.db.ExecContext(ctx,
		`UPDATE users SET password = $1 WHERE id = $2`, string(hashed), userID)
	if err != nil {
		return fmt.Errorf("change password: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("user not found")
	}
	logging.Info("password changed", zap.Int("user_id", userID))
	return nil
}

// UserCount returns the total number of users.
func (a *Auth) UserCount(ctx context.Context) (int64, error) {
	var count int64
	err := a.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count)
	return count, err
}

// ActiveSessionCount returns the number of non-revoked device tokens.
func (a *Auth) ActiveSessionCount(ctx context.Context) (int64, error) {
	var count int64
	err := a.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM device_tokens WHERE revoked = FALSE`).Scan(&count)
	return count, err
}

// DeviceToken represents a device session.
type DeviceToken struct {
	ID         int       `json:"id"`
	DeviceName string    `json:"device_name"`
	CreatedAt  time.Time `json:"created_at"`
	LastUsed   time.Time `json:"last_used"`
	Revoked    bool      `json:"revoked"`
}

// ListSessions returns all device tokens for a user.
func (a *Auth) ListSessions(ctx context.Context, userID int) ([]DeviceToken, error) {
	rows, err := a.db.QueryContext(ctx,
		`SELECT id, device_name, created_at, COALESCE(last_used, created_at), revoked
		 FROM device_tokens WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()

	var tokens []DeviceToken
	for rows.Next() {
		var t DeviceToken
		if err := rows.Scan(&t.ID, &t.DeviceName, &t.CreatedAt, &t.LastUsed, &t.Revoked); err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}
		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
}

// RevokeToken revokes the current token by its hash.
func (a *Auth) RevokeToken(ctx context.Context, tokenStr string) error {
	h := hashToken(tokenStr)
	result, err := a.db.ExecContext(ctx,
		`UPDATE device_tokens SET revoked = TRUE WHERE token_hash = $1`, h)
	if err != nil {
		return fmt.Errorf("revoke token: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("token not found")
	}
	a.updateActiveTokenCount(ctx)
	return nil
}

// RevokeSession revokes a specific device token by ID (must belong to userID).
func (a *Auth) RevokeSession(ctx context.Context, userID, tokenID int) error {
	result, err := a.db.ExecContext(ctx,
		`UPDATE device_tokens SET revoked = TRUE WHERE id = $1 AND user_id = $2`, tokenID, userID)
	if err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("session not found")
	}
	a.updateActiveTokenCount(ctx)
	return nil
}

// RefreshToken generates a new token from a valid existing one.
// The old token is revoked. Returns the new token string and expiry.
func (a *Auth) RefreshToken(ctx context.Context, oldTokenStr string) (string, time.Time, error) {
	claims, err := a.validateToken(oldTokenStr)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("invalid token: %w", err)
	}

	// Check if old token is revoked
	revoked, err := a.isTokenRevoked(ctx, oldTokenStr)
	if err != nil {
		return "", time.Time{}, err
	}
	if revoked {
		return "", time.Time{}, fmt.Errorf("token has been revoked")
	}

	// Re-verify user still exists
	var isAdmin bool
	err = a.db.QueryRowContext(ctx,
		`SELECT is_admin FROM users WHERE id = $1`, claims.UserID).Scan(&isAdmin)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("user not found")
	}

	// Generate new token
	now := time.Now()
	newClaims := &Claims{
		UserID:   claims.UserID,
		Username: claims.Username,
		IsAdmin:  isAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(30 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    "fruitsalade",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, newClaims)
	newTokenStr, err := token.SignedString(a.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign token: %w", err)
	}

	// Revoke old token
	a.RevokeToken(ctx, oldTokenStr)

	// Get device name from old token
	oldHash := hashToken(oldTokenStr)
	var deviceName string
	a.db.QueryRowContext(ctx,
		`SELECT device_name FROM device_tokens WHERE token_hash = $1`, oldHash).Scan(&deviceName)
	if deviceName == "" {
		deviceName = "refreshed"
	}

	// Record new token
	newHash := hashToken(newTokenStr)
	a.db.ExecContext(ctx,
		`INSERT INTO device_tokens (user_id, device_name, token_hash) VALUES ($1, $2, $3)`,
		claims.UserID, deviceName, newHash)

	a.updateActiveTokenCount(ctx)

	logging.Info("token refreshed", zap.Int("user_id", claims.UserID))
	return newTokenStr, newClaims.ExpiresAt.Time, nil
}

// ValidateCredentials checks username/password and returns claims without HTTP.
// Used by WebDAV Basic Auth.
func (a *Auth) ValidateCredentials(ctx context.Context, username, password string) (*Claims, error) {
	var userID int
	var hashedPassword string
	var isAdmin bool
	err := a.db.QueryRowContext(ctx,
		`SELECT id, password, is_admin FROM users WHERE username = $1`,
		username).Scan(&userID, &hashedPassword, &isAdmin)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("invalid credentials")
	}
	if err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password)); err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	return &Claims{
		UserID:   userID,
		Username: username,
		IsAdmin:  isAdmin,
	}, nil
}

// WithClaims injects claims into a context.
func WithClaims(ctx context.Context, claims *Claims) context.Context {
	return context.WithValue(ctx, userContextKey, claims)
}

func sendAuthError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(protocol.ErrorResponse{
		Error: message,
		Code:  code,
	})
}
