<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <SupplierAccountsToolbar
          :filters="filters"
          :selected-supplier-id="selectedSupplierId"
          :suppliers="suppliers"
	          :type-options="typeOptions"
	          :group-options="groupOptions"
	          :column-options="toggleableColumns"
	          :visible-column-keys="visibleColumnKeys"
	          :loading="loading"
          @update:filters="updateFilters"
          @update:selected-supplier-id="selectedSupplierId = $event"
          @refresh="loadAll"
          @reset-filters="resetFilters"
          @select-current-page="selectCurrentPage"
	          @clear-selection="clearSelection"
	          @create-account="goCreateAccount"
	          @toggle-column="toggleColumn"
	        />
      </template>

      <template #table>
        <div class="mb-4 flex flex-wrap items-center justify-between gap-3 rounded-lg bg-primary-50 p-3 dark:bg-primary-900/20">
          <div class="flex flex-wrap items-center gap-2 text-sm">
            <span class="font-medium text-primary-900 dark:text-primary-100">{{ selectedIds.length > 0 ? `已选择 ${selectedIds.length} 个账号` : '批量编辑账号' }}</span>
            <template v-if="selectedIds.length > 0">
              <button class="text-xs font-medium text-primary-700 hover:text-primary-800 dark:text-primary-300" @click="selectCurrentPage">选择当前页</button>
              <span class="text-primary-200 dark:text-primary-700">•</span>
              <button class="text-xs font-medium text-primary-700 hover:text-primary-800 dark:text-primary-300" @click="clearSelection">清空</button>
            </template>
          </div>
          <div class="flex flex-wrap gap-2">
            <button type="button" class="btn btn-secondary btn-sm" :disabled="selectedIds.length !== 1" title="请选择一个账号测试" @click="testSelected">测试账号</button>
            <button type="button" class="btn btn-secondary btn-sm" :disabled="selectedIds.length !== 1 || !selectedSupportsPurity" title="请选择一个 OpenAI API Key 账号检测纯度" @click="puritySelected">纯度检测</button>
            <button type="button" class="btn btn-secondary btn-sm" :disabled="selectedIds.length !== 1" title="请选择一个账号查看分组" @click="openSelectedSupplier">查看分组</button>
            <button type="button" class="btn btn-primary btn-sm" :disabled="selectedIds.length !== 1" title="请选择一个账号后到供应商分组更新" @click="openSelectedSupplier">批量更新</button>
          </div>
        </div>

	        <DataTable
	          :columns="visibleColumns"
          :data="pagedBindings"
          :loading="loadingBindings"
          row-key="id"
          default-sort-key="id"
          default-sort-order="desc"
          :estimate-row-height="72"
        >
          <template #header-select>
            <input
              type="checkbox"
              class="h-4 w-4 cursor-pointer rounded border-gray-300 text-primary-600 focus:ring-primary-500"
              :checked="allCurrentPageSelected"
              @click.stop
              @change="toggleSelectCurrentPage(($event.target as HTMLInputElement).checked)"
            />
          </template>
          <template #cell-select="{ row }">
            <input
              type="checkbox"
              class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
              :checked="selectedSet.has(row.id)"
              @change="toggleSelect(row.id)"
            />
          </template>

          <template #cell-name="{ row }">
            <div class="min-w-[260px]">
              <div class="flex items-center gap-1.5">
	                <span class="font-medium text-gray-900 dark:text-white">{{ localAccount(row)?.name || row.local_account_name }}</span>
              </div>
              <div class="mt-1 flex items-center gap-2">
	                <code class="code text-xs">#{{ row.local_sub2api_account_id }}</code>
	                <span v-if="supplierKeyDisplay(row) !== '-'" class="text-xs text-gray-400 dark:text-dark-500">{{ supplierKeyDisplay(row) }}</span>
              </div>
              <div class="mt-1 flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-dark-400">
	                <span>{{ supplierRelationLabel(row) }}</span>
	                <span v-if="row.supplier_account_label">{{ row.supplier_account_label }}</span>
              </div>
            </div>
          </template>

          <template #cell-account_id="{ row }">
            <span class="font-mono text-xs text-gray-500 dark:text-dark-400">#{{ row.local_sub2api_account_id }}</span>
          </template>

          <template #cell-platform_type="{ row }">
	            <div class="flex min-w-[130px] flex-col gap-1">
	              <div class="flex flex-wrap items-center gap-1">
	                <span class="badge" :class="platformBadgeClass(localAccount(row)?.platform || row.local_account_platform)">{{ platformLabel(localAccount(row)?.platform || row.local_account_platform) }}</span>
	                <span class="badge badge-gray">{{ typeShortLabel(localAccount(row)?.type || row.local_account_type) }}</span>
	              </div>
	            </div>
          </template>

          <template #cell-capacity="{ row }">
            <div class="min-w-[116px]">
              <div class="h-2 w-24 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700">
                <div class="h-full rounded-full bg-amber-300" :style="{ width: capacityPercent(row) + '%' }" />
              </div>
	              <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ accountRuntime(row)?.current_concurrency || row.observed_max_concurrency || 0 }} / {{ accountRuntime(row)?.configured_limit || localAccount(row)?.concurrency || row.configured_concurrency || 0 }}</div>
            </div>
          </template>

          <template #cell-status="{ row }">
	            <div class="flex min-w-[108px] flex-col gap-1.5">
	              <span class="badge w-fit" :class="accountStatusClass(localAccount(row)?.status || '')">{{ accountStatusLabel(localAccount(row)?.status || '') }}</span>
	              <span v-if="accountBlockReason(row)" class="badge badge-warning w-fit">{{ accountBlockReason(row) }}</span>
	            </div>
          </template>

	          <template #cell-groups="{ row }">
	            <div class="min-w-[220px]">
	              <div v-if="displayGroupNames(row).length > 0" class="flex flex-wrap gap-1">
	                <GroupBadge
	                  v-for="name in displayGroupNames(row)"
	                  :key="name"
	                  :name="name"
	                  :platform="groupPlatform(row)"
	                  :show-rate="false"
	                />
	              </div>
	              <span v-else class="text-sm text-gray-400 dark:text-dark-500">-</span>
	              <div class="mt-1 flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-dark-400">
	                <span v-if="supplierGroupLabel(row)">供应商分组：{{ supplierGroupLabel(row) }}</span>
	                <span v-if="bindingsForAccount(row).length > 0">{{ bindingsForAccount(row).length }} 个供应商绑定</span>
	                <span v-if="row.supplier_external_group_id" class="font-mono">#{{ row.supplier_external_group_id }}</span>
	              </div>
	            </div>
	          </template>

          <template #cell-usage="{ row }">
            <div class="min-w-[150px] text-sm">
              <div><span class="text-gray-500 dark:text-dark-400">今日:</span> <span class="font-medium">{{ formatMoneyCompact(accountUsage(row).today.account_cost_cents, 'USD') }}</span></div>
              <div class="mt-0.5"><span class="text-gray-500 dark:text-dark-400">近30天:</span> <span class="font-medium">{{ formatMoneyCompact(accountUsage(row).last30d.account_cost_cents, 'USD') }}</span></div>
              <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ formatInteger(accountUsage(row).last30d.total_tokens) }} tokens</div>
            </div>
          </template>

	          <template #cell-today_stats="{ row }">
	            <div class="min-w-[128px] text-sm">
	              <div class="font-medium text-gray-900 dark:text-gray-100">{{ accountUsage(row).today.request_count }} 次</div>
	              <div class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">{{ formatInteger(accountUsage(row).today.total_tokens) }} tokens</div>
	            </div>
	          </template>

	          <template #cell-last_used_at="{ row }">
	            <span class="text-sm text-gray-500 dark:text-dark-400">{{ formatRelativeDateTime(localAccount(row)?.last_used_at || lastUsedAt(row)) }}</span>
	          </template>

	          <template #cell-created_at="{ row }">
	            <span class="text-sm text-gray-500 dark:text-dark-400">{{ formatDateTime(localAccount(row)?.created_at || row.created_at) }}</span>
	          </template>

	          <template #cell-updated_at="{ row }">
	            <span class="text-sm text-gray-500 dark:text-dark-400">{{ formatDateTime(localAccount(row)?.updated_at || row.updated_at) }}</span>
	          </template>

	          <template #cell-expires_at="{ row }">
	            <div class="min-w-[126px]">
	              <span class="text-sm text-gray-500 dark:text-dark-400">{{ formatDateTime(localAccount(row)?.expires_at) }}</span>
	              <div v-if="localAccount(row)?.expires_at && localAccount(row)?.auto_pause_on_expired" class="mt-1 text-xs text-emerald-700 dark:text-emerald-300">到期自动暂停</div>
	            </div>
	          </template>

	          <template #cell-priority="{ row }">
	            <span class="text-sm text-gray-700 dark:text-gray-300">{{ localAccount(row)?.priority ?? '-' }}</span>
	          </template>

	          <template #cell-rate_multiplier="{ row }">
	            <span class="rounded-md bg-emerald-50 px-2 py-1 font-mono text-base font-bold text-emerald-800 dark:bg-emerald-900/25 dark:text-emerald-200">
	              {{ formatRate(localAccount(row)?.rate_multiplier) }}
	            </span>
	          </template>

	          <template #cell-notes="{ row }">
	            <span v-if="localAccount(row)?.notes" class="block max-w-[220px] truncate text-sm text-gray-600 dark:text-gray-300" :title="localAccount(row)?.notes">{{ localAccount(row)?.notes }}</span>
	            <span v-else class="text-sm text-gray-400 dark:text-dark-500">-</span>
	          </template>

          <template #cell-actions="{ row }">
            <div class="flex min-w-[150px] items-center justify-end gap-1">
              <button type="button" class="row-action hover:bg-emerald-50 hover:text-emerald-600 dark:hover:bg-emerald-900/20 dark:hover:text-emerald-300" title="测试渠道" @click="openTestDialog(row)">
                <Icon name="beaker" size="sm" />
                <span class="text-xs">测试</span>
              </button>
              <button
                type="button"
                class="row-action hover:bg-primary-50 hover:text-primary-600 disabled:cursor-not-allowed disabled:opacity-40 dark:hover:bg-primary-900/20 dark:hover:text-primary-400"
                :disabled="!supportsPurity(row)"
                title="检测 OpenAI API 纯度"
                @click="openPurityDialog(row)"
              >
                <Icon name="shield" size="sm" />
                <span class="text-xs">纯度</span>
              </button>
              <button type="button" class="row-action hover:bg-primary-50 hover:text-primary-600 dark:hover:bg-primary-900/20 dark:hover:text-primary-400" title="查看供应商分组" @click="goSupplierGroups(row)">
                <Icon name="externalLink" size="sm" />
                <span class="text-xs">分组</span>
              </button>
              <button type="button" class="row-action hover:bg-gray-100 hover:text-primary-600 dark:hover:bg-dark-700 dark:hover:text-primary-400" title="刷新" @click="loadAll">
                <Icon name="refresh" size="sm" />
                <span class="text-xs">刷新</span>
              </button>
            </div>
          </template>

          <template #empty>
            <EmptyState title="暂无账号/Key 绑定" description="请进入供应商详情同步分组，然后创建缺失 Key。" />
          </template>
        </DataTable>
      </template>

      <template #pagination>
        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </template>
    </TablePageLayout>

    <LocalAccountTestModal :show="Boolean(testingAccount)" :account="testingAccount" @close="testingAccount = null" />
    <LocalAccountPurityModal :show="Boolean(purityAccount)" :account="purityAccount" @close="purityAccount = null" />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import GroupBadge from '@/components/common/GroupBadge.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'
