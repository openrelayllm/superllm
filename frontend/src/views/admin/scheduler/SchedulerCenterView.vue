<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">调度中心</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
            统一管理供应商采集、对账、会话维护、渠道检测和本地调度联动。
          </p>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <button type="button" class="btn btn-secondary" :disabled="loading" @click="loadPage">
            <Icon name="refresh" size="sm" />
            刷新
          </button>
          <button type="button" class="btn btn-primary" :disabled="running" @click="runBalanceSync">
            <Icon name="play" size="sm" />
            {{ running ? '运行中...' : '刷新余额' }}
          </button>
        </div>
      </section>

      <nav class="flex gap-2 overflow-x-auto border-b border-gray-200 dark:border-dark-700">
        <button
          v-for="tab in tabs"
          :key="tab.value"
          type="button"
          class="whitespace-nowrap border-b-2 px-3 py-2 text-sm font-medium"
          :class="activeTab === tab.value ? 'border-primary-500 text-primary-600 dark:text-primary-400' : 'border-transparent text-gray-500 hover:text-gray-900 dark:text-dark-400 dark:hover:text-white'"
          @click="activeTab = tab.value"
        >
          {{ tab.label }}
        </button>
      </nav>

      <section v-if="activeTab === 'dashboard'" class="grid gap-6 xl:grid-cols-[minmax(0,1.7fr)_minmax(320px,0.8fr)]">
        <div class="space-y-6">
          <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
            <div class="card p-4">
              <p class="text-xs font-medium text-gray-500 dark:text-dark-400">Worker</p>
              <p class="mt-2 text-2xl font-semibold" :class="statusClass(status?.worker_status)">
                {{ workerLabel }}
              </p>
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ intervalLabel }}</p>
            </div>
            <div class="card p-4">
              <p class="text-xs font-medium text-gray-500 dark:text-dark-400">运行中</p>
              <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ status?.running_steps || 0 }}</p>
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">当前 step</p>
            </div>
            <div class="card p-4">
              <p class="text-xs font-medium text-gray-500 dark:text-dark-400">失败</p>
              <p class="mt-2 text-2xl font-semibold text-rose-600 dark:text-rose-400">{{ status?.failed_steps || 0 }}</p>
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">最近 run 聚合</p>
            </div>
            <div class="card p-4">
              <p class="text-xs font-medium text-gray-500 dark:text-dark-400">待处理</p>
              <p class="mt-2 text-2xl font-semibold text-amber-600 dark:text-amber-400">{{ actions.length }}</p>
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">智能动作</p>
            </div>
          </div>

          <div class="card overflow-hidden">
            <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">今日工作台</h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">按优先级处理供应商自动化风险。</p>
            </div>
            <div class="divide-y divide-gray-100 dark:divide-dark-700">
              <div v-if="actions.length === 0" class="px-5 py-10 text-center text-sm text-gray-500 dark:text-dark-400">
                暂无待处理动作。
              </div>
              <div v-for="action in actions.slice(0, 6)" :key="action.id" class="flex flex-col gap-3 px-5 py-4 md:flex-row md:items-center md:justify-between">
                <div class="min-w-0">
                  <div class="flex flex-wrap items-center gap-2">
                    <span class="badge" :class="severityClass(action.severity)">{{ severityLabel(action.severity) }}</span>
                    <span class="text-sm font-medium text-gray-900 dark:text-white">{{ action.title }}</span>
                  </div>
                  <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ action.supplier_name || '-' }} · {{ action.reason }}</p>
                </div>
                <button type="button" class="btn btn-secondary shrink-0" @click="activeTab = 'actions'">
                  {{ action.recommended_operation || '查看' }}
                </button>
              </div>
            </div>
          </div>

          <div class="card overflow-hidden">
            <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">最近运行</h2>
            </div>
            <div class="overflow-x-auto">
              <table class="w-full min-w-[760px] divide-y divide-gray-200 dark:divide-dark-700">
                <thead class="bg-gray-50 dark:bg-dark-800">
                  <tr>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">任务</th>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">状态</th>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">供应商</th>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">Step</th>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">时间</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-100 bg-white dark:divide-dark-700 dark:bg-dark-900">
                  <tr v-if="runs.length === 0">
                    <td colspan="5" class="px-4 py-8 text-center text-sm text-gray-500 dark:text-dark-400">暂无运行记录</td>
                  </tr>
                  <tr v-for="run in runs.slice(0, 5)" :key="run.id">
                    <td class="px-4 py-3 text-sm text-gray-900 dark:text-gray-100">{{ taskLabel(run.task_type) }}</td>
                    <td class="px-4 py-3"><span class="badge" :class="runStatusClass(run.status)">{{ runStatusLabel(run.status) }}</span></td>
                    <td class="px-4 py-3 text-sm text-gray-500 dark:text-dark-400">{{ run.supplier_count }}</td>
                    <td class="px-4 py-3 text-sm text-gray-500 dark:text-dark-400">{{ run.succeeded_steps }}/{{ run.total_steps }}</td>
                    <td class="px-4 py-3 text-sm text-gray-500 dark:text-dark-400">{{ runPrimaryTime(run) }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>

        <aside class="space-y-6">
          <div class="card p-5">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">系统健康</h2>
            <dl class="mt-4 space-y-3 text-sm">
              <div class="flex items-center justify-between">
                <dt class="text-gray-500 dark:text-dark-400">队列</dt>
                <dd class="font-medium text-gray-900 dark:text-white">{{ status?.queue || '-' }}</dd>
              </div>
              <div class="flex items-center justify-between">
                <dt class="text-gray-500 dark:text-dark-400">下次运行</dt>
                <dd class="font-medium text-gray-900 dark:text-white">{{ nextRunLabel }}</dd>
              </div>
              <div class="flex items-center justify-between">
                <dt class="text-gray-500 dark:text-dark-400">最近运行</dt>
                <dd class="font-medium text-gray-900 dark:text-white">{{ formatDateTime(status?.last_run_at) || '-' }}</dd>
              </div>
              <div class="flex items-center justify-between">
                <dt class="text-gray-500 dark:text-dark-400">渠道检测</dt>
                <dd class="font-medium" :class="settings?.channel_checks_enabled ? 'text-emerald-600 dark:text-emerald-400' : 'text-gray-500 dark:text-dark-400'">
                  {{ settings?.channel_checks_enabled ? '已启用' : '默认关闭' }}
                </dd>
              </div>
            </dl>
          </div>

          <div class="card p-5">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">供应商自动化</h2>
            <div class="mt-4 space-y-3">
              <div v-for="item in supplierStatuses.slice(0, 5)" :key="item.supplier_id">
                <div class="flex items-center justify-between text-sm">
                  <span class="font-medium text-gray-900 dark:text-white">{{ item.supplier_name }}</span>
                  <span class="text-gray-500 dark:text-dark-400">{{ item.completion_percent }}%</span>
                </div>
                <div class="mt-2 h-2 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-800">
                  <div class="h-full bg-primary-500" :style="{ width: `${item.completion_percent}%` }"></div>
                </div>
              </div>
            </div>
            <button type="button" class="btn btn-secondary mt-5 w-full" @click="activeTab = 'suppliers'">
              查看 Checklist
            </button>
          </div>
        </aside>
      </section>

      <SchedulerPlansPanel
        v-else-if="activeTab === 'plans'"
        :plans="plans"
        :running="running"
        :updating-plan-id="updatingPlanId"
        @run="runPlan"
        @status="setPlanStatus"
        @save="savePlanConfig"
      />

      <section v-else-if="activeTab === 'runs'" class="card overflow-hidden">
        <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">运行记录</h2>
        </div>
        <div class="overflow-x-auto">
          <table class="w-full min-w-[1260px] divide-y divide-gray-200 dark:divide-dark-700">
            <thead class="bg-gray-50 dark:bg-dark-800">
              <tr>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">Run</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">任务</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">触发</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">状态</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">Step</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">时间</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">耗时</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">错误</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">操作</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 bg-white dark:divide-dark-700 dark:bg-dark-900">
              <tr v-if="runs.length === 0">
                <td colspan="9" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-dark-400">暂无运行记录。点击“刷新余额”会创建一条真实 run。</td>
              </tr>
              <tr v-for="run in runs" :key="run.id">
                <td class="px-4 py-4 font-mono text-xs text-gray-500 dark:text-dark-400">{{ run.id }}</td>
                <td class="px-4 py-4 text-sm text-gray-900 dark:text-gray-100">{{ taskLabel(run.task_type) }}</td>
                <td class="px-4 py-4 text-sm text-gray-500 dark:text-dark-400">{{ run.trigger_type }}</td>
                <td class="px-4 py-4"><span class="badge" :class="runStatusClass(run.status)">{{ runStatusLabel(run.status) }}</span></td>
                <td class="px-4 py-4 text-sm text-gray-500 dark:text-dark-400">{{ run.succeeded_steps }}/{{ run.total_steps }} 成功，{{ run.failed_steps }} 失败</td>
                <td class="px-4 py-4 text-sm text-gray-500 dark:text-dark-400">
                  <div>请求 {{ runPrimaryTime(run) }}</div>
                  <div v-if="run.started_at" class="mt-1 text-xs text-gray-400 dark:text-dark-500">开始 {{ formatDateTime(run.started_at) || '-' }}</div>
                  <div v-if="run.finished_at" class="mt-1 text-xs text-gray-400 dark:text-dark-500">完成 {{ formatDateTime(run.finished_at) || '-' }}</div>
                </td>
                <td class="px-4 py-4 text-sm text-gray-500 dark:text-dark-400">{{ run.duration_ms }} ms</td>
                <td class="px-4 py-4 text-sm text-gray-500 dark:text-dark-400">{{ run.error_message || '-' }}</td>
                <td class="px-4 py-4">
                  <div class="flex flex-wrap gap-2">
                    <button type="button" class="btn btn-secondary btn-sm" @click="openRunDetail(run)">详情</button>
                    <button
                      type="button"
                      class="btn btn-secondary btn-sm"
                      :disabled="updatingRunId === run.id || !runRetryable(run.status, run.failed_steps)"
                      @click="retryRunFailedSteps(run)"
                    >
                      重试失败
                    </button>
                    <button
                      type="button"
                      class="btn btn-secondary btn-sm"
                      :disabled="updatingRunId === run.id || !runCancellable(run.status)"
                      @click="cancelRun(run)"
                    >
                      取消
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <SchedulerSupplierAutomationPanel
        v-else-if="activeTab === 'suppliers'"
        :suppliers="supplierStatuses"
        :running-action-key="runningSupplierActionKey"
        @action="runSupplierAutomationAction"
        @checklist="openSupplierChecklist"
      />

      <section v-else-if="activeTab === 'actions'" class="card overflow-hidden">
        <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">智能动作</h2>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">按状态机单步处理异常供应商事项。</p>
        </div>
        <div class="divide-y divide-gray-100 dark:divide-dark-700">
          <div v-if="actions.length === 0" class="px-5 py-10 text-center text-sm text-gray-500 dark:text-dark-400">暂无智能动作</div>
          <div v-for="action in actions" :key="action.id" class="grid gap-4 px-5 py-4 lg:grid-cols-[180px_minmax(0,1fr)_260px] lg:items-center">
            <div>
              <span class="badge" :class="severityClass(action.severity)">{{ severityLabel(action.severity) }}</span>
              <p class="mt-2 text-xs text-gray-500 dark:text-dark-400">{{ actionStatusLabel(action.status) }}</p>
            </div>
            <div>
              <p class="font-medium text-gray-900 dark:text-white">{{ action.title }}</p>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ action.supplier_name || '-' }} · {{ action.reason }}</p>
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ action.recommended_operation || '查看证据' }}</p>
            </div>
            <div class="flex flex-wrap justify-start gap-2 lg:justify-end">
              <button type="button" class="btn btn-secondary btn-sm" :disabled="updatingActionId === action.id" @click="setActionStatus(action, 'investigating')">处理中</button>
              <button type="button" class="btn btn-secondary btn-sm" :disabled="updatingActionId === action.id" @click="setActionStatus(action, 'ignored')">忽略</button>
              <button type="button" class="btn btn-primary btn-sm" :disabled="updatingActionId === action.id" @click="setActionStatus(action, 'resolved')">标记处理</button>
            </div>
          </div>
        </div>
      </section>

      <section v-else class="grid gap-6 lg:grid-cols-2">
        <div class="card p-5">
          <div class="flex items-center justify-between gap-3">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">全局调度</h2>
            <button type="button" class="btn btn-primary btn-sm" :disabled="settingsSaving" @click="saveSettings">
              {{ settingsSaving ? '保存中...' : '保存' }}
            </button>
          </div>
          <div class="mt-4 space-y-4 text-sm">
            <label class="flex items-center justify-between gap-4">
              <span class="text-gray-500 dark:text-dark-400">调度中心</span>
              <input v-model="settingsForm.enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
            </label>
            <label class="flex items-center justify-between gap-4">
              <span class="text-gray-500 dark:text-dark-400">渠道检测</span>
              <input v-model="settingsForm.channel_checks_enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
            </label>
            <label class="block">
              <span class="text-gray-500 dark:text-dark-400">单供应商并发</span>
              <input v-model.number="settingsForm.default_supplier_concurrency" type="number" min="1" max="20" class="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-800 dark:text-white" />
            </label>
            <label class="block">
              <span class="text-gray-500 dark:text-dark-400">渠道检测每日 token 预算</span>
              <input v-model.number="settingsForm.channel_check_daily_budget_tokens" type="number" min="0" class="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-800 dark:text-white" />
            </label>
            <label class="block">
              <span class="text-gray-500 dark:text-dark-400">首 token 慢阈值 ms</span>
              <input v-model.number="settingsForm.first_token_slow_threshold_ms" type="number" min="0" class="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-800 dark:text-white" />
            </label>
            <label class="block">
              <span class="text-gray-500 dark:text-dark-400">总耗时慢阈值 ms</span>
              <input v-model.number="settingsForm.total_latency_slow_threshold_ms" type="number" min="0" class="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-800 dark:text-white" />
            </label>
          </div>
        </div>

        <div class="card p-5">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">默认任务</h2>
          <div class="mt-4 flex flex-wrap gap-2">
            <span v-for="task in settingsForm.default_enabled_task_types" :key="task" class="badge badge-gray">
              {{ taskLabel(task) }}
            </span>
          </div>
          <h3 class="mt-6 text-sm font-semibold text-gray-900 dark:text-white">高成本任务</h3>
          <div class="mt-3 flex flex-wrap gap-2">
            <span v-for="task in settingsForm.high_cost_task_types" :key="task" class="badge badge-warning">
              {{ taskLabel(task) }}
            </span>
          </div>
        </div>
      </section>
    </div>
    <SchedulerRunDetailDialog
      :show="runDetailOpen"
      :detail="runDetail"
      :loading="runDetailLoading"
      :retrying-step-id="retryingStepId"
      :cancelling-step-id="cancellingStepId"
      @close="closeRunDetail"
      @retry-step="retryStep"
      @cancel-step="cancelStep"
      @refresh="refreshOpenRunDetail(true)"
    />
    <SchedulerSupplierChecklistDialog
      :show="supplierChecklistOpen"
      :checklist="supplierChecklist"
      :loading="supplierChecklistLoading"
      :running-action-key="runningSupplierActionKey"
      @close="closeSupplierChecklist"
      @action="runSupplierChecklistAction"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import SchedulerPlansPanel from './SchedulerPlansPanel.vue'
