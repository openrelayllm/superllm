<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">调度中心</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
            统一管理供应商采集、对账、会话维护、渠道检测和本地调度联动。
          </p>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <RouterLink :to="{ path: '/admin/action-audits', query: { window: '24h' } }" class="btn btn-secondary">
            <Icon name="clipboard" size="sm" />
            操作审计
          </RouterLink>
          <button type="button" class="btn btn-secondary" :disabled="loading" @click="loadPage">
            <Icon name="refresh" size="sm" />
            刷新
          </button>
          <button type="button" class="btn btn-primary" :disabled="running" @click="runBalanceSync">
            <Icon name="play" size="sm" />
            {{ running ? '运行中...' : '刷新余额' }}
          </button>
        </div>
      </section>

      <nav class="flex gap-2 overflow-x-auto border-b border-gray-200 dark:border-dark-700">
        <button
          v-for="tab in tabs"
          :key="tab.value"
          type="button"
          class="whitespace-nowrap border-b-2 px-3 py-2 text-sm font-medium"
          :class="activeTab === tab.value ? 'border-primary-500 text-primary-600 dark:text-primary-400' : 'border-transparent text-gray-500 hover:text-gray-900 dark:text-dark-400 dark:hover:text-white'"
          @click="activeTab = tab.value"
        >
          {{ tab.label }}
        </button>
      </nav>

      <section v-if="activeTab === 'dashboard'" class="grid gap-6 xl:grid-cols-[minmax(0,1.7fr)_minmax(320px,0.8fr)]">
        <div class="space-y-6">
          <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
            <div class="card p-4">
              <p class="text-xs font-medium text-gray-500 dark:text-dark-400">Worker</p>
              <p class="mt-2 text-2xl font-semibold" :class="statusClass(status?.worker_status)">
                {{ workerLabel }}
              </p>
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ intervalLabel }}</p>
            </div>
            <div class="card p-4">
              <p class="text-xs font-medium text-gray-500 dark:text-dark-400">运行中</p>
              <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ status?.running_steps || 0 }}</p>
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">当前 step</p>
            </div>
            <div class="card p-4">
              <p class="text-xs font-medium text-gray-500 dark:text-dark-400">失败</p>
              <p class="mt-2 text-2xl font-semibold text-rose-600 dark:text-rose-400">{{ status?.failed_steps || 0 }}</p>
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">最近 run 聚合</p>
            </div>
            <div class="card p-4">
              <p class="text-xs font-medium text-gray-500 dark:text-dark-400">待处理</p>
              <p class="mt-2 text-2xl font-semibold text-amber-600 dark:text-amber-400">{{ actions.length }}</p>
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">智能动作</p>
            </div>
          </div>

          <div class="card overflow-hidden">
            <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">今日工作台</h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">按优先级处理供应商自动化风险。</p>
            </div>
            <div class="divide-y divide-gray-100 dark:divide-dark-700">
              <div v-if="actions.length === 0" class="px-5 py-10 text-center text-sm text-gray-500 dark:text-dark-400">
                暂无待处理动作。
              </div>
              <div v-for="action in actions.slice(0, 6)" :key="action.id" class="flex flex-col gap-3 px-5 py-4 md:flex-row md:items-center md:justify-between">
                <div class="min-w-0">
                  <div class="flex flex-wrap items-center gap-2">
                    <span class="badge" :class="severityClass(action.severity)">{{ severityLabel(action.severity) }}</span>
                    <span class="text-sm font-medium text-gray-900 dark:text-white">{{ action.title }}</span>
                  </div>
                  <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ action.supplier_name || '-' }} · {{ action.reason }}</p>
                </div>
                <button type="button" class="btn btn-secondary shrink-0" @click="handleWorkbenchAction(action)">
                  {{ workbenchActionLabel(action) }}
                </button>
              </div>
            </div>
          </div>

          <div class="card overflow-hidden">
            <div class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700">
              <div>
                <h2 class="text-lg font-semibold text-gray-900 dark:text-white">本地调度分组容量</h2>
                <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">按用户 Key 和可调度账号定位空池风险。</p>
              </div>
              <RouterLink to="/admin/local-account-ops" class="btn btn-secondary btn-sm">
                <Icon name="externalLink" size="sm" />
                本地账号运营
              </RouterLink>
            </div>
            <div class="overflow-x-auto">
              <table class="w-full min-w-[820px] divide-y divide-gray-200 dark:divide-dark-700">
                <thead class="bg-gray-50 dark:bg-dark-800">
                  <tr>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">本地分组</th>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">状态</th>
                    <th class="px-4 py-3 text-right text-xs font-medium uppercase text-gray-500 dark:text-dark-400">用户 Key</th>
                    <th class="px-4 py-3 text-right text-xs font-medium uppercase text-gray-500 dark:text-dark-400">账号</th>
                    <th class="px-4 py-3 text-right text-xs font-medium uppercase text-gray-500 dark:text-dark-400">可调度</th>
                    <th class="px-4 py-3 text-right text-xs font-medium uppercase text-gray-500 dark:text-dark-400">倍率</th>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">操作</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-100 bg-white dark:divide-dark-700 dark:bg-dark-900">
                  <tr v-if="localGroups.length === 0">
                    <td colspan="7" class="px-4 py-8 text-center text-sm text-gray-500 dark:text-dark-400">暂无本地分组</td>
                  </tr>
                  <tr v-for="group in localGroupCapacityRows" :key="group.id">
                    <td class="px-4 py-3">
                      <p class="text-sm font-medium text-gray-900 dark:text-white">{{ group.name || `#${group.id}` }}</p>
                      <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ group.platform || '-' }} · {{ group.status || '-' }}</p>
                    </td>
                    <td class="px-4 py-3">
                      <span class="badge" :class="localGroupCapacityClass(group)">{{ localGroupCapacityLabel(group) }}</span>
                    </td>
                    <td class="px-4 py-3 text-right text-sm text-gray-700 dark:text-dark-200">{{ group.active_api_key_count }}</td>
                    <td class="px-4 py-3 text-right text-sm text-gray-700 dark:text-dark-200">{{ group.total_accounts }}</td>
                    <td class="px-4 py-3 text-right text-sm font-medium" :class="group.active_api_key_count > 0 && group.schedulable_accounts === 0 ? 'text-rose-600 dark:text-rose-300' : 'text-gray-900 dark:text-white'">
                      {{ group.schedulable_accounts }}
                    </td>
                    <td class="px-4 py-3 text-right text-sm text-gray-700 dark:text-dark-200">{{ candidateRateLabel(group.rate_multiplier) }}</td>
                    <td class="px-4 py-3">
                      <button type="button" class="btn btn-secondary btn-sm" :disabled="refillBusy" @click="previewRoutingRefillForGroup(group.id)">
                        <Icon name="search" size="sm" :class="{ 'animate-spin': refillBusy && refillGroupId === group.id }" />
                        预览补池
                      </button>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>

          <div class="card overflow-hidden">
            <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">最近运行</h2>
            </div>
            <div class="overflow-x-auto">
              <table class="w-full min-w-[760px] divide-y divide-gray-200 dark:divide-dark-700">
                <thead class="bg-gray-50 dark:bg-dark-800">
                  <tr>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">任务</th>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">状态</th>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">供应商</th>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">Step</th>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">时间</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-100 bg-white dark:divide-dark-700 dark:bg-dark-900">
                  <tr v-if="runs.length === 0">
                    <td colspan="5" class="px-4 py-8 text-center text-sm text-gray-500 dark:text-dark-400">暂无运行记录</td>
                  </tr>
                  <tr v-for="run in runs.slice(0, 5)" :key="run.id">
                    <td class="px-4 py-3 text-sm text-gray-900 dark:text-gray-100">{{ taskLabel(run.task_type) }}</td>
                    <td class="px-4 py-3"><span class="badge" :class="runStatusClass(run.status)">{{ runStatusLabel(run.status) }}</span></td>
                    <td class="px-4 py-3 text-sm text-gray-500 dark:text-dark-400">{{ run.supplier_count }}</td>
                    <td class="px-4 py-3 text-sm text-gray-500 dark:text-dark-400">{{ run.succeeded_steps }}/{{ run.total_steps }}</td>
                    <td class="px-4 py-3 text-sm text-gray-500 dark:text-dark-400">{{ runPrimaryTime(run) }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>

        <aside class="space-y-6">
          <div class="card p-5">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">系统健康</h2>
            <dl class="mt-4 space-y-3 text-sm">
              <div class="flex items-center justify-between">
                <dt class="text-gray-500 dark:text-dark-400">队列</dt>
                <dd class="font-medium text-gray-900 dark:text-white">{{ status?.queue || '-' }}</dd>
              </div>
              <div class="flex items-center justify-between">
                <dt class="text-gray-500 dark:text-dark-400">下次运行</dt>
                <dd class="font-medium text-gray-900 dark:text-white">{{ nextRunLabel }}</dd>
              </div>
              <div class="flex items-center justify-between">
                <dt class="text-gray-500 dark:text-dark-400">最近运行</dt>
                <dd class="font-medium text-gray-900 dark:text-white">{{ formatDateTime(status?.last_run_at) || '-' }}</dd>
              </div>
              <div class="flex items-center justify-between">
                <dt class="text-gray-500 dark:text-dark-400">渠道检测</dt>
                <dd class="font-medium" :class="settings?.channel_checks_enabled ? 'text-emerald-600 dark:text-emerald-400' : 'text-gray-500 dark:text-dark-400'">
                  {{ settings?.channel_checks_enabled ? '已启用' : '默认关闭' }}
                </dd>
              </div>
            </dl>
          </div>

          <div class="card p-5">
            <div class="flex items-center justify-between gap-3">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">代理出口</h2>
              <RouterLink to="/admin/proxy#egress" class="text-sm text-primary-600 hover:text-primary-700 dark:text-primary-400">管理</RouterLink>
            </div>
            <dl class="mt-4 space-y-3 text-sm">
              <div class="flex items-center justify-between">
                <dt class="text-gray-500 dark:text-dark-400">运行状态</dt>
                <dd class="font-medium" :class="proxyStatus?.proxy_enabled === false ? 'text-rose-600 dark:text-rose-400' : 'text-emerald-600 dark:text-emerald-400'">
                  {{ proxyStatus?.proxy_enabled === false ? '停用' : '启用' }}
                </dd>
              </div>
              <div class="flex items-center justify-between">
                <dt class="text-gray-500 dark:text-dark-400">Mihomo</dt>
                <dd class="font-medium" :class="proxyStatus?.mihomo_configured ? 'text-emerald-600 dark:text-emerald-400' : 'text-amber-600 dark:text-amber-400'">
                  {{ proxyStatus?.mihomo_configured ? '已配置' : '未配置' }}
                </dd>
              </div>
              <div class="flex items-center justify-between">
                <dt class="text-gray-500 dark:text-dark-400">健康节点</dt>
                <dd class="font-medium text-gray-900 dark:text-white">{{ proxyStatus?.healthy_nodes || 0 }} / {{ proxyStatus?.nodes_total || 0 }}</dd>
              </div>
              <div class="flex items-center justify-between">
                <dt class="text-gray-500 dark:text-dark-400">运行槽位</dt>
                <dd class="font-medium text-gray-900 dark:text-white">{{ proxyStatus?.slots_assigned || 0 }} / {{ proxyStatus?.max_slots || proxyStatus?.slots_total || 0 }}</dd>
              </div>
              <div class="flex items-center justify-between">
                <dt class="text-gray-500 dark:text-dark-400">活跃绑定</dt>
                <dd class="font-medium text-gray-900 dark:text-white">{{ proxyStatus?.assignments_active || 0 }}</dd>
              </div>
              <div class="flex items-center justify-between">
                <dt class="text-gray-500 dark:text-dark-400">24h 切换</dt>
                <dd class="font-medium text-gray-900 dark:text-white">{{ proxyStatus?.node_switches_24h || 0 }}</dd>
              </div>
              <div class="flex items-center justify-between">
                <dt class="text-gray-500 dark:text-dark-400">24h 失败</dt>
                <dd class="font-medium" :class="(proxyStatus?.node_failures_24h || 0) > 0 ? 'text-amber-600 dark:text-amber-400' : 'text-gray-900 dark:text-white'">
                  {{ proxyStatus?.node_failures_24h || 0 }}
                </dd>
              </div>
              <div class="flex items-center justify-between">
                <dt class="text-gray-500 dark:text-dark-400">平均绑定</dt>
                <dd class="font-medium text-gray-900 dark:text-white">{{ formatProxyDurationSeconds(proxyStatus?.avg_assignment_seconds_24h || 0) }}</dd>
              </div>
            </dl>
          </div>

          <div class="card p-5">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">供应商自动化</h2>
            <div class="mt-4 space-y-3">
              <div v-for="item in supplierStatuses.slice(0, 5)" :key="item.supplier_id">
                <div class="flex items-center justify-between text-sm">
                  <span class="font-medium text-gray-900 dark:text-white">{{ item.supplier_name }}</span>
                  <span class="text-gray-500 dark:text-dark-400">{{ item.completion_percent }}%</span>
                </div>
                <div class="mt-2 h-2 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-800">
                  <div class="h-full bg-primary-500" :style="{ width: `${item.completion_percent}%` }"></div>
                </div>
              </div>
            </div>
            <button type="button" class="btn btn-secondary mt-5 w-full" @click="activeTab = 'suppliers'">
              查看 Checklist
            </button>
          </div>

          <div class="card p-5">
            <div class="flex items-start justify-between gap-3">
              <div>
                <h2 class="text-lg font-semibold text-gray-900 dark:text-white">路由补池</h2>
                <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">本地调度分组耗尽时补入最低倍率候选。</p>
              </div>
              <RouterLink to="/admin/local-account-ops" class="text-sm text-primary-600 hover:text-primary-700 dark:text-primary-400">运营页</RouterLink>
            </div>
            <label class="mt-4 block">
              <span class="input-label">目标本地分组</span>
              <select v-model.number="refillGroupId" class="input">
                <option :value="0">选择分组</option>
                <option v-for="group in refillLocalGroupOptions" :key="group.id" :value="group.id">
                  {{ group.name }} · 可调度 {{ group.schedulable_accounts }} · Key {{ group.active_api_key_count }}
                </option>
              </select>
            </label>
            <dl class="mt-3 grid grid-cols-2 gap-2 text-xs text-gray-500 dark:text-dark-400">
              <div>
                <dt>最高倍率</dt>
                <dd class="mt-0.5 font-medium text-gray-900 dark:text-white">{{ routingRefillPolicyRateLabel }}</dd>
              </div>
              <div>
                <dt>冷却</dt>
                <dd class="mt-0.5 font-medium text-gray-900 dark:text-white">{{ formatProxyDurationSeconds(settingsForm.routing_refill_cooldown_seconds) }}</dd>
              </div>
            </dl>
            <div class="mt-3 grid grid-cols-2 gap-2">
              <button type="button" class="btn btn-secondary btn-sm justify-center" :disabled="refillDisabled" @click="previewRoutingRefill">
                <Icon name="search" size="sm" :class="{ 'animate-spin': refillBusy }" />
                预览
              </button>
              <button type="button" class="btn btn-primary btn-sm justify-center" :disabled="refillDisabled" @click="applyRoutingRefill">
                <Icon name="plus" size="sm" :class="{ 'animate-spin': refillBusy }" />
                补入
              </button>
            </div>
            <div v-if="refillResult" class="mt-3 rounded-md border border-gray-100 bg-gray-50 p-3 text-sm dark:border-dark-700 dark:bg-dark-800">
              <div class="flex flex-wrap items-center justify-between gap-2">
                <span class="font-medium text-gray-900 dark:text-white">{{ refillResultTitle }}</span>
                <span v-if="refillResult.candidate" class="badge badge-success">
                  {{ routingRefillMultiplierLabel(refillResult.candidate.effective_rate_multiplier) }}
                </span>
                <span v-else class="badge badge-gray">{{ routingRefillSkippedReasonLabel(refillResult.skipped_reason) }}</span>
              </div>
              <p v-if="refillResult.candidate" class="mt-1 truncate text-gray-600 dark:text-dark-300" :title="refillCandidateLabel">
                {{ refillCandidateLabel }}
              </p>
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                可调度账号 {{ refillResult.availability_before?.schedulable_accounts ?? '-' }}
                <template v-if="refillResult.availability_after"> -> {{ refillResult.availability_after.schedulable_accounts }}</template>
                · 用户 Key {{ refillResult.availability_before?.active_api_key_count ?? '-' }}
              </p>
              <RoutingRefillImpactPanel :availability="refillResult.availability_before" />
            </div>
            <div v-if="refillRuns.length > 0" class="mt-4 border-t border-gray-100 pt-3 dark:border-dark-700">
              <div class="flex items-center justify-between gap-2">
                <p class="text-xs font-medium text-gray-500 dark:text-dark-400">最近补池</p>
                <button type="button" class="text-xs text-primary-600 hover:text-primary-700 dark:text-primary-400" :disabled="loading" @click="loadPage">刷新</button>
              </div>
              <div class="mt-2 space-y-2">
                <div v-for="run in refillRuns.slice(0, 5)" :key="run.id" class="rounded-md border border-gray-100 p-2 text-xs dark:border-dark-700">
                  <div class="flex flex-wrap items-center justify-between gap-2">
                    <span class="font-medium text-gray-900 dark:text-white">{{ run.local_group_name || `#${run.local_group_id}` }}</span>
                    <span class="badge" :class="routingRefillRunStatusClass(run.status)">{{ routingRefillRunStatusLabel(run.status) }}</span>
                  </div>
                  <p class="mt-1 truncate text-gray-500 dark:text-dark-400" :title="routingRefillRunTitle(run)">
                    {{ routingRefillRunTitle(run) }}
                  </p>
                  <p class="mt-1 text-gray-400 dark:text-dark-500">
                    {{ formatDateTime(run.created_at) }} · 可调度 {{ run.before_schedulable_accounts }}
                    <template v-if="run.after_schedulable_accounts > 0 || run.status === 'succeeded'"> -> {{ run.after_schedulable_accounts }}</template>
                  </p>
                </div>
              </div>
            </div>
          </div>
        </aside>
      </section>

      <SchedulerPlansPanel
        v-else-if="activeTab === 'plans'"
        :plans="plans"
        :running="running"
        :updating-plan-id="updatingPlanId"
        @run="runPlan"
        @status="setPlanStatus"
        @save="savePlanConfig"
      />

      <section v-else-if="activeTab === 'runs'" class="card overflow-hidden">
        <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">运行记录</h2>
        </div>
        <div class="overflow-x-auto">
          <table class="w-full min-w-[1260px] divide-y divide-gray-200 dark:divide-dark-700">
            <thead class="bg-gray-50 dark:bg-dark-800">
              <tr>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">Run</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">任务</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">触发</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">状态</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">Step</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">时间</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">耗时</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">错误</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">操作</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 bg-white dark:divide-dark-700 dark:bg-dark-900">
              <tr v-if="runs.length === 0">
                <td colspan="9" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-dark-400">暂无运行记录。点击“刷新余额”会创建一条真实 run。</td>
              </tr>
              <tr v-for="run in runs" :key="run.id">
                <td class="px-4 py-4 font-mono text-xs text-gray-500 dark:text-dark-400">{{ run.id }}</td>
                <td class="px-4 py-4 text-sm text-gray-900 dark:text-gray-100">{{ taskLabel(run.task_type) }}</td>
                <td class="px-4 py-4 text-sm text-gray-500 dark:text-dark-400">{{ run.trigger_type }}</td>
                <td class="px-4 py-4"><span class="badge" :class="runStatusClass(run.status)">{{ runStatusLabel(run.status) }}</span></td>
                <td class="px-4 py-4 text-sm text-gray-500 dark:text-dark-400">{{ run.succeeded_steps }}/{{ run.total_steps }} 成功，{{ run.failed_steps }} 失败</td>
                <td class="px-4 py-4 text-sm text-gray-500 dark:text-dark-400">
                  <div>请求 {{ runPrimaryTime(run) }}</div>
                  <div v-if="run.started_at" class="mt-1 text-xs text-gray-400 dark:text-dark-500">开始 {{ formatDateTime(run.started_at) || '-' }}</div>
                  <div v-if="run.finished_at" class="mt-1 text-xs text-gray-400 dark:text-dark-500">完成 {{ formatDateTime(run.finished_at) || '-' }}</div>
                </td>
                <td class="px-4 py-4 text-sm text-gray-500 dark:text-dark-400">{{ run.duration_ms }} ms</td>
                <td class="px-4 py-4 text-sm text-gray-500 dark:text-dark-400">{{ run.error_message || '-' }}</td>
                <td class="px-4 py-4">
                  <div class="flex flex-wrap gap-2">
                    <button type="button" class="btn btn-secondary btn-sm" @click="openRunDetail(run)">详情</button>
                    <button
                      type="button"
                      class="btn btn-secondary btn-sm"
                      :disabled="updatingRunId === run.id || !runRetryable(run.status, run.failed_steps)"
                      @click="retryRunFailedSteps(run)"
                    >
                      重试失败
                    </button>
                    <button
                      type="button"
                      class="btn btn-secondary btn-sm"
                      :disabled="updatingRunId === run.id || !runCancellable(run.status)"
                      @click="cancelRun(run)"
                    >
                      取消
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <SchedulerSupplierAutomationPanel
        v-else-if="activeTab === 'suppliers'"
        :suppliers="supplierStatuses"
        :running-action-key="runningSupplierActionKey"
        @action="runSupplierAutomationAction"
        @checklist="openSupplierChecklist"
      />

      <section v-else-if="activeTab === 'actions'" class="card overflow-hidden">
        <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">智能动作</h2>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">按状态机单步处理异常供应商事项。</p>
        </div>
        <div class="divide-y divide-gray-100 dark:divide-dark-700">
          <div v-if="actions.length === 0" class="px-5 py-10 text-center text-sm text-gray-500 dark:text-dark-400">暂无智能动作</div>
          <div v-for="action in actions" :key="action.id" class="grid gap-4 px-5 py-4 lg:grid-cols-[180px_minmax(0,1fr)_260px] lg:items-center">
            <div>
              <span class="badge" :class="severityClass(action.severity)">{{ severityLabel(action.severity) }}</span>
              <p class="mt-2 text-xs text-gray-500 dark:text-dark-400">{{ actionStatusLabel(action.status) }}</p>
            </div>
            <div>
              <p class="font-medium text-gray-900 dark:text-white">{{ action.title }}</p>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ action.supplier_name || '-' }} · {{ action.reason }}</p>
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ action.recommended_operation || '查看证据' }}</p>
            </div>
            <div class="flex flex-wrap justify-start gap-2 lg:justify-end">
              <RouterLink
                v-if="isRoutingRefillAction(action) || isLocalAccountDisableAction(action)"
                :to="actionRecommendationsRoute(action)"
                class="btn btn-secondary btn-sm"
              >
                <Icon name="clipboard" size="sm" />
                动作建议
              </RouterLink>
              <button type="button" class="btn btn-secondary btn-sm" :disabled="updatingActionId === action.id" @click="setActionStatus(action, 'investigating')">处理中</button>
              <button type="button" class="btn btn-secondary btn-sm" :disabled="updatingActionId === action.id" @click="setActionStatus(action, 'ignored')">忽略</button>
              <button type="button" class="btn btn-primary btn-sm" :disabled="updatingActionId === action.id" @click="setActionStatus(action, 'resolved')">标记处理</button>
            </div>
          </div>
        </div>
      </section>

      <section v-else class="grid gap-6 lg:grid-cols-2">
        <div class="card p-5">
          <div class="flex items-center justify-between gap-3">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">全局调度</h2>
            <button type="button" class="btn btn-primary btn-sm" :disabled="settingsSaving" @click="saveSettings">
              {{ settingsSaving ? '保存中...' : '保存' }}
            </button>
          </div>
          <div class="mt-4 space-y-4 text-sm">
            <label class="flex items-center justify-between gap-4">
              <span class="text-gray-500 dark:text-dark-400">调度中心</span>
              <input v-model="settingsForm.enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
            </label>
            <label class="flex items-center justify-between gap-4">
              <span class="text-gray-500 dark:text-dark-400">渠道检测</span>
              <input v-model="settingsForm.channel_checks_enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
            </label>
            <label class="block">
              <span class="text-gray-500 dark:text-dark-400">单供应商并发</span>
              <input v-model.number="settingsForm.default_supplier_concurrency" type="number" min="1" max="20" class="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-800 dark:text-white" />
            </label>
            <label class="block">
              <span class="text-gray-500 dark:text-dark-400">渠道检测每日 token 预算</span>
              <input v-model.number="settingsForm.channel_check_daily_budget_tokens" type="number" min="0" class="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-800 dark:text-white" />
            </label>
            <label class="block">
              <span class="text-gray-500 dark:text-dark-400">同分组实测冷却秒数</span>
              <input v-model.number="settingsForm.channel_check_probe_cooldown_seconds" type="number" min="0" max="86400" class="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-800 dark:text-white" />
            </label>
            <label class="block">
              <span class="text-gray-500 dark:text-dark-400">首 token 慢阈值 ms</span>
              <input v-model.number="settingsForm.first_token_slow_threshold_ms" type="number" min="0" class="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-800 dark:text-white" />
            </label>
            <label class="block">
              <span class="text-gray-500 dark:text-dark-400">总耗时慢阈值 ms</span>
              <input v-model.number="settingsForm.total_latency_slow_threshold_ms" type="number" min="0" class="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-800 dark:text-white" />
            </label>
          </div>
        </div>

        <div class="card p-5">
          <div class="flex items-center justify-between gap-3">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">路由补池策略</h2>
            <span class="badge" :class="settingsForm.routing_refill_auto_execute_enabled ? 'badge-warning' : 'badge-gray'">
              {{ settingsForm.routing_refill_auto_execute_enabled ? '自动执行' : '人工确认' }}
            </span>
          </div>
          <div class="mt-4 space-y-4 text-sm">
            <label class="flex items-center justify-between gap-4">
              <span class="text-gray-500 dark:text-dark-400">自动补池</span>
              <input v-model="settingsForm.routing_refill_auto_execute_enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
            </label>
            <label class="block">
              <span class="text-gray-500 dark:text-dark-400">低容量阈值</span>
              <input v-model.number="settingsForm.routing_refill_low_capacity_threshold" type="number" min="1" max="100" class="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-800 dark:text-white" />
            </label>
            <label class="block">
              <span class="text-gray-500 dark:text-dark-400">补池冷却秒数</span>
              <input v-model.number="settingsForm.routing_refill_cooldown_seconds" type="number" min="1" max="86400" class="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-800 dark:text-white" />
            </label>
            <label class="block">
              <span class="text-gray-500 dark:text-dark-400">确认窗口秒数</span>
              <input v-model.number="settingsForm.routing_refill_confirm_window_seconds" type="number" min="0" max="86400" class="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-800 dark:text-white" />
            </label>
            <label class="block">
              <span class="text-gray-500 dark:text-dark-400">最高倍率</span>
              <input v-model.number="settingsForm.routing_refill_max_rate_multiplier" type="number" min="0" step="0.0001" class="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-800 dark:text-white" />
            </label>
          </div>
        </div>

        <div class="card p-5">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">默认任务</h2>
          <div class="mt-4 flex flex-wrap gap-2">
            <span v-for="task in settingsForm.default_enabled_task_types" :key="task" class="badge badge-gray">
              {{ taskLabel(task) }}
            </span>
          </div>
          <h3 class="mt-6 text-sm font-semibold text-gray-900 dark:text-white">高成本任务</h3>
          <div class="mt-3 flex flex-wrap gap-2">
            <span v-for="task in settingsForm.high_cost_task_types" :key="task" class="badge badge-warning">
              {{ taskLabel(task) }}
            </span>
          </div>
        </div>
      </section>
    </div>
    <SchedulerRunDetailDialog
      :show="runDetailOpen"
      :detail="runDetail"
      :loading="runDetailLoading"
      :retrying-step-id="retryingStepId"
      :cancelling-step-id="cancellingStepId"
      :focused-step-id="focusedStepID"
      @close="closeRunDetail"
      @retry-step="retryStep"
      @cancel-step="cancelStep"
      @refresh="refreshOpenRunDetail(true)"
    />
    <SchedulerSupplierChecklistDialog
      :show="supplierChecklistOpen"
      :checklist="supplierChecklist"
      :loading="supplierChecklistLoading"
      :running-action-key="runningSupplierActionKey"
      @close="closeSupplierChecklist"
      @action="runSupplierChecklistAction"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import RoutingRefillImpactPanel from '@/views/admin/RoutingRefillImpactPanel.vue'
