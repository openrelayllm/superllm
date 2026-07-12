#!/usr/bin/env node

import { execFileSync } from 'node:child_process'
import { createServer } from 'node:http'
import process from 'node:process'

const baseURL = trimTrailingSlash(process.env.ADMIN_PLUS_BASE_URL || 'http://localhost:3000')
const email = process.env.ADMIN_PLUS_E2E_EMAIL || 'admin@superllm.local'
const password = process.env.ADMIN_PLUS_E2E_PASSWORD || 'AdminPlus@123456'
const dbURL = process.env.ADMIN_PLUS_E2E_DB_URL || 'postgresql://root:root@127.0.0.1:5432/superllm?sslmode=disable'
const redisURL = process.env.ADMIN_PLUS_E2E_REDIS_URL || 'redis://127.0.0.1:6379/0'
const cleanupEnabled = process.env.ADMIN_PLUS_E2E_CLEANUP !== 'false'
const allowNonLocal = process.env.ADMIN_PLUS_E2E_ALLOW_NON_LOCAL === '1'
const realUpstreamBaseURL = trimTrailingSlash(process.env.ADMIN_PLUS_E2E_REAL_UPSTREAM_BASE_URL || '')
const realUpstreamAPIKey = process.env.ADMIN_PLUS_E2E_REAL_UPSTREAM_API_KEY || ''
const probeModel = 'gpt-5.5'
const runID = `e2e-${new Date().toISOString().replace(/[-:.TZ]/g, '').slice(0, 14)}-${Math.random().toString(36).slice(2, 8)}`

let token = ''
let testUpstreamBaseURL = ''
let testUpstreamRequests = []
let localAccountIDForCleanup = 0

main().catch((error) => {
  console.error(`[FAIL] ${error.message}`)
  if (error.details) {
    console.error(error.details)
  }
  process.exit(1)
})

async function main() {
  assertSafeE2ETarget()
  const testUpstream = realUpstreamBaseURL
    ? await useRealOpenAICompatibleUpstream()
    : await startTestOpenAIResponsesServer()
  testUpstreamBaseURL = testUpstream.url
  testUpstreamRequests = testUpstream.requests
  let runError = null
  try {
    log(`Admin Plus API E2E starting: ${runID}`)
    await waitForService()
    await login()

    const localAccountID = createLocalAccountFixture()
    await exerciseLocalAccountRuntime(localAccountID)
    let supplier = await createSupplier()
    await listAndGetSupplier(supplier.id)
    supplier = await updateSupplier(supplier.id)
    const activeSupplier = await updateSupplierStatus(supplier.id)

    let supplierAccount = await createSupplierAccount(activeSupplier.id, localAccountID)
    supplierAccount = await updateSupplierAccount(activeSupplier.id, supplierAccount)
    await listSupplierAccounts(activeSupplier.id, supplierAccount.id)

    const rateEvent = await exerciseRateMonitoring(activeSupplier.id)
    const balanceEvent = await exerciseBalanceMonitoring(activeSupplier.id)
    const healthEvent = await exerciseHealthMonitoring(activeSupplier.id, supplierAccount.id)
    const announcementEvent = await exerciseAnnouncementMonitoring(activeSupplier.id)
    const reconciliation = await exerciseBillingAndReconciliation(activeSupplier.id, localAccountID)
    await exerciseScheduler(activeSupplier.id)
    await exerciseExtensionTasks(activeSupplier.id)
    const candidateSupplier = await createCandidateSupplier()
    await exerciseActionRecommendations(activeSupplier, candidateSupplier, balanceEvent, announcementEvent, healthEvent, reconciliation.summary)
    await verifyAllListEndpoints(activeSupplier.id)
    await deleteSupplierAccount(activeSupplier.id, supplierAccount.id)

    log('Admin Plus API E2E completed')
    log(`Created test prefix: ${runID}`)
    log(`Verified rate event id: ${rateEvent.id}`)
  } catch (error) {
    runError = error
    throw error
  } finally {
    let cleanupError = null
    if (cleanupEnabled) {
      cleanupError = cleanupE2EFixturesSafely()
    } else {
      log(`cleanup disabled; test prefix remains: ${runID}`)
    }
    await testUpstream.close()
    if (cleanupError && !runError) {
      throw cleanupError
    }
  }
}

async function waitForService() {
  const data = await rawJSON('/setup/status')
  assert(data?.code === 0 && data?.data?.needs_setup === false, 'setup status should report completed')
  log('service is ready')
}

async function login() {
  const data = await api('POST', '/api/v1/auth/login', {
    email,
    password
  }, { auth: false })
  token = data.access_token
  assert(token, 'login should return access token')
  assert(data.user?.role === 'admin', 'login user should be admin')

  const me = await api('GET', '/api/v1/auth/me')
  assert(me.role === 'admin', 'auth/me should return admin user')
  log(`logged in as ${me.email}`)
}

function createLocalAccountFixture() {
  const name = `${runID}-local-openai`
  const credentials = JSON.stringify({
    api_key: realUpstreamAPIKey || 'sk-e2e-test-only',
    base_url: testUpstreamBaseURL
  })
  const sql = `
    INSERT INTO accounts (
      name, platform, type, credentials, extra,
      concurrency, priority, status, schedulable, rate_multiplier,
      created_at, updated_at
    )
    VALUES (
      '${sqlString(name)}', 'openai', 'apikey',
      '${sqlString(credentials)}'::jsonb,
      '{"source":"admin-plus-e2e"}'::jsonb,
      8, 10, 'active', true, 1.0,
      NOW(), NOW()
    )
    RETURNING id;
  `
  const out = execFileSync('psql', [dbURL, '-v', 'ON_ERROR_STOP=1', '-At', '-c', sql], {
    encoding: 'utf8',
    stdio: ['ignore', 'pipe', 'pipe']
  }).trim()
  const id = parseReturningID(out)
  assert(Number.isInteger(id) && id > 0, `local account fixture should return id, got: ${out}`)
  localAccountIDForCleanup = id
  log(`created local account fixture #${id}`)
  return id
}

