<template>
  <AppLayout>
    <div class="space-y-5">
      <section class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">本地账号运营</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
            以本地账号为主线查看供应商、第三方分组、Key、余额、检测和调度状态。
          </p>
        </div>
        <div class="flex flex-wrap gap-2">
          <RouterLink :to="{ path: '/admin/action-audits', query: { component: 'admin_plus.sub2api', window: '24h' } }" class="btn btn-secondary">
            <Icon name="clipboard" size="sm" />
            操作审计
          </RouterLink>
          <RouterLink to="/admin/supplier-bindings" class="btn btn-secondary">
            <Icon name="link" size="sm" />
            账号/Key 绑定
          </RouterLink>
          <button type="button" class="btn btn-secondary" :disabled="loading" @click="loadRows">
            <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
            刷新
          </button>
        </div>
      </section>

      <section class="card p-4">
        <div class="grid gap-3 md:grid-cols-4 xl:grid-cols-8">
          <label class="block md:col-span-2">
            <span class="input-label">关键词</span>
            <div class="relative">
              <Icon name="search" size="sm" class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
              <input v-model.trim="filters.q" class="input pl-9" placeholder="账号、供应商、分组、Key" @keyup.enter="submitFilters" />
            </div>
          </label>
          <label class="block">
            <span class="input-label">供应商</span>
            <select v-model.number="filters.supplier_id" class="input" @change="handleSupplierFilterChanged">
              <option :value="0">全部供应商</option>
              <option v-for="supplier in suppliers" :key="supplier.id" :value="supplier.id">
                {{ supplier.name }}
              </option>
            </select>
          </label>
          <label class="block">
            <span class="input-label">本地分组</span>
            <select v-model.number="filters.local_group_id" class="input">
              <option :value="0">全部分组</option>
              <option v-for="group in localGroupOptions" :key="group.id" :value="group.id">
                {{ group.name }}
              </option>
            </select>
          </label>
          <label class="block">
            <span class="input-label">调度状态</span>
            <select v-model="filters.schedulable" class="input">
              <option value="">全部</option>
              <option value="true">已开启</option>
              <option value="false">已关闭</option>
            </select>
          </label>
          <label class="block">
            <span class="input-label">第三方分组</span>
            <select v-model.number="filters.supplier_group_id" class="input">
              <option :value="0">全部第三方分组</option>
              <option v-for="group in supplierGroupOptions" :key="group.id" :value="group.id">
                {{ group.label }}
              </option>
            </select>
          </label>
          <label class="block">
            <span class="input-label">最高倍率</span>
            <input v-model.trim="filters.max_rate_multiplier" class="input" inputmode="decimal" placeholder="例如 0.2" @keyup.enter="submitFilters" />
          </label>
          <label class="block">
            <span class="input-label">余额状态</span>
            <select v-model="filters.balance_status" class="input">
              <option value="">全部</option>
              <option value="usable">可用</option>
              <option value="insufficient">不足</option>
              <option value="unknown">未知</option>
              <option value="unbound">未绑定</option>
            </select>
          </label>
          <label class="block">
            <span class="input-label">通道状态</span>
            <select v-model="filters.channel_check_status" class="input">
              <option value="">全部</option>
              <option value="available">可用</option>
              <option value="untested">未检测</option>
              <option value="request_error">请求错误</option>
              <option value="remote_unavailable">远端不可用</option>
              <option value="probe_failed">检测失败</option>
              <option value="slow_first_token">首 token 慢</option>
              <option value="slow_total">总耗时慢</option>
              <option value="no_local_account">无本地账号</option>
            </select>
          </label>
        </div>
        <div class="mt-3 flex flex-wrap gap-2">
          <button type="button" class="btn btn-primary" :disabled="loading" @click="submitFilters">
            <Icon name="filter" size="sm" />
            筛选
          </button>
          <button type="button" class="btn btn-ghost" :disabled="loading" @click="resetFilters">重置</button>
        </div>
      </section>

      <section class="grid gap-3 md:grid-cols-4">
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">当前结果</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ pagination.total }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">已开启调度</p>
          <p class="mt-2 text-2xl font-semibold text-emerald-600 dark:text-emerald-300">{{ pageStats.schedulable }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">未绑定供应商</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ pageStats.unbound }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">需处理</p>
          <p class="mt-2 text-2xl font-semibold text-amber-600 dark:text-amber-300">{{ pageStats.actionRequired }}</p>
        </div>
      </section>

      <section class="card p-4">
        <div class="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
          <div>
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">账号调度操作</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
              当前页已选择 {{ selectedAccountIds.length }} 个本地账号，执行前会先预览分组调度池影响。
            </p>
          </div>
          <div class="grid gap-3 lg:min-w-[720px] lg:grid-cols-[minmax(220px,1fr)_auto] lg:items-end">
            <label class="block">
              <span class="input-label">目标本地分组</span>
              <select v-model.number="selectedGroupId" class="input">
                <option :value="0">选择分组后可加入或移出</option>
                <option v-for="group in localGroupOptions" :key="group.id" :value="group.id">
                  {{ group.name }}
                </option>
              </select>
            </label>
            <div class="flex flex-wrap gap-2">
              <button type="button" class="btn btn-success btn-sm" :disabled="bulkActionDisabled" @click="prepareBulkSchedulable(true)">
                <Icon name="checkCircle" size="sm" />
                开启调度
              </button>
              <button type="button" class="btn btn-warning btn-sm" :disabled="bulkActionDisabled" @click="prepareBulkSchedulable(false)">
                <Icon name="ban" size="sm" />
                关闭调度
              </button>
              <button type="button" class="btn btn-secondary btn-sm" :disabled="groupActionDisabled" @click="prepareBulkGroupAction('add_to_groups')">
                <Icon name="plus" size="sm" />
                加入分组
              </button>
              <button type="button" class="btn btn-secondary btn-sm" :disabled="groupActionDisabled" @click="prepareBulkGroupAction('remove_from_groups')">
                <Icon name="x" size="sm" />
                移出分组
              </button>
              <button type="button" class="btn btn-secondary btn-sm" :disabled="syncDisabled" @click="syncLocalState">
                <Icon name="sync" size="sm" :class="{ 'animate-spin': syncBusy }" />
                {{ syncButtonText }}
              </button>
              <button type="button" class="btn btn-secondary btn-sm" :disabled="refillDisabled" @click="previewRoutingRefill">
                <Icon name="search" size="sm" :class="{ 'animate-spin': refillBusy }" />
                预览补池
              </button>
              <button type="button" class="btn btn-primary btn-sm" :disabled="refillDisabled" @click="applyRoutingRefill">
                <Icon name="plus" size="sm" :class="{ 'animate-spin': refillBusy }" />
                补入最低倍率
              </button>
              <button v-if="selectedAccountIds.length > 0" type="button" class="btn btn-ghost btn-sm" :disabled="actionBusy || syncBusy" @click="clearSelection">
                清空选择
              </button>
            </div>
          </div>
        </div>
        <div v-if="refillResult" class="mt-3 rounded-lg border border-gray-200 bg-gray-50 p-3 text-sm dark:border-dark-700 dark:bg-dark-800/70">
          <div class="flex flex-wrap items-center justify-between gap-2">
            <p class="font-medium text-gray-900 dark:text-white">{{ refillResultTitle }}</p>
            <span v-if="refillResult.candidate" class="badge badge-success">
              {{ routingRefillMultiplierLabel(refillResult.candidate.effective_rate_multiplier) }}
            </span>
            <span v-else class="badge badge-gray">{{ routingRefillSkippedReasonLabel(refillResult.skipped_reason) }}</span>
          </div>
          <p v-if="refillResult.candidate" class="mt-1 text-gray-600 dark:text-dark-300">
            {{ refillResult.candidate.supplier_name || '-' }} · {{ refillResult.candidate.supplier_group_name || '-' }} · {{ refillResult.candidate.local_account_name || `#${refillResult.candidate.local_sub2api_account_id}` }}
          </p>
          <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
            可调度账号 {{ refillResult.availability_before?.schedulable_accounts ?? '-' }}
            <template v-if="refillResult.availability_after"> -> {{ refillResult.availability_after.schedulable_accounts }}</template>
            · 用户 Key {{ refillResult.availability_before?.active_api_key_count ?? '-' }}
          </p>
          <RoutingRefillImpactPanel :availability="refillResult.availability_before" />
        </div>
      </section>

      <section class="card overflow-hidden">
        <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">账号运营镜像</h2>
        </div>
        <div class="overflow-x-auto">
          <table class="w-full min-w-[1540px] divide-y divide-gray-200 dark:divide-dark-700">
            <thead class="bg-gray-50 dark:bg-dark-800">
              <tr>
                <th class="w-12 px-4 py-3 text-left">
                  <input
                    type="checkbox"
                    class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                    :checked="allPageSelected"
                    :disabled="rows.length === 0 || actionBusy"
                    aria-label="选择当前页账号"
                    @change="togglePageSelectionFromEvent"
                  />
                </th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">本地账号</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">本地分组</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">供应商 / Key</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">第三方分组</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">倍率</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">调度</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">余额</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">检测 / 漂移</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">更新时间</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">操作</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-900">
              <tr v-if="rows.length === 0">
                <td colspan="11" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-dark-400">暂无账号</td>
              </tr>
              <tr v-for="row in rows" :key="rowKey(row)" class="align-top">
                <td class="px-4 py-4">
                  <input
                    type="checkbox"
                    class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                    :checked="isAccountSelected(row.local_sub2api_account_id)"
                    :disabled="actionBusy"
                    :aria-label="`选择账号 ${row.local_account_name || row.local_sub2api_account_id}`"
                    @change="toggleAccountSelectionFromEvent(row.local_sub2api_account_id, $event)"
                  />
                </td>
                <td class="px-4 py-4">
                  <div class="text-sm font-medium text-gray-900 dark:text-gray-100">{{ row.local_account_name || `#${row.local_sub2api_account_id}` }}</div>
                  <div class="mt-1 flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-dark-400">
                    <span class="font-mono">#{{ row.local_sub2api_account_id }}</span>
                    <span>{{ row.local_account_platform || '-' }}</span>
                    <span>{{ row.local_account_type || '-' }}</span>
                  </div>
                  <div v-if="row.local_account_error_message" class="mt-1 max-w-[240px] truncate text-xs text-rose-500">
                    {{ row.local_account_error_message }}
                  </div>
                </td>
                <td class="px-4 py-4 text-sm text-gray-700 dark:text-gray-200">
                  <div class="flex max-w-[220px] flex-wrap gap-1">
                    <span v-for="name in visibleLocalGroups(row)" :key="name" class="badge bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-dark-200">{{ name }}</span>
                    <span v-if="visibleLocalGroups(row).length === 0" class="text-gray-400">-</span>
                  </div>
                </td>
                <td class="px-4 py-4">
                  <div class="text-sm font-medium text-gray-900 dark:text-gray-100">{{ row.supplier_name || '未绑定' }}</div>
                  <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                    <span v-if="row.supplier_key_name">{{ row.supplier_key_name }}</span>
                    <span v-else-if="row.supplier_key_id">Key #{{ row.supplier_key_id }}</span>
                    <span v-else>-</span>
                    <span v-if="row.supplier_key_last4" class="font-mono"> · ****{{ row.supplier_key_last4 }}</span>
                  </div>
                  <div class="mt-2 flex flex-wrap gap-1">
                    <span v-if="row.supplier_runtime_status" class="badge" :class="runtimeStatusClass(row.supplier_runtime_status)">
                      {{ runtimeStatusLabel(row.supplier_runtime_status) }}
                    </span>
                    <span v-if="row.supplier_key_status" class="badge" :class="keyStatusClass(row.supplier_key_status)">
                      {{ keyStatusLabel(row.supplier_key_status) }}
                    </span>
                  </div>
                </td>
                <td class="px-4 py-4">
                  <div class="text-sm text-gray-900 dark:text-gray-100">{{ row.supplier_group_name || '-' }}</div>
                  <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                    <span v-if="row.supplier_external_group_id" class="font-mono">#{{ row.supplier_external_group_id }}</span>
                    <span v-if="row.supplier_group_provider"> {{ row.supplier_group_provider }}</span>
                  </div>
                  <span v-if="row.supplier_group_status" class="badge mt-2" :class="groupStatusClass(row.supplier_group_status)">
                    {{ groupStatusLabel(row.supplier_group_status) }}
                  </span>
                </td>
                <td class="px-4 py-4 text-right">
                  <div class="text-sm font-semibold text-gray-900 dark:text-gray-100">{{ formatRate(row.effective_rate_multiplier) }}</div>
                  <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">本地 {{ formatRate(row.local_account_rate_multiplier) }}</div>
                </td>
                <td class="px-4 py-4">
                  <span class="badge" :class="row.local_account_schedulable ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300' : 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-dark-200'">
                    {{ row.local_account_schedulable ? '已开启' : '已关闭' }}
                  </span>
                  <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">并发 {{ row.local_account_concurrency }} · 优先级 {{ row.local_account_priority }}</div>
                  <div v-if="row.local_account_temp_unschedulable_reason" class="mt-1 max-w-[180px] truncate text-xs text-amber-600 dark:text-amber-300">
                    {{ row.local_account_temp_unschedulable_reason }}
                  </div>
                </td>
                <td class="px-4 py-4">
                  <span class="badge" :class="balanceStatusClass(row.balance_status)">{{ balanceStatusLabel(row.balance_status) }}</span>
                  <div class="mt-1 text-sm text-gray-900 dark:text-gray-100">{{ formatMoney(row.balance_cents, row.balance_currency) }}</div>
                  <div v-if="row.balance_threshold_cents > 0" class="mt-1 text-xs text-gray-500 dark:text-dark-400">阈值 {{ formatMoney(row.balance_threshold_cents, row.balance_currency) }}</div>
                </td>
                <td class="px-4 py-4">
                  <div class="flex flex-wrap gap-1">
                    <span v-if="row.candidate_status" class="badge" :class="candidateStatusClass(row.candidate_status)">
                      {{ candidateStatusLabel(row.candidate_status) }}
                    </span>
                    <span class="badge" :class="channelStatusClass(row.channel_check_status)">
                      {{ channelStatusLabel(row.channel_check_status) }}
                    </span>
                    <span class="badge" :class="driftStatusClass(row.drift_status)">
                      {{ driftStatusLabel(row.drift_status) }}
                    </span>
                    <span v-if="showPurityBadge(row)" class="badge" :class="purityBadgeClass(row)">
                      {{ purityBadgeLabel(row) }}
                    </span>
                    <span v-if="showProxyBadge(row)" class="badge" :class="proxyStatusClass(row.local_account_proxy_status)">
                      {{ proxyStatusLabel(row.local_account_proxy_status) }}
                    </span>
                  </div>
                  <div v-if="row.channel_error_message" class="mt-1 max-w-[240px] truncate text-xs text-rose-500">
                    {{ row.channel_error_message }}
                  </div>
                  <div v-if="row.blocked_reason" class="mt-1 max-w-[240px] truncate text-xs text-amber-600 dark:text-amber-300">
                    {{ blockedReasonLabel(row.blocked_reason) }}
                    <span v-if="row.check_source"> · {{ checkSourceLabel(row.check_source) }}</span>
                  </div>
                  <div v-if="showPurityBadge(row)" class="mt-1 max-w-[240px] truncate text-xs text-gray-500 dark:text-dark-400" :title="purityTitle(row)">
                    {{ puritySummary(row) }}
                  </div>
                  <div v-if="showProxyBadge(row)" class="mt-1 max-w-[240px] truncate text-xs text-gray-500 dark:text-dark-400" :title="proxyTitle(row)">
                    {{ proxySummary(row) }}
                  </div>
                  <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                    {{ row.last_channel_check_at ? formatDateTime(row.last_channel_check_at) : '未检测' }}
                  </div>
                </td>
                <td class="px-4 py-4 text-xs text-gray-500 dark:text-dark-400">
                  <div>账号 {{ formatDateTime(row.local_account_updated_at) }}</div>
                  <div v-if="row.last_local_sync_at" class="mt-1">同步 {{ formatDateTime(row.last_local_sync_at) }}</div>
                </td>
                <td class="px-4 py-4">
                  <div class="flex max-w-[180px] flex-wrap gap-2">
                    <button
                      type="button"
                      class="btn btn-sm"
                      :class="row.local_account_schedulable ? 'btn-warning' : 'btn-success'"
                      :disabled="rowActionDisabled"
                      @click="prepareRowSchedulable(row, !row.local_account_schedulable)"
                    >
                      <Icon :name="row.local_account_schedulable ? 'ban' : 'checkCircle'" size="sm" />
                      {{ row.local_account_schedulable ? '关闭' : '开启' }}
                    </button>
                    <button type="button" class="btn btn-secondary btn-sm" :disabled="rowActionDisabled || selectedGroupId <= 0" @click="prepareRowGroupAction(row, 'add_to_groups')">
                      加入
                    </button>
                    <button type="button" class="btn btn-secondary btn-sm" :disabled="rowActionDisabled || selectedGroupId <= 0" @click="prepareRowGroupAction(row, 'remove_from_groups')">
                      移出
                    </button>
                    <button
                      v-if="row.drift_status === 'local_account_state_drift'"
                      type="button"
                      class="btn btn-danger btn-sm"
                      :disabled="rowActionDisabled"
                      @click="openDriftDialog(row)"
                    >
                      <Icon name="exclamationTriangle" size="sm" />
                      变更
                    </button>
                    <button
                      v-if="supportsPurity(row)"
                      type="button"
                      class="btn btn-secondary btn-sm"
                      :disabled="rowActionDisabled"
                      :title="purityIsStale(row) ? '重新执行纯度检测，刷新候选复检状态' : '执行本地账号纯度检测'"
                      @click="openPurityDialog(row)"
                    >
                      <Icon name="shield" size="sm" />
                      {{ purityIsStale(row) ? '复检' : '纯度' }}
                    </button>
                    <button
                      type="button"
                      class="btn btn-ghost btn-sm"
                      :disabled="rowActionDisabled"
                      :title="`查看本地账号 #${row.local_sub2api_account_id} 的操作审计`"
                      @click="openAccountAudit(row)"
                    >
                      <Icon name="clipboard" size="sm" />
                      审计
                    </button>
                    <button
                      type="button"
                      class="btn btn-ghost btn-sm"
                      :disabled="rowActionDisabled"
                      :title="`复制账号 ID 并打开 Sub2API 原后台：#${row.local_sub2api_account_id}`"
                      @click="openSub2APIAccount(row)"
                    >
                      <Icon name="externalLink" size="sm" />
                      原后台
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
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

      <div v-if="actionDialogOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" @click.self="closeActionDialog">
        <div class="max-h-[90vh] w-full max-w-3xl overflow-hidden rounded-lg bg-white shadow-xl dark:bg-dark-900" role="dialog" aria-modal="true">
          <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
            <div class="flex items-start justify-between gap-4">
              <div>
                <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ pendingActionTitle }}</h2>
                <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
                  将影响 {{ pendingPayload?.account_ids.length || 0 }} 个本地账号。执行后会写入调度刷新队列。
                </p>
              </div>
              <button type="button" class="btn btn-ghost btn-icon" :disabled="actionBusy" aria-label="关闭" @click="closeActionDialog">
                <Icon name="x" size="sm" />
              </button>
            </div>
          </div>
          <div class="max-h-[62vh] overflow-y-auto px-5 py-4">
            <div v-if="actionBusy && !previewResult" class="flex items-center gap-2 text-sm text-gray-500 dark:text-dark-400">
              <Icon name="refresh" size="sm" class="animate-spin" />
              正在预览调度池影响...
            </div>
            <div v-else-if="previewResult" class="space-y-4">
              <div
                v-if="actionAppliedResult"
                class="rounded-md border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-700 dark:border-emerald-900/50 dark:bg-emerald-900/20 dark:text-emerald-200"
              >
                <div class="flex flex-wrap items-center justify-between gap-2">
                  <div>
                    <p class="font-medium">{{ actionAppliedResult.blocked ? '执行已被安全保护拦截' : '执行已写入统一执行历史' }}</p>
                    <p class="mt-1">{{ actionResultSummary(actionAppliedResult) }}</p>
                  </div>
                  <RouterLink
                    v-if="actionExecutionPath(actionAppliedResult)"
                    :to="actionExecutionPath(actionAppliedResult)"
                    class="btn btn-secondary btn-sm"
                  >
                    <Icon name="clock" size="sm" />
                    查看执行历史
                  </RouterLink>
                </div>
              </div>

              <div
                v-if="previewResult.blocked"
                class="rounded-md border border-rose-200 bg-rose-50 p-3 text-sm text-rose-700 dark:border-rose-900/50 dark:bg-rose-900/20 dark:text-rose-200"
              >
                <div class="flex items-start gap-2">
                  <Icon name="exclamationTriangle" size="sm" class="mt-0.5 shrink-0" />
                  <div>
                    <p class="font-medium">{{ blockedReasonTitle(previewResult.blocked_reason) }}</p>
                    <p class="mt-1">{{ blockedReasonMessage(previewResult.blocked_reason) }}</p>
                  </div>
                </div>
              </div>
              <div
                v-else-if="(previewResult.warnings || []).length > 0"
                class="rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800 dark:border-amber-900/50 dark:bg-amber-900/20 dark:text-amber-200"
              >
                <div class="flex items-start gap-2">
                  <Icon name="exclamationTriangle" size="sm" class="mt-0.5 shrink-0" />
                  <div>
                    <p class="font-medium">执行前需要确认</p>
                    <ul class="mt-1 list-disc space-y-1 pl-4">
                      <li v-for="warning in previewResult.warnings" :key="warning">{{ warning }}</li>
                    </ul>
                  </div>
                </div>
              </div>

              <div class="grid gap-3 sm:grid-cols-3">
                <div class="rounded-md border border-gray-100 p-3 dark:border-dark-700">
                  <p class="text-xs text-gray-500 dark:text-dark-400">账号数</p>
                  <p class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">{{ previewResult.account_ids.length }}</p>
                </div>
                <div class="rounded-md border border-gray-100 p-3 dark:border-dark-700">
                  <p class="text-xs text-gray-500 dark:text-dark-400">目标分组</p>
                  <p class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">{{ previewResult.group_ids?.length || '-' }}</p>
                </div>
                <div class="rounded-md border border-gray-100 p-3 dark:border-dark-700">
                  <p class="text-xs text-gray-500 dark:text-dark-400">影响分组</p>
                  <p class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">{{ previewResult.group_impacts?.length || 0 }}</p>
                </div>
              </div>

              <div v-if="(previewResult.group_impacts || []).length > 0" class="overflow-x-auto rounded-md border border-gray-100 dark:border-dark-700">
                <table class="w-full min-w-[640px] divide-y divide-gray-200 text-sm dark:divide-dark-700">
                  <thead class="bg-gray-50 text-xs uppercase tracking-wider text-gray-500 dark:bg-dark-800 dark:text-dark-400">
                    <tr>
                      <th class="px-3 py-2 text-left font-medium">本地分组</th>
                      <th class="px-3 py-2 text-right font-medium">启用 API Key</th>
                      <th class="px-3 py-2 text-right font-medium">操作前可调度</th>
                      <th class="px-3 py-2 text-right font-medium">操作后可调度</th>
                      <th class="px-3 py-2 text-left font-medium">风险</th>
                    </tr>
                  </thead>
                  <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                    <tr v-for="impact in previewResult.group_impacts" :key="impact.group_id">
                      <td class="px-3 py-2 text-gray-900 dark:text-gray-100">{{ impact.group_name || `#${impact.group_id}` }}</td>
                      <td class="px-3 py-2 text-right text-gray-700 dark:text-dark-200">{{ impact.active_api_key_count }}</td>
                      <td class="px-3 py-2 text-right text-gray-700 dark:text-dark-200">{{ impact.before_schedulable_accounts }}</td>
                      <td class="px-3 py-2 text-right font-medium" :class="impact.after_schedulable_accounts === 0 ? 'text-rose-600 dark:text-rose-300' : 'text-gray-900 dark:text-gray-100'">
                        {{ impact.after_schedulable_accounts }}
                      </td>
                      <td class="px-3 py-2">
                        <span v-if="impact.would_empty_schedulable_pool" class="badge badge-danger">空池</span>
                        <span v-else class="badge bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300">可执行</span>
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>

              <div class="rounded-md bg-gray-50 p-3 text-xs text-gray-500 dark:bg-dark-800 dark:text-dark-400">
                <div class="font-medium text-gray-700 dark:text-dark-200">账号 ID</div>
                <div class="mt-1 break-all font-mono">{{ previewResult.account_ids.join(', ') }}</div>
              </div>
            </div>
          </div>
          <div class="flex flex-col-reverse gap-2 border-t border-gray-100 px-5 py-4 dark:border-dark-700 sm:flex-row sm:items-center sm:justify-end">
            <button type="button" class="btn btn-ghost" :disabled="actionBusy" @click="closeActionDialog">{{ actionAppliedResult ? '关闭' : '取消' }}</button>
            <button
              v-if="!actionAppliedResult"
              type="button"
              class="btn"
              :class="previewResult?.blocked ? 'btn-danger' : pendingActionDanger ? 'btn-warning' : 'btn-primary'"
              :disabled="actionBusy || !previewResult || previewResult.blocked"
              @click="applyPendingAction"
            >
              <Icon v-if="actionBusy" name="refresh" size="sm" class="animate-spin" />
              {{ actionBusy ? '提交中...' : previewResult?.blocked ? '不可执行' : '确认执行' }}
            </button>
          </div>
        </div>
      </div>

      <div v-if="driftDialogOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" @click.self="closeDriftDialog()">
        <div class="max-h-[90vh] w-full max-w-4xl overflow-hidden rounded-lg bg-white shadow-xl dark:bg-dark-900" role="dialog" aria-modal="true">
          <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
            <div class="flex items-start justify-between gap-4">
              <div>
                <h2 class="text-lg font-semibold text-gray-900 dark:text-white">原后台变更</h2>
                <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
                  本地账号 #{{ driftAccountId || '-' }} 的 Sub2API 当前状态与 Admin Plus 已采纳基线不一致。
                </p>
              </div>
              <button type="button" class="btn btn-ghost btn-icon" :disabled="driftBusy || driftActionBusy" aria-label="关闭" @click="closeDriftDialog()">
                <Icon name="x" size="sm" />
              </button>
            </div>
          </div>
          <div class="max-h-[62vh] overflow-y-auto px-5 py-4">
            <div v-if="driftBusy" class="flex items-center gap-2 text-sm text-gray-500 dark:text-dark-400">
              <Icon name="refresh" size="sm" class="animate-spin" />
              正在读取 Sub2API 当前状态...
            </div>
            <div v-else-if="driftSummary" class="space-y-4">
              <div class="rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800 dark:border-amber-900/50 dark:bg-amber-900/20 dark:text-amber-200">
                <div class="flex items-start gap-2">
                  <Icon name="exclamationTriangle" size="sm" class="mt-0.5 shrink-0" />
                  <div>
                    <p class="font-medium">确认后再恢复调度写回</p>
                    <p class="mt-1">采纳会把 Sub2API 当前状态设为新基线；恢复会把 Admin Plus 基线写回 Sub2API，并刷新调度队列。</p>
                  </div>
                </div>
              </div>
              <div class="overflow-x-auto rounded-md border border-gray-100 dark:border-dark-700">
                <table class="w-full min-w-[720px] divide-y divide-gray-200 text-sm dark:divide-dark-700">
                  <thead class="bg-gray-50 text-xs uppercase tracking-wider text-gray-500 dark:bg-dark-800 dark:text-dark-400">
                    <tr>
                      <th class="px-3 py-2 text-left font-medium">字段</th>
                      <th class="px-3 py-2 text-left font-medium">Admin Plus 基线</th>
                      <th class="px-3 py-2 text-left font-medium">Sub2API 当前</th>
                      <th class="px-3 py-2 text-left font-medium">状态</th>
                    </tr>
                  </thead>
                  <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                    <tr v-for="field in driftFieldRows" :key="field.key">
                      <td class="px-3 py-2 text-gray-700 dark:text-dark-200">{{ field.label }}</td>
                      <td class="px-3 py-2 font-mono text-xs text-gray-900 dark:text-gray-100">{{ field.accepted }}</td>
                      <td class="px-3 py-2 font-mono text-xs text-gray-900 dark:text-gray-100">{{ field.observed }}</td>
                      <td class="px-3 py-2">
                        <span class="badge" :class="field.changed ? 'bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-300' : 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'">
                          {{ field.changed ? '已变更' : '一致' }}
                        </span>
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
              <div class="text-xs text-gray-500 dark:text-dark-400">
                最后同步 {{ formatDateTime(driftSummary.last_checked_at) }}
              </div>
            </div>
            <div v-else class="text-sm text-gray-500 dark:text-dark-400">
              未发现待处理的原后台变更。
            </div>
          </div>
          <div class="flex flex-col-reverse gap-2 border-t border-gray-100 px-5 py-4 dark:border-dark-700 sm:flex-row sm:items-center sm:justify-end">
            <button type="button" class="btn btn-ghost" :disabled="driftBusy || driftActionBusy" @click="closeDriftDialog()">关闭</button>
            <button type="button" class="btn btn-primary" :disabled="!driftSummary || driftBusy || driftActionBusy" @click="acceptDrift">
              <Icon v-if="driftActionBusy" name="refresh" size="sm" class="animate-spin" />
              采纳原后台
            </button>
            <button type="button" class="btn btn-warning" :disabled="!driftSummary || driftBusy || driftActionBusy" @click="restoreDrift">
              <Icon v-if="driftActionBusy" name="refresh" size="sm" class="animate-spin" />
              恢复基线
            </button>
          </div>
        </div>
      </div>

      <LocalAccountPurityModal :show="Boolean(purityAccount)" :account="purityAccount" @close="closePurityDialog" />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import Pagination from '@/components/common/Pagination.vue'
