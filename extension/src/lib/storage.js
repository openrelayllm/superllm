const CONFIG_KEY = 'adminPlusOperatorConfig'
const DEFAULT_CONFIG_PATH = 'config/default-config.json'

export async function loadConfig() {
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

export async function saveConfig(config) {
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
  const raw = String(value || '').trim().replace(/\/+$/, '')
  if (!raw) return ''
  try {
    const url = new URL(raw)
    if (!['http:', 'https:'].includes(url.protocol)) return ''
    return url.origin
  } catch {
    return ''
  }
}
