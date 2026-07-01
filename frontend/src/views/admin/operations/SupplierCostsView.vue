<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">成本对账</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
            从供应商充值订单、兑换充值、用量消耗和余额快照生成上游成本台账。
          </p>
        </div>
        <div class="flex flex-wrap gap-2">
          <button type="button" class="btn btn-secondary" :disabled="loading" @click="loadCurrentTab">
            <Icon name="refresh" size="sm" />
            刷新
          </button>
          <button type="button" class="btn btn-primary" :disabled="backfilling" @click="startHistoryBackfill">
            <Icon name="play" size="sm" />
            {{ backfilling ? '回补中...' : '一键回补全部供应商' }}
          </button>
        </div>
      </section>

      <nav class="flex gap-2 overflow-x-auto border-b border-gray-200 dark:border-dark-700">
        <button
          v-for="tab in topTabs"
          :key="tab.value"
          type="button"
          class="whitespace-nowrap border-b-2 px-3 py-2 text-sm font-medium"
          :class="activeTopTab === tab.value ? 'border-primary-500 text-primary-600 dark:text-primary-400' : 'border-transparent text-gray-500 hover:text-gray-900 dark:text-dark-400 dark:hover:text-white'"
          @click="setTopTab(tab.value)"
        >
          {{ tab.label }}
        </button>
      </nav>

      <section class="grid gap-4 lg:grid-cols-[1.2fr_1fr_1fr_auto] lg:items-end">
        <label class="block">
          <span class="input-label">供应商</span>
          <select v-model.number="selectedSupplierId" class="input" @change="handleSupplierChange">
            <option :value="0">全部供应商</option>
            <option v-for="supplier in suppliers" :key="supplier.id" :value="supplier.id">{{ supplier.name }}</option>
          </select>
        </label>
        <label class="block">
          <span class="input-label">开始时间</span>
          <input v-model="syncForm.started_at" type="datetime-local" class="input" />
        </label>
        <label class="block">
          <span class="input-label">结束时间</span>
          <input v-model="syncForm.ended_at" type="datetime-local" class="input" />
        </label>
        <button type="button" class="btn btn-secondary" :disabled="syncing || !selectedSupplierId" @click="syncCosts">
          <Icon name="sync" size="sm" />
          {{ syncing ? '同步中' : '同步当前供应商' }}
        </button>
      </section>

      <section v-if="activeTopTab === 'overview'" class="card overflow-hidden">
        <div class="flex flex-col gap-2 border-b border-gray-100 px-5 py-4 dark:border-dark-700 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">总账统计</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">按币种汇总所有供应商最新成本快照，不跨币种折算。</p>
          </div>
          <div class="text-sm text-gray-500 dark:text-dark-400">
            {{ ledgerOverview?.generated_at ? `生成时间 ${formatDateTime(ledgerOverview.generated_at)}` : '暂无总账快照' }}
          </div>
        </div>
        <div class="overflow-x-auto">
          <table class="w-full min-w-[1260px] divide-y divide-gray-200 dark:divide-dark-700">
            <thead class="bg-gray-50 dark:bg-dark-800">
              <tr>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">币种</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">供应商/快照</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">充值总额</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">实际支付</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">充值订单</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">兑换充值</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">用量消耗</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">实际余额/快照</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">余额差异</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">最近采集</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-900">
              <tr v-if="ledgerOverviewItems.length === 0">
                <td colspan="10" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-dark-400">暂无总账统计</td>
              </tr>
              <tr v-for="item in ledgerOverviewItems" :key="item.currency">
                <td class="px-4 py-4 text-sm font-semibold text-gray-900 dark:text-gray-100">{{ item.currency }}</td>
                <td class="px-4 py-4 text-right text-sm text-gray-900 dark:text-gray-100">
                  {{ item.supplier_count }} / {{ item.snapshot_count }}
                </td>
                <td class="px-4 py-4 text-right text-sm font-medium text-gray-900 dark:text-gray-100">{{ formatMoney(item.recharge_total_cents, item.currency) }}</td>
                <td class="px-4 py-4 text-right text-sm font-semibold text-emerald-700 dark:text-emerald-300">{{ formatMoney(item.recharge_actual_payment_cents, item.currency) }}</td>
                <td class="px-4 py-4 text-right text-sm text-gray-900 dark:text-gray-100">{{ formatMoney(item.completed_funding_amount_cents, item.currency) }}</td>
                <td class="px-4 py-4 text-right text-sm text-gray-900 dark:text-gray-100">{{ formatMoney(item.entitlement_amount_cents, item.currency) }}</td>
                <td class="px-4 py-4 text-right text-sm text-gray-900 dark:text-gray-100">{{ formatMoney(item.usage_cost_cents, item.currency) }}</td>
                <td class="px-4 py-4 text-right text-sm text-gray-900 dark:text-gray-100">
                  {{ item.actual_balance_cents === undefined || item.actual_balance_cents === null ? '-' : formatMoney(item.actual_balance_cents, item.currency) }}
                  <span class="ml-1 text-xs text-gray-400 dark:text-dark-500">({{ item.actual_balance_available_count }})</span>
                </td>
                <td class="px-4 py-4 text-right text-sm font-medium" :class="deltaClass(item.balance_delta_cents)">
                  {{ item.balance_delta_cents === undefined || item.balance_delta_cents === null ? '-' : formatMoney(item.balance_delta_cents, item.currency) }}
                </td>
                <td class="px-4 py-4 text-sm text-gray-500 dark:text-dark-400">{{ formatDateTime(item.latest_captured_at) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section v-else-if="activeTopTab === 'suppliers'" class="card overflow-hidden">
        <div class="flex flex-col gap-2 border-b border-gray-100 px-5 py-4 dark:border-dark-700 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">供应商成本快照</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">点击供应商进入单供应商明细，避免首屏拉取所有明细。</p>
          </div>
          <div class="text-sm text-gray-500 dark:text-dark-400">{{ snapshots.length }} 条快照</div>
        </div>
        <div class="overflow-x-auto">
          <table class="w-full min-w-[1260px] divide-y divide-gray-200 dark:divide-dark-700">
            <thead class="bg-gray-50 dark:bg-dark-800">
              <tr>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">供应商</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">币种</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">充值总额</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">实际支付</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">充值订单</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">兑换充值</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">用量消耗</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">实际余额</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">差异</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">采集时间</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-900">
              <tr v-if="snapshots.length === 0">
                <td colspan="10" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-dark-400">暂无成本快照</td>
              </tr>
              <tr
                v-for="snapshot in snapshots"
                :key="snapshot.id"
                class="cursor-pointer hover:bg-gray-50 dark:hover:bg-dark-800"
                @click="selectSnapshot(snapshot.supplier_id)"
              >
                <td class="px-4 py-4 text-sm text-gray-900 dark:text-gray-100">{{ supplierName(snapshot.supplier_id) }}</td>
                <td class="px-4 py-4 text-sm text-gray-900 dark:text-gray-100">{{ snapshot.currency }}</td>
                <td class="px-4 py-4 text-right text-sm text-gray-900 dark:text-gray-100">{{ formatMoney(supplierRechargeTotalCents(snapshot), snapshot.currency) }}</td>
                <td class="px-4 py-4 text-right text-sm font-semibold text-emerald-700 dark:text-emerald-300">{{ formatMoney(snapshot.recharge_actual_payment_cents, snapshot.currency) }}</td>
                <td class="px-4 py-4 text-right text-sm text-gray-900 dark:text-gray-100">{{ formatMoney(snapshot.completed_funding_amount_cents, snapshot.currency) }}</td>
                <td class="px-4 py-4 text-right text-sm text-gray-900 dark:text-gray-100">{{ formatMoney(snapshot.entitlement_amount_cents, snapshot.currency) }}</td>
                <td class="px-4 py-4 text-right text-sm text-gray-900 dark:text-gray-100">{{ formatMoney(supplierDisplayUsageCents(snapshot), snapshot.currency) }}</td>
                <td class="px-4 py-4 text-right text-sm text-gray-900 dark:text-gray-100">{{ snapshot.actual_balance_cents === undefined || snapshot.actual_balance_cents === null ? '-' : formatMoney(snapshot.actual_balance_cents, snapshot.currency) }}</td>
                <td class="px-4 py-4 text-right text-sm" :class="deltaClass(supplierBalanceDeltaCents(snapshot))">
                  {{ supplierBalanceDeltaCents(snapshot) === null ? '-' : formatMoney(supplierBalanceDeltaCents(snapshot) || 0, snapshot.currency) }}
                </td>
                <td class="px-4 py-4 text-sm text-gray-500 dark:text-dark-400">{{ formatDateTime(snapshot.captured_at) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section v-else-if="activeTopTab === 'detail'" class="space-y-6">
        <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-7">
          <div class="card p-4">
            <p class="text-xs font-medium text-gray-500 dark:text-dark-400">充值总额</p>
            <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ formatMoney(supplierRechargeTotalCents(currentSnapshot), currentCurrency) }}</p>
          </div>
          <div class="card p-4">
            <p class="text-xs font-medium text-gray-500 dark:text-dark-400">实际支付</p>
            <p class="mt-2 text-2xl font-semibold text-emerald-700 dark:text-emerald-300">{{ formatMoney(currentSnapshot?.recharge_actual_payment_cents || 0, currentCurrency) }}</p>
          </div>
          <div class="card p-4">
            <p class="text-xs font-medium text-gray-500 dark:text-dark-400">充值订单</p>
            <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ formatMoney(currentSnapshot?.completed_funding_amount_cents || 0, currentCurrency) }}</p>
          </div>
          <div class="card p-4">
            <p class="text-xs font-medium text-gray-500 dark:text-dark-400">兑换充值</p>
            <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ formatMoney(currentSnapshot?.entitlement_amount_cents || 0, currentCurrency) }}</p>
          </div>
          <div class="card p-4">
            <p class="text-xs font-medium text-gray-500 dark:text-dark-400">用量消耗</p>
            <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ formatMoney(supplierDisplayUsageCents(currentSnapshot), currentCurrency) }}</p>
          </div>
          <div class="card p-4">
            <p class="text-xs font-medium text-gray-500 dark:text-dark-400">实际余额</p>
            <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">
              {{ currentSnapshot?.actual_balance_cents === undefined || currentSnapshot?.actual_balance_cents === null ? '-' : formatMoney(currentSnapshot.actual_balance_cents, currentCurrency) }}
            </p>
          </div>
          <div class="card p-4">
            <p class="text-xs font-medium text-gray-500 dark:text-dark-400">余额差异</p>
            <p class="mt-2 text-2xl font-semibold" :class="balanceDeltaClass">
              {{ currentBalanceDelta === null ? '-' : formatMoney(currentBalanceDelta, currentCurrency) }}
            </p>
          </div>
        </div>

        <section class="card overflow-hidden">
          <div class="flex flex-col gap-4 border-b border-gray-100 px-5 py-4 dark:border-dark-700 lg:flex-row lg:items-center lg:justify-between">
            <div class="inline-flex w-fit rounded-lg border border-gray-200 bg-gray-50 p-1 dark:border-dark-700 dark:bg-dark-800">
              <button
                v-for="tab in detailTabs"
                :key="tab.value"
                type="button"
                class="rounded-md px-3 py-1.5 text-sm font-medium transition"
                :class="activeDetailTab === tab.value ? 'bg-white text-primary-600 shadow-sm dark:bg-dark-700 dark:text-primary-300' : 'text-gray-600 hover:text-gray-900 dark:text-dark-300 dark:hover:text-white'"
                @click="setDetailTab(tab.value)"
              >
                {{ tab.label }}
              </button>
            </div>
            <div class="text-sm text-gray-500 dark:text-dark-400">{{ syncStatusLabel }}</div>
          </div>

          <div v-if="!selectedSupplierId" class="px-5 py-10 text-center text-sm text-gray-500 dark:text-dark-400">
            请选择供应商查看明细。
          </div>
          <div v-else-if="activeDetailTab === 'summary'" class="px-5 py-8 text-sm text-gray-500 dark:text-dark-400">
            当前供应商：{{ supplierName(selectedSupplierId) }}，{{ currentSnapshot ? `最近采集 ${formatDateTime(currentSnapshot.captured_at)}` : '暂无快照' }}。
          </div>
          <div v-else-if="activeDetailTab === 'funding'" class="overflow-x-auto">
            <table class="w-full min-w-[1180px] divide-y divide-gray-200 dark:divide-dark-700">
              <thead class="bg-gray-50 dark:bg-dark-800">
                <tr>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">订单</th>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">支付方式</th>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">状态</th>
                  <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">额度</th>
                  <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">原始实付</th>
                  <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">充值倍率</th>
                  <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">实际支付</th>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">完成时间</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-900">
                <tr v-if="funding.length === 0">
                  <td colspan="8" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-dark-400">暂无充值订单</td>
                </tr>
                <tr v-for="item in funding" :key="item.id">
                  <td class="px-4 py-4 text-sm text-gray-900 dark:text-gray-100">
                    <div class="font-mono text-xs">{{ item.external_id }}</div>
                    <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ item.out_trade_no || '-' }}</div>
                  </td>
                  <td class="px-4 py-4 text-sm text-gray-900 dark:text-gray-100">{{ item.payment_type || '-' }}</td>
                  <td class="px-4 py-4"><span class="badge badge-gray">{{ item.status }}</span></td>
                  <td class="px-4 py-4 text-right text-sm text-gray-900 dark:text-gray-100">{{ formatMoney(item.amount_cents, item.currency) }}</td>
                  <td class="px-4 py-4 text-right text-sm text-gray-900 dark:text-gray-100">{{ formatMoney(item.cash_amount_cents, item.currency) }}</td>
                  <td class="px-4 py-4 text-right text-sm text-gray-900 dark:text-gray-100">{{ formatMultiplier(item.recharge_multiplier) }}</td>
                  <td class="px-4 py-4 text-right text-sm font-semibold text-emerald-700 dark:text-emerald-300">{{ formatMoney(item.actual_payment_cents, item.currency) }}</td>
                  <td class="px-4 py-4 text-sm text-gray-500 dark:text-dark-400">{{ formatDateTime(item.completed_at || item.paid_at || item.created_at_external) }}</td>
                </tr>
              </tbody>
            </table>
          </div>

          <div v-else-if="activeDetailTab === 'entitlements'" class="overflow-x-auto">
            <table class="w-full min-w-[980px] divide-y divide-gray-200 dark:divide-dark-700">
              <thead class="bg-gray-50 dark:bg-dark-800">
                <tr>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">记录</th>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">来源</th>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">状态</th>
                  <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">权益内容</th>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">使用时间</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-900">
                <tr v-if="entitlements.length === 0">
                  <td colspan="5" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-dark-400">暂无兑换记录</td>
                </tr>
                <tr v-for="item in entitlements" :key="item.id">
                  <td class="px-4 py-4 text-sm text-gray-900 dark:text-gray-100">
                    <div class="font-mono text-xs">{{ item.external_id }}</div>
                    <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">尾号 {{ item.code_last4 || '-' }}</div>
                  </td>
                  <td class="px-4 py-4 text-sm text-gray-900 dark:text-gray-100">
                    <div>{{ sourceFamilyLabel(item.source_family) }}</div>
                    <div class="mt-2">
                      <span class="badge" :class="entitlementBadgeClass(item.type)">{{ entitlementTypeLabel(item.type) }}</span>
                    </div>
                  </td>
                  <td class="px-4 py-4"><span class="badge badge-gray">{{ item.status }}</span></td>
                  <td class="px-4 py-4 text-right text-sm text-gray-900 dark:text-gray-100">{{ entitlementValueLabel(item) }}</td>
                  <td class="px-4 py-4 text-sm text-gray-500 dark:text-dark-400">{{ formatDateTime(item.used_at || item.created_at_external) }}</td>
                </tr>
              </tbody>
            </table>
          </div>

          <div v-else class="overflow-x-auto">
            <table class="w-full min-w-[980px] divide-y divide-gray-200 dark:divide-dark-700">
              <thead class="bg-gray-50 dark:bg-dark-800">
                <tr>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">类型</th>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">来源</th>
                  <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">金额</th>
                  <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">实际支付</th>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">发生时间</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-900">
                <tr v-if="ledger.length === 0">
                  <td colspan="5" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-dark-400">暂无成本台账</td>
                </tr>
                <tr v-for="entry in ledger" :key="entry.id">
                  <td class="px-4 py-4"><span class="badge" :class="ledgerBadgeClass(entry.entry_type)">{{ ledgerTypeLabel(entry.entry_type) }}</span></td>
                  <td class="px-4 py-4 text-sm text-gray-900 dark:text-gray-100">
                    <div>{{ entry.source_type }}</div>
                    <div class="mt-1 font-mono text-xs text-gray-500 dark:text-dark-400">{{ entry.source_external_id || `#${entry.source_id}` }}</div>
                  </td>
                  <td class="px-4 py-4 text-right text-sm text-gray-900 dark:text-gray-100">{{ formatMoney(entry.amount_cents, entry.currency) }}</td>
                  <td class="px-4 py-4 text-right text-sm text-gray-900 dark:text-gray-100">{{ entry.actual_payment_cents ? formatMoney(entry.actual_payment_cents, entry.currency) : '-' }}</td>
                  <td class="px-4 py-4 text-sm text-gray-500 dark:text-dark-400">{{ formatDateTime(entry.occurred_at) }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>
      </section>

      <section v-else class="grid gap-6 xl:grid-cols-[minmax(0,1fr)_360px]">
        <div class="card overflow-hidden">
          <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">历史回补运行</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">一键提交后由后台按供应商拆分 step 执行，页面只轮询 run 状态。</p>
          </div>
          <div v-if="!activeBackfillRun" class="px-5 py-10 text-center text-sm text-gray-500 dark:text-dark-400">
            暂无本页提交的历史回补任务。
          </div>
          <div v-else class="space-y-5 p-5">
            <div class="grid gap-3 md:grid-cols-5">
              <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
                <p class="text-xs text-gray-500 dark:text-dark-400">状态</p>
                <span class="badge mt-2" :class="runStatusClass(activeBackfillRun.run.status)">{{ runStatusLabel(activeBackfillRun.run.status) }}</span>
              </div>
              <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
                <p class="text-xs text-gray-500 dark:text-dark-400">供应商</p>
                <p class="mt-2 text-sm font-medium text-gray-900 dark:text-white">{{ activeBackfillRun.run.supplier_count }}</p>
              </div>
              <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
                <p class="text-xs text-gray-500 dark:text-dark-400">成功</p>
                <p class="mt-2 text-sm font-medium text-emerald-600 dark:text-emerald-400">{{ activeBackfillRun.run.succeeded_steps }}</p>
              </div>
              <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
                <p class="text-xs text-gray-500 dark:text-dark-400">失败</p>
                <p class="mt-2 text-sm font-medium text-rose-600 dark:text-rose-400">{{ activeBackfillRun.run.failed_steps }}</p>
              </div>
              <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
                <p class="text-xs text-gray-500 dark:text-dark-400">总 Step</p>
                <p class="mt-2 text-sm font-medium text-gray-900 dark:text-white">{{ activeBackfillRun.run.total_steps }}</p>
              </div>
            </div>

            <div class="overflow-x-auto rounded-lg border border-gray-200 dark:border-dark-700">
              <table class="w-full min-w-[960px] divide-y divide-gray-200 dark:divide-dark-700">
                <thead class="bg-gray-50 dark:bg-dark-800">
                  <tr>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">Step</th>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">供应商</th>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">状态</th>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">结果</th>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">时间</th>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">错误/原因</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-100 bg-white dark:divide-dark-700 dark:bg-dark-900">
                  <tr v-if="activeBackfillRun.steps.length === 0">
                    <td colspan="6" class="px-4 py-8 text-center text-sm text-gray-500 dark:text-dark-400">暂无 step 明细</td>
                  </tr>
                  <tr v-for="step in activeBackfillRun.steps" :key="step.id">
                    <td class="px-4 py-3 font-mono text-xs text-gray-500 dark:text-dark-400">{{ step.id }}</td>
                    <td class="px-4 py-3 text-sm text-gray-900 dark:text-gray-100">{{ step.supplier_name || supplierName(step.supplier_id) }}</td>
                    <td class="px-4 py-3"><span class="badge" :class="runStatusClass(step.status)">{{ runStatusLabel(step.status) }}</span></td>
                    <td class="px-4 py-3 text-sm text-gray-500 dark:text-dark-400">{{ stepResultLabel(step.result_snapshot, step.result_count) }}</td>
                    <td class="px-4 py-3 text-sm text-gray-500 dark:text-dark-400">
                      <div>{{ formatDateTime(step.started_at) }}</div>
                      <div v-if="step.finished_at" class="mt-1 text-xs text-gray-400 dark:text-dark-500">完成 {{ formatDateTime(step.finished_at) }}</div>
                    </td>
                    <td class="max-w-[260px] px-4 py-3 text-sm text-gray-500 dark:text-dark-400">
                      <span class="block truncate" :title="step.reason || ''">{{ step.reason || '-' }}</span>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>

        <aside class="card p-5">
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">回补范围</h2>
          <dl class="mt-4 space-y-3 text-sm">
            <div class="flex items-center justify-between gap-3">
              <dt class="text-gray-500 dark:text-dark-400">目标</dt>
              <dd class="font-medium text-gray-900 dark:text-white">全部供应商</dd>
            </div>
            <div class="flex items-center justify-between gap-3">
              <dt class="text-gray-500 dark:text-dark-400">开始</dt>
              <dd class="font-medium text-gray-900 dark:text-white">{{ formatDateTime(toRFC3339(syncForm.started_at)) }}</dd>
            </div>
            <div class="flex items-center justify-between gap-3">
              <dt class="text-gray-500 dark:text-dark-400">结束</dt>
              <dd class="font-medium text-gray-900 dark:text-white">{{ formatDateTime(toRFC3339(syncForm.ended_at)) }}</dd>
            </div>
            <div class="flex items-center justify-between gap-3">
              <dt class="text-gray-500 dark:text-dark-400">模式</dt>
              <dd class="font-medium text-gray-900 dark:text-white">后台分批</dd>
            </div>
          </dl>
          <button type="button" class="btn btn-primary mt-5 w-full" :disabled="backfilling" @click="startHistoryBackfill">
            <Icon name="play" size="sm" />
            {{ backfilling ? '已提交后台任务' : '提交历史回补' }}
          </button>
        </aside>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import {
  backfillSupplierCosts,
  getSchedulerRunDetail,
  getSupplierCostLedgerOverview,
  getSupplierCostSummary,
  getSupplierProvisionJob,
  listSupplierCostLedger,
  listSupplierCostSnapshots,
  listSupplierEntitlementTransactions,
  listSupplierFundingTransactions,
  listSuppliers,
  syncSupplierCosts,
  type SchedulerRunDetail,
  type Supplier,
  type SupplierCostLedgerEntry,
  type SupplierCostLedgerOverview,
  type SupplierCostLedgerOverviewItem,
  type SupplierCostSnapshot,
  type SupplierCostSyncResultSnapshot,
  type SupplierEntitlementTransaction,
  type SupplierFundingTransaction,
  type SupplierProvisionJob,
  type SupplierProvisionStatus
} from '@/api/admin/adminPlus'
import {
  supplierBalanceDeltaCents,
  supplierDisplayUsageCents,
  supplierRechargeTotalCents
} from './supplierCostPresentation'
import { runStatusClass, runStatusLabel } from '../scheduler/presentation'

type TopTab = 'overview' | 'suppliers' | 'detail' | 'backfill'
type DetailTab = 'summary' | 'funding' | 'entitlements' | 'ledger'

const appStore = useAppStore()
const loading = ref(false)
const syncing = ref(false)
const backfilling = ref(false)
const suppliersLoaded = ref(false)
const overviewLoaded = ref(false)
const snapshotsLoaded = ref(false)
const detailLoadedSupplierId = ref(0)
const suppliers = ref<Supplier[]>([])
const ledgerOverview = ref<SupplierCostLedgerOverview | null>(null)
const snapshots = ref<SupplierCostSnapshot[]>([])
const funding = ref<SupplierFundingTransaction[]>([])
const entitlements = ref<SupplierEntitlementTransaction[]>([])
const ledger = ref<SupplierCostLedgerEntry[]>([])
const selectedSupplierId = ref(0)
const activeTopTab = ref<TopTab>('overview')
const activeDetailTab = ref<DetailTab>('summary')
const activeSyncJob = ref<SupplierProvisionJob | null>(null)
const lastSync = ref<SupplierCostSyncResultSnapshot | null>(null)
const activeBackfillRun = ref<SchedulerRunDetail | null>(null)
let syncJobTimer: number | undefined
let backfillRunTimer: number | undefined

const syncForm = reactive({
  started_at: toDateTimeLocal(new Date(Date.now() - 365 * 24 * 60 * 60 * 1000)),
  ended_at: toDateTimeLocal(new Date())
})

const topTabs: Array<{ value: TopTab; label: string }> = [
  { value: 'overview', label: '总账统计' },
  { value: 'suppliers', label: '供应商快照' },
  { value: 'detail', label: '供应商明细' },
  { value: 'backfill', label: '历史回补' }
]

const detailTabs: Array<{ value: DetailTab; label: string }> = [
  { value: 'summary', label: '成本摘要' },
  { value: 'funding', label: '充值订单' },
  { value: 'entitlements', label: '兑换记录' },
  { value: 'ledger', label: '成本台账' }
]

const currentSnapshot = computed(() => {
  return snapshots.value.find((item) => item.supplier_id === selectedSupplierId.value) || null
})
const currentCurrency = computed(() => currentSnapshot.value?.currency || 'USD')
const currentBalanceDelta = computed(() => supplierBalanceDeltaCents(currentSnapshot.value))
const balanceDeltaClass = computed(() => deltaClass(currentBalanceDelta.value))
const ledgerOverviewItems = computed<SupplierCostLedgerOverviewItem[]>(() => ledgerOverview.value?.items || [])
const syncStatusLabel = computed(() => {
  if (activeSyncJob.value) return syncJobCaption(activeSyncJob.value)
  if (syncing.value) return '成本同步任务已提交'
  if (lastSync.value) return `上次同步：充值 ${lastSync.value.funding_transactions || 0}，兑换 ${lastSync.value.entitlement_transactions || 0}，用量 ${lastSync.value.usage_cost_lines || 0}`
  if (!selectedSupplierId.value) return '选择供应商后同步成本'
  return '等待同步'
})

function formatMoney(cents: number, currency: string): string {
  return new Intl.NumberFormat(undefined, {
    style: 'currency',
    currency: currency || 'USD',
    currencyDisplay: 'narrowSymbol',
    minimumFractionDigits: 2
  }).format((cents || 0) / 100)
}

function formatNumber(value?: number | null): string {
  return new Intl.NumberFormat(undefined, { maximumFractionDigits: 2 }).format(value || 0)
}

function formatMultiplier(value?: number | null): string {
  if (typeof value !== 'number') return '-'
  if (!Number.isFinite(value)) return '-'
  return `${value.toFixed(4).replace(/\.?0+$/, '')}x`
}

function formatDateTime(value?: string | null): string {
  if (!value) return '-'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '-' : date.toLocaleString()
}

function toDateTimeLocal(value: Date): string {
  const offsetMs = value.getTimezoneOffset() * 60 * 1000
  return new Date(value.getTime() - offsetMs).toISOString().slice(0, 16)
}

function toRFC3339(value: string): string {
  if (!value) return ''
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '' : date.toISOString()
}

function supplierName(id: number): string {
  return suppliers.value.find((supplier) => supplier.id === id)?.name || `#${id}`
}

function deltaClass(value?: number | null): string {
  if (value === null || value === undefined || value === 0) return 'text-emerald-600 dark:text-emerald-400'
  return 'text-rose-600 dark:text-rose-400'
}

function sourceFamilyLabel(value: string): string {
  return {
    payment_auto_redeem: '充值自动兑换',
    manual_redeem: '手工兑换'
  }[value] || value || '-'
}

function entitlementTypeLabel(value: string): string {
  return {
    balance: '余额',
    concurrency: '并发',
    subscription: '订阅'
  }[value] || value || '-'
}

function entitlementBadgeClass(value: string): string {
  if (value === 'balance') return 'badge-success'
  if (value === 'concurrency') return 'badge-warning'
  if (value === 'subscription') return 'badge-gray'
  return 'badge-gray'
}

function entitlementValueLabel(item: SupplierEntitlementTransaction): string {
  if (item.type === 'balance') return formatMoney(item.value_cents, item.currency)
  if (item.type === 'concurrency') return `+${formatNumber(item.raw_value)} 请求`
  if (item.type === 'subscription') return item.validity_days ? `${formatNumber(item.validity_days)} 天` : '订阅权益'
  if (item.raw_value !== undefined && item.raw_value !== null) return String(item.raw_value)
  return '-'
}

function ledgerTypeLabel(value: string): string {
  return {
    funding_credit: '充值入账',
    entitlement_credit: '兑换入账',
    refund_debit: '退款扣减',
    manual_adjustment: '手工调整',
    reversal: '冲正',
    usage_debit: '用量扣减'
  }[value] || value
}

function ledgerBadgeClass(value: string): string {
  if (value === 'refund_debit' || value === 'usage_debit') return 'badge-warning'
  if (value === 'manual_adjustment' || value === 'reversal') return 'badge-danger'
  return 'badge-success'
}

function syncJobStatusLabel(status?: SupplierProvisionStatus): string {
  if (status === 'queued') return '排队中'
  if (status === 'running') return '执行中'
  if (status === 'succeeded') return '已完成'
  if (status === 'partial_succeeded') return '部分完成'
  if (status === 'retryable_failed') return '等待重试'
  if (status === 'manual_required') return '需人工处理'
  if (status === 'dead') return '失败'
  if (status === 'cancelled') return '已取消'
  return '未知'
}

function syncJobCaption(job: SupplierProvisionJob): string {
  const prefix = `成本同步任务 #${job.id} ${syncJobStatusLabel(job.status)}`
  if (job.error_message) return `${prefix}：${job.error_message}`
  if (job.status === 'succeeded') return `${prefix}，正在刷新成本数据`
  if (job.status === 'retryable_failed') return `${prefix}，第 ${job.attempts}/${job.max_attempts} 次失败后等待重试`
  if (job.status === 'running') return `${prefix}，Worker 正在采集供应商事实`
  if (job.status === 'queued') return `${prefix}，等待 Worker 执行`
  return prefix
}

function isTerminalSyncJobStatus(status: SupplierProvisionStatus): boolean {
  return ['succeeded', 'partial_succeeded', 'manual_required', 'dead', 'cancelled'].includes(status)
}

function isTerminalRunStatus(status: string): boolean {
  return ['succeeded', 'partial_succeeded', 'retryable_failed', 'manual_required', 'dead', 'cancelled', 'skipped'].includes(status)
}

function stepResultLabel(snapshot?: Record<string, unknown>, fallback = 0): string {
  if (!snapshot) return String(fallback || 0)
  const fundingCount = numberFromSnapshot(snapshot, 'funding_transactions')
  const entitlementCount = numberFromSnapshot(snapshot, 'entitlement_transactions')
  const usageCount = numberFromSnapshot(snapshot, 'usage_cost_lines')
  const ledgerCount = numberFromSnapshot(snapshot, 'ledger_entries')
  return `充值 ${fundingCount}，兑换 ${entitlementCount}，用量 ${usageCount}，台账 ${ledgerCount}`
}

function numberFromSnapshot(snapshot: Record<string, unknown>, key: string): number {
  const value = snapshot[key]
  return typeof value === 'number' && Number.isFinite(value) ? value : 0
}

async function setTopTab(tab: TopTab) {
  activeTopTab.value = tab
  await loadCurrentTab()
}

async function setDetailTab(tab: DetailTab) {
  activeDetailTab.value = tab
  await loadDetailIfNeeded()
}

async function loadCurrentTab() {
  loading.value = true
  try {
    await loadSuppliersIfNeeded()
    if (activeTopTab.value === 'overview') {
      await loadLedgerOverview()
    } else if (activeTopTab.value === 'suppliers') {
      await loadSnapshots()
    } else if (activeTopTab.value === 'detail') {
      await ensureSelectedSupplier()
      await loadDetailIfNeeded(true)
    } else if (activeTopTab.value === 'backfill' && activeBackfillRun.value) {
      await refreshBackfillRun(activeBackfillRun.value.run.id)
    }
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '加载成本对账失败')
  } finally {
    loading.value = false
  }
}

