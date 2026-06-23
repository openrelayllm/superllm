<template>
<BaseDialog
  :show="channelScheduleDialogOpen"
  :title="channelScheduleSupplier ? `加入调度 - ${channelScheduleSupplier.name}` : '加入调度'"
  width="normal"
  @close="closeBestChannelScheduleDialog"
>
  <div class="space-y-4">
    <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
      <div class="grid gap-3 sm:grid-cols-2">
        <div>
          <div class="text-xs text-gray-500 dark:text-dark-400">供应商</div>
          <div class="mt-1 text-sm font-semibold text-gray-900 dark:text-gray-100">{{ channelScheduleSupplier?.name || '-' }}</div>
        </div>
        <div>
          <div class="text-xs text-gray-500 dark:text-dark-400">分组</div>
          <div class="mt-1 text-sm font-semibold text-gray-900 dark:text-gray-100">{{ channelScheduleSnapshot?.group_name || '-' }}</div>
        </div>
        <div>
          <div class="text-xs text-gray-500 dark:text-dark-400">检测状态</div>
          <div class="mt-1">
            <span class="badge" :class="channelProbeStatusClass(channelScheduleSnapshot?.probe_status)">
              {{ channelProbeStatusLabel(channelScheduleSnapshot?.probe_status) }}
            </span>
            <span class="ml-2 text-xs text-gray-500 dark:text-dark-400">
              首 {{ formatLatency(channelScheduleSnapshot?.first_token_ms) }} · 总 {{ formatLatency(channelScheduleSnapshot?.duration_ms) }}
            </span>
          </div>
        </div>
        <div>
          <div class="text-xs text-gray-500 dark:text-dark-400">倍率</div>
          <div class="mt-1" :class="rateMultiplierTextClass(channelCostMultiplier(channelScheduleSnapshot), channelProtocol(channelScheduleSnapshot))">
            {{ formatMultiplier(channelCostMultiplier(channelScheduleSnapshot)) }}
          </div>
          <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">使用 {{ formatMultiplier(channelScheduleSnapshot?.effective_rate_multiplier) }} / 充值 {{ formatMultiplier(channelScheduleSupplierRechargeMultiplier()) }}</div>
        </div>
        <div>
          <div class="text-xs text-gray-500 dark:text-dark-400">本地账号</div>
          <div class="mt-1">
            <span class="badge" :class="channelScheduleLocalAccountBadgeClass">
              {{ channelScheduleLocalAccountLabel }}
            </span>
          </div>
        </div>
        <div>
          <div class="text-xs text-gray-500 dark:text-dark-400">调度状态</div>
          <div class="mt-1">
            <span class="badge" :class="channelScheduleSnapshot?.local_account_schedulable ? 'badge-success' : 'badge-gray'">
              {{ channelScheduleSnapshot?.local_account_schedulable ? '调度中' : '未调度' }}
            </span>
          </div>
        </div>
      </div>
    </div>

    <div class="space-y-2">
      <div
        v-for="step in channelScheduleSteps"
        :key="step.key"
        class="flex gap-3 rounded-lg border p-3"
        :class="channelScheduleStepClass(step.status)"
      >
        <div class="mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-full" :class="channelScheduleStepIconClass(step.status)">
          <Icon :name="channelScheduleStepIconName(step)" size="sm" />
        </div>
        <div class="min-w-0">
          <div class="text-sm font-medium text-gray-900 dark:text-gray-100">{{ step.label }}</div>
          <div class="mt-0.5 text-xs leading-5 text-gray-600 dark:text-dark-300">{{ step.description }}</div>
        </div>
      </div>
    </div>

    <div v-if="channelScheduleSnapshot?.error_message" class="rounded-lg border border-gray-200 bg-gray-50 p-4 text-sm text-gray-700 dark:border-dark-700 dark:bg-dark-900/40 dark:text-dark-200">
      {{ channelScheduleSnapshot.error_message }}
    </div>
  </div>

  <template #footer>
    <button type="button" class="btn btn-secondary" @click="closeBestChannelScheduleDialog">关闭</button>
    <button type="button" class="btn btn-secondary" :disabled="!channelScheduleSupplier" @click="openChannelScheduleGroups">
      <Icon name="database" size="sm" />
      分组详情
    </button>
    <button
      type="button"
      class="btn btn-primary"
      :disabled="channelScheduleSubmitting || !channelScheduleSupplier"
      @click="confirmChannelSchedulePrimaryAction"
    >
      <Icon :name="channelSchedulePrimaryIcon" size="sm" :class="{ 'animate-spin': channelScheduleSubmitting }" />
      {{ channelSchedulePrimaryLabel }}
    </button>
  </template>
