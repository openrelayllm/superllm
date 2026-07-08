package suppliers

import (
	"math"
	"net/url"
	"strings"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
)

const legacyNewAPIQuotaUnitsPerUSD = 500000.0

type supplierInput struct {
	Name                  string
	Kind                  adminplusdomain.SupplierKind
	Type                  adminplusdomain.SupplierType
	RuntimeStatus         adminplusdomain.SupplierRuntimeStatus
	HealthStatus          adminplusdomain.SupplierHealthStatus
	DashboardURL          string
	APIBaseURL            string
	ThirdPartyRechargeURL string
	LocalRechargeURL      string
	BalanceCents          int64
	BalanceCurrency       string
	RechargeMultiplier    float64
	KeyLimitPolicy        string
	KeyLimitValue         int
}

func normalizeSupplierInput(in supplierInput) (supplierInput, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return supplierInput{}, badRequest("SUPPLIER_NAME_REQUIRED", "supplier name is required")
	}
	if len(name) > 80 {
		return supplierInput{}, badRequest("SUPPLIER_NAME_TOO_LONG", "supplier name must be 80 characters or less")
	}
	if !in.Kind.Valid() {
		return supplierInput{}, badRequest("SUPPLIER_KIND_INVALID", "invalid supplier kind")
	}
	if !in.Type.Valid() {
		return supplierInput{}, badRequest("SUPPLIER_TYPE_INVALID", "invalid supplier type")
	}
	runtimeStatus := in.RuntimeStatus
	if runtimeStatus == "" {
		runtimeStatus = adminplusdomain.SupplierRuntimeStatusMonitorOnly
	}
	if !runtimeStatus.Valid() {
		return supplierInput{}, badRequest("SUPPLIER_RUNTIME_STATUS_INVALID", "invalid supplier runtime status")
	}
	healthStatus := in.HealthStatus
	if healthStatus == "" {
		healthStatus = adminplusdomain.SupplierHealthStatusNormal
	}
	if !healthStatus.Valid() {
		return supplierInput{}, badRequest("SUPPLIER_HEALTH_STATUS_INVALID", "invalid supplier health status")
	}
	if in.BalanceCents < 0 {
		return supplierInput{}, badRequest("SUPPLIER_BALANCE_INVALID", "balance cannot be negative")
	}
	balanceCents, balanceCurrency := normalizeBalanceAmountAndCurrency(in.BalanceCents, in.BalanceCurrency)
	rechargeMultiplier, err := normalizeRechargeMultiplier(in.RechargeMultiplier)
	if err != nil {
		return supplierInput{}, err
	}
	keyLimitPolicy, keyLimitValue, err := normalizeKeyLimit(in.KeyLimitPolicy, in.KeyLimitValue)
	if err != nil {
		return supplierInput{}, err
	}
	if runtimeStatus == adminplusdomain.SupplierRuntimeStatusCandidate && balanceCents <= 0 {
		return supplierInput{}, badRequest("SUPPLIER_BALANCE_REQUIRED_FOR_CANDIDATE", "candidate supplier must have positive balance")
	}
	if runtimeStatus == adminplusdomain.SupplierRuntimeStatusActive && balanceCents <= 0 {
		return supplierInput{}, badRequest("SUPPLIER_BALANCE_REQUIRED_FOR_ACTIVE", "active supplier must have positive balance")
	}
	dashboardURL, err := normalizeOptionalURL(in.DashboardURL, "SUPPLIER_DASHBOARD_URL_INVALID")
	if err != nil {
		return supplierInput{}, err
	}
	apiBaseURL, err := normalizeOptionalURL(in.APIBaseURL, "SUPPLIER_API_BASE_URL_INVALID")
	if err != nil {
		return supplierInput{}, err
	}
	thirdPartyRechargeURL, err := normalizeOptionalURL(in.ThirdPartyRechargeURL, "SUPPLIER_THIRD_PARTY_RECHARGE_URL_INVALID")
	if err != nil {
		return supplierInput{}, err
	}
	localRechargeURL, err := normalizeOptionalURL(in.LocalRechargeURL, "SUPPLIER_LOCAL_RECHARGE_URL_INVALID")
	if err != nil {
		return supplierInput{}, err
	}
	return supplierInput{
		Name:                  name,
		Kind:                  in.Kind,
		Type:                  in.Type,
		RuntimeStatus:         runtimeStatus,
		HealthStatus:          healthStatus,
		DashboardURL:          dashboardURL,
		APIBaseURL:            apiBaseURL,
		ThirdPartyRechargeURL: thirdPartyRechargeURL,
		LocalRechargeURL:      localRechargeURL,
		BalanceCents:          balanceCents,
		BalanceCurrency:       balanceCurrency,
		RechargeMultiplier:    rechargeMultiplier,
		KeyLimitPolicy:        keyLimitPolicy,
		KeyLimitValue:         keyLimitValue,
	}, nil
}

