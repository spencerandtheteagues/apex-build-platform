# Multi-stage build for Go application
FROM golang:1.21-alpine AS builder

# Set working directory
WORKDIR /app

# Install git and ca-certificates (needed for private repos and HTTPS)
RUN apk add --no-cache git ca-certificates

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o main ./cmd/main.go

# Final stage - minimal image
FROM alpine:latest

# Install ca-certificates for HTTPS requests to AI APIs
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN adduser -D -s /bin/sh apex

# Set working directory
WORKDIR /home/apex

# Copy the binary from builder stage
COPY --from=builder /app/main .

# Change ownership to apex user
RUN chown apex:apex main

# Switch to non-root user
USER apex

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the application
CMD ["./main"]