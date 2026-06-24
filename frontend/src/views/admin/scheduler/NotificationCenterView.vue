<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">通知中心</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
            配置飞书通知、业务规则、防打扰和投递重试。
          </p>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <button type="button" class="btn btn-secondary" :disabled="loading" @click="loadPage">
            <Icon name="refresh" size="sm" />
            刷新
          </button>
          <button type="button" class="btn btn-primary" :disabled="testing || saving || !settingsForm.feishu.enabled" @click="sendTest">
            <Icon name="play" size="sm" />
            {{ testing ? '发送中...' : '发送测试' }}
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

      <section v-if="activeTab === 'dashboard'" class="grid gap-4 sm:grid-cols-2 xl:grid-cols-5">
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">飞书通道</p>
          <p class="mt-2 text-2xl font-semibold" :class="channelReady ? 'text-emerald-700 dark:text-emerald-400' : 'text-amber-600 dark:text-amber-400'">
            {{ channelReady ? '已就绪' : '待配置' }}
          </p>
          <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ settingsForm.feishu.webhook_host || '未检测到 Webhook' }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">启用规则</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">
            {{ status?.open_rules || enabledRuleCount }}/{{ status?.total_rules || settingsForm.rules.length }}
          </p>
          <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">业务事件 Checklist</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">成功</p>
          <p class="mt-2 text-2xl font-semibold text-emerald-700 dark:text-emerald-400">{{ status?.succeeded || 0 }}</p>
          <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">最近 200 条统计</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">失败</p>
          <p class="mt-2 text-2xl font-semibold text-rose-600 dark:text-rose-400">{{ status?.failed || 0 }}</p>
          <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">可在明细中重试</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">防打扰抑制</p>
          <p class="mt-2 text-2xl font-semibold text-slate-700 dark:text-slate-200">{{ status?.suppressed || 0 }}</p>
          <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ formatDateTime(status?.last_delivery_at) || '暂无投递' }}</p>
        </div>
      </section>

      <section v-if="activeTab === 'dashboard'" class="grid gap-6 xl:grid-cols-[minmax(0,1.7fr)_minmax(320px,0.8fr)]">
        <div class="space-y-6">
          <section class="card overflow-hidden">
            <div class="flex flex-col gap-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700 sm:flex-row sm:items-center sm:justify-between">
              <div>
                <h2 class="text-lg font-semibold text-gray-900 dark:text-white">今日工作台</h2>
                <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">优先处理通道配置、失败通知和业务规则缺口。</p>
              </div>
              <button type="button" class="btn btn-secondary btn-sm" @click="activeTab = 'deliveries'">查看投递记录</button>
            </div>
            <div class="divide-y divide-gray-100 dark:divide-dark-700">
              <div v-if="!channelReady" class="flex flex-col gap-3 px-5 py-4 md:flex-row md:items-center md:justify-between">
                <div>
                  <span class="badge badge-warning">待配置</span>
                  <p class="mt-2 font-medium text-gray-900 dark:text-white">飞书通道还不可用</p>
                  <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">保存 Webhook 并发送测试后，再启用业务通知。</p>
                </div>
                <button type="button" class="btn btn-primary shrink-0" @click="activeTab = 'settings'">去配置</button>
              </div>
              <div v-for="item in failedDeliveries" :key="item.id" class="flex flex-col gap-3 px-5 py-4 md:flex-row md:items-center md:justify-between">
                <div class="min-w-0">
                  <div class="flex flex-wrap items-center gap-2">
                    <span class="badge badge-danger">失败</span>
                    <span class="text-sm font-medium text-gray-900 dark:text-white">{{ eventLabel(item.event_type) }}</span>
                  </div>
                  <p class="mt-1 max-w-3xl break-words text-sm text-gray-500 dark:text-dark-400">{{ item.last_error || '无失败详情' }}</p>
                </div>
                <button type="button" class="btn btn-secondary shrink-0" :disabled="retryingId === item.id" @click="retryDelivery(item)">
                  {{ retryingId === item.id ? '重试中...' : '重试' }}
                </button>
              </div>
              <div v-if="channelReady && failedDeliveries.length === 0" class="px-5 py-10 text-center text-sm text-gray-500 dark:text-dark-400">
                暂无待处理通知。
              </div>
            </div>
          </section>

          <section class="card overflow-hidden">
            <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">最近投递</h2>
            </div>
            <div class="overflow-x-auto">
              <table class="w-full min-w-[720px] divide-y divide-gray-200 dark:divide-dark-700">
                <thead class="bg-gray-50 dark:bg-dark-800">
                  <tr>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">事件</th>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">状态</th>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">时间</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-100 bg-white dark:divide-dark-700 dark:bg-dark-900">
                  <tr v-if="recentDeliveries.length === 0">
                    <td colspan="3" class="px-4 py-8 text-center text-sm text-gray-500 dark:text-dark-400">暂无投递记录</td>
                  </tr>
                  <tr v-for="item in recentDeliveries" :key="item.id">
                    <td class="px-4 py-3">
                      <div class="text-sm font-medium text-gray-900 dark:text-gray-100">{{ eventLabel(item.event_type) }}</div>
                      <div class="mt-1 font-mono text-xs text-gray-500 dark:text-dark-400">{{ item.event_type }}</div>
                    </td>
                    <td class="px-4 py-3">
                      <span class="badge" :class="deliveryStatusClass(item.status)">{{ deliveryStatusLabel(item.status) }}</span>
                    </td>
                    <td class="px-4 py-3 text-sm text-gray-500 dark:text-dark-400">{{ formatDateTime(item.created_at) || '-' }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </section>
        </div>

        <aside class="space-y-6">
          <section class="card p-5">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">系统健康</h2>
            <dl class="mt-4 space-y-3 text-sm">
              <div class="flex items-center justify-between">
                <dt class="text-gray-500 dark:text-dark-400">通道</dt>
                <dd class="font-medium" :class="channelReady ? 'text-emerald-700 dark:text-emerald-400' : 'text-amber-600 dark:text-amber-400'">
                  {{ channelReady ? '已就绪' : '待配置' }}
                </dd>
              </div>
              <div class="flex items-center justify-between">
                <dt class="text-gray-500 dark:text-dark-400">成功</dt>
                <dd class="font-medium text-gray-900 dark:text-white">{{ status?.succeeded || 0 }}</dd>
              </div>
              <div class="flex items-center justify-between">
                <dt class="text-gray-500 dark:text-dark-400">发送中</dt>
                <dd class="font-medium text-gray-900 dark:text-white">{{ status?.sending || 0 }}</dd>
              </div>
              <div class="flex items-center justify-between">
                <dt class="text-gray-500 dark:text-dark-400">最近投递</dt>
                <dd class="font-medium text-gray-900 dark:text-white">{{ formatDateTime(status?.last_delivery_at) || '-' }}</dd>
              </div>
            </dl>
          </section>

          <section class="card overflow-hidden">
            <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">接入向导</h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">按顺序完成通道接入和业务验证。</p>
            </div>
            <div class="divide-y divide-gray-100 dark:divide-dark-700">
              <div v-for="step in setupSteps" :key="step.key" class="flex gap-3 px-5 py-4">
                <span class="mt-0.5 flex h-6 w-6 shrink-0 items-center justify-center rounded-full text-xs font-semibold" :class="step.done ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/20 dark:text-emerald-300' : 'bg-amber-100 text-amber-700 dark:bg-amber-500/20 dark:text-amber-300'">
                  {{ step.done ? '✓' : step.order }}
                </span>
                <div class="min-w-0">
                  <p class="text-sm font-medium text-gray-900 dark:text-white">{{ step.title }}</p>
                  <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ step.description }}</p>
                </div>
              </div>
            </div>
          </section>
        </aside>
      </section>

      <section v-if="activeTab === 'settings' || activeTab === 'rules'" :class="activeTab === 'settings' ? 'grid gap-6 xl:grid-cols-[minmax(0,1fr)_360px]' : 'space-y-6'">
        <div v-if="activeTab === 'settings'" class="space-y-6">
          <section class="card overflow-hidden">
            <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
              <div class="flex items-center justify-between gap-3">
                <div>
                  <h2 class="text-lg font-semibold text-gray-900 dark:text-white">飞书配置</h2>
                  <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">后台保存为主，环境变量只作为兼容来源。</p>
                </div>
                <button type="button" class="btn btn-primary btn-sm" :disabled="saving" @click="saveSettings">
                  {{ saving ? '保存中...' : '保存' }}
                </button>
              </div>
            </div>
            <div class="space-y-4 p-5">
              <label class="flex items-center justify-between gap-4 rounded-md border border-gray-200 px-3 py-3 dark:border-dark-700">
                <span>
                  <span class="block text-sm font-medium text-gray-900 dark:text-white">启用飞书通知</span>
                  <span class="mt-1 block text-xs text-gray-500 dark:text-dark-400">关闭后仍会记录 suppressed，便于审计。</span>
                </span>
                <input v-model="settingsForm.feishu.enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
              </label>

              <label class="block">
                <span class="text-sm font-medium text-gray-700 dark:text-dark-300">Webhook 地址</span>
                <input
                  v-model.trim="settingsForm.feishu.webhook_url"
                  type="text"
                  autocomplete="off"
                  placeholder="https://open.feishu.cn/open-apis/bot/v2/hook/..."
                  class="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-800 dark:text-white"
                />
                <span class="mt-1 block text-xs text-gray-500 dark:text-dark-400">
                  当前来源：{{ configSourceLabel(settingsForm.feishu.config_source) }}；已保存地址会自动脱敏。
                </span>
              </label>

              <label class="block">
                <span class="text-sm font-medium text-gray-700 dark:text-dark-300">签名密钥</span>
                <input
                  v-model.trim="settingsForm.feishu.webhook_secret"
                  type="password"
                  autocomplete="new-password"
                  :placeholder="settingsForm.feishu.secret_configured ? '已配置，留空表示不变' : '可选，飞书机器人开启签名校验时填写'"
                  class="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-800 dark:text-white"
                />
              </label>

              <label class="block">
                <span class="text-sm font-medium text-gray-700 dark:text-dark-300">测试内容</span>
                <div class="mt-1 flex flex-col gap-2 sm:flex-row">
                  <input
                    v-model.trim="testText"
                    type="text"
                    class="w-full rounded-md border border-gray-300 px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-800 dark:text-white"
                    placeholder="Sub2API Admin Plus 飞书通知测试"
                  />
                  <button type="button" class="btn btn-secondary shrink-0" :disabled="testing || saving || !settingsForm.feishu.enabled" @click="sendTest">
                    <Icon name="play" size="sm" />
                    测试
                  </button>
                </div>
              </label>
            </div>
          </section>

          <section class="card overflow-hidden">
            <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">接入向导</h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">按顺序完成通道接入和业务验证。</p>
            </div>
            <div class="divide-y divide-gray-100 dark:divide-dark-700">
              <div v-for="step in setupSteps" :key="step.key" class="flex gap-3 px-5 py-4">
                <span class="mt-0.5 flex h-6 w-6 shrink-0 items-center justify-center rounded-full text-xs font-semibold" :class="step.done ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/20 dark:text-emerald-300' : 'bg-amber-100 text-amber-700 dark:bg-amber-500/20 dark:text-amber-300'">
                  {{ step.done ? '✓' : step.order }}
                </span>
                <div class="min-w-0">
                  <p class="text-sm font-medium text-gray-900 dark:text-white">{{ step.title }}</p>
                  <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ step.description }}</p>
                </div>
              </div>
            </div>
          </section>
        </div>

        <aside v-if="activeTab === 'settings'" class="space-y-6">
          <section class="card p-5">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">配置状态</h2>
            <dl class="mt-4 space-y-3 text-sm">
              <div class="flex items-center justify-between">
                <dt class="text-gray-500 dark:text-dark-400">来源</dt>
                <dd class="font-medium text-gray-900 dark:text-white">{{ configSourceLabel(settingsForm.feishu.config_source) }}</dd>
              </div>
              <div class="flex items-center justify-between">
                <dt class="text-gray-500 dark:text-dark-400">Webhook</dt>
                <dd class="font-medium" :class="settingsForm.feishu.webhook_configured ? 'text-emerald-700 dark:text-emerald-400' : 'text-amber-600 dark:text-amber-400'">
                  {{ settingsForm.feishu.webhook_configured ? '已配置' : '未配置' }}
                </dd>
              </div>
              <div class="flex items-center justify-between">
                <dt class="text-gray-500 dark:text-dark-400">签名</dt>
                <dd class="font-medium text-gray-900 dark:text-white">{{ settingsForm.feishu.secret_configured ? '已配置' : '未配置' }}</dd>
              </div>
              <div class="flex items-center justify-between">
                <dt class="text-gray-500 dark:text-dark-400">Host</dt>
                <dd class="font-medium text-gray-900 dark:text-white">{{ settingsForm.feishu.webhook_host || '-' }}</dd>
              </div>
              <div class="flex items-center justify-between">
                <dt class="text-gray-500 dark:text-dark-400">最近测试</dt>
                <dd class="font-medium" :class="settingsForm.feishu.last_test_status === 'succeeded' ? 'text-emerald-700 dark:text-emerald-400' : 'text-gray-900 dark:text-white'">
                  {{ lastTestSummary }}
                </dd>
              </div>
              <div v-if="settingsForm.feishu.last_test_error" class="rounded-md bg-rose-50 px-3 py-2 dark:bg-rose-500/10">
                <dt class="text-xs text-rose-600 dark:text-rose-300">测试错误</dt>
                <dd class="mt-1 break-words text-xs text-rose-700 dark:text-rose-200">{{ settingsForm.feishu.last_test_error }}</dd>
              </div>
            </dl>
          </section>
        </aside>

        <section v-if="activeTab === 'rules'" class="card overflow-hidden">
          <div class="flex flex-col gap-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700 lg:flex-row lg:items-center lg:justify-between">
            <div>
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">业务规则 Checklist</h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">控制余额、健康、倍率、公告和对账异常的通知策略。</p>
            </div>
            <button type="button" class="btn btn-primary btn-sm" :disabled="saving" @click="saveSettings">
              {{ saving ? '保存中...' : '保存规则' }}
            </button>
          </div>
          <div class="divide-y divide-gray-100 dark:divide-dark-700">
            <div v-if="settingsForm.rules.length === 0" class="px-5 py-10 text-center text-sm text-gray-500 dark:text-dark-400">
              暂无通知规则。
            </div>
            <div v-for="rule in settingsForm.rules" :key="rule.event_type" class="grid gap-4 px-5 py-4 lg:grid-cols-[minmax(0,1fr)_150px] lg:items-center">
              <label class="flex min-w-0 gap-3">
                <input v-model="rule.enabled" type="checkbox" class="mt-1 h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
                <span class="min-w-0">
                  <span class="flex flex-wrap items-center gap-2">
                    <span class="font-medium text-gray-900 dark:text-white">{{ rule.label }}</span>
                    <span class="badge" :class="severityBadgeClass(rule.severity)">{{ severityLabel(rule.severity) }}</span>
                    <span class="badge badge-gray">{{ eventGroupLabel(rule.event_type) }}</span>
                  </span>
                  <span class="mt-1 block text-sm text-gray-500 dark:text-dark-400">{{ rule.description }}</span>
                  <span class="mt-1 block font-mono text-xs text-gray-400 dark:text-dark-500">{{ rule.event_type }} · {{ rule.dedupe_scope }}</span>
                </span>
              </label>
              <label class="block">
                <span class="text-xs text-gray-500 dark:text-dark-400">防打扰分钟</span>
                <input v-model.number="rule.quiet_window_minutes" type="number" min="0" class="mt-1 w-full rounded-md border border-gray-300 px-2 py-1.5 text-sm dark:border-dark-600 dark:bg-dark-800 dark:text-white" />
              </label>
            </div>
          </div>
        </section>
      </section>

      <section v-if="activeTab === 'deliveries'" class="card overflow-hidden">
        <div class="flex flex-col gap-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">投递记录</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">失败可重试，抑制记录会展示触发原因。</p>
          </div>
          <div class="flex flex-wrap gap-2">
            <select v-model="deliveryFilters.status" class="rounded-md border border-gray-300 px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-800 dark:text-white" @change="reloadDeliveries">
              <option value="">全部状态</option>
              <option value="succeeded">成功</option>
              <option value="failed">失败</option>
              <option value="sending">发送中</option>
              <option value="suppressed">已抑制</option>
            </select>
            <select v-model="deliveryFilters.event_type" class="rounded-md border border-gray-300 px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-800 dark:text-white" @change="reloadDeliveries">
              <option value="">全部事件</option>
              <option v-for="eventType in eventTypeOptions" :key="eventType" :value="eventType">{{ eventType }}</option>
            </select>
            <button type="button" class="btn btn-secondary" :disabled="deliveriesLoading" @click="loadDeliveries">
              <Icon name="refresh" size="sm" />
              刷新记录
            </button>
          </div>
        </div>
        <div class="overflow-x-auto">
          <table class="w-full min-w-[1080px] divide-y divide-gray-200 dark:divide-dark-700">
            <thead class="bg-gray-50 dark:bg-dark-800">
              <tr>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">事件</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">供应商</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">状态</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">次数</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">去重键</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">时间</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">操作</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 bg-white dark:divide-dark-700 dark:bg-dark-900">
              <tr v-if="deliveries.length === 0">
                <td colspan="7" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-dark-400">暂无通知投递记录</td>
              </tr>
              <tr v-for="item in deliveries" :key="item.id">
                <td class="px-4 py-4">
                  <div class="text-sm font-medium text-gray-900 dark:text-gray-100">{{ eventLabel(item.event_type) }}</div>
                  <div class="mt-1 font-mono text-xs text-gray-500 dark:text-dark-400">{{ item.event_type }} #{{ item.event_id }}</div>
                </td>
                <td class="px-4 py-4 text-sm text-gray-500 dark:text-dark-400">{{ item.supplier_id || '-' }}</td>
                <td class="px-4 py-4">
                  <span class="badge" :class="deliveryStatusClass(item.status)">{{ deliveryStatusLabel(item.status) }}</span>
                  <p v-if="item.last_error" class="mt-1 max-w-[260px] break-words text-xs text-rose-600 dark:text-rose-400">{{ item.last_error }}</p>
                </td>
                <td class="px-4 py-4 text-sm text-gray-500 dark:text-dark-400">{{ item.attempts }}</td>
                <td class="px-4 py-4">
                  <div class="max-w-[280px] truncate font-mono text-xs text-gray-500 dark:text-dark-400" :title="item.dedupe_key">{{ item.dedupe_key || '-' }}</div>
                </td>
                <td class="px-4 py-4 text-sm text-gray-500 dark:text-dark-400">
                  <div>创建 {{ formatDateTime(item.created_at) || '-' }}</div>
                  <div v-if="item.sent_at" class="mt-1 text-xs text-gray-400 dark:text-dark-500">发送 {{ formatDateTime(item.sent_at) }}</div>
                </td>
                <td class="px-4 py-4">
                  <button type="button" class="btn btn-secondary btn-sm" :disabled="item.status !== 'failed' || retryingId === item.id" @click="retryDelivery(item)">
                    {{ retryingId === item.id ? '重试中...' : '重试' }}
                  </button>
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
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import Pagination from '@/components/common/Pagination.vue'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { useAppStore } from '@/stores/app'
import {
  getNotificationCenterStatus,
  getNotificationSettings,
  listNotificationDeliveries,
  retryNotificationDelivery,
  testNotification,
  updateNotificationSettings,
  type NotificationCenterStatus,
  type NotificationDelivery,
  type NotificationRule,
  type NotificationSettings
} from '@/api/admin/adminPlus'

