<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">动作建议</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
            汇总供应商余额、健康、利润和候选阻塞信号，生成充值、切换、降权和排查建议。
          </p>
        </div>
        <div class="flex flex-wrap gap-2">
          <button type="button" class="btn btn-secondary" :disabled="loading" @click="loadPage">
            <Icon name="refresh" size="sm" />
            刷新
          </button>
          <button type="button" class="btn btn-primary" :disabled="generating || suppliers.length === 0" @click="generate">
            {{ generating ? '生成中...' : '生成建议' }}
          </button>
        </div>
      </section>

      <section class="grid gap-4 md:grid-cols-5">
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">开放建议</p>
          <p class="mt-2 text-2xl font-semibold text-amber-600 dark:text-amber-400">{{ openCount }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">严重</p>
          <p class="mt-2 text-2xl font-semibold text-rose-600 dark:text-rose-400">{{ criticalCount }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">可切换供应商</p>
          <p class="mt-2 text-2xl font-semibold text-emerald-600 dark:text-emerald-400">{{ switchableCount }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">待处理信号</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ openSignalCount }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">低倍率余额机会</p>
          <p class="mt-2 text-2xl font-semibold text-sky-600 dark:text-sky-400">{{ balanceOpportunityCount }}</p>
        </div>
      </section>

      <section class="card overflow-hidden">
        <div class="flex flex-col gap-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700 lg:flex-row lg:items-start lg:justify-between">
          <div>
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">低倍率余额机会</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
              余额不足不会被判定为渠道坏，充值后先刷新余额，再按需手动提交通道检测和重新生成建议。
            </p>
          </div>
          <div class="flex flex-wrap gap-2">
            <button
              type="button"
              class="btn btn-secondary"
              :disabled="loading || balanceBatchRefreshing || balanceOpportunitySupplierIDs.length === 0"
              @click="refreshAllOpportunityBalances"
            >
              <Icon name="refresh" size="sm" :class="{ 'animate-spin': balanceBatchRefreshing }" />
              {{ balanceBatchRefreshing ? `刷新中 ${balanceBatchRefreshDone}/${balanceBatchRefreshTotal}` : `批量刷新余额 (${balanceOpportunitySupplierIDs.length})` }}
            </button>
            <button type="button" class="btn btn-secondary" :disabled="loading || balanceBatchRefreshing" @click="loadPage">
              <Icon name="refresh" size="sm" />
              重算候选
            </button>
          </div>
        </div>
        <div v-if="balanceOpportunities.length === 0" class="px-5 py-8 text-center text-sm text-gray-500 dark:text-dark-400">
          暂无余额阻断的低倍率候选
        </div>
        <div v-else class="overflow-x-auto">
          <table class="min-w-full divide-y divide-gray-200 dark:divide-dark-700">
            <thead class="bg-gray-50 dark:bg-dark-800">
              <tr>
                <th class="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-dark-400">供应商 / 分组</th>
                <th class="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-dark-400">本地账号</th>
                <th class="px-4 py-2 text-right text-xs font-medium text-gray-500 dark:text-dark-400">有效倍率</th>
                <th class="px-4 py-2 text-right text-xs font-medium text-gray-500 dark:text-dark-400">余额</th>
                <th class="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-dark-400">判断</th>
                <th class="px-4 py-2 text-right text-xs font-medium text-gray-500 dark:text-dark-400">操作</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 bg-white dark:divide-dark-800 dark:bg-dark-900">
              <tr v-for="item in balanceOpportunities" :key="item.key">
                <td class="max-w-xs px-4 py-3 text-sm">
                  <div class="truncate font-medium text-gray-900 dark:text-white">{{ item.supplier_name }}</div>
                  <div class="truncate text-xs text-gray-500 dark:text-dark-400">{{ item.supplier_group_name }}</div>
                </td>
                <td class="max-w-xs px-4 py-3 text-sm">
                  <div class="truncate text-gray-900 dark:text-white">{{ item.local_account_name }}</div>
                  <div class="truncate text-xs text-gray-500 dark:text-dark-400">{{ item.local_group_label }}</div>
                </td>
                <td class="px-4 py-3 text-right text-sm font-medium text-sky-600 dark:text-sky-400">{{ multiplierLabel(item.effective_rate_multiplier) }}</td>
                <td class="px-4 py-3 text-right text-sm text-gray-600 dark:text-dark-300">
                  <span class="badge badge-warning">{{ moneyLabel(item.balance_cents, item.balance_currency) }}</span>
                </td>
                <td class="px-4 py-3 text-sm text-gray-600 dark:text-dark-300">
                  <div class="flex flex-wrap gap-1">
                    <span class="badge badge-warning">{{ candidateReasonText(item.blocked_reason) }}</span>
                    <span class="badge badge-gray">{{ item.check_source || '候选评估' }}</span>
                  </div>
                </td>
                <td class="px-4 py-3">
                  <div class="flex justify-end gap-2">
                    <button
                      type="button"
                      class="btn btn-secondary px-3 py-1.5 text-xs"
                      :disabled="isBalanceRefreshLocked(item.supplier_id)"
                      @click="refreshOpportunityBalance(item)"
                    >
                      <Icon name="refresh" size="xs" :class="{ 'animate-spin': isBalanceRefreshing(item.supplier_id) }" />
                      刷新余额
                    </button>
                    <button
                      type="button"
                      class="btn btn-secondary px-3 py-1.5 text-xs"
                      :disabled="channelSyncingID === item.supplier_id"
                      @click="syncOpportunityChannel(item)"
                    >
                      <Icon name="beaker" size="xs" :class="{ 'animate-spin': channelSyncingID === item.supplier_id }" />
                      通道检测
                    </button>
                    <button type="button" class="btn btn-primary px-3 py-1.5 text-xs" @click="openOpportunitySupplier(item)">
                      处理
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section class="card overflow-hidden">
        <div class="flex flex-col gap-2 border-b border-gray-100 px-5 py-4 dark:border-dark-700 sm:flex-row sm:items-center sm:justify-between">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">建议队列</h2>
          <div v-if="actionTypeFilter" class="flex items-center gap-2">
            <span class="badge badge-primary">{{ actionTypeFilterLabel }}</span>
            <button type="button" class="btn btn-secondary btn-sm" @click="clearActionTypeFilter">清除筛选</button>
          </div>
        </div>

        <div class="divide-y divide-gray-100 dark:divide-dark-700">
          <div v-if="recommendations.length === 0" class="px-5 py-10 text-center text-sm text-gray-500 dark:text-dark-400">暂无建议</div>
          <div v-for="item in recommendations" :key="item.id" class="p-5">
            <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
              <div>
                <div class="flex flex-wrap items-center gap-2">
                  <span class="badge" :class="severityClass(item.severity)">{{ item.severity }}</span>
                  <span class="badge badge-gray">{{ item.type }}</span>
                  <span class="badge" :class="statusClass(item.status)">{{ item.status }}</span>
                  <span v-if="item.requires_approval" class="badge badge-warning">需审批</span>
                </div>
                <h3 class="mt-3 font-medium text-gray-900 dark:text-white">{{ item.title }}</h3>
                <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ item.description }}</p>
                <p class="mt-2 text-xs text-gray-500 dark:text-dark-400">
                  {{ supplierName(item.supplier_id) }}
                  <span v-if="item.target_supplier_id"> -> {{ supplierName(item.target_supplier_id) }}</span>
                  · {{ item.reason_code }}
                </p>
                <p v-if="item.expected_impact" class="mt-2 text-xs text-emerald-600 dark:text-emerald-400">
                  {{ item.expected_impact }}
                </p>
                <p v-if="lastExecutions[item.id]" class="mt-2 text-xs" :class="executionClass(lastExecutions[item.id].status)">
                  回执 {{ executionLabel(lastExecutions[item.id].status) }}
                  <span v-if="lastExecutions[item.id].error_message">· {{ lastExecutions[item.id].error_message }}</span>
                </p>
                <p v-if="routingRefillResults[item.id]" class="mt-2 text-xs text-sky-600 dark:text-sky-400">
                  {{ routingRefillSummary(routingRefillResults[item.id]) }}
                </p>
                <p v-if="localAccountOpsResults[item.id]" class="mt-2 text-xs" :class="localAccountOpsResults[item.id].blocked ? 'text-amber-600 dark:text-amber-400' : 'text-sky-600 dark:text-sky-400'">
                  {{ localAccountOpsSummary(localAccountOpsResults[item.id]) }}
                </p>
                <p v-if="costReconcileResults[item.id]" class="mt-2 text-xs text-sky-600 dark:text-sky-400">
                  {{ costReconcileSummary(costReconcileResults[item.id]) }}
                </p>
                <p v-if="costDetailRepairResults[item.id]" class="mt-2 text-xs text-emerald-600 dark:text-emerald-400">
                  {{ costDetailRepairSummary(costDetailRepairResults[item.id]) }}
                </p>
              </div>
              <div class="flex flex-wrap gap-2">
                <button
                  v-if="canOpenSupplierKeyPlan(item)"
                  type="button"
                  class="btn btn-primary px-3 py-1.5 text-xs"
                  @click="openSupplierKeyPlan(item)"
                >
                  <Icon name="clipboard" size="xs" />
                  开通计划
                </button>
                <button
                  v-if="canRunRoutingRefill(item)"
                  type="button"
                  class="btn btn-secondary px-3 py-1.5 text-xs"
                  :disabled="routingRefillPreviewingID === item.id || executingID === item.id"
                  @click="previewRoutingRefillRecommendation(item)"
                >
                  <Icon name="search" size="xs" :class="{ 'animate-spin': routingRefillPreviewingID === item.id }" />
                  {{ routingRefillPreviewingID === item.id ? '预览中' : '补池预览' }}
                </button>
                <button
                  v-if="canRunLocalAccountScheduleDisable(item)"
                  type="button"
                  class="btn btn-secondary px-3 py-1.5 text-xs"
                  :disabled="localAccountSchedulePreviewingID === item.id || executingID === item.id"
                  @click="previewLocalAccountScheduleDisableRecommendation(item)"
                >
                  <Icon name="search" size="xs" :class="{ 'animate-spin': localAccountSchedulePreviewingID === item.id }" />
                  {{ localAccountSchedulePreviewingID === item.id ? '预览中' : '关闭预览' }}
                </button>
                <button
                  v-for="option in costReconcileDetailRepairOptions(item)"
                  :key="option.type"
                  type="button"
                  class="btn btn-secondary px-3 py-1.5 text-xs"
                  :disabled="executingID === item.id"
                  @click="executeCostReconcileDetailRepairRecommendation(item, option.type)"
                >
                  <Icon name="clipboard" size="xs" />
                  {{ option.label }}
                </button>
                <button
                  v-if="canApplyCostReconcileAdjustment(item)"
                  type="button"
                  class="btn btn-secondary px-3 py-1.5 text-xs"
                  :disabled="executingID === item.id"
                  @click="executeCostReconcileAdjustmentRecommendation(item)"
                >
                  <Icon name="clipboard" size="xs" />
                  对账调整
                </button>
                <button
                  v-for="status in statuses"
                  :key="status"
                  type="button"
                  class="btn btn-secondary px-3 py-1.5 text-xs"
                  :disabled="item.status === status"
                  @click="updateStatus(item.id, status)"
                >
                  {{ status }}
                </button>
                <button
                  v-if="item.status === 'approved'"
                  type="button"
                  class="btn btn-primary px-3 py-1.5 text-xs"
                  :disabled="executingID === item.id"
                  @click="executeRecommendation(item)"
                >
                  {{ executingID === item.id ? '执行中' : executeButtonLabel(item) }}
                </button>
                <button
                  type="button"
                  class="btn btn-secondary px-3 py-1.5 text-xs"
                  :disabled="executionState(item.id).loading"
                  @click="toggleExecutions(item.id)"
                >
                  <Icon name="clock" size="xs" :class="{ 'animate-spin': executionState(item.id).loading }" />
                  执行历史
                </button>
              </div>
            </div>
            <div
              v-if="expandedExecutionID === item.id"
              class="mt-4 overflow-hidden rounded-md border border-gray-200 bg-gray-50 dark:border-dark-700 dark:bg-dark-900"
            >
              <div class="flex flex-wrap items-center justify-between gap-2 border-b border-gray-200 px-4 py-3 dark:border-dark-700">
                <div>
                  <p class="text-sm font-medium text-gray-900 dark:text-white">执行历史</p>
                  <p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">
                    记录审批后执行、人工工作流回执、失败原因和自动执行器支持状态。
                  </p>
                </div>
                <button
                  type="button"
                  class="btn btn-secondary px-3 py-1.5 text-xs"
                  :disabled="executionState(item.id).loading"
                  @click="loadExecutions(item.id)"
                >
                  <Icon name="refresh" size="xs" :class="{ 'animate-spin': executionState(item.id).loading }" />
                  刷新
                </button>
              </div>

              <div v-if="executionState(item.id).loading" class="px-4 py-6 text-center text-sm text-gray-500 dark:text-dark-400">
                加载中...
              </div>
              <div v-else-if="executionState(item.id).items.length === 0" class="px-4 py-6 text-center text-sm text-gray-500 dark:text-dark-400">
                暂无执行记录
              </div>
              <div v-else class="overflow-x-auto">
                <table class="min-w-full divide-y divide-gray-200 dark:divide-dark-700">
                  <thead class="bg-white dark:bg-dark-800">
                    <tr>
                      <th class="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-dark-400">时间</th>
                      <th class="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-dark-400">状态</th>
                      <th class="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-dark-400">执行类型</th>
                      <th class="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-dark-400">操作者</th>
                      <th class="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-dark-400">来源</th>
                      <th class="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-dark-400">回执</th>
                      <th class="px-4 py-2 text-right text-xs font-medium text-gray-500 dark:text-dark-400">操作</th>
                    </tr>
                  </thead>
                  <tbody class="divide-y divide-gray-100 bg-white dark:divide-dark-800 dark:bg-dark-900">
                    <tr v-for="execution in executionState(item.id).items" :key="execution.id" :class="focusedExecutionClass(execution)">
                      <td class="whitespace-nowrap px-4 py-3 text-sm text-gray-600 dark:text-dark-300">
                        {{ formatDateTime(execution.created_at) }}
                      </td>
                      <td class="px-4 py-3 text-sm">
                        <span class="badge" :class="executionBadgeClass(execution.status)">
                          {{ executionLabel(execution.status) }}
                        </span>
                        <span v-if="isFocusedExecution(execution)" class="badge badge-primary ml-1">当前</span>
                      </td>
                      <td class="px-4 py-3 text-sm text-gray-600 dark:text-dark-300">
                        {{ execution.action_type }}
                      </td>
                      <td class="px-4 py-3 text-sm text-gray-600 dark:text-dark-300">
                        {{ execution.operator_user_id ? `#${execution.operator_user_id}` : '-' }}
                      </td>
                      <td class="px-4 py-3 text-sm text-gray-600 dark:text-dark-300">
                        <RouterLink
                          v-if="execution.scheduler_run_id"
                          :to="executionSchedulerRoute(execution)"
                          class="text-primary-600 hover:text-primary-700 dark:text-primary-400"
                        >
                          {{ executionSchedulerLabel(execution) }}
                        </RouterLink>
                        <span v-else>-</span>
                      </td>
                      <td class="max-w-xl px-4 py-3 text-sm text-gray-600 dark:text-dark-300">
                        <div class="truncate" :title="executionSummary(execution)">
                          {{ executionSummary(execution) }}
                        </div>
                        <div v-if="execution.error_message" class="mt-0.5 truncate text-xs text-rose-600 dark:text-rose-400">
                          {{ execution.error_message }}
                        </div>
                        <div v-if="executionAuditSummary(execution)" class="mt-1 truncate text-xs text-gray-500 dark:text-dark-400" :title="executionAuditSummary(execution)">
                          {{ executionAuditSummary(execution) }}
                        </div>
                        <div v-if="executionSnapshotSummary(execution)" class="mt-1 truncate text-xs text-gray-500 dark:text-dark-400" :title="executionSnapshotSummary(execution)">
                          {{ executionSnapshotSummary(execution) }}
                        </div>
                      </td>
                      <td class="px-4 py-3 text-right text-sm">
                        <button
                          v-if="canRetryExecution(item, execution)"
                          type="button"
                          class="btn btn-secondary px-3 py-1.5 text-xs"
                          :disabled="isRetryingExecution(execution.id)"
                          @click="retryExecution(item.id, execution)"
                        >
                          <Icon name="refresh" size="xs" :class="{ 'animate-spin': isRetryingExecution(execution.id) }" />
                          重试
                        </button>
                        <button
                          v-else-if="canRollbackExecution(item, execution, executionState(item.id).items)"
                          type="button"
                          class="btn btn-secondary px-3 py-1.5 text-xs"
                          :disabled="isRollingBackExecution(execution.id)"
                          @click="rollbackExecution(item.id, execution)"
                        >
                          <Icon name="refresh" size="xs" :class="{ 'animate-spin': isRollingBackExecution(execution.id) }" />
                          回滚
                        </button>
                        <span v-else class="text-xs text-gray-400 dark:text-dark-500">-</span>
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
              <Pagination
                v-if="executionState(item.id).total > 0"
                :page="executionState(item.id).page"
                :total="executionState(item.id).total"
                :page-size="executionState(item.id).page_size"
                :show-jump="true"
                @update:page="(page) => handleExecutionPageChange(item.id, page)"
                @update:pageSize="(pageSize) => handleExecutionPageSizeChange(item.id, pageSize)"
              />
            </div>
          </div>
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
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import Pagination from '@/components/common/Pagination.vue'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { useAppStore } from '@/stores/app'
import { routingRefillSkippedReasonLabel } from '@/views/admin/routingRefillPresentation'
import {
  applyCostReconcileAdjustment,
  applyCostReconcileDetailRepair,
  applyLocalAccountOpsAction,
  executeActionRecommendation,
  generateActions,
  getSchedulerSettings,
  getSupplierCurrentBalance,
  listActionExecutions,
  listActionRecommendations,
  listBalanceEvents,
  listHealthEvents,
  listKanbanEvents,
  listLocalAccountOps,
  listLocalSub2APIGroups,
  listRateSnapshots,
  listSupplierCostSnapshots,
  listSuppliers,
  previewLocalAccountOpsAction,
  refillLocalGroup,
  retryActionExecution,
  rollbackActionExecution,
  syncSupplierChannelChecks,
  updateActionRecommendationStatus,
  type ActionExecution,
  type ActionRecommendation,
  type BalanceEvent,
  type CostReconcileDetailRepairResult,
  type CostReconcileDetailType,
  type HealthEvent,
  type KanbanEvent,
  type LocalGroupCapacitySignal,
  type LocalAccountOpsActionResult,
  type LocalAccountOpsRow,
  type LocalAccountScheduleSignal,
  type LocalSub2APIGroup,
  type RateSnapshot,
  type RoutingRefillResult,
  type SchedulerSettings,
  type Supplier,
  type SupplierCostSnapshot,
  type SupplierSignal
} from '@/api/admin/adminPlus'

const appStore = useAppStore()
const route = useRoute()
const router = useRouter()

const loading = ref(false)
const generating = ref(false)
const executingID = ref<number | null>(null)
const balanceRefreshingID = ref<number | null>(null)
const balanceBatchRefreshing = ref(false)
const balanceBatchRefreshDone = ref(0)
const balanceBatchRefreshTotal = ref(0)
const balanceBatchRefreshFailed = ref(0)
const balanceBatchRefreshingSupplierIDs = ref<Set<number>>(new Set())
const channelSyncingID = ref<number | null>(null)
const suppliers = ref<Supplier[]>([])
const rateSnapshots = ref<RateSnapshot[]>([])
const balanceEvents = ref<BalanceEvent[]>([])
const healthEvents = ref<HealthEvent[]>([])
const kanbanEvents = ref<KanbanEvent[]>([])
const localAccountOpsRows = ref<LocalAccountOpsRow[]>([])
const localGroups = ref<LocalSub2APIGroup[]>([])
const schedulerSettings = ref<SchedulerSettings | null>(null)
const supplierCostSnapshots = ref<SupplierCostSnapshot[]>([])
const recommendations = ref<ActionRecommendation[]>([])
const lastExecutions = ref<Record<number, ActionExecution>>({})
const routingRefillPreviewingID = ref<number | null>(null)
const routingRefillResults = ref<Record<number, RoutingRefillResult>>({})
const localAccountSchedulePreviewingID = ref<number | null>(null)
const localAccountOpsResults = ref<Record<number, LocalAccountOpsActionResult>>({})
const costReconcileResults = ref<Record<number, CostReconcileAdjustmentResult>>({})
const costDetailRepairResults = ref<Record<number, CostReconcileDetailRepairResult>>({})
const pagination = reactive({ page: 1, page_size: getPersistedPageSize(), total: 0, pages: 0 })
const expandedExecutionID = ref<number | null>(null)
const retryingExecutionIDs = ref<Set<number>>(new Set())
const rollingBackExecutionIDs = ref<Set<number>>(new Set())

interface ExecutionListState {
  items: ActionExecution[]
  page: number
  page_size: number
  total: number
  pages: number
  loading: boolean
}

interface BalanceOpportunity {
  key: string
  supplier_id: number
  supplier_name: string
  supplier_group_id?: number
  supplier_group_name: string
  local_sub2api_account_id: number
  local_account_name: string
  local_group_label: string
  effective_rate_multiplier: number
  balance_cents: number
  balance_currency: string
  blocked_reason?: string
  check_source?: string
}

interface CostReconcileAdjustmentResult {
  supplier_id: number
  snapshot_id: number
  currency: string
  adjustment_amount_cents: number
  after_snapshot?: SupplierCostSnapshot
}

interface CostReconcileRepairOption {
  type: CostReconcileDetailType
  label: string
}

const executionStates = reactive<Record<number, ExecutionListState>>({})

const statuses: ActionRecommendation['status'][] = ['acknowledged', 'approved', 'executed', 'rejected']
const actionTypeFilter = computed<ActionRecommendation['type'] | ''>(() => normalizeActionTypeQuery(route.query.type))
const actionObjectFilter = computed(() => ({
  localGroupID: positiveQueryNumber(route.query.local_group_id),
  localAccountID: positiveQueryNumber(route.query.local_sub2api_account_id)
}))
const focusedRecommendationID = computed(() => positiveQueryNumber(route.query.recommendation_id))
const focusedExecutionID = computed(() => positiveQueryNumber(route.query.execution_id))
const schedulerSource = computed(() => ({
  runID: stringQuery(route.query.scheduler_run_id),
  stepID: positiveQueryNumber(route.query.scheduler_step_id)
}))
const actionTypeFilterLabel = computed(() => actionTypeLabel(actionTypeFilter.value))
const supplierKeyPlanReasonCodes = new Set([
  'candidate_key_capacity_exhausted',
  'supplier_key_capacity_exhausted',
  'supplier_key_capacity_unknown',
  'supplier_key_provisioning_unsupported'
])

const openCount = computed(() => recommendations.value.filter((item) => item.status === 'open').length)
const criticalCount = computed(() => recommendations.value.filter((item) => item.severity === 'critical' && item.status === 'open').length)
const switchableCount = computed(() => suppliers.value.filter((supplier) =>
  ['active', 'candidate'].includes(supplier.runtime_status) && supplier.health_status === 'normal' && supplier.balance_cents > 0
).length)
const openSignalCount = computed(() =>
  balanceEvents.value.filter((event) => event.status === 'open').length +
  healthEvents.value.filter((event) => event.status === 'open').length +
  kanbanEvents.value.filter((event) => event.status === 'open').length +
  suppliers.value.filter((supplier) => ['exhausted', 'unknown', 'unsupported'].includes(String(supplier.key_capacity_status || ''))).length +
  candidateEvaluationSignals().length +
  costSnapshotSignals().length +
  localGroupCapacitySignals().length +
  localAccountScheduleSignals().length
)
const allBalanceOpportunities = computed(() => buildBalanceOpportunities(localAccountOpsRows.value))
const balanceOpportunities = computed(() => allBalanceOpportunities.value.slice(0, 10))
const balanceOpportunityCount = computed(() => allBalanceOpportunities.value.length)
const balanceOpportunitySupplierIDs = computed(() => {
  const ids = new Set<number>()
  for (const item of allBalanceOpportunities.value) {
    if (item.supplier_id > 0) ids.add(item.supplier_id)
  }
  return Array.from(ids).sort((left, right) => left - right)
})

function supplierName(id?: number | null): string {
  if (!id) return '全局'
  return suppliers.value.find((supplier) => supplier.id === id)?.name || `#${id}`
}

function severityClass(severity: ActionRecommendation['severity']): string {
  if (severity === 'critical') return 'badge-danger'
  if (severity === 'warning') return 'badge-warning'
  return 'badge-primary'
}

function statusClass(status: ActionRecommendation['status']): string {
  if (status === 'open') return 'badge-warning'
  if (['approved', 'executed', 'acknowledged'].includes(status)) return 'badge-success'
  if (status === 'rejected') return 'badge-danger'
  return 'badge-gray'
}

function executionLabel(status: ActionExecution['status']): string {
  if (status === 'succeeded') return '成功'
  if (status === 'unsupported') return '暂不支持自动执行'
  if (status === 'failed') return '失败'
  return '执行中'
}

function executionClass(status: ActionExecution['status']): string {
  if (status === 'succeeded') return 'text-emerald-600 dark:text-emerald-400'
  if (status === 'unsupported') return 'text-amber-600 dark:text-amber-400'
  if (status === 'failed') return 'text-rose-600 dark:text-rose-400'
  return 'text-gray-500 dark:text-dark-400'
}

function executionBadgeClass(status: ActionExecution['status']): string {
  if (status === 'succeeded') return 'badge-success'
  if (status === 'failed') return 'badge-danger'
  if (status === 'unsupported') return 'badge-warning'
  return 'badge-gray'
}

function executionState(id: number): ExecutionListState {
  if (!executionStates[id]) {
    executionStates[id] = {
      items: [],
      page: 1,
      page_size: 5,
      total: 0,
      pages: 0,
      loading: false
    }
  }
  return executionStates[id]
}

async function toggleExecutions(id: number) {
  if (expandedExecutionID.value === id) {
    expandedExecutionID.value = null
    return
  }
  expandedExecutionID.value = id
  const state = executionState(id)
  if (state.items.length === 0) {
    state.page = 1
    await loadExecutions(id)
  }
}

async function loadExecutions(id: number) {
  const state = executionState(id)
  state.loading = true
  try {
    const result = await listActionExecutions(id, {
      page: state.page,
      page_size: state.page_size
    })
    state.items = result.items || []
    state.total = result.total || 0
    state.pages = result.pages || 0
    state.page = result.page || state.page
    state.page_size = result.page_size || state.page_size
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '加载执行历史失败')
  } finally {
    state.loading = false
  }
}

