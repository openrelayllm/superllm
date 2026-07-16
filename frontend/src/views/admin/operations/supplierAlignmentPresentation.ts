import type { LocalSub2APIAccount, SupplierAccount, SupplierGroup, SupplierKey } from '@/api/admin/adminPlus'

export type SupplierAlignmentStatus =
  | 'aligned'
  | 'missing_key'
  | 'unbound_key'
  | 'missing_local_account'
  | 'local_account_disabled'
  | 'binding_mismatch'
  | 'key_provisioning'
  | 'key_manual_required'
  | 'key_failed'
  | 'key_disabled'
  | 'group_missing'
  | 'group_disabled'

export interface SupplierAlignmentRow {
  id: string
  group?: SupplierGroup
  key?: SupplierKey
  binding?: SupplierAccount
  local?: LocalSub2APIAccount
  localAccountID?: number
  alignmentStatus: SupplierAlignmentStatus
  canCreate: boolean
  canRepair: boolean
  canManageLocal: boolean
}

export function buildSupplierAlignmentRows(
  groupItems: SupplierGroup[],
  keyItems: SupplierKey[],
  bindingItems: SupplierAccount[],
  localItems: LocalSub2APIAccount[]
): SupplierAlignmentRow[] {
  const groupByID = new Map(groupItems.map((group) => [group.id, group]))
  const keysByGroup = new Map<number, SupplierKey[]>()
  const bindingsByGroup = new Map<number, SupplierAccount[]>()
  const bindingsByKey = new Map<number, SupplierAccount[]>()
  const localByID = new Map(localItems.map((account) => [account.id, account]))
  for (const key of keyItems) keysByGroup.set(key.supplier_group_id, [...(keysByGroup.get(key.supplier_group_id) || []), key])
  for (const binding of bindingItems) {
    if (binding.supplier_group_id) bindingsByGroup.set(binding.supplier_group_id, [...(bindingsByGroup.get(binding.supplier_group_id) || []), binding])
    if (binding.supplier_key_id) bindingsByKey.set(binding.supplier_key_id, [...(bindingsByKey.get(binding.supplier_key_id) || []), binding])
  }

  const out: SupplierAlignmentRow[] = []
  const usedBindingIDs = new Set<number>()
  const usedKeyIDs = new Set<number>()
  for (const group of groupItems) {
    const groupKeys = keysByGroup.get(group.id) || []
    const groupBindings = bindingsByGroup.get(group.id) || []
    if (groupKeys.length === 0) {
      if (groupBindings.length === 0) out.push(makeAlignmentRow(`${group.id}:group`, group, undefined, undefined, localByID))
      for (const binding of groupBindings) {
        usedBindingIDs.add(binding.id)
        out.push(makeAlignmentRow(`${group.id}:binding:${binding.id}`, group, undefined, binding, localByID))
      }
      continue
    }
    for (const key of groupKeys) {
      usedKeyIDs.add(key.id)
      const candidates = bindingsByKey.get(key.id) || groupBindings.filter((binding) => binding.local_sub2api_account_id === key.local_sub2api_account_id)
      if (candidates.length === 0) out.push(makeAlignmentRow(`${group.id}:key:${key.id}`, group, key, undefined, localByID))
      for (const binding of candidates) {
        usedBindingIDs.add(binding.id)
        out.push(makeAlignmentRow(`${group.id}:key:${key.id}:binding:${binding.id}`, group, key, binding, localByID))
      }
    }
  }

  for (const key of keyItems) {
    if (!usedKeyIDs.has(key.id)) out.push(makeAlignmentRow(`key:${key.id}`, groupByID.get(key.supplier_group_id), key, undefined, localByID))
  }
  for (const binding of bindingItems) {
    if (!usedBindingIDs.has(binding.id)) out.push(makeAlignmentRow(`binding:${binding.id}`, groupByID.get(binding.supplier_group_id || 0), undefined, binding, localByID))
  }
  return out
}

export function supplierAlignmentStatusLabel(value: SupplierAlignmentStatus): string {
  return ({
    aligned: '已对齐',
    missing_key: '缺少第三方 Key',
    unbound_key: 'Key 已存在但未绑定',
    missing_local_account: '缺少本地账号',
    local_account_disabled: '本地账号已停用',
    binding_mismatch: '本地账号绑定错误',
    key_provisioning: 'Key 创建中',
    key_manual_required: '需补录密钥',
    key_failed: 'Key 创建失败',
    key_disabled: 'Key 已禁用',
    group_missing: '供应商分组已删除',
    group_disabled: '供应商分组已停用'
  } as Record<SupplierAlignmentStatus, string>)[value]
}

function makeAlignmentRow(
  id: string,
  group: SupplierGroup | undefined,
  key: SupplierKey | undefined,
  binding: SupplierAccount | undefined,
  localByID: Map<number, LocalSub2APIAccount>
): SupplierAlignmentRow {
  const localAccountID = key?.local_sub2api_account_id || binding?.local_sub2api_account_id
  const local = localAccountID ? localByID.get(localAccountID) : undefined
  const bindingMismatch = Boolean(key && binding && (
    (binding.supplier_key_id && binding.supplier_key_id !== key.id) ||
    (key.local_sub2api_account_id && key.local_sub2api_account_id !== binding.local_sub2api_account_id)
  ))
  const alignmentStatus = resolveSupplierAlignmentStatus(group, key, binding, local, localAccountID, bindingMismatch)
  return {
    id,
    group,
    key,
    binding,
    local,
    localAccountID,
    alignmentStatus,
    canCreate: Boolean(group && group.status === 'active' && !key),
    canRepair: Boolean(key && ['unbound_key', 'missing_local_account', 'binding_mismatch', 'key_manual_required', 'key_failed'].includes(alignmentStatus)),
    canManageLocal: alignmentStatus === 'local_account_disabled'
  }
}

function resolveSupplierAlignmentStatus(
  group: SupplierGroup | undefined,
  key: SupplierKey | undefined,
  binding: SupplierAccount | undefined,
  local: LocalSub2APIAccount | undefined,
  localAccountID: number | undefined,
  bindingMismatch: boolean
): SupplierAlignmentStatus {
  if (!group) return 'group_missing'
  if (group.status !== 'active') return 'group_disabled'
  if (!key) return 'missing_key'
  if (key.status === 'manual_secret_required') return 'key_manual_required'
  if (key.status === 'failed') return 'key_failed'
  if (key.status === 'disabled') return 'key_disabled'
  if (key.status !== 'bound') return 'key_provisioning'
  if (!binding) return 'unbound_key'
  if (bindingMismatch) return 'binding_mismatch'
  if (!local && localAccountID) return 'missing_local_account'
  if (!local) return 'unbound_key'
  if (['disabled', 'error', 'failed', 'expired'].includes(String(local.status).toLowerCase())) return 'local_account_disabled'
  return 'aligned'
}
