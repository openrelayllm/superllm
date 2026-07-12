package health

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/adminplus/app/notifications"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/tidwall/gjson"
)

const (
	defaultFirstTokenThresholdMS   = int64(3000)
	defaultTotalLatencyThresholdMS = int64(30000)
	defaultProbeModel              = "gpt-5.5"
	defaultAnthropicProbeModel     = "claude-sonnet-4-6"
	defaultProbeTimeout            = 45 * time.Second
	defaultProbePrompt             = "Return exactly: ok"
	maxProbeErrorBodyBytes         = 64 * 1024
)

type RecordSampleInput struct {
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

type RecordSampleResult struct {
	Sample *adminplusdomain.HealthSample  `json:"sample"`
	Events []*adminplusdomain.HealthEvent `json:"events"`
}

type SampleFilter struct {
	SupplierID int64
	Model      string
	Limit      int
}

type EventFilter struct {
	SupplierID int64
	Status     adminplusdomain.HealthEventStatus
	Type       adminplusdomain.HealthEventType
	Limit      int
}

type ProbeInput struct {
	SupplierID                   int64
	SupplierAccountID            int64
	Model                        string
	Prompt                       string
	FirstTokenThresholdMS        int64
	TotalLatencyThresholdMS      int64
	ConcurrencySaturationPercent float64
}

type SyncFromSessionInput struct {
	SupplierID                   int64
	Model                        string
	FirstTokenThresholdMS        int64
	TotalLatencyThresholdMS      int64
	ConcurrencySaturationPercent float64
}

type SyncFromSessionResult struct {
	SupplierID int64               `json:"supplier_id"`
	SyncedAt   time.Time           `json:"synced_at"`
	Total      int                 `json:"total"`
	Result     *RecordSampleResult `json:"result"`
}

type Repository interface {
	CreateSample(ctx context.Context, sample *adminplusdomain.HealthSample) (*adminplusdomain.HealthSample, error)
	CreateEvent(ctx context.Context, event *adminplusdomain.HealthEvent) (*adminplusdomain.HealthEvent, error)
	ListSamples(ctx context.Context, filter SampleFilter) ([]*adminplusdomain.HealthSample, error)
	ListEvents(ctx context.Context, filter EventFilter) ([]*adminplusdomain.HealthEvent, error)
	UpdateEventStatus(ctx context.Context, id int64, status adminplusdomain.HealthEventStatus) (*adminplusdomain.HealthEvent, error)
	GetProbeTarget(ctx context.Context, supplierID int64, supplierAccountID int64) (*ProbeTarget, error)
}

type Notifier interface {
	NotifyHealthEvent(ctx context.Context, event *adminplusdomain.HealthEvent, sample *adminplusdomain.HealthSample) error
}

type ProbeTarget struct {
	SupplierID              int64
	SupplierName            string
	SupplierAPIBaseURL      string
	SupplierAccountID       int64
	LocalAccountID          int64
	LocalAccountName        string
	LocalAccountPlatform    string
	LocalAccountType        string
	LocalAccountStatus      string
	LocalAccountSchedulable bool
	LocalAccountConcurrency int
	APIKey                  string
	AccountBaseURL          string
}

type Service struct {
	repo       Repository
	notifier   Notifier
	now        func() time.Time
	httpClient *http.Client
}

func NewService(repo Repository) *Service {
	return NewServiceWithNotifier(repo, nil)
}

func NewServiceWithNotifier(repo Repository, notifier Notifier) *Service {
	return &Service{
		repo:       repo,
		notifier:   notifier,
		now:        time.Now,
		httpClient: &http.Client{Timeout: defaultProbeTimeout},
	}
}

func (s *Service) RecordSample(ctx context.Context, in RecordSampleInput) (*RecordSampleResult, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("health service is not configured")
	}
	sample, thresholds, err := s.buildSample(in)
	if err != nil {
		return nil, err
	}
	created, err := s.repo.CreateSample(ctx, sample)
	if err != nil {
		return nil, err
	}
	events := buildHealthEvents(created, thresholds)
	result := &RecordSampleResult{
		Sample: created,
		Events: make([]*adminplusdomain.HealthEvent, 0, len(events)),
	}
	for _, event := range events {
		createdEvent, err := s.repo.CreateEvent(ctx, event)
		if err != nil {
			return nil, err
		}
		result.Events = append(result.Events, createdEvent)
		s.notifyHealthEvent(ctx, createdEvent, created)
	}
	return result, nil
}