import LocalAccountPurityModal from '@/components/admin-plus/LocalAccountPurityModal.vue'
import RoutingRefillImpactPanel from '@/views/admin/RoutingRefillImpactPanel.vue'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { useAppStore } from '@/stores/app'
import { routingRefillMultiplierLabel, routingRefillSkippedReasonLabel } from '@/views/admin/routingRefillPresentation'
import {
  acceptLocalAccountState,
  applyLocalAccountOpsAction,
  listAllSupplierGroups,
  listLocalAccountOps,
  listLocalSub2APIGroups,
  listSuppliers,
  previewLocalAccountOpsAction,
  refillLocalGroup,
  restoreLocalAccountState,
  syncLocalAccountState,
  type LocalAccountOpsAction,
  type LocalAccountOpsActionPayload,
  type LocalAccountOpsActionResult,
  type LocalAccountOpsRow,
  type LocalAccountStateDriftSummary,
  type LocalSub2APIAccount,
  type LocalSub2APIGroup,
  type RoutingRefillResult,
  type Supplier,
  type SupplierGroup
} from '@/api/admin/adminPlus'

interface LocalGroupOption {
  id: number
  name: string
}

interface SupplierGroupOption {
  id: number
  supplierId: number
  name: string
  label: string
}

const appStore = useAppStore()
const route = useRoute()
const router = useRouter()
const loading = ref(false)
const actionBusy = ref(false)
const syncBusy = ref(false)
const refillBusy = ref(false)
const driftBusy = ref(false)
const driftActionBusy = ref(false)
const rows = ref<LocalAccountOpsRow[]>([])
const suppliers = ref<Supplier[]>([])
const supplierGroups = ref<SupplierGroup[]>([])
const localGroups = ref<LocalSub2APIGroup[]>([])
const selectedGroupId = ref(0)
const selectedAccountSet = ref<Set<number>>(new Set())
const actionDialogOpen = ref(false)
const driftDialogOpen = ref(false)
const pendingPayload = ref<LocalAccountOpsActionPayload | null>(null)
const previewResult = ref<LocalAccountOpsActionResult | null>(null)
const actionAppliedResult = ref<LocalAccountOpsActionResult | null>(null)
const refillResult = ref<RoutingRefillResult | null>(null)
const driftSummary = ref<LocalAccountStateDriftSummary | null>(null)
const driftAccountId = ref(0)
const purityAccount = ref<LocalSub2APIAccount | null>(null)
const pagination = reactive({ page: 1, page_size: getPersistedPageSize(), total: 0, pages: 0 })
const filters = reactive({
  q: '',
  supplier_id: 0,
  local_group_id: 0,
  supplier_group_id: 0,
  max_rate_multiplier: '',
  balance_status: '',
  channel_check_status: '',
  schedulable: ''
})

