<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">账号运行态</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
            读取本地 Sub2API 账号状态和 Redis 当前并发，用于判断账号是否可参与切换。
          </p>
        </div>
        <button type="button" class="btn btn-secondary" :disabled="loading" @click="loadRuntime">
          <Icon name="refresh" size="sm" />
          刷新
        </button>
      </section>

      <section class="card p-5">
        <div class="grid gap-4 md:grid-cols-4">
          <label class="block">
            <span class="input-label">本地账号</span>
            <select v-model.number="filters.account_id" class="input">
              <option :value="0">全部账号</option>
              <option v-for="account in accounts" :key="account.id" :value="account.id">
                #{{ account.id }} {{ account.name }}
              </option>
            </select>
          </label>
          <label class="block md:col-span-2">
            <span class="input-label">搜索</span>
            <input v-model.trim="filters.q" class="input" placeholder="账号名 / 平台 / ID" @keyup.enter="submitFilters" />
          </label>
          <div class="flex items-end">
            <button type="button" class="btn btn-primary w-full" :disabled="loading" @click="submitFilters">
              查询
            </button>
          </div>
        </div>
      </section>

      <section class="grid gap-4 md:grid-cols-4">
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">账号数</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ runtimeItems.length }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">可切换</p>
          <p class="mt-2 text-2xl font-semibold text-emerald-600 dark:text-emerald-400">{{ switchEligibleCount }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">当前并发</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ totalConcurrency }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">等待队列</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ totalWaiting }}</p>
        </div>
      </section>

      <section class="card overflow-hidden">
        <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
          <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">运行态列表</h2>
            <p v-if="lastCollectedAt" class="text-xs text-gray-500 dark:text-dark-400">
              采集时间 {{ formatDateTime(lastCollectedAt) }}
            </p>
          </div>
        </div>
        <div class="overflow-x-auto">
          <table class="w-full min-w-[1120px] divide-y divide-gray-200 dark:divide-dark-700">
            <thead class="bg-gray-50 dark:bg-dark-800">
              <tr>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">账号</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">状态</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">并发</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">等待</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">负载</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">切换资格</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">运行阻断</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">最近使用</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-900">
              <tr v-if="runtimeItems.length === 0">
                <td colspan="8" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-dark-400">暂无运行态</td>
              </tr>
              <tr v-for="item in runtimeItems" :key="item.account_id">
                <td class="px-4 py-4 text-sm text-gray-900 dark:text-gray-100">
                  <div class="font-medium">{{ item.account_name || `#${item.account_id}` }}</div>
                  <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                    #{{ item.account_id }} · {{ platformLabel(item.account_platform) }} / {{ item.account_type || '-' }}
                  </div>
                </td>
                <td class="px-4 py-4 text-sm">
                  <span class="badge w-fit" :class="statusClass(item.status, item.schedulable)">
                    {{ statusLabel(item.status, item.schedulable) }}
                  </span>
                </td>
                <td class="px-4 py-4 text-right text-sm text-gray-900 dark:text-gray-100">
                  {{ item.current_concurrency }} / {{ item.configured_limit || '-' }}
                </td>
                <td class="px-4 py-4 text-right text-sm text-gray-900 dark:text-gray-100">{{ item.waiting_count }}</td>
                <td class="px-4 py-4 text-right text-sm text-gray-900 dark:text-gray-100">{{ formatPercent(item.load_percent) }}</td>
                <td class="px-4 py-4 text-sm">
                  <span class="badge w-fit" :class="item.switch_eligible ? 'badge-success' : 'badge-warning'">
                    {{ item.switch_eligible ? '可切换' : '不可切换' }}
                  </span>
                </td>
                <td class="px-4 py-4 text-sm text-gray-700 dark:text-gray-200">
                  <div>{{ blockedLabel(item) }}</div>
                  <div v-if="blockedUntil(item)" class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                    {{ blockedUntil(item) }}
                  </div>
                </td>
                <td class="px-4 py-4 text-sm text-gray-700 dark:text-gray-200">
                  {{ item.last_used_at ? formatDateTime(item.last_used_at) : '-' }}
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
  listLocalAccountRuntime,
  listLocalSub2APIAccounts,
  type LocalAccountRuntime,
  type LocalSub2APIAccount
} from '@/api/admin/adminPlus'

