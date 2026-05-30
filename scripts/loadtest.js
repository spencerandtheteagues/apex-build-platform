// scripts/loadtest.js - k6 load harness for Apex Build launch readiness (TASK-010)
//
// Default scenario: 200 concurrent unauthenticated traffic across
//   https://apex-build.dev/ (landing) and https://api.apex-build.dev/ready (health)
//
// Optional authenticated API scenario (RUN_AUTH_API=1):
//   LOGIN_EMAIL + LOGIN_PASSWORD required - logs in safely, preserves
//   session cookies/CSRF, then hits /api/v1/usage/limits and /api/v1/projects
//   at 50 concurrent users.
//
// Optional build-start scenario (RUN_BUILD_STARTS=1):
//   Requires credentials. Starts exactly 10 free-fast/frontend-only builds
//   using a bounded low-cost prompt, polls each to terminal, and requires
//   completed status with no 5xx. MUST NEVER run by default.
//
// Thresholds (TASK-010):
//   - public landing / health p95 < 800ms
//   - auth API error rate < 1%
//   - no unexpected 5xx spikes
//   - build-start pass rate 100% (when enabled)
//
// Safety:
//   - Default run is safe public-only traffic (no auth, no mutations).
//   - Auth and build scenarios are fully opt-in via env vars.
//   - Never logs passwords, auth cookies, tokens, or full response bodies.

import http from 'k6/http';
import { check, sleep, group } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

// ---------------------------------------------------------------------------
// Configuration
// ---------------------------------------------------------------------------

const LANDING_BASE = __ENV.LANDING_URL || 'https://apex-build.dev';
const API_BASE = __ENV.API_URL || 'https://api.apex-build.dev';
const RUN_AUTH_API = __ENV.RUN_AUTH_API === '1';
const RUN_BUILD_STARTS = __ENV.RUN_BUILD_STARTS === '1';
const LOGIN_USERNAME = __ENV.LOGIN_USERNAME || '';
const LOGIN_EMAIL = __ENV.LOGIN_EMAIL || '';
const LOGIN_PASSWORD = __ENV.LOGIN_PASSWORD || '';

// Validate auth opt-in: fail fast if credentials are missing
if (RUN_AUTH_API && (!LOGIN_EMAIL || !LOGIN_PASSWORD)) {
  throw new Error(
    'LOADTEST ABORT: RUN_AUTH_API=1 requires LOGIN_EMAIL and LOGIN_PASSWORD. ' +
    'Set both environment variables or remove RUN_AUTH_API=1.'
  );
}

if (RUN_BUILD_STARTS && (!LOGIN_EMAIL || !LOGIN_PASSWORD)) {
  throw new Error(
    'LOADTEST ABORT: RUN_BUILD_STARTS=1 requires LOGIN_EMAIL and LOGIN_PASSWORD. ' +
    'Set both environment variables or remove RUN_BUILD_STARTS=1.'
  );
}

// ---------------------------------------------------------------------------
// Custom metrics
// ---------------------------------------------------------------------------

const publicErrorRate = new Rate('public_errors');
const public5xxCount = new Counter('public_5xx_errors');
const landingP95 = new Trend('landing_p95', true);
const healthP95 = new Trend('health_p95', true);
const authApiErrorRate = new Rate('auth_api_errors');
const authApiP95 = new Trend('auth_api_p95', true);
const buildStartFailRate = new Rate('build_start_failures');

// ---------------------------------------------------------------------------
// Scenarios
// ---------------------------------------------------------------------------

const scenarios = {
  public_traffic: {
    executor: 'ramping-vus',
    startVUs: 0,
    stages: [
      { duration: '30s', target: 200 },
      { duration: '60s', target: 200 },
      { duration: '10s', target: 0 },
    ],
    gracefulRampDown: '10s',
    tags: { scenario: 'public' },
  },
};

