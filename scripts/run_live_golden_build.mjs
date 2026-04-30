#!/usr/bin/env node
import fs from 'node:fs'
import path from 'node:path'
import { createRequire } from 'node:module'

const require = createRequire(import.meta.url)
const { chromium } = require('../tests/e2e/node_modules/@playwright/test')

const apiBase = (process.env.BASE_URL || 'https://api.apex-build.dev/api/v1').replace(/\/$/, '')
const promptPath = process.argv[2] || process.env.PROMPT_FILE || ''
const prompt = promptPath ? fs.readFileSync(promptPath, 'utf8') : process.env.PROMPT || ''
const mode = process.env.MODE || 'full'
const powerMode = process.env.POWER_MODE || 'balanced'
const projectName = process.env.PROJECT_NAME || `golden-${powerMode}-${Date.now()}`
const pollSeconds = Number(process.env.POLL_SECONDS || 15)
const maxPolls = Number(process.env.MAX_POLLS || 240)
const artifactDir = process.env.ARTIFACT_DIR || path.join('/tmp', `apex-golden-${Date.now()}`)
const loginEmail = process.env.LOGIN_EMAIL || ''
const loginUsername = process.env.LOGIN_USERNAME || 'spencer'
const loginPassword = process.env.LOGIN_PASSWORD || ''

if (!prompt.trim()) {
  throw new Error('PROMPT or prompt file path is required')
}
if (!loginPassword.trim()) {
  throw new Error('LOGIN_PASSWORD is required')
}

fs.mkdirSync(artifactDir, { recursive: true })

const cookies = new Map()
let csrfToken = ''
let bearerToken = ''

function recordCookies(headers) {
  const raw = typeof headers.getSetCookie === 'function'
    ? headers.getSetCookie()
    : (headers.get('set-cookie') ? headers.get('set-cookie').split(/,(?=[^;,]+=)/g) : [])
  for (const line of raw) {
    const first = String(line).split(';', 1)[0]
    const eq = first.indexOf('=')
    if (eq > 0) cookies.set(first.slice(0, eq), first.slice(eq + 1))
  }
}

function cookieHeader() {
  return [...cookies.entries()].map(([key, value]) => `${key}=${value}`).join('; ')
}

function apiURL(route) {
  return `${apiBase}${route.startsWith('/') ? route : `/${route}`}`
}

async function request(route, options = {}) {
  const { skipReauth = false, ...fetchOptions } = options
  for (let attempt = 0; attempt < 2; attempt += 1) {
    try {
      return await requestOnce(route, fetchOptions)
    } catch (error) {
      if (!skipReauth && error.status === 401 && attempt === 0) {
        console.log(`[${new Date().toISOString()}] auth expired during ${route}; refreshing session and retrying`)
        cookies.clear()
        csrfToken = ''
        bearerToken = ''
        await login()
        continue
      }
      throw error
    }
  }
}

async function requestOnce(route, options = {}) {
  const method = options.method || 'GET'
  const headers = {
    Accept: 'application/json',
    ...(options.headers || {}),
  }
  if (options.body !== undefined) headers['Content-Type'] = 'application/json'
  if (bearerToken) headers.Authorization = `Bearer ${bearerToken}`
  if (csrfToken && method !== 'GET') headers['X-CSRF-Token'] = csrfToken
  const cookie = cookieHeader()
  if (cookie) headers.Cookie = cookie

  const response = await fetch(apiURL(route), {
    ...options,
    method,
    headers,
    body: options.body === undefined ? undefined : JSON.stringify(options.body),
  })
  recordCookies(response.headers)
  const text = await response.text()
  let data = null
  try {
    data = text ? JSON.parse(text) : null
  } catch {
    data = { raw: text }
  }
  if (!response.ok) {
    const err = new Error(`HTTP ${response.status} ${method} ${route}`)
    err.status = response.status
    err.response = data
    throw err
  }
  return data
}

async function fetchCSRF() {
  try {
    const data = await request('/csrf-token', { skipReauth: true })
    csrfToken = data?.token || ''
  } catch {
    csrfToken = ''
  }
}