async function applyFocusedExecutionDeepLink() {
  const recommendationID = focusedRecommendationID.value
  if (recommendationID <= 0) return
  if (!recommendations.value.some((item) => item.id === recommendationID)) return
  expandedExecutionID.value = recommendationID
  const state = executionState(recommendationID)
  if (focusedExecutionID.value > 0) {
    state.page = 1
    state.page_size = Math.max(state.page_size, 50)
  }
  await loadExecutions(recommendationID)
}

function isFocusedExecution(execution: ActionExecution): boolean {
  return focusedExecutionID.value > 0 && execution.id === focusedExecutionID.value
}

function focusedExecutionClass(execution: ActionExecution): string {
  return isFocusedExecution(execution) ? 'bg-primary-50 dark:bg-primary-950/20' : ''
}

function canRetryExecution(item: ActionRecommendation, execution: ActionExecution): boolean {
  return item.status === 'approved' && execution.status === 'failed' && ['routing_refill', 'local_account_schedule_disable'].includes(execution.action_type)
}

function canRollbackExecution(item: ActionRecommendation, execution: ActionExecution, executions: ActionExecution[]): boolean {
  if (item.status !== 'executed') return false
  if (execution.status !== 'succeeded') return false
  if (!['routing_refill', 'local_account_schedule_disable'].includes(execution.action_type)) return false
  const mode = String(execution.request_payload?.mode || execution.response_payload?.mode || '')
  if (mode.includes('rollback')) return false
  return !executions.some((current) =>
    current.status === 'succeeded' && (
      payloadNumber(current.request_payload, 'rollback_source_execution_id') === execution.id ||
      payloadNumber(current.response_payload, 'rollback_source_execution_id') === execution.id
    )
  )
}

