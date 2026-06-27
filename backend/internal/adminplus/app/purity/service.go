package purity

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/httpclient"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	coreservice "github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
	"github.com/tidwall/gjson"
)

const (
	defaultCheckTimeout = 120 * time.Second
	defaultHTTPTimeout  = 45 * time.Second
	maxProbeBodyBytes   = 256 * 1024
	maxErrorMessageLen  = 500
	maxAPIKeyLength     = 8192
	tokenAuditSamples   = 11
	probePNGBase64      = "iVBORw0KGgoAAAANSUhEUgAAABAAAAAQCAIAAACQkWg2AAAAJ0lEQVR42mN44WOLFRm0/MeKGEY10ESD0tE4rOjXGVGsaFQDTTQAAIwskRBmlXeKAAAAAElFTkSuQmCC"
	probePNGData        = "data:image/png;base64," + probePNGBase64

	errorClassAccountBalanceInsufficient = "account_balance_insufficient"
)

type Service struct {
	repo              Repository
	accountResolver   AccountResolver
	httpClient        *http.Client
	accountHTTPClient *http.Client
	now               func() time.Time
	limiter           *publicLimiter
	allowPrivateHosts bool
}

func NewService(repo Repository) *Service {
	client := newPurityHTTPClient(false)
	accountClient := newPurityHTTPClient(true)
	return &Service{
		repo:              repo,
		httpClient:        client,
		accountHTTPClient: accountClient,
		now:               time.Now,
		limiter:           newPublicLimiter(),
	}
}

func newPurityHTTPClient(allowPrivate bool) *http.Client {
	client, err := httpclient.GetClient(httpclient.Options{
		Timeout:               defaultHTTPTimeout,
		ResponseHeaderTimeout: 15 * time.Second,
		ValidateResolvedIP:    true,
		AllowPrivateHosts:     allowPrivate,
		MaxConnsPerHost:       2,
	})
	if err != nil {
		return &http.Client{Timeout: defaultHTTPTimeout}
	}
	return client
}

func NewServiceWithAccountResolver(repo Repository, accountResolver AccountResolver) *Service {
	service := NewService(repo)
	service.accountResolver = accountResolver
	return service
}

func (s *Service) RunPublicCheck(ctx context.Context, in PublicCheckInput) (*PublicReport, error) {
	return s.runCheck(ctx, in, nil, checkRunOptions{
		EnforceRateLimit:  true,
		AllowPrivateHosts: s != nil && s.allowPrivateHosts,
		AccessMode:        AccessModeWeb,
		BillingMode:       BillingModeCaptchaRateLimit,
	})
}

func (s *Service) RunPublicCheckStream(ctx context.Context, in PublicCheckInput, emit PublicCheckEventSink) (*PublicReport, error) {
	return s.runCheck(ctx, in, emit, checkRunOptions{
		EnforceRateLimit:  true,
		AllowPrivateHosts: s != nil && s.allowPrivateHosts,
		AccessMode:        AccessModeWeb,
		BillingMode:       BillingModeCaptchaRateLimit,
	})
}

func (s *Service) RunDeveloperAPICheck(ctx context.Context, in PublicCheckInput) (*PublicReport, error) {
	return s.runCheck(ctx, in, nil, checkRunOptions{
		EnforceRateLimit:  false,
		AllowPrivateHosts: s != nil && s.allowPrivateHosts,
		AccessMode:        AccessModeDeveloperAPI,
		BillingMode:       BillingModeAPIKeyMetered,
	})
}

func (s *Service) RunDeveloperAPICheckStream(ctx context.Context, in PublicCheckInput, emit PublicCheckEventSink) (*PublicReport, error) {
	return s.runCheck(ctx, in, emit, checkRunOptions{
		EnforceRateLimit:  false,
		AllowPrivateHosts: s != nil && s.allowPrivateHosts,
		AccessMode:        AccessModeDeveloperAPI,
		BillingMode:       BillingModeAPIKeyMetered,
	})
}

func (s *Service) RunAccountCheck(ctx context.Context, in AccountCheckInput) (*PublicReport, error) {
	return s.RunAccountCheckStream(ctx, in, nil)
}

func (s *Service) RunAccountCheckStream(ctx context.Context, in AccountCheckInput, emit PublicCheckEventSink) (*PublicReport, error) {
	publicInput, err := s.publicInputFromAccount(ctx, in)
	if err != nil {
		return nil, err
	}
	return s.runCheck(ctx, publicInput, emit, checkRunOptions{
		EnforceRateLimit:  false,
		AllowPrivateHosts: true,
		AccessMode:        AccessModeAccount,
		BillingMode:       BillingModeAccountInternal,
	})
}

type checkRunOptions struct {
	EnforceRateLimit  bool
	AllowPrivateHosts bool
	AccessMode        string
	BillingMode       string
}

func (s *Service) runCheck(ctx context.Context, in PublicCheckInput, emit PublicCheckEventSink, options checkRunOptions) (*PublicReport, error) {
	if s == nil {
		return nil, infraerrors.InternalServer("PURITY_SERVICE_NOT_CONFIGURED", "purity service is not configured")
	}
	requestedProvider := strings.TrimSpace(in.Provider)
	provider := normalizeProvider(requestedProvider)
	if provider == ProviderAnthropic {
		return s.runClaudeCheck(ctx, in, emit, options)
	}
	if provider != ProviderOpenAI {
		return nil, infraerrors.BadRequest("PURITY_PROVIDER_UNSUPPORTED", "only OpenAI and Claude API key purity checks are supported")
	}
	apiKey := strings.TrimSpace(in.APIKey)
	if apiKey == "" {
		return nil, infraerrors.BadRequest("PURITY_API_KEY_REQUIRED", "api key is required")
	}
	if len(apiKey) > maxAPIKeyLength {
		return nil, infraerrors.BadRequest("PURITY_API_KEY_TOO_LARGE", "api key is too large")
	}
	if options.EnforceRateLimit {
		if err := s.enforceRateLimit(in.ClientIP, apiKey); err != nil {
			return nil, err
		}
	}

	model := strings.TrimSpace(in.ModelID)
	if model == "" {
		model = openai.DefaultTestModel
	}
	baseURL, host, officialHost, err := s.normalizeBaseURL(in.APIBaseURL, options.AllowPrivateHosts)
	if err != nil {
		return nil, err
	}
	client := s.clientForRun(options)

	startedAt := s.currentTime()
	checkCtx, cancel := context.WithTimeout(ctx, defaultCheckTimeout)
	defer cancel()

	checkedAt := startedAt.UTC()
	report := &PublicReport{
		Provider:        provider,
		ReportID:        newReportID(provider, host, model, checkedAt),
		AccessMode:      normalizeAccessMode(options.AccessMode),
		BillingMode:     normalizeBillingMode(options.BillingMode),
		APIBaseHost:     host,
		ModelID:         model,
		CheckTokenUsage: !in.SkipTokenAudit,
		ExpectedModel:   model,
		Status:          RunStatusRunning,
		Verdict:         VerdictUnknown,
		Validations:     []ValidationResult{},
		Checks:          []CheckResult{},
		CheckedAt:       checkedAt,
		Metrics:         PublicCheckMetrics{},
	}
	emitPublicCheckEvent(emit, PublicCheckEvent{
		Type:     PublicCheckEventStarted,
		ReportID: report.ReportID,
		Status:   report.Status,
		Report:   clonePublicReport(report),
	})
	emitProgress(report, emit, 1, "tag")
	baseURLCheck := buildBaseURLCheck(host, officialHost)
	appendAndEmitChecks(report, emit, baseURLCheck)

	modelsProbe := s.probeModels(checkCtx, client, baseURL, apiKey)
	report.Metrics.ModelsLatencyMS = modelsProbe.LatencyMS
	modelsCheck := buildModelsCheck(modelsProbe, model)
	appendAndEmitChecks(report, emit, modelsCheck)
	upsertAndEmitValidation(report, emit, buildLLMFingerprintValidation(baseURLCheck, modelsCheck))
	emitMetrics(report, emit)
	if shouldStopAfterProbe(modelsProbe) {
		markReportProbeError(report, modelsProbe, "基础连接或鉴权未通过")
		appendAndEmitChecks(report, emit, skippedCoreChecks("基础连接或鉴权未通过，后续探测未执行")...)
		upsertAndEmitValidation(report, emit, skippedValidation("schema_integrity", "结构完整性", []string{"responses_schema"}, "基础连接或鉴权未通过，结构完整性未执行"))
		upsertAndEmitValidation(report, emit, skippedValidation("behavior", "行为验证", []string{"tool_call", "streaming"}, "基础连接或鉴权未通过，行为验证未执行"))
		upsertAndEmitValidation(report, emit, skippedValidation("signature", "签名校验", []string{"usage"}, "基础连接或鉴权未通过，签名校验未执行"))
		upsertAndEmitValidation(report, emit, skippedValidation("multimodal", "多模态能力", []string{"multimodal"}, "基础连接或鉴权未通过，多模态探测未执行"))
		upsertAndEmitValidation(report, emit, skippedValidation("token_audit", "Token 用量审计", []string{"token_audit"}, "基础连接或鉴权未通过，Token 用量审计未执行"))
		report.Metrics.ErrorClass, report.Metrics.ErrorMessage = firstProbeError(report.Checks)
		report.Metrics.LatencyMS = int64(s.currentTime().Sub(startedAt) / time.Millisecond)
		s.finalizeAndSave(ctx, report, baseURL)
		emitFinalReport(report, emit)
		return report, nil
	}

	emitProgress(report, emit, 2, "structure")
	responsesProbe := s.probeResponsesNonStream(checkCtx, client, baseURL, apiKey, model)
	report.Metrics.ResponsesLatencyMS = responsesProbe.LatencyMS
	report.Metrics.Usage = parseResponsesUsage(responsesProbe.Body)
	report.ResponseModel = strings.TrimSpace(gjson.GetBytes(responsesProbe.Body, "model").String())
	report.NonStreamChannel = channelFromProbe(provider, host, officialHost, responsesProbe)
	responsesCheck := buildResponsesSchemaCheck(responsesProbe, apiKey)
	toolCheck := buildToolCallCheck(responsesProbe)
	usageCheck := buildUsageCheck(report.Metrics.Usage, responsesProbe)
	appendAndEmitChecks(report, emit, responsesCheck, toolCheck, usageCheck)
	upsertAndEmitValidation(report, emit, validationFromExecutedChecks("schema_integrity", "结构完整性", []CheckResult{responsesCheck}))
	emitMetrics(report, emit)

	emitProgress(report, emit, 3, "behavior")
	streamProbe := s.probeResponsesStream(checkCtx, client, baseURL, apiKey, model)
	report.Metrics.StreamFirstTokenMS = streamProbe.FirstTokenMS
	report.Metrics.StreamTotalLatencyMS = streamProbe.TotalLatencyMS
	report.StreamChannel = firstNonEmptyString(channelFromStreamProbe(provider, host, officialHost, streamProbe.Headers), report.NonStreamChannel)
	streamingCheck := buildStreamingCheck(streamProbe, apiKey)
	appendAndEmitChecks(report, emit, streamingCheck)
	report.Metrics.TokensPerSecond = tokensPerSecond(report.Metrics.Usage, streamProbe.TotalLatencyMS)
	upsertAndEmitValidation(report, emit, validationFromExecutedChecks("behavior", "行为验证", []CheckResult{toolCheck, streamingCheck}))
	emitMetrics(report, emit)

	emitProgress(report, emit, 4, "signature")
	upsertAndEmitValidation(report, emit, validationFromExecutedChecks("signature", "签名校验", []CheckResult{usageCheck}))
	emitMetrics(report, emit)

	emitProgress(report, emit, 5, "multimodal")
	multimodalProbe := s.probeResponsesMultimodal(checkCtx, client, baseURL, apiKey, model)
	report.Metrics.MultimodalLatencyMS = multimodalProbe.LatencyMS
	multimodalCheck := buildMultimodalCheck(multimodalProbe, apiKey)
	appendAndEmitChecks(report, emit, multimodalCheck)
	upsertAndEmitValidation(report, emit, validationFromExecutedChecks("multimodal", "多模态能力", []CheckResult{multimodalCheck}))
	emitMetrics(report, emit)

	if in.SkipTokenAudit {
		tokenAuditCheck := CheckResult{ID: "token_audit", Name: "Token 用量审计", Status: CheckStatusWarn, Score: 0, MaxScore: 15, Message: "本次请求已关闭 Token 用量审计。", Details: map[string]any{"skipped": true}}
		appendAndEmitChecks(report, emit, tokenAuditCheck)
		upsertAndEmitValidation(report, emit, validationFromExecutedChecks("token_audit", "Token 用量审计", []CheckResult{tokenAuditCheck}))
	} else if responsesProbe.StatusCode >= 200 && responsesProbe.StatusCode < 300 {
		emitProgress(report, emit, 6, "token_audit")
		report.TokenAudit = s.runTokenAudit(checkCtx, client, baseURL, apiKey, model, func(sample TokenAuditSample) {
			report.TokenAuditPartial = upsertTokenAuditPartial(report.TokenAuditPartial, sample)
			report.TokenAuditProgress = fmt.Sprintf("%d/%d", len(report.TokenAuditPartial), tokenAuditSamples)
			emitPublicCheckEvent(emit, PublicCheckEvent{
				Type:               PublicCheckEventTokenAuditSample,
				ReportID:           report.ReportID,
				Status:             report.Status,
				Step:               report.Step,
				StepName:           report.StepName,
				Progress:           report.Progress,
				Scores:             cloneScores(report.Scores),
				Sample:             &sample,
				TokenAuditProgress: report.TokenAuditProgress,
				TokenAuditPartial:  append([]TokenAuditSample(nil), report.TokenAuditPartial...),
			})
		})
		tokenAuditCheck := buildTokenAuditCheck(report.TokenAudit)
		appendAndEmitChecks(report, emit, tokenAuditCheck)
		upsertAndEmitValidation(report, emit, validationFromExecutedChecks("token_audit", "Token 用量审计", []CheckResult{tokenAuditCheck}))
		emitPublicCheckEvent(emit, PublicCheckEvent{
			Type:       PublicCheckEventTokenAudit,
			ReportID:   report.ReportID,
			TokenAudit: report.TokenAudit,
		})
	} else {
		tokenAuditCheck := failCheck("token_audit", "Token 用量审计", 15, "Responses 非流式探测未通过，Token 用量审计未执行。", nil)
		appendAndEmitChecks(report, emit, tokenAuditCheck)
		upsertAndEmitValidation(report, emit, validationFromExecutedChecks("token_audit", "Token 用量审计", []CheckResult{tokenAuditCheck}))
	}

	if needsChatFallback(report.Checks) {
		chatProbe := s.probeChatCompletions(checkCtx, client, baseURL, apiKey, model)
		report.Metrics.ChatCompletionsLatencyMS = chatProbe.LatencyMS
		appendAndEmitChecks(report, emit, buildChatFallbackCheck(chatProbe, apiKey))
	}

	report.HasVertex = hasVertexFingerprint(report.APIBaseHost, responsesProbe.Headers, streamProbe.Headers)
	report.IsKiro = hasKiroFingerprint(report.APIBaseHost, responsesProbe.Headers, streamProbe.Headers)
	emitProgress(report, emit, 7, "evaluate")
	report.Metrics.LatencyMS = int64(s.currentTime().Sub(startedAt) / time.Millisecond)
	s.finalizeAndSave(ctx, report, baseURL)
	emitFinalReport(report, emit)
	return report, nil
}

