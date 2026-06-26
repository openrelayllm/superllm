package balances

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/adminplus/app/bizlogs"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/adminplus/ports"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type RecordSnapshotInput struct {
	SupplierID               int64
	Source                   string
	RuntimeStatus            adminplusdomain.SupplierRuntimeStatus
	BalanceCents             int64
	Currency                 string
	LowBalanceThresholdCents int64
	RawPayload               map[string]any
	CapturedAt               *time.Time
}

type SnapshotFilter struct {
	SupplierID int64
	Limit      int
}

type EventFilter struct {
	SupplierID int64
	Status     adminplusdomain.BalanceEventStatus
	Limit      int
}

type SyncFromSessionInput struct {
	SupplierID               int64
	LowBalanceThresholdCents int64
}

type SyncFromSessionResult struct {
	SupplierID int64                            `json:"supplier_id"`
	SystemType string                           `json:"system_type"`
	Origin     string                           `json:"origin"`
	APIBaseURL string                           `json:"api_base_url"`
	SyncedAt   time.Time                        `json:"synced_at"`
	Probe      *ports.SessionProbeResult        `json:"probe"`
	Snapshot   *adminplusdomain.BalanceSnapshot `json:"snapshot,omitempty"`
	Event      *adminplusdomain.BalanceEvent    `json:"event,omitempty"`
}

type CurrentBalanceInput struct {
	SupplierID               int64
	Refresh                  bool
	LowBalanceThresholdCents int64
}

type CurrentBalance struct {
	SupplierID          int64                                 `json:"supplier_id"`
	RuntimeStatus       adminplusdomain.SupplierRuntimeStatus `json:"runtime_status"`
	BalanceCents        int64                                 `json:"balance_cents"`
	Currency            string                                `json:"currency"`
	SwitchEligible      bool                                  `json:"switch_eligible"`
	Source              string                                `json:"source"`
	CapturedAt          time.Time                             `json:"captured_at"`
	RefreshAfter        time.Time                             `json:"refresh_after"`
	ExpiresAt           time.Time                             `json:"expires_at"`
	Stale               bool                                  `json:"stale"`
	Expired             bool                                  `json:"expired"`
	Fallback            bool                                  `json:"fallback"`
	RefreshErrorReason  string                                `json:"refresh_error_reason,omitempty"`
	RefreshErrorMessage string                                `json:"refresh_error_message,omitempty"`
}

type BalanceCache interface {
	GetCurrent(ctx context.Context, supplierID int64) (*CurrentBalance, error)
	SetCurrent(ctx context.Context, current *CurrentBalance, ttl time.Duration) error
}

type Repository interface {
	CreateSnapshot(ctx context.Context, snapshot *adminplusdomain.BalanceSnapshot) (*adminplusdomain.BalanceSnapshot, error)
	FindLatestSnapshot(ctx context.Context, supplierID int64, currency string, capturedAt time.Time) (*adminplusdomain.BalanceSnapshot, error)
	CreateEvent(ctx context.Context, event *adminplusdomain.BalanceEvent) (*adminplusdomain.BalanceEvent, error)
	ListSnapshots(ctx context.Context, filter SnapshotFilter) ([]*adminplusdomain.BalanceSnapshot, error)
	ListEvents(ctx context.Context, filter EventFilter) ([]*adminplusdomain.BalanceEvent, error)
	UpdateEventStatus(ctx context.Context, id int64, status adminplusdomain.BalanceEventStatus) (*adminplusdomain.BalanceEvent, error)
}

type Notifier interface {
	NotifyBalanceEvent(ctx context.Context, event *adminplusdomain.BalanceEvent, snapshot *adminplusdomain.BalanceSnapshot) error
}

type SessionReader interface {
	DecryptedProbeInput(ctx context.Context, supplierID int64) (ports.SessionProbeInput, error)
}

type Service struct {
	repo     Repository
	notifier Notifier
	session  SessionReader
	reader   ports.SessionProbeAdapter
	cache    BalanceCache
	bizlog   *bizlogs.Recorder
	freshFor time.Duration
	cacheTTL time.Duration
	now      func() time.Time
}

var ErrCurrentBalanceCacheMiss = errors.New("admin plus current balance cache miss")

