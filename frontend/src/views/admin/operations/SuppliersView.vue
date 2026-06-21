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

          <template #cell-cost="{ row }">
            <div class="min-w-[190px] text-right">
              <template v-if="supplierCostSnapshot(row.id)">
                <div class="text-sm font-semibold text-gray-900 dark:text-gray-100">
                  充值 {{ formatMoney(supplierCostSnapshot(row.id)?.completed_funding_amount_cents || 0, supplierCostSnapshot(row.id)?.currency || row.balance_currency) }}
                </div>
                <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                  兑换 {{ formatMoney(supplierCostSnapshot(row.id)?.entitlement_amount_cents || 0, supplierCostSnapshot(row.id)?.currency || row.balance_currency) }}
                  · 消耗 {{ formatMoney(supplierCostSnapshot(row.id)?.usage_cost_cents || 0, supplierCostSnapshot(row.id)?.currency || row.balance_currency) }}
                </div>
                <div class="mt-1 text-xs" :class="costDeltaClass(row.id)">
                  差异 {{ costDeltaLabel(row.id) }}
                </div>
              </template>
              <template v-else>
                <span class="badge badge-gray">未同步成本</span>
              </template>
            </div>
          </template>

          <template #cell-credential="{ row }">
            <div class="flex min-w-[210px] flex-wrap gap-1">
              <span v-if="row.credential.browser_login_enabled" class="badge badge-warning">Chrome</span>
              <span v-if="row.credential.browser_login_username_configured" class="badge badge-gray">
                {{ row.credential.masked_browser_login_username || '账号' }}
              </span>
              <span v-if="row.credential.browser_login_password_configured" class="badge badge-success">密码</span>
              <span
                v-if="needsDirectLoginCredential(row)"
                class="badge badge-danger"
                title="未配置登录账号密码或临时 Token，请先编辑供应商补齐凭据"
              >
                凭据未配置
              </span>
              <span
                v-if="shouldShowTokenBadge(row)"
                class="badge"
                :class="credentialTokenBadgeClass(row)"
                :title="credentialTokenBadgeTitle(row)"
              >
                {{ credentialTokenBadgeText(row) }}
              </span>
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

    <Teleport to="body">
      <div
        v-if="rowActionsMenuSupplier"
        data-supplier-row-actions-menu
        class="fixed z-[1200] overflow-hidden rounded-lg border border-gray-200 bg-white shadow-xl dark:border-gray-700 dark:bg-gray-800"
        :style="rowActionsMenuStyle"
        role="menu"
        @click.stop
      >
        <div class="p-2">
          <button class="row-action-menu-item" role="menuitem" @click="openRowStatusDialog">
            <span class="row-action-menu-icon bg-blue-50 text-blue-600 dark:bg-blue-900/30 dark:text-blue-300">
              <Icon name="checkCircle" size="sm" />
            </span>
            <span>状态</span>
          </button>
          <button class="row-action-menu-item" role="menuitem" @click="openRowSessionDialog">
            <span class="row-action-menu-icon bg-emerald-50 text-emerald-600 dark:bg-emerald-900/30 dark:text-emerald-300">
              <Icon name="shield" size="sm" />
            </span>
            <span>会话</span>
          </button>
          <button class="row-action-menu-item" role="menuitem" @click="openRowGroupsDialog">
            <span class="row-action-menu-icon bg-slate-100 text-slate-600 dark:bg-slate-700 dark:text-slate-200">
              <Icon name="database" size="sm" />
            </span>
            <span>分组</span>
          </button>
          <button class="row-action-menu-item" role="menuitem" @click="openRowChannelStatusDialog">
            <span class="row-action-menu-icon bg-violet-50 text-violet-600 dark:bg-violet-900/30 dark:text-violet-300">
              <Icon name="chart" size="sm" />
            </span>
            <span>渠道状态</span>
          </button>
          <div class="my-2 border-t border-gray-100 dark:border-gray-700"></div>
          <button class="row-action-menu-item text-red-600 dark:text-red-300" role="menuitem" @click="openRowDeleteDialog">
            <span class="row-action-menu-icon bg-red-50 text-red-600 dark:bg-red-900/30 dark:text-red-300">
              <Icon name="trash" size="sm" />
            </span>
            <span>删除</span>
          </button>
        </div>
      </div>
    </Teleport>

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
              <div>来源：{{ sessionSourceLabel(currentSession?.session_source) }}</div>
              <div class="break-all">Origin：{{ currentSession?.origin || '-' }}</div>
              <div class="break-all">API：{{ currentSession?.api_base_url || '-' }}</div>
              <div v-if="currentSession?.source_extension_task_id">插件任务：{{ currentSession.source_extension_task_id }}</div>
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

        <div class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
          <div class="mb-3 flex items-center justify-between gap-3">
            <div>
              <div class="text-sm font-medium text-gray-900 dark:text-gray-100">当前余额</div>
              <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                {{ currentBalanceCaption }}
              </div>
            </div>
            <span class="badge" :class="currentBalanceBadgeClass">{{ currentBalanceBadgeText }}</span>
          </div>
          <div class="grid gap-3 md:grid-cols-4">
            <div>
              <div class="text-xs text-gray-500 dark:text-dark-400">余额</div>
              <div class="mt-1 text-sm font-semibold text-gray-900 dark:text-gray-100">
                {{ formatMoney(currentBalanceValue?.balance_cents || 0, currentBalanceValue?.currency || 'USD') }}
              </div>
            </div>
            <div>
              <div class="text-xs text-gray-500 dark:text-dark-400">来源</div>
              <div class="mt-1 text-sm font-medium text-gray-900 dark:text-gray-100">{{ currentBalanceSourceLabel }}</div>
            </div>
            <div>
              <div class="text-xs text-gray-500 dark:text-dark-400">刷新时间</div>
              <div class="mt-1 text-sm font-medium text-gray-900 dark:text-gray-100">{{ formatDateTime(currentBalanceValue?.captured_at) }}</div>
            </div>
            <div>
              <div class="text-xs text-gray-500 dark:text-dark-400">下次刷新</div>
              <div class="mt-1 text-sm font-medium text-gray-900 dark:text-gray-100">{{ formatDateTime(currentBalanceValue?.refresh_after) }}</div>
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
        <button type="button" class="btn btn-secondary" :disabled="currentBalanceLoading || !sessionSupplier" @click="reloadCurrentBalance(true)">
          <Icon name="refresh" size="sm" :class="{ 'animate-spin': currentBalanceLoading }" />
          刷新余额
        </button>
        <button type="button" class="btn btn-primary" :disabled="loggingInSession || !sessionSupplier" @click="loginCurrentSession">
          <Icon name="shield" size="sm" :class="{ 'animate-spin': loggingInSession }" />
          后端直登并读取余额
        </button>
        <button type="button" class="btn btn-primary" :disabled="probingSession || !currentSession?.has_encrypted_bundle" @click="probeCurrentSession">
          <Icon name="beaker" size="sm" :class="{ 'animate-spin': probingSession }" />
          读取余额
        </button>
      </template>
    </BaseDialog>

    <BaseDialog :show="channelStatusDialogOpen" :title="channelStatusSupplier ? `供应商渠道状态 - ${channelStatusSupplier.name}` : '供应商渠道状态'" width="full" @close="closeChannelStatusDialog">
      <div class="min-h-[560px] max-w-full overflow-hidden rounded-b-2xl bg-gradient-to-br from-emerald-50/60 via-white to-sky-50/40 px-5 py-5 dark:from-dark-900 dark:via-dark-900 dark:to-dark-800 sm:px-6">
        <section class="pb-5">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div class="min-w-0 text-xs text-gray-500 dark:text-dark-400">
              <div class="truncate" :title="channelMonitorAPIBaseURL || channelMonitorOrigin || '-'">
                {{ channelMonitorAPIBaseURL || channelMonitorOrigin || '供应商 API 未读取' }}
              </div>
              <div class="mt-1">更新于 {{ formatDateTime(channelMonitorCapturedAt) }}</div>
            </div>

            <div class="flex min-w-0 flex-wrap items-center justify-end gap-3">
              <div role="tablist" class="inline-flex rounded-xl border border-gray-200/60 bg-gray-100 p-0.5 text-xs dark:border-dark-700/60 dark:bg-dark-800">
                <button
                  v-for="option in channelStatusWindowOptions"
                  :key="option.value"
                  type="button"
                  role="tab"
                  :aria-selected="channelStatusWindow === option.value"
                  class="rounded-lg px-3 py-1 transition-colors disabled:cursor-not-allowed disabled:opacity-45"
                  :class="channelStatusWindow === option.value ? 'bg-white font-semibold text-gray-900 shadow-sm dark:bg-dark-700 dark:text-white' : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200'"
                  :disabled="option.disabled"
                  :title="option.disabled ? '供应商明细接口接入后启用' : option.label"
                  @click="channelStatusWindow = option.value"
                >
                  {{ option.label }}
                </button>
              </div>

              <span class="inline-flex items-center rounded-full px-2.5 py-1 text-xs font-semibold uppercase tracking-wider" :class="channelStatusOverallChipClass">
                <span class="mr-1.5 h-1.5 w-1.5 rounded-full animate-pulse" :class="channelStatusOverallDotClass"></span>
                {{ channelStatusOverallLabel }}
              </span>

              <button
                type="button"
                class="flex h-8 w-8 items-center justify-center rounded-lg text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-700 disabled:opacity-50 dark:text-gray-400 dark:hover:bg-dark-700 dark:hover:text-gray-200"
                :disabled="channelStatusLoading || !channelStatusSupplier"
                title="刷新"
                @click="loadChannelStatus"
              >
                <Icon name="refresh" size="md" :class="{ 'animate-spin': channelStatusLoading }" />
              </button>

              <button
                type="button"
                class="inline-flex h-8 items-center gap-1.5 rounded-lg border border-gray-200 bg-white px-2.5 text-xs font-medium text-gray-600 shadow-sm transition-colors hover:bg-gray-50 dark:border-dark-700 dark:bg-dark-800 dark:text-gray-300 dark:hover:bg-dark-700"
                :class="{ 'opacity-60': !channelStatusAutoRefresh }"
                @click="toggleChannelStatusAutoRefresh"
              >
                <Icon name="refresh" size="xs" />
                自动刷新: {{ channelStatusCountdown }}s
              </button>
            </div>
          </div>
        </section>

        <div v-if="channelStatusError" class="mb-5 rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-200">
          {{ channelStatusError }}
        </div>

        <div v-if="channelStatusLoading && channelMonitorItems.length === 0" class="grid gap-5 [grid-template-columns:repeat(auto-fit,minmax(min(100%,18rem),1fr))]">
          <div
            v-for="index in 6"
            :key="index"
            class="min-h-[280px] animate-pulse rounded-2xl border border-gray-200/80 bg-white/70 p-5 dark:border-dark-700/70 dark:bg-dark-800/60"
          >
            <div class="flex items-start gap-3">
              <div class="h-9 w-9 rounded-xl bg-gray-200 dark:bg-dark-700"></div>
              <div class="flex-1 space-y-2">
                <div class="h-4 w-2/3 rounded bg-gray-200 dark:bg-dark-700"></div>
                <div class="h-3 w-1/2 rounded bg-gray-200 dark:bg-dark-700"></div>
              </div>
              <div class="h-6 w-16 rounded-full bg-gray-200 dark:bg-dark-700"></div>
            </div>
            <div class="mt-5 grid grid-cols-2 gap-2">
              <div class="h-16 rounded-xl bg-gray-100 dark:bg-dark-900/40"></div>
              <div class="h-16 rounded-xl bg-gray-100 dark:bg-dark-900/40"></div>
            </div>
            <div class="mt-6 h-5 w-full rounded bg-gray-100 dark:bg-dark-900/40"></div>
          </div>
        </div>

        <EmptyState
          v-else-if="channelMonitorItems.length === 0 && !channelStatusError"
          title="暂无渠道状态"
          description="请确认供应商已启用渠道监控，并且当前供应商会话有权限读取 /api/v1/channel-monitors。"
        />

        <div v-else class="grid gap-5 [grid-template-columns:repeat(auto-fit,minmax(min(100%,18rem),1fr))]">
          <SupplierChannelMonitorCard
            v-for="item in channelMonitorItems"
            :key="item.id"
            :item="item"
            :window="channelStatusWindow"
            :countdown-seconds="channelStatusCountdown"
          />
        </div>
      </div>

      <template #footer>
        <button type="button" class="btn btn-secondary" @click="closeChannelStatusDialog">关闭</button>
      </template>
    </BaseDialog>

    <BaseDialog :show="groupsDialogOpen" :title="groupsSupplier ? `供应商分组 - ${groupsSupplier.name}` : '供应商分组'" width="wide" @close="closeGroupsDialog">
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
            <button type="button" class="btn btn-secondary" :disabled="groupsSyncing || !canSubmitGroupSync" @click="syncCurrentGroups">
              <Icon name="sync" size="sm" :class="{ 'animate-spin': groupsSyncing }" />
              同步分组
            </button>
            <button type="button" class="btn btn-primary" :disabled="keysEnsuring || !canSubmitEnsureKeys" @click="ensureCurrentKeys">
              <Icon name="key" size="sm" :class="{ 'animate-spin': keysEnsuring }" />
              补齐 Key/账号
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

        <div v-if="groupsError" class="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-200">
          {{ groupsError }}
        </div>

        <div v-if="provisionJobError" class="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-200">
          {{ provisionJobError }}
        </div>

        <DataTable
          :columns="groupColumns"
          :data="supplierGroups"
          :loading="groupsLoading"
          row-key="id"
          default-sort-key="last_seen_at"
          default-sort-order="desc"
          :estimate-row-height="72"
        >
          <template #cell-name="{ row }">
            <div class="min-w-[220px]">
              <div class="flex items-center gap-2">
                <GroupBadge
                  :name="row.name"
                  :platform="groupPlatform(row.provider_family)"
                  :rate-multiplier="row.effective_rate_multiplier"
                />
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
                v-if="groupAction(row).kind === 'provision'"
                type="button"
                class="btn btn-secondary btn-sm"
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
                class="btn btn-secondary btn-sm"
                :disabled="groupAction(row).disabled"
                :title="groupAction(row).title"
                @click="openRepairDialog(groupKey(row)!)"
              >
                <Icon :name="groupAction(row).icon" size="sm" />
                {{ groupAction(row).label }}
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

        <div v-if="activeProvisionJob?.job_type === 'provision_group_key'" class="rounded-lg border border-gray-200 bg-gray-50 p-4 text-sm dark:border-dark-700 dark:bg-dark-900/40">
          <div class="flex flex-wrap items-center gap-2">
            <span class="badge" :class="provisionJobStatusClass(activeProvisionJob.status)">{{ provisionJobStatusLabel(activeProvisionJob.status) }}</span>
            <span class="font-medium text-gray-900 dark:text-gray-100">开通任务 #{{ activeProvisionJob.id }}</span>
          </div>
          <div class="mt-2 text-xs text-gray-500 dark:text-dark-400">{{ provisionJobCaption(activeProvisionJob) }}</div>
        </div>
      </form>

      <template #footer>
        <button type="button" class="btn btn-secondary" @click="closeProvisionDialog">取消</button>
        <button type="submit" form="supplier-key-provision-form" class="btn btn-primary" :disabled="provisionSubmitting || activeProvisionJobRunning">
          <Icon name="key" size="sm" :class="{ 'animate-spin': provisionSubmitting }" />
          {{ provisionSubmitting ? '提交中...' : '提交开通任务' }}
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
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import GroupBadge from '@/components/common/GroupBadge.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'
import SupplierChannelMonitorCard from '@/components/admin-plus/SupplierChannelMonitorCard.vue'
import type { Column } from '@/components/common/types'
import type { GroupPlatform } from '@/types'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { useTableSelection } from '@/composables/useTableSelection'
import { useAppStore } from '@/stores/app'
import { extractApiErrorCode } from '@/utils/apiError'
import { supplierGroupAction } from './supplierProvisionPresentation'
import {
  createSupplier,
  deleteSupplier,
  ensureSupplierKeys,
  getSupplierProvisionJob,
  getSupplierCurrentBalance,
  getSupplierSession,
  listSupplierChannelMonitors,
  listLocalSub2APIAccounts,
  listSupplierCostSnapshots,
  listSupplierKeys,
  listSupplierGroups,
  listSuppliers,
  loginSupplierSession,
  probeSupplierSession,
  provisionSupplierKey,
  repairSupplierKeyBinding,
  syncSupplierGroups,
  updateSupplier,
  updateSupplierStatus,
  type LocalSub2APIAccount,
  type Supplier,
  type SupplierBrowserSession,
  type SupplierChannelMonitorView,
  type SupplierCostSnapshot,
  type SupplierCurrentBalance,
  type SupplierGroup,
  type SupplierGroupStatus,
  type SupplierHealthStatus,
  type SupplierKey,
  type SupplierKeyStatus,
  type SupplierProvisionJob,
  type SupplierProvisionJobType,
  type SupplierProvisionStatus,
  type SupplierSessionProbeResult,
  type SupplierKind,
  type SupplierMonitorStatus,
  type SupplierRuntimeStatus,
  type SupplierType
} from '@/api/admin/adminPlus'

