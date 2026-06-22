<template>
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
          <span v-if="sessionSummaryString('provider_type')" class="badge badge-primary">Provider {{ sessionSummaryString('provider_type') }}</span>
          <span v-if="currentSessionSummary.has_new_api_user_header" class="badge badge-success">New-Api-User</span>
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
          <div class="mt-1 text-sm font-semibold" :class="currentBalanceAmountClass">
            {{ currentBalanceAmountText }}
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
            {{ formatBalanceMoney(lastProbe.balance_cents || 0, lastProbe.balance_currency || 'USD') }}
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


</template>

<script setup lang="ts">
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
const props = defineProps<{ vm: any }>()
const {
  sessionDialogOpen,
  sessionSupplier,
  sessionLoading,
  loggingInSession,
  probingSession,
  currentBalanceLoading,
  sessionLoadError,
  lastProbe,
  currentSession,
  currentBalanceValue,
  currentBalanceBadgeText,
  currentBalanceBadgeClass,
  currentBalanceCaption,
  currentBalanceSourceLabel,
  currentBalanceAmountText,
  currentBalanceAmountClass,
  currentSessionSummary,
  summaryCookieCount,
  capabilityBadges,
  formatBalanceMoney,
  formatDateTime,
  sessionStatusLabel,
  sessionStatusClass,
  sessionSourceLabel,
  summaryBoolClass,
  sessionSummaryString,
  reloadCurrentSession,
  reloadCurrentBalance,
  probeCurrentSession,
  loginCurrentSession
} = props.vm
</script>
