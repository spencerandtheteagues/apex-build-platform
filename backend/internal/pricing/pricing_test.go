package pricing

import (
	"math"
	"testing"
)

// newTestEngine creates an engine with known values (no env vars).
func newTestEngine() *Engine {
	return &Engine{
		providers: map[string]ProviderPricing{
			"claude": {
				Default: ModelPricing{InputPer1M: 3.00, OutputPer1M: 15.00},
				Models: map[string]ModelPricing{
					"claude-opus-4-6":           {InputPer1M: 15.00, OutputPer1M: 75.00},
					"claude-haiku-4-5-20251001": {InputPer1M: 0.25, OutputPer1M: 1.25},
				},
			},
			"gpt4": {
				Default: ModelPricing{InputPer1M: 5.00, OutputPer1M: 15.00},
				Models: map[string]ModelPricing{
					"gpt-5":       {InputPer1M: 5.00, OutputPer1M: 15.00},
					"gpt-4o-mini": {InputPer1M: 0.15, OutputPer1M: 0.60},
				},
			},
			"gemini": {
				Default: ModelPricing{InputPer1M: 0.50, OutputPer1M: 1.50},
				Models: map[string]ModelPricing{
					"gemini-3-flash-preview": {InputPer1M: 0.50, OutputPer1M: 1.50},
				},
			},
			"grok": {
				Default: ModelPricing{InputPer1M: 0.20, OutputPer1M: 0.50},
				Models: map[string]ModelPricing{
					"grok-4-fast": {InputPer1M: 0.20, OutputPer1M: 0.50},
				},
			},
			"ollama": {
				Default: ModelPricing{InputPer1M: 0.0, OutputPer1M: 0.0},
			},
		},
		profitMargin: 1.50,
		powerSurcharges: map[string]float64{
			ModeFast:     1.00,
			ModeBalanced: 1.12,
			ModeMax:      1.25,
		},
		byokRoutingFeePer1M:  0.25,
		defaultPowerMode:     ModeFast,
		defaultMaxTokensHint: 2000,
	}
}

func almostEqual(a, b, tolerance float64) bool {
	return math.Abs(a-b) < tolerance
}

func TestRawCost_IsActualAPICost(t *testing.T) {
	e := newTestEngine()

	tests := []struct {
		name         string
		provider     string
		model        string
		inputTokens  int
		outputTokens int
		wantCost     float64
	}{
		{
			name:         "Claude Opus: 1000 in / 500 out",
			provider:     "claude",
			model:        "claude-opus-4-6",
			inputTokens:  1000,
			outputTokens: 500,
			// (1000/1M)*15.00 + (500/1M)*75.00 = 0.015 + 0.0375 = 0.0525
			wantCost: 0.0525,
		},
		{
			name:         "Claude Haiku: 10000 in / 5000 out",
			provider:     "claude",
			model:        "claude-haiku-4-5-20251001",
			inputTokens:  10000,
			outputTokens: 5000,
			// (10000/1M)*0.25 + (5000/1M)*1.25 = 0.0025 + 0.00625 = 0.00875
			wantCost: 0.00875,
		},
		{
			name:         "GPT-5: 2000 in / 1000 out",
			provider:     "gpt4",
			model:        "gpt-5",
			inputTokens:  2000,
			outputTokens: 1000,
			// (2000/1M)*5.00 + (1000/1M)*15.00 = 0.01 + 0.015 = 0.025
			wantCost: 0.025,
		},
		{
			name:         "Gemini Flash: 5000 in / 2000 out",
			provider:     "gemini",
			model:        "gemini-3-flash-preview",
			inputTokens:  5000,
			outputTokens: 2000,
			// (5000/1M)*0.50 + (2000/1M)*1.50 = 0.0025 + 0.003 = 0.0055
			wantCost: 0.0055,
		},
		{
			name:         "Grok Fast: 3000 in / 1000 out",
			provider:     "grok",
			model:        "grok-4-fast",
			inputTokens:  3000,
			outputTokens: 1000,
			// (3000/1M)*0.20 + (1000/1M)*0.50 = 0.0006 + 0.0005 = 0.0011
			wantCost: 0.0011,
		},
		{
			name:         "Ollama is free",
			provider:     "ollama",
			model:        "llama3",
			inputTokens:  100000,
			outputTokens: 50000,
			wantCost:     0.0,
		},
		{
			name:         "OpenAI normalizes to gpt4",
			provider:     "openai",
			model:        "gpt-5",
			inputTokens:  1000,
			outputTokens: 1000,
			// Same as gpt4/gpt-5: (1000/1M)*5.00 + (1000/1M)*15.00 = 0.02
			wantCost: 0.02,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := e.RawCost(tt.provider, tt.model, tt.inputTokens, tt.outputTokens)
			if !almostEqual(got, tt.wantCost, 0.000001) {
				t.Errorf("RawCost() = %f, want %f", got, tt.wantCost)
			}
		})
	}
}

