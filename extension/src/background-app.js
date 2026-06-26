(() => {
const CONFIG_KEY = 'adminPlusOperatorConfig'
const LAST_CAPTURE_RESULT_KEY = 'adminPlusLastCaptureResult'
const DEFAULT_CONFIG_PATH = 'config/default-config.json'
let registrationTaskRun = null

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
      ingest: result.ingest || {},
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
        type: payload.supplier_type || payload.provider_type || '',
        supplier_type: payload.supplier_type || payload.provider_type || '',
        provider_type: payload.provider_type || payload.supplier_type || '',
        third_party_recharge_url: payload.third_party_recharge_url || '',
        local_recharge_url: payload.local_recharge_url || '',
        auto_create_supplier: payload.auto_create_supplier,
        page_context: payload.page_context || {},
        payload
      }
    })
  }

  async reportSupplierCandidate(payload) {
    return this.request('/api/v1/admin-plus/extension/suppliers/report-candidate', {
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

  async registrationCredential(task) {
    return this.request(`/api/v1/admin-plus/extension/tasks/${task.id}/registration-credential`, {
      method: 'POST',
      body: {
        device_id: task.device_id,
        lease_token: task.lease_token
      }
    })
  }

  async registrationVerificationCode(task, options = {}) {
    return this.request(`/api/v1/admin-plus/extension/tasks/${task.id}/registration-verification-code/read`, {
      method: 'POST',
      body: {
        device_id: task.device_id,
        lease_token: task.lease_token,
        triggered_at: options.triggeredAt || null,
        timeout_seconds: Number(options.timeoutSeconds || 90),
        poll_interval_seconds: Number(options.pollIntervalSeconds || 5)
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
    case 'site:collect-candidate':
      return collectSiteCandidate(message.includeSensitive === true)
    case 'session:capture':
      return captureSupplierSession(
        message.supplierID,
        message.autoCreate !== false,
        message.candidate || null,
        message.credentials || null
      )
    case 'supplier:report-candidate':
      return reportSupplierCandidate(message.payload || message.candidate || {})
    case 'registration:run-next':
      return runNextRegistrationTask()
    default:
      throw new Error('Unsupported extension message')
  }
}

async function reportSupplierCandidate(payload) {
  const config = await requireConnectedConfig()
  const activeTab = summarizeTab(await getActiveTab())
  const candidate = looksLikeSiteCandidate(payload) ? payload : null
  const reportPayload = candidate
    ? buildSupplierCandidatePayload(config.deviceID, { activeTab, candidate }, candidate, candidate.credential || {})
    : normalizeSupplierCandidateReportPayload(config.deviceID, activeTab, payload)
  const client = new AdminPlusClient(config)
  return client.reportSupplierCandidate(reportPayload)
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
  const probe = await collectSiteCandidate(false, tab?.id).catch(() => emptySiteCandidate())
  const suppliers = await client.listSuppliers()
  const matches = suppliers
    .map((supplier) => ({ supplier, score: matchSupplier(currentURL, supplier) }))
    .filter((item) => item.score > 0)
    .sort((a, b) => b.score - a.score)

  if (matches.length === 0) {
    return {
      status: probe.provider_type || probe.evidence?.length > 0 || probe.credential?.login_like ? 'needs_type_selection' : 'unsupported',
      activeTab: summarizeTab(tab),
      candidate: probe,
      message: '当前网站未匹配已配置供应商'
    }
  }
  const topScore = matches[0].score
  const topMatches = matches.filter((item) => item.score === topScore)
  if (topMatches.length > 1) {
    return {
      status: 'ambiguous',
      activeTab: summarizeTab(tab),
      candidate: probe,
      suppliers: topMatches.map((item) => supplierSummary(item.supplier, item.score)),
      message: '当前网站匹配多个供应商'
    }
  }
  const supplier = topMatches[0].supplier
  return {
    status: 'matched',
    activeTab: summarizeTab(tab),
    candidate: probe,
    supplier: supplierSummary(supplier, topScore),
    supplierLogin: await detectSupplierLogin(tab.id),
    message: supplier.credential?.browser_login_enabled ? '' : '该供应商未启用浏览器登录'
  }
}

async function captureSupplierSession(supplierID, autoCreate, candidate = null, credentials = null) {
  const config = await requireConnectedConfig()
  const identification = await identifyCurrentSite()
  const client = new AdminPlusClient(config)
  let supplier = resolveSupplierFromIdentification(identification, supplierID)
  if (!supplier && candidate?.supplier_id) {
    supplier = supplierFromCandidateHint(candidate)
  }
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
    supplier_type: supplier.type || providerTypeFromIdentification(identification),
    provider_type: supplier.type || providerTypeFromIdentification(identification),
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
    api_base_url: supplier.api_base_url || identification.activeTab?.origin || '',
    type: supplier.type || providerTypeFromIdentification(identification)
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
    const ingest = completed.result?.ingest || {}
    await recordCaptureResult(
      'succeeded',
      '上报成功',
      completed,
      supplier,
      identification.activeTab,
      completed.result?.session_summary || captureResult.session_summary || {},
      ingest
    )
    return {
      task: completed,
      status: 'succeeded',
      supplier,
      result: {
        session_summary: completed.result?.session_summary || captureResult.session_summary || {},
        ingest
      }
    }
  } catch (error) {
    await client.fail(task, error.reason || 'SESSION_CAPTURE_FAILED', error.message || String(error)).catch(() => {})
    await recordCaptureResult('failed', error.message || String(error), task, supplier, identification.activeTab).catch(() => {})
    throw error
  }
}

async function collectSiteCandidate(includeSensitive, tabId) {
  if (!tabId) {
    const active = await getActiveTab()
    tabId = active?.id || 0
  }
  if (!tabId) {
    return emptySiteCandidate()
  }
  const response = await sendContentMessage(tabId, { type: 'admin-plus:collect-candidate:v2', include_sensitive: includeSensitive === true })
  if (response?.ok === false) {
    throw new Error(response.error_message || 'site candidate probe failed')
  }
  const candidate = normalizeSiteCandidate(response)
  if (includeSensitive && !candidate.credential.password) {
    const frameCredential = await collectFrameCredential(tabId).catch(() => null)
    if (frameCredential?.password) {
      candidate.credential.password = frameCredential.password
      candidate.credential.password_present = true
    }
    if (frameCredential?.username && !candidate.credential.username) {
      candidate.credential.username = frameCredential.username
    }
  }
  return candidate
}

async function collectFrameCredential(tabId) {
  const injected = await chrome.scripting.executeScript({
    target: { tabId, allFrames: true },
    func: () => {
      const stringValue = (value) => String(value || '').trim()
      const descriptor = (input) => [
        input.type,
        input.name,
        input.id,
        input.autocomplete,
        input.placeholder,
        input.getAttribute('aria-label')
      ].join(' ').toLowerCase()
      const allInputs = () => {
        const found = []
        const seen = new Set()
        const visit = (root) => {
          if (!root?.querySelectorAll) return
          for (const input of root.querySelectorAll('input')) {
            if (seen.has(input)) continue
            seen.add(input)
            found.push(input)
          }
          for (const element of root.querySelectorAll('*')) {
            if (element.shadowRoot) visit(element.shadowRoot)
          }
        }
        visit(document)
        return found
      }
      const inputs = allInputs().filter((input) => !input.disabled && !input.readOnly)
      const passwordInput = inputs.find((input) => input.type === 'password' && stringValue(input.value)) ||
        inputs.find((input) => input.type === 'password')
      const usernameInput = inputs.find((input) => input.type === 'email' && stringValue(input.value)) ||
        inputs.find((input) => stringValue(input.value) && /(email|mail|邮箱|account|username|user_name|login|用户|账号)/i.test(descriptor(input))) ||
        inputs.find((input) => stringValue(input.value) && ['text', 'search', 'tel', ''].includes(input.type) && input !== passwordInput)
      return {
        username: stringValue(usernameInput?.value),
        password: stringValue(passwordInput?.value),
        password_present: Boolean(passwordInput),
        input_count: inputs.length
      }
    }
  })
  return (injected || [])
    .map((item) => item.result || {})
    .find((item) => item.password) ||
    (injected || []).map((item) => item.result || {}).find((item) => item.username || item.password_present) ||
    null
}

function buildSupplierCandidatePayload(deviceID, identification, candidate, credentials) {
  const activeTab = identification.activeTab || {}
  const page = candidate?.page || {}
  const defaults = candidate?.defaults || {}
  const credential = credentials || candidate?.credential || {}
  const providerType = normalizeProviderType(candidate?.provider_type || identification?.candidate?.provider_type || '')
  const name = trimReportName(firstNonEmpty(
    candidate?.name,
    defaults.name,
    activeTab.title,
    page.title,
    activeTab.host,
    page.host
  ))
  const dashboardURL = firstNonEmpty(defaults.dashboard_url, page.url, activeTab.url)
  const apiBaseURL = firstNonEmpty(defaults.api_base_url, page.origin, activeTab.origin)
  const thirdPartyRechargeURL = firstNonEmpty(defaults.third_party_recharge_url, inferThirdPartyRechargeURL(page.url), inferThirdPartyRechargeURL(activeTab.url))
  const localRechargeURL = firstNonEmpty(defaults.local_recharge_url, '')
  const pageContext = {
    title: page.title || activeTab.title || '',
    url: page.url || activeTab.url || '',
    identification_evidence: Array.isArray(candidate?.evidence) ? candidate.evidence : [],
    host: page.host || activeTab.host || ''
  }
  return {
    device_id: firstNonEmpty(deviceID, ''),
    auto_create_supplier: false,
    provider_type: providerType,
    system_type: providerType,
    type: providerType,
    supplier_type: providerType,
    name,
    contact: firstNonEmpty(credentials?.username, candidate?.credential?.username, defaults.contact),
    supplier_kind: firstNonEmpty(defaults.supplier_kind, 'relay'),
    runtime_status: firstNonEmpty(defaults.runtime_status, 'monitor_only'),
    health_status: firstNonEmpty(defaults.health_status, 'normal'),
    balance_cents: Number.isFinite(Number(defaults.balance_cents)) ? Number(defaults.balance_cents) : 0,
    balance_currency: firstNonEmpty(defaults.balance_currency, 'USD'),
    recharge_multiplier: Number.isFinite(Number(defaults.recharge_multiplier)) ? Number(defaults.recharge_multiplier) : 1,
    dashboard_url: dashboardURL,
    api_base_url: apiBaseURL,
    third_party_recharge_url: thirdPartyRechargeURL,
    local_recharge_url: localRechargeURL,
    source_host: page.host || activeTab.host || '',
    source_url: page.url || activeTab.url || '',
    origin: page.origin || activeTab.origin || '',
    browser_login_enabled: Boolean(firstNonEmpty(credentials?.username, candidate?.credential?.username) && firstNonEmpty(credentials?.password, candidate?.credential?.password)),
    browser_login_username: firstNonEmpty(credentials?.username, candidate?.credential?.username),
    browser_login_password: firstNonEmpty(credentials?.password, candidate?.credential?.password),
    browser_login_token: firstNonEmpty(credentials?.token, candidate?.credential?.token),
    notes: firstNonEmpty(candidate?.notes, 'reported from Chrome plugin'),
    page_context: pageContext
  }
}

function supplierFromCandidateHint(candidate) {
  const providerType = normalizeProviderType(candidate?.provider_type || candidate?.system_type || candidate?.type || candidate?.supplier_type || '')
  const supplierID = Number(candidate?.supplier_id || candidate?.id || 0)
  return {
    id: supplierID,
    supplier_id: supplierID,
    name: candidate?.supplier_name || candidate?.name || '当前供应商',
    type: providerType,
    dashboard_url: candidate?.dashboard_url || '',
    api_base_url: candidate?.api_base_url || '',
    third_party_recharge_url: candidate?.third_party_recharge_url || '',
    local_recharge_url: candidate?.local_recharge_url || '',
    credential: {
      browser_login_enabled: Boolean(candidate?.browser_login_enabled),
      browser_login_username_configured: Boolean(candidate?.browser_login_username),
      browser_login_password_configured: Boolean(candidate?.browser_login_password),
      browser_login_token_configured: Boolean(candidate?.browser_login_token),
      masked_browser_login_username: ''
    }
  }
}

function normalizeSiteCandidate(candidate) {
  const page = candidate?.page || {}
  const defaults = candidate?.defaults || {}
  const credential = candidate?.credential || {}
  const providerType = normalizeProviderType(candidate?.provider_type || '')
  const status = candidate?.status || (providerType ? 'identified' : credential.login_like || credential.username || credential.password_present ? 'needs_type_selection' : 'unsupported')
  return {
    status,
    provider_type: providerType,
    confidence: Number(candidate?.confidence || 0),
    evidence: Array.isArray(candidate?.evidence) ? candidate.evidence : [],
    page: {
      title: page.title || '',
      url: page.url || '',
      origin: page.origin || '',
      host: page.host || ''
    },
    credential: {
      username: String(credential.username || ''),
      password: String(credential.password || ''),
      password_present: Boolean(credential.password_present),
      token: String(credential.token || ''),
      login_like: Boolean(credential.login_like)
    },
    defaults: {
      name: String(defaults.name || ''),
      contact: String(defaults.contact || ''),
      supplier_kind: String(defaults.supplier_kind || 'relay'),
      runtime_status: String(defaults.runtime_status || 'monitor_only'),
      health_status: String(defaults.health_status || 'normal'),
      balance_cents: Number(defaults.balance_cents || 0),
      balance_currency: String(defaults.balance_currency || 'USD'),
      recharge_multiplier: Number(defaults.recharge_multiplier || 1),
      dashboard_url: String(defaults.dashboard_url || ''),
      api_base_url: String(defaults.api_base_url || ''),
      third_party_recharge_url: String(defaults.third_party_recharge_url || ''),
      local_recharge_url: String(defaults.local_recharge_url || '')
    }
  }
}

function emptySiteCandidate() {
  return normalizeSiteCandidate({
    status: 'unsupported',
    provider_type: '',
    confidence: 0,
    evidence: [],
    page: {},
    credential: {},
    defaults: {}
  })
}

async function runNextRegistrationTask() {
  if (registrationTaskRun) {
    return {
      status: 'running',
      message: '已有注册任务正在执行'
    }
  }
  registrationTaskRun = runNextRegistrationTaskOnce().finally(() => {
    registrationTaskRun = null
  })
  return registrationTaskRun
}

async function runNextRegistrationTaskOnce() {
  const config = await requireConnectedConfig()
  const client = new AdminPlusClient(config)
  const task = await client.claimTask(config.deviceID, ['register_supplier_account'])
  await client.heartbeat(task)
  const credential = await client.registrationCredential(task)
  if (!credential?.register_url || !credential?.email || !credential?.password) {
    await client.fail(task, 'REGISTRATION_CREDENTIAL_INCOMPLETE', 'registration credential is incomplete').catch(() => {})
    return { status: 'failed', task, message: '注册凭据不完整' }
  }
  const tab = await chrome.tabs.create({ url: credential.register_url, active: true })
  try {
    await waitForTabComplete(tab.id)
    const [injected] = await chrome.scripting.executeScript({
      target: { tabId: tab.id },
      func: fillRegistrationForm,
      args: [{
        email: credential.email,
        password: credential.password,
        providerType: credential.provider_type,
        registerURL: credential.register_url
      }]
    })
    const result = injected?.result || {}
    if (result.manual_verification_required) {
      if (!result.email_verification_required) {
        await client.fail(task, 'REGISTRATION_VERIFICATION_REQUIRED', result.message || 'registration requires manual verification')
        return {
          status: 'waiting_manual_verification',
          task,
          result,
          message: result.message || '需要人工完成验证码或邮箱验证'
        }
      }
      await client.heartbeat(task, 180)
      let verification
      try {
        verification = await client.registrationVerificationCode(task, {
          triggeredAt: result.submitted_at || result.started_at || new Date().toISOString(),
          timeoutSeconds: 90,
          pollIntervalSeconds: 5
        })
      } catch (error) {
        if (error?.reason === 'MAIL_VERIFICATION_CODE_NOT_FOUND' || error?.reason === 'MAIL_CREDENTIAL_NOT_FOUND') {
          await client.fail(task, 'REGISTRATION_VERIFICATION_CODE_NOT_FOUND', error.message || 'registration verification code not found')
          return {
            status: 'failed',
            task,
            result,
            message: error.message || '未读取到邮箱验证码'
          }
        }
        throw error
      }
      if (!verification?.code) {
        await client.fail(task, 'REGISTRATION_VERIFICATION_CODE_NOT_FOUND', 'registration verification code not found')
        return {
          status: 'failed',
          task,
          result,
          message: '未读取到邮箱验证码'
        }
      }
      await client.heartbeat(task, 180)
      const [verificationInjected] = await chrome.scripting.executeScript({
        target: { tabId: tab.id },
        func: fillRegistrationVerificationCode,
        args: [verification.code]
      })
      const verificationResult = verificationInjected?.result || {}
      if (!verificationResult.ok) {
        await client.fail(task, verificationResult.error_code || 'REGISTRATION_VERIFICATION_FILL_FAILED', verificationResult.message || 'registration verification fill failed')
        return {
          status: 'failed',
          task,
          result: verificationResult,
          message: verificationResult.message || '验证码回填失败'
        }
      }
      const completed = await client.complete(task, {
        registration_submitted: true,
        verification_completed: true,
        verification_message_id: verification.message_id || '',
        verification_received_at: verification.received_at || '',
        provider_type: credential.provider_type,
        register_url: credential.register_url,
        submitted_at: result.submitted_at || new Date().toISOString(),
        result: {
          ...result,
          verification_filled: true,
          verification_result: verificationResult
        }
      })
      return {
        status: 'succeeded',
        task: completed,
        result: verificationResult,
        message: '注册验证码已回填并提交'
      }
    }
    if (!result.ok) {
      await client.fail(task, result.error_code || 'REGISTRATION_AUTOFILL_FAILED', result.message || 'registration autofill failed')
      return {
        status: 'failed',
        task,
        result,
        message: result.message || '注册自动填写失败'
      }
    }
    const completed = await client.complete(task, {
      registration_submitted: true,
      provider_type: credential.provider_type,
      register_url: credential.register_url,
      submitted_at: new Date().toISOString(),
      result
    })
    return {
      status: 'succeeded',
      task: completed,
      result,
      message: '注册表单已提交'
    }
  } catch (error) {
    await client.fail(task, error.reason || 'REGISTRATION_AUTOFILL_FAILED', error.message || String(error)).catch(() => {})
    throw error
  }
}

function fillRegistrationForm(credential) {
  const startedAt = new Date().toISOString()
  const text = () => String(document.body?.innerText || '').toLowerCase()
  const manualVerificationMarkers = [
    'captcha',
    'turnstile',
    'recaptcha',
    'hcaptcha',
    '人机',
    '滑块验证'
  ]
  const emailVerificationMarkers = [
    '验证码',
    '邮箱验证',
    '邮件验证',
    '邮箱验证码',
    '邮件验证码',
    'verification code',
    'email code',
    'email verification',
    'verify your email'
  ]
  const hasManualVerification = () => manualVerificationMarkers.some((marker) => text().includes(marker))
  const hasEmailVerificationText = () => emailVerificationMarkers.some((marker) => text().includes(marker))
  const visible = (element) => {
    if (!element || element.disabled || element.readOnly) return false
    const style = window.getComputedStyle(element)
    if (style.display === 'none' || style.visibility === 'hidden') return false
    const rect = element.getBoundingClientRect()
    return rect.width > 0 && rect.height > 0
  }
  const inputs = Array.from(document.querySelectorAll('input')).filter(visible)
  const lowerAttr = (element) => [
    element.type,
    element.name,
    element.id,
    element.autocomplete,
    element.placeholder,
    element.getAttribute('aria-label')
  ].join(' ').toLowerCase()
  const setValue = (element, value) => {
    const proto = Object.getPrototypeOf(element)
    const descriptor = Object.getOwnPropertyDescriptor(proto, 'value')
    if (descriptor?.set) {
      descriptor.set.call(element, value)
    } else {
      element.value = value
    }
    element.dispatchEvent(new Event('input', { bubbles: true }))
    element.dispatchEvent(new Event('change', { bubbles: true }))
  }
  const isEmailVerificationInput = (input) => {
    const attr = lowerAttr(input)
    if (/(captcha|turnstile|recaptcha|hcaptcha|人机|滑块)/i.test(attr)) return false
    if (/(code|otp|verification|verify|验证码|校验码|动态码|邮箱验证|邮件验证)/i.test(attr)) return true
    return false
  }
  const findEmailVerificationInput = () => Array.from(document.querySelectorAll('input')).filter(visible).find(isEmailVerificationInput)
  const emailInput = inputs.find((input) => input.type === 'email') ||
    inputs.find((input) => /(email|mail|邮箱|account|username|user_name|login)/i.test(lowerAttr(input))) ||
    inputs.find((input) => ['text', 'search', ''].includes(input.type))
  const passwordInputs = inputs.filter((input) => input.type === 'password')
  if (!emailInput || passwordInputs.length === 0) {
    return {
      ok: false,
      error_code: 'REGISTRATION_FORM_NOT_FOUND',
      message: '未找到可填写的注册表单',
      started_at: startedAt
    }
  }
  setValue(emailInput, credential.email)
  passwordInputs.forEach((input) => setValue(input, credential.password))
  for (const checkbox of inputs.filter((input) => input.type === 'checkbox')) {
    if (!checkbox.checked) checkbox.click()
  }
  if (hasManualVerification()) {
    return {
      ok: false,
      manual_verification_required: true,
      message: '页面存在验证码或验证流程，请人工完成',
      started_at: startedAt
    }
  }
  const buttons = Array.from(document.querySelectorAll('button, input[type="submit"], input[type="button"], [role="button"]')).filter(visible)
  const submit = buttons.find((button) => /(注册|sign up|signup|register|创建|create|submit|提交)/i.test(`${button.innerText || ''} ${button.value || ''} ${button.getAttribute('aria-label') || ''}`)) ||
    buttons.find((button) => button.type === 'submit') ||
    document.querySelector('form button[type="submit"], form input[type="submit"]')
  if (!submit) {
    return {
      ok: false,
      error_code: 'REGISTRATION_SUBMIT_NOT_FOUND',
      message: '未找到注册提交按钮',
      started_at: startedAt
    }
  }
  submit.click()
  return new Promise((resolve) => {
    setTimeout(() => {
      const needsManual = hasManualVerification()
      const emailVerificationInput = findEmailVerificationInput()
      const needsEmailVerification = Boolean(emailVerificationInput || hasEmailVerificationText())
      resolve({
        ok: !needsManual && !needsEmailVerification,
        manual_verification_required: needsManual || needsEmailVerification,
        email_verification_required: needsEmailVerification && !needsManual,
        message: needsManual ? '提交后需要人工验证' : needsEmailVerification ? '提交后需要邮箱验证码' : '注册表单已提交',
        started_at: startedAt,
        submitted_at: new Date().toISOString(),
        url: window.location.href
      })
    }, 2500)
  })
}

function fillRegistrationVerificationCode(code) {
  const submittedAt = new Date().toISOString()
  const value = String(code || '').trim()
  const visible = (element) => {
    if (!element || element.disabled || element.readOnly) return false
    const style = window.getComputedStyle(element)
    if (style.display === 'none' || style.visibility === 'hidden') return false
    const rect = element.getBoundingClientRect()
    return rect.width > 0 && rect.height > 0
  }
  const lowerAttr = (element) => [
    element.type,
    element.name,
    element.id,
    element.autocomplete,
    element.placeholder,
    element.getAttribute('aria-label')
  ].join(' ').toLowerCase()
  const setValue = (element, nextValue) => {
    const proto = Object.getPrototypeOf(element)
    const descriptor = Object.getOwnPropertyDescriptor(proto, 'value')
    if (descriptor?.set) {
      descriptor.set.call(element, nextValue)
    } else {
      element.value = nextValue
    }
    element.dispatchEvent(new Event('input', { bubbles: true }))
    element.dispatchEvent(new Event('change', { bubbles: true }))
  }
  if (!value) {
    return {
      ok: false,
      error_code: 'REGISTRATION_VERIFICATION_CODE_EMPTY',
      message: '验证码为空',
      submitted_at: submittedAt
    }
  }
  const inputs = Array.from(document.querySelectorAll('input')).filter(visible)
  const codeInput = inputs.find((input) => {
    const attr = lowerAttr(input)
    if (/(captcha|turnstile|recaptcha|hcaptcha|人机|滑块)/i.test(attr)) return false
    return /(code|otp|verification|verify|验证码|校验码|动态码|邮箱验证|邮件验证)/i.test(attr)
  }) || inputs.find((input) => ['text', 'number', 'tel', ''].includes(input.type) && input.maxLength >= value.length && input.maxLength <= 12)
  if (!codeInput) {
    return {
      ok: false,
      error_code: 'REGISTRATION_VERIFICATION_INPUT_NOT_FOUND',
      message: '未找到邮箱验证码输入框',
      submitted_at: submittedAt
    }
  }
  setValue(codeInput, value)
  const buttons = Array.from(document.querySelectorAll('button, input[type="submit"], input[type="button"], [role="button"]')).filter(visible)
  const submit = buttons.find((button) => /(验证|确认|提交|完成|continue|verify|confirm|submit|next|sign up|register)/i.test(`${button.innerText || ''} ${button.value || ''} ${button.getAttribute('aria-label') || ''}`)) ||
    buttons.find((button) => button.type === 'submit') ||
    document.querySelector('form button[type="submit"], form input[type="submit"]')
  if (submit) submit.click()
  return {
    ok: true,
    verification_filled: true,
    submitted_at: submittedAt,
    url: window.location.href
  }
}

async function recordCaptureResult(status, message, task, supplier, activeTab, summary = {}, ingest = {}) {
  await saveLastCaptureResult({
    status,
    message,
    supplier: supplier?.name || '',
    host: activeTab?.host || '',
    taskID: task?.id || 0,
    summary,
    ingest,
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
  const tokens = bundle.tokens || {}
  const captureURL = bundle.url || fallbackURL || supplier.dashboard_url || ''
  const cookies = await collectCookies(captureURL)
  const mergedBundle = {
    ...bundle,
    supplier_id: bundle.supplier_id || supplier.id || supplier.supplier_id,
    access_token: bundle.access_token || tokens.access_token || '',
    refresh_token: bundle.refresh_token || tokens.refresh_token || '',
    csrf_token: bundle.csrf_token || tokens.csrf_token || '',
    cookies,
    required_headers: {
      ...(bundle.required_headers || {}),
      cookie: cookies.map((cookie) => `${cookie.name}=${cookie.value}`).join('; ')
    }
  }
  normalizeNewAPISessionBundle(mergedBundle, supplier, captureURL)
  return {
    ...result,
    session_bundle: mergedBundle,
    session_summary: summarizeSessionBundle(mergedBundle)
  }
}

function supplierFromCurrentSite(identification, autoCreate) {
  const error = new Error(autoCreate ? '请先完成注册并入库供应商后再上报当前会话' : '当前网站未匹配可上报的供应商')
  error.reason = 'SUPPLIER_SITE_NOT_MATCHED'
  throw error
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
  if (providerTypeFromBundle(bundle) === 'new_api') {
    return Boolean((bundle?.cookies || []).length > 0 && newAPIUserHeader(bundle))
  }
  return Boolean(tokens.access_token || tokens.refresh_token || tokens.csrf_token || (bundle?.cookies || []).length > 0)
}

function summarizeSessionBundle(bundle) {
  const tokens = bundle?.tokens || {}
  const context = bundle?.context || {}
  const headers = bundle?.required_headers || {}
  return {
    origin: bundle?.origin || '',
    provider_type: providerTypeFromBundle(bundle),
    captured_at: bundle?.captured_at || '',
    expires_at: bundle?.expires_at || '',
    has_access_token: Boolean(tokens.access_token),
    has_refresh_token: Boolean(tokens.refresh_token),
    has_csrf_token: Boolean(tokens.csrf_token),
    cookie_count: (bundle?.cookies || []).length,
    api_base_url: context.api_base_url || '',
    user_id: context.user_id || '',
    organization_id: context.organization_id || '',
    project_id: context.project_id || '',
    account_id: context.account_id || '',
    has_required_origin: Boolean(headers.origin),
    has_required_referer: Boolean(headers.referer),
    has_required_cookie: Boolean(headers.cookie),
    has_new_api_user_header: Boolean(newAPIUserHeader(bundle))
  }
}

function normalizeNewAPISessionBundle(bundle, supplier, captureURL) {
  const providerType = normalizeProviderType(
    providerTypeFromBundle(bundle) ||
    supplier?.type ||
    supplier?.supplier_type ||
    supplier?.provider_type
  )
  if (providerType !== 'new_api') return bundle

  const context = bundle.context || {}
  const headers = bundle.required_headers || {}
  const userID = firstNonEmpty(
    newAPIUserHeader(bundle),
    bundle.auth_header_value,
    context.user_id,
    context.id
  )

  bundle.provider_type = 'new_api'
  bundle.system_type = 'new_api'
  bundle.auth_header_name = 'New-Api-User'
  bundle.auth_header_value = userID
  context.provider_type = 'new_api'
  context.system_type = 'new_api'
  context.user_id = userID
  context.api_base_url = firstNonEmpty(context.api_base_url, bundle.api_base_url, supplier?.api_base_url, originFromURL(captureURL), bundle.origin)
  headers.origin = firstNonEmpty(headers.origin, bundle.origin, originFromURL(captureURL))
  headers.referer = firstNonEmpty(headers.referer, captureURL)
  if (userID) {
    headers['New-Api-User'] = userID
  }
  bundle.context = context
  bundle.required_headers = headers
  return bundle
}

function providerTypeFromIdentification(identification) {
  return normalizeProviderType(
    identification?.supplier?.type ||
    identification?.candidate?.provider_type ||
    identification?.supplierLogin?.summary?.provider_type ||
    ''
  )
}

function providerTypeFromBundle(bundle) {
  const context = bundle?.context || {}
  return normalizeProviderType(
    bundle?.provider_type ||
    bundle?.system_type ||
    context.provider_type ||
    context.system_type ||
    ''
  )
}

function normalizeProviderType(value) {
  const normalized = String(value || '').trim().toLowerCase()
  if (normalized === 'newapi' || normalized === 'new-api') return 'new_api'
  if (normalized === 'sub-api' || normalized === 'sub2-api') return 'sub2api'
  return normalized
}

function newAPIUserHeader(bundle) {
  const headers = bundle?.required_headers || {}
  return firstNonEmpty(headers['New-Api-User'], headers['New-API-User'], headers['new-api-user'])
}

function originFromURL(value) {
  const parsed = safeURL(value || '')
  return parsed ? parsed.origin : ''
}

function firstNonEmpty(...values) {
  for (const value of values) {
    const text = String(value || '').trim()
    if (text) return text
  }
  return ''
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
      files: ['src/content/site-probe.js', 'src/content/newapi.js', 'src/content/sub2api.js']
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
    origin: parsed?.origin || '',
    host: parsed?.host || ''
  }
}

function looksLikeSiteCandidate(value) {
  if (!value || typeof value !== 'object') return false
  return Boolean(value.page || value.defaults || value.credential)
}

function normalizeSupplierCandidateReportPayload(deviceID, activeTab, payload = {}) {
  return {
    ...payload,
    device_id: firstNonEmpty(payload.device_id, deviceID),
    auto_create_supplier: payload.auto_create_supplier !== false,
    source_url: firstNonEmpty(payload.source_url, payload.url, activeTab.url),
    source_host: firstNonEmpty(payload.source_host, payload.host, activeTab.host),
    origin: firstNonEmpty(payload.origin, activeTab.origin),
    dashboard_url: firstNonEmpty(payload.dashboard_url, activeTab.url),
    api_base_url: firstNonEmpty(payload.api_base_url, activeTab.origin),
    provider_type: normalizeProviderType(firstNonEmpty(payload.provider_type, payload.system_type, payload.supplier_type, payload.type)),
    system_type: normalizeProviderType(firstNonEmpty(payload.system_type, payload.provider_type, payload.supplier_type, payload.type)),
    supplier_type: normalizeProviderType(firstNonEmpty(payload.supplier_type, payload.provider_type, payload.system_type, payload.type)),
    type: normalizeProviderType(firstNonEmpty(payload.type, payload.provider_type, payload.system_type, payload.supplier_type))
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
  if (identification?.supplier && (!supplierID || identification.supplier.id === supplierID)) {
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
