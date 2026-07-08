<template>
  <div v-if="availability" class="mt-2 space-y-2">
    <div v-if="summaryLabel" class="flex flex-wrap items-center justify-between gap-2">
      <p class="text-xs text-gray-500 dark:text-dark-400">
        {{ summaryLabel }}
      </p>
      <button
        v-if="canOpenDetail"
        type="button"
        class="text-xs text-primary-600 hover:text-primary-700 dark:text-primary-400"
        @click="openDetail()"
      >
        查看详情
      </button>
    </div>

    <div v-if="impactedKeys.length" class="border-t border-gray-100 pt-2 dark:border-dark-700">
      <div class="flex items-center justify-between gap-2">
        <p class="text-xs font-medium text-gray-500 dark:text-dark-400">受影响 Key</p>
        <button v-if="impactedKeys.length > collapsedKeyLimit" type="button" class="text-xs text-primary-600 hover:text-primary-700 dark:text-primary-400" @click="showAllKeys = !showAllKeys">
          {{ showAllKeys ? '收起' : `展开 ${impactedKeys.length}` }}
        </button>
      </div>
      <div class="mt-1 flex flex-wrap gap-1.5">
        <span
          v-for="key in visibleKeys"
          :key="key.id"
          class="badge max-w-full truncate"
          :class="routingImpactedAPIKeyBadgeClass(key)"
          :title="routingImpactedAPIKeyDetailLabel(key)"
        >
          {{ routingImpactedAPIKeyBadgeLabel(key) }}
        </span>
        <span v-if="availability.impacted_api_keys_truncated" class="badge badge-warning">还有更多</span>
      </div>
    </div>

    <div v-if="recentFailures.length" class="border-t border-gray-100 pt-2 dark:border-dark-700">
      <div class="flex items-center justify-between gap-2">
        <p class="text-xs font-medium text-gray-500 dark:text-dark-400">最近失败请求</p>
        <button v-if="recentFailures.length > collapsedFailureLimit" type="button" class="text-xs text-primary-600 hover:text-primary-700 dark:text-primary-400" @click="showAllFailures = !showAllFailures">
          {{ showAllFailures ? '收起' : `展开 ${recentFailures.length}` }}
        </button>
      </div>
      <div class="mt-1 space-y-1">
        <div
          v-for="request in visibleFailures"
          :key="request.id"
          class="flex min-w-0 items-center gap-2 text-xs text-gray-600 dark:text-dark-300"
          :title="routingFailureRequestDetailLabel(request)"
        >
          <span class="badge shrink-0" :class="routingFailureRequestBadgeClass(request)">
            {{ routingFailureRequestStatusLabel(request) }}
          </span>
          <span class="truncate">{{ routingFailureRequestLabel(request) }}</span>
          <button type="button" class="shrink-0 text-primary-600 hover:text-primary-700 dark:text-primary-400" @click.stop="openSensitiveDetail(request)">明细</button>
        </div>
        <span v-if="availability.recent_failures_truncated" class="badge badge-warning">还有更多失败请求</span>
      </div>
    </div>

    <BaseDialog :show="detailOpen" title="补池用户影响详情" width="extra-wide" @close="detailOpen = false">
      <div class="space-y-4">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div class="flex rounded-md border border-gray-200 bg-white p-1 dark:border-dark-700 dark:bg-dark-900">
            <button
              type="button"
              class="rounded px-3 py-1.5 text-sm"
              :class="activeTab === 'keys' ? 'bg-primary-600 text-white' : 'text-gray-600 hover:bg-gray-100 dark:text-dark-300 dark:hover:bg-dark-800'"
              @click="selectTab('keys')"
            >
              受影响 Key
            </button>
            <button
              type="button"
              class="rounded px-3 py-1.5 text-sm"
              :class="activeTab === 'failures' ? 'bg-primary-600 text-white' : 'text-gray-600 hover:bg-gray-100 dark:text-dark-300 dark:hover:bg-dark-800'"
              @click="selectTab('failures')"
            >
              失败请求
            </button>
          </div>
          <p class="text-xs text-gray-500 dark:text-dark-400">
            {{ summaryLabel }}
          </p>
        </div>

        <div v-if="activeTab === 'keys'" class="space-y-3">
          <div class="overflow-hidden rounded-md border border-gray-200 dark:border-dark-700">
            <table class="min-w-full divide-y divide-gray-200 dark:divide-dark-700">
              <thead class="bg-gray-50 dark:bg-dark-800">
                <tr>
                  <th class="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-dark-400">Key</th>
                  <th class="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-dark-400">用户</th>
                  <th class="px-4 py-2 text-right text-xs font-medium text-gray-500 dark:text-dark-400">24h 成功</th>
                  <th class="px-4 py-2 text-right text-xs font-medium text-gray-500 dark:text-dark-400">24h 错误</th>
                  <th class="px-4 py-2 text-right text-xs font-medium text-gray-500 dark:text-dark-400">上游 429</th>
                  <th class="px-4 py-2 text-right text-xs font-medium text-gray-500 dark:text-dark-400">Token</th>
                  <th class="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-dark-400">最近请求</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-100 bg-white dark:divide-dark-800 dark:bg-dark-900">
                <tr v-if="keysLoading">
                  <td colspan="7" class="px-4 py-6 text-center text-sm text-gray-500 dark:text-dark-400">加载中...</td>
                </tr>
                <tr v-else-if="!detailKeys.length">
                  <td colspan="7" class="px-4 py-6 text-center text-sm text-gray-500 dark:text-dark-400">暂无受影响 Key</td>
                </tr>
                <tr v-for="key in detailKeys" v-else :key="key.id">
                  <td class="max-w-xs px-4 py-3 text-sm text-gray-900 dark:text-white">
                    <div class="truncate font-medium">{{ key.name || `Key #${key.id}` }}</div>
                    <div class="truncate text-xs text-gray-500 dark:text-dark-400">{{ key.key_preview || '-' }}</div>
                  </td>
                  <td class="px-4 py-3 text-sm text-gray-600 dark:text-dark-300">#{{ key.user_id }}</td>
                  <td class="px-4 py-3 text-right text-sm text-gray-600 dark:text-dark-300">{{ formatCount(key.recent_success_request_count) }}</td>
                  <td class="px-4 py-3 text-right text-sm text-gray-600 dark:text-dark-300">{{ formatCount(key.recent_error_request_count) }}</td>
                  <td class="px-4 py-3 text-right text-sm text-gray-600 dark:text-dark-300">{{ formatCount(key.recent_upstream_429_count) }}</td>
                  <td class="px-4 py-3 text-right text-sm text-gray-600 dark:text-dark-300">{{ formatCount(key.recent_token_count) }}</td>
                  <td class="px-4 py-3 text-sm text-gray-600 dark:text-dark-300">{{ formatTime(key.recent_last_request_at || key.last_used_at) }}</td>
                </tr>
              </tbody>
            </table>
          </div>
          <Pagination
            v-if="keyPagination.total > 0"
            :page="keyPagination.page"
            :total="keyPagination.total"
            :page-size="keyPagination.page_size"
            :show-jump="true"
            @update:page="handleKeyPageChange"
            @update:pageSize="handleKeyPageSizeChange"
          />
        </div>

        <div v-else class="space-y-3">
          <div class="overflow-hidden rounded-md border border-gray-200 dark:border-dark-700">
            <table class="min-w-full divide-y divide-gray-200 dark:divide-dark-700">
              <thead class="bg-gray-50 dark:bg-dark-800">
                <tr>
                  <th class="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-dark-400">时间</th>
                  <th class="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-dark-400">状态</th>
                  <th class="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-dark-400">Key</th>
                  <th class="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-dark-400">账号</th>
                  <th class="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-dark-400">模型</th>
                  <th class="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-dark-400">错误</th>
                  <th class="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-dark-400">明细</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-100 bg-white dark:divide-dark-800 dark:bg-dark-900">
                <tr v-if="failuresLoading">
                  <td colspan="7" class="px-4 py-6 text-center text-sm text-gray-500 dark:text-dark-400">加载中...</td>
                </tr>
                <tr v-else-if="!detailFailures.length">
                  <td colspan="7" class="px-4 py-6 text-center text-sm text-gray-500 dark:text-dark-400">暂无失败请求</td>
                </tr>
                <tr v-for="request in detailFailures" v-else :key="request.id">
                  <td class="whitespace-nowrap px-4 py-3 text-sm text-gray-600 dark:text-dark-300">{{ formatTime(request.created_at) }}</td>
                  <td class="px-4 py-3 text-sm">
                    <span class="badge" :class="routingFailureRequestBadgeClass(request)">
                      {{ routingFailureRequestStatusLabel(request) }}
                    </span>
                  </td>
                  <td class="max-w-xs px-4 py-3 text-sm text-gray-900 dark:text-white">
                    <div class="truncate font-medium">{{ request.api_key_name || (request.api_key_id ? `Key #${request.api_key_id}` : '未知 Key') }}</div>
                    <div class="truncate text-xs text-gray-500 dark:text-dark-400">{{ request.api_key_preview || '-' }}</div>
                  </td>
                  <td class="px-4 py-3 text-sm text-gray-600 dark:text-dark-300">{{ request.account_id ? `#${request.account_id}` : '-' }}</td>
                  <td class="max-w-[12rem] truncate px-4 py-3 text-sm text-gray-600 dark:text-dark-300">{{ request.model || '-' }}</td>
                  <td class="max-w-md px-4 py-3 text-sm text-gray-600 dark:text-dark-300">
                    <div class="truncate" :title="routingFailureRequestDetailLabel(request)">
                      {{ request.error_message || request.error_type || '-' }}
                    </div>
                    <div v-if="request.request_id" class="truncate text-xs text-gray-500 dark:text-dark-400">{{ request.request_id }}</div>
                  </td>
                  <td class="px-4 py-3 text-sm">
                    <button type="button" class="text-primary-600 hover:text-primary-700 dark:text-primary-400" @click="openSensitiveDetail(request)">查看</button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <Pagination
            v-if="failurePagination.total > 0"
            :page="failurePagination.page"
            :total="failurePagination.total"
            :page-size="failurePagination.page_size"
            :show-jump="true"
            @update:page="handleFailurePageChange"
            @update:pageSize="handleFailurePageSizeChange"
          />
        </div>
      </div>
    </BaseDialog>

    <BaseDialog :show="sensitiveDetailOpen" title="失败请求明细" width="wide" @close="closeSensitiveDetail">
      <div class="space-y-4">
        <div v-if="sensitiveTarget" class="grid gap-3 text-sm text-gray-700 dark:text-dark-200 sm:grid-cols-2">
          <div>请求：{{ sensitiveTarget.request_id || `#${sensitiveTarget.id}` }}</div>
          <div>状态：{{ routingFailureRequestStatusLabel(sensitiveTarget) }}</div>
          <div>Key：{{ sensitiveTarget.api_key_name || sensitiveTarget.api_key_preview || '-' }}</div>
          <div>模型：{{ sensitiveTarget.model || '-' }}</div>
        </div>

        <label class="block text-sm font-medium text-gray-700 dark:text-dark-200">
          查询原因
          <textarea
            v-model.trim="sensitiveReason"
            rows="3"
            class="input mt-1"
            placeholder="例如：排查补池前 429 来源"
          />
        </label>

        <div class="flex items-center justify-end gap-2">
          <button type="button" class="btn btn-secondary" @click="closeSensitiveDetail">关闭</button>
          <button type="button" class="btn btn-primary" :disabled="sensitiveLoading || !sensitiveReason" @click="loadSensitiveDetail">
            {{ sensitiveLoading ? '查询中...' : '查询明细' }}
          </button>
        </div>

        <div v-if="sensitiveDetail" class="space-y-3">
          <div v-if="!sensitiveDetail.available" class="rounded-md border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-700 dark:border-amber-900/50 dark:bg-amber-950/30 dark:text-amber-300">
            {{ sensitiveUnavailableLabel(sensitiveDetail.unavailable_reason) }}
          </div>
          <div class="overflow-hidden rounded-md border border-gray-200 dark:border-dark-700">
            <div
              v-for="field in sensitiveDetail.fields"
              :key="field.name"
              class="border-b border-gray-100 p-3 last:border-b-0 dark:border-dark-800"
            >
              <div class="mb-2 flex flex-wrap items-center gap-2">
                <span class="text-sm font-medium text-gray-900 dark:text-white">{{ sensitiveFieldLabel(field.name) }}</span>
                <span v-if="field.redacted" class="badge badge-info">已脱敏</span>
                <span v-if="field.truncated" class="badge badge-warning">已截断</span>
                <span v-if="!field.available" class="badge badge-gray">{{ sensitiveUnavailableLabel(field.unavailable_reason) }}</span>
              </div>
              <pre v-if="field.available" class="max-h-72 overflow-auto whitespace-pre-wrap rounded bg-gray-50 p-3 text-xs text-gray-700 dark:bg-dark-950 dark:text-dark-200">{{ field.value }}</pre>
            </div>
          </div>
        </div>
      </div>
    </BaseDialog>
  </div>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Pagination from '@/components/common/Pagination.vue'
