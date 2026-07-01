package sitecatalog

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
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

func (r *SQLRepository) ListSites(ctx context.Context, filter SiteFilter) ([]*adminplusdomain.SiteCatalogSite, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	args := make([]any, 0, 5)
	where, args := siteCatalogSiteWhere(filter, args)
	args = append(args, filter.Limit)
	limitRef := fmt.Sprintf("$%d", len(args))
	rows, err := r.db.QueryContext(ctx, siteCatalogSiteSelectClause()+`
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY updated_at DESC, id DESC
		LIMIT `+limitRef, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]*adminplusdomain.SiteCatalogSite, 0)
	for rows.Next() {
		item, err := scanSiteCatalogSite(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return r.hydrateSites(ctx, items)
}

func (r *SQLRepository) GetSite(ctx context.Context, id int64) (*adminplusdomain.SiteCatalogSite, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	row := r.db.QueryRowContext(ctx, siteCatalogSiteSelectClause()+`
		WHERE id = $1
	`, id)
	site, err := scanSiteCatalogSite(row)
	if err == sql.ErrNoRows {
		return nil, infraerrors.New(http.StatusNotFound, "SITE_CATALOG_SITE_NOT_FOUND", "site catalog site not found")
	}
	if err != nil {
		return nil, err
	}
	items, err := r.hydrateSites(ctx, []*adminplusdomain.SiteCatalogSite{site})
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, infraerrors.New(http.StatusNotFound, "SITE_CATALOG_SITE_NOT_FOUND", "site catalog site not found")
	}
	return items[0], nil
}

func (r *SQLRepository) CreateSite(ctx context.Context, site *adminplusdomain.SiteCatalogSite) (*adminplusdomain.SiteCatalogSite, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer rollbackUnlessCommitted(tx, &err)
	created, err := insertSiteTx(ctx, tx, site)
	if err != nil {
		return nil, err
	}
	if err = upsertSiteLinksTx(ctx, tx, created.ID, site.Links); err != nil {
		return nil, err
	}
	if err = replaceSiteCategoriesTx(ctx, tx, created.ID, categoryIDs(site.Categories)); err != nil {
		return nil, err
	}
	if err = replaceSiteTagsTx(ctx, tx, created.ID, tagIDs(site.Tags)); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	err = nil
	return r.GetSite(ctx, created.ID)
}

func (r *SQLRepository) DeleteSite(ctx context.Context, id int64) error {
	if r == nil || r.db == nil {
		return dbNotConfigured()
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollbackUnlessCommitted(tx, &err)

	var lockedID int64
	if err = tx.QueryRowContext(ctx, `
		SELECT id
		FROM admin_plus_site_catalog_sites
		WHERE id = $1
		FOR UPDATE
	`, id).Scan(&lockedID); err == sql.ErrNoRows {
		return infraerrors.New(http.StatusNotFound, "SITE_CATALOG_SITE_NOT_FOUND", "site catalog site not found")
	} else if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `
		UPDATE admin_plus_site_discoveries
		SET catalog_site_id = NULL,
			process_status = CASE
				WHEN process_status = 'added_to_catalog' THEN 'unprocessed'
				ELSE process_status
			END,
			updated_at = NOW()
		WHERE catalog_site_id = $1
	`, id); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `
		DELETE FROM admin_plus_site_catalog_sites
		WHERE id = $1
	`, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return infraerrors.New(http.StatusNotFound, "SITE_CATALOG_SITE_NOT_FOUND", "site catalog site not found")
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	err = nil
	return nil
}

