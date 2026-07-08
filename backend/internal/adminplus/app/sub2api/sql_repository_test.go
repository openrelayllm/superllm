package sub2api

import (
	"context"
	"database/sql"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/lib/pq"
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

func TestSQLRepositoryListLocalAccountOpsReadsBindingsAndHealth(t *testing.T) {
	db, mock := newSub2APISQLMock(t)
	repo := NewSQLRepository(ReadDB{DB: db})
	now := time.Date(2026, 6, 20, 9, 0, 0, 0, time.UTC)
	checkAt := now.Add(-30 * time.Minute)
	schedulable := true

	mock.ExpectQuery(`WITH local_groups AS \(\s+SELECT[\s\S]*FROM accounts a[\s\S]*LEFT JOIN admin_plus_supplier_accounts asa ON asa\.local_sub2api_account_id = a\.id[\s\S]*admin_plus_supplier_channel_check_snapshots c[\s\S]*WHERE a\.deleted_at IS NULL[\s\S]*asa\.supplier_id = \$3[\s\S]*sg\.id = \$4[\s\S]*agf\.group_id = \$5[\s\S]*COALESCE\(sg\.effective_rate_multiplier, a\.rate_multiplier, 1\) <= \$6[\s\S]*END\) = \$7[\s\S]*COALESCE\(lc\.probe_status, 'untested'\) = \$8[\s\S]*COALESCE\(a\.schedulable, false\) = \$9[\s\S]*LIMIT \$10`).
		WithArgs("%lime%", "Lime", int64(7), int64(77), int64(1001), 0.2, "usable", "available", true, 50).
		WillReturnRows(newLocalAccountOpsRows().AddRow(
			int64(42),
			"Lime gpt-4o",
			"openai",
			"api_key",
			"active",
			"",
			true,
			6,
			10,
			0.3,
			nil,
			nil,
			nil,
			nil,
			"",
			now,
			pq.Int64Array{1001},
			pq.StringArray{"Lime"},
			int64(88),
			int64(7),
			"Lime Supplier",
			"new_api",
			"active",
			"normal",
			"active",
			"normal",
			int64(100),
			int64(2000),
			"USD",
			true,
			int64(77),
			"grp-lime",
			"Lime",
			"openai",
			"OpenAI",
			"GPT-4o",
			"active",
			0.2,
			int64(66),
			"key-Lime",
			"abcd",
			"bound",
			"available",
			"available",
			"ok",
			true,
			200,
			"",
			"",
			checkAt,
			"usable",
			"synced",
			now,
		))

	items, err := repo.ListLocalAccountOps(context.Background(), LocalAccountOpsFilter{
		Query:              "Lime",
		SupplierID:         7,
		SupplierGroupID:    77,
		LocalGroupID:       1001,
		MaxRateMultiplier:  0.2,
		BalanceStatus:      "usable",
		ChannelCheckStatus: "available",
		Schedulable:        &schedulable,
		Limit:              50,
	})

	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, int64(42), items[0].LocalSub2APIAccountID)
	require.Equal(t, []int64{1001}, items[0].LocalAccountGroupIDs)
	require.Equal(t, "Lime Supplier", items[0].SupplierName)
	require.Equal(t, int64(77), items[0].SupplierGroupID)
	require.Equal(t, "OpenAI", items[0].SupplierGroupModelFamily)
	require.Equal(t, "GPT-4o", items[0].SupplierGroupModelSpec)
	require.Equal(t, int64(66), items[0].SupplierKeyID)
	require.Equal(t, 0.2, items[0].EffectiveRateMultiplier)
	require.Equal(t, "available", items[0].KeyCapacityStatus)
	require.Equal(t, "usable", items[0].BalanceStatus)
	require.Equal(t, "available", items[0].ChannelCheckStatus)
	require.Equal(t, "synced", items[0].DriftStatus)
	require.NotNil(t, items[0].LastChannelCheckAt)
	require.Equal(t, checkAt, *items[0].LastChannelCheckAt)
}

func TestSQLRepositoryPreviewLocalAccountOpsActionBlocksEmptyPool(t *testing.T) {
	db, mock := newSub2APISQLMock(t)
	repo := NewSQLRepository(ReadDB{DB: db})
	schedulable := false

	mock.ExpectQuery(`SELECT id\s+FROM accounts\s+WHERE deleted_at IS NULL AND id = ANY\(\$1\)\s+ORDER BY id`).
		WithArgs(pq.Array([]int64{42})).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(42)))
	mock.ExpectQuery(`SELECT DISTINCT ag\.group_id\s+FROM account_groups ag\s+INNER JOIN groups g ON g\.id = ag\.group_id AND g\.deleted_at IS NULL\s+WHERE ag\.account_id = ANY\(\$1\)\s+ORDER BY ag\.group_id`).
		WithArgs(pq.Array([]int64{42})).
		WillReturnRows(sqlmock.NewRows([]string{"group_id"}).AddRow(int64(1001)))
	mock.ExpectQuery(`SELECT\s+g\.id,\s+g\.name,[\s\S]*FROM groups g[\s\S]*WHERE g\.deleted_at IS NULL AND g\.id = ANY\(\$1\)`).
		WithArgs(pq.Array([]int64{1001}), pq.Array([]int64{42}), string(adminplusdomain.LocalAccountOpsActionSetSchedulable)).
		WillReturnRows(sqlmock.NewRows([]string{
			"group_id",
			"group_name",
			"active_api_key_count",
			"before_schedulable_accounts",
			"after_schedulable_accounts",
		}).AddRow(int64(1001), "Lime", int64(3), int64(1), int64(0)))

	result, err := repo.PreviewLocalAccountOpsAction(context.Background(), LocalAccountOpsActionInput{
		Action:      adminplusdomain.LocalAccountOpsActionSetSchedulable,
		AccountIDs:  []int64{42},
		Schedulable: &schedulable,
	})

	require.NoError(t, err)
	require.True(t, result.DryRun)
	require.True(t, result.Blocked)
	require.Equal(t, "LOCAL_GROUP_SCHEDULABLE_POOL_WOULD_BE_EMPTY", result.BlockedReason)
	require.Len(t, result.GroupImpacts, 1)
	require.True(t, result.GroupImpacts[0].WouldEmptySchedulablePool)
}

