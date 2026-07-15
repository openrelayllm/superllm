<template>
  <div class="space-y-4">
    <div class="flex flex-wrap items-end justify-between gap-3">
      <div class="grid min-w-0 flex-1 gap-3 sm:grid-cols-[minmax(220px,1fr)_160px]">
        <label class="block">
          <span class="input-label">搜索分组</span>
          <div class="relative">
            <Icon name="search" size="sm" class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
            <input v-model.trim="groupFilters.q" class="input pl-9" placeholder="分组名称、平台、ID" />
          </div>
          <span class="mt-1 block text-xs text-gray-500 dark:text-dark-400">仅筛选下方列表，不改变创建范围</span>
        </label>
        <label class="block">
          <span class="input-label">状态</span>
          <select v-model="groupFilters.status" class="input">
            <option value="">全部</option>
            <option value="active">有效</option>
            <option value="missing">已缺失</option>
            <option value="disabled">停用</option>
          </select>
        </label>
      </div>

      <div class="flex flex-wrap items-center gap-2">
        <button type="button" class="btn btn-secondary" :disabled="groupsLoading || !groupsSupplier" title="刷新当前列表" @click="loadCurrentGroups">
          <Icon name="refresh" size="sm" :class="{ 'animate-spin': groupsLoading }" />
          刷新
        </button>
        <button type="button" class="btn btn-secondary" :disabled="groupsSyncing || !canSubmitGroupSync" @click="syncCurrentGroups">
          <Icon name="sync" size="sm" :class="{ 'animate-spin': groupsSyncing }" />
          同步分组
        </button>
        <button type="button" class="btn btn-secondary" :disabled="channelChecksSyncing || !groupsSupplier || activeProvisionJobRunning" @click="syncCurrentChannelChecks">
          <Icon name="beaker" size="sm" :class="{ 'animate-spin': channelChecksSyncing }" />
          检测渠道
        </button>
        <details class="relative">
          <summary class="btn btn-secondary cursor-pointer list-none" title="更多操作">
            <Icon name="more" size="sm" />
            <span class="sr-only">更多操作</span>
          </summary>
          <div class="absolute right-0 z-30 mt-2 w-72 rounded-md border border-gray-200 bg-white p-3 shadow-lg dark:border-dark-700 dark:bg-dark-800">
            <label class="flex items-start gap-2 text-sm text-gray-700 dark:text-dark-200">
              <input v-model="keyNamingForm.sync_provider_name" type="checkbox" class="mt-0.5 h-4 w-4 rounded border-gray-300" />
              <span>同时修改第三方 Key 名称</span>
            </label>
            <button type="button" class="btn btn-secondary mt-3 w-full" :disabled="keyNamesStandardizing || !groupsSupplier" @click="standardizeCurrentKeyNames">
              <Icon name="key" size="sm" :class="{ 'animate-spin': keyNamesStandardizing }" />
              统一现有 Key 名称
            </button>
          </div>
        </details>
        <button type="button" class="btn btn-primary" :disabled="ensureKeysPlanning || !canSubmitEnsureKeys" @click="previewEnsureCurrentKeys">
          <Icon name="key" size="sm" :class="{ 'animate-spin': ensureKeysPlanning }" />
          {{ ensureKeysPlanning ? '检查中...' : '创建缺失 Key' }}
        </button>
      </div>
    </div>

    <section v-if="plan" aria-labelledby="create-key-confirm-title" class="border-y border-gray-200 bg-gray-50 px-4 py-4 dark:border-dark-700 dark:bg-dark-900/40">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h2 id="create-key-confirm-title" class="text-base font-semibold text-gray-900 dark:text-gray-100">创建缺失 Key</h2>
          <p class="mt-1 text-sm text-gray-600 dark:text-dark-300">将检查全部 {{ plan.total }} 个有效分组，搜索条件不会改变本次范围。</p>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <button type="button" class="btn btn-secondary btn-sm" :disabled="keysEnsuring" @click="dismissPlan">
            <Icon name="x" size="xs" />
            取消
          </button>
          <button v-if="actionableCount > 0" type="button" class="btn btn-primary" :disabled="keysEnsuring || !ensureKeysPlanCanSubmit()" @click="ensureCurrentKeys">
            <Icon name="key" size="sm" :class="{ 'animate-spin': keysEnsuring }" />
            {{ keysEnsuring ? '正在创建...' : primarySubmitLabel }}
          </button>
        </div>
      </div>

      <div class="mt-4 grid gap-3 sm:grid-cols-4">
        <div class="border-l-2 border-emerald-500 pl-3">
          <div class="text-xs text-gray-500 dark:text-dark-400">待创建</div>
          <div class="mt-1 text-xl font-semibold text-gray-900 dark:text-gray-100">{{ plan.to_create }}</div>
        </div>
        <div class="border-l-2 border-blue-500 pl-3">
          <div class="text-xs text-gray-500 dark:text-dark-400">已有 Key，可直接绑定</div>
          <div class="mt-1 text-xl font-semibold text-gray-900 dark:text-gray-100">{{ plan.to_reuse || 0 }}</div>
        </div>
        <div class="border-l-2 border-gray-300 pl-3 dark:border-dark-600">
          <div class="text-xs text-gray-500 dark:text-dark-400">检查现有本地接入</div>
          <div class="mt-1 text-xl font-semibold text-gray-900 dark:text-gray-100">{{ plan.already_satisfied }}</div>
        </div>
        <div class="border-l-2 border-amber-500 pl-3">
          <div class="text-xs text-gray-500 dark:text-dark-400">需人工处理</div>
          <div class="mt-1 text-xl font-semibold text-gray-900 dark:text-gray-100">{{ plan.blocked }}</div>
        </div>
      </div>

      <div v-if="planWarnings.length" class="mt-4 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-900 dark:border-amber-900/60 dark:bg-amber-950/20 dark:text-amber-100">
        <p v-for="warning in planWarnings" :key="warning">{{ planWarningLabel(warning) }}</p>
      </div>

      <div v-if="legacyUnknownCapacityCount > 0" class="mt-4 rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-800 dark:border-red-900/60 dark:bg-red-950/20 dark:text-red-100">
        当前后端仍使用旧配额规则，把 {{ legacyUnknownCapacityCount }} 个缺少 Key 的分组判定为不可创建。请切换到包含“未知配额直接尝试”逻辑的新后端，然后重新检查。
      </div>

      <div class="mt-4 overflow-x-auto">
        <table class="min-w-full text-left text-sm">
          <thead class="text-xs text-gray-500 dark:text-dark-400">
            <tr>
              <th class="px-2 py-2 font-medium">分组</th>
              <th class="px-2 py-2 font-medium">倍率</th>
              <th class="px-2 py-2 font-medium">处理方式</th>
              <th class="px-2 py-2 font-medium">说明</th>
              <th class="px-2 py-2 text-right font-medium">操作</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-200 dark:divide-dark-700">
            <tr v-for="item in plan.items.slice(0, 12)" :key="item.supplier_group_id">
              <td class="px-2 py-2">
                <div class="font-medium text-gray-900 dark:text-gray-100">{{ item.group_name }}</div>
                <div class="font-mono text-xs text-gray-500 dark:text-dark-400">#{{ item.external_group_id }}</div>
              </td>
              <td class="px-2 py-2">{{ formatMultiplier(item.effective_rate_multiplier || item.rate_multiplier) }}</td>
              <td class="px-2 py-2"><span class="badge" :class="planActionClass(item)">{{ planActionLabel(item) }}</span></td>
              <td class="px-2 py-2 text-gray-600 dark:text-dark-300">{{ planItemDescription(item) }}</td>
              <td class="px-2 py-2 text-right">
                <button
                  v-if="item.blocked_reason === 'provider_key_exists_unbound'"
                  type="button"
                  class="btn btn-secondary btn-sm"
                  :disabled="providerKeyImportingGroupID === item.supplier_group_id"
                  @click="importCurrentProviderKeyProjection(item)"
                >
                  导入第三方已有 Key
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="mt-4 flex flex-wrap items-center justify-between gap-3">
        <p class="text-sm text-gray-600 dark:text-dark-300">成功创建或复用后，将自动创建并绑定本地 Sub2API 账号。</p>
        <span v-if="actionableCount === 0" class="text-sm font-medium text-amber-700 dark:text-amber-300">当前没有可自动创建的 Key</span>
      </div>
    </section>

    <section v-if="activeProvisionJob" aria-live="polite" class="border-y border-gray-200 px-4 py-4 dark:border-dark-700">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <div class="flex items-center gap-2">
          <span class="badge" :class="provisionJobStatusClass(activeProvisionJob.status)">{{ provisionJobStatusLabel(activeProvisionJob.status) }}</span>
          <span class="font-medium text-gray-900 dark:text-gray-100">创建 Key 任务 #{{ activeProvisionJob.id }}</span>
        </div>
        <span class="text-xs text-gray-500 dark:text-dark-400">{{ formatDateTime(activeProvisionJob.updated_at) }}</span>
      </div>
      <div class="mt-3 h-2 overflow-hidden rounded bg-gray-100 dark:bg-dark-700">
        <div class="h-full bg-primary-500 transition-all" :style="{ width: `${jobProgress}%` }"></div>
      </div>
      <div class="mt-2 flex flex-wrap justify-between gap-2 text-xs text-gray-500 dark:text-dark-400">
        <span>{{ provisionJobCaption(activeProvisionJob) }}</span>
        <span>{{ activeProvisionJob.succeeded_steps }}/{{ activeProvisionJob.total_steps }} 完成</span>
      </div>
    </section>

    <div v-for="message in errorMessages" :key="message" class="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-200">
      {{ message }}
    </div>

    <DataTable :columns="groupColumns" :data="supplierGroups" :loading="groupsLoading" row-key="id" default-sort-key="last_seen_at" default-sort-order="desc" :estimate-row-height="88">
      <template #cell-name="{ row }">
        <div class="w-[190px] space-y-1 whitespace-normal">
          <GroupBadge class="max-w-full" :name="row.name" :platform="groupPlatform(row.provider_family, row.name, row.description)" :rate-multiplier="groupCostMultiplier(row)" />
          <div class="font-mono text-xs text-gray-500 dark:text-dark-400">#{{ row.external_group_id }}</div>
        </div>
      </template>
      <template #cell-provider_family="{ row }"><span class="badge badge-gray">{{ row.provider_family || 'mixed' }}</span></template>
      <template #cell-rate="{ row }">
        <div class="w-[86px] space-y-0.5 text-right">
          <div :class="rateMultiplierTextClass(groupCostMultiplier(row), channelProtocolFromProviderFamily(row.provider_family, row.name, row.description))">{{ formatMultiplier(groupCostMultiplier(row)) }}</div>
          <div class="text-xs text-gray-500 dark:text-dark-400">使用 {{ formatMultiplier(row.effective_rate_multiplier) }}</div>
        </div>
      </template>
      <template #cell-limits="{ row }">
        <div class="w-[82px] text-xs text-gray-600 dark:text-dark-300"><div>RPM：{{ row.rpm_limit ?? '-' }}</div><div>日：{{ formatUSDLimit(row.daily_limit_usd) }}</div><div>月：{{ formatUSDLimit(row.monthly_limit_usd) }}</div></div>
      </template>
      <template #cell-key_capacity="{ row }">
        <div class="w-[118px] space-y-1 text-xs text-gray-600 dark:text-dark-300">
          <span class="badge" :class="groupKeyCapacityClass(row.key_capacity_status)">{{ groupKeyCapacityStatusLabel(row.key_capacity_status) }}</span>
          <div>{{ groupKeyPolicyLabel(row.key_limit_policy) }} · 已用 {{ row.active_key_count || 0 }}{{ row.key_limit_policy === 'limited' ? `/${row.key_limit_value || 0}` : '' }}</div>
        </div>
      </template>
      <template #cell-account="{ row }">
        <div class="w-[250px] whitespace-normal">
          <template v-if="groupKey(row)">
            <div class="flex min-w-0 items-center gap-2"><span class="truncate font-medium text-gray-900 dark:text-gray-100">{{ groupKey(row)?.name || '-' }}</span><span class="badge shrink-0" :class="supplierKeyStatusClass(groupKey(row)?.status)">{{ supplierKeyStatusLabel(groupKey(row)?.status) }}</span></div>
            <div class="mt-1 text-xs text-gray-500 dark:text-dark-400"><span v-if="groupKey(row)?.key_last4" class="font-mono">****{{ groupKey(row)?.key_last4 }}</span><span v-if="groupKey(row)?.local_sub2api_account_id" class="ml-2">本地 #{{ groupKey(row)?.local_sub2api_account_id }}</span></div>
          </template>
          <span v-else class="badge badge-gray">缺少 Key</span>
        </div>
      </template>
      <template #cell-channel_check="{ row }">
        <div class="w-[190px] whitespace-normal">
          <template v-if="groupChannelCheck(row.id)"><span class="badge" :class="channelProbeStatusClass(groupChannelCheck(row.id)?.probe_status)">{{ channelProbeStatusLabel(groupChannelCheck(row.id)?.probe_status) }}</span><div class="mt-1 text-xs text-gray-500 dark:text-dark-400">首 {{ formatLatency(groupChannelCheck(row.id)?.first_token_ms) }} · 总 {{ formatLatency(groupChannelCheck(row.id)?.duration_ms) }}</div></template>
          <span v-else class="badge badge-gray">未检测</span>
        </div>
      </template>
      <template #cell-status="{ row }"><span class="badge" :class="groupStatusClass(row.status)">{{ groupStatusLabel(row.status) }}</span></template>
      <template #cell-last_seen_at="{ row }"><div class="w-[118px] text-xs text-gray-500 dark:text-dark-400">{{ formatDateTime(row.last_seen_at) }}</div></template>
      <template #cell-group_actions="{ row }">
        <div class="ml-auto flex w-[106px] flex-col gap-1">
          <button v-if="groupAction(row).kind === 'provision'" type="button" class="btn btn-secondary btn-sm" :disabled="groupAction(row).disabled" @click="openProvisionDialog(row)"><Icon name="key" size="xs" />创建 Key</button>
          <button v-if="groupAction(row).kind === 'repair_sub2api_landing'" type="button" class="btn btn-secondary btn-sm" @click="openRepairDialog(groupKey(row)!)"><Icon name="link" size="xs" />完成绑定</button>
          <button type="button" class="btn btn-secondary btn-sm" @click="openGroupKeyCapacitySettings(row)"><Icon name="cog" size="xs" />本地上限</button>
          <button type="button" class="btn btn-secondary btn-sm" :disabled="!groupHasLocalBinding(row)" @click="openGroupChannelProbeDialog(row)"><Icon name="beaker" size="xs" />检测</button>
        </div>
      </template>
      <template #empty>
        <EmptyState :title="hasListFilter ? '没有匹配的分组' : '还没有供应商分组'" :description="hasListFilter ? '调整搜索条件或状态筛选。' : '先同步第三方供应商分组。'" :action-text="hasListFilter ? '清除筛选' : '同步分组'" @action="handleEmptyAction" />
      </template>
    </DataTable>

    <Pagination v-if="groupPagination.total > 0" :page="groupPagination.page" :total="groupPagination.total" :page-size="groupPagination.page_size" @update:page="handleGroupPageChange" @update:pageSize="handleGroupPageSizeChange" />
  </div>

  <BaseDialog :show="keyCapacityDialogOpen" :title="keyCapacityGroup ? `本地 Key 上限 - ${keyCapacityGroup.name}` : '本地 Key 上限'" width="normal" :z-index="60" @close="closeGroupKeyCapacityDialog">
    <div class="space-y-4">
      <p class="text-sm text-gray-600 dark:text-dark-300">这是 SuperLLM 的本地保护规则，不代表第三方平台的真实上限。</p>
      <label class="block"><span class="input-label">处理规则</span><select v-model="keyCapacityForm.key_limit_policy" class="input"><option value="inherit">跟随供应商设置</option><option value="unknown">需要人工确认</option><option value="unlimited">不设置本地上限</option><option value="limited">设置本地上限</option><option value="unsupported">不允许自动创建</option></select></label>
      <label class="block"><span class="input-label">最多创建数量</span><input v-model.number="keyCapacityForm.key_limit_value" type="number" min="1" step="1" class="input" :disabled="keyCapacityForm.key_limit_policy !== 'limited'" /></label>
    </div>
    <template #footer><button type="button" class="btn btn-secondary" :disabled="keyCapacitySubmitting" @click="closeGroupKeyCapacityDialog">取消</button><button type="button" class="btn btn-primary" :disabled="keyCapacitySubmitting || !keyCapacityFormValid" @click="submitGroupKeyCapacity">保存</button></template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { updateSupplierGroupKeyCapacity } from '@/api/admin/adminPlus'
