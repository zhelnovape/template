package middleware

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func Tracing(tp trace.TracerProvider, propagator propagation.TextMapPropagator, routes chi.Routes) Middleware {
	return func(next http.Handler) http.Handler {
		inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			span := trace.SpanFromContext(r.Context())
			if route := routePattern(routes, r); route != "" {
				span.SetAttributes(attribute.String("http.route", route))
			}

			rec := newStatusRecorder(w)
			next.ServeHTTP(rec, r)

			if rec.status >= http.StatusInternalServerError {
				span.SetStatus(codes.Error, http.StatusText(rec.status))
				span.SetAttributes(attribute.String("error.type", strconv.Itoa(rec.status)))
			}
		})

		return otelhttp.NewHandler(inner, "http.server",
			otelhttp.WithTracerProvider(tp),
			otelhttp.WithPropagators(propagator),
			otelhttp.WithMeterProvider(noop.NewMeterProvider()),
			otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
				if route := routePattern(routes, r); route != "" {
					return r.Method + " " + route
				}
				return r.Method
			}),
		)
	}
}