function isRetryingExecution(id: number): boolean {
  return retryingExecutionIDs.value.has(id)
}

function isRollingBackExecution(id: number): boolean {
  return rollingBackExecutionIDs.value.has(id)
}

function setExecutionRetrying(id: number, retrying: boolean) {
  const next = new Set(retryingExecutionIDs.value)
  if (retrying) {
    next.add(id)
  } else {
    next.delete(id)
  }
  retryingExecutionIDs.value = next
}

function setExecutionRollingBack(id: number, rollingBack: boolean) {
  const next = new Set(rollingBackExecutionIDs.value)
  if (rollingBack) {
    next.add(id)
  } else {
    next.delete(id)
  }
  rollingBackExecutionIDs.value = next
}

async function retryExecution(recommendationID: number, execution: ActionExecution) {
  const item = recommendations.value.find((current) => current.id === recommendationID)
  if (!item || !canRetryExecution(item, execution) || isRetryingExecution(execution.id)) return
  setExecutionRetrying(execution.id, true)
  try {
    const retried = await retryActionExecution(recommendationID, execution.id)
    lastExecutions.value = { ...lastExecutions.value, [recommendationID]: retried }
    appStore.showSuccess(retried.status === 'succeeded' ? '重试执行成功' : '重试已写入执行历史')
    await refreshRecommendationExecutionHistory(recommendationID)
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '重试执行失败')
    await loadExecutions(recommendationID)
  } finally {
    setExecutionRetrying(execution.id, false)
  }
}

