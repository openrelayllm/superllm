<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">渠道索引采集</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
            daheiai 渠道索引、注册任务、已注册账号和低倍率充值推荐。
          </p>
        </div>
        <div class="flex flex-wrap gap-2">
          <button type="button" class="btn btn-secondary" :disabled="loading" @click="loadPage">
            <Icon name="refresh" size="sm" />
            刷新
          </button>
          <button type="button" class="btn btn-secondary" :disabled="running || classifying || bulkAddingCatalog" @click="classifyAllItemsNow">
            <Icon name="search" size="sm" />
            {{ classifying ? '识别中...' : '一键识别全部' }}
          </button>
          <button type="button" class="btn btn-secondary" :disabled="running || classifying || bulkAddingCatalog" @click="bulkAddCatalogNow">
            <Icon name="database" size="sm" />
            {{ bulkAddingCatalog ? '加入中...' : '批量加入目录' }}
          </button>
          <button type="button" class="btn btn-primary" :disabled="running || classifying || bulkAddingCatalog" @click="runDiscoveryNow">
            <Icon name="play" size="sm" />
            {{ running ? '采集中...' : '运行采集' }}
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

      <section v-if="activeTab === 'dashboard'" class="grid gap-6 xl:grid-cols-[minmax(0,1.6fr)_minmax(320px,0.8fr)]">
        <div class="space-y-6">
          <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-5">
            <div class="card p-4">
              <p class="text-xs font-medium text-gray-500 dark:text-dark-400">采集网址</p>
              <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ urlPagination.total }}</p>
            </div>
            <div class="card p-4">
              <p class="text-xs font-medium text-gray-500 dark:text-dark-400">已支持</p>
              <p class="mt-2 text-2xl font-semibold text-emerald-600 dark:text-emerald-400">{{ supportedCount }}</p>
            </div>
            <div class="card p-4">
              <p class="text-xs font-medium text-gray-500 dark:text-dark-400">已注册</p>
              <p class="mt-2 text-2xl font-semibold text-sky-600 dark:text-sky-400">{{ registeredPagination.total }}</p>
            </div>
            <div class="card p-4">
              <p class="text-xs font-medium text-gray-500 dark:text-dark-400">待人工验证</p>
              <p class="mt-2 text-2xl font-semibold text-amber-600 dark:text-amber-400">{{ manualCount }}</p>
            </div>
            <div class="card p-4">
              <p class="text-xs font-medium text-gray-500 dark:text-dark-400">推荐充值</p>
              <p class="mt-2 text-2xl font-semibold text-primary-600 dark:text-primary-400">{{ recommendations.length }}</p>
            </div>
          </div>

          <div class="card p-5">
            <div class="flex items-center justify-between gap-3">
              <div>
                <h2 class="text-lg font-semibold text-gray-900 dark:text-white">采集工作台</h2>
                <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">当前默认采集第三方中转分区。</p>
              </div>
              <div class="flex flex-wrap gap-2">
                <button type="button" class="btn btn-secondary btn-sm" :disabled="running || classifying || bulkAddingCatalog" @click="classifyAllItemsNow">
                  {{ classifying ? '识别中...' : '一键识别全部' }}
                </button>
                <button type="button" class="btn btn-secondary btn-sm" :disabled="running || classifying || bulkAddingCatalog" @click="bulkAddCatalogNow">
                  {{ bulkAddingCatalog ? '加入中...' : '批量加入目录' }}
                </button>
                <button type="button" class="btn btn-primary btn-sm" :disabled="running || classifying || bulkAddingCatalog" @click="runDiscoveryNow">
                {{ running ? '采集中...' : '开始采集' }}
                </button>
              </div>
            </div>
            <div class="mt-4 grid gap-4 lg:grid-cols-[minmax(0,1fr)_140px_140px_120px] lg:items-end">
              <label class="block">
                <span class="input-label">采集源</span>
                <input v-model.trim="sourceURL" class="input font-mono text-sm" />
              </label>
              <label class="block">
                <span class="input-label">本次上限</span>
                <input v-model.number="runLimit" type="number" min="0" max="1000" class="input" />
              </label>
              <label class="inline-flex items-center gap-2 pb-2 text-sm text-gray-700 dark:text-dark-200">
                <input v-model="probeInterfaces" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600" />
                接口类型识别
              </label>
              <label class="inline-flex items-center gap-2 pb-2 text-sm text-gray-700 dark:text-dark-200">
                <input v-model="probeSites" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600" />
                页面深度探测
              </label>
            </div>
          </div>

          <div class="card overflow-hidden">
            <div class="flex items-center justify-between border-b border-gray-100 px-5 py-4 dark:border-dark-700">
              <div>
                <h2 class="text-lg font-semibold text-gray-900 dark:text-white">采集进度</h2>
                <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ discoveryProgressLabel }}</p>
              </div>
              <span class="badge" :class="running || classifying || bulkAddingCatalog ? 'badge-warning' : 'badge-gray'">{{ running ? '采集中' : classifying ? '识别中' : bulkAddingCatalog ? '加入目录中' : '空闲' }}</span>
            </div>
            <div class="p-5">
              <div class="h-2 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-800">
                <div class="h-full rounded-full bg-primary-500 transition-all" :style="{ width: `${discoveryProgressPercent}%` }"></div>
              </div>
              <div class="mt-3 flex items-center justify-between text-xs text-gray-500 dark:text-dark-400">
                <span>{{ discoveryProgress.current }} / {{ discoveryProgress.total }}</span>
                <span>{{ discoveryProgressPercent }}%</span>
              </div>
              <div ref="logContainerRef" class="mt-4 max-h-64 overflow-y-auto rounded-md border border-gray-100 bg-gray-50 p-3 font-mono text-xs dark:border-dark-700 dark:bg-dark-950">
                <div v-if="discoveryLogs.length === 0" class="py-6 text-center text-gray-500 dark:text-dark-400">暂无采集日志</div>
                <div
                  v-for="log in discoveryLogs"
                  :key="log.id"
                  class="flex gap-2 py-1"
                  :class="discoveryLogClass(log.level)"
                >
                  <span class="shrink-0">[{{ log.level }}]</span>
                  <span class="min-w-0 flex-1 break-all">{{ log.message }}</span>
                  <span v-if="log.total" class="shrink-0 text-gray-400">{{ log.current || 0 }}/{{ log.total }}</span>
                </div>
              </div>
            </div>
          </div>

          <div class="card overflow-hidden">
            <div class="flex items-center justify-between border-b border-gray-100 px-5 py-4 dark:border-dark-700">
              <div>
                <h2 class="text-lg font-semibold text-gray-900 dark:text-white">最近注册</h2>
                <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">插件已提交注册表单的渠道。</p>
              </div>
              <button type="button" class="btn btn-secondary btn-sm" @click="activeTab = 'registered'">查看全部</button>
            </div>
            <div class="divide-y divide-gray-100 dark:divide-dark-700">
              <div v-if="registeredItems.length === 0" class="px-5 py-8 text-center text-sm text-gray-500 dark:text-dark-400">暂无已注册渠道</div>
              <div v-for="item in registeredItems.slice(0, 5)" :key="item.id" class="grid gap-3 px-5 py-4 md:grid-cols-[minmax(0,1fr)_auto] md:items-center">
                <div class="min-w-0">
                  <div class="truncate text-sm font-semibold text-gray-900 dark:text-white">{{ item.name }}</div>
                  <div class="mt-1 truncate font-mono text-xs text-gray-500 dark:text-dark-400">{{ item.host }}</div>
                </div>
                <div class="flex flex-wrap items-center gap-2">
                  <span class="badge" :class="providerClass(item)">{{ providerLabel(item) }}</span>
                  <span class="badge" :class="registrationClass(item.registration_status)">{{ registrationLabel(item.registration_status) }}</span>
                </div>
              </div>
            </div>
          </div>
        </div>

        <aside class="space-y-6">
          <div class="card p-5">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">注册配置</h2>
            <dl class="mt-4 space-y-3 text-sm">
              <div class="flex items-center justify-between gap-4">
                <dt class="text-gray-500 dark:text-dark-400">插件注册</dt>
                <dd class="font-medium" :class="settings.registration_enabled ? 'text-emerald-600 dark:text-emerald-400' : 'text-gray-500 dark:text-dark-400'">
                  {{ settings.registration_enabled ? '已启用' : '未启用' }}
                </dd>
              </div>
              <div class="flex items-center justify-between gap-4">
                <dt class="text-gray-500 dark:text-dark-400">注册邮箱</dt>
                <dd class="max-w-[180px] truncate font-mono text-xs text-gray-900 dark:text-white">{{ settings.registration_email || '-' }}</dd>
              </div>
              <div class="flex items-center justify-between gap-4">
                <dt class="text-gray-500 dark:text-dark-400">低倍率阈值</dt>
                <dd class="font-medium text-gray-900 dark:text-white">{{ fixedRate(settings.low_rate_threshold) }}</dd>
              </div>
            </dl>
            <button type="button" class="btn btn-secondary mt-5 w-full" @click="activeTab = 'settings'">编辑设置</button>
          </div>

          <div class="card overflow-hidden">
            <div class="flex items-center justify-between border-b border-gray-100 px-5 py-4 dark:border-dark-700">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">注册任务</h2>
              <button type="button" class="btn btn-secondary btn-sm" @click="activeTab = 'tasks'">查看</button>
            </div>
            <div class="divide-y divide-gray-100 dark:divide-dark-700">
              <div v-if="registrationTaskItems.length === 0" class="px-5 py-8 text-center text-sm text-gray-500 dark:text-dark-400">暂无注册任务</div>
              <div v-for="item in registrationTaskItems.slice(0, 5)" :key="item.id" class="px-5 py-4">
                <div class="flex items-center justify-between gap-3">
                  <span class="min-w-0 truncate text-sm font-medium text-gray-900 dark:text-white">{{ item.name }}</span>
                  <span class="badge shrink-0" :class="registrationClass(item.registration_status)">{{ registrationLabel(item.registration_status) }}</span>
                </div>
                <p v-if="item.registration_error_message" class="mt-1 truncate text-xs text-rose-500">{{ item.registration_error_message }}</p>
              </div>
            </div>
          </div>
        </aside>
      </section>

      <section v-else-if="isTableTab" class="card overflow-hidden">
        <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
          <div class="grid gap-3 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-end">
            <div>
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ activeTableTitle }}</h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ activeTableDescription }}</p>
              <div v-if="activeTab === 'urls'" class="mt-3 flex flex-wrap gap-2">
                <button type="button" class="btn btn-sm" :class="processedFilterClass('unprocessed')" @click="setProcessedFilter('unprocessed')">未处理</button>
                <button type="button" class="btn btn-sm" :class="processedFilterClass('processed')" @click="setProcessedFilter('processed')">已处理</button>
                <button type="button" class="btn btn-sm" :class="processedFilterClass('')" @click="setProcessedFilter('')">全部</button>
                <span class="mx-1 h-8 border-l border-gray-200 dark:border-dark-700"></span>
                <button type="button" class="btn btn-sm" :class="providerFilterClass('new_api')" @click="setProviderFilter('new_api')">new-api</button>
                <button type="button" class="btn btn-sm" :class="providerFilterClass('sub2api')" @click="setProviderFilter('sub2api')">sub2api</button>
                <button type="button" class="btn btn-sm" :class="classificationFilterClass('unknown')" @click="setClassificationFilter('unknown')">未知</button>
                <button type="button" class="btn btn-sm" :class="providerFilterClass('')" @click="clearTypeFilters">全部类型</button>
                <span class="mx-1 h-8 border-l border-gray-200 dark:border-dark-700"></span>
                <button type="button" class="btn btn-secondary btn-sm" :disabled="running || classifying || bulkAddingCatalog" @click="bulkAddCatalogNow">
                  {{ bulkAddingCatalog ? '加入中...' : '批量加入目录' }}
                </button>
              </div>
            </div>

            <div v-if="activeTab === 'urls'" class="grid gap-2 sm:grid-cols-2 xl:grid-cols-5">
              <input v-model.trim="urlFilters.q" class="input h-9 py-1 text-sm" placeholder="搜索名称或域名" @keyup.enter="resetURLPagination" />
              <select v-model="urlFilters.provider_type" class="input h-9 py-1 text-sm" @change="resetURLPagination">
                <option value="">全部类型</option>
                <option value="new_api">new-api</option>
                <option value="sub2api">sub2api</option>
              </select>
              <select v-model="urlFilters.classification_status" class="input h-9 py-1 text-sm" @change="resetURLPagination">
                <option value="">全部识别</option>
                <option value="supported">支持</option>
                <option value="unknown">未知</option>
                <option value="unsupported">不支持</option>
              </select>
              <select v-model="urlFilters.import_status" class="input h-9 py-1 text-sm" @change="resetURLPagination">
                <option value="">全部导入</option>
                <option value="new">未导入</option>
                <option value="imported">已导入</option>
                <option value="skipped">已跳过</option>
              </select>
              <select v-model="urlFilters.registration_status" class="input h-9 py-1 text-sm" @change="resetURLPagination">
                <option value="">全部注册</option>
                <option value="queued">已排队</option>
                <option value="running">执行中</option>
                <option value="waiting_manual_verification">待人工验证</option>
                <option value="succeeded">成功</option>
                <option value="failed">失败</option>
              </select>
            </div>

            <div v-else-if="activeTab === 'registered'" class="grid gap-2 sm:grid-cols-2">
              <input v-model.trim="registeredFilters.q" class="input h-9 py-1 text-sm" placeholder="搜索名称或域名" @keyup.enter="resetRegisteredPagination" />
              <select v-model="registeredFilters.provider_type" class="input h-9 py-1 text-sm" @change="resetRegisteredPagination">
                <option value="">全部类型</option>
                <option value="new_api">new-api</option>
                <option value="sub2api">sub2api</option>
              </select>
            </div>

            <div v-else class="grid gap-2 sm:grid-cols-3">
              <input v-model.trim="taskFilters.q" class="input h-9 py-1 text-sm" placeholder="搜索名称或域名" @keyup.enter="resetTaskPagination" />
              <select v-model="taskFilters.provider_type" class="input h-9 py-1 text-sm" @change="resetTaskPagination">
                <option value="">全部类型</option>
                <option value="new_api">new-api</option>
                <option value="sub2api">sub2api</option>
              </select>
              <select v-model="taskFilters.registration_status" class="input h-9 py-1 text-sm" @change="resetTaskPagination">
                <option value="">全部任务</option>
                <option value="queued">已排队</option>
                <option value="running">执行中</option>
                <option value="waiting_manual_verification">待人工验证</option>
                <option value="succeeded">成功</option>
                <option value="failed">失败</option>
              </select>
            </div>
          </div>
        </div>

        <div class="overflow-x-auto">
          <table class="w-full min-w-[1280px] divide-y divide-gray-200 dark:divide-dark-700">
            <thead class="bg-gray-50 dark:bg-dark-800">
              <tr>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">站点</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">类型</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">监控</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">导入</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">注册</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">处理</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">操作</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-900">
              <tr v-if="activeTableItems.length === 0">
                <td colspan="7" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-dark-400">{{ activeEmptyLabel }}</td>
              </tr>
              <tr v-for="item in activeTableItems" :key="item.id">
                <td class="px-4 py-4">
                  <div class="max-w-[360px] truncate text-sm font-medium text-gray-900 dark:text-gray-100">{{ item.name }}</div>
                  <div class="mt-1 flex max-w-[420px] items-center gap-2 truncate font-mono text-xs text-gray-500 dark:text-dark-400">
                    <Icon name="link" size="xs" />
                    <a :href="item.register_url" target="_blank" rel="noreferrer" class="truncate hover:text-primary-600">{{ item.host }}</a>
                  </div>
                  <div v-if="item.description" class="mt-1 max-w-[420px] truncate text-xs text-gray-400">{{ item.description }}</div>
                  <div v-if="item.source_category" class="mt-2 text-xs text-gray-500 dark:text-dark-400">分类：{{ item.source_category }}</div>
                </td>
                <td class="px-4 py-4">
                  <span class="badge" :class="providerClass(item)">{{ providerLabel(item) }}</span>
                  <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">置信度 {{ percent(item.classification_confidence) }}</div>
                  <div class="mt-1 max-w-[220px] truncate text-xs text-gray-500 dark:text-dark-400">依据 {{ classificationEvidenceLabel(item) }}</div>
                </td>
                <td class="px-4 py-4 text-sm text-gray-600 dark:text-dark-300">
                  <span class="badge" :class="monitorClass(item.monitor_available)">{{ monitorLabel(item.monitor_available) }}</span>
                  <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ monitorSummary(item) }}</div>
                </td>
                <td class="px-4 py-4">
                  <span class="badge" :class="importClass(item.import_status)">{{ importLabel(item.import_status) }}</span>
                  <div v-if="item.supplier_id" class="mt-1 font-mono text-xs text-gray-500">#{{ item.supplier_id }}</div>
                </td>
                <td class="px-4 py-4">
                  <span class="badge" :class="registrationClass(item.registration_status)">{{ registrationLabel(item.registration_status) }}</span>
                  <div v-if="item.registration_status === 'waiting_manual_verification'" class="mt-1 max-w-[280px] text-xs text-amber-600 dark:text-amber-400">
                    请人工完成验证码或邮箱验证后重试；未完成前不会入库供应商。
                  </div>
                  <div v-if="item.registration_task_id" class="mt-1 font-mono text-xs text-gray-500">任务 #{{ item.registration_task_id }}</div>
                  <div v-if="item.registration_email" class="mt-1 max-w-[220px] truncate font-mono text-xs text-gray-500">{{ item.registration_email }}</div>
                  <div v-if="item.registration_error_message" class="mt-1 max-w-[260px] truncate text-xs text-rose-500">{{ item.registration_error_message }}</div>
                </td>
                <td class="px-4 py-4">
                  <span class="badge" :class="processedClass(item)">{{ processedLabel(item) }}</span>
                  <div v-if="item.catalog_site_id" class="mt-1 font-mono text-xs text-gray-500">目录 #{{ item.catalog_site_id }}</div>
                </td>
                <td class="px-4 py-4">
                  <div class="flex justify-end gap-2">
                    <button type="button" class="btn btn-secondary btn-sm" :disabled="!canAddToCatalog(item) || busyItemID === item.id" @click="openCatalogDialog(item)">
                      加入目录
                    </button>
                    <button type="button" class="btn btn-secondary btn-sm" :disabled="!canImport(item) || busyItemID === item.id" @click="importItem(item)">
                      导入
                    </button>
                    <button type="button" class="btn btn-primary btn-sm" :disabled="!canRegister(item) || busyItemID === item.id" @click="registerItem(item)">
                      注册
                    </button>
                    <a :href="item.register_url" target="_blank" rel="noreferrer" class="btn btn-secondary btn-sm">打开</a>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <Pagination
          v-if="activeTablePagination.total > 0"
          :page="activeTablePagination.page"
          :total="activeTablePagination.total"
          :page-size="activeTablePagination.page_size"
          @update:page="handleActivePageChange"
          @update:pageSize="handleActivePageSizeChange"
        />
      </section>

      <section v-else-if="activeTab === 'recommendations'" class="card overflow-hidden">
        <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">低倍率可充值推荐</h2>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">已导入且倍率低于阈值、监控可用的渠道。</p>
        </div>
        <div class="divide-y divide-gray-100 dark:divide-dark-700">
          <div v-if="recommendations.length === 0" class="px-5 py-8 text-center text-sm text-gray-500 dark:text-dark-400">暂无推荐</div>
          <div v-for="recommendation in recommendations" :key="recommendation.item.id" class="grid gap-3 px-5 py-4 md:grid-cols-[minmax(0,1fr)_auto] md:items-center">
            <div class="min-w-0">
              <div class="truncate text-sm font-semibold text-gray-900 dark:text-white">{{ recommendation.item.name }}</div>
              <div class="mt-1 truncate font-mono text-xs text-gray-500 dark:text-dark-400">{{ recommendation.item.host }}</div>
            </div>
            <div class="flex flex-wrap items-center gap-2 text-sm">
              <span class="badge badge-success">倍率 {{ fixedRate(recommendation.min_rate_multiplier) }}</span>
              <span class="badge badge-primary">推荐渠道 {{ recommendation.recommended_channels }}</span>
              <a :href="recommendation.item.dashboard_url || recommendation.item.register_url" target="_blank" rel="noreferrer" class="btn btn-secondary btn-sm">
                打开
              </a>
            </div>
          </div>
        </div>
      </section>

      <section v-else class="grid gap-6 lg:grid-cols-[minmax(0,1fr)_360px]">
        <div class="card p-5">
          <div class="flex items-center justify-between gap-3">
            <div>
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">注册设置</h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">后台统一邮箱，密码由系统随机生成并加密保存。</p>
            </div>
            <button type="button" class="btn btn-primary btn-sm" :disabled="savingSettings" @click="saveSettings">
              {{ savingSettings ? '保存中...' : '保存' }}
            </button>
          </div>
          <div class="mt-4 grid gap-4 xl:grid-cols-[minmax(0,1.2fr)_minmax(0,1fr)_auto] xl:items-end">
            <label class="block">
              <span class="input-label">统一注册邮箱</span>
              <input v-model.trim="settings.registration_email" type="email" class="input" placeholder="ops@example.com" />
            </label>
            <label class="block">
              <span class="input-label">低倍率阈值</span>
              <input v-model.number="settings.low_rate_threshold" type="number" min="0.01" step="0.01" class="input" />
            </label>
            <label class="inline-flex items-center gap-2 pb-2 text-sm text-gray-700 dark:text-dark-200">
              <input v-model="settings.registration_enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600" />
              允许插件注册
            </label>
          </div>
        </div>

        <aside class="card p-5">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">采集参数</h2>
          <div class="mt-4 space-y-4">
            <label class="block">
              <span class="input-label">采集源</span>
              <input v-model.trim="sourceURL" class="input font-mono text-sm" />
            </label>
            <label class="block">
              <span class="input-label">本次上限</span>
              <input v-model.number="runLimit" type="number" min="0" max="1000" class="input" />
            </label>
            <label class="inline-flex items-center gap-2 text-sm text-gray-700 dark:text-dark-200">
              <input v-model="probeInterfaces" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600" />
              接口类型识别
            </label>
            <label class="inline-flex items-center gap-2 text-sm text-gray-700 dark:text-dark-200">
              <input v-model="probeSites" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600" />
              页面深度探测
            </label>
          </div>
        </aside>
      </section>
    </div>

    <div v-if="catalogDialogOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4">
      <div class="w-full max-w-3xl overflow-hidden rounded-lg bg-white shadow-xl dark:bg-dark-900">
        <div class="flex items-center justify-between border-b border-gray-100 px-5 py-4 dark:border-dark-700">
          <div>
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">加入网址目录</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ selectedCatalogCandidate?.host || '-' }}</p>
          </div>
          <button type="button" class="btn btn-secondary btn-sm" :disabled="savingCatalog" @click="closeCatalogDialog">关闭</button>
        </div>
        <div class="max-h-[72vh] overflow-y-auto p-5">
          <div class="grid gap-4 md:grid-cols-2">
            <label class="block">
              <span class="input-label">站点名称</span>
              <input v-model.trim="catalogForm.name" class="input" />
            </label>
            <label class="block">
              <span class="input-label">Slug</span>
              <input v-model.trim="catalogForm.slug" class="input font-mono text-sm" />
            </label>
            <label class="block md:col-span-2">
              <span class="input-label">摘要</span>
              <input v-model.trim="catalogForm.summary" class="input" />
            </label>
            <label class="block md:col-span-2">
              <span class="input-label">描述</span>
              <textarea v-model.trim="catalogForm.description" rows="3" class="input"></textarea>
            </label>
            <label class="block">
              <span class="input-label">站点类型</span>
              <select v-model="catalogForm.site_kind" class="input">
                <option value="api_relay">API 中转</option>
                <option value="official">官方平台</option>
                <option value="tool">工具</option>
                <option value="client">客户端</option>
                <option value="benchmark">评测</option>
                <option value="other">其他</option>
              </select>
            </label>
            <label class="block">
              <span class="input-label">发布状态</span>
              <select v-model="catalogForm.status" class="input">
                <option value="draft">草稿</option>
                <option value="reviewing">待审核</option>
                <option value="published">已发布</option>
              </select>
            </label>
            <label class="block">
              <span class="input-label">可见性</span>
              <select v-model="catalogForm.visibility" class="input">
                <option value="public">公开</option>
                <option value="private">私有</option>
              </select>
            </label>
            <label class="block">
              <span class="input-label">推荐级别</span>
              <select v-model="catalogForm.recommendation_level" class="input">
                <option value="none">不推荐</option>
                <option value="normal">普通</option>
                <option value="featured">重点推荐</option>
                <option value="avoid">避坑</option>
              </select>
            </label>
            <label class="block">
              <span class="input-label">注册链接</span>
              <input v-model.trim="catalogForm.register_url" class="input font-mono text-sm" />
            </label>
            <label class="block">
              <span class="input-label">仪表盘链接</span>
              <input v-model.trim="catalogForm.dashboard_url" class="input font-mono text-sm" />
            </label>
            <label class="block md:col-span-2">
              <span class="input-label">API Base</span>
              <input v-model.trim="catalogForm.api_base_url" class="input font-mono text-sm" />
            </label>
          </div>

          <div class="mt-5 grid gap-5 md:grid-cols-2">
            <div>
              <div class="text-sm font-medium text-gray-900 dark:text-white">分类</div>
              <div class="mt-2 max-h-36 overflow-y-auto rounded-md border border-gray-100 p-3 dark:border-dark-700">
                <div v-if="catalogCategories.length === 0" class="text-sm text-gray-500 dark:text-dark-400">{{ catalogLookupsLoading ? '加载中...' : '暂无分类' }}</div>
                <label v-for="category in catalogCategories" :key="category.id" class="mb-2 flex items-center gap-2 text-sm text-gray-700 last:mb-0 dark:text-dark-200">
                  <input v-model="catalogForm.category_ids" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600" :value="category.id" />
                  {{ category.name }}
                </label>
              </div>
            </div>
            <div>
              <div class="text-sm font-medium text-gray-900 dark:text-white">标签</div>
              <div class="mt-2 max-h-36 overflow-y-auto rounded-md border border-gray-100 p-3 dark:border-dark-700">
                <div v-if="catalogTags.length === 0" class="text-sm text-gray-500 dark:text-dark-400">{{ catalogLookupsLoading ? '加载中...' : '暂无标签' }}</div>
                <label v-for="tag in catalogTags" :key="tag.id" class="mb-2 flex items-center gap-2 text-sm text-gray-700 last:mb-0 dark:text-dark-200">
                  <input v-model="catalogForm.tag_ids" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600" :value="tag.id" />
                  {{ tag.name }}
                </label>
              </div>
            </div>
          </div>
        </div>
        <div class="flex justify-end gap-2 border-t border-gray-100 px-5 py-4 dark:border-dark-700">
          <button type="button" class="btn btn-secondary" :disabled="savingCatalog" @click="closeCatalogDialog">取消</button>
          <button type="button" class="btn btn-primary" :disabled="savingCatalog || !catalogForm.name || !catalogForm.slug" @click="submitCatalogDialog">
            {{ savingCatalog ? '保存中...' : '创建目录站点' }}
          </button>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, reactive, ref, watch } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import Pagination from '@/components/common/Pagination.vue'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { useAppStore } from '@/stores/app'
