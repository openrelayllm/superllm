package suppliergroups

import (
	"context"
	"testing"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/adminplus/ports"
	"github.com/stretchr/testify/require"
)

func TestServiceSyncUpsertsGroupsAndMarksMissing(t *testing.T) {
	repo := NewMemoryRepository()
	notifier := &fakeGroupNotifier{}
	session := &stubSessionReader{
		input: ports.SessionProbeInput{
			SupplierID: 7,
			Origin:     "https://relay.example.com",
			APIBaseURL: "https://relay.example.com/api/v1",
			Bundle:     map[string]any{"access_token": "token"},
		},
	}
	reader := &stubSessionGroupReader{
		results: []*ports.ReadGroupsResult{
			{
				SupplierID: 7,
				SystemType: "sub2api",
				Origin:     "https://relay.example.com",
				APIBaseURL: "https://relay.example.com/api/v1",
				CapturedAt: time.Date(2026, 6, 21, 1, 2, 3, 0, time.UTC),
				Groups: []*ports.ProviderGroup{
					{
						ExternalGroupID:         "10",
						Name:                    "Low Cost",
						ProviderFamily:          "openai",
						RateMultiplier:          0.8,
						EffectiveRateMultiplier: 0.05,
						UserRateMultiplier:      float64Ptr(0.05),
						Status:                  "active",
						RawPayload:              map[string]any{"id": 10},
					},
					{
						ExternalGroupID:         "11",
						Name:                    "Private",
						ProviderFamily:          "anthropic",
						RateMultiplier:          1.2,
						EffectiveRateMultiplier: 1.2,
						IsPrivate:               true,
						Status:                  "active",
					},
				},
			},
			{
				SupplierID: 7,
				SystemType: "sub2api",
				Origin:     "https://relay.example.com",
				APIBaseURL: "https://relay.example.com/api/v1",
				CapturedAt: time.Date(2026, 6, 21, 2, 2, 3, 0, time.UTC),
				Groups: []*ports.ProviderGroup{
					{
						ExternalGroupID:         "10",
						Name:                    "Low Cost Updated",
						ProviderFamily:          "openai",
						RateMultiplier:          0.9,
						EffectiveRateMultiplier: 0.9,
						Status:                  "active",
					},
				},
			},
		},
	}
	svc := NewServiceWithNotifier(repo, notifier, session, reader)

	first, err := svc.Sync(context.Background(), 7)
	require.NoError(t, err)
	require.Equal(t, 2, first.Total)
	require.Len(t, first.Events, 2)
	openAIEvent := requireEventByExternalID(t, first.Events, "10")
	require.Equal(t, adminplusdomain.SupplierGroupChangeDirectionNew, openAIEvent.Direction)
	require.True(t, openAIEvent.LowRate)
	anthropicEvent := requireEventByExternalID(t, first.Events, "11")
	require.Equal(t, adminplusdomain.SupplierGroupChangeDirectionNew, anthropicEvent.Direction)
	require.False(t, anthropicEvent.LowRate)
	require.Len(t, notifier.events, 2)
	require.Equal(t, "Low Cost", first.Groups[0].Name)
	require.NotNil(t, first.Groups[0].UserRateMultiplier)
	require.Equal(t, 0.05, *first.Groups[0].UserRateMultiplier)

	second, err := svc.Sync(context.Background(), 7)
	require.NoError(t, err)
	require.Equal(t, 1, second.Total)
	require.Len(t, second.Events, 1)
	require.Equal(t, adminplusdomain.SupplierGroupChangeDirectionIncrease, second.Events[0].Direction)
	require.NotNil(t, second.Events[0].OldEffectiveRateMultiplier)
	require.Equal(t, 0.05, *second.Events[0].OldEffectiveRateMultiplier)
	require.Equal(t, 0.9, second.Events[0].NewEffectiveRateMultiplier)
	require.Equal(t, "Low Cost Updated", second.Groups[0].Name)
	require.Len(t, notifier.events, 3)

	all, err := svc.List(context.Background(), ListFilter{SupplierID: 7})
	require.NoError(t, err)
	require.Len(t, all, 2)
	require.Equal(t, "10", all[0].ExternalGroupID)
	require.Equal(t, adminplusdomain.SupplierGroupStatusActive, all[0].Status)
	require.Equal(t, "11", all[1].ExternalGroupID)
	require.Equal(t, adminplusdomain.SupplierGroupStatusMissing, all[1].Status)

	events, err := svc.ListChangeEvents(context.Background(), EventFilter{SupplierID: 7})
	require.NoError(t, err)
	require.Len(t, events, 3)
	require.Equal(t, adminplusdomain.SupplierGroupChangeDirectionIncrease, events[0].Direction)
}

func TestServiceSyncMarksOpenAISuperLowRateEvent(t *testing.T) {
	repo := NewMemoryRepository()
	session := &stubSessionReader{input: ports.SessionProbeInput{SupplierID: 9}}
	reader := &stubSessionGroupReader{
		results: []*ports.ReadGroupsResult{
			{
				SupplierID: 9,
				CapturedAt: time.Date(2026, 6, 21, 3, 2, 3, 0, time.UTC),
				Groups: []*ports.ProviderGroup{
					{
						ExternalGroupID:         "openai-low",
						Name:                    "GPT low",
						ProviderFamily:          "openai",
						RateMultiplier:          0.05,
						EffectiveRateMultiplier: 0.05,
						Status:                  "active",
					},
				},
			},
		},
	}
	svc := NewService(repo, session, reader)

	result, err := svc.Sync(context.Background(), 9)

	require.NoError(t, err)
	require.Len(t, result.Events, 1)
	require.True(t, result.Events[0].LowRate)
}