func (r *SQLRepository) BulkPublishSites(ctx context.Context, input BulkPublishSitesInput, publishedAt time.Time) (int64, error) {
	if r == nil || r.db == nil {
		return 0, dbNotConfigured()
	}
	args := make([]any, 0, 7)
	where, args := siteCatalogSiteWhere(SiteFilter{
		Query:    input.Query,
		Status:   input.Status,
		SiteKind: input.SiteKind,
		Provider: input.Provider,
	}, args)
	if len(input.IDs) > 0 {
		args = append(args, pq.Array(input.IDs))
		where = append(where, fmt.Sprintf("id = ANY($%d)", len(args)))
	}
	where = append(where, "(status <> 'published' OR visibility <> 'public')")
	args = append(args, publishedAt)
	publishedAtRef := fmt.Sprintf("$%d", len(args))
	result, err := r.db.ExecContext(ctx, `
		UPDATE admin_plus_site_catalog_sites
		SET status = 'published',
			visibility = 'public',
			published_at = COALESCE(published_at, `+publishedAtRef+`),
			updated_at = `+publishedAtRef+`
		WHERE `+strings.Join(where, " AND "), args...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func siteCatalogSiteWhere(filter SiteFilter, args []any) ([]string, []any) {
	where := []string{"1=1"}
	addArg := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}
	if filter.Query != "" {
		query := "%" + strings.ToLower(filter.Query) + "%"
		ref := addArg(query)
		where = append(where, "(LOWER(name) LIKE "+ref+" OR LOWER(canonical_host) LIKE "+ref+" OR LOWER(summary) LIKE "+ref+")")
	}
	if filter.Status != "" {
		where = append(where, "status = "+addArg(string(filter.Status)))
	}
	if filter.SiteKind != "" {
		where = append(where, "site_kind = "+addArg(string(filter.SiteKind)))
	}
	if filter.Provider != "" {
		where = append(where, "provider_type = "+addArg(string(filter.Provider)))
	}
	return where, args
}

func (r *SQLRepository) AddDiscoveryCandidate(ctx context.Context, candidate *adminplusdomain.SiteDiscoveryItem, input AddDiscoveryCandidateInput) (*adminplusdomain.SiteCatalogSite, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer rollbackUnlessCommitted(tx, &err)

	siteID := input.SiteID
	if siteID <= 0 {
		now := time.Now().UTC()
		site := &adminplusdomain.SiteCatalogSite{
			Slug:                 slugify(input.Slug),
			CanonicalHost:        firstNonEmpty(canonicalHostFromLinks(input.Links), candidate.Host),
			Name:                 strings.TrimSpace(input.Name),
			Summary:              strings.TrimSpace(input.Summary),
			Description:          strings.TrimSpace(input.Description),
			ProviderType:         candidate.ProviderType,
			SiteKind:             input.SiteKind,
			Status:               input.Status,
			Visibility:           input.Visibility,
			QualityStatus:        adminplusdomain.SiteCatalogQualityNeedsReview,
			RecommendationLevel:  input.RecommendationLevel,
			RecommendationReason: strings.TrimSpace(input.RecommendationReason),
			RiskLevel:            input.RiskLevel,
			SupplierID:           candidate.SupplierID,
			Metadata: map[string]any{
				"source_section":              candidate.SourceSection,
				"source_category":             candidate.SourceCategory,
				"classification_confidence":   candidate.ClassificationConfidence,
				"classification_evidence":     candidate.ClassificationEvidence,
				"monitor_available":           candidate.MonitorAvailable,
				"monitor_uptime_percent":      candidate.MonitorUptimePercent,
				"monitor_latest_response_ms":  candidate.MonitorLatestResponseMS,
				"monitor_average_response_ms": candidate.MonitorAvgResponseMS,
			},
			CreatedAt: now,
			UpdatedAt: now,
		}
		if site.Slug == "" {
			site.Slug = slugify(firstNonEmpty(candidate.Host, candidate.Name))
		}
		created, insertErr := insertSiteTx(ctx, tx, site)
		if insertErr != nil {
			return nil, insertErr
		}
		siteID = created.ID
	}
	if err = upsertSiteLinksTx(ctx, tx, siteID, linksFromInput(input.Links)); err != nil {
		return nil, err
	}
	if err = replaceSiteCategoriesTx(ctx, tx, siteID, input.CategoryIDs); err != nil {
		return nil, err
	}
	if err = replaceSiteTagsTx(ctx, tx, siteID, input.TagIDs); err != nil {
		return nil, err
	}
	if err = upsertCandidateSourceTx(ctx, tx, siteID, candidate); err != nil {
		return nil, err
	}
	if err = linkDiscoveryCandidateTx(ctx, tx, candidate.ID, siteID); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	err = nil
	return r.GetSite(ctx, siteID)
}

func (r *SQLRepository) ListCategories(ctx context.Context) ([]*adminplusdomain.SiteCatalogCategory, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, COALESCE(parent_id, 0), slug, name, description, display_order, enabled, created_at, updated_at
		FROM admin_plus_site_catalog_categories
		ORDER BY display_order ASC, id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]*adminplusdomain.SiteCatalogCategory, 0)
	for rows.Next() {
		item, err := scanSiteCatalogCategory(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *SQLRepository) ListTags(ctx context.Context) ([]*adminplusdomain.SiteCatalogTag, error) {
	if r == nil || r.db == nil {
		return nil, dbNotConfigured()
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, slug, name, tag_type, color, enabled, created_at, updated_at
		FROM admin_plus_site_catalog_tags
		ORDER BY tag_type ASC, name ASC, id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]*adminplusdomain.SiteCatalogTag, 0)
	for rows.Next() {
		item, err := scanSiteCatalogTag(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *SQLRepository) SlugExists(ctx context.Context, slug string) (bool, error) {
	if r == nil || r.db == nil {
		return false, dbNotConfigured()
	}
	var exists bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM admin_plus_site_catalog_sites WHERE slug = $1)
	`, strings.TrimSpace(slug)).Scan(&exists)
	return exists, err
}

func insertSiteTx(ctx context.Context, tx *sql.Tx, site *adminplusdomain.SiteCatalogSite) (*adminplusdomain.SiteCatalogSite, error) {
	metadata, err := marshalMap(site.Metadata)
	if err != nil {
		return nil, err
	}
	row := tx.QueryRowContext(ctx, `
		INSERT INTO admin_plus_site_catalog_sites (
			slug, canonical_host, name, short_name, summary, description,
			provider_type, site_kind, status, visibility, quality_status,
			recommendation_level, recommendation_reason, risk_level,
			logo_url, screenshot_url, primary_language, country_or_region,
			supplier_id, metadata, published_at, created_at, updated_at
		)
		VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11,
			$12, $13, $14,
			$15, $16, $17, $18,
			NULLIF($19, 0), $20, $21, $22, $23
		)
		`+siteCatalogSiteReturningClause(),
		site.Slug,
		site.CanonicalHost,
		site.Name,
		site.ShortName,
		site.Summary,
		site.Description,
		string(site.ProviderType),
		string(site.SiteKind),
		string(site.Status),
		string(site.Visibility),
		string(site.QualityStatus),
		string(site.RecommendationLevel),
		site.RecommendationReason,
		string(site.RiskLevel),
		site.LogoURL,
		site.ScreenshotURL,
		site.PrimaryLanguage,
		site.CountryOrRegion,
		site.SupplierID,
		metadata,
		nullableTimePtr(site.PublishedAt),
		site.CreatedAt,
		site.UpdatedAt,
	)
	return scanSiteCatalogSite(row)
}

func upsertSiteLinksTx(ctx context.Context, tx *sql.Tx, siteID int64, links []*adminplusdomain.SiteCatalogLink) error {
	for _, link := range links {
		if link == nil || strings.TrimSpace(link.URL) == "" {
			continue
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO admin_plus_site_catalog_links (
				site_id, link_type, url, label, is_primary, status, last_checked_at, created_at, updated_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
			ON CONFLICT (site_id, link_type, url) DO UPDATE
			SET label = EXCLUDED.label,
				is_primary = EXCLUDED.is_primary,
				status = EXCLUDED.status,
				updated_at = NOW()
		`, siteID, string(link.LinkType), link.URL, link.Label, link.IsPrimary, string(link.Status), nullableTimePtr(link.LastCheckedAt))
		if err != nil {
			return err
		}
	}
	return nil
}