const (
	defaultCurrentBalanceFreshFor = 6 * time.Minute
	defaultCurrentBalanceCacheTTL = 15 * time.Minute
	legacyNewAPIQuotaUnitsPerUSD  = 500000.0
)

func NewService(repo Repository) *Service {
	return NewServiceWithNotifier(repo, nil)
}

func NewServiceWithNotifier(repo Repository, notifier Notifier) *Service {
	return &Service{
		repo:     repo,
		notifier: notifier,
		freshFor: defaultCurrentBalanceFreshFor,
		cacheTTL: defaultCurrentBalanceCacheTTL,
		now:      time.Now,
	}
}

func NewServiceWithDependencies(repo Repository, notifier Notifier, session SessionReader, reader ports.SessionProbeAdapter) *Service {
	service := NewServiceWithNotifier(repo, notifier)
	service.session = session
	service.reader = reader
	return service
}

func NewServiceWithCurrentCache(repo Repository, notifier Notifier, session SessionReader, reader ports.SessionProbeAdapter, cache BalanceCache) *Service {
	service := NewServiceWithDependencies(repo, notifier, session, reader)
	service.cache = cache
	return service
}

func (s *Service) WithDiagnostics(recorder *bizlogs.Recorder) *Service {
	if s != nil {
		s.bizlog = recorder
	}
	return s
}

func (s *Service) CanSyncFromSession() bool {
	return s != nil && s.repo != nil && s.session != nil && s.reader != nil
}

func (s *Service) RecordSnapshot(ctx context.Context, in RecordSnapshotInput) (*adminplusdomain.BalanceEvent, *adminplusdomain.BalanceSnapshot, error) {
	if s == nil || s.repo == nil {
		return nil, nil, internalError("balance service is not configured")
	}
	snapshot, err := s.buildSnapshot(in)
	if err != nil {
		return nil, nil, err
	}
	previous, err := s.repo.FindLatestSnapshot(ctx, snapshot.SupplierID, snapshot.Currency, snapshot.CapturedAt)
	if err != nil {
		return nil, nil, err
	}
	created, err := s.repo.CreateSnapshot(ctx, snapshot)
	if err != nil {
		return nil, nil, err
	}
	s.cacheCurrentBalance(ctx, created)
	event := buildBalanceEvent(created, previous, in.LowBalanceThresholdCents)
	if event == nil {
		return nil, created, nil
	}
	createdEvent, err := s.repo.CreateEvent(ctx, event)
	if err != nil {
		return nil, nil, err
	}
	s.notifyBalanceEvent(ctx, createdEvent, created)
	return createdEvent, created, nil
}

func (s *Service) notifyBalanceEvent(ctx context.Context, event *adminplusdomain.BalanceEvent, snapshot *adminplusdomain.BalanceSnapshot) {
	if s == nil || s.notifier == nil || event == nil {
		return
	}
	if err := s.notifier.NotifyBalanceEvent(ctx, event, snapshot); err != nil {
		slog.Warn("admin plus balance notification failed", "supplier_id", event.SupplierID, "event_id", event.ID, "type", event.Type, "err", err)
	}
}

