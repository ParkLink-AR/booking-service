# PLAN: Implementar booking-service — endpoint GET /availability

**Proyecto:** ParkLink — TPP FIUBA — Equipo 198 / Grupo 14  
**Servicio:** booking-service (Go, clean architecture)  
**SPEC de referencia:** `SPEC-availability-endpoint.md`  
**Directorio destino:** `/Users/fluzzi/Desktop/facultad/booking-service/`  
**Fecha:** 2026-08-17

---

## Descripción

Crear desde cero el `booking-service` en Go usando la misma plantilla de clean architecture del template. El único endpoint de esta iteración es `GET /parking-spots/{id}/availability?from&to`. La DB es PostgreSQL (sin datos previos). El cliente a parking-service es un fake struct hardcodeado (Mon–Fri 08:00–20:00).

No hay Redis, no hay autenticación, no hay otros endpoints. El schema debe escalar para todos los endpoints futuros del servicio.

---

## Plan

### Fase 1 — Scaffold del proyecto

- [ ] Crear directorio `/Users/fluzzi/Desktop/facultad/booking-service/`
- [ ] `go mod init github.com/parklink/booking-service`
- [ ] Crear `docker-compose.yml` (app + postgres:16-alpine)
- [ ] Crear `Dockerfile` multi-stage (`CGO_ENABLED=0`, distroless)
- [ ] Crear `Makefile` con targets: `run`, `test`, `build`, `migrate`
- [ ] Agregar dependencias: `lib/pq`, `chi`, `go-json`, `google/uuid`

### Fase 2 — Migrations

- [ ] `migrations/001_create_extensions.sql` — `CREATE EXTENSION IF NOT EXISTS btree_gist`
- [ ] `migrations/002_create_booking_status_enum.sql` — tipo ENUM `booking_status`
- [ ] `migrations/003_create_bookings.sql` — tabla `bookings` con TSTZRANGE + EXCLUDE USING GIST
- [ ] `migrations/004_create_booking_status_history.sql` — tabla `booking_status_history`
- [ ] `cmd/migrate.go` — runner de migrations desde `migrations/` usando `lib/pq`

### Fase 3 — Domain entities

- [ ] `internal/domain/entity/booking.go` — struct `Booking`, type `BookingStatus`
- [ ] `internal/domain/entity/availability.go` — structs `TimeWindow`, `DayAvailability`, `AvailabilityBase`
- [ ] `internal/domain/errors/errors.go` — errores de dominio (`ErrInvalidInput`, `ErrRangeTooLarge`, `ErrSpotNotFound`)

### Fase 4 — Repository: booking

- [ ] `internal/repository/booking/repository.go` — interfaz + implementación PostgreSQL
  - Método: `GetActiveBookingsInRange(ctx, spotID, from, to time.Time) ([]entity.TimeWindow, error)`
  - Query con `TSTZRANGE && tstzrange($2, $3, '[)')` y filtro de status activos
- [ ] `internal/repository/booking/repository_mock_test.go` — mock para tests de usecase

### Fase 5 — Repository: parking-service mock

- [ ] `internal/repository/parkingservice/mock_client.go` — fake struct implementando `ParkingServiceClient`
  - Lunes a viernes: devuelve `[{08:00, 20:00}]` en hora Buenos Aires
  - Sábado y domingo: devuelve `[]`
  - Usa `time.LoadLocation("America/Argentina/Buenos_Aires")`

### Fase 6 — Service: availability (lógica pura)

- [ ] `internal/service/availability/service.go`
  - `SubtractIntervals(base, occupied []TimeWindow) []TimeWindow`
  - `ConvertWindowsToUTC(windows []TimeWindow, date time.Time, loc *time.Location) []TimeWindow`
  - `DetermineStatus(baseEmpty bool, freeWindows []TimeWindow) string`
- [ ] `internal/service/availability/service_test.go` — tabla de casos:
  - Base vacía → `closed`
  - Reserva parcial → ventanas restantes correctas
  - Reserva total → `full`
  - Reservas solapadas entre sí → merge antes de restar
  - Reserva expirada (no incluida) → no afecta

### Fase 7 — Usecase: availability

- [ ] `internal/usecase/availability/parking_service.go` — interfaz `ParkingServiceClient`
- [ ] `internal/usecase/availability/booking_repository.go` — interfaz `BookingRepository`
- [ ] `internal/usecase/availability/usecase.go` — orquestación completa del flujo
  - Validación de input (UUID, fechas, rango)
  - Llamada al mock de parking-service
  - Llamada al repo de bookings
  - Llamada al service para calcular free_windows por día
  - Retorna `[]entity.DayAvailability`

