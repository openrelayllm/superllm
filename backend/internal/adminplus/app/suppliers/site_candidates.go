package suppliers

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type CreateFromSiteCandidateInput struct {
	Name                  string
	Type                  adminplusdomain.SupplierType
	DashboardURL          string
	APIBaseURL            string
	ThirdPartyRechargeURL string
	LocalRechargeURL      string
	SourceHost            string
	SourceURL             string
	Title                 string
}

type SiteMatchInput struct {
	URL    string
	Origin string
	Host   string
}

type SiteMatchResult struct {
	Status    string                      `json:"status"`
	Suppliers []*adminplusdomain.Supplier `json:"suppliers"`
	Host      string                      `json:"host,omitempty"`
	Origin    string                      `json:"origin,omitempty"`
	Reason    string                      `json:"reason,omitempty"`
}

type EnsureFromSiteCandidateOptions struct {
	AllowCreate bool
}

type EnsureFromSiteCandidateResult struct {
	Supplier    *adminplusdomain.Supplier `json:"supplier"`
	Created     bool                      `json:"created"`
	Matched     bool                      `json:"matched"`
	MatchStatus string                    `json:"match_status,omitempty"`
}

func (s *Service) CreateFromSiteCandidate(ctx context.Context, in CreateFromSiteCandidateInput) (*adminplusdomain.Supplier, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier service is not configured")
	}
	dashboardURL, err := normalizeOptionalURL(firstNonEmpty(in.DashboardURL, in.SourceURL), "SUPPLIER_DASHBOARD_URL_INVALID")
	if err != nil {
		return nil, err
	}
	if dashboardURL == "" {
		return nil, badRequest("SUPPLIER_DASHBOARD_URL_REQUIRED", "supplier dashboard url is required")
	}
	parsed, err := url.Parse(dashboardURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, badRequest("SUPPLIER_DASHBOARD_URL_INVALID", "invalid supplier url")
	}
	if in.SourceHost != "" && !sameHost(parsed.Host, in.SourceHost) {
		return nil, badRequest("SUPPLIER_SITE_HOST_MISMATCH", "supplier candidate host does not match current site")
	}
	siteOrigin := parsed.Scheme + "://" + parsed.Host
	existing, err := s.List(ctx, SupplierFilter{})
	if err != nil {
		return nil, err
	}
	for _, supplier := range existing {
		if supplier == nil {
			continue
		}
		if supplierURLHostMatches(supplier.DashboardURL, parsed.Host) || supplierURLHostMatches(supplier.APIBaseURL, parsed.Host) {
			return nil, infraerrors.New(http.StatusConflict, "SUPPLIER_SITE_ALREADY_EXISTS", "supplier for this site already exists")
		}
	}
	apiBaseURL, err := normalizeCandidateAPIBaseURL(in.APIBaseURL, parsed)
	if err != nil {
		return nil, err
	}
	supplierType := inferCandidateSupplierType(in.Type, apiBaseURL, dashboardURL, in.SourceURL)
	supplierKind := candidateSupplierKind(supplierType)
	name := strings.TrimSpace(firstNonEmpty(in.Name, in.Title, parsed.Host))
	if len(name) > 80 {
		name = name[:80]
	}
	return s.Create(ctx, CreateSupplierInput{
		Name:                  name,
		Kind:                  supplierKind,
		Type:                  supplierType,
		RuntimeStatus:         adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:          adminplusdomain.SupplierHealthStatusNormal,
		DashboardURL:          siteOrigin,
		APIBaseURL:            apiBaseURL,
		ThirdPartyRechargeURL: firstNonEmpty(in.ThirdPartyRechargeURL, inferThirdPartyRechargeURL(in.SourceURL), inferThirdPartyRechargeURL(in.DashboardURL)),
		LocalRechargeURL:      in.LocalRechargeURL,
		Notes:                 "created from Chrome extension site candidate",
		BrowserLoginEnabled:   true,
		BalanceCurrency:       defaultCurrencyForCandidateSupplier(supplierType),
	})
}

