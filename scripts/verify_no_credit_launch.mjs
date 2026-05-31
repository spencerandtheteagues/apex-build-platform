#!/usr/bin/env node

const usage = `Usage:
  APEX_NO_CREDIT_ADMIN_PASSWORD=... node scripts/verify_no_credit_launch.mjs

Optional free-account proof:
  APEX_NO_CREDIT_ADMIN_PASSWORD=... \\
  APEX_NO_CREDIT_REGISTER_FREE=1 \\
  node scripts/verify_no_credit_launch.mjs

Environment:
  APEX_API_URL                         API origin or /api/v1 base. Default: https://api.apex-build.dev
  APEX_FRONTEND_URL                    Frontend URL. Default: https://apex-build.dev
  APEX_NO_CREDIT_ADMIN_USERNAME        Admin username. Default: admin
  APEX_NO_CREDIT_ADMIN_EMAIL           Admin email. Optional.
  APEX_NO_CREDIT_ADMIN_PASSWORD        Admin password. Required unless APEX_NO_CREDIT_SKIP_ADMIN=1.
  APEX_NO_CREDIT_SKIP_ADMIN=1          Skip authenticated admin/account checks.
  APEX_NO_CREDIT_REGISTER_FREE=1       Register a throwaway free account and verify free entitlements.
  APEX_NO_CREDIT_FREE_USERNAME         Existing free username/email, or generated when registering.
  APEX_NO_CREDIT_FREE_EMAIL            Existing free email, or generated when registering.
  APEX_NO_CREDIT_FREE_PASSWORD         Existing/generated free password.
  APEX_NO_CREDIT_TIMEOUT_MS            Per-request timeout. Default: 15000.

This verifier intentionally does not call /ai/generate, /build/start, checkout,
portal, deploy, preview-start, or any provider endpoint that would consume AI
credits. It is a no-credit launch smoke gate, not a generation-quality canary.
`

if (process.argv.includes('--help') || process.argv.includes('-h')) {
  console.log(usage)
  process.exit(0)
}

const env = process.env
const trim = (value) => String(value || '').trim()
const boolEnv = (name) => env[name] === '1' || String(env[name] || '').toLowerCase() === 'true'
const timeoutMS = Math.max(3000, Number.parseInt(trim(env.APEX_NO_CREDIT_TIMEOUT_MS) || '15000', 10))

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

const { apiOrigin, apiV1Base } = normalizeTargets(env.APEX_API_URL || env.PLAYWRIGHT_API_URL)
const frontendURL = trim(env.APEX_FRONTEND_URL) || trim(env.PLAYWRIGHT_BASE_URL) || 'https://apex-build.dev'
const skipAdmin = boolEnv('APEX_NO_CREDIT_SKIP_ADMIN')
const registerFree = boolEnv('APEX_NO_CREDIT_REGISTER_FREE')

const failures = []
const warnings = []

const ok = (message) => console.log(`[ok] ${message}`)
const note = (message) => console.log(`[note] ${message}`)
const warn = (message) => {
  warnings.push(message)
  console.log(`[warn] ${message}`)
}
const fail = (message) => {
  failures.push(message)
  console.error(`[fail] ${message}`)
}

const truncate = (value, max = 700) => {
  const text = typeof value === 'string' ? value : JSON.stringify(value)
  return text.length > max ? `${text.slice(0, max)}...` : text
}

class CookieJar {
  constructor() {
    this.cookies = new Map()
  }

  record(headers) {
    const values = typeof headers.getSetCookie === 'function'
      ? headers.getSetCookie()
      : [headers.get('set-cookie')].filter(Boolean)
    for (const raw of values.flatMap((value) => String(value).split(/,(?=\s*[^;,]+=)/))) {
      const first = raw.split(';', 1)[0]
      const eq = first.indexOf('=')
      if (eq > 0) this.cookies.set(first.slice(0, eq), first.slice(eq + 1))
    }
  }

