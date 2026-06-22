<template>
  <article
    class="flex min-h-[280px] min-w-0 w-full flex-col overflow-hidden rounded-2xl border border-gray-200/80 bg-white/70 p-5 text-left shadow-card backdrop-blur-xl transition-all duration-300 ease-out dark:border-dark-700/70 dark:bg-dark-800/60"
  >
    <div class="flex min-w-0 items-start gap-3">
      <span
        class="grid h-9 w-9 flex-shrink-0 place-items-center rounded-xl ring-1 ring-black/5 dark:ring-white/10"
        :class="[providerGradientClass, providerTintClass]"
      >
        <svg
          v-if="providerIconPaths.length > 0"
          width="20"
          height="20"
          viewBox="0 0 24 24"
          xmlns="http://www.w3.org/2000/svg"
          fill="currentColor"
          fill-rule="evenodd"
          aria-hidden="true"
        >
          <path v-for="(path, index) in providerIconPaths" :key="index" :d="path" />
        </svg>
        <span v-else class="text-xs font-bold">{{ providerFallback }}</span>
      </span>
      <div class="min-w-0 flex-1">
        <div class="truncate text-base font-semibold text-gray-900 dark:text-gray-100">
          {{ item.name || '-' }}
        </div>
        <div class="mt-0.5 flex min-w-0 items-center gap-1.5 overflow-hidden">
          <span class="inline-flex flex-shrink-0 items-center rounded-md px-1.5 py-0.5 text-[10px] font-medium" :class="providerBadgeClass">
            {{ providerLabel }}
          </span>
          <span class="truncate font-mono text-xs text-gray-500 dark:text-gray-400">
            {{ item.primary_model || '-' }}
          </span>
          <span
            v-if="item.group_name"
            class="inline-flex max-w-[9rem] flex-shrink items-center truncate rounded-md bg-gray-100 px-1.5 py-0.5 text-[10px] font-medium text-gray-600 dark:bg-dark-700 dark:text-gray-300"
            :title="item.group_name"
          >
            {{ item.group_name }}
          </span>
        </div>
      </div>
      <span class="flex-shrink-0 rounded-full px-2.5 py-1 text-xs font-semibold" :class="statusBadgeClass(item.primary_status)">
        {{ statusLabel(item.primary_status) }}
      </span>
    </div>

    <div class="mt-5 grid grid-cols-2 gap-2">
      <div class="rounded-xl border border-gray-100 bg-gray-50/80 p-3 dark:border-dark-700/50 dark:bg-dark-900/40">
        <div class="flex items-center gap-1.5 text-[10px] font-semibold uppercase tracking-wider text-gray-400">
          <Icon name="bolt" size="xs" />
          <span>{{ t('monitorCommon.dialogLatency') }}</span>
        </div>
        <div class="mt-1.5 font-mono text-lg font-bold tabular-nums text-gray-900 dark:text-gray-100">
          {{ formatLatency(item.primary_latency_ms) }}<span class="ml-0.5 text-xs font-normal text-gray-400">ms</span>
        </div>
      </div>
      <div class="rounded-xl border border-gray-100 bg-gray-50/80 p-3 dark:border-dark-700/50 dark:bg-dark-900/40">
        <div class="flex items-center gap-1.5 text-[10px] font-semibold uppercase tracking-wider text-gray-400">
          <Icon name="globe" size="xs" />
          <span>{{ t('monitorCommon.endpointPing') }}</span>
        </div>
        <div class="mt-1.5 font-mono text-lg font-bold tabular-nums text-gray-900 dark:text-gray-100">
          {{ formatLatency(item.primary_ping_latency_ms) }}<span class="ml-0.5 text-xs font-normal text-gray-400">ms</span>
        </div>
      </div>
    </div>

    <div class="mt-4 border-t border-gray-100 dark:border-dark-700/60"></div>

    <div class="mt-3 flex items-end justify-between">
      <div class="text-[11px] uppercase tracking-widest text-gray-400">
        {{ t('monitorCommon.availabilityPrefix') }} · {{ windowLabel }}
      </div>
      <div class="flex items-baseline gap-0.5">
        <span class="text-3xl font-bold leading-none tabular-nums" :style="availabilityColorStyle">
          {{ availabilityDisplay }}
        </span>
        <span class="text-base font-semibold leading-none" :style="availabilityColorStyle">%</span>
      </div>
    </div>
    <div v-if="item.extra_models?.length" class="mt-1 text-right text-[11px] text-gray-400">
      {{ t('monitorCommon.extraModelsCount', { n: item.extra_models.length }) }}
    </div>

    <div class="mt-4 min-w-0 border-t border-gray-100 pt-3 dark:border-dark-700/60">
      <div class="mb-2 flex min-w-0 justify-between gap-2 text-[10px] font-semibold uppercase tracking-widest text-gray-400">
        <span class="min-w-0 truncate">{{ t('monitorCommon.history60pts', { n: 60 }) }}</span>
        <span class="tabular-nums">{{ t('monitorCommon.nextUpdateIn', { n: countdownSeconds }) }}</span>
      </div>

      <div class="flex h-5 w-full min-w-0 items-end gap-[2px] overflow-hidden">
        <div
          v-for="(bar, index) in displayBars"
          :key="index"
          class="min-w-0 flex-1 rounded-sm"
          :class="bar.colorClass"
          :style="{ height: `${bar.heightPct}%` }"
          :title="bar.title"
        ></div>
      </div>

      <div class="mt-1 flex justify-between text-[9px] uppercase tracking-widest text-gray-400">
        <span>{{ t('monitorCommon.past') }}</span>
        <span>{{ t('monitorCommon.now') }}</span>
      </div>
    </div>
  </article>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { SupplierChannelMonitorView, SupplierMonitorStatus } from '@/api/admin/adminPlus'

