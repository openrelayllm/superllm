<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap-reverse items-start justify-between gap-3">
          <div class="grid flex-1 gap-3 lg:grid-cols-[minmax(220px,1fr)_160px_160px_160px_160px]">
            <label class="block">
              <span class="input-label">搜索</span>
              <div class="relative">
                <Icon name="search" size="sm" class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
                <input v-model.trim="filters.q" class="input pl-9" placeholder="供应商名称、联系人、备注" />
              </div>
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
            <div class="min-w-[220px]">
              <div class="flex items-center gap-2">
                <span class="font-medium text-gray-900 dark:text-white">{{ row.name }}</span>
              </div>
              <div class="mt-1 flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-dark-400">
                <span class="font-mono">#{{ row.id }}</span>
                <span v-if="row.contact">{{ row.contact }}</span>
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
            <div class="flex min-w-[170px] flex-col gap-1.5">
              <div class="flex flex-wrap gap-1.5">
                <span class="badge w-fit" :class="runtimeClass(row.runtime_status)">{{ runtimeLabel(row.runtime_status) }}</span>
                <span class="badge w-fit" :class="healthClass(row.health_status)">{{ healthLabel(row.health_status) }}</span>
              </div>
              <span class="text-xs font-medium" :class="supplierSwitchStateClass(row)">
                {{ supplierSwitchStateLabel(row) }}
              </span>
            </div>
          </template>

          <template #cell-balance="{ row }">
            <div class="min-w-[170px] text-right">
              <div class="text-base font-semibold" :class="supplierBalanceAmountClass(row)">
                {{ formatMoney(row.balance_cents, row.balance_currency) }}
              </div>
              <div class="mt-1 flex justify-end">
                <span class="badge" :class="supplierBalanceBadgeClass(row)">
                  {{ supplierBalanceLabel(row) }}
                </span>
              </div>
              <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ formatDateTime(row.balance_updated_at) }}</div>
            </div>
          </template>

          <template #cell-credential="{ row }">
            <div class="flex min-w-[180px] flex-wrap gap-1">
              <span v-if="row.credential.browser_login_enabled" class="badge badge-warning">Chrome</span>
              <span v-if="row.credential.browser_login_username_configured" class="badge badge-gray">
                {{ row.credential.masked_browser_login_username || '账号' }}
              </span>
              <span v-if="row.credential.browser_login_password_configured" class="badge badge-success">密码</span>
              <span v-if="row.credential.browser_login_token_configured" class="badge badge-primary">Token</span>
              <span v-if="row.credential.postgres_configured" class="badge badge-purple">PG</span>
              <span v-if="row.credential.redis_configured" class="badge badge-primary">Redis</span>
              <span v-if="!hasCredential(row)" class="badge badge-gray">未配置</span>
            </div>
          </template>

          <template #cell-session="{ row }">
            <div class="flex min-w-[150px] flex-col items-start gap-1">
              <span class="badge" :class="sessionBadgeClass(row.id)">{{ sessionBadgeText(row.id) }}</span>
              <span v-if="sessionStore[row.id]?.captured_at" class="text-xs text-gray-500 dark:text-dark-400">
                {{ formatDateTime(sessionStore[row.id]?.captured_at) }}
              </span>
            </div>
          </template>

          <template #cell-address="{ row }">
            <div class="max-w-[300px] text-xs text-gray-500 dark:text-dark-400">
              <a v-if="row.dashboard_url" :href="row.dashboard_url" target="_blank" rel="noreferrer" class="block truncate text-primary-600 hover:underline dark:text-primary-400">
                {{ row.dashboard_url }}
              </a>
              <span v-else class="block">后台地址未配置</span>
              <div class="truncate">{{ row.api_base_url || 'API Base URL 未配置' }}</div>
            </div>
          </template>

          <template #cell-created_at="{ row }">
            <div class="min-w-[150px] text-xs text-gray-500 dark:text-dark-400">{{ formatDateTime(row.created_at) }}</div>
          </template>

          <template #cell-actions="{ row }">
            <div class="flex min-w-[330px] justify-end gap-2">
              <button type="button" class="btn btn-secondary btn-sm" title="编辑" @click="openEditDialog(row)">
                <Icon name="edit" size="sm" />
                编辑
              </button>
              <button type="button" class="btn btn-secondary btn-sm" title="状态" @click="openStatusDialog(row)">
                <Icon name="checkCircle" size="sm" />
                状态
              </button>
              <button type="button" class="btn btn-secondary btn-sm" title="供应商会话" @click="openSessionDialog(row)">
                <Icon name="shield" size="sm" />
                会话
              </button>
              <button type="button" class="btn btn-secondary btn-sm" title="供应商分组" @click="openGroupsDialog(row)">
                <Icon name="database" size="sm" />
                分组
              </button>
              <button type="button" class="btn btn-danger btn-sm" title="删除" @click="openDeleteDialog(row)">
                <Icon name="trash" size="sm" />
              </button>
            </div>
          </template>

          <template #empty>
            <EmptyState
              title="暂无供应商"
              description="先添加供应商父级，再通过插件上报会话，同步分组后按分组开通 Key 和本地账号。"
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

    <BaseDialog :show="editorOpen" :title="editingSupplier ? '编辑供应商' : '添加供应商'" width="wide" @close="closeEditor">
      <form id="supplier-editor-form" class="space-y-5" @submit.prevent="submitSupplier">
        <div class="grid gap-4 sm:grid-cols-2">
          <label class="block">
            <span class="input-label">名称</span>
            <input v-model.trim="form.name" class="input" required placeholder="supplier-a" />
          </label>
          <label class="block">
            <span class="input-label">联系人</span>
            <input v-model.trim="form.contact" class="input" placeholder="ops@example.com" />
          </label>
        </div>

        <div class="grid gap-4 sm:grid-cols-3">
          <label class="block">
            <span class="input-label">供应商归类</span>
            <select v-model="form.kind" class="input">
              <option value="relay">下游中转</option>
              <option value="source_account">源站账号归类</option>
              <option value="browser_only">仅浏览器采集</option>
              <option value="custom">自定义</option>
            </select>
          </label>
          <label class="block">
            <span class="input-label">系统类型</span>
            <select v-model="form.type" class="input">
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
            <select v-model="form.runtime_status" class="input">
              <option value="monitor_only">仅监控</option>
              <option value="candidate">候选</option>
              <option value="active">当前使用</option>
              <option value="disabled">停用</option>
            </select>
          </label>
        </div>

        <div class="grid gap-4 sm:grid-cols-3">
          <label class="block">
            <span class="input-label">健康状态</span>
            <select v-model="form.health_status" class="input">
              <option value="normal">正常</option>
              <option value="unavailable">不可用</option>
              <option value="credential_invalid">凭据失效</option>
              <option value="paused">暂停</option>
            </select>
          </label>
          <label class="block">
            <span class="input-label">余额</span>
            <input v-model.number="form.balance_yuan" type="number" min="0" step="0.01" class="input" />
          </label>
          <label class="block">
            <span class="input-label">币种</span>
            <input v-model.trim="form.balance_currency" class="input" placeholder="CNY" />
          </label>
        </div>

        <div class="grid gap-4 sm:grid-cols-2">
          <label class="block">
            <span class="input-label">后台地址</span>
            <input v-model.trim="form.dashboard_url" class="input" placeholder="https://supplier.example.com" />
          </label>
          <label class="block">
            <span class="input-label">API Base URL</span>
            <input v-model.trim="form.api_base_url" class="input" placeholder="https://supplier.example.com/api/v1" />
          </label>
        </div>

        <div class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
          <div class="mb-3 flex items-center justify-between gap-3">
            <div>
              <h3 class="text-sm font-medium text-gray-900 dark:text-gray-100">Chrome 插件登录凭据</h3>
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">作为下游客户无法拿到 Admin Key 时，由插件使用账号密码或临时 Token 登录供应商后台采集。</p>
            </div>
            <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
              <input v-model="form.browser_login_enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
              启用
            </label>
          </div>
          <div class="grid gap-4 sm:grid-cols-3">
            <label class="block">
              <span class="input-label">登录账号</span>
              <input v-model.trim="form.browser_login_username" class="input" autocomplete="username" :placeholder="editingSupplier ? '留空不修改' : ''" />
            </label>
            <label class="block">
              <span class="input-label">登录密码</span>
              <input v-model.trim="form.browser_login_password" type="password" class="input" autocomplete="new-password" :placeholder="editingSupplier ? '留空不修改' : ''" />
            </label>
            <label class="block">
              <span class="input-label">临时 Token</span>
              <input v-model.trim="form.browser_login_token" type="password" class="input" autocomplete="off" :placeholder="editingSupplier ? '留空不修改' : ''" />
            </label>
          </div>
        </div>

        <label class="block">
          <span class="input-label">备注</span>
          <textarea v-model.trim="form.notes" class="input min-h-[90px]" />
        </label>
      </form>

      <template #footer>
        <button type="button" class="btn btn-secondary" @click="closeEditor">取消</button>
        <button type="submit" form="supplier-editor-form" class="btn btn-primary" :disabled="submitting">
          {{ submitting ? '保存中...' : editingSupplier ? '保存修改' : '创建供应商' }}
        </button>
      </template>
    </BaseDialog>

    <BaseDialog :show="statusDialogOpen" :title="bulkStatusMode ? '批量调整供应商状态' : '调整供应商状态'" width="normal" @close="statusDialogOpen = false">
      <form id="supplier-status-form" class="space-y-4" @submit.prevent="submitStatus">
        <p class="text-sm text-gray-500 dark:text-dark-400">
          {{ bulkStatusMode ? `将调整 ${selectedCount} 个供应商` : statusForm.name }}
        </p>
        <label class="block">
          <span class="input-label">运行状态</span>
          <select v-model="statusForm.runtime_status" class="input">
            <option value="monitor_only">仅监控</option>
            <option value="candidate">候选</option>
            <option value="active">当前使用</option>
            <option value="disabled">停用</option>
          </select>
        </label>
        <label class="block">
          <span class="input-label">健康状态</span>
          <select v-model="statusForm.health_status" class="input">
            <option value="normal">正常</option>
            <option value="unavailable">不可用</option>
            <option value="credential_invalid">凭据失效</option>
            <option value="paused">暂停</option>
          </select>
        </label>
      </form>

      <template #footer>
        <button type="button" class="btn btn-secondary" @click="statusDialogOpen = false">取消</button>
        <button type="submit" form="supplier-status-form" class="btn btn-primary" :disabled="statusSubmitting">保存状态</button>
      </template>
    </BaseDialog>

    <BaseDialog :show="sessionDialogOpen" :title="sessionSupplier ? `供应商会话 - ${sessionSupplier.name}` : '供应商会话'" width="wide" @close="sessionDialogOpen = false">
      <div class="space-y-5">
        <div class="grid gap-4 md:grid-cols-3">
          <div class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
            <div class="text-xs text-gray-500 dark:text-dark-400">会话状态</div>
            <div class="mt-2 flex flex-wrap items-center gap-2">
              <span class="badge" :class="currentSession ? sessionStatusClass(currentSession.status) : 'badge-gray'">
                {{ currentSession ? sessionStatusLabel(currentSession.status) : '未上报' }}
              </span>
              <span v-if="currentSession?.has_encrypted_bundle" class="badge badge-success">已加密保存</span>
            </div>
          </div>
          <div class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
            <div class="text-xs text-gray-500 dark:text-dark-400">采集时间</div>
            <div class="mt-2 text-sm font-medium text-gray-900 dark:text-gray-100">{{ formatDateTime(currentSession?.captured_at) }}</div>
          </div>
          <div class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
            <div class="text-xs text-gray-500 dark:text-dark-400">过期时间</div>
            <div class="mt-2 text-sm font-medium text-gray-900 dark:text-gray-100">{{ formatDateTime(currentSession?.expires_at) }}</div>
          </div>
        </div>

        <div class="grid gap-4 md:grid-cols-2">
          <div class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
            <div class="text-sm font-medium text-gray-900 dark:text-gray-100">会话来源</div>
            <div class="mt-3 space-y-2 text-sm text-gray-600 dark:text-dark-300">
              <div class="break-all">Origin：{{ currentSession?.origin || '-' }}</div>
              <div class="break-all">API：{{ currentSession?.api_base_url || '-' }}</div>
              <div>插件任务：{{ currentSession?.source_extension_task_id || '-' }}</div>
            </div>
          </div>
          <div class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
            <div class="text-sm font-medium text-gray-900 dark:text-gray-100">脱敏摘要</div>
            <div class="mt-3 flex flex-wrap gap-2">
              <span class="badge" :class="summaryBoolClass('has_access_token')">Access Token</span>
              <span class="badge" :class="summaryBoolClass('has_refresh_token')">Refresh Token</span>
              <span class="badge" :class="summaryBoolClass('has_csrf_token')">CSRF</span>
              <span class="badge badge-gray">Cookie {{ summaryCookieCount }}</span>
              <span v-if="sessionSummaryString('organization_id')" class="badge badge-primary">Org {{ sessionSummaryString('organization_id') }}</span>
              <span v-if="sessionSummaryString('project_id')" class="badge badge-primary">Project {{ sessionSummaryString('project_id') }}</span>
            </div>
          </div>
        </div>

        <div v-if="lastProbe" class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
          <div class="mb-3 flex items-center justify-between gap-3">
            <div class="text-sm font-medium text-gray-900 dark:text-gray-100">最近探测结果</div>
            <span class="text-xs text-gray-500 dark:text-dark-400">{{ formatDateTime(lastProbe.probed_at) }}</span>
          </div>
          <div class="grid gap-3 md:grid-cols-4">
            <div>
              <div class="text-xs text-gray-500 dark:text-dark-400">系统</div>
              <div class="mt-1 text-sm font-medium text-gray-900 dark:text-gray-100">{{ lastProbe.system_type }}</div>
            </div>
            <div>
              <div class="text-xs text-gray-500 dark:text-dark-400">余额</div>
              <div class="mt-1 text-sm font-medium text-gray-900 dark:text-gray-100">
                {{ formatMoney(lastProbe.balance_cents || 0, lastProbe.balance_currency || 'USD') }}
              </div>
            </div>
            <div>
              <div class="text-xs text-gray-500 dark:text-dark-400">用户状态</div>
              <div class="mt-1 text-sm font-medium text-gray-900 dark:text-gray-100">{{ lastProbe.profile?.status || '-' }}</div>
            </div>
            <div>
              <div class="text-xs text-gray-500 dark:text-dark-400">可用分组</div>
              <div class="mt-1 text-sm font-medium text-gray-900 dark:text-gray-100">{{ lastProbe.profile?.allowed_groups?.length || 0 }}</div>
            </div>
          </div>
          <div class="mt-3 flex flex-wrap gap-2">
            <span v-for="capability in capabilityBadges" :key="capability.key" class="badge" :class="capability.enabled ? 'badge-success' : 'badge-gray'">
              {{ capability.label }}
            </span>
          </div>
        </div>

        <div v-if="sessionLoadError" class="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-200">
          {{ sessionLoadError }}
        </div>
      </div>

      <template #footer>
        <button type="button" class="btn btn-secondary" @click="sessionDialogOpen = false">关闭</button>
        <button type="button" class="btn btn-secondary" :disabled="sessionLoading || !sessionSupplier" @click="reloadCurrentSession">
          <Icon name="refresh" size="sm" :class="{ 'animate-spin': sessionLoading }" />
          刷新会话
        </button>
        <button type="button" class="btn btn-primary" :disabled="probingSession || !currentSession?.has_encrypted_bundle" @click="probeCurrentSession">
          <Icon name="beaker" size="sm" :class="{ 'animate-spin': probingSession }" />
          读取余额
        </button>
      </template>
    </BaseDialog>

    <BaseDialog :show="groupsDialogOpen" :title="groupsSupplier ? `供应商分组 - ${groupsSupplier.name}` : '供应商分组'" width="wide" @close="groupsDialogOpen = false">
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
          <div class="flex flex-wrap gap-2">
            <button type="button" class="btn btn-secondary" :disabled="groupsLoading || !groupsSupplier" @click="loadCurrentGroups">
              <Icon name="refresh" size="sm" :class="{ 'animate-spin': groupsLoading }" />
              刷新
            </button>
            <button type="button" class="btn btn-primary" :disabled="groupsSyncing || !currentGroupSession?.has_encrypted_bundle" @click="syncCurrentGroups">
              <Icon name="sync" size="sm" :class="{ 'animate-spin': groupsSyncing }" />
              同步分组
            </button>
          </div>
        </div>

        <div v-if="groupsError" class="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-200">
          {{ groupsError }}
        </div>

        <DataTable
          :columns="groupColumns"
          :data="supplierGroups"
          :loading="groupsLoading"
          row-key="id"
          default-sort-key="effective_rate_multiplier"
          default-sort-order="asc"
          :estimate-row-height="72"
        >
          <template #cell-name="{ row }">
            <div class="min-w-[220px]">
              <div class="flex items-center gap-2">
                <span class="font-medium text-gray-900 dark:text-white">{{ row.name }}</span>
                <span v-if="row.is_private" class="badge badge-warning">专属</span>
                <span v-if="row.allow_image_generation" class="badge badge-primary">图片</span>
              </div>
              <div class="mt-1 flex flex-wrap gap-2 text-xs text-gray-500 dark:text-dark-400">
                <span class="font-mono">#{{ row.external_group_id }}</span>
                <span v-if="row.description" class="max-w-[260px] truncate" :title="row.description">{{ row.description }}</span>
              </div>
            </div>
          </template>

          <template #cell-provider_family="{ row }">
            <span class="badge badge-gray">{{ row.provider_family || 'mixed' }}</span>
          </template>

          <template #cell-rate="{ row }">
            <div class="min-w-[120px] text-right">
              <div class="font-medium text-gray-900 dark:text-gray-100">{{ formatMultiplier(row.effective_rate_multiplier) }}</div>
              <div class="text-xs text-gray-500 dark:text-dark-400">
                基础 {{ formatMultiplier(row.rate_multiplier) }}
                <span v-if="row.user_rate_multiplier != null"> / 专属 {{ formatMultiplier(row.user_rate_multiplier) }}</span>
              </div>
            </div>
          </template>

          <template #cell-limits="{ row }">
            <div class="min-w-[150px] text-xs text-gray-600 dark:text-dark-300">
              <div>RPM：{{ row.rpm_limit ?? '-' }}</div>
              <div>日：{{ formatUSDLimit(row.daily_limit_usd) }}</div>
              <div>月：{{ formatUSDLimit(row.monthly_limit_usd) }}</div>
            </div>
          </template>

          <template #cell-account="{ row }">
            <div class="min-w-[260px]">
              <template v-if="groupKey(row)">
                <div class="flex flex-wrap items-center gap-2">
                  <span class="font-medium text-gray-900 dark:text-gray-100">{{ groupKey(row)?.name || '-' }}</span>
                  <span class="badge" :class="supplierKeyStatusClass(groupKey(row)?.status)">{{ supplierKeyStatusLabel(groupKey(row)?.status) }}</span>
                </div>
                <div class="mt-1 flex flex-wrap gap-2 text-xs text-gray-500 dark:text-dark-400">
                  <span v-if="groupKey(row)?.key_last4" class="font-mono">****{{ groupKey(row)?.key_last4 }}</span>
                  <span v-if="groupKey(row)?.external_key_id" class="font-mono">Key #{{ groupKey(row)?.external_key_id }}</span>
                  <span v-if="groupKey(row)?.local_sub2api_account_id">
                    本地账号 #{{ groupKey(row)?.local_sub2api_account_id }}
                  </span>
                  <span v-if="groupKey(row)?.local_account_name">{{ groupKey(row)?.local_account_name }}</span>
                </div>
                <div v-if="groupKey(row)?.error_message" class="mt-1 max-w-[320px] truncate text-xs text-red-600 dark:text-red-300" :title="groupKey(row)?.error_message">
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

          <template #cell-group_actions="{ row }">
            <div class="flex min-w-[190px] justify-end gap-2">
              <button
                type="button"
                class="btn btn-secondary btn-sm"
                :disabled="Boolean(groupKey(row)) || row.status !== 'active'"
                :title="groupKey(row) ? '该分组已有 Key 记录' : '开通第三方 Key 并同步创建本地账号'"
                @click="openProvisionDialog(row)"
              >
                <Icon name="key" size="sm" />
                开通
              </button>
              <button
                v-if="canRepairKey(groupKey(row))"
                type="button"
                class="btn btn-secondary btn-sm"
                title="绑定已有本地 Sub2API 账号"
                @click="openRepairDialog(groupKey(row))"
              >
                <Icon name="link" size="sm" />
                修复
              </button>
            </div>
          </template>

          <template #cell-status="{ row }">
            <span class="badge" :class="groupStatusClass(row.status)">{{ groupStatusLabel(row.status) }}</span>
          </template>

          <template #cell-last_seen_at="{ row }">
            <div class="min-w-[150px] text-xs text-gray-500 dark:text-dark-400">{{ formatDateTime(row.last_seen_at) }}</div>
          </template>

          <template #empty>
            <EmptyState
              title="暂无供应商分组"
              description="先通过 Chrome 插件上报供应商会话，再同步分组。"
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
        <button type="button" class="btn btn-secondary" @click="groupsDialogOpen = false">关闭</button>
      </template>
    </BaseDialog>

    <BaseDialog :show="provisionDialogOpen" :title="provisionGroup ? `开通 Key/账号 - ${provisionGroup.name}` : '开通 Key/账号'" width="wide" @close="closeProvisionDialog">
      <form id="supplier-key-provision-form" class="space-y-5" @submit.prevent="submitProvision">
        <div class="grid gap-4 md:grid-cols-3">
          <div class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
            <div class="text-xs text-gray-500 dark:text-dark-400">供应商</div>
            <div class="mt-2 text-sm font-medium text-gray-900 dark:text-gray-100">{{ groupsSupplier?.name || '-' }}</div>
          </div>
          <div class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
            <div class="text-xs text-gray-500 dark:text-dark-400">分组</div>
            <div class="mt-2 text-sm font-medium text-gray-900 dark:text-gray-100">{{ provisionGroup?.name || '-' }}</div>
            <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">#{{ provisionGroup?.external_group_id || '-' }}</div>
          </div>
          <div class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
            <div class="text-xs text-gray-500 dark:text-dark-400">倍率</div>
            <div class="mt-2 text-sm font-medium text-gray-900 dark:text-gray-100">{{ formatMultiplier(provisionGroup?.effective_rate_multiplier) }}</div>
          </div>
        </div>

        <div class="grid gap-4 sm:grid-cols-2">
          <label class="block">
            <span class="input-label">第三方 Key 名称</span>
            <input v-model.trim="provisionForm.name" class="input" required />
          </label>
          <label class="block">
            <span class="input-label">本地账号名称</span>
            <input v-model.trim="provisionForm.local_account_name" class="input" required />
          </label>
        </div>

        <div class="grid gap-4 sm:grid-cols-3">
          <label class="block">
            <span class="input-label">本地账号平台</span>
            <select v-model="provisionForm.local_account_platform" class="input">
              <option value="openai">OpenAI</option>
              <option value="anthropic">Anthropic</option>
              <option value="gemini">Gemini</option>
              <option value="antigravity">Antigravity</option>
            </select>
          </label>
          <label class="block">
            <span class="input-label">并发</span>
            <input v-model.number="provisionForm.local_account_concurrency" type="number" min="0" step="1" class="input" />
          </label>
          <label class="block">
            <span class="input-label">优先级</span>
            <input v-model.number="provisionForm.local_account_priority" type="number" min="0" step="1" class="input" />
          </label>
        </div>

        <label class="block">
          <span class="input-label">本地账号 Base URL</span>
          <input v-model.trim="provisionForm.local_account_base_url" class="input" required placeholder="https://supplier.example.com/v1" />
        </label>

        <div class="grid gap-4 sm:grid-cols-3">
          <label class="block">
            <span class="input-label">账号倍率</span>
            <input v-model.number="provisionForm.local_account_rate_multiplier" type="number" min="0" step="0.0001" class="input" />
          </label>
          <label class="block">
            <span class="input-label">第三方额度 USD</span>
            <input v-model.number="provisionForm.quota_usd" type="number" min="0" step="0.01" class="input" />
          </label>
          <label class="block">
            <span class="input-label">有效期天数</span>
            <input v-model.number="provisionForm.expires_in_days" type="number" min="0" step="1" class="input" placeholder="不填表示不限" />
          </label>
        </div>

        <div class="grid gap-4 sm:grid-cols-3">
          <label class="block">
            <span class="input-label">运行状态</span>
            <select v-model="provisionForm.runtime_status" class="input">
              <option value="monitor_only">仅监控</option>
              <option value="candidate">候选</option>
              <option value="active">当前使用</option>
              <option value="disabled">停用</option>
            </select>
          </label>
          <label class="block">
            <span class="input-label">余额</span>
            <input v-model.number="provisionForm.balance_yuan" type="number" min="0" step="0.01" class="input" />
          </label>
          <label class="block">
            <span class="input-label">低余额阈值</span>
            <input v-model.number="provisionForm.balance_threshold_yuan" type="number" min="0" step="0.01" class="input" />
          </label>
        </div>

        <div v-if="provisionError" class="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-200">
          {{ provisionError }}
        </div>
      </form>

      <template #footer>
        <button type="button" class="btn btn-secondary" @click="closeProvisionDialog">取消</button>
        <button type="submit" form="supplier-key-provision-form" class="btn btn-primary" :disabled="provisionSubmitting">
          <Icon name="key" size="sm" :class="{ 'animate-spin': provisionSubmitting }" />
          {{ provisionSubmitting ? '开通中...' : '创建 Key 并绑定' }}
        </button>
      </template>
    </BaseDialog>

    <BaseDialog :show="repairDialogOpen" :title="repairKey ? `修复绑定 - ${repairKey.name}` : '修复绑定'" width="normal" @close="closeRepairDialog">
      <form id="supplier-key-repair-form" class="space-y-5" @submit.prevent="submitRepairBinding">
        <div class="rounded-lg border border-amber-200 bg-amber-50 p-4 text-sm text-amber-800 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-200">
          第三方 Key 已创建，但本地账号绑定未完成。请选择已手动补好的本地 Sub2API 账号完成绑定。
        </div>

        <div class="grid gap-4 sm:grid-cols-2">
          <div class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
            <div class="text-xs text-gray-500 dark:text-dark-400">供应商侧 Key</div>
            <div class="mt-2 text-sm font-medium text-gray-900 dark:text-gray-100">{{ repairKey?.name || '-' }}</div>
            <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">
              <span v-if="repairKey?.external_key_id">#{{ repairKey.external_key_id }}</span>
              <span v-if="repairKey?.key_last4" class="ml-2 font-mono">****{{ repairKey.key_last4 }}</span>
            </div>
          </div>
          <div class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
            <div class="text-xs text-gray-500 dark:text-dark-400">失败原因</div>
            <div class="mt-2 text-sm font-medium text-red-700 dark:text-red-300">{{ repairKey?.error_code || '-' }}</div>
            <div class="mt-1 truncate text-xs text-gray-500 dark:text-dark-400" :title="repairKey?.error_message">{{ repairKey?.error_message || '-' }}</div>
          </div>
        </div>

        <label class="block">
          <span class="input-label">本地 Sub2API 账号</span>
          <select v-model.number="repairForm.local_sub2api_account_id" class="input" required :disabled="repairAccountsLoading">
            <option :value="0">{{ repairAccountsLoading ? '加载账号中...' : '请选择账号' }}</option>
            <option v-for="account in localAccounts" :key="account.id" :value="account.id">
              #{{ account.id }} · {{ account.name }} · {{ account.platform }}/{{ account.type }}
            </option>
          </select>
        </label>

        <div class="grid gap-4 sm:grid-cols-2">
          <label class="block">
            <span class="input-label">运行状态</span>
            <select v-model="repairForm.runtime_status" class="input">
              <option value="monitor_only">仅监控</option>
              <option value="candidate">候选</option>
              <option value="active">当前使用</option>
              <option value="disabled">停用</option>
            </select>
          </label>
          <label class="block">
            <span class="input-label">健康状态</span>
            <select v-model="repairForm.health_status" class="input">
              <option value="normal">正常</option>
              <option value="unavailable">不可用</option>
              <option value="credential_invalid">凭据失效</option>
              <option value="paused">暂停</option>
            </select>
          </label>
        </div>

        <div class="grid gap-4 sm:grid-cols-3">
          <label class="block">
            <span class="input-label">配置并发</span>
            <input v-model.number="repairForm.configured_concurrency" type="number" min="0" step="1" class="input" />
          </label>
          <label class="block">
            <span class="input-label">余额</span>
            <input v-model.number="repairForm.balance_yuan" type="number" min="0" step="0.01" class="input" />
          </label>
          <label class="block">
            <span class="input-label">低余额阈值</span>
            <input v-model.number="repairForm.balance_threshold_yuan" type="number" min="0" step="0.01" class="input" />
          </label>
        </div>

        <div v-if="repairError" class="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-200">
          {{ repairError }}
        </div>
      </form>

      <template #footer>
        <button type="button" class="btn btn-secondary" @click="closeRepairDialog">取消</button>
        <button type="submit" form="supplier-key-repair-form" class="btn btn-primary" :disabled="repairSubmitting || repairAccountsLoading">
          <Icon name="link" size="sm" :class="{ 'animate-spin': repairSubmitting }" />
          {{ repairSubmitting ? '修复中...' : '完成绑定' }}
        </button>
      </template>
    </BaseDialog>

    <ConfirmDialog
      :show="deleteDialogOpen"
      title="删除供应商"
      :message="deleteConfirmMessage"
      confirm-text="删除"
      :danger="true"
      @confirm="confirmDelete"
      @cancel="deleteDialogOpen = false"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'
