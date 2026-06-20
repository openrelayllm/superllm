<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">账单对账</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
            将供应商账单与本地 Sub2API 用量匹配，核算成本、收入、利润和异常。
          </p>
        </div>
        <div class="flex flex-wrap gap-2">
          <button type="button" class="btn btn-secondary" :disabled="loading" @click="loadPage">
            <Icon name="refresh" size="sm" />
            刷新
          </button>
          <button type="button" class="btn btn-secondary" @click="openImportDialog">
            <Icon name="upload" size="sm" />
            导入账单
          </button>
          <button type="button" class="btn btn-primary" :disabled="reconciling || billPagination.total === 0" @click="openUsageDialog">
            <Icon name="play" size="sm" />
            读取用量并对账
          </button>
        </div>
      </section>

      <section class="grid gap-4 sm:grid-cols-2 xl:grid-cols-6">
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">供应商账单</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ summarySupplierLines }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">本地用量</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ summaryLocalLines }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">成本</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ formatMoney(summaryCostCents, defaultCurrency) }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">收入</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ formatMoney(summaryRevenueCents, defaultCurrency) }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">利润</p>
          <p class="mt-2 text-2xl font-semibold" :class="profitCents >= 0 ? 'text-emerald-600 dark:text-emerald-400' : 'text-rose-600 dark:text-rose-400'">
            {{ result ? formatMoney(profitCents, defaultCurrency) : '-' }}
          </p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">异常</p>
          <p class="mt-2 text-2xl font-semibold" :class="anomalyCount > 0 ? 'text-rose-600 dark:text-rose-400' : 'text-emerald-600 dark:text-emerald-400'">
            {{ result ? anomalyCount : '-' }}
          </p>
        </div>
      </section>

      <section class="card overflow-hidden">
        <div class="flex flex-col gap-4 border-b border-gray-100 px-5 py-4 dark:border-dark-700 lg:flex-row lg:items-center lg:justify-between">
          <div class="inline-flex w-fit rounded-lg border border-gray-200 bg-gray-50 p-1 dark:border-dark-700 dark:bg-dark-800">
            <button
              v-for="tab in tabs"
              :key="tab.value"
              type="button"
              class="rounded-md px-3 py-1.5 text-sm font-medium transition"
              :class="activeTab === tab.value ? 'bg-white text-primary-600 shadow-sm dark:bg-dark-700 dark:text-primary-300' : 'text-gray-600 hover:text-gray-900 dark:text-dark-300 dark:hover:text-white'"
              @click="activeTab = tab.value"
            >
              {{ tab.label }}
            </button>
          </div>

          <div v-if="activeTab === 'bills'" class="flex flex-col gap-2 sm:flex-row sm:items-center">
            <select v-model.number="billFilter.supplier_id" class="input h-9 min-w-[220px] py-1 text-sm" @change="handleBillFilterChange">
              <option :value="0">全部供应商</option>
              <option v-for="supplier in suppliers" :key="supplier.id" :value="supplier.id">{{ supplier.name }}</option>
            </select>
            <button type="button" class="btn btn-secondary btn-sm" @click="openImportDialog">导入账单</button>
          </div>

          <div v-else-if="activeTab === 'usage'" class="flex flex-col gap-2 sm:flex-row sm:items-center">
            <span class="text-sm text-gray-500 dark:text-dark-400">{{ usageRangeLabel }}</span>
            <button type="button" class="btn btn-secondary btn-sm" @click="openUsageDialog">重新读取</button>
          </div>

          <div v-else class="flex flex-col gap-2 sm:flex-row sm:items-center">
            <select
              v-model="resultStatusFilter"
              class="input h-9 min-w-[180px] py-1 text-sm"
              :disabled="!result"
              @change="resetResultPagination"
            >
              <option value="anomaly">只看异常</option>
              <option value="all">全部结果</option>
              <option value="matched">已匹配</option>
              <option value="supplier_only">仅供应商</option>
              <option value="local_only">仅本地</option>
              <option value="cost_mismatch">成本不一致</option>
              <option value="currency_mismatch">币种不一致</option>
            </select>
            <button type="button" class="btn btn-primary btn-sm" :disabled="reconciling || billPagination.total === 0" @click="openUsageDialog">
              对账
            </button>
          </div>
        </div>

        <div v-if="activeTab === 'result'">
          <div v-if="!result" class="flex flex-col items-center justify-center px-6 py-16 text-center">
            <Icon name="calculator" size="xl" class="mb-4 text-gray-300 dark:text-dark-500" />
            <p class="text-base font-medium text-gray-900 dark:text-white">尚未执行对账</p>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">导入供应商账单后，读取本地用量即可生成成本和利润结果。</p>
            <button type="button" class="btn btn-primary mt-5" :disabled="billPagination.total === 0" @click="openUsageDialog">
              读取用量并对账
            </button>
          </div>
          <template v-else>
            <div class="overflow-x-auto">
              <table class="w-full min-w-[980px] divide-y divide-gray-200 dark:divide-dark-700">
                <thead class="bg-gray-50 dark:bg-dark-800">
                  <tr>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">状态</th>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">请求 ID</th>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">模型</th>
                    <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">成本</th>
                    <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">收入</th>
                    <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">利润</th>
                    <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">利润率</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-900">
                  <tr v-if="visibleReconciliationLines.length === 0">
                    <td colspan="7" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-dark-400">当前筛选下没有对账结果</td>
                  </tr>
                  <tr v-for="line in visibleReconciliationLines" :key="`${line.status}-${line.external_request_id}-${line.supplier_bill_id}-${line.local_usage_id}`">
                    <td class="px-4 py-4">
                      <span class="badge" :class="reconciliationStatusClass(line.status)">{{ reconciliationStatusLabel(line.status) }}</span>
                    </td>
                    <td class="px-4 py-4 font-mono text-xs text-gray-500 dark:text-dark-400">{{ line.external_request_id || '-' }}</td>
                    <td class="px-4 py-4 text-sm text-gray-900 dark:text-gray-100">{{ line.model || '-' }}</td>
                    <td class="px-4 py-4 text-right text-sm text-gray-900 dark:text-gray-100">{{ formatMoney(line.cost_cents, line.currency) }}</td>
                    <td class="px-4 py-4 text-right text-sm text-gray-900 dark:text-gray-100">{{ formatMoney(line.revenue_cents, line.currency) }}</td>
                    <td class="px-4 py-4 text-right text-sm" :class="line.profit_cents >= 0 ? 'text-emerald-600 dark:text-emerald-400' : 'text-rose-600 dark:text-rose-400'">
                      {{ formatMoney(line.profit_cents, line.currency) }}
                    </td>
                    <td class="px-4 py-4 text-right text-sm text-gray-900 dark:text-gray-100">{{ formatPercent(line.profit_margin) }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
            <Pagination
              v-if="resultPagination.total > 0"
              :page="resultPagination.page"
              :total="resultPagination.total"
              :page-size="resultPagination.page_size"
              @update:page="handleResultPageChange"
              @update:pageSize="handleResultPageSizeChange"
            />
          </template>
        </div>

        <div v-else-if="activeTab === 'bills'">
          <div class="overflow-x-auto">
            <table class="w-full min-w-[1500px] divide-y divide-gray-200 dark:divide-dark-700">
              <thead class="bg-gray-50 dark:bg-dark-800">
                <tr>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">供应商</th>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">API Key</th>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">请求 ID</th>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">模型</th>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">端点 / 类型</th>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">计费</th>
                  <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">成本</th>
                  <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">Token 明细</th>
                  <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">耗时</th>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">User-Agent</th>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">时间</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-900">
                <tr v-if="billLines.length === 0">
                  <td colspan="11" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-dark-400">暂无供应商账单</td>
                </tr>
                <tr v-for="line in billLines" :key="line.id">
                  <td class="px-4 py-4 text-sm text-gray-900 dark:text-gray-100">{{ supplierName(line.supplier_id) }}</td>
                  <td class="px-4 py-4 text-sm text-gray-900 dark:text-gray-100">{{ line.api_key_name || '-' }}</td>
                  <td class="px-4 py-4 font-mono text-xs text-gray-500 dark:text-dark-400">{{ line.external_request_id || '-' }}</td>
                  <td class="px-4 py-4 text-sm text-gray-900 dark:text-gray-100">{{ line.model }}</td>
                  <td class="px-4 py-4 text-sm text-gray-900 dark:text-gray-100">
                    <div>{{ line.endpoint || '-' }}</div>
                    <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ line.request_type || '-' }}</div>
                  </td>
                  <td class="px-4 py-4 text-sm text-gray-900 dark:text-gray-100">
                    <div>{{ billingModeLabel(line.billing_mode) }}</div>
                    <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">推理 {{ line.reasoning_effort || '-' }}</div>
                  </td>
                  <td class="px-4 py-4 text-right text-sm text-gray-900 dark:text-gray-100">{{ formatMoney(line.cost_cents, line.currency) }}</td>
                  <td class="px-4 py-4 text-right text-xs text-gray-900 dark:text-gray-100">
                    <div>入 {{ formatInteger(line.input_tokens) }} / 出 {{ formatInteger(line.output_tokens) }}</div>
                    <div class="mt-1 text-gray-500 dark:text-dark-400">缓存 {{ formatInteger(line.cache_read_tokens || 0) }} / 总 {{ formatInteger(totalTokens(line)) }}</div>
                  </td>
                  <td class="px-4 py-4 text-right text-xs text-gray-900 dark:text-gray-100">
                    <div>首 {{ formatDuration(line.first_token_ms) }}</div>
                    <div class="mt-1 text-gray-500 dark:text-dark-400">总 {{ formatDuration(line.duration_ms) }}</div>
                  </td>
                  <td class="max-w-[220px] truncate px-4 py-4 text-sm text-gray-500 dark:text-dark-400" :title="line.user_agent || ''">{{ line.user_agent || '-' }}</td>
                  <td class="px-4 py-4 text-sm text-gray-500 dark:text-dark-400">{{ formatDateTime(line.started_at) }}</td>
                </tr>
              </tbody>
            </table>
          </div>
          <Pagination
            v-if="billPagination.total > 0"
            :page="billPagination.page"
            :total="billPagination.total"
            :page-size="billPagination.page_size"
            @update:page="handleBillPageChange"
            @update:pageSize="handleBillPageSizeChange"
          />
        </div>

        <div v-else>
          <div class="overflow-x-auto">
            <table class="w-full min-w-[920px] divide-y divide-gray-200 dark:divide-dark-700">
              <thead class="bg-gray-50 dark:bg-dark-800">
                <tr>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">本地账号</th>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">请求 ID</th>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">模型</th>
                  <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">收入</th>
                  <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">Token</th>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">时间</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-900">
                <tr v-if="localUsages.length === 0">
                  <td colspan="6" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-dark-400">暂无本地用量</td>
                </tr>
                <tr v-for="line in localUsages" :key="line.id">
                  <td class="px-4 py-4 text-sm text-gray-900 dark:text-gray-100">
                    {{ line.account_name || `#${line.account_id || '-'}` }}
                    <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ line.account_platform || '-' }}</div>
                  </td>
                  <td class="px-4 py-4 font-mono text-xs text-gray-500 dark:text-dark-400">{{ line.external_request_id || '-' }}</td>
                  <td class="px-4 py-4 text-sm text-gray-900 dark:text-gray-100">{{ line.model }}</td>
                  <td class="px-4 py-4 text-right text-sm text-gray-900 dark:text-gray-100">{{ formatMoney(line.revenue_cents, line.currency) }}</td>
                  <td class="px-4 py-4 text-right text-sm text-gray-900 dark:text-gray-100">{{ ((line.input_tokens || 0) + (line.output_tokens || 0)).toLocaleString() }}</td>
                  <td class="px-4 py-4 text-sm text-gray-500 dark:text-dark-400">{{ formatDateTime(line.started_at) }}</td>
                </tr>
              </tbody>
            </table>
          </div>
          <Pagination
            v-if="localUsagePagination.total > 0"
            :page="localUsagePagination.page"
            :total="localUsagePagination.total"
            :page-size="localUsagePagination.page_size"
            @update:page="handleLocalUsagePageChange"
            @update:pageSize="handleLocalUsagePageSizeChange"
          />
        </div>
      </section>
    </div>

    <BaseDialog :show="importDialogOpen" title="导入供应商账单" width="wide" @close="importDialogOpen = false">
      <form id="bill-import-form" class="grid gap-4 md:grid-cols-2" @submit.prevent="importBill">
        <label class="block md:col-span-2">
          <span class="input-label">供应商</span>
          <select v-model.number="billForm.supplier_id" class="input" required>
            <option :value="0" disabled>请选择</option>
            <option v-for="supplier in suppliers" :key="supplier.id" :value="supplier.id">{{ supplier.name }}</option>
          </select>
        </label>
        <label class="block md:col-span-2">
          <span class="input-label">请求 ID</span>
          <input v-model.trim="billForm.external_request_id" class="input" required />
        </label>
        <label class="block">
          <span class="input-label">API Key 名称</span>
          <input v-model.trim="billForm.api_key_name" class="input" placeholder="第三方后台展示的 Key 名称" />
        </label>
        <label class="block">
          <span class="input-label">模型</span>
          <input v-model.trim="billForm.model" class="input" required />
        </label>
        <label class="block">
          <span class="input-label">端点</span>
          <input v-model.trim="billForm.endpoint" class="input" placeholder="/v1/responses" />
        </label>
        <label class="block">
          <span class="input-label">类型</span>
          <input v-model.trim="billForm.request_type" class="input" placeholder="responses / chat" />
        </label>
        <label class="block">
          <span class="input-label">计费模式</span>
          <select v-model="billForm.billing_mode" class="input">
            <option value="token">按 Token</option>
            <option value="request">按次</option>
            <option value="image">图片</option>
          </select>
        </label>
        <label class="block">
          <span class="input-label">推理强度</span>
          <input v-model.trim="billForm.reasoning_effort" class="input" placeholder="low / medium / high" />
        </label>
        <label class="block">
          <span class="input-label">成本</span>
          <input v-model.number="billForm.cost_yuan" type="number" min="0" step="0.01" class="input" required />
        </label>
        <label class="block">
          <span class="input-label">币种</span>
          <input v-model.trim="billForm.currency" class="input" placeholder="CNY" />
        </label>
        <label class="block">
          <span class="input-label">输入 Token</span>
          <input v-model.number="billForm.input_tokens" type="number" min="0" step="1" class="input" />
        </label>
        <label class="block">
          <span class="input-label">输出 Token</span>
          <input v-model.number="billForm.output_tokens" type="number" min="0" step="1" class="input" />
        </label>
        <label class="block">
          <span class="input-label">缓存读取 Token</span>
          <input v-model.number="billForm.cache_read_tokens" type="number" min="0" step="1" class="input" />
        </label>
        <label class="block">
          <span class="input-label">总 Token</span>
          <input v-model.number="billForm.total_tokens" type="number" min="0" step="1" class="input" />
        </label>
        <label class="block">
          <span class="input-label">首 Token 毫秒</span>
          <input v-model.number="billForm.first_token_ms" type="number" min="0" step="1" class="input" />
        </label>
        <label class="block">
          <span class="input-label">总耗时毫秒</span>
          <input v-model.number="billForm.duration_ms" type="number" min="0" step="1" class="input" />
        </label>
        <label class="block md:col-span-2">
          <span class="input-label">User-Agent</span>
          <input v-model.trim="billForm.user_agent" class="input" />
        </label>
      </form>
      <template #footer>
        <button type="button" class="btn btn-secondary" @click="importDialogOpen = false">取消</button>
        <button type="submit" form="bill-import-form" class="btn btn-primary" :disabled="importing">
          {{ importing ? '导入中...' : '导入账单' }}
        </button>
      </template>
    </BaseDialog>

    <BaseDialog :show="usageDialogOpen" title="读取本地用量并对账" width="wide" @close="usageDialogOpen = false">
      <form id="usage-reconcile-form" class="grid gap-4 md:grid-cols-2" @submit.prevent="runRecon">
        <label class="block">
          <span class="input-label">开始时间</span>
          <input v-model="usageForm.from" type="datetime-local" class="input" required />
        </label>
        <label class="block">
          <span class="input-label">结束时间</span>
          <input v-model="usageForm.to" type="datetime-local" class="input" required />
        </label>
        <label class="block md:col-span-2">
          <span class="input-label">模型过滤</span>
          <input v-model.trim="usageForm.model" class="input" placeholder="留空表示全部模型" />
        </label>
        <label class="block">
          <span class="input-label">时间容差秒</span>
          <input v-model.number="reconciliationForm.time_tolerance_seconds" type="number" min="0" class="input" />
        </label>
        <label class="block">
          <span class="input-label">成本差异阈值分</span>
          <input v-model.number="reconciliationForm.cost_mismatch_cents" type="number" min="0" class="input" />
        </label>
      </form>
      <template #footer>
        <button type="button" class="btn btn-secondary" @click="usageDialogOpen = false">取消</button>
        <button type="submit" form="usage-reconcile-form" class="btn btn-primary" :disabled="reconciling || billPagination.total === 0">
          {{ reconciling ? '对账中...' : '读取并对账' }}
        </button>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import Pagination from '@/components/common/Pagination.vue'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { useAppStore } from '@/stores/app'