func (s *Service) notifyHealthEvent(_ context.Context, _ *adminplusdomain.HealthEvent, _ *adminplusdomain.HealthSample) {
	// Health events stay in SuperLLM history; Feishu push is intentionally balance-only.
}

type FeishuNotifier struct {
	service *notifications.Service
}

func NewFeishuNotifierFromEnv(repo notifications.Repository) *FeishuNotifier {
	if repo == nil {
		return nil
	}
	return &FeishuNotifier{service: notifications.NewService(repo)}
}

func NewFeishuNotifier(service *notifications.Service) *FeishuNotifier {
	if service == nil {
		return nil
	}
	return &FeishuNotifier{service: service}
}

func (n *FeishuNotifier) NotifyHealthEvent(ctx context.Context, event *adminplusdomain.HealthEvent, sample *adminplusdomain.HealthSample) error {
	if n == nil || n.service == nil || event == nil {
		return nil
	}
	return n.service.Dispatch(ctx, notifications.DispatchInput{
		Type:           "health." + string(event.Type),
		ID:             event.ID,
		SupplierID:     event.SupplierID,
		ThrottleKey:    fmt.Sprintf("supplier:%d:model:%s:type:%s", event.SupplierID, event.Model, event.Type),
		ThrottleWindow: notifications.DefaultThrottleWindow,
		Text:           buildFeishuHealthText(event, sample),
	})
}

func buildFeishuHealthText(event *adminplusdomain.HealthEvent, sample *adminplusdomain.HealthSample) string {
	statusCode := event.StatusCode
	source := "-"
	capturedAt := event.CreatedAt
	if sample != nil {
		source = sample.Source
		capturedAt = sample.CapturedAt
		if statusCode == 0 {
			statusCode = sample.StatusCode
		}
	}
	return fmt.Sprintf(
		"【SuperLLM 健康通知】\n供应商ID：%d\n模型：%s\n事件：%s\n观测值：%d\n阈值：%d\nHTTP：%d\n错误：%s\n来源：%s\n时间：%s",
		event.SupplierID,
		event.Model,
		event.Type,
		event.ObservedValue,
		event.ThresholdValue,
		statusCode,
		event.ErrorClass,
		source,
		capturedAt.Format(time.RFC3339),
	)
}

func (s *Service) ListSamples(ctx context.Context, filter SampleFilter) ([]*adminplusdomain.HealthSample, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("health service is not configured")
	}
	if filter.SupplierID < 0 {
		return nil, badRequest("HEALTH_SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	filter.Model = strings.TrimSpace(filter.Model)
	filter.Limit = normalizeLimit(filter.Limit)
	return s.repo.ListSamples(ctx, filter)
}

func (s *Service) ListEvents(ctx context.Context, filter EventFilter) ([]*adminplusdomain.HealthEvent, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("health service is not configured")
	}
	if filter.SupplierID < 0 {
		return nil, badRequest("HEALTH_SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	if filter.Status != "" && !filter.Status.Valid() {
		return nil, badRequest("HEALTH_EVENT_STATUS_INVALID", "invalid health event status")
	}
	filter.Limit = normalizeLimit(filter.Limit)
	return s.repo.ListEvents(ctx, filter)
}

func (s *Service) AcknowledgeEvent(ctx context.Context, id int64) (*adminplusdomain.HealthEvent, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("health service is not configured")
	}
	if id <= 0 {
		return nil, badRequest("HEALTH_EVENT_ID_INVALID", "invalid health event id")
	}
	return s.repo.UpdateEventStatus(ctx, id, adminplusdomain.HealthEventStatusAcknowledged)
}

