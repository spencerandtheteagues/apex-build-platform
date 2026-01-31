import { defineConfig, devices } from '@playwright/test';

/**
 * APEX.BUILD E2E Test Configuration
 * Comprehensive Playwright setup for testing the cloud development platform
 */

// Read environment variables
const baseURL = process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:5173';
const apiBaseURL = process.env.PLAYWRIGHT_API_URL || 'http://localhost:8080/api/v1';

export default defineConfig({
  // Test directory
  testDir: './tests',

  // Run tests in files in parallel
  fullyParallel: true,

  // Fail the build on CI if you accidentally left test.only in the source code
  forbidOnly: !!process.env.CI,

  // Retry on CI only
  retries: process.env.CI ? 2 : 0,

  // Opt out of parallel tests on CI (can cause flakiness)
  workers: process.env.CI ? 1 : undefined,

  // Reporter to use
  reporter: [
    ['html', { outputFolder: 'playwright-report' }],
    ['json', { outputFile: 'test-results/results.json' }],
    ['list'],
    ...(process.env.CI ? [['github' as const]] : []),
  ],

  // Shared settings for all projects
  use: {
    // Base URL for all tests
    baseURL,

    // Collect trace when retrying the failed test
    trace: 'on-first-retry',

    // Take screenshot on failure
    screenshot: 'only-on-failure',

    // Video recording on failure
    video: 'on-first-retry',

    // API base URL for custom API requests
    extraHTTPHeaders: {
      'Accept': 'application/json',
    },

    // Timeout for each action
    actionTimeout: 15000,

    // Timeout for navigation
    navigationTimeout: 30000,
  },

  // Global timeout
  timeout: 60000,

  // Expect timeout
  expect: {
    timeout: 10000,
  },

  // Configure projects for major browsers
  projects: [
    // Setup project for authentication
    {
      name: 'setup',
      testMatch: /.*\.setup\.ts/,
      teardown: 'cleanup',
    },

    // Cleanup project
    {
      name: 'cleanup',
      testMatch: /.*\.cleanup\.ts/,
    },

    // Desktop Chrome
    {
      name: 'chromium',
      use: {
        ...devices['Desktop Chrome'],
        storageState: 'playwright/.auth/user.json',
      },
      dependencies: ['setup'],
    },

    // Desktop Firefox
    {
      name: 'firefox',
      use: {
        ...devices['Desktop Firefox'],
        storageState: 'playwright/.auth/user.json',
      },
      dependencies: ['setup'],
    },

    // Desktop Safari
    {
      name: 'webkit',
      use: {
        ...devices['Desktop Safari'],
        storageState: 'playwright/.auth/user.json',
      },
      dependencies: ['setup'],
    },

    // Mobile Chrome
    {
      name: 'mobile-chrome',
      use: {
        ...devices['Pixel 5'],
        storageState: 'playwright/.auth/user.json',
      },
      dependencies: ['setup'],
    },

    // Mobile Safari
    {
      name: 'mobile-safari',
      use: {
        ...devices['iPhone 13'],
        storageState: 'playwright/.auth/user.json',
      },
      dependencies: ['setup'],
    },

    // Tests that need fresh authentication (no stored state)
    {
      name: 'auth-tests',
      testMatch: '**/auth/**/*.spec.ts',
      use: {
        ...devices['Desktop Chrome'],
        // No storageState - tests with fresh browser
      },
    },
  ],

  // Run your local dev server before starting the tests
  webServer: process.env.CI ? undefined : [
    {
      command: 'npm run dev',
      url: baseURL,
      reuseExistingServer: !process.env.CI,
      timeout: 120000,
    },
  ],

  // Global setup/teardown
  globalSetup: require.resolve('./helpers/global-setup'),
  globalTeardown: require.resolve('./helpers/global-teardown'),

  // Output directory
  outputDir: 'test-results/',
});
