import { extractApiErrorCode } from '@/utils/apiError'
import { supplierGroupAction } from '../supplierProvisionPresentation'
import { supplierBalanceDeltaCents } from '../supplierCostPresentation'
import type { Supplier, SupplierBrowserSession, SupplierChannelProbeStatus, SupplierGroup, SupplierGroupStatus, SupplierKey, SupplierKeyStatus, SupplierProvisionJob, SupplierProvisionJobType, SupplierProvisionStatus, SupplierRuntimeStatus } from '@/api/admin/adminPlus'
import type { ChannelScheduleStepStatus, ChannelScheduleStepIcon, ChannelScheduleStep, ScheduleListRow } from './types'
import { ctxFn, ctxValue } from './ctxProxy'
export function attachPresentationStatus(ctx: any) {
  const sessionStore = ctxValue(ctx, 'sessionStore')
  const rowLoginSupplierID = ctxValue(ctx, 'rowLoginSupplierID')
  const currentSessionSummary = ctxValue(ctx, 'currentSessionSummary')
  const formatMoney = ctxFn(ctx, 'formatMoney')
  const formatDateTime = ctxFn(ctx, 'formatDateTime')
  const supplierCostSnapshot = ctxFn(ctx, 'supplierCostSnapshot')
  const supplierBestChannel = ctxFn(ctx, 'supplierBestChannel')
  const groupChannelCheck = ctxFn(ctx, 'groupChannelCheck')
  const isChannelCheckActionRunning = ctxFn(ctx, 'isChannelCheckActionRunning')
  const channelHasLocalBinding = ctxFn(ctx, 'channelHasLocalBinding')
  const channelIsAvailable = ctxFn(ctx, 'channelIsAvailable')
  function firstNonEmptyString(...values: Array<string | number | null | undefined>): string {
    for (const value of values) {
      const normalized = String(value ?? '').trim()
      if (normalized) return normalized
    }
    return ''
  }

  function scheduleRowRisky(row: ScheduleListRow): boolean {
    if (!row.local_account_id) return true
    if (row.health_status !== 'normal') return true
    if (row.local_account_status !== 'active') return true
    return row.probe_status !== 'available'
  }

  function shouldShowScheduleSupplierName(row: ScheduleListRow): boolean {
    const supplierName = row.supplier_name.trim().toLowerCase()
    if (!supplierName) return false
    return !row.local_account_name.trim().toLowerCase().includes(supplierName)
  }

  function channelScheduleStepClass(status: ChannelScheduleStepStatus): string {
    if (status === 'done') return 'border-emerald-200 bg-emerald-50 dark:border-emerald-800 dark:bg-emerald-900/20'
    if (status === 'warning') return 'border-amber-200 bg-amber-50 dark:border-amber-800 dark:bg-amber-900/20'
    return 'border-gray-200 bg-gray-50 dark:border-dark-700 dark:bg-dark-900/40'
  }

  function channelScheduleStepIconClass(status: ChannelScheduleStepStatus): string {
    if (status === 'done') return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200'
    if (status === 'warning') return 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200'
    return 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-dark-300'
  }

  function channelScheduleStepIconName(step: ChannelScheduleStep): ChannelScheduleStepIcon {
    if (step.status === 'done') return 'check'
    if (step.status === 'warning') return 'exclamationTriangle'
    if (step.status === 'pending') return 'clock'
    return step.icon
  }

  function groupHasLocalBinding(group: SupplierGroup): boolean {
    const check = groupChannelCheck(group.id)
    if (channelHasLocalBinding(check)) return true
    return Boolean(groupKey(group)?.local_sub2api_account_id)
  }

  function bestChannelActionLabel(supplier: Supplier): string {
    const current = supplierBestChannel(supplier.id)
    if (!current) return '一键检测'
    if (!channelHasLocalBinding(current)) return '开通账号'
    if (current.local_account_schedulable) return '暂停'
    if (!channelIsAvailable(current)) return '复测'
    return '校验加入'
  }

  function bestChannelActionIcon(supplier: Supplier): 'key' | 'ban' | 'beaker' | 'play' {
    const current = supplierBestChannel(supplier.id)
    if (!current) return 'beaker'
    if (!channelHasLocalBinding(current)) return 'key'
    if (current.local_account_schedulable) return 'ban'
    if (!channelIsAvailable(current)) return 'beaker'
    return 'play'
  }

  function bestChannelActionTitle(supplier: Supplier): string {
    const current = supplierBestChannel(supplier.id)
    if (!current) return '提交异步渠道检测任务'
    if (!channelHasLocalBinding(current)) return '先为该供应商分组开通第三方 Key 和本地 Sub2API 账号'
    if (current.local_account_schedulable) return '暂停该最佳渠道对应本地账号参与调度'
    if (!channelIsAvailable(current)) return '该渠道当前不可用或未通过真实探测，请先复测'
    return '校验本地分组绑定后，将该最佳渠道对应本地账号加入调度'
  }

  function groupScheduleActionLabel(group: SupplierGroup): string {
    const check = groupChannelCheck(group.id)
    if (!groupHasLocalBinding(group)) return '先开通'
    if (check?.local_account_schedulable) return '暂停'
    return '加入'
  }

  function groupScheduleActionIcon(group: SupplierGroup): 'key' | 'ban' | 'play' {
    const check = groupChannelCheck(group.id)
    if (!groupHasLocalBinding(group)) return 'key'
    if (check?.local_account_schedulable) return 'ban'
    return 'play'
  }

  function groupScheduleActionDisabled(group: SupplierGroup): boolean {
    if (isChannelCheckActionRunning(`schedule:${group.id}`)) return true
    const check = groupChannelCheck(group.id)
    if (check?.local_account_schedulable) return false
    return !groupHasLocalBinding(group) || !channelIsAvailable(check)
  }

  function groupScheduleActionTitle(group: SupplierGroup): string {
    const check = groupChannelCheck(group.id)
    if (!groupHasLocalBinding(group)) return '先开通第三方 Key 和本地 Sub2API 账号'
    if (check?.local_account_schedulable) return '暂停该渠道对应本地账号参与调度'
    if (!channelIsAvailable(check)) return '该渠道当前不可用或未通过真实探测，请先复测通过后再加入调度'
    return '校验本地分组绑定后，将该渠道对应本地账号加入调度'
  }

  function costDeltaLabel(supplierID: number): string {
    const snapshot = supplierCostSnapshot(supplierID)
    const delta = supplierBalanceDeltaCents(snapshot)
    if (delta === null) return '-'
    return formatMoney(delta, snapshot?.currency || 'USD')
  }

  function costDeltaClass(supplierID: number): string {
    const delta = supplierBalanceDeltaCents(supplierCostSnapshot(supplierID))
    if (delta === null || delta === 0) return 'text-emerald-600 dark:text-emerald-400'
    return 'text-rose-600 dark:text-rose-400'
  }

  function supplierBalanceLabel(supplier: Supplier): string {
    if (supplier.balance_cents <= 0) return '余额不足'
    if (!supplier.balance_updated_at) return '未读取'
    return '余额有效'
  }

  function supplierBalanceBadgeClass(supplier: Supplier): string {
    if (supplier.balance_cents <= 0) return 'badge-danger'
    if (!supplier.balance_updated_at) return 'badge-warning'
    return 'badge-success'
  }

  function supplierBalanceAmountClass(supplier: Supplier): string {
    if (supplier.balance_cents <= 0) return 'text-rose-600 dark:text-rose-300'
    if (!supplier.balance_updated_at) return 'text-amber-700 dark:text-amber-300'
    return 'text-gray-900 dark:text-gray-100'
  }

  function supplierBalanceUpdatedLabel(supplier: Supplier): string {
    if (!supplier.balance_updated_at) return '未记录余额时间'
    return formatDateTime(supplier.balance_updated_at)
  }

  function supplierSwitchStateLabel(supplier: Supplier): string {
    if (isSwitchable(supplier)) return supplier.runtime_status === 'active' ? '当前承载流量' : '可进入候选'
    if (supplier.runtime_status === 'monitor_only') return '仅监控，不切换'
    if (supplier.balance_cents <= 0) return '余额不足，不切换'
    if (supplier.health_status !== 'normal') return '健康异常，不切换'
    return '不可切换'
  }

  function supplierSwitchStateClass(supplier: Supplier): string {
    if (isSwitchable(supplier)) return 'text-emerald-700 dark:text-emerald-300'
    if (supplier.balance_cents <= 0 || supplier.health_status !== 'normal') return 'text-red-600 dark:text-red-300'
    return 'text-gray-500 dark:text-dark-400'
  }

  function sessionStatusLabel(status?: SupplierBrowserSession['status']): string {
    if (status === 'valid') return '有效'
    if (status === 'expired') return '已过期'
    return '未上报'
  }

  function sessionStatusClass(status?: SupplierBrowserSession['status']): string {
    if (status === 'valid') return 'badge-success'
    if (status === 'expired') return 'badge-warning'
    return 'badge-gray'
  }

  function sessionSourceLabel(source?: SupplierBrowserSession['session_source']): string {
    if (source === 'direct_login') return '后端直登'
    if (source === 'browser_extension') return 'Chrome 兜底'
    if (source === 'manual_import') return '手动导入'
    return '-'
  }

  function groupStatusLabel(status?: SupplierGroupStatus): string {
    if (status === 'active') return '有效'
    if (status === 'missing') return '已缺失'
    if (status === 'disabled') return '停用'
    return '未知'
  }

  function groupStatusClass(status?: SupplierGroupStatus): string {
    if (status === 'active') return 'badge-success'
    if (status === 'missing') return 'badge-warning'
    return 'badge-gray'
  }

  function supplierKeyStatusLabel(status?: SupplierKeyStatus): string {
    if (status === 'bound') return '已绑定'
    if (status === 'provisioning') return '开通中'
    if (status === 'manual_secret_required') return '待补密钥'
    if (status === 'failed') return '失败'
    if (status === 'disabled') return '停用'
    return '未知'
  }

  function supplierKeyStatusClass(status?: SupplierKeyStatus): string {
    if (status === 'bound') return 'badge-success'
    if (status === 'provisioning') return 'badge-primary'
    if (status === 'manual_secret_required') return 'badge-warning'
    if (status === 'failed') return 'badge-danger'
    return 'badge-gray'
  }

  function provisionJobTypeLabel(type?: SupplierProvisionJobType): string {
    if (type === 'sync_groups') return '同步分组'
    if (type === 'provision_group_key') return '开通单组 Key/账号'
    if (type === 'provision_all_group_keys') return '补齐全部 Key/账号'
    if (type === 'repair_binding') return '修复绑定'
    if (type === 'check_supplier_channels') return '检测供应商渠道'
    return '供应商任务'
  }

  function channelProbeStatusLabel(status?: SupplierChannelProbeStatus): string {
    if (status === 'available') return '可用'
    if (status === 'slow_first_token') return '首 token 慢'
    if (status === 'slow_total') return '总耗时慢'
    if (status === 'request_error') return '请求失败'
    if (status === 'remote_unavailable') return '远端异常'
    if (status === 'no_local_account') return '未绑定账号'
    if (status === 'probe_failed') return '检测失败'
    return '未检测'
  }

  function channelProbeStatusClass(status?: SupplierChannelProbeStatus): string {
    if (status === 'available') return 'badge-success'
    if (status === 'slow_first_token' || status === 'slow_total' || status === 'remote_unavailable') return 'badge-warning'
    if (status === 'request_error' || status === 'probe_failed') return 'badge-danger'
    return 'badge-gray'
  }

  function provisionJobStatusLabel(status?: SupplierProvisionStatus): string {
    if (status === 'queued') return '排队中'
    if (status === 'running') return '执行中'
    if (status === 'succeeded') return '已完成'
    if (status === 'partial_succeeded') return '部分完成'
    if (status === 'retryable_failed') return '等待重试'
    if (status === 'manual_required') return '需人工处理'
    if (status === 'dead') return '失败'
    if (status === 'cancelled') return '已取消'
    return '未知'
  }

  function provisionJobStatusClass(status?: SupplierProvisionStatus): string {
    if (status === 'succeeded') return 'badge-success'
    if (status === 'running' || status === 'queued') return 'badge-primary'
    if (status === 'retryable_failed' || status === 'manual_required' || status === 'partial_succeeded') return 'badge-warning'
    if (status === 'dead' || status === 'cancelled') return 'badge-danger'
    return 'badge-gray'
  }

  function workflowStepDotClass(status: SupplierProvisionStatus): string {
    if (status === 'succeeded') return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200'
    if (status === 'running' || status === 'retryable_failed') return 'bg-primary-100 text-primary-700 dark:bg-primary-900/40 dark:text-primary-200'
    if (status === 'manual_required' || status === 'dead') return 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-200'
    return 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-dark-300'
  }

  function provisionJobCaption(job: SupplierProvisionJob): string {
    if (job.error_message) return job.error_message
    if (job.status === 'succeeded') return '任务完成，列表会自动刷新。'
    if (job.status === 'retryable_failed') return `第 ${job.attempts}/${job.max_attempts} 次失败，等待重试。`
    if (job.status === 'running') return '服务端正在执行，请稍候。'
    if (job.status === 'queued') return '任务已进入队列。'
    return `步骤 ${job.succeeded_steps}/${Math.max(job.total_steps, 1)}`
  }

  function middleEllipsis(value?: string | null, maxLength = 34): string {
    const text = String(value || '').trim()
    if (text.length <= maxLength) return text
    const keep = Math.max(4, Math.floor((maxLength - 1) / 2))
    const head = text.slice(0, keep)
    const tail = text.slice(text.length - (maxLength - keep - 1))
    return `${head}…${tail}`
  }

  function splitMiddleEllipsis(value?: string | null, maxLength = 24): { head: string; tail: string; ellipsized: boolean } {
    const text = String(value || '').trim()
    if (text.length <= maxLength) {
      return { head: text, tail: '', ellipsized: false }
    }
    const headLength = Math.max(8, Math.floor((maxLength - 3) * 0.58))
    const tailLength = Math.max(6, maxLength - 3 - headLength)
    return {
      head: text.slice(0, headLength),
      tail: text.slice(text.length - tailLength),
      ellipsized: true
    }
  }

  function groupKey(group: SupplierGroup): SupplierKey | undefined {
    return ctx.supplierKeysByGroupID?.value?.get(group.id)
  }

  function groupAction(group: SupplierGroup) {
    return supplierGroupAction(group, groupKey(group))
  }

  function sessionBadgeText(supplierID: number): string {
    return sessionStatusLabel(sessionStore[supplierID]?.status)
  }

  function sessionBadgeClass(supplierID: number): string {
    return sessionStatusClass(sessionStore[supplierID]?.status)
  }

  function summaryBoolClass(key: string): string {
    return currentSessionSummary.value[key] ? 'badge-success' : 'badge-gray'
  }

  function sessionSummaryString(key: string): string {
    const value = currentSessionSummary.value[key]
    return typeof value === 'string' ? value : ''
  }

  function sessionHasAccessToken(session?: SupplierBrowserSession): boolean {
    return Boolean(session?.session_summary?.has_access_token)
  }

  function sessionHasNewAPIUserHeader(session?: SupplierBrowserSession): boolean {
    return Boolean(session?.session_summary?.has_new_api_user_header)
  }

  function supportsDirectLogin(supplier: Supplier): boolean {
    return supplier.type === 'sub2api' || supplier.type === 'new_api'
  }

  function hasConfiguredDirectLoginCredential(supplier: Supplier): boolean {
    return supplier.credential.browser_login_token_configured ||
      (supplier.credential.browser_login_username_configured && supplier.credential.browser_login_password_configured)
  }

  function needsDirectLoginCredential(supplier: Supplier): boolean {
    return supplier.credential.browser_login_enabled && !hasConfiguredDirectLoginCredential(supplier)
  }

  function shouldShowTokenBadge(supplier: Supplier): boolean {
    return supplier.credential.browser_login_token_configured || Boolean(sessionStore[supplier.id]?.session_summary)
  }

  function credentialTokenBadgeText(supplier: Supplier): string {
    const session = sessionStore[supplier.id]
    if (supplier.type === 'new_api') {
      if (sessionHasNewAPIUserHeader(session) && session?.status === 'valid') return 'Header 有效'
      if (sessionHasNewAPIUserHeader(session) && session?.status === 'expired') return 'Header 过期'
      if (session?.session_summary) return 'Header 缺失'
    }
    if (sessionHasAccessToken(session) && session?.status === 'valid') return 'Token 有效'
    if (sessionHasAccessToken(session) && session?.status === 'expired') return 'Token 过期'
    if (session?.session_summary) return 'Token 缺失'
    if (supplier.credential.browser_login_token_configured) return 'Token 未验证'
    return 'Token 未配置'
  }

  function credentialTokenBadgeClass(supplier: Supplier): string {
    const session = sessionStore[supplier.id]
    if (supplier.type === 'new_api') {
      if (sessionHasNewAPIUserHeader(session) && session?.status === 'valid') return 'badge-success'
      if (sessionHasNewAPIUserHeader(session) && session?.status === 'expired') return 'badge-warning'
      if (session?.session_summary) return 'badge-danger'
    }
    if (sessionHasAccessToken(session) && session?.status === 'valid') return 'badge-success'
    if (sessionHasAccessToken(session) && session?.status === 'expired') return 'badge-warning'
    if (session?.session_summary) return 'badge-danger'
    if (supplier.credential.browser_login_token_configured) return 'badge-primary'
    return 'badge-gray'
  }

  function credentialTokenBadgeTitle(supplier: Supplier): string {
    const session = sessionStore[supplier.id]
    if (supplier.type === 'new_api') {
      if (sessionHasNewAPIUserHeader(session) && session?.status === 'valid') return '当前 New API 会话摘要包含 New-Api-User，且会话未过期'
      if (sessionHasNewAPIUserHeader(session) && session?.status === 'expired') return '当前 New API 会话摘要包含 New-Api-User，但会话已过期，请重新一键登录或使用 Chrome 插件'
      if (session?.session_summary) return '当前 New API 会话摘要没有 New-Api-User，请重新一键登录或使用 Chrome 插件'
    }
    if (sessionHasAccessToken(session) && session?.status === 'valid') return '当前会话摘要包含 Access Token，且会话未过期'
    if (sessionHasAccessToken(session) && session?.status === 'expired') return '当前会话摘要包含 Access Token，但会话已过期，请重新一键登录或使用 Chrome 插件'
    if (session?.session_summary) return '当前会话摘要没有 Access Token，请重新一键登录或使用 Chrome 插件'
    if (supplier.credential.browser_login_token_configured) return '已保存临时 Token，但尚未形成有效供应商会话'
    return '未配置临时 Token'
  }

  function oneClickLoginTitle(supplier: Supplier): string {
    const preflightError = directLoginPreflightError(supplier)
    if (preflightError) return preflightError
    if (rowLoginSupplierID.value === supplier.id) return '正在登录'
    if (supplier.type === 'new_api') return '使用已保存凭据后端直登 New API 并读取余额'
    return '使用已保存凭据后端直登并读取余额'
  }

  function directLoginPreflightError(supplier: Supplier): string {
    if (!supportsDirectLogin(supplier)) return '当前一键登录仅支持 Sub2API 或 New API 供应商'
    if (!supplier.credential.browser_login_enabled) return '未启用登录凭据，请先编辑供应商启用并配置账号密码或临时 Token'
    if (!hasConfiguredDirectLoginCredential(supplier)) return '未配置登录账号密码或临时 Token，请先编辑供应商补齐凭据'
    if (!supplier.dashboard_url && !supplier.api_base_url) return '未配置后台地址或 API Base URL，请先编辑供应商补齐地址'
    return ''
  }

  function directLoginErrorMessage(error: unknown): string {
    const code = extractApiErrorCode(error)
    const diagnostic = errorMetadataDiagnostic(error)
    if (code && ['SUPPLIER_DIRECT_LOGIN_UPSTREAM_ORIGIN_ERROR', 'SUPPLIER_DIRECT_LOGIN_UPSTREAM_HTML', 'SUPPLIER_DIRECT_LOGIN_SETTINGS_BAD_STATUS', 'SUPPLIER_DIRECT_LOGIN_BAD_STATUS'].includes(code)) {
      return withDiagnostic('供应商站点返回源站或前置层异常，后端直登失败；请稍后重试，或使用 Chrome 插件采集会话', diagnostic)
    }
    if (code && ['LOGIN_CAPTCHA_REQUIRED', 'LOGIN_MFA_REQUIRED', 'BROWSER_FALLBACK_REQUIRED'].includes(code)) {
      return '供应商登录需要验证码、2FA 或浏览器上下文，请使用 Chrome 插件采集会话'
    }
    if (code === 'SUPPLIER_DIRECT_LOGIN_ADMIN_REQUIRED') {
      return '供应商启用了后台模式，后端直登需要供应商管理员账号'
    }
    if (code === '429') {
      return '供应商登录限流，请稍后重试'
    }
    if (code === 'SUPPLIER_BROWSER_CREDENTIAL_DECRYPT_FAILED') {
      return '已保存的供应商登录凭据无法解密，请重新编辑并保存账号密码；修复版本保存后重启不会再次失效'
    }
    if (code && ['SUPPLIER_DIRECT_LOGIN_CREDENTIAL_REQUIRED', 'SUPPLIER_BROWSER_CREDENTIAL_REQUIRED', 'SUPPLIER_BROWSER_LOGIN_DISABLED'].includes(code)) {
      return '供应商未配置可用登录凭据，请先编辑供应商补齐账号密码或临时 Token'
    }
    if (code === 'SUPPLIER_DASHBOARD_URL_REQUIRED') {
      return '供应商未配置后台地址，请先编辑供应商补齐地址'
    }
    const rawMessage = (error as { message?: string }).message || ''
    const normalizedMessage = rawMessage.toLowerCase()
    if (normalizedMessage.includes('cloudflare') || normalizedMessage.includes('origin web server') || normalizedMessage.includes('invalid or incomplete response')) {
      return withDiagnostic('供应商站点返回源站或前置层异常，后端直登失败；请稍后重试，或使用 Chrome 插件采集会话', diagnostic)
    }
    return withDiagnostic((error as { message?: string }).message || '后端直登失败', diagnostic)
  }

  function isBalanceProbeError(error: unknown): boolean {
    const code = extractApiErrorCode(error)
    const message = errorMessage(error, '').toLowerCase()
    return code === 'SUPPLIER_SESSION_PERMISSION_DENIED' ||
      message.includes('supplier session cannot access user profile') ||
      message.includes('cannot access user profile')
  }

  function normalizeBalanceErrorMessage(message?: string): string {
    const raw = String(message || '').trim()
    const lower = raw.toLowerCase()
    if (lower.includes('authorization header is required')) {
      return '供应商 profile 接口要求 Authorization，但保存的会话没有带上 access token；请在供应商仪表盘页重新上报'
    }
    if (lower.includes('invalid token') || lower.includes('invalid_auth_header')) {
      return '保存的供应商 access token 已失效或格式不正确；请刷新供应商页面后重新上报'
    }
    if (lower.includes('supplier session cannot access user profile') || lower.includes('cannot access user profile')) {
      return '供应商会话无法读取用户资料或余额，请重新采集具备 Profile 权限的会话'
    }
    return raw || '读取当前余额失败'
  }

  function errorMessage(error: unknown, fallback: string): string {
    return (error as { message?: string })?.message || fallback
  }

  function errorMetadataDiagnostic(error: unknown): string {
    const metadata = (error as { metadata?: Record<string, unknown> })?.metadata
    if (!metadata || typeof metadata !== 'object') return ''
    const keys = ['endpoint', 'status_code', 'content_type', 'body_type', 'body_excerpt']
    return keys
      .map((key) => {
        const value = metadata[key]
        return typeof value === 'string' && value.trim() ? `${key}: ${trimDiagnostic(value)}` : ''
      })
      .filter(Boolean)
      .join(' · ')
  }

  function trimDiagnostic(value: string): string {
    const text = value.replace(/\s+/g, ' ').trim()
    return text.length > 160 ? `${text.slice(0, 157)}...` : text
  }

  function withDiagnostic(message: string, diagnostic: string): string {
    return diagnostic ? `${message}（${diagnostic}）` : message
  }

  function channelStatusErrorMessage(error: unknown): string {
    const code = extractApiErrorCode(error)
    const status = (error as { status?: number })?.status
    if ((status === 404 || code === '404') && !((error as { reason?: string })?.reason)) {
      return 'Admin Plus 后端服务还没有加载供应商渠道状态路由，请确认已发布并重启到包含 /api/v1/admin-plus/suppliers/:id/channel-monitors 的版本。'
    }
    if (code && ['SUPPLIER_SESSION_NOT_FOUND', 'SUPPLIER_SESSION_EXPIRED', 'SUPPLIER_SESSION_DECRYPT_FAILED'].includes(code)) {
      return '当前供应商还没有可用会话，请先一键登录，或使用 Chrome 插件采集供应商会话后再读取渠道状态。'
    }
    if (code === 'SUPPLIER_SESSION_PERMISSION_DENIED') {
      return '当前供应商会话无权限读取渠道状态，请重新一键登录或使用 Chrome 插件采集最新会话。'
    }
    if (code === 'SUPPLIER_SESSION_BASE_URL_REQUIRED') {
      return '供应商未配置后台地址或 API Base URL，请先编辑供应商补齐地址。'
    }
    if (code && ['SUPPLIER_SESSION_API_BASE_URL_INVALID', 'SUPPLIER_SESSION_ORIGIN_INVALID', 'SUPPLIER_SESSION_URL_INVALID', 'SUPPLIER_SESSION_HOST_NOT_ALLOWED'].includes(code)) {
      return '供应商会话地址与供应商配置不匹配，请检查后台地址和 API Base URL。'
    }
    if (code && ['SUPPLIER_SESSION_REQUEST_FAILED', 'SUPPLIER_SESSION_BAD_STATUS', 'SUPPLIER_CHANNEL_MONITORS_RESPONSE_INVALID'].includes(code)) {
      return '供应商渠道状态接口返回异常，请确认 Sub2API 已部署 /api/v1/channel-monitors，或 New API 已配置可访问的 Pulse 监控入口。'
    }
    return (error as { message?: string }).message || '加载供应商渠道状态失败'
  }

  function isSwitchable(supplier: Supplier): boolean {
    return ['candidate', 'active'].includes(supplier.runtime_status) && supplier.health_status === 'normal' && supplier.balance_cents > 0
  }

  function isSwitchableRuntimeStatus(status: SupplierRuntimeStatus): boolean {
    return status === 'candidate' || status === 'active'
  }

  function hasCredential(supplier: Supplier): boolean {
    return supplier.credential.postgres_configured ||
      supplier.credential.redis_configured ||
      supplier.credential.browser_login_enabled ||
      supplier.credential.browser_login_username_configured ||
      supplier.credential.browser_login_password_configured ||
      supplier.credential.browser_login_token_configured
  }

  Object.assign(ctx, {
    firstNonEmptyString,
    scheduleRowRisky,
    shouldShowScheduleSupplierName,
    channelScheduleStepClass,
    channelScheduleStepIconClass,
    channelScheduleStepIconName,
    groupHasLocalBinding,
    bestChannelActionLabel,
    bestChannelActionIcon,
    bestChannelActionTitle,
    groupScheduleActionLabel,
    groupScheduleActionIcon,
    groupScheduleActionDisabled,
    groupScheduleActionTitle,
    costDeltaLabel,
    costDeltaClass,
    supplierBalanceLabel,
    supplierBalanceBadgeClass,
    supplierBalanceAmountClass,
    supplierBalanceUpdatedLabel,
    supplierSwitchStateLabel,
    supplierSwitchStateClass,
    sessionStatusLabel,
    sessionStatusClass,
    sessionSourceLabel,
    groupStatusLabel,
    groupStatusClass,
    supplierKeyStatusLabel,
    supplierKeyStatusClass,
    provisionJobTypeLabel,
    channelProbeStatusLabel,
    channelProbeStatusClass,
    provisionJobStatusLabel,
    provisionJobStatusClass,
    workflowStepDotClass,
    provisionJobCaption,
    middleEllipsis,
    splitMiddleEllipsis,
    groupKey,
    groupAction,
    sessionBadgeText,
    sessionBadgeClass,
    summaryBoolClass,
    sessionSummaryString,
    sessionHasAccessToken,
    sessionHasNewAPIUserHeader,
    supportsDirectLogin,
    hasConfiguredDirectLoginCredential,
    needsDirectLoginCredential,
    shouldShowTokenBadge,
    credentialTokenBadgeText,
    credentialTokenBadgeClass,
    credentialTokenBadgeTitle,
    oneClickLoginTitle,
    directLoginPreflightError,
    directLoginErrorMessage,
    isBalanceProbeError,
    normalizeBalanceErrorMessage,
    errorMessage,
    channelStatusErrorMessage,
    isSwitchable,
    isSwitchableRuntimeStatus,
    hasCredential
  })
}
