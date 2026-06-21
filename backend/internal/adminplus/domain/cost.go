package domain

import "time"

type SupplierFundingTransaction struct {
	ID                int64          `json:"id"`
	SupplierID        int64          `json:"supplier_id"`
	ProviderType      string         `json:"provider_type"`
	ExternalID        string         `json:"external_id"`
	OutTradeNo        string         `json:"out_trade_no,omitempty"`
	PaymentTradeNo    string         `json:"payment_trade_no,omitempty"`
	PaymentType       string         `json:"payment_type,omitempty"`
	OrderType         string         `json:"order_type,omitempty"`
	Status            string         `json:"status"`
	Currency          string         `json:"currency"`
	AmountCents       int64          `json:"amount_cents"`
	CashAmountCents   int64          `json:"cash_amount_cents"`
	RefundAmountCents int64          `json:"refund_amount_cents"`
	FeeRate           *float64       `json:"fee_rate,omitempty"`
	CreatedAtExternal *time.Time     `json:"created_at_external,omitempty"`
	PaidAt            *time.Time     `json:"paid_at,omitempty"`
	CompletedAt       *time.Time     `json:"completed_at,omitempty"`
	RawPayload        map[string]any `json:"raw_payload,omitempty"`
	LastSeenAt        time.Time      `json:"last_seen_at"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
}

type SupplierEntitlementTransaction struct {
	ID                int64          `json:"id"`
	SupplierID        int64          `json:"supplier_id"`
	ProviderType      string         `json:"provider_type"`
	ExternalID        string         `json:"external_id"`
	CodeFingerprint   string         `json:"code_fingerprint,omitempty"`
	CodeLast4         string         `json:"code_last4,omitempty"`
	SourceFamily      string         `json:"source_family"`
	Type              string         `json:"type"`
	Status            string         `json:"status"`
	Currency          string         `json:"currency"`
	ValueCents        int64          `json:"value_cents"`
	RawValue          float64        `json:"raw_value"`
	GroupID           int64          `json:"group_id,omitempty"`
	ValidityDays      int            `json:"validity_days,omitempty"`
	UsedAt            *time.Time     `json:"used_at,omitempty"`
	CreatedAtExternal *time.Time     `json:"created_at_external,omitempty"`
	RawPayload        map[string]any `json:"raw_payload,omitempty"`
	LastSeenAt        time.Time      `json:"last_seen_at"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
}

type SupplierCostLedgerEntry struct {
	ID               int64          `json:"id"`
	SupplierID       int64          `json:"supplier_id"`
	ProviderType     string         `json:"provider_type"`
	EntryType        string         `json:"entry_type"`
	SourceType       string         `json:"source_type"`
	SourceID         int64          `json:"source_id"`
	SourceExternalID string         `json:"source_external_id,omitempty"`
	Currency         string         `json:"currency"`
	AmountCents      int64          `json:"amount_cents"`
	CashAmountCents  int64          `json:"cash_amount_cents"`
	OccurredAt       time.Time      `json:"occurred_at"`
	RawPayload       map[string]any `json:"raw_payload,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
}

type SupplierCostSnapshot struct {
	ID                          int64     `json:"id"`
	SupplierID                  int64     `json:"supplier_id"`
	Currency                    string    `json:"currency"`
	CompletedFundingAmountCents int64     `json:"completed_funding_amount_cents"`
	CompletedFundingCashCents   int64     `json:"completed_funding_cash_cents"`
	EntitlementAmountCents      int64     `json:"entitlement_amount_cents"`
	UsageCostCents              int64     `json:"usage_cost_cents"`
	RefundAmountCents           int64     `json:"refund_amount_cents"`
	AdjustmentAmountCents       int64     `json:"adjustment_amount_cents"`
	ExpectedBalanceCents        int64     `json:"expected_balance_cents"`
	ActualBalanceCents          *int64    `json:"actual_balance_cents,omitempty"`
	BalanceDeltaCents           *int64    `json:"balance_delta_cents,omitempty"`
	CapturedAt                  time.Time `json:"captured_at"`
	CreatedAt                   time.Time `json:"created_at"`
}
