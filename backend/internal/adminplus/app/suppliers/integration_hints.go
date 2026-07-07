package suppliers

import (
	"net/url"
	"strings"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
)

type supplierIntegrationPreset struct {
	id                        string
	label                     string
	providerLabel             string
	protocol                  string
	description               string
	docsURL                   string
	recommendedSkipModelFetch bool
	recommendedModels         []string
	hosts                     []string
	paths                     []string
}

var supplierIntegrationPresets = []supplierIntegrationPreset{
	{
		id:                        "codingplan-openai",
		label:                     "阿里云 CodingPlan / OpenAI",
		providerLabel:             "阿里云 CodingPlan",
		protocol:                  "openai",
		description:               "适合阿里云 CodingPlan 的 OpenAI 兼容入口，建议先添加 API Key，再补入推荐模型完成初始化。",
		docsURL:                   "https://help.aliyun.com/zh/model-studio/coding-plan-faq",
		recommendedSkipModelFetch: true,
		recommendedModels:         []string{"qwen3-coder-plus", "qwen3-coder-next", "qwen3.5-plus", "glm-5"},
		hosts:                     []string{"coding.dashscope.aliyuncs.com"},
		paths:                     []string{"/v1"},
	},
	{
		id:                        "codingplan-claude",
		label:                     "阿里云 CodingPlan / Claude",
		providerLabel:             "阿里云 CodingPlan",
		protocol:                  "claude",
		description:               "适合阿里云 CodingPlan 的 Claude 兼容入口，建议先添加 API Key，再补入推荐模型完成初始化。",
		docsURL:                   "https://help.aliyun.com/zh/model-studio/coding-plan-faq",
		recommendedSkipModelFetch: true,
		recommendedModels:         []string{"qwen3-coder-plus", "qwen3-coder-next", "qwen3.5-plus", "glm-5"},
		hosts:                     []string{"coding.dashscope.aliyuncs.com"},
		paths:                     []string{"/apps/anthropic"},
	},
	{
		id:                        "zhipu-coding-plan-openai",
		label:                     "智谱 Coding Plan / OpenAI",
		providerLabel:             "智谱 Coding Plan",
		protocol:                  "openai",
		description:               "适合智谱 Coding Plan 的 OpenAI 兼容入口，建议先添加 API Key，再补入常用 GLM 编程模型。",
		docsURL:                   "https://docs.bigmodel.cn/cn/coding-plan/faq",
		recommendedSkipModelFetch: true,
		recommendedModels:         []string{"glm-4.7", "glm-4.6", "glm-4.5", "glm-4.5-air"},
		hosts:                     []string{"open.bigmodel.cn"},
		paths:                     []string{"/api/coding/paas/v4"},
	},
	{
		id:                        "deepseek-openai",
		label:                     "DeepSeek / OpenAI",
		providerLabel:             "DeepSeek",
		protocol:                  "openai",
		description:               "适合 DeepSeek 官方 OpenAI 兼容入口，建议直接添加 API Key，并优先补入官方常用编程模型。",
		docsURL:                   "https://api-docs.deepseek.com/",
		recommendedSkipModelFetch: true,
		recommendedModels:         []string{"deepseek-chat", "deepseek-reasoner"},
		hosts:                     []string{"api.deepseek.com"},
		paths:                     []string{"/", "/v1"},
	},
	{
		id:                        "deepseek-claude",
		label:                     "DeepSeek / Claude",
		providerLabel:             "DeepSeek",
		protocol:                  "claude",
		description:               "适合 DeepSeek 官方 Anthropic 兼容入口，便于 Claude Code 一类工具直接接入。",
		docsURL:                   "https://api-docs.deepseek.com/guides/anthropic_api",
		recommendedSkipModelFetch: true,
		recommendedModels:         []string{"deepseek-chat", "deepseek-reasoner"},
		hosts:                     []string{"api.deepseek.com"},
		paths:                     []string{"/anthropic"},
	},
	{
		id:                        "moonshot-openai",
		label:                     "Moonshot(Kimi) / OpenAI",
		providerLabel:             "Moonshot / Kimi",
		protocol:                  "openai",
		description:               "适合 Moonshot 官方 OpenAI 兼容入口，推荐优先使用 Kimi 系列编程与 Agent 模型。",
		docsURL:                   "https://platform.moonshot.cn/",
		recommendedSkipModelFetch: true,
		recommendedModels:         []string{"kimi-k2.5", "kimi-k2", "kimi-k2-thinking"},
		hosts:                     []string{"api.moonshot.cn"},
		paths:                     []string{"/", "/v1"},
	},
	{
		id:                        "moonshot-claude",
		label:                     "Moonshot(Kimi) / Claude",
		providerLabel:             "Moonshot / Kimi",
		protocol:                  "claude",
		description:               "适合 Moonshot 官方 Anthropic 兼容入口，便于 Claude Code 与同类工具接入 Kimi。",
		docsURL:                   "https://platform.moonshot.cn/blog/posts/kimi-k2-0905",
		recommendedSkipModelFetch: true,
		recommendedModels:         []string{"kimi-k2.5", "kimi-k2", "kimi-k2-thinking"},
		hosts:                     []string{"api.moonshot.cn"},
		paths:                     []string{"/anthropic"},
	},
	{
		id:                        "minimax-openai",
		label:                     "MiniMax / OpenAI",
		providerLabel:             "MiniMax",
		protocol:                  "openai",
		description:               "适合 MiniMax 官方 OpenAI 兼容入口，建议直接添加 API Key 后补入常用 M2 编程模型。",
		docsURL:                   "https://platform.minimaxi.com/docs/api-reference/api-overview",
		recommendedSkipModelFetch: true,
		recommendedModels:         []string{"MiniMax-M2.7", "MiniMax-M2.5", "MiniMax-M2.1"},
		hosts:                     []string{"api.minimaxi.com"},
		paths:                     []string{"/", "/v1"},
	},
	{
		id:                        "minimax-claude",
		label:                     "MiniMax / Claude",
		providerLabel:             "MiniMax",
		protocol:                  "claude",
		description:               "适合 MiniMax 官方 Anthropic 兼容入口，适配 Claude Code 等编程工具场景。",
		docsURL:                   "https://platform.minimaxi.com/docs/api-reference/text-anthropic-api",
		recommendedSkipModelFetch: true,
		recommendedModels:         []string{"MiniMax-M2.7", "MiniMax-M2.5", "MiniMax-M2.1"},
		hosts:                     []string{"api.minimaxi.com"},
		paths:                     []string{"/anthropic"},
	},
	{
		id:                        "modelscope-openai",
		label:                     "ModelScope / OpenAI",
		providerLabel:             "ModelScope",
		protocol:                  "openai",
		description:               "适合 ModelScope API-Inference 的 OpenAI 兼容入口，适合直接接入常用开源编程模型。",
		docsURL:                   "https://www.modelscope.cn/docs/model-service/API-Inference/intro",
		recommendedSkipModelFetch: true,
		recommendedModels:         []string{"Qwen/Qwen3-32B", "Qwen/Qwen2.5-Coder-32B-Instruct", "deepseek-ai/DeepSeek-V3.2"},
		hosts:                     []string{"api-inference.modelscope.cn"},
		paths:                     []string{"/v1"},
	},
	{
		id:                        "modelscope-claude",
		label:                     "ModelScope / Claude",
		providerLabel:             "ModelScope",
		protocol:                  "claude",
		description:               "适合 ModelScope API-Inference 的 Claude 兼容入口，便于接入 Claude Code 一类工具。",
		docsURL:                   "https://www.modelscope.cn/docs/model-service/API-Inference/intro",
		recommendedSkipModelFetch: true,
		recommendedModels:         []string{"Qwen/Qwen3-32B", "Qwen/Qwen2.5-Coder-32B-Instruct", "deepseek-ai/DeepSeek-V3.2"},
		hosts:                     []string{"api-inference.modelscope.cn"},
		paths:                     []string{"/"},
	},
	{
		id:                        "doubao-coding-openai",
		label:                     "豆包 Coding Plan / OpenAI",
		providerLabel:             "豆包 Coding Plan",
		protocol:                  "openai",
		description:               "适合火山方舟 Coding Plan 的 OpenAI 兼容入口，推荐优先使用 ark-code 与豆包编程模型。",
		docsURL:                   "https://www.volcengine.com/docs/82379/2205646?lang=zh",
		recommendedSkipModelFetch: true,
		recommendedModels:         []string{"ark-code-latest", "doubao-seed-2.0-code", "doubao-seed-2.0-pro"},
		hosts:                     []string{"ark.cn-beijing.volces.com"},
		paths:                     []string{"/api/coding/v3"},
	},
	{
		id:            "openai-official",
		label:         "OpenAI 官方 / OpenAI",
		providerLabel: "OpenAI",
		protocol:      "openai",
		description:   "识别为 OpenAI 官方兼容入口，可作为源站账号绑定和健康探测对象。",
		docsURL:       "https://platform.openai.com/docs",
		hosts:         []string{"api.openai.com"},
		paths:         []string{"/", "/v1"},
	},
	{
		id:            "anthropic-official",
		label:         "Anthropic 官方 / Claude",
		providerLabel: "Anthropic",
		protocol:      "claude",
		description:   "识别为 Anthropic 官方入口，可作为源站账号绑定和健康探测对象。",
		docsURL:       "https://docs.anthropic.com/",
		hosts:         []string{"api.anthropic.com"},
		paths:         []string{"/", "/v1"},
	},
	{
		id:            "gemini-official",
		label:         "Google Gemini 官方 / Gemini",
		providerLabel: "Google Gemini",
		protocol:      "gemini",
		description:   "识别为 Google Gemini 官方入口，可作为源站账号绑定和健康探测对象。",
		docsURL:       "https://ai.google.dev/gemini-api/docs",
		hosts:         []string{"generativelanguage.googleapis.com"},
		paths:         []string{"/", "/v1beta"},
	},
}