import type { Column } from '@/components/common/types'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { useTableSelection } from '@/composables/useTableSelection'
import { useAppStore } from '@/stores/app'
import {
  createSupplier,
  deleteSupplier,
  getSupplierSession,
  listLocalSub2APIAccounts,
  listSupplierKeys,
  listSupplierGroups,
  listSuppliers,
  probeSupplierSession,
  provisionSupplierKey,
  repairSupplierKeyBinding,
  syncSupplierGroups,
  updateSupplier,
  updateSupplierStatus,
  type LocalSub2APIAccount,
  type Supplier,
  type SupplierBrowserSession,
  type SupplierGroup,
  type SupplierGroupStatus,
  type SupplierHealthStatus,
  type SupplierKey,
  type SupplierKeyStatus,
  type SupplierSessionProbeResult,
  type SupplierKind,
  type SupplierRuntimeStatus,
  type SupplierType
} from '@/api/admin/adminPlus'

const appStore = useAppStore()

const loading = ref(false)
const submitting = ref(false)
const statusSubmitting = ref(false)
const provisionSubmitting = ref(false)
const repairSubmitting = ref(false)
const editorOpen = ref(false)
const statusDialogOpen = ref(false)
const sessionDialogOpen = ref(false)
const groupsDialogOpen = ref(false)
const provisionDialogOpen = ref(false)
const repairDialogOpen = ref(false)
const deleteDialogOpen = ref(false)
const moreMenuOpen = ref(false)
const bulkStatusMode = ref(false)
const bulkDeleteMode = ref(false)
const editingSupplier = ref<Supplier | null>(null)
const sessionSupplier = ref<Supplier | null>(null)
const groupsSupplier = ref<Supplier | null>(null)
const provisionGroup = ref<SupplierGroup | null>(null)
const repairKey = ref<SupplierKey | null>(null)
const deletingSupplier = ref<Supplier | null>(null)
const suppliers = ref<Supplier[]>([])
const supplierGroups = ref<SupplierGroup[]>([])
const supplierKeys = ref<SupplierKey[]>([])
const localAccounts = ref<LocalSub2APIAccount[]>([])
const sessionStore = reactive<Record<number, SupplierBrowserSession | undefined>>({})
const sessionLoading = ref(false)
const probingSession = ref(false)
const groupsLoading = ref(false)
const groupsSyncing = ref(false)
const repairAccountsLoading = ref(false)
const sessionLoadError = ref('')
const groupsError = ref('')
const provisionError = ref('')
const repairError = ref('')
const lastProbe = ref<SupplierSessionProbeResult | null>(null)

