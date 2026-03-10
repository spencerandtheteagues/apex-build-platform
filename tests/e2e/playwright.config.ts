import { defineConfig, devices } from '@playwright/test';

const baseURL = process.env.PLAYWRIGHT_BASE_URL || 'http://127.0.0.1:5180';
const apiBaseURL = process.env.PLAYWRIGHT_API_URL || 'http://127.0.0.1:8080';
const rawWorkers = Number(process.env.PLAYWRIGHT_WORKERS || '1');
const workers = Number.isFinite(rawWorkers) && rawWorkers > 0 ? rawWorkers : 1;
const includeFirefox = process.env.PLAYWRIGHT_INCLUDE_FIREFOX !== '0';

export default defineConfig({
  testDir: './specs',
  fullyParallel: false,
  workers,
  retries: process.env.CI ? 2 : 0,
  timeout: 60_000,
  expect: { timeout: 10_000 },
  reporter: [['list'], ['html', { outputFolder: 'playwright-report' }]],
  use: {
    baseURL,
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
    extraHTTPHeaders: {
      'x-apex-e2e-suite': 'tests-e2e',
      'x-apex-api-base-url': apiBaseURL,
    },
  },
  projects: [
    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
    ...(includeFirefox ? [{ name: 'firefox', use: { ...devices['Desktop Firefox'] } }] : []),
    { name: 'mobile', use: { ...devices['Pixel 5'] } },
  ],
});
