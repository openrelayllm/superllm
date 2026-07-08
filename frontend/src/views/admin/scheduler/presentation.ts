import type {
  ExtensionTaskType,
  SchedulerAttemptRecord,
  SchedulerCandidateSummary,
  SchedulerPlan,
  SchedulerStepRecord
} from '@/api/admin/adminPlus'

export const schedulerWizardSteps = [
  '选择任务类型和成本级别',
  '选择供应商范围',
  '设置频率、窗口和 misfire 策略',
  '设置并发、重试、退避和预算',
  '预览影响范围',
  '确认启用'
]

export const schedulerTabs = [
  { value: 'dashboard', label: '工作台' },
  { value: 'plans', label: '计划配置' },
  { value: 'runs', label: '运行记录' },
  { value: 'suppliers', label: '供应商自动化' },
  { value: 'actions', label: '智能动作' },
  { value: 'settings', label: '全局设置' }
] as const

export function planManualTaskTypes(plan: SchedulerPlan): ExtensionTaskType[] {
  if (Array.isArray(plan.task_types) && plan.task_types.length > 0) {
    return plan.task_types.filter(Boolean) as ExtensionTaskType[]
  }
  return planTaskTypes(plan.task_type)
}

export function planTaskTypes(taskType: string): ExtensionTaskType[] {
  return {
    'supplier.balance.sync': ['fetch_balance'],
    'supplier.groups.sync': ['fetch_groups'],
    'supplier.rates.sync': ['fetch_rates'],
    'supplier.usage_costs.sync': ['fetch_usage_costs'],
    'supplier.costs.reconcile': ['reconcile_supplier_costs'],
    'supplier.session.probe': ['fetch_health'],
    'supplier.channels.check': ['check_supplier_channels'],
    'local.sub2api.routing.capacity_watch': ['routing_capacity_watch']
  }[taskType] as ExtensionTaskType[] || []
}

export function taskLabel(value: string): string {
  return {
    'supplier.balance.sync': '余额同步',
    'supplier.groups.sync': '分组同步',
    'supplier.rates.sync': '倍率同步',
    'supplier.recharge_rate.sync': '充值倍率',
    'supplier.funding_orders.sync': '充值账单',
    'supplier.redeem_orders.sync': '兑换账单',
    'supplier.usage_costs.sync': '用量消耗',
    'supplier.session.probe': '会话探测',
    'supplier.channels.check': '渠道检测',
    'supplier.costs.reconcile': '成本对账',
    'local.sub2api.routing.capacity_watch': '本地容量巡检',
    'local.sub2api.schedule.ensure': '加入本地调度',
    'local.sub2api.schedule.remove_invalid': '移除失效调度',
    fetch_balance: '余额同步',
    fetch_groups: '分组同步',
    fetch_rates: '倍率同步',
    fetch_usage_costs: '用量消耗',
    reconcile_supplier_costs: '成本对账',
    fetch_health: '会话探测',
    check_supplier_channels: '渠道检测',
    routing_capacity_watch: '本地容量巡检',
    capture_supplier_session: '会话直登',
    mixed: '混合任务'
  }[value] || value
}

export function runStatusLabel(value: string): string {
  return {
    succeeded: '成功',
    partial_succeeded: '部分成功',
    retryable_failed: '可重试失败',
    manual_required: '需人工处理',
    dead: '失败终止',
    queued: '排队',
    running: '运行中',
    skipped: '已跳过',
    cancelled: '已取消'
  }[value] || value
}

export function runStatusClass(value: string): string {
  if (value === 'succeeded') return 'badge-success'
  if (value === 'partial_succeeded' || value === 'manual_required') return 'badge-warning'
  if (value === 'retryable_failed' || value === 'dead') return 'badge-danger'
  return 'badge-gray'
}

export function planStatusLabel(value: string): string {
  return {
    enabled: '已启用',
    paused: '已暂停',
    disabled: '已停用'
  }[value] || value
}

export function planStatusClass(value: string): string {
  if (value === 'enabled') return 'badge-success'
  if (value === 'paused') return 'badge-warning'
  return 'badge-gray'
}

export function severityLabel(value: string): string {
  return {
    critical: '严重',
    warning: '警告',
    info: '提示'
  }[value] || value
}

