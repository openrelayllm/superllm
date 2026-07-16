<template>
  <div class="space-y-4">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <h2 class="text-base font-semibold text-gray-900 dark:text-gray-100">Key 对齐</h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">按供应商分组核对第三方 Key、本地 Sub2API 账号和绑定关系。</p>
      </div>
      <div class="flex flex-wrap items-center gap-2">
        <button type="button" class="btn btn-secondary" :disabled="loading" @click="loadData">
          <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
          刷新对齐
        </button>
        <button type="button" class="btn btn-primary" :disabled="actionsDisabled" :title="actionsDisabled ? '数据加载不完整，请刷新重试' : undefined" @click="emit('open-groups')">
          <Icon name="key" size="sm" />
          去创建缺失 Key
        </button>
      </div>
    </div>

    <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-5">
      <div v-for="stat in stats" :key="stat.label" class="border border-gray-200 bg-white px-4 py-3 dark:border-dark-700 dark:bg-dark-800">
        <div class="text-xs text-gray-500 dark:text-dark-400">{{ stat.label }}</div>
        <div class="mt-1 text-xl font-semibold" :class="stat.valueClass">{{ stat.value }}</div>
        <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ stat.caption }}</div>
      </div>
    </div>

    <div v-if="partialError" role="alert" class="border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-900 dark:border-amber-900/60 dark:bg-amber-950/20 dark:text-amber-100">
      {{ partialError }}
    </div>
    <div v-if="error" class="border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-200">
      {{ error }}
    </div>

    <section class="border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800">
      <div class="flex flex-wrap items-end justify-between gap-3 border-b border-gray-100 px-4 py-3 dark:border-dark-700">
        <label class="block min-w-[260px] flex-1">
          <span class="input-label">搜索分组、Key 或本地账号</span>
          <div class="relative">
            <Icon name="search" size="sm" class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
            <input v-model.trim="query" class="input pl-9" placeholder="名称、ID、末四位" />
          </div>
        </label>
        <label class="block w-full sm:w-44">
          <span class="input-label">显示范围</span>
          <select v-model="scope" class="input">
            <option value="all">全部关系</option>
            <option value="attention">只看未对齐</option>
            <option value="aligned">只看已对齐</option>
          </select>
        </label>
        <span class="pb-2 text-xs text-gray-500 dark:text-dark-400">显示 {{ filteredRows.length }} / {{ rows.length }} 条</span>
      </div>

      <div v-if="loading" class="flex items-center justify-center gap-2 py-16 text-sm text-gray-500 dark:text-dark-400">
        <Icon name="refresh" size="sm" class="animate-spin" />
        正在加载供应商分组、Key 和本地账号...
      </div>
      <EmptyState
        v-else-if="filteredRows.length === 0"
        :title="rows.length === 0 ? '还没有可对齐的数据' : '没有匹配的关系'"
        :description="rows.length === 0 ? '先同步供应商分组，或刷新后再检查。' : '调整搜索条件或显示范围。'"
        :action-text="rows.length === 0 ? '查看分组' : '显示全部关系'"
        @action="rows.length === 0 ? emit('open-groups') : resetFilters()"
      />
      <div v-else class="overflow-x-auto">
        <table class="min-w-[1120px] w-full divide-y divide-gray-200 text-sm dark:divide-dark-700">
          <thead class="bg-gray-50 dark:bg-dark-900/60">
            <tr>
              <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-dark-400">供应商分组</th>
              <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-dark-400">第三方 Key</th>
              <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-dark-400">Sub2API 本地账号</th>
              <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-dark-400">对齐结果</th>
              <th class="w-[130px] px-4 py-3 text-right text-xs font-medium text-gray-500 dark:text-dark-400">操作</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-200 dark:divide-dark-700">
            <tr v-for="row in filteredRows" :key="row.id" class="align-top">
              <td class="px-4 py-3">
                <template v-if="row.group">
                  <div class="font-medium text-gray-900 dark:text-gray-100">{{ row.group.name || '-' }}</div>
                  <div class="mt-1 flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-dark-400">
                    <span class="font-mono">#{{ row.group.external_group_id }}</span>
                    <span>{{ row.group.provider_family || 'mixed' }}</span>
                    <span class="badge badge-gray">{{ groupStatusLabel(row.group.status) }}</span>
                  </div>
                </template>
                <span v-else class="badge badge-warning">分组已不存在</span>
              </td>
              <td class="px-4 py-3">
                <template v-if="row.key">
                  <div class="font-medium text-gray-900 dark:text-gray-100">{{ row.key.name || `Key #${row.key.id}` }}</div>
                  <div class="mt-1 flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-dark-400">
                    <span v-if="row.key.external_key_id" class="font-mono">#{{ row.key.external_key_id }}</span>
                    <span v-if="row.key.key_last4" class="font-mono">****{{ row.key.key_last4 }}</span>
                    <span class="badge" :class="keyStatusClass(row.key.status)">{{ keyStatusLabel(row.key.status) }}</span>
                  </div>
                </template>
                <span v-else class="badge badge-danger">缺少第三方 Key</span>
              </td>
              <td class="px-4 py-3">
                <template v-if="row.local">
                  <div class="font-medium text-gray-900 dark:text-gray-100">{{ row.local.name || `账号 #${row.local.id}` }}</div>
                  <div class="mt-1 flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-dark-400">
                    <span class="font-mono">#{{ row.local.id }}</span>
                    <span>{{ row.local.platform || '-' }}</span>
                    <span class="badge" :class="localStatusClass(row.local.status)">{{ localStatusLabel(row.local.status) }}</span>
                  </div>
                </template>
                <template v-else-if="row.localAccountID">
                  <span class="badge badge-danger">找不到账号 #{{ row.localAccountID }}</span>
                </template>
                <span v-else class="badge badge-gray">未绑定本地账号</span>
              </td>
              <td class="px-4 py-3">
                <span class="badge" :class="alignmentStatusClass(row.alignmentStatus)">{{ alignmentStatusLabel(row.alignmentStatus) }}</span>
                <div v-if="row.binding" class="mt-1 text-xs text-gray-500 dark:text-dark-400">绑定记录 #{{ row.binding.id }}</div>
              </td>
              <td class="px-4 py-3 text-right">
                <button v-if="row.canCreate" type="button" class="btn btn-secondary btn-sm" :disabled="actionsDisabled" @click="emit('create-key', row.group!)">
                  <Icon name="key" size="xs" />
                  创建 Key
                </button>
                <button v-else-if="row.canRepair" type="button" class="btn btn-secondary btn-sm" :disabled="actionsDisabled" @click="emit('repair-key', row.key!)">
                  <Icon name="link" size="xs" />
                  完成绑定
                </button>
                <RouterLink v-else-if="row.canManageLocal" :to="{ path: '/admin/local-account-ops', query: { supplier_id: supplierId } }" class="btn btn-secondary btn-sm">
                  <Icon name="externalLink" size="xs" />
                  查看账号
                </RouterLink>
                <span v-else class="text-xs text-gray-400 dark:text-dark-500">无需处理</span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { listLocalSub2APIAccounts, listSupplierAccounts, listSupplierGroups, listSupplierKeys } from '@/api/admin/adminPlus'
