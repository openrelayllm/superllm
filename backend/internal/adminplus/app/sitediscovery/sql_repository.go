package sitediscovery

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/lib/pq"
)

type SQLRepository struct {
	db *sql.DB
}

func NewSQLRepository(db *sql.DB) *SQLRepository {
	return &SQLRepository{db: db}
}

func (r *SQLRepository) GetSettings(ctx context.Context) (*adminplusdomain.SiteDiscoverySettings, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT registration_email, registration_enabled, low_rate_threshold, updated_at
		FROM admin_plus_site_discovery_settings
		WHERE id = TRUE
	`)
	var settings adminplusdomain.SiteDiscoverySettings
	err := row.Scan(&settings.RegistrationEmail, &settings.RegistrationEnabled, &settings.LowRateThreshold, &settings.UpdatedAt)
	if err == sql.ErrNoRows {
		return &adminplusdomain.SiteDiscoverySettings{LowRateThreshold: defaultLowRateThreshold}, nil
	}
	return &settings, err
}

func (r *SQLRepository) UpdateSettings(ctx context.Context, settings adminplusdomain.SiteDiscoverySettings) (*adminplusdomain.SiteDiscoverySettings, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO admin_plus_site_discovery_settings (
			id, registration_email, registration_enabled, low_rate_threshold, updated_at
		)
		VALUES (TRUE, $1, $2, $3, $4)
		ON CONFLICT (id) DO UPDATE
		SET registration_email = EXCLUDED.registration_email,
			registration_enabled = EXCLUDED.registration_enabled,
			low_rate_threshold = EXCLUDED.low_rate_threshold,
			updated_at = EXCLUDED.updated_at
		RETURNING registration_email, registration_enabled, low_rate_threshold, updated_at
	`, settings.RegistrationEmail, settings.RegistrationEnabled, settings.LowRateThreshold, settings.UpdatedAt)
	var out adminplusdomain.SiteDiscoverySettings
	err := row.Scan(&out.RegistrationEmail, &out.RegistrationEnabled, &out.LowRateThreshold, &out.UpdatedAt)
	return &out, err
}