import LocalAccountTestModal from '@/components/admin-plus/LocalAccountTestModal.vue'
import LocalAccountPurityModal from '@/components/admin-plus/LocalAccountPurityModal.vue'
import { loadAllPagedItems } from '@/utils/loadAllPages'
import SupplierAccountsToolbar from './SupplierAccountsToolbar.vue'
import {
  accountStatusClass,
  accountStatusLabel,
  formatDateTime,
  formatInteger,
  formatMoneyCompact,
  formatRate,
  formatRelativeDateTime,
  normalizePlatform,
  normalizeType,
  platformBadgeClass,
  platformLabel,
  typeShortLabel
} from './SupplierAccountsUtils'
import type { Column } from '@/components/common/types'
import type { GroupPlatform } from '@/types'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { useAppStore } from '@/stores/app'
import {
  listLocalAccountUsageSummary,
  listLocalAccountRuntime,
  listLocalSub2APIAccounts,
  listSupplierAccounts,
  listSuppliers,
  type LocalAccountUsageSummary,
  type LocalAccountRuntime,
  type LocalSub2APIAccount,
  type Supplier,
  type SupplierAccount
} from '@/api/admin/adminPlus'

const appStore = useAppStore()
const route = useRoute()
const router = useRouter()

const loading = ref(false)
const loadingBindings = ref(false)
const suppliers = ref<Supplier[]>([])
const bindings = ref<SupplierAccount[]>([])
const supplierBindings = ref<SupplierAccount[]>([])
const localAccountsByID = ref<Record<number, LocalSub2APIAccount>>({})
const runtimeByAccountID = ref<Record<number, LocalAccountRuntime>>({})
const usageByAccountID = ref<Record<number, AccountUsageWindow>>({})
const selectedSupplierId = ref(0)
const selectedIds = ref<number[]>([])
const testingAccount = ref<LocalSub2APIAccount | null>(null)
const purityAccount = ref<LocalSub2APIAccount | null>(null)
const suppressSupplierWatch = ref(false)
const hiddenColumns = reactive(new Set<string>(['notes', 'updated_at']))

