package notifications

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestServiceDispatchSuppressesWhenChannelDisabled(t *testing.T) {
	repo := NewMemoryRepository()
	svc := NewService(repo)
	settings := defaultSettings()
	settings.Feishu.Enabled = false
	require.NoError(t, repo.SaveSettings(context.Background(), settings))

	err := svc.Dispatch(context.Background(), DispatchInput{
		Type:       "balance.low_balance",
		ID:         21,
		SupplierID: 7,
		Text:       "low balance",
	})

	require.NoError(t, err)
	items, err := repo.ListDeliveries(context.Background(), DeliveryFilter{Limit: 10})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, adminplusdomain.NotificationStatusSuppressed, items[0].Status)
	require.Equal(t, "channel_disabled", items[0].LastError)
}

func TestServiceDispatchSuppressesDisabledRule(t *testing.T) {
	repo := NewMemoryRepository()
	svc := NewService(repo)
	settings := defaultSettings()
	for index := range settings.Rules {
		if settings.Rules[index].EventType == "balance.low_balance" {
			settings.Rules[index].Enabled = false
		}
	}
	settings.Feishu.Enabled = true
	settings.Feishu.WebhookURL = "https://open.feishu.cn/webhook/test"
	require.NoError(t, repo.SaveSettings(context.Background(), settings))

	err := svc.Dispatch(context.Background(), DispatchInput{
		Type:       "balance.low_balance",
		ID:         22,
		SupplierID: 7,
		Text:       "low balance",
	})

	require.NoError(t, err)
	items, err := repo.ListDeliveries(context.Background(), DeliveryFilter{Limit: 10})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, adminplusdomain.NotificationStatusSuppressed, items[0].Status)
	require.Equal(t, "rule_disabled", items[0].LastError)
}

func TestServiceSettingsMasksSecretAndWebhook(t *testing.T) {
	repo := NewMemoryRepository()
	svc := NewService(repo)

	updated, err := svc.UpdateSettings(context.Background(), adminplusdomain.NotificationSettings{
		Feishu: adminplusdomain.NotificationChannelSettings{
			Enabled:       true,
			WebhookURL:    "https://open.feishu.cn/open-apis/bot/v2/hook/token",
			WebhookSecret: "secret",
		},
		Rules: defaultSettings().Rules,
	})

	require.NoError(t, err)
	require.True(t, updated.Feishu.WebhookConfigured)
	require.True(t, updated.Feishu.SecretConfigured)
	require.Empty(t, updated.Feishu.WebhookSecret)
	require.Contains(t, updated.Feishu.WebhookURL, "***")
}

func TestServiceUpdateSettingsPreservesStoredWebhookWhenDisplayURLIsEmpty(t *testing.T) {
	repo := NewMemoryRepository()
	svc := NewService(repo)
	settings := defaultSettings()
	settings.Feishu.Enabled = true
	settings.Feishu.WebhookURL = "https://open.feishu.cn/open-apis/bot/v2/hook/token"
	settings.Feishu.WebhookSecret = "secret"
	require.NoError(t, repo.SaveSettings(context.Background(), settings))

	updated, err := svc.UpdateSettings(context.Background(), adminplusdomain.NotificationSettings{
		Feishu: adminplusdomain.NotificationChannelSettings{
			Enabled:    true,
			WebhookURL: "",
		},
		Rules: defaultSettings().Rules,
	})

	require.NoError(t, err)
	require.True(t, updated.Feishu.WebhookConfigured)
	require.Contains(t, updated.Feishu.WebhookURL, "***")
	internal := svc.effectiveSettings(context.Background())
	require.Equal(t, "https://open.feishu.cn/open-apis/bot/v2/hook/token", internal.Feishu.WebhookURL)
	require.Equal(t, "secret", internal.Feishu.WebhookSecret)
}