func TestServiceListAllowsGlobalSupplierGroupLookup(t *testing.T) {
	repo := NewMemoryRepository()
	now := time.Date(2026, 7, 8, 10, 0, 0, 0, time.UTC)
	_, err := repo.UpsertMany(context.Background(), 7, []*adminplusdomain.SupplierGroup{
		{
			SupplierID:              7,
			ExternalGroupID:         "gpt-low",
			Name:                    "GPT Low",
			ProviderFamily:          "openai",
			RateMultiplier:          1,
			EffectiveRateMultiplier: 0.05,
			Status:                  adminplusdomain.SupplierGroupStatusActive,
			LastSeenAt:              now,
			CreatedAt:               now,
			UpdatedAt:               now,
		},
	}, now)
	require.NoError(t, err)
	_, err = repo.UpsertMany(context.Background(), 8, []*adminplusdomain.SupplierGroup{
		{
			SupplierID:              8,
			ExternalGroupID:         "claude-low",
			Name:                    "Claude Low",
			ProviderFamily:          "anthropic",
			RateMultiplier:          1,
			EffectiveRateMultiplier: 0.08,
			Status:                  adminplusdomain.SupplierGroupStatusActive,
			LastSeenAt:              now.Add(time.Minute),
			CreatedAt:               now,
			UpdatedAt:               now,
		},
	}, now)
	require.NoError(t, err)
	svc := NewService(repo, nil, nil)

	all, err := svc.List(context.Background(), ListFilter{Status: adminplusdomain.SupplierGroupStatusActive})

	require.NoError(t, err)
	require.Len(t, all, 2)
	require.Equal(t, int64(8), all[0].SupplierID)
	require.Equal(t, int64(7), all[1].SupplierID)

	_, err = svc.List(context.Background(), ListFilter{SupplierID: -1})
	require.Error(t, err)
}

func TestServiceUpdateKeyCapacityPreservedBySync(t *testing.T) {
	repo := NewMemoryRepository()
	session := &stubSessionReader{input: ports.SessionProbeInput{SupplierID: 7}}
	reader := &stubSessionGroupReader{
		results: []*ports.ReadGroupsResult{
			{
				SupplierID: 7,
				CapturedAt: time.Date(2026, 7, 9, 10, 0, 0, 0, time.UTC),
				Groups: []*ports.ProviderGroup{
					{
						ExternalGroupID:         "gpt-low",
						Name:                    "GPT Low",
						ProviderFamily:          "openai",
						RateMultiplier:          1,
						EffectiveRateMultiplier: 0.05,
						Status:                  "active",
					},
				},
			},
			{
				SupplierID: 7,
				CapturedAt: time.Date(2026, 7, 9, 11, 0, 0, 0, time.UTC),
				Groups: []*ports.ProviderGroup{
					{
						ExternalGroupID:         "gpt-low",
						Name:                    "GPT Low Renamed",
						ProviderFamily:          "openai",
						RateMultiplier:          1,
						EffectiveRateMultiplier: 0.05,
						Status:                  "active",
					},
				},
			},
		},
	}
	svc := NewService(repo, session, reader)

	first, err := svc.Sync(context.Background(), 7)
	require.NoError(t, err)
	require.Len(t, first.Groups, 1)

	updated, err := svc.UpdateKeyCapacity(context.Background(), UpdateKeyCapacityInput{
		SupplierID:      7,
		SupplierGroupID: first.Groups[0].ID,
		KeyLimitPolicy:  adminplusdomain.SupplierGroupKeyLimitPolicyLimited,
		KeyLimitValue:   1,
	})
	require.NoError(t, err)
	require.Equal(t, adminplusdomain.SupplierGroupKeyLimitPolicyLimited, updated.KeyLimitPolicy)
	require.Equal(t, 1, updated.KeyLimitValue)

	second, err := svc.Sync(context.Background(), 7)
	require.NoError(t, err)
	require.Len(t, second.Groups, 1)
	require.Equal(t, "GPT Low Renamed", second.Groups[0].Name)
	require.Equal(t, adminplusdomain.SupplierGroupKeyLimitPolicyLimited, second.Groups[0].KeyLimitPolicy)
	require.Equal(t, 1, second.Groups[0].KeyLimitValue)
}

type stubSessionReader struct {
	input ports.SessionProbeInput
}

func (s *stubSessionReader) DecryptedProbeInput(_ context.Context, _ int64) (ports.SessionProbeInput, error) {
	return s.input, nil
}

type stubSessionGroupReader struct {
	results []*ports.ReadGroupsResult
	calls   int
}

type fakeGroupNotifier struct {
	events []*adminplusdomain.SupplierGroupChangeEvent
}

func (n *fakeGroupNotifier) NotifyGroupChange(_ context.Context, event *adminplusdomain.SupplierGroupChangeEvent) error {
	n.events = append(n.events, cloneSupplierGroupChangeEvent(event))
	return nil
}

func requireEventByExternalID(t *testing.T, events []*adminplusdomain.SupplierGroupChangeEvent, externalID string) *adminplusdomain.SupplierGroupChangeEvent {
	t.Helper()
	for _, event := range events {
		if event != nil && event.ExternalGroupID == externalID {
			return event
		}
	}
	require.Failf(t, "missing supplier group event", "external_group_id=%s events=%v", externalID, events)
	return nil
}

func (s *stubSessionGroupReader) ReadGroups(_ context.Context, _ ports.SessionProbeInput) (*ports.ReadGroupsResult, error) {
	result := s.results[s.calls]
	s.calls++
	return result, nil
}

func float64Ptr(value float64) *float64 {
	return &value
}
