<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">本地用量</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
            从本地 Sub2API 使用记录读取真实收入、token、延迟和账号维度聚合。
          </p>
        </div>
        <button type="button" class="btn btn-secondary" :disabled="loading" @click="loadUsage">
          <Icon name="refresh" size="sm" />
          刷新
        </button>
      </section>

      <section class="card p-5">
        <div class="grid gap-4 md:grid-cols-5">
          <label class="block">
            <span class="input-label">本地账号</span>
            <select v-model.number="filters.account_id" class="input">
              <option :value="0">全部账号</option>
              <option v-for="account in accounts" :key="account.id" :value="account.id">
                #{{ account.id }} {{ account.name }}
              </option>
            </select>
          </label>
          <label class="block">
            <span class="input-label">模型</span>
            <input v-model.trim="filters.model" class="input" placeholder="全部模型" />
          </label>
          <label class="block">
            <span class="input-label">开始时间</span>
            <input v-model="filters.from" type="datetime-local" class="input" />
          </label>
          <label class="block">
            <span class="input-label">结束时间</span>
            <input v-model="filters.to" type="datetime-local" class="input" />
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
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">请求数</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ totalRequests }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">收入</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ formatMoney(totalRevenueCents) }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">Token</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ totalTokens.toLocaleString() }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">平均首 token</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ avgFirstTokenMs }}ms</p>
        </div>
      </section>

      <section class="card overflow-hidden">
        <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">账号 / 模型聚合</h2>
        </div>
        <div class="overflow-x-auto">
          <table class="w-full min-w-[980px] divide-y divide-gray-200 dark:divide-dark-700">
            <thead class="bg-gray-50 dark:bg-dark-800">
              <tr>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">账号</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">模型</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">请求数</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">输入</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">输出</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">收入</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">首 token</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">总耗时</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-900">
              <tr v-if="summaries.length === 0">
                <td colspan="8" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-dark-400">暂无用量</td>
              </tr>
              <tr v-for="item in summaries" :key="`${item.account_id}-${item.model}`">
                <td class="px-4 py-4 text-sm text-gray-900 dark:text-gray-100">
                  {{ item.account_name || `#${item.account_id}` }}
                  <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ item.account_platform || '-' }}</div>
                </td>
                <td class="px-4 py-4 text-sm text-gray-900 dark:text-gray-100">{{ item.model }}</td>
                <td class="px-4 py-4 text-right text-sm text-gray-900 dark:text-gray-100">{{ item.request_count }}</td>
                <td class="px-4 py-4 text-right text-sm text-gray-900 dark:text-gray-100">{{ item.input_tokens.toLocaleString() }}</td>
                <td class="px-4 py-4 text-right text-sm text-gray-900 dark:text-gray-100">{{ item.output_tokens.toLocaleString() }}</td>
                <td class="px-4 py-4 text-right text-sm text-gray-900 dark:text-gray-100">{{ formatMoney(item.revenue_cents) }}</td>
                <td class="px-4 py-4 text-right text-sm text-gray-900 dark:text-gray-100">{{ item.avg_first_token_ms }}ms</td>
                <td class="px-4 py-4 text-right text-sm text-gray-900 dark:text-gray-100">{{ item.avg_total_latency_ms }}ms</td>
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
  listLocalSub2APIAccounts,
  listLocalUsageSummary,
  type LocalSub2APIAccount,
  type LocalUsageSummary
} from '@/api/admin/adminPlus'

const appStore = useAppStore()

const loading = ref(false)
const accounts = ref<LocalSub2APIAccount[]>([])
const summaries = ref<LocalUsageSummary[]>([])
const pagination = reactive({ page: 1, page_size: getPersistedPageSize(), total: 0, pages: 0 })

const filters = reactive({
  account_id: 0,
  model: '',
  from: toDateTimeLocal(new Date(Date.now() - 24 * 60 * 60 * 1000)),
  to: toDateTimeLocal(new Date())
})

const totalRequests = computed(() => summaries.value.reduce((sum, item) => sum + item.request_count, 0))
const totalRevenueCents = computed(() => summaries.value.reduce((sum, item) => sum + item.revenue_cents, 0))
const totalTokens = computed(() => summaries.value.reduce((sum, item) => sum + item.input_tokens + item.output_tokens, 0))
const avgFirstTokenMs = computed(() => {
  const count = summaries.value.reduce((sum, item) => sum + item.request_count, 0)
  if (!count) return 0
  const weighted = summaries.value.reduce((sum, item) => sum + item.avg_first_token_ms * item.request_count, 0)
  return Math.round(weighted / count)
})

function toDateTimeLocal(value: Date): string {
  const offsetMs = value.getTimezoneOffset() * 60 * 1000
  return new Date(value.getTime() - offsetMs).toISOString().slice(0, 16)
}

function toRFC3339(value: string): string {
  return new Date(value).toISOString()
}

function formatMoney(cents: number): string {
  return new Intl.NumberFormat(undefined, {
    style: 'currency',
    currency: 'USD',
    minimumFractionDigits: 2
  }).format((cents || 0) / 100)
}

async function loadUsage() {
  loading.value = true
  try {
    const [accountResult, summaryResult] = await Promise.all([
      listLocalSub2APIAccounts({ limit: 200 }),
      listLocalUsageSummary({
        account_id: filters.account_id || undefined,
        model: filters.model || undefined,
        from: toRFC3339(filters.from),
        to: toRFC3339(filters.to),
        page: pagination.page,
        page_size: pagination.page_size
      })
    ])
    accounts.value = accountResult.items
    summaries.value = summaryResult.items
    pagination.total = summaryResult.total || 0
    pagination.pages = summaryResult.pages || 0
    pagination.page = summaryResult.page || pagination.page
    pagination.page_size = summaryResult.page_size || pagination.page_size
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '加载本地用量失败')
  } finally {
    loading.value = false
  }
}

function submitFilters() {
  pagination.page = 1
  void loadUsage()
}

function handlePageChange(page: number) {
  pagination.page = page
  void loadUsage()
}

function handlePageSizeChange(pageSize: number) {
  pagination.page_size = pageSize
  pagination.page = 1
  void loadUsage()
}

onMounted(loadUsage)
</script>