func TestServiceUpdateSettingsPreservesLastTestWhenCredentialsAreUnchanged(t *testing.T) {
	repo := NewMemoryRepository()
	svc := NewService(repo)
	testedAt := time.Date(2026, 6, 24, 10, 30, 0, 0, time.UTC)
	settings := defaultSettings()
	settings.Feishu.Enabled = true
	settings.Feishu.WebhookURL = "https://open.feishu.cn/open-apis/bot/v2/hook/token"
	settings.Feishu.WebhookSecret = "secret"
	settings.Feishu.LastTestAt = &testedAt
	settings.Feishu.LastTestStatus = string(adminplusdomain.NotificationStatusSucceeded)
	require.NoError(t, repo.SaveSettings(context.Background(), settings))

	updated, err := svc.UpdateSettings(context.Background(), adminplusdomain.NotificationSettings{
		Feishu: adminplusdomain.NotificationChannelSettings{
			Enabled:    true,
			WebhookURL: "",
		},
		Rules: defaultSettings().Rules,
	})

	require.NoError(t, err)
	require.NotNil(t, updated.Feishu.LastTestAt)
	require.Equal(t, string(adminplusdomain.NotificationStatusSucceeded), updated.Feishu.LastTestStatus)
}

func TestServiceUpdateSettingsClearsLastTestWhenWebhookChanges(t *testing.T) {
	repo := NewMemoryRepository()
	svc := NewService(repo)
	testedAt := time.Date(2026, 6, 24, 10, 30, 0, 0, time.UTC)
	settings := defaultSettings()
	settings.Feishu.Enabled = true
	settings.Feishu.WebhookURL = "https://open.feishu.cn/open-apis/bot/v2/hook/old"
	settings.Feishu.WebhookSecret = "secret"
	settings.Feishu.LastTestAt = &testedAt
	settings.Feishu.LastTestStatus = string(adminplusdomain.NotificationStatusSucceeded)
	settings.Feishu.LastTestError = "old error"
	require.NoError(t, repo.SaveSettings(context.Background(), settings))

	updated, err := svc.UpdateSettings(context.Background(), adminplusdomain.NotificationSettings{
		Feishu: adminplusdomain.NotificationChannelSettings{
			Enabled:       true,
			WebhookURL:    "https://open.feishu.cn/open-apis/bot/v2/hook/new",
			WebhookSecret: "",
		},
		Rules: defaultSettings().Rules,
	})

	require.NoError(t, err)
	require.True(t, updated.Feishu.WebhookConfigured)
	require.Nil(t, updated.Feishu.LastTestAt)
	require.Empty(t, updated.Feishu.LastTestStatus)
	require.Empty(t, updated.Feishu.LastTestError)
}

func TestServiceTestUsesStoredWebhookURLNotMaskedDisplayURL(t *testing.T) {
	repo := NewMemoryRepository()
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		require.Equal(t, http.MethodPost, r.Method)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	svc := NewService(repo)
	settings := defaultSettings()
	settings.Feishu.Enabled = true
	settings.Feishu.WebhookURL = server.URL
	require.NoError(t, repo.SaveSettings(context.Background(), settings))

	publicSettings := svc.Settings(context.Background())
	require.Contains(t, publicSettings.Feishu.WebhookURL, "***")

	delivery, err := svc.Test(context.Background(), TestInput{Text: "hello"})

	require.NoError(t, err)
	require.NotNil(t, delivery)
	require.Equal(t, adminplusdomain.NotificationStatusSucceeded, delivery.Status)
	require.Equal(t, 1, requests)
	settingsAfterTest := svc.Settings(context.Background())
	require.NotNil(t, settingsAfterTest.Feishu.LastTestAt)
	require.Equal(t, string(adminplusdomain.NotificationStatusSucceeded), settingsAfterTest.Feishu.LastTestStatus)
	require.Empty(t, settingsAfterTest.Feishu.LastTestError)
}

func TestServiceTestRecordsFailedDeliveryWhenWebhookFails(t *testing.T) {
	repo := NewMemoryRepository()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		http.Error(w, `{"code":9499,"msg":"bad webhook"}`, http.StatusNotFound)
	}))
	t.Cleanup(server.Close)
	svc := NewService(repo)
	settings := defaultSettings()
	settings.Feishu.Enabled = true
	settings.Feishu.WebhookURL = server.URL
	require.NoError(t, repo.SaveSettings(context.Background(), settings))

	delivery, err := svc.Test(context.Background(), TestInput{Text: "hello"})

	require.Nil(t, delivery)
	require.Error(t, err)
	items, listErr := repo.ListDeliveries(context.Background(), DeliveryFilter{EventType: "system.test", Limit: 10})
	require.NoError(t, listErr)
	require.Len(t, items, 1)
	require.Equal(t, adminplusdomain.NotificationStatusFailed, items[0].Status)
	require.Contains(t, items[0].LastError, "Webhook")
	settingsAfterTest := svc.Settings(context.Background())
	require.NotNil(t, settingsAfterTest.Feishu.LastTestAt)
	require.Equal(t, string(adminplusdomain.NotificationStatusFailed), settingsAfterTest.Feishu.LastTestStatus)
	require.Contains(t, settingsAfterTest.Feishu.LastTestError, "Webhook")
}

