.PHONY: build lens all run clean test

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

# Run with default settings
excavate:
	./vectoreologist --collection kae_chunks --sample 5000

# Run on meta-graph
meta:
	./vectoreologist --collection kae_meta_graph --sample 100

# Run on your GPT history
history:
	./vectoreologist --collection marks_gpt_history --sample 2000

# Run on QMU forum
forum:
	./vectoreologist --collection qmu_forum --sample 300

# Watch kae_chunks every 5 minutes
watch:
	./vectoreologist --collection kae_chunks --sample 5000 --watch 5m

# Watch meta-graph every 10 minutes
watch-meta:
	./vectoreologist --collection kae_meta_graph --sample 100 --watch 10m

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
