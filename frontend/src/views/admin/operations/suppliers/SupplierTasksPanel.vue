<template>
  <div class="space-y-4">
    <div class="flex items-center justify-between gap-3">
      <div>
        <h2 class="text-base font-semibold text-gray-900 dark:text-gray-100">任务记录</h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">查看 Key 创建、分组同步和渠道检测的实际执行结果。</p>
      </div>
      <button type="button" class="btn btn-secondary" :disabled="loading" @click="loadJobs">
        <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
        刷新
      </button>
    </div>

    <div v-if="error" class="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-200">{{ error }}</div>

    <div class="overflow-x-auto border-y border-gray-200 dark:border-dark-700">
      <table class="min-w-full text-left text-sm">
        <thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-900/40 dark:text-dark-400">
          <tr><th class="px-3 py-3 font-medium">任务</th><th class="px-3 py-3 font-medium">状态</th><th class="px-3 py-3 font-medium">进度</th><th class="px-3 py-3 font-medium">结果</th><th class="px-3 py-3 font-medium">更新时间</th><th class="px-3 py-3 font-medium"><span class="sr-only">操作</span></th></tr>
        </thead>
        <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
          <template v-for="job in jobs" :key="job.id">
            <tr>
              <td class="px-3 py-3"><div class="font-medium text-gray-900 dark:text-gray-100">{{ jobTypeLabel(job.job_type) }}</div><div class="font-mono text-xs text-gray-500 dark:text-dark-400">#{{ job.id }}</div></td>
              <td class="px-3 py-3"><span class="badge" :class="statusClass(job.status)">{{ statusLabel(job.status) }}</span></td>
              <td class="px-3 py-3 text-gray-600 dark:text-dark-300">{{ job.succeeded_steps }}/{{ job.total_steps || 1 }}</td>
              <td class="max-w-md px-3 py-3 text-gray-600 dark:text-dark-300">{{ resultLabel(job) }}</td>
              <td class="whitespace-nowrap px-3 py-3 text-xs text-gray-500 dark:text-dark-400">{{ formatDate(job.updated_at) }}</td>
              <td class="px-3 py-3 text-right"><button type="button" class="btn btn-secondary btn-sm h-8 w-8 p-0" :disabled="detailLoadingID === job.id" :title="expandedJobID === job.id ? '收起分组结果' : '查看分组结果'" @click="toggleDetails(job)"><Icon :name="expandedJobID === job.id ? 'chevronUp' : 'chevronDown'" size="xs" :class="{ 'animate-spin': detailLoadingID === job.id }" /></button></td>
            </tr>
            <tr v-if="expandedJobID === job.id" class="bg-gray-50/70 dark:bg-dark-900/30">
              <td colspan="6" class="px-4 py-3">
                <div v-if="detailError" class="text-sm text-red-600 dark:text-red-300">{{ detailError }}</div>
                <div v-else class="grid gap-2 lg:grid-cols-2">
                  <div v-for="step in jobDetails[job.id]?.steps || []" :key="step.id" class="flex min-w-0 items-start justify-between gap-3 border-b border-gray-200 py-2 dark:border-dark-700">
                    <div class="min-w-0"><div class="truncate font-medium text-gray-900 dark:text-gray-100">{{ stepGroupLabel(step) }}</div><div class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ stepResultLabel(step) }}</div></div>
                    <span class="badge shrink-0" :class="statusClass(step.status)">{{ statusLabel(step.status) }}</span>
                  </div>
                  <div v-if="(jobDetails[job.id]?.steps || []).length === 0" class="text-sm text-gray-500 dark:text-dark-400">没有分组级明细</div>
                </div>
              </td>
            </tr>
          </template>
          <tr v-if="!loading && jobs.length === 0"><td colspan="6" class="px-3 py-12 text-center text-sm text-gray-500 dark:text-dark-400">还没有任务记录</td></tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { getSupplierProvisionJob, listSupplierProvisionJobs } from '@/api/admin/adminPlus'
