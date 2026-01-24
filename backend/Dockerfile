# APEX.BUILD Backend Dockerfile
# Multi-stage build for Go application

# Build stage - using Go 1.22
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Set Go toolchain to prevent auto-upgrade
ENV GOTOOLCHAIN=local

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy go.mod and go.sum first for dependency caching
COPY go.mod go.sum ./

# Download dependencies (uses cached layer if go.mod/go.sum unchanged)
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the application with optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o apex-server ./cmd/main.go

# Final stage - minimal runtime
FROM alpine:3.19

WORKDIR /app

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user for security
RUN adduser -D -g '' appuser

# Copy the binary from builder
COPY --from=builder /app/apex-server .

# Change ownership
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8080

# Set environment
ENV ENVIRONMENT=production
ENV PORT=8080
ENV GIN_MODE=release

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the application
CMD ["./apex-server"]