func TestBilledCost_FormulaIsAPICostTimesMarginTimesPowerSurcharge(t *testing.T) {
	e := newTestEngine()

	tests := []struct {
		name      string
		provider  string
		model     string
		inTok     int
		outTok    int
		powerMode string
		wantRaw   float64
		wantBill  float64
	}{
		{
			name:      "Claude Opus fast mode: raw × 1.5 × 1.0",
			provider:  "claude",
			model:     "claude-opus-4-6",
			inTok:     1000,
			outTok:    500,
			powerMode: "fast",
			wantRaw:   0.0525,
			wantBill:  0.0525 * 1.5 * 1.0, // 0.07875
		},
		{
			name:      "Claude Opus balanced mode: raw × 1.5 × 1.12",
			provider:  "claude",
			model:     "claude-opus-4-6",
			inTok:     1000,
			outTok:    500,
			powerMode: "balanced",
			wantRaw:   0.0525,
			wantBill:  0.0525 * 1.5 * 1.12, // 0.0882
		},
		{
			name:      "Claude Opus max mode: raw × 1.5 × 1.25",
			provider:  "claude",
			model:     "claude-opus-4-6",
			inTok:     1000,
			outTok:    500,
			powerMode: "max",
			wantRaw:   0.0525,
			wantBill:  0.0525 * 1.5 * 1.25, // 0.0984375
		},
		{
			name:      "GPT-4o-mini fast: cheapest combo",
			provider:  "gpt4",
			model:     "gpt-4o-mini",
			inTok:     10000,
			outTok:    5000,
			powerMode: "fast",
			// raw = (10000/1M)*0.15 + (5000/1M)*0.60 = 0.0015 + 0.003 = 0.0045
			wantRaw:  0.0045,
			wantBill: 0.0045 * 1.5, // 0.00675
		},
		{
			name:      "Grok fast: low-cost provider",
			provider:  "grok",
			model:     "grok-4-fast",
			inTok:     5000,
			outTok:    2000,
			powerMode: "fast",
			// raw = (5000/1M)*0.20 + (2000/1M)*0.50 = 0.001 + 0.001 = 0.002
			wantRaw:  0.002,
			wantBill: 0.002 * 1.5, // 0.003
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := e.RawCost(tt.provider, tt.model, tt.inTok, tt.outTok)
			if !almostEqual(raw, tt.wantRaw, 0.000001) {
				t.Errorf("RawCost = %f, want %f", raw, tt.wantRaw)
			}
			billed := e.BilledCost(tt.provider, tt.model, tt.inTok, tt.outTok, tt.powerMode, false)
			if !almostEqual(billed, tt.wantBill, 0.000001) {
				t.Errorf("BilledCost = %f, want %f (raw=%f × margin=1.5 × surcharge)", billed, tt.wantBill, raw)
			}
		})
	}
}

func TestBilledCost_MultiModelTask(t *testing.T) {
	// Simulates a build where 4 models each do work on the same project.
	// Total user cost = sum of each model's (API cost × margin × surcharge).
	e := newTestEngine()
	powerMode := "balanced" // surcharge = 1.12

	type call struct {
		provider string
		model    string
		inTok    int
		outTok   int
	}

	calls := []call{
		{"claude", "claude-opus-4-6", 5000, 3000},           // architect
		{"gpt4", "gpt-5", 8000, 4000},                       // coder
		{"gemini", "gemini-3-flash-preview", 10000, 5000},    // reviewer
		{"grok", "grok-4-fast", 3000, 1000},                  // quick check
	}

	var totalRaw, totalBilled float64
	for _, c := range calls {
		raw := e.RawCost(c.provider, c.model, c.inTok, c.outTok)
		billed := e.BilledCost(c.provider, c.model, c.inTok, c.outTok, powerMode, false)
		totalRaw += raw
		totalBilled += billed

		// Each call's billed cost must be raw × 1.5 × 1.1
		expectedBilled := roundUSD(raw * 1.5 * 1.12)
		if !almostEqual(billed, expectedBilled, 0.000001) {
			t.Errorf("%s/%s: billed=%f, want raw(%f) × 1.5 × 1.12 = %f",
				c.provider, c.model, billed, raw, expectedBilled)
		}
	}

	// Total billed must equal sum of all individual billed costs
	expectedTotal := roundUSD(totalRaw * 1.5 * 1.12)
	if !almostEqual(totalBilled, expectedTotal, 0.0001) {
		t.Errorf("total billed=%f, want %f (total raw=%f × 1.5 × 1.1)", totalBilled, expectedTotal, totalRaw)
	}

	// Verify the total is reasonable: raw should be less than billed
	if totalRaw >= totalBilled {
		t.Errorf("raw (%f) should be less than billed (%f)", totalRaw, totalBilled)
	}

	t.Logf("4-model task: raw=$%.6f, billed=$%.6f (%.0f%% total markup)",
		totalRaw, totalBilled, (totalBilled/totalRaw-1)*100)
}

