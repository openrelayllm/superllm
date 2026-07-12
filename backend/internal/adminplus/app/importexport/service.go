package importexport

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	ArchiveProduct       = "superllm"
	legacyArchiveProduct = "sub2api-admin-plus"
	ArchiveVersion       = 1
)

var errDatabaseNotConfigured = errors.New("import/export database is not configured")

type ValidationError struct {
	message string
}

func (e *ValidationError) Error() string {
	return e.message
}

func IsValidationError(err error) bool {
	var target *ValidationError
	return errors.As(err, &target)
}

type Service struct {
	db          *sql.DB
	specs       []tableSpec
	specsByName map[string]tableSpec
}

type Archive struct {
	Version    int                          `json:"version"`
	Product    string                       `json:"product"`
	ExportedAt time.Time                    `json:"exported_at"`
	Tables     map[string][]json.RawMessage `json:"tables"`
	Summary    ArchiveSummary               `json:"summary"`
}

type ArchiveSummary struct {
	Tables int            `json:"tables"`
	Rows   int            `json:"rows"`
	Items  []TableSummary `json:"items"`
}

type TableSummary struct {
	Name        string `json:"name"`
	Rows        int    `json:"rows"`
	Sensitive   bool   `json:"sensitive,omitempty"`
	Description string `json:"description,omitempty"`
}

type PreviewResult struct {
	Valid          bool           `json:"valid"`
	Product        string         `json:"product"`
	Version        int            `json:"version"`
	ExportedAt     *time.Time     `json:"exported_at,omitempty"`
	Summary        ArchiveSummary `json:"summary"`
	IncludedTables []TableSummary `json:"included_tables"`
	IgnoredTables  []IgnoredTable `json:"ignored_tables,omitempty"`
	Warnings       []string       `json:"warnings,omitempty"`
}

type ScopeResult struct {
	Product        string            `json:"product"`
	Version        int               `json:"version"`
	IncludedTables []ScopeTable      `json:"included_tables"`
	ExcludedTables []ScopeTable      `json:"excluded_tables"`
	Notes          []string          `json:"notes"`
	Summary        ScopeTableSummary `json:"summary"`
}

type ScopeTable struct {
	Name        string `json:"name"`
	Sensitive   bool   `json:"sensitive,omitempty"`
	Description string `json:"description,omitempty"`
	Reason      string `json:"reason,omitempty"`
}

type ScopeTableSummary struct {
	Included  int `json:"included"`
	Excluded  int `json:"excluded"`
	Sensitive int `json:"sensitive"`
}

type IgnoredTable struct {
	Name   string `json:"name"`
	Rows   int    `json:"rows"`
	Reason string `json:"reason"`
}

type ImportResult struct {
	Summary       ArchiveSummary      `json:"summary"`
	Tables        []ImportTableResult `json:"tables"`
	IgnoredTables []IgnoredTable      `json:"ignored_tables,omitempty"`
	Warnings      []string            `json:"warnings,omitempty"`
}

