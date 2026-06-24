<template>
  <BaseDialog :show="channelProbeDialogOpen" :title="dialogTitle" width="normal" @close="handleClose">
    <div class="space-y-4">
      <div v-if="channelProbeSnapshot" class="rounded-lg border border-gray-200 bg-gray-50 p-3 dark:border-dark-600 dark:bg-dark-800">
        <div class="flex items-center justify-between gap-3">
          <div class="min-w-0">
            <div class="truncate font-semibold text-gray-900 dark:text-gray-100">
              {{ channelProbeSupplier?.name || '-' }} / {{ channelProbeSnapshot.group_name || '未命名渠道' }}
            </div>
            <div class="mt-1 flex flex-wrap items-center gap-1.5 text-xs text-gray-500 dark:text-dark-400">
              <span class="badge badge-primary">{{ protocolLabel }}</span>
              <span class="badge badge-gray">{{ formatMultiplier(channelProbeSnapshot.effective_rate_multiplier) }}</span>
              <span v-if="channelProbeSnapshot.local_sub2api_account_id" class="badge badge-gray">
                本地账号 #{{ channelProbeSnapshot.local_sub2api_account_id }}
              </span>
            </div>
          </div>
          <span class="badge shrink-0" :class="channelProbeStatusClass(channelProbeSnapshot.probe_status)">
            {{ channelProbeStatusLabel(channelProbeSnapshot.probe_status) }}
          </span>
        </div>
      </div>

      <label class="block">
        <span class="input-label">测试模型</span>
        <Select
          v-model="selectedModelId"
          :options="modelOptions"
          :disabled="loadingModels || status === 'running'"
          value-key="id"
          label-key="display_name"
          :placeholder="loadingModels ? '加载中...' : '选择测试模型'"
          empty-text="暂无可测试模型"
        />
      </label>

      <div ref="terminalRef" class="max-h-[260px] min-h-[132px] overflow-y-auto rounded-lg border border-gray-800 bg-gray-950 p-4 font-mono text-sm leading-6 text-gray-300">
        <div v-if="status === 'idle'" class="flex items-center gap-2 text-gray-500">
          <Icon name="terminal" size="sm" />
          <span>准备测速。点击“开始测试”后会写入渠道检测快照。</span>
        </div>
        <div v-else-if="status === 'running'" class="flex items-center gap-2 text-yellow-400">
          <Icon name="refresh" size="sm" class="animate-spin" />
          <span>正在调用本地 Sub2API 账号并等待模型响应</span>
        </div>
        <div v-for="(line, index) in outputLines" :key="index" :class="line.class">
          {{ line.text }}
        </div>
        <div v-if="status === 'success'" class="mt-3 flex items-center gap-2 border-t border-gray-800 pt-3 text-green-400">
          <Icon name="check" size="sm" />
          <span>测速完成，列表数据已刷新</span>
        </div>
        <div v-else-if="status === 'error'" class="mt-3 flex items-center gap-2 border-t border-gray-800 pt-3 text-red-400">
          <Icon name="x" size="sm" />
          <span>{{ errorMessage }}</span>
        </div>
      </div>

      <div class="flex items-center justify-between px-1 text-xs text-gray-500 dark:text-dark-400">
        <span>探测接口：OpenAI Responses</span>
        <span>提示词："Return exactly: ok"</span>
      </div>
    </div>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button type="button" class="btn btn-secondary" @click="handleClose">关闭</button>
        <button
          type="button"
          class="btn"
          :class="status === 'success' ? 'btn-success' : status === 'error' ? 'btn-warning' : 'btn-primary'"
          :disabled="status === 'running' || !selectedModelId"
          @click="startProbe"
        >
          <Icon v-if="status === 'running'" name="refresh" size="sm" class="animate-spin" />
          <Icon v-else name="play" size="sm" />
          <span>{{ status === 'idle' ? '开始测试' : '重新测试' }}</span>
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import { listLocalAccountTestModels, type LocalAccountTestModel, type SupplierChannelCheckSnapshot } from '@/api/admin/adminPlus'

interface OutputLine {
  text: string
  class: string
}

const props = defineProps<{ vm: any }>()
const {
  channelProbeDialogOpen,
  channelProbeSupplier,
  channelProbeSnapshot,
  closeChannelProbeDialog,
  runChannelProbeFromDialog,
  channelProbeStatusLabel,
  channelProbeStatusClass,
  formatMultiplier,
  formatLatency,
  appStore
} = props.vm

const preferredModels = ['gpt-5.4-mini', 'gpt-5.5', 'gpt-5.1', 'gpt-4.1-mini', 'gpt-4o-mini']
const terminalRef = ref<HTMLElement | null>(null)
const status = ref<'idle' | 'running' | 'success' | 'error'>('idle')
const outputLines = ref<OutputLine[]>([])
const errorMessage = ref('')
const loadingModels = ref(false)
const availableModels = ref<LocalAccountTestModel[]>([])
const selectedModelId = ref('')

