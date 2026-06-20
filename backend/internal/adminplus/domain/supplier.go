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
	AdminAPIKeyConfigured          bool   `json:"admin_api_key_configured"`
	PostgresConfigured             bool   `json:"postgres_configured"`
	RedisConfigured                bool   `json:"redis_configured"`
	BrowserLoginEnabled            bool   `json:"browser_login_enabled"`
	BrowserLoginUsernameConfigured bool   `json:"browser_login_username_configured"`
	BrowserLoginPasswordConfigured bool   `json:"browser_login_password_configured"`
	BrowserLoginTokenConfigured    bool   `json:"browser_login_token_configured"`
	MaskedAdminAPIKey              string `json:"masked_admin_api_key,omitempty"`
	MaskedBrowserLoginUsername     string `json:"masked_browser_login_username,omitempty"`
}

type Supplier struct {
	ID                   int64                    `json:"id"`
	Name                 string                   `json:"name"`
	Kind                 SupplierKind             `json:"kind"`
	Type                 SupplierType             `json:"type"`
	RuntimeStatus        SupplierRuntimeStatus    `json:"runtime_status"`
	HealthStatus         SupplierHealthStatus     `json:"health_status"`
	BoundAccountID       int64                    `json:"bound_account_id,omitempty"`
	BoundAccountName     string                   `json:"bound_account_name,omitempty"`
	BoundAccountPlatform string                   `json:"bound_account_platform,omitempty"`
	BoundAccountType     string                   `json:"bound_account_type,omitempty"`
	DashboardURL         string                   `json:"dashboard_url,omitempty"`
	APIBaseURL           string                   `json:"api_base_url,omitempty"`
	Contact              string                   `json:"contact,omitempty"`
	Notes                string                   `json:"notes,omitempty"`
	BrowserLoginUsername string                   `json:"-"`
	BrowserLoginPassword string                   `json:"-"`
	BrowserLoginToken    string                   `json:"-"`
	Credential           SupplierCredentialStatus `json:"credential"`
	BalanceCents         int64                    `json:"balance_cents"`
	BalanceCurrency      string                   `json:"balance_currency"`
	BalanceUpdatedAt     *time.Time               `json:"balance_updated_at,omitempty"`
	CreatedAt            time.Time                `json:"created_at"`
	UpdatedAt            time.Time                `json:"updated_at"`
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