import SchedulerRunDetailDialog from './SchedulerRunDetailDialog.vue'
import SchedulerSupplierChecklistDialog from './SchedulerSupplierChecklistDialog.vue'
import SchedulerSupplierAutomationPanel from './SchedulerSupplierAutomationPanel.vue'
import { useAppStore } from '@/stores/app'
import {
  cancelSchedulerRun,
  cancelSchedulerStep,
  createSchedulerRun,
  getSchedulerCenterStatus,
  getSchedulerSettings,
  getSchedulerRunDetail,
  getSchedulerSupplierChecklist,
  listSchedulerActions,
  listSchedulerPlans,
  listSchedulerRuns,
  listSchedulerSupplierStatuses,
  loginSupplierSession,
  retrySchedulerRunFailedSteps,
  retrySchedulerStep,
  updateSchedulerActionStatus,
  updateSchedulerPlanConfig,
  updateSchedulerPlanStatus,
  updateSchedulerSettings,
  type ExtensionTaskType,
  type SchedulerAction,
  type SchedulerCenterStatus,
  type SchedulerPlan,
  type SchedulerPlanConfig,
  type SchedulerRunDetail,
  type SchedulerRunSummary,
  type SchedulerSettings,
  type SchedulerStepRecord,
  type SchedulerSupplierChecklist,
  type SchedulerSupplierStatus
} from '@/api/admin/adminPlus'
import {
  actionStatusLabel,
  formatDateTime,
  planManualTaskTypes,
  planStatusLabel,
  runCancellable,
  runRetryable,
  runStatusClass,
  runStatusLabel,
  schedulerTabs,
  severityClass,
  severityLabel,
  statusClass,
  taskLabel
} from './presentation'
import {
  checklistActionForKey,
  supplierActionKey,
  supplierActionLabel,
  supplierActionTaskTypes,
  type SupplierAutomationAction
} from './supplierAutomation'

