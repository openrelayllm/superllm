import { apiClient } from '../client'

export interface AdminPlusListResponse<T> {
  items: T[]
  total: number
  page?: number
  page_size?: number
  pages?: number
}

export interface AdminPlusPaginationParams {
  page?: number
  page_size?: number
  limit?: number
}

export type SupplierKind = 'source_account' | 'relay' | 'browser_only' | 'custom'
export type SupplierType = 'openai' | 'anthropic' | 'gemini' | 'sub2api' | 'new_api' | 'browser_only' | 'custom'
export type SupplierRuntimeStatus = 'monitor_only' | 'candidate' | 'active' | 'disabled'
export type SupplierHealthStatus = 'normal' | 'unavailable' | 'credential_invalid' | 'paused'
export type SupplierGroupStatus = 'active' | 'missing' | 'disabled'
export type SupplierKeyStatus = 'provisioning' | 'bound' | 'manual_secret_required' | 'failed' | 'disabled'
export type SupplierSessionSource = 'direct_login' | 'browser_extension' | 'manual_import'
export type SupplierProvisionJobType = 'sync_groups' | 'provision_group_key' | 'provision_all_group_keys' | 'repair_binding' | 'sync_supplier_costs'
export type SupplierProvisionStatus = 'queued' | 'running' | 'succeeded' | 'partial_succeeded' | 'retryable_failed' | 'manual_required' | 'dead' | 'cancelled'
export type SupplierProvisionStepType =
  | 'ensure_supplier_session'
  | 'sync_supplier_group'
  | 'ensure_third_party_key'
  | 'ensure_sub2api_group'
  | 'ensure_sub2api_account'
  | 'upsert_admin_plus_binding'
  | 'enqueue_initial_collection'
  | 'provision_all_group_keys'
  | 'repair_binding'
  | 'sync_supplier_costs'

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
  third_party_recharge_url?: string
  local_recharge_url?: string
  contact?: string
  notes?: string
  credential: SupplierCredentialStatus
  balance_cents: number
  balance_currency: string
  balance_updated_at?: string | null
  created_at: string
  updated_at: string
}

export interface SupplierSiteMatchRequest {
  url?: string
  origin?: string
  host?: string
  path?: string
  title?: string
  favicon_url?: string
}

export interface SupplierSiteMatchCandidate {
  id: number
  name: string
  kind: SupplierKind
  type: SupplierType
  dashboard_url?: string
  api_base_url?: string
  third_party_recharge_url?: string
  local_recharge_url?: string
  match_fields?: string[]
}

export interface SupplierSiteMatchResult {
  status: 'matched' | 'ambiguous' | 'unknown' | 'unsupported'
  supplier_id?: number
  supplier?: SupplierSiteMatchCandidate
  candidates?: SupplierSiteMatchCandidate[]
  suggested_supplier?: Partial<CreateSupplierPayload>
}

export interface SupplierBrowserSession {
  supplier_id: number
  session_source: SupplierSessionSource
  origin: string
  api_base_url?: string
  session_summary?: Record<string, unknown>
  captured_at?: string
  expires_at?: string | null
  source_extension_task_id?: number
  created_at?: string
  updated_at?: string
  status: 'valid' | 'expired'
  has_encrypted_bundle: boolean
}

export interface SupplierSessionProbeResult {
  supplier_id: number
  status: string
  system_type: string
  origin: string
  api_base_url?: string
  capabilities: Record<string, boolean>
  profile?: {
    id?: number
    email?: string
    username?: string
    role?: string
    status?: string
    balance: number
    concurrency?: number
    allowed_groups?: number[]
  }
  balance_cents?: number
  balance_currency?: string
  diagnostics?: Record<string, unknown>
  probed_at: string
}

export interface SupplierCurrentBalance {
  supplier_id: number
  runtime_status: SupplierRuntimeStatus
  balance_cents: number
  currency: string
  switch_eligible: boolean
  source: string
  captured_at: string
  refresh_after: string
  expires_at: string
  stale: boolean
  expired: boolean
  fallback: boolean
  refresh_error_reason?: string
  refresh_error_message?: string
}

export interface ProbeSupplierSessionResponse {
  probe: SupplierSessionProbeResult
  balance_snapshot?: BalanceSnapshot
  balance_event?: BalanceEvent | null
}

export type SupplierMonitorStatus = 'operational' | 'degraded' | 'failed' | 'error' | string

export interface SupplierChannelMonitorTimelinePoint {
  status: SupplierMonitorStatus
  latency_ms?: number | null
  ping_latency_ms?: number | null
  checked_at: string
}

export interface SupplierChannelMonitorExtraModel {
  model: string
  status: SupplierMonitorStatus
  latency_ms?: number | null
}