const localGroupOptions = computed<LocalGroupOption[]>(() => {
  return localGroups.value
    .map((group) => ({ id: group.id, name: group.name || `#${group.id}` }))
    .sort((a, b) => a.name.localeCompare(b.name))
})

const suppliersById = computed(() => new Map(suppliers.value.map((supplier) => [supplier.id, supplier])))

const supplierGroupOptions = computed<SupplierGroupOption[]>(() => {
  return supplierGroups.value
    .map((group) => ({
      id: group.id,
      supplierId: group.supplier_id,
      name: group.name || `#${group.id}`,
      label: supplierGroupLabel(group)
    }))
    .sort((a, b) => a.label.localeCompare(b.label))
})

const pageStats = computed(() => ({
  schedulable: rows.value.filter((row) => row.local_account_schedulable).length,
  unbound: rows.value.filter((row) => row.drift_status === 'unbound').length,
  actionRequired: rows.value.filter(rowNeedsAction).length
}))

const pageAccountIds = computed(() => uniqueNumbers(rows.value.map((row) => row.local_sub2api_account_id)))
const selectedAccountIds = computed(() => Array.from(selectedAccountSet.value).sort((a, b) => a - b))
const allPageSelected = computed(() => pageAccountIds.value.length > 0 && pageAccountIds.value.every((id) => selectedAccountSet.value.has(id)))
const bulkActionDisabled = computed(() => actionBusy.value || syncBusy.value || selectedAccountIds.value.length === 0)
const groupActionDisabled = computed(() => bulkActionDisabled.value || selectedGroupId.value <= 0)
const pendingActionTitle = computed(() => pendingPayload.value ? actionTitle(pendingPayload.value) : '账号调度操作')
const pendingActionDanger = computed(() => pendingPayload.value?.action === 'set_schedulable' && pendingPayload.value.schedulable === false)
const syncTargetAccountIds = computed(() => selectedAccountIds.value.length > 0 ? selectedAccountIds.value : pageAccountIds.value)
const rowActionDisabled = computed(() => actionBusy.value || syncBusy.value || driftBusy.value || driftActionBusy.value)
const refillDisabled = computed(() => loading.value || rowActionDisabled.value || refillBusy.value || selectedGroupId.value <= 0)
const syncDisabled = computed(() => loading.value || rowActionDisabled.value || syncTargetAccountIds.value.length === 0)
const refillResultTitle = computed(() => {
  if (!refillResult.value) return ''
  if (refillResult.value.skipped_reason) return `补池跳过：${routingRefillSkippedReasonLabel(refillResult.value.skipped_reason)}`
  return refillResult.value.dry_run ? '补池预览候选' : '补池已执行'
})
const syncButtonText = computed(() => {
  if (syncBusy.value) return '同步中...'
  if (selectedAccountIds.value.length > 0) return `同步已选 ${selectedAccountIds.value.length}`
  return '同步当前页'
})
const driftFieldRows = computed(() => {
  if (!driftSummary.value) return []
  const fields = [
    { key: 'name', label: '账号名' },
    { key: 'platform', label: '平台' },
    { key: 'type', label: '类型' },
    { key: 'schedulable', label: '调度开关' },
    { key: 'groups', label: '本地分组' }
  ]
  return fields.map((field) => ({
    ...field,
    accepted: snapshotFieldValue(driftSummary.value!, 'accepted', field.key),
    observed: snapshotFieldValue(driftSummary.value!, 'observed', field.key),
    changed: (driftSummary.value!.drift_fields || []).includes(field.key)
  }))
})