async function login() {
  await fetchCSRF()
  const data = await request('/auth/login', {
    method: 'POST',
    skipReauth: true,
    body: {
      username: loginUsername,
      email: loginEmail,
      password: loginPassword,
    },
  })
  bearerToken = data?.access_token || data?.token || data?.data?.access_token || data?.tokens?.access_token || ''
  await fetchCSRF()
}

function writeArtifact(name, data) {
  const file = path.join(artifactDir, name)
  fs.writeFileSync(file, typeof data === 'string' ? data : JSON.stringify(data, null, 2))
  return file
}

function summarizeBuild(data) {
  return {
    id: data?.id || data?.build_id,
    status: data?.status,
    progress: data?.progress,
    phase: data?.current_phase || data?.phase,
    live: data?.live,
    restored_from_snapshot: data?.restored_from_snapshot,
    files_count: data?.files_count ?? data?.files?.length,
    error: data?.error || '',
    quality_gate_status: data?.quality_gate_status,
    quality_gate_stage: data?.quality_gate_stage,
  }
}

async function startBuild() {
  const payload = {
    description: prompt,
    prompt,
    mode,
    power_mode: powerMode,
    provider_mode: 'platform',
    require_preview_ready: true,
    project_name: projectName,
  }
  const data = await request('/build/start', { method: 'POST', body: payload })
  writeArtifact('build-start.json', data)
  const buildID = data?.build_id || data?.buildID
  if (!buildID) throw new Error(`build/start did not return build_id: ${JSON.stringify(data)}`)
  console.log(`BUILD_ID=${buildID}`)
  return buildID
}

async function pollBuild(buildID) {
  let finalStatus = ''
  let lastProgress = -1
  let sameProgressTicks = 0
  for (let i = 0; i < maxPolls; i += 1) {
    const status = await request(`/build/${buildID}/status`).catch(error => ({ error: error.message, response: error.response }))
    const summary = summarizeBuild(status)
    if (summary.progress === lastProgress) sameProgressTicks += 1
    else sameProgressTicks = 0
    lastProgress = summary.progress
    console.log(`[${new Date().toISOString()}] status=${summary.status || ''} progress=${summary.progress ?? ''} phase=${summary.phase || ''} live=${summary.live} files=${summary.files_count ?? ''} stale_ticks=${sameProgressTicks} error=${String(summary.error || '').slice(0, 180)}`)
    if (['completed', 'failed', 'cancelled'].includes(summary.status)) {
      finalStatus = summary.status
      break
    }
    await new Promise(resolve => setTimeout(resolve, pollSeconds * 1000))
  }

  const detail = await request(`/build/${buildID}`)
  writeArtifact('build-detail.json', detail)
  console.log(`FINAL_BUILD=${JSON.stringify(summarizeBuild(detail))}`)
  if (finalStatus !== 'completed' && detail?.status !== 'completed') {
    throw new Error(`build did not complete: ${JSON.stringify(summarizeBuild(detail))}`)
  }
  if ((detail?.progress ?? 0) < 100) throw new Error(`completed build progress below 100: ${detail?.progress}`)
  if (detail?.error) throw new Error(`completed build has error: ${detail.error}`)
  if ((detail?.files?.length || detail?.files_count || 0) < 1) throw new Error('completed build has no files')
  if (detail?.quality_gate_passed !== true) throw new Error('completed build did not pass quality gate')
  return detail
}

async function startPreview(projectID) {
  if (!projectID) throw new Error('completed build detail did not include project_id')
  let data
  try {
    data = await request('/preview/fullstack/start', {
      method: 'POST',
      body: {
        project_id: projectID,
        sandbox: false,
        require_backend: false,
      },
    })
  } catch (error) {
    data = await request('/preview/start', {
      method: 'POST',
      body: {
        project_id: projectID,
        sandbox: false,
      },
    })
  }
  writeArtifact('preview-start.json', data)
  return data
}

