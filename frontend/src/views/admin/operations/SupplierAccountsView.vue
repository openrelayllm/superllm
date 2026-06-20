<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">账号/Key 绑定</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
            在供应商父级下挂载本地 Sub2API 已存在的账号/Key，作为成本、余额、健康和切换建议的决策对象。
          </p>
        </div>
        <div class="flex flex-wrap gap-2">
          <button type="button" class="btn btn-secondary" :disabled="loading" @click="loadAll">
            <Icon name="refresh" size="sm" />
            刷新
          </button>
          <button type="button" class="btn btn-primary" :disabled="suppliers.length === 0" @click="openBindDialog">
            <Icon name="plus" size="sm" />
            绑定账号/Key
          </button>
        </div>
      </section>

      <section class="card">
        <div class="border-b border-gray-100 p-5 dark:border-dark-700">
          <div class="grid gap-3 lg:grid-cols-[minmax(220px,1fr)_240px]">
            <label class="block">
              <span class="input-label">供应商</span>
              <select v-model.number="selectedSupplierID" class="input" @change="loadBindings">
                <option :value="0">请选择供应商</option>
                <option v-for="supplier in suppliers" :key="supplier.id" :value="supplier.id">
                  {{ supplier.name }} · {{ typeLabel(supplier.type) }}
                </option>
              </select>
            </label>
            <label class="block">
              <span class="input-label">本地账号搜索</span>
              <input v-model.trim="localAccountQuery" class="input" placeholder="账号名称、平台或类型" @input="loadLocalAccounts" />
            </label>
          </div>
        </div>

        <DataTable
          :columns="columns"
          :data="bindings"
          :loading="loadingBindings"
          row-key="id"
          default-sort-key="id"
          default-sort-order="desc"
        >
          <template #cell-local_account="{ row }">
            <div class="font-medium text-gray-900 dark:text-white">{{ row.local_account_name }}</div>
            <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">#{{ row.local_sub2api_account_id }} · {{ row.local_account_platform }} / {{ row.local_account_type }}</div>
          </template>

          <template #cell-supplier_account="{ row }">
            <div class="text-sm text-gray-900 dark:text-gray-100">{{ row.supplier_account_label || '-' }}</div>
            <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ row.supplier_account_identifier || '-' }}</div>
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
              <div class="text-xs text-gray-500 dark:text-dark-400">阈值 {{ formatMoney(row.balance_threshold_cents, row.balance_currency) }}</div>
            </div>
          </template>

          <template #cell-concurrency="{ row }">
            <div class="text-right">
              {{ row.observed_max_concurrency || 0 }} / {{ row.configured_concurrency || 0 }}
            </div>
          </template>

          <template #cell-actions="{ row }">
            <div class="flex justify-end gap-2">
              <button type="button" class="btn btn-danger px-3 py-1.5 text-xs" @click="deleteBinding(row)">
                删除
              </button>
            </div>
          </template>

          <template #empty>
            <EmptyState
              title="暂无账号/Key 绑定"
              description="选择供应商后绑定本地 Sub2API 账号。"
              action-text="绑定账号/Key"
              @action="openBindDialog"
            />
          </template>
        </DataTable>
      </section>

      <BaseDialog :show="bindDialogOpen" title="绑定账号/Key" width="wide" @close="bindDialogOpen = false">
        <form id="supplier-account-bind-form" class="space-y-5" @submit.prevent="submitBinding">
          <div class="grid gap-4 sm:grid-cols-2">
            <label class="block">
              <span class="input-label">供应商</span>
              <select v-model.number="bindForm.supplier_id" class="input" required>
                <option :value="0">请选择供应商</option>
                <option v-for="supplier in suppliers" :key="supplier.id" :value="supplier.id">
                  {{ supplier.name }}
                </option>
              </select>
            </label>
            <label class="block">
              <span class="input-label">本地 Sub2API 账号</span>
              <select v-model.number="bindForm.local_sub2api_account_id" class="input" required>
                <option :value="0">请选择账号</option>
                <option v-for="account in localAccounts" :key="account.id" :value="account.id">
                  #{{ account.id }} {{ account.name }} · {{ account.platform }} / {{ account.type }}
                </option>
              </select>
            </label>
          </div>

          <div class="grid gap-4 sm:grid-cols-3">
            <label class="block">
              <span class="input-label">供应商侧标识</span>
              <input v-model.trim="bindForm.supplier_account_identifier" class="input" />
            </label>
            <label class="block">
              <span class="input-label">账号/Key 标签</span>
              <input v-model.trim="bindForm.supplier_account_label" class="input" />
            </label>
            <label class="block">
              <span class="input-label">费率档案</span>
              <input v-model.trim="bindForm.rate_profile" class="input" placeholder="default" />
            </label>
          </div>

          <div class="grid gap-4 sm:grid-cols-3">
            <label class="block">
              <span class="input-label">余额</span>
              <input v-model.number="bindForm.balance_yuan" type="number" min="0" step="0.01" class="input" />
            </label>
            <label class="block">
              <span class="input-label">余额阈值</span>
              <input v-model.number="bindForm.balance_threshold_yuan" type="number" min="0" step="0.01" class="input" />
            </label>
            <label class="block">
              <span class="input-label">币种</span>
              <input v-model.trim="bindForm.balance_currency" class="input" placeholder="CNY" />
            </label>
          </div>

          <div class="grid gap-4 sm:grid-cols-3">
            <label class="block">
              <span class="input-label">配置并发</span>
              <input v-model.number="bindForm.configured_concurrency" type="number" min="0" step="1" class="input" />
            </label>
            <label class="block">
              <span class="input-label">运行状态</span>
              <select v-model="bindForm.runtime_status" class="input">
                <option value="monitor_only">仅监控</option>
                <option value="candidate">候选</option>
                <option value="active">当前使用</option>
                <option value="disabled">停用</option>
              </select>
            </label>
            <label class="block">
              <span class="input-label">健康状态</span>
              <select v-model="bindForm.health_status" class="input">
                <option value="normal">正常</option>
                <option value="unavailable">不可用</option>
                <option value="credential_invalid">凭据失效</option>
                <option value="paused">暂停</option>
              </select>
            </label>
          </div>
        </form>

        <template #footer>
          <button type="button" class="btn btn-secondary" @click="bindDialogOpen = false">取消</button>
          <button type="submit" form="supplier-account-bind-form" class="btn btn-primary" :disabled="submitting">
            {{ submitting ? '绑定中...' : '绑定账号/Key' }}
          </button>
        </template>
      </BaseDialog>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref, watch } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Icon from '@/components/icons/Icon.vue'
