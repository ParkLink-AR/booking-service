package booking

const parkingSpotsBasePath = "/parking-spots"

// internalKeyHeader carries the INTERNAL_SERVICE_KEY shared between services for calls
// that don't go through the gateway (payment-service -> POST /bookings/{id}/confirm).
const internalKeyHeader = "X-Internal-Key"

// userIDHeader is the authenticated caller's user id, set by the gateway on REQUIRED and
// ADMIN routes (stripped from whatever the client itself sent).
const userIDHeader = "X-User-Id"

// adminRoleHeader is set by the gateway only for a ParkLink admin's token.
const adminRoleHeader = "X-Admin-Role"