import SchedulerPlansPanel from './SchedulerPlansPanel.vue'
import SchedulerRunDetailDialog from './SchedulerRunDetailDialog.vue'
import SchedulerSupplierChecklistDialog from './SchedulerSupplierChecklistDialog.vue'
import SchedulerSupplierAutomationPanel from './SchedulerSupplierAutomationPanel.vue'
import { useAppStore } from '@/stores/app'
import {
  cancelSchedulerRun,
  cancelSchedulerStep,
  createSchedulerRun,
  getProxyCenterStatus,
  getSchedulerCenterStatus,
  getSchedulerSettings,
  getSchedulerRunDetail,
  getSchedulerSupplierChecklist,
  listSchedulerActions,
  listLocalSub2APIGroups,
  listRoutingRefillRuns,
  listSchedulerPlans,
  listSchedulerRuns,
  listSchedulerSupplierStatuses,
  loginSupplierSession,
  refillLocalGroup,
  retrySchedulerRunFailedSteps,
  retrySchedulerStep,
  updateSchedulerActionStatus,
  updateSchedulerPlanConfig,
  updateSchedulerPlanStatus,
  updateSchedulerSettings,
  type ExtensionTaskType,
  type LocalSub2APIGroup,
  type ProxyCenterStatus,
  type RoutingRefillResult,
  type RoutingRefillRun,
  type SchedulerAction,
  type SchedulerCenterStatus,
  type SchedulerPlan,
  type SchedulerPlanConfig,
  type SchedulerRunDetail,
  type SchedulerRunSummary,
  type SchedulerSettings,
  type SchedulerStepRecord,
  type SchedulerSupplierChecklist,
  type SchedulerSupplierStatus
} from '@/api/admin/adminPlus'
import {
  routingRefillMultiplierLabel,
  routingRefillRunStatusClass,
  routingRefillRunStatusLabel,
  routingRefillSkippedReasonLabel
} from '@/views/admin/routingRefillPresentation'
import {
  actionStatusLabel,
  candidateRateLabel,
  formatDateTime,
  planManualTaskTypes,
  planStatusLabel,
  runCancellable,
  runRetryable,
  runStatusClass,
  runStatusLabel,
  schedulerTabs,
  severityClass,
  severityLabel,
  statusClass,
  taskLabel
} from './presentation'
import {
  checklistActionForKey,
  supplierActionKey,
  supplierActionLabel,
  supplierActionTaskTypes,
  type SupplierAutomationAction
} from './supplierAutomation'