type ImportTableResult struct {
	Name     string `json:"name"`
	Rows     int    `json:"rows"`
	Imported int    `json:"imported"`
	Skipped  bool   `json:"skipped,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

type tableSpec struct {
	Name            string
	ConflictColumns []string
	SequenceColumn  string
	OrderColumns    []string
	OmitColumns     []string
	Sensitive       bool
	Description     string
}

type columnInfo struct {
	Name               string
	IsGenerated        string
	IdentityGeneration string
}

type tableQuerier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type tableExecutor interface {
	tableQuerier
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func NewService(db *sql.DB) *Service {
	specs := defaultTableSpecs()
	byName := make(map[string]tableSpec, len(specs))
	for _, spec := range specs {
		byName[spec.Name] = spec
	}
	return &Service{
		db:          db,
		specs:       specs,
		specsByName: byName,
	}
}

func (s *Service) Scope() *ScopeResult {
	result := &ScopeResult{
		Product: ArchiveProduct,
		Version: ArchiveVersion,
		Notes: []string{
			"导入采用 upsert，不会清空目标库。",
			"日志、运行记录、监控快照、成本流水和审计事件不属于核心迁移范围。",
			"敏感密文迁移后仍依赖目标服务器的同一套加密配置才能正常解密使用。",
		},
	}
	for _, spec := range s.specs {
		result.IncludedTables = append(result.IncludedTables, ScopeTable{
			Name:        spec.Name,
			Sensitive:   spec.Sensitive,
			Description: spec.Description,
		})
		result.Summary.Included++
		if spec.Sensitive {
			result.Summary.Sensitive++
		}
	}
	excluded := excludedTableReasons()
	names := make([]string, 0, len(excluded))
	for name := range excluded {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		result.ExcludedTables = append(result.ExcludedTables, ScopeTable{
			Name:   name,
			Reason: excluded[name],
		})
		result.Summary.Excluded++
	}
	return result
}

func (s *Service) Export(ctx context.Context) (*Archive, error) {
	if s == nil || s.db == nil {
		return nil, errDatabaseNotConfigured
	}

	archive := &Archive{
		Version:    ArchiveVersion,
		Product:    ArchiveProduct,
		ExportedAt: time.Now().UTC(),
		Tables:     make(map[string][]json.RawMessage, len(s.specs)),
	}

	for _, spec := range s.specs {
		exists, err := tableExists(ctx, s.db, spec.Name)
		if err != nil {
			return nil, fmt.Errorf("check table %s: %w", spec.Name, err)
		}
		if !exists {
			continue
		}

		rows, err := exportTable(ctx, s.db, spec)
		if err != nil {
			return nil, fmt.Errorf("export table %s: %w", spec.Name, err)
		}
		archive.Tables[spec.Name] = rows
		archive.Summary.Items = append(archive.Summary.Items, TableSummary{
			Name:        spec.Name,
			Rows:        len(rows),
			Sensitive:   spec.Sensitive,
			Description: spec.Description,
		})
		archive.Summary.Tables++
		archive.Summary.Rows += len(rows)
	}

	return archive, nil
}

func (s *Service) Preview(_ context.Context, archive Archive) (*PreviewResult, error) {
	if err := s.validateArchive(archive); err != nil {
		return nil, err
	}

	result := &PreviewResult{
		Valid:      true,
		Product:    archive.Product,
		Version:    archive.Version,
		ExportedAt: &archive.ExportedAt,
	}
	if archive.ExportedAt.IsZero() {
		result.ExportedAt = nil
	}

	for _, name := range sortedTableNames(archive.Tables) {
		rows := archive.Tables[name]
		spec, ok := s.specsByName[name]
		switch {
		case ok:
			item := TableSummary{
				Name:        name,
				Rows:        len(rows),
				Sensitive:   spec.Sensitive,
				Description: spec.Description,
			}
			result.IncludedTables = append(result.IncludedTables, item)
			result.Summary.Items = append(result.Summary.Items, item)
			result.Summary.Tables++
			result.Summary.Rows += len(rows)
		case excludedReason(name) != "":
			result.IgnoredTables = append(result.IgnoredTables, IgnoredTable{
				Name:   name,
				Rows:   len(rows),
				Reason: excludedReason(name),
			})
		default:
			result.IgnoredTables = append(result.IgnoredTables, IgnoredTable{
				Name:   name,
				Rows:   len(rows),
				Reason: "not in core migration allowlist",
			})
		}
	}

	if containsSensitiveTable(result.IncludedTables) {
		result.Warnings = append(result.Warnings, "导出包含账号凭据或 API Key 等敏感密文，请只在可信环境保存和传输。")
	}
	if len(result.IgnoredTables) > 0 {
		result.Warnings = append(result.Warnings, "导入时会忽略白名单外的日志、运行记录、审计事件或未知表。")
	}
	return result, nil
}

func (s *Service) Import(ctx context.Context, archive Archive) (*ImportResult, error) {
	if s == nil || s.db == nil {
		return nil, errDatabaseNotConfigured
	}
	if err := s.validateArchive(archive); err != nil {
		return nil, err
	}

	preview, err := s.Preview(ctx, archive)
	if err != nil {
		return nil, err
	}
	result := &ImportResult{
		Summary:       preview.Summary,
		IgnoredTables: preview.IgnoredTables,
		Warnings:      preview.Warnings,
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin import transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, spec := range s.specs {
		rows, ok := archive.Tables[spec.Name]
		if !ok {
			continue
		}
		tableResult := ImportTableResult{Name: spec.Name, Rows: len(rows)}

		exists, err := tableExists(ctx, tx, spec.Name)
		if err != nil {
			return nil, fmt.Errorf("check table %s: %w", spec.Name, err)
		}
		if !exists {
			tableResult.Skipped = true
			tableResult.Reason = "target table does not exist"
			result.Tables = append(result.Tables, tableResult)
			continue
		}

		columns, err := loadColumns(ctx, tx, spec.Name)
		if err != nil {
			return nil, fmt.Errorf("load table %s columns: %w", spec.Name, err)
		}
		importable := importableColumns(columns)
		for _, raw := range rows {
			affected, err := upsertRow(ctx, tx, spec, raw, importable)
			if err != nil {
				return nil, fmt.Errorf("import table %s: %w", spec.Name, err)
			}
			tableResult.Imported += affected
		}
		if tableResult.Imported > 0 && spec.SequenceColumn != "" {
			if err := resetSequence(ctx, tx, spec.Name, spec.SequenceColumn); err != nil {
				return nil, fmt.Errorf("reset table %s sequence: %w", spec.Name, err)
			}
		}
		result.Tables = append(result.Tables, tableResult)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit import transaction: %w", err)
	}
	return result, nil
}

func (s *Service) validateArchive(archive Archive) error {
	if archive.Product != ArchiveProduct && archive.Product != legacyArchiveProduct {
		return &ValidationError{message: fmt.Sprintf("unsupported archive product %q", archive.Product)}
	}
	if archive.Version != ArchiveVersion {
		return &ValidationError{message: fmt.Sprintf("unsupported archive version %d", archive.Version)}
	}
	if archive.Tables == nil {
		return &ValidationError{message: "archive tables cannot be empty"}
	}
	return nil
}

func exportTable(ctx context.Context, db tableQuerier, spec tableSpec) ([]json.RawMessage, error) {
	query := fmt.Sprintf(
		"SELECT to_jsonb(t) FROM (SELECT * FROM %s%s) AS t",
		quoteIdent(spec.Name),
		orderByClause(spec.OrderColumns),
	)
	dbRows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = dbRows.Close() }()

	out := make([]json.RawMessage, 0)
	for dbRows.Next() {
		var raw json.RawMessage
		if err := dbRows.Scan(&raw); err != nil {
			return nil, err
		}
		out = append(out, cloneRawMessage(raw))
	}
	if err := dbRows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func upsertRow(ctx context.Context, tx tableExecutor, spec tableSpec, raw json.RawMessage, importable map[string]columnInfo) (int, error) {
	var row map[string]json.RawMessage
	if err := json.Unmarshal(raw, &row); err != nil {
		return 0, fmt.Errorf("invalid row json: %w", err)
	}
	columns := make([]string, 0, len(row))
	for column := range row {
		if containsString(spec.OmitColumns, column) {
			continue
		}
		if _, ok := importable[column]; ok {
			columns = append(columns, column)
		}
	}
	sort.Strings(columns)
	if len(columns) == 0 {
		return 0, errors.New("row has no importable columns")
	}
	for _, conflictColumn := range spec.ConflictColumns {
		if _, ok := importable[conflictColumn]; !ok {
			return 0, fmt.Errorf("conflict column %s does not exist", conflictColumn)
		}
		if _, ok := row[conflictColumn]; !ok {
			return 0, fmt.Errorf("row misses conflict column %s", conflictColumn)
		}
	}

	query := upsertQuery(spec, columns)
	result, err := tx.ExecContext(ctx, query, []byte(raw))
	if err != nil {
		return 0, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 1, nil
	}
	return int(affected), nil
}

func upsertQuery(spec tableSpec, columns []string) string {
	quotedColumns := quoteIdentList(columns)
	selectColumns := make([]string, 0, len(columns))
	for _, column := range columns {
		selectColumns = append(selectColumns, quoteIdent(column))
	}

	updateColumns := make([]string, 0, len(columns))
	for _, column := range columns {
		if containsString(spec.ConflictColumns, column) {
			continue
		}
		updateColumns = append(updateColumns, fmt.Sprintf("%s = EXCLUDED.%s", quoteIdent(column), quoteIdent(column)))
	}

	conflict := fmt.Sprintf("ON CONFLICT (%s)", strings.Join(quoteIdentList(spec.ConflictColumns), ", "))
	action := "DO NOTHING"
	if len(updateColumns) > 0 {
		action = "DO UPDATE SET " + strings.Join(updateColumns, ", ")
	}

	return fmt.Sprintf(
		"WITH payload AS (SELECT * FROM jsonb_populate_record(NULL::%s, $1::jsonb)) INSERT INTO %s (%s) SELECT %s FROM payload %s %s",
		quoteIdent(spec.Name),
		quoteIdent(spec.Name),
		strings.Join(quotedColumns, ", "),
		strings.Join(selectColumns, ", "),
		conflict,
		action,
	)
}

func tableExists(ctx context.Context, db tableQuerier, tableName string) (bool, error) {
	var exists bool
	err := db.QueryRowContext(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM information_schema.tables
    WHERE table_schema = current_schema()
      AND table_name = $1
)`, tableName).Scan(&exists)
	return exists, err
}

