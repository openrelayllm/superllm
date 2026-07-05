<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">{{ pageTitle }}</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ pageDescription }}</p>
        </div>
        <div v-if="showModelFilter || showProfitControls" class="flex flex-wrap items-end gap-2">
          <label v-if="showModelFilter" class="block min-w-[180px]">
            <span class="input-label">模型</span>
            <input v-model.trim="filters.model" type="text" class="input h-9" placeholder="全部模型" @keyup.enter="loadData" />
          </label>
          <label v-if="showProfitControls" class="block w-28">
            <span class="input-label">目标毛利</span>
            <input v-model.number="filters.target_margin_percent" type="number" min="1" step="1" class="input h-9" />
          </label>
          <label v-if="showProfitControls" class="block w-28">
            <span class="input-label">风险缓冲</span>
            <input v-model.number="filters.risk_buffer_percent" type="number" min="0" step="1" class="input h-9" />
          </label>
          <button type="button" class="btn btn-secondary h-9" :disabled="loading" @click="loadData">
            <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
            刷新
          </button>
        </div>
      </section>

      <section v-if="currentSection !== 'settings'" class="grid gap-4 sm:grid-cols-2 xl:grid-cols-5">
        <div v-if="currentSection === 'market-prices' || currentSection === 'profit'" class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">模型</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ overview?.model_count || 0 }}</p>
        </div>
        <div v-if="currentSection === 'market-prices'" class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">市场价快照</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ overview?.market_snapshot_count || 0 }}</p>
        </div>
        <div v-if="currentSection === 'supply-quality'" class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">缓存审计</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ overview?.cache_snapshot_count || 0 }}</p>
        </div>
        <div v-if="currentSection === 'supply-quality'" class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">质量快照</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ overview?.quality_snapshot_count || 0 }}</p>
        </div>
        <div v-if="currentSection === 'acceptance'" class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">验收报告</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ overview?.acceptance_report_count || 0 }}</p>
        </div>
        <div v-if="currentSection === 'acceptance'" class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">验收阻断</p>
          <p class="mt-2 text-2xl font-semibold" :class="(overview?.blocked_acceptance_count || 0) > 0 ? 'text-red-600 dark:text-red-300' : 'text-emerald-600 dark:text-emerald-300'">
            {{ overview?.blocked_acceptance_count || 0 }}
          </p>
        </div>
        <div v-if="currentSection === 'supply-quality' || currentSection === 'profit'" class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">缓存风险</p>
          <p class="mt-2 text-2xl font-semibold" :class="(overview?.risky_cache_model_count || 0) > 0 ? 'text-amber-600 dark:text-amber-300' : 'text-emerald-600 dark:text-emerald-300'">
            {{ overview?.risky_cache_model_count || 0 }}
          </p>
        </div>
        <div v-if="currentSection === 'profit'" class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">倒挂模型</p>
          <p class="mt-2 text-2xl font-semibold" :class="(overview?.unprofitable_model_count || 0) > 0 ? 'text-red-600 dark:text-red-300' : 'text-emerald-600 dark:text-emerald-300'">
            {{ overview?.unprofitable_model_count || 0 }}
          </p>
        </div>
        <div v-if="currentSection === 'supply-quality'" class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">质量风险</p>
          <p class="mt-2 text-2xl font-semibold" :class="(overview?.risky_quality_model_count || 0) > 0 ? 'text-amber-600 dark:text-amber-300' : 'text-emerald-600 dark:text-emerald-300'">
            {{ overview?.risky_quality_model_count || 0 }}
          </p>
        </div>
        <div v-if="currentSection === 'events'" class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">开放事件</p>
          <p class="mt-2 text-2xl font-semibold" :class="(overview?.critical_event_count || 0) > 0 ? 'text-red-600 dark:text-red-300' : 'text-gray-900 dark:text-white'">
            {{ overview?.open_event_count || 0 }}
          </p>
          <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">严重 {{ overview?.critical_event_count || 0 }}</p>
        </div>
      </section>

      <section v-if="currentSection === 'profit' || currentSection === 'events'" class="grid gap-6">
        <div v-if="currentSection === 'profit'" class="card overflow-hidden">
        <div class="flex flex-col gap-2 border-b border-gray-100 px-5 py-4 dark:border-dark-700 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">模型利润与风险</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">缓存调整成本会计入重复输入浪费；轮询号池命中率低时会优先标为风险。</p>
          </div>
          <div class="text-sm text-gray-500 dark:text-dark-400">{{ overview?.generated_at ? `生成时间 ${formatDateTime(overview.generated_at)}` : '暂无快照' }}</div>
        </div>
        <div v-if="error" class="mx-5 mt-4 rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-200">
          {{ error }}
        </div>
        <div class="overflow-x-auto">
          <table class="w-full min-w-[1360px] divide-y divide-gray-200 dark:divide-dark-700">
            <thead class="bg-gray-50 dark:bg-dark-800">
              <tr>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">模型</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">市场低/中/高</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">供应成本</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">缓存调整成本</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">建议售价</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">毛利</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">缓存</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">质量</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">验收</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">风险</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">建议</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-900">
              <tr v-if="loading && modelMargins.length === 0">
                <td colspan="11" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-dark-400">加载中...</td>
              </tr>
              <tr v-else-if="modelMargins.length === 0">
                <td colspan="11" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-dark-400">暂无模型利润数据</td>
              </tr>
              <tr v-for="row in modelMargins" :key="row.model" class="hover:bg-gray-50 dark:hover:bg-dark-800/60">
                <td class="px-4 py-4">
                  <div class="font-medium text-gray-900 dark:text-gray-100">{{ row.model }}</div>
                  <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">样本 {{ row.market_sample_count }}</div>
                </td>
                <td class="px-4 py-4 text-right text-sm text-gray-900 dark:text-gray-100">
                  {{ formatPriceRange(row) }}
                </td>
                <td class="px-4 py-4 text-right text-sm text-gray-900 dark:text-gray-100">{{ formatMicros(row.best_supplier_cost_micros, row.currency) }}</td>
                <td class="px-4 py-4 text-right text-sm font-medium text-gray-900 dark:text-gray-100">{{ formatMicros(row.cache_adjusted_cost_micros, row.currency) }}</td>
                <td class="px-4 py-4 text-right text-sm font-semibold text-primary-700 dark:text-primary-300">{{ formatMicros(row.suggested_price_micros, row.currency) }}</td>
                <td class="px-4 py-4 text-right text-sm font-medium" :class="marginClass(row.gross_margin_percent)">{{ formatPercent(row.gross_margin_percent) }}</td>
                <td class="px-4 py-4">
                  <span class="badge" :class="cacheStatusClass(row.cache_status)">{{ cacheStatusLabel(row.cache_status) }}</span>
                  <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">命中 {{ formatPercentFraction(row.cache_hit_ratio) }}</div>
                </td>
                <td class="px-4 py-4">
                  <span class="badge" :class="qualityDecisionClass(row.quality_decision)">{{ qualityDecisionLabel(row.quality_decision) }}</span>
                  <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">评分 {{ formatScore(row.quality_score) }}</div>
                </td>
                <td class="px-4 py-4">
                  <span class="badge" :class="qualityDecisionClass(row.acceptance_status)">{{ qualityDecisionLabel(row.acceptance_status) }}</span>
                </td>
                <td class="px-4 py-4">
                  <span class="badge" :class="riskClass(row.risk_level)">{{ riskLabel(row.risk_level) }}</span>
                </td>
                <td class="px-4 py-4 text-sm text-gray-600 dark:text-dark-300">{{ row.recommendation }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        </div>

        <div v-if="currentSection === 'events'" class="card overflow-hidden">
          <div class="flex items-center justify-between border-b border-gray-100 px-5 py-4 dark:border-dark-700">
            <div>
              <h2 class="text-base font-semibold text-gray-900 dark:text-white">价格与质量事件</h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">降价、异常低价和缓存风险会自动进入这里。</p>
            </div>
            <button type="button" class="btn btn-secondary btn-sm" :disabled="loading" @click="loadData">
              <Icon name="refresh" size="xs" />
              刷新
            </button>
          </div>
          <div class="divide-y divide-gray-100 dark:divide-dark-700">
            <div v-if="recentEvents.length === 0" class="px-5 py-10 text-center text-sm text-gray-500 dark:text-dark-400">暂无事件</div>
            <div v-for="event in recentEvents" :key="event.id" class="px-5 py-4">
              <div class="flex items-start justify-between gap-3">
                <div class="min-w-0">
                  <div class="flex flex-wrap items-center gap-2">
                    <span class="badge" :class="eventSeverityClass(event.severity)">{{ eventSeverityLabel(event.severity) }}</span>
                    <span class="badge" :class="eventStatusClass(event.status)">{{ eventStatusLabel(event.status) }}</span>
                    <span class="truncate text-sm font-semibold text-gray-900 dark:text-gray-100">{{ event.title }}</span>
                  </div>
                  <div class="mt-2 text-sm text-gray-600 dark:text-dark-300">
                    {{ event.model || '-' }} · {{ eventTypeLabel(event.event_type) }}
                  </div>
                  <p v-if="event.recommendation" class="mt-2 text-sm text-gray-500 dark:text-dark-400">{{ event.recommendation }}</p>
                  <div class="mt-2 text-xs text-gray-400 dark:text-dark-500">{{ formatDateTime(event.occurred_at) }}</div>
                </div>
                <div v-if="event.status === 'open'" class="flex shrink-0 gap-1">
                  <button type="button" class="btn btn-secondary btn-sm h-8 px-2" :disabled="Boolean(eventActionID)" @click="setEventStatus(event.id, 'acknowledged')">
                    处理
                  </button>
                  <button type="button" class="btn btn-secondary btn-sm h-8 px-2" :disabled="Boolean(eventActionID)" @click="setEventStatus(event.id, 'ignored')">
                    忽略
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section v-if="currentSection === 'market-prices' || currentSection === 'supply-quality'" class="grid gap-6 xl:grid-cols-2">
        <div v-if="currentSection === 'market-prices'" class="card overflow-hidden">
          <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">记录同行售价</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">录入手工调研、网址目录、供应商页面或 API 抓取的价格点。</p>
          </div>
          <form class="space-y-4 p-5" @submit.prevent="submitMarketPrice">
            <div class="grid gap-4 sm:grid-cols-2">
              <label class="block">
                <span class="input-label">来源</span>
                <select v-model="marketForm.source_type" class="input">
                  <option value="manual">手工调研</option>
                  <option value="site_catalog">网址目录</option>
                  <option value="site_discovery">渠道采集</option>
                  <option value="provider_page">供应商页面</option>
                  <option value="api">API</option>
                </select>
              </label>
              <label class="block">
                <span class="input-label">来源名称</span>
                <input v-model.trim="marketForm.source_name" type="text" class="input" placeholder="竞品或供应商名称" />
              </label>
              <label class="block">
                <span class="input-label">站点 ID</span>
                <input v-model.number="marketForm.site_id" type="number" min="0" step="1" class="input" placeholder="可选" />
              </label>
              <label class="block">
                <span class="input-label">供应商 ID</span>
                <input v-model.number="marketForm.supplier_id" type="number" min="0" step="1" class="input" placeholder="可选" />
              </label>
              <label class="block sm:col-span-2">
                <span class="input-label">来源 URL</span>
                <input v-model.trim="marketForm.source_url" type="url" class="input" placeholder="https://example.com/pricing" />
              </label>
              <label class="block">
                <span class="input-label">模型</span>
                <input v-model.trim="marketForm.model" type="text" class="input" required placeholder="gpt-4o-mini" />
              </label>
              <label class="block">
                <span class="input-label">币种</span>
                <input v-model.trim="marketForm.currency" type="text" class="input uppercase" maxlength="3" />
              </label>
              <label class="block">
                <span class="input-label">价格项</span>
                <select v-model="marketForm.price_item" class="input">
                  <option value="blended">综合</option>
                  <option value="input">输入</option>
                  <option value="output">输出</option>
                  <option value="cache_read">缓存读</option>
                  <option value="cache_write">缓存写</option>
                </select>
              </label>
              <label class="block">
                <span class="input-label">每 1M Token 价格</span>
                <input v-model.number="marketPricePerMillion" type="number" min="0" step="0.000001" class="input" required />
              </label>
              <label class="block">
                <span class="input-label">充值倍率</span>
                <input v-model.number="marketForm.rate_multiplier" type="number" min="0" step="0.0001" class="input" />
              </label>
              <label class="block">
                <span class="input-label">置信度</span>
                <input v-model.number="marketForm.confidence" type="number" min="0.01" max="1" step="0.01" class="input" />
              </label>
            </div>
            <div class="flex justify-end">
              <button type="submit" class="btn btn-primary" :disabled="marketSubmitting">
                <Icon name="plus" size="sm" />
                {{ marketSubmitting ? '保存中' : '保存售价' }}
              </button>
            </div>
          </form>
        </div>

        <div v-if="currentSection === 'supply-quality'" class="card overflow-hidden">
          <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">补录缓存效率</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">自有号池和第三方供应商会自动从近 7 天 usage 日志派生；这里用于竞品、自定义或历史证据补录。</p>
          </div>
          <form class="space-y-4 p-5" @submit.prevent="submitCacheEfficiency">
            <div class="grid gap-4 sm:grid-cols-2">
              <label class="block">
                <span class="input-label">供应类型</span>
                <select v-model="cacheForm.supply_type" class="input">
                  <option value="supplier">第三方供应商</option>
                  <option value="own_pool">自有号池</option>
                  <option value="competitor">竞品</option>
                  <option value="custom">自定义</option>
                </select>
              </label>
              <label class="block">
                <span class="input-label">模型</span>
                <input v-model.trim="cacheForm.model" type="text" class="input" required placeholder="gpt-4o-mini" />
              </label>
              <label class="block">
                <span class="input-label">供应商 ID</span>
                <input v-model.number="cacheForm.supplier_id" type="number" min="0" step="1" class="input" placeholder="可选" />
              </label>
              <label class="block">
                <span class="input-label">本地账号 ID</span>
                <input v-model.number="cacheForm.local_sub2api_account_id" type="number" min="0" step="1" class="input" placeholder="可选" />
              </label>
              <label class="block">
                <span class="input-label">路由策略</span>
                <select v-model="cacheForm.routing_strategy" class="input">
                  <option value="round_robin">轮询号池</option>
                  <option value="weighted_round_robin">加权轮询</option>
                  <option value="sticky">粘性路由</option>
                  <option value="fixed_account">固定账号</option>
                  <option value="least_loaded">最小负载</option>
                  <option value="custom">自定义</option>
                  <option value="unknown">未知</option>
                </select>
              </label>
              <label class="block">
                <span class="input-label">粘性范围</span>
                <select v-model="cacheForm.sticky_scope" class="input">
                  <option value="none">无</option>
                  <option value="user">用户</option>
                  <option value="api_key">API Key</option>
                  <option value="project">项目</option>
                  <option value="session">会话</option>
                  <option value="organization">组织</option>
                  <option value="custom">自定义</option>
                </select>
              </label>
              <label class="block">
                <span class="input-label">样本请求</span>
                <input v-model.number="cacheForm.sample_requests" type="number" min="0" step="1" class="input" />
              </label>
              <label class="block">
                <span class="input-label">缓存命中率</span>
                <input v-model.number="cacheHitRatioPercent" type="number" min="0" max="100" step="0.01" class="input" />
              </label>
              <label class="block">
                <span class="input-label">缓存读 Token</span>
                <input v-model.number="cacheForm.cache_read_tokens" type="number" min="0" step="1" class="input" />
              </label>
              <label class="block">
                <span class="input-label">缓存写 Token</span>
                <input v-model.number="cacheForm.cache_write_tokens" type="number" min="0" step="1" class="input" />
              </label>
              <label class="block">
                <span class="input-label">输入 Token</span>
                <input v-model.number="cacheForm.input_tokens" type="number" min="0" step="1" class="input" />
              </label>
              <label class="block">
                <span class="input-label">输出 Token</span>
                <input v-model.number="cacheForm.output_tokens" type="number" min="0" step="1" class="input" />
              </label>
              <label class="block">
                <span class="input-label">重复输入 Token</span>
                <input v-model.number="cacheForm.duplicate_input_tokens" type="number" min="0" step="1" class="input" />
              </label>
              <label class="block">
                <span class="input-label">浪费成本</span>
                <input v-model.number="estimatedWasteDollars" type="number" min="0" step="0.01" class="input" />
              </label>
              <label class="block">
                <span class="input-label">状态</span>
                <select v-model="cacheForm.status" class="input">
                  <option value="">自动判断</option>
                  <option value="healthy">健康</option>
                  <option value="watching">观察</option>
                  <option value="risky">风险</option>
                  <option value="bad">严重</option>
                  <option value="unknown">未知</option>
                </select>
              </label>
              <label class="block">
                <span class="input-label">平均首 Token</span>
                <input v-model.number="cacheForm.avg_ttft_ms" type="number" min="0" step="1" class="input" placeholder="ms" />
              </label>
              <label class="block">
                <span class="input-label">平均总延迟</span>
                <input v-model.number="cacheForm.avg_total_latency_ms" type="number" min="0" step="1" class="input" placeholder="ms" />
              </label>
              <label class="block sm:col-span-2">
                <span class="input-label">备注</span>
                <input v-model.trim="cacheForm.notes" type="text" class="input" placeholder="例如：轮询导致 prompt cache 失效" />
              </label>
            </div>
            <div class="flex justify-end">
              <button type="submit" class="btn btn-primary" :disabled="cacheSubmitting">
                <Icon name="plus" size="sm" />
                {{ cacheSubmitting ? '保存中' : '保存缓存审计' }}
              </button>
            </div>
          </form>
        </div>

        <div v-if="currentSection === 'supply-quality'" class="card overflow-hidden">
          <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">补录供应质量</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">成功请求、错误日志、模型纯度、余额和并发会自动归一为质量证据；这里用于外部或历史证据补录。</p>
          </div>
          <form class="space-y-4 p-5" @submit.prevent="submitSupplyQuality">
            <div class="grid gap-4 sm:grid-cols-2">
              <label class="block">
                <span class="input-label">供应类型</span>
                <select v-model="qualityForm.supply_type" class="input">
                  <option value="supplier">第三方供应商</option>
                  <option value="own_pool">自有号池</option>
                  <option value="competitor">竞品</option>
                  <option value="custom">自定义</option>
                </select>
              </label>
              <label class="block">
                <span class="input-label">模型</span>
                <input v-model.trim="qualityForm.model" type="text" class="input" placeholder="可选" />
              </label>
              <label class="block">
                <span class="input-label">供应商 ID</span>
                <input v-model.number="qualityForm.supplier_id" type="number" min="0" step="1" class="input" placeholder="可选" />
              </label>
              <label class="block">
                <span class="input-label">本地账号 ID</span>
                <input v-model.number="qualityForm.local_sub2api_account_id" type="number" min="0" step="1" class="input" placeholder="可选" />
              </label>
              <label class="block">
                <span class="input-label">可用率</span>
                <input v-model.number="qualityForm.availability_percent" type="number" min="0" max="100" step="0.01" class="input" />
              </label>
              <label class="block">
                <span class="input-label">错误率</span>
                <input v-model.number="qualityForm.error_percent" type="number" min="0" max="100" step="0.01" class="input" />
              </label>
              <label class="block">
                <span class="input-label">缓存命中率</span>
                <input v-model.number="qualityForm.cache_hit_percent" type="number" min="0" max="100" step="0.01" class="input" />
              </label>
              <label class="block">
                <span class="input-label">模型纯度</span>
                <input v-model.number="qualityForm.purity_score" type="number" min="0" max="100" step="1" class="input" />
              </label>
              <label class="block">
                <span class="input-label">账单可信</span>
                <input v-model.number="qualityForm.usage_trust_score" type="number" min="0" max="100" step="1" class="input" />
              </label>
              <label class="block">
                <span class="input-label">余额风险</span>
                <input v-model.number="qualityForm.balance_risk_score" type="number" min="0" max="100" step="1" class="input" />
              </label>
              <label class="block">
                <span class="input-label">并发能力</span>
                <input v-model.number="qualityForm.concurrency_score" type="number" min="0" max="100" step="1" class="input" />
              </label>
              <label class="block">
                <span class="input-label">质量分</span>
                <input v-model.number="qualityForm.quality_score" type="number" min="0" max="100" step="1" class="input" placeholder="0 自动计算" />
              </label>
              <label class="block">
                <span class="input-label">平均首 Token</span>
                <input v-model.number="qualityForm.avg_ttft_ms" type="number" min="0" step="1" class="input" placeholder="ms" />
              </label>
              <label class="block">
                <span class="input-label">平均总延迟</span>
                <input v-model.number="qualityForm.avg_total_latency_ms" type="number" min="0" step="1" class="input" placeholder="ms" />
              </label>
              <label class="block">
                <span class="input-label">决策</span>
                <select v-model="qualityForm.decision" class="input">
                  <option value="">自动判断</option>
                  <option value="production">生产</option>
                  <option value="watching">观察</option>
                  <option value="low_priority">低优先级</option>
                  <option value="paused">暂停</option>
                  <option value="blocked">阻断</option>
                </select>
              </label>
              <label class="block sm:col-span-2">
                <span class="input-label">备注</span>
                <input v-model.trim="qualityForm.notes" type="text" class="input" placeholder="例如：usage 对账失败或纯度异常" />
              </label>
            </div>
            <div class="flex justify-end">
              <button type="submit" class="btn btn-primary" :disabled="qualitySubmitting">
                <Icon name="plus" size="sm" />
                {{ qualitySubmitting ? '保存中' : '保存质量快照' }}
              </button>
            </div>
          </form>
        </div>
      </section>

      <section v-if="currentSection === 'market-prices'" class="card overflow-hidden">
        <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">解析公开价格文本</h2>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">用于粘贴公开价格页、扩展采集或渠道索引提取的价格文本，批量写入市场价快照。</p>
        </div>
        <form class="space-y-4 p-5" @submit.prevent="submitMarketPriceParse">
          <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
            <label class="block">
              <span class="input-label">来源</span>
              <select v-model="marketParseForm.source_type" class="input">
                <option value="provider_page">供应商页面</option>
                <option value="site_catalog">网址目录</option>
                <option value="site_discovery">渠道采集</option>
                <option value="api">API</option>
                <option value="manual">手工调研</option>
              </select>
            </label>
            <label class="block">
              <span class="input-label">来源名称</span>
              <input v-model.trim="marketParseForm.source_name" type="text" class="input" placeholder="竞品或页面名称" />
            </label>
            <label class="block">
              <span class="input-label">默认币种</span>
              <input v-model.trim="marketParseForm.default_currency" type="text" class="input uppercase" maxlength="3" />
            </label>
            <label class="block">
              <span class="input-label">置信度</span>
              <input v-model.number="marketParseForm.confidence" type="number" min="0.01" max="1" step="0.01" class="input" />
            </label>
            <label class="block">
              <span class="input-label">站点 ID</span>
              <input v-model.number="marketParseForm.site_id" type="number" min="0" step="1" class="input" placeholder="可选" />
            </label>
            <label class="block">
              <span class="input-label">供应商 ID</span>
              <input v-model.number="marketParseForm.supplier_id" type="number" min="0" step="1" class="input" placeholder="可选" />
            </label>
            <label class="block sm:col-span-2">
              <span class="input-label">来源 URL</span>
              <input v-model.trim="marketParseForm.source_url" type="url" class="input" placeholder="https://example.com/pricing" />
            </label>
            <label class="block sm:col-span-2 xl:col-span-4">
              <span class="input-label">价格文本</span>
              <textarea v-model.trim="marketParseForm.text" class="input min-h-[120px] resize-y" placeholder="示例：gpt-4o-mini input $0.15 / 1M tokens&#10;gpt-4o-mini output $0.60 / 1M tokens"></textarea>
            </label>
          </div>
          <div class="flex flex-wrap justify-end gap-2">
            <button type="button" class="btn btn-secondary" :disabled="marketURLImporting || marketParseSubmitting" @click="importCurrentMarketPriceURL">
              <Icon name="download" size="sm" />
              {{ marketURLImporting ? '抓取中' : '抓取 URL 并解析' }}
            </button>
            <button type="submit" class="btn btn-primary" :disabled="marketParseSubmitting">
              <Icon name="plus" size="sm" />
              {{ marketParseSubmitting ? '解析中' : '解析并保存' }}
            </button>
          </div>
        </form>
        <div class="border-t border-gray-100 p-5 dark:border-dark-700">
          <div class="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
            <div class="grid flex-1 gap-3 sm:grid-cols-[minmax(0,1fr)_auto_auto] sm:items-end">
              <label class="block">
                <span class="input-label">候选站点</span>
                <input v-model.trim="priceSourceQuery" type="text" class="input" placeholder="按名称、域名或 slug 过滤" @keyup.enter="discoverPriceSources" />
              </label>
              <label class="flex h-10 items-center gap-2 text-sm text-gray-600 dark:text-dark-300">
                <input v-model="includeLowConfidenceSources" type="checkbox" class="rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
                低置信候选
              </label>
              <button type="button" class="btn btn-secondary h-10" :disabled="priceSourceLoading" @click="discoverPriceSources">
                <Icon name="search" size="sm" />
                {{ priceSourceLoading ? '发现中' : '发现价格页' }}
              </button>
            </div>
          </div>
          <div class="mt-4 overflow-hidden rounded-md border border-gray-100 dark:border-dark-700">
            <div v-if="priceSourceCandidates.length === 0" class="px-4 py-6 text-center text-sm text-gray-500 dark:text-dark-400">暂无价格页候选</div>
            <div v-for="candidate in priceSourceCandidates" :key="`${candidate.site_id || 0}:${candidate.source_url}`" class="flex flex-col gap-3 border-t border-gray-100 px-4 py-3 first:border-t-0 dark:border-dark-700 sm:flex-row sm:items-center sm:justify-between">
              <div class="min-w-0">
                <div class="flex flex-wrap items-center gap-2">
                  <span class="text-sm font-medium text-gray-900 dark:text-gray-100">{{ candidate.source_name || '-' }}</span>
                  <span class="badge badge-primary">{{ confidenceLabel(candidate.confidence) }}</span>
                  <span class="badge badge-gray">{{ candidateReasonLabel(candidate.reason) }}</span>
                </div>
                <div class="mt-1 truncate text-sm text-gray-500 dark:text-dark-400">{{ candidate.source_url }}</div>
              </div>
              <div class="flex shrink-0 gap-2">
                <button type="button" class="btn btn-secondary btn-sm" @click="applyPriceSourceCandidate(candidate)">
                  填入
                </button>
                <button type="button" class="btn btn-primary btn-sm" :disabled="priceSourceImportingURL === candidate.source_url" @click="importPriceSourceCandidate(candidate)">
                  {{ priceSourceImportingURL === candidate.source_url ? '抓取中' : '抓取解析' }}
                </button>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section v-if="currentSection === 'acceptance'" class="card overflow-hidden">
        <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">接入验收</h2>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">记录第三方供应商或自有号池进入生产候选前的关键检查结果。</p>
        </div>
        <form class="space-y-5 p-5" @submit.prevent="submitAcceptanceReport">
          <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
            <label class="block">
              <span class="input-label">供应类型</span>
              <select v-model="acceptanceForm.supply_type" class="input">
                <option value="supplier">第三方供应商</option>
                <option value="own_pool">自有号池</option>
                <option value="competitor">竞品</option>
                <option value="custom">自定义</option>
              </select>
            </label>
            <label class="block">
              <span class="input-label">模型</span>
              <input v-model.trim="acceptanceForm.model" type="text" class="input" placeholder="可选" />
            </label>
            <label class="block">
              <span class="input-label">供应商 ID</span>
              <input v-model.number="acceptanceForm.supplier_id" type="number" min="0" step="1" class="input" placeholder="可选" />
            </label>
            <label class="block">
              <span class="input-label">本地账号 ID</span>
              <input v-model.number="acceptanceForm.local_sub2api_account_id" type="number" min="0" step="1" class="input" placeholder="可选" />
            </label>
            <label class="block sm:col-span-2">
              <span class="input-label">调度 Run ID</span>
              <input v-model.trim="acceptanceForm.evidence_scheduler_run_id" type="text" class="input" placeholder="kanban_acceptance-..." />
            </label>
            <label class="block">
              <span class="input-label">验收结论</span>
              <select v-model="acceptanceForm.status" class="input">
                <option value="">自动判断</option>
                <option value="production">生产</option>
                <option value="watching">观察</option>
                <option value="low_priority">低优先级</option>
                <option value="paused">暂停</option>
                <option value="blocked">阻断</option>
              </select>
            </label>
            <label v-for="step in acceptanceSteps" :key="step.key" class="block">
              <span class="input-label">{{ step.label }}</span>
              <select v-model="acceptanceForm[step.key]" class="input">
                <option value="unknown">未知</option>
                <option value="pass">通过</option>
                <option value="warn">警告</option>
                <option value="fail">失败</option>
              </select>
            </label>
            <label class="block sm:col-span-2">
              <span class="input-label">失败原因</span>
              <input v-model.trim="acceptanceForm.failure_reason" type="text" class="input" placeholder="例如：缓存审计失败或 usage 计量不可信" />
            </label>
            <label class="block sm:col-span-2">
              <span class="input-label">处理建议</span>
              <input v-model.trim="acceptanceForm.recommendation" type="text" class="input" placeholder="留空则自动生成" />
            </label>
          </div>
          <div class="flex flex-wrap justify-end gap-2">
            <label class="mr-auto flex min-h-10 items-center gap-2 text-sm text-gray-600 dark:text-dark-300">
              <input v-model="acceptanceForm.enqueue_evidence_tasks" type="checkbox" class="rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
              排队真实探针
            </label>
            <button type="button" class="btn btn-secondary" :disabled="acceptanceGenerating" @click="generateAcceptanceFromEvidence">
              <Icon name="refresh" size="sm" :class="{ 'animate-spin': acceptanceGenerating }" />
              {{ acceptanceGenerating ? '生成中' : '按质量/缓存生成' }}
            </button>
            <button type="button" class="btn btn-secondary" :disabled="acceptanceRefreshing" @click="refreshAcceptanceFromEvidenceRun">
              <Icon name="refresh" size="sm" :class="{ 'animate-spin': acceptanceRefreshing }" />
              {{ acceptanceRefreshing ? '回填中' : '从调度回填' }}
            </button>
            <button type="submit" class="btn btn-primary" :disabled="acceptanceSubmitting">
              <Icon name="plus" size="sm" />
              {{ acceptanceSubmitting ? '保存中' : '保存验收报告' }}
            </button>
          </div>
        </form>
      </section>

      <section v-if="currentSection === 'settings'" class="card overflow-hidden">
        <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">看板参数</h2>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">这些参数保存在当前浏览器，用于利润测算和风险判断。</p>
        </div>
        <form class="space-y-4 p-5" @submit.prevent="saveKanbanSettingsAndReload">
          <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
            <label class="block">
              <span class="input-label">目标毛利</span>
              <input v-model.number="filters.target_margin_percent" type="number" min="1" step="1" class="input" />
            </label>
            <label class="block">
              <span class="input-label">风险缓冲</span>
              <input v-model.number="filters.risk_buffer_percent" type="number" min="0" step="1" class="input" />
            </label>
          </div>
          <div class="flex justify-end">
            <button type="submit" class="btn btn-primary" :disabled="loading">
              <Icon name="check" size="sm" />
              保存设置
            </button>
          </div>
        </form>
      </section>

      <section v-if="currentSection === 'market-prices' || currentSection === 'supply-quality' || currentSection === 'acceptance'" class="grid gap-6 xl:grid-cols-2">
        <RecentSnapshotTable v-if="currentSection === 'market-prices'" title="最近市场价" :items="overview?.recent_market_snapshots || []" type="market" />
        <RecentSnapshotTable v-if="currentSection === 'supply-quality'" title="最近缓存审计" :items="overview?.recent_cache_snapshots || []" type="cache" />
        <RecentSnapshotTable v-if="currentSection === 'supply-quality'" title="最近供应质量" :items="overview?.recent_quality_snapshots || []" type="quality" />
        <RecentSnapshotTable v-if="currentSection === 'acceptance'" title="最近接入验收" :items="overview?.recent_acceptance_reports || []" type="acceptance" />
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onMounted, reactive, ref, watch } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import {
  discoverMarketPriceSources,
  generateAcceptanceReport,
  getKanbanOverview,
  importMarketPricesFromURL,
  parseMarketPrices,
  recordAcceptanceReport,
  recordCacheEfficiency,
  recordMarketPrice,
  recordSupplyQuality,
  refreshAcceptanceReportFromEvidenceRun,
  updateKanbanEventStatus
} from '@/api/admin/adminPlus'
import type {
  AcceptanceReport,
  AcceptanceStepStatus,
  CacheEfficiencySnapshot,
  CacheEfficiencyStatus,
  CacheEfficiencySupplyType,
  CacheRoutingStrategy,
  CacheStickyScope,
  CreateCacheEfficiencyPayload,
  CreateAcceptanceReportPayload,
  GenerateAcceptanceReportPayload,
  CreateMarketPricePayload,
  CreateSupplyQualityPayload,
  KanbanEventStatus,
  KanbanModelMarginRow,
  KanbanOverview,
  KanbanRiskLevel,
  ImportMarketPricesFromURLPayload,
  MarketPriceSourceCandidate,
  MarketPriceSnapshot,
  MarketPriceSourceType,
  ParseMarketPricesPayload,
  RefreshAcceptanceReportFromEvidenceRunPayload,
  SupplyQualityDecision,
  SupplyQualitySnapshot
} from '@/api/admin/adminPlus'
import { useAppStore } from '@/stores/app'
import { formatDateTime } from './SupplierAccountsUtils'