const filters = reactive({
  q: '',
  kind: '' as '' | SupplierKind,
  type: '' as '' | SupplierType,
  runtime_status: '' as '' | SupplierRuntimeStatus,
  health_status: '' as '' | SupplierHealthStatus
})

const pagination = reactive({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0,
  pages: 0
})

const groupPagination = reactive({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0,
  pages: 0
})

const groupFilters = reactive({
  q: '',
  status: '' as '' | SupplierGroupStatus
})

const form = reactive({
  name: '',
  kind: 'relay' as SupplierKind,
  type: 'sub2api' as SupplierType,
  runtime_status: 'monitor_only' as SupplierRuntimeStatus,
  health_status: 'normal' as SupplierHealthStatus,
  dashboard_url: '',
  api_base_url: '',
  contact: '',
  browser_login_username: '',
  browser_login_password: '',
  browser_login_token: '',
  balance_yuan: 0,
  balance_currency: 'CNY',
  browser_login_enabled: true,
  notes: ''
})

const statusForm = reactive({
  id: 0,
  name: '',
  runtime_status: 'monitor_only' as SupplierRuntimeStatus,
  health_status: 'normal' as SupplierHealthStatus
})

const provisionForm = reactive({
  name: '',
  local_account_name: '',
  local_account_platform: 'openai',
  local_account_base_url: '',
  local_account_concurrency: 0,
  local_account_priority: 100,
  local_account_rate_multiplier: 1,
  quota_usd: 0,
  expires_in_days: null as number | null,
  runtime_status: 'monitor_only' as SupplierRuntimeStatus,
  health_status: 'normal' as SupplierHealthStatus,
  balance_yuan: 0,
  balance_threshold_yuan: 0,
  balance_currency: 'USD'
})

