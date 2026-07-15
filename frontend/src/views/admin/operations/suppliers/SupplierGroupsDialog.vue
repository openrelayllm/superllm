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
        <button type="button" class="btn btn-secondary" :disabled="ensureKeysPlanning || !canSubmitEnsureKeys" @click="previewEnsureCurrentKeys">
          <Icon name="clipboard" size="sm" :class="{ 'animate-spin': ensureKeysPlanning }" />
          开通计划
        </button>
        <button type="button" class="btn btn-primary" :disabled="keysEnsuring || !canSubmitEnsureKeys || !ensureKeysPlanCanSubmit()" @click="ensureCurrentKeys">
          <Icon name="key" size="sm" :class="{ 'animate-spin': keysEnsuring }" />
          {{ ensureKeysPlan?.blocked > 0 ? '提交可处理部分' : '提交补齐' }}
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

    <div v-if="ensureKeysPlan" class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h3 class="text-sm font-semibold text-gray-900 dark:text-gray-100">Key 开通计划</h3>
          <p class="mt-1 text-sm text-gray-600 dark:text-dark-300">
            策略 {{ ensurePlanPolicyLabel(ensureKeysPlan.key_limit_policy) }} · 已用 {{ ensureKeysPlan.active_key_count }}{{ ensureKeysPlan.key_limit_policy === 'limited' ? `/${ensureKeysPlan.key_limit_value}` : '' }}
          </p>
        </div>
        <div class="flex flex-wrap justify-end gap-2">
          <button
            v-if="priorityPlanItems.length > 1"
            type="button"
            class="btn btn-secondary btn-sm"
            :disabled="ensureKeysPlanning || ensureKeysPriorityGroupIDs.length === 0"
            @click="resetEnsureKeyPriority"
          >
            <Icon name="refresh" size="xs" :class="{ 'animate-spin': ensureKeysPlanning }" />
            默认优先级
          </button>
          <button type="button" class="btn btn-primary btn-sm" :disabled="keysEnsuring || !ensureKeysPlanCanSubmit()" @click="ensureCurrentKeys">
            <Icon name="key" size="sm" :class="{ 'animate-spin': keysEnsuring }" />
            {{ ensureKeysPlan.blocked > 0 ? '只提交可处理部分' : '提交任务' }}
          </button>
        </div>
      </div>
      <div class="mt-4 grid gap-3 sm:grid-cols-5">
        <div class="rounded-md border border-gray-100 px-3 py-2 dark:border-dark-700">
          <div class="text-xs text-gray-500 dark:text-dark-400">有效分组</div>
          <div class="mt-1 text-lg font-semibold text-gray-900 dark:text-gray-100">{{ ensureKeysPlan.total }}</div>
        </div>
        <div class="rounded-md border border-gray-100 px-3 py-2 dark:border-dark-700">
          <div class="text-xs text-gray-500 dark:text-dark-400">将创建</div>
          <div class="mt-1 text-lg font-semibold text-emerald-700 dark:text-emerald-300">{{ ensureKeysPlan.to_create }}</div>
        </div>
        <div class="rounded-md border border-gray-100 px-3 py-2 dark:border-dark-700">
          <div class="text-xs text-gray-500 dark:text-dark-400">将复用</div>
          <div class="mt-1 text-lg font-semibold text-blue-700 dark:text-blue-300">{{ ensureKeysPlan.to_reuse || 0 }}</div>
        </div>
        <div class="rounded-md border border-gray-100 px-3 py-2 dark:border-dark-700">
          <div class="text-xs text-gray-500 dark:text-dark-400">已覆盖</div>
          <div class="mt-1 text-lg font-semibold text-gray-900 dark:text-gray-100">{{ ensureKeysPlan.already_satisfied }}</div>
        </div>
        <div class="rounded-md border border-gray-100 px-3 py-2 dark:border-dark-700">
          <div class="text-xs text-gray-500 dark:text-dark-400">被阻塞</div>
          <div class="mt-1 text-lg font-semibold" :class="ensureKeysPlan.blocked > 0 ? 'text-red-700 dark:text-red-300' : 'text-gray-900 dark:text-gray-100'">{{ ensureKeysPlan.blocked }}</div>
        </div>
      </div>
      <div v-if="ensureKeysPlan.blocked > 0" class="mt-4 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-900 dark:border-amber-900/60 dark:bg-amber-950/20 dark:text-amber-100">
        当前计划包含阻塞分组。你可以只提交可处理部分；被阻塞分组需要调整供应商 Key 配额策略、删除无用 Key 或改为手动处理。
      </div>
      <div v-if="blockedPlanItems.length > 0" class="mt-4 rounded-lg border border-gray-200 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-900/40">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div>
            <h4 class="text-sm font-semibold text-gray-900 dark:text-gray-100">阻塞分组修复</h4>
            <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
              先处理配额策略、释放本地配额投影或改为手动处理，再重新生成开通计划。
            </p>
          </div>
          <button
            v-if="groupsSupplier"
            type="button"
            class="btn btn-secondary btn-sm"
            @click="openKeyLimitSettings"
          >
            <Icon name="edit" size="xs" />
            调整供应商配额
          </button>
          <button
            v-if="providerKeyUnboundPlanItems.length > 1"
            type="button"
            class="btn btn-secondary btn-sm"
            :disabled="providerKeyBatchImporting"
            @click="importCurrentProviderKeyProjections"
          >
            <Icon name="download" size="xs" :class="{ 'animate-spin': providerKeyBatchImporting }" />
            批量导入 {{ providerKeyUnboundPlanItems.length }} 个
          </button>
        </div>
        <div class="mt-3 grid gap-2 lg:grid-cols-2">
          <div
            v-for="item in blockedPlanItems.slice(0, 8)"
            :key="item.supplier_group_id"
            class="rounded-md border border-gray-200 bg-white p-3 text-sm dark:border-dark-700 dark:bg-dark-800"
          >
            <div class="flex min-w-0 items-start justify-between gap-2">
              <div class="min-w-0">
                <div class="truncate font-medium text-gray-900 dark:text-gray-100" :title="item.group_name">{{ item.group_name }}</div>
                <div class="mt-1 flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-dark-400">
                  <span class="font-mono">#{{ item.external_group_id }}</span>
                  <span>{{ formatMultiplier(item.effective_rate_multiplier || item.rate_multiplier) }}</span>
                  <span>{{ ensurePlanReasonLabel(item.blocked_reason, item.priority) }}</span>
                  <span v-if="item.provider_external_key_id" class="font-mono">Key #{{ item.provider_external_key_id }}</span>
                  <span v-if="item.provider_key_name" class="truncate" :title="item.provider_key_name">{{ item.provider_key_name }}</span>
                </div>
              </div>
              <span class="badge badge-danger shrink-0">阻塞</span>
            </div>
            <p class="mt-2 text-xs text-gray-600 dark:text-dark-300">
              {{ blockedPlanAdvice(item.blocked_reason) }}
            </p>
            <div class="mt-3 flex flex-wrap gap-2">
              <button
                v-if="item.blocked_reason === 'key_capacity_exhausted'"
                type="button"
                class="btn btn-secondary btn-sm"
                @click="openKeyLimitSettings"
              >
                提高上限
              </button>
              <button
                v-if="item.blocked_reason === 'group_key_capacity_exhausted' || item.blocked_reason === 'group_key_capacity_unknown' || item.blocked_reason === 'group_key_provisioning_unsupported'"
                type="button"
                class="btn btn-secondary btn-sm"
                @click="openGroupKeyCapacitySettingsByID(item.supplier_group_id)"
              >
                调整分组配额
              </button>
              <button
                type="button"
                class="btn btn-primary btn-sm"
                @click="focusBlockedGroup(item)"
              >
                定位分组
              </button>
              <button
                v-if="item.blocked_reason === 'key_provisioning_unsupported'"
                type="button"
                class="btn btn-secondary btn-sm"
                @click="openKeyLimitSettings"
              >
                切换策略
              </button>
              <button
                v-if="item.blocked_reason === 'provider_key_exists_unbound'"
                type="button"
                class="btn btn-secondary btn-sm"
                :disabled="providerKeyBatchImporting || providerKeyImportingGroupID === item.supplier_group_id"
                @click="importCurrentProviderKeyProjection(item)"
              >
                <Icon name="download" size="xs" :class="{ 'animate-spin': providerKeyImportingGroupID === item.supplier_group_id }" />
                导入投影
              </button>
              <button
                v-if="item.blocked_reason === 'key_capacity_exhausted'"
                type="button"
                class="btn btn-secondary btn-sm"
                :disabled="releasableSupplierKeys.length === 0"
                title="仅释放 SuperLLM 本地投影，不会删除第三方供应商后台 Key"
                @click="scrollToReleasableKeys"
              >
                可释放 Key
              </button>
            </div>
          </div>
        </div>
        <div
          v-if="hasCapacityBlockedItems"
          ref="releasableKeySection"
          class="mt-4 rounded-md border border-blue-200 bg-blue-50 p-3 dark:border-blue-900/50 dark:bg-blue-950/20"
        >
          <div class="flex flex-wrap items-start justify-between gap-2">
            <div>
              <h5 class="text-sm font-semibold text-blue-950 dark:text-blue-100">可释放的本地 Key 投影</h5>
              <p class="mt-1 text-xs text-blue-800 dark:text-blue-200">
                只把 SuperLLM 中的 Key 投影标记为停用，用于释放本地配额计算；不会删除第三方后台 Key，也不会自动变更本地账号调度。若第三方后台仍占用真实上限，后续创建仍可能被供应商拒绝。
              </p>
            </div>
            <span class="badge badge-primary">{{ releasableSupplierKeys.length }} 个</span>
          </div>
          <div v-if="releasableSupplierKeys.length > 0" class="mt-3 grid gap-2 lg:grid-cols-2">
            <div
              v-for="key in releasableSupplierKeys.slice(0, 8)"
              :key="key.id"
              class="flex min-w-0 items-center justify-between gap-3 rounded-md border border-blue-100 bg-white px-3 py-2 text-sm dark:border-blue-900/50 dark:bg-dark-800"
            >
              <div class="min-w-0">
                <div class="truncate font-medium text-gray-900 dark:text-gray-100" :title="key.name">{{ key.name || '-' }}</div>
                <div class="mt-1 flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-dark-400">
                  <span>{{ supplierKeyStatusLabel(key.status) }}</span>
                  <span>{{ keyGroupName(key) }}</span>
                  <span v-if="key.key_last4" class="font-mono">****{{ key.key_last4 }}</span>
                  <span v-if="key.local_sub2api_account_id">本地 #{{ key.local_sub2api_account_id }}</span>
                </div>
              </div>
              <div class="flex shrink-0 flex-wrap justify-end gap-2">
                <button
                  type="button"
                  class="btn btn-secondary btn-sm"
                  :disabled="keyProjectionDisabling === key.id"
                  title="只释放 SuperLLM 本地投影，不调用第三方后台"
                  @click="disableCurrentKeyLocalProjection(key)"
                >
                  <Icon name="x" size="xs" :class="{ 'animate-spin': keyProjectionDisabling === key.id }" />
                  本地释放
                </button>
                <button
                  type="button"
                  class="btn btn-warning btn-sm"
                  :disabled="keyProjectionDisabling === key.id || !key.external_key_id"
                  title="调用第三方供应商后台停用 Key"
                  @click="disableCurrentProviderKey(key)"
                >
                  <Icon name="xCircle" size="xs" :class="{ 'animate-spin': keyProjectionDisabling === key.id }" />
                  第三方停用
                </button>
                <button
                  type="button"
                  class="btn btn-danger btn-sm"
                  :disabled="keyProjectionDisabling === key.id || !key.external_key_id"
                  title="调用第三方供应商后台删除 Key"
                  @click="deleteCurrentProviderKey(key)"
                >
                  <Icon name="trash" size="xs" :class="{ 'animate-spin': keyProjectionDisabling === key.id }" />
                  第三方删除
                </button>
              </div>
            </div>
          </div>
          <div v-else class="mt-3 text-xs text-blue-800 dark:text-blue-200">
            当前没有可释放的本地 Key 投影。需要提高配额、在第三方后台人工删除/停用 Key 后同步，或走手动 Key 流程。
          </div>
        </div>
        <div v-if="blockedPlanItems.length > 8" class="mt-2 text-xs text-gray-500 dark:text-dark-400">
          还有 {{ blockedPlanItems.length - 8 }} 个阻塞分组未展示，可通过下方计划明细查看。
        </div>
      </div>
      <div class="mt-4 overflow-x-auto">
        <table class="min-w-full text-left text-sm">
          <thead class="text-xs uppercase text-gray-500 dark:text-dark-400">
            <tr>
              <th class="px-2 py-2 font-medium">分组</th>
              <th class="px-2 py-2 font-medium">倍率</th>
              <th class="px-2 py-2 font-medium">优先级</th>
              <th class="px-2 py-2 font-medium">动作</th>
              <th class="px-2 py-2 font-medium">原因</th>
              <th class="px-2 py-2 font-medium">调整</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
            <tr v-for="item in ensureKeysPlan.items.slice(0, 12)" :key="item.supplier_group_id">
              <td class="px-2 py-2">
                <div class="max-w-[220px] truncate font-medium text-gray-900 dark:text-gray-100" :title="item.group_name">{{ item.group_name }}</div>
                <div class="font-mono text-xs text-gray-500 dark:text-dark-400">#{{ item.external_group_id }}</div>
              </td>
              <td class="px-2 py-2">{{ formatMultiplier(item.effective_rate_multiplier || item.rate_multiplier) }}</td>
              <td class="px-2 py-2">
                <span v-if="isEnsurePlanPrioritizable(item)" class="badge badge-primary">#{{ priorityPlanIndex(item) + 1 }}</span>
                <span v-else class="text-xs text-gray-400">-</span>
              </td>
              <td class="px-2 py-2">
                <span class="badge" :class="ensurePlanActionClass(item.action)">{{ ensurePlanActionLabel(item.action) }}</span>
              </td>
              <td class="px-2 py-2 text-gray-600 dark:text-dark-300">
                {{ ensurePlanReasonLabel(item.blocked_reason, item.priority) }}
              </td>
              <td class="px-2 py-2">
                <div v-if="isEnsurePlanPrioritizable(item)" class="flex items-center gap-1">
                  <button
                    type="button"
                    class="btn btn-secondary btn-sm h-8 w-8 p-0"
                    :disabled="ensureKeysPlanning || priorityPlanIndex(item) <= 0"
                    title="提高优先级"
                    @click="moveEnsureKeyPriority(item, 'up')"
                  >
                    <Icon name="arrowUp" size="xs" />
                  </button>
                  <button
                    type="button"
                    class="btn btn-secondary btn-sm h-8 w-8 p-0"
                    :disabled="ensureKeysPlanning || priorityPlanIndex(item) >= priorityPlanItems.length - 1"
                    title="降低优先级"
                    @click="moveEnsureKeyPriority(item, 'down')"
                  >
                    <Icon name="arrowDown" size="xs" />
                  </button>
                </div>
                <span v-else class="text-xs text-gray-400">-</span>
              </td>
            </tr>
          </tbody>
        </table>
        <div v-if="ensureKeysPlan.items.length > 12" class="mt-2 text-xs text-gray-500 dark:text-dark-400">
          还有 {{ ensureKeysPlan.items.length - 12 }} 个分组未展示
        </div>
      </div>
    </div>

    <div v-if="supplierGroupEvents.length > 0" class="rounded-lg border border-amber-200 bg-amber-50 p-4 dark:border-amber-900/60 dark:bg-amber-950/20">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <h3 class="text-sm font-semibold text-amber-950 dark:text-amber-100">最近分组变更</h3>
        <span class="text-xs text-amber-700 dark:text-amber-300">同步分组时自动记录</span>
      </div>
      <div class="mt-3 grid gap-2 md:grid-cols-2 xl:grid-cols-3">
        <div v-for="event in supplierGroupEvents" :key="event.id" class="rounded-md border border-amber-100 bg-white px-3 py-2 text-sm dark:border-amber-900/50 dark:bg-dark-900">
          <div class="flex min-w-0 items-center justify-between gap-2">
            <div class="flex min-w-0 items-center gap-2">
              <span class="badge shrink-0" :class="groupChangeClass(event.direction)">
                {{ groupChangeDirectionLabel(event.direction) }}
              </span>
              <span class="truncate font-medium text-gray-900 dark:text-gray-100" :title="event.group_name">
                {{ event.group_name }}
              </span>
            </div>
            <span v-if="event.low_rate" class="badge badge-success shrink-0">超低价</span>
          </div>
          <div class="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-gray-500 dark:text-dark-400">
            <span class="font-mono">#{{ event.external_group_id }}</span>
            <span>{{ event.provider_family || 'mixed' }}</span>
            <span>{{ formatGroupChangeRate(event) }}</span>
          </div>
          <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">
            {{ formatDateTime(event.created_at) }}
          </div>
        </div>
      </div>
    </div>

    <div v-if="groupsError" class="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-200">
      {{ groupsError }}
    </div>

    <div v-if="provisionJobError" class="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-200">
      {{ provisionJobError }}
    </div>

    <div v-if="ensureKeysPlanError" class="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-200">
      {{ ensureKeysPlanError }}
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

      <template #cell-key_capacity="{ row }">
        <div class="w-[118px] space-y-1 text-xs text-gray-600 dark:text-dark-300">
          <div class="flex flex-wrap items-center gap-1">
            <span class="badge" :class="groupKeyCapacityClass(row.key_capacity_status)">{{ groupKeyCapacityStatusLabel(row.key_capacity_status) }}</span>
            <span>{{ groupKeyPolicyLabel(row.key_limit_policy) }}</span>
          </div>
          <div>
            已用 {{ row.active_key_count || 0 }}{{ row.key_limit_policy === 'limited' ? `/${row.key_limit_value || 0}` : '' }}
          </div>
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
            title="配置该第三方分组的 Key 创建限制"
            @click="openGroupKeyCapacitySettings(row)"
          >
            <Icon name="cog" size="sm" />
            配额
          </button>
          <button
            type="button"
            class="btn btn-secondary btn-sm h-8 px-2"
            :disabled="!groupHasLocalBinding(row)"
            title="选择模型并真实复测该渠道，成功后刷新首 Token 和总耗时"
            @click="openGroupChannelProbeDialog(row)"
          >
            <Icon name="beaker" size="sm" />
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

