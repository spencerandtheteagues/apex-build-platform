#!/usr/bin/env node

import { execFileSync } from 'node:child_process'

const usage = `Usage:
  node scripts/verify_render_launch_env.mjs

Strict production check:
  APEX_RENDER_EXPECT_PRODUCTION=1 \\
  RENDER_API_KEY=... \\
  RENDER_BACKEND_SERVICE_ID=... \\
  RENDER_FRONTEND_SERVICE_ID=... \\
  node scripts/verify_render_launch_env.mjs

Environment:
  APEX_RENDER_BLUEPRINT            Render blueprint path. Default: render.yaml
  APEX_RENDER_EXPECT_PRODUCTION=1  Fail unless blueprint, Render env vars, and live health are launch-ready.
  APEX_RENDER_CHECK_LIVE=1         Check /health and /health/features without requiring Render API credentials.
  APEX_API_URL                     API origin or /api/v1 base. Default: https://api.apex-build.dev
  RENDER_API_KEY or RENDER_TOKEN   Render API bearer token. Values are never printed.
  RENDER_BACKEND_SERVICE_ID        Render service ID for apex-api.
  RENDER_FRONTEND_SERVICE_ID       Render service ID for apex-frontend.
  APEX_RENDER_API_BASE             Render API base. Default: https://api.render.com/v1
`

if (process.argv.includes('--help') || process.argv.includes('-h')) {
  console.log(usage)
  process.exit(0)
}

const env = process.env
const failures = []
const notes = []

const boolEnv = (name) => env[name] === '1' || env[name]?.toLowerCase() === 'true'
const trim = (value) => (value || '').trim()
const expectProduction = boolEnv('APEX_RENDER_EXPECT_PRODUCTION')
const checkLive = expectProduction || boolEnv('APEX_RENDER_CHECK_LIVE')
const renderAPIBase = trim(env.APEX_RENDER_API_BASE) || 'https://api.render.com/v1'
const blueprintPath = trim(env.APEX_RENDER_BLUEPRINT) || 'render.yaml'
const renderToken = trim(env.RENDER_API_KEY) || trim(env.RENDER_TOKEN)
const backendServiceID = trim(env.RENDER_BACKEND_SERVICE_ID)
const frontendServiceID = trim(env.RENDER_FRONTEND_SERVICE_ID)

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

const { apiOrigin } = normalizeTargets(env.APEX_API_URL || env.PLAYWRIGHT_API_URL)

const truncate = (value, max = 600) => {
  const text = typeof value === 'string' ? value : JSON.stringify(value)
  return text.length > max ? `${text.slice(0, max)}...` : text
}

const loadBlueprint = (filePath) => {
  const ruby = `
    require "yaml"
    require "json"
    content = File.read(ARGV.fetch(0))
    data = if YAML.respond_to?(:safe_load)
      YAML.safe_load(content, permitted_classes: [Symbol], aliases: true)
    else
      YAML.load(content)
    end
    puts JSON.dump(data)
  `
  try {
    const output = execFileSync('ruby', ['-e', ruby, filePath], { encoding: 'utf8' })
    return JSON.parse(output)
  } catch (error) {
    throw new Error(`failed to parse ${filePath} with Ruby YAML: ${error.message}`)
  }
}

const envMap = (service) => {
  const map = new Map()
  for (const item of service?.envVars || []) {
    if (item?.key) {
      map.set(item.key, item)
    }
  }
  return map
}

const requireService = (blueprint, name) => {
  const service = (blueprint.services || []).find((candidate) => candidate?.name === name)
  if (!service) {
    fail(`render.yaml is missing service ${name}`)
  }
  return service
}

const requireEnvValue = (map, key, expected, context) => {
  const entry = map.get(key)
  if (!entry) {
    fail(`${context} is missing env var ${key}`)
    return
  }
  if (String(entry.value) !== String(expected)) {
    fail(`${context}.${key} must be ${expected}`)
    return
  }
  ok(`${context}.${key} is ${expected}`)
}

