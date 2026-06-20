<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap-reverse items-start justify-between gap-3">
          <div class="grid flex-1 gap-3 lg:grid-cols-[260px_minmax(220px,1fr)_160px_160px_160px]">
            <label class="block">
              <span class="input-label">供应商</span>
              <select v-model.number="selectedSupplierID" class="input">
                <option :value="0">全部供应商</option>
                <option v-for="supplier in suppliers" :key="supplier.id" :value="supplier.id">
                  {{ supplier.name }} · {{ typeLabel(supplier.type) }}
                </option>
              </select>
            </label>
            <label class="block">
              <span class="input-label">搜索</span>
              <div class="relative">
                <Icon name="search" size="sm" class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
                <input v-model.trim="filters.q" class="input pl-9" placeholder="本地账号、供应商侧标识、标签、费率档案" />
              </div>
            </label>
            <label class="block">
              <span class="input-label">运行状态</span>
              <select v-model="filters.runtime_status" class="input">
                <option value="">全部</option>
                <option value="monitor_only">仅监控</option>
                <option value="candidate">候选</option>
                <option value="active">当前使用</option>
                <option value="disabled">停用</option>
              </select>
            </label>
            <label class="block">
              <span class="input-label">健康状态</span>
              <select v-model="filters.health_status" class="input">
                <option value="">全部</option>
                <option value="normal">正常</option>
                <option value="unavailable">不可用</option>
                <option value="credential_invalid">凭据失效</option>
                <option value="paused">暂停</option>
              </select>
            </label>
            <label class="block">
              <span class="input-label">本地账号搜索</span>
              <input v-model.trim="localAccountQuery" class="input" placeholder="账号名称、平台或类型" @input="loadLocalAccounts" />
            </label>
          </div>

          <div class="flex flex-wrap items-center gap-2">
            <button type="button" class="btn btn-secondary px-2 md:px-3" :disabled="loading" title="刷新" @click="loadAll">
              <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
              <span class="hidden md:inline">刷新</span>
            </button>
            <div class="relative">
              <button type="button" class="btn btn-secondary px-2 md:px-3" title="更多操作" @click="moreMenuOpen = !moreMenuOpen">
                <Icon name="more" size="sm" class="md:mr-1.5" />
                <span class="hidden md:inline">更多操作</span>
                <Icon name="chevronDown" size="xs" class="ml-1 hidden md:inline" />
              </button>
              <div
                v-if="moreMenuOpen"
                class="absolute right-0 z-50 mt-2 w-[min(20rem,calc(100vw-2rem))] overflow-hidden rounded-lg border border-gray-200 bg-white shadow-xl dark:border-gray-700 dark:bg-gray-800"
              >
                <div class="p-2">
                  <div class="px-2 py-2 text-xs font-semibold uppercase tracking-wide text-gray-400 dark:text-gray-500">批量操作</div>
                  <button class="menu-item" :disabled="selectedCount === 0" @click="openBulkStatusDialog">
                    <span class="menu-icon bg-blue-50 text-blue-600 dark:bg-blue-900/30 dark:text-blue-300">
                      <Icon name="edit" size="sm" />
                    </span>
                    <span>批量调整状态</span>
                  </button>
                  <button class="menu-item text-red-600 dark:text-red-300" :disabled="selectedCount === 0" @click="openBulkDeleteDialog">
                    <span class="menu-icon bg-red-50 text-red-600 dark:bg-red-900/30 dark:text-red-300">
                      <Icon name="trash" size="sm" />
                    </span>
                    <span>批量删除绑定</span>
                  </button>
                  <div class="my-2 border-t border-gray-100 dark:border-gray-700"></div>
                  <button class="menu-item" @click="resetFilters">
                    <span class="menu-icon bg-slate-100 text-slate-600 dark:bg-slate-700 dark:text-slate-200">
                      <Icon name="x" size="sm" />
                    </span>
                    <span>清除筛选</span>
                  </button>
                </div>
              </div>
            </div>
            <button type="button" class="btn btn-primary" :disabled="suppliers.length === 0" @click="openCreateDialog">
              <Icon name="plus" size="sm" />
              绑定账号/Key
            </button>
          </div>
        </div>

        <div
          v-if="localAccountError || localAccountsEmptyHint"
          class="mt-3 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800 dark:border-amber-700/40 dark:bg-amber-900/20 dark:text-amber-200"
        >
          {{ localAccountError || localAccountsEmptyHint }}
        </div>
      </template>

      <template #table>
        <div
          v-if="selectedCount > 0"
          class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-100 bg-primary-50/60 px-4 py-3 text-sm dark:border-dark-700 dark:bg-primary-900/20"
        >
          <div class="text-primary-800 dark:text-primary-200">
            已选择 <span class="font-semibold">{{ selectedCount }}</span> 个账号/Key 绑定
          </div>
          <div class="flex flex-wrap gap-2">
            <button type="button" class="btn btn-secondary btn-sm" @click="selectVisible">全选当前页</button>
            <button type="button" class="btn btn-secondary btn-sm" @click="clearSelection">清除选择</button>
            <button type="button" class="btn btn-secondary btn-sm" @click="openBulkStatusDialog">批量状态</button>
            <button type="button" class="btn btn-danger btn-sm" @click="openBulkDeleteDialog">批量删除</button>
          </div>
        </div>

        <DataTable
          :columns="columns"
          :data="pagedBindings"
          :loading="loadingBindings"
          row-key="id"
          default-sort-key="id"
          default-sort-order="desc"
          :estimate-row-height="76"
        >
          <template #header-select>
            <input
              type="checkbox"
              class="h-4 w-4 cursor-pointer rounded border-gray-300 text-primary-600 focus:ring-primary-500"
              :checked="allVisibleSelected"
              @click.stop
              @change="toggleSelectAllVisible($event)"
            />
          </template>

          <template #cell-select="{ row }">
            <input
              type="checkbox"
              class="h-4 w-4 cursor-pointer rounded border-gray-300 text-primary-600 focus:ring-primary-500"
              :checked="isSelected(row.id)"
              @change="toggleSelection(row.id)"
            />
          </template>

          <template #cell-supplier="{ row }">
            <div class="min-w-[180px]">
              <div class="font-medium text-gray-900 dark:text-white">{{ supplierLabel(row.supplier_id) }}</div>
              <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">父级 #{{ row.supplier_id }}</div>
            </div>
          </template>

          <template #cell-local_account="{ row }">
            <div class="min-w-[220px]">
              <div class="font-medium text-gray-900 dark:text-white">{{ row.local_account_name }}</div>
              <div class="mt-1 flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-dark-400">
                <span class="font-mono">#{{ row.local_sub2api_account_id }}</span>
                <span>{{ row.local_account_platform }} / {{ row.local_account_type }}</span>
              </div>
            </div>
          </template>

          <template #cell-supplier_account="{ row }">
            <div class="min-w-[220px]">
              <div class="text-sm text-gray-900 dark:text-gray-100">{{ row.supplier_account_label || '-' }}</div>
              <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ row.supplier_account_identifier || '-' }}</div>
              <div v-if="row.organization_id || row.project_id" class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                {{ row.organization_id || '-' }} / {{ row.project_id || '-' }}
              </div>
            </div>
          </template>

          <template #cell-status="{ row }">
            <div class="flex min-w-[110px] flex-col gap-1">
              <span class="badge w-fit" :class="runtimeClass(row.runtime_status)">{{ runtimeLabel(row.runtime_status) }}</span>
              <span class="badge w-fit" :class="healthClass(row.health_status)">{{ healthLabel(row.health_status) }}</span>
            </div>
          </template>

          <template #cell-balance="{ row }">
            <div class="min-w-[140px] text-right">
              <div class="font-medium text-gray-900 dark:text-gray-100">{{ formatMoney(row.balance_cents, row.balance_currency) }}</div>
              <div class="text-xs text-gray-500 dark:text-dark-400">阈值 {{ formatMoney(row.balance_threshold_cents, row.balance_currency) }}</div>
              <span v-if="row.has_usable_balance" class="badge badge-success mt-1">有额度</span>
              <span v-else class="badge badge-gray mt-1">无额度</span>
            </div>
          </template>

          <template #cell-concurrency="{ row }">
            <div class="min-w-[110px] text-right">
              <div>{{ row.observed_max_concurrency || 0 }} / {{ row.configured_concurrency || 0 }}</div>
              <div class="text-xs text-gray-500 dark:text-dark-400">观测 / 配置</div>
            </div>
          </template>

          <template #cell-rate_profile="{ row }">
            <div class="min-w-[130px]">
              <span class="badge badge-gray">{{ row.rate_profile || 'default' }}</span>
            </div>
          </template>

          <template #cell-created_at="{ row }">
            <div class="min-w-[150px] text-xs text-gray-500 dark:text-dark-400">{{ formatDateTime(row.created_at) }}</div>
          </template>

          <template #cell-actions="{ row }">
            <div class="flex min-w-[160px] justify-end gap-2">
              <button type="button" class="btn btn-secondary btn-sm" title="编辑" @click="openEditDialog(row)">
                <Icon name="edit" size="sm" />
                编辑
              </button>
              <button type="button" class="btn btn-danger btn-sm" title="删除" @click="openDeleteDialog(row)">
                <Icon name="trash" size="sm" />
              </button>
            </div>
          </template>

          <template #empty>
            <EmptyState
              title="暂无账号/Key 绑定"
              description="选择供应商后绑定本地 Sub2API 已存在的账号/Key。"
              action-text="绑定账号/Key"
              @action="openCreateDialog"
            />
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

    <BaseDialog :show="editorOpen" :title="editingBinding ? '编辑账号/Key 绑定' : '绑定账号/Key'" width="wide" @close="closeEditor">
      <form id="supplier-account-editor-form" class="space-y-5" @submit.prevent="submitBinding">
        <div class="grid gap-4 sm:grid-cols-2">
          <label class="block">
            <span class="input-label">供应商父级</span>
            <select v-model.number="form.supplier_id" class="input" required :disabled="Boolean(editingBinding)">
              <option :value="0">请选择供应商</option>
              <option v-for="supplier in suppliers" :key="supplier.id" :value="supplier.id">
                {{ supplier.name }} · {{ typeLabel(supplier.type) }}
              </option>
            </select>
          </label>
          <label class="block">
            <span class="input-label">本地 Sub2API 账号</span>
            <select v-model.number="form.local_sub2api_account_id" class="input" required :disabled="Boolean(editingBinding)">
              <option :value="0">请选择账号</option>
              <option v-for="account in localAccounts" :key="account.id" :value="account.id">
                #{{ account.id }} {{ account.name }} · {{ account.platform }} / {{ account.type }} · {{ account.status }}
              </option>
            </select>
            <p v-if="localAccountError || localAccountsEmptyHint" class="input-error-text mt-1">
              {{ localAccountError || localAccountsEmptyHint }}
            </p>
          </label>
        </div>

        <div class="grid gap-4 sm:grid-cols-3">
          <label class="block">
            <span class="input-label">供应商侧标识</span>
            <input v-model.trim="form.supplier_account_identifier" class="input" placeholder="供应商后台账号、邮箱或 Key ID" />
          </label>
          <label class="block">
            <span class="input-label">账号/Key 标签</span>
            <input v-model.trim="form.supplier_account_label" class="input" placeholder="主账号、备用、活动包" />
          </label>
          <label class="block">
            <span class="input-label">费率档案</span>
            <input v-model.trim="form.rate_profile" class="input" placeholder="default" />
          </label>
        </div>

        <div class="grid gap-4 sm:grid-cols-2">
          <label class="block">
            <span class="input-label">Organization ID</span>
            <input v-model.trim="form.organization_id" class="input" />
          </label>
          <label class="block">
            <span class="input-label">Project ID</span>
            <input v-model.trim="form.project_id" class="input" />
          </label>
        </div>

        <div class="grid gap-4 sm:grid-cols-3">
          <label class="block">
            <span class="input-label">余额</span>
            <input v-model.number="form.balance_yuan" type="number" min="0" step="0.01" class="input" />
          </label>
          <label class="block">
            <span class="input-label">余额阈值</span>
            <input v-model.number="form.balance_threshold_yuan" type="number" min="0" step="0.01" class="input" />
          </label>
          <label class="block">
            <span class="input-label">币种</span>
            <input v-model.trim="form.balance_currency" class="input" placeholder="CNY" />
          </label>
        </div>

        <div class="grid gap-4 sm:grid-cols-4">
          <label class="block">
            <span class="input-label">配置并发</span>
            <input v-model.number="form.configured_concurrency" type="number" min="0" step="1" class="input" />
          </label>
          <label class="block">
            <span class="input-label">观测最大并发</span>
            <input v-model.number="form.observed_max_concurrency" type="number" min="0" step="1" class="input" />
          </label>
          <label class="block">
            <span class="input-label">运行状态</span>
            <select v-model="form.runtime_status" class="input">
              <option value="monitor_only">仅监控</option>
              <option value="candidate">候选</option>
              <option value="active">当前使用</option>
              <option value="disabled">停用</option>
            </select>
          </label>
          <label class="block">
            <span class="input-label">健康状态</span>
            <select v-model="form.health_status" class="input">
              <option value="normal">正常</option>
              <option value="unavailable">不可用</option>
              <option value="credential_invalid">凭据失效</option>
              <option value="paused">暂停</option>
            </select>
          </label>
        </div>
      </form>

      <template #footer>
        <button type="button" class="btn btn-secondary" @click="closeEditor">取消</button>
        <button type="submit" form="supplier-account-editor-form" class="btn btn-primary" :disabled="submitting">
          {{ submitting ? '保存中...' : editingBinding ? '保存修改' : '绑定账号/Key' }}
        </button>
      </template>
    </BaseDialog>

    <BaseDialog :show="statusDialogOpen" title="批量调整账号/Key 状态" width="normal" @close="statusDialogOpen = false">
      <form id="supplier-account-status-form" class="space-y-4" @submit.prevent="submitBulkStatus">
        <p class="text-sm text-gray-500 dark:text-dark-400">将调整 {{ selectedCount }} 个账号/Key 绑定</p>
        <label class="block">
          <span class="input-label">运行状态</span>
          <select v-model="statusForm.runtime_status" class="input">
            <option value="monitor_only">仅监控</option>
            <option value="candidate">候选</option>
            <option value="active">当前使用</option>
            <option value="disabled">停用</option>
          </select>
        </label>
        <label class="block">
          <span class="input-label">健康状态</span>
          <select v-model="statusForm.health_status" class="input">
            <option value="normal">正常</option>
            <option value="unavailable">不可用</option>
            <option value="credential_invalid">凭据失效</option>
            <option value="paused">暂停</option>
          </select>
        </label>
      </form>

      <template #footer>
        <button type="button" class="btn btn-secondary" @click="statusDialogOpen = false">取消</button>
        <button type="submit" form="supplier-account-status-form" class="btn btn-primary" :disabled="statusSubmitting">保存状态</button>
      </template>
    </BaseDialog>

    <ConfirmDialog
      :show="deleteDialogOpen"
      title="删除账号/Key 绑定"
      :message="deleteConfirmMessage"
      confirm-text="删除"
      :danger="true"
      @confirm="confirmDelete"
      @cancel="deleteDialogOpen = false"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'