func TestServiceCenterStatusCountsSuppressed(t *testing.T) {
	repo := NewMemoryRepository()
	now := time.Date(2026, 6, 24, 10, 0, 0, 0, time.UTC)
	_, _, err := repo.CreateDelivery(context.Background(), &adminplusdomain.NotificationDelivery{
		Channel:    adminplusdomain.NotificationChannelFeishu,
		EventType:  "balance.low_balance",
		EventID:    1,
		SupplierID: 7,
		DedupeKey:  "suppressed-1",
		Status:     adminplusdomain.NotificationStatusSuppressed,
		CreatedAt:  now,
	})
	require.NoError(t, err)
	svc := NewService(repo)

	status := svc.CenterStatus(context.Background())

	require.Equal(t, 1, status.TotalDeliveries)
	require.Equal(t, 1, status.Suppressed)
	require.NotNil(t, status.LastDeliveryAt)
}

func TestServiceRetryDeliveryRejectsNonFailedDelivery(t *testing.T) {
	repo := NewMemoryRepository()
	created, _, err := repo.CreateDelivery(context.Background(), &adminplusdomain.NotificationDelivery{
		Channel:    adminplusdomain.NotificationChannelFeishu,
		EventType:  "system.test",
		EventID:    1,
		SupplierID: 0,
		DedupeKey:  "system-test-succeeded",
		Status:     adminplusdomain.NotificationStatusSucceeded,
		Attempts:   1,
		Payload: map[string]any{
			"content": map[string]any{"text": "ok"},
		},
	})
	require.NoError(t, err)
	svc := NewService(repo)

	delivery, err := svc.RetryDelivery(context.Background(), created.ID)

	require.Nil(t, delivery)
	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
	require.Equal(t, "NOTIFICATION_DELIVERY_RETRY_NOT_ALLOWED", infraerrors.Reason(err))
	stored, err := repo.GetDelivery(context.Background(), created.ID)
	require.NoError(t, err)
	require.Equal(t, adminplusdomain.NotificationStatusSucceeded, stored.Status)
	require.Equal(t, 1, stored.Attempts)
}

func TestDefaultSettingsCoverKnownBusinessEventTypes(t *testing.T) {
	settings := defaultSettings()
	rules := make(map[string]struct{}, len(settings.Rules))
	for _, rule := range settings.Rules {
		rules[normalizeEventType(rule.EventType)] = struct{}{}
	}
	knownEvents := []string{
		"balance." + string(adminplusdomain.BalanceEventTypeLowBalance),
		"balance." + string(adminplusdomain.BalanceEventTypeDepleted),
		"balance." + string(adminplusdomain.BalanceEventTypeRecovered),
		"health." + string(adminplusdomain.HealthEventTypeRequestError),
		"health." + string(adminplusdomain.HealthEventTypeSlowFirstToken),
		"health." + string(adminplusdomain.HealthEventTypeSlowTotal),
		"health." + string(adminplusdomain.HealthEventTypeConcurrencyFull),
		"rate." + string(adminplusdomain.RateChangeDirectionNew),
		"rate." + string(adminplusdomain.RateChangeDirectionIncrease),
		"rate." + string(adminplusdomain.RateChangeDirectionDecrease),
		"announcement." + string(adminplusdomain.AnnouncementTypeRechargeBonus),
		"announcement." + string(adminplusdomain.AnnouncementTypeRateDiscount),
		"announcement." + string(adminplusdomain.AnnouncementTypePackageDeal),
		"announcement." + string(adminplusdomain.AnnouncementTypeLimitedOffer),
		"announcement." + string(adminplusdomain.AnnouncementTypeMaintenance),
		"announcement." + string(adminplusdomain.AnnouncementTypeIncident),
		"announcement." + string(adminplusdomain.AnnouncementTypeNotice),
		"announcement." + string(adminplusdomain.AnnouncementTypeOther),
		"cost.reconcile_anomaly",
		"system.test",
	}
	for _, eventType := range knownEvents {
		_, ok := rules[normalizeEventType(eventType)]
		require.Truef(t, ok, "missing notification rule for %s", eventType)
	}
}
