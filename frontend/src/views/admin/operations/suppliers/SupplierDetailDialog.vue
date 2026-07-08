<template>
<BaseDialog
  :show="supplierDetailDialogOpen"
  :title="supplierDetailSupplier ? `供应商详情 - ${supplierDetailSupplier.name}` : '供应商详情'"
  width="full"
  @close="closeSupplierDetailDialog"
>
  <div class="space-y-5">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div class="min-w-0">
        <div class="flex min-w-0 flex-wrap items-center gap-2">
          <span v-if="supplierDetailSupplier" class="truncate text-lg font-semibold text-gray-900 dark:text-white">
            {{ supplierDetailSupplier.name }}
          </span>
          <span v-if="supplierDetailSupplier" class="badge" :class="runtimeClass(supplierDetailSupplier.runtime_status)">
            {{ runtimeLabel(supplierDetailSupplier.runtime_status) }}
          </span>
          <span v-if="supplierDetailSupplier" class="badge" :class="healthClass(supplierDetailSupplier.health_status)">
            {{ healthLabel(supplierDetailSupplier.health_status) }}
          </span>
          <span v-if="supplierDetailSupplier" class="badge badge-gray">
            {{ typeLabel(supplierDetailSupplier.type) }}
          </span>
        </div>
        <div v-if="supplierDetailSupplier" class="mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-gray-500 dark:text-dark-400">
          <span class="font-mono">#{{ supplierDetailSupplier.id }}</span>
          <span>充值倍率 {{ formatMultiplier(supplierDetailSupplier.recharge_multiplier || 1) }}</span>
          <span v-if="supplierDetailSupplier.balance_updated_at">余额更新时间 {{ formatDateTime(supplierDetailSupplier.balance_updated_at) }}</span>
        </div>
      </div>
      <div class="flex flex-wrap items-center gap-2">
        <RouterLink
          v-if="supplierDetailSupplier"
          :to="supplierAuditRoute"
          class="btn btn-secondary"
          title="查看该供应商相关操作审计"
        >
          <Icon name="clipboard" size="sm" />
          操作审计
        </RouterLink>
        <button type="button" class="btn btn-secondary" :disabled="supplierDetailLoading || !supplierDetailSupplier" @click="loadSupplierDetail">
          <Icon name="refresh" size="sm" :class="{ 'animate-spin': supplierDetailLoading }" />
          刷新
        </button>
      </div>
    </div>

    <div v-if="supplierDetailError" class="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-200">
      {{ supplierDetailError }}
    </div>

    <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-6">
      <div v-for="item in detailStatItems" :key="item.key" class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
        <div class="flex items-center justify-between gap-2">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">{{ item.label }}</p>
          <Icon :name="item.icon" size="sm" class="text-gray-400" />
        </div>
        <p class="mt-2 text-2xl font-semibold" :class="item.valueClass">{{ item.value }}</p>
        <p class="mt-1 truncate text-xs text-gray-500 dark:text-dark-400" :title="item.caption">{{ item.caption }}</p>
      </div>
    </div>

    <section class="rounded-lg border border-amber-200 bg-amber-50 p-4 dark:border-amber-900/60 dark:bg-amber-950/20">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h3 class="text-sm font-semibold text-amber-950 dark:text-amber-100">Key 配额风险</h3>
          <p class="mt-1 text-sm text-amber-800 dark:text-amber-200">
            配额策略来自供应商配置，已用数量按本地已同步 Key 派生；未知策略会阻止盲目一键开通。
          </p>
        </div>
        <span class="badge" :class="quotaRiskClass">{{ quotaRiskLabel }}</span>
      </div>
      <div class="mt-3 grid gap-3 text-sm md:grid-cols-6">
        <div>
          <span class="block text-xs text-amber-700 dark:text-amber-300">配额策略</span>
          <span class="font-semibold text-amber-950 dark:text-amber-100">{{ keyLimitPolicyLabel(supplierDetailSupplier?.key_limit_policy) }}</span>
        </div>
        <div>
          <span class="block text-xs text-amber-700 dark:text-amber-300">容量状态</span>
          <span class="font-semibold text-amber-950 dark:text-amber-100">{{ keyCapacityStatusLabel(supplierDetailSupplier?.key_capacity_status) }}</span>
        </div>
        <div>
          <span class="block text-xs text-amber-700 dark:text-amber-300">已创建 Key</span>
          <span class="font-semibold text-amber-950 dark:text-amber-100">{{ keyUsageLabel }}</span>
        </div>
        <div>
          <span class="block text-xs text-amber-700 dark:text-amber-300">已绑定 Key</span>
          <span class="font-semibold text-amber-950 dark:text-amber-100">{{ detailStats.boundKeys }}</span>
        </div>
        <div>
          <span class="block text-xs text-amber-700 dark:text-amber-300">需补密钥/失败</span>
          <span class="font-semibold text-amber-950 dark:text-amber-100">{{ detailStats.keyActionRequired }}</span>
        </div>
        <div>
          <span class="block text-xs text-amber-700 dark:text-amber-300">阻塞有效分组</span>
          <span class="font-semibold text-amber-950 dark:text-amber-100">{{ blockedGroups.length }}</span>
        </div>
      </div>
      <div v-if="blockedGroups.length > 0" class="mt-3 flex flex-wrap gap-2">
        <span v-for="group in blockedGroups.slice(0, 10)" :key="group.id" class="badge badge-warning" :title="group.name">
          {{ shortText(group.name, 18) }}
        </span>
        <span v-if="blockedGroups.length > 10" class="badge badge-gray">+{{ blockedGroups.length - 10 }}</span>
      </div>
    </section>

    <section class="rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800">
      <div class="flex flex-wrap items-center justify-between gap-2 border-b border-gray-100 px-4 py-3 dark:border-dark-700">
        <div>
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">第三方分组覆盖</h3>
          <p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">分组、倍率、Key、本地账号、余额、通道检测和漂移状态。</p>
        </div>
        <span class="badge badge-gray">{{ sortedDetailGroups.length }} 个分组</span>
      </div>
      <div class="overflow-x-auto">
        <table class="w-full min-w-[1320px] divide-y divide-gray-200 text-sm dark:divide-dark-700">
          <thead class="bg-gray-50 dark:bg-dark-900/60">
            <tr>
              <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">第三方分组</th>
              <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">倍率</th>
              <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">Key</th>
              <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">本地账号</th>
              <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">余额</th>
              <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">通道检测</th>
              <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">漂移/状态</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-200 dark:divide-dark-700">
            <tr v-if="!supplierDetailLoading && sortedDetailGroups.length === 0">
              <td colspan="7" class="px-4 py-8 text-center text-sm text-gray-500 dark:text-dark-400">暂无第三方分组</td>
            </tr>
            <tr v-for="group in sortedDetailGroups" :key="group.id" class="align-top">
              <td class="px-4 py-3">
                <div class="max-w-[260px]">
                  <div class="flex min-w-0 items-center gap-2">
                    <span class="truncate font-medium text-gray-900 dark:text-gray-100" :title="group.name">{{ group.name }}</span>
                    <span class="badge shrink-0" :class="groupStatusClass(group.status)">{{ groupStatusLabel(group.status) }}</span>
                  </div>
                  <div class="mt-1 flex flex-wrap gap-2 text-xs text-gray-500 dark:text-dark-400">
                    <span class="font-mono">#{{ group.external_group_id || group.id }}</span>
                    <span>{{ group.provider_family || 'mixed' }}</span>
                  </div>
                  <div v-if="group.description" class="mt-1 truncate text-xs text-gray-500 dark:text-dark-400" :title="group.description">
                    {{ group.description }}
                  </div>
                </div>
              </td>
              <td class="px-4 py-3">
                <div class="space-y-1">
                  <span :class="rateMultiplierTextClass(groupDetailCostMultiplier(group), channelProtocolFromProviderFamily(group.provider_family, group.name, group.description), 'compact')">
                    {{ formatMultiplier(groupDetailCostMultiplier(group)) }}
                  </span>
                  <div class="text-xs text-gray-500 dark:text-dark-400">
                    使用 {{ formatMultiplier(group.effective_rate_multiplier) }}
                  </div>
                </div>
              </td>
              <td class="px-4 py-3">
                <div class="max-w-[220px]">
                  <template v-if="detailGroupKey(group)">
                    <div class="flex min-w-0 items-center gap-2">
                      <span class="truncate font-medium text-gray-900 dark:text-gray-100" :title="detailGroupKey(group)?.name">{{ detailGroupKey(group)?.name }}</span>
                      <span class="badge shrink-0" :class="supplierKeyStatusClass(detailGroupKey(group)?.status)">
                        {{ supplierKeyStatusLabel(detailGroupKey(group)?.status) }}
                      </span>
                    </div>
                    <div class="mt-1 flex flex-wrap gap-2 text-xs text-gray-500 dark:text-dark-400">
                      <span v-if="detailGroupKey(group)?.external_key_id" class="font-mono">Key #{{ detailGroupKey(group)?.external_key_id }}</span>
                      <span v-if="detailGroupKey(group)?.key_last4" class="font-mono">****{{ detailGroupKey(group)?.key_last4 }}</span>
                    </div>
                    <div v-if="detailGroupKey(group)?.error_message" class="mt-1 truncate text-xs text-red-600 dark:text-red-300" :title="detailGroupKey(group)?.error_message">
                      {{ detailGroupKey(group)?.error_message }}
                    </div>
                  </template>
                  <span v-else class="badge badge-warning">未开通</span>
                </div>
              </td>
              <td class="px-4 py-3">
                <div class="max-w-[260px]">
                  <template v-if="groupLocalAccount(group)">
                    <div class="flex min-w-0 items-center gap-2">
                      <span class="truncate font-medium text-gray-900 dark:text-gray-100" :title="groupLocalAccount(group)?.name">
                        {{ groupLocalAccount(group)?.name || `账号 #${groupLocalAccountID(group)}` }}
                      </span>
                      <span class="badge shrink-0" :class="groupLocalAccount(group)?.schedulable ? 'badge-success' : 'badge-warning'">
                        {{ groupLocalAccount(group)?.schedulable ? '调度开启' : '调度关闭' }}
                      </span>
                    </div>
                    <div class="mt-1 flex flex-wrap gap-1">
                      <span v-for="name in localGroupNames(groupLocalAccount(group)).slice(0, 3)" :key="name" class="badge badge-gray" :title="name">
                        {{ shortText(name, 18) }}
                      </span>
                      <span v-if="localGroupNames(groupLocalAccount(group)).length > 3" class="badge badge-gray">
                        +{{ localGroupNames(groupLocalAccount(group)).length - 3 }}
                      </span>
                    </div>
                  </template>
                  <span v-else class="badge badge-gray">未绑定本地账号</span>
                </div>
              </td>
              <td class="px-4 py-3">
                <span class="badge" :class="groupBalanceClass(group)">{{ groupBalanceLabel(group) }}</span>
                <div v-if="groupSupplierAccount(group)" class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                  阈值 {{ formatMoney(groupSupplierAccount(group)?.balance_threshold_cents || 0, groupSupplierAccount(group)?.balance_currency || 'USD') }}
                </div>
              </td>
              <td class="px-4 py-3">
                <template v-if="detailChannelCheck(group)">
                  <div class="flex flex-wrap items-center gap-1.5">
                    <span class="badge" :class="channelProbeStatusClass(detailChannelCheck(group)?.probe_status)">
                      {{ channelProbeStatusLabel(detailChannelCheck(group)?.probe_status) }}
                    </span>
                    <span class="badge" :class="detailChannelCheck(group)?.local_account_schedulable ? 'badge-success' : 'badge-warning'">
                      {{ detailChannelCheck(group)?.local_account_schedulable ? '调度中' : '未调度' }}
                    </span>
                  </div>
                  <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                    首 {{ formatLatency(detailChannelCheck(group)?.first_token_ms) }} · 总 {{ formatLatency(detailChannelCheck(group)?.duration_ms) }}
                  </div>
                  <div class="mt-1 max-w-[240px] truncate text-xs text-gray-500 dark:text-dark-400" :title="detailChannelCheck(group)?.error_message || ''">
                    {{ detailChannelCheck(group)?.error_message || formatDateTime(detailChannelCheck(group)?.captured_at) }}
                  </div>
                </template>
                <span v-else class="badge badge-gray">未检测</span>
              </td>
              <td class="px-4 py-3">
                <div class="flex flex-wrap gap-1.5">
                  <span v-if="groupOpsRow(group)?.candidate_status" class="badge" :class="candidateStatusClass(groupOpsRow(group)?.candidate_status)">
                    {{ candidateStatusLabel(groupOpsRow(group)?.candidate_status) }}
                  </span>
                  <span class="badge" :class="driftStatusClass(groupOpsRow(group)?.drift_status)">
                    {{ driftStatusLabel(groupOpsRow(group)?.drift_status) }}
                  </span>
                  <span v-if="groupOpsRow(group)?.balance_status" class="badge" :class="opsBalanceStatusClass(groupOpsRow(group)?.balance_status)">
                    {{ opsBalanceStatusLabel(groupOpsRow(group)?.balance_status) }}
                  </span>
                </div>
                <div v-if="groupOpsRow(group)?.blocked_reason" class="mt-1 max-w-[240px] truncate text-xs text-amber-600 dark:text-amber-300" :title="candidateReasonLabel(groupOpsRow(group)?.blocked_reason)">
                  {{ candidateReasonLabel(groupOpsRow(group)?.blocked_reason) }}
                  <span v-if="groupOpsRow(group)?.check_source"> · {{ checkSourceLabel(groupOpsRow(group)?.check_source) }}</span>
                </div>
                <div v-if="groupOpsRow(group)?.last_local_sync_at" class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                  同步 {{ formatDateTime(groupOpsRow(group)?.last_local_sync_at) }}
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <div class="grid gap-5 xl:grid-cols-2">
      <section class="rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800">
        <div class="flex items-center justify-between gap-2 border-b border-gray-100 px-4 py-3 dark:border-dark-700">
          <div>
            <h3 class="text-sm font-semibold text-gray-900 dark:text-white">第三方 Key</h3>
            <p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">external id、last4、绑定状态和人工密钥状态。</p>
          </div>
          <span class="badge badge-gray">{{ supplierDetailKeys.length }} 个 Key</span>
        </div>
        <div class="overflow-x-auto">
          <table class="w-full min-w-[920px] divide-y divide-gray-200 text-sm dark:divide-dark-700">
            <thead class="bg-gray-50 dark:bg-dark-900/60">
              <tr>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">Key</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">第三方分组</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">状态</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">本地绑定</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">错误</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-200 dark:divide-dark-700">
              <tr v-if="!supplierDetailLoading && supplierDetailKeys.length === 0">
                <td colspan="5" class="px-4 py-8 text-center text-sm text-gray-500 dark:text-dark-400">暂无 Key</td>
              </tr>
              <tr v-for="key in sortedDetailKeys" :key="key.id" class="align-top">
                <td class="px-4 py-3">
                  <div class="max-w-[240px]">
                    <div class="truncate font-medium text-gray-900 dark:text-gray-100" :title="key.name">{{ key.name || `Key #${key.id}` }}</div>
                    <div class="mt-1 flex flex-wrap gap-2 text-xs text-gray-500 dark:text-dark-400">
                      <span v-if="key.external_key_id" class="font-mono">Key #{{ key.external_key_id }}</span>
                      <span v-if="key.key_last4" class="font-mono">****{{ key.key_last4 }}</span>
                    </div>
                  </div>
                </td>
                <td class="px-4 py-3">
                  <div class="max-w-[220px]">
                    <div class="truncate text-gray-900 dark:text-gray-100" :title="keyGroupName(key)">{{ keyGroupName(key) }}</div>
                    <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ key.provider_family || key.external_group_id || '-' }}</div>
                  </div>
                </td>
                <td class="px-4 py-3">
                  <span class="badge" :class="supplierKeyStatusClass(key.status)">{{ supplierKeyStatusLabel(key.status) }}</span>
                </td>
                <td class="px-4 py-3">
                  <div v-if="key.local_sub2api_account_id" class="max-w-[220px]">
                    <div class="truncate text-gray-900 dark:text-gray-100" :title="key.local_account_name || ''">
                      {{ key.local_account_name || `账号 #${key.local_sub2api_account_id}` }}
                    </div>
                    <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">#{{ key.local_sub2api_account_id }} · {{ key.local_account_platform || '-' }}</div>
                  </div>
                  <span v-else class="badge badge-gray">未绑定</span>
                </td>
                <td class="px-4 py-3">
                  <div class="max-w-[240px] truncate text-xs text-red-600 dark:text-red-300" :title="key.error_message || ''">
                    {{ key.error_message || '-' }}
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section class="rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800">
        <div class="flex items-center justify-between gap-2 border-b border-gray-100 px-4 py-3 dark:border-dark-700">
          <div>
            <h3 class="text-sm font-semibold text-gray-900 dark:text-white">本地绑定</h3>
            <p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">本地账号、来源短标签、有效倍率、drift、调度和本地分组。</p>
          </div>
          <span class="badge badge-gray">{{ supplierDetailAccounts.length }} 个绑定</span>
        </div>
        <div class="overflow-x-auto">
          <table class="w-full min-w-[980px] divide-y divide-gray-200 text-sm dark:divide-dark-700">
            <thead class="bg-gray-50 dark:bg-dark-900/60">
              <tr>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">本地账号</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">第三方分组</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">倍率</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">调度/分组</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">余额/检测</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">漂移</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-200 dark:divide-dark-700">
              <tr v-if="!supplierDetailLoading && sortedDetailAccounts.length === 0">
                <td colspan="6" class="px-4 py-8 text-center text-sm text-gray-500 dark:text-dark-400">暂无本地绑定</td>
              </tr>
              <tr v-for="account in sortedDetailAccounts" :key="account.id" class="align-top">
                <td class="px-4 py-3">
                  <div class="max-w-[220px]">
                    <div class="truncate font-medium text-gray-900 dark:text-gray-100" :title="account.local_account_name">
                      {{ account.local_account_name || `账号 #${account.local_sub2api_account_id}` }}
                    </div>
                    <div class="mt-1 flex flex-wrap gap-2 text-xs text-gray-500 dark:text-dark-400">
                      <span class="font-mono">#{{ account.local_sub2api_account_id }}</span>
                      <span>{{ account.local_account_platform || '-' }}</span>
                      <span>{{ account.local_account_type || '-' }}</span>
                    </div>
                  </div>
                </td>
                <td class="px-4 py-3">
                  <div class="max-w-[220px]">
                    <div class="truncate text-gray-900 dark:text-gray-100" :title="account.supplier_group_name || ''">
                      {{ account.supplier_group_name || '未同步分组' }}
                    </div>
                    <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ account.supplier_group_provider || account.rate_profile || '-' }}</div>
                  </div>
                </td>
                <td class="px-4 py-3">
                  <span :class="rateMultiplierTextClass(accountDetailCostMultiplier(account), channelProtocolFromProviderFamily(account.supplier_group_provider, account.supplier_group_name), 'compact')">
                    {{ formatMultiplier(accountDetailCostMultiplier(account)) }}
                  </span>
                  <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">使用 {{ formatMultiplier(account.supplier_group_rate || accountDetailLocalRate(account)) }}</div>
                </td>
                <td class="px-4 py-3">
                  <span class="badge" :class="accountLocalAccount(account)?.schedulable ? 'badge-success' : 'badge-warning'">
                    {{ accountLocalAccount(account)?.schedulable ? '调度开启' : '调度关闭' }}
                  </span>
                  <div class="mt-1 flex max-w-[240px] flex-wrap gap-1">
                    <span v-for="name in localGroupNames(accountLocalAccount(account)).slice(0, 3)" :key="name" class="badge badge-gray" :title="name">
                      {{ shortText(name, 18) }}
                    </span>
                    <span v-if="localGroupNames(accountLocalAccount(account)).length === 0" class="text-xs text-gray-500 dark:text-dark-400">未分组</span>
                    <span v-if="localGroupNames(accountLocalAccount(account)).length > 3" class="badge badge-gray">
                      +{{ localGroupNames(accountLocalAccount(account)).length - 3 }}
                    </span>
                  </div>
                </td>
                <td class="px-4 py-3">
                  <span class="badge" :class="account.has_usable_balance ? 'badge-success' : 'badge-danger'">
                    {{ account.has_usable_balance ? '余额可用' : '余额不足' }}
                  </span>
                  <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                    {{ formatMoney(account.balance_cents || 0, account.balance_currency || 'USD') }}
                  </div>
                  <div v-if="accountChannelCheck(account)" class="mt-1">
                    <span class="badge" :class="channelProbeStatusClass(accountChannelCheck(account)?.probe_status)">
                      {{ channelProbeStatusLabel(accountChannelCheck(account)?.probe_status) }}
                    </span>
                  </div>
                </td>
                <td class="px-4 py-3">
                  <div class="flex flex-wrap gap-1.5">
                    <span v-if="accountOpsRow(account)?.candidate_status" class="badge" :class="candidateStatusClass(accountOpsRow(account)?.candidate_status)">
                      {{ candidateStatusLabel(accountOpsRow(account)?.candidate_status) }}
                    </span>
                    <span class="badge" :class="driftStatusClass(accountOpsRow(account)?.drift_status)">
                      {{ driftStatusLabel(accountOpsRow(account)?.drift_status) }}
                    </span>
                  </div>
                  <div v-if="accountOpsRow(account)?.blocked_reason" class="mt-1 max-w-[220px] truncate text-xs text-amber-600 dark:text-amber-300" :title="candidateReasonLabel(accountOpsRow(account)?.blocked_reason)">
                    {{ candidateReasonLabel(accountOpsRow(account)?.blocked_reason) }}
                    <span v-if="accountOpsRow(account)?.check_source"> · {{ checkSourceLabel(accountOpsRow(account)?.check_source) }}</span>
                  </div>
                  <div v-if="accountOpsRow(account)?.channel_error_message" class="mt-1 max-w-[220px] truncate text-xs text-red-600 dark:text-red-300" :title="accountOpsRow(account)?.channel_error_message">
                    {{ accountOpsRow(account)?.channel_error_message }}
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </div>

    <div v-if="supplierDetailLoading" class="flex items-center justify-center gap-2 py-8 text-sm text-gray-500 dark:text-dark-400">
      <Icon name="refresh" size="sm" class="animate-spin" />
      正在聚合供应商详情
    </div>
  </div>

  <template #footer>
    <button type="button" class="btn btn-secondary" @click="closeSupplierDetailDialog">关闭</button>
  </template>
