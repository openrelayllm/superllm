import { computed } from 'vue'
import type { ComputedRef, Ref } from 'vue'
import type { LocalSub2APIAccount, Supplier, SupplierAccount, SupplierBrowserSession, SupplierChannelCheckSnapshot, SupplierChannelMonitorView, SupplierCurrentBalance, SupplierGroup, SupplierKey, SupplierMonitorStatus, SupplierProvisionJob, SupplierProvisionStatus, SupplierSessionProbeResult } from '@/api/admin/adminPlus'
import type { ChannelStatusWindow, ChannelScheduleStep, ScheduleListRow } from './types'
import { ctxFn, ctxValue } from './ctxProxy'
export function attachSuppliersComputed(ctx: any) {
  const bulkDeleteMode = ctxValue(ctx, 'bulkDeleteMode')
  const sessionSupplier = ctxValue(ctx, 'sessionSupplier')
  const channelStatusSupplier = ctxValue(ctx, 'channelStatusSupplier')
  const groupsSupplier = ctxValue(ctx, 'groupsSupplier')
  const channelScheduleSupplier = ctxValue(ctx, 'channelScheduleSupplier')
  const deletingSupplier = ctxValue(ctx, 'deletingSupplier')
  const supplierGroups = ctxValue(ctx, 'supplierGroups') as Ref<SupplierGroup[]>
  const supplierKeys = ctxValue(ctx, 'supplierKeys') as Ref<SupplierKey[]>
  const supplierChannelChecks = ctxValue(ctx, 'supplierChannelChecks') as Ref<Record<number, SupplierChannelCheckSnapshot | undefined>>
  const activeProvisionJob = ctxValue(ctx, 'activeProvisionJob') as Ref<SupplierProvisionJob | null>
  const channelMonitorItems = ctxValue(ctx, 'channelMonitorItems') as Ref<SupplierChannelMonitorView[]>
  const sessionStore = ctxValue(ctx, 'sessionStore') as Record<number, SupplierBrowserSession | undefined>
  const currentBalanceStore = ctxValue(ctx, 'currentBalanceStore') as Record<number, SupplierCurrentBalance | undefined>
  const currentBalanceError = ctxValue(ctx, 'currentBalanceError')
  const groupsLoading = ctxValue(ctx, 'groupsLoading')
  const lastProbe = ctxValue(ctx, 'lastProbe') as Ref<SupplierSessionProbeResult | null>
  const selectedCount = ctxValue(ctx, 'selectedCount') as ComputedRef<number>
  const channelProtocolFilter = ctxValue(ctx, 'channelProtocolFilter')
  const scheduleListFilters = ctxValue(ctx, 'scheduleListFilters')
  const scheduleListSuppliers = ctxValue(ctx, 'scheduleListSuppliers') as Ref<Supplier[]>
  const scheduleListBindings = ctxValue(ctx, 'scheduleListBindings') as Ref<SupplierAccount[]>
  const scheduleListLocalAccounts = ctxValue(ctx, 'scheduleListLocalAccounts') as Ref<Record<number, LocalSub2APIAccount | undefined>>
  const scheduleListChannelChecks = ctxValue(ctx, 'scheduleListChannelChecks') as Ref<Record<string, SupplierChannelCheckSnapshot | undefined>>
  const formatBalanceMoney = ctxFn(ctx, 'formatBalanceMoney')
  const formatLatency = ctxFn(ctx, 'formatLatency')
  const supplierBestChannel = ctxFn(ctx, 'supplierBestChannel')
  const channelProtocolLabel = ctxFn(ctx, 'channelProtocolLabel')
  const channelHasLocalBinding = ctxFn(ctx, 'channelHasLocalBinding')
  const channelIsAvailable = ctxFn(ctx, 'channelIsAvailable')
  const scheduleChannelKey = ctxFn(ctx, 'scheduleChannelKey')
  const firstNonEmptyString = ctxFn(ctx, 'firstNonEmptyString')
  const scheduleRowRisky = ctxFn(ctx, 'scheduleRowRisky')
  const sessionSourceLabel = ctxFn(ctx, 'sessionSourceLabel')
  const normalizeBalanceErrorMessage = ctxFn(ctx, 'normalizeBalanceErrorMessage')
  const channelProtocolFilterLabel = computed(() => channelProtocolFilter.value ? channelProtocolLabel(channelProtocolFilter.value) : '协议')
  const scheduleListRows = computed<ScheduleListRow[]>(() => {
    const suppliersByID = new Map(scheduleListSuppliers.value.map((supplier) => [supplier.id, supplier]))
    const rows: ScheduleListRow[] = []
    for (const binding of scheduleListBindings.value) {
      const supplier = suppliersByID.get(binding.supplier_id)
      if (!supplier) continue
      const localAccount = scheduleListLocalAccounts.value[binding.local_sub2api_account_id]
      const check = scheduleListChannelChecks.value[scheduleChannelKey(binding.supplier_id, binding.supplier_group_id)]
      const providerFamily = firstNonEmptyString(check?.provider_family, binding.supplier_group_provider, binding.rate_profile)
      const groupName = firstNonEmptyString(check?.group_name, binding.supplier_group_name, binding.rate_profile, '未同步分组')
      const schedulable = Boolean(localAccount?.schedulable ?? check?.local_account_schedulable ?? false)
      const localAccountName = firstNonEmptyString(binding.local_account_name, localAccount?.name, `账号 #${binding.local_sub2api_account_id}`)
      rows.push({
        key: `${binding.supplier_id}:${binding.id}`,
        name: localAccountName,
        supplier_id: binding.supplier_id,
        supplier_name: supplier.name,
        supplier_type: supplier.type,
        binding_id: binding.id,
        supplier_group_id: binding.supplier_group_id,
        local_account_id: binding.local_sub2api_account_id,
        local_account_name: localAccountName,
        local_account_status: firstNonEmptyString(localAccount?.status, 'unknown'),
        runtime_status: binding.runtime_status,
        health_status: binding.health_status,
        schedulable,
        group_name: groupName,
        provider_family: providerFamily,
        external_group_id: firstNonEmptyString(check?.external_group_id, binding.supplier_external_group_id),
        supplier_group_rate: binding.supplier_group_rate || 0,
        effective_rate_multiplier: check?.effective_rate_multiplier || binding.supplier_group_rate || localAccount?.rate_multiplier || 1,
        probe_status: check?.probe_status || 'untested',
        first_token_ms: check?.first_token_ms || 0,
        duration_ms: check?.duration_ms || 0,
        error_message: check?.error_message || '',
        captured_at: check?.captured_at || ''
      })
    }
    return rows.sort((a, b) => {
      if (a.schedulable !== b.schedulable) return a.schedulable ? -1 : 1
      const supplierName = a.supplier_name.localeCompare(b.supplier_name)
      if (supplierName !== 0) return supplierName
      return a.local_account_id - b.local_account_id
    })
  })

  const filteredScheduleRows = computed(() => {
    const q = scheduleListFilters.q.toLowerCase()
    return scheduleListRows.value.filter((row) => {
      if (scheduleListFilters.status === 'scheduled' && !row.schedulable) return false
      if (scheduleListFilters.status === 'paused' && row.schedulable) return false
      if (scheduleListFilters.status === 'risky' && !scheduleRowRisky(row)) return false
      if (scheduleListFilters.status === 'untested' && row.probe_status !== 'untested') return false
      if (q) {
        const haystack = [
          row.supplier_name,
          row.local_account_name,
          row.group_name,
          row.provider_family,
          row.external_group_id
        ].join(' ').toLowerCase()
        if (!haystack.includes(q)) return false
      }
      return true
    })
  })

  const scheduleListStats = computed(() => {
    const rows = scheduleListRows.value
    return {
      total: rows.length,
      scheduled: rows.filter((row) => row.schedulable).length,
      paused: rows.filter((row) => !row.schedulable).length,
      risky: rows.filter((row) => scheduleRowRisky(row)).length
    }
  })

  const currentSession = computed(() => {
    const supplierID = sessionSupplier.value?.id
    return supplierID ? sessionStore[supplierID] : undefined
  })

  const currentBalanceValue = computed(() => {
    const supplierID = sessionSupplier.value?.id
    return supplierID ? currentBalanceStore[supplierID] : undefined
  })

  const channelScheduleSnapshot = computed(() => {
    const supplierID = channelScheduleSupplier.value?.id
    return supplierID ? supplierBestChannel(supplierID) : undefined
  })

  const channelSchedulePrimaryLabel = computed(() => {
    const supplier = channelScheduleSupplier.value
    const snapshot = channelScheduleSnapshot.value
    if (!supplier || !snapshot) return '一键检测'
    if (!channelHasLocalBinding(snapshot)) return '开通账号并加入调度'
    if (snapshot.local_account_schedulable) return '暂停调度'
    if (!channelIsAvailable(snapshot)) return '复测通过后加入'
    return '校验绑定并加入调度'
  })

  const channelSchedulePrimaryIcon = computed<'key' | 'ban' | 'beaker' | 'play'>(() => {
    const snapshot = channelScheduleSnapshot.value
    if (!snapshot) return 'beaker'
    if (!channelHasLocalBinding(snapshot)) return 'key'
    if (snapshot.local_account_schedulable) return 'ban'
    if (!channelIsAvailable(snapshot)) return 'beaker'
    return 'play'
  })

  const channelScheduleLocalAccountLabel = computed(() => {
    const snapshot = channelScheduleSnapshot.value
    if (!channelHasLocalBinding(snapshot)) return '未绑定'
    if (snapshot?.local_account_schedulable) return `已调度 #${snapshot.local_sub2api_account_id}`
    return `待校验 #${snapshot?.local_sub2api_account_id}`
  })

  const channelScheduleLocalAccountBadgeClass = computed(() => {
    const snapshot = channelScheduleSnapshot.value
    if (!channelHasLocalBinding(snapshot)) return 'badge-warning'
    if (snapshot?.local_account_schedulable) return 'badge-success'
    return 'badge-warning'
  })

  const channelScheduleSteps = computed<ChannelScheduleStep[]>(() => {
    const snapshot = channelScheduleSnapshot.value
    if (!snapshot) {
      return [
        {
          key: 'probe',
          label: '渠道检测',
          description: '先检测供应商渠道，选择最低且可用的倍率分组。',
          status: 'pending',
          icon: 'beaker'
        },
        {
          key: 'local-account',
          label: '本地账号',
          description: '检测出可用渠道后再补齐供应商 Key 和本地 Sub2API 账号。',
          status: 'pending',
          icon: 'key'
        },
        {
          key: 'local-group',
          label: '本地分组绑定',
          description: '加入调度时会校验账号是否已经绑定到本地 Sub2API 分组。',
          status: 'pending',
          icon: 'link'
        }
      ]
    }

    const available = channelIsAvailable(snapshot)
    const hasLocalAccount = channelHasLocalBinding(snapshot)
    const localAccountScheduled = Boolean(snapshot.local_account_schedulable)
    return [
      {
        key: 'probe',
        label: '渠道检测',
        description: available
          ? `检测通过，首 Token ${formatLatency(snapshot.first_token_ms)}，总耗时 ${formatLatency(snapshot.duration_ms)}。`
          : snapshot.error_message || '需要复测通过后才会加入本地调度。',
        status: available ? 'done' : 'warning',
        icon: 'beaker'
      },
      {
        key: 'local-account',
        label: '本地账号',
        description: localAccountScheduled
          ? `本地账号 #${snapshot.local_sub2api_account_id} 已经参与调度。`
          : hasLocalAccount
            ? `检测快照记录了本地账号 #${snapshot.local_sub2api_account_id}，加入调度时会实时校验并修复绑定。`
            : '会先开通供应商 Key，并创建或绑定本地 Sub2API 账号。',
        status: localAccountScheduled ? 'done' : hasLocalAccount ? 'warning' : 'pending',
        icon: 'key'
      },
      {
        key: 'local-group',
        label: '本地分组绑定',
        description: localAccountScheduled
          ? '本地账号已经绑定到对应调度分组。'
          : hasLocalAccount
            ? '尚未确认本地分组绑定有效，点击加入调度会重新校验。'
            : '本地账号准备完成后自动绑定到对应调度分组。',
        status: localAccountScheduled ? 'done' : hasLocalAccount ? 'warning' : 'pending',
        icon: 'link'
      },
      {
        key: 'schedule',
        label: '调度状态',
        description: snapshot.local_account_schedulable ? '该账号已经参与本地 Sub2API 调度。' : '完成前置校验后可加入本地 Sub2API 调度。',
        status: snapshot.local_account_schedulable ? 'done' : 'pending',
        icon: 'play'
      }
    ]
  })

  const currentBalanceFailureMessage = computed(() => {
    const balance = currentBalanceValue.value
    if (balance?.fallback) return normalizeBalanceErrorMessage(balance.refresh_error_message || currentBalanceError.value)
    if (currentBalanceError.value) return normalizeBalanceErrorMessage(currentBalanceError.value)
    return ''
  })

  const currentBalanceBadgeText = computed(() => {
    const balance = currentBalanceValue.value
    if (currentBalanceFailureMessage.value) return '读取失败'
    if (!balance) return '未读取'
    if (balance.expired) return '已过期'
    if (balance.stale) return '待刷新'
    return '最新'
  })

  const currentBalanceBadgeClass = computed(() => {
    const balance = currentBalanceValue.value
    if (currentBalanceFailureMessage.value) return 'badge-danger'
    if (!balance) return 'badge-gray'
    if (balance.expired) return 'badge-danger'
    if (balance.stale) return 'badge-warning'
    return 'badge-success'
  })

  const currentBalanceCaption = computed(() => {
    const balance = currentBalanceValue.value
    if (currentBalanceFailureMessage.value) return `余额读取失败：${currentBalanceFailureMessage.value}`
    if (!balance) return '打开后读取 Redis 当前值，必要时可手动刷新'
    if (balance.stale) return '缓存已到刷新窗口，后台会重新获取'
    return '缓存有效，调度器会周期刷新'
  })

  const currentBalanceSourceLabel = computed(() => {
    if (currentBalanceFailureMessage.value) return '未读取'
    if (currentBalanceValue.value?.fallback) return '未读取'
    const source = currentBalanceValue.value?.source
    if (source === 'provider_session') return '供应商会话'
    if (source === 'fallback') return '兜底'
    return source || '-'
  })

  const currentBalanceAmountText = computed(() => {
    const balance = currentBalanceValue.value
    if (!balance) return currentBalanceFailureMessage.value ? '无法读取' : '-'
    if (balance.fallback) return '无法读取'
    return formatBalanceMoney(balance.balance_cents, balance.currency || 'USD')
  })

  const currentBalanceAmountClass = computed(() => {
    return currentBalanceValue.value?.fallback || currentBalanceFailureMessage.value
      ? 'text-red-600 dark:text-red-300'
      : 'text-gray-900 dark:text-gray-100'
  })

  const currentGroupSession = computed(() => {
    const supplierID = groupsSupplier.value?.id
    return supplierID ? sessionStore[supplierID] : undefined
  })

  const activeProvisionJobRunning = computed(() => {
    const status = activeProvisionJob.value?.status
    return status === 'queued' || status === 'running' || status === 'retryable_failed'
  })

  const groupWorkflowSteps = computed(() => {
    const hasSession = Boolean(currentGroupSession.value?.has_encrypted_bundle)
    const hasGroups = supplierGroups.value.length > 0
    const hasKeys = supplierKeys.value.length > 0
    const hasChannelChecks = Object.keys(supplierChannelChecks.value).length > 0
    const job = activeProvisionJob.value
    return [
      {
        key: 'session',
        label: '会话',
        status: hasSession ? 'succeeded' : 'manual_required',
        caption: hasSession ? sessionSourceLabel(currentGroupSession.value?.session_source) : '先直登或插件上报'
      },
      {
        key: 'groups',
        label: '分组',
        status: job?.job_type === 'sync_groups' ? job.status : (hasGroups ? 'succeeded' : 'queued'),
        caption: hasGroups ? `${supplierGroups.value.length} 个分组` : '待同步'
      },
      {
        key: 'keys',
        label: 'Key',
        status: job?.job_type === 'provision_all_group_keys' || job?.job_type === 'provision_group_key' ? job.status : (hasKeys ? 'succeeded' : 'queued'),
        caption: hasKeys ? `${supplierKeys.value.length} 个 Key` : '按分组补齐'
      },
      {
        key: 'verify',
        label: '检测',
        status: job?.job_type === 'check_supplier_channels' ? job.status : (hasChannelChecks ? 'succeeded' : 'queued'),
        caption: hasChannelChecks ? '已记录渠道快照' : (hasKeys ? '待检测' : '待账号绑定')
      }
    ] as Array<{ key: string; label: string; status: SupplierProvisionStatus; caption: string }>
  })

  const canSubmitGroupSync = computed(() => {
    return Boolean(groupsSupplier.value && currentGroupSession.value?.has_encrypted_bundle && !activeProvisionJobRunning.value && !groupsLoading.value)
  })

  const canSubmitEnsureKeys = computed(() => {
    return Boolean(groupsSupplier.value && supplierGroups.value.length > 0 && !activeProvisionJobRunning.value && !groupsLoading.value)
  })

  const currentSessionSummary = computed(() => currentSession.value?.session_summary || {})

  const supplierKeysByGroupID = computed(() => {
    const out = new Map<number, SupplierKey>()
    for (const key of supplierKeys.value) {
      const existing = out.get(key.supplier_group_id)
      if (!existing || key.id > existing.id) {
        out.set(key.supplier_group_id, key)
      }
    }
    return out
  })

  const summaryCookieCount = computed(() => {
    const value = currentSessionSummary.value.cookie_count
    return typeof value === 'number' ? value : 0
  })

  const capabilityBadges = computed(() => {
    const capabilities = lastProbe.value?.capabilities || {}
    return [
      { key: 'can_read_profile', label: 'Profile', enabled: Boolean(capabilities.can_read_profile) },
      { key: 'can_read_balance', label: '余额', enabled: Boolean(capabilities.can_read_balance) },
      { key: 'can_read_groups', label: '分组', enabled: Boolean(capabilities.can_read_groups) },
      { key: 'can_create_key', label: '创建 Key', enabled: Boolean(capabilities.can_create_key) },
      { key: 'can_read_usage_costs', label: '用量消耗', enabled: Boolean(capabilities.can_read_usage_costs) }
    ]
  })

  const channelStatusOverall = computed<SupplierMonitorStatus>(() => {
    if (channelMonitorItems.value.length === 0) return 'operational'
    if (channelMonitorItems.value.some((item) => item.primary_status === 'failed' || item.primary_status === 'error')) return 'failed'
    if (channelMonitorItems.value.some((item) => item.primary_status !== 'operational')) return 'degraded'
    return 'operational'
  })

  const channelStatusWindowOptions = computed<Array<{ value: ChannelStatusWindow; label: string; disabled: boolean }>>(() => {
    if (channelStatusSupplier.value?.type === 'new_api') {
      return [
        { value: 'pulse', label: '60 秒', disabled: false }
      ]
    }
    return [
      { value: '7d', label: '7 天', disabled: false },
      { value: '15d', label: '15 天', disabled: true },
      { value: '30d', label: '30 天', disabled: true }
    ]
  })

  const channelStatusOverallLabel = computed(() => {
    return channelStatusOverall.value === 'operational' ? 'OPERATIONAL' : 'DEGRADED'
  })

  const channelStatusOverallChipClass = computed(() => {
    if (channelStatusOverall.value === 'operational') {
      return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300'
    }
    return 'bg-amber-100 text-amber-700 dark:bg-amber-500/15 dark:text-amber-300'
  })

  const channelStatusOverallDotClass = computed(() => {
    if (channelStatusOverall.value === 'operational') return 'bg-emerald-500'
    return 'bg-amber-500'
  })

  const deleteConfirmMessage = computed(() => {
    if (bulkDeleteMode.value) {
      return `确认删除已选择的 ${selectedCount.value} 个供应商？该操作会删除供应商父级及其账号/Key 绑定。`
    }
    return deletingSupplier.value
      ? `确认删除供应商「${deletingSupplier.value.name}」？该操作会删除其账号/Key 绑定。`
      : '确认删除该供应商？'
  })

  Object.assign(ctx, {
    channelProtocolFilterLabel,
    scheduleListRows,
    filteredScheduleRows,
    scheduleListStats,
    currentSession,
    currentBalanceValue,
    channelScheduleSnapshot,
    channelSchedulePrimaryLabel,
    channelSchedulePrimaryIcon,
    channelScheduleLocalAccountLabel,
    channelScheduleLocalAccountBadgeClass,
    channelScheduleSteps,
    currentBalanceFailureMessage,
    currentBalanceBadgeText,
    currentBalanceBadgeClass,
    currentBalanceCaption,
    currentBalanceSourceLabel,
    currentBalanceAmountText,
    currentBalanceAmountClass,
    currentGroupSession,
    activeProvisionJobRunning,
    groupWorkflowSteps,
    canSubmitGroupSync,
    canSubmitEnsureKeys,
    currentSessionSummary,
    supplierKeysByGroupID,
    summaryCookieCount,
    capabilityBadges,
    channelStatusOverall,
    channelStatusWindowOptions,
    channelStatusOverallLabel,
    channelStatusOverallChipClass,
    channelStatusOverallDotClass,
    deleteConfirmMessage
  })
}
