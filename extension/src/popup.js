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

let state = null
let identification = null
let busy = false
let primaryHandler = connectFromActiveTab
let secondaryHandler = openAdminPlus

primaryAction.addEventListener('click', () => guard(primaryHandler))
secondaryAction.addEventListener('click', () => guard(secondaryHandler))

init().catch(showError)

async function init() {
  await refresh()
}

async function refresh() {
  setBusy(true)
  try {
    state = await sendMessage({ type: 'state:get' })
    if (state.connection.status === 'connected') {
      identification = await sendMessage({ type: 'site:identify' })
    } else {
      identification = null
    }
    render()
    writeStatus('')
  } finally {
    setBusy(false)
  }
}

async function connectFromActiveTab() {
  setBusy(true)
  try {
    state = await sendMessage({ type: 'connect:from-active-tab' })
    await refresh()
    writeStatus('已连接')
  } finally {
    setBusy(false)
  }
}

async function openAdminPlus() {
  await sendMessage({ type: 'connect:open-admin-plus', baseURL: state?.connection?.baseURL || '' })
  writeStatus('已打开登录页')
}

async function capture() {
  if (!identification || !['matched', 'unknown'].includes(identification.status)) {
    throw Object.assign(new Error('当前网站不可上报'), { reason: 'SUPPLIER_SITE_NOT_MATCHED' })
  }
  setBusy(true)
  try {
    const supplierID = identification.supplier?.id
    const result = await sendMessage({ type: 'session:capture', supplierID, autoCreate: identification.status === 'unknown' })
    writeStatus(formatCaptureResult(result))
    await refresh()
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

  if (!connected) {
    renderStep({
      index: 1,
      label: '连接',
      title: invalid ? '重新连接 sub2apiplus' : '连接 sub2apiplus',
      text: invalid ? '登录态已失效，请回到后台页面重新连接。' : '先在 sub2apiplus 页面登录，再连接当前页。',
      context: null,
      primaryText: '连接当前页',
      primary: connectFromActiveTab,
      secondaryText: '打开 sub2apiplus',
      secondary: openAdminPlus
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
      secondaryText: '打开 sub2apiplus',
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
      secondaryText: '打开 sub2apiplus',
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
    secondaryText: '打开 sub2apiplus',
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
  primaryAction.textContent = config.primaryText
  secondaryAction.textContent = config.secondaryText
  primaryHandler = config.primary
  secondaryHandler = config.secondary
  primaryAction.disabled = busy
  secondaryAction.disabled = busy
}

function formatCaptureResult(result) {
  if (result.status === 'succeeded') {
    const summary = result.result?.session_summary || {}
    const parts = [
      summary.has_access_token ? 'token' : '',
      Number(summary.cookie_count || 0) > 0 ? `${summary.cookie_count} cookies` : ''
    ].filter(Boolean)
    return `上报成功：${parts.join(' / ') || '会话已保存'}`
  }
  return result?.result?.error_message || '上报失败'
}

function sendMessage(message) {
  return chrome.runtime.sendMessage(message).then((response) => {
    if (!response?.ok) throwMessage(response)
    return response.result
  })
}

function throwMessage(response) {
  const error = new Error(response.error?.message || 'operation failed')
  error.reason = response.error?.reason
  throw error
}

function showError(error) {
  if (error?.reason === 'ADMIN_PLUS_LOGIN_REQUIRED') return writeStatus('请先在 sub2apiplus 页面登录')
  if (error?.reason === 'ADMIN_PLUS_PAGE_REQUIRED') return writeStatus('请切换到 sub2apiplus 后台页')
  if (error?.reason === 'ADMIN_PLUS_AUTH_INVALID') return writeStatus('sub2apiplus 登录态无效')
  if (error?.reason === 'ADMIN_PLUS_NOT_CONNECTED') return writeStatus('请先连接 sub2apiplus')
  if (error?.reason === 'SUPPLIER_LOGIN_REQUIRED') return writeStatus('请先在供应商页面登录')
  writeStatus(error.message || String(error))
}

function writeStatus(message) {
  statusEl.textContent = message
  statusEl.classList.toggle('hidden', !message)
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
