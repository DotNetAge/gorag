// Package logging provides structured logging capabilities for the goRAG framework.
// It offers a simple, flexible logging interface with support for multiple log levels,
// file and console output, and structured field logging.
//
// The package provides two main implementations:
//   - Console logger: Outputs to stdout with minimal formatting
//   - File logger: Writes to a file with configurable log level
//   - No-op logger: Discards all log output (useful for testing)
//
// Example usage:
//
//	// Create a console logger
//	logger := logging.DefaultConsoleLogger()
//	logger.Info("Application started", map[string]any{"version": "1.0"})
//
//	// Create a file logger with debug level
//	logger, err := logging.DefaultFileLogger("app.log", logging.WithLevel(logging.DEBUG))
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer logger.(*logging.defaultLogger).Close()
package logging

import (
	"fmt"
	"log"
	"os"
)

// Level represents the severity level of a log message.
// Log levels are ordered from least to most severe: DEBUG < INFO < WARN < ERROR.
type Level int

// Log level constants define the severity of log messages.
// Messages with a level below the configured threshold will not be logged.
const (
	// DEBUG level is for detailed debugging information.
	// Typically enabled during development or troubleshooting.
	DEBUG Level = iota

	// INFO level is for general operational information.
	// Suitable for production use to track normal operations.
	INFO

	// WARN level is for warning messages that indicate potential issues.
	// Not errors, but situations that might need attention.
	WARN

	// ERROR level is for error messages indicating failures.
	// Should be used for errors that affect operation but are recoverable.
	ERROR
)

// String returns the string representation of the log level.
// Returns "DEBUG", "INFO", "WARN", "ERROR", or "UNKNOWN" for invalid levels.
func (l Level) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Logger defines the interface for structured logging.
// Implementations should support multiple log levels and structured field logging.
//
// All methods accept optional fields as map[string]any for structured logging.
// Example:
//
//	logger.Info("User logged in", map[string]any{
//	    "user_id": 123,
//	    "ip": "192.168.1.1",
//	})
type Logger interface {
	// Info logs an informational message.
	// Use for general operational messages.
	//
	// Parameters:
	//   - msg: The log message
	//   - fields: Optional structured fields (can be omitted or nil)
	Info(msg string, fields ...map[string]any)

	// Error logs an error message with the associated error.
	// The error is automatically included in the fields.
	//
	// Parameters:
	//   - msg: The log message describing the error context
	//   - err: The error that occurred (can be nil)
	//   - fields: Optional additional structured fields
	Error(msg string, err error, fields ...map[string]any)

	// Debug logs a debug message.
	// Use for detailed information useful during development.
	//
	// Parameters:
	//   - msg: The debug message
	//   - fields: Optional structured fields
	Debug(msg string, fields ...map[string]any)

	// Warn logs a warning message.
	// Use for potentially problematic situations that aren't errors.
	//
	// Parameters:
	//   - msg: The warning message
	//   - fields: Optional structured fields
	Warn(msg string, fields ...map[string]any)
}

// defaultLogger is the standard implementation of Logger.
// It supports both console and file output with configurable log levels.
type defaultLogger struct {
	filePath string
	file     *os.File
	logger   *log.Logger
	level    Level
}

// Option is a function that configures a defaultLogger.
type Option func(*defaultLogger)

// WithLevel returns an Option that sets the minimum log level.
// Messages below this level will not be logged.
//
// Parameters:
//   - level: The minimum log level to enable
//
// Returns:
//   - Option: A configuration function for the logger
//
// Example:
//
//	logger, _ := logging.DefaultFileLogger("app.log", logging.WithLevel(logging.DEBUG))
func WithLevel(level Level) Option {
	return func(l *defaultLogger) {
		l.level = level
	}
}

// DefaultConsoleLogger creates a logger that writes to stdout.
// It uses INFO level by default and outputs without timestamps or prefixes.
//
// Returns:
//   - Logger: A logger writing to standard output
//
// Example:
//
//	logger := logging.DefaultConsoleLogger()
//	logger.Info("Server started on port 8080")
func DefaultConsoleLogger() Logger {
	return &defaultLogger{
		file:   os.Stdout,
		logger: log.New(os.Stdout, "", 0),
		level:  INFO,
	}
}

// DefaultFileLogger creates a logger that writes to a file.
// The file is created if it doesn't exist, and appended to if it does.
//
// Parameters:
//   - filePath: Path to the log file
//   - opts: Optional configuration options (e.g., WithLevel)
//
// Returns:
//   - Logger: A logger writing to the specified file
//   - error: Any error that occurred while opening the file
//
// Example:
//
//	logger, err := logging.DefaultFileLogger("app.log", logging.WithLevel(logging.DEBUG))
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer logger.(*logging.defaultLogger).Close()
func DefaultFileLogger(filePath string, opts ...Option) (Logger, error) {
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	l := &defaultLogger{
		filePath: filePath,
		file:     file,
		logger:   log.New(file, "", 0),
		level:    INFO,
	}

	for _, opt := range opts {
		opt(l)
	}

	return l, nil
}

// log writes a formatted log message if the level meets the threshold.
// It formats fields as key=value pairs appended to the message.
func (l *defaultLogger) log(level Level, msg string, fields map[string]any) {
	if level < l.level {
		return
	}

	if fields == nil {
		fields = make(map[string]any)
	}

	var fieldStr string
	for k, v := range fields {
		fieldStr += fmt.Sprintf(" %s=%v", k, v)
	}

	l.logger.Printf("[%s] %s%s", level.String(), msg, fieldStr)
}

// Info logs an informational message with optional structured fields.
func (l *defaultLogger) Info(msg string, fields ...map[string]any) {
	var f map[string]any
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(INFO, msg, f)
}

// Error logs an error message with the error and optional structured fields.
// The error is automatically added to the fields with key "error".
func (l *defaultLogger) Error(msg string, err error, fields ...map[string]any) {
	f := make(map[string]any)
	if err != nil {
		f["error"] = err.Error()
	}
	if len(fields) > 0 {
		for k, v := range fields[0] {
			f[k] = v
		}
	}
	l.log(ERROR, msg, f)
}

// Debug logs a debug message with optional structured fields.
func (l *defaultLogger) Debug(msg string, fields ...map[string]any) {
	var f map[string]any
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(DEBUG, msg, f)
}

// Warn logs a warning message with optional structured fields.
func (l *defaultLogger) Warn(msg string, fields ...map[string]any) {
	var f map[string]any
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(WARN, msg, f)
}

// Close closes the underlying file if this is a file logger.
// Should be called when the logger is no longer needed.
//
// Returns:
//   - error: Any error that occurred while closing the file
func (l *defaultLogger) Close() error {
	return l.file.Close()
}

// noopLogger is a no-op implementation that discards all log messages.
type noopLogger struct{}

// DefaultNoopLogger creates a logger that discards all output.
// Useful for testing or when logging should be disabled.
//
// Returns:
//   - Logger: A logger that does nothing
func DefaultNoopLogger() Logger {
	return &noopLogger{}
}

func (l *noopLogger) Info(string, ...map[string]any)         {}
func (l *noopLogger) Error(string, error, ...map[string]any) {}
func (l *noopLogger) Debug(string, ...map[string]any)        {}
func (l *noopLogger) Warn(string, ...map[string]any)         {}
