// Package logger provides a small dependency-free file logger with daily
// rotation and automatic cleanup of old files. Levels are INFO / WARN / ERROR
// / AUDIT, each written to its own daily file under a YYYYMM directory
// (e.g. logs/202608/info_20260805.log). Logger output is file-only; calls made
// before Init are discarded.
package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type logger struct {
	mu        sync.Mutex
	logDir    string
	files     map[string]*os.File
	done      chan struct{}
	closeOnce sync.Once
}

var (
	loggerMu sync.RWMutex
	l        *logger

	retentionMu   sync.RWMutex
	retentionFunc func() int
)

// retentionFunc, when set, is called by the cleanup goroutine to obtain the
// current retention-days value at runtime (e.g. from the sysconfig table).
// When nil the static value passed to Init is used.
// SetRetentionFunc registers a callback that returns the current log retention
// days. The cleanup goroutine calls it on every tick so admin changes take
// effect without a restart.
func SetRetentionFunc(f func() int) {
	retentionMu.Lock()
	defer retentionMu.Unlock()
	retentionFunc = f
}

func currentRetention(fallback int) int {
	retentionMu.RLock()
	defer retentionMu.RUnlock()
	if retentionFunc != nil {
		return retentionFunc()
	}
	return fallback
}

// Init starts the logger writing into logDir, keeping retentionDays of history
// (a value <= 0 disables cleanup). Safe to call once at startup.
func Init(logDir string, retentionDays int) error {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	state := &logger{
		logDir: logDir,
		files:  make(map[string]*os.File),
		done:   make(chan struct{}),
	}
	loggerMu.Lock()
	l = state
	loggerMu.Unlock()

	// Run initial cleanup if retention is configured.
	if days := currentRetention(retentionDays); days > 0 {
		cleanOldLogs(logDir, days)
	}

	// Always start the ticker so that a later admin change from 0 → N
	// takes effect without a restart.
	go func(state *logger) {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if days := currentRetention(retentionDays); days > 0 {
					cleanOldLogs(state.logDir, days)
				}
			case <-state.done:
				return
			}
		}
	}(state)

	return nil
}

// Close flushes and closes open log files. Safe to call when uninitialized.
func Close() {
	loggerMu.RLock()
	state := l
	loggerMu.RUnlock()
	if state == nil {
		return
	}
	loggerMu.Lock()
	if l == state {
		l = nil
	}
	loggerMu.Unlock()
	state.closeOnce.Do(func() { close(state.done) })
	state.mu.Lock()
	for _, f := range state.files {
		f.Close()
	}
	state.files = make(map[string]*os.File)
	state.mu.Unlock()
}

func Info(format string, args ...interface{})  { write("INFO", format, args...) }
func Warn(format string, args ...interface{})  { write("WARN", format, args...) }
func Error(format string, args ...interface{}) { write("ERROR", format, args...) }

// Audit records a security-relevant action to the audit log file. The DB-backed
// audit trail lives in the audit package; this is the human-readable mirror.
func Audit(format string, args ...interface{}) { write("AUDIT", format, args...) }

// Writer returns an io.Writer that sends standard-library log output to the
// requested logger level. It is useful for dependencies such as GORM that
// accept a *log.Logger but must not write to the console.
func Writer(level string) io.Writer {
	return levelWriter{level: level}
}

type levelWriter struct {
	level string
}

func (w levelWriter) Write(p []byte) (int, error) {
	message := strings.TrimRight(string(p), "\r\n")
	if message != "" {
		write(w.level, "%s", message)
	}
	return len(p), nil
}

func write(level, format string, args ...interface{}) {
	level = normalizeLevel(level)
	ts := time.Now().Format("2006-01-02 15:04:05")
	msg := fmt.Sprintf("%s [%s] %s\n", ts, level, fmt.Sprintf(format, args...))

	loggerMu.RLock()
	state := l
	loggerMu.RUnlock()
	if state == nil {
		return
	}

	state.mu.Lock()
	defer state.mu.Unlock()
	if f := state.getFile(level); f != nil {
		f.WriteString(msg)
	}
}

func normalizeLevel(level string) string {
	switch strings.ToUpper(strings.TrimSpace(level)) {
	case "ERROR":
		return "ERROR"
	case "WARN":
		return "WARN"
	case "AUDIT":
		return "AUDIT"
	default:
		return "INFO"
	}
}

func (l *logger) getFile(level string) *os.File {
	key := strings.ToLower(normalizeLevel(level))
	now := time.Now()
	date := now.Format("20060102")
	monthDir := filepath.Join(l.logDir, now.Format("200601"))
	if err := os.MkdirAll(monthDir, 0755); err != nil {
		return nil
	}
	want := filepath.Join(monthDir, fmt.Sprintf("%s_%s.log", key, date))

	if f, ok := l.files[key]; ok {
		if f.Name() == want {
			return f
		}
		f.Close() // date rolled over
	}

	f, err := os.OpenFile(want, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil
	}
	l.files[key] = f
	return f
}

func cleanOldLogs(logDir string, days int) {
	if days <= 0 {
		return
	}

	today := time.Now()
	cutoff := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location()).AddDate(0, 0, -days+1)
	months, err := os.ReadDir(logDir)
	if err != nil {
		return
	}
	for _, month := range months {
		if !month.IsDir() || len(month.Name()) != 6 {
			continue
		}
		monthPath := filepath.Join(logDir, month.Name())
		files, err := os.ReadDir(monthPath)
		if err != nil {
			continue
		}
		for _, file := range files {
			if file.IsDir() {
				continue
			}
			date, ok := parseLogDate(file.Name())
			if ok && date.Before(cutoff) {
				_ = os.Remove(filepath.Join(monthPath, file.Name()))
			}
		}
		if remaining, err := os.ReadDir(monthPath); err == nil && len(remaining) == 0 {
			_ = os.Remove(monthPath)
		}
	}
}

func parseLogDate(name string) (time.Time, bool) {
	parts := strings.SplitN(name, "_", 2)
	if len(parts) != 2 || !strings.HasSuffix(parts[1], ".log") {
		return time.Time{}, false
	}
	date, err := time.ParseInLocation("20060102", strings.TrimSuffix(parts[1], ".log"), time.Local)
	if err != nil {
		return time.Time{}, false
	}
	return date, true
}
