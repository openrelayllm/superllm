<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">系统日志</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
            Admin Plus 业务诊断事件，用于排查直登、余额同步、验证码读取和插件任务。
          </p>
        </div>
        <button type="button" class="btn btn-secondary" :disabled="loading" @click="loadLogs">
          <Icon name="refresh" size="sm" />
          刷新
        </button>
      </section>

      <section class="grid gap-4 md:grid-cols-4">
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">当前结果</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ logs.length }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">失败事件</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ failedCount }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">直登失败</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ loginFailureCount }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">读取验证码</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ mailCount }}</p>
        </div>
      </section>

      <section class="card p-5">
        <div class="grid gap-4 lg:grid-cols-[1fr_1fr_1.2fr_1fr_auto] lg:items-end">
          <label class="block">
            <span class="input-label">分类</span>
            <select v-model="filters.component" class="input" @change="resetAndLoad">
              <option value="">全部 Admin Plus</option>
              <option v-for="option in componentOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
            </select>
          </label>
          <label class="block">
            <span class="input-label">级别</span>
            <select v-model="filters.level" class="input" @change="resetAndLoad">
              <option value="">全部</option>
              <option value="warn">Warn</option>
              <option value="error">Error</option>
              <option value="info">Info</option>
            </select>
          </label>
          <label class="block">
            <span class="input-label">关键字</span>
            <input v-model.trim="filters.q" class="input" placeholder="reason / endpoint / message" @keyup.enter="resetAndLoad" />
          </label>
          <label class="block">
            <span class="input-label">时间范围</span>
            <select v-model="filters.window" class="input" @change="resetAndLoad">
              <option value="1h">最近 1 小时</option>
              <option value="6h">最近 6 小时</option>
              <option value="24h">最近 24 小时</option>
            </select>
          </label>
          <button type="button" class="btn btn-primary" :disabled="loading" @click="resetAndLoad">
            <Icon name="search" size="sm" />
            查询
          </button>
        </div>
      </section>

      <section class="card overflow-hidden">
        <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">日志列表</h2>
        </div>
        <div class="overflow-x-auto">
          <table class="w-full min-w-[1280px] divide-y divide-gray-200 dark:divide-dark-700">
            <thead class="bg-gray-50 dark:bg-dark-800">
              <tr>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">时间</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">级别</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">分类 / 动作</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">供应商</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">结果</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">Reason / Endpoint</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">消息</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-900">
              <tr v-if="logs.length === 0">
                <td colspan="7" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-dark-400">
                  {{ loading ? '加载中...' : '暂无 Admin Plus 业务日志' }}
                </td>
              </tr>
              <tr v-for="log in logs" :key="`${log.component}:${log.id}`" class="align-top">
                <td class="whitespace-nowrap px-4 py-4 text-sm text-gray-500 dark:text-dark-400">{{ formatDateTime(log.created_at) }}</td>
                <td class="px-4 py-4">
                  <span class="badge" :class="levelClass(log.level)">{{ levelLabel(log.level) }}</span>
                </td>
                <td class="px-4 py-4 text-sm text-gray-900 dark:text-gray-100">
                  <div class="font-medium">{{ componentLabel(log.component) }}</div>
                  <div class="mt-1 font-mono text-xs text-gray-500 dark:text-dark-400">{{ stringExtra(log, 'action') || '-' }}</div>
                </td>
                <td class="px-4 py-4 text-sm text-gray-900 dark:text-gray-100">
                  <div>{{ stringExtra(log, 'supplier_name') || supplierIdLabel(log) }}</div>
                  <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ stringExtra(log, 'provider_type') || stringExtra(log, 'system_type') || '-' }}</div>
                </td>
                <td class="px-4 py-4">
                  <span class="badge" :class="outcomeClass(stringExtra(log, 'outcome'))">{{ outcomeLabel(stringExtra(log, 'outcome')) }}</span>
                  <div v-if="stringExtra(log, 'status_code')" class="mt-2 font-mono text-xs text-gray-500 dark:text-dark-400">HTTP {{ stringExtra(log, 'status_code') }}</div>
                </td>
                <td class="px-4 py-4 text-sm text-gray-900 dark:text-gray-100">
                  <div class="font-mono text-xs">{{ stringExtra(log, 'reason') || '-' }}</div>
                  <div class="mt-2 max-w-[360px] break-all font-mono text-xs text-gray-500 dark:text-dark-400">{{ stringExtra(log, 'endpoint') || '-' }}</div>
                  <div v-if="stringExtra(log, 'store_step')" class="mt-2 font-mono text-xs text-gray-500 dark:text-dark-400">step: {{ stringExtra(log, 'store_step') }}</div>
                  <div v-if="stringExtra(log, 'error_message')" class="mt-2 max-w-[360px] whitespace-pre-wrap break-words rounded bg-gray-50 p-2 font-mono text-xs text-gray-600 dark:bg-dark-800 dark:text-dark-300">
                    {{ stringExtra(log, 'error_message') }}
                  </div>
                  <div v-if="stringExtra(log, 'body_excerpt')" class="mt-2 max-w-[360px] whitespace-pre-wrap break-words rounded bg-gray-50 p-2 font-mono text-xs text-gray-600 dark:bg-dark-800 dark:text-dark-300">
                    {{ stringExtra(log, 'body_excerpt') }}
                  </div>
                </td>
                <td class="px-4 py-4 text-sm text-gray-900 dark:text-gray-100">
                  <div>{{ log.message || '-' }}</div>
                  <div v-if="stringExtra(log, 'message_id')" class="mt-2 font-mono text-xs text-gray-500 dark:text-dark-400">message_id: {{ stringExtra(log, 'message_id') }}</div>
                  <div v-if="stringExtra(log, 'task_id')" class="mt-2 font-mono text-xs text-gray-500 dark:text-dark-400">task_id: {{ stringExtra(log, 'task_id') }}</div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <Pagination
          v-if="total > 0"
          :page="pagination.page"
          :total="total"
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

