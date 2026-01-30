#!/bin/bash
# APEX.BUILD Sandbox Image Builder
# Builds all sandbox images for secure code execution

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
IMAGE_PREFIX="${IMAGE_PREFIX:-apex-sandbox}"

echo "Building APEX.BUILD sandbox images..."
echo "Image prefix: $IMAGE_PREFIX"
echo ""

# Languages to build
LANGUAGES=("python" "javascript" "go" "rust" "java" "c")

# Build each image
for lang in "${LANGUAGES[@]}"; do
    dockerfile="$SCRIPT_DIR/Dockerfile.$lang"
    image_name="$IMAGE_PREFIX-$lang:latest"

    if [ -f "$dockerfile" ]; then
        echo "Building $image_name..."
        docker build -t "$image_name" -f "$dockerfile" "$SCRIPT_DIR"
        echo "  Done: $image_name"
    else
        echo "  WARNING: Dockerfile not found for $lang: $dockerfile"
    fi
done

echo ""
echo "All sandbox images built successfully!"
echo ""
echo "Built images:"
docker images | grep "$IMAGE_PREFIX" | awk '{print "  " $1 ":" $2 " (" $7 $8 ")"}'
