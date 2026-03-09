package observability

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics defines the interface for metrics collection
type Metrics interface {
	// RecordQueryLatency records the latency of a query
	RecordQueryLatency(ctx context.Context, duration time.Duration)
	// RecordIndexLatency records the latency of an index operation
	RecordIndexLatency(ctx context.Context, duration time.Duration)
	// RecordQueryCount records the number of queries
	RecordQueryCount(ctx context.Context, status string)
	// RecordIndexCount records the number of index operations
	RecordIndexCount(ctx context.Context, status string)
	// RecordErrorCount records the number of errors
	RecordErrorCount(ctx context.Context, errorType string)
	// RecordIndexedDocuments records the number of indexed documents
	RecordIndexedDocuments(ctx context.Context, count int)
	// RecordIndexingDocuments records the number of documents being indexed
	RecordIndexingDocuments(ctx context.Context, count int)
	// RecordMonitoredDocuments records the number of monitored documents
	RecordMonitoredDocuments(ctx context.Context, count int)
	// RecordSystemMetrics records system metrics (CPU, memory)
	RecordSystemMetrics(ctx context.Context, cpuUsage float64, memoryUsage float64)
	// Register registers the metrics with Prometheus
	Register() error
}

// PrometheusMetrics implements metrics collection using Prometheus
type PrometheusMetrics struct {
	queryLatency         prometheus.Histogram
	indexLatency         prometheus.Histogram
	queryCount           prometheus.Counter
	indexCount           prometheus.Counter
	errorCount           prometheus.Counter
	indexedDocuments     prometheus.Gauge
	indexingDocuments    prometheus.Gauge
	monitoredDocuments   prometheus.Gauge
	cpuUsage             prometheus.Gauge
	memoryUsage          prometheus.Gauge
}

// NewPrometheusMetrics creates a new Prometheus metrics collector
func NewPrometheusMetrics() *PrometheusMetrics {
	queryLatency := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "gorag_query_latency_seconds",
		Help:    "Latency of RAG queries in seconds",
		Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
	})

	indexLatency := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "gorag_index_latency_seconds",
		Help:    "Latency of RAG index operations in seconds",
		Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
	})

	queryCount := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "gorag_query_count",
		Help: "Number of RAG queries",
	})

	indexCount := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "gorag_index_count",
		Help: "Number of RAG index operations",
	})

	errorCount := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "gorag_error_count",
		Help: "Number of RAG errors",
	})

	indexedDocuments := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "gorag_indexed_documents",
		Help: "Number of indexed documents",
	})

	indexingDocuments := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "gorag_indexing_documents",
		Help: "Number of documents being indexed",
	})

	monitoredDocuments := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "gorag_monitored_documents",
		Help: "Number of monitored documents",
	})

	cpuUsage := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "gorag_cpu_usage_percent",
		Help: "CPU usage percentage",
	})

	memoryUsage := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "gorag_memory_usage_mb",
		Help: "Memory usage in MB",
	})

	return &PrometheusMetrics{
		queryLatency:         queryLatency,
		indexLatency:         indexLatency,
		queryCount:           queryCount,
		indexCount:           indexCount,
		errorCount:           errorCount,
		indexedDocuments:     indexedDocuments,
		indexingDocuments:    indexingDocuments,
		monitoredDocuments:   monitoredDocuments,
		cpuUsage:             cpuUsage,
		memoryUsage:          memoryUsage,
	}
}

// RecordQueryLatency records the latency of a query
func (m *PrometheusMetrics) RecordQueryLatency(ctx context.Context, duration time.Duration) {
	m.queryLatency.Observe(duration.Seconds())
}

// RecordIndexLatency records the latency of an index operation
func (m *PrometheusMetrics) RecordIndexLatency(ctx context.Context, duration time.Duration) {
	m.indexLatency.Observe(duration.Seconds())
}

// RecordQueryCount records the number of queries
func (m *PrometheusMetrics) RecordQueryCount(ctx context.Context, status string) {
	m.queryCount.Inc()
}

// RecordIndexCount records the number of index operations
func (m *PrometheusMetrics) RecordIndexCount(ctx context.Context, status string) {
	m.indexCount.Inc()
}

// RecordErrorCount records the number of errors
func (m *PrometheusMetrics) RecordErrorCount(ctx context.Context, errorType string) {
	m.errorCount.Inc()
}

// RecordIndexedDocuments records the number of indexed documents
func (m *PrometheusMetrics) RecordIndexedDocuments(ctx context.Context, count int) {
	m.indexedDocuments.Set(float64(count))
}

// RecordIndexingDocuments records the number of documents being indexed
func (m *PrometheusMetrics) RecordIndexingDocuments(ctx context.Context, count int) {
	m.indexingDocuments.Set(float64(count))
}

// RecordMonitoredDocuments records the number of monitored documents
func (m *PrometheusMetrics) RecordMonitoredDocuments(ctx context.Context, count int) {
	m.monitoredDocuments.Set(float64(count))
}

// RecordSystemMetrics records system metrics (CPU, memory)
func (m *PrometheusMetrics) RecordSystemMetrics(ctx context.Context, cpuUsage float64, memoryUsage float64) {
	m.cpuUsage.Set(cpuUsage)
	m.memoryUsage.Set(memoryUsage)
}

// Register registers the metrics with Prometheus
func (m *PrometheusMetrics) Register() error {
	if err := prometheus.Register(m.queryLatency); err != nil {
		return err
	}
	if err := prometheus.Register(m.indexLatency); err != nil {
		return err
	}
	if err := prometheus.Register(m.queryCount); err != nil {
		return err
	}
	if err := prometheus.Register(m.indexCount); err != nil {
		return err
	}
	if err := prometheus.Register(m.errorCount); err != nil {
		return err
	}
	if err := prometheus.Register(m.indexedDocuments); err != nil {
		return err
	}
	if err := prometheus.Register(m.indexingDocuments); err != nil {
		return err
	}
	if err := prometheus.Register(m.monitoredDocuments); err != nil {
		return err
	}
	if err := prometheus.Register(m.cpuUsage); err != nil {
		return err
	}
	if err := prometheus.Register(m.memoryUsage); err != nil {
		return err
	}
	return nil
}
