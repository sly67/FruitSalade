// Package logger provides leveled logging.
package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"sync"
)

// Level represents the log level.
type Level int

const (
	LevelQuiet Level = iota
	LevelError
	LevelInfo
	LevelDebug
)

// Logger provides leveled logging.
type Logger struct {
	mu     sync.Mutex
	level  Level
	out    io.Writer
	prefix string
}

var defaultLogger = &Logger{
	level: LevelInfo,
	out:   os.Stderr,
}

// SetLevel sets the global log level.
func SetLevel(level Level) {
	defaultLogger.mu.Lock()
	defer defaultLogger.mu.Unlock()
	defaultLogger.level = level
}

// SetOutput sets the output writer.
func SetOutput(w io.Writer) {
	defaultLogger.mu.Lock()
	defer defaultLogger.mu.Unlock()
	defaultLogger.out = w
}

// SetPrefix sets the log prefix.
func SetPrefix(prefix string) {
	defaultLogger.mu.Lock()
	defer defaultLogger.mu.Unlock()
	defaultLogger.prefix = prefix
}

// ParseLevel parses a level string.
func ParseLevel(s string) Level {
	switch s {
	case "quiet", "q":
		return LevelQuiet
	case "error", "e":
		return LevelError
	case "info", "i":
		return LevelInfo
	case "debug", "d", "verbose", "v":
		return LevelDebug
	default:
		return LevelInfo
	}
}

func (l *Logger) log(level Level, format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if level > l.level {
		return
	}

	prefix := ""
	switch level {
	case LevelError:
		prefix = "[ERROR] "
	case LevelInfo:
		prefix = "[INFO]  "
	case LevelDebug:
		prefix = "[DEBUG] "
	}

	msg := fmt.Sprintf(format, args...)
	log.SetOutput(l.out)
	log.SetFlags(log.Ltime)
	log.Printf("%s%s%s", l.prefix, prefix, msg)
}

// Error logs an error message.
func Error(format string, args ...interface{}) {
	defaultLogger.log(LevelError, format, args...)
}

// Info logs an info message.
func Info(format string, args ...interface{}) {
	defaultLogger.log(LevelInfo, format, args...)
}

// Debug logs a debug message.
func Debug(format string, args ...interface{}) {
	defaultLogger.log(LevelDebug, format, args...)
}

// Errorf is an alias for Error.
func Errorf(format string, args ...interface{}) {
	Error(format, args...)
}

// Infof is an alias for Info.
func Infof(format string, args ...interface{}) {
	Info(format, args...)
}

// Debugf is an alias for Debug.
func Debugf(format string, args ...interface{}) {
	Debug(format, args...)
}