async function rollbackExecution(recommendationID: number, execution: ActionExecution) {
  const item = recommendations.value.find((current) => current.id === recommendationID)
  const state = executionState(recommendationID)
  if (!item || !canRollbackExecution(item, execution, state.items) || isRollingBackExecution(execution.id)) return
  setExecutionRollingBack(execution.id, true)
  try {
    const rollback = await rollbackActionExecution(recommendationID, execution.id)
    lastExecutions.value = { ...lastExecutions.value, [recommendationID]: rollback }
    appStore.showSuccess(rollback.status === 'succeeded' ? '回滚执行成功' : '回滚已写入执行历史')
    await refreshRecommendationExecutionHistory(recommendationID)
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '回滚执行失败')
    await loadExecutions(recommendationID)
  } finally {
    setExecutionRollingBack(execution.id, false)
  }
}

function handleExecutionPageChange(id: number, page: number) {
  const state = executionState(id)
  state.page = page
  void loadExecutions(id)
}

function handleExecutionPageSizeChange(id: number, pageSize: number) {
  const state = executionState(id)
  state.page_size = pageSize
  state.page = 1
  void loadExecutions(id)
}

function canOpenSupplierKeyPlan(item: ActionRecommendation): boolean {
  return item.supplier_id > 0 && supplierKeyPlanReasonCodes.has(item.reason_code)
}

function openSupplierKeyPlan(item: ActionRecommendation) {
  if (!canOpenSupplierKeyPlan(item)) return
  void router.push({
    path: '/admin/suppliers',
    query: {
      open: 'groups',
      tool: 'key-plan',
      supplier_id: String(item.supplier_id),
      action_id: String(item.id)
    }
  })
}

function canRunRoutingRefill(item: ActionRecommendation): boolean {
  return item.type === 'routing_refill' && signalNumber(item, 'local_group_id') > 0
}

function canRunLocalAccountScheduleDisable(item: ActionRecommendation): boolean {
  return item.type === 'local_account_schedule_disable' && signalNumber(item, 'local_sub2api_account_id') > 0
}

function canApplyCostReconcileAdjustment(item: ActionRecommendation): boolean {
  return item.type === 'supplier_cost_reconcile_adjustment' &&
    signalNumber(item, 'cost_snapshot_id') > 0 &&
    signalSignedNumber(item, 'balance_delta_cents') !== 0
}

function costReconcileDetailRepairOptions(item: ActionRecommendation): CostReconcileRepairOption[] {
  if (!canApplyCostReconcileAdjustment(item)) return []
  const delta = signalSignedNumber(item, 'balance_delta_cents')
  if (delta > 0) {
    return [
      { type: 'funding_credit', label: '补充值' },
      { type: 'entitlement_credit', label: '补兑换' }
    ]
  }
  if (delta < 0) {
    return [
      { type: 'refund_debit', label: '补退款' },
      { type: 'usage_cost', label: '补 usage' }
    ]
  }
  return []
}

function executeButtonLabel(item: ActionRecommendation): string {
  if (item.type === 'routing_refill') return '补入最低倍率'
  if (item.type === 'local_account_schedule_disable') return '关闭调度'
  if (item.type === 'supplier_cost_reconcile_adjustment') return '对账调整'
  return '执行'
}

