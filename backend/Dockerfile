# APEX.BUILD Backend Dockerfile
# Multi-stage build for Go application

# Build stage - using Go 1.22
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy all source code first
COPY . .

# Clean up and rebuild go.mod/go.sum
RUN go mod tidy && go mod download

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o apex-server ./cmd/main.go

# Final stage
FROM alpine:latest

WORKDIR /app

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Copy the binary from builder
COPY --from=builder /app/apex-server .

# Expose port
EXPOSE 8080

# Set environment
ENV ENV=production
ENV PORT=8080

# Run the application
CMD ["./apex-server"]