const requireEnvSyncFalse = (map, key, context) => {
  const entry = map.get(key)
  if (!entry) {
    fail(`${context} is missing sync:false env var ${key}`)
    return
  }
  if (entry.sync !== false) {
    fail(`${context}.${key} must use sync:false so secrets are supplied from Render, not source`)
    return
  }
  ok(`${context}.${key} is configured as sync:false`)
}

const requireEnvSource = (map, key, sourceKey, expected, context) => {
  const entry = map.get(key)
  if (!entry) {
    fail(`${context} is missing sourced env var ${key}`)
    return
  }
  const source = entry[sourceKey]
  for (const [field, value] of Object.entries(expected)) {
    if (source?.[field] !== value) {
      fail(`${context}.${key}.${sourceKey}.${field} must be ${value}`)
      return
    }
  }
  ok(`${context}.${key} is wired from ${sourceKey}`)
}

const validateBlueprint = (blueprint) => {
  const api = requireService(blueprint, 'apex-api')
  const frontend = requireService(blueprint, 'apex-frontend')
  const redis = (blueprint.services || []).find((service) => service?.name === 'apex-redis' && service?.type === 'keyvalue')
  const database = (blueprint.databases || []).find((db) => db?.name === 'apex-db')

  if (!api || !frontend) {
    return
  }

  if (api.runtime !== 'docker') fail('apex-api must use Docker runtime')
  else ok('apex-api uses Docker runtime')
  if (api.healthCheckPath !== '/ready') fail('apex-api healthCheckPath must be /ready')
  else ok('apex-api healthCheckPath is /ready')

  const apiEnv = envMap(api)
  requireEnvValue(apiEnv, 'ENVIRONMENT', 'production', 'apex-api')
  requireEnvValue(apiEnv, 'GIN_MODE', 'release', 'apex-api')
  requireEnvValue(apiEnv, 'PORT', 8080, 'apex-api')
  requireEnvValue(apiEnv, 'FRONTEND_URL', 'https://apex-build.dev', 'apex-api')
  requireEnvValue(apiEnv, 'BASE_URL', 'https://apex-build.dev', 'apex-api')
  requireEnvValue(apiEnv, 'COOKIE_SECURE', 'true', 'apex-api')
  requireEnvValue(apiEnv, 'COOKIE_SAME_SITE', 'None', 'apex-api')
  requireEnvValue(apiEnv, 'EXECUTION_FORCE_CONTAINER', 'true', 'apex-api')
  requireEnvValue(apiEnv, 'APEX_PREVIEW_BACKEND_RUNTIME', 'container', 'apex-api')
  requireEnvValue(apiEnv, 'APEX_PREVIEW_RUNTIME_VERIFY', 'true', 'apex-api')
  requireEnvValue(apiEnv, 'APEX_ENABLE_HOST_TERMINAL', 'false', 'apex-api')
  requireEnvSource(apiEnv, 'DATABASE_URL', 'fromDatabase', { name: 'apex-db', property: 'connectionString' }, 'apex-api')
  requireEnvSource(apiEnv, 'REDIS_URL', 'fromService', { type: 'keyvalue', name: 'apex-redis', property: 'connectionString' }, 'apex-api')

  const cors = String(apiEnv.get('CORS_ALLOWED_ORIGINS')?.value || '')
  for (const origin of ['https://apex-build.dev', 'https://www.apex-build.dev']) {
    if (!cors.split(',').map((value) => value.trim()).includes(origin)) {
      fail(`apex-api.CORS_ALLOWED_ORIGINS must include ${origin}`)
    }
  }

  for (const key of [
    'JWT_SECRET',
    'JWT_REFRESH_SECRET',
    'SECRETS_MASTER_KEY',
    'STRIPE_SECRET_KEY',
    'STRIPE_WEBHOOK_SECRET',
    'STRIPE_PRICE_BUILDER_MONTHLY',
    'STRIPE_PRICE_BUILDER_ANNUAL',
    'STRIPE_PRICE_PRO_MONTHLY',
    'STRIPE_PRICE_PRO_ANNUAL',
    'STRIPE_PRICE_TEAM_MONTHLY',
    'STRIPE_PRICE_TEAM_ANNUAL',
    'ANTHROPIC_API_KEY',
    'OPENAI_API_KEY',
    'GEMINI_API_KEY',
    'OLLAMA_API_KEY',
    'E2B_API_KEY',
    'APEX_EXECUTION_DOCKER_HOST',
    'APEX_PREVIEW_DOCKER_HOST',
    'APEX_PREVIEW_CONNECT_HOST',
  ]) {
    requireEnvSyncFalse(apiEnv, key, 'apex-api')
  }

  if (frontend.runtime !== 'docker') fail('apex-frontend must use Docker runtime')
  else ok('apex-frontend uses Docker runtime')
  if (frontend.healthCheckPath !== '/health') fail('apex-frontend healthCheckPath must be /health')
  else ok('apex-frontend healthCheckPath is /health')

  const frontendEnv = envMap(frontend)
  requireEnvValue(frontendEnv, 'VITE_API_URL', 'https://api.apex-build.dev/api/v1', 'apex-frontend')
  requireEnvValue(frontendEnv, 'VITE_WS_URL', 'wss://api.apex-build.dev/ws', 'apex-frontend')
  requireEnvValue(frontendEnv, 'NODE_ENV', 'production', 'apex-frontend')

  if (!redis) {
    fail('render.yaml is missing apex-redis keyvalue service')
  } else if (Array.isArray(redis.ipAllowList) && redis.ipAllowList.length === 0) {
    ok('apex-redis is internal-only via empty ipAllowList')
  } else {
    fail('apex-redis ipAllowList must be an empty array for internal-only access')
  }

  if (!database) {
    fail('render.yaml is missing apex-db database')
  } else {
    if (database.postgresMajorVersion !== 15) fail('apex-db postgresMajorVersion must be 15')
    else ok('apex-db PostgreSQL major version is 15')
  }
}