<BaseDialog :show="keyCapacityDialogOpen" :title="keyCapacityGroup ? `分组配额 - ${keyCapacityGroup.name}` : '分组配额'" width="normal" :z-index="60" @close="closeGroupKeyCapacityDialog">
  <div class="space-y-4">
    <div v-if="keyCapacityGroup" class="rounded-md border border-gray-200 bg-gray-50 p-3 text-sm dark:border-dark-700 dark:bg-dark-900/40">
      <div class="font-medium text-gray-900 dark:text-gray-100">{{ keyCapacityGroup.name }}</div>
      <div class="mt-1 flex flex-wrap gap-2 text-xs text-gray-500 dark:text-dark-400">
        <span class="font-mono">#{{ keyCapacityGroup.external_group_id }}</span>
        <span>{{ formatMultiplier(keyCapacityGroup.effective_rate_multiplier || keyCapacityGroup.rate_multiplier) }}</span>
        <span>当前已用 {{ keyCapacityGroup.active_key_count || 0 }}</span>
      </div>
    </div>
    <label class="block">
      <span class="input-label">Key 配额策略</span>
      <select v-model="keyCapacityForm.key_limit_policy" class="input">
        <option value="inherit">继承供应商策略</option>
        <option value="unknown">未知，阻止自动创建</option>
        <option value="unlimited">不限分组上限</option>
        <option value="limited">有限分组上限</option>
        <option value="unsupported">该分组不支持自动创建</option>
      </select>
    </label>
    <label class="block">
      <span class="input-label">分组 Key 上限</span>
      <input
        v-model.number="keyCapacityForm.key_limit_value"
        type="number"
        min="0"
        step="1"
        class="input"
        :disabled="keyCapacityForm.key_limit_policy !== 'limited'"
      />
    </label>
    <div class="rounded-md border border-blue-200 bg-blue-50 px-3 py-2 text-xs text-blue-900 dark:border-blue-900/60 dark:bg-blue-950/20 dark:text-blue-100">
      继承表示只使用供应商级配额；有限表示该分组也有独立上限。未知和不支持会阻止自动开通，避免创建失败或误判渠道不可用。
    </div>
  </div>

  <template #footer>
    <button type="button" class="btn btn-secondary" :disabled="keyCapacitySubmitting" @click="closeGroupKeyCapacityDialog">取消</button>
    <button type="button" class="btn btn-primary" :disabled="keyCapacitySubmitting || !keyCapacityFormValid" @click="submitGroupKeyCapacity">
      <Icon name="check" size="sm" :class="{ 'animate-spin': keyCapacitySubmitting }" />
      保存
    </button>
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
import { computed, reactive, ref } from 'vue'
import { updateSupplierGroupKeyCapacity } from '@/api/admin/adminPlus'
import type { SupplierGroup, SupplierGroupKeyLimitPolicy, SupplierKey } from '@/api/admin/adminPlus'
const props = defineProps<{ vm: any }>()
const {
  appStore,
  groupsDialogOpen,
  groupsSupplier,
  supplierGroups,
  supplierKeys,
  supplierGroupEvents,
  activeProvisionJob,
  ensureKeysPlan,
  ensureKeysPriorityGroupIDs,
  groupsLoading,
  groupsSyncing,
  keysEnsuring,
  ensureKeysPlanning,
  keyNamesStandardizing,
  keyProjectionDisabling,
  providerKeyImportingGroupID,
  providerKeyBatchImporting,
  channelChecksSyncing,
  groupsError,
  provisionJobError,
  ensureKeysPlanError,
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
  openGroupChannelProbeDialog,
  handleGroupScheduleAction,
  openProvisionDialog,
  openRepairDialog,
  syncCurrentGroups,
  previewEnsureCurrentKeys,
  ensureKeysPlanCanSubmit,
  ensureKeysPriorityItems,
  moveEnsureKeyPriority,
  resetEnsureKeyPriority,
  ensureCurrentKeys,
  standardizeCurrentKeyNames,
  disableCurrentKeyLocalProjection,
  disableCurrentProviderKey,
  deleteCurrentProviderKey,
  importCurrentProviderKeyProjection,
  importCurrentProviderKeyProjections,
  openEditDialog
} = props.vm

