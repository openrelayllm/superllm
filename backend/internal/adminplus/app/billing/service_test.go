package billing

import (
	"context"
	"net/http"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestServiceImportBillLinesNormalizesSupplierBill(t *testing.T) {
	svc := NewService(NewMemoryRepository())
	startedAt := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)

	lines, err := svc.ImportBillLines(context.Background(), []ImportBillLineInput{
		{
			SupplierID:        7,
			Source:            "Chrome",
			ExternalBillID:    "bill-1",
			ExternalRequestID: "req-1",
			APIKeyName:        "sk-prod",
			Model:             "gpt-4o-mini",
			Endpoint:          "/v1/chat/completions",
			RequestType:       "chat",
			BillingMode:       "token",
			ReasoningEffort:   "low",
			Currency:          "usd",
			CostCents:         123,
			InputTokens:       1000,
			OutputTokens:      500,
			CacheReadTokens:   200,
			FirstTokenMS:      680,
			DurationMS:        2200,
			UserAgent:         "OpenAI/Python",
			StartedAt:         startedAt,
		},
	})

	require.NoError(t, err)
	require.Len(t, lines, 1)
	require.Equal(t, int64(1), lines[0].ID)
	require.Equal(t, "chrome", lines[0].Source)
	require.Equal(t, "USD", lines[0].Currency)
	require.Equal(t, int64(123), lines[0].CostCents)
	require.Equal(t, "sk-prod", lines[0].APIKeyName)
	require.Equal(t, "/v1/chat/completions", lines[0].Endpoint)
	require.Equal(t, "token", lines[0].BillingMode)
	require.Equal(t, int64(1700), lines[0].TotalTokens)
	require.Equal(t, int64(680), lines[0].FirstTokenMS)
}

func TestServiceImportBillLinesValidatesInput(t *testing.T) {
	svc := NewService(NewMemoryRepository())

	_, err := svc.ImportBillLines(context.Background(), []ImportBillLineInput{
		{
			SupplierID: 7,
			Model:      "gpt-4o-mini",
			CostCents:  -1,
			StartedAt:  time.Now(),
		},
	})

	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
	require.Equal(t, "BILLING_COST_INVALID", infraerrors.Reason(err))
}

func TestServiceImportBillLinesValidatesDetailMetrics(t *testing.T) {
	svc := NewService(NewMemoryRepository())

	_, err := svc.ImportBillLines(context.Background(), []ImportBillLineInput{
		{
			SupplierID: 7,
			Model:      "gpt-4o-mini",
			CostCents:  1,
			DurationMS: -1,
			StartedAt:  time.Now(),
		},
	})

	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
	require.Equal(t, "BILLING_LATENCY_INVALID", infraerrors.Reason(err))
}
