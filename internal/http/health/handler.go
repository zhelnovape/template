package health

import (
	"net/http"

	"github.com/zhelnovape/template/internal/generated/api"
	"github.com/zhelnovape/template/internal/http/response"
)

type Handler struct{}

func New() *Handler {
	return &Handler{}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	response.JSON(w, http.StatusOK, api.HealthResponse{Status: api.Ok})
}