func TestSQLRepositoryApplyLocalAccountOpsActionUpdatesSchedulableAndOutbox(t *testing.T) {
	db, mock := newSub2APISQLMock(t)
	repo := NewSQLRepository(ReadDB{DB: db})
	schedulable := false

	mock.ExpectBegin()
	expectLocalAccountStateSyncForApply(mock, []int64{42}, nil)
	mock.ExpectQuery(`SELECT id\s+FROM accounts\s+WHERE deleted_at IS NULL AND id = ANY\(\$1\)\s+ORDER BY id`).
		WithArgs(pq.Array([]int64{42})).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(42)))
	mock.ExpectQuery(`SELECT DISTINCT ag\.group_id\s+FROM account_groups ag\s+INNER JOIN groups g ON g\.id = ag\.group_id AND g\.deleted_at IS NULL\s+WHERE ag\.account_id = ANY\(\$1\)\s+ORDER BY ag\.group_id`).
		WithArgs(pq.Array([]int64{42})).
		WillReturnRows(sqlmock.NewRows([]string{"group_id"}).AddRow(int64(1001)))
	mock.ExpectQuery(`SELECT\s+g\.id,\s+g\.name,[\s\S]*FROM groups g[\s\S]*WHERE g\.deleted_at IS NULL AND g\.id = ANY\(\$1\)`).
		WithArgs(pq.Array([]int64{1001}), pq.Array([]int64{42}), string(adminplusdomain.LocalAccountOpsActionSetSchedulable)).
		WillReturnRows(sqlmock.NewRows([]string{
			"group_id",
			"group_name",
			"active_api_key_count",
			"before_schedulable_accounts",
			"after_schedulable_accounts",
		}).AddRow(int64(1001), "Lime", int64(0), int64(1), int64(0)))
	mock.ExpectExec(`UPDATE accounts\s+SET schedulable = \$2, updated_at = NOW\(\)\s+WHERE deleted_at IS NULL AND id = ANY\(\$1\) AND COALESCE\(schedulable, false\) <> \$2`).
		WithArgs(pq.Array([]int64{42}), false).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO scheduler_outbox \(event_type, account_id, group_id, payload\)`).
		WithArgs(service.SchedulerOutboxEventAccountBulkChanged, nil, nil, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(`SELECT DISTINCT ag\.group_id\s+FROM account_groups ag\s+INNER JOIN groups g ON g\.id = ag\.group_id AND g\.deleted_at IS NULL\s+WHERE ag\.account_id = ANY\(\$1\)\s+ORDER BY ag\.group_id`).
		WithArgs(pq.Array([]int64{42})).
		WillReturnRows(sqlmock.NewRows([]string{"group_id"}).AddRow(int64(1001)))
	mock.ExpectExec(`INSERT INTO scheduler_outbox \(event_type, account_id, group_id, payload\)`).
		WithArgs(service.SchedulerOutboxEventGroupChanged, nil, ptrInt64(1001), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(2, 1))
	expectAcceptLocalAccountStateSnapshots(mock, []int64{42})
	mock.ExpectCommit()

	result, err := repo.ApplyLocalAccountOpsAction(context.Background(), LocalAccountOpsActionInput{
		Action:      adminplusdomain.LocalAccountOpsActionSetSchedulable,
		AccountIDs:  []int64{42},
		Schedulable: &schedulable,
		RequestedBy: 99,
	})

	require.NoError(t, err)
	require.False(t, result.DryRun)
	require.False(t, result.Blocked)
	require.Equal(t, int64(1), result.UpdatedAccounts)
}

func TestSQLRepositoryApplyLocalAccountOpsActionBlocksPendingLocalStateDrift(t *testing.T) {
	db, mock := newSub2APISQLMock(t)
	repo := NewSQLRepository(ReadDB{DB: db})
	schedulable := false

	mock.ExpectBegin()
	expectLocalAccountStateSyncForApply(mock, []int64{42}, []int64{42})
	mock.ExpectRollback()

	result, err := repo.ApplyLocalAccountOpsAction(context.Background(), LocalAccountOpsActionInput{
		Action:      adminplusdomain.LocalAccountOpsActionSetSchedulable,
		AccountIDs:  []int64{42},
		Schedulable: &schedulable,
		RequestedBy: 99,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.DryRun)
	require.True(t, result.Blocked)
	require.Equal(t, "LOCAL_ACCOUNT_STATE_DRIFT_PENDING", result.BlockedReason)
	require.Equal(t, []int64{42}, result.AccountIDs)
	require.Contains(t, result.Warnings, "检测到 Sub2API 原后台手工改动，请先同步并采纳或恢复本地状态")
}

func TestSQLRepositoryGetGroupAvailabilityReadsSchedulablePool(t *testing.T) {
	db, mock := newSub2APISQLMock(t)
	repo := NewSQLRepository(ReadDB{DB: db})
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`FROM groups g\s+LEFT JOIN account_groups ag ON ag\.group_id = g\.id\s+LEFT JOIN accounts a ON a\.id = ag\.account_id\s+LEFT JOIN api_keys ak ON ak\.group_id = g\.id\s+WHERE g\.deleted_at IS NULL AND g\.id = \$1\s+GROUP BY g\.id, g\.name`).
		WithArgs(int64(1001), "openai").
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"name",
			"total_accounts",
			"schedulable_accounts",
			"active_api_key_count",
		}).AddRow(int64(1001), "Lime", int64(3), int64(0), int64(2)))
	mock.ExpectQuery(`(?s)WITH usage_agg AS .*FROM usage_agg, error_agg`).
		WithArgs(int64(1001), int64(86400)).
		WillReturnRows(sqlmock.NewRows([]string{
			"success_request_count",
			"token_count",
			"last_request_at",
			"error_request_count",
			"upstream_429_count",
			"last_error_at",
		}).AddRow(int64(12), int64(3456), now.Add(-time.Hour), int64(3), int64(2), now.Add(-30*time.Minute)))
	mock.ExpectQuery(`(?s)FROM ops_error_logs e\s+LEFT JOIN api_keys ak ON ak\.id = e\.api_key_id\s+WHERE e\.group_id = \$1.*ORDER BY e\.created_at DESC, e\.id DESC\s+LIMIT \$3`).
		WithArgs(int64(1001), int64(86400), 6).
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"request_id",
			"api_key_id",
			"api_key_name",
			"key_preview",
			"user_id",
			"account_id",
			"model",
			"status_code",
			"upstream_status_code",
			"error_owner",
			"error_type",
			"error_message",
			"created_at",
		}).AddRow(int64(88), "req-failed", int64(10), "prod-key", "sk-abc...1234", int64(7), int64(42), "gpt-4o", 502, 429, "provider", "upstream_http", "rate limited", now.Add(-30*time.Minute)))
	mock.ExpectQuery(`(?s)FROM api_keys ak\s+LEFT JOIN LATERAL .*WHERE ak\.deleted_at IS NULL\s+AND ak\.status = 'active'\s+AND ak\.group_id = \$1\s+ORDER BY ak\.last_used_at DESC NULLS LAST, ak\.id DESC\s+LIMIT \$3`).
		WithArgs(int64(1001), int64(86400), 21).
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"user_id",
			"name",
			"key_preview",
			"status",
			"last_used_at",
			"success_request_count",
			"token_count",
			"last_request_at",
			"error_request_count",
			"upstream_429_count",
			"last_error_at",
		}).AddRow(int64(10), int64(7), "prod-key", "sk-abc...1234", "active", now, int64(8), int64(2345), now.Add(-2*time.Hour), int64(2), int64(1), now.Add(-time.Hour)))

	item, err := repo.GetGroupAvailability(context.Background(), 1001, "openai")

	require.NoError(t, err)
	require.Equal(t, int64(1001), item.GroupID)
	require.Equal(t, "Lime", item.GroupName)
	require.Equal(t, int64(3), item.TotalAccounts)
	require.Equal(t, int64(0), item.SchedulableAccounts)
	require.Equal(t, int64(2), item.ActiveAPIKeyCount)
	require.True(t, item.WouldEmptySchedulablePool)
	require.Equal(t, int64(86400), item.RecentWindowSeconds)
	require.Equal(t, int64(12), item.RecentSuccessRequestCount)
	require.Equal(t, int64(3), item.RecentErrorRequestCount)
	require.Equal(t, int64(2), item.RecentUpstream429Count)
	require.Equal(t, int64(3456), item.RecentTokenCount)
	require.NotNil(t, item.RecentLastRequestAt)
	require.NotNil(t, item.RecentLastErrorAt)
	require.False(t, item.RecentFailuresTruncated)
	require.Len(t, item.RecentFailureRequests, 1)
	require.Equal(t, int64(88), item.RecentFailureRequests[0].ID)
	require.Equal(t, "req-failed", item.RecentFailureRequests[0].RequestID)
	require.Equal(t, int64(10), item.RecentFailureRequests[0].APIKeyID)
	require.Equal(t, "prod-key", item.RecentFailureRequests[0].APIKeyName)
	require.Equal(t, "sk-abc...1234", item.RecentFailureRequests[0].APIKeyPreview)
	require.Equal(t, int64(42), item.RecentFailureRequests[0].AccountID)
	require.Equal(t, "gpt-4o", item.RecentFailureRequests[0].Model)
	require.Equal(t, 502, item.RecentFailureRequests[0].StatusCode)
	require.Equal(t, 429, item.RecentFailureRequests[0].UpstreamStatusCode)
	require.Equal(t, "provider", item.RecentFailureRequests[0].ErrorOwner)
	require.Equal(t, "rate limited", item.RecentFailureRequests[0].ErrorMessage)
	require.False(t, item.ImpactedAPIKeysTruncated)
	require.Len(t, item.ImpactedAPIKeys, 1)
	require.Equal(t, int64(10), item.ImpactedAPIKeys[0].ID)
	require.Equal(t, int64(7), item.ImpactedAPIKeys[0].UserID)
	require.Equal(t, "prod-key", item.ImpactedAPIKeys[0].Name)
	require.Equal(t, "sk-abc...1234", item.ImpactedAPIKeys[0].KeyPreview)
	require.NotNil(t, item.ImpactedAPIKeys[0].LastUsedAt)
	require.Equal(t, int64(8), item.ImpactedAPIKeys[0].RecentSuccessRequestCount)
	require.Equal(t, int64(2), item.ImpactedAPIKeys[0].RecentErrorRequestCount)
	require.Equal(t, int64(1), item.ImpactedAPIKeys[0].RecentUpstream429Count)
	require.Equal(t, int64(2345), item.ImpactedAPIKeys[0].RecentTokenCount)
	require.NotNil(t, item.ImpactedAPIKeys[0].RecentLastRequestAt)
	require.NotNil(t, item.ImpactedAPIKeys[0].RecentLastErrorAt)
}

func TestSQLRepositoryListLocalGroupsReadsCapacityProjection(t *testing.T) {
	db, mock := newSub2APISQLMock(t)
	repo := NewSQLRepository(ReadDB{DB: db})

	mock.ExpectQuery(`FROM groups g\s+LEFT JOIN account_groups ag ON ag\.group_id = g\.id\s+LEFT JOIN accounts a ON a\.id = ag\.account_id\s+LEFT JOIN api_keys ak ON ak\.group_id = g\.id\s+WHERE g\.deleted_at IS NULL\s+GROUP BY g\.id, g\.name, g\.platform, g\.status, g\.rate_multiplier, g\.is_exclusive\s+ORDER BY LOWER\(g\.name\), g\.id\s+LIMIT \$1`).
		WithArgs(1000).
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"name",
			"platform",
			"status",
			"rate_multiplier",
			"is_exclusive",
			"total_accounts",
			"schedulable_accounts",
			"active_api_key_count",
		}).AddRow(int64(1001), "Lime", "openai", "active", 1.25, false, int64(3), int64(0), int64(2)))

	items, err := repo.ListLocalGroups(context.Background(), 0)

	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, int64(1001), items[0].ID)
	require.Equal(t, "Lime", items[0].Name)
	require.Equal(t, "openai", items[0].Platform)
	require.Equal(t, "active", items[0].Status)
	require.Equal(t, 1.25, items[0].RateMultiplier)
	require.False(t, items[0].IsExclusive)
	require.Equal(t, int64(3), items[0].TotalAccounts)
	require.Equal(t, int64(0), items[0].SchedulableAccounts)
	require.Equal(t, int64(2), items[0].ActiveAPIKeyCount)
	require.True(t, items[0].WouldEmptySchedulablePool)
}

func TestSQLRepositoryCreateRoutingRefillRunPersistsSnapshot(t *testing.T) {
	db, mock := newSub2APISQLMock(t)
	repo := NewSQLRepository(ReadDB{DB: db})
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`INSERT INTO admin_plus_routing_refill_runs`).
		WithArgs(
			"",
			"local",
			int64(1001),
			"Lime",
			"openai",
			"",
			"manual",
			false,
			"succeeded",
			"manual_scheduler_center",
			"",
			int64(0),
			int64(0),
			int64(2),
			int64(1),
			int64(1),
			int64(2),
			int64(7),
			int64(45),
			int64(99),
			int64(42),
			0.12,
			int64(10),
			"",
			"",
			`{"source":"test"}`,
			`{"ok":true}`,
		).
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"run_id",
			"sub2api_instance_id",
			"local_group_id",
			"local_group_name",
			"platform",
			"model_scope",
			"trigger_type",
			"dry_run",
			"status",
			"reason",
			"skipped_reason",
			"before_total_accounts",
			"before_schedulable_accounts",
			"before_active_api_key_count",
			"after_total_accounts",
			"after_schedulable_accounts",
			"after_active_api_key_count",
			"selected_supplier_id",
			"selected_supplier_group_id",
			"selected_supplier_key_id",
			"selected_local_account_id",
			"selected_effective_rate_multiplier",
			"requested_by",
			"error_code",
			"error_message",
			"request_snapshot",
			"result_snapshot",
			"created_at",
			"updated_at",
		}).AddRow(
			int64(1),
			"",
			"local",
			int64(1001),
			"Lime",
			"openai",
			"",
			"manual",
			false,
			"succeeded",
			"manual_scheduler_center",
			"",
			int64(0),
			int64(0),
			int64(2),
			int64(1),
			int64(1),
			int64(2),
			int64(7),
			int64(45),
			int64(99),
			int64(42),
			0.12,
			int64(10),
			"",
			"",
			[]byte(`{"source":"test"}`),
			[]byte(`{"ok":true}`),
			now,
			now,
		))

	item, err := repo.CreateRoutingRefillRun(context.Background(), &RoutingRefillRun{
		Sub2APIInstanceID:               "local",
		LocalGroupID:                    1001,
		LocalGroupName:                  "Lime",
		Platform:                        "openai",
		TriggerType:                     "manual",
		Status:                          "succeeded",
		Reason:                          "manual_scheduler_center",
		BeforeActiveAPIKeyCount:         2,
		AfterTotalAccounts:              1,
		AfterSchedulableAccounts:        1,
		AfterActiveAPIKeyCount:          2,
		SelectedSupplierID:              7,
		SelectedSupplierGroupID:         45,
		SelectedSupplierKeyID:           99,
		SelectedLocalAccountID:          42,
		SelectedEffectiveRateMultiplier: 0.12,
		RequestedBy:                     10,
		RequestSnapshot:                 map[string]any{"source": "test"},
		ResultSnapshot:                  map[string]any{"ok": true},
	})

	require.NoError(t, err)
	require.Equal(t, int64(1), item.ID)
	require.Equal(t, "succeeded", item.Status)
	require.Equal(t, "test", item.RequestSnapshot["source"])
	require.Equal(t, true, item.ResultSnapshot["ok"])
}

func TestSQLRepositoryGetRoutingFailureSensitiveDetailRedactsRecordedFields(t *testing.T) {
	db, mock := newSub2APISQLMock(t)
	repo := NewSQLRepository(ReadDB{DB: db})
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`FROM ops_error_logs e\s+LEFT JOIN api_keys ak ON ak\.id = e\.api_key_id\s+WHERE e\.id = \$1\s+AND e\.group_id = \$2`).
		WithArgs(int64(123), int64(1001)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"group_id",
			"request_id",
			"api_key_id",
			"name",
			"key_preview",
			"user_id",
			"account_id",
			"model",
			"status_code",
			"upstream_status_code",
			"error_owner",
			"error_type",
			"created_at",
			"error_message",
			"error_body",
			"upstream_error_message",
			"upstream_error_detail",
			"provider_error_code",
			"provider_error_type",
			"network_error_type",
			"error_source",
			"inbound_endpoint",
			"upstream_endpoint",
			"requested_model",
			"upstream_model",
			"retry_after_seconds",
		}).AddRow(
			int64(123),
			int64(1001),
			"req-1",
			int64(7),
			"prod key",
			"sk-abc...1234",
			int64(9),
			int64(42),
			"gpt-4.1",
			500,
			429,
			"upstream",
			"rate_limit",
			now,
			`upstream failed authorization=Bearer secret-token`,
			`{"error":{"api_key":"sk-secret","message":"quota"}}`,
			"upstream says token=abc123",
			"detail",
			"rate_limit",
			"quota",
			"timeout",
			"upstream_http",
			"/v1/chat/completions",
			"/v1/responses",
			"gpt-4.1",
			"gpt-4.1-mini",
			30,
		))

	item, err := repo.GetRoutingFailureSensitiveDetail(context.Background(), RoutingSensitiveFailureDetailInput{
		FailureID:    123,
		LocalGroupID: 1001,
		Fields:       []string{"error_message", "error_body", "request_body", "request_headers", "retry_after_seconds"},
	})

	require.NoError(t, err)
	require.Equal(t, int64(123), item.ID)
	require.True(t, item.Available)
	require.Len(t, item.Fields, 5)
	require.Equal(t, "error_message", item.Fields[0].Name)
	require.Contains(t, item.Fields[0].Value, "authorization=***")
	require.Equal(t, "error_body", item.Fields[1].Name)
	require.Contains(t, item.Fields[1].Value, `"api_key":"***"`)
	require.Equal(t, "request_body", item.Fields[2].Name)
	require.False(t, item.Fields[2].Available)
	require.Equal(t, routingSensitiveUnavailableNotRecorded, item.Fields[2].UnavailableReason)
	require.Equal(t, "request_headers", item.Fields[3].Name)
	require.False(t, item.Fields[3].Available)
	require.Equal(t, routingSensitiveUnavailableNotRecorded, item.Fields[3].UnavailableReason)
	require.Equal(t, "30", item.Fields[4].Value)
}

func TestSQLRepositoryListRoutingRefillRunsFiltersByGroupAndStatus(t *testing.T) {
	db, mock := newSub2APISQLMock(t)
	repo := NewSQLRepository(ReadDB{DB: db})
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`FROM admin_plus_routing_refill_runs\s+WHERE 1=1 AND local_group_id = \$1 AND status = \$2\s+ORDER BY created_at DESC, id DESC\s+LIMIT \$3`).
		WithArgs(int64(1001), "skipped", 10).
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"run_id",
			"sub2api_instance_id",
			"local_group_id",
			"local_group_name",
			"platform",
			"model_scope",
			"trigger_type",
			"dry_run",
			"status",
			"reason",
			"skipped_reason",
			"before_total_accounts",
			"before_schedulable_accounts",
			"before_active_api_key_count",
			"after_total_accounts",
			"after_schedulable_accounts",
			"after_active_api_key_count",
			"selected_supplier_id",
			"selected_supplier_group_id",
			"selected_supplier_key_id",
			"selected_local_account_id",
			"selected_effective_rate_multiplier",
			"requested_by",
			"error_code",
			"error_message",
			"request_snapshot",
			"result_snapshot",
			"created_at",
			"updated_at",
		}).AddRow(
			int64(2),
			"",
			"local",
			int64(1001),
			"Lime",
			"openai",
			"",
			"manual",
			false,
			"skipped",
			"manual_scheduler_center",
			"group_has_schedulable_accounts",
			int64(1),
			int64(1),
			int64(2),
			int64(0),
			int64(0),
			int64(0),
			int64(0),
			int64(0),
			int64(0),
			int64(0),
			0.0,
			int64(10),
			"",
			"",
			[]byte(`{}`),
			[]byte(`{}`),
			now,
			now,
		))

	items, err := repo.ListRoutingRefillRuns(context.Background(), RoutingRefillRunFilter{
		LocalGroupID: 1001,
		Status:       "skipped",
		Limit:        10,
	})

	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, int64(2), items[0].ID)
	require.Equal(t, "group_has_schedulable_accounts", items[0].SkippedReason)
}

func TestSQLRepositoryTryRoutingRefillLockAcquiresAndReleases(t *testing.T) {
	db, mock := newSub2APISQLMock(t)
	repo := NewSQLRepository(ReadDB{DB: db})

	mock.ExpectQuery(`SELECT pg_try_advisory_lock\(\$1\)`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"pg_try_advisory_lock"}).AddRow(true))
	mock.ExpectQuery(`SELECT pg_advisory_unlock\(\$1\)`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"pg_advisory_unlock"}).AddRow(true))

	unlock, acquired, err := repo.TryRoutingRefillLock(context.Background(), 1001)

	require.NoError(t, err)
	require.True(t, acquired)
	require.NotNil(t, unlock)
	require.NoError(t, unlock())
}

func TestSQLRepositoryTryRoutingRefillLockReturnsBusy(t *testing.T) {
	db, mock := newSub2APISQLMock(t)
	repo := NewSQLRepository(ReadDB{DB: db})

	mock.ExpectQuery(`SELECT pg_try_advisory_lock\(\$1\)`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"pg_try_advisory_lock"}).AddRow(false))

	unlock, acquired, err := repo.TryRoutingRefillLock(context.Background(), 1001)

	require.NoError(t, err)
	require.False(t, acquired)
	require.Nil(t, unlock)
}

func TestSQLRepositoryGetAccountReadsLocalGroups(t *testing.T) {
	db, mock := newSub2APISQLMock(t)
	repo := NewSQLRepository(ReadDB{DB: db})

	mock.ExpectQuery(`FROM accounts a\s+LEFT JOIN account_groups ag ON ag\.account_id = a\.id\s+WHERE a\.deleted_at IS NULL AND a\.id = \$1\s+GROUP BY a\.id, a\.name, a\.platform, a\.type, a\.status, a\.schedulable`).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"name",
			"platform",
			"type",
			"status",
			"schedulable",
			"group_ids",
		}).AddRow(int64(42), "Lime gpt-4o", "openai", "api_key", "active", true, pq.Int64Array{1001, 1002}))

	item, err := repo.GetAccount(context.Background(), 42)

	require.NoError(t, err)
	require.Equal(t, int64(42), item.AccountID)
	require.Equal(t, "Lime gpt-4o", item.Name)
	require.Equal(t, "openai", item.Platform)
	require.True(t, item.Schedulable)
	require.Equal(t, []int64{1001, 1002}, item.GroupIDs)
}

func TestSQLRepositorySyncLocalAccountStateReturnsPendingDrift(t *testing.T) {
	db, mock := newSub2APISQLMock(t)
	repo := NewSQLRepository(ReadDB{DB: db})
	now := time.Date(2026, 6, 20, 9, 0, 0, 0, time.UTC)
	firstDetectedAt := now.Add(-time.Hour)

	mock.ExpectBegin()
	expectLocalAccountStateSync(mock, []int64{42}, 25, true)
	mock.ExpectQuery(`SELECT\s+lss\.local_sub2api_account_id,[\s\S]*FROM admin_plus_local_account_state_snapshots lss[\s\S]*ORDER BY CASE WHEN lss\.drift_status = 'pending' THEN 0 ELSE 1 END`).
		WithArgs(pq.Array([]int64{42}), 25).
		WillReturnRows(sqlmock.NewRows([]string{
			"local_sub2api_account_id",
			"account_name",
			"accepted_account_name",
			"accepted_account_platform",
			"accepted_account_type",
			"accepted_schedulable",
			"accepted_group_ids",
			"observed_account_name",
			"observed_account_platform",
			"observed_account_type",
			"observed_schedulable",
			"observed_group_ids",
			"drift_fields",
			"first_drift_detected_at",
			"last_checked_at",
			"drift_status",
		}).AddRow(
			int64(42),
			"Lime gpt-4o",
			"Lime old",
			"openai",
			"api_key",
			true,
			pq.Int64Array{1001},
			"Lime gpt-4o",
			"openai",
			"api_key",
			false,
			pq.Int64Array{1001, 1002},
			pq.StringArray{"name", "schedulable", "groups"},
			firstDetectedAt,
			now,
			"pending",
		))
	mock.ExpectCommit()

	result, err := repo.SyncLocalAccountState(context.Background(), LocalAccountStateSyncInput{
		AccountIDs: []int64{42},
		Limit:      25,
	})

	require.NoError(t, err)
	require.Equal(t, int64(1), result.CheckedAccounts)
	require.Equal(t, int64(0), result.SyncedAccounts)
	require.Equal(t, int64(1), result.DriftedAccounts)
	require.Equal(t, int64(1), result.PendingDriftAccounts)
	require.Len(t, result.Items, 1)
	require.Equal(t, []string{"name", "schedulable", "groups"}, result.Items[0].DriftFields)
	require.Equal(t, []int64{1001}, result.Items[0].Accepted.GroupIDs)
	require.Equal(t, []int64{1001, 1002}, result.Items[0].Observed.GroupIDs)
	require.NotNil(t, result.Items[0].FirstDetectedAt)
}

func TestSQLRepositoryResolveLocalAccountStateAcceptsObservedSnapshot(t *testing.T) {
	db, mock := newSub2APISQLMock(t)
	repo := NewSQLRepository(ReadDB{DB: db})
	now := time.Date(2026, 6, 20, 9, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	expectLocalAccountStateSync(mock, []int64{42}, 1, true)
	expectPendingLocalAccountStateDrift(mock, []int64{42}, []int64{42})
	mock.ExpectExec(`UPDATE admin_plus_local_account_state_snapshots\s+SET accepted_account_name = observed_account_name`).
		WithArgs(pq.Array([]int64{42})).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE admin_plus_local_account_drift_events\s+SET status = \$2`).
		WithArgs(pq.Array([]int64{42}), "accepted").
		WillReturnResult(sqlmock.NewResult(0, 1))
	expectLocalAccountStateSyncResult(mock, []int64{42}, 1, now, "synced")
	mock.ExpectCommit()

	result, err := repo.ResolveLocalAccountState(context.Background(), LocalAccountStateResolutionInput{
		Action:     adminplusdomain.LocalAccountStateResolutionAcceptObserved,
		AccountIDs: []int64{42},
	})

	require.NoError(t, err)
	require.Equal(t, adminplusdomain.LocalAccountStateResolutionAcceptObserved, result.Action)
	require.Equal(t, int64(1), result.ResolvedAccounts)
	require.Equal(t, int64(0), result.PendingDriftAccounts)
}