import {
  importBillLines,
  listBillLines,
  listLocalUsageLines,
  listSuppliers,
  runReconciliation,
  type LocalUsageLine,
  type ReconciliationLine,
  type ReconciliationResult,
  type Supplier,
  type SupplierBillLine
} from '@/api/admin/adminPlus'

type BillingTab = 'result' | 'bills' | 'usage'
type ResultStatusFilter = 'all' | 'anomaly' | ReconciliationLine['status']

const appStore = useAppStore()

const loading = ref(false)
const importing = ref(false)
const reconciling = ref(false)
const importDialogOpen = ref(false)
const usageDialogOpen = ref(false)
const activeTab = ref<BillingTab>('bills')
const resultStatusFilter = ref<ResultStatusFilter>('anomaly')

const suppliers = ref<Supplier[]>([])
const billLines = ref<SupplierBillLine[]>([])
const localUsages = ref<LocalUsageLine[]>([])
const result = ref<ReconciliationResult | null>(null)

const billPagination = reactive({ page: 1, page_size: getPersistedPageSize(), total: 0, pages: 0 })
const localUsagePagination = reactive({ page: 1, page_size: getPersistedPageSize(), total: 0, pages: 0 })
const resultPagination = reactive({ page: 1, page_size: getPersistedPageSize(), total: 0, pages: 0 })