const renderRequest = async (path) => {
  const response = await fetch(`${renderAPIBase}${path}`, {
    headers: {
      accept: 'application/json',
      authorization: `Bearer ${renderToken}`,
    },
  })
  const text = await response.text()
  let body = null
  if (text) {
    try {
      body = JSON.parse(text)
    } catch {
      throw new Error(`Render API ${path} returned non-JSON ${response.status}: ${truncate(text)}`)
    }
  }
  if (!response.ok) {
    throw new Error(`Render API ${path} returned ${response.status}: ${truncate(body || text)}`)
  }
  return body
}

const fetchRenderEnvVars = async (serviceID) => {
  const vars = new Map()
  let cursor = ''
  for (let page = 0; page < 20; page += 1) {
    const params = new URLSearchParams({ limit: '100' })
    if (cursor) params.set('cursor', cursor)
    const body = await renderRequest(`/services/${encodeURIComponent(serviceID)}/env-vars?${params.toString()}`)
    if (!Array.isArray(body)) {
      throw new Error(`Render env vars response for ${serviceID} was not an array`)
    }
    if (body.length === 0) break

    for (const item of body) {
      const envVar = item.envVar || item
      if (envVar?.key) {
        vars.set(envVar.key, trim(envVar.value))
      }
    }

    const nextCursor = trim(body[body.length - 1]?.cursor)
    if (!nextCursor || nextCursor === cursor) break
    cursor = nextCursor
  }
  return vars
}

const requireLiveEnv = (vars, key, label, options = {}) => {
  const value = vars.get(key)
  if (!value) {
    fail(`${label} Render env var ${key} is missing or empty`)
    return ''
  }
  if (options.prefix && !value.startsWith(options.prefix)) {
    fail(`${label} Render env var ${key} does not match expected ${options.prefix} prefix`)
    return value
  }
  if (options.rejectPlaceholders?.has(value)) {
    fail(`${label} Render env var ${key} is still a placeholder`)
    return value
  }
  if (options.minLength && value.length < options.minLength) {
    fail(`${label} Render env var ${key} is shorter than the launch minimum`)
    return value
  }
  if (options.equals && value !== options.equals) {
    fail(`${label} Render env var ${key} must be ${options.equals}`)
    return value
  }
  ok(`${label} Render env var ${key} is present`)
  return value
}