import {
  getRoutingFailureSensitiveDetail,
  listRoutingImpactAPIKeys,
  listRoutingImpactFailureRequests,
  type RoutingFailureRequest,
  type RoutingGroupAvailability,
  type RoutingImpactedAPIKey,
  type RoutingSensitiveFailureDetail
} from '@/api/admin/adminPlus'
import { useAppStore } from '@/stores/app'
import {
  routingFailureRequestBadgeClass,
  routingFailureRequestDetailLabel,
  routingFailureRequestLabel,
  routingFailureRequestStatusLabel,
  routingImpactSummaryLabel,
  routingImpactedAPIKeyBadgeClass,
  routingImpactedAPIKeyBadgeLabel,
  routingImpactedAPIKeyDetailLabel
} from '@/views/admin/routingRefillPresentation'

const collapsedKeyLimit = 5
const collapsedFailureLimit = 5

const props = defineProps<{
  availability?: RoutingGroupAvailability | null
}>()

const appStore = useAppStore()
const showAllKeys = ref(false)
const showAllFailures = ref(false)
const detailOpen = ref(false)
const activeTab = ref<'keys' | 'failures'>('keys')
const keysLoading = ref(false)
const failuresLoading = ref(false)
const detailKeys = ref<RoutingImpactedAPIKey[]>([])
const detailFailures = ref<RoutingFailureRequest[]>([])
const keyPagination = reactive({ page: 1, page_size: 20, total: 0, pages: 0 })
const failurePagination = reactive({ page: 1, page_size: 20, total: 0, pages: 0 })
const sensitiveDetailOpen = ref(false)
const sensitiveLoading = ref(false)
const sensitiveTarget = ref<RoutingFailureRequest | null>(null)
const sensitiveReason = ref('')
const sensitiveDetail = ref<RoutingSensitiveFailureDetail | null>(null)
const sensitiveFields = [
  'error_message',
  'error_body',
  'upstream_error_message',
  'upstream_error_detail',
  'provider_error_code',
  'provider_error_type',
  'network_error_type',
  'error_source',
  'inbound_endpoint',
  'upstream_endpoint',
  'requested_model',
  'upstream_model',
  'retry_after_seconds',
  'request_body',
  'request_headers'
]

