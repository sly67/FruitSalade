// Package auth provides JWT-based authentication middleware.
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
	"golang.org/x/crypto/bcrypt"

	"github.com/fruitsalade/fruitsalade/shared/pkg/logger"
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
			sendAuthError(w, http.StatusUnauthorized, "missing authentication token")
			return
		}

		claims, err := a.validateToken(tokenStr)
		if err != nil {
			sendAuthError(w, http.StatusUnauthorized, "invalid token: "+err.Error())
			return
		}

		// Check if token is revoked
		revoked, err := a.isTokenRevoked(r.Context(), tokenStr)
		if err != nil {
			logger.Error("Token revocation check failed: %v", err)
		}
		if revoked {
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
		sendAuthError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Username == "" || req.Password == "" {
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
		sendAuthError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if err != nil {
		sendAuthError(w, http.StatusInternalServerError, "database error")
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(req.Password)); err != nil {
		sendAuthError(w, http.StatusUnauthorized, "invalid credentials")
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
		logger.Error("Failed to record device token: %v", err)
	}

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
		logger.Info("No users found, creating default admin (admin/admin)")
		logger.Info("** Change the default password immediately! **")
		return a.CreateUser(ctx, "admin", "admin", true)
	}
	return nil
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

func sendAuthError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(protocol.ErrorResponse{
		Error: message,
		Code:  code,
	})
}
