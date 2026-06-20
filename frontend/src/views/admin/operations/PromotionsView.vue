<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">优惠监控</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
            关注供应商充值赠送、折扣和限时活动；无余额供应商只能提示充值，不能参与切换。
          </p>
        </div>
        <button type="button" class="btn btn-secondary" :disabled="loading" @click="loadPage">
          <Icon name="refresh" size="sm" />
          刷新
        </button>
      </section>

      <section class="grid gap-4 md:grid-cols-4">
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">优惠事件</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ events.length }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">可切换优惠</p>
          <p class="mt-2 text-2xl font-semibold text-emerald-600 dark:text-emerald-400">{{ switchCandidateCount }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">需充值解锁</p>
          <p class="mt-2 text-2xl font-semibold text-amber-600 dark:text-amber-400">{{ rechargeUnlockCount }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">待处理</p>
          <p class="mt-2 text-2xl font-semibold text-rose-600 dark:text-rose-400">{{ openCount }}</p>
        </div>
      </section>

      <section class="grid gap-6 xl:grid-cols-[420px_minmax(0,1fr)]">
        <form class="card p-5" @submit.prevent="submitPromotion">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">录入优惠</h2>
          <div class="mt-5 space-y-4">
            <label class="block">
              <span class="input-label">供应商</span>
              <select v-model.number="form.supplier_id" class="input" required>
                <option :value="0" disabled>请选择</option>
                <option v-for="supplier in suppliers" :key="supplier.id" :value="supplier.id">{{ supplier.name }}</option>
              </select>
            </label>
            <label class="block">
              <span class="input-label">优惠类型</span>
              <select v-model="form.type" class="input">
                <option value="recharge_bonus">充值赠送</option>
                <option value="rate_discount">费率折扣</option>
                <option value="package_deal">套餐</option>
                <option value="limited_offer">限时活动</option>
                <option value="other">其他</option>
              </select>
            </label>
            <label class="block">
              <span class="input-label">标题</span>
              <input v-model.trim="form.title" class="input" required />
            </label>
            <label class="block">
              <span class="input-label">说明</span>
              <textarea v-model.trim="form.description" rows="3" class="input resize-none" />
            </label>
            <div class="grid gap-4 sm:grid-cols-2">
              <label class="block">
                <span class="input-label">最低充值</span>
                <input v-model.number="form.min_recharge_yuan" type="number" min="0" step="0.01" class="input" />
              </label>
              <label class="block">
                <span class="input-label">赠送比例 %</span>
                <input v-model.number="form.bonus_percent" type="number" min="0" step="0.01" class="input" />
              </label>
            </div>
            <div class="grid gap-4 sm:grid-cols-2">
              <label class="block">
                <span class="input-label">折扣比例 %</span>
                <input v-model.number="form.discount_percent" type="number" min="0" step="0.01" class="input" />
              </label>
              <label class="block">
                <span class="input-label">币种</span>
                <input v-model.trim="form.currency" class="input" placeholder="CNY" />
              </label>
            </div>
            <button type="submit" class="btn btn-primary w-full" :disabled="submitting">
              {{ submitting ? '记录中...' : '记录优惠' }}
            </button>
          </div>
        </form>

        <div class="card overflow-hidden">
          <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">优惠事件</h2>
          </div>
          <div class="divide-y divide-gray-100 dark:divide-dark-700">
            <div v-if="events.length === 0" class="px-5 py-10 text-center text-sm text-gray-500 dark:text-dark-400">暂无优惠事件</div>
            <div v-for="event in events" :key="event.id" class="p-5">
              <div class="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
                <div>
                  <div class="flex flex-wrap items-center gap-2">
                    <span class="badge badge-gray">{{ event.type }}</span>
                    <span class="badge" :class="recommendationClass(event.recommendation)">{{ recommendationText(event.recommendation) }}</span>
                    <span class="badge" :class="event.status === 'open' ? 'badge-warning' : 'badge-success'">{{ event.status }}</span>
                  </div>
                  <h3 class="mt-3 font-medium text-gray-900 dark:text-white">{{ event.title }}</h3>
                  <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ event.description || '-' }}</p>
                  <p class="mt-2 text-xs text-gray-500 dark:text-dark-400">
                    {{ supplierName(event.supplier_id) }} · 余额 {{ formatMoney(event.balance_cents, event.currency) }} · 最低充值 {{ formatMoney(event.min_recharge_cents, event.currency) }}
                  </p>
                </div>
                <button v-if="event.status === 'open'" type="button" class="btn btn-secondary px-3 py-1.5 text-xs" @click="ackEvent(event.id)">
                  确认
                </button>
              </div>
            </div>
          </div>
          <Pagination
            v-if="eventPagination.total > 0"
            :page="eventPagination.page"
            :total="eventPagination.total"
            :page-size="eventPagination.page_size"
            @update:page="handleEventPageChange"
            @update:pageSize="handleEventPageSizeChange"
          />
        </div>
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
  acknowledgePromotionEvent,
  listPromotionEvents,
  listSuppliers,
  recordPromotion as recordPromotionAPI,
  type PromotionEvent,
  type Supplier
} from '@/api/admin/adminPlus'

