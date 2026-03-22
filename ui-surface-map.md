# UI Surface Map

## Primary product surfaces

- `frontend/src/pages/Landing.tsx`
  - public value proposition, provider positioning, pricing ladder, free-vs-paid messaging
- `frontend/src/App.tsx`
  - auth entry, signup assent, plan-aware onboarding seam
- `frontend/src/components/builder/AppBuilder.tsx`
  - build launcher, upgrade-required handling, IDE shell, orchestration entry point
- `frontend/src/components/builder/OrchestrationOverview.tsx`
  - timeline, truth tags, blockers, approvals, journal, architecture, readiness, provider/repair signals
- `frontend/src/components/builder/ModelRoleConfig.tsx`
  - hosted provider selection and Ollama/BYOK positioning

## Billing and monetization surfaces

- `frontend/src/components/billing/BillingSettings.tsx`
  - plan state, entitlement messaging, invoices, subscription management
- `frontend/src/components/billing/BuyCreditsModal.tsx`
  - credit-pack purchase and top-up truth
- `frontend/src/components/usage/UsageDashboard.tsx`
  - spend and usage visibility

## Trust and policy surfaces

- `frontend/src/components/settings/LegalDocuments.tsx`
  - terms, privacy, policy center
- `frontend/src/components/help/HelpCenter.tsx`
  - user-facing capability and plan guidance

## Theme and system surfaces

- `frontend/src/styles/globals.css`
  - root theme tokens and blue-theme remapping
- `frontend/src/styles/themes.ts`
  - legacy theme set and effect definitions

## Highest-traffic surfaces for revenue and trust

1. `Landing.tsx`
2. `App.tsx`
3. `AppBuilder.tsx`
4. `OrchestrationOverview.tsx`
5. `BillingSettings.tsx`
6. `BuyCreditsModal.tsx`
