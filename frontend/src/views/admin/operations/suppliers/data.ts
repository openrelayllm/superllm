import { enableSupplierChannelScheduling, getSupplierCurrentBalance, getSupplierSession, listSupplierBestChannelChecks, listSupplierChannelChecks, listSupplierChannelMonitors, listLocalSub2APIAccounts, listSupplierAccounts, listSupplierCostSnapshots, listSuppliers, pauseSupplierChannelScheduling, probeSupplierChannel } from '@/api/admin/adminPlus'
import type { Ref } from 'vue'
import type { LocalSub2APIAccount, Supplier, SupplierAccount, SupplierBrowserSession, SupplierChannelCheckSnapshot, SupplierChannelMonitorView, SupplierCostSnapshot, SupplierCurrentBalance } from '@/api/admin/adminPlus'
import type { ScheduleListRow, LoadBestChannelChecksOptions } from './types'
import { ctxFn, ctxValue } from './ctxProxy'
export function attachSuppliersData(ctx: any) {
  const appStore = ctxValue(ctx, 'appStore')
  const route = ctxValue(ctx, 'route')
  const handledDeepLinkKey = ctxValue(ctx, 'handledDeepLinkKey')
  const loading = ctxValue(ctx, 'loading')
  const scheduleListDialogOpen = ctxValue(ctx, 'scheduleListDialogOpen')
  const moreMenuOpen = ctxValue(ctx, 'moreMenuOpen')
  const sessionSupplier = ctxValue(ctx, 'sessionSupplier')
  const channelStatusSupplier = ctxValue(ctx, 'channelStatusSupplier')
  const groupsSupplier = ctxValue(ctx, 'groupsSupplier')
  const suppliers = ctxValue(ctx, 'suppliers') as Ref<Supplier[]>
  const supplierCostSnapshots = ctxValue(ctx, 'supplierCostSnapshots') as Ref<Record<number, SupplierCostSnapshot | undefined>>
  const supplierBestChannels = ctxValue(ctx, 'supplierBestChannels') as Ref<Record<number, SupplierChannelCheckSnapshot[] | undefined>>
  const channelMonitorItems = ctxValue(ctx, 'channelMonitorItems') as Ref<SupplierChannelMonitorView[]>
  const sessionStore = ctxValue(ctx, 'sessionStore') as Record<number, SupplierBrowserSession | undefined>
  const currentBalanceStore = ctxValue(ctx, 'currentBalanceStore') as Record<number, SupplierCurrentBalance | undefined>
  const sessionLoading = ctxValue(ctx, 'sessionLoading')
  const currentBalanceLoading = ctxValue(ctx, 'currentBalanceLoading')
  const currentBalanceError = ctxValue(ctx, 'currentBalanceError')
  const channelStatusLoading = ctxValue(ctx, 'channelStatusLoading')
  const scheduleListLoading = ctxValue(ctx, 'scheduleListLoading')
  const scheduleListActionKey = ctxValue(ctx, 'scheduleListActionKey')
  const sessionLoadError = ctxValue(ctx, 'sessionLoadError')
  const channelStatusError = ctxValue(ctx, 'channelStatusError')
  const channelMonitorCapturedAt = ctxValue(ctx, 'channelMonitorCapturedAt')
  const channelMonitorOrigin = ctxValue(ctx, 'channelMonitorOrigin')
  const channelMonitorAPIBaseURL = ctxValue(ctx, 'channelMonitorAPIBaseURL')
  const channelStatusCountdown = ctxValue(ctx, 'channelStatusCountdown')
  const scheduleListError = ctxValue(ctx, 'scheduleListError')
  const filters = ctxValue(ctx, 'filters')
  const pagination = ctxValue(ctx, 'pagination')
  const groupPagination = ctxValue(ctx, 'groupPagination')
  const scheduleListSuppliers = ctxValue(ctx, 'scheduleListSuppliers') as Ref<Supplier[]>
  const scheduleListBindings = ctxValue(ctx, 'scheduleListBindings') as Ref<SupplierAccount[]>
  const scheduleListLocalAccounts = ctxValue(ctx, 'scheduleListLocalAccounts') as Ref<Record<number, LocalSub2APIAccount | undefined>>
  const scheduleListChannelChecks = ctxValue(ctx, 'scheduleListChannelChecks') as Ref<Record<string, SupplierChannelCheckSnapshot | undefined>>
  const preferredSupplierCostSnapshots = ctxFn(ctx, 'preferredSupplierCostSnapshots')
  const channelProtocol = ctxFn(ctx, 'channelProtocol')
  const compareChannelProtocolSnapshots = ctxFn(ctx, 'compareChannelProtocolSnapshots')
  const scheduleChannelKey = ctxFn(ctx, 'scheduleChannelKey')
  const channelStatusErrorMessage = ctxFn(ctx, 'channelStatusErrorMessage')
  const openGroupsDialog = ctxFn(ctx, 'openGroupsDialog')
  const loadCurrentGroups = ctxFn(ctx, 'loadCurrentGroups')
  const mergeChannelCheckSnapshots = ctxFn(ctx, 'mergeChannelCheckSnapshots')
  async function loadSuppliers() {
    loading.value = true
    try {
      const [result, costResult] = await Promise.all([
        listSuppliers({
          q: filters.q || undefined,
          kind: filters.kind || undefined,
          type: filters.type || undefined,
          runtime_status: filters.runtime_status || undefined,
          health_status: filters.health_status || undefined,
          page: pagination.page,
          page_size: pagination.page_size
        }),
        listSupplierCostSnapshots({ page: 1, page_size: 200 })
      ])
      suppliers.value = result.items
      supplierCostSnapshots.value = preferredSupplierCostSnapshots(result.items, costResult.items)
      await loadBestChannelChecks(result.items.map((item) => item.id), { replace: true })
      pagination.total = result.total || 0
      pagination.pages = result.pages || 0
      pagination.page = result.page || pagination.page
      pagination.page_size = result.page_size || pagination.page_size
      openDeepLinkedDialog()
    } catch (error) {
      appStore.showError((error as { message?: string }).message || '加载供应商失败')
    } finally {
      loading.value = false
    }
  }

  async function loadBestChannelChecks(supplierIds: number[] = suppliers.value.map((item) => item.id), options: LoadBestChannelChecksOptions = {}) {
    if (supplierIds.length === 0) {
      if (options.replace) {
        supplierBestChannels.value = {}
      }
      return
    }
    try {
      const result = await listSupplierBestChannelChecks(supplierIds)
      const next = options.replace ? {} : { ...supplierBestChannels.value }
      for (const supplierID of supplierIds) {
        delete next[supplierID]
      }
      for (const item of result.items) {
        const existing = next[item.supplier_id] || []
        const protocol = channelProtocol(item)
        next[item.supplier_id] = [
          ...existing.filter((snapshot) => channelProtocol(snapshot) !== protocol),
          item
        ].sort(compareChannelProtocolSnapshots)
      }
      supplierBestChannels.value = next
    } catch {
      if (options.replace) {
        supplierBestChannels.value = {}
      }
    }
  }

  function openScheduleListDialog() {
    moreMenuOpen.value = false
    scheduleListDialogOpen.value = true
    void loadScheduleList()
  }

  function closeScheduleListDialog() {
    if (scheduleListActionKey.value) return
    scheduleListDialogOpen.value = false
  }

  async function loadScheduleList() {
    if (scheduleListLoading.value) return
    scheduleListLoading.value = true
    scheduleListError.value = ''
    try {
      const supplierResult = await listSuppliers({ page: 1, page_size: 1000 })
      const supplierItems = supplierResult.items || []
      scheduleListSuppliers.value = supplierItems
      if (supplierItems.length === 0) {
        scheduleListBindings.value = []
        scheduleListLocalAccounts.value = {}
        scheduleListChannelChecks.value = {}
        return
      }

      const [localResult, accountResults, channelResults] = await Promise.all([
        listLocalSub2APIAccounts({ page: 1, page_size: 1000 }),
        Promise.all(supplierItems.map((supplier) => listSupplierAccounts(supplier.id, { page: 1, page_size: 1000 }))),
        Promise.all(supplierItems.map((supplier) => listSupplierChannelChecks(supplier.id, { page: 1, page_size: 300 })))
      ])

      scheduleListBindings.value = accountResults.flatMap((result) => result.items)
      scheduleListLocalAccounts.value = Object.fromEntries(localResult.items.map((account) => [account.id, account]))

      const nextChecks: Record<string, SupplierChannelCheckSnapshot | undefined> = {}
      for (const result of channelResults) {
        for (const snapshot of result.items) {
          const key = scheduleChannelKey(snapshot.supplier_id, snapshot.supplier_group_id)
          const existing = nextChecks[key]
          if (!existing || Date.parse(snapshot.captured_at || '') > Date.parse(existing.captured_at || '')) {
            nextChecks[key] = snapshot
          }
        }
      }
      scheduleListChannelChecks.value = nextChecks
    } catch (error) {
      scheduleListError.value = (error as { message?: string }).message || '加载调度列表失败'
      appStore.showError(scheduleListError.value)
    } finally {
      scheduleListLoading.value = false
    }
  }

  function upsertScheduleListSnapshots(items: SupplierChannelCheckSnapshot[]) {
    if (items.length === 0) return
    const next = { ...scheduleListChannelChecks.value }
    for (const item of items) {
      next[scheduleChannelKey(item.supplier_id, item.supplier_group_id)] = item
    }
    scheduleListChannelChecks.value = next
  }

  function updateScheduleListLocalAccount(row: ScheduleListRow, schedulable: boolean) {
    const localAccount = scheduleListLocalAccounts.value[row.local_account_id]
    if (!localAccount) return
    scheduleListLocalAccounts.value = {
      ...scheduleListLocalAccounts.value,
      [row.local_account_id]: {
        ...localAccount,
        schedulable
      }
    }
  }

  async function toggleScheduleRow(row: ScheduleListRow) {
    if (scheduleListActionKey.value) return
    if (!row.supplier_group_id) {
      appStore.showError('该账号缺少供应商分组，无法切换调度')
      return
    }
    scheduleListActionKey.value = row.key
    scheduleListError.value = ''
    const nextSchedulable = !row.schedulable
    try {
      const snapshot = nextSchedulable
        ? await enableSupplierChannelScheduling(row.supplier_id, row.supplier_group_id)
        : await pauseSupplierChannelScheduling(row.supplier_id, row.supplier_group_id)
      upsertScheduleListSnapshots([snapshot])
      updateScheduleListLocalAccount(row, snapshot.local_account_schedulable)
      mergeChannelCheckSnapshots([snapshot])
      await loadBestChannelChecks([row.supplier_id])
      appStore.showSuccess(nextSchedulable ? '已加入本地调度' : '已暂停本地调度')
    } catch (error) {
      scheduleListError.value = (error as { message?: string }).message || (nextSchedulable ? '加入调度失败' : '暂停调度失败')
      appStore.showError(scheduleListError.value)
    } finally {
      scheduleListActionKey.value = ''
    }
  }

  async function probeScheduleRow(row: ScheduleListRow) {
    if (scheduleListActionKey.value) return
    if (!row.supplier_group_id) {
      appStore.showError('该账号缺少供应商分组，无法复测')
      return
    }
    scheduleListActionKey.value = row.key
    scheduleListError.value = ''
    try {
      const result = await probeSupplierChannel(row.supplier_id, {
        supplier_group_id: row.supplier_group_id,
        auto_pause_on_failure: true
      })
      upsertScheduleListSnapshots(result.items)
      mergeChannelCheckSnapshots(result.items)
      const current = result.items.find((item) => item.supplier_group_id === row.supplier_group_id)
      if (current) {
        updateScheduleListLocalAccount(row, current.local_account_schedulable)
      }
      await loadBestChannelChecks([row.supplier_id])
      appStore.showSuccess('渠道复测完成')
    } catch (error) {
      scheduleListError.value = (error as { message?: string }).message || '渠道复测失败'
      appStore.showError(scheduleListError.value)
    } finally {
      scheduleListActionKey.value = ''
    }
  }

  function openScheduleRowGroups(row: ScheduleListRow) {
    const supplier = scheduleListSuppliers.value.find((item) => item.id === row.supplier_id) || suppliers.value.find((item) => item.id === row.supplier_id)
    if (!supplier) return
    scheduleListDialogOpen.value = false
    openGroupsDialog(supplier)
  }

  function openDeepLinkedDialog() {
    if (route.query.open !== 'groups') return
    const supplierID = Number(route.query.supplier_id || 0)
    if (!supplierID) return
    const deepLinkKey = `${route.query.open}:${supplierID}`
    if (handledDeepLinkKey.value === deepLinkKey) return
    const supplier = suppliers.value.find((item) => item.id === supplierID)
    if (!supplier) return
    handledDeepLinkKey.value = deepLinkKey
    openGroupsDialog(supplier)
  }

  async function reloadCurrentSession() {
    if (!sessionSupplier.value) return
    sessionLoading.value = true
    sessionLoadError.value = ''
    try {
      sessionStore[sessionSupplier.value.id] = await getSupplierSession(sessionSupplier.value.id)
    } catch (error) {
      sessionStore[sessionSupplier.value.id] = undefined
      sessionLoadError.value = (error as { message?: string }).message || '当前供应商还没有浏览器会话'
    } finally {
      sessionLoading.value = false
    }
  }

  async function reloadCurrentBalance(refresh: boolean) {
    if (!sessionSupplier.value) return
    currentBalanceLoading.value = true
    currentBalanceError.value = ''
    try {
      const balance = await getSupplierCurrentBalance(sessionSupplier.value.id, { refresh })
      currentBalanceStore[sessionSupplier.value.id] = balance
      currentBalanceError.value = balance.fallback ? balance.refresh_error_message || '读取当前余额失败' : ''
      if (refresh) {
        await loadSuppliers()
      }
    } catch (error) {
      currentBalanceError.value = (error as { message?: string }).message || '读取当前余额失败'
    } finally {
      currentBalanceLoading.value = false
    }
  }

  async function reloadGroupSession() {
    if (!groupsSupplier.value) return
    try {
      sessionStore[groupsSupplier.value.id] = await getSupplierSession(groupsSupplier.value.id)
    } catch {
      sessionStore[groupsSupplier.value.id] = undefined
    }
  }

  async function loadChannelStatus() {
    if (!channelStatusSupplier.value) return
    if (channelStatusSupplier.value.type !== 'sub2api' && channelStatusSupplier.value.type !== 'new_api') {
      channelStatusError.value = '当前仅支持读取 Sub2API 或 New API 类型供应商的渠道状态。'
      channelMonitorItems.value = []
      return
    }
    channelStatusLoading.value = true
    channelStatusError.value = ''
    try {
      const result = await listSupplierChannelMonitors(channelStatusSupplier.value.id)
      channelMonitorItems.value = result.items || []
      channelMonitorCapturedAt.value = result.captured_at || ''
      channelMonitorOrigin.value = result.origin || ''
      channelMonitorAPIBaseURL.value = result.api_base_url || ''
      channelStatusCountdown.value = 16
    } catch (error) {
      channelMonitorItems.value = []
      channelStatusError.value = channelStatusErrorMessage(error)
    } finally {
      channelStatusLoading.value = false
    }
  }

  function reloadFirstPage() {
    pagination.page = 1
    void loadSuppliers()
  }

  function handlePageChange(page: number) {
    pagination.page = page
    void loadSuppliers()
  }

  function handlePageSizeChange(pageSize: number) {
    pagination.page_size = pageSize
    pagination.page = 1
    void loadSuppliers()
  }

  function handleGroupPageChange(page: number) {
    groupPagination.page = page
    void loadCurrentGroups()
  }

  function handleGroupPageSizeChange(pageSize: number) {
    groupPagination.page_size = pageSize
    groupPagination.page = 1
    void loadCurrentGroups()
  }

  Object.assign(ctx, {
    loadSuppliers,
    loadBestChannelChecks,
    openScheduleListDialog,
    closeScheduleListDialog,
    loadScheduleList,
    upsertScheduleListSnapshots,
    updateScheduleListLocalAccount,
    toggleScheduleRow,
    probeScheduleRow,
    openScheduleRowGroups,
    openDeepLinkedDialog,
    reloadCurrentSession,
    reloadCurrentBalance,
    reloadGroupSession,
    loadChannelStatus,
    reloadFirstPage,
    handlePageChange,
    handlePageSizeChange,
    handleGroupPageChange,
    handleGroupPageSizeChange
  })
}
