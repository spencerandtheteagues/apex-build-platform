#!/usr/bin/env node

const placeholderStripePriceIds = new Set([
  'price_builder_monthly',
  'price_builder_annual',
  'price_pro_monthly',
  'price_pro_annual',
  'price_team_monthly',
  'price_team_annual',
  'price_enterprise_monthly',
  'price_enterprise_annual',
])

const usage = `Usage:
  APEX_STRIPE_EXPECT_LIVE=1 APEX_STRIPE_REGISTER_SMOKE_USER=1 node scripts/verify_stripe_launch.mjs

Environment:
  APEX_API_URL                         API origin or /api/v1 base. Default: https://api.apex-build.dev
  APEX_STRIPE_EXPECT_LIVE=1            Fail unless payments readiness and paid price IDs are launch-ready.
  APEX_STRIPE_USERNAME                 Existing launch-smoke username or email.
  APEX_STRIPE_EMAIL                    Existing launch-smoke email. Used when username is omitted.
  APEX_STRIPE_PASSWORD                 Existing launch-smoke password.
  APEX_STRIPE_REGISTER_SMOKE_USER=1    Register a throwaway smoke user when existing credentials are not provided.
  APEX_STRIPE_RUN_CHECKOUT=1           Create a subscription checkout session. Does not complete payment.
  APEX_STRIPE_CHECKOUT_PLAN=builder    builder | pro | team. Default: builder.
  APEX_STRIPE_CHECKOUT_CYCLE=monthly   monthly | annual. Default: monthly.
  APEX_STRIPE_RUN_CREDIT_CHECKOUT=1    Create a credit top-up checkout session. Does not complete payment.
  APEX_STRIPE_CREDIT_AMOUNT=25         One of the supported credit top-up amounts. Default: 25.
  APEX_FRONTEND_URL                    Redirect base for checkout probes. Default: https://apex-build.dev
`

if (process.argv.includes('--help') || process.argv.includes('-h')) {
  console.log(usage)
  process.exit(0)
}

const env = process.env
const boolEnv = (name) => env[name] === '1' || env[name]?.toLowerCase() === 'true'
const trim = (value) => (value || '').trim()

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
const expectLive = boolEnv('APEX_STRIPE_EXPECT_LIVE') || boolEnv('PLAYWRIGHT_EXPECT_LIVE_STRIPE')
const registerSmokeUser = boolEnv('APEX_STRIPE_REGISTER_SMOKE_USER')
const runSubscriptionCheckout = boolEnv('APEX_STRIPE_RUN_CHECKOUT')
const runCreditCheckout = boolEnv('APEX_STRIPE_RUN_CREDIT_CHECKOUT')
const frontendURL = trim(env.APEX_FRONTEND_URL) || trim(env.PLAYWRIGHT_BASE_URL) || 'https://apex-build.dev'
const checkoutPlan = trim(env.APEX_STRIPE_CHECKOUT_PLAN) || 'builder'
const checkoutCycle = trim(env.APEX_STRIPE_CHECKOUT_CYCLE) || 'monthly'
const creditAmount = Number.parseInt(trim(env.APEX_STRIPE_CREDIT_AMOUNT) || '25', 10)

const failures = []
const notes = []

const fail = (message) => {
  failures.push(message)
  console.error(`[fail] ${message}`)
}

const ok = (message) => {
  console.log(`[ok] ${message}`)
}

const note = (message) => {
  notes.push(message)
  console.log(`[note] ${message}`)
}

const truncate = (value, max = 500) => {
  const text = typeof value === 'string' ? value : JSON.stringify(value)
  return text.length > max ? `${text.slice(0, max)}...` : text
}

const requestJSON = async (url, options = {}) => {
  const method = options.method || 'GET'
  const headers = {
    accept: 'application/json',
    ...(options.body ? { 'content-type': 'application/json' } : {}),
    ...(options.headers || {}),
  }
  const response = await fetch(url, { ...options, headers })
  const text = await response.text()
  let body = null
  if (text) {
    try {
      body = JSON.parse(text)
    } catch {
      throw new Error(`${method} ${url} returned non-JSON ${response.status}: ${truncate(text)}`)
    }
  }
  if (!response.ok) {
    throw new Error(`${method} ${url} returned ${response.status}: ${truncate(body || text)}`)
  }
  return { response, body }
}

const cookieHeaderFromResponse = (response) => {
  const getSetCookie = response.headers?.getSetCookie
  const setCookieValues = typeof getSetCookie === 'function'
    ? getSetCookie.call(response.headers)
    : [response.headers.get('set-cookie')].filter(Boolean)
  return setCookieValues
    .flatMap((value) => String(value).split(/,(?=\s*[^;,]+=)/))
    .map((value) => value.split(';')[0]?.trim())
    .filter(Boolean)
    .join('; ')
}

