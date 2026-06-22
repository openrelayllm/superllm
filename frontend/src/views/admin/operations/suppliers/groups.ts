import { groupsAPI } from '@/api/admin/groups'
import { enableSupplierChannelScheduling, getSupplierProvisionJob, listSupplierChannelChecks, listSupplierKeys, listSupplierGroups, pauseSupplierChannelScheduling, probeSupplierChannel, provisionSupplierKey, syncSupplierChannelChecks } from '@/api/admin/adminPlus'
import type { AdminGroup } from '@/types'
import type { Supplier, SupplierChannelCheckSnapshot, SupplierGroup, SupplierProvisionStatus } from '@/api/admin/adminPlus'
import type { QuickProvisionBestChannelOptions } from './types'
import { ctxFn, ctxValue } from './ctxProxy'
export function attachSupplierGroups(ctx: any) {
  const appStore = ctxValue(ctx, 'appStore')
  const channelScheduleDialogOpen = ctxValue(ctx, 'channelScheduleDialogOpen')
  const moreMenuOpen = ctxValue(ctx, 'moreMenuOpen')
  const bulkChannelChecksSyncing = ctxValue(ctx, 'bulkChannelChecksSyncing')
  const groupsSupplier = ctxValue(ctx, 'groupsSupplier')
  const channelScheduleSupplier = ctxValue(ctx, 'channelScheduleSupplier')
  const supplierGroups = ctxValue(ctx, 'supplierGroups')
  const supplierKeys = ctxValue(ctx, 'supplierKeys')
  const supplierChannelChecks = ctxValue(ctx, 'supplierChannelChecks')
  const groupsLoading = ctxValue(ctx, 'groupsLoading')
  const channelChecksSyncing = ctxValue(ctx, 'channelChecksSyncing')
  const channelScheduleSubmitting = ctxValue(ctx, 'channelScheduleSubmitting')
  const groupsError = ctxValue(ctx, 'groupsError')
  const provisionJobError = ctxValue(ctx, 'provisionJobError')
  const channelCheckError = ctxValue(ctx, 'channelCheckError')
  const rowChannelCheckSupplierID = ctxValue(ctx, 'rowChannelCheckSupplierID')
  const channelCheckActionKey = ctxValue(ctx, 'channelCheckActionKey')
  const localOpenAIGroups = ctxValue(ctx, 'localOpenAIGroups')
  const groupPagination = ctxValue(ctx, 'groupPagination')
  const groupFilters = ctxValue(ctx, 'groupFilters')
  const selectedCount = ctxValue(ctx, 'selectedCount')
  const selectedRows = ctxValue(ctx, 'selectedRows')
  const channelCostMultiplier = ctxFn(ctx, 'channelCostMultiplier')
  const supplierBestChannel = ctxFn(ctx, 'supplierBestChannel')
  const channelProtocol = ctxFn(ctx, 'channelProtocol')
  const upsertSupplierBestChannelSnapshot = ctxFn(ctx, 'upsertSupplierBestChannelSnapshot')
  const groupChannelCheck = ctxFn(ctx, 'groupChannelCheck')
  const limeProvisionActionKey = ctxFn(ctx, 'limeProvisionActionKey')
  const channelHasLocalBinding = ctxFn(ctx, 'channelHasLocalBinding')
  const channelIsAvailable = ctxFn(ctx, 'channelIsAvailable')
  const groupHasLocalBinding = ctxFn(ctx, 'groupHasLocalBinding')
  const errorMessage = ctxFn(ctx, 'errorMessage')
  const loadBestChannelChecks = ctxFn(ctx, 'loadBestChannelChecks')
  const openGroupsDialog = ctxFn(ctx, 'openGroupsDialog')
  const openProvisionDialog = ctxFn(ctx, 'openProvisionDialog')
  const normalizeLocalPlatform = ctxFn(ctx, 'normalizeLocalPlatform')
  const defaultProviderBaseURL = ctxFn(ctx, 'defaultProviderBaseURL')
  const watchProvisionJob = ctxFn(ctx, 'watchProvisionJob')
  const isTerminalProvisionJobStatus = ctxFn(ctx, 'isTerminalProvisionJobStatus')
  async function loadCurrentGroups() {
    if (!groupsSupplier.value) return
    groupsLoading.value = true
    groupsError.value = ''
    try {
      const [result, keyResult] = await Promise.all([
        listSupplierGroups(groupsSupplier.value.id, {
          q: groupFilters.q || undefined,
          status: groupFilters.status || undefined,
          page: groupPagination.page,
          page_size: groupPagination.page_size
        }),
        listSupplierKeys(groupsSupplier.value.id, {
          page: 1,
          page_size: 1000
        }),
        loadCurrentChannelChecks()
      ])
      supplierGroups.value = result.items
      supplierKeys.value = keyResult.items
      groupPagination.total = result.total || 0
      groupPagination.pages = result.pages || 0
      groupPagination.page = result.page || groupPagination.page
      groupPagination.page_size = result.page_size || groupPagination.page_size
    } catch (error) {
      groupsError.value = (error as { message?: string }).message || '加载供应商分组失败'
    } finally {
      groupsLoading.value = false
    }
  }

  async function loadCurrentChannelChecks() {
    if (!groupsSupplier.value) return
    try {
      const result = await listSupplierChannelChecks(groupsSupplier.value.id, { page: 1, page_size: 500 })
      const latestByGroup = new Map<number, SupplierChannelCheckSnapshot>()
      for (const item of result.items) {
        const existing = latestByGroup.get(item.supplier_group_id)
        const itemTime = new Date(item.captured_at).getTime()
        const existingTime = existing ? new Date(existing.captured_at).getTime() : 0
        if (!existing || itemTime > existingTime || (itemTime === existingTime && item.id > existing.id)) {
          latestByGroup.set(item.supplier_group_id, item)
        }
      }
      supplierChannelChecks.value = Object.fromEntries(latestByGroup) as Record<number, SupplierChannelCheckSnapshot>
    } catch {
      supplierChannelChecks.value = {}
    }
  }

  function mergeChannelCheckSnapshots(items: SupplierChannelCheckSnapshot[]) {
    const next = { ...supplierChannelChecks.value }
    for (const item of items) {
      next[item.supplier_group_id] = item
    }
    supplierChannelChecks.value = next
    const best = items.find((item) => item.recommended)
    if (best) {
      upsertSupplierBestChannelSnapshot(best)
    }
  }

  async function syncCurrentChannelChecks() {
    if (!groupsSupplier.value || channelChecksSyncing.value) return
    channelChecksSyncing.value = true
    channelCheckError.value = ''
    provisionJobError.value = ''
    try {
      const job = await syncSupplierChannelChecks(groupsSupplier.value.id, {
        candidate_limit: 3,
        auto_pause_on_failure: true
      })
      appStore.showSuccess(`渠道检测任务已提交 #${job.job_id}`)
      await watchProvisionJob(job.job_id)
    } catch (error) {
      channelCheckError.value = (error as { message?: string }).message || '渠道检测任务提交失败'
      appStore.showError(channelCheckError.value)
    } finally {
      channelChecksSyncing.value = false
    }
  }

  async function syncSupplierChannelFromRow(supplier: Supplier) {
    if (rowChannelCheckSupplierID.value) return
    rowChannelCheckSupplierID.value = supplier.id
    try {
      const job = await syncSupplierChannelChecks(supplier.id, {
        candidate_limit: 3,
        auto_pause_on_failure: true
      })
      appStore.showSuccess(`渠道检测任务已提交 #${job.job_id}`)
      void refreshChannelCheckAfterJob(supplier.id, job.job_id)
    } catch (error) {
      appStore.showError((error as { message?: string }).message || '渠道检测任务提交失败')
    } finally {
      rowChannelCheckSupplierID.value = null
    }
  }

  async function syncSelectedChannelChecks() {
    if (selectedCount.value === 0 || bulkChannelChecksSyncing.value) return
    const targets = [...selectedRows.value]
    if (targets.length === 0) {
      appStore.showWarning('当前页没有可检测的已选供应商')
      return
    }

    moreMenuOpen.value = false
    bulkChannelChecksSyncing.value = true
    let submitted = 0
    let failed = 0
    const jobs: Array<{ supplierID: number; jobID: number }> = []
    const failures: string[] = []

    try {
      for (const supplier of targets) {
        try {
          const job = await syncSupplierChannelChecks(supplier.id, {
            candidate_limit: 3,
            auto_pause_on_failure: true
          })
          submitted++
          jobs.push({ supplierID: supplier.id, jobID: job.job_id })
        } catch (error) {
          failed++
          failures.push(`${supplier.name}: ${(error as { message?: string }).message || '提交失败'}`)
        }
      }

      const failureText = failures.length > 0
        ? `；${failures.slice(0, 3).join('；')}${failures.length > 3 ? ` 等 ${failures.length} 项` : ''}`
        : ''

      if (submitted > 0) {
        appStore.showSuccess(`渠道检测任务已提交：成功 ${submitted}${failed > 0 ? `，失败 ${failed}${failureText}` : ''}`, failed > 0 ? 7000 : undefined)
        void refreshChannelChecksAfterJobs(jobs)
        return
      }
      appStore.showError(`渠道检测任务提交失败：失败 ${failed}${failureText}`, 8000)
    } finally {
      bulkChannelChecksSyncing.value = false
    }
  }

  async function refreshChannelCheckAfterJob(supplierID: number, jobID: number) {
    await waitProvisionJobTerminal(jobID)
    await loadBestChannelChecks([supplierID])
  }

  async function refreshChannelChecksAfterJobs(jobs: Array<{ supplierID: number; jobID: number }>) {
    if (jobs.length === 0) return
    await Promise.allSettled(jobs.map((job) => waitProvisionJobTerminal(job.jobID)))
    await loadBestChannelChecks(jobs.map((job) => job.supplierID))
  }

  async function waitProvisionJobTerminal(jobID: number) {
    const deadline = Date.now() + 120_000
    while (Date.now() < deadline) {
      const job = await getSupplierProvisionJob(jobID)
      if (isTerminalProvisionJobStatus(job.status)) {
        return job
      }
      await sleep(2000)
    }
    return null
  }

  function sleep(ms: number) {
    return new Promise((resolve) => window.setTimeout(resolve, ms))
  }

  async function probeGroupChannel(group: SupplierGroup) {
    if (!groupsSupplier.value || channelCheckActionKey.value) return
    const actionKey = `probe:${group.id}`
    channelCheckActionKey.value = actionKey
    channelCheckError.value = ''
    try {
      const result = await probeSupplierChannel(groupsSupplier.value.id, {
        supplier_group_id: group.id,
        auto_pause_on_failure: true
      })
      mergeChannelCheckSnapshots(result.items)
      await loadBestChannelChecks([groupsSupplier.value.id])
      appStore.showSuccess('渠道复测完成')
    } catch (error) {
      channelCheckError.value = (error as { message?: string }).message || '渠道复测失败'
      appStore.showError(channelCheckError.value)
    } finally {
      if (channelCheckActionKey.value === actionKey) {
        channelCheckActionKey.value = ''
      }
    }
  }

  async function setGroupChannelScheduling(group: SupplierGroup, schedulable: boolean) {
    if (!groupsSupplier.value || channelCheckActionKey.value) return
    const actionKey = `schedule:${group.id}`
    channelCheckActionKey.value = actionKey
    channelCheckError.value = ''
    try {
      const snapshot = schedulable
        ? await enableSupplierChannelScheduling(groupsSupplier.value.id, group.id)
        : await pauseSupplierChannelScheduling(groupsSupplier.value.id, group.id)
      mergeChannelCheckSnapshots([snapshot])
      await loadBestChannelChecks([groupsSupplier.value.id])
      appStore.showSuccess(schedulable ? '已加入本地调度' : '已暂停本地调度')
    } catch (error) {
      channelCheckError.value = (error as { message?: string }).message || (schedulable ? '加入调度失败' : '暂停调度失败')
      appStore.showError(channelCheckError.value)
    } finally {
      if (channelCheckActionKey.value === actionKey) {
        channelCheckActionKey.value = ''
      }
    }
  }

  async function handleGroupScheduleAction(group: SupplierGroup) {
    const check = groupChannelCheck(group.id)
    if (!groupHasLocalBinding(group)) {
      openProvisionDialog(group)
      return
    }
    if (check?.local_account_schedulable) {
      await setGroupChannelScheduling(group, false)
      return
    }
    if (!channelIsAvailable(check)) {
      appStore.showWarning('请先复测通过，再加入本地调度')
      return
    }
    await setGroupChannelScheduling(group, true)
  }

  function openBestChannelScheduleDialog(supplier: Supplier) {
    channelScheduleSupplier.value = supplier
    channelScheduleDialogOpen.value = true
  }

  function closeBestChannelScheduleDialog() {
    if (channelScheduleSubmitting.value) return
    channelScheduleDialogOpen.value = false
    channelScheduleSupplier.value = null
  }

  function openChannelScheduleGroups() {
    const supplier = channelScheduleSupplier.value
    if (!supplier) return
    channelScheduleDialogOpen.value = false
    channelScheduleSupplier.value = null
    openGroupsDialog(supplier)
  }

  async function setBestChannelScheduling(supplier: Supplier, schedulable: boolean) {
    const current = supplierBestChannel(supplier.id)
    if (!current) {
      appStore.showError('请先完成渠道检测，再加入调度')
      return
    }
    if (channelCheckActionKey.value) return

    const actionKey = `best-schedule:${supplier.id}`
    channelCheckActionKey.value = actionKey
    try {
      const snapshot = schedulable
        ? await enableSupplierChannelScheduling(supplier.id, current.supplier_group_id)
        : await pauseSupplierChannelScheduling(supplier.id, current.supplier_group_id)
      upsertSupplierBestChannelSnapshot(snapshot)
      if (groupsSupplier.value?.id === supplier.id) {
        supplierChannelChecks.value = {
          ...supplierChannelChecks.value,
          [snapshot.supplier_group_id]: snapshot
        }
      }
      appStore.showSuccess(schedulable ? '已加入本地调度' : '已暂停本地调度')
    } catch (error) {
      appStore.showError((error as { message?: string }).message || (schedulable ? '加入调度失败' : '暂停调度失败'))
      await loadBestChannelChecks([supplier.id])
    } finally {
      if (channelCheckActionKey.value === actionKey) {
        channelCheckActionKey.value = ''
      }
    }
  }

  async function confirmChannelSchedulePrimaryAction() {
    const supplier = channelScheduleSupplier.value
    if (!supplier || channelScheduleSubmitting.value) return
    channelScheduleSubmitting.value = true
    try {
      await handleBestChannelScheduleAction(supplier)
    } finally {
      channelScheduleSubmitting.value = false
    }
  }

  async function handleBestChannelScheduleAction(supplier: Supplier) {
    const current = supplierBestChannel(supplier.id)
    if (!current) {
      await syncSupplierChannelFromRow(supplier)
      return
    }
    if (!channelHasLocalBinding(current)) {
      await quickProvisionBestChannel(supplier, current)
      return
    }
    if (current.local_account_schedulable) {
      await setBestChannelScheduling(supplier, false)
      return
    }
    if (!channelIsAvailable(current)) {
      await probeAndScheduleBestChannel(supplier, current)
      return
    }
    await setBestChannelScheduling(supplier, true)
  }

  async function probeAndScheduleBestChannel(supplier: Supplier, snapshot: SupplierChannelCheckSnapshot) {
    if (channelCheckActionKey.value) return
    const actionKey = `best-schedule:${supplier.id}`
    channelCheckActionKey.value = actionKey
    try {
      const result = await probeSupplierChannel(supplier.id, {
        supplier_group_id: snapshot.supplier_group_id,
        auto_pause_on_failure: false
      })
      mergeChannelCheckSnapshots(result.items)
      const checked = result.items.find((item) => item.supplier_group_id === snapshot.supplier_group_id)
      if (!channelIsAvailable(checked)) {
        appStore.showWarning(checked?.error_message || '渠道复测未通过，暂未加入调度')
        await loadBestChannelChecks([supplier.id])
        return
      }
      const scheduled = await enableSupplierChannelScheduling(supplier.id, snapshot.supplier_group_id)
      upsertSupplierBestChannelSnapshot(scheduled)
      appStore.showSuccess('渠道复测通过，已加入本地调度')
    } catch (error) {
      appStore.showError((error as { message?: string }).message || '复测并加入调度失败')
      await loadBestChannelChecks([supplier.id])
    } finally {
      if (channelCheckActionKey.value === actionKey) {
        channelCheckActionKey.value = ''
      }
    }
  }

  async function quickProvisionBestChannel(
    supplier: Supplier,
    snapshot: SupplierChannelCheckSnapshot,
    options: QuickProvisionBestChannelOptions = {}
  ) {
    if (channelCheckActionKey.value) return
    const baseURL = defaultProviderBaseURL(supplier)
    if (!baseURL) {
      appStore.showError('未配置供应商 API Base URL，无法开通本地账号')
      return
    }

    const actionKey = options.actionKey || `best-schedule:${supplier.id}`
    channelCheckActionKey.value = actionKey
    try {
      const job = await provisionSupplierKey(supplier.id, {
        supplier_group_id: snapshot.supplier_group_id,
        name: defaultChannelProvisionName(supplier, snapshot),
        quota_usd: 0,
        expires_in_days: null,
        local_account_platform: normalizeLocalPlatform(snapshot.provider_family),
        local_account_name: defaultChannelProvisionName(supplier, snapshot),
        local_account_base_url: baseURL,
        local_account_concurrency: 2,
        local_account_priority: 100,
        local_account_rate_multiplier: channelCostMultiplier(snapshot) || 1,
        local_account_group_ids: options.localGroupIDs,
        runtime_status: 'monitor_only',
        health_status: 'normal',
        balance_threshold_cents: 0,
        balance_cents: Math.max(0, supplier.balance_cents || 0),
        balance_currency: supplier.balance_currency || 'USD'
      })
      appStore.showSuccess(options.submittedMessage?.(job.job_id) || `账号开通任务已提交 #${job.job_id}`)
      const finished = await waitProvisionJobTerminal(job.job_id)
      if (!finished || !isSuccessfulProvisionJobStatus(finished.status)) {
        appStore.showError(finished?.error_message || '账号开通未完成，暂未加入调度')
        return
      }
      const probe = await probeSupplierChannel(supplier.id, {
        supplier_group_id: snapshot.supplier_group_id,
        auto_pause_on_failure: false
      })
      mergeChannelCheckSnapshots(probe.items)
      const checked = probe.items.find((item) => item.supplier_group_id === snapshot.supplier_group_id)
      if (!channelIsAvailable(checked)) {
        appStore.showWarning(checked?.error_message || '渠道复测未通过，暂未加入调度')
        await loadBestChannelChecks([supplier.id])
        return
      }
      const scheduled = await enableSupplierChannelScheduling(supplier.id, snapshot.supplier_group_id)
      upsertSupplierBestChannelSnapshot(scheduled)
      appStore.showSuccess(options.completedMessage || '已开通账号并加入本地调度')
    } catch (error) {
      appStore.showError((error as { message?: string }).message || '开通账号并加入调度失败')
      await loadBestChannelChecks([supplier.id])
    } finally {
      if (channelCheckActionKey.value === actionKey) {
        channelCheckActionKey.value = ''
      }
    }
  }

  async function loadLocalOpenAIGroups(): Promise<AdminGroup[]> {
    if (localOpenAIGroups.value.length > 0) return localOpenAIGroups.value
    localOpenAIGroups.value = await groupsAPI.getAll('openai')
    return localOpenAIGroups.value
  }

  async function findLocalLimeOpenAIGroup(): Promise<AdminGroup | undefined> {
    const groups = await loadLocalOpenAIGroups()
    const activeGroups = groups.filter((group) => group.platform === 'openai' && group.status === 'active')
    const exact = activeGroups.find((group) => group.name.trim().toLowerCase() === 'lime')
    if (exact) return exact
    return activeGroups.find((group) => group.name.trim().toLowerCase().includes('lime'))
  }

  async function quickProvisionBestChannelToLime(supplier: Supplier, snapshot: SupplierChannelCheckSnapshot) {
    if (channelCheckActionKey.value) return
    if (channelProtocol(snapshot) !== 'openai') {
      appStore.showWarning('Lime 快捷加入仅适用于 OpenAI 协议渠道')
      return
    }
    if (channelHasLocalBinding(snapshot)) {
      appStore.showWarning('该渠道已绑定本地账号；如需补充 Lime 分组，请在本地账号管理中调整分组')
      return
    }

    let limeGroup: AdminGroup | undefined
    try {
      limeGroup = await findLocalLimeOpenAIGroup()
    } catch (error) {
      appStore.showError(errorMessage(error, '读取本地 OpenAI 分组失败'))
      return
    }
    if (!limeGroup) {
      appStore.showError('未找到本地 Lime(OpenAI) 分组')
      return
    }

    await quickProvisionBestChannel(supplier, snapshot, {
      actionKey: limeProvisionActionKey(supplier.id, snapshot.supplier_group_id),
      localGroupIDs: [limeGroup.id],
      submittedMessage: (jobID) => `账号开通任务已提交 #${jobID}，已指定 Lime(OpenAI) 分组`,
      completedMessage: '已开通账号、绑定 Lime(OpenAI) 分组并加入本地调度'
    })
  }

  function defaultChannelProvisionName(supplier: Supplier, snapshot: SupplierChannelCheckSnapshot): string {
    return [supplier.name, snapshot.group_name].filter(Boolean).join('-') || `supplier-channel-${snapshot.supplier_group_id}`
  }

  function isSuccessfulProvisionJobStatus(status?: SupplierProvisionStatus): boolean {
    return status === 'succeeded' || status === 'partial_succeeded'
  }

  Object.assign(ctx, {
    loadCurrentGroups,
    loadCurrentChannelChecks,
    mergeChannelCheckSnapshots,
    syncCurrentChannelChecks,
    syncSupplierChannelFromRow,
    syncSelectedChannelChecks,
    refreshChannelCheckAfterJob,
    refreshChannelChecksAfterJobs,
    waitProvisionJobTerminal,
    sleep,
    probeGroupChannel,
    setGroupChannelScheduling,
    handleGroupScheduleAction,
    openBestChannelScheduleDialog,
    closeBestChannelScheduleDialog,
    openChannelScheduleGroups,
    setBestChannelScheduling,
    confirmChannelSchedulePrimaryAction,
    handleBestChannelScheduleAction,
    probeAndScheduleBestChannel,
    quickProvisionBestChannel,
    loadLocalOpenAIGroups,
    findLocalLimeOpenAIGroup,
    quickProvisionBestChannelToLime,
    defaultChannelProvisionName,
    isSuccessfulProvisionJobStatus
  })
}