type KanbanSection = 'market-prices' | 'supply-quality' | 'profit' | 'acceptance' | 'events' | 'settings'

const props = defineProps<{
  section?: KanbanSection
}>()

type MarketForm = {
  source_type: MarketPriceSourceType
  source_name: string
  source_url: string
  site_id: number | null
  supplier_id: number | null
  model: string
  billing_mode: string
  price_item: string
  unit: string
  currency: string
  rate_multiplier: number | null
  confidence: number
}

type MarketParseForm = {
  source_type: MarketPriceSourceType
  source_name: string
  source_url: string
  site_id: number | null
  supplier_id: number | null
  default_currency: string
  confidence: number
  text: string
}

type CacheForm = {
  supply_type: CacheEfficiencySupplyType
  supplier_id: number | null
  local_sub2api_account_id: number | null
  model: string
  routing_strategy: CacheRoutingStrategy
  sticky_scope: CacheStickyScope
  sample_requests: number
  cache_read_tokens: number
  cache_write_tokens: number
  input_tokens: number
  output_tokens: number
  duplicate_input_tokens: number
  avg_ttft_ms: number | null
  avg_total_latency_ms: number | null
  status: CacheEfficiencyStatus | ''
  notes: string
}

type QualityForm = {
  supply_type: CacheEfficiencySupplyType
  supplier_id: number | null
  local_sub2api_account_id: number | null
  model: string
  availability_percent: number
  error_percent: number
  avg_ttft_ms: number | null
  avg_total_latency_ms: number | null
  cache_hit_percent: number
  purity_score: number
  usage_trust_score: number
  balance_risk_score: number
  concurrency_score: number
  quality_score: number
  decision: SupplyQualityDecision | ''
  notes: string
}

