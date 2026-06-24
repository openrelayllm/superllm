package costs

import (
	"context"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/adminplus/app/notifications"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
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

func (n *FeishuNotifier) NotifyCostReconcileAnomaly(ctx context.Context, snapshot *adminplusdomain.SupplierCostSnapshot) error {
	if n == nil || n.service == nil || snapshot == nil {
		return nil
	}
	return n.service.Dispatch(ctx, notifications.DispatchInput{
		Type:           "cost.reconcile_anomaly",
		ID:             snapshot.ID,
		SupplierID:     snapshot.SupplierID,
		ThrottleKey:    fmt.Sprintf("supplier:%d:currency:%s:cost_reconcile_anomaly", snapshot.SupplierID, normalizeCurrency(snapshot.Currency)),
		ThrottleWindow: time.Hour,
		Text:           buildCostReconcileAnomalyText(snapshot),
	})
}

func buildCostReconcileAnomalyText(snapshot *adminplusdomain.SupplierCostSnapshot) string {
	actualBalance := "-"
	if snapshot.ActualBalanceCents != nil {
		actualBalance = formatCostCents(*snapshot.ActualBalanceCents, snapshot.Currency)
	}
	delta := "-"
	if snapshot.BalanceDeltaCents != nil {
		delta = formatSignedCostCents(*snapshot.BalanceDeltaCents, snapshot.Currency)
	}
	return fmt.Sprintf(
		"【Sub2API Admin Plus 对账异常】\n供应商ID：%d\n币种：%s\n预期余额：%s\n实际余额：%s\n余额差异：%s\n充值合计：%s\n兑换合计：%s\n使用成本：%s\n退款合计：%s\n调整合计：%s\n采集时间：%s",
		snapshot.SupplierID,
		normalizeCurrency(snapshot.Currency),
		formatCostCents(snapshot.ExpectedBalanceCents, snapshot.Currency),
		actualBalance,
		delta,
		formatCostCents(snapshot.CompletedFundingAmountCents, snapshot.Currency),
		formatCostCents(snapshot.EntitlementAmountCents, snapshot.Currency),
		formatCostCents(snapshot.UsageCostCents, snapshot.Currency),
		formatCostCents(snapshot.RefundAmountCents, snapshot.Currency),
		formatSignedCostCents(snapshot.AdjustmentAmountCents, snapshot.Currency),
		snapshot.CapturedAt.Format(time.RFC3339),
	)
}

func formatCostCents(cents int64, currency string) string {
	return fmt.Sprintf("%.2f %s", float64(cents)/100, normalizeCurrency(currency))
}

func formatSignedCostCents(cents int64, currency string) string {
	sign := ""
	if cents > 0 {
		sign = "+"
	}
	return fmt.Sprintf("%s%.2f %s", sign, float64(cents)/100, normalizeCurrency(currency))
}
