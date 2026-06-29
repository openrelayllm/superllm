package purity

import (
	"context"
	"fmt"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
)

func (s *Service) runOpenAICheck(ctx context.Context, in PublicCheckInput, emit PublicCheckEventSink, options checkRunOptions) (*PublicReport, error) {
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
		Provider:        ProviderOpenAI,
		ReportID:        newReportID(ProviderOpenAI, host, model, checkedAt),
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
	gatewayProbe := httpProbe{}
	if !officialHost && shouldProbeGatewayRoot(host) {
		gatewayProbe = s.probeGatewayRoot(checkCtx, client, baseURL)
	}

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
		report.HasVertex = hasVertexFingerprint(report.APIBaseHost, modelsProbe.Headers)
		report.IsKiro = hasKiroFingerprint(report.APIBaseHost, modelsProbe.Headers)
		report.WrapperSignals = wrapperFingerprintSignalsForReportWithValues(report, fingerprintValuesFromHTTPProbes(gatewayProbe, modelsProbe), gatewayProbe.Headers, modelsProbe.Headers)
		appendAndEmitModelIdentity(report, emit)
		appendAndEmitWrapperFingerprint(report, emit)
		s.finalizeAndSave(ctx, report, baseURL)
		emitFinalReport(report, emit)
		return report, nil
	}

	emitProgress(report, emit, 2, "structure")
	responsesProbe := s.probeResponsesNonStream(checkCtx, client, baseURL, apiKey, model)
	report.Metrics.ResponsesLatencyMS = responsesProbe.LatencyMS
	report.Metrics.Usage = parseResponsesUsage(responsesProbe.Body)
	report.ResponseModel, report.ResponseModelSource, report.responseBodyModel, report.responseHeaderModel = openAIResponseModelFromProbe(responsesProbe)
	report.NonStreamChannel = channelFromProbe(ProviderOpenAI, host, officialHost, responsesProbe)
	responsesCheck := buildResponsesSchemaCheck(responsesProbe, apiKey)
	toolCheck := buildToolCallCheck(responsesProbe)
	structuredOutputProbe := s.probeResponsesStructuredOutput(checkCtx, client, baseURL, apiKey, model)
	structuredOutputCheck := buildResponsesStructuredOutputCheck(structuredOutputProbe, apiKey)
	usageCheck := buildUsageCheck(report.Metrics.Usage, responsesProbe)
	appendAndEmitChecks(report, emit, responsesCheck, toolCheck, structuredOutputCheck, usageCheck)
	upsertAndEmitValidation(report, emit, validationFromExecutedChecks("schema_integrity", "结构完整性", []CheckResult{responsesCheck, structuredOutputCheck}))
	emitMetrics(report, emit)

	emitProgress(report, emit, 3, "behavior")
	streamProbe := s.probeResponsesStream(checkCtx, client, baseURL, apiKey, model)
	report.Metrics.StreamFirstTokenMS = streamProbe.FirstTokenMS
	report.Metrics.StreamTotalLatencyMS = streamProbe.TotalLatencyMS
	report.StreamChannel = firstNonEmptyString(channelFromStreamProbe(ProviderOpenAI, host, officialHost, streamProbe.Headers), report.NonStreamChannel)
	streamingCheck := buildStreamingCheck(streamProbe, apiKey)
	appendAndEmitChecks(report, emit, streamingCheck)
	report.Metrics.TokensPerSecond = tokensPerSecond(report.Metrics.Usage, streamProbe.TotalLatencyMS)
	upsertAndEmitValidation(report, emit, validationFromExecutedChecks("behavior", "行为验证", []CheckResult{toolCheck, streamingCheck}))
	emitMetrics(report, emit)

	emitProgress(report, emit, 4, "signature")
	storeIncludeProbe := s.probeResponsesStoreInclude(checkCtx, client, baseURL, apiKey, model)
	storeIncludeCheck := buildResponsesStoreIncludeCheck(storeIncludeProbe, apiKey)
	appendAndEmitChecks(report, emit, storeIncludeCheck)
	upsertAndEmitValidation(report, emit, validationFromExecutedChecks("signature", "签名校验", []CheckResult{usageCheck, storeIncludeCheck}))
	emitMetrics(report, emit)

	emitProgress(report, emit, 5, "multimodal")
	multimodalProbe := s.probeResponsesMultimodal(checkCtx, client, baseURL, apiKey, model)
	report.Metrics.MultimodalLatencyMS = multimodalProbe.LatencyMS
	multimodalCheck := buildMultimodalCheck(multimodalProbe, apiKey)
	appendAndEmitChecks(report, emit, multimodalCheck)
	upsertAndEmitValidation(report, emit, validationFromExecutedChecks("multimodal", "多模态能力", []CheckResult{multimodalCheck}))
	emitMetrics(report, emit)

	chatProbe := httpProbe{}
	chatFallbackUsable := false
	if needsChatFallback(report.Checks) {
		chatProbe = s.probeChatCompletions(checkCtx, client, baseURL, apiKey, model)
		report.Metrics.ChatCompletionsLatencyMS = chatProbe.LatencyMS
		chatFallbackCheck := buildChatFallbackCheck(chatProbe, apiKey)
		chatFallbackUsable = chatFallbackCheck.Status == CheckStatusPass
		appendAndEmitChecks(report, emit, chatFallbackCheck)
		if chatFallbackUsable {
			appendAndEmitChecks(report, emit, buildChatCompletionsChoiceCountCheck(chatProbe, apiKey, 2))
		}
		emitMetrics(report, emit)
	}

	report.HasVertex = hasVertexFingerprint(report.APIBaseHost, modelsProbe.Headers, responsesProbe.Headers, streamProbe.Headers)
	report.IsKiro = hasKiroFingerprint(report.APIBaseHost, modelsProbe.Headers, responsesProbe.Headers, streamProbe.Headers)
	report.WrapperSignals = wrapperFingerprintSignalsForReportWithValues(report, fingerprintValuesFromHTTPProbes(gatewayProbe, modelsProbe, responsesProbe, storeIncludeProbe, multimodalProbe, chatProbe), gatewayProbe.Headers, modelsProbe.Headers, responsesProbe.Headers, streamProbe.Headers, storeIncludeProbe.Headers, multimodalProbe.Headers, chatProbe.Headers)
	appendAndEmitModelIdentity(report, emit)
	appendAndEmitWrapperFingerprint(report, emit)

	if in.SkipTokenAudit {
		tokenAuditCheck := CheckResult{ID: "token_audit", Name: "Token 用量审计", Status: CheckStatusWarn, Score: 0, MaxScore: 15, Message: "本次请求已关闭 Token 用量审计。", Details: map[string]any{"skipped": true}}
		appendAndEmitChecks(report, emit, tokenAuditCheck)
		upsertAndEmitValidation(report, emit, validationFromExecutedChecks("token_audit", "Token 用量审计", []CheckResult{tokenAuditCheck}))
	} else if responsesProbe.StatusCode >= 200 && responsesProbe.StatusCode < 300 {
		emitProgress(report, emit, 6, "token_audit")
		billingSnapshot := s.captureBillingUsageSnapshotForAudit(checkCtx, client, ProviderOpenAI, baseURL, apiKey, options)
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
		s.applyTokenAuditBillingMultiplierForAudit(checkCtx, client, ProviderOpenAI, baseURL, apiKey, report.TokenAudit, billingSnapshot, options)
		tokenAuditCheck := buildTokenAuditCheck(report.TokenAudit)
		appendAndEmitChecks(report, emit, tokenAuditCheck)
		upsertAndEmitValidation(report, emit, validationFromExecutedChecks("token_audit", "Token 用量审计", []CheckResult{tokenAuditCheck}))
		emitPublicCheckEvent(emit, PublicCheckEvent{
			Type:       PublicCheckEventTokenAudit,
			ReportID:   report.ReportID,
			TokenAudit: report.TokenAudit,
		})
	} else if chatFallbackUsable {
		emitProgress(report, emit, 6, "token_audit")
		billingSnapshot := s.captureBillingUsageSnapshotForAudit(checkCtx, client, ProviderOpenAI, baseURL, apiKey, options)
		report.TokenAudit = s.runChatCompletionsTokenAudit(checkCtx, client, baseURL, apiKey, model, func(sample TokenAuditSample) {
			report.TokenAuditPartial = upsertTokenAuditPartial(report.TokenAuditPartial, sample)
			report.TokenAuditProgress = fmt.Sprintf("%d/%d", len(report.TokenAuditPartial), chatTokenAuditSamples)
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
		s.applyTokenAuditBillingMultiplierForAudit(checkCtx, client, ProviderOpenAI, baseURL, apiKey, report.TokenAudit, billingSnapshot, options)
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

	emitProgress(report, emit, 7, "evaluate")
	report.Metrics.LatencyMS = int64(s.currentTime().Sub(startedAt) / time.Millisecond)
	s.finalizeAndSave(ctx, report, baseURL)
	emitFinalReport(report, emit)
	return report, nil
}