const props = defineProps<{
  item: SupplierChannelMonitorView
  window: 'pulse' | '7d' | '15d' | '30d'
  countdownSeconds: number
}>()

const { t } = useI18n()

const PROVIDER_ICONS: Record<string, string[]> = {
  openai: [
    'M21.55 10.004a5.416 5.416 0 00-.478-4.501c-1.217-2.09-3.662-3.166-6.05-2.66A5.59 5.59 0 0010.831 1C8.39.995 6.224 2.546 5.473 4.838A5.553 5.553 0 001.76 7.496a5.487 5.487 0 00.691 6.5 5.416 5.416 0 00.477 4.502c1.217 2.09 3.662 3.165 6.05 2.66A5.586 5.586 0 0013.168 23c2.443.006 4.61-1.546 5.361-3.84a5.553 5.553 0 003.715-2.66 5.488 5.488 0 00-.693-6.497v.001zm-8.381 11.558a4.199 4.199 0 01-2.675-.954c.034-.018.093-.05.132-.074l4.44-2.53a.71.71 0 00.364-.623v-6.176l1.877 1.069c.02.01.033.029.036.05v5.115c-.003 2.274-1.87 4.118-4.174 4.123zM4.192 17.78a4.059 4.059 0 01-.498-2.763c.032.02.09.055.131.078l4.44 2.53c.225.13.504.13.73 0l5.42-3.088v2.138a.068.068 0 01-.027.057L9.9 19.288c-1.999 1.136-4.552.46-5.707-1.51h-.001zM3.023 8.216A4.15 4.15 0 015.198 6.41l-.002.151v5.06a.711.711 0 00.364.624l5.42 3.087-1.876 1.07a.067.067 0 01-.063.005l-4.489-2.559c-1.995-1.14-2.679-3.658-1.53-5.63h.001zm15.417 3.54l-5.42-3.088L14.896 7.6a.067.067 0 01.063-.006l4.489 2.557c1.998 1.14 2.683 3.662 1.529 5.633a4.163 4.163 0 01-2.174 1.807V12.38a.71.71 0 00-.363-.623zm1.867-2.773a6.04 6.04 0 00-.132-.078l-4.44-2.53a.731.731 0 00-.729 0l-5.42 3.088V7.325a.068.068 0 01.027-.057L14.1 4.713c2-1.137 4.555-.46 5.707 1.513.487.833.664 1.809.499 2.757h.001zm-11.741 3.81l-1.877-1.068a.065.065 0 01-.036-.051V6.559c.001-2.277 1.873-4.122 4.181-4.12.976 0 1.92.338 2.671.954-.034.018-.092.05-.131.073l-4.44 2.53a.71.71 0 00-.365.623l-.003 6.173v.002zm1.02-2.168L12 9.25l2.414 1.375v2.75L12 14.75l-2.415-1.375v-2.75z',
  ],
  anthropic: [
    'M4.709 15.955l4.72-2.647.08-.23-.08-.128H9.2l-.79-.048-2.698-.073-2.339-.097-2.266-.122-.571-.121L0 11.784l.055-.352.48-.321.686.06 1.52.103 2.278.158 1.652.097 2.449.255h.389l.055-.157-.134-.098-.103-.097-2.358-1.596-2.552-1.688-1.336-.972-.724-.491-.364-.462-.158-1.008.656-.722.881.06.225.061.893.686 1.908 1.476 2.491 1.833.365.304.145-.103.019-.073-.164-.274-1.355-2.446-1.446-2.49-.644-1.032-.17-.619a2.97 2.97 0 01-.104-.729L6.283.134 6.696 0l.996.134.42.364.62 1.414 1.002 2.229 1.555 3.03.456.898.243.832.091.255h.158V9.01l.128-1.706.237-2.095.23-2.695.08-.76.376-.91.747-.492.584.28.48.685-.067.444-.286 1.851-.559 2.903-.364 1.942h.212l.243-.242.985-1.306 1.652-2.064.73-.82.85-.904.547-.431h1.033l.76 1.129-.34 1.166-1.064 1.347-.881 1.142-1.264 1.7-.79 1.36.073.11.188-.02 2.856-.606 1.543-.28 1.841-.315.833.388.091.395-.328.807-1.969.486-2.309.462-3.439.813-.042.03.049.061 1.549.146.662.036h1.622l3.02.225.79.522.474.638-.079.485-1.215.62-1.64-.389-3.829-.91-1.312-.329h-.182v.11l1.093 1.068 2.006 1.81 2.509 2.33.127.578-.322.455-.34-.049-2.205-1.657-.851-.747-1.926-1.62h-.128v.17l.444.649 2.345 3.521.122 1.08-.17.353-.608.213-.668-.122-1.374-1.925-1.415-2.167-1.143-1.943-.14.08-.674 7.254-.316.37-.729.28-.607-.461-.322-.747.322-1.476.389-1.924.315-1.53.286-1.9.17-.632-.012-.042-.14.018-1.434 1.967-2.18 2.945-1.726 1.845-.414.164-.717-.37.067-.662.401-.589 2.388-3.036 1.44-1.882.93-1.086-.006-.158h-.055L4.132 18.56l-1.13.146-.487-.456.061-.746.231-.243 1.908-1.312-.006.006z',
  ],
  gemini: [
    'M20.616 10.835a14.147 14.147 0 01-4.45-3.001 14.111 14.111 0 01-3.678-6.452.503.503 0 00-.975 0 14.134 14.134 0 01-3.679 6.452 14.155 14.155 0 01-4.45 3.001c-.65.28-1.318.505-2.002.678a.502.502 0 000 .975c.684.172 1.35.397 2.002.677a14.147 14.147 0 014.45 3.001 14.112 14.112 0 013.679 6.453.502.502 0 00.975 0c.172-.685.397-1.351.677-2.003a14.145 14.145 0 013.001-4.45 14.113 14.113 0 016.453-3.678.503.503 0 000-.975 13.245 13.245 0 01-2.003-.678z',
  ],
}

