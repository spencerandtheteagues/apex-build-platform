import { defineConfig, devices } from '@playwright/test';

const baseURL = process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:5180';
const apiBaseURL = process.env.PLAYWRIGHT_API_URL || 'http://localhost:8080';
const includeFirefox = process.env.CI === 'true' || process.env.PLAYWRIGHT_INCLUDE_FIREFOX === 'true';

const projects = [
  { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
  { name: 'mobile', use: { ...devices['Pixel 5'] } },
];

if (includeFirefox) {
  projects.splice(1, 0, { name: 'firefox', use: { ...devices['Desktop Firefox'] } });
}

export default defineConfig({
  testDir: './specs/generated',
  fullyParallel: true,
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
  projects,
});
