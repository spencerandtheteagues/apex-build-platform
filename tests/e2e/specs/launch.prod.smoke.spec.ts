import { expect, test } from '@playwright/test'

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

const launchUsername = process.env.PLAYWRIGHT_LAUNCH_USERNAME?.trim() || ''
const launchPassword = process.env.PLAYWRIGHT_LAUNCH_PASSWORD?.trim() || ''
const expectLiveStripe = process.env.PLAYWRIGHT_EXPECT_LIVE_STRIPE === '1'
const expectLaunchReady = process.env.PLAYWRIGHT_EXPECT_LAUNCH_READY === '1'

const resolveApiTargets = (): { apiOrigin: string; apiV1Base: string } => {
  const configured = (process.env.PLAYWRIGHT_API_URL || 'http://127.0.0.1:8080').replace(/\/$/, '')
  if (configured.endsWith('/api/v1')) {
    return {
      apiOrigin: configured.slice(0, -'/api/v1'.length),
      apiV1Base: configured,
    }
  }

  return {
    apiOrigin: configured,
    apiV1Base: `${configured}/api/v1`,
  }
}

const { apiOrigin, apiV1Base } = resolveApiTargets()

const findReadinessService = (body: any, name: string): any | undefined => {
  const services = Array.isArray(body?.services) ? body.services : []
  return services.find((service: any) => service?.name === name)
}

