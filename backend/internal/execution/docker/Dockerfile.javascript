# APEX.BUILD JavaScript/Node.js Sandbox Image
# Minimal, secure Node.js execution environment

FROM node:20-slim

# Security: Create unprivileged user
RUN groupadd -r sandbox && useradd -r -g sandbox -d /home/sandbox -s /bin/false sandbox

# Remove unnecessary packages and update
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/* \
    && rm -rf /var/cache/apt/archives/*

# Create work directories
RUN mkdir -p /work /tmp/sandbox \
    && chown -R sandbox:sandbox /work /tmp/sandbox \
    && chmod 1777 /tmp/sandbox

# Remove unnecessary binaries
RUN rm -f /usr/bin/curl /usr/bin/wget /usr/bin/nc /usr/bin/netcat \
    /usr/bin/ssh /usr/bin/scp /usr/bin/sftp 2>/dev/null || true

# Set Node.js environment
ENV NODE_ENV=production \
    NPM_CONFIG_CACHE=/tmp/.npm \
    NODE_OPTIONS="--max-old-space-size=256"

# Switch to unprivileged user
USER sandbox
WORKDIR /work

# Default command
CMD ["node", "--version"]
