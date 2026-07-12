#!/usr/bin/env node

import { createServer } from 'node:http'
import process from 'node:process'

const baseURL = trimTrailingSlash(process.env.ADMIN_PLUS_BASE_URL || 'http://localhost:3000')
const email = process.env.ADMIN_PLUS_E2E_EMAIL || 'admin@superllm.local'
const password = process.env.ADMIN_PLUS_E2E_PASSWORD || 'AdminPlus@123456'
const allowNonLocal = process.env.ADMIN_PLUS_E2E_ALLOW_NON_LOCAL === '1'
const runID = `e2e-session-${new Date().toISOString().replace(/[-:.TZ]/g, '').slice(0, 14)}-${Math.random().toString(36).slice(2, 8)}`

let token = ''

main().catch((error) => {
  console.error(`[FAIL] ${error.message}`)
  if (error.details) console.error(error.details)
  process.exit(1)
})

async function main() {
  assertSafeE2ETarget()
  const supplierServer = await startTestSub2APISupplierServer()
  try {
    log(`session upload E2E starting: ${runID}`)
    await waitForService()
    await login()

    const supplier = await createSupplier(supplierServer.url)
    await matchSupplierSite(supplier)
    const task = await createCaptureTask(supplier, supplierServer.url)
    await verifyBrowserCredential(task, supplier, supplierServer.url)
    await completeCaptureTask(task, supplier, supplierServer.url)
    await verifyStoredSession(task, supplier)
    await verifySessionProbe(supplier, supplierServer.requests)

    log('session upload E2E completed')
    log(`Created test prefix: ${runID}`)
    log('Cleanup intentionally not executed by this script. Use tools/cleanup-admin-plus-e2e.mjs after review.')
  } finally {
    await supplierServer.close()
  }
}

async function waitForService() {
  const data = await rawJSON('/setup/status')
  assert(data?.code === 0 && data?.data?.needs_setup === false, 'setup status should report completed')
  log('service is ready')
}

async function login() {
  const data = await api('POST', '/api/v1/auth/login', { email, password }, { auth: false })
  token = data.access_token
  assert(token, 'login should return access token')
  assert(data.user?.role === 'admin', 'login user should be admin')
  log(`logged in as ${data.user.email}`)
}

async function createSupplier(supplierBaseURL) {
  const supplier = await api('POST', '/api/v1/admin-plus/suppliers', {
    name: `${runID}-supplier`,
    kind: 'relay',
    type: 'sub2api',
    runtime_status: 'active',
    health_status: 'normal',
    dashboard_url: supplierBaseURL,
    api_base_url: supplierBaseURL,
    contact: 'session-upload-e2e@example.com',
    notes: `created by ${runID}`,
    browser_login_enabled: true,
    browser_login_username: `${runID}@supplier.example.com`,
    browser_login_password: 'e2e-session-password',
    browser_login_token: 'e2e-session-token',
    balance_cents: 123400,
    balance_currency: 'USD'
  }, { expected: 201 })
  assert(supplier.id > 0, 'supplier should have id')
  assert(supplier.credential?.browser_login_enabled === true, 'supplier browser login should be enabled')
  log(`created session supplier #${supplier.id}`)
  return supplier
}

async function matchSupplierSite(supplier) {
  const matched = await api('POST', '/api/v1/admin-plus/suppliers/site-match', {
    url: `${supplier.dashboard_url}/dashboard`,
    origin: supplier.dashboard_url,
    host: new URL(supplier.dashboard_url).host
  })
  assert(matched.status === 'matched', 'site match should identify supplier')
  const matchedSupplier = matched.supplier || matched.suppliers?.[0]
  assert(matched.supplier_id === supplier.id || matchedSupplier?.id === supplier.id, 'site match should return created supplier')
  log('supplier site match verified')
}

