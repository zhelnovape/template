package create

import (
	"encoding/json"
	"net/http"

	"github.com/zhelnovape/template/internal/generated/api"
	"github.com/zhelnovape/template/internal/http/domainerror"
	"github.com/zhelnovape/template/internal/http/response"
	"github.com/zhelnovape/template/internal/http/trip"
)

type Handler struct {
	usecase tripCreator
}

func New(usecase tripCreator) *Handler {
	return &Handler{usecase: usecase}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req api.TripData

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil || !validTripData(req) {
		domainerror.WriteInvalidRequest(w, r)
		return
	}

	t := toModel(req)
	if err := h.usecase.Execute(r.Context(), t); err != nil {
		domainerror.Write(w, r, err)
		return
	}

	w.Header().Set("Location", "/api/v1/trips/"+t.ID.String())
	response.JSON(w, http.StatusCreated, trip.ToAPI(t))
}