import type { EnsureSupplierKeysPlanItem, SupplierGroup, SupplierGroupKeyLimitPolicy } from '@/api/admin/adminPlus'
import BaseDialog from '@/components/common/BaseDialog.vue'
import DataTable from '@/components/common/DataTable.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import GroupBadge from '@/components/common/GroupBadge.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{ vm: any }>()
const {
  appStore, groupsSupplier, supplierGroups, activeProvisionJob, ensureKeysPlan, groupsLoading, groupsSyncing, keysEnsuring,
  ensureKeysPlanning, keyNamesStandardizing, providerKeyImportingGroupID, channelChecksSyncing, groupsError, provisionJobError,
  ensureKeysPlanError, channelCheckError, groupPagination, groupColumns, groupFilters, keyNamingForm, activeProvisionJobRunning,
  canSubmitGroupSync, canSubmitEnsureKeys, formatDateTime, formatMultiplier, rateMultiplierTextClass, formatLatency, formatUSDLimit,
  groupPlatform, channelProtocolFromProviderFamily, groupCostMultiplier, groupChannelCheck, groupHasLocalBinding, groupStatusLabel,
  groupStatusClass, supplierKeyStatusLabel, supplierKeyStatusClass, channelProbeStatusLabel, channelProbeStatusClass,
  provisionJobStatusLabel, provisionJobStatusClass, provisionJobCaption, groupKey, groupAction, handleGroupPageChange,
  handleGroupPageSizeChange, loadCurrentGroups, syncCurrentChannelChecks, openGroupChannelProbeDialog, openProvisionDialog,
  openRepairDialog, syncCurrentGroups, previewEnsureCurrentKeys, ensureKeysPlanCanSubmit, ensureKeysPlanActionableCount,
  ensureCurrentKeys, standardizeCurrentKeyNames, importCurrentProviderKeyProjection
} = props.vm