  header() {
    return [...this.cookies.entries()].map(([key, value]) => `${key}=${value}`).join('; ')
  }
}

const request = async (url, options = {}) => {
  const method = options.method || 'GET'
  const headers = {
    accept: 'application/json',
    ...(options.body ? { 'content-type': 'application/json' } : {}),
    ...(options.headers || {}),
  }
  if (options.jar) {
    const cookie = options.jar.header()
    if (cookie) headers.cookie = cookie
  }
  const signal = AbortSignal.timeout(timeoutMS)
  const response = await fetch(url, {
    ...options,
    method,
    headers,
    body: options.body === undefined ? undefined : JSON.stringify(options.body),
    signal,
  })
  if (options.jar) options.jar.record(response.headers)
  const text = await response.text()
  let body = null
  if (text) {
    try {
      body = JSON.parse(text)
    } catch {
      body = { raw: text }
    }
  }
  if (!response.ok) {
    const error = new Error(`${method} ${url} returned ${response.status}: ${truncate(body || text)}`)
    error.status = response.status
    error.body = body
    throw error
  }
  return { response, body, text }
}

const getJSON = async (url, options = {}) => (await request(url, options)).body

const csrfToken = async (jar) => {
  const body = await getJSON(`${apiV1Base}/csrf-token`, { jar })
  return trim(body?.token) || trim(body?.csrf_token)
}

const authHeaders = async (jar) => {
  const token = await csrfToken(jar)
  if (!token) throw new Error('CSRF endpoint did not return a token')
  return { 'x-csrf-token': token }
}

const login = async ({ username, email, password }) => {
  const jar = new CookieJar()
  const headers = await authHeaders(jar)
  const body = await getJSON(`${apiV1Base}/auth/login`, {
    jar,
    method: 'POST',
    headers,
    body: {
      username,
      email,
      password,
    },
  })
  if (!body?.user) throw new Error('login response did not include user')
  return { jar, user: body.user }
}

const register = async ({ username, email, password }) => {
  const jar = new CookieJar()
  const headers = await authHeaders(jar)
  const body = await getJSON(`${apiV1Base}/auth/register`, {
    jar,
    method: 'POST',
    headers,
    body: {
      username,
      email,
      password,
      full_name: 'APEX No Credit Smoke',
      accept_legal_terms: true,
    },
  })
  if (!body?.user && !body?.data?.user) throw new Error('register response did not include user')
  return { jar, user: body.user || body.data.user }
}

const authenticatedGet = async (jar, path) => getJSON(`${apiV1Base}${path}`, { jar })

const requireEqual = (label, actual, expected) => {
  if (actual !== expected) fail(`${label}: got ${JSON.stringify(actual)}, expected ${JSON.stringify(expected)}`)
  else ok(`${label}: ${JSON.stringify(actual)}`)
}

const requireTruthy = (label, value) => {
  if (!value) fail(`${label}: expected truthy value`)
  else ok(label)
}