async function createCaptureTask(supplier, supplierBaseURL) {
  const task = await api('POST', '/api/v1/admin-plus/extension/session/capture-task', {
    supplier_id: supplier.id,
    device_id: `${runID}-chrome`,
    lease_ttl_seconds: 60,
    payload: {
      source_url: `${supplierBaseURL}/dashboard`,
      source_host: new URL(supplierBaseURL).host,
      run_id: runID
    }
  }, { expected: 201 })
  assert(task.type === 'capture_supplier_session', 'capture task should use session task type')
  assert(task.status === 'claimed', 'capture task should be leased immediately')
  assert(task.lease_token, 'capture task should return lease token')
  log(`created capture session task #${task.id}`)
  return task
}

async function verifyBrowserCredential(task, supplier, supplierBaseURL) {
  const denied = await api('POST', `/api/v1/admin-plus/extension/tasks/${task.id}/browser-credential`, {
    device_id: task.device_id,
    lease_token: 'bad-token'
  }, { expected: 409, allowError: true })
  assert(denied.reason === 'EXTENSION_TASK_LEASE_MISMATCH', 'browser credential should require valid lease')

  const credential = await api('POST', `/api/v1/admin-plus/extension/tasks/${task.id}/browser-credential`, {
    device_id: task.device_id,
    lease_token: task.lease_token
  })
  assert(credential.supplier_id === supplier.id, 'browser credential should belong to capture supplier')
  assert(credential.dashboard_url === supplierBaseURL, 'browser credential should include supplier dashboard url')
  assert(credential.username === `${runID}@supplier.example.com`, 'browser credential should return username')
  assert(credential.password === 'e2e-session-password', 'browser credential should return password under valid lease')
  assert(credential.token === 'e2e-session-token', 'browser credential should return token under valid lease')
  log('leased browser credential verified')
}

async function completeCaptureTask(task, supplier, supplierBaseURL) {
  const heartbeat = await api('POST', `/api/v1/admin-plus/extension/tasks/${task.id}/heartbeat`, {
    device_id: task.device_id,
    lease_token: task.lease_token,
    lease_ttl_seconds: 60
  })
  assert(heartbeat.status === 'running', 'heartbeat should mark capture task running')

  const completed = await api('POST', `/api/v1/admin-plus/extension/tasks/${task.id}/complete`, {
    device_id: task.device_id,
    lease_token: task.lease_token,
    result: {
      source: 'chrome',
      run_id: runID,
      captured_at: new Date().toISOString(),
      session_bundle: {
        supplier_id: supplier.id,
        origin: supplierBaseURL,
        url: `${supplierBaseURL}/dashboard`,
        captured_at: new Date().toISOString(),
        tokens: {
          access_token: 'e2e-browser-access-token',
          csrf_token: 'e2e-csrf-token'
        },
        required_headers: {
          cookie: 'sid=e2e-browser-cookie',
          origin: supplierBaseURL,
          referer: `${supplierBaseURL}/dashboard`
        },
        context: {
          api_base_url: supplierBaseURL,
          user_id: `${runID}-supplier-user`
        }
      }
    }
  })
  assert(completed.status === 'succeeded', 'complete should mark capture task succeeded')
  assert(completed.result?.ingest?.session_captured === true, 'complete should ingest supplier session')
  assert(completed.result?.session_summary?.has_access_token === true, 'complete should retain session summary')
  assertNoSecret(completed, 'complete response')
  log('capture task completion verified')
}

async function verifyStoredSession(task, supplier) {
  const session = await api('GET', `/api/v1/admin-plus/suppliers/${supplier.id}/session`)
  assert(session.has_encrypted_bundle === true, 'captured session should be encrypted at rest')
  assert(session.source_extension_task_id === task.id, 'captured session should retain source extension task id')
  assert(session.session_summary?.has_access_token === true, 'session summary should report token presence')
  assert(session.session_summary?.cookie_count === 1, 'session summary should report cookie presence')
  assertNoSecret(session, 'session response')
  log('stored encrypted session verified')
}

