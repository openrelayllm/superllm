import { AdminPlusClient, trimTrailingSlash } from './lib/admin-plus-client.js'
import { loadConfig, saveConfig } from './lib/storage.js'

const ADMIN_PLUS_PATH_HINTS = [
  '/admin/operations/scheduler',
  '/admin/operations/suppliers',
  '/admin/dashboard',
  '/login'
]

chrome.runtime.onMessage.addListener((message, _sender, sendResponse) => {
  handleMessage(message)
    .then((result) => sendResponse({ ok: true, result }))
    .catch((error) => sendResponse({
      ok: false,
      error: {
        reason: error.reason || 'EXTENSION_ERROR',
        message: error.message || String(error)
      }
    }))
  return true
})

async function handleMessage(message) {
  switch (message?.type) {
    case 'state:get':
      return getState()
    case 'connect:from-active-tab':
      return connectFromActiveTab()
    case 'connect:open-admin-plus':
      return openAdminPlus(message.baseURL)
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

async function connectFromActiveTab() {
  const tab = await getActiveTab()
  if (!tab?.id) {
    throw new Error('No active tab')
  }
  const injected = await chrome.scripting.executeScript({
    target: { tabId: tab.id },
    func: () => ({
      token: window.localStorage.getItem('auth_token') || '',
      refreshToken: window.localStorage.getItem('refresh_token') || '',
      expiresAt: window.localStorage.getItem('token_expires_at') || '',
      origin: window.location.origin,
      path: window.location.pathname
    })
  })
  const result = injected[0]?.result || {}
  if (!result.token) {
    const error = new Error('当前页面没有 sub2apiplus 登录态，请先在 Web 页面完成登录')
    error.reason = 'ADMIN_PLUS_LOGIN_REQUIRED'
    throw error
  }
  const config = await loadConfig()
  const candidate = {
    ...config,
    baseURL: result.origin,
    token: result.token,
    connectedAt: new Date().toISOString()
  }
  await verifyAdminPlusConnection(candidate, result.path)
  const next = {
    ...candidate
  }
  await saveConfig(next)
  return getState()
}

async function verifyAdminPlusConnection(config, path) {
  try {
    const client = new AdminPlusClient(config)
    await client.listSuppliers()
    return
  } catch (error) {
    const reason = hasAdminPlusPathHint(path) ? 'ADMIN_PLUS_AUTH_INVALID' : 'ADMIN_PLUS_PAGE_REQUIRED'
    const message = hasAdminPlusPathHint(path)
      ? 'sub2apiplus 登录态无效，请刷新 Web 页面或重新登录'
      : '当前页面不是已登录的 sub2apiplus 后台页面'
    const wrapped = new Error(message)
    wrapped.reason = reason
    wrapped.cause = error
    throw wrapped
  }
}

function hasAdminPlusPathHint(path) {
  return ADMIN_PLUS_PATH_HINTS.some((hint) => String(path || '').startsWith(hint))
}

async function openAdminPlus(baseURL) {
  const config = await loadConfig()
  const targetBaseURL = trimTrailingSlash(baseURL || config.baseURL || '')
  if (!targetBaseURL) {
    const error = new Error('插件包缺少 sub2apiplus 地址，请从当前后台重新下载插件包')
    error.reason = 'ADMIN_PLUS_BASE_URL_REQUIRED'
    throw error
  }
  const url = `${targetBaseURL}/login?redirect=${encodeURIComponent('/admin/operations/scheduler')}`
  await chrome.tabs.create({ url })
  return { opened: true, url }
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
      return { task, status: 'failed', result: response }
    }
    const captureResult = await enrichCaptureResult(response.result, supplier, tab.url || identification.activeTab?.url || '')
    if (!hasSessionEvidence(captureResult.session_bundle)) {
      const failed = { ok: false, error_code: 'SESSION_INCOMPLETE', error_message: 'no supported supplier token or cookie was found' }
      await client.fail(task, failed.error_code, failed.error_message)
      return { task, status: 'failed', result: failed }
    }
    const completed = await client.complete(task, captureResult)
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
    throw error
  }
}

async function runCaptureInTab(tabId, task, supplier) {
  for (let attempt = 0; attempt < 4; attempt++) {
    await waitForTabComplete(tabId)
    const response = await chrome.tabs.sendMessage(tabId, {
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
    const response = await chrome.tabs.sendMessage(tabId, { type: 'admin-plus:detect-login' })
    return response || { status: 'unknown' }
  } catch {
    return { status: 'unknown' }
  }
}

async function requireConnectedConfig() {
  const config = await loadConfig()
  if (!config.baseURL || !config.token) {
    const error = new Error('请先连接 sub2apiplus')
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
