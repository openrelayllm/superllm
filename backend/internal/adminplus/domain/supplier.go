package domain

import (
	"strings"
	"time"
)

type SupplierKind string

const (
	SupplierKindSourceAccount SupplierKind = "source_account"
	SupplierKindRelay         SupplierKind = "relay"
	SupplierKindBrowserOnly   SupplierKind = "browser_only"
	SupplierKindCustom        SupplierKind = "custom"
)

type SupplierType string

const (
	SupplierTypeOpenAI      SupplierType = "openai"
	SupplierTypeAnthropic   SupplierType = "anthropic"
	SupplierTypeGemini      SupplierType = "gemini"
	SupplierTypeSub2API     SupplierType = "sub2api"
	SupplierTypeNewAPI      SupplierType = "new_api"
	SupplierTypeBrowserOnly SupplierType = "browser_only"
	SupplierTypeCustom      SupplierType = "custom"
)

type SupplierRuntimeStatus string

const (
	SupplierRuntimeStatusMonitorOnly SupplierRuntimeStatus = "monitor_only"
	SupplierRuntimeStatusCandidate   SupplierRuntimeStatus = "candidate"
	SupplierRuntimeStatusActive      SupplierRuntimeStatus = "active"
	SupplierRuntimeStatusDisabled    SupplierRuntimeStatus = "disabled"
)

type SupplierHealthStatus string

const (
	SupplierHealthStatusNormal            SupplierHealthStatus = "normal"
	SupplierHealthStatusUnavailable       SupplierHealthStatus = "unavailable"
	SupplierHealthStatusCredentialInvalid SupplierHealthStatus = "credential_invalid"
	SupplierHealthStatusPaused            SupplierHealthStatus = "paused"
)

type SupplierCredentialStatus struct {
	PostgresConfigured             bool   `json:"postgres_configured"`
	RedisConfigured                bool   `json:"redis_configured"`
	BrowserLoginEnabled            bool   `json:"browser_login_enabled"`
	BrowserLoginUsernameConfigured bool   `json:"browser_login_username_configured"`
	BrowserLoginPasswordConfigured bool   `json:"browser_login_password_configured"`
	BrowserLoginTokenConfigured    bool   `json:"browser_login_token_configured"`
	MaskedBrowserLoginUsername     string `json:"masked_browser_login_username,omitempty"`
}

type SupplierCapabilityStatus string

const (
	SupplierCapabilityStatusAvailable       SupplierCapabilityStatus = "available"
	SupplierCapabilityStatusNeedsSession    SupplierCapabilityStatus = "needs_session"
	SupplierCapabilityStatusNeedsReadonlyDB SupplierCapabilityStatus = "needs_readonly_db"
	SupplierCapabilityStatusUnsupported     SupplierCapabilityStatus = "unsupported"
	SupplierCapabilityStatusPlanned         SupplierCapabilityStatus = "planned"
)

type SupplierCapability struct {
	Key         string                   `json:"key"`
	Label       string                   `json:"label"`
	Status      SupplierCapabilityStatus `json:"status"`
	Source      string                   `json:"source"`
	Description string                   `json:"description,omitempty"`
}

type SupplierIntegrationHint struct {
	ID                        string   `json:"id"`
	Label                     string   `json:"label"`
	ProviderLabel             string   `json:"provider_label"`
	Protocol                  string   `json:"protocol"`
	Description               string   `json:"description,omitempty"`
	DocsURL                   string   `json:"docs_url,omitempty"`
	RecommendedSkipModelFetch bool     `json:"recommended_skip_model_fetch"`
	RecommendedModels         []string `json:"recommended_models,omitempty"`
	SourceURL                 string   `json:"source_url,omitempty"`
}

type SupplierPlatformHint struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Family      string `json:"family"`
	Source      string `json:"source"`
	Description string `json:"description,omitempty"`
}

type SupplierAPIEndpointCandidate struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	URL         string `json:"url"`
	Protocol    string `json:"protocol,omitempty"`
	Source      string `json:"source"`
	Recommended bool   `json:"recommended"`
	Description string `json:"description,omitempty"`
}