func loadColumns(ctx context.Context, db tableQuerier, tableName string) ([]columnInfo, error) {
	rows, err := db.QueryContext(ctx, `
SELECT column_name, is_generated, COALESCE(identity_generation, '')
FROM information_schema.columns
WHERE table_schema = current_schema()
  AND table_name = $1
ORDER BY ordinal_position`, tableName)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []columnInfo
	for rows.Next() {
		var column columnInfo
		if err := rows.Scan(&column.Name, &column.IsGenerated, &column.IdentityGeneration); err != nil {
			return nil, err
		}
		out = append(out, column)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func importableColumns(columns []columnInfo) map[string]columnInfo {
	out := make(map[string]columnInfo, len(columns))
	for _, column := range columns {
		if column.IsGenerated != "" && column.IsGenerated != "NEVER" {
			continue
		}
		if column.IdentityGeneration == "ALWAYS" {
			continue
		}
		out[column.Name] = column
	}
	return out
}

func resetSequence(ctx context.Context, tx tableQuerier, tableName, columnName string) error {
	var sequence sql.NullString
	if err := tx.QueryRowContext(ctx, "SELECT pg_get_serial_sequence($1, $2)", tableName, columnName).Scan(&sequence); err != nil {
		return err
	}
	if !sequence.Valid || strings.TrimSpace(sequence.String) == "" {
		return nil
	}
	query := fmt.Sprintf(
		"SELECT setval($1::regclass, GREATEST(COALESCE((SELECT MAX(%s) FROM %s), 1), 1), (SELECT COUNT(*) > 0 FROM %s))",
		quoteIdent(columnName),
		quoteIdent(tableName),
		quoteIdent(tableName),
	)
	var ignored int64
	return tx.QueryRowContext(ctx, query, sequence.String).Scan(&ignored)
}

func orderByClause(columns []string) string {
	if len(columns) == 0 {
		return ""
	}
	quoted := make([]string, 0, len(columns))
	for _, column := range columns {
		quoted = append(quoted, quoteIdent(column))
	}
	return " ORDER BY " + strings.Join(quoted, ", ")
}

func quoteIdentList(names []string) []string {
	out := make([]string, 0, len(names))
	for _, name := range names {
		out = append(out, quoteIdent(name))
	}
	return out
}

func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func sortedTableNames(tables map[string][]json.RawMessage) []string {
	names := make([]string, 0, len(tables))
	for name := range tables {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func containsSensitiveTable(items []TableSummary) bool {
	for _, item := range items {
		if item.Sensitive {
			return true
		}
	}
	return false
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func cloneRawMessage(raw json.RawMessage) json.RawMessage {
	if raw == nil {
		return nil
	}
	out := make([]byte, len(raw))
	copy(out, raw)
	return out
}
