import type { SiteDiscoveryItem, SiteDiscoveryRegistrationTask, SupplierRegistrationStatus } from '@/api/admin/adminPlus'

export function canImportDiscoveredSupplier(item: SiteDiscoveryItem): boolean {
  return item.registration_status === 'succeeded' && Boolean(item.supplier_id)
}

export function canQueueSiteRegistration(item: SiteDiscoveryItem, registrationEnabled: boolean): boolean {
  if (!registrationEnabled || !isSupportedProvider(item)) return false
  return !['queued', 'running', 'succeeded'].includes(item.registration_status || '')
}

export function canRerunRegistration(task: SiteDiscoveryRegistrationTask): boolean {
  return task.can_retry === true && ['queued', 'running', 'failed', 'waiting_manual_verification'].includes(task.status)
}

export function registrationLabel(status?: SupplierRegistrationStatus | ''): string {
  return {
    pending: '待处理',
    queued: '待浏览器兜底',
    running: '执行中',
    waiting_manual_verification: '待人工验证',
    succeeded: '成功',
    failed: '失败',
    '': '未注册'
  }[status || ''] || status || '未注册'
}

export function registrationClass(status?: SupplierRegistrationStatus | ''): string {
  if (status === 'succeeded') return 'badge-success'
  if (status === 'failed') return 'badge-danger'
  if (status === 'waiting_manual_verification' || status === 'queued' || status === 'running') return 'badge-warning'
  return 'badge-gray'
}

export function siteDiscoveryImportHint(item: SiteDiscoveryItem): string {
  if (item.supplier_id && item.registration_status === 'succeeded') return `已入库 #${item.supplier_id}`
  if (item.registration_status === 'succeeded') return '已注册，等待供应商入库'
  if (item.registration_status === 'queued') return '等待浏览器兜底完成后自动入库'
  if (item.registration_status) return '注册完成后自动入库'
  if (isSupportedProvider(item)) return '需先注册'
  return '不支持导入'
}

export function isSiteDiscoveryProcessed(item: SiteDiscoveryItem): boolean {
  if (item.process_status && item.process_status !== 'unprocessed') return true
  if (item.import_status !== 'new') return true
  return item.registration_status === 'succeeded'
}

export function siteDiscoveryProcessedLabel(item: SiteDiscoveryItem): string {
  if (item.process_status === 'added_to_catalog') return '已入目录'
  if (item.supplier_id || item.import_status === 'imported') return '已入库'
  if (item.process_status === 'ignored') return '已忽略'
  if (item.registration_status === 'queued' || item.registration_status === 'running') return '注册中'
  if (item.registration_status === 'waiting_manual_verification') return '待验证'
  if (item.registration_status === 'failed') return '注册失败'
  return isSiteDiscoveryProcessed(item) ? '已处理' : '未处理'
}

function isSupportedProvider(item: SiteDiscoveryItem): boolean {
  return item.classification_status === 'supported' && ['new_api', 'sub2api'].includes(item.provider_type || '')
}