</BaseDialog>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import type { LocalAccountOpsRow, LocalSub2APIAccount, SupplierAccount, SupplierChannelCheckSnapshot, SupplierGroup, SupplierKey } from '@/api/admin/adminPlus'

type DetailStatIcon = 'grid' | 'key' | 'server' | 'beaker' | 'dollar' | 'exclamationTriangle'

interface DetailStatItem {
  key: string
  label: string
  value: string
  caption: string
  icon: DetailStatIcon
  valueClass: string
}

const props = defineProps<{ vm: any }>()
const {
  supplierDetailDialogOpen,
  supplierDetailLoading,
  supplierDetailError,
  supplierDetailSupplier,
  supplierDetailGroups,
  supplierDetailKeys,
  supplierDetailAccounts,
  supplierDetailLocalAccounts,
  supplierDetailLocalOpsRows,
  supplierDetailChannelChecks,
  formatDateTime,
  formatMoney,
  formatMultiplier,
  formatLatency,
  actualCostMultiplier,
  rateMultiplierTextClass,
  channelProtocolFromProviderFamily,
  runtimeLabel,
  runtimeClass,
  healthLabel,
  healthClass,
  typeLabel,
  groupStatusLabel,
  groupStatusClass,
  supplierKeyStatusLabel,
  supplierKeyStatusClass,
  channelProbeStatusLabel,
  channelProbeStatusClass,
  closeSupplierDetailDialog,
  loadSupplierDetail
} = props.vm