async function loadSuppliersIfNeeded() {
  if (suppliersLoaded.value) return
  const supplierResult = await listSuppliers({ limit: 200 })
  suppliers.value = supplierResult.items
  suppliersLoaded.value = true
}

async function loadLedgerOverview() {
  ledgerOverview.value = await getSupplierCostLedgerOverview()
  overviewLoaded.value = true
}

async function loadSnapshots() {
  const snapshotResult = await listSupplierCostSnapshots({ page: 1, page_size: 200 })
  snapshots.value = snapshotResult.items
  snapshotsLoaded.value = true
}

async function ensureSelectedSupplier() {
  if (selectedSupplierId.value) return
  selectedSupplierId.value = snapshots.value[0]?.supplier_id || suppliers.value[0]?.id || 0
}

async function loadDetailIfNeeded(force = false) {
  if (!selectedSupplierId.value) return
  if (!force && detailLoadedSupplierId.value === selectedSupplierId.value) return
  const [summaryResult, fundingResult, entitlementResult, ledgerResult] = await Promise.all([
    getSupplierCostSummary(selectedSupplierId.value),
    listSupplierFundingTransactions(selectedSupplierId.value, { page: 1, page_size: 100 }),
    listSupplierEntitlementTransactions(selectedSupplierId.value, { page: 1, page_size: 100 }),
    listSupplierCostLedger(selectedSupplierId.value, { page: 1, page_size: 100 })
  ])
  const others = snapshots.value.filter((item) => item.supplier_id !== selectedSupplierId.value)
  snapshots.value = [...summaryResult.items, ...others]
  funding.value = fundingResult.items
  entitlements.value = entitlementResult.items
  ledger.value = ledgerResult.items
  snapshotsLoaded.value = true
  detailLoadedSupplierId.value = selectedSupplierId.value
}

