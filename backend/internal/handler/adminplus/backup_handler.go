package adminplus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	notificationsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/notifications"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const (
	settingKeyAdminPlusServerRenewal = "admin_plus_server_renewal"

	defaultBackupCronExpr       = "30 3 * * *"
	defaultBackupRetainDays     = 5
	defaultBackupRetainCount    = 30
	defaultCleanupSchedule      = "0 2 * * *"
	defaultCleanupRetentionDays = 5
)

type BackupHandler struct {
	backupService       *service.BackupService
	notificationService *notificationsapp.Service
	opsService          *service.OpsService
	settingRepo         service.SettingRepository
	secretEncryptor     service.SecretEncryptor

	renewalLoopOnce sync.Once
}

type BackupSettingsResponse struct {
	S3       service.BackupS3Config       `json:"s3"`
	Schedule service.BackupScheduleConfig `json:"schedule"`
	Renewal  serverRenewalStatus          `json:"renewal"`
	Cleanup  historyCleanupSettings       `json:"cleanup"`
}

type backupSettingsRequest struct {
	S3       *service.BackupS3Config       `json:"s3"`
	Schedule *service.BackupScheduleConfig `json:"schedule"`
	Renewal  *serverRenewalConfig          `json:"renewal"`
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
	Renewal           serverRenewalStatus          `json:"renewal"`
	Cleanup           historyCleanupSettings       `json:"cleanup"`
}

type serverRenewalConfig struct {
	Enabled               bool   `json:"enabled"`
	ServerName            string `json:"server_name"`
	Provider              string `json:"provider"`
	HostID                string `json:"host_id,omitempty"`
	IPAddress             string `json:"ip_address,omitempty"`
	OperatingSystem       string `json:"operating_system,omitempty"`
	SSHUsername           string `json:"ssh_username,omitempty"`
	SSHPassword           string `json:"ssh_password,omitempty"`
	SSHPasswordConfigured bool   `json:"ssh_password_configured,omitempty"`
	SSHPort               int    `json:"ssh_port,omitempty"`
	PanelURL              string `json:"panel_url,omitempty"`
	ExpiresAt             string `json:"expires_at"`
	ExpiresAtTime         string `json:"expires_at_time,omitempty"`
	ReminderDays          []int  `json:"reminder_days"`
	LastNotifiedAt        string `json:"last_notified_at,omitempty"`
	LastNotifiedKey       string `json:"last_notified_key,omitempty"`
}

type serverRenewalStatus struct {
	serverRenewalConfig
	DaysRemaining int    `json:"days_remaining"`
	State         string `json:"state"`
	NextReminder  string `json:"next_reminder,omitempty"`
}

type historyCleanupSettings struct {
	Enabled     bool   `json:"enabled"`
	RetainDays  int    `json:"retain_days"`
	CronExpr    string `json:"cron_expr"`
	Description string `json:"description,omitempty"`
}

func NewBackupHandler(
	backupService *service.BackupService,
	notificationService *notificationsapp.Service,
	opsService *service.OpsService,
	settingRepo service.SettingRepository,
	secretEncryptor service.SecretEncryptor,
) *BackupHandler {
	h := &BackupHandler{
		backupService:       backupService,
		notificationService: notificationService,
		opsService:          opsService,
		settingRepo:         settingRepo,
		secretEncryptor:     secretEncryptor,
	}
	h.startRenewalLoop()
	return h
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
		Renewal:           settings.Renewal,
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
	if req.Renewal != nil {
		normalizeRenewalConfig(req.Renewal)
		if err := h.updateRenewalConfig(ctx, *req.Renewal); response.ErrorFrom(c, err) {
			return
		}
		_ = h.maybeDispatchRenewalReminder(ctx, time.Now())
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

func (h *BackupHandler) GetServerRenewal(c *gin.Context) {
	status, err := h.renewalStatus(c.Request.Context(), time.Now())
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, sanitizeRenewalStatus(status))
}

func (h *BackupHandler) UpdateServerRenewal(c *gin.Context) {
	var cfg serverRenewalConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	normalizeRenewalConfig(&cfg)
	if err := h.updateRenewalConfig(c.Request.Context(), cfg); response.ErrorFrom(c, err) {
		return
	}
	status, err := h.renewalStatus(c.Request.Context(), time.Now())
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, sanitizeRenewalStatus(status))
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
	renewal, err := h.renewalStatus(ctx, time.Now())
	if err != nil {
		return nil, err
	}
	cleanup, err := h.cleanupSettings(ctx)
	if err != nil {
		return nil, err
	}
	return &BackupSettingsResponse{
		S3:       *s3,
		Schedule: *schedule,
		Renewal:  sanitizeRenewalStatus(renewal),
		Cleanup:  cleanup,
	}, nil
}

