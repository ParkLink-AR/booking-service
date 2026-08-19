# Go Coding Rules Setup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Configure this Go repository with portable coding rules covering architecture, standards, testing, security, and observability aligned to Go 1.22+ best practices.

**Architecture:** Create three configuration layers—CLAUDE.md (agent behavior rules), docs/go-rules.md (reference documentation), and Makefile (development commands)—ensuring new sessions inherit consistent coding standards without external dependencies.

**Tech Stack:** Go 1.22+, goccy/go-json, testify/suite, go.uber.org/mock, OpenTelemetry

**Spec:** User-provided "Go Coding Rules — Referencia Portable" specification

## Global Constraints

- Target Go 1.22+
- No hardcoded secrets; validate all external input
- Errors wrapped with `%w`; custom types for domain errors
- JSON tags always `snake_case`; use goccy/go-json (not encoding/json)
- Enums use `iota`, never `type X string`
- Tests use testify/suite with ≥90% coverage on critical packages
- Variable naming: ≥3 chars (except `i`, `j`, `T` in generics)
- Package-oriented design (POD): controller → usecase → service → repository → domain
- Dependencies flow only inward; interfaces declared in consumer layer
- Do not commit code without explicit user request

---

### Task 1: Create CLAUDE.md with agent behavior rules

**Files:**
- Create: `CLAUDE.md`

**Interfaces:**
- Produces: Agent behavior expectations, project structure, naming conventions, and command reference

- [ ] **Step 1: Create CLAUDE.md in repository root**

Create `/home/fluzzi/Desktop/parklink/booking-service/CLAUDE.md` with:

```markdown
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
```

- [ ] **Step 2: Verify CLAUDE.md is readable**

Run: `cat CLAUDE.md | head -50` to confirm file created successfully

- [ ] **Step 3: Commit CLAUDE.md**

```bash
git add CLAUDE.md
git commit -m "docs: add CLAUDE.md with coding rules and architecture"
```

---

### Task 2: Create docs/go-rules.md with complete reference

**Files:**
- Create: `docs/go-rules.md`

**Interfaces:**
- Produces: Complete portable Go rules reference for future sessions

- [ ] **Step 1: Create docs directory if missing**

Run: `mkdir -p docs`

- [ ] **Step 2: Create docs/go-rules.md with full specification**

Create `/home/fluzzi/Desktop/parklink/booking-service/docs/go-rules.md` with the complete specification provided (the entire "Go Coding Rules — Referencia Portable" section from the user specification). This should include sections 0–10 covering behavior, architecture, Go standards, testing, security, logging, observability, performance, documentation, and CI checklist.

The file should be exactly as provided in the specification—no modifications needed. The content starts with "# Go Coding Rules — Referencia Portable" and includes all sections through "## Resumen: qué es portable y qué no".

- [ ] **Step 3: Verify file was created**

Run: `wc -l docs/go-rules.md` (should be 500+ lines)

- [ ] **Step 4: Commit docs/go-rules.md**

```bash
git add docs/go-rules.md
git commit -m "docs: add complete Go coding rules reference"
```

---

### Task 3: Create Makefile with standard development commands

**Files:**
- Create: `Makefile`

**Interfaces:**
- Produces: Standard make targets for testing, linting, formatting, code generation

- [ ] **Step 1: Create Makefile in repository root**

Create `/home/fluzzi/Desktop/parklink/booking-service/Makefile`:

```makefile
.PHONY: test test-cover lint fmt generate check help race

# Default target
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  test        - Run all tests"
	@echo "  test-cover  - Run tests with coverage report (HTML)"
	@echo "  lint        - Run go vet and staticcheck"
	@echo "  fmt         - Format code (gofmt + gofumpt)"
	@echo "  generate    - Regenerate mocks (go generate)"
	@echo "  check       - Run all checks (generate, fmt, lint, test)"
	@echo "  race        - Run tests with race detector"

test:
	@echo "Running tests..."
	go test -short -v ./...

test-cover:
	@echo "Running tests with coverage..."
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

lint:
	@echo "Running go vet..."
	go vet ./...
	@echo "Running staticcheck (if installed)..."
	@staticcheck ./... || echo "staticcheck not installed, skipping"

fmt:
	@echo "Formatting code..."
	go fmt ./...
	@echo "Running gofumpt (if installed)..."
	@gofumpt -w . || echo "gofumpt not installed, using go fmt only"

generate:
	@echo "Running go generate..."
	go generate ./...

race:
	@echo "Running tests with race detector..."
	go test -race ./...

check: generate fmt lint test
	@echo "✓ All checks passed"
```

- [ ] **Step 2: Verify Makefile syntax**

Run: `make help` (should display target list without errors)

- [ ] **Step 3: Test a make target**

Run: `make test` (should run go test successfully or show no tests found)

- [ ] **Step 4: Commit Makefile**

```bash
git add Makefile
git commit -m "build: add Makefile with standard development targets"
```

---

### Task 4: Create .gitignore entries for Go development

**Files:**
- Modify: `.gitignore` (create if missing)

**Interfaces:**
- Produces: Standard Go development exclusions

- [ ] **Step 1: Check if .gitignore exists**

Run: `test -f .gitignore && echo "exists" || echo "missing"`

- [ ] **Step 2: Add Go development patterns**

If .gitignore exists, append these lines. If .gitignore doesn't exist, create it with this content:

```
# Go binaries
bin/
dist/

# Test coverage
coverage.out
coverage.html
*.coverprofile

# IDE
.vscode/
.idea/
*.swp
*.swo

# pprof
cpu.prof
mem.prof
```

- [ ] **Step 3: Verify .gitignore changes**

Run: `tail -20 .gitignore` (should show Go patterns)

- [ ] **Step 4: Commit .gitignore**

```bash
git add .gitignore
git commit -m "build: add Go development entries to .gitignore"
```

---

### Task 5: Create docs/architecture/ directory with C4 diagram template

**Files:**
- Create: `docs/architecture/context.md`

**Interfaces:**
- Produces: Architecture documentation structure for future diagrams

- [ ] **Step 1: Create architecture directory**

Run: `mkdir -p docs/architecture`

- [ ] **Step 2: Create context.md template**

Create `/home/fluzzi/Desktop/parklink/booking-service/docs/architecture/context.md`:

```markdown
# Architecture Overview

## System Context (C4 Level 1)

\`\`\`mermaid
graph TB
    subgraph External["External Systems"]
        PaymentSvc["Payment Service"]
        EmailSvc["Email Service"]
    end
    
    subgraph System["Booking Service"]
        API["HTTP API"]
        Service["Core Logic"]
        DB["Database"]
    end
    
    User["User / Client"]
    
    User -->|REST| API
    API -->|uses| Service
    Service -->|read/write| DB
    Service -->|calls| PaymentSvc
    Service -->|calls| EmailSvc
\`\`\`

## Container Architecture (C4 Level 2)

| Layer | Responsibility | Key Types |
|-------|----------------|-----------|
| **Controller** | HTTP parsing, validation, response mapping | Request, Response DTO |
| **UseCase** | Service orchestration, transaction coordination | Execute() method |
| **Service** | Business logic, domain rules, metrics | entity.*, errors.* |
| **Repository** | Data access, query translation | DTO→Entity mappers |
| **Domain** | Core types, no external dependencies | entity.*, enum.*, errors.* |

## Dependencies

\`\`\`
controller/ ──→ usecase/ ──→ service/ ──→ repository/ ──→ domain/
                                ↓
                            repository/
\`\`\`

**Rule:** Never import backwards or sideways. Interfaces declared in consuming layer.

## Directory Structure

\`\`\`
cmd/
├── bookingd/           → Main binary, DI setup only
└── migrations/         → Database migrations
internal/
├── controller/         → HTTP handlers
├── usecase/            → Orchestration layer
├── service/            → Business logic
├── repository/         → Data access
└── domain/
    ├── entity/         → Core domain structs
    ├── dto/            → Transfer objects
    ├── enum/           → Enums (iota)
    ├── errors/         → Custom error types
    └── constants/      → Domain constants
pkg/
├── logger/             → Structured logging
├── errors/             → Error utilities
└── clock/              → Time abstraction
tests/
├── integration/        → Database + service tests
└── e2e/                → Full request cycle tests
docs/
├── go-rules.md         → Coding standards
├── architecture/       → C4 diagrams & ADRs
└── specs/              → API specs
\`\`\`

## Next Steps

1. Create ADR files under \`docs/architecture/adr/\` for architectural decisions
2. Add OpenAPI spec under \`docs/specs/\` for each HTTP API
3. Document README.md for each package explaining purpose and public API
```

- [ ] **Step 3: Verify template created**

Run: `head -20 docs/architecture/context.md`

- [ ] **Step 4: Commit architecture template**

```bash
git add docs/architecture/
git commit -m "docs: add architecture documentation template"
```

---

### Task 6: Verify configuration with a test commit

**Files:**
- No files modified (verification only)

**Interfaces:**
- Consumes: All files created in Tasks 1–5
- Produces: Confirmation that repository is configured and ready

- [ ] **Step 1: Verify all files exist**

```bash
test -f CLAUDE.md && echo "✓ CLAUDE.md" || echo "✗ CLAUDE.md"
test -f docs/go-rules.md && echo "✓ docs/go-rules.md" || echo "✗ docs/go-rules.md"
test -f Makefile && echo "✓ Makefile" || echo "✗ Makefile"
test -d docs/architecture && echo "✓ docs/architecture/" || echo "✗ docs/architecture/"
test -f docs/architecture/context.md && echo "✓ docs/architecture/context.md" || echo "✗ docs/architecture/context.md"
```

- [ ] **Step 2: Verify git status is clean**

Run: `git status` (should show nothing to commit after all tasks complete)

- [ ] **Step 3: View commit log for session**

Run: `git log --oneline -5` (should show the commits from tasks 1, 2, 3, 4, 5)

- [ ] **Step 4: Confirm configuration is complete**

All configuration files are now in place. New Claude sessions will inherit:
- ✓ CLAUDE.md with project rules and architecture
- ✓ docs/go-rules.md with complete reference for future lookup
- ✓ Makefile with standard development commands
- ✓ docs/architecture/ structure for design documentation
- ✓ Updated .gitignore for Go development

---

## Plan Complete

**Specification coverage:**
- ✅ Section 0 (First session setup) → CLAUDE.md
- ✅ Section 1 (Behavior) → CLAUDE.md + docs/go-rules.md
- ✅ Section 2 (Architecture/POD) → CLAUDE.md + docs/architecture/
- ✅ Section 3–10 (Standards) → docs/go-rules.md (complete reference)
- ✅ Commands → Makefile
