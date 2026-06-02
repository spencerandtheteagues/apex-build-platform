#!/usr/bin/env node

const usage = `Usage:
  RENDER_API_KEY=... \\
  RENDER_BACKEND_SERVICE_ID=... \\
  node scripts/run_render_rollback_drill.mjs

Execute the rollback + roll-forward drill:
  RENDER_API_KEY=... \\
  RENDER_BACKEND_SERVICE_ID=... \\
  APEX_RENDER_ROLLBACK_EXECUTE=1 \\
  APEX_RENDER_CONFIRM_ROLLBACK_DEPLOY_ID=dep_previous \\
  APEX_RENDER_CONFIRM_ROLL_FORWARD_DEPLOY_ID=dep_current \\
  node scripts/run_render_rollback_drill.mjs

Environment:
  RENDER_API_KEY or RENDER_TOKEN              Render API bearer token. Values are never printed.
  RENDER_BACKEND_SERVICE_ID                   Render backend service ID. Required.
  APEX_RENDER_API_BASE                        Render API base. Default: https://api.render.com/v1.
  APEX_RENDER_ROLLBACK_DEPLOY_ID              Optional explicit deploy ID to roll back to.
  APEX_RENDER_CONFIRM_ROLLBACK_DEPLOY_ID      Must exactly match the rollback target when executing.
  APEX_RENDER_CONFIRM_ROLL_FORWARD_DEPLOY_ID  Must exactly match the original current deploy when executing.
  APEX_RENDER_ROLLBACK_EXECUTE=1              Execute the drill. Default is dry-run only.
  APEX_RENDER_WAIT_TIMEOUT_SECONDS            Wait timeout per rollback step. Default: 600.
  APEX_RENDER_WAIT_INTERVAL_SECONDS           Poll interval. Default: 10.
  APEX_RENDER_SKIP_HEALTH_CHECK=1             Skip public health checks after each step.
  APEX_API_URL                                API origin or /api/v1 base. Default: https://api.apex-build.dev.

This script uses Render's current rollback endpoint:
  POST /v1/services/{serviceId}/rollback with body {"deployId":"..."}.
It does not use deploy-trigger or commit placeholder endpoints.
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
const serviceID = trim(env.RENDER_BACKEND_SERVICE_ID) || trim(env.APEX_RENDER_SERVICE_ID)
const renderAPIBase = trim(env.APEX_RENDER_API_BASE) || 'https://api.render.com/v1'
const explicitRollbackID = trim(env.APEX_RENDER_ROLLBACK_DEPLOY_ID)
const confirmRollbackID = trim(env.APEX_RENDER_CONFIRM_ROLLBACK_DEPLOY_ID)
const confirmRollForwardID = trim(env.APEX_RENDER_CONFIRM_ROLL_FORWARD_DEPLOY_ID)
const execute = boolEnv('APEX_RENDER_ROLLBACK_EXECUTE') || process.argv.includes('--execute')
const skipHealthCheck = boolEnv('APEX_RENDER_SKIP_HEALTH_CHECK')
const waitTimeoutSeconds = Number.parseInt(trim(env.APEX_RENDER_WAIT_TIMEOUT_SECONDS) || '600', 10)
const waitIntervalSeconds = Number.parseInt(trim(env.APEX_RENDER_WAIT_INTERVAL_SECONDS) || '10', 10)

const failures = []
const ok = (message) => console.log(`[ok] ${message}`)
const note = (message) => console.log(`[note] ${message}`)
const fail = (message) => {
  failures.push(message)
  console.error(`[fail] ${message}`)
}
const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms))

const truncate = (value, max = 700) => {
  const text = typeof value === 'string' ? value : JSON.stringify(value)
  return text.length > max ? `${text.slice(0, max)}...` : text
}

const normalizeTargets = (input) => {
  const configured = trim(input) || 'https://api.apex-build.dev'
  const withoutSlash = configured.replace(/\/+$/, '')
  if (withoutSlash.endsWith('/api/v1')) {
    return { apiOrigin: withoutSlash.slice(0, -'/api/v1'.length) }
  }
  return { apiOrigin: withoutSlash }
}

const { apiOrigin } = normalizeTargets(env.APEX_API_URL)

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

const extractDeploy = (item) => item?.deploy || item
const deployID = (deploy) => trim(deploy?.id)
const deployStatus = (deploy) => trim(deploy?.status)
const deployCreatedAt = (deploy) => trim(deploy?.createdAt)

const listDeploys = async () => {
  const response = await renderRequest(`/services/${encodeURIComponent(serviceID)}/deploys?limit=20`)
  const items = Array.isArray(response) ? response : Array.isArray(response?.deploys) ? response.deploys : []
  return items.map(extractDeploy).filter((deploy) => deployID(deploy))
}

const selectTargets = (deploys) => {
  const current = deploys.find((deploy) => deployStatus(deploy) === 'live') || deploys[0]
  if (!current) throw new Error('no deploys were returned for this service')

  const successfulStatuses = new Set(['live', 'deactivated', 'build_complete', 'update_complete'])
  const rollbackTarget = explicitRollbackID
    ? deploys.find((deploy) => deployID(deploy) === explicitRollbackID)
    : deploys.find((deploy) => deployID(deploy) !== deployID(current) && successfulStatuses.has(deployStatus(deploy)))

  if (!rollbackTarget) {
    throw new Error(explicitRollbackID
      ? `explicit rollback deploy ${explicitRollbackID} was not found in recent deploys`
      : 'could not find a previous successful deploy to roll back to')
  }

  return { current, rollbackTarget }
}

const assertExecutionConfirmations = ({ current, rollbackTarget }) => {
  if (!execute) return

  if (confirmRollbackID !== deployID(rollbackTarget)) {
    fail(`APEX_RENDER_CONFIRM_ROLLBACK_DEPLOY_ID must exactly match rollback target ${deployID(rollbackTarget)}`)
  }
  if (confirmRollForwardID !== deployID(current)) {
    fail(`APEX_RENDER_CONFIRM_ROLL_FORWARD_DEPLOY_ID must exactly match roll-forward target ${deployID(current)}`)
  }
  if (failures.length > 0) {
    throw new Error('refusing to execute rollback drill without exact deploy-id confirmations')
  }
}

const rollbackToDeploy = async (label, deploy) => {
  const id = deployID(deploy)
  const body = await renderRequest(`/services/${encodeURIComponent(serviceID)}/rollback`, {
    method: 'POST',
    body: JSON.stringify({ deployId: id }),
  })
  const newDeploy = extractDeploy(body)
  const newID = deployID(newDeploy)
  if (!newID) {
    throw new Error(`${label} rollback response did not include a deploy id: ${truncate(body)}`)
  }
  ok(`${label} rollback requested to ${id}${newID ? `; Render created deploy ${newID}` : ''}`)
  return { ...newDeploy, id: newID }
}

const readDeploy = async (id) => {
  return extractDeploy(await renderRequest(`/services/${encodeURIComponent(serviceID)}/deploys/${encodeURIComponent(id)}`))
}

const waitForLive = async (label, deploy) => {
  const id = deployID(deploy)
  const deadline = Date.now() + waitTimeoutSeconds * 1000
  const failedStatuses = new Set(['build_failed', 'update_failed', 'canceled'])
  let lastStatus = ''

  while (Date.now() < deadline) {
    const current = await readDeploy(id)
    const status = deployStatus(current) || 'unknown'
    if (status !== lastStatus) {
      note(`${label} deploy ${id} status: ${status}`)
      lastStatus = status
    }
    if (status === 'live') {
      ok(`${label} deploy ${id} is live`)
      return
    }
    if (failedStatuses.has(status)) {
      throw new Error(`${label} deploy ${id} ended with ${status}`)
    }
    await sleep(waitIntervalSeconds * 1000)
  }

  throw new Error(`${label} deploy ${id} did not become live within ${waitTimeoutSeconds}s`)
}

const verifyPublicHealth = async (label) => {
  if (skipHealthCheck) {
    note(`${label} public health check skipped by APEX_RENDER_SKIP_HEALTH_CHECK=1`)
    return
  }

  const ready = await publicJSON(`${apiOrigin}/ready`)
  if (ready.ready !== true || ready.startup?.ready !== true) {
    throw new Error(`${label}: /ready is not ready: ${truncate(ready)}`)
  }
  const features = await publicJSON(`${apiOrigin}/health/features`)
  const services = Array.isArray(features?.services) ? features.services : []
  for (const name of ['code_execution', 'preview_service']) {
    const service = services.find((candidate) => candidate?.name === name)
    if (service?.details?.launch_ready !== true) {
      throw new Error(`${label}: ${name}.details.launch_ready is not true: ${truncate(service)}`)
    }
  }
  ok(`${label}: /ready and launch-critical health/features are ready`)
}

try {
  if (!renderToken) throw new Error('RENDER_API_KEY or RENDER_TOKEN is required')
  if (!serviceID) throw new Error('RENDER_BACKEND_SERVICE_ID or APEX_RENDER_SERVICE_ID is required')

  const deploys = await listDeploys()
  const { current, rollbackTarget } = selectTargets(deploys)

  console.log('Render rollback drill target summary:')
  console.log(`  service_id:        ${serviceID}`)
  console.log(`  current_live:      ${deployID(current)} status=${deployStatus(current) || '?'} created=${deployCreatedAt(current) || '?'}`)
  console.log(`  rollback_target:   ${deployID(rollbackTarget)} status=${deployStatus(rollbackTarget) || '?'} created=${deployCreatedAt(rollbackTarget) || '?'}`)
  console.log(`  rollback_endpoint: POST ${renderAPIBase}/services/${serviceID}/rollback`)

  if (!execute) {
    note('dry run only; set APEX_RENDER_ROLLBACK_EXECUTE=1 plus exact confirmation env vars to execute rollback and roll-forward')
    console.log(`  required confirmation: APEX_RENDER_CONFIRM_ROLLBACK_DEPLOY_ID=${deployID(rollbackTarget)}`)
    console.log(`  required confirmation: APEX_RENDER_CONFIRM_ROLL_FORWARD_DEPLOY_ID=${deployID(current)}`)
    process.exit(0)
  }

  assertExecutionConfirmations({ current, rollbackTarget })
  const rollbackDeploy = await rollbackToDeploy('rollback', rollbackTarget)
  await waitForLive('rollback', rollbackDeploy)
  await verifyPublicHealth('after rollback')

  const rollForwardDeploy = await rollbackToDeploy('roll-forward', current)
  await waitForLive('roll-forward', rollForwardDeploy)
  await verifyPublicHealth('after roll-forward')
  ok('Render rollback/roll-forward drill completed')
} catch (error) {
  fail(error.message)
}

if (failures.length > 0) {
  console.error(`Render rollback drill failed with ${failures.length} issue(s).`)
  process.exit(1)
}
