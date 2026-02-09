package auth

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"

	"github.com/fruitsalade/fruitsalade/phase2/internal/logging"
	"github.com/fruitsalade/fruitsalade/phase2/internal/metrics"
)

// OIDCConfig holds OIDC provider configuration.
type OIDCConfig struct {
	IssuerURL    string // e.g. https://keycloak.example.com/realms/fruitsalade
	ClientID     string
	ClientSecret string
	AdminClaim   string // claim key for admin status (default: "is_admin")
	AdminValue   string // claim value that indicates admin (default: "true")
}

// OIDCProvider validates OIDC tokens and auto-creates local users.
type OIDCProvider struct {
	verifier *oidc.IDTokenVerifier
	config   OIDCConfig
	auth     *Auth
}

// NewOIDCProvider creates an OIDC provider from config.
// Returns nil if IssuerURL is empty (OIDC disabled).
func NewOIDCProvider(ctx context.Context, cfg OIDCConfig, a *Auth) (*OIDCProvider, error) {
	if cfg.IssuerURL == "" {
		return nil, nil
	}

	provider, err := oidc.NewProvider(ctx, cfg.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("oidc provider init: %w", err)
	}

	verifier := provider.Verifier(&oidc.Config{ClientID: cfg.ClientID})

	if cfg.AdminClaim == "" {
		cfg.AdminClaim = "is_admin"
	}
	if cfg.AdminValue == "" {
		cfg.AdminValue = "true"
	}

	logging.Info("OIDC provider initialized",
		zap.String("issuer", cfg.IssuerURL),
		zap.String("client_id", cfg.ClientID))

	return &OIDCProvider{
		verifier: verifier,
		config:   cfg,
		auth:     a,
	}, nil
}

// ValidateToken attempts to verify a token as an OIDC ID token.
// If valid, ensures the user exists locally and returns local Claims.
func (o *OIDCProvider) ValidateToken(ctx context.Context, tokenStr string) (*Claims, error) {
	idToken, err := o.verifier.Verify(ctx, tokenStr)
	if err != nil {
		return nil, err
	}

	// Extract standard claims
	var oidcClaims struct {
		Sub               string `json:"sub"`
		PreferredUsername  string `json:"preferred_username"`
		Email             string `json:"email"`
		Name              string `json:"name"`
	}
	if err := idToken.Claims(&oidcClaims); err != nil {
		return nil, fmt.Errorf("parse oidc claims: %w", err)
	}

	// Determine username: prefer preferred_username, fallback to email, then sub
	username := oidcClaims.PreferredUsername
	if username == "" {
		username = oidcClaims.Email
	}
	if username == "" {
		username = oidcClaims.Sub
	}

	// Check admin claim from raw claims
	var rawClaims map[string]interface{}
	idToken.Claims(&rawClaims)
	isAdmin := false
	if val, ok := rawClaims[o.config.AdminClaim]; ok {
		isAdmin = fmt.Sprintf("%v", val) == o.config.AdminValue
	}

	// Ensure user exists locally (auto-create on first OIDC login)
	userID, err := o.ensureUser(ctx, username, isAdmin)
	if err != nil {
		return nil, fmt.Errorf("ensure user: %w", err)
	}

	return &Claims{
		UserID:   userID,
		Username: username,
		IsAdmin:  isAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: oidcClaims.Sub,
			Issuer:  idToken.Issuer,
		},
	}, nil
}

func (o *OIDCProvider) ensureUser(ctx context.Context, username string, isAdmin bool) (int, error) {
	var userID int
	err := o.auth.db.QueryRowContext(ctx,
		`SELECT id FROM users WHERE username = $1`, username).Scan(&userID)
	if err == nil {
		return userID, nil
	}
	if err != sql.ErrNoRows {
		return 0, err
	}

	// Auto-create user (random password since they authenticate via OIDC)
	err = o.auth.db.QueryRowContext(ctx,
		`INSERT INTO users (username, password, is_admin) VALUES ($1, $2, $3) RETURNING id`,
		username, "oidc-managed", isAdmin).Scan(&userID)
	if err != nil {
		return 0, fmt.Errorf("create oidc user: %w", err)
	}

	logging.Info("auto-created OIDC user", zap.String("username", username), zap.Bool("is_admin", isAdmin))
	return userID, nil
}

// SetOIDCProvider sets the OIDC provider on the Auth handler.
func (a *Auth) SetOIDCProvider(p *OIDCProvider) {
	a.oidc = p
}

// MiddlewareWithOIDC returns middleware that tries JWT first, then OIDC.
func (a *Auth) MiddlewareWithOIDC(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenStr := extractToken(r)
		if tokenStr == "" {
			metrics.RecordAuthAttempt(false)
			sendAuthError(w, http.StatusUnauthorized, "missing authentication token")
			return
		}

		// Try local JWT first
		claims, err := a.validateToken(tokenStr)
		if err == nil {
			revoked, rerr := a.isTokenRevoked(r.Context(), tokenStr)
			if rerr != nil {
				logging.Error("token revocation check failed", zap.Error(rerr))
			}
			if !revoked {
				ctx := context.WithValue(r.Context(), userContextKey, claims)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}

		// Try OIDC if configured
		if a.oidc != nil {
			claims, err = a.oidc.ValidateToken(r.Context(), tokenStr)
			if err == nil {
				metrics.RecordAuthAttempt(true)
				ctx := context.WithValue(r.Context(), userContextKey, claims)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}

		metrics.RecordAuthAttempt(false)
		sendAuthError(w, http.StatusUnauthorized, "invalid token")
	})
}