const keyCapacityDialogOpen = ref(false)
const keyCapacitySubmitting = ref(false)
const keyCapacityGroup = ref<SupplierGroup | null>(null)
const keyCapacityForm = reactive<{ key_limit_policy: SupplierGroupKeyLimitPolicy; key_limit_value: number }>({ key_limit_policy: 'inherit', key_limit_value: 0 })

const plan = computed(() => rawValue(ensureKeysPlan))
const actionableCount = computed(() => ensureKeysPlanActionableCount())
const planWarnings = computed(() => plan.value?.warnings || [])
const legacyUnknownCapacityCount = computed(() => plan.value?.items.filter((item: EnsureSupplierKeysPlanItem) => item.action === 'blocked' && item.blocked_reason === 'key_capacity_unknown').length || 0)
const hasListFilter = computed(() => Boolean(groupFilters.q || groupFilters.status))
const errorMessages = computed(() => [groupsError, provisionJobError, ensureKeysPlanError, channelCheckError].map(rawValue).filter(Boolean))
const jobProgress = computed(() => {
  const job = rawValue(activeProvisionJob)
  if (job && ['succeeded', 'partial_succeeded', 'manual_required', 'dead', 'cancelled'].includes(job.status)) return 100
  if (!job?.total_steps) return job?.status === 'succeeded' ? 100 : 0
  return Math.min(100, Math.round(((job.succeeded_steps + job.failed_steps + job.manual_required_steps) / job.total_steps) * 100))
})
const keyCapacityFormValid = computed(() => keyCapacityForm.key_limit_policy !== 'limited' || Number(keyCapacityForm.key_limit_value) > 0)
const primarySubmitLabel = computed(() => {
  const createCount = plan.value?.to_create || 0
  return createCount > 0 ? `开始创建 ${createCount} 个 Key` : `开始接入 ${actionableCount.value} 个已有 Key`
})

