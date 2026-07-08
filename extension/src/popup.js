const connectionBadge = document.querySelector('#connectionBadge')
const deviceEl = document.querySelector('#device')
const siteHostEl = document.querySelector('#siteHost')
const supplierTextEl = document.querySelector('#supplierText')
const statusEl = document.querySelector('#status')
const primaryAction = document.querySelector('#primaryAction')
const captureAction = document.querySelector('#captureAction')
const secondaryAction = document.querySelector('#secondaryAction')
const registrationAction = document.querySelector('#registrationAction')
const stepIndex = document.querySelector('#stepIndex')
const stepLabel = document.querySelector('#stepLabel')
const stepTitle = document.querySelector('#stepTitle')
const stepText = document.querySelector('#stepText')
const contextPanel = document.querySelector('#contextPanel')
const sitePanel = document.querySelector('#sitePanel')
const candidatePanel = document.querySelector('#candidatePanel')
const adminPlusBaseURLEl = document.querySelector('#adminPlusBaseURL')
const saveEndpointAction = document.querySelector('#saveEndpointAction')
const connectEndpointAction = document.querySelector('#connectEndpointAction')
const supplierTypeEl = document.querySelector('#supplierType')
const supplierNameEl = document.querySelector('#supplierName')
const supplierContactEl = document.querySelector('#supplierContact')
const supplierUsernameEl = document.querySelector('#supplierUsername')
const supplierPasswordEl = document.querySelector('#supplierPassword')
const togglePasswordAction = document.querySelector('#togglePasswordAction')
const supplierTokenEl = document.querySelector('#supplierToken')
const supplierNotesEl = document.querySelector('#supplierNotes')
const passwordHintEl = document.querySelector('#passwordHint')
const readCredentialAction = document.querySelector('#readCredentialAction')
const confidenceEl = document.querySelector('#confidence')
const evidenceEl = document.querySelector('#evidence')
const lastResultPanel = document.querySelector('#lastResultPanel')
const lastResultBadge = document.querySelector('#lastResultBadge')
const lastResultTitle = document.querySelector('#lastResultTitle')
const lastResultMeta = document.querySelector('#lastResultMeta')
const lastResultDetail = document.querySelector('#lastResultDetail')

let state = null
let identification = null
let candidate = null
let candidateFormScope = ''
let lastCaptureResult = null
let busy = false
let primaryHandler = connectFromActiveTab
let secondaryHandler = openAdminPlus

