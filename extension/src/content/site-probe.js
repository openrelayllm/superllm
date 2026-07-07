(() => {
  const SITE_PROBE_VERSION = 'credential-v2'
  if (window.__adminPlusSiteProbeLoaded === SITE_PROBE_VERSION) return
  window.__adminPlusSiteProbeLoaded = SITE_PROBE_VERSION

  const SUPPORTED_MESSAGES = new Set([
    'admin-plus:probe-site',
    'admin-plus:collect-candidate',
    'admin-plus:collect-candidate:v2'
  ])

  chrome.runtime.onMessage.addListener((message, _sender, sendResponse) => {
    if (!SUPPORTED_MESSAGES.has(message?.type)) return false
    Promise.resolve(collectSiteCandidate({ includeSensitive: message.include_sensitive === true }))
      .then((result) => sendResponse(result))
      .catch((error) => sendResponse({
        ok: false,
        error_code: error.reason || 'SITE_PROBE_FAILED',
        error_message: error.message || String(error)
      }))
    return true
  })

  async function collectSiteCandidate(options = {}) {
    const storage = collectStorage()
    const page = pageSummary()
    const identification = await identifyProvider(storage)
    const login = collectLoginForm(options.includeSensitive)
    const debug = summarizeInputs()
    const token = options.includeSensitive ? extractToken(storage) : ''
    const apiBaseURL = page.origin
    const status = identification.provider_type
      ? 'identified'
      : (identification.evidence.length > 0 || login.username || login.password_present || login.login_like)
        ? 'needs_type_selection'
        : 'unsupported'

    return {
      status,
      page,
      provider_type: identification.provider_type,
      confidence: identification.confidence,
      evidence: identification.evidence,
      credential: {
        username: login.username,
        password: login.password,
        password_present: login.password_present,
        login_like: login.login_like,
        debug,
        token
      },
      defaults: {
        name: suggestedName(page),
        contact: login.username,
        supplier_kind: 'relay',
        runtime_status: 'monitor_only',
        health_status: 'normal',
        balance_cents: 0,
        balance_currency: inferCurrency(document.body?.innerText || '') || 'USD',
        recharge_multiplier: 1,
        dashboard_url: page.url,
        api_base_url: apiBaseURL,
        third_party_recharge_url: inferRechargeURL(page.url),
        local_recharge_url: ''
      }
    }
  }

  async function identifyProvider(storage) {
    const evidence = []
    let sub2apiScore = 0
    let newAPIScore = 0
    const title = document.title || ''
    const path = location.pathname || ''
    const text = String(document.body?.innerText || '').slice(0, 8000)
    const storageKeys = Object.keys(storage)
      .map((key) => key.includes(':') ? key.slice(key.indexOf(':') + 1) : key)
      .map((key) => key.toLowerCase())
    const storageKeySet = new Set(storageKeys)

    if (/sub(?:2)?api/i.test(`${title} ${text}`)) {
      sub2apiScore += 3
      evidence.push('dom:sub2api-brand')
    }
    if (/new[\s_-]?api/i.test(`${title} ${text}`)) {
      newAPIScore += 3
      evidence.push('dom:new-api-brand')
    }
    if (/\/api\/v1\/|\/admin|\/dashboard|\/login/i.test(path)) {
      sub2apiScore += 1
      evidence.push('route:sub2api-like')
    }
    if (storageKeySet.has('user') || storageKeySet.has('uid')) {
      newAPIScore += 1
      evidence.push('storage:user')
    }
    if (storageKeySet.has('quota_per_unit') || storageKeySet.has('quota_display_type') || storageKeySet.has('display_in_currency')) {
      newAPIScore += 2
      evidence.push('storage:new-api-quota')
    }
    if (storageKeySet.has('auth_token') || storageKeySet.has('access_token') || storageKeySet.has('refresh_token')) {
      sub2apiScore += 1
      evidence.push('storage:token')
    }
    if (storageKeySet.has('auth_user') && storageKeySet.has('auth_token')) {
      sub2apiScore += 3
      evidence.push('storage:sub2api-auth')
    }

    const apiEvidence = await probeKnownAPIs()
    sub2apiScore += apiEvidence.sub2apiScore
    newAPIScore += apiEvidence.newAPIScore
    evidence.push(...apiEvidence.evidence)

    if (newAPIScore >= 3 && newAPIScore >= sub2apiScore + 2) {
      return {
        provider_type: 'new_api',
        confidence: confidenceFromScore(newAPIScore),
        evidence
      }
    }
    if (sub2apiScore >= 3 && sub2apiScore >= newAPIScore + 2) {
      return {
        provider_type: 'sub2api',
        confidence: confidenceFromScore(sub2apiScore),
        evidence
      }
    }
    return {
      provider_type: '',
      confidence: 0,
      evidence
    }
  }

  async function probeKnownAPIs() {
    const [sub2api, newAPI] = await Promise.all([
      probeJSON('/api/v1/settings/public'),
      probeJSON('/api/status')
    ])
    const result = { sub2apiScore: 0, newAPIScore: 0, evidence: [] }
    if (sub2api.ok && looksLikeSub2APISettings(sub2api.json)) {
      result.sub2apiScore += 5
      result.evidence.push('api:/api/v1/settings/public')
    }
    if (newAPI.ok && looksLikeNewAPIStatus(newAPI.json)) {
      result.newAPIScore += 5
      result.evidence.push('api:/api/status')
    }
    return result
  }

  async function probeJSON(path) {
    const controller = new AbortController()
    const timer = setTimeout(() => controller.abort(), 1500)
    try {
      const response = await fetch(path, {
        credentials: 'include',
        cache: 'no-store',
        headers: { Accept: 'application/json' },
        signal: controller.signal
      })
      if (!response.ok) return { ok: false }
      const contentType = response.headers.get('content-type') || ''
      if (!contentType.includes('json')) return { ok: false }
      const json = await response.clone().json().catch(() => null)
      if (!json || typeof json !== 'object') return { ok: false }
      return { ok: true, json }
    } catch {
      return { ok: false }
    } finally {
      clearTimeout(timer)
    }
  }

  function looksLikeSub2APISettings(payload) {
    const data = payload?.data && typeof payload.data === 'object' ? payload.data : payload
    return payload?.code === 0 && Boolean(
      data?.site_name ||
      data?.site_logo ||
      data?.api_base_url ||
      data?.version ||
      Object.prototype.hasOwnProperty.call(data || {}, 'registration_enabled')
    )
  }

  function looksLikeNewAPIStatus(payload) {
    const data = payload?.data && typeof payload.data === 'object' ? payload.data : payload
    return payload?.success === true && Boolean(
      data?.system_name ||
      data?.quota_per_unit ||
      data?.quota_display_type ||
      Object.prototype.hasOwnProperty.call(data || {}, 'display_in_currency')
    )
  }

  function collectLoginForm(includeSensitive) {
    const inputs = visibleInputs()
    const passwordInput = findPasswordInput(inputs, includeSensitive)
    const usernameInput = inputs.find((input) => input.type === 'email') ||
      inputs.find((input) => /(email|mail|邮箱|account|username|user_name|login|用户|账号)/i.test(inputDescriptor(input))) ||
      inputs.find((input) => ['text', 'search', 'tel', ''].includes(input.type) && input !== passwordInput)
    return {
      username: stringValue(usernameInput?.value),
      password: includeSensitive ? stringValue(passwordInput?.value) : '',
      password_present: Boolean(passwordInput),
      login_like: Boolean(passwordInput || usernameInput)
    }
  }

  function findPasswordInput(visible, includeSensitive) {
    const visiblePassword = visible.find((input) => input.type === 'password')
    if (visiblePassword) return visiblePassword
    if (!includeSensitive) return null
    return allInputs()
      .find((input) => !input.disabled && !input.readOnly && stringValue(input.value)) || null
  }

  function visibleInputs() {
    return allInputs().filter((input) => {
      if (!input || input.disabled || input.readOnly) return false
      const style = window.getComputedStyle(input)
      if (style.display === 'none' || style.visibility === 'hidden') return false
      const rect = input.getBoundingClientRect()
      return rect.width > 0 && rect.height > 0
    })
  }

  function allInputs() {
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

  function summarizeInputs() {
    const inputs = allInputs()
    const passwordInputs = inputs.filter((input) => input.type === 'password')
    return {
      input_count: inputs.length,
      password_input_count: passwordInputs.length,
      password_value_present: passwordInputs.some((input) => stringValue(input.value))
    }
  }

  function inputDescriptor(input) {
    return [
      input.type,
      input.name,
      input.id,
      input.autocomplete,
      input.placeholder,
      input.getAttribute('aria-label')
    ].join(' ').toLowerCase()
  }

  function collectStorage() {
    const values = {}
    collectStorageArea(window.localStorage, values, 'local')
    collectStorageArea(window.sessionStorage, values, 'session')
    return values
  }

  function collectStorageArea(area, values, prefix) {
    try {
      for (let index = 0; index < area.length; index += 1) {
        const key = area.key(index)
        if (!key) continue
        values[`${prefix}:${key}`] = area.getItem(key)
      }
    } catch {
      // Storage can be blocked by browser or page policy.
    }
  }

  function extractToken(storage) {
    return firstStorageValue(storage, ['auth_token', 'access_token', 'accessToken'], { exact: true }) ||
      firstStorageValue(storage, ['token', 'jwt', 'bearer'], { exact: true }) ||
      firstStorageValue(storage, ['auth_token', 'access_token', 'accessToken']) ||
      firstStorageValue(storage, ['token', 'jwt', 'bearer'], { exclude: ['token_expires_at', 'expires_at', 'today_token', 'total_token'] })
  }

  function firstStorageValue(storage, patterns, options = {}) {
    for (const [key, value] of Object.entries(storage || {})) {
      const storageKey = key.includes(':') ? key.slice(key.indexOf(':') + 1) : key
      const normalized = storageKey.toLowerCase()
      const matched = patterns.some((pattern) => {
        const expected = String(pattern).toLowerCase()
        return options.exact ? normalized === expected : normalized.includes(expected)
      })
      if (!matched) continue
      if ((options.exclude || []).some((pattern) => normalized.includes(String(pattern).toLowerCase()))) continue
      const parsed = extractValue(value)
      if (parsed) return parsed
    }
    return ''
  }

  function extractValue(value) {
    const raw = String(value || '').trim()
    if (!raw || raw.length > 5000) return ''
    if (!raw.startsWith('{') && !raw.startsWith('[')) return raw
    try {
      return findTokenLikeValue(JSON.parse(raw))
    } catch {
      return ''
    }
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

  function pageSummary() {
    return {
      title: document.title || '',
      url: location.href,
      origin: location.origin,
      host: location.host,
      path: location.pathname
    }
  }

  function suggestedName(page) {
    const title = String(page.title || '').trim()
    if (title) return title.slice(0, 80)
    return page.host || '当前供应商'
  }

  function inferCurrency(text) {
    if (/[¥￥]|CNY|RMB/i.test(text)) return 'CNY'
    if (/\$|USD/i.test(text)) return 'USD'
    return ''
  }

  function inferRechargeURL(value) {
    const parsed = safeURL(value)
    if (!parsed || !/^https?:$/.test(parsed.protocol)) return ''
    const path = parsed.pathname.toLowerCase()
    const markers = ['/custom/', '/recharge', '/payment', '/topup', '/redeem', '/card', '/pay']
    return markers.some((marker) => path.includes(marker)) ? parsed.href : ''
  }

  function confidenceFromScore(score) {
    return Math.min(0.99, Math.max(0.5, score / 8))
  }

  function stringValue(value) {
    return String(value || '').trim()
  }

  function safeURL(value) {
    try {
      return value ? new URL(value) : null
    } catch {
      return null
    }
  }
})()