function rawValue<T>(value: T | { value: T }): T {
  return value && typeof value === 'object' && 'value' in value ? (value as { value: T }).value : value as T
}

function dismissPlan() {
  if (ensureKeysPlan && typeof ensureKeysPlan === 'object' && 'value' in ensureKeysPlan) ensureKeysPlan.value = null
}

function planWarningLabel(warning: string): string {
  if (warning === 'key_capacity_unknown') return '第三方未提供 Key 数量上限，系统将直接尝试创建，并按实际结果逐项记录。'
  if (warning === 'provider_key_capacity_incomplete') return '第三方 Key 列表未完整读取；创建时仍会按稳定名称检查并复用已有 Key。'
  return warning
}

function planActionLabel(item: EnsureSupplierKeysPlanItem): string {
  if (item.action === 'create') return '待创建'
  if (item.action === 'reuse') return '直接绑定'
  if (item.action === 'skipped_existing') return '检查本地接入'
  if (item.blocked_reason === 'provider_key_exists_unbound') return '需补录密钥'
  if (item.blocked_reason === 'group_key_provisioning_unsupported') return '无法自动创建'
  return '需人工处理'
}

function planActionClass(item: EnsureSupplierKeysPlanItem): string {
  if (item.action === 'create') return 'badge-success'
  if (item.action === 'reuse') return 'badge-primary'
  if (item.action === 'blocked') return 'badge-warning'
  return 'badge-gray'
}

