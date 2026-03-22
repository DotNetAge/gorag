package observability

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultNoopCollector(t *testing.T) {
	collector := DefaultNoopCollector()
	assert.NotNil(t, collector)
	_, ok := collector.(Collector)
	assert.True(t, ok)
}

func TestNoopCollector_RecordDuration(t *testing.T) {
	collector := &noopCollector{}
	collector.RecordDuration("test_op", time.Second, map[string]string{"key": "value"})
}

func TestNoopCollector_RecordCount(t *testing.T) {
	collector := &noopCollector{}
	collector.RecordCount("test_op", "success", map[string]string{"key": "value"})
}

func TestNoopCollector_RecordValue(t *testing.T) {
	collector := &noopCollector{}
	collector.RecordValue("test_metric", 42.0, map[string]string{"key": "value"})
}

func TestDefaultNoopTracer(t *testing.T) {
	tracer := DefaultNoopTracer()
	assert.NotNil(t, tracer)
	_, ok := tracer.(Tracer)
	assert.True(t, ok)
}

func TestNoopTracer_StartSpan(t *testing.T) {
	tracer := &noopTracer{}
	ctx := context.Background()
	newCtx, span := tracer.StartSpan(ctx, "test_span")
	assert.NotNil(t, newCtx)
	assert.NotNil(t, span)
}

func TestNoopTracer_GetSpan(t *testing.T) {
	tracer := &noopTracer{}
	ctx := context.Background()
	span := tracer.GetSpan(ctx)
	assert.NotNil(t, span)
}

func TestNoopSpan_AllMethodsNoOp(t *testing.T) {
	span := &noopSpan{}
	span.SetTag("key", "value")
	span.LogEvent("event", map[string]interface{}{"key": "value"})
	span.End()
}

func TestCollector_Interface(t *testing.T) {
	var c Collector = &noopCollector{}
	assert.NotNil(t, c)
}

func TestTracer_Interface(t *testing.T) {
	var tr Tracer = &noopTracer{}
	assert.NotNil(t, tr)
}

func TestSpan_Interface(t *testing.T) {
	var s Span = &noopSpan{}
	assert.NotNil(t, s)
}

func TestNoopCollector_MultipleOperations(t *testing.T) {
	collector := &noopCollector{}
	collector.RecordDuration("op1", 100*time.Millisecond, nil)
	collector.RecordDuration("op2", 200*time.Millisecond, map[string]string{"label": "value"})
	collector.RecordCount("op1", "success", nil)
	collector.RecordCount("op2", "error", map[string]string{"label": "value"})
	collector.RecordValue("metric1", 1.5, nil)
	collector.RecordValue("metric2", 2.5, map[string]string{"label": "value"})
}

func TestNoopTracer_MultipleSpans(t *testing.T) {
	tracer := &noopTracer{}
	ctx := context.Background()

	ctx1, span1 := tracer.StartSpan(ctx, "span1")
	assert.NotNil(t, span1)

	ctx2, span2 := tracer.StartSpan(ctx1, "span2")
	assert.NotNil(t, ctx2)
	assert.NotNil(t, span2)

	span1.End()
	span2.End()

	currentSpan := tracer.GetSpan(ctx)
	assert.NotNil(t, currentSpan)
}
