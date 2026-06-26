<template>
<TablePageLayout>
  <template #filters>
    <div class="flex flex-wrap-reverse items-start justify-between gap-3">
      <div class="grid flex-1 gap-3 lg:grid-cols-[minmax(220px,1fr)_150px_160px_160px_160px_160px]">
        <label class="block">
          <span class="input-label">搜索</span>
          <div class="relative">
            <Icon name="search" size="sm" class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
            <input v-model.trim="filters.q" class="input pl-9" placeholder="供应商名称、联系人、备注" />
          </div>
        </label>
        <label class="block">
          <span class="input-label">有效倍率渠道</span>
          <select v-model="channelProtocolFilter" class="input">
            <option value="openai">OpenAI</option>
            <option value="claude">Claude</option>
            <option value="gemini">Gemini</option>
            <option value="">全部协议</option>
          </select>
        </label>
        <label class="block">
          <span class="input-label">供应商归类</span>
          <select v-model="filters.kind" class="input">
            <option value="">全部</option>
            <option value="relay">下游中转</option>
            <option value="source_account">源站账号归类</option>
            <option value="browser_only">仅浏览器采集</option>
            <option value="custom">自定义</option>
          </select>
        </label>
        <label class="block">
          <span class="input-label">系统类型</span>
          <select v-model="filters.type" class="input">
            <option value="">全部</option>
            <option value="sub2api">Sub2API</option>
            <option value="new_api">New API</option>
            <option value="openai">OpenAI</option>
            <option value="anthropic">Anthropic</option>
            <option value="gemini">Gemini</option>
            <option value="browser_only">仅浏览器</option>
            <option value="custom">自定义</option>
          </select>
        </label>
        <label class="block">
          <span class="input-label">运行状态</span>
          <select v-model="filters.runtime_status" class="input">
            <option value="">全部</option>
            <option value="monitor_only">仅监控</option>
            <option value="candidate">候选</option>
            <option value="active">当前使用</option>
            <option value="disabled">停用</option>
          </select>
        </label>
        <label class="block">
          <span class="input-label">健康状态</span>
          <select v-model="filters.health_status" class="input">
            <option value="">全部</option>
            <option value="normal">正常</option>
            <option value="unavailable">不可用</option>
            <option value="credential_invalid">凭据失效</option>
            <option value="paused">暂停</option>
          </select>
        </label>
      </div>

      <div class="flex flex-wrap items-center gap-2">
        <button type="button" class="btn btn-secondary px-2 md:px-3" :disabled="loading" title="刷新" @click="loadSuppliers">
          <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
          <span class="hidden md:inline">刷新</span>
        </button>
        <button type="button" class="btn btn-secondary px-2 md:px-3" :disabled="scheduleListLoading" title="查看和操作供应商本地调度账号" @click="openScheduleListDialog">
          <Icon name="server" size="sm" :class="{ 'animate-spin': scheduleListLoading }" />
          <span class="hidden md:inline">调度列表</span>
        </button>
        <div class="relative">
          <button type="button" class="btn btn-secondary px-2 md:px-3" title="更多操作" @click="moreMenuOpen = !moreMenuOpen">
            <Icon name="more" size="sm" class="md:mr-1.5" />
            <span class="hidden md:inline">更多操作</span>
            <Icon name="chevronDown" size="xs" class="ml-1 hidden md:inline" />
          </button>
          <div
            v-if="moreMenuOpen"
            class="absolute right-0 z-50 mt-2 w-[min(20rem,calc(100vw-2rem))] overflow-hidden rounded-lg border border-gray-200 bg-white shadow-xl dark:border-gray-700 dark:bg-gray-800"
          >
            <div class="p-2">
              <div class="px-2 py-2 text-xs font-semibold uppercase tracking-wide text-gray-400 dark:text-gray-500">批量操作</div>
              <button class="menu-item" :disabled="selectedCount === 0" @click="openBulkStatusDialog">
                <span class="menu-icon bg-blue-50 text-blue-600 dark:bg-blue-900/30 dark:text-blue-300">
                  <Icon name="edit" size="sm" />
                </span>
                <span>批量调整状态</span>
              </button>
              <button class="menu-item" :disabled="selectedCount === 0 || bulkBalanceRefreshing" @click="refreshSelectedBalances">
                <span class="menu-icon bg-emerald-50 text-emerald-600 dark:bg-emerald-900/30 dark:text-emerald-300">
                  <Icon name="sync" size="sm" :class="{ 'animate-spin': bulkBalanceRefreshing }" />
                </span>
                <span>{{ bulkBalanceRefreshing ? '更新额度中...' : '批量更新额度' }}</span>
              </button>
              <button class="menu-item" :disabled="selectedCount === 0 || bulkChannelChecksSyncing" @click="syncSelectedChannelChecks">
                <span class="menu-icon bg-violet-50 text-violet-600 dark:bg-violet-900/30 dark:text-violet-300">
                  <Icon name="beaker" size="sm" :class="{ 'animate-spin': bulkChannelChecksSyncing }" />
                </span>
                <span>{{ bulkChannelChecksSyncing ? '提交检测中...' : '批量检测渠道' }}</span>
              </button>
              <button class="menu-item text-red-600 dark:text-red-300" :disabled="selectedCount === 0" @click="openBulkDeleteDialog">
                <span class="menu-icon bg-red-50 text-red-600 dark:bg-red-900/30 dark:text-red-300">
                  <Icon name="trash" size="sm" />
                </span>
                <span>批量删除供应商</span>
              </button>
              <div class="my-2 border-t border-gray-100 dark:border-gray-700"></div>
              <button class="menu-item" @click="resetFilters">
                <span class="menu-icon bg-slate-100 text-slate-600 dark:bg-slate-700 dark:text-slate-200">
                  <Icon name="x" size="sm" />
                </span>
                <span>清除筛选</span>
              </button>
            </div>
          </div>
        </div>
        <button type="button" class="btn btn-primary" @click="openCreateDialog">
          <Icon name="plus" size="sm" />
          添加供应商
        </button>
      </div>
    </div>
  </template>

  <template #table>
    <div
      v-if="selectedCount > 0"
      class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-100 bg-primary-50/60 px-4 py-3 text-sm dark:border-dark-700 dark:bg-primary-900/20"
    >
      <div class="text-primary-800 dark:text-primary-200">
        已选择 <span class="font-semibold">{{ selectedCount }}</span> 个供应商
      </div>
      <div class="flex flex-wrap gap-2">
        <button type="button" class="btn btn-secondary btn-sm" @click="selectVisible">全选当前页</button>
        <button type="button" class="btn btn-secondary btn-sm" @click="clearSelection">清除选择</button>
        <button type="button" class="btn btn-secondary btn-sm" @click="openBulkStatusDialog">批量状态</button>
        <button type="button" class="btn btn-secondary btn-sm" :disabled="bulkBalanceRefreshing" @click="refreshSelectedBalances">
          <Icon name="sync" size="xs" :class="{ 'animate-spin': bulkBalanceRefreshing }" />
          {{ bulkBalanceRefreshing ? '更新中' : '批量更新额度' }}
        </button>
        <button type="button" class="btn btn-secondary btn-sm" :disabled="bulkChannelChecksSyncing" @click="syncSelectedChannelChecks">
          <Icon name="beaker" size="xs" :class="{ 'animate-spin': bulkChannelChecksSyncing }" />
          {{ bulkChannelChecksSyncing ? '提交中' : '批量检测渠道' }}
        </button>
        <button type="button" class="btn btn-danger btn-sm" @click="openBulkDeleteDialog">批量删除</button>
      </div>
    </div>

    <DataTable
      :columns="columns"
      :data="filteredSuppliers"
      :loading="loading"
      row-key="id"
      default-sort-key="id"
      default-sort-order="desc"
      :estimate-row-height="76"
    >
      <template #header-select>
        <input
          type="checkbox"
          class="h-4 w-4 cursor-pointer rounded border-gray-300 text-primary-600 focus:ring-primary-500"
          :checked="allVisibleSelected"
          @click.stop
          @change="toggleSelectAllVisible($event)"
        />
      </template>

      <template #cell-select="{ row }">
        <input
          type="checkbox"
          class="h-4 w-4 cursor-pointer rounded border-gray-300 text-primary-600 focus:ring-primary-500"
          :checked="isSelected(row.id)"
          @change="toggleSelection(row.id)"
        />
      </template>

      <template #cell-name="{ row }">
        <div class="w-[210px] max-w-[210px]">
          <div class="flex min-w-0 items-center gap-2">
            <a
              v-if="supplierLinkURL(row)"
              :href="supplierLinkURL(row)"
              target="_blank"
              rel="noreferrer"
              class="flex max-w-full min-w-0 items-center font-medium text-primary-600 hover:underline dark:text-primary-400"
              :title="supplierNameTitle(row)"
            >
              <span class="truncate">{{ splitMiddleEllipsis(row.name, 24).head }}</span>
              <span v-if="splitMiddleEllipsis(row.name, 24).ellipsized" class="shrink-0">...</span>
              <span v-if="splitMiddleEllipsis(row.name, 24).ellipsized" class="shrink-0">{{ splitMiddleEllipsis(row.name, 24).tail }}</span>
            </a>
            <span v-else class="flex max-w-full min-w-0 items-center font-medium text-gray-900 dark:text-white" :title="row.name">
              <span class="truncate">{{ splitMiddleEllipsis(row.name, 24).head }}</span>
              <span v-if="splitMiddleEllipsis(row.name, 24).ellipsized" class="shrink-0">...</span>
              <span v-if="splitMiddleEllipsis(row.name, 24).ellipsized" class="shrink-0">{{ splitMiddleEllipsis(row.name, 24).tail }}</span>
            </span>
          </div>
          <div class="mt-1 flex min-w-0 flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-dark-400">
            <span class="font-mono">#{{ row.id }}</span>
            <span v-if="row.contact" class="max-w-[100px] truncate" :title="row.contact">{{ middleEllipsis(row.contact, 18) }}</span>
            <span v-if="row.notes" class="max-w-[260px] truncate" :title="row.notes">{{ row.notes }}</span>
          </div>
        </div>
      </template>

      <template #cell-kind_type="{ row }">
        <div class="flex min-w-[150px] flex-wrap gap-1">
          <span class="badge badge-gray">{{ kindLabel(row.kind) }}</span>
          <span class="badge badge-primary">{{ typeLabel(row.type) }}</span>
        </div>
      </template>

      <template #cell-status="{ row }">
        <div class="flex w-[178px] flex-col items-start gap-1.5">
          <div class="flex flex-wrap gap-1.5">
            <span class="relative inline-flex items-center">
              <select
                :value="row.runtime_status"
                class="status-quick-select"
                :class="runtimeClass(row.runtime_status)"
                :disabled="quickStatusSupplierID === row.id"
                title="快速切换运行状态"
                @change="handleQuickRuntimeStatusChange(row, $event)"
              >
                <option value="monitor_only">仅监控</option>
                <option value="candidate">候选</option>
                <option value="active">当前使用</option>
                <option value="disabled">停用</option>
              </select>
              <Icon name="chevronDown" size="xs" class="pointer-events-none absolute right-1.5 text-current opacity-60" />
            </span>
            <span class="relative inline-flex items-center">
              <select
                :value="row.health_status"
                class="status-quick-select"
                :class="healthClass(row.health_status)"
                :disabled="quickStatusSupplierID === row.id"
                title="快速切换健康状态"
                @change="handleQuickHealthStatusChange(row, $event)"
              >
                <option value="normal">正常</option>
                <option value="unavailable">不可用</option>
                <option value="credential_invalid">凭据失效</option>
                <option value="paused">暂停</option>
              </select>
              <Icon name="chevronDown" size="xs" class="pointer-events-none absolute right-1.5 text-current opacity-60" />
            </span>
          </div>
          <span class="text-xs font-medium" :class="supplierSwitchStateClass(row)">
            {{ supplierSwitchStateLabel(row) }}
          </span>
          <div class="flex flex-col items-start gap-1 border-t border-gray-100 pt-1.5 text-xs dark:border-dark-700">
            <div class="flex items-center gap-1.5">
              <span class="text-gray-500 dark:text-dark-400">会话</span>
              <span class="badge" :class="sessionBadgeClass(row.id)">{{ sessionBadgeText(row.id) }}</span>
            </div>
            <span v-if="sessionStore[row.id]?.captured_at" class="text-gray-500 dark:text-dark-400">
              {{ formatDateTime(sessionStore[row.id]?.captured_at) }}
            </span>
          </div>
        </div>
      </template>

      <template #cell-best_channel="{ row }">
        <div class="w-[280px]" :title="supplierBestChannelTooltip(row.id)">
          <template v-if="supplierBestChannelVariants(row.id).length > 0">
            <div class="space-y-1.5">
              <div
                v-for="snapshot in supplierBestChannelVariants(row.id)"
                :key="snapshot.id || `${snapshot.supplier_id}:${snapshot.supplier_group_id}:${channelProtocol(snapshot)}`"
                class="min-w-0"
              >
                <div class="flex min-w-0 items-center gap-1.5">
                  <span class="badge shrink-0" :class="channelProtocolBadgeClass(channelProtocol(snapshot))">
                    {{ channelProtocolLabel(channelProtocol(snapshot)) }}
                  </span>
                  <span class="badge shrink-0" :class="channelProbeStatusClass(snapshot.probe_status)">
                    {{ channelProbeStatusLabel(snapshot.probe_status) }}
                  </span>
                  <span class="truncate text-sm font-semibold text-gray-900 dark:text-gray-100">
                    {{ snapshot.group_name || '-' }}
                  </span>
                  <button
                    v-if="channelProtocol(snapshot) === 'openai' && !channelHasLocalBinding(snapshot)"
                    type="button"
                    class="btn btn-secondary btn-sm h-7 shrink-0 px-2"
                    :disabled="channelCheckActionKey !== ''"
                    title="开通供应商 Key 和本地账号，并绑定到本地 Lime(OpenAI) 用户组"
                    @click.stop="quickProvisionBestChannelToLime(row, snapshot)"
                  >
                    <Icon name="users" size="xs" :class="{ 'animate-spin': isLimeProvisionRunning(row.id, snapshot.supplier_group_id) }" />
                    Lime
                  </button>
                </div>
                <div class="mt-0.5 flex min-w-0 flex-wrap items-center gap-x-2 gap-y-0.5 text-xs text-gray-500 dark:text-dark-400">
                  <span :class="rateMultiplierTextClass(channelCostMultiplier(snapshot), channelProtocol(snapshot), 'compact')">{{ formatMultiplier(channelCostMultiplier(snapshot)) }}</span>
                  <span>首 {{ formatLatency(snapshot.first_token_ms) }}</span>
                  <span>总 {{ formatLatency(snapshot.duration_ms) }}</span>
                  <span :class="snapshot.local_account_schedulable ? 'text-emerald-600 dark:text-emerald-400' : 'text-amber-600 dark:text-amber-300'">
                    {{ snapshot.local_account_schedulable ? '已入调度' : '未入调度' }}
                  </span>
                </div>
              </div>
            </div>
            <div class="mt-2 flex flex-wrap gap-1.5">
              <button
                type="button"
                class="btn btn-secondary btn-sm"
                title="打开供应商分组，补齐 Key/账号或查看绑定"
                @click="openGroupsDialog(row)"
              >
                <Icon name="database" size="xs" />
                分组
              </button>
              <button
                v-if="bestChannelProbeVisible(row)"
                type="button"
                class="btn btn-secondary btn-sm"
                :disabled="rowChannelCheckSupplierID === row.id"
                title="选择模型并测试最低倍率候选渠道，成功后刷新首 token 和总耗时"
                @click="openBestChannelProbeDialog(row)"
              >
                <Icon name="beaker" size="xs" :class="{ 'animate-spin': rowChannelCheckSupplierID === row.id }" />
                {{ rowChannelCheckSupplierID === row.id ? '提交中' : '检测' }}
              </button>
              <button
                type="button"
                class="btn btn-secondary btn-sm"
                :disabled="isBestChannelSchedulingRunning(row.id)"
                :title="bestChannelActionTitle(row)"
                @click="openBestChannelScheduleDialog(row)"
              >
                <Icon :name="bestChannelActionIcon(row)" size="xs" :class="{ 'animate-spin': isBestChannelSchedulingRunning(row.id) }" />
                {{ bestChannelActionLabel(row) }}
              </button>
            </div>
          </template>
          <template v-else-if="supplierAllBestChannelVariants(row.id).length > 0">
            <div class="flex flex-col gap-1">
              <span class="badge badge-gray w-fit">无 {{ channelProtocolFilterLabel }}</span>
              <span class="text-xs text-gray-500 dark:text-dark-400">
                可切换查看 {{ supplierAvailableChannelProtocolLabels(row.id) }}
              </span>
              <div class="mt-1 flex flex-wrap gap-1.5">
                <button
                  type="button"
                  class="btn btn-secondary btn-sm"
                  title="打开供应商分组，查看其他协议渠道"
                  @click="openGroupsDialog(row)"
                >
                  <Icon name="database" size="xs" />
                  分组
                </button>
                <button
                  type="button"
                  class="btn btn-secondary btn-sm"
                  :disabled="rowChannelCheckSupplierID === row.id"
                  title="提交异步渠道检测任务"
                  @click="syncSupplierChannelFromRow(row)"
                >
                  <Icon name="beaker" size="xs" :class="{ 'animate-spin': rowChannelCheckSupplierID === row.id }" />
                  {{ rowChannelCheckSupplierID === row.id ? '提交中' : '一键检测' }}
                </button>
              </div>
            </div>
          </template>
          <template v-else>
            <div class="flex flex-col gap-1">
              <span class="badge badge-gray w-fit">未检测</span>
              <span class="text-xs text-gray-500 dark:text-dark-400">同步分组后可直接检测</span>
              <div class="mt-1 flex flex-wrap gap-1.5">
                <button
                  type="button"
                  class="btn btn-secondary btn-sm"
                  title="打开供应商分组，先补齐 Key/账号或确认绑定"
                  @click="openGroupsDialog(row)"
                >
                  <Icon name="database" size="xs" />
                  分组
                </button>
                <button
                  type="button"
                  class="btn btn-secondary btn-sm"
                  :disabled="rowChannelCheckSupplierID === row.id"
                  title="提交异步渠道检测任务"
                  @click="syncSupplierChannelFromRow(row)"
                >
                  <Icon name="beaker" size="xs" :class="{ 'animate-spin': rowChannelCheckSupplierID === row.id }" />
                  {{ rowChannelCheckSupplierID === row.id ? '提交中' : '一键检测' }}
                </button>
              </div>
            </div>
          </template>
        </div>
      </template>

      <template #cell-balance_cents="{ row }">
        <div class="min-w-[230px] text-left">
          <div class="flex items-center justify-start gap-2">
            <span class="text-base font-semibold" :class="supplierBalanceAmountClass(row)">
              {{ formatBalanceMoney(row.balance_cents, row.balance_currency || 'USD') }}
            </span>
            <span class="badge" :class="supplierBalanceBadgeClass(row)">
              {{ supplierBalanceLabel(row) }}
            </span>
            <button
              type="button"
              class="btn btn-secondary btn-sm h-7 w-7 px-0"
              :disabled="rowBalanceRefreshingID === row.id"
              title="刷新供应商余额"
              @click="refreshSupplierBalance(row)"
            >
              <Icon name="refresh" size="xs" :class="{ 'animate-spin': rowBalanceRefreshingID === row.id }" />
            </button>
          </div>
          <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">余额更新 {{ supplierBalanceUpdatedLabel(row) }}</div>
          <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">充值倍率 {{ formatMultiplier(supplierRechargeMultiplier(row.id)) }}</div>

          <template v-if="supplierCostSnapshot(row.id)">
            <div class="mt-1 space-y-0.5 text-xs">
              <div class="flex items-center gap-2 text-gray-500 dark:text-dark-400">
                <span class="w-8 shrink-0">充值</span>
                <span>{{ formatMoney(supplierRechargeTotalCents(supplierCostSnapshot(row.id)), supplierCostSnapshot(row.id)?.currency || row.balance_currency) }}</span>
              </div>
              <div class="flex items-center gap-2 text-gray-500 dark:text-dark-400">
                <span class="w-8 shrink-0">消耗</span>
                <span>{{ formatMoney(supplierDisplayUsageCents(supplierCostSnapshot(row.id)), supplierCostSnapshot(row.id)?.currency || row.balance_currency) }}</span>
              </div>
              <div class="flex items-center gap-2 text-gray-500 dark:text-dark-400">
                <span class="w-8 shrink-0">订单</span>
                <span>{{ formatMoney(supplierCostSnapshot(row.id)?.completed_funding_amount_cents || 0, supplierCostSnapshot(row.id)?.currency || row.balance_currency) }}</span>
              </div>
              <div class="flex items-center gap-2 text-gray-500 dark:text-dark-400">
                <span class="w-8 shrink-0">兑换</span>
                <span>{{ formatMoney(supplierCostSnapshot(row.id)?.entitlement_amount_cents || 0, supplierCostSnapshot(row.id)?.currency || row.balance_currency) }}</span>
              </div>
              <div class="flex items-center gap-2 text-gray-500 dark:text-dark-400">
                <span class="w-8 shrink-0">实付</span>
                <span>{{ formatMoney(supplierCostSnapshot(row.id)?.recharge_actual_payment_cents || 0, supplierCostSnapshot(row.id)?.currency || row.balance_currency) }}</span>
              </div>
              <div class="flex items-center gap-2" :class="costDeltaClass(row.id)">
                <span class="w-8 shrink-0">差异</span>
                <span>{{ costDeltaLabel(row.id) }}</span>
              </div>
            </div>
          </template>
          <template v-else>
            <div class="mt-1">
              <span class="badge badge-gray">成本未同步</span>
            </div>
          </template>
        </div>
      </template>

      <template #cell-credential="{ row }">
        <div class="min-w-[220px] space-y-1 text-xs">
          <div class="flex items-center gap-2">
            <span class="w-10 shrink-0 text-gray-500 dark:text-dark-400">方式</span>
            <div class="flex flex-wrap gap-1">
              <span v-if="row.credential.browser_login_enabled" class="badge badge-warning">Chrome</span>
              <span v-else class="badge badge-gray">未启用</span>
            </div>
          </div>
          <div v-if="row.credential.browser_login_username_configured" class="flex items-center gap-2">
            <span class="w-10 shrink-0 text-gray-500 dark:text-dark-400">账号</span>
            <span class="badge badge-gray">{{ row.credential.masked_browser_login_username || '账号' }}</span>
          </div>
          <div class="flex items-center gap-2">
            <span class="w-10 shrink-0 text-gray-500 dark:text-dark-400">凭据</span>
            <div class="flex flex-wrap gap-1">
              <span v-if="row.credential.browser_login_password_configured" class="badge badge-success">密码</span>
              <span
                v-if="needsDirectLoginCredential(row)"
                class="badge badge-danger"
                title="未配置登录账号密码或临时 Token，请先编辑供应商补齐凭据"
              >
                未配置
              </span>
              <span
                v-if="shouldShowTokenBadge(row)"
                class="badge"
                :class="credentialTokenBadgeClass(row)"
                :title="credentialTokenBadgeTitle(row)"
              >
                {{ credentialTokenBadgeText(row) }}
              </span>
              <span v-if="!hasCredential(row)" class="badge badge-gray">未配置</span>
            </div>
          </div>
          <div v-if="row.credential.postgres_configured || row.credential.redis_configured" class="flex items-center gap-2">
            <span class="w-10 shrink-0 text-gray-500 dark:text-dark-400">数据源</span>
            <div class="flex flex-wrap gap-1">
              <span v-if="row.credential.postgres_configured" class="badge badge-purple">PG</span>
              <span v-if="row.credential.redis_configured" class="badge badge-primary">Redis</span>
            </div>
          </div>
        </div>
      </template>

      <template #cell-created_at="{ row }">
        <div class="min-w-[150px] text-xs text-gray-500 dark:text-dark-400">{{ formatDateTime(row.created_at) }}</div>
      </template>

      <template #cell-actions="{ row }">
        <div class="flex min-w-[280px] justify-end gap-2">
          <button type="button" class="btn btn-secondary btn-sm" title="编辑" @click="openEditDialog(row)">
            <Icon name="edit" size="sm" />
            编辑
          </button>
          <button
            type="button"
            class="btn btn-secondary btn-sm"
            :disabled="Boolean(rowLoginSupplierID)"
            :title="oneClickLoginTitle(row)"
            @click="loginSupplierFromRow(row)"
          >
            <Icon name="login" size="sm" :class="{ 'animate-spin': rowLoginSupplierID === row.id }" />
            一键登录
          </button>
          <button
            type="button"
            class="btn btn-secondary btn-sm"
            :aria-expanded="rowActionsMenuSupplier?.id === row.id"
            aria-haspopup="menu"
            data-supplier-row-actions-trigger
            title="更多操作"
            @click.stop="toggleRowActionsMenu(row, $event)"
          >
            <Icon name="more" size="sm" />
            更多
          </button>
        </div>
      </template>

      <template #empty>
        <EmptyState
          title="暂无供应商"
          description="先添加供应商父级，优先后端直登读取余额，再同步分组并按分组开通 Key 和本地账号。"
          action-text="添加供应商"
          @action="openCreateDialog"
        />
      </template>
    </DataTable>
  </template>

  <template #pagination>
    <Pagination
      v-if="pagination.total > 0"
      :page="pagination.page"
      :total="pagination.total"
      :page-size="pagination.page_size"
      @update:page="handlePageChange"
      @update:pageSize="handlePageSizeChange"
    />
  </template>