type SupplierURLHint struct {
	Key          string `json:"key"`
	Label        string `json:"label"`
	URL          string `json:"url"`
	Source       string `json:"source"`
	Action       string `json:"action"`
	Severity     string `json:"severity"`
	MatchedPath  string `json:"matched_path,omitempty"`
	SuggestedURL string `json:"suggested_url,omitempty"`
	Description  string `json:"description,omitempty"`
}

type SupplierOperationHint struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Severity    string `json:"severity"`
	Source      string `json:"source"`
	Description string `json:"description,omitempty"`
}

const (
	SupplierKeyLimitPolicyUnknown     = "unknown"
	SupplierKeyLimitPolicyUnlimited   = "unlimited"
	SupplierKeyLimitPolicyLimited     = "limited"
	SupplierKeyLimitPolicyUnsupported = "unsupported"

	SupplierKeyCapacityAvailable   = "available"
	SupplierKeyCapacityLimited     = "limited"
	SupplierKeyCapacityExhausted   = "exhausted"
	SupplierKeyCapacityUnknown     = "unknown"
	SupplierKeyCapacityUnsupported = "unsupported"
)

type Supplier struct {
	ID                    int64                          `json:"id"`
	Name                  string                         `json:"name"`
	Kind                  SupplierKind                   `json:"kind"`
	Type                  SupplierType                   `json:"type"`
	RuntimeStatus         SupplierRuntimeStatus          `json:"runtime_status"`
	HealthStatus          SupplierHealthStatus           `json:"health_status"`
	DashboardURL          string                         `json:"dashboard_url,omitempty"`
	APIBaseURL            string                         `json:"api_base_url,omitempty"`
	ThirdPartyRechargeURL string                         `json:"third_party_recharge_url,omitempty"`
	LocalRechargeURL      string                         `json:"local_recharge_url,omitempty"`
	Contact               string                         `json:"contact,omitempty"`
	Notes                 string                         `json:"notes,omitempty"`
	BrowserLoginUsername  string                         `json:"-"`
	BrowserLoginPassword  string                         `json:"-"`
	BrowserLoginToken     string                         `json:"-"`
	Credential            SupplierCredentialStatus       `json:"credential"`
	Capabilities          []SupplierCapability           `json:"capabilities,omitempty"`
	IntegrationHint       *SupplierIntegrationHint       `json:"integration_hint,omitempty"`
	PlatformHint          *SupplierPlatformHint          `json:"platform_hint,omitempty"`
	APIEndpointCandidates []SupplierAPIEndpointCandidate `json:"api_endpoint_candidates,omitempty"`
	URLHints              []SupplierURLHint              `json:"url_hints,omitempty"`
	OperationHints        []SupplierOperationHint        `json:"operation_hints,omitempty"`
	BalanceCents          int64                          `json:"balance_cents"`
	BalanceCurrency       string                         `json:"balance_currency"`
	BalanceUpdatedAt      *time.Time                     `json:"balance_updated_at,omitempty"`
	RechargeMultiplier    float64                        `json:"recharge_multiplier"`
	KeyLimitPolicy        string                         `json:"key_limit_policy"`
	KeyLimitValue         int                            `json:"key_limit_value"`
	ActiveKeyCount        int                            `json:"active_key_count"`
	KeyCapacityStatus     string                         `json:"key_capacity_status"`
	CreatedAt             time.Time                      `json:"created_at"`
	UpdatedAt             time.Time                      `json:"updated_at"`
}

type SupplierBrowserCredential struct {
	SupplierID   int64        `json:"supplier_id"`
	SupplierName string       `json:"supplier_name"`
	Kind         SupplierKind `json:"supplier_kind"`
	Type         SupplierType `json:"supplier_type"`
	DashboardURL string       `json:"dashboard_url"`
	APIBaseURL   string       `json:"api_base_url,omitempty"`
	Username     string       `json:"username,omitempty"`
	Password     string       `json:"password,omitempty"`
	Token        string       `json:"token,omitempty"`
}

