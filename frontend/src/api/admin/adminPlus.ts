import { apiClient } from '../client'

export interface AdminPlusListResponse<T> {
  items: T[]
  total: number
}

export type SupplierKind = 'source_account' | 'relay' | 'browser_only' | 'custom'
export type SupplierType = 'openai' | 'anthropic' | 'gemini' | 'sub2api' | 'new_api' | 'browser_only' | 'custom'
export type SupplierRuntimeStatus = 'monitor_only' | 'candidate' | 'active' | 'disabled'
export type SupplierHealthStatus = 'normal' | 'unavailable' | 'credential_invalid' | 'paused'

export interface SupplierCredentialStatus {
  postgres_configured: boolean
  redis_configured: boolean
  browser_login_enabled: boolean
  browser_login_username_configured: boolean
  browser_login_password_configured: boolean
  browser_login_token_configured: boolean
  masked_browser_login_username?: string
}

export interface Supplier {
  id: number
  name: string
  kind: SupplierKind
  type: SupplierType
  runtime_status: SupplierRuntimeStatus
  health_status: SupplierHealthStatus
  dashboard_url?: string
  api_base_url?: string
  contact?: string
  notes?: string
  credential: SupplierCredentialStatus
  balance_cents: number
  balance_currency: string
  balance_updated_at?: string | null
  created_at: string
  updated_at: string
}

export interface CreateSupplierPayload {
  name: string
  kind: SupplierKind
  type: SupplierType
  runtime_status?: SupplierRuntimeStatus
  health_status?: SupplierHealthStatus
  dashboard_url?: string
  api_base_url?: string
  contact?: string
  notes?: string
  postgres_read_dsn?: string
  redis_read_dsn?: string
  browser_login_enabled?: boolean
  browser_login_username?: string
  browser_login_password?: string
  browser_login_token?: string
  balance_cents?: number
  balance_currency?: string
}

export interface UpdateSupplierStatusPayload {
  runtime_status: SupplierRuntimeStatus
  health_status: SupplierHealthStatus
}

export interface LocalSub2APIAccount {
  id: number
  name: string
  platform: string
  type: string
  status: string
  schedulable: boolean
  concurrency: number
  priority: number
  rate_multiplier: number
}

export interface SupplierAccount {
  id: number
  supplier_id: number
  local_sub2api_account_id: number
  local_account_name: string
  local_account_platform: string
  local_account_type: string
  supplier_account_identifier?: string
  supplier_account_label?: string
  organization_id?: string
  project_id?: string
  rate_profile?: string
  configured_concurrency: number
  observed_max_concurrency: number
  balance_threshold_cents: number
  balance_cents: number
  balance_currency: string
  has_usable_balance: boolean
  runtime_status: SupplierRuntimeStatus
  health_status: SupplierHealthStatus
  created_at: string
  updated_at: string
}

export interface CreateSupplierAccountPayload {
  local_sub2api_account_id: number
  supplier_account_identifier?: string
  supplier_account_label?: string
  organization_id?: string
  project_id?: string
  rate_profile?: string
  configured_concurrency?: number
  balance_threshold_cents?: number
  balance_cents?: number
  balance_currency?: string
  runtime_status?: SupplierRuntimeStatus
  health_status?: SupplierHealthStatus
}

export interface RateSnapshotEntryPayload {
  model: string
  billing_mode: string
  price_item: string
  unit: string
  currency?: string
  price_micros: number
  raw_payload?: Record<string, unknown>
}

export interface RateSnapshot {
  id: number
  supplier_id: number
  source: string
  model: string
  billing_mode: string
  price_item: string
  unit: string
  currency: string
  price_micros: number
  raw_payload?: Record<string, unknown>
  captured_at: string
  created_at: string
}

export interface RateChangeEvent {
  id: number
  supplier_id: number
  snapshot_id: number
  model: string
  billing_mode: string
  price_item: string
  unit: string
  currency: string
  old_price_micros?: number | null
  new_price_micros: number
  direction: 'new' | 'increase' | 'decrease'
  change_percent?: number | null
  threshold_percent: number
  threshold_exceeded: boolean
  status: 'open' | 'acknowledged' | 'ignored'
  created_at: string
  acknowledged_at?: string | null
}