function planItemDescription(item: EnsureSupplierKeysPlanItem): string {
  if (item.blocked_reason === 'provider_key_exists_unbound') return '第三方已有 Key，但 SuperLLM 无法读取明文。'
  if (item.blocked_reason === 'key_capacity_unknown') return '当前后端仍把未知的第三方 Key 上限作为创建前置条件。'
  if (item.blocked_reason === 'key_capacity_exhausted') return '已达到运营者设置的本地 Key 上限。'
  if (item.blocked_reason === 'group_key_capacity_exhausted') return '该分组已达到本地 Key 上限。'
  if (item.blocked_reason === 'group_key_capacity_unknown') return '该分组设置为需要人工确认。'
  if (item.blocked_reason === 'group_key_provisioning_unsupported') return '该分组被设置为不允许自动创建。'
  if (item.action === 'create' && item.warnings?.length) return item.warnings.map(planWarningLabel).join(' ')
  if (item.action === 'create') return `按优先级 ${item.priority || '-'} 创建。`
  if (item.action === 'reuse') return '使用第三方已有 Key，并自动接入本地账号。'
  return '确认现有 Key 的本地账号绑定。'
}

function groupKeyPolicyLabel(value?: string): string {
  return ({ inherit: '跟随供应商', unknown: '人工确认', unlimited: '无本地上限', limited: '本地有限', unsupported: '禁止自动创建' } as Record<string, string>)[value || 'inherit'] || value || '-'
}

