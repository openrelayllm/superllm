<template>
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

    <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
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
        <input v-model.trim="form.balance_currency" class="input" placeholder="USD" />
      </label>
      <label class="block">
        <span class="input-label">充值倍率</span>
        <input v-model.number="form.recharge_multiplier" type="number" min="0.000001" step="any" class="input" placeholder="1" />
      </label>
    </div>

    <div class="grid gap-4 sm:grid-cols-2">
      <label class="block">
        <span class="input-label">Key 配额策略</span>
        <select v-model="form.key_limit_policy" class="input">
          <option value="unknown">未知</option>
          <option value="unlimited">无限制</option>
          <option value="limited">限制数量</option>
          <option value="unsupported">不支持自动创建</option>
        </select>
      </label>
      <label class="block">
        <span class="input-label">Key 上限</span>
        <input
          v-model.number="form.key_limit_value"
          type="number"
          :min="form.key_limit_policy === 'limited' ? 1 : 0"
          step="1"
          class="input"
          :disabled="form.key_limit_policy !== 'limited'"
          placeholder="例如 10"
        />
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
      <label class="block">
        <span class="input-label">第三方兑换入口</span>
        <input v-model.trim="form.third_party_recharge_url" class="input" placeholder="https://supplier.example.com/custom/..." />
      </label>
      <label class="block">
        <span class="input-label">本地充值入口</span>
        <input v-model.trim="form.local_recharge_url" class="input" placeholder="https://sub2apiplus.example.com/custom/..." />
      </label>
    </div>

    <div class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
      <div class="mb-3 flex items-center justify-between gap-3">
        <div>
          <h3 class="text-sm font-medium text-gray-900 dark:text-gray-100">供应商登录凭据</h3>
          <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">用于后台一键登录和插件采集供应商后台会话。</p>
        </div>
        <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
          <input v-model="form.browser_login_enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
          启用
        </label>
      </div>
      <div class="grid gap-4 sm:grid-cols-3">
        <label class="block">
          <span class="input-label">{{ browserLoginIdentityLabel(form.type) }}</span>
          <input v-model.trim="form.browser_login_username" class="input" autocomplete="username" :placeholder="browserLoginIdentityPlaceholder(form.type)" />
        </label>
        <label class="block">
          <span class="input-label">登录密码</span>
          <input v-model="form.browser_login_password" type="password" class="input" autocomplete="new-password" :placeholder="editingSupplier ? '留空不修改' : ''" />
        </label>
        <label class="block">
          <span class="input-label">临时 Token</span>
          <input v-model="form.browser_login_token" type="password" class="input" autocomplete="off" :placeholder="browserLoginTokenPlaceholder(form.type)" />
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


</template>

<script setup lang="ts">
import BaseDialog from '@/components/common/BaseDialog.vue'
const props = defineProps<{ vm: any }>()
const {
  submitting,
  statusSubmitting,
  editorOpen,
  statusDialogOpen,
  bulkStatusMode,
  editingSupplier,
  selectedCount,
  form,
  statusForm,
  closeEditor,
  submitSupplier,
  submitStatus
} = props.vm

function browserLoginIdentityLabel(type: string): string {
  if (type === 'sub2api') return '登录邮箱'
  if (type === 'new_api') return '登录用户名 / User ID'
  return '登录账号'
}

function browserLoginIdentityPlaceholder(type: string): string {
  if (isEditingSupplier()) return '留空不修改'
  if (type === 'sub2api') return 'ops@example.com'
  if (type === 'new_api') return 'username，Token 登录时填数字 User ID'
  return ''
}

function browserLoginTokenPlaceholder(type: string): string {
  if (isEditingSupplier()) return '留空不修改'
  if (type === 'new_api') return '{"access_token":"...","user_id":42}'
  return ''
}

function isEditingSupplier(): boolean {
  return Boolean(editingSupplier?.value ?? editingSupplier)
}
</script>
