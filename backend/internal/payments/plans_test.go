package payments

import "testing"

func TestUpdatedPlanPricing(t *testing.T) {
	tests := []struct {
		planType       PlanType
		monthlyCents   int64
		monthlyCredits float64
	}{
		{PlanBuilder, 2400, 12},
		{PlanPro, 5900, 40},
		{PlanTeam, 14900, 110},
	}

	for _, tc := range tests {
		t.Run(string(tc.planType), func(t *testing.T) {
			plan := GetPlanByType(tc.planType)
			if plan == nil {
				t.Fatalf("expected plan %s", tc.planType)
			}
			if plan.MonthlyPriceCents != tc.monthlyCents {
				t.Fatalf("MonthlyPriceCents = %d, want %d", plan.MonthlyPriceCents, tc.monthlyCents)
			}
			if plan.MonthlyCreditsUSD != tc.monthlyCredits {
				t.Fatalf("MonthlyCreditsUSD = %v, want %v", plan.MonthlyCreditsUSD, tc.monthlyCredits)
			}
		})
	}
}

func TestCreditPacksMatchPricingStructure(t *testing.T) {
	packs := CreditPacks()
	if len(packs) != 4 {
		t.Fatalf("expected 4 credit packs, got %d", len(packs))
	}

	want := []int64{25, 50, 100, 250}
	for i, amount := range want {
		if packs[i].AmountUSD != amount {
			t.Fatalf("pack %d amount = %d, want %d", i, packs[i].AmountUSD, amount)
		}
		if packs[i].CreditUSD != float64(amount) {
			t.Fatalf("pack %d credit value = %v, want %v", i, packs[i].CreditUSD, float64(amount))
		}
	}
}