type AcceptanceStepKey =
  | 'connectivity_status'
  | 'model_list_status'
  | 'purity_status'
  | 'trial_call_status'
  | 'usage_metering_status'
  | 'cache_audit_status'
  | 'balance_status'
  | 'concurrency_status'

type AcceptanceForm = {
  supply_type: CacheEfficiencySupplyType
  supplier_id: number | null
  local_sub2api_account_id: number | null
  model: string
  status: SupplyQualityDecision | ''
  connectivity_status: AcceptanceStepStatus
  model_list_status: AcceptanceStepStatus
  purity_status: AcceptanceStepStatus
  trial_call_status: AcceptanceStepStatus
  usage_metering_status: AcceptanceStepStatus
  cache_audit_status: AcceptanceStepStatus
  balance_status: AcceptanceStepStatus
  concurrency_status: AcceptanceStepStatus
  failure_reason: string
  recommendation: string
  enqueue_evidence_tasks: boolean
  evidence_scheduler_run_id: string
}

const acceptanceSteps: Array<{ key: AcceptanceStepKey; label: string }> = [
  { key: 'connectivity_status', label: '连通性' },
  { key: 'model_list_status', label: '模型列表' },
  { key: 'purity_status', label: '模型纯度' },
  { key: 'trial_call_status', label: '轻量调用' },
  { key: 'usage_metering_status', label: 'Usage 计量' },
  { key: 'cache_audit_status', label: '缓存审计' },
  { key: 'balance_status', label: '余额账单' },
  { key: 'concurrency_status', label: '并发限速' }
]

