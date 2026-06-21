<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">后端采集调度</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
            由 Admin Plus 后端 Provider Adapter 执行分组、费率、余额、公告、健康和用量消耗采集。
          </p>
        </div>
        <button type="button" class="btn btn-secondary" :disabled="loading" @click="loadPage">
          <Icon name="refresh" size="sm" />
          刷新
        </button>
      </section>

      <section class="grid gap-4 sm:grid-cols-2 xl:grid-cols-5">
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">周期调度</p>
          <p class="mt-2 text-2xl font-semibold" :class="status?.enabled ? 'text-emerald-600 dark:text-emerald-400' : 'text-gray-900 dark:text-white'">
            {{ status?.enabled ? '已开启' : '已关闭' }}
          </p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">间隔</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ intervalLabel }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">可执行</p>
          <p class="mt-2 text-2xl font-semibold text-primary-600 dark:text-primary-400">{{ eligibleCount }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">已同步</p>
          <p class="mt-2 text-2xl font-semibold text-sky-600 dark:text-sky-400">{{ syncedCount }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">跳过</p>
          <p class="mt-2 text-2xl font-semibold text-amber-600 dark:text-amber-400">{{ skippedCount }}</p>
        </div>
      </section>

      <section class="grid gap-6 xl:grid-cols-[420px_minmax(0,1fr)]">
        <form class="card p-5" @submit.prevent="submitRun">
          <div class="flex items-start justify-between gap-3">
            <div>
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">执行调度</h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">业务采集只走后端服务，不写入插件业务任务。</p>
            </div>
            <span class="badge badge-gray">{{ status?.queue || 'backend' }}</span>
          </div>

          <div class="mt-5 space-y-4">
            <label class="block">
              <span class="input-label">供应商</span>
              <select v-model.number="form.supplier_id" class="input" @change="runDiagnosis">
                <option :value="0">全部供应商</option>
                <option v-for="supplier in suppliers" :key="supplier.id" :value="supplier.id">
                  {{ supplier.name }}
                </option>
              </select>
            </label>

            <label class="block">
              <span class="input-label">窗口分钟</span>
              <input v-model.number="form.window_minutes" type="number" min="1" max="1440" class="input" />
            </label>

            <div>
              <span class="input-label">采集类型</span>
              <div class="mt-2 grid gap-2">
                <label v-for="option in taskTypeOptions" :key="option.value" class="flex items-center justify-between rounded-lg border border-gray-200 px-3 py-2 text-sm dark:border-dark-700">
                  <span>
                    <span class="block text-gray-700 dark:text-gray-200">{{ option.label }}</span>
                    <span class="text-xs text-gray-500 dark:text-dark-400">{{ option.description }}</span>
                  </span>
                  <input v-model="form.task_types" :value="option.value" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
                </label>
              </div>
            </div>

            <button type="button" class="btn btn-secondary w-full" :disabled="diagnosing || form.task_types.length === 0" @click="runDiagnosis">
              {{ diagnosing ? '预检中...' : '预检供应商' }}
            </button>
            <button type="submit" class="btn btn-primary w-full" :disabled="running || form.task_types.length === 0">
              {{ running ? '执行中...' : '执行调度' }}
            </button>
          </div>
        </form>

        <div class="card overflow-hidden">
          <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
            <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ lastRun ? '执行结果' : '预检结果' }}</h2>
              <span class="text-xs text-gray-500 dark:text-dark-400">{{ runTimeLabel || diagnosisTimeLabel }}</span>
            </div>
          </div>
          <div class="overflow-x-auto">
            <table class="w-full min-w-[920px] divide-y divide-gray-200 dark:divide-dark-700">
              <thead class="bg-gray-50 dark:bg-dark-800">
                <tr>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">供应商</th>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">采集</th>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">动作</th>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">状态</th>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">幂等键</th>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">结果</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-900">
                <tr v-if="visibleRunItems.length === 0">
                  <td colspan="6" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-dark-400">暂无预检结果</td>
                </tr>
                <tr v-for="item in visibleRunItems" :key="`${item.supplier_id}-${item.task_type}-${item.schedule_key}-${item.reason}`">
                  <td class="px-4 py-4 text-sm text-gray-900 dark:text-gray-100">{{ item.supplier_name || supplierName(item.supplier_id) }}</td>
                  <td class="px-4 py-4"><span class="badge badge-gray">{{ taskTypeLabel(item.task_type) }}</span></td>
                  <td class="px-4 py-4 text-sm text-gray-500 dark:text-dark-400">{{ actionLabel(item.action) }}</td>
                  <td class="px-4 py-4">
                    <span class="badge" :class="runItemClass(item)">{{ runItemStatus(item) }}</span>
                  </td>
                  <td class="px-4 py-4 font-mono text-xs text-gray-500 dark:text-dark-400">{{ item.schedule_key || '-' }}</td>
                  <td class="px-4 py-4 text-sm text-gray-500 dark:text-dark-400">{{ runItemResult(item) }}</td>
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
  getSchedulerStatus,
  listSuppliers,
  runScheduler,
  type ExtensionTaskType,
  type ScheduledTask,
  type SchedulerRun,
  type SchedulerStatus,
  type Supplier
} from '@/api/admin/adminPlus'

const appStore = useAppStore()