type TabValue = 'dashboard' | 'plans' | 'runs' | 'suppliers' | 'actions' | 'settings'

const appStore = useAppStore()
const route = useRoute()
const router = useRouter()
const loading = ref(false)
const running = ref(false)
const settingsSaving = ref(false)
const updatingPlanId = ref<string | null>(null)
const updatingActionId = ref<string | null>(null)
const updatingRunId = ref<string | null>(null)
const runningSupplierActionKey = ref<string | null>(null)
const refillBusy = ref(false)
const runDetailOpen = ref(false)
const runDetailLoading = ref(false)
const retryingStepId = ref<number | null>(null)
const cancellingStepId = ref<number | null>(null)
const supplierChecklistOpen = ref(false)
const supplierChecklistLoading = ref(false)
const activeTab = ref<TabValue>('dashboard')
const status = ref<SchedulerCenterStatus | null>(null)
const settings = ref<SchedulerSettings | null>(null)
const plans = ref<SchedulerPlan[]>([])
const runs = ref<SchedulerRunSummary[]>([])
const supplierStatuses = ref<SchedulerSupplierStatus[]>([])
const actions = ref<SchedulerAction[]>([])
const proxyStatus = ref<ProxyCenterStatus | null>(null)
const localGroups = ref<LocalSub2APIGroup[]>([])
const refillGroupId = ref(0)
const refillResult = ref<RoutingRefillResult | null>(null)
const refillRuns = ref<RoutingRefillRun[]>([])
const runDetail = ref<SchedulerRunDetail | null>(null)
const supplierChecklist = ref<SchedulerSupplierChecklist | null>(null)
const settingsForm = reactive<SchedulerSettings>({
  enabled: true,
  default_supplier_concurrency: 1,
  channel_checks_enabled: false,
  channel_check_daily_budget_tokens: 0,
  channel_check_probe_cooldown_seconds: 600,
  first_token_slow_threshold_ms: 0,
  total_latency_slow_threshold_ms: 0,
  routing_refill_auto_execute_enabled: false,
  routing_refill_low_capacity_threshold: 1,
  routing_refill_cooldown_seconds: 180,
  routing_refill_confirm_window_seconds: 0,
  routing_refill_max_rate_multiplier: 0,
  default_enabled_task_types: [],
  high_cost_task_types: []
})