const summaryLabel = computed(() => routingImpactSummaryLabel(props.availability || undefined))
const impactedKeys = computed(() => props.availability?.impacted_api_keys || [])
const recentFailures = computed(() => props.availability?.recent_failure_requests || [])
const localGroupID = computed(() => Number(props.availability?.group_id || 0))
const canOpenDetail = computed(() => localGroupID.value > 0 && (
  (props.availability?.active_api_key_count || 0) > 0 ||
  impactedKeys.value.length > 0 ||
  recentFailures.value.length > 0
))
const visibleKeys = computed(() => showAllKeys.value ? impactedKeys.value : impactedKeys.value.slice(0, collapsedKeyLimit))
const visibleFailures = computed(() => showAllFailures.value ? recentFailures.value : recentFailures.value.slice(0, collapsedFailureLimit))

function openDetail(tab?: 'keys' | 'failures') {
  if (!canOpenDetail.value) return
  activeTab.value = tab || (recentFailures.value.length > 0 ? 'failures' : 'keys')
  detailOpen.value = true
  void loadActiveTab()
}

function selectTab(tab: 'keys' | 'failures') {
  activeTab.value = tab
  void loadActiveTab()
}

async function loadActiveTab() {
  if (activeTab.value === 'keys') {
    await loadDetailKeys()
    return
  }
  await loadDetailFailures()
}

