// Package pricing provides centralized AI pricing and billing utilities.
//
// Pricing formula:
//
//	BilledCost = max(RawCost(model) × profitMargin × powerSurcharge, RawCost)
//
// Where:
//   - RawCost = actual API cost for the specific model based on token usage
//   - profitMargin = our markup (default 1.5 = 50%), configurable via APEX_PROFIT_MARGIN
//   - powerSurcharge = extra charge for power modes that use more resources
//     (fast=1.0, balanced=1.12, max=1.25) — reflects actual orchestration overhead
//     with a small margin buffer. Model routing already selects more expensive
//     models for higher power modes; the surcharge covers retries, longer context
//     windows, and heavier orchestration.
//
// No-loss guarantee: BilledCost is always >= RawCost, even if env overrides
// set profitMargin or surcharges below 1.0 by mistake.
package pricing

import (
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
)

// Power mode identifiers (aligned with agents.PowerMode values)
const (
	ModeFast     = "fast"
	ModeBalanced = "balanced"
	ModeMax      = "max"
)

// ModelPricing defines per-1M token pricing for a model.
// These MUST match the actual API pricing from each provider.
type ModelPricing struct {
	InputPer1M  float64
	OutputPer1M float64
}

// ProviderPricing groups model pricing for a provider.
type ProviderPricing struct {
	Default ModelPricing
	Models  map[string]ModelPricing
}

// Engine computes pricing, billing, and token estimates.
type Engine struct {
	providers            map[string]ProviderPricing
	profitMargin         float64            // our markup on top of raw API cost (1.5 = 50%)
	powerSurcharges      map[string]float64 // additional surcharge per power mode
	byokRoutingFeePer1M  float64
	defaultPowerMode     string
	defaultMaxTokensHint int
}

var (
	defaultEngine *Engine
	engineOnce    sync.Once
)

// Get returns a singleton pricing engine initialized from environment.
func Get() *Engine {
	engineOnce.Do(func() {
		defaultEngine = newEngineFromEnv()
	})
	return defaultEngine
}

func newEngineFromEnv() *Engine {
	engine := &Engine{
		// Actual API pricing per provider/model (what they charge us per 1M tokens).
		// These must be kept in sync with each provider's published pricing.
		providers: map[string]ProviderPricing{
			"claude": {
				Default: ModelPricing{InputPer1M: 3.00, OutputPer1M: 15.00},
				Models: map[string]ModelPricing{
					"claude-opus-4-6":            {InputPer1M: 15.00, OutputPer1M: 75.00},
					"claude-sonnet-4-5-20250929": {InputPer1M: 3.00, OutputPer1M: 15.00},
					"claude-haiku-4-5-20251001":  {InputPer1M: 0.25, OutputPer1M: 1.25},
				},
			},
			"gpt4": {
				Default: ModelPricing{InputPer1M: 5.00, OutputPer1M: 15.00},
				Models: map[string]ModelPricing{
					"gpt-5.2-codex": {InputPer1M: 8.00, OutputPer1M: 24.00},
					"gpt-5":         {InputPer1M: 5.00, OutputPer1M: 15.00},
					"gpt-4o-mini":   {InputPer1M: 0.15, OutputPer1M: 0.60},
				},
			},
			"gemini": {
				Default: ModelPricing{InputPer1M: 0.50, OutputPer1M: 1.50},
				Models: map[string]ModelPricing{
					"gemini-3-pro-preview":   {InputPer1M: 2.00, OutputPer1M: 6.00},
					"gemini-3-flash-preview": {InputPer1M: 0.50, OutputPer1M: 1.50},
					"gemini-2.5-flash-lite":  {InputPer1M: 0.075, OutputPer1M: 0.30},
				},
			},
			"grok": {
				Default: ModelPricing{InputPer1M: 0.20, OutputPer1M: 0.50},
				Models: map[string]ModelPricing{
					"grok-4-heavy":      {InputPer1M: 2.00, OutputPer1M: 10.00},
					"grok-4.1-thinking": {InputPer1M: 0.30, OutputPer1M: 0.50},
					"grok-4.1":          {InputPer1M: 0.30, OutputPer1M: 0.50},
					"grok-4-fast":       {InputPer1M: 0.20, OutputPer1M: 0.50},
				},
			},
			"ollama": {
				Default: ModelPricing{InputPer1M: 0.0, OutputPer1M: 0.0},
				Models:  map[string]ModelPricing{},
			},
		},

		// Our profit margin: 1.5 = 50% markup on API cost.
		profitMargin: 1.50,

		// Power mode surcharges — reflects actual extra cost of orchestration.
		// Higher modes cost more because they use longer context, more retries,
		// multi-agent coordination, and heavier validation loops.
		// Values set slightly above break-even to maintain margin on overhead.
		powerSurcharges: map[string]float64{
			ModeFast:     1.00, // no surcharge — single-pass, minimal orchestration
			ModeBalanced: 1.12, // 12% — retry loops, extended context, multi-step validation
			ModeMax:      1.25, // 25% — full orchestration: retries, long context, parallel agents, verification
		},

		byokRoutingFeePer1M:  0.25,
		defaultPowerMode:     ModeFast,
		defaultMaxTokensHint: 2000,
	}

	// Environment overrides — change pricing without redeploying
	engine.profitMargin = getEnvFloat("APEX_PROFIT_MARGIN", engine.profitMargin)
	engine.byokRoutingFeePer1M = getEnvFloat("BYOK_ROUTING_FEE_PER_1M", engine.byokRoutingFeePer1M)
	engine.powerSurcharges[ModeFast] = getEnvFloat("APEX_POWER_SURCHARGE_FAST", engine.powerSurcharges[ModeFast])
	engine.powerSurcharges[ModeBalanced] = getEnvFloat("APEX_POWER_SURCHARGE_BALANCED", engine.powerSurcharges[ModeBalanced])
	engine.powerSurcharges[ModeMax] = getEnvFloat("APEX_POWER_SURCHARGE_MAX", engine.powerSurcharges[ModeMax])

	return engine
}