interface UsageSummary {
  request_count: number
  input_tokens: number
  output_tokens: number
  total_tokens: number
  revenue_cents: number
  account_cost_cents: number
  last_request_created_at: string
}

interface AccountUsageWindow {
  today: UsageSummary
  last30d: UsageSummary
}

const filters = reactive({
  q: '',
  platform: '',
  type: '',
  status: '',
  group: ''
})
const pagination = reactive({ page: 1, page_size: getPersistedPageSize(), total: 0, pages: 0 })

const columns: Column[] = [
	  { key: 'select', label: '' },
	  { key: 'name', label: '名称', sortable: true },
	  { key: 'account_id', label: '账号ID', sortable: true },
	  { key: 'platform_type', label: '平台/类型' },
	  { key: 'capacity', label: '容量' },
	  { key: 'status', label: '状态' },
	  { key: 'today_stats', label: '今日统计' },
	  { key: 'groups', label: '分组' },
	  { key: 'usage', label: '用量窗口' },
	  { key: 'priority', label: '优先级', sortable: true },
	  { key: 'rate_multiplier', label: '倍率', sortable: true },
	  { key: 'last_used_at', label: '最近使用', sortable: true },
	  { key: 'created_at', label: '创建时间', sortable: true },
	  { key: 'updated_at', label: '更新时间', sortable: true },
	  { key: 'expires_at', label: '过期时间', sortable: true },
	  { key: 'notes', label: '备注' },
	  { key: 'actions', label: '操作', class: 'text-right' }
	]

