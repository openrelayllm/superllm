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
)

type SupplierGroup struct {
	ID                      int64               `json:"id"`
	SupplierID              int64               `json:"supplier_id"`
	ExternalGroupID         string              `json:"external_group_id"`
	Name                    string              `json:"name"`
	Description             string              `json:"description"`
	ProviderFamily          string              `json:"provider_family"`
	RateMultiplier          float64             `json:"rate_multiplier"`
	UserRateMultiplier      *float64            `json:"user_rate_multiplier,omitempty"`
	EffectiveRateMultiplier float64             `json:"effective_rate_multiplier"`
	RPMLimit                *int64              `json:"rpm_limit,omitempty"`
	DailyLimitUSD           *float64            `json:"daily_limit_usd,omitempty"`
	WeeklyLimitUSD          *float64            `json:"weekly_limit_usd,omitempty"`
	MonthlyLimitUSD         *float64            `json:"monthly_limit_usd,omitempty"`
	AllowImageGeneration    bool                `json:"allow_image_generation"`
	IsPrivate               bool                `json:"is_private"`
	Status                  SupplierGroupStatus `json:"status"`
	RawPayload              map[string]any      `json:"raw_payload,omitempty"`
	LastSeenAt              time.Time           `json:"last_seen_at"`
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
