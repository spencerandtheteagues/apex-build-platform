// Package pricing provides centralized AI pricing and billing utilities.
// It computes platform costs, BYOK routing fees, and credit multipliers.
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
	powerMultipliers     map[string]float64
	byokRoutingFeePer1M  float64
	safetyMultiplier     float64
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
	// Defaults are conservative and can be tuned via env without code changes.
	engine := &Engine{
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
		powerMultipliers: map[string]float64{
			ModeFast:     1.60,
			ModeBalanced: 1.80,
			ModeMax:      2.00,
		},
		byokRoutingFeePer1M:  0.25,
		safetyMultiplier:     1.00,
		defaultPowerMode:     ModeFast,
		defaultMaxTokensHint: 2000,
	}

	// Optional environment overrides
	engine.byokRoutingFeePer1M = getEnvFloat("BYOK_ROUTING_FEE_PER_1M", engine.byokRoutingFeePer1M)
	engine.safetyMultiplier = getEnvFloat("PRICING_SAFETY_MULTIPLIER", engine.safetyMultiplier)
	engine.powerMultipliers[ModeFast] = getEnvFloat("PRICING_MULTIPLIER_FAST", engine.powerMultipliers[ModeFast])
	engine.powerMultipliers[ModeBalanced] = getEnvFloat("PRICING_MULTIPLIER_BALANCED", engine.powerMultipliers[ModeBalanced])
	engine.powerMultipliers[ModeMax] = getEnvFloat("PRICING_MULTIPLIER_MAX", engine.powerMultipliers[ModeMax])

	return engine
}

// BilledCost returns the user-facing cost (USD) for a request.
func (e *Engine) BilledCost(provider, model string, inputTokens, outputTokens int, powerMode string, isBYOK bool) float64 {
	providerKey := normalizeProvider(provider)
	if providerKey == "ollama" {
		return 0.0
	}
	if isBYOK {
		totalTokens := inputTokens + outputTokens
		return roundUSD((float64(totalTokens) / 1_000_000.0) * e.byokRoutingFeePer1M)
	}

	raw := e.RawCost(providerKey, model, inputTokens, outputTokens)
	multiplier := e.powerMultiplier(powerMode)
	return roundUSD(raw * multiplier * e.safetyMultiplier)
}

// RawCost returns the platform cost (USD) before markup.
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

// PowerMultiplier returns the billing multiplier for a power mode.
func (e *Engine) PowerMultiplier(powerMode string) float64 {
	return e.powerMultiplier(powerMode)
}

func (e *Engine) powerMultiplier(powerMode string) float64 {
	mode := normalizePowerMode(powerMode, e.defaultPowerMode)
	if m, ok := e.powerMultipliers[mode]; ok {
		return m
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
