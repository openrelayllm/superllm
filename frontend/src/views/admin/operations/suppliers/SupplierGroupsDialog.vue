<template>
<BaseDialog :show="groupsDialogOpen" :title="groupsSupplier ? `供应商分组 - ${groupsSupplier.name}` : '供应商分组'" width="full" @close="closeGroupsDialog">
  <div class="space-y-4">
    <div class="flex flex-wrap items-end justify-between gap-3">
      <div class="grid flex-1 gap-3 sm:grid-cols-[minmax(180px,1fr)_160px]">
        <label class="block">
          <span class="input-label">搜索</span>
          <div class="relative">
            <Icon name="search" size="sm" class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
            <input v-model.trim="groupFilters.q" class="input pl-9" placeholder="分组名称、平台、ID" />
          </div>
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
        <button type="button" class="btn btn-secondary" :disabled="groupsLoading || !groupsSupplier" @click="loadCurrentGroups">
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
        <label class="flex h-9 items-center gap-2 rounded-md border border-gray-200 px-3 text-sm text-gray-700 dark:border-dark-700 dark:text-dark-200" title="默认只更新本地名称；勾选后把第三方 Key 名称改为稳定别名。">
          <input v-model="keyNamingForm.sync_provider_name" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500 dark:border-dark-600 dark:bg-dark-900" />
          <span>同步第三方 Key 名称</span>
        </label>
        <button type="button" class="btn btn-secondary" :disabled="keyNamesStandardizing || !groupsSupplier" @click="standardizeCurrentKeyNames">
          <Icon name="key" size="sm" :class="{ 'animate-spin': keyNamesStandardizing }" />
          规范 Key 名称
        </button>
        <button type="button" class="btn btn-primary" :disabled="keysEnsuring || !canSubmitEnsureKeys" @click="ensureCurrentKeys">
          <Icon name="key" size="sm" :class="{ 'animate-spin': keysEnsuring }" />
          补齐 Key/账号
        </button>
      </div>
    </div>

    <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
      <div class="grid gap-3 sm:grid-cols-4">
        <div v-for="step in groupWorkflowSteps" :key="step.key" class="flex items-start gap-3">
          <span class="mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-full text-xs font-semibold" :class="workflowStepDotClass(step.status)">
            <Icon v-if="step.status === 'succeeded'" name="checkCircle" size="xs" />
            <Icon v-else-if="step.status === 'running' || step.status === 'retryable_failed'" name="refresh" size="xs" class="animate-spin" />
            <span v-else>{{ step.label.slice(0, 1) }}</span>
          </span>
          <span class="min-w-0">
            <span class="block text-sm font-medium text-gray-900 dark:text-gray-100">{{ step.label }}</span>
            <span class="block truncate text-xs text-gray-500 dark:text-dark-400" :title="step.caption">{{ step.caption }}</span>
          </span>
        </div>
      </div>

      <div v-if="activeProvisionJob" class="mt-4 rounded-md border border-gray-100 bg-gray-50 p-3 text-sm dark:border-dark-700 dark:bg-dark-900/40">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <div class="flex flex-wrap items-center gap-2">
            <span class="badge" :class="provisionJobStatusClass(activeProvisionJob.status)">{{ provisionJobStatusLabel(activeProvisionJob.status) }}</span>
            <span class="font-medium text-gray-900 dark:text-gray-100">{{ provisionJobTypeLabel(activeProvisionJob.job_type) }}</span>
            <span class="font-mono text-xs text-gray-500 dark:text-dark-400">#{{ activeProvisionJob.id }}</span>
          </div>
          <span class="text-xs text-gray-500 dark:text-dark-400">{{ formatDateTime(activeProvisionJob.updated_at) }}</span>
        </div>
        <div v-if="activeProvisionJob.error_message" class="mt-2 text-xs text-red-600 dark:text-red-300">
          {{ activeProvisionJob.error_message }}
        </div>
        <div v-else class="mt-2 text-xs text-gray-500 dark:text-dark-400">
          {{ provisionJobCaption(activeProvisionJob) }}
        </div>
      </div>
    </div>

    <div v-if="groupsError" class="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-200">
      {{ groupsError }}
    </div>

    <div v-if="provisionJobError" class="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-200">
      {{ provisionJobError }}
    </div>

    <div v-if="channelCheckError" class="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-200">
      {{ channelCheckError }}
    </div>

    <DataTable
      :columns="groupColumns"
      :data="supplierGroups"
      :loading="groupsLoading"
      row-key="id"
      default-sort-key="last_seen_at"
      default-sort-order="desc"
      :estimate-row-height="88"
    >
      <template #cell-name="{ row }">
        <div class="w-[190px] space-y-1 whitespace-normal">
          <div class="flex min-w-0 items-center gap-2">
            <GroupBadge
              class="max-w-full"
              :name="row.name"
              :platform="groupPlatform(row.provider_family, row.name, row.description)"
              :rate-multiplier="groupCostMultiplier(row)"
            />
            <span v-if="row.is_private" class="badge badge-warning">专属</span>
            <span v-if="row.allow_image_generation" class="badge badge-primary">图片</span>
          </div>
          <div class="flex min-w-0 flex-col gap-0.5 text-xs text-gray-500 dark:text-dark-400">
            <span class="font-mono">#{{ row.external_group_id }}</span>
            <span v-if="row.description" class="truncate" :title="row.description">{{ row.description }}</span>
          </div>
        </div>
      </template>

      <template #cell-provider_family="{ row }">
        <div class="w-[72px]">
          <span class="badge badge-gray max-w-full truncate">{{ row.provider_family || 'mixed' }}</span>
        </div>
      </template>

      <template #cell-rate="{ row }">
        <div class="w-[86px] space-y-0.5 text-right">
          <div :class="rateMultiplierTextClass(groupCostMultiplier(row), channelProtocolFromProviderFamily(row.provider_family, row.name, row.description))">
            {{ formatMultiplier(groupCostMultiplier(row)) }}
          </div>
          <div class="flex flex-col text-xs text-gray-500 dark:text-dark-400">
            <span>
              使用
              <span>{{ formatMultiplier(row.effective_rate_multiplier) }}</span>
            </span>
            <span>充值 {{ formatMultiplier(currentSupplierRechargeMultiplier()) }}</span>
          </div>
        </div>
      </template>

      <template #cell-limits="{ row }">
        <div class="w-[82px] text-xs text-gray-600 dark:text-dark-300">
          <div>RPM：{{ row.rpm_limit ?? '-' }}</div>
          <div>日：{{ formatUSDLimit(row.daily_limit_usd) }}</div>
          <div>月：{{ formatUSDLimit(row.monthly_limit_usd) }}</div>
        </div>
      </template>

      <template #cell-account="{ row }">
        <div class="w-[250px] whitespace-normal">
          <template v-if="groupKey(row)">
            <div class="flex min-w-0 items-center gap-2">
              <span class="truncate font-medium text-gray-900 dark:text-gray-100" :title="groupKey(row)?.name || ''">{{ groupKey(row)?.name || '-' }}</span>
              <span class="badge shrink-0" :class="supplierKeyStatusClass(groupKey(row)?.status)">{{ supplierKeyStatusLabel(groupKey(row)?.status) }}</span>
            </div>
            <div class="mt-1 grid grid-cols-[auto_1fr] gap-x-2 gap-y-0.5 text-xs text-gray-500 dark:text-dark-400">
              <span v-if="groupKey(row)?.key_last4" class="font-mono">****{{ groupKey(row)?.key_last4 }}</span>
              <span v-if="groupKey(row)?.external_key_id" class="font-mono">Key #{{ groupKey(row)?.external_key_id }}</span>
              <span v-if="groupKey(row)?.local_sub2api_account_id">本地 #{{ groupKey(row)?.local_sub2api_account_id }}</span>
              <span v-if="groupKey(row)?.local_account_name" class="truncate" :title="groupKey(row)?.local_account_name">{{ groupKey(row)?.local_account_name }}</span>
            </div>
            <div v-if="groupKey(row)?.error_message" class="mt-1 truncate text-xs text-red-600 dark:text-red-300" :title="groupKey(row)?.error_message">
              {{ groupKey(row)?.error_message }}
            </div>
          </template>
          <template v-else>
            <div class="flex flex-wrap items-center gap-2">
              <span class="badge badge-gray">未开通</span>
              <span class="text-xs text-gray-500 dark:text-dark-400">未进入切换候选</span>
            </div>
          </template>
        </div>
      </template>

      <template #cell-channel_check="{ row }">
        <div class="w-[190px] whitespace-normal">
          <template v-if="groupChannelCheck(row.id)">
            <div class="flex flex-wrap items-center gap-1.5">
              <span class="badge" :class="channelProbeStatusClass(groupChannelCheck(row.id)?.probe_status)">
                {{ channelProbeStatusLabel(groupChannelCheck(row.id)?.probe_status) }}
              </span>
              <span class="badge" :class="groupChannelCheck(row.id)?.local_account_schedulable ? 'badge-success' : 'badge-warning'">
                {{ groupChannelCheck(row.id)?.local_account_schedulable ? '调度中' : '已暂停' }}
              </span>
            </div>
            <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">
              首 {{ formatLatency(groupChannelCheck(row.id)?.first_token_ms) }} · 总 {{ formatLatency(groupChannelCheck(row.id)?.duration_ms) }}
            </div>
            <div class="mt-1 truncate text-xs text-gray-500 dark:text-dark-400" :title="groupChannelCheck(row.id)?.error_message || ''">
              {{ groupChannelCheck(row.id)?.error_message || formatDateTime(groupChannelCheck(row.id)?.captured_at) }}
            </div>
          </template>
          <template v-else>
            <span class="badge badge-gray">未检测</span>
          </template>
        </div>
      </template>

      <template #cell-group_actions="{ row }">
        <div class="ml-auto flex w-[90px] flex-col gap-1">
          <button
            v-if="groupAction(row).kind === 'provision'"
            type="button"
            class="btn btn-secondary btn-sm h-8 px-2"
            :disabled="groupAction(row).disabled"
            :title="groupAction(row).title"
            @click="openProvisionDialog(row)"
          >
            <Icon :name="groupAction(row).icon" size="sm" />
            {{ groupAction(row).label }}
          </button>
          <button
            v-if="groupAction(row).kind === 'repair_sub2api_landing'"
            type="button"
            class="btn btn-secondary btn-sm h-8 px-2"
            :disabled="groupAction(row).disabled"
            :title="groupAction(row).title"
            @click="openRepairDialog(groupKey(row)!)"
          >
            <Icon :name="groupAction(row).icon" size="sm" />
            {{ groupAction(row).label }}
          </button>
          <button
            type="button"
            class="btn btn-secondary btn-sm h-8 px-2"
            :disabled="isChannelCheckActionRunning(`probe:${row.id}`)"
            title="使用 GPT-5.4 Mini 真实复测该渠道，失败时自动暂停本地调度"
            @click="probeGroupChannel(row)"
          >
            <Icon name="beaker" size="sm" :class="{ 'animate-spin': isChannelCheckActionRunning(`probe:${row.id}`) }" />
            复测
          </button>
          <button
            v-if="groupHasLocalBinding(row)"
            type="button"
            class="btn btn-secondary btn-sm h-8 px-2"
            :disabled="groupScheduleActionDisabled(row)"
            :title="groupScheduleActionTitle(row)"
            @click="handleGroupScheduleAction(row)"
          >
            <Icon :name="groupScheduleActionIcon(row)" size="sm" :class="{ 'animate-spin': isChannelCheckActionRunning(`schedule:${row.id}`) }" />
            {{ groupScheduleActionLabel(row) }}
          </button>
        </div>
      </template>

      <template #cell-status="{ row }">
        <div class="w-[64px]">
          <span class="badge" :class="groupStatusClass(row.status)">{{ groupStatusLabel(row.status) }}</span>
        </div>
      </template>

      <template #cell-last_seen_at="{ row }">
        <div class="w-[118px] whitespace-normal text-xs text-gray-500 dark:text-dark-400">{{ formatDateTime(row.last_seen_at) }}</div>
      </template>

      <template #empty>
        <EmptyState
          title="暂无供应商分组"
          description="先完成后端直登或浏览器会话上报，再同步供应商分组。"
          action-text="同步分组"
          @action="syncCurrentGroups"
        />
      </template>
    </DataTable>

    <Pagination
      v-if="groupPagination.total > 0"
      :page="groupPagination.page"
      :total="groupPagination.total"
      :page-size="groupPagination.page_size"
      @update:page="handleGroupPageChange"
      @update:pageSize="handleGroupPageSizeChange"
    />
  </div>

  <template #footer>
    <button type="button" class="btn btn-secondary" @click="closeGroupsDialog">关闭</button>
  </template>