const extractAccessToken = (body) =>
  trim(body?.data?.access_token) ||
  trim(body?.data?.tokens?.access_token) ||
  trim(body?.tokens?.access_token) ||
  trim(body?.access_token)

const authHeaders = (auth) => {
  if (auth?.token) {
    return { authorization: `Bearer ${auth.token}` }
  }
  if (auth?.cookie) {
    return { cookie: auth.cookie }
  }
  return {}
}

const paidSelfServePlans = ['builder', 'pro', 'team']

const findService = (readinessBody, name) => {
  const services = Array.isArray(readinessBody?.services) ? readinessBody.services : []
  return services.find((service) => service?.name === name)
}

const validatePaymentsReadiness = async () => {
  const { body } = await requestJSON(`${apiOrigin}/health/features`)
  const payments = findService(body, 'payments')
  if (!payments) {
    if (expectLive) {
      fail('/health/features does not include the payments readiness service')
    } else {
      note('/health/features does not include the payments readiness service')
    }
    return
  }

  const ready = payments.state === 'ready' && payments.details?.ready !== false
  if (ready) {
    ok('/health/features reports payments ready')
    return
  }

  const summary = truncate({
    state: payments.state,
    message: payments.message,
    details: payments.details,
  })
  if (expectLive) {
    fail(`/health/features payments service is not launch-ready: ${summary}`)
  } else {
    note(`/health/features payments service is not launch-ready: ${summary}`)
  }
}

const authenticate = async () => {
  const username = trim(env.APEX_STRIPE_USERNAME)
  const email = trim(env.APEX_STRIPE_EMAIL)
  const password = trim(env.APEX_STRIPE_PASSWORD)

  if ((username || email) && password) {
    const { response, body } = await requestJSON(`${apiV1Base}/auth/login`, {
      method: 'POST',
      body: JSON.stringify({ username, email, password }),
    })
    const token = extractAccessToken(body)
    const cookie = cookieHeaderFromResponse(response)
    if (!token && !cookie) {
      throw new Error('login succeeded but response did not include a bearer token or session cookie')
    }
    ok(`authenticated existing smoke user ${username || email}`)
    return { token, cookie, username: username || email, email }
  }

  if (!registerSmokeUser) {
    return null
  }

  const stamp = Date.now()
  const generatedUsername = `stripe-smoke-${stamp}`
  const generatedEmail = `stripe-smoke-${stamp}@example.com`
  const generatedPassword = `StripeSmoke!${stamp}aA1`
  const { response, body } = await requestJSON(`${apiV1Base}/auth/register`, {
    method: 'POST',
    body: JSON.stringify({
      username: generatedUsername,
      email: generatedEmail,
      password: generatedPassword,
      full_name: 'Stripe Launch Smoke',
      accept_legal_terms: true,
    }),
  })
  const token = extractAccessToken(body)
  const cookie = cookieHeaderFromResponse(response)
  if (!token && !cookie) {
    throw new Error('registration succeeded but response did not include a bearer token or session cookie')
  }
  ok(`registered throwaway smoke user ${generatedUsername}`)
  return { token, cookie, username: generatedUsername, email: generatedEmail }
}

const validateConfigStatus = async (auth) => {
  const { body } = await requestJSON(`${apiV1Base}/billing/config-status`, {
    headers: authHeaders(auth),
  })
  const status = body?.data
  if (!status) {
    throw new Error('/billing/config-status did not return data')
  }

  const ready = status.ready === true && status.webhook_configured === true && status.required_price_ids_configured === true
  if (ready) {
    ok('/billing/config-status reports Stripe launch config ready')
    return
  }

  const summary = truncate({
    configured: status.configured,
    ready: status.ready,
    webhook_configured: status.webhook_configured,
    required_price_ids_configured: status.required_price_ids_configured,
    missing_env: status.missing_env,
    placeholder_env: status.placeholder_env,
    issues: status.issues,
  })
  if (expectLive) {
    fail(`/billing/config-status is not launch-ready: ${summary}`)
  } else {
    note(`/billing/config-status is not launch-ready: ${summary}`)
  }
}

const validatePlans = async (auth) => {
  const { body } = await requestJSON(`${apiV1Base}/billing/plans`, {
    headers: authHeaders(auth),
  })
  const plans = body?.data?.plans
  if (!Array.isArray(plans)) {
    throw new Error('/billing/plans did not return data.plans')
  }

  for (const planType of ['free', ...paidSelfServePlans]) {
    if (!plans.some((plan) => plan?.type === planType)) {
      fail(`/billing/plans is missing ${planType}`)
    }
  }

  for (const plan of plans.filter((candidate) => paidSelfServePlans.includes(candidate?.type))) {
    for (const field of ['monthly_price_id', 'annual_price_id']) {
      const value = trim(plan[field])
      if (!value || placeholderStripePriceIds.has(value)) {
        const message = `${plan.type}.${field} is missing or placeholder`
        if (expectLive) fail(message)
        else note(message)
      }
    }
  }

  ok('/billing/plans returned the self-serve plan ladder')
  return plans
}

