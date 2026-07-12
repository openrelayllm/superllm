package adminplus

import (
	"context"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const (
	defaultBackupCronExpr       = "30 3 * * *"
	defaultBackupRetainDays     = 5
	defaultBackupRetainCount    = 30
	defaultCleanupSchedule      = "0 2 * * *"
	defaultCleanupRetentionDays = 5
)

type BackupHandler struct {
	backupService *service.BackupService
	opsService    *service.OpsService
}

type BackupSettingsResponse struct {
	S3       service.BackupS3Config       `json:"s3"`
	Schedule service.BackupScheduleConfig `json:"schedule"`
	Cleanup  historyCleanupSettings       `json:"cleanup"`
}

type backupSettingsRequest struct {
	S3       *service.BackupS3Config       `json:"s3"`
	Schedule *service.BackupScheduleConfig `json:"schedule"`
	Cleanup  *historyCleanupSettings       `json:"cleanup"`
}

type createBackupRequest struct {
	ExpireDays int `json:"expire_days"`
}

type restoreBackupRequest struct {
	Confirmation string `json:"confirmation"`
}

type backupStatusResponse struct {
	StorageConfigured bool                         `json:"storage_configured"`
	StorageProvider   string                       `json:"storage_provider"`
	Schedule          service.BackupScheduleConfig `json:"schedule"`
	LatestSuccess     *service.BackupRecord        `json:"latest_success,omitempty"`
	LatestFailure     *service.BackupRecord        `json:"latest_failure,omitempty"`
	Running           *service.BackupRecord        `json:"running,omitempty"`
	Cleanup           historyCleanupSettings       `json:"cleanup"`
}

type historyCleanupSettings struct {
	Enabled     bool   `json:"enabled"`
	RetainDays  int    `json:"retain_days"`
	CronExpr    string `json:"cron_expr"`
	Description string `json:"description,omitempty"`
}

func NewBackupHandler(
	backupService *service.BackupService,
	opsService *service.OpsService,
) *BackupHandler {
	return &BackupHandler{
		backupService: backupService,
		opsService:    opsService,
	}
}

func (h *BackupHandler) Status(c *gin.Context) {
	settings, err := h.settings(c.Request.Context())
	if response.ErrorFrom(c, err) {
		return
	}
	records, err := h.backupService.ListBackups(c.Request.Context())
	if response.ErrorFrom(c, err) {
		return
	}

	out := backupStatusResponse{
		StorageConfigured: isBackupStorageConfigured(settings.S3),
		StorageProvider:   normalizeStorageProvider(settings.S3.Provider),
		Schedule:          settings.Schedule,
		Cleanup:           settings.Cleanup,
	}
	for i := range records {
		record := records[i]
		switch record.Status {
		case "completed":
			if out.LatestSuccess == nil {
				out.LatestSuccess = &record
			}
		case "failed":
			if out.LatestFailure == nil {
				out.LatestFailure = &record
			}
		case "running":
			if out.Running == nil {
				out.Running = &record
			}
		}
	}
	response.Success(c, out)
}

func (h *BackupHandler) Settings(c *gin.Context) {
	settings, err := h.settings(c.Request.Context())
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, settings)
}

func (h *BackupHandler) UpdateSettings(c *gin.Context) {
	var req backupSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	ctx := c.Request.Context()
	if req.S3 != nil {
		normalizeS3Config(req.S3)
		if _, err := h.backupService.UpdateS3Config(ctx, *req.S3); response.ErrorFrom(c, err) {
			return
		}
	}
	if req.Schedule != nil {
		normalizeSchedule(req.Schedule, false)
		if _, err := h.backupService.UpdateSchedule(ctx, *req.Schedule); response.ErrorFrom(c, err) {
			return
		}
	}
	if req.Cleanup != nil {
		normalizeCleanupSettings(req.Cleanup)
		if err := h.saveCleanupSettings(ctx, *req.Cleanup); response.ErrorFrom(c, err) {
			return
		}
	}
	updated, err := h.settings(ctx)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, updated)
}

