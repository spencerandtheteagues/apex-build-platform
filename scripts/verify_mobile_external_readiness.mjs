#!/usr/bin/env node

import { readFileSync } from 'node:fs'

const usage = `Usage:
  node scripts/verify_mobile_external_readiness.mjs

Strict native/store evidence check:
  APEX_MOBILE_EXPECT_NATIVE_READY=1 \\
  APEX_AUTH_TOKEN=... \\
  APEX_MOBILE_PROJECT_ID=123 \\
  node scripts/verify_mobile_external_readiness.mjs

Environment:
  APEX_API_URL                         API origin or /api/v1 base. Default: https://api.apex-build.dev
  APEX_MOBILE_CHECK_LIVE=1             Check live /api/v1/platform/truth.
  APEX_MOBILE_EXPECT_NATIVE_READY=1    Require project-level native build/store-upload evidence.
  APEX_AUTH_TOKEN                      Bearer token for project-level mobile evidence checks.
  APEX_MOBILE_PROJECT_ID               Mobile Expo project ID to verify.
`

if (process.argv.includes('--help') || process.argv.includes('-h')) {
  console.log(usage)
  process.exit(0)
}

const env = process.env
const boolEnv = (name) => env[name] === '1' || env[name]?.toLowerCase() === 'true'
const trim = (value) => (value || '').trim()
const expectNativeReady = boolEnv('APEX_MOBILE_EXPECT_NATIVE_READY')
const checkLive = expectNativeReady || boolEnv('APEX_MOBILE_CHECK_LIVE')
const authToken = trim(env.APEX_AUTH_TOKEN)
const projectID = trim(env.APEX_MOBILE_PROJECT_ID)
const failures = []
const notes = []

const ok = (message) => console.log(`[ok] ${message}`)
const note = (message) => {
  notes.push(message)
  console.log(`[note] ${message}`)
}
const fail = (message) => {
  failures.push(message)
  console.error(`[fail] ${message}`)
}

const normalizeTargets = (input) => {
  const configured = trim(input) || 'https://api.apex-build.dev'
  const withoutSlash = configured.replace(/\/+$/, '')
  if (withoutSlash.endsWith('/api/v1')) {
    return {
      apiOrigin: withoutSlash.slice(0, -'/api/v1'.length),
      apiV1Base: withoutSlash,
    }
  }
  return {
    apiOrigin: withoutSlash,
    apiV1Base: `${withoutSlash}/api/v1`,
  }
}

const { apiV1Base } = normalizeTargets(env.APEX_API_URL || env.PLAYWRIGHT_API_URL)

const truncate = (value, max = 600) => {
  const text = typeof value === 'string' ? value : JSON.stringify(value)
  return text.length > max ? `${text.slice(0, max)}...` : text
}

const readText = (path) => readFileSync(path, 'utf8')

const validateStaticTruth = () => {
  const platformTruth = readText('backend/internal/api/platform_truth.go')
  for (const snippet of [
    '{Key: "mobile_source_generation", Status: "flagged_beta", Source: "MOBILE_BUILDER_ENABLED"}',
    '{Key: "mobile_eas_builds", Status: "gated", Source: "MOBILE_EAS_BUILD_ENABLED"}',
    '{Key: "mobile_store_submission", Status: "gated", Source: "MOBILE_EAS_SUBMIT_ENABLED"}',
  ]) {
    if (!platformTruth.includes(snippet)) {
      fail(`platform truth is missing mobile status snippet: ${snippet}`)
    }
  }

  const flags = readText('backend/internal/mobile/flags.go')
  for (const snippet of [
    'envBool("MOBILE_EAS_BUILD_ENABLED", false)',
    'envBool("MOBILE_EAS_POLLING_ENABLED", false)',
    'envBool("MOBILE_EAS_SUBMIT_ENABLED", false)',
    'envBool("MOBILE_IOS_BUILDS_ENABLED", false)',
    'envBool("MOBILE_ANDROID_BUILDS_ENABLED", false)',
  ]) {
    if (!flags.includes(snippet)) {
      fail(`mobile feature flag default drifted: ${snippet}`)
    }
  }

  const render = readText('render.yaml')
  for (const key of [
    'MOBILE_EAS_BUILD_ENABLED',
    'MOBILE_EAS_SUBMIT_ENABLED',
    'MOBILE_IOS_BUILDS_ENABLED',
    'MOBILE_ANDROID_BUILDS_ENABLED',
  ]) {
    if (new RegExp(`key:\\s*${key}[\\s\\S]{0,80}value:\\s*"?true"?`).test(render)) {
      fail(`render.yaml enables ${key}; native/store mobile launch must remain gated until live evidence exists`)
    }
  }

  ok('mobile native/store feature flags remain default-off and platform truth remains gated')
}

