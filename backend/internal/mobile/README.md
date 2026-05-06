# Mobile Builder Foundation

This package contains Apex Build's first-class mobile target foundation.

It is intentionally independent of web templates:

- `TargetPlatform` distinguishes web, full-stack web, Expo mobile, Capacitor mobile, and API-only targets.
- `MobileAppSpec` captures app identity, native platforms, architecture, capabilities, permissions, store metadata, and quality requirements.
- `ValidateMobileAppSpec` rejects broken app identifiers and missing native permission descriptions before generation.
- `ClassifyTargetPlatform` routes prompts with iOS/Android/APK/App Store/TestFlight/native capability signals to `mobile_expo`.
- `NativeCapabilityRegistry` is the dependency and permission allowlist for generated mobile apps.
- `GenerateExpoProject` creates the first Expo/React Native source package under `mobile/` with app config, EAS profiles, screens, API/auth/offline scaffolding, and release docs.
- `LoadFeatureFlagsFromEnv` keeps EAS/build/submit/store paths disabled until their credential and security models are implemented.

The current supported behavior is first-class mobile target metadata plus Expo source generation/validation. Do not claim native binary generation until the EAS provider, credential vault, validation, artifact, and store-readiness workflows are wired and tested.
