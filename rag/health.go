package rag

import (
	"context"
	"github.com/DotNetAge/gorag/utils/llmutil"
	"time"

	"github.com/DotNetAge/gorag/vectorstore"
)

// HealthStatus represents the health status of a component
type HealthStatus string

const (
	// HealthStatusUp indicates the component is healthy
	HealthStatusUp HealthStatus = "up"
	// HealthStatusDown indicates the component is unhealthy
	HealthStatusDown HealthStatus = "down"
	// HealthStatusDegraded indicates the component is partially healthy
	HealthStatusDegraded HealthStatus = "degraded"
)

// ComponentHealth represents the health of a single component
type ComponentHealth struct {
	Name    string            `json:"name"`
	Status  HealthStatus      `json:"status"`
	Latency time.Duration     `json:"latency"`
	Error   string            `json:"error,omitempty"`
	Details map[string]string `json:"details,omitempty"`
}

// HealthReport represents the overall health of the RAG engine
type HealthReport struct {
	Status     HealthStatus      `json:"status"`
	Components []ComponentHealth `json:"components"`
	Timestamp  time.Time         `json:"timestamp"`
	Uptime     time.Duration     `json:"uptime"`
	Version    string            `json:"version"`
}

// HealthChecker performs health checks on engine components
type HealthChecker struct {
	engine    *Engine
	startTime time.Time
	version   string
	timeout   time.Duration
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(engine *Engine) *HealthChecker {
	return &HealthChecker{
		engine:    engine,
		startTime: time.Now(),
		version:   "0.5.0",
		timeout:   5 * time.Second,
	}
}

// WithTimeout sets the health check timeout
func (h *HealthChecker) WithTimeout(timeout time.Duration) *HealthChecker {
	h.timeout = timeout
	return h
}

// WithVersion sets the version string
func (h *HealthChecker) WithVersion(version string) *HealthChecker {
	h.version = version
	return h
}

// Check performs a full health check
func (h *HealthChecker) Check(ctx context.Context) *HealthReport {
	ctx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()

	report := &HealthReport{
		Status:     HealthStatusUp,
		Components: []ComponentHealth{},
		Timestamp:  time.Now(),
		Uptime:     time.Since(h.startTime),
		Version:    h.version,
	}

	// Check vector store
	report.Components = append(report.Components, h.checkVectorStore(ctx))

	// Check embedder
	report.Components = append(report.Components, h.checkEmbedder(ctx))

	// Check LLM
	report.Components = append(report.Components, h.checkLLM(ctx))

	// Determine overall status
	report.Status = h.determineOverallStatus(report.Components)

	return report
}

// checkVectorStore checks the vector store health
func (h *HealthChecker) checkVectorStore(ctx context.Context) ComponentHealth {
	health := ComponentHealth{
		Name:    "vectorstore",
		Status:  HealthStatusUp,
		Details: make(map[string]string),
	}

	if h.engine.store == nil {
		health.Status = HealthStatusDown
		health.Error = "vector store not configured"
		return health
	}

	start := time.Now()

	// Try a simple search to verify connectivity
	_, err := h.engine.store.Search(ctx, []float32{0.0}, vectorstore.SearchOptions{TopK: 1})
	health.Latency = time.Since(start)

	if err != nil {
		health.Status = HealthStatusDown
		health.Error = err.Error()
	}

	return health
}

// checkEmbedder checks the embedder health
func (h *HealthChecker) checkEmbedder(ctx context.Context) ComponentHealth {
	health := ComponentHealth{
		Name:    "embedder",
		Status:  HealthStatusUp,
		Details: make(map[string]string),
	}

	if h.engine.embedder == nil {
		health.Status = HealthStatusDown
		health.Error = "embedder not configured"
		return health
	}

	start := time.Now()

	// Try a simple embedding to verify connectivity
	_, err := h.engine.embedder.Embed(ctx, []string{"health check"})
	health.Latency = time.Since(start)

	if err != nil {
		health.Status = HealthStatusDown
		health.Error = err.Error()
	}

	return health
}

// checkLLM checks the LLM client health
func (h *HealthChecker) checkLLM(ctx context.Context) ComponentHealth {
	health := ComponentHealth{
		Name:    "llm",
		Status:  HealthStatusUp,
		Details: make(map[string]string),
	}

	if h.engine.llm == nil {
		health.Status = HealthStatusDown
		health.Error = "LLM client not configured"
		return health
	}

	start := time.Now()

	// Try a simple completion to verify connectivity
	_, err := llmutil.Complete(ctx, h.engine.llm, "ping")
	health.Latency = time.Since(start)

	if err != nil {
		// LLM errors might be rate limiting, treat as degraded
		health.Status = HealthStatusDegraded
		health.Error = err.Error()
	}

	return health
}

// determineOverallStatus determines the overall health status
func (h *HealthChecker) determineOverallStatus(components []ComponentHealth) HealthStatus {
	hasDown := false
	hasDegraded := false

	for _, c := range components {
		switch c.Status {
		case HealthStatusDown:
			hasDown = true
		case HealthStatusDegraded:
			hasDegraded = true
		}
	}

	if hasDown {
		return HealthStatusDown
	}
	if hasDegraded {
		return HealthStatusDegraded
	}
	return HealthStatusUp
}