const tabs = schedulerTabs

const intervalLabel = computed(() => {
  const seconds = status.value?.interval_seconds || 0
  if (seconds <= 0) return '未配置'
  if (seconds % 60 === 0) return `${seconds / 60} 分钟周期`
  return `${seconds} 秒周期`
})

const workerLabel = computed(() => {
  const value = status.value?.worker_status || 'unknown'
  return {
    running: '运行中',
    paused: '已暂停',
    degraded: '降级',
    down: '停止'
  }[value] || value
})

const nextRunLabel = computed(() => {
  const current = status.value
  if (!current?.next_run_at) return '-'
  const formatted = formatDateTime(current.next_run_at) || '-'
  if ((current.overdue_plans || 0) > 0) {
    return `待调度 ${current.overdue_plans} 个 · ${formatted}`
  }
  return formatted
})

const refillLocalGroupOptions = computed(() => {
  return [...localGroups.value]
    .sort((a, b) => a.name.localeCompare(b.name))
})

const refillDisabled = computed(() => loading.value || refillBusy.value || refillGroupId.value <= 0)

const refillResultTitle = computed(() => {
  if (!refillResult.value) return ''
  if (refillResult.value.skipped_reason) return `补池跳过：${routingRefillSkippedReasonLabel(refillResult.value.skipped_reason)}`
  return refillResult.value.dry_run ? '补池预览候选' : '补池已执行'
})