function openOpportunitySupplier(item: BalanceOpportunity) {
  void router.push({
    path: '/admin/suppliers',
    query: {
      open: 'groups',
      supplier_id: String(item.supplier_id),
      q: item.supplier_name
    }
  })
}

async function refreshOpportunityBalance(item: BalanceOpportunity) {
  if (balanceRefreshingID.value !== null || balanceBatchRefreshing.value) return
  balanceRefreshingID.value = item.supplier_id
  try {
    const balance = await getSupplierCurrentBalance(item.supplier_id, { refresh: true })
    applySupplierBalance(item.supplier_id, balance)
    await loadPage()
    appStore.showSuccess(`${item.supplier_name} 余额已刷新，候选状态已重算`)
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '刷新余额失败')
  } finally {
    balanceRefreshingID.value = null
  }
}

async function refreshAllOpportunityBalances() {
  if (balanceBatchRefreshing.value || balanceRefreshingID.value !== null) return
  const supplierIDs = balanceOpportunitySupplierIDs.value
  if (supplierIDs.length === 0) return

  balanceBatchRefreshing.value = true
  balanceBatchRefreshDone.value = 0
  balanceBatchRefreshTotal.value = supplierIDs.length
  balanceBatchRefreshFailed.value = 0
  balanceBatchRefreshingSupplierIDs.value = new Set()

  try {
    await runWithConcurrency(supplierIDs, 3, async (supplierID) => {
      markBatchRefreshingSupplier(supplierID, true)
      try {
        const balance = await getSupplierCurrentBalance(supplierID, { refresh: true })
        applySupplierBalance(supplierID, balance)
      } catch {
        balanceBatchRefreshFailed.value += 1
      } finally {
        balanceBatchRefreshDone.value += 1
        markBatchRefreshingSupplier(supplierID, false)
      }
    })

    await loadPage()
    const successCount = supplierIDs.length - balanceBatchRefreshFailed.value
    if (balanceBatchRefreshFailed.value > 0) {
      appStore.showError(`余额批量刷新完成：成功 ${successCount} 个，失败 ${balanceBatchRefreshFailed.value} 个`)
    } else {
      appStore.showSuccess(`余额批量刷新完成：${successCount} 个供应商已重算候选`)
    }
  } finally {
    balanceBatchRefreshing.value = false
    balanceBatchRefreshingSupplierIDs.value = new Set()
  }
}

function applySupplierBalance(
  supplierID: number,
  balance: { balance_cents: number; currency: string; runtime_status: Supplier['runtime_status']; captured_at?: string | null }
) {
  const supplier = suppliers.value.find((current) => current.id === supplierID)
  if (!supplier) return
  supplier.balance_cents = balance.balance_cents
  supplier.balance_currency = balance.currency
  supplier.runtime_status = balance.runtime_status
  supplier.balance_updated_at = balance.captured_at
}

function isBalanceRefreshing(supplierID: number): boolean {
  return balanceRefreshingID.value === supplierID || balanceBatchRefreshingSupplierIDs.value.has(supplierID)
}

function isBalanceRefreshLocked(supplierID: number): boolean {
  return balanceBatchRefreshing.value || balanceRefreshingID.value === supplierID
}

function markBatchRefreshingSupplier(supplierID: number, refreshing: boolean) {
  const next = new Set(balanceBatchRefreshingSupplierIDs.value)
  if (refreshing) {
    next.add(supplierID)
  } else {
    next.delete(supplierID)
  }
  balanceBatchRefreshingSupplierIDs.value = next
}

async function runWithConcurrency<T>(items: T[], concurrency: number, worker: (item: T) => Promise<void>) {
  let nextIndex = 0
  const workerCount = Math.min(Math.max(concurrency, 1), items.length)
  await Promise.all(Array.from({ length: workerCount }, async () => {
    while (nextIndex < items.length) {
      const item = items[nextIndex]
      nextIndex += 1
      await worker(item)
    }
  }))
}

async function syncOpportunityChannel(item: BalanceOpportunity) {
  if (channelSyncingID.value !== null) return
  channelSyncingID.value = item.supplier_id
  try {
    await syncSupplierChannelChecks(item.supplier_id, {
      candidate_limit: 3,
      auto_pause_on_failure: false
    })
    appStore.showSuccess(`${item.supplier_name} 通道检测任务已提交`)
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '提交通道检测失败')
  } finally {
    channelSyncingID.value = null
  }
}

async function loadPage() {
  loading.value = true
  try {
    const [supplierResult, rateResult, balanceResult, healthResult, kanbanResult, localOpsResult, localGroupResult, settingsResult, costSnapshotResult, actionResult] = await Promise.all([
      listSuppliers(),
      listRateSnapshots({ limit: 200 }),
      listBalanceEvents({ limit: 100 }),
      listHealthEvents({ limit: 100 }),
      listKanbanEvents({ status: 'open', limit: 100 }),
      listLocalAccountOps({ limit: 1000 }),
      listLocalSub2APIGroups({ limit: 1000 }),
      getSchedulerSettings().catch(() => null),
      listSupplierCostSnapshots({ page: 1, page_size: 200 }),
      listActionRecommendations({
        page: pagination.page,
        page_size: pagination.page_size,
        recommendation_id: focusedRecommendationID.value || undefined,
        type: actionTypeFilter.value || undefined,
        local_group_id: actionObjectFilter.value.localGroupID || undefined,
        local_sub2api_account_id: actionObjectFilter.value.localAccountID || undefined
      })
    ])
    suppliers.value = supplierResult.items
    rateSnapshots.value = rateResult.items
    balanceEvents.value = balanceResult.items
    healthEvents.value = healthResult.items
    kanbanEvents.value = kanbanResult.items
    localAccountOpsRows.value = localOpsResult.items
    localGroups.value = localGroupResult.items || []
    schedulerSettings.value = settingsResult
    supplierCostSnapshots.value = costSnapshotResult.items
    recommendations.value = actionResult.items
    pagination.total = actionResult.total || 0
    pagination.pages = actionResult.pages || 0
    pagination.page = actionResult.page || pagination.page
    pagination.page_size = actionResult.page_size || pagination.page_size
    await applyFocusedExecutionDeepLink()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '加载动作建议失败')
  } finally {
    loading.value = false
  }
}

watch(() => [actionTypeFilter.value, actionObjectFilter.value.localGroupID, actionObjectFilter.value.localAccountID, focusedRecommendationID.value, focusedExecutionID.value], () => {
  pagination.page = 1
  void loadPage()
})

function handlePageChange(page: number) {
  pagination.page = page
  void loadPage()
}

function handlePageSizeChange(pageSize: number) {
  pagination.page_size = pageSize
  pagination.page = 1
  void loadPage()
}

function supplierSignals(): SupplierSignal[] {
  return suppliers.value.map((supplier) => ({
    supplier_id: supplier.id,
    name: supplier.name,
    runtime_status: supplier.runtime_status,
    health_status: supplier.health_status,
    balance_cents: supplier.balance_cents,
    currency: supplier.balance_currency,
    effective_cost_cents: estimateCost(supplier.id),
    key_limit_policy: supplier.key_limit_policy,
    key_limit_value: supplier.key_limit_value,
    active_key_count: supplier.active_key_count,
    key_capacity_status: supplier.key_capacity_status
  }))
}

function clearActionTypeFilter() {
  void router.push({
    path: '/admin/actions',
    query: {
      ...route.query,
      type: undefined,
      local_group_id: undefined,
      local_sub2api_account_id: undefined,
      recommendation_id: undefined,
      execution_id: undefined
    }
  })
}

function normalizeActionTypeQuery(value: unknown): ActionRecommendation['type'] | '' {
  const raw = Array.isArray(value) ? value[0] : value
  const normalized = String(raw || '').trim()
  switch (normalized) {
    case 'routing_refill':
    case 'local_account_schedule_disable':
    case 'local_account_manual_ops':
    case 'supplier_cost_reconcile_adjustment':
    case 'switch_supplier':
    case 'pause_supplier':
    case 'degrade_supplier':
    case 'increase_weight':
    case 'recharge_supplier':
    case 'investigate_profit':
    case 'review_credential':
      return normalized
    default:
      return ''
  }
}

function positiveQueryNumber(value: unknown): number {
  const raw = Array.isArray(value) ? value[0] : value
  const next = Number(raw)
  return Number.isFinite(next) && next > 0 ? next : 0
}