const appStore = useAppStore()

const loading = ref(false)
const submitting = ref(false)
const suppliers = ref<Supplier[]>([])
const events = ref<PromotionEvent[]>([])
const eventPagination = reactive({ page: 1, page_size: getPersistedPageSize(), total: 0, pages: 0 })

const form = reactive({
  supplier_id: 0,
  type: 'recharge_bonus' as PromotionEvent['type'],
  title: '',
  description: '',
  min_recharge_yuan: 0,
  bonus_percent: 0,
  discount_percent: 0,
  currency: 'CNY'
})

const switchCandidateCount = computed(() => events.value.filter((event) => event.recommendation === 'switch_candidate').length)
const rechargeUnlockCount = computed(() => events.value.filter((event) => event.recommendation === 'recharge_to_unlock').length)
const openCount = computed(() => events.value.filter((event) => event.status === 'open').length)

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

function supplierName(id: number): string {
  return suppliers.value.find((supplier) => supplier.id === id)?.name || `#${id}`
}

function recommendationText(recommendation: PromotionEvent['recommendation']): string {
  if (recommendation === 'switch_candidate') return '可作为切换候选'
  if (recommendation === 'recharge_to_unlock') return '充值可解锁'
  if (recommendation === 'monitor_only') return '仅监控'
  return '信息'
}

function recommendationClass(recommendation: PromotionEvent['recommendation']): string {
  if (recommendation === 'switch_candidate') return 'badge-success'
  if (recommendation === 'recharge_to_unlock') return 'badge-warning'
  return 'badge-gray'
}

async function loadPage() {
  loading.value = true
  try {
    const [supplierResult, eventResult] = await Promise.all([
      listSuppliers(),
      listPromotionEvents({ page: eventPagination.page, page_size: eventPagination.page_size })
    ])
    suppliers.value = supplierResult.items
    events.value = eventResult.items
    eventPagination.total = eventResult.total || 0
    eventPagination.pages = eventResult.pages || 0
    eventPagination.page = eventResult.page || eventPagination.page
    eventPagination.page_size = eventResult.page_size || eventPagination.page_size
    if (!form.supplier_id && suppliers.value[0]) {
      form.supplier_id = suppliers.value[0].id
    }
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '加载优惠数据失败')
  } finally {
    loading.value = false
  }
}

function handleEventPageChange(page: number) {
  eventPagination.page = page
  void loadPage()
}

function handleEventPageSizeChange(pageSize: number) {
  eventPagination.page_size = pageSize
  eventPagination.page = 1
  void loadPage()
}

async function submitPromotion() {
  submitting.value = true
  try {
    const supplier = suppliers.value.find((item) => item.id === form.supplier_id)
    await recordPromotionAPI({
      supplier_id: form.supplier_id,
      source: 'manual',
      type: form.type,
      title: form.title,
      description: form.description || undefined,
      currency: form.currency || supplier?.balance_currency || 'CNY',
      min_recharge_cents: centsFromYuan(form.min_recharge_yuan),
      bonus_percent: form.bonus_percent || null,
      discount_percent: form.discount_percent || null,
      runtime_status: supplier?.runtime_status,
      balance_cents: supplier?.balance_cents || 0
    })
    form.title = ''
    form.description = ''
    appStore.showSuccess('优惠已记录')
    await loadPage()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '记录优惠失败')
  } finally {
    submitting.value = false
  }
}

async function ackEvent(id: number) {
  try {
    await acknowledgePromotionEvent(id)
    appStore.showSuccess('事件已确认')
    await loadPage()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '确认事件失败')
  }
}

onMounted(loadPage)
</script>