const refillCandidateLabel = computed(() => {
  const candidate = refillResult.value?.candidate
  if (!candidate) return ''
  return [
    candidate.supplier_name || '-',
    candidate.supplier_group_name || '-',
    candidate.local_account_name || `#${candidate.local_sub2api_account_id}`
  ].join(' / ')
})

const routingRefillPolicyRateLabel = computed(() => {
  const rate = normalizedRateMultiplier(settingsForm.routing_refill_max_rate_multiplier)
  return rate > 0 ? routingRefillMultiplierLabel(rate) : '不限'
})

const routingLowCapacityThreshold = computed(() => {
  return clampInteger(settingsForm.routing_refill_low_capacity_threshold, 1, 1, 100)
})
const focusedStepID = computed(() => positiveQueryNumber(route.query.step_id))

const localGroupCapacityRows = computed(() => {
  return [...localGroups.value].sort((a, b) => {
    const leftEmpty = a.active_api_key_count > 0 && a.schedulable_accounts === 0
    const rightEmpty = b.active_api_key_count > 0 && b.schedulable_accounts === 0
    if (leftEmpty !== rightEmpty) return leftEmpty ? -1 : 1
    if (a.schedulable_accounts !== b.schedulable_accounts) return a.schedulable_accounts - b.schedulable_accounts
    return a.name.localeCompare(b.name)
  })
})