const publicReadiness = async () => {
  const health = await getJSON(`${apiOrigin}/health`)
  requireEqual('/health status', health?.status, 'healthy')
  requireEqual('/health ready', health?.ready, true)
  if (health?.startup?.started_at) ok(`/health startup ${health.startup.started_at}`)

  const providers = health?.ai_providers || {}
  const providerStatuses = Object.fromEntries(Object.entries(providers).map(([key, value]) => [key, value?.status || value]))
  note(`provider statuses: ${JSON.stringify(providerStatuses)}`)
  for (const provider of ['claude', 'gemini', 'grok', 'ollama']) {
    if (providerStatuses[provider] && providerStatuses[provider] === 'ok') {
      warn(`${provider} reports ok in /health; verify account credits manually before generation canaries`)
    }
  }

  const ready = await getJSON(`${apiOrigin}/ready`)
  requireEqual('/ready status', ready?.status, 'healthy')
  requireEqual('/ready ready', ready?.ready, true)

  const features = await getJSON(`${apiOrigin}/health/features`)
  const services = Array.isArray(features?.services) ? features.services : []
  const serviceByName = new Map(services.map((service) => [service?.name, service]))
  for (const name of ['payments', 'preview_service', 'preview_runtime_verify', 'code_execution']) {
    const service = serviceByName.get(name)
    if (!service) {
      fail(`/health/features missing ${name}`)
      continue
    }
    requireEqual(`/health/features ${name} state`, service.state, 'ready')
  }
  const payments = serviceByName.get('payments')
  if (payments?.details) {
    requireEqual('payments ready flag', payments.details.ready, true)
    requireEqual('payments webhook configured', payments.details.webhook_configured, true)
    requireEqual('payments price ids configured', payments.details.required_price_ids_configured, true)
  }

  const platformTruth = await getJSON(`${apiV1Base}/platform/truth`)
  const featureCount = Array.isArray(platformTruth?.features) ? platformTruth.features.length : 0
  if (featureCount <= 0) fail('/platform/truth returned no features')
  else ok(`/platform/truth feature count ${featureCount}`)

  const frontend = await request(frontendURL, { headers: { accept: 'text/html,application/xhtml+xml' } })
  const body = frontend.text || ''
  if (!/<html/i.test(body) || !/id=["']root["']/i.test(body)) {
    fail(`${frontendURL} did not look like the React app shell`)
  } else {
    ok(`${frontendURL} serves React app shell`)
  }
}

const verifyAccount = async (label, jar, expectations) => {
  const profile = await authenticatedGet(jar, '/user/profile')
  const user = profile?.data?.user || {}
  requireEqual(`${label} profile subscription_type`, user.subscription_type, expectations.subscriptionType)
  requireEqual(`${label} profile bypass_billing`, Boolean(user.bypass_billing), expectations.bypassBilling)
  requireEqual(`${label} profile unlimited`, Boolean(user.has_unlimited_credits), expectations.unlimited)
  if (expectations.verified !== undefined) {
    requireEqual(`${label} profile verified`, Boolean(user.is_verified), expectations.verified)
  }

  const subscription = await authenticatedGet(jar, '/billing/subscription')
  const sub = subscription?.data || {}
  requireEqual(`${label} billing plan_type`, sub.plan_type, expectations.subscriptionType)
  requireTruthy(`${label} billing plan_name present`, sub.plan_name)
  if (label === 'admin' && sub.status !== 'active') {
    warn('admin account has owner entitlements but billing subscription status is not active; this is not Stripe paid lifecycle proof')
  }

  const balance = await authenticatedGet(jar, '/billing/credits/balance')
  const credit = balance?.data || {}
  requireEqual(`${label} credit bypass_billing`, Boolean(credit.bypass_billing), expectations.bypassBilling)
  requireEqual(`${label} credit unlimited`, Boolean(credit.has_unlimited), expectations.unlimited)

  const usage = await authenticatedGet(jar, '/billing/usage')
  requireTruthy(`${label} billing usage returned`, usage?.success === true && usage?.data)

  const limit = await authenticatedGet(jar, '/billing/check-limit/ai_requests')
  if (expectations.bypassBilling) {
    requireEqual(`${label} check-limit bypassed`, Boolean(limit?.data?.bypassed), true)
  } else {
    requireEqual(`${label} check-limit bypassed`, Boolean(limit?.data?.bypassed), false)
    requireEqual(`${label} check-limit plan_type`, limit?.data?.plan_type, expectations.subscriptionType)
  }
}

const adminProof = async () => {
  if (skipAdmin) {
    warn('admin checks skipped by APEX_NO_CREDIT_SKIP_ADMIN=1')
    return
  }
  const password = trim(env.APEX_NO_CREDIT_ADMIN_PASSWORD)
  if (!password) {
    fail('APEX_NO_CREDIT_ADMIN_PASSWORD is required for admin checks')
    return
  }

  const username = trim(env.APEX_NO_CREDIT_ADMIN_USERNAME) || 'admin'
  const email = trim(env.APEX_NO_CREDIT_ADMIN_EMAIL)
  const { jar, user } = await login({ username, email, password })
  requireEqual('admin login username', user.username, username)
  requireEqual('admin login is_admin', Boolean(user.is_admin), true)
  requireEqual('admin login is_super_admin', Boolean(user.is_super_admin), true)
  requireEqual('admin login subscription_type', user.subscription_type, 'owner')
  requireEqual('admin login bypass_billing', Boolean(user.bypass_billing), true)
  requireEqual('admin login unlimited', Boolean(user.has_unlimited_credits), true)

  await verifyAccount('admin', jar, {
    subscriptionType: 'owner',
    bypassBilling: true,
    unlimited: true,
    verified: true,
  })

  const dashboard = await authenticatedGet(jar, '/admin/dashboard')
  requireTruthy('admin dashboard stats returned', dashboard?.stats?.users && dashboard?.stats?.projects)

  const users = await authenticatedGet(jar, '/admin/users?limit=1')
  const returned = Array.isArray(users?.users) ? users.users.length : 0
  if (returned < 1) fail('admin users returned no rows')
  else ok(`admin users returned ${returned} row`)

  const config = await authenticatedGet(jar, '/billing/config-status')
  requireEqual('billing config ready', config?.data?.ready, true)
  requireEqual('billing config webhook', config?.data?.webhook_configured, true)
  requireEqual('billing config price ids', config?.data?.required_price_ids_configured, true)

  const plans = await authenticatedGet(jar, '/billing/plans')
  requireTruthy('billing plans returned', plans?.success === true && plans?.data)
}

const freeAccountProof = async () => {
  if (!registerFree && !(trim(env.APEX_NO_CREDIT_FREE_USERNAME) || trim(env.APEX_NO_CREDIT_FREE_EMAIL))) {
    note('free account proof skipped; set APEX_NO_CREDIT_REGISTER_FREE=1 or provide free credentials')
    return
  }

  let username = trim(env.APEX_NO_CREDIT_FREE_USERNAME)
  let email = trim(env.APEX_NO_CREDIT_FREE_EMAIL)
  let password = trim(env.APEX_NO_CREDIT_FREE_PASSWORD)
  let session

  if (registerFree) {
    const stamp = `${Date.now()}${Math.floor(Math.random() * 10000)}`
    username ||= `nocredit${stamp}`
    email ||= `${username}@example.com`
    password ||= `NoCredit!${stamp}aA1`
    session = await register({ username, email, password })
    ok(`registered throwaway free account ${username}`)
  } else {
    if (!password) {
      fail('APEX_NO_CREDIT_FREE_PASSWORD is required when using an existing free account')
      return
    }
    session = await login({ username, email, password })
    ok(`authenticated existing free account ${username || email}`)
  }

  await verifyAccount('free', session.jar, {
    subscriptionType: 'free',
    bypassBilling: false,
    unlimited: false,
  })
}

try {
  console.log(`APEX_NO_CREDIT_LAUNCH_SMOKE api=${apiV1Base} frontend=${frontendURL}`)
  await publicReadiness()
  await adminProof()
  await freeAccountProof()
} catch (error) {
  fail(error instanceof Error ? error.message : String(error))
}

console.log('')
console.log('================ NO-CREDIT LAUNCH SMOKE ================')
for (const warning of warnings) console.log(`  WARN  ${warning}`)
for (const failure of failures) console.log(`  FAIL  ${failure}`)
console.log('=========================================================')

if (failures.length > 0) {
  console.error(`NO_CREDIT_LAUNCH_SMOKE_FAILED failures=${failures.length} warnings=${warnings.length}`)
  process.exit(1)
}

console.log(`NO_CREDIT_LAUNCH_SMOKE_PASSED warnings=${warnings.length}`)