const billFilter = reactive({
  supplier_id: 0
})

const billForm = reactive({
  supplier_id: 0,
  external_request_id: '',
  api_key_name: '',
  model: 'gpt-4o-mini',
  endpoint: '',
  request_type: '',
  billing_mode: 'token',
  reasoning_effort: '',
  cost_yuan: 0,
  currency: 'CNY',
  input_tokens: 0,
  output_tokens: 0,
  cache_read_tokens: 0,
  total_tokens: 0,
  first_token_ms: 0,
  duration_ms: 0,
  user_agent: ''
})

const usageForm = reactive({
  model: '',
  from: toDateTimeLocal(new Date(Date.now() - 24 * 60 * 60 * 1000)),
  to: toDateTimeLocal(new Date())
})

const reconciliationForm = reactive({
  time_tolerance_seconds: 300,
  cost_mismatch_cents: 1
})

const tabs: Array<{ value: BillingTab; label: string }> = [
  { value: 'result', label: '对账结果' },
  { value: 'bills', label: '供应商账单' },
  { value: 'usage', label: '本地用量' }
]

const defaultCurrency = computed(() => billLines.value[0]?.currency || localUsages.value[0]?.currency || 'CNY')
const pageCostCents = computed(() => billLines.value.reduce((sum, line) => sum + line.cost_cents, 0))
const pageRevenueCents = computed(() => localUsages.value.reduce((sum, line) => sum + line.revenue_cents, 0))
const summarySupplierLines = computed(() => result.value?.summary.total_supplier_lines ?? billPagination.total)
const summaryLocalLines = computed(() => result.value?.summary.total_local_lines ?? localUsagePagination.total)
const summaryCostCents = computed(() => result.value?.summary.cost_cents ?? pageCostCents.value)
const summaryRevenueCents = computed(() => result.value?.summary.revenue_cents ?? pageRevenueCents.value)
const profitCents = computed(() => result.value?.summary.profit_cents || 0)
const anomalyCount = computed(() => result.value?.lines.filter((line) => line.status !== 'matched').length || 0)
const filteredReconciliationLines = computed(() => {
  if (!result.value) return []
  if (resultStatusFilter.value === 'all') return result.value.lines
  if (resultStatusFilter.value === 'anomaly') {
    return result.value.lines.filter((line) => line.status !== 'matched')
  }
  return result.value.lines.filter((line) => line.status === resultStatusFilter.value)
})
const visibleReconciliationLines = computed(() => {
  const start = (resultPagination.page - 1) * resultPagination.page_size
  return filteredReconciliationLines.value.slice(start, start + resultPagination.page_size)
})
const usageRangeLabel = computed(() => `${formatDateTime(toRFC3339(usageForm.from))} - ${formatDateTime(toRFC3339(usageForm.to))}`)

