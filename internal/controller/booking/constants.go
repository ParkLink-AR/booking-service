package booking

const parkingSpotsBasePath = "/parking-spots"

// internalKeyHeader carries the INTERNAL_SERVICE_KEY shared between services for calls
// that don't go through the gateway (payment-service -> POST /bookings/{id}/confirm).
const internalKeyHeader = "X-Internal-Key"
