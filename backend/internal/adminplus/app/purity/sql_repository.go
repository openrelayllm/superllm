package purity

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type SQLRepository struct {
	db *sql.DB
}

func NewSQLRepository(db *sql.DB) *SQLRepository {
	return &SQLRepository{db: db}
}

func (r *SQLRepository) SavePublicReport(ctx context.Context, record PublicReportRecord) error {
	if r == nil || r.db == nil || record.Report == nil {
		return nil
	}
	checksJSON, err := json.Marshal(record.Report.Checks)
	if err != nil {
		return err
	}
	metricsJSON, err := json.Marshal(record.Report.Metrics)
	if err != nil {
		return err
	}
	summaryJSON, err := json.Marshal(record.PublicSummaryJSON)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO admin_plus_purity_public_reports (
			request_hash, provider, api_base_host, score, verdict,
			checks_json, metrics_json, public_summary_json, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7::jsonb, $8::jsonb, $9)
	`,
		record.RequestHash,
		record.Provider,
		record.APIBaseHost,
		record.Report.Score,
		record.Report.Verdict,
		string(checksJSON),
		string(metricsJSON),
		string(summaryJSON),
		record.Report.CheckedAt,
	)
	return err
}

func (r *SQLRepository) SaveAccountCheckResult(ctx context.Context, record AccountCheckRecord) error {
	if r == nil || r.db == nil || record.AccountID <= 0 || record.Report == nil {
		return nil
	}

	var supplierID int64
	var supplierName string
	err := r.db.QueryRowContext(ctx, `
		SELECT asa.supplier_id, COALESCE(s.name, '')
		FROM admin_plus_supplier_accounts asa
		LEFT JOIN admin_plus_suppliers s ON s.id = asa.supplier_id
		WHERE asa.local_sub2api_account_id = $1
		ORDER BY CASE WHEN asa.runtime_status = 'active' THEN 0 ELSE 1 END, asa.updated_at DESC, asa.id DESC
		LIMIT 1
	`, record.AccountID).Scan(&supplierID, &supplierName)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}

	capturedAt := record.CapturedAt.UTC()
	if capturedAt.IsZero() {
		capturedAt = record.Report.CheckedAt.UTC()
	}
	if capturedAt.IsZero() {
		capturedAt = time.Now().UTC()
	}
	provider := normalizeProvider(firstNonEmptyString(record.Provider, record.Report.Provider))
	runID := fmt.Sprintf("manual-purity-account-%d-%d", record.AccountID, capturedAt.UnixNano())
	requestSnapshot := accountPurityRequestSnapshot(record, provider)
	resultSnapshot := accountPurityReportSnapshot(record.Report, record.AccountID, provider, capturedAt)
	requestJSON, err := json.Marshal(requestSnapshot)
	if err != nil {
		return err
	}
	resultJSON, err := json.Marshal(resultSnapshot)
	if err != nil {
		return err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO admin_plus_scheduler_runs (
			id, legacy_run_id, trigger_type, task_type, status,
			requested_at, started_at, finished_at,
			supplier_count, total_steps, succeeded_steps, failed_steps, skipped_steps,
			duration_ms, error_code, error_message,
			request_snapshot, result_snapshot
		)
		VALUES ($1, $2, 'manual', 'supplier.purity.check', 'succeeded',
			$3, $3, $3,
			1, 1, 1, 0, 0,
			0, '', '',
			$4::jsonb, $5::jsonb
		)
	`, runID, runID, capturedAt, string(requestJSON), string(resultJSON)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO admin_plus_scheduler_steps (
			run_id, supplier_id, supplier_name, task_type, action, status,
			schedule_key, result_count, reason, attempts, max_attempts, next_attempt_at,
			request_snapshot, result_snapshot, started_at, finished_at
		)
		VALUES ($1, $2, $3, 'run_purity_check', 'supplier.purity.check', 'succeeded',
			'manual.local_account_purity', 1, '', 1, 1, $4,
			$5::jsonb, $6::jsonb, $4, $4
		)
	`, runID, supplierID, supplierName, capturedAt, string(requestJSON), string(resultJSON)); err != nil {
		return err
	}
	return tx.Commit()
}

func accountPurityRequestSnapshot(record AccountCheckRecord, provider string) map[string]any {
	model := ""
	if record.Report != nil {
		model = record.Report.ModelID
	}
	return map[string]any{
		"source":                   "local_account_purity_modal",
		"local_sub2api_account_id": record.AccountID,
		"provider":                 provider,
		"model":                    model,
	}
}

func accountPurityReportSnapshot(report *PublicReport, accountID int64, provider string, capturedAt time.Time) map[string]any {
	if report == nil {
		return map[string]any{
			"local_sub2api_account_id": accountID,
			"provider":                 provider,
			"checked_at":               capturedAt.UTC().Format(time.RFC3339),
		}
	}
	checkedAt := report.CheckedAt.UTC()
	if checkedAt.IsZero() {
		checkedAt = capturedAt.UTC()
	}
	out := map[string]any{
		"local_sub2api_account_id": accountID,
		"provider":                 provider,
		"report_id":                report.ReportID,
		"model":                    report.ModelID,
		"status":                   report.Status,
		"verdict":                  report.Verdict,
		"score":                    report.Score,
		"total":                    report.Total,
		"official_score":           report.OfficialScore,
		"compatibility_score":      report.CompatibilityScore,
		"summary":                  report.Summary,
		"checked_at":               checkedAt.Format(time.RFC3339),
	}
	if report.Metrics.Usage != nil {
		out["input_tokens"] = report.Metrics.Usage.InputTokens
		out["output_tokens"] = report.Metrics.Usage.OutputTokens
		out["cached_tokens"] = report.Metrics.Usage.CachedTokens
	}
	if report.TokenAudit != nil {
		out["token_audit_status"] = report.TokenAudit.Status
		out["token_audit_summary"] = report.TokenAudit.Summary
	}
	if report.ModelIdentity != nil {
		out["model_identity_status"] = report.ModelIdentity.Status
		out["model_identity_reason"] = report.ModelIdentity.Reason
		out["response_model"] = report.ModelIdentity.ResponseModel
		out["response_vendor"] = report.ModelIdentity.ResponseVendor
	}
	return out
}