const repairForm = reactive({
  local_sub2api_account_id: 0,
  runtime_status: 'monitor_only' as SupplierRuntimeStatus,
  health_status: 'normal' as SupplierHealthStatus,
  configured_concurrency: 0,
  balance_yuan: 0,
  balance_threshold_yuan: 0,
  balance_currency: 'USD'
})

const columns: Column[] = [
  { key: 'select', label: '', class: 'w-10' },
  { key: 'name', label: '供应商', sortable: true },
  { key: 'balance', label: '余额', class: 'text-right' },
  { key: 'status', label: '状态' },
  { key: 'kind_type', label: '归类 / 类型' },
  { key: 'credential', label: '采集凭据' },
  { key: 'session', label: '浏览器会话' },
  { key: 'address', label: '地址' },
  { key: 'created_at', label: '创建时间', sortable: true },
  { key: 'actions', label: '操作', class: 'text-right' }
]

const groupColumns: Column[] = [
  { key: 'name', label: '分组' },
  { key: 'provider_family', label: '平台' },
  { key: 'rate', label: '倍率', class: 'text-right' },
  { key: 'limits', label: '限制' },
  { key: 'account', label: 'Key / 本地账号' },
  { key: 'status', label: '状态' },
  { key: 'last_seen_at', label: '最后同步', sortable: true },
  { key: 'group_actions', label: '操作', class: 'text-right' }
]

