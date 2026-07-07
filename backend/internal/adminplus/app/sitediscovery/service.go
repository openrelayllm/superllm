package sitediscovery

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/adminplus/app/bizlogs"
	extensionapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/extension"
	mailverificationapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/mailverification"
	proxyapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/proxy"
	suppliersapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/suppliers"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/adminplus/ports"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	opsservice "github.com/Wei-Shaw/sub2api/internal/service"
	"golang.org/x/net/html"
)

const (
	DefaultSourceURL             = "https://api.daheiai.com/"
	defaultLowRateThreshold      = 0.8
	defaultDiscoveryFetchLimit   = 8 << 20
	defaultSiteProbeLimit        = 512 << 10
	defaultSiteProbeWorkers      = 48
	defaultInterfaceProbeTTL     = 3 * time.Second
	defaultEndpointProbeTTL      = 1400 * time.Millisecond
	defaultPageProbeTTL          = 2500 * time.Millisecond
	defaultPasswordLength        = 20
	defaultDirectRegistrationTTL = 140 * time.Second
)

type CredentialCipher interface {
	Encrypt(plaintext string) (string, error)
	Decrypt(ciphertext string) (string, error)
}

type RegistrationMailReader interface {
	ReadVerificationCodeForEmail(ctx context.Context, in mailverificationapp.ReadVerificationCodeForEmailInput) (*mailverificationapp.ReadVerificationCodeResult, error)
}

type RegistrationLogReader interface {
	ListSystemLogs(ctx context.Context, filter *opsservice.OpsSystemLogFilter) (*opsservice.OpsSystemLogList, error)
}

type RegistrationRecord struct {
	Credential *adminplusdomain.SupplierRegistrationCredential
	Item       *adminplusdomain.SiteDiscoveryItem
}

type RunInput struct {
	SourceURL       string
	ProbeInterfaces bool
	ProbeSites      bool
	Limit           int
	ProxyPolicyID   int64
}

type ClassifyInput struct {
	Query                string
	ProviderType         adminplusdomain.SupplierType
	ClassificationStatus adminplusdomain.SiteDiscoveryClassificationStatus
	ImportStatus         adminplusdomain.SiteDiscoveryImportStatus
	RegistrationStatus   adminplusdomain.SupplierRegistrationStatus
	ProcessedStatus      string
	ProbeInterfaces      bool
	ProbeSites           bool
	Limit                int
	ProxyPolicyID        int64
}

type RunResult struct {
	Run   *adminplusdomain.SiteDiscoveryRun    `json:"run"`
	Items []*adminplusdomain.SiteDiscoveryItem `json:"items"`
}

type ClassifyResult struct {
	Total          int                                  `json:"total"`
	SupportedTotal int                                  `json:"supported_total"`
	UnknownTotal   int                                  `json:"unknown_total"`
	Items          []*adminplusdomain.SiteDiscoveryItem `json:"items"`
}

type RunProgressEvent struct {
	Type           string                             `json:"type"`
	Level          string                             `json:"level,omitempty"`
	Message        string                             `json:"message"`
	Current        int                                `json:"current,omitempty"`
	Total          int                                `json:"total,omitempty"`
	Run            *adminplusdomain.SiteDiscoveryRun  `json:"run,omitempty"`
	Item           *adminplusdomain.SiteDiscoveryItem `json:"item,omitempty"`
	Result         *RunResult                         `json:"result,omitempty"`
	ClassifyResult *ClassifyResult                    `json:"classify_result,omitempty"`
}

type RunProgressEmitter func(RunProgressEvent)

type ListFilter struct {
	Query                string
	ProviderType         adminplusdomain.SupplierType
	ClassificationStatus adminplusdomain.SiteDiscoveryClassificationStatus
	ImportStatus         adminplusdomain.SiteDiscoveryImportStatus
	RegistrationStatus   adminplusdomain.SupplierRegistrationStatus
	ProcessedStatus      string
	Limit                int
}

type RegisterCredentialView struct {
	DiscoveryID  int64                        `json:"discovery_id"`
	SupplierID   int64                        `json:"supplier_id"`
	ProviderType adminplusdomain.SupplierType `json:"provider_type"`
	RegisterURL  string                       `json:"register_url"`
	Email        string                       `json:"email"`
	Password     string                       `json:"password"`
}

type RegistrationTaskView struct {
	ID             int64                                      `json:"id"`
	DiscoveryID    int64                                      `json:"discovery_id"`
	RegistrationID int64                                      `json:"registration_id,omitempty"`
	TaskID         int64                                      `json:"task_id,omitempty"`
	Status         adminplusdomain.SupplierRegistrationStatus `json:"status"`
	TaskStatus     adminplusdomain.ExtensionTaskStatus        `json:"task_status,omitempty"`
	Email          string                                     `json:"email,omitempty"`
	ErrorCode      string                                     `json:"error_code,omitempty"`
	ErrorMessage   string                                     `json:"error_message,omitempty"`
	Attempts       int                                        `json:"attempts,omitempty"`
	MaxAttempts    int                                        `json:"max_attempts,omitempty"`
	DeviceID       string                                     `json:"device_id,omitempty"`
	CanRetry       bool                                       `json:"can_retry"`
	LastAttemptAt  *time.Time                                 `json:"last_attempt_at,omitempty"`
	CreatedAt      time.Time                                  `json:"created_at"`
	UpdatedAt      time.Time                                  `json:"updated_at"`
	FinishedAt     *time.Time                                 `json:"finished_at,omitempty"`
	Discovery      *adminplusdomain.SiteDiscoveryItem         `json:"discovery"`
}

type RegistrationTaskLogsResult struct {
	Items []*opsservice.OpsSystemLog `json:"items"`
}

type ReadRegistrationVerificationCodeInput struct {
	TaskID              int64
	DeviceID            string
	LeaseToken          string
	TriggeredAt         *time.Time
	TimeoutSeconds      int
	PollIntervalSeconds int
}

type RegisterItemInput struct {
	ItemID        int64
	ProxyPolicyID int64
}

type RerunRegistrationInput struct {
	RegistrationID int64
	ProxyPolicyID  int64
}

type registrationProxyContext struct {
	Assignment *adminplusdomain.ProxyAssignment
	ProxyURL   string
}

type Repository interface {
	GetSettings(ctx context.Context) (*adminplusdomain.SiteDiscoverySettings, error)
	UpdateSettings(ctx context.Context, settings adminplusdomain.SiteDiscoverySettings) (*adminplusdomain.SiteDiscoverySettings, error)
	CreateRun(ctx context.Context, run *adminplusdomain.SiteDiscoveryRun) (*adminplusdomain.SiteDiscoveryRun, error)
	UpdateRun(ctx context.Context, run *adminplusdomain.SiteDiscoveryRun) (*adminplusdomain.SiteDiscoveryRun, error)
	FindExistingItem(ctx context.Context, sourceURL string, sourceSiteID string, registerURL string) (*adminplusdomain.SiteDiscoveryItem, error)
	UpsertItem(ctx context.Context, item *adminplusdomain.SiteDiscoveryItem) (*adminplusdomain.SiteDiscoveryItem, error)
	GetItem(ctx context.Context, id int64) (*adminplusdomain.SiteDiscoveryItem, error)
	ListItems(ctx context.Context, filter ListFilter) ([]*adminplusdomain.SiteDiscoveryItem, error)
	ListRegistrationRecords(ctx context.Context, filter ListFilter) ([]*RegistrationRecord, error)
	LinkSupplier(ctx context.Context, itemID int64, supplierID int64) (*adminplusdomain.SiteDiscoveryItem, error)
	UpsertRegistrationCredential(ctx context.Context, credential *adminplusdomain.SupplierRegistrationCredential) (*adminplusdomain.SupplierRegistrationCredential, error)
	StartRegistrationAttempt(ctx context.Context, credentialID int64, attemptedAt time.Time) (*adminplusdomain.SupplierRegistrationCredential, error)
	UpdateRegistrationTask(ctx context.Context, credentialID int64, taskID int64, status adminplusdomain.SupplierRegistrationStatus, attemptedAt time.Time) (*adminplusdomain.SupplierRegistrationCredential, error)
	GetRegistrationCredential(ctx context.Context, credentialID int64) (*adminplusdomain.SupplierRegistrationCredential, *adminplusdomain.SiteDiscoveryItem, error)
	GetRegistrationCredentialByDiscoveryID(ctx context.Context, discoveryID int64) (*adminplusdomain.SupplierRegistrationCredential, *adminplusdomain.SiteDiscoveryItem, error)
	GetRegistrationCredentialByTaskID(ctx context.Context, taskID int64) (*adminplusdomain.SupplierRegistrationCredential, *adminplusdomain.SiteDiscoveryItem, error)
	MarkRegistrationSucceeded(ctx context.Context, credentialID int64, supplierID int64, attemptedAt time.Time) (*adminplusdomain.SupplierRegistrationCredential, error)
	CompleteRegistration(ctx context.Context, credentialID int64, supplierID int64, status adminplusdomain.SupplierRegistrationStatus, errorCode string, errorMessage string, attemptedAt time.Time) (*adminplusdomain.SupplierRegistrationCredential, error)
	ListRecommendations(ctx context.Context, threshold float64, limit int) ([]*adminplusdomain.SiteDiscoveryRecommendation, error)
}

type Service struct {
	repo               Repository
	suppliers          *suppliersapp.Service
	extension          *extensionapp.Service
	mail               RegistrationMailReader
	directRegistration ports.DirectRegistrationAdapter
	cipher             CredentialCipher
	bizlog             *bizlogs.Recorder
	logs               RegistrationLogReader
	client             *http.Client
	proxyManager       ProxyManager
	now                func() time.Time
	registrationLocks  keyedMutex
}

type ProxyManager interface {
	RequestAssignment(ctx context.Context, input proxyapp.RequestAssignmentInput) (*adminplusdomain.ProxyAssignment, error)
	ReleaseAssignment(ctx context.Context, id int64, failed bool, errorCode string, errorMessage string) (*adminplusdomain.ProxyAssignment, error)
	ReportFailure(ctx context.Context, id int64, input proxyapp.ReportFailureInput) (*adminplusdomain.ProxyAssignment, error)
}

type keyedMutex struct {
	mu    sync.Mutex
	locks map[int64]*sync.Mutex
}

func (m *keyedMutex) lock(key int64) func() {
	m.mu.Lock()
	if m.locks == nil {
		m.locks = make(map[int64]*sync.Mutex)
	}
	lock := m.locks[key]
	if lock == nil {
		lock = &sync.Mutex{}
		m.locks[key] = lock
	}
	m.mu.Unlock()
	lock.Lock()
	return lock.Unlock
}

func NewService(repo Repository, suppliers *suppliersapp.Service, extension *extensionapp.Service, mail RegistrationMailReader, cipher CredentialCipher, client *http.Client) *Service {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	return &Service{
		repo:      repo,
		suppliers: suppliers,
		extension: extension,
		mail:      mail,
		cipher:    cipher,
		client:    client,
		now:       time.Now,
	}
}

func (s *Service) WithDirectRegistration(adapter ports.DirectRegistrationAdapter) *Service {
	if s != nil {
		s.directRegistration = adapter
	}
	return s
}

func (s *Service) WithDiagnostics(recorder *bizlogs.Recorder) *Service {
	if s != nil {
		s.bizlog = recorder
	}
	return s
}

func (s *Service) WithRegistrationLogs(reader RegistrationLogReader) *Service {
	if s != nil {
		s.logs = reader
	}
	return s
}

func (s *Service) WithProxyManager(manager ProxyManager) *Service {
	if s != nil {
		s.proxyManager = manager
	}
	return s
}

func (s *Service) GetSettings(ctx context.Context) (*adminplusdomain.SiteDiscoverySettings, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("site discovery service is not configured")
	}
	return normalizeSettings(s.repo.GetSettings(ctx))
}

func (s *Service) UpdateSettings(ctx context.Context, settings adminplusdomain.SiteDiscoverySettings) (*adminplusdomain.SiteDiscoverySettings, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("site discovery service is not configured")
	}
	settings.RegistrationEmail = strings.TrimSpace(settings.RegistrationEmail)
	if settings.LowRateThreshold <= 0 {
		settings.LowRateThreshold = defaultLowRateThreshold
	}
	settings.UpdatedAt = s.now().UTC()
	return s.repo.UpdateSettings(ctx, settings)
}

func (s *Service) Run(ctx context.Context, in RunInput) (*RunResult, error) {
	return s.run(ctx, in, nil)
}

func (s *Service) RunWithProgress(ctx context.Context, in RunInput, emit RunProgressEmitter) (*RunResult, error) {
	return s.run(ctx, in, emit)
}

