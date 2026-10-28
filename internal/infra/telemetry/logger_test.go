package telemetry_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/zhelnovape/template/internal/infra/telemetry"
)

const service = "test-service"

func decode(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()

	var record map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &record))
	return record
}

func TestNewLogger_AddsTraceIDInsideSpan(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger, err := telemetry.NewLogger(&buf, "info", service)
	require.NoError(t, err)

	ctx, span := sdktrace.NewTracerProvider().Tracer("test").Start(context.Background(), "test")
	logger.InfoContext(ctx, "trip created")
	span.End()

	record := decode(t, &buf)
	assert.Equal(t, service, record["service"])
	assert.Equal(t, span.SpanContext().TraceID().String(), record["trace_id"])
	assert.Equal(t, span.SpanContext().SpanID().String(), record["span_id"])
}

func TestNewLogger_WithoutSpan(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger, err := telemetry.NewLogger(&buf, "info", service)
	require.NoError(t, err)

	logger.InfoContext(context.Background(), "service started")

	record := decode(t, &buf)
	assert.NotContains(t, record, "trace_id")
	assert.NotContains(t, record, "span_id")
}

func TestNewLogger_Level(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger, err := telemetry.NewLogger(&buf, "warn", service)
	require.NoError(t, err)

	logger.Info("hidden")
	assert.Empty(t, buf.String())

	_, err = telemetry.NewLogger(&buf, "loud", service)
	require.Error(t, err)
}