func TestSQLRepositoryResolveLocalAccountStateRestoresAcceptedSnapshot(t *testing.T) {
	db, mock := newSub2APISQLMock(t)
	repo := NewSQLRepository(ReadDB{DB: db})
	now := time.Date(2026, 6, 20, 9, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	expectLocalAccountStateSync(mock, []int64{42}, 1, true)
	expectPendingLocalAccountStateDrift(mock, []int64{42}, []int64{42})
	mock.ExpectQuery(`SELECT DISTINCT accepted_group_id[\s\S]*FROM admin_plus_local_account_state_snapshots lss[\s\S]*g\.id IS NULL`).
		WithArgs(pq.Array([]int64{42})).
		WillReturnRows(sqlmock.NewRows([]string{"accepted_group_id"}))
	mock.ExpectQuery(`SELECT DISTINCT group_id[\s\S]*unnest\(lss\.accepted_group_ids \|\| lss\.observed_group_ids\)`).
		WithArgs(pq.Array([]int64{42})).
		WillReturnRows(sqlmock.NewRows([]string{"group_id"}).AddRow(int64(1001)).AddRow(int64(1002)))
	mock.ExpectExec(`UPDATE accounts a\s+SET name = lss\.accepted_account_name`).
		WithArgs(pq.Array([]int64{42})).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`DELETE FROM account_groups ag\s+USING admin_plus_local_account_state_snapshots lss`).
		WithArgs(pq.Array([]int64{42})).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO account_groups \(account_id, group_id, priority, created_at\)`).
		WithArgs(pq.Array([]int64{42})).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO scheduler_outbox \(event_type, account_id, group_id, payload\)`).
		WithArgs(service.SchedulerOutboxEventAccountBulkChanged, nil, nil, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`INSERT INTO scheduler_outbox \(event_type, account_id, group_id, payload\)`).
		WithArgs(service.SchedulerOutboxEventGroupChanged, nil, ptrInt64(1001), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectExec(`INSERT INTO scheduler_outbox \(event_type, account_id, group_id, payload\)`).
		WithArgs(service.SchedulerOutboxEventGroupChanged, nil, ptrInt64(1002), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(3, 1))
	expectAcceptLocalAccountStateSnapshots(mock, []int64{42})
	mock.ExpectExec(`UPDATE admin_plus_local_account_drift_events\s+SET status = \$2`).
		WithArgs(pq.Array([]int64{42}), "restored").
		WillReturnResult(sqlmock.NewResult(0, 1))
	expectLocalAccountStateSyncResult(mock, []int64{42}, 1, now, "synced")
	mock.ExpectCommit()

	result, err := repo.ResolveLocalAccountState(context.Background(), LocalAccountStateResolutionInput{
		Action:      adminplusdomain.LocalAccountStateResolutionRestoreAccepted,
		AccountIDs:  []int64{42},
		RequestedBy: 99,
	})

	require.NoError(t, err)
	require.Equal(t, adminplusdomain.LocalAccountStateResolutionRestoreAccepted, result.Action)
	require.Equal(t, int64(1), result.ResolvedAccounts)
	require.Equal(t, int64(1), result.RestoredAccounts)
	require.Equal(t, int64(0), result.PendingDriftAccounts)
}

