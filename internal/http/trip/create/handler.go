package create

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"slices"

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
	req, err := decode(r.Body)
	if err != nil {
		domainerror.Write(w, r, h.logger, err)
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

func decode(body io.Reader) (api.TripData, error) {
	var req api.TripData
	invalidBody := &domain.ValidationError{Field: "body", Reason: "must be a valid JSON object"}

	raw, err := io.ReadAll(body)
	if err != nil {
		return req, invalidBody
	}

	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		return req, invalidBody
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return req, invalidBody
	}

	if err := checkRequired(raw); err != nil {
		return req, err
	}
	return req, nil
}

func checkRequired(raw []byte) error {
	var data map[string]json.RawMessage
	if err := json.Unmarshal(raw, &data); err != nil {
		return &domain.ValidationError{Field: "body", Reason: "must be a valid JSON object"}
	}
	if err := requireFields(data, "", "user_id", "driver_id", "start_point", "end_point", "price"); err != nil {
		return err
	}

	for _, name := range []string{"start_point", "end_point"} {
		var point map[string]json.RawMessage
		if err := json.Unmarshal(data[name], &point); err != nil {
			return &domain.ValidationError{Field: name, Reason: "must be an object"}
		}
		if err := requireFields(point, name+".", "latitude", "longitude"); err != nil {
			return err
		}
	}
	return nil
}

func requireFields(obj map[string]json.RawMessage, prefix string, fields ...string) error {
	for _, f := range fields {
		if v, ok := obj[f]; !ok || string(v) == "null" {
			return &domain.ValidationError{Field: prefix + f, Reason: "is required"}
		}
	}
	if len(obj) == len(fields) {
		return nil
	}
	for key := range obj {
		if !slices.Contains(fields, key) {
			return &domain.ValidationError{Field: prefix + key, Reason: "is not allowed"}
		}
	}
	return nil
}
