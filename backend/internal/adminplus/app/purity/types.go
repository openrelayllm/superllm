package purity

import (
	"context"
	"time"

	coreservice "github.com/Wei-Shaw/sub2api/internal/service"
)

const (
	ProviderOpenAI    = "openai"
	ProviderAnthropic = "anthropic"

	CheckStatusPass = "pass"
	CheckStatusWarn = "warn"
	CheckStatusFail = "fail"

	RunStatusPending = "pending"
	RunStatusRunning = "running"
	RunStatusDone    = "done"
	RunStatusError   = "error"

	VerdictOfficialOpenAI       = "official_openai"
	VerdictOpenAICompatible     = "openai_compatible"
	VerdictOfficialClaude       = "official_claude"
	VerdictClaudeCompatible     = "claude_compatible"
	VerdictPartialCompatible    = "partial_compatible"
	VerdictInvalidOrUnavailable = "invalid_or_unavailable"
	VerdictUnknown              = "unknown"
)

type PublicCheckInput struct {
	Provider       string
	APIBaseURL     string
	APIKey         string
	ModelID        string
	ClientIP       string
	SkipTokenAudit bool
}

type AccountCheckInput struct {
	AccountID int64
	Provider  string
	ModelID   string
}

type AccountResolver interface {
	GetByID(ctx context.Context, id int64) (*coreservice.Account, error)
}

type PublicReport struct {
	Provider               string             `json:"provider"`
	ReportID               string             `json:"report_id"`
	APIBaseHost            string             `json:"api_base_host"`
	ModelID                string             `json:"model_id"`
	CheckTokenUsage        bool               `json:"checkTokenUsage"`
	ExpectedModel          string             `json:"expected_model,omitempty"`
	ExpectedModelCompat    string             `json:"expectedModel,omitempty"`
	ResponseModel          string             `json:"response_model,omitempty"`
	ResponseModelCompat    string             `json:"responseModel,omitempty"`
	Status                 string             `json:"status"`
	Step                   int                `json:"step"`
	StepName               string             `json:"step_name,omitempty"`
	StepNameCompat         string             `json:"stepName,omitempty"`
	Progress               float64            `json:"progress"`
	Scores                 map[string]int     `json:"scores,omitempty"`
	Score                  int                `json:"score"`
	Total                  int                `json:"total"`
	OfficialScore          int                `json:"official_score"`
	CompatibilityScore     int                `json:"compatibility_score"`
	Verdict                string             `json:"verdict"`
	VerdictKey             string             `json:"verdictKey,omitempty"`
	Summary                string             `json:"summary"`
	Error                  string             `json:"error,omitempty"`
	StreamChannel          string             `json:"stream_channel,omitempty"`
	StreamChannelCompat    string             `json:"streamChannel,omitempty"`
	NonStreamChannel       string             `json:"non_stream_channel,omitempty"`
	NonStreamChannelCompat string             `json:"nonStreamChannel,omitempty"`
	HasVertex              bool               `json:"has_vertex"`
	HasVertexCompat        bool               `json:"hasVertex"`
	IsKiro                 bool               `json:"is_kiro"`
	IsKiroCompat           bool               `json:"isKiro"`
	Validations            []ValidationResult `json:"validations"`
	Checks                 []CheckResult      `json:"checks"`
	Metrics                PublicCheckMetrics `json:"metrics"`
	TokenAudit             *TokenAuditReport  `json:"token_audit,omitempty"`
	TokenAuditProgress     string             `json:"token_audit_progress,omitempty"`
	TokenAuditPartial      []TokenAuditSample `json:"token_audit_partial,omitempty"`
	CheckedAt              time.Time          `json:"checked_at"`
}

const (
	PublicCheckEventStarted          = "started"
	PublicCheckEventProgress         = "progress"
	PublicCheckEventCheck            = "check"
	PublicCheckEventValidation       = "validation"
	PublicCheckEventMetrics          = "metrics"
	PublicCheckEventTokenAuditSample = "token_audit_sample"
	PublicCheckEventTokenAudit       = "token_audit"
	PublicCheckEventReport           = "report"
	PublicCheckEventError            = "error"
)

