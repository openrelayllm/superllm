package extension

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"time"

	announcementsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/announcements"
	balancesapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/balances"
	healthapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/health"
	ratesapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/rates"
	sessionsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/sessions"
	usagecostsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/usagecosts"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
)

type ResultProcessor interface {
	ProcessTaskResult(ctx context.Context, task *adminplusdomain.ExtensionTask, result map[string]any) (map[string]any, error)
}

type IngestProcessor struct {
	rates         *ratesapp.Service
	balances      *balancesapp.Service
	announcements *announcementsapp.Service
	health        *healthapp.Service
	billing       *usagecostsapp.Service
	sessions      *sessionsapp.Service
	cipher        SessionCipher
}

type SessionCipher interface {
	Encrypt(plaintext string) (string, error)
}

func NewIngestProcessor(
	rates *ratesapp.Service,
	balances *balancesapp.Service,
	announcements *announcementsapp.Service,
	health *healthapp.Service,
	billing *usagecostsapp.Service,
	sessions *sessionsapp.Service,
) *IngestProcessor {
	return &IngestProcessor{
		rates:         rates,
		balances:      balances,
		announcements: announcements,
		health:        health,
		billing:       billing,
		sessions:      sessions,
	}
}

func NewIngestProcessorWithCipher(
	rates *ratesapp.Service,
	balances *balancesapp.Service,
	announcements *announcementsapp.Service,
	health *healthapp.Service,
	billing *usagecostsapp.Service,
	sessions *sessionsapp.Service,
	cipher SessionCipher,
) *IngestProcessor {
	processor := NewIngestProcessor(rates, balances, announcements, health, billing, sessions)
	processor.cipher = cipher
	return processor
}

func (p *IngestProcessor) ProcessTaskResult(ctx context.Context, task *adminplusdomain.ExtensionTask, result map[string]any) (map[string]any, error) {
	if p == nil || task == nil {
		return nil, nil
	}
	switch task.Type {
	case adminplusdomain.ExtensionTaskTypeCaptureSession:
		return p.processSessionBundle(ctx, task, result)
	case adminplusdomain.ExtensionTaskTypeFetchRates:
		return p.processRates(ctx, task, result)
	case adminplusdomain.ExtensionTaskTypeFetchBalance:
		return p.processBalance(ctx, task, result)
	case adminplusdomain.ExtensionTaskTypeFetchAnnouncements:
		return p.processAnnouncements(ctx, task, result)
	case adminplusdomain.ExtensionTaskTypeFetchHealth:
		return p.processHealth(ctx, task, result)
	case adminplusdomain.ExtensionTaskTypeFetchUsageCosts:
		return p.processUsageCosts(ctx, task, result)
	default:
		return nil, nil
	}
}

func (p *IngestProcessor) processSessionBundle(ctx context.Context, task *adminplusdomain.ExtensionTask, result map[string]any) (map[string]any, error) {
	bundle := mapValue(result, "session_bundle")
	if len(bundle) == 0 {
		return nil, nil
	}
	bundle = normalizeSessionBundle(bundle, task)
	raw, err := json.Marshal(bundle)
	if err != nil {
		return nil, err
	}
	out := map[string]any{
		"session_captured": true,
	}
	summary := sessionSummary(bundle)
	out["session_summary"] = summary
	capturedAt := timeValue(bundle, "captured_at")
	if capturedAt.IsZero() {
		capturedAt = timeValue(result, "captured_at")
	}
	expiresAt := optionalTimeValue(bundle, "expires_at")
	var ciphertext string
	if p.cipher != nil {
		encrypted, err := p.cipher.Encrypt(string(raw))
		if err != nil {
			return nil, err
		}
		ciphertext = encrypted
	}
	if p.sessions != nil {
		_, err := p.sessions.Upsert(ctx, sessionsapp.UpsertInput{
			SupplierID:              task.SupplierID,
			SessionSource:           adminplusdomain.SupplierSessionSourceBrowserExtension,
			Origin:                  stringValue(bundle, "origin"),
			APIBaseURL:              stringValue(mapValue(bundle, "context"), "api_base_url"),
			SessionSummary:          summary,
			SessionBundle:           bundle,
			SessionBundleCiphertext: ciphertext,
			CapturedAt:              capturedAt,
			ExpiresAt:               expiresAt,
			SourceExtensionTaskID:   task.ID,
		})
		if err != nil {
			return nil, err
		}
	}
	delete(result, "session_bundle")
	result["session_summary"] = out["session_summary"]
	return out, nil
}