func TestBilledCost_BYOKOnlyChargesRoutingFee(t *testing.T) {
	e := newTestEngine()

	// BYOK users pay the API directly, we only charge routing fee
	billed := e.BilledCost("claude", "claude-opus-4-6", 10000, 5000, "max", true)

	// 15000 total tokens × $0.25/1M = $0.00375
	expected := roundUSD(15000.0 / 1_000_000.0 * 0.25)
	if !almostEqual(billed, expected, 0.000001) {
		t.Errorf("BYOK BilledCost = %f, want %f (routing fee only)", billed, expected)
	}

	// Verify it's way less than what we'd charge non-BYOK
	nonByok := e.BilledCost("claude", "claude-opus-4-6", 10000, 5000, "max", false)
	if billed >= nonByok {
		t.Errorf("BYOK cost (%f) should be much less than non-BYOK (%f)", billed, nonByok)
	}
}

func TestBilledCost_OllamaIsFree(t *testing.T) {
	e := newTestEngine()
	billed := e.BilledCost("ollama", "llama3", 100000, 50000, "max", false)
	if billed != 0.0 {
		t.Errorf("Ollama should be free, got %f", billed)
	}
}

func TestPowerSurchargeScales(t *testing.T) {
	e := newTestEngine()

	fast := e.BilledCost("claude", "claude-opus-4-6", 1000, 500, "fast", false)
	balanced := e.BilledCost("claude", "claude-opus-4-6", 1000, 500, "balanced", false)
	max := e.BilledCost("claude", "claude-opus-4-6", 1000, 500, "max", false)

	if fast >= balanced {
		t.Errorf("fast (%f) should cost less than balanced (%f)", fast, balanced)
	}
	if balanced >= max {
		t.Errorf("balanced (%f) should cost less than max (%f)", balanced, max)
	}

	// Verify exact ratios
	if !almostEqual(balanced/fast, 1.12, 0.001) {
		t.Errorf("balanced/fast ratio = %f, want 1.12", balanced/fast)
	}
	if !almostEqual(max/fast, 1.25, 0.001) {
		t.Errorf("max/fast ratio = %f, want 1.25", max/fast)
	}
}

func TestProfitMargin(t *testing.T) {
	e := newTestEngine()

	raw := e.RawCost("claude", "claude-opus-4-6", 1000, 500)
	billed := e.BilledCost("claude", "claude-opus-4-6", 1000, 500, "fast", false)

	// Fast mode has 1.0 surcharge, so billed = raw × 1.5
	expectedMargin := 1.5
	actualMargin := billed / raw
	if !almostEqual(actualMargin, expectedMargin, 0.001) {
		t.Errorf("profit margin = %f, want %f", actualMargin, expectedMargin)
	}

	if e.ProfitMargin() != 1.5 {
		t.Errorf("ProfitMargin() = %f, want 1.5", e.ProfitMargin())
	}
}

func TestProviderNormalization(t *testing.T) {
	e := newTestEngine()

	// "openai" should resolve to "gpt4" provider pricing
	openaiRaw := e.RawCost("openai", "gpt-5", 1000, 1000)
	gpt4Raw := e.RawCost("gpt4", "gpt-5", 1000, 1000)
	if openaiRaw != gpt4Raw {
		t.Errorf("openai (%f) should equal gpt4 (%f)", openaiRaw, gpt4Raw)
	}
}