const filteredSuppliers = computed(() => suppliers.value)

const {
  selectedIds,
  selectedCount,
  allVisibleSelected,
  isSelected,
  toggle: toggleSelection,
  clear: clearSelection,
  selectVisible,
  toggleVisible
} = useTableSelection<Supplier>({
  rows: filteredSuppliers,
  getId: (row) => row.id
})

const selectedRows = computed(() => suppliers.value.filter((item) => selectedIds.value.includes(item.id)))

const currentSession = computed(() => {
  const supplierID = sessionSupplier.value?.id
  return supplierID ? sessionStore[supplierID] : undefined
})

const currentGroupSession = computed(() => {
  const supplierID = groupsSupplier.value?.id
  return supplierID ? sessionStore[supplierID] : undefined
})

const currentSessionSummary = computed(() => currentSession.value?.session_summary || {})

const supplierKeysByGroupID = computed(() => {
  const out = new Map<number, SupplierKey>()
  for (const key of supplierKeys.value) {
    const existing = out.get(key.supplier_group_id)
    if (!existing || key.id > existing.id) {
      out.set(key.supplier_group_id, key)
    }
  }
  return out
})

const summaryCookieCount = computed(() => {
  const value = currentSessionSummary.value.cookie_count
  return typeof value === 'number' ? value : 0
})

const capabilityBadges = computed(() => {
  const capabilities = lastProbe.value?.capabilities || {}
  return [
    { key: 'can_read_profile', label: 'Profile', enabled: Boolean(capabilities.can_read_profile) },
    { key: 'can_read_balance', label: '余额', enabled: Boolean(capabilities.can_read_balance) },
    { key: 'can_read_groups', label: '分组', enabled: Boolean(capabilities.can_read_groups) },
    { key: 'can_create_key', label: '创建 Key', enabled: Boolean(capabilities.can_create_key) },
    { key: 'can_read_billing', label: '账单', enabled: Boolean(capabilities.can_read_billing) }
  ]
})

const deleteConfirmMessage = computed(() => {
  if (bulkDeleteMode.value) {
    return `确认删除已选择的 ${selectedCount.value} 个供应商？该操作会删除供应商父级及其账号/Key 绑定。`
  }
  return deletingSupplier.value
    ? `确认删除供应商「${deletingSupplier.value.name}」？该操作会删除其账号/Key 绑定。`
    : '确认删除该供应商？'
})

function toggleSelectAllVisible(event: Event) {
  toggleVisible((event.target as HTMLInputElement).checked)
}

function centsFromYuan(value: number): number {
  return Math.round(Number(value || 0) * 100)
}

function yuanFromCents(value: number): number {
  return Number(((value || 0) / 100).toFixed(2))
}

function formatMoney(cents: number, currency: string): string {
  return new Intl.NumberFormat(undefined, {
    style: 'currency',
    currency: currency || 'CNY',
    minimumFractionDigits: 2
  }).format((cents || 0) / 100)
}

function formatDateTime(value?: string | null): string {
  if (!value) return '-'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '-' : date.toLocaleString()
}

function formatMultiplier(value?: number | null): string {
  if (typeof value !== 'number') return '-'
  return `${Number(value).toFixed(4)}x`
}

function formatUSDLimit(value?: number | null): string {
  if (typeof value !== 'number') return '-'
  return new Intl.NumberFormat(undefined, {
    style: 'currency',
    currency: 'USD',
    maximumFractionDigits: 2
  }).format(value)
}

function kindLabel(value: SupplierKind): string {
  return {
    source_account: '源站',
    relay: '中转',
    browser_only: '浏览器',
    custom: '自定义'
  }[value]
}

function typeLabel(value: SupplierType): string {
  return {
    openai: 'OpenAI',
    anthropic: 'Anthropic',
    gemini: 'Gemini',
    sub2api: 'Sub2API',
    new_api: 'New API',
    browser_only: '仅浏览器',
    custom: '自定义'
  }[value]
}

function runtimeLabel(value: SupplierRuntimeStatus): string {
  return {
    monitor_only: '仅监控',
    candidate: '候选',
    active: '使用中',
    disabled: '停用'
  }[value]
}

