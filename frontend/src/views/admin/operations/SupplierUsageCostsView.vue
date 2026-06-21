<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">用量消耗</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
            供应商用户侧 usage 明细，只作为成本台账的消耗输入，不代表完整成本对账。
          </p>
        </div>
        <div class="flex flex-wrap gap-2">
          <button type="button" class="btn btn-secondary" :disabled="loading" @click="loadPage">
            <Icon name="refresh" size="sm" />
            刷新
          </button>
          <button type="button" class="btn btn-primary" :disabled="syncing || !filter.supplier_id" @click="syncFromSupplier">
            <Icon name="sync" size="sm" />
            {{ syncing ? '同步中...' : '同步用量' }}
          </button>
        </div>
      </section>

      <section class="grid gap-4 md:grid-cols-4">
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">明细数</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ pagination.total }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">当前页消耗</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ formatMoney(pageCostCents, defaultCurrency) }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">当前页 Token</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ pageTokens.toLocaleString() }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">同步状态</p>
          <p class="mt-2 text-lg font-semibold text-gray-900 dark:text-white">{{ syncStatusLabel }}</p>
        </div>
      </section>

      <section class="card p-5">
        <div class="grid gap-4 lg:grid-cols-[1.2fr_1fr_1fr_auto_auto] lg:items-end">
          <label class="block">
            <span class="input-label">供应商</span>
            <select v-model.number="filter.supplier_id" class="input" @change="handleSupplierChange">
              <option :value="0">全部供应商</option>
              <option v-for="supplier in suppliers" :key="supplier.id" :value="supplier.id">{{ supplier.name }}</option>
            </select>
          </label>
          <label class="block">
            <span class="input-label">开始时间</span>
            <input v-model="syncForm.started_at" type="datetime-local" class="input" />
          </label>
          <label class="block">
            <span class="input-label">结束时间</span>
            <input v-model="syncForm.ended_at" type="datetime-local" class="input" />
          </label>
          <button type="button" class="btn btn-secondary" @click="openImportDialog">
            <Icon name="plus" size="sm" />
            手工补录
          </button>
          <button type="button" class="btn btn-primary" :disabled="syncing || !filter.supplier_id" @click="syncFromSupplier">
            <Icon name="sync" size="sm" />
            同步
          </button>
        </div>
      </section>

      <section class="card overflow-hidden">
        <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">供应商 usage 明细</h2>
        </div>
        <div class="overflow-x-auto">
          <table class="w-full min-w-[1500px] divide-y divide-gray-200 dark:divide-dark-700">
            <thead class="bg-gray-50 dark:bg-dark-800">
              <tr>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">供应商</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">API Key</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">请求 ID</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">模型</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">端点 / 类型</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">计费</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">消耗</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">Token</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">耗时</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">时间</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-900">
              <tr v-if="lines.length === 0">
                <td colspan="10" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-dark-400">暂无用量消耗明细</td>
              </tr>
              <tr v-for="line in lines" :key="line.id">
                <td class="px-4 py-4 text-sm text-gray-900 dark:text-gray-100">{{ supplierName(line.supplier_id) }}</td>
                <td class="px-4 py-4 text-sm text-gray-900 dark:text-gray-100">{{ line.api_key_name || '-' }}</td>
                <td class="px-4 py-4 font-mono text-xs text-gray-500 dark:text-dark-400">{{ line.external_request_id || '-' }}</td>
                <td class="px-4 py-4 text-sm text-gray-900 dark:text-gray-100">{{ line.model }}</td>
                <td class="px-4 py-4 text-sm text-gray-900 dark:text-gray-100">
                  <div>{{ line.endpoint || '-' }}</div>
                  <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ line.request_type || '-' }}</div>
                </td>
                <td class="px-4 py-4 text-sm text-gray-900 dark:text-gray-100">
                  <div>{{ usageModeLabel(line.billing_mode) }}</div>
                  <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">推理 {{ line.reasoning_effort || '-' }}</div>
                </td>
                <td class="px-4 py-4 text-right text-sm text-gray-900 dark:text-gray-100">{{ formatMoney(line.cost_cents, line.currency) }}</td>
                <td class="px-4 py-4 text-right text-xs text-gray-900 dark:text-gray-100">
                  <div>入 {{ formatInteger(line.input_tokens) }} / 出 {{ formatInteger(line.output_tokens) }}</div>
                  <div class="mt-1 text-gray-500 dark:text-dark-400">缓存 {{ formatInteger(line.cache_read_tokens) }} / 总 {{ formatInteger(totalTokens(line)) }}</div>
                </td>
                <td class="px-4 py-4 text-right text-xs text-gray-900 dark:text-gray-100">
                  <div>首 {{ formatDuration(line.first_token_ms) }}</div>
                  <div class="mt-1 text-gray-500 dark:text-dark-400">总 {{ formatDuration(line.duration_ms) }}</div>
                </td>
                <td class="px-4 py-4 text-sm text-gray-500 dark:text-dark-400">{{ formatDateTime(line.started_at) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </section>
    </div>

    <BaseDialog :show="importDialogOpen" title="补录用量消耗" width="wide" @close="importDialogOpen = false">
      <form id="usage-cost-import-form" class="grid gap-4 md:grid-cols-2" @submit.prevent="importLine">
        <label class="block md:col-span-2">
          <span class="input-label">供应商</span>
          <select v-model.number="form.supplier_id" class="input" required>
            <option :value="0" disabled>请选择</option>
            <option v-for="supplier in suppliers" :key="supplier.id" :value="supplier.id">{{ supplier.name }}</option>
          </select>
        </label>
        <label class="block md:col-span-2">
          <span class="input-label">请求 ID</span>
          <input v-model.trim="form.external_request_id" class="input" required />
        </label>
        <label class="block">
          <span class="input-label">API Key 名称</span>
          <input v-model.trim="form.api_key_name" class="input" />
        </label>
        <label class="block">
          <span class="input-label">模型</span>
          <input v-model.trim="form.model" class="input" required />
        </label>
        <label class="block">
          <span class="input-label">端点</span>
          <input v-model.trim="form.endpoint" class="input" placeholder="/v1/responses" />
        </label>
        <label class="block">
          <span class="input-label">类型</span>
          <input v-model.trim="form.request_type" class="input" placeholder="responses / chat" />
        </label>
        <label class="block">
          <span class="input-label">计费模式</span>
          <select v-model="form.billing_mode" class="input">
            <option value="token">按 Token</option>
            <option value="request">按次</option>
            <option value="image">图片</option>
          </select>
        </label>
        <label class="block">
          <span class="input-label">消耗金额</span>
          <input v-model.number="form.cost_yuan" type="number" min="0" step="0.01" class="input" required />
        </label>
        <label class="block">
          <span class="input-label">币种</span>
          <input v-model.trim="form.currency" class="input" placeholder="USD" />
        </label>
        <label class="block">
          <span class="input-label">输入 Token</span>
          <input v-model.number="form.input_tokens" type="number" min="0" step="1" class="input" />
        </label>
        <label class="block">
          <span class="input-label">输出 Token</span>
          <input v-model.number="form.output_tokens" type="number" min="0" step="1" class="input" />
        </label>
        <label class="block">
          <span class="input-label">缓存 Token</span>
          <input v-model.number="form.cache_read_tokens" type="number" min="0" step="1" class="input" />
        </label>
        <label class="block">
          <span class="input-label">首 Token 毫秒</span>
          <input v-model.number="form.first_token_ms" type="number" min="0" step="1" class="input" />
        </label>
        <label class="block">
          <span class="input-label">总耗时毫秒</span>
          <input v-model.number="form.duration_ms" type="number" min="0" step="1" class="input" />
        </label>
      </form>
      <template #footer>
        <button type="button" class="btn btn-secondary" @click="importDialogOpen = false">取消</button>
        <button type="submit" form="usage-cost-import-form" class="btn btn-primary" :disabled="importing">
          {{ importing ? '补录中...' : '补录' }}
        </button>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import Pagination from '@/components/common/Pagination.vue'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { useAppStore } from '@/stores/app'
import {
  importUsageCostLines,
  listSuppliers,
  listUsageCostLines,
  syncSupplierUsageCosts,
  type Supplier,
  type SupplierUsageCostLine,
  type SyncSupplierUsageCostsResponse
} from '@/api/admin/adminPlus'

const appStore = useAppStore()

const loading = ref(false)
const syncing = ref(false)
const importing = ref(false)
const importDialogOpen = ref(false)
const suppliers = ref<Supplier[]>([])
const lines = ref<SupplierUsageCostLine[]>([])
const lastSync = ref<SyncSupplierUsageCostsResponse | null>(null)
const pagination = reactive({ page: 1, page_size: getPersistedPageSize(), total: 0, pages: 0 })

const filter = reactive({ supplier_id: 0 })
const syncForm = reactive({
  started_at: toDateTimeLocal(new Date(Date.now() - 24 * 60 * 60 * 1000)),
  ended_at: toDateTimeLocal(new Date())
})
const form = reactive({
  supplier_id: 0,
  external_request_id: '',
  api_key_name: '',
  model: 'gpt-4o-mini',
  endpoint: '',
  request_type: '',
  billing_mode: 'token',
  cost_yuan: 0,
  currency: 'USD',
  input_tokens: 0,
  output_tokens: 0,
  cache_read_tokens: 0,
  first_token_ms: 0,
  duration_ms: 0
})

const defaultCurrency = computed(() => lines.value[0]?.currency || 'USD')
const pageCostCents = computed(() => lines.value.reduce((sum, line) => sum + line.cost_cents, 0))
const pageTokens = computed(() => lines.value.reduce((sum, line) => sum + totalTokens(line), 0))
const syncStatusLabel = computed(() => {
  if (syncing.value) return '正在同步'
  if (lastSync.value) return `${lastSync.value.total} 条`
  if (!filter.supplier_id) return '选择供应商'
  return '等待同步'
})

function centsFromYuan(value: number): number {
  return Math.round(Number(value || 0) * 100)
}

function formatMoney(cents: number, currency: string): string {
  return new Intl.NumberFormat(undefined, {
    style: 'currency',
    currency: currency || 'USD',
    minimumFractionDigits: 2
  }).format((cents || 0) / 100)
}

function formatDateTime(value?: string | null): string {
  if (!value) return '-'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '-' : date.toLocaleString()
}

function formatInteger(value?: number | null): string {
  return Number(value || 0).toLocaleString()
}

function totalTokens(line: SupplierUsageCostLine): number {
  return line.total_tokens || line.input_tokens + line.output_tokens + (line.cache_read_tokens || 0)
}

function formatDuration(value?: number | null): string {
  const ms = Number(value || 0)
  if (ms <= 0) return '-'
  if (ms >= 1000) return `${(ms / 1000).toFixed(2)}s`
  return `${ms}ms`
}

function usageModeLabel(value?: string): string {
  return {
    token: '按 Token',
    request: '按次',
    image: '图片'
  }[value || ''] || value || '-'
}

function supplierName(id: number): string {
  return suppliers.value.find((supplier) => supplier.id === id)?.name || `#${id}`
}

function toDateTimeLocal(value: Date): string {
  const offsetMs = value.getTimezoneOffset() * 60 * 1000
  return new Date(value.getTime() - offsetMs).toISOString().slice(0, 16)
}

function toRFC3339(value: string): string {
  return new Date(value).toISOString()
}

async function loadPage() {
  loading.value = true
  try {
    const [supplierResult] = await Promise.all([
      listSuppliers({ limit: 200 }),
      loadLines()
    ])
    suppliers.value = supplierResult.items
    if (!filter.supplier_id && suppliers.value[0]) {
      filter.supplier_id = suppliers.value[0].id
      form.supplier_id = suppliers.value[0].id
      await loadLines()
    }
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '加载用量消耗失败')
  } finally {
    loading.value = false
  }
}

async function loadLines() {
  const result = await listUsageCostLines({
    supplier_id: filter.supplier_id || undefined,
    page: pagination.page,
    page_size: pagination.page_size
  })
  lines.value = result.items
  pagination.total = result.total || 0
  pagination.pages = result.pages || 0
  pagination.page = result.page || pagination.page
  pagination.page_size = result.page_size || pagination.page_size
}

function handleSupplierChange() {
  pagination.page = 1
  lastSync.value = null
  if (filter.supplier_id) {
    form.supplier_id = filter.supplier_id
  }
  void loadLines()
}

function handlePageChange(page: number) {
  pagination.page = page
  void loadLines()
}

function handlePageSizeChange(pageSize: number) {
  pagination.page_size = pageSize
  pagination.page = 1
  void loadLines()
}

function openImportDialog() {
  if (!form.supplier_id && filter.supplier_id) {
    form.supplier_id = filter.supplier_id
  }
  importDialogOpen.value = true
}

async function importLine() {
  importing.value = true
  try {
    await importUsageCostLines([{
      supplier_id: form.supplier_id,
      external_request_id: form.external_request_id,
      api_key_name: form.api_key_name,
      model: form.model,
      endpoint: form.endpoint,
      request_type: form.request_type,
      billing_mode: form.billing_mode,
      currency: form.currency || 'USD',
      cost_cents: centsFromYuan(form.cost_yuan),
      input_tokens: Number(form.input_tokens || 0),
      output_tokens: Number(form.output_tokens || 0),
      cache_read_tokens: Number(form.cache_read_tokens || 0),
      total_tokens: Number(form.input_tokens || 0) + Number(form.output_tokens || 0) + Number(form.cache_read_tokens || 0),
      first_token_ms: Number(form.first_token_ms || 0),
      duration_ms: Number(form.duration_ms || 0),
      started_at: new Date().toISOString()
    }])
    form.external_request_id = ''
    pagination.page = 1
    importDialogOpen.value = false
    await loadLines()
    appStore.showSuccess('用量消耗已补录')
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '补录用量消耗失败')
  } finally {
    importing.value = false
  }
}

async function syncFromSupplier() {
  if (!filter.supplier_id) {
    appStore.showError('请选择供应商')
    return
  }
  syncing.value = true
  try {
    const result = await syncSupplierUsageCosts(filter.supplier_id, {
      started_at: toRFC3339(syncForm.started_at),
      ended_at: toRFC3339(syncForm.ended_at)
    })
    lastSync.value = result
    pagination.page = 1
    await loadLines()
    appStore.showSuccess(`已同步 ${result.total} 条用量消耗`)
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '同步用量消耗失败')
  } finally {
    syncing.value = false
  }
}

onMounted(loadPage)
</script>
