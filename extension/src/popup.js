const connectionBadge = document.querySelector('#connectionBadge')
const deviceEl = document.querySelector('#device')
const siteHostEl = document.querySelector('#siteHost')
const supplierTextEl = document.querySelector('#supplierText')
const statusEl = document.querySelector('#status')
const primaryAction = document.querySelector('#primaryAction')
const secondaryAction = document.querySelector('#secondaryAction')
const stepIndex = document.querySelector('#stepIndex')
const stepLabel = document.querySelector('#stepLabel')
const stepTitle = document.querySelector('#stepTitle')
const stepText = document.querySelector('#stepText')
const contextPanel = document.querySelector('#contextPanel')
const configPanel = document.querySelector('#configPanel')
const adminPlusBaseURLEl = document.querySelector('#adminPlusBaseURL')
const lastResultPanel = document.querySelector('#lastResultPanel')
const lastResultBadge = document.querySelector('#lastResultBadge')
const lastResultTitle = document.querySelector('#lastResultTitle')
const lastResultMeta = document.querySelector('#lastResultMeta')

let state = null
let identification = null
let lastCaptureResult = null
let busy = false
let primaryHandler = connectFromActiveTab
let secondaryHandler = openAdminPlus

primaryAction.addEventListener('click', () => guard(primaryHandler))
secondaryAction.addEventListener('click', () => guard(secondaryHandler))
adminPlusBaseURLEl.addEventListener('keydown', (event) => {
  if (event.key === 'Enter') {
    event.preventDefault()
    guard(saveBaseURLOnly)
  }
})

init().catch(showError)

async function init() {
  await refresh()
}

async function refresh() {
  setBusy(true)
  try {
    state = await sendMessage({ type: 'state:get' })
    lastCaptureResult = await sendMessage({ type: 'capture:last-result' })
    if (state.connection.status === 'connected') {
      identification = await sendMessage({ type: 'site:identify' })
    } else {
      identification = null
    }
    render()
    if (!lastCaptureResult) writeStatus('')
  } finally {
    setBusy(false)
  }
}

async function connectFromActiveTab() {
  setBusy(true)
  try {
    state = await sendMessage({ type: 'connect:from-active-tab', baseURL: currentBaseURLInput() || state?.connection?.baseURL || '' })
    await refresh()
    writeStatus('已连接', 'success')
  } finally {
    setBusy(false)
  }
}

async function openAdminPlus() {
  await sendMessage({ type: 'connect:open-admin-plus', baseURL: currentBaseURLInput() || state?.connection?.baseURL || '' })
  writeStatus('已打开登录页', 'success')
}

async function saveBaseURLOnly() {
  setBusy(true)
  try {
    state = await sendMessage({ type: 'connect:save-base-url', baseURL: currentBaseURLInput() })
    adminPlusBaseURLEl.value = state?.connection?.baseURL || ''
    render()
    writeStatus('地址已保存', 'success')
  } finally {
    setBusy(false)
  }
}

async function capture() {
  if (!identification || !['matched', 'unknown'].includes(identification.status)) {
    throw Object.assign(new Error('当前网站不可上报'), { reason: 'SUPPLIER_SITE_NOT_MATCHED' })
  }
  const startedAt = Date.now()
  setBusy(true)
  try {
    writeStatus('')
    const supplierID = identification.supplier?.id
    const result = await sendMessage({ type: 'session:capture', supplierID, autoCreate: identification.status === 'unknown' })
    lastCaptureResult = await sendMessage({ type: 'capture:last-result' })
    await refresh()
    writeStatus('')
  } catch (error) {
    const storedResult = await sendMessage({ type: 'capture:last-result' }).catch(() => null)
    if (isFreshCaptureResult(storedResult, startedAt)) {
      lastCaptureResult = storedResult
      render()
      writeStatus('')
      return
    }
    throw error
  } finally {
    setBusy(false)
  }
}

function render() {
  const connection = state?.connection || {}
  const connected = connection.status === 'connected'
  const invalid = connection.status === 'expired_or_invalid'
  deviceEl.textContent = shortDeviceID(connection.deviceID)
  connectionBadge.textContent = connected ? '已连接' : invalid ? '需登录' : '未连接'
  connectionBadge.className = `pill ${connected ? 'success' : invalid ? 'warning' : 'neutral'}`
  if (document.activeElement !== adminPlusBaseURLEl) {
    adminPlusBaseURLEl.value = connection.baseURL || adminPlusBaseURLEl.value || ''
  }
  renderLastCaptureResult()

  if (!connected) {
    renderStep({
      index: 1,
      label: '连接',
      title: invalid ? '重新连接 Admin Plus' : '连接 Admin Plus',
      text: invalid ? '登录态已失效，请回到后台页面重新连接。' : '填写后端地址，打开并登录后台后再连接。',
      context: null,
      primaryText: '连接后台',
      primary: connectFromActiveTab,
      secondaryText: '保存地址',
      secondary: saveBaseURLOnly,
      config: true
    })
    return
  }

  const status = identification?.status || 'unknown'
  const host = identification?.activeTab?.host || state?.activeTab?.host || '-'
  if (status === 'matched') {
    const loggedOut = identification?.supplierLogin?.status === 'logged_out'
    renderStep({
      index: loggedOut ? 2 : 3,
      label: loggedOut ? '登录供应商' : '上报',
      title: loggedOut ? '先完成供应商登录' : '上报当前会话',
      text: loggedOut ? '在当前网页完成登录后刷新状态。' : '只上报 token、cookie 和页面上下文。',
      context: { host, supplier: identification.supplier?.name || '-' },
      primaryText: loggedOut ? '刷新状态' : '上报当前会话',
      primary: loggedOut ? refresh : capture,
      secondaryText: '打开 Admin Plus',
      secondary: openAdminPlus
    })
    return
  }

  if (status === 'unknown') {
    renderStep({
      index: 2,
      label: '新供应商',
      title: '创建供应商并上报',
      text: '确认后创建 browser_only 供应商，并保存当前会话。',
      context: { host, supplier: '新供应商' },
      primaryText: '创建并上报',
      primary: capture,
      secondaryText: '刷新状态',
      secondary: refresh
    })
    return
  }

  if (status === 'ambiguous') {
    renderStep({
      index: 2,
      label: '需确认',
      title: '匹配多个供应商',
      text: '请切换到更具体的供应商后台页面。',
      context: { host, supplier: '多个供应商' },
      primaryText: '刷新状态',
      primary: refresh,
      secondaryText: '打开 Admin Plus',
      secondary: openAdminPlus
    })
    return
  }

  renderStep({
    index: 2,
    label: '不可用',
    title: status === 'unsupported' ? '供应商未启用' : '当前页面不可识别',
    text: identification?.message || '请打开供应商后台页面后刷新。',
    context: { host, supplier: '-' },
    primaryText: '刷新状态',
    primary: refresh,
    secondaryText: '打开 Admin Plus',
    secondary: openAdminPlus
  })
}

