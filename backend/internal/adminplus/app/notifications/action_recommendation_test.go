package notifications

import (
	"context"
	"testing"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/stretchr/testify/require"
)

func TestServiceDispatchActionRecommendationCreatesSuppressedDeliveryWhenChannelDisabled(t *testing.T) {
	repo := NewMemoryRepository()
	svc := NewService(repo)

	err := svc.DispatchActionRecommendation(context.Background(), &adminplusdomain.ActionRecommendation{
		ID:         11,
		SupplierID: 7,
		Type:       adminplusdomain.ActionTypeRoutingRefill,
		Severity:   adminplusdomain.ActionSeverityCritical,
		Status:     adminplusdomain.ActionStatusOpen,
		ReasonCode: "local_group_routing_refill_required",
		Title:      "Refill empty local routing group",
		Signals:    []string{"local_group_id=1001", "candidate_local_account_id=42"},
	})

	require.NoError(t, err)
	items, err := repo.ListDeliveries(context.Background(), DeliveryFilter{Limit: 10})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "action.routing_capacity", items[0].EventType)
	require.Equal(t, adminplusdomain.NotificationStatusSuppressed, items[0].Status)
	require.Equal(t, "channel_disabled", items[0].LastError)
	require.Equal(t, int64(7), items[0].SupplierID)
	require.Equal(t, int64(11), items[0].Payload["action_recommendation_id"])
	require.Equal(t, "local_group_routing_refill_required", items[0].Payload["reason_code"])
	require.Contains(t, items[0].DedupeKey, "local_group_id=1001")
}

func TestServiceDispatchActionRecommendationSkipsInfoAction(t *testing.T) {
	repo := NewMemoryRepository()
	svc := NewService(repo)

	err := svc.DispatchActionRecommendation(context.Background(), &adminplusdomain.ActionRecommendation{
		ID:         12,
		SupplierID: 7,
		Type:       adminplusdomain.ActionTypeReviewCredential,
		Severity:   adminplusdomain.ActionSeverityInfo,
		Status:     adminplusdomain.ActionStatusOpen,
		ReasonCode: "candidate_purity_risk",
		Title:      "Review candidate purity risk",
	})

	require.NoError(t, err)
	items, err := repo.ListDeliveries(context.Background(), DeliveryFilter{Limit: 10})
	require.NoError(t, err)
	require.Empty(t, items)
}