function stringQuery(value: unknown): string {
  const raw = Array.isArray(value) ? value[0] : value
  return String(raw || '').trim()
}

function actionTypeLabel(value: ActionRecommendation['type'] | ''): string {
  switch (value) {
    case 'routing_refill':
      return '补池建议'
    case 'local_account_schedule_disable':
      return '关闭调度建议'
    case 'local_account_manual_ops':
      return '本地账号手工操作'
    case 'supplier_cost_reconcile_adjustment':
      return '成本对账调整'
    case 'switch_supplier':
      return '供应商切换'
    case 'pause_supplier':
      return '暂停供应商'
    case 'degrade_supplier':
      return '降权供应商'
    case 'increase_weight':
      return '提高权重'
    case 'recharge_supplier':
      return '充值建议'
    case 'investigate_profit':
      return '利润排查'
    case 'review_credential':
      return '凭据排查'
    default:
      return '全部建议'
  }
}

function estimateCost(supplierID: number): number {
  const prices = rateSnapshots.value
    .filter((item) => item.supplier_id === supplierID)
    .map((item) => item.price_micros)
    .filter((value) => value > 0)
  if (prices.length === 0) return 0
  return Math.round(Math.min(...prices) / 10000)
}

function candidateEvaluationSignals() {
  return localAccountOpsRows.value
    .filter((row) => row.supplier_id && row.candidate_status && row.candidate_status !== 'available')
    .filter((row) => !isLocalAccountScheduleDisableRow(row))
    .map((row) => ({
      supplier_id: row.supplier_id,
      supplier_group_id: row.supplier_group_id,
      local_sub2api_account_id: row.local_sub2api_account_id,
      candidate_status: row.candidate_status,
      blocked_reason: row.blocked_reason,
      check_source: row.check_source,
      balance_status: row.balance_status,
      key_capacity_status: row.key_capacity_status,
      effective_rate_multiplier: row.effective_rate_multiplier
    }))
}

function localGroupCapacitySignals(): LocalGroupCapacitySignal[] {
  const threshold = routingRefillLowCapacityThreshold()
  return localGroups.value
    .filter((group) => Number(group.active_api_key_count || 0) > 0)
    .filter((group) => Number(group.schedulable_accounts || 0) <= threshold)
    .map((group) => {
      const candidate = bestRoutingRefillCandidateForGroup(group)
      return {
        local_group_id: group.id,
        local_group_name: group.name,
        platform: group.platform,
        total_accounts: group.total_accounts,
        schedulable_accounts: group.schedulable_accounts,
        active_api_key_count: group.active_api_key_count,
        rate_multiplier: group.rate_multiplier,
        low_capacity_threshold: threshold,
        best_candidate_supplier_id: Number(candidate?.supplier_id || 0),
        best_candidate_supplier_group_id: Number(candidate?.supplier_group_id || 0),
        best_candidate_local_account_id: Number(candidate?.local_sub2api_account_id || 0),
        best_candidate_rate_multiplier: Number(candidate?.effective_rate_multiplier || 0),
        best_candidate_check_source: candidate?.check_source,
        best_candidate_supplier_name: candidate?.supplier_name,
        best_candidate_supplier_group_name: candidate?.supplier_group_name
      }
    })
}

function localAccountScheduleSignals(): LocalAccountScheduleSignal[] {
  const seen = new Set<number>()
  const signals: LocalAccountScheduleSignal[] = []
  for (const row of localAccountOpsRows.value) {
    const accountID = Number(row.local_sub2api_account_id || 0)
    if (accountID <= 0 || seen.has(accountID)) continue
    if (!isLocalAccountScheduleDisableRow(row)) continue
    seen.add(accountID)
    signals.push({
      supplier_id: Number(row.supplier_id || 0),
      supplier_group_id: Number(row.supplier_group_id || 0),
      local_sub2api_account_id: accountID,
      local_account_name: row.local_account_name,
      supplier_name: row.supplier_name,
      supplier_group_name: row.supplier_group_name,
      local_group_ids: row.local_account_group_ids || [],
      local_group_names: row.local_account_group_names || [],
      local_account_schedulable: row.local_account_schedulable,
      candidate_status: row.candidate_status,
      blocked_reason: row.blocked_reason,
      check_source: row.check_source,
      balance_status: row.balance_status,
      key_capacity_status: row.key_capacity_status,
      channel_check_status: row.channel_check_status,
      effective_rate_multiplier: Number(row.effective_rate_multiplier || 0)
    })
  }
  return signals
}

function isLocalAccountScheduleDisableRow(row: LocalAccountOpsRow): boolean {
  if (!row.local_account_schedulable) return false
  if ((row.local_account_group_ids || []).length === 0) return false
  const status = String(row.candidate_status || '').toLowerCase()
  const reason = String(row.blocked_reason || '').toLowerCase()
  return status === 'blocked' && localAccountScheduleDisableReason(reason)
}

function localAccountScheduleDisableReason(reason: string): boolean {
  return reason === 'channel_monitor_failed' || reason === 'channel_active_probe_failed'
}

function routingRefillLowCapacityThreshold(): number {
  const value = Number(schedulerSettings.value?.routing_refill_low_capacity_threshold || 1)
  if (!Number.isFinite(value) || value <= 0) return 1
  return Math.max(1, Math.floor(value))
}

function bestRoutingRefillCandidateForGroup(group: LocalSub2APIGroup): LocalAccountOpsRow | null {
  const platform = String(group.platform || '').trim().toLowerCase()
  const maxRateMultiplier = routingRefillMaxRateMultiplier()
  const candidates = localAccountOpsRows.value
    .filter((row) => String(row.candidate_status || '').toLowerCase() === 'available')
    .filter((row) => !platform || String(row.local_account_platform || '').trim().toLowerCase() === platform)
    .filter((row) => !(row.local_account_group_ids || []).includes(group.id))
    .filter((row) => Number(row.effective_rate_multiplier || 0) > 0)
    .filter((row) => maxRateMultiplier <= 0 || Number(row.effective_rate_multiplier || 0) <= maxRateMultiplier)
  if (candidates.length === 0) return null
  return [...candidates].sort((left, right) => {
    const leftRate = Number(left.effective_rate_multiplier || 0)
    const rightRate = Number(right.effective_rate_multiplier || 0)
    if (leftRate !== rightRate) return leftRate - rightRate
    return Number(left.local_sub2api_account_id || 0) - Number(right.local_sub2api_account_id || 0)
  })[0]
}

function routingRefillMaxRateMultiplier(): number {
  const value = Number(schedulerSettings.value?.routing_refill_max_rate_multiplier || 0)
  if (!Number.isFinite(value) || value <= 0) return 0
  return value
}

function costSnapshotSignals(): SupplierCostSnapshot[] {
  return supplierCostSnapshots.value.filter((snapshot) => {
    const actualBalance = snapshot.actual_balance_cents
    const delta = snapshot.balance_delta_cents
    return actualBalance !== null &&
      actualBalance !== undefined &&
      delta !== null &&
      delta !== undefined &&
      Number(delta) !== 0
  })
}

function buildBalanceOpportunities(rows: LocalAccountOpsRow[]): BalanceOpportunity[] {
  const byKey = new Map<string, BalanceOpportunity>()
  for (const row of rows) {
    if (!isBalanceOpportunityRow(row)) continue
    const supplierID = Number(row.supplier_id || 0)
    if (supplierID <= 0) continue
    const key = [
      supplierID,
      row.supplier_group_id || 0,
      row.local_sub2api_account_id || 0
    ].join(':')
    if (byKey.has(key)) continue
    byKey.set(key, {
      key,
      supplier_id: supplierID,
      supplier_name: row.supplier_name || supplierName(supplierID),
      supplier_group_id: row.supplier_group_id,
      supplier_group_name: row.supplier_group_name || row.supplier_external_group_id || `第三方分组 #${row.supplier_group_id || '-'}`,
      local_sub2api_account_id: row.local_sub2api_account_id,
      local_account_name: row.local_account_name || `本地账号 #${row.local_sub2api_account_id}`,
      local_group_label: (row.local_account_group_names || []).join(' / ') || '未加入本地分组',
      effective_rate_multiplier: Number(row.effective_rate_multiplier || 0),
      balance_cents: Number(row.balance_cents || 0),
      balance_currency: row.balance_currency || 'USD',
      blocked_reason: row.blocked_reason,
      check_source: row.check_source
    })
  }
  return Array.from(byKey.values()).sort((left, right) => {
    if (left.effective_rate_multiplier !== right.effective_rate_multiplier) {
      return left.effective_rate_multiplier - right.effective_rate_multiplier
    }
    if (left.balance_cents !== right.balance_cents) {
      return left.balance_cents - right.balance_cents
    }
    return left.supplier_id - right.supplier_id
  })
}