function centsFromYuan(value: number): number {
  return Math.round(Number(value || 0) * 100)
}

function formatMoney(cents: number, currency: string): string {
  return new Intl.NumberFormat(undefined, {
    style: 'currency',
    currency: currency || 'CNY',
    minimumFractionDigits: 2
  }).format((cents || 0) / 100)
}

function formatPercent(value?: number | null): string {
  if (value === null || value === undefined) return '-'
  return `${(value * 100).toFixed(2)}%`
}

function formatDateTime(value?: string | null): string {
  if (!value) return '-'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '-' : date.toLocaleString()
}

function formatInteger(value?: number | null): string {
  return Number(value || 0).toLocaleString()
}

function totalTokens(line: SupplierBillLine): number {
  return line.total_tokens || line.input_tokens + line.output_tokens + (line.cache_read_tokens || 0)
}

function formatDuration(value?: number | null): string {
  const ms = Number(value || 0)
  if (ms <= 0) return '-'
  if (ms >= 1000) return `${(ms / 1000).toFixed(2)}s`
  return `${ms}ms`
}

function billingModeLabel(value?: string): string {
  return {
    token: '按 Token',
    request: '按次',
    image: '图片'
  }[value || ''] || value || '-'
}

function supplierName(id: number): string {
  return suppliers.value.find((supplier) => supplier.id === id)?.name || `#${id}`
}