func (r *SQLRepository) CreateRun(ctx context.Context, run *adminplusdomain.SiteDiscoveryRun) (*adminplusdomain.SiteDiscoveryRun, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO admin_plus_site_discovery_runs (
			source_url, status, total, supported_total, imported_total,
			error_message, started_at, finished_at, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, source_url, status, total, supported_total, imported_total,
			error_message, started_at, finished_at, created_at
	`,
		run.SourceURL,
		string(run.Status),
		run.Total,
		run.SupportedTotal,
		run.ImportedTotal,
		run.ErrorMessage,
		run.StartedAt,
		nullableTimePtr(run.FinishedAt),
		run.CreatedAt,
	)
	return scanRun(row)
}

func (r *SQLRepository) UpdateRun(ctx context.Context, run *adminplusdomain.SiteDiscoveryRun) (*adminplusdomain.SiteDiscoveryRun, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	row := r.db.QueryRowContext(ctx, `
		UPDATE admin_plus_site_discovery_runs
		SET status = $2,
			total = $3,
			supported_total = $4,
			imported_total = $5,
			error_message = $6,
			finished_at = $7
		WHERE id = $1
		RETURNING id, source_url, status, total, supported_total, imported_total,
			error_message, started_at, finished_at, created_at
	`,
		run.ID,
		string(run.Status),
		run.Total,
		run.SupportedTotal,
		run.ImportedTotal,
		run.ErrorMessage,
		nullableTimePtr(run.FinishedAt),
	)
	return scanRun(row)
}

func (r *SQLRepository) FindExistingItem(ctx context.Context, sourceURL string, sourceSiteID string, registerURL string) (*adminplusdomain.SiteDiscoveryItem, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	where := make([]string, 0, 2)
	args := make([]any, 0, 3)
	addArg := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}
	if strings.TrimSpace(registerURL) != "" {
		where = append(where, "d.register_url = "+addArg(strings.TrimSpace(registerURL)))
	}
	if strings.TrimSpace(sourceURL) != "" && strings.TrimSpace(sourceSiteID) != "" {
		where = append(where, "(d.source_url = "+addArg(strings.TrimSpace(sourceURL))+" AND d.source_site_id = "+addArg(strings.TrimSpace(sourceSiteID))+")")
	}
	if len(where) == 0 {
		return nil, nil
	}
	row := r.db.QueryRowContext(ctx, siteDiscoverySelectClause()+`
		WHERE `+strings.Join(where, " OR ")+`
		ORDER BY d.updated_at DESC, d.id DESC
		LIMIT 1
	`, args...)
	item, err := scanSiteDiscoveryItem(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return item, err
}

func (r *SQLRepository) UpsertItem(ctx context.Context, item *adminplusdomain.SiteDiscoveryItem) (*adminplusdomain.SiteDiscoveryItem, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	evidence, err := json.Marshal(item.ClassificationEvidence)
	if err != nil {
		return nil, err
	}
	raw, err := marshalMap(item.RawPayload)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(item.SourceSiteID) != "" && strings.TrimSpace(item.RegisterURL) != "" {
		updated, err := r.updateItemByRegisterURL(ctx, item, evidence, raw)
		if err == nil {
			return updated, nil
		}
		if err != sql.ErrNoRows {
			return nil, err
		}
	}
	conflictClause := `
		ON CONFLICT (source_url, source_site_id) WHERE source_site_id <> '' DO UPDATE
	`
	if strings.TrimSpace(item.SourceSiteID) == "" {
		conflictClause = `
		ON CONFLICT (register_url) WHERE register_url <> '' DO UPDATE
	`
	}
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO admin_plus_site_discoveries (
			run_id, source_url, source_site_id, source_section, source_category, name, register_url,
			dashboard_url, api_base_url, host, domain_hint, description,
			provider_type, classification_status, classification_confidence, classification_evidence,
			monitor_status, monitor_available, monitor_uptime_percent, monitor_avg_response_ms,
			monitor_latest_response_ms, import_status, process_status, catalog_site_id, supplier_id, raw_payload, created_at, updated_at
		)
		VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11, $12,
			$13, $14, $15, $16,
			$17, $18, $19, $20,
			$21, $22, $23, NULLIF($24, 0), NULLIF($25, 0), $26, NOW(), NOW()
		)
		`+conflictClause+`
		SET run_id = EXCLUDED.run_id,
			source_section = EXCLUDED.source_section,
			source_category = EXCLUDED.source_category,
			name = EXCLUDED.name,
			register_url = EXCLUDED.register_url,
			dashboard_url = EXCLUDED.dashboard_url,
			api_base_url = EXCLUDED.api_base_url,
			host = EXCLUDED.host,
			domain_hint = EXCLUDED.domain_hint,
			description = EXCLUDED.description,
			provider_type = EXCLUDED.provider_type,
			classification_status = EXCLUDED.classification_status,
			classification_confidence = EXCLUDED.classification_confidence,
			classification_evidence = EXCLUDED.classification_evidence,
			monitor_status = EXCLUDED.monitor_status,
			monitor_available = EXCLUDED.monitor_available,
			monitor_uptime_percent = EXCLUDED.monitor_uptime_percent,
			monitor_avg_response_ms = EXCLUDED.monitor_avg_response_ms,
			monitor_latest_response_ms = EXCLUDED.monitor_latest_response_ms,
			process_status = CASE
				WHEN admin_plus_site_discoveries.process_status = 'unprocessed' THEN EXCLUDED.process_status
				ELSE admin_plus_site_discoveries.process_status
			END,
			raw_payload = EXCLUDED.raw_payload,
			updated_at = NOW()
		`+siteDiscoveryReturningClause(),
		siteDiscoveryItemWriteArgs(item, evidence, raw)...,
	)
	created, err := scanSiteDiscoveryItem(row)
	if err == nil {
		return created, nil
	}
	if isSiteDiscoveryRegisterURLUniqueViolation(err) && strings.TrimSpace(item.RegisterURL) != "" {
		return r.updateItemByRegisterURL(ctx, item, evidence, raw)
	}
	return nil, err
}