type TabValue = 'dashboard' | 'plans' | 'runs' | 'suppliers' | 'actions' | 'settings'

const appStore = useAppStore()
const loading = ref(false)
const running = ref(false)
const settingsSaving = ref(false)
const updatingPlanId = ref<string | null>(null)
const updatingActionId = ref<string | null>(null)
const updatingRunId = ref<string | null>(null)
const runningSupplierActionKey = ref<string | null>(null)
const runDetailOpen = ref(false)
const runDetailLoading = ref(false)
const retryingStepId = ref<number | null>(null)
const cancellingStepId = ref<number | null>(null)
const supplierChecklistOpen = ref(false)
const supplierChecklistLoading = ref(false)
const activeTab = ref<TabValue>('dashboard')
const status = ref<SchedulerCenterStatus | null>(null)
const settings = ref<SchedulerSettings | null>(null)
const plans = ref<SchedulerPlan[]>([])
const runs = ref<SchedulerRunSummary[]>([])
const supplierStatuses = ref<SchedulerSupplierStatus[]>([])
const actions = ref<SchedulerAction[]>([])
const runDetail = ref<SchedulerRunDetail | null>(null)
const supplierChecklist = ref<SchedulerSupplierChecklist | null>(null)
const settingsForm = reactive<SchedulerSettings>({
  enabled: true,
  default_supplier_concurrency: 1,
  channel_checks_enabled: false,
  channel_check_daily_budget_tokens: 0,
  first_token_slow_threshold_ms: 0,
  total_latency_slow_threshold_ms: 0,
  default_enabled_task_types: [],
  high_cost_task_types: []
})

