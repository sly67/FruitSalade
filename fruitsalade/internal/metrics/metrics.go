// Package metrics provides Prometheus metrics for the FruitSalade server.
package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// HTTP request metrics
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "fruitsalade_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "fruitsalade_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	// Content transfer metrics
	contentBytesDownloaded = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "fruitsalade_content_bytes_downloaded_total",
			Help: "Total bytes downloaded from content endpoint",
		},
	)

	contentBytesUploaded = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "fruitsalade_content_bytes_uploaded_total",
			Help: "Total bytes uploaded to content endpoint",
		},
	)

	contentDownloadsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "fruitsalade_content_downloads_total",
			Help: "Total number of content downloads",
		},
		[]string{"status"},
	)

	contentUploadsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "fruitsalade_content_uploads_total",
			Help: "Total number of content uploads",
		},
		[]string{"status"},
	)

	// Metadata metrics
	metadataTreeSize = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "fruitsalade_metadata_tree_size",
			Help: "Number of files/directories in metadata tree",
		},
	)

	metadataRefreshDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "fruitsalade_metadata_refresh_duration_seconds",
			Help:    "Time to rebuild metadata tree from database",
			Buckets: prometheus.DefBuckets,
		},
	)

	// Auth metrics
	authAttemptsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "fruitsalade_auth_attempts_total",
			Help: "Total authentication attempts",
		},
		[]string{"result"},
	)

	activeTokens = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "fruitsalade_active_tokens",
			Help: "Number of active (non-revoked) tokens",
		},
	)

	// Database metrics
	dbQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "fruitsalade_db_query_duration_seconds",
			Help:    "Database query duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"query"},
	)

	dbConnectionsOpen = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "fruitsalade_db_connections_open",
			Help: "Number of open database connections",
		},
	)

	// SSE metrics
	sseConnectionsActive = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "fruitsalade_sse_connections_active",
			Help: "Number of active SSE connections",
		},
	)

	sseEventsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "fruitsalade_sse_events_total",
			Help: "Total SSE events published",
		},
		[]string{"type"},
	)

	// Sharing metrics
	shareLinksActive = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "fruitsalade_share_links_active",
			Help: "Number of active share links",
		},
	)

	shareDownloadsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "fruitsalade_share_downloads_total",
			Help: "Total downloads via share links",
		},
	)

	permissionChecksTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "fruitsalade_permission_checks_total",
			Help: "Total permission checks",
		},
		[]string{"result"},
	)

	// Quota metrics
	rateLimitHitsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "fruitsalade_rate_limit_hits_total",
			Help: "Total rate limit rejections (429s)",
		},
	)

	quotaExceededTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "fruitsalade_quota_exceeded_total",
			Help: "Total quota exceeded rejections",
		},
		[]string{"type"},
	)

	// S3 metrics
	s3OperationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "fruitsalade_s3_operation_duration_seconds",
			Help:    "S3 operation duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"operation"},
	)

	s3OperationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "fruitsalade_s3_operations_total",
			Help: "Total S3 operations",
		},
		[]string{"operation", "status"},
	)
)

// Handler returns the Prometheus metrics HTTP handler.
func Handler() http.Handler {
	return promhttp.Handler()
}

// RecordHTTPRequest records an HTTP request metric.
func RecordHTTPRequest(method, path string, status int, duration time.Duration) {
	httpRequestsTotal.WithLabelValues(method, path, strconv.Itoa(status)).Inc()
	httpRequestDuration.WithLabelValues(method, path).Observe(duration.Seconds())
}

// RecordContentDownload records a content download.
func RecordContentDownload(bytes int64, success bool) {
	contentBytesDownloaded.Add(float64(bytes))
	status := "success"
	if !success {
		status = "error"
	}
	contentDownloadsTotal.WithLabelValues(status).Inc()
}

// RecordContentUpload records a content upload.
func RecordContentUpload(bytes int64, success bool) {
	contentBytesUploaded.Add(float64(bytes))
	status := "success"
	if !success {
		status = "error"
	}
	contentUploadsTotal.WithLabelValues(status).Inc()
}

// SetMetadataTreeSize sets the current metadata tree size.
func SetMetadataTreeSize(size int64) {
	metadataTreeSize.Set(float64(size))
}

// RecordMetadataRefresh records metadata refresh duration.
func RecordMetadataRefresh(duration time.Duration) {
	metadataRefreshDuration.Observe(duration.Seconds())
}

// RecordAuthAttempt records an authentication attempt.
func RecordAuthAttempt(success bool) {
	result := "success"
	if !success {
		result = "failure"
	}
	authAttemptsTotal.WithLabelValues(result).Inc()
}

// SetActiveTokens sets the number of active tokens.
func SetActiveTokens(count int64) {
	activeTokens.Set(float64(count))
}

// RecordDBQuery records a database query duration.
func RecordDBQuery(query string, duration time.Duration) {
	dbQueryDuration.WithLabelValues(query).Observe(duration.Seconds())
}

// SetDBConnectionsOpen sets the number of open database connections.
func SetDBConnectionsOpen(count int) {
	dbConnectionsOpen.Set(float64(count))
}

// RecordS3Operation records an S3 operation.
func RecordS3Operation(operation string, duration time.Duration, success bool) {
	s3OperationDuration.WithLabelValues(operation).Observe(duration.Seconds())
	status := "success"
	if !success {
		status = "error"
	}
	s3OperationsTotal.WithLabelValues(operation, status).Inc()
}

// SetSSEConnectionsActive sets the number of active SSE connections.
func SetSSEConnectionsActive(count int64) {
	sseConnectionsActive.Set(float64(count))
}

// RecordSSEEvent records an SSE event publication.
func RecordSSEEvent(eventType string) {
	sseEventsTotal.WithLabelValues(eventType).Inc()
}

// SetShareLinksActive sets the number of active share links.
func SetShareLinksActive(count int64) {
	shareLinksActive.Set(float64(count))
}

// RecordShareDownload records a share link download.
func RecordShareDownload() {
	shareDownloadsTotal.Inc()
}

// RecordPermissionCheck records a permission check result.
func RecordPermissionCheck(allowed bool) {
	result := "allowed"
	if !allowed {
		result = "denied"
	}
	permissionChecksTotal.WithLabelValues(result).Inc()
}

// RecordRateLimitHit records a rate limit rejection.
func RecordRateLimitHit() {
	rateLimitHitsTotal.Inc()
}

// RecordQuotaExceeded records a quota exceeded rejection.
func RecordQuotaExceeded(quotaType string) {
	quotaExceededTotal.WithLabelValues(quotaType).Inc()
}

// responseWriter wraps http.ResponseWriter to capture status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (rw *responseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

// Middleware returns HTTP middleware that records request metrics.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r)
		RecordHTTPRequest(r.Method, r.URL.Path, rw.statusCode, time.Since(start))
	})
}
