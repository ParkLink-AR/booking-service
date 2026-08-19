# Go Coding Rules — Referencia Portable

Reglas de codificación en Go extraídas de la configuración de esta sesión, adaptadas para proyectos personales fuera de Mercado Libre.

Leyenda: ✅ portable | ⚠️ adaptar | ❌ solo MeLi

---

## 0. Primera sesión — Setup con Claude Code

Pegá este prompt al inicio de cada proyecto nuevo para configurar Claude con estas reglas:

```
Hola! Estoy empezando un proyecto Go personal. Necesito que configures tu comportamiento siguiendo las reglas de docs/go-rules.md (si el archivo no existe todavía, voy a pegarte las reglas a continuación).

Reglas de comportamiento que quiero que sigas en esta sesión:

**Arquitectura:**
- Seguir cinco capas: controller → usecase → service → repository → domain
- Dependencias solo hacia adentro, nunca hacia afuera
- Interfaces declaradas en la capa consumidora
- Un solo método Execute() por UseCase
- errgroup para llamadas paralelas en usecases

**Código Go:**
- Go 1.22+, sin init(), sin variables globales mutables exportadas
- Errores envueltos con %w, tipos custom para errores de dominio
- errors.Is/As para clasificar, nunca type assertion directa
- JSON tags siempre snake_case
- goccy/go-json en lugar de encoding/json
- Enums con iota, nunca type X string
- Variables: mínimo 3 chars (excepto i, j, T en generics)

**Testing:**
- testify/suite para todos los tests
- Table-driven tests con structs declarados fuera de la función
- SetupTest() recrea mocks, TearDownTest() hace AssertExpectations
- go.uber.org/mock para mocks generados (mockgen)
- Fake structs con function-fields para interfaces que envuelven clientes externos sin interfaz propia
- Sin subtests a menos que lo pida explícitamente
- ≥90% cobertura en paquetes críticos
- go test -race en CI

**Seguridad:**
- Nunca hardcodear secrets
- Validar todo input externo con structs tipados
- UUIDs en lugar de IDs secuenciales
- Nunca reflect, unsafe, os/exec para lógica arbitraria

**Commits:**
- No commitear sin pedirlo explícitamente

Cuando termines de leer estas reglas, confirmá con "Reglas cargadas" y preguntame qué queremos construir.
```

### Plugins a instalar (Claude Code)

Si usás Claude Code CLI, instalá estos plugins universales:

```bash
# En ~/.claude/settings.json, agregar a enabledPlugins:
"superpowers@claude-plugins-official": true,
"code-review@claude-plugins-official": true,
"code-simplifier@claude-plugins-official": true,
"skill-creator@claude-plugins-official": true,
"gopls-lsp@claude-plugins-official": true
```

O instalá via CLI:
```bash
claude plugins install superpowers@claude-plugins-official
claude plugins install code-review@claude-plugins-official
claude plugins install code-simplifier@claude-plugins-official
```

### CLAUDE.md mínimo para proyecto Go

Crear `CLAUDE.md` en la raíz del proyecto:

```markdown
# CLAUDE.md

## Reglas

Ver `docs/go-rules.md` para la guía completa.

## Comandos

```bash
go test ./...                    # todos los tests
go test -race ./...              # con race detector
go test -v -run TestName ./...   # test específico
go generate ./...                # regenerar mocks
```

## Arquitectura

Cinco capas, dependencias solo hacia adentro:
controller → usecase → service → repository → domain

## Restricciones

- No commitear sin confirmación explícita
- encoding/json prohibido, usar goccy/go-json
- JSON tags en snake_case
- Enums con iota, nunca type X string
```

---

## 1. Comportamiento del agente ✅

### Flujo de trabajo para cada tarea nueva

1. Limpiar contexto no relacionado
2. Enunciar la tarea y sus requisitos
3. Listar pasos accionables con checkboxes
4. Elaborar el enfoque **antes** de modificar código

Formato de plan obligatorio:

```md
## [Título de la tarea]

**Descripción:** ...
**Plan:**
- [ ] Paso 1
- [ ] Paso 2

**Estado actual:** En progreso
**Lessons Learned:** (se completa al terminar)
```