func replaceSiteCategoriesTx(ctx context.Context, tx *sql.Tx, siteID int64, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := tx.ExecContext(ctx, `DELETE FROM admin_plus_site_catalog_site_categories WHERE site_id = $1`, siteID)
	if err != nil {
		return err
	}
	for i, id := range uniqueInt64s(ids) {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO admin_plus_site_catalog_site_categories (site_id, category_id, is_primary, display_order)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (site_id, category_id) DO UPDATE
			SET is_primary = EXCLUDED.is_primary,
				display_order = EXCLUDED.display_order
		`, siteID, id, i == 0, i)
		if err != nil {
			return err
		}
	}
	return nil
}

func replaceSiteTagsTx(ctx context.Context, tx *sql.Tx, siteID int64, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := tx.ExecContext(ctx, `DELETE FROM admin_plus_site_catalog_site_tags WHERE site_id = $1`, siteID)
	if err != nil {
		return err
	}
	for _, id := range uniqueInt64s(ids) {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO admin_plus_site_catalog_site_tags (site_id, tag_id)
			VALUES ($1, $2)
			ON CONFLICT (site_id, tag_id) DO NOTHING
		`, siteID, id)
		if err != nil {
			return err
		}
	}
	return nil
}

func upsertCandidateSourceTx(ctx context.Context, tx *sql.Tx, siteID int64, candidate *adminplusdomain.SiteDiscoveryItem) error {
	payload, err := json.Marshal(candidate.RawPayload)
	if err != nil {
		return err
	}
	if len(payload) == 0 || string(payload) == "null" {
		payload = []byte("{}")
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO admin_plus_site_catalog_sources (
			site_id, source_type, source_name, source_url, source_external_id,
			discovery_candidate_id, observed_payload, first_seen_at, last_seen_at, created_at
		)
		VALUES ($1, 'discovery_candidate', 'site_discovery', $2, $3, $4, $5, NOW(), NOW(), NOW())
		ON CONFLICT (site_id, discovery_candidate_id) WHERE discovery_candidate_id IS NOT NULL DO UPDATE
		SET source_url = EXCLUDED.source_url,
			source_external_id = EXCLUDED.source_external_id,
			observed_payload = EXCLUDED.observed_payload,
			last_seen_at = NOW()
	`, siteID, candidate.SourceURL, candidate.SourceSiteID, candidate.ID, payload)
	return err
}

func linkDiscoveryCandidateTx(ctx context.Context, tx *sql.Tx, candidateID int64, siteID int64) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE admin_plus_site_discoveries
		SET catalog_site_id = $2,
			process_status = 'added_to_catalog',
			updated_at = NOW()
		WHERE id = $1
	`, candidateID, siteID)
	return err
}

