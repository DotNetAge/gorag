package observability

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
)

type otelTracer struct {
	tracer trace.Tracer
}

type otelSpan struct {
	span trace.Span
}

// DefaultOpenTelemetryTracer creates a new OTel tracer.
func DefaultOpenTelemetryTracer(ctx context.Context, endpoint string, serviceName string) (Tracer, error) {
	exp, err := otlptracegrpc.New(ctx, otlptracegrpc.WithEndpoint(endpoint), otlptracegrpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	bsp := sdktrace.NewBatchSpanProcessor(exp)
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)

	otel.SetTracerProvider(tracerProvider)
	
	return &otelTracer{
		tracer: otel.Tracer(serviceName),
	}, nil
}

func (t *otelTracer) StartSpan(ctx context.Context, operationName string) (context.Context, Span) {
	newCtx, span := t.tracer.Start(ctx, operationName)
	return newCtx, &otelSpan{span: span}
}

func (t *otelTracer) GetSpan(ctx context.Context) Span {
	span := trace.SpanFromContext(ctx)
	if span == nil {
		return &noopSpan{}
	}
	return &otelSpan{span: span}
}

func (s *otelSpan) SetTag(key string, value interface{}) {
	s.span.SetAttributes(attribute.String(key, fmt.Sprintf("%v", value)))
}

func (s *otelSpan) LogEvent(eventName string, fields map[string]interface{}) {
	attrs := make([]attribute.KeyValue, 0, len(fields))
	for k, v := range fields {
		attrs = append(attrs, attribute.String(k, fmt.Sprintf("%v", v)))
	}
	s.span.AddEvent(eventName, trace.WithAttributes(attrs...))
}

func (s *otelSpan) End() {
	s.span.End()
}