import {
  getSiteDiscoverySettings,
  addDiscoveryCandidateToCatalog,
  bulkAddDiscoveryCandidatesToCatalogStream,
  classifySiteDiscoveryItemsStream,
  importSiteDiscoveryItem,
  listSiteCatalogCategories,
  listSiteCatalogTags,
  listSiteDiscoveryItems,
  listSiteDiscoveryRecommendations,
  registerSiteDiscoveryItem,
  runSiteDiscoveryStream,
  updateSiteDiscoverySettings,
  type AddDiscoveryCandidateToCatalogPayload,
  type SiteCatalogCategory,
  type SiteCatalogKind,
  type SiteCatalogRecommendationLevel,
  type SiteCatalogStatus,
  type SiteCatalogTag,
  type SiteCatalogVisibility,
  type SiteDiscoveryItem,
  type SiteDiscoveryRecommendation,
  type SiteDiscoveryRunProgressEvent,
  type SiteDiscoveryRunProgressLevel,
  type SiteDiscoveryRunResult,
  type SiteDiscoverySettings
} from '@/api/admin/adminPlus'

type SiteDiscoveryTab = 'dashboard' | 'urls' | 'registered' | 'tasks' | 'recommendations' | 'settings'
type SiteDiscoveryPagination = {
  page: number
  page_size: number
  total: number
  pages: number
}
type DiscoveryLogEntry = {
  id: number
  level: SiteDiscoveryRunProgressLevel
  message: string
  current?: number
  total?: number
}
type CatalogForm = {
  name: string
  slug: string
  summary: string
  description: string
  site_kind: SiteCatalogKind
  status: SiteCatalogStatus
  visibility: SiteCatalogVisibility
  recommendation_level: SiteCatalogRecommendationLevel
  risk_level: 'unknown' | 'low' | 'medium' | 'high'
  category_ids: number[]
  tag_ids: number[]
  register_url: string
  dashboard_url: string
  api_base_url: string
}

