#!/bin/sh

# APEX-BUILD Frontend Docker Entrypoint
# Handles runtime configuration and starts nginx

set -e

echo "🚀 APEX-BUILD Frontend starting..."
PORT="${PORT:-3000}"
API_URL="${VITE_API_URL:-${VITE_API_BASE_URL:-}}"
WS_URL="${VITE_WS_URL:-}"
BROWSER_API_URL="${APEX_BROWSER_API_URL:-/api/v1}"
BROWSER_WS_URL="${APEX_BROWSER_WS_URL:-/ws}"

trim() {
    printf '%s' "$1" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//'
}

derive_proxy_origin() {
    raw="$(trim "$1")"
    suffix="$2"

    raw="${raw%/}"
    if [ -n "$suffix" ]; then
        case "$raw" in
            *"$suffix") raw="${raw%"$suffix"}" ;;
        esac
    fi

    case "$raw" in
        ws://*) raw="http://${raw#ws://}" ;;
        wss://*) raw="https://${raw#wss://}" ;;
    esac

    case "$raw" in
        http://localhost:*|http://127.0.0.1:*)
            printf 'http://host.docker.internal:%s' "${raw##*:}"
            ;;
        https://localhost:*|https://127.0.0.1:*)
            printf 'https://host.docker.internal:%s' "${raw##*:}"
            ;;
        *)
            printf '%s' "$raw"
            ;;
    esac
}

API_PROXY_PASS="$(derive_proxy_origin "$API_URL" "/api/v1")"
if [ -z "$API_PROXY_PASS" ]; then
    API_PROXY_PASS="http://host.docker.internal:8080"
fi

if [ -n "$WS_URL" ]; then
    WS_PROXY_PASS="$(derive_proxy_origin "$WS_URL" "/ws")"
else
    WS_PROXY_PASS="$API_PROXY_PASS"
fi

# Render sets PORT dynamically for Docker web services. Patch nginx config at runtime.
sed -i "s/__PORT__/${PORT}/g" /etc/nginx/nginx.conf
sed -i "s|__API_PROXY_PASS__|${API_PROXY_PASS}|g" /etc/nginx/nginx.conf
sed -i "s|__WS_PROXY_PASS__|${WS_PROXY_PASS}|g" /etc/nginx/nginx.conf

case "$(printf '%s' "${APEX_ENABLE_CROSS_ORIGIN_ISOLATION:-false}" | tr '[:upper:]' '[:lower:]')" in
    1|true|yes|on)
        sed -i 's/# __APEX_CROSS_ORIGIN_ISOLATION_HEADERS__/add_header Cross-Origin-Opener-Policy "same-origin" always;/' /etc/nginx/nginx.conf
        sed -i '/Cross-Origin-Opener-Policy/a\    add_header Cross-Origin-Embedder-Policy "require-corp" always;' /etc/nginx/nginx.conf
        echo "🧪 Cross-origin isolation headers enabled"
        ;;
    *)
        sed -i '/__APEX_CROSS_ORIGIN_ISOLATION_HEADERS__/d' /etc/nginx/nginx.conf
        ;;
esac

# nginx warns if a "user" directive is present while the master process is not root.
# Keep root-based behavior intact, but strip the directive for non-root containers.
if [ "$(id -u)" != "0" ]; then
    sed -i '/^[[:space:]]*user[[:space:]]\+/d' /etc/nginx/nginx.conf
fi

# Runtime environment variable substitution
echo "📝 Updating runtime configuration..."

# The Docker frontend already proxies /api/v1 and /ws through nginx.
# Keep browser-facing runtime config same-origin so auth and websocket traffic
# do not depend on a separate cross-origin API host being reachable.
cat > /usr/share/nginx/html/config.js << EOF
window.__APEX_CONFIG__ = {
  API_URL: "${BROWSER_API_URL}",
  WS_URL: "${BROWSER_WS_URL}",
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

# Ensure proper permissions
echo "🔐 Setting up permissions..."
chown -R nginx:nginx /usr/share/nginx/html 2>/dev/null || true

# Test nginx configuration
echo "🔧 Testing nginx configuration..."
nginx -t

echo "🌐 Starting APEX-BUILD Frontend on port ${PORT}..."
echo "📡 Browser API URL: ${BROWSER_API_URL}"
echo "🔌 Browser WebSocket URL: ${BROWSER_WS_URL}"
echo "↪️ API proxy target: ${API_PROXY_PASS}"
echo "↪️ WS proxy target: ${WS_PROXY_PASS}"

# Execute the command passed to the container
exec "$@"
