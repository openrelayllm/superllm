package extension

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	balancesapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/balances"
	healthapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/health"
	ratesapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/rates"
	sessionsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/sessions"
	usagecostsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/usagecosts"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/adminplus/ports"
	"github.com/stretchr/testify/require"
)

func TestCompleteTaskIngestsRateResult(t *testing.T) {
	rateRepo := newIngestRateRepository()
	processor := NewIngestProcessor(
		ratesapp.NewService(rateRepo),
		balancesapp.NewService(balancesapp.NewMemoryRepository()),
		healthapp.NewService(healthapp.NewMemoryRepository()),
		usagecostsapp.NewService(usagecostsapp.NewMemoryRepository()),
		sessionsapp.NewService(sessionsapp.NewMemoryRepository(), stubSessionCipher{}),
	)
	svc := NewServiceWithResultProcessor(NewMemoryRepository(), processor)
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }
	svc.newToken = func() (string, error) { return "lease-token", nil }

	_, err := svc.CreateTask(context.Background(), CreateTaskInput{
		SupplierID: 7,
		Type:       adminplusdomain.ExtensionTaskTypeFetchRates,
	})
	require.NoError(t, err)
	task, err := svc.ClaimTask(context.Background(), ClaimTaskInput{DeviceID: "chrome-1"})
	require.NoError(t, err)
	_, err = svc.Heartbeat(context.Background(), HeartbeatInput{
		TaskID:     task.ID,
		DeviceID:   "chrome-1",
		LeaseToken: "lease-token",
	})
	require.NoError(t, err)

	completed, err := svc.CompleteTask(context.Background(), CompleteTaskInput{
		TaskID:     task.ID,
		DeviceID:   "chrome-1",
		LeaseToken: "lease-token",
		Result: map[string]any{
			"source":            "chrome",
			"threshold_percent": float64(1),
			"entries": []any{
				map[string]any{
					"model":        "gpt-4o-mini",
					"billing_mode": "token",
					"price_item":   "input",
					"unit":         "1m_tokens",
					"currency":     "USD",
					"price_micros": float64(100000),
				},
			},
		},
	})

	require.NoError(t, err)
	require.Equal(t, adminplusdomain.ExtensionTaskStatusSucceeded, completed.Status)
	require.Len(t, rateRepo.snapshots, 1)
	require.Len(t, rateRepo.events, 1)
	require.Equal(t, "gpt-4o-mini", rateRepo.snapshots[0].Model)
	ingest, ok := completed.Result["ingest"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, 1, ingest["rate_snapshots"])
}

func TestCompleteTaskProcessesRegistrationResult(t *testing.T) {
	registration := &stubRegistrationProcessor{
		resultIngest: map[string]any{
			"registration_status": "succeeded",
			"supplier_id":         int64(7),
		},
	}
	processor := NewIngestProcessor(nil, nil, nil, nil, nil).WithRegistrationProcessor(registration)
	svc := NewServiceWithResultProcessor(NewMemoryRepository(), processor)
	task, err := svc.CreateLeasedTask(context.Background(), CreateLeasedTaskInput{
		Type:     adminplusdomain.ExtensionTaskTypeRegisterSupplier,
		DeviceID: "device-1",
		Payload:  map[string]any{"discovery_id": int64(9)},
	})
	require.NoError(t, err)

	completed, err := svc.CompleteTask(context.Background(), CompleteTaskInput{
		TaskID:     task.ID,
		DeviceID:   "device-1",
		LeaseToken: task.LeaseToken,
		Result: map[string]any{
			"registration_submitted": true,
		},
	})

	require.NoError(t, err)
	require.Equal(t, task.ID, registration.resultTaskID)
	ingest, ok := completed.Result["ingest"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "succeeded", ingest["registration_status"])
	require.Equal(t, int64(7), ingest["supplier_id"])
}