func (s *Service) ClassifyWithProgress(ctx context.Context, in ClassifyInput, emit RunProgressEmitter) (*ClassifyResult, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("site discovery service is not configured")
	}
	if in.ProviderType != "" && in.ProviderType != adminplusdomain.SupplierTypeNewAPI && in.ProviderType != adminplusdomain.SupplierTypeSub2API {
		return nil, badRequest("SITE_DISCOVERY_PROVIDER_TYPE_UNSUPPORTED", "only new_api and sub2api discovery filters are supported")
	}
	if in.ProcessedStatus != "" && in.ProcessedStatus != "processed" && in.ProcessedStatus != "unprocessed" {
		return nil, badRequest("SITE_DISCOVERY_PROCESSED_STATUS_UNSUPPORTED", "processed status filter must be processed or unprocessed")
	}
	if in.Limit <= 0 || in.Limit > 10000 {
		in.Limit = 10000
	}
	items, err := s.repo.ListItems(ctx, ListFilter{
		Query:                strings.TrimSpace(in.Query),
		ProviderType:         in.ProviderType,
		ClassificationStatus: in.ClassificationStatus,
		ImportStatus:         in.ImportStatus,
		RegistrationStatus:   in.RegistrationStatus,
		ProcessedStatus:      strings.TrimSpace(in.ProcessedStatus),
		Limit:                in.Limit,
	})
	if err != nil {
		return nil, err
	}
	total := len(items)
	emitRunProgress(emit, RunProgressEvent{
		Type:    "started",
		Level:   "info",
		Message: "批量识别已开始，共 " + stringFromInt64(int64(total)) + " 个候选网址",
		Total:   total,
	})
	if total == 0 {
		result := &ClassifyResult{}
		emitRunProgress(emit, RunProgressEvent{
			Type:           "completed",
			Level:          "success",
			Message:        "没有需要识别的候选网址",
			Current:        0,
			Total:          0,
			ClassifyResult: result,
		})
		return result, nil
	}
	if in.ProbeInterfaces {
		emitRunProgress(emit, RunProgressEvent{Type: "log", Level: "info", Message: "正在通过 /api/status 与 /api/v1/settings/public 判断类型", Total: total})
	}
	if in.ProbeSites {
		emitRunProgress(emit, RunProgressEvent{Type: "log", Level: "info", Message: "已启用页面深度探测作为补充判断", Total: total})
	}
	classifyCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	result := &ClassifyResult{Total: total, Items: make([]*adminplusdomain.SiteDiscoveryItem, 0, total)}
	current := 0
	for item := range s.classifyCandidatesStream(classifyCtx, items, in.ProbeInterfaces, in.ProbeSites) {
		current++
		updated, err := s.repo.UpsertItem(ctx, item)
		if err != nil {
			cancel()
			return nil, err
		}
		if updated.ClassificationStatus == adminplusdomain.SiteDiscoveryClassificationSupported {
			result.SupportedTotal++
		} else {
			result.UnknownTotal++
		}
		result.Items = append(result.Items, updated)
		emitRunProgress(emit, siteDiscoveryClassifyProgressEvent(current, total, updated))
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	emitRunProgress(emit, RunProgressEvent{
		Type:           "completed",
		Level:          "success",
		Message:        "批量识别完成：" + stringFromInt64(int64(result.Total)) + " 个站点，支持 " + stringFromInt64(int64(result.SupportedTotal)) + " 个，未知 " + stringFromInt64(int64(result.UnknownTotal)) + " 个",
		Current:        total,
		Total:          total,
		ClassifyResult: result,
	})
	return result, nil
}

func (s *Service) run(ctx context.Context, in RunInput, emit RunProgressEmitter) (*RunResult, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("site discovery service is not configured")
	}
	sourceURL := strings.TrimSpace(in.SourceURL)
	if sourceURL == "" {
		sourceURL = DefaultSourceURL
	}
	if _, err := normalizeAbsoluteURL(sourceURL); err != nil {
		return nil, badRequest("SITE_DISCOVERY_SOURCE_URL_INVALID", "source url must be a valid http or https url")
	}
	now := s.now().UTC()
	run, err := s.repo.CreateRun(ctx, &adminplusdomain.SiteDiscoveryRun{
		SourceURL: sourceURL,
		Status:    adminplusdomain.SiteDiscoveryRunStatusRunning,
		StartedAt: now,
		CreatedAt: now,
	})
	if err != nil {
		return nil, err
	}
	emitRunProgress(emit, RunProgressEvent{Type: "started", Level: "info", Message: "采集任务已创建", Run: run})
	runClient := s.client
	var proxyAssignment *adminplusdomain.ProxyAssignment
	proxyReleased := false
	releaseProxy := func(failed bool, code string, message string) {
		if proxyAssignment == nil || proxyReleased || s.proxyManager == nil {
			return
		}
		proxyReleased = true
		_, _ = s.proxyManager.ReleaseAssignment(context.Background(), proxyAssignment.ID, failed, code, message)
	}
	if in.ProxyPolicyID > 0 {
		assignment, client, err := s.acquireProxyForRun(ctx, in.ProxyPolicyID, run, sourceURL)
		if err != nil {
			return nil, s.failRunWithProgress(ctx, run, err, emit)
		}
		proxyAssignment = assignment
		runClient = client
		emitRunProgress(emit, RunProgressEvent{Type: "log", Level: "info", Message: "已绑定代理出口 assignment #" + stringFromInt64(assignment.ID), Run: run})
		defer func() {
			releaseProxy(false, "", "")
		}()
	}
	emitRunProgress(emit, RunProgressEvent{Type: "log", Level: "info", Message: "正在读取采集源：" + sourceURL, Run: run})
	body, err := s.fetchTextWithClient(ctx, runClient, sourceURL, defaultDiscoveryFetchLimit)
	if err != nil {
		releaseProxy(true, "SITE_DISCOVERY_FETCH_FAILED", err.Error())
		return nil, s.failRunWithProgress(ctx, run, err, emit)
	}
	candidates, err := s.parseSourceCandidates(ctx, runClient, sourceURL, body)
	if err != nil {
		return nil, s.failRunWithProgress(ctx, run, err, emit)
	}
	if in.Limit > 0 && in.Limit < len(candidates) {
		candidates = candidates[:in.Limit]
	}
	emitRunProgress(emit, RunProgressEvent{Type: "log", Level: "info", Message: "采集源解析完成，发现 " + stringFromInt64(int64(len(candidates))) + " 个候选网址", Run: run, Total: len(candidates)})
	monitor := s.fetchMonitorDataWithClient(ctx, runClient, sourceURL)
	if in.ProbeInterfaces {
		emitRunProgress(emit, RunProgressEvent{Type: "log", Level: "info", Message: "正在通过公开接口进行二次分类并写入候选库", Run: run, Total: len(candidates)})
	} else {
		emitRunProgress(emit, RunProgressEvent{Type: "log", Level: "info", Message: "已关闭接口类型识别，正在使用索引特征写入候选库", Run: run, Total: len(candidates)})
	}
	classifyCtx, cancelClassify := context.WithCancel(ctx)
	defer cancelClassify()
	items := make([]*adminplusdomain.SiteDiscoveryItem, 0, len(candidates))
	supported := 0
	imported := 0
	for _, item := range candidates {
		item.RunID = run.ID
		item.SourceURL = sourceURL
		if m := monitor[item.SourceSiteID]; m != nil {
			applyMonitor(item, m)
		}
	}
	for item := range s.classifyCandidatesStreamWithClient(classifyCtx, candidates, in.ProbeInterfaces, in.ProbeSites, runClient) {
		if item.ClassificationStatus == adminplusdomain.SiteDiscoveryClassificationSupported {
			supported++
		}
		existing, err := s.repo.FindExistingItem(ctx, sourceURL, item.SourceSiteID, item.RegisterURL)
		if err != nil {
			cancelClassify()
			return nil, s.failRunWithProgress(ctx, run, err, emit)
		}
		created, err := s.repo.UpsertItem(ctx, item)
		if err != nil {
			cancelClassify()
			return nil, s.failRunWithProgress(ctx, run, err, emit)
		}
		if created.ImportStatus == adminplusdomain.SiteDiscoveryImportImported {
			imported++
		}
		items = append(items, created)
		emitRunProgress(emit, siteDiscoveryItemProgressEvent(len(items), len(candidates), created, existing != nil, run))
	}
	if err := ctx.Err(); err != nil {
		return nil, s.failRunWithProgress(ctx, run, err, emit)
	}
	finished := s.now().UTC()
	run.Status = adminplusdomain.SiteDiscoveryRunStatusSucceeded
	run.Total = len(items)
	run.SupportedTotal = supported
	run.ImportedTotal = imported
	run.FinishedAt = &finished
	updated, err := s.repo.UpdateRun(ctx, run)
	if err != nil {
		return nil, err
	}
	result := &RunResult{Run: updated, Items: items}
	emitRunProgress(emit, RunProgressEvent{
		Type:    "completed",
		Level:   "success",
		Message: "采集完成：" + stringFromInt64(int64(updated.Total)) + " 个站点，支持 " + stringFromInt64(int64(updated.SupportedTotal)) + " 个",
		Current: len(items),
		Total:   len(candidates),
		Run:     updated,
		Result:  result,
	})
	return result, nil
}

func (s *Service) acquireProxyForRun(ctx context.Context, policyID int64, run *adminplusdomain.SiteDiscoveryRun, sourceURL string) (*adminplusdomain.ProxyAssignment, *http.Client, error) {
	if s.proxyManager == nil {
		return nil, nil, badRequest("SITE_DISCOVERY_PROXY_NOT_CONFIGURED", "proxy manager is not configured")
	}
	parsed, err := url.Parse(sourceURL)
	if err != nil || parsed.Host == "" {
		return nil, nil, badRequest("SITE_DISCOVERY_SOURCE_URL_INVALID", "source url must be a valid http or https url")
	}
	assignment, err := s.proxyManager.RequestAssignment(ctx, proxyapp.RequestAssignmentInput{
		TaskType:   "site_discovery",
		TaskID:     stringFromInt64(run.ID),
		PolicyID:   policyID,
		TargetHost: parsed.Host,
		Purpose:    adminplusdomain.ProxyPurposeSiteDiscovery,
		Method:     http.MethodGet,
	})
	if err != nil {
		return nil, nil, err
	}
	if assignment.MixedPort <= 0 {
		_, _ = s.proxyManager.ReleaseAssignment(context.Background(), assignment.ID, true, "SITE_DISCOVERY_PROXY_PORT_MISSING", "proxy assignment does not include mixed port")
		return nil, nil, badRequest("SITE_DISCOVERY_PROXY_PORT_MISSING", "proxy assignment does not include mixed port")
	}
	proxyURL, err := url.Parse("http://127.0.0.1:" + stringFromInt64(int64(assignment.MixedPort)))
	if err != nil {
		return nil, nil, err
	}
	client := &http.Client{
		Timeout: 20 * time.Second,
		Transport: &proxyAwareTransport{
			base: &http.Transport{
				Proxy: http.ProxyURL(proxyURL),
			},
			manager:      s.proxyManager,
			assignmentID: assignment.ID,
			errorCode:    "SITE_DISCOVERY_PROXY_NETWORK_FAILED",
		},
	}
	return assignment, client, nil
}

type proxyAwareTransport struct {
	base         *http.Transport
	manager      ProxyManager
	assignmentID int64
	errorCode    string
	mu           sync.Mutex
}

func (t *proxyAwareTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t == nil || t.base == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "SITE_DISCOVERY_PROXY_TRANSPORT_NOT_CONFIGURED", "proxy transport is not configured")
	}
	resp, err := t.base.RoundTrip(req)
	if err == nil || !t.shouldRetryAfterSwitch(req, err) {
		return resp, err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	_, switchErr := t.manager.ReportFailure(req.Context(), t.assignmentID, proxyapp.ReportFailureInput{
		ErrorCode:    t.errorCode,
		ErrorMessage: err.Error(),
	})
	if switchErr != nil {
		return nil, err
	}
	t.base.CloseIdleConnections()
	retryReq, cloneErr := cloneHTTPClientRequest(req)
	if cloneErr != nil {
		return nil, err
	}
	return t.base.RoundTrip(retryReq)
}

func (t *proxyAwareTransport) shouldRetryAfterSwitch(req *http.Request, err error) bool {
	if t == nil || t.manager == nil || t.assignmentID <= 0 || req == nil || err == nil {
		return false
	}
	if req.Context().Err() != nil {
		return false
	}
	if !requestCanBeRetried(req) {
		return false
	}
	return true
}

func cloneHTTPClientRequest(req *http.Request) (*http.Request, error) {
	if req == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "SITE_DISCOVERY_REQUEST_EMPTY", "request is empty")
	}
	clone := req.Clone(req.Context())
	if req.Body != nil && req.GetBody != nil {
		body, err := req.GetBody()
		if err != nil {
			return nil, err
		}
		clone.Body = body
	}
	return clone, nil
}

func requestCanBeRetried(req *http.Request) bool {
	if req == nil {
		return false
	}
	switch req.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return req.Body == nil || req.GetBody != nil
	default:
		return false
	}
}

func (t *proxyAwareTransport) CloseIdleConnections() {
	if t != nil && t.base != nil {
		t.base.CloseIdleConnections()
	}
}

func (s *Service) ListItems(ctx context.Context, filter ListFilter) ([]*adminplusdomain.SiteDiscoveryItem, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("site discovery service is not configured")
	}
	if filter.ProviderType != "" && filter.ProviderType != adminplusdomain.SupplierTypeNewAPI && filter.ProviderType != adminplusdomain.SupplierTypeSub2API {
		return nil, badRequest("SITE_DISCOVERY_PROVIDER_TYPE_UNSUPPORTED", "only new_api and sub2api discovery filters are supported")
	}
	if filter.ProcessedStatus != "" && filter.ProcessedStatus != "processed" && filter.ProcessedStatus != "unprocessed" {
		return nil, badRequest("SITE_DISCOVERY_PROCESSED_STATUS_UNSUPPORTED", "processed status filter must be processed or unprocessed")
	}
	if filter.Limit <= 0 || filter.Limit > 1000 {
		filter.Limit = 1000
	}
	filter.Query = strings.TrimSpace(filter.Query)
	return s.repo.ListItems(ctx, filter)
}

func (s *Service) GetItem(ctx context.Context, itemID int64) (*adminplusdomain.SiteDiscoveryItem, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("site discovery service is not configured")
	}
	if itemID <= 0 {
		return nil, badRequest("SITE_DISCOVERY_ITEM_ID_INVALID", "invalid discovery item id")
	}
	return s.repo.GetItem(ctx, itemID)
}

