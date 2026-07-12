package notifications

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	EnvFeishuWebhookURL    = "ADMIN_PLUS_FEISHU_WEBHOOK_URL"
	EnvFeishuWebhookSecret = "ADMIN_PLUS_FEISHU_WEBHOOK_SECRET"
	envLegacyWebhookURL    = "ADMIN_PLUS_FEISHU_BALANCE_WEBHOOK_URL"
	envLegacyWebhookSecret = "ADMIN_PLUS_FEISHU_BALANCE_WEBHOOK_SECRET"
)

type SecretCipher interface {
	Encrypt(plaintext string) (string, error)
	Decrypt(ciphertext string) (string, error)
}

type Service struct {
	repo   Repository
	cipher SecretCipher
	now    func() time.Time
}

type DispatchInput struct {
	Type           string
	ID             int64
	SupplierID     int64
	DedupeKey      string
	ThrottleKey    string
	ThrottleWindow time.Duration
	Text           string
	Payload        map[string]any
}

type TestInput struct {
	Text string `json:"text"`
}

func NewService(repo Repository) *Service {
	return NewServiceWithCipher(repo, nil)
}

func NewServiceWithCipher(repo Repository, cipher SecretCipher) *Service {
	return &Service{repo: repo, cipher: cipher, now: time.Now}
}

func (s *Service) Dispatch(ctx context.Context, in DispatchInput) error {
	if s == nil || s.repo == nil {
		return nil
	}
	if strings.TrimSpace(in.Text) == "" {
		return nil
	}
	settings := s.effectiveSettings(ctx)
	rule := findRule(settings.Rules, normalizeEventType(in.Type))
	if !settings.Feishu.Enabled || !settings.Feishu.WebhookConfigured {
		return s.createSuppressed(ctx, in, rule, "channel_disabled")
	}
	if rule != nil && !rule.Enabled {
		return s.createSuppressed(ctx, in, rule, "rule_disabled")
	}
	event := Event{
		Type:           in.Type,
		ID:             positiveEventID(in.ID, s.currentTime()),
		SupplierID:     in.SupplierID,
		DedupeKey:      in.DedupeKey,
		ThrottleKey:    firstNonEmpty(in.ThrottleKey, throttleKeyForRule(rule, in)),
		ThrottleWindow: throttleWindowForRule(rule, in.ThrottleWindow),
		Text:           in.Text,
	}
	sender := s.feishuSender(settings.Feishu)
	if sender == nil {
		return s.createSuppressed(ctx, in, rule, "channel_disabled")
	}
	sender.repo = s.repo
	return sender.SendEvent(ctx, event)
}

func (s *Service) Settings(ctx context.Context) adminplusdomain.NotificationSettings {
	return s.loadSettings(ctx, true)
}

func (s *Service) effectiveSettings(ctx context.Context) adminplusdomain.NotificationSettings {
	return s.loadSettings(ctx, false)
}

func (s *Service) loadSettings(ctx context.Context, redact bool) adminplusdomain.NotificationSettings {
	defaults := defaultSettings()
	if s == nil || s.repo == nil {
		return withRuntimeChannel(defaults, redact)
	}
	stored, err := s.repo.LoadSettings(ctx)
	if err != nil || stored == nil {
		return withRuntimeChannel(defaults, redact)
	}
	normalized := normalizeSettings(*stored, defaults)
	return withRuntimeChannel(normalized, redact)
}

