import type { ExtensionTaskType, SchedulerPlan } from '@/api/admin/adminPlus'

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
    'supplier.channels.check': ['check_supplier_channels']
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
    'local.sub2api.schedule.ensure': '加入本地调度',
    'local.sub2api.schedule.remove_invalid': '移除失效调度',
    fetch_balance: '余额同步',
    fetch_groups: '分组同步',
    fetch_rates: '倍率同步',
    fetch_usage_costs: '用量消耗',
    reconcile_supplier_costs: '成本对账',
    fetch_health: '会话探测',
    check_supplier_channels: '渠道检测',
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

export function runRetryable(status: string, failedSteps: number): boolean {
  return failedSteps > 0 || ['retryable_failed', 'partial_succeeded', 'manual_required', 'dead', 'skipped', 'cancelled'].includes(status)
}
