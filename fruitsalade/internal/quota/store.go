// Package quota provides user quota management and storage tracking.
package quota

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Quota represents a user's quota settings.
type Quota struct {
	UserID             int
	MaxStorageBytes    int64
	MaxBandwidthPerDay int64
	MaxRequestsPerMin  int
	MaxUploadSizeBytes int64
}

// QuotaStore manages user quotas and usage tracking.
type QuotaStore struct {
	db *sql.DB
}

// NewQuotaStore creates a new quota store.
func NewQuotaStore(db *sql.DB) *QuotaStore {
	return &QuotaStore{db: db}
}

// GetQuota returns the quota for a user. Returns zero-value quota if none set.
func (s *QuotaStore) GetQuota(ctx context.Context, userID int) (*Quota, error) {
	q := &Quota{UserID: userID}
	err := s.db.QueryRowContext(ctx,
		`SELECT max_storage_bytes, max_bandwidth_per_day, max_requests_per_minute, max_upload_size_bytes
		 FROM user_quotas WHERE user_id = $1`, userID).
		Scan(&q.MaxStorageBytes, &q.MaxBandwidthPerDay, &q.MaxRequestsPerMin, &q.MaxUploadSizeBytes)
	if err == sql.ErrNoRows {
		return q, nil // Zero-value = unlimited
	}
	if err != nil {
		return nil, fmt.Errorf("get quota: %w", err)
	}
	return q, nil
}

// SetQuota sets or updates the quota for a user.
func (s *QuotaStore) SetQuota(ctx context.Context, q *Quota) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO user_quotas (user_id, max_storage_bytes, max_bandwidth_per_day, max_requests_per_minute, max_upload_size_bytes, updated_at)
		 VALUES ($1, $2, $3, $4, $5, NOW())
		 ON CONFLICT (user_id) DO UPDATE SET
			max_storage_bytes = EXCLUDED.max_storage_bytes,
			max_bandwidth_per_day = EXCLUDED.max_bandwidth_per_day,
			max_requests_per_minute = EXCLUDED.max_requests_per_minute,
			max_upload_size_bytes = EXCLUDED.max_upload_size_bytes,
			updated_at = NOW()`,
		q.UserID, q.MaxStorageBytes, q.MaxBandwidthPerDay, q.MaxRequestsPerMin, q.MaxUploadSizeBytes)
	if err != nil {
		return fmt.Errorf("set quota: %w", err)
	}
	return nil
}

// GetStorageUsed returns the total storage used by a user (sum of owned file sizes).
func (s *QuotaStore) GetStorageUsed(ctx context.Context, userID int) (int64, error) {
	var used sql.NullInt64
	err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(size), 0) FROM files WHERE owner_id = $1 AND is_dir = FALSE`,
		userID).Scan(&used)
	if err != nil {
		return 0, fmt.Errorf("get storage used: %w", err)
	}
	return used.Int64, nil
}

// CheckStorageQuota checks if a user can upload a file of the given size.
func (s *QuotaStore) CheckStorageQuota(ctx context.Context, userID int, additionalBytes int64) (bool, error) {
	q, err := s.GetQuota(ctx, userID)
	if err != nil {
		return false, err
	}
	if q.MaxStorageBytes == 0 {
		return true, nil // Unlimited
	}

	used, err := s.GetStorageUsed(ctx, userID)
	if err != nil {
		return false, err
	}

	return used+additionalBytes <= q.MaxStorageBytes, nil
}

// GetUploadSizeLimit returns the effective upload size limit for a user.
// Returns the user-specific limit if set, otherwise 0 (caller uses global default).
func (s *QuotaStore) GetUploadSizeLimit(ctx context.Context, userID int) (int64, error) {
	q, err := s.GetQuota(ctx, userID)
	if err != nil {
		return 0, err
	}
	return q.MaxUploadSizeBytes, nil
}

// TrackBandwidth records bandwidth usage for a user (daily aggregation).
func (s *QuotaStore) TrackBandwidth(ctx context.Context, userID int, bytesIn, bytesOut int64) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO bandwidth_usage (user_id, date, bytes_in, bytes_out)
		 VALUES ($1, CURRENT_DATE, $2, $3)
		 ON CONFLICT (user_id, date) DO UPDATE SET
			bytes_in = bandwidth_usage.bytes_in + EXCLUDED.bytes_in,
			bytes_out = bandwidth_usage.bytes_out + EXCLUDED.bytes_out`,
		userID, bytesIn, bytesOut)
	if err != nil {
		return fmt.Errorf("track bandwidth: %w", err)
	}
	return nil
}

// GetBandwidthToday returns today's bandwidth usage for a user.
func (s *QuotaStore) GetBandwidthToday(ctx context.Context, userID int) (bytesIn, bytesOut int64, err error) {
	err = s.db.QueryRowContext(ctx,
		`SELECT COALESCE(bytes_in, 0), COALESCE(bytes_out, 0)
		 FROM bandwidth_usage WHERE user_id = $1 AND date = CURRENT_DATE`,
		userID).Scan(&bytesIn, &bytesOut)
	if err == sql.ErrNoRows {
		return 0, 0, nil
	}
	if err != nil {
		return 0, 0, fmt.Errorf("get bandwidth today: %w", err)
	}
	return bytesIn, bytesOut, nil
}

// CheckBandwidthQuota checks if a user has bandwidth remaining today.
func (s *QuotaStore) CheckBandwidthQuota(ctx context.Context, userID int, additionalBytes int64) (bool, error) {
	q, err := s.GetQuota(ctx, userID)
	if err != nil {
		return false, err
	}
	if q.MaxBandwidthPerDay == 0 {
		return true, nil // Unlimited
	}

	bIn, bOut, err := s.GetBandwidthToday(ctx, userID)
	if err != nil {
		return false, err
	}

	totalToday := bIn + bOut
	return totalToday+additionalBytes <= q.MaxBandwidthPerDay, nil
}

// CleanupOldBandwidth removes bandwidth records older than the given duration.
func (s *QuotaStore) CleanupOldBandwidth(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	result, err := s.db.ExecContext(ctx,
		`DELETE FROM bandwidth_usage WHERE date < $1`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("cleanup bandwidth: %w", err)
	}
	return result.RowsAffected()
}