const selectedSet = computed(() => new Set(selectedIds.value))
const selectedSupportsPurity = computed(() => {
  const first = selectedRows()[0]
  return Boolean(first && supportsPurity(first))
})
const filteredBindings = computed(() => sortBindingsDesc(bindings.value.filter(matchesFilters)))
const pagedBindings = computed(() => {
  const start = (pagination.page - 1) * pagination.page_size
  return filteredBindings.value.slice(start, start + pagination.page_size)
})
const allCurrentPageSelected = computed(() => pagedBindings.value.length > 0 && pagedBindings.value.every((row) => selectedSet.value.has(row.id)))
const visibleColumns = computed(() => columns.filter((column) => column.key === 'select' || column.key === 'name' || column.key === 'actions' || !hiddenColumns.has(column.key)))
const toggleableColumns = computed(() => columns.filter((column) => column.key !== 'select' && column.key !== 'name' && column.key !== 'actions').map((column) => ({ key: column.key, label: column.label })))
const visibleColumnKeys = computed(() => visibleColumns.value.map((column) => column.key))
const typeOptions = computed(() => {
  const values = new Map<string, string>()
  for (const row of bindings.value) {
    const value = normalizeType(row.local_account_type)
    if (value) values.set(value, typeShortLabel(row.local_account_type))
  }
  return [...values.entries()].map(([value, label]) => ({ value, label }))
})
const groupOptions = computed(() => {
  const values = new Set<string>()
  for (const row of bindings.value) {
    for (const name of groupFilterNames(row)) values.add(name)
  }
  return [...values].sort((a, b) => a.localeCompare(b))
})

