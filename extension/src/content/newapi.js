(() => {
  if (window.AdminPlusNewAPI) return

  window.AdminPlusNewAPI = {
    enrichSessionBundle,
    hasSessionEvidence
  }

  async function enrichSessionBundle({ bundle, storage, supplier }) {
    const context = bundle.context || {}
    const userID = firstNonEmpty(
      context.user_id,
      userIDFromStorage(storage)
    )
    const profile = await probeProfile(storage, supplier, userID)
    const providerType = inferProviderType(supplier, profile)
    if (providerType !== 'new_api') return bundle

    const profileUserID = firstNonEmpty(profile?.id, userID)
    bundle.provider_type = 'new_api'
    bundle.system_type = 'new_api'
    bundle.auth_header_name = 'New-Api-User'
    bundle.auth_header_value = profileUserID

    context.provider_type = 'new_api'
    context.system_type = 'new_api'
    context.api_base_url = supplier?.api_base_url || location.origin
    context.user_id = profileUserID
    if (profile) {
      context.username = stringFromAny(profile.username)
      context.display_name = stringFromAny(profile.display_name)
      context.group = stringFromAny(profile.group)
    }
    bundle.context = context

    const headers = bundle.required_headers || {}
    headers.origin = headers.origin || location.origin
    headers.referer = headers.referer || location.href
    if (profileUserID) {
      headers['New-Api-User'] = profileUserID
    }
    bundle.required_headers = headers
    return bundle
  }

  function hasSessionEvidence(bundle) {
    if (normalizeProviderType(bundle?.provider_type || bundle?.system_type) !== 'new_api') {
      return false
    }
    return Boolean(
      bundle?.required_headers?.['New-Api-User'] ||
      bundle?.required_headers?.['New-API-User'] ||
      bundle?.context?.user_id ||
      bundle?.auth_header_value
    )
  }

  async function probeProfile(storage, supplier, userID) {
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
          'New-Api-User': String(userID)
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

  function inferProviderType(supplier, profile) {
    const explicit = normalizeProviderType(supplier?.type || supplier?.supplier_type || supplier?.provider_type)
    if (explicit) return explicit
    return profile ? 'new_api' : ''
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
      userObjectFromStorage(storage)?.id
    )
  }

  function userObjectFromStorage(storage) {
    for (const [key, value] of Object.entries(storage || {})) {
      const storageKey = key.includes(':') ? key.slice(key.indexOf(':') + 1) : key
      if (storageKey !== 'user') continue
      try {
        const parsed = JSON.parse(String(value || '{}'))
        return parsed && typeof parsed === 'object' ? parsed : {}
      } catch {
        return {}
      }
    }
    return {}
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