func (s *Service) UpdateSettings(ctx context.Context, settings adminplusdomain.NotificationSettings) (adminplusdomain.NotificationSettings, error) {
	current, _ := s.repo.LoadSettings(ctx)
	normalized := normalizeSettings(settings, defaultSettings())
	clearWebhook := normalized.Feishu.ClearWebhook
	clearLastTest := false
	if clearWebhook {
		normalized.Feishu.WebhookURL = ""
		normalized.Feishu.WebhookSecret = ""
		normalized.Feishu.WebhookHost = ""
		normalized.Feishu.WebhookConfigured = false
		normalized.Feishu.SecretConfigured = false
		clearLastTestState(&normalized.Feishu)
	} else if current != nil {
		currentNormalized := normalizeSettings(*current, defaultSettings())
		if shouldPreserveSecret(normalized.Feishu.WebhookURL) {
			normalized.Feishu.WebhookURL = currentNormalized.Feishu.WebhookURL
		}
		if strings.TrimSpace(normalized.Feishu.WebhookSecret) == "" {
			normalized.Feishu.WebhookSecret = currentNormalized.Feishu.WebhookSecret
		}
		clearLastTest = s.channelCredentialsChanged(currentNormalized.Feishu, normalized.Feishu)
		if !clearLastTest {
			copyLastTestState(&normalized.Feishu, currentNormalized.Feishu)
		}
	}
	normalized.Feishu.ClearWebhook = false
	if s != nil && s.cipher != nil && strings.TrimSpace(normalized.Feishu.WebhookSecret) != "" && !strings.HasPrefix(normalized.Feishu.WebhookSecret, "enc:") {
		encrypted, err := s.cipher.Encrypt(normalized.Feishu.WebhookSecret)
		if err != nil {
			return adminplusdomain.NotificationSettings{}, err
		}
		normalized.Feishu.WebhookSecret = "enc:" + encrypted
	}
	if clearLastTest {
		clearLastTestState(&normalized.Feishu)
	}
	if strings.TrimSpace(normalized.Feishu.WebhookURL) != "" {
		normalized.Feishu.WebhookConfigured = true
		normalized.Feishu.WebhookHost = webhookHost(normalized.Feishu.WebhookURL)
	}
	normalized.Feishu.SecretConfigured = strings.TrimSpace(normalized.Feishu.WebhookSecret) != ""
	normalized.Feishu.ConfigSource = "database"
	if err := s.repo.SaveSettings(ctx, normalized); err != nil {
		return adminplusdomain.NotificationSettings{}, err
	}
	return s.Settings(ctx), nil
}

func (s *Service) channelCredentialsChanged(current, next adminplusdomain.NotificationChannelSettings) bool {
	if strings.TrimSpace(current.WebhookURL) != strings.TrimSpace(next.WebhookURL) {
		return true
	}
	return s.secretForCompare(current.WebhookSecret) != s.secretForCompare(next.WebhookSecret)
}

func (s *Service) secretForCompare(secret string) string {
	secret = strings.TrimSpace(secret)
	if strings.HasPrefix(secret, "enc:") && s != nil && s.cipher != nil {
		if plain, err := s.cipher.Decrypt(strings.TrimPrefix(secret, "enc:")); err == nil {
			return plain
		}
	}
	return secret
}

func clearLastTestState(settings *adminplusdomain.NotificationChannelSettings) {
	if settings == nil {
		return
	}
	settings.LastTestAt = nil
	settings.LastTestStatus = ""
	settings.LastTestError = ""
}

func copyLastTestState(target *adminplusdomain.NotificationChannelSettings, source adminplusdomain.NotificationChannelSettings) {
	if target == nil {
		return
	}
	target.LastTestAt = source.LastTestAt
	target.LastTestStatus = source.LastTestStatus
	target.LastTestError = source.LastTestError
}

func (s *Service) CenterStatus(ctx context.Context) adminplusdomain.NotificationCenterStatus {
	settings := s.Settings(ctx)
	deliveries, _ := s.repo.ListDeliveries(ctx, DeliveryFilter{Limit: 200})
	status := adminplusdomain.NotificationCenterStatus{
		FeishuConfigured: settings.Feishu.WebhookConfigured,
		FeishuEnabled:    settings.Feishu.Enabled,
		TotalRules:       len(settings.Rules),
	}
	for _, rule := range settings.Rules {
		if rule.Enabled {
			status.OpenRules++
		}
	}
	for _, item := range deliveries {
		status.TotalDeliveries++
		if status.LastDeliveryAt == nil || item.CreatedAt.After(*status.LastDeliveryAt) {
			t := item.CreatedAt
			status.LastDeliveryAt = &t
		}
		switch item.Status {
		case adminplusdomain.NotificationStatusSucceeded:
			status.Succeeded++
		case adminplusdomain.NotificationStatusFailed:
			status.Failed++
		case adminplusdomain.NotificationStatusSending:
			status.Sending++
		case adminplusdomain.NotificationStatusSuppressed:
			status.Suppressed++
		}
	}
	return status
}