func (p *IngestProcessor) processRates(ctx context.Context, task *adminplusdomain.ExtensionTask, result map[string]any) (map[string]any, error) {
	entriesRaw, ok := result["entries"].([]any)
	if !ok || len(entriesRaw) == 0 {
		return nil, nil
	}
	entries := make([]ratesapp.RateEntryInput, 0, len(entriesRaw))
	for _, raw := range entriesRaw {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		entries = append(entries, ratesapp.RateEntryInput{
			Model:       stringValue(item, "model"),
			BillingMode: stringValue(item, "billing_mode"),
			PriceItem:   stringValue(item, "price_item"),
			Unit:        stringValue(item, "unit"),
			Currency:    stringValue(item, "currency"),
			PriceMicros: int64Value(item, "price_micros"),
			RawPayload:  mapValue(item, "raw_payload"),
		})
	}
	if len(entries) == 0 {
		return nil, nil
	}
	capturedAt := optionalTimeValue(result, "captured_at")
	ingested, err := p.rates.RecordSnapshot(ctx, ratesapp.RecordSnapshotInput{
		SupplierID:       task.SupplierID,
		Source:           sourceValue(result),
		CapturedAt:       capturedAt,
		ThresholdPercent: float64Value(result, "threshold_percent"),
		Entries:          entries,
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"rate_snapshots": len(ingested.Snapshots),
		"rate_events":    len(ingested.Events),
	}, nil
}

func (p *IngestProcessor) processBalance(ctx context.Context, task *adminplusdomain.ExtensionTask, result map[string]any) (map[string]any, error) {
	capturedAt := optionalTimeValue(result, "captured_at")
	event, snapshot, err := p.balances.RecordSnapshot(ctx, balancesapp.RecordSnapshotInput{
		SupplierID:               task.SupplierID,
		Source:                   sourceValue(result),
		RuntimeStatus:            adminplusdomain.SupplierRuntimeStatus(stringValue(result, "runtime_status")),
		BalanceCents:             int64Value(result, "balance_cents"),
		Currency:                 stringValue(result, "currency"),
		LowBalanceThresholdCents: int64Value(result, "low_balance_threshold_cents"),
		RawPayload:               mapValue(result, "raw_payload"),
		CapturedAt:               capturedAt,
	})
	if err != nil {
		return nil, err
	}
	out := map[string]any{"balance_snapshot_id": snapshot.ID}
	if event != nil {
		out["balance_event_id"] = event.ID
		out["balance_event_type"] = string(event.Type)
	}
	return out, nil
}

func (p *IngestProcessor) processAnnouncements(ctx context.Context, task *adminplusdomain.ExtensionTask, result map[string]any) (map[string]any, error) {
	itemsRaw, ok := result["announcements"].([]any)
	if !ok || len(itemsRaw) == 0 {
		return nil, nil
	}
	count := 0
	for _, raw := range itemsRaw {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		startsAt := optionalTimeValue(item, "starts_at")
		endsAt := optionalTimeValue(item, "ends_at")
		capturedAt := optionalTimeValue(item, "captured_at")
		_, err := p.announcements.RecordAnnouncement(ctx, announcementsapp.RecordAnnouncementInput{
			SupplierID:       task.SupplierID,
			Source:           sourceValue(result),
			Type:             adminplusdomain.AnnouncementType(stringValue(item, "type")),
			Title:            stringValue(item, "title"),
			Description:      stringValue(item, "description"),
			Currency:         stringValue(item, "currency"),
			MinRechargeCents: int64Value(item, "min_recharge_cents"),
			BonusPercent:     optionalFloat64Value(item, "bonus_percent"),
			DiscountPercent:  optionalFloat64Value(item, "discount_percent"),
			RuntimeStatus:    adminplusdomain.SupplierRuntimeStatus(stringValue(item, "runtime_status")),
			BalanceCents:     int64Value(item, "balance_cents"),
			StartsAt:         startsAt,
			EndsAt:           endsAt,
			CapturedAt:       capturedAt,
			RawPayload:       mapValue(item, "raw_payload"),
		})
		if err != nil {
			return nil, err
		}
		count++
	}
	return map[string]any{"announcement_events": count}, nil
}

func (p *IngestProcessor) processHealth(ctx context.Context, task *adminplusdomain.ExtensionTask, result map[string]any) (map[string]any, error) {
	capturedAt := optionalTimeValue(result, "captured_at")
	ingested, err := p.health.RecordSample(ctx, healthapp.RecordSampleInput{
		SupplierID:                   task.SupplierID,
		Source:                       sourceValue(result),
		Model:                        stringValue(result, "model"),
		FirstTokenLatencyMS:          int64Value(result, "first_token_latency_ms"),
		TotalLatencyMS:               int64Value(result, "total_latency_ms"),
		StatusCode:                   intValue(result, "status_code"),
		ErrorClass:                   stringValue(result, "error_class"),
		ObservedConcurrency:          intValue(result, "observed_concurrency"),
		AvailableConcurrency:         optionalIntValue(result, "available_concurrency"),
		ConcurrencyLimit:             optionalIntValue(result, "concurrency_limit"),
		FirstTokenThresholdMS:        int64Value(result, "first_token_threshold_ms"),
		TotalLatencyThresholdMS:      int64Value(result, "total_latency_threshold_ms"),
		ConcurrencySaturationPercent: float64Value(result, "concurrency_saturation_percent"),
		RawPayload:                   mapValue(result, "raw_payload"),
		CapturedAt:                   capturedAt,
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"health_sample_id": ingested.Sample.ID,
		"health_events":    len(ingested.Events),
	}, nil
}

func (p *IngestProcessor) processUsageCosts(ctx context.Context, task *adminplusdomain.ExtensionTask, result map[string]any) (map[string]any, error) {
	linesRaw, ok := result["lines"].([]any)
	if !ok || len(linesRaw) == 0 {
		return nil, nil
	}
	lines := make([]usagecostsapp.ImportUsageCostLineInput, 0, len(linesRaw))
	for _, raw := range linesRaw {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		startedAt := timeValue(item, "started_at")
		if startedAt.IsZero() {
			continue
		}
		lines = append(lines, usagecostsapp.ImportUsageCostLineInput{
			SupplierID:          task.SupplierID,
			Source:              sourceValue(result),
			ExternalUsageCostID: stringValue(item, "external_usage_cost_id"),
			ExternalRequestID:   stringValue(item, "external_request_id"),
			APIKeyName:          stringValue(item, "api_key_name"),
			Model:               stringValue(item, "model"),
			Endpoint:            stringValue(item, "endpoint"),
			RequestType:         stringValue(item, "request_type"),
			BillingMode:         stringValue(item, "billing_mode"),
			ReasoningEffort:     stringValue(item, "reasoning_effort"),
			Currency:            stringValue(item, "currency"),
			CostCents:           int64Value(item, "cost_cents"),
			InputTokens:         int64Value(item, "input_tokens"),
			OutputTokens:        int64Value(item, "output_tokens"),
			CacheReadTokens:     int64Value(item, "cache_read_tokens"),
			TotalTokens:         int64Value(item, "total_tokens"),
			FirstTokenMS:        int64Value(item, "first_token_ms"),
			DurationMS:          int64Value(item, "duration_ms"),
			UserAgent:           stringValue(item, "user_agent"),
			StartedAt:           startedAt,
			EndedAt:             optionalTimeValue(item, "ended_at"),
			RawPayload:          mapValue(item, "raw_payload"),
		})
	}
	if len(lines) == 0 {
		return nil, nil
	}
	created, err := p.billing.ImportUsageCostLines(ctx, lines)
	if err != nil {
		return nil, err
	}
	return map[string]any{"usage_cost_lines": len(created)}, nil
}

func sourceValue(values map[string]any) string {
	value := stringValue(values, "source")
	if value == "" {
		return "chrome"
	}
	return value
}

func stringValue(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	if value, ok := values[key].(string); ok {
		return value
	}
	return ""
}

func intValue(values map[string]any, key string) int {
	return int(int64Value(values, key))
}

func int64Value(values map[string]any, key string) int64 {
	if values == nil {
		return 0
	}
	switch value := values[key].(type) {
	case int:
		return int64(value)
	case int64:
		return value
	case float64:
		return int64(math.Round(value))
	case json.Number:
		parsed, err := value.Int64()
		if err == nil {
			return parsed
		}
		parsedFloat, err := value.Float64()
		if err == nil {
			return int64(math.Round(parsedFloat))
		}
		return 0
	default:
		return 0
	}
}

func float64Value(values map[string]any, key string) float64 {
	if values == nil {
		return 0
	}
	switch value := values[key].(type) {
	case float64:
		return value
	case int:
		return float64(value)
	case int64:
		return float64(value)
	case json.Number:
		parsed, err := value.Float64()
		if err == nil {
			return parsed
		}
		return 0
	default:
		return 0
	}
}

func optionalFloat64Value(values map[string]any, key string) *float64 {
	if values == nil {
		return nil
	}
	if _, ok := values[key]; !ok || values[key] == nil {
		return nil
	}
	v := float64Value(values, key)
	return &v
}

func optionalIntValue(values map[string]any, key string) *int {
	if values == nil {
		return nil
	}
	if _, ok := values[key]; !ok || values[key] == nil {
		return nil
	}
	v := intValue(values, key)
	return &v
}

func mapValue(values map[string]any, key string) map[string]any {
	if values == nil {
		return nil
	}
	if value, ok := values[key].(map[string]any); ok {
		return value
	}
	return nil
}

func optionalTimeValue(values map[string]any, key string) *time.Time {
	t := timeValue(values, key)
	if t.IsZero() {
		return nil
	}
	return &t
}

func timeValue(values map[string]any, key string) time.Time {
	raw := stringValue(values, key)
	if raw == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}
	}
	return parsed.UTC()
}

