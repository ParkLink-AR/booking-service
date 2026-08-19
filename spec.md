# SPEC: GET /parking-spots/{id}/availability

**Proyecto:** ParkLink — TPP FIUBA — Equipo 198 / Grupo 14  
**Servicio:** booking-service (Go)  
**Fecha:** 2026-08-17  
**Estado:** Finalizado (listo para implementar)

---

## 1. Objetivo

Exponer un endpoint público en `booking-service` que devuelva, por día, las ventanas horarias **efectivamente libres** de una cochera en un rango de fechas. La disponibilidad efectiva resulta de restar las reservas activas a la base publicada por parking-service.

---

## 2. Contexto de Arquitectura

```
App (React Native)
      │
      ▼
booking-service  ──── HTTP ────▶  parking-service
      │                          (base publicada — MOCKEADO en esta iteración)
      │
      └── PostgreSQL   (tabla bookings + booking_status_history)
```

- **parking-service**: propietario de las reglas del spot. Está **mockeado** en esta iteración con respuesta hardcodeada (lunes a viernes 08:00–20:00, fines de semana cerrado).
- **booking-service**: propietario de las reservas. Combina la base publicada con reservas activas para calcular disponibilidad efectiva.
- **Redis**: pospuesto. Sin cache en esta iteración.

---

## 3. Decisiones de Diseño

| Aspecto | Decisión |
|---|---|
| Lenguaje | Go, clean architecture (cinco capas) |
| Timezone | `America/Argentina/Buenos_Aires` — UTC-3 fijo, sin DST |
| Almacenamiento de tiempo | UTC en DB (`TIMESTAMPTZ`, `TSTZRANGE`) |
| Formato de tiempo en respuesta | RFC3339 con offset `-03:00` — ej: `2026-08-18T08:00:00-03:00` |
| Campo `timezone` en response | No existe — el offset está embebido en cada timestamp |
| Montos | `INT` (pesos ARS enteros, sin decimales, sin columna currency) |
| Cache Redis | Pospuesto — no incluido |
| parking-service | Mockeado: fake struct Go, Mon–Fri 08:00–20:00, fines de semana `[]` |
| Autenticación | Endpoint público (sin JWT) |
| DB schema | Desde cero — primera iteración del servicio |

---

## 4. Endpoint Principal

### `GET /parking-spots/{id}/availability?from=YYYY-MM-DD&to=YYYY-MM-DD`

Devuelve las ventanas horarias libres por día en el rango `[from, to)`, en zona horaria Buenos Aires.

#### Query params

| Parámetro | Tipo | Requerido | Descripción |
|---|---|---|---|
| `id` | path | sí | UUID del parking spot |
| `from` | query | sí | Fecha inicio inclusiva `YYYY-MM-DD` |
| `to` | query | sí | Fecha fin exclusiva `YYYY-MM-DD` |

**Límite:** máximo 14 días (`BOOKING_AVAILABILITY_MAX_DAYS`).

#### Response 200

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
    },
    {
      "date": "2026-08-19",
      "status": "full",
      "free_windows": []
    },
    {
      "date": "2026-08-20",
      "status": "closed",
      "free_windows": []
    }
  ]
}
```

**Reglas de `status`:**

| Valor | Condición |
|---|---|
| `closed` | No hay ventanas publicadas ese día (ej: fin de semana) |
| `full` | Hay ventanas publicadas pero todas ocupadas |
| `available` | Al menos una ventana libre |

**Regla de `start`/`end`:** RFC3339 con offset `-03:00` siempre. La hora local Buenos Aires está implícita en el offset.

#### Errors

| HTTP | Código | Condición |
|---|---|---|
| 400 | `INVALID_DATE` | `from` o `to` no son `YYYY-MM-DD` |
| 400 | `INVALID_DATE_RANGE` | `from >= to` |
| 400 | `RANGE_TOO_LARGE` | Rango > 14 días |
| 400 | `INVALID_SPOT_ID` | `id` no es UUID válido |
| 404 | `SPOT_NOT_FOUND` | Spot no encontrado (reservado para cuando parking-service sea real) |
| 502 | `PARKING_SERVICE_ERROR` | parking-service no responde (reservado para cuando sea real) |

---

## 5. Mock de parking-service

En esta iteración, la interfaz `ParkingServiceClient` está implementada por un fake struct Go declarado en `repository/parkingservice/`.

**Comportamiento del mock:**
- Lunes a viernes: devuelve una ventana `08:00–20:00` en hora local Buenos Aires para cada día del rango.
- Sábado y domingo: devuelve ventana vacía (`[]`).
- El mock NO valida que el spot exista (retorna siempre datos).

**Interfaz Go (declarada en usecase, implementada en repository):**

```go
// internal/usecase/availability/parking_service.go
type ParkingServiceClient interface {
    GetAvailabilityBase(ctx context.Context, spotID string, from, to time.Time) (*AvailabilityBase, error)
}