const sortedDetailGroups = computed<SupplierGroup[]>(() => {
  return [...supplierDetailGroups.value].sort((a, b) => {
    const statusPriority = groupStatusPriority(a.status) - groupStatusPriority(b.status)
    if (statusPriority !== 0) return statusPriority
    const rateDelta = groupDetailCostMultiplier(a) - groupDetailCostMultiplier(b)
    if (Math.abs(rateDelta) > 0.000001) return rateDelta
    return a.name.localeCompare(b.name)
  })
})

const sortedDetailKeys = computed<SupplierKey[]>(() => {
  return [...supplierDetailKeys.value].sort((a, b) => {
    const statusPriority = keyStatusPriority(a.status) - keyStatusPriority(b.status)
    if (statusPriority !== 0) return statusPriority
    return (b.id || 0) - (a.id || 0)
  })
})

const sortedDetailAccounts = computed<SupplierAccount[]>(() => {
  return [...supplierDetailAccounts.value].sort((a, b) => {
    const aLocal = accountLocalAccount(a)
    const bLocal = accountLocalAccount(b)
    if (Boolean(aLocal?.schedulable) !== Boolean(bLocal?.schedulable)) return aLocal?.schedulable ? -1 : 1
    const rateDelta = accountDetailCostMultiplier(a) - accountDetailCostMultiplier(b)
    if (Math.abs(rateDelta) > 0.000001) return rateDelta
    return (a.local_account_name || '').localeCompare(b.local_account_name || '')
  })
})

