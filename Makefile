.PHONY: build run clean test

build:
	go build -o vectoreologist ./cmd/vectoreologist

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

# Install dependencies
deps:
	go mod tidy
	go mod download

# Clean build artifacts
clean:
	rm -f vectoreologist
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
