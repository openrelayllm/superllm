import { listLocalAccountOps, listLocalSub2APIAccounts, listSupplierAccounts, listSupplierChannelChecks, listSupplierGroups, listSupplierKeys } from '@/api/admin/adminPlus'
import type { Supplier } from '@/api/admin/adminPlus'
import { ctxFn, ctxValue } from './ctxProxy'

export function attachSupplierDetail(ctx: any) {
  const supplierDetailDialogOpen = ctxValue(ctx, 'supplierDetailDialogOpen')
  const supplierDetailLoading = ctxValue(ctx, 'supplierDetailLoading')
  const supplierDetailError = ctxValue(ctx, 'supplierDetailError')
  const supplierDetailSupplier = ctxValue(ctx, 'supplierDetailSupplier')
  const supplierDetailGroups = ctxValue(ctx, 'supplierDetailGroups')
  const supplierDetailKeys = ctxValue(ctx, 'supplierDetailKeys')
  const supplierDetailAccounts = ctxValue(ctx, 'supplierDetailAccounts')
  const supplierDetailLocalAccounts = ctxValue(ctx, 'supplierDetailLocalAccounts')
  const supplierDetailLocalOpsRows = ctxValue(ctx, 'supplierDetailLocalOpsRows')
  const supplierDetailChannelChecks = ctxValue(ctx, 'supplierDetailChannelChecks')
  const errorMessage = ctxFn(ctx, 'errorMessage')

  let supplierDetailLoadSeq = 0

  function resetSupplierDetailData() {
    supplierDetailGroups.value = []
    supplierDetailKeys.value = []
    supplierDetailAccounts.value = []
    supplierDetailLocalAccounts.value = []
    supplierDetailLocalOpsRows.value = []
    supplierDetailChannelChecks.value = []
  }

  function openSupplierDetailDialog(supplier: Supplier) {
    supplierDetailSupplier.value = supplier
    supplierDetailDialogOpen.value = true
    supplierDetailError.value = ''
    resetSupplierDetailData()
    void loadSupplierDetail()
  }

  function closeSupplierDetailDialog() {
    supplierDetailLoadSeq++
    supplierDetailDialogOpen.value = false
    supplierDetailLoading.value = false
    supplierDetailError.value = ''
    supplierDetailSupplier.value = null
    resetSupplierDetailData()
  }

  async function loadSupplierDetail() {
    const supplier = supplierDetailSupplier.value
    if (!supplier) return
    const seq = ++supplierDetailLoadSeq
    supplierDetailLoading.value = true
    supplierDetailError.value = ''
    try {
      const [
        groupResult,
        keyResult,
        accountResult,
        localAccountResult,
        localOpsResult,
        channelCheckResult
      ] = await Promise.all([
        listSupplierGroups(supplier.id, { page: 1, page_size: 1000 }),
        listSupplierKeys(supplier.id, { page: 1, page_size: 1000 }),
        listSupplierAccounts(supplier.id, { page: 1, page_size: 1000 }),
        listLocalSub2APIAccounts({ page: 1, page_size: 1000 }),
        listLocalAccountOps({ supplier_id: supplier.id, page: 1, page_size: 1000 }),
        listSupplierChannelChecks(supplier.id, { page: 1, page_size: 1000 })
      ])
      if (seq !== supplierDetailLoadSeq) return
      supplierDetailGroups.value = groupResult.items || []
      supplierDetailKeys.value = keyResult.items || []
      supplierDetailAccounts.value = accountResult.items || []
      supplierDetailLocalAccounts.value = localAccountResult.items || []
      supplierDetailLocalOpsRows.value = localOpsResult.items || []
      supplierDetailChannelChecks.value = channelCheckResult.items || []
    } catch (error) {
      if (seq !== supplierDetailLoadSeq) return
      supplierDetailError.value = errorMessage(error, '加载供应商详情失败')
    } finally {
      if (seq === supplierDetailLoadSeq) {
        supplierDetailLoading.value = false
      }
    }
  }

  Object.assign(ctx, {
    openSupplierDetailDialog,
    closeSupplierDetailDialog,
    loadSupplierDetail
  })
}