func TestFailTaskProcessesRegistrationManualVerification(t *testing.T) {
	registration := &stubRegistrationProcessor{
		failureIngest: map[string]any{
			"registration_status": "waiting_manual_verification",
			"manual_required":     true,
		},
	}
	processor := NewIngestProcessor(nil, nil, nil, nil, nil).WithRegistrationProcessor(registration)
	svc := NewServiceWithResultProcessor(NewMemoryRepository(), processor)
	task, err := svc.CreateLeasedTask(context.Background(), CreateLeasedTaskInput{
		Type:     adminplusdomain.ExtensionTaskTypeRegisterSupplier,
		DeviceID: "device-1",
		Payload:  map[string]any{"discovery_id": int64(9)},
	})
	require.NoError(t, err)

	failed, err := svc.FailTask(context.Background(), FailTaskInput{
		TaskID:       task.ID,
		DeviceID:     "device-1",
		LeaseToken:   task.LeaseToken,
		ErrorCode:    "REGISTRATION_VERIFICATION_REQUIRED",
		ErrorMessage: "需要验证码",
	})

	require.NoError(t, err)
	require.Equal(t, adminplusdomain.ExtensionTaskStatusFailed, failed.Status)
	require.Equal(t, task.ID, registration.failureTaskID)
	ingest, ok := failed.Result["ingest"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "waiting_manual_verification", ingest["registration_status"])
	require.Equal(t, true, ingest["manual_required"])
}

func TestCompleteTaskIngestsUsageCostResult(t *testing.T) {
	usageCostRepo := usagecostsapp.NewMemoryRepository()
	processor := NewIngestProcessor(
		ratesapp.NewService(newIngestRateRepository()),
		balancesapp.NewService(balancesapp.NewMemoryRepository()),
		healthapp.NewService(healthapp.NewMemoryRepository()),
		usagecostsapp.NewService(usageCostRepo),
		sessionsapp.NewService(sessionsapp.NewMemoryRepository(), stubSessionCipher{}),
	)
	svc := NewServiceWithResultProcessor(NewMemoryRepository(), processor)
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }
	svc.newToken = func() (string, error) { return "lease-token", nil }

	_, err := svc.CreateTask(context.Background(), CreateTaskInput{
		SupplierID: 7,
		Type:       adminplusdomain.ExtensionTaskTypeFetchUsageCosts,
	})
	require.NoError(t, err)
	task, err := svc.ClaimTask(context.Background(), ClaimTaskInput{DeviceID: "chrome-1"})
	require.NoError(t, err)
	_, err = svc.Heartbeat(context.Background(), HeartbeatInput{
		TaskID:     task.ID,
		DeviceID:   "chrome-1",
		LeaseToken: "lease-token",
	})
	require.NoError(t, err)

	completed, err := svc.CompleteTask(context.Background(), CompleteTaskInput{
		TaskID:     task.ID,
		DeviceID:   "chrome-1",
		LeaseToken: "lease-token",
		Result: map[string]any{
			"source": "chrome",
			"lines": []any{
				map[string]any{
					"external_usage_cost_id": "bill-1",
					"external_request_id":    "req-1",
					"model":                  "gpt-4o-mini",
					"currency":               "USD",
					"cost_cents":             float64(120),
					"input_tokens":           float64(1000),
					"output_tokens":          float64(300),
					"started_at":             "2026-06-20T10:00:00Z",
					"ended_at":               "2026-06-20T10:00:02Z",
				},
			},
		},
	})

	require.NoError(t, err)
	require.Equal(t, adminplusdomain.ExtensionTaskStatusSucceeded, completed.Status)
	lines, err := usageCostRepo.ListUsageCostLines(context.Background(), usagecostsapp.UsageCostLineFilter{SupplierID: 7, Limit: 20})
	require.NoError(t, err)
	require.Len(t, lines, 1)
	require.Equal(t, "req-1", lines[0].ExternalRequestID)
	ingest, ok := completed.Result["ingest"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, 1, ingest["usage_cost_lines"])
}