func (s *Service) ListDeliveries(ctx context.Context, filter DeliveryFilter) ([]*adminplusdomain.NotificationDelivery, error) {
	if s == nil || s.repo == nil {
		return []*adminplusdomain.NotificationDelivery{}, nil
	}
	return s.repo.ListDeliveries(ctx, filter)
}

func (s *Service) Test(ctx context.Context, in TestInput) (*adminplusdomain.NotificationDelivery, error) {
	text := strings.TrimSpace(in.Text)
	if text == "" {
		text = "SuperLLM 飞书通知测试"
	}
	now := s.currentTime()
	err := s.Dispatch(ctx, DispatchInput{
		Type:        "system.test",
		ID:          now.Unix(),
		SupplierID:  0,
		ThrottleKey: fmt.Sprintf("system:test:%d", now.Unix()),
		Text:        text,
	})
	if err != nil {
		_ = s.recordLastTest(ctx, now, string(adminplusdomain.NotificationStatusFailed), deliveryErrorMessage(err))
		return nil, err
	}
	items, listErr := s.repo.ListDeliveries(ctx, DeliveryFilter{EventType: "system.test", Limit: 1})
	if listErr != nil || len(items) == 0 {
		_ = s.recordLastTest(ctx, now, string(adminplusdomain.NotificationStatusFailed), "notification delivery was not recorded")
		return nil, listErr
	}
	_ = s.recordLastTest(ctx, now, string(items[0].Status), items[0].LastError)
	return items[0], nil
}

func (s *Service) recordLastTest(ctx context.Context, testedAt time.Time, status string, message string) error {
	if s == nil || s.repo == nil {
		return nil
	}
	stored, err := s.repo.LoadSettings(ctx)
	if err != nil {
		return err
	}
	settings := defaultSettings()
	if stored != nil {
		settings = normalizeSettings(*stored, defaultSettings())
	}
	t := testedAt.UTC()
	settings.Feishu.LastTestAt = &t
	settings.Feishu.LastTestStatus = strings.TrimSpace(status)
	settings.Feishu.LastTestError = truncateError(message)
	return s.repo.SaveSettings(ctx, settings)
}

func (s *Service) RetryDelivery(ctx context.Context, id int64) (*adminplusdomain.NotificationDelivery, error) {
	delivery, err := s.repo.GetDelivery(ctx, id)
	if err != nil {
		return nil, err
	}
	if delivery.Status != adminplusdomain.NotificationStatusFailed {
		return nil, infraerrors.New(http.StatusBadRequest, "NOTIFICATION_DELIVERY_RETRY_NOT_ALLOWED", "only failed notification deliveries can be retried")
	}
	updated, err := s.repo.IncrementDeliveryAttempt(ctx, id)
	if err != nil {
		return nil, err
	}
	text := textFromDelivery(delivery)
	if text == "" {
		text = fmt.Sprintf("重试通知：%s #%d", delivery.EventType, delivery.EventID)
	}
	settings := s.effectiveSettings(ctx)
	sender := s.feishuSender(settings.Feishu)
	if sender == nil {
		_ = s.repo.MarkDeliveryFailed(ctx, id, "feishu webhook is not configured")
		return s.repo.GetDelivery(ctx, id)
	}
	err = sender.sendPayload(ctx, sender.buildPayload(text))
	if err != nil {
		_ = s.repo.MarkDeliveryFailed(ctx, id, deliveryErrorMessage(err))
		return s.repo.GetDelivery(ctx, id)
	}
	if err := s.repo.MarkDeliverySucceeded(ctx, id); err != nil {
		return nil, err
	}
	return s.repo.GetDelivery(ctx, updated.ID)
}

