package suppliergroups

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/adminplus/app/notifications"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
)

const (
	groupEventTypePriceIncrease = "supplier_group.price_increase"
	groupEventTypeSuperLowRate  = "supplier_group.super_low_rate"
)

type FeishuNotifier struct {
	service *notifications.Service
}

func NewFeishuNotifier(service *notifications.Service) *FeishuNotifier {
	if service == nil {
		return nil
	}
	return &FeishuNotifier{service: service}
}

func (n *FeishuNotifier) NotifyGroupChange(ctx context.Context, event *adminplusdomain.SupplierGroupChangeEvent) error {
	if n == nil || n.service == nil || event == nil {
		return nil
	}
	settings := n.service.Settings(ctx)
	policy := settings.SupplierGroup
	if !policy.Enabled || !isOpenAIGroupEvent(event) {
		return nil
	}

	eventType := ""
	switch event.Direction {
	case adminplusdomain.SupplierGroupChangeDirectionIncrease:
		if event.NewEffectiveRateMultiplier > policy.OpenAIPriceIncreaseRate {
			eventType = groupEventTypePriceIncrease
		}
	case adminplusdomain.SupplierGroupChangeDirectionNew, adminplusdomain.SupplierGroupChangeDirectionDecrease:
		if crossesSuperLowRateThreshold(event, policy.OpenAISuperLowRateThreshold) {
			eventType = groupEventTypeSuperLowRate
		}
	}
	if eventType == "" {
		return nil
	}
	return n.service.Dispatch(ctx, notifications.DispatchInput{
		Type:           eventType,
		ID:             event.ID,
		SupplierID:     event.SupplierID,
		ThrottleKey:    fmt.Sprintf("supplier:%d:group:%s:event:%s", event.SupplierID, event.ExternalGroupID, eventType),
		ThrottleWindow: notifications.DefaultThrottleWindow,
		Text:           buildFeishuGroupChangeText(eventType, event, policy),
	})
}

func crossesSuperLowRateThreshold(event *adminplusdomain.SupplierGroupChangeEvent, threshold float64) bool {
	if event == nil || threshold <= 0 || event.NewEffectiveRateMultiplier <= 0 || event.NewEffectiveRateMultiplier >= threshold {
		return false
	}
	return event.OldEffectiveRateMultiplier == nil || *event.OldEffectiveRateMultiplier >= threshold
}

func buildFeishuGroupChangeText(eventType string, event *adminplusdomain.SupplierGroupChangeEvent, policy adminplusdomain.SupplierGroupNotificationSettings) string {
	title := "OpenAI 分组变更"
	thresholdLine := ""
	switch eventType {
	case groupEventTypePriceIncrease:
		title = "OpenAI 分组涨价"
		thresholdLine = fmt.Sprintf("涨价阈值：%s", formatGroupRate(policy.OpenAIPriceIncreaseRate))
	case groupEventTypeSuperLowRate:
		title = "OpenAI 超低价分组"
		thresholdLine = fmt.Sprintf("超低价阈值：%s", formatGroupRate(policy.OpenAISuperLowRateThreshold))
	}
	oldRate := "-"
	if event.OldEffectiveRateMultiplier != nil {
		oldRate = formatGroupRate(*event.OldEffectiveRateMultiplier)
	}
	change := "-"
	if event.ChangePercent != nil {
		change = fmt.Sprintf("%+.1f%%", *event.ChangePercent)
	}
	return fmt.Sprintf(
		"【SuperLLM %s】\n供应商ID：%d\n分组：%s\n分组ID：%s\n平台：%s\n变更：%s\n原倍率：%s\n新倍率：%s\n变化：%s\n%s\n时间：%s",
		title,
		event.SupplierID,
		event.GroupName,
		event.ExternalGroupID,
		firstNonEmpty(event.ProviderFamily, "openai"),
		groupChangeDirectionLabel(event.Direction),
		oldRate,
		formatGroupRate(event.NewEffectiveRateMultiplier),
		change,
		thresholdLine,
		event.CreatedAt.UTC().Format(time.RFC3339),
	)
}

func isOpenAIGroupEvent(event *adminplusdomain.SupplierGroupChangeEvent) bool {
	if event == nil {
		return false
	}
	haystack := strings.ToLower(strings.Join([]string{
		event.ProviderFamily,
		event.GroupName,
		event.ExternalGroupID,
	}, " "))
	return strings.Contains(haystack, "openai") || strings.Contains(haystack, "gpt")
}

func groupChangeDirectionLabel(direction adminplusdomain.SupplierGroupChangeDirection) string {
	switch direction {
	case adminplusdomain.SupplierGroupChangeDirectionNew:
		return "新增"
	case adminplusdomain.SupplierGroupChangeDirectionIncrease:
		return "上调"
	case adminplusdomain.SupplierGroupChangeDirectionDecrease:
		return "下调"
	default:
		return string(direction)
	}
}

func formatGroupRate(value float64) string {
	return fmt.Sprintf("%.4fx", value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