export interface SupplierChannelMonitorView {
  id: number
  name: string
  provider: string
  group_name: string
  primary_model: string
  primary_status: SupplierMonitorStatus
  primary_latency_ms?: number | null
  primary_ping_latency_ms?: number | null
  availability_7d: number
  extra_models: SupplierChannelMonitorExtraModel[]
  timeline: SupplierChannelMonitorTimelinePoint[]
}

export interface SupplierChannelMonitorListResponse {
  supplier_id: number
  system_type: string
  origin: string
  api_base_url?: string
  items: SupplierChannelMonitorView[]
  captured_at: string
}

export interface LoginSupplierSessionPayload {
  origin?: string
  api_base_url?: string
  username?: string
  password?: string
  token?: string
  login_context?: Record<string, unknown>
  low_balance_threshold_cents?: number
  record_balance_snapshot?: boolean
}

export interface LoginSupplierSessionResponse {
  session: SupplierBrowserSession
  probe?: SupplierSessionProbeResult
  balance_snapshot?: BalanceSnapshot
  balance_event?: BalanceEvent | null
  diagnostics?: Record<string, unknown>
}

export interface UpsertSupplierBrowserSessionPayload {
  origin: string
  api_base_url?: string
  captured_at?: string
  expires_at?: string | null
  session_bundle: Record<string, unknown>
}

