<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">健康探测</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
            对绑定的供应商账号/Key 发起 OpenAI-compatible Responses 探测，记录首字、总耗时、错误和并发信号。
          </p>
        </div>
        <button type="button" class="btn btn-secondary" :disabled="loading" @click="loadPage">
          <Icon name="refresh" size="sm" />
          刷新
        </button>
      </section>

      <section class="grid gap-4 md:grid-cols-4">
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">样本数</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ samples.length }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">平均首字</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ avgFirstToken }}ms</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">平均耗时</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ avgTotal }}ms</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">待处理事件</p>
          <p class="mt-2 text-2xl font-semibold text-amber-600 dark:text-amber-400">{{ openEvents.length }}</p>
        </div>
      </section>

      <section class="grid gap-6 xl:grid-cols-[420px_minmax(0,1fr)]">
        <form class="card p-5" @submit.prevent="runProbe">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">立即探测</h2>
          <div class="mt-5 space-y-4">
            <label class="block">
              <span class="input-label">供应商</span>
              <select v-model.number="form.supplier_id" class="input" required>
                <option :value="0" disabled>请选择</option>
                <option v-for="supplier in suppliers" :key="supplier.id" :value="supplier.id">
                  {{ supplier.name }}
                </option>
              </select>
            </label>

            <label class="block">
              <span class="input-label">账号/Key</span>
              <select
                v-model.number="form.supplier_account_id"
                class="input"
                :disabled="!form.supplier_id || accountsLoading"
                required
              >
                <option :value="0" disabled>{{ accountPlaceholder }}</option>
                <option v-for="account in supplierAccounts" :key="account.id" :value="account.id">
                  {{ accountLabel(account) }}
                </option>
              </select>
            </label>

            <label class="block">
              <span class="input-label">模型</span>
              <input v-model.trim="form.model" class="input" required placeholder="gpt-5.5" />
            </label>

            <label class="block">
              <span class="input-label">Prompt</span>
              <input v-model.trim="form.prompt" class="input" placeholder="Return exactly: ok" />
            </label>

            <div class="grid gap-4 sm:grid-cols-2">
              <label class="block">
                <span class="input-label">首字阈值 ms</span>
                <input v-model.number="form.first_token_threshold_ms" type="number" min="1" class="input" />
              </label>
              <label class="block">
                <span class="input-label">总耗时阈值 ms</span>
                <input v-model.number="form.total_latency_threshold_ms" type="number" min="1" class="input" />
              </label>
            </div>

            <label class="block">
              <span class="input-label">并发饱和阈值 %</span>
              <input
                v-model.number="form.concurrency_saturation_percent"
                type="number"
                min="1"
                max="100"
                step="1"
                class="input"
              />
            </label>

            <button type="submit" class="btn btn-primary w-full" :disabled="probeDisabled">
              {{ probing ? '探测中...' : '开始探测' }}
            </button>

            <div v-if="lastProbe" class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
              <div class="flex flex-wrap items-center gap-2">
                <span class="badge" :class="sampleStatusClass(lastProbe.sample)">{{ lastProbe.sample.status_code }}</span>
                <span class="badge badge-primary">{{ lastProbe.sample.model }}</span>
                <span v-if="lastProbe.events.length === 0" class="badge badge-success">无新事件</span>
                <span v-else class="badge badge-warning">新事件 {{ lastProbe.events.length }}</span>
              </div>
              <div class="mt-3 grid grid-cols-2 gap-3 text-sm">
                <div>
                  <p class="text-xs text-gray-500 dark:text-dark-400">首字</p>
                  <p class="font-medium text-gray-900 dark:text-gray-100">{{ lastProbe.sample.first_token_latency_ms }}ms</p>
                </div>
                <div>
                  <p class="text-xs text-gray-500 dark:text-dark-400">总耗时</p>
                  <p class="font-medium text-gray-900 dark:text-gray-100">{{ lastProbe.sample.total_latency_ms }}ms</p>
                </div>
              </div>
            </div>
          </div>
        </form>

        <div class="space-y-6">
          <div class="card overflow-hidden">
            <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">最近样本</h2>
            </div>
            <div class="overflow-x-auto">
              <table class="w-full min-w-[980px] divide-y divide-gray-200 dark:divide-dark-700">
                <thead class="bg-gray-50 dark:bg-dark-800">
                  <tr>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">供应商</th>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">账号/Key</th>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">模型</th>
                    <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">首字</th>
                    <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">总耗时</th>
                    <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">并发</th>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">状态</th>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">时间</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-900">
                  <tr v-if="samples.length === 0">
                    <td colspan="8" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-dark-400">暂无样本</td>
                  </tr>
                  <tr v-for="sample in samples" :key="sample.id">
                    <td class="px-4 py-4 text-sm text-gray-900 dark:text-gray-100">{{ supplierName(sample.supplier_id) }}</td>
                    <td class="px-4 py-4 text-sm text-gray-500 dark:text-dark-400">
                      {{ payloadValue(sample, 'local_sub2api_account_name') }}
                    </td>
                    <td class="px-4 py-4 text-sm text-gray-900 dark:text-gray-100">{{ sample.model }}</td>
                    <td class="px-4 py-4 text-right text-sm text-gray-900 dark:text-gray-100">{{ sample.first_token_latency_ms }}ms</td>
                    <td class="px-4 py-4 text-right text-sm text-gray-900 dark:text-gray-100">{{ sample.total_latency_ms }}ms</td>
                    <td class="px-4 py-4 text-right text-sm text-gray-900 dark:text-gray-100">
                      {{ sample.observed_concurrency }} / {{ sample.concurrency_limit ?? '-' }}
                    </td>
                    <td class="px-4 py-4">
                      <span class="badge" :class="sampleStatusClass(sample)">{{ sample.status_code }}</span>
                    </td>
                    <td class="px-4 py-4 text-sm text-gray-500 dark:text-dark-400">{{ formatDateTime(sample.captured_at) }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
            <Pagination
              v-if="samplePagination.total > 0"
              :page="samplePagination.page"
              :total="samplePagination.total"
              :page-size="samplePagination.page_size"
              @update:page="handleSamplePageChange"
              @update:pageSize="handleSamplePageSizeChange"
            />
          </div>

          <div class="card overflow-hidden">
            <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">健康事件</h2>
            </div>
            <div class="divide-y divide-gray-100 dark:divide-dark-700">
              <div v-if="events.length === 0" class="px-5 py-10 text-center text-sm text-gray-500 dark:text-dark-400">暂无健康事件</div>
              <div v-for="event in events" :key="event.id" class="p-5">
                <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                  <div>
                    <div class="flex flex-wrap gap-2">
                      <span class="badge" :class="eventClass(event.type)">{{ eventLabel(event.type) }}</span>
                      <span class="badge" :class="event.status === 'open' ? 'badge-warning' : 'badge-success'">{{ event.status }}</span>
                    </div>
                    <p class="mt-2 font-medium text-gray-900 dark:text-white">{{ supplierName(event.supplier_id) }} · {{ event.model }}</p>
                    <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
                      {{ event.observed_value }} / {{ event.threshold_value }} · HTTP {{ event.status_code }}
                    </p>
                  </div>
                  <button v-if="event.status === 'open'" type="button" class="btn btn-secondary px-3 py-1.5 text-xs" @click="ackEvent(event.id)">
                    确认
                  </button>
                </div>
              </div>
            </div>
            <Pagination
              v-if="eventPagination.total > 0"
              :page="eventPagination.page"
              :total="eventPagination.total"
              :page-size="eventPagination.page_size"
              @update:page="handleEventPageChange"
              @update:pageSize="handleEventPageSizeChange"
            />
          </div>
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import Pagination from '@/components/common/Pagination.vue'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { useAppStore } from '@/stores/app'
import {
  acknowledgeHealthEvent,
  listHealthEvents,
  listHealthSamples,
  listSupplierAccounts,
  listSuppliers,
  probeOpenAIResponsesHealth,
  type HealthEvent,
  type HealthSample,
  type Supplier,
  type SupplierAccount
} from '@/api/admin/adminPlus'

const appStore = useAppStore()

const loading = ref(false)
const accountsLoading = ref(false)
const probing = ref(false)
const suppliers = ref<Supplier[]>([])
const supplierAccounts = ref<SupplierAccount[]>([])
const samples = ref<HealthSample[]>([])
const events = ref<HealthEvent[]>([])
const lastProbe = ref<{ sample: HealthSample; events: HealthEvent[] } | null>(null)
const samplePagination = reactive({ page: 1, page_size: getPersistedPageSize(), total: 0, pages: 0 })
const eventPagination = reactive({ page: 1, page_size: getPersistedPageSize(), total: 0, pages: 0 })

const form = reactive({
  supplier_id: 0,
  supplier_account_id: 0,
  model: 'gpt-5.5',
  prompt: 'Return exactly: ok',
  first_token_threshold_ms: 3000,
  total_latency_threshold_ms: 30000,
  concurrency_saturation_percent: 100
})

const openEvents = computed(() => events.value.filter((event) => event.status === 'open'))
const avgFirstToken = computed(() => average(samples.value.map((sample) => sample.first_token_latency_ms)))
const avgTotal = computed(() => average(samples.value.map((sample) => sample.total_latency_ms)))
const accountPlaceholder = computed(() => {
  if (!form.supplier_id) return '请先选择供应商'
  if (accountsLoading.value) return '加载中...'
  if (supplierAccounts.value.length === 0) return '无可用账号/Key'
  return '请选择账号/Key'
})
const probeDisabled = computed(() => probing.value || !form.supplier_id || !form.supplier_account_id || !form.model)

function average(values: number[]): number {
  const valid = values.filter((value) => value > 0)
  if (valid.length === 0) return 0
  return Math.round(valid.reduce((sum, value) => sum + value, 0) / valid.length)
}

function supplierName(id: number): string {
  return suppliers.value.find((supplier) => supplier.id === id)?.name || `#${id}`
}

function accountLabel(account: SupplierAccount): string {
  const name = account.supplier_account_label || account.local_account_name || `#${account.local_sub2api_account_id}`
  return `${name} · ${account.local_account_platform}/${account.local_account_type}`
}

function payloadValue(sample: HealthSample, key: string): string {
  const value = sample.raw_payload?.[key]
  if (typeof value === 'string' && value.trim()) return value
  if (typeof value === 'number' && Number.isFinite(value)) return String(value)
  return '-'
}

function formatDateTime(value?: string | null): string {
  if (!value) return '-'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '-' : date.toLocaleString()
}

function sampleStatusClass(sample: HealthSample): string {
  if (sample.status_code >= 400 || sample.error_class) return 'badge-danger'
  return 'badge-success'
}

function eventClass(type: HealthEvent['type']): string {
  if (type === 'request_error') return 'badge-danger'
  if (type === 'concurrency_full') return 'badge-primary'
  return 'badge-warning'
}

function eventLabel(type: HealthEvent['type']): string {
  const labels: Record<HealthEvent['type'], string> = {
    slow_first_token: '首字慢',
    slow_total: '总耗时慢',
    request_error: '请求错误',
    concurrency_full: '并发满'
  }
  return labels[type] || type
}

function positiveNumber(value: number): number | undefined {
  const normalized = Number(value || 0)
  return normalized > 0 ? normalized : undefined
}

async function loadSupplierAccounts(supplierId: number) {
  supplierAccounts.value = []
  form.supplier_account_id = 0
  if (!supplierId) return

  accountsLoading.value = true
  try {
    const result = await listSupplierAccounts(supplierId)
    supplierAccounts.value = result.items
    form.supplier_account_id = result.items[0]?.id || 0
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '加载供应商账号失败')
  } finally {
    accountsLoading.value = false
  }
}