func (s *Service) createSuppressed(ctx context.Context, in DispatchInput, rule *adminplusdomain.NotificationRule, reason string) error {
	payload := map[string]any{
		"msg_type":          "text",
		"content":           map[string]any{"text": in.Text},
		"suppressed_reason": reason,
	}
	if rule != nil {
		payload["rule_id"] = rule.EventType
		payload["severity"] = rule.Severity
		payload["dedupe_scope"] = rule.DedupeScope
	}
	for key, value := range in.Payload {
		payload[key] = value
	}
	_, _, err := s.repo.CreateDelivery(ctx, &adminplusdomain.NotificationDelivery{
		Channel:    adminplusdomain.NotificationChannelFeishu,
		EventType:  normalizeEventType(in.Type),
		EventID:    positiveEventID(in.ID, s.currentTime()),
		SupplierID: in.SupplierID,
		DedupeKey:  suppressDedupeKey(in, rule, reason, s.currentTime()),
		Status:     adminplusdomain.NotificationStatusSuppressed,
		LastError:  reason,
		Payload:    payload,
	})
	return err
}

func (s *Service) feishuSender(settings adminplusdomain.NotificationChannelSettings) *Feishu {
	webhookURL := strings.TrimSpace(settings.WebhookURL)
	secret := strings.TrimSpace(settings.WebhookSecret)
	if strings.HasPrefix(secret, "enc:") && s.cipher != nil {
		if plain, err := s.cipher.Decrypt(strings.TrimPrefix(secret, "enc:")); err == nil {
			secret = plain
		}
	}
	if webhookURL == "" {
		webhookURL = strings.TrimSpace(os.Getenv(EnvFeishuWebhookURL))
	}
	if webhookURL == "" {
		webhookURL = strings.TrimSpace(os.Getenv(envLegacyWebhookURL))
	}
	if secret == "" {
		secret = strings.TrimSpace(os.Getenv(EnvFeishuWebhookSecret))
	}
	if secret == "" {
		secret = strings.TrimSpace(os.Getenv(envLegacyWebhookSecret))
	}
	if webhookURL == "" {
		return nil
	}
	return &Feishu{webhookURL: webhookURL, secret: secret, httpClient: defaultHTTPClient(), now: s.currentTime}
}

func (s *Service) currentTime() time.Time {
	if s != nil && s.now != nil {
		return s.now().UTC()
	}
	return time.Now().UTC()
}