function createLocalUsageFixture(localAccountID, requestID, model, actualCostUSD) {
  const userID = ensureLocalUserFixture()
  const apiKeyID = ensureLocalAPIKeyFixture(userID)
  const sql = `
    INSERT INTO usage_logs (
      user_id, api_key_id, account_id, request_id, model, requested_model,
      input_tokens, output_tokens, total_cost, actual_cost,
      duration_ms, first_token_ms, created_at
    )
    VALUES (
      ${userID}, ${apiKeyID}, ${localAccountID}, '${sqlString(requestID)}', '${sqlString(model)}', '${sqlString(model)}',
      1000, 300, ${actualCostUSD}, ${actualCostUSD},
      2400, 800, NOW()
    )
    RETURNING id;
  `
  const out = execFileSync('psql', [dbURL, '-v', 'ON_ERROR_STOP=1', '-At', '-c', sql], {
    encoding: 'utf8',
    stdio: ['ignore', 'pipe', 'pipe']
  }).trim()
  const id = parseReturningID(out)
  assert(Number.isInteger(id) && id > 0, `local usage fixture should return id, got: ${out}`)
  log(`created local usage fixture #${id}`)
  return id
}

async function exerciseLocalAccountRuntime(localAccountID) {
  writeRuntimeRedisFixture(localAccountID)
  const runtime = await api('GET', `/api/v1/admin-plus/sub2api/account-runtime?account_id=${localAccountID}&limit=20`)
  const item = runtime.items.find((entry) => entry.account_id === localAccountID)
  assert(item, 'account runtime should include local account fixture')
  assert(item.current_concurrency === 2, 'account runtime should read Redis concurrency')
  assert(item.waiting_count === 1, 'account runtime should read Redis waiting queue')
  assert(item.configured_limit === 8, 'account runtime should read configured concurrency from Sub2API database')
  assert(item.switch_eligible === true, 'account runtime should mark active account switch eligible')
  log('local account runtime verified')
}

function writeRuntimeRedisFixture(localAccountID) {
  const now = Math.floor(Date.now() / 1000)
  execFileSync('redis-cli', ['-u', redisURL, 'ZADD', `concurrency:account:${localAccountID}`, String(now), `${runID}-req-a`, String(now), `${runID}-req-b`], {
    encoding: 'utf8',
    stdio: ['ignore', 'pipe', 'pipe']
  })
  execFileSync('redis-cli', ['-u', redisURL, 'SET', `wait:account:${localAccountID}`, '1', 'EX', '300'], {
    encoding: 'utf8',
    stdio: ['ignore', 'pipe', 'pipe']
  })
}

function ensureLocalUserFixture() {
  const emailValue = `${runID}@e2e.local`
  const sql = `
    INSERT INTO users (email, password_hash, role, balance, concurrency, status, created_at, updated_at)
    VALUES ('${sqlString(emailValue)}', 'e2e-test-only-password-hash', 'user', 100, 5, 'active', NOW(), NOW())
    RETURNING id;
  `
  const out = execFileSync('psql', [dbURL, '-v', 'ON_ERROR_STOP=1', '-At', '-c', sql], {
    encoding: 'utf8',
    stdio: ['ignore', 'pipe', 'pipe']
  }).trim()
  const id = parseReturningID(out)
  assert(Number.isInteger(id) && id > 0, `local user fixture should return id, got: ${out}`)
  return id
}

function ensureLocalAPIKeyFixture(userID) {
  const keyValue = `sk-${runID}`
  const name = `${runID}-api-key`
  const sql = `
    INSERT INTO api_keys (user_id, key, name, status, created_at, updated_at)
    VALUES (${userID}, '${sqlString(keyValue)}', '${sqlString(name)}', 'active', NOW(), NOW())
    RETURNING id;
  `
  const out = execFileSync('psql', [dbURL, '-v', 'ON_ERROR_STOP=1', '-At', '-c', sql], {
    encoding: 'utf8',
    stdio: ['ignore', 'pipe', 'pipe']
  }).trim()
  const id = parseReturningID(out)
  assert(Number.isInteger(id) && id > 0, `local api key fixture should return id, got: ${out}`)
  return id
}

async function createSupplier() {
  const supplier = await api('POST', '/api/v1/admin-plus/suppliers', {
    name: `${runID}-supplier`,
    kind: 'relay',
    type: 'sub2api',
    runtime_status: 'candidate',
    health_status: 'normal',
    dashboard_url: 'https://supplier.example.com',
    api_base_url: 'https://supplier.example.com/api/v1',
    contact: 'ops@example.com',
    notes: `created by ${runID}`,
    browser_login_enabled: true,
    browser_login_username: `${runID}@supplier.example.com`,
    browser_login_password: 'e2e-test-only-password',
    browser_login_token: 'e2e-test-only-token',
    balance_cents: 500000,
    balance_currency: 'CNY'
  }, { expected: 201 })
  assert(supplier.id > 0, 'supplier should have id')
  assert(supplier.credential?.browser_login_enabled === true, 'supplier browser login should be enabled')
  assert(supplier.credential?.browser_login_username_configured === true, 'supplier browser username should be configured')
  log(`created supplier #${supplier.id}`)
  return supplier
}

async function createCandidateSupplier() {
  const supplier = await api('POST', '/api/v1/admin-plus/suppliers', {
    name: `${runID}-candidate-supplier`,
    kind: 'relay',
    type: 'sub2api',
    runtime_status: 'candidate',
    health_status: 'normal',
    dashboard_url: 'https://candidate.example.com',
    api_base_url: 'https://candidate.example.com/api/v1',
    contact: 'candidate-ops@example.com',
    notes: `created by ${runID} as switch target`,
    browser_login_enabled: true,
    browser_login_username: `${runID}-candidate@supplier.example.com`,
    browser_login_password: 'e2e-test-only-password',
    balance_cents: 800000,
    balance_currency: 'CNY'
  }, { expected: 201 })
  assert(supplier.id > 0, 'candidate supplier should have id')
  assert(supplier.runtime_status === 'candidate', 'candidate supplier should be switchable')
  log(`created candidate supplier #${supplier.id}`)
  return supplier
}

