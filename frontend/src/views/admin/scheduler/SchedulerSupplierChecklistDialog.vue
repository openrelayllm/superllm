<template>
  <BaseDialog :show="show" :title="title" width="extra-wide" @close="emit('close')">
    <div v-if="loading" class="py-12 text-center text-sm text-gray-500 dark:text-dark-400">加载 Checklist...</div>
    <div v-else-if="checklist" class="space-y-5">
      <div class="flex flex-col gap-3 rounded-lg border border-gray-200 p-4 dark:border-dark-700 md:flex-row md:items-center md:justify-between">
        <div>
          <p class="text-sm font-medium text-gray-900 dark:text-white">{{ checklist.supplier_name }}</p>
          <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ checklist.supplier_type }} · {{ checklist.recommended_action || '暂无推荐动作' }}</p>
        </div>
        <div class="w-full md:w-56">
          <div class="flex items-center justify-between text-xs text-gray-500 dark:text-dark-400">
            <span>完成度</span>
            <span>{{ checklist.completion_percent }}%</span>
          </div>
          <div class="mt-2 h-2 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-800">
            <div class="h-full bg-primary-500" :style="{ width: `${checklist.completion_percent}%` }"></div>
          </div>
        </div>
      </div>

      <div v-if="checklist.candidate_summary" class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <p class="text-sm font-medium text-gray-900 dark:text-white">候选池</p>
              <span class="badge" :class="candidateStatusClass(checklist.candidate_summary.candidate_status)">
                {{ candidateStatusLabel(checklist.candidate_summary.candidate_status) }}
              </span>
            </div>
            <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
              {{ candidateCountsLabel(checklist.candidate_summary) }}
            </p>
          </div>
          <div class="grid w-full gap-3 text-xs text-gray-600 dark:text-dark-300 sm:w-auto sm:grid-cols-3">
            <div>
              <span class="block text-gray-500 dark:text-dark-400">最低倍率</span>
              <span class="font-medium text-gray-900 dark:text-white">{{ candidateRateLabel(checklist.candidate_summary.lowest_effective_rate_multiplier) }}</span>
            </div>
            <div>
              <span class="block text-gray-500 dark:text-dark-400">来源</span>
              <span class="font-medium text-gray-900 dark:text-white">{{ candidateCheckSourceLabel(checklist.candidate_summary.check_source) }}</span>
            </div>
            <div>
              <span class="block text-gray-500 dark:text-dark-400">阻断</span>
              <span class="font-medium text-gray-900 dark:text-white">{{ candidateReasonLabel(checklist.candidate_summary.blocked_reason) }}</span>
            </div>
          </div>
        </div>
      </div>

      <div class="grid gap-3 md:grid-cols-2">
        <div v-for="item in checklist.items" :key="item.key" class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
          <div class="flex items-start justify-between gap-3">
            <div>
              <p class="text-sm font-medium text-gray-900 dark:text-white">{{ item.label }}</p>
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ item.description }}</p>
            </div>
            <span class="badge shrink-0" :class="statusBadgeClass(item.status)">{{ statusValueLabel(item.status) }}</span>
          </div>
          <dl class="mt-3 space-y-2 text-xs">
            <div class="flex gap-3">
              <dt class="w-20 shrink-0 text-gray-500 dark:text-dark-400">证据</dt>
              <dd class="min-w-0 truncate text-gray-700 dark:text-gray-200" :title="item.evidence || ''">{{ item.evidence || '-' }}</dd>
            </div>
            <div class="flex gap-3">
              <dt class="w-20 shrink-0 text-gray-500 dark:text-dark-400">建议</dt>
              <dd class="text-gray-700 dark:text-gray-200">{{ item.recommended_action || '-' }}</dd>
            </div>
            <div class="flex gap-3">
              <dt class="w-20 shrink-0 text-gray-500 dark:text-dark-400">检查时间</dt>
              <dd class="text-gray-700 dark:text-gray-200">{{ formatDateTime(item.last_checked_at) || '-' }}</dd>
            </div>
          </dl>
          <div class="mt-4 flex items-center justify-between gap-3 border-t border-gray-100 pt-3 dark:border-dark-700">
            <p class="text-xs text-gray-500 dark:text-dark-400">{{ actionHint(item.key) }}</p>
            <button
              v-if="itemAction(item.key)"
              type="button"
              class="btn btn-secondary btn-sm shrink-0"
              :disabled="itemBusy(item.key, item.status)"
              @click="emit('action', item.key)"
            >
              {{ actionButtonLabel(item.key, item.status) }}
            </button>
            <button v-else type="button" class="btn btn-secondary btn-sm shrink-0" disabled>
              {{ checklistManualActionLabel(item.key) }}
            </button>
          </div>
        </div>
      </div>
    </div>
    <div v-else class="py-12 text-center text-sm text-gray-500 dark:text-dark-400">请选择供应商。</div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import type { SchedulerSupplierChecklist } from '@/api/admin/adminPlus'
import {
  candidateCheckSourceLabel,
  candidateCountsLabel,
  candidateRateLabel,
  candidateReasonLabel,
  candidateStatusClass,
  candidateStatusLabel,
  formatDateTime,
  statusBadgeClass,
  statusValueLabel
} from './presentation'
import { checklistActionForKey, checklistManualActionLabel, supplierActionKey, supplierActionLabel } from './supplierAutomation'

const props = defineProps<{
  show: boolean
  checklist: SchedulerSupplierChecklist | null
  loading: boolean
  runningActionKey: string | null
}>()

const emit = defineEmits<{
  (event: 'close'): void
  (event: 'action', itemKey: string): void
}>()

const title = computed(() => (props.checklist ? `供应商 Checklist - ${props.checklist.supplier_name}` : '供应商 Checklist'))

function itemAction(key: string) {
  return checklistActionForKey(key)
}

function itemBusy(key: string, status: string): boolean {
  const action = checklistActionForKey(key)
  if (!action || !props.checklist) return true
  return props.runningActionKey === supplierActionKey(props.checklist.supplier_id, action) || ['queued', 'running'].includes(status)
}

function actionButtonLabel(key: string, status: string): string {
  if (['queued', 'running'].includes(status)) return '运行中'
  const action = checklistActionForKey(key)
  if (!action) return checklistManualActionLabel(key)
  if (['ready', 'ok'].includes(status)) return '重新执行'
  return supplierActionLabel(action)
}

function actionHint(key: string): string {
  return checklistActionForKey(key) ? '提交真实调度任务' : '需要在供应商管理中处理'
}
</script>
