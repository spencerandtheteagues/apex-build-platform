# Mobile App Builder Architecture

## Current Architecture Summary

Apex Build is a Go backend plus React/Vite frontend platform. The backend API, agent orchestration, billing, websocket handling, build persistence, preview readiness, and export logic live under `backend/`. The frontend workspace, builder UI, admin panels, export UI, and API client live under `frontend/`.

The existing build path is centered on `backend/internal/agents`: a `BuildRequest` is normalized into an `IntentBrief`, a planner produces a `BuildPlan`, Apex freezes a `BuildContract`, validates it into a `ValidatedBuildSpec`, then dispatches role-owned work orders. Generated files are stored in live build state and terminal `CompletedBuild` snapshots. Completed builds can be linked into `models.Project` rows and exported.

The repo already had a generic template category named `mobile`, but no first-class Expo, React Native, Capacitor, EAS Build, or store-submission subsystem existed before this branch. `apex-build-mobile-client-prompt.md` is for a future Apex platform mobile client, not for generated customer mobile apps.

## Inspected Source Map

- Project creation and persistence: `backend/pkg/models/models.go`, `backend/internal/agents/manager.go`, `backend/internal/agents/handlers.go`, `backend/internal/db/database.go`, `backend/internal/handlers/projects.go`, `backend/internal/handlers/files.go`.
- AI prompt routing and orchestration: `backend/internal/agents/types.go`, `backend/internal/agents/orchestration_contracts.go`, `backend/internal/agents/build_spec.go`, `backend/internal/agents/validated_build_spec.go`, `backend/internal/agents/manager.go`, `backend/internal/agents/context_selector.go`.
- Template selection and scaffold hints: `backend/internal/templates/templates.go`, `backend/internal/agents/app_templates.go`, `backend/internal/agents/build_spec.go`.
- Generated file storage: live `agents.Build`, snapshot `models.CompletedBuild`, linked project files in `models.File`.
- Preview/build execution: `backend/internal/agents/preview_*`, `backend/internal/agents/build_test_artifacts.go`, readiness and validation paths in `backend/internal/agents/manager.go`.
- Export and GitHub export: `backend/internal/handlers/export.go`, `frontend/src/components/export/GitHubExportModal.tsx`.
- Billing and plan limits: `backend/internal/payments/plans.go`, `backend/internal/agents/entitlements.go`.
- Auth/secrets: `backend/pkg/models/models.go` `UserAPIKey`, `backend/internal/secrets`, BYOK handling in agent manager/router code.
- Frontend build UI/API client: `frontend/src/components/builder/AppBuilder.tsx`, `frontend/src/components/builder/OrchestrationOverview.tsx`, `frontend/src/services/api.ts`.
- Deployment config: root `docker-compose.yml`, `render.yaml`, `deploy.sh`, frontend runtime config.

## Proposed Mobile Architecture

Mobile generation is a first-class target path, not a web-template variant. The core discriminator is `target_platform`:

- `web`
- `fullstack_web`
- `mobile_expo`
- `mobile_capacitor`
- `api_only`

The first production-quality path is `mobile_expo` using Expo/React Native with TypeScript. Capacitor can later wrap existing web apps, but it must remain a separate target with honest limitations.

This branch adds the safe foundation:

- `backend/internal/mobile` defines mobile target enums, `MobileAppSpec`, validation, feature flags, target classification, and native capability allowlists.
- `BuildRequest`, `Build`, `BuildPlan`, `IntentBrief`, `BuildContract`, `ValidatedBuildSpec`, snapshots, `CompletedBuild`, and `Project` now carry optional mobile metadata.
- Existing web requests keep zero-value/default behavior and do not route through mobile generation unless the prompt or request explicitly asks for native mobile.
- All EAS/build/submit/store flags default off.

## Data Model Changes

`models.Project` now stores optional mobile metadata:

- target platform, mobile platforms, framework, release level, capabilities
- preview/build/store readiness statuses
- generated backend URL and generated mobile client path
- EAS project ID
- Android package and iOS bundle identifier
- app display name, version, build number/version code
- icon/splash references
- permission manifest and store metadata draft reference
- flexible `mobile_metadata`

`models.CompletedBuild` now stores a snapshot copy of mobile target metadata and `mobile_spec_json`. This allows history, restore, and project linking to preserve mobile intent without changing existing web snapshot semantics.

## New Backend Services

Implemented now:

