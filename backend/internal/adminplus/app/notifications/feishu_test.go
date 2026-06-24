package notifications

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/stretchr/testify/require"
)

func TestFeishuBuildPayload(t *testing.T) {
	notifier := &Feishu{secret: "secret"}

	payload := notifier.buildPayload("hello")

	require.Equal(t, "text", payload["msg_type"])
	require.NotEmpty(t, payload["timestamp"])
	require.NotEmpty(t, payload["sign"])
	content, ok := payload["content"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "hello", content["text"])
}

func TestSignDoesNotExposeSecret(t *testing.T) {
	sign := Sign(1720000000, "secret")

	require.NotEmpty(t, sign)
	require.False(t, strings.Contains(sign, "secret"))
}

func TestFeishuSendEventCreatesDeliveryAndMarksSucceeded(t *testing.T) {
	repo := &fakeNotificationRepository{}
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		require.Equal(t, http.MethodPost, r.Method)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	notifier := &Feishu{webhookURL: server.URL, repo: repo}

	err := notifier.SendEvent(context.Background(), Event{
		Type:       "balance.low_balance",
		ID:         21,
		SupplierID: 7,
		Text:       "hello",
	})

	require.NoError(t, err)
	require.Equal(t, 1, requests)
	require.Len(t, repo.deliveries, 1)
	require.Equal(t, "feishu:balance.low_balance:21", repo.deliveries[0].DedupeKey)
	require.Equal(t, int64(1), repo.succeededID)
}

func TestFeishuSendEventSkipsDuplicateDelivery(t *testing.T) {
	repo := &fakeNotificationRepository{duplicate: true}
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	notifier := &Feishu{webhookURL: server.URL, repo: repo}

	err := notifier.SendEvent(context.Background(), Event{
		Type:       "balance.low_balance",
		ID:         21,
		SupplierID: 7,
		Text:       "hello",
	})

	require.NoError(t, err)
	require.Equal(t, 0, requests)
	require.Empty(t, repo.succeededID)
}

func TestFeishuSendEventMarksFailed(t *testing.T) {
	repo := &fakeNotificationRepository{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	t.Cleanup(server.Close)
	notifier := &Feishu{webhookURL: server.URL, repo: repo}

	err := notifier.SendEvent(context.Background(), Event{
		Type:       "health.request_error",
		ID:         31,
		SupplierID: 7,
		Text:       "hello",
	})

	require.Error(t, err)
	require.Equal(t, int64(1), repo.failedID)
	require.Contains(t, repo.failedMessage, "502")
}

func TestFeishuSendEventReturnsReadableWebhookError(t *testing.T) {
	repo := &fakeNotificationRepository{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":9499,"msg":"not found"}`))
	}))
	t.Cleanup(server.Close)
	notifier := &Feishu{webhookURL: server.URL, repo: repo}

	err := notifier.SendEvent(context.Background(), Event{
		Type:       "system.test",
		ID:         41,
		SupplierID: 0,
		Text:       "hello",
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "FEISHU_WEBHOOK_FAILED")
	require.Contains(t, repo.failedMessage, "飞书 Webhook 返回 HTTP 404")
	require.Contains(t, repo.failedMessage, "Webhook 地址")
	require.NotContains(t, repo.failedMessage, "internal error")
}

func TestFeishuSendEventThrottlesWithinWindow(t *testing.T) {
	repo := NewMemoryRepository()
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	now := time.Date(2026, 6, 20, 10, 5, 0, 0, time.UTC)
	notifier := &Feishu{
		webhookURL: server.URL,
		repo:       repo,
		now:        func() time.Time { return now },
	}

	first := Event{
		Type:           "health.request_error",
		ID:             31,
		SupplierID:     7,
		ThrottleKey:    "supplier:7:model:gpt-5.5:type:request_error",
		ThrottleWindow: 30 * time.Minute,
		Text:           "first",
	}
	second := first
	second.ID = 32
	second.Text = "second"

	require.NoError(t, notifier.SendEvent(context.Background(), first))
	require.NoError(t, notifier.SendEvent(context.Background(), second))
	require.Equal(t, 1, requests)

	now = now.Add(30 * time.Minute)
	third := first
	third.ID = 33
	third.Text = "third"
	require.NoError(t, notifier.SendEvent(context.Background(), third))
	require.Equal(t, 2, requests)

	deliveries, err := repo.ListDeliveries(context.Background(), DeliveryFilter{Limit: 10})
	require.NoError(t, err)
	require.Len(t, deliveries, 2)
	require.NotEqual(t, deliveries[0].DedupeKey, deliveries[1].DedupeKey)
}

type fakeNotificationRepository struct {
	duplicate     bool
	deliveries    []*adminplusdomain.NotificationDelivery
	succeededID   int64
	failedID      int64
	failedMessage string
}

func (r *fakeNotificationRepository) CreateDelivery(_ context.Context, delivery *adminplusdomain.NotificationDelivery) (*adminplusdomain.NotificationDelivery, bool, error) {
	if r.duplicate {
		return nil, false, nil
	}
	copied := *delivery
	copied.ID = int64(len(r.deliveries) + 1)
	r.deliveries = append(r.deliveries, &copied)
	return &copied, true, nil
}

func (r *fakeNotificationRepository) ListDeliveries(_ context.Context, _ DeliveryFilter) ([]*adminplusdomain.NotificationDelivery, error) {
	return r.deliveries, nil
}

func (r *fakeNotificationRepository) GetDelivery(_ context.Context, id int64) (*adminplusdomain.NotificationDelivery, error) {
	for _, item := range r.deliveries {
		if item.ID == id {
			return item, nil
		}
	}
	return nil, nil
}

func (r *fakeNotificationRepository) MarkDeliverySucceeded(_ context.Context, id int64) error {
	r.succeededID = id
	return nil
}

func (r *fakeNotificationRepository) MarkDeliveryFailed(_ context.Context, id int64, message string) error {
	r.failedID = id
	r.failedMessage = message
	return nil
}

func (r *fakeNotificationRepository) IncrementDeliveryAttempt(_ context.Context, id int64) (*adminplusdomain.NotificationDelivery, error) {
	return r.GetDelivery(context.Background(), id)
}

func (r *fakeNotificationRepository) LoadSettings(_ context.Context) (*adminplusdomain.NotificationSettings, error) {
	return nil, nil
}

func (r *fakeNotificationRepository) SaveSettings(_ context.Context, _ adminplusdomain.NotificationSettings) error {
	return nil
}
