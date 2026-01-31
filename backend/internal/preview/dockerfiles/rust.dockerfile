# APEX.BUILD Preview Container - Rust
# Multi-stage build for secure Rust application previews

# Build stage
FROM rust:1.75-slim-bookworm AS builder

# Install build dependencies
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        pkg-config \
        libssl-dev && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /build

# Copy Cargo files first for caching
COPY Cargo.toml Cargo.lock* ./

# Create dummy source for dependency caching
RUN mkdir src && \
    echo "fn main() {}" > src/main.rs && \
    cargo build --release 2>/dev/null || true && \
    rm -rf src

# Copy actual source code
COPY . .

# Build the application
RUN cargo build --release 2>/dev/null && \
    cp target/release/$(grep -m1 'name' Cargo.toml | cut -d'"' -f2) /app 2>/dev/null || \
    cp target/release/* /app 2>/dev/null || \
    echo "Build failed"

# Runtime stage
FROM debian:bookworm-slim

# Install runtime dependencies
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates \
        libssl3 && \
    rm -rf /var/lib/apt/lists/* && \
    groupadd -r sandbox && \
    useradd -r -g sandbox sandbox

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app /app/server

# Copy static files if they exist
COPY --from=builder /build/static /app/static
COPY --from=builder /build/public /app/public

# Set permissions
RUN chown -R sandbox:sandbox /app

# Switch to non-root user
USER sandbox

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

# Run the application
CMD ["./server"]