func normalizeOptionalURL(raw string, reason string) (string, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return "", nil
	}
	u, err := url.ParseRequestURI(v)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", badRequest(reason, "invalid supplier url")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", badRequest(reason, "supplier url must use http or https")
	}
	return v, nil
}

func normalizeCandidateSupplierType(value adminplusdomain.SupplierType) adminplusdomain.SupplierType {
	normalized := adminplusdomain.NormalizeSupplierType(string(value))
	switch normalized {
	case "":
		return adminplusdomain.SupplierTypeSub2API
	default:
		return normalized
	}
}

func inferCandidateSupplierType(value adminplusdomain.SupplierType, urlHints ...string) adminplusdomain.SupplierType {
	if strings.TrimSpace(string(value)) != "" {
		return normalizeCandidateSupplierType(value)
	}
	for _, raw := range urlHints {
		if supplierType := supplierTypeFromURLHint(raw); supplierType != "" {
			return supplierType
		}
	}
	return adminplusdomain.SupplierTypeSub2API
}

func supplierTypeFromURLHint(raw string) adminplusdomain.SupplierType {
	u, ok := parseSupplierIntegrationURL(raw)
	if !ok {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	path := normalizeURLPath(u.Path)
	if host == "api.openai.com" {
		return adminplusdomain.SupplierTypeOpenAI
	}
	if host == "api.anthropic.com" || (host == "anthropic.com" && strings.HasPrefix(path, "/v1")) {
		return adminplusdomain.SupplierTypeAnthropic
	}
	if host == "generativelanguage.googleapis.com" || host == "gemini.google.com" || host == "cloudcode-pa.googleapis.com" || ((host == "googleapis.com" || strings.HasSuffix(host, ".googleapis.com")) && strings.HasPrefix(path, "/v1beta/openai")) {
		return adminplusdomain.SupplierTypeGemini
	}
	if containsAny(host, "sub2api", "sub2-api", "subapi", "sub-api") {
		return adminplusdomain.SupplierTypeSub2API
	}
	if containsAny(host, "newapi", "new-api", "oneapi", "one-api", "onehub", "one-hub", "donehub", "done-hub", "veloera", "anyrouter", "vo-api", "voapi", "super-api", "superapi", "rix-api", "rixapi", "neo-api", "neoapi", "wong-gongyi") {
		return adminplusdomain.SupplierTypeNewAPI
	}
	return ""
}

func candidateSupplierKind(supplierType adminplusdomain.SupplierType) adminplusdomain.SupplierKind {
	switch supplierType {
	case adminplusdomain.SupplierTypeOpenAI, adminplusdomain.SupplierTypeAnthropic, adminplusdomain.SupplierTypeGemini:
		return adminplusdomain.SupplierKindSourceAccount
	case adminplusdomain.SupplierTypeBrowserOnly:
		return adminplusdomain.SupplierKindBrowserOnly
	default:
		return adminplusdomain.SupplierKindRelay
	}
}

func defaultCurrencyForCandidateSupplier(supplierType adminplusdomain.SupplierType) string {
	return "USD"
}

func normalizeCandidateAPIBaseURL(raw string, fallback *url.URL) (string, error) {
	v := strings.TrimSpace(raw)
	if v == "" && fallback != nil {
		v = inferCandidateAPIBaseURL(fallback)
	}
	v = canonicalizeCandidateAPIBaseURL(v)
	return normalizeOptionalURL(v, "SUPPLIER_API_BASE_URL_INVALID")
}

func inferCandidateAPIBaseURL(u *url.URL) string {
	if u == nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	path := normalizeURLPath(u.Path)
	if path == "/" {
		return u.Scheme + "://" + u.Host
	}
	if basePath, ok := knownAPIEndpointBasePaths[path]; ok {
		return urlWithPath(u, basePath)
	}
	if semanticAPIBasePaths[path] || strings.HasPrefix(path, "/api/") {
		return urlWithPath(u, path)
	}
	return u.Scheme + "://" + u.Host
}

func canonicalizeCandidateAPIBaseURL(raw string) string {
	v := strings.TrimSpace(raw)
	if v == "" {
		return ""
	}
	u, err := url.Parse(v)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return v
	}
	path := normalizeURLPath(u.Path)
	if basePath, ok := knownAPIEndpointBasePaths[path]; ok {
		return urlWithPath(u, basePath)
	}
	if path == "/" {
		u.Path = ""
		u.RawPath = ""
	}
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

func normalizeURLPath(path string) string {
	normalized := strings.TrimSpace(path)
	if normalized == "" || normalized == "/" {
		return "/"
	}
	if !strings.HasPrefix(normalized, "/") {
		normalized = "/" + normalized
	}
	for len(normalized) > 1 && strings.HasSuffix(normalized, "/") {
		normalized = strings.TrimSuffix(normalized, "/")
	}
	return normalized
}

func urlWithPath(in *url.URL, path string) string {
	if in == nil {
		return ""
	}
	u := *in
	if path == "/" {
		u.Path = ""
	} else {
		u.Path = path
	}
	u.RawPath = ""
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

var knownAPIEndpointBasePaths = map[string]string{
	"/v1":                  "/v1",
	"/v1/models":           "/v1",
	"/v1/chat/completions": "/v1",
	"/v1/responses":        "/v1",
	"/v1/messages":         "/v1",
	"/v1beta":              "/v1beta",
	"/v1beta/models":       "/v1beta",
	"/api/v1":              "/api/v1",
	"/api/v1/models":       "/api/v1",
	"/api/v1/user/self":    "/api/v1",
	"/api/v1/users/self":   "/api/v1",
}

var semanticAPIBasePaths = map[string]bool{
	"/anthropic":          true,
	"/api/anthropic":      true,
	"/api/coding/v3":      true,
	"/apps/anthropic":     true,
	"/api/coding/paas/v4": true,
	"/v1beta/openai":      true,
}

func normalizeCurrency(value string) string {
	v := strings.ToUpper(strings.TrimSpace(value))
	if v == "" || v == "QTA" || v == "CNY" {
		return "USD"
	}
	if len(v) != 3 {
		return "USD"
	}
	return v
}

func normalizeBalanceAmountAndCurrency(cents int64, currency string) (int64, string) {
	v := strings.ToUpper(strings.TrimSpace(currency))
	if v == "QTA" {
		return int64(math.Round(float64(cents) / legacyNewAPIQuotaUnitsPerUSD)), "USD"
	}
	return cents, normalizeCurrency(v)
}

func normalizeRechargeMultiplier(value float64) (float64, error) {
	if value == 0 {
		return 1, nil
	}
	if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, badRequest("SUPPLIER_RECHARGE_MULTIPLIER_INVALID", "recharge multiplier must be positive")
	}
	return value, nil
}

func normalizeKeyLimit(policy string, value int) (string, int, error) {
	normalized := strings.ToLower(strings.TrimSpace(policy))
	if normalized == "" {
		normalized = adminplusdomain.SupplierKeyLimitPolicyUnknown
	}
	switch normalized {
	case adminplusdomain.SupplierKeyLimitPolicyUnknown, adminplusdomain.SupplierKeyLimitPolicyUnlimited, adminplusdomain.SupplierKeyLimitPolicyUnsupported:
		if value < 0 {
			return "", 0, badRequest("SUPPLIER_KEY_LIMIT_VALUE_INVALID", "key limit value cannot be negative")
		}
		return normalized, 0, nil
	case adminplusdomain.SupplierKeyLimitPolicyLimited:
		if value <= 0 {
			return "", 0, badRequest("SUPPLIER_KEY_LIMIT_VALUE_REQUIRED", "limited key policy requires a positive value")
		}
		return normalized, value, nil
	default:
		return "", 0, badRequest("SUPPLIER_KEY_LIMIT_POLICY_INVALID", "invalid key limit policy")
	}
}

func supplierKeyCapacityStatus(policy string, limit int, activeCount int) string {
	policy, limit, err := normalizeKeyLimit(policy, limit)
	if err != nil {
		return adminplusdomain.SupplierKeyCapacityUnknown
	}
	if activeCount < 0 {
		activeCount = 0
	}
	switch policy {
	case adminplusdomain.SupplierKeyLimitPolicyUnlimited:
		return adminplusdomain.SupplierKeyCapacityAvailable
	case adminplusdomain.SupplierKeyLimitPolicyUnsupported:
		return adminplusdomain.SupplierKeyCapacityUnsupported
	case adminplusdomain.SupplierKeyLimitPolicyLimited:
		if activeCount >= limit {
			return adminplusdomain.SupplierKeyCapacityExhausted
		}
		if activeCount >= limit-1 {
			return adminplusdomain.SupplierKeyCapacityLimited
		}
		return adminplusdomain.SupplierKeyCapacityAvailable
	default:
		return adminplusdomain.SupplierKeyCapacityUnknown
	}
}

func normalizeSupplierForRead(in *adminplusdomain.Supplier) *adminplusdomain.Supplier {
	if in == nil {
		return nil
	}
	out := *in
	out.BalanceCents, out.BalanceCurrency = normalizeBalanceAmountAndCurrency(in.BalanceCents, in.BalanceCurrency)
	if multiplier, err := normalizeRechargeMultiplier(in.RechargeMultiplier); err == nil {
		out.RechargeMultiplier = multiplier
	} else {
		out.RechargeMultiplier = 1
	}
	if in.BalanceUpdatedAt != nil {
		t := *in.BalanceUpdatedAt
		out.BalanceUpdatedAt = &t
	}
	if policy, limit, err := normalizeKeyLimit(in.KeyLimitPolicy, in.KeyLimitValue); err == nil {
		out.KeyLimitPolicy = policy
		out.KeyLimitValue = limit
	} else {
		out.KeyLimitPolicy = adminplusdomain.SupplierKeyLimitPolicyUnknown
		out.KeyLimitValue = 0
	}
	out.KeyCapacityStatus = supplierKeyCapacityStatus(out.KeyLimitPolicy, out.KeyLimitValue, out.ActiveKeyCount)
	out.Capabilities = buildSupplierCapabilities(&out)
	out.IntegrationHint = buildSupplierIntegrationHint(&out)
	out.PlatformHint = buildSupplierPlatformHint(&out)
	out.APIEndpointCandidates = buildSupplierAPIEndpointCandidates(&out)
	out.URLHints = buildSupplierURLHints(&out)
	out.OperationHints = buildSupplierOperationHints(&out)
	return &out
}
