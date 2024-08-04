.PHONY: build test run docker-build docker-up clean lint help

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
BINARY_NAME=tonkeyd
MAIN_PATH=./cmd/tonkeyd

# Build the binary
build:
	CGO_ENABLED=1 $(GOBUILD) -o $(BINARY_NAME) $(MAIN_PATH)

# Run tests
test:
	$(GOTEST) -v -race -coverprofile=coverage.out ./...

# Run tests with coverage report
test-coverage: test
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Run the application locally
run: build
	./$(BINARY_NAME) -config ./configs/config.example.yaml

# Run with debug logging
run-debug: build
	./$(BINARY_NAME) -config ./configs/config.example.yaml -debug

# Tidy dependencies
tidy:
	$(GOMOD) tidy

# Download dependencies
deps:
	$(GOMOD) download

# Docker build
docker-build:
	docker build -t tonkey:latest .

# Docker compose up
docker-up:
	docker compose up --build

# Docker compose down
docker-down:
	docker compose down

# Clean build artifacts
clean:
	rm -f $(BINARY_NAME)
	rm -f coverage.out coverage.html
	rm -f tonkey.db
	$(GOCMD) clean -cache -testcache

# Run linter
lint:
	@which golangci-lint > /dev/null || (echo "golangci-lint not installed. Install: https://golangci-lint.run/usage/install/" && exit 1)
	golangci-lint run

# Format code
fmt:
	$(GOCMD) fmt ./...

# Vet code
vet:
	$(GOCMD) vet ./...

# Quick check before commit
check: fmt vet test

# Help
help:
	@echo "Tonkey Makefile Commands:"
	@echo "  make build         - Build the binary"
	@echo "  make test          - Run tests with race detection"
	@echo "  make test-coverage - Run tests and generate HTML coverage report"
	@echo "  make run           - Build and run locally"
	@echo "  make run-debug     - Build and run with debug logging"
	@echo "  make docker-build  - Build Docker image"
	@echo "  make docker-up     - Run with Docker Compose"
	@echo "  make docker-down   - Stop Docker Compose"
	@echo "  make clean         - Clean build artifacts"
	@echo "  make lint          - Run golangci-lint"
	@echo "  make fmt           - Format code"
	@echo "  make vet           - Vet code"
	@echo "  make check         - Run fmt, vet, and test"
	@echo "  make tidy          - Tidy dependencies"
	@echo "  make help          - Show this help message"