func (s *Service) ImportItem(ctx context.Context, itemID int64) (*adminplusdomain.SiteDiscoveryItem, error) {
	if s == nil || s.repo == nil || s.suppliers == nil {
		return nil, internalError("site discovery import dependencies are not configured")
	}
	item, err := s.requireSupportedItem(ctx, itemID)
	if err != nil {
		return nil, err
	}
	if item.SupplierID > 0 {
		return item, nil
	}
	ensured, err := s.suppliers.EnsureFromSiteCandidate(ctx, suppliersapp.CreateFromSiteCandidateInput{
		Name:         item.Name,
		Type:         item.ProviderType,
		DashboardURL: firstNonEmpty(item.DashboardURL, item.RegisterURL, item.APIBaseURL),
		APIBaseURL:   item.APIBaseURL,
		SourceHost:   item.Host,
		SourceURL:    item.RegisterURL,
		Title:        item.Name,
	})
	if err != nil {
		return nil, err
	}
	if ensured == nil || ensured.Supplier == nil {
		return nil, internalError("failed to import discovered supplier")
	}
	return s.repo.LinkSupplier(ctx, item.ID, ensured.Supplier.ID)
}

func (s *Service) RegisterItem(ctx context.Context, itemID int64) (*adminplusdomain.SupplierRegistrationCredential, *adminplusdomain.ExtensionTask, error) {
	return s.RegisterItemWithOptions(ctx, RegisterItemInput{ItemID: itemID})
}

func (s *Service) RegisterItemWithOptions(ctx context.Context, in RegisterItemInput) (*adminplusdomain.SupplierRegistrationCredential, *adminplusdomain.ExtensionTask, error) {
	if s == nil || s.repo == nil {
		return nil, nil, internalError("site discovery registration dependencies are not configured")
	}
	unlock := s.registrationLocks.lock(in.ItemID)
	defer unlock()
	settings, err := s.GetSettings(ctx)
	if err != nil {
		return nil, nil, err
	}
	if !settings.RegistrationEnabled {
		return nil, nil, badRequest("SITE_DISCOVERY_REGISTRATION_DISABLED", "site discovery registration is disabled")
	}
	if strings.TrimSpace(settings.RegistrationEmail) == "" {
		return nil, nil, badRequest("SITE_DISCOVERY_REGISTRATION_EMAIL_REQUIRED", "registration email is required")
	}
	item, err := s.requireSupportedItem(ctx, in.ItemID)
	if err != nil {
		return nil, nil, err
	}
	if item.RegistrationStatus == adminplusdomain.SupplierRegistrationStatusSucceeded {
		return nil, nil, badRequest("SITE_DISCOVERY_ALREADY_REGISTERED", "site discovery item is already registered")
	}
	if isActiveRegistrationStatus(item.RegistrationStatus) {
		credential, _, err := s.repo.GetRegistrationCredentialByDiscoveryID(ctx, item.ID)
		if err != nil {
			return nil, nil, err
		}
		var task *adminplusdomain.ExtensionTask
		if credential != nil && credential.ExtensionTaskID > 0 && s.extension != nil {
			task, _ = s.extension.GetTask(ctx, credential.ExtensionTaskID)
		}
		return credential, task, nil
	}
	password, err := generateRegistrationPassword(item.ProviderType)
	if err != nil {
		return nil, nil, internalError("failed to generate registration password")
	}
	if s.cipher == nil {
		return nil, nil, internalError("registration credential cipher is not configured")
	}
	encrypted, err := s.cipher.Encrypt(password)
	if err != nil {
		return nil, nil, infraerrors.New(http.StatusInternalServerError, "SITE_DISCOVERY_PASSWORD_ENCRYPT_FAILED", "failed to encrypt registration password")
	}
	now := s.now().UTC()
	credential, err := s.repo.UpsertRegistrationCredential(ctx, &adminplusdomain.SupplierRegistrationCredential{
		DiscoveryID:        item.ID,
		SupplierID:         0,
		Email:              settings.RegistrationEmail,
		PasswordCiphertext: encrypted,
		PasswordConfigured: true,
		Status:             adminplusdomain.SupplierRegistrationStatusRunning,
		CreatedAt:          now,
		UpdatedAt:          now,
	})
	if err != nil {
		return nil, nil, err
	}
	return s.runRegistrationWorkflow(ctx, item, credential, password, "start_registration", in.ProxyPolicyID)
}

func (s *Service) ListRegistrationTasks(ctx context.Context, filter ListFilter) ([]*RegistrationTaskView, error) {
	if s == nil || s.repo == nil || s.extension == nil {
		return nil, internalError("site discovery registration dependencies are not configured")
	}
	if filter.ProviderType != "" && filter.ProviderType != adminplusdomain.SupplierTypeNewAPI && filter.ProviderType != adminplusdomain.SupplierTypeSub2API {
		return nil, badRequest("SITE_DISCOVERY_PROVIDER_TYPE_UNSUPPORTED", "only new_api and sub2api discovery filters are supported")
	}
	if filter.Limit <= 0 || filter.Limit > 1000 {
		filter.Limit = 1000
	}
	filter.Query = strings.TrimSpace(filter.Query)
	records, err := s.repo.ListRegistrationRecords(ctx, filter)
	if err != nil {
		return nil, err
	}
	tasks, err := s.extension.ListTasks(ctx, extensionapp.TaskFilter{
		Type:  adminplusdomain.ExtensionTaskTypeRegisterSupplier,
		Limit: 1000,
	})
	if err != nil {
		return nil, err
	}
	byID := make(map[int64]*adminplusdomain.ExtensionTask, len(tasks))
	for _, task := range tasks {
		if task != nil {
			byID[task.ID] = task
		}
	}
	views := make([]*RegistrationTaskView, 0, len(records))
	for _, record := range records {
		if record == nil || record.Credential == nil || record.Item == nil {
			continue
		}
		views = append(views, registrationTaskViewFromRecord(record.Credential, record.Item, byID[record.Credential.ExtensionTaskID]))
	}
	return views, nil
}

func (s *Service) RerunRegistration(ctx context.Context, registrationID int64) (*adminplusdomain.SupplierRegistrationCredential, *adminplusdomain.ExtensionTask, error) {
	return s.RerunRegistrationWithOptions(ctx, RerunRegistrationInput{RegistrationID: registrationID})
}

func (s *Service) RerunRegistrationWithOptions(ctx context.Context, in RerunRegistrationInput) (*adminplusdomain.SupplierRegistrationCredential, *adminplusdomain.ExtensionTask, error) {
	if s == nil || s.repo == nil {
		return nil, nil, internalError("site discovery registration dependencies are not configured")
	}
	if in.RegistrationID <= 0 {
		return nil, nil, badRequest("SITE_DISCOVERY_REGISTRATION_ID_INVALID", "invalid registration id")
	}
	credential, item, err := s.repo.GetRegistrationCredential(ctx, in.RegistrationID)
	if err != nil {
		return nil, nil, err
	}
	if credential == nil || item == nil {
		return nil, nil, infraerrors.New(http.StatusNotFound, "SITE_DISCOVERY_REGISTRATION_CREDENTIAL_NOT_FOUND", "registration credential not found")
	}
	unlock := s.registrationLocks.lock(item.ID)
	defer unlock()
	if isRegisteredDiscovery(item, credential) {
		now := s.now().UTC()
		updated, err := s.repo.MarkRegistrationSucceeded(ctx, credential.ID, firstPositiveInt64(credential.SupplierID, item.SupplierID), now)
		if err != nil {
			return nil, nil, err
		}
		return updated, nil, nil
	}
	var task *adminplusdomain.ExtensionTask
	if s.extension != nil && credential.ExtensionTaskID > 0 {
		task, err = s.extension.GetTask(ctx, credential.ExtensionTaskID)
		if err != nil {
			return nil, nil, err
		}
		if task.Type != adminplusdomain.ExtensionTaskTypeRegisterSupplier {
			return nil, nil, badRequest("SITE_DISCOVERY_REGISTRATION_TASK_REQUIRED", "extension task is not a registration task")
		}
	}
	status := item.RegistrationStatus
	if status == "" {
		status = credential.Status
	}
	status = registrationStatusFromTask(task, status)
	if !isRerunnableRegistrationStatus(status) {
		return nil, nil, badRequest("SITE_DISCOVERY_REGISTRATION_NOT_RERUNNABLE", "registration workflow is not rerunnable")
	}
	if task != nil && s.extension != nil && task.Status != adminplusdomain.ExtensionTaskStatusSucceeded && task.Status != adminplusdomain.ExtensionTaskStatusCancelled {
		if _, err := s.extension.CancelTask(ctx, extensionapp.CancelTaskInput{TaskID: task.ID, Reason: "REGISTRATION_RERUN_REQUESTED"}); err != nil {
			return nil, nil, err
		}
	}
	password, err := s.decryptRegistrationPassword(credential)
	if err != nil {
		return nil, nil, err
	}
	now := s.now().UTC()
	credential, err = s.repo.StartRegistrationAttempt(ctx, credential.ID, now)
	if err != nil {
		return nil, nil, err
	}
	return s.runRegistrationWorkflow(ctx, item, credential, password, "rerun_registration", in.ProxyPolicyID)
}

func (s *Service) ListRegistrationLogs(ctx context.Context, registrationID int64, limit int) (*RegistrationTaskLogsResult, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("site discovery registration dependencies are not configured")
	}
	if registrationID <= 0 {
		return nil, badRequest("SITE_DISCOVERY_REGISTRATION_ID_INVALID", "invalid registration id")
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	credential, item, err := s.repo.GetRegistrationCredential(ctx, registrationID)
	if err != nil {
		return nil, err
	}
	if credential == nil {
		return nil, infraerrors.New(http.StatusNotFound, "SITE_DISCOVERY_REGISTRATION_CREDENTIAL_NOT_FOUND", "registration credential not found")
	}
	var task *adminplusdomain.ExtensionTask
	if s.extension != nil && credential.ExtensionTaskID > 0 {
		if currentTask, err := s.extension.GetTask(ctx, credential.ExtensionTaskID); err == nil {
			task = currentTask
		}
	}
	components := []string{
		"admin_plus.registration",
		"admin_plus.extension",
	}
	out := make([]*opsservice.OpsSystemLog, 0, limit)
	if s.logs != nil {
		for _, component := range components {
			if list, err := s.logs.ListSystemLogs(ctx, &opsservice.OpsSystemLogFilter{
				Page:        1,
				PageSize:    limit,
				Component:   component,
				ExtraEquals: map[string]string{"registration_id": stringFromInt64(credential.ID)},
			}); err == nil {
				out = append(out, list.Logs...)
			}
		}
		if mailLogs, err := s.logs.ListSystemLogs(ctx, &opsservice.OpsSystemLogFilter{
			Page:        1,
			PageSize:    limit,
			Component:   "admin_plus.mail",
			ExtraEquals: map[string]string{"claim_key": registrationClaimKey(credential.ID)},
		}); err == nil {
			out = append(out, mailLogs.Logs...)
		}
	}
	out = append(out, registrationWorkflowSnapshotLog(credential, item, task))
	out = uniqueSystemLogs(out)
	sortSystemLogs(out)
	if len(out) > limit {
		out = out[:limit]
	}
	return &RegistrationTaskLogsResult{Items: out}, nil
}

func registrationWorkflowSnapshotLog(credential *adminplusdomain.SupplierRegistrationCredential, item *adminplusdomain.SiteDiscoveryItem, task *adminplusdomain.ExtensionTask) *opsservice.OpsSystemLog {
	if credential == nil {
		return nil
	}
	view := registrationTaskViewFromRecord(credential, item, task)
	if view == nil {
		return nil
	}
	createdAt := credential.UpdatedAt
	if credential.LastAttemptAt != nil && credential.LastAttemptAt.After(createdAt) {
		createdAt = *credential.LastAttemptAt
	}
	if createdAt.IsZero() {
		createdAt = credential.CreatedAt
	}
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	extra := map[string]any{
		"category":            bizlogs.CategoryRegistration,
		"action":              "current_status",
		"registration_id":     credential.ID,
		"registration_status": string(view.Status),
	}
	if outcome := registrationSnapshotOutcome(view.Status); outcome != "" {
		extra["outcome"] = outcome
	}
	if item != nil {
		extra["discovery_id"] = item.ID
		extra["host"] = item.Host
		extra["provider_type"] = string(item.ProviderType)
		extra["source_site_id"] = item.SourceSiteID
	}
	if task != nil {
		extra["task_id"] = task.ID
		extra["task_status"] = string(task.Status)
		extra["attempts"] = task.Attempts
		extra["max_attempts"] = task.MaxAttempts
		extra["device_id"] = task.DeviceID
	}
	if view.ErrorCode != "" {
		extra["reason"] = view.ErrorCode
	}
	if view.ErrorMessage != "" {
		extra["error_message"] = view.ErrorMessage
	}
	return &opsservice.OpsSystemLog{
		CreatedAt: createdAt.UTC(),
		Level:     registrationSnapshotLevel(view.Status),
		Component: "admin_plus.registration",
		Message:   "注册流程当前状态",
		Extra:     extra,
	}
}

func registrationSnapshotOutcome(status adminplusdomain.SupplierRegistrationStatus) string {
	switch status {
	case adminplusdomain.SupplierRegistrationStatusSucceeded:
		return bizlogs.OutcomeSucceeded
	case adminplusdomain.SupplierRegistrationStatusFailed:
		return bizlogs.OutcomeFailed
	default:
		return ""
	}
}

func registrationSnapshotLevel(status adminplusdomain.SupplierRegistrationStatus) string {
	switch status {
	case adminplusdomain.SupplierRegistrationStatusFailed, adminplusdomain.SupplierRegistrationStatusWaitingManualVerification:
		return bizlogs.LevelWarn
	default:
		return bizlogs.LevelInfo
	}
}