function isBalanceOpportunityRow(row: LocalAccountOpsRow): boolean {
  const status = String(row.candidate_status || '').toLowerCase()
  const reason = String(row.blocked_reason || '').toLowerCase()
  const balanceStatus = String(row.balance_status || '').toLowerCase()
  return status === 'balance_blocked' ||
    reason === 'recharge_required' ||
    balanceStatus === 'insufficient' ||
    balanceStatus === 'recharge_required' ||
    balanceStatus === 'balance_blocked'
}

function multiplierLabel(value?: number): string {
  const number = Number(value || 0)
  if (!Number.isFinite(number) || number <= 0) return '-'
  return `${number.toFixed(4).replace(/0+$/, '').replace(/\.$/, '')}x`
}

function moneyLabel(cents: number, currency: string): string {
  const value = Number(cents || 0) / 100
  return `${value.toLocaleString('zh-CN', { maximumFractionDigits: 2 })} ${currency || 'USD'}`
}

function candidateReasonText(value?: string): string {
  if (value === 'recharge_required') return '余额不足'
  if (value === 'balance_unknown') return '余额未知'
  return value || '余额阻断'
}

async function generate() {
  generating.value = true
  try {
    const result = await generateActions({
      suppliers: supplierSignals(),
      candidate_evaluations: candidateEvaluationSignals(),
      local_group_capacity: localGroupCapacitySignals(),
      local_account_schedule: localAccountScheduleSignals(),
      balance_events: balanceEvents.value.filter((event) => event.status === 'open'),
      health_events: healthEvents.value.filter((event) => event.status === 'open'),
      kanban_events: kanbanEvents.value.filter((event) => event.status === 'open'),
      cost_snapshots: costSnapshotSignals(),
      min_profit_margin: 0.1
    })
    recommendations.value = result.items
    appStore.showSuccess('建议已生成')
    await loadPage()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '生成建议失败')
  } finally {
    generating.value = false
  }
}

async function updateStatus(id: number, status: ActionRecommendation['status']) {
  try {
    await updateActionRecommendationStatus(id, status)
    appStore.showSuccess('状态已更新')
    await loadPage()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '更新状态失败')
  }
}

async function previewRoutingRefillRecommendation(item: ActionRecommendation) {
  if (!canRunRoutingRefill(item) || routingRefillPreviewingID.value !== null) return
  routingRefillPreviewingID.value = item.id
  try {
    const result = await refillLocalGroup(routingRefillPayload(item, true))
    routingRefillResults.value = { ...routingRefillResults.value, [item.id]: result }
    if (result.skipped_reason) {
      appStore.showWarning(routingRefillSkippedReasonLabel(result.skipped_reason))
      return
    }
    appStore.showSuccess('已生成补池预览')
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '补池预览失败')
  } finally {
    routingRefillPreviewingID.value = null
  }
}

async function executeRecommendation(item: ActionRecommendation) {
  executingID.value = item.id
  try {
    if (item.type === 'routing_refill') {
      await executeRoutingRefillRecommendation(item)
      return
    }
    if (item.type === 'local_account_schedule_disable') {
      await executeLocalAccountScheduleDisableRecommendation(item)
      return
    }
    if (item.type === 'supplier_cost_reconcile_adjustment') {
      await executeCostReconcileAdjustmentRecommendation(item)
      return
    }
    const execution = await executeActionRecommendation(item.id, {
      ...schedulerSourcePayload(),
      request_payload: {
        source: 'admin_plus_action_recommendations_view',
        action_type: item.type,
        reason_code: item.reason_code
      }
    })
    lastExecutions.value = { ...lastExecutions.value, [item.id]: execution }
    if (execution.status === 'unsupported') {
      appStore.showError('该动作暂未接入自动执行器，已写入回执')
    } else {
      appStore.showSuccess('动作执行回执已写入')
    }
    await refreshRecommendationExecutionHistory(item.id)
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '执行动作失败')
  } finally {
    executingID.value = null
  }
}

async function previewLocalAccountScheduleDisableRecommendation(item: ActionRecommendation) {
  if (!canRunLocalAccountScheduleDisable(item) || localAccountSchedulePreviewingID.value !== null) return
  localAccountSchedulePreviewingID.value = item.id
  try {
    const result = await previewLocalAccountOpsAction(localAccountScheduleDisablePayload(item, false))
    localAccountOpsResults.value = { ...localAccountOpsResults.value, [item.id]: result }
    if (result.blocked) {
      appStore.showWarning(localAccountOpsBlockedReasonLabel(result.blocked_reason))
      return
    }
    appStore.showSuccess('已生成关闭调度预览')
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '关闭调度预览失败')
  } finally {
    localAccountSchedulePreviewingID.value = null
  }
}

async function executeRoutingRefillRecommendation(item: ActionRecommendation) {
  if (!canRunRoutingRefill(item)) {
    throw new Error('动作缺少本地分组信号，无法执行补池')
  }
  const result = await refillLocalGroup(routingRefillPayload(item, false))
  routingRefillResults.value = { ...routingRefillResults.value, [item.id]: result }
  if (result.skipped_reason) {
    appStore.showWarning(routingRefillSkippedReasonLabel(result.skipped_reason))
  } else {
    appStore.showSuccess('已补入最低倍率候选')
  }
  await refreshRecommendationExecutionHistory(item.id)
}

async function executeLocalAccountScheduleDisableRecommendation(item: ActionRecommendation) {
  if (!canRunLocalAccountScheduleDisable(item)) {
    throw new Error('动作缺少本地账号信号，无法关闭调度')
  }
  const result = await applyLocalAccountOpsAction(localAccountScheduleDisablePayload(item, true))
  localAccountOpsResults.value = { ...localAccountOpsResults.value, [item.id]: result }
  if (result.blocked) {
    appStore.showWarning(localAccountOpsBlockedReasonLabel(result.blocked_reason))
  } else {
    appStore.showSuccess('已关闭本地账号调度')
  }
  await refreshRecommendationExecutionHistory(item.id)
}

async function executeCostReconcileAdjustmentRecommendation(item: ActionRecommendation) {
  if (!canApplyCostReconcileAdjustment(item)) {
    throw new Error('动作缺少成本快照差额信号，无法执行对账调整')
  }
  const result = await applyCostReconcileAdjustment(item.id, {
    snapshot_id: signalNumber(item, 'cost_snapshot_id'),
    adjustment_amount_cents: signalSignedNumber(item, 'balance_delta_cents'),
    reason: 'action_recommendation',
    ...schedulerSourcePayload()
  })
  costReconcileResults.value = { ...costReconcileResults.value, [item.id]: result }
  appStore.showSuccess('已记录成本对账调整')
  await refreshRecommendationExecutionHistory(item.id)
}

async function executeCostReconcileDetailRepairRecommendation(item: ActionRecommendation, detailType: CostReconcileDetailType) {
  if (!canApplyCostReconcileAdjustment(item)) {
    throw new Error('动作缺少成本快照差额信号，无法修复对账明细')
  }
  executingID.value = item.id
  try {
    const delta = signalSignedNumber(item, 'balance_delta_cents')
    const result = await applyCostReconcileDetailRepair(item.id, {
      snapshot_id: signalNumber(item, 'cost_snapshot_id'),
      detail_type: detailType,
      amount_cents: Math.abs(delta),
      model: detailType === 'usage_cost' ? 'manual-reconcile' : undefined,
      reason: 'action_recommendation',
      ...schedulerSourcePayload()
    })
    costDetailRepairResults.value = { ...costDetailRepairResults.value, [item.id]: result }
    appStore.showSuccess('已写入成本对账明细并刷新快照')
    await refreshRecommendationExecutionHistory(item.id)
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '修复成本对账明细失败')
  } finally {
    executingID.value = null
  }
}

async function refreshRecommendationExecutionHistory(id: number) {
  await loadPage()
  expandedExecutionID.value = id
  const state = executionState(id)
  state.page = 1
  await loadExecutions(id)
  if (state.items[0]) {
    lastExecutions.value = { ...lastExecutions.value, [id]: state.items[0] }
  }
}