func (r *SQLRepository) hydrateSites(ctx context.Context, sites []*adminplusdomain.SiteCatalogSite) ([]*adminplusdomain.SiteCatalogSite, error) {
	if len(sites) == 0 {
		return sites, nil
	}
	byID := make(map[int64]*adminplusdomain.SiteCatalogSite, len(sites))
	ids := make([]int64, 0, len(sites))
	for _, site := range sites {
		if site == nil {
			continue
		}
		byID[site.ID] = site
		ids = append(ids, site.ID)
	}
	if err := r.hydrateLinks(ctx, byID, ids); err != nil {
		return nil, err
	}
	if err := r.hydrateSources(ctx, byID, ids); err != nil {
		return nil, err
	}
	if err := r.hydrateCategories(ctx, byID, ids); err != nil {
		return nil, err
	}
	if err := r.hydrateTags(ctx, byID, ids); err != nil {
		return nil, err
	}
	return sites, nil
}

func (r *SQLRepository) hydrateLinks(ctx context.Context, byID map[int64]*adminplusdomain.SiteCatalogSite, ids []int64) error {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, site_id, link_type, url, label, is_primary, status, last_checked_at, created_at, updated_at
		FROM admin_plus_site_catalog_links
		WHERE site_id = ANY($1)
		ORDER BY is_primary DESC, id ASC
	`, pqInt64Array(ids))
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		link, err := scanSiteCatalogLink(rows)
		if err != nil {
			return err
		}
		if site := byID[link.SiteID]; site != nil {
			site.Links = append(site.Links, link)
		}
	}
	return rows.Err()
}

func (r *SQLRepository) hydrateSources(ctx context.Context, byID map[int64]*adminplusdomain.SiteCatalogSite, ids []int64) error {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, site_id, source_type, source_name, source_url, source_external_id,
			COALESCE(discovery_candidate_id, 0), observed_payload,
			first_seen_at, last_seen_at, created_at
		FROM admin_plus_site_catalog_sources
		WHERE site_id = ANY($1)
		ORDER BY last_seen_at DESC, id DESC
	`, pqInt64Array(ids))
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		source, err := scanSiteCatalogSource(rows)
		if err != nil {
			return err
		}
		if site := byID[source.SiteID]; site != nil {
			site.Sources = append(site.Sources, source)
		}
	}
	return rows.Err()
}

