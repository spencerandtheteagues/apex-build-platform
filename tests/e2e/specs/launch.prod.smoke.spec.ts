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

test.describe('Launch readiness smoke', () => {
  test('landing exposes real public resource links', async ({ page }) => {
    await page.goto('/')

    await expect(page.getByRole('button', { name: /Get Started Free|Start Building Free/i }).first()).toBeVisible()

    await expect(page.getByRole('link', { name: 'Privacy' })).toHaveAttribute('href', '/?legal=privacy')
    await expect(page.getByRole('link', { name: 'Terms' })).toHaveAttribute('href', '/?legal=terms')
    await expect(page.getByRole('link', { name: 'Docs' })).toHaveAttribute('href', '/?help=1')

    const statusLink = page.getByRole('link', { name: 'Status' })
    await expect(statusLink).toHaveAttribute('href', /health\/features$/)
    await expect(statusLink).not.toHaveAttribute('href', '#')
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
    await expect(page.getByRole('button', { name: 'Terms of Service' })).toBeVisible()
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
  })

  test('billing plans endpoint is customer-ready', async ({ request }) => {
    const response = await request.get(`${apiV1Base}/billing/plans`)
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
      const paidPlans = plans.filter((plan: { type: string }) => plan.type !== 'free')
      for (const plan of paidPlans) {
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