func TestNoLossGuarantee_BilledNeverLessThanRaw(t *testing.T) {
	// Even if someone misconfigures profitMargin or surcharges below 1.0,
	// we must never bill less than our API cost.

	// Create an engine with a broken margin (below 1.0 = losing money)
	broken := &Engine{
		providers: map[string]ProviderPricing{
			"claude": {
				Default: ModelPricing{InputPer1M: 3.00, OutputPer1M: 15.00},
				Models: map[string]ModelPricing{
					"claude-opus-4-6": {InputPer1M: 15.00, OutputPer1M: 75.00},
				},
			},
		},
		profitMargin: 0.50, // BROKEN: 50% discount = losing money
		powerSurcharges: map[string]float64{
			ModeFast:     1.00,
			ModeBalanced: 1.12,
			ModeMax:      1.25,
		},
		byokRoutingFeePer1M:  0.25,
		defaultPowerMode:     ModeFast,
		defaultMaxTokensHint: 2000,
	}

	raw := broken.RawCost("claude", "claude-opus-4-6", 1000, 500)
	billed := broken.BilledCost("claude", "claude-opus-4-6", 1000, 500, "fast", false)

	// Without the floor, billed would be raw * 0.5 = way less than raw.
	// The no-loss floor should clamp it to at least raw cost.
	if billed < raw {
		t.Errorf("NO-LOSS VIOLATION: billed=%f < raw=%f — we're losing money!", billed, raw)
	}
	if !almostEqual(billed, raw, 0.000001) {
		t.Errorf("expected billed to be clamped to raw=%f, got %f", raw, billed)
	}
}

func TestNoLossGuarantee_BrokenSurcharge(t *testing.T) {
	// Surcharge below 1.0 combined with margin could still lose money
	broken := &Engine{
		providers: map[string]ProviderPricing{
			"claude": {
				Default: ModelPricing{InputPer1M: 3.00, OutputPer1M: 15.00},
				Models: map[string]ModelPricing{
					"claude-opus-4-6": {InputPer1M: 15.00, OutputPer1M: 75.00},
				},
			},
		},
		profitMargin: 1.10, // 10% margin
		powerSurcharges: map[string]float64{
			ModeFast: 0.50, // BROKEN: surcharge cuts price in half
		},
		byokRoutingFeePer1M:  0.25,
		defaultPowerMode:     ModeFast,
		defaultMaxTokensHint: 2000,
	}

	raw := broken.RawCost("claude", "claude-opus-4-6", 5000, 3000)
	billed := broken.BilledCost("claude", "claude-opus-4-6", 5000, 3000, "fast", false)

	// 1.10 * 0.50 = 0.55 effective multiplier — losing money.
	// Floor should kick in.
	if billed < raw {
		t.Errorf("NO-LOSS VIOLATION: billed=%f < raw=%f — we're losing money!", billed, raw)
	}
}

func TestNoLossGuarantee_NormalConfigStillProfitable(t *testing.T) {
	// With normal config (margin=1.5, surcharges>=1.0), the floor should never activate.
	e := newTestEngine()

	providers := []struct{ p, m string }{
		{"claude", "claude-opus-4-6"},
		{"gpt4", "gpt-5"},
		{"gemini", "gemini-3-flash-preview"},
		{"grok", "grok-4-fast"},
	}

	for _, pp := range providers {
		for _, mode := range []string{"fast", "balanced", "max"} {
			raw := e.RawCost(pp.p, pp.m, 10000, 5000)
			billed := e.BilledCost(pp.p, pp.m, 10000, 5000, mode, false)
			if billed < raw {
				t.Errorf("%s/%s/%s: billed=%f < raw=%f", pp.p, pp.m, mode, billed, raw)
			}
			// With 1.5 margin and >=1.0 surcharge, profit should be at least 50%
			if raw > 0 && billed/raw < 1.5-0.001 {
				t.Errorf("%s/%s/%s: effective markup=%.2fx, want >= 1.5x", pp.p, pp.m, mode, billed/raw)
			}
		}
	}
}

func TestDefaultModel(t *testing.T) {
	e := newTestEngine()

	tests := []struct {
		provider  string
		powerMode string
		want      string
	}{
		{"claude", "fast", "claude-haiku-4-5-20251001"},
		{"claude", "max", "claude-opus-4-6"},
		{"gpt4", "fast", "gpt-4o-mini"},
		{"gpt4", "max", "gpt-5.2-codex"},
		{"gemini", "fast", "gemini-2.5-flash-lite"},
		{"grok", "fast", "grok-4-fast"},
	}

	for _, tt := range tests {
		t.Run(tt.provider+"/"+tt.powerMode, func(t *testing.T) {
			got := e.DefaultModel(tt.provider, tt.powerMode)
			if got != tt.want {
				t.Errorf("DefaultModel(%s, %s) = %s, want %s", tt.provider, tt.powerMode, got, tt.want)
			}
		})
	}
}