function renderStep(config) {
  stepIndex.textContent = String(config.index)
  stepLabel.textContent = config.label
  stepTitle.textContent = config.title
  stepText.textContent = config.text
  if (config.context) {
    siteHostEl.textContent = config.context.host || '-'
    supplierTextEl.textContent = config.context.supplier || '-'
  }
  contextPanel.classList.toggle('hidden', !config.context)
  configPanel.classList.toggle('hidden', !config.config)
  primaryAction.textContent = config.primaryText
  secondaryAction.textContent = config.secondaryText
  primaryHandler = config.primary
  secondaryHandler = config.secondary
  primaryAction.disabled = busy
  secondaryAction.disabled = busy
}

function renderLastCaptureResult() {
  if (!lastCaptureResult) {
    lastResultPanel.classList.add('hidden')
    return
  }
  const succeeded = lastCaptureResult.status === 'succeeded'
  const partial = lastCaptureResult.status === 'partial' || Boolean(lastCaptureResult.ingest?.balance_probe_error)
  lastResultPanel.className = `result ${partial ? 'partial' : succeeded ? 'succeeded' : 'failed'}`
  lastResultBadge.textContent = partial ? '已保存' : succeeded ? '成功' : '失败'
  lastResultTitle.textContent = lastCaptureResult.message || (partial ? '会话已保存，余额读取失败' : succeeded ? '最近上报成功' : '最近上报失败')
  lastResultMeta.textContent = formatLastResultMeta(lastCaptureResult)
}

function formatLastResultMeta(result) {
  const parts = []
  if (result.supplier) parts.push(result.supplier)
  if (result.host) parts.push(result.host)
  if (result.taskID) parts.push(`#${result.taskID}`)
  if (result.recordedAt) parts.push(formatTime(result.recordedAt))
  const summary = result.summary || {}
  const evidence = [
    summary.has_access_token ? 'token' : '',
    Number(summary.cookie_count || 0) > 0 ? `${summary.cookie_count} cookies` : ''
  ].filter(Boolean).join(' / ')
  if (evidence) parts.push(evidence)
  return parts.join(' · ')
}

function formatTime(value) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  return date.toLocaleString()
}

function isFreshCaptureResult(result, startedAt) {
  if (!result?.recordedAt) return false
  const recordedAt = new Date(result.recordedAt).getTime()
  return Number.isFinite(recordedAt) && recordedAt >= startedAt - 2000
}

function sendMessage(message) {
  if (typeof globalThis.adminPlusHandleMessage !== 'function') {
    return Promise.reject(new Error('extension app is not loaded'))
  }
  return Promise.resolve(globalThis.adminPlusHandleMessage(message, { source: 'popup' }))
}

function throwMessage(response) {
  const error = new Error(response.error?.message || 'operation failed')
  error.reason = response.error?.reason
  throw error
}

function showError(error) {
  if (error?.reason === 'ADMIN_PLUS_LOGIN_REQUIRED') return writeStatus('请先打开并登录 Admin Plus 后台页', 'failed')
  if (error?.reason === 'ADMIN_PLUS_PAGE_REQUIRED') return writeStatus('请切换到 Admin Plus 后台页', 'failed')
  if (error?.reason === 'ADMIN_PLUS_AUTH_INVALID') return writeStatus('Admin Plus 登录态无效或后端地址不对', 'failed')
  if (error?.reason === 'ADMIN_PLUS_NOT_CONNECTED') return writeStatus('请先连接 Admin Plus', 'failed')
  if (error?.reason === 'SUPPLIER_LOGIN_REQUIRED') return writeStatus('请先在供应商页面登录', 'failed')
  writeStatus(error.message || String(error), 'failed')
}

function writeStatus(message, variant = 'neutral') {
  statusEl.textContent = message
  statusEl.className = `notice ${variant}`
  statusEl.classList.toggle('hidden', !message)
}

function currentBaseURLInput() {
  return String(adminPlusBaseURLEl.value || '').trim()
}

function shortDeviceID(deviceID) {
  if (!deviceID) return ''
  return `设备 ${deviceID.slice(-8)}`
}

function setBusy(nextBusy) {
  busy = nextBusy
  primaryAction.disabled = nextBusy
  secondaryAction.disabled = nextBusy
}

function guard(action) {
  Promise.resolve(action()).catch(showError)
}
