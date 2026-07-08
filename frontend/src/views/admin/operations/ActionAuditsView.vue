<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">操作审计</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
            聚合 Admin Plus 人工和自动动作，优先用于排查本地账号写回、drift 同步、供应商直登、余额同步和插件任务。
          </p>
        </div>
        <div class="flex flex-wrap gap-2">
          <RouterLink to="/admin/system-logs" class="btn btn-secondary">
            <Icon name="terminal" size="sm" />
            系统日志
          </RouterLink>
          <button type="button" class="btn btn-secondary" :disabled="loading" @click="loadAudits">
            <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
            刷新
          </button>
        </div>
      </section>

      <section class="grid gap-4 md:grid-cols-4">
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">当前结果</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ filteredLogs.length }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">成功动作</p>
          <p class="mt-2 text-2xl font-semibold text-emerald-600 dark:text-emerald-300">{{ auditStats.succeeded }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">阻断/漂移</p>
          <p class="mt-2 text-2xl font-semibold text-amber-600 dark:text-amber-300">{{ auditStats.blocked }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">失败动作</p>
          <p class="mt-2 text-2xl font-semibold text-rose-600 dark:text-rose-300">{{ auditStats.failed }}</p>
        </div>
      </section>

      <section class="card p-5">
        <div class="grid gap-4 xl:grid-cols-[1fr_1fr_1fr_1fr_1.4fr_auto] xl:items-end">
          <label class="block">
            <span class="input-label">动作域</span>
            <select v-model="filters.action" class="input" @change="resetPagination">
              <option value="">全部动作</option>
              <option v-for="option in actionOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
            </select>
          </label>
          <label class="block">
            <span class="input-label">组件</span>
            <select v-model="filters.component" class="input" @change="resetPagination">
              <option value="">全部 Admin Plus</option>
              <option v-for="option in componentOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
            </select>
          </label>
          <label class="block">
            <span class="input-label">结果</span>
            <select v-model="filters.outcome" class="input" @change="resetPagination">
              <option value="">全部结果</option>
              <option value="succeeded">成功</option>
              <option value="blocked">阻断</option>
              <option value="drift_detected">发现漂移</option>
              <option value="failed">失败</option>
            </select>
          </label>
          <label class="block">
            <span class="input-label">时间范围</span>
            <select v-model="filters.window" class="input" @change="resetAndLoad">
              <option value="1h">最近 1 小时</option>
              <option value="6h">最近 6 小时</option>
              <option value="24h">最近 24 小时</option>
              <option value="72h">最近 72 小时</option>
            </select>
          </label>
          <label class="block">
            <span class="input-label">关键字</span>
            <input v-model.trim="filters.q" class="input" placeholder="账号 ID、分组、供应商、reason、message" @keyup.enter="resetPagination" />
          </label>
          <button type="button" class="btn btn-primary" :disabled="loading" @click="resetAndLoad">
            <Icon name="search" size="sm" />
            查询
          </button>
        </div>
      </section>

      <section class="card overflow-hidden">
        <div class="flex flex-wrap items-center justify-between gap-2 border-b border-gray-100 px-5 py-4 dark:border-dark-700">
          <div>
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">动作时间线</h2>
            <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">当前从每个组件最多读取最近 200 条日志；完整底层日志可进入系统日志查看。</p>
          </div>
          <span class="badge badge-gray">{{ pagedLogs.length }}/{{ filteredLogs.length }}</span>
        </div>
        <div class="overflow-x-auto">
          <table class="w-full min-w-[1360px] divide-y divide-gray-200 dark:divide-dark-700">
            <thead class="bg-gray-50 dark:bg-dark-800">
              <tr>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">时间</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">动作</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">结果</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">对象</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">影响</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">原因/错误</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">消息</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-900">
              <tr v-if="pagedLogs.length === 0">
                <td colspan="7" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-dark-400">
                  {{ loading ? '加载中...' : '暂无操作审计记录' }}
                </td>
              </tr>
              <tr v-for="log in pagedLogs" :key="`${log.component}:${log.id}`" class="align-top">
                <td class="whitespace-nowrap px-4 py-4 text-sm text-gray-500 dark:text-dark-400">{{ formatDateTime(log.created_at) }}</td>
                <td class="px-4 py-4">
                  <div class="flex flex-wrap items-center gap-2">
                    <span class="badge badge-gray">{{ componentLabel(log.component) }}</span>
                    <span class="badge" :class="levelClass(log.level)">{{ levelLabel(log.level) }}</span>
                  </div>
                  <div class="mt-2 font-mono text-xs text-gray-700 dark:text-dark-300">{{ actionLabel(stringExtra(log, 'action')) }}</div>
                </td>
                <td class="px-4 py-4">
                  <span class="badge" :class="outcomeClass(stringExtra(log, 'outcome'))">{{ outcomeLabel(stringExtra(log, 'outcome')) }}</span>
                  <div v-if="stringExtra(log, 'requested_by')" class="mt-2 text-xs text-gray-500 dark:text-dark-400">operator #{{ stringExtra(log, 'requested_by') }}</div>
                </td>
                <td class="px-4 py-4 text-sm text-gray-900 dark:text-gray-100">
                  <div>{{ objectLabel(log) }}</div>
                  <div v-if="stringExtra(log, 'supplier_name') || stringExtra(log, 'provider_type')" class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                    {{ stringExtra(log, 'provider_type') || '-' }}
                  </div>
                </td>
                <td class="px-4 py-4 text-sm text-gray-900 dark:text-gray-100">
                  <div class="grid max-w-[320px] grid-cols-[auto_1fr] gap-x-2 gap-y-1 text-xs">
                    <span class="text-gray-500 dark:text-dark-400">账号</span>
                    <span class="truncate font-mono" :title="accountIdsLabel(log)">{{ accountIdsLabel(log) }}</span>
                    <span class="text-gray-500 dark:text-dark-400">分组</span>
                    <span class="truncate font-mono" :title="groupIdsLabel(log)">{{ groupIdsLabel(log) }}</span>
                    <span class="text-gray-500 dark:text-dark-400">写入</span>
                    <span>{{ effectLabel(log) }}</span>
                  </div>
                </td>
                <td class="px-4 py-4 text-sm text-gray-900 dark:text-gray-100">
                  <div class="max-w-[280px] truncate font-mono text-xs" :title="stringExtra(log, 'reason') || ''">
                    {{ stringExtra(log, 'reason') || '-' }}
                  </div>
                  <div v-if="stringExtra(log, 'error_message')" class="mt-2 max-w-[280px] whitespace-pre-wrap break-words rounded bg-gray-50 p-2 font-mono text-xs text-red-600 dark:bg-dark-800 dark:text-red-300">
                    {{ stringExtra(log, 'error_message') }}
                  </div>
                </td>
                <td class="px-4 py-4 text-sm text-gray-900 dark:text-gray-100">
                  <div class="max-w-[360px] whitespace-pre-wrap break-words">{{ log.message || '-' }}</div>
                  <div v-if="stringExtra(log, 'endpoint')" class="mt-2 max-w-[360px] break-all font-mono text-xs text-gray-500 dark:text-dark-400">{{ stringExtra(log, 'endpoint') }}</div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <Pagination
          v-if="filteredLogs.length > 0"
          :page="pagination.page"
          :total="filteredLogs.length"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import Pagination from '@/components/common/Pagination.vue'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { useAppStore } from '@/stores/app'
import {
  listAdminPlusSystemLogs,
  type AdminPlusSystemLog,
  type AdminPlusSystemLogComponent,
  type AdminPlusSystemLogLevel
} from '@/api/admin/adminPlus'

type AuditOutcome = '' | 'succeeded' | 'blocked' | 'drift_detected' | 'failed'

const appStore = useAppStore()
const route = useRoute()

const componentOptions: Array<{ value: Exclude<AdminPlusSystemLogComponent, ''>; label: string }> = [
  { value: 'admin_plus.sub2api', label: '本地账号动作' },
  { value: 'admin_plus.login', label: '供应商直登' },
  { value: 'admin_plus.balance', label: '余额同步' },
  { value: 'admin_plus.registration', label: '注册任务' },
  { value: 'admin_plus.extension', label: '插件任务' },
  { value: 'admin_plus.mail', label: '邮箱验证码' }
]

const actionOptions = [
  { value: 'set_schedulable', label: '开启/关闭调度' },
  { value: 'add_to_groups', label: '加入本地分组' },
  { value: 'remove_from_groups', label: '移出本地分组' },
  { value: 'local_account_state_sync', label: '同步本地状态' },
  { value: 'accept_observed', label: '采纳原后台变更' },
  { value: 'restore_accepted', label: '恢复 Admin Plus 基线' },
  { value: 'direct_login', label: '供应商直登' },
  { value: 'refresh_balance', label: '余额同步' }
]

const loading = ref(false)
const rawLogs = ref<AdminPlusSystemLog[]>([])
const pagination = reactive({
  page: 1,
  page_size: getPersistedPageSize()
})
const filters = reactive({
  component: normalizeComponentQuery(route.query.component),
  level: normalizeLevelQuery(route.query.level),
  outcome: normalizeOutcomeQuery(route.query.outcome),
  action: normalizeActionQuery(route.query.action),
  q: typeof route.query.q === 'string' ? route.query.q : '',
  window: normalizeWindowQuery(route.query.window)
})

const filteredLogs = computed(() => {
  const q = filters.q.trim().toLowerCase()
  return rawLogs.value.filter((log) => {
    if (filters.component && log.component !== filters.component) return false
    if (filters.level && log.level !== filters.level) return false
    if (filters.outcome && stringExtra(log, 'outcome') !== filters.outcome) return false
    if (filters.action && stringExtra(log, 'action') !== filters.action) return false
    if (q && !auditHaystack(log).includes(q)) return false
    return true
  })
})

const pagedLogs = computed(() => {
  const offset = (pagination.page - 1) * pagination.page_size
  return filteredLogs.value.slice(offset, offset + pagination.page_size)
})

const auditStats = computed(() => {
  let succeeded = 0
  let blocked = 0
  let failed = 0
  for (const log of filteredLogs.value) {
    const outcome = stringExtra(log, 'outcome')
    if (outcome === 'succeeded') succeeded++
    if (outcome === 'blocked' || outcome === 'drift_detected') blocked++
    if (outcome === 'failed') failed++
  }
  return { succeeded, blocked, failed }
})

onMounted(() => {
  void loadAudits()
})

async function loadAudits() {
  loading.value = true
  try {
    const components = filters.component
      ? [filters.component as Exclude<AdminPlusSystemLogComponent, ''>]
      : componentOptions.map((item) => item.value)
    const resultSets = await Promise.all(components.map((component) =>
      listAdminPlusSystemLogs({
        page: 1,
        page_size: 200,
        component,
        start_time: startTimeISO(filters.window),
        end_time: new Date().toISOString()
      })
    ))
    rawLogs.value = resultSets
      .flatMap((result) => result.items || [])
      .sort((a, b) => {
        const timeDelta = new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
        if (timeDelta !== 0) return timeDelta
        return b.id - a.id
      })
    resetPagination()
  } catch (error) {
    appStore.showError((error as { message?: string })?.message || '加载操作审计失败')
  } finally {
    loading.value = false
  }
}

function resetAndLoad() {
  pagination.page = 1
  void loadAudits()
}

function resetPagination() {
  pagination.page = 1
}

function handlePageChange(page: number) {
  pagination.page = page
}

function handlePageSizeChange(pageSize: number) {
  pagination.page_size = pageSize
  pagination.page = 1
}

function startTimeISO(windowValue: string): string {
  const hours = windowValue === '72h' ? 72 : windowValue === '24h' ? 24 : windowValue === '1h' ? 1 : 6
  return new Date(Date.now() - hours * 60 * 60 * 1000).toISOString()
}

function normalizeComponentQuery(value: unknown): AdminPlusSystemLogComponent {
  const text = typeof value === 'string' ? value : ''
  return componentOptions.some((option) => option.value === text) ? text as AdminPlusSystemLogComponent : ''
}

function normalizeLevelQuery(value: unknown): AdminPlusSystemLogLevel {
  const text = typeof value === 'string' ? value : ''
  return ['', 'info', 'warn', 'error'].includes(text) ? text as AdminPlusSystemLogLevel : ''
}

function normalizeOutcomeQuery(value: unknown): AuditOutcome {
  const text = typeof value === 'string' ? value : ''
  return ['', 'succeeded', 'blocked', 'drift_detected', 'failed'].includes(text) ? text as AuditOutcome : ''
}

function normalizeActionQuery(value: unknown): string {
  const text = typeof value === 'string' ? value : ''
  return actionOptions.some((option) => option.value === text) ? text : ''
}

function normalizeWindowQuery(value: unknown): string {
  const text = typeof value === 'string' ? value : ''
  return ['1h', '6h', '24h', '72h'].includes(text) ? text : '24h'
}

function stringExtra(log: AdminPlusSystemLog, key: string): string {
  const value = log.extra?.[key]
  if (value == null) return ''
  if (Array.isArray(value)) return value.join(',')
  return String(value)
}

function arrayExtra(log: AdminPlusSystemLog, key: string): string[] {
  const value = log.extra?.[key]
  if (Array.isArray(value)) return value.map((item) => String(item))
  if (value == null || value === '') return []
  return [String(value)]
}

function auditHaystack(log: AdminPlusSystemLog): string {
  return [
    log.message,
    log.component,
    log.level,
    log.request_id,
    log.client_request_id,
    log.platform,
    log.model,
    JSON.stringify(log.extra || {})
  ].filter(Boolean).join(' ').toLowerCase()
}

function componentLabel(component: string): string {
  return componentOptions.find((item) => item.value === component)?.label || component
}

function actionLabel(action: string): string {
  return actionOptions.find((item) => item.value === action)?.label || action || '-'
}

function objectLabel(log: AdminPlusSystemLog): string {
  const supplierName = stringExtra(log, 'supplier_name')
  if (supplierName) return supplierName
  const supplierID = stringExtra(log, 'supplier_id')
  if (supplierID) return `供应商 #${supplierID}`
  const accountIDs = arrayExtra(log, 'account_ids')
  if (accountIDs.length > 0) return `本地账号 ${accountIDs.slice(0, 3).map((id) => `#${id}`).join(', ')}${accountIDs.length > 3 ? ` +${accountIDs.length - 3}` : ''}`
  return '-'
}

function accountIdsLabel(log: AdminPlusSystemLog): string {
  const ids = arrayExtra(log, 'account_ids')
  return ids.length > 0 ? ids.join(', ') : '-'
}

function groupIdsLabel(log: AdminPlusSystemLog): string {
  const ids = arrayExtra(log, 'group_ids')
  return ids.length > 0 ? ids.join(', ') : '-'
}

function effectLabel(log: AdminPlusSystemLog): string {
  const parts = [
    countLabel(log, 'updated_accounts', '更新'),
    countLabel(log, 'added_bindings', '加入'),
    countLabel(log, 'removed_bindings', '移出'),
    countLabel(log, 'resolved_accounts', '解决'),
    countLabel(log, 'restored_accounts', '恢复'),
    countLabel(log, 'pending_drift_accounts', '漂移')
  ].filter(Boolean)
  return parts.length > 0 ? parts.join(' / ') : '-'
}

function countLabel(log: AdminPlusSystemLog, key: string, label: string): string {
  const value = Number(stringExtra(log, key) || 0)
  return value > 0 ? `${label} ${value}` : ''
}

function levelLabel(level: string): string {
  return level ? level.toUpperCase() : '-'
}

function levelClass(level: string): string {
  if (level === 'error') return 'badge-danger'
  if (level === 'warn' || level === 'warning') return 'badge-warning'
  return 'badge-gray'
}

function outcomeLabel(outcome: string): string {
  if (outcome === 'succeeded') return '成功'
  if (outcome === 'blocked') return '阻断'
  if (outcome === 'drift_detected') return '发现漂移'
  if (outcome === 'failed') return '失败'
  return outcome || '-'
}

function outcomeClass(outcome: string): string {
  if (outcome === 'succeeded') return 'badge-success'
  if (outcome === 'blocked' || outcome === 'drift_detected') return 'badge-warning'
  if (outcome === 'failed') return 'badge-danger'
  return 'badge-gray'
}

function formatDateTime(value?: string | null): string {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleString()
}
</script>
