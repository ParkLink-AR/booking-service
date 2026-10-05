// Package trustedcaller rejects any request that didn't come through the ParkLink
// gateway or from another ParkLink service.
//
// Every handler in this service trusts X-User-Id / X-Admin-Role as already verified --
// true only for requests the gateway forwarded (it strips whatever the client sent and
// sets them from the JWT). If this service is reachable directly (e.g. its own public URL
// on a PaaS), anyone could send those headers themselves; this closes that hole.
package trustedcaller

import (
	"crypto/subtle"
	"net/http"
)

const (
	gatewaySecretHeader = "X-Gateway-Secret"
	internalKeyHeader   = "X-Internal-Key"
)

// Middleware lets a request through when it carries the gateway's shared secret
// (GATEWAY_INTERNAL_SECRET) or the service-to-service key (INTERNAL_SERVICE_KEY). With
// neither configured -- a bare local run outside docker compose -- every request passes,
// same as payment-service's requireGatewaySecret.
func Middleware(gatewaySecret, internalKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			if gatewaySecret == "" && internalKey == "" {
				next.ServeHTTP(responseWriter, request)
				return
			}
			if matches(request.Header.Get(gatewaySecretHeader), gatewaySecret) ||
				matches(request.Header.Get(internalKeyHeader), internalKey) {
				next.ServeHTTP(responseWriter, request)
				return
			}
			responseWriter.Header().Set("Content-Type", "application/json")
			responseWriter.WriteHeader(http.StatusUnauthorized)
			_, _ = responseWriter.Write([]byte(`{"code":"UNTRUSTED_CALLER","error":"requests must go through the gateway"}`))
		})
	}
}

func matches(provided, expected string) bool {
	return expected != "" && subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1
}