async function loadPage() {
  loading.value = true
  try {
    const [nextStatus, nextPlans, nextRuns, nextSuppliers, nextActions, nextSettings, nextProxyStatus, nextLocalGroups, nextRefillRuns] = await Promise.all([
      getSchedulerCenterStatus(),
      listSchedulerPlans(),
      listSchedulerRuns({ limit: 30 }),
      listSchedulerSupplierStatuses(),
      listSchedulerActions(),
      getSchedulerSettings(),
      getProxyCenterStatus(),
      listLocalSub2APIGroups({ limit: 1000 }),
      listRoutingRefillRuns({ limit: 5 })
    ])
    status.value = nextStatus
    plans.value = nextPlans
    runs.value = nextRuns
    supplierStatuses.value = nextSuppliers
    actions.value = nextActions
    settings.value = nextSettings
    proxyStatus.value = nextProxyStatus
    localGroups.value = nextLocalGroups.items || []
    refillRuns.value = nextRefillRuns.items || []
    pruneRefillGroup()
    syncSettingsForm(nextSettings)
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '加载调度中心失败')
  } finally {
    loading.value = false
  }
}

function routingRefillRunTitle(run: RoutingRefillRun): string {
  if (run.status === 'failed') return run.error_message || run.error_code || '补池失败'
  if (run.skipped_reason) return routingRefillSkippedReasonLabel(run.skipped_reason)
  if (run.selected_local_account_id) {
    return `账号 #${run.selected_local_account_id} · ${routingRefillMultiplierLabel(run.selected_effective_rate_multiplier)}`
  }
  return run.reason || '-'
}

function localGroupCapacityLabel(group: LocalSub2APIGroup): string {
  if (group.active_api_key_count > 0 && group.schedulable_accounts === 0) return '空池'
  if (group.active_api_key_count > 0 && group.schedulable_accounts <= routingLowCapacityThreshold.value) return '低容量'
  if (group.active_api_key_count === 0) return '未服务'
  return '正常'
}

function localGroupCapacityClass(group: LocalSub2APIGroup): string {
  if (group.active_api_key_count > 0 && group.schedulable_accounts === 0) return 'badge-danger'
  if (group.active_api_key_count > 0 && group.schedulable_accounts <= routingLowCapacityThreshold.value) return 'badge-warning'
  if (group.active_api_key_count === 0) return 'badge-gray'
  return 'badge-success'
}

function pruneRefillGroup() {
  if (refillGroupId.value <= 0) return
  if (refillLocalGroupOptions.value.some((group) => group.id === refillGroupId.value)) return
  refillGroupId.value = 0
  refillResult.value = null
}

async function previewRoutingRefill() {
  await runRoutingRefill(true)
}

async function previewRoutingRefillForGroup(groupId: number) {
  refillGroupId.value = groupId
  await runRoutingRefill(true)
}

function handleWorkbenchAction(action: SchedulerAction) {
  if (isRoutingRefillAction(action) || isLocalAccountDisableAction(action)) {
    void router.push(actionRecommendationsRoute(action))
    return
  }
  activeTab.value = 'actions'
}

function workbenchActionLabel(action: SchedulerAction): string {
  if (isRoutingRefillAction(action) || isLocalAccountDisableAction(action)) {
    return '进入动作建议'
  }
  return action.recommended_operation || '查看'
}

function isRoutingRefillAction(action: SchedulerAction): boolean {
  return action.type === 'local_group.routing.refill' || action.type === 'local_group.routing.low_capacity'
}

function isLocalAccountDisableAction(action: SchedulerAction): boolean {
  return action.type === 'local_account.schedule.disable'
}