export interface BalanceSnapshot {
  id: number
  supplier_id: number
  source: string
  runtime_status: SupplierRuntimeStatus
  balance_cents: number
  currency: string
  switch_eligible: boolean
  raw_payload?: Record<string, unknown>
  captured_at: string
  created_at: string
}

export interface BalanceEvent {
  id: number
  supplier_id: number
  snapshot_id: number
  type: 'low_balance' | 'depleted' | 'recovered'
  runtime_status: SupplierRuntimeStatus
  old_balance_cents?: number | null
  new_balance_cents: number
  low_balance_threshold_cents: number
  currency: string
  switch_eligible: boolean
  status: 'open' | 'acknowledged' | 'ignored'
  created_at: string
  acknowledged_at?: string | null
}

export interface PromotionEvent {
  id: number
  supplier_id: number
  source: string
  type: 'recharge_bonus' | 'rate_discount' | 'package_deal' | 'limited_offer' | 'other'
  title: string
  description?: string
  currency: string
  min_recharge_cents: number
  bonus_percent?: number | null
  discount_percent?: number | null
  runtime_status: SupplierRuntimeStatus
  balance_cents: number
  switch_eligible: boolean
  recommendation: 'recharge_to_unlock' | 'switch_candidate' | 'monitor_only' | 'informational'
  status: 'open' | 'acknowledged' | 'ignored'
  starts_at?: string | null
  ends_at?: string | null
  captured_at: string
  created_at: string
  acknowledged_at?: string | null
  raw_payload?: Record<string, unknown>
}

export interface HealthSample {
  id: number
  supplier_id: number
  source: string
  model: string
  first_token_latency_ms: number
  total_latency_ms: number
  status_code: number
  error_class?: string
  observed_concurrency: number
  available_concurrency?: number | null
  concurrency_limit?: number | null
  raw_payload?: Record<string, unknown>
  captured_at: string
  created_at: string
}

export interface HealthEvent {
  id: number
  supplier_id: number
  sample_id: number
  type: 'slow_first_token' | 'slow_total' | 'request_error' | 'concurrency_full'
  model: string
  observed_value: number
  threshold_value: number
  status_code: number
  error_class?: string
  status: 'open' | 'acknowledged' | 'ignored'
  created_at: string
  acknowledged_at?: string | null
}

export interface SupplierBillLine {
  id: number
  supplier_id: number
  source: string
  external_bill_id?: string
  external_request_id?: string
  model: string
  currency: string
  cost_cents: number
  input_tokens: number
  output_tokens: number
  started_at: string
  ended_at?: string | null
  raw_payload?: Record<string, unknown>
  created_at: string
}

export interface LocalUsageLine {
  id: number
  external_request_id?: string
  model: string
  currency: string
  revenue_cents: number
  input_tokens?: number
  output_tokens?: number
  started_at: string
}

export interface ReconciliationLine {
  status: 'matched' | 'supplier_only' | 'local_only' | 'currency_mismatch' | 'cost_mismatch'
  supplier_bill_id?: number
  local_usage_id?: number
  external_request_id?: string
  model: string
  currency: string
  cost_cents: number
  revenue_cents: number
  profit_cents: number
  profit_margin?: number | null
  notes?: string
}

export interface ReconciliationSummary {
  total_supplier_lines: number
  total_local_lines: number
  matched_lines: number
  supplier_only_lines: number
  local_only_lines: number
  cost_cents: number
  revenue_cents: number
  profit_cents: number
  profit_margin?: number | null
}

export interface ReconciliationResult {
  lines: ReconciliationLine[]
  summary: ReconciliationSummary
}