function reconciliationStatusLabel(status: ReconciliationLine['status']): string {
  return {
    matched: '已匹配',
    supplier_only: '仅供应商',
    local_only: '仅本地',
    currency_mismatch: '币种不一致',
    cost_mismatch: '成本不一致'
  }[status] || status
}

function reconciliationStatusClass(status: ReconciliationLine['status']): string {
  if (status === 'matched') return 'badge-success'
  if (status === 'cost_mismatch' || status === 'currency_mismatch') return 'badge-danger'
  return 'badge-warning'
}

function toDateTimeLocal(value: Date): string {
  const offsetMs = value.getTimezoneOffset() * 60 * 1000
  return new Date(value.getTime() - offsetMs).toISOString().slice(0, 16)
}

function toRFC3339(value: string): string {
  return new Date(value).toISOString()
}

async function loadPage() {
  loading.value = true
  try {
    const [supplierResult] = await Promise.all([
      listSuppliers(),
      loadBillLines()
    ])
    suppliers.value = supplierResult.items
    if (!billForm.supplier_id && suppliers.value[0]) {
      billForm.supplier_id = suppliers.value[0].id
    }
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '加载账单失败')
  } finally {
    loading.value = false
  }
}

async function loadBillLines() {
  const billResult = await listBillLines({
    supplier_id: billFilter.supplier_id || undefined,
    page: billPagination.page,
    page_size: billPagination.page_size
  })
  billLines.value = billResult.items
  billPagination.total = billResult.total || 0
  billPagination.pages = billResult.pages || 0
  billPagination.page = billResult.page || billPagination.page
  billPagination.page_size = billResult.page_size || billPagination.page_size
}