import type { Column } from '@/components/common/types'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { useTableSelection } from '@/composables/useTableSelection'
import { useAppStore } from '@/stores/app'
import {
  createSupplierAccount,
  deleteSupplierAccount,
  listLocalSub2APIAccounts,
  listSupplierAccounts,
  listSuppliers,
  updateSupplierAccount,
  type LocalSub2APIAccount,
  type Supplier,
  type SupplierAccount,
  type SupplierHealthStatus,
  type SupplierRuntimeStatus,
  type SupplierType
} from '@/api/admin/adminPlus'

const appStore = useAppStore()
const route = useRoute()

const loading = ref(false)
const loadingBindings = ref(false)
const submitting = ref(false)
const statusSubmitting = ref(false)
const editorOpen = ref(false)
const statusDialogOpen = ref(false)
const deleteDialogOpen = ref(false)
const moreMenuOpen = ref(false)
const bulkDeleteMode = ref(false)
const suppliers = ref<Supplier[]>([])
const bindings = ref<SupplierAccount[]>([])
const localAccounts = ref<LocalSub2APIAccount[]>([])
const localAccountError = ref('')
const selectedSupplierID = ref(0)
const localAccountQuery = ref('')
const editingBinding = ref<SupplierAccount | null>(null)
const deletingBinding = ref<SupplierAccount | null>(null)

