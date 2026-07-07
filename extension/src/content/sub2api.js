(() => {
  if (window.__adminPlusSessionCaptureLoaded) return
  window.__adminPlusSessionCaptureLoaded = true
  const SUPPORTED_MESSAGES = new Set([
    'admin-plus:detect-login',
    'admin-plus:capture-session'
  ])
  const NEW_API_USER_HEADER_NAMES = [
    'New-Api-User',
    'New-API-User',
    'Veloera-User',
    'voapi-user',
    'User-id',
    'X-User-Id',
    'Rix-Api-User',
    'neo-api-user',
    'new-api-user'
  ]

  chrome.runtime.onMessage.addListener((message, _sender, sendResponse) => {
    if (!SUPPORTED_MESSAGES.has(message?.type)) return false
    Promise.resolve(handleMessage(message))
      .then((result) => sendResponse(result))
      .catch((error) => sendResponse({
        ok: false,
        error_code: error.reason || 'CONTENT_SCRIPT_ERROR',
        error_message: error.message || String(error)
      }))
    return true
  })

  async function handleMessage(message) {
    switch (message.type) {
      case 'admin-plus:detect-login':
        return detectLogin()
      case 'admin-plus:capture-session':
        return captureSession(message.task, message.supplier)
      default:
        return fail('UNSUPPORTED_CONTENT_MESSAGE', `unsupported message: ${message.type}`)
    }
  }

  async function captureSession(task, supplier) {
    const loginLike = isLoginLikePage()
    const bundle = await collectSessionBundle(supplier)
    if (!hasSessionEvidence(bundle) && loginLike && !supportsCookieBackedCapture(bundle, supplier)) {
      return fail('SUPPLIER_LOGIN_REQUIRED', '请先在当前供应商页面完成登录，再执行一键上报')
    }
    return ok({
      source: 'chrome',
      captured_at: new Date().toISOString(),
      session_bundle: bundle,
      session_summary: summarizeBundle(bundle),
      raw_payload: {
        url: location.href,
        title: document.title,
        task_id: task?.id
      }
    })
  }

  async function detectLogin() {
    const bundle = await collectSessionBundle({})
    if (hasSessionEvidence(bundle)) {
      return { status: 'logged_in', summary: summarizeBundle(bundle) }
    }
    if (isLoginLikePage()) {
      return { status: 'logged_out' }
    }
    return { status: 'unknown' }
  }

  function isLoginLikePage() {
    return Boolean(document.querySelector('input[type="password"]') || /login|signin|auth/i.test(location.pathname))
  }

  async function collectSessionBundle(supplier) {
    const storage = collectStorage()
    const tokens = extractTokens(storage)
    const providerType = normalizeProviderType(
      supplier?.type ||
      supplier?.supplier_type ||
      supplier?.provider_type ||
      inferProviderTypeFromStorage(storage)
    )
    const context = extractContext(storage, supplier, providerType)
    const bundle = {
      supplier_id: supplier?.id || supplier?.supplier_id,
      provider_type: providerType,
      system_type: providerType,
      origin: location.origin,
      url: location.href,
      captured_at: new Date().toISOString(),
      expires_at: inferExpiresAt(storage),
      access_token: tokens.access_token,
      refresh_token: tokens.refresh_token,
      csrf_token: tokens.csrf_token,
      tokens,
      cookies: [],
      context,
      required_headers: {
        origin: location.origin,
        referer: location.href,
        'user-agent': navigator.userAgent
      },
      storage_keys: Object.keys(storage).slice(0, 80)
    }
    if (window.AdminPlusNewAPI?.enrichSessionBundle) {
      await window.AdminPlusNewAPI.enrichSessionBundle({ bundle, storage, supplier })
    }
    return bundle
  }

  function collectStorage() {
    const values = {}
    collectStorageArea(window.localStorage, values, 'local')
    collectStorageArea(window.sessionStorage, values, 'session')
    return values
  }

  function collectStorageArea(area, values, prefix) {
    try {
      for (let index = 0; index < area.length; index++) {
        const key = area.key(index)
        if (!key) continue
        const value = area.getItem(key)
        values[`${prefix}:${key}`] = value
      }
    } catch {
      // ignore inaccessible storage
    }
  }

  function extractTokens(storage) {
    const access = firstFoundStorageValue([
      firstStorageValue(storage, ['auth_token', 'authToken', 'access_token', 'accessToken'], { exact: true }),
      firstStorageValue(storage, ['token', 'jwt', 'bearer'], { exact: true }),
      firstStorageValue(storage, ['auth_token', 'authToken', 'access_token', 'accessToken']),
      firstStorageValue(storage, ['token', 'jwt', 'bearer'], { exclude: ['token_expires_at', 'expires_at', 'today_token', 'total_token'] })
    ])
    return {
      access_token: access.value,
      access_token_source: access.source,
      refresh_token: firstStorageValue(storage, ['refresh_token', 'refreshToken'], { exact: true }).value,
      csrf_token: firstStorageValue(storage, ['csrf', 'xsrf', 'csrf_token', 'csrfToken']).value
    }
  }

  function firstFoundStorageValue(candidates) {
    return candidates.find((item) => item.value) || { value: '', source: '' }
  }

  function extractContext(storage, supplier, providerType) {
    const identity = userObjectFromStorage(storage)
    return {
      provider_type: providerType,
      system_type: providerType,
      user_id: firstNonEmpty(firstStorageValue(storage, ['user_id', 'userid', 'uid']).value, identity?.id),
      username: firstNonEmpty(identity?.username, identity?.name, identity?.email),
      email: firstNonEmpty(identity?.email),
      role: firstNonEmpty(identity?.role),
      organization_id: firstStorageValue(storage, ['organization_id', 'org_id', 'orgid']).value,
      project_id: firstStorageValue(storage, ['project_id', 'projectid']).value,
      account_id: firstStorageValue(storage, ['account_id', 'accountid']).value,
      api_base_url: supplier?.api_base_url || location.origin
    }
  }

  function firstStorageValue(storage, patterns, options = {}) {
    const entries = Object.entries(storage)
    for (const [key, value] of entries) {
      const storageKey = key.includes(':') ? key.slice(key.indexOf(':') + 1) : key
      const normalized = storageKey.toLowerCase()
      const matched = patterns.some((pattern) => {
        const expected = String(pattern).toLowerCase()
        return options.exact ? normalized === expected : normalized.includes(expected)
      })
      if (!matched) continue
      if ((options.exclude || []).some((pattern) => normalized.includes(String(pattern).toLowerCase()))) continue
      const parsed = extractValue(value)
      if (parsed) return { value: parsed, source: key }
    }
    return { value: '', source: '' }
  }

  function extractValue(value) {
    const raw = String(value || '').trim()
    if (!raw) return ''
    if (raw.startsWith('{') || raw.startsWith('[')) {
      try {
        const parsed = JSON.parse(raw)
        return findTokenLikeValue(parsed)
      } catch {
        return ''
      }
    }
    return raw.length > 5000 ? '' : raw
  }

  function findTokenLikeValue(value) {
    if (!value || typeof value !== 'object') return ''
    for (const key of ['access_token', 'accessToken', 'auth_token', 'authToken', 'token', 'jwt', 'refresh_token', 'refreshToken', 'csrf_token', 'csrfToken']) {
      if (typeof value[key] === 'string' && value[key]) return value[key]
    }
    for (const nested of Object.values(value)) {
      const found = findTokenLikeValue(nested)
      if (found) return found
    }
    return ''
  }

  function inferProviderTypeFromStorage(storage) {
    const keys = new Set(Object.keys(storage || {})
      .map((key) => key.includes(':') ? key.slice(key.indexOf(':') + 1) : key)
      .map((key) => key.toLowerCase()))
    if (keys.has('auth_token') || keys.has('auth_user')) return 'sub2api'
    if (keys.has('user') || keys.has('uid') || keys.has('quota_per_unit') || keys.has('quota_display_type')) return 'new_api'
    return ''
  }

  function userObjectFromStorage(storage) {
    const prioritized = []
    const fallback = []
    for (const [key, value] of Object.entries(storage || {})) {
      const storageKey = key.includes(':') ? key.slice(key.indexOf(':') + 1) : key
      const normalized = storageKey.toLowerCase()
      if (['auth_user', 'user', 'userstorage', 'auth-store', 'auth_storage', 'authstore'].includes(normalized)) {
        prioritized.push(value)
      } else if (looksLikeJSON(value)) {
        fallback.push(value)
      }
    }
    for (const value of [...prioritized, ...fallback]) {
      const user = findUserLikeObject(parseJSON(value))
      if (user?.id || user?.username || user?.email) return user
    }
    return {}
  }

  function findUserLikeObject(value, depth = 0) {
    if (!value || typeof value !== 'object' || depth > 5) return null
    const candidate = normalizeUserCandidate(value)
    if (candidate) return candidate
    for (const key of ['user', 'currentUser', 'profile', 'self', 'data', 'auth', 'state']) {
      const found = findUserLikeObject(value[key], depth + 1)
      if (found) return found
    }
    for (const nested of Object.values(value)) {
      const found = findUserLikeObject(nested, depth + 1)
      if (found) return found
    }
    return null
  }

  function normalizeUserCandidate(value) {
    if (!value || typeof value !== 'object') return null
    const id = firstNonEmpty(value.id, value.user_id, value.userId, value.uid)
    const username = firstNonEmpty(value.username, value.name, value.display_name, value.displayName, value.email)
    const email = firstNonEmpty(value.email)
    if (!id && !username && !email) return null
    return {
      id,
      username,
      name: firstNonEmpty(value.name, value.display_name, value.displayName),
      email,
      role: firstNonEmpty(value.role)
    }
  }

  function parseJSON(value) {
    try {
      const raw = String(value || '').trim()
      if (!raw) return null
      return JSON.parse(raw)
    } catch {
      return null
    }
  }

  function looksLikeJSON(value) {
    const raw = String(value || '').trim()
    return raw.startsWith('{') || raw.startsWith('[')
  }

  function inferExpiresAt(storage) {
    const raw = firstStorageValue(storage, ['token_expires_at', 'expires_at', 'expire']).value
    if (!raw) return ''
    const numeric = Number(raw)
    if (Number.isFinite(numeric)) {
      const milliseconds = numeric > 10_000_000_000 ? numeric : numeric * 1000
      return new Date(milliseconds).toISOString()
    }
    const parsed = new Date(raw)
    return Number.isNaN(parsed.getTime()) ? '' : parsed.toISOString()
  }

  function hasSessionEvidence(bundle) {
    if (window.AdminPlusNewAPI?.hasSessionEvidence?.(bundle)) {
      return true
    }
    return Boolean(bundle.tokens.access_token || bundle.tokens.refresh_token || bundle.tokens.csrf_token)
  }

  function isNewAPISession(bundle, supplier) {
    return providerTypeForSession(bundle, supplier) === 'new_api'
  }

  function supportsCookieBackedCapture(bundle, supplier) {
    return ['new_api', 'sub2api'].includes(providerTypeForSession(bundle, supplier))
  }

  function providerTypeForSession(bundle, supplier) {
    const providerType = normalizeProviderType(
      bundle?.provider_type ||
      bundle?.system_type ||
      bundle?.context?.provider_type ||
      bundle?.context?.system_type ||
      supplier?.type ||
      supplier?.supplier_type ||
      supplier?.provider_type ||
      ''
    )
    return providerType
  }

  function normalizeProviderType(value) {
    const normalized = String(value || '').trim().toLowerCase()
    if (normalized === 'newapi' || normalized === 'new-api') return 'new_api'
    if (['subapi', 'sub api', 'sub-api', 'sub_api', 'sub2api', 'sub2 api', 'sub2-api', 'sub2_api'].includes(normalized)) return 'sub2api'
    return normalized
  }

  function firstNonEmpty(...values) {
    for (const value of values) {
      const text = String(value || '').trim()
      if (text) return text
    }
    return ''
  }

  function summarizeBundle(bundle) {
    return {
      origin: bundle.origin,
      provider_type: bundle.provider_type || '',
      captured_at: bundle.captured_at,
      expires_at: bundle.expires_at,
      has_access_token: Boolean(bundle.tokens.access_token),
      has_refresh_token: Boolean(bundle.tokens.refresh_token),
      has_csrf_token: Boolean(bundle.tokens.csrf_token),
      cookie_count: bundle.cookies.length,
      api_base_url: bundle.context.api_base_url,
      user_id: bundle.context.user_id,
      organization_id: bundle.context.organization_id,
      project_id: bundle.context.project_id,
      account_id: bundle.context.account_id,
      has_new_api_user_header: Boolean(newAPIUserHeader(bundle))
    }
  }

  function newAPIUserHeader(bundle) {
    const headers = bundle?.required_headers || {}
    return firstNonEmpty(...NEW_API_USER_HEADER_NAMES.map((headerName) => headers[headerName]))
  }

  function ok(result) {
    return { ok: true, result }
  }

  function fail(errorCode, errorMessage) {
    return { ok: false, error_code: errorCode, error_message: errorMessage }
  }

})()
