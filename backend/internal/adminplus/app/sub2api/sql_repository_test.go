package sub2api

import (
	"context"
	"database/sql"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func newSub2APISQLMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, mock.ExpectationsWereMet())
		_ = db.Close()
	})
	return db, mock
}

func TestSQLRepositoryListLocalUsageLinesReadsUsageLogs(t *testing.T) {
	db, mock := newSub2APISQLMock(t)
	repo := NewSQLRepository(ReadDB{DB: db})
	from := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)

	mock.ExpectQuery(`FROM usage_logs ul\s+LEFT JOIN accounts a ON a\.id = ul\.account_id\s+WHERE ul\.created_at >= \$1 AND ul\.created_at < \$2 AND ul\.account_id = \$3.*LIMIT \$4`).
		WithArgs(from, to, int64(7), 10).
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"account_id",
			"name",
			"platform",
			"request_id",
			"model",
			"input_tokens",
			"output_tokens",
			"revenue_cents",
			"created_at",
		}).AddRow(
			int64(99),
			int64(7),
			"OpenAI Production",
			"openai",
			"req-1",
			"gpt-4o-mini",
			int64(1000),
			int64(500),
			int64(123),
			from.Add(time.Hour),
		))

	items, err := repo.ListLocalUsageLines(context.Background(), UsageFilter{
		AccountID: 7,
		From:      from,
		To:        to,
		Limit:     10,
	})

	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, int64(99), items[0].ID)
	require.Equal(t, "OpenAI Production", items[0].AccountName)
	require.Equal(t, int64(123), items[0].RevenueCents)
	require.Equal(t, "USD", items[0].Currency)
}

func TestSQLRepositoryListLocalUsageSummariesGroupsByAccountAndModel(t *testing.T) {
	db, mock := newSub2APISQLMock(t)
	repo := NewSQLRepository(ReadDB{DB: db})
	from := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)

	mock.ExpectQuery(`GROUP BY ul\.account_id, a\.name, a\.platform, COALESCE`).
		WithArgs(from, to, "gpt-4o-mini", 20).
		WillReturnRows(sqlmock.NewRows([]string{
			"account_id",
			"name",
			"platform",
			"model",
			"request_count",
			"input_tokens",
			"output_tokens",
			"revenue_cents",
			"account_cost_cents",
			"original_cost_cents",
			"avg_first_token_ms",
			"avg_total_latency_ms",
			"window_start",
			"window_end",
			"last_request_created_at",
		}).AddRow(
			int64(7),
			"OpenAI Production",
			"openai",
			"gpt-4o-mini",
			int64(3),
			int64(3000),
			int64(1500),
			int64(456),
			int64(420),
			int64(300),
			int64(800),
			int64(2400),
			from,
			to.Add(-time.Hour),
			to.Add(-time.Hour),
		))

	items, err := repo.ListLocalUsageSummaries(context.Background(), UsageFilter{
		Model: "gpt-4o-mini",
		From:  from,
		To:    to,
		Limit: 20,
	})

	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, int64(3), items[0].RequestCount)
	require.Equal(t, int64(456), items[0].RevenueCents)
	require.Equal(t, int64(420), items[0].AccountCostCents)
	require.Equal(t, int64(800), items[0].AvgFirstTokenMs)
}

func TestSQLRepositoryListLocalAccountUsageSummariesGroupsByAccount(t *testing.T) {
	db, mock := newSub2APISQLMock(t)
	repo := NewSQLRepository(ReadDB{DB: db})
	from := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)

	mock.ExpectQuery(`GROUP BY ul\.account_id, a\.name, a\.platform\s+ORDER BY request_count DESC`).
		WithArgs(from, to, int64(7), 20).
		WillReturnRows(sqlmock.NewRows([]string{
			"account_id",
			"name",
			"platform",
			"request_count",
			"input_tokens",
			"output_tokens",
			"total_tokens",
			"revenue_cents",
			"account_cost_cents",
			"original_cost_cents",
			"avg_first_token_ms",
			"avg_total_latency_ms",
			"window_start",
			"window_end",
			"last_request_created_at",
		}).AddRow(
			int64(7),
			"OpenAI Production",
			"openai",
			int64(3),
			int64(3000),
			int64(1500),
			int64(4500),
			int64(456),
			int64(420),
			int64(300),
			int64(800),
			int64(2400),
			from,
			to.Add(-time.Hour),
			to.Add(-time.Hour),
		))

	items, err := repo.ListLocalAccountUsageSummaries(context.Background(), UsageFilter{
		AccountID: 7,
		From:      from,
		To:        to,
		Limit:     20,
	})

	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, int64(7), items[0].AccountID)
	require.Equal(t, int64(4500), items[0].TotalTokens)
	require.Equal(t, int64(456), items[0].RevenueCents)
	require.Equal(t, int64(420), items[0].AccountCostCents)
}

