<template>
  <BaseDialog :show="show" title="测试账号连接" width="normal" @close="handleClose">
    <div class="space-y-4">
      <div
        v-if="account"
        class="flex items-center justify-between rounded-lg border border-gray-200 bg-gray-50 p-3 dark:border-dark-500 dark:bg-dark-700"
      >
        <div class="flex min-w-0 items-center gap-3">
          <div class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-lg bg-teal-500 text-white">
            <Icon name="play" size="md" :stroke-width="2" />
          </div>
          <div class="min-w-0">
            <div class="truncate font-semibold text-gray-900 dark:text-gray-100">
              {{ account.local_account_name }}
            </div>
            <div class="mt-1 flex flex-wrap items-center gap-1.5 text-xs text-gray-500 dark:text-gray-400">
              <span class="rounded bg-gray-200 px-1.5 py-0.5 text-[10px] font-medium uppercase dark:bg-dark-500">
                {{ accountTypeLabel }}
              </span>
              <span>账号</span>
            </div>
          </div>
        </div>
        <span class="badge ml-3 flex-shrink-0" :class="runtimeBadgeClass(account.runtime_status)">
          {{ runtimeBadgeLabel(account.runtime_status) }}
        </span>
      </div>

      <label class="block">
        <span class="input-label">测试模型</span>
        <Select
          v-model="selectedModelId"
          :options="modelOptions"
          :disabled="loadingModels || status === 'connecting'"
          value-key="id"
          label-key="display_name"
          :placeholder="loadingModels ? '加载中...' : '选择测试模型'"
          empty-text="暂无可测试模型"
        />
      </label>

      <label v-if="isOpenAIAccount" class="block">
        <span class="input-label">测试模式</span>
        <Select
          v-model="testMode"
          :options="openAITestModeOptions"
          :disabled="status === 'connecting'"
        />
      </label>

      <label v-if="supportsImageTest" class="block">
        <span class="input-label">图片测试 Prompt</span>
        <textarea
          v-model="testPrompt"
          class="input min-h-[82px] resize-y"
          :disabled="status === 'connecting'"
          placeholder="输入用于图片模型的测试提示词"
        />
      </label>

      <div class="group relative">
        <div
          ref="terminalRef"
          class="max-h-[260px] min-h-[132px] overflow-y-auto rounded-lg border border-gray-800 bg-gray-950 p-4 font-mono text-sm leading-6 text-gray-300"
        >
          <div v-if="status === 'idle'" class="flex items-center gap-2 text-gray-500">
            <Icon name="terminal" size="sm" :stroke-width="2" />
            <span>准备测试</span>
          </div>
          <div v-else-if="status === 'connecting'" class="flex items-center gap-2 text-yellow-400">
            <Icon name="refresh" size="sm" class="animate-spin" :stroke-width="2" />
            <span>正在连接上游 API</span>
          </div>

          <div v-for="(line, index) in outputLines" :key="index" :class="line.class">
            {{ line.text }}
          </div>

          <div v-if="streamingContent" class="whitespace-pre-wrap text-green-300">
            {{ streamingContent }}<span class="animate-pulse">_</span>
          </div>

          <div
            v-if="status === 'success'"
            class="mt-3 flex items-center gap-2 border-t border-gray-800 pt-3 text-green-400"
          >
            <Icon name="check" size="sm" :stroke-width="2" />
            <span>测试完成，渠道可用</span>
          </div>
          <div
            v-else-if="status === 'error'"
            class="mt-3 flex items-center gap-2 border-t border-gray-800 pt-3 text-red-400"
          >
            <Icon name="x" size="sm" :stroke-width="2" />
            <span>{{ errorMessage }}</span>
          </div>
        </div>

        <button
          v-if="outputLines.length > 0 || streamingContent"
          type="button"
          class="absolute right-2 top-2 rounded-lg bg-gray-800/90 p-1.5 text-gray-400 opacity-0 transition-all hover:bg-gray-700 hover:text-white group-hover:opacity-100"
          title="复制输出"
          @click="copyOutput"
        >
          <Icon name="copy" size="sm" :stroke-width="2" />
        </button>
      </div>

      <div v-if="generatedImages.length > 0" class="space-y-2">
        <div class="text-xs font-medium text-gray-600 dark:text-gray-300">图片结果</div>
        <div class="grid gap-3 sm:grid-cols-2">
          <button
            v-for="(image, index) in generatedImages"
            :key="`${image.url}-${index}`"
            type="button"
            class="overflow-hidden rounded-lg border border-gray-200 bg-white text-left shadow-sm transition hover:border-primary-300 dark:border-dark-500 dark:bg-dark-700"
            @click="previewImageUrl = image.url"
          >
            <img :src="image.url" :alt="`测试图片 ${index + 1}`" class="max-h-[260px] w-full object-contain" />
            <div class="border-t border-gray-100 px-3 py-1.5 text-xs text-gray-500 dark:border-dark-500 dark:text-gray-300">
              {{ image.mimeType || 'image/*' }}
            </div>
          </button>
        </div>
      </div>

      <Teleport to="body">
        <Transition name="fade">
          <div
            v-if="previewImageUrl"
            class="fixed inset-0 z-[100] flex items-center justify-center bg-black/80 p-4"
            @click.self="previewImageUrl = ''"
          >
            <button
              type="button"
              class="absolute right-4 top-4 rounded-full bg-black/50 p-2 text-white transition-colors hover:bg-black/70"
              title="关闭预览"
              @click="previewImageUrl = ''"
            >
              <Icon name="x" size="lg" :stroke-width="2" />
            </button>
            <img :src="previewImageUrl" alt="图片预览" class="max-h-[90vh] max-w-[90vw] rounded-lg object-contain shadow-2xl" />
          </div>
        </Transition>
      </Teleport>

      <div class="flex items-center justify-between px-1 text-xs text-gray-500 dark:text-gray-400">
        <span class="flex items-center gap-1">
          <Icon name="grid" size="sm" :stroke-width="2" />
          测试模型
        </span>
        <span class="flex items-center gap-1">
          <Icon name="chat" size="sm" :stroke-width="2" />
          {{ supportsImageTest ? '图片提示词' : '提示词："hi"' }}
        </span>
      </div>
    </div>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button type="button" class="btn btn-secondary" @click="handleClose">关闭</button>
        <button
          type="button"
          class="btn"
          :class="status === 'success' ? 'btn-success' : status === 'error' ? 'btn-warning' : 'btn-primary'"
          :disabled="status === 'connecting' || !selectedModelId"
          @click="startTest"
        >
          <Icon v-if="status === 'connecting'" name="refresh" size="sm" class="animate-spin" :stroke-width="2" />
          <Icon v-else-if="status === 'idle'" name="play" size="sm" :stroke-width="2" />
          <Icon v-else name="refresh" size="sm" :stroke-width="2" />
          <span>{{ status === 'connecting' ? '测试中' : status === 'idle' ? '开始测试' : '重新测试' }}</span>
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
import { useClipboard } from '@/composables/useClipboard'
import {
  listLocalAccountTestModels,
  localAccountTestURL,
  type LocalAccountTestModel,
  type LocalAccountTestPayload,
  type SupplierAccount,
  type SupplierRuntimeStatus
} from '@/api/admin/adminPlus'