func (s *Service) ProbeOpenAIResponses(ctx context.Context, in ProbeInput) (*RecordSampleResult, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("health service is not configured")
	}
	if in.SupplierID <= 0 {
		return nil, badRequest("HEALTH_SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	target, err := s.repo.GetProbeTarget(ctx, in.SupplierID, in.SupplierAccountID)
	if err != nil {
		return nil, err
	}
	model := strings.TrimSpace(in.Model)
	if model == "" {
		model = defaultProbeModel
	}
	prompt := strings.TrimSpace(in.Prompt)
	if prompt == "" {
		prompt = defaultProbePrompt
	}
	baseURL, err := probeBaseURL(target)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(target.APIKey) == "" {
		return nil, infraerrors.New(http.StatusConflict, "HEALTH_PROBE_API_KEY_REQUIRED", "local Sub2API account api key is required for health probe")
	}

	startedAt := s.now().UTC()
	probe, err := s.executeOpenAIResponsesProbe(ctx, baseURL, target.APIKey, model, prompt)
	if err != nil {
		return nil, err
	}
	firstTokenMS := probe.FirstTokenLatencyMS
	totalMS := probe.TotalLatencyMS
	errorClass := probe.ErrorClass
	source := "responses_probe"
	rawPayload := map[string]any{
		"provider":                    "openai",
		"api_mode":                    "responses",
		"model":                       model,
		"endpoint":                    redactProbeURL(joinURL(baseURL, "/v1/responses")),
		"supplier_account_id":         target.SupplierAccountID,
		"local_sub2api_account_id":    target.LocalAccountID,
		"local_sub2api_account_name":  target.LocalAccountName,
		"local_sub2api_account_type":  target.LocalAccountType,
		"local_sub2api_account_state": target.LocalAccountStatus,
		"configured_concurrency":      target.LocalAccountConcurrency,
		"response_text_present":       probe.ResponseTextPresent,
		"error_message":               probe.ErrorMessage,
	}
	availableConcurrency := positiveIntPtr(target.LocalAccountConcurrency)
	return s.RecordSample(ctx, RecordSampleInput{
		SupplierID:                   in.SupplierID,
		Source:                       source,
		Model:                        model,
		FirstTokenLatencyMS:          firstTokenMS,
		TotalLatencyMS:               totalMS,
		StatusCode:                   probe.StatusCode,
		ErrorClass:                   errorClass,
		ObservedConcurrency:          0,
		AvailableConcurrency:         availableConcurrency,
		ConcurrencyLimit:             positiveIntPtr(target.LocalAccountConcurrency),
		FirstTokenThresholdMS:        in.FirstTokenThresholdMS,
		TotalLatencyThresholdMS:      in.TotalLatencyThresholdMS,
		ConcurrencySaturationPercent: in.ConcurrencySaturationPercent,
		RawPayload:                   rawPayload,
		CapturedAt:                   &startedAt,
	})
}

func (s *Service) ProbeAnthropicMessages(ctx context.Context, in ProbeInput) (*RecordSampleResult, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("health service is not configured")
	}
	if in.SupplierID <= 0 {
		return nil, badRequest("HEALTH_SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	target, err := s.repo.GetProbeTarget(ctx, in.SupplierID, in.SupplierAccountID)
	if err != nil {
		return nil, err
	}
	model := strings.TrimSpace(in.Model)
	if model == "" {
		model = defaultAnthropicProbeModel
	}
	prompt := strings.TrimSpace(in.Prompt)
	if prompt == "" {
		prompt = defaultProbePrompt
	}
	baseURL, err := probeBaseURL(target)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(target.APIKey) == "" {
		return nil, infraerrors.New(http.StatusConflict, "HEALTH_PROBE_API_KEY_REQUIRED", "local Sub2API account api key is required for health probe")
	}

	startedAt := s.now().UTC()
	probe, err := s.executeAnthropicMessagesProbe(ctx, baseURL, target.APIKey, model, prompt)
	if err != nil {
		return nil, err
	}
	rawPayload := map[string]any{
		"provider":                    "anthropic",
		"api_mode":                    "messages",
		"model":                       model,
		"endpoint":                    redactProbeURL(joinURL(baseURL, "/v1/messages")),
		"supplier_account_id":         target.SupplierAccountID,
		"local_sub2api_account_id":    target.LocalAccountID,
		"local_sub2api_account_name":  target.LocalAccountName,
		"local_sub2api_account_type":  target.LocalAccountType,
		"local_sub2api_account_state": target.LocalAccountStatus,
		"configured_concurrency":      target.LocalAccountConcurrency,
		"response_text_present":       probe.ResponseTextPresent,
		"error_message":               probe.ErrorMessage,
	}
	return s.RecordSample(ctx, RecordSampleInput{
		SupplierID:                   in.SupplierID,
		Source:                       "anthropic_messages_probe",
		Model:                        model,
		FirstTokenLatencyMS:          probe.FirstTokenLatencyMS,
		TotalLatencyMS:               probe.TotalLatencyMS,
		StatusCode:                   probe.StatusCode,
		ErrorClass:                   probe.ErrorClass,
		ObservedConcurrency:          0,
		AvailableConcurrency:         positiveIntPtr(target.LocalAccountConcurrency),
		ConcurrencyLimit:             positiveIntPtr(target.LocalAccountConcurrency),
		FirstTokenThresholdMS:        in.FirstTokenThresholdMS,
		TotalLatencyThresholdMS:      in.TotalLatencyThresholdMS,
		ConcurrencySaturationPercent: in.ConcurrencySaturationPercent,
		RawPayload:                   rawPayload,
		CapturedAt:                   &startedAt,
	})
}

func (s *Service) SyncFromSession(ctx context.Context, in SyncFromSessionInput) (*SyncFromSessionResult, error) {
	if in.SupplierID <= 0 {
		return nil, badRequest("HEALTH_SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	result, err := s.ProbeOpenAIResponses(ctx, ProbeInput{
		SupplierID:                   in.SupplierID,
		Model:                        in.Model,
		FirstTokenThresholdMS:        in.FirstTokenThresholdMS,
		TotalLatencyThresholdMS:      in.TotalLatencyThresholdMS,
		ConcurrencySaturationPercent: in.ConcurrencySaturationPercent,
	})
	if err != nil {
		return nil, err
	}
	syncedAt := s.now().UTC()
	total := 0
	if result != nil && result.Sample != nil {
		syncedAt = result.Sample.CapturedAt
		total = 1
	}
	return &SyncFromSessionResult{
		SupplierID: in.SupplierID,
		SyncedAt:   syncedAt,
		Total:      total,
		Result:     result,
	}, nil
}

type healthThresholds struct {
	firstTokenMS          int64
	totalLatencyMS        int64
	concurrencySaturation float64
}

func (s *Service) buildSample(in RecordSampleInput) (*adminplusdomain.HealthSample, healthThresholds, error) {
	if in.SupplierID <= 0 {
		return nil, healthThresholds{}, badRequest("HEALTH_SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	model := strings.TrimSpace(in.Model)
	if model == "" {
		return nil, healthThresholds{}, badRequest("HEALTH_MODEL_REQUIRED", "health model is required")
	}
	if in.FirstTokenLatencyMS < 0 || in.TotalLatencyMS < 0 {
		return nil, healthThresholds{}, badRequest("HEALTH_LATENCY_INVALID", "latency must be non-negative")
	}
	if in.ObservedConcurrency < 0 {
		return nil, healthThresholds{}, badRequest("HEALTH_CONCURRENCY_INVALID", "observed concurrency must be non-negative")
	}
	if in.AvailableConcurrency != nil && *in.AvailableConcurrency < 0 {
		return nil, healthThresholds{}, badRequest("HEALTH_AVAILABLE_CONCURRENCY_INVALID", "available concurrency must be non-negative")
	}
	if in.ConcurrencyLimit != nil && *in.ConcurrencyLimit < 0 {
		return nil, healthThresholds{}, badRequest("HEALTH_CONCURRENCY_LIMIT_INVALID", "concurrency limit must be non-negative")
	}
	thresholds := healthThresholds{
		firstTokenMS:          in.FirstTokenThresholdMS,
		totalLatencyMS:        in.TotalLatencyThresholdMS,
		concurrencySaturation: in.ConcurrencySaturationPercent,
	}
	if thresholds.firstTokenMS <= 0 {
		thresholds.firstTokenMS = defaultFirstTokenThresholdMS
	}
	if thresholds.totalLatencyMS <= 0 {
		thresholds.totalLatencyMS = defaultTotalLatencyThresholdMS
	}
	if thresholds.concurrencySaturation <= 0 || thresholds.concurrencySaturation > 100 {
		thresholds.concurrencySaturation = 100
	}
	capturedAt := s.now().UTC()
	if in.CapturedAt != nil {
		capturedAt = in.CapturedAt.UTC()
	}
	return &adminplusdomain.HealthSample{
		SupplierID:           in.SupplierID,
		Source:               normalizeSource(in.Source),
		Model:                model,
		FirstTokenLatencyMS:  in.FirstTokenLatencyMS,
		TotalLatencyMS:       in.TotalLatencyMS,
		StatusCode:           in.StatusCode,
		ErrorClass:           trimLimit(in.ErrorClass, 80),
		ObservedConcurrency:  in.ObservedConcurrency,
		AvailableConcurrency: cloneInt(in.AvailableConcurrency),
		ConcurrencyLimit:     cloneInt(in.ConcurrencyLimit),
		RawPayload:           in.RawPayload,
		CapturedAt:           capturedAt,
	}, thresholds, nil
}

func buildHealthEvents(sample *adminplusdomain.HealthSample, thresholds healthThresholds) []*adminplusdomain.HealthEvent {
	if sample == nil {
		return nil
	}
	events := make([]*adminplusdomain.HealthEvent, 0, 4)
	if sample.FirstTokenLatencyMS > thresholds.firstTokenMS {
		events = append(events, newHealthEvent(sample, adminplusdomain.HealthEventTypeSlowFirstToken, sample.FirstTokenLatencyMS, thresholds.firstTokenMS))
	}
	if sample.TotalLatencyMS > thresholds.totalLatencyMS {
		events = append(events, newHealthEvent(sample, adminplusdomain.HealthEventTypeSlowTotal, sample.TotalLatencyMS, thresholds.totalLatencyMS))
	}
	if sample.StatusCode >= 400 || sample.ErrorClass != "" {
		observed := int64(sample.StatusCode)
		if observed == 0 {
			observed = 1
		}
		events = append(events, newHealthEvent(sample, adminplusdomain.HealthEventTypeRequestError, observed, 0))
	}
	if sample.ConcurrencyLimit != nil && *sample.ConcurrencyLimit > 0 {
		observedPercent := float64(sample.ObservedConcurrency) / float64(*sample.ConcurrencyLimit) * 100
		if observedPercent >= thresholds.concurrencySaturation {
			events = append(events, newHealthEvent(sample, adminplusdomain.HealthEventTypeConcurrencyFull, int64(sample.ObservedConcurrency), int64(*sample.ConcurrencyLimit)))
		}
	}
	return events
}

func newHealthEvent(sample *adminplusdomain.HealthSample, eventType adminplusdomain.HealthEventType, observed int64, threshold int64) *adminplusdomain.HealthEvent {
	return &adminplusdomain.HealthEvent{
		SupplierID:     sample.SupplierID,
		SampleID:       sample.ID,
		Type:           eventType,
		Model:          sample.Model,
		ObservedValue:  observed,
		ThresholdValue: threshold,
		StatusCode:     sample.StatusCode,
		ErrorClass:     sample.ErrorClass,
		Status:         adminplusdomain.HealthEventStatusOpen,
	}
}

func normalizeSource(value string) string {
	v := strings.ToLower(strings.TrimSpace(value))
	if v == "" {
		return "manual"
	}
	if len(v) > 60 {
		return v[:60]
	}
	return v
}

func trimLimit(value string, limit int) string {
	v := strings.TrimSpace(value)
	if len(v) <= limit {
		return v
	}
	return v[:limit]
}

func cloneInt(in *int) *int {
	if in == nil {
		return nil
	}
	v := *in
	return &v
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return 200
	}
	if limit > 1000 {
		return 1000
	}
	return limit
}

func badRequest(reason string, message string) error {
	return infraerrors.New(http.StatusBadRequest, reason, message)
}

func internalError(message string) error {
	return infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_INTERNAL_ERROR", message)
}

type openAIResponsesProbeResult struct {
	StatusCode          int
	FirstTokenLatencyMS int64
	TotalLatencyMS      int64
	ResponseTextPresent bool
	ErrorClass          string
	ErrorMessage        string
}

func (s *Service) executeOpenAIResponsesProbe(ctx context.Context, baseURL string, apiKey string, model string, prompt string) (*openAIResponsesProbeResult, error) {
	started := s.now()
	payload, err := json.Marshal(map[string]any{
		"model":             model,
		"instructions":      "You are a health-check endpoint. Reply briefly.",
		"input":             prompt,
		"max_output_tokens": 16,
		"stream":            true,
		"text":              map[string]any{"verbosity": "low"},
		"reasoning":         map[string]any{"effort": "low"},
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, joinURL(baseURL, "/v1/responses"), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		total := int64(s.now().Sub(started) / time.Millisecond)
		return &openAIResponsesProbeResult{
			StatusCode:     0,
			TotalLatencyMS: total,
			ErrorClass:     "network_error",
			ErrorMessage:   sanitizeProbeMessage(err.Error()),
		}, nil
	}
	defer func() { _ = resp.Body.Close() }()

	result := &openAIResponsesProbeResult{StatusCode: resp.StatusCode}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxProbeErrorBodyBytes))
		result.TotalLatencyMS = int64(s.now().Sub(started) / time.Millisecond)
		result.ErrorClass = errorClassForStatus(resp.StatusCode)
		result.ErrorMessage = sanitizeProbeMessage(string(body))
		return result, nil
	}

	firstToken, textPresent, readErr := readOpenAIResponsesStream(resp.Body, started, s.now)
	result.FirstTokenLatencyMS = firstToken
	result.TotalLatencyMS = int64(s.now().Sub(started) / time.Millisecond)
	result.ResponseTextPresent = textPresent
	if readErr != nil {
		result.ErrorClass = "stream_error"
		result.ErrorMessage = sanitizeProbeMessage(readErr.Error())
	}
	return result, nil
}

func (s *Service) executeAnthropicMessagesProbe(ctx context.Context, baseURL string, apiKey string, model string, prompt string) (*openAIResponsesProbeResult, error) {
	started := s.now()
	payload, err := json.Marshal(map[string]any{
		"model":      model,
		"system":     "You are a health-check endpoint. Reply briefly.",
		"max_tokens": 16,
		"stream":     true,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, joinURL(baseURL, "/v1/messages"), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		total := int64(s.now().Sub(started) / time.Millisecond)
		return &openAIResponsesProbeResult{
			StatusCode:     0,
			TotalLatencyMS: total,
			ErrorClass:     "network_error",
			ErrorMessage:   sanitizeProbeMessage(err.Error()),
		}, nil
	}
	defer func() { _ = resp.Body.Close() }()

	result := &openAIResponsesProbeResult{StatusCode: resp.StatusCode}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxProbeErrorBodyBytes))
		result.TotalLatencyMS = int64(s.now().Sub(started) / time.Millisecond)
		result.ErrorClass = errorClassForStatus(resp.StatusCode)
		result.ErrorMessage = sanitizeProbeMessage(string(body))
		return result, nil
	}

	firstToken, textPresent, readErr := readAnthropicMessagesStream(resp.Body, started, s.now)
	result.FirstTokenLatencyMS = firstToken
	result.TotalLatencyMS = int64(s.now().Sub(started) / time.Millisecond)
	result.ResponseTextPresent = textPresent
	if readErr != nil {
		result.ErrorClass = "stream_error"
		result.ErrorMessage = sanitizeProbeMessage(readErr.Error())
	}
	return result, nil
}

