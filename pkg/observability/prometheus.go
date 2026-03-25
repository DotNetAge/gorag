package observability

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// PrometheusMetrics implements core.Metrics using Prometheus (duck typed)
type PrometheusMetrics struct {
	searchDuration      *prometheus.HistogramVec
	searchResult        *prometheus.CounterVec
	searchError         *prometheus.CounterVec
	indexingDuration    *prometheus.HistogramVec
	indexingResult      *prometheus.CounterVec
	embeddingCount      prometheus.Counter
	vectorStoreOpsCount *prometheus.CounterVec

	// RAG Specific Metrics
	queryCount *prometheus.CounterVec
	llmTokens  *prometheus.CounterVec
	ragQuality *prometheus.HistogramVec
}

// DefaultPrometheusMetrics creates a new prometheus-based metrics collector
// and optionally starts an HTTP server for scraping at the given address if addr != "".
func DefaultPrometheusMetrics(addr string) *PrometheusMetrics {
	m := &PrometheusMetrics{
		searchDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "gorag_search_duration_seconds",
				Help:    "Duration of search operations by engine",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"engine"},
		),
		searchResult: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gorag_search_result_count",
				Help: "Total number of search results by engine",
			},
			[]string{"engine"},
		),
		searchError: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gorag_search_error_count",
				Help: "Total number of search errors by engine",
			},
			[]string{"engine"},
		),
		indexingDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "gorag_indexing_duration_seconds",
				Help:    "Duration of indexing operations by parser",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"parser"},
		),
		indexingResult: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gorag_indexing_result_count",
				Help: "Total number of items indexed by parser",
			},
			[]string{"parser"},
		),
		embeddingCount: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "gorag_embedding_count",
				Help: "Total number of embeddings generated",
			},
		),
		vectorStoreOpsCount: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gorag_vectorstore_ops_count",
				Help: "Total number of vector store operations",
			},
			[]string{"operation"},
		),
		queryCount: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gorag_queries_total",
				Help: "Total number of RAG queries (for QPS calculation)",
			},
			[]string{"engine"},
		),
		llmTokens: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gorag_llm_tokens_total",
				Help: "Total number of LLM tokens consumed",
			},
			[]string{"model", "type"}, // prompt or completion
		),
		ragQuality: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "gorag_qa_quality_score",
				Help:    "Scores for Faithfulness, Relevance, and Precision",
				Buckets: []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0},
			},
			[]string{"metric"}, // faithfulness, relevance, precision
		),
	}

	prometheus.MustRegister(m.searchDuration)
	prometheus.MustRegister(m.searchResult)
	prometheus.MustRegister(m.searchError)
	prometheus.MustRegister(m.indexingDuration)
	prometheus.MustRegister(m.indexingResult)
	prometheus.MustRegister(m.embeddingCount)
	prometheus.MustRegister(m.vectorStoreOpsCount)
	prometheus.MustRegister(m.queryCount)
	prometheus.MustRegister(m.llmTokens)
	prometheus.MustRegister(m.ragQuality)

	if addr != "" {
		go func() {
			mux := http.NewServeMux()
			mux.Handle("/metrics", promhttp.Handler())
			// Ignoring error for background routine in init
			_ = http.ListenAndServe(addr, mux)
		}()
	}

	return m
}

func (m *PrometheusMetrics) RecordSearchDuration(engine string, duration any) {
	if d, ok := duration.(time.Duration); ok {
		m.searchDuration.WithLabelValues(engine).Observe(d.Seconds())
	}
}

func (m *PrometheusMetrics) RecordSearchResult(engine string, count int) {
	m.searchResult.WithLabelValues(engine).Add(float64(count))
}

func (m *PrometheusMetrics) RecordSearchError(engine string, err error) {
	if err != nil {
		m.searchError.WithLabelValues(engine).Inc()
	}
}

func (m *PrometheusMetrics) RecordIndexingDuration(parser string, duration any) {
	if d, ok := duration.(time.Duration); ok {
		m.indexingDuration.WithLabelValues(parser).Observe(d.Seconds())
	}
}

func (m *PrometheusMetrics) RecordIndexingResult(parser string, count int) {
	m.indexingResult.WithLabelValues(parser).Add(float64(count))
}

func (m *PrometheusMetrics) RecordEmbeddingCount(count int) {
	m.embeddingCount.Add(float64(count))
}

func (m *PrometheusMetrics) RecordVectorStoreOperations(op string, count int) {
	m.vectorStoreOpsCount.WithLabelValues(op).Add(float64(count))
}

func (m *PrometheusMetrics) RecordQueryCount(engine string) {
	m.queryCount.WithLabelValues(engine).Inc()
}

func (m *PrometheusMetrics) RecordLLMTokenUsage(model string, prompt int, completion int) {
	m.llmTokens.WithLabelValues(model, "prompt").Add(float64(prompt))
	m.llmTokens.WithLabelValues(model, "completion").Add(float64(completion))
}

func (m *PrometheusMetrics) RecordRAGEvaluation(metric string, score float32) {
	m.ragQuality.WithLabelValues(metric).Observe(float64(score))
}
