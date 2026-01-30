# APEX.BUILD Go Sandbox Image
# Minimal, secure Go execution environment

FROM golang:1.22-bookworm

# Security: Create unprivileged user
RUN groupadd -r sandbox && useradd -r -g sandbox -d /home/sandbox -s /bin/false sandbox

# Remove unnecessary packages
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/* \
    && rm -rf /var/cache/apt/archives/*

# Create work directories with proper permissions
RUN mkdir -p /work /tmp/sandbox /go/pkg /go/bin \
    && chown -R sandbox:sandbox /work /tmp/sandbox /go \
    && chmod 1777 /tmp/sandbox

# Remove unnecessary binaries
RUN rm -f /usr/bin/curl /usr/bin/wget /usr/bin/nc /usr/bin/netcat \
    /usr/bin/ssh /usr/bin/scp /usr/bin/sftp 2>/dev/null || true

# Set Go environment
ENV GOCACHE=/tmp/go-cache \
    GOPATH=/go \
    CGO_ENABLED=0 \
    GO111MODULE=on

# Switch to unprivileged user
USER sandbox
WORKDIR /work

# Default command
CMD ["go", "version"]