func defaultSettings() adminplusdomain.NotificationSettings {
	return adminplusdomain.NotificationSettings{
		Feishu: adminplusdomain.NotificationChannelSettings{
			Enabled:      true,
			ConfigSource: "database",
		},
		SupplierGroup: adminplusdomain.SupplierGroupNotificationSettings{
			Enabled:                     true,
			OpenAISuperLowRateThreshold: 0.06,
			OpenAIPriceIncreaseRate:     0.1,
		},
		Rules: []adminplusdomain.NotificationRule{
			{EventType: "balance.low_balance", Label: "余额不足", Description: "供应商余额低于阈值", Enabled: true, Severity: "warning", QuietWindowMinutes: 30, DedupeScope: "supplier", NotifyRecovery: true},
			{EventType: "balance.depleted", Label: "余额耗尽", Description: "供应商余额归零", Enabled: true, Severity: "critical", QuietWindowMinutes: 30, DedupeScope: "supplier", NotifyRecovery: true},
			{EventType: "balance.recovered", Label: "余额恢复", Description: "供应商余额从低位恢复", Enabled: true, Severity: "info", QuietWindowMinutes: 30, DedupeScope: "supplier", NotifyRecovery: true},
			{EventType: "health.request_error", Label: "请求异常", Description: "渠道检测或健康探测返回错误", Enabled: true, Severity: "critical", QuietWindowMinutes: 30, DedupeScope: "supplier_model_type"},
			{EventType: "health.slow_first_token", Label: "首 token 慢", Description: "首 token 延迟超过阈值", Enabled: true, Severity: "warning", QuietWindowMinutes: 30, DedupeScope: "supplier_model_type"},
			{EventType: "health.slow_total", Label: "总耗时慢", Description: "总响应耗时超过阈值", Enabled: true, Severity: "warning", QuietWindowMinutes: 30, DedupeScope: "supplier_model_type"},
			{EventType: "health.concurrency_full", Label: "并发耗尽", Description: "供应商并发容量耗尽", Enabled: true, Severity: "warning", QuietWindowMinutes: 30, DedupeScope: "supplier_model_type"},
			{EventType: "rate.new", Label: "新增费率", Description: "供应商新增模型价格项", Enabled: true, Severity: "info", QuietWindowMinutes: 30, DedupeScope: "supplier_model_price"},
			{EventType: "rate.increase", Label: "费率上涨", Description: "供应商模型价格上涨", Enabled: true, Severity: "warning", QuietWindowMinutes: 30, DedupeScope: "supplier_model_price"},
			{EventType: "rate.decrease", Label: "费率下降", Description: "供应商模型价格下降", Enabled: true, Severity: "info", QuietWindowMinutes: 30, DedupeScope: "supplier_model_price"},
			{EventType: "supplier_group.price_increase", Label: "OpenAI 分组涨价", Description: "OpenAI 分组有效倍率高于涨价阈值", Enabled: true, Severity: "critical", QuietWindowMinutes: 30, DedupeScope: "supplier_group"},
			{EventType: "supplier_group.super_low_rate", Label: "OpenAI 超低价分组", Description: "OpenAI 分组有效倍率低于超低价阈值", Enabled: true, Severity: "info", QuietWindowMinutes: 30, DedupeScope: "supplier_group"},
			{EventType: "cost.reconcile_anomaly", Label: "对账异常", Description: "成本台账差异超过阈值", Enabled: true, Severity: "critical", QuietWindowMinutes: 60, DedupeScope: "supplier_period"},
			{EventType: "action.balance_required", Label: "动作：余额处理", Description: "动作建议要求充值、切换或暂停低余额供应商", Enabled: true, Severity: "warning", QuietWindowMinutes: 30, DedupeScope: "action_supplier"},
			{EventType: "action.key_capacity", Label: "动作：Key 配额", Description: "动作建议要求处理第三方 Key 配额或开通阻塞", Enabled: true, Severity: "warning", QuietWindowMinutes: 60, DedupeScope: "action_supplier_group"},
			{EventType: "action.routing_capacity", Label: "动作：本地分组容量", Description: "动作建议要求补池空池或低容量本地分组", Enabled: true, Severity: "critical", QuietWindowMinutes: 15, DedupeScope: "action_local_group"},
			{EventType: "action.local_state", Label: "动作：本地状态", Description: "动作建议要求处理本地账号状态或原后台 drift", Enabled: true, Severity: "warning", QuietWindowMinutes: 30, DedupeScope: "action_local_account"},
			{EventType: "action.channel_failure", Label: "动作：通道失败", Description: "动作建议要求处理通道监控或主动实测失败账号", Enabled: true, Severity: "warning", QuietWindowMinutes: 30, DedupeScope: "action_local_account"},
			{EventType: "action.proxy_review", Label: "动作：代理复核", Description: "动作建议要求复核本地账号绑定代理", Enabled: true, Severity: "warning", QuietWindowMinutes: 30, DedupeScope: "action_local_account"},
			{EventType: "action.purity_review", Label: "动作：纯度复核", Description: "动作建议要求复核模型纯度或能力匹配", Enabled: true, Severity: "warning", QuietWindowMinutes: 60, DedupeScope: "action_local_account"},
			{EventType: "action.credential_review", Label: "动作：凭据复核", Description: "动作建议要求修复供应商或 Key 凭据", Enabled: true, Severity: "warning", QuietWindowMinutes: 60, DedupeScope: "action_supplier"},
			{EventType: "action.cost_reconcile", Label: "动作：成本对账", Description: "动作建议要求处理成本或余额对账差异", Enabled: true, Severity: "critical", QuietWindowMinutes: 60, DedupeScope: "action_supplier"},
			{EventType: "action.profit_review", Label: "动作：利润复核", Description: "动作建议要求处理利润或供应质量风险", Enabled: true, Severity: "warning", QuietWindowMinutes: 60, DedupeScope: "action_model"},
			{EventType: "action.operational_review", Label: "动作：运营复核", Description: "其他关键动作建议", Enabled: true, Severity: "warning", QuietWindowMinutes: 60, DedupeScope: "action_generic"},
			{EventType: "system.backup_succeeded", Label: "备份成功", Description: "数据库备份已成功上传", Enabled: true, Severity: "info", QuietWindowMinutes: 60, DedupeScope: "none"},
			{EventType: "system.backup_failed", Label: "备份失败", Description: "数据库备份失败，需要排查存储或数据库连接", Enabled: true, Severity: "critical", QuietWindowMinutes: 60, DedupeScope: "none"},
			{EventType: "system.restore_completed", Label: "恢复完成", Description: "数据库恢复任务已完成", Enabled: true, Severity: "warning", QuietWindowMinutes: 60, DedupeScope: "none"},
			{EventType: "system.restore_failed", Label: "恢复失败", Description: "数据库恢复任务失败", Enabled: true, Severity: "critical", QuietWindowMinutes: 60, DedupeScope: "none"},
			{EventType: "system.test", Label: "测试通知", Description: "管理员手动测试飞书通道", Enabled: true, Severity: "info", QuietWindowMinutes: 0, DedupeScope: "none"},
		},
	}
}