const appStore = useAppStore()

const tabs: { value: SiteDiscoveryTab; label: string }[] = [
  { value: 'dashboard', label: '工作台' },
  { value: 'urls', label: '采集网址' },
  { value: 'registered', label: '注册列表' },
  { value: 'tasks', label: '注册任务' },
  { value: 'recommendations', label: '低倍率推荐' },
  { value: 'settings', label: '设置' }
]

const activeTab = ref<SiteDiscoveryTab>('dashboard')
const loading = ref(false)
const running = ref(false)
const classifying = ref(false)
const bulkAddingCatalog = ref(false)
const savingSettings = ref(false)
const busyItemID = ref<number | null>(null)
const sourceURL = ref('https://api.daheiai.com/')
const probeInterfaces = ref(true)
const probeSites = ref(false)
const runLimit = ref(0)
const items = ref<SiteDiscoveryItem[]>([])
const registeredItems = ref<SiteDiscoveryItem[]>([])
const registrationTaskItems = ref<SiteDiscoveryItem[]>([])
const recommendations = ref<SiteDiscoveryRecommendation[]>([])
const logContainerRef = ref<HTMLElement | null>(null)
const discoveryLogs = ref<DiscoveryLogEntry[]>([])
const discoveryProgress = reactive({
  current: 0,
  total: 0
})
const catalogDialogOpen = ref(false)
const savingCatalog = ref(false)
const catalogLookupsLoading = ref(false)
const selectedCatalogCandidate = ref<SiteDiscoveryItem | null>(null)
const catalogCategories = ref<SiteCatalogCategory[]>([])
const catalogTags = ref<SiteCatalogTag[]>([])
let discoveryLogID = 0