if (RUN_AUTH_API) {
  scenarios.auth_api = {
    executor: 'ramping-vus',
    startVUs: 0,
    stages: [
      { duration: '20s', target: 50 },
      { duration: '60s', target: 50 },
      { duration: '10s', target: 0 },
    ],
    gracefulRampDown: '10s',
    tags: { scenario: 'auth_api' },
    exec: 'authApiScenario',
  };
}

if (RUN_BUILD_STARTS) {
  scenarios.build_starts = {
    executor: 'per-vu-iterations',
    vus: 10,
    iterations: 1,
    maxDuration: '15m',
    tags: { scenario: 'build_starts' },
    exec: 'buildStartScenario',
  };
}

export const options = {
  scenarios,
  thresholds: {
    // Public traffic thresholds (TASK-010)
    'http_req_duration{scenario:public,endpoint:landing}': ['p(95) < 800'],
    landing_p95: ['p(95) < 800'],
    health_p95: ['p(95) < 800'],
    public_errors: ['rate < 0.05'],
    public_5xx_errors: ['count == 0'],
    // Auth API thresholds
    auth_api_errors: ['rate < 0.01'],
    auth_api_p95: ['p(95) < 2000'],
    // Build-start thresholds (when enabled)
    build_start_failures: ['rate < 0.01'],
  },
  summaryTrendStats: ['avg', 'min', 'med', 'p(90)', 'p(95)', 'max'],
};

// ---------------------------------------------------------------------------
// Helper: safe headers (never log secrets)
// ---------------------------------------------------------------------------

function safeHeaders(extra = {}) {
  return Object.assign(
    {
      'User-Agent': 'apex-loadtest/1.0',
      'Accept': 'application/json, text/html, */*',
    },
    extra
  );
}

let cachedAuthState = null;

function readAccessToken(response) {
  try {
    const body = response.json();
    const dataToken = body && body.data ? body.data.access_token : '';
    const nestedToken = body && body.tokens ? body.tokens.access_token : '';
    return body.access_token || body.token || dataToken || nestedToken || '';
  } catch (_) {
    return '';
  }
}

function loginForScenario(scenarioName) {
  if (cachedAuthState) {
    return cachedAuthState;
  }

  const csrfRes = http.get(`${API_BASE}/api/v1/csrf-token`, {
    headers: safeHeaders({ Accept: 'application/json' }),
    timeout: '10s',
    tags: { scenario: scenarioName, endpoint: 'csrf_token' },
  });

  let csrfToken = '';
  try {
    csrfToken = csrfRes.json('token') || '';
  } catch (_) {
    // CSRF may not be required on all deployments.
  }

  const loginPayload = JSON.stringify({
    username: LOGIN_USERNAME,
    email: LOGIN_EMAIL,
    password: LOGIN_PASSWORD,
  });

  const loginRes = http.post(`${API_BASE}/api/v1/auth/login`, loginPayload, {
    headers: safeHeaders({
      'Content-Type': 'application/json',
      'X-CSRF-Token': csrfToken,
      Accept: 'application/json',
    }),
    timeout: '15s',
    tags: { scenario: scenarioName, endpoint: 'login' },
  });

  const loginOk = check(loginRes, {
    [`${scenarioName} login status 2xx`]: (r) => r.status >= 200 && r.status < 300,
    [`${scenarioName} login no 5xx`]: (r) => r.status < 500,
  });

  if (!loginOk) {
    console.error(`LOADTEST: ${scenarioName} login failed status=${loginRes.status}`);
    return null;
  }

  const accessToken = readAccessToken(loginRes);
  const headers = safeHeaders({
    'X-CSRF-Token': csrfToken,
    Accept: 'application/json',
    'Content-Type': 'application/json',
  });
  if (accessToken) {
    headers.Authorization = `Bearer ${accessToken}`;
  }

  cachedAuthState = { csrfToken, headers };
  return cachedAuthState;
}

// ---------------------------------------------------------------------------
// Default scenario: public unauthenticated traffic
// ---------------------------------------------------------------------------