const filters = reactive({
  q: '',
  runtime_status: '' as '' | SupplierRuntimeStatus,
  health_status: '' as '' | SupplierHealthStatus
})

const pagination = reactive({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0,
  pages: 0
})

const form = reactive({
  supplier_id: 0,
  local_sub2api_account_id: 0,
  supplier_account_identifier: '',
  supplier_account_label: '',
  organization_id: '',
  project_id: '',
  rate_profile: '',
  balance_yuan: 0,
  balance_threshold_yuan: 0,
  balance_currency: 'CNY',
  configured_concurrency: 0,
  observed_max_concurrency: 0,
  runtime_status: 'monitor_only' as SupplierRuntimeStatus,
  health_status: 'normal' as SupplierHealthStatus
})

const statusForm = reactive({
  runtime_status: 'monitor_only' as SupplierRuntimeStatus,
  health_status: 'normal' as SupplierHealthStatus
})

const columns: Column[] = [
  { key: 'select', label: '', class: 'w-10' },
  { key: 'supplier', label: '供应商父级' },
  { key: 'local_account', label: '本地 Sub2API 账号', sortable: true },
  { key: 'supplier_account', label: '供应商侧账号/Key' },
  { key: 'status', label: '状态' },
  { key: 'balance', label: '余额', class: 'text-right' },
  { key: 'concurrency', label: '并发', class: 'text-right' },
  { key: 'rate_profile', label: '费率档案' },
  { key: 'created_at', label: '创建时间', sortable: true },
  { key: 'actions', label: '操作', class: 'text-right' }
]

