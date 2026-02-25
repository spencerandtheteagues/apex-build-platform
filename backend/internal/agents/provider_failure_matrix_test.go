package agents

import (
	"testing"
)

// TestNonRetriableErrorMatrix exhaustively tests every provider-specific error
// pattern that must be classified as non-retriable (auth, billing, quota, model).
func TestNonRetriableErrorMatrix(t *testing.T) {
	am := &AgentManager{}

	nonRetriable := []struct {
		category string
		errMsg   string
	}{
		// Auth failures
		{"auth", "invalid api key provided"},
		{"auth", "OpenAI: Invalid API Key"},
		{"auth", "incorrect api key for claude"},
		{"auth", "api key is invalid for provider openai"},
		{"auth", "Anthropic authentication failed"},
		{"auth", "authentication error: bad token"},
		{"auth", "unauthorized access to model"},
		{"auth", "permission denied for resource"},
		{"auth", "HTTP 401 Unauthorized"},

		// Billing / quota
		{"billing", "billing hard limit reached"},
		{"billing", "payment required to continue"},
		{"billing", "quota exhausted for this project"},
		{"billing", "quota exceeded for openai"},
		{"billing", "INSUFFICIENT_CREDITS from provider"},
		{"billing", "insufficient credits to run generation"},
		{"billing", "insufficient_credits: account balance is 0"},

		// Build-internal terminal states
		{"terminal", "build not active anymore"},
		{"terminal", "build request budget exceeded"},

		// Provider infrastructure gone
		{"provider", "no ai providers available for user"},
		{"provider", "client not available for provider anthropic"},
		{"provider", "failed to select provider: none configured"},

		// Model not found
		{"model", "model gpt-5-turbo not found"},
		{"model", "model not_found_error: claude-x"},
		{"model", "model is unsupported for this request"},
		{"model", "unknown model identifier"},
		{"model", "invalid model specified"},
		{"model", "unsupported for generateContent"},
	}

	for _, tc := range nonRetriable {
		t.Run(tc.category+"/"+tc.errMsg[:min(len(tc.errMsg), 40)], func(t *testing.T) {
			if !am.isNonRetriableAIErrorMessage(tc.errMsg) {
				t.Fatalf("expected non-retriable for %q", tc.errMsg)
			}
		})
	}
}

// TestRetriableErrorMatrix verifies errors that SHOULD allow retries are NOT
// classified as non-retriable.
func TestRetriableErrorMatrix(t *testing.T) {
	am := &AgentManager{}

	retriable := []struct {
		category string
		errMsg   string
	}{
		// Rate limiting (backoff, not terminal)
		{"ratelimit", "429 rate limit exceeded"},
		{"ratelimit", "too many requests to OpenAI"},

		// Transient provider issues (switch provider)
		{"transient", "503 service unavailable"},
		{"transient", "connection timeout after 30s"},
		{"transient", "upstream connection refused"},

		// Context length (reduce context)
		{"context", "context length exceeded: 128000 tokens"},
		{"context", "request too long for model"},
		{"context", "max tokens exceeded"},

		// Build verification (fix and retry)
		{"build", "verification failed: syntax error in main.js"},
		{"build", "build failed: missing dependency"},
		{"build", "compilation error in generated code"},
		{"build", "syntax error at line 42"},

		// Chat endpoint mismatch (recoverable via model switch)
		{"endpoint", "This is not a chat model and not supported in the v1/chat/completions endpoint"},

		// Generic errors (standard retry)
		{"generic", "unexpected server error"},
		{"generic", "internal processing failure"},
	}

	for _, tc := range retriable {
		t.Run(tc.category+"/"+tc.errMsg[:min(len(tc.errMsg), 40)], func(t *testing.T) {
			if am.isNonRetriableAIErrorMessage(tc.errMsg) {
				t.Fatalf("expected retriable for %q", tc.errMsg)
			}
		})
	}
}

// TestDetermineRetryStrategyFullMatrix tests every error category maps to the
// correct retry strategy.
func TestDetermineRetryStrategyFullMatrix(t *testing.T) {
	am := &AgentManager{}

	tests := []struct {
		name     string
		errMsg   string
		expected string
	}{
		// Non-retriable → non_retriable
		{"auth: invalid key", "invalid api key provided", "non_retriable"},
		{"billing: insufficient credits", "INSUFFICIENT_CREDITS from provider", "non_retriable"},
		{"billing: quota exhausted", "quota exhausted for this project", "non_retriable"},
		{"provider: none available", "no ai providers available", "non_retriable"},
		{"model: not found", "model gpt-5-turbo not found", "non_retriable"},

		// Rate limit → backoff
		{"ratelimit: 429", "429 rate limit exceeded", "backoff"},
		{"ratelimit: too many", "too many requests", "backoff"},

		// Transient → switch_provider
		{"transient: 503", "service unavailable 503", "switch_provider"},
		{"transient: timeout", "connection timeout after 30s", "switch_provider"},
		{"transient: connection", "connection refused by upstream", "switch_provider"},

		// Context → reduce_context
		{"context: length", "context length exceeded", "reduce_context"},
		{"context: too long", "request too long for model", "reduce_context"},
		{"context: max tokens", "max tokens limit reached", "reduce_context"},

		// Build failures → fix_and_retry
		{"build: verification", "verification failed: missing export", "fix_and_retry"},
		{"build: syntax", "syntax error in generated code", "fix_and_retry"},
		{"build: compilation", "compilation error at line 10", "fix_and_retry"},

		// Unknown → standard_retry
		{"unknown: generic", "some random unexpected error", "standard_retry"},
		{"unknown: empty", "", "standard_retry"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := am.determineRetryStrategy(tc.errMsg, &Task{})
			if result != tc.expected {
				t.Fatalf("determineRetryStrategy(%q) = %q, want %q", tc.errMsg, result, tc.expected)
			}
		})
	}
}