func (s *Service) clientForRun(options checkRunOptions) *http.Client {
	if s == nil {
		return &http.Client{Timeout: defaultHTTPTimeout}
	}
	if options.AllowPrivateHosts && s.accountHTTPClient != nil {
		return s.accountHTTPClient
	}
	if s.httpClient != nil {
		return s.httpClient
	}
	return &http.Client{Timeout: defaultHTTPTimeout}
}

func (s *Service) publicInputFromAccount(ctx context.Context, in AccountCheckInput) (PublicCheckInput, error) {
	if s == nil {
		return PublicCheckInput{}, infraerrors.InternalServer("PURITY_SERVICE_NOT_CONFIGURED", "purity service is not configured")
	}
	if s.accountResolver == nil {
		return PublicCheckInput{}, infraerrors.InternalServer("PURITY_ACCOUNT_RESOLVER_NOT_CONFIGURED", "account resolver is not configured")
	}
	if in.AccountID <= 0 {
		return PublicCheckInput{}, infraerrors.BadRequest("PURITY_ACCOUNT_ID_INVALID", "invalid account id")
	}
	requestedProvider := strings.TrimSpace(in.Provider)
	provider := normalizeProvider(requestedProvider)
	account, err := s.accountResolver.GetByID(ctx, in.AccountID)
	if err != nil {
		return PublicCheckInput{}, err
	}
	if account == nil {
		return PublicCheckInput{}, infraerrors.BadRequest("PURITY_ACCOUNT_UNSUPPORTED", "unsupported account")
	}
	if requestedProvider == "" {
		provider = normalizeProvider(account.Platform)
	}
	if provider == ProviderOpenAI && !account.IsOpenAIApiKey() {
		return PublicCheckInput{}, infraerrors.BadRequest("PURITY_ACCOUNT_UNSUPPORTED", "only OpenAI API key accounts can run purity checks")
	}
	if provider == ProviderAnthropic && (account.Platform != coreservice.PlatformAnthropic || account.Type != coreservice.AccountTypeAPIKey) {
		return PublicCheckInput{}, infraerrors.BadRequest("PURITY_ACCOUNT_UNSUPPORTED", "only Claude API key accounts can run purity checks")
	}
	if provider != ProviderOpenAI && provider != ProviderAnthropic {
		return PublicCheckInput{}, infraerrors.BadRequest("PURITY_PROVIDER_UNSUPPORTED", "only OpenAI and Claude API key purity checks are supported")
	}
	apiKey := account.GetOpenAIApiKey()
	baseURL := account.GetOpenAIBaseURL()
	if provider == ProviderAnthropic {
		apiKey = account.GetCredential("api_key")
		baseURL = account.GetBaseURL()
	}
	if strings.TrimSpace(apiKey) == "" {
		return PublicCheckInput{}, infraerrors.BadRequest("PURITY_ACCOUNT_API_KEY_MISSING", "account api key is missing")
	}
	return PublicCheckInput{
		Provider:   provider,
		APIBaseURL: baseURL,
		APIKey:     apiKey,
		ModelID:    in.ModelID,
		ClientIP:   fmt.Sprintf("account:%d", in.AccountID),
	}, nil
}

func normalizeProvider(provider string) string {
	value := strings.ToLower(strings.TrimSpace(provider))
	if value == "" {
		return ProviderOpenAI
	}
	if value == "claude" {
		return ProviderAnthropic
	}
	return value
}

func normalizeAccessMode(mode string) string {
	switch strings.TrimSpace(mode) {
	case AccessModeDeveloperAPI:
		return AccessModeDeveloperAPI
	case AccessModeAccount:
		return AccessModeAccount
	default:
		return AccessModeWeb
	}
}

func normalizeBillingMode(mode string) string {
	switch strings.TrimSpace(mode) {
	case BillingModeAPIKeyMetered:
		return BillingModeAPIKeyMetered
	case BillingModeAccountInternal:
		return BillingModeAccountInternal
	default:
		return BillingModeCaptchaRateLimit
	}
}

func (s *Service) normalizeBaseURL(raw string, allowPrivate bool) (string, string, bool, error) {
	normalized, err := urlvalidator.ValidateHTTPURL(raw, true, urlvalidator.ValidationOptions{
		AllowPrivate: allowPrivate,
	})
	if err != nil {
		return "", "", false, infraerrors.BadRequest("PURITY_BASE_URL_INVALID", "invalid api base url")
	}
	parsed, err := url.Parse(normalized)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", "", false, infraerrors.BadRequest("PURITY_BASE_URL_INVALID", "invalid api base url")
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	parsed.Path = normalizeOpenAIBasePath(parsed.EscapedPath())
	parsed.RawPath = ""

	host := strings.ToLower(parsed.Host)
	officialHost := strings.EqualFold(parsed.Hostname(), "api.openai.com")
	return strings.TrimRight(parsed.String(), "/"), host, officialHost, nil
}

func normalizeOpenAIBasePath(path string) string {
	value := strings.TrimRight(strings.TrimSpace(path), "/")
	if value == "" {
		return ""
	}
	for _, suffix := range []string{"/v1/chat/completions", "/chat/completions", "/v1/responses", "/responses", "/v1/models", "/models"} {
		if strings.HasSuffix(value, suffix) {
			value = strings.TrimRight(strings.TrimSuffix(value, suffix), "/")
			break
		}
	}
	return value
}

func (s *Service) currentTime() time.Time {
	if s != nil && s.now != nil {
		return s.now()
	}
	return time.Now()
}

func (s *Service) enforceRateLimit(clientIP string, apiKey string) error {
	if s == nil || s.limiter == nil {
		return nil
	}
	keyHash := sha256Hex(apiKey)
	return s.limiter.allow(s.currentTime(), clientIP, keyHash)
}

func (s *Service) finalizeAndSave(ctx context.Context, report *PublicReport, baseURL string) {
	finalizeReport(report)
	if s == nil || s.repo == nil || report == nil {
		return
	}
	_ = s.repo.SavePublicReport(ctx, PublicReportRecord{
		RequestHash:       reportHash(report, baseURL),
		Provider:          report.Provider,
		APIBaseHost:       report.APIBaseHost,
		Report:            report,
		PublicSummaryJSON: buildPublicSummary(report),
	})
}

func finalizeReport(report *PublicReport) {
	if report == nil {
		return
	}
	totalScore := 0
	totalMax := 0
	capabilityScore := 0
	capabilityMax := 0
	for _, check := range report.Checks {
		if check.MaxScore <= 0 {
			continue
		}
		totalScore += check.Score
		totalMax += check.MaxScore
		if check.ID != "base_url" {
			capabilityScore += check.Score
			capabilityMax += check.MaxScore
		}
	}
	report.Score = percent(totalScore, totalMax)
	report.OfficialScore = report.Score
	report.CompatibilityScore = percent(capabilityScore, capabilityMax)
	report.Verdict = decideVerdict(report)
	report.Summary = summaryForVerdict(report.Verdict)
	finalizeValidations(report)
	report.Scores = scoreBreakdown(report)
	if report.Status == RunStatusError && report.Metrics.ErrorClass == "" {
		report.Metrics.ErrorClass, report.Metrics.ErrorMessage = firstProbeError(report.Checks)
	}
	if report.Status == RunStatusError && report.Error == "" {
		report.Error = firstNonEmptyString(report.Metrics.ErrorMessage, "请求目标 API 失败")
	}
	if report.Status == RunStatusError {
		report.Score = 0
		report.OfficialScore = 0
		report.CompatibilityScore = 0
		report.Verdict = VerdictInvalidOrUnavailable
		report.Summary = summaryForReportError(report)
		report.Scores = scoreBreakdown(report)
	}
	syncReportCompat(report)
}

func syncReportCompat(report *PublicReport) {
	if report == nil {
		return
	}
	report.StepNameCompat = report.StepName
	report.AccessModeCompat = report.AccessMode
	report.BillingModeCompat = report.BillingMode
	report.Total = report.Score
	report.VerdictKey = cctestVerdictKey(report.Verdict)
	report.ExpectedModelCompat = report.ExpectedModel
	report.ResponseModelCompat = report.ResponseModel
	report.StreamChannelCompat = report.StreamChannel
	report.NonStreamChannelCompat = report.NonStreamChannel
	report.HasVertexCompat = report.HasVertex
	report.IsKiroCompat = report.IsKiro
	syncMetricsCompat(&report.Metrics)
}

func syncMetricsCompat(metrics *PublicCheckMetrics) {
	if metrics == nil {
		return
	}
	metrics.LatencyMSCompat = metrics.LatencyMS
	metrics.TTFBMSCompat = metrics.StreamFirstTokenMS
	metrics.TokensPerSecondCompat = metrics.TokensPerSecond
	if metrics.Usage != nil {
		metrics.InputTokensCompat = metrics.Usage.InputTokens
		metrics.OutputTokensCompat = metrics.Usage.OutputTokens
	}
}

