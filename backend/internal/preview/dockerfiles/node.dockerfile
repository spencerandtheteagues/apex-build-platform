# APEX.BUILD Preview Container - Node.js/React/Vue/Svelte
# Secure, minimal image for frontend application previews

FROM node:20-slim

# Create non-root user for security
RUN groupadd -r sandbox && useradd -r -g sandbox -d /home/sandbox -s /sbin/nologin sandbox

# Install serve for static file serving (no-interaction)
RUN npm install -g serve@14 --silent && \
    npm cache clean --force && \
    rm -rf /root/.npm /tmp/*

# Create app directory with proper permissions
RUN mkdir -p /app && chown -R sandbox:sandbox /app

# Set working directory
WORKDIR /app

# Copy project files with sandbox ownership
COPY --chown=sandbox:sandbox . .

# Install dependencies if package.json exists (production only)
RUN if [ -f package.json ]; then \
      npm ci --production --silent 2>/dev/null || \
      npm install --production --silent 2>/dev/null || \
      true; \
    fi && \
    rm -rf /root/.npm /tmp/*

# Build the project if build script exists
RUN if [ -f package.json ] && grep -q '"build"' package.json; then \
      npm run build 2>/dev/null || true; \
    fi

# Clean up npm cache
RUN rm -rf /root/.npm /home/sandbox/.npm /tmp/* 2>/dev/null || true

# Switch to non-root user
USER sandbox

# Expose default port
EXPOSE 3000

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:3000/ || exit 1

# Start server - detect build output directory
CMD if [ -d "dist" ]; then \
      exec serve -s dist -l 3000 -n; \
    elif [ -d "build" ]; then \
      exec serve -s build -l 3000 -n; \
    elif [ -d "out" ]; then \
      exec serve -s out -l 3000 -n; \
    elif [ -d ".next" ]; then \
      exec serve -s .next -l 3000 -n; \
    elif [ -d "public" ]; then \
      exec serve -s public -l 3000 -n; \
    else \
      exec serve -s . -l 3000 -n; \
    fi
