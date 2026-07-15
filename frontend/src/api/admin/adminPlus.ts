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

export type BackupStorageProvider = 'cloudflare_r2' | 's3' | 'aliyun_oss'
export type BackupRecordStatus = 'pending' | 'running' | 'completed' | 'failed'

export interface BackupS3Config {
  provider: BackupStorageProvider
  endpoint: string
  region: string
  bucket: string
  access_key_id: string
  secret_access_key?: string
  secret_configured?: boolean
  prefix: string
  force_path_style: boolean
}

export interface BackupScheduleConfig {
  enabled: boolean
  cron_expr: string
  retain_days: number
  retain_count: number
}


export interface HistoryCleanupSettings {
  enabled: boolean
  retain_days: number
  cron_expr: string
  description?: string
}

export interface BackupSettings {
  s3: BackupS3Config
  schedule: BackupScheduleConfig
  cleanup: HistoryCleanupSettings
}

export interface BackupSettingsUpdatePayload {
  s3?: BackupS3Config
  schedule?: BackupScheduleConfig
  cleanup?: HistoryCleanupSettings
}

export interface BackupRecord {
  id: string
  status: BackupRecordStatus
  backup_type: string
  file_name: string
  s3_key: string
  size_bytes: number
  triggered_by: string
  error_message?: string
  started_at: string
  finished_at?: string
  expires_at?: string
  progress?: string
  restore_status?: string
  restore_error?: string
  restored_at?: string
}

export interface BackupStatus {
  storage_configured: boolean
  storage_provider: BackupStorageProvider | string
  schedule: BackupScheduleConfig
  latest_success?: BackupRecord
  latest_failure?: BackupRecord
  running?: BackupRecord
  cleanup: HistoryCleanupSettings
}

export interface ImportExportTableSummary {
  name: string
  rows: number
  sensitive?: boolean
  description?: string
}

export interface ImportExportArchiveSummary {
  tables: number
  rows: number
  items: ImportExportTableSummary[]
}

export interface ImportExportArchive {
  version: number
  product: string
  exported_at: string
  tables: Record<string, Array<Record<string, unknown>>>
  summary: ImportExportArchiveSummary
}

export interface ImportExportIgnoredTable {
  name: string
  rows: number
  reason: string
}

export interface ImportExportPreview {
  valid: boolean
  product: string
  version: number
  exported_at?: string
  summary: ImportExportArchiveSummary
  included_tables: ImportExportTableSummary[]
  ignored_tables?: ImportExportIgnoredTable[]
  warnings?: string[]
}

export interface ImportExportScopeTable {
  name: string
  sensitive?: boolean
  description?: string
  reason?: string
}

export interface ImportExportScope {
  product: string
  version: number
  included_tables: ImportExportScopeTable[]
  excluded_tables: ImportExportScopeTable[]
  notes: string[]
  summary: {
    included: number
    excluded: number
    sensitive: number
  }
}

export interface ImportExportTableResult {
  name: string
  rows: number
  imported: number
  skipped?: boolean
  reason?: string
}

export interface ImportExportResult {
  summary: ImportExportArchiveSummary
  tables: ImportExportTableResult[]
  ignored_tables?: ImportExportIgnoredTable[]
  warnings?: string[]
}


export type SupplierKind = 'source_account' | 'relay' | 'browser_only' | 'custom'
export type SupplierType = 'openai' | 'anthropic' | 'gemini' | 'sub2api' | 'new_api' | 'browser_only' | 'custom'
export type SupplierRuntimeStatus = 'monitor_only' | 'candidate' | 'active' | 'disabled'
export type SupplierHealthStatus = 'normal' | 'unavailable' | 'credential_invalid' | 'paused'
export type SupplierCapabilityStatus = 'available' | 'needs_session' | 'needs_readonly_db' | 'unsupported' | 'planned'
export type SupplierIntegrationProtocol = 'openai' | 'claude' | 'gemini'
export type SupplierGroupStatus = 'active' | 'missing' | 'disabled'
export type SupplierKeyStatus = 'provisioning' | 'bound' | 'manual_secret_required' | 'failed' | 'disabled'
export type SupplierKeyLimitPolicy = 'unknown' | 'unlimited' | 'limited' | 'unsupported'
export type SupplierGroupKeyLimitPolicy = 'inherit' | SupplierKeyLimitPolicy
export type SupplierKeyCapacityStatus = 'available' | 'limited' | 'exhausted' | 'unknown' | 'unsupported'
export type SupplierSessionSource = 'direct_login' | 'browser_extension' | 'manual_import'
export type SupplierProvisionJobType = 'sync_groups' | 'provision_group_key' | 'provision_all_group_keys' | 'repair_binding' | 'sync_supplier_costs' | 'check_supplier_channels'
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
  | 'check_supplier_channels'

export interface SupplierCredentialStatus {
  postgres_configured: boolean
  redis_configured: boolean
  browser_login_enabled: boolean
  browser_login_username_configured: boolean
  browser_login_password_configured: boolean
  browser_login_token_configured: boolean
  masked_browser_login_username?: string
}

export interface SupplierCapability {
  key: string
  label: string
  status: SupplierCapabilityStatus
  source: string
  description?: string
}

export interface SupplierIntegrationHint {
  id: string
  label: string
  provider_label: string
  protocol: string
  description?: string
  docs_url?: string
  recommended_skip_model_fetch: boolean
  recommended_models?: string[]
  source_url?: string
}

export interface SupplierPlatformHint {
  id: string
  label: string
  family: string
  source: string
  description?: string
}

export interface SupplierAPIEndpointCandidate {
  id: string
  label: string
  url: string
  protocol?: string
  source: string
  recommended: boolean
  description?: string
}

export interface SupplierURLHint {
  key: string
  label: string
  url: string
  source: string
  action: string
  severity: 'info' | 'warning' | 'success' | string
  matched_path?: string
  suggested_url?: string
  description?: string
}

