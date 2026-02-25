import { test, expect } from '@playwright/test';

// AUTO-GENERATED FILE. Source manifest: apps/apex-build-web.json

test.describe('APEX.BUILD Web (apex-build-web)', () => {
  test('health: backend-health', async ({ request }) => {
    const target = (process.env.PLAYWRIGHT_API_URL || 'http://localhost:8080') + "/health";
    const res = await request.get(target);
    expect(res.status()).toBe(200);
  });
  test('route: home', async ({ page }) => {
    await page.goto("/");
    await page.locator("body").first().waitFor({ state: 'visible' });
    await expect(page).toHaveURL(/\//);
  });

  test('route: auth-login', async ({ page }) => {
    await page.goto("/login");
    await page.locator("body").first().waitFor({ state: 'visible' });
    await expect(page).toHaveURL(/\/login/);
  });
  test('ui: app-shell-renders', async ({ page }) => {
    await page.goto('/');
    await page.locator("body").first().waitFor({ state: "visible" });
  });
});