### Formato de respuesta

1. **Problem Analysis** — requisitos, desafíos técnicos, consideraciones
2. **Proposed Solution** — enfoque elegido con snippets concretos
3. **Additional Considerations** — scalabilidad, seguridad, performance
4. **Summary & Recommendations** — próximos pasos

### Reglas de respuesta

- Código en triple backticks con el identificador de lenguaje
- Líneas ≤ 120 caracteres
- Comentarios e identificadores en inglés
- Pedir aclaración antes de codear si los requisitos son ambiguos

---

## 2. Arquitectura: Package-Oriented Design (POD) ✅

```
cmd/                    → DI wiring solo; *Factory() ensambla el grafo
internal/
  controller/           → HTTP handlers
  usecase/              → Orquestación: llamadas paralelas, coordinación
  service/              → Lógica de negocio pura
  repository/           → Acceso a datos: DB, cache, APIs externas
  domain/
    entity/             → Structs core (sin deps externas)
    dto/                → Request/response transfer objects
    enum/               → Enums con iota
    errors/             → Tipos de error de dominio
    constants/          → Constantes de dominio
pkg/                    → Utilidades compartidas
tests/                  → Tests de integración e2e
```

### Reglas de importación

| Capa | Puede importar | No puede importar |
|------|---------------|-------------------|
| `cmd/` | `internal/*`, `pkg/*` | Otras binaries |
| `internal/<dominio>` | Sub-paquetes propios, `pkg/*` | Hermanos del mismo nivel |
| `pkg/*` | Otros `pkg/*` | `internal/*`, `cmd/*` |

### Reglas de capas

- **Controller**: solo parsing HTTP, validación de schema, mapeo entity→DTO de respuesta
- **UseCase**: solo orquestación de servicios; un único método `Execute()`; nombre: `<Verb><Domain>UseCase`
- **Service**: lógica de negocio pura, validaciones de dominio, métricas de negocio
- **Repository**: acceso a datos, mapeo DTO→entity; sin lógica condicional ni validaciones
- **Domain**: structs, enums, errores; sin deps externas

### Anti-patterns prohibidos

1. Controller llamando directamente a service o repository
2. Service llamando a otro service; repository llamando a otro repository
3. Lógica de negocio en controller o usecase
4. Métricas en controller, usecase o repository (solo en service)
5. Enums como `type X string` (siempre usar `iota`)
6. UseCase con más de un método público
7. Nombre de UseCase sin el patrón `<Verb><Domain>UseCase`

---

## 3. Estándares de código Go ✅

### Generales

- Target **Go 1.22+**; usar workspaces (`go work`) en monorepos
- Sin `init()` — preferir inicialización explícita
- Sin variables globales mutables exportadas
- Naming: `MixedCaps` exportado, `mixedCaps` no exportado; sin underscores
- Nombres de variables ≥3 caracteres (excepciones: `i`, `j`, `T` en generics)
- `encoding/json` prohibido — usar `github.com/goccy/go-json`
- JSON tags siempre `snake_case`

### Manejo de errores

```go
// Envolver con contexto
if err := repo.Find(ctx, id); err != nil {
    return fmt.Errorf("finding order %s: %w", id, err)
}

// Tipos custom para errores de dominio
type OrderNotFoundError struct {
    ID string
}

func (e *OrderNotFoundError) Error() string {
    return fmt.Sprintf("order %s not found", e.ID)
}

// Clasificar con errors.Is / errors.As
var notFound *OrderNotFoundError
if errors.As(err, &notFound) {
    return http.StatusNotFound
}
```

- Loguear errores una sola vez, en el edge donde se pierde el contexto
- Nunca ignorar errores con `_` sin un comentario que explique por qué

### Concurrencia

```go
// errgroup para llamadas independientes en paralelo
g, ctx := errgroup.WithContext(ctx)

var product *entity.Product
var inventory *entity.Inventory

g.Go(func() error {
    var err error
    product, err = repo.GetProduct(ctx, id)
    return err
})

g.Go(func() error {
    var err error
    inventory, err = repo.GetInventory(ctx, id)
    return err
})

if err := g.Wait(); err != nil {
    return nil, err
}
```