primaryAction.addEventListener('click', () => guard(primaryHandler))
captureAction.addEventListener('click', () => guard(captureCurrentSession))
secondaryAction.addEventListener('click', () => guard(secondaryHandler))
registrationAction.addEventListener('click', () => guard(runRegistrationTask))
saveEndpointAction.addEventListener('click', () => guard(saveBaseURLOnly))
connectEndpointAction.addEventListener('click', () => guard(connectFromActiveTab))
adminPlusBaseURLEl.addEventListener('keydown', (event) => {
  if (event.key === 'Enter') {
    event.preventDefault()
    guard(saveBaseURLOnly)
  }
})
supplierTypeEl.addEventListener('change', () => {
  renderCandidatePanel()
})
readCredentialAction.addEventListener('click', () => guard(readCredentialFromPage))
togglePasswordAction.addEventListener('click', togglePasswordVisibility)

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
      candidate = identification?.candidate || null
    } else {
      identification = null
      candidate = null
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

async function capture(options = {}) {
  if (!identification) {
    throw Object.assign(new Error('当前网站不可上报'), { reason: 'SUPPLIER_SITE_NOT_MATCHED' })
  }
  const startedAt = Date.now()
  setBusy(true)
  try {
    writeStatus('')
    const currentCandidate = options.candidate || await collectCurrentCandidate(Boolean(options.includeSensitive))
    const supplierID = options.supplierID || currentCandidate.supplier_id || identification.supplier?.id || 0
    const result = await sendMessage({
      type: 'session:capture',
      supplierID,
      autoCreate: options.autoCreate === true,
      candidate: currentCandidate,
      credentials: options.credentials || currentCandidate.credential
    })
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

async function captureCurrentSession() {
  const currentCandidate = await collectCurrentCandidate(true)
  if (!identification?.supplier && !currentCandidate.supplier_id) {
    if (!currentCandidate.provider_type) {
      throw Object.assign(new Error('无法判断系统类型，请选择'), { reason: 'SUPPLIER_TYPE_REQUIRED' })
    }
    if (canCaptureSessionWithoutCredential(currentCandidate)) {
      await capture({
        autoCreate: true,
        candidate: currentCandidate,
        credentials: currentCandidate.credential
      })
      return
    }
    if (!hasCompleteBrowserCredential(currentCandidate)) {
      throw Object.assign(new Error('请先填写账号和密码后再上报 Session'), { reason: 'SUPPLIER_CREDENTIAL_REQUIRED' })
    }
    setBusy(true)
    try {
      writeStatus('正在保存账号密码', 'neutral')
      const report = await reportCandidate(currentCandidate, true)
      if (!report?.supplier_id) {
        throw Object.assign(new Error('供应商未保存，无法上报 Session'), { reason: 'SUPPLIER_SITE_NOT_MATCHED' })
      }
      const savedCandidate = withReportedSupplier(currentCandidate, report)
      setBusy(false)
      await capture({
        supplierID: report.supplier_id,
        candidate: savedCandidate,
        credentials: savedCandidate.credential
      })
    } finally {
      setBusy(false)
    }
    return
  }
  await capture({
    candidate: currentCandidate,
    credentials: currentCandidate.credential
  })
}

async function runRegistrationTask() {
  setBusy(true)
  try {
    writeStatus('正在领取注册任务...', 'neutral')
    const result = await sendMessage({ type: 'registration:run-next' })
    if (result.status === 'running') {
      writeStatus(result.message || '已有注册任务正在执行', 'neutral')
      return
    }
    if (result.status === 'waiting_manual_verification') {
      writeStatus(result.message || '需要人工完成验证码或邮箱验证', 'failed')
      await refresh()
      return
    }
    if (result.status === 'succeeded') {
      writeStatus(result.message || '注册表单已提交', 'success')
      await refresh()
      return
    }
    writeStatus(result.message || '注册任务执行失败', 'failed')
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
  renderRegistrationAction(connected)

  if (!connected) {
    hideSitePanels()
    renderCaptureAction(false)
    renderStep({
      index: 1,
      label: '连接',
      title: invalid ? '重新连接 Admin Plus' : '连接 Admin Plus',
      text: invalid ? '登录态已失效，请回到后台页面重新连接。' : '填写后端地址，打开并登录后台后再连接。',
      context: null,
      primaryText: '连接后台',
      primary: connectFromActiveTab,
      secondaryText: '保存地址',
      secondary: saveBaseURLOnly
    })
    return
  }

  const site = identification?.activeTab || state?.activeTab || {}
  const siteStatus = identification?.status || 'unsupported'
  const candidateStatus = candidate?.status || (identification?.supplier ? 'identified' : 'unsupported')
  const resolvedStatus = identification?.supplier ? 'identified' : siteStatus
  const sessionFirst = canCaptureSessionWithoutCredential(candidate || identification?.candidate || {})
  if (siteStatus === 'ambiguous') {
    renderStep({
      index: 2,
      label: '需处理',
      title: '供应商已存在多个候选',
      text: '当前站点匹配多个供应商，已停止自动创建和更新，请在后台人工处理。',
      context: { host: site.host || '-', supplier: '多个供应商' },
      primaryText: '刷新状态',
      primary: refresh,
      secondaryText: '打开 Admin Plus',
      secondary: openAdminPlus
    })
    renderSitePanel(site, 'ambiguous')
    candidatePanel.classList.add('hidden')
    renderCaptureAction(false)
    return
  }

  renderStep({
    index: resolvedStatus === 'identified' ? 2 : 2,
    label: resolvedStatus === 'identified' ? '已识别' : candidateStatus === 'needs_type_selection' ? '需选择' : '待识别',
    title: resolvedStatus === 'identified' ? '供应商已识别' : candidateStatus === 'needs_type_selection' ? '无法判断系统类型，请选择' : '当前页面未匹配供应商',
    text: sessionFirst
      ? '已识别到供应商，可直接上报当前 Session；账号密码仅用于保存浏览器登录。'
      : resolvedStatus === 'identified'
        ? '已识别到供应商，可提交账号密码或上报当前 Session。'
      : candidateStatus === 'needs_type_selection'
        ? '页面特征不足，先选择系统类型，再提交账号密码或候选。'
        : '未注册站点可先提交账号密码，保存后再上报 Session。',
    context: { host: site.host || '-', supplier: identification?.supplier?.name || candidate?.defaults?.name || '-' },
    primaryText: sessionFirst ? '上报 Session' : '提交账号密码',
    primary: sessionFirst ? captureCurrentSession : submitSiteCandidate,
    secondaryText: '刷新状态',
    secondary: refresh
  })

  renderSitePanel(site, resolvedStatus)
  renderCandidatePanel()
  renderCaptureAction(!sessionFirst)
}

function renderSitePanel(site, resolvedStatus) {
  if (!sitePanel) return
  sitePanel.classList.toggle('hidden', false)
  siteHostEl.textContent = site.host || '-'
  supplierTextEl.textContent = identification?.supplier?.name || candidate?.defaults?.name || '-'
  statusEl.textContent = ''
  if (resolvedStatus === 'ambiguous') {
    writeStatus('供应商已存在多个候选，请人工处理', 'failed')
  } else if (candidate?.status === 'identified' || resolvedStatus === 'identified') {
    const sessionFirst = canCaptureSessionWithoutCredential(candidate || identification?.candidate || {})
    writeStatus(identification?.message || (sessionFirst ? '供应商已识别，可直接上报 Session' : '供应商已识别，可提交账号密码或上报 Session'), 'success')
  } else if (candidate?.status === 'needs_type_selection') {
    writeStatus('无法判断系统类型，请选择', 'neutral')
  } else {
    writeStatus(identification?.message || '当前页面未匹配已配置供应商', 'neutral')
  }
}

function renderCandidatePanel() {
  const visible = Boolean(identification && state?.connection?.status === 'connected' && identification?.status !== 'ambiguous')
  candidatePanel.classList.toggle('hidden', !visible)
  if (!visible) return

  const nextScope = candidateFormScopeKey()
  if (nextScope !== candidateFormScope) {
    candidateFormScope = nextScope
    clearCandidateForm()
  }

  const defaults = candidate?.defaults || {}
  const credential = candidate?.credential || {}
  const currentType = supplierTypeEl.value || candidate?.provider_type || identification?.supplier?.type || ''
  supplierTypeEl.value = currentType || ''
  supplierNameEl.value = supplierNameEl.value || identification?.supplier?.name || defaults.name || identification?.activeTab?.title || identification?.activeTab?.host || ''
  supplierContactEl.value = supplierContactEl.value || defaults.contact || credential.username || ''
  supplierUsernameEl.value = supplierUsernameEl.value || credential.username || ''
  supplierTokenEl.value = supplierTokenEl.value || credential.token || ''
  supplierNotesEl.value = supplierNotesEl.value || buildNotes()
  confidenceEl.textContent = formatConfidence(candidate?.confidence || 0)
  evidenceEl.textContent = (candidate?.evidence || []).join(' · ') || '无明显指纹'

  const password = supplierPasswordEl.value
  const sessionFirst = canCaptureSessionWithoutCredential(candidate || {})
  passwordHintEl.textContent = sessionFirst ? '当前已登录页通常无法读取密码，可直接上报 Session。' : credential.password_present ? '密码只会在点击读取页面凭据或创建时读取。' : '无法自动读取密码，请手动输入。'
  if (!password && credential.password_present) {
    passwordHintEl.textContent = '页面存在密码输入框，点击读取页面凭据或创建时会尝试读取。'
  }

  const needsType = !identification?.supplier && !candidate?.provider_type && supplierTypeEl.value === ''
  primaryAction.textContent = sessionFirst ? '上报 Session' : '提交账号密码'
  primaryHandler = sessionFirst ? captureCurrentSession : submitSiteCandidate
  secondaryHandler = refresh
  secondaryAction.textContent = '刷新状态'
  primaryAction.disabled = busy || (!identification?.supplier && needsType)
  secondaryAction.disabled = busy
  readCredentialAction.disabled = busy
  readCredentialAction.textContent = sessionFirst ? '读取登录页账号密码' : '读取页面凭据'
}

async function readCredentialFromPage() {
  setBusy(true)
  try {
    const pageCandidate = await readPageCandidate(true)
    const credential = pageCandidate?.credential || {}
    candidate = pageCandidate || candidate
    candidateFormScope = candidateFormScopeKey()
    if (credential.username) {
      supplierUsernameEl.value = credential.username
      if (!supplierContactEl.value) supplierContactEl.value = credential.username
    }
    if (credential.password) {
      supplierPasswordEl.value = credential.password
    }
    if (credential.token) {
      supplierTokenEl.value = credential.token
    }
    if (!credential.username && !credential.password && !credential.token) {
      const canCaptureDirectly = canCaptureSessionWithoutCredential(pageCandidate)
      writeStatus(credentialReadFailureMessage(credential, pageCandidate), canCaptureDirectly ? 'neutral' : 'failed')
      return
    }
    renderCandidatePanel()
    writeStatus(credential.password ? '已读取页面凭据' : '已读取部分页面凭据，密码仍需手动填写', 'success')
  } finally {
    setBusy(false)
  }
}

function credentialReadFailureMessage(credential, pageCandidate = null) {
  const debug = credential?.debug || {}
  if (debug.password_input_count > 0 && !debug.password_value_present) {
    return '找到密码输入框，但页面没有暴露密码值，请手动填写'
  }
  if (debug.input_count > 0) {
    return '页面表单没有暴露可读取的账号、密码或 Token，请手动填写'
  }
  if (canCaptureSessionWithoutCredential(pageCandidate || candidate || {})) {
    return '当前已登录页面没有登录表单，无法读取密码；可直接点击上报 Session'
  }
  return '未找到可读取的登录表单，请手动填写'
}

async function submitSiteCandidate() {
  const currentCandidate = await collectCurrentCandidate(true)
  if (!currentCandidate.provider_type) {
    throw Object.assign(new Error('无法判断系统类型，请选择'), { reason: 'SUPPLIER_TYPE_REQUIRED' })
  }
  const hasCredential = hasAnyBrowserCredential(currentCandidate)
  const canCreateSupplier = hasCompleteBrowserCredential(currentCandidate)
  if (!hasCredential) {
    throw Object.assign(new Error('请先填写账号和密码'), { reason: 'SUPPLIER_CREDENTIAL_REQUIRED' })
  }
  if (!identification?.supplier && !currentCandidate.supplier_id && !canCreateSupplier) {
    throw Object.assign(new Error('未入库站点需要账号和密码'), { reason: 'SUPPLIER_CREDENTIAL_REQUIRED' })
  }
  setBusy(true)
  try {
    writeStatus('正在提交账号密码', 'neutral')
    const report = await reportCandidate(currentCandidate, canCreateSupplier)
    let message = report?.message || '账号密码已提交'
    if (report?.credential_saved) {
      message = report.message || '账号密码已保存'
    } else if (report?.already_exists) {
      message = '供应商已存在，可继续上报 Session'
    } else if (report?.created) {
      message = report.message || '供应商已创建并保存账号密码'
    } else {
      message = report?.message || '候选已提交，请在后台注册任务中发起注册'
    }
    await refresh()
    writeStatus(message, 'success')
  } finally {
    setBusy(false)
  }
}

async function reportCandidate(currentCandidate, autoCreate) {
  return sendMessage({
    type: 'supplier:report-candidate',
    payload: buildReportPayload(currentCandidate, { autoCreate })
  })
}

function hasCompleteBrowserCredential(currentCandidate) {
  const credential = currentCandidate?.credential || {}
  return Boolean(String(credential.username || '').trim() && String(credential.password || '').trim())
}

function hasAnyBrowserCredential(currentCandidate) {
  const credential = currentCandidate?.credential || {}
  return Boolean(
    String(credential.username || '').trim() ||
    String(credential.password || '').trim() ||
    String(credential.token || '').trim()
  )
}

function canCaptureSessionWithoutCredential(currentCandidate) {
  const providerType = normalizeProviderType(
    currentCandidate?.provider_type ||
    currentCandidate?.system_type ||
    currentCandidate?.type ||
    currentCandidate?.supplier_type ||
    supplierTypeEl.value ||
    identification?.supplier?.type ||
    ''
  )
  return providerType === 'new_api' || providerType === 'sub2api'
}

function withReportedSupplier(currentCandidate, report) {
  const page = currentCandidate?.page || {}
  const defaults = currentCandidate?.defaults || {}
  return {
    ...currentCandidate,
    supplier_id: Number(report?.supplier_id || currentCandidate?.supplier_id || 0),
    supplier_name: report?.supplier_name || currentCandidate?.supplier_name || currentCandidate?.name || '',
    dashboard_url: currentCandidate?.dashboard_url || defaults.dashboard_url || page.url || page.origin || '',
    api_base_url: currentCandidate?.api_base_url || defaults.api_base_url || page.origin || ''
  }
}

async function collectCurrentCandidate(includeSensitive = false) {
  const pageCandidate = includeSensitive ? await readPageCandidate(true).catch(() => null) : null
  return collectCandidateForm(pageCandidate || candidate || {}, includeSensitive)
}

async function readPageCandidate(includeSensitive = false) {
  const response = await sendMessage({
    type: 'site:collect-candidate',
    includeSensitive
  })
  return response || null
}

function collectCandidateForm(sourceCandidate = candidate || {}, includeSensitive = false) {
  const defaults = sourceCandidate?.defaults || {}
  const page = sourceCandidate?.page || {}
  const sourceCredential = sourceCandidate?.credential || {}
  const formPassword = String(supplierPasswordEl.value || '').trim()
  const livePassword = includeSensitive ? String(sourceCredential.password || '').trim() : ''
  return {
    provider_type: normalizeProviderType(supplierTypeEl.value || sourceCandidate?.provider_type || identification?.supplier?.type || ''),
    name: String(supplierNameEl.value || identification?.supplier?.name || defaults.name || page.title || page.host || '').trim(),
    contact: String(supplierContactEl.value || defaults.contact || supplierUsernameEl.value || '').trim(),
    credential: {
      username: String(supplierUsernameEl.value || sourceCredential.username || '').trim(),
      password: formPassword || livePassword,
      token: String(supplierTokenEl.value || sourceCredential.token || '').trim()
    },
    notes: String(supplierNotesEl.value || buildNotes()).trim(),
    page,
    defaults,
    evidence: Array.isArray(sourceCandidate?.evidence) ? sourceCandidate.evidence : [],
    confidence: Number(sourceCandidate?.confidence || 0),
    supplier_id: Number(identification?.supplier?.id || sourceCandidate?.supplier_id || 0)
  }
}

function buildReportPayload(currentCandidate, options = {}) {
  const page = currentCandidate.page || {}
  const defaults = currentCandidate.defaults || {}
  const credential = currentCandidate.credential || {}
  const providerType = currentCandidate.provider_type || ''
  return {
    device_id: state?.connection?.deviceID || '',
    auto_create_supplier: options.autoCreate === true,
    provider_type: providerType,
    system_type: providerType,
    type: providerType,
    supplier_type: providerType,
    name: currentCandidate.name || defaults.name || page.title || page.host || '当前供应商',
    contact: currentCandidate.contact || credential.username || '',
    supplier_kind: defaults.supplier_kind || 'relay',
    runtime_status: defaults.runtime_status || 'monitor_only',
    health_status: defaults.health_status || 'normal',
    balance_cents: Number(defaults.balance_cents || 0),
    balance_currency: defaults.balance_currency || 'USD',
    recharge_multiplier: Number(defaults.recharge_multiplier || 1),
    dashboard_url: defaults.dashboard_url || page.url || page.origin || '',
    api_base_url: defaults.api_base_url || page.origin || '',
    third_party_recharge_url: defaults.third_party_recharge_url || '',
    local_recharge_url: defaults.local_recharge_url || '',
    source_host: page.host || '',
    source_url: page.url || '',
    origin: page.origin || '',
    browser_login_enabled: Boolean(credential.username && credential.password),
    browser_login_username: credential.username || '',
    browser_login_password: credential.password || '',
    browser_login_token: credential.token || '',
    notes: currentCandidate.notes || 'reported from Chrome plugin',
    page_context: {
      title: page.title || '',
      url: page.url || '',
      host: page.host || '',
      identification_evidence: Array.isArray(currentCandidate.evidence) ? currentCandidate.evidence : []
    }
  }
}

function candidateFormScopeKey() {
  const site = identification?.activeTab || {}
  const page = candidate?.page || {}
  return [
    identification?.supplier?.id || '',
    site.origin || page.origin || '',
    site.host || page.host || ''
  ].join('|')
}

function clearCandidateForm() {
  supplierTypeEl.value = ''
  supplierNameEl.value = ''
  supplierContactEl.value = ''
  supplierUsernameEl.value = ''
  supplierPasswordEl.value = ''
  supplierPasswordEl.type = 'password'
  togglePasswordAction.textContent = '显示'
  supplierTokenEl.value = ''
  supplierNotesEl.value = ''
}

function hideSitePanels() {
  sitePanel.classList.add('hidden')
  candidatePanel.classList.add('hidden')
  renderCaptureAction(false)
}

function buildNotes() {
  const evidence = Array.isArray(candidate?.evidence) ? candidate.evidence.slice(0, 6) : []
  const pieces = ['reported from Chrome plugin']
  if (candidate?.provider_type) pieces.push(`type:${candidate.provider_type}`)
  if (evidence.length > 0) pieces.push(`evidence:${evidence.join('|')}`)
  return pieces.join(' · ')
}

function normalizeProviderType(value) {
  const normalized = String(value || '').trim().toLowerCase()
  if (normalized === 'newapi' || normalized === 'new-api') return 'new_api'
  if (['subapi', 'sub api', 'sub-api', 'sub_api', 'sub2api', 'sub2 api', 'sub2-api', 'sub2_api'].includes(normalized)) return 'sub2api'
  return normalized
}

function formatConfidence(value) {
  const num = Number(value || 0)
  if (!Number.isFinite(num) || num <= 0) return '置信度 0%'
  return `置信度 ${(num * 100).toFixed(0)}%`
}

function renderRegistrationAction(connected) {
  registrationAction.classList.toggle('hidden', !connected)
  registrationAction.disabled = busy || !connected
}

function renderCaptureAction(visible) {
  captureAction.classList.toggle('hidden', !visible)
  captureAction.disabled = busy || !visible || state?.connection?.status !== 'connected' || !identification
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
  captureAction.disabled = busy || captureAction.classList.contains('hidden')
  secondaryAction.disabled = busy
  registrationAction.disabled = busy || state?.connection?.status !== 'connected'
}

function renderLastCaptureResult() {
  if (!lastCaptureResult) {
    lastResultPanel.classList.add('hidden')
    return
  }
  const succeeded = lastCaptureResult.status === 'succeeded'
  lastResultPanel.className = `result ${succeeded ? 'succeeded' : 'failed'}`
  lastResultBadge.textContent = succeeded ? '成功' : '失败'
  lastResultTitle.textContent = lastCaptureResult.message || (succeeded ? '最近上报成功' : '最近上报失败')
  lastResultMeta.textContent = formatLastResultMeta(lastCaptureResult)
  lastResultDetail.textContent = ''
  lastResultDetail.classList.add('hidden')
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
  if (globalThis.chrome?.runtime?.sendMessage) {
    return new Promise((resolve, reject) => {
      chrome.runtime.sendMessage(message, (response) => {
        const runtimeError = chrome.runtime.lastError
        if (runtimeError) {
          reject(new Error(runtimeError.message || 'extension app is not loaded'))
          return
        }
        if (response?.__adminPlusError) {
          const error = new Error(response.message || 'extension app request failed')
          error.reason = response.reason || ''
          reject(error)
          return
        }
        resolve(response?.__adminPlusOK ? response.result : response)
      })
    })
  }
  if (typeof globalThis.adminPlusHandleMessage === 'function') {
    return Promise.resolve(globalThis.adminPlusHandleMessage(message, { source: 'popup' }))
  }
  return Promise.reject(Object.assign(new Error('请从 Chrome 扩展弹窗打开，不要作为普通网页打开'), { reason: 'EXTENSION_RUNTIME_UNAVAILABLE' }))
}

function showError(error) {
  if (error?.reason === 'EXTENSION_RUNTIME_UNAVAILABLE') return writeStatus(error.message || '扩展运行环境不可用', 'failed')
  if (error?.reason === 'ADMIN_PLUS_LOGIN_REQUIRED') return writeStatus('请先打开并登录 Admin Plus 后台页', 'failed')
  if (error?.reason === 'ADMIN_PLUS_PAGE_REQUIRED') return writeStatus('请切换到 Admin Plus 后台页', 'failed')
  if (error?.reason === 'ADMIN_PLUS_AUTH_INVALID') return writeStatus('Admin Plus 登录态无效或后端地址不对', 'failed')
  if (error?.reason === 'ADMIN_PLUS_NOT_CONNECTED') return writeStatus('请先连接 Admin Plus', 'failed')
  if (error?.reason === 'EXTENSION_TASK_NOT_AVAILABLE') return writeStatus('暂无待执行注册任务', 'failed')
  if (error?.reason === 'SUPPLIER_LOGIN_REQUIRED') return writeStatus('请先在供应商页面登录', 'failed')
  if (error?.reason === 'SUPPLIER_TYPE_REQUIRED') return writeStatus('无法判断系统类型，请选择', 'failed')
  if (error?.reason === 'SUPPLIER_USERNAME_REQUIRED') return writeStatus('请填写账号', 'failed')
  if (error?.reason === 'SUPPLIER_PASSWORD_REQUIRED') return writeStatus('无法自动读取密码，请手动输入', 'failed')
  if (error?.reason === 'SUPPLIER_CREDENTIAL_REQUIRED') return writeStatus('请先填写账号和密码', 'failed')
  if (error?.reason === 'SUPPLIER_CREDENTIAL_SAVE_FAILED') return writeStatus('供应商凭据未保存，已停止上报', 'failed')
  if (error?.reason === 'SUPPLIER_SITE_AMBIGUOUS') return writeStatus('供应商已存在多个候选，请人工处理', 'failed')
  if (error?.reason === 'SUPPLIER_SITE_REGISTRATION_REQUIRED') return writeStatus('未注册站点请先在后台注册任务中发起注册', 'failed')
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
  captureAction.disabled = nextBusy || captureAction.classList.contains('hidden')
  secondaryAction.disabled = nextBusy
  registrationAction.disabled = nextBusy || state?.connection?.status !== 'connected'
  saveEndpointAction.disabled = nextBusy
  connectEndpointAction.disabled = nextBusy
  togglePasswordAction.disabled = nextBusy
  if (readCredentialAction) readCredentialAction.disabled = nextBusy
}

function togglePasswordVisibility() {
  const visible = supplierPasswordEl.type === 'text'
  supplierPasswordEl.type = visible ? 'password' : 'text'
  togglePasswordAction.textContent = visible ? '显示' : '隐藏'
}

function guard(action) {
  Promise.resolve(action()).catch(showError)
}