const dialogTitle = computed(() => (channelProbeSupplier.value ? `渠道测速 - ${channelProbeSupplier.value.name}` : '渠道测速'))
const modelOptions = computed(() => availableModels.value as unknown as Array<Record<string, unknown>>)
const protocolLabel = computed(() => protocolDisplay(channelProbeSnapshot.value?.provider_family || 'openai'))

watch(
  () => channelProbeDialogOpen.value,
  async (open) => {
    if (!open) return
    resetState()
    await loadModels()
  }
)

async function loadModels() {
  const localAccountID = Number(channelProbeSnapshot.value?.local_sub2api_account_id || 0)
  if (!localAccountID) {
    availableModels.value = []
    selectedModelId.value = ''
    status.value = 'error'
    errorMessage.value = '该渠道没有本地账号绑定，无法测速'
    return
  }
  loadingModels.value = true
  try {
    const models = await listLocalAccountTestModels(localAccountID)
    availableModels.value = sortModels(models)
    selectedModelId.value = preferredModelId(availableModels.value)
  } catch (error) {
    availableModels.value = []
    selectedModelId.value = ''
    status.value = 'error'
    errorMessage.value = (error as { message?: string }).message || '加载测试模型失败'
  } finally {
    loadingModels.value = false
  }
}

function sortModels(models: LocalAccountTestModel[]): LocalAccountTestModel[] {
  const priority = new Map(preferredModels.map((id, index) => [id, index]))
  return [...models].sort((a, b) => {
    const left = priority.get(a.id) ?? Number.MAX_SAFE_INTEGER
    const right = priority.get(b.id) ?? Number.MAX_SAFE_INTEGER
    if (left !== right) return left - right
    return a.id.localeCompare(b.id)
  })
}

function preferredModelId(models: LocalAccountTestModel[]): string {
  if (models.length === 0) return ''
  return models.find((model) => preferredModels.includes(model.id))?.id || models[0].id
}

function resetState() {
  status.value = 'idle'
  outputLines.value = []
  errorMessage.value = ''
}

function handleClose() {
  closeChannelProbeDialog()
}

async function startProbe() {
  if (!selectedModelId.value || !channelProbeSnapshot.value) return
  resetState()
  status.value = 'running'
  const snapshot = channelProbeSnapshot.value as SupplierChannelCheckSnapshot
  addLine(`开始测试渠道: ${snapshot.group_name || snapshot.supplier_group_id}`, 'text-blue-400')
  addLine(`使用模型: ${selectedModelId.value}`, 'text-cyan-400')
  addLine(`本地账号: #${snapshot.local_sub2api_account_id || '-'}`, 'text-gray-400')
  addLine('', 'text-gray-300')
  try {
    const result = await runChannelProbeFromDialog(selectedModelId.value)
    const current = result.items.find((item: SupplierChannelCheckSnapshot) => item.supplier_group_id === snapshot.supplier_group_id) || result.items[0]
    if (!current) {
      throw new Error('测速返回为空')
    }
    addLine(`状态: ${channelProbeStatusLabel(current.probe_status)}`, current.recommended ? 'text-green-400' : 'text-yellow-400')
    addLine(`首 Token: ${formatLatency(current.first_token_ms)}`, 'text-gray-200')
    addLine(`总耗时: ${formatLatency(current.duration_ms)}`, 'text-gray-200')
    if (current.status_code) {
      addLine(`HTTP: ${current.status_code}`, 'text-gray-400')
    }
    if (current.error_message) {
      addLine(`错误: ${current.error_message}`, 'text-red-300')
    }
    status.value = current.recommended ? 'success' : 'error'
    errorMessage.value = current.error_message || (current.recommended ? '' : '测速未通过')
    if (current.recommended) {
      appStore.showSuccess('渠道测速完成，首 token 和总耗时已更新')
    }
  } catch (error) {
    const message = (error as { message?: string }).message || '渠道测速失败'
    status.value = 'error'
    errorMessage.value = message
    addLine(`Error: ${message}`, 'text-red-400')
  }
}

function addLine(text: string, className = 'text-gray-300') {
  outputLines.value.push({ text, class: className })
  void scrollToBottom()
}

async function scrollToBottom() {
  await nextTick()
  if (terminalRef.value) {
    terminalRef.value.scrollTop = terminalRef.value.scrollHeight
  }
}

function protocolDisplay(value: string): string {
  const normalized = value.toLowerCase()
  if (normalized.includes('claude') || normalized.includes('anthropic')) return 'Claude'
  if (normalized.includes('gemini')) return 'Gemini'
  return 'OpenAI'
}
</script>
