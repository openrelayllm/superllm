import { createSuppliersState } from './state'
import { attachPresentationCore } from './presentationCore'
import { attachPresentationStatus } from './presentationStatus'
import { attachSuppliersComputed } from './computed'
import { attachSuppliersData } from './data'
import { attachSupplierDialogs } from './dialogs'
import { attachSupplierGroups } from './groups'
import { attachSupplierProvision } from './provision'
import { attachSupplierBulkAndLifecycle } from './bulkAndLifecycle'

export function useSuppliersViewModel() {
  const ctx = createSuppliersState()
  attachPresentationCore(ctx)
  attachPresentationStatus(ctx)
  attachSuppliersComputed(ctx)
  attachSuppliersData(ctx)
  attachSupplierDialogs(ctx)
  attachSupplierGroups(ctx)
  attachSupplierProvision(ctx)
  attachSupplierBulkAndLifecycle(ctx)
  return ctx
}

export type SuppliersViewModel = ReturnType<typeof useSuppliersViewModel>
