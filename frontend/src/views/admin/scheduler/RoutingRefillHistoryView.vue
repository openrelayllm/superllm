<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">补池影响历史</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
            按补池运行查看本地分组容量变化、最低倍率候选和受影响用户 Key 摘要。
          </p>
        </div>
        <div class="flex flex-wrap gap-2">
          <RouterLink to="/admin/scheduler" class="btn btn-secondary">
            <Icon name="arrowLeft" size="sm" />
            调度中心
          </RouterLink>
          <button type="button" class="btn btn-secondary" :disabled="loading" @click="loadPage">
            <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
            刷新
          </button>
        </div>
      </section>

      <section class="grid gap-4 md:grid-cols-4">
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">当前结果</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ pagination.total }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">已补入</p>
          <p class="mt-2 text-2xl font-semibold text-emerald-600 dark:text-emerald-300">{{ visibleStats.succeeded }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">已跳过</p>
          <p class="mt-2 text-2xl font-semibold text-amber-600 dark:text-amber-300">{{ visibleStats.skipped }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">失败</p>
          <p class="mt-2 text-2xl font-semibold text-rose-600 dark:text-rose-300">{{ visibleStats.failed }}</p>
        </div>
      </section>

      <section class="card p-5">
        <div class="grid gap-4 lg:grid-cols-[1fr_1fr_auto_auto] lg:items-end">
          <label class="block">
            <span class="input-label">本地分组</span>
            <select v-model.number="filters.local_group_id" class="input" @change="resetAndLoad">
              <option :value="0">全部分组</option>
              <option v-for="group in localGroups" :key="group.id" :value="group.id">
                {{ group.name }} · {{ group.platform || '-' }}
              </option>
            </select>
          </label>
          <label class="block">
            <span class="input-label">运行状态</span>
            <select v-model="filters.status" class="input" @change="resetAndLoad">
              <option value="">全部状态</option>
              <option value="previewed">已预览</option>
              <option value="succeeded">已补入</option>
              <option value="skipped">已跳过</option>
              <option value="failed">失败</option>
            </select>
          </label>
          <button type="button" class="btn btn-primary" :disabled="loading" @click="resetAndLoad">
            <Icon name="search" size="sm" />
            查询
          </button>
          <button type="button" class="btn btn-secondary" :disabled="loading" @click="clearFilters">
            清除
          </button>
        </div>
      </section>

      <section class="grid gap-6 xl:grid-cols-[minmax(0,1.35fr)_minmax(380px,0.65fr)]">
        <div class="card overflow-hidden">
          <div class="flex flex-wrap items-center justify-between gap-2 border-b border-gray-100 px-5 py-4 dark:border-dark-700">
            <div>
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">运行时间线</h2>
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">按运行时间倒序展示 dry-run、真实补入、跳过和失败记录。</p>
            </div>
            <span class="badge badge-gray">{{ runs.length }}/{{ pagination.total }}</span>
          </div>
          <div class="overflow-x-auto">
            <table class="w-full min-w-[1120px] divide-y divide-gray-200 dark:divide-dark-700">
              <thead class="bg-gray-50 dark:bg-dark-800">
                <tr>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">时间</th>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">状态</th>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">本地分组</th>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">容量</th>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">候选</th>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">原因</th>
                  <th class="px-4 py-3 text-right text-xs font-medium uppercase text-gray-500 dark:text-dark-400">操作</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-900">
                <tr v-if="runs.length === 0">
                  <td colspan="7" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-dark-400">
                    {{ loading ? '加载中...' : '暂无补池运行记录' }}
                  </td>
                </tr>
                <tr
                  v-for="run in runs"
                  v-else
                  :key="run.id"
                  class="align-top hover:bg-gray-50 dark:hover:bg-dark-800"
                  :class="{ 'bg-primary-50/60 dark:bg-primary-950/20': selectedRunID === run.id }"
                >
                  <td class="whitespace-nowrap px-4 py-4 text-sm text-gray-500 dark:text-dark-400">
                    {{ formatTime(run.created_at) }}
                    <div class="mt-1 text-xs text-gray-400 dark:text-dark-500">{{ run.trigger_type || '-' }}</div>
                  </td>
                  <td class="px-4 py-4">
                    <span class="badge" :class="routingRefillRunStatusClass(run.status)">
                      {{ routingRefillRunStatusLabel(run.status) }}
                    </span>
                    <span v-if="run.dry_run" class="badge badge-gray ml-1">dry-run</span>
                  </td>
                  <td class="px-4 py-4 text-sm text-gray-900 dark:text-white">
                    <div class="font-medium">{{ run.local_group_name || `分组 #${run.local_group_id}` }}</div>
                    <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ run.platform || '-' }} · #{{ run.local_group_id }}</div>
                  </td>
                  <td class="px-4 py-4 text-sm text-gray-700 dark:text-dark-200">
                    <div>可调度 {{ run.before_schedulable_accounts }} -> {{ run.after_schedulable_accounts }}</div>
                    <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                      用户 Key {{ run.before_active_api_key_count }} -> {{ run.after_active_api_key_count }}
                    </div>
                  </td>
                  <td class="px-4 py-4 text-sm text-gray-700 dark:text-dark-200">
                    <div>{{ candidateLabel(run) }}</div>
                    <div v-if="run.selected_effective_rate_multiplier" class="mt-1 text-xs text-emerald-600 dark:text-emerald-300">
                      {{ routingRefillMultiplierLabel(run.selected_effective_rate_multiplier) }}
                    </div>
                  </td>
                  <td class="max-w-xs px-4 py-4 text-sm text-gray-700 dark:text-dark-200">
                    <div class="truncate" :title="runReasonLabel(run)">{{ runReasonLabel(run) }}</div>
                    <div v-if="run.error_code" class="mt-1 font-mono text-xs text-rose-600 dark:text-rose-300">{{ run.error_code }}</div>
                  </td>
                  <td class="px-4 py-4 text-right">
                    <button type="button" class="btn btn-secondary btn-sm" @click="selectRun(run)">
                      查看影响
                    </button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <Pagination
            v-if="pagination.total > 0"
            :page="pagination.page"
            :total="pagination.total"
            :page-size="pagination.page_size"
            :show-jump="true"
            @update:page="handlePageChange"
            @update:pageSize="handlePageSizeChange"
          />
        </div>

        <aside class="card p-5">
          <div v-if="selectedRun" class="space-y-4">
            <div>
              <div class="flex flex-wrap items-center gap-2">
                <span class="badge" :class="routingRefillRunStatusClass(selectedRun.status)">
                  {{ routingRefillRunStatusLabel(selectedRun.status) }}
                </span>
                <span class="badge badge-gray">{{ selectedRun.trigger_type || '-' }}</span>
              </div>
              <h2 class="mt-3 text-lg font-semibold text-gray-900 dark:text-white">
                {{ selectedRun.local_group_name || `分组 #${selectedRun.local_group_id}` }}
              </h2>
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                {{ formatTime(selectedRun.created_at) }} · {{ selectedRun.platform || '-' }}
              </p>
            </div>

            <div class="grid grid-cols-2 gap-3 text-sm">
              <div class="rounded-md border border-gray-200 p-3 dark:border-dark-700">
                <p class="text-xs text-gray-500 dark:text-dark-400">可调度账号</p>
                <p class="mt-1 font-semibold text-gray-900 dark:text-white">{{ selectedRun.before_schedulable_accounts }} -> {{ selectedRun.after_schedulable_accounts }}</p>
              </div>
              <div class="rounded-md border border-gray-200 p-3 dark:border-dark-700">
                <p class="text-xs text-gray-500 dark:text-dark-400">用户 Key</p>
                <p class="mt-1 font-semibold text-gray-900 dark:text-white">{{ selectedRun.before_active_api_key_count }} -> {{ selectedRun.after_active_api_key_count }}</p>
              </div>
            </div>

            <div class="rounded-md border border-gray-200 p-3 dark:border-dark-700">
              <p class="text-xs font-medium text-gray-500 dark:text-dark-400">候选</p>
              <p class="mt-2 text-sm text-gray-900 dark:text-white">{{ candidateLabel(selectedRun) }}</p>
              <p v-if="selectedRun.selected_effective_rate_multiplier" class="mt-1 text-xs text-emerald-600 dark:text-emerald-300">
                {{ routingRefillMultiplierLabel(selectedRun.selected_effective_rate_multiplier) }}
              </p>
            </div>

            <RoutingRefillImpactPanel :availability="selectedAvailability" />
          </div>
          <div v-else class="py-10 text-center text-sm text-gray-500 dark:text-dark-400">
            选择一条补池运行查看影响详情
          </div>
        </aside>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import Pagination from '@/components/common/Pagination.vue'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { useAppStore } from '@/stores/app'