const appStore = useAppStore()
const loading = ref(false)
const marketSubmitting = ref(false)
const marketParseSubmitting = ref(false)
const marketURLImporting = ref(false)
const priceSourceLoading = ref(false)
const priceSourceImportingURL = ref('')
const cacheSubmitting = ref(false)
const qualitySubmitting = ref(false)
const acceptanceSubmitting = ref(false)
const acceptanceGenerating = ref(false)
const acceptanceRefreshing = ref(false)
const eventActionID = ref<number | null>(null)
const error = ref('')
const overview = ref<KanbanOverview | null>(null)
const priceSourceQuery = ref('')
const includeLowConfidenceSources = ref(false)
const priceSourceCandidates = ref<MarketPriceSourceCandidate[]>([])
const kanbanSettingsStorageKey = 'admin_plus_kanban_settings'
const savedKanbanSettings = loadKanbanSettings()

const filters = reactive({
  model: '',
  target_margin_percent: savedKanbanSettings.target_margin_percent,
  risk_buffer_percent: savedKanbanSettings.risk_buffer_percent
})

const marketPricePerMillion = ref(0)
const cacheHitRatioPercent = ref(0)
const estimatedWasteDollars = ref(0)

const marketForm = reactive<MarketForm>(defaultMarketForm())
const marketParseForm = reactive<MarketParseForm>(defaultMarketParseForm())
const cacheForm = reactive<CacheForm>(defaultCacheForm())
const qualityForm = reactive<QualityForm>(defaultQualityForm())
const acceptanceForm = reactive<AcceptanceForm>(defaultAcceptanceForm())