function groupKeyCapacityStatusLabel(value?: string): string {
  return ({ inherit: '跟随供应商', available: '可创建', limited: '接近本地上限', exhausted: '本地已满', unknown: '人工确认', unsupported: '禁止自动创建' } as Record<string, string>)[value || 'inherit'] || value || '-'
}

function groupKeyCapacityClass(value?: string): string {
  if (value === 'available' || value === 'inherit') return 'badge-success'
  if (value === 'limited' || value === 'unknown') return 'badge-warning'
  if (value === 'exhausted' || value === 'unsupported') return 'badge-danger'
  return 'badge-gray'
}

function handleEmptyAction() {
  if (hasListFilter.value) {
    groupFilters.q = ''
    groupFilters.status = ''
    return
  }
  void syncCurrentGroups()
}

function openGroupKeyCapacitySettings(group: SupplierGroup) {
  keyCapacityGroup.value = group
  keyCapacityForm.key_limit_policy = normalizeGroupKeyPolicy(group.key_limit_policy)
  keyCapacityForm.key_limit_value = Number(group.key_limit_value || 0)
  keyCapacityDialogOpen.value = true
}

function closeGroupKeyCapacityDialog() {
  if (keyCapacitySubmitting.value) return
  keyCapacityDialogOpen.value = false
  keyCapacityGroup.value = null
}

function normalizeGroupKeyPolicy(value?: string): SupplierGroupKeyLimitPolicy {
  return value === 'unknown' || value === 'unlimited' || value === 'limited' || value === 'unsupported' ? value : 'inherit'
}

async function submitGroupKeyCapacity() {
  const supplier = rawValue(groupsSupplier)
  if (!supplier || !keyCapacityGroup.value || !keyCapacityFormValid.value) return
  keyCapacitySubmitting.value = true
  try {
    await updateSupplierGroupKeyCapacity(supplier.id, keyCapacityGroup.value.id, {
      key_limit_policy: keyCapacityForm.key_limit_policy,
      key_limit_value: keyCapacityForm.key_limit_policy === 'limited' ? Number(keyCapacityForm.key_limit_value) : 0
    })
    appStore.showSuccess('本地 Key 上限已更新')
    keyCapacityDialogOpen.value = false
    keyCapacityGroup.value = null
    await loadCurrentGroups()
    if (plan.value) await previewEnsureCurrentKeys()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '更新本地 Key 上限失败')
  } finally {
    keyCapacitySubmitting.value = false
  }
}
</script>
