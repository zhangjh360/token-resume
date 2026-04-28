package logger

import (
	"log"
	"os"
	"strings"
)

type Logger struct {
	base  *log.Logger
	level int
}

const (
	levelDebug = iota
	levelInfo
	levelWarn
	levelError
)

func New(level string) *Logger {
	return &Logger{
		base:  log.New(os.Stdout, "", log.LstdFlags),
		level: parseLevel(level),
	}
}

func (l *Logger) Debug(format string, args ...any) {
	if l.level <= levelDebug {
		l.base.Printf("[DEBUG] "+format, args...)
	}
}

func (l *Logger) Info(format string, args ...any) {
	if l.level <= levelInfo {
		l.base.Printf("[INFO] "+format, args...)
	}
}

func (l *Logger) Warn(format string, args ...any) {
	if l.level <= levelWarn {
		l.base.Printf("[WARN] "+format, args...)
	}
}

func (l *Logger) Error(format string, args ...any) {
	if l.level <= levelError {
		l.base.Printf("[ERROR] "+format, args...)
	}
}

func parseLevel(v string) int {
	switch strings.ToLower(v) {
	case "debug":
		return levelDebug
	case "warn":
		return levelWarn
	case "error":
		return levelError
	default:
		return levelInfo
	}
}