async function listAndGetSupplier(supplierID) {
  const list = await api('GET', `/api/v1/admin-plus/suppliers?q=${encodeURIComponent(runID)}`)
  assert(list.total >= 1, 'supplier list should include test supplier')
  assert(!JSON.stringify(list).includes('e2e-test-only-password'), 'supplier list should not expose browser login password')
  assert(!JSON.stringify(list).includes('e2e-test-only-token'), 'supplier list should not expose browser login token')

  const supplier = await api('GET', `/api/v1/admin-plus/suppliers/${supplierID}`)
  assert(supplier.id === supplierID, 'supplier get should return requested supplier')
  assert(!JSON.stringify(supplier).includes('e2e-test-only-password'), 'supplier get should not expose browser login password')
  assert(!JSON.stringify(supplier).includes('e2e-test-only-token'), 'supplier get should not expose browser login token')
  log('supplier list/get verified')
}

async function updateSupplier(supplierID) {
  const supplier = await api('PUT', `/api/v1/admin-plus/suppliers/${supplierID}`, {
    name: `${runID}-supplier-updated`,
    kind: 'relay',
    type: 'sub2api',
    runtime_status: 'candidate',
    health_status: 'normal',
    dashboard_url: 'https://supplier.example.com',
    api_base_url: 'https://supplier.example.com/api/v1',
    contact: 'ops-updated@example.com',
    notes: `updated by ${runID}`,
    browser_login_enabled: true,
    balance_cents: 650000,
    balance_currency: 'CNY'
  })
  assert(supplier.id === supplierID, 'supplier update should keep id')
  assert(supplier.name.endsWith('-supplier-updated'), 'supplier update should persist name')
  assert(supplier.contact === 'ops-updated@example.com', 'supplier update should persist contact')
  assert(supplier.credential?.browser_login_username_configured === true, 'supplier update should preserve browser username when omitted')
  assert(supplier.credential?.browser_login_password_configured === true, 'supplier update should preserve browser password when omitted')
  assert(supplier.credential?.browser_login_token_configured === true, 'supplier update should preserve browser token when omitted')
  assert(!JSON.stringify(supplier).includes('e2e-test-only-password'), 'supplier update should not expose browser login password')
  assert(!JSON.stringify(supplier).includes('e2e-test-only-token'), 'supplier update should not expose browser login token')
  log('supplier update verified')
  return supplier
}

async function updateSupplierStatus(supplierID) {
  const supplier = await api('PATCH', `/api/v1/admin-plus/suppliers/${supplierID}/status`, {
    runtime_status: 'active',
    health_status: 'normal'
  })
  assert(supplier.runtime_status === 'active', 'supplier should become active')
  assert(supplier.health_status === 'normal', 'supplier health should remain normal')
  log('supplier status update verified')
  return supplier
}

async function createSupplierAccount(supplierID, localAccountID) {
  const account = await api('POST', `/api/v1/admin-plus/suppliers/${supplierID}/accounts`, {
    local_sub2api_account_id: localAccountID,
    supplier_account_identifier: `${runID}-upstream-key`,
    supplier_account_label: 'E2E upstream key',
    organization_id: 'org-e2e',
    project_id: 'proj-e2e',
    rate_profile: 'default',
    configured_concurrency: 8,
    balance_threshold_cents: 1000,
    balance_cents: 200000,
    balance_currency: 'CNY',
    runtime_status: 'candidate',
    health_status: 'normal'
  }, { expected: 201 })
  assert(account.id > 0, 'supplier account should have id')
  assert(account.local_sub2api_account_id === localAccountID, 'supplier account should bind local account')
  assert(account.has_usable_balance === true, 'supplier account should have usable balance')
  log(`created supplier account binding #${account.id}`)
  return account
}

async function updateSupplierAccount(supplierID, account) {
  const updated = await api('PUT', `/api/v1/admin-plus/suppliers/${supplierID}/accounts/${account.id}`, {
    supplier_account_identifier: `${runID}-upstream-key-updated`,
    supplier_account_label: 'E2E updated upstream key',
    organization_id: 'org-e2e-updated',
    project_id: 'proj-e2e-updated',
    rate_profile: 'discount-e2e',
    configured_concurrency: 12,
    observed_max_concurrency: 9,
    balance_threshold_cents: 2000,
    balance_cents: 300000,
    balance_currency: 'CNY',
    runtime_status: 'candidate',
    health_status: 'normal'
  })
  assert(updated.id === account.id, 'supplier account update should keep id')
  assert(updated.local_sub2api_account_id === account.local_sub2api_account_id, 'supplier account update should keep local account binding')
  assert(updated.supplier_account_identifier.endsWith('-upstream-key-updated'), 'supplier account update should persist supplier identifier')
  assert(updated.rate_profile === 'discount-e2e', 'supplier account update should persist rate profile')
  assert(updated.configured_concurrency === 12, 'supplier account update should persist configured concurrency')
  assert(updated.observed_max_concurrency === 9, 'supplier account update should persist observed max concurrency')
  assert(updated.balance_cents === 300000, 'supplier account update should persist balance')
  assert(updated.has_usable_balance === true, 'supplier account update should keep usable balance')
  log('supplier account update verified')
  return updated
}

async function listSupplierAccounts(supplierID, accountID) {
  const list = await api('GET', `/api/v1/admin-plus/suppliers/${supplierID}/accounts`)
  assert(list.items.some((item) => item.id === accountID), 'supplier account list should include created binding')

  const localAccounts = await api('GET', `/api/v1/admin-plus/sub2api/accounts?q=${encodeURIComponent(runID)}&limit=20`)
  assert(localAccounts.items.some((item) => item.name.includes(runID)), 'local Sub2API account list should include fixture')
  log('supplier account list verified')
}

async function deleteSupplierAccount(supplierID, accountID) {
  const result = await api('DELETE', `/api/v1/admin-plus/suppliers/${supplierID}/accounts/${accountID}`)
  assert(result.deleted === true, 'supplier account delete should confirm deletion')

  const list = await api('GET', `/api/v1/admin-plus/suppliers/${supplierID}/accounts`)
  assert(!list.items.some((item) => item.id === accountID), 'deleted supplier account should not remain in list')
  log('supplier account delete verified')
}