func (r *SQLRepository) hydrateCategories(ctx context.Context, byID map[int64]*adminplusdomain.SiteCatalogSite, ids []int64) error {
	rows, err := r.db.QueryContext(ctx, `
		SELECT sc.site_id, c.id, COALESCE(c.parent_id, 0), c.slug, c.name, c.description, c.display_order, c.enabled, c.created_at, c.updated_at
		FROM admin_plus_site_catalog_site_categories sc
		INNER JOIN admin_plus_site_catalog_categories c ON c.id = sc.category_id
		WHERE sc.site_id = ANY($1)
		ORDER BY sc.is_primary DESC, sc.display_order ASC, c.display_order ASC
	`, pqInt64Array(ids))
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var siteID int64
		category, err := scanSiteCatalogCategory(siteCategoryScanner{scanner: rows, siteID: &siteID})
		if err != nil {
			return err
		}
		if site := byID[siteID]; site != nil {
			site.Categories = append(site.Categories, category)
		}
	}
	return rows.Err()
}

func (r *SQLRepository) hydrateTags(ctx context.Context, byID map[int64]*adminplusdomain.SiteCatalogSite, ids []int64) error {
	rows, err := r.db.QueryContext(ctx, `
		SELECT st.site_id, t.id, t.slug, t.name, t.tag_type, t.color, t.enabled, t.created_at, t.updated_at
		FROM admin_plus_site_catalog_site_tags st
		INNER JOIN admin_plus_site_catalog_tags t ON t.id = st.tag_id
		WHERE st.site_id = ANY($1)
		ORDER BY t.tag_type ASC, t.name ASC
	`, pqInt64Array(ids))
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var siteID int64
		tag, err := scanSiteCatalogTag(siteTagScanner{scanner: rows, siteID: &siteID})
		if err != nil {
			return err
		}
		if site := byID[siteID]; site != nil {
			site.Tags = append(site.Tags, tag)
		}
	}
	return rows.Err()
}

func siteCatalogSiteSelectClause() string {
	return `
		SELECT id, slug, canonical_host, name, short_name, summary, description,
			provider_type, site_kind, status, visibility, quality_status,
			recommendation_level, recommendation_reason, risk_level,
			logo_url, screenshot_url, primary_language, country_or_region,
			COALESCE(supplier_id, 0), metadata, published_at, created_at, updated_at
		FROM admin_plus_site_catalog_sites
	`
}

func siteCatalogSiteReturningClause() string {
	return `
		RETURNING id, slug, canonical_host, name, short_name, summary, description,
			provider_type, site_kind, status, visibility, quality_status,
			recommendation_level, recommendation_reason, risk_level,
			logo_url, screenshot_url, primary_language, country_or_region,
			COALESCE(supplier_id, 0), metadata, published_at, created_at, updated_at
	`
}

type scanner interface {
	Scan(dest ...any) error
}

