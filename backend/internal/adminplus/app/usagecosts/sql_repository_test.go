package usagecosts

import (
	"context"
	"database/sql"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/stretchr/testify/require"
)

func newUsageCostSQLMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, mock.ExpectationsWereMet())
		_ = db.Close()
	})
	return db, mock
}

func TestSQLRepositoryCreateUsageCostLine(t *testing.T) {
	db, mock := newUsageCostSQLMock(t)
	repo := NewSQLRepository(db)
	startedAt := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	endedAt := startedAt.Add(2 * time.Second)
	createdAt := startedAt.Add(time.Minute)

	mock.ExpectQuery(`INSERT INTO admin_plus_supplier_usage_cost_lines`).
		WithArgs(
			int64(7),
			"chrome",
			"bill-1",
			"req-1",
			"sk-prod",
			"gpt-4o-mini",
			"/v1/chat/completions",
			"chat",
			"token",
			"low",
			"USD",
			int64(123),
			int64(1000),
			int64(500),
			int64(200),
			int64(1700),
			int64(680),
			int64(2200),
			"OpenAI/Python",
			startedAt,
			endedAt,
			sqlmock.AnyArg(),
			createdAt,
		).
		WillReturnRows(newUsageCostLineRows().AddRow(
			int64(11),
			int64(7),
			"chrome",
			"bill-1",
			"req-1",
			"sk-prod",
			"gpt-4o-mini",
			"/v1/chat/completions",
			"chat",
			"token",
			"low",
			"USD",
			int64(123),
			int64(1000),
			int64(500),
			int64(200),
			int64(1700),
			int64(680),
			int64(2200),
			"OpenAI/Python",
			startedAt,
			endedAt,
			[]byte(`{"file":"daily.csv"}`),
			createdAt,
		))

	got, err := repo.CreateUsageCostLine(context.Background(), &adminplusdomain.SupplierUsageCostLine{
		SupplierID:          7,
		Source:              "chrome",
		ExternalUsageCostID: "bill-1",
		ExternalRequestID:   "req-1",
		APIKeyName:          "sk-prod",
		Model:               "gpt-4o-mini",
		Endpoint:            "/v1/chat/completions",
		RequestType:         "chat",
		BillingMode:         "token",
		ReasoningEffort:     "low",
		Currency:            "USD",
		CostCents:           123,
		InputTokens:         1000,
		OutputTokens:        500,
		CacheReadTokens:     200,
		TotalTokens:         1700,
		FirstTokenMS:        680,
		DurationMS:          2200,
		UserAgent:           "OpenAI/Python",
		StartedAt:           startedAt,
		EndedAt:             &endedAt,
		RawPayload:          map[string]any{"file": "daily.csv"},
		CreatedAt:           createdAt,
	})

	require.NoError(t, err)
	require.Equal(t, int64(11), got.ID)
	require.NotNil(t, got.EndedAt)
	require.Equal(t, "sk-prod", got.APIKeyName)
	require.Equal(t, int64(1700), got.TotalTokens)
	require.Equal(t, int64(680), got.FirstTokenMS)
	require.Equal(t, "daily.csv", got.RawPayload["file"])
}

func newUsageCostLineRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id",
		"supplier_id",
		"source",
		"external_usage_cost_id",
		"external_request_id",
		"api_key_name",
		"model",
		"endpoint",
		"request_type",
		"billing_mode",
		"reasoning_effort",
		"currency",
		"cost_cents",
		"input_tokens",
		"output_tokens",
		"cache_read_tokens",
		"total_tokens",
		"first_token_ms",
		"duration_ms",
		"user_agent",
		"started_at",
		"ended_at",
		"raw_payload",
		"created_at",
	})
}
