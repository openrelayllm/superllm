import { onBeforeUnmount, onMounted, watch } from 'vue'
import { deleteSupplier, getSupplierCurrentBalance, updateSupplierStatus } from '@/api/admin/adminPlus'
import type { Supplier, SupplierHealthStatus, SupplierRuntimeStatus } from '@/api/admin/adminPlus'
import type { ComputedRef, Ref } from 'vue'
import { ctxFn, ctxValue } from './ctxProxy'
export function attachSupplierBulkAndLifecycle(ctx: any) {
  const appStore = ctxValue(ctx, 'appStore')
  const statusSubmitting = ctxValue(ctx, 'statusSubmitting')
  const statusDialogOpen = ctxValue(ctx, 'statusDialogOpen')
  const groupsDialogOpen = ctxValue(ctx, 'groupsDialogOpen')
  const deleteDialogOpen = ctxValue(ctx, 'deleteDialogOpen')
  const moreMenuOpen = ctxValue(ctx, 'moreMenuOpen')
  const bulkStatusMode = ctxValue(ctx, 'bulkStatusMode')
  const bulkDeleteMode = ctxValue(ctx, 'bulkDeleteMode')
  const bulkBalanceRefreshing = ctxValue(ctx, 'bulkBalanceRefreshing')
  const editingSupplier = ctxValue(ctx, 'editingSupplier')
  const sessionSupplier = ctxValue(ctx, 'sessionSupplier')
  const channelStatusSupplier = ctxValue(ctx, 'channelStatusSupplier')
  const groupsSupplier = ctxValue(ctx, 'groupsSupplier')
  const channelScheduleSupplier = ctxValue(ctx, 'channelScheduleSupplier')
  const deletingSupplier = ctxValue(ctx, 'deletingSupplier')
  const rowActionsMenuSupplier = ctxValue(ctx, 'rowActionsMenuSupplier')
  const rowActionsMenuStyle = ctxValue(ctx, 'rowActionsMenuStyle')
  const suppliers = ctxValue(ctx, 'suppliers') as Ref<Supplier[]>
  const currentBalanceStore = ctxValue(ctx, 'currentBalanceStore')
  const rowBalanceRefreshingID = ctxValue(ctx, 'rowBalanceRefreshingID')
  const quickStatusSupplierID = ctxValue(ctx, 'quickStatusSupplierID')
  const ROW_ACTIONS_MENU_WIDTH = ctxValue(ctx, 'ROW_ACTIONS_MENU_WIDTH')
  const ROW_ACTIONS_MENU_HEIGHT = ctxValue(ctx, 'ROW_ACTIONS_MENU_HEIGHT')
  const ROW_ACTIONS_MENU_MARGIN = ctxValue(ctx, 'ROW_ACTIONS_MENU_MARGIN')
  const filters = ctxValue(ctx, 'filters')
  const groupPagination = ctxValue(ctx, 'groupPagination')
  const groupFilters = ctxValue(ctx, 'groupFilters')
  const statusForm = ctxValue(ctx, 'statusForm')
  const selectedCount = ctxValue(ctx, 'selectedCount')
  const selectedRows = ctxValue(ctx, 'selectedRows') as ComputedRef<Supplier[]>
  const clearSelection = ctxFn(ctx, 'clearSelection')
  const loadSuppliers = ctxFn(ctx, 'loadSuppliers')
  const reloadFirstPage = ctxFn(ctx, 'reloadFirstPage')
  const openStatusDialog = ctxFn(ctx, 'openStatusDialog')
  const openSessionDialog = ctxFn(ctx, 'openSessionDialog')
  const openChannelStatusDialog = ctxFn(ctx, 'openChannelStatusDialog')
  const stopChannelStatusAutoRefresh = ctxFn(ctx, 'stopChannelStatusAutoRefresh')
  const openGroupsDialog = ctxFn(ctx, 'openGroupsDialog')
  const loadCurrentGroups = ctxFn(ctx, 'loadCurrentGroups')
  const stopProvisionJobPolling = ctxFn(ctx, 'stopProvisionJobPolling')
  const normalizeBalanceErrorMessage = ctxFn(ctx, 'normalizeBalanceErrorMessage')
  const errorMessage = ctxFn(ctx, 'errorMessage')
  function openBulkStatusDialog() {
    if (selectedCount.value === 0) return
    moreMenuOpen.value = false
    bulkStatusMode.value = true
    const first = selectedRows.value[0]
    statusForm.id = 0
    statusForm.name = ''
    statusForm.runtime_status = first?.runtime_status || 'monitor_only'
    statusForm.health_status = first?.health_status || 'normal'
    statusDialogOpen.value = true
  }

  async function refreshSelectedBalances() {
    if (selectedCount.value === 0 || bulkBalanceRefreshing.value) return
    const targets = [...selectedRows.value]
    if (targets.length === 0) {
      appStore.showWarning('当前页没有可更新的已选供应商')
      return
    }

    moreMenuOpen.value = false
    bulkBalanceRefreshing.value = true
    let success = 0
    let failed = 0
    const failures: string[] = []

    try {
      for (const supplier of targets) {
        try {
          const balance = await getSupplierCurrentBalance(supplier.id, { refresh: true })
          currentBalanceStore[supplier.id] = balance
          if (balance.fallback) {
            failed++
            failures.push(`${supplier.name}: ${normalizeBalanceErrorMessage(balance.refresh_error_message)}`)
          } else {
            success++
          }
        } catch (error) {
          failed++
          failures.push(`${supplier.name}: ${normalizeBalanceErrorMessage(errorMessage(error, '读取当前额度失败'))}`)
        }
      }

      await loadSuppliers()
      const failureText = failures.length > 0
        ? `；${failures.slice(0, 3).join('；')}${failures.length > 3 ? ` 等 ${failures.length} 项` : ''}`
        : ''

      if (failed === 0) {
        appStore.showSuccess(`批量更新额度完成：成功 ${success}`)
        return
      }
      if (success > 0) {
        appStore.showWarning(`批量更新额度完成：成功 ${success}，失败 ${failed}${failureText}`, 7000)
        return
      }
      appStore.showError(`批量更新额度失败：失败 ${failed}${failureText}`, 8000)
    } finally {
      bulkBalanceRefreshing.value = false
    }
  }

  async function refreshSupplierBalance(supplier: Supplier) {
    if (rowBalanceRefreshingID.value !== null) return
    rowBalanceRefreshingID.value = supplier.id
    try {
      const balance = await getSupplierCurrentBalance(supplier.id, { refresh: true })
      currentBalanceStore[supplier.id] = balance
      await loadSuppliers()
      if (balance.fallback) {
        appStore.showWarning(`${supplier.name}: ${normalizeBalanceErrorMessage(balance.refresh_error_message)}`, 7000)
        return
      }
      appStore.showSuccess(`${supplier.name} 余额已刷新`)
    } catch (error) {
      appStore.showError(`${supplier.name}: ${normalizeBalanceErrorMessage(errorMessage(error, '读取当前额度失败'))}`, 8000)
    } finally {
      rowBalanceRefreshingID.value = null
    }
  }

  async function submitStatus() {
    statusSubmitting.value = true
    try {
      if (bulkStatusMode.value) {
        await runSequential(selectedRows.value, async (supplier) => {
          await updateSupplierStatus(supplier.id, {
            runtime_status: statusForm.runtime_status,
            health_status: statusForm.health_status
          })
        })
        appStore.showSuccess('批量状态已更新')
        clearSelection()
      } else if (statusForm.id) {
        await updateSupplierStatus(statusForm.id, {
          runtime_status: statusForm.runtime_status,
          health_status: statusForm.health_status
        })
        appStore.showSuccess('状态已更新')
      }
      statusDialogOpen.value = false
      await loadSuppliers()
    } catch (error) {
      appStore.showError((error as { message?: string }).message || '更新状态失败')
    } finally {
      statusSubmitting.value = false
    }
  }

  function handleQuickRuntimeStatusChange(supplier: Supplier, event: Event) {
    const select = event.target as HTMLSelectElement
    void quickUpdateSupplierStatus(supplier, { runtime_status: select.value as SupplierRuntimeStatus }, select)
  }

  function handleQuickHealthStatusChange(supplier: Supplier, event: Event) {
    const select = event.target as HTMLSelectElement
    void quickUpdateSupplierStatus(supplier, { health_status: select.value as SupplierHealthStatus }, select)
  }

  async function quickUpdateSupplierStatus(
    supplier: Supplier,
    patch: Partial<Pick<Supplier, 'runtime_status' | 'health_status'>>,
    select?: HTMLSelectElement
  ) {
    const nextRuntimeStatus = patch.runtime_status || supplier.runtime_status
    const nextHealthStatus = patch.health_status || supplier.health_status
    if (nextRuntimeStatus === supplier.runtime_status && nextHealthStatus === supplier.health_status) return
    if (quickStatusSupplierID.value === supplier.id) {
      resetQuickStatusSelect(supplier, patch, select)
      return
    }

    quickStatusSupplierID.value = supplier.id
    try {
      const updated = await updateSupplierStatus(supplier.id, {
        runtime_status: nextRuntimeStatus,
        health_status: nextHealthStatus
      })
      replaceSupplier(updated)
      appStore.showSuccess('状态已更新')
    } catch (error) {
      resetQuickStatusSelect(supplier, patch, select)
      appStore.showError((error as { message?: string }).message || '更新状态失败')
    } finally {
      if (quickStatusSupplierID.value === supplier.id) {
        quickStatusSupplierID.value = null
      }
    }
  }

  function resetQuickStatusSelect(
    supplier: Supplier,
    patch: Partial<Pick<Supplier, 'runtime_status' | 'health_status'>>,
    select?: HTMLSelectElement
  ) {
    if (!select) return
    select.value = patch.runtime_status ? supplier.runtime_status : supplier.health_status
  }

  function replaceSupplier(updated: Supplier) {
    suppliers.value = suppliers.value.map((item) => item.id === updated.id ? updated : item)
    if (editingSupplier.value?.id === updated.id) editingSupplier.value = updated
    if (sessionSupplier.value?.id === updated.id) sessionSupplier.value = updated
    if (channelStatusSupplier.value?.id === updated.id) channelStatusSupplier.value = updated
    if (groupsSupplier.value?.id === updated.id) groupsSupplier.value = updated
    if (channelScheduleSupplier.value?.id === updated.id) channelScheduleSupplier.value = updated
    if (deletingSupplier.value?.id === updated.id) deletingSupplier.value = updated
    if (rowActionsMenuSupplier.value?.id === updated.id) rowActionsMenuSupplier.value = updated
  }

  function openDeleteDialog(supplier: Supplier) {
    closeRowActionsMenu()
    bulkDeleteMode.value = false
    deletingSupplier.value = supplier
    deleteDialogOpen.value = true
  }

  function toggleRowActionsMenu(supplier: Supplier, event: MouseEvent) {
    if (rowActionsMenuSupplier.value?.id === supplier.id) {
      closeRowActionsMenu()
      return
    }
    const target = event.currentTarget as HTMLElement | null
    const rect = target?.getBoundingClientRect()
    rowActionsMenuSupplier.value = supplier
    rowActionsMenuStyle.value = positionRowActionsMenu(rect)
  }

  function positionRowActionsMenu(rect?: DOMRect): Record<string, string> {
    if (!rect || typeof window === 'undefined') {
      return {
        top: '96px',
        left: `${ROW_ACTIONS_MENU_MARGIN}px`,
        width: `${ROW_ACTIONS_MENU_WIDTH}px`
      }
    }

    const maxLeft = Math.max(ROW_ACTIONS_MENU_MARGIN, window.innerWidth - ROW_ACTIONS_MENU_WIDTH - ROW_ACTIONS_MENU_MARGIN)
    const left = Math.min(Math.max(rect.right - ROW_ACTIONS_MENU_WIDTH, ROW_ACTIONS_MENU_MARGIN), maxLeft)
    const belowTop = rect.bottom + ROW_ACTIONS_MENU_MARGIN
    const aboveTop = rect.top - ROW_ACTIONS_MENU_HEIGHT - ROW_ACTIONS_MENU_MARGIN
    const hasSpaceBelow = belowTop + ROW_ACTIONS_MENU_HEIGHT <= window.innerHeight - ROW_ACTIONS_MENU_MARGIN
    const top = hasSpaceBelow ? belowTop : Math.max(ROW_ACTIONS_MENU_MARGIN, aboveTop)

    return {
      top: `${top}px`,
      left: `${left}px`,
      width: `${ROW_ACTIONS_MENU_WIDTH}px`
    }
  }

  function closeRowActionsMenu() {
    rowActionsMenuSupplier.value = null
    rowActionsMenuStyle.value = {}
  }

  function handleRowActionsOutsideClick(event: MouseEvent) {
    if (!rowActionsMenuSupplier.value) return
    const target = event.target as HTMLElement | null
    if (!target) return
    if (target.closest('[data-supplier-row-actions-menu]') || target.closest('[data-supplier-row-actions-trigger]')) return
    closeRowActionsMenu()
  }

  function mountRowActionsMenuListeners() {
    document.addEventListener('click', handleRowActionsOutsideClick)
    window.addEventListener('resize', closeRowActionsMenu)
    window.addEventListener('scroll', closeRowActionsMenu, true)
  }

  function unmountRowActionsMenuListeners() {
    document.removeEventListener('click', handleRowActionsOutsideClick)
    window.removeEventListener('resize', closeRowActionsMenu)
    window.removeEventListener('scroll', closeRowActionsMenu, true)
  }

  function openRowStatusDialog() {
    const supplier = rowActionsMenuSupplier.value
    if (!supplier) return
    openStatusDialog(supplier)
  }

  function openRowSessionDialog() {
    const supplier = rowActionsMenuSupplier.value
    if (!supplier) return
    openSessionDialog(supplier)
  }

  function openRowGroupsDialog() {
    const supplier = rowActionsMenuSupplier.value
    if (!supplier) return
    openGroupsDialog(supplier)
  }

  function openRowChannelStatusDialog() {
    const supplier = rowActionsMenuSupplier.value
    if (!supplier) return
    openChannelStatusDialog(supplier)
  }

  function openRowDeleteDialog() {
    const supplier = rowActionsMenuSupplier.value
    if (!supplier) return
    openDeleteDialog(supplier)
  }

  function openBulkDeleteDialog() {
    if (selectedCount.value === 0) return
    moreMenuOpen.value = false
    bulkDeleteMode.value = true
    deletingSupplier.value = null
    deleteDialogOpen.value = true
  }

  async function confirmDelete() {
    try {
      if (bulkDeleteMode.value) {
        await runSequential(selectedRows.value, async (supplier) => {
          await deleteSupplier(supplier.id)
        })
        appStore.showSuccess('已删除选中的供应商')
        clearSelection()
      } else if (deletingSupplier.value) {
        await deleteSupplier(deletingSupplier.value.id)
        appStore.showSuccess('供应商已删除')
      }
      deleteDialogOpen.value = false
      await loadSuppliers()
    } catch (error) {
      appStore.showError((error as { message?: string }).message || '删除供应商失败')
    }
  }

  async function runSequential<T>(items: T[], runner: (item: T) => Promise<void>) {
    for (const item of items) {
      await runner(item)
    }
  }

  watch(
    () => ({ ...filters }),
    () => {
      reloadFirstPage()
    }
  )

  watch(
    () => ({ ...groupFilters }),
    () => {
      if (!groupsDialogOpen.value) return
      groupPagination.page = 1
      void loadCurrentGroups()
    }
  )

  function cleanupTimers() {
    stopProvisionJobPolling()
    stopChannelStatusAutoRefresh()
    closeRowActionsMenu()
    unmountRowActionsMenuListeners()
  }

  onMounted(() => {
    mountRowActionsMenuListeners()
    void loadSuppliers()
  })
  onBeforeUnmount(cleanupTimers)

  Object.assign(ctx, {
    openBulkStatusDialog,
    refreshSelectedBalances,
    refreshSupplierBalance,
    submitStatus,
    handleQuickRuntimeStatusChange,
    handleQuickHealthStatusChange,
    quickUpdateSupplierStatus,
    resetQuickStatusSelect,
    replaceSupplier,
    openDeleteDialog,
    toggleRowActionsMenu,
    positionRowActionsMenu,
    closeRowActionsMenu,
    handleRowActionsOutsideClick,
    mountRowActionsMenuListeners,
    unmountRowActionsMenuListeners,
    openRowStatusDialog,
    openRowSessionDialog,
    openRowGroupsDialog,
    openRowChannelStatusDialog,
    openRowDeleteDialog,
    openBulkDeleteDialog,
    confirmDelete,
    cleanupTimers
  })
}
