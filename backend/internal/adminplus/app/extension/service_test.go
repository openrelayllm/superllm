package extension

import (
	"context"
	"net/http"
	"testing"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestServiceClaimTaskUsesPriorityAndCreatesLease(t *testing.T) {
	repo := NewMemoryRepository()
	svc := NewService(repo)
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }
	svc.newToken = func() (string, error) { return "lease-token", nil }

	_, err := svc.CreateTask(context.Background(), CreateTaskInput{
		SupplierID: 1,
		Type:       adminplusdomain.ExtensionTaskTypeFetchRates,
		Priority:   1,
	})
	require.NoError(t, err)
	_, err = svc.CreateTask(context.Background(), CreateTaskInput{
		SupplierID: 2,
		Type:       adminplusdomain.ExtensionTaskTypeFetchUsageCosts,
		Priority:   10,
	})
	require.NoError(t, err)

	task, err := svc.ClaimTask(context.Background(), ClaimTaskInput{
		DeviceID: "chrome-1",
		LeaseTTL: time.Minute,
	})

	require.NoError(t, err)
	require.Equal(t, int64(2), task.SupplierID)
	require.Equal(t, adminplusdomain.ExtensionTaskStatusClaimed, task.Status)
	require.Equal(t, "chrome-1", task.DeviceID)
	require.Equal(t, "lease-token", task.LeaseToken)
	require.Equal(t, 1, task.Attempts)
	require.NotNil(t, task.LeaseExpiresAt)
}

func TestServiceHeartbeatAndCompleteRequireLease(t *testing.T) {
	repo := NewMemoryRepository()
	svc := NewService(repo)
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }
	svc.newToken = func() (string, error) { return "lease-token", nil }
	_, err := svc.CreateTask(context.Background(), CreateTaskInput{
		SupplierID: 1,
		Type:       adminplusdomain.ExtensionTaskTypeFetchBalance,
	})
	require.NoError(t, err)
	task, err := svc.ClaimTask(context.Background(), ClaimTaskInput{DeviceID: "chrome-1"})
	require.NoError(t, err)

	_, err = svc.Heartbeat(context.Background(), HeartbeatInput{
		TaskID:     task.ID,
		DeviceID:   "chrome-1",
		LeaseToken: "bad-token",
	})
	require.Error(t, err)
	require.Equal(t, "EXTENSION_TASK_LEASE_MISMATCH", infraerrors.Reason(err))

	heartbeat, err := svc.Heartbeat(context.Background(), HeartbeatInput{
		TaskID:     task.ID,
		DeviceID:   "chrome-1",
		LeaseToken: "lease-token",
	})
	require.NoError(t, err)
	require.Equal(t, adminplusdomain.ExtensionTaskStatusRunning, heartbeat.Status)

	done, err := svc.CompleteTask(context.Background(), CompleteTaskInput{
		TaskID:     task.ID,
		DeviceID:   "chrome-1",
		LeaseToken: "lease-token",
		Result:     map[string]any{"file": "bill.csv"},
	})
	require.NoError(t, err)
	require.Equal(t, adminplusdomain.ExtensionTaskStatusSucceeded, done.Status)
	require.NotNil(t, done.FinishedAt)
}

func TestServiceCreateLeasedTaskCreatesClaimedTask(t *testing.T) {
	repo := NewMemoryRepository()
	svc := NewService(repo)
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }
	svc.newToken = func() (string, error) { return "lease-token", nil }

	task, err := svc.CreateLeasedTask(context.Background(), CreateLeasedTaskInput{
		SupplierID: 7,
		Type:       adminplusdomain.ExtensionTaskTypeCaptureSession,
		DeviceID:   "chrome-1",
		Payload:    map[string]any{"source_host": "relay.example.com"},
		LeaseTTL:   time.Minute,
	})

	require.NoError(t, err)
	require.Equal(t, adminplusdomain.ExtensionTaskStatusClaimed, task.Status)
	require.Equal(t, "chrome-1", task.DeviceID)
	require.Equal(t, "lease-token", task.LeaseToken)
	require.Equal(t, 1, task.Attempts)
	require.Equal(t, 1, task.MaxAttempts)
	require.NotNil(t, task.LeaseExpiresAt)
	require.Equal(t, now.Add(time.Minute), *task.LeaseExpiresAt)
	require.Equal(t, "relay.example.com", task.Payload["source_host"])
}

