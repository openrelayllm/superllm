package channelchecks

import (
	"context"
	"regexp"
	"sort"
	"strings"
	"time"

	healthapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/health"
	sessionsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/sessions"
	supplierkeysapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/supplierkeys"
	suppliersapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/suppliers"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/adminplus/ports"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	DefaultProbeModel       = "gpt-5.4-mini"
	defaultCandidateLimit   = 3
	defaultFirstTokenSlowMS = int64(3000)
	defaultTotalSlowMS      = int64(30000)
)

var openAIGroupKeywordPattern = regexp.MustCompile(`\b(pro|plus)\b`)

type CheckInput struct {
	SupplierID              int64
	SupplierGroupID         int64
	CandidateLimit          int
	AutoPauseOnFailure      bool
	ProbeModel              string
	FirstTokenThresholdMS   int64
	TotalLatencyThresholdMS int64
}

type CheckResult struct {
	SupplierID int64                                           `json:"supplier_id"`
	CheckedAt  time.Time                                       `json:"checked_at"`
	Total      int                                             `json:"total"`
	Best       *adminplusdomain.SupplierChannelCheckSnapshot   `json:"best,omitempty"`
	Items      []*adminplusdomain.SupplierChannelCheckSnapshot `json:"items"`
}

type Candidate struct {
	SupplierID              int64
	SupplierName            string
	SupplierType            adminplusdomain.SupplierType
	SupplierRuntimeStatus   adminplusdomain.SupplierRuntimeStatus
	SupplierHealthStatus    adminplusdomain.SupplierHealthStatus
	SupplierGroupID         int64
	ExternalGroupID         string
	GroupName               string
	ProviderFamily          string
	EffectiveRateMultiplier float64
	SupplierKeyID           int64
	SupplierAccountID       int64
	LocalSub2APIAccountID   int64
	LocalAccountName        string
	LocalAccountPlatform    string
	LocalAccountType        string
	LocalAccountStatus      string
	LocalAccountSchedulable bool
	LocalAccountGroupIDs    []int64
}

type Repository interface {
	ListCandidates(ctx context.Context, supplierID int64) ([]*Candidate, error)
	CreateSnapshot(ctx context.Context, snapshot *adminplusdomain.SupplierChannelCheckSnapshot) (*adminplusdomain.SupplierChannelCheckSnapshot, error)
	ListLatestSnapshots(ctx context.Context, supplierID int64, limit int) ([]*adminplusdomain.SupplierChannelCheckSnapshot, error)
	ListLatestSnapshotsBySupplierIDs(ctx context.Context, supplierIDs []int64) ([]*adminplusdomain.SupplierChannelCheckSnapshot, error)
	SetLocalAccountSchedulable(ctx context.Context, localAccountID int64, schedulable bool) error
}

type LocalBindingEnsurer interface {
	EnsureGroup(ctx context.Context, in supplierkeysapp.EnsureGroupInput) (*supplierkeysapp.EnsureAllResultItem, error)
}

type Service struct {
	repo            Repository
	supplierService *suppliersapp.Service
	sessionService  *sessionsapp.Service
	healthService   *healthapp.Service
	bindingEnsurer  LocalBindingEnsurer
	now             func() time.Time
}

func NewService(repo Repository, supplierService *suppliersapp.Service, sessionService *sessionsapp.Service, healthService *healthapp.Service) *Service {
	return &Service{
		repo:            repo,
		supplierService: supplierService,
		sessionService:  sessionService,
		healthService:   healthService,
		now:             time.Now,
	}
}

func NewServiceWithBindingEnsurer(repo Repository, supplierService *suppliersapp.Service, sessionService *sessionsapp.Service, healthService *healthapp.Service, bindingEnsurer LocalBindingEnsurer) *Service {
	service := NewService(repo, supplierService, sessionService, healthService)
	service.bindingEnsurer = bindingEnsurer
	return service
}

