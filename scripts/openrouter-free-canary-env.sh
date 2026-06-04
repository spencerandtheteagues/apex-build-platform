#!/usr/bin/env bash
set -euo pipefail

# Source this before local/live paid canaries and paid testing when the test must
# exercise paid/full-stack Apex behavior without spending flagship-provider model
# credits. It keeps provider_mode=platform and pins every build role to the
# OpenRouter provider with a free model override.
#
# Usage:
#   source scripts/openrouter-free-canary-env.sh
#   ./scripts/run_platform_canary_matrix.sh
#
# Required for real calls:
#   Production/staging backend must already have OPENROUTER_API_KEY configured.
#   This script never prints or sets secret values.

export APEX_AI_TESTING_PROFILE="${APEX_AI_TESTING_PROFILE:-openrouter-free}"
export APEX_LIVE_TEST_MODEL_PROFILE="${APEX_LIVE_TEST_MODEL_PROFILE:-openrouter-free}"
export APEX_PROVIDER_MODE="${APEX_PROVIDER_MODE:-platform}"
export PROVIDER_MODE="${PROVIDER_MODE:-$APEX_PROVIDER_MODE}"
export APEX_BYOK_OLLAMA_ONLY="${APEX_BYOK_OLLAMA_ONLY:-0}"
export BYOK_OLLAMA_ONLY="${BYOK_OLLAMA_ONLY:-$APEX_BYOK_OLLAMA_ONLY}"

OPENROUTER_FREE_MODEL="${APEX_OPENROUTER_FREE_MODEL:-${OPENROUTER_FREE_MODEL:-moonshotai/kimi-k2.6:free}}"
export APEX_OPENROUTER_FREE_MODEL="$OPENROUTER_FREE_MODEL"

export APEX_ROLE_ASSIGNMENTS_JSON="${APEX_ROLE_ASSIGNMENTS_JSON:-{\"architect\":\"openrouter\",\"coder\":\"openrouter\",\"tester\":\"openrouter\",\"devops\":\"openrouter\"}}"
export ROLE_ASSIGNMENTS_JSON="${ROLE_ASSIGNMENTS_JSON:-$APEX_ROLE_ASSIGNMENTS_JSON}"
export APEX_PROVIDER_MODEL_OVERRIDES_JSON="${APEX_PROVIDER_MODEL_OVERRIDES_JSON:-{\"openrouter\":\"$OPENROUTER_FREE_MODEL\"}}"
export PROVIDER_MODEL_OVERRIDES_JSON="${PROVIDER_MODEL_OVERRIDES_JSON:-$APEX_PROVIDER_MODEL_OVERRIDES_JSON}"

echo "APEX OpenRouter free canary profile enabled"
echo "APEX_PROVIDER_MODE=$APEX_PROVIDER_MODE"
echo "APEX_LIVE_TEST_MODEL_PROFILE=$APEX_LIVE_TEST_MODEL_PROFILE"
echo "APEX_OPENROUTER_FREE_MODEL=$APEX_OPENROUTER_FREE_MODEL"