func TestSQLRepositoryHasSupplierUsageSinceReadsBoundAccounts(t *testing.T) {
	db, mock := newSub2APISQLMock(t)
	repo := NewSQLRepository(ReadDB{DB: db})
	since := time.Date(2026, 6, 20, 9, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`FROM usage_logs ul\s+INNER JOIN admin_plus_supplier_accounts asa\s+ON asa\.local_sub2api_account_id = ul\.account_id\s+WHERE asa\.supplier_id = \$1\s+AND ul\.created_at >= \$2`).
		WithArgs(int64(7), since).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	hasUsage, err := repo.HasSupplierUsageSince(context.Background(), 7, since)

	require.NoError(t, err)
	require.True(t, hasUsage)
}

func TestSQLRepositoryHasSupplierUsageSinceReturnsFalseWhenUnused(t *testing.T) {
	db, mock := newSub2APISQLMock(t)
	repo := NewSQLRepository(ReadDB{DB: db})
	since := time.Date(2026, 6, 20, 9, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`FROM usage_logs ul\s+INNER JOIN admin_plus_supplier_accounts asa\s+ON asa\.local_sub2api_account_id = ul\.account_id\s+WHERE asa\.supplier_id = \$1\s+AND ul\.created_at >= \$2`).
		WithArgs(int64(7), since).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	hasUsage, err := repo.HasSupplierUsageSince(context.Background(), 7, since)

	require.NoError(t, err)
	require.False(t, hasUsage)
}