export function severityClass(value: string): string {
  if (value === 'critical') return 'badge-danger'
  if (value === 'warning') return 'badge-warning'
  return 'badge-gray'
}

export function actionStatusLabel(value: string): string {
  return {
    open: '待处理',
    investigating: '处理中',
    ready_to_execute: '待执行',
    executing: '执行中',
    verifying: '验证中',
    resolved: '已处理',
    ignored: '已忽略'
  }[value] || value
}

export function statusClass(value?: string): string {
  if (value === 'running') return 'text-emerald-600 dark:text-emerald-400'
  if (value === 'paused') return 'text-amber-600 dark:text-amber-400'
  return 'text-rose-600 dark:text-rose-400'
}

export function statusBadgeClass(value: string): string {
  if (['ready', 'ok', 'enabled'].includes(value)) return 'badge-success'
  if (['failed', 'empty', 'missing', 'missing_url', 'paused'].includes(value)) return 'badge-warning'
  if (['skipped', 'manual', 'not_checked'].includes(value)) return 'badge-gray'
  return 'badge-gray'
}

export function statusValueLabel(value: string): string {
  return {
    ready: '就绪',
    ok: '正常',
    failed: '失败',
    empty: '无余额',
    missing: '缺失',
    missing_url: '缺地址',
    skipped: '跳过',
    manual: '手动',
    not_checked: '未检测',
    paused: '暂停'
  }[value] || value
}

export function candidateStatusLabel(value?: string): string {
  return {
    available: '可调度候选',
    unknown: '待确认',
    degraded: '质量降级',
    needs_provisioning: '待开通',
    balance_blocked: '余额阻断',
    blocked: '不可用',
    local_blocked: '本地阻断',
    capacity_blocked: '配额阻断'
  }[value || ''] || value || '-'
}

export function candidateStatusClass(value?: string): string {
  if (value === 'available') return 'badge-success'
  if (value === 'balance_blocked' || value === 'capacity_blocked' || value === 'blocked' || value === 'local_blocked') return 'badge-danger'
  if (value === 'needs_provisioning' || value === 'unknown' || value === 'degraded') return 'badge-warning'
  return 'badge-gray'
}

export function candidateCheckSourceLabel(value?: string): string {
  return {
    supplier: '供应商',
    supplier_group: '第三方分组',
    supplier_key: '第三方 Key',
    key_capacity: 'Key 配额',
    balance: '余额',
    local_state: '本地状态',
    channel_monitor: '通道监控',
    active_probe: '实测',
    candidate_evaluator: '候选评估'
  }[value || ''] || value || '-'
}

export function candidateReasonLabel(value?: string): string {
  return {
    recharge_required: '余额不足，充值后重测',
    balance_unknown: '余额未知，需确认',
    channel_monitor_failed: '通道监控不可用',
    channel_active_probe_failed: '实测失败',
    channel_untested: '尚未检测通道',
    supplier_binding_missing: '缺少供应商绑定',
    supplier_disabled: '供应商已禁用',
    supplier_unavailable: '供应商不可用',
    supplier_credential_invalid: '供应商凭据失效',
    supplier_paused: '供应商已暂停',
    supplier_account_disabled: '供应商账号禁用',
    supplier_group_missing: '第三方分组缺失',
    supplier_group_disabled: '第三方分组禁用',
    supplier_key_missing: '缺少第三方 Key',
    supplier_key_failed: '第三方 Key 失败',
    supplier_key_disabled: '第三方 Key 禁用',
    supplier_key_manual_secret_required: '第三方 Key 需补密钥',
    supplier_key_provisioning: '第三方 Key 创建中',
    key_capacity_exhausted: 'Key 配额已满',
    local_account_missing: '缺少本地账号',
    local_account_unschedulable: '本地账号已关调度',
    local_account_temp_unschedulable: '本地账号临时不可调度',
    local_account_state_drift: '原后台变更待处理',
    local_account_metadata_drift: '本地账号元数据漂移',
    key_local_account_mismatch: 'Key 绑定账号不一致',
    rate_missing: '倍率缺失',
    candidate_unavailable: '候选池暂无可用账号',
    candidate_missing: '候选池未同步'
  }[value || ''] || value || '-'
}