- Pasar `context.Context` para cancelación — nunca goroutines sin escape
- Cerrar canales cuando se terminan; documentar ownership
- `go test -race` en CI

### Memoria y performance

```go
// Pre-alocar slices cuando el tamaño es predecible
result := make([]string, 0, len(items))

// strings.Builder para concatenación pesada
var b strings.Builder
b.Grow(len(parts) * 10)
for _, p := range parts {
    b.WriteString(p)
}

// sync.Pool para allocations de alta frecuencia
var bufPool = sync.Pool{
    New: func() any { return make([]byte, 0, 1024) },
}
```

### Dependency management

```go
// go.mod y go.sum; run go mod tidy antes de commitear
// Usar govulncheck para auditoría de vulnerabilidades
// Minimizar dependencias externas
```

---

## 4. Testing ✅

### Suite structure

```go
package order_test

import (
    "context"
    "testing"

    "github.com/stretchr/testify/suite"
    "go.uber.org/mock/gomock"
)

type orderServiceSuite struct {
    suite.Suite
    ctrl     *gomock.Controller
    repoMock *MockOrderRepository
    svc      *OrderService
    ctx      context.Context
}

func TestOrderServiceSuite(t *testing.T) {
    suite.Run(t, new(orderServiceSuite))
}

func (s *orderServiceSuite) SetupTest() {
    s.ctrl = gomock.NewController(s.T())
    s.repoMock = NewMockOrderRepository(s.ctrl)
    s.ctx = context.Background()
    s.svc = NewOrderService(s.repoMock)
}

func (s *orderServiceSuite) TearDownTest() {
    s.ctrl.Finish()
}
```

### Table-driven tests

```go
// Struct FUERA de la función de test
var createOrderCases = []struct {
    name    string
    input   CreateOrderParams
    wantErr error
}{
    {name: "happy path", input: CreateOrderParams{UserID: "u1", Amount: 100}, wantErr: nil},
    {name: "invalid amount", input: CreateOrderParams{UserID: "u1", Amount: -1}, wantErr: ErrInvalidAmount},
}

func (s *orderServiceSuite) TestCreateOrder() {
    for _, tc := range createOrderCases {
        s.repoMock.EXPECT().Save(s.ctx, gomock.Any()).Return(tc.wantErr)
        err := s.svc.Create(s.ctx, tc.input)
        s.Equal(tc.wantErr, err, tc.name)
    }
}
```

### Mocks con mockgen

```go
// Directiva en el archivo que define la interfaz:
//go:generate mockgen -destination=./order_repository_mock_test.go -package=order_test -source=./order_repository.go

// Regenerar todos:
// go generate ./...
```

### Fake structs (para clientes externos sin interfaz propia)

```go
// En lugar de gomock cuando el cliente externo no tiene interfaz
type fakeEmailClient struct {
    sendFn func(to, subject, body string) error
}

func (f *fakeEmailClient) Send(to, subject, body string) error {
    return f.sendFn(to, subject, body)
}

// En el test:
client := &fakeEmailClient{
    sendFn: func(to, subject, body string) error {
        s.Equal("user@example.com", to)
        return nil
    },
}
```

### Reglas

- Sin subtests a menos que se solicite explícitamente
- Sin `mock.Anything` — usar inputs esperados explícitos
- Sin lógica de negocio (`if`) en tests — crear filas adicionales en la tabla
- Sin `init()` ni variables globales mutables
- ≥90% cobertura en paquetes críticos
- `t.Parallel()` dentro de cada método cuando no hay estado compartido mutable
- Tests de repositorio: usar SQLite `:memory:` (no mockear `*sql.DB`)

---

## 5. Seguridad ✅

### Reglas core