const modelMargins = computed(() => overview.value?.model_margins || [])
const recentEvents = computed(() => overview.value?.recent_events || [])
const currentSection = computed<KanbanSection>(() => props.section || 'profit')
const showModelFilter = computed(() => currentSection.value !== 'settings')
const showProfitControls = computed(() => currentSection.value === 'profit')
const pageTitle = computed(() => {
  return {
    'market-prices': '市场价格',
    'supply-quality': '供应质量',
    profit: '模型利润',
    acceptance: '接入验收',
    events: '价格事件',
    settings: '运营看板设置'
  }[currentSection.value]
})
const pageDescription = computed(() => {
  return {
    'market-prices': '采集、解析和查看同行公开售价、套餐、倍率和促销。',
    'supply-quality': '用真实 usage 和错误日志派生自有号池、第三方供应商的缓存效率、质量评分和生产决策。',
    profit: '按模型汇总市场价、真实成本、缓存惩罚、建议售价和风险等级。',
    acceptance: '对新供应商或自有号池账号执行接入前检查并生成验收报告。',
    events: '处理市场降价、异常低价、缓存风险、质量风险和验收风险事件。',
    settings: '配置运营看板计算毛利和风险缓冲时使用的本地参数。'
  }[currentSection.value]
})

watch(
  () => [filters.target_margin_percent, filters.risk_buffer_percent],
  () => saveKanbanSettings(),
  { deep: false }
)

async function loadData() {
  if (loading.value) return
  loading.value = true
  error.value = ''
  try {
    overview.value = await getKanbanOverview({
      model: filters.model || undefined,
      target_margin_percent: filters.target_margin_percent || undefined,
      risk_buffer_percent: filters.risk_buffer_percent || undefined,
      limit: 500
    })
  } catch (err) {
    overview.value = null
    error.value = errorMessage(err, '加载运营看板失败')
    appStore.showError(error.value)
  } finally {
    loading.value = false
  }
}

function loadKanbanSettings(): { target_margin_percent: number; risk_buffer_percent: number } {
  try {
    const raw = localStorage.getItem(kanbanSettingsStorageKey)
    const parsed = raw ? JSON.parse(raw) as Partial<{ target_margin_percent: number; risk_buffer_percent: number }> : {}
    return {
      target_margin_percent: positiveNumber(parsed.target_margin_percent, 25),
      risk_buffer_percent: nonNegativeNumber(parsed.risk_buffer_percent, 8)
    }
  } catch {
    return { target_margin_percent: 25, risk_buffer_percent: 8 }
  }
}

function saveKanbanSettings() {
  localStorage.setItem(kanbanSettingsStorageKey, JSON.stringify({
    target_margin_percent: positiveNumber(filters.target_margin_percent, 25),
    risk_buffer_percent: nonNegativeNumber(filters.risk_buffer_percent, 8)
  }))
}

async function saveKanbanSettingsAndReload() {
  saveKanbanSettings()
  appStore.showSuccess('运营看板设置已保存')
  await loadData()
}

async function submitMarketPrice() {
  if (marketSubmitting.value) return
  if (!marketForm.model) {
    appStore.showError('请填写模型')
    return
  }
  marketSubmitting.value = true
  try {
    const payload: CreateMarketPricePayload = {
      source_type: marketForm.source_type,
      source_name: marketForm.source_name,
      source_url: marketForm.source_url,
      site_id: optionalPositiveID(marketForm.site_id),
      supplier_id: optionalPositiveID(marketForm.supplier_id),
      model: marketForm.model,
      billing_mode: marketForm.billing_mode,
      price_item: marketForm.price_item,
      unit: marketForm.unit,
      currency: normalizeCurrency(marketForm.currency),
      price_micros: dollarsToMicros(marketPricePerMillion.value),
      rate_multiplier: nullableNumber(marketForm.rate_multiplier),
      confidence: marketForm.confidence || 1
    }
    await recordMarketPrice(payload)
    appStore.showSuccess('市场价快照已保存')
    resetMarketForm()
    await loadData()
  } catch (err) {
    appStore.showError(errorMessage(err, '保存市场价失败'))
  } finally {
    marketSubmitting.value = false
  }
}

async function submitMarketPriceParse() {
  if (marketParseSubmitting.value) return
  if (!marketParseForm.text.trim()) {
    appStore.showError('请粘贴公开价格文本')
    return
  }
  marketParseSubmitting.value = true
  try {
    const payload: ParseMarketPricesPayload = {
      source_type: marketParseForm.source_type,
      source_name: marketParseForm.source_name,
      source_url: marketParseForm.source_url,
      site_id: optionalPositiveID(marketParseForm.site_id),
      supplier_id: optionalPositiveID(marketParseForm.supplier_id),
      default_currency: normalizeCurrency(marketParseForm.default_currency),
      confidence: marketParseForm.confidence || 0.7,
      text: marketParseForm.text
    }
    const result = await parseMarketPrices(payload)
    appStore.showSuccess(`已解析 ${result.total} 条市场价`)
    resetMarketParseForm()
    await loadData()
  } catch (err) {
    appStore.showError(errorMessage(err, '解析价格文本失败'))
  } finally {
    marketParseSubmitting.value = false
  }
}

function marketPriceURLPayload(sourceURL: string): ImportMarketPricesFromURLPayload {
  return {
    source_type: marketParseForm.source_type,
    source_name: marketParseForm.source_name,
    source_url: sourceURL,
    site_id: optionalPositiveID(marketParseForm.site_id),
    supplier_id: optionalPositiveID(marketParseForm.supplier_id),
    default_currency: normalizeCurrency(marketParseForm.default_currency),
    confidence: marketParseForm.confidence || 0.7
  }
}

async function importCurrentMarketPriceURL() {
  if (marketURLImporting.value) return
  const sourceURL = marketParseForm.source_url.trim()
  if (!sourceURL) {
    appStore.showError('请填写来源 URL')
    return
  }
  marketURLImporting.value = true
  try {
    const result = await importMarketPricesFromURL(marketPriceURLPayload(sourceURL))
    appStore.showSuccess(`已抓取并解析 ${result.total} 条市场价`)
    await loadData()
  } catch (err) {
    appStore.showError(errorMessage(err, '抓取价格页失败'))
  } finally {
    marketURLImporting.value = false
  }
}

async function discoverPriceSources() {
  if (priceSourceLoading.value) return
  priceSourceLoading.value = true
  try {
    const result = await discoverMarketPriceSources({
      q: priceSourceQuery.value || undefined,
      include_low_confidence: includeLowConfidenceSources.value || undefined,
      limit: 20
    })
    priceSourceCandidates.value = result.items || []
    if (result.total === 0) {
      appStore.showSuccess('未发现价格页候选')
    } else {
      appStore.showSuccess(`已发现 ${result.total} 个价格页候选`)
    }
  } catch (err) {
    priceSourceCandidates.value = []
    appStore.showError(errorMessage(err, '发现价格页候选失败'))
  } finally {
    priceSourceLoading.value = false
  }
}

function applyPriceSourceCandidate(candidate: MarketPriceSourceCandidate) {
  marketParseForm.source_type = (candidate.source_type || 'site_catalog') as MarketPriceSourceType
  marketParseForm.source_name = candidate.source_name || ''
  marketParseForm.source_url = candidate.source_url || ''
  marketParseForm.site_id = candidate.site_id || null
  marketParseForm.supplier_id = candidate.supplier_id || null
  appStore.showSuccess('已填入价格解析来源')
}

async function importPriceSourceCandidate(candidate: MarketPriceSourceCandidate) {
  if (!candidate.source_url || priceSourceImportingURL.value) return
  applyPriceSourceCandidate(candidate)
  priceSourceImportingURL.value = candidate.source_url
  try {
    const result = await importMarketPricesFromURL(marketPriceURLPayload(candidate.source_url))
    appStore.showSuccess(`已抓取并解析 ${result.total} 条市场价`)
    await loadData()
  } catch (err) {
    appStore.showError(errorMessage(err, '抓取候选价格页失败'))
  } finally {
    priceSourceImportingURL.value = ''
  }
}