- `MobileAppSpec` schema and validation.
- `TargetPlatformClassifier`.
- `NativeCapabilityRegistry`.
- mobile feature flags.
- contract propagation through orchestration and persistence.
- Expo/React Native source generator for a field-service quote-builder starter under `mobile/`.
- generated mobile source validation for dependency allowlist and browser-only runtime API usage.
- mobile source preparation for GitHub export and owner ZIP download.
- internal `MobileBuildService` abstraction with feature-flag/platform gating, mocked provider seam, restart-safe build-state records, artifact/log metadata, failure classification, and secret redaction.
- project-scoped mobile build-job API for request/status/log/artifact reads, gated by project ownership, mobile target type, feature flags, credentials, and provider availability.
- encrypted, project-scoped mobile credential vault for EAS, Apple App Store Connect, Google Play service accounts, and Android signing.
- authenticated mobile credential status API that returns metadata only and updates the mobile readiness scorecard.
- `EASBuildProvider` seam behind `MOBILE_EAS_BUILD_ENABLED`; it materializes stored project `mobile/` files into a temporary source directory, resolves the project-scoped EAS token from the encrypted vault, invokes `eas build --non-interactive --no-wait --json`, and refreshes status/artifacts through `eas build:view --json` with redacted logs.

Next backend services:

- generated mobile API client generator.
- Expo Web preview provider.
- automated polling workers, cancellation/retry endpoints, and repair-loop integration.
- store metadata/readiness generators.

## Frontend UI Components

Implemented now:

- `frontend/src/services/api.ts` accepts and reads optional mobile target metadata.
- `frontend/src/services/api.ts` exposes mobile credential status/create/delete methods with metadata-only response types.
- `frontend/src/services/api.ts` exposes project-scoped mobile build-job methods for request, status, refresh, logs, and artifact metadata.

Next frontend work:

- new-project target selector.
- mobile setup step with Android/iOS/capability/build-level controls.
- mobile preview frame with honest Expo Web labeling.
- build logs/artifacts panel.
- credentials panel.
- store-readiness checklist.
- mobile export options in `GitHubExportModal`.

## Agent Instructions

Mobile agent rules must be appended to generated mobile work orders before Expo generation is enabled:

- Use Expo/React Native and TypeScript for `mobile_expo`.
- Never use DOM APIs, `window`, `document`, `localStorage`, or browser-only packages in mobile client code.
- Use `expo-router` or the chosen navigation standard consistently.
- Use `SafeAreaProvider`.
- Store sensitive tokens with `expo-secure-store`; use AsyncStorage/SQLite only for non-sensitive local data.
- Keep server secrets out of mobile code.
- Add app config permission strings when native capabilities are present.
- Use only the native dependency allowlist unless a capability registry entry permits the package.
- Include loading, empty, error, and offline states for every data screen.

## Build Pipeline Flow

1. Classify prompt into target platform.
2. If `mobile_expo`, compile `MobileAppSpec`.
3. Validate identifiers, capabilities, permissions, dependencies, and backend/API contracts.
4. Generate backend/API where required.
5. Generate Expo/React Native source under `mobile/`.
6. Run source validation and Expo config checks.
7. Offer source export and GitHub export.
8. Offer Expo Web preview with clear "web-rendered mobile preview" labeling.
9. If enabled and credentials exist, queue EAS Build jobs.
10. Capture logs/artifacts and classify/repair failures.
11. Generate store-readiness package.
12. If enabled and credentials/store setup are valid, submit/upload artifacts.

## Credential Handling And Security

EAS Build, EAS Submit, Apple, Google Play, and Android signing credentials must use encrypted, scoped credential storage. Secrets must never be returned to the frontend after storage, embedded in generated mobile source, logged, or included in AI prompts. Missing credentials must produce actionable disabled states, not generic failures.

The current branch stores mobile credentials through `backend/internal/mobile.MobileCredentialVault` and exposes safe project-owner routes under `/api/v1/projects/:id/mobile/credentials`. Raw values are accepted only on create/update, stored through the encrypted secrets subsystem, and never returned in status responses. The readiness scorecard can now distinguish missing, partial, and validated mobile credential states.

The current branch also exposes native mobile build-job endpoints under `/api/v1/projects/:id/mobile/builds`. These endpoints fail closed unless mobile build feature flags are enabled, the requested platform is enabled for the project, the build credential requirement is complete, and a provider is configured. Build queueing currently requires the project-scoped EAS token. Store-readiness credential status remains broader: Android store release still tracks Google Play credentials and iOS store release tracks Apple/App Store Connect credentials. Public requests cannot choose arbitrary server source paths; the API materializes the project-owned stored `mobile/` files into a temporary build directory before provider execution.