const appStore = useAppStore()

const loading = ref(false)
const accounts = ref<LocalSub2APIAccount[]>([])
const runtimeItems = ref<LocalAccountRuntime[]>([])
const pagination = reactive({ page: 1, page_size: getPersistedPageSize(), total: 0, pages: 0 })

const filters = reactive({
  account_id: 0,
  q: ''
})

const switchEligibleCount = computed(() => runtimeItems.value.filter((item) => item.switch_eligible).length)
const totalConcurrency = computed(() => runtimeItems.value.reduce((sum, item) => sum + item.current_concurrency, 0))
const totalWaiting = computed(() => runtimeItems.value.reduce((sum, item) => sum + item.waiting_count, 0))
const lastCollectedAt = computed(() => runtimeItems.value[0]?.collected_at || '')

async function loadRuntime() {
  loading.value = true
  try {
    const [accountResult, runtimeResult] = await Promise.all([
      listLocalSub2APIAccounts({ limit: 200 }),
      listLocalAccountRuntime({
        account_id: filters.account_id || undefined,
        q: filters.q || undefined,
        page: pagination.page,
        page_size: pagination.page_size
      })
    ])
    accounts.value = accountResult.items
    runtimeItems.value = runtimeResult.items
    pagination.total = runtimeResult.total || 0
    pagination.pages = runtimeResult.pages || 0
    pagination.page = runtimeResult.page || pagination.page
    pagination.page_size = runtimeResult.page_size || pagination.page_size
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '加载账号运行态失败')
  } finally {
    loading.value = false
  }
}

function submitFilters() {
  pagination.page = 1
  void loadRuntime()
}

function handlePageChange(page: number) {
  pagination.page = page
  void loadRuntime()
}

function handlePageSizeChange(pageSize: number) {
  pagination.page_size = pageSize
  pagination.page = 1
  void loadRuntime()
}

function platformLabel(value: string): string {
  const labels: Record<string, string> = {
    openai: 'OpenAI',
    anthropic: 'Anthropic',
    gemini: 'Gemini',
    antigravity: 'Antigravity',
    vertex: 'Vertex AI'
  }
  return labels[value] || value || '-'
}

function statusLabel(status: string, schedulable: boolean): string {
  if (!schedulable) return '暂停调度'
  if (status === 'active') return '正常'
  if (status === 'error') return '错误'
  if (status === 'disabled') return '禁用'
  return status || '-'
}

function statusClass(status: string, schedulable: boolean): string {
  if (!schedulable) return 'badge-warning'
  if (status === 'active') return 'badge-success'
  if (status === 'error') return 'badge-danger'
  return 'badge-muted'
}

function blockedLabel(item: LocalAccountRuntime): string {
  if (item.switch_eligible) return '-'
  if (item.blocked_reason === 'rate_limited') return '限流中'
  if (item.blocked_reason === 'overloaded') return '过载保护'
  if (item.blocked_reason === 'temp_unschedulable') return item.temp_unsched_reason || '临时不可调度'
  if (item.blocked_reason === 'concurrency_full') return '并发已满'
  if (item.blocked_reason === 'not_schedulable') return '调度关闭'
  if (item.blocked_reason?.startsWith('status_')) return statusLabel(item.status, item.schedulable)
  return item.error_message || item.blocked_reason || '-'
}

function blockedUntil(item: LocalAccountRuntime): string {
  const value = item.rate_limit_reset_at || item.overload_until || item.temp_unsched_until
  return value ? `恢复时间 ${formatDateTime(value)}` : ''
}

function formatPercent(value: number): string {
  return `${Math.round(value || 0)}%`
}

function formatDateTime(value: string): string {
  return new Intl.DateTimeFormat(undefined, {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  }).format(new Date(value))
}

onMounted(loadRuntime)
</script>