import RoutingRefillImpactPanel from '@/views/admin/RoutingRefillImpactPanel.vue'
import {
  listLocalSub2APIGroups,
  listRoutingRefillRuns,
  type LocalSub2APIGroup,
  type RoutingGroupAvailability,
  type RoutingRefillRun
} from '@/api/admin/adminPlus'
import {
  routingRefillMultiplierLabel,
  routingRefillRunStatusClass,
  routingRefillRunStatusLabel,
  routingRefillSkippedReasonLabel
} from '@/views/admin/routingRefillPresentation'

const appStore = useAppStore()
const loading = ref(false)
const runs = ref<RoutingRefillRun[]>([])
const localGroups = ref<LocalSub2APIGroup[]>([])
const selectedRunID = ref<number | null>(null)
const filters = reactive({
  local_group_id: 0,
  status: ''
})
const pagination = reactive({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0,
  pages: 0
})

const selectedRun = computed(() => runs.value.find((run) => run.id === selectedRunID.value) || runs.value[0] || null)
const selectedAvailability = computed(() => availabilityFromRun(selectedRun.value))
const visibleStats = computed(() => {
  return runs.value.reduce((acc, run) => {
    if (run.status === 'succeeded') acc.succeeded++
    if (run.status === 'skipped') acc.skipped++
    if (run.status === 'failed') acc.failed++
    return acc
  }, { succeeded: 0, skipped: 0, failed: 0 })
})

