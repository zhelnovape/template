package middleware

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

type Middleware func(http.Handler) http.Handler

func Chain(handler http.Handler, middlewares ...Middleware) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	return handler
}

func routePattern(routes chi.Routes, r *http.Request) string {
	pattern := routes.Find(chi.NewRouteContext(), r.Method, r.URL.Path)
	if len(pattern) > 1 {
		pattern = strings.TrimSuffix(pattern, "/")
	}
	return pattern
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func newStatusRecorder(w http.ResponseWriter) *statusRecorder {
	return &statusRecorder{ResponseWriter: w, status: http.StatusOK}
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}