func normalizeSettings(settings, defaults adminplusdomain.NotificationSettings) adminplusdomain.NotificationSettings {
	out := settings
	if out.Feishu.ConfigSource == "" {
		out.Feishu.ConfigSource = "database"
	}
	if !out.SupplierGroup.Enabled && out.SupplierGroup.OpenAISuperLowRateThreshold <= 0 && out.SupplierGroup.OpenAIPriceIncreaseRate <= 0 {
		out.SupplierGroup.Enabled = defaults.SupplierGroup.Enabled
	}
	if len(out.Rules) == 0 {
		out.Rules = defaults.Rules
	} else {
		out.Rules = mergeRules(defaults.Rules, out.Rules)
	}
	if out.SupplierGroup.OpenAISuperLowRateThreshold <= 0 {
		out.SupplierGroup.OpenAISuperLowRateThreshold = defaults.SupplierGroup.OpenAISuperLowRateThreshold
	}
	if out.SupplierGroup.OpenAIPriceIncreaseRate <= 0 {
		out.SupplierGroup.OpenAIPriceIncreaseRate = defaults.SupplierGroup.OpenAIPriceIncreaseRate
	}
	for i := range out.Rules {
		if out.Rules[i].Severity == "" {
			out.Rules[i].Severity = "warning"
		}
		if out.Rules[i].DedupeScope == "" {
			out.Rules[i].DedupeScope = "supplier"
		}
		if out.Rules[i].QuietWindowMinutes < 0 {
			out.Rules[i].QuietWindowMinutes = 0
		}
	}
	out.Feishu.WebhookHost = webhookHost(out.Feishu.WebhookURL)
	out.Feishu.WebhookConfigured = strings.TrimSpace(out.Feishu.WebhookURL) != ""
	out.Feishu.SecretConfigured = strings.TrimSpace(out.Feishu.WebhookSecret) != ""
	return out
}

func mergeRules(defaults, custom []adminplusdomain.NotificationRule) []adminplusdomain.NotificationRule {
	index := make(map[string]adminplusdomain.NotificationRule, len(custom))
	for _, rule := range custom {
		if deprecatedNotificationRule(rule.EventType) {
			continue
		}
		index[normalizeEventType(rule.EventType)] = rule
	}
	out := make([]adminplusdomain.NotificationRule, 0, len(defaults)+len(custom))
	seen := make(map[string]struct{})
	for _, def := range defaults {
		key := normalizeEventType(def.EventType)
		if customRule, ok := index[key]; ok {
			if customRule.Label == "" {
				customRule.Label = def.Label
			}
			if customRule.Description == "" {
				customRule.Description = def.Description
			}
			out = append(out, customRule)
		} else {
			out = append(out, def)
		}
		seen[key] = struct{}{}
	}
	for _, rule := range custom {
		key := normalizeEventType(rule.EventType)
		if deprecatedNotificationRule(key) {
			continue
		}
		if _, ok := seen[key]; !ok {
			out = append(out, rule)
		}
	}
	return out
}

