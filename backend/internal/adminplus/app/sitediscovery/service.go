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
	"strings"
	"sync"
	"time"

	extensionapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/extension"
	suppliersapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/suppliers"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"golang.org/x/net/html"
)

const (
	DefaultSourceURL           = "https://api.daheiai.com/"
	defaultLowRateThreshold    = 0.8
	defaultDiscoveryFetchLimit = 8 << 20
	defaultSiteProbeLimit      = 512 << 10
	defaultSiteProbeWorkers    = 48
	defaultInterfaceProbeTTL   = 3 * time.Second
	defaultEndpointProbeTTL    = 1400 * time.Millisecond
	defaultPageProbeTTL        = 2500 * time.Millisecond
	defaultPasswordLength      = 20
)

type CredentialCipher interface {
	Encrypt(plaintext string) (string, error)
	Decrypt(ciphertext string) (string, error)
}

type RunInput struct {
	SourceURL       string
	ProbeInterfaces bool
	ProbeSites      bool
	Limit           int
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

type Repository interface {
	GetSettings(ctx context.Context) (*adminplusdomain.SiteDiscoverySettings, error)
	UpdateSettings(ctx context.Context, settings adminplusdomain.SiteDiscoverySettings) (*adminplusdomain.SiteDiscoverySettings, error)
	CreateRun(ctx context.Context, run *adminplusdomain.SiteDiscoveryRun) (*adminplusdomain.SiteDiscoveryRun, error)
	UpdateRun(ctx context.Context, run *adminplusdomain.SiteDiscoveryRun) (*adminplusdomain.SiteDiscoveryRun, error)
	FindExistingItem(ctx context.Context, sourceURL string, sourceSiteID string, registerURL string) (*adminplusdomain.SiteDiscoveryItem, error)
	UpsertItem(ctx context.Context, item *adminplusdomain.SiteDiscoveryItem) (*adminplusdomain.SiteDiscoveryItem, error)
	GetItem(ctx context.Context, id int64) (*adminplusdomain.SiteDiscoveryItem, error)
	ListItems(ctx context.Context, filter ListFilter) ([]*adminplusdomain.SiteDiscoveryItem, error)
	LinkSupplier(ctx context.Context, itemID int64, supplierID int64) (*adminplusdomain.SiteDiscoveryItem, error)
	UpsertRegistrationCredential(ctx context.Context, credential *adminplusdomain.SupplierRegistrationCredential) (*adminplusdomain.SupplierRegistrationCredential, error)
	UpdateRegistrationTask(ctx context.Context, credentialID int64, taskID int64, status adminplusdomain.SupplierRegistrationStatus, attemptedAt time.Time) (*adminplusdomain.SupplierRegistrationCredential, error)
	GetRegistrationCredentialByTaskID(ctx context.Context, taskID int64) (*adminplusdomain.SupplierRegistrationCredential, *adminplusdomain.SiteDiscoveryItem, error)
	ListRecommendations(ctx context.Context, threshold float64, limit int) ([]*adminplusdomain.SiteDiscoveryRecommendation, error)
}

type Service struct {
	repo      Repository
	suppliers *suppliersapp.Service
	extension *extensionapp.Service
	cipher    CredentialCipher
	client    *http.Client
	now       func() time.Time
}

func NewService(repo Repository, suppliers *suppliersapp.Service, extension *extensionapp.Service, cipher CredentialCipher, client *http.Client) *Service {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	return &Service{
		repo:      repo,
		suppliers: suppliers,
		extension: extension,
		cipher:    cipher,
		client:    client,
		now:       time.Now,
	}
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
	emitRunProgress(emit, RunProgressEvent{Type: "log", Level: "info", Message: "正在读取采集源：" + sourceURL, Run: run})
	body, err := s.fetchText(ctx, sourceURL, defaultDiscoveryFetchLimit)
	if err != nil {
		return nil, s.failRunWithProgress(ctx, run, err, emit)
	}
	candidates, err := parseDaheiAIItems(sourceURL, body)
	if err != nil {
		return nil, s.failRunWithProgress(ctx, run, err, emit)
	}
	if in.Limit > 0 && in.Limit < len(candidates) {
		candidates = candidates[:in.Limit]
	}
	emitRunProgress(emit, RunProgressEvent{Type: "log", Level: "info", Message: "采集源解析完成，发现 " + stringFromInt64(int64(len(candidates))) + " 个候选网址", Run: run, Total: len(candidates)})
	monitor := s.fetchMonitorData(ctx, sourceURL)
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
	for item := range s.classifyCandidatesStream(classifyCtx, candidates, in.ProbeInterfaces, in.ProbeSites) {
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
	if s == nil || s.repo == nil || s.extension == nil {
		return nil, nil, internalError("site discovery registration dependencies are not configured")
	}
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
	item, err := s.ImportItem(ctx, itemID)
	if err != nil {
		return nil, nil, err
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
		SupplierID:         item.SupplierID,
		Email:              settings.RegistrationEmail,
		PasswordCiphertext: encrypted,
		PasswordConfigured: true,
		Status:             adminplusdomain.SupplierRegistrationStatusQueued,
		CreatedAt:          now,
		UpdatedAt:          now,
	})
	if err != nil {
		return nil, nil, err
	}
	task, err := s.extension.CreateTask(ctx, extensionapp.CreateTaskInput{
		SupplierID:  item.SupplierID,
		Type:        adminplusdomain.ExtensionTaskTypeRegisterSupplier,
		ScheduleKey: registrationScheduleKey(item.ID, now),
		Priority:    50,
		MaxAttempts: 1,
		Payload: map[string]any{
			"discovery_id":     item.ID,
			"registration_id":  credential.ID,
			"register_url":     item.RegisterURL,
			"dashboard_url":    item.DashboardURL,
			"api_base_url":     item.APIBaseURL,
			"provider_type":    string(item.ProviderType),
			"source_site_id":   item.SourceSiteID,
			"requires_manual":  true,
			"password_in_task": false,
		},
	})
	if err != nil {
		return nil, nil, err
	}
	credential, err = s.repo.UpdateRegistrationTask(ctx, credential.ID, task.ID, adminplusdomain.SupplierRegistrationStatusQueued, now)
	if err != nil {
		return nil, nil, err
	}
	return credential, task, nil
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
					s.classifyCandidate(ctx, item, probeInterfaces, probePages)
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

func (s *Service) classifyCandidate(ctx context.Context, item *adminplusdomain.SiteDiscoveryItem, probeInterfaces bool, probePages bool) {
	classification := classifyItem(item).Merge(existingClassification(item))
	if probeInterfaces && classification.Confidence < 0.95 {
		classification = classification.Merge(s.probeKnownProviderInterfaces(ctx, item))
	}
	if probePages && classification.Confidence < 0.95 {
		classification = classification.Merge(s.probeSitePageClassification(ctx, item))
	}
	item.ProviderType = classification.ProviderType
	item.ClassificationStatus = classification.Status
	item.ClassificationConfidence = classification.Confidence
	item.ClassificationEvidence = classification.Evidence
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Sub2API-Admin-Plus-SiteDiscovery/1.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/json;q=0.8,*/*;q=0.5")
	resp, err := s.client.Do(req)
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

func (s *Service) fetchMonitorData(ctx context.Context, sourceURL string) map[string]map[string]any {
	parsed, err := url.Parse(sourceURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil
	}
	monitorURL := parsed.Scheme + "://" + parsed.Host + "/api.php?action=status&days=7"
	body, err := s.fetchText(ctx, monitorURL, defaultSiteProbeLimit)
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

func (s *Service) probeSitePageClassification(ctx context.Context, item *adminplusdomain.SiteDiscoveryItem) classificationResult {
	target := firstNonEmpty(item.RegisterURL, item.DashboardURL, item.APIBaseURL)
	if target == "" {
		return classificationResult{}
	}
	ctx, cancel := context.WithTimeout(ctx, defaultPageProbeTTL)
	defer cancel()
	body, err := s.fetchText(ctx, target, defaultSiteProbeLimit)
	if err != nil {
		return classificationResult{}
	}
	return classifyText(body, "site")
}

func (s *Service) probeKnownProviderInterfaces(ctx context.Context, item *adminplusdomain.SiteDiscoveryItem) classificationResult {
	ctx, cancel := context.WithTimeout(ctx, defaultInterfaceProbeTTL)
	defer cancel()
	for _, origin := range candidateOrigins(item) {
		if ctx.Err() != nil {
			return classificationResult{}
		}
		sub2apiCtx, cancelSub2API := context.WithTimeout(ctx, defaultEndpointProbeTTL)
		result := s.probeSub2APIInterface(sub2apiCtx, origin)
		cancelSub2API()
		if result.Status == adminplusdomain.SiteDiscoveryClassificationSupported {
			return result
		}
		if ctx.Err() != nil {
			return classificationResult{}
		}
		newAPICtx, cancelNewAPI := context.WithTimeout(ctx, defaultEndpointProbeTTL)
		result = s.probeNewAPIInterface(newAPICtx, origin)
		cancelNewAPI()
		if result.Status == adminplusdomain.SiteDiscoveryClassificationSupported {
			return result
		}
	}
	return classificationResult{}
}

func (s *Service) probeSub2APIInterface(ctx context.Context, origin string) classificationResult {
	body, err := s.fetchText(ctx, joinURLPath(origin, "/api/v1/settings/public"), defaultSiteProbeLimit)
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

func (s *Service) probeNewAPIInterface(ctx context.Context, origin string) classificationResult {
	body, err := s.fetchText(ctx, joinURLPath(origin, "/api/status"), defaultSiteProbeLimit)
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
	for _, marker := range []string{"sub2api", "sub2 api", "sub2-api", "sub2api-admin"} {
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