const settings = reactive<SiteDiscoverySettings>({
  registration_email: '',
  registration_enabled: false,
  low_rate_threshold: 0.8
})

const catalogForm = reactive<CatalogForm>({
  name: '',
  slug: '',
  summary: '',
  description: '',
  site_kind: 'api_relay',
  status: 'draft',
  visibility: 'public',
  recommendation_level: 'none',
  risk_level: 'unknown',
  category_ids: [],
  tag_ids: [],
  register_url: '',
  dashboard_url: '',
  api_base_url: ''
})

const urlFilters = reactive({
  q: '',
  provider_type: '',
  classification_status: '',
  import_status: '',
  registration_status: '',
  processed_status: 'unprocessed' as 'processed' | 'unprocessed' | ''
})

const registeredFilters = reactive({
  q: '',
  provider_type: ''
})

const taskFilters = reactive({
  q: '',
  provider_type: '',
  registration_status: ''
})

const urlPagination = reactive<SiteDiscoveryPagination>(defaultPagination())
const registeredPagination = reactive<SiteDiscoveryPagination>(defaultPagination())
const taskPagination = reactive<SiteDiscoveryPagination>(defaultPagination())

const isTableTab = computed(() => ['urls', 'registered', 'tasks'].includes(activeTab.value))
const supportedCount = computed(() => items.value.filter((item) => item.classification_status === 'supported').length)
const manualCount = computed(() => registrationTaskItems.value.filter((item) => item.registration_status === 'waiting_manual_verification').length)
const discoveryProgressPercent = computed(() => {
  if (!discoveryProgress.total) return running.value ? 2 : 0
  return Math.min(100, Math.round((discoveryProgress.current / discoveryProgress.total) * 100))
})
const discoveryProgressLabel = computed(() => {
  if (bulkAddingCatalog.value) return '正在把已识别候选批量加入网址目录。'
  if (classifying.value) return '正在通过接口批量判断 new-api / sub2api 类型。'
  if (running.value) return '正在采集、去重、分类并写入候选库。'
  if (discoveryLogs.value.length > 0) return '最近一次采集或识别日志。'
  return '启动采集或一键识别后显示实时进度和日志。'
})

