package sharing

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/fruitsalade/fruitsalade/phase2/internal/metrics"
)

// ShareLink represents a file share link.
type ShareLink struct {
	ID            string
	Path          string
	CreatedBy     int
	ExpiresAt     *time.Time
	PasswordHash  string
	MaxDownloads  int
	DownloadCount int
	IsActive      bool
	CreatedAt     time.Time
}

// ShareLinkStore manages share links.
type ShareLinkStore struct {
	db *sql.DB
}

// NewShareLinkStore creates a new share link store.
func NewShareLinkStore(db *sql.DB) *ShareLinkStore {
	return &ShareLinkStore{db: db}
}

// Create creates a new share link.
func (s *ShareLinkStore) Create(ctx context.Context, path string, createdBy int, password string, expiresInSec int64, maxDownloads int) (*ShareLink, error) {
	id, err := generateToken()
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	var passwordHash sql.NullString
	if password != "" {
		hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("hash password: %w", err)
		}
		passwordHash = sql.NullString{String: string(hashed), Valid: true}
	}

	var expiresAt *time.Time
	if expiresInSec > 0 {
		t := time.Now().Add(time.Duration(expiresInSec) * time.Second)
		expiresAt = &t
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO share_links (id, path, created_by, expires_at, password_hash, max_downloads)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		id, path, createdBy, expiresAt, passwordHash, maxDownloads)
	if err != nil {
		return nil, fmt.Errorf("insert share link: %w", err)
	}

	link := &ShareLink{
		ID:           id,
		Path:         path,
		CreatedBy:    createdBy,
		ExpiresAt:    expiresAt,
		MaxDownloads: maxDownloads,
		IsActive:     true,
		CreatedAt:    time.Now(),
	}

	s.updateActiveCount(ctx)
	return link, nil
}

// Validate checks if a share link is valid and returns it.
// Checks: exists, active, not expired, download limit not reached.
func (s *ShareLinkStore) Validate(ctx context.Context, id string, password string) (*ShareLink, error) {
	var link ShareLink
	var expiresAt sql.NullTime
	var passwordHash sql.NullString

	err := s.db.QueryRowContext(ctx,
		`SELECT id, path, created_by, expires_at, password_hash, max_downloads, download_count, is_active, created_at
		 FROM share_links WHERE id = $1`, id).
		Scan(&link.ID, &link.Path, &link.CreatedBy, &expiresAt, &passwordHash,
			&link.MaxDownloads, &link.DownloadCount, &link.IsActive, &link.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("share link not found")
	}
	if err != nil {
		return nil, fmt.Errorf("query share link: %w", err)
	}

	if expiresAt.Valid {
		link.ExpiresAt = &expiresAt.Time
	}
	if passwordHash.Valid {
		link.PasswordHash = passwordHash.String
	}

	if !link.IsActive {
		return nil, fmt.Errorf("share link has been revoked")
	}

	if link.ExpiresAt != nil && time.Now().After(*link.ExpiresAt) {
		return nil, fmt.Errorf("share link has expired")
	}

	if link.MaxDownloads > 0 && link.DownloadCount >= link.MaxDownloads {
		return nil, fmt.Errorf("share link download limit reached")
	}

	// Check password if required
	if link.PasswordHash != "" {
		if password == "" {
			return nil, fmt.Errorf("password required")
		}
		if err := bcrypt.CompareHashAndPassword([]byte(link.PasswordHash), []byte(password)); err != nil {
			return nil, fmt.Errorf("invalid password")
		}
	}

	return &link, nil
}

// IncrementDownloads increments the download count for a share link.
func (s *ShareLinkStore) IncrementDownloads(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE share_links SET download_count = download_count + 1 WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("increment downloads: %w", err)
	}
	metrics.RecordShareDownload()
	return nil
}

// Revoke deactivates a share link.
func (s *ShareLinkStore) Revoke(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE share_links SET is_active = FALSE WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("revoke share link: %w", err)
	}
	s.updateActiveCount(ctx)
	return nil
}

// GetByID returns a share link by ID (without validation).
func (s *ShareLinkStore) GetByID(ctx context.Context, id string) (*ShareLink, error) {
	var link ShareLink
	var expiresAt sql.NullTime
	var passwordHash sql.NullString

	err := s.db.QueryRowContext(ctx,
		`SELECT id, path, created_by, expires_at, password_hash, max_downloads, download_count, is_active, created_at
		 FROM share_links WHERE id = $1`, id).
		Scan(&link.ID, &link.Path, &link.CreatedBy, &expiresAt, &passwordHash,
			&link.MaxDownloads, &link.DownloadCount, &link.IsActive, &link.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("share link not found")
	}
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}

	if expiresAt.Valid {
		link.ExpiresAt = &expiresAt.Time
	}
	return &link, nil
}

