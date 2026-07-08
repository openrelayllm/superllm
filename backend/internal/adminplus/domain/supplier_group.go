package domain

import (
	"strings"
	"time"
)

type SupplierGroupStatus string

const (
	SupplierGroupStatusActive   SupplierGroupStatus = "active"
	SupplierGroupStatusMissing  SupplierGroupStatus = "missing"
	SupplierGroupStatusDisabled SupplierGroupStatus = "disabled"

	SupplierGroupKeyLimitPolicyInherit     = "inherit"
	SupplierGroupKeyLimitPolicyUnknown     = "unknown"
	SupplierGroupKeyLimitPolicyUnlimited   = "unlimited"
	SupplierGroupKeyLimitPolicyLimited     = "limited"
	SupplierGroupKeyLimitPolicyUnsupported = "unsupported"
)

type SupplierGroup struct {
	ID                      int64               `json:"id"`
	SupplierID              int64               `json:"supplier_id"`
	ExternalGroupID         string              `json:"external_group_id"`
	Name                    string              `json:"name"`
	Description             string              `json:"description"`
	ProviderFamily          string              `json:"provider_family"`
	OfficialName            string              `json:"official_name"`
	ModelFamily             string              `json:"model_family"`
	ModelSpec               string              `json:"model_spec"`
	StandardKeyName         string              `json:"standard_key_name"`
	RateMultiplier          float64             `json:"rate_multiplier"`
	UserRateMultiplier      *float64            `json:"user_rate_multiplier,omitempty"`
	EffectiveRateMultiplier float64             `json:"effective_rate_multiplier"`
	RPMLimit                *int64              `json:"rpm_limit,omitempty"`
	DailyLimitUSD           *float64            `json:"daily_limit_usd,omitempty"`
	WeeklyLimitUSD          *float64            `json:"weekly_limit_usd,omitempty"`
	MonthlyLimitUSD         *float64            `json:"monthly_limit_usd,omitempty"`
	AllowImageGeneration    bool                `json:"allow_image_generation"`
	IsPrivate               bool                `json:"is_private"`
	KeyLimitPolicy          string              `json:"key_limit_policy"`
	KeyLimitValue           int                 `json:"key_limit_value"`
	ActiveKeyCount          int                 `json:"active_key_count"`
	KeyCapacityStatus       string              `json:"key_capacity_status"`
	Status                  SupplierGroupStatus `json:"status"`
	RawPayload              map[string]any      `json:"raw_payload,omitempty"`
	LastSeenAt              time.Time           `json:"last_seen_at"`
	NamingUpdatedAt         *time.Time          `json:"naming_updated_at,omitempty"`
	CreatedAt               time.Time           `json:"created_at"`
	UpdatedAt               time.Time           `json:"updated_at"`
}

func (s SupplierGroupStatus) Valid() bool {
	switch s {
	case SupplierGroupStatusActive, SupplierGroupStatusMissing, SupplierGroupStatusDisabled:
		return true
	default:
		return false
	}
}

func NormalizeSupplierGroupStatus(value string) SupplierGroupStatus {
	return SupplierGroupStatus(strings.ToLower(strings.TrimSpace(value)))
}
