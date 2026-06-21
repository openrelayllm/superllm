<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">采集会话</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
            Chrome 插件只负责上报供应商浏览器会话；业务采集由后端 Provider Adapter 执行。
          </p>
        </div>
        <div class="flex flex-wrap gap-2">
          <button type="button" class="btn btn-secondary" :disabled="loading" @click="loadPage">
            <Icon name="refresh" size="sm" />
            刷新
          </button>
          <button type="button" class="btn btn-primary" @click="installDialogOpen = true">
            下载插件
          </button>
        </div>
      </section>

      <section class="grid gap-4 sm:grid-cols-2 xl:grid-cols-5">
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">会话任务</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ taskPagination.total }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">待领取</p>
          <p class="mt-2 text-2xl font-semibold text-amber-600 dark:text-amber-400">{{ pendingTaskCount }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">执行中</p>
          <p class="mt-2 text-2xl font-semibold text-sky-600 dark:text-sky-400">{{ runningTaskCount }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">成功</p>
          <p class="mt-2 text-2xl font-semibold text-emerald-600 dark:text-emerald-400">{{ succeededTaskCount }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">失败</p>
          <p class="mt-2 text-2xl font-semibold text-rose-600 dark:text-rose-400">{{ failedTaskCount }}</p>
        </div>
      </section>

      <section class="card p-5">
        <div class="grid gap-4 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-end">
          <label class="block">
            <span class="input-label">供应商</span>
            <select v-model.number="selectedSupplierId" class="input" @change="handleSupplierChange">
              <option :value="0">请选择供应商</option>
              <option v-for="supplier in suppliers" :key="supplier.id" :value="supplier.id">{{ supplier.name }}</option>
            </select>
          </label>
          <button type="button" class="btn btn-primary" :disabled="creating || !selectedSupplierId" @click="createCaptureTask">
            {{ creating ? '创建中...' : '创建会话上报任务' }}
          </button>
        </div>
      </section>

      <section class="card overflow-hidden">
        <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
          <div class="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
            <div>
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">会话上报任务记录</h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">仅展示 `capture_supplier_session` 任务。</p>
            </div>
            <div class="grid gap-2 sm:grid-cols-2">
              <select v-model.number="taskFilters.supplier_id" class="input h-9 py-1 text-sm" @change="resetTaskPagination">
                <option :value="0">全部供应商</option>
                <option v-for="supplier in suppliers" :key="supplier.id" :value="supplier.id">{{ supplier.name }}</option>
              </select>
              <select v-model="taskFilters.status" class="input h-9 py-1 text-sm" @change="resetTaskPagination">
                <option value="">全部状态</option>
                <option value="pending">待领取</option>
                <option value="claimed">已领取</option>
                <option value="running">执行中</option>
                <option value="succeeded">成功</option>
                <option value="failed">失败</option>
                <option value="cancelled">已取消</option>
              </select>
            </div>
          </div>
        </div>
        <div class="overflow-x-auto">
          <table class="w-full min-w-[1080px] divide-y divide-gray-200 dark:divide-dark-700">
            <thead class="bg-gray-50 dark:bg-dark-800">
              <tr>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">ID</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">供应商</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">状态</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">尝试</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">设备</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">结果/错误</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">更新时间</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-900">
              <tr v-if="tasks.length === 0">
                <td colspan="7" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-dark-400">暂无会话任务</td>
              </tr>
              <tr v-for="task in tasks" :key="task.id">
                <td class="px-4 py-4 font-mono text-sm text-gray-900 dark:text-gray-100">#{{ task.id }}</td>
                <td class="px-4 py-4 text-sm text-gray-900 dark:text-gray-100">
                  <div>{{ supplierName(task.supplier_id) }}</div>
                  <div v-if="task.schedule_key" class="mt-1 max-w-[220px] truncate font-mono text-xs text-gray-400">{{ task.schedule_key }}</div>
                </td>
                <td class="px-4 py-4"><span class="badge" :class="taskStatusClass(task.status)">{{ taskStatusLabel(task.status) }}</span></td>
                <td class="px-4 py-4 text-sm text-gray-500 dark:text-dark-400">{{ task.attempts }} / {{ task.max_attempts }}</td>
                <td class="px-4 py-4 font-mono text-xs text-gray-500 dark:text-dark-400">{{ task.device_id || '-' }}</td>
                <td class="px-4 py-4 text-sm text-gray-500 dark:text-dark-400">
                  <button type="button" class="text-primary-600 hover:text-primary-700 dark:text-primary-400" @click="openTaskDetail(task)">
                    {{ task.error_code || resultSummary(task) }}
                  </button>
                </td>
                <td class="px-4 py-4 text-sm text-gray-500 dark:text-dark-400">{{ formatDateTime(task.updated_at) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <Pagination
          v-if="taskPagination.total > 0"
          :page="taskPagination.page"
          :total="taskPagination.total"
          :page-size="taskPagination.page_size"
          @update:page="handleTaskPageChange"
          @update:pageSize="handleTaskPageSizeChange"
        />
      </section>

      <div v-if="installDialogOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" @click.self="installDialogOpen = false">
        <div class="w-full max-w-xl overflow-hidden rounded-lg bg-white shadow-xl dark:bg-dark-900">
          <div class="flex items-center justify-between border-b border-gray-100 px-5 py-4 dark:border-dark-700">
            <div>
              <h3 class="text-lg font-semibold text-gray-900 dark:text-white">Chrome 插件</h3>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ manifest?.name || 'Sub2API Plus Session Capture' }} {{ manifest?.version ? `v${manifest.version}` : '' }}</p>
            </div>
            <button type="button" class="btn btn-secondary btn-sm" @click="installDialogOpen = false">关闭</button>
          </div>
          <div class="space-y-4 p-5">
            <div class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
              <p class="text-sm font-semibold text-gray-900 dark:text-white">安装流程</p>
              <ol class="mt-3 list-decimal space-y-2 pl-5 text-sm text-gray-600 dark:text-dark-300">
                <li>下载 ZIP 并解压到本地目录。</li>
                <li>打开 <span class="font-mono">chrome://extensions</span>，启用开发者模式。</li>
                <li>选择“加载已解压的扩展程序”，指向解压后的目录。</li>
                <li>在已登录 sub2apiplus 页面连接插件，再到供应商后台上报会话。</li>
              </ol>
            </div>
            <div class="grid gap-3 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-end">
              <label class="block">
                <span class="input-label">sub2apiplus 地址</span>
                <input :value="adminPlusOrigin" readonly class="input font-mono text-xs" />
              </label>
              <button type="button" class="btn btn-secondary" @click="copyText(adminPlusOrigin, '后台地址已复制')">复制地址</button>
            </div>
            <button type="button" class="btn btn-primary w-full" :disabled="packageDownloading" @click="downloadPackage">
              {{ packageDownloading ? '下载中...' : '下载插件包' }}
            </button>
          </div>
        </div>
      </div>

      <div v-if="detailTask" class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" @click.self="detailTask = null">
        <div class="max-h-[80vh] w-full max-w-3xl overflow-hidden rounded-lg bg-white shadow-xl dark:bg-dark-900">
          <div class="flex items-center justify-between border-b border-gray-100 px-5 py-4 dark:border-dark-700">
            <h3 class="text-lg font-semibold text-gray-900 dark:text-white">任务 #{{ detailTask.id }}</h3>
            <button type="button" class="btn btn-secondary btn-sm" @click="detailTask = null">关闭</button>
          </div>
          <div class="max-h-[65vh] overflow-auto p-5">
            <pre class="whitespace-pre-wrap rounded-lg bg-gray-50 p-4 text-xs text-gray-700 dark:bg-dark-800 dark:text-gray-200">{{ taskDetailJSON }}</pre>
          </div>
        </div>
      </div>
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
  createExtensionTask,
  downloadExtensionPackage,
  getExtensionManifest,
  listExtensionTasks,
  listSuppliers,
  type ExtensionManifestInfo,
  type ExtensionTask,
  type Supplier
} from '@/api/admin/adminPlus'

const appStore = useAppStore()

const loading = ref(false)
const creating = ref(false)
const packageDownloading = ref(false)
const installDialogOpen = ref(false)
const suppliers = ref<Supplier[]>([])
const tasks = ref<ExtensionTask[]>([])
const manifest = ref<ExtensionManifestInfo | null>(null)
const detailTask = ref<ExtensionTask | null>(null)
const selectedSupplierId = ref(0)

const taskPagination = reactive({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0,
  pages: 0
})

const taskFilters = reactive({
  supplier_id: 0,
  status: ''
})

const adminPlusOrigin = computed(() => (typeof window === 'undefined' ? '' : window.location.origin))
const pendingTaskCount = computed(() => tasks.value.filter((task) => task.status === 'pending').length)
const runningTaskCount = computed(() => tasks.value.filter((task) => ['claimed', 'running'].includes(task.status)).length)
const succeededTaskCount = computed(() => tasks.value.filter((task) => task.status === 'succeeded').length)
const failedTaskCount = computed(() => tasks.value.filter((task) => task.status === 'failed').length)
const taskDetailJSON = computed(() => JSON.stringify(detailTask.value, null, 2))

function supplierName(id: number): string {
  return suppliers.value.find((supplier) => supplier.id === id)?.name || `#${id}`
}

function taskStatusLabel(statusValue: ExtensionTask['status']): string {
  return {
    pending: '待领取',
    claimed: '已领取',
    running: '执行中',
    succeeded: '成功',
    failed: '失败',
    cancelled: '已取消'
  }[statusValue] || statusValue
}

function taskStatusClass(statusValue: ExtensionTask['status']): string {
  if (statusValue === 'succeeded') return 'badge-success'
  if (statusValue === 'failed' || statusValue === 'cancelled') return 'badge-danger'
  if (['pending', 'claimed', 'running'].includes(statusValue)) return 'badge-warning'
  return 'badge-gray'
}

function formatDateTime(value?: string | null): string {
  if (!value) return ''
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '' : date.toLocaleString()
}

function resultSummary(task: ExtensionTask): string {
  if (task.status === 'succeeded') {
    const summary = task.result?.session_summary as Record<string, unknown> | undefined
    if (summary) return `会话: ${summary.cookie_count || 0} cookies`
    if (task.result) return '查看结果'
  }
  if (task.status === 'failed') return task.error_message || '查看错误'
  return '查看详情'
}

function openTaskDetail(task: ExtensionTask) {
  detailTask.value = task
}

async function copyText(value: string, message: string) {
  await navigator.clipboard.writeText(value)
  appStore.showSuccess(message)
}

async function loadPage() {
  loading.value = true
  try {
    const [supplierResult, extensionManifest] = await Promise.all([
      listSuppliers(),
      getExtensionManifest()
    ])
    suppliers.value = supplierResult.items
    manifest.value = extensionManifest
    if (!selectedSupplierId.value && suppliers.value[0]) {
      selectedSupplierId.value = suppliers.value[0].id
    }
    await loadTasks()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '加载采集会话失败')
  } finally {
    loading.value = false
  }
}

async function loadTasks() {
  const taskResult = await listExtensionTasks({
    supplier_id: taskFilters.supplier_id || undefined,
    status: taskFilters.status || undefined,
    type: 'capture_supplier_session',
    page: taskPagination.page,
    page_size: taskPagination.page_size
  })
  tasks.value = taskResult.items
  taskPagination.total = taskResult.total || 0
  taskPagination.pages = taskResult.pages || 0
  taskPagination.page = taskResult.page || taskPagination.page
  taskPagination.page_size = taskResult.page_size || taskPagination.page_size
}

function handleSupplierChange() {
  taskFilters.supplier_id = selectedSupplierId.value
  resetTaskPagination()
}

function resetTaskPagination() {
  taskPagination.page = 1
  void loadTasks()
}

function handleTaskPageChange(page: number) {
  taskPagination.page = page
  void loadTasks()
}

function handleTaskPageSizeChange(pageSize: number) {
  taskPagination.page_size = pageSize
  taskPagination.page = 1
  void loadTasks()
}

async function createCaptureTask() {
  if (!selectedSupplierId.value) {
    appStore.showError('请选择供应商')
    return
  }
  creating.value = true
  try {
    await createExtensionTask({
      supplier_id: selectedSupplierId.value,
      type: 'capture_supplier_session',
      priority: 95,
      max_attempts: 3,
      payload: {
        source: 'admin_console',
        task_type: 'capture_supplier_session'
      }
    })
    appStore.showSuccess('会话上报任务已创建')
    taskFilters.supplier_id = selectedSupplierId.value
    resetTaskPagination()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '创建会话上报任务失败')
  } finally {
    creating.value = false
  }
}

async function downloadPackage() {
  packageDownloading.value = true
  try {
    const blob = await downloadExtensionPackage(adminPlusOrigin.value)
    const version = manifest.value?.version || 'dev'
    const url = URL.createObjectURL(blob)
    const anchor = document.createElement('a')
    anchor.href = url
    anchor.download = `sub2api-plus-session-capture-${version}.zip`
    document.body.appendChild(anchor)
    anchor.click()
    anchor.remove()
    URL.revokeObjectURL(url)
    appStore.showSuccess('插件包已下载')
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '下载插件包失败')
  } finally {
    packageDownloading.value = false
  }
}

onMounted(loadPage)
</script>