type TabValue = 'dashboard' | 'settings' | 'rules' | 'deliveries'

const appStore = useAppStore()
const loading = ref(false)
const saving = ref(false)
const testing = ref(false)
const deliveriesLoading = ref(false)
const retryingId = ref<number | null>(null)
const activeTab = ref<TabValue>('dashboard')
const status = ref<NotificationCenterStatus | null>(null)
const deliveries = ref<NotificationDelivery[]>([])
const testText = ref('')
const pagination = reactive({ page: 1, page_size: getPersistedPageSize(), total: 0, pages: 0 })
const deliveryFilters = reactive<{ status: NotificationDelivery['status'] | ''; event_type: string }>({
  status: '',
  event_type: ''
})
const settingsForm = reactive<NotificationSettings>({
  feishu: {
    enabled: true,
    webhook_url: '',
    webhook_secret: '',
    webhook_host: '',
    webhook_configured: false,
    secret_configured: false,
    config_source: 'database'
  },
  rules: []
})

const tabs: Array<{ value: TabValue; label: string }> = [
  { value: 'dashboard', label: '工作台' },
  { value: 'settings', label: '飞书配置' },
  { value: 'rules', label: '业务规则' },
  { value: 'deliveries', label: '投递记录' }
]

const channelReady = computed(() => settingsForm.feishu.enabled && settingsForm.feishu.webhook_configured)
const enabledRuleCount = computed(() => settingsForm.rules.filter((rule) => rule.enabled).length)
const lastTestSummary = computed(() => {
  if (!settingsForm.feishu.last_test_at) return '-'
  const statusText = settingsForm.feishu.last_test_status ? deliveryStatusLabel(settingsForm.feishu.last_test_status as NotificationDelivery['status']) : '未知'
  return `${statusText} · ${formatDateTime(settingsForm.feishu.last_test_at)}`
})
const failedDeliveries = computed(() => deliveries.value.filter((item) => item.status === 'failed').slice(0, 5))
const recentDeliveries = computed(() => deliveries.value.slice(0, 5))
const eventTypeOptions = computed(() => {
  const values = new Set<string>()
  settingsForm.rules.forEach((rule) => values.add(rule.event_type))
  deliveries.value.forEach((item) => values.add(item.event_type))
  return [...values].sort()
})
const setupSteps = computed(() => [
  {
    key: 'webhook',
    order: 1,
    done: settingsForm.feishu.webhook_configured,
    title: '配置飞书 Webhook',
    description: settingsForm.feishu.webhook_host || '填写飞书机器人 Webhook 地址。'
  },
  {
    key: 'secret',
    order: 2,
    done: settingsForm.feishu.secret_configured,
    title: '配置签名密钥',
    description: settingsForm.feishu.secret_configured ? '已启用签名校验。' : '如飞书机器人未开启签名校验，可跳过。'
  },
  {
    key: 'test',
    order: 3,
    done: status.value?.succeeded ? status.value.succeeded > 0 : false,
    title: '发送测试通知',
    description: '用真实飞书通道验证网络、Webhook 和签名。'
  },
  {
    key: 'rules',
    order: 4,
    done: enabledRuleCount.value > 0,
    title: '启用业务规则',
    description: `${enabledRuleCount.value}/${settingsForm.rules.length} 条规则已启用。`
  }
])

