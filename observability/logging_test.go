package observability

import (
	"context"
	"fmt"
	"testing"
)

func TestJSONLogger_Info(t *testing.T) {
	logger := NewJSONLogger()
	// Just test that the method doesn't panic
	logger.Info(context.Background(), "Test info message", map[string]interface{}{
		"key": "value",
	})
}

func TestJSONLogger_Error(t *testing.T) {
	logger := NewJSONLogger()
	// Just test that the method doesn't panic
	err := fmt.Errorf("test error")
	logger.Error(context.Background(), "Test error message", err, map[string]interface{}{
		"key": "value",
	})
}

func TestJSONLogger_Debug(t *testing.T) {
	logger := NewJSONLogger()
	// Just test that the method doesn't panic
	logger.Debug(context.Background(), "Test debug message", map[string]interface{}{
		"key": "value",
	})
}

func TestJSONLogger_Warn(t *testing.T) {
	logger := NewJSONLogger()
	// Just test that the method doesn't panic
	logger.Warn(context.Background(), "Test warn message", map[string]interface{}{
		"key": "value",
	})
}