function healthLabel(value: SupplierHealthStatus): string {
  return {
    normal: '正常',
    unavailable: '不可用',
    credential_invalid: '凭据失效',
    paused: '暂停'
  }[value]
}

function runtimeClass(status: SupplierRuntimeStatus): string {
  if (status === 'active') return 'badge-success'
  if (status === 'candidate') return 'badge-primary'
  if (status === 'disabled') return 'badge-danger'
  return 'badge-gray'
}

function healthClass(status: SupplierHealthStatus): string {
  if (status === 'normal') return 'badge-success'
  if (status === 'paused') return 'badge-warning'
  return 'badge-danger'
}

function supplierBalanceAmountClass(supplier: Supplier): string {
  if (supplier.balance_cents > 0 && isSwitchableRuntimeStatus(supplier.runtime_status)) return 'text-emerald-700 dark:text-emerald-300'
  if (supplier.balance_cents <= 0) return 'text-red-600 dark:text-red-300'
  return 'text-gray-900 dark:text-gray-100'
}

function supplierBalanceBadgeClass(supplier: Supplier): string {
  if (supplier.balance_cents <= 0) return 'badge-danger'
  if (isSwitchableRuntimeStatus(supplier.runtime_status)) return 'badge-success'
  return 'badge-gray'
}

function supplierBalanceLabel(supplier: Supplier): string {
  if (supplier.balance_cents <= 0) return '无余额'
  if (isSwitchableRuntimeStatus(supplier.runtime_status)) return '可用余额'
  return '仅监控余额'
}

function supplierSwitchStateLabel(supplier: Supplier): string {
  if (isSwitchable(supplier)) return supplier.runtime_status === 'active' ? '当前承载流量' : '可进入候选'
  if (supplier.runtime_status === 'monitor_only') return '仅监控，不切换'
  if (supplier.balance_cents <= 0) return '余额不足，不切换'
  if (supplier.health_status !== 'normal') return '健康异常，不切换'
  return '不可切换'
}

function supplierSwitchStateClass(supplier: Supplier): string {
  if (isSwitchable(supplier)) return 'text-emerald-700 dark:text-emerald-300'
  if (supplier.balance_cents <= 0 || supplier.health_status !== 'normal') return 'text-red-600 dark:text-red-300'
  return 'text-gray-500 dark:text-dark-400'
}

function sessionStatusLabel(status?: SupplierBrowserSession['status']): string {
  if (status === 'valid') return '有效'
  if (status === 'expired') return '已过期'
  return '未上报'
}

function sessionStatusClass(status?: SupplierBrowserSession['status']): string {
  if (status === 'valid') return 'badge-success'
  if (status === 'expired') return 'badge-warning'
  return 'badge-gray'
}

function groupStatusLabel(status?: SupplierGroupStatus): string {
  if (status === 'active') return '有效'
  if (status === 'missing') return '已缺失'
  if (status === 'disabled') return '停用'
  return '未知'
}

function groupStatusClass(status?: SupplierGroupStatus): string {
  if (status === 'active') return 'badge-success'
  if (status === 'missing') return 'badge-warning'
  return 'badge-gray'
}

function supplierKeyStatusLabel(status?: SupplierKeyStatus): string {
  if (status === 'bound') return '已绑定'
  if (status === 'provisioning') return '开通中'
  if (status === 'manual_secret_required') return '待补密钥'
  if (status === 'failed') return '失败'
  if (status === 'disabled') return '停用'
  return '未知'
}

function supplierKeyStatusClass(status?: SupplierKeyStatus): string {
  if (status === 'bound') return 'badge-success'
  if (status === 'provisioning') return 'badge-primary'
  if (status === 'manual_secret_required') return 'badge-warning'
  if (status === 'failed') return 'badge-danger'
  return 'badge-gray'
}

function groupKey(group: SupplierGroup): SupplierKey | undefined {
  return supplierKeysByGroupID.value.get(group.id)
}

function canRepairKey(key?: SupplierKey): key is SupplierKey {
  return key?.status === 'failed' && ['LOCAL_ACCOUNT_CREATE_FAILED', 'SUPPLIER_ACCOUNT_BIND_FAILED'].includes(key.error_code || '')
}

function sessionBadgeText(supplierID: number): string {
  return sessionStatusLabel(sessionStore[supplierID]?.status)
}

function sessionBadgeClass(supplierID: number): string {
  return sessionStatusClass(sessionStore[supplierID]?.status)
}

function summaryBoolClass(key: string): string {
  return currentSessionSummary.value[key] ? 'badge-success' : 'badge-gray'
}

function sessionSummaryString(key: string): string {
  const value = currentSessionSummary.value[key]
  return typeof value === 'string' ? value : ''
}

function isSwitchable(supplier: Supplier): boolean {
  return ['candidate', 'active'].includes(supplier.runtime_status) && supplier.health_status === 'normal' && supplier.balance_cents > 0
}

function isSwitchableRuntimeStatus(status: SupplierRuntimeStatus): boolean {
  return status === 'candidate' || status === 'active'
}

function hasCredential(supplier: Supplier): boolean {
  return supplier.credential.postgres_configured ||
    supplier.credential.redis_configured ||
    supplier.credential.browser_login_enabled ||
    supplier.credential.browser_login_username_configured ||
    supplier.credential.browser_login_password_configured ||
    supplier.credential.browser_login_token_configured
}

async function loadSuppliers() {
  loading.value = true
  try {
    const result = await listSuppliers({
      q: filters.q || undefined,
      kind: filters.kind || undefined,
      type: filters.type || undefined,
      runtime_status: filters.runtime_status || undefined,
      health_status: filters.health_status || undefined,
      page: pagination.page,
      page_size: pagination.page_size
    })
    suppliers.value = result.items
    pagination.total = result.total || 0
    pagination.pages = result.pages || 0
    pagination.page = result.page || pagination.page
    pagination.page_size = result.page_size || pagination.page_size
    void preloadVisibleSessions()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '加载供应商失败')
  } finally {
    loading.value = false
  }
}

async function preloadVisibleSessions() {
  await Promise.all(suppliers.value.map(async (supplier) => {
    try {
      sessionStore[supplier.id] = await getSupplierSession(supplier.id)
    } catch {
      sessionStore[supplier.id] = undefined
    }
  }))
}

async function reloadCurrentSession() {
  if (!sessionSupplier.value) return
  sessionLoading.value = true
  sessionLoadError.value = ''
  try {
    sessionStore[sessionSupplier.value.id] = await getSupplierSession(sessionSupplier.value.id)
  } catch (error) {
    sessionStore[sessionSupplier.value.id] = undefined
    sessionLoadError.value = (error as { message?: string }).message || '当前供应商还没有浏览器会话'
  } finally {
    sessionLoading.value = false
  }
}

async function reloadGroupSession() {
  if (!groupsSupplier.value) return
  try {
    sessionStore[groupsSupplier.value.id] = await getSupplierSession(groupsSupplier.value.id)
  } catch {
    sessionStore[groupsSupplier.value.id] = undefined
  }
}

function reloadFirstPage() {
  pagination.page = 1
  void loadSuppliers()
}

function handlePageChange(page: number) {
  pagination.page = page
  void loadSuppliers()
}

function handlePageSizeChange(pageSize: number) {
  pagination.page_size = pageSize
  pagination.page = 1
  void loadSuppliers()
}

function handleGroupPageChange(page: number) {
  groupPagination.page = page
  void loadCurrentGroups()
}

function handleGroupPageSizeChange(pageSize: number) {
  groupPagination.page_size = pageSize
  groupPagination.page = 1
  void loadCurrentGroups()
}

function resetFilters() {
  filters.q = ''
  filters.kind = ''
  filters.type = ''
  filters.runtime_status = ''
  filters.health_status = ''
  moreMenuOpen.value = false
}

