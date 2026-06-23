package ports

import (
	"context"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
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

type DirectLoginInput struct {
	SupplierID   int64
	Origin       string
	APIBaseURL   string
	Username     string
	Password     string
	Token        string
	LoginContext map[string]any
}

type DirectLoginResult struct {
	SupplierID    int64
	Origin        string
	APIBaseURL    string
	SessionBundle map[string]any
	CapturedAt    time.Time
	ExpiresAt     *time.Time
	Diagnostics   map[string]any
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

type ChannelMonitorTimelinePoint struct {
	Status        string `json:"status"`
	LatencyMS     *int64 `json:"latency_ms,omitempty"`
	PingLatencyMS *int64 `json:"ping_latency_ms,omitempty"`
	CheckedAt     string `json:"checked_at"`
}

type ChannelMonitorExtraModel struct {
	Model     string `json:"model"`
	Status    string `json:"status"`
	LatencyMS *int64 `json:"latency_ms,omitempty"`
}

type ChannelMonitorView struct {
	ID                   int64                         `json:"id"`
	Name                 string                        `json:"name"`
	Provider             string                        `json:"provider"`
	GroupName            string                        `json:"group_name"`
	PrimaryModel         string                        `json:"primary_model"`
	PrimaryStatus        string                        `json:"primary_status"`
	PrimaryLatencyMS     *int64                        `json:"primary_latency_ms,omitempty"`
	PrimaryPingLatencyMS *int64                        `json:"primary_ping_latency_ms,omitempty"`
	Availability7D       float64                       `json:"availability_7d"`
	ExtraModels          []ChannelMonitorExtraModel    `json:"extra_models"`
	Timeline             []ChannelMonitorTimelinePoint `json:"timeline"`
}

type ReadChannelMonitorsResult struct {
	SupplierID int64                `json:"supplier_id"`
	SystemType string               `json:"system_type"`
	Origin     string               `json:"origin"`
	APIBaseURL string               `json:"api_base_url"`
	Items      []ChannelMonitorView `json:"items"`
	CapturedAt time.Time            `json:"captured_at"`
}

type SessionChannelMonitorAdapter interface {
	ReadChannelMonitors(ctx context.Context, in SessionProbeInput) (*ReadChannelMonitorsResult, error)
}

type SessionLoginAdapter interface {
	DirectLogin(ctx context.Context, in DirectLoginInput) (*DirectLoginResult, error)
}

type ProviderBalanceSnapshotInput struct {
	SupplierID               int64
	Source                   string
	RuntimeStatus            adminplusdomain.SupplierRuntimeStatus
	BalanceCents             int64
	Currency                 string
	LowBalanceThresholdCents int64
	RawPayload               map[string]any
	CapturedAt               *time.Time
}

type ProviderAnnouncement struct {
	Type             adminplusdomain.AnnouncementType
	Title            string
	Description      string
	Currency         string
	MinRechargeCents int64
	BonusPercent     *float64
	DiscountPercent  *float64
	RuntimeStatus    adminplusdomain.SupplierRuntimeStatus
	BalanceCents     int64
	StartsAt         *time.Time
	EndsAt           *time.Time
	RawPayload       map[string]any
}

type ReadAnnouncementsResult struct {
	SupplierID    int64
	SystemType    string
	Origin        string
	APIBaseURL    string
	Announcements []ProviderAnnouncement
	CapturedAt    time.Time
}

type SessionAnnouncementAdapter interface {
	ReadAnnouncements(ctx context.Context, in SessionProbeInput) (*ReadAnnouncementsResult, error)
}

type ProviderHealthSampleInput struct {
	SupplierID                   int64
	Source                       string
	Model                        string
	FirstTokenLatencyMS          int64
	TotalLatencyMS               int64
	StatusCode                   int
	ErrorClass                   string
	ObservedConcurrency          int
	AvailableConcurrency         *int
	ConcurrencyLimit             *int
	FirstTokenThresholdMS        int64
	TotalLatencyThresholdMS      int64
	ConcurrencySaturationPercent float64
	RawPayload                   map[string]any
	CapturedAt                   *time.Time
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

type CreateProviderKeyInput struct {
	SupplierID      int64
	ExternalGroupID string
	Name            string
	QuotaUSD        float64
	ExpiresInDays   *int
	Metadata        map[string]any
}

type RenameProviderKeyInput struct {
	SupplierID      int64
	ExternalKeyID   string
	ExternalGroupID string
	Name            string
	Metadata        map[string]any
}

type ProviderKeyResult struct {
	SupplierID      int64
	ExternalGroupID string
	ExternalKeyID   string
	Name            string
	Secret          string
	Status          string
	RawPayload      map[string]any
	CreatedAt       time.Time
}

type SessionKeyAdapter interface {
	CreateKey(ctx context.Context, in SessionProbeInput, request CreateProviderKeyInput) (*ProviderKeyResult, error)
	RenameKey(ctx context.Context, in SessionProbeInput, request RenameProviderKeyInput) (*ProviderKeyResult, error)
}

type ProviderRateEntry struct {
	Model       string
	BillingMode string
	PriceItem   string
	Unit        string
	Currency    string
	PriceMicros int64
	RawPayload  map[string]any
}

type ReadRatesResult struct {
	SupplierID int64
	SystemType string
	Origin     string
	APIBaseURL string
	Entries    []ProviderRateEntry
	CapturedAt time.Time
}

type SessionRateAdapter interface {
	ReadRates(ctx context.Context, in SessionProbeInput) (*ReadRatesResult, error)
}

type ProviderUsageCostLine struct {
	ExternalUsageCostID string
	ExternalRequestID   string
	APIKeyName          string
	Model               string
	Endpoint            string
	RequestType         string
	BillingMode         string
	ReasoningEffort     string
	Currency            string
	CostCents           int64
	InputTokens         int64
	OutputTokens        int64
	CacheReadTokens     int64
	TotalTokens         int64
	FirstTokenMS        int64
	DurationMS          int64
	UserAgent           string
	StartedAt           time.Time
	EndedAt             *time.Time
	RawPayload          map[string]any
}

type ProviderFundingTransaction struct {
	ExternalID        string
	OutTradeNo        string
	PaymentTradeNo    string
	PaymentType       string
	OrderType         string
	Status            string
	Currency          string
	AmountCents       int64
	CashAmountCents   int64
	RefundAmountCents int64
	FeeRate           *float64
	CreatedAtExternal *time.Time
	PaidAt            *time.Time
	CompletedAt       *time.Time
	RawPayload        map[string]any
}

type ProviderEntitlementTransaction struct {
	ExternalID        string
	CodeFingerprint   string
	CodeLast4         string
	SourceFamily      string
	Type              string
	Status            string
	Currency          string
	ValueCents        int64
	RawValue          float64
	GroupID           int64
	ValidityDays      int
	UsedAt            *time.Time
	CreatedAtExternal *time.Time
	RawPayload        map[string]any
}

type ReadUsageCostsInput struct {
	SupplierID int64
	StartedAt  time.Time
	EndedAt    time.Time
}

type ReadUsageCostsResult struct {
	SupplierID int64
	SystemType string
	Origin     string
	APIBaseURL string
	Lines      []ProviderUsageCostLine
	CapturedAt time.Time
}

type SessionUsageCostAdapter interface {
	ReadUsageCosts(ctx context.Context, in SessionProbeInput, request ReadUsageCostsInput) (*ReadUsageCostsResult, error)
}

type ReadFundingTransactionsInput struct {
	SupplierID int64
	StartedAt  *time.Time
	EndedAt    *time.Time
}

type ReadFundingTransactionsResult struct {
	SupplierID   int64
	ProviderType string
	SystemType   string
	Origin       string
	APIBaseURL   string
	Items        []ProviderFundingTransaction
	CapturedAt   time.Time
}

type ReadEntitlementTransactionsInput struct {
	SupplierID int64
	StartedAt  *time.Time
	EndedAt    *time.Time
}

type ReadEntitlementTransactionsResult struct {
	SupplierID   int64
	ProviderType string
	SystemType   string
	Origin       string
	APIBaseURL   string
	Items        []ProviderEntitlementTransaction
	CapturedAt   time.Time
}

type SessionFundingAdapter interface {
	ReadFundingTransactions(ctx context.Context, in SessionProbeInput, request ReadFundingTransactionsInput) (*ReadFundingTransactionsResult, error)
}

type SessionEntitlementAdapter interface {
	ReadEntitlementTransactions(ctx context.Context, in SessionProbeInput, request ReadEntitlementTransactionsInput) (*ReadEntitlementTransactionsResult, error)
}

type ProviderAdapter interface {
	Identity() ProviderIdentity
	FetchRateCatalog(ctx context.Context, fetch FetchContext) ([]ProviderRateEntry, error)
	FetchBalance(ctx context.Context, fetch FetchContext) (*ProviderBalanceSnapshotInput, error)
	FetchAnnouncements(ctx context.Context, fetch FetchContext) ([]ProviderAnnouncement, error)
	FetchHealthSample(ctx context.Context, fetch FetchContext) (*ProviderHealthSampleInput, error)
}