function handleBillFilterChange() {
  billPagination.page = 1
  void loadBillLines()
}

function handleBillPageChange(page: number) {
  billPagination.page = page
  void loadBillLines()
}

function handleBillPageSizeChange(pageSize: number) {
  billPagination.page_size = pageSize
  billPagination.page = 1
  void loadBillLines()
}

function openImportDialog() {
  if (!billForm.supplier_id && suppliers.value[0]) {
    billForm.supplier_id = suppliers.value[0].id
  }
  importDialogOpen.value = true
}

function openUsageDialog() {
  usageDialogOpen.value = true
}

async function importBill() {
  importing.value = true
  try {
    await importBillLines([{
      supplier_id: billForm.supplier_id,
      external_request_id: billForm.external_request_id,
      api_key_name: billForm.api_key_name,
      model: billForm.model,
      endpoint: billForm.endpoint,
      request_type: billForm.request_type,
      billing_mode: billForm.billing_mode,
      reasoning_effort: billForm.reasoning_effort,
      currency: billForm.currency || 'CNY',
      cost_cents: centsFromYuan(billForm.cost_yuan),
      input_tokens: Number(billForm.input_tokens || 0),
      output_tokens: Number(billForm.output_tokens || 0),
      cache_read_tokens: Number(billForm.cache_read_tokens || 0),
      total_tokens: Number(billForm.total_tokens || 0),
      first_token_ms: Number(billForm.first_token_ms || 0),
      duration_ms: Number(billForm.duration_ms || 0),
      user_agent: billForm.user_agent,
      started_at: new Date().toISOString()
    }])
    billForm.external_request_id = ''
    billPagination.page = 1
    importDialogOpen.value = false
    appStore.showSuccess('账单已导入')
    await loadBillLines()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '导入账单失败')
  } finally {
    importing.value = false
  }
}

