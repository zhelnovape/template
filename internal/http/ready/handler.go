package ready

import (
	"context"
	"net/http"
	"time"

	"github.com/zhelnovape/template/internal/generated/api"
	"github.com/zhelnovape/template/internal/http/response"
)

type Handler struct {
	db      pinger
	timeout time.Duration
}

func New(db pinger, timeout time.Duration) *Handler {
	return &Handler{db: db, timeout: timeout}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	if err := h.db.Ping(ctx); err != nil {
		response.JSON(w, http.StatusServiceUnavailable, api.HealthResponse{Status: api.Unavailable})
		return
	}

	response.JSON(w, http.StatusOK, api.HealthResponse{Status: api.Ok})
}
