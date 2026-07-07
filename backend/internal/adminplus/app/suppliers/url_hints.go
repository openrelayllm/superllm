package suppliers

import (
	"net/url"
	"strings"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
)

func buildSupplierURLHints(in *adminplusdomain.Supplier) []adminplusdomain.SupplierURLHint {
	if in == nil {
		return nil
	}
	hints := make([]adminplusdomain.SupplierURLHint, 0, 2)
	if hint, ok := analyzeSupplierURLHint("dashboard_url", in.DashboardURL); ok {
		hints = append(hints, hint)
	}
	if hint, ok := analyzeSupplierURLHint("api_base_url", in.APIBaseURL); ok {
		hints = append(hints, hint)
	}
	return hints
}

func analyzeSupplierURLHint(source string, raw string) (adminplusdomain.SupplierURLHint, bool) {
	u, ok := parseSupplierIntegrationURL(raw)
	if !ok {
		return adminplusdomain.SupplierURLHint{}, false
	}
	u.RawQuery = ""
	u.Fragment = ""
	path := normalizeURLPath(u.Path)
	canonicalURL := urlWithPath(u, path)
	if source == "dashboard_url" {
		return analyzeSupplierDashboardURLHint(canonicalURL, u, path), true
	}
	return analyzeSupplierAPIBaseURLHint(canonicalURL, u, path), true
}

func analyzeSupplierDashboardURLHint(canonicalURL string, u *url.URL, path string) adminplusdomain.SupplierURLHint {
	if path == "/" {
		return supplierURLHint("dashboard_url", "后台根地址", canonicalURL, "dashboard_url", "root", "info", path, "", "后台地址已是站点根地址，适合用于浏览器登录、会话采集和运营入口跳转。")
	}
	if suggestedPath, ok := knownAPIEndpointBasePaths[path]; ok {
		return supplierURLHint("dashboard_url", "后台疑似 API", canonicalURL, "dashboard_url", "api_path_in_dashboard", "warning", path, urlWithPath(u, "/"), "后台地址看起来是 API 路径；建议后台地址保留站点根地址，API Base 单独配置为 "+urlWithPath(u, suggestedPath)+"。")
	}
	if semanticAPIBasePaths[path] {
		return supplierURLHint("dashboard_url", "语义后台路径", canonicalURL, "dashboard_url", "semantic_path", "info", path, "", "后台地址包含需要保留的语义路径，适合兼容入口或特定产品入口识别。")
	}
	if strings.HasPrefix(path, "/api") {
		return supplierURLHint("dashboard_url", "后台 API 路径", canonicalURL, "dashboard_url", "api_path", "warning", path, urlWithPath(u, "/"), "后台地址包含 API 路径；如这是浏览器后台入口请保留，否则建议拆分为后台根地址和 API Base。")
	}
	return supplierURLHint("dashboard_url", "后台自定义路径", canonicalURL, "dashboard_url", "custom_path", "info", path, "", "后台地址包含自定义路径，将按当前配置展示和跳转。")
}

func analyzeSupplierAPIBaseURLHint(canonicalURL string, u *url.URL, path string) adminplusdomain.SupplierURLHint {
	if basePath, ok := knownAPIEndpointBasePaths[path]; ok {
		if basePath != path {
			return supplierURLHint("api_base_url", "接口路径", canonicalURL, "api_base_url", "endpoint_path", "warning", path, urlWithPath(u, basePath), "API Base 看起来是具体接口路径；建议改为 "+urlWithPath(u, basePath)+"，避免后续本地账号绑定时重复拼接路径。")
		}
		return supplierURLHint("api_base_url", "标准 API Base", canonicalURL, "api_base_url", "standard_base", "success", path, "", "API Base 是已知标准基础路径。")
	}
	if semanticAPIBasePaths[path] {
		return supplierURLHint("api_base_url", "语义 API Base", canonicalURL, "api_base_url", "semantic_base", "success", path, "", "API Base 是需要保留的语义路径，适合 Coding Plan、Anthropic 兼容入口或 OpenAI 兼容子路径。")
	}
	if path == "/" {
		return supplierURLHint("api_base_url", "根 API Base", canonicalURL, "api_base_url", "root_base", "info", path, "", "API Base 使用站点根地址，适合部分 OpenAI 兼容服务。")
	}
	if strings.HasPrefix(path, "/api") {
		return supplierURLHint("api_base_url", "自定义 API Base", canonicalURL, "api_base_url", "custom_api_base", "info", path, "", "API Base 使用自定义 API 路径，将按当前配置传递给本地账号绑定。")
	}
	return supplierURLHint("api_base_url", "自定义路径", canonicalURL, "api_base_url", "custom_path", "info", path, "", "API Base 使用自定义路径；如上游要求该路径，可保持当前配置。")
}

func supplierURLHint(key string, label string, rawURL string, source string, action string, severity string, matchedPath string, suggestedURL string, description string) adminplusdomain.SupplierURLHint {
	return adminplusdomain.SupplierURLHint{
		Key:          key,
		Label:        label,
		URL:          rawURL,
		Source:       source,
		Action:       action,
		Severity:     severity,
		MatchedPath:  matchedPath,
		SuggestedURL: suggestedURL,
		Description:  description,
	}
}
