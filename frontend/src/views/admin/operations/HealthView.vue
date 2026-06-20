<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">健康监控</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
            记录供应商首字延迟、总耗时、错误和并发，用于判断是否降权或切换。
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
        <form class="card p-5" @submit.prevent="recordSample">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">录入健康样本</h2>
          <div class="mt-5 space-y-4">
            <label class="block">
              <span class="input-label">供应商</span>
              <select v-model.number="form.supplier_id" class="input" required>
                <option :value="0" disabled>请选择</option>
                <option v-for="supplier in suppliers" :key="supplier.id" :value="supplier.id">{{ supplier.name }}</option>
              </select>
            </label>
            <label class="block">
              <span class="input-label">模型</span>
              <input v-model.trim="form.model" class="input" required placeholder="gpt-4o-mini" />
            </label>
            <div class="grid gap-4 sm:grid-cols-2">
              <label class="block">
                <span class="input-label">首字延迟 ms</span>
                <input v-model.number="form.first_token_latency_ms" type="number" min="0" class="input" />
              </label>
              <label class="block">
                <span class="input-label">总耗时 ms</span>
                <input v-model.number="form.total_latency_ms" type="number" min="0" class="input" />
              </label>
            </div>
            <div class="grid gap-4 sm:grid-cols-2">
              <label class="block">
                <span class="input-label">状态码</span>
                <input v-model.number="form.status_code" type="number" min="0" class="input" />
              </label>
              <label class="block">
                <span class="input-label">错误类别</span>
                <input v-model.trim="form.error_class" class="input" placeholder="timeout / 5xx" />
              </label>
            </div>
            <div class="grid gap-4 sm:grid-cols-3">
              <label class="block">
                <span class="input-label">当前并发</span>
                <input v-model.number="form.observed_concurrency" type="number" min="0" class="input" />
              </label>
              <label class="block">
                <span class="input-label">可用并发</span>
                <input v-model.number="form.available_concurrency" type="number" min="0" class="input" />
              </label>
              <label class="block">
                <span class="input-label">并发上限</span>
                <input v-model.number="form.concurrency_limit" type="number" min="0" class="input" />
              </label>
            </div>
            <button type="submit" class="btn btn-primary w-full" :disabled="submitting">
              {{ submitting ? '记录中...' : '记录健康样本' }}
            </button>
          </div>
        </form>

        <div class="space-y-6">
          <div class="card overflow-hidden">
            <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">最近样本</h2>
            </div>
            <div class="overflow-x-auto">
              <table class="w-full min-w-[900px] divide-y divide-gray-200 dark:divide-dark-700">
                <thead class="bg-gray-50 dark:bg-dark-800">
                  <tr>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">供应商</th>
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
                    <td colspan="7" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-dark-400">暂无样本</td>
                  </tr>
                  <tr v-for="sample in samples" :key="sample.id">
                    <td class="px-4 py-4 text-sm text-gray-900 dark:text-gray-100">{{ supplierName(sample.supplier_id) }}</td>
                    <td class="px-4 py-4 text-sm text-gray-900 dark:text-gray-100">{{ sample.model }}</td>
                    <td class="px-4 py-4 text-right text-sm text-gray-900 dark:text-gray-100">{{ sample.first_token_latency_ms }}ms</td>
                    <td class="px-4 py-4 text-right text-sm text-gray-900 dark:text-gray-100">{{ sample.total_latency_ms }}ms</td>
                    <td class="px-4 py-4 text-right text-sm text-gray-900 dark:text-gray-100">{{ sample.observed_concurrency }} / {{ sample.concurrency_limit ?? '-' }}</td>
                    <td class="px-4 py-4">
                      <span class="badge" :class="sample.status_code >= 400 ? 'badge-danger' : 'badge-success'">{{ sample.status_code }}</span>
                    </td>
                    <td class="px-4 py-4 text-sm text-gray-500 dark:text-dark-400">{{ formatDateTime(sample.captured_at) }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
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
                      <span class="badge" :class="event.type === 'request_error' ? 'badge-danger' : 'badge-warning'">{{ event.type }}</span>
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
          </div>
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import {
  acknowledgeHealthEvent,
  listHealthEvents,
  listHealthSamples,
  listSuppliers,
  recordHealthSample,
  type HealthEvent,
  type HealthSample,
  type Supplier
} from '@/api/admin/adminPlus'

const appStore = useAppStore()

const loading = ref(false)
const submitting = ref(false)
const suppliers = ref<Supplier[]>([])
const samples = ref<HealthSample[]>([])
const events = ref<HealthEvent[]>([])

const form = reactive({
  supplier_id: 0,
  model: 'gpt-4o-mini',
  first_token_latency_ms: 0,
  total_latency_ms: 0,
  status_code: 200,
  error_class: '',
  observed_concurrency: 0,
  available_concurrency: 0,
  concurrency_limit: 0
})

const openEvents = computed(() => events.value.filter((event) => event.status === 'open'))
const avgFirstToken = computed(() => average(samples.value.map((sample) => sample.first_token_latency_ms)))
const avgTotal = computed(() => average(samples.value.map((sample) => sample.total_latency_ms)))

function average(values: number[]): number {
  const valid = values.filter((value) => value > 0)
  if (valid.length === 0) return 0
  return Math.round(valid.reduce((sum, value) => sum + value, 0) / valid.length)
}

function supplierName(id: number): string {
  return suppliers.value.find((supplier) => supplier.id === id)?.name || `#${id}`
}

function formatDateTime(value: string): string {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '-' : date.toLocaleString()
}

function positiveOrNull(value: number): number | null {
  return value > 0 ? value : null
}

async function loadPage() {
  loading.value = true
  try {
    const [supplierResult, sampleResult, eventResult] = await Promise.all([
      listSuppliers(),
      listHealthSamples({ limit: 100 }),
      listHealthEvents({ limit: 100 })
    ])
    suppliers.value = supplierResult.items
    samples.value = sampleResult.items
    events.value = eventResult.items
    if (!form.supplier_id && suppliers.value[0]) {
      form.supplier_id = suppliers.value[0].id
    }
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '加载健康数据失败')
  } finally {
    loading.value = false
  }
}

async function recordSample() {
  submitting.value = true
  try {
    await recordHealthSample({
      supplier_id: form.supplier_id,
      source: 'manual',
      model: form.model,
      first_token_latency_ms: Number(form.first_token_latency_ms || 0),
      total_latency_ms: Number(form.total_latency_ms || 0),
      status_code: Number(form.status_code || 200),
      error_class: form.error_class || undefined,
      observed_concurrency: Number(form.observed_concurrency || 0),
      available_concurrency: positiveOrNull(form.available_concurrency),
      concurrency_limit: positiveOrNull(form.concurrency_limit)
    })
    appStore.showSuccess('健康样本已记录')
    await loadPage()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '记录健康样本失败')
  } finally {
    submitting.value = false
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

onMounted(loadPage)
</script>