func (h *BackupHandler) loadRenewalConfig(ctx context.Context) (serverRenewalConfig, error) {
	cfg := serverRenewalConfig{
		Enabled:      true,
		ServerName:   "sub2api-admin-plus",
		Provider:     "unknown",
		ReminderDays: []int{7, 3, 1},
	}
	if h == nil || h.settingRepo == nil {
		return cfg, nil
	}
	raw, err := h.settingRepo.GetValue(ctx, settingKeyAdminPlusServerRenewal)
	if err != nil {
		if errors.Is(err, service.ErrSettingNotFound) {
			return cfg, nil
		}
		return cfg, err
	}
	if strings.TrimSpace(raw) == "" {
		return cfg, nil
	}
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return serverRenewalConfig{}, err
	}
	normalizeRenewalConfig(&cfg)
	return cfg, nil
}

func (h *BackupHandler) saveRenewalConfig(ctx context.Context, cfg serverRenewalConfig) error {
	if h == nil || h.settingRepo == nil {
		return nil
	}
	normalizeRenewalConfig(&cfg)
	raw, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return h.settingRepo.Set(ctx, settingKeyAdminPlusServerRenewal, string(raw))
}

func (h *BackupHandler) updateRenewalConfig(ctx context.Context, cfg serverRenewalConfig) error {
	if h == nil || h.settingRepo == nil {
		return nil
	}
	normalizeRenewalConfig(&cfg)
	old, _ := h.loadRenewalConfig(ctx)
	if strings.TrimSpace(cfg.SSHPassword) == "" {
		cfg.SSHPassword = old.SSHPassword
	} else if h.secretEncryptor != nil {
		encrypted, err := h.secretEncryptor.Encrypt(cfg.SSHPassword)
		if err != nil {
			return fmt.Errorf("encrypt ssh password: %w", err)
		}
		cfg.SSHPassword = encrypted
	}
	cfg.SSHPasswordConfigured = strings.TrimSpace(cfg.SSHPassword) != ""
	cfg.LastNotifiedAt = old.LastNotifiedAt
	cfg.LastNotifiedKey = old.LastNotifiedKey
	return h.saveRenewalConfig(ctx, cfg)
}

func (h *BackupHandler) renewalStatus(ctx context.Context, now time.Time) (serverRenewalStatus, error) {
	cfg, err := h.loadRenewalConfig(ctx)
	if err != nil {
		return serverRenewalStatus{}, err
	}
	status := serverRenewalStatus{serverRenewalConfig: cfg, State: "unconfigured", DaysRemaining: 0}
	expiresAt, ok := parseRenewalDate(cfg.ExpiresAt)
	if !ok {
		return status, nil
	}
	today := startOfLocalDay(now)
	days := int(expiresAt.Sub(today).Hours() / 24)
	status.DaysRemaining = days
	switch {
	case days < 0:
		status.State = "expired"
	case days == 0:
		status.State = "due_today"
	case containsInt(cfg.ReminderDays, days):
		status.State = "reminder_due"
	default:
		status.State = "active"
	}
	for _, day := range cfg.ReminderDays {
		if day >= days {
			status.NextReminder = startOfLocalDay(expiresAt.AddDate(0, 0, -day)).Format("2006-01-02")
			break
		}
	}
	return status, nil
}

