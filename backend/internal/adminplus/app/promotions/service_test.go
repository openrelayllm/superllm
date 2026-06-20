package promotions

import (
	"context"
	"net/http"
	"testing"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestServiceRecordPromotionRecommendsRechargeForEmptySupplier(t *testing.T) {
	repo := newFakePromotionRepository()
	notifier := &fakePromotionNotifier{}
	svc := NewServiceWithNotifier(repo, notifier)
	bonus := 20.0

	event, err := svc.RecordPromotion(context.Background(), RecordPromotionInput{
		SupplierID:       7,
		Type:             adminplusdomain.PromotionTypeRechargeBonus,
		Title:            "June recharge bonus",
		MinRechargeCents: 10000,
		BonusPercent:     &bonus,
		RuntimeStatus:    adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		BalanceCents:     0,
		Currency:         "usd",
	})

	require.NoError(t, err)
	require.Equal(t, adminplusdomain.PromotionRecommendationRechargeToUnlock, event.Recommendation)
	require.False(t, event.SwitchEligible)
	require.Equal(t, "USD", event.Currency)
	require.Equal(t, adminplusdomain.PromotionStatusOpen, event.Status)
	require.Len(t, notifier.events, 1)
	require.Equal(t, event.ID, notifier.events[0].ID)
}

func TestServiceRecordPromotionMarksSwitchCandidateWhenBalanceIsUsable(t *testing.T) {
	repo := newFakePromotionRepository()
	svc := NewService(repo)
	discount := 15.0

	event, err := svc.RecordPromotion(context.Background(), RecordPromotionInput{
		SupplierID:      7,
		Type:            adminplusdomain.PromotionTypeRateDiscount,
		Title:           "Lower GPT rate",
		DiscountPercent: &discount,
		RuntimeStatus:   adminplusdomain.SupplierRuntimeStatusCandidate,
		BalanceCents:    3000,
		Currency:        "USD",
	})

	require.NoError(t, err)
	require.Equal(t, adminplusdomain.PromotionRecommendationSwitchCandidate, event.Recommendation)
	require.True(t, event.SwitchEligible)
}

func TestServiceRecordPromotionDisabledSupplierIsInformational(t *testing.T) {
	repo := newFakePromotionRepository()
	svc := NewService(repo)

	event, err := svc.RecordPromotion(context.Background(), RecordPromotionInput{
		SupplierID:    7,
		Type:          adminplusdomain.PromotionTypeLimitedOffer,
		Title:         "Paused supplier campaign",
		RuntimeStatus: adminplusdomain.SupplierRuntimeStatusDisabled,
		BalanceCents:  5000,
		Currency:      "USD",
	})

	require.NoError(t, err)
	require.Equal(t, adminplusdomain.PromotionRecommendationInformational, event.Recommendation)
	require.False(t, event.SwitchEligible)
}

func TestServiceRecordPromotionValidatesInput(t *testing.T) {
	svc := NewService(newFakePromotionRepository())
	discount := 101.0

	_, err := svc.RecordPromotion(context.Background(), RecordPromotionInput{
		SupplierID:      7,
		Type:            adminplusdomain.PromotionTypeRateDiscount,
		Title:           "Bad discount",
		DiscountPercent: &discount,
	})

	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
	require.Equal(t, "PROMOTION_DISCOUNT_PERCENT_INVALID", infraerrors.Reason(err))
}

func TestServiceRecordPromotionValidatesTimeRange(t *testing.T) {
	svc := NewService(newFakePromotionRepository())
	start := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	end := start.Add(-time.Minute)

	_, err := svc.RecordPromotion(context.Background(), RecordPromotionInput{
		SupplierID: 7,
		Type:       adminplusdomain.PromotionTypePackageDeal,
		Title:      "Invalid range",
		StartsAt:   &start,
		EndsAt:     &end,
	})

	require.Error(t, err)
	require.Equal(t, "PROMOTION_TIME_RANGE_INVALID", infraerrors.Reason(err))
}

type fakePromotionRepository struct {
	nextEventID int64
	events      []*adminplusdomain.PromotionEvent
}

type fakePromotionNotifier struct {
	events []*adminplusdomain.PromotionEvent
}

func (n *fakePromotionNotifier) NotifyPromotion(_ context.Context, event *adminplusdomain.PromotionEvent) error {
	n.events = append(n.events, clonePromotionEvent(event))
	return nil
}

func newFakePromotionRepository() *fakePromotionRepository {
	return &fakePromotionRepository{nextEventID: 1}
}

func (r *fakePromotionRepository) CreateEvent(_ context.Context, event *adminplusdomain.PromotionEvent) (*adminplusdomain.PromotionEvent, error) {
	cp := clonePromotionEvent(event)
	cp.ID = r.nextEventID
	r.nextEventID++
	if cp.CreatedAt.IsZero() {
		cp.CreatedAt = cp.CapturedAt
	}
	r.events = append(r.events, cp)
	return clonePromotionEvent(cp), nil
}

func (r *fakePromotionRepository) ListEvents(_ context.Context, _ EventFilter) ([]*adminplusdomain.PromotionEvent, error) {
	items := make([]*adminplusdomain.PromotionEvent, 0, len(r.events))
	for _, item := range r.events {
		items = append(items, clonePromotionEvent(item))
	}
	return items, nil
}

func (r *fakePromotionRepository) UpdateEventStatus(_ context.Context, id int64, status adminplusdomain.PromotionStatus) (*adminplusdomain.PromotionEvent, error) {
	for _, event := range r.events {
		if event.ID == id {
			event.Status = status
			return clonePromotionEvent(event), nil
		}
	}
	return nil, nil
}

func clonePromotionEvent(in *adminplusdomain.PromotionEvent) *adminplusdomain.PromotionEvent {
	if in == nil {
		return nil
	}
	out := *in
	if in.BonusPercent != nil {
		v := *in.BonusPercent
		out.BonusPercent = &v
	}
	if in.DiscountPercent != nil {
		v := *in.DiscountPercent
		out.DiscountPercent = &v
	}
	if in.StartsAt != nil {
		t := *in.StartsAt
		out.StartsAt = &t
	}
	if in.EndsAt != nil {
		t := *in.EndsAt
		out.EndsAt = &t
	}
	if in.AcknowledgedAt != nil {
		t := *in.AcknowledgedAt
		out.AcknowledgedAt = &t
	}
	if in.RawPayload != nil {
		out.RawPayload = make(map[string]any, len(in.RawPayload))
		for k, v := range in.RawPayload {
			out.RawPayload[k] = v
		}
	}
	return &out
}
