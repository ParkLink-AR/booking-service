package booking

import (
	"crypto/subtle"
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

// isInternalCaller reports whether request carries the INTERNAL_SERVICE_KEY shared with
// the other ParkLink services -- i.e. it's a service-to-service call, never a user's.
// Always false when the key isn't configured (fail closed).
func (handler *Handler) isInternalCaller(request *http.Request) bool {
	providedKey := request.Header.Get(internalKeyHeader)
	return handler.internalServiceKey != "" &&
		subtle.ConstantTimeCompare([]byte(providedKey), []byte(handler.internalServiceKey)) == 1
}

// requireSelfOrAdmin lets a /users/{id}/... request through only when the gateway-
// authenticated caller (X-User-Id) is that same user, or a ParkLink admin (X-Admin-Role,
// which the gateway only ever sets for an admin token). Otherwise it writes a 403 and
// returns false.
func requireSelfOrAdmin(responseWriter http.ResponseWriter, request *http.Request, userID string) bool {
	if request.Header.Get(adminRoleHeader) != "" {
		return true
	}
	callerID := request.Header.Get(userIDHeader)
	if callerID == "" || callerID != userID {
		writeErrorCode(responseWriter, http.StatusForbidden, "FORBIDDEN", "solo podés ver tu propia información")
		return false
	}
	return true
}
