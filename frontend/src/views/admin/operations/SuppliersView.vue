<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">供应商管理</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
            维护供应商父级、后台登录采集凭据、运行状态和余额状态。
          </p>
        </div>
        <div class="flex flex-wrap gap-2">
          <button type="button" class="btn btn-secondary" :disabled="loading" @click="loadSuppliers">
            <Icon name="refresh" size="sm" />
            刷新
          </button>
          <button type="button" class="btn btn-primary" @click="openCreateDialog">
            <Icon name="plus" size="sm" />
            添加供应商
          </button>
        </div>
      </section>

      <section class="grid gap-4 md:grid-cols-4">
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">供应商总数</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ filteredSuppliers.length }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">可切换</p>
          <p class="mt-2 text-2xl font-semibold text-emerald-600 dark:text-emerald-400">{{ switchableCount }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">仅监控</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ monitorOnlyCount }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">异常健康</p>
          <p class="mt-2 text-2xl font-semibold text-rose-600 dark:text-rose-400">{{ unhealthyCount }}</p>
        </div>
      </section>

      <section class="card">
        <div class="border-b border-gray-100 p-5 dark:border-dark-700">
          <div class="grid gap-3 lg:grid-cols-[minmax(220px,1fr)_180px_180px_180px]">
            <label class="block">
              <span class="input-label">搜索</span>
              <input v-model.trim="filters.q" class="input" placeholder="供应商名称、联系人或备注" />
            </label>
            <label class="block">
              <span class="input-label">系统类型</span>
              <select v-model="filters.type" class="input">
                <option value="">全部</option>
                <option value="sub2api">Sub2API</option>
                <option value="new_api">New API</option>
                <option value="openai">OpenAI</option>
                <option value="anthropic">Anthropic</option>
                <option value="gemini">Gemini</option>
                <option value="browser_only">仅浏览器</option>
                <option value="custom">自定义</option>
              </select>
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
          </div>
        </div>

        <DataTable
          :columns="columns"
          :data="filteredSuppliers"
          :loading="loading"
          row-key="id"
          default-sort-key="id"
          default-sort-order="desc"
        >
          <template #cell-name="{ row }">
            <div class="font-medium text-gray-900 dark:text-white">{{ row.name }}</div>
            <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">#{{ row.id }} · {{ formatDateTime(row.created_at) }}</div>
          </template>

          <template #cell-type="{ row }">
            <div class="flex flex-wrap gap-1">
              <span class="badge badge-gray">{{ kindLabel(row.kind) }}</span>
              <span class="badge badge-primary">{{ typeLabel(row.type) }}</span>
            </div>
          </template>

          <template #cell-status="{ row }">
            <div class="flex flex-col gap-1">
              <span class="badge w-fit" :class="runtimeClass(row.runtime_status)">{{ runtimeLabel(row.runtime_status) }}</span>
              <span class="badge w-fit" :class="healthClass(row.health_status)">{{ healthLabel(row.health_status) }}</span>
            </div>
          </template>

          <template #cell-balance="{ row }">
            <div class="text-right">
              {{ formatMoney(row.balance_cents, row.balance_currency) }}
              <div class="text-xs text-gray-500 dark:text-dark-400">{{ formatDateTime(row.balance_updated_at) }}</div>
            </div>
          </template>

          <template #cell-credential="{ row }">
            <div class="flex flex-wrap gap-1">
              <span v-if="row.credential.browser_login_enabled" class="badge badge-warning">Chrome</span>
              <span v-if="row.credential.browser_login_username_configured" class="badge badge-gray">{{ row.credential.masked_browser_login_username || '账号' }}</span>
              <span v-if="row.credential.browser_login_password_configured" class="badge badge-success">密码</span>
              <span v-if="row.credential.browser_login_token_configured" class="badge badge-primary">Token</span>
              <span v-if="row.credential.postgres_configured" class="badge badge-purple">PG</span>
              <span v-if="row.credential.redis_configured" class="badge badge-primary">Redis</span>
              <span v-if="!hasCredential(row)" class="badge badge-gray">未配置</span>
            </div>
          </template>

          <template #cell-address="{ row }">
            <div class="max-w-[260px] text-xs text-gray-500 dark:text-dark-400">
              <div class="truncate">{{ row.dashboard_url || '-' }}</div>
              <div class="truncate">{{ row.api_base_url || '-' }}</div>
            </div>
          </template>

          <template #cell-actions="{ row }">
            <div class="flex justify-end gap-2">
              <button type="button" class="btn btn-secondary px-3 py-1.5 text-xs" @click="openStatusDialog(row)">
                状态
              </button>
            </div>
          </template>

          <template #empty>
            <EmptyState
              title="暂无供应商"
              description="添加供应商父级后，再到账号/Key 绑定模块挂载本地 Sub2API 账号。"
              action-text="添加供应商"
              @action="openCreateDialog"
            />
          </template>
        </DataTable>
      </section>

      <BaseDialog :show="createDialogOpen" title="添加供应商" width="wide" @close="createDialogOpen = false">
        <form id="supplier-create-form" class="space-y-5" @submit.prevent="submitSupplier">
          <div class="grid gap-4 sm:grid-cols-2">
            <label class="block">
              <span class="input-label">名称</span>
              <input v-model.trim="form.name" class="input" required placeholder="supplier-a" />
            </label>
            <label class="block">
              <span class="input-label">供应商归类</span>
              <select v-model="form.kind" class="input">
                <option value="relay">下游中转</option>
                <option value="source_account">源站账号归类</option>
                <option value="browser_only">仅浏览器采集</option>
                <option value="custom">自定义</option>
              </select>
            </label>
          </div>

          <div class="grid gap-4 sm:grid-cols-2">
            <label class="block">
              <span class="input-label">系统类型</span>
              <select v-model="form.type" class="input">
                <option value="sub2api">Sub2API</option>
                <option value="new_api">New API</option>
                <option value="openai">OpenAI</option>
                <option value="anthropic">Anthropic</option>
                <option value="gemini">Gemini</option>
                <option value="browser_only">仅浏览器</option>
                <option value="custom">自定义</option>
              </select>
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
          </div>

          <div class="grid gap-4 sm:grid-cols-2">
            <label class="block">
              <span class="input-label">后台地址</span>
              <input v-model.trim="form.dashboard_url" class="input" placeholder="https://supplier.example.com" />
            </label>
            <label class="block">
              <span class="input-label">API Base URL</span>
              <input v-model.trim="form.api_base_url" class="input" placeholder="https://supplier.example.com/api/v1" />
            </label>
          </div>

          <div class="grid gap-4 sm:grid-cols-3">
            <label class="block">
              <span class="input-label">Chrome 登录账号</span>
              <input v-model.trim="form.browser_login_username" class="input" autocomplete="username" />
            </label>
            <label class="block">
              <span class="input-label">Chrome 登录密码</span>
              <input v-model.trim="form.browser_login_password" type="password" class="input" autocomplete="new-password" />
            </label>
            <label class="block">
              <span class="input-label">临时 Token</span>
              <input v-model.trim="form.browser_login_token" type="password" class="input" autocomplete="off" />
            </label>
          </div>

          <div class="grid gap-4 sm:grid-cols-3">
            <label class="block">
              <span class="input-label">初始余额</span>
              <input v-model.number="form.balance_yuan" type="number" min="0" step="0.01" class="input" />
            </label>
            <label class="block">
              <span class="input-label">币种</span>
              <input v-model.trim="form.balance_currency" class="input" placeholder="CNY" />
            </label>
            <label class="flex items-end gap-2 pb-3 text-sm text-gray-700 dark:text-gray-300">
              <input v-model="form.browser_login_enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
              允许 Chrome 插件登录采集
            </label>
          </div>

          <label class="block">
            <span class="input-label">备注</span>
            <textarea v-model.trim="form.notes" class="input min-h-[90px]" />
          </label>
        </form>

        <template #footer>
          <button type="button" class="btn btn-secondary" @click="createDialogOpen = false">取消</button>
          <button type="submit" form="supplier-create-form" class="btn btn-primary" :disabled="submitting">
            {{ submitting ? '创建中...' : '创建供应商' }}
          </button>
        </template>
      </BaseDialog>

      <BaseDialog :show="statusDialogOpen" title="调整供应商状态" width="normal" @close="statusDialogOpen = false">
        <form id="supplier-status-form" class="space-y-4" @submit.prevent="submitStatus">
          <p class="text-sm text-gray-500 dark:text-dark-400">{{ statusForm.name }}</p>
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
          <button type="submit" form="supplier-status-form" class="btn btn-primary" :disabled="statusSubmitting">
            保存状态
          </button>
        </template>
      </BaseDialog>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import type { Column } from '@/components/common/types'
import {
  createSupplier,
  listSuppliers,
  updateSupplierStatus,
  type Supplier,
  type SupplierHealthStatus,
  type SupplierKind,
  type SupplierRuntimeStatus,
  type SupplierType
} from '@/api/admin/adminPlus'

const appStore = useAppStore()

const loading = ref(false)
const submitting = ref(false)
const statusSubmitting = ref(false)
const createDialogOpen = ref(false)
const statusDialogOpen = ref(false)
const suppliers = ref<Supplier[]>([])

const filters = reactive({
  q: '',
  type: '' as '' | SupplierType,
  runtime_status: '' as '' | SupplierRuntimeStatus,
  health_status: '' as '' | SupplierHealthStatus
})

const form = reactive({
  name: '',
  kind: 'relay' as SupplierKind,
  type: 'sub2api' as SupplierType,
  runtime_status: 'monitor_only' as SupplierRuntimeStatus,
  health_status: 'normal' as SupplierHealthStatus,
  dashboard_url: '',
  api_base_url: '',
  browser_login_username: '',
  browser_login_password: '',
  browser_login_token: '',
  balance_yuan: 0,
  balance_currency: 'CNY',
  browser_login_enabled: true,
  notes: ''
})

const statusForm = reactive({
  id: 0,
  name: '',
  runtime_status: 'monitor_only' as SupplierRuntimeStatus,
  health_status: 'normal' as SupplierHealthStatus
})

const columns: Column[] = [
  { key: 'name', label: '名称', sortable: true },
  { key: 'type', label: '类型' },
  { key: 'status', label: '状态' },
  { key: 'balance', label: '余额', class: 'text-right' },
  { key: 'credential', label: '凭据' },
  { key: 'address', label: '地址' },
  { key: 'actions', label: '操作', class: 'text-right' }
]

const filteredSuppliers = computed(() => {
  const q = filters.q.toLowerCase()
  return suppliers.value.filter((item) => {
    if (filters.type && item.type !== filters.type) return false
    if (filters.runtime_status && item.runtime_status !== filters.runtime_status) return false
    if (filters.health_status && item.health_status !== filters.health_status) return false
    if (q) {
      const haystack = `${item.name} ${item.contact || ''} ${item.notes || ''}`.toLowerCase()
      if (!haystack.includes(q)) return false
    }
    return true
  })
})

const switchableCount = computed(() => filteredSuppliers.value.filter((item) =>
  ['candidate', 'active'].includes(item.runtime_status) && item.health_status === 'normal' && item.balance_cents > 0
).length)
const monitorOnlyCount = computed(() => filteredSuppliers.value.filter((item) => item.runtime_status === 'monitor_only').length)
const unhealthyCount = computed(() => filteredSuppliers.value.filter((item) => item.health_status !== 'normal').length)

function centsFromYuan(value: number): number {
  return Math.round(Number(value || 0) * 100)
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

function kindLabel(value: SupplierKind): string {
  return {
    source_account: '源站',
    relay: '中转',
    browser_only: '浏览器',
    custom: '自定义'
  }[value]
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

function hasCredential(supplier: Supplier): boolean {
  return supplier.credential.postgres_configured ||
    supplier.credential.redis_configured ||
    supplier.credential.browser_login_enabled ||
    supplier.credential.browser_login_username_configured ||
    supplier.credential.browser_login_password_configured ||
    supplier.credential.browser_login_token_configured
}

async function loadSuppliers() {
  loading.value = true
  try {
    const result = await listSuppliers()
    suppliers.value = result.items
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '加载供应商失败')
  } finally {
    loading.value = false
  }
}

function resetForm() {
  form.name = ''
  form.kind = 'relay'
  form.type = 'sub2api'
  form.runtime_status = 'monitor_only'
  form.health_status = 'normal'
  form.dashboard_url = ''
  form.api_base_url = ''
  form.browser_login_username = ''
  form.browser_login_password = ''
  form.browser_login_token = ''
  form.balance_yuan = 0
  form.balance_currency = 'CNY'
  form.browser_login_enabled = true
  form.notes = ''
}

function openCreateDialog() {
  resetForm()
  createDialogOpen.value = true
}

async function submitSupplier() {
  submitting.value = true
  try {
    await createSupplier({
      name: form.name,
      kind: form.kind,
      type: form.type,
      runtime_status: form.runtime_status,
      health_status: form.health_status,
      dashboard_url: form.dashboard_url || undefined,
      api_base_url: form.api_base_url || undefined,
      browser_login_username: form.browser_login_username || undefined,
      browser_login_password: form.browser_login_password || undefined,
      browser_login_token: form.browser_login_token || undefined,
      balance_cents: centsFromYuan(form.balance_yuan),
      balance_currency: form.balance_currency || 'CNY',
      browser_login_enabled: form.browser_login_enabled,
      notes: form.notes || undefined
    })
    createDialogOpen.value = false
    appStore.showSuccess('供应商已创建')
    await loadSuppliers()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '创建供应商失败')
  } finally {
    submitting.value = false
  }
}

function openStatusDialog(supplier: Supplier) {
  statusForm.id = supplier.id
  statusForm.name = supplier.name
  statusForm.runtime_status = supplier.runtime_status
  statusForm.health_status = supplier.health_status
  statusDialogOpen.value = true
}

async function submitStatus() {
  if (!statusForm.id) return
  statusSubmitting.value = true
  try {
    await updateSupplierStatus(statusForm.id, {
      runtime_status: statusForm.runtime_status,
      health_status: statusForm.health_status
    })
    statusDialogOpen.value = false
    appStore.showSuccess('状态已更新')
    await loadSuppliers()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '更新状态失败')
  } finally {
    statusSubmitting.value = false
  }
}

onMounted(loadSuppliers)
</script>