func cctestVerdictKey(verdict string) string {
	switch verdict {
	case VerdictOfficialOpenAI, VerdictOfficialClaude:
		return "official"
	case VerdictOpenAICompatible, VerdictClaudeCompatible:
		return "compatible"
	case VerdictPartialCompatible:
		return "partial"
	case VerdictInvalidOrUnavailable:
		return "invalid"
	case VerdictUnknown, "":
		return "unknown"
	default:
		return verdict
	}
}

func newReportID(provider string, host string, model string, checkedAt time.Time) string {
	raw := strings.Join([]string{provider, host, model, checkedAt.UTC().Format(time.RFC3339Nano)}, "\x00")
	hash := sha256Hex(raw)
	if len(hash) > 36 {
		hash = hash[:36]
	}
	return hash
}

func decideVerdict(report *PublicReport) string {
	if report == nil {
		return VerdictUnknown
	}
	officialHost := false
	responsesOK := false
	messagesOK := false
	toolOK := false
	streamOK := false
	chatOK := false
	for _, check := range report.Checks {
		switch check.ID {
		case "base_url":
			officialHost = check.Status == CheckStatusPass
		case "responses_schema":
			responsesOK = check.Status == CheckStatusPass
		case "claude_messages_schema":
			messagesOK = check.Status == CheckStatusPass
		case "tool_call":
			toolOK = check.Status == CheckStatusPass
		case "claude_tool_use":
			toolOK = check.Status == CheckStatusPass
		case "streaming":
			streamOK = check.Status == CheckStatusPass
		case "claude_streaming":
			streamOK = check.Status == CheckStatusPass
		case "chat_completions":
			chatOK = check.Status == CheckStatusPass
		}
	}
	if report.Provider == ProviderAnthropic {
		if officialHost && messagesOK && toolOK && streamOK && report.Score >= 85 {
			return VerdictOfficialClaude
		}
		if report.CompatibilityScore >= 80 && messagesOK && streamOK {
			return VerdictClaudeCompatible
		}
		if report.CompatibilityScore >= 50 {
			return VerdictPartialCompatible
		}
		return VerdictInvalidOrUnavailable
	}
	if officialHost && responsesOK && toolOK && streamOK && report.Score >= 85 {
		return VerdictOfficialOpenAI
	}
	if report.CompatibilityScore >= 80 && responsesOK && streamOK {
		return VerdictOpenAICompatible
	}
	if report.CompatibilityScore >= 50 || chatOK {
		return VerdictPartialCompatible
	}
	return VerdictInvalidOrUnavailable
}

func summaryForVerdict(verdict string) string {
	switch verdict {
	case VerdictOfficialOpenAI:
		return "该接口表现为 OpenAI 官方协议与模型行为，Responses、工具调用和流式事件均通过。"
	case VerdictOpenAICompatible:
		return "该接口 OpenAI 兼容能力较完整，仍建议结合 Token 用量审计复核。"
	case VerdictOfficialClaude:
		return "该接口表现为 Anthropic Claude 官方协议与模型行为，Messages、工具调用、流式事件和 usage 均通过。"
	case VerdictClaudeCompatible:
		return "该接口 Claude Messages 兼容能力较完整，仍建议结合 Token 用量审计复核。"
	case VerdictPartialCompatible:
		return "该接口只通过部分兼容检查，生产前需确认模型、工具调用和流式行为。"
	case VerdictInvalidOrUnavailable:
		return "该接口未通过基础鉴权或协议检查。"
	default:
		return "该接口检测结果暂无法判断。"
	}
}

func summaryForReportError(report *PublicReport) string {
	if report != nil && report.Metrics.ErrorClass == errorClassAccountBalanceInsufficient {
		return "账号余额不足，无法完成纯度检测；请充值或切换有余额的账号后重试。"
	}
	return summaryForVerdict(VerdictInvalidOrUnavailable)
}

func markReportProbeError(report *PublicReport, probe httpProbe, fallback string) {
	if report == nil {
		return
	}
	report.Status = RunStatusError
	report.Step = 1
	report.StepName = "tag"
	report.Progress = roundProgress(1.0 / 7.0)
	report.Scores = scoreBreakdown(report)
	report.Error = targetAPIErrorMessage(probe, fallback)
	syncReportCompat(report)
}

func targetAPIErrorMessage(probe httpProbe, fallback string) string {
	message := strings.TrimSpace(probe.ErrorMessage)
	if message == "" {
		message = strings.TrimSpace(fallback)
	}
	if message == "" {
		message = "请求目标 API 失败"
	}
	if probe.ErrorClass == errorClassAccountBalanceInsufficient {
		message = "账号余额不足，无法完成纯度检测；请充值或切换有余额的账号后重试。"
	}
	if probe.StatusCode > 0 {
		return fmt.Sprintf("请求目标 API 失败: %d %s", probe.StatusCode, message)
	}
	return "请求目标 API 失败: " + message
}

func buildBaseURLCheck(host string, officialHost bool) CheckResult {
	if officialHost {
		return CheckResult{
			ID:       "base_url",
			Name:     "API Base 域名",
			Status:   CheckStatusPass,
			Score:    20,
			MaxScore: 20,
			Message:  "命中 OpenAI 官方 API 域名。",
			Details:  map[string]any{"host": host, "official_host": true},
		}
	}
	return CheckResult{
		ID:       "base_url",
		Name:     "API Base 域名",
		Status:   CheckStatusPass,
		Score:    20,
		MaxScore: 20,
		Message:  "当前为 OpenAI 兼容入口，域名仅作路由记录，纯度由后续探针判断。",
		Details:  map[string]any{"host": host, "official_host": false},
	}
}

type httpProbe struct {
	StatusCode   int
	Body         []byte
	Headers      map[string]string
	LatencyMS    int64
	ErrorClass   string
	ErrorMessage string
}

func (s *Service) probeModels(ctx context.Context, client *http.Client, baseURL string, apiKey string) httpProbe {
	return s.doJSON(ctx, client, http.MethodGet, buildOpenAIEndpointURL(baseURL, "/v1/models"), apiKey, nil, "application/json")
}

func (s *Service) probeResponsesNonStream(ctx context.Context, client *http.Client, baseURL string, apiKey string, model string) httpProbe {
	return s.doJSON(ctx, client, http.MethodPost, buildOpenAIEndpointURL(baseURL, "/v1/responses"), apiKey, responsesToolProbePayload(model, false), "application/json")
}

func (s *Service) probeChatCompletions(ctx context.Context, client *http.Client, baseURL string, apiKey string, model string) httpProbe {
	body, _ := json.Marshal(map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": "Return exactly: ok"},
		},
		"stream": false,
	})
	return s.doJSON(ctx, client, http.MethodPost, buildOpenAIEndpointURL(baseURL, "/v1/chat/completions"), apiKey, body, "application/json")
}

func (s *Service) probeResponsesMultimodal(ctx context.Context, client *http.Client, baseURL string, apiKey string, model string) httpProbe {
	return s.doJSON(ctx, client, http.MethodPost, buildOpenAIEndpointURL(baseURL, "/v1/responses"), apiKey, responsesMultimodalProbePayload(model), "application/json")
}

func (s *Service) runTokenAudit(ctx context.Context, client *http.Client, baseURL string, apiKey string, model string, emitSample func(TokenAuditSample)) *TokenAuditReport {
	pricing := openAIModelPricingFor(model)
	auditNonce := newAuditNonce(model, s.currentTime())
	promptCacheKey := openAITokenAuditPromptCacheKey(model, auditNonce)
	report := &TokenAuditReport{
		Status:         CheckStatusWarn,
		Summary:        "Token 用量审计样本不足。",
		PriceSource:    pricing.Source,
		PromptCacheKey: promptCacheKey,
		StoreEnabled:   true,
		Samples:        make([]TokenAuditSample, 0, tokenAuditSamples),
	}
	var totalOfficial float64
	var totalActual float64
	var totalUncached float64
	previousResponseID := ""
	for i := 1; i <= tokenAuditSamples; i++ {
		select {
		case <-ctx.Done():
			report.Summary = "Token 用量审计超时，已保留完成样本。"
			finalizeOpenAITokenAudit(report)
			return report
		default:
		}
		probe := s.probeResponsesAudit(ctx, client, baseURL, apiKey, model, i, auditNonce, previousResponseID, promptCacheKey)
		sample := tokenAuditSampleFromProbe(i, probe, pricing, previousResponseID, promptCacheKey, true)
		report.Samples = append(report.Samples, sample)
		if emitSample != nil {
			emitSample(sample)
		}
		if sample.Status != CheckStatusPass {
			continue
		}
		report.InputTokens += sample.InputTokens
		report.OutputTokens += sample.OutputTokens
		report.CacheCreationTokens += sample.CacheCreationTokens
		report.CachedTokens += sample.CachedTokens
		totalOfficial += sample.OfficialBaselineUSD
		totalActual += sample.ActualCostUSD
		totalUncached += sample.UncachedBaselineUSD
		if sample.StateLinked {
			report.StatefulRounds++
		}
		if sample.ResponseID != "" {
			previousResponseID = sample.ResponseID
		}
	}
	report.OfficialBaselineUSD = roundMoney(totalOfficial)
	report.ActualCostUSD = roundMoney(totalActual)
	report.UncachedBaselineUSD = roundMoney(totalUncached)
	finalizeOpenAITokenAudit(report)
	return report
}

func (s *Service) probeResponsesAudit(ctx context.Context, client *http.Client, baseURL string, apiKey string, model string, round int, auditNonce string, previousResponseID string, promptCacheKey string) httpProbe {
	body := responsesAuditProbePayload(model, round, auditNonce, previousResponseID, promptCacheKey)
	return s.doJSON(ctx, client, http.MethodPost, buildOpenAIEndpointURL(baseURL, "/v1/responses"), apiKey, body, "application/json")
}

func responsesAuditProbePayload(model string, round int, auditNonce string, previousResponseID string, promptCacheKey string) []byte {
	if strings.TrimSpace(model) == "" {
		model = openai.DefaultTestModel
	}
	bodyMap := map[string]any{
		"model":        model,
		"instructions": openAITokenAuditInstructions(auditNonce),
		"input": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{"type": "input_text", "text": openAITokenAuditRoundInput(round, auditNonce)},
				},
			},
		},
		"tools": []map[string]any{
			openAITokenAuditToolDefinition(),
		},
		"tool_choice":         "auto",
		"parallel_tool_calls": true,
		"prompt_cache_key":    promptCacheKey,
		"store":               true,
		"max_output_tokens":   tokenAuditOutputBudget(round),
		"stream":              false,
	}
	if strings.TrimSpace(previousResponseID) != "" {
		bodyMap["previous_response_id"] = strings.TrimSpace(previousResponseID)
	}
	body, _ := json.Marshal(bodyMap)
	return body
}

func (s *Service) doJSON(ctx context.Context, client *http.Client, method string, endpoint string, apiKey string, body []byte, accept string) httpProbe {
	headers := map[string]string{
		"Accept":        accept,
		"Authorization": "Bearer " + apiKey,
	}
	if body != nil {
		headers["Content-Type"] = "application/json"
	}
	return s.doJSONWithHeaders(ctx, client, method, endpoint, body, headers, apiKey)
}

func (s *Service) doJSONWithHeaders(ctx context.Context, client *http.Client, method string, endpoint string, body []byte, headers map[string]string, secret string) httpProbe {
	started := s.currentTime()
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return httpProbe{ErrorClass: "request_build_error", ErrorMessage: sanitizeMessage(err.Error(), secret)}
	}
	for key, value := range headers {
		if strings.TrimSpace(key) == "" || value == "" {
			continue
		}
		req.Header.Set(key, value)
	}
	if client == nil {
		client = s.clientForRun(checkRunOptions{})
	}
	resp, err := client.Do(req)
	if err != nil {
		return httpProbe{
			LatencyMS:    int64(s.currentTime().Sub(started) / time.Millisecond),
			ErrorClass:   "network_error",
			ErrorMessage: sanitizeMessage(err.Error(), secret),
		}
	}
	defer func() { _ = resp.Body.Close() }()
	bodyBytes, readErr := io.ReadAll(io.LimitReader(resp.Body, maxProbeBodyBytes))
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxProbeBodyBytes))
	result := httpProbe{
		StatusCode: resp.StatusCode,
		Body:       bodyBytes,
		Headers:    selectedResponseHeaders(resp.Header),
		LatencyMS:  int64(s.currentTime().Sub(started) / time.Millisecond),
	}
	if readErr != nil {
		result.ErrorClass = "read_error"
		result.ErrorMessage = sanitizeMessage(readErr.Error(), secret)
		return result
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errorMessage := upstreamErrorMessage(bodyBytes)
		result.ErrorClass = errorClassForStatusAndMessage(resp.StatusCode, errorMessage)
		result.ErrorMessage = sanitizeMessage(errorMessage, secret)
	}
	return result
}