const fetchCSRFToken = async (auth) => {
  const { body } = await requestJSON(`${apiV1Base}/csrf-token`, {
    headers: authHeaders(auth),
  })
  const token = trim(body?.token) || trim(body?.data?.token)
  if (!token) {
    throw new Error('/csrf-token did not return token')
  }
  return token
}

const createSubscriptionCheckout = async (auth, plans) => {
  if (!paidSelfServePlans.includes(checkoutPlan)) {
    throw new Error(`APEX_STRIPE_CHECKOUT_PLAN must be one of ${paidSelfServePlans.join(', ')}`)
  }
  if (!['monthly', 'annual'].includes(checkoutCycle)) {
    throw new Error('APEX_STRIPE_CHECKOUT_CYCLE must be monthly or annual')
  }

  const plan = plans.find((candidate) => candidate?.type === checkoutPlan)
  const priceID = trim(plan?.[checkoutCycle === 'annual' ? 'annual_price_id' : 'monthly_price_id'])
  if (!priceID || placeholderStripePriceIds.has(priceID)) {
    throw new Error(`cannot create checkout for ${checkoutPlan}/${checkoutCycle}; price ID is missing or placeholder`)
  }

  const csrfToken = await fetchCSRFToken(auth)
  const { body } = await requestJSON(`${apiV1Base}/billing/checkout`, {
    method: 'POST',
    headers: {
      ...authHeaders(auth),
      'x-csrf-token': csrfToken,
    },
    body: JSON.stringify({
      price_id: priceID,
      success_url: `${frontendURL.replace(/\/+$/, '')}/billing?launch_stripe=success`,
      cancel_url: `${frontendURL.replace(/\/+$/, '')}/billing?launch_stripe=canceled`,
    }),
  })
  const checkoutURL = trim(body?.data?.checkout_url)
  if (!checkoutURL || !checkoutURL.includes('stripe.com')) {
    throw new Error(`/billing/checkout did not return a Stripe checkout_url: ${truncate(body)}`)
  }
  ok(`created ${checkoutPlan}/${checkoutCycle} subscription checkout session`)
}

const createCreditCheckout = async (auth) => {
  const csrfToken = await fetchCSRFToken(auth)
  const { body } = await requestJSON(`${apiV1Base}/billing/credits/purchase`, {
    method: 'POST',
    headers: {
      ...authHeaders(auth),
      'x-csrf-token': csrfToken,
    },
    body: JSON.stringify({ amount_usd: creditAmount }),
  })
  const checkoutURL = trim(body?.data?.checkout_url)
  if (!checkoutURL || !checkoutURL.includes('stripe.com')) {
    throw new Error(`/billing/credits/purchase did not return a Stripe checkout_url: ${truncate(body)}`)
  }
  ok(`created $${creditAmount} credit checkout session`)
}

try {
  console.log(`[info] API origin: ${apiOrigin}`)
  await validatePaymentsReadiness()

  const auth = await authenticate()
  if (!auth) {
    const message = 'protected billing checks skipped; set APEX_STRIPE_USERNAME/APEX_STRIPE_PASSWORD or APEX_STRIPE_REGISTER_SMOKE_USER=1'
    if (expectLive || runSubscriptionCheckout || runCreditCheckout) {
      fail(message)
    } else {
      note(message)
    }
  } else {
    await validateConfigStatus(auth)
    const plans = await validatePlans(auth)

    if (runSubscriptionCheckout) {
      await createSubscriptionCheckout(auth, plans)
    } else {
      note('subscription checkout creation skipped; set APEX_STRIPE_RUN_CHECKOUT=1 to create a non-paid checkout session')
    }

    if (runCreditCheckout) {
      await createCreditCheckout(auth)
    } else {
      note('credit checkout creation skipped; set APEX_STRIPE_RUN_CREDIT_CHECKOUT=1 to create a non-paid credit checkout session')
    }
  }
} catch (error) {
  fail(error instanceof Error ? error.message : String(error))
}

if (failures.length > 0) {
  console.error(`\nStripe launch verification failed with ${failures.length} issue(s).`)
  process.exit(1)
}

console.log('\nStripe launch verification completed.')
if (notes.length > 0) {
  console.log('Notes remain; rerun with stricter env vars for launch evidence.')
}
console.log('Live webhook replay still requires Stripe event resend evidence against the deployed webhook endpoint.')