export interface ExtensionTask {
  id: number
  supplier_id: number
  type: 'fetch_rates' | 'fetch_balance' | 'fetch_promotions' | 'export_bills' | 'fetch_health'
  status: 'pending' | 'claimed' | 'running' | 'succeeded' | 'failed' | 'cancelled'
  priority: number
  attempts: number
  max_attempts: number
  device_id?: string
  lease_token?: string
  lease_expires_at?: string | null
  last_heartbeat_at?: string | null
  available_after: string
  payload?: Record<string, unknown>
  result?: Record<string, unknown>
  error_code?: string
  error_message?: string
  created_at: string
  updated_at: string
  finished_at?: string | null
}

export interface ActionRecommendation {
  id: number
  supplier_id: number
  target_supplier_id?: number | null
  type: 'switch_supplier' | 'pause_supplier' | 'degrade_supplier' | 'increase_weight' | 'recharge_supplier' | 'investigate_profit' | 'review_credential'
  severity: 'info' | 'warning' | 'critical'
  status: 'open' | 'acknowledged' | 'approved' | 'executed' | 'rejected'
  reason_code: string
  title: string
  description: string
  expected_impact?: string
  requires_approval: boolean
  signals?: string[]
  created_at: string
}

export interface SupplierSignal {
  supplier_id: number
  name?: string
  runtime_status: SupplierRuntimeStatus
  health_status: SupplierHealthStatus
  balance_cents: number
  currency?: string
  effective_cost_cents: number
}

export async function listSuppliers(params?: Partial<Record<'kind' | 'type' | 'runtime_status' | 'health_status' | 'q', string>>): Promise<AdminPlusListResponse<Supplier>> {
  const { data } = await apiClient.get<AdminPlusListResponse<Supplier>>('/admin-plus/suppliers', { params })
  return data
}

export async function createSupplier(payload: CreateSupplierPayload): Promise<Supplier> {
  const { data } = await apiClient.post<Supplier>('/admin-plus/suppliers', payload)
  return data
}

export async function updateSupplierStatus(id: number, payload: UpdateSupplierStatusPayload): Promise<Supplier> {
  const { data } = await apiClient.patch<Supplier>(`/admin-plus/suppliers/${id}/status`, payload)
  return data
}

export async function listLocalSub2APIAccounts(params?: { q?: string; limit?: number }): Promise<AdminPlusListResponse<LocalSub2APIAccount>> {
  const { data } = await apiClient.get<AdminPlusListResponse<LocalSub2APIAccount>>('/admin-plus/sub2api/accounts', { params })
  return data
}

export async function listSupplierAccounts(supplierId: number): Promise<AdminPlusListResponse<SupplierAccount>> {
  const { data } = await apiClient.get<AdminPlusListResponse<SupplierAccount>>(`/admin-plus/suppliers/${supplierId}/accounts`)
  return data
}

export async function createSupplierAccount(supplierId: number, payload: CreateSupplierAccountPayload): Promise<SupplierAccount> {
  const { data } = await apiClient.post<SupplierAccount>(`/admin-plus/suppliers/${supplierId}/accounts`, payload)
  return data
}

export async function deleteSupplierAccount(supplierId: number, accountId: number): Promise<void> {
  await apiClient.delete(`/admin-plus/suppliers/${supplierId}/accounts/${accountId}`)
}

export async function recordRateSnapshot(payload: {
  supplier_id: number
  source?: string
  threshold_percent?: number
  entries: RateSnapshotEntryPayload[]
}) {
  const { data } = await apiClient.post<{ snapshots: RateSnapshot[]; events: RateChangeEvent[] }>('/admin-plus/rates/snapshots', payload)
  return data
}

export async function listRateSnapshots(params?: { supplier_id?: number; model?: string; limit?: number }) {
  const { data } = await apiClient.get<AdminPlusListResponse<RateSnapshot>>('/admin-plus/rates/snapshots', { params })
  return data
}

export async function listRateEvents(params?: { supplier_id?: number; status?: string; limit?: number }) {
  const { data } = await apiClient.get<AdminPlusListResponse<RateChangeEvent>>('/admin-plus/rates/events', { params })
  return data
}

export async function acknowledgeRateEvent(id: number): Promise<RateChangeEvent> {
  const { data } = await apiClient.patch<RateChangeEvent>(`/admin-plus/rates/events/${id}/ack`)
  return data
}

