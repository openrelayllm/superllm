<template>
  <BaseDialog :show="show" :title="title" width="full" @close="emit('close')">
    <div v-if="loading" class="py-12 text-center text-sm text-gray-500 dark:text-dark-400">加载运行详情...</div>
    <div v-else-if="detail" class="space-y-5">
      <div class="grid gap-3 md:grid-cols-4">
        <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
          <p class="text-xs text-gray-500 dark:text-dark-400">状态</p>
          <span class="badge mt-2" :class="runStatusClass(detail.run.status)">{{ runStatusLabel(detail.run.status) }}</span>
        </div>
        <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
          <p class="text-xs text-gray-500 dark:text-dark-400">任务</p>
          <p class="mt-2 text-sm font-medium text-gray-900 dark:text-white">{{ taskLabel(detail.run.task_type) }}</p>
        </div>
        <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
          <p class="text-xs text-gray-500 dark:text-dark-400">Step</p>
          <p class="mt-2 text-sm font-medium text-gray-900 dark:text-white">{{ detail.run.succeeded_steps }}/{{ detail.run.total_steps }} 成功，{{ detail.run.failed_steps }} 失败</p>
        </div>
        <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
          <p class="text-xs text-gray-500 dark:text-dark-400">请求时间</p>
          <p class="mt-2 text-sm font-medium text-gray-900 dark:text-white">{{ formatDateTime(detail.run.requested_at) || '-' }}</p>
        </div>
      </div>

      <div class="overflow-x-auto rounded-lg border border-gray-200 dark:border-dark-700">
        <table class="w-full min-w-[1320px] divide-y divide-gray-200 dark:divide-dark-700">
          <thead class="bg-gray-50 dark:bg-dark-800">
            <tr>
              <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">Step</th>
              <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">供应商</th>
              <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">任务</th>
              <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">状态</th>
              <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">Attempt</th>
              <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">结果</th>
              <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">时间</th>
              <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">错误/原因</th>
              <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">下次重试</th>
              <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">操作</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100 bg-white dark:divide-dark-700 dark:bg-dark-900">
            <tr v-if="detail.steps.length === 0">
              <td colspan="10" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-dark-400">暂无 step 明细</td>
            </tr>
            <tr v-for="step in detail.steps" :key="step.id">
              <td class="px-4 py-3 font-mono text-xs text-gray-500 dark:text-dark-400">{{ step.id }}</td>
              <td class="px-4 py-3 text-sm text-gray-900 dark:text-gray-100">{{ step.supplier_name || step.supplier_id }}</td>
              <td class="px-4 py-3 text-sm text-gray-500 dark:text-dark-400">{{ taskLabel(step.task_type) }}</td>
              <td class="px-4 py-3"><span class="badge" :class="runStatusClass(step.status)">{{ runStatusLabel(step.status) }}</span></td>
              <td class="px-4 py-3 text-sm text-gray-500 dark:text-dark-400">{{ step.attempts }}/{{ step.max_attempts }}</td>
              <td class="px-4 py-3 text-sm text-gray-500 dark:text-dark-400">{{ step.result_count }}</td>
              <td class="px-4 py-3 text-sm text-gray-500 dark:text-dark-400">
                <div>{{ formatDateTime(step.started_at) || '-' }}</div>
                <div v-if="step.finished_at" class="mt-1 text-xs text-gray-400 dark:text-dark-500">完成 {{ formatDateTime(step.finished_at) }}</div>
              </td>
              <td class="max-w-[280px] px-4 py-3 text-sm text-gray-500 dark:text-dark-400">
                <button
                  v-if="step.reason"
                  type="button"
                  class="block max-w-full text-left hover:text-gray-900 dark:hover:text-gray-100"
                  @click="selectedStep = step"
                >
                  <span class="block truncate" :title="step.reason">{{ reasonSummary(step.reason) }}</span>
                  <span class="mt-1 block text-xs font-medium text-blue-700 dark:text-blue-300">查看详情</span>
                </button>
                <span v-else>-</span>
              </td>
              <td class="px-4 py-3 text-sm text-gray-500 dark:text-dark-400">{{ formatDateTime(step.next_attempt_at) || '-' }}</td>
              <td class="px-4 py-3">
                <div class="flex flex-wrap gap-2">
                  <button
                    type="button"
                    class="btn btn-secondary btn-sm"
                    :disabled="retryingStepId === step.id || !stepRetryable(step.status)"
                    @click="emit('retry-step', step)"
                  >
                    重试
                  </button>
                  <button
                    type="button"
                    class="btn btn-secondary btn-sm"
                    :disabled="cancellingStepId === step.id || !stepCancellable(step.status)"
                    @click="emit('cancel-step', step)"
                  >
                    取消
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
    <div v-else class="py-12 text-center text-sm text-gray-500 dark:text-dark-400">请选择一条运行记录。</div>
  </BaseDialog>

  <BaseDialog
    :show="Boolean(selectedStep)"
    :title="selectedStep ? `错误详情 - Step ${selectedStep.id}` : '错误详情'"
    width="wide"
    :z-index="70"
    @close="selectedStep = null"
  >
    <div v-if="selectedStep" class="space-y-5">
      <div class="grid gap-3 md:grid-cols-3">
        <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
          <p class="text-xs text-gray-500 dark:text-dark-400">供应商</p>
          <p class="mt-2 text-sm font-medium text-gray-900 dark:text-white">{{ selectedStep.supplier_name || selectedStep.supplier_id }}</p>
        </div>
        <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
          <p class="text-xs text-gray-500 dark:text-dark-400">任务</p>
          <p class="mt-2 text-sm font-medium text-gray-900 dark:text-white">{{ taskLabel(selectedStep.task_type) }}</p>
        </div>
        <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
          <p class="text-xs text-gray-500 dark:text-dark-400">状态</p>
          <span class="badge mt-2" :class="runStatusClass(selectedStep.status)">{{ runStatusLabel(selectedStep.status) }}</span>
        </div>
      </div>

      <dl class="grid gap-3 md:grid-cols-2">
        <div v-for="row in selectedReasonRows" :key="row.label" class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
          <dt class="text-xs text-gray-500 dark:text-dark-400">{{ row.label }}</dt>
          <dd class="mt-2 break-words text-sm font-medium text-gray-900 dark:text-gray-100">{{ row.value || '-' }}</dd>
        </div>
      </dl>

      <div class="rounded-lg border border-gray-200 dark:border-dark-700">
        <div class="border-b border-gray-100 px-3 py-2 dark:border-dark-700">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">操作日志</p>
        </div>
        <div v-if="selectedOperationLogs.length === 0" class="px-3 py-4 text-sm text-gray-500 dark:text-dark-400">暂无 attempt 日志</div>
        <div v-else class="divide-y divide-gray-100 dark:divide-dark-700">
          <div v-for="log in selectedOperationLogs" :key="log.id" class="grid gap-3 px-3 py-3 md:grid-cols-[120px_minmax(0,1fr)]">
            <div class="space-y-1 text-xs text-gray-500 dark:text-dark-400">
              <p class="font-mono">#{{ log.attempt_no }}</p>
              <p>开始 {{ formatDateTime(log.started_at) || '-' }}</p>
              <p>完成 {{ formatDateTime(log.finished_at) || '-' }}</p>
              <p>{{ log.duration_ms }} ms</p>
            </div>
            <div class="min-w-0">
              <div class="flex flex-wrap items-center gap-2">
                <span class="badge" :class="runStatusClass(log.status)">{{ runStatusLabel(log.status) }}</span>
                <span v-if="log.error_code" class="font-mono text-xs text-rose-600 dark:text-rose-300">{{ log.error_code }}</span>
              </div>
              <p v-if="log.error_message" class="mt-2 break-words text-sm font-medium text-gray-900 dark:text-gray-100">{{ log.error_message }}</p>
              <pre v-if="log.response_snapshot" class="mt-2 max-h-40 overflow-auto whitespace-pre-wrap break-words rounded bg-gray-50 p-3 text-xs text-gray-700 dark:bg-dark-800 dark:text-dark-200">{{ formatSnapshot(log.response_snapshot) }}</pre>
            </div>
          </div>
        </div>
      </div>

      <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
        <p class="text-xs text-gray-500 dark:text-dark-400">完整错误</p>
        <pre class="mt-2 max-h-72 overflow-auto whitespace-pre-wrap break-words rounded bg-gray-50 p-3 text-xs text-gray-700 dark:bg-dark-800 dark:text-dark-200">{{ selectedRawReason }}</pre>
      </div>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import type { SchedulerRunDetail, SchedulerStepRecord } from '@/api/admin/adminPlus'
