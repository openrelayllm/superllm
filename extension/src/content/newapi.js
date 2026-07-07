(() => {
  if (window.AdminPlusNewAPI) return
  const NEW_API_USER_HEADER_NAMES = [
    'New-Api-User',
    'New-API-User',
    'Veloera-User',
    'voapi-user',
    'User-id',
    'X-User-Id',
    'Rix-Api-User',
    'neo-api-user'
  ]
  const NEW_API_USER_HEADER_ALIASES = [
    ...NEW_API_USER_HEADER_NAMES,
    'new-api-user'
  ]

  window.AdminPlusNewAPI = {
    enrichSessionBundle,
    hasSessionEvidence
  }

  async function enrichSessionBundle({ bundle, storage, supplier }) {
    const context = bundle.context || {}
    const storageUser = userObjectFromStorage(storage)
    const userID = normalizePositiveIntegerText(firstNonEmpty(
      context.user_id,
      userIDFromStorage(storage),
      userIDFromTokenStorage(storage),
      storageUser?.id
    ))
    const profile = await probeProfile(storage, supplier, userID)
    const providerType = inferProviderType(supplier, profile, storageUser, storage)
    if (providerType !== 'new_api') return bundle

    const profileUser = profile || storageUser || {}
    const profileUserID = normalizePositiveIntegerText(firstNonEmpty(profileUser?.id, userID))
    bundle.provider_type = 'new_api'
    bundle.system_type = 'new_api'
    bundle.auth_header_name = 'New-Api-User'
    bundle.auth_header_value = profileUserID

    context.provider_type = 'new_api'
    context.system_type = 'new_api'
    context.api_base_url = supplier?.api_base_url || location.origin
    context.user_id = profileUserID
    if (profileUser) {
      context.username = stringFromAny(profileUser.username)
      context.display_name = stringFromAny(profileUser.display_name || profileUser.displayName)
      context.email = stringFromAny(profileUser.email)
      context.group = stringFromAny(profileUser.group)
      context.role = stringFromAny(profileUser.role)
      context.status = stringFromAny(profileUser.status)
    }
    bundle.context = context

    const headers = bundle.required_headers || {}
    headers.origin = headers.origin || location.origin
    headers.referer = headers.referer || location.href
    if (profileUserID) {
      applyNewAPIUserHeaders(headers, profileUserID)
    }
    bundle.required_headers = headers
    return bundle
  }

  function hasSessionEvidence(bundle) {
    if (normalizeProviderType(bundle?.provider_type || bundle?.system_type) !== 'new_api') {
      return false
    }
    return Boolean(
      newAPIUserHeader(bundle) ||
      bundle?.context?.user_id ||
      bundle?.auth_header_value
    )
  }

  async function probeProfile(storage, supplier, userID) {
    userID = normalizePositiveIntegerText(userID)
    const providerType = normalizeProviderType(supplier?.type || supplier?.supplier_type || supplier?.provider_type)
    if (!userID || (providerType !== 'new_api' && !hasStorageMarkers(storage))) {
      return null
    }
    const controller = new AbortController()
    const timer = setTimeout(() => controller.abort(), 3000)
    try {
      const response = await fetch('/api/user/self', {
        method: 'GET',
        credentials: 'include',
        cache: 'no-store',
        headers: {
          'Accept': 'application/json',
          'Cache-Control': 'no-store',
          ...newAPIUserHeaders(userID)
        },
        signal: controller.signal
      })
      if (!response.ok) return null
      const envelope = await response.json()
      if (!envelope?.success || !envelope?.data) return null
      const profileID = String(envelope.data.id || '')
      if (profileID && profileID !== String(userID)) return null
      return envelope.data
    } catch {
      return null
    } finally {
      clearTimeout(timer)
    }
  }

  function inferProviderType(supplier, profile, storageUser, storage) {
    const explicit = normalizeProviderType(supplier?.type || supplier?.supplier_type || supplier?.provider_type)
    if (explicit) return explicit
    if (profile || storageUser) return 'new_api'
    return hasStorageMarkers(storage) ? 'new_api' : ''
  }

  function normalizeProviderType(value) {
    const normalized = String(value || '').trim().toLowerCase()
    if (normalized === 'newapi' || normalized === 'new-api') return 'new_api'
    return normalized
  }

  function hasStorageMarkers(storage) {
    const keys = Object.keys(storage).map((key) => key.includes(':') ? key.slice(key.indexOf(':') + 1) : key)
    const hasUser = keys.includes('user') || keys.includes('uid')
    const hasQuotaSetting = keys.includes('quota_per_unit') || keys.includes('quota_display_type') || keys.includes('display_in_currency')
    return hasUser && hasQuotaSetting
  }

  function userIDFromStorage(storage) {
    return firstNonEmpty(
      firstStorageValue(storage, ['uid'], { exact: true }),
      firstStorageValue(storage, ['user_id', 'userid'], { exact: true }),
      firstStorageValue(storage, ['new_api_user', 'new-api-user', 'newApiUser'], { exact: true }),
      userObjectFromStorage(storage)?.id
    )
  }

  function userIDFromTokenStorage(storage) {
    for (const token of [
      firstStorageValue(storage, ['auth_token', 'authToken', 'access_token', 'accessToken', 'token', 'jwt'], { exact: true }),
      firstStorageValue(storage, ['auth_token', 'authToken', 'access_token', 'accessToken', 'token', 'jwt'])
    ]) {
      const userID = userIDFromJWT(token)
      if (userID) return userID
    }
    return ''
  }

  function userIDFromJWT(value) {
    const token = String(value || '').replace(/^Bearer\s+/i, '').trim()
    const parts = token.split('.')
    if (parts.length !== 3) return ''
    const payload = parseJSON(decodeBase64URL(parts[1]))
    return normalizePositiveIntegerText(firstNonEmpty(
      payload?.id,
      payload?.sub,
      payload?.user_id,
      payload?.userId,
      payload?.uid
    ))
  }

  function decodeBase64URL(value) {
    const raw = String(value || '').trim()
    if (!raw || raw.length > 8192) return ''
    const normalized = raw.replace(/-/g, '+').replace(/_/g, '/')
    const padded = normalized + '='.repeat((4 - (normalized.length % 4)) % 4)
    try {
      return atob(padded)
    } catch {
      return ''
    }
  }

  function userObjectFromStorage(storage) {
    const prioritized = []
    const fallback = []
    for (const [key, value] of Object.entries(storage || {})) {
      const storageKey = key.includes(':') ? key.slice(key.indexOf(':') + 1) : key
      const normalized = storageKey.toLowerCase()
      if (['user', 'userstorage', 'auth-store', 'auth_storage', 'authstore'].includes(normalized)) {
        prioritized.push(value)
      } else if (looksLikeJSON(value)) {
        fallback.push(value)
      }
    }
    for (const value of [...prioritized, ...fallback]) {
      const parsed = parseJSON(value)
      const user = findUserLikeObject(parsed)
      if (user?.id) return user
    }
    return {}
  }

  function findUserLikeObject(value, depth = 0) {
    if (!value || typeof value !== 'object' || depth > 5) return null
    const candidate = normalizeUserCandidate(value)
    if (candidate?.id) return candidate
    for (const key of ['user', 'currentUser', 'profile', 'self', 'data', 'auth', 'state']) {
      const found = findUserLikeObject(value[key], depth + 1)
      if (found?.id) return found
    }
    for (const nested of Object.values(value)) {
      const found = findUserLikeObject(nested, depth + 1)
      if (found?.id) return found
    }
    return null
  }

  function normalizeUserCandidate(value) {
    if (!value || typeof value !== 'object') return null
    const id = firstNonEmpty(value.id, value.user_id, value.userId, value.uid)
    if (!id) return null
    const identity = firstNonEmpty(
      value.username,
      value.display_name,
      value.displayName,
      value.email
    )
    const role = firstNonEmpty(value.role)
    const statusOrUsage = firstNonEmpty(value.status, value.quota, value.used_quota, value.request_count)
    if (!identity && !(role && statusOrUsage)) return null
    return {
      id,
      username: value.username,
      display_name: value.display_name || value.displayName,
      email: value.email,
      group: value.group,
      role: value.role,
      status: value.status
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

  function firstStorageValue(storage, patterns, options = {}) {
    for (const [key, value] of Object.entries(storage || {})) {
      const storageKey = key.includes(':') ? key.slice(key.indexOf(':') + 1) : key
      const normalized = storageKey.toLowerCase()
      const matched = patterns.some((pattern) => {
        const expected = String(pattern).toLowerCase()
        return options.exact ? normalized === expected : normalized.includes(expected)
      })
      if (!matched) continue
      const parsed = stringFromAny(value)
      if (parsed) return parsed
    }
    return ''
  }

  function newAPIUserHeader(bundle) {
    const headers = bundle?.required_headers || {}
    return firstNonEmpty(...NEW_API_USER_HEADER_ALIASES.map((headerName) => headers[headerName]))
  }

  function applyNewAPIUserHeaders(headers, userID) {
    const value = normalizePositiveIntegerText(userID)
    if (!headers || !value) return headers
    for (const headerName of NEW_API_USER_HEADER_NAMES) {
      headers[headerName] = value
    }
    return headers
  }

  function newAPIUserHeaders(userID) {
    const headers = {}
    applyNewAPIUserHeaders(headers, userID)
    return headers
  }

  function normalizePositiveIntegerText(value) {
    const text = firstNonEmpty(value)
    if (!/^\d+$/.test(text)) return ''
    const numeric = Number(text)
    if (!Number.isSafeInteger(numeric) || numeric <= 0 || numeric > 10000000) return ''
    return String(numeric)
  }

  function firstNonEmpty(...values) {
    for (const value of values) {
      const text = stringFromAny(value)
      if (text) return text
    }
    return ''
  }

  function stringFromAny(value) {
    if (value === undefined || value === null) return ''
    return String(value).trim()
  }
})()
