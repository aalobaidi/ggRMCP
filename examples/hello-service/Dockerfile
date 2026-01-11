# Hello Service Dockerfile
# Example gRPC service for testing ggRMCP Gateway

# ============================================
# Builder Stage - Build with protobuf generation
# ============================================
FROM golang:1.24-alpine AS builder

# Build argument to control mirror selection
# Usage: docker build --build-arg USE_CHINA_MIRROR=true .
ARG USE_CHINA_MIRROR=false

# Conditionally replace Alpine mirrors for China users (faster apk downloads)
RUN if [ "$USE_CHINA_MIRROR" = "true" ]; then \
        sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories; \
    fi

# Install build dependencies and protobuf tools
RUN apk add --no-cache git make gcc musl-dev protobuf protobuf-dev

# Conditionally set Go proxy for China users (faster Go module downloads)
ENV GO111MODULE=on
RUN if [ "$USE_CHINA_MIRROR" = "true" ]; then \
        go env -w GOPROXY=https://goproxy.cn,https://goproxy.io,direct; \
    fi

# Install Go protobuf plugins (reference: Makefile install-tools)
# Use v1.5.1 for protoc-gen-go-grpc to maintain Go 1.23 compatibility
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1

# Set working directory
WORKDIR /build

# Copy go.mod and go.sum
COPY go.mod go.sum ./
RUN go mod download

# Copy proto files first (only .proto files, exclude .pb.go)
COPY proto/*.proto ./proto/

# Generate protobuf code using source_relative paths
# This generates .pb.go files in the proto/ directory
RUN protoc --proto_path=proto \
    --go_out=proto --go_opt=paths=source_relative \
    --go-grpc_out=proto --go-grpc_opt=paths=source_relative \
    proto/*.proto

# Generate FileDescriptorSet (.binpb) for ggRMCP Gateway (reference: Makefile descriptor)
# This includes source info and comments for enhanced MCP tool descriptions
RUN mkdir -p build && \
    protoc --proto_path=proto \
    --descriptor_set_out=build/hello.binpb \
    --include_source_info \
    --include_imports \
    proto/*.proto

# Copy only application source code (main.go), not the entire directory
# This avoids copying pre-generated .pb.go files or build artifacts
COPY main.go ./

# Build the binary directly
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w" \
    -o /build/hello-service \
    .

# ============================================
# Runner Stage - Minimal runtime environment
# ============================================
FROM alpine:latest AS runner

# Build argument (inherited from builder stage)
ARG USE_CHINA_MIRROR=false

# Conditionally replace Alpine mirrors for runtime dependencies
RUN if [ "$USE_CHINA_MIRROR" = "true" ]; then \
        sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories; \
    fi

# Install ca-certificates and create non-root user
RUN apk --no-cache add ca-certificates && \
    addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

# Set working directory
WORKDIR /app

# Create directory for descriptor files that will be shared with gateway
RUN mkdir -p /app/descriptors

# Copy the compiled binary and descriptor file from builder stage
COPY --from=builder /build/hello-service /app/hello-service
COPY --from=builder /build/build/hello.binpb /app/descriptors/hello.binpb

# Change ownership
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Expose gRPC port
EXPOSE 50051

# Health check (basic TCP check)
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD nc -z localhost 50051 || exit 1

# Environment variables
ENV PORT=50051

# Run the service
CMD ["/app/hello-service", "--port=50051"]

