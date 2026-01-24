#!/bin/bash

# APEX.BUILD Production Deployment Script
echo "🚀 Starting APEX.BUILD Production Deployment"
echo "============================================="

# Set environment variables
export BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
export VERSION="2.0.0"
export COMMIT_HASH="production"

# Check if existing services are running
echo "📋 Checking existing services..."
if docker ps | grep -q "apex-postgres"; then
    echo "✅ PostgreSQL already running"
else
    echo "❌ PostgreSQL not running - starting..."
    docker run -d \
        --name apex-postgres \
        --network apex-network \
        -e POSTGRES_DB=apex_build \
        -e POSTGRES_USER=postgres \
        -e POSTGRES_PASSWORD=apex_build_production_2024_secure \
        -p 5432:5432 \
        -v postgres_data:/var/lib/postgresql/data \
        postgres:16-alpine
fi

if docker ps | grep -q "apex-redis"; then
    echo "✅ Redis already running"
else
    echo "❌ Redis not running - starting..."
    docker run -d \
        --name apex-redis \
        --network apex-network \
        -p 6379:6379 \
        -v redis_data:/data \
        redis:7-alpine redis-server --requirepass apex_redis_production_2024_secure
fi

# Build the backend image
echo "🔨 Building APEX.BUILD backend..."
cd backend
docker build -f Dockerfile.production -t apex-backend:production . || {
    echo "❌ Build failed"
    exit 1
}

# Stop existing backend if running
echo "🔄 Updating backend service..."
docker stop apex-backend 2>/dev/null || true
docker rm apex-backend 2>/dev/null || true

# Start the backend service
echo "🚀 Starting APEX.BUILD backend..."
docker run -d \
    --name apex-backend \
    --network apex-network \
    -p 8080:8080 \
    -e ENV=production \
    -e DATABASE_URL="postgresql://postgres:apex_build_production_2024_secure@apex-postgres:5432/apex_build" \
    -e REDIS_URL="redis://apex-redis:6379" \
    -e JWT_SECRET="apex_jwt_production_secret_2024_enterprise_grade_security" \
    -e JWT_REFRESH_SECRET="apex_jwt_refresh_production_secret_2024_enterprise_grade" \
    -e ANTHROPIC_API_KEY="sk-ant-api03-your-claude-key-here" \
    -e OPENAI_API_KEY="sk-your-openai-key-here" \
    -e GOOGLE_AI_API_KEY="your-gemini-key-here" \
    -e LOG_LEVEL=info \
    -e ENABLE_METRICS=true \
    -e ENABLE_TRACING=true \
    -v ./uploads:/app/uploads \
    -v ./logs:/app/logs \
    apex-backend:production

# Wait for services to be ready
echo "⏳ Waiting for services to be ready..."
sleep 10

# Check health
echo "🏥 Checking deployment health..."
if curl -f http://localhost:8080/health 2>/dev/null; then
    echo "✅ APEX.BUILD backend is healthy!"
else
    echo "⚠️  Backend health check failed - checking logs..."
    docker logs apex-backend --tail 20
fi

echo ""
echo "🎉 APEX.BUILD Deployment Complete!"
echo "=================================="
echo "📊 Backend: http://localhost:8080"
echo "🏥 Health:  http://localhost:8080/health"
echo "💾 Database: localhost:5432"
echo "🔄 Redis: localhost:6379"
echo ""
echo "📋 Container Status:"
docker ps --filter "name=apex-"
echo ""
echo "🚀 APEX.BUILD is ready to compete!"