func TestServiceFailTaskRetriesUntilMaxAttempts(t *testing.T) {
	repo := NewMemoryRepository()
	svc := NewService(repo)
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }
	svc.newToken = func() (string, error) { return "lease-token", nil }
	_, err := svc.CreateTask(context.Background(), CreateTaskInput{
		SupplierID:  1,
		Type:        adminplusdomain.ExtensionTaskTypeFetchAnnouncements,
		MaxAttempts: 2,
	})
	require.NoError(t, err)
	task, err := svc.ClaimTask(context.Background(), ClaimTaskInput{DeviceID: "chrome-1"})
	require.NoError(t, err)

	failedOnce, err := svc.FailTask(context.Background(), FailTaskInput{
		TaskID:       task.ID,
		DeviceID:     "chrome-1",
		LeaseToken:   "lease-token",
		ErrorCode:    "LOGIN_FAILED",
		ErrorMessage: "login failed",
	})
	require.NoError(t, err)
	require.Equal(t, adminplusdomain.ExtensionTaskStatusPending, failedOnce.Status)
	require.Empty(t, failedOnce.LeaseToken)

	task, err = svc.ClaimTask(context.Background(), ClaimTaskInput{DeviceID: "chrome-1"})
	require.NoError(t, err)
	failedFinal, err := svc.FailTask(context.Background(), FailTaskInput{
		TaskID:       task.ID,
		DeviceID:     "chrome-1",
		LeaseToken:   "lease-token",
		ErrorCode:    "LOGIN_FAILED",
		ErrorMessage: "login failed again",
	})
	require.NoError(t, err)
	require.Equal(t, adminplusdomain.ExtensionTaskStatusFailed, failedFinal.Status)
	require.NotNil(t, failedFinal.FinishedAt)
}

func TestServiceClaimTaskValidatesInput(t *testing.T) {
	svc := NewService(NewMemoryRepository())

	_, err := svc.ClaimTask(context.Background(), ClaimTaskInput{})

	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
	require.Equal(t, "EXTENSION_DEVICE_ID_REQUIRED", infraerrors.Reason(err))
}

func TestServiceGetBrowserCredentialRequiresLease(t *testing.T) {
	repo := NewMemoryRepository()
	credentials := &stubBrowserCredentialProvider{
		credential: &adminplusdomain.SupplierBrowserCredential{
			SupplierID:   1,
			SupplierName: "Relay",
			Type:         adminplusdomain.SupplierTypeSub2API,
			DashboardURL: "https://relay.example.com",
			Username:     "ops@example.com",
			Password:     "secret",
		},
	}
	svc := NewServiceWithDependencies(repo, nil, credentials)
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }
	svc.newToken = func() (string, error) { return "lease-token", nil }
	_, err := svc.CreateTask(context.Background(), CreateTaskInput{
		SupplierID: 1,
		Type:       adminplusdomain.ExtensionTaskTypeFetchRates,
	})
	require.NoError(t, err)
	task, err := svc.ClaimTask(context.Background(), ClaimTaskInput{DeviceID: "chrome-1"})
	require.NoError(t, err)

	_, err = svc.GetBrowserCredential(context.Background(), BrowserCredentialInput{
		TaskID:     task.ID,
		DeviceID:   "chrome-1",
		LeaseToken: "bad-token",
	})
	require.Error(t, err)
	require.Equal(t, "EXTENSION_TASK_LEASE_MISMATCH", infraerrors.Reason(err))

	got, err := svc.GetBrowserCredential(context.Background(), BrowserCredentialInput{
		TaskID:     task.ID,
		DeviceID:   "chrome-1",
		LeaseToken: "lease-token",
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), credentials.requestedSupplierID)
	require.Equal(t, "ops@example.com", got.Username)
	require.Equal(t, "secret", got.Password)
}

type stubBrowserCredentialProvider struct {
	requestedSupplierID int64
	credential          *adminplusdomain.SupplierBrowserCredential
}

func (p *stubBrowserCredentialProvider) GetBrowserCredential(_ context.Context, supplierID int64) (*adminplusdomain.SupplierBrowserCredential, error) {
	p.requestedSupplierID = supplierID
	return p.credential, nil
}