async function exerciseRateMonitoring(supplierID) {
  const first = await api('POST', '/api/v1/admin-plus/rates/snapshots', {
    supplier_id: supplierID,
    source: 'chrome',
    threshold_percent: 1,
    entries: [{
      model: `${runID}-model`,
      billing_mode: 'token',
      price_item: 'input',
      unit: '1m_tokens',
      currency: 'USD',
      price_micros: 100000,
      raw_payload: { run_id: runID, version: 1 }
    }]
  }, { expected: 201 })
  assert(first.snapshots.length === 1, 'first rate snapshot should be recorded')
  assert(first.events.length === 1 && first.events[0].direction === 'new', 'first rate snapshot should create new event')

  const second = await api('POST', '/api/v1/admin-plus/rates/snapshots', {
    supplier_id: supplierID,
    source: 'chrome',
    threshold_percent: 1,
    entries: [{
      model: `${runID}-model`,
      billing_mode: 'token',
      price_item: 'input',
      unit: '1m_tokens',
      currency: 'USD',
      price_micros: 125000,
      raw_payload: { run_id: runID, version: 2 }
    }]
  }, { expected: 201 })
  assert(second.events.length === 1 && second.events[0].direction === 'increase', 'second rate snapshot should create increase event')

  const ack = await api('PATCH', `/api/v1/admin-plus/rates/events/${second.events[0].id}/ack`)
  assert(ack.status === 'acknowledged', 'rate event should be acknowledged')

  const snapshots = await api('GET', `/api/v1/admin-plus/rates/snapshots?supplier_id=${supplierID}&model=${encodeURIComponent(`${runID}-model`)}`)
  assert(snapshots.total >= 2, 'rate snapshots list should include both snapshots')
  log('rate monitoring verified')
  return second.events[0]
}

async function exerciseBalanceMonitoring(supplierID) {
  await api('POST', '/api/v1/admin-plus/balances/snapshots', {
    supplier_id: supplierID,
    source: 'chrome',
    runtime_status: 'active',
    balance_cents: 300000,
    currency: 'CNY',
    low_balance_threshold_cents: 1000
  }, { expected: 201 })

  const low = await api('POST', '/api/v1/admin-plus/balances/snapshots', {
    supplier_id: supplierID,
    source: 'chrome',
    runtime_status: 'active',
    balance_cents: 500,
    currency: 'CNY',
    low_balance_threshold_cents: 1000
  }, { expected: 201 })
  assert(low.snapshot.switch_eligible === true, 'low positive active balance should remain switch eligible')
  assert(low.event?.type === 'low_balance', 'low balance snapshot should create low_balance event')

  const ack = await api('PATCH', `/api/v1/admin-plus/balances/events/${low.event.id}/ack`)
  assert(ack.status === 'acknowledged', 'balance event should be acknowledged')
  log('balance monitoring verified')
  return low.event
}

async function exerciseHealthMonitoring(supplierID, supplierAccountID) {
  const requestCountBefore = testUpstreamRequests.length
  const probe = await api('POST', '/api/v1/admin-plus/health/probe', {
    supplier_id: supplierID,
    supplier_account_id: supplierAccountID,
    model: probeModel,
    prompt: 'Return exactly: ok',
    first_token_threshold_ms: 3000,
    total_latency_threshold_ms: 30000,
    concurrency_saturation_percent: 100
  }, { expected: 201 })
  assert(probe.sample.source === 'responses_probe', 'health probe should persist responses_probe source')
  assert(probe.sample.model === probeModel, 'health probe should use latest configured probe model')
  assert(probe.sample.status_code === 200, 'health probe should record upstream HTTP 200')
  assert(probe.sample.raw_payload?.local_sub2api_account_id > 0, 'health probe should bind to local Sub2API account')
  assert(probe.sample.raw_payload?.supplier_account_id === supplierAccountID, 'health probe should bind to supplier account child')
  assert(!JSON.stringify(probe.sample).includes(realUpstreamAPIKey || 'sk-e2e-test-only'), 'health probe response should not expose api key')
  assert(testUpstreamRequests.length === requestCountBefore + 1, 'health probe should call OpenAI-compatible upstream once')
  const request = testUpstreamRequests[testUpstreamRequests.length - 1]
  assert(request.path === '/v1/responses', 'health probe should call /v1/responses')
  if (!realUpstreamBaseURL) {
    assert(request.authorization === 'Bearer sk-e2e-test-only', 'health probe should send local account bearer key upstream')
  }
  assert(request.body?.model === probeModel, 'health probe upstream payload should request gpt-5.5')
  assert(request.body?.stream === true, 'health probe upstream payload should use streaming')

  const result = await api('POST', '/api/v1/admin-plus/health/samples', {
    supplier_id: supplierID,
    source: 'probe',
    model: `${runID}-model`,
    first_token_latency_ms: 5000,
    total_latency_ms: 35000,
    status_code: 502,
    error_class: 'bad_gateway',
    observed_concurrency: 10,
    available_concurrency: 0,
    concurrency_limit: 10,
    first_token_threshold_ms: 3000,
    total_latency_threshold_ms: 30000,
    concurrency_saturation_percent: 100
  }, { expected: 201 })
  const eventTypes = result.events.map((item) => item.type)
  assert(eventTypes.includes('slow_first_token'), 'health should detect slow first token')
  assert(eventTypes.includes('slow_total'), 'health should detect slow total latency')
  assert(eventTypes.includes('request_error'), 'health should detect request error')
  assert(eventTypes.includes('concurrency_full'), 'health should detect full concurrency')

  const ack = await api('PATCH', `/api/v1/admin-plus/health/events/${result.events[0].id}/ack`)
  assert(ack.status === 'acknowledged', 'health event should be acknowledged')
  log('health monitoring and OpenAI-compatible responses probe verified')
  return result.events.find((item) => item.type === 'request_error') || result.events[0]
}