func readOpenAIResponsesStream(body io.Reader, started time.Time, now func() time.Time) (int64, bool, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var firstTokenMS int64
	textPresent := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		if gjson.Get(data, "type").String() == "response.output_text.delta" {
			if strings.TrimSpace(gjson.Get(data, "delta").String()) != "" {
				textPresent = true
				if firstTokenMS == 0 {
					firstTokenMS = int64(now().Sub(started) / time.Millisecond)
				}
			}
			continue
		}
		if strings.TrimSpace(gjson.Get(data, "response.output.0.content.0.text").String()) != "" {
			textPresent = true
			if firstTokenMS == 0 {
				firstTokenMS = int64(now().Sub(started) / time.Millisecond)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return firstTokenMS, textPresent, err
	}
	if firstTokenMS == 0 && textPresent {
		firstTokenMS = int64(now().Sub(started) / time.Millisecond)
	}
	return firstTokenMS, textPresent, nil
}

func readAnthropicMessagesStream(body io.Reader, started time.Time, now func() time.Time) (int64, bool, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var firstTokenMS int64
	textPresent := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		if gjson.Get(data, "type").String() == "error" {
			message := firstNonEmptyString(
				gjson.Get(data, "error.message").String(),
				gjson.Get(data, "error.type").String(),
				"anthropic stream error",
			)
			return firstTokenMS, textPresent, errors.New(message)
		}
		text := firstNonEmptyString(
			gjson.Get(data, "delta.text").String(),
			gjson.Get(data, "content_block.text").String(),
		)
		if strings.TrimSpace(text) != "" {
			textPresent = true
			if firstTokenMS == 0 {
				firstTokenMS = int64(now().Sub(started) / time.Millisecond)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return firstTokenMS, textPresent, err
	}
	if firstTokenMS == 0 && textPresent {
		firstTokenMS = int64(now().Sub(started) / time.Millisecond)
	}
	return firstTokenMS, textPresent, nil
}

func probeBaseURL(target *ProbeTarget) (string, error) {
	if target == nil {
		return "", infraerrors.New(http.StatusNotFound, "HEALTH_PROBE_TARGET_NOT_FOUND", "health probe target not found")
	}
	raw := strings.TrimSpace(target.AccountBaseURL)
	if raw == "" {
		raw = strings.TrimSpace(target.SupplierAPIBaseURL)
	}
	if raw == "" {
		raw = "https://api.openai.com"
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", badRequest("HEALTH_PROBE_BASE_URL_INVALID", "invalid probe base url")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", badRequest("HEALTH_PROBE_BASE_URL_INVALID", "probe base url must use http or https")
	}
	normalized := strings.TrimRight(u.Scheme+"://"+u.Host+strings.TrimRight(u.EscapedPath(), "/"), "/")
	normalized = strings.TrimSuffix(normalized, "/v1")
	return normalized, nil
}

func joinURL(base, path string) string {
	base = strings.TrimRight(base, "/")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return base + path
}

func errorClassForStatus(status int) string {
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return "credential_invalid"
	case status == http.StatusTooManyRequests:
		return "rate_limited"
	case status >= 500:
		return "upstream_5xx"
	case status >= 400:
		return "request_error"
	default:
		return ""
	}
}

func positiveIntPtr(value int) *int {
	if value <= 0 {
		return nil
	}
	v := value
	return &v
}

func sanitizeProbeMessage(message string) string {
	value := strings.TrimSpace(message)
	if value == "" {
		return ""
	}
	if len(value) > 500 {
		value = value[:500]
	}
	return value
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func redactProbeURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	u.RawQuery = ""
	return u.String()
}

func translateNoRows(err error, reason string, message string) error {
	if errors.Is(err, sql.ErrNoRows) {
		return infraerrors.New(http.StatusNotFound, reason, message)
	}
	return err
}
