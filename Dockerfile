# ggRMCP Gateway Dockerfile
# Multi-stage build - separate build and runtime environments for minimal image size

# ============================================
# Builder Stage - Compile the application
# ============================================
FROM golang:1.23-alpine AS builder

# Build argument to control mirror selection
# Usage: docker build --build-arg USE_CHINA_MIRROR=false .
ARG USE_CHINA_MIRROR=false

# Conditionally replace Alpine mirrors for China users (faster apk downloads)
# For China: mirrors.aliyun.com
# For others: default dl-cdn.alpinelinux.org
RUN if [ "$USE_CHINA_MIRROR" = "true" ]; then \
        sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories; \
    fi

# Install build-only dependencies (git, make, gcc, musl-dev)
# These will NOT be in the final image
RUN apk add --no-cache git make gcc musl-dev

# Conditionally set Go proxy for China users (faster Go module downloads)
# For China: goproxy.cn
# For others: default proxy.golang.org
ENV GO111MODULE=on
RUN if [ "$USE_CHINA_MIRROR" = "true" ]; then \
        go env -w GOPROXY=https://goproxy.cn,https://goproxy.io,direct; \
    fi

# Set working directory
WORKDIR /build

# Copy go.mod and go.sum first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary with optimizations
# CGO_ENABLED=0 ensures a fully static binary (no libc dependency)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w -X main.version=${VERSION:-dev}" \
    -o /build/grmcp \
    ./cmd/grmcp

# ============================================
# Runner Stage - Minimal runtime environment
# ============================================
FROM alpine:3.19 AS runner

# Build argument (inherited from builder stage)
ARG USE_CHINA_MIRROR=false

# Conditionally replace Alpine mirrors for runtime dependencies
RUN if [ "$USE_CHINA_MIRROR" = "true" ]; then \
        sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories; \
    fi

# Install ONLY runtime dependencies (ca-certificates for HTTPS, tzdata for timezone, wget for healthcheck)
# NO git, gcc, make, or musl-dev!
RUN apk add --no-cache ca-certificates tzdata wget curl

# Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

# Set working directory
WORKDIR /app

# Copy the compiled binary from builder stage
COPY --from=builder /build/grmcp /app/grmcp

# Create directories for descriptors (FileDescriptorSet files)
RUN mkdir -p /app/descriptors && \
    chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Expose HTTP port (MCP Gateway)
EXPOSE 50053

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:50053/health || exit 1

# Environment variables with defaults
ENV GRPC_HOST=localhost \
    GRPC_PORT=50051 \
    HTTP_PORT=50053 \
    LOG_LEVEL=info \
    DEV_MODE=false \
    DESCRIPTOR_PATH=""

# Use shell form to allow environment variable expansion
# The ${DESCRIPTOR_PATH:+--descriptor=${DESCRIPTOR_PATH}} syntax adds the flag only if DESCRIPTOR_PATH is set
CMD /app/grmcp \
    --grpc-host=${GRPC_HOST} \
    --grpc-port=${GRPC_PORT} \
    --http-port=${HTTP_PORT} \
    --log-level=${LOG_LEVEL} \
    ${DESCRIPTOR_PATH:+--descriptor=${DESCRIPTOR_PATH}}