import { formatDateTime, runStatusClass, runStatusLabel, stepCancellable, stepRetryable, taskLabel } from './presentation'

const props = defineProps<{
  show: boolean
  detail: SchedulerRunDetail | null
  loading: boolean
  retryingStepId: number | null
  cancellingStepId: number | null
}>()

const emit = defineEmits<{
  (event: 'close'): void
  (event: 'retry-step', step: SchedulerStepRecord): void
  (event: 'cancel-step', step: SchedulerStepRecord): void
  (event: 'refresh'): void
}>()

const title = computed(() => (props.detail ? `运行详情 - ${props.detail.run.id}` : '运行详情'))
const selectedStep = ref<SchedulerStepRecord | null>(null)

interface StepFailureReason {
  stage?: string
  code?: string
  message?: string
  action?: string
  outcome?: string
  login_code?: string
  login_message?: string
  suggestion?: string
  raw_error?: string
  metadata?: Record<string, string>
}

const selectedFailure = computed(() => parseReason(selectedStep.value?.reason))
const selectedRawReason = computed(() => selectedStep.value?.reason || '-')
const selectedOperationLogs = computed(() => selectedStep.value?.operation_logs || [])
const selectedReasonRows = computed(() => {
  const step = selectedStep.value
  const reason = selectedFailure.value
  if (!step) return []
  return [
    { label: '阶段', value: stageLabel(reason.stage) },
    { label: '动作', value: actionLabel(reason.action) },
    { label: '结果', value: outcomeLabel(reason.outcome) },
    { label: '错误码', value: firstText(reason.login_code, reason.code, codeFromText(step.reason || '')) },
    { label: '错误信息', value: firstText(reason.login_message, reason.message, plainReason(step.reason || '')) },
    { label: '建议操作', value: reason.suggestion || suggestionFromCode(firstText(reason.login_code, reason.code, codeFromText(step.reason || ''))) },
    { label: '上游诊断', value: metadataSummary(reason.metadata) },
    { label: 'Attempt', value: `${step.attempts}/${step.max_attempts}` },
    { label: '下次重试', value: formatDateTime(step.next_attempt_at) || '-' }
  ]
})

