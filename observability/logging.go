package observability

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"
)

// Logger defines the interface for structured logging
type Logger interface {
	// Info logs an info message
	Info(ctx context.Context, message string, fields map[string]interface{})
	// Error logs an error message
	Error(ctx context.Context, message string, err error, fields map[string]interface{})
	// Debug logs a debug message
	Debug(ctx context.Context, message string, fields map[string]interface{})
	// Warn logs a warning message
	Warn(ctx context.Context, message string, fields map[string]interface{})
}

// JSONLogger implements structured logging in JSON format
type JSONLogger struct {
	logger *log.Logger
}

// NewJSONLogger creates a new JSON logger
func NewJSONLogger() *JSONLogger {
	return &JSONLogger{
		logger: log.New(os.Stdout, "", 0),
	}
}

// logEntry represents a log entry
type logEntry struct {
	Level     string                 `json:"level"`
	Timestamp time.Time              `json:"timestamp"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
	Error     string                 `json:"error,omitempty"`
}

// Info logs an info message
func (l *JSONLogger) Info(ctx context.Context, message string, fields map[string]interface{}) {
	entry := logEntry{
		Level:     "info",
		Timestamp: time.Now(),
		Message:   message,
		Fields:    fields,
	}
	l.log(entry)
}

// Error logs an error message
func (l *JSONLogger) Error(ctx context.Context, message string, err error, fields map[string]interface{}) {
	entry := logEntry{
		Level:     "error",
		Timestamp: time.Now(),
		Message:   message,
		Fields:    fields,
	}
	if err != nil {
		entry.Error = err.Error()
	}
	l.log(entry)
}

// Debug logs a debug message
func (l *JSONLogger) Debug(ctx context.Context, message string, fields map[string]interface{}) {
	entry := logEntry{
		Level:     "debug",
		Timestamp: time.Now(),
		Message:   message,
		Fields:    fields,
	}
	l.log(entry)
}

// Warn logs a warning message
func (l *JSONLogger) Warn(ctx context.Context, message string, fields map[string]interface{}) {
	entry := logEntry{
		Level:     "warn",
		Timestamp: time.Now(),
		Message:   message,
		Fields:    fields,
	}
	l.log(entry)
}

// log logs a log entry
func (l *JSONLogger) log(entry logEntry) {
	data, err := json.Marshal(entry)
	if err != nil {
		fmt.Printf("Failed to marshal log entry: %v\n", err)
		return
	}
	l.logger.Println(string(data))
}
