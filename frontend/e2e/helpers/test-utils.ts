import { Page, expect, Locator } from '@playwright/test';

/**
 * APEX.BUILD E2E Test Utilities
 * Common helper functions for testing
 */

// API URL configuration
export const API_URL = process.env.PLAYWRIGHT_API_URL || 'http://localhost:8080/api/v1';

/**
 * Wait for network idle state
 */
export async function waitForNetworkIdle(page: Page, timeout = 5000) {
  await page.waitForLoadState('networkidle', { timeout });
}

/**
 * Wait for element to be visible and stable
 */
export async function waitForStableElement(locator: Locator, timeout = 10000) {
  await locator.waitFor({ state: 'visible', timeout });
  // Wait for any animations to complete
  await locator.page().waitForTimeout(100);
}

/**
 * Retry an action until it succeeds
 */
export async function retryAction<T>(
  action: () => Promise<T>,
  maxRetries = 3,
  delay = 1000
): Promise<T> {
  let lastError: Error | null = null;

  for (let i = 0; i < maxRetries; i++) {
    try {
      return await action();
    } catch (error) {
      lastError = error as Error;
      if (i < maxRetries - 1) {
        await new Promise(resolve => setTimeout(resolve, delay));
      }
    }
  }

  throw lastError;
}

/**
 * Generate unique test data
 */
export function generateTestData() {
  const timestamp = Date.now();
  const randomSuffix = Math.random().toString(36).substring(7);

  return {
    username: `testuser_${timestamp}_${randomSuffix}`,
    email: `test_${timestamp}_${randomSuffix}@apex.test`,
    password: `TestPass123!${randomSuffix}`,
    projectName: `TestProject_${timestamp}`,
    fileName: `test_${timestamp}.ts`,
  };
}

/**
 * Wait for toast notification and verify message
 */
export async function waitForToast(page: Page, expectedMessage?: string, timeout = 5000) {
  const toast = page.locator('[role="alert"], [data-testid="toast"], .toast');
  await toast.first().waitFor({ state: 'visible', timeout });

  if (expectedMessage) {
    await expect(toast.first()).toContainText(expectedMessage);
  }

  return toast.first();
}

/**
 * Wait for loading overlay to disappear
 */
export async function waitForLoadingToComplete(page: Page, timeout = 30000) {
  const loadingOverlay = page.locator('[data-testid="loading-overlay"], .loading-overlay, [class*="LoadingOverlay"]');

  // Wait for it to appear first (if it does)
  try {
    await loadingOverlay.waitFor({ state: 'visible', timeout: 2000 });
    // Then wait for it to disappear
    await loadingOverlay.waitFor({ state: 'hidden', timeout });
  } catch {
    // Loading overlay may not appear for quick operations
  }
}

/**
 * Fill form input with proper clearing
 */
export async function fillInput(locator: Locator, value: string) {
  await locator.click();
  await locator.fill('');
  await locator.fill(value);
}

/**
 * Wait for API response
 */
export async function waitForApiResponse(
  page: Page,
  urlPattern: string | RegExp,
  options?: { timeout?: number; status?: number }
) {
  const response = await page.waitForResponse(
    response => {
      const urlMatch = typeof urlPattern === 'string'
        ? response.url().includes(urlPattern)
        : urlPattern.test(response.url());

      const statusMatch = options?.status
        ? response.status() === options.status
        : true;

      return urlMatch && statusMatch;
    },
    { timeout: options?.timeout || 30000 }
  );

  return response;
}

/**
 * Mock API endpoint
 */
export async function mockApiEndpoint(
  page: Page,
  urlPattern: string | RegExp,
  response: { status?: number; body?: any; headers?: Record<string, string> }
) {
  await page.route(urlPattern, async route => {
    await route.fulfill({
      status: response.status || 200,
      contentType: 'application/json',
      headers: response.headers,
      body: JSON.stringify(response.body),
    });
  });
}

/**
 * Take screenshot with timestamp
 */
export async function takeScreenshotWithTimestamp(page: Page, name: string) {
  const timestamp = new Date().toISOString().replace(/[:.]/g, '-');
  await page.screenshot({
    path: `test-results/screenshots/${name}_${timestamp}.png`,
    fullPage: true,
  });
}

/**
 * Scroll element into view and click
 */
export async function scrollAndClick(locator: Locator) {
  await locator.scrollIntoViewIfNeeded();
  await locator.click();
}

/**
 * Wait for Monaco editor to be ready
 */
export async function waitForMonacoEditor(page: Page, timeout = 30000) {
  // Wait for Monaco container
  await page.waitForSelector('.monaco-editor', { state: 'visible', timeout });

  // Wait for editor to be initialized
  await page.waitForFunction(() => {
    const editor = document.querySelector('.monaco-editor');
    return editor && !editor.classList.contains('loading');
  }, { timeout });

  // Give Monaco a moment to fully initialize
  await page.waitForTimeout(500);
}

/**
 * Type in Monaco editor
 */
export async function typeInMonacoEditor(page: Page, text: string) {
  await waitForMonacoEditor(page);

  // Focus the editor
  await page.click('.monaco-editor');

  // Type the text
  await page.keyboard.type(text, { delay: 50 });
}

/**
 * Get Monaco editor content
 */
export async function getMonacoEditorContent(page: Page): Promise<string> {
  await waitForMonacoEditor(page);

  const content = await page.evaluate(() => {
    // Access Monaco's model
    const editor = (window as any).monaco?.editor?.getEditors()[0];
    if (editor) {
      return editor.getValue();
    }

    // Fallback: get text content from DOM
    const lines = document.querySelectorAll('.monaco-editor .view-line');
    return Array.from(lines).map(line => line.textContent).join('\n');
  });

  return content || '';
}

/**
 * Wait for terminal to be ready
 */
export async function waitForTerminal(page: Page, timeout = 15000) {
  await page.waitForSelector('.xterm', { state: 'visible', timeout });
  await page.waitForTimeout(500); // Allow terminal to initialize
}

/**
 * Type command in terminal
 */
export async function typeInTerminal(page: Page, command: string) {
  await waitForTerminal(page);

  // Focus terminal
  await page.click('.xterm');

  // Type command
  await page.keyboard.type(command, { delay: 30 });
  await page.keyboard.press('Enter');
}

/**
 * Check if element has specific CSS class
 */
export async function hasClass(locator: Locator, className: string): Promise<boolean> {
  const classes = await locator.getAttribute('class');
  return classes?.includes(className) || false;
}

/**
 * Wait for navigation to complete
 */
export async function waitForNavigation(page: Page, urlPattern?: string | RegExp) {
  await page.waitForLoadState('domcontentloaded');

  if (urlPattern) {
    await page.waitForURL(urlPattern);
  }

  await waitForLoadingToComplete(page);
}

/**
 * Clear local storage and cookies
 */
export async function clearBrowserState(page: Page) {
  await page.evaluate(() => {
    localStorage.clear();
    sessionStorage.clear();
  });

  const context = page.context();
  await context.clearCookies();
}

/**
 * Debug helper - pause test and open inspector
 */
export async function debugPause(page: Page) {
  if (process.env.DEBUG) {
    await page.pause();
  }
}
