import { createSupplier, loginSupplierSession, probeSupplierSession, updateSupplier } from '@/api/admin/adminPlus'
import type { Supplier } from '@/api/admin/adminPlus'
import { ctxFn, ctxValue } from './ctxProxy'
export function attachSupplierDialogs(ctx: any) {
  const appStore = ctxValue(ctx, 'appStore')
  const submitting = ctxValue(ctx, 'submitting')
  const editorOpen = ctxValue(ctx, 'editorOpen')
  const statusDialogOpen = ctxValue(ctx, 'statusDialogOpen')
  const sessionDialogOpen = ctxValue(ctx, 'sessionDialogOpen')
  const channelStatusDialogOpen = ctxValue(ctx, 'channelStatusDialogOpen')
  const groupsDialogOpen = ctxValue(ctx, 'groupsDialogOpen')
  const moreMenuOpen = ctxValue(ctx, 'moreMenuOpen')
  const bulkStatusMode = ctxValue(ctx, 'bulkStatusMode')
  const editingSupplier = ctxValue(ctx, 'editingSupplier')
  const sessionSupplier = ctxValue(ctx, 'sessionSupplier')
  const channelStatusSupplier = ctxValue(ctx, 'channelStatusSupplier')
  const groupsSupplier = ctxValue(ctx, 'groupsSupplier')
  const supplierGroups = ctxValue(ctx, 'supplierGroups')
  const supplierKeys = ctxValue(ctx, 'supplierKeys')
  const supplierChannelChecks = ctxValue(ctx, 'supplierChannelChecks')
  const activeProvisionJob = ctxValue(ctx, 'activeProvisionJob')
  const channelMonitorItems = ctxValue(ctx, 'channelMonitorItems')
  const sessionStore = ctxValue(ctx, 'sessionStore')
  const loggingInSession = ctxValue(ctx, 'loggingInSession')
  const probingSession = ctxValue(ctx, 'probingSession')
  const currentBalanceError = ctxValue(ctx, 'currentBalanceError')
  const channelStatusLoading = ctxValue(ctx, 'channelStatusLoading')
  const sessionLoadError = ctxValue(ctx, 'sessionLoadError')
  const channelStatusError = ctxValue(ctx, 'channelStatusError')
  const channelMonitorCapturedAt = ctxValue(ctx, 'channelMonitorCapturedAt')
  const channelMonitorOrigin = ctxValue(ctx, 'channelMonitorOrigin')
  const channelMonitorAPIBaseURL = ctxValue(ctx, 'channelMonitorAPIBaseURL')
  const channelStatusWindow = ctxValue(ctx, 'channelStatusWindow')
  const channelStatusAutoRefresh = ctxValue(ctx, 'channelStatusAutoRefresh')
  const channelStatusCountdown = ctxValue(ctx, 'channelStatusCountdown')
  const groupsError = ctxValue(ctx, 'groupsError')
  const provisionJobError = ctxValue(ctx, 'provisionJobError')
  const channelCheckError = ctxValue(ctx, 'channelCheckError')
  const lastProbe = ctxValue(ctx, 'lastProbe')
  const rowLoginSupplierID = ctxValue(ctx, 'rowLoginSupplierID')
  const channelStatusAutoRefreshTimer = ctxValue(ctx, 'channelStatusAutoRefreshTimer')
  const filters = ctxValue(ctx, 'filters')
  const channelProtocolFilter = ctxValue(ctx, 'channelProtocolFilter')
  const groupPagination = ctxValue(ctx, 'groupPagination')
  const groupFilters = ctxValue(ctx, 'groupFilters')
  const form = ctxValue(ctx, 'form')
  const statusForm = ctxValue(ctx, 'statusForm')
  const centsFromYuan = ctxFn(ctx, 'centsFromYuan')
  const yuanFromCents = ctxFn(ctx, 'yuanFromCents')
  const normalizeRechargeMultiplierForForm = ctxFn(ctx, 'normalizeRechargeMultiplierForForm')
  const directLoginPreflightError = ctxFn(ctx, 'directLoginPreflightError')
  const directLoginErrorMessage = ctxFn(ctx, 'directLoginErrorMessage')
  const isBalanceProbeError = ctxFn(ctx, 'isBalanceProbeError')
  const normalizeBalanceErrorMessage = ctxFn(ctx, 'normalizeBalanceErrorMessage')
  const errorMessage = ctxFn(ctx, 'errorMessage')
  const loadSuppliers = ctxFn(ctx, 'loadSuppliers')
  const reloadCurrentSession = ctxFn(ctx, 'reloadCurrentSession')
  const reloadCurrentBalance = ctxFn(ctx, 'reloadCurrentBalance')
  const reloadGroupSession = ctxFn(ctx, 'reloadGroupSession')
  const loadChannelStatus = ctxFn(ctx, 'loadChannelStatus')
  const loadCurrentGroups = ctxFn(ctx, 'loadCurrentGroups')
  const stopProvisionJobPolling = ctxFn(ctx, 'stopProvisionJobPolling')
  const closeRowActionsMenu = ctxFn(ctx, 'closeRowActionsMenu')
  function resetFilters() {
    filters.q = ''
    filters.kind = ''
    filters.type = ''
    filters.runtime_status = ''
    filters.health_status = ''
    channelProtocolFilter.value = 'openai'
    moreMenuOpen.value = false
  }

  function resetForm() {
    form.name = ''
    form.kind = 'relay'
    form.type = 'sub2api'
    form.runtime_status = 'monitor_only'
    form.health_status = 'normal'
    form.dashboard_url = ''
    form.api_base_url = ''
    form.third_party_recharge_url = ''
    form.local_recharge_url = ''
    form.contact = ''
    form.browser_login_username = ''
    form.browser_login_password = ''
    form.browser_login_token = ''
    form.balance_yuan = 0
    form.balance_currency = 'USD'
    form.recharge_multiplier = 1
    form.browser_login_enabled = true
    form.notes = ''
  }

  function fillForm(supplier: Supplier) {
    form.name = supplier.name
    form.kind = supplier.kind
    form.type = supplier.type
    form.runtime_status = supplier.runtime_status
    form.health_status = supplier.health_status
    form.dashboard_url = supplier.dashboard_url || ''
    form.api_base_url = supplier.api_base_url || ''
    form.third_party_recharge_url = supplier.third_party_recharge_url || ''
    form.local_recharge_url = supplier.local_recharge_url || ''
    form.contact = supplier.contact || ''
    form.browser_login_username = ''
    form.browser_login_password = ''
    form.browser_login_token = ''
    form.balance_yuan = yuanFromCents(supplier.balance_cents)
    form.balance_currency = supplier.balance_currency || 'USD'
    form.recharge_multiplier = normalizeRechargeMultiplierForForm(supplier.recharge_multiplier)
    form.browser_login_enabled = supplier.credential.browser_login_enabled
    form.notes = supplier.notes || ''
  }

  function openCreateDialog() {
    editingSupplier.value = null
    resetForm()
    editorOpen.value = true
  }

  function openEditDialog(supplier: Supplier) {
    closeRowActionsMenu()
    editingSupplier.value = supplier
    fillForm(supplier)
    editorOpen.value = true
  }

  function closeEditor() {
    editorOpen.value = false
  }

  function buildPayload() {
    return {
      name: form.name,
      kind: form.kind,
      type: form.type,
      runtime_status: form.runtime_status,
      health_status: form.health_status,
      dashboard_url: form.dashboard_url || undefined,
      api_base_url: form.api_base_url || undefined,
      third_party_recharge_url: form.third_party_recharge_url || undefined,
      local_recharge_url: form.local_recharge_url || undefined,
      contact: form.contact || undefined,
      browser_login_username: form.browser_login_username || undefined,
      browser_login_password: form.browser_login_password || undefined,
      browser_login_token: form.browser_login_token || undefined,
      balance_cents: centsFromYuan(form.balance_yuan),
      balance_currency: form.balance_currency || 'USD',
      recharge_multiplier: normalizeRechargeMultiplierForForm(form.recharge_multiplier),
      browser_login_enabled: form.browser_login_enabled,
      notes: form.notes || undefined
    }
  }

  async function submitSupplier() {
    submitting.value = true
    try {
      if (editingSupplier.value) {
        await updateSupplier(editingSupplier.value.id, buildPayload())
        appStore.showSuccess('供应商已更新')
      } else {
        await createSupplier(buildPayload())
        appStore.showSuccess('供应商已创建')
      }
      editorOpen.value = false
      await loadSuppliers()
    } catch (error) {
      appStore.showError((error as { message?: string }).message || '保存供应商失败')
    } finally {
      submitting.value = false
    }
  }

  function openStatusDialog(supplier: Supplier) {
    closeRowActionsMenu()
    bulkStatusMode.value = false
    statusForm.id = supplier.id
    statusForm.name = supplier.name
    statusForm.runtime_status = supplier.runtime_status
    statusForm.health_status = supplier.health_status
    statusDialogOpen.value = true
  }

  function openSessionDialog(supplier: Supplier) {
    closeRowActionsMenu()
    sessionSupplier.value = supplier
    lastProbe.value = null
    sessionLoadError.value = ''
    currentBalanceError.value = ''
    sessionDialogOpen.value = true
    void Promise.all([reloadCurrentSession(), reloadCurrentBalance(false)])
  }

  function openChannelStatusDialog(supplier: Supplier) {
    closeRowActionsMenu()
    channelStatusSupplier.value = supplier
    channelMonitorItems.value = []
    channelMonitorCapturedAt.value = ''
    channelMonitorOrigin.value = ''
    channelMonitorAPIBaseURL.value = ''
    channelStatusError.value = ''
    channelStatusWindow.value = supplier.type === 'new_api' ? 'pulse' : '7d'
    channelStatusCountdown.value = 16
    channelStatusDialogOpen.value = true
    startChannelStatusAutoRefresh()
    void loadChannelStatus()
  }

  function closeChannelStatusDialog() {
    channelStatusDialogOpen.value = false
    stopChannelStatusAutoRefresh()
  }

  function toggleChannelStatusAutoRefresh() {
    channelStatusAutoRefresh.value = !channelStatusAutoRefresh.value
    if (channelStatusAutoRefresh.value) {
      channelStatusCountdown.value = 16
      startChannelStatusAutoRefresh()
    }
  }

  function startChannelStatusAutoRefresh() {
    stopChannelStatusAutoRefresh()
    channelStatusAutoRefreshTimer.value = window.setInterval(() => {
      if (!channelStatusDialogOpen.value || !channelStatusAutoRefresh.value) return
      channelStatusCountdown.value = Math.max(0, channelStatusCountdown.value - 1)
      if (channelStatusCountdown.value === 0 && !channelStatusLoading.value) {
        void loadChannelStatus()
      }
    }, 1000)
  }

  function stopChannelStatusAutoRefresh() {
    if (channelStatusAutoRefreshTimer.value) {
      window.clearInterval(channelStatusAutoRefreshTimer.value)
      channelStatusAutoRefreshTimer.value = undefined
    }
  }

  function openGroupsDialog(supplier: Supplier) {
    closeRowActionsMenu()
    stopProvisionJobPolling()
    groupsSupplier.value = supplier
    supplierGroups.value = []
    supplierKeys.value = []
    supplierChannelChecks.value = {}
    activeProvisionJob.value = null
    groupsError.value = ''
    provisionJobError.value = ''
    channelCheckError.value = ''
    groupPagination.page = 1
    groupFilters.q = ''
    groupFilters.status = ''
    groupsDialogOpen.value = true
    void Promise.all([reloadGroupSession(), loadCurrentGroups()])
  }

  function closeGroupsDialog() {
    groupsDialogOpen.value = false
    stopProvisionJobPolling()
  }

  async function probeCurrentSession() {
    if (!sessionSupplier.value) return
    probingSession.value = true
    sessionLoadError.value = ''
    currentBalanceError.value = ''
    try {
      const result = await probeSupplierSession(sessionSupplier.value.id, {
        record_balance_snapshot: true
      })
      lastProbe.value = result.probe
      appStore.showSuccess('会话探测完成，已读取供应商余额')
      await Promise.all([reloadCurrentSession(), reloadCurrentBalance(false), loadSuppliers()])
    } catch (error) {
      if (isBalanceProbeError(error)) {
        currentBalanceError.value = normalizeBalanceErrorMessage(errorMessage(error, '会话可用，但余额读取失败'))
        return
      }
      sessionLoadError.value = (error as { message?: string }).message || '会话探测失败'
      appStore.showError(sessionLoadError.value)
    } finally {
      probingSession.value = false
    }
  }

  async function loginCurrentSession() {
    if (!sessionSupplier.value) return
    const preflightError = directLoginPreflightError(sessionSupplier.value)
    if (preflightError) {
      sessionLoadError.value = preflightError
      appStore.showError(preflightError)
      return
    }
    loggingInSession.value = true
    sessionLoadError.value = ''
    currentBalanceError.value = ''
    try {
      await directLoginSupplier(sessionSupplier.value, {
        updateLastProbe: true,
        successMessage: '后端直登完成'
      })
      await Promise.all([reloadCurrentBalance(false), loadSuppliers()])
    } catch (error) {
      if (isBalanceProbeError(error)) {
        currentBalanceError.value = normalizeBalanceErrorMessage(errorMessage(error, '后端直登完成，但余额读取失败'))
        return
      }
      sessionLoadError.value = directLoginErrorMessage(error)
      appStore.showError(sessionLoadError.value)
    } finally {
      loggingInSession.value = false
    }
  }

  async function loginSupplierFromRow(supplier: Supplier) {
    const preflightError = directLoginPreflightError(supplier)
    if (preflightError) {
      appStore.showError(preflightError)
      return
    }
    rowLoginSupplierID.value = supplier.id
    try {
      await directLoginSupplier(supplier, {
        updateLastProbe: sessionSupplier.value?.id === supplier.id,
        successMessage: '一键登录完成'
      })
      await loadSuppliers()
    } catch (error) {
      appStore.showError(directLoginErrorMessage(error))
    } finally {
      rowLoginSupplierID.value = null
    }
  }

  async function directLoginSupplier(supplier: Supplier, options: { updateLastProbe: boolean; successMessage: string }) {
    const result = await loginSupplierSession(supplier.id, {
      record_balance_snapshot: true
    })
    sessionStore[supplier.id] = result.session
    if (options.updateLastProbe) {
      lastProbe.value = result.probe || null
    }
    if (result.balance_sync_error) {
      appStore.showWarning(`${options.successMessage}，余额读取失败，已保留会话并交给调度中心重试`)
    } else {
      appStore.showSuccess(result.probe ? `${options.successMessage}，已读取供应商余额` : options.successMessage)
    }
    return result
  }

  Object.assign(ctx, {
    resetFilters,
    resetForm,
    fillForm,
    openCreateDialog,
    openEditDialog,
    closeEditor,
    buildPayload,
    submitSupplier,
    openStatusDialog,
    openSessionDialog,
    openChannelStatusDialog,
    closeChannelStatusDialog,
    toggleChannelStatusAutoRefresh,
    startChannelStatusAutoRefresh,
    stopChannelStatusAutoRefresh,
    openGroupsDialog,
    closeGroupsDialog,
    probeCurrentSession,
    loginCurrentSession,
    loginSupplierFromRow,
    directLoginSupplier
  })
}