let refreshTimer: ReturnType<typeof setInterval> | null = null

function clearRefreshTimer() {
  if (!refreshTimer) return
  clearInterval(refreshTimer)
  refreshTimer = null
}

function hasPendingStep(detail: SchedulerRunDetail): boolean {
  return detail.run.status === 'queued' || detail.run.status === 'running' || detail.steps.some((step) => step.status === 'queued' || step.status === 'running')
}

function parseReason(reason?: string): StepFailureReason {
  if (!reason) return {}
  try {
    const parsed = JSON.parse(reason)
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      return parsed as StepFailureReason
    }
  } catch {
    // Keep old plain-text scheduler reasons readable.
  }
  return {}
}

function reasonSummary(reason?: string): string {
  const parsed = parseReason(reason)
  return firstText(parsed.login_code, parsed.code, parsed.login_message, parsed.message, codeFromText(reason || ''), plainReason(reason || ''), '-')
}

function plainReason(reason: string): string {
  const parsed = parseReason(reason)
  return firstText(parsed.login_message, parsed.message, parsed.raw_error, reason)
}

function codeFromText(reason: string): string {
  const upper = reason.toUpperCase()
  const knownCodes = [
    'SUPPLIER_SESSION_NOT_FOUND',
    'SUPPLIER_SESSION_EXPIRED',
    'SUPPLIER_SESSION_DECRYPT_FAILED',
    'SUPPLIER_SESSION_PERMISSION_DENIED',
    'SUPPLIER_DIRECT_LOGIN_CREDENTIAL_REQUIRED',
    'LOGIN_CREDENTIAL_INVALID',
    'LOGIN_CAPTCHA_REQUIRED',
    'LOGIN_MFA_REQUIRED',
    'BROWSER_FALLBACK_REQUIRED',
    'BROWSER_CHALLENGE_REQUIRED',
    'PASSWORD_LOGIN_DISABLED'
  ]
  return knownCodes.find((code) => upper.includes(code)) || ''
}