func (s *Service) GetTaskRegistrationCredential(ctx context.Context, taskID int64, deviceID string, leaseToken string) (*RegisterCredentialView, error) {
	if s == nil || s.repo == nil || s.extension == nil {
		return nil, internalError("site discovery registration dependencies are not configured")
	}
	task, err := s.extension.LeasedTask(ctx, taskID, deviceID, leaseToken)
	if err != nil {
		return nil, err
	}
	if task.Type != adminplusdomain.ExtensionTaskTypeRegisterSupplier {
		return nil, badRequest("SITE_DISCOVERY_REGISTRATION_TASK_REQUIRED", "extension task is not a registration task")
	}
	credential, item, err := s.repo.GetRegistrationCredentialByTaskID(ctx, task.ID)
	if err != nil {
		return nil, err
	}
	if credential == nil || item == nil {
		return nil, infraerrors.New(http.StatusNotFound, "SITE_DISCOVERY_REGISTRATION_CREDENTIAL_NOT_FOUND", "registration credential not found")
	}
	if s.cipher == nil {
		return nil, internalError("registration credential cipher is not configured")
	}
	password, err := s.cipher.Decrypt(credential.PasswordCiphertext)
	if err != nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "SITE_DISCOVERY_PASSWORD_DECRYPT_FAILED", "failed to decrypt registration password")
	}
	return &RegisterCredentialView{
		DiscoveryID:  item.ID,
		SupplierID:   item.SupplierID,
		ProviderType: item.ProviderType,
		RegisterURL:  item.RegisterURL,
		Email:        credential.Email,
		Password:     password,
	}, nil
}

func (s *Service) ReadTaskRegistrationVerificationCode(ctx context.Context, in ReadRegistrationVerificationCodeInput) (*mailverificationapp.ReadVerificationCodeResult, error) {
	if s == nil || s.repo == nil || s.extension == nil || s.mail == nil {
		return nil, internalError("site discovery registration mail dependencies are not configured")
	}
	task, err := s.extension.LeasedTask(ctx, in.TaskID, in.DeviceID, in.LeaseToken)
	if err != nil {
		return nil, err
	}
	if task.Type != adminplusdomain.ExtensionTaskTypeRegisterSupplier {
		return nil, badRequest("SITE_DISCOVERY_REGISTRATION_TASK_REQUIRED", "extension task is not a registration task")
	}
	credential, item, err := s.repo.GetRegistrationCredentialByTaskID(ctx, task.ID)
	if err != nil {
		return nil, err
	}
	if credential == nil || item == nil {
		return nil, infraerrors.New(http.StatusNotFound, "SITE_DISCOVERY_REGISTRATION_CREDENTIAL_NOT_FOUND", "registration credential not found")
	}
	triggeredAt := s.now().UTC().Add(-2 * time.Minute)
	if in.TriggeredAt != nil && !in.TriggeredAt.IsZero() {
		triggeredAt = in.TriggeredAt.UTC()
	}
	return s.mail.ReadVerificationCodeForEmail(ctx, mailverificationapp.ReadVerificationCodeForEmailInput{
		Provider:            mailverificationapp.ProviderGmail,
		Email:               credential.Email,
		ClaimKey:            registrationClaimKey(credential.ID),
		To:                  credential.Email,
		Keywords:            []string{"验证码", "verification code", "security code", "login code", "code"},
		SupplierType:        item.ProviderType,
		ExpectedPurpose:     mailverificationapp.PurposeEmailVerification,
		SiteName:            "",
		TriggeredAt:         &triggeredAt,
		TimeoutSeconds:      normalizeRegistrationCodeTimeout(in.TimeoutSeconds),
		PollIntervalSeconds: normalizeRegistrationCodePollInterval(in.PollIntervalSeconds),
		MaxResults:          10,
	})
}

func registrationClaimKey(registrationID int64) string {
	return "registration:" + stringFromInt64(registrationID)
}

func (s *Service) ListRecommendations(ctx context.Context, limit int) ([]*adminplusdomain.SiteDiscoveryRecommendation, error) {
	settings, err := s.GetSettings(ctx)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	return s.repo.ListRecommendations(ctx, settings.LowRateThreshold, limit)
}

func normalizeRegistrationCodeTimeout(seconds int) int {
	if seconds <= 0 {
		return 90
	}
	if seconds > 120 {
		return 120
	}
	return seconds
}

func normalizeRegistrationCodePollInterval(seconds int) int {
	if seconds <= 0 {
		return 5
	}
	if seconds < 2 {
		return 2
	}
	if seconds > 30 {
		return 30
	}
	return seconds
}

func (s *Service) runRegistrationWorkflow(ctx context.Context, item *adminplusdomain.SiteDiscoveryItem, credential *adminplusdomain.SupplierRegistrationCredential, password string, action string, proxyPolicyID int64) (*adminplusdomain.SupplierRegistrationCredential, *adminplusdomain.ExtensionTask, error) {
	if item == nil || credential == nil {
		return nil, nil, internalError("registration workflow requires discovery item and credential")
	}
	s.recordRegistrationEvent(ctx, action, bizlogs.OutcomeSucceeded, "site discovery direct registration started", item, credential, nil, "")
	proxyCtx, err := s.acquireProxyForRegistration(ctx, proxyPolicyID, item, credential)
	if err != nil {
		return s.failDirectRegistration(ctx, item, credential, err)
	}
	releaseProxy := func(failed bool, code string, message string) {
		s.releaseRegistrationProxy(context.Background(), proxyCtx, failed, code, message)
	}
	if s.directRegistration == nil {
		return s.queueBrowserRegistrationFallback(ctx, item, credential, "DIRECT_REGISTRATION_ADAPTER_MISSING", "direct registration adapter is not configured", proxyCtx)
	}
	workflowCtx, cancel := context.WithTimeout(ctx, defaultDirectRegistrationTTL)
	defer cancel()
	result, err := s.directRegistration.RegisterAccount(workflowCtx, ports.DirectRegistrationInput{
		ProviderType:        item.ProviderType,
		Origin:              firstNonEmpty(item.DashboardURL, item.RegisterURL, item.APIBaseURL),
		APIBaseURL:          item.APIBaseURL,
		RegisterURL:         item.RegisterURL,
		Email:               credential.Email,
		Password:            password,
		Username:            credential.Email,
		ProxyURL:            registrationProxyURL(proxyCtx),
		RegistrationContext: registrationContextPayload(item, credential, proxyCtx),
	})
	if err != nil {
		if registrationRequiresBrowserFallback(err) {
			return s.queueBrowserRegistrationFallback(ctx, item, credential, infraerrors.Reason(err), safeRegistrationErrorMessage(err), proxyCtx)
		}
		releaseProxy(true, infraerrors.Reason(err), safeRegistrationErrorMessage(err))
		return s.failDirectRegistration(ctx, item, credential, err)
	}
	if result == nil {
		releaseProxy(true, "DIRECT_REGISTRATION_RESULT_EMPTY", "direct registration returned empty result")
		return s.failDirectRegistration(ctx, item, credential, internalError("direct registration returned empty result"))
	}
	if result.Stage == ports.DirectRegistrationStageNeedEmailCode || result.EmailCodeRequired {
		s.recordRegistrationEvent(ctx, "verification_code_requested", bizlogs.OutcomeSucceeded, "registration verification code requested", item, credential, nil, "")
		code, readErr := s.readRegistrationVerificationCode(ctx, item, credential, registrationMailSiteName(item, result))
		if readErr != nil {
			releaseProxy(true, infraerrors.Reason(readErr), safeRegistrationErrorMessage(readErr))
			return s.failDirectRegistration(ctx, item, credential, readErr)
		}
		s.recordRegistrationEvent(ctx, "verification_code_read", bizlogs.OutcomeSucceeded, "registration verification code read", item, credential, nil, "")
		result, err = s.directRegistration.RegisterAccount(workflowCtx, ports.DirectRegistrationInput{
			ProviderType:        item.ProviderType,
			Origin:              firstNonEmpty(item.DashboardURL, item.RegisterURL, item.APIBaseURL),
			APIBaseURL:          item.APIBaseURL,
			RegisterURL:         item.RegisterURL,
			Email:               credential.Email,
			Password:            password,
			Username:            credential.Email,
			VerificationCode:    code,
			ProxyURL:            registrationProxyURL(proxyCtx),
			RegistrationContext: registrationContextPayload(item, credential, proxyCtx),
		})
		if err != nil {
			if registrationRequiresBrowserFallback(err) {
				return s.queueBrowserRegistrationFallback(ctx, item, credential, infraerrors.Reason(err), safeRegistrationErrorMessage(err), proxyCtx)
			}
			releaseProxy(true, infraerrors.Reason(err), safeRegistrationErrorMessage(err))
			return s.failDirectRegistration(ctx, item, credential, err)
		}
	}
	if result == nil || !result.Submitted {
		releaseProxy(true, "DIRECT_REGISTRATION_RESULT_INCOMPLETE", "direct registration result is incomplete")
		return s.failDirectRegistration(ctx, item, credential, infraerrors.New(http.StatusBadGateway, "DIRECT_REGISTRATION_RESULT_INCOMPLETE", "direct registration result is incomplete"))
	}
	supplier, err := s.ensureRegisteredSupplier(ctx, item, credential.Email, password)
	if err != nil {
		releaseProxy(true, infraerrors.Reason(err), safeRegistrationErrorMessage(err))
		return nil, nil, err
	}
	updated, err := s.repo.CompleteRegistration(ctx, credential.ID, supplier.ID, adminplusdomain.SupplierRegistrationStatusSucceeded, "", "", s.now().UTC())
	if err != nil {
		releaseProxy(true, infraerrors.Reason(err), safeRegistrationErrorMessage(err))
		return nil, nil, err
	}
	releaseProxy(false, "", "")
	s.recordRegistrationEvent(ctx, "direct_registration_succeeded", bizlogs.OutcomeSucceeded, "site discovery direct registration succeeded", item, updated, nil, "")
	return updated, nil, nil
}

func (s *Service) readRegistrationVerificationCode(ctx context.Context, item *adminplusdomain.SiteDiscoveryItem, credential *adminplusdomain.SupplierRegistrationCredential, siteName string) (string, error) {
	if s.mail == nil {
		return "", infraerrors.New(http.StatusConflict, "MAIL_VERIFICATION_READER_NOT_CONFIGURED", "mail verification reader is not configured")
	}
	siteName = firstNonEmpty(siteName, item.Name, item.Host)
	triggeredAt := s.now().UTC().Add(-2 * time.Minute)
	result, err := s.mail.ReadVerificationCodeForEmail(ctx, mailverificationapp.ReadVerificationCodeForEmailInput{
		Provider:            mailverificationapp.ProviderGmail,
		Email:               credential.Email,
		ClaimKey:            registrationClaimKey(credential.ID),
		To:                  credential.Email,
		Keywords:            []string{"验证码", "verification code", "security code", "login code", "code"},
		SupplierType:        item.ProviderType,
		ExpectedPurpose:     mailverificationapp.PurposeEmailVerification,
		SiteName:            siteName,
		TriggeredAt:         &triggeredAt,
		TimeoutSeconds:      90,
		PollIntervalSeconds: 5,
		MaxResults:          10,
	})
	if err != nil {
		return "", err
	}
	if result == nil || strings.TrimSpace(result.Code) == "" {
		return "", infraerrors.NotFound("MAIL_VERIFICATION_CODE_NOT_FOUND", "mail verification code not found")
	}
	return strings.TrimSpace(result.Code), nil
}

func registrationMailSiteName(item *adminplusdomain.SiteDiscoveryItem, result *ports.DirectRegistrationResult) string {
	if result != nil && result.Diagnostics != nil {
		if name := firstNonEmpty(
			stringFromAny(result.Diagnostics["system_name"]),
			stringFromAny(result.Diagnostics["systemName"]),
			stringFromAny(result.Diagnostics["site_name"]),
			stringFromAny(result.Diagnostics["siteName"]),
		); name != "" {
			return name
		}
	}
	if item == nil {
		return ""
	}
	return firstNonEmpty(item.Name, item.Host)
}

func (s *Service) acquireProxyForRegistration(ctx context.Context, policyID int64, item *adminplusdomain.SiteDiscoveryItem, credential *adminplusdomain.SupplierRegistrationCredential) (*registrationProxyContext, error) {
	if policyID <= 0 {
		return nil, nil
	}
	if s.proxyManager == nil {
		return nil, badRequest("SITE_DISCOVERY_PROXY_NOT_CONFIGURED", "proxy manager is not configured")
	}
	targetHost := registrationTargetHost(item)
	if targetHost == "" {
		return nil, badRequest("SITE_DISCOVERY_REGISTRATION_TARGET_INVALID", "registration target host is invalid")
	}
	assignment, err := s.proxyManager.RequestAssignment(ctx, proxyapp.RequestAssignmentInput{
		TaskType:   "registration",
		TaskID:     stringFromInt64(credential.ID),
		PolicyID:   policyID,
		TargetHost: targetHost,
		Purpose:    adminplusdomain.ProxyPurposeRegistration,
		Method:     http.MethodPost,
	})
	if err != nil {
		return nil, err
	}
	if assignment.MixedPort <= 0 {
		_, _ = s.proxyManager.ReleaseAssignment(context.Background(), assignment.ID, true, "REGISTRATION_PROXY_PORT_MISSING", "proxy assignment does not include mixed port")
		return nil, badRequest("REGISTRATION_PROXY_PORT_MISSING", "proxy assignment does not include mixed port")
	}
	return &registrationProxyContext{
		Assignment: assignment,
		ProxyURL:   "http://127.0.0.1:" + stringFromInt64(int64(assignment.MixedPort)),
	}, nil
}

func (s *Service) releaseRegistrationProxy(ctx context.Context, proxyCtx *registrationProxyContext, failed bool, code string, message string) {
	if s == nil || s.proxyManager == nil || proxyCtx == nil || proxyCtx.Assignment == nil {
		return
	}
	if proxyCtx.Assignment.Status != adminplusdomain.ProxyAssignmentActive {
		return
	}
	_, _ = s.proxyManager.ReleaseAssignment(ctx, proxyCtx.Assignment.ID, failed, code, message)
	proxyCtx.Assignment.Status = adminplusdomain.ProxyAssignmentReleased
	if failed {
		proxyCtx.Assignment.Status = adminplusdomain.ProxyAssignmentFailed
	}
}

