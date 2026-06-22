<template>
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

</template>

<script setup lang="ts">
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
const props = defineProps<{ vm: any }>()
const {
  repairSubmitting,
  repairDialogOpen,
  deleteDialogOpen,
  repairKey,
  localAccounts,
  repairAccountsLoading,
  repairError,
  repairForm,
  deleteConfirmMessage,
  closeRepairDialog,
  submitRepairBinding,
  confirmDelete
} = props.vm
</script>