func mergeIngestResult(result map[string]any, ingest map[string]any) map[string]any {
	if len(ingest) == 0 {
		return result
	}
	out := make(map[string]any, len(result)+1)
	for key, value := range result {
		out[key] = value
	}
	out["ingest"] = ingest
	return out
}

func sessionSummary(bundle map[string]any) map[string]any {
	tokens := mapValue(bundle, "tokens")
	cookiesRaw, _ := bundle["cookies"].([]any)
	context := mapValue(bundle, "context")
	requiredHeaders := mapValue(bundle, "required_headers")
	providerType := providerTypeFromBundle(bundle)
	accessToken := firstNonEmptyString(
		stringValue(bundle, "access_token"),
		stringValue(bundle, "accessToken"),
		stringValue(tokens, "access_token"),
		stringValue(tokens, "accessToken"),
	)
	refreshToken := firstNonEmptyString(
		stringValue(bundle, "refresh_token"),
		stringValue(bundle, "refreshToken"),
		stringValue(tokens, "refresh_token"),
		stringValue(tokens, "refreshToken"),
	)
	csrfToken := firstNonEmptyString(
		stringValue(bundle, "csrf_token"),
		stringValue(bundle, "csrfToken"),
		stringValue(tokens, "csrf_token"),
		stringValue(tokens, "csrfToken"),
	)
	cookieCount := len(cookiesRaw)
	if cookieCount == 0 && firstNonEmptyString(stringValue(requiredHeaders, "cookie"), stringValue(bundle, "cookie"), stringValue(bundle, "cookies")) != "" {
		cookieCount = 1
	}
	return map[string]any{
		"origin":               stringValue(bundle, "origin"),
		"provider_type":        providerType,
		"captured_at":          stringValue(bundle, "captured_at"),
		"expires_at":           stringValue(bundle, "expires_at"),
		"has_access_token":     accessToken != "",
		"has_refresh_token":    refreshToken != "",
		"has_csrf_token":       csrfToken != "",
		"cookie_count":         cookieCount,
		"user_id":              stringValue(context, "user_id"),
		"organization_id":      stringValue(context, "organization_id"),
		"project_id":           stringValue(context, "project_id"),
		"account_id":           stringValue(context, "account_id"),
		"api_base_url":         stringValue(context, "api_base_url"),
		"has_required_origin":  strings.TrimSpace(stringValue(requiredHeaders, "origin")) != "",
		"has_required_referer": strings.TrimSpace(stringValue(requiredHeaders, "referer")) != "",
		"has_new_api_user_header": firstNonEmptyString(
			stringValue(requiredHeaders, "New-Api-User"),
			stringValue(requiredHeaders, "New-API-User"),
			stringValue(requiredHeaders, "new-api-user"),
		) != "",
	}
}