func expectLocalAccountStateSyncForApply(mock sqlmock.Sqlmock, accountIDs []int64, pendingAccountIDs []int64) {
	expectLocalAccountStateSync(mock, accountIDs, len(accountIDs), false)
	expectPendingLocalAccountStateDrift(mock, accountIDs, pendingAccountIDs)
}

func expectPendingLocalAccountStateDrift(mock sqlmock.Sqlmock, accountIDs []int64, pendingAccountIDs []int64) {
	rows := sqlmock.NewRows([]string{"local_sub2api_account_id"})
	for _, id := range pendingAccountIDs {
		rows.AddRow(id)
	}
	mock.ExpectQuery(`SELECT local_sub2api_account_id\s+FROM admin_plus_local_account_state_snapshots\s+WHERE drift_status = 'pending'[\s\S]*local_sub2api_account_id = ANY\(\$1\)[\s\S]*ORDER BY local_sub2api_account_id`).
		WithArgs(pq.Array(accountIDs)).
		WillReturnRows(rows)
}

func expectLocalAccountStateSyncResult(mock sqlmock.Sqlmock, accountIDs []int64, limit int, checkedAt time.Time, driftStatus string) {
	mock.ExpectQuery(`SELECT\s+lss\.local_sub2api_account_id,[\s\S]*FROM admin_plus_local_account_state_snapshots lss[\s\S]*ORDER BY CASE WHEN lss\.drift_status = 'pending' THEN 0 ELSE 1 END`).
		WithArgs(pq.Array(accountIDs), limit).
		WillReturnRows(sqlmock.NewRows([]string{
			"local_sub2api_account_id",
			"account_name",
			"accepted_account_name",
			"accepted_account_platform",
			"accepted_account_type",
			"accepted_schedulable",
			"accepted_group_ids",
			"observed_account_name",
			"observed_account_platform",
			"observed_account_type",
			"observed_schedulable",
			"observed_group_ids",
			"drift_fields",
			"first_drift_detected_at",
			"last_checked_at",
			"drift_status",
		}).AddRow(
			int64(42),
			"Lime gpt-4o",
			"Lime gpt-4o",
			"openai",
			"api_key",
			true,
			pq.Int64Array{1001},
			"Lime gpt-4o",
			"openai",
			"api_key",
			true,
			pq.Int64Array{1001},
			pq.StringArray{},
			nil,
			checkedAt,
			driftStatus,
		))
}

