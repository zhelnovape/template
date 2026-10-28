package finish

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	domain "github.com/zhelnovape/template/internal/domain/trip"
	"github.com/zhelnovape/template/internal/http/domainerror"
	"github.com/zhelnovape/template/internal/http/response"
	"github.com/zhelnovape/template/internal/http/trip"
)

type Handler struct {
	usecase tripFinisher
}

func New(usecase tripFinisher) *Handler {
	return &Handler{usecase: usecase}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "tripId"))
	if err != nil {
		domainerror.Write(w, r, &domain.ValidationError{Field: "tripId", Reason: "must be a valid UUID"})
		return
	}

	t, err := h.usecase.Execute(r.Context(), id)
	if err != nil {
		domainerror.Write(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, trip.ToAPI(t))
}
