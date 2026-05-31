# Lane L3 — Instant Start (audit 2026-05-16)

Source: Wave-1 Auditor. Key finding: `OnboardingTour.tsx` is complete but **never mounted in production** (dead). No blank-project path exists. GitHub deep-link URL is documented but unrouted.

| ID | Tier | Item | Anchor | Acceptance |
| --- | --- | --- | --- | --- |
| L3-01 | T0 | Mount OnboardingTour post-auth | App.tsx ~882 / OnboardingTour.tsx:110 | New user sees tour, returning users don't |
| L3-02 | T0 | "Blank Project" → IDE, no build | AppBuilder.tsx 7482-7535; backend `POST /projects/blank` | IDE+empty tree < 3s, no build prompt |
| L3-04 | T1 | Wire `/import/github.com/owner/repo` deep-link | App.tsx 126-168; GitHubImportWizard.tsx:585 | Deep link auto-opens import wizard prefilled |
| L3-05 | T1 | Template "Open instantly" (FE-only templates) | TemplateGallery.tsx 129-179 | Scaffold in Monaco < 5s, no build |
| L3-03 | T1 | First-run intent capture (describe/template/import) | App.tsx 1015-1432 | New user picks path, no "what now" |
| L3-06 | T2 | GitHub import in top nav | App.tsx 385-430 | Import icon visible all authed views |
| L3-07 | T2 | Contextual build tooltips (agent roles) | new BuildTooltip; BuildScreen.tsx | First build shows beacons, then gone |
| L3-08 | T2 | Free-tier guaranteed working preview | AppBuilder.tsx 6155-6169 | Free first build auto-opens preview |
| L3-09 | T3 | `/ide` no-project empty state | App.tsx 636-641; IDELayout.tsx | 3 CTAs instead of blank editor |
| L3-10 | T3 | time-to-first-working-build metric | AppBuilder.tsx 2150 + completion handlers | `time_to_first_preview` measured |