function matchesFilters(item: SupplierAccount): boolean {
  const account = localAccount(item)
  if (selectedSupplierId.value && !bindingsForAccount(item).some((binding) => binding.supplier_id === selectedSupplierId.value)) return false
  if (filters.platform && normalizePlatform(account?.platform || item.local_account_platform) !== filters.platform) return false
  if (filters.type && normalizeType(account?.type || item.local_account_type) !== filters.type) return false
  if (filters.status && accountStatusFilter(item) !== filters.status && item.runtime_status !== filters.status && item.health_status !== filters.status) return false
  if (filters.group === 'ungrouped' && groupFilterNames(item).length > 0) return false
  if (filters.group && filters.group !== 'ungrouped' && !groupFilterNames(item).includes(filters.group)) return false
  const q = filters.q.toLowerCase()
  if (!q) return true
  return [
    account?.name || item.local_account_name,
    account?.platform || item.local_account_platform,
    account?.type || item.local_account_type,
    account?.status || '',
    account?.notes || '',
    account?.error_message || '',
    item.supplier_account_identifier || '',
    item.supplier_account_label || '',
    item.supplier_key_name || '',
    item.supplier_key_external_id || '',
    item.supplier_key_last4 || '',
    item.supplier_group_name || '',
    item.supplier_group_provider || '',
    item.supplier_external_group_id || '',
    item.organization_id || '',
    item.project_id || '',
    item.rate_profile || '',
    supplierLabel(item.supplier_id),
    bindingsForAccount(item).map((binding) => supplierLabel(binding.supplier_id)).join(' ')
  ].join(' ').toLowerCase().includes(q)
}

function sortBindingsDesc(items: SupplierAccount[]): SupplierAccount[] {
  return [...items].sort((a, b) => {
    const createdA = Date.parse(localAccount(a)?.created_at || a.created_at || '')
    const createdB = Date.parse(localAccount(b)?.created_at || b.created_at || '')
    if (!Number.isNaN(createdA) || !Number.isNaN(createdB)) {
      const normalizedA = Number.isNaN(createdA) ? 0 : createdA
      const normalizedB = Number.isNaN(createdB) ? 0 : createdB
      if (normalizedA !== normalizedB) return normalizedB - normalizedA
    }
    return b.id - a.id
  })
}

function accountStatusFilter(row: SupplierAccount): string {
  const account = localAccount(row)
  if (!account) return row.health_status
  if (account.temp_unschedulable_until) return 'temp_unschedulable'
  return account.status || ''
}

function accountBlockReason(row: SupplierAccount): string {
  const account = localAccount(row)
  const runtime = accountRuntime(row)
  if (runtime?.blocked_reason) return runtime.blocked_reason
  if (account?.temp_unschedulable_until) return '临时不可调度'
  if (account?.rate_limit_reset_at) return '限流中'
  if (account?.overload_until) return '过载'
  return ''
}

