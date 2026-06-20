<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">动作建议</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
            汇总供应商余额、健康、优惠和利润信号，生成充值、切换、降权和排查建议。
          </p>
        </div>
        <div class="flex flex-wrap gap-2">
          <button type="button" class="btn btn-secondary" :disabled="loading" @click="loadPage">
            <Icon name="refresh" size="sm" />
            刷新
          </button>
          <button type="button" class="btn btn-primary" :disabled="generating || suppliers.length === 0" @click="generate">
            {{ generating ? '生成中...' : '生成建议' }}
          </button>
        </div>
      </section>

      <section class="grid gap-4 md:grid-cols-4">
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">开放建议</p>
          <p class="mt-2 text-2xl font-semibold text-amber-600 dark:text-amber-400">{{ openCount }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">严重</p>
          <p class="mt-2 text-2xl font-semibold text-rose-600 dark:text-rose-400">{{ criticalCount }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">可切换供应商</p>
          <p class="mt-2 text-2xl font-semibold text-emerald-600 dark:text-emerald-400">{{ switchableCount }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">待处理信号</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ openSignalCount }}</p>
        </div>
      </section>

      <section class="card overflow-hidden">
        <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">建议队列</h2>
        </div>

        <div class="divide-y divide-gray-100 dark:divide-dark-700">
          <div v-if="recommendations.length === 0" class="px-5 py-10 text-center text-sm text-gray-500 dark:text-dark-400">暂无建议</div>
          <div v-for="item in recommendations" :key="item.id" class="p-5">
            <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
              <div>
                <div class="flex flex-wrap items-center gap-2">
                  <span class="badge" :class="severityClass(item.severity)">{{ item.severity }}</span>
                  <span class="badge badge-gray">{{ item.type }}</span>
                  <span class="badge" :class="statusClass(item.status)">{{ item.status }}</span>
                  <span v-if="item.requires_approval" class="badge badge-warning">需审批</span>
                </div>
                <h3 class="mt-3 font-medium text-gray-900 dark:text-white">{{ item.title }}</h3>
                <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ item.description }}</p>
                <p class="mt-2 text-xs text-gray-500 dark:text-dark-400">
                  {{ supplierName(item.supplier_id) }}
                  <span v-if="item.target_supplier_id"> -> {{ supplierName(item.target_supplier_id) }}</span>
                  · {{ item.reason_code }}
                </p>
                <p v-if="item.expected_impact" class="mt-2 text-xs text-emerald-600 dark:text-emerald-400">
                  {{ item.expected_impact }}
                </p>
              </div>
              <div class="flex flex-wrap gap-2">
                <button
                  v-for="status in statuses"
                  :key="status"
                  type="button"
                  class="btn btn-secondary px-3 py-1.5 text-xs"
                  :disabled="item.status === status"
                  @click="updateStatus(item.id, status)"
                >
                  {{ status }}
                </button>
              </div>
            </div>
          </div>
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import {
  generateActions,
  listActionRecommendations,
  listBalanceEvents,
  listHealthEvents,
  listPromotionEvents,
  listRateSnapshots,
  listSuppliers,
  updateActionRecommendationStatus,
  type ActionRecommendation,
  type BalanceEvent,
  type HealthEvent,
  type PromotionEvent,
  type RateSnapshot,
  type Supplier,
  type SupplierSignal
} from '@/api/admin/adminPlus'

const appStore = useAppStore()

const loading = ref(false)
const generating = ref(false)
const suppliers = ref<Supplier[]>([])
const rateSnapshots = ref<RateSnapshot[]>([])
const balanceEvents = ref<BalanceEvent[]>([])
const promotionEvents = ref<PromotionEvent[]>([])
const healthEvents = ref<HealthEvent[]>([])
const recommendations = ref<ActionRecommendation[]>([])

const statuses: ActionRecommendation['status'][] = ['acknowledged', 'approved', 'executed', 'rejected']

const openCount = computed(() => recommendations.value.filter((item) => item.status === 'open').length)
const criticalCount = computed(() => recommendations.value.filter((item) => item.severity === 'critical' && item.status === 'open').length)
const switchableCount = computed(() => suppliers.value.filter((supplier) =>
  ['active', 'candidate'].includes(supplier.runtime_status) && supplier.health_status === 'normal' && supplier.balance_cents > 0
).length)
const openSignalCount = computed(() =>
  balanceEvents.value.filter((event) => event.status === 'open').length +
  promotionEvents.value.filter((event) => event.status === 'open').length +
  healthEvents.value.filter((event) => event.status === 'open').length
)

function supplierName(id?: number | null): string {
  if (!id) return '全局'
  return suppliers.value.find((supplier) => supplier.id === id)?.name || `#${id}`
}

function severityClass(severity: ActionRecommendation['severity']): string {
  if (severity === 'critical') return 'badge-danger'
  if (severity === 'warning') return 'badge-warning'
  return 'badge-primary'
}

function statusClass(status: ActionRecommendation['status']): string {
  if (status === 'open') return 'badge-warning'
  if (['approved', 'executed', 'acknowledged'].includes(status)) return 'badge-success'
  if (status === 'rejected') return 'badge-danger'
  return 'badge-gray'
}

async function loadPage() {
  loading.value = true
  try {
    const [supplierResult, rateResult, balanceResult, promotionResult, healthResult, actionResult] = await Promise.all([
      listSuppliers(),
      listRateSnapshots({ limit: 200 }),
      listBalanceEvents({ limit: 100 }),
      listPromotionEvents({ limit: 100 }),
      listHealthEvents({ limit: 100 }),
      listActionRecommendations({ limit: 100 })
    ])
    suppliers.value = supplierResult.items
    rateSnapshots.value = rateResult.items
    balanceEvents.value = balanceResult.items
    promotionEvents.value = promotionResult.items
    healthEvents.value = healthResult.items
    recommendations.value = actionResult.items
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '加载动作建议失败')
  } finally {
    loading.value = false
  }
}

function supplierSignals(): SupplierSignal[] {
  return suppliers.value.map((supplier) => ({
    supplier_id: supplier.id,
    name: supplier.name,
    runtime_status: supplier.runtime_status,
    health_status: supplier.health_status,
    balance_cents: supplier.balance_cents,
    currency: supplier.balance_currency,
    effective_cost_cents: estimateCost(supplier.id)
  }))
}

function estimateCost(supplierID: number): number {
  const prices = rateSnapshots.value
    .filter((item) => item.supplier_id === supplierID)
    .map((item) => item.price_micros)
    .filter((value) => value > 0)
  if (prices.length === 0) return 0
  return Math.round(Math.min(...prices) / 10000)
}

async function generate() {
  generating.value = true
  try {
    const result = await generateActions({
      suppliers: supplierSignals(),
      balance_events: balanceEvents.value.filter((event) => event.status === 'open'),
      promotion_events: promotionEvents.value.filter((event) => event.status === 'open'),
      health_events: healthEvents.value.filter((event) => event.status === 'open'),
      min_profit_margin: 0.1
    })
    recommendations.value = result.items
    appStore.showSuccess('建议已生成')
    await loadPage()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '生成建议失败')
  } finally {
    generating.value = false
  }
}

async function updateStatus(id: number, status: ActionRecommendation['status']) {
  try {
    await updateActionRecommendationStatus(id, status)
    appStore.showSuccess('状态已更新')
    await loadPage()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '更新状态失败')
  }
}

onMounted(loadPage)
</script>
