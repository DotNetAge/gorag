package rag

import (
	"context"

	"errors"
	"testing"
	"time"

	"github.com/DotNetAge/gorag/core"
	"github.com/DotNetAge/gorag/vectorstore"
	"github.com/stretchr/testify/assert"
)

func TestNewHealthChecker(t *testing.T) {
	engine := &Engine{}
	hc := NewHealthChecker(engine)

	assert.NotNil(t, hc)
	assert.Equal(t, "0.5.0", hc.version)
	assert.Equal(t, 5*time.Second, hc.timeout)
}

func TestHealthChecker_WithTimeout(t *testing.T) {
	engine := &Engine{}
	hc := NewHealthChecker(engine).WithTimeout(10 * time.Second)

	assert.Equal(t, 10*time.Second, hc.timeout)
}

func TestHealthChecker_WithVersion(t *testing.T) {
	engine := &Engine{}
	hc := NewHealthChecker(engine).WithVersion("1.0.0")

	assert.Equal(t, "1.0.0", hc.version)
}

func TestHealthChecker_AllHealthy(t *testing.T) {
	engine := &Engine{
		store:    &mockStore{},
		embedder: &mockEmbedder{},
		llm:      &mockLLM{},
	}

	hc := NewHealthChecker(engine)
	report := hc.Check(context.Background())

	assert.Equal(t, HealthStatusUp, report.Status)
	assert.Len(t, report.Components, 3)
	assert.NotZero(t, report.Timestamp)
	assert.NotZero(t, report.Uptime)
	assert.Equal(t, "0.5.0", report.Version)

	for _, c := range report.Components {
		assert.Equal(t, HealthStatusUp, c.Status)
		assert.Empty(t, c.Error)
	}
}

func TestHealthChecker_VectorStoreDown(t *testing.T) {
	engine := &Engine{
		store: &mockStore{
			searchFunc: func(ctx context.Context, query []float32, opts vectorstore.SearchOptions) ([]core.Result, error) {
				return nil, errors.New("connection refused")
			},
		},
		embedder: &mockEmbedder{},
		llm:      &mockLLM{},
	}

	hc := NewHealthChecker(engine)
	report := hc.Check(context.Background())

	assert.Equal(t, HealthStatusDown, report.Status)

	var vsHealth ComponentHealth
	for _, c := range report.Components {
		if c.Name == "vectorstore" {
			vsHealth = c
			break
		}
	}

	assert.Equal(t, HealthStatusDown, vsHealth.Status)
	assert.Contains(t, vsHealth.Error, "connection refused")
}

func TestHealthChecker_EmbedderDown(t *testing.T) {
	engine := &Engine{
		store: &mockStore{},
		embedder: &mockEmbedder{
			embedFunc: func(ctx context.Context, texts []string) ([][]float32, error) {
				return nil, errors.New("API key invalid")
			},
		},
		llm: &mockLLM{},
	}

	hc := NewHealthChecker(engine)
	report := hc.Check(context.Background())

	assert.Equal(t, HealthStatusDown, report.Status)

	var embHealth ComponentHealth
	for _, c := range report.Components {
		if c.Name == "embedder" {
			embHealth = c
			break
		}
	}

	assert.Equal(t, HealthStatusDown, embHealth.Status)
	assert.Contains(t, embHealth.Error, "API key invalid")
}

func TestHealthChecker_LLMDegraded(t *testing.T) {
	engine := &Engine{
		store:    &mockStore{},
		embedder: &mockEmbedder{},
		llm: &mockLLM{
			completeFunc: func(ctx context.Context, prompt string) (string, error) {
				return "", errors.New("rate limit exceeded")
			},
		},
	}

	hc := NewHealthChecker(engine)
	report := hc.Check(context.Background())

	assert.Equal(t, HealthStatusDegraded, report.Status)

	var llmHealth ComponentHealth
	for _, c := range report.Components {
		if c.Name == "llm" {
			llmHealth = c
			break
		}
	}

	assert.Equal(t, HealthStatusDegraded, llmHealth.Status)
	assert.Contains(t, llmHealth.Error, "rate limit exceeded")
}

func TestHealthChecker_NilComponents(t *testing.T) {
	engine := &Engine{}

	hc := NewHealthChecker(engine)
	report := hc.Check(context.Background())

	assert.Equal(t, HealthStatusDown, report.Status)

	for _, c := range report.Components {
		assert.Equal(t, HealthStatusDown, c.Status)
		assert.NotEmpty(t, c.Error)
	}
}

func TestHealthChecker_MixedStatus(t *testing.T) {
	engine := &Engine{
		store:    &mockStore{},
		embedder: &mockEmbedder{},
		llm: &mockLLM{
			completeFunc: func(ctx context.Context, prompt string) (string, error) {
				return "", errors.New("timeout")
			},
		},
	}

	hc := NewHealthChecker(engine)
	report := hc.Check(context.Background())

	assert.Equal(t, HealthStatusDegraded, report.Status)
}

func TestDetermineOverallStatus(t *testing.T) {
	hc := &HealthChecker{}

	tests := []struct {
		name       string
		components []ComponentHealth
		expected   HealthStatus
	}{
		{
			name: "all up",
			components: []ComponentHealth{
				{Status: HealthStatusUp},
				{Status: HealthStatusUp},
			},
			expected: HealthStatusUp,
		},
		{
			name: "one degraded",
			components: []ComponentHealth{
				{Status: HealthStatusUp},
				{Status: HealthStatusDegraded},
			},
			expected: HealthStatusDegraded,
		},
		{
			name: "one down",
			components: []ComponentHealth{
				{Status: HealthStatusUp},
				{Status: HealthStatusDown},
			},
			expected: HealthStatusDown,
		},
		{
			name: "down takes priority over degraded",
			components: []ComponentHealth{
				{Status: HealthStatusDegraded},
				{Status: HealthStatusDown},
			},
			expected: HealthStatusDown,
		},
		{
			name:       "empty components",
			components: []ComponentHealth{},
			expected:   HealthStatusUp,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hc.determineOverallStatus(tt.components)
			assert.Equal(t, tt.expected, result)
		})
	}
}