type SupplierAccount struct {
	ID                        int64                 `json:"id"`
	SupplierID                int64                 `json:"supplier_id"`
	SupplierKeyID             int64                 `json:"supplier_key_id,omitempty"`
	LocalSub2APIAccountID     int64                 `json:"local_sub2api_account_id"`
	LocalAccountName          string                `json:"local_account_name"`
	LocalAccountPlatform      string                `json:"local_account_platform"`
	LocalAccountType          string                `json:"local_account_type"`
	SupplierAccountIdentifier string                `json:"supplier_account_identifier,omitempty"`
	SupplierAccountLabel      string                `json:"supplier_account_label,omitempty"`
	SupplierGroupID           int64                 `json:"supplier_group_id,omitempty"`
	SupplierExternalGroupID   string                `json:"supplier_external_group_id,omitempty"`
	SupplierGroupName         string                `json:"supplier_group_name,omitempty"`
	SupplierGroupProvider     string                `json:"supplier_group_provider,omitempty"`
	SupplierGroupRate         float64               `json:"supplier_group_rate,omitempty"`
	SupplierKeyName           string                `json:"supplier_key_name,omitempty"`
	SupplierKeyExternalID     string                `json:"supplier_key_external_id,omitempty"`
	SupplierKeyLast4          string                `json:"supplier_key_last4,omitempty"`
	OrganizationID            string                `json:"organization_id,omitempty"`
	ProjectID                 string                `json:"project_id,omitempty"`
	RateProfile               string                `json:"rate_profile,omitempty"`
	ConfiguredConcurrency     int                   `json:"configured_concurrency"`
	ObservedMaxConcurrency    int                   `json:"observed_max_concurrency"`
	BalanceThresholdCents     int64                 `json:"balance_threshold_cents"`
	BalanceCents              int64                 `json:"balance_cents"`
	BalanceCurrency           string                `json:"balance_currency"`
	HasUsableBalance          bool                  `json:"has_usable_balance"`
	RuntimeStatus             SupplierRuntimeStatus `json:"runtime_status"`
	HealthStatus              SupplierHealthStatus  `json:"health_status"`
	CreatedAt                 time.Time             `json:"created_at"`
	UpdatedAt                 time.Time             `json:"updated_at"`
}

type LocalSub2APIAccount struct {
	ID                      int64      `json:"id"`
	Name                    string     `json:"name"`
	Notes                   string     `json:"notes,omitempty"`
	Platform                string     `json:"platform"`
	Type                    string     `json:"type"`
	Status                  string     `json:"status"`
	ErrorMessage            string     `json:"error_message,omitempty"`
	Schedulable             bool       `json:"schedulable"`
	Concurrency             int        `json:"concurrency"`
	LoadFactor              *int       `json:"load_factor,omitempty"`
	Priority                int        `json:"priority"`
	RateMultiplier          float64    `json:"rate_multiplier"`
	LastUsedAt              *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt               *time.Time `json:"expires_at,omitempty"`
	AutoPauseOnExpired      bool       `json:"auto_pause_on_expired"`
	RateLimitedAt           *time.Time `json:"rate_limited_at,omitempty"`
	RateLimitResetAt        *time.Time `json:"rate_limit_reset_at,omitempty"`
	OverloadUntil           *time.Time `json:"overload_until,omitempty"`
	TempUnschedulableUntil  *time.Time `json:"temp_unschedulable_until,omitempty"`
	TempUnschedulableReason string     `json:"temp_unschedulable_reason,omitempty"`
	SessionWindowStart      *time.Time `json:"session_window_start,omitempty"`
	SessionWindowEnd        *time.Time `json:"session_window_end,omitempty"`
	SessionWindowStatus     string     `json:"session_window_status,omitempty"`
	CreatedAt               time.Time  `json:"created_at"`
	UpdatedAt               time.Time  `json:"updated_at"`
	GroupIDs                []int64    `json:"group_ids,omitempty"`
	GroupNames              []string   `json:"group_names,omitempty"`
}