function resetForm() {
  form.name = ''
  form.kind = 'relay'
  form.type = 'sub2api'
  form.runtime_status = 'monitor_only'
  form.health_status = 'normal'
  form.dashboard_url = ''
  form.api_base_url = ''
  form.contact = ''
  form.browser_login_username = ''
  form.browser_login_password = ''
  form.browser_login_token = ''
  form.balance_yuan = 0
  form.balance_currency = 'CNY'
  form.browser_login_enabled = true
  form.notes = ''
}

function fillForm(supplier: Supplier) {
  form.name = supplier.name
  form.kind = supplier.kind
  form.type = supplier.type
  form.runtime_status = supplier.runtime_status
  form.health_status = supplier.health_status
  form.dashboard_url = supplier.dashboard_url || ''
  form.api_base_url = supplier.api_base_url || ''
  form.contact = supplier.contact || ''
  form.browser_login_username = ''
  form.browser_login_password = ''
  form.browser_login_token = ''
  form.balance_yuan = yuanFromCents(supplier.balance_cents)
  form.balance_currency = supplier.balance_currency || 'CNY'
  form.browser_login_enabled = supplier.credential.browser_login_enabled
  form.notes = supplier.notes || ''
}

function openCreateDialog() {
  editingSupplier.value = null
  resetForm()
  editorOpen.value = true
}

function openEditDialog(supplier: Supplier) {
  editingSupplier.value = supplier
  fillForm(supplier)
  editorOpen.value = true
}

function closeEditor() {
  editorOpen.value = false
}

function buildPayload() {
  return {
    name: form.name,
    kind: form.kind,
    type: form.type,
    runtime_status: form.runtime_status,
    health_status: form.health_status,
    dashboard_url: form.dashboard_url || undefined,
    api_base_url: form.api_base_url || undefined,
    contact: form.contact || undefined,
    browser_login_username: form.browser_login_username || undefined,
    browser_login_password: form.browser_login_password || undefined,
    browser_login_token: form.browser_login_token || undefined,
    balance_cents: centsFromYuan(form.balance_yuan),
    balance_currency: form.balance_currency || 'CNY',
    browser_login_enabled: form.browser_login_enabled,
    notes: form.notes || undefined
  }
}

async function submitSupplier() {
  submitting.value = true
  try {
    if (editingSupplier.value) {
      await updateSupplier(editingSupplier.value.id, buildPayload())
      appStore.showSuccess('供应商已更新')
    } else {
      await createSupplier(buildPayload())
      appStore.showSuccess('供应商已创建')
    }
    editorOpen.value = false
    await loadSuppliers()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '保存供应商失败')
  } finally {
    submitting.value = false
  }
}

function openStatusDialog(supplier: Supplier) {
  bulkStatusMode.value = false
  statusForm.id = supplier.id
  statusForm.name = supplier.name
  statusForm.runtime_status = supplier.runtime_status
  statusForm.health_status = supplier.health_status
  statusDialogOpen.value = true
}

function openSessionDialog(supplier: Supplier) {
  sessionSupplier.value = supplier
  lastProbe.value = null
  sessionLoadError.value = ''
  sessionDialogOpen.value = true
  void reloadCurrentSession()
}

function openGroupsDialog(supplier: Supplier) {
  groupsSupplier.value = supplier
  supplierGroups.value = []
  supplierKeys.value = []
  groupsError.value = ''
  groupPagination.page = 1
  groupFilters.q = ''
  groupFilters.status = ''
  groupsDialogOpen.value = true
  void Promise.all([reloadGroupSession(), loadCurrentGroups()])
}

async function probeCurrentSession() {
  if (!sessionSupplier.value) return
  probingSession.value = true
  sessionLoadError.value = ''
  try {
    const result = await probeSupplierSession(sessionSupplier.value.id, {
      record_balance_snapshot: true
    })
    lastProbe.value = result.probe
    appStore.showSuccess('会话探测完成，已读取供应商余额')
    await Promise.all([reloadCurrentSession(), loadSuppliers()])
  } catch (error) {
    sessionLoadError.value = (error as { message?: string }).message || '会话探测失败'
    appStore.showError(sessionLoadError.value)
  } finally {
    probingSession.value = false
  }
}

async function loadCurrentGroups() {
  if (!groupsSupplier.value) return
  groupsLoading.value = true
  groupsError.value = ''
  try {
    const [result, keyResult] = await Promise.all([
      listSupplierGroups(groupsSupplier.value.id, {
        q: groupFilters.q || undefined,
        status: groupFilters.status || undefined,
        page: groupPagination.page,
        page_size: groupPagination.page_size
      }),
      listSupplierKeys(groupsSupplier.value.id, {
        page: 1,
        page_size: 1000
      })
    ])
    supplierGroups.value = result.items
    supplierKeys.value = keyResult.items
    groupPagination.total = result.total || 0
    groupPagination.pages = result.pages || 0
    groupPagination.page = result.page || groupPagination.page
    groupPagination.page_size = result.page_size || groupPagination.page_size
  } catch (error) {
    groupsError.value = (error as { message?: string }).message || '加载供应商分组失败'
  } finally {
    groupsLoading.value = false
  }
}

function openProvisionDialog(group: SupplierGroup) {
  if (!groupsSupplier.value) return
  if (!currentGroupSession.value?.has_encrypted_bundle) {
    appStore.showError('当前供应商还没有可用浏览器会话，请先通过插件登录并上报会话')
    return
  }
  provisionGroup.value = group
  provisionError.value = ''
  fillProvisionForm(group)
  provisionDialogOpen.value = true
}

function closeProvisionDialog() {
  provisionDialogOpen.value = false
  provisionGroup.value = null
  provisionError.value = ''
}

function openRepairDialog(key?: SupplierKey) {
  if (!key || !groupsSupplier.value) return
  repairKey.value = key
  repairError.value = ''
  fillRepairForm(key)
  repairDialogOpen.value = true
  void loadRepairLocalAccounts()
}

function closeRepairDialog() {
  repairDialogOpen.value = false
  repairKey.value = null
  repairError.value = ''
  repairForm.local_sub2api_account_id = 0
}

function fillProvisionForm(group: SupplierGroup) {
  const supplier = groupsSupplier.value
  const name = defaultProvisionName(supplier, group)
  provisionForm.name = name
  provisionForm.local_account_name = name
  provisionForm.local_account_platform = normalizeLocalPlatform(group.provider_family)
  provisionForm.local_account_base_url = defaultProviderBaseURL(supplier)
  provisionForm.local_account_concurrency = Number(group.rpm_limit || 0)
  provisionForm.local_account_priority = 100
  provisionForm.local_account_rate_multiplier = Number(group.effective_rate_multiplier || 1)
  provisionForm.quota_usd = 0
  provisionForm.expires_in_days = null
  provisionForm.runtime_status = 'monitor_only'
  provisionForm.health_status = 'normal'
  provisionForm.balance_yuan = yuanFromCents(supplier?.balance_cents || 0)
  provisionForm.balance_threshold_yuan = 0
  provisionForm.balance_currency = supplier?.balance_currency || 'USD'
}

function fillRepairForm(key: SupplierKey) {
  const supplier = groupsSupplier.value
  repairForm.local_sub2api_account_id = key.local_sub2api_account_id || 0
  repairForm.runtime_status = 'monitor_only'
  repairForm.health_status = 'normal'
  repairForm.configured_concurrency = 0
  repairForm.balance_yuan = yuanFromCents(supplier?.balance_cents || 0)
  repairForm.balance_threshold_yuan = 0
  repairForm.balance_currency = supplier?.balance_currency || 'USD'
}

function defaultProvisionName(supplier: Supplier | null, group: SupplierGroup): string {
  return [supplier?.name, group.name].filter(Boolean).join('-') || `supplier-group-${group.id}`
}

function normalizeLocalPlatform(providerFamily?: string): string {
  const value = String(providerFamily || '').toLowerCase()
  if (value.includes('anthropic') || value.includes('claude')) return 'anthropic'
  if (value.includes('gemini') || value.includes('google')) return 'gemini'
  if (value.includes('antigravity')) return 'antigravity'
  return 'openai'
}