function stageLabel(value?: string): string {
  return {
    session_precheck: '会话预检',
    session_refresh: '自动登录',
    session_refresh_after_sync: '采集后会话刷新',
    supplier_groups_sync: '分组同步',
    supplier_rates_sync: '倍率同步',
    supplier_balance_sync: '余额同步',
    supplier_announcements_sync: '公告同步',
    supplier_usage_costs_sync: '用量对账',
    supplier_health_sync: '健康检测',
    supplier_channel_check: '渠道检测'
  }[value || ''] || value || '-'
}

function actionLabel(value?: string): string {
  return {
    direct_login: '自动登录',
    sync_groups: '同步分组',
    sync_rates: '同步倍率',
    sync_balance: '同步余额',
    sync_announcements: '同步公告',
    sync_usage_costs: '同步用量',
    sync_health: '健康检测',
    check_channels: '检测渠道',
    sync: '同步'
  }[value || ''] || value || '-'
}

function outcomeLabel(value?: string): string {
  return {
    skipped: '已跳过',
    failed: '失败',
    manual_required: '需人工处理'
  }[value || ''] || value || '-'
}

function suggestionFromCode(code?: string): string {
  return {
    SUPPLIER_SESSION_NOT_FOUND: '当前没有可用会话，请配置登录凭据后重试，或使用插件采集会话。',
    SUPPLIER_SESSION_EXPIRED: '当前会话已过期，请重新登录或使用插件刷新会话。',
    SUPPLIER_SESSION_DECRYPT_FAILED: '会话解密失败，请重新一键登录或使用插件采集会话。',
    SUPPLIER_SESSION_PROBE_FAILED: '供应商接口超时或不可达，请检查供应商地址、网络出口和前置防护后重试。',
    SUPPLIER_SESSION_PROBE_HTML: '供应商 profile 接口返回 HTML，通常是 Cloudflare/Nginx/风控页面，请检查前置层策略。',
    SUPPLIER_SESSION_PROBE_BAD_STATUS: '供应商 profile 接口返回非成功状态，请检查会话权限和供应商接口。',
    SUPPLIER_SESSION_PROFILE_INVALID: '供应商 profile 返回结构异常，请检查供应商程序版本和接口兼容性。',
    SUPPLIER_DIRECT_LOGIN_CREDENTIAL_REQUIRED: '补充供应商登录账号密码或 access token 后重试。',
    SUPPLIER_DIRECT_LOGIN_UPSTREAM_HTML: '供应商登录接口返回 HTML，通常是前置层或风控页面，请改用浏览器会话或调整防护策略。',
    SUPPLIER_DIRECT_LOGIN_UPSTREAM_ORIGIN_ERROR: '供应商前置层或源站返回异常，请检查 Cloudflare/Nginx/源站健康。',
    LOGIN_CREDENTIAL_INVALID: '供应商登录凭据无效，请更新账号密码或 token 后重试。',
    LOGIN_CAPTCHA_REQUIRED: '供应商要求验证码，请使用一键登录或插件采集会话后重试。',
    LOGIN_MFA_REQUIRED: '供应商要求二次验证，请人工完成登录或使用插件采集会话。',
    BROWSER_FALLBACK_REQUIRED: '供应商要求浏览器验证，请使用一键登录或插件采集会话。',
    BROWSER_CHALLENGE_REQUIRED: '供应商要求浏览器验证，请使用一键登录或插件采集会话。',
    PASSWORD_LOGIN_DISABLED: '供应商关闭密码登录，请改用 token 或插件采集会话。'
  }[code || ''] || '查看供应商地址、登录凭据和上游防护策略后重试。'
}

function metadataSummary(metadata?: Record<string, string>): string {
  if (!metadata) return ''
  return Object.entries(metadata)
    .filter(([, value]) => value)
    .map(([key, value]) => `${key}: ${value}`)
    .join(' · ')
}

function formatSnapshot(value?: Record<string, unknown>): string {
  if (!value) return ''
  try {
    return JSON.stringify(value, null, 2)
  } catch {
    return String(value)
  }
}

function firstText(...values: Array<string | undefined | null>): string {
  return values.find((value) => typeof value === 'string' && value.trim())?.trim() || ''
}

watch(
  () => [props.show, props.detail?.run.status, props.detail?.steps.map((step) => step.status).join('|')],
  () => {
    clearRefreshTimer()
    if (!props.show) selectedStep.value = null
    if (!props.show || !props.detail || !hasPendingStep(props.detail)) return
    refreshTimer = setInterval(() => emit('refresh'), 2000)
  },
  { immediate: true }
)

onBeforeUnmount(clearRefreshTimer)
</script>