When `MOBILE_EAS_BUILD_ENABLED=true`, startup wires `EASBuildProvider` with `EAS_CLI_PATH` (default `eas`), `MOBILE_EAS_BUILD_TIMEOUT` (default `30m`), and the encrypted mobile credential vault. The provider queues no-wait EAS builds and records the initial provider build ID/status/artifact URL when the CLI returns it. Project owners can explicitly refresh a stored build job; refresh calls `eas build:view --json`, updates status/artifact metadata, appends redacted logs, and updates project readiness metadata. The current branch does not yet run background polling, cancel provider jobs, retry provider jobs, or submit to stores.

The current branch still does not enable store submission. All EAS/build/submit/store feature flags default to off so production cannot accidentally claim or run mobile binary workflows before credentials, provider execution, and submission workflows are configured and validated.

## Store Readiness Workflow

Store readiness is separate from binary build success. Apex must track and display separate states for:

- source generated
- preview passed
- validation passed
- Android build passed
- iOS build passed
- store metadata ready
- uploaded/submitted
- approved/released

EAS Submit can upload binaries, but store listing metadata, screenshots, release notes, privacy/data-safety answers, and App Review/Play review completion must be tracked separately and never overclaimed.

## Rollout Strategy

1. Internal: source schema, routing, validation, export package structure.
2. Beta: Expo source generation, GitHub export, Expo Web preview.
3. Paid beta: Android internal APK via EAS.
4. Advanced beta: Android AAB, iOS simulator/internal builds.
5. Production: store-readiness package and optional submission helpers.

Feature flags:

- `MOBILE_BUILDER_ENABLED`
- `MOBILE_EXPO_ENABLED`
- `MOBILE_CAPACITOR_ENABLED`
- `MOBILE_EAS_BUILD_ENABLED`
- `MOBILE_EAS_SUBMIT_ENABLED`
- `MOBILE_STORE_METADATA_ENABLED`
- `MOBILE_IOS_BUILDS_ENABLED`
- `MOBILE_ANDROID_BUILDS_ENABLED`

## Acceptance Tests

Implemented now:

- Mobile spec validation accepts a realistic full-stack Expo spec.
- Invalid Android/iOS identifiers are rejected.
- iOS permission descriptions are required for native capabilities.
- Native dependency registry allows known Expo packages and rejects arbitrary native modules.
- Feature flags default safely off.
- Mobile prompt routes to `mobile_expo` without changing existing web/in-memory prompt behavior.
- Mobile metadata reaches `BuildContract` and `ValidatedBuildSpec`.

Required next:

- exported generated Expo template install/typecheck.
- generated mobile export clone/install.
- frontend mobile wizard tests.
- mocked mobile build-provider state and project-scoped API tests.
- credential redaction/encryption tests.
- project-owner mobile credential route tests.
- store-readiness generator tests.

## Known Limitations

- This branch generates Expo source files through `backend/internal/mobile.GenerateExpoProject` and prepares them for GitHub export and owner ZIP download.
- This branch has an internal mobile build-service abstraction, restart-safe build-job persistence, project-scoped build API, credential-gated EAS provider queue/refresh seam, and mocked provider tests.
- This branch does not yet run Expo Web preview.
- This branch can invoke EAS Build only when `MOBILE_EAS_BUILD_ENABLED` and the relevant platform flags are enabled and an EAS token is stored; this has not been live-validated against EAS in this session.
- This branch does not yet call EAS Submit.
- This branch does not yet automate App Store Connect or Google Play metadata.
- Public product copy must not claim native mobile binary generation until the EAS path is deployed, credential-gated, live-validated, and surfaced honestly in the UI.

## Official Reference Notes

- Expo documents EAS Build as the hosted service for Android/iOS binaries for Expo and React Native projects: https://docs.expo.dev/build/introduction/
- Expo development builds may need rebuilds when adding native-code libraries, so generated dependency policy must be strict: https://docs.expo.dev/develop/development-builds/use-development-builds/
- Expo EAS Submit uploads binaries, but does not by itself manage all store listing metadata and release materials: https://docs.expo.dev/submit/introduction/
- Android install/update artifacts must be signed, and Play distribution uses app signing/upload key workflows: https://developer.android.com/guide/publishing/app-signing.html
- Apple submission requires App Store Connect metadata and selecting the correct uploaded build before review: https://developer.apple.com/help/app-store-connect/manage-submissions-to-app-review/submit-an-app
