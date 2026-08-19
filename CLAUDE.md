# CLAUDE.md

## Project Overview

Personal Go booking service following portable Go coding standards (Go 1.22+).

## Rules & Constraints

See `docs/go-rules.md` for the complete reference.

### Core Rules

**Architecture:** Five-layer package-oriented design:
```
cmd/              → DI wiring only (*Factory() assembles graph)
internal/
  controller/     → HTTP handlers, request/response mapping
  usecase/        → Service orchestration (one Execute() method per UseCase)
  service/        → Pure business logic
  repository/     → Data access (DB, cache, external APIs)
  domain/
    entity/       → Core structs (no external deps)
    dto/          → Transfer objects
    enum/         → Enums using iota
    errors/       → Custom error types
    constants/    → Domain constants
pkg/              → Shared utilities
tests/            → Integration & e2e tests
```

**Dependency Rules:**
- Dependencies flow only inward (controller → usecase → service → repository → domain)
- Interfaces declared in the consuming layer
- No sibling-to-sibling imports within same layer
- Packages import sub-packages only

**Go Standards:**
- Go 1.22+ required
- No init(), no exported mutable globals
- MixedCaps exported, mixedCaps internal; no underscores
- Variable naming: ≥3 chars minimum (except i, j, T in generics)
- Errors wrapped with %w; use custom types for domain errors
- Use errors.Is/errors.As; never type assertion on errors
- JSON tags always snake_case
- Use goccy/go-json (not encoding/json)
- Enums: iota only (never type X string)

**Testing:**
- testify/suite for all test suites
- Table-driven tests with structs declared outside function
- go.uber.org/mock for code-generated mocks (mockgen)
- SetupTest() recreates mocks; TearDownTest() asserts expectations
- ≥90% coverage on critical packages
- go test -race in CI
- No subtests unless explicitly requested

**Security:**
- Validate all external input with typed structs
- UUIDs instead of sequential IDs
- Constant-time comparison for tokens (crypto/subtle)
- No hardcoded secrets
- Never use reflect/unsafe/os/exec for arbitrary logic
- Never expose sensitive info in logs/errors/responses
- crypto/rand for tokens, never math/rand

**Commits:**
- Do NOT commit code without explicit user request

## Commands

```bash
# Testing
go test ./...                    # Run all tests
go test -race ./...              # With race detector
go test -v -run TestName ./...   # Specific test
go test -cover ./... | grep coverage

# Code quality
go vet ./...
go fmt ./...
staticcheck ./...
govulncheck ./...

# Mocks
go generate ./...

# Development
make test                         # Quick test
make test-cover                   # With coverage report
make lint                         # Static analysis
make fmt                          # Format code
make check                        # All checks (generate, fmt, lint, test)
```

## Plugins (Optional)

When using Claude Code CLI, install:
```bash
claude plugins install superpowers@claude-plugins-official
claude plugins install code-review@claude-plugins-official
claude plugins install code-simplifier@claude-plugins-official
claude plugins install gopls-lsp@claude-plugins-official
```

Or add to `~/.claude/settings.json`:
```json
{
  "enabledPlugins": [
    "superpowers@claude-plugins-official",
    "code-review@claude-plugins-official",
    "code-simplifier@claude-plugins-official",
    "skill-creator@claude-plugins-official",
    "gopls-lsp@claude-plugins-official"
  ]
}
```

## Key Principles

1. **Explicit over implicit** — no magic init(), no hidden globals
2. **Validate at boundaries** — type structs for all external input
3. **Structured logging** — JSON only, correlation IDs in every log
4. **Test-driven** — write tests before code; ≥90% coverage
5. **Fail fast** — errors wrapped with context; use custom types
6. **Lean dependencies** — prefer stdlib; audit external packages
