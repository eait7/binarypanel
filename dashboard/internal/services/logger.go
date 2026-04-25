package services

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// LogLevel represents the severity of a log entry.
type LogLevel string

const (
	LogInfo  LogLevel = "INFO"
	LogWarn  LogLevel = "WARN"
	LogError LogLevel = "ERROR"
)

// LogEntry represents a single structured log record.
type LogEntry struct {
	Timestamp string   `json:"timestamp"`
	Level     LogLevel `json:"level"`
	Component string   `json:"component"`
	Message   string   `json:"message"`
	Context   string   `json:"context,omitempty"`
}

// PanelLogger provides structured logging for BinaryPanel with in-memory ring buffer and file persistence.
type PanelLogger struct {
	mu       sync.RWMutex
	entries  []LogEntry
	maxSize  int
	filePath string
}

// Global logger instance accessible from all packages.
var globalLogger *PanelLogger

// NewPanelLogger creates a new logger that writes to both memory and disk.
func NewPanelLogger(filePath string, maxEntries int) *PanelLogger {
	logger := &PanelLogger{
		entries:  make([]LogEntry, 0, maxEntries),
		maxSize:  maxEntries,
		filePath: filePath,
	}
	globalLogger = logger
	return logger
}

// GetLogger returns the global logger instance.
func GetLogger() *PanelLogger {
	return globalLogger
}

// Log records a structured log entry.
func (l *PanelLogger) Log(level LogLevel, component, message string, context ...string) {
	entry := LogEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Level:     level,
		Component: component,
		Message:   message,
	}
	if len(context) > 0 && context[0] != "" {
		entry.Context = context[0]
	}

	l.mu.Lock()
	// Ring buffer: drop oldest when full
	if len(l.entries) >= l.maxSize {
		l.entries = l.entries[1:]
	}
	l.entries = append(l.entries, entry)
	l.mu.Unlock()

	// Persist to file asynchronously
	go l.appendToFile(entry)
}

// Info logs an informational message.
func (l *PanelLogger) Info(component, message string, context ...string) {
	l.Log(LogInfo, component, message, context...)
}

// Warn logs a warning message.
func (l *PanelLogger) Warn(component, message string, context ...string) {
	l.Log(LogWarn, component, message, context...)
}

// Error logs an error message.
func (l *PanelLogger) Error(component, message string, context ...string) {
	l.Log(LogError, component, message, context...)
}

// GetEntries returns the last N log entries (most recent first).
func (l *PanelLogger) GetEntries(limit int) []LogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	total := len(l.entries)
	if limit <= 0 || limit > total {
		limit = total
	}

	// Return newest first
	result := make([]LogEntry, limit)
	for i := 0; i < limit; i++ {
		result[i] = l.entries[total-1-i]
	}
	return result
}

// GetEntriesByLevel returns entries filtered by level.
func (l *PanelLogger) GetEntriesByLevel(level LogLevel, limit int) []LogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var filtered []LogEntry
	for i := len(l.entries) - 1; i >= 0 && len(filtered) < limit; i-- {
		if l.entries[i].Level == level {
			filtered = append(filtered, l.entries[i])
		}
	}
	return filtered
}

// appendToFile writes a log entry to the persistent log file.
func (l *PanelLogger) appendToFile(entry LogEntry) {
	if l.filePath == "" {
		return
	}

	f, err := os.OpenFile(l.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	line := fmt.Sprintf("[%s] [%s] [%s] %s", entry.Timestamp, entry.Level, entry.Component, entry.Message)
	if entry.Context != "" {
		line += " | " + entry.Context
	}
	f.WriteString(line + "\n")
}

// DiagnoseCaddyError analyzes a Caddy API error and returns a user-friendly diagnostic hint.
func DiagnoseCaddyError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()

	switch {
	case strings.Contains(msg, "lookup") && strings.Contains(msg, "server misbehaving"):
		return "DNS resolution failed inside the container. This is typically caused by systemd-resolved interfering with Docker's internal DNS. Fix: Run 'sudo systemctl restart docker' on the host, or add dns: [8.8.8.8, 1.1.1.1] to the dashboard service in docker-compose.yml."
	case strings.Contains(msg, "lookup") && strings.Contains(msg, "no such host"):
		return "DNS lookup failed — the hostname could not be resolved. Ensure the Caddy container is on the same Docker network as the dashboard. Check 'docker network ls' and verify the 'binarypanel' network exists."
	case strings.Contains(msg, "connection refused"):
		return "Connection refused — the Caddy API is not accepting connections. Ensure Caddy is running: 'docker ps | grep caddy'. If stopped, restart it: 'docker compose restart caddy'."
	case strings.Contains(msg, "i/o timeout") || strings.Contains(msg, "deadline exceeded"):
		return "Connection timed out reaching Caddy API. The caddy container may be overloaded or unreachable. Check container health: 'docker logs binarypanel-caddy'."
	case strings.Contains(msg, "EOF"):
		return "Caddy API returned an empty response. The server may be restarting. Wait a few seconds and retry."
	default:
		return ""
	}
}

// ClearEntries clears all in-memory log entries.
func (l *PanelLogger) ClearEntries() {
	l.mu.Lock()
	l.entries = make([]LogEntry, 0, l.maxSize)
	l.mu.Unlock()
}

// ToJSON serializes entries to JSON.
func (l *PanelLogger) ToJSON(limit int) ([]byte, error) {
	entries := l.GetEntries(limit)
	return json.Marshal(entries)
}
