# APEX.BUILD C/C++ Sandbox Image
# Minimal, secure C/C++ execution environment

FROM gcc:13-bookworm

# Security: Create unprivileged user
RUN groupadd -r sandbox && useradd -r -g sandbox -d /home/sandbox -s /bin/false sandbox

# Install minimal dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    libc6-dev \
    && rm -rf /var/lib/apt/lists/* \
    && rm -rf /var/cache/apt/archives/*

# Create work directories
RUN mkdir -p /work /tmp/sandbox \
    && chown -R sandbox:sandbox /work /tmp/sandbox \
    && chmod 1777 /tmp/sandbox

# Remove unnecessary binaries
RUN rm -f /usr/bin/curl /usr/bin/wget /usr/bin/nc /usr/bin/netcat \
    /usr/bin/ssh /usr/bin/scp /usr/bin/sftp 2>/dev/null || true

# Switch to unprivileged user
USER sandbox
WORKDIR /work

# Default command
CMD ["gcc", "--version"]