const releasableKeySection = ref<HTMLElement | null>(null)
const keyCapacityDialogOpen = ref(false)
const keyCapacitySubmitting = ref(false)
const keyCapacityGroup = ref<SupplierGroup | null>(null)
const keyCapacityForm = reactive<{ key_limit_policy: SupplierGroupKeyLimitPolicy; key_limit_value: number }>({
  key_limit_policy: 'inherit',
  key_limit_value: 0
})

const blockedPlanItems = computed(() => {
  return rawValue(ensureKeysPlan)?.items?.filter((item: EnsurePlanItem) => item.action === 'blocked') || []
})

const hasCapacityBlockedItems = computed(() => {
  return blockedPlanItems.value.some((item: EnsurePlanItem) => item.blocked_reason === 'key_capacity_exhausted' || item.blocked_reason === 'group_key_capacity_exhausted')
})

const providerKeyUnboundPlanItems = computed(() => {
  return blockedPlanItems.value.filter((item: EnsurePlanItem) => item.blocked_reason === 'provider_key_exists_unbound' && item.provider_external_key_id)
})

const priorityPlanItems = computed<EnsurePlanItem[]>(() => ensureKeysPriorityItems())

const releasableSupplierKeys = computed<SupplierKey[]>(() => {
  return [...(rawValue(supplierKeys) || [])]
    .filter((key: SupplierKey) => isBlockingSupplierKeyStatus(key.status))
    .sort((left: SupplierKey, right: SupplierKey) => {
      const leftGroup = keyGroupRate(left)
      const rightGroup = keyGroupRate(right)
      if (leftGroup !== rightGroup) return rightGroup - leftGroup
      return right.id - left.id
    })
})

