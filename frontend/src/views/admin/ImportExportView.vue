<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">导入/导出</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
            用核心配置 JSON 迁移服务器，排除日志、运行记录、监控快照和审计事件。
          </p>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <button type="button" class="btn btn-secondary" :disabled="exporting" @click="downloadArchive">
            <Icon name="download" size="sm" />
            {{ exporting ? '导出中...' : '导出 JSON' }}
          </button>
          <button type="button" class="btn btn-primary" :disabled="!canImport" @click="submitImport">
            <Icon name="upload" size="sm" />
            {{ importing ? '导入中...' : '确认导入' }}
          </button>
        </div>
      </section>

      <section class="grid gap-4 md:grid-cols-4">
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">核心表</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ coreTableCount }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">核心行数</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ preview?.summary.rows ?? '-' }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">忽略表</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ ignoredTables.length || '-' }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">文件大小</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ archiveSizeLabel }}</p>
        </div>
      </section>

      <section class="grid gap-6 xl:grid-cols-[minmax(0,0.95fr)_minmax(0,1.05fr)]">
        <div class="card overflow-hidden">
          <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">导出</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">导出核心业务配置和迁移所需密文。</p>
          </div>
          <div class="space-y-4 p-5">
            <div class="rounded-md border border-amber-200 bg-amber-50 px-4 py-3 dark:border-amber-900/60 dark:bg-amber-950/30">
              <div class="flex gap-3">
                <Icon name="exclamationTriangle" size="sm" class="mt-0.5 shrink-0 text-amber-600 dark:text-amber-400" />
                <div class="text-sm text-amber-800 dark:text-amber-300">
                  导出文件包含账号凭据密文和 API Key。浏览器会话、调度运行、成本流水和审计日志不会导出。
                </div>
              </div>
            </div>

            <div class="grid gap-3 sm:grid-cols-2">
              <div class="rounded-md border border-gray-200 p-4 dark:border-dark-700">
                <p class="text-sm font-medium text-gray-900 dark:text-white">包含</p>
                <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">
                  {{ scopeIncludedLabel }}
                </p>
              </div>
              <div class="rounded-md border border-gray-200 p-4 dark:border-dark-700">
                <p class="text-sm font-medium text-gray-900 dark:text-white">排除</p>
                <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">
                  {{ scopeExcludedLabel }}
                </p>
              </div>
            </div>

            <div class="rounded-md border border-gray-200 dark:border-dark-700">
              <div class="flex flex-col gap-3 border-b border-gray-100 px-4 py-3 dark:border-dark-700 sm:flex-row sm:items-center sm:justify-between">
                <div>
                  <p class="text-sm font-semibold text-gray-900 dark:text-white">迁移范围</p>
                  <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                    {{ scopeSummaryLabel }}
                  </p>
                </div>
                <div class="inline-flex rounded-md border border-gray-200 bg-white p-1 dark:border-dark-700 dark:bg-dark-900">
                  <button
                    type="button"
                    class="rounded px-3 py-1 text-xs font-medium"
                    :class="scopeTab === 'included' ? 'bg-primary-600 text-white' : 'text-gray-600 hover:text-gray-900 dark:text-dark-300 dark:hover:text-white'"
                    @click="scopeTab = 'included'"
                  >
                    包含
                  </button>
                  <button
                    type="button"
                    class="rounded px-3 py-1 text-xs font-medium"
                    :class="scopeTab === 'excluded' ? 'bg-primary-600 text-white' : 'text-gray-600 hover:text-gray-900 dark:text-dark-300 dark:hover:text-white'"
                    @click="scopeTab = 'excluded'"
                  >
                    排除
                  </button>
                </div>
              </div>
              <div class="space-y-3 p-4">
                <input v-model.trim="scopeQuery" class="input" placeholder="搜索表名" />
                <div class="max-h-56 overflow-auto">
                  <table class="w-full min-w-[640px] divide-y divide-gray-200 text-sm dark:divide-dark-700">
                    <thead class="bg-gray-50 dark:bg-dark-800">
                      <tr>
                        <th class="px-3 py-2 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">表</th>
                        <th class="px-3 py-2 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">说明</th>
                      </tr>
                    </thead>
                    <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-900">
                      <tr v-if="scopeRows.length === 0">
                        <td colspan="2" class="px-3 py-6 text-center text-sm text-gray-500 dark:text-dark-400">
                          {{ scopeLoading ? '加载中...' : '暂无匹配表' }}
                        </td>
                      </tr>
                      <tr v-for="table in scopeRows" :key="`${scopeTab}:${table.name}`">
                        <td class="px-3 py-2 font-mono text-xs text-gray-900 dark:text-white">
                          {{ table.name }}
                          <span v-if="table.sensitive" class="ml-2 badge badge-warning">敏感</span>
                        </td>
                        <td class="px-3 py-2 text-gray-500 dark:text-dark-400">{{ table.description || table.reason || '-' }}</td>
                      </tr>
                    </tbody>
                  </table>
                </div>
              </div>
            </div>

            <button type="button" class="btn btn-primary w-full justify-center" :disabled="exporting" @click="downloadArchive">
              <Icon name="download" size="sm" />
              {{ exporting ? '正在生成 JSON...' : '下载迁移 JSON' }}
            </button>
          </div>
        </div>

        <div class="card overflow-hidden">
          <div class="flex flex-col gap-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">导入</h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">导入使用 upsert，不会清空目标库。</p>
            </div>
            <div class="flex flex-wrap gap-2">
              <input ref="fileInput" type="file" accept="application/json,.json" class="hidden" @change="handleFileChange" />
              <button type="button" class="btn btn-secondary btn-sm" @click="openFilePicker">
                <Icon name="upload" size="sm" />
                选择文件
              </button>
              <button type="button" class="btn btn-secondary btn-sm" :disabled="previewing || !archiveText.trim()" @click="previewArchive">
                <Icon name="eye" size="sm" />
                {{ previewing ? '预览中...' : '预览' }}
              </button>
            </div>
          </div>

          <div class="space-y-4 p-5">
            <div
              class="rounded-md border border-dashed px-4 py-5 transition"
              :class="dragActive ? 'border-primary-400 bg-primary-50 dark:border-primary-500 dark:bg-primary-950/20' : 'border-gray-300 bg-gray-50 dark:border-dark-700 dark:bg-dark-800/60'"
              @dragenter.prevent="dragActive = true"
              @dragover.prevent="dragActive = true"
              @dragleave.prevent="dragActive = false"
              @drop.prevent="handleDrop"
            >
              <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                <div>
                  <p class="text-sm font-medium text-gray-900 dark:text-white">{{ selectedFileName || '迁移 JSON' }}</p>
                  <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">拖入文件，或直接在下方粘贴 JSON。</p>
                </div>
                <button type="button" class="btn btn-secondary btn-sm shrink-0" @click="openFilePicker">浏览</button>
              </div>
            </div>

            <label class="block">
              <span class="input-label">JSON 内容</span>
              <textarea
                v-model="archiveText"
                class="input min-h-[220px] resize-y font-mono text-xs leading-5"
                spellcheck="false"
                placeholder="{&quot;version&quot;:1,&quot;product&quot;:&quot;superllm&quot;,...}"
                @input="clearImportState"
              />
            </label>

            <div v-if="preview" class="rounded-md border border-gray-200 dark:border-dark-700">
              <div class="flex flex-col gap-2 border-b border-gray-100 px-4 py-3 dark:border-dark-700 sm:flex-row sm:items-center sm:justify-between">
                <div>
                  <p class="text-sm font-semibold text-gray-900 dark:text-white">预览结果</p>
                  <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                    {{ preview.product }} v{{ preview.version }} · {{ formatDateTime(preview.exported_at) || '未标记导出时间' }}
                  </p>
                </div>
                <span class="badge badge-success">可导入</span>
              </div>
              <div class="max-h-[280px] overflow-auto">
                <table class="w-full min-w-[720px] divide-y divide-gray-200 text-sm dark:divide-dark-700">
                  <thead class="bg-gray-50 dark:bg-dark-800">
                    <tr>
                      <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">表</th>
                      <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">行数</th>
                      <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">范围</th>
                    </tr>
                  </thead>
                  <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-900">
                    <tr v-for="table in includedTables" :key="table.name">
                      <td class="px-4 py-3 font-mono text-xs text-gray-900 dark:text-white">{{ table.name }}</td>
                      <td class="px-4 py-3 text-gray-700 dark:text-dark-200">{{ table.rows }}</td>
                      <td class="px-4 py-3 text-gray-500 dark:text-dark-400">
                        {{ table.description || '-' }}
                        <span v-if="table.sensitive" class="ml-2 badge badge-warning">敏感</span>
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>

            <div v-if="ignoredTables.length" class="rounded-md border border-gray-200 dark:border-dark-700">
              <div class="border-b border-gray-100 px-4 py-3 dark:border-dark-700">
                <p class="text-sm font-semibold text-gray-900 dark:text-white">已忽略</p>
              </div>
              <ul class="max-h-36 divide-y divide-gray-100 overflow-auto text-sm dark:divide-dark-700">
                <li v-for="table in ignoredTables" :key="table.name" class="flex gap-3 px-4 py-2">
                  <span class="min-w-0 flex-1 truncate font-mono text-xs text-gray-700 dark:text-dark-200">{{ table.name }}</span>
                  <span class="shrink-0 text-xs text-gray-500 dark:text-dark-400">{{ table.rows }} 行</span>
                </li>
              </ul>
            </div>

            <div v-if="preview?.warnings?.length" class="rounded-md border border-amber-200 bg-amber-50 px-4 py-3 dark:border-amber-900/60 dark:bg-amber-950/30">
              <ul class="space-y-1 text-sm text-amber-800 dark:text-amber-300">
                <li v-for="warning in preview.warnings" :key="warning">{{ warning }}</li>
              </ul>
            </div>

            <div class="grid gap-3 sm:grid-cols-[1fr_auto] sm:items-end">
              <label class="block">
                <span class="input-label">导入确认</span>
                <input v-model.trim="confirmation" class="input font-mono" placeholder="IMPORT" @input="importResult = null" />
              </label>
              <button type="button" class="btn btn-primary justify-center" :disabled="!canImport" @click="submitImport">
                <Icon name="upload" size="sm" />
                {{ importing ? '导入中...' : '导入核心配置' }}
              </button>
            </div>

            <div v-if="importResult" class="rounded-md border border-emerald-200 bg-emerald-50 px-4 py-3 dark:border-emerald-900/60 dark:bg-emerald-950/30">
              <p class="text-sm font-medium text-emerald-800 dark:text-emerald-300">
                已导入 {{ importedRows }} 行核心配置，{{ importResult.tables.length }} 张表完成处理。
              </p>
              <div class="mt-3 max-h-44 overflow-auto rounded border border-emerald-200 bg-white/70 dark:border-emerald-900/60 dark:bg-dark-900/60">
                <table class="w-full min-w-[560px] divide-y divide-emerald-100 text-sm dark:divide-emerald-900/50">
                  <thead>
                    <tr>
                      <th class="px-3 py-2 text-left text-xs font-medium uppercase text-emerald-700 dark:text-emerald-300">表</th>
                      <th class="px-3 py-2 text-left text-xs font-medium uppercase text-emerald-700 dark:text-emerald-300">导入</th>
                      <th class="px-3 py-2 text-left text-xs font-medium uppercase text-emerald-700 dark:text-emerald-300">状态</th>
                    </tr>
                  </thead>
                  <tbody class="divide-y divide-emerald-100 dark:divide-emerald-900/50">
                    <tr v-for="table in importResult.tables" :key="table.name">
                      <td class="px-3 py-2 font-mono text-xs text-emerald-950 dark:text-emerald-100">{{ table.name }}</td>
                      <td class="px-3 py-2 text-emerald-800 dark:text-emerald-200">{{ table.imported }} / {{ table.rows }}</td>
                      <td class="px-3 py-2 text-emerald-800 dark:text-emerald-200">{{ table.skipped ? table.reason || '跳过' : '完成' }}</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>
          </div>
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import {
  adminPlusAPI,
  type ImportExportArchive,
  type ImportExportPreview,
  type ImportExportResult,
  type ImportExportScope
} from '@/api/admin/adminPlus'

