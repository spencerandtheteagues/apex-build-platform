#!/bin/sh

# APEX.BUILD Frontend Docker Entrypoint
# Handles runtime configuration and starts nginx

set -e

echo "🚀 APEX.BUILD Frontend starting..."

# Runtime environment variable substitution
if [ -n "$VITE_API_URL" ] || [ -n "$VITE_WS_URL" ]; then
    echo "📝 Updating runtime configuration..."

    # Create a config file that can be loaded by the app
    cat > /usr/share/nginx/html/config.js << EOF
window.__APEX_CONFIG__ = {
  API_URL: "${VITE_API_URL:-http://localhost:8080/api/v1}",
  WS_URL: "${VITE_WS_URL:-ws://localhost:8080/ws}",
  VERSION: "${APP_VERSION:-1.0.0}",
  ENVIRONMENT: "${NODE_ENV:-production}",
  FEATURES: {
    AI_PROVIDERS: ["claude", "openai", "gemini"],
    COLLABORATION: true,
    CODE_EXECUTION: true,
    REAL_TIME_SYNC: true
  }
};
EOF

    echo "✅ Configuration updated"
fi

# Ensure proper permissions
echo "🔐 Setting up permissions..."
chown -R nginx:nginx /usr/share/nginx/html 2>/dev/null || true

# Test nginx configuration
echo "🔧 Testing nginx configuration..."
nginx -t

echo "🌐 Starting APEX.BUILD Frontend on port 3000..."
echo "📡 API URL: ${VITE_API_URL:-http://localhost:8080/api/v1}"
echo "🔌 WebSocket URL: ${VITE_WS_URL:-ws://localhost:8080/ws}"

# Execute the command passed to the container
exec "$@"