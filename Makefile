# ─────────────────────────────────────────────────────────────────────────────
# EDI Simulator – Test commands
# ─────────────────────────────────────────────────────────────────────────────
# make test           → run ALL tests
# make test-unit      → models, validation, transformation, storage (mock only)
# make test-storage   → storage package incl. sqlmock postgres tests
# make test-pipeline  → end-to-end pipeline simulation (no external deps)
# make test-api       → API gateway handler tests
# make test-cover     → full suite with HTML coverage report
# make build          → compile all binaries
# ─────────────────────────────────────────────────────────────────────────────

.PHONY: test test-unit test-storage test-pipeline test-api test-cover build

## Run all tests
test:
	go test ./... -count=1

## Unit tests: models, validation, transformation
test-unit:
	go test ./internal/models/... ./internal/validation/... ./internal/transformation/... -v -count=1

## Storage tests: mock repository + postgres (sqlmock)
test-storage:
	go test ./internal/storage/... -v -count=1

## Pipeline integration tests: full pending→sent→received flow
test-pipeline:
	go test ./internal/pipeline/... -v -count=1

## API gateway handler tests
test-api:
	go test ./cmd/api-gateway/... -v -count=1

## Full suite with coverage report (opens coverage.html)
test-cover:
	go test ./... -count=1 -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## Build all services
build:
	go build ./cmd/api-gateway/...
	go build ./cmd/worker/...
	go build ./cmd/sender/...
	go build ./cmd/receiver/...
