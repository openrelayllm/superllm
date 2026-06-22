(() => {
const CONFIG_KEY = 'adminPlusOperatorConfig'
const LAST_CAPTURE_RESULT_KEY = 'adminPlusLastCaptureResult'
const DEFAULT_CONFIG_PATH = 'config/default-config.json'

async function loadConfig() {
  const stored = await chrome.storage.local.get(CONFIG_KEY)
  const config = stored[CONFIG_KEY] || {}
  const defaultConfig = await loadDefaultConfig()
  if (!config.deviceID) {
    config.deviceID = `admin-plus-chrome-${crypto.randomUUID()}`
    await saveConfig(config)
  }
  return {
    baseURL: config.baseURL || defaultConfig.baseURL || '',
    token: config.token || '',
    deviceID: config.deviceID,
    taskTypes: Array.isArray(config.taskTypes) ? config.taskTypes : [],
    connectedAt: config.connectedAt || ''
  }
}

async function saveConfig(config) {
  await chrome.storage.local.set({
    [CONFIG_KEY]: {
      baseURL: config.baseURL || '',
      token: config.token || '',
      deviceID: config.deviceID || `admin-plus-chrome-${crypto.randomUUID()}`,
      taskTypes: Array.isArray(config.taskTypes) ? config.taskTypes : [],
      connectedAt: config.connectedAt || ''
    }
  })
}

async function saveBaseURL(baseURL) {
  const normalized = normalizeBaseURL(baseURL)
  const config = await loadConfig()
  const changed = normalizeBaseURL(config.baseURL) !== normalized
  await saveConfig({
    ...config,
    baseURL: normalized,
    token: changed ? '' : config.token,
    connectedAt: changed ? '' : config.connectedAt
  })
}

async function loadLastCaptureResult() {
  const stored = await chrome.storage.local.get(LAST_CAPTURE_RESULT_KEY)
  return stored[LAST_CAPTURE_RESULT_KEY] || null
}

async function saveLastCaptureResult(result) {
  await chrome.storage.local.set({
    [LAST_CAPTURE_RESULT_KEY]: {
      status: result.status || 'failed',
      message: result.message || '',
      supplier: result.supplier || '',
      host: result.host || '',
      taskID: result.taskID || 0,
      summary: result.summary || {},
      recordedAt: result.recordedAt || new Date().toISOString()
    }
  })
}

async function loadDefaultConfig() {
  try {
    const response = await fetch(chrome.runtime.getURL(DEFAULT_CONFIG_PATH), {
      cache: 'no-store'
    })
    if (!response.ok) return {}
    const config = await response.json()
    return {
      baseURL: normalizeBaseURL(config.baseURL)
    }
  } catch {
    return {}
  }
}

function normalizeBaseURL(value) {
  let raw = String(value || '').trim().replace(/\/+$/, '')
  if (!raw) return ''
  if (!/^[a-z][a-z\d+\-.]*:\/\//i.test(raw)) {
    raw = `${usesLocalHTTPByDefault(raw) ? 'http' : 'https'}://${raw}`
  }
  try {
    const url = new URL(raw)
    if (!['http:', 'https:'].includes(url.protocol)) return ''
    return url.origin
  } catch {
    return ''
  }
}

function usesLocalHTTPByDefault(value) {
  const raw = String(value || '').trim().toLowerCase()
  return raw.startsWith('localhost') ||
    raw.startsWith('127.') ||
    raw.startsWith('0.0.0.0') ||
    raw.startsWith('[::1]') ||
    raw.startsWith('::1')
}


class AdminPlusClient {
  constructor(config) {
    this.baseURL = trimTrailingSlash(config.baseURL || '')
    this.token = config.token || ''
  }

  ready() {
    return this.baseURL !== '' && this.token !== ''
  }

  async listSuppliers() {
    const page = await this.request('/api/v1/admin-plus/suppliers?page=1&page_size=500')
    return Array.isArray(page?.items) ? page.items : []
  }

  async createCaptureSessionTask(deviceID, supplierID, payload = {}) {
    return this.request('/api/v1/admin-plus/extension/session/capture-task', {
      method: 'POST',
      body: {
        device_id: deviceID,
        ...(supplierID ? { supplier_id: supplierID } : {}),
        lease_ttl_seconds: 300,
        url: payload.source_url || '',
        host: payload.source_host || '',
        origin: payload.source_origin || '',
        dashboard_url: payload.dashboard_url || '',
        api_base_url: payload.api_base_url || '',
        third_party_recharge_url: payload.third_party_recharge_url || '',
        local_recharge_url: payload.local_recharge_url || '',
        auto_create_supplier: payload.auto_create_supplier,
        page_context: payload.page_context || {},
        payload
      }
    })
  }

  async createDiscoveredSupplier(payload) {
    return this.request('/api/v1/admin-plus/suppliers/from-site-candidate', {
      method: 'POST',
      body: payload
    })
  }

  async claimTask(deviceID, types = []) {
    return this.request('/api/v1/admin-plus/extension/tasks/claim', {
      method: 'POST',
      body: {
        device_id: deviceID,
        types,
        lease_ttl_seconds: 300
      }
    })
  }

  async heartbeat(task, leaseTTLSeconds = 300) {
    return this.request(`/api/v1/admin-plus/extension/tasks/${task.id}/heartbeat`, {
      method: 'POST',
      body: {
        device_id: task.device_id,
        lease_token: task.lease_token,
        lease_ttl_seconds: leaseTTLSeconds
      }
    })
  }

  async browserCredential(task) {
    return this.request(`/api/v1/admin-plus/extension/tasks/${task.id}/browser-credential`, {
      method: 'POST',
      body: {
        device_id: task.device_id,
        lease_token: task.lease_token
      }
    })
  }

  async complete(task, result) {
    return this.request(`/api/v1/admin-plus/extension/tasks/${task.id}/complete`, {
      method: 'POST',
      body: {
        device_id: task.device_id,
        lease_token: task.lease_token,
        result
      }
    })
  }

  async fail(task, errorCode, errorMessage) {
    return this.request(`/api/v1/admin-plus/extension/tasks/${task.id}/fail`, {
      method: 'POST',
      body: {
        device_id: task.device_id,
        lease_token: task.lease_token,
        error_code: errorCode,
        error_message: errorMessage
      }
    })
  }

  async request(path, options = {}) {
    if (!this.ready()) {
      throw new Error('Admin Plus URL and token are required')
    }
    const response = await fetch(`${this.baseURL}${path}`, {
      method: options.method || 'GET',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${this.token}`
      },
      body: options.body === undefined ? undefined : JSON.stringify(options.body)
    })
    const text = await response.text()
    const json = parseJSON(text)
    if (!response.ok || json.code !== 0) {
      const reason = json.reason || `HTTP_${response.status}`
      const message = json.message || text || 'Admin Plus request failed'
      const error = new Error(message)
      error.reason = reason
      throw error
    }
    return json.data
  }
}

function trimTrailingSlash(value) {
  return String(value || '').replace(/\/+$/, '')
}

function parseJSON(text) {
  try {
    return JSON.parse(text)
  } catch {
    throw new Error('Admin Plus did not return JSON')
  }
}



globalThis.adminPlusHandleMessage = handleMessage

const ADMIN_PLUS_PATH_HINTS = [
  '/admin',
  '/admin/operations',
  '/admin/operations/scheduler',
  '/admin/operations/suppliers',
  '/dashboard',
  '/admin/dashboard',
  '/login',
  '/setup'
]

async function handleMessage(message) {
  switch (message?.type) {
    case 'state:get':
      return getState()
    case 'capture:last-result':
      return loadLastCaptureResult()
    case 'connect:from-active-tab':
      return connectFromActiveTab(message.baseURL)
    case 'connect:open-admin-plus':
      return openAdminPlus(message.baseURL)
    case 'connect:save-base-url':
      return saveAdminPlusBaseURL(message.baseURL)
    case 'site:identify':
      return identifyCurrentSite()
    case 'session:capture':
      return captureSupplierSession(message.supplierID, message.autoCreate !== false)
    default:
      throw new Error('Unsupported extension message')
  }
}

async function getState() {
  const config = await loadConfig()
  const activeTab = await getActiveTab()
  let connection = {
    status: config.baseURL && config.token ? 'connected' : 'disconnected',
    baseURL: config.baseURL,
    deviceID: config.deviceID,
    connectedAt: config.connectedAt
  }
  if (connection.status === 'connected') {
    try {
      const client = new AdminPlusClient(config)
      await client.listSuppliers()
    } catch (error) {
      connection = {
        ...connection,
        status: 'expired_or_invalid',
        error: error.message || String(error),
        reason: error.reason || 'ADMIN_PLUS_AUTH_INVALID'
      }
    }
  }
  return {
    connection,
    activeTab: summarizeTab(activeTab)
  }
}

async function connectFromActiveTab(baseURL = '') {
  const config = await loadConfig()
  const targetBaseURL = normalizeBaseURL(baseURL || config.baseURL || '')
  const tabs = await adminPlusCandidateTabs(targetBaseURL)
  if (tabs.length === 0) {
    const error = new Error('未找到已打开的 Admin Plus 后台标签页，请先在浏览器中打开并登录后台')
    error.reason = 'ADMIN_PLUS_LOGIN_REQUIRED'
    throw error
  }

  let lastError = null
  for (const tab of tabs) {
    const result = await readAdminPlusAuthFromTab(tab)
    if (!result?.token) continue
    const apiBaseURL = targetBaseURL || result.origin
    const candidate = {
      ...config,
      baseURL: apiBaseURL,
      token: result.token,
      connectedAt: new Date().toISOString()
    }
    try {
      await verifyAdminPlusConnection(candidate, result)
      await saveConfig(candidate)
      return getState()
    } catch (error) {
      lastError = error
    }
  }

  if (lastError) {
    throw lastError
  }
  const error = new Error(targetBaseURL ? '未找到已登录的 Admin Plus 标签页，请确认后台页面已登录' : '当前页面没有 Admin Plus 登录态，请先在 Web 页面完成登录')
  error.reason = 'ADMIN_PLUS_LOGIN_REQUIRED'
  throw error
}

async function adminPlusCandidateTabs(targetBaseURL) {
  const active = await getActiveTab()
  const tabs = []
  if (active?.id && isLikelyAdminPlusTab(active, targetBaseURL)) {
    tabs.push(active)
  }
  const allTabs = await chrome.tabs.query({})
  for (const tab of allTabs) {
    if (!tab?.id || !isLikelyAdminPlusTab(tab, targetBaseURL)) continue
    if (tabs.some((item) => item.id === tab.id)) continue
    tabs.push(tab)
  }
  return tabs
}

async function readAdminPlusAuthFromTab(tab) {
  if (!tab?.id) return null
  try {
    const injected = await chrome.scripting.executeScript({
      target: { tabId: tab.id },
      func: () => {
        const get = (area, key) => {
          try {
            return area.getItem(key) || ''
          } catch {
            return ''
          }
        }
        return {
          token: get(window.localStorage, 'auth_token') || get(window.sessionStorage, 'auth_token'),
          refreshToken: get(window.localStorage, 'refresh_token') || get(window.sessionStorage, 'refresh_token'),
          expiresAt: get(window.localStorage, 'token_expires_at') || get(window.sessionStorage, 'token_expires_at'),
          origin: window.location.origin,
          path: window.location.pathname,
          url: window.location.href
        }
      }
    })
    return injected[0]?.result || null
  } catch {
    return null
  }
}

async function verifyAdminPlusConnection(config, tabAuth) {
  try {
    const client = new AdminPlusClient(config)
    await client.listSuppliers()
    return
  } catch (error) {
    const looksLikeAdminPlus = hasAdminPlusPathHint(tabAuth?.path)
    const reason = looksLikeAdminPlus ? 'ADMIN_PLUS_AUTH_INVALID' : 'ADMIN_PLUS_PAGE_REQUIRED'
    const message = looksLikeAdminPlus
      ? 'Admin Plus 登录态无效，请刷新 Web 页面或重新登录'
      : '当前页面不是已登录的 Admin Plus 后台页面'
    const wrapped = new Error(message)
    wrapped.reason = reason
    wrapped.cause = error
    throw wrapped
  }
}

function hasAdminPlusPathHint(path) {
  const value = String(path || '')
  return ADMIN_PLUS_PATH_HINTS.some((hint) => value === hint || value.startsWith(`${hint}/`))
}

function isLikelyAdminPlusTab(tab, targetBaseURL) {
  if (!tab?.id) return false
  const parsed = safeURL(tab.url || '')
  if (!parsed || !/^https?:$/.test(parsed.protocol)) return false
  if (!targetBaseURL) return hasAdminPlusPathHint(parsed.pathname)
  if (sameOriginOrURL(parsed.href, targetBaseURL)) return true
  return isLocalURL(parsed.href) && isLocalURL(targetBaseURL) && hasAdminPlusPathHint(parsed.pathname)
}

function isLocalURL(value) {
  const parsed = safeURL(value)
  if (!parsed) return false
  const hostname = parsed.hostname.toLowerCase()
  return hostname === 'localhost' || hostname === '127.0.0.1' || hostname === '0.0.0.0' || hostname === '::1'
}

async function openAdminPlus(baseURL) {
  const config = await loadConfig()
  const targetBaseURL = trimTrailingSlash(baseURL || config.baseURL || '')
  if (!targetBaseURL) {
    const error = new Error('请先填写 Admin Plus 后端地址')
    error.reason = 'ADMIN_PLUS_BASE_URL_REQUIRED'
    throw error
  }
  const url = `${targetBaseURL}/login?redirect=${encodeURIComponent('/admin/operations/scheduler')}`
  await chrome.tabs.create({ url })
  return { opened: true, url }
}

async function saveAdminPlusBaseURL(baseURL) {
  const normalized = normalizeBaseURL(baseURL)
  if (!normalized) {
    const error = new Error('请输入有效的 Admin Plus 后端地址')
    error.reason = 'ADMIN_PLUS_BASE_URL_INVALID'
    throw error
  }
  await saveBaseURL(normalized)
  return getState()
}

async function identifyCurrentSite() {
  const config = await requireConnectedConfig()
  const tab = await getActiveTab()
  const currentURL = safeURL(tab?.url || '')
  if (!currentURL || !/^https?:$/.test(currentURL.protocol)) {
    return {
      status: 'unsupported',
      activeTab: summarizeTab(tab),
      message: '当前页面不支持供应商识别'
    }
  }
  const client = new AdminPlusClient(config)
  const suppliers = await client.listSuppliers()
  const matches = suppliers
    .map((supplier) => ({ supplier, score: matchSupplier(currentURL, supplier) }))
    .filter((item) => item.score > 0)
    .sort((a, b) => b.score - a.score)

  if (matches.length === 0) {
    return {
      status: 'unknown',
      activeTab: summarizeTab(tab),
      message: '当前网站未匹配已配置供应商'
    }
  }
  const topScore = matches[0].score
  const topMatches = matches.filter((item) => item.score === topScore)
  if (topMatches.length > 1) {
    return {
      status: 'ambiguous',
      activeTab: summarizeTab(tab),
      suppliers: topMatches.map((item) => supplierSummary(item.supplier, item.score)),
      message: '当前网站匹配多个供应商'
    }
  }
  const supplier = topMatches[0].supplier
  return {
    status: supplier.credential?.browser_login_enabled ? 'matched' : 'unsupported',
    activeTab: summarizeTab(tab),
    supplier: supplierSummary(supplier, topScore),
    supplierLogin: await detectSupplierLogin(tab.id),
    message: supplier.credential?.browser_login_enabled ? '' : '该供应商未启用浏览器登录'
  }
}

async function captureSupplierSession(supplierID, autoCreate) {
  const config = await requireConnectedConfig()
  const identification = await identifyCurrentSite()
  const client = new AdminPlusClient(config)
  let supplier = resolveSupplierFromIdentification(identification, supplierID)
  if (!supplier) {
    supplier = supplierFromCurrentSite(identification, autoCreate)
  }
  const activeTabURL = identification.activeTab?.url || ''
  const thirdPartyRechargeURL = inferThirdPartyRechargeURL(activeTabURL)

  const task = await client.createCaptureSessionTask(config.deviceID, supplier.id, {
    source_url: activeTabURL,
    source_host: identification.activeTab?.host || '',
    source_origin: identification.activeTab?.origin || '',
    dashboard_url: supplier.dashboard_url || activeTabURL,
    api_base_url: supplier.api_base_url || identification.activeTab?.origin || '',
    third_party_recharge_url: thirdPartyRechargeURL,
    local_recharge_url: supplier.local_recharge_url || '',
    auto_create_supplier: Boolean(autoCreate),
    page_context: {
      title: identification.activeTab?.title || ''
    }
  })
  supplier = {
    ...supplier,
    id: task.supplier_id,
    supplier_id: task.supplier_id,
    dashboard_url: supplier.dashboard_url || activeTabURL,
    api_base_url: supplier.api_base_url || identification.activeTab?.origin || ''
  }
  await client.heartbeat(task)
  const tab = await ensureSupplierTab(supplier.dashboard_url || identification.activeTab?.url)
  try {
    const response = await runCaptureInTab(tab.id, task, supplier)
    if (!response?.ok) {
      await client.fail(task, response?.error_code || 'SESSION_CAPTURE_FAILED', response?.error_message || 'session capture failed')
      await recordCaptureResult('failed', response?.error_message || '上报失败', task, supplier, identification.activeTab)
      return { task, status: 'failed', result: response }
    }
    const captureResult = await enrichCaptureResult(response.result, supplier, tab.url || identification.activeTab?.url || '')
    if (!hasSessionEvidence(captureResult.session_bundle)) {
      const failed = { ok: false, error_code: 'SESSION_INCOMPLETE', error_message: 'no supported supplier token or cookie was found' }
      await client.fail(task, failed.error_code, failed.error_message)
      await recordCaptureResult('failed', '未找到可上报的 token 或 cookie', task, supplier, identification.activeTab)
      return { task, status: 'failed', result: failed }
    }
    const completed = await client.complete(task, captureResult)
    await recordCaptureResult('succeeded', '上报成功', completed, supplier, identification.activeTab, completed.result?.session_summary || captureResult.session_summary || {})
    return {
      task: completed,
      status: 'succeeded',
      supplier,
      result: {
        session_summary: completed.result?.session_summary || captureResult.session_summary || {},
        ingest: completed.result?.ingest || {}
      }
    }
  } catch (error) {
    await client.fail(task, error.reason || 'SESSION_CAPTURE_FAILED', error.message || String(error)).catch(() => {})
    await recordCaptureResult('failed', error.message || String(error), task, supplier, identification.activeTab).catch(() => {})
    throw error
  }
}

async function recordCaptureResult(status, message, task, supplier, activeTab, summary = {}) {
  await saveLastCaptureResult({
    status,
    message,
    supplier: supplier?.name || '',
    host: activeTab?.host || '',
    taskID: task?.id || 0,
    summary,
    recordedAt: new Date().toISOString()
  })
  await setCaptureBadge(status)
}

async function setCaptureBadge(status) {
  if (!chrome.action?.setBadgeText) return
  const ok = status === 'succeeded'
  await chrome.action.setBadgeText({ text: ok ? 'OK' : 'ERR' })
  await chrome.action.setBadgeBackgroundColor({ color: ok ? '#12b76a' : '#f04438' })
}

async function runCaptureInTab(tabId, task, supplier) {
  for (let attempt = 0; attempt < 4; attempt++) {
    await waitForTabComplete(tabId)
    const response = await sendContentMessage(tabId, {
      type: 'admin-plus:capture-session',
      task,
      supplier
    })
    return response
  }
  return { ok: false, error_code: 'SESSION_CAPTURE_FAILED', error_message: 'session capture failed' }
}

async function enrichCaptureResult(result, supplier, fallbackURL) {
  const bundle = result?.session_bundle || {}
  const captureURL = bundle.url || fallbackURL || supplier.dashboard_url || ''
  const cookies = await collectCookies(captureURL)
  const mergedBundle = {
    ...bundle,
    supplier_id: bundle.supplier_id || supplier.id || supplier.supplier_id,
    cookies,
    required_headers: {
      ...(bundle.required_headers || {}),
      cookie: cookies.map((cookie) => `${cookie.name}=${cookie.value}`).join('; ')
    }
  }
  return {
    ...result,
    session_bundle: mergedBundle,
    session_summary: summarizeSessionBundle(mergedBundle)
  }
}

async function discoverSupplierFromCurrentSite(client, identification) {
  const tab = identification.activeTab || summarizeTab(await getActiveTab())
  const parsed = safeURL(tab?.url || '')
  if (!parsed || !/^https?:$/.test(parsed.protocol)) {
    const error = new Error('当前页面不能自动创建供应商')
    error.reason = 'SUPPLIER_SITE_UNSUPPORTED'
    throw error
  }
  const supplier = await client.createDiscoveredSupplier({
    name: suggestedSupplierName(parsed, tab),
    dashboard_url: parsed.href,
    api_base_url: parsed.origin,
    third_party_recharge_url: inferThirdPartyRechargeURL(parsed.href),
    source_host: parsed.host,
    source_url: parsed.href
  })
  return {
    ...supplier,
    auto_created: true,
    score: 100
  }
}

function supplierFromCurrentSite(identification, autoCreate) {
  if (!autoCreate) {
    const error = new Error('当前网站未匹配可上报的供应商')
    error.reason = 'SUPPLIER_SITE_NOT_MATCHED'
    throw error
  }
  const tab = identification.activeTab || {}
  return {
    id: 0,
    supplier_id: 0,
    name: tab.host || '当前供应商',
    dashboard_url: tab.url || '',
    api_base_url: tab.origin || '',
    third_party_recharge_url: inferThirdPartyRechargeURL(tab.url || ''),
    local_recharge_url: ''
  }
}

async function collectCookies(url) {
  if (!url) return []
  try {
    const cookies = await chrome.cookies.getAll({ url })
    return cookies.map((cookie) => ({
      name: cookie.name,
      value: cookie.value,
      domain: cookie.domain,
      path: cookie.path,
      secure: cookie.secure,
      http_only: cookie.httpOnly,
      same_site: cookie.sameSite,
      expiration_date: cookie.expirationDate || null
    }))
  } catch {
    return []
  }
}

function hasSessionEvidence(bundle) {
  const tokens = bundle?.tokens || {}
  return Boolean(tokens.access_token || tokens.refresh_token || tokens.csrf_token || (bundle?.cookies || []).length > 0)
}

function summarizeSessionBundle(bundle) {
  const tokens = bundle?.tokens || {}
  const context = bundle?.context || {}
  const headers = bundle?.required_headers || {}
  return {
    origin: bundle?.origin || '',
    captured_at: bundle?.captured_at || '',
    expires_at: bundle?.expires_at || '',
    has_access_token: Boolean(tokens.access_token),
    has_refresh_token: Boolean(tokens.refresh_token),
    has_csrf_token: Boolean(tokens.csrf_token),
    cookie_count: (bundle?.cookies || []).length,
    api_base_url: context.api_base_url || '',
    organization_id: context.organization_id || '',
    project_id: context.project_id || '',
    account_id: context.account_id || '',
    has_required_origin: Boolean(headers.origin),
    has_required_referer: Boolean(headers.referer),
    has_required_cookie: Boolean(headers.cookie)
  }
}

async function ensureSupplierTab(url) {
  const active = await getActiveTab()
  if (active?.id && active.url && sameOriginOrURL(active.url, url)) {
    return active
  }
  if (!url) {
    throw new Error('supplier dashboard url is required')
  }
  const tab = await chrome.tabs.create({ url, active: true })
  return tab
}

async function detectSupplierLogin(tabId) {
  if (!tabId) return { status: 'unknown' }
  try {
    const response = await sendContentMessage(tabId, { type: 'admin-plus:detect-login' })
    return response || { status: 'unknown' }
  } catch {
    return { status: 'unknown' }
  }
}

async function sendContentMessage(tabId, message) {
  try {
    return await chrome.tabs.sendMessage(tabId, message)
  } catch (error) {
    if (!isMissingContentScriptError(error)) throw error
  }
  await injectContentScript(tabId)
  await sleep(150)
  return chrome.tabs.sendMessage(tabId, message)
}

async function injectContentScript(tabId) {
  try {
    await chrome.scripting.executeScript({
      target: { tabId },
      files: ['src/content/sub2api.js']
    })
  } catch (error) {
    const wrapped = new Error('当前页面无法注入插件脚本，请刷新供应商页面后重试')
    wrapped.reason = 'CONTENT_SCRIPT_INJECTION_FAILED'
    wrapped.cause = error
    throw wrapped
  }
}

function isMissingContentScriptError(error) {
  const message = String(error?.message || error || '')
  return message.includes('Receiving end does not exist') || message.includes('Could not establish connection')
}

async function requireConnectedConfig() {
  const config = await loadConfig()
  if (!config.baseURL || !config.token) {
    const error = new Error('请先连接 Admin Plus')
    error.reason = 'ADMIN_PLUS_NOT_CONNECTED'
    throw error
  }
  return config
}

async function getActiveTab() {
  const [tab] = await chrome.tabs.query({ active: true, currentWindow: true })
  return tab || null
}

function summarizeTab(tab) {
  const parsed = safeURL(tab?.url || '')
  return {
    id: tab?.id,
    title: tab?.title || '',
    url: tab?.url || '',
    host: parsed?.host || ''
  }
}

function matchSupplier(currentURL, supplier) {
  const dashboardURL = safeURL(supplier.dashboard_url || '')
  const apiURL = safeURL(supplier.api_base_url || '')
  if (dashboardURL && currentURL.origin === dashboardURL.origin) return currentURL.pathname.startsWith(dashboardURL.pathname || '/') ? 100 : 90
  if (apiURL && currentURL.origin === apiURL.origin) return 70
  return 0
}

function supplierSummary(supplier, score) {
  return {
    id: supplier.id,
    name: supplier.name,
    kind: supplier.kind,
    type: supplier.type,
    dashboard_url: supplier.dashboard_url || '',
    api_base_url: supplier.api_base_url || '',
    third_party_recharge_url: supplier.third_party_recharge_url || '',
    local_recharge_url: supplier.local_recharge_url || '',
    credential: supplier.credential || {},
    score
  }
}

function suggestedSupplierName(url, tab) {
  const title = String(tab?.title || '').trim()
  if (title) return title.slice(0, 80)
  return url.host
}

function resolveSupplierFromIdentification(identification, supplierID) {
  if (identification.status === 'matched' && identification.supplier && (!supplierID || identification.supplier.id === supplierID)) {
    return identification.supplier
  }
  if (identification.status === 'ambiguous' && supplierID) {
    return (identification.suppliers || []).find((supplier) => supplier.id === supplierID)
  }
  return null
}

function safeURL(value) {
  try {
    return value ? new URL(value) : null
  } catch {
    return null
  }
}

function inferThirdPartyRechargeURL(value) {
  const parsed = safeURL(value)
  if (!parsed || !/^https?:$/.test(parsed.protocol)) return ''
  const path = parsed.pathname.toLowerCase()
  const markers = ['/custom/', '/recharge', '/payment', '/topup', '/redeem', '/card', '/pay']
  return markers.some((marker) => path.includes(marker)) ? parsed.href : ''
}

function sameOriginOrURL(left, right) {
  const a = safeURL(left)
  const b = safeURL(right)
  if (!a || !b) return false
  return a.origin === b.origin
}

function waitForTabComplete(tabId) {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => {
      chrome.tabs.onUpdated.removeListener(listener)
      reject(new Error('supplier page load timed out'))
    }, 30000)
    const listener = (updatedTabId, changeInfo) => {
      if (updatedTabId !== tabId || changeInfo.status !== 'complete') return
      clearTimeout(timer)
      chrome.tabs.onUpdated.removeListener(listener)
      resolve()
    }
    chrome.tabs.onUpdated.addListener(listener)
    chrome.tabs.get(tabId, (tab) => {
      if (chrome.runtime.lastError) {
        clearTimeout(timer)
        chrome.tabs.onUpdated.removeListener(listener)
        reject(new Error(chrome.runtime.lastError.message))
        return
      }
      if (tab.status === 'complete') {
        clearTimeout(timer)
        chrome.tabs.onUpdated.removeListener(listener)
        resolve()
      }
    })
  })
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}
})()