func (s *Service) EnsureFromSiteCandidate(ctx context.Context, in CreateFromSiteCandidateInput) (*EnsureFromSiteCandidateResult, error) {
	return s.EnsureFromSiteCandidateWithOptions(ctx, in, EnsureFromSiteCandidateOptions{})
}

func (s *Service) EnsureFromSiteCandidateWithOptions(ctx context.Context, in CreateFromSiteCandidateInput, options EnsureFromSiteCandidateOptions) (*EnsureFromSiteCandidateResult, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier service is not configured")
	}
	match, err := s.MatchSite(ctx, SiteMatchInput{
		URL:    firstNonEmpty(in.SourceURL, in.DashboardURL),
		Origin: in.DashboardURL,
		Host:   in.SourceHost,
	})
	if err == nil && len(match.Suppliers) == 1 {
		supplier, enrichErr := s.enrichSupplierRechargeURLs(ctx, match.Suppliers[0], in)
		if enrichErr != nil {
			return nil, enrichErr
		}
		return &EnsureFromSiteCandidateResult{
			Supplier:    supplier,
			Matched:     true,
			MatchStatus: match.Status,
		}, nil
	}
	if err == nil && len(match.Suppliers) > 1 {
		return nil, infraerrors.New(http.StatusConflict, "SUPPLIER_SITE_AMBIGUOUS", "multiple suppliers match current site")
	}
	if err != nil && infraerrors.Code(err) != http.StatusBadRequest {
		return nil, err
	}
	if !options.AllowCreate {
		return nil, infraerrors.New(http.StatusConflict, "SUPPLIER_SITE_REGISTRATION_REQUIRED", "site candidate must be registered before importing as supplier")
	}
	created, err := s.CreateFromSiteCandidate(ctx, in)
	if err != nil {
		if infraerrors.Reason(err) != "SUPPLIER_SITE_ALREADY_EXISTS" {
			return nil, err
		}
		match, matchErr := s.MatchSite(ctx, SiteMatchInput{
			URL:    firstNonEmpty(in.SourceURL, in.DashboardURL),
			Origin: in.DashboardURL,
			Host:   in.SourceHost,
		})
		if matchErr != nil {
			return nil, matchErr
		}
		if len(match.Suppliers) == 1 {
			supplier, enrichErr := s.enrichSupplierRechargeURLs(ctx, match.Suppliers[0], in)
			if enrichErr != nil {
				return nil, enrichErr
			}
			return &EnsureFromSiteCandidateResult{
				Supplier:    supplier,
				Matched:     true,
				MatchStatus: match.Status,
			}, nil
		}
		return nil, err
	}
	return &EnsureFromSiteCandidateResult{
		Supplier: created,
		Created:  true,
	}, nil
}

func (s *Service) EnrichRechargeURLs(ctx context.Context, id int64, in CreateFromSiteCandidateInput) (*adminplusdomain.Supplier, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier service is not configured")
	}
	if id <= 0 {
		return nil, badRequest("SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	supplier, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.enrichSupplierRechargeURLs(ctx, supplier, in)
}

func (s *Service) MatchSite(ctx context.Context, in SiteMatchInput) (*SiteMatchResult, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier service is not configured")
	}
	host, origin, err := normalizeSiteMatchInput(in)
	if err != nil {
		return nil, err
	}
	suppliers, err := s.List(ctx, SupplierFilter{})
	if err != nil {
		return nil, err
	}
	matches := make([]*adminplusdomain.Supplier, 0, 1)
	for _, supplier := range suppliers {
		if supplier == nil {
			continue
		}
		if supplierURLExactHostMatches(supplier.DashboardURL, host) || supplierURLExactHostMatches(supplier.APIBaseURL, host) {
			matches = append(matches, supplier)
		}
	}
	if len(matches) == 0 {
		for _, supplier := range suppliers {
			if supplier == nil {
				continue
			}
			if supplierURLHostMatches(supplier.DashboardURL, host) || supplierURLHostMatches(supplier.APIBaseURL, host) {
				matches = append(matches, supplier)
			}
		}
	}
	status := "unknown"
	if len(matches) == 1 {
		status = "matched"
	}
	if len(matches) > 1 {
		status = "ambiguous"
	}
	return &SiteMatchResult{
		Status:    status,
		Suppliers: matches,
		Host:      host,
		Origin:    origin,
	}, nil
}