async function loadPage() {
  loading.value = true
  try {
    const [supplierResult, sampleResult, eventResult] = await Promise.all([
      listSuppliers(),
      listHealthSamples({ page: samplePagination.page, page_size: samplePagination.page_size }),
      listHealthEvents({ page: eventPagination.page, page_size: eventPagination.page_size })
    ])
    suppliers.value = supplierResult.items
    samples.value = sampleResult.items
    events.value = eventResult.items
    samplePagination.total = sampleResult.total || 0
    samplePagination.pages = sampleResult.pages || 0
    samplePagination.page = sampleResult.page || samplePagination.page
    samplePagination.page_size = sampleResult.page_size || samplePagination.page_size
    eventPagination.total = eventResult.total || 0
    eventPagination.pages = eventResult.pages || 0
    eventPagination.page = eventResult.page || eventPagination.page
    eventPagination.page_size = eventResult.page_size || eventPagination.page_size
    if (!form.supplier_id && suppliers.value[0]) {
      form.supplier_id = suppliers.value[0].id
    } else if (form.supplier_id) {
      await loadSupplierAccounts(form.supplier_id)
    }
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '加载健康数据失败')
  } finally {
    loading.value = false
  }
}

function handleSamplePageChange(page: number) {
  samplePagination.page = page
  void loadPage()
}

