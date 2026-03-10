package observability

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics defines the interface for metrics collection
//
// This interface defines methods for recording various metrics related to
// the RAG engine, including latencies, counts, and system metrics.
//
// Example:
//
//     type CustomMetrics struct {
//         // Custom metrics implementation
//     }
//
//     func (m *CustomMetrics) RecordQueryLatency(ctx context.Context, duration time.Duration) {
//         // Record query latency
//     }
//
//     // Implement other methods...
type Metrics interface {
	// RecordQueryLatency records the latency of a query
	//
	// Parameters:
	// - ctx: Context for cancellation
	// - duration: Duration of the query
	RecordQueryLatency(ctx context.Context, duration time.Duration)
	
	// RecordIndexLatency records the latency of an index operation
	//
	// Parameters:
	// - ctx: Context for cancellation
	// - duration: Duration of the index operation
	RecordIndexLatency(ctx context.Context, duration time.Duration)
	
	// RecordQueryCount records the number of queries
	//
	// Parameters:
	// - ctx: Context for cancellation
	// - status: Status of the query (e.g., "success", "error")
	RecordQueryCount(ctx context.Context, status string)
	
	// RecordIndexCount records the number of index operations
	//
	// Parameters:
	// - ctx: Context for cancellation
	// - status: Status of the index operation (e.g., "success", "error")
	RecordIndexCount(ctx context.Context, status string)
	
	// RecordErrorCount records the number of errors
	//
	// Parameters:
	// - ctx: Context for cancellation
	// - errorType: Type of error
	RecordErrorCount(ctx context.Context, errorType string)
	
	// RecordIndexedDocuments records the number of indexed documents
	//
	// Parameters:
	// - ctx: Context for cancellation
	// - count: Number of indexed documents
	RecordIndexedDocuments(ctx context.Context, count int)
	
	// RecordIndexingDocuments records the number of documents being indexed
	//
	// Parameters:
	// - ctx: Context for cancellation
	// - count: Number of documents being indexed
	RecordIndexingDocuments(ctx context.Context, count int)
	
	// RecordMonitoredDocuments records the number of monitored documents
	//
	// Parameters:
	// - ctx: Context for cancellation
	// - count: Number of monitored documents
	RecordMonitoredDocuments(ctx context.Context, count int)
	
	// RecordSystemMetrics records system metrics (CPU, memory)
	//
	// Parameters:
	// - ctx: Context for cancellation
	// - cpuUsage: CPU usage percentage
	// - memoryUsage: Memory usage in MB
	RecordSystemMetrics(ctx context.Context, cpuUsage float64, memoryUsage float64)
	
	// Register registers the metrics with the underlying system
	//
	// Returns:
	// - error: Error if registration fails
	Register() error
}

// PrometheusMetrics implements metrics collection using Prometheus
//
// This implementation uses Prometheus to collect and expose metrics
// about the RAG engine's performance and health.
//
// Example:
//
//     metrics := NewPrometheusMetrics()
//     if err := metrics.Register(); err != nil {
//         log.Fatal(err)
//     }
//     
//     // Record a query latency
//     start := time.Now()
//     // Perform query...
//     metrics.RecordQueryLatency(ctx, time.Since(start))
//     metrics.RecordQueryCount(ctx, "success")
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
//
// Returns:
// - *PrometheusMetrics: New Prometheus metrics collector instance
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
//
// Parameters:
// - ctx: Context for cancellation
// - duration: Duration of the query
func (m *PrometheusMetrics) RecordQueryLatency(ctx context.Context, duration time.Duration) {
	m.queryLatency.Observe(duration.Seconds())
}

// RecordIndexLatency records the latency of an index operation
//
// Parameters:
// - ctx: Context for cancellation
// - duration: Duration of the index operation
func (m *PrometheusMetrics) RecordIndexLatency(ctx context.Context, duration time.Duration) {
	m.indexLatency.Observe(duration.Seconds())
}

// RecordQueryCount records the number of queries
//
// Parameters:
// - ctx: Context for cancellation
// - status: Status of the query (e.g., "success", "error")
func (m *PrometheusMetrics) RecordQueryCount(ctx context.Context, status string) {
	m.queryCount.Inc()
}

// RecordIndexCount records the number of index operations
//
// Parameters:
// - ctx: Context for cancellation
// - status: Status of the index operation (e.g., "success", "error")
func (m *PrometheusMetrics) RecordIndexCount(ctx context.Context, status string) {
	m.indexCount.Inc()
}

// RecordErrorCount records the number of errors
//
// Parameters:
// - ctx: Context for cancellation
// - errorType: Type of error
func (m *PrometheusMetrics) RecordErrorCount(ctx context.Context, errorType string) {
	m.errorCount.Inc()
}

// RecordIndexedDocuments records the number of indexed documents
//
// Parameters:
// - ctx: Context for cancellation
// - count: Number of indexed documents
func (m *PrometheusMetrics) RecordIndexedDocuments(ctx context.Context, count int) {
	m.indexedDocuments.Set(float64(count))
}

// RecordIndexingDocuments records the number of documents being indexed
//
// Parameters:
// - ctx: Context for cancellation
// - count: Number of documents being indexed
func (m *PrometheusMetrics) RecordIndexingDocuments(ctx context.Context, count int) {
	m.indexingDocuments.Set(float64(count))
}

// RecordMonitoredDocuments records the number of monitored documents
//
// Parameters:
// - ctx: Context for cancellation
// - count: Number of monitored documents
func (m *PrometheusMetrics) RecordMonitoredDocuments(ctx context.Context, count int) {
	m.monitoredDocuments.Set(float64(count))
}

// RecordSystemMetrics records system metrics (CPU, memory)
//
// Parameters:
// - ctx: Context for cancellation
// - cpuUsage: CPU usage percentage
// - memoryUsage: Memory usage in MB
func (m *PrometheusMetrics) RecordSystemMetrics(ctx context.Context, cpuUsage float64, memoryUsage float64) {
	m.cpuUsage.Set(cpuUsage)
	m.memoryUsage.Set(memoryUsage)
}

// Register registers the metrics with Prometheus
//
// Returns:
// - error: Error if registration fails
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