const tabs = schedulerTabs

const intervalLabel = computed(() => {
  const seconds = status.value?.interval_seconds || 0
  if (seconds <= 0) return '未配置'
  if (seconds % 60 === 0) return `${seconds / 60} 分钟周期`
  return `${seconds} 秒周期`
})

const workerLabel = computed(() => {
  const value = status.value?.worker_status || 'unknown'
  return {
    running: '运行中',
    paused: '已暂停',
    degraded: '降级',
    down: '停止'
  }[value] || value
})

const nextRunLabel = computed(() => {
  const current = status.value
  if (!current?.next_run_at) return '-'
  const formatted = formatDateTime(current.next_run_at) || '-'
  if ((current.overdue_plans || 0) > 0) {
    return `待调度 ${current.overdue_plans} 个 · ${formatted}`
  }
  return formatted
})

async function loadPage() {
  loading.value = true
  try {
    const [nextStatus, nextPlans, nextRuns, nextSuppliers, nextActions, nextSettings] = await Promise.all([
      getSchedulerCenterStatus(),
      listSchedulerPlans(),
      listSchedulerRuns({ limit: 30 }),
      listSchedulerSupplierStatuses(),
      listSchedulerActions(),
      getSchedulerSettings()
    ])
    status.value = nextStatus
    plans.value = nextPlans
    runs.value = nextRuns
    supplierStatuses.value = nextSuppliers
    actions.value = nextActions
    settings.value = nextSettings
    syncSettingsForm(nextSettings)
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '加载调度中心失败')
  } finally {
    loading.value = false
  }
}

