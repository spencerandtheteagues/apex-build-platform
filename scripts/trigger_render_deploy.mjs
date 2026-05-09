#!/usr/bin/env node

const usage = `Usage:
  RENDER_API_KEY=... \\
  RENDER_BACKEND_SERVICE_ID=... \\
  RENDER_FRONTEND_SERVICE_ID=... \\
  node scripts/trigger_render_deploy.mjs

Optional wait/verify:
  APEX_RENDER_WAIT_DEPLOY=1 \\
  APEX_RENDER_EXPECT_LAUNCH_READY=1 \\
  RENDER_API_KEY=... \\
  RENDER_BACKEND_SERVICE_ID=... \\
  RENDER_FRONTEND_SERVICE_ID=... \\
  node scripts/trigger_render_deploy.mjs

Environment:
  RENDER_API_KEY or RENDER_TOKEN       Render API bearer token. Values are never printed.
  RENDER_BACKEND_SERVICE_ID            Render service ID for apex-api.
  RENDER_FRONTEND_SERVICE_ID           Render service ID for apex-frontend.
  APEX_RENDER_DEPLOY_BACKEND=0         Skip backend deploy trigger.
  APEX_RENDER_DEPLOY_FRONTEND=0        Skip frontend deploy trigger.
  APEX_RENDER_CLEAR_CACHE=clear|do_not_clear
                                       Render deploy cache mode. Default: do_not_clear.
  APEX_RENDER_WAIT_DEPLOY=1            Poll Render deploy status until live/failed.
  APEX_RENDER_WAIT_TIMEOUT_SECONDS=900 Max deploy wait time. Default: 900.
  APEX_RENDER_WAIT_INTERVAL_SECONDS=15 Poll interval. Default: 15.
  APEX_RENDER_EXPECT_LAUNCH_READY=1    After deploy wait, require public launch-ready health.
  APEX_API_URL                         API origin or /api/v1 base. Default: https://api.apex-build.dev.
  APEX_RENDER_API_BASE                 Render API base. Default: https://api.render.com/v1.
`

if (process.argv.includes('--help') || process.argv.includes('-h')) {
  console.log(usage)
  process.exit(0)
}

const env = process.env
const trim = (value) => (value || '').trim()
const boolEnv = (name) => env[name] === '1' || env[name]?.toLowerCase() === 'true'
const falseEnv = (name) => env[name] === '0' || env[name]?.toLowerCase() === 'false'

const renderToken = trim(env.RENDER_API_KEY) || trim(env.RENDER_TOKEN)
const backendServiceID = trim(env.RENDER_BACKEND_SERVICE_ID)
const frontendServiceID = trim(env.RENDER_FRONTEND_SERVICE_ID)
const renderAPIBase = trim(env.APEX_RENDER_API_BASE) || 'https://api.render.com/v1'
const clearCache = trim(env.APEX_RENDER_CLEAR_CACHE) || 'do_not_clear'
const deployBackend = !falseEnv('APEX_RENDER_DEPLOY_BACKEND')
const deployFrontend = !falseEnv('APEX_RENDER_DEPLOY_FRONTEND')
const waitDeploy = boolEnv('APEX_RENDER_WAIT_DEPLOY') || boolEnv('APEX_RENDER_EXPECT_LAUNCH_READY')
const expectLaunchReady = boolEnv('APEX_RENDER_EXPECT_LAUNCH_READY')
const waitTimeoutSeconds = Number.parseInt(trim(env.APEX_RENDER_WAIT_TIMEOUT_SECONDS) || '900', 10)
const waitIntervalSeconds = Number.parseInt(trim(env.APEX_RENDER_WAIT_INTERVAL_SECONDS) || '15', 10)

const failures = []
const triggered = []

const fail = (message) => {
  failures.push(message)
  console.error(`[fail] ${message}`)
}

const ok = (message) => console.log(`[ok] ${message}`)
const note = (message) => console.log(`[note] ${message}`)
const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms))