function rawValue<T>(value: T | { value: T }): T {
  if (value && typeof value === 'object' && 'value' in value) {
    return (value as { value: T }).value
  }
  return value as T
}

interface EnsurePlanItem {
  supplier_group_id: number
  external_group_id: string
  group_name: string
  rate_multiplier?: number
  effective_rate_multiplier?: number
  action: string
  priority?: number
  provider_external_key_id?: string
  provider_key_name?: string
  provider_key_status?: string
  group_key_limit_policy?: string
  group_key_limit_value?: number
  group_active_key_count?: number
  group_remaining_key_slots?: number
  blocked_reason?: string
}

function isEnsurePlanPrioritizable(item: EnsurePlanItem): boolean {
  return item.action === 'create' || item.blocked_reason === 'key_capacity_exhausted' || item.blocked_reason === 'group_key_capacity_exhausted'
}

function priorityPlanIndex(item: EnsurePlanItem): number {
  return priorityPlanItems.value.findIndex((candidate) => candidate.supplier_group_id === item.supplier_group_id)
}

type GroupChangeDirection = 'new' | 'increase' | 'decrease'
interface GroupChangeEvent {
  direction: GroupChangeDirection
  old_effective_rate_multiplier?: number | null
  new_effective_rate_multiplier: number
  change_percent?: number | null
}