async function submitCacheEfficiency() {
  if (cacheSubmitting.value) return
  if (!cacheForm.model) {
    appStore.showError('请填写模型')
    return
  }
  cacheSubmitting.value = true
  try {
    const payload: CreateCacheEfficiencyPayload = {
      supply_type: cacheForm.supply_type,
      supplier_id: optionalPositiveID(cacheForm.supplier_id),
      local_sub2api_account_id: optionalPositiveID(cacheForm.local_sub2api_account_id),
      model: cacheForm.model,
      routing_strategy: cacheForm.routing_strategy,
      sticky_scope: cacheForm.sticky_scope,
      sample_requests: wholeNumber(cacheForm.sample_requests),
      cache_read_tokens: wholeNumber(cacheForm.cache_read_tokens),
      cache_write_tokens: wholeNumber(cacheForm.cache_write_tokens),
      input_tokens: wholeNumber(cacheForm.input_tokens),
      output_tokens: wholeNumber(cacheForm.output_tokens),
      duplicate_input_tokens: wholeNumber(cacheForm.duplicate_input_tokens),
      estimated_waste_cents: dollarsToCents(estimatedWasteDollars.value),
      cache_hit_ratio: clampRatio(cacheHitRatioPercent.value / 100),
      avg_ttft_ms: nullableWholeNumber(cacheForm.avg_ttft_ms),
      avg_total_latency_ms: nullableWholeNumber(cacheForm.avg_total_latency_ms),
      status: cacheForm.status || undefined,
      notes: cacheForm.notes
    }
    await recordCacheEfficiency(payload)
    appStore.showSuccess('缓存效率快照已保存')
    resetCacheForm()
    await loadData()
  } catch (err) {
    appStore.showError(errorMessage(err, '保存缓存审计失败'))
  } finally {
    cacheSubmitting.value = false
  }
}

async function submitSupplyQuality() {
  if (qualitySubmitting.value) return
  qualitySubmitting.value = true
  try {
    const payload: CreateSupplyQualityPayload = {
      supply_type: qualityForm.supply_type,
      supplier_id: optionalPositiveID(qualityForm.supplier_id),
      local_sub2api_account_id: optionalPositiveID(qualityForm.local_sub2api_account_id),
      model: qualityForm.model,
      availability_ratio: clampRatio(qualityForm.availability_percent / 100),
      error_ratio: clampRatio(qualityForm.error_percent / 100),
      avg_ttft_ms: nullableWholeNumber(qualityForm.avg_ttft_ms),
      avg_total_latency_ms: nullableWholeNumber(qualityForm.avg_total_latency_ms),
      cache_hit_ratio: clampRatio(qualityForm.cache_hit_percent / 100),
      purity_score: clampScore(qualityForm.purity_score),
      usage_trust_score: clampScore(qualityForm.usage_trust_score),
      balance_risk_score: clampScore(qualityForm.balance_risk_score),
      concurrency_score: clampScore(qualityForm.concurrency_score),
      quality_score: clampScore(qualityForm.quality_score),
      decision: qualityForm.decision || undefined,
      notes: qualityForm.notes
    }
    await recordSupplyQuality(payload)
    appStore.showSuccess('供应质量快照已保存')
    resetQualityForm()
    await loadData()
  } catch (err) {
    appStore.showError(errorMessage(err, '保存供应质量失败'))
  } finally {
    qualitySubmitting.value = false
  }
}

async function submitAcceptanceReport() {
  if (acceptanceSubmitting.value) return
  acceptanceSubmitting.value = true
  try {
    const payload: CreateAcceptanceReportPayload = {
      supply_type: acceptanceForm.supply_type,
      supplier_id: optionalPositiveID(acceptanceForm.supplier_id),
      local_sub2api_account_id: optionalPositiveID(acceptanceForm.local_sub2api_account_id),
      model: acceptanceForm.model,
      status: acceptanceForm.status || undefined,
      connectivity_status: acceptanceForm.connectivity_status,
      model_list_status: acceptanceForm.model_list_status,
      purity_status: acceptanceForm.purity_status,
      trial_call_status: acceptanceForm.trial_call_status,
      usage_metering_status: acceptanceForm.usage_metering_status,
      cache_audit_status: acceptanceForm.cache_audit_status,
      balance_status: acceptanceForm.balance_status,
      concurrency_status: acceptanceForm.concurrency_status,
      failure_reason: acceptanceForm.failure_reason,
      recommendation: acceptanceForm.recommendation
    }
    await recordAcceptanceReport(payload)
    appStore.showSuccess('接入验收报告已保存')
    resetAcceptanceForm()
    await loadData()
  } catch (err) {
    appStore.showError(errorMessage(err, '保存接入验收失败'))
  } finally {
    acceptanceSubmitting.value = false
  }
}

async function generateAcceptanceFromEvidence() {
  if (acceptanceGenerating.value) return
  const supplierID = optionalPositiveID(acceptanceForm.supplier_id)
  const accountID = optionalPositiveID(acceptanceForm.local_sub2api_account_id)
  if (!acceptanceForm.model && !supplierID && !accountID) {
    appStore.showError('请填写模型、供应商 ID 或本地账号 ID')
    return
  }
  acceptanceGenerating.value = true
  try {
    const payload: GenerateAcceptanceReportPayload = {
      supply_type: acceptanceForm.supply_type,
      supplier_id: supplierID,
      local_sub2api_account_id: accountID,
      model: acceptanceForm.model || undefined,
      enqueue_evidence_tasks: acceptanceForm.enqueue_evidence_tasks || undefined
    }
    await generateAcceptanceReport(payload)
    appStore.showSuccess('已按质量和缓存证据生成验收报告')
    await loadData()
  } catch (err) {
    appStore.showError(errorMessage(err, '生成接入验收失败'))
  } finally {
    acceptanceGenerating.value = false
  }
}

async function refreshAcceptanceFromEvidenceRun() {
  if (acceptanceRefreshing.value) return
  const runID = acceptanceForm.evidence_scheduler_run_id.trim()
  if (!runID) {
    appStore.showError('请填写调度 Run ID')
    return
  }
  acceptanceRefreshing.value = true
  try {
    const payload: RefreshAcceptanceReportFromEvidenceRunPayload = {
      run_id: runID
    }
    await refreshAcceptanceReportFromEvidenceRun(payload)
    appStore.showSuccess('已从调度证据回填验收报告')
    await loadData()
  } catch (err) {
    appStore.showError(errorMessage(err, '回填调度验收失败'))
  } finally {
    acceptanceRefreshing.value = false
  }
}

async function setEventStatus(id: number, status: KanbanEventStatus) {
  if (eventActionID.value) return
  eventActionID.value = id
  try {
    await updateKanbanEventStatus(id, status)
    appStore.showSuccess(status === 'acknowledged' ? '事件已处理' : '事件已忽略')
    await loadData()
  } catch (err) {
    appStore.showError(errorMessage(err, '更新事件失败'))
  } finally {
    eventActionID.value = null
  }
}

function defaultMarketForm(): MarketForm {
  return {
    source_type: 'manual',
    source_name: '',
    source_url: '',
    site_id: null,
    supplier_id: null,
    model: '',
    billing_mode: 'tokens',
    price_item: 'blended',
    unit: '1m_tokens',
    currency: 'USD',
    rate_multiplier: null,
    confidence: 1
  }
}

function defaultMarketParseForm(): MarketParseForm {
  return {
    source_type: 'provider_page',
    source_name: '',
    source_url: '',
    site_id: null,
    supplier_id: null,
    default_currency: 'USD',
    confidence: 0.7,
    text: ''
  }
}

function defaultCacheForm(): CacheForm {
  return {
    supply_type: 'own_pool',
    supplier_id: null,
    local_sub2api_account_id: null,
    model: '',
    routing_strategy: 'round_robin',
    sticky_scope: 'none',
    sample_requests: 0,
    cache_read_tokens: 0,
    cache_write_tokens: 0,
    input_tokens: 0,
    output_tokens: 0,
    duplicate_input_tokens: 0,
    avg_ttft_ms: null,
    avg_total_latency_ms: null,
    status: '',
    notes: '轮询号池可能降低 prompt cache 命中率'
  }
}

function defaultQualityForm(): QualityForm {
  return {
    supply_type: 'supplier',
    supplier_id: null,
    local_sub2api_account_id: null,
    model: '',
    availability_percent: 99,
    error_percent: 1,
    avg_ttft_ms: null,
    avg_total_latency_ms: null,
    cache_hit_percent: 65,
    purity_score: 90,
    usage_trust_score: 90,
    balance_risk_score: 10,
    concurrency_score: 80,
    quality_score: 0,
    decision: '',
    notes: ''
  }
}

function defaultAcceptanceForm(): AcceptanceForm {
  return {
    supply_type: 'supplier',
    supplier_id: null,
    local_sub2api_account_id: null,
    model: '',
    status: '',
    connectivity_status: 'unknown',
    model_list_status: 'unknown',
    purity_status: 'unknown',
    trial_call_status: 'unknown',
    usage_metering_status: 'unknown',
    cache_audit_status: 'unknown',
    balance_status: 'unknown',
    concurrency_status: 'unknown',
    failure_reason: '',
    recommendation: '',
    enqueue_evidence_tasks: false,
    evidence_scheduler_run_id: ''
  }
}

function resetMarketForm() {
  Object.assign(marketForm, defaultMarketForm())
  marketPricePerMillion.value = 0
}

function resetMarketParseForm() {
  Object.assign(marketParseForm, defaultMarketParseForm())
}

function resetCacheForm() {
  Object.assign(cacheForm, defaultCacheForm())
  cacheHitRatioPercent.value = 0
  estimatedWasteDollars.value = 0
}

function resetQualityForm() {
  Object.assign(qualityForm, defaultQualityForm())
}

function resetAcceptanceForm() {
  Object.assign(acceptanceForm, defaultAcceptanceForm())
}

function dollarsToMicros(value: number): number {
  return Math.max(0, Math.round((Number(value) || 0) * 1_000_000))
}

function dollarsToCents(value: number): number {
  return Math.max(0, Math.round((Number(value) || 0) * 100))
}