func (k SupplierKind) Valid() bool {
	switch k {
	case SupplierKindSourceAccount, SupplierKindRelay, SupplierKindBrowserOnly, SupplierKindCustom:
		return true
	default:
		return false
	}
}

func (t SupplierType) Valid() bool {
	switch t {
	case SupplierTypeOpenAI, SupplierTypeAnthropic, SupplierTypeGemini, SupplierTypeSub2API, SupplierTypeNewAPI, SupplierTypeBrowserOnly, SupplierTypeCustom:
		return true
	default:
		return false
	}
}

func (s SupplierRuntimeStatus) Valid() bool {
	switch s {
	case SupplierRuntimeStatusMonitorOnly, SupplierRuntimeStatusCandidate, SupplierRuntimeStatusActive, SupplierRuntimeStatusDisabled:
		return true
	default:
		return false
	}
}

func (s SupplierHealthStatus) Valid() bool {
	switch s {
	case SupplierHealthStatusNormal, SupplierHealthStatusUnavailable, SupplierHealthStatusCredentialInvalid, SupplierHealthStatusPaused:
		return true
	default:
		return false
	}
}

func (s SupplierCapabilityStatus) Valid() bool {
	switch s {
	case SupplierCapabilityStatusAvailable, SupplierCapabilityStatusNeedsSession, SupplierCapabilityStatusNeedsReadonlyDB, SupplierCapabilityStatusUnsupported, SupplierCapabilityStatusPlanned:
		return true
	default:
		return false
	}
}

func NormalizeSupplierKind(value string) SupplierKind {
	return SupplierKind(strings.ToLower(strings.TrimSpace(value)))
}

func NormalizeSupplierType(value string) SupplierType {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "newapi", "new api", "new-api", "oneapi", "one api", "one-api", "onehub", "one-hub", "donehub", "done-hub", "veloera", "anyrouter", "vo-api", "voapi", "super-api", "superapi", "rix-api", "rixapi", "neo-api", "neoapi", "wong-gongyi", "wong gongyi":
		return SupplierTypeNewAPI
	case "sub2api", "sub2 api", "sub2-api", "sub2_api", "subapi", "sub api", "sub-api", "sub_api":
		return SupplierTypeSub2API
	case "anthropic", "claude", "claude-compatible", "claude_compatible":
		return SupplierTypeAnthropic
	case "gemini", "google", "google-ai", "google_ai", "google-ai-studio", "google_ai_studio":
		return SupplierTypeGemini
	case "browser", "browser-only", "browser_only":
		return SupplierTypeBrowserOnly
	default:
		return SupplierType(normalized)
	}
}

func NormalizeSupplierRuntimeStatus(value string) SupplierRuntimeStatus {
	return SupplierRuntimeStatus(strings.ToLower(strings.TrimSpace(value)))
}

func NormalizeSupplierHealthStatus(value string) SupplierHealthStatus {
	return SupplierHealthStatus(strings.ToLower(strings.TrimSpace(value)))
}

func NormalizeSupplierCapabilityStatus(value string) SupplierCapabilityStatus {
	return SupplierCapabilityStatus(strings.ToLower(strings.TrimSpace(value)))
}

func IsSwitchableSupplierStatus(status SupplierRuntimeStatus) bool {
	return status == SupplierRuntimeStatusCandidate || status == SupplierRuntimeStatusActive
}

func CanUseSupplierForSwitching(status SupplierRuntimeStatus, balanceCents int64) bool {
	return IsSwitchableSupplierStatus(status) && balanceCents > 0
}

func CanUseSupplierAccountForSwitching(parentStatus SupplierRuntimeStatus, accountStatus SupplierRuntimeStatus, balanceCents int64, healthStatus SupplierHealthStatus) bool {
	return IsSwitchableSupplierStatus(parentStatus) &&
		IsSwitchableSupplierStatus(accountStatus) &&
		balanceCents > 0 &&
		healthStatus == SupplierHealthStatusNormal
}