function emptyUsage(): UsageSummary {
  return { request_count: 0, input_tokens: 0, output_tokens: 0, total_tokens: 0, revenue_cents: 0, account_cost_cents: 0, last_request_created_at: '' }
}

function accountUsage(row: SupplierAccount): AccountUsageWindow {
  return usageByAccountID.value[row.local_sub2api_account_id] || { today: emptyUsage(), last30d: emptyUsage() }
}

function localAccount(row: SupplierAccount): LocalSub2APIAccount | undefined {
  return localAccountsByID.value[row.local_sub2api_account_id]
}

function accountRuntime(row: SupplierAccount): LocalAccountRuntime | undefined {
  return runtimeByAccountID.value[row.local_sub2api_account_id]
}

function bindingsForAccount(row: SupplierAccount): SupplierAccount[] {
  return supplierBindings.value.filter((binding) => binding.local_sub2api_account_id === row.local_sub2api_account_id)
}

function groupFilterNames(row: SupplierAccount): string[] {
  return localGroupNames(row)
}

function localGroupNames(row: SupplierAccount): string[] {
  return (localAccount(row)?.group_names || []).map((name) => name.trim()).filter(Boolean)
}

function displayGroupNames(row: SupplierAccount): string[] {
  return localGroupNames(row).slice(0, 4)
}

function lastUsedAt(row: SupplierAccount): string {
  return accountRuntime(row)?.last_used_at || accountUsage(row).last30d.last_request_created_at || accountUsage(row).today.last_request_created_at || ''
}

function capacityPercent(row: SupplierAccount): number {
  const configured = accountRuntime(row)?.configured_limit || row.configured_concurrency || localAccount(row)?.concurrency || 0
  if (configured <= 0) return 0
  return Math.min(100, Math.round(((accountRuntime(row)?.current_concurrency || row.observed_max_concurrency || 0) / configured) * 100))
}

function supplierGroupLabel(row: SupplierAccount): string {
  return row.supplier_group_name?.trim() || row.rate_profile?.trim() || ''
}

function groupPlatform(row: SupplierAccount): GroupPlatform {
  const provider = (localAccount(row)?.platform || row.local_account_platform || '').toLowerCase()
  if (provider.includes('anthropic') || provider.includes('claude')) return 'anthropic'
  if (provider.includes('gemini') || provider.includes('google')) return 'gemini'
  if (provider.includes('openai') || provider.includes('gpt')) return 'openai'
  return 'antigravity'
}

function supplierLabel(id: number): string {
  if (!id) return '未绑定供应商'
  return suppliers.value.find((supplier) => supplier.id === id)?.name || `#${id}`
}

function supplierRelationLabel(row: SupplierAccount): string {
  const related = bindingsForAccount(row)
  if (related.length === 0) return '未绑定供应商'
  return related.map((binding) => supplierLabel(binding.supplier_id)).join(' / ')
}

function supplierKeyDisplay(row: SupplierAccount): string {
  if (row.supplier_key_last4) return `sk-...${row.supplier_key_last4}`
  if (row.supplier_account_identifier) return row.supplier_account_identifier
  if (row.supplier_key_name) return row.supplier_key_name
  return '-'
}

function toggleSelect(id: number) {
  selectedIds.value = selectedSet.value.has(id) ? selectedIds.value.filter((item) => item !== id) : [...selectedIds.value, id]
}

function toggleSelectCurrentPage(checked: boolean) {
  const current = new Set(selectedIds.value)
  for (const row of pagedBindings.value) {
    if (checked) current.add(row.id)
    else current.delete(row.id)
  }
  selectedIds.value = [...current]
}

function selectCurrentPage() {
  toggleSelectCurrentPage(true)
}

function clearSelection() {
  selectedIds.value = []
}

function toggleColumn(key: string) {
  if (hiddenColumns.has(key)) hiddenColumns.delete(key)
  else hiddenColumns.add(key)
}

function selectedRows(): SupplierAccount[] {
  const ids = selectedSet.value
  return bindings.value.filter((row) => ids.has(row.id))
}

function testSelected() {
  const first = selectedRows()[0]
  if (first) openTestDialog(first)
}

