# APEX.BUILD Frontend Dockerfile
# Multi-stage production build for React/Vite application
# Optimized for security, size, and performance

# ==================================================
# Stage 1: Dependencies
# ==================================================
FROM node:18-alpine AS deps

WORKDIR /app

# Copy package files for dependency caching
COPY package.json package-lock.json* ./

# Install dependencies with clean install
RUN npm ci --only=production=false

# ==================================================
# Stage 2: Build
# ==================================================
FROM node:18-alpine AS builder

# Build arguments
ARG VITE_API_URL=http://localhost:8080/api/v1
ARG VITE_WS_URL=ws://localhost:8080/ws
ARG VERSION=dev
ARG NODE_ENV=production

WORKDIR /app

# Copy dependencies from deps stage
COPY --from=deps /app/node_modules ./node_modules

# Copy source code
COPY . .

# Set environment variables for build
ENV VITE_API_URL=${VITE_API_URL}
ENV VITE_WS_URL=${VITE_WS_URL}
ENV VITE_VERSION=${VERSION}
ENV NODE_ENV=${NODE_ENV}

# Build the application
RUN npm run build

# ==================================================
# Stage 3: Production Runtime (Nginx)
# ==================================================
FROM nginx:1.25-alpine AS production

# Labels for image metadata
LABEL org.opencontainers.image.title="APEX.BUILD Frontend"
LABEL org.opencontainers.image.description="Multi-AI Cloud Development Platform UI"
LABEL org.opencontainers.image.vendor="APEX.BUILD"
LABEL org.opencontainers.image.source="https://github.com/apex-build/apex-build"

# Install runtime dependencies and security updates
RUN apk update && \
    apk upgrade && \
    apk --no-cache add \
        curl \
        tzdata \
    && rm -rf /var/cache/apk/*

# Remove default nginx config
RUN rm /etc/nginx/conf.d/default.conf

# Copy custom nginx configuration
COPY nginx.conf /etc/nginx/nginx.conf

# Copy built application from builder stage
COPY --from=builder /app/dist /usr/share/nginx/html

# Copy entrypoint script for runtime configuration
COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

# Create necessary directories with proper permissions
RUN mkdir -p /var/cache/nginx /var/log/nginx /run && \
    chown -R nginx:nginx /usr/share/nginx/html && \
    chown -R nginx:nginx /var/cache/nginx && \
    chown -R nginx:nginx /var/log/nginx && \
    chown -R nginx:nginx /run && \
    chown -R nginx:nginx /etc/nginx/conf.d && \
    touch /run/nginx.pid && \
    chown nginx:nginx /run/nginx.pid

# Switch to non-root user
USER nginx

# Environment variables
ENV TZ=UTC
ENV NODE_ENV=production

# Expose port
EXPOSE 3000

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:3000/health || exit 1

# Start nginx via entrypoint
ENTRYPOINT ["/docker-entrypoint.sh"]
CMD ["nginx", "-g", "daemon off;"]