async function exerciseAnnouncementMonitoring(supplierID) {
  const bonus = 20
  const event = await api('POST', '/api/v1/admin-plus/announcements', {
    supplier_id: supplierID,
    source: 'chrome',
    type: 'recharge_bonus',
    title: `${runID} recharge bonus`,
    description: 'E2E announcement for zero-balance monitor-only supplier.',
    currency: 'CNY',
    min_recharge_cents: 10000,
    bonus_percent: bonus,
    runtime_status: 'monitor_only',
    balance_cents: 0
  }, { expected: 201 })
  assert(event.recommendation === 'recharge_to_unlock', 'zero-balance announcement should recommend recharge_to_unlock')
  assert(event.switch_eligible === false, 'zero-balance announcement should not be switch eligible')

  const ack = await api('PATCH', `/api/v1/admin-plus/announcements/${event.id}/ack`)
  assert(ack.status === 'acknowledged', 'announcement event should be acknowledged')
  log('announcement monitoring verified')
  return event
}

async function exerciseBillingAndReconciliation(supplierID, localAccountID) {
  const startedAt = new Date().toISOString()
  const endedAt = new Date(Date.now() + 1500).toISOString()
  const externalRequestID = `${runID}-req-1`
  const localUsageID = createLocalUsageFixture(localAccountID, externalRequestID, `${runID}-model`, 2.4)
  const imported = await api('POST', '/api/v1/admin-plus/billing/lines/import', {
    lines: [{
      supplier_id: supplierID,
      source: 'chrome',
      external_bill_id: `${runID}-bill-1`,
      external_request_id: externalRequestID,
      model: `${runID}-model`,
      currency: 'USD',
      cost_cents: 120,
      input_tokens: 1000,
      output_tokens: 300,
      started_at: startedAt,
      ended_at: endedAt,
      raw_payload: { run_id: runID }
    }]
  }, { expected: 201 })
  assert(imported.total === 1, 'billing import should create one bill line')
  const bill = imported.items[0]

  const list = await api('GET', `/api/v1/admin-plus/billing/lines?supplier_id=${supplierID}`)
  assert(list.items.some((item) => item.id === bill.id), 'billing list should include imported bill line')

  const usageLines = await api('GET', `/api/v1/admin-plus/sub2api/usage-lines?account_id=${localAccountID}&model=${encodeURIComponent(`${runID}-model`)}&limit=20`)
  assert(usageLines.items.some((item) => item.id === localUsageID), 'local usage lines should include usage log fixture')

  const usageSummary = await api('GET', `/api/v1/admin-plus/sub2api/usage-summary?account_id=${localAccountID}&model=${encodeURIComponent(`${runID}-model`)}&limit=20`)
  assert(usageSummary.items.some((item) => item.account_id === localAccountID && item.request_count >= 1), 'local usage summary should include usage log fixture')

  const reconciliation = await api('POST', '/api/v1/admin-plus/reconciliation/run', {
    supplier_bills: [{
      id: bill.id,
      supplier_id: supplierID,
      external_bill_id: bill.external_bill_id,
      external_request_id: externalRequestID,
      model: bill.model,
      currency: bill.currency,
      cost_cents: bill.cost_cents,
      input_tokens: bill.input_tokens,
      output_tokens: bill.output_tokens,
      started_at: startedAt
    }],
    local_usages: usageLines.items.filter((item) => item.id === localUsageID),
    time_tolerance_seconds: 60,
    cost_mismatch_cents: 0
  })
  assert(reconciliation.summary.matched_lines === 1, 'reconciliation should match supplier bill with local usage')
  assert(reconciliation.summary.profit_cents === 120, 'reconciliation should calculate profit')
  log('billing and reconciliation verified')
  return reconciliation
}