function puritySelected() {
  const first = selectedRows()[0]
  if (first) openPurityDialog(first)
}

function openSelectedSupplier() {
  const first = selectedRows()[0]
  if (first) goSupplierGroups(first)
}

function goSupplierGroups(row: SupplierAccount) {
  void router.push({ path: '/admin/suppliers', query: { supplier_id: row.supplier_id, q: supplierLabel(row.supplier_id), open: 'groups' } })
}

function openTestDialog(row: SupplierAccount) {
  const account = localAccount(row)
  if (account) testingAccount.value = account
}

function openPurityDialog(row: SupplierAccount) {
  const account = localAccount(row)
  if (!account) return
  if (!supportsPurity(row)) {
    appStore.showError('仅支持 OpenAI API Key 账号执行纯度检测')
    return
  }
  purityAccount.value = account
}

function supportsPurity(row: SupplierAccount): boolean {
  const account = localAccount(row)
  const platform = (account?.platform || row.local_account_platform || '').toLowerCase()
  const type = (account?.type || row.local_account_type || '').toLowerCase()
  return platform === 'openai' && type === 'apikey'
}

async function loadSuppliers() {
  const items = await loadAllPagedItems((page, page_size) => listSuppliers({ page, page_size }))
  suppliers.value = items
  const querySupplierID = Number(route.query.supplier_id || 0)
  if (querySupplierID && suppliers.value.some((supplier) => supplier.id === querySupplierID)) {
    selectedSupplierId.value = querySupplierID
  } else if (querySupplierID) {
    selectedSupplierId.value = 0
  }
}

async function loadLocalAccounts() {
  const items = await loadAllPagedItems((page, page_size) => listLocalSub2APIAccounts({ page, page_size }))
  localAccountsByID.value = Object.fromEntries(items.map((account) => [account.id, account]))
}

async function loadBindings() {
  loadingBindings.value = true
  try {
    const bindingPages = await Promise.all(
      suppliers.value.map((supplier) => loadAllPagedItems((page, page_size) => listSupplierAccounts(supplier.id, { page, page_size })))
    )
    const allSupplierBindings = bindingPages.flat()
    supplierBindings.value = allSupplierBindings
    bindings.value = buildAccountRows(Object.values(localAccountsByID.value), allSupplierBindings)
    await loadAccountRuntime()
    await loadUsageSummaries()
    syncBindingPagination()
    selectedIds.value = selectedIds.value.filter((id) => bindings.value.some((row) => row.id === id))
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '加载账号/Key 绑定失败')
  } finally {
    loadingBindings.value = false
  }
}

function buildAccountRows(accounts: LocalSub2APIAccount[], supplierRows: SupplierAccount[]): SupplierAccount[] {
  const byAccountID = new Map<number, SupplierAccount[]>()
  for (const row of supplierRows) {
    const rows = byAccountID.get(row.local_sub2api_account_id) || []
    rows.push(row)
    byAccountID.set(row.local_sub2api_account_id, rows)
  }
  return accounts.map((account) => {
    const primary = byAccountID.get(account.id)?.[0]
    if (primary) {
      return {
        ...primary,
        id: account.id,
        local_account_name: account.name,
        local_account_platform: account.platform,
        local_account_type: account.type,
        configured_concurrency: account.concurrency,
        runtime_status: account.schedulable ? 'active' : 'disabled',
        health_status: account.status === 'active' ? 'normal' : 'paused',
        created_at: account.created_at,
        updated_at: account.updated_at
      }
    }
    return {
      id: account.id,
      supplier_id: 0,
      local_sub2api_account_id: account.id,
      local_account_name: account.name,
      local_account_platform: account.platform,
      local_account_type: account.type,
      configured_concurrency: account.concurrency,
      observed_max_concurrency: 0,
      balance_threshold_cents: 0,
      balance_cents: 0,
      balance_currency: 'USD',
      has_usable_balance: true,
      runtime_status: account.schedulable ? 'active' : 'disabled',
      health_status: account.status === 'active' ? 'normal' : 'paused',
      created_at: account.created_at,
      updated_at: account.updated_at
    }
  })
}