// BilledCost returns the user-facing cost (USD) for a request.
//
//	BilledCost = max(RawCost × profitMargin × powerSurcharge, RawCost)
//
// The no-loss floor guarantees we never charge less than our API cost,
// even if env overrides accidentally set margin/surcharges below 1.0.
//
// For BYOK users we only charge a flat routing fee since they pay the API directly.
func (e *Engine) BilledCost(provider, model string, inputTokens, outputTokens int, powerMode string, isBYOK bool) float64 {
	providerKey := normalizeProvider(provider)
	if providerKey == "ollama" {
		return 0.0
	}
	if isBYOK {
		totalTokens := inputTokens + outputTokens
		return roundUSD((float64(totalTokens) / 1_000_000.0) * e.byokRoutingFeePer1M)
	}

	apiCost := e.RawCost(providerKey, model, inputTokens, outputTokens)
	surcharge := e.powerSurcharge(powerMode)
	billed := roundUSD(apiCost * e.profitMargin * surcharge)

	// No-loss guarantee: never bill less than what the API charges us.
	if billed < apiCost {
		return apiCost
	}
	return billed
}

// RawCost returns the actual API cost (USD) — what the provider charges us.
func (e *Engine) RawCost(provider, model string, inputTokens, outputTokens int) float64 {
	pricing := e.modelPricing(provider, model)
	inputCost := (float64(inputTokens) / 1_000_000.0) * pricing.InputPer1M
	outputCost := (float64(outputTokens) / 1_000_000.0) * pricing.OutputPer1M
	return roundUSD(inputCost + outputCost)
}

// EstimateCost returns a conservative estimate for pre-authorization.
func (e *Engine) EstimateCost(provider, model string, promptChars int, maxTokens int, powerMode string, isBYOK bool) float64 {
	if maxTokens <= 0 {
		maxTokens = e.defaultMaxTokensHint
	}
	inputTokens := e.estimateInputTokens(promptChars)
	return e.BilledCost(provider, model, inputTokens, maxTokens, powerMode, isBYOK)
}

// ProfitMargin returns the current profit margin multiplier.
func (e *Engine) ProfitMargin() float64 {
	return e.profitMargin
}

// PowerMultiplier returns the power surcharge for a power mode.
func (e *Engine) PowerMultiplier(powerMode string) float64 {
	return e.powerSurcharge(powerMode)
}

// DefaultModel returns a reasonable default model for a provider and power mode.
func (e *Engine) DefaultModel(provider, powerMode string) string {
	providerKey := normalizeProvider(provider)
	mode := normalizePowerMode(powerMode, e.defaultPowerMode)

	switch providerKey {
	case "claude":
		if mode == ModeMax {
			return "claude-opus-4-6"
		}
		if mode == ModeBalanced {
			return "claude-sonnet-4-5-20250929"
		}
		return "claude-haiku-4-5-20251001"
	case "gpt4":
		if mode == ModeMax {
			return "gpt-5.2-codex"
		}
		if mode == ModeBalanced {
			return "gpt-5"
		}
		return "gpt-4o-mini"
	case "gemini":
		if mode == ModeMax {
			return "gemini-3-pro-preview"
		}
		if mode == ModeBalanced {
			return "gemini-3-flash-preview"
		}
		return "gemini-2.5-flash-lite"
	case "grok":
		if mode == ModeMax {
			return "grok-4-heavy"
		}
		if mode == ModeBalanced {
			return "grok-4.1-thinking"
		}
		return "grok-4-fast"
	default:
		return ""
	}
}

func (e *Engine) powerSurcharge(powerMode string) float64 {
	mode := normalizePowerMode(powerMode, e.defaultPowerMode)
	if s, ok := e.powerSurcharges[mode]; ok {
		return s
	}
	return 1.0
}

func (e *Engine) modelPricing(provider, model string) ModelPricing {
	providerKey := normalizeProvider(provider)
	pp, ok := e.providers[providerKey]
	if !ok {
		return ModelPricing{}
	}
	if model != "" {
		if mp, ok := pp.Models[model]; ok {
			return mp
		}
	}
	return pp.Default
}

func (e *Engine) estimateInputTokens(promptChars int) int {
	if promptChars <= 0 {
		return 0
	}
	// Conservative heuristic: ~3 chars/token with a 15% buffer + small constant.
	base := int(math.Ceil(float64(promptChars) / 3.0))
	adjusted := int(math.Ceil(float64(base) * 1.15))
	return adjusted + 32
}

func normalizeProvider(provider string) string {
	p := strings.ToLower(strings.TrimSpace(provider))
	switch p {
	case "openai":
		return "gpt4"
	default:
		return p
	}
}

func normalizePowerMode(mode string, fallback string) string {
	m := strings.ToLower(strings.TrimSpace(mode))
	switch m {
	case "power", "max", "maximum":
		return ModeMax
	case "balanced", "balance":
		return ModeBalanced
	case "fast", "cheap", "economy":
		return ModeFast
	}
	if fallback != "" {
		return normalizePowerMode(fallback, ModeFast)
	}
	return ModeFast
}

func getEnvFloat(key string, fallback float64) float64 {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return fallback
	}
	if parsed, err := strconv.ParseFloat(val, 64); err == nil {
		return parsed
	}
	return fallback
}

func roundUSD(value float64) float64 {
	if value == 0 {
		return 0
	}
	return math.Round(value*1_000_000) / 1_000_000
}