const localAccountsByID = computed(() => {
  const out = new Map<number, LocalSub2APIAccount>()
  for (const account of supplierDetailLocalAccounts.value) {
    out.set(account.id, account)
  }
  return out
})

const keysByGroupID = computed(() => {
  const out = new Map<number, SupplierKey[]>()
  for (const key of sortedDetailKeys.value) {
    const items = out.get(key.supplier_group_id) || []
    items.push(key)
    out.set(key.supplier_group_id, items)
  }
  return out
})

const groupsByID = computed(() => {
  const out = new Map<number, SupplierGroup>()
  for (const group of supplierDetailGroups.value) {
    out.set(group.id, group)
  }
  return out
})

const accountsByGroupID = computed(() => {
  const out = new Map<number, SupplierAccount>()
  for (const account of supplierDetailAccounts.value) {
    if (!account.supplier_group_id) continue
    const existing = out.get(account.supplier_group_id)
    if (!existing || account.id > existing.id) out.set(account.supplier_group_id, account)
  }
  return out
})

const latestChannelCheckByGroupID = computed(() => {
  const out = new Map<number, SupplierChannelCheckSnapshot>()
  for (const snapshot of supplierDetailChannelChecks.value) {
    const existing = out.get(snapshot.supplier_group_id)
    if (!existing || snapshotTime(snapshot) > snapshotTime(existing) || (snapshotTime(snapshot) === snapshotTime(existing) && snapshot.id > existing.id)) {
      out.set(snapshot.supplier_group_id, snapshot)
    }
  }
  return out
})

