package actions

import (
	"context"
	"testing"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/stretchr/testify/require"
)

func TestServiceGenerateSwitchesFromDepletedActiveSupplierToEligibleCandidate(t *testing.T) {
	svc := NewRuleService()

	result, err := svc.Generate(context.Background(), GenerateInput{
		Suppliers: []SupplierSignal{
			{
				SupplierID:         1,
				Name:               "active",
				RuntimeStatus:      adminplusdomain.SupplierRuntimeStatusActive,
				HealthStatus:       adminplusdomain.SupplierHealthStatusNormal,
				BalanceCents:       0,
				EffectiveCostCents: 100,
			},
			{
				SupplierID:         2,
				Name:               "candidate",
				RuntimeStatus:      adminplusdomain.SupplierRuntimeStatusCandidate,
				HealthStatus:       adminplusdomain.SupplierHealthStatusNormal,
				BalanceCents:       5000,
				EffectiveCostCents: 80,
			},
		},
	})

	require.NoError(t, err)
	requireAction(t, result.Items, adminplusdomain.ActionTypePauseSupplier, "active_supplier_depleted")
	switchAction := requireAction(t, result.Items, adminplusdomain.ActionTypeSwitchSupplier, "switch_from_depleted_supplier")
	require.NotNil(t, switchAction.TargetSupplierID)
	require.Equal(t, int64(2), *switchAction.TargetSupplierID)
}

func TestServiceGeneratePausesAndSwitchesFromFailingActiveSupplier(t *testing.T) {
	svc := NewRuleService()

	result, err := svc.Generate(context.Background(), GenerateInput{
		Suppliers: []SupplierSignal{
			{
				SupplierID:         1,
				RuntimeStatus:      adminplusdomain.SupplierRuntimeStatusActive,
				HealthStatus:       adminplusdomain.SupplierHealthStatusNormal,
				BalanceCents:       5000,
				EffectiveCostCents: 100,
			},
			{
				SupplierID:         2,
				RuntimeStatus:      adminplusdomain.SupplierRuntimeStatusCandidate,
				HealthStatus:       adminplusdomain.SupplierHealthStatusNormal,
				BalanceCents:       5000,
				EffectiveCostCents: 90,
			},
		},
		HealthEvents: []*adminplusdomain.HealthEvent{
			{
				SupplierID: 1,
				Type:       adminplusdomain.HealthEventTypeRequestError,
				Status:     adminplusdomain.HealthEventStatusOpen,
			},
		},
	})

	require.NoError(t, err)
	requireAction(t, result.Items, adminplusdomain.ActionTypePauseSupplier, "supplier_request_errors")
	requireAction(t, result.Items, adminplusdomain.ActionTypeSwitchSupplier, "switch_from_failing_supplier")
}

func TestServiceGenerateDegradesSlowOrSaturatedSupplier(t *testing.T) {
	svc := NewRuleService()

	result, err := svc.Generate(context.Background(), GenerateInput{
		Suppliers: []SupplierSignal{
			{
				SupplierID:    1,
				RuntimeStatus: adminplusdomain.SupplierRuntimeStatusActive,
				HealthStatus:  adminplusdomain.SupplierHealthStatusNormal,
				BalanceCents:  5000,
			},
		},
		HealthEvents: []*adminplusdomain.HealthEvent{
			{
				SupplierID: 1,
				Type:       adminplusdomain.HealthEventTypeSlowFirstToken,
				Status:     adminplusdomain.HealthEventStatusOpen,
			},
		},
	})

	require.NoError(t, err)
	requireAction(t, result.Items, adminplusdomain.ActionTypeDegradeSupplier, "supplier_performance_degraded")
}

func requireAction(t *testing.T, items []*adminplusdomain.ActionRecommendation, actionType adminplusdomain.ActionType, reason string) *adminplusdomain.ActionRecommendation {
	t.Helper()
	for _, item := range items {
		if item.Type == actionType && item.ReasonCode == reason {
			return item
		}
	}
	require.Failf(t, "missing action", "type=%s reason=%s items=%v", actionType, reason, items)
	return nil
}