const validateRenderAPIEnv = async () => {
  if (!renderToken || !backendServiceID) {
    const message = 'Render API env-var verification skipped; set RENDER_API_KEY and RENDER_BACKEND_SERVICE_ID'
    if (expectProduction) fail(message)
    else note(message)
    return
  }

  const backendVars = await fetchRenderEnvVars(backendServiceID)
  ok(`loaded ${backendVars.size} direct Render env vars for apex-api`)

  requireLiveEnv(backendVars, 'DATABASE_URL', 'apex-api')
  requireLiveEnv(backendVars, 'REDIS_URL', 'apex-api')
  requireLiveEnv(backendVars, 'FRONTEND_URL', 'apex-api', { equals: 'https://apex-build.dev' })
  requireLiveEnv(backendVars, 'JWT_SECRET', 'apex-api', { minLength: 32 })
  requireLiveEnv(backendVars, 'JWT_REFRESH_SECRET', 'apex-api', { minLength: 32 })
  requireLiveEnv(backendVars, 'SECRETS_MASTER_KEY', 'apex-api', { minLength: 32 })
  requireLiveEnv(backendVars, 'STRIPE_SECRET_KEY', 'apex-api', { prefix: 'sk_' })
  requireLiveEnv(backendVars, 'STRIPE_WEBHOOK_SECRET', 'apex-api', { prefix: 'whsec_' })

  for (const key of [
    'STRIPE_PRICE_BUILDER_MONTHLY',
    'STRIPE_PRICE_BUILDER_ANNUAL',
    'STRIPE_PRICE_PRO_MONTHLY',
    'STRIPE_PRICE_PRO_ANNUAL',
    'STRIPE_PRICE_TEAM_MONTHLY',
    'STRIPE_PRICE_TEAM_ANNUAL',
  ]) {
    requireLiveEnv(backendVars, key, 'apex-api', {
      prefix: 'price_',
      rejectPlaceholders: new Set([
        'price_builder_monthly',
        'price_builder_annual',
        'price_pro_monthly',
        'price_pro_annual',
        'price_team_monthly',
        'price_team_annual',
      ]),
    })
  }

  const cors = backendVars.get('CORS_ALLOWED_ORIGINS') || ''
  if (!cors.includes('https://apex-build.dev') || !cors.includes('https://www.apex-build.dev')) {
    fail('apex-api Render env var CORS_ALLOWED_ORIGINS must include apex-build.dev and www.apex-build.dev')
  } else {
    ok('apex-api Render CORS origins include production frontend domains')
  }

  const hasProvider = ['ANTHROPIC_API_KEY', 'OPENAI_API_KEY', 'GEMINI_API_KEY', 'OLLAMA_API_KEY'].
    some((key) => Boolean(backendVars.get(key)))
  if (!hasProvider) {
    fail('apex-api Render env must include at least one managed AI provider key before public launch')
  } else {
    ok('apex-api has at least one managed AI provider key configured')
  }

  const hasExecutionRuntime = Boolean(backendVars.get('E2B_API_KEY')) || Boolean(backendVars.get('APEX_EXECUTION_DOCKER_HOST'))
  if (!hasExecutionRuntime) {
    fail('apex-api Render env must include E2B_API_KEY or APEX_EXECUTION_DOCKER_HOST for isolated execution')
  } else {
    ok('apex-api has an isolated execution runtime env path configured')
  }

  const previewRuntime = backendVars.get('APEX_PREVIEW_BACKEND_RUNTIME') || 'container'
  if (previewRuntime === 'container') {
    requireLiveEnv(backendVars, 'APEX_PREVIEW_DOCKER_HOST', 'apex-api')
    requireLiveEnv(backendVars, 'APEX_PREVIEW_CONNECT_HOST', 'apex-api')
  } else if (previewRuntime === 'e2b') {
    requireLiveEnv(backendVars, 'E2B_API_KEY', 'apex-api')
  } else {
    fail(`apex-api APEX_PREVIEW_BACKEND_RUNTIME must be container or e2b, got ${previewRuntime}`)
  }

  if (frontendServiceID) {
    const frontendVars = await fetchRenderEnvVars(frontendServiceID)
    ok(`loaded ${frontendVars.size} direct Render env vars for apex-frontend`)
    requireLiveEnv(frontendVars, 'VITE_API_URL', 'apex-frontend', { equals: 'https://api.apex-build.dev/api/v1' })
    requireLiveEnv(frontendVars, 'VITE_WS_URL', 'apex-frontend', { equals: 'wss://api.apex-build.dev/ws' })
  } else {
    note('frontend Render API env-var verification skipped; set RENDER_FRONTEND_SERVICE_ID')
  }
}

