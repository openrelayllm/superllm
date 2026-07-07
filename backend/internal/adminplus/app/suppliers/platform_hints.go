package suppliers

import (
	"strings"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
)

type supplierPlatformHintPreset struct {
	id          string
	label       string
	family      string
	description string
	needles     []string
}

var supplierPlatformHintPresets = []supplierPlatformHintPreset{
	{
		id:          "anyrouter",
		label:       "AnyRouter",
		family:      "new_api",
		description: "识别为 AnyRouter / New API 家族站点，适合按供应商后台与分组维度做运营观测。",
		needles:     []string{"anyrouter", "any router"},
	},
	{
		id:          "done-hub",
		label:       "DoneHub",
		family:      "new_api",
		description: "识别为 DoneHub / New API 家族站点，供应商能力应按 New API 兼容边界处理。",
		needles:     []string{"donehub", "done-hub", "done hub"},
	},
	{
		id:          "one-hub",
		label:       "OneHub",
		family:      "new_api",
		description: "识别为 OneHub / New API 家族站点，供应商能力应按 New API 兼容边界处理。",
		needles:     []string{"onehub", "one-hub", "one hub"},
	},
	{
		id:          "veloera",
		label:       "Veloera",
		family:      "new_api",
		description: "识别为 Veloera / New API 家族站点，供应商能力应按 New API 兼容边界处理。",
		needles:     []string{"veloera"},
	},
	{
		id:          "sub2api",
		label:       "Sub2API",
		family:      "sub2api",
		description: "识别为 Sub2API 供应商后台，可使用 Provider Router 读取余额、分组、费率和成本事实。",
		needles:     []string{"sub2api", "sub2-api", "sub2 api", "subapi", "sub-api", "sub api"},
	},
	{
		id:          "new-api",
		label:       "New API",
		family:      "new_api",
		description: "识别为 New API 家族站点，适合做供应商后台运营观测与本地账号绑定提示。",
		needles:     []string{"newapi", "new-api", "new api", "vo-api", "voapi", "super-api", "superapi", "rix-api", "rixapi", "neo-api", "neoapi", "wong-gongyi"},
	},
	{
		id:          "one-api",
		label:       "One API",
		family:      "new_api",
		description: "识别为 One API 家族站点，通常按 New API 类兼容后台处理。",
		needles:     []string{"oneapi", "one-api", "one api"},
	},
	{
		id:          "cliproxyapi",
		label:       "CLI Proxy API",
		family:      "cli_proxy",
		description: "识别为 CLI Proxy API 兼容服务，适合先做兼容性与模型身份观测。",
		needles:     []string{"cliproxyapi", "cli-proxy-api", "cli proxy api"},
	},
}

func buildSupplierPlatformHint(in *adminplusdomain.Supplier) *adminplusdomain.SupplierPlatformHint {
	if in == nil {
		return nil
	}
	for _, item := range []struct {
		value  string
		source string
	}{
		{value: in.APIBaseURL, source: "url_hint"},
		{value: in.DashboardURL, source: "url_hint"},
		{value: in.Name, source: "name_hint"},
	} {
		if hint := supplierPlatformHintFromValue(item.value, item.source); hint != nil {
			return hint
		}
	}
	if in.IntegrationHint != nil && strings.TrimSpace(in.IntegrationHint.ProviderLabel) != "" {
		return supplierPlatformHint(
			in.IntegrationHint.ID,
			in.IntegrationHint.ProviderLabel,
			"api_provider",
			"integration_preset",
			in.IntegrationHint.Description,
		)
	}
	return supplierPlatformHintFromType(in.Type)
}

func supplierPlatformHintFromValue(value string, source string) *adminplusdomain.SupplierPlatformHint {
	text := strings.TrimSpace(value)
	if text == "" {
		return nil
	}
	if source == "url_hint" {
		if hint := supplierPlatformHintFromURL(text); hint != nil {
			return hint
		}
	}
	for _, preset := range supplierPlatformHintPresets {
		if supplierPlatformTextMatches(text, preset.needles) {
			return supplierPlatformHintFromPreset(preset, source)
		}
	}
	return nil
}

