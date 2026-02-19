package gallery

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// SearchParams holds query parameters for gallery search.
type SearchParams struct {
	Query       string
	DateFrom    *time.Time
	DateTo      *time.Time
	Tags        []string
	CameraMake  string
	CameraModel string
	Country     string
	City        string
	SortBy      string // "date", "name", "size"
	SortOrder   string // "asc", "desc"
	Limit       int
	Offset      int

	// Permission context
	UserID       int
	UserGroupIDs []int
	UserPermPaths []string
	IsAdmin      bool
}

// SearchResult holds a single search result row.
type SearchResult struct {
	FilePath        string
	FileName        string
	Size            int64
	ModTime         time.Time
	Hash            string
	Width           int
	Height          int
	CameraMake      string
	CameraModel     string
	DateTaken       *time.Time
	Latitude        *float64
	Longitude       *float64
	LocationCity    string
	LocationCountry string
	HasThumbnail    bool
}

// PermFilter holds a SQL WHERE fragment and its arguments for permission filtering.
type PermFilter struct {
	Condition string        // SQL fragment like "(f.owner_id = $1 OR ...)"
	Args      []interface{} // positional arguments
	ArgCount  int           // number of args consumed
}

// BuildPermFilter creates a permission filter for gallery queries.
// argStart is the first $N placeholder to use. Returns nil if isAdmin.
func BuildPermFilter(argStart int, userID int, groupIDs []int, permPaths []string, isAdmin bool) *PermFilter {
	if isAdmin {
		return nil
	}

	n := argStart
	var args []interface{}

	cond := fmt.Sprintf(`(
		f.owner_id = $%d
		OR f.visibility = 'public'
		OR (f.visibility = 'group' AND f.group_id = ANY($%d))
	)`, n, n+1)
	args = append(args, userID, groupIDs)
	n += 2

	if len(permPaths) > 0 {
		cond = fmt.Sprintf(`(
			f.owner_id = $%d
			OR f.visibility = 'public'
			OR (f.visibility = 'group' AND f.group_id = ANY($%d))
			OR f.path = ANY($%d)
		)`, n-2, n-1, n)
		args = append(args, permPaths)
		n++
	}

	return &PermFilter{
		Condition: cond,
		Args:      args,
		ArgCount:  n - argStart,
	}
}

// Search performs a permission-filtered gallery search.
func (s *GalleryStore) Search(ctx context.Context, p *SearchParams) ([]SearchResult, int, error) {
	if p.Limit <= 0 || p.Limit > 200 {
		p.Limit = 50
	}
	if p.SortOrder != "asc" {
		p.SortOrder = "desc"
	}

	// Build WHERE clauses
	var conditions []string
	var args []interface{}
	argN := 1

	conditions = append(conditions, "f.is_dir = FALSE")
	conditions = append(conditions, "im.status = 'done'")

	// Permission filter
	if !p.IsAdmin {
		pf := BuildPermFilter(argN, p.UserID, p.UserGroupIDs, p.UserPermPaths, false)
		conditions = append(conditions, pf.Condition)
		args = append(args, pf.Args...)
		argN += pf.ArgCount
	}

	// Text search (file name)
	if p.Query != "" {
		conditions = append(conditions, fmt.Sprintf("LOWER(f.name) LIKE $%d", argN))
		args = append(args, "%"+strings.ToLower(p.Query)+"%")
		argN++
	}

	// Date range
	if p.DateFrom != nil {
		conditions = append(conditions, fmt.Sprintf("im.date_taken >= $%d", argN))
		args = append(args, *p.DateFrom)
		argN++
	}
	if p.DateTo != nil {
		conditions = append(conditions, fmt.Sprintf("im.date_taken <= $%d", argN))
		args = append(args, *p.DateTo)
		argN++
	}

	// Camera filter
	if p.CameraMake != "" {
		conditions = append(conditions, fmt.Sprintf("im.camera_make = $%d", argN))
		args = append(args, p.CameraMake)
		argN++
	}
	if p.CameraModel != "" {
		conditions = append(conditions, fmt.Sprintf("im.camera_model = $%d", argN))
		args = append(args, p.CameraModel)
		argN++
	}

	// Location filter
	if p.Country != "" {
		conditions = append(conditions, fmt.Sprintf("im.location_country = $%d", argN))
		args = append(args, p.Country)
		argN++
	}
	if p.City != "" {
		conditions = append(conditions, fmt.Sprintf("im.location_city = $%d", argN))
		args = append(args, p.City)
		argN++
	}

	// Tag filter — require all specified tags
	if len(p.Tags) > 0 {
		for _, tag := range p.Tags {
			conditions = append(conditions, fmt.Sprintf(
				"EXISTS (SELECT 1 FROM image_tags it WHERE it.file_path = f.path AND it.tag = $%d)", argN))
			args = append(args, tag)
			argN++
		}
	}

	where := strings.Join(conditions, " AND ")

	// Count total
	countQuery := fmt.Sprintf(`
		SELECT COUNT(*) FROM files f
		JOIN image_metadata im ON im.file_path = f.path
		WHERE %s`, where)

	var total int
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count: %w", err)
	}

	// Sort
	orderCol := "im.date_taken"
	switch p.SortBy {
	case "name":
		orderCol = "f.name"
	case "size":
		orderCol = "f.size"
	}
	nullsLast := ""
	if orderCol == "im.date_taken" {
		nullsLast = " NULLS LAST"
	}

	// Select
	selectQuery := fmt.Sprintf(`
		SELECT f.path, f.name, f.size, f.mod_time, f.hash,
			im.width, im.height, im.camera_make, im.camera_model,
			im.date_taken, im.latitude, im.longitude,
			im.location_city, im.location_country, im.has_thumbnail
		FROM files f
		JOIN image_metadata im ON im.file_path = f.path
		WHERE %s
		ORDER BY %s %s%s
		LIMIT $%d OFFSET $%d`,
		where, orderCol, p.SortOrder, nullsLast, argN, argN+1)

	args = append(args, p.Limit, p.Offset)

	rows, err := s.db.QueryContext(ctx, selectQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("search: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(
			&r.FilePath, &r.FileName, &r.Size, &r.ModTime, &r.Hash,
			&r.Width, &r.Height, &r.CameraMake, &r.CameraModel,
			&r.DateTaken, &r.Latitude, &r.Longitude,
			&r.LocationCity, &r.LocationCountry, &r.HasThumbnail,
		); err != nil {
			return nil, 0, fmt.Errorf("scan: %w", err)
		}
		results = append(results, r)
	}

	return results, total, rows.Err()
}