</BaseDialog>

<BaseDialog
  :show="scheduleListDialogOpen"
  title="调度列表"
  width="full"
  @close="closeScheduleListDialog"
>
  <div class="space-y-4">
    <div class="grid gap-3 md:grid-cols-4">
      <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
        <div class="text-xs text-gray-500 dark:text-dark-400">绑定账号</div>
        <div class="mt-1 text-xl font-semibold text-gray-900 dark:text-gray-100">{{ scheduleListStats.total }}</div>
      </div>
      <div class="rounded-lg border border-emerald-200 bg-emerald-50 p-4 dark:border-emerald-800 dark:bg-emerald-900/20">
        <div class="text-xs text-emerald-700 dark:text-emerald-300">调度中</div>
        <div class="mt-1 text-xl font-semibold text-emerald-800 dark:text-emerald-100">{{ scheduleListStats.scheduled }}</div>
      </div>
      <div class="rounded-lg border border-amber-200 bg-amber-50 p-4 dark:border-amber-800 dark:bg-amber-900/20">
        <div class="text-xs text-amber-700 dark:text-amber-300">已暂停</div>
        <div class="mt-1 text-xl font-semibold text-amber-800 dark:text-amber-100">{{ scheduleListStats.paused }}</div>
      </div>
      <div class="rounded-lg border border-rose-200 bg-rose-50 p-4 dark:border-rose-800 dark:bg-rose-900/20">
        <div class="text-xs text-rose-700 dark:text-rose-300">异常/未检测</div>
        <div class="mt-1 text-xl font-semibold text-rose-800 dark:text-rose-100">{{ scheduleListStats.risky }}</div>
      </div>
    </div>

    <div class="flex flex-wrap items-end gap-3">
      <label class="block min-w-[240px] flex-1">
        <span class="input-label">搜索</span>
        <div class="relative">
          <Icon name="search" size="sm" class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
          <input v-model.trim="scheduleListFilters.q" class="input pl-9" placeholder="供应商、账号、分组、渠道" />
        </div>
      </label>
      <label class="block w-44">
        <span class="input-label">调度状态</span>
        <select v-model="scheduleListFilters.status" class="input">
          <option value="">全部</option>
          <option value="scheduled">调度中</option>
          <option value="paused">已暂停</option>
          <option value="risky">异常/未检测</option>
          <option value="untested">未检测</option>
        </select>
      </label>
      <label class="block w-52">
        <span class="input-label">本地分组</span>
        <select v-model="scheduleListFilters.local_group" class="input">
          <option value="">全部本地分组</option>
          <option
            v-for="option in scheduleListLocalGroupOptions"
            :key="option.value"
            :value="option.value"
          >
            {{ option.label }} ({{ option.count }})
          </option>
        </select>
      </label>
      <button type="button" class="btn btn-secondary" :disabled="scheduleListLoading" @click="loadScheduleList">
        <Icon name="refresh" size="sm" :class="{ 'animate-spin': scheduleListLoading }" />
        刷新
      </button>
    </div>

    <div v-if="scheduleListError" class="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-200">
      {{ scheduleListError }}
    </div>

    <DataTable
      :columns="scheduleListColumns"
      :data="filteredScheduleRows"
      :loading="scheduleListLoading"
      row-key="key"
      default-sort-key="supplier_name"
      default-sort-order="asc"
      :estimate-row-height="76"
      :sticky-first-column="false"
      :sticky-actions-column="false"
    >
      <template #cell-name="{ row }">
        <div class="min-w-[260px] max-w-[360px]">
          <div class="flex min-w-0 items-center gap-2">
            <span class="truncate font-medium text-gray-900 dark:text-white" :title="row.local_account_name">{{ row.local_account_name }}</span>
            <span class="badge badge-gray">#{{ row.local_account_id }}</span>
          </div>
          <div v-if="shouldShowScheduleSupplierName(row)" class="mt-1 flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-dark-400">
            <span>{{ row.supplier_name }}</span>
          </div>
        </div>
      </template>

      <template #cell-status="{ row }">
        <div class="flex min-w-[132px] flex-col gap-1.5">
          <div class="flex flex-wrap gap-1.5">
            <span class="badge w-fit" :class="runtimeClass(row.runtime_status)">{{ runtimeLabel(row.runtime_status) }}</span>
            <span class="badge w-fit" :class="healthClass(row.health_status)">{{ healthLabel(row.health_status) }}</span>
          </div>
          <span v-if="row.local_account_status !== 'active'" class="text-xs font-medium text-red-600 dark:text-red-300">账号非 active</span>
        </div>
      </template>

      <template #cell-schedulable="{ row }">
        <div class="min-w-[92px]">
          <button
            type="button"
            class="relative inline-flex h-5 w-9 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50 dark:focus:ring-offset-dark-800"
            :class="[row.schedulable ? 'bg-primary-500 hover:bg-primary-600' : 'bg-gray-200 hover:bg-gray-300 dark:bg-dark-600 dark:hover:bg-dark-500']"
            :disabled="scheduleListActionKey === row.key || !row.supplier_group_id"
            :title="row.schedulable ? '暂停调度' : '校验绑定并加入调度'"
            @click="toggleScheduleRow(row)"
          >
            <span class="pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out" :class="[row.schedulable ? 'translate-x-4' : 'translate-x-0']" />
          </button>
          <div class="mt-1 text-xs" :class="row.schedulable ? 'text-emerald-600 dark:text-emerald-400' : 'text-gray-500 dark:text-dark-400'">
            {{ row.schedulable ? '参与调度' : '未入调度' }}
          </div>
        </div>
      </template>

      <template #cell-group="{ row }">
        <div class="min-w-[320px] max-w-[380px] space-y-2">
          <div>
            <div class="text-[11px] font-medium text-gray-500 dark:text-dark-400">供应商渠道</div>
            <div class="mt-1 flex flex-wrap items-center gap-2">
              <GroupBadge
                :name="row.group_name || '未同步分组'"
                :platform="groupPlatformFromProvider(row.provider_family, row.group_name)"
                :rate-multiplier="scheduleRowCostMultiplier(row)"
              />
              <span class="text-xs text-gray-500 dark:text-dark-400">{{ providerLabel(row.provider_family) }}</span>
              <span v-if="row.external_group_id" class="font-mono text-xs text-gray-500 dark:text-dark-400">#{{ row.external_group_id }}</span>
            </div>
          </div>
          <div>
            <div class="text-[11px] font-medium text-gray-500 dark:text-dark-400">本地调度分组</div>
            <div v-if="row.local_group_names.length > 0" class="mt-1 flex flex-wrap gap-1.5">
              <span
                v-for="(groupName, index) in row.local_group_names"
                :key="`${row.key}:local-group:${row.local_group_ids[index] || groupName}`"
                class="inline-flex max-w-[120px] items-center gap-1 rounded-md border border-primary-200 bg-primary-50 px-2 py-0.5 text-xs font-medium text-primary-700 dark:border-primary-800 dark:bg-primary-900/20 dark:text-primary-200"
                :title="row.local_group_ids[index] ? `${groupName} #${row.local_group_ids[index]}` : groupName"
              >
                <span class="truncate">{{ groupName }}</span>
                <span v-if="row.local_group_ids[index]" class="font-mono text-[10px] opacity-70">#{{ row.local_group_ids[index] }}</span>
              </span>
            </div>
            <span v-else class="mt-1 inline-flex rounded-md border border-amber-200 bg-amber-50 px-2 py-0.5 text-xs font-medium text-amber-700 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-200">
              未分配本地分组
            </span>
          </div>
        </div>
      </template>

      <template #cell-probe="{ row }">
        <div class="min-w-[190px] max-w-[260px]">
          <span class="badge" :class="channelProbeStatusClass(row.probe_status)">{{ channelProbeStatusLabel(row.probe_status) }}</span>
          <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">
            首 {{ formatLatency(row.first_token_ms) }} · 总 {{ formatLatency(row.duration_ms) }}
          </div>
          <div class="mt-1 max-w-[180px] truncate text-xs text-gray-400" :title="row.error_message || ''">
            {{ row.error_message || formatDateTime(row.captured_at) }}
          </div>
        </div>
      </template>

      <template #cell-actions="{ row }">
        <div class="flex min-w-[116px] items-center justify-end gap-1">
          <button
            type="button"
            class="btn btn-secondary btn-sm h-8 w-8 px-0"
            :disabled="scheduleListActionKey === row.key || !row.supplier_group_id"
            title="重新检测该渠道"
            @click="probeScheduleRow(row)"
          >
            <Icon name="beaker" size="xs" :class="{ 'animate-spin': scheduleListActionKey === row.key }" />
            <span class="sr-only">复测</span>
          </button>
          <button
            type="button"
            class="btn btn-secondary btn-sm h-8 w-8 px-0"
            :disabled="scheduleListActionKey === row.key || !row.supplier_group_id"
            :title="row.schedulable ? '暂停该本地账号调度' : '校验绑定并加入调度'"
            @click="toggleScheduleRow(row)"
          >
            <Icon :name="row.schedulable ? 'ban' : 'play'" size="xs" :class="{ 'animate-spin': scheduleListActionKey === row.key }" />
            <span class="sr-only">{{ row.schedulable ? '暂停' : '加入' }}</span>
          </button>
          <button type="button" class="btn btn-secondary btn-sm h-8 w-8 px-0" title="打开供应商分组" @click="openScheduleRowGroups(row)">
            <Icon name="database" size="xs" />
            <span class="sr-only">分组</span>
          </button>
        </div>
      </template>

      <template #empty>
        <EmptyState
          title="暂无调度账号"
          description="请先在供应商分组中补齐 Key/账号，或同步供应商分组后重新检测渠道。"
        />
      </template>
    </DataTable>
  </div>

  <template #footer>
    <button type="button" class="btn btn-secondary" @click="closeScheduleListDialog">关闭</button>
    <button type="button" class="btn btn-secondary" :disabled="scheduleListLoading" @click="loadScheduleList">
      <Icon name="refresh" size="sm" :class="{ 'animate-spin': scheduleListLoading }" />
      刷新
    </button>
  </template>