type openAIModelPricing struct {
	InputPerToken     float64
	OutputPerToken    float64
	CacheReadPerToken float64
	Source            string
}

func openAIModelPricingFor(model string) openAIModelPricing {
	key := strings.ToLower(strings.TrimSpace(model))
	table := openAIModelPricingTable()
	if pricing, ok := table[key]; ok {
		return pricing
	}
	var best openAIModelPricing
	bestLen := 0
	for prefix, pricing := range table {
		if strings.HasPrefix(key, prefix) && len(prefix) > bestLen {
			best = pricing
			bestLen = len(prefix)
		}
	}
	if bestLen > 0 {
		return best
	}
	return openAIModelPricing{
		InputPerToken:     2.5e-6,
		OutputPerToken:    15e-6,
		CacheReadPerToken: 0.25e-6,
		Source:            "Official OpenAI API pricing, Standard tier direct API, default gpt-5.5 baseline, verified 2026-06-27",
	}
}

func openAIModelPricingTable() map[string]openAIModelPricing {
	return map[string]openAIModelPricing{
		"gpt-5.5": {
			InputPerToken:     5e-6,
			OutputPerToken:    30e-6,
			CacheReadPerToken: 0.5e-6,
			Source:            "Official OpenAI API pricing, Standard tier direct API, gpt-5.5, verified 2026-06-27",
		},
		"gpt-5.5-pro": {
			InputPerToken:     30e-6,
			OutputPerToken:    180e-6,
			CacheReadPerToken: 30e-6,
			Source:            "Official OpenAI API pricing, Standard tier direct API, gpt-5.5-pro, cached input not separately listed, verified 2026-06-27",
		},
		"gpt-5.4-pro": {
			InputPerToken:     30e-6,
			OutputPerToken:    180e-6,
			CacheReadPerToken: 30e-6,
			Source:            "Official OpenAI API pricing, Standard tier direct API, gpt-5.4-pro, cached input not separately listed, verified 2026-06-27",
		},
		"gpt-5.4-mini": {
			InputPerToken:     0.75e-6,
			OutputPerToken:    4.5e-6,
			CacheReadPerToken: 0.075e-6,
			Source:            "Official OpenAI API pricing, Standard tier direct API, gpt-5.4-mini, verified 2026-06-27",
		},
		"gpt-5.4-nano": {
			InputPerToken:     0.2e-6,
			OutputPerToken:    1.25e-6,
			CacheReadPerToken: 0.02e-6,
			Source:            "Official OpenAI API pricing, Standard tier direct API, gpt-5.4-nano, verified 2026-06-27",
		},
		"gpt-5.4": {
			InputPerToken:     2.5e-6,
			OutputPerToken:    15e-6,
			CacheReadPerToken: 0.25e-6,
			Source:            "Official OpenAI API pricing, Standard tier direct API, gpt-5.4, verified 2026-06-27",
		},
		"gpt-5.2": {
			InputPerToken:     1.75e-6,
			OutputPerToken:    14e-6,
			CacheReadPerToken: 0.175e-6,
			Source:            "Official OpenAI API pricing, Standard tier direct API, gpt-5.2, verified 2026-06-27",
		},
		"gpt-5.2-pro": {
			InputPerToken:     21e-6,
			OutputPerToken:    168e-6,
			CacheReadPerToken: 21e-6,
			Source:            "Official OpenAI API pricing, Standard tier direct API, gpt-5.2-pro, cached input not separately listed, verified 2026-06-27",
		},
		"gpt-5.1": {
			InputPerToken:     1.25e-6,
			OutputPerToken:    10e-6,
			CacheReadPerToken: 0.125e-6,
			Source:            "Official OpenAI API pricing, Standard tier direct API, gpt-5.1, verified 2026-06-27",
		},
		"gpt-5-mini": {
			InputPerToken:     0.25e-6,
			OutputPerToken:    2e-6,
			CacheReadPerToken: 0.025e-6,
			Source:            "Official OpenAI API pricing, Standard tier direct API, gpt-5-mini, verified 2026-06-27",
		},
		"gpt-5-nano": {
			InputPerToken:     0.05e-6,
			OutputPerToken:    0.4e-6,
			CacheReadPerToken: 0.005e-6,
			Source:            "Official OpenAI API pricing, Standard tier direct API, gpt-5-nano, verified 2026-06-27",
		},
		"gpt-5-pro": {
			InputPerToken:     15e-6,
			OutputPerToken:    120e-6,
			CacheReadPerToken: 15e-6,
			Source:            "Official OpenAI API pricing, Standard tier direct API, gpt-5-pro, cached input not separately listed, verified 2026-06-27",
		},
		"gpt-5": {
			InputPerToken:     1.25e-6,
			OutputPerToken:    10e-6,
			CacheReadPerToken: 0.125e-6,
			Source:            "Official OpenAI API pricing, Standard tier direct API, gpt-5, verified 2026-06-27",
		},
		"chat-latest": {
			InputPerToken:     5e-6,
			OutputPerToken:    30e-6,
			CacheReadPerToken: 0.5e-6,
			Source:            "Official OpenAI API pricing, Standard tier direct API, chat-latest, verified 2026-06-27",
		},
		"gpt-5.3-chat-latest": {
			InputPerToken:     1.75e-6,
			OutputPerToken:    14e-6,
			CacheReadPerToken: 0.175e-6,
			Source:            "Official OpenAI API pricing, Standard tier direct API, gpt-5.3-chat-latest, verified 2026-06-27",
		},
		"gpt-5.2-chat-latest": {
			InputPerToken:     1.75e-6,
			OutputPerToken:    14e-6,
			CacheReadPerToken: 0.175e-6,
			Source:            "Official OpenAI API pricing, Standard tier direct API, gpt-5.2-chat-latest, verified 2026-06-27",
		},
		"gpt-5.1-chat-latest": {
			InputPerToken:     1.25e-6,
			OutputPerToken:    10e-6,
			CacheReadPerToken: 0.125e-6,
			Source:            "Official OpenAI API pricing, Standard tier direct API, gpt-5.1-chat-latest, verified 2026-06-27",
		},
		"gpt-5-chat-latest": {
			InputPerToken:     1.25e-6,
			OutputPerToken:    10e-6,
			CacheReadPerToken: 0.125e-6,
			Source:            "Official OpenAI API pricing, Standard tier direct API, gpt-5-chat-latest, verified 2026-06-27",
		},
		"gpt-5.3-codex": {
			InputPerToken:     1.75e-6,
			OutputPerToken:    14e-6,
			CacheReadPerToken: 0.175e-6,
			Source:            "Official OpenAI API pricing, Standard tier direct API, gpt-5.3-codex, verified 2026-06-27",
		},
		"gpt-5.2-codex": {
			InputPerToken:     1.75e-6,
			OutputPerToken:    14e-6,
			CacheReadPerToken: 0.175e-6,
			Source:            "Official OpenAI API pricing, Standard tier direct API, gpt-5.2-codex, verified 2026-06-27",
		},
		"gpt-5.1-codex-max": {
			InputPerToken:     1.25e-6,
			OutputPerToken:    10e-6,
			CacheReadPerToken: 0.125e-6,
			Source:            "Official OpenAI API pricing, Standard tier direct API, gpt-5.1-codex-max, verified 2026-06-27",
		},
		"gpt-5.1-codex": {
			InputPerToken:     1.25e-6,
			OutputPerToken:    10e-6,
			CacheReadPerToken: 0.125e-6,
			Source:            "Official OpenAI API pricing, Standard tier direct API, gpt-5.1-codex, verified 2026-06-27",
		},
		"gpt-5-codex": {
			InputPerToken:     1.25e-6,
			OutputPerToken:    10e-6,
			CacheReadPerToken: 0.125e-6,
			Source:            "Official OpenAI API pricing, Standard tier direct API, gpt-5-codex, verified 2026-06-27",
		},
	}
}

func tokenAuditSampleFromProbe(index int, probe httpProbe, pricing openAIModelPricing, previousResponseID string, promptCacheKey string, store bool) TokenAuditSample {
	sample := TokenAuditSample{
		Index:              index,
		Round:              index,
		LatencyMS:          probe.LatencyMS,
		Status:             CheckStatusFail,
		PreviousResponseID: strings.TrimSpace(previousResponseID),
		PromptCacheKey:     strings.TrimSpace(promptCacheKey),
		Store:              store,
	}
	if probe.StatusCode < 200 || probe.StatusCode >= 300 {
		return sample
	}
	usage := parseResponsesUsage(probe.Body)
	if usage == nil {
		return sample
	}
	sample.InputTokens = usage.InputTokens
	sample.OutputTokens = usage.OutputTokens
	sample.CachedTokens = usage.CachedTokens
	sample.CacheCreationTokens = usage.CacheCreationTokens
	sample.CacheReadInputTokens = usage.CachedTokens
	sample.CacheCreationInputTokens = usage.CacheCreationTokens
	sample.UncachedInputTokens = maxInt64(0, usage.InputTokens-usage.CachedTokens)
	sample.ReasoningTokens = usage.ReasoningTokens
	sample.TotalTokens = usage.TotalTokens
	sample.ResponseID = strings.TrimSpace(gjson.GetBytes(probe.Body, "id").String())
	sample.StateLinked = sample.PreviousResponseID != "" && sample.ResponseID != ""
	sample.UncachedBaselineUSD = roundMoney(tokenCost(usage, pricing, false))
	sample.OfficialBaselineUSD = roundMoney(tokenCost(usage, pricing, true))
	sample.BaselineCostUSD = sample.OfficialBaselineUSD
	sample.ActualCostUSD = sample.OfficialBaselineUSD
	sample.CostUSD = sample.ActualCostUSD
	if sample.UncachedBaselineUSD > sample.ActualCostUSD {
		sample.CacheDiscountUSD = roundMoney(sample.UncachedBaselineUSD - sample.ActualCostUSD)
	}
	if sample.OfficialBaselineUSD > 0 {
		sample.Multiplier = roundRatio(sample.ActualCostUSD / sample.OfficialBaselineUSD)
		sample.Ratio = sample.Multiplier
	}
	sample.Status = CheckStatusPass
	applyTokenAuditSampleDerivedFields(&sample)
	return sample
}

func tokenCost(usage *TokenUsage, pricing openAIModelPricing, cacheAware bool) float64 {
	if usage == nil {
		return 0
	}
	inputTokens := usage.InputTokens
	cachedTokens := usage.CachedTokens
	if cachedTokens < 0 {
		cachedTokens = 0
	}
	if cachedTokens > inputTokens {
		cachedTokens = inputTokens
	}
	billedInput := inputTokens
	cost := float64(billedInput) * pricing.InputPerToken
	if cacheAware && cachedTokens > 0 {
		uncached := inputTokens - cachedTokens
		cost = float64(uncached)*pricing.InputPerToken + float64(cachedTokens)*pricing.CacheReadPerToken
	}
	cost += float64(usage.OutputTokens) * pricing.OutputPerToken
	return cost
}

