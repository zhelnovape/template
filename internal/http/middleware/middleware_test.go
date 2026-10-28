package middleware_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/zhelnovape/template/internal/generated/api"
	"github.com/zhelnovape/template/internal/http/middleware"
)

const (
	tripRoute = "/api/v1/trips/{tripId}"
	tripPath  = "/api/v1/trips/5cb72c04-8a04-4b7e-9f1e-1d2c3b4a5f60"
)

func newRouter(status int) *chi.Mux {
	router := chi.NewRouter()
	router.Get(tripRoute, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
	})
	router.Get("/panic", func(http.ResponseWriter, *http.Request) {
		panic("something broke")
	})
	return router
}

func serve(handler http.Handler, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, http.NoBody))
	return w
}

func TestTracing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		status     int
		wantStatus codes.Code
	}{
		{name: "success", status: http.StatusOK, wantStatus: codes.Unset},
		{name: "client error", status: http.StatusNotFound, wantStatus: codes.Unset},
		{name: "server error", status: http.StatusInternalServerError, wantStatus: codes.Error},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			recorder := tracetest.NewSpanRecorder()
			tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
			router := newRouter(tt.status)
			handler := middleware.Tracing(tp, propagation.TraceContext{}, router)(router)

			serve(handler, tripPath)

			spans := recorder.Ended()
			require.Len(t, spans, 1)
			assert.Equal(t, "GET "+tripRoute, spans[0].Name())
			assert.Contains(t, spans[0].Attributes(), attribute.String("http.route", tripRoute))
			assert.Equal(t, tt.wantStatus, spans[0].Status().Code)
		})
	}
}

func TestLogging(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		status    int
		wantLevel string
	}{
		{name: "success is info", status: http.StatusCreated, wantLevel: "INFO"},
		{name: "client error is info", status: http.StatusConflict, wantLevel: "INFO"},
		{name: "server error is error", status: http.StatusInternalServerError, wantLevel: "ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&buf, nil))
			router := newRouter(tt.status)

			serve(middleware.Logging(logger, router)(router), tripPath)

			lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
			require.Len(t, lines, 1)

			var record map[string]any
			require.NoError(t, json.Unmarshal([]byte(lines[0]), &record))
			assert.Equal(t, "request handled", record["msg"])
			assert.Equal(t, tt.wantLevel, record["level"])
			assert.Equal(t, http.MethodGet, record["method"])
			assert.Equal(t, tripRoute, record["route"])
			assert.InDelta(t, tt.status, record["status"], 0)
			assert.Contains(t, record, "duration_ms")
		})
	}
}

func TestMetrics(t *testing.T) {
	t.Parallel()

	reader := sdkmetric.NewManualReader()
	meter := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader)).Meter("test")
	router := newRouter(http.StatusOK)
	metrics, err := middleware.Metrics(meter, router)
	require.NoError(t, err)

	serve(metrics(router), tripPath)

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))

	found := false
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != "http.server.request.duration" {
				continue
			}
			hist, ok := m.Data.(metricdata.Histogram[float64])
			require.True(t, ok)
			require.Len(t, hist.DataPoints, 1)

			route, _ := hist.DataPoints[0].Attributes.Value("http.route")
			code, _ := hist.DataPoints[0].Attributes.Value("http.response.status_code")
			assert.Equal(t, tripRoute, route.AsString())
			assert.Equal(t, int64(http.StatusOK), code.AsInt64())
			assert.Equal(t, uint64(1), hist.DataPoints[0].Count)
			found = true
		}
	}
	assert.True(t, found, "http.server.request.duration was not recorded")
}

func TestRecover(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	router := newRouter(http.StatusOK)
	handler := middleware.Recover(logger)(router)

	w := serve(handler, "/panic")

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	var problem api.Problem
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &problem))
	assert.Equal(t, "internal_error", problem.Code)
	assert.NotContains(t, w.Body.String(), "something broke")

	assert.Contains(t, buf.String(), "panic recovered")
	assert.Contains(t, buf.String(), "something broke")
	assert.Contains(t, buf.String(), "stack")

	assert.Equal(t, http.StatusOK, serve(handler, tripPath).Code)
}

func TestChainOrder(t *testing.T) {
	t.Parallel()

	var order []string
	mark := func(name string) middleware.Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, name)
				next.ServeHTTP(w, r)
			})
		}
	}
	final := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		order = append(order, "router")
	})

	serve(middleware.Chain(final, mark("recover"), mark("tracing"), mark("logging"), mark("metrics")), "/")

	assert.Equal(t, []string{"recover", "tracing", "logging", "metrics", "router"}, order)
}