func (s *Service) Check(ctx context.Context, in CheckInput) (*CheckResult, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier channel check service is not configured")
	}
	if in.SupplierID <= 0 {
		return nil, badRequest("SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	firstThreshold := in.FirstTokenThresholdMS
	if firstThreshold <= 0 {
		firstThreshold = defaultFirstTokenSlowMS
	}
	totalThreshold := in.TotalLatencyThresholdMS
	if totalThreshold <= 0 {
		totalThreshold = defaultTotalSlowMS
	}
	candidateLimit := in.CandidateLimit
	if candidateLimit <= 0 {
		candidateLimit = defaultCandidateLimit
	}
	if in.SupplierGroupID > 0 {
		candidateLimit = 1
	}
	if candidateLimit > 10 {
		candidateLimit = 10
	}
	probeModel := strings.TrimSpace(in.ProbeModel)
	if probeModel == "" {
		probeModel = DefaultProbeModel
	}

	candidates, err := s.repo.ListCandidates(ctx, in.SupplierID)
	if err != nil {
		return nil, err
	}
	candidates = filterCandidates(candidates, in.SupplierGroupID)
	if len(candidates) == 0 {
		return nil, conflict("SUPPLIER_CHANNEL_CANDIDATE_NOT_FOUND", "supplier channel candidate not found")
	}

	monitors, _ := s.readRemoteMonitors(ctx, in.SupplierID)
	checkedAt := s.now().UTC()
	items := make([]*adminplusdomain.SupplierChannelCheckSnapshot, 0, minInt(candidateLimit, len(candidates)))
	for _, candidate := range candidates {
		if len(items) >= candidateLimit {
			break
		}
		monitor, hasMonitor := matchMonitor(candidate, monitors)
		snapshot := baseSnapshot(candidate, monitor, checkedAt)
		if hasMonitor && !remoteStatusOK(snapshot.RemoteStatus) {
			snapshot.ProbeStatus = adminplusdomain.SupplierChannelProbeStatusRemoteUnavailable
			snapshot.ErrorClass = "remote_status"
			snapshot.ErrorMessage = "remote channel monitor is not operational"
			created, err := s.saveAndMaybePause(ctx, snapshot, in.AutoPauseOnFailure)
			if err != nil {
				return nil, err
			}
			items = append(items, created)
			continue
		}
		if candidate.SupplierAccountID <= 0 || candidate.LocalSub2APIAccountID <= 0 {
			snapshot.ProbeStatus = adminplusdomain.SupplierChannelProbeStatusNoLocalAccount
			snapshot.ErrorClass = "local_account_missing"
			snapshot.ErrorMessage = "local Sub2API account binding is missing"
			created, err := s.saveAndMaybePause(ctx, snapshot, false)
			if err != nil {
				return nil, err
			}
			items = append(items, created)
			continue
		}
		s.applyProbe(ctx, snapshot, candidate, probeModel, firstThreshold, totalThreshold)
		created, err := s.saveAndMaybePause(ctx, snapshot, in.AutoPauseOnFailure)
		if err != nil {
			return nil, err
		}
		items = append(items, created)
	}
	return &CheckResult{
		SupplierID: in.SupplierID,
		CheckedAt:  checkedAt,
		Total:      len(items),
		Best:       chooseBest(items),
		Items:      items,
	}, nil
}

func (s *Service) ListLatest(ctx context.Context, supplierID int64, limit int) ([]*adminplusdomain.SupplierChannelCheckSnapshot, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier channel check service is not configured")
	}
	if supplierID <= 0 {
		return nil, badRequest("SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	return s.repo.ListLatestSnapshots(ctx, supplierID, limit)
}

func (s *Service) ListBest(ctx context.Context, supplierIDs []int64) ([]*adminplusdomain.SupplierChannelCheckSnapshot, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier channel check service is not configured")
	}
	normalized := normalizeSupplierIDs(supplierIDs)
	if len(normalized) == 0 {
		if s.supplierService == nil {
			return []*adminplusdomain.SupplierChannelCheckSnapshot{}, nil
		}
		suppliers, err := s.supplierService.List(ctx, suppliersapp.SupplierFilter{})
		if err != nil {
			return nil, err
		}
		for _, supplier := range suppliers {
			if supplier != nil {
				normalized = append(normalized, supplier.ID)
			}
		}
	}
	snapshots, err := s.repo.ListLatestSnapshotsBySupplierIDs(ctx, normalized)
	if err != nil {
		return nil, err
	}
	bySupplier := make(map[int64][]*adminplusdomain.SupplierChannelCheckSnapshot)
	latestByGroup := make(map[supplierGroupKey]*adminplusdomain.SupplierChannelCheckSnapshot)
	for _, snapshot := range snapshots {
		if snapshot == nil {
			continue
		}
		bySupplier[snapshot.SupplierID] = append(bySupplier[snapshot.SupplierID], snapshot)
		key := supplierGroupKey{supplierID: snapshot.SupplierID, groupID: snapshot.SupplierGroupID}
		if current := latestByGroup[key]; current == nil || snapshotNewer(snapshot, current) {
			latestByGroup[key] = snapshot
		}
	}
	out := make([]*adminplusdomain.SupplierChannelCheckSnapshot, 0, len(normalized))
	now := s.now().UTC()
	for _, supplierID := range normalized {
		candidates, err := s.repo.ListCandidates(ctx, supplierID)
		if err != nil {
			return nil, err
		}
		candidates = filterCandidates(candidates, 0)
		if len(candidates) > 0 {
			for _, protocolItems := range groupSnapshotsByProtocol(projectCandidateSnapshots(candidates, latestByGroup, now)) {
				if selected := chooseBestOrLowestCurrent(protocolItems); selected != nil {
					out = append(out, selected)
				}
			}
			continue
		}
		for _, protocolItems := range groupSnapshotsByProtocol(bySupplier[supplierID]) {
			if selected := chooseBestOrLatest(protocolItems); selected != nil {
				out = append(out, selected)
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].SupplierID != out[j].SupplierID {
			return out[i].SupplierID < out[j].SupplierID
		}
		leftProtocol := snapshotProtocolKey(out[i])
		rightProtocol := snapshotProtocolKey(out[j])
		if leftProtocol != rightProtocol {
			return channelProtocolPriority(leftProtocol) < channelProtocolPriority(rightProtocol)
		}
		if out[i].EffectiveRateMultiplier != out[j].EffectiveRateMultiplier {
			return out[i].EffectiveRateMultiplier < out[j].EffectiveRateMultiplier
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

type supplierGroupKey struct {
	supplierID int64
	groupID    int64
}

func (s *Service) SetScheduling(ctx context.Context, supplierID int64, supplierGroupID int64, schedulable bool) (*adminplusdomain.SupplierChannelCheckSnapshot, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier channel check service is not configured")
	}
	if supplierID <= 0 || supplierGroupID <= 0 {
		return nil, badRequest("SUPPLIER_CHANNEL_GROUP_INVALID", "invalid supplier or group id")
	}
	candidates, err := s.repo.ListCandidates(ctx, supplierID)
	if err != nil {
		return nil, err
	}
	candidates = filterCandidates(candidates, supplierGroupID)
	if len(candidates) == 0 {
		return nil, conflict("SUPPLIER_CHANNEL_CANDIDATE_NOT_FOUND", "supplier channel candidate not found")
	}
	candidate := candidates[0]
	if schedulable && (candidate.LocalSub2APIAccountID <= 0 || len(candidate.LocalAccountGroupIDs) == 0) {
		refreshed, err := s.ensureSchedulingPrerequisites(ctx, candidate)
		if err != nil {
			return nil, err
		}
		candidate = refreshed
	}
	if candidate.LocalSub2APIAccountID <= 0 {
		return nil, conflict("SUPPLIER_CHANNEL_LOCAL_ACCOUNT_MISSING", "local Sub2API account binding is missing")
	}
	if schedulable {
		if len(candidate.LocalAccountGroupIDs) == 0 {
			return nil, conflict("SUPPLIER_CHANNEL_LOCAL_GROUP_BINDING_MISSING", "local Sub2API account is not bound to any group")
		}
	}
	if err := s.repo.SetLocalAccountSchedulable(ctx, candidate.LocalSub2APIAccountID, schedulable); err != nil {
		return nil, err
	}
	now := s.now().UTC()
	snapshot := baseSnapshot(candidate, nil, now)
	snapshot.LocalAccountSchedulable = schedulable
	snapshot.ProbeStatus = adminplusdomain.SupplierChannelProbeStatusUntested
	if schedulable {
		snapshot.ErrorMessage = "local account scheduling enabled"
	} else {
		snapshot.ErrorMessage = "local account scheduling paused"
	}
	return s.repo.CreateSnapshot(ctx, snapshot)
}

func (s *Service) ensureSchedulingPrerequisites(ctx context.Context, candidate *Candidate) (*Candidate, error) {
	if candidate == nil {
		return nil, conflict("SUPPLIER_CHANNEL_CANDIDATE_NOT_FOUND", "supplier channel candidate not found")
	}
	if s.bindingEnsurer == nil {
		return candidate, nil
	}
	_, err := s.bindingEnsurer.EnsureGroup(ctx, supplierkeysapp.EnsureGroupInput{
		EnsureAllInput: supplierkeysapp.EnsureAllInput{
			SupplierID: candidate.SupplierID,
		},
		SupplierGroupID: candidate.SupplierGroupID,
	})
	if err != nil {
		return nil, err
	}
	candidates, err := s.repo.ListCandidates(ctx, candidate.SupplierID)
	if err != nil {
		return nil, err
	}
	candidates = filterCandidates(candidates, candidate.SupplierGroupID)
	if len(candidates) == 0 {
		return nil, conflict("SUPPLIER_CHANNEL_CANDIDATE_NOT_FOUND", "supplier channel candidate not found")
	}
	return candidates[0], nil
}

func (s *Service) readRemoteMonitors(ctx context.Context, supplierID int64) ([]ports.ChannelMonitorView, error) {
	if s == nil || s.sessionService == nil {
		return nil, nil
	}
	result, err := s.sessionService.ReadChannelMonitors(ctx, supplierID)
	if err != nil || result == nil {
		return nil, err
	}
	return result.Items, nil
}

func (s *Service) applyProbe(ctx context.Context, snapshot *adminplusdomain.SupplierChannelCheckSnapshot, candidate *Candidate, probeModel string, firstThreshold int64, totalThreshold int64) {
	if s == nil || s.healthService == nil || snapshot == nil || candidate == nil {
		return
	}
	model := strings.TrimSpace(probeModel)
	if model == "" {
		model = DefaultProbeModel
	}
	result, err := s.healthService.ProbeOpenAIResponses(ctx, healthapp.ProbeInput{
		SupplierID:                   candidate.SupplierID,
		SupplierAccountID:            candidate.SupplierAccountID,
		Model:                        model,
		FirstTokenThresholdMS:        firstThreshold,
		TotalLatencyThresholdMS:      totalThreshold,
		ConcurrencySaturationPercent: 100,
	})
	snapshot.ProbeModel = model
	if err != nil {
		snapshot.ProbeStatus = adminplusdomain.SupplierChannelProbeStatusProbeFailed
		snapshot.ErrorClass = firstNonEmpty(infraerrors.Reason(err), "probe_failed")
		snapshot.ErrorMessage = trimLimit(firstNonEmpty(infraerrors.Message(err), err.Error()), 500)
		return
	}
	if result == nil || result.Sample == nil {
		snapshot.ProbeStatus = adminplusdomain.SupplierChannelProbeStatusProbeFailed
		snapshot.ErrorClass = "probe_empty"
		snapshot.ErrorMessage = "health probe returned empty result"
		return
	}
	sample := result.Sample
	snapshot.FirstTokenMS = sample.FirstTokenLatencyMS
	snapshot.DurationMS = sample.TotalLatencyMS
	snapshot.StatusCode = sample.StatusCode
	snapshot.ErrorClass = sample.ErrorClass
	if sample.ErrorClass != "" || sample.StatusCode >= 400 || sample.StatusCode == 0 {
		snapshot.ProbeStatus = adminplusdomain.SupplierChannelProbeStatusRequestError
		snapshot.ErrorMessage = trimLimit(stringFromRaw(sample.RawPayload, "error_message"), 500)
		return
	}
	if sample.FirstTokenLatencyMS > firstThreshold {
		snapshot.ProbeStatus = adminplusdomain.SupplierChannelProbeStatusSlowFirstToken
		snapshot.ErrorMessage = "first token latency exceeded threshold"
		return
	}
	if sample.TotalLatencyMS > totalThreshold {
		snapshot.ProbeStatus = adminplusdomain.SupplierChannelProbeStatusSlowTotal
		snapshot.ErrorMessage = "total latency exceeded threshold"
		return
	}
	snapshot.ProbeStatus = adminplusdomain.SupplierChannelProbeStatusAvailable
	snapshot.Recommended = true
}

func (s *Service) saveAndMaybePause(ctx context.Context, snapshot *adminplusdomain.SupplierChannelCheckSnapshot, autoPause bool) (*adminplusdomain.SupplierChannelCheckSnapshot, error) {
	if autoPause && snapshot != nil && !snapshot.Recommended && snapshot.LocalSub2APIAccountID > 0 && snapshot.LocalAccountSchedulable {
		if err := s.repo.SetLocalAccountSchedulable(ctx, snapshot.LocalSub2APIAccountID, false); err != nil {
			return nil, err
		}
		snapshot.LocalAccountSchedulable = false
	}
	return s.repo.CreateSnapshot(ctx, snapshot)
}

func filterCandidates(candidates []*Candidate, supplierGroupID int64) []*Candidate {
	out := make([]*Candidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate == nil {
			continue
		}
		if supplierGroupID > 0 && candidate.SupplierGroupID != supplierGroupID {
			continue
		}
		if candidate.SupplierRuntimeStatus == adminplusdomain.SupplierRuntimeStatusDisabled {
			continue
		}
		if candidate.SupplierHealthStatus != "" && candidate.SupplierHealthStatus != adminplusdomain.SupplierHealthStatusNormal {
			continue
		}
		if candidate.EffectiveRateMultiplier <= 0 {
			continue
		}
		out = append(out, candidate)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].EffectiveRateMultiplier != out[j].EffectiveRateMultiplier {
			return out[i].EffectiveRateMultiplier < out[j].EffectiveRateMultiplier
		}
		return out[i].SupplierGroupID < out[j].SupplierGroupID
	})
	return out
}

func baseSnapshot(candidate *Candidate, monitor *ports.ChannelMonitorView, capturedAt time.Time) *adminplusdomain.SupplierChannelCheckSnapshot {
	snapshot := &adminplusdomain.SupplierChannelCheckSnapshot{
		SupplierID:              candidate.SupplierID,
		SupplierGroupID:         candidate.SupplierGroupID,
		SupplierKeyID:           candidate.SupplierKeyID,
		SupplierAccountID:       candidate.SupplierAccountID,
		LocalSub2APIAccountID:   candidate.LocalSub2APIAccountID,
		ExternalGroupID:         candidate.ExternalGroupID,
		GroupName:               candidate.GroupName,
		ProviderFamily:          candidate.ProviderFamily,
		RemoteStatus:            "unknown",
		ProbeModel:              DefaultProbeModel,
		ProbeStatus:             adminplusdomain.SupplierChannelProbeStatusUntested,
		EffectiveRateMultiplier: candidate.EffectiveRateMultiplier,
		LocalAccountSchedulable: candidate.LocalAccountSchedulable,
		CapturedAt:              capturedAt,
	}
	if monitor != nil {
		snapshot.ChannelMonitorID = monitor.ID
		snapshot.ChannelName = monitor.Name
		snapshot.ChannelProvider = monitor.Provider
		snapshot.PrimaryModel = monitor.PrimaryModel
		snapshot.RemoteStatus = normalizeRemoteStatus(monitor.PrimaryStatus)
	}
	return snapshot
}

func matchMonitor(candidate *Candidate, monitors []ports.ChannelMonitorView) (*ports.ChannelMonitorView, bool) {
	if candidate == nil || len(monitors) == 0 {
		return nil, false
	}
	group := normalizeKey(candidate.GroupName)
	provider := normalizeKey(candidate.ProviderFamily)
	for i := range monitors {
		monitor := &monitors[i]
		if group != "" && (normalizeKey(monitor.GroupName) == group || normalizeKey(monitor.Name) == group) {
			return monitor, true
		}
	}
	for i := range monitors {
		monitor := &monitors[i]
		if provider != "" && normalizeKey(monitor.Provider) == provider {
			return monitor, true
		}
	}
	return nil, false
}

func normalizeRemoteStatus(status string) string {
	value := strings.ToLower(strings.TrimSpace(status))
	switch value {
	case "ok", "normal", "healthy", "success":
		return "operational"
	case "down", "timeout":
		return "failed"
	case "":
		return "unknown"
	default:
		return trimLimit(value, 80)
	}
}

func remoteStatusOK(status string) bool {
	switch normalizeRemoteStatus(status) {
	case "operational", "unknown":
		return true
	default:
		return false
	}
}

func chooseBest(items []*adminplusdomain.SupplierChannelCheckSnapshot) *adminplusdomain.SupplierChannelCheckSnapshot {
	candidates := make([]*adminplusdomain.SupplierChannelCheckSnapshot, 0, len(items))
	for _, item := range items {
		if item != nil && item.Recommended {
			candidates = append(candidates, item)
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	sortSnapshotsForBest(candidates)
	return candidates[0]
}

func chooseBestOrLatest(items []*adminplusdomain.SupplierChannelCheckSnapshot) *adminplusdomain.SupplierChannelCheckSnapshot {
	if best := chooseBest(items); best != nil {
		return best
	}
	if len(items) == 0 {
		return nil
	}
	sort.SliceStable(items, func(i, j int) bool {
		if !items[i].CapturedAt.Equal(items[j].CapturedAt) {
			return items[i].CapturedAt.After(items[j].CapturedAt)
		}
		return items[i].ID > items[j].ID
	})
	return items[0]
}

func chooseBestOrLowestCurrent(items []*adminplusdomain.SupplierChannelCheckSnapshot) *adminplusdomain.SupplierChannelCheckSnapshot {
	if best := chooseBest(items); best != nil {
		return best
	}
	return chooseLowestCurrent(items)
}

func chooseLowestCurrent(items []*adminplusdomain.SupplierChannelCheckSnapshot) *adminplusdomain.SupplierChannelCheckSnapshot {
	if len(items) == 0 {
		return nil
	}
	sortSnapshotsForBest(items)
	return items[0]
}

func projectCandidateSnapshots(candidates []*Candidate, latestByGroup map[supplierGroupKey]*adminplusdomain.SupplierChannelCheckSnapshot, capturedAt time.Time) []*adminplusdomain.SupplierChannelCheckSnapshot {
	out := make([]*adminplusdomain.SupplierChannelCheckSnapshot, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate == nil {
			continue
		}
		key := supplierGroupKey{supplierID: candidate.SupplierID, groupID: candidate.SupplierGroupID}
		out = append(out, projectCandidateSnapshot(candidate, latestByGroup[key], capturedAt))
	}
	return out
}

func projectCandidateSnapshot(candidate *Candidate, latest *adminplusdomain.SupplierChannelCheckSnapshot, capturedAt time.Time) *adminplusdomain.SupplierChannelCheckSnapshot {
	var snapshot adminplusdomain.SupplierChannelCheckSnapshot
	if latest != nil {
		snapshot = *latest
	} else {
		snapshot = *baseSnapshot(candidate, nil, capturedAt)
		snapshot.CreatedAt = capturedAt
	}
	snapshot.SupplierID = candidate.SupplierID
	snapshot.SupplierGroupID = candidate.SupplierGroupID
	snapshot.SupplierKeyID = candidate.SupplierKeyID
	snapshot.SupplierAccountID = candidate.SupplierAccountID
	snapshot.LocalSub2APIAccountID = candidate.LocalSub2APIAccountID
	snapshot.ExternalGroupID = candidate.ExternalGroupID
	snapshot.GroupName = candidate.GroupName
	snapshot.ProviderFamily = candidate.ProviderFamily
	snapshot.EffectiveRateMultiplier = candidate.EffectiveRateMultiplier
	snapshot.LocalAccountSchedulable = candidate.LocalAccountSchedulable
	if latest == nil {
		snapshot.ProbeStatus = adminplusdomain.SupplierChannelProbeStatusUntested
		if candidate.SupplierAccountID <= 0 || candidate.LocalSub2APIAccountID <= 0 {
			snapshot.ProbeStatus = adminplusdomain.SupplierChannelProbeStatusNoLocalAccount
			snapshot.ErrorClass = "local_account_missing"
			snapshot.ErrorMessage = "local Sub2API account binding is missing"
		}
		return &snapshot
	}
	if latest.ProbeStatus == adminplusdomain.SupplierChannelProbeStatusNoLocalAccount && candidate.SupplierAccountID > 0 && candidate.LocalSub2APIAccountID > 0 {
		snapshot.ProbeStatus = adminplusdomain.SupplierChannelProbeStatusUntested
		snapshot.Recommended = false
		snapshot.ErrorClass = ""
		snapshot.ErrorMessage = ""
	}
	return &snapshot
}

func groupSnapshotsByProtocol(items []*adminplusdomain.SupplierChannelCheckSnapshot) map[string][]*adminplusdomain.SupplierChannelCheckSnapshot {
	out := make(map[string][]*adminplusdomain.SupplierChannelCheckSnapshot)
	for _, item := range items {
		if item == nil {
			continue
		}
		key := snapshotProtocolKey(item)
		out[key] = append(out[key], item)
	}
	return out
}

func snapshotProtocolKey(item *adminplusdomain.SupplierChannelCheckSnapshot) string {
	if item == nil {
		return "other"
	}
	switch strings.ToLower(strings.TrimSpace(item.ProviderFamily)) {
	case "openai":
		return "openai"
	case "anthropic":
		return "claude"
	case "gemini":
		return "gemini"
	}
	haystack := strings.ToLower(strings.Join([]string{
		item.ProviderFamily,
		item.GroupName,
		item.ChannelName,
		item.ChannelProvider,
		item.PrimaryModel,
		item.ProbeModel,
	}, " "))
	switch {
	case strings.Contains(haystack, "anthropic") || strings.Contains(haystack, "claude"):
		return "claude"
	case strings.Contains(haystack, "gemini") || strings.Contains(haystack, "google"):
		return "gemini"
	case strings.Contains(haystack, "openai") ||
		strings.Contains(haystack, "gpt") ||
		strings.Contains(haystack, "chatgpt") ||
		strings.Contains(haystack, "o3") ||
		strings.Contains(haystack, "o4") ||
		openAIGroupKeywordPattern.MatchString(haystack):
		return "openai"
	default:
		return "other"
	}
}

func channelProtocolPriority(protocol string) int {
	switch protocol {
	case "openai":
		return 0
	case "claude":
		return 1
	case "gemini":
		return 2
	default:
		return 3
	}
}

func sortSnapshotsForBest(items []*adminplusdomain.SupplierChannelCheckSnapshot) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].EffectiveRateMultiplier != items[j].EffectiveRateMultiplier {
			return items[i].EffectiveRateMultiplier < items[j].EffectiveRateMultiplier
		}
		if items[i].FirstTokenMS != items[j].FirstTokenMS {
			return items[i].FirstTokenMS < items[j].FirstTokenMS
		}
		if items[i].DurationMS != items[j].DurationMS {
			return items[i].DurationMS < items[j].DurationMS
		}
		return items[i].ID < items[j].ID
	})
}

func snapshotNewer(candidate *adminplusdomain.SupplierChannelCheckSnapshot, current *adminplusdomain.SupplierChannelCheckSnapshot) bool {
	if current == nil {
		return true
	}
	if !candidate.CapturedAt.Equal(current.CapturedAt) {
		return candidate.CapturedAt.After(current.CapturedAt)
	}
	return candidate.ID > current.ID
}

func normalizeSupplierIDs(ids []int64) []int64 {
	seen := map[int64]struct{}{}
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func normalizeKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "")
	value = strings.ReplaceAll(value, "_", "")
	value = strings.ReplaceAll(value, "-", "")
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func stringFromRaw(values map[string]any, key string) string {
	if len(values) == 0 {
		return ""
	}
	if value, ok := values[key].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
