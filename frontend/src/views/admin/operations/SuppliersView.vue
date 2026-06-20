<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">供应商管理</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
            维护上游供应商身份、类型、切换资格、凭据状态和初始余额。
          </p>
        </div>
        <button type="button" class="btn btn-secondary" :disabled="loading" @click="loadSuppliers">
          <Icon name="refresh" size="sm" />
          刷新
        </button>
      </section>

      <section class="grid gap-4 md:grid-cols-4">
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">供应商总数</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ suppliers.length }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">可切换</p>
          <p class="mt-2 text-2xl font-semibold text-emerald-600 dark:text-emerald-400">{{ switchableCount }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">仅监控</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ monitorOnlyCount }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">异常健康</p>
          <p class="mt-2 text-2xl font-semibold text-rose-600 dark:text-rose-400">{{ unhealthyCount }}</p>
        </div>
      </section>

      <section class="grid gap-6 xl:grid-cols-[minmax(0,1fr)_420px]">
        <div class="card overflow-hidden">
          <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">供应商列表</h2>
          </div>

          <div v-if="loading" class="flex justify-center py-10">
            <LoadingSpinner />
          </div>
          <div v-else class="overflow-x-auto">
            <table class="w-full min-w-[980px] divide-y divide-gray-200 dark:divide-dark-700">
              <thead class="bg-gray-50 dark:bg-dark-800">
                <tr>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">名称</th>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">类型</th>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">状态</th>
                  <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">余额</th>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">凭据</th>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">地址</th>
                  <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">操作</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-900">
                <tr v-if="suppliers.length === 0">
                  <td colspan="7" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-dark-400">暂无供应商</td>
                </tr>
                <tr v-for="supplier in suppliers" :key="supplier.id" class="hover:bg-gray-50 dark:hover:bg-dark-800">
                  <td class="px-4 py-4">
                    <div class="font-medium text-gray-900 dark:text-white">{{ supplier.name }}</div>
                    <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">#{{ supplier.id }} · {{ formatDateTime(supplier.created_at) }}</div>
                  </td>
                  <td class="px-4 py-4">
                    <div class="flex flex-wrap gap-1">
                      <span class="badge badge-gray">{{ supplier.kind }}</span>
                      <span class="badge badge-primary">{{ supplier.type }}</span>
                    </div>
                  </td>
                  <td class="px-4 py-4">
                    <div class="flex flex-col gap-1">
                      <span class="badge w-fit" :class="runtimeClass(supplier.runtime_status)">{{ supplier.runtime_status }}</span>
                      <span class="badge w-fit" :class="healthClass(supplier.health_status)">{{ supplier.health_status }}</span>
                    </div>
                  </td>
                  <td class="px-4 py-4 text-right text-sm text-gray-900 dark:text-gray-100">
                    {{ formatMoney(supplier.balance_cents, supplier.balance_currency) }}
                    <div class="text-xs text-gray-500 dark:text-dark-400">{{ formatDateTime(supplier.balance_updated_at) }}</div>
                  </td>
                  <td class="px-4 py-4">
                    <div class="flex flex-wrap gap-1">
                      <span v-if="supplier.credential.admin_api_key_configured" class="badge badge-success">API</span>
                      <span v-if="supplier.credential.postgres_configured" class="badge badge-purple">PG</span>
                      <span v-if="supplier.credential.redis_configured" class="badge badge-primary">Redis</span>
                      <span v-if="supplier.credential.browser_login_enabled" class="badge badge-warning">Chrome</span>
                      <span v-if="!hasCredential(supplier)" class="badge badge-gray">无</span>
                    </div>
                  </td>
                  <td class="max-w-[260px] px-4 py-4 text-xs text-gray-500 dark:text-dark-400">
                    <div class="truncate">{{ supplier.dashboard_url || '-' }}</div>
                    <div class="truncate">{{ supplier.api_base_url || '-' }}</div>
                  </td>
                  <td class="px-4 py-4 text-right">
                    <button type="button" class="btn btn-secondary px-3 py-1.5 text-xs" @click="prepareStatusEdit(supplier)">
                      改状态
                    </button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>

        <div class="space-y-6">
          <form class="card p-5" @submit.prevent="submitSupplier">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">新增供应商</h2>
            <div class="mt-5 space-y-4">
              <label class="block">
                <span class="input-label">名称</span>
                <input v-model.trim="form.name" class="input" required placeholder="supplier-a" />
              </label>

              <div class="grid gap-4 sm:grid-cols-2">
                <label class="block">
                  <span class="input-label">供应商归类</span>
                  <select v-model="form.kind" class="input">
                    <option value="relay">下游中转</option>
                    <option value="source_account">源站账号</option>
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
                    <option value="custom">自定义</option>
                  </select>
                </label>
              </div>

              <div class="grid gap-4 sm:grid-cols-2">
                <label class="block">
                  <span class="input-label">运行状态</span>
                  <select v-model="form.runtime_status" class="input">
                    <option value="monitor_only">仅监控</option>
                    <option value="candidate">候选</option>
                    <option value="active">当前使用</option>
                    <option value="disabled">停用</option>
                  </select>
                </label>
                <label class="block">
                  <span class="input-label">健康状态</span>
                  <select v-model="form.health_status" class="input">
                    <option value="normal">正常</option>
                    <option value="unavailable">不可用</option>
                    <option value="credential_invalid">凭据失效</option>
                    <option value="paused">暂停</option>
                  </select>
                </label>
              </div>

              <label class="block">
                <span class="input-label">后台地址</span>
                <input v-model.trim="form.dashboard_url" class="input" placeholder="https://supplier.example.com" />
              </label>

              <label class="block">
                <span class="input-label">API Base URL</span>
                <input v-model.trim="form.api_base_url" class="input" placeholder="https://supplier.example.com/api/v1" />
              </label>

              <label class="block">
                <span class="input-label">Admin API Key</span>
                <input v-model.trim="form.admin_api_key" class="input" autocomplete="off" />
              </label>

              <div class="grid gap-4 sm:grid-cols-2">
                <label class="block">
                  <span class="input-label">初始余额</span>
                  <input v-model.number="form.balance_yuan" type="number" min="0" step="0.01" class="input" />
                </label>
                <label class="block">
                  <span class="input-label">币种</span>
                  <input v-model.trim="form.balance_currency" class="input" placeholder="CNY" />
                </label>
              </div>

              <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
                <input v-model="form.browser_login_enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
                允许 Chrome 插件登录采集
              </label>

              <button type="submit" class="btn btn-primary w-full" :disabled="submitting">
                {{ submitting ? '创建中...' : '创建供应商' }}
              </button>
            </div>
          </form>

          <form v-if="statusForm.id" class="card p-5" @submit.prevent="submitStatus">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">状态调整</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ statusForm.name }}</p>
            <div class="mt-5 space-y-4">
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
              <button type="submit" class="btn btn-primary w-full" :disabled="statusSubmitting">
                保存状态
              </button>
            </div>
          </form>
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import {
  createSupplier,
  listSuppliers,
  updateSupplierStatus,
  type Supplier,
  type SupplierHealthStatus,
  type SupplierKind,
  type SupplierRuntimeStatus,
  type SupplierType
} from '@/api/admin/adminPlus'

const appStore = useAppStore()

const loading = ref(false)
const submitting = ref(false)
const statusSubmitting = ref(false)
const suppliers = ref<Supplier[]>([])

const form = reactive({
  name: '',
  kind: 'relay' as SupplierKind,
  type: 'sub2api' as SupplierType,
  runtime_status: 'monitor_only' as SupplierRuntimeStatus,
  health_status: 'normal' as SupplierHealthStatus,
  dashboard_url: '',
  api_base_url: '',
  admin_api_key: '',
  balance_yuan: 0,
  balance_currency: 'CNY',
  browser_login_enabled: true
})

const statusForm = reactive({
  id: 0,
  name: '',
  runtime_status: 'monitor_only' as SupplierRuntimeStatus,
  health_status: 'normal' as SupplierHealthStatus
})

const switchableCount = computed(() => suppliers.value.filter((item) =>
  ['candidate', 'active'].includes(item.runtime_status) && item.health_status === 'normal' && item.balance_cents > 0
).length)
const monitorOnlyCount = computed(() => suppliers.value.filter((item) => item.runtime_status === 'monitor_only').length)
const unhealthyCount = computed(() => suppliers.value.filter((item) => item.health_status !== 'normal').length)

function centsFromYuan(value: number): number {
  return Math.round(Number(value || 0) * 100)
}

function formatMoney(cents: number, currency: string): string {
  return new Intl.NumberFormat(undefined, {
    style: 'currency',
    currency: currency || 'CNY',
    minimumFractionDigits: 2
  }).format((cents || 0) / 100)
}

function formatDateTime(value?: string | null): string {
  if (!value) return '-'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '-' : date.toLocaleString()
}

function runtimeClass(status: SupplierRuntimeStatus): string {
  if (status === 'active') return 'badge-success'
  if (status === 'candidate') return 'badge-primary'
  if (status === 'disabled') return 'badge-danger'
  return 'badge-gray'
}

function healthClass(status: SupplierHealthStatus): string {
  if (status === 'normal') return 'badge-success'
  if (status === 'paused') return 'badge-warning'
  return 'badge-danger'
}

function hasCredential(supplier: Supplier): boolean {
  return supplier.credential.admin_api_key_configured ||
    supplier.credential.postgres_configured ||
    supplier.credential.redis_configured ||
    supplier.credential.browser_login_enabled
}

async function loadSuppliers() {
  loading.value = true
  try {
    const result = await listSuppliers()
    suppliers.value = result.items
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '加载供应商失败')
  } finally {
    loading.value = false
  }
}