func (r *SQLRepository) updateItemByRegisterURL(ctx context.Context, item *adminplusdomain.SiteDiscoveryItem, evidence []byte, raw []byte) (*adminplusdomain.SiteDiscoveryItem, error) {
	row := r.db.QueryRowContext(ctx, `
		UPDATE admin_plus_site_discoveries d
		SET run_id = $1,
			source_url = $2,
			source_site_id = CASE
				WHEN $3 = '' THEN d.source_site_id
				WHEN NOT EXISTS (
					SELECT 1
					FROM admin_plus_site_discoveries existing
					WHERE existing.source_url = $2
						AND existing.source_site_id = $3
						AND existing.id <> d.id
				) THEN $3
				ELSE d.source_site_id
			END,
			source_section = $4,
			source_category = $5,
			name = $6,
			register_url = $7,
			dashboard_url = $8,
			api_base_url = $9,
			host = $10,
			domain_hint = $11,
			description = $12,
			provider_type = $13,
			classification_status = $14,
			classification_confidence = $15,
			classification_evidence = $16,
			monitor_status = $17,
			monitor_available = $18,
			monitor_uptime_percent = $19,
			monitor_avg_response_ms = $20,
			monitor_latest_response_ms = $21,
			import_status = CASE
				WHEN d.import_status = 'new' THEN $22
				ELSE d.import_status
			END,
			process_status = CASE
				WHEN d.process_status = 'unprocessed' THEN $23
				ELSE d.process_status
			END,
			catalog_site_id = COALESCE(NULLIF($24::bigint, 0), d.catalog_site_id),
			supplier_id = COALESCE(NULLIF($25::bigint, 0), d.supplier_id),
			raw_payload = $26,
			updated_at = NOW()
		WHERE d.register_url = $7
		`+siteDiscoveryReturningClause(),
		siteDiscoveryItemWriteArgs(item, evidence, raw)...,
	)
	return scanSiteDiscoveryItem(row)
}

func (r *SQLRepository) GetItem(ctx context.Context, id int64) (*adminplusdomain.SiteDiscoveryItem, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	row := r.db.QueryRowContext(ctx, siteDiscoverySelectClause()+`
		WHERE d.id = $1
	`, id)
	item, err := scanSiteDiscoveryItem(row)
	if err == sql.ErrNoRows {
		return nil, infraerrors.New(http.StatusNotFound, "SITE_DISCOVERY_ITEM_NOT_FOUND", "site discovery item not found")
	}
	return item, err
}

func (r *SQLRepository) ListItems(ctx context.Context, filter ListFilter) ([]*adminplusdomain.SiteDiscoveryItem, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	where := []string{"1=1"}
	args := make([]any, 0, 6)
	addArg := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}
	if filter.Query != "" {
		query := "%" + strings.ToLower(filter.Query) + "%"
		where = append(where, "(LOWER(discovery.name) LIKE "+addArg(query)+" OR LOWER(discovery.host) LIKE "+addArg(query)+" OR LOWER(discovery.description) LIKE "+addArg(query)+")")
	}
	if filter.ProviderType != "" {
		where = append(where, "discovery.provider_type = "+addArg(string(filter.ProviderType)))
	}
	if filter.ClassificationStatus != "" {
		where = append(where, "discovery.classification_status = "+addArg(string(filter.ClassificationStatus)))
	}
	if filter.ImportStatus != "" {
		where = append(where, "discovery.import_status = "+addArg(string(filter.ImportStatus)))
	}
	if filter.RegistrationStatus != "" {
		where = append(where, "discovery.registration_status = "+addArg(string(filter.RegistrationStatus)))
	}
	switch filter.ProcessedStatus {
	case "unprocessed":
		where = append(where, "discovery.process_status = 'unprocessed' AND discovery.import_status = 'new' AND COALESCE(discovery.registration_status, '') = ''")
	case "processed":
		where = append(where, "(discovery.process_status <> 'unprocessed' OR discovery.import_status <> 'new' OR COALESCE(discovery.registration_status, '') <> '')")
	}
	limitRef := addArg(filter.Limit)
	query := `
		SELECT *
		FROM (` + siteDiscoverySelectClause() + `) discovery
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY
			CASE
				WHEN discovery.provider_type = 'new_api' THEN 0
				WHEN discovery.provider_type = 'sub2api' THEN 1
				WHEN discovery.classification_status = 'supported' THEN 2
				ELSE 3
			END,
			discovery.updated_at DESC,
			discovery.id DESC
		LIMIT ` + limitRef

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]*adminplusdomain.SiteDiscoveryItem, 0)
	for rows.Next() {
		item, err := scanSiteDiscoveryItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *SQLRepository) LinkSupplier(ctx context.Context, itemID int64, supplierID int64) (*adminplusdomain.SiteDiscoveryItem, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	row := r.db.QueryRowContext(ctx, `
		UPDATE admin_plus_site_discoveries
		SET supplier_id = $2,
			import_status = 'imported',
			process_status = CASE
				WHEN process_status = 'unprocessed' THEN 'registered'
				ELSE process_status
			END,
			updated_at = NOW()
		WHERE id = $1
		`+siteDiscoveryReturningClause(),
		itemID,
		supplierID,
	)
	item, err := scanSiteDiscoveryItem(row)
	if err == sql.ErrNoRows {
		return nil, infraerrors.New(http.StatusNotFound, "SITE_DISCOVERY_ITEM_NOT_FOUND", "site discovery item not found")
	}
	return item, err
}