const appStore = useAppStore()

const fileInput = ref<HTMLInputElement | null>(null)
const archiveText = ref('')
const selectedFileName = ref('')
const scope = ref<ImportExportScope | null>(null)
const scopeLoading = ref(false)
const scopeTab = ref<'included' | 'excluded'>('included')
const scopeQuery = ref('')
const preview = ref<ImportExportPreview | null>(null)
const importResult = ref<ImportExportResult | null>(null)
const confirmation = ref('')
const exporting = ref(false)
const previewing = ref(false)
const importing = ref(false)
const dragActive = ref(false)

const includedTables = computed(() => preview.value?.included_tables || [])
const ignoredTables = computed(() => preview.value?.ignored_tables || [])
const importedRows = computed(() => importResult.value?.tables.reduce((sum, table) => sum + table.imported, 0) || 0)
const canImport = computed(() => Boolean(preview.value?.valid && confirmation.value === 'IMPORT' && !importing.value))
const coreTableCount = computed(() => preview.value?.summary.tables ?? scope.value?.summary.included ?? '-')
const scopeSummaryLabel = computed(() => {
  if (!scope.value) return scopeLoading.value ? '加载中...' : '范围信息暂不可用'
  return `包含 ${scope.value.summary.included} 张表，排除 ${scope.value.summary.excluded} 张运行/日志表，敏感表 ${scope.value.summary.sensitive} 张`
})
const scopeRows = computed(() => {
  const rows = scopeTab.value === 'included' ? scope.value?.included_tables || [] : scope.value?.excluded_tables || []
  const query = scopeQuery.value.toLowerCase()
  if (!query) return rows
  return rows.filter((table) =>
    [table.name, table.description || '', table.reason || ''].some((value) => value.toLowerCase().includes(query))
  )
})
const scopeIncludedLabel = computed(() => {
  if (!scope.value) return '设置、分组、账号、Key、渠道、供应商、调度计划和通知。用户身份不在迁移范围内。'
  return previewTableNames(scope.value.included_tables)
})
const scopeExcludedLabel = computed(() => {
  if (!scope.value) return '使用日志、任务运行、健康采样、余额事件、成本流水、通知投递和本机运行态。'
  return previewTableNames(scope.value.excluded_tables)
})
const archiveSizeLabel = computed(() => {
  if (!archiveText.value) return '-'
  return formatBytes(new Blob([archiveText.value]).size)
})