func expectLocalAccountStateSync(mock sqlmock.Sqlmock, accountIDs []int64, limit int, recordEvents bool) {
	mock.ExpectExec(`WITH current_state AS \([\s\S]*INSERT INTO admin_plus_local_account_state_snapshots[\s\S]*ON CONFLICT \(local_sub2api_account_id\) DO UPDATE[\s\S]*last_checked_at = NOW\(\)`).
		WithArgs(pq.Array(accountIDs), limit).
		WillReturnResult(sqlmock.NewResult(0, int64(len(accountIDs))))
	mock.ExpectExec(`UPDATE admin_plus_supplier_accounts asa\s+SET local_account_name = COALESCE\(a\.name, ''\),[\s\S]*asa\.local_sub2api_account_id = a\.id`).
		WithArgs(pq.Array(accountIDs)).
		WillReturnResult(sqlmock.NewResult(0, int64(len(accountIDs))))
	if recordEvents {
		mock.ExpectExec(`INSERT INTO admin_plus_local_account_drift_events \([\s\S]*FROM admin_plus_local_account_state_snapshots lss[\s\S]*lss\.drift_status = 'pending'`).
			WithArgs(pq.Array(accountIDs)).
			WillReturnResult(sqlmock.NewResult(0, 1))
	}
}