func finalizeTokenAudit(report *TokenAuditReport) {
	if report == nil {
		return
	}
	report.SampleCount = len(report.Samples)
	if report.OfficialBaselineUSD == 0 || report.ActualCostUSD == 0 {
		for _, sample := range report.Samples {
			if sample.Status != CheckStatusPass {
				continue
			}
			report.OfficialBaselineUSD += sample.OfficialBaselineUSD
			report.ActualCostUSD += sample.ActualCostUSD
		}
		report.OfficialBaselineUSD = roundMoney(report.OfficialBaselineUSD)
		report.ActualCostUSD = roundMoney(report.ActualCostUSD)
	}
	if report.OfficialBaselineUSD > 0 {
		report.Multiplier = roundRatio(report.ActualCostUSD / report.OfficialBaselineUSD)
		report.OverallRatio = report.Multiplier
	}
	report.BaselineTotalCostUSD = report.OfficialBaselineUSD
	report.TotalCostUSD = report.ActualCostUSD
	report.BaselineTotalCost = report.BaselineTotalCostUSD
	report.TotalCost = report.TotalCostUSD
	report.OverallRatioCompat = report.Multiplier
	for i := range report.Samples {
		applyTokenAuditSampleDerivedFields(&report.Samples[i])
	}
	report.Rows = append([]TokenAuditSample(nil), report.Samples...)
	cacheDenominator := report.InputTokens + report.CacheCreationTokens + report.CachedTokens
	if cacheDenominator > 0 {
		report.CacheHitRate = roundRatio(float64(report.CachedTokens) / float64(cacheDenominator))
	}
	report.CacheHitRatePercent = math.Round(report.CacheHitRate * 100)
	passed := 0
	for _, sample := range report.Samples {
		if sample.Status == CheckStatusPass {
			passed++
		}
	}
	ratioLooksNormal := report.Multiplier >= 0.5 && report.Multiplier <= 1.5
	cacheDiscountLooksNormal := report.CachedTokens > 0 && report.Multiplier >= 0.05 && report.Multiplier <= 1.5
	switch {
	case passed == tokenAuditSamples && (ratioLooksNormal || cacheDiscountLooksNormal):
		report.Status = CheckStatusPass
		report.Summary = "用量正常"
	case passed > 0:
		report.Status = CheckStatusWarn
		report.Summary = "用量样本不完整或倍率波动"
		report.Anomalies = append(report.Anomalies, "sample_or_ratio_anomaly")
	default:
		report.Status = CheckStatusFail
		report.Summary = "未获取到可审计 usage"
		report.Anomalies = append(report.Anomalies, "usage_missing")
	}
}

func finalizeOpenAITokenAudit(report *TokenAuditReport) {
	finalizeTokenAudit(report)
	if report == nil || report.SampleCount == 0 || report.Status == CheckStatusFail {
		return
	}
	expectedStatefulRounds := maxInt(0, minInt(tokenAuditSamples, report.SampleCount)-1)
	report.PreviousChainOK = expectedStatefulRounds == 0 || report.StatefulRounds >= expectedStatefulRounds
	if !report.PreviousChainOK {
		report.Status = CheckStatusWarn
		report.Summary = "Responses 状态链路不完整"
		report.Anomalies = appendUniqueString(report.Anomalies, "previous_response_chain_incomplete")
	}
	if report.CachedTokens == 0 {
		report.Status = CheckStatusWarn
		if report.PreviousChainOK {
			report.Summary = "状态链路正常，但未观察到 cached_tokens"
		}
		report.Anomalies = appendUniqueString(report.Anomalies, "cached_tokens_missing")
	}
	if report.UncachedBaselineUSD > 0 && report.ActualCostUSD > 0 {
		report.OverallRatio = roundRatio(report.ActualCostUSD / report.UncachedBaselineUSD)
		report.OverallRatioCompat = report.OverallRatio
	}
}

func applyTokenAuditSampleDerivedFields(sample *TokenAuditSample) {
	if sample == nil {
		return
	}
	if sample.BaselineCostUSD > 0 {
		sample.CostDeltaPct = roundDeltaPercent(sample.CostUSD, sample.BaselineCostUSD)
	}
	if sample.BaselineInputTokens > 0 {
		sample.InputDeltaPct = roundDeltaPercent(float64(sample.InputTokens), float64(sample.BaselineInputTokens))
	}
	if sample.BaselineOutputTokens > 0 {
		sample.OutputDeltaPct = roundDeltaPercent(float64(sample.OutputTokens), float64(sample.BaselineOutputTokens))
	}
	if sample.BaselineCacheCreation > 0 {
		sample.CacheCreationDeltaPct = roundDeltaPercent(float64(sample.CacheCreationInputTokens), float64(sample.BaselineCacheCreation))
	}
	if sample.BaselineCacheRead > 0 {
		sample.CacheReadDeltaPct = roundDeltaPercent(float64(sample.CacheReadInputTokens), float64(sample.BaselineCacheRead))
	}
}

func roundDeltaPercent(actual float64, baseline float64) float64 {
	if baseline <= 0 || math.IsNaN(actual) || math.IsNaN(baseline) || math.IsInf(actual, 0) || math.IsInf(baseline, 0) {
		return 0
	}
	return math.Round((actual - baseline) * 100 / baseline)
}

type streamProbe struct {
	StatusCode     int
	Headers        map[string]string
	FirstTokenMS   int64
	TotalLatencyMS int64
	SeenData       bool
	SeenDelta      bool
	SeenCompleted  bool
	SeenDone       bool
	ErrorClass     string
	ErrorMessage   string
}

func (s *Service) probeResponsesStream(ctx context.Context, client *http.Client, baseURL string, apiKey string, model string) streamProbe {
	started := s.currentTime()
	body := responsesStreamProbePayload(model)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, buildOpenAIEndpointURL(baseURL, "/v1/responses"), bytes.NewReader(body))
	if err != nil {
		return streamProbe{ErrorClass: "request_build_error", ErrorMessage: sanitizeMessage(err.Error(), apiKey)}
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	if client == nil {
		client = s.clientForRun(checkRunOptions{})
	}
	resp, err := client.Do(req)
	if err != nil {
		return streamProbe{
			TotalLatencyMS: int64(s.currentTime().Sub(started) / time.Millisecond),
			ErrorClass:     "network_error",
			ErrorMessage:   sanitizeMessage(err.Error(), apiKey),
		}
	}
	defer func() { _ = resp.Body.Close() }()

	result := streamProbe{StatusCode: resp.StatusCode, Headers: selectedResponseHeaders(resp.Header)}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, maxProbeBodyBytes))
		result.TotalLatencyMS = int64(s.currentTime().Sub(started) / time.Millisecond)
		errorMessage := upstreamErrorMessage(bodyBytes)
		result.ErrorClass = errorClassForStatusAndMessage(resp.StatusCode, errorMessage)
		result.ErrorMessage = sanitizeMessage(errorMessage, apiKey)
		return result
	}
	readResponsesStream(resp.Body, started, s.currentTime, &result, apiKey)
	result.TotalLatencyMS = int64(s.currentTime().Sub(started) / time.Millisecond)
	return result
}

func readResponsesStream(body io.Reader, started time.Time, now func() time.Time, result *streamProbe, apiKey string) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" {
			continue
		}
		result.SeenData = true
		if data == "[DONE]" {
			result.SeenDone = true
			continue
		}
		eventType := strings.TrimSpace(gjson.Get(data, "type").String())
		switch eventType {
		case "response.output_text.delta":
			if strings.TrimSpace(gjson.Get(data, "delta").String()) != "" {
				result.SeenDelta = true
				if result.FirstTokenMS == 0 {
					result.FirstTokenMS = int64(now().Sub(started) / time.Millisecond)
				}
			}
		case "response.completed", "response.done":
			result.SeenCompleted = true
		case "response.failed":
			result.ErrorClass = "response_failed"
			result.ErrorMessage = sanitizeMessage(upstreamErrorMessage([]byte(data)), apiKey)
		}
		if !result.SeenDelta && strings.TrimSpace(gjson.Get(data, "response.output.0.content.0.text").String()) != "" {
			result.SeenDelta = true
			if result.FirstTokenMS == 0 {
				result.FirstTokenMS = int64(now().Sub(started) / time.Millisecond)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		result.ErrorClass = "stream_error"
		result.ErrorMessage = sanitizeMessage(err.Error(), apiKey)
	}
}

func responsesToolProbePayload(model string, stream bool) []byte {
	if strings.TrimSpace(model) == "" {
		model = openai.DefaultTestModel
	}
	body, _ := json.Marshal(map[string]any{
		"model": model,
		"input": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{"type": "input_text", "text": "Call the probe_ping function with ok=true to acknowledge readiness. You must use the tool."},
				},
			},
		},
		"tools": []map[string]any{
			{
				"type":        "function",
				"name":        "probe_ping",
				"description": "Capability probe. Call to acknowledge.",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"ok": map[string]any{"type": "boolean"},
					},
					"required": []string{"ok"},
				},
			},
		},
		"tool_choice":       "required",
		"max_output_tokens": 512,
		"stream":            stream,
	})
	return body
}

func responsesStreamProbePayload(model string) []byte {
	if strings.TrimSpace(model) == "" {
		model = openai.DefaultTestModel
	}
	body, _ := json.Marshal(map[string]any{
		"model":             model,
		"input":             "Return exactly: ok",
		"max_output_tokens": 16,
		"stream":            true,
	})
	return body
}

func responsesMultimodalProbePayload(model string) []byte {
	if strings.TrimSpace(model) == "" {
		model = openai.DefaultTestModel
	}
	body, _ := json.Marshal(map[string]any{
		"model": model,
		"input": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{"type": "input_text", "text": "Read the attached image and return exactly: ok"},
					{"type": "input_image", "image_url": probePNGData},
				},
			},
		},
		"max_output_tokens": 16,
		"stream":            false,
	})
	return body
}

func openAITokenAuditPrompt(round int, auditNonce string) string {
	return strings.Join([]string{
		openAITokenAuditInstructions(auditNonce),
		auditCumulativeRoundText(round),
		tokenAuditRoundInstruction(round),
	}, "\n\n")
}

func openAITokenAuditInstructions(auditNonce string) string {
	return strings.Join([]string{
		"proxyai.best OpenAI token audit. Keep responses concise and do not call tools unless explicitly requested. audit_nonce=" + auditNonce,
		auditStableCacheText(auditNonce),
	}, "\n\n")
}

func openAITokenAuditRoundInput(round int, auditNonce string) string {
	return strings.Join([]string{
		fmt.Sprintf("Responses stateful audit round %02d. Use the prior response context when previous_response_id is present. audit_nonce=%s", clampAuditRound(round), auditNonce),
		auditRoundCacheText(round),
		tokenAuditRoundInstruction(round),
	}, "\n\n")
}

func openAITokenAuditToolDefinition() map[string]any {
	return map[string]any{
		"type":        "function",
		"name":        "audit_marker",
		"description": "Stable tool schema for prompt-cache and tool-passthrough audit. Do not call unless requested.",
		"parameters": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"round": map[string]any{"type": "integer"},
				"ok":    map[string]any{"type": "boolean"},
			},
			"required": []string{"round", "ok"},
		},
	}
}

func openAITokenAuditPromptCacheKey(model string, auditNonce string) string {
	raw := strings.Join([]string{"proxyai.best", "openai-token-audit", strings.TrimSpace(model), auditNonce}, "\x00")
	hash := sha256Hex(raw)
	if len(hash) > 24 {
		hash = hash[:24]
	}
	return "proxyai_best_" + hash
}

func auditStableCacheText(auditNonce string) string {
	return "stable-cache-prefix " + auditNonce + " " + strings.Repeat("cache-anchor proxyai-best-purity "+auditNonce+" ", 620)
}

func auditCumulativeRoundText(round int) string {
	if round < 1 {
		round = 1
	}
	if round > tokenAuditSamples {
		round = tokenAuditSamples
	}
	parts := make([]string, 0, round)
	for i := 1; i <= round; i++ {
		parts = append(parts, auditRoundCacheText(i))
	}
	return strings.Join(parts, "\n")
}

func auditRoundCacheText(round int) string {
	repeats := []int{590, 27, 10, 58, 26, 47, 40, 173, 65, 81, 28}
	round = clampAuditRound(round)
	return fmt.Sprintf("round-cache-%02d ", round) + strings.Repeat(fmt.Sprintf("segment-%02d proxyai-best-cache ", round), repeats[round-1])
}

func tokenAuditRoundInstruction(round int) string {
	targets := []int{166, 142, 859, 351, 600, 867, 1041, 743, 156, 90, 487}
	round = clampAuditRound(round)
	return fmt.Sprintf("Round %02d: write a compact comma-separated sequence of exactly %d lowercase letter x characters and nothing else.", round, targets[round-1])
}

