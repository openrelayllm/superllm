import { computed, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { useTableSelection } from '@/composables/useTableSelection'
import { useAppStore } from '@/stores/app'
import { supplierDisplayUsageCents, supplierRechargeTotalCents } from '../supplierCostPresentation'
import type { Column } from '@/components/common/types'
import type { AdminGroup } from '@/types'
import type { LocalSub2APIAccount, Supplier, SupplierAccount, SupplierBrowserSession, SupplierChannelCheckSnapshot, SupplierChannelMonitorView, SupplierCostSnapshot, SupplierCurrentBalance, SupplierGroup, SupplierGroupStatus, SupplierHealthStatus, SupplierKey, SupplierProvisionJob, SupplierSessionProbeResult, SupplierKind, SupplierRuntimeStatus, SupplierType } from '@/api/admin/adminPlus'
import type { ChannelStatusWindow, ScheduleListStatusFilter, ScheduleListLocalGroupFilter, ChannelProtocol } from './types'

export function createSuppliersState() {
  const appStore = useAppStore()
  const route = useRoute()
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
  const supplierKeys = ref<SupplierKey[]>([])
  const supplierCostSnapshots = ref<Record<number, SupplierCostSnapshot | undefined>>({})
  const supplierBestChannels = ref<Record<number, SupplierChannelCheckSnapshot[] | undefined>>({})
  const supplierChannelChecks = ref<Record<number, SupplierChannelCheckSnapshot | undefined>>({})
  const activeProvisionJob = ref<SupplierProvisionJob | null>(null)
  const localAccounts = ref<LocalSub2APIAccount[]>([])
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
  const groupsSyncing = ref(false)
  const keysEnsuring = ref(false)
  const keyNamesStandardizing = ref(false)
  const channelChecksSyncing = ref(false)
  const channelScheduleSubmitting = ref(false)
  const scheduleListLoading = ref(false)
  const scheduleListActionKey = ref('')
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
  const provisionError = ref('')
  const provisionJobError = ref('')
  const channelCheckError = ref('')
  const scheduleListError = ref('')
  const repairError = ref('')
  const lastProbe = ref<SupplierSessionProbeResult | null>(null)
  const rowLoginSupplierID = ref<number | null>(null)
  const rowChannelCheckSupplierID = ref<number | null>(null)
  const rowBalanceRefreshingID = ref<number | null>(null)
  const quickStatusSupplierID = ref<number | null>(null)
  const channelCheckActionKey = ref('')
  const localOpenAIGroups = ref<AdminGroup[]>([])
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
    health_status: '' as '' | SupplierHealthStatus
  })
  const channelProtocolFilter = ref<ChannelProtocol | ''>('openai')
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

  const repairForm = reactive({
    local_sub2api_account_id: 0,
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
    { key: 'account', label: 'Key / 本地账号', class: 'w-[274px] max-w-[274px]' },
    { key: 'channel_check', label: '检测', class: 'w-[214px] max-w-[214px]' },
    { key: 'status', label: '状态', class: 'w-[88px] max-w-[88px]' },
    { key: 'last_seen_at', label: '最后同步', sortable: true, class: 'w-[142px] max-w-[142px]' },
    { key: 'group_actions', label: '操作', class: 'w-[114px] max-w-[114px] text-right' }
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
    supplierKeys,
    supplierCostSnapshots,
    supplierBestChannels,
    supplierChannelChecks,
    activeProvisionJob,
    localAccounts,
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
    groupsSyncing,
    keysEnsuring,
    keyNamesStandardizing,
    channelChecksSyncing,
    channelScheduleSubmitting,
    scheduleListLoading,
    scheduleListActionKey,
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
    provisionError,
    provisionJobError,
    channelCheckError,
    scheduleListError,
    repairError,
    lastProbe,
    rowLoginSupplierID,
    rowChannelCheckSupplierID,
    rowBalanceRefreshingID,
    quickStatusSupplierID,
    channelCheckActionKey,
    localOpenAIGroups,
    provisionJobTimer,
    channelStatusAutoRefreshTimer,
    ROW_ACTIONS_MENU_WIDTH,
    ROW_ACTIONS_MENU_HEIGHT,
    ROW_ACTIONS_MENU_MARGIN,
    filters,
    channelProtocolFilter,
    pagination,
    groupPagination,
    groupFilters,
    form,
    statusForm,
    provisionForm,
    keyNamingForm,
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