const filteredBindings = computed(() => {
  const q = filters.q.toLowerCase()
  return bindings.value.filter((item) => {
    if (filters.runtime_status && item.runtime_status !== filters.runtime_status) return false
    if (filters.health_status && item.health_status !== filters.health_status) return false
    if (q) {
      const haystack = [
        item.local_account_name,
        item.local_account_platform,
        item.local_account_type,
        item.supplier_account_identifier || '',
        item.supplier_account_label || '',
        item.organization_id || '',
        item.project_id || '',
        item.rate_profile || ''
      ].join(' ').toLowerCase()
      if (!haystack.includes(q)) return false
    }
    return true
  })
})

const pagedBindings = computed(() => {
  const start = (pagination.page - 1) * pagination.page_size
  return filteredBindings.value.slice(start, start + pagination.page_size)
})

const {
  selectedIds,
  selectedCount,
  allVisibleSelected,
  isSelected,
  toggle: toggleSelection,
  clear: clearSelection,
  selectVisible,
  toggleVisible
} = useTableSelection<SupplierAccount>({
  rows: pagedBindings,
  getId: (row) => row.id
})

const selectedRows = computed(() => bindings.value.filter((item) => selectedIds.value.includes(item.id)))

const localAccountsEmptyHint = computed(() => {
  if (localAccountError.value || loading.value || localAccounts.value.length > 0) return ''
  return '未读取到 Sub2API 账号。线上部署请检查后端 SUB2API_READONLY_DATABASE_URL 是否指向 Sub2API 只读数据库；否则后端会回退读取 Admin Plus 自己的库。'
})