import type { SupplierProvisionJob, SupplierProvisionJobType, SupplierProvisionStatus, SupplierProvisionStep } from '@/api/admin/adminPlus'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{ supplierId: number }>()
const jobs = ref<SupplierProvisionJob[]>([])
const loading = ref(false)
const error = ref('')
const expandedJobID = ref<number | null>(null)
const detailLoadingID = ref<number | null>(null)
const detailError = ref('')
const jobDetails = ref<Record<number, SupplierProvisionJob | undefined>>({})

async function loadJobs() {
  if (!props.supplierId) return
  loading.value = true
  error.value = ''
  try {
    const result = await listSupplierProvisionJobs({ supplier_id: props.supplierId, page: 1, page_size: 50 })
    jobs.value = result.items
  } catch (cause) {
    error.value = (cause as { message?: string }).message || '加载任务记录失败'
  } finally {
    loading.value = false
  }
}

function jobTypeLabel(type: SupplierProvisionJobType): string {
  if (type === 'provision_all_group_keys') return '创建缺失 Key'
  if (type === 'provision_group_key') return '创建单个 Key'
  if (type === 'sync_groups') return '同步分组'
  if (type === 'check_supplier_channels') return '检测渠道'
  return type
}

function statusLabel(status: SupplierProvisionStatus | 'skipped'): string {
  return ({ queued: '排队中', running: '执行中', succeeded: '已完成', partial_succeeded: '部分完成', retryable_failed: '等待重试', manual_required: '需人工处理', dead: '失败', cancelled: '已取消', skipped: '未执行' } as Record<string, string>)[status] || status
}

function statusClass(status: SupplierProvisionStatus | 'skipped'): string {
  if (status === 'succeeded') return 'badge-success'
  if (status === 'queued' || status === 'running') return 'badge-primary'
  if (status === 'partial_succeeded' || status === 'retryable_failed' || status === 'manual_required') return 'badge-warning'
  if (status === 'skipped') return 'badge-gray'
  return 'badge-danger'
}

async function toggleDetails(job: SupplierProvisionJob) {
  if (expandedJobID.value === job.id) {
    expandedJobID.value = null
    return
  }
  expandedJobID.value = job.id
  detailError.value = ''
  if (jobDetails.value[job.id]) return
  detailLoadingID.value = job.id
  try {
    jobDetails.value = { ...jobDetails.value, [job.id]: await getSupplierProvisionJob(job.id) }
  } catch (cause) {
    detailError.value = (cause as { message?: string }).message || '加载分组结果失败'
  } finally {
    detailLoadingID.value = null
  }
}

function stepGroupLabel(step: SupplierProvisionStep): string {
  const snapshot = step.request_snapshot || {}
  return String(snapshot.group_name || snapshot.external_group_id || (step.supplier_group_id ? `分组 #${step.supplier_group_id}` : '任务步骤'))
}

function stepResultLabel(step: SupplierProvisionStep): string {
  if (step.error_message) return step.error_message
  const result = step.result_snapshot || {}
  if (result.action === 'created') return 'Key 已创建并接入本地账号'
  if (result.action === 'reused') return '已复用第三方 Key 并接入本地账号'
  if (result.action === 'skipped') return '已有 Key，本地接入已检查'
  return step.status === 'succeeded' ? '执行成功' : '-'
}

function resultLabel(job: SupplierProvisionJob): string {
  if (job.error_message) return job.error_message
  const result = job.result_snapshot || {}
  if (job.job_type === 'provision_all_group_keys') {
    return `成功 ${Number(result.created || 0)}，复用 ${Number(result.reused || 0)}，失败 ${Number(result.failed || job.failed_steps || 0)}`
  }
  return job.status === 'succeeded' ? '任务执行成功' : '-'
}

function formatDate(value?: string): string {
  if (!value) return '-'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString('zh-CN', { hour12: false })
}

watch(() => props.supplierId, loadJobs)
onMounted(loadJobs)
</script>
