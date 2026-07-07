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

  it('汇总供应商能力时忽略不支持项', () => {
    const ctx = viewModel()

    expect(ctx.supplierCapabilitySummary(supplier({
      capabilities: [
        { key: 'runtime', label: '运行态', status: 'available', source: 'sub2api_readonly' },
        { key: 'groups', label: '分组', status: 'needs_session', source: 'provider_router' },
        { key: 'rates', label: '费率', status: 'needs_session', source: 'provider_router' },
        { key: 'local_runtime_observation', label: '本地运行态', status: 'needs_readonly_db', source: 'sub2api_readonly' },
        { key: 'announcements', label: '公告', status: 'planned', source: 'provider_router' },
        { key: 'source_keys', label: '源站 Key', status: 'unsupported', source: 'source_provider' }
      ]
    }))).toBe('1 可用 · 2 需会话 · 1 需只读库 · 1 待接入')
  })

  it('展示供应商初始化预设提示', () => {
    const ctx = viewModel()
    const current = supplier({
      integration_hint: {
        id: 'deepseek-openai',
        label: 'DeepSeek / OpenAI',
        provider_label: 'DeepSeek',
        protocol: 'openai',
        recommended_skip_model_fetch: true,
        recommended_models: ['deepseek-chat', 'deepseek-reasoner', 'deepseek-coder'],
        docs_url: 'https://api-docs.deepseek.com/'
      }
    })

    expect(ctx.supplierIntegrationProtocolLabel(current)).toBe('OpenAI')
    expect(ctx.supplierIntegrationBadgeClass(current)).toBe('badge-success')
    expect(ctx.supplierIntegrationModelsLabel(current)).toBe('deepseek-chat / deepseek-reasoner / +1')
    expect(ctx.supplierIntegrationSetupLabel(current)).toBe('API Key 优先')
    expect(ctx.supplierIntegrationTitle(current)).toContain('建议 API Key 优先初始化')
  })

  it('展示供应商 API 候选和运营建议', () => {
    const ctx = viewModel()
    const current = supplier({
      api_endpoint_candidates: [
        {
          id: 'configured_api_base',
          label: '当前 API Base',
          url: 'https://api.deepseek.com/v1',
          protocol: 'openai',
          source: 'configured',
          recommended: true
        },
        {
          id: 'deepseek-claude:/anthropic',
          label: 'DeepSeek / Claude',
          url: 'https://api.deepseek.com/anthropic',
          protocol: 'claude',
          source: 'integration_preset',
          recommended: true
        },
        {
          id: 'zhipu-coding-plan-claude:/api/anthropic',
          label: '智谱 Coding Plan / Claude',
          url: 'https://open.bigmodel.cn/api/anthropic',
          protocol: 'claude',
          source: 'manual_integration_preset',
          recommended: false
        }
      ],
      operation_hints: [
        {
          key: 'post_sync_probe',
          label: '同步后检测',
          severity: 'action',
          source: 'metapi_post_refresh_probe',
          description: '同步后建议执行渠道检测'
        }
      ]
    })

    expect(ctx.supplierVisibleEndpointCandidates(current)).toHaveLength(3)
    expect(ctx.supplierEndpointCandidateLabel(current.api_endpoint_candidates![0])).toBe('当前 API')
    expect(ctx.supplierEndpointCandidateBadgeClass(current.api_endpoint_candidates![1])).toBe('badge-purple')
    expect(ctx.supplierEndpointCandidateLabel(current.api_endpoint_candidates![2])).toBe('Claude 手动')
    expect(ctx.supplierEndpointCandidateTitle(current.api_endpoint_candidates![2])).toContain('手动候选入口')
    expect(ctx.supplierVisibleOperationHints(current)).toHaveLength(1)
    expect(ctx.supplierOperationHintBadgeClass(current.operation_hints![0])).toBe('badge-warning')
    expect(ctx.supplierOperationHintTitle(current.operation_hints![0])).toContain('metapi_post_refresh_probe')
  })

  it('展示供应商平台身份提示', () => {
    const ctx = viewModel()
    const current = supplier({
      platform_hint: {
        id: 'done-hub',
        label: 'DoneHub',
        family: 'new_api',
        source: 'url_hint',
        description: '识别为 DoneHub / New API 家族站点'
      }
    })

    expect(ctx.supplierPlatformHintBadgeClass(current)).toBe('badge-success')
    expect(ctx.supplierPlatformHintFamilyLabel(current.platform_hint)).toBe('New API 家族')
    expect(ctx.supplierPlatformHintTitle(current)).toContain('DoneHub')
    expect(ctx.supplierPlatformHintTitle(current)).toContain('来源: url_hint')
  })

  it('展示供应商 URL 形态提示', () => {
    const ctx = viewModel()
    const current = supplier({
      url_hints: [
        {
          key: 'api_base_url',
          label: '接口路径',
          url: 'https://relay.example.com/v1/chat/completions',
          source: 'api_base_url',
          action: 'endpoint_path',
          severity: 'warning',
          matched_path: '/v1/chat/completions',
          suggested_url: 'https://relay.example.com/v1',
          description: 'API Base 看起来是具体接口路径'
        }
      ]
    })

    expect(ctx.supplierVisibleURLHints(current)).toHaveLength(1)
    expect(ctx.supplierURLHintBadgeClass(current.url_hints![0])).toBe('badge-warning')
    expect(ctx.supplierURLHintTitle(current.url_hints![0])).toContain('建议: https://relay.example.com/v1')
  })

  it('展示推荐 API Base 运营提示', () => {
    const ctx = viewModel()
    const current = supplier({
      operation_hints: [
        {
          key: 'recommended_api_base',
          label: '推荐入口',
          severity: 'action',
          source: 'metapi_site_initialization_preset',
          description: '建议本地账号 API Base 使用 https://coding.dashscope.aliyuncs.com/v1。'
        }
      ]
    })

    expect(ctx.supplierVisibleOperationHints(current)).toHaveLength(1)
    expect(ctx.supplierOperationHintBadgeClass(current.operation_hints![0])).toBe('badge-warning')
    expect(ctx.supplierOperationHintTitle(current.operation_hints![0])).toContain('coding.dashscope.aliyuncs.com/v1')
  })
})