func (s *Service) SyncFromSession(ctx context.Context, in SyncFromSessionInput) (*SyncFromSessionResult, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("balance service is not configured")
	}
	if s.session == nil {
		return nil, internalError("supplier browser session service is not configured")
	}
	if s.reader == nil {
		return nil, internalError("supplier provider adapter is not configured")
	}
	if in.SupplierID <= 0 {
		return nil, badRequest("BALANCE_SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	if in.LowBalanceThresholdCents < 0 {
		return nil, badRequest("BALANCE_THRESHOLD_INVALID", "low balance threshold must be non-negative")
	}
	probeInput, err := s.session.DecryptedProbeInput(ctx, in.SupplierID)
	if err != nil {
		s.recordSyncFailure(ctx, in.SupplierID, "decrypt_session", err, nil)
		return nil, err
	}
	probe, err := s.reader.ProbeSub2APIUserProfile(ctx, probeInput)
	if err != nil {
		s.recordSyncFailure(ctx, in.SupplierID, "probe_profile", err, map[string]any{
			"origin":       probeInput.Origin,
			"api_base_url": probeInput.APIBaseURL,
		})
		return nil, err
	}
	result := &SyncFromSessionResult{
		SupplierID: in.SupplierID,
		Probe:      probe,
	}
	if probe != nil {
		result.SystemType = probe.SystemType
		result.Origin = probe.Origin
		result.APIBaseURL = probe.APIBaseURL
		result.SyncedAt = probe.ProbedAt.UTC()
		if result.SyncedAt.IsZero() {
			result.SyncedAt = s.now().UTC()
		}
		if probe.BalanceCents != nil {
			event, snapshot, err := s.RecordSnapshot(ctx, RecordSnapshotInput{
				SupplierID:               in.SupplierID,
				Source:                   "provider_session",
				RuntimeStatus:            runtimeStatusForBalance(*probe.BalanceCents),
				BalanceCents:             *probe.BalanceCents,
				Currency:                 probe.BalanceCurrency,
				LowBalanceThresholdCents: in.LowBalanceThresholdCents,
				RawPayload: map[string]any{
					"provider":     probe.SystemType,
					"profile":      probe.Profile,
					"capabilities": probe.Capabilities,
				},
				CapturedAt: &probe.ProbedAt,
			})
			if err != nil {
				s.recordSyncFailure(ctx, in.SupplierID, "record_snapshot", err, map[string]any{
					"system_type":  probe.SystemType,
					"origin":       probe.Origin,
					"api_base_url": probe.APIBaseURL,
				})
				return nil, err
			}
			result.Event = event
			result.Snapshot = snapshot
		}
	}
	if result.SyncedAt.IsZero() {
		result.SyncedAt = s.now().UTC()
	}
	s.recordSyncSuccess(ctx, result)
	return result, nil
}

func (s *Service) recordSyncFailure(ctx context.Context, supplierID int64, action string, err error, metadata map[string]any) {
	if s == nil || s.bizlog == nil {
		return
	}
	if strings.TrimSpace(action) == "" {
		action = "sync_from_session"
	}
	event := bizlogs.EventFromError(bizlogs.Event{
		Level:      bizlogs.LevelWarn,
		Category:   bizlogs.CategoryBalance,
		Action:     action,
		Outcome:    bizlogs.OutcomeFailed,
		Message:    "supplier balance sync failed",
		SupplierID: supplierID,
		Metadata:   metadata,
	}, err)
	s.bizlog.Record(ctx, event)
}

func (s *Service) recordSyncSuccess(ctx context.Context, result *SyncFromSessionResult) {
	if s == nil || s.bizlog == nil || result == nil {
		return
	}
	metadata := map[string]any{
		"system_type":  result.SystemType,
		"origin":       result.Origin,
		"api_base_url": result.APIBaseURL,
	}
	if result.Probe != nil {
		metadata["probe_status"] = result.Probe.Status
	}
	if result.Snapshot != nil {
		metadata["balance_cents"] = result.Snapshot.BalanceCents
		metadata["currency"] = result.Snapshot.Currency
		metadata["switch_eligible"] = result.Snapshot.SwitchEligible
	}
	if result.Event != nil {
		metadata["event_type"] = string(result.Event.Type)
		metadata["event_status"] = string(result.Event.Status)
	}
	message := "supplier balance sync succeeded"
	if result.Snapshot == nil {
		message = "supplier balance sync succeeded without balance snapshot"
	}
	s.bizlog.Record(ctx, bizlogs.Event{
		Level:        bizlogs.LevelInfo,
		Category:     bizlogs.CategoryBalance,
		Action:       "sync_from_session",
		Outcome:      bizlogs.OutcomeSucceeded,
		Message:      message,
		SupplierID:   result.SupplierID,
		ProviderType: result.SystemType,
		Metadata:     metadata,
	})
}

