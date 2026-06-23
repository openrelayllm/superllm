import { ensureSupplierKeys, getSupplierProvisionJob, listLocalSub2APIAccounts, provisionSupplierKey, repairSupplierKeyBinding, standardizeSupplierKeyNames, syncSupplierGroups } from '@/api/admin/adminPlus'
import type { Supplier, SupplierGroup, SupplierKey, SupplierProvisionStatus } from '@/api/admin/adminPlus'
import { ctxFn, ctxValue } from './ctxProxy'
export function attachSupplierProvision(ctx: any) {
  const appStore = ctxValue(ctx, 'appStore')
  const provisionSubmitting = ctxValue(ctx, 'provisionSubmitting')
  const repairSubmitting = ctxValue(ctx, 'repairSubmitting')
  const provisionDialogOpen = ctxValue(ctx, 'provisionDialogOpen')
  const repairDialogOpen = ctxValue(ctx, 'repairDialogOpen')
  const groupsSupplier = ctxValue(ctx, 'groupsSupplier')
  const provisionGroup = ctxValue(ctx, 'provisionGroup')
  const repairKey = ctxValue(ctx, 'repairKey')
  const activeProvisionJob = ctxValue(ctx, 'activeProvisionJob')
  const localAccounts = ctxValue(ctx, 'localAccounts')
  const groupsSyncing = ctxValue(ctx, 'groupsSyncing')
  const keysEnsuring = ctxValue(ctx, 'keysEnsuring')
  const keyNamesStandardizing = ctxValue(ctx, 'keyNamesStandardizing')
  const repairAccountsLoading = ctxValue(ctx, 'repairAccountsLoading')
  const groupsError = ctxValue(ctx, 'groupsError')
  const provisionError = ctxValue(ctx, 'provisionError')
  const provisionJobError = ctxValue(ctx, 'provisionJobError')
  const repairError = ctxValue(ctx, 'repairError')
  const provisionJobTimer = ctxValue(ctx, 'provisionJobTimer')
  const groupPagination = ctxValue(ctx, 'groupPagination')
  const provisionForm = ctxValue(ctx, 'provisionForm')
  const keyNamingForm = ctxValue(ctx, 'keyNamingForm')
  const repairForm = ctxValue(ctx, 'repairForm')
  const currentGroupSession = ctxValue(ctx, 'currentGroupSession')
  const centsFromYuan = ctxFn(ctx, 'centsFromYuan')
  const yuanFromCents = ctxFn(ctx, 'yuanFromCents')
  const groupCostMultiplier = ctxFn(ctx, 'groupCostMultiplier')
  const isSwitchableRuntimeStatus = ctxFn(ctx, 'isSwitchableRuntimeStatus')
  const loadSuppliers = ctxFn(ctx, 'loadSuppliers')
  const reloadGroupSession = ctxFn(ctx, 'reloadGroupSession')
  const loadCurrentGroups = ctxFn(ctx, 'loadCurrentGroups')
  function openProvisionDialog(group: SupplierGroup) {
    if (!groupsSupplier.value) return
    if (!currentGroupSession.value?.has_encrypted_bundle) {
      appStore.showError('当前供应商还没有可用会话，请先后端直登或通过插件兜底上报')
      return
    }
    provisionGroup.value = group
    provisionError.value = ''
    fillProvisionForm(group)
    provisionDialogOpen.value = true
  }

  function closeProvisionDialog() {
    provisionDialogOpen.value = false
    provisionGroup.value = null
    provisionError.value = ''
  }

  function openRepairDialog(key?: SupplierKey) {
    if (!key || !groupsSupplier.value) return
    repairKey.value = key
    repairError.value = ''
    fillRepairForm(key)
    repairDialogOpen.value = true
    void loadRepairLocalAccounts()
  }

  function closeRepairDialog() {
    repairDialogOpen.value = false
    repairKey.value = null
    repairError.value = ''
    repairForm.local_sub2api_account_id = 0
  }

  function fillProvisionForm(group: SupplierGroup) {
    const supplier = groupsSupplier.value
    const name = standardProvisionName(supplier, group)
    provisionForm.name = name
    provisionForm.sync_provider_name = false
    provisionForm.local_account_name = name
    provisionForm.local_account_platform = normalizeLocalPlatform(group.provider_family)
    provisionForm.local_account_base_url = defaultProviderBaseURL(supplier)
    provisionForm.local_account_concurrency = Number(group.rpm_limit || 0)
    provisionForm.local_account_priority = 100
    provisionForm.local_account_rate_multiplier = groupCostMultiplier(group) || 1
    provisionForm.quota_usd = 0
    provisionForm.expires_in_days = null
    provisionForm.runtime_status = 'monitor_only'
    provisionForm.health_status = 'normal'
    provisionForm.balance_yuan = yuanFromCents(supplier?.balance_cents || 0)
    provisionForm.balance_threshold_yuan = 0
    provisionForm.balance_currency = supplier?.balance_currency || 'USD'
  }

  function fillRepairForm(key: SupplierKey) {
    const supplier = groupsSupplier.value
    repairForm.local_sub2api_account_id = key.local_sub2api_account_id || 0
    repairForm.runtime_status = 'monitor_only'
    repairForm.health_status = 'normal'
    repairForm.configured_concurrency = 0
    repairForm.balance_yuan = yuanFromCents(supplier?.balance_cents || 0)
    repairForm.balance_threshold_yuan = 0
    repairForm.balance_currency = supplier?.balance_currency || 'USD'
  }

  function defaultProvisionName(supplier: Supplier | null, group: SupplierGroup): string {
    return [supplier?.name, group.name].filter(Boolean).join('-') || `supplier-group-${group.id}`
  }

  function standardProvisionName(supplier: Supplier | null, group: SupplierGroup): string {
    const standardName = String(group.standard_key_name || '').trim()
    return standardName || defaultProvisionName(supplier, group)
  }

  function normalizeLocalPlatform(providerFamily?: string): string {
    const value = String(providerFamily || '').toLowerCase()
    if (value.includes('anthropic') || value.includes('claude')) return 'anthropic'
    if (value.includes('gemini') || value.includes('google')) return 'gemini'
    if (value.includes('antigravity')) return 'antigravity'
    return 'openai'
  }

  function defaultProviderBaseURL(supplier: Supplier | null): string {
    const configured = supplier?.api_base_url?.trim()
    if (configured) return normalizeGatewayBaseURL(configured)
    const dashboard = supplier?.dashboard_url?.trim()
    if (dashboard) return normalizeGatewayBaseURL(dashboard)
    return ''
  }

  function normalizeGatewayBaseURL(raw: string): string {
    try {
      const url = new URL(raw)
      const pathname = url.pathname.replace(/\/+$/, '')
      if (pathname.endsWith('/api/v1')) {
        url.pathname = `${pathname.slice(0, -'/api/v1'.length)}/v1`
      } else if (!pathname.endsWith('/v1')) {
        url.pathname = `${pathname}/v1`
      }
      url.search = ''
      url.hash = ''
      return url.toString().replace(/\/$/, '')
    } catch {
      return raw.replace(/\/+$/, '')
    }
  }

  async function submitProvision() {
    if (!groupsSupplier.value || !provisionGroup.value) return
    if (!provisionForm.local_account_base_url.trim()) {
      provisionError.value = '请填写本地账号 Base URL'
      return
    }
    if (isSwitchableRuntimeStatus(provisionForm.runtime_status) && centsFromYuan(provisionForm.balance_yuan) <= 0) {
      provisionError.value = '候选或使用中账号必须有可用余额'
      return
    }
    provisionSubmitting.value = true
    provisionError.value = ''
    try {
      const job = await provisionSupplierKey(groupsSupplier.value.id, {
        supplier_group_id: provisionGroup.value.id,
        name: provisionForm.name,
        sync_provider_name: Boolean(provisionForm.sync_provider_name),
        quota_usd: Number(provisionForm.quota_usd || 0),
        expires_in_days: provisionForm.expires_in_days || null,
        local_account_platform: provisionForm.local_account_platform,
        local_account_name: provisionForm.local_account_name,
        local_account_base_url: provisionForm.local_account_base_url,
        local_account_concurrency: Number(provisionForm.local_account_concurrency || 0),
        local_account_priority: Number(provisionForm.local_account_priority || 0),
        local_account_rate_multiplier: Number(provisionForm.local_account_rate_multiplier || 0),
        runtime_status: provisionForm.runtime_status,
        health_status: provisionForm.health_status,
        balance_threshold_cents: centsFromYuan(provisionForm.balance_threshold_yuan),
        balance_cents: centsFromYuan(provisionForm.balance_yuan),
        balance_currency: provisionForm.balance_currency || 'USD'
      })
      appStore.showSuccess(`开通任务已提交 #${job.job_id}`)
      await watchProvisionJob(job.job_id)
      closeProvisionDialog()
    } catch (error) {
      provisionError.value = (error as { message?: string }).message || '开通 Key/账号失败'
      appStore.showError(provisionError.value)
    } finally {
      provisionSubmitting.value = false
    }
  }

  async function loadRepairLocalAccounts() {
    repairAccountsLoading.value = true
    try {
      const result = await listLocalSub2APIAccounts({ page: 1, page_size: 200 })
      localAccounts.value = result.items
    } catch (error) {
      repairError.value = (error as { message?: string }).message || '加载本地账号失败'
    } finally {
      repairAccountsLoading.value = false
    }
  }

  async function submitRepairBinding() {
    if (!groupsSupplier.value || !repairKey.value) return
    if (!repairForm.local_sub2api_account_id) {
      repairError.value = '请选择本地 Sub2API 账号'
      return
    }
    if (isSwitchableRuntimeStatus(repairForm.runtime_status) && centsFromYuan(repairForm.balance_yuan) <= 0) {
      repairError.value = '候选或使用中账号必须有可用余额'
      return
    }
    repairSubmitting.value = true
    repairError.value = ''
    try {
      await repairSupplierKeyBinding(groupsSupplier.value.id, repairKey.value.id, {
        local_sub2api_account_id: repairForm.local_sub2api_account_id,
        runtime_status: repairForm.runtime_status,
        health_status: repairForm.health_status,
        configured_concurrency: Number(repairForm.configured_concurrency || 0),
        balance_threshold_cents: centsFromYuan(repairForm.balance_threshold_yuan),
        balance_cents: centsFromYuan(repairForm.balance_yuan),
        balance_currency: repairForm.balance_currency || 'USD'
      })
      appStore.showSuccess('已修复 Key 与本地账号绑定')
      closeRepairDialog()
      await loadCurrentGroups()
    } catch (error) {
      repairError.value = (error as { message?: string }).message || '修复绑定失败'
      appStore.showError(repairError.value)
    } finally {
      repairSubmitting.value = false
    }
  }

  async function syncCurrentGroups() {
    if (!groupsSupplier.value) return
    groupsSyncing.value = true
    groupsError.value = ''
    provisionJobError.value = ''
    try {
      const job = await syncSupplierGroups(groupsSupplier.value.id)
      appStore.showSuccess(`同步分组任务已提交 #${job.job_id}`)
      await watchProvisionJob(job.job_id)
    } catch (error) {
      groupsError.value = (error as { message?: string }).message || '同步分组任务提交失败'
      appStore.showError(groupsError.value)
    } finally {
      groupsSyncing.value = false
    }
  }

  async function ensureCurrentKeys() {
    if (!groupsSupplier.value) return
    keysEnsuring.value = true
    groupsError.value = ''
    provisionJobError.value = ''
    try {
      const supplier = groupsSupplier.value
      const job = await ensureSupplierKeys(supplier.id, {
        sync_provider_name: Boolean(keyNamingForm.sync_provider_name),
        local_account_base_url: defaultProviderBaseURL(supplier),
        local_account_concurrency: 2,
        local_account_priority: 100,
        runtime_status: 'monitor_only',
        health_status: 'normal',
        balance_threshold_cents: 0,
        balance_cents: Math.max(0, supplier.balance_cents || 0),
        balance_currency: supplier.balance_currency || 'USD'
      })
      appStore.showSuccess(`补齐 Key/账号任务已提交 #${job.job_id}`)
      await watchProvisionJob(job.job_id)
    } catch (error) {
      groupsError.value = (error as { message?: string }).message || '补齐 Key/账号任务提交失败'
      appStore.showError(groupsError.value)
    } finally {
      keysEnsuring.value = false
    }
  }

  async function standardizeCurrentKeyNames() {
    if (!groupsSupplier.value || keyNamesStandardizing.value) return
    keyNamesStandardizing.value = true
    groupsError.value = ''
    try {
      const result = await standardizeSupplierKeyNames(groupsSupplier.value.id, {
        sync_provider_name: Boolean(keyNamingForm.sync_provider_name)
      })
      const failedText = result.failed > 0 ? `，失败 ${result.failed}` : ''
      appStore.showSuccess(`已规范 Key 名称：更新 ${result.updated}，跳过 ${result.skipped}${failedText}`)
      await loadCurrentGroups()
    } catch (error) {
      groupsError.value = (error as { message?: string }).message || '规范 Key 名称失败'
      appStore.showError(groupsError.value)
    } finally {
      keyNamesStandardizing.value = false
    }
  }

  async function watchProvisionJob(jobID: number) {
    stopProvisionJobPolling()
    await refreshProvisionJob(jobID)
  }

  async function refreshProvisionJob(jobID: number) {
    try {
      const job = await getSupplierProvisionJob(jobID)
      activeProvisionJob.value = job
      provisionJobError.value = ''
      if (isTerminalProvisionJobStatus(job.status)) {
        groupPagination.page = 1
        await Promise.all([reloadGroupSession(), loadCurrentGroups(), loadSuppliers()])
        return
      }
      provisionJobTimer.value = window.setTimeout(() => {
        void refreshProvisionJob(jobID)
      }, 2000)
    } catch (error) {
      provisionJobError.value = (error as { message?: string }).message || '读取任务状态失败'
    }
  }

  function stopProvisionJobPolling() {
    if (provisionJobTimer.value) {
      window.clearTimeout(provisionJobTimer.value)
      provisionJobTimer.value = undefined
    }
  }

  function isTerminalProvisionJobStatus(status: SupplierProvisionStatus): boolean {
    return ['succeeded', 'partial_succeeded', 'manual_required', 'dead', 'cancelled'].includes(status)
  }

  Object.assign(ctx, {
    openProvisionDialog,
    closeProvisionDialog,
    openRepairDialog,
    closeRepairDialog,
    fillProvisionForm,
    fillRepairForm,
    defaultProvisionName,
    standardProvisionName,
    normalizeLocalPlatform,
    defaultProviderBaseURL,
    normalizeGatewayBaseURL,
    submitProvision,
    loadRepairLocalAccounts,
    submitRepairBinding,
    syncCurrentGroups,
    ensureCurrentKeys,
    standardizeCurrentKeyNames,
    watchProvisionJob,
    refreshProvisionJob,
    stopProvisionJobPolling,
    isTerminalProvisionJobStatus
  })
}