const opsRowsByAccountID = computed(() => {
  const out = new Map<number, LocalAccountOpsRow[]>()
  for (const row of supplierDetailLocalOpsRows.value) {
    const items = out.get(row.local_sub2api_account_id) || []
    items.push(row)
    out.set(row.local_sub2api_account_id, items)
  }
  return out
})

const activeGroups = computed(() => supplierDetailGroups.value.filter((group: SupplierGroup) => group.status === 'active'))

const blockedGroups = computed(() => {
  return activeGroups.value.filter((group: SupplierGroup) => {
    const key = detailGroupKey(group)
    return !key || key.status !== 'bound'
  })
})

const detailStats = computed(() => {
  const boundKeys = supplierDetailKeys.value.filter((key: SupplierKey) => key.status === 'bound').length
  const keyActionRequired = supplierDetailKeys.value.filter((key: SupplierKey) => ['manual_secret_required', 'failed', 'disabled'].includes(key.status)).length
  const schedulableLocalIDs = new Set(
    supplierDetailAccounts.value
      .map((account: SupplierAccount) => accountLocalAccount(account))
      .filter((account?: LocalSub2APIAccount) => account?.schedulable)
      .map((account: LocalSub2APIAccount) => account.id)
  )
  const availableChecks = supplierDetailChannelChecks.value.filter((item: SupplierChannelCheckSnapshot) => item.probe_status === 'available').length
  const lowBalanceAccounts = supplierDetailAccounts.value.filter((account: SupplierAccount) => !account.has_usable_balance).length
  const unknownBalanceGroups = activeGroups.value.filter((group: SupplierGroup) => !groupSupplierAccount(group)).length
  const driftRows = supplierDetailLocalOpsRows.value.filter((row: LocalAccountOpsRow) => row.drift_status && !['synced', 'unbound'].includes(String(row.drift_status))).length
  return {
    groups: supplierDetailGroups.value.length,
    activeGroups: activeGroups.value.length,
    keys: supplierDetailKeys.value.length,
    boundKeys,
    keyActionRequired,
    schedulableLocalAccounts: schedulableLocalIDs.size,
    availableChecks,
    balanceRisks: lowBalanceAccounts + unknownBalanceGroups,
    driftRows
  }
})

