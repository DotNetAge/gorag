package logging

import (
	"fmt"
	"log"
	"os"
)

type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

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

type Logger interface {
	Info(msg string, fields ...map[string]any)
	Error(msg string, err error, fields ...map[string]any)
	Debug(msg string, fields ...map[string]any)
	Warn(msg string, fields ...map[string]any)
}

type defaultLogger struct {
	filePath string
	file     *os.File
	logger   *log.Logger
	level    Level
}

type Option func(*defaultLogger)

func WithLevel(level Level) Option {
	return func(l *defaultLogger) {
		l.level = level
	}
}

func DefaultConsoleLogger() Logger {
	return &defaultLogger{
		file:   os.Stdout,
		logger: log.New(os.Stdout, "", 0),
		level:  INFO,
	}
}

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

func (l *defaultLogger) Info(msg string, fields ...map[string]any) {
	var f map[string]any
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(INFO, msg, f)
}

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

func (l *defaultLogger) Debug(msg string, fields ...map[string]any) {
	var f map[string]any
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(DEBUG, msg, f)
}

func (l *defaultLogger) Warn(msg string, fields ...map[string]any) {
	var f map[string]any
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(WARN, msg, f)
}

func (l *defaultLogger) Close() error {
	return l.file.Close()
}

type noopLogger struct{}

func DefaultNoopLogger() Logger {
	return &noopLogger{}
}

func (l *noopLogger) Info(string, ...map[string]any)         {}
func (l *noopLogger) Error(string, error, ...map[string]any) {}
func (l *noopLogger) Debug(string, ...map[string]any)        {}
func (l *noopLogger) Warn(string, ...map[string]any)         {}