const requestJSON = async (url) => {
  const response = await fetch(url, { headers: { accept: 'application/json' } })
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

const findReadinessService = (body, name) => {
  const services = Array.isArray(body?.services) ? body.services : []
  return services.find((service) => service?.name === name)
}

const validateLiveHealth = async () => {
  if (!checkLive) {
    note('live production health check skipped; set APEX_RENDER_CHECK_LIVE=1 or APEX_RENDER_EXPECT_PRODUCTION=1')
    return
  }

  const ready = await requestJSON(`${apiOrigin}/ready`)
  if (ready.ready !== true || ready.startup?.ready !== true) {
    fail(`/ready is not ready: ${truncate(ready)}`)
  } else {
    ok('/ready is healthy and ready')
  }

  const health = await requestJSON(`${apiOrigin}/health`)
  if (health.status !== 'healthy' || health.ready !== true) {
    fail(`/health is not healthy: ${truncate(health)}`)
  } else {
    ok('/health remains healthy')
  }

  const features = await requestJSON(`${apiOrigin}/health/features`)
  const redis = findReadinessService(features, 'redis_cache')
  const codeExecution = findReadinessService(features, 'code_execution')
  const previewService = findReadinessService(features, 'preview_service')
  const runtimeVerify = findReadinessService(features, 'preview_runtime_verify')

  if (redis?.state === 'degraded') {
    fail(`redis_cache is degraded: ${truncate(redis)}`)
  } else {
    ok('redis_cache is not degraded')
  }

  for (const [name, service] of [
    ['code_execution', codeExecution],
    ['preview_service', previewService],
  ]) {
    if (!service) {
      fail(`${name} readiness service is missing`)
    } else if (service.details?.launch_ready !== true) {
      fail(`${name}.details.launch_ready is not true: ${truncate(service)}`)
    } else {
      ok(`${name}.details.launch_ready is true`)
    }
  }

  if (!runtimeVerify) {
    fail('preview_runtime_verify readiness service is missing')
  } else if (runtimeVerify.state !== 'ready' || runtimeVerify.details?.browser_proof !== true) {
    fail(`preview_runtime_verify is not browser-proof ready: ${truncate(runtimeVerify)}`)
  } else {
    ok('preview_runtime_verify is browser-proof ready')
  }
}

try {
  console.log(`[info] blueprint: ${blueprintPath}`)
  const blueprint = loadBlueprint(blueprintPath)
  validateBlueprint(blueprint)
  await validateRenderAPIEnv()
  await validateLiveHealth()
} catch (error) {
  fail(error instanceof Error ? error.message : String(error))
}

if (failures.length > 0) {
  console.error(`\nRender launch verification failed with ${failures.length} issue(s).`)
  process.exit(1)
}

console.log('\nRender launch verification completed.')
if (notes.length > 0) {
  console.log('Notes remain; rerun with strict env vars for launch evidence.')
}