// Structs de respuesta
type AvailabilityBase struct {
    Days []DayBase
}

type DayBase struct {
    Date    time.Time    // medianoche Buenos Aires del día
    Windows []TimeWindow // ventanas en hora local Buenos Aires
}

type TimeWindow struct {
    Start time.Time
    End   time.Time
}
```

**Ejemplo de respuesta del mock para `from=2026-08-18, to=2026-08-21`:**

| Fecha | Windows |
|---|---|
| 2026-08-18 (lun) | `[08:00, 20:00)` |
| 2026-08-19 (mar) | `[08:00, 20:00)` |
| 2026-08-20 (mié) | `[08:00, 20:00)` |
| 2026-08-21 (jue) — excluido (`to` exclusivo) | — |

---

## 6. Modelo de Datos (PostgreSQL)

### 6.1 Enum de estado

```sql
CREATE TYPE booking_status AS ENUM (
  'PENDING_PAYMENT',
  'CONFIRMED',
  'COMPLETED',
  'CANCELLED',
  'EXPIRED'
);
```

### 6.2 Tabla `bookings`

```sql
CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE TABLE bookings (
  id               UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
  parking_spot_id  UUID          NOT NULL,
  driver_user_id   UUID          NOT NULL,
  owner_user_id    UUID          NOT NULL,
  time_range       TSTZRANGE     NOT NULL,         -- [start, end) en UTC
  status           booking_status NOT NULL,
  hold_expires_at  TIMESTAMPTZ,                    -- solo para PENDING_PAYMENT
  amount           INT           NOT NULL,         -- pesos ARS enteros
  payment_id       TEXT,                           -- referencia al payment-service
  created_at       TIMESTAMPTZ   NOT NULL DEFAULT now(),
  updated_at       TIMESTAMPTZ   NOT NULL DEFAULT now(),

  -- Previene solapamiento de reservas activas para el mismo spot
  EXCLUDE USING GIST (
    parking_spot_id WITH =,
    time_range WITH &&
  ) WHERE (status IN ('PENDING_PAYMENT', 'CONFIRMED'))
);

CREATE INDEX idx_bookings_spot_range
  ON bookings USING GIST (parking_spot_id, time_range);
```

### 6.3 Tabla `booking_status_history`

Registra cada transición de estado (inmutable, append-only).

```sql
CREATE TABLE booking_status_history (
  id          UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
  booking_id  UUID          NOT NULL REFERENCES bookings(id),
  from_status booking_status,           -- NULL si es la creación inicial
  to_status   booking_status NOT NULL,
  reason      TEXT,
  created_at  TIMESTAMPTZ   NOT NULL DEFAULT now()
);

CREATE INDEX idx_bsh_booking_id ON booking_status_history (booking_id);
```

### 6.4 Estados que "ocupan" disponibilidad

| Status | ¿Ocupa? | Condición |
|---|---|---|
| `CONFIRMED` | Sí | Siempre |
| `PENDING_PAYMENT` | Sí | Solo si `hold_expires_at > now()` |
| `COMPLETED` | No | Ya pasó |
| `CANCELLED` | No | Cancelada |
| `EXPIRED` | No | Hold venció |

**Query de reservas activas:**
```sql
SELECT time_range
FROM bookings
WHERE parking_spot_id = $1
  AND (
    status = 'CONFIRMED'
    OR (status = 'PENDING_PAYMENT' AND hold_expires_at > now())
  )
  AND time_range && tstzrange($2, $3, '[)')
```

---

## 7. Algoritmo de Disponibilidad

### 7.1 Flujo general

```
Input: spotID (UUID), from (date), to (date)   — ambos en Buenos Aires local

1. Validar input
   - UUID válido
   - Formato YYYY-MM-DD
   - from < to
   - Rango ≤ MAX_DAYS

2. Obtener base publicada
   GET ParkingServiceClient.GetAvailabilityBase(spotID, from, to)
   → map[date][]TimeWindow  (ventanas en hora local Buenos Aires, por día)

3. Cargar reservas activas que solapan el rango completo
   from_utc = toUTC(from, "00:00", BuenosAires)
   to_utc   = toUTC(to,   "00:00", BuenosAires)
   → []Interval  (rangos UTC de reservas activas)

4. Para cada día D en [from, to):
   a. base = ventanas publicadas del día D
      Si vacío → { date: D, status: "closed", free_windows: [] }. Siguiente.

   b. Convertir base a UTC:
      base_utc = convertWindowsToUTC(base, D, BuenosAires)

   c. Filtrar reservas del día:
      day_bookings = reservas que solapan [dayStart_utc, dayEnd_utc)

   d. Restar reservas de base:
      free_utc = subtractIntervals(base_utc, day_bookings)

   e. Determinar status:
      si len(free_utc) > 0  → "available"
      sino                  → "full"

   f. Convertir free_utc a RFC3339 con offset -03:00