function routingRefillPayload(item: ActionRecommendation, dryRun: boolean) {
  const localGroupID = signalNumber(item, 'local_group_id')
  const group = localGroups.value.find((current) => current.id === localGroupID)
  const maxRateMultiplier = routingRefillMaxRateMultiplier()
  return {
    local_group_id: localGroupID,
    platform: signalValue(item, 'platform') || group?.platform,
    max_rate_multiplier: Number.isFinite(maxRateMultiplier) && maxRateMultiplier > 0 ? maxRateMultiplier : undefined,
    limit: 1000,
    dry_run: dryRun,
    action_id: item.id,
    ...schedulerSourcePayload(),
    reason: 'action_recommendation',
    trigger_type: 'manual_action_recommendation',
    cooldown_seconds: schedulerSettings.value?.routing_refill_cooldown_seconds,
    confirm_window_seconds: schedulerSettings.value?.routing_refill_confirm_window_seconds
  }
}

function localAccountScheduleDisablePayload(item: ActionRecommendation, includeActionID: boolean) {
  return {
    action: 'set_schedulable' as const,
    account_ids: [signalNumber(item, 'local_sub2api_account_id')],
    schedulable: false,
    allow_empty_pool: false,
    action_id: includeActionID ? item.id : undefined,
    ...schedulerSourcePayload(),
    reason: 'action_recommendation'
  }
}

function schedulerSourcePayload() {
  const payload: { scheduler_run_id?: string; scheduler_step_id?: number } = {}
  if (schedulerSource.value.runID) {
    payload.scheduler_run_id = schedulerSource.value.runID
  }
  if (schedulerSource.value.stepID > 0) {
    payload.scheduler_step_id = schedulerSource.value.stepID
  }
  return payload
}

function executionSchedulerRoute(execution: ActionExecution) {
  return {
    path: '/admin/scheduler',
    query: {
      run_id: execution.scheduler_run_id,
      ...(execution.scheduler_step_id ? { step_id: String(execution.scheduler_step_id) } : {})
    }
  }
}

function executionSchedulerLabel(execution: ActionExecution): string {
  const runID = String(execution.scheduler_run_id || '').trim()
  const suffix = execution.scheduler_step_id ? ` / Step ${execution.scheduler_step_id}` : ''
  return `${shortRunID(runID)}${suffix}`
}

function shortRunID(value: string): string {
  if (value.length <= 24) return value
  return `${value.slice(0, 12)}...${value.slice(-8)}`
}

function routingRefillSummary(result: RoutingRefillResult): string {
  if (result.skipped_reason) return `补池结果：${routingRefillSkippedReasonLabel(result.skipped_reason)}`
  const before = result.availability_before?.schedulable_accounts
  const after = result.availability_after?.schedulable_accounts
  const candidate = result.candidate?.effective_rate_multiplier
  const capacity = before !== undefined || after !== undefined ? ` · 可调度 ${before ?? '-'} -> ${after ?? '-'}` : ''
  const rate = candidate ? ` · 候选 ${multiplierLabel(candidate)}` : ''
  return `${result.dry_run ? '补池预览' : '补池完成'}${capacity}${rate}`
}

function localAccountOpsSummary(result: LocalAccountOpsActionResult): string {
  if (result.blocked) return `关闭调度阻断：${localAccountOpsBlockedReasonLabel(result.blocked_reason)}`
  const impacts = result.group_impacts || []
  const impact = impacts.length > 0
    ? ` · 影响分组 ${impacts.length} 个`
    : ''
  return `${result.dry_run ? '关闭调度预览' : '关闭调度完成'} · 账号 ${result.account_ids?.length || 0} 个 · 更新 ${result.updated_accounts || 0} 个${impact}`
}

function costReconcileSummary(result: CostReconcileAdjustmentResult): string {
  const afterDelta = result.after_snapshot?.balance_delta_cents
  const after = afterDelta === null || afterDelta === undefined
    ? ''
    : ` · 调整后差额 ${moneyLabel(Number(afterDelta || 0), result.currency)}`
  return `对账调整完成 · ${moneyLabel(result.adjustment_amount_cents, result.currency)}${after}`
}

function costDetailRepairSummary(result: CostReconcileDetailRepairResult): string {
  const afterDelta = result.after_snapshot?.balance_delta_cents
  const after = afterDelta === null || afterDelta === undefined
    ? ''
    : ` · 修复后差额 ${moneyLabel(Number(afterDelta || 0), result.currency)}`
  return `${costDetailRepairLabel(result.detail_type)}完成 · ${moneyLabel(result.amount_cents, result.currency)}${after}`
}

function costDetailRepairLabel(type: string): string {
  if (type === 'funding_credit') return '补充值'
  if (type === 'entitlement_credit') return '补兑换'
  if (type === 'refund_debit') return '补退款'
  if (type === 'usage_cost') return '补 usage'
  return '明细修复'
}

function localAccountOpsBlockedReasonLabel(reason?: string): string {
  if (reason === 'LOCAL_ACCOUNT_STATE_DRIFT_PENDING') return '检测到原后台变更，请先同步并采纳或恢复本地状态'
  if (reason === 'LOCAL_GROUP_SCHEDULABLE_POOL_WOULD_BE_EMPTY') return '关闭后会导致有用户 Key 的本地分组空池'
  return reason || '操作被安全保护阻断'
}

function signalValue(item: ActionRecommendation, key: string): string {
  const prefix = `${key}=`
  return (item.signals || [])
    .map((signal) => String(signal || '').trim())
    .find((signal) => signal.startsWith(prefix))
    ?.slice(prefix.length)
    .trim() || ''
}

function signalNumber(item: ActionRecommendation, key: string): number {
  const value = Number(signalValue(item, key))
  return Number.isFinite(value) && value > 0 ? value : 0
}

function signalSignedNumber(item: ActionRecommendation, key: string): number {
  const value = Number(signalValue(item, key))
  return Number.isFinite(value) ? value : 0
}

function executionSummary(execution: ActionExecution): string {
  const response = payloadPreview(execution.response_payload)
  if (response !== '-') return response
  const request = payloadPreview(execution.request_payload)
  if (request !== '-') return `请求 ${request}`
  return execution.error_message || '-'
}

function executionAuditSummary(execution: ActionExecution): string {
  const parts: string[] = []
  if (execution.idempotency_key_hash) {
    parts.push(`幂等 ${shortHash(execution.idempotency_key_hash)}`)
  }
  if (execution.idempotency_replayed) {
    parts.push('replay')
  }
  return parts.join(' · ')
}

function executionSnapshotSummary(execution: ActionExecution): string {
  const before = payloadPreview(execution.before_snapshot)
  const after = payloadPreview(execution.after_snapshot)
  if (before === '-' && after === '-') return ''
  return `前 ${before} / 后 ${after}`
}

function shortHash(value: string): string {
  const normalized = String(value || '').trim()
  if (normalized.length <= 12) return normalized
  return `${normalized.slice(0, 8)}...${normalized.slice(-4)}`
}

function payloadNumber(payload: Record<string, unknown> | undefined, key: string): number {
  const value = payload?.[key]
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value === 'string') {
    const parsed = Number(value)
    return Number.isFinite(parsed) ? parsed : 0
  }
  return 0
}

function payloadPreview(payload?: Record<string, unknown>): string {
  if (!payload || Object.keys(payload).length === 0) return '-'
  return Object.entries(payload)
    .slice(0, 5)
    .map(([key, value]) => `${key}=${payloadValuePreview(key, value)}`)
    .join(' · ')
}

function payloadValuePreview(key: string, value: unknown): string {
  if (isSensitivePayloadKey(key)) return '[已隐藏]'
  if (value === null || value === undefined) return '-'
  if (typeof value === 'string') return value.length > 80 ? `${value.slice(0, 77)}...` : value
  if (typeof value === 'number' || typeof value === 'boolean') return String(value)
  if (Array.isArray(value)) return `[${value.length}]`
  return '{...}'
}

function isSensitivePayloadKey(key: string): boolean {
  const normalized = key.toLowerCase()
  return normalized.includes('key') || normalized.includes('token') || normalized.includes('secret') || normalized.includes('password') || normalized.includes('cookie') || normalized.includes('authorization')
}

function formatDateTime(value?: string | null): string {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString('zh-CN', { hour12: false })
}

onMounted(loadPage)
</script>
