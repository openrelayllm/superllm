import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { SupplierGroup } from '@/api/admin/adminPlus'
import SupplierAlignmentPanel from '../suppliers/SupplierAlignmentPanel.vue'

const api = vi.hoisted(() => ({
  listSupplierGroups: vi.fn(),
  listSupplierKeys: vi.fn(),
  listSupplierAccounts: vi.fn(),
  listLocalSub2APIAccounts: vi.fn()
}))

vi.mock('@/api/admin/adminPlus', () => api)

const group: SupplierGroup = {
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
  active_key_count: 0,
  key_capacity_status: 'available',
  status: 'active',
  last_seen_at: '2026-07-16T00:00:00Z',
  created_at: '2026-07-16T00:00:00Z',
  updated_at: '2026-07-16T00:00:00Z'
}

function mountPanel() {
  return mount(SupplierAlignmentPanel, {
    props: { supplierId: 9 },
    global: {
      stubs: {
        EmptyState: true,
        Icon: true,
        RouterLink: true
      }
    }
  })
}

function buttonByText(wrapper: ReturnType<typeof mountPanel>, text: string) {
  const button = wrapper.findAll('button').find((item) => item.text().includes(text))
  if (!button) throw new Error(`button not found: ${text}`)
  return button
}

describe('SupplierAlignmentPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    api.listSupplierGroups.mockResolvedValue({ items: [group], pages: 1 })
    api.listSupplierKeys.mockResolvedValue({ items: [], pages: 1 })
    api.listSupplierAccounts.mockResolvedValue({ items: [], pages: 1 })
    api.listLocalSub2APIAccounts.mockResolvedValue({ items: [], pages: 1 })
  })

  it('数据完整时允许创建缺失 Key', async () => {
    const wrapper = mountPanel()
    await flushPromises()

    const createButton = buttonByText(wrapper, '创建 Key')
    expect(createButton.attributes('disabled')).toBeUndefined()

    await createButton.trigger('click')
    expect(wrapper.emitted('create-key')?.[0]).toEqual([group])
  })

  it('任一数据源加载失败时禁用创建和修复入口', async () => {
    api.listSupplierKeys.mockRejectedValue(new Error('upstream unavailable'))

    const wrapper = mountPanel()
    await flushPromises()

    expect(wrapper.get('[role="alert"]').text()).toContain('第三方 Key加载失败')
    expect(wrapper.get('[role="alert"]').text()).toContain('创建和修复操作已禁用')
    expect(buttonByText(wrapper, '去创建缺失 Key').attributes('disabled')).toBeDefined()
    expect(buttonByText(wrapper, '创建 Key').attributes('disabled')).toBeDefined()
  })
})