// ShareLinkWithUser represents a share link with the creator's username.
type ShareLinkWithUser struct {
	ID              string     `json:"id"`
	Path            string     `json:"path"`
	CreatedBy       int        `json:"created_by"`
	CreatedByUser   string     `json:"created_by_username"`
	ExpiresAt       *time.Time `json:"expires_at,omitempty"`
	MaxDownloads    int        `json:"max_downloads"`
	DownloadCount   int        `json:"download_count"`
	IsActive        bool       `json:"is_active"`
	CreatedAt       time.Time  `json:"created_at"`
}

// ListAll returns all share links with creator usernames, optionally filtered to active only.
func (s *ShareLinkStore) ListAll(ctx context.Context, activeOnly bool) ([]ShareLinkWithUser, error) {
	query := `SELECT sl.id, sl.path, sl.created_by, u.username, sl.expires_at,
	           sl.max_downloads, sl.download_count, sl.is_active, sl.created_at
	          FROM share_links sl
	          JOIN users u ON u.id = sl.created_by`
	if activeOnly {
		query += ` WHERE sl.is_active = TRUE`
	}
	query += ` ORDER BY sl.created_at DESC`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list share links: %w", err)
	}
	defer rows.Close()

	var links []ShareLinkWithUser
	for rows.Next() {
		var l ShareLinkWithUser
		var expiresAt sql.NullTime
		if err := rows.Scan(&l.ID, &l.Path, &l.CreatedBy, &l.CreatedByUser,
			&expiresAt, &l.MaxDownloads, &l.DownloadCount, &l.IsActive, &l.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan share link: %w", err)
		}
		if expiresAt.Valid {
			l.ExpiresAt = &expiresAt.Time
		}
		links = append(links, l)
	}
	return links, rows.Err()
}

// ListByUser returns active share links created by a specific user.
func (s *ShareLinkStore) ListByUser(ctx context.Context, userID int) ([]ShareLinkWithUser, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT sl.id, sl.path, sl.created_by, u.username, sl.expires_at,
		        sl.max_downloads, sl.download_count, sl.is_active, sl.created_at
		 FROM share_links sl
		 JOIN users u ON u.id = sl.created_by
		 WHERE sl.created_by = $1 AND sl.is_active = TRUE
		 ORDER BY sl.created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list share links by user: %w", err)
	}
	defer rows.Close()

	var links []ShareLinkWithUser
	for rows.Next() {
		var l ShareLinkWithUser
		var expiresAt sql.NullTime
		if err := rows.Scan(&l.ID, &l.Path, &l.CreatedBy, &l.CreatedByUser,
			&expiresAt, &l.MaxDownloads, &l.DownloadCount, &l.IsActive, &l.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan share link: %w", err)
		}
		if expiresAt.Valid {
			l.ExpiresAt = &expiresAt.Time
		}
		links = append(links, l)
	}
	return links, rows.Err()
}

// ListByPath returns active share links for a specific file path.
func (s *ShareLinkStore) ListByPath(ctx context.Context, path string) ([]ShareLinkWithUser, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT sl.id, sl.path, sl.created_by, u.username, sl.expires_at,
		        sl.max_downloads, sl.download_count, sl.is_active, sl.created_at
		 FROM share_links sl
		 JOIN users u ON u.id = sl.created_by
		 WHERE sl.path = $1 AND sl.is_active = TRUE
		 ORDER BY sl.created_at DESC`, path)
	if err != nil {
		return nil, fmt.Errorf("list share links by path: %w", err)
	}
	defer rows.Close()

	var links []ShareLinkWithUser
	for rows.Next() {
		var l ShareLinkWithUser
		var expiresAt sql.NullTime
		if err := rows.Scan(&l.ID, &l.Path, &l.CreatedBy, &l.CreatedByUser,
			&expiresAt, &l.MaxDownloads, &l.DownloadCount, &l.IsActive, &l.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan share link: %w", err)
		}
		if expiresAt.Valid {
			l.ExpiresAt = &expiresAt.Time
		}
		links = append(links, l)
	}
	return links, rows.Err()
}

func (s *ShareLinkStore) updateActiveCount(ctx context.Context) {
	var count int64
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM share_links WHERE is_active = TRUE`).Scan(&count)
	if err == nil {
		metrics.SetShareLinksActive(count)
	}
}

func generateToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