export async function recordBalanceSnapshot(payload: {
  supplier_id: number
  source?: string
  runtime_status?: SupplierRuntimeStatus
  balance_cents: number
  currency?: string
  low_balance_threshold_cents?: number
  raw_payload?: Record<string, unknown>
}) {
  const { data } = await apiClient.post<{ snapshot: BalanceSnapshot; event?: BalanceEvent | null }>('/admin-plus/balances/snapshots', payload)
  return data
}

export async function listBalanceSnapshots(params?: { supplier_id?: number; limit?: number }) {
  const { data } = await apiClient.get<AdminPlusListResponse<BalanceSnapshot>>('/admin-plus/balances/snapshots', { params })
  return data
}

export async function listBalanceEvents(params?: { supplier_id?: number; status?: string; limit?: number }) {
  const { data } = await apiClient.get<AdminPlusListResponse<BalanceEvent>>('/admin-plus/balances/events', { params })
  return data
}

export async function acknowledgeBalanceEvent(id: number): Promise<BalanceEvent> {
  const { data } = await apiClient.patch<BalanceEvent>(`/admin-plus/balances/events/${id}/ack`)
  return data
}

export async function recordPromotion(payload: {
  supplier_id: number
  source?: string
  type: PromotionEvent['type']
  title: string
  description?: string
  currency?: string
  min_recharge_cents?: number
  bonus_percent?: number | null
  discount_percent?: number | null
  runtime_status?: SupplierRuntimeStatus
  balance_cents?: number
  raw_payload?: Record<string, unknown>
}) {
  const { data } = await apiClient.post<PromotionEvent>('/admin-plus/promotions', payload)
  return data
}

export async function listPromotionEvents(params?: { supplier_id?: number; status?: string; recommendation?: string; limit?: number }) {
  const { data } = await apiClient.get<AdminPlusListResponse<PromotionEvent>>('/admin-plus/promotions', { params })
  return data
}

export async function acknowledgePromotionEvent(id: number): Promise<PromotionEvent> {
  const { data } = await apiClient.patch<PromotionEvent>(`/admin-plus/promotions/${id}/ack`)
  return data
}

export async function recordHealthSample(payload: {
  supplier_id: number
  source?: string
  model: string
  first_token_latency_ms?: number
  total_latency_ms?: number
  status_code?: number
  error_class?: string
  observed_concurrency?: number
  available_concurrency?: number | null
  concurrency_limit?: number | null
  first_token_threshold_ms?: number
  total_latency_threshold_ms?: number
  concurrency_saturation_percent?: number
  raw_payload?: Record<string, unknown>
}) {
  const { data } = await apiClient.post<{ sample: HealthSample; events: HealthEvent[] }>('/admin-plus/health/samples', payload)
  return data
}

export async function listHealthSamples(params?: { supplier_id?: number; model?: string; limit?: number }) {
  const { data } = await apiClient.get<AdminPlusListResponse<HealthSample>>('/admin-plus/health/samples', { params })
  return data
}

export async function listHealthEvents(params?: { supplier_id?: number; status?: string; type?: string; limit?: number }) {
  const { data } = await apiClient.get<AdminPlusListResponse<HealthEvent>>('/admin-plus/health/events', { params })
  return data
}

export async function acknowledgeHealthEvent(id: number): Promise<HealthEvent> {
  const { data } = await apiClient.patch<HealthEvent>(`/admin-plus/health/events/${id}/ack`)
  return data
}

export async function importBillLines(lines: Array<Omit<SupplierBillLine, 'id' | 'created_at' | 'source'> & { source?: string }>) {
  const { data } = await apiClient.post<AdminPlusListResponse<SupplierBillLine>>('/admin-plus/billing/lines/import', { lines })
  return data
}

export async function listBillLines(params?: { supplier_id?: number; limit?: number }) {
  const { data } = await apiClient.get<AdminPlusListResponse<SupplierBillLine>>('/admin-plus/billing/lines', { params })
  return data
}