type PublicCheckEvent struct {
	Type               string              `json:"type"`
	ReportID           string              `json:"report_id,omitempty"`
	Status             string              `json:"status,omitempty"`
	Step               int                 `json:"step,omitempty"`
	StepName           string              `json:"step_name,omitempty"`
	StepNameCompat     string              `json:"stepName,omitempty"`
	Progress           float64             `json:"progress,omitempty"`
	Scores             map[string]int      `json:"scores,omitempty"`
	Check              *CheckResult        `json:"check,omitempty"`
	Validation         *ValidationResult   `json:"validation,omitempty"`
	Metrics            *PublicCheckMetrics `json:"metrics,omitempty"`
	Sample             *TokenAuditSample   `json:"sample,omitempty"`
	TokenAudit         *TokenAuditReport   `json:"token_audit,omitempty"`
	TokenAuditProgress string              `json:"token_audit_progress,omitempty"`
	TokenAuditPartial  []TokenAuditSample  `json:"token_audit_partial,omitempty"`
	Report             *PublicReport       `json:"report,omitempty"`
	ErrorClass         string              `json:"error_class,omitempty"`
	ErrorMessage       string              `json:"error_message,omitempty"`
}

type PublicCheckEventSink func(PublicCheckEvent)

type ValidationResult struct {
	ID              string         `json:"id"`
	Name            string         `json:"name"`
	Status          string         `json:"status"`
	Message         string         `json:"message"`
	RelatedCheckIDs []string       `json:"related_check_ids,omitempty"`
	Details         map[string]any `json:"details,omitempty"`
}

type CheckResult struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Status   string         `json:"status"`
	Score    int            `json:"score"`
	MaxScore int            `json:"max_score"`
	Message  string         `json:"message"`
	Details  map[string]any `json:"details,omitempty"`
}

type PublicCheckMetrics struct {
	ModelsLatencyMS          int64       `json:"models_latency_ms,omitempty"`
	ResponsesLatencyMS       int64       `json:"responses_latency_ms,omitempty"`
	MessagesLatencyMS        int64       `json:"messages_latency_ms,omitempty"`
	StreamFirstTokenMS       int64       `json:"stream_first_token_ms,omitempty"`
	StreamTotalLatencyMS     int64       `json:"stream_total_latency_ms,omitempty"`
	MultimodalLatencyMS      int64       `json:"multimodal_latency_ms,omitempty"`
	ChatCompletionsLatencyMS int64       `json:"chat_completions_latency_ms,omitempty"`
	LatencyMS                int64       `json:"latency_ms,omitempty"`
	TokensPerSecond          float64     `json:"tokens_per_second,omitempty"`
	LatencyMSCompat          int64       `json:"latencyMs,omitempty"`
	TTFBMSCompat             int64       `json:"ttfbMs,omitempty"`
	TokensPerSecondCompat    float64     `json:"tokensPerSec,omitempty"`
	InputTokensCompat        int64       `json:"inputTokens,omitempty"`
	OutputTokensCompat       int64       `json:"outputTokens,omitempty"`
	Usage                    *TokenUsage `json:"usage,omitempty"`
	ErrorClass               string      `json:"error_class,omitempty"`
	ErrorMessage             string      `json:"error_message,omitempty"`
}

type TokenUsage struct {
	InputTokens         int64 `json:"input_tokens"`
	OutputTokens        int64 `json:"output_tokens"`
	TotalTokens         int64 `json:"total_tokens"`
	CacheCreationTokens int64 `json:"cache_creation_tokens,omitempty"`
	CachedTokens        int64 `json:"cached_tokens,omitempty"`
	ReasoningTokens     int64 `json:"reasoning_tokens,omitempty"`
}

