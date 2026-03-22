// Package tracing initialises an OpenTelemetry trace provider that exports
// spans to a Jaeger-compatible OTLP/HTTP endpoint.  If the endpoint is empty
// the function returns immediately and a no-op tracer is used everywhere.
package tracing

import (
	"context"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// InitProvider configures the global OTel trace provider and text-map propagator.
// It exports spans over OTLP/HTTP to `endpoint` (e.g. "jaeger:4318").
// Pass an empty string to skip instrumentation entirely.
// The returned function must be called on shutdown (typically via defer).
func InitProvider(serviceName, endpoint string) func() {
	if endpoint == "" {
		return func() {}
	}

	ctx := context.Background()

	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(endpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		slog.Warn("tracing: failed to create OTLP exporter — tracing disabled", "error", err)
		return func() {}
	}

	res, _ := resource.New(ctx,
		resource.WithAttributes(attribute.String("service.name", serviceName)),
	)

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tp.Shutdown(ctx); err != nil {
			slog.Warn("tracing: shutdown error", "error", err)
		}
	}
}