const activeTableItems = computed(() => {
  if (activeTab.value === 'registered') return registeredItems.value
  if (activeTab.value === 'tasks') return registrationTaskItems.value
  return items.value
})

const activeTablePagination = computed(() => {
  if (activeTab.value === 'registered') return registeredPagination
  if (activeTab.value === 'tasks') return taskPagination
  return urlPagination
})

const activeTableTitle = computed(() => {
  if (activeTab.value === 'registered') return '已注册列表'
  if (activeTab.value === 'tasks') return '注册任务列表'
  return '采集网址列表'
})

const activeTableDescription = computed(() => {
  if (activeTab.value === 'registered') return '插件已完成提交的注册记录。'
  if (activeTab.value === 'tasks') return '排队、执行中、待人工验证和失败的注册记录。'
  return '从 daheiai 第三方中转分区采集到的网址，默认仅显示未处理项。'
})

const activeEmptyLabel = computed(() => {
  if (activeTab.value === 'registered') return '暂无已注册记录'
  if (activeTab.value === 'tasks') return '暂无注册任务'
  return '暂无采集网址'
})

watch(activeTab, (tab) => {
  if (tab === 'urls') void loadItems()
  if (tab === 'registered') void loadRegisteredItems()
  if (tab === 'tasks') void loadRegistrationTasks()
  if (tab === 'recommendations') void loadRecommendations()
  if (tab === 'settings') void loadSettings()
})

onMounted(() => {
  void loadPage()
})

function defaultPagination(): SiteDiscoveryPagination {
  return {
    page: 1,
    page_size: getPersistedPageSize(),
    total: 0,
    pages: 0
  }
}

async function loadPage() {
  loading.value = true
  try {
    await Promise.all([loadSettings(), loadItems(), loadRegisteredItems(), loadRegistrationTasks(), loadRecommendations()])
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    loading.value = false
  }
}

