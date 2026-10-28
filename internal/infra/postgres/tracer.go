package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
)

const meterName = "github.com/zhelnovape/template/internal/infra/postgres"

type QueryTracer struct {
	spans    *otelpgx.Tracer
	duration metric.Float64Histogram
}

type queryKey struct{}

type queryInfo struct {
	start      time.Time
	operation  string
	collection string
}

func NewQueryTracer(tp trace.TracerProvider, mp metric.MeterProvider) (*QueryTracer, error) {
	duration, err := mp.Meter(meterName).Float64Histogram(
		"db.client.operation.duration",
		metric.WithUnit("s"),
		metric.WithDescription("Duration of database client operations."),
		metric.WithExplicitBucketBoundaries(0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 5, 10),
	)
	if err != nil {
		return nil, fmt.Errorf("create db duration histogram: %w", err)
	}

	return &QueryTracer{
		spans: otelpgx.NewTracer(
			otelpgx.WithTracerProvider(tp),
			otelpgx.WithMeterProvider(noop.NewMeterProvider()),
			otelpgx.WithSpanNameFunc(spanName),
		),
		duration: duration,
	}, nil
}

func (t *QueryTracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	info := queryInfo{
		start:      time.Now(),
		operation:  operationName(data.SQL),
		collection: collectionName(data.SQL),
	}

	ctx = t.spans.TraceQueryStart(ctx, conn, data)
	trace.SpanFromContext(ctx).SetAttributes(info.attributes()...)

	return context.WithValue(ctx, queryKey{}, info)
}

func (t *QueryTracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	if info, ok := ctx.Value(queryKey{}).(queryInfo); ok {
		attrs := info.attributes()
		if data.Err != nil {
			errType := attribute.String("error.type", errorType(data.Err))
			trace.SpanFromContext(ctx).SetAttributes(errType)
			attrs = append(attrs, errType)
		}
		t.duration.Record(ctx, time.Since(info.start).Seconds(), metric.WithAttributes(attrs...))
	}

	t.spans.TraceQueryEnd(ctx, conn, data)
}

func (i queryInfo) attributes() []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		attribute.String("db.system.name", "postgresql"),
		attribute.String("db.operation.name", i.operation),
	}
	if i.collection != "" {
		attrs = append(attrs, attribute.String("db.collection.name", i.collection))
	}
	return attrs
}

func spanName(sql string) string {
	return strings.TrimSpace(operationName(sql) + " " + collectionName(sql))
}

func operationName(sql string) string {
	fields := strings.Fields(sql)
	if len(fields) == 0 {
		return "UNKNOWN"
	}
	return strings.ToUpper(fields[0])
}

func collectionName(sql string) string {
	fields := strings.Fields(sql)
	for i := 0; i < len(fields)-1; i++ {
		switch strings.ToUpper(fields[i]) {
		case "FROM", "INTO", "UPDATE":
			return strings.Trim(fields[i+1], `"(;`)
		}
	}
	return ""
}

func errorType(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return "_OTHER"
}