async function loadPage() {
  loading.value = true
  try {
    const [nextStatus, nextSettings] = await Promise.all([
      getNotificationCenterStatus(),
      getNotificationSettings()
    ])
    status.value = nextStatus
    syncSettingsForm(nextSettings)
    await loadDeliveries()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '加载通知中心失败')
  } finally {
    loading.value = false
  }
}

async function loadDeliveries() {
  deliveriesLoading.value = true
  try {
    const result = await listNotificationDeliveries({
      page: pagination.page,
      page_size: pagination.page_size,
      status: deliveryFilters.status,
      event_type: deliveryFilters.event_type
    })
    deliveries.value = result.items
    pagination.total = result.total || 0
    pagination.pages = result.pages || 0
    pagination.page = result.page || pagination.page
    pagination.page_size = result.page_size || pagination.page_size
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '加载投递记录失败')
  } finally {
    deliveriesLoading.value = false
  }
}

async function reloadDeliveries() {
  pagination.page = 1
  await loadDeliveries()
}

async function saveSettings() {
  await persistSettings(true)
}

async function persistSettings(showToast: boolean): Promise<boolean> {
  saving.value = true
  try {
    const updated = await updateNotificationSettings(normalizedSettingsPayload())
    syncSettingsForm(updated)
    status.value = await getNotificationCenterStatus()
    if (showToast) {
      appStore.showSuccess('通知设置已保存')
    }
    return true
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '保存通知设置失败')
    return false
  } finally {
    saving.value = false
  }
}