function defaultProviderBaseURL(supplier: Supplier | null): string {
  const configured = supplier?.api_base_url?.trim()
  if (configured) return normalizeGatewayBaseURL(configured)
  const dashboard = supplier?.dashboard_url?.trim()
  if (dashboard) return normalizeGatewayBaseURL(dashboard)
  return ''
}

function normalizeGatewayBaseURL(raw: string): string {
  try {
    const url = new URL(raw)
    const pathname = url.pathname.replace(/\/+$/, '')
    if (pathname.endsWith('/api/v1')) {
      url.pathname = `${pathname.slice(0, -'/api/v1'.length)}/v1`
    } else if (!pathname.endsWith('/v1')) {
      url.pathname = `${pathname}/v1`
    }
    url.search = ''
    url.hash = ''
    return url.toString().replace(/\/$/, '')
  } catch {
    return raw.replace(/\/+$/, '')
  }
}

async function submitProvision() {
  if (!groupsSupplier.value || !provisionGroup.value) return
  if (!provisionForm.local_account_base_url.trim()) {
    provisionError.value = '请填写本地账号 Base URL'
    return
  }
  if (isSwitchableRuntimeStatus(provisionForm.runtime_status) && centsFromYuan(provisionForm.balance_yuan) <= 0) {
    provisionError.value = '候选或使用中账号必须有可用余额'
    return
  }
  provisionSubmitting.value = true
  provisionError.value = ''
  try {
    await provisionSupplierKey(groupsSupplier.value.id, {
      supplier_group_id: provisionGroup.value.id,
      name: provisionForm.name,
      quota_usd: Number(provisionForm.quota_usd || 0),
      expires_in_days: provisionForm.expires_in_days || null,
      local_account_platform: provisionForm.local_account_platform,
      local_account_name: provisionForm.local_account_name,
      local_account_base_url: provisionForm.local_account_base_url,
      local_account_concurrency: Number(provisionForm.local_account_concurrency || 0),
      local_account_priority: Number(provisionForm.local_account_priority || 0),
      local_account_rate_multiplier: Number(provisionForm.local_account_rate_multiplier || 0),
      runtime_status: provisionForm.runtime_status,
      health_status: provisionForm.health_status,
      balance_threshold_cents: centsFromYuan(provisionForm.balance_threshold_yuan),
      balance_cents: centsFromYuan(provisionForm.balance_yuan),
      balance_currency: provisionForm.balance_currency || 'USD'
    })
    appStore.showSuccess('已创建第三方 Key，并同步创建本地 Sub2API 账号')
    closeProvisionDialog()
    await loadCurrentGroups()
  } catch (error) {
    provisionError.value = (error as { message?: string }).message || '开通 Key/账号失败'
    appStore.showError(provisionError.value)
  } finally {
    provisionSubmitting.value = false
  }
}

async function loadRepairLocalAccounts() {
  repairAccountsLoading.value = true
  try {
    const result = await listLocalSub2APIAccounts({ page: 1, page_size: 200 })
    localAccounts.value = result.items
  } catch (error) {
    repairError.value = (error as { message?: string }).message || '加载本地账号失败'
  } finally {
    repairAccountsLoading.value = false
  }
}

async function submitRepairBinding() {
  if (!groupsSupplier.value || !repairKey.value) return
  if (!repairForm.local_sub2api_account_id) {
    repairError.value = '请选择本地 Sub2API 账号'
    return
  }
  if (isSwitchableRuntimeStatus(repairForm.runtime_status) && centsFromYuan(repairForm.balance_yuan) <= 0) {
    repairError.value = '候选或使用中账号必须有可用余额'
    return
  }
  repairSubmitting.value = true
  repairError.value = ''
  try {
    await repairSupplierKeyBinding(groupsSupplier.value.id, repairKey.value.id, {
      local_sub2api_account_id: repairForm.local_sub2api_account_id,
      runtime_status: repairForm.runtime_status,
      health_status: repairForm.health_status,
      configured_concurrency: Number(repairForm.configured_concurrency || 0),
      balance_threshold_cents: centsFromYuan(repairForm.balance_threshold_yuan),
      balance_cents: centsFromYuan(repairForm.balance_yuan),
      balance_currency: repairForm.balance_currency || 'USD'
    })
    appStore.showSuccess('已修复 Key 与本地账号绑定')
    closeRepairDialog()
    await loadCurrentGroups()
  } catch (error) {
    repairError.value = (error as { message?: string }).message || '修复绑定失败'
    appStore.showError(repairError.value)
  } finally {
    repairSubmitting.value = false
  }
}

async function syncCurrentGroups() {
  if (!groupsSupplier.value) return
  groupsSyncing.value = true
  groupsError.value = ''
  try {
    const result = await syncSupplierGroups(groupsSupplier.value.id)
    appStore.showSuccess(`已同步 ${result.total} 个供应商分组`)
    groupPagination.page = 1
    await Promise.all([reloadGroupSession(), loadCurrentGroups()])
  } catch (error) {
    groupsError.value = (error as { message?: string }).message || '同步供应商分组失败'
    appStore.showError(groupsError.value)
  } finally {
    groupsSyncing.value = false
  }
}

function openBulkStatusDialog() {
  if (selectedCount.value === 0) return
  moreMenuOpen.value = false
  bulkStatusMode.value = true
  const first = selectedRows.value[0]
  statusForm.id = 0
  statusForm.name = ''
  statusForm.runtime_status = first?.runtime_status || 'monitor_only'
  statusForm.health_status = first?.health_status || 'normal'
  statusDialogOpen.value = true
}

async function submitStatus() {
  statusSubmitting.value = true
  try {
    if (bulkStatusMode.value) {
      await runSequential(selectedRows.value, async (supplier) => {
        await updateSupplierStatus(supplier.id, {
          runtime_status: statusForm.runtime_status,
          health_status: statusForm.health_status
        })
      })
      appStore.showSuccess('批量状态已更新')
      clearSelection()
    } else if (statusForm.id) {
      await updateSupplierStatus(statusForm.id, {
        runtime_status: statusForm.runtime_status,
        health_status: statusForm.health_status
      })
      appStore.showSuccess('状态已更新')
    }
    statusDialogOpen.value = false
    await loadSuppliers()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '更新状态失败')
  } finally {
    statusSubmitting.value = false
  }
}

function openDeleteDialog(supplier: Supplier) {
  bulkDeleteMode.value = false
  deletingSupplier.value = supplier
  deleteDialogOpen.value = true
}

function openBulkDeleteDialog() {
  if (selectedCount.value === 0) return
  moreMenuOpen.value = false
  bulkDeleteMode.value = true
  deletingSupplier.value = null
  deleteDialogOpen.value = true
}

async function confirmDelete() {
  try {
    if (bulkDeleteMode.value) {
      await runSequential(selectedRows.value, async (supplier) => {
        await deleteSupplier(supplier.id)
      })
      appStore.showSuccess('已删除选中的供应商')
      clearSelection()
    } else if (deletingSupplier.value) {
      await deleteSupplier(deletingSupplier.value.id)
      appStore.showSuccess('供应商已删除')
    }
    deleteDialogOpen.value = false
    await loadSuppliers()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '删除供应商失败')
  }
}

async function runSequential<T>(items: T[], runner: (item: T) => Promise<void>) {
  for (const item of items) {
    await runner(item)
  }
}

watch(
  () => ({ ...filters }),
  () => {
    reloadFirstPage()
  }
)

watch(
  () => ({ ...groupFilters }),
  () => {
    if (!groupsDialogOpen.value) return
    groupPagination.page = 1
    void loadCurrentGroups()
  }
)

onMounted(loadSuppliers)
</script>

<style scoped>
.menu-item {
  @apply flex w-full items-center gap-3 rounded-md px-3 py-2 text-sm text-gray-700 transition-colors hover:bg-gray-100 disabled:cursor-not-allowed disabled:opacity-50 dark:text-gray-200 dark:hover:bg-gray-700;
}

.menu-icon {
  @apply flex h-8 w-8 items-center justify-center rounded-md;
}
</style>
