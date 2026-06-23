<template>
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
        <div class="mt-2" :class="rateMultiplierTextClass(groupCostMultiplier(provisionGroup), channelProtocolFromProviderFamily(provisionGroup?.provider_family, provisionGroup?.name, provisionGroup?.description))">
          {{ formatMultiplier(groupCostMultiplier(provisionGroup)) }}
        </div>
        <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">使用 {{ formatMultiplier(provisionGroup?.effective_rate_multiplier) }} / 充值 {{ formatMultiplier(currentSupplierRechargeMultiplier()) }}</div>
      </div>
    </div>

    <div class="grid gap-4 sm:grid-cols-2">
      <label class="block">
        <span class="input-label">本地规范 Key 名称</span>
        <input v-model.trim="provisionForm.name" class="input bg-gray-50 dark:bg-dark-900" required readonly />
      </label>
      <label class="block">
        <span class="input-label">本地账号名称</span>
        <input v-model.trim="provisionForm.local_account_name" class="input" required />
      </label>
    </div>

    <label class="inline-flex items-start gap-2 text-sm text-gray-700 dark:text-dark-200">
      <input v-model="provisionForm.sync_provider_name" type="checkbox" class="mt-0.5 h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500 dark:border-dark-600 dark:bg-dark-900" />
      <span>
        <span class="font-medium text-gray-900 dark:text-gray-100">同步第三方 Key 名称</span>
        <span class="ml-2 text-xs text-gray-500 dark:text-dark-400">默认不改第三方；勾选后使用稳定别名。</span>
      </span>
    </label>

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


</template>

<script setup lang="ts">
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
const props = defineProps<{ vm: any }>()
const {
  provisionSubmitting,
  provisionDialogOpen,
  groupsSupplier,
  provisionGroup,
  activeProvisionJob,
  provisionError,
  provisionForm,
  activeProvisionJobRunning,
  formatMultiplier,
  rateMultiplierTextClass,
  channelProtocolFromProviderFamily,
  currentSupplierRechargeMultiplier,
  groupCostMultiplier,
  provisionJobStatusLabel,
  provisionJobStatusClass,
  provisionJobCaption,
  closeProvisionDialog,
  submitProvision
} = props.vm
</script>