async function loadRows() {
  const maxRateMultiplier = parsedMaxRateMultiplier()
  if (filters.max_rate_multiplier.trim() && maxRateMultiplier === undefined) {
    appStore.showWarning('最高倍率必须是大于 0 的数字')
    return
  }
  loading.value = true
  try {
    const [supplierResult, localGroupResult, supplierGroupResult, opsResult] = await Promise.all([
      listSuppliers({ limit: 1000 }),
      listLocalSub2APIGroups({ limit: 1000 }),
      listAllSupplierGroups({
        supplier_id: filters.supplier_id || undefined,
        limit: 1000
      }),
      listLocalAccountOps({
        q: filters.q || undefined,
        supplier_id: filters.supplier_id || undefined,
        local_group_id: filters.local_group_id || undefined,
        supplier_group_id: filters.supplier_group_id || undefined,
        max_rate_multiplier: maxRateMultiplier,
        balance_status: filters.balance_status || undefined,
        channel_check_status: filters.channel_check_status || undefined,
        schedulable: filters.schedulable === '' ? undefined : filters.schedulable === 'true',
        page: pagination.page,
        page_size: pagination.page_size
      })
    ])
    suppliers.value = supplierResult.items
    localGroups.value = localGroupResult.items
    supplierGroups.value = supplierGroupResult.items
    pruneSupplierGroupFilter()
    rows.value = opsResult.items
    pruneSelection()
    pagination.total = opsResult.total || 0
    pagination.pages = opsResult.pages || 0
    pagination.page = opsResult.page || pagination.page
    pagination.page_size = opsResult.page_size || pagination.page_size
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '加载本地账号运营视图失败')
  } finally {
    loading.value = false
  }
}