async function loadSettings() {
  const next = await getSiteDiscoverySettings()
  Object.assign(settings, next)
}

async function loadItems() {
  const result = await listSiteDiscoveryItems({
    page: urlPagination.page,
    page_size: urlPagination.page_size,
    q: urlFilters.q || undefined,
    provider_type: normalizeEmpty(urlFilters.provider_type) as 'new_api' | 'sub2api' | '',
    classification_status: normalizeEmpty(urlFilters.classification_status) as never,
    import_status: normalizeEmpty(urlFilters.import_status) as never,
    registration_status: normalizeEmpty(urlFilters.registration_status) as never,
    processed_status: urlFilters.processed_status
  })
  items.value = result.items
  applyPagination(urlPagination, result)
}

async function loadRegisteredItems() {
  const result = await listSiteDiscoveryItems({
    page: registeredPagination.page,
    page_size: registeredPagination.page_size,
    q: registeredFilters.q || undefined,
    provider_type: normalizeEmpty(registeredFilters.provider_type) as 'new_api' | 'sub2api' | '',
    registration_status: 'succeeded'
  })
  registeredItems.value = result.items
  applyPagination(registeredPagination, result)
}

async function loadRegistrationTasks() {
  if (taskFilters.registration_status) {
    const result = await listSiteDiscoveryItems({
      page: taskPagination.page,
      page_size: taskPagination.page_size,
      q: taskFilters.q || undefined,
      provider_type: normalizeEmpty(taskFilters.provider_type) as 'new_api' | 'sub2api' | '',
      registration_status: taskFilters.registration_status as never
    })
    registrationTaskItems.value = result.items
    applyPagination(taskPagination, result)
    return
  }

  const result = await listSiteDiscoveryItems({
    page: 1,
    page_size: 1000,
    q: taskFilters.q || undefined,
    provider_type: normalizeEmpty(taskFilters.provider_type) as 'new_api' | 'sub2api' | ''
  })
  const filtered = result.items.filter((item) => Boolean(item.registration_status))
  taskPagination.total = filtered.length
  taskPagination.pages = Math.ceil(filtered.length / taskPagination.page_size)
  const start = (taskPagination.page - 1) * taskPagination.page_size
  registrationTaskItems.value = filtered.slice(start, start + taskPagination.page_size)
}

async function loadRecommendations() {
  const result = await listSiteDiscoveryRecommendations({ limit: 20 })
  recommendations.value = result.items
}

async function saveSettings() {
  savingSettings.value = true
  try {
    const next = await updateSiteDiscoverySettings({ ...settings })
    Object.assign(settings, next)
    appStore.showSuccess('设置已保存')
    await loadRecommendations()
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    savingSettings.value = false
  }
}

async function runDiscoveryNow() {
  running.value = true
  resetDiscoveryProgress()
  const completedResult: { value: SiteDiscoveryRunResult | null } = { value: null }
  let failedMessage = ''
  try {
    await runSiteDiscoveryStream({
      source_url: sourceURL.value,
      probe_interfaces: probeInterfaces.value,
      probe_sites: probeSites.value,
      limit: runLimit.value > 0 ? runLimit.value : undefined
    }, (event) => {
      handleDiscoveryProgressEvent(event)
      if (event.result) completedResult.value = event.result
      if (event.type === 'failed') failedMessage = event.message
    })
    if (failedMessage) throw new Error(failedMessage)
    const run = (completedResult.value as SiteDiscoveryRunResult | null)?.run
    appStore.showSuccess(run ? `采集完成：${run.total} 个站点，支持 ${run.supported_total} 个` : '采集完成')
    urlPagination.page = 1
    await Promise.all([loadItems(), loadRegisteredItems(), loadRegistrationTasks(), loadRecommendations()])
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    running.value = false
  }
}

async function classifyAllItemsNow() {
  classifying.value = true
  resetDiscoveryProgress()
  let failedMessage = ''
  try {
    await classifySiteDiscoveryItemsStream({
      probe_interfaces: true,
      probe_sites: probeSites.value
    }, (event) => {
      handleDiscoveryProgressEvent(event)
      if (event.type === 'failed') failedMessage = event.message
    })
    if (failedMessage) throw new Error(failedMessage)
    appStore.showSuccess('批量识别完成')
    urlPagination.page = 1
    await Promise.all([loadItems(), loadRegisteredItems(), loadRegistrationTasks(), loadRecommendations()])
    activeTab.value = 'urls'
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    classifying.value = false
  }
}

async function bulkAddCatalogNow() {
  bulkAddingCatalog.value = true
  resetDiscoveryProgress()
  let failedMessage = ''
  try {
    await bulkAddDiscoveryCandidatesToCatalogStream({
      q: urlFilters.q || undefined,
      provider_type: normalizeEmpty(urlFilters.provider_type) as 'new_api' | 'sub2api' | '',
      import_status: normalizeEmpty(urlFilters.import_status) as never,
      registration_status: normalizeEmpty(urlFilters.registration_status) as never,
      processed_status: urlFilters.processed_status || 'unprocessed',
      only_supported: true,
      limit: 1000,
      site_kind: 'api_relay',
      status: 'draft',
      visibility: 'public',
      recommendation_level: 'none',
      risk_level: 'unknown'
    }, (event) => {
      handleBulkAddCatalogProgressEvent(event)
      if (event.type === 'failed') failedMessage = event.message
    })
    if (failedMessage) throw new Error(failedMessage)
    appStore.showSuccess('批量加入目录完成')
    urlPagination.page = 1
    activeTab.value = 'urls'
    await refreshActiveLists()
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    bulkAddingCatalog.value = false
  }
}

function resetDiscoveryProgress() {
  discoveryProgress.current = 0
  discoveryProgress.total = 0
  discoveryLogs.value = []
}

function handleDiscoveryProgressEvent(event: SiteDiscoveryRunProgressEvent) {
  if (typeof event.total === 'number') discoveryProgress.total = event.total
  if (typeof event.current === 'number') discoveryProgress.current = event.current
  if (event.run?.total) discoveryProgress.total = event.run.total
  if (event.result?.run.total) {
    discoveryProgress.total = event.result.run.total
    discoveryProgress.current = event.result.run.total
  }
  if (event.classify_result?.total) {
    discoveryProgress.total = event.classify_result.total
    discoveryProgress.current = event.classify_result.total
  }
  appendDiscoveryLog({
    id: ++discoveryLogID,
    level: event.level || discoveryLevelFromType(event.type),
    message: event.message || event.type,
    current: event.current,
    total: event.total
  })
}

function handleBulkAddCatalogProgressEvent(event: { type: string; level?: SiteDiscoveryRunProgressLevel; message: string; current?: number; total?: number; result?: { total: number } }) {
  if (typeof event.total === 'number') discoveryProgress.total = event.total
  if (typeof event.current === 'number') discoveryProgress.current = event.current
  if (event.result?.total) {
    discoveryProgress.total = event.result.total
    discoveryProgress.current = event.result.total
  }
  appendDiscoveryLog({
    id: ++discoveryLogID,
    level: event.level || discoveryLevelFromType(event.type),
    message: event.message || event.type,
    current: event.current,
    total: event.total
  })
}