onMounted(() => {
  void loadScope()
})

async function loadScope() {
  scopeLoading.value = true
  try {
    scope.value = await adminPlusAPI.getMigrationScope()
  } catch (error) {
    appStore.showError(errorMessage(error, '迁移范围加载失败'))
  } finally {
    scopeLoading.value = false
  }
}

async function downloadArchive() {
  exporting.value = true
  try {
    const archive = await adminPlusAPI.exportMigrationArchive()
    const text = JSON.stringify(archive, null, 2)
    const filename = `superllm-core-${new Date().toISOString().replace(/[:.]/g, '-')}.json`
    downloadTextFile(filename, text)
    appStore.showSuccess('迁移 JSON 已生成')
  } catch (error) {
    appStore.showError(errorMessage(error, '导出失败'))
  } finally {
    exporting.value = false
  }
}

function openFilePicker() {
  fileInput.value?.click()
}

async function handleFileChange(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (file) {
    await loadFile(file)
  }
  input.value = ''
}

async function handleDrop(event: DragEvent) {
  dragActive.value = false
  const file = event.dataTransfer?.files?.[0]
  if (file) {
    await loadFile(file)
  }
}

async function loadFile(file: File) {
  if (!file.name.endsWith('.json') && file.type !== 'application/json') {
    appStore.showWarning('请选择 JSON 文件')
    return
  }
  selectedFileName.value = file.name
  archiveText.value = await file.text()
  clearImportState()
  await previewArchive()
}