const deleteConfirmMessage = computed(() => {
  if (bulkDeleteMode.value) {
    return `确认删除已选择的 ${selectedCount.value} 个账号/Key 绑定？`
  }
  return deletingBinding.value
    ? `确认删除「${deletingBinding.value.local_account_name}」与供应商「${supplierLabel(deletingBinding.value.supplier_id)}」的绑定？`
    : '确认删除该账号/Key 绑定？'
})

function toggleSelectAllVisible(event: Event) {
  toggleVisible((event.target as HTMLInputElement).checked)
}

function centsFromYuan(value: number): number {
  return Math.round(Number(value || 0) * 100)
}

function yuanFromCents(value: number): number {
  return Number(((value || 0) / 100).toFixed(2))
}

function formatMoney(cents: number, currency: string): string {
  return new Intl.NumberFormat(undefined, {
    style: 'currency',
    currency: currency || 'CNY',
    minimumFractionDigits: 2
  }).format((cents || 0) / 100)
}

function formatDateTime(value?: string | null): string {
  if (!value) return '-'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '-' : date.toLocaleString()
}

function typeLabel(value: SupplierType): string {
  return {
    openai: 'OpenAI',
    anthropic: 'Anthropic',
    gemini: 'Gemini',
    sub2api: 'Sub2API',
    new_api: 'New API',
    browser_only: '仅浏览器',
    custom: '自定义'
  }[value]
}

