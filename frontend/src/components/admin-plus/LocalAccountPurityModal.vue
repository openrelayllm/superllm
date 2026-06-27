<template>
  <BaseDialog :show="show" :title="dialogTitle" width="extra-wide" @close="handleClose">
    <div class="space-y-4">
      <div
        v-if="account"
        class="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-gray-200 bg-white p-3 dark:border-dark-500 dark:bg-dark-700"
      >
        <div class="flex min-w-0 items-center gap-3">
          <div class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-lg bg-primary-500 text-white">
            <Icon name="shield" size="md" :stroke-width="2" />
          </div>
          <div class="min-w-0">
            <div class="truncate font-semibold text-gray-900 dark:text-gray-100">{{ account.name }}</div>
            <div class="mt-1 flex flex-wrap items-center gap-1.5 text-xs text-gray-500 dark:text-gray-400">
              <span class="rounded bg-primary-50 px-1.5 py-0.5 font-medium uppercase text-primary-700 dark:bg-primary-900/30 dark:text-primary-300">
                {{ providerLabel }} / {{ account.type }}
              </span>
              <span class="font-mono">#{{ account.id }}</span>
            </div>
          </div>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <Select
            v-model="selectedModelId"
            :options="modelOptions"
            :disabled="loadingModels || runStatus === 'running'"
            value-key="id"
            label-key="display_name"
            class="min-w-[220px]"
            :placeholder="loadingModels ? '加载中...' : '选择模型'"
            empty-text="暂无模型"
          />
          <button type="button" class="btn btn-primary btn-sm" :disabled="runStatus === 'running' || !selectedModelId || !isSupportedAccount" @click="startCheck">
            <Icon v-if="runStatus === 'running'" name="refresh" size="sm" class="animate-spin" :stroke-width="2" />
            <Icon v-else name="play" size="sm" :stroke-width="2" />
            <span>{{ runStatus === 'running' ? '检测中' : '开始检测' }}</span>
          </button>
        </div>
      </div>

      <div v-if="!isSupportedAccount" class="rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800 dark:border-amber-500/40 dark:bg-amber-900/20 dark:text-amber-200">
        仅支持 OpenAI 或 Claude API Key 账号执行纯度检测。
      </div>
      <div v-if="fatalReportError" class="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-800 dark:border-red-500/40 dark:bg-red-900/20 dark:text-red-200">
        <div class="font-semibold">检测失败</div>
        <div class="mt-1 break-words">{{ fatalReportError }}</div>
      </div>
      <div v-else-if="probeIssueMessage" class="rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800 dark:border-amber-500/40 dark:bg-amber-900/20 dark:text-amber-200">
        <div class="font-semibold">部分探针异常</div>
        <div class="mt-1 break-words">{{ probeIssueMessage }}</div>
      </div>

      <div class="grid gap-3 lg:grid-cols-[260px_1fr]">
        <div class="rounded-lg border border-gray-200 bg-gray-50 p-4 dark:border-dark-500 dark:bg-dark-700">
          <div class="flex items-center justify-center">
            <div class="score-ring" :style="scoreRingStyle">
              <div class="score-ring-inner">
                <div class="text-3xl font-bold text-gray-950 dark:text-white">{{ displayScore }}</div>
                <div class="text-xs uppercase tracking-wide text-gray-500 dark:text-dark-400">proxyai.best</div>
              </div>
            </div>
          </div>
          <div class="mt-4 text-center">
            <div class="text-sm font-semibold text-gray-900 dark:text-gray-100">{{ verdictLabel }}</div>
            <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ report?.summary || runningSummary }}</div>
          </div>
          <div class="mt-4">
            <div class="mb-1 flex items-center justify-between text-xs text-gray-500 dark:text-dark-400">
              <span>{{ stepLabel }}</span>
              <span>{{ progressPercent }}%</span>
            </div>
            <div class="h-2 overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
              <div class="h-full rounded-full bg-primary-500 transition-all" :style="{ width: `${progressPercent}%` }" />
            </div>
          </div>
          <div class="mt-4 grid grid-cols-2 gap-2 text-center text-xs">
            <div class="rounded-md bg-white p-2 dark:bg-dark-600">
              <div class="font-semibold text-gray-900 dark:text-gray-100">{{ report?.compatibility_score ?? '-' }}</div>
              <div class="text-gray-500 dark:text-dark-400">兼容分</div>
            </div>
            <div class="rounded-md bg-white p-2 dark:bg-dark-600">
              <div class="font-semibold text-gray-900 dark:text-gray-100">{{ report?.official_score ?? '-' }}</div>
              <div class="text-gray-500 dark:text-dark-400">官方分</div>
            </div>
          </div>
        </div>

        <div class="space-y-3">
          <div class="grid gap-2 sm:grid-cols-2 xl:grid-cols-3">
          <div
            v-for="item in displayedValidations"
            :key="item.id"
            class="rounded-lg border bg-white p-3 dark:bg-dark-700"
            :class="validationCardClass(item.status)"
          >
            <div class="flex items-start gap-2">
              <span class="mt-0.5 flex h-6 w-6 flex-shrink-0 items-center justify-center rounded-full" :class="validationIconClass(item.status)">
                <Icon :name="validationIcon(item.status)" size="sm" :class="{ 'animate-spin': item.status === 'running' }" :stroke-width="2" />
              </span>
              <div class="min-w-0">
                <div class="flex flex-wrap items-center gap-1.5">
                  <span class="font-medium text-gray-900 dark:text-gray-100">{{ item.name }}</span>
                  <span class="rounded px-1.5 py-0.5 text-[10px] font-medium uppercase" :class="validationBadgeClass(item.status)">
                    {{ validationStatusLabel(item.status) }}
                  </span>
                </div>
                <div class="mt-1 text-xs leading-5 text-gray-500 dark:text-dark-400">{{ item.message }}</div>
              </div>
            </div>
          </div>
          </div>
          <div class="grid gap-2 sm:grid-cols-2 xl:grid-cols-3">
            <div v-for="item in scoreBreakdownItems" :key="item.key" class="rounded-lg border border-gray-200 bg-white p-3 dark:border-dark-500 dark:bg-dark-700">
              <div class="flex items-center justify-between gap-2">
                <span class="text-xs font-medium text-gray-600 dark:text-dark-300">{{ item.label }}</span>
                <span class="text-xs font-semibold text-gray-900 dark:text-gray-100">{{ item.value }}/{{ item.max }}</span>
              </div>
              <div class="mt-2 h-1.5 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-600">
                <div class="h-full rounded-full bg-primary-500 transition-all" :style="{ width: `${Math.round((item.value / item.max) * 100)}%` }" />
              </div>
            </div>
          </div>
        </div>
      </div>

      <div class="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <div v-for="metric in metricCards" :key="metric.label" class="rounded-lg border border-gray-200 bg-white p-3 dark:border-dark-500 dark:bg-dark-700">
          <div class="text-xs text-gray-500 dark:text-dark-400">{{ metric.label }}</div>
          <div class="mt-1 text-lg font-semibold text-gray-900 dark:text-gray-100">{{ metric.value }}</div>
        </div>
      </div>

      <div class="grid gap-3 lg:grid-cols-[1fr_320px]">
        <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-500 dark:bg-dark-700">
          <div class="mb-3 flex items-center justify-between gap-3">
            <div>
              <div class="text-sm font-semibold text-gray-900 dark:text-gray-100">Token 用量审计</div>
              <div class="text-xs text-gray-500 dark:text-dark-400">{{ tokenAuditSummary }}</div>
            </div>
            <span class="badge" :class="tokenAuditBadgeClass">{{ tokenAuditStatusLabel }}</span>
          </div>
          <div class="mb-3 grid gap-2 sm:grid-cols-2 xl:grid-cols-4">
            <div v-for="item in tokenAuditMetricCards" :key="item.label" class="rounded-md bg-gray-50 p-2 dark:bg-dark-600">
              <div class="text-[10px] text-gray-500 dark:text-dark-400">{{ item.label }}</div>
              <div class="mt-0.5 text-sm font-semibold text-gray-900 dark:text-gray-100">{{ item.value }}</div>
            </div>
          </div>
          <div class="h-36 overflow-x-auto">
            <div class="flex h-full min-w-[520px] items-end gap-2">
              <div v-for="sample in auditSamplesForChart" :key="sample.index" class="flex h-full flex-1 flex-col justify-end gap-1">
                <div class="flex flex-1 items-end rounded bg-gray-100 px-1 dark:bg-dark-600">
                  <div
                    class="w-full rounded-t bg-primary-500 transition-all"
                    :class="{ 'bg-red-400': sample.status === 'fail', 'bg-amber-400': sample.status === 'warn' }"
                    :style="{ height: `${sampleBarHeight(sample)}%` }"
                  />
                </div>
                <div class="text-center text-[10px] text-gray-500 dark:text-dark-400">R{{ sample.index }}</div>
              </div>
            </div>
          </div>
          <div class="mt-3 overflow-x-auto">
            <table class="min-w-full text-left text-xs">
              <thead class="text-gray-500 dark:text-dark-400">
                <tr>
                  <th class="py-1 pr-3 font-medium">轮次</th>
                  <th class="py-1 pr-3 font-medium">输入</th>
                  <th class="py-1 pr-3 font-medium">输出</th>
                  <th class="py-1 pr-3 font-medium">缓存创建</th>
                  <th class="py-1 pr-3 font-medium">缓存读取</th>
                  <th class="py-1 pr-3 font-medium">实际消耗</th>
                  <th class="py-1 pr-3 font-medium">倍率</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-100 dark:divide-dark-600">
                <tr v-for="sample in auditSamplesForTable" :key="sample.index">
                  <td class="py-1.5 pr-3 font-mono">R{{ sample.index }}</td>
                  <td class="py-1.5 pr-3">{{ formatInteger(sample.input_tokens) }}</td>
                  <td class="py-1.5 pr-3">{{ formatInteger(sample.output_tokens) }}</td>
                  <td class="py-1.5 pr-3">{{ formatInteger(auditCacheCreationTokens(sample)) }}</td>
                  <td class="py-1.5 pr-3">{{ formatInteger(auditCachedTokens(sample)) }}</td>
                  <td class="py-1.5 pr-3">{{ formatUSD(sample.actual_cost_usd || sample.cost || 0) }}</td>
                  <td class="py-1.5 pr-3">{{ formatMultiplier(auditRatio(sample)) }}</td>
                </tr>
                <tr v-if="auditSamplesForTable.length === 0">
                  <td colspan="7" class="py-4 text-center text-gray-400 dark:text-dark-400">等待审计样本</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>

        <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-500 dark:bg-dark-700">
          <div class="text-sm font-semibold text-gray-900 dark:text-gray-100">检测明细</div>
          <div class="mt-3 max-h-[300px] space-y-2 overflow-y-auto pr-1">
            <div v-for="check in reportChecks" :key="check.id" class="rounded-md bg-gray-50 p-2 dark:bg-dark-600">
              <div class="flex items-center justify-between gap-2">
                <span class="text-xs font-medium text-gray-800 dark:text-gray-100">{{ check.name }}</span>
                <span class="text-xs" :class="checkStatusClass(check.status)">{{ check.score }}/{{ check.max_score }}</span>
              </div>
              <div class="mt-1 text-xs leading-5 text-gray-500 dark:text-dark-400">{{ check.message }}</div>
            </div>
            <div v-if="reportChecks.length === 0" class="rounded-md bg-gray-50 p-4 text-center text-xs text-gray-400 dark:bg-dark-600 dark:text-dark-400">
              等待后端探针结果
            </div>
          </div>
        </div>
      </div>

      <div v-if="errorMessage" class="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700 dark:border-red-500/40 dark:bg-red-900/20 dark:text-red-200">
        {{ errorMessage }}
      </div>
    </div>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button type="button" class="btn btn-secondary" @click="handleClose">关闭</button>
        <button type="button" class="btn btn-primary" :disabled="runStatus === 'running' || !selectedModelId || !isSupportedAccount" @click="startCheck">
          <Icon v-if="runStatus === 'running'" name="refresh" size="sm" class="animate-spin" :stroke-width="2" />
          <Icon v-else name="shield" size="sm" :stroke-width="2" />
          <span>{{ runStatus === 'running' ? '检测中' : report ? '重新检测' : '开始检测' }}</span>
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import {
  listLocalAccountTestModels,
  localAccountPurityStreamURL,
  type LocalAccountPurityPayload,
  type LocalAccountTestModel,
  type LocalSub2APIAccount,
  type PurityCheckEvent,
  type PurityCheckMetrics,
  type PurityCheckResult,
  type PurityCheckStatus,
  type PurityProvider,
  type PurityReport,
  type PurityScoreBreakdown,
  type PurityTokenAuditReport,
  type PurityTokenAuditSample,
  type PurityValidationResult
} from '@/api/admin/adminPlus'
import { formatInteger } from '@/views/admin/operations/SupplierAccountsUtils'

