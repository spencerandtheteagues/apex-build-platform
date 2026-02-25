import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(__dirname, '..');
const appsDir = path.join(root, 'apps');
const outDir = path.join(root, 'specs', 'generated');

fs.mkdirSync(outDir, { recursive: true });

const manifestFiles = fs.readdirSync(appsDir).filter((f) => f.endsWith('.json')).sort();
let generatedCount = 0;

for (const file of manifestFiles) {
  const manifestPath = path.join(appsDir, file);
  const manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf8'));
  if (!isManifestEnabled(manifest)) {
    continue;
  }
  const id = manifest.id || path.basename(file, '.json');
  const routes = Array.isArray(manifest.routes) ? manifest.routes : [];
  const healthChecks = Array.isArray(manifest.healthChecks) ? manifest.healthChecks : [];
  const assertions = Array.isArray(manifest.uiAssertions) ? manifest.uiAssertions : [];
  const authFlows = Array.isArray(manifest.authFlows) ? manifest.authFlows : [];
  const previewProxyScenarios = Array.isArray(manifest.previewProxyScenarios) ? manifest.previewProxyScenarios : [];
  const preflightScenarios = Array.isArray(manifest.preflightScenarios) ? manifest.preflightScenarios : [];
  const buildErrorScenarios = Array.isArray(manifest.buildErrorScenarios) ? manifest.buildErrorScenarios : [];

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

  const authFlowTests = authFlows.map((flow, idx) => {
    const flowName = flow.name || `auth-flow-${idx + 1}`;
    const register = flow.register !== false;
    const login = flow.login !== false;
    const password = String(flow.password || 'Password123!');
    return `
  test('auth: ${escapeTs(flowName)}', async ({ request }) => {
    const apiBase = process.env.PLAYWRIGHT_API_URL || 'http://localhost:8080';
    const suffix = \`\${Date.now()}-\${Math.floor(Math.random() * 100000)}\`;
    const username = ${JSON.stringify(flow.usernamePrefix || 'pw_user_')} + suffix;
    const email = username + '@example.com';
    const password = ${JSON.stringify(password)};

    ${register ? `const registerRes = await request.post(apiBase + '/api/v1/auth/register', {
      data: { username, email, password, full_name: 'Playwright E2E' }
    });
    expect([200, 201, 409]).toContain(registerRes.status());` : ''}

    ${login ? `const loginRes = await request.post(apiBase + '/api/v1/auth/login', {
      data: { username, password }
    });
    expect(loginRes.ok()).toBeTruthy();
    const loginBody = await loginRes.json();
    const accessToken = loginBody?.data?.tokens?.access_token ?? loginBody?.tokens?.access_token;
    expect(typeof accessToken).toBe('string');
    expect(accessToken.length).toBeGreaterThan(20);` : ''}
  });`;
  }).join('\n');

  const previewProxyTests = previewProxyScenarios.map((scenario, idx) => {
    const scenarioName = scenario.name || `preview-proxy-${idx + 1}`;
    const project = scenario.project || {};
    const files = Array.isArray(scenario.files) ? scenario.files : [];
    const preview = scenario.preview || {};
    const server = scenario.server || {};
    const proxyAssertions = Array.isArray(scenario.proxyAssertions) ? scenario.proxyAssertions : [];
    const websocketAssertions = Array.isArray(scenario.websocketAssertions) ? scenario.websocketAssertions : [];

    const filePayloads = files.map((f) => ({
      name: f.name || path.basename(f.path || 'file.txt'),
      path: f.path || '',
      type: f.type || 'file',
      content: f.content || '',
      mime_type: f.mimeType,
    }));

    const proxyAssertionSteps = proxyAssertions.map((a, i) => {
      const method = String(a.method || 'GET').toUpperCase();
      const p = String(a.path || '/');
      const expectStatus = Number.isInteger(a.expectStatus) ? a.expectStatus : 200;
      const bodyIncludes = a.bodyIncludes != null ? String(a.bodyIncludes) : '';
      const jsonPath = a.jsonPath != null ? String(a.jsonPath) : '';
      const jsonEquals = a.jsonEquals;
      const bodyCheck = bodyIncludes
        ? `expect(text${i}).toContain(${JSON.stringify(bodyIncludes)});`
        : '';
      const jsonCheck = jsonPath
        ? `
      let parsed${i};
      try { parsed${i} = JSON.parse(text${i}); } catch { parsed${i} = null; }
      expect(parsed${i}).not.toBeNull();
      expect(readJsonPath(parsed${i}, ${JSON.stringify(jsonPath)})).toEqual(${JSON.stringify(jsonEquals)});`
        : '';
      return `
    {
      const res${i} = await request.fetch(joinProxyURLPath(proxyBase, ${JSON.stringify(p)}), {
        method: ${JSON.stringify(method)},
        headers: { Authorization: 'Bearer ' + token }
      });
      expect(res${i}.status()).toBe(${expectStatus});
      const text${i} = await res${i}.text();
      ${bodyCheck}
      ${jsonCheck}
    }`;
    }).join('\n');

    const websocketSteps = websocketAssertions.map((a, i) => {
      const wsPath = String(a.path || '/ws');
      const timeoutMs = Number.isInteger(a.timeoutMs) ? a.timeoutMs : 3000;
      const expectOpen = a.expectOpen !== false;
      return `
    {
      const wsURL${i} = toWebSocketURL(joinProxyURLPath(proxyBase, ${JSON.stringify(wsPath)}));
      const result${i} = await page.evaluate(async ({ url, timeoutMs }) => {
        return await new Promise((resolve) => {
          let settled = false;
          const done = (value) => {
            if (settled) return;
            settled = true;
            resolve(value);
          };
          try {
            const ws = new WebSocket(url);
            const timer = setTimeout(() => done({ opened: false, reason: 'timeout' }), timeoutMs);
            ws.onopen = () => {
              clearTimeout(timer);
              try { ws.close(); } catch {}
              done({ opened: true, reason: 'open' });
            };
            ws.onerror = () => {
              clearTimeout(timer);
              done({ opened: false, reason: 'error' });
            };
          } catch (err) {
            done({ opened: false, reason: String(err) });
          }
        });
      }, { url: wsURL${i}, timeoutMs: ${timeoutMs} });
      expect(result${i}.opened).toBe(${expectOpen ? 'true' : 'false'});
    }`;
    }).join('\n');

    return `
  test('preview-proxy: ${escapeTs(scenarioName)}', async ({ request, page }) => {
    const apiBase = process.env.PLAYWRIGHT_API_URL || 'http://localhost:8080';
    const suffix = \`\${Date.now()}-\${Math.floor(Math.random() * 100000)}\`;
    const username = ${JSON.stringify((scenario.auth && scenario.auth.usernamePrefix) || 'pw_preview_')} + suffix;
    const email = username + '@example.com';
    const password = ${JSON.stringify((scenario.auth && scenario.auth.password) || 'Password123!')};

    const registerRes = await request.post(apiBase + '/api/v1/auth/register', {
      data: { username, email, password, full_name: 'Playwright Preview' }
    });
    expect([200, 201]).toContain(registerRes.status());
    const registerBody = await registerRes.json();
    const token = registerBody?.data?.tokens?.access_token ?? registerBody?.tokens?.access_token;
    expect(typeof token).toBe('string');

    const projectRes = await request.post(apiBase + '/api/v1/projects', {
      headers: { Authorization: 'Bearer ' + token },
      data: {
        name: ${JSON.stringify(project.name || 'PW Preview Project')},
        description: ${JSON.stringify(project.description || 'Generated by preview proxy scenario')},
        language: ${JSON.stringify(project.language || 'javascript')},
        framework: ${JSON.stringify(project.framework || '')}
      }
    });
    expect(projectRes.ok()).toBeTruthy();
    const projectBody = await projectRes.json();
    const projectId = projectBody?.project?.id;
    expect(typeof projectId).toBe('number');

    const files = ${JSON.stringify(filePayloads)};
    for (const f of files) {
      const res = await request.post(apiBase + '/api/v1/projects/' + projectId + '/files', {
        headers: { Authorization: 'Bearer ' + token },
        data: f
      });
      expect([200, 201]).toContain(res.status());
    }

    const previewStartRes = await request.post(apiBase + '/api/v1/preview/fullstack/start', {
      headers: { Authorization: 'Bearer ' + token },
      data: {
        project_id: projectId,
        entry_point: ${JSON.stringify(preview.entryPoint || '')},
        framework: ${JSON.stringify(preview.framework || '')},
        sandbox: ${Boolean(preview.sandbox)},
        start_backend: ${server.disabled ? 'false' : 'true'},
        require_backend: ${Boolean(server.requireBackend)},
        backend_entry_file: ${JSON.stringify(server.entryFile || '')},
        backend_command: ${JSON.stringify(server.command || '')}
      }
    });
    ${server.requireBackend ? 'expect(previewStartRes.ok()).toBeTruthy();' : 'expect([200, 502]).toContain(previewStartRes.status());'}
    const previewBody = await previewStartRes.json();
    const proxyBase = previewBody.proxy_url || previewBody?.preview?.url || (apiBase + '/api/v1/preview/proxy/' + projectId);
    expect(typeof proxyBase).toBe('string');

    ${proxyAssertionSteps}
    ${websocketSteps}
  });`;
  }).join('\n');

  const preflightTests = preflightScenarios.map((scenario, idx) => {
    const scenarioName = scenario.name || `preflight-${idx + 1}`;
    const expectReady = scenario.expectReady !== false;
    const minProviders = Number.isInteger(scenario.expectMinProviders) ? scenario.expectMinProviders : 0;
    return `
  test('preflight: ${escapeTs(scenarioName)}', async ({ request }) => {
    const apiBase = process.env.PLAYWRIGHT_API_URL || 'http://localhost:8080';
    const suffix = \`\${Date.now()}-\${Math.floor(Math.random() * 100000)}\`;
    const username = 'pw_preflight_' + suffix;
    const email = username + '@example.com';

    const registerRes = await request.post(apiBase + '/api/v1/auth/register', {
      data: { username, email, password: 'Preflight123!', full_name: 'Playwright Preflight' }
    });
    expect([200, 201]).toContain(registerRes.status());
    const registerBody = await registerRes.json();
    const token = registerBody?.data?.tokens?.access_token ?? registerBody?.tokens?.access_token;
    expect(typeof token).toBe('string');

    const preflightRes = await request.post(apiBase + '/api/v1/build/preflight', {
      headers: { Authorization: 'Bearer ' + token }
    });
    expect(preflightRes.ok()).toBeTruthy();
    const body = await preflightRes.json();
    expect(body.ready).toBe(${expectReady});
    ${minProviders > 0 ? `expect(body.providers_available).toBeGreaterThanOrEqual(${minProviders});` : ''}
  });`;
  }).join('\n');

  const buildErrorTests = buildErrorScenarios.map((scenario, idx) => {
    const scenarioName = scenario.name || `build-error-${idx + 1}`;
    const description = scenario.description != null ? String(scenario.description) : '';
    const expectStatus = Number.isInteger(scenario.expectStatus) ? scenario.expectStatus : 400;
    const expectErrorContains = scenario.expectErrorContains != null ? String(scenario.expectErrorContains) : '';
    return `
  test('build-error: ${escapeTs(scenarioName)}', async ({ request }) => {
    const apiBase = process.env.PLAYWRIGHT_API_URL || 'http://localhost:8080';
    const suffix = \`\${Date.now()}-\${Math.floor(Math.random() * 100000)}\`;
    const username = 'pw_builderr_' + suffix;
    const email = username + '@example.com';

    const registerRes = await request.post(apiBase + '/api/v1/auth/register', {
      data: { username, email, password: 'BuildErr123!', full_name: 'Playwright BuildErr' }
    });
    expect([200, 201]).toContain(registerRes.status());
    const registerBody = await registerRes.json();
    const token = registerBody?.data?.tokens?.access_token ?? registerBody?.tokens?.access_token;
    expect(typeof token).toBe('string');

    const buildRes = await request.post(apiBase + '/api/v1/build/start', {
      headers: { Authorization: 'Bearer ' + token },
      data: { description: ${JSON.stringify(description)} }
    });
    expect(buildRes.status()).toBe(${expectStatus});
    ${expectErrorContains ? `const body = await buildRes.json();
    const errorText = JSON.stringify(body).toLowerCase();
    expect(errorText).toContain(${JSON.stringify(expectErrorContains.toLowerCase())});` : ''}
  });`;
  }).join('\n');

  const helperBlock = previewProxyScenarios.length > 0 ? `
function joinProxyURLPath(baseURL, routePath) {
  const normalizedPath = routePath.startsWith('/') ? routePath : '/' + routePath;
  try {
    const url = new URL(baseURL);
    url.pathname = url.pathname.replace(/\\/$/, '') + normalizedPath;
    return url.toString();
  } catch {
    return baseURL.replace(/\\/$/, '') + normalizedPath;
  }
}

function toWebSocketURL(httpURL) {
  if (httpURL.startsWith('https://')) return 'wss://' + httpURL.slice('https://'.length);
  if (httpURL.startsWith('http://')) return 'ws://' + httpURL.slice('http://'.length);
  return httpURL;
}

function readJsonPath(obj, dottedPath) {
  return dottedPath.split('.').reduce((acc, key) => (acc == null ? undefined : acc[key]), obj);
}

` : '';

  const source = `import { test, expect } from '@playwright/test';

// AUTO-GENERATED FILE. Source manifest: apps/${file}

${helperBlock}test.describe('${escapeTs(manifest.name || id)} (${id})', () => {${healthTests}${routeTests}${uiTests}${authFlowTests}${preflightTests}${buildErrorTests}${previewProxyTests}
});
`;

  const outFile = path.join(outDir, `${id}.smoke.spec.ts`);
  fs.writeFileSync(outFile, source, 'utf8');
  generatedCount += 1;
}

console.log(`Generated ${generatedCount} app spec file(s) in ${path.relative(process.cwd(), outDir)}`);

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

function isManifestEnabled(manifest) {
  const envs = manifest.enabledWhenEnv;
  if (!envs) return true;
  const keys = Array.isArray(envs) ? envs : [envs];
  return keys.every((key) => {
    if (!key) return true;
    return Boolean(process.env[String(key)]);
  });
}