async function runBalanceSync() {
  await runTask(['fetch_balance'])
}

async function runPlan(plan: SchedulerPlan) {
  const taskTypes = planManualTaskTypes(plan)
  if (taskTypes.length === 0) {
    appStore.showError('该计划需要持久化调度执行器支持，当前不可手动运行')
    return
  }
  await runTask(taskTypes)
}

async function runTask(taskTypes: ExtensionTaskType[]) {
  running.value = true
  try {
    const run = await createSchedulerRun({
      mode: 'manual',
      task_types: taskTypes,
      window_minutes: 10
    })
    appStore.showSuccess(`已提交 ${taskLabel(run.task_type)}，${run.succeeded_steps}/${run.total_steps} step 完成`)
    await loadPage()
    activeTab.value = 'runs'
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '提交调度运行失败')
  } finally {
    running.value = false
  }
}

async function openRunDetail(run: SchedulerRunSummary) {
  runDetailOpen.value = true
  runDetailLoading.value = true
  try {
    runDetail.value = await getSchedulerRunDetail(run.id)
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '加载运行详情失败')
  } finally {
    runDetailLoading.value = false
  }
}

function closeRunDetail() {
  runDetailOpen.value = false
  runDetail.value = null
}

async function retryStep(step: SchedulerStepRecord) {
  retryingStepId.value = step.id
  try {
    await retrySchedulerStep(step.id)
    appStore.showSuccess('已提交 step 重试')
    await refreshOpenRunDetail()
    await loadPage()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '提交 step 重试失败')
  } finally {
    retryingStepId.value = null
  }
}

