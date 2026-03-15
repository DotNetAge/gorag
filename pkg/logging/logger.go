package logging

import (
	"log"
	"os"
)

// Logger 通用日志接口
type Logger interface {
	Info(msg string, fields ...map[string]interface{})
	Error(msg string, err error, fields ...map[string]interface{})
	Debug(msg string, fields ...map[string]interface{})
	Warn(msg string, fields ...map[string]interface{})
}

// defaultLogger 默认日志实现
type defaultLogger struct {
	filePath string
	file     *os.File
}

// NewDefaultLogger 创建默认日志记录器
func NewDefaultLogger(filePath string) (Logger, error) {
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	return &defaultLogger{
		filePath: filePath,
		file:     file,
	}, nil
}

// Info 输出信息日志
func (l *defaultLogger) Info(msg string, fields ...map[string]interface{}) {
	log.Printf("[INFO] %s - %v", msg, fields)
}

// Error 输出错误日志
func (l *defaultLogger) Error(msg string, err error, fields ...map[string]interface{}) {
	log.Printf("[ERROR] %s - %v - error: %v", msg, fields, err)
}

// Debug 输出调试日志
func (l *defaultLogger) Debug(msg string, fields ...map[string]interface{}) {
	log.Printf("[DEBUG] %s - %v", msg, fields)
}

// Warn 输出警告日志
func (l *defaultLogger) Warn(msg string, fields ...map[string]interface{}) {
	log.Printf("[WARN] %s - %v", msg, fields)
}

// noopLogger 是一个空的日志记录器，用于测试或不需要日志的场景
type noopLogger struct{}

// NewNoopLogger 创建一个空的日志记录器
func NewNoopLogger() Logger {
	return &noopLogger{}
}

func (l *noopLogger) Info(string, ...map[string]interface{})         {}
func (l *noopLogger) Error(string, error, ...map[string]interface{}) {}
func (l *noopLogger) Debug(string, ...map[string]interface{})        {}
func (l *noopLogger) Warn(string, ...map[string]interface{})         {}