const requestJSON = async (url, options = {}) => {
  const response = await fetch(url, {
    ...options,
    headers: {
      accept: 'application/json',
      ...(options.headers || {}),
    },
  })
  const text = await response.text()
  let body = null
  if (text) {
    try {
      body = JSON.parse(text)
    } catch {
      throw new Error(`${url} returned non-JSON ${response.status}: ${truncate(text)}`)
    }
  }
  if (!response.ok) {
    throw new Error(`${url} returned ${response.status}: ${truncate(body || text)}`)
  }
  return body
}

const checkLivePlatformTruth = async () => {
  if (!checkLive) {
    note('live mobile platform-truth check skipped; set APEX_MOBILE_CHECK_LIVE=1 or APEX_MOBILE_EXPECT_NATIVE_READY=1')
    return
  }
  const body = await requestJSON(`${apiV1Base}/platform/truth`)
  const features = new Map((body.features || []).map((feature) => [feature.key, feature]))
  const expected = {
    mobile_source_generation: 'flagged_beta',
    mobile_eas_builds: 'gated',
    mobile_store_submission: 'gated',
  }
  for (const [key, status] of Object.entries(expected)) {
    const feature = features.get(key)
    if (feature?.status !== status) {
      fail(`live platform truth ${key} status = ${feature?.status || 'missing'}, want ${status}`)
    } else {
      ok(`live platform truth keeps ${key} as ${status}`)
    }
  }
}

const projectHeaders = () => ({ authorization: `Bearer ${authToken}` })

const fetchProjectEvidence = async () => {
  if (!authToken || !projectID) {
    const message = 'project-level mobile evidence skipped; set APEX_AUTH_TOKEN and APEX_MOBILE_PROJECT_ID'
    if (expectNativeReady) fail(message)
    else note(message)
    return null
  }

  const [credentials, storeReadiness, builds, submissions] = await Promise.all([
    requestJSON(`${apiV1Base}/projects/${encodeURIComponent(projectID)}/mobile/credentials`, { headers: projectHeaders() }),
    requestJSON(`${apiV1Base}/projects/${encodeURIComponent(projectID)}/mobile/store-readiness`, { headers: projectHeaders() }),
    requestJSON(`${apiV1Base}/projects/${encodeURIComponent(projectID)}/mobile/builds`, { headers: projectHeaders() }),
    requestJSON(`${apiV1Base}/projects/${encodeURIComponent(projectID)}/mobile/submissions`, { headers: projectHeaders() }),
  ])

  ok(`loaded project-level mobile evidence for project ${projectID}`)
  return {
    credentials: credentials.credentials,
    storeReadiness: storeReadiness.store_readiness,
    builds: builds.builds || [],
    submissions: submissions.submissions || [],
  }
}

const verifyProjectEvidence = (evidence) => {
  if (!evidence) return

  if (evidence.credentials?.complete === true) {
    ok('mobile credential status is complete')
  } else {
    const missing = evidence.credentials?.missing?.join(', ') || 'unknown'
    fail(`mobile credential status is not complete; missing ${missing}`)
  }

  if (evidence.storeReadiness?.ready_for_submission === true) {
    ok('mobile store-readiness report is ready for submission workflow')
  } else {
    fail(`mobile store-readiness is not submission-ready: ${truncate(evidence.storeReadiness)}`)
  }

  const succeededNativeBuild = evidence.builds.find((build) =>
    build?.status === 'succeeded' &&
    ['android', 'ios'].includes(build?.platform) &&
    (trim(build?.artifact_url) || trim(build?.provider_build_id))
  )
  if (succeededNativeBuild) {
    ok(`found succeeded native ${succeededNativeBuild.platform} build with provider/artifact evidence`)
  } else {
    fail('no succeeded native Android/iOS build with provider/artifact evidence found')
  }

  const acceptedSubmissionStatuses = new Set([
    'completed_upload',
    'submitted_to_store_pipeline',
    'ready_for_testflight',
    'ready_for_google_internal_testing',
    'requires_manual_review_submission',
  ])
  const submission = evidence.submissions.find((candidate) => acceptedSubmissionStatuses.has(candidate?.status))
  if (submission) {
    ok(`found store-upload submission evidence with status ${submission.status}`)
  } else {
    fail('no completed or active store-upload submission evidence found')
  }
}

try {
  validateStaticTruth()
  await checkLivePlatformTruth()
  const evidence = await fetchProjectEvidence()
  if (expectNativeReady) {
    verifyProjectEvidence(evidence)
  } else if (evidence) {
    note('project evidence was loaded but strict native readiness assertions are off; set APEX_MOBILE_EXPECT_NATIVE_READY=1')
  }
} catch (error) {
  fail(error instanceof Error ? error.message : String(error))
}

if (failures.length > 0) {
  console.error(`\nMobile external readiness verification failed with ${failures.length} issue(s).`)
  process.exit(1)
}

console.log('\nMobile external readiness verification completed.')
if (notes.length > 0) {
  console.log('Notes remain; strict native/store evidence still requires real project credentials and provider history.')
}