var manualSupplierIntegrationPresets = []supplierIntegrationPreset{
	{
		id:                        "zhipu-coding-plan-claude",
		label:                     "智谱 Coding Plan / Claude",
		providerLabel:             "智谱 Coding Plan",
		protocol:                  "claude",
		description:               "适合智谱 Coding Plan 的 Claude 兼容入口；该入口按 Metapi 经验保留为手动候选，不按 URL 强制自动识别。",
		docsURL:                   "https://docs.bigmodel.cn/cn/coding-plan/faq",
		recommendedSkipModelFetch: true,
		recommendedModels:         []string{"glm-4.7", "glm-4.6", "glm-4.5", "glm-4.5-air"},
		hosts:                     []string{"open.bigmodel.cn"},
		paths:                     []string{"/api/anthropic"},
	},
}

func buildSupplierIntegrationHint(in *adminplusdomain.Supplier) *adminplusdomain.SupplierIntegrationHint {
	if in == nil {
		return nil
	}
	preferredProtocol := supplierIntegrationPreferredProtocol(in.Type)
	for _, raw := range []string{in.APIBaseURL, in.DashboardURL} {
		hint := matchSupplierIntegrationHint(raw, preferredProtocol)
		if hint != nil {
			return hint
		}
	}
	return nil
}

