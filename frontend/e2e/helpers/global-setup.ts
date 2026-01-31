import { FullConfig } from '@playwright/test';
import fs from 'fs';
import path from 'path';

/**
 * Global setup for Playwright tests
 * Runs once before all tests
 */
async function globalSetup(config: FullConfig) {
  console.log('\n[Global Setup] Starting APEX.BUILD E2E test suite...\n');

  // Create necessary directories
  const dirs = [
    path.join(__dirname, '..', 'playwright', '.auth'),
    path.join(__dirname, '..', 'test-results'),
    path.join(__dirname, '..', 'playwright-report'),
  ];

  for (const dir of dirs) {
    if (!fs.existsSync(dir)) {
      fs.mkdirSync(dir, { recursive: true });
      console.log(`[Global Setup] Created directory: ${dir}`);
    }
  }

  // Set up environment variables
  process.env.PLAYWRIGHT_BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:5173';
  process.env.PLAYWRIGHT_API_URL = process.env.PLAYWRIGHT_API_URL || 'http://localhost:8080/api/v1';

  // Wait for services to be ready if needed
  if (process.env.CI) {
    console.log('[Global Setup] Running in CI environment');
    // Add any CI-specific setup here
  }

  console.log('[Global Setup] Environment configured successfully');
  console.log(`[Global Setup] Base URL: ${process.env.PLAYWRIGHT_BASE_URL}`);
  console.log(`[Global Setup] API URL: ${process.env.PLAYWRIGHT_API_URL}`);
}

export default globalSetup;