func tokenAuditOutputBudget(round int) int {
	targets := []int{220, 190, 1100, 460, 760, 1080, 1300, 940, 210, 140, 640}
	round = clampAuditRound(round)
	return targets[round-1]
}

func newAuditNonce(model string, at time.Time) string {
	raw := strings.Join([]string{"proxyai.best", model, at.UTC().Format(time.RFC3339Nano)}, "\x00")
	hash := sha256Hex(raw)
	if len(hash) > 12 {
		return hash[:12]
	}
	return hash
}

func buildModelsCheck(probe httpProbe, model string) CheckResult {
	details := probeDetails(probe)
	if probe.StatusCode == 0 {
		return failCheck("models_schema", "模型列表结构", 15, "无法连接模型列表端点。", details)
	}
	if probe.ErrorClass == errorClassAccountBalanceInsufficient {
		return failCheck("models_schema", "模型列表结构", 15, "账号余额不足，模型列表探测无法执行。", details)
	}
	if probe.StatusCode == http.StatusUnauthorized || probe.StatusCode == http.StatusForbidden {
		return failCheck("models_schema", "模型列表结构", 15, "API Key 鉴权失败。", details)
	}
	if probe.StatusCode < 200 || probe.StatusCode >= 300 {
		return CheckResult{ID: "models_schema", Name: "模型列表结构", Status: CheckStatusWarn, Score: 4, MaxScore: 15, Message: "模型列表端点未返回标准 2xx 响应。", Details: details}
	}
	object := gjson.GetBytes(probe.Body, "object").String()
	data := gjson.GetBytes(probe.Body, "data")
	if object != "list" || !data.IsArray() {
		return CheckResult{ID: "models_schema", Name: "模型列表结构", Status: CheckStatusWarn, Score: 6, MaxScore: 15, Message: "模型列表响应不是标准 OpenAI list 结构。", Details: details}
	}
	modelListed := false
	count := 0
	for _, item := range data.Array() {
		count++
		if strings.TrimSpace(item.Get("id").String()) == model {
			modelListed = true
		}
	}
	details["model_count"] = count
	details["model_listed"] = modelListed
	if modelListed {
		return passCheck("models_schema", "模型列表结构", 15, "模型列表结构标准，且包含本次检测模型。", details)
	}
	return CheckResult{ID: "models_schema", Name: "模型列表结构", Status: CheckStatusWarn, Score: 12, MaxScore: 15, Message: "模型列表结构标准，但未列出本次检测模型。", Details: details}
}

func buildResponsesSchemaCheck(probe httpProbe, apiKey string) CheckResult {
	details := probeDetails(probe)
	if probe.StatusCode == 0 {
		return failCheck("responses_schema", "Responses 非流式结构", 20, "无法连接 Responses 端点。", details)
	}
	if probe.ErrorClass == errorClassAccountBalanceInsufficient {
		return failCheck("responses_schema", "Responses 非流式结构", 20, "账号余额不足，Responses 探测无法执行。", details)
	}
	if probe.StatusCode == http.StatusUnauthorized || probe.StatusCode == http.StatusForbidden {
		return failCheck("responses_schema", "Responses 非流式结构", 20, "Responses 端点鉴权失败。", details)
	}
	if probe.StatusCode == http.StatusNotFound || probe.StatusCode == http.StatusMethodNotAllowed {
		return failCheck("responses_schema", "Responses 非流式结构", 20, "Responses 端点不存在或方法不支持。", details)
	}
	if probe.StatusCode < 200 || probe.StatusCode >= 300 {
		details["error_message"] = sanitizeMessage(upstreamErrorMessage(probe.Body), apiKey)
		return failCheck("responses_schema", "Responses 非流式结构", 20, "Responses 端点未返回可用响应。", details)
	}
	if gjson.GetBytes(probe.Body, "object").String() == "response" && gjson.GetBytes(probe.Body, "output").IsArray() {
		return passCheck("responses_schema", "Responses 非流式结构", 20, "Responses 响应结构符合 OpenAI 预期。", details)
	}
	return CheckResult{ID: "responses_schema", Name: "Responses 非流式结构", Status: CheckStatusWarn, Score: 8, MaxScore: 20, Message: "Responses 返回 2xx，但响应结构不完整。", Details: details}
}

func buildToolCallCheck(probe httpProbe) CheckResult {
	details := probeDetails(probe)
	if probe.StatusCode < 200 || probe.StatusCode >= 300 {
		return failCheck("tool_call", "强制工具调用", 20, "Responses 探测未成功，无法确认工具调用。", details)
	}
	ok, toolDetails := responsesBodyHasExpectedFunctionCall(probe.Body)
	for key, value := range toolDetails {
		details[key] = value
	}
	if ok {
		return passCheck("tool_call", "强制工具调用", 20, "tool_choice=required 成功产出 probe_ping(ok=true) function_call。", details)
	}
	return failCheck("tool_call", "强制工具调用", 20, "强制工具调用没有产出预期 function_call。", details)
}

func buildUsageCheck(usage *TokenUsage, probe httpProbe) CheckResult {
	details := probeDetails(probe)
	if probe.StatusCode < 200 || probe.StatusCode >= 300 {
		return failCheck("usage", "Usage 计量", 10, "Responses 探测未成功，无法读取 usage。", details)
	}
	if usage == nil {
		return failCheck("usage", "Usage 计量", 10, "响应缺少 usage 计量字段。", details)
	}
	details["input_tokens"] = usage.InputTokens
	details["output_tokens"] = usage.OutputTokens
	details["total_tokens"] = usage.TotalTokens
	if usage.TotalTokens >= usage.InputTokens+usage.OutputTokens && usage.TotalTokens > 0 {
		return passCheck("usage", "Usage 计量", 10, "usage token 计量字段完整。", details)
	}
	return CheckResult{ID: "usage", Name: "Usage 计量", Status: CheckStatusWarn, Score: 5, MaxScore: 10, Message: "usage 字段存在，但 token 汇总不完全一致。", Details: details}
}

func buildStreamingCheck(probe streamProbe, apiKey string) CheckResult {
	details := map[string]any{
		"status_code":      probe.StatusCode,
		"first_token_ms":   probe.FirstTokenMS,
		"total_latency_ms": probe.TotalLatencyMS,
		"seen_data":        probe.SeenData,
		"seen_delta":       probe.SeenDelta,
		"seen_completed":   probe.SeenCompleted,
		"seen_done":        probe.SeenDone,
	}
	if probe.ErrorClass != "" {
		details["error_class"] = probe.ErrorClass
		details["error_message"] = sanitizeMessage(probe.ErrorMessage, apiKey)
	}
	if probe.StatusCode == 0 {
		return failCheck("streaming", "Responses 流式事件", 15, "无法连接 Responses 流式端点。", details)
	}
	if probe.ErrorClass == errorClassAccountBalanceInsufficient {
		return failCheck("streaming", "Responses 流式事件", 15, "账号余额不足，Responses 流式探测无法执行。", details)
	}
	if probe.StatusCode < 200 || probe.StatusCode >= 300 {
		return failCheck("streaming", "Responses 流式事件", 15, "Responses 流式端点未返回可用响应。", details)
	}
	if probe.SeenDelta && (probe.SeenCompleted || probe.SeenDone) && probe.ErrorClass == "" {
		return passCheck("streaming", "Responses 流式事件", 15, "SSE delta 与完成事件完整。", details)
	}
	if probe.SeenData && (probe.SeenCompleted || probe.SeenDone) {
		return CheckResult{ID: "streaming", Name: "Responses 流式事件", Status: CheckStatusWarn, Score: 8, MaxScore: 15, Message: "SSE 生命周期结束，但未观察到文本 delta。", Details: details}
	}
	return failCheck("streaming", "Responses 流式事件", 15, "SSE 生命周期不完整。", details)
}

func buildMultimodalCheck(probe httpProbe, apiKey string) CheckResult {
	details := probeDetails(probe)
	if probe.ErrorMessage != "" {
		details["error_message"] = sanitizeMessage(probe.ErrorMessage, apiKey)
	}
	if probe.StatusCode == 0 {
		return failCheck("multimodal", "多模态输入", 10, "无法连接 Responses 多模态探测端点。", details)
	}
	if probe.ErrorClass == errorClassAccountBalanceInsufficient {
		return failCheck("multimodal", "多模态输入", 10, "账号余额不足，多模态探测无法执行。", details)
	}
	if probe.StatusCode >= 200 && probe.StatusCode < 300 && gjson.GetBytes(probe.Body, "object").String() == "response" {
		return passCheck("multimodal", "多模态输入", 10, "Responses 接受 input_image 多模态输入结构。", details)
	}
	if probe.StatusCode == http.StatusBadRequest || probe.StatusCode == http.StatusUnprocessableEntity {
		return CheckResult{ID: "multimodal", Name: "多模态输入", Status: CheckStatusWarn, Score: 5, MaxScore: 10, Message: "端点存在，但当前模型或上游不接受 input_image 输入。", Details: details}
	}
	return failCheck("multimodal", "多模态输入", 10, "多模态探测未返回标准 Responses 响应。", details)
}

func buildTokenAuditCheck(audit *TokenAuditReport) CheckResult {
	details := map[string]any{}
	if audit != nil {
		details["sample_count"] = audit.SampleCount
		details["multiplier"] = audit.Multiplier
		details["overall_ratio"] = audit.OverallRatio
		details["cache_hit_rate"] = audit.CacheHitRate
		details["official_baseline_usd"] = audit.OfficialBaselineUSD
		details["uncached_baseline_usd"] = audit.UncachedBaselineUSD
		details["actual_cost_usd"] = audit.ActualCostUSD
		details["total_cost"] = audit.TotalCostUSD
		if audit.PromptCacheKey != "" {
			details["prompt_cache_key"] = audit.PromptCacheKey
		}
		details["store_enabled"] = audit.StoreEnabled
		details["stateful_rounds"] = audit.StatefulRounds
		details["previous_response_chain_ok"] = audit.PreviousChainOK
		if len(audit.Anomalies) > 0 {
			details["anomalies"] = append([]string(nil), audit.Anomalies...)
		}
	}
	switch {
	case audit == nil:
		return failCheck("token_audit", "Token 用量审计", 15, "未执行 Token 用量审计。", details)
	case audit.Status == CheckStatusPass:
		return passCheck("token_audit", "Token 用量审计", 15, audit.Summary, details)
	case audit.Status == CheckStatusWarn:
		return CheckResult{ID: "token_audit", Name: "Token 用量审计", Status: CheckStatusWarn, Score: 8, MaxScore: 15, Message: audit.Summary, Details: details}
	default:
		return failCheck("token_audit", "Token 用量审计", 15, audit.Summary, details)
	}
}

func buildChatFallbackCheck(probe httpProbe, apiKey string) CheckResult {
	details := probeDetails(probe)
	if probe.ErrorMessage != "" {
		details["error_message"] = sanitizeMessage(probe.ErrorMessage, apiKey)
	}
	if probe.StatusCode >= 200 && probe.StatusCode < 300 && gjson.GetBytes(probe.Body, "choices").IsArray() {
		return CheckResult{ID: "chat_completions", Name: "Chat Completions 兼容回退", Status: CheckStatusPass, Score: 0, MaxScore: 0, Message: "Chat Completions 回退端点可用。", Details: details}
	}
	return CheckResult{ID: "chat_completions", Name: "Chat Completions 兼容回退", Status: CheckStatusFail, Score: 0, MaxScore: 0, Message: "Chat Completions 回退端点不可用。", Details: details}
}

func responsesBodyHasExpectedFunctionCall(body []byte) (bool, map[string]any) {
	details := map[string]any{"function_call_seen": false}
	output := gjson.GetBytes(body, "output")
	if !output.IsArray() {
		return false, details
	}
	for _, item := range output.Array() {
		if strings.TrimSpace(item.Get("type").String()) == "function_call" {
			details["function_call_seen"] = true
			details["function_name"] = item.Get("name").String()
			arguments := item.Get("arguments").String()
			details["arguments_json"] = arguments != ""
			if strings.TrimSpace(item.Get("name").String()) != "probe_ping" {
				continue
			}
			if gjson.Get(arguments, "ok").Bool() {
				details["arguments_ok"] = true
				return true, details
			}
			details["arguments_ok"] = false
		}
	}
	return false, details
}