interface OutputLine {
  text: string
  class: string
}

interface PreviewImage {
  url: string
  mimeType?: string
}

interface TestEvent {
  type: string
  text?: string
  model?: string
  status?: string
  success?: boolean
  error?: string
  image_url?: string
  mime_type?: string
}

const props = defineProps<{
  show: boolean
  account: SupplierAccount | null
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

const { copyToClipboard } = useClipboard()

const terminalRef = ref<HTMLElement | null>(null)
const status = ref<'idle' | 'connecting' | 'success' | 'error'>('idle')
const outputLines = ref<OutputLine[]>([])
const streamingContent = ref('')
const errorMessage = ref('')
const availableModels = ref<LocalAccountTestModel[]>([])
const selectedModelId = ref('')
const testPrompt = ref('')
const testMode = ref<'default' | 'compact'>('default')
const loadingModels = ref(false)
const generatedImages = ref<PreviewImage[]>([])
const previewImageUrl = ref('')

let abortController: AbortController | null = null

const prioritizedGeminiModels = [
  'gemini-3.1-flash-image',
  'gemini-2.5-flash-image',
  'gemini-3.5-flash',
  'gemini-2.5-flash',
  'gemini-2.5-pro',
  'gemini-3-flash-preview',
  'gemini-3-pro-preview',
  'gemini-2.0-flash'
]

const modelOptions = computed(() => availableModels.value as unknown as Array<Record<string, unknown>>)

const normalizedPlatform = computed(() => props.account?.local_account_platform.toLowerCase() || '')
const normalizedType = computed(() => props.account?.local_account_type.toLowerCase() || '')
const isOpenAIAccount = computed(() => normalizedPlatform.value === 'openai')
const openAITestModeOptions = computed(() => [
  { value: 'default', label: '默认测试' },
  { value: 'compact', label: 'Compact 探测' }
])
const accountTypeLabel = computed(() => {
  const value = normalizedType.value || props.account?.local_account_type || ''
  if (value.toLowerCase() === 'apikey') return 'APIKEY'
  if (value.toLowerCase() === 'oauth') return 'OAUTH'
  return value.toUpperCase()
})

const supportsImageTest = computed(() => {
  const modelID = selectedModelId.value.toLowerCase()
  if (modelID.startsWith('gpt-image-')) return normalizedPlatform.value === 'openai'
  if (!modelID.startsWith('gemini-') || !modelID.includes('-image')) return false
  return normalizedPlatform.value === 'gemini' || (normalizedPlatform.value === 'antigravity' && normalizedType.value === 'apikey')
})

watch(
  () => props.show,
  async (show) => {
    if (show && props.account) {
      testPrompt.value = ''
      testMode.value = 'default'
      resetState()
      await loadAvailableModels()
      return
    }
    abortStream()
  }
)

watch(selectedModelId, () => {
  if (supportsImageTest.value && !testPrompt.value.trim()) {
    testPrompt.value = 'Generate a cute orange cat astronaut sticker on a clean pastel background.'
  }
})

async function loadAvailableModels() {
  if (!props.account) return

  loadingModels.value = true
  selectedModelId.value = ''
  try {
    const models = await listLocalAccountTestModels(props.account.local_sub2api_account_id)
    availableModels.value = normalizedPlatform.value === 'gemini' || normalizedPlatform.value === 'antigravity'
      ? sortTestModels(models)
      : models
    selectedModelId.value = preferredModelId(availableModels.value)
  } catch (error) {
    availableModels.value = []
    selectedModelId.value = ''
    errorMessage.value = (error as { message?: string }).message || '加载模型失败'
    status.value = 'error'
  } finally {
    loadingModels.value = false
  }
}

function preferredModelId(models: LocalAccountTestModel[]): string {
  if (models.length === 0) return ''
  if (normalizedPlatform.value === 'anthropic' || normalizedPlatform.value === 'antigravity') {
    return models.find((model) => model.id.includes('sonnet'))?.id || models[0].id
  }
  return models[0].id
}

function sortTestModels(models: LocalAccountTestModel[]): LocalAccountTestModel[] {
  const priorityMap = new Map(prioritizedGeminiModels.map((id, index) => [id, index]))
  return [...models].sort((a, b) => {
    const aPriority = priorityMap.get(a.id) ?? Number.MAX_SAFE_INTEGER
    const bPriority = priorityMap.get(b.id) ?? Number.MAX_SAFE_INTEGER
    if (aPriority !== bPriority) return aPriority - bPriority
    return a.id.localeCompare(b.id)
  })
}

function resetState() {
  status.value = 'idle'
  outputLines.value = []
  streamingContent.value = ''
  errorMessage.value = ''
  generatedImages.value = []
  previewImageUrl.value = ''
}

function handleClose() {
  abortStream()
  emit('close')
}

function abortStream() {
  if (abortController) {
    abortController.abort()
    abortController = null
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

async function startTest() {
  if (!props.account || !selectedModelId.value) return

  resetState()
  status.value = 'connecting'
  addLine(`开始测试账号: ${props.account.local_account_name}`, 'text-blue-400')
  addLine(`账号类型: ${props.account.local_account_type}`, 'text-gray-400')
  addLine('', 'text-gray-300')

  abortStream()
  abortController = new AbortController()

  try {
    const payload: LocalAccountTestPayload = {
      model_id: selectedModelId.value,
      prompt: supportsImageTest.value ? testPrompt.value.trim() : '',
      mode: isOpenAIAccount.value ? testMode.value : 'default'
    }
    const response = await fetch(localAccountTestURL(props.account.local_sub2api_account_id), {
      method: 'POST',
      headers: {
        Authorization: `Bearer ${localStorage.getItem('auth_token') || ''}`,
        'Content-Type': 'application/json'
      },
      credentials: 'include',
      body: JSON.stringify(payload),
      signal: abortController.signal
    })

    if (!response.ok) {
      throw new Error(`HTTP ${response.status}`)
    }
    if (!response.body) {
      throw new Error('响应体为空')
    }

    await readEventStream(response.body)
    if (status.value === 'connecting') {
      status.value = 'success'
    }
  } catch (error) {
    if (error instanceof DOMException && error.name === 'AbortError') {
      status.value = 'idle'
      return
    }
    const message = error instanceof Error ? error.message : '未知错误'
    status.value = 'error'
    errorMessage.value = message
    addLine(`Error: ${message}`, 'text-red-400')
  } finally {
    abortController = null
  }
}

async function readEventStream(body: ReadableStream<Uint8Array>) {
  const reader = body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''

  while (true) {
    const { done, value } = await reader.read()
    if (done) break

    buffer += decoder.decode(value, { stream: true })
    const lines = buffer.split('\n')
    buffer = lines.pop() || ''

    for (const line of lines) {
      const trimmed = line.trim()
      if (!trimmed.startsWith('data:')) continue
      const json = trimmed.slice(5).trim()
      if (!json || json === '[DONE]') continue
      try {
        handleEvent(JSON.parse(json) as TestEvent)
      } catch {
        addLine(`无法解析事件: ${json}`, 'text-yellow-400')
      }
    }
  }
}

function handleEvent(event: TestEvent) {
  switch (event.type) {
    case 'test_start':
      addLine('已连接到 API', 'text-green-400')
      if (event.model) {
        addLine(`使用模型: ${event.model}`, 'text-cyan-400')
      }
      addLine(supportsImageTest.value ? '发送图片测试请求' : '发送测试消息: "hi"', 'text-gray-400')
      addLine('', 'text-gray-300')
      addLine('响应:', 'text-yellow-400')
      break
    case 'content':
      if (event.text) {
        streamingContent.value += event.text
        void scrollToBottom()
      }
      break
    case 'image':
      if (event.image_url) {
        generatedImages.value.push({ url: event.image_url, mimeType: event.mime_type })
        addLine(`收到图片结果 ${generatedImages.value.length}`, 'text-purple-300')
      }
      break
    case 'status':
      if (event.text || event.status) {
        addLine(event.text || event.status || '', 'text-gray-400')
      }
      break
    case 'test_complete':
      flushStreamingContent()
      if (event.success === false) {
        status.value = 'error'
        errorMessage.value = event.error || '测试失败'
        return
      }
      status.value = 'success'
      break
    case 'error':
      flushStreamingContent()
      status.value = 'error'
      errorMessage.value = event.error || '测试失败'
      addLine(errorMessage.value, 'text-red-400')
      break
  }
}

function flushStreamingContent() {
  if (!streamingContent.value) return
  addLine(streamingContent.value, 'whitespace-pre-wrap text-green-300')
  streamingContent.value = ''
}

function copyOutput() {
  const text = [
    ...outputLines.value.map((line) => line.text),
    streamingContent.value
  ].filter(Boolean).join('\n')
  void copyToClipboard(text, '输出已复制')
}

function runtimeBadgeLabel(value: SupplierRuntimeStatus): string {
  return {
    active: 'active',
    candidate: 'candidate',
    monitor_only: 'monitor',
    disabled: 'disabled'
  }[value]
}

function runtimeBadgeClass(value: SupplierRuntimeStatus): string {
  if (value === 'active') return 'bg-green-100 text-green-700 dark:bg-green-500/20 dark:text-green-400'
  if (value === 'candidate') return 'bg-primary-100 text-primary-700 dark:bg-primary-500/20 dark:text-primary-300'
  if (value === 'disabled') return 'bg-red-100 text-red-700 dark:bg-red-500/20 dark:text-red-300'
  return 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-400'
}
</script>

<style>
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
