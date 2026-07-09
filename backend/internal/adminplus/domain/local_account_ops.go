package domain

import "time"

type LocalAccountOpsRow struct {
	LocalSub2APIAccountID         int64      `json:"local_sub2api_account_id"`
	LocalAccountName              string     `json:"local_account_name"`
	LocalAccountPlatform          string     `json:"local_account_platform"`
	LocalAccountType              string     `json:"local_account_type"`
	LocalAccountStatus            string     `json:"local_account_status"`
	LocalAccountErrorMessage      string     `json:"local_account_error_message,omitempty"`
	LocalAccountSchedulable       bool       `json:"local_account_schedulable"`
	LocalAccountConcurrency       int        `json:"local_account_concurrency"`
	LocalAccountPriority          int        `json:"local_account_priority"`
	LocalAccountRateMultiplier    float64    `json:"local_account_rate_multiplier"`
	LocalAccountRateLimitedAt     *time.Time `json:"local_account_rate_limited_at,omitempty"`
	LocalAccountRateLimitResetAt  *time.Time `json:"local_account_rate_limit_reset_at,omitempty"`
	LocalAccountOverloadUntil     *time.Time `json:"local_account_overload_until,omitempty"`
	LocalAccountTempUnschedAt     *time.Time `json:"local_account_temp_unschedulable_until,omitempty"`
	LocalAccountTempUnschedReason string     `json:"local_account_temp_unschedulable_reason,omitempty"`
	LocalAccountUpdatedAt         time.Time  `json:"local_account_updated_at"`
	LocalAccountGroupIDs          []int64    `json:"local_account_group_ids,omitempty"`
	LocalAccountGroupNames        []string   `json:"local_account_group_names,omitempty"`
	LocalAccountProxyID           int64      `json:"local_account_proxy_id,omitempty"`
	LocalAccountProxyName         string     `json:"local_account_proxy_name,omitempty"`
	LocalAccountProxyStatus       string     `json:"local_account_proxy_status,omitempty"`
	LocalAccountProxyExpiresAt    *time.Time `json:"local_account_proxy_expires_at,omitempty"`

	SupplierAccountID            int64  `json:"supplier_account_id,omitempty"`
	SupplierID                   int64  `json:"supplier_id,omitempty"`
	SupplierName                 string `json:"supplier_name,omitempty"`
	SupplierType                 string `json:"supplier_type,omitempty"`
	SupplierRuntimeStatus        string `json:"supplier_runtime_status,omitempty"`
	SupplierHealthStatus         string `json:"supplier_health_status,omitempty"`
	SupplierAccountRuntimeStatus string `json:"supplier_account_runtime_status,omitempty"`
	SupplierAccountHealthStatus  string `json:"supplier_account_health_status,omitempty"`

	SupplierGroupID          int64   `json:"supplier_group_id,omitempty"`
	SupplierExternalGroupID  string  `json:"supplier_external_group_id,omitempty"`
	SupplierGroupName        string  `json:"supplier_group_name,omitempty"`
	SupplierGroupProvider    string  `json:"supplier_group_provider,omitempty"`
	SupplierGroupModelFamily string  `json:"supplier_group_model_family,omitempty"`
	SupplierGroupModelSpec   string  `json:"supplier_group_model_spec,omitempty"`
	SupplierGroupStatus      string  `json:"supplier_group_status,omitempty"`
	EffectiveRateMultiplier  float64 `json:"effective_rate_multiplier"`

	SupplierKeyID     int64  `json:"supplier_key_id,omitempty"`
	SupplierKeyName   string `json:"supplier_key_name,omitempty"`
	SupplierKeyLast4  string `json:"supplier_key_last4,omitempty"`
	SupplierKeyStatus string `json:"supplier_key_status,omitempty"`

	BalanceThresholdCents int64  `json:"balance_threshold_cents"`
	BalanceCents          int64  `json:"balance_cents"`
	BalanceCurrency       string `json:"balance_currency"`
	HasUsableBalance      bool   `json:"has_usable_balance"`
	BalanceStatus         string `json:"balance_status"`

	ChannelCheckStatus  string     `json:"channel_check_status"`
	ChannelRemoteStatus string     `json:"channel_remote_status,omitempty"`
	ChannelRecommended  bool       `json:"channel_recommended"`
	ChannelStatusCode   int        `json:"channel_status_code,omitempty"`
	ChannelErrorClass   string     `json:"channel_error_class,omitempty"`
	ChannelErrorMessage string     `json:"channel_error_message,omitempty"`
	LastChannelCheckAt  *time.Time `json:"last_channel_check_at,omitempty"`
	DriftStatus         string     `json:"drift_status"`
	LastLocalSyncAt     *time.Time `json:"last_local_sync_at,omitempty"`

	CandidateStatus   string     `json:"candidate_status,omitempty"`
	BlockedReason     string     `json:"blocked_reason,omitempty"`
	CheckSource       string     `json:"check_source,omitempty"`
	KeyCapacityStatus string     `json:"key_capacity_status,omitempty"`
	ModelScope        string     `json:"model_scope,omitempty"`
	ModelMatchStatus  string     `json:"model_match_status,omitempty"`
	PurityStatus      string     `json:"purity_status,omitempty"`
	PurityFreshness   string     `json:"purity_freshness_status,omitempty"`
	PurityVerdict     string     `json:"purity_verdict,omitempty"`
	PurityReportID    string     `json:"purity_report_id,omitempty"`
	PurityModel       string     `json:"purity_model,omitempty"`
	PurityScore       int        `json:"purity_score,omitempty"`
	PurityCheckedAt   *time.Time `json:"purity_checked_at,omitempty"`
}