func (r *SQLRepository) UpsertRegistrationCredential(ctx context.Context, credential *adminplusdomain.SupplierRegistrationCredential) (*adminplusdomain.SupplierRegistrationCredential, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO admin_plus_supplier_registration_credentials (
			discovery_id, supplier_id, email, password_ciphertext, status,
			verification_status, extension_task_id, error_code, error_message,
			last_attempt_at, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, 0), $8, $9, $10, $11, $12)
		ON CONFLICT (discovery_id) DO UPDATE
		SET supplier_id = EXCLUDED.supplier_id,
			email = EXCLUDED.email,
			password_ciphertext = EXCLUDED.password_ciphertext,
			status = EXCLUDED.status,
			verification_status = EXCLUDED.verification_status,
			extension_task_id = EXCLUDED.extension_task_id,
			error_code = EXCLUDED.error_code,
			error_message = EXCLUDED.error_message,
			last_attempt_at = EXCLUDED.last_attempt_at,
			updated_at = EXCLUDED.updated_at
		RETURNING id, discovery_id, supplier_id, email, password_ciphertext, status,
			verification_status, extension_task_id, error_code, error_message,
			last_attempt_at, created_at, updated_at
	`,
		credential.DiscoveryID,
		sql.NullInt64{Int64: credential.SupplierID, Valid: credential.SupplierID > 0},
		credential.Email,
		credential.PasswordCiphertext,
		string(credential.Status),
		credential.VerificationStatus,
		credential.ExtensionTaskID,
		credential.ErrorCode,
		credential.ErrorMessage,
		nullableTimePtr(credential.LastAttemptAt),
		credential.CreatedAt,
		credential.UpdatedAt,
	)
	return scanRegistrationCredential(row)
}

func (r *SQLRepository) UpdateRegistrationTask(ctx context.Context, credentialID int64, taskID int64, status adminplusdomain.SupplierRegistrationStatus, attemptedAt time.Time) (*adminplusdomain.SupplierRegistrationCredential, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	row := r.db.QueryRowContext(ctx, `
		UPDATE admin_plus_supplier_registration_credentials
		SET extension_task_id = $2,
			status = $3,
			last_attempt_at = $4,
			updated_at = NOW()
		WHERE id = $1
		RETURNING id, discovery_id, supplier_id, email, password_ciphertext, status,
			verification_status, extension_task_id, error_code, error_message,
			last_attempt_at, created_at, updated_at
	`, credentialID, taskID, string(status), nullableTimeValue(attemptedAt))
	return scanRegistrationCredential(row)
}

func (r *SQLRepository) CompleteRegistration(ctx context.Context, credentialID int64, supplierID int64, status adminplusdomain.SupplierRegistrationStatus, errorCode string, errorMessage string, attemptedAt time.Time) (*adminplusdomain.SupplierRegistrationCredential, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	row := r.db.QueryRowContext(ctx, `
		UPDATE admin_plus_supplier_registration_credentials
		SET supplier_id = COALESCE(NULLIF($2, 0), supplier_id),
			status = $3,
			verification_status = CASE WHEN $3 = 'waiting_manual_verification' THEN $4 ELSE '' END,
			error_code = $4,
			error_message = $5,
			last_attempt_at = $6,
			updated_at = NOW()
		WHERE id = $1
		RETURNING id, discovery_id, supplier_id, email, password_ciphertext, status,
			verification_status, extension_task_id, error_code, error_message,
			last_attempt_at, created_at, updated_at
	`, credentialID, supplierID, string(status), strings.TrimSpace(errorCode), trimLimit(errorMessage, 1000), nullableTimeValue(attemptedAt))
	credential, err := scanRegistrationCredential(row)
	if err == sql.ErrNoRows {
		return nil, infraerrors.New(http.StatusNotFound, "SITE_DISCOVERY_REGISTRATION_CREDENTIAL_NOT_FOUND", "registration credential not found")
	}
	return credential, err
}

func (r *SQLRepository) GetRegistrationCredentialByTaskID(ctx context.Context, taskID int64) (*adminplusdomain.SupplierRegistrationCredential, *adminplusdomain.SiteDiscoveryItem, error) {
	if r == nil || r.db == nil {
		return nil, nil, dbNotConfigured()
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT rc.id, rc.discovery_id, rc.supplier_id, rc.email, rc.password_ciphertext, rc.status,
			rc.verification_status, rc.extension_task_id, rc.error_code, rc.error_message,
			rc.last_attempt_at, rc.created_at, rc.updated_at,
			`+siteDiscoveryColumnList("d")+`
		FROM admin_plus_supplier_registration_credentials rc
		INNER JOIN admin_plus_site_discoveries d ON d.id = rc.discovery_id
		WHERE rc.extension_task_id = $1
	`, taskID)
	credential, item, err := scanRegistrationCredentialWithItem(row)
	if err == sql.ErrNoRows {
		return nil, nil, infraerrors.New(http.StatusNotFound, "SITE_DISCOVERY_REGISTRATION_CREDENTIAL_NOT_FOUND", "registration credential not found")
	}
	return credential, item, err
}

