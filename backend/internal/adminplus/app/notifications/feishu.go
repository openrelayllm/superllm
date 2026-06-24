package notifications

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	envFeishuWebhookURL    = "ADMIN_PLUS_FEISHU_WEBHOOK_URL"
	envFeishuWebhookSecret = "ADMIN_PLUS_FEISHU_WEBHOOK_SECRET"
	defaultFeishuTimeout   = 10 * time.Second
	DefaultThrottleWindow  = 30 * time.Minute
)

type Feishu struct {
	webhookURL string
	secret     string
	httpClient *http.Client
	repo       Repository
	now        func() time.Time
}

func NewFeishuFromEnv(repo Repository) *Feishu {
	webhookURL := strings.TrimSpace(os.Getenv(envFeishuWebhookURL))
	if webhookURL == "" {
		webhookURL = strings.TrimSpace(os.Getenv("ADMIN_PLUS_FEISHU_BALANCE_WEBHOOK_URL"))
	}
	if webhookURL == "" {
		return nil
	}
	secret := strings.TrimSpace(os.Getenv(envFeishuWebhookSecret))
	if secret == "" {
		secret = strings.TrimSpace(os.Getenv("ADMIN_PLUS_FEISHU_BALANCE_WEBHOOK_SECRET"))
	}
	return &Feishu{
		webhookURL: webhookURL,
		secret:     secret,
		httpClient: &http.Client{Timeout: defaultFeishuTimeout},
		repo:       repo,
		now:        time.Now,
	}
}

type Event struct {
	Type           string
	ID             int64
	SupplierID     int64
	DedupeKey      string
	ThrottleKey    string
	ThrottleWindow time.Duration
	Text           string
}

func (f *Feishu) SendEvent(ctx context.Context, event Event) error {
	if f == nil || strings.TrimSpace(f.webhookURL) == "" {
		return nil
	}
	if strings.TrimSpace(event.Text) == "" {
		return nil
	}
	payload := f.buildPayload(event.Text)
	delivery, created, err := f.createDelivery(ctx, event, payload)
	if err != nil {
		return err
	}
	if !created {
		return nil
	}
	err = f.sendPayload(ctx, payload)
	if err != nil {
		if delivery != nil && f.repo != nil {
			_ = f.repo.MarkDeliveryFailed(ctx, delivery.ID, deliveryErrorMessage(err))
		}
		return err
	}
	if delivery != nil && f.repo != nil {
		return f.repo.MarkDeliverySucceeded(ctx, delivery.ID)
	}
	return nil
}

func (f *Feishu) SendText(ctx context.Context, text string) error {
	return f.SendEvent(ctx, Event{Text: text})
}

func (f *Feishu) createDelivery(ctx context.Context, event Event, payload map[string]any) (*adminplusdomain.NotificationDelivery, bool, error) {
	if f.repo == nil || event.ID <= 0 || strings.TrimSpace(event.Type) == "" {
		return nil, true, nil
	}
	return f.repo.CreateDelivery(ctx, &adminplusdomain.NotificationDelivery{
		Channel:    adminplusdomain.NotificationChannelFeishu,
		EventType:  normalizeEventType(event.Type),
		EventID:    event.ID,
		SupplierID: event.SupplierID,
		DedupeKey:  f.normalizeDedupeKey(event),
		Status:     adminplusdomain.NotificationStatusSending,
		Payload:    payload,
	})
}

func (f *Feishu) sendPayload(ctx context.Context, payload map[string]any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.webhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := f.httpClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return feishuWebhookError(resp.StatusCode, string(raw))
	}
	return nil
}

func (f *Feishu) buildPayload(text string) map[string]any {
	payload := map[string]any{
		"msg_type": "text",
		"content": map[string]any{
			"text": text,
		},
	}
	if strings.TrimSpace(f.secret) != "" {
		timestamp := f.currentTime().Unix()
		payload["timestamp"] = fmt.Sprintf("%d", timestamp)
		payload["sign"] = Sign(timestamp, f.secret)
	}
	return payload
}

func (f *Feishu) currentTime() time.Time {
	if f != nil && f.now != nil {
		return f.now()
	}
	return time.Now()
}

func Sign(timestamp int64, secret string) string {
	stringToSign := fmt.Sprintf("%d\n%s", timestamp, secret)
	mac := hmac.New(sha256.New, []byte(stringToSign))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func normalizeEventType(value string) string {
	v := strings.ToLower(strings.TrimSpace(value))
	if v == "" {
		return "generic"
	}
	if len(v) > 120 {
		return v[:120]
	}
	return v
}

func (f *Feishu) normalizeDedupeKey(event Event) string {
	if strings.TrimSpace(event.DedupeKey) != "" {
		return strings.TrimSpace(event.DedupeKey)
	}
	eventType := normalizeEventType(event.Type)
	throttleKey := strings.TrimSpace(event.ThrottleKey)
	if event.ThrottleWindow > 0 && throttleKey != "" {
		windowSeconds := int64(event.ThrottleWindow.Seconds())
		if windowSeconds <= 0 {
			windowSeconds = int64(DefaultThrottleWindow.Seconds())
		}
		windowStart := f.currentTime().Unix() / windowSeconds * windowSeconds
		return fmt.Sprintf("feishu:%s:%s:%d", eventType, normalizeThrottleKey(throttleKey), windowStart)
	}
	if event.ID <= 0 || eventType == "generic" {
		return ""
	}
	return fmt.Sprintf("feishu:%s:%d", eventType, event.ID)
}

func normalizeThrottleKey(value string) string {
	v := strings.ToLower(strings.TrimSpace(value))
	if v == "" {
		return "generic"
	}
	v = strings.Join(strings.Fields(v), "_")
	if len(v) > 160 {
		return v[:160]
	}
	return v
}

func feishuWebhookError(statusCode int, responseBody string) error {
	message := fmt.Sprintf("飞书 Webhook 返回 HTTP %d", statusCode)
	if hint := feishuStatusHint(statusCode); hint != "" {
		message += "：" + hint
	}
	body := truncateWebhookResponse(responseBody)
	if body != "" {
		message += "；响应：" + body
	}
	httpStatus := http.StatusBadGateway
	if statusCode >= 400 && statusCode < 500 {
		httpStatus = http.StatusBadRequest
	}
	if statusCode == http.StatusTooManyRequests {
		httpStatus = http.StatusTooManyRequests
	}
	if statusCode >= 500 {
		httpStatus = http.StatusServiceUnavailable
	}
	return infraerrors.New(httpStatus, "FEISHU_WEBHOOK_FAILED", message).WithMetadata(map[string]string{
		"upstream_status": strconv.Itoa(statusCode),
	})
}

func feishuStatusHint(statusCode int) string {
	switch statusCode {
	case http.StatusBadRequest:
		return "请求格式或签名参数不被飞书接受"
	case http.StatusUnauthorized, http.StatusForbidden:
		return "请检查机器人签名密钥或机器人权限"
	case http.StatusNotFound:
		return "请检查机器人 Webhook 地址是否完整、有效或已被删除"
	case http.StatusTooManyRequests:
		return "飞书限流，请稍后重试"
	default:
		if statusCode >= 500 {
			return "飞书服务暂时不可用"
		}
		return ""
	}
}

func deliveryErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	if message := infraerrors.Message(err); strings.TrimSpace(message) != "" && message != infraerrors.UnknownMessage {
		return message
	}
	return err.Error()
}

func truncateWebhookResponse(value string) string {
	v := strings.TrimSpace(value)
	if len(v) > 300 {
		return v[:300]
	}
	return v
}
