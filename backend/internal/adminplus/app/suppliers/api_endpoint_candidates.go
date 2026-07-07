package suppliers

import (
	"net/url"
	"strings"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
)

func buildSupplierAPIEndpointCandidates(in *adminplusdomain.Supplier) []adminplusdomain.SupplierAPIEndpointCandidate {
	if in == nil {
		return nil
	}
	candidates := make([]adminplusdomain.SupplierAPIEndpointCandidate, 0, 4)
	seen := make(map[string]bool)
	addCandidate := func(candidate adminplusdomain.SupplierAPIEndpointCandidate) {
		candidate.URL = normalizeAPIEndpointCandidateURL(candidate.URL)
		if candidate.URL == "" {
			return
		}
		key := strings.ToLower(candidate.URL)
		if seen[key] {
			return
		}
		seen[key] = true
		if candidate.ID == "" {
			candidate.ID = candidate.Source + ":" + key
		}
		candidates = append(candidates, candidate)
	}

	if strings.TrimSpace(in.APIBaseURL) != "" {
		addCandidate(adminplusdomain.SupplierAPIEndpointCandidate{
			ID:          "configured_api_base",
			Label:       "当前 API Base",
			URL:         in.APIBaseURL,
			Protocol:    supplierEndpointProtocol(in),
			Source:      "configured",
			Recommended: true,
			Description: "当前供应商配置的 API Base URL。",
		})
	}

	for _, seed := range supplierEndpointSeedURLs(in) {
		for _, preset := range supplierIntegrationPresets {
			if !stringInSlice(strings.ToLower(seed.Hostname()), preset.hosts) {
				continue
			}
			for _, path := range preset.paths {
				addCandidate(adminplusdomain.SupplierAPIEndpointCandidate{
					ID:          preset.id + ":" + path,
					Label:       preset.providerLabel + " / " + supplierEndpointProtocolLabel(preset.protocol),
					URL:         urlWithPath(seed, path),
					Protocol:    preset.protocol,
					Source:      "integration_preset",
					Recommended: true,
					Description: preset.description,
				})
			}
		}
		for _, preset := range manualSupplierIntegrationPresets {
			if !stringInSlice(strings.ToLower(seed.Hostname()), preset.hosts) {
				continue
			}
			for _, path := range preset.paths {
				addCandidate(adminplusdomain.SupplierAPIEndpointCandidate{
					ID:          preset.id + ":" + path,
					Label:       preset.providerLabel + " / " + supplierEndpointProtocolLabel(preset.protocol),
					URL:         urlWithPath(seed, path),
					Protocol:    preset.protocol,
					Source:      "manual_integration_preset",
					Recommended: false,
					Description: preset.description,
				})
			}
		}
	}

	for _, candidate := range conventionalSupplierAPIEndpointCandidates(in) {
		addCandidate(candidate)
	}

	if len(candidates) > 4 {
		return candidates[:4]
	}
	return candidates
}

func supplierEndpointSeedURLs(in *adminplusdomain.Supplier) []*url.URL {
	if in == nil {
		return nil
	}
	seeds := make([]*url.URL, 0, 2)
	seenHosts := make(map[string]bool)
	for _, raw := range []string{in.APIBaseURL, in.DashboardURL} {
		u, ok := parseSupplierIntegrationURL(raw)
		if !ok {
			continue
		}
		host := strings.ToLower(u.Hostname())
		if host == "" || seenHosts[host] {
			continue
		}
		seenHosts[host] = true
		seeds = append(seeds, u)
	}
	return seeds
}

func conventionalSupplierAPIEndpointCandidates(in *adminplusdomain.Supplier) []adminplusdomain.SupplierAPIEndpointCandidate {
	if in == nil {
		return nil
	}
	candidates := make([]adminplusdomain.SupplierAPIEndpointCandidate, 0, 1)
	apiBaseConfigured := strings.TrimSpace(in.APIBaseURL) != ""
	if !apiBaseConfigured {
		if origin := supplierDashboardOrigin(in.DashboardURL); origin != "" {
			switch in.Type {
			case adminplusdomain.SupplierTypeSub2API, adminplusdomain.SupplierTypeNewAPI:
				candidates = append(candidates, adminplusdomain.SupplierAPIEndpointCandidate{
					ID:          "dashboard_api_v1",
					Label:       "后台 API",
					URL:         origin + "/api/v1",
					Source:      "derived",
					Recommended: true,
					Description: "从供应商后台域名派生的管理 API Base，用于会话、余额、分组和成本读取。",
				})
			}
		}
	}
	if apiBaseConfigured || supplierHasIntegrationPresetHost(in.DashboardURL) {
		return candidates
	}
	switch in.Type {
	case adminplusdomain.SupplierTypeOpenAI:
		candidates = append(candidates, officialSourceEndpointCandidate("official_openai_v1", "OpenAI 官方 API", "https://api.openai.com/v1", "openai"))
	case adminplusdomain.SupplierTypeAnthropic:
		candidates = append(candidates, officialSourceEndpointCandidate("official_anthropic_v1", "Anthropic 官方 API", "https://api.anthropic.com/v1", "claude"))
	case adminplusdomain.SupplierTypeGemini:
		candidates = append(candidates, officialSourceEndpointCandidate("official_gemini_v1beta", "Gemini 官方 API", "https://generativelanguage.googleapis.com/v1beta", "gemini"))
	}
	return candidates
}

func supplierHasIntegrationPresetHost(raw string) bool {
	u, ok := parseSupplierIntegrationURL(raw)
	if !ok {
		return false
	}
	host := strings.ToLower(u.Hostname())
	for _, preset := range supplierIntegrationPresets {
		if stringInSlice(host, preset.hosts) {
			return true
		}
	}
	return false
}

func officialSourceEndpointCandidate(id string, label string, endpointURL string, protocol string) adminplusdomain.SupplierAPIEndpointCandidate {
	return adminplusdomain.SupplierAPIEndpointCandidate{
		ID:          id,
		Label:       label,
		URL:         endpointURL,
		Protocol:    protocol,
		Source:      "derived",
		Recommended: true,
		Description: "源站官方 API Base，可用于本地 Sub2API 账号绑定和健康探测。",
	}
}

func supplierDashboardOrigin(raw string) string {
	u, ok := parseSupplierIntegrationURL(raw)
	if !ok {
		return ""
	}
	u.Path = ""
	u.RawPath = ""
	u.RawQuery = ""
	u.Fragment = ""
	return strings.TrimRight(u.String(), "/")
}

func normalizeAPIEndpointCandidateURL(raw string) string {
	u, ok := parseSupplierIntegrationURL(raw)
	if !ok {
		return strings.TrimRight(strings.TrimSpace(raw), "/")
	}
	return urlWithPath(u, normalizeURLPath(u.Path))
}

func supplierEndpointProtocol(in *adminplusdomain.Supplier) string {
	if in == nil {
		return ""
	}
	if in.IntegrationHint != nil {
		return normalizeIntegrationProtocol(in.IntegrationHint.Protocol)
	}
	return supplierIntegrationPreferredProtocol(in.Type)
}

func supplierEndpointProtocolLabel(protocol string) string {
	switch normalizeIntegrationProtocol(protocol) {
	case "openai":
		return "OpenAI"
	case "claude":
		return "Claude"
	case "gemini":
		return "Gemini"
	default:
		return "API"
	}
}