async function loadDetailKeys() {
  if (localGroupID.value <= 0) return
  keysLoading.value = true
  try {
    const result = await listRoutingImpactAPIKeys({
      local_group_id: localGroupID.value,
      page: keyPagination.page,
      page_size: keyPagination.page_size
    })
    detailKeys.value = result.items || []
    keyPagination.total = result.total || 0
    keyPagination.pages = result.pages || 0
    keyPagination.page = result.page || keyPagination.page
    keyPagination.page_size = result.page_size || keyPagination.page_size
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '加载受影响 Key 失败')
  } finally {
    keysLoading.value = false
  }
}

async function loadDetailFailures() {
  if (localGroupID.value <= 0) return
  failuresLoading.value = true
  try {
    const result = await listRoutingImpactFailureRequests({
      local_group_id: localGroupID.value,
      page: failurePagination.page,
      page_size: failurePagination.page_size
    })
    detailFailures.value = result.items || []
    failurePagination.total = result.total || 0
    failurePagination.pages = result.pages || 0
    failurePagination.page = result.page || failurePagination.page
    failurePagination.page_size = result.page_size || failurePagination.page_size
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '加载失败请求失败')
  } finally {
    failuresLoading.value = false
  }
}

function handleKeyPageChange(page: number) {
  keyPagination.page = page
  void loadDetailKeys()
}