export async function runReconciliation(payload: {
  supplier_bills: SupplierBillLine[]
  local_usages: LocalUsageLine[]
  time_tolerance_seconds?: number
  cost_mismatch_cents?: number
}) {
  const { data } = await apiClient.post<ReconciliationResult>('/admin-plus/reconciliation/run', payload)
  return data
}

export async function createExtensionTask(payload: {
  supplier_id: number
  type: ExtensionTask['type']
  priority?: number
  max_attempts?: number
  payload?: Record<string, unknown>
}) {
  const { data } = await apiClient.post<ExtensionTask>('/admin-plus/extension/tasks', payload)
  return data
}

export async function listExtensionTasks(params?: { supplier_id?: number; status?: string; type?: string; limit?: number }) {
  const { data } = await apiClient.get<AdminPlusListResponse<ExtensionTask>>('/admin-plus/extension/tasks', { params })
  return data
}

export async function claimExtensionTask(payload: { device_id: string; types?: ExtensionTask['type'][]; lease_ttl_seconds?: number }) {
  const { data } = await apiClient.post<ExtensionTask>('/admin-plus/extension/tasks/claim', payload)
  return data
}

export async function heartbeatExtensionTask(task: ExtensionTask, leaseTTLSeconds = 300) {
  const { data } = await apiClient.post<ExtensionTask>(`/admin-plus/extension/tasks/${task.id}/heartbeat`, {
    device_id: task.device_id,
    lease_token: task.lease_token,
    lease_ttl_seconds: leaseTTLSeconds
  })
  return data
}

export async function completeExtensionTask(task: ExtensionTask, result: Record<string, unknown>) {
  const { data } = await apiClient.post<ExtensionTask>(`/admin-plus/extension/tasks/${task.id}/complete`, {
    device_id: task.device_id,
    lease_token: task.lease_token,
    result
  })
  return data
}

export async function failExtensionTask(task: ExtensionTask, error_code: string, error_message: string) {
  const { data } = await apiClient.post<ExtensionTask>(`/admin-plus/extension/tasks/${task.id}/fail`, {
    device_id: task.device_id,
    lease_token: task.lease_token,
    error_code,
    error_message
  })
  return data
}

export async function generateActions(payload: {
  suppliers: SupplierSignal[]
  balance_events?: BalanceEvent[]
  promotion_events?: PromotionEvent[]
  health_events?: HealthEvent[]
  reconciliation?: ReconciliationSummary
  min_profit_margin?: number
}) {
  const { data } = await apiClient.post<AdminPlusListResponse<ActionRecommendation>>('/admin-plus/actions/generate', payload)
  return data
}

export async function listActionRecommendations(params?: { supplier_id?: number; status?: string; severity?: string; type?: string; limit?: number }) {
  const { data } = await apiClient.get<AdminPlusListResponse<ActionRecommendation>>('/admin-plus/actions/recommendations', { params })
  return data
}

export async function updateActionRecommendationStatus(id: number, status: ActionRecommendation['status']) {
  const { data } = await apiClient.patch<ActionRecommendation>(`/admin-plus/actions/recommendations/${id}/status`, { status })
  return data
}

export const adminPlusAPI = {
  listSuppliers,
  createSupplier,
  updateSupplierStatus,
  listLocalSub2APIAccounts,
  listSupplierAccounts,
  createSupplierAccount,
  deleteSupplierAccount,
  recordRateSnapshot,
  listRateSnapshots,
  listRateEvents,
  acknowledgeRateEvent,
  recordBalanceSnapshot,
  listBalanceSnapshots,
  listBalanceEvents,
  acknowledgeBalanceEvent,
  recordPromotion,
  listPromotionEvents,
  acknowledgePromotionEvent,
  recordHealthSample,
  listHealthSamples,
  listHealthEvents,
  acknowledgeHealthEvent,
  importBillLines,
  listBillLines,
  runReconciliation,
  createExtensionTask,
  listExtensionTasks,
  claimExtensionTask,
  heartbeatExtensionTask,
  completeExtensionTask,
  failExtensionTask,
  generateActions,
  listActionRecommendations,
  updateActionRecommendationStatus
}

export default adminPlusAPI
