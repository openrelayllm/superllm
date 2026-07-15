import { applyLocalAccountOpsAction, deleteSupplierProviderKey, disableSupplierKeyLocalProjection, disableSupplierProviderKey, ensureSupplierKeys, getSupplierProvisionJob, importSupplierProviderKeyProjection, importSupplierProviderKeyProjections, listLocalSub2APIAccounts, planEnsureSupplierKeys, previewLocalAccountOpsAction, provisionSupplierKey, repairSupplierKeyBinding, standardizeSupplierKeyNames, syncSupplierGroups } from '@/api/admin/adminPlus'
import type { EnsureSupplierKeysPayload, EnsureSupplierKeysPlanItem, LocalAccountOpsActionPayload, LocalAccountOpsActionResult, Supplier, SupplierGroup, SupplierKey, SupplierProvisionStatus } from '@/api/admin/adminPlus'
import { ctxFn, ctxValue } from './ctxProxy'

interface ProviderKeyScheduleDecision {
  confirmed: boolean
  applyLocalScheduleDisable: boolean
  payload?: LocalAccountOpsActionPayload
}

export function isPartialProvisionSkippableBlockReason(reason?: string): boolean {
  return reason === 'key_capacity_exhausted'
    || reason === 'group_key_capacity_exhausted'
    || reason === 'provider_key_exists_unbound'
}