func deprecatedNotificationRule(eventType string) bool {
	return strings.HasPrefix(normalizeEventType(eventType), "announcement.")
}

func withRuntimeChannel(settings adminplusdomain.NotificationSettings, redact bool) adminplusdomain.NotificationSettings {
	if strings.TrimSpace(settings.Feishu.WebhookURL) == "" {
		if envURL := firstNonEmpty(os.Getenv(EnvFeishuWebhookURL), os.Getenv(envLegacyWebhookURL)); envURL != "" {
			settings.Feishu.WebhookURL = envURL
			settings.Feishu.ConfigSource = "environment"
		}
	}
	if strings.TrimSpace(settings.Feishu.WebhookSecret) == "" {
		settings.Feishu.WebhookSecret = firstNonEmpty(os.Getenv(EnvFeishuWebhookSecret), os.Getenv(envLegacyWebhookSecret))
	}
	settings.Feishu.WebhookConfigured = strings.TrimSpace(settings.Feishu.WebhookURL) != ""
	settings.Feishu.SecretConfigured = strings.TrimSpace(settings.Feishu.WebhookSecret) != ""
	settings.Feishu.WebhookHost = webhookHost(settings.Feishu.WebhookURL)
	if redact {
		settings.Feishu.WebhookURL = maskURL(settings.Feishu.WebhookURL)
		settings.Feishu.WebhookSecret = ""
	}
	return settings
}

func findRule(rules []adminplusdomain.NotificationRule, eventType string) *adminplusdomain.NotificationRule {
	eventType = normalizeEventType(eventType)
	for i := range rules {
		if normalizeEventType(rules[i].EventType) == eventType {
			return &rules[i]
		}
	}
	return nil
}

func throttleWindowForRule(rule *adminplusdomain.NotificationRule, fallback time.Duration) time.Duration {
	if rule != nil {
		return time.Duration(rule.QuietWindowMinutes) * time.Minute
	}
	return fallback
}

func throttleKeyForRule(rule *adminplusdomain.NotificationRule, in DispatchInput) string {
	if rule == nil || rule.QuietWindowMinutes <= 0 {
		return ""
	}
	return fmt.Sprintf("supplier:%d:event:%s:scope:%s", in.SupplierID, normalizeEventType(in.Type), rule.DedupeScope)
}

func suppressDedupeKey(in DispatchInput, rule *adminplusdomain.NotificationRule, reason string, now time.Time) string {
	window := throttleWindowForRule(rule, in.ThrottleWindow)
	key := firstNonEmpty(in.DedupeKey, in.ThrottleKey, throttleKeyForRule(rule, in))
	if window > 0 && key != "" {
		seconds := int64(window.Seconds())
		if seconds <= 0 {
			seconds = int64(DefaultThrottleWindow.Seconds())
		}
		return fmt.Sprintf("feishu:%s:suppressed:%s:%s:%d", normalizeEventType(in.Type), reason, normalizeThrottleKey(key), now.Unix()/seconds*seconds)
	}
	return fmt.Sprintf("feishu:%s:suppressed:%s:%d:%d", normalizeEventType(in.Type), reason, in.ID, now.UnixNano())
}

func textFromDelivery(delivery *adminplusdomain.NotificationDelivery) string {
	if delivery == nil || delivery.Payload == nil {
		return ""
	}
	content, ok := delivery.Payload["content"].(map[string]any)
	if !ok {
		return ""
	}
	text, _ := content["text"].(string)
	return strings.TrimSpace(text)
}

func webhookHost(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	return parsed.Host
}

func maskURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	host := webhookHost(raw)
	if host == "" {
		return "configured"
	}
	return "https://" + host + "/***"
}

func shouldPreserveSecret(value string) bool {
	value = strings.TrimSpace(value)
	return value == "" || strings.Contains(value, "***")
}

func positiveEventID(id int64, now time.Time) int64 {
	if id > 0 {
		return id
	}
	return now.UnixNano()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func defaultHTTPClient() *http.Client {
	return &http.Client{Timeout: defaultFeishuTimeout}
}
