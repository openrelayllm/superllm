package suppliers

import (
	"strings"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
)

func buildSupplierOperationHints(in *adminplusdomain.Supplier) []adminplusdomain.SupplierOperationHint {
	if in == nil {
		return nil
	}
	hints := make([]adminplusdomain.SupplierOperationHint, 0, 3)
	if len(in.APIEndpointCandidates) > 1 {
		hints = append(hints, adminplusdomain.SupplierOperationHint{
			Key:         "api_endpoint_candidates",
			Label:       "多入口",
			Severity:    "info",
			Source:      "metapi_site_api_endpoints",
			Description: "存在多个兼容 API Base 候选；这里只做运营提示，实际调度仍由本地 Sub2API 配置决定。",
		})
	}
	if hint := supplierRecommendedAPIBaseOperationHint(in); hint != nil {
		hints = append(hints, *hint)
	}
	if supplierHasManualIntegrationCandidate(in) {
		hints = append(hints, adminplusdomain.SupplierOperationHint{
			Key:         "manual_integration_candidate",
			Label:       "手动入口",
			Severity:    "info",
			Source:      "metapi_manual_preset",
			Description: "存在 Metapi 标记为手动预设的兼容入口；仅作为运营候选提示，不自动覆盖当前供应商协议识别。",
		})
	}
	if in.Type == adminplusdomain.SupplierTypeSub2API || in.Type == adminplusdomain.SupplierTypeNewAPI {
		hints = append(hints, adminplusdomain.SupplierOperationHint{
			Key:         "post_sync_probe",
			Label:       "同步后检测",
			Severity:    "action",
			Source:      "metapi_post_refresh_probe",
			Description: "同步分组、Key 或成本后建议立即执行渠道检测，确认首 token、总耗时和本地账号绑定状态。",
		})
	}
	if in.IntegrationHint != nil && len(in.IntegrationHint.RecommendedModels) > 0 {
		models := strings.Join(in.IntegrationHint.RecommendedModels[:minInt(len(in.IntegrationHint.RecommendedModels), 3)], " / ")
		hints = append(hints, adminplusdomain.SupplierOperationHint{
			Key:         "recommended_model_review",
			Label:       "模型建议",
			Severity:    "info",
			Source:      "metapi_site_disabled_models",
			Description: "创建本地账号时优先补入推荐模型 " + models + "；不可用模型应在本地账号白名单或映射层收敛。",
		})
	}
	if len(hints) > 3 {
		return hints[:3]
	}
	return hints
}

func supplierRecommendedAPIBaseOperationHint(in *adminplusdomain.Supplier) *adminplusdomain.SupplierOperationHint {
	if in == nil || in.IntegrationHint == nil || strings.TrimSpace(in.IntegrationHint.SourceURL) == "" {
		return nil
	}
	recommendedURL := normalizeAPIEndpointCandidateURL(in.IntegrationHint.SourceURL)
	if recommendedURL == "" {
		return nil
	}
	for _, raw := range []string{in.APIBaseURL, in.DashboardURL} {
		if normalizeAPIEndpointCandidateURL(raw) == recommendedURL {
			return nil
		}
	}
	return &adminplusdomain.SupplierOperationHint{
		Key:         "recommended_api_base",
		Label:       "推荐入口",
		Severity:    "action",
		Source:      "metapi_site_initialization_preset",
		Description: "当前 URL 已匹配供应商初始化预设，建议本地账号 API Base 使用 " + recommendedURL + "。",
	}
}

func supplierHasManualIntegrationCandidate(in *adminplusdomain.Supplier) bool {
	if in == nil {
		return false
	}
	for _, candidate := range in.APIEndpointCandidates {
		if candidate.Source == "manual_integration_preset" {
			return true
		}
	}
	return false
}