func (s *Service) enrichSupplierRechargeURLs(ctx context.Context, supplier *adminplusdomain.Supplier, in CreateFromSiteCandidateInput) (*adminplusdomain.Supplier, error) {
	if supplier == nil {
		return supplier, nil
	}
	thirdPartyRechargeURL, err := normalizeOptionalURL(firstNonEmpty(in.ThirdPartyRechargeURL, inferThirdPartyRechargeURL(in.SourceURL), inferThirdPartyRechargeURL(in.DashboardURL)), "SUPPLIER_THIRD_PARTY_RECHARGE_URL_INVALID")
	if err != nil {
		return nil, err
	}
	localRechargeURL, err := normalizeOptionalURL(in.LocalRechargeURL, "SUPPLIER_LOCAL_RECHARGE_URL_INVALID")
	if err != nil {
		return nil, err
	}
	if (thirdPartyRechargeURL == "" || supplier.ThirdPartyRechargeURL != "") && (localRechargeURL == "" || supplier.LocalRechargeURL != "") {
		return supplier, nil
	}
	updated := *supplier
	if updated.ThirdPartyRechargeURL == "" {
		updated.ThirdPartyRechargeURL = thirdPartyRechargeURL
	}
	if updated.LocalRechargeURL == "" {
		updated.LocalRechargeURL = localRechargeURL
	}
	updated.UpdatedAt = s.now().UTC()
	updatedSupplier, err := s.repo.Update(ctx, &updated)
	if err != nil {
		return nil, err
	}
	return normalizeSupplierForRead(updatedSupplier), nil
}

func inferThirdPartyRechargeURL(raw string) string {
	v := strings.TrimSpace(raw)
	if v == "" {
		return ""
	}
	u, err := url.Parse(v)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	path := strings.ToLower(u.EscapedPath())
	for _, marker := range []string{"/custom/", "/recharge", "/payment", "/topup", "/redeem", "/card", "/pay"} {
		if strings.Contains(path, marker) {
			return v
		}
	}
	return ""
}

func normalizeSiteMatchInput(in SiteMatchInput) (string, string, error) {
	raw := firstNonEmpty(in.URL, in.Origin)
	if raw != "" {
		parsed, err := url.Parse(strings.TrimSpace(raw))
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return "", "", badRequest("SUPPLIER_SITE_URL_INVALID", "invalid site url")
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return "", "", badRequest("SUPPLIER_SITE_URL_INVALID", "site url must use http or https")
		}
		origin := parsed.Scheme + "://" + parsed.Host
		return parsed.Host, origin, nil
	}
	host := strings.TrimSpace(in.Host)
	if host == "" {
		return "", "", badRequest("SUPPLIER_SITE_REQUIRED", "site url or host is required")
	}
	return host, "", nil
}

func supplierURLHostMatches(raw string, host string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return false
	}
	return sameHost(u.Host, host)
}

func supplierURLExactHostMatches(raw string, host string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(u.Host), strings.TrimSpace(host))
}

func sameHost(left string, right string) bool {
	return strings.EqualFold(hostWithoutPort(left), hostWithoutPort(right))
}

func hostWithoutPort(value string) string {
	host := strings.TrimSpace(value)
	if parsed, err := url.Parse("https://" + host); err == nil && parsed.Hostname() != "" {
		return parsed.Hostname()
	}
	return host
}