</TablePageLayout>
</template>

<script setup lang="ts">
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'
const props = defineProps<{ vm: any }>()
const {
  loading,
  moreMenuOpen,
  bulkBalanceRefreshing,
  bulkChannelChecksSyncing,
  rowActionsMenuSupplier,
  sessionStore,
  scheduleListLoading,
  rowLoginSupplierID,
  rowChannelCheckSupplierID,
  rowBalanceRefreshingID,
  quickStatusSupplierID,
  channelCheckActionKey,
  filters,
  channelProtocolFilter,
  channelProtocolFilterLabel,
  pagination,
  columns,
  filteredSuppliers,
  selectedCount,
  allVisibleSelected,
  isSelected,
  toggleSelection,
  clearSelection,
  selectVisible,
  toggleSelectAllVisible,
  formatMoney,
  formatBalanceMoney,
  formatDateTime,
  supplierLinkURL,
  supplierNameTitle,
  formatMultiplier,
  rateMultiplierTextClass,
  formatLatency,
  kindLabel,
  typeLabel,
  supplierRechargeMultiplier,
  channelCostMultiplier,
  runtimeClass,
  healthClass,
  supplierCostSnapshot,
  supplierAllBestChannelVariants,
  supplierBestChannelVariants,
  supplierBestChannelTooltip,
  supplierAvailableChannelProtocolLabels,
  channelProtocol,
  channelProtocolLabel,
  channelProtocolBadgeClass,
  isBestChannelSchedulingRunning,
  isLimeProvisionRunning,
  channelHasLocalBinding,
  bestChannelActionLabel,
  bestChannelActionIcon,
  bestChannelActionTitle,
  costDeltaLabel,
  costDeltaClass,
  supplierBalanceLabel,
  supplierBalanceBadgeClass,
  supplierBalanceAmountClass,
  supplierBalanceUpdatedLabel,
  supplierSwitchStateLabel,
  supplierSwitchStateClass,
  channelProbeStatusLabel,
  channelProbeStatusClass,
  middleEllipsis,
  splitMiddleEllipsis,
  sessionBadgeText,
  sessionBadgeClass,
  needsDirectLoginCredential,
  shouldShowTokenBadge,
  credentialTokenBadgeText,
  credentialTokenBadgeClass,
  credentialTokenBadgeTitle,
  oneClickLoginTitle,
  hasCredential,
  loadSuppliers,
  openScheduleListDialog,
  handlePageChange,
  handlePageSizeChange,
  resetFilters,
  openCreateDialog,
  openEditDialog,
  openGroupsDialog,
  loginSupplierFromRow,
  openBestChannelProbeDialog,
  bestChannelProbeVisible,
  syncSupplierChannelFromRow,
  syncSelectedChannelChecks,
  openBestChannelScheduleDialog,
  quickProvisionBestChannelToLime,
  openBulkStatusDialog,
  refreshSelectedBalances,
  refreshSupplierBalance,
  handleQuickRuntimeStatusChange,
  handleQuickHealthStatusChange,
  toggleRowActionsMenu,
  openBulkDeleteDialog,
  supplierDisplayUsageCents,
  supplierRechargeTotalCents
} = props.vm
</script>

<style scoped>
.menu-item {
  @apply flex w-full items-center gap-3 rounded-md px-3 py-2 text-sm text-gray-700 transition-colors hover:bg-gray-100 disabled:cursor-not-allowed disabled:opacity-50 dark:text-gray-200 dark:hover:bg-gray-700;
}

.menu-icon {
  @apply flex h-8 w-8 items-center justify-center rounded-md;
}

.status-quick-select {
  @apply h-6 cursor-pointer appearance-none rounded-full border-0 py-0.5 pl-2.5 pr-5 text-xs font-medium outline-none ring-1 ring-transparent transition focus:ring-primary-400 disabled:cursor-wait disabled:opacity-60;
}

.status-quick-select option {
  @apply bg-white text-gray-900 dark:bg-dark-800 dark:text-gray-100;
}
</style>