// TestStrategyToDecisionFullMatrix verifies every strategy maps to the correct
// consensus decision.
func TestStrategyToDecisionFullMatrix(t *testing.T) {
	am := &AgentManager{}

	tests := []struct {
		strategy string
		expected consensusDecision
	}{
		{"switch_provider", decisionSwitchProvider},
		{"fix_and_retry", decisionRetrySame},
		{"standard_retry", decisionRetrySame},
		{"backoff", decisionRetrySame},
		{"reduce_context", decisionRetrySame},
		{"non_retriable", decisionAbort},
		{"abort", decisionAbort},
		{"unknown_strategy", decisionRetrySame},
		{"", decisionRetrySame},
		{"  SWITCH_PROVIDER  ", decisionSwitchProvider},
		{"  NON_RETRIABLE  ", decisionAbort},
	}

	for _, tc := range tests {
		t.Run("strategy="+tc.strategy, func(t *testing.T) {
			decision := am.strategyToDecision(tc.strategy)
			if decision != tc.expected {
				t.Fatalf("strategyToDecision(%q) = %q, want %q", tc.strategy, decision, tc.expected)
			}
		})
	}
}

// TestEndToEndErrorToDecision tests the full pipeline:
// error message → determineRetryStrategy → strategyToDecision
// This confirms no non-retriable error can accidentally become a retry.
func TestEndToEndErrorToDecision(t *testing.T) {
	am := &AgentManager{}

	// Every auth/billing/quota error must end at decisionAbort
	terminalErrors := []string{
		"invalid api key provided",
		"INSUFFICIENT_CREDITS from provider",
		"billing hard limit reached",
		"quota exhausted for this project",
		"authentication failed",
		"build not active anymore",
		"build request budget exceeded",
		"no ai providers available for user",
		"failed to select provider: none configured",
		"model gpt-5-turbo not found",
	}

	for _, errMsg := range terminalErrors {
		t.Run("terminal/"+errMsg[:min(len(errMsg), 35)], func(t *testing.T) {
			strategy := am.determineRetryStrategy(errMsg, &Task{})
			decision := am.strategyToDecision(strategy)
			if decision != decisionAbort {
				t.Fatalf("error %q → strategy=%q → decision=%q, want abort",
					errMsg, strategy, decision)
			}
		})
	}

	// Transient errors must NOT abort
	transientErrors := []string{
		"429 rate limit exceeded",
		"service unavailable",
		"connection timeout",
		"context length exceeded",
		"verification failed: syntax error",
		"unexpected server error",
	}

	for _, errMsg := range transientErrors {
		t.Run("transient/"+errMsg[:min(len(errMsg), 35)], func(t *testing.T) {
			strategy := am.determineRetryStrategy(errMsg, &Task{})
			decision := am.strategyToDecision(strategy)
			if decision == decisionAbort {
				t.Fatalf("error %q → strategy=%q → decision=abort, want non-abort",
					errMsg, strategy)
			}
		})
	}
}

// TestParseConsensusVoteMapping verifies vote parsing from AI provider responses.
func TestParseConsensusVoteMapping(t *testing.T) {
	am := &AgentManager{}

	tests := []struct {
		name     string
		content  string
		expected consensusDecision
	}{
		{"switch provider vote", "VOTE: switch_provider\nRATIONALE: Provider seems down", decisionSwitchProvider},
		{"retry same vote", "VOTE: retry_same\nRATIONALE: Transient error", decisionRetrySame},
		{"spawn solver vote", "VOTE: spawn_solver\nRATIONALE: Needs code fix", decisionSpawnSolver},
		{"abort vote", "VOTE: abort\nRATIONALE: Auth failure", decisionAbort},
		{"no vote found", "I think we should try again", decisionRetrySame},
		{"case insensitive", "vote: SWITCH_PROVIDER\nrationale: down", decisionSwitchProvider},
		{"embedded in text", "Based on the error, my VOTE: abort is to stop. RATIONALE: terminal", decisionAbort},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			decision, _ := am.parseConsensusVote(tc.content, decisionRetrySame)
			if decision != tc.expected {
				t.Fatalf("parseConsensusVote(%q) = %q, want %q", tc.content, decision, tc.expected)
			}
		})
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