async function exerciseExtensionTasks(supplierID) {
  const supplierServer = await startTestSub2APISupplierServer()
  try {
    const supplier = await api('POST', '/api/v1/admin-plus/suppliers', {
      name: `${runID}-session-supplier`,
      kind: 'relay',
      type: 'sub2api',
      runtime_status: 'active',
      health_status: 'normal',
      dashboard_url: supplierServer.url,
      api_base_url: supplierServer.url,
      contact: 'session-ops@example.com',
      notes: `session upload e2e for ${runID}`,
      browser_login_enabled: true,
      browser_login_username: `${runID}-session@supplier.example.com`,
      browser_login_password: 'e2e-session-password',
      browser_login_token: 'e2e-session-token',
      balance_cents: 123400,
      balance_currency: 'USD'
    }, { expected: 201 })
    assert(supplier.id > 0, 'session supplier should be created')

    const deviceID = `${runID}-chrome-session`
    const task = await api('POST', '/api/v1/admin-plus/extension/session/capture-task', {
      supplier_id: supplier.id,
      device_id: deviceID,
      lease_ttl_seconds: 60,
      payload: {
        source_url: `${supplierServer.url}/dashboard`,
        source_host: new URL(supplierServer.url).host,
        run_id: runID
      }
    }, { expected: 201 })
    assert(task.type === 'capture_supplier_session', 'capture task should use session task type')
    assert(task.status === 'claimed', 'capture task should be leased immediately')
    assert(task.lease_token, 'capture task should return lease token')

    const deniedCredential = await api('POST', `/api/v1/admin-plus/extension/tasks/${task.id}/browser-credential`, {
      device_id: deviceID,
      lease_token: 'bad-token'
    }, { expected: 409, allowError: true })
    assert(deniedCredential.reason === 'EXTENSION_TASK_LEASE_MISMATCH', 'browser credential should require valid capture lease')

    const credential = await api('POST', `/api/v1/admin-plus/extension/tasks/${task.id}/browser-credential`, {
      device_id: deviceID,
      lease_token: task.lease_token
    })
    assert(credential.supplier_id === supplier.id, 'browser credential should belong to capture supplier')
    assert(credential.dashboard_url === supplierServer.url, 'browser credential should include supplier dashboard url')
    assert(credential.username === `${runID}-session@supplier.example.com`, 'browser credential should return configured username')
    assert(credential.password === 'e2e-session-password', 'browser credential should return configured password')
    assert(credential.token === 'e2e-session-token', 'browser credential should return configured token')

    const heartbeat = await api('POST', `/api/v1/admin-plus/extension/tasks/${task.id}/heartbeat`, {
      device_id: deviceID,
      lease_token: task.lease_token,
      lease_ttl_seconds: 60
    })
    assert(heartbeat.status === 'running', 'capture heartbeat should mark task running')

    const completed = await api('POST', `/api/v1/admin-plus/extension/tasks/${task.id}/complete`, {
      device_id: deviceID,
      lease_token: task.lease_token,
      result: {
        source: 'chrome',
        run_id: runID,
        captured_at: new Date().toISOString(),
        session_bundle: {
          supplier_id: supplier.id,
          origin: supplierServer.url,
          url: `${supplierServer.url}/dashboard`,
          captured_at: new Date().toISOString(),
          tokens: {
            access_token: 'e2e-browser-access-token',
            csrf_token: 'e2e-csrf-token'
          },
          required_headers: {
            cookie: 'sid=e2e-browser-cookie',
            origin: supplierServer.url,
            referer: `${supplierServer.url}/dashboard`
          },
          context: {
            api_base_url: supplierServer.url,
            user_id: `${runID}-supplier-user`
          }
        }
      }
    })
    assert(completed.status === 'succeeded', 'capture complete should mark task succeeded')
    assert(completed.result?.ingest?.session_captured === true, 'capture complete should ingest supplier session')
    assert(completed.result?.session_summary?.has_access_token === true, 'capture complete should keep only session summary')
    assert(!JSON.stringify(completed).includes('e2e-browser-access-token'), 'complete response should not expose access token')
    assert(!JSON.stringify(completed).includes('e2e-browser-cookie'), 'complete response should not expose cookie')

    const session = await api('GET', `/api/v1/admin-plus/suppliers/${supplier.id}/session`)
    assert(session.has_encrypted_bundle === true, 'captured session should be encrypted at rest')
    assert(session.source_extension_task_id === task.id, 'captured session should retain source task id')
    assert(session.session_summary?.has_access_token === true, 'session summary should report token presence')
    assert(!JSON.stringify(session).includes('e2e-browser-access-token'), 'session get should not expose access token')
    assert(!JSON.stringify(session).includes('e2e-browser-cookie'), 'session get should not expose cookie')

    const probed = await api('POST', `/api/v1/admin-plus/suppliers/${supplier.id}/session/probe`, {
      low_balance_threshold_cents: 2000
    })
    assert(probed.probe?.system_type === 'sub2api', 'session probe should use Sub2API provider adapter')
    assert(probed.probe?.balance_cents === 1234, 'session probe should read supplier profile balance')
    assert(probed.balance_snapshot?.balance_cents === 1234, 'session probe should record balance snapshot from uploaded session')
    assert(supplierServer.requests.some((request) => request.path === '/api/v1/user/profile'), 'session probe should call supplier profile endpoint')
    const profileRequest = supplierServer.requests.find((request) => request.path === '/api/v1/user/profile')
    assert(profileRequest.authorization === 'Bearer e2e-browser-access-token', 'session probe should send uploaded bearer token')
    assert(profileRequest.cookie === 'sid=e2e-browser-cookie', 'session probe should send uploaded cookie')
    assert(!JSON.stringify(probed).includes('e2e-browser-access-token'), 'session probe response should not expose access token')
    assert(!JSON.stringify(probed).includes('e2e-browser-cookie'), 'session probe response should not expose cookie')
    log('capture supplier session upload E2E verified')
  } finally {
    await supplierServer.close()
  }
}

async function exerciseScheduler(supplierID) {
  const status = await api('GET', '/api/v1/admin-plus/scheduler/status')
  assert(status.queue === 'admin_plus_extension_tasks', 'scheduler should use extension task queue')

  const first = await api('POST', '/api/v1/admin-plus/scheduler/run', {
    mode: 'e2e',
    supplier_id: supplierID,
    task_types: ['capture_supplier_session'],
    window_minutes: 10
  })
  assert(first.created_count === 1, 'scheduler should create one capture session task')
  assert(first.items.every((item) => item.schedule_key), 'scheduler items should include schedule keys')
  assert(first.items.every((item) => item.action === 'extension_task'), 'scheduler should keep only session capture in extension queue')

  const queued = await api('GET', `/api/v1/admin-plus/extension/tasks?supplier_id=${supplierID}&limit=100`)
  for (const item of first.items) {
    assert(queued.items.some((task) => task.id === item.task_id && task.schedule_key === item.schedule_key), 'scheduler-created task should be persisted in extension queue')
  }

  const second = await api('POST', '/api/v1/admin-plus/scheduler/run', {
    mode: 'e2e',
    supplier_id: supplierID,
    task_types: ['capture_supplier_session'],
    window_minutes: 10
  })
  assert(second.created_count === 0, 'scheduler should not duplicate tasks in the same window')
  assert(second.skipped_count === 1, 'scheduler should report skipped duplicate capture task')
  assert(second.items.every((item) => item.reason === 'duplicate'), 'scheduler duplicate skips should explain the reason')
  log('scheduler capture-session task generation verified')
}

async function exerciseActionRecommendations(supplier, candidateSupplier, balanceEvent, announcementEvent, healthEvent, reconciliationSummary) {
  const generated = await api('POST', '/api/v1/admin-plus/actions/generate', {
    suppliers: [{
      supplier_id: supplier.id,
      name: supplier.name,
      runtime_status: 'active',
      health_status: 'credential_invalid',
      balance_cents: 0,
      currency: supplier.balance_currency,
      effective_cost_cents: 120
    }, {
      supplier_id: candidateSupplier.id,
      name: candidateSupplier.name,
      runtime_status: 'candidate',
      health_status: 'normal',
      balance_cents: candidateSupplier.balance_cents,
      currency: candidateSupplier.balance_currency,
      effective_cost_cents: 80
    }],
    balance_events: [balanceEvent],
    announcement_events: [announcementEvent],
    health_events: [healthEvent],
    reconciliation: reconciliationSummary,
    min_profit_margin: 0.6
  })
  assert(generated.total > 0, 'actions generate should create recommendations')
  assert(generated.items.some((item) => item.type === 'switch_supplier'), 'actions should include switch supplier recommendation')
  assert(generated.items.some((item) => item.target_supplier_id === candidateSupplier.id), 'switch recommendation should target real candidate supplier')
  assert(generated.items.every((item) => item.requires_approval === true), 'actions should require approval')

  const list = await api('GET', `/api/v1/admin-plus/actions/recommendations?supplier_id=${supplier.id}&limit=20`)
  assert(list.total > 0, 'actions list should include generated recommendations')

  const updated = await api('PATCH', `/api/v1/admin-plus/actions/recommendations/${list.items[0].id}/status`, {
    status: 'acknowledged'
  })
  assert(updated.status === 'acknowledged', 'action recommendation status should update')
  log('action recommendations verified')
}