const truncate = (value, max = 700) => {
  const text = typeof value === 'string' ? value : JSON.stringify(value)
  return text.length > max ? `${text.slice(0, max)}...` : text
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

const { apiOrigin, apiV1Base } = normalizeTargets(env.APEX_API_URL)

const parseJSONResponse = async (response, label) => {
  const text = await response.text()
  if (!text) return null
  try {
    return JSON.parse(text)
  } catch {
    throw new Error(`${label} returned non-JSON ${response.status}: ${truncate(text)}`)
  }
}

const renderRequest = async (path, options = {}) => {
  const response = await fetch(`${renderAPIBase}${path}`, {
    ...options,
    headers: {
      accept: 'application/json',
      authorization: `Bearer ${renderToken}`,
      ...(options.body ? { 'content-type': 'application/json' } : {}),
      ...(options.headers || {}),
    },
  })
  const body = await parseJSONResponse(response, `${options.method || 'GET'} ${path}`)
  if (!response.ok) {
    throw new Error(`${options.method || 'GET'} ${path} returned ${response.status}: ${truncate(body)}`)
  }
  return body
}

const publicJSON = async (url) => {
  const response = await fetch(url, { headers: { accept: 'application/json' } })
  const body = await parseJSONResponse(response, `GET ${url}`)
  if (!response.ok) {
    throw new Error(`GET ${url} returned ${response.status}: ${truncate(body)}`)
  }
  return body
}

const extractDeploy = (body) => body?.deploy || body
const deployID = (deploy) => trim(deploy?.id) || trim(deploy?.deploy?.id)
const deployStatus = (deploy) => trim(deploy?.status) || trim(deploy?.deploy?.status)

const triggerDeploy = async (label, serviceID) => {
  if (!serviceID) {
    fail(`${label} service ID is missing`)
    return
  }
  const body = await renderRequest(`/services/${encodeURIComponent(serviceID)}/deploys`, {
    method: 'POST',
    body: JSON.stringify({ clearCache }),
  })
  const deploy = extractDeploy(body)
  const id = deployID(deploy)
  const status = deployStatus(deploy) || 'created'
  ok(`triggered ${label} deploy${id ? ` ${id}` : ''} (${status})`)
  triggered.push({ label, serviceID, id })
}

const readDeploy = async ({ serviceID, id }) => {
  if (id) {
    try {
      return extractDeploy(await renderRequest(`/services/${encodeURIComponent(serviceID)}/deploys/${encodeURIComponent(id)}`))
    } catch (error) {
      note(`direct deploy status lookup failed; falling back to latest deploy list: ${error.message}`)
    }
  }

  const list = await renderRequest(`/services/${encodeURIComponent(serviceID)}/deploys?limit=1`)
  const deploys = Array.isArray(list) ? list : Array.isArray(list?.deploys) ? list.deploys : []
  return extractDeploy(deploys[0])
}

const waitForDeploy = async (item) => {
  const deadline = Date.now() + waitTimeoutSeconds * 1000
  const failedStatuses = new Set(['build_failed', 'update_failed', 'canceled', 'deactivated'])
  let lastStatus = ''

  while (Date.now() < deadline) {
    const deploy = await readDeploy(item)
    const status = deployStatus(deploy) || 'unknown'
    if (status !== lastStatus) {
      note(`${item.label} deploy status: ${status}`)
      lastStatus = status
    }
    if (status === 'live') {
      ok(`${item.label} deploy is live`)
      return
    }
    if (failedStatuses.has(status)) {
      throw new Error(`${item.label} deploy ended with ${status}`)
    }
    await sleep(waitIntervalSeconds * 1000)
  }

  throw new Error(`${item.label} deploy did not become live within ${waitTimeoutSeconds}s`)
}

const readinessService = (body, name) => {
  const services = Array.isArray(body?.services) ? body.services : []
  return services.find((service) => service?.name === name)
}

const verifyPublicLaunchReadiness = async () => {
  const health = await publicJSON(`${apiOrigin}/health`)
  if (health.status !== 'healthy' || health.ready !== true) {
    throw new Error(`/health is not ready: ${truncate(health)}`)
  }
  ok('/health is healthy and ready')

  const platformTruth = await publicJSON(`${apiV1Base}/platform/truth`)
  if (!Array.isArray(platformTruth?.features)) {
    throw new Error('/platform/truth did not return features')
  }
  ok('/platform/truth is available')

  const features = await publicJSON(`${apiOrigin}/health/features`)
  for (const name of ['code_execution', 'preview_service']) {
    const service = readinessService(features, name)
    if (service?.details?.launch_ready !== true) {
      throw new Error(`${name}.details.launch_ready is not true: ${truncate(service)}`)
    }
    ok(`${name}.details.launch_ready is true`)
  }

  const runtimeVerify = readinessService(features, 'preview_runtime_verify')
  if (runtimeVerify?.state !== 'ready' || runtimeVerify?.details?.browser_proof !== true) {
    throw new Error(`preview_runtime_verify is not browser-proof ready: ${truncate(runtimeVerify)}`)
  }
  ok('preview_runtime_verify is browser-proof ready')
}

try {
  if (!renderToken) {
    throw new Error('RENDER_API_KEY or RENDER_TOKEN is required')
  }
  if (!['clear', 'do_not_clear'].includes(clearCache)) {
    throw new Error('APEX_RENDER_CLEAR_CACHE must be clear or do_not_clear')
  }
  if (!Number.isFinite(waitTimeoutSeconds) || waitTimeoutSeconds < 30) {
    throw new Error('APEX_RENDER_WAIT_TIMEOUT_SECONDS must be at least 30')
  }
  if (!Number.isFinite(waitIntervalSeconds) || waitIntervalSeconds < 5) {
    throw new Error('APEX_RENDER_WAIT_INTERVAL_SECONDS must be at least 5')
  }

  if (deployBackend) {
    await triggerDeploy('backend', backendServiceID)
  }
  if (deployFrontend) {
    await triggerDeploy('frontend', frontendServiceID)
  }
  if (triggered.length === 0) {
    throw new Error('no deploys requested; both backend and frontend deploy triggers are disabled')
  }

  if (waitDeploy) {
    for (const item of triggered) {
      await waitForDeploy(item)
    }
  } else {
    note('deploy status wait skipped; set APEX_RENDER_WAIT_DEPLOY=1 to poll Render until live')
  }

  if (expectLaunchReady) {
    await verifyPublicLaunchReadiness()
  } else {
    note('public launch-ready verification skipped; set APEX_RENDER_EXPECT_LAUNCH_READY=1 after deploy')
  }
} catch (error) {
  fail(error instanceof Error ? error.message : String(error))
}

if (failures.length > 0) {
  console.error(`\nRender deploy trigger failed with ${failures.length} issue(s).`)
  process.exit(1)
}

console.log('\nRender deploy trigger completed.')