func registrationTargetHost(item *adminplusdomain.SiteDiscoveryItem) string {
	if item == nil {
		return ""
	}
	for _, rawURL := range []string{item.RegisterURL, item.DashboardURL, item.APIBaseURL} {
		parsed, err := url.Parse(strings.TrimSpace(rawURL))
		if err == nil && parsed.Host != "" {
			return parsed.Host
		}
	}
	return strings.TrimSpace(item.Host)
}

func registrationProxyURL(proxyCtx *registrationProxyContext) string {
	if proxyCtx == nil {
		return ""
	}
	return proxyCtx.ProxyURL
}

func registrationContextPayload(item *adminplusdomain.SiteDiscoveryItem, credential *adminplusdomain.SupplierRegistrationCredential, proxyCtx *registrationProxyContext) map[string]any {
	payload := map[string]any{
		"discovery_id":     item.ID,
		"registration_id":  credential.ID,
		"provider_type":    string(item.ProviderType),
		"source_site_id":   item.SourceSiteID,
		"registration_src": "site_discovery",
	}
	if proxyCtx != nil && proxyCtx.Assignment != nil {
		payload["proxy_assignment_id"] = proxyCtx.Assignment.ID
		payload["proxy_policy_id"] = proxyCtx.Assignment.PolicyID
		payload["proxy_slot_id"] = proxyCtx.Assignment.SlotID
		payload["proxy_node_id"] = proxyCtx.Assignment.NodeID
		payload["proxy_mixed_port"] = proxyCtx.Assignment.MixedPort
		payload["proxy_required"] = true
	}
	return payload
}

func (s *Service) failDirectRegistration(ctx context.Context, item *adminplusdomain.SiteDiscoveryItem, credential *adminplusdomain.SupplierRegistrationCredential, err error) (*adminplusdomain.SupplierRegistrationCredential, *adminplusdomain.ExtensionTask, error) {
	reason := firstNonEmpty(infraerrors.Reason(err), "DIRECT_REGISTRATION_FAILED")
	message := safeRegistrationErrorMessage(err)
	updated, updateErr := s.repo.CompleteRegistration(ctx, credential.ID, credential.SupplierID, adminplusdomain.SupplierRegistrationStatusFailed, reason, message, s.now().UTC())
	if updateErr != nil {
		return nil, nil, updateErr
	}
	s.recordRegistrationErrorEvent(ctx, "direct_registration_failed", "site discovery direct registration failed", item, updated, nil, reason, err)
	return updated, nil, err
}

func (s *Service) queueBrowserRegistrationFallback(ctx context.Context, item *adminplusdomain.SiteDiscoveryItem, credential *adminplusdomain.SupplierRegistrationCredential, reason string, message string, proxyCtx *registrationProxyContext) (*adminplusdomain.SupplierRegistrationCredential, *adminplusdomain.ExtensionTask, error) {
	if s.extension == nil {
		err := infraerrors.New(http.StatusConflict, "BROWSER_FALLBACK_UNAVAILABLE", "browser registration fallback is not configured")
		s.releaseRegistrationProxy(context.Background(), proxyCtx, true, firstNonEmpty(reason, infraerrors.Reason(err)), firstNonEmpty(message, err.Error()))
		updated, updateErr := s.repo.CompleteRegistration(ctx, credential.ID, credential.SupplierID, adminplusdomain.SupplierRegistrationStatusFailed, firstNonEmpty(reason, infraerrors.Reason(err)), firstNonEmpty(message, err.Error()), s.now().UTC())
		if updateErr != nil {
			return nil, nil, updateErr
		}
		s.recordRegistrationEvent(ctx, "direct_registration_browser_fallback_unavailable", bizlogs.OutcomeFailed, "browser registration fallback is unavailable", item, updated, nil, reason)
		return updated, nil, err
	}
	now := s.now().UTC()
	task, err := s.createRegistrationAttempt(ctx, item, credential, now, proxyCtx)
	if err != nil {
		s.releaseRegistrationProxy(context.Background(), proxyCtx, true, infraerrors.Reason(err), safeRegistrationErrorMessage(err))
		return nil, nil, err
	}
	if task == nil {
		err := internalError("site discovery registration task was not created")
		s.releaseRegistrationProxy(context.Background(), proxyCtx, true, infraerrors.Reason(err), safeRegistrationErrorMessage(err))
		return nil, nil, err
	}
	updated, err := s.repo.UpdateRegistrationTask(ctx, credential.ID, task.ID, adminplusdomain.SupplierRegistrationStatusQueued, now)
	if err != nil {
		return nil, nil, err
	}
	s.recordRegistrationEvent(ctx, "direct_registration_browser_fallback", bizlogs.OutcomeSucceeded, "site discovery registration queued for browser fallback", item, updated, task, reason)
	return updated, task, nil
}

func (s *Service) decryptRegistrationPassword(credential *adminplusdomain.SupplierRegistrationCredential) (string, error) {
	if credential == nil {
		return "", infraerrors.New(http.StatusNotFound, "SITE_DISCOVERY_REGISTRATION_CREDENTIAL_NOT_FOUND", "registration credential not found")
	}
	if s.cipher == nil {
		return "", internalError("registration credential cipher is not configured")
	}
	password, err := s.cipher.Decrypt(credential.PasswordCiphertext)
	if err != nil {
		return "", infraerrors.New(http.StatusInternalServerError, "SITE_DISCOVERY_PASSWORD_DECRYPT_FAILED", "failed to decrypt registration password")
	}
	return password, nil
}

func (s *Service) ensureRegisteredSupplier(ctx context.Context, item *adminplusdomain.SiteDiscoveryItem, email string, password string) (*adminplusdomain.Supplier, error) {
	if s.suppliers == nil {
		return nil, internalError("supplier service is not configured")
	}
	if item == nil {
		return nil, infraerrors.New(http.StatusNotFound, "SITE_DISCOVERY_ITEM_NOT_FOUND", "site discovery item not found")
	}
	ensured, err := s.suppliers.EnsureFromSiteCandidateWithOptions(ctx, suppliersapp.CreateFromSiteCandidateInput{
		Name:         item.Name,
		Type:         item.ProviderType,
		DashboardURL: firstNonEmpty(item.DashboardURL, item.RegisterURL, item.APIBaseURL),
		APIBaseURL:   item.APIBaseURL,
		SourceHost:   item.Host,
		SourceURL:    item.RegisterURL,
		Title:        item.Name,
	}, suppliersapp.EnsureFromSiteCandidateOptions{AllowCreate: true})
	if err != nil {
		return nil, err
	}
	if ensured == nil || ensured.Supplier == nil {
		return nil, internalError("failed to import discovered supplier")
	}
	supplier := ensured.Supplier
	updated, err := s.suppliers.Update(ctx, supplier.ID, suppliersapp.UpdateSupplierInput{
		Name:                  supplier.Name,
		Kind:                  supplier.Kind,
		Type:                  supplier.Type,
		RuntimeStatus:         supplier.RuntimeStatus,
		HealthStatus:          supplier.HealthStatus,
		DashboardURL:          supplier.DashboardURL,
		APIBaseURL:            supplier.APIBaseURL,
		ThirdPartyRechargeURL: supplier.ThirdPartyRechargeURL,
		LocalRechargeURL:      supplier.LocalRechargeURL,
		Contact:               supplier.Contact,
		Notes:                 supplier.Notes,
		BrowserLoginEnabled:   true,
		BrowserLoginUsername:  email,
		BrowserLoginPassword:  password,
		BalanceCents:          supplier.BalanceCents,
		BalanceCurrency:       supplier.BalanceCurrency,
		RechargeMultiplier:    supplier.RechargeMultiplier,
	})
	if err != nil {
		return nil, err
	}
	if _, err := s.repo.LinkSupplier(ctx, item.ID, updated.ID); err != nil {
		return nil, err
	}
	return updated, nil
}

func registrationRequiresBrowserFallback(err error) bool {
	switch infraerrors.Reason(err) {
	case "BROWSER_FALLBACK_REQUIRED", "BROWSER_CHALLENGE_REQUIRED", "LOGIN_CAPTCHA_REQUIRED", "REGISTRATION_CAPTCHA_REQUIRED":
		return true
	default:
		return false
	}
}

func safeRegistrationErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	message := strings.TrimSpace(infraerrors.Message(err))
	if message == "" {
		message = strings.TrimSpace(err.Error())
	}
	if message == "" {
		return ""
	}
	message = strings.ReplaceAll(message, "\n", " ")
	return trimLimit(message, 500)
}

func (s *Service) createRegistrationAttempt(ctx context.Context, item *adminplusdomain.SiteDiscoveryItem, credential *adminplusdomain.SupplierRegistrationCredential, now time.Time, proxyCtx *registrationProxyContext) (*adminplusdomain.ExtensionTask, error) {
	if item == nil || credential == nil {
		return nil, internalError("registration attempt requires discovery item and credential")
	}
	payload := registrationContextPayload(item, credential, proxyCtx)
	payload["register_url"] = item.RegisterURL
	payload["dashboard_url"] = item.DashboardURL
	payload["api_base_url"] = item.APIBaseURL
	payload["requires_manual"] = true
	payload["password_in_task"] = false
	task, err := s.extension.CreateTask(ctx, extensionapp.CreateTaskInput{
		SupplierID:  0,
		Type:        adminplusdomain.ExtensionTaskTypeRegisterSupplier,
		ScheduleKey: registrationScheduleKey(item.ID, now),
		Priority:    50,
		MaxAttempts: 1,
		Payload:     payload,
	})
	if err != nil {
		s.recordRegistrationEvent(ctx, "create_attempt", bizlogs.OutcomeFailed, "site discovery registration attempt creation failed", item, credential, nil, infraerrors.Reason(err))
		return nil, err
	}
	return task, nil
}

func (s *Service) recordRegistrationEvent(ctx context.Context, action string, outcome string, message string, item *adminplusdomain.SiteDiscoveryItem, credential *adminplusdomain.SupplierRegistrationCredential, task *adminplusdomain.ExtensionTask, reason string) {
	if s == nil || s.bizlog == nil {
		return
	}
	metadata := registrationEventMetadata(action, item, credential, task)
	s.bizlog.Record(ctx, bizlogs.Event{
		Level:        registrationLogLevel(outcome),
		Category:     bizlogs.CategoryRegistration,
		Action:       action,
		Outcome:      outcome,
		Message:      message,
		SupplierID:   registrationSupplierID(credential),
		ProviderType: registrationProviderType(item),
		Reason:       reason,
		Metadata:     metadata,
	})
}

func (s *Service) recordRegistrationErrorEvent(ctx context.Context, action string, message string, item *adminplusdomain.SiteDiscoveryItem, credential *adminplusdomain.SupplierRegistrationCredential, task *adminplusdomain.ExtensionTask, reason string, err error) {
	if s == nil || s.bizlog == nil {
		return
	}
	event := bizlogs.Event{
		Level:        bizlogs.LevelWarn,
		Category:     bizlogs.CategoryRegistration,
		Action:       action,
		Outcome:      bizlogs.OutcomeFailed,
		Message:      message,
		SupplierID:   registrationSupplierID(credential),
		ProviderType: registrationProviderType(item),
		Reason:       firstNonEmpty(reason, infraerrors.Reason(err)),
		Metadata:     registrationEventMetadata(action, item, credential, task),
	}
	s.bizlog.Record(ctx, bizlogs.EventFromError(event, err))
}

func registrationEventMetadata(action string, item *adminplusdomain.SiteDiscoveryItem, credential *adminplusdomain.SupplierRegistrationCredential, task *adminplusdomain.ExtensionTask) map[string]any {
	metadata := map[string]any{
		"action": action,
	}
	if item != nil {
		metadata["discovery_id"] = item.ID
		metadata["host"] = item.Host
		metadata["provider_type"] = string(item.ProviderType)
		metadata["source_site_id"] = item.SourceSiteID
	}
	if credential != nil {
		metadata["registration_id"] = credential.ID
		metadata["registration_status"] = string(credential.Status)
		if credential.SupplierID > 0 {
			metadata["supplier_id"] = credential.SupplierID
		}
	}
	if task != nil {
		metadata["task_id"] = task.ID
		metadata["task_status"] = string(task.Status)
		metadata["attempts"] = task.Attempts
		metadata["max_attempts"] = task.MaxAttempts
	}
	return metadata
}

func registrationLogLevel(outcome string) string {
	if outcome == bizlogs.OutcomeFailed {
		return bizlogs.LevelWarn
	}
	return bizlogs.LevelInfo
}

func registrationSupplierID(credential *adminplusdomain.SupplierRegistrationCredential) int64 {
	if credential == nil {
		return 0
	}
	return credential.SupplierID
}

func registrationProviderType(item *adminplusdomain.SiteDiscoveryItem) string {
	if item == nil {
		return ""
	}
	return string(item.ProviderType)
}

func sortSystemLogs(items []*opsservice.OpsSystemLog) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i] == nil {
			return false
		}
		if items[j] == nil {
			return true
		}
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].ID > items[j].ID
		}
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
}

func uniqueSystemLogs(items []*opsservice.OpsSystemLog) []*opsservice.OpsSystemLog {
	if len(items) == 0 {
		return items
	}
	seen := make(map[int64]struct{}, len(items))
	out := make([]*opsservice.OpsSystemLog, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		if item.ID > 0 {
			if _, ok := seen[item.ID]; ok {
				continue
			}
			seen[item.ID] = struct{}{}
		}
		out = append(out, item)
	}
	return out
}