function discoveryLevelFromType(type: string): SiteDiscoveryRunProgressLevel {
  if (type === 'item_success' || type === 'completed') return 'success'
  if (type === 'item_skipped' || type === 'item_unknown') return 'warning'
  if (type === 'failed' || type === 'item_failed') return 'error'
  return 'info'
}

function appendDiscoveryLog(entry: DiscoveryLogEntry) {
  discoveryLogs.value = [...discoveryLogs.value.slice(-299), entry]
  void nextTick(() => {
    const el = logContainerRef.value
    if (el) el.scrollTop = el.scrollHeight
  })
}

function discoveryLogClass(level: SiteDiscoveryRunProgressLevel): string {
  if (level === 'success') return 'text-emerald-600 dark:text-emerald-400'
  if (level === 'warning') return 'text-amber-600 dark:text-amber-400'
  if (level === 'error') return 'text-rose-600 dark:text-rose-400'
  return 'text-gray-600 dark:text-dark-300'
}

async function importItem(item: SiteDiscoveryItem) {
  busyItemID.value = item.id
  try {
    await importSiteDiscoveryItem(item.id)
    appStore.showSuccess('供应商已导入')
    await refreshActiveLists()
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    busyItemID.value = null
  }
}

async function registerItem(item: SiteDiscoveryItem) {
  busyItemID.value = item.id
  try {
    await registerSiteDiscoveryItem(item.id)
    appStore.showSuccess('注册任务已排队；注册成功并取得凭据后才会入库供应商')
    await refreshActiveLists()
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    busyItemID.value = null
  }
}

async function openCatalogDialog(item: SiteDiscoveryItem) {
  selectedCatalogCandidate.value = item
  resetCatalogForm(item)
  catalogDialogOpen.value = true
  try {
    await ensureCatalogLookups()
    preselectCatalogCategory()
    catalogForm.tag_ids = suggestedTagIDs(item)
  } catch (error) {
    appStore.showError(errorMessage(error))
  }
}

function closeCatalogDialog() {
  if (savingCatalog.value) return
  catalogDialogOpen.value = false
  selectedCatalogCandidate.value = null
}

async function ensureCatalogLookups() {
  if (catalogCategories.value.length > 0 || catalogTags.value.length > 0) return
  catalogLookupsLoading.value = true
  try {
    const [categories, tags] = await Promise.all([listSiteCatalogCategories(), listSiteCatalogTags()])
    catalogCategories.value = categories.items
    catalogTags.value = tags.items
  } finally {
    catalogLookupsLoading.value = false
  }
}

function resetCatalogForm(item: SiteDiscoveryItem) {
  catalogForm.name = item.name || item.host || `site-${item.id}`
  catalogForm.slug = slugFromDiscoveryItem(item)
  catalogForm.summary = truncateText(item.description || item.name || item.host || '', 120)
  catalogForm.description = item.description || ''
  catalogForm.site_kind = 'api_relay'
  catalogForm.status = 'draft'
  catalogForm.visibility = 'public'
  catalogForm.recommendation_level = 'none'
  catalogForm.risk_level = 'unknown'
  catalogForm.category_ids = []
  catalogForm.tag_ids = suggestedTagIDs(item)
  catalogForm.register_url = item.register_url || ''
  catalogForm.dashboard_url = item.dashboard_url || item.register_url || ''
  catalogForm.api_base_url = item.api_base_url || ''
}

function preselectCatalogCategory() {
  if (catalogForm.category_ids.length > 0) return
  const matched = catalogCategories.value.find((category) => {
    const value = `${category.slug} ${category.name}`.toLowerCase()
    return value.includes('third') || value.includes('relay') || value.includes('中转') || value.includes('第三方')
  })
  if (matched) catalogForm.category_ids = [matched.id]
}

function suggestedTagIDs(item: SiteDiscoveryItem): number[] {
  const text = `${item.provider_type || ''} ${item.description || ''} ${item.source_category || ''}`.toLowerCase()
  return catalogTags.value.filter((tag) => {
    const value = `${tag.slug} ${tag.name}`.toLowerCase()
    return text.includes(value) || (item.provider_type === 'new_api' && value.includes('new')) || (item.provider_type === 'sub2api' && value.includes('sub2api'))
  }).map((tag) => tag.id)
}

async function submitCatalogDialog() {
  const candidate = selectedCatalogCandidate.value
  if (!candidate) return
  savingCatalog.value = true
  busyItemID.value = candidate.id
  try {
    const payload: AddDiscoveryCandidateToCatalogPayload = {
      name: catalogForm.name,
      slug: catalogForm.slug,
      summary: catalogForm.summary,
      description: catalogForm.description,
      site_kind: catalogForm.site_kind,
      status: catalogForm.status,
      visibility: catalogForm.visibility,
      recommendation_level: catalogForm.recommendation_level,
      risk_level: catalogForm.risk_level,
      category_ids: catalogForm.category_ids,
      tag_ids: catalogForm.tag_ids,
      links: catalogLinksFromForm()
    }
    await addDiscoveryCandidateToCatalog(candidate.id, payload)
    appStore.showSuccess('已加入网址目录')
    catalogDialogOpen.value = false
    selectedCatalogCandidate.value = null
    await refreshActiveLists()
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    savingCatalog.value = false
    busyItemID.value = null
  }
}

function catalogLinksFromForm(): AddDiscoveryCandidateToCatalogPayload['links'] {
  const links: NonNullable<AddDiscoveryCandidateToCatalogPayload['links']> = []
  if (catalogForm.register_url) links.push({ link_type: 'register', url: catalogForm.register_url, label: '注册', is_primary: true })
  if (catalogForm.dashboard_url && catalogForm.dashboard_url !== catalogForm.register_url) links.push({ link_type: 'dashboard', url: catalogForm.dashboard_url, label: '控制台' })
  if (catalogForm.api_base_url) links.push({ link_type: 'api_base', url: catalogForm.api_base_url, label: 'API Base' })
  return links
}

async function refreshActiveLists() {
  await Promise.all([loadItems(), loadRegisteredItems(), loadRegistrationTasks(), loadRecommendations()])
}

function resetURLPagination() {
  urlPagination.page = 1
  void loadItems()
}

function setProcessedFilter(value: 'processed' | 'unprocessed' | '') {
  urlFilters.processed_status = value
  resetURLPagination()
}

function setProviderFilter(value: 'new_api' | 'sub2api') {
  urlFilters.provider_type = value
  urlFilters.classification_status = ''
  resetURLPagination()
}

function setClassificationFilter(value: 'unknown') {
  urlFilters.provider_type = ''
  urlFilters.classification_status = value
  resetURLPagination()
}

function clearTypeFilters() {
  urlFilters.provider_type = ''
  urlFilters.classification_status = ''
  resetURLPagination()
}

function resetRegisteredPagination() {
  registeredPagination.page = 1
  void loadRegisteredItems()
}

function resetTaskPagination() {
  taskPagination.page = 1
  void loadRegistrationTasks()
}