function groupChangeDirectionLabel(direction: GroupChangeDirection): string {
  if (direction === 'new') return '新增'
  if (direction === 'increase') return '上调'
  return '下调'
}

function groupChangeClass(direction: GroupChangeDirection): string {
  if (direction === 'new') return 'badge-primary'
  if (direction === 'increase') return 'badge-warning'
  return 'badge-success'
}

function formatGroupChangeRate(event: GroupChangeEvent): string {
  const next = formatMultiplier(event.new_effective_rate_multiplier)
  if (event.old_effective_rate_multiplier == null) {
    return `当前 ${next}`
  }
  const previous = formatMultiplier(event.old_effective_rate_multiplier)
  const change = typeof event.change_percent === 'number'
    ? ` (${event.change_percent > 0 ? '+' : ''}${event.change_percent.toFixed(1)}%)`
    : ''
  return `${previous} -> ${next}${change}`
}

function ensurePlanPolicyLabel(value?: string): string {
  return {
    unknown: '未知',
    unlimited: '不限',
    limited: '有限',
    unsupported: '不支持'
  }[value || ''] || value || '-'
}

function groupKeyPolicyLabel(value?: string): string {
  return {
    inherit: '继承',
    unknown: '未知',
    unlimited: '不限',
    limited: '有限',
    unsupported: '不支持'
  }[value || 'inherit'] || value || '继承'
}