export function candidateCountsLabel(summary?: SchedulerCandidateSummary | null): string {
  if (!summary) return '-'
  const blocked = (summary.blocked_count || 0) + (summary.balance_blocked_count || 0) + (summary.capacity_blocked_count || 0)
  return `可用 ${summary.available_count || 0} / 阻断 ${blocked} / 未知 ${summary.unknown_count || 0}`
}

export function candidateRateLabel(value?: number | null): string {
  if (value === undefined || value === null || !Number.isFinite(Number(value)) || Number(value) <= 0) return '-'
  return `${Number(value).toFixed(4).replace(/0+$/, '').replace(/\.$/, '')}x`
}

export function moneyLabel(cents: number, currency: string): string {
  const amount = cents / 100
  const normalizedCurrency = currency || 'USD'
  if (normalizedCurrency === 'USD') return `$${amount.toFixed(2)}`
  return `${amount.toFixed(2)} ${normalizedCurrency}`
}

export function formatDateTime(value?: string | null): string {
  if (!value) return ''
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '' : date.toLocaleString()
}

export function stepRetryable(status: string): boolean {
  return ['retryable_failed', 'manual_required', 'dead', 'skipped', 'cancelled'].includes(status)
}

export function stepCancellable(status: string): boolean {
  return ['queued', 'running', 'retryable_failed', 'manual_required'].includes(status)
}

export function runCancellable(status: string): boolean {
  return ['queued', 'running', 'retryable_failed', 'partial_succeeded', 'manual_required'].includes(status)
}

export function runDeletable(status: string): boolean {
  return !['queued', 'running'].includes(status)
}

export function runRetryable(status: string, failedSteps: number): boolean {
  return failedSteps > 0 || ['retryable_failed', 'partial_succeeded', 'manual_required', 'dead', 'skipped', 'cancelled'].includes(status)
}

export function stepHasDiagnostics(step?: SchedulerStepRecord | null): boolean {
  if (!step) return false
  const issueStatus = ['retryable_failed', 'manual_required', 'dead', 'skipped', 'cancelled'].includes(step.status)
  const issueAttempt = step.operation_logs?.some((log) => log.status !== 'succeeded' || log.error_code || log.error_message)
  return Boolean(step.reason || issueStatus || issueAttempt)
}

export interface StepFailureReason {
  stage?: string
  code?: string
  message?: string
  action?: string
  outcome?: string
  login_code?: string
  login_message?: string
  suggestion?: string
  raw_error?: string
  metadata?: Record<string, unknown>
}

export function parseStepReason(reason?: string): StepFailureReason {
  if (!reason) return {}
  try {
    const parsed = JSON.parse(reason)
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      return parsed as StepFailureReason
    }
  } catch {
    // 兼容旧版本纯文本失败原因。
  }
  return {}
}

export function stepReasonSummary(reason?: string): string {
  const parsed = parseStepReason(reason)
  return firstText(parsed.login_code, parsed.code, parsed.login_message, parsed.message, codeFromReasonText(reason || ''), plainStepReason(reason || ''), '-')
}

export function latestStepAttempt(step?: SchedulerStepRecord | null): SchedulerAttemptRecord | null {
  const logs = step?.operation_logs || []
  if (logs.length === 0) return null
  return logs.reduce((latest, current) => {
    if (current.attempt_no !== latest.attempt_no) return current.attempt_no > latest.attempt_no ? current : latest
    return current.id > latest.id ? current : latest
  })
}

export function stepDiagnosticSummary(step?: SchedulerStepRecord | null): string {
  if (!step) return '-'
  const latest = latestStepAttempt(step)
  return firstText(
    step.reason ? stepReasonSummary(step.reason) : '',
    latest?.error_code,
    latest?.error_message,
    codeFromReasonText(latest?.error_message || ''),
    runStatusLabel(step.status),
    '-'
  )
}

export function stepRawDiagnostics(step?: SchedulerStepRecord | null): string {
  if (!step) return '-'
  const latest = latestStepAttempt(step)
  return firstText(
    step.reason,
    latest?.error_message,
    latest?.error_code,
    step.result_snapshot ? formatReasonSnapshot(step.result_snapshot) : '',
    runStatusLabel(step.status),
    '-'
  )
}

export function plainStepReason(reason: string): string {
  const parsed = parseStepReason(reason)
  return firstText(parsed.login_message, parsed.message, parsed.raw_error, reason)
}

