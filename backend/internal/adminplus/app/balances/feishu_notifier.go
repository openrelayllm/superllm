package balances

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/adminplus/app/notifications"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
)

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

func (n *FeishuNotifier) NotifyBalanceEvent(ctx context.Context, event *adminplusdomain.BalanceEvent, snapshot *adminplusdomain.BalanceSnapshot) error {
	if n == nil || n.service == nil || event == nil {
		return nil
	}
	return n.service.Dispatch(ctx, notifications.DispatchInput{
		Type:       "balance." + string(event.Type),
		ID:         event.ID,
		SupplierID: event.SupplierID,
		Text:       buildFeishuBalanceText(event, snapshot),
	})
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
		"【SuperLLM 余额通知】\n事件：%s\n供应商ID：%d\n余额：%s\n上次余额：%s\n低余额阈值：%s\n运行状态：%s\n可切换：%s\n来源：%s\n时间：%s",
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