type TokenAuditReport struct {
	Status               string             `json:"status"`
	Summary              string             `json:"summary"`
	PriceSource          string             `json:"price_source"`
	OfficialBaselineUSD  float64            `json:"official_baseline_usd"`
	UncachedBaselineUSD  float64            `json:"uncached_baseline_usd,omitempty"`
	BaselineTotalCostUSD float64            `json:"baseline_total_cost_usd,omitempty"`
	BaselineTotalCost    float64            `json:"baselineTotalCost,omitempty"`
	ActualCostUSD        float64            `json:"actual_cost_usd"`
	TotalCostUSD         float64            `json:"total_cost,omitempty"`
	TotalCost            float64            `json:"totalCost,omitempty"`
	Multiplier           float64            `json:"multiplier"`
	OverallRatio         float64            `json:"overall_ratio,omitempty"`
	OverallRatioCompat   float64            `json:"overallRatio,omitempty"`
	CacheHitRate         float64            `json:"cache_hit_rate"`
	CacheHitRatePercent  float64            `json:"cacheHitRate,omitempty"`
	InputTokens          int64              `json:"input_tokens"`
	OutputTokens         int64              `json:"output_tokens"`
	CacheCreationTokens  int64              `json:"cache_creation_tokens"`
	CachedTokens         int64              `json:"cached_tokens"`
	SampleCount          int                `json:"sample_count"`
	PromptCacheKey       string             `json:"prompt_cache_key,omitempty"`
	StoreEnabled         bool               `json:"store_enabled,omitempty"`
	StatefulRounds       int                `json:"stateful_rounds,omitempty"`
	PreviousChainOK      bool               `json:"previous_response_chain_ok,omitempty"`
	Anomalies            []string           `json:"anomalies,omitempty"`
	Samples              []TokenAuditSample `json:"samples"`
	Rows                 []TokenAuditSample `json:"rows,omitempty"`
}

type TokenAuditSample struct {
	Index                    int     `json:"index"`
	Round                    int     `json:"round,omitempty"`
	InputTokens              int64   `json:"input_tokens"`
	BaselineInputTokens      int64   `json:"baseline_input_tokens,omitempty"`
	InputDeltaPct            float64 `json:"input_delta_pct,omitempty"`
	OutputTokens             int64   `json:"output_tokens"`
	BaselineOutputTokens     int64   `json:"baseline_output_tokens,omitempty"`
	OutputDeltaPct           float64 `json:"output_delta_pct,omitempty"`
	UncachedInputTokens      int64   `json:"uncached_input_tokens"`
	CacheCreationTokens      int64   `json:"cache_creation_tokens"`
	CacheCreationInputTokens int64   `json:"cache_creation_input_tokens,omitempty"`
	BaselineCacheCreation    int64   `json:"baseline_cache_creation_input_tokens,omitempty"`
	CacheCreationDeltaPct    float64 `json:"cache_creation_delta_pct,omitempty"`
	CachedTokens             int64   `json:"cached_tokens"`
	CacheReadInputTokens     int64   `json:"cache_read_input_tokens,omitempty"`
	BaselineCacheRead        int64   `json:"baseline_cache_read_input_tokens,omitempty"`
	CacheReadDeltaPct        float64 `json:"cache_read_delta_pct,omitempty"`
	ReasoningTokens          int64   `json:"reasoning_tokens,omitempty"`
	TotalTokens              int64   `json:"total_tokens"`
	OfficialBaselineUSD      float64 `json:"official_baseline_usd"`
	UncachedBaselineUSD      float64 `json:"uncached_baseline_usd,omitempty"`
	CacheDiscountUSD         float64 `json:"cache_discount_usd,omitempty"`
	BaselineCostUSD          float64 `json:"baseline_cost,omitempty"`
	ActualCostUSD            float64 `json:"actual_cost_usd"`
	CostUSD                  float64 `json:"cost,omitempty"`
	CostDeltaPct             float64 `json:"cost_delta_pct,omitempty"`
	Multiplier               float64 `json:"multiplier"`
	Ratio                    float64 `json:"ratio,omitempty"`
	LatencyMS                int64   `json:"latency_ms"`
	Status                   string  `json:"status"`
	ResponseID               string  `json:"response_id,omitempty"`
	PreviousResponseID       string  `json:"previous_response_id,omitempty"`
	PromptCacheKey           string  `json:"prompt_cache_key,omitempty"`
	Store                    bool    `json:"store,omitempty"`
	StateLinked              bool    `json:"state_linked,omitempty"`
}

type PublicReportRecord struct {
	RequestHash       string
	Provider          string
	APIBaseHost       string
	Report            *PublicReport
	PublicSummaryJSON map[string]any
}

type Repository interface {
	SavePublicReport(ctx context.Context, record PublicReportRecord) error
}