func parseResponsesUsage(body []byte) *TokenUsage {
	usage := gjson.GetBytes(body, "usage")
	if !usage.Exists() || !usage.IsObject() {
		return nil
	}
	return &TokenUsage{
		InputTokens:         usage.Get("input_tokens").Int(),
		OutputTokens:        usage.Get("output_tokens").Int(),
		TotalTokens:         usage.Get("total_tokens").Int(),
		CacheCreationTokens: firstUsageInt(usage, "input_tokens_details.cache_creation_tokens", "input_tokens_details.cache_creation_input_tokens", "prompt_tokens_details.cache_creation_tokens"),
		CachedTokens:        firstUsageInt(usage, "input_tokens_details.cached_tokens", "prompt_tokens_details.cached_tokens"),
		ReasoningTokens:     usage.Get("output_tokens_details.reasoning_tokens").Int(),
	}
}

func firstUsageInt(usage gjson.Result, paths ...string) int64 {
	for _, path := range paths {
		if value := usage.Get(path); value.Exists() {
			return value.Int()
		}
	}
	return 0
}

func shouldStopAfterProbe(probe httpProbe) bool {
	if probe.StatusCode == http.StatusUnauthorized || probe.StatusCode == http.StatusForbidden {
		return true
	}
	return probe.StatusCode == 0 && probe.ErrorClass == "network_error"
}

func skippedCoreChecks(message string) []CheckResult {
	return []CheckResult{
		failCheck("responses_schema", "Responses 非流式结构", 20, message, nil),
		failCheck("tool_call", "强制工具调用", 20, message, nil),
		failCheck("streaming", "Responses 流式事件", 15, message, nil),
		failCheck("usage", "Usage 计量", 10, message, nil),
		failCheck("multimodal", "多模态输入", 10, message, nil),
		failCheck("token_audit", "Token 用量审计", 15, message, nil),
	}
}

func needsChatFallback(checks []CheckResult) bool {
	for _, check := range checks {
		if check.ID == "responses_schema" || check.ID == "tool_call" || check.ID == "streaming" {
			if check.Status != CheckStatusPass {
				return true
			}
		}
	}
	return false
}

func passCheck(id, name string, max int, message string, details map[string]any) CheckResult {
	return CheckResult{ID: id, Name: name, Status: CheckStatusPass, Score: max, MaxScore: max, Message: message, Details: details}
}

func failCheck(id, name string, max int, message string, details map[string]any) CheckResult {
	return CheckResult{ID: id, Name: name, Status: CheckStatusFail, Score: 0, MaxScore: max, Message: message, Details: details}
}

func probeDetails(probe httpProbe) map[string]any {
	details := map[string]any{
		"status_code": probe.StatusCode,
		"latency_ms":  probe.LatencyMS,
	}
	if probe.ErrorClass != "" {
		details["error_class"] = probe.ErrorClass
	}
	if probe.ErrorMessage != "" {
		details["error_message"] = probe.ErrorMessage
	}
	return details
}

func firstProbeError(checks []CheckResult) (string, string) {
	for _, check := range checks {
		if check.Status == CheckStatusPass || check.Details == nil {
			continue
		}
		errorClass, _ := check.Details["error_class"].(string)
		errorMessage, _ := check.Details["error_message"].(string)
		if errorClass != "" || errorMessage != "" {
			return errorClass, errorMessage
		}
	}
	return "", ""
}

type validationSpec struct {
	ID       string
	Name     string
	CheckIDs []string
}

func validationSpecs() []validationSpec {
	return []validationSpec{
		{ID: "llm_fingerprint", Name: "LLM 指纹验证", CheckIDs: []string{"base_url", "models_schema"}},
		{ID: "schema_integrity", Name: "结构完整性", CheckIDs: []string{"responses_schema"}},
		{ID: "behavior", Name: "行为验证", CheckIDs: []string{"tool_call", "streaming"}},
		{ID: "signature", Name: "签名校验", CheckIDs: []string{"usage"}},
		{ID: "multimodal", Name: "多模态能力", CheckIDs: []string{"multimodal"}},
		{ID: "token_audit", Name: "Token 用量审计", CheckIDs: []string{"token_audit"}},
	}
}

func buildLLMFingerprintValidation(baseURLCheck CheckResult, modelsCheck CheckResult) ValidationResult {
	validation := validationFromExecutedChecks("llm_fingerprint", "LLM 指纹验证", []CheckResult{baseURLCheck, modelsCheck})
	validation.Details["detector"] = "openai_base_url_and_models_probe"
	return validation
}

func validationFromExecutedChecks(id string, name string, checks []CheckResult) ValidationResult {
	out := ValidationResult{
		ID:              id,
		Name:            name,
		Status:          CheckStatusPass,
		RelatedCheckIDs: checkIDsFromResults(checks),
		Details: map[string]any{
			"detector": "programmatic_probe",
		},
	}
	messages := make([]string, 0, len(checks))
	evidence := make(map[string]any, len(checks))
	for _, check := range checks {
		evidence[check.ID] = map[string]any{
			"name":       check.Name,
			"status":     check.Status,
			"score":      check.Score,
			"max_score":  check.MaxScore,
			"message":    check.Message,
			"probe_data": check.Details,
		}
		messages = append(messages, check.Message)
		switch check.Status {
		case CheckStatusFail:
			out.Status = CheckStatusFail
		case CheckStatusWarn:
			if out.Status != CheckStatusFail {
				out.Status = CheckStatusWarn
			}
		}
	}
	out.Details["evidence"] = evidence
	out.Message = strings.Join(messages, " ")
	return out
}

func checkIDsFromResults(checks []CheckResult) []string {
	ids := make([]string, 0, len(checks))
	for _, check := range checks {
		ids = append(ids, check.ID)
	}
	return ids
}

func appendAndEmitChecks(report *PublicReport, emit PublicCheckEventSink, checks ...CheckResult) {
	if report == nil {
		return
	}
	for _, check := range checks {
		report.Checks = append(report.Checks, check)
		checkCopy := check
		emitPublicCheckEvent(emit, PublicCheckEvent{
			Type:     PublicCheckEventCheck,
			ReportID: report.ReportID,
			Check:    &checkCopy,
		})
	}
}

func skippedValidation(id string, name string, checkIDs []string, message string) ValidationResult {
	return ValidationResult{
		ID:              id,
		Name:            name,
		Status:          CheckStatusFail,
		Message:         message,
		RelatedCheckIDs: checkIDs,
		Details: map[string]any{
			"detector": "programmatic_probe",
			"skipped":  true,
		},
	}
}

func upsertValidation(report *PublicReport, validation ValidationResult) {
	if report == nil || validation.ID == "" {
		return
	}
	for i := range report.Validations {
		if report.Validations[i].ID == validation.ID {
			report.Validations[i] = validation
			return
		}
	}
	report.Validations = append(report.Validations, validation)
}

func upsertAndEmitValidation(report *PublicReport, emit PublicCheckEventSink, validation ValidationResult) {
	upsertValidation(report, validation)
	if report == nil {
		return
	}
	validationCopy := validation
	emitPublicCheckEvent(emit, PublicCheckEvent{
		Type:       PublicCheckEventValidation,
		ReportID:   report.ReportID,
		Validation: &validationCopy,
	})
}

func finalizeValidations(report *PublicReport) {
	if report == nil {
		return
	}
	byID := make(map[string]ValidationResult, len(report.Validations))
	for _, validation := range report.Validations {
		if validation.ID != "" {
			byID[validation.ID] = validation
		}
	}
	ordered := make([]ValidationResult, 0, len(validationSpecs()))
	for _, spec := range validationSpecs() {
		validation, ok := byID[spec.ID]
		if !ok {
			validation = skippedValidation(spec.ID, spec.Name, spec.CheckIDs, "该验证项未执行。")
		}
		ordered = append(ordered, validation)
	}
	report.Validations = ordered
}

func emitMetrics(report *PublicReport, emit PublicCheckEventSink) {
	if report == nil {
		return
	}
	metrics := report.Metrics
	syncMetricsCompat(&metrics)
	emitPublicCheckEvent(emit, PublicCheckEvent{
		Type:     PublicCheckEventMetrics,
		ReportID: report.ReportID,
		Metrics:  &metrics,
	})
}

func emitProgress(report *PublicReport, emit PublicCheckEventSink, step int, stepName string) {
	if report == nil {
		return
	}
	if step < 1 {
		step = 1
	}
	if step > 7 {
		step = 7
	}
	report.Status = RunStatusRunning
	report.Step = step
	report.StepName = stepName
	report.Progress = roundProgress(float64(step) / 7)
	report.Scores = scoreBreakdown(report)
	syncReportCompat(report)
	emitPublicCheckEvent(emit, PublicCheckEvent{
		Type:               PublicCheckEventProgress,
		ReportID:           report.ReportID,
		Status:             report.Status,
		Step:               report.Step,
		StepName:           report.StepName,
		Progress:           report.Progress,
		Scores:             cloneScores(report.Scores),
		TokenAuditProgress: report.TokenAuditProgress,
		TokenAuditPartial:  append([]TokenAuditSample(nil), report.TokenAuditPartial...),
		Report:             clonePublicReport(report),
	})
}

func emitFinalReport(report *PublicReport, emit PublicCheckEventSink) {
	if report == nil {
		return
	}
	if report.Status == RunStatusError {
		if report.Step <= 0 {
			report.Step = 1
		}
		if report.StepName == "" {
			report.StepName = "tag"
		}
		if report.Progress <= 0 {
			report.Progress = roundProgress(float64(report.Step) / 7)
		}
	} else {
		report.Status = RunStatusDone
		report.Step = 7
		report.StepName = "evaluate"
		report.Progress = 1
	}
	report.Scores = scoreBreakdown(report)
	syncReportCompat(report)
	emitPublicCheckEvent(emit, PublicCheckEvent{
		Type:     PublicCheckEventReport,
		ReportID: report.ReportID,
		Status:   report.Status,
		Step:     report.Step,
		StepName: report.StepName,
		Progress: report.Progress,
		Scores:   cloneScores(report.Scores),
		Report:   clonePublicReport(report),
	})
}

func emitPublicCheckEvent(emit PublicCheckEventSink, event PublicCheckEvent) {
	if emit != nil {
		if event.StepNameCompat == "" {
			event.StepNameCompat = event.StepName
		}
		if event.Metrics != nil {
			syncMetricsCompat(event.Metrics)
		}
		if event.Report != nil {
			syncReportCompat(event.Report)
		}
		emit(event)
	}
}

func clonePublicReport(report *PublicReport) *PublicReport {
	if report == nil {
		return nil
	}
	value := *report
	value.Scores = cloneScores(report.Scores)
	value.TokenAuditPartial = append([]TokenAuditSample(nil), report.TokenAuditPartial...)
	syncReportCompat(&value)
	return &value
}

func cloneScores(scores map[string]int) map[string]int {
	if len(scores) == 0 {
		return nil
	}
	out := make(map[string]int, len(scores))
	for key, value := range scores {
		out[key] = value
	}
	return out
}

func roundProgress(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return math.Round(value*1000) / 1000
}

func scoreBreakdown(report *PublicReport) map[string]int {
	if report == nil {
		return nil
	}
	out := map[string]int{
		"tag_check":       0,
		"structure":       0,
		"behavior":        0,
		"signature_proto": 0,
		"multimodal":      0,
	}
	if hasCheckStatus(report.Checks, "base_url", CheckStatusPass, CheckStatusWarn) ||
		hasCheckStatus(report.Checks, "models_schema", CheckStatusPass, CheckStatusWarn) ||
		hasCheckStatus(report.Checks, "claude_messages_schema", CheckStatusPass, CheckStatusWarn) {
		out["tag_check"] = validationWeightedScore(report, "llm_fingerprint", 10)
	}
	out["structure"] = validationWeightedScore(report, "schema_integrity", 20)
	out["behavior"] = validationWeightedScore(report, "behavior", 30)
	out["signature_proto"] = validationWeightedScore(report, "signature", 30)
	out["multimodal"] = validationWeightedScore(report, "multimodal", 10)
	if hasValidation(report.Validations, "token_audit") {
		out["token_audit"] = validationWeightedScore(report, "token_audit", 10)
	}
	return out
}

