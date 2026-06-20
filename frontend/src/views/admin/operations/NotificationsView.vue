<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">通知记录</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
            查看飞书通知投递状态、去重键和失败原因。
          </p>
        </div>
        <button type="button" class="btn btn-secondary" :disabled="loading" @click="loadPage">
          <Icon name="refresh" size="sm" />
          刷新
        </button>
      </section>

      <section class="grid gap-4 md:grid-cols-4">
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">总投递</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ deliveries.length }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">成功</p>
          <p class="mt-2 text-2xl font-semibold text-emerald-600 dark:text-emerald-400">{{ succeededCount }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">失败</p>
          <p class="mt-2 text-2xl font-semibold text-rose-600 dark:text-rose-400">{{ failedCount }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">发送中</p>
          <p class="mt-2 text-2xl font-semibold text-amber-600 dark:text-amber-400">{{ sendingCount }}</p>
        </div>
      </section>

      <section class="card overflow-hidden">
        <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">投递明细</h2>
        </div>
        <div class="overflow-x-auto">
          <table class="w-full min-w-[980px] divide-y divide-gray-200 dark:divide-dark-700">
            <thead class="bg-gray-50 dark:bg-dark-800">
              <tr>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">ID</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">通道</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">事件</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">供应商</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">状态</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">去重键</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">时间</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-900">
              <tr v-if="deliveries.length === 0">
                <td colspan="7" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-dark-400">暂无通知记录</td>
              </tr>
              <tr v-for="item in deliveries" :key="item.id">
                <td class="px-4 py-4 text-sm text-gray-900 dark:text-gray-100">{{ item.id }}</td>
                <td class="px-4 py-4"><span class="badge badge-gray">{{ item.channel }}</span></td>
                <td class="px-4 py-4 text-sm text-gray-900 dark:text-gray-100">
                  <div>{{ item.event_type }}</div>
                  <div class="text-xs text-gray-500 dark:text-dark-400">#{{ item.event_id }}</div>
                </td>
                <td class="px-4 py-4 text-sm text-gray-900 dark:text-gray-100">{{ item.supplier_id || '-' }}</td>
                <td class="px-4 py-4">
                  <span class="badge" :class="statusClass(item.status)">{{ item.status }}</span>
                  <p v-if="item.last_error" class="mt-1 max-w-[260px] text-xs text-rose-600 dark:text-rose-400">{{ item.last_error }}</p>
                </td>
                <td class="px-4 py-4 font-mono text-xs text-gray-500 dark:text-dark-400">{{ item.dedupe_key }}</td>
                <td class="px-4 py-4 text-sm text-gray-500 dark:text-dark-400">
                  <div>创建：{{ formatDateTime(item.created_at) }}</div>
                  <div v-if="item.sent_at">发送：{{ formatDateTime(item.sent_at) }}</div>
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
import { listNotificationDeliveries, type NotificationDelivery } from '@/api/admin/adminPlus'

const appStore = useAppStore()
const loading = ref(false)
const deliveries = ref<NotificationDelivery[]>([])
const pagination = reactive({ page: 1, page_size: getPersistedPageSize(), total: 0, pages: 0 })

const succeededCount = computed(() => deliveries.value.filter((item) => item.status === 'succeeded').length)
const failedCount = computed(() => deliveries.value.filter((item) => item.status === 'failed').length)
const sendingCount = computed(() => deliveries.value.filter((item) => item.status === 'sending').length)

function statusClass(status: NotificationDelivery['status']): string {
  if (status === 'succeeded') return 'badge-success'
  if (status === 'failed') return 'badge-danger'
  return 'badge-warning'
}

function formatDateTime(value?: string | null): string {
  if (!value) return '-'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '-' : date.toLocaleString()
}

async function loadPage() {
  loading.value = true
  try {
    const result = await listNotificationDeliveries({
      page: pagination.page,
      page_size: pagination.page_size
    })
    deliveries.value = result.items
    pagination.total = result.total || 0
    pagination.pages = result.pages || 0
    pagination.page = result.page || pagination.page
    pagination.page_size = result.page_size || pagination.page_size
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '加载通知记录失败')
  } finally {
    loading.value = false
  }
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

onMounted(loadPage)
</script>