onMounted(() => {
  void loadPage()
})

async function loadPage() {
  loading.value = true
  try {
    const [groupsResult, runsResult] = await Promise.all([
      listLocalSub2APIGroups({ limit: 1000 }),
      listRoutingRefillRuns({
        local_group_id: filters.local_group_id || undefined,
        status: filters.status || undefined,
        page: pagination.page,
        page_size: pagination.page_size
      })
    ])
    localGroups.value = groupsResult.items || []
    runs.value = runsResult.items || []
    pagination.total = runsResult.total || 0
    pagination.pages = runsResult.pages || 0
    pagination.page = runsResult.page || pagination.page
    pagination.page_size = runsResult.page_size || pagination.page_size
    if (!runs.value.some((run) => run.id === selectedRunID.value)) {
      selectedRunID.value = runs.value[0]?.id || null
    }
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '加载补池影响历史失败')
  } finally {
    loading.value = false
  }
}

function resetAndLoad() {
  pagination.page = 1
  void loadPage()
}

function clearFilters() {
  filters.local_group_id = 0
  filters.status = ''
  resetAndLoad()
}

function handlePageChange(page: number) {
  pagination.page = page
  void loadPage()
}

function handlePageSizeChange(pageSize: number) {
  pagination.page_size = pageSize
  pagination.page = 1
  void loadPage()
}

function selectRun(run: RoutingRefillRun) {
  selectedRunID.value = run.id
}

function candidateLabel(run: RoutingRefillRun): string {
  if (!run.selected_local_account_id) return '-'
  const parts = [
    run.selected_supplier_id ? `供应商 #${run.selected_supplier_id}` : '',
    run.selected_supplier_group_id ? `第三方分组 #${run.selected_supplier_group_id}` : '',
    `本地账号 #${run.selected_local_account_id}`
  ].filter(Boolean)
  return parts.join(' / ')
}

function runReasonLabel(run: RoutingRefillRun): string {
  if (run.error_message) return run.error_message
  if (run.skipped_reason) return routingRefillSkippedReasonLabel(run.skipped_reason)
  return run.reason || '-'
}

function availabilityFromRun(run?: RoutingRefillRun | null): RoutingGroupAvailability | null {
  if (!run) return null
  const snapshot = recordValue(run.result_snapshot, 'availability_before')
  const fromSnapshot = availabilityFromRecord(snapshot)
  if (fromSnapshot) return fromSnapshot
  return {
    group_id: run.local_group_id,
    group_name: run.local_group_name || `分组 #${run.local_group_id}`,
    platform: run.platform || '',
    total_accounts: run.before_total_accounts,
    schedulable_accounts: run.before_schedulable_accounts,
    active_api_key_count: run.before_active_api_key_count,
    would_empty_schedulable_pool: run.before_active_api_key_count > 0 && run.before_schedulable_accounts === 0,
    recent_window_seconds: 86400
  }
}

function availabilityFromRecord(value: unknown): RoutingGroupAvailability | null {
  if (!isRecord(value)) return null
  const groupID = numberValue(value.group_id)
  if (groupID <= 0) return null
  return {
    group_id: groupID,
    group_name: stringValue(value.group_name) || `分组 #${groupID}`,
    platform: stringValue(value.platform),
    total_accounts: numberValue(value.total_accounts),
    schedulable_accounts: numberValue(value.schedulable_accounts),
    active_api_key_count: numberValue(value.active_api_key_count),
    would_empty_schedulable_pool: booleanValue(value.would_empty_schedulable_pool),
    recent_window_seconds: numberValue(value.recent_window_seconds),
    recent_success_request_count: numberValue(value.recent_success_request_count),
    recent_error_request_count: numberValue(value.recent_error_request_count),
    recent_upstream_429_count: numberValue(value.recent_upstream_429_count),
    recent_token_count: numberValue(value.recent_token_count),
    recent_last_request_at: nullableStringValue(value.recent_last_request_at),
    recent_last_error_at: nullableStringValue(value.recent_last_error_at),
    impacted_api_keys: Array.isArray(value.impacted_api_keys) ? value.impacted_api_keys as RoutingGroupAvailability['impacted_api_keys'] : [],
    impacted_api_keys_truncated: booleanValue(value.impacted_api_keys_truncated),
    recent_failure_requests: Array.isArray(value.recent_failure_requests) ? value.recent_failure_requests as RoutingGroupAvailability['recent_failure_requests'] : [],
    recent_failures_truncated: booleanValue(value.recent_failures_truncated)
  }
}

function recordValue(source: Record<string, unknown> | undefined, key: string): unknown {
  return source ? source[key] : undefined
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function numberValue(value: unknown): number {
  const number = Number(value || 0)
  return Number.isFinite(number) ? number : 0
}

function stringValue(value: unknown): string {
  return typeof value === 'string' ? value : ''
}

function nullableStringValue(value: unknown): string | null {
  return typeof value === 'string' && value ? value : null
}

function booleanValue(value: unknown): boolean {
  return value === true
}

function formatTime(value?: string | null): string {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString('zh-CN', { hour12: false })
}
</script>