### Fase 8 — Controller: availability

- [ ] `internal/controller/availability/handler.go`
  - Parsear `id` (UUID), `from`, `to` (YYYY-MM-DD)
  - Llamar usecase
  - Mapear `[]DayAvailability` a DTOs de respuesta
  - Serializar `start`/`end` como RFC3339 con offset `-03:00`: `t.In(loc).Format(time.RFC3339)`
  - Mapear errores de dominio a HTTP status codes
- [ ] DTOs de respuesta en el controller (no en domain):
  ```
  availabilityResponse → days []dayAvailabilityResponse
  dayAvailabilityResponse → date, status, free_windows []timeWindowResponse
  timeWindowResponse → start, end (string RFC3339)
  ```

### Fase 9 — DI wiring

- [ ] `cmd/main.go`
  - Setup DB (postgres)
  - Instanciar mock de parking-service
  - Instanciar booking repo → usecase → controller
  - Registrar rutas con chi: `GET /parking-spots/{id}/availability`
  - Graceful shutdown (SIGINT/SIGTERM)
- [ ] `cmd/setup_db.go` — conexión PostgreSQL + ping + run migrations

### Fase 10 — Verificación manual

- [ ] Levantar con `docker-compose up`
- [ ] Verificar migrations aplicadas
- [ ] `curl` con día disponible, día full, fin de semana — verificar respuesta JSON
- [ ] `curl` con UUID inválido, rango > 14 días, from >= to — verificar 400
- [ ] Verificar que `start`/`end` tienen offset `-03:00` en el JSON

---

## Decisiones de implementación

| Punto | Decisión |
|---|---|
| Router HTTP | `github.com/go-chi/chi/v5` |
| JSON | `github.com/goccy/go-json` |
| PostgreSQL driver | `github.com/lib/pq` |
| UUID | `github.com/google/uuid` |
| Timezone Buenos Aires | `time.LoadLocation("America/Argentina/Buenos_Aires")` — cargado una vez al inicio |
| Formato tiempo en response | `t.In(loc).Format(time.RFC3339)` → `"2026-08-18T08:00:00-03:00"` |
| GOPROXY | `GOPROXY=direct` para evitar proxy Fury si aplica |
| Migrations | Desde archivos `.sql` en `migrations/`, corridas al startup |

---

## Estado Actual

`[x] Implementado — endpoint funcional, verificado end-to-end contra Postgres real`

**Desviaciones respecto al plan original:**
- El repo ya tenía un template genérico (`go-clean-api`, dominio orders/products/rabbitmq). Por decisión del usuario se mantuvo intacto y el feature de booking-service se agregó al lado, en el mismo módulo Go (`github.com/example/go-clean-api`), en vez de crear un módulo/directorio nuevo desde cero.
- Migrations: además de los archivos `.sql` en `migrations/`, se extendió `runMigrations()` en `cmd/setup_db.go` con el mismo SQL, porque el patrón existente del template corre las migrations inline al startup (no lee los archivos `.sql`).
- Se usó `errgroup` (`golang.org/x/sync/errgroup`) para paralelizar `GetAvailabilityBase` y `GetActiveBookingsInRange` en el usecase, siguiendo la regla de coding rules del proyecto.
- Se agregó `internal/repository/booking/repository_integration_test.go`, gateado por `BOOKING_TEST_DSN` (se skipea si no está seteada), para validar contra Postgres real: el filtro de `PENDING_PAYMENT` expirado y el `EXCLUDE USING GIST` — comportamiento que un mock de `*sql.DB` no puede verificar.

## Lessons Learned

- `mockgen` global instalado en el sistema era `github.com/golang/mock` (deprecado), no `go.uber.org/mock` (el que usa el proyecto). Los mocks se generan correctamente con `go tool mockgen` (requiere `go get -tool go.uber.org/mock/mockgen@v0.4.0` una vez, ya aplicado a `go.mod`).
- Verificación end-to-end: se corrió el binario real de la app con Postgres real (`docker run postgres:16-alpine` en un puerto libre) contra los 6 escenarios gherkin del spec (día con reserva parcial, full, fin de semana, UUID inválido, rango > 14 días, `from >= to`) — todos devolvieron la respuesta exacta esperada, incluyendo el offset `-03:00` en RFC3339.
- RabbitMQ (dependencia no relacionada del template existente, requerida por `cmd/main.go` para bootear) no pudo levantarse en este sandbox (error de permisos del cookie de Erlang). Para la verificación e2e del endpoint de availability se usó un `main.go` temporal standalone (fuera del repo tras la prueba) que wireaba solo lo necesario para este feature, evitando esa dependencia no relacionada.