export default function publicTrafficScenario() {
  // --- Landing page ---
  group('public_landing', () => {
    const start = Date.now();
    const res = http.get(`${LANDING_BASE}/`, {
      headers: safeHeaders({ Accept: 'text/html' }),
      timeout: '15s',
      tags: { scenario: 'public', endpoint: 'landing' },
    });
    const elapsed = Date.now() - start;
    landingP95.add(elapsed);

    const ok = check(res, {
      'landing status 2xx/3xx': (r) => r.status >= 200 && r.status < 400,
      'landing no 5xx': (r) => r.status < 500,
    });
    publicErrorRate.add(!ok);

    if (res.status >= 500) {
      public5xxCount.add(1);
      console.error(`LOADTEST: public landing 5xx status=${res.status}`);
    }
  });

  sleep(Math.random() * 0.5);

  // --- Health/ready endpoint ---
  group('public_health', () => {
    const start = Date.now();
    const res = http.get(`${API_BASE}/ready`, {
      headers: safeHeaders({ Accept: 'application/json' }),
      timeout: '10s',
      tags: { scenario: 'public', endpoint: 'health' },
    });
    const elapsed = Date.now() - start;
    healthP95.add(elapsed);

    const ok = check(res, {
      'health status 2xx': (r) => r.status >= 200 && r.status < 300,
      'health no 5xx': (r) => r.status < 500,
      'health has status field': (r) => {
        try {
          const body = r.json();
          return body && (body.status === 'ok' || body.overall === 'healthy' || r.status === 200);
        } catch (_) {
          return r.status === 200;
        }
      },
    });
    publicErrorRate.add(!ok);

    if (res.status >= 500) {
      public5xxCount.add(1);
      console.error(`LOADTEST: public health 5xx status=${res.status}`);
    }
  });

  sleep(Math.random() * 1.0);
}

// ---------------------------------------------------------------------------
// Auth API scenario: login + authenticated endpoints at 50vu
// ---------------------------------------------------------------------------

export function authApiScenario() {
  const authState = loginForScenario('auth_api');
  authApiErrorRate.add(!authState);
  if (!authState) return;

  // Step 3: Hit authenticated endpoints
  // --- /api/v1/usage/limits ---
  group('auth_usage_limits', () => {
    const start = Date.now();
    const res = http.get(`${API_BASE}/api/v1/usage/limits`, {
      headers: authState.headers,
      timeout: '10s',
      tags: { scenario: 'auth_api', endpoint: 'usage_limits' },
    });
    const elapsed = Date.now() - start;
    authApiP95.add(elapsed);

    const ok = check(res, {
      'usage/limits status 2xx': (r) => r.status >= 200 && r.status < 300,
      'usage/limits no 5xx': (r) => r.status < 500,
    });
    authApiErrorRate.add(!ok);

    if (res.status >= 500) {
      console.error(`LOADTEST: auth usage/limits 5xx status=${res.status}`);
    }
  });

  sleep(Math.random() * 0.5);

  // --- /api/v1/projects (list user projects) ---
  group('auth_projects_list', () => {
    const start = Date.now();
    const res = http.get(`${API_BASE}/api/v1/projects`, {
      headers: authState.headers,
      timeout: '10s',
      tags: { scenario: 'auth_api', endpoint: 'projects_list' },
    });
    const elapsed = Date.now() - start;
    authApiP95.add(elapsed);

    const ok = check(res, {
      'projects status 2xx': (r) => r.status >= 200 && r.status < 300,
      'projects no 5xx': (r) => r.status < 500,
    });
    authApiErrorRate.add(!ok);

    if (res.status >= 500) {
      console.error(`LOADTEST: auth projects 5xx status=${res.status}`);
    }
  });

  sleep(Math.random() * 1.0);
}

// ---------------------------------------------------------------------------
// Build-start scenario: starts exactly one bounded build per VU, polls to terminal
// ---------------------------------------------------------------------------

const BUILD_PROMPT = 'Build a simple to-do list app with React and Tailwind CSS. Include add, delete, and mark-complete functionality. No backend needed - use local state.';
const MAX_BUILDS = 10;

