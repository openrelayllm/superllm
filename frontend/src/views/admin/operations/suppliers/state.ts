import { computed, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { useTableSelection } from '@/composables/useTableSelection'
import { useAppStore } from '@/stores/app'
import { supplierDisplayUsageCents, supplierRechargeTotalCents } from '../supplierCostPresentation'
import type { Column } from '@/components/common/types'
import type { AdminGroup } from '@/types'
import type { EnsureSupplierKeysPlan, LocalAccountOpsRow, LocalSub2APIAccount, Supplier, SupplierAccount, SupplierBrowserSession, SupplierCapabilityStatus, SupplierChannelCheckOverviewRow, SupplierChannelCheckSnapshot, SupplierChannelMonitorView, SupplierCostSnapshot, SupplierCurrentBalance, SupplierGroup, SupplierGroupChangeEvent, SupplierGroupStatus, SupplierHealthStatus, SupplierIntegrationProtocol, SupplierKey, SupplierProvisionJob, SupplierSessionProbeResult, SupplierKind, SupplierRuntimeStatus, SupplierType } from '@/api/admin/adminPlus'
import type { ChannelStatusWindow, ScheduleListStatusFilter, ScheduleListLocalGroupFilter, ChannelProtocol, RateCheckMode, RateCheckProtocol } from './types'

export function createSuppliersState() {
  const appStore = useAppStore()
  const route = useRoute()
  const router = useRouter()
  const handledDeepLinkKey = ref('')

  const loading = ref(false)
  const submitting = ref(false)
  const statusSubmitting = ref(false)
  const provisionSubmitting = ref(false)
  const repairSubmitting = ref(false)
  const editorOpen = ref(false)
  const statusDialogOpen = ref(false)
  const sessionDialogOpen = ref(false)
  const channelStatusDialogOpen = ref(false)
  const groupsDialogOpen = ref(false)
  const supplierDetailDialogOpen = ref(false)
  const channelProbeDialogOpen = ref(false)
  const provisionDialogOpen = ref(false)
  const repairDialogOpen = ref(false)
  const channelScheduleDialogOpen = ref(false)
  const scheduleListDialogOpen = ref(false)
  const deleteDialogOpen = ref(false)
  const moreMenuOpen = ref(false)
  const bulkStatusMode = ref(false)
  const bulkDeleteMode = ref(false)
  const bulkBalanceRefreshing = ref(false)
  const bulkChannelChecksSyncing = ref(false)
  const editingSupplier = ref<Supplier | null>(null)
  const sessionSupplier = ref<Supplier | null>(null)
  const channelStatusSupplier = ref<Supplier | null>(null)
  const groupsSupplier = ref<Supplier | null>(null)
  const supplierDetailSupplier = ref<Supplier | null>(null)
  const channelProbeSupplier = ref<Supplier | null>(null)
  const channelProbeSnapshot = ref<SupplierChannelCheckSnapshot | null>(null)
  const provisionGroup = ref<SupplierGroup | null>(null)
  const repairKey = ref<SupplierKey | null>(null)
  const channelScheduleSupplier = ref<Supplier | null>(null)
  const deletingSupplier = ref<Supplier | null>(null)
  const rowActionsMenuSupplier = ref<Supplier | null>(null)
  const rowActionsMenuStyle = ref<Record<string, string>>({})
  const suppliers = ref<Supplier[]>([])
  const supplierGroups = ref<SupplierGroup[]>([])
  const supplierGroupEvents = ref<SupplierGroupChangeEvent[]>([])
  const supplierKeys = ref<SupplierKey[]>([])
  const ensureKeysPlan = ref<EnsureSupplierKeysPlan | null>(null)
  const supplierCostSnapshots = ref<Record<number, SupplierCostSnapshot | undefined>>({})
  const supplierBestChannels = ref<Record<number, SupplierChannelCheckSnapshot[] | undefined>>({})
  const supplierChannelChecks = ref<Record<number, SupplierChannelCheckSnapshot | undefined>>({})
  const activeProvisionJob = ref<SupplierProvisionJob | null>(null)
  const localAccounts = ref<LocalSub2APIAccount[]>([])
  const supplierDetailGroups = ref<SupplierGroup[]>([])
  const supplierDetailKeys = ref<SupplierKey[]>([])
  const supplierDetailAccounts = ref<SupplierAccount[]>([])
  const supplierDetailLocalAccounts = ref<LocalSub2APIAccount[]>([])
  const supplierDetailLocalOpsRows = ref<LocalAccountOpsRow[]>([])
  const supplierDetailChannelChecks = ref<SupplierChannelCheckSnapshot[]>([])
  const channelMonitorItems = ref<SupplierChannelMonitorView[]>([])
  const sessionStore = reactive<Record<number, SupplierBrowserSession | undefined>>({})
  const currentBalanceStore = reactive<Record<number, SupplierCurrentBalance | undefined>>({})
  const sessionLoading = ref(false)
  const loggingInSession = ref(false)
  const probingSession = ref(false)
  const currentBalanceLoading = ref(false)
  const currentBalanceError = ref('')
  const channelStatusLoading = ref(false)
  const groupsLoading = ref(false)
  const supplierDetailLoading = ref(false)
  const groupsSyncing = ref(false)
  const keysEnsuring = ref(false)
  const ensureKeysPlanning = ref(false)
  const keyNamesStandardizing = ref(false)
  const keyProjectionDisabling = ref<number | null>(null)
  const providerKeyImportingGroupID = ref<number | null>(null)
  const providerKeyBatchImporting = ref(false)
  const channelChecksSyncing = ref(false)
  const channelScheduleSubmitting = ref(false)
  const scheduleListLoading = ref(false)
  const scheduleListActionKey = ref('')
  const rateCheckLoading = ref(false)
  const rateCheckSchedulerSubmitting = ref(false)
  const rateCheckActionKey = ref('')
  const repairAccountsLoading = ref(false)
  const sessionLoadError = ref('')
  const channelStatusError = ref('')
  const channelMonitorCapturedAt = ref('')
  const channelMonitorOrigin = ref('')
  const channelMonitorAPIBaseURL = ref('')
  const channelStatusWindow = ref<ChannelStatusWindow>('7d')
  const channelStatusAutoRefresh = ref(true)
  const channelStatusCountdown = ref(16)
  const groupsError = ref('')
  const supplierDetailError = ref('')
  const provisionError = ref('')
  const provisionJobError = ref('')
  const ensureKeysPlanError = ref('')
  const channelCheckError = ref('')
  const scheduleListError = ref('')
  const rateCheckError = ref('')
  const repairError = ref('')
  const lastProbe = ref<SupplierSessionProbeResult | null>(null)
  const rowLoginSupplierID = ref<number | null>(null)
  const rowChannelCheckSupplierID = ref<number | null>(null)
  const rowBalanceRefreshingID = ref<number | null>(null)
  const quickStatusSupplierID = ref<number | null>(null)
  const channelCheckActionKey = ref('')
  const localOpenAIGroups = ref<AdminGroup[]>([])
  const rateCheckRows = ref<SupplierChannelCheckOverviewRow[]>([])
  const rateCheckLocalGroups = ref<AdminGroup[]>([])
  const provisionJobTimer = ref<ReturnType<typeof window.setTimeout> | undefined>()
  const channelStatusAutoRefreshTimer = ref<ReturnType<typeof window.setInterval> | undefined>()

  const ROW_ACTIONS_MENU_WIDTH = 224
  const ROW_ACTIONS_MENU_HEIGHT = 304
  const ROW_ACTIONS_MENU_MARGIN = 8

  const filters = reactive({
    q: typeof route.query.q === 'string' ? route.query.q : '',
    kind: '' as '' | SupplierKind,
    type: '' as '' | SupplierType,
    runtime_status: '' as '' | SupplierRuntimeStatus,
    health_status: '' as '' | SupplierHealthStatus,
    capability_status: '' as '' | SupplierCapabilityStatus,
    integration_protocol: '' as '' | SupplierIntegrationProtocol
  })
  const channelProtocolFilter = ref<ChannelProtocol | ''>('openai')
  const rateCheckProtocol = ref<RateCheckProtocol>('openai')
  const rateCheckMode = ref<RateCheckMode>('best')
  const rateCheckSelectedLocalGroupID = ref<number | ''>('')
  const pagination = reactive({
    page: 1,
    page_size: getPersistedPageSize(),
    total: 0,
    pages: 0
  })

  const groupPagination = reactive({
    page: 1,
    page_size: getPersistedPageSize(),
    total: 0,
    pages: 0
  })

  const groupFilters = reactive({
    q: '',
    status: '' as '' | SupplierGroupStatus
  })

  const form = reactive({
    name: '',
    kind: 'relay' as SupplierKind,
    type: 'sub2api' as SupplierType,
    runtime_status: 'monitor_only' as SupplierRuntimeStatus,
    health_status: 'normal' as SupplierHealthStatus,
    dashboard_url: '',
    api_base_url: '',
    third_party_recharge_url: '',
    local_recharge_url: '',
    contact: '',
    browser_login_username: '',
    browser_login_password: '',
    browser_login_token: '',
    balance_yuan: 0,
    balance_currency: 'USD',
    recharge_multiplier: 1,
    key_limit_policy: 'unknown',
    key_limit_value: 0,
    browser_login_enabled: true,
    notes: ''
  })

  const statusForm = reactive({
    id: 0,
    name: '',
    runtime_status: 'monitor_only' as SupplierRuntimeStatus,
    health_status: 'normal' as SupplierHealthStatus
  })

  const provisionForm = reactive({
    name: '',
    sync_provider_name: false,
    local_account_name: '',
    local_account_platform: 'openai',
    local_account_base_url: '',
    local_account_concurrency: 0,
    local_account_priority: 100,
    local_account_rate_multiplier: 1,
    quota_usd: 0,
    expires_in_days: null as number | null,
    runtime_status: 'monitor_only' as SupplierRuntimeStatus,
    health_status: 'normal' as SupplierHealthStatus,
    balance_yuan: 0,
    balance_threshold_yuan: 0,
    balance_currency: 'USD'
  })

  const keyNamingForm = reactive({
    sync_provider_name: false
  })
  const ensureKeysPriorityGroupIDs = ref<number[]>([])

  const repairForm = reactive({
    mode: 'bind_existing' as 'bind_existing' | 'manual_secret',
    local_sub2api_account_id: 0,
    manual_secret: '',
    local_account_platform: 'openai',
    local_account_name: '',
    local_account_base_url: '',
    local_account_priority: 100,
    local_account_rate_multiplier: 1,
    runtime_status: 'monitor_only' as SupplierRuntimeStatus,
    health_status: 'normal' as SupplierHealthStatus,
    configured_concurrency: 0,
    balance_yuan: 0,
    balance_threshold_yuan: 0,
    balance_currency: 'USD'
  })

  const scheduleListFilters = reactive({
    q: '',
    status: '' as ScheduleListStatusFilter,
    local_group: '' as ScheduleListLocalGroupFilter
  })

  const scheduleListSuppliers = ref<Supplier[]>([])
  const scheduleListBindings = ref<SupplierAccount[]>([])
  const scheduleListLocalAccounts = ref<Record<number, LocalSub2APIAccount | undefined>>({})
  const scheduleListChannelChecks = ref<Record<string, SupplierChannelCheckSnapshot | undefined>>({})

  const columns: Column[] = [
    { key: 'select', label: '', class: 'w-10' },
    { key: 'name', label: '供应商', sortable: true },
    { key: 'best_channel', label: '有效低倍率渠道', class: 'w-[312px] max-w-[312px]' },
    { key: 'balance_cents', label: '余额 / 成本', sortable: true },
    { key: 'status', label: '状态', class: 'w-[202px] max-w-[202px]' },
    { key: 'kind_type', label: '归类 / 类型' },
    { key: 'credential', label: '采集凭据' },
    { key: 'created_at', label: '创建时间', sortable: true },
    { key: 'actions', label: '操作', class: 'text-right' }
  ]

  const groupColumns: Column[] = [
    { key: 'name', label: '分组', class: 'w-[214px] max-w-[214px]' },
    { key: 'provider_family', label: '平台', class: 'w-[96px] max-w-[96px]' },
    { key: 'rate', label: '倍率', class: 'w-[110px] max-w-[110px] text-right' },
    { key: 'limits', label: '限制', class: 'w-[106px] max-w-[106px]' },
    { key: 'key_capacity', label: '分组配额', class: 'w-[132px] max-w-[132px]' },
    { key: 'account', label: 'Key / 本地账号', class: 'w-[274px] max-w-[274px]' },
    { key: 'channel_check', label: '检测', class: 'w-[214px] max-w-[214px]' },
    { key: 'status', label: '状态', class: 'w-[88px] max-w-[88px]' },
    { key: 'last_seen_at', label: '最后同步', sortable: true, class: 'w-[142px] max-w-[142px]' },
    { key: 'group_actions', label: '操作', class: 'w-[126px] max-w-[126px] text-right' }
  ]

  const scheduleListColumns: Column[] = [
    { key: 'name', label: '账号', sortable: true, class: 'min-w-[280px]' },
    { key: 'status', label: '状态', class: 'min-w-[132px]' },
    { key: 'schedulable', label: '调度', sortable: true, class: 'w-28' },
    { key: 'group', label: '渠道 / 本地分组', class: 'min-w-[340px] max-w-[380px]' },
    { key: 'probe', label: '检测结果', class: 'min-w-[210px]' },
    { key: 'actions', label: '操作', class: 'w-[132px] text-right' }
  ]

  const filteredSuppliers = computed(() => suppliers.value)

  const {
    selectedIds,
    selectedCount,
    allVisibleSelected,
    isSelected,
    toggle: toggleSelection,
    clear: clearSelection,
    selectVisible,
    toggleVisible
  } = useTableSelection<Supplier>({
    rows: filteredSuppliers,
    getId: (row) => row.id
  })

  const selectedRows = computed(() => suppliers.value.filter((item) => selectedIds.value.includes(item.id)))


  return {
    appStore,
    route,
    router,
    handledDeepLinkKey,
    loading,
    submitting,
    statusSubmitting,
    provisionSubmitting,
    repairSubmitting,
    editorOpen,
    statusDialogOpen,
    sessionDialogOpen,
    channelStatusDialogOpen,
    groupsDialogOpen,
    supplierDetailDialogOpen,
    channelProbeDialogOpen,
    provisionDialogOpen,
    repairDialogOpen,
    channelScheduleDialogOpen,
    scheduleListDialogOpen,
    deleteDialogOpen,
    moreMenuOpen,
    bulkStatusMode,
    bulkDeleteMode,
    bulkBalanceRefreshing,
    bulkChannelChecksSyncing,
    editingSupplier,
    sessionSupplier,
    channelStatusSupplier,
    groupsSupplier,
    supplierDetailSupplier,
    channelProbeSupplier,
    channelProbeSnapshot,
    provisionGroup,
    repairKey,
    channelScheduleSupplier,
    deletingSupplier,
    rowActionsMenuSupplier,
    rowActionsMenuStyle,
    suppliers,
    supplierGroups,
    supplierGroupEvents,
    supplierKeys,
    ensureKeysPlan,
    supplierCostSnapshots,
    supplierBestChannels,
    supplierChannelChecks,
    activeProvisionJob,
    localAccounts,
    supplierDetailGroups,
    supplierDetailKeys,
    supplierDetailAccounts,
    supplierDetailLocalAccounts,
    supplierDetailLocalOpsRows,
    supplierDetailChannelChecks,
    channelMonitorItems,
    sessionStore,
    currentBalanceStore,
    sessionLoading,
    loggingInSession,
    probingSession,
    currentBalanceLoading,
    currentBalanceError,
    channelStatusLoading,
    groupsLoading,
    supplierDetailLoading,
    groupsSyncing,
    keysEnsuring,
    ensureKeysPlanning,
    keyNamesStandardizing,
    keyProjectionDisabling,
    providerKeyImportingGroupID,
    providerKeyBatchImporting,
    channelChecksSyncing,
    channelScheduleSubmitting,
    scheduleListLoading,
    scheduleListActionKey,
    rateCheckLoading,
    rateCheckSchedulerSubmitting,
    rateCheckActionKey,
    repairAccountsLoading,
    sessionLoadError,
    channelStatusError,
    channelMonitorCapturedAt,
    channelMonitorOrigin,
    channelMonitorAPIBaseURL,
    channelStatusWindow,
    channelStatusAutoRefresh,
    channelStatusCountdown,
    groupsError,
    supplierDetailError,
    provisionError,
    provisionJobError,
    ensureKeysPlanError,
    channelCheckError,
    scheduleListError,
    rateCheckError,
    repairError,
    lastProbe,
    rowLoginSupplierID,
    rowChannelCheckSupplierID,
    rowBalanceRefreshingID,
    quickStatusSupplierID,
    channelCheckActionKey,
    localOpenAIGroups,
    rateCheckRows,
    rateCheckLocalGroups,
    provisionJobTimer,
    channelStatusAutoRefreshTimer,
    ROW_ACTIONS_MENU_WIDTH,
    ROW_ACTIONS_MENU_HEIGHT,
    ROW_ACTIONS_MENU_MARGIN,
    filters,
    channelProtocolFilter,
    rateCheckProtocol,
    rateCheckMode,
    rateCheckSelectedLocalGroupID,
    pagination,
    groupPagination,
    groupFilters,
    form,
    statusForm,
    provisionForm,
    keyNamingForm,
    ensureKeysPriorityGroupIDs,
    repairForm,
    scheduleListFilters,
    scheduleListSuppliers,
    scheduleListBindings,
    scheduleListLocalAccounts,
    scheduleListChannelChecks,
    columns,
    groupColumns,
    scheduleListColumns,
    filteredSuppliers,
    selectedIds,
    selectedCount,
    allVisibleSelected,
    isSelected,
    toggleSelection,
    clearSelection,
    selectVisible,
    toggleVisible,
    selectedRows,
    supplierDisplayUsageCents,
    supplierRechargeTotalCents
  }
}
