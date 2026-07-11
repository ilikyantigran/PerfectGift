// Package telemetry wires the three observability concerns for a service:
// structured logging (slog JSON, trace-correlated), distributed tracing
// (OpenTelemetry → OTLP/Jaeger), and metrics (Prometheus). It is intentionally
// self-contained and duplicated per service (each is its own Go module and its
// Docker build context is the service directory). Copy this file as-is; the only
// thing that varies per service is the serviceName you pass to Setup.
package telemetry

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	logkit "github.com/ilikyantigran/PerfectGift/services/backend/poll/internal/logkit"
)

type Provider struct {
	registry  *prometheus.Registry
	shutdowns []func(context.Context) error
}

// Setup installs a JSON slog logger (with trace correlation), a Prometheus
// metrics registry, and — when OTEL_EXPORTER_OTLP_ENDPOINT is set — an OTLP
// trace exporter. Call Shutdown on exit.
func Setup(ctx context.Context, serviceName string) (*Provider, error) {
	// logkit installs the JSON slog logger (stdout, trace-correlated) AND ships logs to
	// the central log-server with on-disk store-and-forward. Stdout-only (non-mandatory)
	// when LOG_SERVER_URL is unset. Replaces the old inline contextHandler.
	logFlush := logkit.Install(serviceName)

	res, err := resource.New(ctx, resource.WithAttributes(attribute.String("service.name", serviceName)))
	if err != nil {
		return nil, err
	}

	p := &Provider{}
	p.shutdowns = append(p.shutdowns, func(c context.Context) error { logFlush(c); return nil })

	if endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); endpoint != "" {
		exp, err := otlptracegrpc.New(ctx, otlptracegrpc.WithEndpoint(endpoint), otlptracegrpc.WithInsecure())
		if err != nil {
			return nil, err
		}
		tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exp), sdktrace.WithResource(res))
		otel.SetTracerProvider(tp)
		p.shutdowns = append(p.shutdowns, tp.Shutdown)
		slog.Info("tracing enabled", "otlp_endpoint", endpoint)
	}
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	registry := prometheus.NewRegistry()
	registry.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	reader, err := otelprom.New(otelprom.WithRegisterer(registry))
	if err != nil {
		return nil, err
	}
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader), sdkmetric.WithResource(res))
	otel.SetMeterProvider(mp)
	p.shutdowns = append(p.shutdowns, mp.Shutdown)
	p.registry = registry

	return p, nil
}

// MetricsHandler is the /metrics endpoint Prometheus scrapes.
func (p *Provider) MetricsHandler() http.Handler {
	return promhttp.HandlerFor(p.registry, promhttp.HandlerOpts{})
}

func (p *Provider) Shutdown(ctx context.Context) {
	for _, fn := range p.shutdowns {
		_ = fn(ctx)
	}
}