export function attachSupplierProvision(ctx: any) {
  const appStore = ctxValue(ctx, 'appStore')
  const provisionSubmitting = ctxValue(ctx, 'provisionSubmitting')
  const repairSubmitting = ctxValue(ctx, 'repairSubmitting')
  const provisionDialogOpen = ctxValue(ctx, 'provisionDialogOpen')
  const repairDialogOpen = ctxValue(ctx, 'repairDialogOpen')
  const groupsSupplier = ctxValue(ctx, 'groupsSupplier')
  const supplierGroups = ctxValue(ctx, 'supplierGroups')
  const provisionGroup = ctxValue(ctx, 'provisionGroup')
  const repairKey = ctxValue(ctx, 'repairKey')
  const activeProvisionJob = ctxValue(ctx, 'activeProvisionJob')
  const ensureKeysPlan = ctxValue(ctx, 'ensureKeysPlan')
  const ensureKeysPriorityGroupIDs = ctxValue(ctx, 'ensureKeysPriorityGroupIDs')
  const localAccounts = ctxValue(ctx, 'localAccounts')
  const groupsSyncing = ctxValue(ctx, 'groupsSyncing')
  const keysEnsuring = ctxValue(ctx, 'keysEnsuring')
  const ensureKeysPlanning = ctxValue(ctx, 'ensureKeysPlanning')
  const keyNamesStandardizing = ctxValue(ctx, 'keyNamesStandardizing')
  const keyProjectionDisabling = ctxValue(ctx, 'keyProjectionDisabling')
  const providerKeyImportingGroupID = ctxValue(ctx, 'providerKeyImportingGroupID')
  const providerKeyBatchImporting = ctxValue(ctx, 'providerKeyBatchImporting')
  const repairAccountsLoading = ctxValue(ctx, 'repairAccountsLoading')
  const groupsError = ctxValue(ctx, 'groupsError')
  const provisionError = ctxValue(ctx, 'provisionError')
  const provisionJobError = ctxValue(ctx, 'provisionJobError')
  const ensureKeysPlanError = ctxValue(ctx, 'ensureKeysPlanError')
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
    repairForm.mode = 'bind_existing'
    repairForm.local_sub2api_account_id = 0
    repairForm.manual_secret = ''
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
    const group = repairGroupForKey(key)
    const defaultName = group ? standardProvisionName(supplier, group) : key.name
    repairForm.mode = key.status === 'manual_secret_required' ? 'manual_secret' : 'bind_existing'
    repairForm.local_sub2api_account_id = key.local_sub2api_account_id || 0
    repairForm.manual_secret = ''
    repairForm.local_account_platform = normalizeLocalPlatform(group?.provider_family || key.provider_family)
    repairForm.local_account_name = defaultName
    repairForm.local_account_base_url = defaultProviderBaseURL(supplier)
    repairForm.local_account_priority = 100
    repairForm.local_account_rate_multiplier = group ? groupCostMultiplier(group) || 1 : 1
    repairForm.runtime_status = 'monitor_only'
    repairForm.health_status = 'normal'
    repairForm.configured_concurrency = 0
    repairForm.balance_yuan = yuanFromCents(supplier?.balance_cents || 0)
    repairForm.balance_threshold_yuan = 0
    repairForm.balance_currency = supplier?.balance_currency || 'USD'
  }

  function repairGroupForKey(key: SupplierKey): SupplierGroup | null {
    return supplierGroups.value.find((group: SupplierGroup) => group.id === key.supplier_group_id) || null
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
    if (repairForm.mode === 'bind_existing' && !repairForm.local_sub2api_account_id) {
      repairError.value = '请选择本地 Sub2API 账号'
      return
    }
    if (repairForm.mode === 'manual_secret') {
      if (!repairForm.manual_secret.trim()) {
        repairError.value = '请粘贴第三方 Key 明文'
        return
      }
      if (!repairForm.local_account_base_url.trim()) {
        repairError.value = '请填写本地账号 Base URL'
        return
      }
    }
    if (isSwitchableRuntimeStatus(repairForm.runtime_status) && centsFromYuan(repairForm.balance_yuan) <= 0) {
      repairError.value = '候选或使用中账号必须有可用余额'
      return
    }
    repairSubmitting.value = true
    repairError.value = ''
    try {
      await repairSupplierKeyBinding(groupsSupplier.value.id, repairKey.value.id, {
        local_sub2api_account_id: repairForm.mode === 'bind_existing' ? repairForm.local_sub2api_account_id : 0,
        manual_secret: repairForm.mode === 'manual_secret' ? repairForm.manual_secret.trim() : undefined,
        local_account_platform: repairForm.mode === 'manual_secret' ? repairForm.local_account_platform : undefined,
        local_account_name: repairForm.mode === 'manual_secret' ? repairForm.local_account_name : undefined,
        local_account_base_url: repairForm.mode === 'manual_secret' ? repairForm.local_account_base_url : undefined,
        local_account_priority: repairForm.mode === 'manual_secret' ? Number(repairForm.local_account_priority || 0) : undefined,
        local_account_rate_multiplier: repairForm.mode === 'manual_secret' ? Number(repairForm.local_account_rate_multiplier || 0) : undefined,
        runtime_status: repairForm.runtime_status,
        health_status: repairForm.health_status,
        configured_concurrency: Number(repairForm.configured_concurrency || 0),
        balance_threshold_cents: centsFromYuan(repairForm.balance_threshold_yuan),
        balance_cents: centsFromYuan(repairForm.balance_yuan),
        balance_currency: repairForm.balance_currency || 'USD'
      })
      appStore.showSuccess(repairForm.mode === 'manual_secret' ? '已补录密钥并完成本地账号绑定' : '已修复 Key 与本地账号绑定')
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

  function ensureKeysPayload(supplier: Supplier, allowPartial = false): EnsureSupplierKeysPayload {
    const payload: EnsureSupplierKeysPayload = {
      sync_provider_name: Boolean(keyNamingForm.sync_provider_name),
      allow_partial: allowPartial,
      local_account_base_url: defaultProviderBaseURL(supplier),
      local_account_concurrency: 2,
      local_account_priority: 100,
      runtime_status: 'monitor_only',
      health_status: 'normal',
      balance_threshold_cents: 0,
      balance_cents: Math.max(0, supplier.balance_cents || 0),
      balance_currency: supplier.balance_currency || 'USD'
    }
    if (ensureKeysPriorityGroupIDs.value.length > 0) {
      payload.supplier_group_priority_ids = ensureKeysPriorityGroupIDs.value
    }
    return payload
  }

  function ensureKeysPlanActionableCount(): number {
    if (!ensureKeysPlan.value) return 0
    return ensureKeysPlan.value.items.filter((item: EnsureSupplierKeysPlanItem) => item.action === 'create' || item.action === 'skipped_existing').length
  }

  function ensureKeysPlanCanSubmit(): boolean {
    if (!ensureKeysPlan.value) return false
    if (ensureKeysPlan.value.blocked === 0) return ensureKeysPlanActionableCount() > 0
    return ensureKeysPlanActionableCount() > 0 && ensureKeysPlan.value.items
      .filter((item: EnsureSupplierKeysPlanItem) => item.action === 'blocked')
      .every((item: EnsureSupplierKeysPlanItem) => isPartialProvisionSkippableBlockReason(item.blocked_reason))
  }

  function ensureKeysPriorityItems(): EnsureSupplierKeysPlanItem[] {
    if (!ensureKeysPlan.value) return []
    return ensureKeysPlan.value.items
      .filter((item: EnsureSupplierKeysPlanItem) => item.action === 'create' || isCapacityExhaustedBlockReason(item.blocked_reason))
      .sort((left: EnsureSupplierKeysPlanItem, right: EnsureSupplierKeysPlanItem) => {
        const leftPriority = left.priority || Number.MAX_SAFE_INTEGER
        const rightPriority = right.priority || Number.MAX_SAFE_INTEGER
        if (leftPriority !== rightPriority) return leftPriority - rightPriority
        const leftRate = left.effective_rate_multiplier || left.rate_multiplier || Number.MAX_SAFE_INTEGER
        const rightRate = right.effective_rate_multiplier || right.rate_multiplier || Number.MAX_SAFE_INTEGER
        if (leftRate !== rightRate) return leftRate - rightRate
        return left.supplier_group_id - right.supplier_group_id
      })
  }

  async function moveEnsureKeyPriority(item: EnsureSupplierKeysPlanItem, direction: 'up' | 'down') {
    if (!ensureKeysPlan.value || !groupsSupplier.value || ensureKeysPlanning.value) return
    const orderedIDs = ensureKeysPriorityItems().map((planItem: EnsureSupplierKeysPlanItem) => planItem.supplier_group_id)
    const index = orderedIDs.indexOf(item.supplier_group_id)
    if (index < 0) return
    const targetIndex = direction === 'up' ? index - 1 : index + 1
    if (targetIndex < 0 || targetIndex >= orderedIDs.length) return
    const next = [...orderedIDs]
    const moved = next[index]
    next[index] = next[targetIndex]
    next[targetIndex] = moved
    ensureKeysPriorityGroupIDs.value = next
    await previewEnsureCurrentKeys()
  }

  async function resetEnsureKeyPriority() {
    if (ensureKeysPriorityGroupIDs.value.length === 0 || ensureKeysPlanning.value) return
    ensureKeysPriorityGroupIDs.value = []
    await previewEnsureCurrentKeys()
  }

  async function previewEnsureCurrentKeys() {
    if (!groupsSupplier.value) return
    ensureKeysPlanning.value = true
    groupsError.value = ''
    provisionJobError.value = ''
    ensureKeysPlanError.value = ''
    try {
      const supplier = groupsSupplier.value
      ensureKeysPlan.value = await planEnsureSupplierKeys(supplier.id, ensureKeysPayload(supplier))
      if (ensureKeysPlan.value.blocked > 0) {
        const message = ensureKeysPlanCanSubmit()
          ? 'Key 开通计划存在可跳过的阻塞项，可只提交其余可创建分组'
          : 'Key 开通计划存在阻塞项，请按计划明细处理后重试'
        appStore.showWarning(message)
      } else {
        appStore.showSuccess('Key 开通计划已生成')
      }
    } catch (error) {
      ensureKeysPlanError.value = (error as { message?: string }).message || '生成 Key 开通计划失败'
      appStore.showError(ensureKeysPlanError.value)
    } finally {
      ensureKeysPlanning.value = false
    }
  }

  function isCapacityExhaustedBlockReason(reason?: string): boolean {
    return reason === 'key_capacity_exhausted' || reason === 'group_key_capacity_exhausted'
  }

  async function ensureCurrentKeys() {
    if (!groupsSupplier.value) return
    if (!ensureKeysPlan.value || ensureKeysPlan.value.supplier_id !== groupsSupplier.value.id) {
      await previewEnsureCurrentKeys()
    }
    if (!ensureKeysPlanCanSubmit()) return
    const actionable = ensureKeysPlanActionableCount()
    if (actionable === 0) {
      appStore.showWarning('没有需要补齐的供应商分组')
      return
    }
    keysEnsuring.value = true
    groupsError.value = ''
    provisionJobError.value = ''
    ensureKeysPlanError.value = ''
    try {
      const supplier = groupsSupplier.value
      const allowPartial = ensureKeysPlan.value.blocked > 0
      const job = await ensureSupplierKeys(supplier.id, ensureKeysPayload(supplier, allowPartial))
      appStore.showSuccess(`补齐 Key/账号任务已提交 #${job.job_id}`)
      await watchProvisionJob(job.job_id)
      ensureKeysPlan.value = null
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

  async function disableCurrentKeyLocalProjection(key: SupplierKey) {
    if (!groupsSupplier.value || !key || keyProjectionDisabling.value) return
    const confirmed = window.confirm([
      '仅释放 SuperLLM 本地配额投影，不会删除第三方供应商后台 Key。',
      '如果该 Key 已落地本地账号，本地账号调度状态不会在这里自动变更。',
      '确认继续？'
    ].join('\n'))
    if (!confirmed) return
    keyProjectionDisabling.value = key.id
    groupsError.value = ''
    ensureKeysPlanError.value = ''
    try {
      await disableSupplierKeyLocalProjection(groupsSupplier.value.id, key.id, {
        reason: '运营手动释放本地 Key 配额投影'
      })
      appStore.showSuccess('已释放本地 Key 配额投影')
      await Promise.all([loadCurrentGroups(), loadSuppliers()])
      if (ensureKeysPlan.value) {
        await previewEnsureCurrentKeys()
      }
    } catch (error) {
      groupsError.value = (error as { message?: string }).message || '释放本地 Key 配额投影失败'
      appStore.showError(groupsError.value)
    } finally {
      keyProjectionDisabling.value = null
    }
  }

  async function disableCurrentProviderKey(key: SupplierKey) {
    if (!groupsSupplier.value || !key || keyProjectionDisabling.value) return
    if (!key.external_key_id) {
      appStore.showError('当前 Key 缺少第三方 ID，无法调用供应商后台停用')
      return
    }
    const scheduleDecision = await confirmProviderKeyScheduleDecision(key, 'disable')
    if (!scheduleDecision.confirmed) return
    keyProjectionDisabling.value = key.id
    groupsError.value = ''
    ensureKeysPlanError.value = ''
    try {
      await disableSupplierProviderKey(groupsSupplier.value.id, key.id, {
        reason: '运营手动停用第三方供应商 Key'
      })
      await applyProviderKeyLocalScheduleDecision(scheduleDecision)
      appStore.showSuccess(scheduleDecision.applyLocalScheduleDisable ? '已停用第三方 Key，并关闭对应本地账号调度' : '已停用第三方供应商 Key，并释放本地投影')
      await Promise.all([loadCurrentGroups(), loadSuppliers()])
      if (ensureKeysPlan.value) {
        await previewEnsureCurrentKeys()
      }
    } catch (error) {
      groupsError.value = (error as { message?: string }).message || '停用第三方供应商 Key 失败'
      appStore.showError(groupsError.value)
    } finally {
      keyProjectionDisabling.value = null
    }
  }

  async function deleteCurrentProviderKey(key: SupplierKey) {
    if (!groupsSupplier.value || !key || keyProjectionDisabling.value) return
    if (!key.external_key_id) {
      appStore.showError('当前 Key 缺少第三方 ID，无法调用供应商后台删除')
      return
    }
    const confirmation = window.prompt([
      '将调用第三方供应商后台删除这个 Key，可能不可恢复。',
      '成功后 SuperLLM 会把本地 Key 投影标记为停用；下一步会预览本地账号调度影响，可选择同步关闭或仅处理第三方 Key。',
      '请输入“删除”确认。'
    ].join('\n'))
    if (confirmation !== '删除') return
    const scheduleDecision = await confirmProviderKeyScheduleDecision(key, 'delete')
    if (!scheduleDecision.confirmed) return
    keyProjectionDisabling.value = key.id
    groupsError.value = ''
    ensureKeysPlanError.value = ''
    try {
      await deleteSupplierProviderKey(groupsSupplier.value.id, key.id, {
        reason: '运营手动删除第三方供应商 Key'
      })
      await applyProviderKeyLocalScheduleDecision(scheduleDecision)
      appStore.showSuccess(scheduleDecision.applyLocalScheduleDisable ? '已删除第三方 Key，并关闭对应本地账号调度' : '已删除第三方供应商 Key，并释放本地投影')
      await Promise.all([loadCurrentGroups(), loadSuppliers()])
      if (ensureKeysPlan.value) {
        await previewEnsureCurrentKeys()
      }
    } catch (error) {
      groupsError.value = (error as { message?: string }).message || '删除第三方供应商 Key 失败'
      appStore.showError(groupsError.value)
    } finally {
      keyProjectionDisabling.value = null
    }
  }

  async function confirmProviderKeyScheduleDecision(key: SupplierKey, mode: 'disable' | 'delete'): Promise<ProviderKeyScheduleDecision> {
    const verb = mode === 'delete' ? '删除' : '停用'
    if (!key.local_sub2api_account_id) {
      return {
        confirmed: window.confirm([
          `将调用第三方供应商后台${verb}这个 Key。`,
          '当前 Key 没有本地账号绑定，不涉及本地调度变更。',
          '确认继续？'
        ].join('\n')),
        applyLocalScheduleDisable: false
      }
    }
    const payload = providerKeyLocalScheduleDisablePayload(key, mode)
    let preview: LocalAccountOpsActionResult | null = null
    try {
      preview = await previewLocalAccountOpsAction(payload)
    } catch (error) {
      const confirmed = window.confirm([
        `本地调度联动 preview 失败：${(error as { message?: string }).message || '未知错误'}`,
        `是否仍仅${verb}第三方 Key？本地账号调度不会变更。`
      ].join('\n'))
      return { confirmed, applyLocalScheduleDisable: false }
    }
    if (preview.blocked) {
      const confirmed = window.confirm([
        '本地调度联动 preview 已被保护阻断。',
        providerKeySchedulePreviewSummary(preview),
        `是否仍仅${verb}第三方 Key？本地账号调度不会变更。`
      ].join('\n'))
      return { confirmed, applyLocalScheduleDisable: false }
    }
    const choice = window.prompt([
      `将调用第三方供应商后台${verb}这个 Key。`,
      providerKeySchedulePreviewSummary(preview),
      '输入“同步”会在第三方操作成功后关闭对应本地账号调度。',
      `输入“仅第三方”只${verb}第三方 Key，不变更本地调度。`
    ].join('\n'))
    if (choice === '同步') {
      return { confirmed: true, applyLocalScheduleDisable: true, payload }
    }
    if (choice === '仅第三方') {
      return { confirmed: true, applyLocalScheduleDisable: false }
    }
    return { confirmed: false, applyLocalScheduleDisable: false }
  }

  function providerKeyLocalScheduleDisablePayload(key: SupplierKey, mode: 'disable' | 'delete'): LocalAccountOpsActionPayload {
    return {
      action: 'set_schedulable',
      account_ids: [key.local_sub2api_account_id || 0].filter(Boolean),
      schedulable: false,
      allow_empty_pool: false,
      reason: `supplier_key_provider_${mode}:${key.id}`
    }
  }

  function providerKeySchedulePreviewSummary(result: LocalAccountOpsActionResult): string {
    const impacts = result.group_impacts || []
    const emptyGroups = impacts.filter((item) => item.would_empty_schedulable_pool)
    const groupText = impacts.length > 0
      ? impacts.slice(0, 3).map((item) => `${item.group_name || `分组 #${item.group_id}`} ${item.before_schedulable_accounts}->${item.after_schedulable_accounts}`).join('；')
      : '无本地分组影响'
    const emptyText = emptyGroups.length > 0 ? `；空池风险 ${emptyGroups.length} 个` : ''
    return `preview：影响账号 ${result.account_ids.length} 个，${groupText}${emptyText}。`
  }

  async function applyProviderKeyLocalScheduleDecision(decision: ProviderKeyScheduleDecision) {
    if (!decision.applyLocalScheduleDisable || !decision.payload) return
    try {
      await applyLocalAccountOpsAction(decision.payload)
    } catch (error) {
      appStore.showWarning(`第三方 Key 已处理，但关闭本地调度失败：${(error as { message?: string }).message || '未知错误'}`, 8000)
    }
  }

  async function importCurrentProviderKeyProjection(item: EnsureSupplierKeysPlanItem) {
    if (!groupsSupplier.value || !item || providerKeyImportingGroupID.value) return
    providerKeyImportingGroupID.value = item.supplier_group_id
    groupsError.value = ''
    ensureKeysPlanError.value = ''
    try {
      const key = await importSupplierProviderKeyProjection(groupsSupplier.value.id, {
        supplier_group_id: item.supplier_group_id,
        external_key_id: item.provider_external_key_id
      })
      appStore.showSuccess('已导入第三方 Key 投影，请补录密钥完成本地账号绑定')
      await loadCurrentGroups()
      openRepairDialog(key)
    } catch (error) {
      groupsError.value = (error as { message?: string }).message || '导入第三方 Key 投影失败'
      appStore.showError(groupsError.value)
    } finally {
      providerKeyImportingGroupID.value = null
    }
  }

  async function importCurrentProviderKeyProjections() {
    if (!groupsSupplier.value || providerKeyBatchImporting.value) return
    const items = (ensureKeysPlan.value?.items || [])
      .filter((item: EnsureSupplierKeysPlanItem) => item.action === 'blocked' && item.blocked_reason === 'provider_key_exists_unbound' && item.provider_external_key_id)
    if (items.length === 0) {
      appStore.showWarning('当前开通计划没有可批量导入的第三方未绑定 Key')
      return
    }
    const confirmation = window.prompt([
      `将批量导入 ${items.length} 个第三方已有 Key 的本地投影。`,
      '导入后不会保存 Key 明文，也不会自动创建本地账号；仍需要逐个补录密钥完成绑定。',
      '请输入“导入”确认。'
    ].join('\n'))
    if (confirmation !== '导入') return
    providerKeyBatchImporting.value = true
    groupsError.value = ''
    ensureKeysPlanError.value = ''
    try {
      const result = await importSupplierProviderKeyProjections(groupsSupplier.value.id, {
        items: items.map((item: EnsureSupplierKeysPlanItem) => ({
          supplier_group_id: item.supplier_group_id,
          external_key_id: item.provider_external_key_id
        }))
      })
      const summary = `已导入 ${result.imported} 个，跳过 ${result.skipped} 个，失败 ${result.failed} 个`
      if (result.failed > 0) {
        const firstFailed = result.items.find((item) => item.action === 'failed')
        appStore.showWarning(`${summary}；首个失败：${firstFailed?.error_message || firstFailed?.error_code || '未知错误'}`, 8000)
      } else {
        appStore.showSuccess(summary)
      }
      await loadCurrentGroups()
      await previewEnsureCurrentKeys()
    } catch (error) {
      groupsError.value = (error as { message?: string }).message || '批量导入第三方 Key 投影失败'
      appStore.showError(groupsError.value)
    } finally {
      providerKeyBatchImporting.value = false
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
    ensureKeysPayload,
    ensureKeysPlanActionableCount,
    ensureKeysPlanCanSubmit,
    ensureKeysPriorityItems,
    moveEnsureKeyPriority,
    resetEnsureKeyPriority,
    previewEnsureCurrentKeys,
    ensureCurrentKeys,
    standardizeCurrentKeyNames,
    disableCurrentKeyLocalProjection,
    disableCurrentProviderKey,
    deleteCurrentProviderKey,
    importCurrentProviderKeyProjection,
    importCurrentProviderKeyProjections,
    watchProvisionJob,
    refreshProvisionJob,
    stopProvisionJobPolling,
    isTerminalProvisionJobStatus
  })
}