function wholeNumber(value: number): number {
  return Math.max(0, Math.round(Number(value) || 0))
}

function nullableWholeNumber(value: number | null): number | null {
  if (value === null || value === undefined || !Number.isFinite(Number(value))) return null
  return wholeNumber(value)
}

function optionalPositiveID(value: number | null): number | undefined {
  const normalized = wholeNumber(Number(value) || 0)
  return normalized > 0 ? normalized : undefined
}

function nullableNumber(value: number | null): number | null {
  if (value === null || value === undefined || !Number.isFinite(Number(value))) return null
  return Number(value)
}

function clampRatio(value: number): number {
  if (!Number.isFinite(value)) return 0
  return Math.min(1, Math.max(0, value))
}

function positiveNumber(value: unknown, fallback: number): number {
  const parsed = Number(value)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback
}

function nonNegativeNumber(value: unknown, fallback: number): number {
  const parsed = Number(value)
  return Number.isFinite(parsed) && parsed >= 0 ? parsed : fallback
}

function clampScore(value: number): number {
  if (!Number.isFinite(value)) return 0
  return Math.min(100, Math.max(0, value))
}

function normalizeCurrency(value: string): string {
  const normalized = (value || 'USD').trim().toUpperCase()
  return normalized.length === 3 ? normalized : 'USD'
}

function formatMicros(value: number | null | undefined, currency: string): string {
  if (value === null || value === undefined) return '-'
  return new Intl.NumberFormat(undefined, {
    style: 'currency',
    currency: normalizeCurrency(currency),
    currencyDisplay: 'narrowSymbol',
    maximumFractionDigits: 6
  }).format(value / 1_000_000)
}

function formatPriceRange(row: KanbanModelMarginRow): string {
  if (row.market_low_price_micros === undefined || row.market_low_price_micros === null) return '-'
  return [
    formatMicros(row.market_low_price_micros, row.currency),
    formatMicros(row.market_median_price_micros, row.currency),
    formatMicros(row.market_high_price_micros, row.currency)
  ].join(' / ')
}

function formatPercent(value?: number | null): string {
  if (value === null || value === undefined || !Number.isFinite(value)) return '-'
  return `${value.toFixed(2).replace(/\.?0+$/, '')}%`
}

function formatPercentFraction(value?: number | null): string {
  if (value === null || value === undefined || !Number.isFinite(value)) return '-'
  return formatPercent(value * 100)
}

function marginClass(value?: number | null): string {
  if (value === null || value === undefined) return 'text-gray-500 dark:text-dark-400'
  if (value < 0) return 'text-red-600 dark:text-red-300'
  if (value < filters.target_margin_percent) return 'text-amber-600 dark:text-amber-300'
  return 'text-emerald-600 dark:text-emerald-300'
}

function formatScore(value?: number | null): string {
  if (value === null || value === undefined || !Number.isFinite(value)) return '-'
  return value.toFixed(1).replace(/\.0$/, '')
}

function riskLabel(value?: string): string {
  if (value === 'high') return '高'
  if (value === 'medium') return '中'
  if (value === 'low') return '低'
  return '未知'
}

function riskClass(value?: KanbanRiskLevel | string): string {
  if (value === 'high') return 'badge-danger'
  if (value === 'medium') return 'badge-warning'
  if (value === 'low') return 'badge-success'
  return 'badge-gray'
}

function cacheStatusLabel(value?: string): string {
  if (value === 'healthy') return '健康'
  if (value === 'watching') return '观察'
  if (value === 'risky') return '风险'
  if (value === 'bad') return '严重'
  return '未知'
}

function cacheStatusClass(value?: string): string {
  if (value === 'healthy') return 'badge-success'
  if (value === 'watching') return 'badge-primary'
  if (value === 'risky') return 'badge-warning'
  if (value === 'bad') return 'badge-danger'
  return 'badge-gray'
}

function eventTypeLabel(value?: string): string {
  return {
    market_price_drop: '市场降价',
    market_price_rise: '市场涨价',
    market_price_anomaly: '异常低价',
    market_model_added: '新增模型',
    market_model_removed: '模型下架',
    market_promotion: '限时活动',
    cache_efficiency_risk: '缓存风险',
    supply_quality_risk: '质量风险',
    acceptance_risk: '验收风险',
    unprofitable_model: '毛利倒挂',
    pricing_recommendation: '定价建议'
  }[value || ''] || value || '-'
}

function confidenceLabel(value?: number): string {
  if (value === undefined || value === null || !Number.isFinite(value)) return '置信 -'
  return `置信 ${Math.round(value * 100)}%`
}

function candidateReasonLabel(value?: string): string {
  return {
    price_keyword_in_link: '价格链接',
    recharge_or_topup_link: '充值链接',
    docs_link_may_contain_pricing: '文档链接',
    homepage_candidate: '主页候选',
    derived_pricing_path: '推测 /pricing',
    derived_price_path: '推测 /price',
    derived_models_path: '推测 /models',
    derived_recharge_path: '推测 /recharge',
    low_confidence_link: '低置信链接'
  }[value || ''] || value || '-'
}

function eventSeverityLabel(value?: string): string {
  if (value === 'critical') return '严重'
  if (value === 'warning') return '警告'
  return '信息'
}

function eventSeverityClass(value?: string): string {
  if (value === 'critical') return 'badge-danger'
  if (value === 'warning') return 'badge-warning'
  return 'badge-primary'
}

function eventStatusLabel(value?: string): string {
  if (value === 'acknowledged') return '已处理'
  if (value === 'ignored') return '已忽略'
  return '开放'
}

function eventStatusClass(value?: string): string {
  if (value === 'acknowledged') return 'badge-success'
  if (value === 'ignored') return 'badge-gray'
  return 'badge-warning'
}

function qualityDecisionLabel(value?: string): string {
  if (value === 'production') return '生产'
  if (value === 'watching') return '观察'
  if (value === 'low_priority') return '低优先'
  if (value === 'paused') return '暂停'
  if (value === 'blocked') return '阻断'
  return '未知'
}

function qualityDecisionClass(value?: string): string {
  if (value === 'production') return 'badge-success'
  if (value === 'watching') return 'badge-primary'
  if (value === 'low_priority') return 'badge-warning'
  if (value === 'paused' || value === 'blocked') return 'badge-danger'
  return 'badge-gray'
}

function acceptanceStepLabel(value?: string): string {
  if (value === 'pass') return '通过'
  if (value === 'warn') return '警告'
  if (value === 'fail') return '失败'
  return '未知'
}

function acceptanceStepClass(value?: string): string {
  if (value === 'pass') return 'badge-success'
  if (value === 'warn') return 'badge-warning'
  if (value === 'fail') return 'badge-danger'
  return 'badge-gray'
}

function sourceTypeLabel(value?: string): string {
  return {
    manual: '手工',
    site_catalog: '目录',
    site_discovery: '采集',
    provider_page: '页面',
    api: 'API'
  }[value || ''] || value || '-'
}

function supplyTypeLabel(value?: string): string {
  return {
    supplier: '第三方供应商',
    own_pool: '自有号池',
    competitor: '竞品',
    custom: '自定义'
  }[value || ''] || value || '-'
}

function snapshotSourceLabel(item: { raw_payload?: Record<string, unknown> }): string {
  const payload = item.raw_payload || {}
  const source = typeof payload.source === 'string' ? payload.source : ''
  const errorCount = Number(payload.error_count || 0)
  if (payload.derived === true) {
    if (source === 'usage_logs' && errorCount > 0) return '自动派生 · usage/错误日志'
    if (source === 'usage_logs') return '自动派生 · usage 日志'
    return source ? `自动派生 · ${source}` : '自动派生'
  }
  return source ? source : '手工/任务快照'
}

function snapshotTargetLabel(item: { supplier_id?: number; local_sub2api_account_id?: number }): string {
  const parts: string[] = []
  if (item.supplier_id) parts.push(`供应商 ${item.supplier_id}`)
  if (item.local_sub2api_account_id) parts.push(`账号 ${item.local_sub2api_account_id}`)
  return parts.join(' · ')
}

function routingStrategyLabel(value?: string): string {
  return {
    fixed_account: '固定账号',
    round_robin: '轮询号池',
    weighted_round_robin: '加权轮询',
    sticky: '粘性路由',
    least_loaded: '最小负载',
    custom: '自定义',
    unknown: '未知'
  }[value || ''] || value || '-'
}

function formatMoneyCents(value?: number | null, currency = 'USD'): string {
  if (value === null || value === undefined) return '-'
  return new Intl.NumberFormat(undefined, {
    style: 'currency',
    currency: normalizeCurrency(currency),
    currencyDisplay: 'narrowSymbol',
    minimumFractionDigits: 2
  }).format(value / 100)
}

function errorMessage(error: unknown, fallback: string): string {
  return (error as { message?: string })?.message || fallback
}

const RecentSnapshotTable = defineComponent({
  name: 'RecentSnapshotTable',
  props: {
    title: { type: String, required: true },
    items: { type: Array, required: true },
    type: { type: String, required: true }
  },
  setup(props) {
    return () => h('section', { class: 'card overflow-hidden' }, [
      h('div', { class: 'border-b border-gray-100 px-5 py-4 dark:border-dark-700' }, [
        h('h2', { class: 'text-base font-semibold text-gray-900 dark:text-white' }, props.title)
      ]),
      h('div', { class: 'overflow-x-auto' }, [
        props.type === 'market'
          ? h(MarketSnapshotTable, { items: props.items as MarketPriceSnapshot[] })
          : props.type === 'cache'
            ? h(CacheSnapshotTable, { items: props.items as CacheEfficiencySnapshot[] })
            : props.type === 'quality'
              ? h(QualitySnapshotTable, { items: props.items as SupplyQualitySnapshot[] })
              : h(AcceptanceSnapshotTable, { items: props.items as AcceptanceReport[] })
      ])
    ])
  }
})