async function cancelStep(step: SchedulerStepRecord) {
  cancellingStepId.value = step.id
  try {
    await cancelSchedulerStep(step.id)
    appStore.showSuccess('step 已取消')
    await refreshOpenRunDetail()
    await loadPage()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '取消 step 失败')
  } finally {
    cancellingStepId.value = null
  }
}

async function cancelRun(run: SchedulerRunSummary) {
  updatingRunId.value = run.id
  try {
    await cancelSchedulerRun(run.id)
    appStore.showSuccess('运行已取消')
    await refreshOpenRunDetail()
    await loadPage()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '取消运行失败')
  } finally {
    updatingRunId.value = null
  }
}

async function retryRunFailedSteps(run: SchedulerRunSummary) {
  updatingRunId.value = run.id
  try {
    const detail = await retrySchedulerRunFailedSteps(run.id)
    if (runDetail.value?.run.id === run.id) {
      runDetail.value = detail
    }
    appStore.showSuccess('已提交失败 step 重试')
    await loadPage()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '提交失败 step 重试失败')
  } finally {
    updatingRunId.value = null
  }
}

async function refreshOpenRunDetail(silent = false) {
  if (!runDetail.value) return
  try {
    runDetail.value = await getSchedulerRunDetail(runDetail.value.run.id)
  } catch (error) {
    if (!silent) {
      appStore.showError((error as { message?: string }).message || '刷新运行详情失败')
    }
  }
}

async function openSupplierChecklist(supplier: SchedulerSupplierStatus) {
  supplierChecklistOpen.value = true
  supplierChecklistLoading.value = true
  try {
    supplierChecklist.value = await getSchedulerSupplierChecklist(supplier.supplier_id)
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '加载供应商 Checklist 失败')
  } finally {
    supplierChecklistLoading.value = false
  }
}

function closeSupplierChecklist() {
  supplierChecklistOpen.value = false
  supplierChecklist.value = null
}

async function runSupplierChecklistAction(itemKey: string) {
  const checklist = supplierChecklist.value
  if (!checklist) return
  const action = checklistActionForKey(itemKey)
  if (!action) {
    appStore.showError('该项需要在供应商管理中处理')
    return
  }
  const supplier = supplierStatuses.value.find((item) => item.supplier_id === checklist.supplier_id)
  if (!supplier) {
    appStore.showError('未找到供应商状态，请刷新后重试')
    return
  }
  await runSupplierAutomationAction(supplier, action)
  if (supplierChecklistOpen.value) {
    supplierChecklistLoading.value = true
    try {
      supplierChecklist.value = await getSchedulerSupplierChecklist(checklist.supplier_id)
    } catch (error) {
      appStore.showError((error as { message?: string }).message || '刷新供应商 Checklist 失败')
    } finally {
      supplierChecklistLoading.value = false
    }
  }
}

