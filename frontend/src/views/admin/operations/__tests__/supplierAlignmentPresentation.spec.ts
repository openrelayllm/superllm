import { describe, expect, it } from 'vitest'
import type { LocalSub2APIAccount, SupplierAccount, SupplierGroup, SupplierKey } from '@/api/admin/adminPlus'
import { buildSupplierAlignmentRows } from '../supplierAlignmentPresentation'

function makeGroup(overrides: Partial<SupplierGroup> = {}): SupplierGroup {
  return {
    id: 1,
    supplier_id: 9,
    external_group_id: 'provider-group-1',
    name: 'OpenAI 标准组',
    description: '',
    provider_family: 'openai',
    rate_multiplier: 1,
    effective_rate_multiplier: 1,
    allow_image_generation: false,
    is_private: false,
    key_limit_policy: 'inherit',
    key_limit_value: 0,
    active_key_count: 1,
    key_capacity_status: 'available',
    status: 'active',
    last_seen_at: '2026-07-16T00:00:00Z',
    created_at: '2026-07-16T00:00:00Z',
    updated_at: '2026-07-16T00:00:00Z',
    ...overrides
  }
}

function makeKey(overrides: Partial<SupplierKey> = {}): SupplierKey {
  return {
    id: 10,
    supplier_id: 9,
    supplier_group_id: 1,
    external_group_id: 'provider-group-1',
    external_key_id: 'provider-key-10',
    name: 'OpenAI 标准组 Key',
    status: 'bound',
    provider_family: 'openai',
    local_sub2api_account_id: 100,
    created_at: '2026-07-16T00:00:00Z',
    updated_at: '2026-07-16T00:00:00Z',
    ...overrides
  }
}

function makeBinding(overrides: Partial<SupplierAccount> = {}): SupplierAccount {
  return {
    id: 20,
    supplier_id: 9,
    supplier_key_id: 10,
    local_sub2api_account_id: 100,
    local_account_name: 'OpenAI 标准组 Key',
    local_account_platform: 'openai',
    local_account_type: 'api_key',
    supplier_group_id: 1,
    configured_concurrency: 10,
    observed_max_concurrency: 0,
    balance_threshold_cents: 0,
    balance_cents: 1000,
    balance_currency: 'USD',
    has_usable_balance: true,
    runtime_status: 'active',
    health_status: 'normal',
    created_at: '2026-07-16T00:00:00Z',
    updated_at: '2026-07-16T00:00:00Z',
    ...overrides
  }
}

function makeLocalAccount(overrides: Partial<LocalSub2APIAccount> = {}): LocalSub2APIAccount {
  return {
    id: 100,
    name: 'OpenAI 标准组 Key',
    platform: 'openai',
    type: 'api_key',
    status: 'active',
    schedulable: true,
    concurrency: 10,
    priority: 100,
    rate_multiplier: 1,
    auto_pause_on_expired: true,
    created_at: '2026-07-16T00:00:00Z',
    updated_at: '2026-07-16T00:00:00Z',
    ...overrides
  }
}

describe('buildSupplierAlignmentRows', () => {
  it('仅在分组、Key、绑定记录和本地账号全部存在时判定为已对齐', () => {
    const rows = buildSupplierAlignmentRows([makeGroup()], [makeKey()], [makeBinding()], [makeLocalAccount()])

    expect(rows).toHaveLength(1)
    expect(rows[0].alignmentStatus).toBe('aligned')
  })

  it('有效分组没有 Key 时直接显示缺少第三方 Key', () => {
    const rows = buildSupplierAlignmentRows([makeGroup()], [], [], [])

    expect(rows[0].alignmentStatus).toBe('missing_key')
    expect(rows[0].canCreate).toBe(true)
  })

  it('Key 指向本地账号但没有供应商绑定记录时不判定为已对齐', () => {
    const rows = buildSupplierAlignmentRows([makeGroup()], [makeKey()], [], [makeLocalAccount()])

    expect(rows[0].alignmentStatus).toBe('unbound_key')
    expect(rows[0].canRepair).toBe(true)
  })

  it('绑定记录指向另一个本地账号时显示绑定错误', () => {
    const rows = buildSupplierAlignmentRows(
      [makeGroup()],
      [makeKey()],
      [makeBinding({ local_sub2api_account_id: 101 })],
      [makeLocalAccount(), makeLocalAccount({ id: 101 })]
    )

    expect(rows.map((row) => row.alignmentStatus)).toContain('binding_mismatch')
  })

  it('绑定记录指向已删除的本地账号时显示缺少本地账号', () => {
    const rows = buildSupplierAlignmentRows([makeGroup()], [makeKey()], [makeBinding()], [])

    expect(rows[0].alignmentStatus).toBe('missing_local_account')
  })

  it('本地账号已停用时单独显示账号状态问题', () => {
    const rows = buildSupplierAlignmentRows([makeGroup()], [makeKey()], [makeBinding()], [makeLocalAccount({ status: 'disabled' })])

    expect(rows[0].alignmentStatus).toBe('local_account_disabled')
    expect(rows[0].canManageLocal).toBe(true)
  })

  it('保留同一分组下的全部 Key 行', () => {
    const rows = buildSupplierAlignmentRows(
      [makeGroup()],
      [makeKey(), makeKey({ id: 11, external_key_id: 'provider-key-11', local_sub2api_account_id: 101 })],
      [makeBinding(), makeBinding({ id: 21, supplier_key_id: 11, local_sub2api_account_id: 101 })],
      [makeLocalAccount(), makeLocalAccount({ id: 101 })]
    )

    expect(rows).toHaveLength(2)
    expect(rows.every((row) => row.alignmentStatus === 'aligned')).toBe(true)
  })

  it('保留找不到分组的孤立绑定记录且不提供修复动作', () => {
    const rows = buildSupplierAlignmentRows([], [], [makeBinding()], [makeLocalAccount()])

    expect(rows).toHaveLength(1)
    expect(rows[0]).toMatchObject({
      alignmentStatus: 'group_missing',
      canCreate: false,
      canRepair: false
    })
  })

  it.each([
    ['manual_secret_required', 'key_manual_required', true],
    ['failed', 'key_failed', true],
    ['disabled', 'key_disabled', false]
  ] as const)('Key 状态为 %s 时映射为 %s', (keyStatus, alignmentStatus, canRepair) => {
    const rows = buildSupplierAlignmentRows([makeGroup()], [makeKey({ status: keyStatus })], [], [])

    expect(rows[0].alignmentStatus).toBe(alignmentStatus)
    expect(rows[0].canRepair).toBe(canRepair)
  })

  it('分组停用时优先显示分组状态并禁止创建 Key', () => {
    const rows = buildSupplierAlignmentRows([makeGroup({ status: 'disabled' })], [], [], [])

    expect(rows[0].alignmentStatus).toBe('group_disabled')
    expect(rows[0].canCreate).toBe(false)
  })

  it('Key 引用已删除分组时显示分组已删除', () => {
    const rows = buildSupplierAlignmentRows([], [makeKey()], [], [])

    expect(rows[0].alignmentStatus).toBe('group_missing')
  })
})
