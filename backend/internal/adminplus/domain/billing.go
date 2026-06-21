package domain

import "time"

type SupplierUsageCostLine struct {
	ID                  int64          `json:"id"`
	SupplierID          int64          `json:"supplier_id"`
	Source              string         `json:"source"`
	ExternalUsageCostID string         `json:"external_usage_cost_id,omitempty"`
	ExternalRequestID   string         `json:"external_request_id,omitempty"`
	APIKeyName          string         `json:"api_key_name,omitempty"`
	Model               string         `json:"model"`
	Endpoint            string         `json:"endpoint,omitempty"`
	RequestType         string         `json:"request_type,omitempty"`
	BillingMode         string         `json:"billing_mode,omitempty"`
	ReasoningEffort     string         `json:"reasoning_effort,omitempty"`
	Currency            string         `json:"currency"`
	CostCents           int64          `json:"cost_cents"`
	InputTokens         int64          `json:"input_tokens"`
	OutputTokens        int64          `json:"output_tokens"`
	CacheReadTokens     int64          `json:"cache_read_tokens"`
	TotalTokens         int64          `json:"total_tokens"`
	FirstTokenMS        int64          `json:"first_token_ms"`
	DurationMS          int64          `json:"duration_ms"`
	UserAgent           string         `json:"user_agent,omitempty"`
	StartedAt           time.Time      `json:"started_at"`
	EndedAt             *time.Time     `json:"ended_at,omitempty"`
	RawPayload          map[string]any `json:"raw_payload,omitempty"`
	CreatedAt           time.Time      `json:"created_at"`
}

type LocalUsageLine struct {
	ID                int64     `json:"id"`
	AccountID         int64     `json:"account_id,omitempty"`
	AccountName       string    `json:"account_name,omitempty"`
	AccountPlatform   string    `json:"account_platform,omitempty"`
	ExternalRequestID string    `json:"external_request_id,omitempty"`
	Model             string    `json:"model"`
	Currency          string    `json:"currency"`
	RevenueCents      int64     `json:"revenue_cents"`
	InputTokens       int64     `json:"input_tokens"`
	OutputTokens      int64     `json:"output_tokens"`
	StartedAt         time.Time `json:"started_at"`
}

type LocalUsageSummary struct {
	AccountID            int64     `json:"account_id"`
	AccountName          string    `json:"account_name"`
	AccountPlatform      string    `json:"account_platform"`
	Model                string    `json:"model"`
	RequestCount         int64     `json:"request_count"`
	InputTokens          int64     `json:"input_tokens"`
	OutputTokens         int64     `json:"output_tokens"`
	RevenueCents         int64     `json:"revenue_cents"`
	AccountCostCents     int64     `json:"account_cost_cents"`
	OriginalCostCents    int64     `json:"original_cost_cents"`
	AvgFirstTokenMs      int64     `json:"avg_first_token_ms"`
	AvgTotalLatencyMs    int64     `json:"avg_total_latency_ms"`
	WindowStart          time.Time `json:"window_start"`
	WindowEnd            time.Time `json:"window_end"`
	LastRequestCreatedAt time.Time `json:"last_request_created_at"`
}

type LocalAccountUsageSummary struct {
	AccountID            int64     `json:"account_id"`
	AccountName          string    `json:"account_name"`
	AccountPlatform      string    `json:"account_platform"`
	RequestCount         int64     `json:"request_count"`
	InputTokens          int64     `json:"input_tokens"`
	OutputTokens         int64     `json:"output_tokens"`
	TotalTokens          int64     `json:"total_tokens"`
	RevenueCents         int64     `json:"revenue_cents"`
	AccountCostCents     int64     `json:"account_cost_cents"`
	OriginalCostCents    int64     `json:"original_cost_cents"`
	AvgFirstTokenMs      int64     `json:"avg_first_token_ms"`
	AvgTotalLatencyMs    int64     `json:"avg_total_latency_ms"`
	WindowStart          time.Time `json:"window_start"`
	WindowEnd            time.Time `json:"window_end"`
	LastRequestCreatedAt time.Time `json:"last_request_created_at"`
}

type LocalAccountRuntime struct {
	AccountID           int64      `json:"account_id"`
	AccountName         string     `json:"account_name"`
	AccountPlatform     string     `json:"account_platform"`
	AccountType         string     `json:"account_type"`
	Status              string     `json:"status"`
	Schedulable         bool       `json:"schedulable"`
	ConfiguredLimit     int        `json:"configured_limit"`
	CurrentConcurrency  int        `json:"current_concurrency"`
	WaitingCount        int        `json:"waiting_count"`
	LoadPercent         float64    `json:"load_percent"`
	SwitchEligible      bool       `json:"switch_eligible"`
	BlockedReason       string     `json:"blocked_reason,omitempty"`
	ErrorMessage        string     `json:"error_message,omitempty"`
	RateLimitResetAt    *time.Time `json:"rate_limit_reset_at,omitempty"`
	OverloadUntil       *time.Time `json:"overload_until,omitempty"`
	TempUnschedUntil    *time.Time `json:"temp_unsched_until,omitempty"`
	TempUnschedReason   string     `json:"temp_unsched_reason,omitempty"`
	LastUsedAt          *time.Time `json:"last_used_at,omitempty"`
	CollectedAt         time.Time  `json:"collected_at"`
	RedisReadConfigured bool       `json:"redis_read_configured"`
}
