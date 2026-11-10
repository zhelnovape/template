package telemetry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/zhelnovape/template/internal/config"
)

type Telemetry struct {
	TracerProvider *sdktrace.TracerProvider
	MeterProvider  *sdkmetric.MeterProvider
	Propagator     propagation.TextMapPropagator
}

func New(ctx context.Context, cfg config.Telemetry, service string, logger *slog.Logger) (*Telemetry, error) {
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		logger.Warn("telemetry export failed", "error", err)
	}))

	res, err := resource.New(ctx,
		resource.WithAttributes(
			attribute.String("service.name", service),
			attribute.String("service.version", buildVersion()),
		),
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
	)
	if err != nil {
		return nil, fmt.Errorf("create resource: %w", err)
	}

	spanExporter, metricExporter, err := newExporters(ctx, cfg.Protocol)
	if err != nil {
		return nil, err
	}

	logger.Info("telemetry configured", "endpoint", cfg.Endpoint, "protocol", cfg.Protocol)

	return &Telemetry{
		TracerProvider: sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(spanExporter),
			sdktrace.WithResource(res),
		),
		MeterProvider: sdkmetric.NewMeterProvider(
			sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
			sdkmetric.WithResource(res),
		),
		Propagator: propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	}, nil
}

func (t *Telemetry) Shutdown(ctx context.Context) error {
	return errors.Join(
		t.TracerProvider.Shutdown(ctx),
		t.MeterProvider.Shutdown(ctx),
	)
}

func newExporters(ctx context.Context, protocol string) (sdktrace.SpanExporter, sdkmetric.Exporter, error) {
	if protocol == "grpc" {
		spanExporter, err := otlptracegrpc.New(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("create grpc span exporter: %w", err)
		}
		metricExporter, err := otlpmetricgrpc.New(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("create grpc metric exporter: %w", err)
		}
		return spanExporter, metricExporter, nil
	}

	spanExporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("create http span exporter: %w", err)
	}
	metricExporter, err := otlpmetrichttp.New(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("create http metric exporter: %w", err)
	}
	return spanExporter, metricExporter, nil
}

func buildVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" || info.Main.Version == "(devel)" {
		return "dev"
	}
	return info.Main.Version
}