async function runSupplierAutomationAction(supplier: SchedulerSupplierStatus, action: SupplierAutomationAction) {
  const key = supplierActionKey(supplier.supplier_id, action)
  runningSupplierActionKey.value = key
  try {
    if (action === 'login_session') {
      const result = await loginSupplierSession(supplier.supplier_id)
      if (result.balance_sync_error) {
        appStore.showWarning('已完成供应商直登，余额读取失败，调度中心会继续重试')
      } else {
        appStore.showSuccess('已完成供应商直登')
      }
      await loadPage()
      return
    }
    const taskTypes = supplierActionTaskTypes(action)
    if (taskTypes.length === 0) {
      appStore.showError('该动作暂未接入自动执行')
      return
    }
    const run = await createSchedulerRun({
      mode: `supplier:${action}`,
      supplier_id: supplier.supplier_id,
      task_types: taskTypes,
      window_minutes: 10
    })
    appStore.showSuccess(`已提交 ${supplier.supplier_name} 的${supplierActionLabel(action)}，${run.total_steps} 个 step`)
    await loadPage()
    activeTab.value = 'runs'
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '提交供应商自动化动作失败')
  } finally {
    runningSupplierActionKey.value = null
  }
}

async function setPlanStatus(plan: SchedulerPlan, status: 'enabled' | 'paused' | 'disabled') {
  updatingPlanId.value = plan.id
  try {
    const updated = await updateSchedulerPlanStatus(plan.id, status)
    plans.value = plans.value.map((item) => (item.id === updated.id ? updated : item))
    appStore.showSuccess(`计划已${planStatusLabel(updated.status)}`)
    await loadPage()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '更新计划状态失败')
  } finally {
    updatingPlanId.value = null
  }
}

async function savePlanConfig(plan: SchedulerPlan, config: SchedulerPlanConfig) {
  updatingPlanId.value = plan.id
  try {
    const updated = await updateSchedulerPlanConfig(plan.id, config)
    plans.value = plans.value.map((item) => (item.id === updated.id ? updated : item))
    appStore.showSuccess('计划配置已保存')
    await loadPage()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '保存计划配置失败')
  } finally {
    updatingPlanId.value = null
  }
}

async function setActionStatus(action: SchedulerAction, status: 'resolved' | 'ignored' | 'investigating') {
  updatingActionId.value = action.id
  try {
    await updateSchedulerActionStatus(action.id, status)
    appStore.showSuccess(`动作已标记为${actionStatusLabel(status)}`)
    await loadPage()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '更新动作状态失败')
  } finally {
    updatingActionId.value = null
  }
}

async function saveSettings() {
  settingsSaving.value = true
  try {
    const updated = await updateSchedulerSettings(normalizedSettingsPayload())
    settings.value = updated
    syncSettingsForm(updated)
    appStore.showSuccess('调度设置已保存')
    await loadPage()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '保存调度设置失败')
  } finally {
    settingsSaving.value = false
  }
}

function syncSettingsForm(value: SchedulerSettings) {
  settingsForm.enabled = value.enabled
  settingsForm.default_supplier_concurrency = value.default_supplier_concurrency || 1
  settingsForm.channel_checks_enabled = value.channel_checks_enabled
  settingsForm.channel_check_daily_budget_tokens = value.channel_check_daily_budget_tokens || 0
  settingsForm.first_token_slow_threshold_ms = value.first_token_slow_threshold_ms || 0
  settingsForm.total_latency_slow_threshold_ms = value.total_latency_slow_threshold_ms || 0
  settingsForm.default_enabled_task_types = [...(value.default_enabled_task_types || [])]
  settingsForm.high_cost_task_types = [...(value.high_cost_task_types || [])]
}

function normalizedSettingsPayload(): SchedulerSettings {
  return {
    enabled: settingsForm.enabled,
    default_supplier_concurrency: Math.max(1, Number(settingsForm.default_supplier_concurrency) || 1),
    channel_checks_enabled: settingsForm.channel_checks_enabled,
    channel_check_daily_budget_tokens: Math.max(0, Number(settingsForm.channel_check_daily_budget_tokens) || 0),
    first_token_slow_threshold_ms: Math.max(0, Number(settingsForm.first_token_slow_threshold_ms) || 0),
    total_latency_slow_threshold_ms: Math.max(0, Number(settingsForm.total_latency_slow_threshold_ms) || 0),
    default_enabled_task_types: [...settingsForm.default_enabled_task_types],
    high_cost_task_types: [...settingsForm.high_cost_task_types]
  }
}

function runPrimaryTime(run: SchedulerRunSummary): string {
  return formatDateTime(run.requested_at) || formatDateTime(run.started_at) || formatDateTime(run.finished_at) || '-'
}

onMounted(loadPage)
</script>