function runtimeLabel(value: SupplierRuntimeStatus): string {
  return {
    monitor_only: '仅监控',
    candidate: '候选',
    active: '使用中',
    disabled: '停用'
  }[value]
}

function healthLabel(value: SupplierHealthStatus): string {
  return {
    normal: '正常',
    unavailable: '不可用',
    credential_invalid: '凭据失效',
    paused: '暂停'
  }[value]
}

function runtimeClass(status: SupplierRuntimeStatus): string {
  if (status === 'active') return 'badge-success'
  if (status === 'candidate') return 'badge-primary'
  if (status === 'disabled') return 'badge-danger'
  return 'badge-gray'
}

function healthClass(status: SupplierHealthStatus): string {
  if (status === 'normal') return 'badge-success'
  if (status === 'paused') return 'badge-warning'
  return 'badge-danger'
}

function supplierLabel(id: number): string {
  return suppliers.value.find((supplier) => supplier.id === id)?.name || `#${id}`
}

async function loadSuppliers() {
  const result = await listSuppliers()
  suppliers.value = result.items
  const querySupplierID = Number(route.query.supplier_id || 0)
  if (querySupplierID && suppliers.value.some((supplier) => supplier.id === querySupplierID)) {
    selectedSupplierID.value = querySupplierID
  } else if (!selectedSupplierID.value && suppliers.value.length > 0) {
    selectedSupplierID.value = suppliers.value[0].id
  }
}

async function loadLocalAccounts() {
  localAccountError.value = ''
  try {
    const result = await listLocalSub2APIAccounts({ q: localAccountQuery.value || undefined, limit: 200 })
    localAccounts.value = result.items
  } catch (error) {
    localAccounts.value = []
    localAccountError.value = `${(error as { message?: string }).message || '加载本地 Sub2API 账号失败'}。请确认 SUB2API_READONLY_DATABASE_URL 指向 Sub2API 只读数据库，且 accounts 表可读。`
    appStore.showError(localAccountError.value)
  }
}