</BaseDialog>


</template>

<script setup lang="ts">
import BaseDialog from '@/components/common/BaseDialog.vue'
import DataTable from '@/components/common/DataTable.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import GroupBadge from '@/components/common/GroupBadge.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'
const props = defineProps<{ vm: any }>()
const {
  groupsDialogOpen,
  groupsSupplier,
  supplierGroups,
  activeProvisionJob,
  groupsLoading,
  groupsSyncing,
  keysEnsuring,
  keyNamesStandardizing,
  channelChecksSyncing,
  groupsError,
  provisionJobError,
  channelCheckError,
  groupPagination,
  groupColumns,
  groupFilters,
  keyNamingForm,
  activeProvisionJobRunning,
  groupWorkflowSteps,
  canSubmitGroupSync,
  canSubmitEnsureKeys,
  formatDateTime,
  formatMultiplier,
  rateMultiplierTextClass,
  formatLatency,
  formatUSDLimit,
  groupPlatform,
  channelProtocolFromProviderFamily,
  currentSupplierRechargeMultiplier,
  groupCostMultiplier,
  groupChannelCheck,
  isChannelCheckActionRunning,
  groupHasLocalBinding,
  groupScheduleActionLabel,
  groupScheduleActionIcon,
  groupScheduleActionDisabled,
  groupScheduleActionTitle,
  groupStatusLabel,
  groupStatusClass,
  supplierKeyStatusLabel,
  supplierKeyStatusClass,
  provisionJobTypeLabel,
  channelProbeStatusLabel,
  channelProbeStatusClass,
  provisionJobStatusLabel,
  provisionJobStatusClass,
  workflowStepDotClass,
  provisionJobCaption,
  groupKey,
  groupAction,
  handleGroupPageChange,
  handleGroupPageSizeChange,
  closeGroupsDialog,
  loadCurrentGroups,
  syncCurrentChannelChecks,
  probeGroupChannel,
  handleGroupScheduleAction,
  openProvisionDialog,
  openRepairDialog,
  syncCurrentGroups,
  ensureCurrentKeys,
  standardizeCurrentKeyNames
} = props.vm
</script>
