# Hello Service Makefile

.PHONY: build run proto descriptor clean install-tools setup

# Install protobuf tools if not available
install-tools:
	@echo "Installing protobuf tools..."
	@which protoc >/dev/null || (echo "Error: protoc not found. Please install Protocol Buffers compiler" && exit 1)
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@echo "Protobuf tools installed successfully"

# Setup: install tools and generate protobuf files
setup: install-tools proto
	@echo "Setup complete"

# Build the hello service binary
build: setup
	@echo "Building hello service..."
	@mkdir -p build
	go build -o build/hello-service .

# Run the hello service
run: build
	@echo "Starting hello service on port 50051..."
	./build/hello-service

# Generate protobuf files
proto: install-tools
	@echo "Generating protobuf files..."
	@mkdir -p proto
	@export PATH=$$PATH:$$HOME/go/bin && protoc --go_out=proto --go-grpc_out=proto --proto_path=proto proto/hello.proto proto/complex_service.proto
	@echo "Protobuf files generated in local proto/ directory"

# Generate FileDescriptorSet for enhanced ggRMCP features
descriptor: install-tools
	@echo "Generating FileDescriptorSet..."
	@mkdir -p build
	@export PATH=$$PATH:$$HOME/go/bin && protoc --descriptor_set_out=build/hello.binpb \
		--include_source_info \
		--include_imports \
		--proto_path=proto \
		proto/hello.proto
	@export PATH=$$PATH:$$HOME/go/bin && protoc --descriptor_set_out=build/complex_service.binpb \
		--include_source_info \
		--include_imports \
		--proto_path=proto \
		proto/complex_service.proto
	@echo "FileDescriptorSet files created at: examples/hello-service/build/"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -f hello-service
	rm -rf build/
	rm -f proto/*.pb.go

# Show help
help:
	@echo "Available targets:"
	@echo "  install-tools - Install protobuf tools (protoc-gen-go, protoc-gen-go-grpc)"
	@echo "  setup         - Install tools and generate protobuf files"
	@echo "  build         - Build the hello service binary"
	@echo "  run           - Build and run the hello service"
	@echo "  proto         - Generate protobuf Go files"
	@echo "  descriptor    - Generate FileDescriptorSet for ggRMCP"
	@echo "  clean         - Clean build artifacts"
	@echo "  help          - Show this help message"