async function loadBindings() {
  loadingBindings.value = true
  clearSelection()
  try {
    if (selectedSupplierID.value) {
      const result = await listSupplierAccounts(selectedSupplierID.value, { page: 1, page_size: 1000 })
      bindings.value = result.items
      syncBindingPagination()
      return
    }
    const all: SupplierAccount[] = []
    for (const supplier of suppliers.value) {
      const result = await listSupplierAccounts(supplier.id, { page: 1, page_size: 1000 })
      all.push(...result.items)
    }
    bindings.value = all
    syncBindingPagination()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '加载账号/Key 绑定失败')
  } finally {
    loadingBindings.value = false
  }
}

function syncBindingPagination() {
  pagination.total = filteredBindings.value.length
  pagination.pages = Math.ceil(pagination.total / pagination.page_size)
  if (pagination.page > Math.max(1, pagination.pages)) {
    pagination.page = Math.max(1, pagination.pages)
  }
}

function reloadFirstPage() {
  pagination.page = 1
  void loadBindings()
}

function handlePageChange(page: number) {
  pagination.page = page
  void loadBindings()
}

function handlePageSizeChange(pageSize: number) {
  pagination.page_size = pageSize
  pagination.page = 1
  void loadBindings()
}

async function loadAll() {
  loading.value = true
  try {
    await Promise.all([loadSuppliers(), loadLocalAccounts()])
    await loadBindings()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '加载数据失败')
  } finally {
    loading.value = false
  }
}

function resetFilters() {
  filters.q = ''
  filters.runtime_status = ''
  filters.health_status = ''
  localAccountQuery.value = ''
  moreMenuOpen.value = false
  void loadLocalAccounts()
}

function resetForm() {
  form.supplier_id = selectedSupplierID.value || suppliers.value[0]?.id || 0
  form.local_sub2api_account_id = 0
  form.supplier_account_identifier = ''
  form.supplier_account_label = ''
  form.organization_id = ''
  form.project_id = ''
  form.rate_profile = ''
  form.balance_yuan = 0
  form.balance_threshold_yuan = 0
  form.balance_currency = 'CNY'
  form.configured_concurrency = 0
  form.observed_max_concurrency = 0
  form.runtime_status = 'monitor_only'
  form.health_status = 'normal'
}

function fillForm(row: SupplierAccount) {
  form.supplier_id = row.supplier_id
  form.local_sub2api_account_id = row.local_sub2api_account_id
  form.supplier_account_identifier = row.supplier_account_identifier || ''
  form.supplier_account_label = row.supplier_account_label || ''
  form.organization_id = row.organization_id || ''
  form.project_id = row.project_id || ''
  form.rate_profile = row.rate_profile || ''
  form.balance_yuan = yuanFromCents(row.balance_cents)
  form.balance_threshold_yuan = yuanFromCents(row.balance_threshold_cents)
  form.balance_currency = row.balance_currency || 'CNY'
  form.configured_concurrency = row.configured_concurrency || 0
  form.observed_max_concurrency = row.observed_max_concurrency || 0
  form.runtime_status = row.runtime_status
  form.health_status = row.health_status
}

function openCreateDialog() {
  editingBinding.value = null
  resetForm()
  editorOpen.value = true
}

function openEditDialog(row: SupplierAccount) {
  editingBinding.value = row
  fillForm(row)
  editorOpen.value = true
}

function closeEditor() {
  editorOpen.value = false
}

function buildPayload() {
  return {
    supplier_account_identifier: form.supplier_account_identifier || undefined,
    supplier_account_label: form.supplier_account_label || undefined,
    organization_id: form.organization_id || undefined,
    project_id: form.project_id || undefined,
    rate_profile: form.rate_profile || undefined,
    configured_concurrency: form.configured_concurrency,
    observed_max_concurrency: form.observed_max_concurrency,
    balance_threshold_cents: centsFromYuan(form.balance_threshold_yuan),
    balance_cents: centsFromYuan(form.balance_yuan),
    balance_currency: form.balance_currency || 'CNY',
    runtime_status: form.runtime_status,
    health_status: form.health_status
  }
}