function pruneSupplierGroupFilter() {
  if (filters.supplier_group_id <= 0) return
  if (supplierGroups.value.some((group) => group.id === filters.supplier_group_id)) return
  filters.supplier_group_id = 0
}

function submitFilters() {
  pagination.page = 1
  void loadRows()
}

function handleSupplierFilterChanged() {
  filters.supplier_group_id = 0
}

function resetFilters() {
  filters.q = ''
  filters.supplier_id = 0
  filters.local_group_id = 0
  filters.supplier_group_id = 0
  filters.max_rate_multiplier = ''
  filters.balance_status = ''
  filters.channel_check_status = ''
  filters.schedulable = ''
  pagination.page = 1
  void loadRows()
}

function applyRouteFilters() {
  filters.q = stringQueryValue(route.query.q)
  filters.supplier_id = numberQueryValue(route.query.supplier_id)
  filters.local_group_id = numberQueryValue(route.query.local_group_id)
  filters.supplier_group_id = numberQueryValue(route.query.supplier_group_id)
  filters.max_rate_multiplier = stringQueryValue(route.query.max_rate_multiplier)
  filters.balance_status = stringQueryValue(route.query.balance_status)
  filters.channel_check_status = stringQueryValue(route.query.channel_check_status)
  filters.schedulable = stringQueryValue(route.query.schedulable)
}

function stringQueryValue(value: unknown): string {
  if (Array.isArray(value)) return String(value[0] || '').trim()
  return String(value || '').trim()
}

function numberQueryValue(value: unknown): number {
  const parsed = Number(stringQueryValue(value))
  return Number.isFinite(parsed) && parsed > 0 ? parsed : 0
}

function parsedMaxRateMultiplier(): number | undefined {
  const raw = filters.max_rate_multiplier.trim()
  if (!raw) return undefined
  const value = Number(raw)
  if (!Number.isFinite(value) || value <= 0) return undefined
  return value
}

function handlePageChange(page: number) {
  pagination.page = page
  void loadRows()
}

function handlePageSizeChange(pageSize: number) {
  pagination.page_size = pageSize
  pagination.page = 1
  void loadRows()
}

function isAccountSelected(accountId: number): boolean {
  return selectedAccountSet.value.has(accountId)
}

function toggleAccountSelectionFromEvent(accountId: number, event: Event) {
  const checked = (event.target as HTMLInputElement | null)?.checked ?? false
  const next = new Set(selectedAccountSet.value)
  if (checked) next.add(accountId)
  else next.delete(accountId)
  selectedAccountSet.value = next
}

function togglePageSelectionFromEvent(event: Event) {
  const checked = (event.target as HTMLInputElement | null)?.checked ?? false
  const next = new Set(selectedAccountSet.value)
  pageAccountIds.value.forEach((id) => {
    if (checked) next.add(id)
    else next.delete(id)
  })
  selectedAccountSet.value = next
}

function clearSelection() {
  selectedAccountSet.value = new Set()
}

function pruneSelection() {
  const visible = new Set(pageAccountIds.value)
  selectedAccountSet.value = new Set(selectedAccountIds.value.filter((id) => visible.has(id)))
}

function prepareBulkSchedulable(schedulable: boolean) {
  if (selectedAccountIds.value.length === 0) {
    appStore.showWarning('请先选择本地账号')
    return
  }
  void prepareAction({
    action: 'set_schedulable',
    account_ids: selectedAccountIds.value,
    schedulable
  })
}