async function loadLocalUsagePage() {
  const usageResult = await listLocalUsageLines({
    model: usageForm.model || undefined,
    from: toRFC3339(usageForm.from),
    to: toRFC3339(usageForm.to),
    page: localUsagePagination.page,
    page_size: localUsagePagination.page_size
  })
  localUsages.value = usageResult.items
  localUsagePagination.total = usageResult.total || 0
  localUsagePagination.pages = usageResult.pages || 0
  localUsagePagination.page = usageResult.page || localUsagePagination.page
  localUsagePagination.page_size = usageResult.page_size || localUsagePagination.page_size
}

async function fetchAllBillLinesForReconciliation() {
  const first = await listBillLines({
    supplier_id: billFilter.supplier_id || undefined,
    page: 1,
    page_size: 1000
  })
  if (!first.pages || first.pages <= 1) return first.items
  const rest = await Promise.all(Array.from({ length: first.pages - 1 }, (_, index) =>
    listBillLines({
      supplier_id: billFilter.supplier_id || undefined,
      page: index + 2,
      page_size: first.page_size || 1000
    })
  ))
  return [...first.items, ...rest.flatMap((page) => page.items)]
}

async function fetchAllLocalUsageLinesForReconciliation() {
  const first = await listLocalUsageLines({
    model: usageForm.model || undefined,
    from: toRFC3339(usageForm.from),
    to: toRFC3339(usageForm.to),
    page: 1,
    page_size: 1000
  })
  if (!first.pages || first.pages <= 1) return first.items
  const rest = await Promise.all(Array.from({ length: first.pages - 1 }, (_, index) =>
    listLocalUsageLines({
      model: usageForm.model || undefined,
      from: toRFC3339(usageForm.from),
      to: toRFC3339(usageForm.to),
      page: index + 2,
      page_size: first.page_size || 1000
    })
  ))
  return [...first.items, ...rest.flatMap((page) => page.items)]
}

function resetResultPagination() {
  resultPagination.page = 1
  syncResultPagination()
}

function syncResultPagination() {
  resultPagination.total = filteredReconciliationLines.value.length
  resultPagination.pages = resultPagination.total > 0 ? Math.ceil(resultPagination.total / resultPagination.page_size) : 0
  if (resultPagination.page > Math.max(1, resultPagination.pages)) {
    resultPagination.page = Math.max(1, resultPagination.pages)
  }
}

function handleLocalUsagePageChange(page: number) {
  localUsagePagination.page = page
  void loadLocalUsagePage().catch((error) => {
    appStore.showError((error as { message?: string }).message || '加载本地用量失败')
  })
}

function handleLocalUsagePageSizeChange(pageSize: number) {
  localUsagePagination.page_size = pageSize
  localUsagePagination.page = 1
  void loadLocalUsagePage().catch((error) => {
    appStore.showError((error as { message?: string }).message || '加载本地用量失败')
  })
}

function handleResultPageChange(page: number) {
  resultPagination.page = page
}

function handleResultPageSizeChange(pageSize: number) {
  resultPagination.page_size = pageSize
  resultPagination.page = 1
  syncResultPagination()
}

async function runRecon() {
  reconciling.value = true
  try {
    localUsagePagination.page = 1
    resultPagination.page = 1
    const [allBills, allLocalUsages] = await Promise.all([
      fetchAllBillLinesForReconciliation(),
      fetchAllLocalUsageLinesForReconciliation()
    ])
    await loadLocalUsagePage()
    result.value = await runReconciliation({
      supplier_bills: allBills,
      local_usages: allLocalUsages,
      time_tolerance_seconds: Number(reconciliationForm.time_tolerance_seconds || 300),
      cost_mismatch_cents: Number(reconciliationForm.cost_mismatch_cents || 0)
    })
    resultStatusFilter.value = anomalyCount.value > 0 ? 'anomaly' : 'all'
    syncResultPagination()
    activeTab.value = 'result'
    usageDialogOpen.value = false
    appStore.showSuccess('对账完成')
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '对账失败')
  } finally {
    reconciling.value = false
  }
}

onMounted(loadPage)
</script>