func expectAcceptLocalAccountStateSnapshots(mock sqlmock.Sqlmock, accountIDs []int64) {
	mock.ExpectExec(`WITH current_state AS \([\s\S]*WHERE a\.deleted_at IS NULL AND a\.id = ANY\(\$1\)[\s\S]*INSERT INTO admin_plus_local_account_state_snapshots[\s\S]*accepted_account_name = EXCLUDED\.accepted_account_name`).
		WithArgs(pq.Array(accountIDs)).
		WillReturnResult(sqlmock.NewResult(0, int64(len(accountIDs))))
}

func newLocalAccountOpsRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"local_sub2api_account_id",
		"local_account_name",
		"local_account_platform",
		"local_account_type",
		"local_account_status",
		"local_account_error_message",
		"local_account_schedulable",
		"local_account_concurrency",
		"local_account_priority",
		"local_account_rate_multiplier",
		"local_account_rate_limited_at",
		"local_account_rate_limit_reset_at",
		"local_account_overload_until",
		"local_account_temp_unschedulable_until",
		"local_account_temp_unschedulable_reason",
		"local_account_updated_at",
		"local_account_group_ids",
		"local_account_group_names",
		"supplier_account_id",
		"supplier_id",
		"supplier_name",
		"supplier_type",
		"supplier_runtime_status",
		"supplier_health_status",
		"supplier_account_runtime_status",
		"supplier_account_health_status",
		"balance_threshold_cents",
		"balance_cents",
		"balance_currency",
		"has_usable_balance",
		"supplier_group_id",
		"supplier_external_group_id",
		"supplier_group_name",
		"supplier_group_provider",
		"supplier_group_model_family",
		"supplier_group_model_spec",
		"supplier_group_status",
		"effective_rate_multiplier",
		"supplier_key_id",
		"supplier_key_name",
		"supplier_key_last4",
		"supplier_key_status",
		"key_capacity_status",
		"channel_check_status",
		"channel_remote_status",
		"channel_recommended",
		"channel_status_code",
		"channel_error_class",
		"channel_error_message",
		"last_channel_check_at",
		"balance_status",
		"drift_status",
		"last_local_sync_at",
	})
}

func ptrInt64(value int64) *int64 {
	return &value
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
