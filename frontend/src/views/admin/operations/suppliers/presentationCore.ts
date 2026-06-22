import type { GroupPlatform } from '@/types'
import type { Supplier, SupplierChannelCheckSnapshot, SupplierCostSnapshot, SupplierGroup, SupplierHealthStatus, SupplierKind, SupplierRuntimeStatus, SupplierType } from '@/api/admin/adminPlus'
import type { Ref } from 'vue'
import type { ChannelProtocol, ScheduleListRow } from './types'
import { ctxFn, ctxValue } from './ctxProxy'
export function attachPresentationCore(ctx: any) {
  const groupsSupplier = ctxValue(ctx, 'groupsSupplier')
  const channelScheduleSupplier = ctxValue(ctx, 'channelScheduleSupplier')
  const suppliers = ctxValue(ctx, 'suppliers') as Ref<Supplier[]>
  const supplierCostSnapshots = ctxValue(ctx, 'supplierCostSnapshots') as Ref<Record<number, SupplierCostSnapshot | undefined>>
  const supplierBestChannels = ctxValue(ctx, 'supplierBestChannels') as Ref<Record<number, SupplierChannelCheckSnapshot[] | undefined>>
  const supplierChannelChecks = ctxValue(ctx, 'supplierChannelChecks') as Ref<Record<number, SupplierChannelCheckSnapshot | undefined>>
  const channelCheckActionKey = ctxValue(ctx, 'channelCheckActionKey')
  const channelProtocolFilter = ctxValue(ctx, 'channelProtocolFilter')
  const channelProbeStatusLabel = ctxFn(ctx, 'channelProbeStatusLabel')
  const toggleVisible = ctxFn(ctx, 'toggleVisible')
  function toggleSelectAllVisible(event: Event) {
    toggleVisible((event.target as HTMLInputElement).checked)
  }

  function centsFromYuan(value: number): number {
    return Math.round(Number(value || 0) * 100)
  }

  function yuanFromCents(value: number): number {
    return Number(((value || 0) / 100).toFixed(2))
  }

  function normalizeRechargeMultiplierForForm(value?: number | null): number {
    const multiplier = Number(value || 0)
    return Number.isFinite(multiplier) && multiplier > 0 ? multiplier : 1
  }

  function formatMoney(cents: number, currency: string): string {
    return new Intl.NumberFormat(undefined, {
      style: 'currency',
      currency: currency || 'USD',
      currencyDisplay: 'narrowSymbol',
      minimumFractionDigits: 2
    }).format((cents || 0) / 100)
  }

  function formatBalanceMoney(cents: number, currency: string): string {
    const normalized = normalizeBalanceAmountForDisplay(cents, currency)
    return formatMoney(normalized.cents, normalized.currency)
  }

  function normalizeBalanceAmountForDisplay(cents: number, currency: string): { cents: number; currency: string } {
    const normalizedCurrency = (currency || '').trim().toUpperCase()
    if (normalizedCurrency === 'QTA') {
      return {
        cents: Math.round(Number(cents || 0) / 500000),
        currency: 'USD'
      }
    }
    if (normalizedCurrency === '' || normalizedCurrency === 'CNY' || normalizedCurrency.length !== 3) {
      return {
        cents: Number(cents || 0),
        currency: 'USD'
      }
    }
    return {
      cents: Number(cents || 0),
      currency: normalizedCurrency
    }
  }

  function formatDateTime(value?: string | null): string {
    if (!value) return '-'
    const date = new Date(value)
    return Number.isNaN(date.getTime()) ? '-' : date.toLocaleString()
  }

  function supplierLinkURL(supplier: Supplier): string {
    return supplier.dashboard_url?.trim() || supplier.api_base_url?.trim() || ''
  }

  function supplierNameTitle(supplier: Supplier): string {
    const url = supplierLinkURL(supplier)
    return url ? `${supplier.name} · ${url}` : supplier.name
  }

  function formatMultiplier(value?: number | null): string {
    if (typeof value !== 'number') return '-'
    if (!Number.isFinite(value)) return '-'
    return `${value.toFixed(4).replace(/\.?0+$/, '')}x`
  }

  function rateMultiplierTextClass(value?: number | null, protocol?: ChannelProtocol, size: 'normal' | 'compact' = 'normal'): string {
    const sizeClass = size === 'compact' ? 'text-lg' : 'text-xl'
    const base = `inline-flex items-center justify-end rounded-md px-1.5 py-0.5 ${sizeClass} font-extrabold leading-tight ring-1 whitespace-nowrap`
    if (protocol === 'openai' && typeof value === 'number' && Number.isFinite(value) && value > 0.1) {
      return `${base} bg-rose-50 text-rose-700 ring-rose-200 dark:bg-rose-950/50 dark:text-rose-300 dark:ring-rose-800/60`
    }
    return `${base} bg-green-50 text-green-800 ring-green-200 dark:bg-green-950/50 dark:text-green-300 dark:ring-green-800/60`
  }

  function formatLatency(value?: number | null): string {
    if (typeof value !== 'number' || value <= 0) return '-'
    if (value >= 1000) return `${(value / 1000).toFixed(value >= 10000 ? 0 : 1)}s`
    return `${Math.round(value)}ms`
  }

  function formatUSDLimit(value?: number | null): string {
    if (typeof value !== 'number') return '-'
    return new Intl.NumberFormat(undefined, {
      style: 'currency',
      currency: 'USD',
      currencyDisplay: 'narrowSymbol',
      maximumFractionDigits: 2
    }).format(value)
  }

  function kindLabel(value: SupplierKind): string {
    return {
      source_account: '源站',
      relay: '中转',
      browser_only: '浏览器',
      custom: '自定义'
    }[value]
  }

  function typeLabel(value: SupplierType): string {
    return {
      openai: 'OpenAI',
      anthropic: 'Anthropic',
      gemini: 'Gemini',
      sub2api: 'Sub2API',
      new_api: 'New API',
      browser_only: '仅浏览器',
      custom: '自定义'
    }[value]
  }

  function groupPlatform(value?: string, ...hints: Array<string | undefined>): GroupPlatform {
    const provider = protocolHaystack(value, ...hints)
    if (provider.includes('anthropic') || provider.includes('claude')) return 'anthropic'
    if (provider.includes('gemini') || provider.includes('google')) return 'gemini'
    if (providerLooksOpenAI(provider)) return 'openai'
    return 'antigravity'
  }

  function groupPlatformFromProvider(value?: string, ...hints: Array<string | undefined>): GroupPlatform {
    return groupPlatform(value, ...hints)
  }

  function channelProtocolFromProviderFamily(value?: string, ...hints: Array<string | undefined>): ChannelProtocol {
    const platform = groupPlatform(value, ...hints)
    if (platform === 'openai') return 'openai'
    if (platform === 'anthropic') return 'claude'
    if (platform === 'gemini') return 'gemini'
    return 'other'
  }

  function normalizedRechargeMultiplier(value?: number | null): number {
    const multiplier = Number(value || 0)
    return Number.isFinite(multiplier) && multiplier > 0 ? multiplier : 1
  }

  function supplierRechargeMultiplier(supplierID?: number | null): number {
    if (!supplierID) return 1
    return normalizedRechargeMultiplier(suppliers.value.find((supplier) => supplier.id === supplierID)?.recharge_multiplier)
  }

  function currentSupplierRechargeMultiplier(): number {
    return normalizedRechargeMultiplier(groupsSupplier.value?.recharge_multiplier)
  }

  function channelScheduleSupplierRechargeMultiplier(): number {
    return normalizedRechargeMultiplier(channelScheduleSupplier.value?.recharge_multiplier)
  }

  function actualCostMultiplier(rate?: number | null, rechargeMultiplier?: number | null): number {
    const usageRate = Number(rate || 0)
    if (!Number.isFinite(usageRate) || usageRate <= 0) return 0
    return usageRate / normalizedRechargeMultiplier(rechargeMultiplier)
  }

  function groupCostMultiplier(group?: SupplierGroup | null): number {
    return actualCostMultiplier(group?.effective_rate_multiplier, currentSupplierRechargeMultiplier())
  }

  function channelCostMultiplier(snapshot?: SupplierChannelCheckSnapshot | null): number {
    return actualCostMultiplier(snapshot?.effective_rate_multiplier, supplierRechargeMultiplier(snapshot?.supplier_id))
  }

  function scheduleRowCostMultiplier(row: ScheduleListRow): number {
    return actualCostMultiplier(row.effective_rate_multiplier || row.supplier_group_rate, supplierRechargeMultiplier(row.supplier_id))
  }

  function protocolHaystack(...values: Array<string | undefined>): string {
    return values.filter(Boolean).join(' ').toLowerCase()
  }

  function providerLooksOpenAI(value: string): boolean {
    return value.includes('openai') ||
      value.includes('gpt') ||
      value.includes('chatgpt') ||
      /\bo[34]\b/.test(value) ||
      /\bpro\b/.test(value) ||
      /\bplus\b/.test(value)
  }

  function providerLabel(value?: string): string {
    const provider = (value || 'mixed').toLowerCase()
    if (provider.includes('anthropic') || provider.includes('claude')) return 'Anthropic / Claude'
    if (provider.includes('gemini') || provider.includes('google')) return 'Gemini'
    if (provider.includes('openai') || provider.includes('gpt')) return 'OpenAI'
    if (provider.includes('image')) return 'Image'
    return provider === 'mixed' ? '混合渠道' : value || '混合渠道'
  }

  function runtimeLabel(value: SupplierRuntimeStatus): string {
    return {
      monitor_only: '仅监控',
      candidate: '候选',
      active: '使用中',
      disabled: '停用'
    }[value]
  }

  function healthLabel(value: SupplierHealthStatus): string {
    return {
      normal: '正常',
      unavailable: '不可用',
      credential_invalid: '凭据失效',
      paused: '暂停'
    }[value]
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

  function supplierCostSnapshot(supplierID: number): SupplierCostSnapshot | undefined {
    return supplierCostSnapshots.value[supplierID]
  }

  function preferredSupplierCostSnapshots(supplierItems: Supplier[], snapshots: SupplierCostSnapshot[]): Record<number, SupplierCostSnapshot | undefined> {
    const suppliersByID = new Map(supplierItems.map((supplier) => [supplier.id, supplier]))
    const selected: Record<number, SupplierCostSnapshot | undefined> = {}
    for (const snapshot of snapshots) {
      const current = selected[snapshot.supplier_id]
      if (!current || shouldUseCostSnapshot(snapshot, current, suppliersByID.get(snapshot.supplier_id))) {
        selected[snapshot.supplier_id] = snapshot
      }
    }
    return selected
  }

  function shouldUseCostSnapshot(candidate: SupplierCostSnapshot, current: SupplierCostSnapshot, supplier?: Supplier): boolean {
    const candidateScore = costSnapshotCurrencyScore(candidate.currency, supplier)
    const currentScore = costSnapshotCurrencyScore(current.currency, supplier)
    if (candidateScore !== currentScore) return candidateScore > currentScore
    return new Date(candidate.captured_at).getTime() > new Date(current.captured_at).getTime()
  }

  function costSnapshotCurrencyScore(currency: string, supplier?: Supplier): number {
    const value = (currency || '').toUpperCase()
    if (value === 'USD') return 3
    if (supplier && value === (supplier.balance_currency || '').toUpperCase()) return 2
    return 1
  }

  function supplierBestChannel(supplierID: number): SupplierChannelCheckSnapshot | undefined {
    return supplierBestChannelVariants(supplierID)[0]
  }

  function supplierAllBestChannelVariants(supplierID: number): SupplierChannelCheckSnapshot[] {
    return [...(supplierBestChannels.value[supplierID] || [])].sort(compareChannelProtocolSnapshots)
  }

  function supplierBestChannelVariants(supplierID: number): SupplierChannelCheckSnapshot[] {
    const items = supplierAllBestChannelVariants(supplierID)
    if (!channelProtocolFilter.value) return items
    return items.filter((item) => channelProtocol(item) === channelProtocolFilter.value)
  }

  function supplierBestChannelTooltip(supplierID: number): string {
    const items = supplierAllBestChannelVariants(supplierID)
    if (items.length === 0) return ''
    return items.map((item) => [
      `${channelProtocolLabel(channelProtocol(item))}：${item.group_name || '-'}`,
      `实际倍率 ${formatMultiplier(channelCostMultiplier(item))}`,
      `使用倍率 ${formatMultiplier(item.effective_rate_multiplier)}`,
      `充值倍率 ${formatMultiplier(supplierRechargeMultiplier(item.supplier_id))}`,
      `首 Token ${formatLatency(item.first_token_ms)}`,
      `总耗时 ${formatLatency(item.duration_ms)}`,
      `状态 ${channelProbeStatusLabel(item.probe_status)}`,
      item.local_account_schedulable ? '已入调度' : '未入调度',
      `检测时间 ${formatDateTime(item.captured_at)}`,
      item.error_message ? `错误 ${item.error_message}` : ''
    ].filter(Boolean).join(' · ')).join('\n')
  }

  function supplierAvailableChannelProtocolLabels(supplierID: number): string {
    const protocols = new Set(supplierAllBestChannelVariants(supplierID).map((item) => channelProtocol(item)))
    const labels = [...protocols]
      .sort((a, b) => channelProtocolPriority(a) - channelProtocolPriority(b))
      .map((item) => channelProtocolLabel(item))
    return labels.join(' / ') || '-'
  }

  function channelProtocol(snapshot?: SupplierChannelCheckSnapshot): ChannelProtocol {
    if (!snapshot) return 'other'
    const provider = (snapshot.provider_family || '').trim().toLowerCase()
    if (provider === 'openai') return 'openai'
    if (provider === 'anthropic') return 'claude'
    if (provider === 'gemini') return 'gemini'
    const haystack = protocolHaystack(
      snapshot.provider_family,
      snapshot.group_name,
      snapshot.channel_name,
      snapshot.channel_provider,
      snapshot.primary_model,
      snapshot.probe_model
    )
    if (haystack.includes('anthropic') || haystack.includes('claude')) return 'claude'
    if (haystack.includes('gemini') || haystack.includes('google')) return 'gemini'
    if (providerLooksOpenAI(haystack)) return 'openai'
    return 'other'
  }

  function channelProtocolLabel(protocol: ChannelProtocol): string {
    return {
      openai: 'OpenAI',
      claude: 'Claude',
      gemini: 'Gemini',
      other: '其他'
    }[protocol]
  }

  function channelProtocolBadgeClass(protocol: ChannelProtocol): string {
    return {
      openai: 'badge-primary',
      claude: 'badge-purple',
      gemini: 'badge-success',
      other: 'badge-gray'
    }[protocol]
  }

  function channelProtocolPriority(protocol: ChannelProtocol): number {
    return {
      openai: 0,
      claude: 1,
      gemini: 2,
      other: 3
    }[protocol]
  }

  function compareChannelProtocolSnapshots(a: SupplierChannelCheckSnapshot, b: SupplierChannelCheckSnapshot): number {
    const protocolDelta = channelProtocolPriority(channelProtocol(a)) - channelProtocolPriority(channelProtocol(b))
    if (protocolDelta !== 0) return protocolDelta
    const availableDelta = Number(channelIsAvailable(b)) - Number(channelIsAvailable(a))
    if (availableDelta !== 0) return availableDelta
    const schedulableDelta = Number(b.local_account_schedulable) - Number(a.local_account_schedulable)
    if (schedulableDelta !== 0) return schedulableDelta
    const rateDelta = channelCostMultiplier(a) - channelCostMultiplier(b)
    if (rateDelta !== 0) return rateDelta
    return new Date(b.captured_at).getTime() - new Date(a.captured_at).getTime()
  }

  function upsertSupplierBestChannelSnapshot(snapshot: SupplierChannelCheckSnapshot) {
    const existing = supplierBestChannels.value[snapshot.supplier_id] || []
    const protocol = channelProtocol(snapshot)
    supplierBestChannels.value = {
      ...supplierBestChannels.value,
      [snapshot.supplier_id]: [
        ...existing.filter((item) => channelProtocol(item) !== protocol),
        snapshot
      ].sort(compareChannelProtocolSnapshots)
    }
  }

  function groupChannelCheck(groupID: number): SupplierChannelCheckSnapshot | undefined {
    return supplierChannelChecks.value[groupID]
  }

  function isChannelCheckActionRunning(key: string): boolean {
    return channelCheckActionKey.value === key
  }

  function isBestChannelSchedulingRunning(supplierID: number): boolean {
    return channelCheckActionKey.value === `best-schedule:${supplierID}`
  }

  function limeProvisionActionKey(supplierID: number, supplierGroupID: number): string {
    return `lime-provision:${supplierID}:${supplierGroupID}`
  }

  function isLimeProvisionRunning(supplierID: number, supplierGroupID: number): boolean {
    return channelCheckActionKey.value === limeProvisionActionKey(supplierID, supplierGroupID)
  }

  function channelHasLocalBinding(snapshot?: SupplierChannelCheckSnapshot): boolean {
    return Boolean(snapshot?.supplier_account_id && snapshot.local_sub2api_account_id)
  }

  function channelIsAvailable(snapshot?: SupplierChannelCheckSnapshot): boolean {
    return snapshot?.probe_status === 'available'
  }

  function scheduleChannelKey(supplierID: number, supplierGroupID?: number): string {
    return `${supplierID}:${supplierGroupID || 0}`
  }

  Object.assign(ctx, {
    toggleSelectAllVisible,
    centsFromYuan,
    yuanFromCents,
    normalizeRechargeMultiplierForForm,
    formatMoney,
    formatBalanceMoney,
    normalizeBalanceAmountForDisplay,
    formatDateTime,
    supplierLinkURL,
    supplierNameTitle,
    formatMultiplier,
    rateMultiplierTextClass,
    formatLatency,
    formatUSDLimit,
    kindLabel,
    typeLabel,
    groupPlatform,
    groupPlatformFromProvider,
    channelProtocolFromProviderFamily,
    normalizedRechargeMultiplier,
    supplierRechargeMultiplier,
    currentSupplierRechargeMultiplier,
    channelScheduleSupplierRechargeMultiplier,
    actualCostMultiplier,
    groupCostMultiplier,
    channelCostMultiplier,
    scheduleRowCostMultiplier,
    protocolHaystack,
    providerLooksOpenAI,
    providerLabel,
    runtimeLabel,
    healthLabel,
    runtimeClass,
    healthClass,
    supplierCostSnapshot,
    preferredSupplierCostSnapshots,
    shouldUseCostSnapshot,
    costSnapshotCurrencyScore,
    supplierBestChannel,
    supplierAllBestChannelVariants,
    supplierBestChannelVariants,
    supplierBestChannelTooltip,
    supplierAvailableChannelProtocolLabels,
    channelProtocol,
    channelProtocolLabel,
    channelProtocolBadgeClass,
    channelProtocolPriority,
    compareChannelProtocolSnapshots,
    upsertSupplierBestChannelSnapshot,
    groupChannelCheck,
    isChannelCheckActionRunning,
    isBestChannelSchedulingRunning,
    limeProvisionActionKey,
    isLimeProvisionRunning,
    channelHasLocalBinding,
    channelIsAvailable,
    scheduleChannelKey
  })
}
