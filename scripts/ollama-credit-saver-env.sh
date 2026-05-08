#!/usr/bin/env bash
set -euo pipefail

# Source this before local/live AI-backed canaries to route model-heavy testing
# through one Ollama-compatible endpoint instead of managed cloud-provider keys.
#
# Usage:
#   source scripts/ollama-credit-saver-env.sh
#   ./scripts/run_live_golden_canary_matrix.sh
#
# Required for real calls:
#   OLLAMA_URL=http://127.0.0.1:11434
#   Optional: OLLAMA_API_KEY=... for Ollama Cloud / compatible hosted endpoint

export APEX_AI_TESTING_PROFILE="${APEX_AI_TESTING_PROFILE:-ollama-credit-saver}"
export APEX_LIVE_TEST_MODEL_PROFILE="${APEX_LIVE_TEST_MODEL_PROFILE:-ollama-credit-saver}"

export OLLAMA_URL="${OLLAMA_URL:-${OLLAMA_HOST:-http://127.0.0.1:11434}}"
export OLLAMA_HOST="${OLLAMA_HOST:-$OLLAMA_URL}"

export KIMI_OLLAMA_MODEL="${KIMI_OLLAMA_MODEL:-kimi-k2.6}"
export GLM_OLLAMA_MODEL="${GLM_OLLAMA_MODEL:-glm-5.1}"
export DEEPSEEK_OLLAMA_MODEL="${DEEPSEEK_OLLAMA_MODEL:-deepseek-v4-pro}"
export QWEN_OLLAMA_MODEL="${QWEN_OLLAMA_MODEL:-qwen3:latest}"

# Plain Ollama provider defaults: Kimi is the orchestrator / planning model.
export OLLAMA_MODEL_DEFAULT="${OLLAMA_MODEL_DEFAULT:-$KIMI_OLLAMA_MODEL}"
export OLLAMA_MODEL_BALANCED="${OLLAMA_MODEL_BALANCED:-$KIMI_OLLAMA_MODEL}"
export OLLAMA_MODEL_MAX="${OLLAMA_MODEL_MAX:-$KIMI_OLLAMA_MODEL}"
export OLLAMA_MODEL_FAST="${OLLAMA_MODEL_FAST:-$QWEN_OLLAMA_MODEL}"

# Provider-slot emulation. These make Apex exercise its normal provider routing
# graph while the actual calls still go to the same Ollama-compatible endpoint.
export CLAUDE_OLLAMA_URL="${CLAUDE_OLLAMA_URL:-$OLLAMA_URL}"
export CLAUDE_OLLAMA_MODEL="${CLAUDE_OLLAMA_MODEL:-$KIMI_OLLAMA_MODEL}"
export OPENAI_OLLAMA_URL="${OPENAI_OLLAMA_URL:-$OLLAMA_URL}"
export OPENAI_OLLAMA_MODEL="${OPENAI_OLLAMA_MODEL:-$QWEN_OLLAMA_MODEL}"
export GEMINI_OLLAMA_URL="${GEMINI_OLLAMA_URL:-$OLLAMA_URL}"
export GEMINI_OLLAMA_MODEL="${GEMINI_OLLAMA_MODEL:-$GLM_OLLAMA_MODEL}"
export GROK_OLLAMA_URL="${GROK_OLLAMA_URL:-$OLLAMA_URL}"
export GROK_OLLAMA_MODEL="${GROK_OLLAMA_MODEL:-$DEEPSEEK_OLLAMA_MODEL}"
export DEEPSEEK_OLLAMA_URL="${DEEPSEEK_OLLAMA_URL:-$OLLAMA_URL}"
export GLM_OLLAMA_URL="${GLM_OLLAMA_URL:-$OLLAMA_URL}"

echo "APEX Ollama credit-saver profile enabled"
echo "OLLAMA_URL=$OLLAMA_URL"
echo "KIMI_OLLAMA_MODEL=$KIMI_OLLAMA_MODEL"
echo "GLM_OLLAMA_MODEL=$GLM_OLLAMA_MODEL"
echo "DEEPSEEK_OLLAMA_MODEL=$DEEPSEEK_OLLAMA_MODEL"
echo "QWEN_OLLAMA_MODEL=$QWEN_OLLAMA_MODEL"