async function verifySessionProbe(supplier, requests) {
  const probed = await api('POST', `/api/v1/admin-plus/suppliers/${supplier.id}/session/probe`, {
    low_balance_threshold_cents: 2000
  })
  assert(probed.probe?.system_type === 'sub2api', 'session probe should use Sub2API provider adapter')
  assert(probed.probe?.balance_cents === 1234, 'session probe should read supplier profile balance')
  assert(probed.balance_snapshot?.balance_cents === 1234, 'session probe should record balance snapshot')
  const profileRequest = requests.find((request) => request.path === '/api/v1/user/profile')
  assert(profileRequest, 'session probe should call supplier profile endpoint')
  assert(profileRequest.authorization === 'Bearer e2e-browser-access-token', 'session probe should send uploaded bearer token')
  assert(profileRequest.cookie === 'sid=e2e-browser-cookie', 'session probe should send uploaded cookie')
  assertNoSecret(probed, 'probe response')
  log('session probe from uploaded browser session verified')
}

function startTestSub2APISupplierServer() {
  const requests = []
  const server = createServer((req, res) => {
    const path = new URL(req.url, 'http://127.0.0.1').pathname
    requests.push({
      method: req.method,
      path,
      authorization: req.headers.authorization || '',
      cookie: req.headers.cookie || '',
      origin: req.headers.origin || '',
      referer: req.headers.referer || ''
    })
    if (req.method === 'GET' && path === '/api/v1/user/profile') {
      res.writeHead(200, { 'Content-Type': 'application/json' })
      res.end(JSON.stringify({
        data: {
          id: 42,
          email: `${runID}-supplier-user@example.com`,
          username: `${runID}-supplier-user`,
          role: 'user',
          status: 'active',
          balance: 12.34,
          concurrency: 5,
          allowed_groups: [1, 2]
        }
      }))
      return
    }
    res.writeHead(404, { 'Content-Type': 'application/json' })
    res.end(JSON.stringify({ error: 'not found' }))
  })

  return new Promise((resolve, reject) => {
    server.once('error', reject)
    server.listen(0, '127.0.0.1', () => {
      server.off('error', reject)
      const address = server.address()
      resolve({
        url: `http://127.0.0.1:${address.port}`,
        requests,
        close: () => new Promise((done) => server.close(() => done()))
      })
    })
  })
}

async function api(method, path, body, options = {}) {
  const headers = { 'Content-Type': 'application/json' }
  if (options.auth !== false) headers.Authorization = `Bearer ${token}`
  const response = await fetch(`${baseURL}${path}`, {
    method,
    headers,
    body: body === undefined ? undefined : JSON.stringify(body)
  })
  const text = await response.text()
  const json = parseJSON(text, `${method} ${path}`)
  const expected = Array.isArray(options.expected) ? options.expected : [options.expected || 200]
  assert(expected.includes(response.status), `${method} ${path} expected ${expected.join('/')} got ${response.status}`, text)
  if (options.allowError) return json
  assert(json.code === 0, `${method} ${path} should return code 0`, text)
  return json.data
}

async function rawJSON(path) {
  const response = await fetch(`${baseURL}${path}`)
  const text = await response.text()
  assert(response.ok, `GET ${path} should be ok`, text)
  return parseJSON(text, `GET ${path}`)
}

function parseJSON(text, label) {
  try {
    return JSON.parse(text)
  } catch {
    fail(`${label} should return JSON`, text)
  }
}

function assertNoSecret(value, label) {
  const text = JSON.stringify(value)
  assert(!text.includes('e2e-browser-access-token'), `${label} should not expose access token`)
  assert(!text.includes('e2e-browser-cookie'), `${label} should not expose cookie`)
  assert(!text.includes('e2e-csrf-token'), `${label} should not expose csrf token`)
}

function assertSafeE2ETarget() {
  if (allowNonLocal) return
  const apiURL = new URL(baseURL)
  assert(isLocalHost(apiURL.hostname), `refuse to run E2E against non-local API host: ${apiURL.hostname}`)
}

function trimTrailingSlash(value) {
  return String(value || '').replace(/\/+$/, '')
}

function isLocalHost(hostname) {
  return ['localhost', '127.0.0.1', '::1'].includes(hostname)
}

function assert(condition, message, details) {
  if (!condition) fail(message, details)
}

function fail(message, details) {
  const error = new Error(message)
  error.details = details
  throw error
}

function log(message) {
  console.log(`[OK] ${message}`)
}