func (s *Service) GetCurrent(ctx context.Context, in CurrentBalanceInput) (*CurrentBalance, error) {
	if s == nil {
		return nil, internalError("balance service is not configured")
	}
	if in.SupplierID <= 0 {
		return nil, badRequest("BALANCE_SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	if in.LowBalanceThresholdCents < 0 {
		return nil, badRequest("BALANCE_THRESHOLD_INVALID", "low balance threshold must be non-negative")
	}

	var cached *CurrentBalance
	if s.cache != nil && !in.Refresh {
		item, err := s.cache.GetCurrent(ctx, in.SupplierID)
		if err == nil && item != nil {
			cached = s.withFreshness(item)
			if !cached.Stale && !cached.Expired {
				return cached, nil
			}
		} else if err != nil && !errors.Is(err, ErrCurrentBalanceCacheMiss) {
			slog.Warn("admin plus current balance cache read failed", "supplier_id", in.SupplierID, "err", err)
		}
	}

	current, err := s.refreshCurrent(ctx, in)
	if err == nil && current != nil {
		return current, nil
	}
	if err != nil {
		slog.Warn("admin plus current balance refresh failed", "supplier_id", in.SupplierID, "err", err)
	}
	if cached != nil && !cached.Expired {
		cached.RefreshErrorReason = refreshErrorReason(err)
		cached.RefreshErrorMessage = refreshErrorMessage(err)
		return cached, nil
	}
	return fallbackCurrentBalance(in.SupplierID, s.now().UTC(), err), nil
}

func (s *Service) ListSnapshots(ctx context.Context, filter SnapshotFilter) ([]*adminplusdomain.BalanceSnapshot, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("balance service is not configured")
	}
	if filter.SupplierID < 0 {
		return nil, badRequest("BALANCE_SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	filter.Limit = normalizeLimit(filter.Limit)
	items, err := s.repo.ListSnapshots(ctx, filter)
	if err != nil {
		return nil, err
	}
	for index, item := range items {
		items[index] = normalizeBalanceSnapshotForRead(item)
	}
	return items, nil
}

func (s *Service) ListEvents(ctx context.Context, filter EventFilter) ([]*adminplusdomain.BalanceEvent, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("balance service is not configured")
	}
	if filter.SupplierID < 0 {
		return nil, badRequest("BALANCE_SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	if filter.Status != "" && !filter.Status.Valid() {
		return nil, badRequest("BALANCE_EVENT_STATUS_INVALID", "invalid balance event status")
	}
	filter.Limit = normalizeLimit(filter.Limit)
	items, err := s.repo.ListEvents(ctx, filter)
	if err != nil {
		return nil, err
	}
	for index, item := range items {
		items[index] = normalizeBalanceEventForRead(item)
	}
	return items, nil
}

func (s *Service) AcknowledgeEvent(ctx context.Context, id int64) (*adminplusdomain.BalanceEvent, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("balance service is not configured")
	}
	if id <= 0 {
		return nil, badRequest("BALANCE_EVENT_ID_INVALID", "invalid balance event id")
	}
	event, err := s.repo.UpdateEventStatus(ctx, id, adminplusdomain.BalanceEventStatusAcknowledged)
	if err != nil {
		return nil, err
	}
	return normalizeBalanceEventForRead(event), nil
}

func (s *Service) buildSnapshot(in RecordSnapshotInput) (*adminplusdomain.BalanceSnapshot, error) {
	if in.SupplierID <= 0 {
		return nil, badRequest("BALANCE_SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	if in.BalanceCents < 0 {
		return nil, badRequest("BALANCE_AMOUNT_INVALID", "balance must be non-negative")
	}
	if in.LowBalanceThresholdCents < 0 {
		return nil, badRequest("BALANCE_THRESHOLD_INVALID", "low balance threshold must be non-negative")
	}
	balanceCents, currency := normalizeBalanceAmountAndCurrency(in.BalanceCents, in.Currency)
	runtimeStatus := in.RuntimeStatus
	if runtimeStatus == "" {
		runtimeStatus = adminplusdomain.SupplierRuntimeStatusMonitorOnly
	}
	if !runtimeStatus.Valid() {
		return nil, badRequest("BALANCE_RUNTIME_STATUS_INVALID", "invalid supplier runtime status")
	}
	capturedAt := s.now().UTC()
	if in.CapturedAt != nil {
		capturedAt = in.CapturedAt.UTC()
	}
	return &adminplusdomain.BalanceSnapshot{
		SupplierID:     in.SupplierID,
		Source:         normalizeSource(in.Source),
		RuntimeStatus:  runtimeStatus,
		BalanceCents:   balanceCents,
		Currency:       currency,
		SwitchEligible: adminplusdomain.CanUseSupplierForSwitching(runtimeStatus, balanceCents),
		RawPayload:     in.RawPayload,
		CapturedAt:     capturedAt,
	}, nil
}

func (s *Service) refreshCurrent(ctx context.Context, in CurrentBalanceInput) (*CurrentBalance, error) {
	result, err := s.SyncFromSession(ctx, SyncFromSessionInput{
		SupplierID:               in.SupplierID,
		LowBalanceThresholdCents: in.LowBalanceThresholdCents,
	})
	if err != nil {
		return nil, err
	}
	if result == nil || result.Snapshot == nil {
		return nil, infraerrors.New(http.StatusBadGateway, "BALANCE_NOT_AVAILABLE", "supplier did not return balance")
	}
	return s.currentFromSnapshot(result.Snapshot), nil
}

func (s *Service) cacheCurrentBalance(ctx context.Context, snapshot *adminplusdomain.BalanceSnapshot) {
	if s == nil || s.cache == nil || snapshot == nil {
		return
	}
	current := s.currentFromSnapshot(snapshot)
	ttl := current.ExpiresAt.Sub(s.now().UTC())
	if ttl <= 0 {
		return
	}
	if err := s.cache.SetCurrent(ctx, current, ttl); err != nil {
		slog.Warn("admin plus current balance cache write failed", "supplier_id", snapshot.SupplierID, "err", err)
	}
}

func (s *Service) currentFromSnapshot(snapshot *adminplusdomain.BalanceSnapshot) *CurrentBalance {
	if snapshot == nil {
		return nil
	}
	capturedAt := snapshot.CapturedAt.UTC()
	balanceCents, currency := normalizeBalanceAmountAndCurrency(snapshot.BalanceCents, snapshot.Currency)
	current := &CurrentBalance{
		SupplierID:     snapshot.SupplierID,
		RuntimeStatus:  snapshot.RuntimeStatus,
		BalanceCents:   balanceCents,
		Currency:       currency,
		SwitchEligible: snapshot.SwitchEligible,
		Source:         normalizeSource(snapshot.Source),
		CapturedAt:     capturedAt,
		RefreshAfter:   capturedAt.Add(s.currentBalanceFreshFor()),
		ExpiresAt:      capturedAt.Add(s.currentBalanceTTL()),
	}
	return s.withFreshness(current)
}

func (s *Service) withFreshness(current *CurrentBalance) *CurrentBalance {
	if current == nil {
		return nil
	}
	out := *current
	out.BalanceCents, out.Currency = normalizeBalanceAmountAndCurrency(out.BalanceCents, out.Currency)
	now := s.now().UTC()
	if out.RefreshAfter.IsZero() {
		out.RefreshAfter = out.CapturedAt.UTC().Add(s.currentBalanceFreshFor())
	}
	if out.ExpiresAt.IsZero() {
		out.ExpiresAt = out.CapturedAt.UTC().Add(s.currentBalanceTTL())
	}
	out.Stale = !now.Before(out.RefreshAfter.UTC())
	out.Expired = !now.Before(out.ExpiresAt.UTC())
	return &out
}

func (s *Service) currentBalanceFreshFor() time.Duration {
	if s != nil && s.freshFor > 0 {
		return s.freshFor
	}
	return defaultCurrentBalanceFreshFor
}

func (s *Service) currentBalanceTTL() time.Duration {
	if s != nil && s.cacheTTL > 0 {
		return s.cacheTTL
	}
	return defaultCurrentBalanceCacheTTL
}

func fallbackCurrentBalance(supplierID int64, now time.Time, err error) *CurrentBalance {
	return &CurrentBalance{
		SupplierID:          supplierID,
		RuntimeStatus:       adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		BalanceCents:        0,
		Currency:            "USD",
		SwitchEligible:      false,
		Source:              "fallback",
		CapturedAt:          now,
		RefreshAfter:        now,
		ExpiresAt:           now,
		Stale:               true,
		Expired:             true,
		Fallback:            true,
		RefreshErrorReason:  refreshErrorReason(err),
		RefreshErrorMessage: refreshErrorMessage(err),
	}
}

func refreshErrorReason(err error) string {
	if err == nil {
		return ""
	}
	reason := infraerrors.Reason(err)
	if reason == "" {
		return "BALANCE_REFRESH_FAILED"
	}
	return reason
}

func refreshErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	message := infraerrors.Message(err)
	if message == "" {
		return "balance refresh failed"
	}
	diagnostic := refreshErrorDiagnostic(err)
	if diagnostic == "" {
		return message
	}
	return message + " (" + diagnostic + ")"
}

func refreshErrorDiagnostic(err error) string {
	var appErr *infraerrors.ApplicationError
	if !errors.As(err, &appErr) || len(appErr.Metadata) == 0 {
		return ""
	}
	keys := []string{"endpoint", "status_code", "content_type", "body_type", "body_excerpt"}
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		value := strings.TrimSpace(appErr.Metadata[key])
		if value == "" {
			continue
		}
		parts = append(parts, key+"="+trimBalanceDiagnostic(value))
	}
	return strings.Join(parts, "; ")
}