func TestCompleteTaskEncryptsCapturedSessionBundle(t *testing.T) {
	sessionRepo := sessionsapp.NewMemoryRepository()
	processor := NewIngestProcessorWithCipher(
		ratesapp.NewService(newIngestRateRepository()),
		balancesapp.NewService(balancesapp.NewMemoryRepository()),
		healthapp.NewService(healthapp.NewMemoryRepository()),
		usagecostsapp.NewService(usagecostsapp.NewMemoryRepository()),
		sessionsapp.NewService(sessionRepo, stubSessionCipher{}),
		stubSessionCipher{},
		nil,
	)
	svc := NewServiceWithResultProcessor(NewMemoryRepository(), processor)
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }
	svc.newToken = func() (string, error) { return "lease-token", nil }

	task, err := svc.CreateLeasedTask(context.Background(), CreateLeasedTaskInput{
		SupplierID: 7,
		Type:       adminplusdomain.ExtensionTaskTypeCaptureSession,
		DeviceID:   "chrome-1",
		LeaseTTL:   time.Minute,
	})
	require.NoError(t, err)

	completed, err := svc.CompleteTask(context.Background(), CompleteTaskInput{
		TaskID:     task.ID,
		DeviceID:   "chrome-1",
		LeaseToken: "lease-token",
		Result: map[string]any{
			"source": "chrome",
			"session_bundle": map[string]any{
				"origin":      "https://relay.example.com",
				"captured_at": "2026-06-20T10:00:00Z",
				"tokens": map[string]any{
					"access_token": "secret-access-token",
				},
				"cookies": []any{
					map[string]any{"name": "sid", "value": "secret-cookie"},
				},
				"context": map[string]any{
					"api_base_url": "https://relay.example.com/api",
				},
				"required_headers": map[string]any{
					"origin":  "https://relay.example.com",
					"referer": "https://relay.example.com/dashboard",
				},
			},
		},
	})

	require.NoError(t, err)
	require.Equal(t, adminplusdomain.ExtensionTaskStatusSucceeded, completed.Status)
	require.NotContains(t, completed.Result, "session_bundle")
	resultJSON, err := json.Marshal(completed.Result)
	require.NoError(t, err)
	require.NotContains(t, string(resultJSON), "secret-access-token")
	require.NotContains(t, string(resultJSON), "secret-cookie")
	summary, ok := completed.Result["session_summary"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, summary["has_access_token"])
	require.Equal(t, 1, summary["cookie_count"])
	ingest, ok := completed.Result["ingest"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, ingest["session_captured"])
	require.NotContains(t, ingest, "session_bundle_ciphertext")

	latest, err := sessionRepo.Get(context.Background(), 7)
	require.NoError(t, err)
	require.Equal(t, int64(7), latest.SupplierID)
	require.Equal(t, "https://relay.example.com", latest.Origin)
	require.Equal(t, "https://relay.example.com/api", latest.APIBaseURL)
	require.Equal(t, int64(task.ID), latest.SourceExtensionTaskID)
	require.Contains(t, latest.SessionBundleCiphertext, "encrypted:")
}

func TestCompleteTaskNormalizesNewAPIBrowserSessionWithoutSyncingBalance(t *testing.T) {
	sessionRepo := sessionsapp.NewMemoryRepository()
	cipher := &recordingSessionCipher{}
	sessionService := sessionsapp.NewService(sessionRepo, cipher)
	probe := &recordingSessionProbe{balanceCents: 1234500}
	processor := NewIngestProcessorWithCipher(
		ratesapp.NewService(newIngestRateRepository()),
		balancesapp.NewServiceWithDependencies(balancesapp.NewMemoryRepository(), nil, sessionService, probe),
		healthapp.NewService(healthapp.NewMemoryRepository()),
		usagecostsapp.NewService(usagecostsapp.NewMemoryRepository()),
		sessionService,
		cipher,
		nil,
	)
	svc := NewServiceWithResultProcessor(NewMemoryRepository(), processor)
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }
	svc.newToken = func() (string, error) { return "lease-token", nil }

	task, err := svc.CreateLeasedTask(context.Background(), CreateLeasedTaskInput{
		SupplierID: 42,
		Type:       adminplusdomain.ExtensionTaskTypeCaptureSession,
		DeviceID:   "chrome-1",
		Payload: map[string]any{
			"provider_type": "new-api",
		},
		LeaseTTL: time.Minute,
	})
	require.NoError(t, err)

	completed, err := svc.CompleteTask(context.Background(), CompleteTaskInput{
		TaskID:     task.ID,
		DeviceID:   "chrome-1",
		LeaseToken: "lease-token",
		Result: map[string]any{
			"source": "chrome",
			"session_bundle": map[string]any{
				"origin":      "https://www.codexapis.com",
				"captured_at": "2026-06-20T10:00:00Z",
				"cookies": []any{
					map[string]any{"name": "session", "value": "secret-cookie"},
				},
				"context": map[string]any{
					"api_base_url": "https://www.codexapis.com",
					"id":           float64(4111),
				},
				"required_headers": map[string]any{
					"origin":  "https://www.codexapis.com",
					"referer": "https://www.codexapis.com/console",
				},
			},
		},
	})

	require.NoError(t, err)
	require.Equal(t, adminplusdomain.ExtensionTaskStatusSucceeded, completed.Status)
	require.NotContains(t, completed.Result, "session_bundle")
	summary, ok := completed.Result["session_summary"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "new_api", summary["provider_type"])
	require.Equal(t, "4111", summary["user_id"])
	require.Equal(t, true, summary["has_new_api_user_header"])
	require.Equal(t, 1, summary["cookie_count"])

	ingest, ok := completed.Result["ingest"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, ingest["session_captured"])
	require.NotContains(t, ingest, "balance_cents")
	require.NotContains(t, ingest, "balance_currency")
	require.NotContains(t, ingest, "balance_probe")
	require.NotContains(t, ingest, "balance_probe_error")
	require.Equal(t, 0, probe.calls)

	latest, err := sessionRepo.Get(context.Background(), 42)
	require.NoError(t, err)
	require.Equal(t, "https://www.codexapis.com", latest.APIBaseURL)
	require.Equal(t, int64(task.ID), latest.SourceExtensionTaskID)

	var savedBundle map[string]any
	require.NoError(t, json.Unmarshal([]byte(cipher.plaintext), &savedBundle))
	require.Equal(t, "new_api", savedBundle["provider_type"])
	require.Equal(t, "new_api", savedBundle["system_type"])
	require.Equal(t, "New-Api-User", savedBundle["auth_header_name"])
	require.Equal(t, "4111", savedBundle["auth_header_value"])
	headers, ok := savedBundle["required_headers"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "4111", headers["New-Api-User"])
	require.Equal(t, "4111", headers["Veloera-User"])
	require.Equal(t, "4111", headers["X-User-Id"])
	require.Equal(t, "4111", headers["neo-api-user"])
}