function groupKeyCapacityStatusLabel(value?: string): string {
  return {
    inherit: '继承',
    available: '可创建',
    limited: '接近上限',
    exhausted: '已满',
    unknown: '未知',
    unsupported: '不支持'
  }[value || 'inherit'] || value || '-'
}

function groupKeyCapacityClass(value?: string): string {
  if (value === 'available' || value === 'inherit') return 'badge-success'
  if (value === 'limited') return 'badge-warning'
  if (value === 'exhausted' || value === 'unsupported') return 'badge-danger'
  return 'badge-gray'
}

function ensurePlanActionLabel(value?: string): string {
  return {
    create: '创建',
    reuse: '复用',
    skipped_existing: '检查绑定',
    blocked: '阻塞'
  }[value || ''] || value || '-'
}

function ensurePlanActionClass(value?: string): string {
  if (value === 'create') return 'badge-success'
  if (value === 'reuse') return 'badge-primary'
  if (value === 'blocked') return 'badge-danger'
  return 'badge-gray'
}

function ensurePlanReasonLabel(reason?: string, priority?: number): string {
  if (reason === 'key_capacity_exhausted') return 'Key 配额已满'
  if (reason === 'group_key_capacity_exhausted') return '分组 Key 配额已满'
  if (reason === 'group_key_capacity_unknown') return '分组 Key 配额未知'
  if (reason === 'group_key_provisioning_unsupported') return '分组不支持自动开通'
  if (reason === 'key_capacity_unknown') return 'Key 配额未知'
  if (reason === 'key_provisioning_unsupported') return '不支持自动开通'
  if (reason === 'provider_key_exists_unbound') return '第三方已有未绑定 Key'
  if (reason === 'provider_key_capacity_incomplete') return '第三方 Key 列表未读完整'
  if (priority) return `优先级 ${priority}`
  return '-'
}