func (r *SQLRepository) ListRecommendations(ctx context.Context, threshold float64, limit int) ([]*adminplusdomain.SiteDiscoveryRecommendation, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	rows, err := r.db.QueryContext(ctx, `
		WITH supplier_rates AS (
			SELECT supplier_id,
				MIN(NULLIF(CASE
					WHEN effective_rate_multiplier > 0 THEN effective_rate_multiplier
					WHEN rate_multiplier > 0 THEN rate_multiplier
					ELSE 0
				END, 0)) AS min_rate_multiplier
			FROM admin_plus_supplier_groups
			GROUP BY supplier_id
		),
		channel_recommendations AS (
			SELECT supplier_id, COUNT(*) AS recommended_channels
			FROM admin_plus_supplier_channel_check_snapshots
			WHERE recommended = TRUE
			GROUP BY supplier_id
		)
		SELECT `+siteDiscoveryColumnList("d")+`,
			sr.min_rate_multiplier,
			COALESCE(cr.recommended_channels, 0)
		FROM admin_plus_site_discoveries d
		INNER JOIN supplier_rates sr ON sr.supplier_id = d.supplier_id
		LEFT JOIN channel_recommendations cr ON cr.supplier_id = d.supplier_id
		WHERE d.import_status = 'imported'
			AND d.provider_type IN ('new_api', 'sub2api')
			AND sr.min_rate_multiplier <= $1
			AND (d.monitor_available IS NULL OR d.monitor_available = TRUE)
		ORDER BY sr.min_rate_multiplier ASC, COALESCE(cr.recommended_channels, 0) DESC, d.updated_at DESC
		LIMIT $2
	`, threshold, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]*adminplusdomain.SiteDiscoveryRecommendation, 0)
	for rows.Next() {
		item, minRate, channels, err := scanRecommendation(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, &adminplusdomain.SiteDiscoveryRecommendation{
			Item:                item,
			MinRateMultiplier:   minRate,
			RecommendedChannels: channels,
			Reason:              "rate_below_threshold",
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

type runScanner interface {
	Scan(dest ...any) error
}

func scanRun(scanner runScanner) (*adminplusdomain.SiteDiscoveryRun, error) {
	var run adminplusdomain.SiteDiscoveryRun
	var status string
	var finished sql.NullTime
	err := scanner.Scan(
		&run.ID,
		&run.SourceURL,
		&status,
		&run.Total,
		&run.SupportedTotal,
		&run.ImportedTotal,
		&run.ErrorMessage,
		&run.StartedAt,
		&finished,
		&run.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	run.Status = adminplusdomain.SiteDiscoveryRunStatus(status)
	if finished.Valid {
		t := finished.Time
		run.FinishedAt = &t
	}
	return &run, nil
}

type itemScanner interface {
	Scan(dest ...any) error
}

func scanSiteDiscoveryItem(scanner itemScanner) (*adminplusdomain.SiteDiscoveryItem, error) {
	item, err := scanSiteDiscoveryItemColumns(scanner, true)
	return item, err
}

func scanSiteDiscoveryItemColumns(scanner itemScanner, withRegistration bool) (*adminplusdomain.SiteDiscoveryItem, error) {
	var item adminplusdomain.SiteDiscoveryItem
	var providerType, classificationStatus, importStatus, processStatus string
	var evidence, raw []byte
	var monitorAvailable sql.NullBool
	var monitorUptime sql.NullFloat64
	var monitorAvg, monitorLatest sql.NullInt64
	var catalogSiteID, supplierID sql.NullInt64
	dests := []any{
		&item.ID,
		&item.RunID,
		&item.SourceURL,
		&item.SourceSiteID,
		&item.SourceSection,
		&item.SourceCategory,
		&item.Name,
		&item.RegisterURL,
		&item.DashboardURL,
		&item.APIBaseURL,
		&item.Host,
		&item.DomainHint,
		&item.Description,
		&providerType,
		&classificationStatus,
		&item.ClassificationConfidence,
		&evidence,
		&item.MonitorStatus,
		&monitorAvailable,
		&monitorUptime,
		&monitorAvg,
		&monitorLatest,
		&importStatus,
		&processStatus,
		&catalogSiteID,
		&supplierID,
		&raw,
		&item.CreatedAt,
		&item.UpdatedAt,
	}
	var registrationStatus, registrationEmail, registrationErrorCode, registrationErrorMessage sql.NullString
	var registrationTaskID sql.NullInt64
	if withRegistration {
		dests = append(dests, &registrationStatus, &registrationTaskID, &registrationEmail, &registrationErrorCode, &registrationErrorMessage)
	}
	if err := scanner.Scan(dests...); err != nil {
		return nil, err
	}
	item.ProviderType = adminplusdomain.SupplierType(providerType)
	item.ClassificationStatus = adminplusdomain.SiteDiscoveryClassificationStatus(classificationStatus)
	item.ImportStatus = adminplusdomain.SiteDiscoveryImportStatus(importStatus)
	item.ProcessStatus = adminplusdomain.SiteDiscoveryProcessStatus(processStatus)
	if len(evidence) > 0 {
		_ = json.Unmarshal(evidence, &item.ClassificationEvidence)
	}
	if monitorAvailable.Valid {
		v := monitorAvailable.Bool
		item.MonitorAvailable = &v
	}
	if monitorUptime.Valid {
		v := monitorUptime.Float64
		item.MonitorUptimePercent = &v
	}
	if monitorAvg.Valid {
		v := int(monitorAvg.Int64)
		item.MonitorAvgResponseMS = &v
	}
	if monitorLatest.Valid {
		v := int(monitorLatest.Int64)
		item.MonitorLatestResponseMS = &v
	}
	if catalogSiteID.Valid {
		item.CatalogSiteID = catalogSiteID.Int64
	}
	if supplierID.Valid {
		item.SupplierID = supplierID.Int64
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &item.RawPayload)
	}
	if withRegistration {
		if registrationStatus.Valid {
			item.RegistrationStatus = adminplusdomain.SupplierRegistrationStatus(registrationStatus.String)
		}
		if registrationTaskID.Valid {
			item.RegistrationTaskID = registrationTaskID.Int64
		}
		if registrationEmail.Valid {
			item.RegistrationEmail = registrationEmail.String
		}
		if registrationErrorCode.Valid {
			item.RegistrationErrorCode = registrationErrorCode.String
		}
		if registrationErrorMessage.Valid {
			item.RegistrationErrorMessage = registrationErrorMessage.String
		}
	}
	return &item, nil
}

func scanRegistrationCredential(scanner itemScanner) (*adminplusdomain.SupplierRegistrationCredential, error) {
	var credential adminplusdomain.SupplierRegistrationCredential
	var status string
	var supplierID sql.NullInt64
	var extensionTaskID sql.NullInt64
	var lastAttemptAt sql.NullTime
	err := scanner.Scan(
		&credential.ID,
		&credential.DiscoveryID,
		&supplierID,
		&credential.Email,
		&credential.PasswordCiphertext,
		&status,
		&credential.VerificationStatus,
		&extensionTaskID,
		&credential.ErrorCode,
		&credential.ErrorMessage,
		&lastAttemptAt,
		&credential.CreatedAt,
		&credential.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	credential.Status = adminplusdomain.SupplierRegistrationStatus(status)
	credential.PasswordConfigured = credential.PasswordCiphertext != ""
	if supplierID.Valid {
		credential.SupplierID = supplierID.Int64
	}
	if extensionTaskID.Valid {
		credential.ExtensionTaskID = extensionTaskID.Int64
	}
	if lastAttemptAt.Valid {
		t := lastAttemptAt.Time
		credential.LastAttemptAt = &t
	}
	return &credential, nil
}

func scanRegistrationCredentialWithItem(scanner itemScanner) (*adminplusdomain.SupplierRegistrationCredential, *adminplusdomain.SiteDiscoveryItem, error) {
	var credential adminplusdomain.SupplierRegistrationCredential
	var status string
	var supplierID sql.NullInt64
	var extensionTaskID sql.NullInt64
	var lastAttemptAt sql.NullTime
	item, err := scanSiteDiscoveryItemColumns(registrationItemScanner{scanner: scanner, prefix: []any{
		&credential.ID,
		&credential.DiscoveryID,
		&supplierID,
		&credential.Email,
		&credential.PasswordCiphertext,
		&status,
		&credential.VerificationStatus,
		&extensionTaskID,
		&credential.ErrorCode,
		&credential.ErrorMessage,
		&lastAttemptAt,
		&credential.CreatedAt,
		&credential.UpdatedAt,
	}}, false)
	if err != nil {
		return nil, nil, err
	}
	credential.Status = adminplusdomain.SupplierRegistrationStatus(status)
	credential.PasswordConfigured = credential.PasswordCiphertext != ""
	if supplierID.Valid {
		credential.SupplierID = supplierID.Int64
	}
	if extensionTaskID.Valid {
		credential.ExtensionTaskID = extensionTaskID.Int64
	}
	if lastAttemptAt.Valid {
		t := lastAttemptAt.Time
		credential.LastAttemptAt = &t
	}
	return &credential, item, nil
}

type registrationItemScanner struct {
	scanner itemScanner
	prefix  []any
}

func (s registrationItemScanner) Scan(dest ...any) error {
	combined := append(append([]any{}, s.prefix...), dest...)
	return s.scanner.Scan(combined...)
}

func scanRecommendation(scanner itemScanner) (*adminplusdomain.SiteDiscoveryItem, float64, int, error) {
	var minRate sql.NullFloat64
	var channels sql.NullInt64
	item, err := scanSiteDiscoveryItemColumns(recommendationScanner{scanner: scanner, suffix: []any{&minRate, &channels}}, false)
	if err != nil {
		return nil, 0, 0, err
	}
	rate := 0.0
	if minRate.Valid {
		rate = minRate.Float64
	}
	recommendedChannels := 0
	if channels.Valid {
		recommendedChannels = int(channels.Int64)
	}
	return item, rate, recommendedChannels, nil
}

type recommendationScanner struct {
	scanner itemScanner
	suffix  []any
}

func (s recommendationScanner) Scan(dest ...any) error {
	combined := append(append([]any{}, dest...), s.suffix...)
	return s.scanner.Scan(combined...)
}

func siteDiscoverySelectClause() string {
	return `
		SELECT ` + siteDiscoveryColumnList("d") + `,
			CASE
				WHEN rc.registration_id IS NULL THEN ''
				WHEN rc.registration_status IN ('succeeded', 'failed', 'waiting_manual_verification') THEN rc.registration_status
				WHEN rc.task_status IN ('claimed', 'running') THEN 'running'
				WHEN rc.task_status = 'pending' THEN 'queued'
				WHEN rc.task_status = 'failed' AND rc.task_error_code = 'REGISTRATION_VERIFICATION_REQUIRED' THEN 'waiting_manual_verification'
				WHEN rc.task_status = 'failed' THEN 'failed'
				ELSE rc.registration_status
			END AS registration_status,
			rc.extension_task_id,
			rc.registration_email,
			COALESCE(NULLIF(rc.task_error_code, ''), rc.registration_error_code) AS registration_error_code,
			COALESCE(NULLIF(rc.task_error_message, ''), rc.registration_error_message) AS registration_error_message
		FROM admin_plus_site_discoveries d
		LEFT JOIN LATERAL (
			SELECT rc.id AS registration_id,
				rc.status AS registration_status,
				rc.email AS registration_email,
				rc.extension_task_id,
				rc.error_code AS registration_error_code,
				rc.error_message AS registration_error_message,
				t.status AS task_status,
				t.error_code AS task_error_code,
				t.error_message AS task_error_message
			FROM admin_plus_supplier_registration_credentials rc
			LEFT JOIN admin_plus_extension_tasks t ON t.id = rc.extension_task_id
			WHERE rc.discovery_id = d.id
			ORDER BY rc.updated_at DESC, rc.id DESC
			LIMIT 1
		) rc ON TRUE
	`
}

func siteDiscoveryReturningClause() string {
	return `
		RETURNING id, run_id, source_url, source_site_id, source_section, source_category, name, register_url,
			dashboard_url, api_base_url, host, domain_hint, description,
			provider_type, classification_status, classification_confidence, classification_evidence,
			monitor_status, monitor_available, monitor_uptime_percent, monitor_avg_response_ms,
			monitor_latest_response_ms, import_status, process_status, catalog_site_id, supplier_id, raw_payload, created_at, updated_at,
			''::text AS registration_status,
			NULL::bigint AS extension_task_id,
			''::text AS registration_email,
			''::text AS registration_error_code,
			''::text AS registration_error_message
	`
}

func siteDiscoveryColumnList(alias string) string {
	prefix := alias + "."
	return prefix + `id, ` + prefix + `run_id, ` + prefix + `source_url, ` + prefix + `source_site_id, ` + prefix + `source_section, ` + prefix + `source_category, ` + prefix + `name, ` + prefix + `register_url,
			` + prefix + `dashboard_url, ` + prefix + `api_base_url, ` + prefix + `host, ` + prefix + `domain_hint, ` + prefix + `description,
			` + prefix + `provider_type, ` + prefix + `classification_status, ` + prefix + `classification_confidence, ` + prefix + `classification_evidence,
			` + prefix + `monitor_status, ` + prefix + `monitor_available, ` + prefix + `monitor_uptime_percent, ` + prefix + `monitor_avg_response_ms,
			` + prefix + `monitor_latest_response_ms, ` + prefix + `import_status, ` + prefix + `process_status, COALESCE(` + prefix + `catalog_site_id, 0), ` + prefix + `supplier_id, ` + prefix + `raw_payload, ` + prefix + `created_at, ` + prefix + `updated_at`
}

func marshalMap(value map[string]any) ([]byte, error) {
	if len(value) == 0 {
		return []byte("{}"), nil
	}
	return json.Marshal(value)
}

func siteDiscoveryItemWriteArgs(item *adminplusdomain.SiteDiscoveryItem, evidence []byte, raw []byte) []any {
	return []any{
		item.RunID,
		item.SourceURL,
		item.SourceSiteID,
		item.SourceSection,
		item.SourceCategory,
		item.Name,
		item.RegisterURL,
		item.DashboardURL,
		item.APIBaseURL,
		item.Host,
		item.DomainHint,
		item.Description,
		string(item.ProviderType),
		string(item.ClassificationStatus),
		item.ClassificationConfidence,
		evidence,
		item.MonitorStatus,
		nullableBool(item.MonitorAvailable),
		nullableFloat(item.MonitorUptimePercent),
		nullableInt(item.MonitorAvgResponseMS),
		nullableInt(item.MonitorLatestResponseMS),
		string(item.ImportStatus),
		siteDiscoveryProcessStatusForWrite(item.ProcessStatus),
		item.CatalogSiteID,
		item.SupplierID,
		raw,
	}
}

func siteDiscoveryProcessStatusForWrite(status adminplusdomain.SiteDiscoveryProcessStatus) string {
	if status == "" {
		return string(adminplusdomain.SiteDiscoveryProcessUnprocessed)
	}
	return string(status)
}

func isSiteDiscoveryRegisterURLUniqueViolation(err error) bool {
	var pqErr *pq.Error
	if !errors.As(err, &pqErr) || pqErr.Code != "23505" {
		return false
	}
	constraint := string(pqErr.Constraint)
	return constraint == "" ||
		constraint == "idx_admin_plus_site_discoveries_register_url" ||
		strings.Contains(err.Error(), "idx_admin_plus_site_discoveries_register_url")
}

func nullableTimePtr(value *time.Time) any {
	if value == nil || value.IsZero() {
		return nil
	}
	return *value
}

func nullableTimeValue(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}

func nullableBool(value *bool) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableFloat(value *float64) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableInt(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}
