import type { RoutingFailureRequest, RoutingGroupAvailability, RoutingImpactedAPIKey } from '@/api/admin/adminPlus'

export function routingRefillMultiplierLabel(value?: number | null): string {
  const multiplier = Number(value)
  if (!Number.isFinite(multiplier)) return '-'
  return `${multiplier.toFixed(4).replace(/0+$/, '').replace(/\.$/, '')}x`
}

export function routingRefillSkippedReasonLabel(value?: string): string {
  return {
    group_has_schedulable_accounts: '目标分组已有可调度账号',
    group_recovered_before_write: '写回前目标分组已恢复',
    candidate_not_found: '没有可补入的可用候选',
    candidate_suppressed_after_failure: '候选近期失败，已临时抑制',
    refill_locked: '同一分组正在补池',
    refill_cooldown: '同一分组补池冷却中',
    refill_confirmation_required: '需要先完成补池预览确认'
  }[value || ''] || value || '-'
}

export function routingRefillRunStatusLabel(value?: string): string {
  return {
    previewed: '已预览',
    succeeded: '已补入',
    skipped: '已跳过',
    failed: '失败'
  }[value || ''] || value || '-'
}

export function routingRefillRunStatusClass(value?: string): string {
  if (value === 'succeeded') return 'badge-success'
  if (value === 'failed') return 'badge-danger'
  if (value === 'skipped') return 'badge-warning'
  return 'badge-gray'
}

export function routingImpactedAPIKeyLabel(key: RoutingImpactedAPIKey): string {
  const name = key.name || `Key #${key.id}`
  const preview = key.key_preview ? ` ${key.key_preview}` : ''
  return `${name}${preview} · 用户 #${key.user_id}`
}

export function routingImpactSummaryLabel(availability?: RoutingGroupAvailability): string {
  if (!availability) return ''
  const hours = Math.max(1, Math.round((availability.recent_window_seconds || 86400) / 3600))
  return `近${hours}h 成功 ${formatCount(availability.recent_success_request_count)} · 错误 ${formatCount(availability.recent_error_request_count)} · 429 ${formatCount(availability.recent_upstream_429_count)} · Token ${formatCount(availability.recent_token_count)}`
}

export function routingImpactedAPIKeyRecentLabel(key: RoutingImpactedAPIKey): string {
  return `24h 成功 ${formatCount(key.recent_success_request_count)} / 错误 ${formatCount(key.recent_error_request_count)} / 429 ${formatCount(key.recent_upstream_429_count)}`
}

export function routingImpactedAPIKeyBadgeLabel(key: RoutingImpactedAPIKey): string {
  return `${routingImpactedAPIKeyLabel(key)} · ${routingImpactedAPIKeyRecentLabel(key)}`
}

export function routingImpactedAPIKeyDetailLabel(key: RoutingImpactedAPIKey): string {
  const tokens = formatCount(key.recent_token_count)
  return `${routingImpactedAPIKeyBadgeLabel(key)} · Token ${tokens}`
}

export function routingImpactedAPIKeyBadgeClass(key: RoutingImpactedAPIKey): string {
  if ((key.recent_upstream_429_count || 0) > 0) return 'badge-danger'
  if ((key.recent_error_request_count || 0) > 0) return 'badge-warning'
  if ((key.recent_success_request_count || 0) > 0) return 'badge-success'
  return 'badge-gray'
}

export function routingFailureRequestLabel(request: RoutingFailureRequest): string {
  const status = routingFailureRequestStatusLabel(request)
  const key = request.api_key_name || (request.api_key_id ? `Key #${request.api_key_id}` : '未知 Key')
  const account = request.account_id ? `账号 #${request.account_id}` : '未路由账号'
  const model = request.model || '-'
  return `${status} · ${key} · ${account} · ${model}`
}

export function routingFailureRequestDetailLabel(request: RoutingFailureRequest): string {
  const preview = request.api_key_preview ? ` ${request.api_key_preview}` : ''
  const error = request.error_message || request.error_type || '-'
  const requestID = request.request_id ? ` · ${request.request_id}` : ''
  return `${routingFailureRequestLabel(request)}${preview}${requestID} · ${error}`
}

export function routingFailureRequestStatusLabel(request: RoutingFailureRequest): string {
  const upstream = Number(request.upstream_status_code || 0)
  const client = Number(request.status_code || 0)
  if (upstream > 0 && upstream !== client) return `${client || '-'} / 上游 ${upstream}`
  return `${client || '-'}`
}

export function routingFailureRequestBadgeClass(request: RoutingFailureRequest): string {
  const status = Number(request.upstream_status_code || request.status_code || 0)
  if (status === 429) return 'badge-danger'
  if (status >= 500) return 'badge-danger'
  return 'badge-warning'
}

function formatCount(value?: number | null): string {
  const number = Number(value || 0)
  if (!Number.isFinite(number)) return '0'
  return Math.trunc(number).toLocaleString('zh-CN')
}