func supplierIntegrationPreferredProtocol(supplierType adminplusdomain.SupplierType) string {
	switch supplierType {
	case adminplusdomain.SupplierTypeOpenAI:
		return "openai"
	case adminplusdomain.SupplierTypeAnthropic:
		return "claude"
	case adminplusdomain.SupplierTypeGemini:
		return "gemini"
	default:
		return ""
	}
}

func matchSupplierIntegrationHint(raw string, preferredProtocol string) *adminplusdomain.SupplierIntegrationHint {
	u, ok := parseSupplierIntegrationURL(raw)
	if !ok {
		return nil
	}
	host := strings.ToLower(u.Hostname())
	path := normalizeURLPath(u.Path)
	for _, preset := range supplierIntegrationPresets {
		if preferredProtocol != "" && preset.protocol != preferredProtocol {
			continue
		}
		if !stringInSlice(host, preset.hosts) || !stringInSlice(path, preset.paths) {
			continue
		}
		return supplierIntegrationHintFromPreset(preset, urlWithPath(u, path))
	}
	if preferredProtocol != "" && path == "/" {
		for _, preset := range supplierIntegrationPresets {
			if preset.protocol != preferredProtocol || !stringInSlice(host, preset.hosts) {
				continue
			}
			defaultPath := supplierIntegrationPresetDefaultPath(preset)
			if defaultPath == "" {
				continue
			}
			return supplierIntegrationHintFromPreset(preset, urlWithPath(u, defaultPath))
		}
	}
	return nil
}

func supplierIntegrationPresetDefaultPath(preset supplierIntegrationPreset) string {
	for _, path := range preset.paths {
		normalized := normalizeURLPath(path)
		if normalized != "/" {
			return normalized
		}
	}
	if len(preset.paths) > 0 {
		return normalizeURLPath(preset.paths[0])
	}
	return ""
}

func parseSupplierIntegrationURL(raw string) (*url.URL, bool) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return nil, false
	}
	candidates := []string{v}
	if !strings.Contains(v, "://") {
		candidates = append([]string{"https://" + v}, candidates...)
	}
	for _, candidate := range candidates {
		u, err := url.Parse(strings.TrimSpace(candidate))
		if err == nil && u.Scheme != "" && u.Host != "" {
			return u, true
		}
	}
	return nil, false
}

func supplierIntegrationHintFromPreset(preset supplierIntegrationPreset, sourceURL string) *adminplusdomain.SupplierIntegrationHint {
	return &adminplusdomain.SupplierIntegrationHint{
		ID:                        preset.id,
		Label:                     preset.label,
		ProviderLabel:             preset.providerLabel,
		Protocol:                  preset.protocol,
		Description:               preset.description,
		DocsURL:                   preset.docsURL,
		RecommendedSkipModelFetch: preset.recommendedSkipModelFetch,
		RecommendedModels:         append([]string(nil), preset.recommendedModels...),
		SourceURL:                 sourceURL,
	}
}

func cloneSupplierIntegrationHint(in *adminplusdomain.SupplierIntegrationHint) *adminplusdomain.SupplierIntegrationHint {
	if in == nil {
		return nil
	}
	out := *in
	out.RecommendedModels = append([]string(nil), in.RecommendedModels...)
	return &out
}