5. Devolver { days: [...] }
```

### 7.2 `subtractIntervals` (pure function)

```
subtractIntervals(base []Interval, occupied []Interval) []Interval:
  1. Hacer merge de occupied (unir solapamientos entre reservas)
  2. Para cada ventana en base:
     Restar cada intervalo de occupied → puede dividir en 0, 1 o 2 sub-ventanas
  3. Descartar sub-ventanas con duración ≤ 0
  4. Retornar resultado ordenado
```

### 7.3 Conversión UTC ↔ Buenos Aires

```go
loc, _ := time.LoadLocation("America/Argentina/Buenos_Aires")

// local → UTC (para guardar en DB o comparar con TSTZRANGE)
t := time.Date(day.Year(), day.Month(), day.Day(), 8, 0, 0, 0, loc)
// t.UTC() → usado en queries

// UTC → RFC3339 con -03:00 (para la respuesta)
t.In(loc).Format(time.RFC3339)  // "2026-08-18T08:00:00-03:00"
```

Argentina no tiene DST, pero usar `time.LoadLocation` es correcto por convención y para que el código escale si aparecen spots en otras ciudades.

---

## 8. Estructura de Capas (Go Clean Architecture)

```
cmd/
  main.go                             ← DI wiring

internal/
  controller/
    availability/
      handler.go                      ← HTTP handler, parsing, response mapping
      handler_test.go

  usecase/
    availability/
      usecase.go                      ← orquestación
      usecase_test.go
      parking_service.go              ← interfaz ParkingServiceClient

  service/
    availability/
      service.go                      ← subtractIntervals, lógica pura
      service_test.go

  repository/
    booking/
      repository.go                   ← query active bookings (TSTZRANGE)
      repository_mock_test.go
    parkingservice/
      mock_client.go                  ← fake struct: Mon-Fri 08:00-20:00

  domain/
    entity/
      booking.go                      ← Booking, BookingStatus enum
      availability.go                 ← TimeWindow, DayAvailability, AvailabilityBase

migrations/
  001_create_extensions.sql
  002_create_booking_status_enum.sql
  003_create_bookings.sql
  004_create_booking_status_history.sql
```

---

## 9. Configuración

| Variable | Default | Descripción |
|---|---|---|
| `BOOKING_AVAILABILITY_MAX_DAYS` | `14` | Máximo de días por request |
| `BOOKING_HOLD_MINUTES` | `15` | Duración del hold PENDING_PAYMENT |
| `DATABASE_URL` | — | PostgreSQL connection string |

---

## 10. Criterios de Aceptación

```gherkin
Feature: Disponibilidad efectiva de cochera

  Scenario: Día disponible con reserva parcial
    Given el mock devuelve ventana 08:00-20:00 para el 2026-08-18 (lunes)
    And existe una reserva CONFIRMED que ocupa 10:00-11:00 Buenos Aires
    When GET /parking-spots/{id}/availability?from=2026-08-18&to=2026-08-19
    Then status 200
    And el día 2026-08-18 tiene status "available"
    And free_windows = ["08:00-03:00 a 10:00-03:00", "11:00-03:00 a 20:00-03:00"]

  Scenario: Día completamente ocupado
    Given el mock devuelve ventana 08:00-20:00 para el 2026-08-19 (martes)
    And existe una reserva CONFIRMED que cubre 08:00-20:00 completo
    When GET /parking-spots/{id}/availability?from=2026-08-19&to=2026-08-20
    Then el día 2026-08-19 tiene status "full" y free_windows = []

  Scenario: Fin de semana cerrado
    Given el mock devuelve ventana vacía para el 2026-08-22 (sábado)
    When GET /parking-spots/{id}/availability?from=2026-08-22&to=2026-08-23
    Then el día 2026-08-22 tiene status "closed" y free_windows = []

  Scenario: PENDING_PAYMENT expirado no ocupa disponibilidad
    Given existe una reserva PENDING_PAYMENT con hold_expires_at en el pasado
    When se calcula disponibilidad del día afectado
    Then la reserva expirada no resta disponibilidad

  Scenario: Rango excede máximo
    When GET con rango de 15 días (max=14)
    Then responde 400 RANGE_TOO_LARGE

  Scenario: UUID inválido
    When GET /parking-spots/no-es-uuid/availability
    Then responde 400 INVALID_SPOT_ID
```

---

## 11. Consideraciones de Seguridad

- **Autenticación:** Endpoint público. Sin JWT. La creación de reservas sí requerirá auth.
- **Rate limiting:** A implementar a nivel infraestructura (nginx / API Gateway). No en el handler.
- **Exposición de datos:** La respuesta nunca expone IDs de reservas, datos de conductores ni montos. Solo ventanas horarias.
- **Validación de input:** `spotID` debe ser UUID válido (regex). `from`/`to` deben ser `YYYY-MM-DD` parseables.