const STATUS_HEIGHT: Record<string, number> = {
  operational: 100,
  degraded: 65,
  failed: 35,
  error: 35,
  empty: 15,
}

const STATUS_COLOR: Record<string, string> = {
  operational: 'bg-emerald-500',
  degraded: 'bg-amber-500',
  failed: 'bg-red-500',
  error: 'bg-red-500',
  empty: 'bg-gray-300 dark:bg-dark-600',
}

const windowLabel = computed(() => {
  if (props.window === 'pulse') return '60 秒'
  if (props.window === '15d') return t('channelStatus.windowTab.15d')
  if (props.window === '30d') return t('channelStatus.windowTab.30d')
  return t('channelStatus.windowTab.7d')
})

const providerKey = computed(() => normalizeProvider(props.item.provider))
const providerIconPaths = computed(() => PROVIDER_ICONS[providerKey.value] || [])
const providerFallback = computed(() => (props.item.provider || '?').charAt(0).toUpperCase())

const providerLabel = computed(() => {
  if (providerKey.value === 'openai') return t('monitorCommon.providers.openai')
  if (providerKey.value === 'anthropic') return t('monitorCommon.providers.anthropic')
  if (providerKey.value === 'gemini') return t('monitorCommon.providers.gemini')
  return props.item.provider || '-'
})

const providerBadgeClass = computed(() => {
  if (providerKey.value === 'openai') return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300'
  if (providerKey.value === 'anthropic') return 'bg-orange-100 text-orange-700 dark:bg-orange-500/15 dark:text-orange-300'
  if (providerKey.value === 'gemini') return 'bg-sky-100 text-sky-700 dark:bg-sky-500/15 dark:text-sky-300'
  return 'bg-gray-100 text-gray-800 dark:bg-dark-700 dark:text-gray-300'
})

const providerGradientClass = computed(() => {
  if (providerKey.value === 'openai') return 'bg-gradient-to-br from-emerald-50 to-emerald-100 dark:from-emerald-500/10 dark:to-emerald-500/20'
  if (providerKey.value === 'anthropic') return 'bg-gradient-to-br from-orange-50 to-amber-100 dark:from-orange-500/10 dark:to-amber-500/20'
  if (providerKey.value === 'gemini') return 'bg-gradient-to-br from-sky-50 to-indigo-100 dark:from-sky-500/10 dark:to-indigo-500/20'
  return 'bg-gradient-to-br from-gray-100 to-gray-200 dark:from-dark-700 dark:to-dark-600'
})

