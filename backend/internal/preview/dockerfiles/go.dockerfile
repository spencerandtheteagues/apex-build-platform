# APEX.BUILD Preview Container - Go
# Multi-stage build for secure Go application previews

# Build stage
FROM golang:1.22-bookworm AS builder

WORKDIR /build

# Copy go mod files first for caching
COPY go.mod go.sum* ./
RUN go mod download 2>/dev/null || true

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/server . 2>/dev/null || \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/server ./main.go 2>/dev/null || \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/server ./cmd/... 2>/dev/null || \
    echo "No buildable Go files found"

# Runtime stage
FROM gcr.io/distroless/static-debian12

# Copy the binary from builder
COPY --from=builder /app/server /server

# Copy static files if they exist
COPY --from=builder /build/static /static
COPY --from=builder /build/public /public
COPY --from=builder /build/templates /templates

# Expose port
EXPOSE 8080

# Health check is not supported in distroless, rely on container orchestration

# Run the application
ENTRYPOINT ["/server"]