```go
// ✅ Validar todo input externo con structs tipados
type CreateOrderRequest struct {
    UserID string `json:"user_id"`
    Amount int    `json:"amount"`
}

func (r *CreateOrderRequest) Validate() error {
    if r.UserID == "" {
        return ErrMissingUserID
    }
    if r.Amount <= 0 {
        return ErrInvalidAmount
    }
    return nil
}

// ✅ UUIDs en lugar de IDs secuenciales
id := uuid.New().String()

// ✅ Comparación en tiempo constante para tokens
import "crypto/subtle"
if subtle.ConstantTimeCompare([]byte(token), []byte(expected)) != 1 {
    return ErrUnauthorized
}

// ✅ Context timeouts en llamadas externas
ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
defer cancel()
```

### Prohibiciones absolutas

- Nunca hardcodear secrets, API keys, passwords en código o configs
- Nunca usar `reflect`, `unsafe`, `os/exec` para lógica arbitraria
- Nunca exponer información sensible en logs, errores o responses
- Nunca usar `math/rand` para tokens/IDs — siempre `crypto/rand`
- Nunca compartir estado global mutable desde handlers/goroutines
- Nunca exportar variables globales mutables
- Nunca usar GET para operaciones que modifican estado
- Nunca concatenar inputs de usuario en queries SQL — usar parámetros
- Nunca pasar input de usuario directamente a `http.Get`/`http.Post`

### Checklist antes de agregar una dependencia

1. ¿Tiene vulnerabilidades conocidas? (`govulncheck ./...`)
2. ¿Está mantenida activamente?
3. ¿Hay alternativa en stdlib que sirva?
4. Correr `go mod verify` después de agregar

---

## 6. Logging ✅

### Structured JSON logging

```go
// Campos requeridos en cada log entry
type LogEntry struct {
    Timestamp     time.Time              `json:"timestamp"`
    Level         string                 `json:"level"`
    Message       string                 `json:"message"`
    Service       string                 `json:"service"`
    CorrelationID string                 `json:"correlation_id"`
    TraceID       string                 `json:"trace_id,omitempty"`
    Context       map[string]interface{} `json:"context,omitempty"`
}

// Siempre stdout/stderr — nunca escribir a archivos
fmt.Fprintln(os.Stdout, string(jsonData))
```

### Niveles

- **ERROR**: fallos del sistema que requieren atención
- **WARN**: situaciones potencialmente problemáticas, reintentos
- **INFO**: eventos de negocio, cambios de estado
- **DEBUG**: solo desarrollo

### Reglas

- Propagar `context.Context` en todas las llamadas de logging
- Incluir correlation ID (`X-Request-ID`) en cada log
- Nunca loguear passwords, tokens, PII, credit cards
- Loguear errores una sola vez, en el borde donde se pierde el contexto

---

## 7. Observabilidad ✅

### Métricas async (patrón portable)

```go
// pkg/metrics/metrics.go
// Las métricas se emiten de forma asíncrona — nunca bloquear el hot path

type Worker struct {
    ch chan func()
}

func NewWorker(bufferSize int) *Worker {
    w := &Worker{ch: make(chan func(), bufferSize)}
    go w.run()
    return w
}

func (w *Worker) run() {
    for fn := range w.ch {
        fn()
    }
}

func (w *Worker) Emit(fn func()) {
    select {
    case w.ch <- fn:
    default: // drop if buffer full — never block
    }
}
```

### OpenTelemetry (tracing portable) ✅

```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/codes"
)

var tracer = otel.Tracer("order-service")

func (s *OrderService) Process(ctx context.Context, id string) error {
    ctx, span := tracer.Start(ctx, "order.process")
    defer span.End()

    span.SetAttributes(attribute.String("order.id", id))

    if err := s.repo.Save(ctx, id); err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
        return err
    }

    span.SetStatus(codes.Ok, "")
    return nil
}
```

### Health checks ✅

```go
func (h *HealthHandler) Check(w http.ResponseWriter, r *http.Request) {
    ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
    defer cancel()

    status := "healthy"
    if err := h.db.PingContext(ctx); err != nil {
        status = "unhealthy"
        w.WriteHeader(http.StatusServiceUnavailable)
    }

    json.NewEncoder(w).Encode(map[string]string{"status": status})
}
```

### Nombres de métricas