func TestRuntimeRepositoryListAccountRuntimeReadsSQLAndRedis(t *testing.T) {
	db, mock := newSub2APISQLMock(t)
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		require.NoError(t, rdb.Close())
	})
	repo := NewRuntimeRepository(ReadDB{DB: db}, Sub2APIRedis{Client: rdb, Configured: true})
	now := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)
	repo.now = func() time.Time { return now }
	resetAt := now.Add(30 * time.Minute)

	require.NoError(t, rdb.ZAdd(context.Background(), "concurrency:account:7", redis.Z{Score: float64(now.Unix()), Member: "req-1"}).Err())
	require.NoError(t, rdb.ZAdd(context.Background(), "concurrency:account:7", redis.Z{Score: float64(now.Unix()), Member: "req-2"}).Err())
	require.NoError(t, rdb.Set(context.Background(), "wait:account:7", "1", time.Hour).Err())

	mock.ExpectQuery(`FROM accounts\s+WHERE deleted_at IS NULL AND id = \$1\s+ORDER BY id DESC\s+LIMIT \$2`).
		WithArgs(int64(7), 20).
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"name",
			"platform",
			"type",
			"status",
			"schedulable",
			"concurrency",
			"error_message",
			"rate_limit_reset_at",
			"overload_until",
			"temp_unschedulable_until",
			"temp_unschedulable_reason",
			"last_used_at",
		}).AddRow(
			int64(7),
			"OpenAI Production",
			"openai",
			"api_key",
			"active",
			true,
			4,
			"",
			nil,
			nil,
			nil,
			"",
			resetAt,
		))

	items, err := repo.ListAccountRuntime(context.Background(), RuntimeFilter{AccountID: 7, Limit: 20})

	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, int64(7), items[0].AccountID)
	require.Equal(t, 2, items[0].CurrentConcurrency)
	require.Equal(t, 1, items[0].WaitingCount)
	require.Equal(t, float64(75), items[0].LoadPercent)
	require.True(t, items[0].SwitchEligible)
	require.True(t, items[0].RedisReadConfigured)
}

func TestRuntimeRepositoryListAccountRuntimeMarksRateLimitedAccountIneligible(t *testing.T) {
	db, mock := newSub2APISQLMock(t)
	repo := NewRuntimeRepository(ReadDB{DB: db}, Sub2APIRedis{})
	now := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)
	repo.now = func() time.Time { return now }
	resetAt := now.Add(time.Hour)

	mock.ExpectQuery(`FROM accounts\s+WHERE deleted_at IS NULL\s+ORDER BY id DESC\s+LIMIT \$1`).
		WithArgs(defaultRuntimeLimit).
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"name",
			"platform",
			"type",
			"status",
			"schedulable",
			"concurrency",
			"error_message",
			"rate_limit_reset_at",
			"overload_until",
			"temp_unschedulable_until",
			"temp_unschedulable_reason",
			"last_used_at",
		}).AddRow(
			int64(8),
			"Anthropic",
			"anthropic",
			"oauth",
			"active",
			true,
			2,
			"",
			resetAt,
			nil,
			nil,
			"",
			nil,
		))

	items, err := repo.ListAccountRuntime(context.Background(), RuntimeFilter{})

	require.NoError(t, err)
	require.Len(t, items, 1)
	require.False(t, items[0].SwitchEligible)
	require.Equal(t, "rate_limited", items[0].BlockedReason)
	require.False(t, items[0].RedisReadConfigured)
}
