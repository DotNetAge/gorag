package testkit

import (
	"sync"
	"time"

	"github.com/DotNetAge/gorag/pkg/observability"
)

// MetricRecord represents a single metric recording.
type MetricRecord struct {
	Name   string
	Value  float64
	Labels map[string]string
	Time   time.Time
}

// DurationRecord represents a duration recording.
type DurationRecord struct {
	Operation string
	Duration  time.Duration
	Labels    map[string]string
	Time      time.Time
}

// CountRecord represents a count recording.
type CountRecord struct {
	Operation string
	Status    string
	Labels    map[string]string
	Time      time.Time
}

// MockCollector is a testable metrics collector that records all metrics.
type MockCollector struct {
	mu              sync.RWMutex
	durations       []DurationRecord
	counts          []CountRecord
	values          []MetricRecord
	lastOperation   string
	lastDuration    time.Duration
	lastCountStatus string
}

var _ observability.Collector = (*MockCollector)(nil)

// NewMockCollector creates a new mock collector for testing.
func NewMockCollector() *MockCollector {
	return &MockCollector{
		durations: make([]DurationRecord, 0),
		counts:    make([]CountRecord, 0),
		values:    make([]MetricRecord, 0),
	}
}

// RecordDuration records a duration metric.
func (m *MockCollector) RecordDuration(operation string, duration time.Duration, labels map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	record := DurationRecord{
		Operation: operation,
		Duration:  duration,
		Labels:    labels,
		Time:      time.Now(),
	}
	m.durations = append(m.durations, record)
	m.lastOperation = operation
	m.lastDuration = duration
}

// RecordCount records a count metric.
func (m *MockCollector) RecordCount(operation string, status string, labels map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	record := CountRecord{
		Operation: operation,
		Status:    status,
		Labels:    labels,
		Time:      time.Now(),
	}
	m.counts = append(m.counts, record)
	if operation == m.lastOperation {
		m.lastCountStatus = status
	}
}

// RecordValue records a value metric.
func (m *MockCollector) RecordValue(metricName string, value float64, labels map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	record := MetricRecord{
		Name:   metricName,
		Value:  value,
		Labels: labels,
		Time:   time.Now(),
	}
	m.values = append(m.values, record)
}

// GetDurations returns all recorded duration metrics.
func (m *MockCollector) GetDurations() []DurationRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]DurationRecord, len(m.durations))
	copy(result, m.durations)
	return result
}

// GetCounts returns all recorded count metrics.
func (m *MockCollector) GetCounts() []CountRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]CountRecord, len(m.counts))
	copy(result, m.counts)
	return result
}

// GetValues returns all recorded value metrics.
func (m *MockCollector) GetValues() []MetricRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]MetricRecord, len(m.values))
	copy(result, m.values)
	return result
}

// LastOperation returns the last recorded operation name.
func (m *MockCollector) LastOperation() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastOperation
}

// LastDuration returns the last recorded duration.
func (m *MockCollector) LastDuration() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastDuration
}

// LastCountStatus returns the last recorded count status.
func (m *MockCollector) LastCountStatus() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastCountStatus
}

// Reset clears all recorded metrics.
func (m *MockCollector) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.durations = make([]DurationRecord, 0)
	m.counts = make([]CountRecord, 0)
	m.values = make([]MetricRecord, 0)
	m.lastOperation = ""
	m.lastDuration = 0
	m.lastCountStatus = ""
}

// AssertDurationRecorded checks if a duration was recorded for the given operation.
func (m *MockCollector) AssertDurationRecorded(operation string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, record := range m.durations {
		if record.Operation == operation {
			return true
		}
	}
	return false
}

// AssertCountRecorded checks if a count was recorded for the given operation and status.
func (m *MockCollector) AssertCountRecorded(operation, status string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, record := range m.counts {
		if record.Operation == operation && record.Status == status {
			return true
		}
	}
	return false
}

// CountDurations returns the number of duration recordings.
func (m *MockCollector) CountDurations() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.durations)
}

// CountCounts returns the number of count recordings.
func (m *MockCollector) CountCounts() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.counts)
}

// CountValues returns the number of value recordings.
func (m *MockCollector) CountValues() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.values)
}

// TotalDuration returns the total duration recorded for an operation.
func (m *MockCollector) TotalDuration(operation string) time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var total time.Duration
	for _, record := range m.durations {
		if record.Operation == operation {
			total += record.Duration
		}
	}
	return total
}