function blockedPlanAdvice(reason?: string): string {
  if (reason === 'key_capacity_exhausted') {
    return '当前供应商 Key 上限已满。先提高上限、释放无用 Key，或只提交计划中的可创建部分。'
  }
  if (reason === 'group_key_capacity_exhausted') {
    return '当前第三方分组 Key 上限已满。提高该分组上限、释放该分组无用 Key，或只提交其他可创建分组。'
  }
  if (reason === 'group_key_capacity_unknown') {
    return '当前第三方分组 Key 配额未知。先配置该分组上限、改为继承供应商策略，或标记为不限后再重新生成计划。'
  }
  if (reason === 'group_key_provisioning_unsupported') {
    return '该第三方分组被标记为不支持自动创建 Key。需要切换分组策略，或走手动 Key 录入和绑定。'
  }
  if (reason === 'key_capacity_unknown') {
    return '当前供应商 Key 配额未知。先配置上限或标记为无限制，再重新生成开通计划。'
  }
  if (reason === 'key_provisioning_unsupported') {
    return '该供应商标记为不支持自动创建 Key。需要切换策略，或后续接入手动 Key 录入流程。'
  }
  if (reason === 'provider_key_exists_unbound') {
    return '第三方后台已存在该分组的有效 Key，但 SuperLLM 没有绑定记录。先导入或修复绑定；不再使用时先在第三方停用/删除并重新同步。'
  }
  if (reason === 'provider_key_capacity_incomplete') {
    return '第三方 Key 列表读取不完整，不能安全判断真实占用。请重新同步，检查注册用户会话权限或稍后重试后再创建。'
  }
  return '请先处理阻塞原因，再重新生成开通计划。'
}