async function pollPreview(projectID) {
  let last = null
  for (let i = 0; i < 30; i += 1) {
    last = await request(`/preview/status/${projectID}?sandbox=0`)
    writeArtifact('preview-status-latest.json', last)
    const active = last?.preview?.active === true
    const url = last?.proxy_url || last?.preview?.url || last?.url || ''
    console.log(`[${new Date().toISOString()}] preview_active=${active} url=${url}`)
    if (active && url) return { status: last, url }
    await new Promise(resolve => setTimeout(resolve, 3000))
  }
  throw new Error(`preview did not become active: ${JSON.stringify(last)}`)
}

function absolutePreviewURL(url) {
  if (/^https?:\/\//i.test(url)) return url
  const origin = new URL(apiBase).origin
  return `${origin}${url.startsWith('/') ? url : `/${url}`}`
}

async function verifyPreview(url) {
  const browser = await chromium.launch({ headless: true })
  const context = await browser.newContext({
    viewport: { width: 1440, height: 1000 },
    extraHTTPHeaders: cookieHeader() ? { Cookie: cookieHeader() } : {},
  })
  const page = await context.newPage()
  const consoleErrors = []
  page.on('console', msg => {
    if (msg.type() === 'error') consoleErrors.push(msg.text())
  })
  const response = await page.goto(absolutePreviewURL(url), { waitUntil: 'networkidle', timeout: 60000 })
  if (!response || !response.ok()) {
    throw new Error(`preview navigation failed: status=${response?.status()}`)
  }
  await page.waitForFunction(() => document.body && document.body.innerText.trim().length > 80, null, { timeout: 30000 })
  const bodyText = await page.locator('body').innerText({ timeout: 5000 })
  if (/failed to start preview|application error|vite error|runtime error/i.test(bodyText)) {
    throw new Error(`preview rendered failure text: ${bodyText.slice(0, 500)}`)
  }
  if (/page not found|sorry,\s*that page does not exist|route not found|\b404\b[\s\S]{0,80}not found|not found[\s\S]{0,80}\b404\b/i.test(bodyText)) {
    throw new Error(`preview rendered an app-level not-found route: ${bodyText.slice(0, 500)}`)
  }
  const shellOnlyNav = /dashboard/i.test(bodyText) &&
    /job pipeline/i.test(bodyText) &&
    /new job/i.test(bodyText) &&
    /crew management/i.test(bodyText) &&
    /settings/i.test(bodyText) &&
    /bootstrapped by apex\.build/i.test(bodyText) &&
    bodyText.trim().length < 180 &&
    !/open jobs|pending estimate|launch estimate swarm|recommended final quote/i.test(bodyText)
  if (shellOnlyNav || /future patches|real ui screens will be routed here|routes will be added later/i.test(bodyText)) {
    throw new Error(`preview rendered only an app shell instead of working screen content: ${bodyText.slice(0, 500)}`)
  }
  const screenshotPath = path.join(artifactDir, 'preview.png')
  await page.screenshot({ path: screenshotPath, fullPage: true })
  await browser.close()
  writeArtifact('preview-proof.json', {
    url: absolutePreviewURL(url),
    body_length: bodyText.length,
    console_errors: consoleErrors,
    screenshot: screenshotPath,
  })
  if (consoleErrors.length > 0 && process.env.ALLOW_CONSOLE_ERRORS !== '1') {
    throw new Error(`preview console errors: ${consoleErrors.slice(0, 5).join(' | ')}`)
  }
  console.log(`PREVIEW_SCREENSHOT=${screenshotPath}`)
}

try {
  console.log(`ARTIFACT_DIR=${artifactDir}`)
  console.log(`BASE_URL=${apiBase}`)
  console.log(`MODE=${mode}`)
  console.log(`POWER_MODE=${powerMode}`)
  await login()
  const buildID = await startBuild()
  const detail = await pollBuild(buildID)
  await startPreview(detail.project_id)
  const preview = await pollPreview(detail.project_id)
  await verifyPreview(preview.url)
  console.log(`GOLDEN_BUILD_PASSED build_id=${buildID} project_id=${detail.project_id}`)
} catch (error) {
  console.error(`GOLDEN_BUILD_FAILED: ${error.message}`)
  if (error.response) console.error(JSON.stringify(error.response, null, 2))
  process.exit(1)
}
