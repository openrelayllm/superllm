import { describe, expect, it } from 'vitest'
import { attachPresentationStatus } from '../suppliers/presentationStatus'
import type { Supplier, SupplierChannelCheckSnapshot } from '@/api/admin/adminPlus'

function supplier(overrides: Partial<Supplier> = {}): Supplier {
  return {
    id: 38,
    name: '登录 - 何意味',
    kind: 'relay',
    type: 'sub2api',
    runtime_status: 'monitor_only',
    health_status: 'normal',
    dashboard_url: '',
    api_base_url: '',
    contact: '',
    notes: '',
    balance_cents: 1149,
    balance_currency: 'USD',
    created_at: '2026-06-26T00:00:00Z',
    updated_at: '2026-06-26T00:00:00Z',
    credential: {
      admin_api_key_configured: false,
      postgres_configured: false,
      redis_configured: false,
      browser_login_enabled: true,
      browser_login_username_configured: true,
      browser_login_password_configured: true,
      browser_login_token_configured: false
    },
    ...overrides
  }
}

function snapshot(overrides: Partial<SupplierChannelCheckSnapshot> = {}): SupplierChannelCheckSnapshot {
  return {
    id: 10,
    supplier_id: 38,
    supplier_group_id: 88,
    supplier_key_id: 0,
    supplier_account_id: 0,
    local_sub2api_account_id: 0,
    external_group_id: '88',
    group_name: '小米 private',
    provider_family: 'openai',
    channel_monitor_id: 0,
    channel_name: '',
    channel_provider: '',
    primary_model: '',
    remote_status: 'unknown',
    probe_model: '',
    probe_status: 'available',
    recommended: true,
    effective_rate_multiplier: 0.005,
    first_token_ms: 0,
    duration_ms: 0,
    status_code: 0,
    local_account_schedulable: false,
    captured_at: '2026-06-26T00:00:00Z',
    created_at: '2026-06-26T00:00:00Z',
    ...overrides
  }
}

function viewModel(check?: SupplierChannelCheckSnapshot) {
  const ctx: any = {
    sessionStore: {},
    rowLoginSupplierID: { value: null },
    currentSessionSummary: { value: undefined },
    formatMoney: () => '$0.00',
    formatDateTime: () => '-',
    supplierCostSnapshot: () => undefined,
    supplierBestChannel: () => check,
    groupChannelCheck: () => undefined,
    isChannelCheckActionRunning: () => false,
    channelHasLocalBinding: (item?: SupplierChannelCheckSnapshot) => Boolean(item?.supplier_account_id && item.local_sub2api_account_id),
    channelIsAvailable: (item?: SupplierChannelCheckSnapshot) => item?.probe_status === 'available'
  }
  attachPresentationStatus(ctx)
  return ctx
}

describe('supplier best channel actions', () => {
  it('未绑定本地账号时隐藏测速入口并保留开通主动作', () => {
    const ctx = viewModel(snapshot())

    expect(ctx.bestChannelProbeVisible(supplier())).toBe(false)
    expect(ctx.bestChannelActionLabel(supplier())).toBe('开通账号')
  })

  it('已绑定本地账号后显示测速入口', () => {
    const ctx = viewModel(snapshot({ supplier_account_id: 100, local_sub2api_account_id: 200 }))

    expect(ctx.bestChannelProbeVisible(supplier())).toBe(true)
    expect(ctx.bestChannelActionLabel(supplier())).toBe('校验加入')
  })
})
