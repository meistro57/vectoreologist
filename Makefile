.PHONY: build lens all run clean test \
        redis-start redis-stop \
        run-collection run-redis run-watch

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

all: build lens

build:
	go build $(LDFLAGS) -o vectoreologist ./cmd/vectoreologist

lens:
	go build $(LDFLAGS) -o vectoreologist-lens ./cmd/vectoreologist-lens

install-lens: lens
	cp vectoreologist-lens /usr/local/bin/

run-lens:
	./vectoreologist-lens findings/vectoreology_*.json

run:
	go run ./cmd/vectoreologist

# ── Redis Docker helpers ──────────────────────────────────────────────────────

redis-start:
	./scripts/start-redis.sh

redis-stop:
	docker stop vectoreologist-redis 2>/dev/null || true

# ── Standard runs ─────────────────────────────────────────────────────────────
# Override COLLECTION to target a specific Qdrant collection.
# Example: make run-collection COLLECTION=my_embeddings SAMPLE=5000

COLLECTION ?= my_collection
SAMPLE     ?= 5000

run-collection:
	./vectoreologist --collection $(COLLECTION) --sample $(SAMPLE)

# ── Redis-backed runs (streaming extraction, lower Go heap) ──────────────────
# Start Redis first: make redis-start
# Example: make run-redis COLLECTION=my_large_collection

REDIS_URL ?= redis://localhost:6379

run-redis:
	./vectoreologist --collection $(COLLECTION) \
	    --redis-url $(REDIS_URL)

# ── Watch mode ────────────────────────────────────────────────────────────────
# Example: make run-watch COLLECTION=my_collection WATCH=5m

WATCH ?= 5m

run-watch:
	./vectoreologist --collection $(COLLECTION) --sample $(SAMPLE) --watch $(WATCH)

# Install dependencies
deps:
	go mod tidy
	go mod download

# Clean build artifacts
clean:
	rm -f vectoreologist vectoreologist-lens
	rm -rf findings/

# Run tests
test:
	go test ./...

# Format code
fmt:
	go fmt ./...

# Lint
lint:
	golangci-lint run
