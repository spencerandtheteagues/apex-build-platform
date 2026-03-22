# Theme Parity Report

## Current state

The product primarily presents a black/red flagship theme with a blue light-mode override in `frontend/src/styles/globals.css`.

## Strengths

- The blue theme already overrides most common dark utility classes, so core readability holds up.
- Border, shadow, and glow tokens exist for both theme directions.
- The orchestration and billing polish in this pass stayed inside the established visual language instead of introducing a third competing style.

## Risks

- Some dense surfaces still rely on hardcoded dark utility combinations that are only corrected by broad CSS remapping.
- The dark theme feels more intentional than the blue theme because many components are authored as dark-first surfaces.
- Inline-styled marketing sections can drift from theme parity more easily than token-driven components.

## This pass

- Kept the flagship dark look intact while improving hierarchy, spacing, and trust.
- Used border/shadow layering that remains legible when the blue theme remaps dark utility classes.
- Avoided one-off colors that would weaken blue-theme parity.

## Recommended next parity pass

1. Move more dense builder and billing surfaces onto token-driven utility helpers instead of raw dark color literals.
2. Audit the landing page separately, since inline styles there bypass a lot of theme-token leverage.
3. Add visual regression checks for the blue theme on the billing and builder surfaces.
