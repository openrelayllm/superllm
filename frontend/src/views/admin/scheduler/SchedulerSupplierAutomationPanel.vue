<template>
  <section class="card overflow-hidden">
    <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">供应商自动化</h2>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">基于调度 Step 和供应商事实数据检查自动化完成度，并可直接提交修复动作。</p>
        </div>
        <label class="flex items-center gap-2 text-sm text-gray-500 dark:text-dark-400">
          <input v-model="issuesOnly" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
          只看未完成
        </label>
      </div>
    </div>
    <div class="overflow-x-auto">
      <table class="w-full min-w-[1480px] divide-y divide-gray-200 dark:divide-dark-700">
        <thead class="bg-gray-50 dark:bg-dark-800">
          <tr>
            <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">供应商</th>
            <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">完成度</th>
            <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">会话</th>
            <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">余额</th>
            <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">分组/倍率</th>
            <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">账务</th>
            <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">渠道/调度</th>
            <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">建议</th>
            <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">操作</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-100 bg-white dark:divide-dark-700 dark:bg-dark-900">
          <tr v-if="visibleSuppliers.length === 0">
            <td colspan="9" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-dark-400">暂无供应商</td>
          </tr>
          <tr v-for="supplier in visibleSuppliers" :key="supplier.supplier_id">
            <td class="px-4 py-4">
              <p class="text-sm font-medium text-gray-900 dark:text-white">{{ supplier.supplier_name }}</p>
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ supplier.supplier_type }} · {{ supplier.runtime_status }}</p>
            </td>
            <td class="px-4 py-4">
              <div class="w-32">
                <div class="flex items-center justify-between text-xs text-gray-500 dark:text-dark-400">
                  <span>Checklist</span>
                  <span>{{ supplier.completion_percent }}%</span>
                </div>
                <div class="mt-2 h-2 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-800">
                  <div class="h-full bg-primary-500" :style="{ width: `${supplier.completion_percent}%` }"></div>
                </div>
              </div>
            </td>
            <td class="px-4 py-4"><StatusPill :value="supplier.session_status" /></td>
            <td class="px-4 py-4 text-sm text-gray-700 dark:text-gray-200">
              <StatusPill :value="supplier.balance_status" />
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ moneyLabel(supplier.balance_cents, supplier.balance_currency) }}</p>
            </td>
            <td class="px-4 py-4 text-sm text-gray-500 dark:text-dark-400">
              <StatusPill :value="supplier.group_status" />
              <span class="mx-1">/</span>
              <StatusPill :value="supplier.rate_status" />
            </td>
            <td class="px-4 py-4"><StatusPill :value="supplier.billing_status" /></td>
            <td class="px-4 py-4 text-sm text-gray-500 dark:text-dark-400">
              <StatusPill :value="supplier.channel_status" />
              <span class="mx-1">/</span>
              <StatusPill :value="supplier.schedule_status" />
              <div v-if="supplier.candidate_summary" class="mt-2 max-w-[260px] space-y-1 text-xs">
                <div class="flex min-w-0 flex-wrap items-center gap-1.5">
                  <span class="badge" :class="candidateStatusClass(supplier.candidate_summary.candidate_status)">
                    {{ candidateStatusLabel(supplier.candidate_summary.candidate_status) }}
                  </span>
                  <span class="text-gray-500 dark:text-dark-400">{{ candidateCountsLabel(supplier.candidate_summary) }}</span>
                </div>
                <p class="truncate text-gray-500 dark:text-dark-400" :title="candidateSummaryMeta(supplier)">
                  最低倍率 {{ candidateRateLabel(supplier.candidate_summary.lowest_effective_rate_multiplier) }} · {{ candidateCheckSourceLabel(supplier.candidate_summary.check_source) }}
                </p>
                <p v-if="supplier.candidate_summary.blocked_reason" class="truncate text-amber-700 dark:text-amber-300" :title="candidateReasonLabel(supplier.candidate_summary.blocked_reason)">
                  {{ candidateReasonLabel(supplier.candidate_summary.blocked_reason) }}
                </p>
              </div>
            </td>
            <td class="max-w-[220px] px-4 py-4 text-sm text-gray-500 dark:text-dark-400">
              <span class="block truncate" :title="supplier.recommended_action || supplier.last_error || ''">{{ supplier.recommended_action || supplier.last_error || '-' }}</span>
            </td>
            <td class="px-4 py-4">
              <div class="flex max-w-[300px] flex-wrap gap-2">
                <button type="button" class="btn btn-primary btn-sm" :disabled="isRunning(supplier, 'full_collect')" @click="emit('action', supplier, 'full_collect')">一键采集</button>
                <button type="button" class="btn btn-secondary btn-sm" :disabled="isRunning(supplier, 'login_session')" @click="emit('action', supplier, 'login_session')">直登</button>
                <button type="button" class="btn btn-secondary btn-sm" :disabled="isRunning(supplier, 'fetch_balance')" @click="emit('action', supplier, 'fetch_balance')">余额</button>
                <button type="button" class="btn btn-secondary btn-sm" :disabled="isRunning(supplier, 'fetch_groups')" @click="emit('action', supplier, 'fetch_groups')">分组</button>
                <button type="button" class="btn btn-secondary btn-sm" :disabled="isRunning(supplier, 'fetch_rates')" @click="emit('action', supplier, 'fetch_rates')">倍率</button>
                <button type="button" class="btn btn-secondary btn-sm" :disabled="isRunning(supplier, 'reconcile_costs')" @click="emit('action', supplier, 'reconcile_costs')">对账</button>
                <button type="button" class="btn btn-secondary btn-sm" :disabled="isRunning(supplier, 'check_channels')" @click="emit('action', supplier, 'check_channels')">检测</button>
                <button type="button" class="btn btn-secondary btn-sm" @click="emit('checklist', supplier)">Checklist</button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import type { SchedulerSupplierStatus } from '@/api/admin/adminPlus'
import SchedulerStatusPill from './SchedulerStatusPill.vue'
import {
  candidateCheckSourceLabel,
  candidateCountsLabel,
  candidateRateLabel,
  candidateReasonLabel,
  candidateStatusClass,
  candidateStatusLabel,
  moneyLabel
} from './presentation'
import { supplierActionKey, type SupplierAutomationAction } from './supplierAutomation'

const props = defineProps<{
  suppliers: SchedulerSupplierStatus[]
  runningActionKey: string | null
}>()

const emit = defineEmits<{
  (event: 'action', supplier: SchedulerSupplierStatus, action: SupplierAutomationAction): void
  (event: 'checklist', supplier: SchedulerSupplierStatus): void
}>()

const StatusPill = SchedulerStatusPill
const issuesOnly = ref(false)

const visibleSuppliers = computed(() => {
  if (!issuesOnly.value) return props.suppliers
  return props.suppliers.filter((supplier) => supplier.completion_percent < 100 || Boolean(supplier.recommended_action || supplier.last_error))
})

function isRunning(supplier: SchedulerSupplierStatus, action: SupplierAutomationAction): boolean {
  return props.runningActionKey === supplierActionKey(supplier.supplier_id, action)
}

function candidateSummaryMeta(supplier: SchedulerSupplierStatus): string {
  const summary = supplier.candidate_summary
  if (!summary) return ''
  return `最低倍率 ${candidateRateLabel(summary.lowest_effective_rate_multiplier)} · ${candidateCheckSourceLabel(summary.check_source)}`
}
</script>