function handleSamplePageSizeChange(pageSize: number) {
  samplePagination.page_size = pageSize
  samplePagination.page = 1
  void loadPage()
}

function handleEventPageChange(page: number) {
  eventPagination.page = page
  void loadPage()
}

function handleEventPageSizeChange(pageSize: number) {
  eventPagination.page_size = pageSize
  eventPagination.page = 1
  void loadPage()
}

async function runProbe() {
  probing.value = true
  try {
    const result = await probeOpenAIResponsesHealth({
      supplier_id: form.supplier_id,
      supplier_account_id: form.supplier_account_id,
      model: form.model,
      prompt: form.prompt || undefined,
      first_token_threshold_ms: positiveNumber(form.first_token_threshold_ms),
      total_latency_threshold_ms: positiveNumber(form.total_latency_threshold_ms),
      concurrency_saturation_percent: positiveNumber(form.concurrency_saturation_percent)
    })
    lastProbe.value = result
    appStore.showSuccess(result.events.length > 0 ? `探测完成，生成 ${result.events.length} 个事件` : '探测完成')
    await loadPage()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '健康探测失败')
  } finally {
    probing.value = false
  }
}

async function ackEvent(id: number) {
  try {
    await acknowledgeHealthEvent(id)
    appStore.showSuccess('事件已确认')
    await loadPage()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '确认事件失败')
  }
}

watch(
  () => form.supplier_id,
  (supplierId) => {
    void loadSupplierAccounts(supplierId)
  }
)

onMounted(loadPage)
</script>
