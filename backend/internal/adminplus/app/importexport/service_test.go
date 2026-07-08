package importexport

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestDefaultTableSpecsIncludeCoreButNotRuntimeLogs(t *testing.T) {
	specs := defaultTableSpecs()
	names := make(map[string]struct{}, len(specs))
	for _, spec := range specs {
		names[spec.Name] = struct{}{}
		require.NotEmpty(t, spec.ConflictColumns, spec.Name)
	}

	for _, table := range []string{
		"settings",
		"users",
		"proxies",
		"accounts",
		"api_keys",
		"admin_plus_suppliers",
		"admin_plus_supplier_keys",
		"admin_plus_scheduler_plans",
		"admin_plus_mail_credentials",
		"admin_plus_proxy_subscriptions",
	} {
		require.Contains(t, names, table)
	}

	for _, table := range []string{
		"usage_logs",
		"admin_plus_scheduler_runs",
		"admin_plus_routing_refill_runs",
		"admin_plus_extension_tasks",
		"admin_plus_proxy_audit_events",
		"admin_plus_supplier_browser_sessions",
	} {
		require.NotContains(t, names, table)
		require.NotEmpty(t, excludedReason(table))
	}
}

func TestScopeReturnsIncludedAndExcludedTables(t *testing.T) {
	svc := NewService(nil)

	scope := svc.Scope()

	require.Equal(t, ArchiveProduct, scope.Product)
	require.Equal(t, ArchiveVersion, scope.Version)
	require.NotEmpty(t, scope.Notes)
	require.Greater(t, scope.Summary.Included, 0)
	require.Greater(t, scope.Summary.Excluded, 0)
	require.Greater(t, scope.Summary.Sensitive, 0)
	require.Contains(t, scope.IncludedTables, ScopeTable{
		Name:        "settings",
		Description: "系统键值配置",
	})
	require.Contains(t, scope.ExcludedTables, ScopeTable{
		Name:   "admin_plus_scheduler_runs",
		Reason: "调度运行历史属于运行态数据",
	})
}

func TestPreviewIgnoresRuntimeAndUnknownTables(t *testing.T) {
	svc := NewService(nil)
	exportedAt := time.Date(2026, 7, 7, 0, 0, 0, 0, time.UTC)
	archive := Archive{
		Version:    ArchiveVersion,
		Product:    ArchiveProduct,
		ExportedAt: exportedAt,
		Tables: map[string][]json.RawMessage{
			"settings": {
				json.RawMessage(`{"key":"site_name","value":"Lime"}`),
			},
			"admin_plus_scheduler_runs": {
				json.RawMessage(`{"id":"run-1"}`),
				json.RawMessage(`{"id":"run-2"}`),
			},
			"custom_runtime_table": {
				json.RawMessage(`{"id":1}`),
			},
		},
	}

	preview, err := svc.Preview(context.Background(), archive)

	require.NoError(t, err)
	require.True(t, preview.Valid)
	require.Equal(t, 1, preview.Summary.Tables)
	require.Equal(t, 1, preview.Summary.Rows)
	require.Len(t, preview.IncludedTables, 1)
	require.Equal(t, "settings", preview.IncludedTables[0].Name)
	require.Len(t, preview.IgnoredTables, 2)
	require.Contains(t, preview.Warnings, "导入时会忽略白名单外的日志、运行记录、审计事件或未知表。")
}

func TestImportSettingsUpsertsByKeyAndOmitsID(t *testing.T) {
	db, mock := newImportExportSQLMock(t)
	svc := NewService(db)
	archive := Archive{
		Version: ArchiveVersion,
		Product: ArchiveProduct,
		Tables: map[string][]json.RawMessage{
			"settings": {
				json.RawMessage(`{"id":9,"key":"site_name","value":"Lime","updated_at":"2026-07-07T00:00:00Z"}`),
			},
		},
	}

	mock.ExpectBegin()
	mock.ExpectQuery(`information_schema\.tables`).
		WithArgs("settings").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectQuery(`information_schema\.columns`).
		WithArgs("settings").
		WillReturnRows(sqlmock.NewRows([]string{"column_name", "is_generated", "identity_generation"}).
			AddRow("id", "NEVER", "").
			AddRow("key", "NEVER", "").
			AddRow("value", "NEVER", "").
			AddRow("updated_at", "NEVER", ""))
	mock.ExpectExec(`INSERT INTO "settings" \("key", "updated_at", "value"\).*ON CONFLICT \("key"\) DO UPDATE`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	result, err := svc.Import(context.Background(), archive)

	require.NoError(t, err)
	require.Equal(t, 1, result.Summary.Rows)
	require.Len(t, result.Tables, 1)
	require.Equal(t, "settings", result.Tables[0].Name)
	require.Equal(t, 1, result.Tables[0].Imported)
}

func TestPreviewRejectsUnsupportedArchive(t *testing.T) {
	svc := NewService(nil)
	_, err := svc.Preview(context.Background(), Archive{
		Version: ArchiveVersion,
		Product: "metapi",
		Tables:  map[string][]json.RawMessage{},
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported archive product")
}

func newImportExportSQLMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, mock.ExpectationsWereMet())
		_ = db.Close()
	})
	return db, mock
}