func TestNormalizeSessionBundleBackfillsNewAPICompatibleUserHeaders(t *testing.T) {
	bundle := normalizeSessionBundle(map[string]any{
		"origin": "https://veloera.example.com",
		"context": map[string]any{
			"api_base_url": "https://veloera.example.com",
		},
		"required_headers": map[string]any{
			"Veloera-User": float64(4111),
		},
	}, &adminplusdomain.ExtensionTask{
		Payload: map[string]any{
			"provider_type": "new-api",
		},
	})

	require.Equal(t, "new_api", bundle["provider_type"])
	require.Equal(t, "New-Api-User", bundle["auth_header_name"])
	require.Equal(t, "4111", bundle["auth_header_value"])
	contextValue, ok := bundle["context"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "4111", contextValue["user_id"])
	headers, ok := bundle["required_headers"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "4111", headers["New-Api-User"])
	require.Equal(t, "4111", headers["Veloera-User"])
	require.Equal(t, "4111", headers["X-User-Id"])
	require.Equal(t, "4111", headers["neo-api-user"])
}

type stubSessionCipher struct{}

func (stubSessionCipher) Encrypt(plaintext string) (string, error) {
	return "encrypted:opaque", nil
}

func (stubSessionCipher) Decrypt(ciphertext string) (string, error) {
	return `{}`, nil
}

type recordingSessionCipher struct {
	plaintext string
}

func (c *recordingSessionCipher) Encrypt(plaintext string) (string, error) {
	c.plaintext = plaintext
	return "encrypted:recorded", nil
}

func (c *recordingSessionCipher) Decrypt(ciphertext string) (string, error) {
	return c.plaintext, nil
}

type recordingSessionProbe struct {
	calls        int
	lastInput    ports.SessionProbeInput
	balanceCents int64
}

func (p *recordingSessionProbe) ProbeSub2APIUserProfile(_ context.Context, in ports.SessionProbeInput) (*ports.SessionProbeResult, error) {
	p.calls++
	p.lastInput = in
	return &ports.SessionProbeResult{
		SupplierID:      in.SupplierID,
		Status:          "ok",
		SystemType:      "new_api",
		Origin:          in.Origin,
		APIBaseURL:      in.APIBaseURL,
		BalanceCents:    &p.balanceCents,
		BalanceCurrency: "USD",
		Profile: &ports.UserProfileSnapshot{
			ID:       42,
			Username: "wutongci",
			Balance:  12345,
		},
		ProbedAt: time.Date(2026, 6, 20, 10, 0, 5, 0, time.UTC),
	}, nil
}

type ingestRateRepository struct {
	nextSnapshotID int64
	nextEventID    int64
	snapshots      []*adminplusdomain.RateSnapshot
	events         []*adminplusdomain.RateChangeEvent
}

func newIngestRateRepository() *ingestRateRepository {
	return &ingestRateRepository{
		nextSnapshotID: 1,
		nextEventID:    1,
	}
}