function handleKeyPageSizeChange(pageSize: number) {
  keyPagination.page_size = pageSize
  keyPagination.page = 1
  void loadDetailKeys()
}

function handleFailurePageChange(page: number) {
  failurePagination.page = page
  void loadDetailFailures()
}

function handleFailurePageSizeChange(pageSize: number) {
  failurePagination.page_size = pageSize
  failurePagination.page = 1
  void loadDetailFailures()
}

function openSensitiveDetail(request: RoutingFailureRequest) {
  if (localGroupID.value <= 0) return
  sensitiveTarget.value = request
  sensitiveReason.value = ''
  sensitiveDetail.value = null
  sensitiveDetailOpen.value = true
}

function closeSensitiveDetail() {
  sensitiveDetailOpen.value = false
  sensitiveLoading.value = false
  sensitiveTarget.value = null
  sensitiveReason.value = ''
  sensitiveDetail.value = null
}

async function loadSensitiveDetail() {
  if (!sensitiveTarget.value || localGroupID.value <= 0) return
  if (!sensitiveReason.value.trim()) {
    appStore.showError('请填写查询原因')
    return
  }
  sensitiveLoading.value = true
  try {
    sensitiveDetail.value = await getRoutingFailureSensitiveDetail(sensitiveTarget.value.id, {
      local_group_id: localGroupID.value,
      reason: sensitiveReason.value,
      fields: sensitiveFields
    })
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '查询失败请求明细失败')
  } finally {
    sensitiveLoading.value = false
  }
}

function formatCount(value?: number | null): string {
  const number = Number(value || 0)
  if (!Number.isFinite(number)) return '0'
  return Math.trunc(number).toLocaleString('zh-CN')
}

function formatTime(value?: string | null): string {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString('zh-CN', { hour12: false })
}

function sensitiveFieldLabel(name: string): string {
  const labels: Record<string, string> = {
    error_message: '错误消息',
    error_body: '错误响应体',
    upstream_error_message: '上游错误消息',
    upstream_error_detail: '上游错误详情',
    provider_error_code: '供应商错误码',
    provider_error_type: '供应商错误类型',
    network_error_type: '网络错误类型',
    error_source: '错误来源',
    inbound_endpoint: '入口端点',
    upstream_endpoint: '上游端点',
    requested_model: '请求模型',
    upstream_model: '上游模型',
    retry_after_seconds: 'Retry-After',
    request_body: '请求体',
    request_headers: '请求头'
  }
  return labels[name] || name
}

function sensitiveUnavailableLabel(reason?: string): string {
  if (reason === 'not_recorded') return '未记录'
  return reason || '不可用'
}

watch(
  () => `${props.availability?.group_id || 0}:${props.availability?.recent_last_error_at || ''}:${props.availability?.recent_last_request_at || ''}`,
  () => {
    showAllKeys.value = false
    showAllFailures.value = false
    detailOpen.value = false
    closeSensitiveDetail()
    activeTab.value = 'keys'
    detailKeys.value = []
    detailFailures.value = []
    keyPagination.page = 1
    keyPagination.total = 0
    keyPagination.pages = 0
    failurePagination.page = 1
    failurePagination.total = 0
    failurePagination.pages = 0
  }
)
</script>
