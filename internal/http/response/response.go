package response

import (
	"encoding/json"
	"net/http"

	"github.com/zhelnovape/template/internal/generated/api"
)

func JSON(w http.ResponseWriter, status int, v any) {
	write(w, status, "application/json", v)
}

func Problem(w http.ResponseWriter, status int, p api.Problem) {
	write(w, status, "application/problem+json", p)
}

func write(w http.ResponseWriter, status int, contentType string, v any) {
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