func (h *BackupHandler) TestStorage(c *gin.Context) {
	var cfg service.BackupS3Config
	if err := c.ShouldBindJSON(&cfg); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	normalizeS3Config(&cfg)
	if err := h.backupService.TestS3Connection(c.Request.Context(), cfg); response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, gin.H{"ok": true})
}

func (h *BackupHandler) CreateBackup(c *gin.Context) {
	var req createBackupRequest
	_ = c.ShouldBindJSON(&req)
	if req.ExpireDays < 0 {
		response.BadRequest(c, "expire_days cannot be negative")
		return
	}
	if req.ExpireDays == 0 {
		schedule, err := h.backupService.GetSchedule(c.Request.Context())
		if response.ErrorFrom(c, err) {
			return
		}
		req.ExpireDays = schedule.RetainDays
		if req.ExpireDays <= 0 {
			req.ExpireDays = defaultBackupRetainDays
		}
	}
	record, err := h.backupService.StartBackup(c.Request.Context(), "manual", req.ExpireDays)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Accepted(c, record)
}

func (h *BackupHandler) ListBackups(c *gin.Context) {
	records, err := h.backupService.ListBackups(c.Request.Context())
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, records)
}

func (h *BackupHandler) GetBackup(c *gin.Context) {
	record, err := h.backupService.GetBackupRecord(c.Request.Context(), c.Param("id"))
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, record)
}

func (h *BackupHandler) RestoreBackup(c *gin.Context) {
	var req restoreBackupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	if strings.TrimSpace(req.Confirmation) != "RESTORE" {
		response.BadRequest(c, "confirmation must be RESTORE")
		return
	}
	record, err := h.backupService.StartRestore(c.Request.Context(), c.Param("id"))
	if response.ErrorFrom(c, err) {
		return
	}
	response.Accepted(c, record)
}

func (h *BackupHandler) DownloadURL(c *gin.Context) {
	url, err := h.backupService.GetBackupDownloadURL(c.Request.Context(), c.Param("id"))
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, gin.H{"url": url})
}