func trimBalanceDiagnostic(value string) string {
	text := strings.Join(strings.Fields(value), " ")
	if len(text) <= 180 {
		return text
	}
	return text[:177] + "..."
}

func buildBalanceEvent(current, previous *adminplusdomain.BalanceSnapshot, thresholdCents int64) *adminplusdomain.BalanceEvent {
	if current == nil {
		return nil
	}
	event := &adminplusdomain.BalanceEvent{
		SupplierID:               current.SupplierID,
		SnapshotID:               current.ID,
		RuntimeStatus:            current.RuntimeStatus,
		NewBalanceCents:          current.BalanceCents,
		LowBalanceThresholdCents: thresholdCents,
		Currency:                 current.Currency,
		SwitchEligible:           current.SwitchEligible,
		Status:                   adminplusdomain.BalanceEventStatusOpen,
	}
	if previous != nil {
		oldBalance := previous.BalanceCents
		event.OldBalanceCents = &oldBalance
	}

	if current.BalanceCents == 0 {
		if previous == nil || previous.BalanceCents > 0 {
			event.Type = adminplusdomain.BalanceEventTypeDepleted
			return event
		}
		return nil
	}
	if previous != nil && previous.BalanceCents == 0 {
		event.Type = adminplusdomain.BalanceEventTypeRecovered
		return event
	}
	if thresholdCents > 0 && current.BalanceCents <= thresholdCents {
		if previous == nil || previous.BalanceCents > thresholdCents {
			event.Type = adminplusdomain.BalanceEventTypeLowBalance
			return event
		}
	}
	if previous != nil && thresholdCents > 0 && previous.BalanceCents <= thresholdCents && current.BalanceCents > thresholdCents {
		event.Type = adminplusdomain.BalanceEventTypeRecovered
		return event
	}
	return nil
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

func normalizeCurrency(value string) string {
	v := strings.ToUpper(strings.TrimSpace(value))
	if len(v) != 3 || v == "QTA" || v == "CNY" {
		return "USD"
	}
	return v
}

func normalizeBalanceAmountAndCurrency(cents int64, currency string) (int64, string) {
	v := strings.ToUpper(strings.TrimSpace(currency))
	if v == "QTA" {
		return int64(math.Round(float64(cents) / legacyNewAPIQuotaUnitsPerUSD)), "USD"
	}
	return cents, normalizeCurrency(v)
}

func normalizeBalanceSnapshotForRead(in *adminplusdomain.BalanceSnapshot) *adminplusdomain.BalanceSnapshot {
	if in == nil {
		return nil
	}
	out := *in
	out.BalanceCents, out.Currency = normalizeBalanceAmountAndCurrency(in.BalanceCents, in.Currency)
	if in.RawPayload != nil {
		out.RawPayload = make(map[string]any, len(in.RawPayload))
		for key, value := range in.RawPayload {
			out.RawPayload[key] = value
		}
	}
	return &out
}

func normalizeBalanceEventForRead(in *adminplusdomain.BalanceEvent) *adminplusdomain.BalanceEvent {
	if in == nil {
		return nil
	}
	out := *in
	out.NewBalanceCents, out.Currency = normalizeBalanceAmountAndCurrency(in.NewBalanceCents, in.Currency)
	if in.OldBalanceCents != nil {
		oldBalance, _ := normalizeBalanceAmountAndCurrency(*in.OldBalanceCents, in.Currency)
		out.OldBalanceCents = &oldBalance
	}
	if in.AcknowledgedAt != nil {
		t := *in.AcknowledgedAt
		out.AcknowledgedAt = &t
	}
	return &out
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

func runtimeStatusForBalance(balanceCents int64) adminplusdomain.SupplierRuntimeStatus {
	if balanceCents > 0 {
		return adminplusdomain.SupplierRuntimeStatusCandidate
	}
	return adminplusdomain.SupplierRuntimeStatusMonitorOnly
}
