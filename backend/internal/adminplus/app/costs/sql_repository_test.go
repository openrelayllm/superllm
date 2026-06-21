package costs

import (
	"testing"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/stretchr/testify/require"
)

func TestNormalizeCostSnapshotDerivedAmountsUsesActualBalance(t *testing.T) {
	actualBalance := int64(151)
	oldDelta := int64(-52264)
	snapshot := &adminplusdomain.SupplierCostSnapshot{
		CompletedFundingAmountCents: 16000,
		EntitlementAmountCents:      36500,
		UsageCostCents:              85,
		ExpectedBalanceCents:        52415,
		ActualBalanceCents:          &actualBalance,
		BalanceDeltaCents:           &oldDelta,
	}

	normalizeCostSnapshotDerivedAmounts(snapshot)

	require.Equal(t, int64(52349), snapshot.UsageCostCents)
	require.Equal(t, int64(151), snapshot.ExpectedBalanceCents)
	require.NotNil(t, snapshot.BalanceDeltaCents)
	require.Equal(t, int64(0), *snapshot.BalanceDeltaCents)
}

func TestNormalizeCostSnapshotDerivedAmountsKeepsRawUsageWithoutActualBalance(t *testing.T) {
	snapshot := &adminplusdomain.SupplierCostSnapshot{
		CompletedFundingAmountCents: 16000,
		EntitlementAmountCents:      36500,
		UsageCostCents:              85,
		ExpectedBalanceCents:        52415,
	}

	normalizeCostSnapshotDerivedAmounts(snapshot)

	require.Equal(t, int64(85), snapshot.UsageCostCents)
	require.Equal(t, int64(52415), snapshot.ExpectedBalanceCents)
	require.Nil(t, snapshot.BalanceDeltaCents)
}