const detailStatItems = computed<DetailStatItem[]>(() => [
  {
    key: 'groups',
    label: '第三方分组',
    value: `${detailStats.value.activeGroups}/${detailStats.value.groups}`,
    caption: '有效分组 / 全部分组',
    icon: 'grid',
    valueClass: 'text-gray-900 dark:text-white'
  },
  {
    key: 'keys',
    label: 'Key 覆盖',
    value: `${detailStats.value.boundKeys}/${detailStats.value.keys}`,
    caption: '已绑定 Key / 已创建 Key',
    icon: 'key',
    valueClass: detailStats.value.boundKeys >= detailStats.value.activeGroups && detailStats.value.activeGroups > 0 ? 'text-emerald-600 dark:text-emerald-300' : 'text-amber-600 dark:text-amber-300'
  },
  {
    key: 'schedulable',
    label: '可调度账号',
    value: String(detailStats.value.schedulableLocalAccounts),
    caption: '本地 schedulable=true',
    icon: 'server',
    valueClass: detailStats.value.schedulableLocalAccounts > 0 ? 'text-emerald-600 dark:text-emerald-300' : 'text-amber-600 dark:text-amber-300'
  },
  {
    key: 'checks',
    label: '可用通道',
    value: String(detailStats.value.availableChecks),
    caption: 'probe_status=available',
    icon: 'beaker',
    valueClass: detailStats.value.availableChecks > 0 ? 'text-emerald-600 dark:text-emerald-300' : 'text-gray-900 dark:text-white'
  },
  {
    key: 'balance',
    label: '余额风险',
    value: String(detailStats.value.balanceRisks),
    caption: '余额不足或未绑定余额',
    icon: 'dollar',
    valueClass: detailStats.value.balanceRisks > 0 ? 'text-rose-600 dark:text-rose-300' : 'text-emerald-600 dark:text-emerald-300'
  },
  {
    key: 'drift',
    label: '漂移风险',
    value: String(detailStats.value.driftRows),
    caption: '运营镜像待处理项',
    icon: 'exclamationTriangle',
    valueClass: detailStats.value.driftRows > 0 ? 'text-rose-600 dark:text-rose-300' : 'text-emerald-600 dark:text-emerald-300'
  }
])