func registrationTaskViewFromRecord(credential *adminplusdomain.SupplierRegistrationCredential, item *adminplusdomain.SiteDiscoveryItem, task *adminplusdomain.ExtensionTask) *RegistrationTaskView {
	if credential == nil || item == nil {
		return nil
	}
	status := credential.Status
	if status == "" {
		status = item.RegistrationStatus
	}
	if status == "" {
		status = adminplusdomain.SupplierRegistrationStatusPending
	}
	if isRegisteredDiscovery(item, credential) {
		status = adminplusdomain.SupplierRegistrationStatusSucceeded
	}
	errorCode := firstNonEmpty(credential.ErrorCode, item.RegistrationErrorCode)
	errorMessage := firstNonEmpty(credential.ErrorMessage, item.RegistrationErrorMessage)
	createdAt := credential.CreatedAt
	updatedAt := credential.UpdatedAt
	if createdAt.IsZero() {
		createdAt = item.CreatedAt
	}
	if updatedAt.IsZero() {
		updatedAt = item.UpdatedAt
	}
	finishedAt := credential.LastAttemptAt
	taskID := credential.ExtensionTaskID
	taskStatus := adminplusdomain.ExtensionTaskStatus("")
	attempts := 0
	maxAttempts := 0
	deviceID := ""
	if status == adminplusdomain.SupplierRegistrationStatusSucceeded && isRegisteredDiscovery(item, credential) {
		taskID = 0
		errorCode = ""
		errorMessage = ""
	} else if task != nil && !isTerminalRegistrationStatus(status) {
		taskID = task.ID
		taskStatus = task.Status
		if derived := registrationStatusFromTask(task, status); derived != "" {
			status = derived
		}
		if task.ErrorCode != "" {
			errorCode = task.ErrorCode
		}
		if task.ErrorMessage != "" {
			errorMessage = task.ErrorMessage
		}
		updatedAt = task.UpdatedAt
		finishedAt = task.FinishedAt
		attempts = task.Attempts
		maxAttempts = task.MaxAttempts
		deviceID = task.DeviceID
	}
	discovery := cloneRegistrationDiscoveryForView(item, credential, status, taskID, errorCode, errorMessage)
	return &RegistrationTaskView{
		ID:             credential.ID,
		DiscoveryID:    item.ID,
		RegistrationID: credential.ID,
		TaskID:         taskID,
		Status:         status,
		TaskStatus:     taskStatus,
		Email:          credential.Email,
		ErrorCode:      errorCode,
		ErrorMessage:   errorMessage,
		Attempts:       attempts,
		MaxAttempts:    maxAttempts,
		DeviceID:       deviceID,
		CanRetry:       isRerunnableRegistrationStatus(status),
		LastAttemptAt:  credential.LastAttemptAt,
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
		FinishedAt:     finishedAt,
		Discovery:      discovery,
	}
}

func cloneRegistrationDiscoveryForView(item *adminplusdomain.SiteDiscoveryItem, credential *adminplusdomain.SupplierRegistrationCredential, status adminplusdomain.SupplierRegistrationStatus, taskID int64, errorCode string, errorMessage string) *adminplusdomain.SiteDiscoveryItem {
	if item == nil {
		return nil
	}
	cp := *item
	if credential != nil {
		cp.SupplierID = firstPositiveInt64(credential.SupplierID, cp.SupplierID)
		cp.RegistrationEmail = credential.Email
	}
	cp.RegistrationStatus = status
	cp.RegistrationTaskID = taskID
	cp.RegistrationErrorCode = errorCode
	cp.RegistrationErrorMessage = errorMessage
	return &cp
}

func isRegisteredDiscovery(item *adminplusdomain.SiteDiscoveryItem, credential *adminplusdomain.SupplierRegistrationCredential) bool {
	if credential != nil && credential.SupplierID > 0 {
		return true
	}
	if item == nil {
		return false
	}
	if item.SupplierID > 0 {
		return true
	}
	return item.ProcessStatus == adminplusdomain.SiteDiscoveryProcessRegistered ||
		item.RegistrationStatus == adminplusdomain.SupplierRegistrationStatusSucceeded
}

func registrationStatusFromTask(task *adminplusdomain.ExtensionTask, fallback adminplusdomain.SupplierRegistrationStatus) adminplusdomain.SupplierRegistrationStatus {
	if task == nil {
		return fallback
	}
	if isTerminalRegistrationStatus(fallback) {
		return fallback
	}
	switch task.Status {
	case adminplusdomain.ExtensionTaskStatusPending:
		return adminplusdomain.SupplierRegistrationStatusQueued
	case adminplusdomain.ExtensionTaskStatusClaimed, adminplusdomain.ExtensionTaskStatusRunning:
		return adminplusdomain.SupplierRegistrationStatusRunning
	case adminplusdomain.ExtensionTaskStatusFailed:
		if task.ErrorCode == "REGISTRATION_VERIFICATION_REQUIRED" {
			return adminplusdomain.SupplierRegistrationStatusWaitingManualVerification
		}
		return adminplusdomain.SupplierRegistrationStatusFailed
	case adminplusdomain.ExtensionTaskStatusSucceeded:
		if fallback != "" {
			return fallback
		}
		return adminplusdomain.SupplierRegistrationStatusSucceeded
	default:
		return fallback
	}
}

func isTerminalRegistrationStatus(status adminplusdomain.SupplierRegistrationStatus) bool {
	return status == adminplusdomain.SupplierRegistrationStatusSucceeded || status == adminplusdomain.SupplierRegistrationStatusFailed || status == adminplusdomain.SupplierRegistrationStatusWaitingManualVerification
}

func isActiveRegistrationStatus(status adminplusdomain.SupplierRegistrationStatus) bool {
	return status == adminplusdomain.SupplierRegistrationStatusQueued || status == adminplusdomain.SupplierRegistrationStatusRunning || status == adminplusdomain.SupplierRegistrationStatusPending
}

func isRerunnableRegistrationStatus(status adminplusdomain.SupplierRegistrationStatus) bool {
	return status == adminplusdomain.SupplierRegistrationStatusQueued ||
		status == adminplusdomain.SupplierRegistrationStatusRunning ||
		status == adminplusdomain.SupplierRegistrationStatusFailed ||
		status == adminplusdomain.SupplierRegistrationStatusWaitingManualVerification
}

func firstPositiveInt64(values ...int64) int64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func (s *Service) requireSupportedItem(ctx context.Context, itemID int64) (*adminplusdomain.SiteDiscoveryItem, error) {
	if itemID <= 0 {
		return nil, badRequest("SITE_DISCOVERY_ITEM_ID_INVALID", "invalid discovery item id")
	}
	item, err := s.repo.GetItem(ctx, itemID)
	if err != nil {
		return nil, err
	}
	if item.ProviderType != adminplusdomain.SupplierTypeNewAPI && item.ProviderType != adminplusdomain.SupplierTypeSub2API {
		return nil, badRequest("SITE_DISCOVERY_PROVIDER_TYPE_UNSUPPORTED", "only new_api and sub2api discoveries can be imported")
	}
	if item.ClassificationStatus != adminplusdomain.SiteDiscoveryClassificationSupported {
		return nil, badRequest("SITE_DISCOVERY_ITEM_NOT_SUPPORTED", "discovery item is not classified as supported")
	}
	return item, nil
}

func (s *Service) failRun(ctx context.Context, run *adminplusdomain.SiteDiscoveryRun, cause error) error {
	if run == nil {
		return cause
	}
	finished := s.now().UTC()
	run.Status = adminplusdomain.SiteDiscoveryRunStatusFailed
	run.ErrorMessage = strings.TrimSpace(cause.Error())
	run.FinishedAt = &finished
	_, _ = s.repo.UpdateRun(ctx, run)
	return cause
}

func (s *Service) failRunWithProgress(ctx context.Context, run *adminplusdomain.SiteDiscoveryRun, cause error, emit RunProgressEmitter) error {
	err := s.failRun(ctx, run, cause)
	message := "采集失败"
	if cause != nil {
		message = "采集失败：" + strings.TrimSpace(cause.Error())
	}
	emitRunProgress(emit, RunProgressEvent{
		Type:    "failed",
		Level:   "error",
		Message: message,
		Run:     run,
	})
	return err
}

func emitRunProgress(emit RunProgressEmitter, event RunProgressEvent) {
	if emit == nil {
		return
	}
	emit(event)
}

func siteDiscoveryItemProgressEvent(current int, total int, item *adminplusdomain.SiteDiscoveryItem, existed bool, run *adminplusdomain.SiteDiscoveryRun) RunProgressEvent {
	if item == nil {
		return RunProgressEvent{
			Type:    "log",
			Level:   "warning",
			Message: "候选处理完成，但返回结果为空",
			Current: current,
			Total:   total,
			Run:     run,
		}
	}
	name := firstNonEmpty(item.Name, item.Host, item.RegisterURL)
	provider := string(item.ProviderType)
	switch {
	case existed:
		return RunProgressEvent{
			Type:    "item_skipped",
			Level:   "warning",
			Message: "已存在，已更新候选快照：" + name,
			Current: current,
			Total:   total,
			Run:     run,
			Item:    item,
		}
	case item.ClassificationStatus == adminplusdomain.SiteDiscoveryClassificationSupported:
		if provider == "" {
			provider = "supported"
		}
		return RunProgressEvent{
			Type:    "item_success",
			Level:   "success",
			Message: "识别成功：" + name + "（" + provider + "）",
			Current: current,
			Total:   total,
			Run:     run,
			Item:    item,
		}
	case item.ClassificationStatus == adminplusdomain.SiteDiscoveryClassificationUnsupported:
		return RunProgressEvent{
			Type:    "item_unknown",
			Level:   "warning",
			Message: "暂不支持：" + name,
			Current: current,
			Total:   total,
			Run:     run,
			Item:    item,
		}
	default:
		return RunProgressEvent{
			Type:    "item_unknown",
			Level:   "warning",
			Message: "未识别：" + name,
			Current: current,
			Total:   total,
			Run:     run,
			Item:    item,
		}
	}
}

func siteDiscoveryClassifyProgressEvent(current int, total int, item *adminplusdomain.SiteDiscoveryItem) RunProgressEvent {
	if item == nil {
		return RunProgressEvent{
			Type:    "item_unknown",
			Level:   "warning",
			Message: "候选识别完成，但返回结果为空",
			Current: current,
			Total:   total,
		}
	}
	name := firstNonEmpty(item.Name, item.Host, item.RegisterURL)
	provider := string(item.ProviderType)
	if item.ClassificationStatus == adminplusdomain.SiteDiscoveryClassificationSupported {
		if provider == "" {
			provider = "supported"
		}
		return RunProgressEvent{
			Type:    "item_success",
			Level:   "success",
			Message: "识别成功：" + name + "（" + provider + "）",
			Current: current,
			Total:   total,
			Item:    item,
		}
	}
	return RunProgressEvent{
		Type:    "item_unknown",
		Level:   "warning",
		Message: "接口未命中：" + name,
		Current: current,
		Total:   total,
		Item:    item,
	}
}

func (s *Service) classifyCandidatesStream(ctx context.Context, candidates []*adminplusdomain.SiteDiscoveryItem, probeInterfaces bool, probePages bool) <-chan *adminplusdomain.SiteDiscoveryItem {
	return s.classifyCandidatesStreamWithClient(ctx, candidates, probeInterfaces, probePages, s.client)
}

func (s *Service) classifyCandidatesStreamWithClient(ctx context.Context, candidates []*adminplusdomain.SiteDiscoveryItem, probeInterfaces bool, probePages bool, client *http.Client) <-chan *adminplusdomain.SiteDiscoveryItem {
	results := make(chan *adminplusdomain.SiteDiscoveryItem)
	if len(candidates) == 0 {
		close(results)
		return results
	}
	workers := defaultSiteProbeWorkers
	if len(candidates) < workers {
		workers = len(candidates)
	}
	jobs := make(chan *adminplusdomain.SiteDiscoveryItem)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case item, ok := <-jobs:
					if !ok {
						return
					}
					s.classifyCandidateWithClient(ctx, item, probeInterfaces, probePages, client)
					select {
					case <-ctx.Done():
						return
					case results <- item:
					}
				}
			}
		}()
	}
	go func() {
		defer close(jobs)
		for _, item := range candidates {
			select {
			case <-ctx.Done():
				return
			case jobs <- item:
			}
		}
	}()
	go func() {
		wg.Wait()
		close(results)
	}()
	return results
}

func (s *Service) classifyCandidateWithClient(ctx context.Context, item *adminplusdomain.SiteDiscoveryItem, probeInterfaces bool, probePages bool, client *http.Client) {
	classification := classifyItem(item).Merge(existingClassification(item))
	if probeInterfaces && classification.Confidence < 0.95 {
		classification = classification.Merge(s.probeKnownProviderInterfacesWithClient(ctx, item, client))
	}
	if probePages && classification.Confidence < 0.95 {
		classification = classification.Merge(s.probeSitePageClassificationWithClient(ctx, item, client))
	}
	item.ProviderType = classification.ProviderType
	item.ClassificationStatus = classification.Status
	item.ClassificationConfidence = classification.Confidence
	item.ClassificationEvidence = classification.Evidence
}

func (s *Service) classifyCandidate(ctx context.Context, item *adminplusdomain.SiteDiscoveryItem, probeInterfaces bool, probePages bool) {
	s.classifyCandidateWithClient(ctx, item, probeInterfaces, probePages, s.client)
}

func existingClassification(item *adminplusdomain.SiteDiscoveryItem) classificationResult {
	if item == nil || item.ProviderType == "" || item.ClassificationStatus != adminplusdomain.SiteDiscoveryClassificationSupported {
		return classificationResult{}
	}
	confidence := item.ClassificationConfidence
	if confidence <= 0 {
		confidence = 0.95
	}
	return classificationResult{
		ProviderType: item.ProviderType,
		Status:       item.ClassificationStatus,
		Confidence:   confidence,
		Evidence:     append([]string(nil), item.ClassificationEvidence...),
	}
}