async function submitSupplier() {
  submitting.value = true
  try {
    await createSupplier({
      name: form.name,
      kind: form.kind,
      type: form.type,
      runtime_status: form.runtime_status,
      health_status: form.health_status,
      dashboard_url: form.dashboard_url || undefined,
      api_base_url: form.api_base_url || undefined,
      admin_api_key: form.admin_api_key || undefined,
      balance_cents: centsFromYuan(form.balance_yuan),
      balance_currency: form.balance_currency || 'CNY',
      browser_login_enabled: form.browser_login_enabled
    })
    form.name = ''
    form.admin_api_key = ''
    appStore.showSuccess('供应商已创建')
    await loadSuppliers()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '创建供应商失败')
  } finally {
    submitting.value = false
  }
}

function prepareStatusEdit(supplier: Supplier) {
  statusForm.id = supplier.id
  statusForm.name = supplier.name
  statusForm.runtime_status = supplier.runtime_status
  statusForm.health_status = supplier.health_status
}

async function submitStatus() {
  if (!statusForm.id) return
  statusSubmitting.value = true
  try {
    await updateSupplierStatus(statusForm.id, {
      runtime_status: statusForm.runtime_status,
      health_status: statusForm.health_status
    })
    appStore.showSuccess('状态已更新')
    await loadSuppliers()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '更新状态失败')
  } finally {
    statusSubmitting.value = false
  }
}

onMounted(loadSuppliers)
</script>
