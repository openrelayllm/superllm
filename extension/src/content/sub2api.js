(() => {
  if (window.__adminPlusSessionCaptureLoaded) return
  window.__adminPlusSessionCaptureLoaded = true

  chrome.runtime.onMessage.addListener((message, _sender, sendResponse) => {
    if (!message?.type?.startsWith('admin-plus:')) return false
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
    if (isLoginLikePage()) {
      return fail('SUPPLIER_LOGIN_REQUIRED', '请先在当前供应商页面完成登录，再执行一键上报')
    }
    const bundle = await collectSessionBundle(supplier)
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
    if (isLoginLikePage()) {
      return { status: 'logged_out' }
    }
    const bundle = await collectSessionBundle({})
    if (hasSessionEvidence(bundle)) {
      return { status: 'logged_in', summary: summarizeBundle(bundle) }
    }
    return { status: 'unknown' }
  }

  function isLoginLikePage() {
    return Boolean(document.querySelector('input[type="password"]') || /login|signin|auth/i.test(location.pathname))
  }

  async function collectSessionBundle(supplier) {
    const storage = collectStorage()
    const tokens = extractTokens(storage)
    const context = extractContext(storage, supplier)
    const bundle = {
      supplier_id: supplier?.id || supplier?.supplier_id,
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
        referer: location.href
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
      firstStorageValue(storage, ['auth_token', 'access_token', 'accessToken'], { exact: true }),
      firstStorageValue(storage, ['token', 'jwt', 'bearer'], { exact: true }),
      firstStorageValue(storage, ['auth_token', 'access_token', 'accessToken']),
      firstStorageValue(storage, ['token', 'jwt', 'bearer'], { exclude: ['token_expires_at', 'expires_at', 'today_token', 'total_token'] })
    ])
    return {
      access_token: access.value,
      access_token_source: access.source,
      refresh_token: firstStorageValue(storage, ['refresh_token', 'refreshToken'], { exact: true }).value,
      csrf_token: firstStorageValue(storage, ['csrf', 'xsrf']).value
    }
  }

  function firstFoundStorageValue(candidates) {
    return candidates.find((item) => item.value) || { value: '', source: '' }
  }

  function extractContext(storage, supplier) {
    return {
      user_id: firstStorageValue(storage, ['user_id', 'userid', 'uid']).value,
      organization_id: firstStorageValue(storage, ['organization_id', 'org_id', 'orgid']).value,
      project_id: firstStorageValue(storage, ['project_id', 'projectid']).value,
      account_id: firstStorageValue(storage, ['account_id', 'accountid']).value,
      api_base_url: supplier?.api_base_url || `${location.origin}/api`
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
    for (const key of ['access_token', 'auth_token', 'token', 'jwt', 'refresh_token', 'csrf_token']) {
      if (typeof value[key] === 'string' && value[key]) return value[key]
    }
    for (const nested of Object.values(value)) {
      const found = findTokenLikeValue(nested)
      if (found) return found
    }
    return ''
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
      has_new_api_user_header: Boolean(bundle.required_headers?.['New-Api-User'])
    }
  }

  function ok(result) {
    return { ok: true, result }
  }

  function fail(errorCode, errorMessage) {
    return { ok: false, error_code: errorCode, error_message: errorMessage }
  }

})()
