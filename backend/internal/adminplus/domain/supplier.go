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

type Supplier struct {
	ID                    int64                    `json:"id"`
	Name                  string                   `json:"name"`
	Kind                  SupplierKind             `json:"kind"`
	Type                  SupplierType             `json:"type"`
	RuntimeStatus         SupplierRuntimeStatus    `json:"runtime_status"`
	HealthStatus          SupplierHealthStatus     `json:"health_status"`
	DashboardURL          string                   `json:"dashboard_url,omitempty"`
	APIBaseURL            string                   `json:"api_base_url,omitempty"`
	ThirdPartyRechargeURL string                   `json:"third_party_recharge_url,omitempty"`
	LocalRechargeURL      string                   `json:"local_recharge_url,omitempty"`
	Contact               string                   `json:"contact,omitempty"`
	Notes                 string                   `json:"notes,omitempty"`
	BrowserLoginUsername  string                   `json:"-"`
	BrowserLoginPassword  string                   `json:"-"`
	BrowserLoginToken     string                   `json:"-"`
	Credential            SupplierCredentialStatus `json:"credential"`
	BalanceCents          int64                    `json:"balance_cents"`
	BalanceCurrency       string                   `json:"balance_currency"`
	BalanceUpdatedAt      *time.Time               `json:"balance_updated_at,omitempty"`
	RechargeMultiplier    float64                  `json:"recharge_multiplier"`
	CreatedAt             time.Time                `json:"created_at"`
	UpdatedAt             time.Time                `json:"updated_at"`
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
	ID             int64    `json:"id"`
	Name           string   `json:"name"`
	Platform       string   `json:"platform"`
	Type           string   `json:"type"`
	Status         string   `json:"status"`
	Schedulable    bool     `json:"schedulable"`
	Concurrency    int      `json:"concurrency"`
	Priority       int      `json:"priority"`
	RateMultiplier float64  `json:"rate_multiplier"`
	GroupIDs       []int64  `json:"group_ids,omitempty"`
	GroupNames     []string `json:"group_names,omitempty"`
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

func NormalizeSupplierKind(value string) SupplierKind {
	return SupplierKind(strings.ToLower(strings.TrimSpace(value)))
}

func NormalizeSupplierType(value string) SupplierType {
	return SupplierType(strings.ToLower(strings.TrimSpace(value)))
}

func NormalizeSupplierRuntimeStatus(value string) SupplierRuntimeStatus {
	return SupplierRuntimeStatus(strings.ToLower(strings.TrimSpace(value)))
}

func NormalizeSupplierHealthStatus(value string) SupplierHealthStatus {
	return SupplierHealthStatus(strings.ToLower(strings.TrimSpace(value)))
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