// ─── Album Queries ──────────────────────────────────────────────────────────

// DateAlbumRow holds a year/month/count grouping.
type DateAlbumRow struct {
	Year  int
	Month int
	Count int
}

// GetAlbumsByDate returns images grouped by year and month.
func (s *GalleryStore) GetAlbumsByDate(ctx context.Context, pf *PermFilter) ([]DateAlbumRow, error) {
	var args []interface{}
	join := ""
	permWhere := ""
	if pf != nil {
		join = " JOIN files f ON f.path = im.file_path"
		permWhere = " AND " + pf.Condition
		args = pf.Args
	}

	query := fmt.Sprintf(`
		SELECT EXTRACT(YEAR FROM im.date_taken)::INT, EXTRACT(MONTH FROM im.date_taken)::INT, COUNT(*)
		FROM image_metadata im%s
		WHERE im.date_taken IS NOT NULL AND im.status = 'done'%s
		GROUP BY 1, 2
		ORDER BY 1 DESC, 2 DESC`, join, permWhere)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []DateAlbumRow
	for rows.Next() {
		var r DateAlbumRow
		if err := rows.Scan(&r.Year, &r.Month, &r.Count); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// LocationAlbumRow holds a country/city/count grouping.
type LocationAlbumRow struct {
	Country string
	City    string
	Count   int
}

// GetAlbumsByLocation returns images grouped by country and city.
func (s *GalleryStore) GetAlbumsByLocation(ctx context.Context, pf *PermFilter) ([]LocationAlbumRow, error) {
	var args []interface{}
	join := ""
	permWhere := ""
	if pf != nil {
		join = " JOIN files f ON f.path = im.file_path"
		permWhere = " AND " + pf.Condition
		args = pf.Args
	}

	query := fmt.Sprintf(`
		SELECT COALESCE(im.location_country, 'Unknown'), COALESCE(im.location_city, 'Unknown'), COUNT(*)
		FROM image_metadata im%s
		WHERE (im.location_country IS NOT NULL OR im.location_city IS NOT NULL) AND im.status = 'done'%s
		GROUP BY 1, 2
		ORDER BY 1, 3 DESC`, join, permWhere)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []LocationAlbumRow
	for rows.Next() {
		var r LocationAlbumRow
		if err := rows.Scan(&r.Country, &r.City, &r.Count); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// CameraAlbumRow holds a make/model/count grouping.
type CameraAlbumRow struct {
	Make  string
	Model string
	Count int
}

// GetAlbumsByCamera returns images grouped by camera make and model.
func (s *GalleryStore) GetAlbumsByCamera(ctx context.Context, pf *PermFilter) ([]CameraAlbumRow, error) {
	var args []interface{}
	join := ""
	permWhere := ""
	if pf != nil {
		join = " JOIN files f ON f.path = im.file_path"
		permWhere = " AND " + pf.Condition
		args = pf.Args
	}

	query := fmt.Sprintf(`
		SELECT COALESCE(im.camera_make, 'Unknown'), COALESCE(im.camera_model, 'Unknown'), COUNT(*)
		FROM image_metadata im%s
		WHERE (im.camera_make IS NOT NULL OR im.camera_model IS NOT NULL) AND im.status = 'done'%s
		GROUP BY 1, 2
		ORDER BY 1, 3 DESC`, join, permWhere)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []CameraAlbumRow
	for rows.Next() {
		var r CameraAlbumRow
		if err := rows.Scan(&r.Make, &r.Model, &r.Count); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// GetTagsForFiles batch-loads tags for a set of file paths.
func (s *GalleryStore) GetTagsForFiles(ctx context.Context, paths []string) (map[string][]string, error) {
	if len(paths) == 0 {
		return nil, nil
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT file_path, tag FROM image_tags WHERE file_path = ANY($1) ORDER BY file_path, tag`,
		paths)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]string)
	for rows.Next() {
		var fp, tag string
		if err := rows.Scan(&fp, &tag); err != nil {
			return nil, err
		}
		result[fp] = append(result[fp], tag)
	}
	return result, rows.Err()
}

// ListUnprocessedImages returns file paths from the files table that are images
// but don't have an image_metadata row yet.
func (s *GalleryStore) ListUnprocessedImages(ctx context.Context, extensions []string, limit int) ([]string, error) {
	// Build LIKE conditions for image extensions
	var likeConds []string
	var args []interface{}
	for i, ext := range extensions {
		likeConds = append(likeConds, fmt.Sprintf("LOWER(f.name) LIKE $%d", i+1))
		args = append(args, "%"+ext)
	}

	query := fmt.Sprintf(`
		SELECT f.path FROM files f
		LEFT JOIN image_metadata im ON im.file_path = f.path
		WHERE f.is_dir = FALSE AND im.id IS NULL
		AND (%s)
		ORDER BY f.mod_time DESC
		LIMIT $%d`, strings.Join(likeConds, " OR "), len(args)+1)
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		// Table might not exist yet during migration
		if strings.Contains(err.Error(), "does not exist") {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		paths = append(paths, p)
	}
	return paths, rows.Err()
}

// ─── Map Points ─────────────────────────────────────────────────────────────

// MapPoint holds minimal data for a geolocated image marker.
type MapPoint struct {
	FilePath     string
	FileName     string
	Latitude     float64
	Longitude    float64
	HasThumbnail bool
	DateTaken    *time.Time
}

// GetMapPoints returns all geolocated images with minimal fields for map display.
func (s *GalleryStore) GetMapPoints(ctx context.Context, pf *PermFilter) ([]MapPoint, error) {
	var args []interface{}
	join := ""
	permWhere := ""
	if pf != nil {
		join = " JOIN files f ON f.path = im.file_path"
		permWhere = " AND " + pf.Condition
		args = pf.Args
	}

	query := fmt.Sprintf(`
		SELECT %s im.latitude, im.longitude, im.has_thumbnail, im.date_taken
		FROM image_metadata im%s
		WHERE im.latitude IS NOT NULL AND im.longitude IS NOT NULL AND im.status = 'done'%s
		ORDER BY im.date_taken DESC NULLS LAST`,
		filePathSelect(pf), join, permWhere)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("map points: %w", err)
	}
	defer rows.Close()

	var results []MapPoint
	for rows.Next() {
		var r MapPoint
		if err := rows.Scan(&r.FilePath, &r.FileName, &r.Latitude, &r.Longitude, &r.HasThumbnail, &r.DateTaken); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// filePathSelect returns the SELECT columns for file path/name depending on permission filter.
// When pf is nil (admin), we need to join files to get the name.
func filePathSelect(pf *PermFilter) string {
	if pf != nil {
		return "f.path, f.name,"
	}
	return "im.file_path, SUBSTRING(im.file_path FROM '[^/]+$'),"
}

// ImageExistsInDB checks if an image_metadata row exists for the given path.
func (s *GalleryStore) ImageExistsInDB(ctx context.Context, filePath string) bool {
	var exists bool
	s.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM image_metadata WHERE file_path = $1)`, filePath).Scan(&exists)
	return exists
}

// EnsureRow creates a pending image_metadata row if one doesn't exist.
func (s *GalleryStore) EnsureRow(ctx context.Context, filePath string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO image_metadata (file_path, status) VALUES ($1, 'pending')
		ON CONFLICT (file_path) DO NOTHING`, filePath)
	if err != nil && strings.Contains(err.Error(), "violates foreign key") {
		// File doesn't exist in files table (race condition)
		return sql.ErrNoRows
	}
	return err
}
