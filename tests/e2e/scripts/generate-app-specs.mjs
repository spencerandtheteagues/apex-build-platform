import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(__dirname, '..');
const appsDir = path.join(root, 'apps');
const outDir = path.join(root, 'specs', 'generated');

fs.mkdirSync(outDir, { recursive: true });

const manifestFiles = fs.readdirSync(appsDir).filter((f) => f.endsWith('.json')).sort();

for (const file of manifestFiles) {
  const manifestPath = path.join(appsDir, file);
  const manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf8'));
  const id = manifest.id || path.basename(file, '.json');
  const routes = Array.isArray(manifest.routes) ? manifest.routes : [];
  const healthChecks = Array.isArray(manifest.healthChecks) ? manifest.healthChecks : [];
  const assertions = Array.isArray(manifest.uiAssertions) ? manifest.uiAssertions : [];

  const routeTests = routes.map((route) => {
    const pathValue = route.path || '/';
    const testName = route.name || pathValue;
    const waitFor = route.waitFor || 'body';
    return `
  test('route: ${escapeTs(testName)}', async ({ page }) => {
    await page.goto(${JSON.stringify(pathValue)});
    await page.locator(${JSON.stringify(waitFor)}).first().waitFor({ state: 'visible' });
    await expect(page).toHaveURL(/${escapeRegex(pathValue.replaceAll('/', '\\/') || '\\/')}/);
  });`;
  }).join('\n');

  const healthTests = healthChecks.map((check) => {
    const name = check.name || check.url;
    const status = Number.isInteger(check.expectStatus) ? check.expectStatus : 200;
    const targetExpr = compileURLExpression(String(check.url || ''));
    return `
  test('health: ${escapeTs(name)}', async ({ request }) => {
    const target = ${targetExpr};
    const res = await request.get(target);
    expect(res.status()).toBe(${status});
  });`;
  }).join('\n');

  const uiTests = assertions.map((a) => {
    const state = a.state || 'visible';
    const name = a.name || a.locator || 'assertion';
    const locator = a.locator || 'body';
    return `
  test('ui: ${escapeTs(name)}', async ({ page }) => {
    await page.goto('/');
    await page.locator(${JSON.stringify(locator)}).first().waitFor({ state: ${JSON.stringify(state)} });
  });`;
  }).join('\n');

  const source = `import { test, expect } from '@playwright/test';

// AUTO-GENERATED FILE. Source manifest: apps/${file}

test.describe('${escapeTs(manifest.name || id)} (${id})', () => {${healthTests}${routeTests}${uiTests}
});
`;

  const outFile = path.join(outDir, `${id}.smoke.spec.ts`);
  fs.writeFileSync(outFile, source, 'utf8');
}

console.log(`Generated ${manifestFiles.length} app spec file(s) in ${path.relative(process.cwd(), outDir)}`);

function escapeTs(value) {
  return String(value).replaceAll("'", "\\'");
}

function escapeRegex(value) {
  return String(value).replace(/[.*+?^${}()|[\\]\\\\]/g, '\\\\$&');
}

function compileURLExpression(raw) {
  const placeholder = '${PLAYWRIGHT_API_URL:-http://localhost:8080}';
  if (!raw.includes(placeholder)) {
    return JSON.stringify(raw);
  }

  const parts = raw.split(placeholder);
  const chunks = [];
  for (let i = 0; i < parts.length; i++) {
    if (i > 0) {
      chunks.push("(process.env.PLAYWRIGHT_API_URL || 'http://localhost:8080')");
    }
    if (parts[i]) {
      chunks.push(JSON.stringify(parts[i]));
    }
  }
  return chunks.join(' + ') || "(process.env.PLAYWRIGHT_API_URL || 'http://localhost:8080')";
}