func (s *Service) fetchText(ctx context.Context, rawURL string, limit int64) (string, error) {
	return s.fetchTextWithClient(ctx, s.client, rawURL, limit)
}

func (s *Service) fetchTextWithClient(ctx context.Context, client *http.Client, rawURL string, limit int64) (string, error) {
	if client == nil {
		client = s.client
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Sub2API-Admin-Plus-SiteDiscovery/1.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/json;q=0.8,*/*;q=0.5")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", infraerrors.New(resp.StatusCode, "SITE_DISCOVERY_FETCH_FAILED", "failed to fetch discovery source")
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func (s *Service) fetchMonitorDataWithClient(ctx context.Context, client *http.Client, sourceURL string) map[string]map[string]any {
	parsed, err := url.Parse(sourceURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil
	}
	monitorURL := parsed.Scheme + "://" + parsed.Host + "/api.php?action=status&days=7"
	body, err := s.fetchTextWithClient(ctx, client, monitorURL, defaultSiteProbeLimit)
	if err != nil {
		return nil
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(body), &rows); err != nil {
		return nil
	}
	out := make(map[string]map[string]any, len(rows))
	for _, row := range rows {
		site := mapValue(row, "site")
		id := strings.TrimSpace(stringFromAny(site["id"]))
		if id != "" {
			out[id] = row
		}
	}
	return out
}

func (s *Service) probeSiteClassification(ctx context.Context, item *adminplusdomain.SiteDiscoveryItem) classificationResult {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	if result := s.probeKnownProviderInterfaces(ctx, item); result.Status == adminplusdomain.SiteDiscoveryClassificationSupported {
		return result
	}
	target := firstNonEmpty(item.RegisterURL, item.DashboardURL, item.APIBaseURL)
	if target == "" {
		return classificationResult{}
	}
	body, err := s.fetchText(ctx, target, defaultSiteProbeLimit)
	if err != nil {
		return classificationResult{}
	}
	return classifyText(body, "site")
}

func (s *Service) probeSitePageClassificationWithClient(ctx context.Context, item *adminplusdomain.SiteDiscoveryItem, client *http.Client) classificationResult {
	target := firstNonEmpty(item.RegisterURL, item.DashboardURL, item.APIBaseURL)
	if target == "" {
		return classificationResult{}
	}
	ctx, cancel := context.WithTimeout(ctx, defaultPageProbeTTL)
	defer cancel()
	body, err := s.fetchTextWithClient(ctx, client, target, defaultSiteProbeLimit)
	if err != nil {
		return classificationResult{}
	}
	return classifyText(body, "site")
}

func (s *Service) probeKnownProviderInterfaces(ctx context.Context, item *adminplusdomain.SiteDiscoveryItem) classificationResult {
	return s.probeKnownProviderInterfacesWithClient(ctx, item, s.client)
}

func (s *Service) probeKnownProviderInterfacesWithClient(ctx context.Context, item *adminplusdomain.SiteDiscoveryItem, client *http.Client) classificationResult {
	ctx, cancel := context.WithTimeout(ctx, defaultInterfaceProbeTTL)
	defer cancel()
	for _, origin := range candidateOrigins(item) {
		if ctx.Err() != nil {
			return classificationResult{}
		}
		sub2apiCtx, cancelSub2API := context.WithTimeout(ctx, defaultEndpointProbeTTL)
		result := s.probeSub2APIInterfaceWithClient(sub2apiCtx, client, origin)
		cancelSub2API()
		if result.Status == adminplusdomain.SiteDiscoveryClassificationSupported {
			return result
		}
		if ctx.Err() != nil {
			return classificationResult{}
		}
		newAPICtx, cancelNewAPI := context.WithTimeout(ctx, defaultEndpointProbeTTL)
		result = s.probeNewAPIInterfaceWithClient(newAPICtx, client, origin)
		cancelNewAPI()
		if result.Status == adminplusdomain.SiteDiscoveryClassificationSupported {
			return result
		}
	}
	return classificationResult{}
}

func (s *Service) probeSub2APIInterfaceWithClient(ctx context.Context, client *http.Client, origin string) classificationResult {
	body, err := s.fetchTextWithClient(ctx, client, joinURLPath(origin, "/api/v1/settings/public"), defaultSiteProbeLimit)
	if err != nil {
		return classificationResult{}
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return classificationResult{}
	}
	data := mapValue(payload, "data")
	if _, hasCode := payload["code"]; !hasCode || !looksLikeSub2APIPublicSettings(data) {
		return classificationResult{}
	}
	return classificationResult{
		ProviderType: adminplusdomain.SupplierTypeSub2API,
		Status:       adminplusdomain.SiteDiscoveryClassificationSupported,
		Confidence:   0.98,
		Evidence:     []string{"api:/api/v1/settings/public", "api:sub2api_public_settings"},
	}
}

func (s *Service) probeNewAPIInterfaceWithClient(ctx context.Context, client *http.Client, origin string) classificationResult {
	body, err := s.fetchTextWithClient(ctx, client, joinURLPath(origin, "/api/status"), defaultSiteProbeLimit)
	if err != nil {
		return classificationResult{}
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return classificationResult{}
	}
	data := mapValue(payload, "data")
	if !boolFromAny(payload["success"]) || !looksLikeNewAPIStatus(data) {
		return classificationResult{}
	}
	return classificationResult{
		ProviderType: adminplusdomain.SupplierTypeNewAPI,
		Status:       adminplusdomain.SiteDiscoveryClassificationSupported,
		Confidence:   0.98,
		Evidence:     []string{"api:/api/status", "api:new_api_status"},
	}
}

func candidateOrigins(item *adminplusdomain.SiteDiscoveryItem) []string {
	rawURLs := []string{item.APIBaseURL, item.DashboardURL, item.RegisterURL}
	seen := make(map[string]struct{}, len(rawURLs))
	origins := make([]string, 0, len(rawURLs))
	for _, rawURL := range rawURLs {
		parsed, err := url.Parse(strings.TrimSpace(rawURL))
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			continue
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			continue
		}
		origin := parsed.Scheme + "://" + parsed.Host
		if _, ok := seen[origin]; ok {
			continue
		}
		seen[origin] = struct{}{}
		origins = append(origins, origin)
	}
	return origins
}

func joinURLPath(origin string, path string) string {
	return strings.TrimRight(origin, "/") + "/" + strings.TrimLeft(path, "/")
}

func looksLikeSub2APIPublicSettings(data map[string]any) bool {
	if len(data) == 0 {
		return false
	}
	_, hasVersion := data["version"]
	_, hasSiteName := data["site_name"]
	_, hasAPIBaseURL := data["api_base_url"]
	_, hasRegistration := data["registration_enabled"]
	_, hasPageSize := data["table_default_page_size"]
	_, hasChannelMonitor := data["channel_monitor_enabled"]
	return hasVersion && hasSiteName && hasAPIBaseURL && hasRegistration && (hasPageSize || hasChannelMonitor)
}

func looksLikeNewAPIStatus(data map[string]any) bool {
	if len(data) == 0 {
		return false
	}
	_, hasVersion := data["version"]
	_, hasQuotaPerUnit := data["quota_per_unit"]
	_, hasSystemName := data["system_name"]
	_, hasSetup := data["setup"]
	_, hasRegister := data["register_enabled"]
	_, hasPasswordLogin := data["password_login_enabled"]
	return hasVersion && hasQuotaPerUnit && hasSystemName && hasSetup && (hasRegister || hasPasswordLogin)
}

func parseDaheiAIItems(sourceURL string, body string) ([]*adminplusdomain.SiteDiscoveryItem, error) {
	root, err := html.Parse(bytes.NewBufferString(body))
	if err != nil {
		return nil, err
	}
	items := make([]*adminplusdomain.SiteDiscoveryItem, 0)
	var walkChildren func(*html.Node, string, string)
	walkChildren = func(n *html.Node, section string, category string) {
		if n == nil {
			return
		}
		currentCategory := category
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			childSection := section
			if child.Type == html.ElementNode && child.Data == "section" {
				if id := attr(child, "id"); id != "" {
					childSection = id
					currentCategory = ""
				}
			}
			childCategory := currentCategory
			if child.Type == html.ElementNode && child.Data == "h2" && hasClass(child, "sub-title") {
				currentCategory = trimLimit(nodeText(child), 120)
				childCategory = currentCategory
			}
			if child.Type == html.ElementNode && child.Data == "a" && hasClass(child, "card") {
				if item := parseCard(sourceURL, child, childSection, childCategory); item != nil {
					items = append(items, item)
				}
			}
			walkChildren(child, childSection, childCategory)
		}
	}
	walkChildren(root, "", "")
	thirdParty := make([]*adminplusdomain.SiteDiscoveryItem, 0, len(items))
	for _, item := range items {
		if item.SourceSection == "third-party" {
			thirdParty = append(thirdParty, item)
		}
	}
	if len(thirdParty) > 0 {
		return thirdParty, nil
	}
	return items, nil
}

func (s *Service) parseSourceCandidates(ctx context.Context, client *http.Client, sourceURL string, body string) ([]*adminplusdomain.SiteDiscoveryItem, error) {
	items, err := parseDaheiAIItems(sourceURL, body)
	if err != nil {
		return nil, err
	}
	if len(items) > 0 {
		return items, nil
	}
	if items := parseKanLLMSummaryItems(sourceURL, body); len(items) > 0 {
		return items, nil
	}
	if summaryURL := kanLLMSummaryURL(sourceURL); summaryURL != "" {
		if summaryBody, err := s.fetchTextWithClient(ctx, client, summaryURL, defaultDiscoveryFetchLimit); err == nil {
			if items := parseKanLLMSummaryItems(sourceURL, summaryBody); len(items) > 0 {
				return items, nil
			}
		}
	}
	if item := parseDirectSiteItem(sourceURL, body); item != nil {
		return []*adminplusdomain.SiteDiscoveryItem{item}, nil
	}
	return []*adminplusdomain.SiteDiscoveryItem{}, nil
}

func parseCard(sourceURL string, n *html.Node, section string, category string) *adminplusdomain.SiteDiscoveryItem {
	href := strings.TrimSpace(attr(n, "href"))
	if href == "" {
		return nil
	}
	registerURL := resolveURL(sourceURL, href)
	parsed, err := url.Parse(registerURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil
	}
	name := strings.TrimSpace(firstTextByClass(n, "name"))
	if name == "" {
		name = parsed.Host
	}
	desc := strings.TrimSpace(firstTextByClass(n, "desc"))
	domainHint := strings.TrimSpace(attr(n, "data-domain"))
	sourceSiteID := strings.TrimSpace(attr(n, "data-site-id"))
	origin := parsed.Scheme + "://" + parsed.Host
	return &adminplusdomain.SiteDiscoveryItem{
		SourceURL:                sourceURL,
		SourceSiteID:             sourceSiteID,
		SourceSection:            section,
		SourceCategory:           category,
		Name:                     trimLimit(name, 120),
		RegisterURL:              registerURL,
		DashboardURL:             origin,
		APIBaseURL:               origin,
		Host:                     parsed.Host,
		DomainHint:               domainHint,
		Description:              trimLimit(desc, 1000),
		ClassificationStatus:     adminplusdomain.SiteDiscoveryClassificationUnknown,
		ClassificationConfidence: 0,
		ImportStatus:             adminplusdomain.SiteDiscoveryImportNew,
		RawPayload: map[string]any{
			"href":            href,
			"data_site_id":    sourceSiteID,
			"data_domain":     domainHint,
			"source_section":  section,
			"source_category": category,
		},
	}
}

type kanLLMSummaryPayload struct {
	GeneratedAt string             `json:"generatedAt"`
	APIs        []kanLLMSummaryAPI `json:"apis"`
}

type kanLLMSummaryAPI struct {
	ID              string             `json:"id"`
	Name            string             `json:"name"`
	WebsiteURL      string             `json:"websiteUrl"`
	PlanType        string             `json:"planType"`
	IsSelfPurchased bool               `json:"isSelfPurchased"`
	PriceMultiplier float64            `json:"priceMultiplier"`
	Enabled         bool               `json:"enabled"`
	Available       bool               `json:"available"`
	SuccessRates    map[string]float64 `json:"successRates"`
	CheckedAt       string             `json:"checkedAt"`
	ErrorType       string             `json:"errorType"`
	ErrorMessage    string             `json:"errorMessage"`
}

type kanLLMSiteAggregate struct {
	RegisterURL      string
	Origin           string
	Host             string
	Name             string
	APIIDs           []string
	PlanTypes        []string
	PlanSet          map[string]struct{}
	MinRate          float64
	EnabledCount     int
	AvailableCount   int
	TotalCount       int
	IsSelfPurchased  bool
	LatestCheckedAt  string
	LastErrorType    string
	LastErrorMessage string
	SuccessRate24H   float64
	SuccessRate24HOK bool
}

func parseKanLLMSummaryItems(sourceURL string, body string) []*adminplusdomain.SiteDiscoveryItem {
	var payload kanLLMSummaryPayload
	if err := json.Unmarshal([]byte(body), &payload); err != nil || len(payload.APIs) == 0 {
		return nil
	}
	aggregates := make(map[string]*kanLLMSiteAggregate)
	order := make([]string, 0)
	for _, api := range payload.APIs {
		registerURL, origin, host, ok := normalizeDiscoverySiteURL(api.WebsiteURL)
		if !ok {
			continue
		}
		agg := aggregates[registerURL]
		if agg == nil {
			agg = &kanLLMSiteAggregate{
				RegisterURL: registerURL,
				Origin:      origin,
				Host:        host,
				Name:        firstNonEmpty(api.Name, host),
				PlanSet:     make(map[string]struct{}),
			}
			aggregates[registerURL] = agg
			order = append(order, registerURL)
		}
		agg.TotalCount++
		if strings.TrimSpace(api.ID) != "" {
			agg.APIIDs = append(agg.APIIDs, strings.TrimSpace(api.ID))
		}
		if api.Enabled {
			agg.EnabledCount++
		}
		if api.Available {
			agg.AvailableCount++
		}
		if api.IsSelfPurchased {
			agg.IsSelfPurchased = true
		}
		if api.PriceMultiplier > 0 && (agg.MinRate <= 0 || api.PriceMultiplier < agg.MinRate) {
			agg.MinRate = api.PriceMultiplier
		}
		if plan := strings.TrimSpace(api.PlanType); plan != "" {
			if _, ok := agg.PlanSet[plan]; !ok {
				agg.PlanSet[plan] = struct{}{}
				agg.PlanTypes = append(agg.PlanTypes, plan)
			}
		}
		if api.CheckedAt > agg.LatestCheckedAt {
			agg.LatestCheckedAt = api.CheckedAt
		}
		if strings.TrimSpace(api.ErrorType) != "" {
			agg.LastErrorType = strings.TrimSpace(api.ErrorType)
			agg.LastErrorMessage = strings.TrimSpace(api.ErrorMessage)
		}
		if rate, ok := api.SuccessRates["24h"]; ok && (!agg.SuccessRate24HOK || rate > agg.SuccessRate24H) {
			agg.SuccessRate24H = rate
			agg.SuccessRate24HOK = true
		}
	}
	items := make([]*adminplusdomain.SiteDiscoveryItem, 0, len(order))
	for _, key := range order {
		agg := aggregates[key]
		if agg == nil {
			continue
		}
		items = append(items, kanLLMDiscoveryItem(sourceURL, payload.GeneratedAt, agg))
	}
	return items
}

func kanLLMDiscoveryItem(sourceURL string, generatedAt string, agg *kanLLMSiteAggregate) *adminplusdomain.SiteDiscoveryItem {
	description := "KanLLM 监测索引"
	if len(agg.PlanTypes) > 0 {
		description += "；分组：" + strings.Join(agg.PlanTypes, "、")
	}
	if agg.MinRate > 0 {
		description += "；最低倍率 " + trimTrailingZeros(agg.MinRate)
	}
	if agg.TotalCount > 0 {
		description += "；可用 " + stringFromInt64(int64(agg.AvailableCount)) + "/" + stringFromInt64(int64(agg.TotalCount))
	}
	raw := map[string]any{
		"source_kind":       "kanllm_summary",
		"generated_at":      generatedAt,
		"api_ids":           append([]string(nil), agg.APIIDs...),
		"plan_types":        append([]string(nil), agg.PlanTypes...),
		"min_rate":          agg.MinRate,
		"enabled_count":     agg.EnabledCount,
		"available_count":   agg.AvailableCount,
		"total_count":       agg.TotalCount,
		"is_self_purchased": agg.IsSelfPurchased,
		"latest_checked_at": agg.LatestCheckedAt,
	}
	if agg.SuccessRate24HOK {
		raw["success_rate_24h"] = agg.SuccessRate24H
	}
	if agg.LastErrorType != "" {
		raw["last_error_type"] = agg.LastErrorType
		raw["last_error_message"] = agg.LastErrorMessage
	}
	return &adminplusdomain.SiteDiscoveryItem{
		SourceURL:                sourceURL,
		SourceSiteID:             agg.Host,
		SourceSection:            "kanllm",
		SourceCategory:           "monitor-summary",
		Name:                     trimLimit(firstNonEmpty(agg.Name, agg.Host), 120),
		RegisterURL:              agg.RegisterURL,
		DashboardURL:             agg.Origin,
		APIBaseURL:               agg.Origin,
		Host:                     agg.Host,
		DomainHint:               agg.Host,
		Description:              trimLimit(description, 1000),
		ClassificationStatus:     adminplusdomain.SiteDiscoveryClassificationUnknown,
		ClassificationConfidence: 0,
		ImportStatus:             adminplusdomain.SiteDiscoveryImportNew,
		ProcessStatus:            adminplusdomain.SiteDiscoveryProcessUnprocessed,
		RawPayload:               raw,
	}
}

func parseDirectSiteItem(sourceURL string, body string) *adminplusdomain.SiteDiscoveryItem {
	registerURL, origin, host, ok := normalizeDiscoverySiteURL(sourceURL)
	if !ok {
		return nil
	}
	name := firstNonEmpty(htmlTitle(body), host)
	return &adminplusdomain.SiteDiscoveryItem{
		SourceURL:                sourceURL,
		SourceSiteID:             host,
		SourceSection:            "direct-url",
		Name:                     trimLimit(name, 120),
		RegisterURL:              registerURL,
		DashboardURL:             origin,
		APIBaseURL:               origin,
		Host:                     host,
		DomainHint:               host,
		ClassificationStatus:     adminplusdomain.SiteDiscoveryClassificationUnknown,
		ClassificationConfidence: 0,
		ImportStatus:             adminplusdomain.SiteDiscoveryImportNew,
		ProcessStatus:            adminplusdomain.SiteDiscoveryProcessUnprocessed,
		RawPayload: map[string]any{
			"source_kind": "direct_url",
		},
	}
}

func kanLLMSummaryURL(sourceURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(sourceURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ""
	}
	if strings.TrimRight(parsed.Path, "/") == "/data/summary.json" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host + "/data/summary.json"
}

func normalizeDiscoverySiteURL(raw string) (string, string, string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", "", "", false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", "", "", false
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Fragment = ""
	parsed.RawQuery = ""
	if parsed.Path == "/" {
		parsed.Path = ""
	}
	registerURL := strings.TrimRight(parsed.String(), "/")
	origin := parsed.Scheme + "://" + parsed.Host
	return registerURL, origin, parsed.Host, true
}

func htmlTitle(body string) string {
	root, err := html.Parse(bytes.NewBufferString(body))
	if err != nil {
		return ""
	}
	var title string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n == nil || title != "" {
			return
		}
		if n.Type == html.ElementNode && n.Data == "title" {
			title = nodeText(n)
			return
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return strings.TrimSpace(title)
}

type classificationResult struct {
	ProviderType adminplusdomain.SupplierType
	Status       adminplusdomain.SiteDiscoveryClassificationStatus
	Confidence   float64
	Evidence     []string
}

func (r classificationResult) Merge(other classificationResult) classificationResult {
	if r.Status == "" && other.Status != "" {
		return other
	}
	if other.Confidence > r.Confidence {
		return other
	}
	if other.Confidence == r.Confidence && r.ProviderType == "" && other.ProviderType != "" {
		return other
	}
	return r
}

func classifyItem(item *adminplusdomain.SiteDiscoveryItem) classificationResult {
	text := strings.Join([]string{item.Name, item.Description, item.RegisterURL, item.DomainHint}, " ")
	result := classifyText(text, "source")
	if result.Status == "" {
		result.Status = adminplusdomain.SiteDiscoveryClassificationUnknown
	}
	return result
}

func classifyText(text string, source string) classificationResult {
	lower := strings.ToLower(text)
	var newScore float64
	var subScore float64
	evidence := make([]string, 0, 4)
	add := func(score *float64, value float64, marker string) {
		if value > *score {
			*score = value
		}
		evidence = append(evidence, source+":"+marker)
	}
	for _, marker := range []string{"new-api-user", "new_api_user", "/api/user/self", "quota_per_unit", "new api", "new-api", "newapi", "one-api", "one api"} {
		if strings.Contains(lower, marker) {
			add(&newScore, 0.85, marker)
		}
	}
	for _, marker := range []string{"sub2api", "sub2 api", "sub2-api", "sub2api-admin", "subapi", "sub api", "sub-api", "subapi-admin"} {
		if strings.Contains(lower, marker) {
			add(&subScore, 0.9, marker)
		}
	}
	if strings.Contains(lower, "new api 模板") || strings.Contains(lower, "new-api 模板") {
		add(&newScore, 0.92, "new_api_template")
	}
	if newScore == 0 && subScore == 0 {
		return classificationResult{
			Status:     adminplusdomain.SiteDiscoveryClassificationUnknown,
			Confidence: 0,
		}
	}
	if newScore >= subScore {
		return classificationResult{
			ProviderType: adminplusdomain.SupplierTypeNewAPI,
			Status:       adminplusdomain.SiteDiscoveryClassificationSupported,
			Confidence:   clamp01(newScore),
			Evidence:     uniqueStrings(evidence),
		}
	}
	return classificationResult{
		ProviderType: adminplusdomain.SupplierTypeSub2API,
		Status:       adminplusdomain.SiteDiscoveryClassificationSupported,
		Confidence:   clamp01(subScore),
		Evidence:     uniqueStrings(evidence),
	}
}

func applyMonitor(item *adminplusdomain.SiteDiscoveryItem, row map[string]any) {
	latest := mapValue(row, "latest_check")
	if len(latest) > 0 {
		available := intFromAny(latest["is_available"]) == 1
		item.MonitorAvailable = &available
		latestMS := intFromAny(latest["response_time_ms"])
		if latestMS > 0 {
			item.MonitorLatestResponseMS = &latestMS
		}
		if available {
			item.MonitorStatus = "online"
		} else {
			item.MonitorStatus = "offline"
		}
	}
	if uptime := floatFromAny(row["uptime_percentage"]); uptime > 0 {
		item.MonitorUptimePercent = &uptime
	}
	if avg := intFromAny(row["avg_response_time"]); avg > 0 {
		item.MonitorAvgResponseMS = &avg
	}
}

func normalizeSettings(settings *adminplusdomain.SiteDiscoverySettings, err error) (*adminplusdomain.SiteDiscoverySettings, error) {
	if err != nil {
		return nil, err
	}
	if settings == nil {
		now := time.Now().UTC()
		return &adminplusdomain.SiteDiscoverySettings{LowRateThreshold: defaultLowRateThreshold, UpdatedAt: now}, nil
	}
	if settings.LowRateThreshold <= 0 {
		settings.LowRateThreshold = defaultLowRateThreshold
	}
	return settings, nil
}

func registrationScheduleKey(discoveryID int64, createdAt time.Time) string {
	return "site-discovery-register:" + stringFromInt64(discoveryID) + ":" + stringFromInt64(createdAt.UnixNano())
}

func generateRegistrationPassword(providerType adminplusdomain.SupplierType) (string, error) {
	length := defaultPasswordLength
	if providerType == adminplusdomain.SupplierTypeSub2API {
		length = 18
	}
	lower := "abcdefghijkmnopqrstuvwxyz"
	upper := "ABCDEFGHJKLMNPQRSTUVWXYZ"
	digits := "23456789"
	special := "!@#_-"
	all := lower + upper + digits + special
	chars := []byte{
		randomChar(lower),
		randomChar(upper),
		randomChar(digits),
		randomChar(special),
	}
	for len(chars) < length {
		chars = append(chars, randomChar(all))
	}
	for i := len(chars) - 1; i > 0; i-- {
		jRaw, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return "", err
		}
		j := int(jRaw.Int64())
		chars[i], chars[j] = chars[j], chars[i]
	}
	return string(chars), nil
}

func randomChar(chars string) byte {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
	if err != nil {
		return chars[0]
	}
	return chars[n.Int64()]
}

func normalizeAbsoluteURL(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", badRequest("SITE_DISCOVERY_URL_INVALID", "url must be a valid absolute url")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", badRequest("SITE_DISCOVERY_URL_SCHEME_INVALID", "url must use http or https")
	}
	return parsed.String(), nil
}

func resolveURL(base string, raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return strings.TrimSpace(raw)
	}
	if parsed.IsAbs() {
		return parsed.String()
	}
	baseURL, err := url.Parse(base)
	if err != nil {
		return parsed.String()
	}
	return baseURL.ResolveReference(parsed).String()
}

func firstTextByClass(root *html.Node, className string) string {
	var out string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n == nil || out != "" {
			return
		}
		if n.Type == html.ElementNode && hasClass(n, className) {
			out = nodeText(n)
			return
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return out
}

func nodeText(n *html.Node) string {
	var parts []string
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node == nil {
			return
		}
		if node.Type == html.TextNode {
			if text := strings.TrimSpace(node.Data); text != "" {
				parts = append(parts, text)
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(n)
	return strings.Join(parts, " ")
}

func attr(n *html.Node, name string) string {
	for _, attr := range n.Attr {
		if attr.Key == name {
			return attr.Val
		}
	}
	return ""
}

func hasClass(n *html.Node, className string) bool {
	for _, item := range strings.Fields(attr(n, "class")) {
		if item == className {
			return true
		}
	}
	return false
}

func mapValue(value map[string]any, key string) map[string]any {
	raw, _ := value[key].(map[string]any)
	return raw
}

func boolValue(value map[string]any, key string) bool {
	raw, _ := value[key].(bool)
	return raw
}

func intFromAny(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case json.Number:
		n, _ := v.Int64()
		return int(n)
	default:
		return 0
	}
}

func int64FromAny(value any) int64 {
	switch v := value.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	case json.Number:
		n, _ := v.Int64()
		return n
	default:
		return 0
	}
}

func floatFromAny(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case json.Number:
		n, _ := v.Float64()
		return n
	default:
		return 0
	}
}

func boolFromAny(value any) bool {
	v, _ := value.(bool)
	return v
}

func stringFromAny(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		return stringFromInt64(int64(v))
	case int:
		return stringFromInt64(int64(v))
	case int64:
		return stringFromInt64(v)
	default:
		return ""
	}
}

func stringFromInt64(value int64) string {
	return strings.TrimSpace(big.NewInt(value).String())
}

func trimTrailingZeros(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if text := strings.TrimSpace(value); text != "" {
			return text
		}
	}
	return ""
}

func trimLimit(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}

func clamp01(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
