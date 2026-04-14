package logging

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLevel_String(t *testing.T) {
	tests := []struct {
		level    Level
		expected string
	}{
		{DEBUG, "DEBUG"},
		{INFO, "INFO"},
		{WARN, "WARN"},
		{ERROR, "ERROR"},
		{Level(100), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.level.String())
		})
	}
}

func TestWithLevel(t *testing.T) {
	logger := &defaultLogger{level: INFO}
	opt := WithLevel(DEBUG)
	opt(logger)
	assert.Equal(t, DEBUG, logger.level)
}

func TestDefaultConsoleLogger(t *testing.T) {
	logger := DefaultConsoleLogger()
	assert.NotNil(t, logger)
	assert.IsType(t, &defaultLogger{}, logger)
}

func TestDefaultConsoleLogger_ImplementsInterface(t *testing.T) {
	logger := DefaultConsoleLogger()
	_, ok := logger.(Logger)
	assert.True(t, ok)
}

func TestDefaultFileLogger_Success(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.log")
	fileLogger, err := DefaultFileLogger(filePath)

	assert.NoError(t, err)
	assert.NotNil(t, fileLogger)

	if dl, ok := fileLogger.(*defaultLogger); ok {
		err = dl.Close()
		assert.NoError(t, err)
	}
}

func TestDefaultFileLogger_InvalidPath(t *testing.T) {
	logger, err := DefaultFileLogger("/nonexistent/directory/test.log")
	assert.Error(t, err)
	assert.Nil(t, logger)
}

func TestDefaultNoopLogger(t *testing.T) {
	logger := DefaultNoopLogger()
	assert.NotNil(t, logger)
	_, ok := logger.(Logger)
	assert.True(t, ok)
}

func TestNoopLogger_AllMethodsNoOp(t *testing.T) {
	logger := &noopLogger{}
	logger.Info("info message", map[string]any{"key": "value"})
	logger.Debug("debug message", map[string]any{"key": "value"})
	logger.Warn("warn message", map[string]any{"key": "value"})
	logger.Error("error message", nil, map[string]any{"key": "value"})
}

func TestDefaultLogger_Info(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.log")
	fileLogger, _ := DefaultFileLogger(filePath, WithLevel(DEBUG))
	fileLogger.Info("test info", map[string]any{"key": "value"})
	if dl, ok := fileLogger.(*defaultLogger); ok {
		dl.Close()
	}

	content, err := os.ReadFile(filePath)
	assert.NoError(t, err)
	assert.Contains(t, string(content), "[INFO]")
	assert.Contains(t, string(content), "test info")
}

func TestDefaultLogger_Error(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.log")
	fileLogger, _ := DefaultFileLogger(filePath, WithLevel(DEBUG))
	fileLogger.Error("test error", nil, map[string]any{"key": "value"})
	if dl, ok := fileLogger.(*defaultLogger); ok {
		dl.Close()
	}

	content, err := os.ReadFile(filePath)
	assert.NoError(t, err)
	assert.Contains(t, string(content), "[ERROR]")
	assert.Contains(t, string(content), "test error")
}

func TestDefaultLogger_Debug(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.log")
	fileLogger, _ := DefaultFileLogger(filePath, WithLevel(DEBUG))
	fileLogger.Debug("test debug", map[string]any{"key": "value"})
	if dl, ok := fileLogger.(*defaultLogger); ok {
		dl.Close()
	}

	content, err := os.ReadFile(filePath)
	assert.NoError(t, err)
	assert.Contains(t, string(content), "[DEBUG]")
	assert.Contains(t, string(content), "test debug")
}

func TestDefaultLogger_Warn(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.log")
	fileLogger, _ := DefaultFileLogger(filePath, WithLevel(DEBUG))
	fileLogger.Warn("test warn", map[string]any{"key": "value"})
	if dl, ok := fileLogger.(*defaultLogger); ok {
		dl.Close()
	}

	content, err := os.ReadFile(filePath)
	assert.NoError(t, err)
	assert.Contains(t, string(content), "[WARN]")
	assert.Contains(t, string(content), "test warn")
}

func TestDefaultLogger_LevelFiltering(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.log")
	fileLogger, _ := DefaultFileLogger(filePath, WithLevel(WARN))
	fileLogger.Debug("debug should not appear", map[string]any{})
	fileLogger.Info("info should not appear", map[string]any{})
	fileLogger.Error("error should appear", nil, map[string]any{})
	if dl, ok := fileLogger.(*defaultLogger); ok {
		dl.Close()
	}

	content, err := os.ReadFile(filePath)
	assert.NoError(t, err)
	assert.NotContains(t, string(content), "debug should not appear")
	assert.NotContains(t, string(content), "info should not appear")
	assert.Contains(t, string(content), "error should appear")
}

func TestDefaultLogger_ErrorWithNilError(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.log")
	fileLogger, _ := DefaultFileLogger(filePath, WithLevel(DEBUG))
	fileLogger.Error("test error", nil, map[string]any{"key": "value"})
	if dl, ok := fileLogger.(*defaultLogger); ok {
		dl.Close()
	}

	content, err := os.ReadFile(filePath)
	assert.NoError(t, err)
	assert.Contains(t, string(content), "[ERROR]")
	assert.Contains(t, string(content), "test error")
}

func TestDefaultLogger_ErrorWithRealError(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.log")
	fileLogger, _ := DefaultFileLogger(filePath, WithLevel(DEBUG))
	testErr := os.ErrPermission
	fileLogger.Error("test error", testErr, map[string]any{"key": "value"})
	if dl, ok := fileLogger.(*defaultLogger); ok {
		dl.Close()
	}

	content, err := os.ReadFile(filePath)
	assert.NoError(t, err)
	assert.Contains(t, string(content), "[ERROR]")
	assert.Contains(t, string(content), "test error")
	assert.Contains(t, string(content), testErr.Error())
}

func TestDefaultLogger_FieldsFormat(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.log")
	fileLogger, _ := DefaultFileLogger(filePath, WithLevel(DEBUG))
	fileLogger.Info("test message", map[string]any{
		"string": "value",
		"int":    42,
		"bool":   true,
	})
	if dl, ok := fileLogger.(*defaultLogger); ok {
		dl.Close()
	}

	content, err := os.ReadFile(filePath)
	assert.NoError(t, err)
	assert.Contains(t, string(content), "string=value")
	assert.Contains(t, string(content), "int=42")
	assert.Contains(t, string(content), "bool=true")
}

func TestDefaultLogger_NilFields(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.log")
	fileLogger, _ := DefaultFileLogger(filePath, WithLevel(DEBUG))
	fileLogger.Info("test message", nil)
	if dl, ok := fileLogger.(*defaultLogger); ok {
		dl.Close()
	}

	content, err := os.ReadFile(filePath)
	assert.NoError(t, err)
	assert.Contains(t, string(content), "test message")
}

func TestDefaultLogger_Close(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.log")
	fileLogger, err := DefaultFileLogger(filePath)
	assert.NoError(t, err)

	if dl, ok := fileLogger.(*defaultLogger); ok {
		err = dl.Close()
		assert.NoError(t, err)
	}
}