async function submitBinding() {
  if (!form.supplier_id || (!editingBinding.value && !form.local_sub2api_account_id)) {
    appStore.showError('请选择供应商和本地账号')
    return
  }
  submitting.value = true
  try {
    if (editingBinding.value) {
      await updateSupplierAccount(form.supplier_id, editingBinding.value.id, buildPayload())
      appStore.showSuccess('账号/Key 绑定已更新')
    } else {
      await createSupplierAccount(form.supplier_id, {
        local_sub2api_account_id: form.local_sub2api_account_id,
        ...buildPayload()
      })
      selectedSupplierID.value = form.supplier_id
      appStore.showSuccess('账号/Key 已绑定')
    }
    editorOpen.value = false
    await loadBindings()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '保存账号/Key 绑定失败')
  } finally {
    submitting.value = false
  }
}

function openBulkStatusDialog() {
  if (selectedCount.value === 0) return
  moreMenuOpen.value = false
  const first = selectedRows.value[0]
  statusForm.runtime_status = first?.runtime_status || 'monitor_only'
  statusForm.health_status = first?.health_status || 'normal'
  statusDialogOpen.value = true
}

async function submitBulkStatus() {
  statusSubmitting.value = true
  try {
    await runSequential(selectedRows.value, async (row) => {
      await updateSupplierAccount(row.supplier_id, row.id, {
        supplier_account_identifier: row.supplier_account_identifier,
        supplier_account_label: row.supplier_account_label,
        organization_id: row.organization_id,
        project_id: row.project_id,
        rate_profile: row.rate_profile,
        configured_concurrency: row.configured_concurrency,
        observed_max_concurrency: row.observed_max_concurrency,
        balance_threshold_cents: row.balance_threshold_cents,
        balance_cents: row.balance_cents,
        balance_currency: row.balance_currency,
        runtime_status: statusForm.runtime_status,
        health_status: statusForm.health_status
      })
    })
    appStore.showSuccess('批量状态已更新')
    clearSelection()
    statusDialogOpen.value = false
    await loadBindings()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '批量更新状态失败')
  } finally {
    statusSubmitting.value = false
  }
}

function openDeleteDialog(row: SupplierAccount) {
  bulkDeleteMode.value = false
  deletingBinding.value = row
  deleteDialogOpen.value = true
}

function openBulkDeleteDialog() {
  if (selectedCount.value === 0) return
  moreMenuOpen.value = false
  bulkDeleteMode.value = true
  deletingBinding.value = null
  deleteDialogOpen.value = true
}

async function confirmDelete() {
  try {
    if (bulkDeleteMode.value) {
      await runSequential(selectedRows.value, async (row) => {
        await deleteSupplierAccount(row.supplier_id, row.id)
      })
      appStore.showSuccess('已删除选中的账号/Key 绑定')
      clearSelection()
    } else if (deletingBinding.value) {
      await deleteSupplierAccount(deletingBinding.value.supplier_id, deletingBinding.value.id)
      appStore.showSuccess('账号/Key 绑定已删除')
    }
    deleteDialogOpen.value = false
    await loadBindings()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '删除账号/Key 绑定失败')
  }
}

async function runSequential<T>(items: T[], runner: (item: T) => Promise<void>) {
  for (const item of items) {
    await runner(item)
  }
}

watch(selectedSupplierID, () => {
  reloadFirstPage()
})

watch(
  () => ({ ...filters }),
  () => {
    reloadFirstPage()
  }
)

onMounted(loadAll)
</script>

<style scoped>
.menu-item {
  @apply flex w-full items-center gap-3 rounded-md px-3 py-2 text-sm text-gray-700 transition-colors hover:bg-gray-100 disabled:cursor-not-allowed disabled:opacity-50 dark:text-gray-200 dark:hover:bg-gray-700;
}

.menu-icon {
  @apply flex h-8 w-8 items-center justify-center rounded-md;
}
</style>
