# booking-service

**ParkLink** — TPP FIUBA — Equipo 198 / Grupo 14

`booking-service` owns parking spot availability and reservations: computing free time windows, listing a spot's bookings, and creating new bookings.

---

## Endpoints

### `GET /parking-spots/{id}/availability?from=YYYY-MM-DD&to=YYYY-MM-DD`

Effective free time windows per day, combining the published base (parking-service, mocked in this iteration) with active bookings. `from`/`to` are required, half-open range.

```json
{
  "days": [
    {
      "date": "2026-08-18",
      "status": "available",
      "free_windows": [
        { "start": "2026-08-18T08:00:00-03:00", "end": "2026-08-18T10:00:00-03:00" },
        { "start": "2026-08-18T11:00:00-03:00", "end": "2026-08-18T20:00:00-03:00" }
      ]
    }
  ]
}
```

### `GET /parking-spots/{id}/bookings?from=YYYY-MM-DD&to=YYYY-MM-DD&status=CONFIRMED`

Lists a spot's bookings grouped by day. `from`, `to`, and `status` are all optional; without `from`/`to` it returns the full history. Only days with at least one booking appear.

```json
{
  "bookings": [
    {
      "date": "2026-08-18",
      "bookings": [
        { "start": "2026-08-18T10:00:00-03:00", "end": "2026-08-18T11:00:00-03:00", "status": "CONFIRMED" }
      ]
    }
  ]
}
```

### `POST /parking-spots/{id}/bookings`

Creates a booking. Validates the range, checks it falls within the spot's published hours, and checks that fewer than the spot's `capacity` active bookings already overlap the requested range (`parking_spot.capacity` in the body, default 1 — a "playa" passes N so up to N reservations can overlap). The check is a fast pre-check plus an authoritative recount inside the repository transaction under a per-spot advisory lock (there is no `EXCLUDE` constraint anymore). Created as `PENDING_PAYMENT` with a hold that expires after `BOOKING_HOLD_MINUTES` (default 5).

Request:
```json
{ "user_id": "8f14e45f-ceea-4b0e-b6b4-5a1b6c9a2d0f", "start": "2026-08-24T10:00:00-03:00", "end": "2026-08-24T11:00:00-03:00", "amount": 5000 }
```

Response `201`:
```json
{
  "id": "f5fef5a2-7316-4613-a811-4228eb5484c1",
  "parking_spot_id": "99999999-9999-9999-9999-999999999999",
  "user_id": "8f14e45f-ceea-4b0e-b6b4-5a1b6c9a2d0f",
  "start": "2026-08-24T10:00:00-03:00",
  "end": "2026-08-24T11:00:00-03:00",
  "status": "PENDING_PAYMENT",
  "hold_expires_at": "2026-08-24T09:45:00-03:00",
  "amount": 5000
}
```

Error codes across the three endpoints: `INVALID_SPOT_ID`, `INVALID_DATE`, `INVALID_DATE_RANGE`, `RANGE_TOO_LARGE`, `INVALID_STATUS`, `INVALID_USER_ID`, `INVALID_TIME_RANGE`, `INVALID_AMOUNT`, `OUTSIDE_PUBLISHED_HOURS` (400), `BOOKING_CONFLICT` (409), `PARKING_SERVICE_ERROR` (502).

### Not yet implemented

Scoped in the original design but out of scope for this iteration: `GET /bookings/{id}`, `POST /bookings/{id}/cancel`, `GET /users/{id}/bookings/history`, `POST /bookings/{id}/reviews`, `GET /parking-spots/{id}/reviews`.

---

## Known technical debt

- **No authentication.** All endpoints are public. `user_id` on creation is client-supplied and not verified against any identity system.
- **No real pricing.** `amount` is client-supplied, not calculated from a rate.
- **No payment integration.** Nothing ever transitions a booking from `PENDING_PAYMENT` to `CONFIRMED`.
- **No hold-expiry job.** An expired `PENDING_PAYMENT` hold correctly stops occupying availability (the read queries already check `hold_expires_at`), but its row never physically flips to `EXPIRED` — there's no reconciler yet.
- **parking-service is mocked.** Fixed Mon-Fri 08:00-20:00, closed weekends (`internal/repository/parkingservice`). It doesn't know about spot ownership, which is why bookings only reference `user_id` (the driver), not an owner.

---

## Architecture

Five-layer Clean Architecture, dependencies flow inward only:

```
controller/   → HTTP handlers: parsing, validation, response mapping
usecase/      → pure orchestration between services, one Execute() each, no business logic
service/      → business logic; declares the repository/service interfaces it consumes
repository/   → data access (Postgres, or the parking-service mock)
domain/       → entities, enums, errors, constants — no external deps
```

Interfaces are declared in the consuming layer (never in a separate file), and a usecase never talks to a repository directly — it always goes through a service, even when that service is a pure pass-through (see `internal/service/parkingservice`).

---

## Running locally

```bash
docker compose up --build
```

Starts Postgres, RabbitMQ, and the app on `:8080`. Migrations run automatically on startup.

Config (`.env.example`):

| Variable | Default | Description |
|---|---|---|
| `HTTP_PORT` | `8080` | |
| `DB_DSN` | — | PostgreSQL connection string |
| `RABBITMQ_URL` | — | Kept wired per project requirements; no feature publishes to it yet |
| `BOOKING_HOLD_MINUTES` | `5` | How long a `PENDING_PAYMENT` hold lasts |

## Testing

```bash
make test              # unit tests
make race               # unit tests with -race
BOOKING_TEST_DSN="postgres://appuser:apppassword@localhost:5432/cleanapi?sslmode=disable" go test ./internal/repository/booking/...
```

The last command runs the repository integration suite against a real PostgreSQL instance (skipped by default) — it's what actually exercises the `TSTZRANGE` overlap logic and the advisory-lock + capacity recount in `CreateBooking`, which a mocked `*sql.DB` can't verify.

See `docs/go-rules.md` for the full coding standards, and `spec.md` / `plan.md` for the original availability-endpoint design.

## Datos de demostración

Por defecto (se apaga con `SEED_DEMO_DATA=false`), al arrancar, le pide a parking-service sus cocheras demo y les carga entre 20 y 50 reseñas a cada una
(`internal/usecase/booking/seed_demo_data_usecase.go`). Las reseñas cuelgan de una reserva terminada y sin pagar por cochera, así que nunca
generan cobros. Detalles y cómo quitarlos en `parking-service/README.md`, sección "Datos de demostración".