async function loadAccountRuntime() {
  try {
    const items = await loadAllPagedItems((page, page_size) => listLocalAccountRuntime({ page, page_size }))
    runtimeByAccountID.value = Object.fromEntries(items.map((item) => [item.account_id, item]))
  } catch {
    runtimeByAccountID.value = {}
  }
}

async function loadUsageSummaries() {
  const now = new Date()
  const todayStart = new Date(now)
  todayStart.setHours(0, 0, 0, 0)
  const last30dStart = new Date(now.getTime() - 30 * 24 * 60 * 60 * 1000)
  const [todayItems, last30dItems] = await Promise.all([
    loadAllPagedItems((page, page_size) => listLocalAccountUsageSummary({ from: todayStart.toISOString(), to: now.toISOString(), page, page_size })),
    loadAllPagedItems((page, page_size) => listLocalAccountUsageSummary({ from: last30dStart.toISOString(), to: now.toISOString(), page, page_size }))
  ])
  const next: Record<number, AccountUsageWindow> = {}
  for (const row of bindings.value) next[row.local_sub2api_account_id] = { today: emptyUsage(), last30d: emptyUsage() }
  for (const item of todayItems) {
    if (!next[item.account_id]) next[item.account_id] = { today: emptyUsage(), last30d: emptyUsage() }
    next[item.account_id].today = normalizeUsageSummary(item)
  }
  for (const item of last30dItems) {
    if (!next[item.account_id]) next[item.account_id] = { today: emptyUsage(), last30d: emptyUsage() }
    next[item.account_id].last30d = normalizeUsageSummary(item)
  }
  usageByAccountID.value = next
}

function normalizeUsageSummary(item: LocalAccountUsageSummary): UsageSummary {
  return {
    request_count: item.request_count || 0,
    input_tokens: item.input_tokens || 0,
    output_tokens: item.output_tokens || 0,
    total_tokens: item.total_tokens || item.input_tokens + item.output_tokens,
    revenue_cents: item.revenue_cents || 0,
    account_cost_cents: item.account_cost_cents || 0,
    last_request_created_at: item.last_request_created_at || ''
  }
}

function syncBindingPagination() {
  pagination.total = filteredBindings.value.length
  pagination.pages = Math.ceil(pagination.total / pagination.page_size)
  if (pagination.page > Math.max(1, pagination.pages)) pagination.page = Math.max(1, pagination.pages)
}

function reloadFirstPage() {
  pagination.page = 1
  void loadBindings()
}

function resetBindingPagination() {
  pagination.page = 1
  syncBindingPagination()
}

function handlePageChange(page: number) {
  pagination.page = page
}

function handlePageSizeChange(pageSize: number) {
  pagination.page_size = pageSize
  pagination.page = 1
  syncBindingPagination()
}

async function loadAll() {
  loading.value = true
  try {
    suppressSupplierWatch.value = true
    await loadSuppliers()
    await loadLocalAccounts()
    suppressSupplierWatch.value = false
    await loadBindings()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '加载数据失败')
  } finally {
    suppressSupplierWatch.value = false
    loading.value = false
  }
}

function resetFilters() {
  filters.q = ''
  filters.platform = ''
  filters.type = ''
  filters.status = ''
  filters.group = ''
  resetBindingPagination()
}

function updateFilters(next: typeof filters) {
  filters.q = next.q
  filters.platform = next.platform
  filters.type = next.type
  filters.status = next.status
  filters.group = next.group
  resetBindingPagination()
}

function goCreateAccount() {
  void router.push({ path: '/admin/suppliers', query: { open: 'groups' } })
}

watch(selectedSupplierId, () => {
  if (suppressSupplierWatch.value) return
  reloadFirstPage()
})
watch(() => ({ ...filters }), resetBindingPagination)
onMounted(loadAll)
</script>

<style scoped>
.row-action {
  @apply flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors;
}
</style>