import type { Column } from '@/components/common/types'
import { useAppStore } from '@/stores/app'
import {
  createSupplierAccount,
  deleteSupplierAccount,
  listLocalSub2APIAccounts,
  listSupplierAccounts,
  listSuppliers,
  type LocalSub2APIAccount,
  type Supplier,
  type SupplierAccount,
  type SupplierHealthStatus,
  type SupplierRuntimeStatus,
  type SupplierType
} from '@/api/admin/adminPlus'

const appStore = useAppStore()

const loading = ref(false)
const loadingBindings = ref(false)
const submitting = ref(false)
const bindDialogOpen = ref(false)
const suppliers = ref<Supplier[]>([])
const bindings = ref<SupplierAccount[]>([])
const localAccounts = ref<LocalSub2APIAccount[]>([])
const selectedSupplierID = ref(0)
const localAccountQuery = ref('')

const bindForm = reactive({
  supplier_id: 0,
  local_sub2api_account_id: 0,
  supplier_account_identifier: '',
  supplier_account_label: '',
  rate_profile: '',
  balance_yuan: 0,
  balance_threshold_yuan: 0,
  balance_currency: 'CNY',
  configured_concurrency: 0,
  runtime_status: 'monitor_only' as SupplierRuntimeStatus,
  health_status: 'normal' as SupplierHealthStatus
})

const columns: Column[] = [
  { key: 'local_account', label: '本地账号', sortable: true },
  { key: 'supplier_account', label: '供应商侧' },
  { key: 'status', label: '状态' },
  { key: 'balance', label: '余额', class: 'text-right' },
  { key: 'concurrency', label: '并发', class: 'text-right' },
  { key: 'actions', label: '操作', class: 'text-right' }
]

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

async function loadSuppliers() {
  const result = await listSuppliers()
  suppliers.value = result.items
  if (!selectedSupplierID.value && suppliers.value.length > 0) {
    selectedSupplierID.value = suppliers.value[0].id
  }
}

async function loadLocalAccounts() {
  try {
    const result = await listLocalSub2APIAccounts({ q: localAccountQuery.value || undefined, limit: 100 })
    localAccounts.value = result.items
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '加载本地账号失败')
  }
}

async function loadBindings() {
  if (!selectedSupplierID.value) {
    bindings.value = []
    return
  }
  loadingBindings.value = true
  try {
    const result = await listSupplierAccounts(selectedSupplierID.value)
    bindings.value = result.items
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '加载账号/Key 绑定失败')
  } finally {
    loadingBindings.value = false
  }
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

function resetBindForm() {
  bindForm.supplier_id = selectedSupplierID.value
  bindForm.local_sub2api_account_id = 0
  bindForm.supplier_account_identifier = ''
  bindForm.supplier_account_label = ''
  bindForm.rate_profile = ''
  bindForm.balance_yuan = 0
  bindForm.balance_threshold_yuan = 0
  bindForm.balance_currency = 'CNY'
  bindForm.configured_concurrency = 0
  bindForm.runtime_status = 'monitor_only'
  bindForm.health_status = 'normal'
}

function openBindDialog() {
  resetBindForm()
  bindDialogOpen.value = true
}

async function submitBinding() {
  if (!bindForm.supplier_id || !bindForm.local_sub2api_account_id) {
    appStore.showError('请选择供应商和本地账号')
    return
  }
  submitting.value = true
  try {
    await createSupplierAccount(bindForm.supplier_id, {
      local_sub2api_account_id: bindForm.local_sub2api_account_id,
      supplier_account_identifier: bindForm.supplier_account_identifier || undefined,
      supplier_account_label: bindForm.supplier_account_label || undefined,
      rate_profile: bindForm.rate_profile || undefined,
      configured_concurrency: bindForm.configured_concurrency,
      balance_threshold_cents: centsFromYuan(bindForm.balance_threshold_yuan),
      balance_cents: centsFromYuan(bindForm.balance_yuan),
      balance_currency: bindForm.balance_currency || 'CNY',
      runtime_status: bindForm.runtime_status,
      health_status: bindForm.health_status
    })
    selectedSupplierID.value = bindForm.supplier_id
    bindDialogOpen.value = false
    appStore.showSuccess('账号/Key 已绑定')
    await loadBindings()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '绑定账号/Key 失败')
  } finally {
    submitting.value = false
  }
}

async function deleteBinding(row: SupplierAccount) {
  try {
    await deleteSupplierAccount(row.supplier_id, row.id)
    appStore.showSuccess('账号/Key 绑定已删除')
    await loadBindings()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '删除账号/Key 绑定失败')
  }
}

watch(selectedSupplierID, () => {
  loadBindings()
})

onMounted(loadAll)
</script>