function handleSupplierChange() {
  stopSyncJobPolling()
  activeSyncJob.value = null
  lastSync.value = null
  detailLoadedSupplierId.value = 0
  if (activeTopTab.value === 'detail' && selectedSupplierId.value) {
    void loadCurrentTab()
  }
}

function selectSnapshot(supplierID: number) {
  selectedSupplierId.value = supplierID
  detailLoadedSupplierId.value = 0
  activeTopTab.value = 'detail'
  void loadCurrentTab()
}

async function syncCosts() {
  if (!selectedSupplierId.value) {
    appStore.showError('请选择供应商')
    return
  }
  stopSyncJobPolling()
  syncing.value = true
  try {
    const job = await syncSupplierCosts(selectedSupplierId.value, syncPayload())
    appStore.showSuccess(`成本同步任务已提交 #${job.job_id}`)
    await watchSyncJob(job.job_id)
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '同步成本失败')
    syncing.value = false
  }
}

async function startHistoryBackfill() {
  stopBackfillPolling()
  backfilling.value = true
  activeTopTab.value = 'backfill'
  try {
    const run = await backfillSupplierCosts({
      ...syncPayload()
    })
    appStore.showSuccess(`历史回补已提交 #${run.id}`)
    await watchBackfillRun(run.id)
  } catch (error) {
    backfilling.value = false
    appStore.showError((error as { message?: string }).message || '提交历史回补失败')
  }
}