const appStore = useAppStore()
const route = useRoute()
const handledDeepLinkKey = ref('')

const loading = ref(false)
const submitting = ref(false)
const statusSubmitting = ref(false)
const provisionSubmitting = ref(false)
const repairSubmitting = ref(false)
const editorOpen = ref(false)
const statusDialogOpen = ref(false)
const sessionDialogOpen = ref(false)
const channelStatusDialogOpen = ref(false)
const groupsDialogOpen = ref(false)
const provisionDialogOpen = ref(false)
const repairDialogOpen = ref(false)
const deleteDialogOpen = ref(false)
const moreMenuOpen = ref(false)
const bulkStatusMode = ref(false)
const bulkDeleteMode = ref(false)
const editingSupplier = ref<Supplier | null>(null)
const sessionSupplier = ref<Supplier | null>(null)
const channelStatusSupplier = ref<Supplier | null>(null)
const groupsSupplier = ref<Supplier | null>(null)
const provisionGroup = ref<SupplierGroup | null>(null)
const repairKey = ref<SupplierKey | null>(null)
const deletingSupplier = ref<Supplier | null>(null)
const rowActionsMenuSupplier = ref<Supplier | null>(null)
const rowActionsMenuStyle = ref<Record<string, string>>({})
const suppliers = ref<Supplier[]>([])
const supplierGroups = ref<SupplierGroup[]>([])
const supplierKeys = ref<SupplierKey[]>([])
const supplierCostSnapshots = ref<Record<number, SupplierCostSnapshot | undefined>>({})
const activeProvisionJob = ref<SupplierProvisionJob | null>(null)
const localAccounts = ref<LocalSub2APIAccount[]>([])
const channelMonitorItems = ref<SupplierChannelMonitorView[]>([])
const sessionStore = reactive<Record<number, SupplierBrowserSession | undefined>>({})
const currentBalanceStore = reactive<Record<number, SupplierCurrentBalance | undefined>>({})
const sessionLoading = ref(false)
const loggingInSession = ref(false)
const probingSession = ref(false)
const currentBalanceLoading = ref(false)
const channelStatusLoading = ref(false)
const groupsLoading = ref(false)
const groupsSyncing = ref(false)
const keysEnsuring = ref(false)
const repairAccountsLoading = ref(false)
const sessionLoadError = ref('')
const channelStatusError = ref('')
const channelMonitorCapturedAt = ref('')
const channelMonitorOrigin = ref('')
const channelMonitorAPIBaseURL = ref('')
const channelStatusWindow = ref<'7d' | '15d' | '30d'>('7d')
const channelStatusAutoRefresh = ref(true)
const channelStatusCountdown = ref(16)
const groupsError = ref('')
const provisionError = ref('')
const provisionJobError = ref('')
const repairError = ref('')
const lastProbe = ref<SupplierSessionProbeResult | null>(null)
const rowLoginSupplierID = ref<number | null>(null)
let provisionJobTimer: ReturnType<typeof window.setTimeout> | undefined
let channelStatusAutoRefreshTimer: ReturnType<typeof window.setInterval> | undefined