async function sendTest() {
  testing.value = true
  try {
    const saved = await persistSettings(false)
    if (!saved) return
    const delivery = await testNotification({ text: testText.value })
    if (delivery.status === 'succeeded') {
      appStore.showSuccess('测试通知发送成功')
    } else {
      appStore.showError(`测试通知未发送：${delivery.last_error || deliveryStatusLabel(delivery.status)}`)
    }
    const [nextStatus, nextSettings] = await Promise.all([
      getNotificationCenterStatus(),
      getNotificationSettings()
    ])
    status.value = nextStatus
    syncSettingsForm(nextSettings)
    await reloadDeliveries()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '发送测试通知失败')
    await refreshNotificationState()
  } finally {
    testing.value = false
  }
}

async function refreshNotificationState() {
  try {
    const [nextStatus, nextSettings] = await Promise.all([
      getNotificationCenterStatus(),
      getNotificationSettings()
    ])
    status.value = nextStatus
    syncSettingsForm(nextSettings)
    await reloadDeliveries()
  } catch {
    // 保留原始错误提示，刷新失败不覆盖用户真正需要处理的通知失败原因。
  }
}

async function retryDelivery(item: NotificationDelivery) {
  retryingId.value = item.id
  try {
    await retryNotificationDelivery(item.id)
    appStore.showSuccess('通知重试已提交')
    status.value = await getNotificationCenterStatus()
    await loadDeliveries()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '重试通知失败')
  } finally {
    retryingId.value = null
  }
}