function actionRecommendationsRoute(action: SchedulerAction) {
  const objectId = schedulerActionTrailingId(action)
  if (isRoutingRefillAction(action)) {
    return {
      path: '/admin/actions',
      query: {
        type: 'routing_refill',
        ...(objectId > 0 ? { local_group_id: String(objectId) } : {})
      }
    }
  }
  return {
    path: '/admin/actions',
    query: {
      type: 'local_account_schedule_disable',
      ...(objectId > 0 ? { local_sub2api_account_id: String(objectId) } : {})
    }
  }
}

function schedulerActionTrailingId(action: SchedulerAction): number {
  const raw = String(action.id || '').split(':').pop() || ''
  const value = Number(raw)
  return Number.isFinite(value) && value > 0 ? value : 0
}

async function applyRoutingRefill() {
  await runRoutingRefill(false)
}

async function runRoutingRefill(dryRun: boolean) {
  if (refillGroupId.value <= 0) {
    appStore.showWarning('请先选择目标本地分组')
    return
  }
  refillBusy.value = true
  try {
    const policy = normalizedSettingsPayload()
    const maxRateMultiplier = normalizedRateMultiplier(policy.routing_refill_max_rate_multiplier)
    const result = await refillLocalGroup({
      local_group_id: refillGroupId.value,
      max_rate_multiplier: maxRateMultiplier > 0 ? maxRateMultiplier : undefined,
      limit: 1000,
      dry_run: dryRun,
      reason: 'manual_scheduler_center',
      cooldown_seconds: policy.routing_refill_cooldown_seconds,
      confirm_window_seconds: policy.routing_refill_confirm_window_seconds
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
    await loadPage()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '补池操作失败')
  } finally {
    refillBusy.value = false
  }
}

async function runBalanceSync() {
  await runTask(['fetch_balance'])
}

async function runPlan(plan: SchedulerPlan) {
  const taskTypes = planManualTaskTypes(plan)
  if (taskTypes.length === 0) {
    appStore.showError('该计划需要持久化调度执行器支持，当前不可手动运行')
    return
  }
  await runTask(taskTypes)
}

async function runTask(taskTypes: ExtensionTaskType[]) {
  running.value = true
  try {
    const run = await createSchedulerRun({
      mode: 'manual',
      task_types: taskTypes,
      window_minutes: 10
    })
    appStore.showSuccess(`已提交 ${taskLabel(run.task_type)}，${run.succeeded_steps}/${run.total_steps} step 完成`)
    await loadPage()
    activeTab.value = 'runs'
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '提交调度运行失败')
  } finally {
    running.value = false
  }
}

async function openRunDetail(run: SchedulerRunSummary) {
  await openRunDetailByID(run.id)
}

async function openRunDetailByID(runID: string) {
  if (!runID) return
  if (runDetailOpen.value && runDetail.value?.run.id === runID) return
  runDetailOpen.value = true
  runDetailLoading.value = true
  try {
    runDetail.value = await getSchedulerRunDetail(runID)
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '加载运行详情失败')
  } finally {
    runDetailLoading.value = false
  }
}

function closeRunDetail() {
  runDetailOpen.value = false
  runDetail.value = null
}

async function retryStep(step: SchedulerStepRecord) {
  retryingStepId.value = step.id
  try {
    await retrySchedulerStep(step.id)
    appStore.showSuccess('已提交 step 重试')
    await refreshOpenRunDetail()
    await loadPage()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '提交 step 重试失败')
  } finally {
    retryingStepId.value = null
  }
}

async function cancelStep(step: SchedulerStepRecord) {
  cancellingStepId.value = step.id
  try {
    await cancelSchedulerStep(step.id)
    appStore.showSuccess('step 已取消')
    await refreshOpenRunDetail()
    await loadPage()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '取消 step 失败')
  } finally {
    cancellingStepId.value = null
  }
}

async function cancelRun(run: SchedulerRunSummary) {
  updatingRunId.value = run.id
  try {
    await cancelSchedulerRun(run.id)
    appStore.showSuccess('运行已取消')
    await refreshOpenRunDetail()
    await loadPage()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '取消运行失败')
  } finally {
    updatingRunId.value = null
  }
}

async function retryRunFailedSteps(run: SchedulerRunSummary) {
  updatingRunId.value = run.id
  try {
    const detail = await retrySchedulerRunFailedSteps(run.id)
    if (runDetail.value?.run.id === run.id) {
      runDetail.value = detail
    }
    appStore.showSuccess('已提交失败 step 重试')
    await loadPage()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '提交失败 step 重试失败')
  } finally {
    updatingRunId.value = null
  }
}

async function refreshOpenRunDetail(silent = false) {
  if (!runDetail.value) return
  try {
    runDetail.value = await getSchedulerRunDetail(runDetail.value.run.id)
  } catch (error) {
    if (!silent) {
      appStore.showError((error as { message?: string }).message || '刷新运行详情失败')
    }
  }
}

async function openSupplierChecklist(supplier: SchedulerSupplierStatus) {
  supplierChecklistOpen.value = true
  supplierChecklistLoading.value = true
  try {
    supplierChecklist.value = await getSchedulerSupplierChecklist(supplier.supplier_id)
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '加载供应商 Checklist 失败')
  } finally {
    supplierChecklistLoading.value = false
  }
}

function closeSupplierChecklist() {
  supplierChecklistOpen.value = false
  supplierChecklist.value = null
}

async function runSupplierChecklistAction(itemKey: string) {
  const checklist = supplierChecklist.value
  if (!checklist) return
  const action = checklistActionForKey(itemKey)
  if (!action) {
    appStore.showError('该项需要在供应商管理中处理')
    return
  }
  const supplier = supplierStatuses.value.find((item) => item.supplier_id === checklist.supplier_id)
  if (!supplier) {
    appStore.showError('未找到供应商状态，请刷新后重试')
    return
  }
  await runSupplierAutomationAction(supplier, action)
  if (supplierChecklistOpen.value) {
    supplierChecklistLoading.value = true
    try {
      supplierChecklist.value = await getSchedulerSupplierChecklist(checklist.supplier_id)
    } catch (error) {
      appStore.showError((error as { message?: string }).message || '刷新供应商 Checklist 失败')
    } finally {
      supplierChecklistLoading.value = false
    }
  }
}

async function runSupplierAutomationAction(supplier: SchedulerSupplierStatus, action: SupplierAutomationAction) {
  const key = supplierActionKey(supplier.supplier_id, action)
  runningSupplierActionKey.value = key
  try {
    if (action === 'login_session') {
      const result = await loginSupplierSession(supplier.supplier_id)
      if (result.balance_sync_error) {
        appStore.showWarning('已完成供应商直登，余额读取失败，调度中心会继续重试')
      } else {
        appStore.showSuccess('已完成供应商直登')
      }
      await loadPage()
      return
    }
    const taskTypes = supplierActionTaskTypes(action)
    if (taskTypes.length === 0) {
      appStore.showError('该动作暂未接入自动执行')
      return
    }
    const run = await createSchedulerRun({
      mode: `supplier:${action}`,
      supplier_id: supplier.supplier_id,
      task_types: taskTypes,
      window_minutes: 10
    })
    appStore.showSuccess(`已提交 ${supplier.supplier_name} 的${supplierActionLabel(action)}，${run.total_steps} 个 step`)
    await loadPage()
    activeTab.value = 'runs'
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '提交供应商自动化动作失败')
  } finally {
    runningSupplierActionKey.value = null
  }
}