const quotaRiskLabel = computed(() => {
  if (supplierDetailSupplier.value?.key_capacity_status === 'exhausted') return 'Key 配额已满'
  if (supplierDetailSupplier.value?.key_capacity_status === 'unsupported') return '不支持自动开通'
  if (blockedGroups.value.length > 0) return `阻塞 ${blockedGroups.value.length} 个分组`
  if (detailStats.value.keyActionRequired > 0) return `需处理 ${detailStats.value.keyActionRequired} 个 Key`
  return '暂无阻塞'
})

const quotaRiskClass = computed(() => {
  if (supplierDetailSupplier.value?.key_capacity_status === 'exhausted') return 'badge-danger'
  if (supplierDetailSupplier.value?.key_capacity_status === 'unsupported') return 'badge-warning'
  if (blockedGroups.value.length > 0) return 'badge-warning'
  if (detailStats.value.keyActionRequired > 0) return 'badge-warning'
  return 'badge-success'
})

const keyUsageLabel = computed(() => {
  const supplier = supplierDetailSupplier.value
  if (!supplier) return String(supplierDetailKeys.value.length)
  if (supplier.key_limit_policy === 'limited') return `${supplier.active_key_count || supplierDetailKeys.value.length}/${supplier.key_limit_value || 0}`
  return String(supplier.active_key_count || supplierDetailKeys.value.length)
})

function keyLimitPolicyLabel(value?: string): string {
  return {
    unknown: '未知',
    unlimited: '无限制',
    limited: '限制数量',
    unsupported: '不支持自动创建'
  }[value || ''] || value || '-'
}

function keyCapacityStatusLabel(value?: string): string {
  return {
    available: '可创建',
    limited: '接近上限',
    exhausted: '配额已满',
    unknown: '未知',
    unsupported: '不支持'
  }[value || ''] || value || '-'
}

const supplierAuditRoute = computed(() => {
  const supplier = supplierDetailSupplier.value
  return {
    path: '/admin/action-audits',
    query: {
      q: supplier?.name || (supplier?.id ? String(supplier.id) : ''),
      window: '24h'
    }
  }
})

function detailGroupKey(group: SupplierGroup): SupplierKey | undefined {
  const keys = keysByGroupID.value.get(group.id) || []
  return keys.find((key) => key.status === 'bound') || keys[0]
}

function detailChannelCheck(group: SupplierGroup): SupplierChannelCheckSnapshot | undefined {
  return latestChannelCheckByGroupID.value.get(group.id)
}

function groupSupplierAccount(group: SupplierGroup): SupplierAccount | undefined {
  return accountsByGroupID.value.get(group.id)
}

function groupLocalAccountID(group: SupplierGroup): number {
  return Number(detailGroupKey(group)?.local_sub2api_account_id || groupSupplierAccount(group)?.local_sub2api_account_id || 0)
}

function groupLocalAccount(group: SupplierGroup): LocalSub2APIAccount | undefined {
  const id = groupLocalAccountID(group)
  return id ? localAccountsByID.value.get(id) : undefined
}

function accountLocalAccount(account: SupplierAccount): LocalSub2APIAccount | undefined {
  return localAccountsByID.value.get(account.local_sub2api_account_id)
}

function groupOpsRow(group: SupplierGroup): LocalAccountOpsRow | undefined {
  const accountID = groupLocalAccountID(group)
  if (!accountID) return undefined
  const rows = opsRowsByAccountID.value.get(accountID) || []
  return rows.find((row) => row.supplier_group_id === group.id) || rows[0]
}

function accountOpsRow(account: SupplierAccount): LocalAccountOpsRow | undefined {
  const rows = opsRowsByAccountID.value.get(account.local_sub2api_account_id) || []
  return rows.find((row) => row.supplier_account_id === account.id) ||
    rows.find((row) => row.supplier_group_id === account.supplier_group_id) ||
    rows[0]
}

function accountChannelCheck(account: SupplierAccount): SupplierChannelCheckSnapshot | undefined {
  return account.supplier_group_id ? latestChannelCheckByGroupID.value.get(account.supplier_group_id) : undefined
}