function isBlockingSupplierKeyStatus(status?: string): boolean {
  return status === 'provisioning' || status === 'bound' || status === 'manual_secret_required'
}

function keyGroupName(key: SupplierKey): string {
  const group = (rawValue(supplierGroups) || []).find((item: any) => item.id === key.supplier_group_id)
  return group?.name || key.external_group_id || `分组 #${key.supplier_group_id}`
}

function keyGroupRate(key: SupplierKey): number {
  const group = (rawValue(supplierGroups) || []).find((item: any) => item.id === key.supplier_group_id)
  return Number(group?.effective_rate_multiplier || group?.rate_multiplier || 0)
}

function scrollToReleasableKeys() {
  releasableKeySection.value?.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
}

function openKeyLimitSettings() {
  const supplier = groupsSupplier?.value ?? groupsSupplier
  if (!supplier) return
  closeGroupsDialog()
  openEditDialog(supplier)
}

const keyCapacityFormValid = computed(() => {
  if (keyCapacityForm.key_limit_policy !== 'limited') return true
  return Number.isFinite(Number(keyCapacityForm.key_limit_value)) && Number(keyCapacityForm.key_limit_value) > 0
})

function openGroupKeyCapacitySettings(group: SupplierGroup) {
  keyCapacityGroup.value = group
  keyCapacityForm.key_limit_policy = normalizeGroupKeyPolicy(group.key_limit_policy)
  keyCapacityForm.key_limit_value = Number(group.key_limit_value || 0)
  keyCapacityDialogOpen.value = true
}

function openGroupKeyCapacitySettingsByID(groupID: number) {
  const group = (rawValue(supplierGroups) || []).find((item: SupplierGroup) => item.id === groupID)
  if (group) {
    openGroupKeyCapacitySettings(group)
    return
  }
  focusBlockedGroup({
    supplier_group_id: groupID,
    external_group_id: '',
    group_name: '',
    action: 'blocked'
  })
}

function closeGroupKeyCapacityDialog() {
  if (keyCapacitySubmitting.value) return
  keyCapacityDialogOpen.value = false
  keyCapacityGroup.value = null
}

function normalizeGroupKeyPolicy(value?: string): SupplierGroupKeyLimitPolicy {
  if (value === 'unknown' || value === 'unlimited' || value === 'limited' || value === 'unsupported') return value
  return 'inherit'
}

async function submitGroupKeyCapacity() {
  const supplier = groupsSupplier?.value ?? groupsSupplier
  if (!supplier || !keyCapacityGroup.value || !keyCapacityFormValid.value) return
  keyCapacitySubmitting.value = true
  try {
    await updateSupplierGroupKeyCapacity(supplier.id, keyCapacityGroup.value.id, {
      key_limit_policy: keyCapacityForm.key_limit_policy,
      key_limit_value: keyCapacityForm.key_limit_policy === 'limited' ? Number(keyCapacityForm.key_limit_value || 0) : 0
    })
    appStore?.showSuccess?.('分组 Key 配额已更新')
    keyCapacityDialogOpen.value = false
    keyCapacityGroup.value = null
    await loadCurrentGroups()
    if (rawValue(ensureKeysPlan)) {
      await previewEnsureCurrentKeys()
    }
  } catch (error) {
    const message = (error as { message?: string }).message || '更新分组 Key 配额失败'
    appStore?.showError?.(message)
  } finally {
    keyCapacitySubmitting.value = false
  }
}

function focusBlockedGroup(item: EnsurePlanItem) {
  groupFilters.q = item.external_group_id || item.group_name || String(item.supplier_group_id)
  groupPagination.page = 1
  void loadCurrentGroups()
}
</script>