const ROW_ACTIONS_MENU_WIDTH = 224
const ROW_ACTIONS_MENU_HEIGHT = 248
const ROW_ACTIONS_MENU_MARGIN = 8

const filters = reactive({
  q: typeof route.query.q === 'string' ? route.query.q : '',
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
  { key: 'cost', label: '成本概览', class: 'text-right' },
  { key: 'status', label: '状态' },
  { key: 'kind_type', label: '归类 / 类型' },
  { key: 'credential', label: '采集凭据' },
  { key: 'session', label: '供应商会话' },
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

const currentBalanceValue = computed(() => {
  const supplierID = sessionSupplier.value?.id
  return supplierID ? currentBalanceStore[supplierID] : undefined
})

const currentBalanceBadgeText = computed(() => {
  const balance = currentBalanceValue.value
  if (!balance) return '未读取'
  if (balance.fallback) return '兜底值'
  if (balance.expired) return '已过期'
  if (balance.stale) return '待刷新'
  return '最新'
})

const currentBalanceBadgeClass = computed(() => {
  const balance = currentBalanceValue.value
  if (!balance || balance.fallback || balance.expired) return 'badge-danger'
  if (balance.stale) return 'badge-warning'
  return 'badge-success'
})

const currentBalanceCaption = computed(() => {
  const balance = currentBalanceValue.value
  if (!balance) return '打开后读取 Redis 当前值，必要时可手动刷新'
  if (balance.fallback) return balance.refresh_error_message || '读取失败，暂按 0 处理'
  if (balance.stale) return '缓存已到刷新窗口，后台会重新获取'
  return '缓存有效，调度器会周期刷新'
})

const currentBalanceSourceLabel = computed(() => {
  const source = currentBalanceValue.value?.source
  if (source === 'provider_session') return '供应商会话'
  if (source === 'fallback') return '兜底'
  return source || '-'
})

const currentGroupSession = computed(() => {
  const supplierID = groupsSupplier.value?.id
  return supplierID ? sessionStore[supplierID] : undefined
})

const activeProvisionJobRunning = computed(() => {
  const status = activeProvisionJob.value?.status
  return status === 'queued' || status === 'running' || status === 'retryable_failed'
})

const groupWorkflowSteps = computed(() => {
  const hasSession = Boolean(currentGroupSession.value?.has_encrypted_bundle)
  const hasGroups = supplierGroups.value.length > 0
  const hasKeys = supplierKeys.value.length > 0
  const job = activeProvisionJob.value
  return [
    {
      key: 'session',
      label: '会话',
      status: hasSession ? 'succeeded' : 'manual_required',
      caption: hasSession ? sessionSourceLabel(currentGroupSession.value?.session_source) : '先直登或插件上报'
    },
    {
      key: 'groups',
      label: '分组',
      status: job?.job_type === 'sync_groups' ? job.status : (hasGroups ? 'succeeded' : 'queued'),
      caption: hasGroups ? `${supplierGroups.value.length} 个分组` : '待同步'
    },
    {
      key: 'keys',
      label: 'Key',
      status: job?.job_type === 'provision_all_group_keys' || job?.job_type === 'provision_group_key' ? job.status : (hasKeys ? 'succeeded' : 'queued'),
      caption: hasKeys ? `${supplierKeys.value.length} 个 Key` : '按分组补齐'
    },
    {
      key: 'verify',
      label: '验证',
      status: hasKeys ? 'succeeded' : 'queued',
      caption: hasKeys ? '可查看绑定' : '待任务完成'
    }
  ] as Array<{ key: string; label: string; status: SupplierProvisionStatus; caption: string }>
})

const canSubmitGroupSync = computed(() => {
  return Boolean(groupsSupplier.value && currentGroupSession.value?.has_encrypted_bundle && !activeProvisionJobRunning.value && !groupsLoading.value)
})

const canSubmitEnsureKeys = computed(() => {
  return Boolean(groupsSupplier.value && supplierGroups.value.length > 0 && !activeProvisionJobRunning.value && !groupsLoading.value)
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
    { key: 'can_read_usage_costs', label: '用量消耗', enabled: Boolean(capabilities.can_read_usage_costs) }
  ]
})

const channelStatusOverall = computed<SupplierMonitorStatus>(() => {
  if (channelMonitorItems.value.length === 0) return 'operational'
  if (channelMonitorItems.value.some((item) => item.primary_status === 'failed' || item.primary_status === 'error')) return 'failed'
  if (channelMonitorItems.value.some((item) => item.primary_status !== 'operational')) return 'degraded'
  return 'operational'
})

const channelStatusWindowOptions = computed<Array<{ value: '7d' | '15d' | '30d'; label: string; disabled: boolean }>>(() => [
  { value: '7d', label: '7 天', disabled: false },
  { value: '15d', label: '15 天', disabled: true },
  { value: '30d', label: '30 天', disabled: true }
])

const channelStatusOverallLabel = computed(() => {
  return channelStatusOverall.value === 'operational' ? 'OPERATIONAL' : 'DEGRADED'
})

const channelStatusOverallChipClass = computed(() => {
  if (channelStatusOverall.value === 'operational') {
    return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300'
  }
  return 'bg-amber-100 text-amber-700 dark:bg-amber-500/15 dark:text-amber-300'
})

const channelStatusOverallDotClass = computed(() => {
  if (channelStatusOverall.value === 'operational') return 'bg-emerald-500'
  return 'bg-amber-500'
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

function supplierLinkURL(supplier: Supplier): string {
  return supplier.dashboard_url?.trim() || supplier.api_base_url?.trim() || ''
}

function supplierNameTitle(supplier: Supplier): string {
  const url = supplierLinkURL(supplier)
  return url ? `${supplier.name} · ${url}` : supplier.name
}

function formatMultiplier(value?: number | null): string {
  if (typeof value !== 'number') return '-'
  if (!Number.isFinite(value)) return '-'
  return `${value.toFixed(4).replace(/\.?0+$/, '')}x`
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

function groupPlatform(value?: string): GroupPlatform {
  const provider = (value || '').toLowerCase()
  if (provider.includes('anthropic') || provider.includes('claude')) return 'anthropic'
  if (provider.includes('gemini') || provider.includes('google')) return 'gemini'
  if (provider.includes('openai') || provider.includes('gpt')) return 'openai'
  return 'antigravity'
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

function supplierCostSnapshot(supplierID: number): SupplierCostSnapshot | undefined {
  return supplierCostSnapshots.value[supplierID]
}

function costDeltaLabel(supplierID: number): string {
  const snapshot = supplierCostSnapshot(supplierID)
  if (!snapshot || snapshot.balance_delta_cents === null || snapshot.balance_delta_cents === undefined) return '-'
  return formatMoney(snapshot.balance_delta_cents, snapshot.currency)
}

function costDeltaClass(supplierID: number): string {
  const delta = supplierCostSnapshot(supplierID)?.balance_delta_cents
  if (delta === null || delta === undefined || delta === 0) return 'text-emerald-600 dark:text-emerald-400'
  return 'text-rose-600 dark:text-rose-400'
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

function sessionSourceLabel(source?: SupplierBrowserSession['session_source']): string {
  if (source === 'direct_login') return '后端直登'
  if (source === 'browser_extension') return 'Chrome 兜底'
  if (source === 'manual_import') return '手动导入'
  return '-'
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

function provisionJobTypeLabel(type?: SupplierProvisionJobType): string {
  if (type === 'sync_groups') return '同步分组'
  if (type === 'provision_group_key') return '开通单组 Key/账号'
  if (type === 'provision_all_group_keys') return '补齐全部 Key/账号'
  if (type === 'repair_binding') return '修复绑定'
  return '供应商任务'
}

function provisionJobStatusLabel(status?: SupplierProvisionStatus): string {
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

function provisionJobStatusClass(status?: SupplierProvisionStatus): string {
  if (status === 'succeeded') return 'badge-success'
  if (status === 'running' || status === 'queued') return 'badge-primary'
  if (status === 'retryable_failed' || status === 'manual_required' || status === 'partial_succeeded') return 'badge-warning'
  if (status === 'dead' || status === 'cancelled') return 'badge-danger'
  return 'badge-gray'
}

function workflowStepDotClass(status: SupplierProvisionStatus): string {
  if (status === 'succeeded') return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200'
  if (status === 'running' || status === 'retryable_failed') return 'bg-primary-100 text-primary-700 dark:bg-primary-900/40 dark:text-primary-200'
  if (status === 'manual_required' || status === 'dead') return 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-200'
  return 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-dark-300'
}

function provisionJobCaption(job: SupplierProvisionJob): string {
  if (job.error_message) return job.error_message
  if (job.status === 'succeeded') return '任务完成，列表会自动刷新。'
  if (job.status === 'retryable_failed') return `第 ${job.attempts}/${job.max_attempts} 次失败，等待重试。`
  if (job.status === 'running') return '服务端正在执行，请稍候。'
  if (job.status === 'queued') return '任务已进入队列。'
  return `步骤 ${job.succeeded_steps}/${Math.max(job.total_steps, 1)}`
}

function middleEllipsis(value?: string | null, maxLength = 34): string {
  const text = String(value || '').trim()
  if (text.length <= maxLength) return text
  const keep = Math.max(4, Math.floor((maxLength - 1) / 2))
  const head = text.slice(0, keep)
  const tail = text.slice(text.length - (maxLength - keep - 1))
  return `${head}…${tail}`
}

function splitMiddleEllipsis(value?: string | null, maxLength = 24): { head: string; tail: string; ellipsized: boolean } {
  const text = String(value || '').trim()
  if (text.length <= maxLength) {
    return { head: text, tail: '', ellipsized: false }
  }
  const headLength = Math.max(8, Math.floor((maxLength - 3) * 0.58))
  const tailLength = Math.max(6, maxLength - 3 - headLength)
  return {
    head: text.slice(0, headLength),
    tail: text.slice(text.length - tailLength),
    ellipsized: true
  }
}

function groupKey(group: SupplierGroup): SupplierKey | undefined {
  return supplierKeysByGroupID.value.get(group.id)
}

function groupAction(group: SupplierGroup) {
  return supplierGroupAction(group, groupKey(group))
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

function sessionHasAccessToken(session?: SupplierBrowserSession): boolean {
  return Boolean(session?.session_summary?.has_access_token)
}

function hasConfiguredDirectLoginCredential(supplier: Supplier): boolean {
  return supplier.credential.browser_login_token_configured ||
    (supplier.credential.browser_login_username_configured && supplier.credential.browser_login_password_configured)
}

function needsDirectLoginCredential(supplier: Supplier): boolean {
  return supplier.credential.browser_login_enabled && !hasConfiguredDirectLoginCredential(supplier)
}

function shouldShowTokenBadge(supplier: Supplier): boolean {
  return supplier.credential.browser_login_token_configured || Boolean(sessionStore[supplier.id]?.session_summary)
}

function credentialTokenBadgeText(supplier: Supplier): string {
  const session = sessionStore[supplier.id]
  if (sessionHasAccessToken(session) && session?.status === 'valid') return 'Token 有效'
  if (sessionHasAccessToken(session) && session?.status === 'expired') return 'Token 过期'
  if (session?.session_summary) return 'Token 缺失'
  if (supplier.credential.browser_login_token_configured) return 'Token 未验证'
  return 'Token 未配置'
}

function credentialTokenBadgeClass(supplier: Supplier): string {
  const session = sessionStore[supplier.id]
  if (sessionHasAccessToken(session) && session?.status === 'valid') return 'badge-success'
  if (sessionHasAccessToken(session) && session?.status === 'expired') return 'badge-warning'
  if (session?.session_summary) return 'badge-danger'
  if (supplier.credential.browser_login_token_configured) return 'badge-primary'
  return 'badge-gray'
}

function credentialTokenBadgeTitle(supplier: Supplier): string {
  const session = sessionStore[supplier.id]
  if (sessionHasAccessToken(session) && session?.status === 'valid') return '当前会话摘要包含 Access Token，且会话未过期'
  if (sessionHasAccessToken(session) && session?.status === 'expired') return '当前会话摘要包含 Access Token，但会话已过期，请重新一键登录或使用 Chrome 插件'
  if (session?.session_summary) return '当前会话摘要没有 Access Token，请重新一键登录或使用 Chrome 插件'
  if (supplier.credential.browser_login_token_configured) return '已保存临时 Token，但尚未形成有效供应商会话'
  return '未配置临时 Token'
}

function oneClickLoginTitle(supplier: Supplier): string {
  const preflightError = directLoginPreflightError(supplier)
  if (preflightError) return preflightError
  if (supplier.type !== 'sub2api') return '当前后端直登仅支持 Sub2API 供应商'
  if (rowLoginSupplierID.value === supplier.id) return '正在登录'
  return '使用已保存凭据后端直登并读取余额'
}

function directLoginPreflightError(supplier: Supplier): string {
  if (supplier.type !== 'sub2api') return '当前一键登录仅支持 Sub2API 供应商'
  if (!supplier.credential.browser_login_enabled) return '未启用登录凭据，请先编辑供应商启用并配置账号密码或临时 Token'
  if (!hasConfiguredDirectLoginCredential(supplier)) return '未配置登录账号密码或临时 Token，请先编辑供应商补齐凭据'
  if (!supplier.dashboard_url && !supplier.api_base_url) return '未配置后台地址或 API Base URL，请先编辑供应商补齐地址'
  return ''
}

function directLoginErrorMessage(error: unknown): string {
  const code = extractApiErrorCode(error)
  if (code && ['SUPPLIER_DIRECT_LOGIN_UPSTREAM_ORIGIN_ERROR', 'SUPPLIER_DIRECT_LOGIN_UPSTREAM_HTML', 'SUPPLIER_DIRECT_LOGIN_SETTINGS_BAD_STATUS', 'SUPPLIER_DIRECT_LOGIN_BAD_STATUS'].includes(code)) {
    return '供应商站点返回源站或前置层异常，后端直登失败；请稍后重试，或使用 Chrome 插件采集会话'
  }
  if (code && ['LOGIN_CAPTCHA_REQUIRED', 'LOGIN_MFA_REQUIRED', 'BROWSER_FALLBACK_REQUIRED'].includes(code)) {
    return '供应商登录需要验证码、2FA 或浏览器上下文，请使用 Chrome 插件采集会话'
  }
  if (code === 'SUPPLIER_DIRECT_LOGIN_ADMIN_REQUIRED') {
    return '供应商启用了后台模式，后端直登需要供应商管理员账号'
  }
  if (code === '429') {
    return '供应商登录限流，请稍后重试'
  }
  if (code === 'SUPPLIER_BROWSER_CREDENTIAL_DECRYPT_FAILED') {
    return '已保存的供应商登录凭据无法解密，请重新编辑并保存账号密码；修复版本保存后重启不会再次失效'
  }
  if (code && ['SUPPLIER_DIRECT_LOGIN_CREDENTIAL_REQUIRED', 'SUPPLIER_BROWSER_CREDENTIAL_REQUIRED', 'SUPPLIER_BROWSER_LOGIN_DISABLED'].includes(code)) {
    return '供应商未配置可用登录凭据，请先编辑供应商补齐账号密码或临时 Token'
  }
  if (code === 'SUPPLIER_DASHBOARD_URL_REQUIRED') {
    return '供应商未配置后台地址，请先编辑供应商补齐地址'
  }
  const rawMessage = (error as { message?: string }).message || ''
  const normalizedMessage = rawMessage.toLowerCase()
  if (normalizedMessage.includes('cloudflare') || normalizedMessage.includes('origin web server') || normalizedMessage.includes('invalid or incomplete response')) {
    return '供应商站点返回源站或前置层异常，后端直登失败；请稍后重试，或使用 Chrome 插件采集会话'
  }
  return (error as { message?: string }).message || '后端直登失败'
}

function channelStatusErrorMessage(error: unknown): string {
  const code = extractApiErrorCode(error)
  const status = (error as { status?: number })?.status
  if ((status === 404 || code === '404') && !((error as { reason?: string })?.reason)) {
    return 'Admin Plus 后端服务还没有加载供应商渠道状态路由，请确认已发布并重启到包含 /api/v1/admin-plus/suppliers/:id/channel-monitors 的版本。'
  }
  if (code && ['SUPPLIER_SESSION_NOT_FOUND', 'SUPPLIER_SESSION_EXPIRED', 'SUPPLIER_SESSION_DECRYPT_FAILED'].includes(code)) {
    return '当前供应商还没有可用会话，请先一键登录，或使用 Chrome 插件采集供应商会话后再读取渠道状态。'
  }
  if (code === 'SUPPLIER_SESSION_PERMISSION_DENIED') {
    return '当前供应商会话无权限读取渠道状态，请重新一键登录或使用 Chrome 插件采集最新会话。'
  }
  if (code === 'SUPPLIER_SESSION_BASE_URL_REQUIRED') {
    return '供应商未配置后台地址或 API Base URL，请先编辑供应商补齐地址。'
  }
  if (code && ['SUPPLIER_SESSION_API_BASE_URL_INVALID', 'SUPPLIER_SESSION_ORIGIN_INVALID', 'SUPPLIER_SESSION_URL_INVALID', 'SUPPLIER_SESSION_HOST_NOT_ALLOWED'].includes(code)) {
    return '供应商会话地址与供应商配置不匹配，请检查后台地址和 API Base URL。'
  }
  if (code && ['SUPPLIER_SESSION_REQUEST_FAILED', 'SUPPLIER_SESSION_BAD_STATUS', 'SUPPLIER_CHANNEL_MONITORS_RESPONSE_INVALID'].includes(code)) {
    return '供应商渠道状态接口返回异常，请确认该供应商已部署支持 /api/v1/channel-monitors 的 Sub2API 版本。'
  }
  return (error as { message?: string }).message || '加载供应商渠道状态失败'
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
    const [result, costResult] = await Promise.all([
      listSuppliers({
        q: filters.q || undefined,
        kind: filters.kind || undefined,
        type: filters.type || undefined,
        runtime_status: filters.runtime_status || undefined,
        health_status: filters.health_status || undefined,
        page: pagination.page,
        page_size: pagination.page_size
      }),
      listSupplierCostSnapshots({ page: 1, page_size: 200 })
    ])
    suppliers.value = result.items
    supplierCostSnapshots.value = Object.fromEntries(costResult.items.map((item) => [item.supplier_id, item]))
    pagination.total = result.total || 0
    pagination.pages = result.pages || 0
    pagination.page = result.page || pagination.page
    pagination.page_size = result.page_size || pagination.page_size
    void preloadVisibleSessions()
    openDeepLinkedDialog()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '加载供应商失败')
  } finally {
    loading.value = false
  }
}

function openDeepLinkedDialog() {
  if (route.query.open !== 'groups') return
  const supplierID = Number(route.query.supplier_id || 0)
  if (!supplierID) return
  const deepLinkKey = `${route.query.open}:${supplierID}`
  if (handledDeepLinkKey.value === deepLinkKey) return
  const supplier = suppliers.value.find((item) => item.id === supplierID)
  if (!supplier) return
  handledDeepLinkKey.value = deepLinkKey
  openGroupsDialog(supplier)
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

async function reloadCurrentBalance(refresh: boolean) {
  if (!sessionSupplier.value) return
  currentBalanceLoading.value = true
  try {
    const balance = await getSupplierCurrentBalance(sessionSupplier.value.id, { refresh })
    currentBalanceStore[sessionSupplier.value.id] = balance
    if (refresh) {
      await loadSuppliers()
    }
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '读取当前余额失败')
  } finally {
    currentBalanceLoading.value = false
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

async function loadChannelStatus() {
  if (!channelStatusSupplier.value) return
  if (channelStatusSupplier.value.type !== 'sub2api') {
    channelStatusError.value = '当前仅支持读取 Sub2API 类型供应商的渠道状态。'
    channelMonitorItems.value = []
    return
  }
  channelStatusLoading.value = true
  channelStatusError.value = ''
  try {
    const result = await listSupplierChannelMonitors(channelStatusSupplier.value.id)
    channelMonitorItems.value = result.items || []
    channelMonitorCapturedAt.value = result.captured_at || ''
    channelMonitorOrigin.value = result.origin || ''
    channelMonitorAPIBaseURL.value = result.api_base_url || ''
    channelStatusCountdown.value = 16
  } catch (error) {
    channelMonitorItems.value = []
    channelStatusError.value = channelStatusErrorMessage(error)
  } finally {
    channelStatusLoading.value = false
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
  closeRowActionsMenu()
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
  closeRowActionsMenu()
  bulkStatusMode.value = false
  statusForm.id = supplier.id
  statusForm.name = supplier.name
  statusForm.runtime_status = supplier.runtime_status
  statusForm.health_status = supplier.health_status
  statusDialogOpen.value = true
}

function openSessionDialog(supplier: Supplier) {
  closeRowActionsMenu()
  sessionSupplier.value = supplier
  lastProbe.value = null
  sessionLoadError.value = ''
  sessionDialogOpen.value = true
  void Promise.all([reloadCurrentSession(), reloadCurrentBalance(false)])
}

function openChannelStatusDialog(supplier: Supplier) {
  closeRowActionsMenu()
  channelStatusSupplier.value = supplier
  channelMonitorItems.value = []
  channelMonitorCapturedAt.value = ''
  channelMonitorOrigin.value = ''
  channelMonitorAPIBaseURL.value = ''
  channelStatusError.value = ''
  channelStatusWindow.value = '7d'
  channelStatusCountdown.value = 16
  channelStatusDialogOpen.value = true
  startChannelStatusAutoRefresh()
  void loadChannelStatus()
}

function closeChannelStatusDialog() {
  channelStatusDialogOpen.value = false
  stopChannelStatusAutoRefresh()
}

function toggleChannelStatusAutoRefresh() {
  channelStatusAutoRefresh.value = !channelStatusAutoRefresh.value
  if (channelStatusAutoRefresh.value) {
    channelStatusCountdown.value = 16
    startChannelStatusAutoRefresh()
  }
}

function startChannelStatusAutoRefresh() {
  stopChannelStatusAutoRefresh()
  channelStatusAutoRefreshTimer = window.setInterval(() => {
    if (!channelStatusDialogOpen.value || !channelStatusAutoRefresh.value) return
    channelStatusCountdown.value = Math.max(0, channelStatusCountdown.value - 1)
    if (channelStatusCountdown.value === 0 && !channelStatusLoading.value) {
      void loadChannelStatus()
    }
  }, 1000)
}

function stopChannelStatusAutoRefresh() {
  if (channelStatusAutoRefreshTimer) {
    window.clearInterval(channelStatusAutoRefreshTimer)
    channelStatusAutoRefreshTimer = undefined
  }
}

function openGroupsDialog(supplier: Supplier) {
  closeRowActionsMenu()
  stopProvisionJobPolling()
  groupsSupplier.value = supplier
  supplierGroups.value = []
  supplierKeys.value = []
  activeProvisionJob.value = null
  groupsError.value = ''
  provisionJobError.value = ''
  groupPagination.page = 1
  groupFilters.q = ''
  groupFilters.status = ''
  groupsDialogOpen.value = true
  void Promise.all([reloadGroupSession(), loadCurrentGroups()])
}

function closeGroupsDialog() {
  groupsDialogOpen.value = false
  stopProvisionJobPolling()
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
    await Promise.all([reloadCurrentSession(), reloadCurrentBalance(false), loadSuppliers()])
  } catch (error) {
    sessionLoadError.value = (error as { message?: string }).message || '会话探测失败'
    appStore.showError(sessionLoadError.value)
  } finally {
    probingSession.value = false
  }
}

async function loginCurrentSession() {
  if (!sessionSupplier.value) return
  const preflightError = directLoginPreflightError(sessionSupplier.value)
  if (preflightError) {
    sessionLoadError.value = preflightError
    appStore.showError(preflightError)
    return
  }
  loggingInSession.value = true
  sessionLoadError.value = ''
  try {
    await directLoginSupplier(sessionSupplier.value, {
      updateLastProbe: true,
      successMessage: '后端直登完成'
    })
    await Promise.all([reloadCurrentBalance(false), loadSuppliers()])
  } catch (error) {
    sessionLoadError.value = directLoginErrorMessage(error)
    appStore.showError(sessionLoadError.value)
  } finally {
    loggingInSession.value = false
  }
}

async function loginSupplierFromRow(supplier: Supplier) {
  const preflightError = directLoginPreflightError(supplier)
  if (preflightError) {
    appStore.showError(preflightError)
    return
  }
  rowLoginSupplierID.value = supplier.id
  try {
    await directLoginSupplier(supplier, {
      updateLastProbe: sessionSupplier.value?.id === supplier.id,
      successMessage: '一键登录完成'
    })
    await loadSuppliers()
  } catch (error) {
    appStore.showError(directLoginErrorMessage(error))
  } finally {
    rowLoginSupplierID.value = null
  }
}

async function directLoginSupplier(supplier: Supplier, options: { updateLastProbe: boolean; successMessage: string }) {
  const result = await loginSupplierSession(supplier.id, {
    record_balance_snapshot: true
  })
  sessionStore[supplier.id] = result.session
  if (options.updateLastProbe) {
    lastProbe.value = result.probe || null
  }
  appStore.showSuccess(result.probe ? `${options.successMessage}，已读取供应商余额` : options.successMessage)
  return result
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
    appStore.showError('当前供应商还没有可用会话，请先后端直登或通过插件兜底上报')
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
    const job = await provisionSupplierKey(groupsSupplier.value.id, {
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
    appStore.showSuccess(`开通任务已提交 #${job.job_id}`)
    await watchProvisionJob(job.job_id)
    closeProvisionDialog()
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
  provisionJobError.value = ''
  try {
    const job = await syncSupplierGroups(groupsSupplier.value.id)
    appStore.showSuccess(`同步分组任务已提交 #${job.job_id}`)
    await watchProvisionJob(job.job_id)
  } catch (error) {
    groupsError.value = (error as { message?: string }).message || '同步分组任务提交失败'
    appStore.showError(groupsError.value)
  } finally {
    groupsSyncing.value = false
  }
}

async function ensureCurrentKeys() {
  if (!groupsSupplier.value) return
  keysEnsuring.value = true
  groupsError.value = ''
  provisionJobError.value = ''
  try {
    const supplier = groupsSupplier.value
    const job = await ensureSupplierKeys(supplier.id, {
      local_account_base_url: defaultProviderBaseURL(supplier),
      local_account_concurrency: 2,
      local_account_priority: 100,
      runtime_status: 'monitor_only',
      health_status: 'normal',
      balance_threshold_cents: 0,
      balance_cents: Math.max(0, supplier.balance_cents || 0),
      balance_currency: supplier.balance_currency || 'USD'
    })
    appStore.showSuccess(`补齐 Key/账号任务已提交 #${job.job_id}`)
    await watchProvisionJob(job.job_id)
  } catch (error) {
    groupsError.value = (error as { message?: string }).message || '补齐 Key/账号任务提交失败'
    appStore.showError(groupsError.value)
  } finally {
    keysEnsuring.value = false
  }
}

async function watchProvisionJob(jobID: number) {
  stopProvisionJobPolling()
  await refreshProvisionJob(jobID)
}

async function refreshProvisionJob(jobID: number) {
  try {
    const job = await getSupplierProvisionJob(jobID)
    activeProvisionJob.value = job
    provisionJobError.value = ''
    if (isTerminalProvisionJobStatus(job.status)) {
      groupPagination.page = 1
      await Promise.all([reloadGroupSession(), loadCurrentGroups(), loadSuppliers()])
      return
    }
    provisionJobTimer = window.setTimeout(() => {
      void refreshProvisionJob(jobID)
    }, 2000)
  } catch (error) {
    provisionJobError.value = (error as { message?: string }).message || '读取任务状态失败'
  }
}

function stopProvisionJobPolling() {
  if (provisionJobTimer) {
    window.clearTimeout(provisionJobTimer)
    provisionJobTimer = undefined
  }
}

function isTerminalProvisionJobStatus(status: SupplierProvisionStatus): boolean {
  return ['succeeded', 'partial_succeeded', 'manual_required', 'dead', 'cancelled'].includes(status)
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
  closeRowActionsMenu()
  bulkDeleteMode.value = false
  deletingSupplier.value = supplier
  deleteDialogOpen.value = true
}

function toggleRowActionsMenu(supplier: Supplier, event: MouseEvent) {
  if (rowActionsMenuSupplier.value?.id === supplier.id) {
    closeRowActionsMenu()
    return
  }
  const target = event.currentTarget as HTMLElement | null
  const rect = target?.getBoundingClientRect()
  rowActionsMenuSupplier.value = supplier
  rowActionsMenuStyle.value = positionRowActionsMenu(rect)
}

function positionRowActionsMenu(rect?: DOMRect): Record<string, string> {
  if (!rect || typeof window === 'undefined') {
    return {
      top: '96px',
      left: `${ROW_ACTIONS_MENU_MARGIN}px`,
      width: `${ROW_ACTIONS_MENU_WIDTH}px`
    }
  }

  const maxLeft = Math.max(ROW_ACTIONS_MENU_MARGIN, window.innerWidth - ROW_ACTIONS_MENU_WIDTH - ROW_ACTIONS_MENU_MARGIN)
  const left = Math.min(Math.max(rect.right - ROW_ACTIONS_MENU_WIDTH, ROW_ACTIONS_MENU_MARGIN), maxLeft)
  const belowTop = rect.bottom + ROW_ACTIONS_MENU_MARGIN
  const aboveTop = rect.top - ROW_ACTIONS_MENU_HEIGHT - ROW_ACTIONS_MENU_MARGIN
  const hasSpaceBelow = belowTop + ROW_ACTIONS_MENU_HEIGHT <= window.innerHeight - ROW_ACTIONS_MENU_MARGIN
  const top = hasSpaceBelow ? belowTop : Math.max(ROW_ACTIONS_MENU_MARGIN, aboveTop)

  return {
    top: `${top}px`,
    left: `${left}px`,
    width: `${ROW_ACTIONS_MENU_WIDTH}px`
  }
}

function closeRowActionsMenu() {
  rowActionsMenuSupplier.value = null
  rowActionsMenuStyle.value = {}
}

function handleRowActionsOutsideClick(event: MouseEvent) {
  if (!rowActionsMenuSupplier.value) return
  const target = event.target as HTMLElement | null
  if (!target) return
  if (target.closest('[data-supplier-row-actions-menu]') || target.closest('[data-supplier-row-actions-trigger]')) return
  closeRowActionsMenu()
}

function mountRowActionsMenuListeners() {
  document.addEventListener('click', handleRowActionsOutsideClick)
  window.addEventListener('resize', closeRowActionsMenu)
  window.addEventListener('scroll', closeRowActionsMenu, true)
}

function unmountRowActionsMenuListeners() {
  document.removeEventListener('click', handleRowActionsOutsideClick)
  window.removeEventListener('resize', closeRowActionsMenu)
  window.removeEventListener('scroll', closeRowActionsMenu, true)
}

function openRowStatusDialog() {
  const supplier = rowActionsMenuSupplier.value
  if (!supplier) return
  openStatusDialog(supplier)
}

function openRowSessionDialog() {
  const supplier = rowActionsMenuSupplier.value
  if (!supplier) return
  openSessionDialog(supplier)
}

function openRowGroupsDialog() {
  const supplier = rowActionsMenuSupplier.value
  if (!supplier) return
  openGroupsDialog(supplier)
}

function openRowChannelStatusDialog() {
  const supplier = rowActionsMenuSupplier.value
  if (!supplier) return
  openChannelStatusDialog(supplier)
}

function openRowDeleteDialog() {
  const supplier = rowActionsMenuSupplier.value
  if (!supplier) return
  openDeleteDialog(supplier)
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

function cleanupTimers() {
  stopProvisionJobPolling()
  stopChannelStatusAutoRefresh()
  closeRowActionsMenu()
  unmountRowActionsMenuListeners()
}

onMounted(() => {
  mountRowActionsMenuListeners()
  void loadSuppliers()
})
onBeforeUnmount(cleanupTimers)
</script>

<style scoped>
.menu-item {
  @apply flex w-full items-center gap-3 rounded-md px-3 py-2 text-sm text-gray-700 transition-colors hover:bg-gray-100 disabled:cursor-not-allowed disabled:opacity-50 dark:text-gray-200 dark:hover:bg-gray-700;
}

.menu-icon {
  @apply flex h-8 w-8 items-center justify-center rounded-md;
}

.row-action-menu-item {
  @apply flex w-full items-center gap-3 rounded-md px-3 py-2 text-sm text-gray-700 transition-colors hover:bg-gray-100 dark:text-gray-200 dark:hover:bg-gray-700;
}

.row-action-menu-icon {
  @apply flex h-8 w-8 shrink-0 items-center justify-center rounded-md;
}
</style>
