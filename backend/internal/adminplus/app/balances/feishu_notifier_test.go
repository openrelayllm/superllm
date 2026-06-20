package balances

import (
	"testing"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/stretchr/testify/require"
)

func TestBuildFeishuBalanceText(t *testing.T) {
	oldBalance := int64(3000)
	event := &adminplusdomain.BalanceEvent{
		ID:                       9,
		SupplierID:               7,
		Type:                     adminplusdomain.BalanceEventTypeLowBalance,
		RuntimeStatus:            adminplusdomain.SupplierRuntimeStatusActive,
		OldBalanceCents:          &oldBalance,
		NewBalanceCents:          500,
		LowBalanceThresholdCents: 1000,
		Currency:                 "CNY",
		SwitchEligible:           true,
		CreatedAt:                time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC),
	}
	snapshot := &adminplusdomain.BalanceSnapshot{
		Source:     "chrome",
		CapturedAt: time.Date(2026, 6, 20, 10, 1, 0, 0, time.UTC),
	}

	text := buildFeishuBalanceText(event, snapshot)

	require.Contains(t, text, "余额不足")
	require.Contains(t, text, "供应商ID：7")
	require.Contains(t, text, "余额：5.00 CNY")
	require.Contains(t, text, "低余额阈值：10.00 CNY")
	require.Contains(t, text, "可切换：是")
	require.Contains(t, text, "来源：chrome")
}
