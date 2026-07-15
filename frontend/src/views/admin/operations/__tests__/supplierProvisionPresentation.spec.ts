import { describe, expect, it } from 'vitest'
import type { SupplierGroup, SupplierKey } from '@/api/admin/adminPlus'
import { isRepairableSub2APILandingKey, supplierGroupAction } from '../supplierProvisionPresentation'

function group(overrides: Partial<SupplierGroup> = {}): SupplierGroup {
  return {
    id: 10,
    supplier_id: 7,
    external_group_id: '88',
    name: 'private-openai',
    description: '',
    provider_family: 'openai',
    rate_multiplier: 1,
    effective_rate_multiplier: 1,
    allow_image_generation: false,
    is_private: false,
    status: 'active',
    last_seen_at: '2026-06-21T10:00:00Z',
    created_at: '2026-06-21T10:00:00Z',
    updated_at: '2026-06-21T10:00:00Z',
    ...overrides
  }
}

function key(overrides: Partial<SupplierKey> = {}): SupplierKey {
  return {
    id: 100,
    supplier_id: 7,
    supplier_group_id: 10,
    external_group_id: '88',
    external_key_id: '99',
    name: 'share-api / private-openai',
    status: 'bound',
    provider_family: 'openai',
    created_at: '2026-06-21T10:00:00Z',
    updated_at: '2026-06-21T10:00:00Z',
    ...overrides
  }
}

describe('supplierProvisionPresentation', () => {
  it('缺少 Key 的分组显示直接创建动作', () => {
    expect(supplierGroupAction(group(), undefined)).toMatchObject({
      kind: 'provision',
      label: '创建 Key',
      icon: 'key',
      disabled: false
    })
  })

  it('已绑定分组不再显示重复开通或修复动作', () => {
    expect(supplierGroupAction(group(), key({ status: 'bound', local_sub2api_account_id: 1001 }))).toMatchObject({
      kind: 'none',
      disabled: true
    })
  })

  it('真实 Sub2API 落地失败时显示步骤内修复动作', () => {
    const failedKey = key({ status: 'failed', error_code: 'SUB2API_GATEWAY_CONFIG_REQUIRED' })

    expect(isRepairableSub2APILandingKey(failedKey)).toBe(true)
    expect(supplierGroupAction(group(), failedKey)).toMatchObject({
      kind: 'repair_sub2api_landing',
      label: '完成绑定',
      icon: 'link',
      disabled: false
    })
  })

  it('非本地落地失败不显示修复动作', () => {
    const failedKey = key({ status: 'failed', error_code: 'SUPPLIER_KEY_PROVIDER_UNSUPPORTED' })

    expect(isRepairableSub2APILandingKey(failedKey)).toBe(false)
    expect(supplierGroupAction(group(), failedKey)).toMatchObject({
      kind: 'none',
      disabled: true
    })
  })

  it('无效分组保留动作语义但禁用提交', () => {
    expect(supplierGroupAction(group({ status: 'disabled' }), undefined)).toMatchObject({
      kind: 'provision',
      disabled: true
    })
    expect(supplierGroupAction(group({ status: 'missing' }), key({ status: 'failed', error_code: 'LOCAL_ACCOUNT_CREATE_FAILED' }))).toMatchObject({
      kind: 'repair_sub2api_landing',
      disabled: true
    })
  })
})