type RunStatus = 'idle' | 'running' | 'success' | 'error'
type DisplayStatus = 'idle' | 'running' | PurityCheckStatus
type IconName = 'checkCircle' | 'exclamationTriangle' | 'xCircle' | 'refresh' | 'clock'
type ScoreBreakdownKey = 'tag_check' | 'structure' | 'behavior' | 'signature_proto' | 'multimodal' | 'token_audit'

interface ValidationDefinition {
  id: string
  name: string
  message: string
}

interface DisplayValidation {
  id: string
  name: string
  status: DisplayStatus
  message: string
}

const props = defineProps<{
  show: boolean
  account: LocalSub2APIAccount | null
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

const validationDefinitions: ValidationDefinition[] = [
  { id: 'llm_fingerprint', name: 'LLM 指纹验证', message: '等待模型列表和 Base 域名探测' },
  { id: 'schema_integrity', name: '结构完整性', message: '等待协议 schema 探测' },
  { id: 'behavior', name: '行为验证', message: '等待工具调用和流式事件探测' },
  { id: 'signature', name: '签名校验', message: '等待 usage 与协议签名探测' },
  { id: 'multimodal', name: '多模态能力', message: '等待图像输入探测' },
  { id: 'token_audit', name: 'Token 用量审计', message: '等待 R1-R11 用量审计' }
]

const stepLabels: Record<string, string> = {
  tag: 'LLM 指纹验证',
  structure: '结构完整性',
  behavior: '行为验证',
  signature: '签名校验',
  multimodal: '多模态能力',
  token_audit: 'Token 用量审计',
  evaluate: '最终评估'
}

const activeValidationByStep: Record<string, string> = {
  tag: 'llm_fingerprint',
  structure: 'schema_integrity',
  behavior: 'behavior',
  signature: 'signature',
  multimodal: 'multimodal',
  token_audit: 'token_audit'
}

const scoreDefinitions: Array<{ key: ScoreBreakdownKey; label: string; max: number }> = [
  { key: 'tag_check', label: '指纹', max: 10 },
  { key: 'structure', label: '结构', max: 20 },
  { key: 'behavior', label: '行为', max: 30 },
  { key: 'signature_proto', label: '签名', max: 30 },
  { key: 'multimodal', label: '多模态', max: 10 },
  { key: 'token_audit', label: 'Token', max: 10 }
]

const runStatus = ref<RunStatus>('idle')
const loadingModels = ref(false)
const availableModels = ref<LocalAccountTestModel[]>([])
const selectedModelId = ref('')
const report = ref<PurityReport | null>(null)
const metrics = ref<PurityCheckMetrics>({})
const scores = ref<PurityScoreBreakdown>({})
const tokenAudit = ref<PurityTokenAuditReport | null>(null)
const auditSamples = ref<PurityTokenAuditSample[]>([])
const checks = ref<PurityCheckResult[]>([])
const validations = ref<Record<string, PurityValidationResult>>({})
const stepName = ref('')
const progress = ref(0)
const tokenAuditProgress = ref('')
const errorMessage = ref('')
const started = ref(false)

let abortController: AbortController | null = null

const modelOptions = computed(() => availableModels.value as unknown as Array<Record<string, unknown>>)
const currentProvider = computed<PurityProvider | null>(() => normalizeAccountProvider(props.account?.platform))
const isSupportedAccount = computed(() => {
  const account = props.account
  return Boolean(account && currentProvider.value && account.type.toLowerCase() === 'apikey')
})
const providerLabel = computed(() => currentProvider.value === 'anthropic' ? 'Claude' : 'OpenAI')
const dialogTitle = computed(() => `${providerLabel.value} API 纯度检测`)
const displayScore = computed(() => report.value?.score ?? (started.value ? 0 : '-'))
const scoreRingStyle = computed(() => {
  const score = typeof displayScore.value === 'number' ? displayScore.value : 0
  return { '--score-angle': `${Math.max(0, Math.min(100, score))}%` }
})
const verdictLabel = computed(() => {
  const verdict = report.value?.verdict || ''
  if (verdict === 'official_openai') return 'OpenAI 官方'
  if (verdict === 'openai_compatible') return 'OpenAI 兼容'
  if (verdict === 'official_claude') return 'Claude 官方'
  if (verdict === 'claude_compatible') return 'Claude 兼容'
  if (verdict === 'partial_compatible') return '部分兼容'
  if (verdict === 'invalid_or_unavailable') return '不可用'
  return started.value ? '检测中' : '等待检测'
})
const currentStepName = computed(() => report.value?.step_name || stepName.value)
const stepLabel = computed(() => stepLabels[currentStepName.value] || (started.value ? '准备检测' : '等待开始'))
const progressPercent = computed(() => {
  const value = normalizeProgress(report.value?.progress ?? progress.value)
  return Math.round(value * 100)
})
const runningSummary = computed(() => runStatus.value === 'running' ? `后端探针正在执行：${stepLabel.value}` : '尚未开始检测')
const currentRunningValidation = computed(() => activeValidationByStep[currentStepName.value] || '')
const fatalReportError = computed(() => {
  if (report.value?.status === 'error' || report.value?.error) {
    return report.value?.error || metrics.value.error_message || '检测失败'
  }
  return ''
})
const probeIssueMessage = computed(() => {
  if (fatalReportError.value || !metrics.value.error_message) return ''
  return metrics.value.error_message
})
const displayedValidations = computed<DisplayValidation[]>(() => validationDefinitions.map((definition) => {
  const result = validations.value[definition.id]
  if (result) {
    return {
      id: definition.id,
      name: result.name || definition.name,
      status: result.status as DisplayStatus,
      message: result.message || definition.message
    }
  }
  return {
    ...definition,
    name: validationDisplayName(definition),
    message: validationWaitingMessage(definition),
    status: started.value && runStatus.value === 'running' && currentRunningValidation.value === definition.id ? 'running' : 'idle'
  }
}))
const scoreBreakdownItems = computed(() => {
  const source = report.value?.scores || scores.value
  return scoreDefinitions.map((definition) => {
    const rawValue = source[definition.key] ?? 0
    const value = Math.max(0, Math.min(definition.max, rawValue))
    return { ...definition, value }
  })
})
const auditSamplesForChart = computed(() => normalizedAuditSamples())
const auditSamplesForTable = computed(() => normalizedAuditSamples().filter((sample) => sample.status !== 'fail' || sample.total_tokens > 0))
const reportChecks = computed<PurityCheckResult[]>(() => (report.value?.checks?.length ? report.value.checks : checks.value))
const tokenAuditSummary = computed(() => {
  if (tokenAudit.value) return `${tokenAudit.value.summary} · ${tokenAudit.value.sample_count}/11`
  if (auditSamples.value.length > 0) return `采集中 · ${tokenAuditProgress.value || `${auditSamples.value.length}/11`}`
  return started.value ? '等待样本' : '尚未开始'
})
const tokenAuditMetricCards = computed(() => {
  const audit = tokenAudit.value
  return [
    { label: '官方基线', value: formatUSD(audit?.official_baseline_usd || audit?.baseline_total_cost_usd || 0) },
    { label: '实际消耗', value: formatUSD(audit?.actual_cost_usd || audit?.total_cost || 0) },
    { label: '倍率', value: formatMultiplier(audit?.multiplier || audit?.overall_ratio || 0) },
    { label: '缓存命中率', value: formatPercent(audit?.cache_hit_rate || 0) }
  ]
})
const tokenAuditStatusLabel = computed(() => validationStatusLabel((tokenAudit.value?.status || (auditSamples.value.length > 0 ? 'running' : 'idle')) as DisplayStatus))
const tokenAuditBadgeClass = computed(() => validationBadgeClass((tokenAudit.value?.status || (auditSamples.value.length > 0 ? 'running' : 'idle')) as DisplayStatus))
const metricCards = computed(() => [
  { label: '模型列表', value: latencyLabel(metrics.value.models_latency_ms) },
  { label: currentProvider.value === 'anthropic' ? 'Messages' : 'Responses', value: latencyLabel(currentProvider.value === 'anthropic' ? metrics.value.messages_latency_ms : metrics.value.responses_latency_ms) },
  { label: '首 Token', value: latencyLabel(metrics.value.stream_first_token_ms) },
  { label: '总耗时', value: latencyLabel(metrics.value.latency_ms) }
])

watch(
  () => props.show,
  async (show) => {
    if (show && props.account) {
      resetAll()
      await loadModels()
      return
    }
    abortStream()
  }
)

async function loadModels() {
  if (!props.account) return
  loadingModels.value = true
  selectedModelId.value = ''
  try {
    const models = await listLocalAccountTestModels(props.account.id)
    availableModels.value = models
    selectedModelId.value = preferredModel(models)
  } catch (error) {
    availableModels.value = []
    errorMessage.value = (error as { message?: string }).message || '加载模型失败'
    runStatus.value = 'error'
  } finally {
    loadingModels.value = false
  }
}

function preferredModel(models: LocalAccountTestModel[]): string {
  if (currentProvider.value === 'anthropic') {
    return findPreferredModel(models, ['claude-opus-4-8', 'claude-opus-4-7', 'claude-opus', 'opus', 'claude-sonnet-4-6', 'claude-sonnet-4-5', 'claude-sonnet', 'sonnet', 'claude'])
  }
  return findPreferredModel(models, ['gpt-5.4', 'gpt-5.4-mini', 'gpt-5.5', 'gpt'])
}

function resetAll() {
  abortStream()
  resetRun()
  runStatus.value = 'idle'
}

function resetRun() {
  report.value = null
  metrics.value = {}
  scores.value = {}
  tokenAudit.value = null
  auditSamples.value = []
  checks.value = []
  validations.value = {}
  stepName.value = ''
  progress.value = 0
  tokenAuditProgress.value = ''
  errorMessage.value = ''
  started.value = false
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

async function startCheck() {
  if (!props.account || !selectedModelId.value || !isSupportedAccount.value) return
  resetRun()
  runStatus.value = 'running'
  started.value = true
  abortController = new AbortController()

  try {
    const payload: LocalAccountPurityPayload = {
      provider: currentProvider.value || 'openai',
      model_id: selectedModelId.value
    }
    const response = await fetch(localAccountPurityStreamURL(props.account.id), {
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
      throw new Error(await responseErrorMessage(response))
    }
    if (!response.body) {
      throw new Error('响应体为空')
    }
    await readNDJSON(response.body)
    if (runStatus.value === 'running') runStatus.value = 'success'
  } catch (error) {
    if (error instanceof DOMException && error.name === 'AbortError') {
      runStatus.value = 'idle'
      return
    }
    runStatus.value = 'error'
    errorMessage.value = error instanceof Error ? error.message : '检测失败'
  } finally {
    abortController = null
  }
}

async function responseErrorMessage(response: Response): Promise<string> {
  const text = await response.text()
  if (!text) return `HTTP ${response.status}`
  try {
    const payload = JSON.parse(text) as { message?: string; error?: string }
    return payload.message || payload.error || `HTTP ${response.status}`
  } catch {
    return text.slice(0, 160)
  }
}

async function readNDJSON(body: ReadableStream<Uint8Array>) {
  const reader = body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''
  while (true) {
    const { done, value } = await reader.read()
    if (done) break
    buffer += decoder.decode(value, { stream: true })
    const lines = buffer.split('\n')
    buffer = lines.pop() || ''
    for (const line of lines) handleEventLine(line)
  }
  if (buffer.trim()) handleEventLine(buffer)
}

function handleEventLine(line: string) {
  const trimmed = line.trim()
  if (!trimmed) return
  try {
    handleEvent(JSON.parse(trimmed) as PurityCheckEvent)
  } catch {
    errorMessage.value = `无法解析检测事件: ${trimmed.slice(0, 120)}`
  }
}

function handleEvent(event: PurityCheckEvent) {
  applyEventState(event)
  switch (event.type) {
    case 'started':
      if (event.report) {
        applyReportSnapshot(event.report)
      }
      break
    case 'progress':
      if (event.report) applyReportSnapshot(event.report)
      break
    case 'check':
      if (event.check) upsertCheck(event.check)
      break
    case 'validation':
      if (event.validation) validations.value = { ...validations.value, [event.validation.id]: event.validation }
      break
    case 'metrics':
      if (event.metrics) metrics.value = event.metrics
      break
    case 'token_audit_sample':
      if (event.sample) upsertAuditSample(event.sample)
      break
    case 'token_audit':
      if (event.token_audit) tokenAudit.value = event.token_audit
      break
    case 'report':
      if (event.report) {
        applyReportSnapshot(event.report)
        runStatus.value = event.report.status === 'error' ? 'error' : 'success'
      }
      break
    case 'error':
      errorMessage.value = event.error_message || '检测失败'
      runStatus.value = 'error'
      break
  }
}

function applyEventState(event: PurityCheckEvent) {
  if (event.step_name) stepName.value = event.step_name
  if (typeof event.progress === 'number') progress.value = normalizeProgress(event.progress)
  if (event.scores) scores.value = { ...scores.value, ...event.scores }
  if (event.metrics) metrics.value = event.metrics
  if (event.token_audit_progress) tokenAuditProgress.value = event.token_audit_progress
  if (event.token_audit_partial?.length) auditSamples.value = sortAuditSamples(event.token_audit_partial)
  if (event.token_audit) tokenAudit.value = event.token_audit
}

function applyReportSnapshot(snapshot: PurityReport) {
  report.value = snapshot
  metrics.value = snapshot.metrics || metrics.value
  if (snapshot.scores) scores.value = { ...scores.value, ...snapshot.scores }
  if (snapshot.token_audit_progress) tokenAuditProgress.value = snapshot.token_audit_progress
  if (snapshot.token_audit_partial?.length) auditSamples.value = sortAuditSamples(snapshot.token_audit_partial)
  tokenAudit.value = snapshot.token_audit || tokenAudit.value
  checks.value = snapshot.checks?.length ? snapshot.checks : checks.value
  if (snapshot.validations?.length) {
    validations.value = Object.fromEntries(snapshot.validations.map((item) => [item.id, item]))
  }
  if (snapshot.step_name) stepName.value = snapshot.step_name
  if (typeof snapshot.progress === 'number') progress.value = normalizeProgress(snapshot.progress)
}

function upsertAuditSample(sample: PurityTokenAuditSample) {
  const next = auditSamples.value.filter((item) => item.index !== sample.index)
  next.push(sample)
  auditSamples.value = sortAuditSamples(next)
}

function upsertCheck(check: PurityCheckResult) {
  const next = checks.value.filter((item) => item.id !== check.id)
  next.push(check)
  checks.value = next
}

function normalizedAuditSamples(): PurityTokenAuditSample[] {
  const source = tokenAudit.value?.samples?.length ? tokenAudit.value.samples : tokenAudit.value?.rows?.length ? tokenAudit.value.rows : auditSamples.value
  return sortAuditSamples(source)
}

function sampleBarHeight(sample: PurityTokenAuditSample): number {
  const maxTokens = Math.max(1, ...normalizedAuditSamples().map((item) => item.total_tokens || 0))
  return Math.max(8, Math.round(((sample.total_tokens || 0) / maxTokens) * 100))
}

function validationStatusLabel(status: DisplayStatus): string {
  if (status === 'pass') return '通过'
  if (status === 'warn') return '警告'
  if (status === 'fail') return '失败'
  if (status === 'running') return '检测中'
  return '等待'
}

function validationIcon(status: DisplayStatus): IconName {
  if (status === 'pass') return 'checkCircle'
  if (status === 'warn') return 'exclamationTriangle'
  if (status === 'fail') return 'xCircle'
  if (status === 'running') return 'refresh'
  return 'clock'
}

function validationCardClass(status: DisplayStatus): string {
  if (status === 'pass') return 'border-emerald-200 dark:border-emerald-500/40'
  if (status === 'warn') return 'border-amber-200 dark:border-amber-500/40'
  if (status === 'fail') return 'border-red-200 dark:border-red-500/40'
  if (status === 'running') return 'border-primary-200 dark:border-primary-500/40'
  return 'border-gray-200 dark:border-dark-500'
}

function validationIconClass(status: DisplayStatus): string {
  if (status === 'pass') return 'bg-emerald-50 text-emerald-600 dark:bg-emerald-900/25 dark:text-emerald-300'
  if (status === 'warn') return 'bg-amber-50 text-amber-600 dark:bg-amber-900/25 dark:text-amber-300'
  if (status === 'fail') return 'bg-red-50 text-red-600 dark:bg-red-900/25 dark:text-red-300'
  if (status === 'running') return 'bg-primary-50 text-primary-600 dark:bg-primary-900/25 dark:text-primary-300'
  return 'bg-gray-100 text-gray-400 dark:bg-dark-600 dark:text-dark-400'
}

function validationBadgeClass(status: DisplayStatus): string {
  if (status === 'pass') return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
  if (status === 'warn') return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
  if (status === 'fail') return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300'
  if (status === 'running') return 'bg-primary-100 text-primary-700 dark:bg-primary-900/30 dark:text-primary-300'
  return 'bg-gray-100 text-gray-500 dark:bg-dark-600 dark:text-dark-300'
}

function checkStatusClass(status: PurityCheckStatus): string {
  if (status === 'pass') return 'text-emerald-600 dark:text-emerald-300'
  if (status === 'warn') return 'text-amber-600 dark:text-amber-300'
  return 'text-red-600 dark:text-red-300'
}

function latencyLabel(value?: number): string {
  if (!value || value < 0) return '-'
  return `${Math.round(value)} ms`
}

function formatMultiplier(value?: number): string {
  if (!value) return '-'
  return `${value.toFixed(2)}x`
}

function formatUSD(value?: number): string {
  if (!value) return '$0'
  return `$${value.toFixed(6).replace(/0+$/, '').replace(/\.$/, '.0')}`
}

function formatPercent(value?: number): string {
  if (!value) return '-'
  return `${Math.round(value > 1 ? value : value * 100)}%`
}

function normalizeAccountProvider(platform?: string): PurityProvider | null {
  const value = (platform || '').toLowerCase()
  if (value === 'openai') return 'openai'
  if (value === 'anthropic' || value === 'claude') return 'anthropic'
  return null
}

function normalizeProgress(value?: number): number {
  if (!value || value < 0) return 0
  if (value > 1) return Math.min(1, value / 100)
  return value
}

function findPreferredModel(models: LocalAccountTestModel[], candidates: string[]): string {
  for (const candidate of candidates) {
    const exact = models.find((model) => model.id === candidate)
    if (exact) return exact.id
    const fuzzy = models.find((model) => model.id.toLowerCase().includes(candidate))
    if (fuzzy) return fuzzy.id
  }
  return models[0]?.id || ''
}

function validationDisplayName(definition: ValidationDefinition): string {
  if (currentProvider.value !== 'anthropic') return definition.name
  if (definition.id === 'schema_integrity') return 'Messages 结构完整性'
  if (definition.id === 'multimodal') return 'Image Block 多模态'
  return definition.name
}

function validationWaitingMessage(definition: ValidationDefinition): string {
  if (currentProvider.value !== 'anthropic') return definition.message
  if (definition.id === 'schema_integrity') return '等待 Messages schema 探测'
  if (definition.id === 'multimodal') return '等待 image block 探测'
  return definition.message
}

function sortAuditSamples(samples: PurityTokenAuditSample[]): PurityTokenAuditSample[] {
  return [...samples].sort((a, b) => a.index - b.index)
}

function auditCachedTokens(sample: PurityTokenAuditSample): number {
  return sample.cached_tokens || sample.cache_read_input_tokens || 0
}

function auditCacheCreationTokens(sample: PurityTokenAuditSample): number {
  return sample.cache_creation_tokens || sample.cache_creation_input_tokens || 0
}

function auditRatio(sample: PurityTokenAuditSample): number | undefined {
  return sample.multiplier || sample.ratio
}
</script>

<style scoped>
.score-ring {
  display: grid;
  width: 128px;
  height: 128px;
  place-items: center;
  border-radius: 9999px;
  background: conic-gradient(#14b8a6 var(--score-angle), #e5e7eb 0);
}

.score-ring-inner {
  display: grid;
  width: 96px;
  height: 96px;
  place-items: center;
  border-radius: 9999px;
  background: #fff;
}

:global(.dark) .score-ring {
  background: conic-gradient(#2dd4bf var(--score-angle), #374151 0);
}

:global(.dark) .score-ring-inner {
  background: #1f2937;
}
</style>
