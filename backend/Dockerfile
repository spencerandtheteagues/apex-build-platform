# APEX.BUILD Backend Dockerfile
# Multi-stage production build for Go application
# Optimized for security, size, and performance

# ==================================================
# Stage 1: Build
# ==================================================
FROM golang:1.23-alpine AS builder

# Build arguments for versioning
ARG VERSION=dev
ARG BUILD_TIME
ARG COMMIT_SHA

# Set working directory
WORKDIR /app

# Prevent Go toolchain auto-upgrade
ENV GOTOOLCHAIN=local
ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64

# Install build dependencies
RUN apk add --no-cache \
    git \
    ca-certificates \
    tzdata

# Copy dependency files first for layer caching
COPY go.mod go.sum ./

# Download dependencies (cached if go.mod/go.sum unchanged)
RUN go mod download && go mod verify

# Copy source code
COPY . .

# Build with optimizations and version info
RUN go build \
    -ldflags="-w -s -extldflags '-static' \
    -X main.Version=${VERSION} \
    -X main.BuildTime=${BUILD_TIME} \
    -X main.CommitSHA=${COMMIT_SHA}" \
    -a -installsuffix cgo \
    -o /app/apex-server ./cmd/main.go

# ==================================================
# Stage 2: Production Runtime
# ==================================================
FROM alpine:3.19 AS production

# Labels for image metadata
LABEL org.opencontainers.image.title="APEX.BUILD Backend"
LABEL org.opencontainers.image.description="Multi-AI Cloud Development Platform API"
LABEL org.opencontainers.image.vendor="APEX.BUILD"
LABEL org.opencontainers.image.source="https://github.com/apex-build/apex-build"

# Install runtime dependencies + Node.js (esbuild bundling) + Python3 (FastAPI/Flask preview)
RUN apk --no-cache add \
    ca-certificates \
    tzdata \
    curl \
    nodejs \
    npm \
    python3 \
    py3-pip \
    && npm install -g esbuild@0.24.2 \
    && ln -sf /usr/bin/python3 /usr/bin/python \
    && ln -sf /usr/bin/pip3 /usr/bin/pip \
    && rm -rf /var/cache/apk/* /root/.npm /tmp/*

# Create non-root user for security
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/apex-server /app/apex-server

# Copy timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Create necessary directories
RUN mkdir -p /app/uploads /app/logs /tmp/apex-build-projects && \
    chown -R appuser:appgroup /app /tmp/apex-build-projects

# Switch to non-root user
USER appuser

# Environment variables with secure defaults
ENV ENVIRONMENT=production
ENV PORT=8080
ENV GIN_MODE=release
ENV TZ=UTC

# Log configuration
ENV LOG_FORMAT=json
ENV LOG_LEVEL=info

# Graceful shutdown timeout
ENV GRACEFUL_SHUTDOWN_TIMEOUT=30s

# Expose port
EXPOSE 8080

# Health check - checks /health endpoint every 30 seconds
HEALTHCHECK --interval=30s --timeout=10s --start-period=120s --retries=5 \
    CMD curl -f http://localhost:8080/health || exit 1

# Run the application
ENTRYPOINT ["/app/apex-server"]