function syncPayload() {
  return {
    started_at: toRFC3339(syncForm.started_at),
    ended_at: toRFC3339(syncForm.ended_at),
    include_funding_transactions: true,
    include_entitlement_transactions: true,
    include_usage_cost_lines: true,
    include_balance_snapshot: true
  }
}

async function watchSyncJob(jobID: number) {
  stopSyncJobPolling()
  await refreshSyncJob(jobID)
}

async function refreshSyncJob(jobID: number) {
  try {
    const job = await getSupplierProvisionJob(jobID)
    activeSyncJob.value = job
    if (isTerminalSyncJobStatus(job.status)) {
      syncing.value = false
      if (job.result_snapshot) {
        lastSync.value = job.result_snapshot as unknown as SupplierCostSyncResultSnapshot
      }
      await Promise.all([loadDetailIfNeeded(true), overviewLoaded.value ? loadLedgerOverview() : Promise.resolve()])
      if (snapshotsLoaded.value) await loadSnapshots()
      if (job.status === 'succeeded' || job.status === 'partial_succeeded') {
        appStore.showSuccess('成本同步完成')
      } else if (job.error_message) {
        appStore.showError(job.error_message)
      }
      return
    }
    syncJobTimer = window.setTimeout(() => {
      void refreshSyncJob(jobID)
    }, 2000)
  } catch (error) {
    syncing.value = false
    appStore.showError((error as { message?: string }).message || '读取成本同步任务失败')
  }
}

