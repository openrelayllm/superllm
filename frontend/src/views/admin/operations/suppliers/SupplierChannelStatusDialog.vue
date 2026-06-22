<template>
<BaseDialog :show="channelStatusDialogOpen" :title="channelStatusSupplier ? `供应商渠道状态 - ${channelStatusSupplier.name}` : '供应商渠道状态'" width="full" @close="closeChannelStatusDialog">
  <div class="min-h-[560px] max-w-full overflow-hidden rounded-b-2xl bg-gradient-to-br from-emerald-50/60 via-white to-sky-50/40 px-5 py-5 dark:from-dark-900 dark:via-dark-900 dark:to-dark-800 sm:px-6">
    <section class="pb-5">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div class="min-w-0 text-xs text-gray-500 dark:text-dark-400">
          <div class="truncate" :title="channelMonitorAPIBaseURL || channelMonitorOrigin || '-'">
            {{ channelMonitorAPIBaseURL || channelMonitorOrigin || '供应商 API 未读取' }}
          </div>
          <div class="mt-1">更新于 {{ formatDateTime(channelMonitorCapturedAt) }}</div>
        </div>

        <div class="flex min-w-0 flex-wrap items-center justify-end gap-3">
          <div role="tablist" class="inline-flex rounded-xl border border-gray-200/60 bg-gray-100 p-0.5 text-xs dark:border-dark-700/60 dark:bg-dark-800">
            <button
              v-for="option in channelStatusWindowOptions"
              :key="option.value"
              type="button"
              role="tab"
              :aria-selected="channelStatusWindow === option.value"
              class="rounded-lg px-3 py-1 transition-colors disabled:cursor-not-allowed disabled:opacity-45"
              :class="channelStatusWindow === option.value ? 'bg-white font-semibold text-gray-900 shadow-sm dark:bg-dark-700 dark:text-white' : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200'"
              :disabled="option.disabled"
              :title="option.disabled ? '供应商明细接口接入后启用' : option.label"
              @click="channelStatusWindow = option.value"
            >
              {{ option.label }}
            </button>
          </div>

          <span class="inline-flex items-center rounded-full px-2.5 py-1 text-xs font-semibold uppercase tracking-wider" :class="channelStatusOverallChipClass">
            <span class="mr-1.5 h-1.5 w-1.5 rounded-full animate-pulse" :class="channelStatusOverallDotClass"></span>
            {{ channelStatusOverallLabel }}
          </span>

          <button
            type="button"
            class="flex h-8 w-8 items-center justify-center rounded-lg text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-700 disabled:opacity-50 dark:text-gray-400 dark:hover:bg-dark-700 dark:hover:text-gray-200"
            :disabled="channelStatusLoading || !channelStatusSupplier"
            title="刷新"
            @click="loadChannelStatus"
          >
            <Icon name="refresh" size="md" :class="{ 'animate-spin': channelStatusLoading }" />
          </button>

          <button
            type="button"
            class="inline-flex h-8 items-center gap-1.5 rounded-lg border border-gray-200 bg-white px-2.5 text-xs font-medium text-gray-600 shadow-sm transition-colors hover:bg-gray-50 dark:border-dark-700 dark:bg-dark-800 dark:text-gray-300 dark:hover:bg-dark-700"
            :class="{ 'opacity-60': !channelStatusAutoRefresh }"
            @click="toggleChannelStatusAutoRefresh"
          >
            <Icon name="refresh" size="xs" />
            自动刷新: {{ channelStatusCountdown }}s
          </button>
        </div>
      </div>
    </section>

    <div v-if="channelStatusError" class="mb-5 rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-200">
      {{ channelStatusError }}
    </div>

    <div v-if="channelStatusLoading && channelMonitorItems.length === 0" class="grid gap-5 [grid-template-columns:repeat(auto-fit,minmax(min(100%,18rem),1fr))]">
      <div
        v-for="index in 6"
        :key="index"
        class="min-h-[280px] animate-pulse rounded-2xl border border-gray-200/80 bg-white/70 p-5 dark:border-dark-700/70 dark:bg-dark-800/60"
      >
        <div class="flex items-start gap-3">
          <div class="h-9 w-9 rounded-xl bg-gray-200 dark:bg-dark-700"></div>
          <div class="flex-1 space-y-2">
            <div class="h-4 w-2/3 rounded bg-gray-200 dark:bg-dark-700"></div>
            <div class="h-3 w-1/2 rounded bg-gray-200 dark:bg-dark-700"></div>
          </div>
          <div class="h-6 w-16 rounded-full bg-gray-200 dark:bg-dark-700"></div>
        </div>
        <div class="mt-5 grid grid-cols-2 gap-2">
          <div class="h-16 rounded-xl bg-gray-100 dark:bg-dark-900/40"></div>
          <div class="h-16 rounded-xl bg-gray-100 dark:bg-dark-900/40"></div>
        </div>
        <div class="mt-6 h-5 w-full rounded bg-gray-100 dark:bg-dark-900/40"></div>
      </div>
    </div>

    <EmptyState
      v-else-if="channelMonitorItems.length === 0 && !channelStatusError"
      title="暂无渠道状态"
      description="请确认供应商已启用渠道监控；Sub2API 需要 /api/v1/channel-monitors，New API 需要可访问的 Pulse 监控入口。"
    />

    <div v-else class="grid gap-5 [grid-template-columns:repeat(auto-fit,minmax(min(100%,18rem),1fr))]">
      <SupplierChannelMonitorCard
        v-for="item in channelMonitorItems"
        :key="item.id"
        :item="item"
        :window="channelStatusWindow"
        :countdown-seconds="channelStatusCountdown"
      />
    </div>
  </div>

  <template #footer>
    <button type="button" class="btn btn-secondary" @click="closeChannelStatusDialog">关闭</button>
  </template>
</BaseDialog>


</template>

<script setup lang="ts">
import BaseDialog from '@/components/common/BaseDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Icon from '@/components/icons/Icon.vue'
import SupplierChannelMonitorCard from '@/components/admin-plus/SupplierChannelMonitorCard.vue'
const props = defineProps<{ vm: any }>()
const {
  channelStatusDialogOpen,
  channelStatusSupplier,
  channelMonitorItems,
  channelStatusLoading,
  channelStatusError,
  channelMonitorCapturedAt,
  channelMonitorOrigin,
  channelMonitorAPIBaseURL,
  channelStatusWindow,
  channelStatusAutoRefresh,
  channelStatusCountdown,
  channelStatusWindowOptions,
  channelStatusOverallLabel,
  channelStatusOverallChipClass,
  channelStatusOverallDotClass,
  formatDateTime,
  loadChannelStatus,
  closeChannelStatusDialog,
  toggleChannelStatusAutoRefresh
} = props.vm
</script>