func scanSiteCatalogSite(row scanner) (*adminplusdomain.SiteCatalogSite, error) {
	var site adminplusdomain.SiteCatalogSite
	var providerType, siteKind, status, visibility, qualityStatus, recommendationLevel, riskLevel string
	var metadata []byte
	var published sql.NullTime
	err := row.Scan(
		&site.ID,
		&site.Slug,
		&site.CanonicalHost,
		&site.Name,
		&site.ShortName,
		&site.Summary,
		&site.Description,
		&providerType,
		&siteKind,
		&status,
		&visibility,
		&qualityStatus,
		&recommendationLevel,
		&site.RecommendationReason,
		&riskLevel,
		&site.LogoURL,
		&site.ScreenshotURL,
		&site.PrimaryLanguage,
		&site.CountryOrRegion,
		&site.SupplierID,
		&metadata,
		&published,
		&site.CreatedAt,
		&site.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	site.ProviderType = adminplusdomain.SupplierType(providerType)
	site.SiteKind = adminplusdomain.SiteCatalogKind(siteKind)
	site.Status = adminplusdomain.SiteCatalogStatus(status)
	site.Visibility = adminplusdomain.SiteCatalogVisibility(visibility)
	site.QualityStatus = adminplusdomain.SiteCatalogQualityStatus(qualityStatus)
	site.RecommendationLevel = adminplusdomain.SiteCatalogRecommendationLevel(recommendationLevel)
	site.RiskLevel = adminplusdomain.SiteCatalogRiskLevel(riskLevel)
	if len(metadata) > 0 {
		_ = json.Unmarshal(metadata, &site.Metadata)
	}
	if published.Valid {
		t := published.Time
		site.PublishedAt = &t
	}
	return &site, nil
}

func scanSiteCatalogLink(row scanner) (*adminplusdomain.SiteCatalogLink, error) {
	var link adminplusdomain.SiteCatalogLink
	var linkType, status string
	var lastChecked sql.NullTime
	err := row.Scan(&link.ID, &link.SiteID, &linkType, &link.URL, &link.Label, &link.IsPrimary, &status, &lastChecked, &link.CreatedAt, &link.UpdatedAt)
	if err != nil {
		return nil, err
	}
	link.LinkType = adminplusdomain.SiteCatalogLinkType(linkType)
	link.Status = adminplusdomain.SiteCatalogLinkStatus(status)
	if lastChecked.Valid {
		t := lastChecked.Time
		link.LastCheckedAt = &t
	}
	return &link, nil
}

func scanSiteCatalogSource(row scanner) (*adminplusdomain.SiteCatalogSource, error) {
	var source adminplusdomain.SiteCatalogSource
	var payload []byte
	err := row.Scan(
		&source.ID,
		&source.SiteID,
		&source.SourceType,
		&source.SourceName,
		&source.SourceURL,
		&source.SourceExternalID,
		&source.DiscoveryCandidateID,
		&payload,
		&source.FirstSeenAt,
		&source.LastSeenAt,
		&source.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if len(payload) > 0 {
		_ = json.Unmarshal(payload, &source.ObservedPayload)
	}
	return &source, nil
}

func scanSiteCatalogCategory(row scanner) (*adminplusdomain.SiteCatalogCategory, error) {
	var category adminplusdomain.SiteCatalogCategory
	err := row.Scan(&category.ID, &category.ParentID, &category.Slug, &category.Name, &category.Description, &category.DisplayOrder, &category.Enabled, &category.CreatedAt, &category.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &category, nil
}

func scanSiteCatalogTag(row scanner) (*adminplusdomain.SiteCatalogTag, error) {
	var tag adminplusdomain.SiteCatalogTag
	err := row.Scan(&tag.ID, &tag.Slug, &tag.Name, &tag.TagType, &tag.Color, &tag.Enabled, &tag.CreatedAt, &tag.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &tag, nil
}

type siteCategoryScanner struct {
	scanner scanner
	siteID  *int64
}

func (s siteCategoryScanner) Scan(dest ...any) error {
	return s.scanner.Scan(append([]any{s.siteID}, dest...)...)
}

type siteTagScanner struct {
	scanner scanner
	siteID  *int64
}

func (s siteTagScanner) Scan(dest ...any) error {
	return s.scanner.Scan(append([]any{s.siteID}, dest...)...)
}

func linksFromInput(inputs []SiteLinkInput) []*adminplusdomain.SiteCatalogLink {
	now := time.Now().UTC()
	links := make([]*adminplusdomain.SiteCatalogLink, 0, len(inputs))
	for _, input := range inputs {
		if link := siteCatalogLinkFromInput(input, now); link != nil {
			links = append(links, link)
		}
	}
	return links
}

func categoryIDs(categories []*adminplusdomain.SiteCatalogCategory) []int64 {
	ids := make([]int64, 0, len(categories))
	for _, category := range categories {
		if category != nil && category.ID > 0 {
			ids = append(ids, category.ID)
		}
	}
	return ids
}

func tagIDs(tags []*adminplusdomain.SiteCatalogTag) []int64 {
	ids := make([]int64, 0, len(tags))
	for _, tag := range tags {
		if tag != nil && tag.ID > 0 {
			ids = append(ids, tag.ID)
		}
	}
	return ids
}

func uniqueInt64s(values []int64) []int64 {
	seen := make(map[int64]struct{}, len(values))
	out := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func marshalMap(value map[string]any) ([]byte, error) {
	if len(value) == 0 {
		return []byte("{}"), nil
	}
	return json.Marshal(value)
}

func nullableTimePtr(value *time.Time) any {
	if value == nil || value.IsZero() {
		return nil
	}
	return *value
}

func rollbackUnlessCommitted(tx *sql.Tx, err *error) {
	if err != nil && *err != nil {
		_ = tx.Rollback()
	}
}

type pqInt64Array []int64

func (a pqInt64Array) Value() (driver.Value, error) {
	if len(a) == 0 {
		return "{}", nil
	}
	parts := make([]string, 0, len(a))
	for _, value := range a {
		parts = append(parts, fmt.Sprintf("%d", value))
	}
	return "{" + strings.Join(parts, ",") + "}", nil
}