export interface CreateSupplierPayload {
  name: string
  kind: SupplierKind
  type: SupplierType
  runtime_status?: SupplierRuntimeStatus
  health_status?: SupplierHealthStatus
  dashboard_url?: string
  api_base_url?: string
  third_party_recharge_url?: string
  local_recharge_url?: string
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

export type UpdateSupplierPayload = CreateSupplierPayload

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

export interface LocalAccountTestModel {
  id: string
  type: string
  display_name: string
  created_at?: string
}

export interface LocalAccountTestPayload {
  model_id: string
  prompt?: string
  mode?: string
}

export interface SupplierAccount {
  id: number
  supplier_id: number
  supplier_key_id?: number
  local_sub2api_account_id: number
  local_account_name: string
  local_account_platform: string
  local_account_type: string
  supplier_account_identifier?: string
  supplier_account_label?: string
  supplier_group_id?: number
  supplier_external_group_id?: string
  supplier_group_name?: string
  supplier_group_provider?: string
  supplier_group_rate?: number
  supplier_key_name?: string
  supplier_key_external_id?: string
  supplier_key_last4?: string
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

export interface SupplierKey {
  id: number
  supplier_id: number
  supplier_group_id: number
  external_group_id: string
  external_key_id?: string
  name: string
  key_fingerprint?: string
  key_last4?: string
  status: SupplierKeyStatus
  provider_family: string
  local_sub2api_account_id?: number
  local_account_name?: string
  local_account_platform?: string
  local_account_type?: string
  error_code?: string
  error_message?: string
  provision_request?: Record<string, unknown>
  provision_response?: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface ProvisionSupplierKeyPayload {
  supplier_group_id: number
  name?: string
  quota_usd?: number
  expires_in_days?: number | null
  local_account_platform?: string
  local_account_name?: string
  local_account_base_url: string
  local_account_concurrency?: number
  local_account_priority?: number
  local_account_rate_multiplier?: number | null
  local_account_group_ids?: number[]
  runtime_status?: SupplierRuntimeStatus
  health_status?: SupplierHealthStatus
  balance_threshold_cents?: number
  balance_cents?: number
  balance_currency?: string
}

export interface ProvisionSupplierKeyResponse {
  key: SupplierKey
  binding: SupplierAccount
}

export interface EnsureSupplierKeysPayload {
  local_account_base_url?: string
  local_account_concurrency?: number
  local_account_priority?: number
  runtime_status?: SupplierRuntimeStatus
  health_status?: SupplierHealthStatus
  balance_threshold_cents?: number
  balance_cents?: number
  balance_currency?: string
}

export interface EnsureSupplierKeyItem {
  supplier_group_id: number
  external_group_id: string
  group_name: string
  action: 'created' | 'skipped' | 'failed'
  key?: SupplierKey
  binding?: SupplierAccount
  local_sub2api_group_id?: number
  local_sub2api_group_name?: string
  local_group_created?: boolean
  local_account_group_bound?: boolean
  error_code?: string
  error_message?: string
}

export interface EnsureSupplierKeysResponse {
  supplier_id: number
  total: number
  created: number
  skipped: number
  failed: number
  local_groups_created: number
  local_accounts_bound: number
  items: EnsureSupplierKeyItem[]
}

export interface SupplierProvisionStep {
  id: number
  job_id: number
  supplier_id: number
  supplier_group_id?: number
  step_type: SupplierProvisionStepType
  status: SupplierProvisionStatus | 'skipped'
  attempts: number
  max_attempts: number
  next_run_at: string
  error_code?: string
  error_message?: string
  request_snapshot?: Record<string, unknown>
  result_snapshot?: Record<string, unknown>
  created_at: string
  updated_at: string
  finished_at?: string | null
}

export interface SupplierProvisionJob {
  id: number
  job_type: SupplierProvisionJobType
  supplier_id: number
  status: SupplierProvisionStatus
  requested_by?: number
  request_snapshot?: Record<string, unknown>
  result_snapshot?: Record<string, unknown>
  total_steps: number
  succeeded_steps: number
  failed_steps: number
  manual_required_steps: number
  attempts: number
  max_attempts: number
  next_run_at: string
  error_code?: string
  error_message?: string
  created_at: string
  updated_at: string
  finished_at?: string | null
  steps?: SupplierProvisionStep[]
}

export interface SubmitProvisionJobResponse {
  job_id: number
  status: SupplierProvisionStatus
  job_type: SupplierProvisionJobType
  supplier_id: number
  supplier_group_id?: number
  poll_url: string
  mode: 'async_job'
  replayed?: boolean
}

export interface RepairSupplierKeyBindingPayload {
  local_sub2api_account_id: number
  runtime_status?: SupplierRuntimeStatus
  health_status?: SupplierHealthStatus
  configured_concurrency?: number
  balance_threshold_cents?: number
  balance_cents?: number
  balance_currency?: string
  supplier_account_identifier?: string
  supplier_account_label?: string
}

export interface SupplierGroup {
  id: number
  supplier_id: number
  external_group_id: string
  name: string
  description: string
  provider_family: string
  rate_multiplier: number
  user_rate_multiplier?: number | null
  effective_rate_multiplier: number
  rpm_limit?: number | null
  daily_limit_usd?: number | null
  weekly_limit_usd?: number | null
  monthly_limit_usd?: number | null
  allow_image_generation: boolean
  is_private: boolean
  status: SupplierGroupStatus
  raw_payload?: Record<string, unknown>
  last_seen_at: string
  created_at: string
  updated_at: string
}

export interface SyncSupplierGroupsResponse {
  supplier_id: number
  system_type: string
  origin: string
  api_base_url?: string
  groups: SupplierGroup[]
  synced_at: string
  total: number
}

export interface CreateSupplierAccountPayload {
  local_sub2api_account_id: number
  supplier_key_id?: number
  supplier_account_identifier?: string
  supplier_account_label?: string
  organization_id?: string
  project_id?: string
  rate_profile?: string
  configured_concurrency?: number
  observed_max_concurrency?: number
  balance_threshold_cents?: number
  balance_cents?: number
  balance_currency?: string
  runtime_status?: SupplierRuntimeStatus
  health_status?: SupplierHealthStatus
}

export type UpdateSupplierAccountPayload = Omit<CreateSupplierAccountPayload, 'local_sub2api_account_id'>

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

export interface AnnouncementEvent {
  id: number
  supplier_id: number
  source: string
  type: 'recharge_bonus' | 'rate_discount' | 'package_deal' | 'limited_offer' | 'maintenance' | 'incident' | 'notice' | 'other'
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

export interface SupplierUsageCostLine {
  id: number
  supplier_id: number
  source: string
  external_usage_cost_id?: string
  external_request_id?: string
  api_key_name?: string
  model: string
  endpoint?: string
  request_type?: string
  billing_mode?: string
  reasoning_effort?: string
  currency: string
  cost_cents: number
  input_tokens: number
  output_tokens: number
  cache_read_tokens: number
  total_tokens: number
  first_token_ms: number
  duration_ms: number
  user_agent?: string
  started_at: string
  ended_at?: string | null
  raw_payload?: Record<string, unknown>
  created_at: string
}

export interface SyncSupplierUsageCostsPayload {
  started_at: string
  ended_at: string
}

export interface SyncSupplierUsageCostsResponse {
  supplier_id: number
  system_type: string
  origin: string
  api_base_url?: string
  synced_at: string
  total: number
  items: SupplierUsageCostLine[]
}

export interface SupplierCostSnapshot {
  id: number
  supplier_id: number
  currency: string
  completed_funding_amount_cents: number
  completed_funding_cash_cents: number
  entitlement_amount_cents: number
  usage_cost_cents: number
  refund_amount_cents: number
  adjustment_amount_cents: number
  expected_balance_cents: number
  actual_balance_cents?: number | null
  balance_delta_cents?: number | null
  captured_at: string
  created_at: string
}

export interface SupplierFundingTransaction {
  id: number
  supplier_id: number
  provider_type: string
  external_id: string
  out_trade_no?: string
  payment_trade_no?: string
  payment_type?: string
  order_type?: string
  status: string
  currency: string
  amount_cents: number
  cash_amount_cents: number
  refund_amount_cents: number
  fee_rate?: number | null
  created_at_external?: string | null
  paid_at?: string | null
  completed_at?: string | null
  last_seen_at: string
  created_at: string
  updated_at: string
}

export interface SupplierEntitlementTransaction {
  id: number
  supplier_id: number
  provider_type: string
  external_id: string
  code_fingerprint?: string
  code_last4?: string
  source_family: string
  type: string
  status: string
  currency: string
  value_cents: number
  raw_value: number
  group_id?: number
  validity_days?: number
  used_at?: string | null
  created_at_external?: string | null
  last_seen_at: string
  created_at: string
  updated_at: string
}

export interface SupplierCostLedgerEntry {
  id: number
  supplier_id: number
  provider_type: string
  entry_type: string
  source_type: string
  source_id: number
  source_external_id?: string
  currency: string
  amount_cents: number
  cash_amount_cents: number
  occurred_at: string
  created_at: string
}

export interface SyncSupplierCostsPayload {
  started_at?: string
  ended_at?: string
  include_funding_transactions?: boolean
  include_entitlement_transactions?: boolean
  include_usage_cost_lines?: boolean
  include_balance_snapshot?: boolean
  low_balance_threshold_cents?: number
}

export type SyncSupplierCostsResponse = SubmitProvisionJobResponse

export interface SupplierCostSyncResultSnapshot {
  supplier_id: number
  provider_type?: string
  system_type?: string
  origin?: string
  api_base_url?: string
  synced_at?: string
  funding_transactions?: number
  entitlement_transactions?: number
  usage_cost_lines?: number
  ledger_entries?: number
  snapshot_id?: number
  currency?: string
  usage_cost_cents?: number
  expected_balance_cents?: number
  actual_balance_cents?: number | null
  balance_delta_cents?: number | null
  capabilities?: Record<string, boolean>
  diagnostics?: Record<string, string>
}

export interface LocalUsageLine {
  id: number
  account_id?: number
  account_name?: string
  account_platform?: string
  external_request_id?: string
  model: string
  currency: string
  revenue_cents: number
  input_tokens?: number
  output_tokens?: number
  started_at: string
}

export interface LocalUsageSummary {
  account_id: number
  account_name: string
  account_platform: string
  model: string
  request_count: number
  input_tokens: number
  output_tokens: number
  revenue_cents: number
  account_cost_cents: number
  original_cost_cents: number
  avg_first_token_ms: number
  avg_total_latency_ms: number
  window_start: string
  window_end: string
  last_request_created_at: string
}

export interface LocalAccountUsageSummary {
  account_id: number
  account_name: string
  account_platform: string
  request_count: number
  input_tokens: number
  output_tokens: number
  total_tokens: number
  revenue_cents: number
  account_cost_cents: number
  original_cost_cents: number
  avg_first_token_ms: number
  avg_total_latency_ms: number
  window_start: string
  window_end: string
  last_request_created_at: string
}

export interface ExtensionTask {
  id: number
  supplier_id: number
  type: 'fetch_rates' | 'fetch_groups' | 'fetch_balance' | 'fetch_announcements' | 'fetch_usage_costs' | 'fetch_health' | 'capture_supplier_session'
  schedule_key?: string
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

export type ExtensionTaskType = ExtensionTask['type']

export interface ExtensionBrowserCredential {
  supplier_id: number
  supplier_name: string
  supplier_kind: SupplierKind
  supplier_type: SupplierType
  dashboard_url: string
  api_base_url?: string
  username?: string
  password?: string
  token?: string
}

export interface ScheduledTask {
  supplier_id: number
  supplier_name: string
  task_type: ExtensionTaskType
  action: 'direct_sync' | 'extension_task' | 'compat_task'
  task_id?: number
  schedule_key: string
  created: boolean
  synced?: boolean
  total?: number
  reason?: string
}

export interface SchedulerRun {
  run_id: string
  mode: string
  dry_run: boolean
  requested_at: string
  task_types: ExtensionTaskType[]
  created_count: number
  skipped_count: number
  eligible_count: number
  items: ScheduledTask[]
}

export interface SchedulerStatus {
  enabled: boolean
  interval_seconds: number
  queue: string
}

export interface ExtensionManifestInfo {
  name: string
  version: string
  description: string
  permissions: string[]
  path: string
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

export interface NotificationDelivery {
  id: number
  channel: 'feishu'
  event_type: string
  event_id: number
  supplier_id: number
  dedupe_key: string
  status: 'sending' | 'succeeded' | 'failed'
  attempts: number
  last_error?: string
  payload?: Record<string, unknown>
  sent_at?: string | null
  created_at: string
  updated_at: string
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

export async function listSuppliers(params?: Partial<Record<'kind' | 'type' | 'runtime_status' | 'health_status' | 'q', string>> & AdminPlusPaginationParams): Promise<AdminPlusListResponse<Supplier>> {
  const { data } = await apiClient.get<AdminPlusListResponse<Supplier>>('/admin-plus/suppliers', { params })
  return data
}

export async function createSupplier(payload: CreateSupplierPayload): Promise<Supplier> {
  const { data } = await apiClient.post<Supplier>('/admin-plus/suppliers', payload)
  return data
}

export async function updateSupplier(id: number, payload: UpdateSupplierPayload): Promise<Supplier> {
  const { data } = await apiClient.put<Supplier>(`/admin-plus/suppliers/${id}`, payload)
  return data
}

export async function deleteSupplier(id: number): Promise<void> {
  await apiClient.delete(`/admin-plus/suppliers/${id}`)
}

export async function updateSupplierStatus(id: number, payload: UpdateSupplierStatusPayload): Promise<Supplier> {
  const { data } = await apiClient.patch<Supplier>(`/admin-plus/suppliers/${id}/status`, payload)
  return data
}

export async function matchSupplierSite(payload: SupplierSiteMatchRequest): Promise<SupplierSiteMatchResult> {
  const { data } = await apiClient.post<SupplierSiteMatchResult>('/admin-plus/suppliers/site-match', payload)
  return data
}

export async function getSupplierSession(id: number): Promise<SupplierBrowserSession> {
  const { data } = await apiClient.get<SupplierBrowserSession>(`/admin-plus/suppliers/${id}/session`)
  return data
}

export async function probeSupplierSession(id: number, payload?: {
  low_balance_threshold_cents?: number
  record_balance_snapshot?: boolean
}): Promise<ProbeSupplierSessionResponse> {
  const { data } = await apiClient.post<ProbeSupplierSessionResponse>(`/admin-plus/suppliers/${id}/session/probe`, payload || {})
  return data
}

export async function listSupplierChannelMonitors(id: number): Promise<SupplierChannelMonitorListResponse> {
  const { data } = await apiClient.get<SupplierChannelMonitorListResponse>(`/admin-plus/suppliers/${id}/channel-monitors`)
  return data
}

export async function getSupplierCurrentBalance(id: number, params?: { refresh?: boolean; low_balance_threshold_cents?: number }): Promise<SupplierCurrentBalance> {
  const { data } = await apiClient.get<SupplierCurrentBalance>(`/admin-plus/suppliers/${id}/balance/current`, { params })
  return data
}

export async function loginSupplierSession(id: number, payload?: LoginSupplierSessionPayload): Promise<LoginSupplierSessionResponse> {
  const { data } = await apiClient.post<LoginSupplierSessionResponse>(`/admin-plus/suppliers/${id}/session/login`, payload || {})
  return data
}

export async function upsertSupplierBrowserSession(id: number, payload: UpsertSupplierBrowserSessionPayload): Promise<SupplierBrowserSession> {
  const { data } = await apiClient.post<SupplierBrowserSession>(`/admin-plus/suppliers/${id}/browser-sessions`, payload)
  return data
}

export async function listSupplierGroups(supplierId: number, params?: { status?: SupplierGroupStatus | ''; q?: string } & AdminPlusPaginationParams): Promise<AdminPlusListResponse<SupplierGroup>> {
  const { data } = await apiClient.get<AdminPlusListResponse<SupplierGroup>>(`/admin-plus/suppliers/${supplierId}/groups`, { params })
  return data
}

export async function syncSupplierGroups(supplierId: number): Promise<SubmitProvisionJobResponse> {
  const { data } = await apiClient.post<SubmitProvisionJobResponse>(`/admin-plus/suppliers/${supplierId}/groups/sync`, {}, {
    headers: { 'Idempotency-Key': createAdminPlusIdempotencyKey('supplier-groups-sync') }
  })
  return data
}

export async function listSupplierKeys(supplierId: number, params?: { status?: SupplierKeyStatus | ''; q?: string } & AdminPlusPaginationParams): Promise<AdminPlusListResponse<SupplierKey>> {
  const { data } = await apiClient.get<AdminPlusListResponse<SupplierKey>>(`/admin-plus/suppliers/${supplierId}/keys`, { params })
  return data
}

export async function provisionSupplierKey(supplierId: number, payload: ProvisionSupplierKeyPayload): Promise<SubmitProvisionJobResponse> {
  const { data } = await apiClient.post<SubmitProvisionJobResponse>(`/admin-plus/suppliers/${supplierId}/keys/provision`, payload, {
    headers: { 'Idempotency-Key': createAdminPlusIdempotencyKey('supplier-key-provision') }
  })
  return data
}

export async function ensureSupplierKeys(supplierId: number, payload?: EnsureSupplierKeysPayload): Promise<SubmitProvisionJobResponse> {
  const { data } = await apiClient.post<SubmitProvisionJobResponse>(`/admin-plus/suppliers/${supplierId}/keys/ensure-all`, payload || {}, {
    headers: { 'Idempotency-Key': createAdminPlusIdempotencyKey('supplier-key-ensure-all') }
  })
  return data
}

export async function repairSupplierKeyBinding(supplierId: number, keyId: number, payload: RepairSupplierKeyBindingPayload): Promise<ProvisionSupplierKeyResponse> {
  const { data } = await apiClient.post<ProvisionSupplierKeyResponse>(`/admin-plus/suppliers/${supplierId}/keys/${keyId}/repair-binding`, payload, {
    headers: { 'Idempotency-Key': createAdminPlusIdempotencyKey('supplier-key-repair') }
  })
  return data
}

function createAdminPlusIdempotencyKey(prefix: string): string {
  const random = typeof crypto !== 'undefined' && 'randomUUID' in crypto
    ? crypto.randomUUID()
    : `${Date.now()}-${Math.random().toString(36).slice(2)}`
  return `${prefix}-${random}`
}

export async function getSupplierProvisionJob(jobId: number): Promise<SupplierProvisionJob> {
  const { data } = await apiClient.get<SupplierProvisionJob>(`/admin-plus/supplier-provision-jobs/${jobId}`)
  return data
}

export async function listSupplierProvisionJobs(params?: { supplier_id?: number; status?: SupplierProvisionStatus | '' } & AdminPlusPaginationParams): Promise<AdminPlusListResponse<SupplierProvisionJob>> {
  const { data } = await apiClient.get<AdminPlusListResponse<SupplierProvisionJob>>('/admin-plus/supplier-provision-jobs', { params })
  return data
}

export async function listLocalSub2APIAccounts(params?: { q?: string } & AdminPlusPaginationParams): Promise<AdminPlusListResponse<LocalSub2APIAccount>> {
  const { data } = await apiClient.get<AdminPlusListResponse<LocalSub2APIAccount>>('/admin-plus/sub2api/accounts', { params })
  return data
}

export async function listLocalAccountTestModels(accountId: number): Promise<LocalAccountTestModel[]> {
  const { data } = await apiClient.get<LocalAccountTestModel[]>(`/admin-plus/sub2api/accounts/${accountId}/models`)
  return data
}

export function localAccountTestURL(accountId: number): string {
  const baseURL = apiClient.defaults.baseURL || '/api/v1'
  return `${String(baseURL).replace(/\/+$/, '')}/admin-plus/sub2api/accounts/${accountId}/test`
}

export async function listLocalUsageLines(params?: { account_id?: number; model?: string; from?: string; to?: string } & AdminPlusPaginationParams): Promise<AdminPlusListResponse<LocalUsageLine>> {
  const { data } = await apiClient.get<AdminPlusListResponse<LocalUsageLine>>('/admin-plus/sub2api/usage-lines', { params })
  return data
}

export async function listLocalUsageSummary(params?: { account_id?: number; model?: string; from?: string; to?: string } & AdminPlusPaginationParams): Promise<AdminPlusListResponse<LocalUsageSummary>> {
  const { data } = await apiClient.get<AdminPlusListResponse<LocalUsageSummary>>('/admin-plus/sub2api/usage-summary', { params })
  return data
}

export async function listLocalAccountUsageSummary(params?: { account_id?: number; model?: string; from?: string; to?: string } & AdminPlusPaginationParams): Promise<AdminPlusListResponse<LocalAccountUsageSummary>> {
  const { data } = await apiClient.get<AdminPlusListResponse<LocalAccountUsageSummary>>('/admin-plus/sub2api/account-usage-summary', { params })
  return data
}

export async function listSupplierAccounts(supplierId: number, params?: AdminPlusPaginationParams): Promise<AdminPlusListResponse<SupplierAccount>> {
  const { data } = await apiClient.get<AdminPlusListResponse<SupplierAccount>>(`/admin-plus/suppliers/${supplierId}/accounts`, { params })
  return data
}

export async function createSupplierAccount(supplierId: number, payload: CreateSupplierAccountPayload): Promise<SupplierAccount> {
  const { data } = await apiClient.post<SupplierAccount>(`/admin-plus/suppliers/${supplierId}/accounts`, payload)
  return data
}

export async function updateSupplierAccount(supplierId: number, accountId: number, payload: UpdateSupplierAccountPayload): Promise<SupplierAccount> {
  const { data } = await apiClient.put<SupplierAccount>(`/admin-plus/suppliers/${supplierId}/accounts/${accountId}`, payload)
  return data
}

export async function deleteSupplierAccount(supplierId: number, accountId: number): Promise<void> {
  await apiClient.delete(`/admin-plus/suppliers/${supplierId}/accounts/${accountId}`)
}

export async function listRateSnapshots(params?: { supplier_id?: number; model?: string } & AdminPlusPaginationParams) {
  const { data } = await apiClient.get<AdminPlusListResponse<RateSnapshot>>('/admin-plus/rates/snapshots', { params })
  return data
}

export async function listBalanceEvents(params?: { supplier_id?: number; status?: string } & AdminPlusPaginationParams) {
  const { data } = await apiClient.get<AdminPlusListResponse<BalanceEvent>>('/admin-plus/balances/events', { params })
  return data
}

export async function recordAnnouncement(payload: {
  supplier_id: number
  source?: string
  type: AnnouncementEvent['type']
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
  const { data } = await apiClient.post<AnnouncementEvent>('/admin-plus/announcements', payload)
  return data
}

export async function syncSupplierAnnouncements(supplierId: number) {
  const { data } = await apiClient.post<{
    supplier_id: number
    system_type: string
    origin: string
    api_base_url: string
    synced_at: string
    total: number
    events: AnnouncementEvent[]
  }>(`/admin-plus/suppliers/${supplierId}/announcements/sync`)
  return data
}

export async function listAnnouncementEvents(params?: { supplier_id?: number; status?: string; recommendation?: string } & AdminPlusPaginationParams) {
  const { data } = await apiClient.get<AdminPlusListResponse<AnnouncementEvent>>('/admin-plus/announcements', { params })
  return data
}

export async function acknowledgeAnnouncementEvent(id: number): Promise<AnnouncementEvent> {
  const { data } = await apiClient.patch<AnnouncementEvent>(`/admin-plus/announcements/${id}/ack`)
  return data
}

export async function listHealthEvents(params?: { supplier_id?: number; status?: string; type?: string } & AdminPlusPaginationParams) {
  const { data } = await apiClient.get<AdminPlusListResponse<HealthEvent>>('/admin-plus/health/events', { params })
  return data
}

export async function importUsageCostLines(lines: Array<Omit<SupplierUsageCostLine, 'id' | 'created_at' | 'source'> & { source?: string }>) {
  const { data } = await apiClient.post<AdminPlusListResponse<SupplierUsageCostLine>>('/admin-plus/usage-costs/lines/import', { lines })
  return data
}

export async function syncSupplierUsageCosts(supplierId: number, payload: SyncSupplierUsageCostsPayload): Promise<SyncSupplierUsageCostsResponse> {
  const { data } = await apiClient.post<SyncSupplierUsageCostsResponse>(`/admin-plus/suppliers/${supplierId}/usage-costs/sync`, payload)
  return data
}

export async function listUsageCostLines(params?: { supplier_id?: number } & AdminPlusPaginationParams) {
  const { data } = await apiClient.get<AdminPlusListResponse<SupplierUsageCostLine>>('/admin-plus/usage-costs/lines', { params })
  return data
}

export async function syncSupplierCosts(supplierId: number, payload: SyncSupplierCostsPayload): Promise<SyncSupplierCostsResponse> {
  const { data } = await apiClient.post<SyncSupplierCostsResponse>(`/admin-plus/suppliers/${supplierId}/costs/sync`, payload)
  return data
}

export async function listSupplierCostSnapshots(params?: { supplier_id?: number } & AdminPlusPaginationParams) {
  const { data } = await apiClient.get<AdminPlusListResponse<SupplierCostSnapshot>>('/admin-plus/costs/suppliers', { params })
  return data
}

export async function getSupplierCostSummary(supplierId: number): Promise<{ items: SupplierCostSnapshot[]; total: number }> {
  const { data } = await apiClient.get<{ items: SupplierCostSnapshot[]; total: number }>(`/admin-plus/suppliers/${supplierId}/costs/summary`)
  return data
}

export async function listSupplierFundingTransactions(supplierId: number, params?: AdminPlusPaginationParams) {
  const { data } = await apiClient.get<AdminPlusListResponse<SupplierFundingTransaction>>(`/admin-plus/suppliers/${supplierId}/funding-transactions`, { params })
  return data
}

export async function listSupplierEntitlementTransactions(supplierId: number, params?: AdminPlusPaginationParams) {
  const { data } = await apiClient.get<AdminPlusListResponse<SupplierEntitlementTransaction>>(`/admin-plus/suppliers/${supplierId}/entitlement-transactions`, { params })
  return data
}

export async function listSupplierCostLedger(supplierId: number, params?: AdminPlusPaginationParams) {
  const { data } = await apiClient.get<AdminPlusListResponse<SupplierCostLedgerEntry>>(`/admin-plus/suppliers/${supplierId}/cost-ledger`, { params })
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

export async function listExtensionTasks(params?: { supplier_id?: number; status?: string; type?: string } & AdminPlusPaginationParams) {
  const { data } = await apiClient.get<AdminPlusListResponse<ExtensionTask>>('/admin-plus/extension/tasks', { params })
  return data
}

export async function getExtensionManifest(): Promise<ExtensionManifestInfo> {
  const { data } = await apiClient.get<ExtensionManifestInfo>('/admin-plus/extension/manifest')
  return data
}

export function extensionPackageURL(adminPlusOrigin?: string): string {
  const baseURL = apiClient.defaults.baseURL || '/api/v1'
  const packageURL = `${String(baseURL).replace(/\/+$/, '')}/admin-plus/extension/package.zip`
  if (!adminPlusOrigin) return packageURL
  const separator = packageURL.includes('?') ? '&' : '?'
  return `${packageURL}${separator}admin_plus_origin=${encodeURIComponent(adminPlusOrigin)}`
}

export async function downloadExtensionPackage(adminPlusOrigin?: string): Promise<Blob> {
  const { data } = await apiClient.get<Blob>('/admin-plus/extension/package.zip', {
    params: adminPlusOrigin ? { admin_plus_origin: adminPlusOrigin } : undefined,
    responseType: 'blob'
  })
  return data
}

export async function getSchedulerStatus(): Promise<SchedulerStatus> {
  const { data } = await apiClient.get<SchedulerStatus>('/admin-plus/scheduler/status')
  return data
}

export async function runScheduler(payload: {
  mode?: string
  supplier_id?: number
  task_types?: ExtensionTaskType[]
  window_minutes?: number
  dry_run?: boolean
}): Promise<SchedulerRun> {
  const { data } = await apiClient.post<SchedulerRun>('/admin-plus/scheduler/run', payload)
  return data
}

export async function claimExtensionTask(payload: { device_id: string; types?: ExtensionTask['type'][]; lease_ttl_seconds?: number }) {
  const { data } = await apiClient.post<ExtensionTask>('/admin-plus/extension/tasks/claim', payload)
  return data
}

export async function getExtensionTaskBrowserCredential(task: ExtensionTask): Promise<ExtensionBrowserCredential> {
  const { data } = await apiClient.post<ExtensionBrowserCredential>(`/admin-plus/extension/tasks/${task.id}/browser-credential`, {
    device_id: task.device_id,
    lease_token: task.lease_token
  })
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
  announcement_events?: AnnouncementEvent[]
  health_events?: HealthEvent[]
  min_profit_margin?: number
}) {
  const { data } = await apiClient.post<AdminPlusListResponse<ActionRecommendation>>('/admin-plus/actions/generate', payload)
  return data
}

export async function listActionRecommendations(params?: { supplier_id?: number; status?: string; severity?: string; type?: string } & AdminPlusPaginationParams) {
  const { data } = await apiClient.get<AdminPlusListResponse<ActionRecommendation>>('/admin-plus/actions/recommendations', { params })
  return data
}

export async function updateActionRecommendationStatus(id: number, status: ActionRecommendation['status']) {
  const { data } = await apiClient.patch<ActionRecommendation>(`/admin-plus/actions/recommendations/${id}/status`, { status })
  return data
}

export async function listNotificationDeliveries(params?: { supplier_id?: number; status?: string; channel?: string; event_type?: string } & AdminPlusPaginationParams) {
  const { data } = await apiClient.get<AdminPlusListResponse<NotificationDelivery>>('/admin-plus/notifications/deliveries', { params })
  return data
}

export const adminPlusAPI = {
  listSuppliers,
  createSupplier,
  updateSupplier,
  deleteSupplier,
  updateSupplierStatus,
  matchSupplierSite,
  getSupplierSession,
  loginSupplierSession,
  probeSupplierSession,
  listSupplierChannelMonitors,
  upsertSupplierBrowserSession,
  listSupplierGroups,
  syncSupplierGroups,
  listSupplierKeys,
  ensureSupplierKeys,
  provisionSupplierKey,
  repairSupplierKeyBinding,
  listLocalSub2APIAccounts,
  listLocalAccountTestModels,
  localAccountTestURL,
  listLocalUsageLines,
  listLocalUsageSummary,
  listSupplierAccounts,
  createSupplierAccount,
  updateSupplierAccount,
  deleteSupplierAccount,
  listRateSnapshots,
  listBalanceEvents,
  recordAnnouncement,
  syncSupplierAnnouncements,
  listAnnouncementEvents,
  acknowledgeAnnouncementEvent,
  listHealthEvents,
  importUsageCostLines,
  syncSupplierUsageCosts,
  listUsageCostLines,
  syncSupplierCosts,
  listSupplierCostSnapshots,
  getSupplierCostSummary,
  listSupplierFundingTransactions,
  listSupplierEntitlementTransactions,
  listSupplierCostLedger,
  createExtensionTask,
  listExtensionTasks,
  getExtensionManifest,
  extensionPackageURL,
  downloadExtensionPackage,
  getSchedulerStatus,
  runScheduler,
  claimExtensionTask,
  getExtensionTaskBrowserCredential,
  heartbeatExtensionTask,
  completeExtensionTask,
  failExtensionTask,
  generateActions,
  listActionRecommendations,
  updateActionRecommendationStatus,
  listNotificationDeliveries
}

export default adminPlusAPI