const providerTintClass = computed(() => {
  if (providerKey.value === 'openai') return 'text-emerald-600 dark:text-emerald-300'
  if (providerKey.value === 'anthropic') return 'text-orange-600 dark:text-orange-300'
  if (providerKey.value === 'gemini') return 'text-sky-600 dark:text-sky-300'
  return 'text-gray-500 dark:text-gray-300'
})

const availabilityDisplay = computed(() => {
  const value = props.item.availability_7d
  if (value == null || Number.isNaN(value)) return t('monitorCommon.latencyEmpty')
  return value.toFixed(2)
})

const availabilityColorStyle = computed(() => {
  const colour = hslForPct(props.item.availability_7d)
  return colour ? { color: colour } : { color: 'rgb(156 163 175)' }
})

const displayBars = computed(() => {
  const real = [...(props.item.timeline || [])].slice(0, 60).reverse()
  const padCount = Math.max(0, 60 - real.length)
  const bars: Array<{ colorClass: string; heightPct: number; title: string }> = []
  for (let i = 0; i < padCount; i += 1) {
    bars.push({ colorClass: STATUS_COLOR.empty, heightPct: STATUS_HEIGHT.empty, title: '' })
  }
  for (const point of real) {
    const status = normalizeStatus(point.status)
    bars.push({
      colorClass: STATUS_COLOR[status] || STATUS_COLOR.empty,
      heightPct: STATUS_HEIGHT[status] || STATUS_HEIGHT.empty,
      title: `${formatRelativeTime(point.checked_at)} · ${statusLabel(point.status)} · ${formatLatency(point.latency_ms)}ms`,
    })
  }
  return bars
})

function normalizeProvider(provider?: string): string {
  const value = String(provider || '').toLowerCase()
  if (value.includes('anthropic') || value.includes('claude')) return 'anthropic'
  if (value.includes('gemini') || value.includes('google')) return 'gemini'
  if (value.includes('openai') || value.includes('gpt')) return 'openai'
  return value
}

function normalizeStatus(status?: SupplierMonitorStatus): string {
  const value = String(status || '').toLowerCase()
  if (value === 'ok' || value === 'normal' || value === 'healthy' || value === 'success') return 'operational'
  if (value === 'down' || value === 'timeout') return 'failed'
  return value || 'empty'
}

function statusLabel(status?: SupplierMonitorStatus): string {
  const value = normalizeStatus(status)
  if (value === 'operational') return t('monitorCommon.status.operational')
  if (value === 'degraded') return t('monitorCommon.status.degraded')
  if (value === 'failed') return t('monitorCommon.status.failed')
  if (value === 'error') return t('monitorCommon.status.error')
  return t('monitorCommon.status.unknown')
}

function statusBadgeClass(status?: SupplierMonitorStatus): string {
  const value = normalizeStatus(status)
  if (value === 'operational') return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300'
  if (value === 'degraded') return 'bg-amber-100 text-amber-700 dark:bg-amber-500/15 dark:text-amber-300'
  if (value === 'failed') return 'bg-red-100 text-red-700 dark:bg-red-500/15 dark:text-red-300'
  return 'bg-gray-100 text-gray-800 dark:bg-dark-700 dark:text-gray-300'
}

function formatLatency(ms: number | null | undefined): string {
  if (ms == null) return t('monitorCommon.latencyEmpty')
  return String(Math.round(ms))
}

function formatRelativeTime(iso: string | null | undefined): string {
  if (!iso) return t('monitorCommon.latencyEmpty')
  const ts = Date.parse(iso)
  if (Number.isNaN(ts)) return t('monitorCommon.latencyEmpty')
  const diffSec = Math.max(0, Math.floor((Date.now() - ts) / 1000))
  if (diffSec < 60) return t('monitorCommon.relativeSecondsAgo', { n: diffSec })
  const diffMin = Math.floor(diffSec / 60)
  if (diffMin < 60) return t('monitorCommon.relativeMinutesAgo', { n: diffMin })
  const diffHour = Math.floor(diffMin / 60)
  if (diffHour < 24) return t('monitorCommon.relativeHoursAgo', { n: diffHour })
  return t('monitorCommon.relativeDaysAgo', { n: Math.floor(diffHour / 24) })
}

function hslForPct(pct: number | null | undefined): string | undefined {
  if (pct === null || pct === undefined || Number.isNaN(pct)) return undefined
  return `hsl(${Math.max(0, Math.min(100, pct)) * 1.2} 72% 42%)`
}
</script>