function prepareRowSchedulable(row: LocalAccountOpsRow, schedulable: boolean) {
  void prepareAction({
    action: 'set_schedulable',
    account_ids: [row.local_sub2api_account_id],
    schedulable
  })
}

function prepareBulkGroupAction(action: Extract<LocalAccountOpsAction, 'add_to_groups' | 'remove_from_groups'>) {
  if (selectedAccountIds.value.length === 0) {
    appStore.showWarning('请先选择本地账号')
    return
  }
  if (selectedGroupId.value <= 0) {
    appStore.showWarning('请先选择目标本地分组')
    return
  }
  void prepareAction({
    action,
    account_ids: selectedAccountIds.value,
    group_ids: [selectedGroupId.value]
  })
}

function prepareRowGroupAction(row: LocalAccountOpsRow, action: Extract<LocalAccountOpsAction, 'add_to_groups' | 'remove_from_groups'>) {
  if (selectedGroupId.value <= 0) {
    appStore.showWarning('请先选择目标本地分组')
    return
  }
  void prepareAction({
    action,
    account_ids: [row.local_sub2api_account_id],
    group_ids: [selectedGroupId.value]
  })
}

async function prepareAction(payload: LocalAccountOpsActionPayload) {
  pendingPayload.value = { ...payload, account_ids: uniqueNumbers(payload.account_ids), group_ids: uniqueNumbers(payload.group_ids || []) }
  previewResult.value = null
  actionAppliedResult.value = null
  actionDialogOpen.value = true
  actionBusy.value = true
  try {
    previewResult.value = await previewLocalAccountOpsAction(pendingPayload.value)
  } catch (error) {
    actionDialogOpen.value = false
    pendingPayload.value = null
    appStore.showError((error as { message?: string }).message || '预览本地账号操作失败')
  } finally {
    actionBusy.value = false
  }
}

async function applyPendingAction() {
  if (!pendingPayload.value || !previewResult.value || previewResult.value.blocked) return
  actionBusy.value = true
  try {
    const result = await applyLocalAccountOpsAction(pendingPayload.value)
    if (result.blocked) {
      previewResult.value = result
      actionAppliedResult.value = result
      appStore.showWarning(blockedApplyMessage(result.blocked_reason))
      return
    }
    previewResult.value = result
    actionAppliedResult.value = result
    appStore.showSuccess(actionSuccessMessage(result))
    clearSelection()
    await loadRows()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '执行本地账号操作失败')
  } finally {
    actionBusy.value = false
  }
}

async function previewRoutingRefill() {
  await runRoutingRefill(true)
}

async function applyRoutingRefill() {
  await runRoutingRefill(false)
}

async function runRoutingRefill(dryRun: boolean) {
  if (selectedGroupId.value <= 0) {
    appStore.showWarning('请先选择目标本地分组')
    return
  }
  const maxRateMultiplier = parsedMaxRateMultiplier()
  if (filters.max_rate_multiplier.trim() && maxRateMultiplier === undefined) {
    appStore.showWarning('最高倍率必须是大于 0 的数字')
    return
  }
  refillBusy.value = true
  try {
    const result = await refillLocalGroup({
      local_group_id: selectedGroupId.value,
      max_rate_multiplier: maxRateMultiplier,
      limit: 1000,
      dry_run: dryRun,
      reason: 'manual_local_account_ops'
    })
    refillResult.value = result
    if (result.skipped_reason) {
      appStore.showWarning(routingRefillSkippedReasonLabel(result.skipped_reason))
      return
    }
    if (dryRun) {
      appStore.showSuccess('已生成补池预览')
      return
    }
    appStore.showSuccess('已补入最低倍率候选')
    await loadRows()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '补池操作失败')
  } finally {
    refillBusy.value = false
  }
}

async function syncLocalState() {
  const accountIds = syncTargetAccountIds.value
  if (accountIds.length === 0) {
    appStore.showWarning('当前页没有可同步的本地账号')
    return
  }
  syncBusy.value = true
  try {
    const result = await syncLocalAccountState({
      account_ids: accountIds,
      limit: accountIds.length
    })
    if (result.pending_drift_accounts > 0) {
      appStore.showWarning(`发现 ${result.pending_drift_accounts} 个原后台手工变更，已标记为待处理`)
    } else {
      appStore.showSuccess(`已同步 ${result.checked_accounts} 个本地账号状态`)
    }
    await loadRows()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '同步本地账号状态失败')
  } finally {
    syncBusy.value = false
  }
}

async function openDriftDialog(row: LocalAccountOpsRow) {
  driftDialogOpen.value = true
  driftAccountId.value = row.local_sub2api_account_id
  driftSummary.value = null
  driftBusy.value = true
  try {
    const result = await syncLocalAccountState({
      account_ids: [row.local_sub2api_account_id],
      limit: 1
    })
    driftSummary.value = result.items?.find((item) => item.local_sub2api_account_id === row.local_sub2api_account_id) || null
    if (!driftSummary.value) {
      appStore.showSuccess('本地账号状态已同步，未发现待处理变更')
      await loadRows()
    }
  } catch (error) {
    driftDialogOpen.value = false
    appStore.showError((error as { message?: string }).message || '读取本地状态差异失败')
  } finally {
    driftBusy.value = false
  }
}

async function acceptDrift() {
  if (!driftAccountId.value) return
  driftActionBusy.value = true
  try {
    const result = await acceptLocalAccountState({ account_ids: [driftAccountId.value] })
    appStore.showSuccess(`已采纳 ${result.resolved_accounts} 个原后台变更`)
    closeDriftDialog(true)
    await loadRows()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '采纳原后台变更失败')
  } finally {
    driftActionBusy.value = false
  }
}

async function restoreDrift() {
  if (!driftAccountId.value) return
  driftActionBusy.value = true
  try {
    const result = await restoreLocalAccountState({ account_ids: [driftAccountId.value] })
    appStore.showSuccess(`已恢复 ${result.restored_accounts} 个本地账号基线，并刷新调度队列`)
    closeDriftDialog(true)
    await loadRows()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '恢复 Admin Plus 基线失败')
  } finally {
    driftActionBusy.value = false
  }
}

function closeDriftDialog(force = false) {
  if (!force && (driftBusy.value || driftActionBusy.value)) return
  driftDialogOpen.value = false
  driftSummary.value = null
  driftAccountId.value = 0
}

function openPurityDialog(row: LocalAccountOpsRow) {
  if (!supportsPurity(row)) {
    appStore.showError('仅支持 OpenAI API Key 账号执行纯度检测')
    return
  }
  purityAccount.value = localAccountFromOpsRow(row)
}

function closePurityDialog() {
  purityAccount.value = null
}

function supportsPurity(row?: LocalAccountOpsRow): boolean {
  const platform = String(row?.local_account_platform || '').toLowerCase()
  const type = String(row?.local_account_type || '').toLowerCase()
  return platform === 'openai' && type === 'apikey'
}

function localAccountFromOpsRow(row: LocalAccountOpsRow): LocalSub2APIAccount {
  return {
    id: row.local_sub2api_account_id,
    name: row.local_account_name || `#${row.local_sub2api_account_id}`,
    platform: row.local_account_platform,
    type: row.local_account_type,
    status: row.local_account_status,
    error_message: row.local_account_error_message,
    schedulable: row.local_account_schedulable,
    concurrency: row.local_account_concurrency,
    priority: row.local_account_priority,
    rate_multiplier: row.local_account_rate_multiplier,
    rate_limited_at: row.local_account_rate_limited_at,
    rate_limit_reset_at: row.local_account_rate_limit_reset_at,
    overload_until: row.local_account_overload_until,
    temp_unschedulable_until: row.local_account_temp_unschedulable_until,
    temp_unschedulable_reason: row.local_account_temp_unschedulable_reason,
    auto_pause_on_expired: false,
    created_at: row.local_account_updated_at,
    updated_at: row.local_account_updated_at,
    group_ids: row.local_account_group_ids || [],
    group_names: row.local_account_group_names || []
  }
}

function closeActionDialog() {
  if (actionBusy.value) return
  resetActionDialog()
}

function resetActionDialog() {
  actionDialogOpen.value = false
  pendingPayload.value = null
  previewResult.value = null
  actionAppliedResult.value = null
}