test.describe('Launch readiness smoke', () => {
  test('landing exposes real public resource links', async ({ page }) => {
    await page.goto('/')

    await expect(page.getByRole('button', { name: /Start building|Get Started Free|Start Building Free|Build with Apex/i }).first()).toBeVisible()

    await expect(page.getByRole('link', { name: 'Privacy' }).first()).toHaveAttribute('href', /(?:\/\?legal=privacy|\/privacy)$/)
    await expect(page.getByRole('link', { name: 'Terms' }).first()).toHaveAttribute('href', /(?:\/\?legal=terms|\/terms)$/)
    const docsLink = page.getByRole('link', { name: 'Docs' })
    if (await docsLink.count()) {
      await expect(docsLink.first()).toHaveAttribute('href', /(?:\/\?help=1|#docs)$/)
    } else {
      await expect(page.locator('#docs')).toBeAttached()
    }

    const helpLink = page.getByRole('link', { name: 'Help' }).first()
    await expect(helpLink).toHaveAttribute('href', '/?help=1')
  })

  test('public legal and help surfaces open without authentication', async ({ page }) => {
    await page.goto('/?legal=privacy')
    await expect(page.getByText('Privacy Policy').first()).toBeVisible()

    await page.goto('/?help=1')
    await expect(page.getByRole('heading', { name: 'Help Center' })).toBeVisible()
  })

  test('auth screen keeps legal acceptance reachable', async ({ page }) => {
    await page.goto('/settings')

    await expect(page.getByText(/Welcome Back|Join the Future/).first()).toBeVisible()

    await page.getByRole('button', { name: 'Sign up' }).click()
    await expect(page.getByText(/I agree to the Terms of Service/i)).toBeVisible()
    await expect(page.getByRole('button', { name: 'Terms of Service' }).first()).toBeVisible()
  })

  test('backend health and feature readiness are healthy', async ({ request }) => {
    const healthResponse = await request.get(`${apiOrigin}/health`)
    expect(healthResponse.status()).toBe(200)
    const healthBody = await healthResponse.json()
    expect(healthBody.status).toBe('healthy')
    expect(healthBody.ready).toBeTruthy()

    const readinessResponse = await request.get(`${apiOrigin}/health/features`)
    expect(readinessResponse.status()).toBe(200)
    const readinessBody = await readinessResponse.json()
    expect(readinessBody.ready).toBeTruthy()
    expect(['healthy', 'degraded']).toContain(readinessBody.status)

    const codeExecution = findReadinessService(readinessBody, 'code_execution')
    const previewService = findReadinessService(readinessBody, 'preview_service')
    const runtimeVerify = findReadinessService(readinessBody, 'preview_runtime_verify')

    if (expectLaunchReady) {
      expect(codeExecution, 'code_execution readiness service must be registered').toBeTruthy()
      expect(codeExecution?.details?.launch_ready, JSON.stringify(codeExecution, null, 2)).toBe(true)
      expect(previewService, 'preview_service readiness service must be registered').toBeTruthy()
      expect(previewService?.details?.launch_ready, JSON.stringify(previewService, null, 2)).toBe(true)
      expect(runtimeVerify, 'preview_runtime_verify readiness service must be registered').toBeTruthy()
      expect(runtimeVerify?.state, JSON.stringify(runtimeVerify, null, 2)).toBe('ready')
      expect(runtimeVerify?.details?.enabled, JSON.stringify(runtimeVerify, null, 2)).toBe(true)
      expect(runtimeVerify?.details?.browser_proof, JSON.stringify(runtimeVerify, null, 2)).toBe(true)
    } else {
      if (Object.prototype.hasOwnProperty.call(codeExecution?.details ?? {}, 'launch_ready')) {
        expect(codeExecution.details.launch_ready, JSON.stringify(codeExecution, null, 2)).not.toBe(false)
      }
      if (Object.prototype.hasOwnProperty.call(previewService?.details ?? {}, 'launch_ready')) {
        expect(previewService.details.launch_ready, JSON.stringify(previewService, null, 2)).not.toBe(false)
      }
      if (runtimeVerify?.details?.enabled === true) {
        expect(runtimeVerify.details.browser_proof, JSON.stringify(runtimeVerify, null, 2)).toBe(true)
      }
    }
  })

  test('billing plans endpoint is customer-ready', async ({ request }) => {
    const stamp = Date.now()
    const username = `launchsmoke${stamp}`
    const email = `${username}@example.com`
    const password = 'Passw0rd!Passw0rd!'

    const registerResponse = await request.post(`${apiV1Base}/auth/register`, {
      data: {
        username,
        email,
        password,
        full_name: 'Launch Smoke',
        accept_legal_terms: true,
      },
    })
    expect(registerResponse.ok()).toBeTruthy()

    const registerCookie = registerResponse.headersArray()
      .filter((header) => header.name.toLowerCase() === 'set-cookie')
      .map((header) => header.value.split(';')[0])
      .join('; ')

    const response = await request.get(`${apiV1Base}/billing/plans`, {
      headers: registerCookie ? { cookie: registerCookie } : undefined,
    })
    expect(response.status()).toBe(200)

    const body = await response.json()
    expect(body.success).toBe(true)
    expect(Array.isArray(body.data?.plans)).toBe(true)

    const plans = body.data?.plans || []
    const planTypes = new Set(plans.map((plan: { type: string }) => plan.type))
    expect(planTypes.has('free')).toBe(true)
    expect(planTypes.has('builder')).toBe(true)
    expect(planTypes.has('pro')).toBe(true)
    expect(planTypes.has('team')).toBe(true)

    if (expectLiveStripe) {
      const selfServePaidPlans = plans.filter((plan: { type: string }) =>
        ['builder', 'pro', 'team'].includes(plan.type)
      )
      for (const plan of selfServePaidPlans) {
        expect(typeof plan.monthly_price_id).toBe('string')
        expect(plan.monthly_price_id.length).toBeGreaterThan(0)
        expect(placeholderStripePriceIds.has(plan.monthly_price_id)).toBe(false)
      }
    }
  })

  test('optional authenticated launch login succeeds', async ({ page }) => {
    test.skip(!launchUsername || !launchPassword, 'Set PLAYWRIGHT_LAUNCH_USERNAME and PLAYWRIGHT_LAUNCH_PASSWORD to enable the authenticated launch smoke.')

    await page.goto('/settings')
    await page.locator('input[placeholder="Username"]').fill(launchUsername)
    await page.locator('input[placeholder="Password"]').fill(launchPassword)
    await page.getByRole('button', { name: 'Sign In' }).click()

    await expect(page.getByText('Build App').first()).toBeVisible()
    await expect(page.getByRole('button', { name: /New Build/i })).toBeVisible()
  })
})