import type { LocalSub2APIAccount, SupplierAccount, SupplierGroup, SupplierKey } from '@/api/admin/adminPlus'
import EmptyState from '@/components/common/EmptyState.vue'
import Icon from '@/components/icons/Icon.vue'
import { loadAllPagedItems } from '@/utils/loadAllPages'
import { buildSupplierAlignmentRows, supplierAlignmentStatusLabel as alignmentStatusLabel } from '../supplierAlignmentPresentation'
import type { SupplierAlignmentRow, SupplierAlignmentStatus } from '../supplierAlignmentPresentation'

type AlignmentScope = 'all' | 'attention' | 'aligned'

interface StatItem {
  label: string
  value: number
  caption: string
  valueClass: string
}

const props = defineProps<{ supplierId: number }>()
const emit = defineEmits<{
  (event: 'open-groups'): void
  (event: 'create-key', group: SupplierGroup): void
  (event: 'repair-key', key: SupplierKey): void
}>()

const groups = ref<SupplierGroup[]>([])
const keys = ref<SupplierKey[]>([])
const bindings = ref<SupplierAccount[]>([])
const localAccounts = ref<LocalSub2APIAccount[]>([])
const loading = ref(false)
const error = ref('')
const partialError = ref('')
const query = ref('')
const scope = ref<AlignmentScope>('all')
let loadSequence = 0

const actionsDisabled = computed(() => loading.value || Boolean(partialError.value || error.value))
const rows = computed<SupplierAlignmentRow[]>(() => buildSupplierAlignmentRows(groups.value, keys.value, bindings.value, localAccounts.value))
const filteredRows = computed(() => {
  const keyword = query.value.toLowerCase()
  return rows.value
    .filter((row) => scope.value === 'all' || (scope.value === 'aligned' ? row.alignmentStatus === 'aligned' : row.alignmentStatus !== 'aligned'))
    .filter((row) => {
      if (!keyword) return true
      const text = [
        row.group?.name,
        row.group?.external_group_id,
        row.key?.name,
        row.key?.external_key_id,
        row.key?.key_last4,
        row.local?.name,
        row.local?.id,
        row.localAccountID
      ].map((value) => String(value || '').toLowerCase()).join(' ')
      return text.includes(keyword)
    })
    .sort((a, b) => {
      if ((a.alignmentStatus === 'aligned') !== (b.alignmentStatus === 'aligned')) return a.alignmentStatus === 'aligned' ? 1 : -1
      return (a.group?.name || a.key?.name || '').localeCompare(b.group?.name || b.key?.name || '')
    })
})

