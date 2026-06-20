<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">账单对账</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
            导入供应商账单，与本地收入记录对账，确认成本、收入和利润率。
          </p>
        </div>
        <button type="button" class="btn btn-secondary" :disabled="loading" @click="loadPage">
          <Icon name="refresh" size="sm" />
          刷新
        </button>
      </section>

      <section class="grid gap-4 md:grid-cols-4">
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">供应商账单</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ billLines.length }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">成本</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ formatMoney(totalCostCents, defaultCurrency) }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">利润</p>
          <p class="mt-2 text-2xl font-semibold" :class="profitCents >= 0 ? 'text-emerald-600 dark:text-emerald-400' : 'text-rose-600 dark:text-rose-400'">
            {{ result ? formatMoney(profitCents, defaultCurrency) : '-' }}
          </p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">利润率</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ result ? formatPercent(result.summary.profit_margin) : '-' }}</p>
        </div>
      </section>

      <section class="grid gap-6 xl:grid-cols-[420px_minmax(0,1fr)]">
        <div class="space-y-6">
          <form class="card p-5" @submit.prevent="importBill">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">导入供应商账单</h2>
            <div class="mt-5 space-y-4">
              <label class="block">
                <span class="input-label">供应商</span>
                <select v-model.number="billForm.supplier_id" class="input" required>
                  <option :value="0" disabled>请选择</option>
                  <option v-for="supplier in suppliers" :key="supplier.id" :value="supplier.id">{{ supplier.name }}</option>
                </select>
              </label>
              <label class="block">
                <span class="input-label">请求 ID</span>
                <input v-model.trim="billForm.external_request_id" class="input" required />
              </label>
              <label class="block">
                <span class="input-label">模型</span>
                <input v-model.trim="billForm.model" class="input" required />
              </label>
              <div class="grid gap-4 sm:grid-cols-2">
                <label class="block">
                  <span class="input-label">成本</span>
                  <input v-model.number="billForm.cost_yuan" type="number" min="0" step="0.01" class="input" required />
                </label>
                <label class="block">
                  <span class="input-label">币种</span>
                  <input v-model.trim="billForm.currency" class="input" placeholder="CNY" />
                </label>
              </div>
              <button type="submit" class="btn btn-primary w-full" :disabled="importing">
                {{ importing ? '导入中...' : '导入账单' }}
              </button>
            </div>
          </form>

          <form class="card p-5" @submit.prevent="runRecon">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">本地收入记录</h2>
            <div class="mt-5 space-y-4">
              <label class="block">
                <span class="input-label">本地请求 ID</span>
                <input v-model.trim="usageForm.external_request_id" class="input" required />
              </label>
              <label class="block">
                <span class="input-label">模型</span>
                <input v-model.trim="usageForm.model" class="input" required />
              </label>
              <div class="grid gap-4 sm:grid-cols-2">
                <label class="block">
                  <span class="input-label">收入</span>
                  <input v-model.number="usageForm.revenue_yuan" type="number" min="0" step="0.01" class="input" required />
                </label>
                <label class="block">
                  <span class="input-label">币种</span>
                  <input v-model.trim="usageForm.currency" class="input" placeholder="CNY" />
                </label>
              </div>
              <button type="submit" class="btn btn-primary w-full" :disabled="reconciling || billLines.length === 0">
                {{ reconciling ? '对账中...' : '运行对账' }}
              </button>
            </div>
          </form>
        </div>

        <div class="space-y-6">
          <div class="card overflow-hidden">
            <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">供应商账单</h2>
            </div>
            <div class="overflow-x-auto">
              <table class="w-full min-w-[760px] divide-y divide-gray-200 dark:divide-dark-700">
                <thead class="bg-gray-50 dark:bg-dark-800">
                  <tr>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">供应商</th>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">请求 ID</th>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">模型</th>
                    <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">成本</th>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">时间</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-900">
                  <tr v-if="billLines.length === 0">
                    <td colspan="5" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-dark-400">暂无账单</td>
                  </tr>
                  <tr v-for="line in billLines" :key="line.id">
                    <td class="px-4 py-4 text-sm text-gray-900 dark:text-gray-100">{{ supplierName(line.supplier_id) }}</td>
                    <td class="px-4 py-4 font-mono text-xs text-gray-500 dark:text-dark-400">{{ line.external_request_id || '-' }}</td>
                    <td class="px-4 py-4 text-sm text-gray-900 dark:text-gray-100">{{ line.model }}</td>
                    <td class="px-4 py-4 text-right text-sm text-gray-900 dark:text-gray-100">{{ formatMoney(line.cost_cents, line.currency) }}</td>
                    <td class="px-4 py-4 text-sm text-gray-500 dark:text-dark-400">{{ formatDateTime(line.started_at) }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>

          <div v-if="result" class="card overflow-hidden">
            <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">对账结果</h2>
            </div>
            <div class="overflow-x-auto">
              <table class="w-full min-w-[820px] divide-y divide-gray-200 dark:divide-dark-700">
                <thead class="bg-gray-50 dark:bg-dark-800">
                  <tr>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">状态</th>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">请求 ID</th>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">模型</th>
                    <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">成本</th>
                    <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">收入</th>
                    <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">利润</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-900">
                  <tr v-for="line in result.lines" :key="`${line.status}-${line.external_request_id}-${line.supplier_bill_id}`">
                    <td class="px-4 py-4"><span class="badge" :class="line.status === 'matched' ? 'badge-success' : 'badge-warning'">{{ line.status }}</span></td>
                    <td class="px-4 py-4 font-mono text-xs text-gray-500 dark:text-dark-400">{{ line.external_request_id || '-' }}</td>
                    <td class="px-4 py-4 text-sm text-gray-900 dark:text-gray-100">{{ line.model }}</td>
                    <td class="px-4 py-4 text-right text-sm text-gray-900 dark:text-gray-100">{{ formatMoney(line.cost_cents, line.currency) }}</td>
                    <td class="px-4 py-4 text-right text-sm text-gray-900 dark:text-gray-100">{{ formatMoney(line.revenue_cents, line.currency) }}</td>
                    <td class="px-4 py-4 text-right text-sm" :class="line.profit_cents >= 0 ? 'text-emerald-600 dark:text-emerald-400' : 'text-rose-600 dark:text-rose-400'">
                      {{ formatMoney(line.profit_cents, line.currency) }}
                    </td>
                  </tr>
                </tbody>
              </table>
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
  importBillLines,
  listBillLines,
  listSuppliers,
  runReconciliation,
  type ReconciliationResult,
  type Supplier,
  type SupplierBillLine
} from '@/api/admin/adminPlus'