func validationWeightedScore(report *PublicReport, validationID string, weight int) int {
	if report == nil || weight <= 0 {
		return 0
	}
	for _, validation := range report.Validations {
		if validation.ID != validationID {
			continue
		}
		switch validation.Status {
		case CheckStatusPass:
			return weight
		case CheckStatusWarn:
			return weight / 2
		default:
			return 0
		}
	}
	return 0
}

func hasValidation(validations []ValidationResult, id string) bool {
	for _, validation := range validations {
		if validation.ID == id {
			return true
		}
	}
	return false
}

func hasCheckStatus(checks []CheckResult, id string, statuses ...string) bool {
	for _, check := range checks {
		if check.ID != id {
			continue
		}
		for _, status := range statuses {
			if check.Status == status {
				return true
			}
		}
	}
	return false
}

func upsertTokenAuditPartial(samples []TokenAuditSample, sample TokenAuditSample) []TokenAuditSample {
	out := make([]TokenAuditSample, 0, len(samples)+1)
	replaced := false
	for _, existing := range samples {
		if existing.Index == sample.Index {
			out = append(out, sample)
			replaced = true
			continue
		}
		out = append(out, existing)
	}
	if !replaced {
		out = append(out, sample)
	}
	return out
}

func selectedResponseHeaders(headers http.Header) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	keys := []string{
		"content-type",
		"request-id",
		"x-request-id",
		"x-should-retry",
		"x-ratelimit-limit-requests",
		"x-ratelimit-remaining-requests",
		"anthropic-ratelimit-requests-limit",
		"anthropic-ratelimit-requests-remaining",
		"x-amzn-requestid",
		"x-amzn-trace-id",
		"x-goog-request-id",
		"x-cloud-trace-context",
		"server",
		"via",
		"cf-ray",
	}
	out := make(map[string]string)
	for _, key := range keys {
		if value := strings.TrimSpace(headers.Get(key)); value != "" {
			out[key] = sanitizeMessage(value, "")
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func channelFromProbe(provider string, host string, officialHost bool, probe httpProbe) string {
	return channelFromHeaders(provider, host, officialHost, probe.Headers)
}

func channelFromStreamProbe(provider string, host string, officialHost bool, headers map[string]string) string {
	return channelFromHeaders(provider, host, officialHost, headers)
}

func channelFromHeaders(provider string, host string, officialHost bool, headers map[string]string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	if strings.Contains(host, "bedrock") || headerExists(headers, "x-amzn-requestid", "x-amzn-trace-id") {
		return "aws-bedrock"
	}
	if strings.Contains(host, "googleapis.com") || strings.Contains(host, "vertex") || headerExists(headers, "x-goog-request-id", "x-cloud-trace-context") {
		return "vertex"
	}
	if provider == ProviderAnthropic {
		if officialHost || strings.EqualFold(host, "api.anthropic.com") {
			return "anthropic"
		}
		return "anthropic-compatible"
	}
	if provider == ProviderOpenAI {
		if officialHost || strings.EqualFold(host, "api.openai.com") {
			return "openai"
		}
		return "openai-compatible"
	}
	return "compatible"
}

func headerExists(headers map[string]string, keys ...string) bool {
	for _, key := range keys {
		if strings.TrimSpace(headers[strings.ToLower(key)]) != "" {
			return true
		}
	}
	return false
}

func hasVertexFingerprint(host string, headerSets ...map[string]string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if strings.Contains(host, "googleapis.com") || strings.Contains(host, "vertex") {
		return true
	}
	for _, headers := range headerSets {
		if headerExists(headers, "x-goog-request-id", "x-cloud-trace-context") {
			return true
		}
	}
	return false
}

func hasKiroFingerprint(host string, headerSets ...map[string]string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if strings.Contains(host, "kiro") {
		return true
	}
	for _, headers := range headerSets {
		for key, value := range headers {
			combined := strings.ToLower(key + ":" + value)
			if strings.Contains(combined, "kiro") {
				return true
			}
		}
	}
	return false
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func errorClassForStatus(status int) string {
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return "credential_invalid"
	case status == http.StatusTooManyRequests:
		return "upstream_rate_limited"
	case status >= 500:
		return "upstream_5xx"
	case status >= 400:
		return "request_error"
	default:
		return ""
	}
}

func errorClassForStatusAndMessage(status int, message string) string {
	if isAccountBalanceInsufficientMessage(message) {
		return errorClassAccountBalanceInsufficient
	}
	return errorClassForStatus(status)
}

func isAccountBalanceInsufficientMessage(message string) bool {
	value := strings.ToLower(strings.TrimSpace(message))
	if value == "" {
		return false
	}
	hasInsufficient := strings.Contains(value, "insufficient") || strings.Contains(value, "exceeded")
	if !hasInsufficient {
		return false
	}
	return strings.Contains(value, "balance") ||
		strings.Contains(value, "quota") ||
		strings.Contains(value, "credit") ||
		strings.Contains(value, "billing") ||
		strings.Contains(value, "fund")
}

func upstreamErrorMessage(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	for _, path := range []string{"error.message", "message", "response.error.message"} {
		if value := strings.TrimSpace(gjson.GetBytes(body, path).String()); value != "" {
			return value
		}
	}
	return string(body)
}

func sanitizeMessage(message string, apiKey string) string {
	value := strings.TrimSpace(message)
	if value == "" {
		return ""
	}
	if apiKey != "" {
		value = strings.ReplaceAll(value, apiKey, "[redacted]")
	}
	if len(value) > maxErrorMessageLen {
		value = value[:maxErrorMessageLen]
	}
	return value
}

func percent(score int, max int) int {
	if max <= 0 {
		return 0
	}
	return int(math.Round(float64(score) * 100 / float64(max)))
}

func tokensPerSecond(usage *TokenUsage, latencyMS int64) float64 {
	if usage == nil || usage.OutputTokens <= 0 || latencyMS <= 0 {
		return 0
	}
	return roundRatio(float64(usage.OutputTokens) / (float64(latencyMS) / 1000))
}

func roundRatio(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return math.Round(value*100) / 100
}

func roundMoney(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return math.Round(value*1_000_000) / 1_000_000
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func appendUniqueString(values []string, value string) []string {
	if strings.TrimSpace(value) == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func maxInt64(a int64, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func buildPublicSummary(report *PublicReport) map[string]any {
	if report == nil {
		return nil
	}
	out := map[string]any{
		"provider":            report.Provider,
		"report_id":           report.ReportID,
		"api_base_host":       report.APIBaseHost,
		"model_id":            report.ModelID,
		"expected_model":      report.ExpectedModel,
		"response_model":      report.ResponseModel,
		"status":              report.Status,
		"step":                report.Step,
		"step_name":           report.StepName,
		"progress":            report.Progress,
		"scores":              report.Scores,
		"score":               report.Score,
		"official_score":      report.OfficialScore,
		"compatibility_score": report.CompatibilityScore,
		"verdict":             report.Verdict,
		"summary":             report.Summary,
		"error":               report.Error,
		"stream_channel":      report.StreamChannel,
		"non_stream_channel":  report.NonStreamChannel,
		"has_vertex":          report.HasVertex,
		"is_kiro":             report.IsKiro,
		"validations":         report.Validations,
		"checked_at":          report.CheckedAt,
	}
	if report.TokenAudit != nil {
		out["token_audit"] = map[string]any{
			"status":                report.TokenAudit.Status,
			"summary":               report.TokenAudit.Summary,
			"sample_count":          report.TokenAudit.SampleCount,
			"official_baseline_usd": report.TokenAudit.OfficialBaselineUSD,
			"uncached_baseline_usd": report.TokenAudit.UncachedBaselineUSD,
			"actual_cost_usd":       report.TokenAudit.ActualCostUSD,
			"total_cost":            report.TokenAudit.TotalCostUSD,
			"multiplier":            report.TokenAudit.Multiplier,
			"overall_ratio":         report.TokenAudit.OverallRatio,
			"cache_hit_rate":        report.TokenAudit.CacheHitRate,
			"prompt_cache_key":      report.TokenAudit.PromptCacheKey,
			"store_enabled":         report.TokenAudit.StoreEnabled,
			"stateful_rounds":       report.TokenAudit.StatefulRounds,
			"previous_chain_ok":     report.TokenAudit.PreviousChainOK,
		}
	}
	return out
}

func reportHash(report *PublicReport, baseURL string) string {
	if report == nil {
		return ""
	}
	raw := strings.Join([]string{
		report.Provider,
		baseURL,
		report.ModelID,
		report.CheckedAt.UTC().Format("2006-01-02T15:04"),
	}, "\x00")
	return sha256Hex(raw)
}

func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func buildOpenAIEndpointURL(base string, endpoint string) string {
	normalized := strings.TrimRight(strings.TrimSpace(base), "/")
	endpoint = "/" + strings.TrimLeft(strings.TrimSpace(endpoint), "/")
	relative := strings.TrimPrefix(endpoint, "/v1")
	if strings.HasSuffix(normalized, endpoint) || strings.HasSuffix(normalized, relative) {
		return normalized
	}
	if openAIBaseURLHasVersionSuffix(normalized) {
		return normalized + relative
	}
	return normalized + endpoint
}

func openAIBaseURLHasVersionSuffix(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return false
	}
	pathValue := ""
	if parsed, err := url.Parse(trimmed); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		pathValue = parsed.Path
	} else if slash := strings.Index(trimmed, "/"); slash >= 0 {
		pathValue = trimmed[slash:]
	}
	pathValue = strings.TrimRight(pathValue, "/")
	if pathValue == "" {
		return false
	}
	lastSlash := strings.LastIndex(pathValue, "/")
	segment := pathValue
	if lastSlash >= 0 {
		segment = pathValue[lastSlash+1:]
	}
	return isOpenAIAPIVersionSegment(segment)
}

func isOpenAIAPIVersionSegment(segment string) bool {
	s := strings.ToLower(strings.TrimSpace(segment))
	if len(s) < 2 || s[0] != 'v' || !isASCIIDigit(s[1]) {
		return false
	}
	i := 1
	for i < len(s) && isASCIIDigit(s[i]) {
		i++
	}
	if i == len(s) {
		return true
	}
	if s[i] == '.' {
		i++
		if i == len(s) || !isASCIIDigit(s[i]) {
			return false
		}
		for i < len(s) && isASCIIDigit(s[i]) {
			i++
		}
		return i == len(s)
	}
	suffix := s[i:]
	return strings.HasPrefix(suffix, "alpha") || strings.HasPrefix(suffix, "beta") || strings.HasPrefix(suffix, "preview")
}

func isASCIIDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

type publicLimiter struct {
	mu      sync.Mutex
	entries map[string]limitBucket
}

type limitBucket struct {
	Count   int
	ResetAt time.Time
}

func newPublicLimiter() *publicLimiter {
	return &publicLimiter{entries: map[string]limitBucket{}}
}

func (l *publicLimiter) allow(now time.Time, clientIP string, keyHash string) error {
	if l == nil {
		return nil
	}
	ipKey := strings.TrimSpace(clientIP)
	if ipKey == "" {
		ipKey = "unknown"
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.cleanup(now)
	if !l.allowBucket(now, "ip-hour:"+ipKey, 5, time.Hour) ||
		!l.allowBucket(now, "ip-day:"+ipKey, 20, 24*time.Hour) ||
		!l.allowBucket(now, "key-hour:"+keyHash, 3, time.Hour) {
		return infraerrors.TooManyRequests("PURITY_RATE_LIMITED", "too many purity checks; retry later")
	}
	return nil
}

func (l *publicLimiter) allowBucket(now time.Time, key string, limit int, window time.Duration) bool {
	bucket := l.entries[key]
	if bucket.ResetAt.IsZero() || !now.Before(bucket.ResetAt) {
		bucket = limitBucket{ResetAt: now.Add(window)}
	}
	if bucket.Count >= limit {
		l.entries[key] = bucket
		return false
	}
	bucket.Count++
	l.entries[key] = bucket
	return true
}

func (l *publicLimiter) cleanup(now time.Time) {
	for key, bucket := range l.entries {
		if !bucket.ResetAt.IsZero() && now.After(bucket.ResetAt) {
			delete(l.entries, key)
		}
	}
}
