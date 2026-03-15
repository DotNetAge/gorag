package testkit

import (
	"sync"

	"github.com/DotNetAge/gorag/pkg/logging"
)

// MockLogger is a testable logger implementation that records all log entries.
type MockLogger struct {
	mu       sync.RWMutex
	infos    []LogEntry
	warns    []LogEntry
	errors   []LogEntry
	debugs   []LogEntry
	lastInfo string
}

// LogEntry represents a single log entry.
type LogEntry struct {
	Message string
	Fields  map[string]interface{}
}

// NewMockLogger creates a new mock logger for testing.
func NewMockLogger() *MockLogger {
	return &MockLogger{
		infos:  make([]LogEntry, 0),
		warns:  make([]LogEntry, 0),
		errors: make([]LogEntry, 0),
		debugs: make([]LogEntry, 0),
	}
}

var _ logging.Logger = (*MockLogger)(nil)

// Info logs an info message and records it for testing.
func (m *MockLogger) Info(message string, fields ...map[string]interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var fieldMap map[string]interface{}
	if len(fields) > 0 {
		fieldMap = fields[0]
	} else {
		fieldMap = make(map[string]interface{})
	}
	entry := LogEntry{Message: message, Fields: fieldMap}
	m.infos = append(m.infos, entry)
	if msg, ok := fieldMap["message"].(string); ok {
		m.lastInfo = msg
	} else {
		m.lastInfo = message
	}
}

// Warn logs a warning message and records it for testing.
func (m *MockLogger) Warn(message string, fields ...map[string]interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var fieldMap map[string]interface{}
	if len(fields) > 0 {
		fieldMap = fields[0]
	} else {
		fieldMap = make(map[string]interface{})
	}
	m.warns = append(m.warns, LogEntry{Message: message, Fields: fieldMap})
}

// Error logs an error message and records it for testing.
func (m *MockLogger) Error(message string, err error, fields ...map[string]interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var fieldMap map[string]interface{}
	if len(fields) > 0 {
		fieldMap = fields[0]
	} else {
		fieldMap = make(map[string]interface{})
	}
	if fieldMap == nil {
		fieldMap = make(map[string]interface{})
	}
	fieldMap["error"] = err.Error()
	m.errors = append(m.errors, LogEntry{Message: message, Fields: fieldMap})
}

// Debug logs a debug message and records it for testing.
func (m *MockLogger) Debug(message string, fields ...map[string]interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var fieldMap map[string]interface{}
	if len(fields) > 0 {
		fieldMap = fields[0]
	} else {
		fieldMap = make(map[string]interface{})
	}
	m.debugs = append(m.debugs, LogEntry{Message: message, Fields: fieldMap})
}

// GetInfos returns all recorded info messages.
func (m *MockLogger) GetInfos() []LogEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]LogEntry, len(m.infos))
	copy(result, m.infos)
	return result
}

// GetWarns returns all recorded warning messages.
func (m *MockLogger) GetWarns() []LogEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]LogEntry, len(m.warns))
	copy(result, m.warns)
	return result
}

// GetErrors returns all recorded error messages.
func (m *MockLogger) GetErrors() []LogEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]LogEntry, len(m.errors))
	copy(result, m.errors)
	return result
}

// GetDebugs returns all recorded debug messages.
func (m *MockLogger) GetDebugs() []LogEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]LogEntry, len(m.debugs))
	copy(result, m.debugs)
	return result
}

// LastInfo returns the last info message.
func (m *MockLogger) LastInfo() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastInfo
}

// Reset clears all recorded log entries.
func (m *MockLogger) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.infos = make([]LogEntry, 0)
	m.warns = make([]LogEntry, 0)
	m.errors = make([]LogEntry, 0)
	m.debugs = make([]LogEntry, 0)
	m.lastInfo = ""
}

// AssertInfoContains checks if any info message contains the given substring.
func (m *MockLogger) AssertInfoContains(substring string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, entry := range m.infos {
		if contains(entry.Message, substring) {
			return true
		}
		for _, v := range entry.Fields {
			if str, ok := v.(string); ok && contains(str, substring) {
				return true
			}
		}
	}
	return false
}

// AssertErrorContains checks if any error message contains the given substring.
func (m *MockLogger) AssertErrorContains(substring string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, entry := range m.errors {
		if contains(entry.Message, substring) {
			return true
		}
		for _, v := range entry.Fields {
			if str, ok := v.(string); ok && contains(str, substring) {
				return true
			}
		}
	}
	return false
}

// CountInfos returns the number of info messages.
func (m *MockLogger) CountInfos() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.infos)
}

// CountErrors returns the number of error messages.
func (m *MockLogger) CountErrors() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.errors)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