function actionTitle(payload: LocalAccountOpsActionPayload): string {
  if (payload.action === 'set_schedulable') return payload.schedulable ? '开启本地账号调度' : '关闭本地账号调度'
  if (payload.action === 'add_to_groups') return `加入本地分组：${selectedGroupName(payload.group_ids?.[0])}`
  if (payload.action === 'remove_from_groups') return `移出本地分组：${selectedGroupName(payload.group_ids?.[0])}`
  return '账号调度操作'
}

function actionSuccessMessage(result: LocalAccountOpsActionResult): string {
  if (result.action === 'set_schedulable') return `已更新 ${result.updated_accounts} 个本地账号，并刷新调度队列`
  if (result.action === 'add_to_groups') return `已新增 ${result.added_bindings} 条账号分组绑定，并刷新调度队列`
  if (result.action === 'remove_from_groups') return `已移除 ${result.removed_bindings} 条账号分组绑定，并刷新调度队列`
  return '本地账号操作已提交'
}

function actionResultSummary(result: LocalAccountOpsActionResult): string {
  if (result.blocked) {
    const base = blockedApplyMessage(result.blocked_reason)
    return result.action_execution_id ? `${base}，执行记录 #${result.action_execution_id}` : base
  }
  const base = actionSuccessMessage(result)
  if (result.action_recommendation_id && result.action_execution_id) {
    return `${base}，执行记录 #${result.action_execution_id}`
  }
  return base
}

function actionExecutionPath(result: LocalAccountOpsActionResult | null): string {
  if (!result?.action_recommendation_id || !result.action_execution_id) return ''
  const params = new URLSearchParams({
    type: 'local_account_manual_ops',
    recommendation_id: String(result.action_recommendation_id),
    execution_id: String(result.action_execution_id)
  })
  return `/admin/actions?${params.toString()}`
}

function blockedApplyMessage(reason?: string): string {
  if (reason === 'LOCAL_ACCOUNT_STATE_DRIFT_PENDING') return '检测到原后台手工变更，操作已被保护拦截'
  if (reason === 'LOCAL_GROUP_SCHEDULABLE_POOL_WOULD_BE_EMPTY') return '操作被空池保护拦截'
  return '操作被安全保护拦截'
}

function blockedReasonTitle(reason?: string): string {
  if (reason === 'LOCAL_ACCOUNT_STATE_DRIFT_PENDING') return '检测到原后台手工变更'
  if (reason === 'LOCAL_GROUP_SCHEDULABLE_POOL_WOULD_BE_EMPTY') return '本次操作已被空池保护拦截'
  return '本次操作已被安全保护拦截'
}

function blockedReasonMessage(reason?: string): string {
  if (reason === 'LOCAL_ACCOUNT_STATE_DRIFT_PENDING') return '请先同步本地状态，确认采纳或恢复原后台变更后再执行调度操作。'
  if (reason === 'LOCAL_GROUP_SCHEDULABLE_POOL_WOULD_BE_EMPTY') return '存在本地分组仍有启用 API Key，但执行后没有可调度账号。请先加入替代账号或关闭对应 API Key。'
  return '请检查账号、分组和供应商绑定状态后重试。'
}

function selectedGroupName(groupId?: number): string {
  const group = localGroupOptions.value.find((item) => item.id === groupId)
  return group?.name || (groupId ? `#${groupId}` : '-')
}

function snapshotFieldValue(summary: LocalAccountStateDriftSummary, side: 'accepted' | 'observed', key: string): string {
  const snapshot = summary[side]
  if (key === 'name') return snapshot.name || '-'
  if (key === 'platform') return snapshot.platform || '-'
  if (key === 'type') return snapshot.type || '-'
  if (key === 'schedulable') return snapshot.schedulable ? '已开启' : '已关闭'
  if (key === 'groups') return formatGroupIDs(snapshot.group_ids || [])
  return '-'
}

function formatGroupIDs(groupIds: number[]): string {
  if (groupIds.length === 0) return '-'
  return groupIds.map((id) => selectedGroupName(id)).join(', ')
}

async function openSub2APIAccount(row: LocalAccountOpsRow) {
  const accountId = row.local_sub2api_account_id
  try {
    await copyText(String(accountId))
    appStore.showSuccess(`已复制本地账号 ID #${accountId}`)
  } catch {
    appStore.showWarning(`请在原后台搜索本地账号 ID #${accountId}`)
  }
  window.open(sub2APIAccountsURL(accountId), '_blank', 'noopener,noreferrer')
}

function openAccountAudit(row: LocalAccountOpsRow) {
  void router.push({
    path: '/admin/action-audits',
    query: {
      component: 'admin_plus.sub2api',
      q: String(row.local_sub2api_account_id),
      window: '24h'
    }
  })
}

async function copyText(value: string) {
  if (!navigator.clipboard) throw new Error('clipboard unavailable')
  await navigator.clipboard.writeText(value)
}

function sub2APIAccountsURL(accountId: number): string {
  const url = new URL('/admin/accounts', window.location.origin)
  url.searchParams.set('q', String(accountId))
  return url.toString()
}

function supplierGroupLabel(group: SupplierGroup): string {
  const supplierName = suppliersById.value.get(group.supplier_id)?.name || `供应商 #${group.supplier_id}`
  const parts = [
    supplierName,
    group.name || `#${group.id}`,
    formatRate(group.effective_rate_multiplier || group.rate_multiplier || 0)
  ]
  if (group.status && group.status !== 'active') parts.push(groupStatusLabel(group.status))
  return parts.join(' / ')
}

function uniqueNumbers(values: number[]): number[] {
  return Array.from(new Set(values.filter((value) => Number.isFinite(value) && value > 0))).sort((a, b) => a - b)
}

function rowKey(row: LocalAccountOpsRow): string {
  return `${row.local_sub2api_account_id}-${row.supplier_account_id || 0}-${row.supplier_key_id || 0}`
}

function visibleLocalGroups(row: LocalAccountOpsRow): string[] {
  return (row.local_account_group_names || []).slice(0, 4)
}

function rowNeedsAction(row: LocalAccountOpsRow): boolean {
  if (row.drift_status && !['synced', 'unbound'].includes(row.drift_status)) return true
  if (['fail', 'warn'].includes(String(row.purity_status || '').toLowerCase())) return true
  if (purityIsStale(row)) return true
  if (proxyNeedsAction(row.local_account_proxy_status)) return true
  if (['insufficient', 'unknown'].includes(row.balance_status) && row.drift_status !== 'unbound') return true
  return ['request_error', 'remote_unavailable', 'probe_failed', 'slow_first_token', 'slow_total'].includes(row.channel_check_status)
}

function formatRate(value: number): string {
  return `${Number(value || 0).toFixed(3)}x`
}

function formatMoney(cents: number, currency = 'USD'): string {
  return new Intl.NumberFormat(undefined, {
    style: 'currency',
    currency: currency || 'USD',
    currencyDisplay: 'narrowSymbol',
    minimumFractionDigits: 2
  }).format((cents || 0) / 100)
}

function formatDateTime(value?: string | null): string {
  if (!value) return '-'
  return new Intl.DateTimeFormat(undefined, {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  }).format(new Date(value))
}

function runtimeStatusLabel(value?: string): string {
  return {
    monitor_only: '观察',
    candidate: '候选',
    active: '启用',
    disabled: '禁用'
  }[value || ''] || value || '-'
}

function runtimeStatusClass(value?: string): string {
  if (value === 'active' || value === 'candidate') return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
  if (value === 'disabled') return 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-dark-200'
  return 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300'
}

function keyStatusLabel(value?: string): string {
  return {
    provisioning: '创建中',
    bound: '已绑定',
    manual_secret_required: '需密钥',
    failed: '失败',
    disabled: '禁用'
  }[value || ''] || value || '-'
}

function keyStatusClass(value?: string): string {
  if (value === 'bound') return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
  if (value === 'failed' || value === 'manual_secret_required') return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
  return 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-dark-200'
}

function groupStatusLabel(value?: string): string {
  return { active: '可用', missing: '缺失', disabled: '禁用' }[value || ''] || value || '-'
}

function groupStatusClass(value?: string): string {
  if (value === 'active') return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
  if (value === 'missing') return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
  return 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-dark-200'
}

function balanceStatusLabel(value?: string): string {
  return {
    unbound: '未绑定',
    usable: '可用',
    insufficient: '不足',
    unknown: '未知'
  }[value || ''] || value || '-'
}