export interface SupplierOperationHint {
  key: string
  label: string
  severity: 'info' | 'warning' | 'action' | string
  source: string
  description?: string
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
  capabilities?: SupplierCapability[]
  integration_hint?: SupplierIntegrationHint
  platform_hint?: SupplierPlatformHint
  api_endpoint_candidates?: SupplierAPIEndpointCandidate[]
  url_hints?: SupplierURLHint[]
  operation_hints?: SupplierOperationHint[]
  balance_cents: number
  balance_currency: string
  balance_updated_at?: string | null
  recharge_multiplier: number
  key_limit_policy: SupplierKeyLimitPolicy | string
  key_limit_value: number
  active_key_count: number
  key_capacity_status: SupplierKeyCapacityStatus | string
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

export type SupplierChannelProbeStatus =
  | 'untested'
  | 'available'
  | 'slow_first_token'
  | 'slow_total'
  | 'request_error'
  | 'remote_unavailable'
  | 'no_local_account'
  | 'probe_failed'

export interface SupplierChannelCheckSnapshot {
  id: number
  supplier_id: number
  supplier_group_id: number
  supplier_key_id?: number
  supplier_account_id?: number
  local_sub2api_account_id?: number
  external_group_id?: string
  group_name: string
  provider_family: string
  channel_monitor_id?: number
  channel_name?: string
  channel_provider?: string
  primary_model?: string
  remote_status: string
  probe_model: string
  probe_status: SupplierChannelProbeStatus
  recommended: boolean
  effective_rate_multiplier: number
  first_token_ms: number
  duration_ms: number
  status_code: number
  error_class?: string
  error_message?: string
  local_account_schedulable: boolean
  captured_at: string
  created_at: string
}

export type SupplierChannelCheckProtocol = 'openai' | 'anthropic' | 'gemini' | 'other'
export type SupplierChannelCheckOverviewMode = 'best' | 'all'

export interface SupplierChannelCheckOverviewRow {
  supplier_id: number
  supplier_name: string
  supplier_type: SupplierType
  supplier_runtime_status: SupplierRuntimeStatus
  supplier_health_status: SupplierHealthStatus
  supplier_group_id: number
  external_group_id: string
  group_name: string
  description?: string
  provider_family: string
  official_name?: string
  model_family?: string
  model_spec?: string
  protocol: SupplierChannelCheckProtocol
  effective_rate_multiplier: number
  supplier_key_id?: number
  supplier_account_id?: number
  local_sub2api_account_id?: number
  local_account_name?: string
  local_account_platform?: string
  local_account_status?: string
  local_account_schedulable: boolean
  local_account_group_ids?: number[]
  local_account_group_names?: string[]
  snapshot_id?: number
  channel_monitor_id?: number
  channel_name?: string
  channel_provider?: string
  primary_model?: string
  remote_status: string
  probe_model: string
  probe_status: SupplierChannelProbeStatus
  recommended: boolean
  first_token_ms: number
  duration_ms: number
  status_code: number
  error_class?: string
  error_message?: string
  captured_at?: string
  change_event_id?: number
  change_direction?: SupplierGroupChangeEvent['direction']
  old_effective_rate_multiplier?: number | null
  new_effective_rate_multiplier?: number
  change_percent?: number | null
  low_rate?: boolean
  changed_at?: string
}

export interface SupplierChannelCheckResult {
  supplier_id: number
  checked_at: string
  total: number
  best?: SupplierChannelCheckSnapshot
  items: SupplierChannelCheckSnapshot[]
  active_probe_budget_tokens?: number
  active_probe_estimated_tokens?: number
  active_probe_tokens_used_today?: number
  active_probes_attempted?: number
  active_probes_skipped_by_budget?: number
  active_probes_skipped_by_cooldown?: number
}

export interface ProbeSupplierChannelPayload {
  supplier_group_id?: number
  auto_pause_on_failure?: boolean
  probe_model?: string
  first_token_threshold_ms?: number
  total_latency_threshold_ms?: number
  active_probe_daily_budget_tokens?: number
  active_probe_estimated_tokens?: number
  active_probe_cooldown_seconds?: number
}

export interface SyncSupplierChannelsPayload {
  candidate_limit?: number
  auto_pause_on_failure?: boolean
  probe_model?: string
  first_token_threshold_ms?: number
  total_latency_threshold_ms?: number
  active_probe_daily_budget_tokens?: number
  active_probe_estimated_tokens?: number
  active_probe_cooldown_seconds?: number
}

export interface SetSupplierChannelSchedulingPayload {
  local_account_group_ids?: number[]
}

export type AccountRateSyncStatus = 'matched' | 'renamed' | 'not_found' | 'ambiguous' | 'failed'

export interface AccountRateSyncHistory {
  id: number
  local_sub2api_account_id: number
  local_account_name: string
  local_account_platform: string
  key_last4?: string
  supplier_id?: number
  supplier_name?: string
  supplier_type?: string
  supplier_group_id?: number
  supplier_group_name?: string
  supplier_key_id?: number
  match_source?: string
  effective_rate_multiplier: number
  target_account_name?: string
  status: AccountRateSyncStatus
  error_code?: string
  error_message?: string
  renamed: boolean
  old_account_name?: string
  new_account_name?: string
  synced_at: string
  created_at: string
}

export interface AccountRateSyncRow {
  local_sub2api_account_id: number
  local_account_name: string
  local_account_platform: string
  local_account_status?: string
  local_account_schedulable: boolean
  key_last4?: string
  history?: AccountRateSyncHistory
  status: AccountRateSyncStatus
  supplier_id?: number
  supplier_name?: string
  supplier_type?: string
  supplier_group_id?: number
  supplier_group_name?: string
  supplier_key_id?: number
  match_source?: string
  effective_rate_multiplier: number
  target_account_name?: string
  renamed: boolean
  error_code?: string
  error_message?: string
  synced_at?: string
}

export interface AccountRateSyncResult {
  items: AccountRateSyncRow[]
  total: number
  matched: number
  renamed: number
  not_found: number
  ambiguous: number
  failed: number
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
  balance_sync_error?: {
    code?: string
    message?: string
    raw_error?: string
    metadata?: Record<string, string>
  }
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
  recharge_multiplier?: number
  key_limit_policy?: SupplierKeyLimitPolicy
  key_limit_value?: number
}

export type UpdateSupplierPayload = CreateSupplierPayload

export interface UpdateSupplierStatusPayload {
  runtime_status: SupplierRuntimeStatus
  health_status: SupplierHealthStatus
}

export interface LocalSub2APIAccount {
  id: number
  name: string
  notes?: string
  platform: string
  type: string
  status: string
  error_message?: string
  schedulable: boolean
  concurrency: number
  load_factor?: number | null
  priority: number
  rate_multiplier: number
  last_used_at?: string | null
  expires_at?: string | null
  auto_pause_on_expired: boolean
  rate_limited_at?: string | null
  rate_limit_reset_at?: string | null
  overload_until?: string | null
  temp_unschedulable_until?: string | null
  temp_unschedulable_reason?: string
  session_window_start?: string | null
  session_window_end?: string | null
  session_window_status?: string
  created_at: string
  updated_at: string
  group_ids?: number[]
  group_names?: string[]
}

export interface LocalSub2APIGroup {
  id: number
  name: string
  platform?: string
  status: string
  rate_multiplier: number
  is_exclusive: boolean
  total_accounts: number
  schedulable_accounts: number
  active_api_key_count: number
  would_empty_schedulable_pool: boolean
}

export type LocalAccountOpsBalanceStatus = 'unbound' | 'usable' | 'insufficient' | 'unknown'
export type LocalAccountOpsDriftStatus =
  | 'unbound'
  | 'synced'
  | 'supplier_disabled'
  | 'binding_disabled'
  | 'missing_key'
  | 'key_local_account_mismatch'
  | 'missing_group'
  | 'group_missing'
  | 'group_disabled'
  | 'local_account_metadata_drift'
  | 'local_account_state_drift'
  | 'unknown'
export type CandidateStatus = 'available' | 'unknown' | 'degraded' | 'needs_provisioning' | 'balance_blocked' | 'blocked' | 'local_blocked' | 'capacity_blocked'

export interface LocalAccountOpsRow {
  local_sub2api_account_id: number
  local_account_name: string
  local_account_platform: string
  local_account_type: string
  local_account_status: string
  local_account_error_message?: string
  local_account_schedulable: boolean
  local_account_concurrency: number
  local_account_priority: number
  local_account_rate_multiplier: number
  local_account_rate_limited_at?: string | null
  local_account_rate_limit_reset_at?: string | null
  local_account_overload_until?: string | null
  local_account_temp_unschedulable_until?: string | null
  local_account_temp_unschedulable_reason?: string
  local_account_updated_at: string
  local_account_group_ids?: number[]
  local_account_group_names?: string[]
  local_account_proxy_id?: number
  local_account_proxy_name?: string
  local_account_proxy_status?: string
  local_account_proxy_expires_at?: string | null
  supplier_account_id?: number
  supplier_id?: number
  supplier_name?: string
  supplier_type?: SupplierType | string
  supplier_runtime_status?: SupplierRuntimeStatus | string
  supplier_health_status?: SupplierHealthStatus | string
  supplier_account_runtime_status?: SupplierRuntimeStatus | string
  supplier_account_health_status?: SupplierHealthStatus | string
  supplier_group_id?: number
  supplier_external_group_id?: string
  supplier_group_name?: string
  supplier_group_provider?: string
  supplier_group_model_family?: string
  supplier_group_model_spec?: string
  supplier_group_status?: SupplierGroupStatus | string
  effective_rate_multiplier: number
  supplier_key_id?: number
  supplier_key_name?: string
  supplier_key_last4?: string
  supplier_key_status?: SupplierKeyStatus | string
  balance_threshold_cents: number
  balance_cents: number
  balance_currency: string
  has_usable_balance: boolean
  balance_status: LocalAccountOpsBalanceStatus | string
  channel_check_status: SupplierChannelProbeStatus | string
  channel_remote_status?: string
  channel_recommended: boolean
  channel_status_code?: number
  channel_error_class?: string
  channel_error_message?: string
  last_channel_check_at?: string | null
  drift_status: LocalAccountOpsDriftStatus | string
  last_local_sync_at?: string | null
  candidate_status?: CandidateStatus | string
  blocked_reason?: string
  check_source?: string
  key_capacity_status?: string
  model_scope?: string
  model_match_status?: string
  purity_status?: string
  purity_freshness_status?: string
  purity_verdict?: string
  purity_report_id?: string
  purity_scheduler_run_id?: string
  purity_scheduler_step_id?: number
  purity_model?: string
  purity_score?: number
  purity_checked_at?: string | null
}

export type LocalAccountOpsAction = 'set_schedulable' | 'add_to_groups' | 'remove_from_groups'

export interface LocalAccountOpsGroupImpact {
  group_id: number
  group_name: string
  active_api_key_count: number
  before_schedulable_accounts: number
  after_schedulable_accounts: number
  would_empty_schedulable_pool: boolean
}

export interface LocalAccountOpsActionResult {
  action: LocalAccountOpsAction
  dry_run: boolean
  blocked: boolean
  blocked_reason?: string
  account_ids: number[]
  group_ids?: number[]
  updated_accounts: number
  added_bindings: number
  removed_bindings: number
  group_impacts?: LocalAccountOpsGroupImpact[]
  warnings?: string[]
  action_recommendation_id?: number
  action_execution_id?: number
}

export interface LocalAccountOpsActionPayload {
  action: LocalAccountOpsAction
  account_ids: number[]
  group_ids?: number[]
  schedulable?: boolean
  allow_empty_pool?: boolean
  action_id?: number
  scheduler_run_id?: string
  scheduler_step_id?: number
  reason?: string
}

export interface RoutingGroupAvailability {
  group_id: number
  group_name: string
  platform?: string
  total_accounts: number
  schedulable_accounts: number
  active_api_key_count: number
  would_empty_schedulable_pool: boolean
  recent_window_seconds?: number
  recent_success_request_count?: number
  recent_error_request_count?: number
  recent_upstream_429_count?: number
  recent_token_count?: number
  recent_last_request_at?: string | null
  recent_last_error_at?: string | null
  impacted_api_keys?: RoutingImpactedAPIKey[]
  impacted_api_keys_truncated?: boolean
  recent_failure_requests?: RoutingFailureRequest[]
  recent_failures_truncated?: boolean
}

export interface RoutingImpactedAPIKey {
  id: number
  user_id: number
  name: string
  key_preview?: string
  status: string
  last_used_at?: string | null
  recent_success_request_count?: number
  recent_error_request_count?: number
  recent_upstream_429_count?: number
  recent_token_count?: number
  recent_last_request_at?: string | null
  recent_last_error_at?: string | null
}

export interface RoutingFailureRequest {
  id: number
  request_id?: string
  api_key_id?: number
  api_key_name?: string
  api_key_preview?: string
  user_id?: number
  account_id?: number
  model?: string
  status_code?: number
  upstream_status_code?: number
  error_owner?: string
  error_type?: string
  error_message?: string
  created_at: string
}

export interface RoutingSensitiveFailureField {
  name: string
  available: boolean
  value?: string
  unavailable_reason?: string
  redacted?: boolean
  truncated?: boolean
}

export interface RoutingSensitiveFailureDetail {
  id: number
  local_group_id: number
  request_id?: string
  api_key_id?: number
  api_key_name?: string
  api_key_preview?: string
  user_id?: number
  account_id?: number
  model?: string
  status_code?: number
  upstream_status_code?: number
  error_owner?: string
  error_type?: string
  created_at: string
  available: boolean
  unavailable_reason?: string
  fields: RoutingSensitiveFailureField[]
}

export interface RoutingRefillCandidate {
  local_sub2api_account_id: number
  local_account_name?: string
  local_account_platform?: string
  supplier_id?: number
  supplier_name?: string
  supplier_group_id?: number
  supplier_group_name?: string
  supplier_key_id?: number
  candidate_status: CandidateStatus | string
  blocked_reason?: string
  check_source?: string
  model_scope?: string
  model_match_status?: string
  effective_rate_multiplier: number
}

export interface RoutingRefillPayload {
  local_group_id: number
  platform?: string
  model_scope?: string
  max_rate_multiplier?: number
  limit?: number
  dry_run?: boolean
  action_id?: number
  scheduler_run_id?: string
  scheduler_step_id?: number
  reason?: string
  trigger_type?: string
  cooldown_seconds?: number
  confirm_window_seconds?: number
}

export interface RoutingRefillResult {
  action: string
  dry_run: boolean
  local_group_id: number
  platform?: string
  model_scope?: string
  availability_before?: RoutingGroupAvailability
  availability_after?: RoutingGroupAvailability
  candidate?: RoutingRefillCandidate
  account?: {
    account_id: number
    name: string
    platform: string
    type: string
    status: string
    schedulable: boolean
    group_ids?: number[]
  }
  skipped_reason?: string
}

export interface RoutingRefillRun {
  id: number
  run_id?: string
  sub2api_instance_id: string
  local_group_id: number
  local_group_name: string
  platform?: string
  model_scope?: string
  trigger_type: string
  dry_run: boolean
  status: 'previewed' | 'succeeded' | 'skipped' | 'failed' | string
  reason?: string
  skipped_reason?: string
  before_total_accounts: number
  before_schedulable_accounts: number
  before_active_api_key_count: number
  after_total_accounts: number
  after_schedulable_accounts: number
  after_active_api_key_count: number
  selected_supplier_id?: number
  selected_supplier_group_id?: number
  selected_supplier_key_id?: number
  selected_local_account_id?: number
  selected_effective_rate_multiplier?: number
  requested_by?: number
  error_code?: string
  error_message?: string
  request_snapshot?: Record<string, unknown>
  result_snapshot?: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface LocalAccountStateSnapshot {
  name: string
  platform: string
  type: string
  schedulable: boolean
  group_ids?: number[]
}

export interface LocalAccountStateDriftSummary {
  local_sub2api_account_id: number
  account_name: string
  accepted: LocalAccountStateSnapshot
  observed: LocalAccountStateSnapshot
  drift_fields: string[]
  first_detected_at?: string
  last_checked_at: string
}

export interface LocalAccountStateSyncResult {
  checked_accounts: number
  synced_accounts: number
  drifted_accounts: number
  pending_drift_accounts: number
  items?: LocalAccountStateDriftSummary[]
}

export interface LocalAccountStateSyncPayload {
  account_ids?: number[]
  limit?: number
}

export type LocalAccountStateResolutionAction = 'accept_observed' | 'restore_accepted'

export interface LocalAccountStateResolutionPayload {
  account_ids: number[]
}

export interface LocalAccountStateResolutionResult {
  action: LocalAccountStateResolutionAction
  account_ids: number[]
  resolved_accounts: number
  restored_accounts: number
  pending_drift_accounts: number
  items?: LocalAccountStateDriftSummary[]
  warnings?: string[]
}

export interface LocalAccountRuntime {
  account_id: number
  account_name: string
  account_platform: string
  account_type: string
  status: string
  schedulable: boolean
  configured_limit: number
  current_concurrency: number
  waiting_count: number
  load_percent: number
  switch_eligible: boolean
  blocked_reason?: string
  error_message?: string
  rate_limit_reset_at?: string | null
  overload_until?: string | null
  temp_unsched_until?: string | null
  temp_unsched_reason?: string
  last_used_at?: string | null
  collected_at: string
  redis_read_configured: boolean
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

export type PurityProvider = 'openai' | 'anthropic' | 'gemini'
export type PurityCheckStatus = 'pass' | 'warn' | 'fail'
export type PurityRunStatus = 'pending' | 'running' | 'done' | string

export interface PurityScoreBreakdown {
  tag_check?: number
  structure?: number
  behavior?: number
  signature_proto?: number
  multimodal?: number
  token_audit?: number
  [key: string]: number | undefined
}

export interface PurityValidationResult {
  id: string
  name: string
  status: PurityCheckStatus
  message: string
  related_check_ids?: string[]
  details?: Record<string, unknown>
}

export interface PurityCheckResult {
  id: string
  name: string
  status: PurityCheckStatus
  score: number
  max_score: number
  message: string
  details?: Record<string, unknown>
}

export interface PurityTokenUsage {
  input_tokens: number
  output_tokens: number
  total_tokens: number
  cache_creation_tokens?: number
  cached_tokens?: number
  reasoning_tokens?: number
}

export interface PurityCheckMetrics {
  models_latency_ms?: number
  responses_latency_ms?: number
  messages_latency_ms?: number
  generate_content_latency_ms?: number
  stream_first_token_ms?: number
  stream_total_latency_ms?: number
  multimodal_latency_ms?: number
  chat_completions_latency_ms?: number
  latency_ms?: number
  tokens_per_second?: number
  usage?: PurityTokenUsage
  error_class?: string
  error_message?: string
}

export interface PurityTokenAuditSample {
  index: number
  round?: number
  input_tokens: number
  baseline_input_tokens?: number
  input_delta_pct?: number
  output_tokens: number
  baseline_output_tokens?: number
  output_delta_pct?: number
  uncached_input_tokens: number
  cache_creation_tokens: number
  cache_creation_input_tokens?: number
  cache_creation_tokens_present?: boolean
  baseline_cache_creation_input_tokens?: number
  cache_creation_delta_pct?: number
  cached_tokens: number
  cache_read_input_tokens?: number
  cached_tokens_present?: boolean
  baseline_cache_read_input_tokens?: number
  cache_read_delta_pct?: number
  reasoning_tokens?: number
  total_tokens: number
  official_baseline_usd: number
  uncached_baseline_usd?: number
  cache_discount_usd?: number
  baseline_cost?: number
  actual_cost_usd: number
  cost?: number
  cost_delta_pct?: number
  multiplier: number
  ratio?: number
  latency_ms: number
  status: PurityCheckStatus
  status_code?: number
  error_class?: string
  error_message?: string
  response_id?: string
  previous_response_id?: string
  prompt_cache_key?: string
  store?: boolean
  state_linked?: boolean
  request_mode?: string
  retried?: boolean
  history_messages?: number
  system_cache_control_blocks?: number
  message_cache_control_blocks?: number
}

export interface PurityTokenAuditReport {
  status: PurityCheckStatus
  summary: string
  price_source: string
  official_baseline_usd: number
  uncached_baseline_usd?: number
  baseline_total_cost_usd?: number
  baselineTotalCost?: number
  actual_cost_usd: number
  total_cost?: number
  totalCost?: number
  multiplier: number
  overall_ratio?: number
  overallRatio?: number
  billing_multiplier?: number | null
  billingMultiplier?: number | null
  billing_multiplier_source?: string
  billingMultiplierSource?: string
  cache_hit_rate: number
  cacheHitRate?: number
  input_tokens: number
  output_tokens: number
  cache_creation_tokens: number
  cached_tokens: number
  cached_tokens_field_observed?: boolean
  cache_creation_field_observed?: boolean
  cache_read_field_observed?: boolean
  cache_probe_rounds?: number
  cache_probe_hits?: number
  context_replay_rounds?: number
  context_replay_links?: number
  context_replay_links_expected?: number
  context_replay_ok?: boolean
  history_replay_rounds?: number
  history_replay_links?: number
  history_replay_links_expected?: number
  history_replay_ok?: boolean
  sample_count: number
  prompt_cache_key?: string
  store_enabled?: boolean
  stateful_rounds?: number
  previous_response_chain_ok?: boolean
  anomalies?: string[]
  samples: PurityTokenAuditSample[]
  rows?: PurityTokenAuditSample[]
}

export interface PurityModelIdentityResult {
  status: PurityCheckStatus
  reason?: string
  requested_model?: string
  response_model?: string
  requested_vendor?: string
  response_vendor?: string
  requested_family?: string
  response_family?: string
  version_delta?: string
  tier_delta?: string
  model_list_contains_requested?: boolean
  evidence?: Record<string, unknown>
}

export interface PurityReport {
  provider: string
  report_id: string
  access_mode?: 'web' | 'developer_api' | 'account' | string
  accessMode?: string
  billing_mode?: 'captcha_rate_limit' | 'api_key_metered' | 'account_internal' | string
  billingMode?: string
  api_base_host: string
  model_id: string
  expected_model?: string
  expectedModel?: string
  response_model?: string
  responseModel?: string
  response_model_source?: string
  responseModelSource?: string
  status?: PurityRunStatus
  step?: number
  step_name?: string
  progress?: number
  scores?: PurityScoreBreakdown
  score: number
  official_score: number
  compatibility_score: number
  verdict: string
  summary: string
  error?: string
  stream_channel?: string
  non_stream_channel?: string
  has_vertex?: boolean
  is_kiro?: boolean
  wrapper_signals?: string[]
  wrapperSignals?: string[]
  model_identity?: PurityModelIdentityResult
  modelIdentity?: PurityModelIdentityResult
  validations: PurityValidationResult[]
  checks: PurityCheckResult[]
  metrics: PurityCheckMetrics
  token_audit?: PurityTokenAuditReport
  token_audit_progress?: string
  token_audit_partial?: PurityTokenAuditSample[]
  checked_at: string
}

export interface PurityCheckEvent {
  type: 'started' | 'progress' | 'check' | 'validation' | 'metrics' | 'token_audit_sample' | 'token_audit' | 'report' | 'error'
  report_id?: string
  status?: PurityRunStatus
  step?: number
  step_name?: string
  progress?: number
  scores?: PurityScoreBreakdown
  check?: PurityCheckResult
  validation?: PurityValidationResult
  metrics?: PurityCheckMetrics
  sample?: PurityTokenAuditSample
  token_audit?: PurityTokenAuditReport
  token_audit_progress?: string
  token_audit_partial?: PurityTokenAuditSample[]
  report?: PurityReport
  error_class?: string
  error_message?: string
}

export interface LocalAccountPurityPayload {
  provider?: PurityProvider
  model_id?: string
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
  sync_provider_name?: boolean
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
  sync_provider_name?: boolean
  allow_partial?: boolean
  supplier_group_priority_ids?: number[]
  local_account_base_url?: string
  local_account_concurrency?: number
  local_account_priority?: number
  local_account_group_ids?: number[]
  runtime_status?: SupplierRuntimeStatus
  health_status?: SupplierHealthStatus
  balance_threshold_cents?: number
  balance_cents?: number
  balance_currency?: string
}

export interface EnsureSupplierKeysPlanItem {
  supplier_group_id: number
  external_group_id: string
  group_name: string
  provider_family: string
  rate_multiplier?: number
  effective_rate_multiplier?: number
  action: 'create' | 'reuse' | 'skipped_existing' | 'blocked' | string
  priority?: number
  existing_key_id?: number
  existing_key_status?: SupplierKeyStatus | string
  existing_local_sub2api_account_id?: number
  provider_external_key_id?: string
  provider_key_name?: string
  provider_key_status?: string
  group_key_limit_policy?: SupplierGroupKeyLimitPolicy | string
  group_key_limit_value?: number
  group_active_key_count?: number
  group_remaining_key_slots?: number
  blocked_reason?: string
  warnings?: string[]
}

export interface EnsureSupplierKeysPlan {
  supplier_id: number
  key_limit_policy: SupplierKeyLimitPolicy | string
  key_limit_value: number
  active_key_count: number
  remaining_key_slots: number
  total: number
  to_create: number
  to_reuse: number
  already_satisfied: number
  blocked: number
  warnings?: string[]
  items: EnsureSupplierKeysPlanItem[]
}

export interface ImportSupplierProviderKeyProjectionPayload {
  supplier_group_id: number
  external_key_id?: string
}

export interface ImportSupplierProviderKeyProjectionsPayload {
  items: ImportSupplierProviderKeyProjectionPayload[]
}

export interface ImportSupplierProviderKeyProjectionResultItem {
  supplier_group_id: number
  external_key_id?: string
  action: 'imported' | 'skipped_existing' | 'failed' | string
  key?: SupplierKey
  error_code?: string
  error_message?: string
}

export interface ImportSupplierProviderKeyProjectionsResponse {
  supplier_id: number
  total: number
  imported: number
  skipped: number
  failed: number
  items: ImportSupplierProviderKeyProjectionResultItem[]
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

export interface StandardizeSupplierKeyNameItem {
  key_id: number
  supplier_group_id: number
  external_key_id?: string
  local_name: string
  target_local_name: string
  target_provider_name?: string
  local_updated?: boolean
  provider_updated?: boolean
  action: 'updated' | 'skipped' | 'failed'
  error_code?: string
  error_message?: string
}

export interface StandardizeSupplierKeyNamesResponse {
  supplier_id: number
  sync_provider_name: boolean
  total: number
  updated: number
  skipped: number
  failed: number
  items: StandardizeSupplierKeyNameItem[]
}

export interface StandardizeSupplierKeyNamesPayload {
  sync_provider_name?: boolean
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
  manual_secret?: string
  local_account_platform?: string
  local_account_name?: string
  local_account_base_url?: string
  local_account_priority?: number
  local_account_rate_multiplier?: number | null
  local_account_group_ids?: number[]
  runtime_status?: SupplierRuntimeStatus
  health_status?: SupplierHealthStatus
  configured_concurrency?: number
  balance_threshold_cents?: number
  balance_cents?: number
  balance_currency?: string
  supplier_account_identifier?: string
  supplier_account_label?: string
}

export interface DisableSupplierKeyLocalProjectionPayload {
  reason?: string
}

export interface ProviderSupplierKeyOperationPayload {
  reason?: string
}

export interface SupplierGroup {
  id: number
  supplier_id: number
  external_group_id: string
  name: string
  description: string
  provider_family: string
  official_name?: string
  model_family?: string
  model_spec?: string
  standard_key_name?: string
  rate_multiplier: number
  user_rate_multiplier?: number | null
  effective_rate_multiplier: number
  rpm_limit?: number | null
  daily_limit_usd?: number | null
  weekly_limit_usd?: number | null
  monthly_limit_usd?: number | null
  allow_image_generation: boolean
  is_private: boolean
  key_limit_policy: SupplierGroupKeyLimitPolicy | string
  key_limit_value: number
  active_key_count: number
  key_capacity_status: SupplierKeyCapacityStatus | 'inherit' | string
  status: SupplierGroupStatus
  raw_payload?: Record<string, unknown>
  last_seen_at: string
  naming_updated_at?: string | null
  created_at: string
  updated_at: string
}

export interface SupplierGroupChangeEvent {
  id: number
  supplier_id: number
  supplier_group_id: number
  external_group_id: string
  group_name: string
  provider_family: string
  direction: 'new' | 'increase' | 'decrease'
  old_effective_rate_multiplier?: number | null
  new_effective_rate_multiplier: number
  change_percent?: number | null
  low_rate: boolean
  created_at: string
}

export interface SyncSupplierGroupsResponse {
  supplier_id: number
  system_type: string
  origin: string
  api_base_url?: string
  groups: SupplierGroup[]
  events?: SupplierGroupChangeEvent[]
  synced_at: string
  total: number
}

export interface UpdateSupplierGroupKeyCapacityPayload {
  key_limit_policy: SupplierGroupKeyLimitPolicy | string
  key_limit_value?: number
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
  recharge_actual_payment_cents: number
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

export interface SupplierCostLedgerOverviewItem {
  currency: string
  supplier_count: number
  snapshot_count: number
  actual_balance_available_count: number
  completed_funding_amount_cents: number
  completed_funding_cash_cents: number
  recharge_actual_payment_cents: number
  entitlement_amount_cents: number
  recharge_total_cents: number
  usage_cost_cents: number
  refund_amount_cents: number
  adjustment_amount_cents: number
  expected_balance_cents: number
  actual_balance_cents?: number | null
  balance_delta_cents?: number | null
  latest_captured_at?: string | null
}

export interface SupplierCostLedgerOverview {
  generated_at: string
  items: SupplierCostLedgerOverviewItem[]
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
  recharge_multiplier: number
  actual_payment_cents: number
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
  actual_payment_cents: number
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

export type CostReconcileDetailType = 'funding_credit' | 'entitlement_credit' | 'refund_debit' | 'usage_cost'

export interface CostReconcileDetailRepairResult {
  supplier_id: number
  snapshot_id: number
  currency: string
  detail_type: CostReconcileDetailType | string
  amount_cents: number
  funding_transaction?: SupplierFundingTransaction
  entitlement_transaction?: SupplierEntitlementTransaction
  usage_cost_line?: SupplierUsageCostLine
  ledger_entry?: SupplierCostLedgerEntry
  before_snapshot?: SupplierCostSnapshot
  after_snapshot?: SupplierCostSnapshot
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
  type: 'fetch_rates' | 'fetch_groups' | 'fetch_balance' | 'fetch_usage_costs' | 'reconcile_supplier_costs' | 'fetch_health' | 'check_supplier_channels' | 'capture_supplier_session' | 'register_supplier_account' | 'routing_capacity_watch'
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

export type SiteDiscoveryClassificationStatus = 'supported' | 'unknown' | 'unsupported'
export type SiteDiscoveryImportStatus = 'new' | 'imported' | 'skipped'
export type SiteDiscoveryProcessStatus = 'unprocessed' | 'added_to_catalog' | 'registered' | 'ignored'
export type SupplierRegistrationStatus =
  | 'pending'
  | 'queued'
  | 'running'
  | 'waiting_manual_verification'
  | 'succeeded'
  | 'failed'

export interface SiteDiscoverySettings {
  registration_email: string
  registration_enabled: boolean
  low_rate_threshold: number
  updated_at?: string
}

export interface SiteDiscoveryRun {
  id: number
  source_url: string
  status: 'running' | 'succeeded' | 'failed'
  total: number
  supported_total: number
  imported_total: number
  error_message?: string
  started_at: string
  finished_at?: string | null
  created_at: string
}

export interface SiteDiscoveryItem {
  id: number
  run_id: number
  source_url: string
  source_site_id: string
  source_section: string
  source_category?: string
  name: string
  register_url: string
  dashboard_url: string
  api_base_url: string
  host: string
  domain_hint?: string
  description?: string
  provider_type?: SupplierType | ''
  classification_status: SiteDiscoveryClassificationStatus
  classification_confidence: number
  classification_evidence?: string[]
  monitor_status?: string
  monitor_available?: boolean | null
  monitor_uptime_percent?: number | null
  monitor_avg_response_ms?: number | null
  monitor_latest_response_ms?: number | null
  import_status: SiteDiscoveryImportStatus
  process_status?: SiteDiscoveryProcessStatus
  catalog_site_id?: number
  supplier_id?: number
  registration_status?: SupplierRegistrationStatus | ''
  registration_task_id?: number
  registration_email?: string
  registration_error_code?: string
  registration_error_message?: string
  raw_payload?: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface SiteDiscoveryRunResult {
  run: SiteDiscoveryRun
  items: SiteDiscoveryItem[]
}

export interface SiteDiscoveryClassifyResult {
  total: number
  supported_total: number
  unknown_total: number
  items: SiteDiscoveryItem[]
}

export type SiteDiscoveryRunProgressLevel = 'info' | 'success' | 'warning' | 'error'

export interface SiteDiscoveryRunProgressEvent {
  type: 'started' | 'log' | 'item_success' | 'item_skipped' | 'item_unknown' | 'failed' | 'completed' | string
  level?: SiteDiscoveryRunProgressLevel
  message: string
  current?: number
  total?: number
  run?: SiteDiscoveryRun
  item?: SiteDiscoveryItem
  result?: SiteDiscoveryRunResult
  classify_result?: SiteDiscoveryClassifyResult
}


export interface SupplierRegistrationCredential {
  id: number
  discovery_id: number
  supplier_id: number
  email: string
  password_configured: boolean
  status: SupplierRegistrationStatus
  verification_status?: string
  extension_task_id?: number
  error_code?: string
  error_message?: string
  last_attempt_at?: string | null
  created_at: string
  updated_at: string
}

export interface SiteDiscoveryRegistrationTask {
  id: number
  discovery_id: number
  registration_id?: number
  task_id?: number
  status: SupplierRegistrationStatus
  task_status?: ExtensionTask['status'] | ''
  email?: string
  error_code?: string
  error_message?: string
  attempts?: number
  max_attempts?: number
  device_id?: string
  can_retry: boolean
  last_attempt_at?: string | null
  created_at: string
  updated_at: string
  finished_at?: string | null
  discovery: SiteDiscoveryItem
}

export interface SiteDiscoveryRegistrationTaskLogsResult {
  items: AdminPlusSystemLog[]
}

export interface RegisterSiteDiscoveryItemResponse {
  credential: SupplierRegistrationCredential
  task?: ExtensionTask | null
}

export interface SiteDiscoveryRecommendation {
  item: SiteDiscoveryItem
  min_rate_multiplier: number
  recommended_channels: number
  reason: string
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

export interface SchedulerCenterStatus {
  enabled: boolean
  worker_status: 'running' | 'paused' | 'degraded' | 'down' | string
  queue: string
  interval_seconds: number
  running_steps: number
  queued_steps: number
  failed_steps: number
  overdue_plans: number
  open_actions: number
  last_run_at?: string | null
  next_run_at?: string | null
}

export interface SchedulerPlan {
  id: string
  name: string
  task_type: string
  task_types?: ExtensionTaskType[] | string[]
  status: 'enabled' | 'paused' | 'disabled' | string
  scope: string
  frequency_label: string
  interval_seconds: number
  window_minutes: number
  misfire_policy: string
  concurrency_policy: string
  high_cost: boolean
  description: string
  last_run_at?: string | null
  last_success_at?: string | null
  issue_count: number
  last_issue_at?: string | null
  last_issue?: string
  next_run_at?: string | null
}

export interface SchedulerPlanConfig {
  status: 'enabled' | 'paused' | 'disabled'
  scope: string
  interval_seconds: number
  window_minutes: number
  misfire_policy: 'fire_once' | 'backfill' | 'skip' | string
  concurrency_policy: 'forbid' | 'allow' | string
}

export interface SchedulerRunSummary {
  id: string
  legacy_run_id?: string
  trigger_type: string
  task_type: string
  status: 'queued' | 'running' | 'succeeded' | 'partial_succeeded' | 'retryable_failed' | 'dead' | 'cancelled' | 'skipped' | string
  requested_at: string
  started_at?: string | null
  finished_at?: string | null
  supplier_count: number
  total_steps: number
  succeeded_steps: number
  failed_steps: number
  skipped_steps: number
  duration_ms: number
  error_code?: string
  error_message?: string
  request_snapshot?: Record<string, unknown>
  result_snapshot?: Record<string, unknown>
}

export interface SchedulerStepRecord {
  id: number
  run_id: string
  supplier_id: number
  supplier_name: string
  task_type: ExtensionTaskType | string
  action: string
  status: 'queued' | 'running' | 'succeeded' | 'skipped' | 'retryable_failed' | 'manual_required' | 'dead' | 'cancelled' | string
  schedule_key: string
  extension_task_id?: number
  result_count: number
  reason?: string
  attempts: number
  max_attempts: number
  next_attempt_at?: string | null
  locked_by?: string
  locked_until?: string | null
  request_snapshot?: Record<string, unknown>
  result_snapshot?: Record<string, unknown>
  started_at?: string | null
  finished_at?: string | null
  operation_logs?: SchedulerAttemptRecord[]
}

export interface SchedulerAttemptRecord {
  id: number
  step_id: number
  run_id: string
  supplier_id: number
  task_type: ExtensionTaskType | string
  status: string
  worker_id?: string
  attempt_no: number
  started_at?: string | null
  finished_at: string
  duration_ms: number
  error_code?: string
  error_message?: string
  request_snapshot?: Record<string, unknown>
  response_snapshot?: Record<string, unknown>
}

export interface SchedulerRunDetail {
  run: SchedulerRunSummary
  steps: SchedulerStepRecord[]
}

export interface SchedulerCleanupResult {
  run_id: string
  deleted_runs: number
  deleted_steps: number
  deleted_attempts: number
}

export interface SchedulerCandidateSummary {
  candidate_status: CandidateStatus | string
  blocked_reason?: string
  check_source?: string
  available_count: number
  blocked_count: number
  balance_blocked_count: number
  capacity_blocked_count: number
  unknown_count: number
  lowest_effective_rate_multiplier?: number
}

export interface SchedulerSupplierStatus {
  supplier_id: number
  supplier_name: string
  supplier_type: string
  runtime_status: string
  health_status: string
  balance_cents: number
  balance_currency: string
  completion_percent: number
  session_status: string
  balance_status: string
  group_status: string
  rate_status: string
  billing_status: string
  channel_status: string
  schedule_status: string
  last_error?: string
  recommended_action?: string
  candidate_summary?: SchedulerCandidateSummary
}

export interface SchedulerSupplierChecklistItem {
  key: string
  label: string
  status: string
  description: string
  evidence?: string
  recommended_action?: string
  last_checked_at?: string | null
}

export interface SchedulerSupplierChecklist {
  supplier_id: number
  supplier_name: string
  supplier_type: string
  completion_percent: number
  recommended_action?: string
  candidate_summary?: SchedulerCandidateSummary
  items: SchedulerSupplierChecklistItem[]
}

export interface SchedulerAction {
  id: string
  supplier_id?: number
  supplier_name?: string
  severity: 'info' | 'warning' | 'critical' | string
  status: 'open' | 'investigating' | 'ready_to_execute' | 'executing' | 'verifying' | 'resolved' | 'ignored' | string
  type: string
  title: string
  reason: string
  recommended_operation: string
  created_at: string
  updated_at?: string
  resolved_at?: string | null
}

export interface SchedulerSettings {
  enabled: boolean
  default_supplier_concurrency: number
  channel_checks_enabled: boolean
  channel_check_daily_budget_tokens: number
  channel_check_probe_cooldown_seconds: number
  first_token_slow_threshold_ms: number
  total_latency_slow_threshold_ms: number
  routing_refill_auto_execute_enabled: boolean
  routing_refill_low_capacity_threshold: number
  routing_refill_cooldown_seconds: number
  routing_refill_confirm_window_seconds: number
  routing_refill_max_rate_multiplier: number
  default_enabled_task_types: string[]
  high_cost_task_types: string[]
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
  type: 'switch_supplier' | 'pause_supplier' | 'degrade_supplier' | 'increase_weight' | 'recharge_supplier' | 'investigate_profit' | 'review_credential' | 'routing_refill' | 'local_account_schedule_disable' | 'local_account_manual_ops' | 'supplier_cost_reconcile_adjustment'
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

export interface ActionExecution {
  id: number
  recommendation_id: number
  action_type: ActionRecommendation['type']
  supplier_id: number
  target_supplier_id?: number | null
  status: 'running' | 'succeeded' | 'failed' | 'unsupported'
  request_payload?: Record<string, unknown>
  response_payload?: Record<string, unknown>
  error_message?: string
  operator_user_id?: number
  scheduler_run_id?: string
  scheduler_step_id?: number
  idempotency_key_hash?: string
  idempotency_replayed?: boolean
  before_snapshot?: Record<string, unknown>
  after_snapshot?: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface NotificationDelivery {
  id: number
  channel: 'feishu'
  event_type: string
  event_id: number
  supplier_id: number
  dedupe_key: string
  status: 'sending' | 'succeeded' | 'failed' | 'suppressed'
  attempts: number
  last_error?: string
  payload?: Record<string, unknown>
  sent_at?: string | null
  created_at: string
  updated_at: string
}

export interface NotificationRule {
  event_type: string
  label: string
  description: string
  enabled: boolean
  severity: 'info' | 'warning' | 'critical' | string
  quiet_window_minutes: number
  dedupe_scope: string
  notify_recovery: boolean
  threshold?: string
}

export interface NotificationChannelSettings {
  enabled: boolean
  webhook_url?: string
  webhook_secret?: string
  clear_webhook?: boolean
  webhook_host?: string
  webhook_configured: boolean
  secret_configured: boolean
  config_source: 'database' | 'environment' | string
  last_test_at?: string | null
  last_test_status?: string
  last_test_error?: string
}

export interface SupplierGroupNotificationSettings {
  enabled: boolean
  openai_super_low_rate_threshold: number
  openai_price_increase_rate: number
}

export interface NotificationSettings {
  feishu: NotificationChannelSettings
  rules: NotificationRule[]
  supplier_group: SupplierGroupNotificationSettings
}

export interface NotificationCenterStatus {
  feishu_configured: boolean
  feishu_enabled: boolean
  open_rules: number
  total_rules: number
  total_deliveries: number
  succeeded: number
  failed: number
  sending: number
  suppressed: number
  last_delivery_at?: string | null
}

export interface SupplierSignal {
  supplier_id: number
  name?: string
  runtime_status: SupplierRuntimeStatus
  health_status: SupplierHealthStatus
  balance_cents: number
  currency?: string
  effective_cost_cents: number
  key_limit_policy?: SupplierKeyLimitPolicy | string
  key_limit_value?: number
  active_key_count?: number
  key_capacity_status?: SupplierKeyCapacityStatus | string
}

export interface LocalGroupCapacitySignal {
  local_group_id: number
  local_group_name?: string
  platform?: string
  total_accounts?: number
  schedulable_accounts?: number
  active_api_key_count?: number
  rate_multiplier?: number
  low_capacity_threshold?: number
  best_candidate_supplier_id?: number
  best_candidate_supplier_group_id?: number
  best_candidate_local_account_id?: number
  best_candidate_rate_multiplier?: number
  best_candidate_check_source?: string
  best_candidate_supplier_name?: string
  best_candidate_supplier_group_name?: string
}

export interface LocalAccountScheduleSignal {
  supplier_id?: number
  supplier_group_id?: number
  local_sub2api_account_id: number
  local_account_name?: string
  supplier_name?: string
  supplier_group_name?: string
  local_group_ids?: number[]
  local_group_names?: string[]
  local_account_schedulable?: boolean
  candidate_status?: string
  blocked_reason?: string
  check_source?: string
  balance_status?: string
  key_capacity_status?: string
  channel_check_status?: string
  effective_rate_multiplier?: number
}

export type AdminPlusSystemLogLevel = '' | 'info' | 'warn' | 'error'
export type AdminPlusSystemLogComponent = '' | 'admin_plus.login' | 'admin_plus.balance' | 'admin_plus.registration' | 'admin_plus.extension' | 'admin_plus.sub2api'

export interface AdminPlusSystemLog {
  id: number
  created_at: string
  level: string
  component: string
  message: string
  request_id?: string
  client_request_id?: string
  user_id?: number | null
  account_id?: number | null
  platform?: string
  model?: string
  extra?: Record<string, unknown>
}

export interface ListAdminPlusSystemLogsParams extends AdminPlusPaginationParams {
  component?: AdminPlusSystemLogComponent | string
  level?: AdminPlusSystemLogLevel | string
  q?: string
  start_time?: string
  end_time?: string
}

export async function listSuppliers(params?: Partial<Record<'kind' | 'type' | 'runtime_status' | 'health_status' | 'capability_status' | 'integration_protocol' | 'q', string>> & AdminPlusPaginationParams): Promise<AdminPlusListResponse<Supplier>> {
  const { data } = await apiClient.get<AdminPlusListResponse<Supplier>>('/admin-plus/suppliers', { params })
  return data
}

export async function getSupplier(id: number): Promise<Supplier> {
  const { data } = await apiClient.get<Supplier>(`/admin-plus/suppliers/${id}`)
  return data
}


export async function listAdminPlusSystemLogs(params?: ListAdminPlusSystemLogsParams): Promise<AdminPlusListResponse<AdminPlusSystemLog>> {
  const { data } = await apiClient.get<AdminPlusListResponse<AdminPlusSystemLog>>('/admin/ops/system-logs', { params })
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

export async function listSupplierBestChannelChecks(supplierIds?: number[]): Promise<AdminPlusListResponse<SupplierChannelCheckSnapshot>> {
  const params = supplierIds && supplierIds.length > 0 ? { supplier_ids: supplierIds.join(',') } : undefined
  const { data } = await apiClient.get<AdminPlusListResponse<SupplierChannelCheckSnapshot>>('/admin-plus/supplier-channel-checks/best', { params })
  return data
}

export async function listSupplierChannelCheckOverview(params?: {
  protocol?: SupplierChannelCheckProtocol
  mode?: SupplierChannelCheckOverviewMode
  supplier_ids?: number[]
}): Promise<AdminPlusListResponse<SupplierChannelCheckOverviewRow>> {
  const query = {
    protocol: params?.protocol,
    mode: params?.mode,
    supplier_ids: params?.supplier_ids && params.supplier_ids.length > 0 ? params.supplier_ids.join(',') : undefined
  }
  const { data } = await apiClient.get<AdminPlusListResponse<SupplierChannelCheckOverviewRow>>('/admin-plus/supplier-channel-checks/overview', { params: query })
  return data
}

export async function listSupplierChannelChecks(supplierId: number, params?: AdminPlusPaginationParams): Promise<AdminPlusListResponse<SupplierChannelCheckSnapshot>> {
  const { data } = await apiClient.get<AdminPlusListResponse<SupplierChannelCheckSnapshot>>(`/admin-plus/suppliers/${supplierId}/channel-checks`, { params })
  return data
}

export async function probeSupplierChannel(supplierId: number, payload?: ProbeSupplierChannelPayload): Promise<SupplierChannelCheckResult> {
  const { data } = await apiClient.post<SupplierChannelCheckResult>(`/admin-plus/suppliers/${supplierId}/channel-checks/probe`, payload || {})
  return data
}

export async function syncSupplierChannelChecks(supplierId: number, payload?: SyncSupplierChannelsPayload): Promise<SubmitProvisionJobResponse> {
  const { data } = await apiClient.post<SubmitProvisionJobResponse>(`/admin-plus/suppliers/${supplierId}/channel-checks/sync`, payload || {}, {
    headers: { 'Idempotency-Key': createAdminPlusIdempotencyKey('supplier-channel-checks-sync') }
  })
  return data
}

export async function enableSupplierChannelScheduling(supplierId: number, supplierGroupId: number, payload?: SetSupplierChannelSchedulingPayload): Promise<SupplierChannelCheckSnapshot> {
  const { data } = await apiClient.post<SupplierChannelCheckSnapshot>(`/admin-plus/suppliers/${supplierId}/channel-checks/scheduling/enable`, {
    supplier_group_id: supplierGroupId,
    local_account_group_ids: payload?.local_account_group_ids
  })
  return data
}

export async function pauseSupplierChannelScheduling(supplierId: number, supplierGroupId: number, payload?: SetSupplierChannelSchedulingPayload): Promise<SupplierChannelCheckSnapshot> {
  const { data } = await apiClient.post<SupplierChannelCheckSnapshot>(`/admin-plus/suppliers/${supplierId}/channel-checks/scheduling/pause`, {
    supplier_group_id: supplierGroupId,
    local_account_group_ids: payload?.local_account_group_ids
  })
  return data
}

export async function listAccountRateSyncRows(params?: { protocol?: SupplierChannelCheckProtocol; limit?: number }): Promise<AdminPlusListResponse<AccountRateSyncRow>> {
  const { data } = await apiClient.get<AdminPlusListResponse<AccountRateSyncRow>>('/admin-plus/account-rate-sync/accounts', { params })
  return data
}

export async function syncAccountRateRows(payload?: { protocol?: SupplierChannelCheckProtocol; limit?: number }): Promise<AccountRateSyncResult> {
  const { data } = await apiClient.post<AccountRateSyncResult>('/admin-plus/account-rate-sync/sync', payload || {})
  return data
}

export async function retryAccountRateSyncRow(accountId: number): Promise<AccountRateSyncRow> {
  const { data } = await apiClient.post<AccountRateSyncRow>(`/admin-plus/account-rate-sync/accounts/${accountId}/retry`)
  return data
}

export async function renameAccountRateSyncRow(historyId: number): Promise<AccountRateSyncRow> {
  const { data } = await apiClient.post<AccountRateSyncRow>(`/admin-plus/account-rate-sync/history/${historyId}/rename`)
  return data
}

export async function renameMatchedAccountRateSyncRows(payload?: { protocol?: SupplierChannelCheckProtocol; limit?: number }): Promise<AccountRateSyncResult> {
  const { data } = await apiClient.post<AccountRateSyncResult>('/admin-plus/account-rate-sync/rename-matched', payload || {})
  return data
}

export async function clearAccountRateSyncHistory(): Promise<{ deleted: number }> {
  const { data } = await apiClient.delete<{ deleted: number }>('/admin-plus/account-rate-sync/history')
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

export async function listAllSupplierGroups(params?: { supplier_id?: number; status?: SupplierGroupStatus | ''; q?: string } & AdminPlusPaginationParams): Promise<AdminPlusListResponse<SupplierGroup>> {
  const { data } = await apiClient.get<AdminPlusListResponse<SupplierGroup>>('/admin-plus/supplier-groups', { params })
  return data
}

export async function listSupplierGroupChangeEvents(supplierId: number, params?: { direction?: SupplierGroupChangeEvent['direction'] | ''; low_rate?: boolean } & AdminPlusPaginationParams): Promise<AdminPlusListResponse<SupplierGroupChangeEvent>> {
  const { data } = await apiClient.get<AdminPlusListResponse<SupplierGroupChangeEvent>>(`/admin-plus/suppliers/${supplierId}/groups/events`, { params })
  return data
}

export async function syncSupplierGroups(supplierId: number): Promise<SubmitProvisionJobResponse> {
  const { data } = await apiClient.post<SubmitProvisionJobResponse>(`/admin-plus/suppliers/${supplierId}/groups/sync`, {}, {
    headers: { 'Idempotency-Key': createAdminPlusIdempotencyKey('supplier-groups-sync') }
  })
  return data
}

export async function updateSupplierGroupKeyCapacity(supplierId: number, groupId: number, payload: UpdateSupplierGroupKeyCapacityPayload): Promise<SupplierGroup> {
  const { data } = await apiClient.put<SupplierGroup>(`/admin-plus/suppliers/${supplierId}/groups/${groupId}/key-capacity`, payload)
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

export async function planEnsureSupplierKeys(supplierId: number, payload?: EnsureSupplierKeysPayload): Promise<EnsureSupplierKeysPlan> {
  const { data } = await apiClient.post<EnsureSupplierKeysPlan>(`/admin-plus/suppliers/${supplierId}/keys/ensure-all-plan`, payload || {})
  return data
}

export async function importSupplierProviderKeyProjection(supplierId: number, payload: ImportSupplierProviderKeyProjectionPayload): Promise<SupplierKey> {
  const { data } = await apiClient.post<SupplierKey>(`/admin-plus/suppliers/${supplierId}/keys/import-provider-projection`, payload, {
    headers: { 'Idempotency-Key': createAdminPlusIdempotencyKey('supplier-key-import-provider-projection') }
  })
  return data
}

export async function importSupplierProviderKeyProjections(supplierId: number, payload: ImportSupplierProviderKeyProjectionsPayload): Promise<ImportSupplierProviderKeyProjectionsResponse> {
  const { data } = await apiClient.post<ImportSupplierProviderKeyProjectionsResponse>(`/admin-plus/suppliers/${supplierId}/keys/import-provider-projections`, payload, {
    headers: { 'Idempotency-Key': createAdminPlusIdempotencyKey('supplier-key-import-provider-projections') }
  })
  return data
}

export async function standardizeSupplierKeyNames(supplierId: number, payload?: StandardizeSupplierKeyNamesPayload): Promise<StandardizeSupplierKeyNamesResponse> {
  const { data } = await apiClient.post<StandardizeSupplierKeyNamesResponse>(`/admin-plus/suppliers/${supplierId}/keys/standardize-names`, payload || {}, {
    headers: { 'Idempotency-Key': createAdminPlusIdempotencyKey('supplier-key-standardize-names') }
  })
  return data
}

export async function repairSupplierKeyBinding(supplierId: number, keyId: number, payload: RepairSupplierKeyBindingPayload): Promise<ProvisionSupplierKeyResponse> {
  const { data } = await apiClient.post<ProvisionSupplierKeyResponse>(`/admin-plus/suppliers/${supplierId}/keys/${keyId}/repair-binding`, payload, {
    headers: { 'Idempotency-Key': createAdminPlusIdempotencyKey('supplier-key-repair') }
  })
  return data
}

export async function disableSupplierKeyLocalProjection(supplierId: number, keyId: number, payload?: DisableSupplierKeyLocalProjectionPayload): Promise<SupplierKey> {
  const { data } = await apiClient.post<SupplierKey>(`/admin-plus/suppliers/${supplierId}/keys/${keyId}/disable-local-projection`, payload || {}, {
    headers: { 'Idempotency-Key': createAdminPlusIdempotencyKey('supplier-key-disable-local-projection') }
  })
  return data
}

export async function disableSupplierProviderKey(supplierId: number, keyId: number, payload?: ProviderSupplierKeyOperationPayload): Promise<SupplierKey> {
  const { data } = await apiClient.post<SupplierKey>(`/admin-plus/suppliers/${supplierId}/keys/${keyId}/disable-provider`, payload || {}, {
    headers: { 'Idempotency-Key': createAdminPlusIdempotencyKey('supplier-key-disable-provider') }
  })
  return data
}

export async function deleteSupplierProviderKey(supplierId: number, keyId: number, payload?: ProviderSupplierKeyOperationPayload): Promise<SupplierKey> {
  const { data } = await apiClient.post<SupplierKey>(`/admin-plus/suppliers/${supplierId}/keys/${keyId}/delete-provider`, payload || {}, {
    headers: { 'Idempotency-Key': createAdminPlusIdempotencyKey('supplier-key-delete-provider') }
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

export async function listLocalSub2APIGroups(params?: AdminPlusPaginationParams): Promise<AdminPlusListResponse<LocalSub2APIGroup>> {
  const { data } = await apiClient.get<AdminPlusListResponse<LocalSub2APIGroup>>('/admin-plus/sub2api/groups', { params })
  return data
}

export async function listLocalAccountOps(params?: {
  q?: string
  supplier_id?: number
  local_group_id?: number
  supplier_group_id?: number
  model_scope?: string
  max_rate_multiplier?: number
  balance_status?: LocalAccountOpsBalanceStatus | string
  channel_check_status?: SupplierChannelProbeStatus | string
  schedulable?: boolean
} & AdminPlusPaginationParams): Promise<AdminPlusListResponse<LocalAccountOpsRow>> {
  const { data } = await apiClient.get<AdminPlusListResponse<LocalAccountOpsRow>>('/admin-plus/sub2api/local-account-ops', { params })
  return data
}

export async function previewLocalAccountOpsAction(payload: LocalAccountOpsActionPayload): Promise<LocalAccountOpsActionResult> {
  const { data } = await apiClient.post<LocalAccountOpsActionResult>('/admin-plus/sub2api/local-account-ops/preview', payload)
  return data
}

export async function applyLocalAccountOpsAction(payload: LocalAccountOpsActionPayload): Promise<LocalAccountOpsActionResult> {
  const { data } = await apiClient.post<LocalAccountOpsActionResult>('/admin-plus/sub2api/local-account-ops/apply', payload, {
    headers: { 'Idempotency-Key': createAdminPlusIdempotencyKey('local-account-ops-apply') }
  })
  return data
}

export async function refillLocalGroup(payload: RoutingRefillPayload): Promise<RoutingRefillResult> {
  const { data } = await apiClient.post<RoutingRefillResult>('/admin-plus/sub2api/routing/refill-local-group', payload, {
    headers: payload.dry_run ? undefined : { 'Idempotency-Key': createAdminPlusIdempotencyKey('routing-refill-local-group') }
  })
  return data
}

export async function listRoutingRefillRuns(params?: { local_group_id?: number; status?: string } & AdminPlusPaginationParams): Promise<AdminPlusListResponse<RoutingRefillRun>> {
  const { data } = await apiClient.get<AdminPlusListResponse<RoutingRefillRun>>('/admin-plus/sub2api/routing/refill-runs', { params })
  return data
}

export async function listRoutingImpactAPIKeys(params?: { local_group_id?: number } & AdminPlusPaginationParams): Promise<AdminPlusListResponse<RoutingImpactedAPIKey>> {
  const { data } = await apiClient.get<AdminPlusListResponse<RoutingImpactedAPIKey>>('/admin-plus/sub2api/routing/group-impact/api-keys', { params })
  return data
}

export async function listRoutingImpactFailureRequests(params?: { local_group_id?: number } & AdminPlusPaginationParams): Promise<AdminPlusListResponse<RoutingFailureRequest>> {
  const { data } = await apiClient.get<AdminPlusListResponse<RoutingFailureRequest>>('/admin-plus/sub2api/routing/group-impact/failures', { params })
  return data
}

export async function getRoutingFailureSensitiveDetail(id: number, payload: { local_group_id: number; reason: string; fields?: string[] }): Promise<RoutingSensitiveFailureDetail> {
  const { data } = await apiClient.post<RoutingSensitiveFailureDetail>(`/admin-plus/sub2api/routing/group-impact/failures/${id}/sensitive-detail`, payload)
  return data
}

export async function syncLocalAccountState(payload: LocalAccountStateSyncPayload = {}): Promise<LocalAccountStateSyncResult> {
  const { data } = await apiClient.post<LocalAccountStateSyncResult>('/admin-plus/sub2api/local-account-ops/sync-local-state', payload)
  return data
}

export async function acceptLocalAccountState(payload: LocalAccountStateResolutionPayload): Promise<LocalAccountStateResolutionResult> {
  const { data } = await apiClient.post<LocalAccountStateResolutionResult>('/admin-plus/sub2api/local-account-ops/accept-local-state', payload, {
    headers: { 'Idempotency-Key': createAdminPlusIdempotencyKey('local-account-state-accept') }
  })
  return data
}

export async function restoreLocalAccountState(payload: LocalAccountStateResolutionPayload): Promise<LocalAccountStateResolutionResult> {
  const { data } = await apiClient.post<LocalAccountStateResolutionResult>('/admin-plus/sub2api/local-account-ops/restore-local-state', payload, {
    headers: { 'Idempotency-Key': createAdminPlusIdempotencyKey('local-account-state-restore') }
  })
  return data
}

export async function listLocalAccountRuntime(params?: { account_id?: number; q?: string } & AdminPlusPaginationParams): Promise<AdminPlusListResponse<LocalAccountRuntime>> {
  const { data } = await apiClient.get<AdminPlusListResponse<LocalAccountRuntime>>('/admin-plus/sub2api/account-runtime', { params })
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

export function localAccountPurityStreamURL(accountId: number): string {
  const baseURL = apiClient.defaults.baseURL || '/api/v1'
  return `${String(baseURL).replace(/\/+$/, '')}/admin-plus/proxyai/accounts/${accountId}/purity/checks/stream`
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

export async function backfillSupplierCosts(payload: SyncSupplierCostsPayload & { supplier_id?: number }): Promise<SchedulerRunSummary> {
  const { data } = await apiClient.post<SchedulerRunSummary>('/admin-plus/costs/backfill-history', payload)
  return data
}

export async function listSupplierCostSnapshots(params?: { supplier_id?: number } & AdminPlusPaginationParams) {
  const { data } = await apiClient.get<AdminPlusListResponse<SupplierCostSnapshot>>('/admin-plus/costs/suppliers', { params })
  return data
}

export async function getSupplierCostLedgerOverview(): Promise<SupplierCostLedgerOverview> {
  const { data } = await apiClient.get<SupplierCostLedgerOverview>('/admin-plus/costs/ledger-overview')
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

export async function applyCostReconcileAdjustment(recommendationId: number, payload: {
  snapshot_id?: number
  adjustment_amount_cents?: number
  reason?: string
  occurred_at?: string
  scheduler_run_id?: string
  scheduler_step_id?: number
}): Promise<{
  supplier_id: number
  snapshot_id: number
  currency: string
  adjustment_amount_cents: number
  ledger_entry?: SupplierCostLedgerEntry
  before_snapshot?: SupplierCostSnapshot
  after_snapshot?: SupplierCostSnapshot
}> {
  const { data } = await apiClient.post(`/admin-plus/actions/recommendations/${recommendationId}/cost-reconcile-adjustment`, payload, {
    headers: { 'Idempotency-Key': createAdminPlusIdempotencyKey('cost-reconcile-adjustment') }
  })
  return data
}

export async function applyCostReconcileDetailRepair(recommendationId: number, payload: {
  snapshot_id?: number
  detail_type: CostReconcileDetailType | string
  amount_cents?: number
  external_id?: string
  model?: string
  reason?: string
  occurred_at?: string
  scheduler_run_id?: string
  scheduler_step_id?: number
}): Promise<CostReconcileDetailRepairResult> {
  const { data } = await apiClient.post<CostReconcileDetailRepairResult>(`/admin-plus/actions/recommendations/${recommendationId}/cost-reconcile-detail-repair`, payload, {
    headers: { 'Idempotency-Key': createAdminPlusIdempotencyKey('cost-reconcile-detail-repair') }
  })
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

export async function getSchedulerCenterStatus(): Promise<SchedulerCenterStatus> {
  const { data } = await apiClient.get<SchedulerCenterStatus>('/admin-plus/scheduler/center/status')
  return data
}

export async function listSchedulerPlans(): Promise<SchedulerPlan[]> {
  const { data } = await apiClient.get<SchedulerPlan[]>('/admin-plus/scheduler/plans')
  return data
}

export async function updateSchedulerPlanStatus(id: string, status: 'enabled' | 'paused' | 'disabled'): Promise<SchedulerPlan> {
  const { data } = await apiClient.patch<SchedulerPlan>(`/admin-plus/scheduler/plans/${id}/status`, { status })
  return data
}

export async function updateSchedulerPlanConfig(id: string, payload: SchedulerPlanConfig): Promise<SchedulerPlan> {
  const { data } = await apiClient.put<SchedulerPlan>(`/admin-plus/scheduler/plans/${id}`, payload)
  return data
}

export async function createSchedulerRun(payload: {
  mode?: string
  supplier_id?: number
  task_types?: ExtensionTaskType[]
  window_minutes?: number
  dry_run?: boolean
}): Promise<SchedulerRunSummary> {
  const { data } = await apiClient.post<SchedulerRunSummary>('/admin-plus/scheduler/runs', payload)
  return data
}

export async function listSchedulerRuns(params?: { limit?: number; offset?: number; task_type?: string }): Promise<SchedulerRunSummary[]> {
  const { data } = await apiClient.get<SchedulerRunSummary[]>('/admin-plus/scheduler/runs', { params })
  return data
}

export async function getSchedulerRunDetail(id: string): Promise<SchedulerRunDetail> {
  const { data } = await apiClient.get<SchedulerRunDetail>(`/admin-plus/scheduler/runs/${id}`)
  return data
}

export async function listSchedulerSteps(params?: { run_id?: string; limit?: number; offset?: number }): Promise<SchedulerStepRecord[]> {
  const { data } = await apiClient.get<SchedulerStepRecord[]>('/admin-plus/scheduler/steps', { params })
  return data
}

export async function deleteSchedulerRun(id: string): Promise<SchedulerCleanupResult> {
  const { data } = await apiClient.delete<SchedulerCleanupResult>(`/admin-plus/scheduler/runs/${id}`)
  return data
}

export async function deleteSchedulerRuns(params?: { task_type?: string }): Promise<SchedulerCleanupResult> {
  const { data } = await apiClient.delete<SchedulerCleanupResult>('/admin-plus/scheduler/runs', { params })
  return data
}

export async function cancelSchedulerRun(id: string): Promise<SchedulerRunSummary> {
  const { data } = await apiClient.post<SchedulerRunSummary>(`/admin-plus/scheduler/runs/${id}/cancel`)
  return data
}

export async function retrySchedulerRunFailedSteps(id: string): Promise<SchedulerRunDetail> {
  const { data } = await apiClient.post<SchedulerRunDetail>(`/admin-plus/scheduler/runs/${id}/retry-failed`)
  return data
}

export async function retrySchedulerStep(id: number): Promise<SchedulerStepRecord> {
  const { data } = await apiClient.post<SchedulerStepRecord>(`/admin-plus/scheduler/steps/${id}/retry`)
  return data
}

export async function cancelSchedulerStep(id: number): Promise<SchedulerStepRecord> {
  const { data } = await apiClient.post<SchedulerStepRecord>(`/admin-plus/scheduler/steps/${id}/cancel`)
  return data
}

export async function listSchedulerSupplierStatuses(): Promise<SchedulerSupplierStatus[]> {
  const { data } = await apiClient.get<SchedulerSupplierStatus[]>('/admin-plus/scheduler/suppliers/status')
  return data
}

export async function getSchedulerSupplierChecklist(id: number): Promise<SchedulerSupplierChecklist> {
  const { data } = await apiClient.get<SchedulerSupplierChecklist>(`/admin-plus/scheduler/suppliers/${id}/checklist`)
  return data
}

export async function listSchedulerActions(): Promise<SchedulerAction[]> {
  const { data } = await apiClient.get<SchedulerAction[]>('/admin-plus/scheduler/actions')
  return data
}

export async function updateSchedulerActionStatus(id: string, status: 'resolved' | 'ignored' | 'investigating'): Promise<SchedulerAction> {
  const { data } = await apiClient.patch<SchedulerAction>(`/admin-plus/scheduler/actions/${id}/status`, { status })
  return data
}

export async function getSchedulerSettings(): Promise<SchedulerSettings> {
  const { data } = await apiClient.get<SchedulerSettings>('/admin-plus/scheduler/settings')
  return data
}

export async function updateSchedulerSettings(payload: SchedulerSettings): Promise<SchedulerSettings> {
  const { data } = await apiClient.put<SchedulerSettings>('/admin-plus/scheduler/settings', payload)
  return data
}

export async function runScheduler(payload: {
  mode?: string
  supplier_id?: number
  task_types?: ExtensionTaskType[]
  window_minutes?: number
  dry_run?: boolean
}): Promise<SchedulerRun | SchedulerRunSummary> {
  const { data } = await apiClient.post<SchedulerRun | SchedulerRunSummary>('/admin-plus/scheduler/run', payload)
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

export async function getSiteDiscoverySettings(): Promise<SiteDiscoverySettings> {
  const { data } = await apiClient.get<SiteDiscoverySettings>('/admin-plus/site-discovery/settings')
  return data
}

export async function updateSiteDiscoverySettings(payload: SiteDiscoverySettings): Promise<SiteDiscoverySettings> {
  const { data } = await apiClient.put<SiteDiscoverySettings>('/admin-plus/site-discovery/settings', payload)
  return data
}

export async function runSiteDiscovery(payload?: {
  source_url?: string
  probe_interfaces?: boolean
  probe_sites?: boolean
  limit?: number
}): Promise<SiteDiscoveryRunResult> {
  const { data } = await apiClient.post<SiteDiscoveryRunResult>('/admin-plus/site-discovery/runs', payload || {})
  return data
}

export async function runSiteDiscoveryStream(
  payload: {
    source_url?: string
    probe_interfaces?: boolean
    probe_sites?: boolean
    limit?: number
  },
  onEvent: (event: SiteDiscoveryRunProgressEvent) => void
): Promise<void> {
  const baseURL = String(apiClient.defaults.baseURL || import.meta.env.VITE_API_BASE_URL || '/api/v1').replace(/\/+$/, '')
  const token = localStorage.getItem('auth_token')
  const response = await fetch(`${baseURL}/admin-plus/site-discovery/runs/stream`, {
    method: 'POST',
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {})
    },
    body: JSON.stringify(payload || {})
  })
  if (!response.ok || !response.body) {
    throw new Error(`采集启动失败：HTTP ${response.status}`)
  }
  const reader = response.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''
  while (true) {
    const { done, value } = await reader.read()
    if (done) break
    buffer += decoder.decode(value, { stream: true })
    const lines = buffer.split('\n')
    buffer = lines.pop() || ''
    for (const line of lines) {
      const text = line.trim()
      if (!text) continue
      onEvent(JSON.parse(text) as SiteDiscoveryRunProgressEvent)
    }
  }
  if (buffer.trim()) {
    onEvent(JSON.parse(buffer.trim()) as SiteDiscoveryRunProgressEvent)
  }
}

export async function classifySiteDiscoveryItemsStream(
  payload: {
    q?: string
    provider_type?: 'new_api' | 'sub2api' | ''
    classification_status?: SiteDiscoveryClassificationStatus | ''
    import_status?: SiteDiscoveryImportStatus | ''
    registration_status?: SupplierRegistrationStatus | ''
    processed_status?: 'processed' | 'unprocessed' | ''
    probe_interfaces?: boolean
    probe_sites?: boolean
    limit?: number
  },
  onEvent: (event: SiteDiscoveryRunProgressEvent) => void
): Promise<void> {
  const baseURL = String(apiClient.defaults.baseURL || import.meta.env.VITE_API_BASE_URL || '/api/v1').replace(/\/+$/, '')
  const token = localStorage.getItem('auth_token')
  const response = await fetch(`${baseURL}/admin-plus/site-discovery/items/classify/stream`, {
    method: 'POST',
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {})
    },
    body: JSON.stringify(payload || {})
  })
  if (!response.ok || !response.body) {
    throw new Error(`批量识别启动失败：HTTP ${response.status}`)
  }
  const reader = response.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''
  while (true) {
    const { done, value } = await reader.read()
    if (done) break
    buffer += decoder.decode(value, { stream: true })
    const lines = buffer.split('\n')
    buffer = lines.pop() || ''
    for (const line of lines) {
      const text = line.trim()
      if (!text) continue
      onEvent(JSON.parse(text) as SiteDiscoveryRunProgressEvent)
    }
  }
  if (buffer.trim()) {
    onEvent(JSON.parse(buffer.trim()) as SiteDiscoveryRunProgressEvent)
  }
}

export async function listSiteDiscoveryItems(params?: {
  q?: string
  provider_type?: 'new_api' | 'sub2api' | ''
  classification_status?: SiteDiscoveryClassificationStatus | ''
  import_status?: SiteDiscoveryImportStatus | ''
  registration_status?: SupplierRegistrationStatus | ''
  processed_status?: 'processed' | 'unprocessed' | ''
} & AdminPlusPaginationParams): Promise<AdminPlusListResponse<SiteDiscoveryItem>> {
  const { data } = await apiClient.get<AdminPlusListResponse<SiteDiscoveryItem>>('/admin-plus/site-discovery/items', { params })
  return data
}

export async function importSiteDiscoveryItem(id: number): Promise<SiteDiscoveryItem> {
  const { data } = await apiClient.post<SiteDiscoveryItem>(`/admin-plus/site-discovery/items/${id}/import`)
  return data
}

export async function registerSiteDiscoveryItem(id: number): Promise<RegisterSiteDiscoveryItemResponse> {
  const { data } = await apiClient.post<RegisterSiteDiscoveryItemResponse>(`/admin-plus/site-discovery/items/${id}/register`)
  return data
}

export async function listSiteDiscoveryRegistrationTasks(params?: {
  q?: string
  provider_type?: 'new_api' | 'sub2api' | ''
  registration_status?: SupplierRegistrationStatus | ''
} & AdminPlusPaginationParams): Promise<AdminPlusListResponse<SiteDiscoveryRegistrationTask>> {
  const { data } = await apiClient.get<AdminPlusListResponse<SiteDiscoveryRegistrationTask>>('/admin-plus/site-discovery/registrations', { params })
  return data
}

export async function rerunSiteDiscoveryRegistration(id: number): Promise<RegisterSiteDiscoveryItemResponse> {
  const { data } = await apiClient.post<RegisterSiteDiscoveryItemResponse>(`/admin-plus/site-discovery/registrations/${id}/rerun`)
  return data
}

export async function listSiteDiscoveryRegistrationLogs(id: number, params?: { limit?: number }): Promise<SiteDiscoveryRegistrationTaskLogsResult> {
  const { data } = await apiClient.get<SiteDiscoveryRegistrationTaskLogsResult>(`/admin-plus/site-discovery/registrations/${id}/logs`, { params })
  return data
}

export async function listSiteDiscoveryRecommendations(params?: { limit?: number }): Promise<{ items: SiteDiscoveryRecommendation[] }> {
  const { data } = await apiClient.get<{ items: SiteDiscoveryRecommendation[] }>('/admin-plus/site-discovery/recommendations', { params })
  return data
}


export async function generateActions(payload: {
  suppliers: SupplierSignal[]
  candidate_evaluations?: Array<{
    supplier_id?: number
    supplier_group_id?: number
    local_sub2api_account_id?: number
    candidate_status?: string
    blocked_reason?: string
    check_source?: string
    balance_status?: string
    key_capacity_status?: string
    purity_freshness_status?: string
    effective_rate_multiplier?: number
  }>
  local_group_capacity?: LocalGroupCapacitySignal[]
  local_account_schedule?: LocalAccountScheduleSignal[]
  balance_events?: BalanceEvent[]
  health_events?: HealthEvent[]
  cost_snapshots?: SupplierCostSnapshot[]
  min_profit_margin?: number
}) {
  const { data } = await apiClient.post<AdminPlusListResponse<ActionRecommendation>>('/admin-plus/actions/generate', payload)
  return data
}

export async function listActionRecommendations(params?: {
  recommendation_id?: number
  supplier_id?: number
  status?: string
  severity?: string
  type?: string
  signal?: string
  local_group_id?: number
  local_sub2api_account_id?: number
} & AdminPlusPaginationParams) {
  const { data } = await apiClient.get<AdminPlusListResponse<ActionRecommendation>>('/admin-plus/actions/recommendations', { params })
  return data
}

export async function updateActionRecommendationStatus(id: number, status: ActionRecommendation['status']) {
  const { data } = await apiClient.patch<ActionRecommendation>(`/admin-plus/actions/recommendations/${id}/status`, { status })
  return data
}

export async function executeActionRecommendation(id: number, payload?: { operator_user_id?: number; scheduler_run_id?: string; scheduler_step_id?: number; request_payload?: Record<string, unknown> }): Promise<ActionExecution> {
  const { data } = await apiClient.post<ActionExecution>(`/admin-plus/actions/recommendations/${id}/execute`, payload || {})
  return data
}

export async function listActionExecutions(id: number, params?: AdminPlusPaginationParams): Promise<AdminPlusListResponse<ActionExecution>> {
  const { data } = await apiClient.get<AdminPlusListResponse<ActionExecution>>(`/admin-plus/actions/recommendations/${id}/executions`, { params })
  return data
}

export async function retryActionExecution(recommendationId: number, executionId: number): Promise<ActionExecution> {
  const { data } = await apiClient.post<ActionExecution>(`/admin-plus/actions/recommendations/${recommendationId}/executions/${executionId}/retry`, {}, {
    headers: { 'Idempotency-Key': createAdminPlusIdempotencyKey('action-execution-retry') }
  })
  return data
}

export async function rollbackActionExecution(recommendationId: number, executionId: number): Promise<ActionExecution> {
  const { data } = await apiClient.post<ActionExecution>(`/admin-plus/actions/recommendations/${recommendationId}/executions/${executionId}/rollback`, {}, {
    headers: { 'Idempotency-Key': createAdminPlusIdempotencyKey('action-execution-rollback') }
  })
  return data
}

export async function getNotificationCenterStatus(): Promise<NotificationCenterStatus> {
  const { data } = await apiClient.get<NotificationCenterStatus>('/admin-plus/notifications/center/status')
  return data
}

export async function getNotificationSettings(): Promise<NotificationSettings> {
  const { data } = await apiClient.get<NotificationSettings>('/admin-plus/notifications/settings')
  return data
}

export async function updateNotificationSettings(payload: NotificationSettings): Promise<NotificationSettings> {
  const { data } = await apiClient.put<NotificationSettings>('/admin-plus/notifications/settings', payload)
  return data
}

export async function testNotification(payload?: { text?: string }): Promise<NotificationDelivery> {
  const { data } = await apiClient.post<NotificationDelivery>('/admin-plus/notifications/test', payload || {})
  return data
}

export async function retryNotificationDelivery(id: number): Promise<NotificationDelivery> {
  const { data } = await apiClient.post<NotificationDelivery>(`/admin-plus/notifications/deliveries/${id}/retry`)
  return data
}

export async function listNotificationDeliveries(params?: { supplier_id?: number; status?: NotificationDelivery['status'] | ''; channel?: string; event_type?: string } & AdminPlusPaginationParams) {
  const { data } = await apiClient.get<AdminPlusListResponse<NotificationDelivery>>('/admin-plus/notifications/deliveries', { params })
  return data
}


export async function getBackupStatus(): Promise<BackupStatus> {
  const { data } = await apiClient.get<BackupStatus>('/admin-plus/backups/status')
  return data
}

export async function getBackupSettings(): Promise<BackupSettings> {
  const { data } = await apiClient.get<BackupSettings>('/admin-plus/backups/settings')
  return data
}

export async function updateBackupSettings(payload: BackupSettingsUpdatePayload): Promise<BackupSettings> {
  const { data } = await apiClient.put<BackupSettings>('/admin-plus/backups/settings', payload)
  return data
}

export async function testBackupStorage(payload: BackupS3Config): Promise<{ ok: boolean }> {
  const { data } = await apiClient.post<{ ok: boolean }>('/admin-plus/backups/test-storage', payload)
  return data
}

export async function createBackup(payload?: { expire_days?: number }): Promise<BackupRecord> {
  const { data } = await apiClient.post<BackupRecord>('/admin-plus/backups', payload || {})
  return data
}

export async function listBackups(): Promise<BackupRecord[]> {
  const { data } = await apiClient.get<BackupRecord[]>('/admin-plus/backups')
  return data
}

export async function getBackup(id: string): Promise<BackupRecord> {
  const { data } = await apiClient.get<BackupRecord>(`/admin-plus/backups/${id}`)
  return data
}

export async function restoreBackup(id: string, confirmation: string): Promise<BackupRecord> {
  const { data } = await apiClient.post<BackupRecord>(`/admin-plus/backups/${id}/restore`, { confirmation })
  return data
}

export async function getBackupDownloadURL(id: string): Promise<{ url: string }> {
  const { data } = await apiClient.get<{ url: string }>(`/admin-plus/backups/${id}/download-url`)
  return data
}

export async function deleteBackup(id: string): Promise<{ deleted: boolean }> {
  const { data } = await apiClient.delete<{ deleted: boolean }>(`/admin-plus/backups/${id}`)
  return data
}

export async function getMigrationScope(): Promise<ImportExportScope> {
  const { data } = await apiClient.get<ImportExportScope>('/admin-plus/import-export/scope')
  return data
}

export async function exportMigrationArchive(): Promise<ImportExportArchive> {
  const { data } = await apiClient.get<ImportExportArchive>('/admin-plus/import-export/export')
  return data
}

export async function previewMigrationArchive(archive: ImportExportArchive): Promise<ImportExportPreview> {
  const { data } = await apiClient.post<ImportExportPreview>('/admin-plus/import-export/preview', archive)
  return data
}

export async function importMigrationArchive(archive: ImportExportArchive): Promise<ImportExportResult> {
  const { data } = await apiClient.post<ImportExportResult>('/admin-plus/import-export/import', archive)
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
  listSupplierBestChannelChecks,
  listSupplierChannelCheckOverview,
  listSupplierChannelChecks,
  probeSupplierChannel,
  syncSupplierChannelChecks,
  enableSupplierChannelScheduling,
  pauseSupplierChannelScheduling,
  listAccountRateSyncRows,
  syncAccountRateRows,
  retryAccountRateSyncRow,
  renameAccountRateSyncRow,
  renameMatchedAccountRateSyncRows,
  clearAccountRateSyncHistory,
  upsertSupplierBrowserSession,
  listSupplierGroups,
  listSupplierGroupChangeEvents,
  syncSupplierGroups,
  updateSupplierGroupKeyCapacity,
  listSupplierKeys,
  ensureSupplierKeys,
  planEnsureSupplierKeys,
  importSupplierProviderKeyProjection,
  importSupplierProviderKeyProjections,
  provisionSupplierKey,
  standardizeSupplierKeyNames,
  repairSupplierKeyBinding,
  disableSupplierKeyLocalProjection,
  disableSupplierProviderKey,
  deleteSupplierProviderKey,
  listLocalSub2APIAccounts,
  listLocalSub2APIGroups,
  listLocalAccountRuntime,
  listLocalAccountTestModels,
  localAccountTestURL,
  localAccountPurityStreamURL,
  refillLocalGroup,
  listRoutingRefillRuns,
  listRoutingImpactAPIKeys,
  listRoutingImpactFailureRequests,
  getRoutingFailureSensitiveDetail,
  listLocalUsageLines,
  listLocalUsageSummary,
  listSupplierAccounts,
  createSupplierAccount,
  updateSupplierAccount,
  deleteSupplierAccount,
  listRateSnapshots,
  listBalanceEvents,
  listHealthEvents,
  importUsageCostLines,
  syncSupplierUsageCosts,
  listUsageCostLines,
  syncSupplierCosts,
  listSupplierCostSnapshots,
  getSupplierCostLedgerOverview,
  getSupplierCostSummary,
  listSupplierFundingTransactions,
  listSupplierEntitlementTransactions,
  listSupplierCostLedger,
  listLocalAccountOps,
  previewLocalAccountOpsAction,
  applyLocalAccountOpsAction,
  syncLocalAccountState,
  acceptLocalAccountState,
  restoreLocalAccountState,
  createExtensionTask,
  listExtensionTasks,
  getExtensionManifest,
  extensionPackageURL,
  downloadExtensionPackage,
  getSchedulerStatus,
  getSchedulerCenterStatus,
  listSchedulerPlans,
  updateSchedulerPlanConfig,
  updateSchedulerPlanStatus,
  createSchedulerRun,
  listSchedulerRuns,
  getSchedulerRunDetail,
  listSchedulerSteps,
  deleteSchedulerRun,
  deleteSchedulerRuns,
  cancelSchedulerRun,
  retrySchedulerRunFailedSteps,
  retrySchedulerStep,
  cancelSchedulerStep,
  listSchedulerSupplierStatuses,
  getSchedulerSupplierChecklist,
  listSchedulerActions,
  updateSchedulerActionStatus,
  getSchedulerSettings,
  updateSchedulerSettings,
  runScheduler,
  claimExtensionTask,
  getExtensionTaskBrowserCredential,
  heartbeatExtensionTask,
  completeExtensionTask,
  failExtensionTask,
  getSiteDiscoverySettings,
  updateSiteDiscoverySettings,
  runSiteDiscovery,
  runSiteDiscoveryStream,
  listSiteDiscoveryItems,
  importSiteDiscoveryItem,
  registerSiteDiscoveryItem,
  listSiteDiscoveryRegistrationTasks,
  rerunSiteDiscoveryRegistration,
  listSiteDiscoveryRegistrationLogs,
  listSiteDiscoveryRecommendations,
  generateActions,
  listActionRecommendations,
  updateActionRecommendationStatus,
  executeActionRecommendation,
  applyCostReconcileAdjustment,
  applyCostReconcileDetailRepair,
  listActionExecutions,
  retryActionExecution,
  rollbackActionExecution,
  getNotificationCenterStatus,
  getNotificationSettings,
  updateNotificationSettings,
  testNotification,
  retryNotificationDelivery,
  listNotificationDeliveries,
  getBackupStatus,
  getBackupSettings,
  updateBackupSettings,
  testBackupStorage,
  createBackup,
  listBackups,
  getBackup,
  restoreBackup,
  getBackupDownloadURL,
  deleteBackup,
  getMigrationScope,
  exportMigrationArchive,
  previewMigrationArchive,
  importMigrationArchive,
  listAdminPlusSystemLogs
}

export default adminPlusAPI
