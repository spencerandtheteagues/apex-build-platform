import { FullConfig } from '@playwright/test';

/**
 * Global teardown for Playwright tests
 * Runs once after all tests
 */
async function globalTeardown(config: FullConfig) {
  console.log('\n[Global Teardown] APEX.BUILD E2E test suite completed\n');

  // Clean up any test artifacts if needed
  // Add cleanup logic here if necessary
}

export default globalTeardown;