async function verifyAllListEndpoints(supplierID) {
  const checks = [
    `/api/v1/admin-plus/suppliers?q=${encodeURIComponent(runID)}`,
    `/api/v1/admin-plus/sub2api/accounts?q=${encodeURIComponent(runID)}&limit=20`,
    `/api/v1/admin-plus/sub2api/account-runtime?q=${encodeURIComponent(runID)}&limit=20`,
    `/api/v1/admin-plus/sub2api/usage-lines?limit=20`,
    `/api/v1/admin-plus/sub2api/usage-summary?limit=20`,
    `/api/v1/admin-plus/rates/snapshots?supplier_id=${supplierID}`,
    `/api/v1/admin-plus/rates/events?supplier_id=${supplierID}`,
    `/api/v1/admin-plus/balances/snapshots?supplier_id=${supplierID}`,
    `/api/v1/admin-plus/balances/events?supplier_id=${supplierID}`,
    `/api/v1/admin-plus/health/samples?supplier_id=${supplierID}`,
    `/api/v1/admin-plus/health/events?supplier_id=${supplierID}`,
    `/api/v1/admin-plus/announcements?supplier_id=${supplierID}`,
    `/api/v1/admin-plus/extension/tasks?supplier_id=${supplierID}`,
    `/api/v1/admin-plus/billing/lines?supplier_id=${supplierID}`,
    `/api/v1/admin-plus/actions/recommendations?limit=20`
  ]
  for (const path of checks) {
    const data = await api('GET', path)
    assert(Array.isArray(data.items), `${path} should return items array`)
  }
  log('all list endpoints verified')
}

async function api(method, path, body, options = {}) {
  const headers = {
    'Content-Type': 'application/json'
  }
  if (options.auth !== false) {
    headers.Authorization = `Bearer ${token}`
  }
  const response = await fetch(`${baseURL}${path}`, {
    method,
    headers,
    body: body === undefined ? undefined : JSON.stringify(body)
  })
  const text = await response.text()
  const json = parseJSON(text, `${method} ${path}`)
  const expected = Array.isArray(options.expected) ? options.expected : [options.expected || 200]
  assert(expected.includes(response.status), `${method} ${path} expected ${expected.join('/')} got ${response.status}`, text)
  if (options.allowError) {
    return json
  }
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
  } catch (error) {
    fail(`${label} should return JSON`, text)
  }
}

function assert(condition, message, details) {
  if (!condition) {
    fail(message, details)
  }
}

function fail(message, details) {
  const error = new Error(message)
  error.details = details
  throw error
}

function log(message) {
  console.log(`[OK] ${message}`)
}

function assertSafeE2ETarget() {
  if (allowNonLocal) return
  const apiURL = new URL(baseURL)
  const dbHost = dbURLHost(dbURL)
  const redisHost = redisURLHost(redisURL)
  assert(isLocalHost(apiURL.hostname), `refuse to run E2E against non-local API host: ${apiURL.hostname}`)
  assert(isLocalHost(dbHost), `refuse to run E2E against non-local database host: ${dbHost}`)
  assert(isLocalHost(redisHost), `refuse to run E2E against non-local Redis host: ${redisHost}`)
}

async function useRealOpenAICompatibleUpstream() {
  assert(realUpstreamAPIKey, 'ADMIN_PLUS_E2E_REAL_UPSTREAM_API_KEY is required when ADMIN_PLUS_E2E_REAL_UPSTREAM_BASE_URL is set')
  const requests = []
  const wrappedFetch = globalThis.fetch
  globalThis.fetch = async (input, init = {}) => {
    const url = typeof input === 'string' ? input : input.url
    if (String(url).startsWith(realUpstreamBaseURL)) {
      let body = null
      try {
        body = JSON.parse(init.body || '{}')
      } catch {
        body = null
      }
      requests.push({
        path: new URL(url).pathname,
        authorization: init.headers?.Authorization || init.headers?.authorization || '',
        contentType: init.headers?.['Content-Type'] || init.headers?.['content-type'] || '',
        body
      })
    }
    return wrappedFetch(input, init)
  }
  return {
    url: realUpstreamBaseURL,
    requests,
    close: async () => {
      globalThis.fetch = wrappedFetch
    }
  }
}

