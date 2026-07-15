import { describe, expect, it } from 'vitest'
import { isPartialProvisionSkippableBlockReason } from '../suppliers/provision'

describe('supplier provision partial plan', () => {
  it('允许部分提交时跳过配额耗尽或第三方未绑定 Key', () => {
    expect(isPartialProvisionSkippableBlockReason('key_capacity_exhausted')).toBe(true)
    expect(isPartialProvisionSkippableBlockReason('group_key_capacity_exhausted')).toBe(true)
    expect(isPartialProvisionSkippableBlockReason('provider_key_exists_unbound')).toBe(true)
  })

  it('不会跳过需要先修复配置或重新同步的阻塞项', () => {
    expect(isPartialProvisionSkippableBlockReason('key_capacity_unknown')).toBe(false)
    expect(isPartialProvisionSkippableBlockReason('provider_key_capacity_incomplete')).toBe(false)
    expect(isPartialProvisionSkippableBlockReason(undefined)).toBe(false)
  })
})
