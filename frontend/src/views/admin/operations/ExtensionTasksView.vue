<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">插件任务</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
            为 Chrome 插件下发登录采集、费率抓取、余额抓取、优惠抓取和账单导出任务。
          </p>
        </div>
        <div class="flex flex-wrap gap-2">
          <button type="button" class="btn btn-secondary" :disabled="loading" @click="loadPage">
            <Icon name="refresh" size="sm" />
            刷新
          </button>
          <button type="button" class="btn btn-primary" @click="claimTask">本机领取</button>
        </div>
      </section>

      <section class="grid gap-4 md:grid-cols-4">
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">待领取</p>
          <p class="mt-2 text-2xl font-semibold text-amber-600 dark:text-amber-400">{{ pendingCount }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">执行中</p>
          <p class="mt-2 text-2xl font-semibold text-sky-600 dark:text-sky-400">{{ runningCount }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">成功</p>
          <p class="mt-2 text-2xl font-semibold text-emerald-600 dark:text-emerald-400">{{ succeededCount }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">失败</p>
          <p class="mt-2 text-2xl font-semibold text-rose-600 dark:text-rose-400">{{ failedCount }}</p>
        </div>
      </section>

      <section class="grid gap-6 xl:grid-cols-[420px_minmax(0,1fr)]">
        <form class="card p-5" @submit.prevent="createTask">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">创建任务</h2>
          <div class="mt-5 space-y-4">
            <label class="block">
              <span class="input-label">供应商</span>
              <select v-model.number="form.supplier_id" class="input" required>
                <option :value="0" disabled>请选择</option>
                <option v-for="supplier in suppliers" :key="supplier.id" :value="supplier.id">{{ supplier.name }}</option>
              </select>
            </label>
            <label class="block">
              <span class="input-label">任务类型</span>
              <select v-model="form.type" class="input">
                <option value="fetch_rates">抓取费率</option>
                <option value="fetch_balance">抓取余额</option>
                <option value="fetch_promotions">抓取优惠</option>
                <option value="export_bills">导出账单</option>
                <option value="fetch_health">抓取健康</option>
              </select>
            </label>
            <div class="grid gap-4 sm:grid-cols-2">
              <label class="block">
                <span class="input-label">优先级</span>
                <input v-model.number="form.priority" type="number" class="input" />
              </label>
              <label class="block">
                <span class="input-label">最大尝试</span>
                <input v-model.number="form.max_attempts" type="number" min="1" class="input" />
              </label>
            </div>
            <button type="submit" class="btn btn-primary w-full" :disabled="submitting">
              {{ submitting ? '创建中...' : '创建插件任务' }}
            </button>
          </div>
        </form>

        <div class="card overflow-hidden">
          <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">任务队列</h2>
          </div>
          <div class="overflow-x-auto">
            <table class="w-full min-w-[920px] divide-y divide-gray-200 dark:divide-dark-700">
              <thead class="bg-gray-50 dark:bg-dark-800">
                <tr>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">ID</th>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">供应商</th>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">类型</th>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">状态</th>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">设备</th>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">更新时间</th>
                  <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">操作</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-900">
                <tr v-if="tasks.length === 0">
                  <td colspan="7" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-dark-400">暂无任务</td>
                </tr>
                <tr v-for="task in tasks" :key="task.id">
                  <td class="px-4 py-4 text-sm text-gray-900 dark:text-gray-100">{{ task.id }}</td>
                  <td class="px-4 py-4 text-sm text-gray-900 dark:text-gray-100">{{ supplierName(task.supplier_id) }}</td>
                  <td class="px-4 py-4"><span class="badge badge-gray">{{ task.type }}</span></td>
                  <td class="px-4 py-4"><span class="badge" :class="statusClass(task.status)">{{ task.status }}</span></td>
                  <td class="px-4 py-4 font-mono text-xs text-gray-500 dark:text-dark-400">{{ task.device_id || '-' }}</td>
                  <td class="px-4 py-4 text-sm text-gray-500 dark:text-dark-400">{{ formatDateTime(task.updated_at) }}</td>
                  <td class="px-4 py-4 text-right">
                    <div class="flex justify-end gap-2">
                      <button type="button" class="btn btn-secondary px-3 py-1.5 text-xs" :disabled="!canOperate(task)" @click="completeTask(task)">
                        完成
                      </button>
                      <button type="button" class="btn btn-danger px-3 py-1.5 text-xs" :disabled="!canOperate(task)" @click="failTask(task)">
                        失败
                      </button>
                    </div>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import {
  claimExtensionTask,
  completeExtensionTask,
  createExtensionTask,
  failExtensionTask,
  listExtensionTasks,
  listSuppliers,
  type ExtensionTask,
  type Supplier
} from '@/api/admin/adminPlus'

const appStore = useAppStore()

const loading = ref(false)
const submitting = ref(false)
const suppliers = ref<Supplier[]>([])
const tasks = ref<ExtensionTask[]>([])

const form = reactive({
  supplier_id: 0,
  type: 'fetch_rates' as ExtensionTask['type'],
  priority: 10,
  max_attempts: 3
})

const pendingCount = computed(() => tasks.value.filter((task) => task.status === 'pending').length)
const runningCount = computed(() => tasks.value.filter((task) => ['claimed', 'running'].includes(task.status)).length)
const succeededCount = computed(() => tasks.value.filter((task) => task.status === 'succeeded').length)
const failedCount = computed(() => tasks.value.filter((task) => task.status === 'failed').length)

function supplierName(id: number): string {
  return suppliers.value.find((supplier) => supplier.id === id)?.name || `#${id}`
}

function formatDateTime(value: string): string {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '-' : date.toLocaleString()
}

function statusClass(status: ExtensionTask['status']): string {
  if (status === 'succeeded') return 'badge-success'
  if (status === 'failed' || status === 'cancelled') return 'badge-danger'
  if (['pending', 'claimed', 'running'].includes(status)) return 'badge-warning'
  return 'badge-gray'
}

function deviceID(): string {
  const key = 'admin_plus_device_id'
  const existing = localStorage.getItem(key)
  if (existing) return existing
  const value = `admin-plus-ui-${Math.random().toString(36).slice(2, 10)}`
  localStorage.setItem(key, value)
  return value
}

function canOperate(task: ExtensionTask): boolean {
  return !!task.device_id && !!task.lease_token && ['claimed', 'running'].includes(task.status)
}

async function loadPage() {
  loading.value = true
  try {
    const [supplierResult, taskResult] = await Promise.all([
      listSuppliers(),
      listExtensionTasks({ limit: 100 })
    ])
    suppliers.value = supplierResult.items
    tasks.value = taskResult.items
    if (!form.supplier_id && suppliers.value[0]) {
      form.supplier_id = suppliers.value[0].id
    }
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '加载插件任务失败')
  } finally {
    loading.value = false
  }
}

async function createTask() {
  submitting.value = true
  try {
    await createExtensionTask({
      supplier_id: form.supplier_id,
      type: form.type,
      priority: Number(form.priority || 0),
      max_attempts: Number(form.max_attempts || 3)
    })
    appStore.showSuccess('任务已创建')
    await loadPage()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '创建任务失败')
  } finally {
    submitting.value = false
  }
}

async function claimTask() {
  try {
    await claimExtensionTask({ device_id: deviceID(), lease_ttl_seconds: 300 })
    appStore.showSuccess('任务已领取')
    await loadPage()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '领取任务失败')
  }
}

async function completeTask(task: ExtensionTask) {
  try {
    await completeExtensionTask(task, { completed_from: 'admin-plus-ui' })
    appStore.showSuccess('任务已完成')
    await loadPage()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '完成任务失败')
  }
}

async function failTask(task: ExtensionTask) {
  try {
    await failExtensionTask(task, 'MANUAL_FAIL', 'Marked failed from Admin Plus UI')
    appStore.showSuccess('任务已标记失败')
    await loadPage()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '标记任务失败')
  }
}

onMounted(loadPage)
</script>