async function previewArchive() {
  const archive = parseArchiveText()
  if (!archive) return
  previewing.value = true
  try {
    preview.value = await adminPlusAPI.previewMigrationArchive(archive)
    importResult.value = null
    appStore.showSuccess('预览完成')
  } catch (error) {
    preview.value = null
    appStore.showError(errorMessage(error, '预览失败'))
  } finally {
    previewing.value = false
  }
}

async function submitImport() {
  const archive = parseArchiveText()
  if (!archive || !canImport.value) return
  importing.value = true
  try {
    importResult.value = await adminPlusAPI.importMigrationArchive(archive)
    confirmation.value = ''
    appStore.showSuccess('核心配置导入完成')
  } catch (error) {
    appStore.showError(errorMessage(error, '导入失败'))
  } finally {
    importing.value = false
  }
}

function parseArchiveText(): ImportExportArchive | null {
  const text = archiveText.value.trim()
  if (!text) {
    appStore.showWarning('请先提供迁移 JSON')
    return null
  }
  try {
    return JSON.parse(text) as ImportExportArchive
  } catch {
    appStore.showError('JSON 格式无效')
    return null
  }
}

function clearImportState() {
  preview.value = null
  importResult.value = null
  confirmation.value = ''
}

function downloadTextFile(filename: string, text: string) {
  const blob = new Blob([text], { type: 'application/json;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  document.body.appendChild(link)
  link.click()
  link.remove()
  URL.revokeObjectURL(url)
}

function formatBytes(bytes: number): string {
  if (bytes <= 0) return '-'
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}

function formatDateTime(value?: string): string {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}

function previewTableNames(rows: Array<{ name: string }>): string {
  if (rows.length === 0) return '-'
  const head = rows.slice(0, 8).map((row) => row.name).join('、')
  const rest = rows.length - 8
  return rest > 0 ? `${head} 等 ${rows.length} 张表。` : `${head}。`
}

function errorMessage(error: unknown, fallback: string): string {
  if (error && typeof error === 'object' && 'message' in error) {
    const message = String((error as { message?: unknown }).message || '').trim()
    if (message) return message
  }
  return fallback
}
</script>