async function watchBackfillRun(runID: string) {
  stopBackfillPolling()
  await refreshBackfillRun(runID)
}

async function refreshBackfillRun(runID: string) {
  try {
    const detail = await getSchedulerRunDetail(runID)
    activeBackfillRun.value = detail
    if (isTerminalRunStatus(detail.run.status)) {
      backfilling.value = false
      if (snapshotsLoaded.value) await loadSnapshots()
      if (overviewLoaded.value) await loadLedgerOverview()
      if (detail.run.status === 'succeeded' || detail.run.status === 'partial_succeeded') {
        appStore.showSuccess('历史回补完成')
      } else if (detail.run.error_message) {
        appStore.showError(detail.run.error_message)
      }
      return
    }
    backfillRunTimer = window.setTimeout(() => {
      void refreshBackfillRun(runID)
    }, 2500)
  } catch (error) {
    backfilling.value = false
    appStore.showError((error as { message?: string }).message || '读取历史回补状态失败')
  }
}

function stopSyncJobPolling() {
  if (!syncJobTimer) return
  window.clearTimeout(syncJobTimer)
  syncJobTimer = undefined
}

function stopBackfillPolling() {
  if (!backfillRunTimer) return
  window.clearTimeout(backfillRunTimer)
  backfillRunTimer = undefined
}

onMounted(loadCurrentTab)
onBeforeUnmount(() => {
  stopSyncJobPolling()
  stopBackfillPolling()
})
</script>