function syncSettingsForm(value: NotificationSettings) {
  Object.assign(settingsForm.feishu, {
    enabled: value.feishu.enabled,
    webhook_url: value.feishu.webhook_url || '',
    webhook_secret: '',
    webhook_host: value.feishu.webhook_host || '',
    webhook_configured: value.feishu.webhook_configured,
    secret_configured: value.feishu.secret_configured,
    config_source: value.feishu.config_source || 'database',
    last_test_at: value.feishu.last_test_at || null,
    last_test_status: value.feishu.last_test_status || '',
    last_test_error: value.feishu.last_test_error || ''
  })
  settingsForm.rules = value.rules.map(cloneRule)
}

function normalizedSettingsPayload(): NotificationSettings {
  const webhookURL = settingsForm.feishu.webhook_url || ''
  return {
    feishu: {
      ...settingsForm.feishu,
      webhook_url: webhookURL.includes('***') ? '' : webhookURL.trim(),
      webhook_secret: (settingsForm.feishu.webhook_secret || '').trim(),
      webhook_configured: Boolean(settingsForm.feishu.webhook_configured),
      secret_configured: Boolean(settingsForm.feishu.secret_configured)
    },
    rules: settingsForm.rules.map((rule) => ({
      ...cloneRule(rule),
      quiet_window_minutes: Math.max(0, Number(rule.quiet_window_minutes) || 0)
    }))
  }
}

