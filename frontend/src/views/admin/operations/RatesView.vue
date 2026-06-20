<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">费率监控</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
            记录第三方费率快照，发现模型价格新增、上涨、下降，并对异常变更做确认。
          </p>
        </div>
        <button type="button" class="btn btn-secondary" :disabled="loading" @click="loadPage">
          <Icon name="refresh" size="sm" />
          刷新
        </button>
      </section>

      <section class="grid gap-4 md:grid-cols-4">
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">快照条目</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ snapshots.length }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">待确认变更</p>
          <p class="mt-2 text-2xl font-semibold text-amber-600 dark:text-amber-400">{{ openEvents.length }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">涨价</p>
          <p class="mt-2 text-2xl font-semibold text-rose-600 dark:text-rose-400">{{ increaseCount }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">降价</p>
          <p class="mt-2 text-2xl font-semibold text-emerald-600 dark:text-emerald-400">{{ decreaseCount }}</p>
        </div>
      </section>

      <section class="grid gap-6 xl:grid-cols-[420px_minmax(0,1fr)]">
        <form class="card p-5" @submit.prevent="recordSnapshot">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">手动录入费率</h2>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">Chrome 插件和定时任务接入前，先用手动录入验证价格模型。</p>
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
                <span class="input-label">计费模式</span>
                <input v-model.trim="form.billing_mode" class="input" required placeholder="token" />
              </label>
              <label class="block">
                <span class="input-label">价格项</span>
                <input v-model.trim="form.price_item" class="input" required placeholder="input" />
              </label>
            </div>
            <div class="grid gap-4 sm:grid-cols-2">
              <label class="block">
                <span class="input-label">单位</span>
                <input v-model.trim="form.unit" class="input" required placeholder="1M tokens" />
              </label>
              <label class="block">
                <span class="input-label">币种</span>
                <input v-model.trim="form.currency" class="input" placeholder="CNY" />
              </label>
            </div>
            <label class="block">
              <span class="input-label">价格 micros</span>
              <input v-model.number="form.price_micros" type="number" min="0" class="input" required />
            </label>
            <label class="block">
              <span class="input-label">变动阈值 %</span>
              <input v-model.number="form.threshold_percent" type="number" min="0" step="0.01" class="input" />
            </label>
            <button type="submit" class="btn btn-primary w-full" :disabled="submitting">
              {{ submitting ? '记录中...' : '记录费率快照' }}
            </button>
          </div>
        </form>

        <div class="space-y-6">
          <div class="card overflow-hidden">
            <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">最近费率快照</h2>
            </div>
            <div class="overflow-x-auto">
              <table class="w-full min-w-[840px] divide-y divide-gray-200 dark:divide-dark-700">
                <thead class="bg-gray-50 dark:bg-dark-800">
                  <tr>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">供应商</th>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">模型</th>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">价格项</th>
                    <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">micros</th>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">时间</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-900">
                  <tr v-if="snapshots.length === 0">
                    <td colspan="5" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-dark-400">暂无快照</td>
                  </tr>
                  <tr v-for="snapshot in snapshots" :key="snapshot.id">
                    <td class="px-4 py-4 text-sm text-gray-900 dark:text-gray-100">{{ supplierName(snapshot.supplier_id) }}</td>
                    <td class="px-4 py-4 text-sm text-gray-900 dark:text-gray-100">{{ snapshot.model }}</td>
                    <td class="px-4 py-4 text-sm text-gray-500 dark:text-dark-400">{{ snapshot.billing_mode }} / {{ snapshot.price_item }} / {{ snapshot.unit }}</td>
                    <td class="px-4 py-4 text-right font-mono text-sm text-gray-900 dark:text-gray-100">{{ snapshot.price_micros }}</td>
                    <td class="px-4 py-4 text-sm text-gray-500 dark:text-dark-400">{{ formatDateTime(snapshot.captured_at) }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>

          <div class="card overflow-hidden">
            <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">费率变更事件</h2>
            </div>
            <div class="divide-y divide-gray-100 dark:divide-dark-700">
              <div v-if="events.length === 0" class="px-5 py-10 text-center text-sm text-gray-500 dark:text-dark-400">暂无变更事件</div>
              <div v-for="event in events" :key="event.id" class="p-5">
                <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                  <div>
                    <div class="flex flex-wrap items-center gap-2">
                      <span class="badge" :class="directionClass(event.direction)">{{ event.direction }}</span>
                      <span class="badge" :class="statusClass(event.status)">{{ event.status }}</span>
                      <span v-if="event.threshold_exceeded" class="badge badge-danger">超过阈值</span>
                    </div>
                    <p class="mt-2 font-medium text-gray-900 dark:text-white">{{ supplierName(event.supplier_id) }} · {{ event.model }}</p>
                    <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
                      {{ event.old_price_micros ?? '-' }} -> {{ event.new_price_micros }} micros
                      <span v-if="event.change_percent"> · {{ event.change_percent.toFixed(2) }}%</span>
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
  acknowledgeRateEvent,
  listRateEvents,
  listRateSnapshots,
  listSuppliers,
  recordRateSnapshot,
  type RateChangeEvent,
  type RateSnapshot,
  type Supplier
} from '@/api/admin/adminPlus'

const appStore = useAppStore()

const loading = ref(false)
const submitting = ref(false)
const suppliers = ref<Supplier[]>([])
const snapshots = ref<RateSnapshot[]>([])
const events = ref<RateChangeEvent[]>([])

const form = reactive({
  supplier_id: 0,
  model: 'gpt-4o-mini',
  billing_mode: 'token',
  price_item: 'input',
  unit: '1M tokens',
  currency: 'CNY',
  price_micros: 0,
  threshold_percent: 1
})

const openEvents = computed(() => events.value.filter((event) => event.status === 'open'))
const increaseCount = computed(() => events.value.filter((event) => event.direction === 'increase').length)
const decreaseCount = computed(() => events.value.filter((event) => event.direction === 'decrease').length)

function supplierName(id: number): string {
  return suppliers.value.find((supplier) => supplier.id === id)?.name || `#${id}`
}

function formatDateTime(value: string): string {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '-' : date.toLocaleString()
}

function directionClass(direction: RateChangeEvent['direction']): string {
  if (direction === 'increase') return 'badge-danger'
  if (direction === 'decrease') return 'badge-success'
  return 'badge-primary'
}

function statusClass(status: string): string {
  if (status === 'open') return 'badge-warning'
  if (status === 'acknowledged') return 'badge-success'
  return 'badge-gray'
}

async function loadPage() {
  loading.value = true
  try {
    const [supplierResult, snapshotResult, eventResult] = await Promise.all([
      listSuppliers(),
      listRateSnapshots({ limit: 100 }),
      listRateEvents({ limit: 100 })
    ])
    suppliers.value = supplierResult.items
    snapshots.value = snapshotResult.items
    events.value = eventResult.items
    if (!form.supplier_id && suppliers.value[0]) {
      form.supplier_id = suppliers.value[0].id
    }
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '加载费率数据失败')
  } finally {
    loading.value = false
  }
}

async function recordSnapshot() {
  submitting.value = true
  try {
    await recordRateSnapshot({
      supplier_id: form.supplier_id,
      source: 'manual',
      threshold_percent: form.threshold_percent,
      entries: [{
        model: form.model,
        billing_mode: form.billing_mode,
        price_item: form.price_item,
        unit: form.unit,
        currency: form.currency || 'CNY',
        price_micros: Number(form.price_micros || 0)
      }]
    })
    appStore.showSuccess('费率快照已记录')
    await loadPage()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '记录费率失败')
  } finally {
    submitting.value = false
  }
}

async function ackEvent(id: number) {
  try {
    await acknowledgeRateEvent(id)
    appStore.showSuccess('事件已确认')
    await loadPage()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '确认事件失败')
  }
}

onMounted(loadPage)
</script>
