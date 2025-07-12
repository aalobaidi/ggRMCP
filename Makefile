# GrMCP Makefile

.PHONY: build run test build-test-deps clean proto generate lint install-tools

# Build configuration
BINARY_NAME=grmcp
BUILD_DIR=build
PROTO_DIR=proto
EXAMPLE_DIR=examples

# Go build flags
GO_BUILD_FLAGS=-ldflags="-s -w"
GO_TEST_FLAGS=-v -race -coverprofile=coverage.out

# Default target
all: build

# Install development tools
install-tools:
	@echo "Installing development tools..."
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Generate protobuf files for tests
proto: install-tools
	@echo "Generating protobuf files for tests..."
	@if [ -f tests/testdata/complex.proto ]; then \
		mkdir -p pkg/testproto && \
		export PATH=$$PATH:$$HOME/go/bin && protoc --proto_path=tests/testdata \
			--go_out=. --go_opt=paths=source_relative \
			--go-grpc_out=. --go-grpc_opt=paths=source_relative \
			tests/testdata/*.proto && \
		echo "Moving generated files to pkg/testproto..." && \
		find . -maxdepth 1 -name "complex*.pb.go" -exec mv {} pkg/testproto/ \; && \
		echo "Syncing module dependencies..." && \
		go mod tidy; \
	else \
		echo "No protobuf files found in tests/testdata"; \
	fi

# Generate code
generate:
	@echo "Generating code..."
	go generate ./...

# Build the application
build: proto generate
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build $(GO_BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/grmcp

# Build example service
build-example:
	@echo "Building example service..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build $(GO_BUILD_FLAGS) -o $(BUILD_DIR)/hello-service ./examples/hello-service

# Run the application
run: build
	@echo "Running $(BINARY_NAME)..."
	./$(BUILD_DIR)/$(BINARY_NAME) --grpc-host=localhost --grpc-port=50051 --http-port=50052

# Run example service
run-example: build-example
	@echo "Running example service..."
	./$(BUILD_DIR)/hello-service

# Build hello-service dependencies for tests
build-test-deps:
	@echo "Building test dependencies..."
	@make proto
	@cd examples/hello-service && make descriptor

# Run tests
test: build-test-deps
	@echo "Running tests..."
	go test $(GO_TEST_FLAGS) ./...

# Run integration tests
test-integration:
	@echo "Running integration tests..."
	go test $(GO_TEST_FLAGS) -tags=integration ./tests/...

# Lint code
lint: proto
	@echo "Running linter..."
	@export PATH=$$PATH:$$(go env GOPATH)/bin && golangci-lint run --timeout=5m

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
	rm -f coverage.out

# Security scan
security:
	@echo "Running security scan..."
	go list -json -m all | nancy sleuth

# Dependencies
deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod verify

# Update dependencies
deps-update:
	@echo "Updating dependencies..."
	go get -u ./...
	go mod tidy

# Development workflow
dev: install-tools proto generate lint test build

# Help
help:
	@echo "Available targets:"
	@echo "  build          - Build the application"
	@echo "  build-example  - Build example service"
	@echo "  run            - Run the application"
	@echo "  run-example    - Run example service"
	@echo "  test           - Run tests (builds test dependencies automatically)"
	@echo "  build-test-deps - Build test dependencies (hello-service FileDescriptorSet)"
	@echo "  test-integration - Run integration tests"
	@echo "  proto          - Generate protobuf files"
	@echo "  generate       - Generate code"
	@echo "  lint           - Run linter"
	@echo "  fmt            - Format code"
	@echo "  clean          - Clean build artifacts"
	@echo "  security       - Run security scan"
	@echo "  deps           - Download dependencies"
	@echo "  deps-update    - Update dependencies"
	@echo "  dev            - Full development workflow"
	@echo "  install-tools  - Install development tools"
	@echo "  help           - Show this help"