function cloneRule(rule: NotificationRule): NotificationRule {
  return {
    event_type: rule.event_type,
    label: rule.label,
    description: rule.description,
    enabled: rule.enabled,
    severity: rule.severity,
    quiet_window_minutes: rule.quiet_window_minutes,
    dedupe_scope: rule.dedupe_scope,
    notify_recovery: rule.notify_recovery,
    threshold: rule.threshold
  }
}

function handlePageChange(page: number) {
  pagination.page = page
  void loadDeliveries()
}

function handlePageSizeChange(pageSize: number) {
  pagination.page_size = pageSize
  pagination.page = 1
  void loadDeliveries()
}

function eventLabel(eventType: string): string {
  return settingsForm.rules.find((rule) => rule.event_type === eventType)?.label || eventType
}

function eventGroupLabel(eventType: string): string {
  const group = eventType.split('.')[0]
  return {
    balance: '余额',
    health: '健康',
    rate: '倍率',
    announcement: '公告',
    cost: '对账',
    system: '系统'
  }[group] || group
}

function severityLabel(value: string): string {
  return {
    critical: '严重',
    warning: '警告',
    info: '提示'
  }[value] || value
}

function severityBadgeClass(value: string): string {
  if (value === 'critical') return 'badge-danger'
  if (value === 'warning') return 'badge-warning'
  return 'badge-gray'
}

function deliveryStatusLabel(value: NotificationDelivery['status']): string {
  return {
    succeeded: '成功',
    failed: '失败',
    sending: '发送中',
    suppressed: '已抑制'
  }[value] || value
}

function deliveryStatusClass(value: NotificationDelivery['status']): string {
  if (value === 'succeeded') return 'badge-success'
  if (value === 'failed') return 'badge-danger'
  if (value === 'suppressed') return 'badge-gray'
  return 'badge-warning'
}

function configSourceLabel(value: string): string {
  if (value === 'environment') return '环境变量'
  if (value === 'database') return '后台配置'
  return value || '-'
}

function formatDateTime(value?: string | null): string {
  if (!value) return ''
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '' : date.toLocaleString()
}

onMounted(loadPage)
</script>
