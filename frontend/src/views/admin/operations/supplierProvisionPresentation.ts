import type { SupplierGroup, SupplierKey } from '@/api/admin/adminPlus'

export type SupplierGroupActionKind = 'provision' | 'repair_sub2api_landing' | 'none'

export interface SupplierGroupAction {
  kind: SupplierGroupActionKind
  label: string
  icon: 'key' | 'link'
  title: string
  disabled: boolean
}

const REPAIRABLE_KEY_ERRORS = new Set([
  'LOCAL_ACCOUNT_CREATE_FAILED',
  'SUPPLIER_ACCOUNT_BIND_FAILED',
  'LOCAL_SUB2API_GROUP_LIST_FAILED',
  'LOCAL_SUB2API_GROUP_CREATE_FAILED',
  'LOCAL_SUB2API_ACCOUNT_LOOKUP_FAILED',
  'LOCAL_SUB2API_ACCOUNT_GET_FAILED',
  'LOCAL_SUB2API_ACCOUNT_CREATE_FAILED',
  'LOCAL_SUB2API_ACCOUNT_STATE_SYNC_FAILED',
  'SUB2API_GATEWAY_CONFIG_REQUIRED',
  'SUB2API_GATEWAY_BASE_URL_INVALID',
  'SUB2API_GATEWAY_NOT_CONFIGURED',
  'SUB2API_GATEWAY_REQUEST_FAILED',
  'SUB2API_GATEWAY_RESPONSE_INVALID',
  'SUB2API_GATEWAY_BAD_STATUS'
])

export function isRepairableSub2APILandingKey(key?: Pick<SupplierKey, 'status' | 'error_code'> | null): key is SupplierKey {
  return key?.status === 'failed' && REPAIRABLE_KEY_ERRORS.has(key.error_code || '')
}

export function supplierGroupAction(group: Pick<SupplierGroup, 'status'>, key?: Pick<SupplierKey, 'status' | 'error_code'> | null): SupplierGroupAction {
  if (!key) {
    return {
      kind: 'provision',
      label: '创建 Key',
      icon: 'key',
      title: group.status === 'active' ? '创建第三方 Key，并自动创建和绑定本地 Sub2API 账号' : '只有有效分组可以创建 Key',
      disabled: group.status !== 'active'
    }
  }
  if (isRepairableSub2APILandingKey(key)) {
    return {
      kind: 'repair_sub2api_landing',
      label: '完成绑定',
      icon: 'link',
      title: '第三方 Key 已存在，但本地 Sub2API 分组或账号尚未绑定',
      disabled: group.status !== 'active'
    }
  }
  return {
    kind: 'none',
    label: '',
    icon: 'key',
    title: '该分组已有 Key 记录',
    disabled: true
  }
}