func normalizeSessionBundle(bundle map[string]any, task *adminplusdomain.ExtensionTask) map[string]any {
	providerType := providerTypeFromBundle(bundle)
	if providerType == "" && task != nil {
		providerType = normalizeProviderType(firstNonEmptyString(
			stringValue(task.Payload, "provider_type"),
			stringValue(task.Payload, "supplier_type"),
		))
	}
	if providerType != "new_api" {
		return bundle
	}
	context := mapValue(bundle, "context")
	if context == nil {
		context = map[string]any{}
		bundle["context"] = context
	}
	requiredHeaders := mapValue(bundle, "required_headers")
	if requiredHeaders == nil {
		requiredHeaders = map[string]any{}
		bundle["required_headers"] = requiredHeaders
	}
	userID := firstNonEmptyString(
		stringValue(requiredHeaders, "New-Api-User"),
		stringValue(requiredHeaders, "New-API-User"),
		stringValue(requiredHeaders, "new-api-user"),
		stringValue(bundle, "auth_header_value"),
		stringValue(context, "user_id"),
		stringValue(context, "id"),
	)
	bundle["provider_type"] = "new_api"
	bundle["system_type"] = "new_api"
	bundle["auth_header_name"] = "New-Api-User"
	bundle["auth_header_value"] = userID
	context["provider_type"] = "new_api"
	context["system_type"] = "new_api"
	context["user_id"] = userID
	if firstNonEmptyString(stringValue(context, "api_base_url")) == "" {
		context["api_base_url"] = firstNonEmptyString(stringValue(bundle, "api_base_url"), stringValue(bundle, "origin"))
	}
	if userID != "" {
		requiredHeaders["New-Api-User"] = userID
	}
	return bundle
}

func providerTypeFromBundle(bundle map[string]any) string {
	context := mapValue(bundle, "context")
	return normalizeProviderType(firstNonEmptyString(
		stringValue(bundle, "provider_type"),
		stringValue(bundle, "system_type"),
		stringValue(context, "provider_type"),
		stringValue(context, "system_type"),
	))
}

func normalizeProviderType(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "newapi", "new-api":
		return "new_api"
	default:
		return normalized
	}
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func ingestError(err error) error {
	return err
}