</BaseDialog>


</template>

<script setup lang="ts">
import BaseDialog from '@/components/common/BaseDialog.vue'
import DataTable from '@/components/common/DataTable.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import GroupBadge from '@/components/common/GroupBadge.vue'
import Icon from '@/components/icons/Icon.vue'
const props = defineProps<{ vm: any }>()
const {
  channelScheduleDialogOpen,
  scheduleListDialogOpen,
  channelScheduleSupplier,
  channelScheduleSubmitting,
  scheduleListLoading,
  scheduleListActionKey,
  scheduleListError,
  scheduleListFilters,
  scheduleListColumns,
  filteredScheduleRows,
  scheduleListLocalGroupOptions,
  scheduleListStats,
  channelScheduleSnapshot,
  channelSchedulePrimaryLabel,
  channelSchedulePrimaryIcon,
  channelScheduleLocalAccountLabel,
  channelScheduleLocalAccountBadgeClass,
  channelScheduleSteps,
  formatDateTime,
  formatMultiplier,
  rateMultiplierTextClass,
  formatLatency,
  groupPlatformFromProvider,
  channelScheduleSupplierRechargeMultiplier,
  channelCostMultiplier,
  scheduleRowCostMultiplier,
  providerLabel,
  runtimeLabel,
  healthLabel,
  runtimeClass,
  healthClass,
  channelProtocol,
  shouldShowScheduleSupplierName,
  channelScheduleStepClass,
  channelScheduleStepIconClass,
  channelScheduleStepIconName,
  channelProbeStatusLabel,
  channelProbeStatusClass,
  closeScheduleListDialog,
  loadScheduleList,
  toggleScheduleRow,
  probeScheduleRow,
  openScheduleRowGroups,
  closeBestChannelScheduleDialog,
  openChannelScheduleGroups,
  confirmChannelSchedulePrimaryAction
} = props.vm
</script>