func sanitizeRenewalStatus(status serverRenewalStatus) serverRenewalStatus {
	status.SSHPasswordConfigured = strings.TrimSpace(status.SSHPassword) != "" || status.SSHPasswordConfigured
	status.SSHPassword = ""
	return status
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

func (h *BackupHandler) startRenewalLoop() {
	if h == nil || h.notificationService == nil || h.settingRepo == nil {
		return
	}
	h.renewalLoopOnce.Do(func() {
		go func() {
			timer := time.NewTimer(2 * time.Minute)
			defer timer.Stop()
			for {
				<-timer.C
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
				_ = h.maybeDispatchRenewalReminder(ctx, time.Now())
				cancel()
				timer.Reset(12 * time.Hour)
			}
		}()
	})
}

func (h *BackupHandler) maybeDispatchRenewalReminder(ctx context.Context, now time.Time) error {
	if h == nil || h.notificationService == nil {
		return nil
	}
	status, err := h.renewalStatus(ctx, now)
	if err != nil || !status.Enabled {
		return err
	}
	if status.State != "reminder_due" && status.State != "due_today" && status.State != "expired" {
		return nil
	}
	key := fmt.Sprintf("%s:%s:%d", status.ServerName, status.ExpiresAt, status.DaysRemaining)
	if status.LastNotifiedKey == key {
		return nil
	}
	text := fmt.Sprintf("服务器续费提醒：%s（%s）到期日 %s，剩余 %d 天，请及时续费。",
		firstNonEmptyBackupValue(status.ServerName, "服务器"),
		firstNonEmptyBackupValue(status.Provider, "未知服务商"),
		status.ExpiresAt,
		status.DaysRemaining,
	)
	if status.DaysRemaining < 0 {
		text = fmt.Sprintf("服务器续费提醒：%s（%s）已于 %s 到期，请立即处理。",
			firstNonEmptyBackupValue(status.ServerName, "服务器"),
			firstNonEmptyBackupValue(status.Provider, "未知服务商"),
			status.ExpiresAt,
		)
	}
	if err := h.notificationService.Dispatch(ctx, notificationsapp.DispatchInput{
		Type:           "system.server_renewal_due",
		ID:             int64(absInt(status.DaysRemaining)) + now.Unix()/86400,
		DedupeKey:      key,
		ThrottleKey:    key,
		ThrottleWindow: 24 * time.Hour,
		Text:           text,
		Payload: map[string]any{
			"server_name":    status.ServerName,
			"provider":       status.Provider,
			"expires_at":     status.ExpiresAt,
			"days_remaining": status.DaysRemaining,
			"state":          status.State,
		},
	}); err != nil {
		return err
	}
	cfg := status.serverRenewalConfig
	cfg.LastNotifiedKey = key
	cfg.LastNotifiedAt = now.UTC().Format(time.RFC3339)
	return h.saveRenewalConfig(ctx, cfg)
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

func normalizeRenewalConfig(cfg *serverRenewalConfig) {
	if cfg == nil {
		return
	}
	cfg.ServerName = strings.TrimSpace(cfg.ServerName)
	if cfg.ServerName == "" {
		cfg.ServerName = "sub2api-admin-plus"
	}
	cfg.Provider = strings.TrimSpace(cfg.Provider)
	cfg.HostID = strings.TrimSpace(cfg.HostID)
	cfg.IPAddress = strings.TrimSpace(cfg.IPAddress)
	cfg.OperatingSystem = strings.TrimSpace(cfg.OperatingSystem)
	cfg.SSHUsername = strings.TrimSpace(cfg.SSHUsername)
	cfg.SSHPassword = strings.TrimSpace(cfg.SSHPassword)
	cfg.SSHPasswordConfigured = strings.TrimSpace(cfg.SSHPassword) != "" || cfg.SSHPasswordConfigured
	cfg.PanelURL = strings.TrimSpace(cfg.PanelURL)
	if cfg.SSHPort <= 0 {
		cfg.SSHPort = 22
	}
	if cfg.SSHPort > 65535 {
		cfg.SSHPort = 65535
	}
	cfg.ExpiresAt = strings.TrimSpace(cfg.ExpiresAt)
	cfg.ExpiresAtTime = strings.TrimSpace(cfg.ExpiresAtTime)
	if len(cfg.ReminderDays) == 0 {
		cfg.ReminderDays = []int{7, 3, 1}
	}
	seen := map[int]struct{}{}
	days := make([]int, 0, len(cfg.ReminderDays))
	for _, day := range cfg.ReminderDays {
		if day < 0 || day > 365 {
			continue
		}
		if _, ok := seen[day]; ok {
			continue
		}
		seen[day] = struct{}{}
		days = append(days, day)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(days)))
	cfg.ReminderDays = days
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

func parseRenewalDate(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	t, err := time.ParseInLocation("2006-01-02", value, time.Local)
	if err != nil {
		return time.Time{}, false
	}
	return startOfLocalDay(t), true
}

func startOfLocalDay(t time.Time) time.Time {
	local := t.In(time.Local)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, local.Location())
}

func containsInt(values []int, target int) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func firstNonEmptyBackupValue(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
