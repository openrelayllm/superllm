package purity

import (
	"context"
	"database/sql"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func newPuritySQLMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, mock.ExpectationsWereMet())
		_ = db.Close()
	})
	return db, mock
}

func TestSQLRepositorySaveAccountCheckResultPersistsSchedulerFact(t *testing.T) {
	db, mock := newPuritySQLMock(t)
	repo := NewSQLRepository(db)
	capturedAt := time.Date(2026, 7, 9, 9, 30, 0, 0, time.UTC)
	runID := "manual-purity-account-42-1783589400000000000"
	report := &PublicReport{
		Provider:           ProviderOpenAI,
		ReportID:           "report-42",
		ModelID:            "gpt-5.4",
		Status:             RunStatusDone,
		Verdict:            VerdictOfficialOpenAI,
		Score:              96,
		Total:              96,
		OfficialScore:      96,
		CompatibilityScore: 94,
		Summary:            "ok",
		CheckedAt:          capturedAt,
		Metrics:            PublicCheckMetrics{Usage: &TokenUsage{InputTokens: 11, OutputTokens: 7, CachedTokens: 3}},
		TokenAudit:         &TokenAuditReport{Status: CheckStatusPass, Summary: "usage ok"},
		ModelIdentity:      &ModelIdentityResult{Status: "match", Reason: "same model", ResponseModel: "gpt-5.4", ResponseVendor: "openai"},
	}

	mock.ExpectQuery(`FROM admin_plus_supplier_accounts`).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"supplier_id", "name"}).AddRow(int64(7), "Lime"))
	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO admin_plus_scheduler_runs`).
		WithArgs(runID, runID, capturedAt, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO admin_plus_scheduler_steps`).
		WithArgs(runID, int64(7), "Lime", capturedAt, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := repo.SaveAccountCheckResult(context.Background(), AccountCheckRecord{
		AccountID:  42,
		Provider:   ProviderOpenAI,
		Report:     report,
		CapturedAt: capturedAt,
	})

	require.NoError(t, err)
}

func TestAccountPurityReportSnapshotIncludesCandidateFields(t *testing.T) {
	checkedAt := time.Date(2026, 7, 9, 9, 30, 0, 0, time.UTC)
	snapshot := accountPurityReportSnapshot(&PublicReport{
		ReportID:           "report-42",
		ModelID:            "gpt-5.4",
		Status:             RunStatusDone,
		Verdict:            VerdictOfficialOpenAI,
		Score:              96,
		OfficialScore:      96,
		CompatibilityScore: 94,
		Summary:            "ok",
		CheckedAt:          checkedAt,
		TokenAudit:         &TokenAuditReport{Status: CheckStatusPass, Summary: "usage ok"},
		ModelIdentity:      &ModelIdentityResult{Status: "match", Reason: "same model", ResponseModel: "gpt-5.4", ResponseVendor: "openai"},
	}, 42, ProviderOpenAI, checkedAt)

	require.Equal(t, int64(42), snapshot["local_sub2api_account_id"])
	require.Equal(t, ProviderOpenAI, snapshot["provider"])
	require.Equal(t, "report-42", snapshot["report_id"])
	require.Equal(t, VerdictOfficialOpenAI, snapshot["verdict"])
	require.Equal(t, 96, snapshot["score"])
	require.Equal(t, CheckStatusPass, snapshot["token_audit_status"])
	require.Equal(t, "match", snapshot["model_identity_status"])
	require.Equal(t, checkedAt.Format(time.RFC3339), snapshot["checked_at"])
}