export function codeFromReasonText(reason: string): string {
  const upper = reason.toUpperCase()
  const knownCodes = [
    'SUPPLIER_SESSION_NOT_FOUND',
    'SUPPLIER_SESSION_EXPIRED',
    'SUPPLIER_SESSION_DECRYPT_FAILED',
    'SUPPLIER_SESSION_PERMISSION_DENIED',
    'SUPPLIER_NEW_API_ADMIN_SESSION_REQUIRED',
    'SUPPLIER_SESSION_PROBE_FAILED',
    'SUPPLIER_SESSION_PROBE_HTML',
    'SUPPLIER_SESSION_PROBE_BAD_STATUS',
    'SUPPLIER_SESSION_PROFILE_INVALID',
    'SUPPLIER_DIRECT_LOGIN_API_BASE_URL_REQUIRED',
    'SUPPLIER_DIRECT_LOGIN_CREDENTIAL_REQUIRED',
    'SUPPLIER_DIRECT_LOGIN_ADMIN_REQUIRED',
    'SUPPLIER_DIRECT_LOGIN_FAILED',
    'SUPPLIER_DIRECT_LOGIN_RESPONSE_INVALID',
    'SUPPLIER_DIRECT_LOGIN_SETTINGS_FAILED',
    'SUPPLIER_DIRECT_LOGIN_SETTINGS_BAD_STATUS',
    'SUPPLIER_DIRECT_LOGIN_SETTINGS_INVALID',
    'SUPPLIER_DIRECT_LOGIN_UPSTREAM_HTML',
    'SUPPLIER_DIRECT_LOGIN_UPSTREAM_ORIGIN_ERROR',
    'SUPPLIER_DIRECT_LOGIN_BAD_STATUS',
    'SUPPLIER_DIRECT_LOGIN_EMPTY_SESSION',
    'SUPPLIER_DIRECT_LOGIN_UNSUPPORTED',
    'SUPPLIER_FUNDING_CAPABILITY_MISSING',
    'SUPPLIER_ENTITLEMENT_CAPABILITY_MISSING',
    'USAGE_COST_LINES_TOO_MANY',
    'LOGIN_CREDENTIAL_INVALID',
    'LOGIN_CAPTCHA_REQUIRED',
    'LOGIN_MFA_REQUIRED',
    'BROWSER_FALLBACK_REQUIRED',
    'BROWSER_CHALLENGE_REQUIRED',
    'PASSWORD_LOGIN_DISABLED',
    'SCHEDULER_STEP_FAILED'
  ]
  const known = knownCodes.find((code) => upper.includes(code))
  if (known) return known
  return upper.match(/\b(?:SUPPLIER|LOGIN|BROWSER|PASSWORD|SCHEDULER)_[A-Z0-9_]+\b/)?.[0] || ''
}

export function stageLabel(value?: string): string {
  return {
    session_precheck: '会话预检',
    session_refresh: '自动登录',
    session_refresh_after_sync: '采集后会话刷新',
    supplier_groups_sync: '分组同步',
    supplier_rates_sync: '倍率同步',
    supplier_balance_sync: '余额同步',
    supplier_usage_costs_sync: '用量对账',
    supplier_costs_reconcile: '成本对账',
    supplier_health_sync: '健康检测',
    supplier_channel_check: '渠道检测'
  }[value || ''] || value || '-'
}

export function actionLabel(value?: string): string {
  return {
    direct_login: '自动登录',
    sync_groups: '同步分组',
    sync_rates: '同步倍率',
    sync_balance: '同步余额',
    sync_usage_costs: '同步用量',
    sync_costs: '同步成本',
    sync_health: '健康检测',
    check_channels: '检测渠道',
    sync: '同步'
  }[value || ''] || value || '-'
}

export function outcomeLabel(value?: string): string {
  return {
    skipped: '已跳过',
    failed: '失败',
    manual_required: '需人工处理'
  }[value || ''] || value || '-'
}

