import { describe, expect, it } from 'vitest'
import { isPartialProvisionSkippableBlockReason } from '../suppliers/provision'

describe('supplier provision partial plan', () => {
  it('允许批量任务跳过明确需要人工处理的分组', () => {
    expect(isPartialProvisionSkippableBlockReason('key_capacity_exhausted')).toBe(true)
    expect(isPartialProvisionSkippableBlockReason('group_key_capacity_exhausted')).toBe(true)
    expect(isPartialProvisionSkippableBlockReason('group_key_capacity_unknown')).toBe(true)
    expect(isPartialProvisionSkippableBlockReason('group_key_provisioning_unsupported')).toBe(true)
    expect(isPartialProvisionSkippableBlockReason('provider_key_exists_unbound')).toBe(true)
  })

  it('未知供应商配额和列表风险不再作为可跳过阻塞项', () => {
    expect(isPartialProvisionSkippableBlockReason('key_capacity_unknown')).toBe(false)
    expect(isPartialProvisionSkippableBlockReason('provider_key_capacity_incomplete')).toBe(false)
    expect(isPartialProvisionSkippableBlockReason(undefined)).toBe(false)
  })
})