func (h *BackupHandler) DeleteBackup(c *gin.Context) {
	if err := h.backupService.DeleteBackup(c.Request.Context(), c.Param("id")); response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (h *BackupHandler) settings(ctx context.Context) (*BackupSettingsResponse, error) {
	s3, err := h.backupService.GetS3Config(ctx)
	if err != nil {
		return nil, err
	}
	normalizeS3Config(s3)
	schedule, err := h.backupService.GetSchedule(ctx)
	if err != nil {
		return nil, err
	}
	normalizeSchedule(schedule, true)
	cleanup, err := h.cleanupSettings(ctx)
	if err != nil {
		return nil, err
	}
	return &BackupSettingsResponse{
		S3:       *s3,
		Schedule: *schedule,
		Cleanup:  cleanup,
	}, nil
}

func (h *BackupHandler) cleanupSettings(ctx context.Context) (historyCleanupSettings, error) {
	out := historyCleanupSettings{
		Enabled:     true,
		RetainDays:  defaultCleanupRetentionDays,
		CronExpr:    defaultCleanupSchedule,
		Description: "清理 5 天以前的运维历史、通知投递和聚合指标；不清理用户、账单、用量明细等业务事实。",
	}
	if h == nil || h.opsService == nil {
		return out, nil
	}
	adv, err := h.opsService.GetOpsAdvancedSettings(ctx)
	if err != nil {
		return out, err
	}
	if adv == nil {
		return out, nil
	}
	out.Enabled = adv.DataRetention.CleanupEnabled
	out.RetainDays = adv.DataRetention.ErrorLogRetentionDays
	out.CronExpr = adv.DataRetention.CleanupSchedule
	if out.RetainDays <= 0 {
		out.RetainDays = defaultCleanupRetentionDays
	}
	if strings.TrimSpace(out.CronExpr) == "" {
		out.CronExpr = defaultCleanupSchedule
	}
	return out, nil
}

func (h *BackupHandler) saveCleanupSettings(ctx context.Context, cfg historyCleanupSettings) error {
	if h == nil || h.opsService == nil {
		return nil
	}
	normalizeCleanupSettings(&cfg)
	adv, err := h.opsService.GetOpsAdvancedSettings(ctx)
	if err != nil {
		return err
	}
	if adv == nil {
		adv = &service.OpsAdvancedSettings{}
	}
	adv.DataRetention.CleanupEnabled = cfg.Enabled
	adv.DataRetention.CleanupSchedule = cfg.CronExpr
	adv.DataRetention.ErrorLogRetentionDays = cfg.RetainDays
	adv.DataRetention.MinuteMetricsRetentionDays = cfg.RetainDays
	adv.DataRetention.HourlyMetricsRetentionDays = cfg.RetainDays
	_, err = h.opsService.UpdateOpsAdvancedSettings(ctx, adv)
	return err
}

func normalizeS3Config(cfg *service.BackupS3Config) {
	if cfg == nil {
		return
	}
	cfg.Provider = normalizeStorageProvider(cfg.Provider)
	cfg.Endpoint = strings.TrimSpace(cfg.Endpoint)
	cfg.Region = strings.TrimSpace(cfg.Region)
	cfg.Bucket = strings.TrimSpace(cfg.Bucket)
	cfg.AccessKeyID = strings.TrimSpace(cfg.AccessKeyID)
	cfg.SecretAccessKey = strings.TrimSpace(cfg.SecretAccessKey)
	cfg.Prefix = strings.Trim(strings.TrimSpace(cfg.Prefix), "/")
	if cfg.Provider == "cloudflare_r2" && cfg.Region == "" {
		cfg.Region = "auto"
	}
	if cfg.Prefix == "" {
		cfg.Prefix = "backups"
	}
}

func normalizeSchedule(cfg *service.BackupScheduleConfig, defaultEnabled bool) {
	if cfg == nil {
		return
	}
	emptyConfig := !cfg.Enabled && strings.TrimSpace(cfg.CronExpr) == "" && cfg.RetainDays == 0 && cfg.RetainCount == 0
	if defaultEnabled && emptyConfig {
		cfg.Enabled = true
	}
	cfg.CronExpr = strings.TrimSpace(cfg.CronExpr)
	if cfg.CronExpr == "" {
		cfg.CronExpr = defaultBackupCronExpr
	}
	if cfg.RetainDays <= 0 {
		cfg.RetainDays = defaultBackupRetainDays
	}
	if cfg.RetainCount <= 0 {
		cfg.RetainCount = defaultBackupRetainCount
	}
}

func normalizeCleanupSettings(cfg *historyCleanupSettings) {
	if cfg == nil {
		return
	}
	if cfg.RetainDays <= 0 {
		cfg.RetainDays = defaultCleanupRetentionDays
	}
	if cfg.RetainDays > 365 {
		cfg.RetainDays = 365
	}
	cfg.CronExpr = strings.TrimSpace(cfg.CronExpr)
	if cfg.CronExpr == "" {
		cfg.CronExpr = defaultCleanupSchedule
	}
}

func normalizeStorageProvider(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "cloudflare", "cloudflare_r2", "r2":
		return "cloudflare_r2"
	case "aliyun", "aliyun_oss", "oss":
		return "aliyun_oss"
	case "s3", "aws_s3", "compatible_s3":
		return "s3"
	default:
		return "cloudflare_r2"
	}
}

func isBackupStorageConfigured(cfg service.BackupS3Config) bool {
	return strings.TrimSpace(cfg.Bucket) != "" &&
		strings.TrimSpace(cfg.AccessKeyID) != "" &&
		cfg.SecretConfigured
}