export function suggestionFromCode(code?: string): string {
  return {
    SUPPLIER_SESSION_NOT_FOUND: '当前没有可用会话，请配置登录凭据后重试，或使用插件采集会话。',
    SUPPLIER_SESSION_EXPIRED: '当前会话已过期，请重新登录或使用插件刷新会话。',
    SUPPLIER_SESSION_DECRYPT_FAILED: '会话解密失败，请重新一键登录或使用插件采集会话。',
    SUPPLIER_SESSION_PERMISSION_DENIED: '当前注册用户会话权限不足，请确认账号权限或换用具备该接口权限的账号。',
    SUPPLIER_NEW_API_ADMIN_SESSION_REQUIRED: 'new-api 历史接口需要更高数据权限，请确认注册用户具备该接口权限，或换用可读取该数据的账号/token。',
    SUPPLIER_SESSION_PROBE_FAILED: '供应商接口超时或不可达，请检查供应商地址、网络出口和前置防护后重试。',
    SUPPLIER_SESSION_PROBE_HTML: '供应商 profile 接口返回 HTML，通常是 Cloudflare/Nginx/风控页面，请检查前置层策略。',
    SUPPLIER_SESSION_PROBE_BAD_STATUS: '供应商 profile 接口返回非成功状态，请检查会话权限和供应商接口。',
    SUPPLIER_SESSION_PROFILE_INVALID: '供应商 profile 返回结构异常，请检查供应商程序版本和接口兼容性。',
    SUPPLIER_DIRECT_LOGIN_API_BASE_URL_REQUIRED: '补充供应商 API 地址后重试。',
    SUPPLIER_DIRECT_LOGIN_CREDENTIAL_REQUIRED: '补充供应商登录账号密码或 access token 后重试。',
    SUPPLIER_DIRECT_LOGIN_ADMIN_REQUIRED: '当前账号无权完成供应商后台直登，请换用具备对应接口权限的账号/token 后重试。',
    SUPPLIER_DIRECT_LOGIN_UPSTREAM_HTML: '供应商登录接口返回 HTML，通常是前置层或风控页面，请改用浏览器会话或调整防护策略。',
    SUPPLIER_DIRECT_LOGIN_UPSTREAM_ORIGIN_ERROR: '供应商前置层或源站返回异常，请检查 Cloudflare/Nginx/源站健康。',
    SUPPLIER_DIRECT_LOGIN_BAD_STATUS: '供应商登录接口返回非成功状态，请检查登录地址和凭据。',
    SUPPLIER_DIRECT_LOGIN_EMPTY_SESSION: '供应商登录没有返回有效会话，请改用浏览器会话或检查上游登录接口。',
    SUPPLIER_FUNDING_CAPABILITY_MISSING: '该供应商不支持读取充值订单，当前版本会降级跳过这项并继续采集余额和用量。',
    SUPPLIER_ENTITLEMENT_CAPABILITY_MISSING: '该供应商不支持读取兑换记录，当前版本会降级跳过这项并继续采集余额和用量。',
    USAGE_COST_LINES_TOO_MANY: '用量明细超过单批导入上限。当前版本已改为后台分批导入，重试该 step 或重新提交历史回补即可继续。',
    LOGIN_CREDENTIAL_INVALID: '供应商登录凭据无效，请更新账号密码或 token 后重试。',
    LOGIN_CAPTCHA_REQUIRED: '供应商要求验证码，请使用一键登录或插件采集会话后重试。',
    LOGIN_MFA_REQUIRED: '供应商要求二次验证，请人工完成登录或使用插件采集会话。',
    BROWSER_FALLBACK_REQUIRED: '供应商要求浏览器验证，请使用一键登录或插件采集会话。',
    BROWSER_CHALLENGE_REQUIRED: '供应商要求浏览器验证，请使用一键登录或插件采集会话。',
    PASSWORD_LOGIN_DISABLED: '供应商关闭密码登录，请改用 token 或插件采集会话。'
  }[code || ''] || '查看供应商地址、登录凭据和上游防护策略后重试。'
}

export function metadataSummary(metadata?: Record<string, unknown>): string {
  if (!metadata) return ''
  return Object.entries(metadata)
    .filter(([, value]) => value !== undefined && value !== null && String(value).trim())
    .map(([key, value]) => `${key}: ${String(value)}`)
    .join(' · ')
}

export function formatReasonSnapshot(value?: unknown): string {
  if (!value) return ''
  try {
    return JSON.stringify(value, null, 2)
  } catch {
    return String(value)
  }
}

export function firstText(...values: Array<string | undefined | null>): string {
  return values.find((value) => typeof value === 'string' && value.trim())?.trim() || ''
}