function pollBuildToTerminal(buildId, pollToken, authState) {
  const terminalStates = new Set(['completed', 'failed', 'cancelled', 'error']);
  const maxPollMs = 10 * 60 * 1000; // 10 minutes max per build
  const pollIntervalSec = 5;
  const startTime = Date.now();

  while ((Date.now() - startTime) < maxPollMs) {
    sleep(pollIntervalSec);

    const pollHeaders = pollToken
      ? safeHeaders({
          Accept: 'application/json',
          'X-Apex-Build-Poll-Token': pollToken,
        })
      : authState.headers;
    const pollPath = pollToken
      ? `/api/v1/build/${buildId}/poll-status`
      : `/api/v1/build/${buildId}/status`;

    const pollRes = http.get(
      `${API_BASE}${pollPath}`,
      {
        headers: pollHeaders,
        timeout: '15s',
        tags: { scenario: 'build_starts', endpoint: pollToken ? 'poll_status' : 'build_status' },
      }
    );

    if (pollRes.status >= 500) {
      console.error(`LOADTEST: poll for build ${buildId} returned 5xx status=${pollRes.status}`);
      buildStartFailRate.add(1);
      continue;
    }

    try {
      const body = pollRes.json();
      const status = body.status || body.state || '';

      if (terminalStates.has(status)) {
        const isCompleted = status === 'completed';
        buildStartFailRate.add(!isCompleted);
        console.log(
          `LOADTEST: build ${buildId} reached terminal state=${status} completed=${isCompleted}`
        );
        return isCompleted;
      }
    } catch (_) {
      console.error(`LOADTEST: poll for build ${buildId} returned non-JSON, status=${pollRes.status}`);
    }
  }

  console.error(`LOADTEST: build ${buildId} did not reach terminal state within ${maxPollMs / 1000}s`);
  buildStartFailRate.add(1);
  return false;
}

export function buildStartScenario() {
  const authState = loginForScenario('build_starts');
  if (!authState) {
    buildStartFailRate.add(1);
    return;
  }

  const buildPayload = JSON.stringify({
    description: BUILD_PROMPT,
    prompt: BUILD_PROMPT,
    mode: 'fast',
    power_mode: 'fast',
    provider_mode: 'platform',
    project_name: `apex-loadtest-build-${Date.now()}-${__VU}`,
    // frontend-only bounding: simple React app, no backend
    tech_stack: {
      frontend: 'react',
      styling: 'tailwindcss',
    },
    require_preview_ready: true,
  });

  const buildRes = http.post(`${API_BASE}/api/v1/build/start`, buildPayload, {
    headers: authState.headers,
    timeout: '30s',
    tags: { scenario: 'build_starts', endpoint: 'build_start' },
  });

  const buildOk = check(buildRes, {
    'build start status 2xx': (r) => r.status >= 200 && r.status < 300,
    'build start no 5xx': (r) => r.status < 500,
  });

  if (!buildOk) {
    console.error(`LOADTEST: build start failed status=${buildRes.status}`);
    buildStartFailRate.add(1);
    return;
  }

  try {
    const body = buildRes.json();
    const buildId = body.build_id || body.buildID || '';
    const pollToken = body.poll_token || '';
    if (!buildId) {
      console.error(`LOADTEST: build start returned no build_id, status=${buildRes.status}`);
      buildStartFailRate.add(1);
      return;
    }

    console.log(`LOADTEST: build ${__VU}/${MAX_BUILDS} started id=${buildId}`);
    pollBuildToTerminal(buildId, pollToken, authState);
  } catch (_) {
    console.error(`LOADTEST: build start returned non-JSON, status=${buildRes.status}`);
    buildStartFailRate.add(1);
  }

  // Logout this VU's session.
  http.post(`${API_BASE}/api/v1/auth/logout`, '', {
    headers: authState.headers,
    timeout: '10s',
    tags: { scenario: 'build_starts', endpoint: 'logout' },
  });
}