const stats = computed<StatItem[]>(() => {
  const localIDs = new Set<number>()
  for (const row of rows.value) {
    if (row.local?.id) localIDs.add(row.local.id)
    if (row.localAccountID) localIDs.add(row.localAccountID)
  }
  const aligned = rows.value.filter((row) => row.alignmentStatus === 'aligned').length
  const activeGroups = groups.value.filter((group) => group.status === 'active').length
  return [
    { label: '有效分组', value: activeGroups, caption: `共 ${groups.value.length} 个供应商分组`, valueClass: 'text-gray-900 dark:text-gray-100' },
    { label: '第三方 Key', value: keys.value.length, caption: '已读取的 Key', valueClass: 'text-gray-900 dark:text-gray-100' },
    { label: '本地账号', value: localIDs.size, caption: '参与供应商绑定的账号', valueClass: 'text-gray-900 dark:text-gray-100' },
    { label: '已对齐', value: aligned, caption: 'Key、绑定和账号均正常', valueClass: 'text-emerald-700 dark:text-emerald-300' },
    { label: '待处理', value: Math.max(0, rows.value.length - aligned), caption: '需要创建或修复', valueClass: 'text-amber-700 dark:text-amber-300' }
  ]
})

async function loadData() {
  const sequence = ++loadSequence
  loading.value = true
  error.value = ''
  partialError.value = ''
  try {
    const results = await Promise.allSettled([
      loadAllPagedItems((page, pageSize) => listSupplierGroups(props.supplierId, { page, page_size: pageSize })),
      loadAllPagedItems((page, pageSize) => listSupplierKeys(props.supplierId, { page, page_size: pageSize })),
      loadAllPagedItems((page, pageSize) => listSupplierAccounts(props.supplierId, { page, page_size: pageSize })),
      loadAllPagedItems((page, pageSize) => listLocalSub2APIAccounts({ page, page_size: pageSize }))
    ])
    if (sequence !== loadSequence) return

    const labels = ['供应商分组', '第三方 Key', '绑定记录', '本地账号']
    const failed: string[] = []
    const values = results.map((result, index) => {
      if (result.status === 'fulfilled') return result.value
      failed.push(labels[index])
      return []
    })
    groups.value = values[0] as SupplierGroup[]
    keys.value = values[1] as SupplierKey[]
    bindings.value = values[2] as SupplierAccount[]
    localAccounts.value = values[3] as LocalSub2APIAccount[]
    if (failed.length > 0) partialError.value = `${failed.join('、')}加载失败，当前结果可能不完整；创建和修复操作已禁用，请刷新重试。`
    if (failed.length === results.length) error.value = '供应商对齐数据加载失败，请稍后重试。'
  } catch (cause) {
    if (sequence === loadSequence) error.value = (cause as { message?: string }).message || '供应商对齐数据加载失败'
  } finally {
    if (sequence === loadSequence) loading.value = false
  }
}

function resetFilters() {
  query.value = ''
  scope.value = 'all'
}

function alignmentStatusClass(value: SupplierAlignmentStatus): string {
  if (value === 'aligned') return 'badge-success'
  if (['key_provisioning'].includes(value)) return 'badge-primary'
  if (['missing_key', 'unbound_key', 'missing_local_account', 'local_account_disabled', 'binding_mismatch', 'key_manual_required', 'group_disabled'].includes(value)) return 'badge-warning'
  return 'badge-danger'
}

function keyStatusLabel(value: string): string {
  return ({ provisioning: '创建中', bound: '已绑定', manual_secret_required: '需补密钥', failed: '失败', disabled: '已禁用' } as Record<string, string>)[value] || value || '-'
}

function keyStatusClass(value: string): string {
  if (value === 'bound') return 'badge-success'
  if (value === 'provisioning') return 'badge-primary'
  if (value === 'manual_secret_required') return 'badge-warning'
  return 'badge-danger'
}

function groupStatusLabel(value: string): string {
  return ({ active: '有效', missing: '已缺失', disabled: '停用' } as Record<string, string>)[value] || value || '-'
}

function localStatusLabel(value: string): string {
  return ({ active: '正常', enabled: '正常', schedulable: '可调度', paused: '已暂停', disabled: '已停用', error: '异常', failed: '异常', expired: '已过期' } as Record<string, string>)[String(value).toLowerCase()] || value || '-'
}

function localStatusClass(value: string): string {
  const normalized = String(value || '').toLowerCase()
  if (['active', 'enabled', 'schedulable', 'normal'].includes(normalized)) return 'badge-success'
  if (['paused', 'rate_limited'].includes(normalized)) return 'badge-warning'
  return 'badge-danger'
}

watch(() => props.supplierId, () => void loadData(), { immediate: true })
</script>
