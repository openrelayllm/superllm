package ports

import (
	"context"
	"time"

	balancesapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/balances"
	healthapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/health"
	promotionsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/promotions"
	ratesapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/rates"
)

type ProviderKind string

const (
	ProviderKindSub2API     ProviderKind = "sub2api"
	ProviderKindNewAPI      ProviderKind = "new_api"
	ProviderKindSourceLLM   ProviderKind = "source_llm"
	ProviderKindBrowserOnly ProviderKind = "browser_only"
	ProviderKindCustom      ProviderKind = "custom"
)

type ProviderIdentity struct {
	SupplierID int64
	Kind       ProviderKind
	Name       string
	BaseURL    string
}

type FetchContext struct {
	SupplierID int64
	CapturedAt time.Time
	TraceID    string
}

type SessionProbeInput struct {
	SupplierID int64
	Origin     string
	APIBaseURL string
	Bundle     map[string]any
}

type SessionProbeResult struct {
	SupplierID      int64                `json:"supplier_id"`
	Status          string               `json:"status"`
	SystemType      string               `json:"system_type"`
	Origin          string               `json:"origin"`
	APIBaseURL      string               `json:"api_base_url"`
	Capabilities    map[string]bool      `json:"capabilities"`
	Profile         *UserProfileSnapshot `json:"profile,omitempty"`
	BalanceCents    *int64               `json:"balance_cents,omitempty"`
	BalanceCurrency string               `json:"balance_currency,omitempty"`
	Diagnostics     map[string]any       `json:"diagnostics,omitempty"`
	ProbedAt        time.Time            `json:"probed_at"`
}

type UserProfileSnapshot struct {
	ID            int64   `json:"id,omitempty"`
	Email         string  `json:"email,omitempty"`
	Username      string  `json:"username,omitempty"`
	Role          string  `json:"role,omitempty"`
	Status        string  `json:"status,omitempty"`
	Balance       float64 `json:"balance"`
	Concurrency   int     `json:"concurrency,omitempty"`
	AllowedGroups []int64 `json:"allowed_groups,omitempty"`
}

type SessionProbeAdapter interface {
	ProbeSub2APIUserProfile(ctx context.Context, in SessionProbeInput) (*SessionProbeResult, error)
}

type ProviderGroup struct {
	ExternalGroupID         string
	Name                    string
	Description             string
	ProviderFamily          string
	RateMultiplier          float64
	UserRateMultiplier      *float64
	EffectiveRateMultiplier float64
	RPMLimit                *int64
	DailyLimitUSD           *float64
	WeeklyLimitUSD          *float64
	MonthlyLimitUSD         *float64
	AllowImageGeneration    bool
	IsPrivate               bool
	Status                  string
	RawPayload              map[string]any
}

type ReadGroupsResult struct {
	SupplierID int64
	SystemType string
	Origin     string
	APIBaseURL string
	Groups     []*ProviderGroup
	CapturedAt time.Time
}

type SessionGroupAdapter interface {
	ReadGroups(ctx context.Context, in SessionProbeInput) (*ReadGroupsResult, error)
}

type BillExportRequest struct {
	SupplierID int64
	StartedAt  time.Time
	EndedAt    time.Time
	Format     string
}

type BillExportResult struct {
	SupplierID int64
	FileName   string
	MimeType   string
	Content    []byte
	ExportedAt time.Time
}

type ProviderAdapter interface {
	Identity() ProviderIdentity
	FetchRateCatalog(ctx context.Context, fetch FetchContext) ([]ratesapp.RateEntryInput, error)
	FetchBalance(ctx context.Context, fetch FetchContext) (*balancesapp.RecordSnapshotInput, error)
	FetchPromotions(ctx context.Context, fetch FetchContext) ([]promotionsapp.RecordPromotionInput, error)
	FetchHealthSample(ctx context.Context, fetch FetchContext) (*healthapp.RecordSampleInput, error)
	ExportBills(ctx context.Context, request BillExportRequest) (*BillExportResult, error)
}