```
<dominio>.<entidad>.<verbo>   →  orders.total.created
<servicio>.<recurso>.<estado> →  http.request.duration
```

- Evitar alta cardinalidad: nunca usar user IDs, request IDs, UUIDs como tags
- Usar valores categóricos y acotados: `status:success`, `method:GET`

---

## 8. Performance ✅

### Profiling con pprof

```go
// Solo en dev/staging:
if os.Getenv("ENABLE_PPROF") == "true" {
    go http.ListenAndServe("localhost:6060", nil)
}

// Comandos:
// CPU:    go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30
// Memory: go tool pprof http://localhost:6060/debug/pprof/heap
// Goroutines: go tool pprof http://localhost:6060/debug/pprof/goroutine
```

### Worker pool pattern

```go
func processJobs(ctx context.Context, jobs <-chan Job, workers int) {
    var wg sync.WaitGroup
    for i := 0; i < workers; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for {
                select {
                case job, ok := <-jobs:
                    if !ok {
                        return
                    }
                    processJob(job)
                case <-ctx.Done():
                    return
                }
            }
        }()
    }
    wg.Wait()
}
```

### Anti-patterns de performance

- No optimizar antes de medir (nunca premature optimization)
- No usar `strings.Join` en loops para construir strings grandes — usar `strings.Builder`
- No hacer doble lookup en maps: `val := m[k]` luego `m[k] = x` — verificar en un paso
- No ignorar el overhead del sidecar/proxy al medir latencia end-to-end

---

## 9. Documentación ✅

### Inline comments

```go
// Solo cuando el WHY no es obvio: constraint oculta, invariante sutil, workaround
// No explicar QUÉ hace el código (los nombres ya lo dicen)
// No referenciar la tarea, PR o issue actual (pertenece al commit message)

// max 5 retries matches the upstream SLA window before client timeout fires
for attempt := range 5 {
    ...
}
```

### Package README

Cada paquete debe tener `README.md` con:
1. Purpose & Overview
2. Usage Examples (snippets ejecutables)
3. Configuration Options
4. Public API Reference
5. Dependencies

### API documentation

- Mantener spec OpenAPI/Swagger para cada API externa
- Versionar specs bajo `docs/specs/`
- Validar spec en CI

### Architecture documentation

- Diagramas C4 (Context, Container, Component) con Mermaid bajo `docs/architecture/`
- ADRs (Architecture Decision Records) con template MADR bajo `docs/adr/`

---

## 10. CI checklist ✅

```bash
go vet ./...
staticcheck ./...
go test ./... -race -cover
go mod tidy && git diff --exit-code go.sum
govulncheck ./...
```

### Makefile mínimo recomendado

```makefile
.PHONY: test lint fmt check

test:
    go test -race -short ./...

test-cover:
    go test -coverprofile=coverage.out ./...
    go tool cover -html=coverage.out

lint:
    golangci-lint run ./...

fmt:
    go fmt ./...
    gofumpt -w .

generate:
    go generate ./...

check: generate fmt lint test
```

---

## Resumen: qué es portable y qué no

| Área | Portable ✅ | No portable ❌ |
|------|------------|---------------|
| Arquitectura POD | Todo | — |
| Go coding standards | Todo | goccy/go-json (opcional, no obligatorio) |
| Testing con testify/suite | Todo | — |
| Mocks con gomock | Todo | — |
| Seguridad (OWASP, validación) | Todo | Fury Secrets SDK |
| Logging JSON structured | Todo | Fury Logs Service, cuotas diarias |
| OpenTelemetry tracing | Todo | — |
| Health checks | Todo | — |
| pprof + performance patterns | Todo | — |
| Documentación (README, ADR) | Todo | — |
| Métricas async pattern | Todo | Datadog/DogStatsD específico |
| MCP servers | Google Calendar, Drive, Slack, Jira, Notion, Miro, Figma | fury_for_development, genai_code_review, meli_appsec_codeguard |
| Plugins/Skills | superpowers, code-review, simplify, skill-creator, gopls-lsp | fury-deployments, fury-services, meli-o11y, meli-security-expert, noa-*, fintech-*, grid-sharing |