function handleActivePageChange(page: number) {
  if (activeTab.value === 'registered') {
    registeredPagination.page = page
    void loadRegisteredItems()
    return
  }
  if (activeTab.value === 'tasks') {
    taskPagination.page = page
    void loadRegistrationTasks()
    return
  }
  urlPagination.page = page
  void loadItems()
}

function handleActivePageSizeChange(pageSize: number) {
  if (activeTab.value === 'registered') {
    registeredPagination.page_size = pageSize
    registeredPagination.page = 1
    void loadRegisteredItems()
    return
  }
  if (activeTab.value === 'tasks') {
    taskPagination.page_size = pageSize
    taskPagination.page = 1
    void loadRegistrationTasks()
    return
  }
  urlPagination.page_size = pageSize
  urlPagination.page = 1
  void loadItems()
}

function applyPagination(target: SiteDiscoveryPagination, result: { total: number; page?: number; page_size?: number; pages?: number }) {
  target.total = result.total
  target.page = result.page || target.page
  target.page_size = result.page_size || target.page_size
  target.pages = result.pages || 0
}

function canImport(item: SiteDiscoveryItem): boolean {
  return item.classification_status === 'supported' && item.import_status !== 'imported' && ['new_api', 'sub2api'].includes(item.provider_type || '')
}

function canAddToCatalog(item: SiteDiscoveryItem): boolean {
  return item.process_status !== 'added_to_catalog' && !item.catalog_site_id
}

function canRegister(item: SiteDiscoveryItem): boolean {
  return settings.registration_enabled && canImportOrImported(item) && !['queued', 'running', 'succeeded'].includes(item.registration_status || '')
}

function canImportOrImported(item: SiteDiscoveryItem): boolean {
  return item.classification_status === 'supported' && ['new_api', 'sub2api'].includes(item.provider_type || '')
}

function providerLabel(item: SiteDiscoveryItem): string {
  if (item.provider_type === 'new_api') return 'new-api'
  if (item.provider_type === 'sub2api') return 'sub2api'
  return item.classification_status === 'supported' ? '支持' : '未知'
}

function providerClass(item: SiteDiscoveryItem): string {
  if (item.provider_type === 'new_api') return 'badge-primary'
  if (item.provider_type === 'sub2api') return 'badge-purple'
  return classificationClass(item.classification_status)
}

function classificationClass(status: SiteDiscoveryItem['classification_status']): string {
  if (status === 'supported') return 'badge-success'
  if (status === 'unsupported') return 'badge-danger'
  return 'badge-gray'
}

function classificationEvidenceLabel(item: SiteDiscoveryItem): string {
  const evidence = item.classification_evidence || []
  if (evidence.some((value) => value.startsWith('api:'))) return '接口二次分类'
  if (evidence.some((value) => value.startsWith('site:'))) return '页面深度探测'
  if (evidence.some((value) => value.startsWith('source:'))) return '索引特征'
  return '-'
}

function importLabel(status: SiteDiscoveryItem['import_status']): string {
  return { new: '未导入', imported: '已导入', skipped: '已跳过' }[status] || status
}

function importClass(status: SiteDiscoveryItem['import_status']): string {
  if (status === 'imported') return 'badge-success'
  if (status === 'skipped') return 'badge-warning'
  return 'badge-gray'
}

function registrationLabel(status?: SiteDiscoveryItem['registration_status']): string {
  return {
    pending: '待处理',
    queued: '已排队',
    running: '执行中',
    waiting_manual_verification: '待人工验证',
    succeeded: '成功',
    failed: '失败',
    '': '未注册'
  }[status || ''] || status || '未注册'
}

function registrationClass(status?: SiteDiscoveryItem['registration_status']): string {
  if (status === 'succeeded') return 'badge-success'
  if (status === 'failed') return 'badge-danger'
  if (status === 'waiting_manual_verification' || status === 'queued' || status === 'running') return 'badge-warning'
  return 'badge-gray'
}

function processedLabel(item: SiteDiscoveryItem): string {
  if (item.process_status === 'added_to_catalog') return '已入目录'
  if (item.process_status === 'registered') return '已注册'
  if (item.process_status === 'ignored') return '已忽略'
  return isProcessed(item) ? '已处理' : '未处理'
}

function processedClass(item: SiteDiscoveryItem): string {
  return isProcessed(item) ? 'badge-success' : 'badge-warning'
}

function isProcessed(item: SiteDiscoveryItem): boolean {
  return Boolean(item.process_status && item.process_status !== 'unprocessed') || item.import_status !== 'new' || Boolean(item.registration_status)
}

function processedFilterClass(value: 'processed' | 'unprocessed' | ''): string {
  return urlFilters.processed_status === value ? 'btn-primary' : 'btn-secondary'
}

function providerFilterClass(value: 'new_api' | 'sub2api' | ''): string {
  return urlFilters.provider_type === value && (!value || !urlFilters.classification_status) ? 'btn-primary' : 'btn-secondary'
}

function classificationFilterClass(value: 'unknown'): string {
  return urlFilters.classification_status === value && !urlFilters.provider_type ? 'btn-primary' : 'btn-secondary'
}

function monitorLabel(value?: boolean | null): string {
  if (value === true) return '在线'
  if (value === false) return '离线'
  return '未监控'
}

function monitorClass(value?: boolean | null): string {
  if (value === true) return 'badge-success'
  if (value === false) return 'badge-danger'
  return 'badge-gray'
}

function monitorSummary(item: SiteDiscoveryItem): string {
  const parts: string[] = []
  if (typeof item.monitor_uptime_percent === 'number') parts.push(`可用率 ${item.monitor_uptime_percent.toFixed(1)}%`)
  if (typeof item.monitor_latest_response_ms === 'number') parts.push(`${item.monitor_latest_response_ms}ms`)
  return parts.join(' / ') || '-'
}

function percent(value: number): string {
  return `${Math.round((value || 0) * 100)}%`
}

function fixedRate(value: number): string {
  return Number.isFinite(value) ? value.toFixed(3) : '-'
}

function slugFromDiscoveryItem(item: SiteDiscoveryItem): string {
  const value = item.host || safeHostname(item.register_url) || item.name || `site-${item.id}`
  return value.toLowerCase().replace(/^www\./, '').replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '') || `site-${item.id}`
}

function safeHostname(value?: string): string {
  if (!value) return ''
  try {
    return new URL(value).hostname
  } catch {
    return ''
  }
}

function truncateText(value: string, max: number): string {
  return value.length > max ? `${value.slice(0, max - 1)}...` : value
}

function normalizeEmpty(value: string): string | undefined {
  return value.trim() || undefined
}

function errorMessage(error: unknown): string {
  if (error instanceof Error) return error.message
  if (error && typeof error === 'object') {
    const data = error as { message?: unknown; reason?: unknown }
    const message = typeof data.message === 'string' ? data.message.trim() : ''
    const reason = typeof data.reason === 'string' ? data.reason.trim() : ''
    if (message && reason) return `${message}（${reason}）`
    return message || reason || '操作失败'
  }
  return '操作失败'
}
</script>