function balanceStatusClass(value?: string): string {
  if (value === 'usable') return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
  if (value === 'insufficient') return 'bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-300'
  if (value === 'unbound') return 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-dark-200'
  return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
}

function channelStatusLabel(value?: string): string {
  return {
    untested: '未检测',
    available: '可用',
    slow_first_token: '首 token 慢',
    slow_total: '总耗时慢',
    request_error: '请求错误',
    remote_unavailable: '远端不可用',
    no_local_account: '无本地账号',
    probe_failed: '检测失败'
  }[value || ''] || value || '-'
}

function channelStatusClass(value?: string): string {
  if (value === 'available') return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
  if (value === 'untested') return 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-dark-200'
  return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
}

function candidateStatusLabel(value?: string): string {
  return {
    available: '可调度候选',
    unknown: '待确认',
    degraded: '质量降级',
    needs_provisioning: '待开通',
    balance_blocked: '余额阻断',
    blocked: '不可用',
    local_blocked: '本地阻断',
    capacity_blocked: '配额阻断'
  }[value || ''] || value || '-'
}

function candidateStatusClass(value?: string): string {
  if (value === 'available') return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
  if (value === 'balance_blocked' || value === 'capacity_blocked' || value === 'blocked') return 'bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-300'
  if (value === 'needs_provisioning' || value === 'unknown' || value === 'degraded') return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
  return 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-dark-200'
}

function checkSourceLabel(value?: string): string {
  return {
    supplier: '供应商',
    supplier_group: '第三方分组',
    supplier_key: '第三方 Key',
    key_capacity: 'Key 配额',
    balance: '余额',
    local_state: '本地状态',
    channel_monitor: '通道监控',
    active_probe: '实测',
    purity: '纯度检测',
    proxy: '代理'
  }[value || ''] || value || '-'
}

function blockedReasonLabel(value?: string): string {
  return {
    recharge_required: '余额不足，充值后重测',
    balance_unknown: '余额未知，需确认',
    channel_monitor_failed: '通道监控不可用',
    channel_active_probe_failed: '实测失败',
    channel_untested: '尚未检测通道',
    supplier_binding_missing: '缺少供应商绑定',
    supplier_disabled: '供应商已禁用',
    supplier_unavailable: '供应商不可用',
    supplier_credential_invalid: '供应商凭据失效',
    supplier_paused: '供应商已暂停',
    supplier_account_disabled: '供应商账号禁用',
    supplier_group_missing: '第三方分组缺失',
    supplier_group_disabled: '第三方分组禁用',
    supplier_key_missing: '缺少第三方 Key',
    supplier_key_failed: '第三方 Key 失败',
    supplier_key_disabled: '第三方 Key 禁用',
    supplier_key_manual_secret_required: '第三方 Key 需补密钥',
    supplier_key_provisioning: '第三方 Key 创建中',
    key_capacity_exhausted: 'Key 配额已满',
    local_account_missing: '缺少本地账号',
    local_account_unschedulable: '本地账号已关调度',
    local_account_temp_unschedulable: '本地账号临时不可调度',
    local_account_state_drift: '原后台变更待处理',
    local_account_metadata_drift: '本地账号元数据漂移',
    key_local_account_mismatch: 'Key 绑定账号不一致',
    rate_missing: '倍率缺失',
    purity_failed: '纯度检测失败',
    purity_risk: '纯度检测有风险',
    purity_stale: '纯度检测已过期',
    proxy_deleted: '代理已删除',
    proxy_disabled: '代理已禁用',
    proxy_expired: '代理已过期',
    proxy_unavailable: '代理不可用'
  }[value || ''] || value || '-'
}

function showPurityBadge(row: LocalAccountOpsRow): boolean {
  const status = String(row.purity_status || '').toLowerCase()
  return Boolean(purityIsStale(row) || row.purity_checked_at || (status && status !== 'unknown'))
}

function purityIsStale(row?: LocalAccountOpsRow): boolean {
  return String(row?.purity_freshness_status || '').toLowerCase() === 'stale'
}

function purityBadgeLabel(row: LocalAccountOpsRow): string {
  if (purityIsStale(row)) return '纯度已过期'
  return purityStatusLabel(row.purity_status)
}

function purityBadgeClass(row: LocalAccountOpsRow): string {
  if (purityIsStale(row)) return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
  return purityStatusClass(row.purity_status)
}

function purityStatusLabel(value?: string): string {
  return {
    pass: '纯度通过',
    warn: '纯度风险',
    fail: '纯度失败',
    unknown: '纯度未知'
  }[value || ''] || value || '-'
}

function purityStatusClass(value?: string): string {
  if (value === 'pass') return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
  if (value === 'fail') return 'bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-300'
  if (value === 'warn') return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
  return 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-dark-200'
}

function purityVerdictLabel(value?: string): string {
  return {
    official_openai: '官方 OpenAI',
    openai_compatible: 'OpenAI 兼容',
    official_claude: '官方 Claude',
    claude_compatible: 'Claude 兼容',
    official_gemini: '官方 Gemini',
    gemini_compatible: 'Gemini 兼容',
    partial_compatible: '部分兼容',
    invalid_or_unavailable: '无效或不可用',
    unknown: '未知'
  }[value || ''] || value || ''
}

function puritySummary(row: LocalAccountOpsRow): string {
  const parts: string[] = []
  if (purityIsStale(row)) parts.push('已过期')
  if (row.purity_model) parts.push(row.purity_model)
  const verdict = purityVerdictLabel(row.purity_verdict)
  if (verdict) parts.push(verdict)
  if (Number(row.purity_score || 0) > 0) parts.push(`${row.purity_score}分`)
  if (row.purity_checked_at) parts.push(formatDateTime(row.purity_checked_at))
  return parts.length ? parts.join(' · ') : purityStatusLabel(row.purity_status)
}

function purityTitle(row: LocalAccountOpsRow): string {
  const report = row.purity_report_id ? `报告 ${row.purity_report_id}` : '最近一次纯度检测'
  return `${report}：${puritySummary(row)}`
}

function showProxyBadge(row: LocalAccountOpsRow): boolean {
  return Boolean(row.local_account_proxy_id && String(row.local_account_proxy_status || 'unbound').toLowerCase() !== 'unbound')
}

function proxyStatusLabel(value?: string): string {
  return {
    active: '代理正常',
    disabled: '代理禁用',
    expired: '代理过期',
    error: '代理异常',
    deleted: '代理删除',
    missing: '代理缺失',
    unavailable: '代理不可用',
    unknown: '代理未知'
  }[String(value || '').toLowerCase()] || value || '代理未知'
}

function proxyStatusClass(value?: string): string {
  const status = String(value || '').toLowerCase()
  if (status === 'active') return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
  if (status === 'unknown') return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
  return 'bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-300'
}

function proxyNeedsAction(value?: string): boolean {
  return ['deleted', 'missing', 'disabled', 'expired', 'error', 'unavailable'].includes(String(value || '').toLowerCase())
}

function proxySummary(row: LocalAccountOpsRow): string {
  const parts = [row.local_account_proxy_name || `代理 #${row.local_account_proxy_id}`]
  if (row.local_account_proxy_expires_at) parts.push(`到期 ${formatDateTime(row.local_account_proxy_expires_at)}`)
  return parts.join(' · ')
}

function proxyTitle(row: LocalAccountOpsRow): string {
  return `${proxyStatusLabel(row.local_account_proxy_status)}：${proxySummary(row)}`
}

function driftStatusLabel(value?: string): string {
  return {
    unbound: '未绑定',
    synced: '一致',
    supplier_disabled: '供应商禁用',
    binding_disabled: '绑定禁用',
    missing_key: '缺 Key',
    key_local_account_mismatch: 'Key 错绑',
    missing_group: '缺分组',
    group_missing: '分组缺失',
    group_disabled: '分组禁用',
    local_account_metadata_drift: '账号漂移',
    local_account_state_drift: '原后台变更',
    unknown: '未知'
  }[value || ''] || value || '-'
}

function driftStatusClass(value?: string): string {
  if (value === 'synced') return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
  if (value === 'unbound') return 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-dark-200'
  if (value === 'local_account_state_drift') return 'bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-300'
  return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
}

watch(
  () => route.query,
  () => {
    applyRouteFilters()
    pagination.page = 1
    void loadRows()
  }
)

onMounted(() => {
  applyRouteFilters()
  void loadRows()
})
</script>
