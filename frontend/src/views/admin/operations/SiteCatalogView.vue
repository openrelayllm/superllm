<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">网址目录</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
            未来前台网址导航的站点主数据，和采集候选保持独立。
          </p>
        </div>
        <div class="flex flex-wrap gap-2">
          <button type="button" class="btn btn-primary" :disabled="loading || publishing || pagination.total === 0" @click="publishAllMatchingSites">
            {{ publishButtonLabel }}
          </button>
          <button type="button" class="btn btn-secondary" :disabled="loading || publishing" @click="loadSites">
            <Icon name="refresh" size="sm" />
            刷新
          </button>
        </div>
      </section>

      <section class="grid gap-4 sm:grid-cols-2 xl:grid-cols-5">
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">目录站点</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ pagination.total }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">当前页已发布</p>
          <p class="mt-2 text-2xl font-semibold text-emerald-600 dark:text-emerald-400">{{ statusCount('published') }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">当前页草稿</p>
          <p class="mt-2 text-2xl font-semibold text-amber-600 dark:text-amber-400">{{ statusCount('draft') }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">当前页 new-api</p>
          <p class="mt-2 text-2xl font-semibold text-primary-600 dark:text-primary-400">{{ providerCount('new_api') }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">当前页 sub2api</p>
          <p class="mt-2 text-2xl font-semibold text-purple-600 dark:text-purple-400">{{ providerCount('sub2api') }}</p>
        </div>
      </section>

      <section class="card overflow-hidden">
        <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
          <div class="grid gap-3 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-end">
            <div>
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">站点列表</h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">目录站点默认来自采集候选“加入目录”，也可以后续人工维护。</p>
            </div>
            <div class="grid gap-2 sm:grid-cols-2 xl:grid-cols-4">
              <input v-model.trim="filters.q" class="input h-9 py-1 text-sm" placeholder="搜索名称或域名" @keyup.enter="resetPagination" />
              <select v-model="filters.status" class="input h-9 py-1 text-sm" @change="resetPagination">
                <option value="">全部状态</option>
                <option value="draft">草稿</option>
                <option value="reviewing">待审核</option>
                <option value="published">已发布</option>
                <option value="archived">已归档</option>
              </select>
              <select v-model="filters.provider_type" class="input h-9 py-1 text-sm" @change="resetPagination">
                <option value="">全部类型</option>
                <option value="new_api">new-api</option>
                <option value="sub2api">sub2api</option>
              </select>
              <select v-model="filters.site_kind" class="input h-9 py-1 text-sm" @change="resetPagination">
                <option value="">全部站点</option>
                <option value="api_relay">API 中转</option>
                <option value="official">官方平台</option>
                <option value="tool">工具</option>
                <option value="client">客户端</option>
                <option value="benchmark">评测</option>
                <option value="other">其他</option>
              </select>
            </div>
          </div>
        </div>

        <div class="overflow-x-auto">
          <table class="w-full min-w-[1180px] divide-y divide-gray-200 dark:divide-dark-700">
            <thead class="bg-gray-50 dark:bg-dark-800">
              <tr>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">站点</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">状态</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">分类</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">标签</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">链接</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">更新时间</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">操作</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-900">
              <tr v-if="sites.length === 0">
                <td colspan="7" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-dark-400">暂无目录站点</td>
              </tr>
              <tr v-for="site in sites" :key="site.id">
                <td class="px-4 py-4">
                  <div class="max-w-[360px] truncate text-sm font-medium text-gray-900 dark:text-white">{{ site.name }}</div>
                  <div class="mt-1 truncate font-mono text-xs text-gray-500 dark:text-dark-400">{{ site.canonical_host || site.slug }}</div>
                  <div v-if="site.summary" class="mt-1 max-w-[420px] truncate text-xs text-gray-400">{{ site.summary }}</div>
                </td>
                <td class="px-4 py-4">
                  <div class="flex flex-wrap gap-2">
                    <span class="badge" :class="statusClass(site.status)">{{ statusLabel(site.status) }}</span>
                    <span class="badge" :class="visibilityClass(site.visibility)">{{ visibilityLabel(site.visibility) }}</span>
                    <span class="badge" :class="providerClass(site.provider_type)">{{ providerLabel(site.provider_type) }}</span>
                    <span class="badge" :class="qualityClass(site.quality_status)">{{ qualityLabel(site.quality_status) }}</span>
                  </div>
                </td>
                <td class="px-4 py-4">
                  <div class="flex max-w-[220px] flex-wrap gap-1">
                    <span v-for="category in site.categories || []" :key="category.id" class="badge badge-gray">{{ category.name }}</span>
                    <span v-if="!site.categories || site.categories.length === 0" class="text-sm text-gray-400">-</span>
                  </div>
                </td>
                <td class="px-4 py-4">
                  <div class="flex max-w-[260px] flex-wrap gap-1">
                    <span v-for="tag in site.tags || []" :key="tag.id" class="badge badge-primary">{{ tag.name }}</span>
                    <span v-if="!site.tags || site.tags.length === 0" class="text-sm text-gray-400">-</span>
                  </div>
                </td>
                <td class="px-4 py-4">
                  <div class="flex max-w-[280px] flex-wrap gap-2">
                    <a
                      v-for="link in visibleLinks(site)"
                      :key="link.id"
                      :href="link.url"
                      target="_blank"
                      rel="noreferrer"
                      class="btn btn-secondary btn-sm"
                    >
                      {{ link.label || linkTypeLabel(link.link_type) }}
                    </a>
                    <span v-if="visibleLinks(site).length === 0" class="text-sm text-gray-400">-</span>
                  </div>
                </td>
                <td class="px-4 py-4 text-right text-xs text-gray-500 dark:text-dark-400">{{ formatTime(site.updated_at) }}</td>
                <td class="px-4 py-4 text-right">
                  <div class="flex items-center justify-end gap-2">
                    <button type="button" class="btn btn-danger btn-sm" :disabled="loading || deletingSiteID === site.id" @click="deleteSite(site)">
                      <Icon name="trash" size="sm" />
                      {{ deletingSiteID === site.id ? '删除中...' : '删除' }}
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import Pagination from '@/components/common/Pagination.vue'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { useAppStore } from '@/stores/app'
import {
  bulkPublishSiteCatalogSites,
  deleteSiteCatalogSite,
  listSiteCatalogSites,
  type SiteCatalogKind,
  type SiteCatalogLink,
  type SiteCatalogQualityStatus,
  type SiteCatalogSite,
  type SiteCatalogStatus,
  type SiteCatalogVisibility
} from '@/api/admin/adminPlus'

type PaginationState = {
  page: number
  page_size: number
  total: number
  pages: number
}

const appStore = useAppStore()
const loading = ref(false)
const publishing = ref(false)
const deletingSiteID = ref<number | null>(null)
const sites = ref<SiteCatalogSite[]>([])
const filters = reactive({
  q: '',
  status: '' as SiteCatalogStatus | '',
  site_kind: '' as SiteCatalogKind | '',
  provider_type: '' as 'new_api' | 'sub2api' | ''
})
const pagination = reactive<PaginationState>({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0,
  pages: 0
})

const publishButtonLabel = computed(() => {
  if (publishing.value) return '公开中...'
  if (pagination.total === 0) return '暂无可公开站点'
  return `一键公开全部 ${pagination.total}`
})

onMounted(() => {
  void loadSites()
})

async function loadSites() {
  loading.value = true
  try {
    const result = await listSiteCatalogSites({
      page: pagination.page,
      page_size: pagination.page_size,
      q: filters.q || undefined,
      status: filters.status,
      site_kind: filters.site_kind,
      provider_type: filters.provider_type
    })
    sites.value = result.items
    pagination.total = result.total
    pagination.page = result.page || pagination.page
    pagination.page_size = result.page_size || pagination.page_size
    pagination.pages = result.pages || 0
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    loading.value = false
  }
}

function resetPagination() {
  pagination.page = 1
  void loadSites()
}

function handlePageChange(page: number) {
  pagination.page = page
  void loadSites()
}

function handlePageSizeChange(pageSize: number) {
  pagination.page_size = pageSize
  pagination.page = 1
  void loadSites()
}

async function publishAllMatchingSites() {
  if (pagination.total === 0 || publishing.value) return
  const confirmed = window.confirm(`确认将当前筛选下的 ${pagination.total} 个站点全部转为公开并发布？`)
  if (!confirmed) return
  publishing.value = true
  try {
    const result = await bulkPublishSiteCatalogSites({
      q: filters.q || undefined,
      status: filters.status,
      site_kind: filters.site_kind,
      provider_type: filters.provider_type
    })
    appStore.showSuccess(result.updated > 0 ? `已公开 ${result.updated} 个站点` : '当前筛选下没有需要公开的站点')
    await loadSites()
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    publishing.value = false
  }
}

async function deleteSite(site: SiteCatalogSite) {
  if (deletingSiteID.value === site.id) return
  const confirmed = window.confirm(`确认永久删除站点「${site.name}」？关联链接、分类、标签和采集关联会一并清理。`)
  if (!confirmed) return
  deletingSiteID.value = site.id
  try {
    await deleteSiteCatalogSite(site.id)
    appStore.showSuccess('站点已删除')
    await loadSites()
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    deletingSiteID.value = null
  }
}

function statusCount(status: SiteCatalogStatus): number {
  return sites.value.filter((site) => site.status === status).length
}

function providerCount(providerType: 'new_api' | 'sub2api'): number {
  return sites.value.filter((site) => site.provider_type === providerType).length
}

function visibleLinks(site: SiteCatalogSite): SiteCatalogLink[] {
  return (site.links || []).slice(0, 3)
}

function statusLabel(status: SiteCatalogStatus): string {
  return { draft: '草稿', reviewing: '待审核', published: '已发布', archived: '已归档' }[status] || status
}

function statusClass(status: SiteCatalogStatus): string {
  if (status === 'published') return 'badge-success'
  if (status === 'reviewing') return 'badge-warning'
  if (status === 'archived') return 'badge-gray'
  return 'badge-primary'
}

function visibilityLabel(value: SiteCatalogVisibility): string {
  return { public: '公开', private: '私有' }[value] || value
}

function visibilityClass(value: SiteCatalogVisibility): string {
  if (value === 'public') return 'badge-success'
  return 'badge-gray'
}

function providerLabel(value?: string): string {
  if (value === 'new_api') return 'new-api'
  if (value === 'sub2api') return 'sub2api'
  return '未知'
}

function providerClass(value?: string): string {
  if (value === 'new_api') return 'badge-primary'
  if (value === 'sub2api') return 'badge-purple'
  return 'badge-gray'
}

function qualityLabel(value: SiteCatalogQualityStatus): string {
  return { complete: '完整', needs_review: '待完善', link_broken: '链接异常', duplicate: '重复' }[value] || value
}

function qualityClass(value: SiteCatalogQualityStatus): string {
  if (value === 'complete') return 'badge-success'
  if (value === 'link_broken' || value === 'duplicate') return 'badge-danger'
  return 'badge-warning'
}

function linkTypeLabel(value: SiteCatalogLink['link_type']): string {
  return {
    homepage: '主页',
    register: '注册',
    dashboard: '控制台',
    api_base: 'API',
    recharge: '充值',
    docs: '文档',
    contact: '联系'
  }[value] || value
}

function formatTime(value?: string): string {
  if (!value) return '-'
  return new Date(value).toLocaleString()
}

function errorMessage(error: unknown): string {
  if (error instanceof Error) return error.message
  if (error && typeof error === 'object') {
    const data = error as { message?: unknown; reason?: unknown }
    const message = typeof data.message === 'string' ? data.message.trim() : ''
    const reason = typeof data.reason === 'string' ? data.reason.trim() : ''
    if (message && reason) return `${message}（${reason}）`
    return message || reason || '操作失败'
  }
  return '操作失败'
}
</script>
