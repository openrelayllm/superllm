package balances

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
)

const (
	envFeishuWebhookURL    = "ADMIN_PLUS_FEISHU_BALANCE_WEBHOOK_URL"
	envFeishuWebhookSecret = "ADMIN_PLUS_FEISHU_BALANCE_WEBHOOK_SECRET"
	defaultFeishuTimeout   = 10 * time.Second
)

type FeishuNotifier struct {
	webhookURL string
	secret     string
	httpClient *http.Client
}

func NewFeishuNotifierFromEnv() *FeishuNotifier {
	webhookURL := strings.TrimSpace(os.Getenv(envFeishuWebhookURL))
	if webhookURL == "" {
		return nil
	}
	return &FeishuNotifier{
		webhookURL: webhookURL,
		secret:     strings.TrimSpace(os.Getenv(envFeishuWebhookSecret)),
		httpClient: &http.Client{Timeout: defaultFeishuTimeout},
	}
}

func (n *FeishuNotifier) NotifyBalanceEvent(ctx context.Context, event *adminplusdomain.BalanceEvent, snapshot *adminplusdomain.BalanceSnapshot) error {
	if n == nil || strings.TrimSpace(n.webhookURL) == "" || event == nil {
		return nil
	}
	payload := n.buildPayload(event, snapshot)
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.webhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := n.httpClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("feishu webhook returned status %d", resp.StatusCode)
	}
	return nil
}

func (n *FeishuNotifier) buildPayload(event *adminplusdomain.BalanceEvent, snapshot *adminplusdomain.BalanceSnapshot) map[string]any {
	text := buildFeishuBalanceText(event, snapshot)
	payload := map[string]any{
		"msg_type": "text",
		"content": map[string]any{
			"text": text,
		},
	}
	if n.secret != "" {
		timestamp := time.Now().Unix()
		payload["timestamp"] = fmt.Sprintf("%d", timestamp)
		payload["sign"] = feishuSign(timestamp, n.secret)
	}
	return payload
}

func buildFeishuBalanceText(event *adminplusdomain.BalanceEvent, snapshot *adminplusdomain.BalanceSnapshot) string {
	eventType := balanceEventLabel(event.Type)
	current := formatBalanceCents(event.NewBalanceCents, event.Currency)
	threshold := formatBalanceCents(event.LowBalanceThresholdCents, event.Currency)
	old := "-"
	if event.OldBalanceCents != nil {
		old = formatBalanceCents(*event.OldBalanceCents, event.Currency)
	}
	switchEligible := "否"
	if event.SwitchEligible {
		switchEligible = "是"
	}
	source := "-"
	capturedAt := event.CreatedAt
	if snapshot != nil {
		source = snapshot.Source
		capturedAt = snapshot.CapturedAt
	}
	return fmt.Sprintf(
		"【Sub2API Admin Plus 余额通知】\n事件：%s\n供应商ID：%d\n余额：%s\n上次余额：%s\n低余额阈值：%s\n运行状态：%s\n可切换：%s\n来源：%s\n时间：%s",
		eventType,
		event.SupplierID,
		current,
		old,
		threshold,
		event.RuntimeStatus,
		switchEligible,
		source,
		capturedAt.Format(time.RFC3339),
	)
}

func balanceEventLabel(eventType adminplusdomain.BalanceEventType) string {
	switch eventType {
	case adminplusdomain.BalanceEventTypeLowBalance:
		return "余额不足"
	case adminplusdomain.BalanceEventTypeDepleted:
		return "余额耗尽"
	case adminplusdomain.BalanceEventTypeRecovered:
		return "余额恢复"
	default:
		return string(eventType)
	}
}

func formatBalanceCents(cents int64, currency string) string {
	return fmt.Sprintf("%.2f %s", float64(cents)/100, strings.ToUpper(strings.TrimSpace(currency)))
}

func feishuSign(timestamp int64, secret string) string {
	stringToSign := fmt.Sprintf("%d\n%s", timestamp, secret)
	mac := hmac.New(sha256.New, []byte(stringToSign))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}
