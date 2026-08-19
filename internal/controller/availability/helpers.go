package availability

import (
	"net/http"

	"github.com/goccy/go-json"
)

type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"error"`
}

func writeJSON(responseWriter http.ResponseWriter, status int, payload any) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(status)
	_ = json.NewEncoder(responseWriter).Encode(payload)
}

func writeErrorCode(responseWriter http.ResponseWriter, status int, code, message string) {
	writeJSON(responseWriter, status, errorResponse{Code: code, Message: message})
}