const MarketSnapshotTable = defineComponent({
  name: 'MarketSnapshotTable',
  props: {
    items: { type: Array, required: true }
  },
  setup(props) {
    return () => h('table', { class: 'w-full min-w-[760px] divide-y divide-gray-200 dark:divide-dark-700' }, [
      h('thead', { class: 'bg-gray-50 dark:bg-dark-800' }, [
        h('tr', [
          h('th', { class: 'px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400' }, '来源'),
          h('th', { class: 'px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400' }, '模型'),
          h('th', { class: 'px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400' }, '价格'),
          h('th', { class: 'px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400' }, '时间')
        ])
      ]),
      h('tbody', { class: 'divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-900' }, [
        ...(props.items as MarketPriceSnapshot[]).length === 0
          ? [h('tr', [h('td', { class: 'px-4 py-8 text-center text-sm text-gray-500 dark:text-dark-400', colspan: 4 }, '暂无市场价快照')])]
          : (props.items as MarketPriceSnapshot[]).map((item) => h('tr', { key: item.id }, [
            h('td', { class: 'px-4 py-4 text-sm text-gray-900 dark:text-gray-100' }, [
              h('div', { class: 'font-medium' }, item.source_name || '-'),
              h('div', { class: 'mt-1 text-xs text-gray-500 dark:text-dark-400' }, sourceTypeLabel(item.source_type))
            ]),
            h('td', { class: 'px-4 py-4 text-sm text-gray-900 dark:text-gray-100' }, item.model),
            h('td', { class: 'px-4 py-4 text-right text-sm font-medium text-gray-900 dark:text-gray-100' }, formatMicros(item.price_micros, item.currency)),
            h('td', { class: 'px-4 py-4 text-sm text-gray-500 dark:text-dark-400' }, formatDateTime(item.observed_at))
          ]))
      ])
    ])
  }
})

const CacheSnapshotTable = defineComponent({
  name: 'CacheSnapshotTable',
  props: {
    items: { type: Array, required: true }
  },
  setup(props) {
    return () => h('table', { class: 'w-full min-w-[860px] divide-y divide-gray-200 dark:divide-dark-700' }, [
      h('thead', { class: 'bg-gray-50 dark:bg-dark-800' }, [
        h('tr', [
          h('th', { class: 'px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400' }, '对象'),
          h('th', { class: 'px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400' }, '模型'),
          h('th', { class: 'px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400' }, '策略'),
          h('th', { class: 'px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400' }, '命中率'),
          h('th', { class: 'px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400' }, '浪费'),
          h('th', { class: 'px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400' }, '状态')
        ])
      ]),
      h('tbody', { class: 'divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-900' }, [
        ...(props.items as CacheEfficiencySnapshot[]).length === 0
          ? [h('tr', [h('td', { class: 'px-4 py-8 text-center text-sm text-gray-500 dark:text-dark-400', colspan: 6 }, '暂无缓存审计快照')])]
          : (props.items as CacheEfficiencySnapshot[]).map((item) => {
            const target = snapshotTargetLabel(item)
            return h('tr', { key: item.id }, [
              h('td', { class: 'px-4 py-4 text-sm text-gray-900 dark:text-gray-100' }, [
                h('div', supplyTypeLabel(item.supply_type)),
                h('div', { class: 'mt-1 text-xs text-gray-500 dark:text-dark-400' }, snapshotSourceLabel(item)),
                target
                  ? h('div', { class: 'mt-1 text-xs text-gray-500 dark:text-dark-400' }, target)
                  : null
              ]),
              h('td', { class: 'px-4 py-4 text-sm text-gray-900 dark:text-gray-100' }, item.model),
              h('td', { class: 'px-4 py-4 text-sm text-gray-900 dark:text-gray-100' }, [
                h('div', routingStrategyLabel(item.routing_strategy)),
                h('div', { class: 'mt-1 text-xs text-gray-500 dark:text-dark-400' }, `sticky: ${item.sticky_scope || 'none'}`)
              ]),
              h('td', { class: 'px-4 py-4 text-right text-sm font-medium text-gray-900 dark:text-gray-100' }, formatPercentFraction(item.cache_hit_ratio)),
              h('td', { class: 'px-4 py-4 text-right text-sm text-gray-900 dark:text-gray-100' }, formatMoneyCents(item.estimated_waste_cents)),
              h('td', { class: 'px-4 py-4 text-sm' }, [
                h('span', { class: ['badge', cacheStatusClass(item.status)] }, cacheStatusLabel(item.status)),
                h('div', { class: 'mt-1 text-xs text-gray-500 dark:text-dark-400' }, formatDateTime(item.observed_at))
              ])
            ])
          })
      ])
    ])
  }
})

const QualitySnapshotTable = defineComponent({
  name: 'QualitySnapshotTable',
  props: {
    items: { type: Array, required: true }
  },
  setup(props) {
    return () => h('table', { class: 'w-full min-w-[760px] divide-y divide-gray-200 dark:divide-dark-700' }, [
      h('thead', { class: 'bg-gray-50 dark:bg-dark-800' }, [
        h('tr', [
          h('th', { class: 'px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400' }, '对象'),
          h('th', { class: 'px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400' }, '模型'),
          h('th', { class: 'px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400' }, '质量分'),
          h('th', { class: 'px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400' }, '决策')
        ])
      ]),
      h('tbody', { class: 'divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-900' }, [
        ...(props.items as SupplyQualitySnapshot[]).length === 0
          ? [h('tr', [h('td', { class: 'px-4 py-8 text-center text-sm text-gray-500 dark:text-dark-400', colspan: 4 }, '暂无供应质量快照')])]
          : (props.items as SupplyQualitySnapshot[]).map((item) => {
            const target = snapshotTargetLabel(item)
            return h('tr', { key: item.id }, [
              h('td', { class: 'px-4 py-4 text-sm text-gray-900 dark:text-gray-100' }, [
                h('div', supplyTypeLabel(item.supply_type)),
                h('div', { class: 'mt-1 text-xs text-gray-500 dark:text-dark-400' }, snapshotSourceLabel(item)),
                target
                  ? h('div', { class: 'mt-1 text-xs text-gray-500 dark:text-dark-400' }, target)
                  : null
              ]),
              h('td', { class: 'px-4 py-4 text-sm text-gray-900 dark:text-gray-100' }, item.model || '-'),
              h('td', { class: 'px-4 py-4 text-right text-sm font-medium text-gray-900 dark:text-gray-100' }, formatScore(item.quality_score)),
              h('td', { class: 'px-4 py-4 text-sm' }, [
                h('span', { class: ['badge', qualityDecisionClass(item.decision)] }, qualityDecisionLabel(item.decision)),
                h('div', { class: 'mt-1 text-xs text-gray-500 dark:text-dark-400' }, `${formatPercentFraction(item.availability_ratio)} 可用 · ${formatDateTime(item.observed_at)}`)
              ])
            ])
          })
      ])
    ])
  }
})

const AcceptanceSnapshotTable = defineComponent({
  name: 'AcceptanceSnapshotTable',
  props: {
    items: { type: Array, required: true }
  },
  setup(props) {
    return () => h('table', { class: 'w-full min-w-[960px] divide-y divide-gray-200 dark:divide-dark-700' }, [
      h('thead', { class: 'bg-gray-50 dark:bg-dark-800' }, [
        h('tr', [
          h('th', { class: 'px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400' }, '对象'),
          h('th', { class: 'px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400' }, '模型'),
          h('th', { class: 'px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400' }, '结论'),
          h('th', { class: 'px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400' }, '检查项'),
          h('th', { class: 'px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400' }, '时间')
        ])
      ]),
      h('tbody', { class: 'divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-900' }, [
        ...(props.items as AcceptanceReport[]).length === 0
          ? [h('tr', [h('td', { class: 'px-4 py-8 text-center text-sm text-gray-500 dark:text-dark-400', colspan: 5 }, '暂无接入验收报告')])]
          : (props.items as AcceptanceReport[]).map((item) => h('tr', { key: item.id }, [
            h('td', { class: 'px-4 py-4 text-sm text-gray-900 dark:text-gray-100' }, supplyTypeLabel(item.supply_type)),
            h('td', { class: 'px-4 py-4 text-sm text-gray-900 dark:text-gray-100' }, item.model || '-'),
            h('td', { class: 'px-4 py-4 text-sm' }, [
              h('span', { class: ['badge', qualityDecisionClass(item.status)] }, qualityDecisionLabel(item.status)),
              item.failure_reason
                ? h('div', { class: 'mt-1 max-w-[220px] truncate text-xs text-gray-500 dark:text-dark-400' }, item.failure_reason)
                : null
            ]),
            h('td', { class: 'px-4 py-4 text-sm' }, [
              h('div', { class: 'flex max-w-[420px] flex-wrap gap-1' }, acceptanceSteps.map((step) => h('span', {
                key: step.key,
                class: ['badge', acceptanceStepClass(item[step.key])]
              }, `${step.label} ${acceptanceStepLabel(item[step.key])}`)))
            ]),
            h('td', { class: 'px-4 py-4 text-sm text-gray-500 dark:text-dark-400' }, formatDateTime(item.observed_at))
          ]))
      ])
    ])
  }
})

onMounted(loadData)
</script>
