package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

func Logging(logger *slog.Logger, routes chi.Routes) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := newStatusRecorder(w)
			panicked := true

			defer func() {
				status := rec.finalStatus(panicked)

				level := slog.LevelInfo
				if status >= http.StatusInternalServerError {
					level = slog.LevelError
				}

				logger.Log(r.Context(), level, "request handled",
					"method", r.Method,
					"route", routePattern(routes, r),
					"status", status,
					"duration_ms", time.Since(start).Milliseconds(),
				)
			}()

			next.ServeHTTP(rec, r)
			panicked = false
		})
	}
}