const appStore = useAppStore()

const loading = ref(false)
const importing = ref(false)
const reconciling = ref(false)
const suppliers = ref<Supplier[]>([])
const billLines = ref<SupplierBillLine[]>([])
const result = ref<ReconciliationResult | null>(null)

const billForm = reactive({
  supplier_id: 0,
  external_request_id: '',
  model: 'gpt-4o-mini',
  cost_yuan: 0,
  currency: 'CNY'
})

const usageForm = reactive({
  external_request_id: '',
  model: 'gpt-4o-mini',
  revenue_yuan: 0,
  currency: 'CNY'
})

const defaultCurrency = computed(() => billLines.value[0]?.currency || usageForm.currency || 'CNY')
const totalCostCents = computed(() => billLines.value.reduce((sum, line) => sum + line.cost_cents, 0))
const profitCents = computed(() => result.value?.summary.profit_cents || 0)

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

function formatPercent(value?: number | null): string {
  if (value === null || value === undefined) return '-'
  return `${(value * 100).toFixed(2)}%`
}

function formatDateTime(value: string): string {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '-' : date.toLocaleString()
}

function supplierName(id: number): string {
  return suppliers.value.find((supplier) => supplier.id === id)?.name || `#${id}`
}

async function loadPage() {
  loading.value = true
  try {
    const [supplierResult, billResult] = await Promise.all([
      listSuppliers(),
      listBillLines({ limit: 200 })
    ])
    suppliers.value = supplierResult.items
    billLines.value = billResult.items
    if (!billForm.supplier_id && suppliers.value[0]) {
      billForm.supplier_id = suppliers.value[0].id
    }
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '加载账单失败')
  } finally {
    loading.value = false
  }
}

async function importBill() {
  importing.value = true
  try {
    await importBillLines([{
      supplier_id: billForm.supplier_id,
      external_request_id: billForm.external_request_id,
      model: billForm.model,
      currency: billForm.currency || 'CNY',
      cost_cents: centsFromYuan(billForm.cost_yuan),
      input_tokens: 0,
      output_tokens: 0,
      started_at: new Date().toISOString()
    }])
    billForm.external_request_id = ''
    appStore.showSuccess('账单已导入')
    await loadPage()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '导入账单失败')
  } finally {
    importing.value = false
  }
}

async function runRecon() {
  reconciling.value = true
  try {
    result.value = await runReconciliation({
      supplier_bills: billLines.value,
      local_usages: [{
        id: Date.now(),
        external_request_id: usageForm.external_request_id,
        model: usageForm.model,
        currency: usageForm.currency || 'CNY',
        revenue_cents: centsFromYuan(usageForm.revenue_yuan),
        input_tokens: 0,
        output_tokens: 0,
        started_at: new Date().toISOString()
      }],
      time_tolerance_seconds: 300,
      cost_mismatch_cents: 1
    })
    appStore.showSuccess('对账完成')
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '对账失败')
  } finally {
    reconciling.value = false
  }
}

onMounted(loadPage)
</script>