function keyGroupName(key: SupplierKey): string {
  return groupsByID.value.get(key.supplier_group_id)?.name || key.external_group_id || `分组 #${key.supplier_group_id}`
}

function groupDetailCostMultiplier(group: SupplierGroup): number {
  return actualCostMultiplier(group.effective_rate_multiplier, supplierDetailSupplier.value?.recharge_multiplier || 1)
}

function accountDetailCostMultiplier(account: SupplierAccount): number {
  return actualCostMultiplier(account.supplier_group_rate || accountDetailLocalRate(account), supplierDetailSupplier.value?.recharge_multiplier || 1)
}

function accountDetailLocalRate(account: SupplierAccount): number {
  return accountLocalAccount(account)?.rate_multiplier || 1
}

function groupBalanceLabel(group: SupplierGroup): string {
  const account = groupSupplierAccount(group)
  if (!account) return '余额未知'
  if (account.has_usable_balance) return formatMoney(account.balance_cents || 0, account.balance_currency || 'USD')
  return `不足 ${formatMoney(account.balance_cents || 0, account.balance_currency || 'USD')}`
}

function groupBalanceClass(group: SupplierGroup): string {
  const account = groupSupplierAccount(group)
  if (!account) return 'badge-warning'
  return account.has_usable_balance ? 'badge-success' : 'badge-danger'
}

function localGroupNames(account?: LocalSub2APIAccount): string[] {
  if (!account) return []
  const names = Array.isArray(account.group_names) ? account.group_names.filter(Boolean) : []
  const ids = Array.isArray(account.group_ids) ? account.group_ids : []
  if (names.length > 0) return names
  return ids.map((id) => `分组 #${id}`)
}

function driftStatusLabel(value?: string): string {
  const labels: Record<string, string> = {
    unbound: '未绑定',
    synced: '已同步',
    supplier_disabled: '供应商停用',
    binding_disabled: '绑定停用',
    missing_key: '缺少 Key',
    key_local_account_mismatch: 'Key 账号不一致',
    missing_group: '缺少分组',
    group_missing: '分组缺失',
    group_disabled: '分组停用',
    local_account_metadata_drift: '账号漂移',
    local_account_state_drift: '原后台变更',
    unknown: '未知'
  }
  return labels[String(value || 'unknown')] || String(value || '未知')
}

function driftStatusClass(value?: string): string {
  if (!value || value === 'unbound') return 'badge-gray'
  if (value === 'synced') return 'badge-success'
  if (value === 'local_account_state_drift') return 'badge-danger'
  if (value === 'local_account_metadata_drift') return 'badge-warning'
  return 'badge-warning'
}

function opsBalanceStatusLabel(value?: string): string {
  if (value === 'usable') return '余额可用'
  if (value === 'insufficient') return '余额不足'
  if (value === 'unknown') return '余额未知'
  if (value === 'unbound') return '未绑定'
  return '余额未知'
}

function opsBalanceStatusClass(value?: string): string {
  if (value === 'usable') return 'badge-success'
  if (value === 'insufficient') return 'badge-danger'
  if (value === 'unbound') return 'badge-gray'
  return 'badge-warning'
}

function candidateStatusLabel(value?: string): string {
  const labels: Record<string, string> = {
    available: '可调度候选',
    unknown: '待确认',
    degraded: '质量降级',
    needs_provisioning: '待开通',
    balance_blocked: '余额阻断',
    blocked: '不可用',
    local_blocked: '本地阻断',
    capacity_blocked: '配额阻断'
  }
  return labels[String(value || '')] || String(value || '-')
}

function candidateStatusClass(value?: string): string {
  if (value === 'available') return 'badge-success'
  if (value === 'balance_blocked' || value === 'capacity_blocked' || value === 'blocked') return 'badge-danger'
  if (value === 'needs_provisioning' || value === 'unknown' || value === 'degraded') return 'badge-warning'
  return 'badge-gray'
}

function checkSourceLabel(value?: string): string {
  const labels: Record<string, string> = {
    supplier: '供应商',
    supplier_group: '第三方分组',
    supplier_key: '第三方 Key',
    key_capacity: 'Key 配额',
    balance: '余额',
    local_state: '本地状态',
    channel_monitor: '通道监控',
    active_probe: '实测'
  }
  return labels[String(value || '')] || String(value || '-')
}

function candidateReasonLabel(value?: string): string {
  const labels: Record<string, string> = {
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
    rate_missing: '倍率缺失'
  }
  return labels[String(value || '')] || String(value || '-')
}

function groupStatusPriority(status?: string): number {
  if (status === 'active') return 0
  if (status === 'missing') return 1
  return 2
}

function keyStatusPriority(status?: string): number {
  if (status === 'manual_secret_required' || status === 'failed') return 0
  if (status === 'bound') return 1
  if (status === 'provisioning') return 2
  return 3
}

function snapshotTime(snapshot: SupplierChannelCheckSnapshot): number {
  const value = new Date(snapshot.captured_at).getTime()
  return Number.isFinite(value) ? value : 0
}

function shortText(value: string, maxLength: number): string {
  if (!value || value.length <= maxLength) return value
  return `${value.slice(0, Math.max(maxLength - 3, 1))}...`
}
</script>