func (r *ingestRateRepository) CreateSnapshot(_ context.Context, snapshot *adminplusdomain.RateSnapshot) (*adminplusdomain.RateSnapshot, error) {
	cp := cloneIngestRateSnapshot(snapshot)
	cp.ID = r.nextSnapshotID
	r.nextSnapshotID++
	if cp.CreatedAt.IsZero() {
		cp.CreatedAt = cp.CapturedAt
	}
	r.snapshots = append(r.snapshots, cp)
	return cloneIngestRateSnapshot(cp), nil
}

func (r *ingestRateRepository) FindLatestComparableSnapshot(_ context.Context, snapshot *adminplusdomain.RateSnapshot) (*adminplusdomain.RateSnapshot, error) {
	var latest *adminplusdomain.RateSnapshot
	for _, item := range r.snapshots {
		if item.SupplierID != snapshot.SupplierID ||
			item.Model != snapshot.Model ||
			item.BillingMode != snapshot.BillingMode ||
			item.PriceItem != snapshot.PriceItem ||
			item.Unit != snapshot.Unit ||
			item.Currency != snapshot.Currency {
			continue
		}
		if item.CapturedAt.After(snapshot.CapturedAt) {
			continue
		}
		if latest == nil || item.CapturedAt.After(latest.CapturedAt) || (item.CapturedAt.Equal(latest.CapturedAt) && item.ID > latest.ID) {
			latest = item
		}
	}
	return cloneIngestRateSnapshot(latest), nil
}

func (r *ingestRateRepository) CreateChangeEvent(_ context.Context, event *adminplusdomain.RateChangeEvent) (*adminplusdomain.RateChangeEvent, error) {
	cp := cloneIngestRateChangeEvent(event)
	cp.ID = r.nextEventID
	r.nextEventID++
	r.events = append(r.events, cp)
	return cloneIngestRateChangeEvent(cp), nil
}

func (r *ingestRateRepository) ListSnapshots(_ context.Context, _ ratesapp.SnapshotFilter) ([]*adminplusdomain.RateSnapshot, error) {
	items := make([]*adminplusdomain.RateSnapshot, 0, len(r.snapshots))
	for _, item := range r.snapshots {
		items = append(items, cloneIngestRateSnapshot(item))
	}
	return items, nil
}

func (r *ingestRateRepository) ListChangeEvents(_ context.Context, _ ratesapp.EventFilter) ([]*adminplusdomain.RateChangeEvent, error) {
	items := make([]*adminplusdomain.RateChangeEvent, 0, len(r.events))
	for _, item := range r.events {
		items = append(items, cloneIngestRateChangeEvent(item))
	}
	return items, nil
}

func (r *ingestRateRepository) UpdateChangeEventStatus(_ context.Context, id int64, status adminplusdomain.RateChangeStatus) (*adminplusdomain.RateChangeEvent, error) {
	for _, event := range r.events {
		if event.ID == id {
			event.Status = status
			return cloneIngestRateChangeEvent(event), nil
		}
	}
	return nil, nil
}

func cloneIngestRateSnapshot(in *adminplusdomain.RateSnapshot) *adminplusdomain.RateSnapshot {
	if in == nil {
		return nil
	}
	out := *in
	if in.RawPayload != nil {
		out.RawPayload = make(map[string]any, len(in.RawPayload))
		for key, value := range in.RawPayload {
			out.RawPayload[key] = value
		}
	}
	return &out
}

func cloneIngestRateChangeEvent(in *adminplusdomain.RateChangeEvent) *adminplusdomain.RateChangeEvent {
	if in == nil {
		return nil
	}
	out := *in
	if in.OldPriceMicros != nil {
		value := *in.OldPriceMicros
		out.OldPriceMicros = &value
	}
	if in.ChangePercent != nil {
		value := *in.ChangePercent
		out.ChangePercent = &value
	}
	if in.AcknowledgedAt != nil {
		value := *in.AcknowledgedAt
		out.AcknowledgedAt = &value
	}
	return &out
}

type stubRegistrationProcessor struct {
	resultTaskID  int64
	failureTaskID int64
	resultIngest  map[string]any
	failureIngest map[string]any
}

func (p *stubRegistrationProcessor) ProcessRegistrationTaskResult(_ context.Context, task *adminplusdomain.ExtensionTask, _ map[string]any) (map[string]any, error) {
	p.resultTaskID = task.ID
	return p.resultIngest, nil
}

func (p *stubRegistrationProcessor) ProcessRegistrationTaskFailure(_ context.Context, task *adminplusdomain.ExtensionTask, _ string, _ string) (map[string]any, error) {
	p.failureTaskID = task.ID
	return p.failureIngest, nil
}
