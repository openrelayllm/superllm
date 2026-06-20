<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">余额监控</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
            跟踪供应商余额，区分可切换供应商和仅关注优惠但余额不足的供应商。
          </p>
        </div>
        <button type="button" class="btn btn-secondary" :disabled="loading" @click="loadPage">
          <Icon name="refresh" size="sm" />
          刷新
        </button>
      </section>

      <section class="grid gap-4 md:grid-cols-4">
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">可切换余额</p>
          <p class="mt-2 text-2xl font-semibold text-emerald-600 dark:text-emerald-400">{{ formatMoney(switchableBalanceCents, defaultCurrency) }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">监控余额</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ formatMoney(totalBalanceCents, defaultCurrency) }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">低余额事件</p>
          <p class="mt-2 text-2xl font-semibold text-amber-600 dark:text-amber-400">{{ lowBalanceCount }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">耗尽事件</p>
          <p class="mt-2 text-2xl font-semibold text-rose-600 dark:text-rose-400">{{ depletedCount }}</p>
        </div>
      </section>

      <section class="grid gap-6 xl:grid-cols-[420px_minmax(0,1fr)]">
        <form class="card p-5" @submit.prevent="recordBalance">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">录入余额快照</h2>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">余额大于 0 且状态为 active/candidate 才能参与切换。</p>
          <div class="mt-5 space-y-4">
            <label class="block">
              <span class="input-label">供应商</span>
              <select v-model.number="form.supplier_id" class="input" required>
                <option :value="0" disabled>请选择</option>
                <option v-for="supplier in suppliers" :key="supplier.id" :value="supplier.id">{{ supplier.name }}</option>
              </select>
            </label>
            <div class="grid gap-4 sm:grid-cols-2">
              <label class="block">
                <span class="input-label">余额</span>
                <input v-model.number="form.balance_yuan" type="number" min="0" step="0.01" class="input" required />
              </label>
              <label class="block">
                <span class="input-label">低余额阈值</span>
                <input v-model.number="form.low_balance_threshold_yuan" type="number" min="0" step="0.01" class="input" />
              </label>
            </div>
            <div class="grid gap-4 sm:grid-cols-2">
              <label class="block">
                <span class="input-label">币种</span>
                <input v-model.trim="form.currency" class="input" placeholder="CNY" />
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
            <button type="submit" class="btn btn-primary w-full" :disabled="submitting">
              {{ submitting ? '记录中...' : '记录余额' }}
            </button>
          </div>
        </form>

        <div class="space-y-6">
          <div class="card overflow-hidden">
            <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">供应商余额状态</h2>
            </div>
            <div class="overflow-x-auto">
              <table class="w-full min-w-[760px] divide-y divide-gray-200 dark:divide-dark-700">
                <thead class="bg-gray-50 dark:bg-dark-800">
                  <tr>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">供应商</th>
                    <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">余额</th>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">切换资格</th>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">更新时间</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-900">
                  <tr v-if="suppliers.length === 0">
                    <td colspan="4" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-dark-400">暂无供应商</td>
                  </tr>
                  <tr v-for="supplier in suppliers" :key="supplier.id">
                    <td class="px-4 py-4">
                      <div class="font-medium text-gray-900 dark:text-white">{{ supplier.name }}</div>
                      <div class="text-xs text-gray-500 dark:text-dark-400">{{ supplier.runtime_status }}</div>
                    </td>
                    <td class="px-4 py-4 text-right text-sm text-gray-900 dark:text-gray-100">{{ formatMoney(supplier.balance_cents, supplier.balance_currency) }}</td>
                    <td class="px-4 py-4">
                      <span class="badge" :class="canSwitch(supplier) ? 'badge-success' : 'badge-gray'">
                        {{ canSwitch(supplier) ? '可切换' : '不可切换' }}
                      </span>
                    </td>
                    <td class="px-4 py-4 text-sm text-gray-500 dark:text-dark-400">{{ formatDateTime(supplier.balance_updated_at) }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>

          <div class="card overflow-hidden">
            <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">余额事件</h2>
            </div>
            <div class="divide-y divide-gray-100 dark:divide-dark-700">
              <div v-if="events.length === 0" class="px-5 py-10 text-center text-sm text-gray-500 dark:text-dark-400">暂无余额事件</div>
              <div v-for="event in events" :key="event.id" class="p-5">
                <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                  <div>
                    <div class="flex flex-wrap gap-2">
                      <span class="badge" :class="event.type === 'depleted' ? 'badge-danger' : 'badge-warning'">{{ event.type }}</span>
                      <span class="badge" :class="event.status === 'open' ? 'badge-warning' : 'badge-success'">{{ event.status }}</span>
                      <span class="badge" :class="event.switch_eligible ? 'badge-success' : 'badge-gray'">{{ event.switch_eligible ? '可切换' : '仅监控' }}</span>
                    </div>
                    <p class="mt-2 font-medium text-gray-900 dark:text-white">{{ supplierName(event.supplier_id) }}</p>
                    <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
                      {{ formatMoney(event.old_balance_cents || 0, event.currency) }} -> {{ formatMoney(event.new_balance_cents, event.currency) }}
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
  acknowledgeBalanceEvent,
  listBalanceEvents,
  listSuppliers,
  recordBalanceSnapshot,
  type BalanceEvent,
  type Supplier,
  type SupplierRuntimeStatus
} from '@/api/admin/adminPlus'

const appStore = useAppStore()

const loading = ref(false)
const submitting = ref(false)
const suppliers = ref<Supplier[]>([])
const events = ref<BalanceEvent[]>([])

const form = reactive({
  supplier_id: 0,
  balance_yuan: 0,
  low_balance_threshold_yuan: 10,
  currency: 'CNY',
  runtime_status: 'candidate' as SupplierRuntimeStatus
})

const defaultCurrency = computed(() => suppliers.value[0]?.balance_currency || 'CNY')
const totalBalanceCents = computed(() => suppliers.value.reduce((sum, supplier) => sum + supplier.balance_cents, 0))
const switchableBalanceCents = computed(() => suppliers.value.filter(canSwitch).reduce((sum, supplier) => sum + supplier.balance_cents, 0))
const lowBalanceCount = computed(() => events.value.filter((event) => event.type === 'low_balance' && event.status === 'open').length)
const depletedCount = computed(() => events.value.filter((event) => event.type === 'depleted' && event.status === 'open').length)

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

function supplierName(id: number): string {
  return suppliers.value.find((supplier) => supplier.id === id)?.name || `#${id}`
}

function canSwitch(supplier: Supplier): boolean {
  return ['active', 'candidate'].includes(supplier.runtime_status) && supplier.health_status === 'normal' && supplier.balance_cents > 0
}

async function loadPage() {
  loading.value = true
  try {
    const [supplierResult, eventResult] = await Promise.all([
      listSuppliers(),
      listBalanceEvents({ limit: 100 })
    ])
    suppliers.value = supplierResult.items
    events.value = eventResult.items
    if (!form.supplier_id && suppliers.value[0]) {
      form.supplier_id = suppliers.value[0].id
    }
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '加载余额数据失败')
  } finally {
    loading.value = false
  }
}

async function recordBalance() {
  submitting.value = true
  try {
    await recordBalanceSnapshot({
      supplier_id: form.supplier_id,
      source: 'manual',
      runtime_status: form.runtime_status,
      balance_cents: centsFromYuan(form.balance_yuan),
      low_balance_threshold_cents: centsFromYuan(form.low_balance_threshold_yuan),
      currency: form.currency || 'CNY'
    })
    appStore.showSuccess('余额已记录')
    await loadPage()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '记录余额失败')
  } finally {
    submitting.value = false
  }
}

async function ackEvent(id: number) {
  try {
    await acknowledgeBalanceEvent(id)
    appStore.showSuccess('事件已确认')
    await loadPage()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '确认事件失败')
  }
}

onMounted(loadPage)
</script>
