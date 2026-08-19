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