func supplierPlatformHintFromURL(raw string) *adminplusdomain.SupplierPlatformHint {
	u, ok := parseSupplierIntegrationURL(raw)
	if !ok {
		return nil
	}
	host := strings.ToLower(u.Hostname())
	path := strings.ToLower(normalizeURLPath(u.Path))
	switch {
	case host == "api.openai.com":
		return supplierPlatformHint("openai", "OpenAI", "source_account", "url_hint", "识别为 OpenAI 官方 API 入口。")
	case host == "api.anthropic.com" || (host == "anthropic.com" && strings.HasPrefix(path, "/v1")):
		return supplierPlatformHint("anthropic", "Anthropic", "source_account", "url_hint", "识别为 Anthropic 官方 API 入口。")
	case host == "generativelanguage.googleapis.com" || host == "gemini.google.com" || ((host == "googleapis.com" || strings.HasSuffix(host, ".googleapis.com")) && strings.HasPrefix(path, "/v1beta/openai")):
		return supplierPlatformHint("gemini", "Google Gemini", "source_account", "url_hint", "识别为 Google Gemini 官方 API 入口。")
	case host == "chatgpt.com" && strings.HasPrefix(path, "/backend-api/codex"):
		return supplierPlatformHint("codex", "ChatGPT Codex", "source_account", "url_hint", "识别为 ChatGPT Codex 后端入口，仅作为源站身份提示。")
	case host == "cloudcode-pa.googleapis.com":
		return supplierPlatformHint("gemini-cli", "Gemini CLI", "source_account", "url_hint", "识别为 Gemini CLI / Code Assist 官方入口。")
	case (host == "127.0.0.1" || host == "localhost") && u.Port() == "8317":
		return supplierPlatformHint("cliproxyapi", "CLI Proxy API", "cli_proxy", "url_hint", "识别为 CLI Proxy API 兼容服务，适合先做兼容性与模型身份观测。")
	default:
		return nil
	}
}

func supplierPlatformHintFromType(supplierType adminplusdomain.SupplierType) *adminplusdomain.SupplierPlatformHint {
	switch supplierType {
	case adminplusdomain.SupplierTypeSub2API:
		return supplierPlatformHint("sub2api", "Sub2API", "sub2api", "type", "供应商类型配置为 Sub2API。")
	case adminplusdomain.SupplierTypeNewAPI:
		return supplierPlatformHint("new-api", "New API", "new_api", "type", "供应商类型配置为 New API。")
	case adminplusdomain.SupplierTypeOpenAI:
		return supplierPlatformHint("openai", "OpenAI", "source_account", "type", "供应商类型配置为 OpenAI 源站账号。")
	case adminplusdomain.SupplierTypeAnthropic:
		return supplierPlatformHint("anthropic", "Anthropic", "source_account", "type", "供应商类型配置为 Anthropic 源站账号。")
	case adminplusdomain.SupplierTypeGemini:
		return supplierPlatformHint("gemini", "Google Gemini", "source_account", "type", "供应商类型配置为 Gemini 源站账号。")
	case adminplusdomain.SupplierTypeBrowserOnly:
		return supplierPlatformHint("browser-only", "仅浏览器", "browser_only", "type", "供应商类型配置为仅浏览器采集。")
	default:
		return nil
	}
}

func supplierPlatformTextMatches(value string, needles []string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	compact := strings.NewReplacer("-", "", "_", "", " ", "").Replace(normalized)
	for _, needle := range needles {
		current := strings.ToLower(strings.TrimSpace(needle))
		if current == "" {
			continue
		}
		if strings.Contains(normalized, current) {
			return true
		}
		currentCompact := strings.NewReplacer("-", "", "_", "", " ", "").Replace(current)
		if currentCompact != "" && strings.Contains(compact, currentCompact) {
			return true
		}
	}
	return false
}

func supplierPlatformHintFromPreset(preset supplierPlatformHintPreset, source string) *adminplusdomain.SupplierPlatformHint {
	return supplierPlatformHint(preset.id, preset.label, preset.family, source, preset.description)
}

func supplierPlatformHint(id string, label string, family string, source string, description string) *adminplusdomain.SupplierPlatformHint {
	return &adminplusdomain.SupplierPlatformHint{
		ID:          id,
		Label:       label,
		Family:      family,
		Source:      source,
		Description: description,
	}
}
