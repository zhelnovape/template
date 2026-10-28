package create

import (
	"encoding/json"
	"log/slog"
	"net/http"

	domain "github.com/zhelnovape/template/internal/domain/trip"
	"github.com/zhelnovape/template/internal/generated/api"
	"github.com/zhelnovape/template/internal/http/domainerror"
	"github.com/zhelnovape/template/internal/http/response"
	"github.com/zhelnovape/template/internal/http/trip"
)

type Handler struct {
	usecase tripCreator
	logger  *slog.Logger
}

func New(usecase tripCreator, logger *slog.Logger) *Handler {
	return &Handler{usecase: usecase, logger: logger}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req api.TripData

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		domainerror.Write(w, r, h.logger, &domain.ValidationError{Field: "body", Reason: "must be a valid JSON object"})
		return
	}

	t, err := h.usecase.Execute(r.Context(), toModel(req))
	if err != nil {
		domainerror.Write(w, r, h.logger, err)
		return
	}

	w.Header().Set("Location", "/api/v1/trips/"+t.ID.String())
	response.JSON(w, http.StatusCreated, trip.ToAPI(t))
}
