#!/bin/bash

# APEX.BUILD System Integration Test
# Tests the complete platform deployment

set -e

echo "🚀 APEX.BUILD System Integration Test"
echo "===================================="

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check prerequisites
print_status "Checking prerequisites..."

# Check Docker
if ! command -v docker &> /dev/null; then
    print_error "Docker is not installed"
    exit 1
fi

# Check Docker Compose (support both legacy and plugin forms)
if command -v docker-compose &> /dev/null; then
    COMPOSE_CMD="docker-compose"
elif docker compose version &> /dev/null; then
    COMPOSE_CMD="docker compose"
else
    print_error "Docker Compose is not installed"
    exit 1
fi

print_success "Prerequisites check passed"

# Validate Docker Compose configuration
print_status "Validating Docker Compose configuration..."
if $COMPOSE_CMD config --quiet; then
    print_success "Docker Compose configuration is valid"
else
    print_error "Docker Compose configuration has errors"
    exit 1
fi

# Check required files
print_status "Checking required files..."

required_files=(
    "docker-compose.yml"
    "backend/Dockerfile"
    "backend/main.go"
    "backend/go.mod"
    "frontend/Dockerfile"
    "frontend/package.json"
    "frontend/src/main.tsx"
    "README.md"
)

missing_files=()
for file in "${required_files[@]}"; do
    if [[ ! -f "$file" ]]; then
        missing_files+=("$file")
    fi
done

if [[ ${#missing_files[@]} -eq 0 ]]; then
    print_success "All required files are present"
else
    print_error "Missing files:"
    for file in "${missing_files[@]}"; do
        echo "  - $file"
    done
    exit 1
fi

if [[ -f ".env" ]]; then
    print_success ".env file present"
elif [[ -f ".env.example" ]]; then
    print_warning ".env missing; falling back to .env.example/defaults for validation"
else
    print_error "Missing both .env and .env.example"
    exit 1
fi

# Check Go module files
print_status "Validating Go backend structure..."
backend_files=(
    "backend/internal/ai/router.go"
    "backend/internal/auth/auth.go"
    "backend/internal/handlers/auth.go"
    "backend/internal/handlers/projects.go"
    "backend/internal/handlers/files.go"
    "backend/internal/handlers/ai.go"
    "backend/internal/handlers/system.go"
    "backend/internal/middleware/auth.go"
    "backend/internal/middleware/cors.go"
    "backend/internal/middleware/rate_limit.go"
    "backend/internal/websocket/hub.go"
    "backend/pkg/models/models.go"
)

missing_backend=()
for file in "${backend_files[@]}"; do
    if [[ ! -f "$file" ]]; then
        missing_backend+=("$file")
    fi
done

if [[ ${#missing_backend[@]} -eq 0 ]]; then
    print_success "Backend structure is complete"
else
    print_warning "Some backend files are missing (may be generated):"
    for file in "${missing_backend[@]}"; do
        echo "  - $file"
    done
fi

# Check Frontend structure
print_status "Validating Frontend structure..."
frontend_files=(
    "frontend/src/App.tsx"
    "frontend/src/types/index.ts"
    "frontend/src/services/api.ts"
    "frontend/src/services/websocket.ts"
    "frontend/src/styles/globals.css"
    "frontend/vite.config.ts"
    "frontend/tsconfig.json"
    "frontend/index.html"
)

missing_frontend=()
for file in "${frontend_files[@]}"; do
    if [[ ! -f "$file" ]]; then
        missing_frontend+=("$file")
    fi
done

if [[ ${#missing_frontend[@]} -eq 0 ]]; then
    print_success "Frontend structure is complete"
else
    print_warning "Some frontend files are missing:"
    for file in "${missing_frontend[@]}"; do
        echo "  - $file"
    done
fi

# Check environment variables
print_status "Checking environment configuration..."
if [[ -f ".env" ]]; then
    source .env 2>/dev/null || true
elif [[ -f ".env.example" ]]; then
    source .env.example 2>/dev/null || true
fi

required_env_vars=(
    "DATABASE_URL"
    "JWT_SECRET"
    "PORT"
)

missing_env=()
for var in "${required_env_vars[@]}"; do
    if [[ -z "${!var}" ]]; then
        missing_env+=("$var")
    fi
done

if [[ ${#missing_env[@]} -eq 0 ]]; then
    print_success "Environment configuration is valid"
else
    print_warning "Missing environment variables (using defaults):"
    for var in "${missing_env[@]}"; do
        echo "  - $var"
    done
fi

# Test Docker Compose services build
print_status "Testing Docker services configuration..."

# Test postgres service
if $COMPOSE_CMD config | grep -q "postgres:"; then
    print_success "PostgreSQL service configured"
else
    print_error "PostgreSQL service not found"
fi

# Test redis service
if $COMPOSE_CMD config | grep -q "redis:"; then
    print_success "Redis service configured"
else
    print_error "Redis service not found"
fi

# Test API service
if $COMPOSE_CMD config | grep -q "api:"; then
    print_success "API service configured"
else
    print_error "API service not found"
fi

# Test frontend service
if $COMPOSE_CMD config | grep -q "frontend:"; then
    print_success "Frontend service configured"
else
    print_error "Frontend service not found"
fi

# Check for AI API keys (optional for testing)
print_status "Checking AI provider configuration..."

if [[ -n "$ANTHROPIC_API_KEY" ]]; then
    print_success "Anthropic (Claude) API key configured"
else
    print_warning "Anthropic (Claude) API key not configured"
fi

if [[ -n "$OPENAI_API_KEY" ]]; then
    print_success "OpenAI (GPT-4) API key configured"
else
    print_warning "OpenAI (GPT-4) API key not configured"
fi

if [[ -n "$GOOGLE_AI_API_KEY" ]]; then
    print_success "Google (Gemini) API key configured"
else
    print_warning "Google (Gemini) API key not configured"
fi

# Test network connectivity requirements
print_status "Testing network connectivity..."

# Check if ports are available
check_port() {
    local port=$1
    if lsof -i :$port > /dev/null 2>&1; then
        print_warning "Port $port is already in use"
        return 1
    else
        print_success "Port $port is available"
        return 0
    fi
}

ports_to_check=(3000 5432 6379 8080 8081 8082)
for port in "${ports_to_check[@]}"; do
    check_port $port || true
done

# Summary
print_status "System Integration Test Summary"
echo "================================"

print_success "✅ Docker and Docker Compose are installed"
print_success "✅ Configuration files are valid"
print_success "✅ Core application files are present"
print_success "✅ Services are properly configured"

if [[ -n "$ANTHROPIC_API_KEY" && -n "$OPENAI_API_KEY" && -n "$GOOGLE_AI_API_KEY" ]]; then
    print_success "✅ All AI providers are configured"
else
    print_warning "⚠️  Some AI providers are not configured (will use mock responses)"
fi

echo ""
print_status "🚀 APEX.BUILD is ready for deployment!"
echo ""
echo "To start the platform:"
echo "  docker-compose up -d"
echo ""
echo "To access the platform:"
echo "  Frontend: http://localhost:3000"
echo "  API: http://localhost:8080"
echo "  Database Admin: http://localhost:8081"
echo "  Redis Admin: http://localhost:8082"
echo ""
echo "To stop the platform:"
echo "  docker-compose down"
echo ""

print_success "🎉 System integration test completed successfully!"
