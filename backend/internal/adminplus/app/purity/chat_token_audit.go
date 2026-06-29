package purity

import (
	"context"
	"fmt"
	"net/http"
)

const chatTokenAuditSamples = 3

func (s *Service) runChatCompletionsTokenAudit(ctx context.Context, client *http.Client, baseURL string, apiKey string, model string, emitSample func(TokenAuditSample)) *TokenAuditReport {
	pricing := openAIModelPricingFor(model)
	report := &TokenAuditReport{
		Status:      CheckStatusWarn,
		Summary:     "Chat Completions usage 回退审计样本不足。",
		PriceSource: pricing.Source,
		Anomalies:   []string{"chat_completions_audit_fallback"},
		Samples:     make([]TokenAuditSample, 0, chatTokenAuditSamples),
	}
	for i := 1; i <= chatTokenAuditSamples; i++ {
		prompt := fmt.Sprintf("proxyai.best usage audit fallback round %02d. Return exactly: ok", i)
		probe := s.probeChatCompletionsWithPrompt(ctx, client, baseURL, apiKey, model, prompt, 1)
		sample := chatCompletionsTokenAuditSampleFromProbe(i, probe, pricing)
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
		if sample.CacheCreationFieldPresent {
			report.CacheCreationFieldObserved = true
		}
		if sample.CachedTokensFieldPresent {
			report.CachedTokensFieldObserved = true
		}
		report.OfficialBaselineUSD += sample.OfficialBaselineUSD
		report.ActualCostUSD += sample.ActualCostUSD
		report.UncachedBaselineUSD += sample.UncachedBaselineUSD
	}
	report.OfficialBaselineUSD = roundMoney(report.OfficialBaselineUSD)
	report.ActualCostUSD = roundMoney(report.ActualCostUSD)
	report.UncachedBaselineUSD = roundMoney(report.UncachedBaselineUSD)
	finalizeTokenAudit(report)
	if report.Status == CheckStatusPass {
		report.Status = CheckStatusWarn
	}
	if report.SampleCount > 0 && report.ActualCostUSD > 0 {
		report.Summary = "已通过 Chat Completions usage 回退审计；未执行 Responses 状态链和缓存审计。"
	}
	report.Anomalies = appendUniqueString(report.Anomalies, "chat_completions_audit_fallback")
	return report
}

func chatCompletionsTokenAuditSampleFromProbe(index int, probe httpProbe, pricing openAIModelPricing) TokenAuditSample {
	sample := TokenAuditSample{
		Index:        index,
		Round:        index,
		LatencyMS:    probe.LatencyMS,
		Status:       CheckStatusFail,
		StatusCode:   probe.StatusCode,
		ErrorClass:   probe.ErrorClass,
		ErrorMessage: probe.ErrorMessage,
	}
	if probe.StatusCode < 200 || probe.StatusCode >= 300 {
		applyTokenAuditProbeFailure(&sample, probe, "Chat Completions 用量审计请求未成功")
		return sample
	}
	usage := parseChatCompletionsUsage(probe.Body)
	if usage == nil {
		applyTokenAuditProbeFailure(&sample, probe, "Chat Completions 响应未返回 usage 字段")
		return sample
	}
	sample.InputTokens = usage.InputTokens
	sample.OutputTokens = usage.OutputTokens
	sample.CacheCreationTokens = usage.CacheCreationTokens
	sample.CacheCreationInputTokens = usage.CacheCreationTokens
	sample.CacheCreationFieldPresent = usage.CacheCreationFieldPresent
	sample.CachedTokens = usage.CachedTokens
	sample.CacheReadInputTokens = usage.CachedTokens
	sample.CachedTokensFieldPresent = usage.CachedTokensFieldPresent
	sample.UncachedInputTokens = maxInt64(0, usage.InputTokens-usage.CachedTokens)
	sample.ReasoningTokens = usage.ReasoningTokens
	sample.TotalTokens = usage.TotalTokens
	sample.UncachedBaselineUSD = roundMoney(tokenCost(usage, pricing, false))
	sample.OfficialBaselineUSD = roundMoney(tokenCost(usage, pricing, true))
	sample.BaselineCostUSD = sample.OfficialBaselineUSD
	sample.ActualCostUSD = sample.OfficialBaselineUSD
	sample.CostUSD = sample.ActualCostUSD
	if sample.OfficialBaselineUSD > 0 {
		sample.Multiplier = roundRatio(sample.ActualCostUSD / sample.OfficialBaselineUSD)
		sample.Ratio = sample.Multiplier
	}
	sample.Status = CheckStatusPass
	applyTokenAuditSampleDerivedFields(&sample)
	return sample
}