function startTestOpenAIResponsesServer() {
  const requests = []
  const server = createServer(async (req, res) => {
    if (req.method !== 'POST' || req.url !== '/v1/responses') {
      res.writeHead(404, { 'Content-Type': 'application/json' })
      res.end(JSON.stringify({ error: 'not found' }))
      return
    }

    const rawBody = await readRequestBody(req)
    let body = null
    try {
      body = JSON.parse(rawBody || '{}')
    } catch {
      body = null
    }
    requests.push({
      path: req.url,
      authorization: req.headers.authorization || '',
      contentType: req.headers['content-type'] || '',
      body
    })

    res.writeHead(200, {
      'Content-Type': 'text/event-stream',
      'Cache-Control': 'no-cache',
      Connection: 'keep-alive'
    })
    res.write('data: {"type":"response.output_text.delta","delta":"ok"}\n\n')
    res.write('data: [DONE]\n\n')
    res.end()
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

function startTestSub2APISupplierServer() {
  const requests = []
  const server = createServer(async (req, res) => {
    requests.push({
      method: req.method,
      path: new URL(req.url, 'http://127.0.0.1').pathname,
      authorization: req.headers.authorization || '',
      cookie: req.headers.cookie || '',
      origin: req.headers.origin || '',
      referer: req.headers.referer || ''
    })
    if (req.method === 'GET' && req.url === '/api/v1/user/profile') {
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

function cleanupE2EFixturesSafely() {
  try {
    cleanupE2EFixtures()
    log(`cleaned E2E fixtures for ${runID}`)
    return null
  } catch (error) {
    console.error(`[WARN] cleanup failed for ${runID}: ${error.message}`)
    return error
  }
}

function cleanupE2EFixtures() {
  const escapedRunID = sqlString(runID)
  const sql = `
    DELETE FROM admin_plus_notification_deliveries
    WHERE dedupe_key LIKE '%${escapedRunID}%'
       OR payload::text LIKE '%${escapedRunID}%';

    DELETE FROM admin_plus_action_recommendations
    WHERE reason_code LIKE '%${escapedRunID}%'
       OR title LIKE '%${escapedRunID}%'
       OR description LIKE '%${escapedRunID}%'
       OR expected_impact LIKE '%${escapedRunID}%'
       OR signals::text LIKE '%${escapedRunID}%';

    DELETE FROM admin_plus_supplier_bill_lines
    WHERE external_bill_id LIKE '${escapedRunID}%'
       OR external_request_id LIKE '${escapedRunID}%'
       OR model LIKE '${escapedRunID}%'
       OR raw_payload::text LIKE '%${escapedRunID}%';

    DELETE FROM admin_plus_extension_tasks
    WHERE device_id LIKE '${escapedRunID}%'
       OR schedule_key LIKE '%${escapedRunID}%'
       OR payload::text LIKE '%${escapedRunID}%'
       OR result::text LIKE '%${escapedRunID}%';

    DELETE FROM admin_plus_supplier_browser_sessions
    WHERE supplier_id IN (SELECT id FROM admin_plus_suppliers WHERE name LIKE '${escapedRunID}%')
       OR session_summary::text LIKE '%${escapedRunID}%';

    DELETE FROM admin_plus_health_events
    WHERE supplier_id IN (SELECT id FROM admin_plus_suppliers WHERE name LIKE '${escapedRunID}%')
       OR model LIKE '${escapedRunID}%';

    DELETE FROM admin_plus_health_samples
    WHERE supplier_id IN (SELECT id FROM admin_plus_suppliers WHERE name LIKE '${escapedRunID}%')
       OR model LIKE '${escapedRunID}%'
       OR raw_payload::text LIKE '%${escapedRunID}%';

    DELETE FROM admin_plus_announcement_events
    WHERE supplier_id IN (SELECT id FROM admin_plus_suppliers WHERE name LIKE '${escapedRunID}%')
       OR title LIKE '%${escapedRunID}%'
       OR description LIKE '%${escapedRunID}%'
       OR raw_payload::text LIKE '%${escapedRunID}%';

    DELETE FROM admin_plus_balance_events
    WHERE supplier_id IN (SELECT id FROM admin_plus_suppliers WHERE name LIKE '${escapedRunID}%');

    DELETE FROM admin_plus_balance_snapshots
    WHERE supplier_id IN (SELECT id FROM admin_plus_suppliers WHERE name LIKE '${escapedRunID}%')
       OR raw_payload::text LIKE '%${escapedRunID}%';

    DELETE FROM admin_plus_rate_change_events
    WHERE supplier_id IN (SELECT id FROM admin_plus_suppliers WHERE name LIKE '${escapedRunID}%')
       OR model LIKE '${escapedRunID}%';

    DELETE FROM admin_plus_rate_snapshots
    WHERE supplier_id IN (SELECT id FROM admin_plus_suppliers WHERE name LIKE '${escapedRunID}%')
       OR model LIKE '${escapedRunID}%'
       OR raw_payload::text LIKE '%${escapedRunID}%';

    DELETE FROM admin_plus_supplier_accounts
    WHERE supplier_id IN (SELECT id FROM admin_plus_suppliers WHERE name LIKE '${escapedRunID}%')
       OR supplier_account_identifier LIKE '${escapedRunID}%';

    DELETE FROM admin_plus_suppliers
    WHERE name LIKE '${escapedRunID}%'
       OR contact LIKE '%${escapedRunID}%'
       OR notes LIKE '%${escapedRunID}%';

    DELETE FROM usage_logs
    WHERE request_id LIKE '${escapedRunID}%'
       OR model LIKE '${escapedRunID}%'
       OR requested_model LIKE '${escapedRunID}%';

    DELETE FROM api_keys
    WHERE key LIKE 'sk-${escapedRunID}%'
       OR name LIKE '${escapedRunID}%';

    DELETE FROM accounts
    WHERE name LIKE '${escapedRunID}%'
       OR extra::text LIKE '%${escapedRunID}%';

    DELETE FROM users
    WHERE email LIKE '${escapedRunID}@e2e.local';
  `
  execFileSync('psql', [dbURL, '-v', 'ON_ERROR_STOP=1', '-c', sql], {
    encoding: 'utf8',
    stdio: ['ignore', 'pipe', 'pipe']
  })
  cleanupRuntimeRedisFixture()
}

function cleanupRuntimeRedisFixture() {
  if (!localAccountIDForCleanup) return
  execFileSync('redis-cli', ['-u', redisURL, 'DEL', `concurrency:account:${localAccountIDForCleanup}`, `wait:account:${localAccountIDForCleanup}`], {
    encoding: 'utf8',
    stdio: ['ignore', 'pipe', 'pipe']
  })
}

function readRequestBody(req) {
  return new Promise((resolve, reject) => {
    let body = ''
    req.setEncoding('utf8')
    req.on('data', (chunk) => {
      body += chunk
    })
    req.on('end', () => resolve(body))
    req.on('error', reject)
  })
}

function trimTrailingSlash(value) {
  return value.replace(/\/+$/, '')
}

function isLocalHost(hostname) {
  return ['localhost', '127.0.0.1', '::1'].includes(hostname)
}

function dbURLHost(value) {
  return new URL(value).hostname
}

function redisURLHost(value) {
  return new URL(value).hostname
}

function sqlString(value) {
  return String(value).replaceAll("'", "''")
}

function parseReturningID(output) {
  const line = String(output)
    .split(/\r?\n/)
    .map((value) => value.trim())
    .find((value) => /^\d+$/.test(value))
  return Number(line)
}