async function setPlanStatus(plan: SchedulerPlan, status: 'enabled' | 'paused' | 'disabled') {
  updatingPlanId.value = plan.id
  try {
    const updated = await updateSchedulerPlanStatus(plan.id, status)
    plans.value = plans.value.map((item) => (item.id === updated.id ? updated : item))
    appStore.showSuccess(`计划已${planStatusLabel(updated.status)}`)
    await loadPage()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '更新计划状态失败')
  } finally {
    updatingPlanId.value = null
  }
}

async function savePlanConfig(plan: SchedulerPlan, config: SchedulerPlanConfig) {
  updatingPlanId.value = plan.id
  try {
    const updated = await updateSchedulerPlanConfig(plan.id, config)
    plans.value = plans.value.map((item) => (item.id === updated.id ? updated : item))
    appStore.showSuccess('计划配置已保存')
    await loadPage()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '保存计划配置失败')
  } finally {
    updatingPlanId.value = null
  }
}

async function setActionStatus(action: SchedulerAction, status: 'resolved' | 'ignored' | 'investigating') {
  updatingActionId.value = action.id
  try {
    await updateSchedulerActionStatus(action.id, status)
    appStore.showSuccess(`动作已标记为${actionStatusLabel(status)}`)
    await loadPage()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '更新动作状态失败')
  } finally {
    updatingActionId.value = null
  }
}

async function saveSettings() {
  settingsSaving.value = true
  try {
    const updated = await updateSchedulerSettings(normalizedSettingsPayload())
    settings.value = updated
    syncSettingsForm(updated)
    appStore.showSuccess('调度设置已保存')
    await loadPage()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '保存调度设置失败')
  } finally {
    settingsSaving.value = false
  }
}

function syncSettingsForm(value: SchedulerSettings) {
  settingsForm.enabled = value.enabled
  settingsForm.default_supplier_concurrency = value.default_supplier_concurrency || 1
  settingsForm.channel_checks_enabled = value.channel_checks_enabled
  settingsForm.channel_check_daily_budget_tokens = value.channel_check_daily_budget_tokens || 0
  settingsForm.channel_check_probe_cooldown_seconds = value.channel_check_probe_cooldown_seconds || 600
  settingsForm.first_token_slow_threshold_ms = value.first_token_slow_threshold_ms || 0
  settingsForm.total_latency_slow_threshold_ms = value.total_latency_slow_threshold_ms || 0
  settingsForm.routing_refill_auto_execute_enabled = value.routing_refill_auto_execute_enabled || false
  settingsForm.routing_refill_low_capacity_threshold = value.routing_refill_low_capacity_threshold || 1
  settingsForm.routing_refill_cooldown_seconds = value.routing_refill_cooldown_seconds || 180
  settingsForm.routing_refill_confirm_window_seconds = value.routing_refill_confirm_window_seconds || 0
  settingsForm.routing_refill_max_rate_multiplier = value.routing_refill_max_rate_multiplier || 0
  settingsForm.default_enabled_task_types = [...(value.default_enabled_task_types || [])]
  settingsForm.high_cost_task_types = [...(value.high_cost_task_types || [])]
}

function normalizedSettingsPayload(): SchedulerSettings {
  return {
    enabled: settingsForm.enabled,
    default_supplier_concurrency: Math.max(1, Number(settingsForm.default_supplier_concurrency) || 1),
    channel_checks_enabled: settingsForm.channel_checks_enabled,
    channel_check_daily_budget_tokens: Math.max(0, Number(settingsForm.channel_check_daily_budget_tokens) || 0),
    channel_check_probe_cooldown_seconds: clampInteger(settingsForm.channel_check_probe_cooldown_seconds, 600, 0, 86400),
    first_token_slow_threshold_ms: Math.max(0, Number(settingsForm.first_token_slow_threshold_ms) || 0),
    total_latency_slow_threshold_ms: Math.max(0, Number(settingsForm.total_latency_slow_threshold_ms) || 0),
    routing_refill_auto_execute_enabled: settingsForm.routing_refill_auto_execute_enabled,
    routing_refill_low_capacity_threshold: clampInteger(settingsForm.routing_refill_low_capacity_threshold, 1, 1, 100),
    routing_refill_cooldown_seconds: clampInteger(settingsForm.routing_refill_cooldown_seconds, 180, 1, 86400),
    routing_refill_confirm_window_seconds: clampInteger(settingsForm.routing_refill_confirm_window_seconds, 0, 0, 86400),
    routing_refill_max_rate_multiplier: normalizedRateMultiplier(settingsForm.routing_refill_max_rate_multiplier),
    default_enabled_task_types: [...settingsForm.default_enabled_task_types],
    high_cost_task_types: [...settingsForm.high_cost_task_types]
  }
}

function clampInteger(value: number, fallback: number, min: number, max: number): number {
  const next = Math.round(Number(value))
  if (!Number.isFinite(next)) return fallback
  return Math.min(max, Math.max(min, next))
}

function normalizedRateMultiplier(value: number): number {
  const next = Number(value)
  if (!Number.isFinite(next) || next <= 0) return 0
  return next
}

function runPrimaryTime(run: SchedulerRunSummary): string {
  return formatDateTime(run.requested_at) || formatDateTime(run.started_at) || formatDateTime(run.finished_at) || '-'
}

function formatProxyDurationSeconds(value: number): string {
  const seconds = Math.max(0, Math.round(Number(value) || 0))
  if (seconds < 60) return `${seconds}s`
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes}m`
  const hours = Math.floor(minutes / 60)
  const remain = minutes % 60
  return remain ? `${hours}h ${remain}m` : `${hours}h`
}

async function initializePage() {
  await loadPage()
  await openRunDetailFromQuery()
}

async function openRunDetailFromQuery() {
  const runID = stringQuery(route.query.run_id)
  if (!runID) return
  activeTab.value = 'runs'
  await openRunDetailByID(runID)
}

function stringQuery(value: unknown): string {
  const raw = Array.isArray(value) ? value[0] : value
  return String(raw || '').trim()
}

function positiveQueryNumber(value: unknown): number {
  const raw = Array.isArray(value) ? value[0] : value
  const parsed = Number(raw)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : 0
}

watch(() => route.query.run_id, () => {
  void openRunDetailFromQuery()
})

onMounted(initializePage)
</script>
