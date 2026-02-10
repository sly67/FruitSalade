// Package logging provides structured logging with zap.
package logging

import (
	"context"
	"net/http"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type contextKey string

const (
	loggerKey    contextKey = "logger"
	requestIDKey contextKey = "request_id"
)

var (
	globalLogger *zap.Logger
	globalLevel  zap.AtomicLevel
)

// Config holds logging configuration.
type Config struct {
	Level      string // debug, info, warn, error
	Format     string // json, console
	OutputPath string // stdout, stderr, or file path
}

// Init initializes the global logger.
func Init(cfg Config) error {
	var level zapcore.Level
	if err := level.UnmarshalText([]byte(cfg.Level)); err != nil {
		level = zapcore.InfoLevel
	}

	var config zap.Config
	if cfg.Format == "console" {
		config = zap.NewDevelopmentConfig()
	} else {
		config = zap.NewProductionConfig()
	}

	globalLevel = zap.NewAtomicLevelAt(level)
	config.Level = globalLevel
	if cfg.OutputPath != "" {
		config.OutputPaths = []string{cfg.OutputPath}
	}

	logger, err := config.Build(
		zap.AddCallerSkip(1),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)
	if err != nil {
		return err
	}

	globalLogger = logger
	return nil
}

// InitDefault initializes with default production settings.
func InitDefault() {
	logger, _ := zap.NewProduction(zap.AddCallerSkip(1))
	globalLogger = logger
}

// Sync flushes any buffered log entries.
func Sync() error {
	if globalLogger != nil {
		return globalLogger.Sync()
	}
	return nil
}

// SetLevel changes the global log level at runtime.
func SetLevel(level string) {
	var l zapcore.Level
	if err := l.UnmarshalText([]byte(level)); err != nil {
		return
	}
	globalLevel.SetLevel(l)
}

// L returns the global logger.
func L() *zap.Logger {
	if globalLogger == nil {
		InitDefault()
	}
	return globalLogger
}

// S returns the global sugared logger.
func S() *zap.SugaredLogger {
	return L().Sugar()
}

// WithContext returns a logger from context, or the global logger.
func WithContext(ctx context.Context) *zap.Logger {
	if logger, ok := ctx.Value(loggerKey).(*zap.Logger); ok {
		return logger
	}
	return L()
}

// WithRequestID adds a request ID to the logger and returns a new context.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	logger := WithContext(ctx).With(zap.String("request_id", requestID))
	return context.WithValue(ctx, loggerKey, logger)
}

// GetRequestID returns the request ID from context.
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}

// Debug logs a debug message.
func Debug(msg string, fields ...zap.Field) {
	L().Debug(msg, fields...)
}

// Info logs an info message.
func Info(msg string, fields ...zap.Field) {
	L().Info(msg, fields...)
}

// Warn logs a warning message.
func Warn(msg string, fields ...zap.Field) {
	L().Warn(msg, fields...)
}

// Error logs an error message.
func Error(msg string, fields ...zap.Field) {
	L().Error(msg, fields...)
}

// Fatal logs a fatal message and exits.
func Fatal(msg string, fields ...zap.Field) {
	L().Fatal(msg, fields...)
}

// requestIDGenerator generates unique request IDs.
var requestIDCounter uint64

func generateRequestID() string {
	requestIDCounter++
	return time.Now().Format("20060102150405") + "-" + string(rune('A'+requestIDCounter%26))
}

// responseWriter wraps http.ResponseWriter to capture status and size.
type responseWriter struct {
	http.ResponseWriter
	status int
	size   int64
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.size += int64(n)
	return n, err
}

// Middleware returns HTTP middleware that adds request logging.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}

		// Add request ID to context
		ctx := WithRequestID(r.Context(), requestID)
		ctx = context.WithValue(ctx, requestIDKey, requestID)
		r = r.WithContext(ctx)

		// Add request ID to response headers
		w.Header().Set("X-Request-ID", requestID)

		// Wrap response writer
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		// Log request start
		logger := WithContext(ctx)
		logger.Debug("request started",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.String("remote_addr", r.RemoteAddr),
			zap.String("user_agent", r.UserAgent()),
		)

		// Handle request
		next.ServeHTTP(rw, r)

		// Log request completion
		duration := time.Since(start)
		logger.Info("request completed",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.Int("status", rw.status),
			zap.Int64("size", rw.size),
			zap.Duration("duration", duration),
		)
	})
}

// Field helpers for common fields.
func String(key, val string) zap.Field {
	return zap.String(key, val)
}

func Int(key string, val int) zap.Field {
	return zap.Int(key, val)
}

func Int64(key string, val int64) zap.Field {
	return zap.Int64(key, val)
}

func Err(err error) zap.Field {
	return zap.Error(err)
}

func Duration(key string, val time.Duration) zap.Field {
	return zap.Duration(key, val)
}

func Any(key string, val interface{}) zap.Field {
	return zap.Any(key, val)
}