const appStore = useAppStore()

const componentOptions: Array<{ value: AdminPlusSystemLogComponent; label: string }> = [
  { value: 'admin_plus.login', label: '供应商直登' },
  { value: 'admin_plus.balance', label: '余额同步' },
  { value: 'admin_plus.mail', label: '邮箱验证码' },
  { value: 'admin_plus.registration', label: '注册任务' },
  { value: 'admin_plus.extension', label: '插件任务' },
  { value: 'admin_plus.sub2api', label: '本地账号动作' }
]

const loading = ref(false)
const logs = ref<AdminPlusSystemLog[]>([])
const total = ref(0)
const pagination = reactive({
  page: 1,
  page_size: getPersistedPageSize(),
})
const filters = reactive({
  component: '' as AdminPlusSystemLogComponent,
  level: '' as AdminPlusSystemLogLevel,
  q: '',
  window: '6h'
})

const failedCount = computed(() => logs.value.filter((item) => stringExtra(item, 'outcome') === 'failed').length)
const loginFailureCount = computed(() => logs.value.filter((item) => item.component === 'admin_plus.login' && stringExtra(item, 'outcome') === 'failed').length)
const mailCount = computed(() => logs.value.filter((item) => item.component === 'admin_plus.mail').length)

onMounted(() => {
  void loadLogs()
})

async function loadLogs() {
  loading.value = true
  try {
    const params = {
      page: pagination.page,
      page_size: pagination.page_size,
      level: filters.level || undefined,
      q: filters.q || undefined,
      start_time: startTimeISO(filters.window),
      end_time: new Date().toISOString()
    }
    if (filters.component) {
      const result = await listAdminPlusSystemLogs({ ...params, component: filters.component })
      logs.value = result.items || []
      total.value = result.total || 0
      pagination.page = result.page || pagination.page
      pagination.page_size = result.page_size || pagination.page_size
      return
    }

    const results = await Promise.all(componentOptions.map((option) =>
      listAdminPlusSystemLogs({
        ...params,
        page: 1,
        page_size: Math.max(20, Math.min(pagination.page_size, 50)),
        component: option.value
      })
    ))
    const merged = results.flatMap((result) => result.items || [])
      .sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime())
    const offset = (pagination.page - 1) * pagination.page_size
    logs.value = merged.slice(offset, offset + pagination.page_size)
    total.value = results.reduce((sum, item) => sum + (item.total || 0), 0)
  } catch (error) {
    appStore.showError((error as { message?: string })?.message || '加载系统日志失败')
  } finally {
    loading.value = false
  }
}

function resetAndLoad() {
  pagination.page = 1
  void loadLogs()
}

function handlePageChange(page: number) {
  pagination.page = page
  void loadLogs()
}

function handlePageSizeChange(pageSize: number) {
  pagination.page_size = pageSize
  pagination.page = 1
  void loadLogs()
}

function startTimeISO(windowValue: string): string {
  const hours = windowValue === '24h' ? 24 : windowValue === '1h' ? 1 : 6
  return new Date(Date.now() - hours * 60 * 60 * 1000).toISOString()
}

function stringExtra(log: AdminPlusSystemLog, key: string): string {
  const value = log.extra?.[key]
  if (value == null) return ''
  return String(value)
}

function supplierIdLabel(log: AdminPlusSystemLog): string {
  const supplierID = stringExtra(log, 'supplier_id')
  return supplierID ? `#${supplierID}` : '-'
}

function componentLabel(component: string): string {
  return componentOptions.find((item) => item.value === component)?.label || component
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
  if (outcome === 'failed') return '失败'
  if (outcome === 'retry_scheduled') return '已重试'
  return outcome || '-'
}

function outcomeClass(outcome: string): string {
  if (outcome === 'succeeded') return 'badge-success'
  if (outcome === 'failed') return 'badge-danger'
  if (outcome === 'retry_scheduled') return 'badge-warning'
  return 'badge-gray'
}

function formatDateTime(value?: string | null): string {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleString()
}
</script>