const loading = ref(false)
const running = ref(false)
const diagnosing = ref(false)
const suppliers = ref<Supplier[]>([])
const status = ref<SchedulerStatus | null>(null)
const lastRun = ref<SchedulerRun | null>(null)
const diagnosis = ref<SchedulerRun | null>(null)

const taskTypeOptions: Array<{ value: ExtensionTaskType; label: string; description: string }> = [
  { value: 'fetch_groups', label: '分组同步', description: '后端使用供应商会话读取分组' },
  { value: 'fetch_rates', label: '费率同步', description: '后端使用供应商会话读取费率' },
  { value: 'fetch_balance', label: '余额同步', description: '后端使用供应商会话读取余额' },
  { value: 'fetch_announcements', label: '公告同步', description: '后端使用供应商会话读取公告、通知和充值页' },
  { value: 'fetch_health', label: '健康探测', description: '后端使用本地 Sub2API 账号执行 OpenAI-compatible 探测' },
  { value: 'fetch_usage_costs', label: '用量消耗', description: '后端使用供应商会话读取 usage 消耗明细' }
]

const form = reactive({
  supplier_id: 0,
  window_minutes: 10,
  task_types: ['fetch_balance'] as ExtensionTaskType[]
})

const intervalLabel = computed(() => {
  const seconds = status.value?.interval_seconds || 0
  if (seconds <= 0) return '-'
  if (seconds % 60 === 0) return `${seconds / 60} 分钟`
  return `${seconds} 秒`
})
const runTimeLabel = computed(() => formatDateTime(lastRun.value?.requested_at))
const diagnosisTimeLabel = computed(() => formatDateTime(diagnosis.value?.requested_at))
const visibleRunItems = computed(() => lastRun.value?.items || diagnosis.value?.items || [])
const eligibleCount = computed(() => visibleRunItems.value.filter((item) => !item.reason).length)
const syncedCount = computed(() => visibleRunItems.value.filter((item) => item.synced).length)
const skippedCount = computed(() => visibleRunItems.value.filter((item) => item.reason).length)

function supplierName(id: number): string {
  return suppliers.value.find((supplier) => supplier.id === id)?.name || `#${id}`
}

function taskTypeLabel(value: ExtensionTaskType): string {
  return taskTypeOptions.find((option) => option.value === value)?.label || value
}

function reasonLabel(reason?: string): string {
  if (!reason) return '-'
  return {
    duplicate: '同一窗口已存在任务',
    supplier_disabled: '供应商已停用',
    supplier_paused: '供应商已暂停',
    credential_invalid: '凭据失效',
    supplier_url_missing: '缺少后台或 API 地址',
    not_switch_eligible: '无可用余额或不可切换',
    group_syncer_missing: '分组同步未配置',
    rate_syncer_missing: '费率同步未配置',
    balance_syncer_missing: '余额同步未配置',
    announcement_syncer_missing: '公告同步未配置',
    health_syncer_missing: '健康探测未配置',
    usage_cost_syncer_missing: '用量消耗未配置',
    direct_sync_not_supported: '不支持后端直连'
  }[reason] || reason
}

function actionLabel(action?: ScheduledTask['action']): string {
  return {
    direct_sync: '后端同步',
    extension_task: '会话任务',
    compat_task: '兼容兜底'
  }[action || 'direct_sync']
}

function runItemStatus(item: ScheduledTask): string {
  if (item.synced) return '已同步'
  if (item.reason) return '已跳过'
  return '可同步'
}

function runItemClass(item: ScheduledTask): string {
  if (item.synced) return 'badge-success'
  if (item.reason) return 'badge-warning'
  return 'badge-success'
}

function runItemResult(item: ScheduledTask): string {
  if (item.reason) return reasonLabel(item.reason)
  if (item.synced) return `同步 ${item.total || 0} 条`
  return '预检通过'
}

function formatDateTime(value?: string | null): string {
  if (!value) return ''
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '' : date.toLocaleString()
}

async function loadPage() {
  loading.value = true
  try {
    const [supplierResult, schedulerStatus] = await Promise.all([
      listSuppliers(),
      getSchedulerStatus()
    ])
    suppliers.value = supplierResult.items
    status.value = schedulerStatus
    await runDiagnosis()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '加载后端采集调度失败')
  } finally {
    loading.value = false
  }
}

async function runDiagnosis() {
  diagnosing.value = true
  try {
    diagnosis.value = await runScheduler({
      mode: 'diagnose',
      supplier_id: form.supplier_id || undefined,
      task_types: form.task_types,
      window_minutes: Number(form.window_minutes || 10),
      dry_run: true
    })
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '预检供应商失败')
  } finally {
    diagnosing.value = false
  }
}

async function submitRun() {
  running.value = true
  try {
    lastRun.value = await runScheduler({
      mode: 'manual',
      supplier_id: form.supplier_id || undefined,
      task_types: form.task_types,
      window_minutes: Number(form.window_minutes || 10)
    })
    const synced = lastRun.value.items.filter((item) => item.synced).length
    appStore.showSuccess(`已同步 ${synced} 项，跳过 ${lastRun.value.skipped_count} 项`)
    await runDiagnosis()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '执行调度失败')
  } finally {
    running.value = false
  }
}

onMounted(loadPage)
</script>
