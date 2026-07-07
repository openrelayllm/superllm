package suppliers

import (
	"context"
	"net/http"
	"testing"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestServiceCreateSupplierDefaultsToMonitorOnly(t *testing.T) {
	svc := NewService(NewMemoryRepository())

	supplier, err := svc.Create(context.Background(), CreateSupplierInput{
		Name:                 "Local OpenAI Pool",
		Kind:                 adminplusdomain.SupplierKindSourceAccount,
		Type:                 adminplusdomain.SupplierTypeOpenAI,
		BrowserLoginEnabled:  true,
		BrowserLoginUsername: "ops@example.com",
		BrowserLoginPassword: "secret",
	})

	require.NoError(t, err)
	require.Equal(t, int64(1), supplier.ID)
	require.Equal(t, adminplusdomain.SupplierRuntimeStatusMonitorOnly, supplier.RuntimeStatus)
	require.Equal(t, adminplusdomain.SupplierHealthStatusNormal, supplier.HealthStatus)
	require.True(t, supplier.Credential.BrowserLoginEnabled)
	require.True(t, supplier.Credential.BrowserLoginUsernameConfigured)
	require.True(t, supplier.Credential.BrowserLoginPasswordConfigured)
	require.Equal(t, "op***@example.com", supplier.Credential.MaskedBrowserLoginUsername)
	require.Equal(t, "USD", supplier.BalanceCurrency)
}

func TestServiceCreateSupplierDefaultsToCapabilities(t *testing.T) {
	svc := NewService(NewMemoryRepository())

	supplier, err := svc.Create(context.Background(), CreateSupplierInput{
		Name:            "Relay",
		Kind:            adminplusdomain.SupplierKindRelay,
		Type:            adminplusdomain.SupplierTypeSub2API,
		PostgresReadDSN: "postgres://readonly",
	})

	require.NoError(t, err)
	require.Equal(t, adminplusdomain.SupplierCapabilityStatusNeedsSession, supplierCapabilityStatus(t, supplier, "rates"))
	require.Equal(t, adminplusdomain.SupplierCapabilityStatusNeedsSession, supplierCapabilityStatus(t, supplier, "keys"))
	require.Equal(t, adminplusdomain.SupplierCapabilityStatusAvailable, supplierCapabilityStatus(t, supplier, "local_runtime_observation"))
}

func TestServiceNewAPICapabilitiesShowPlannedRateAnnouncement(t *testing.T) {
	svc := NewService(NewMemoryRepository())

	supplier, err := svc.Create(context.Background(), CreateSupplierInput{
		Name: "New API Relay",
		Kind: adminplusdomain.SupplierKindRelay,
		Type: adminplusdomain.SupplierTypeNewAPI,
	})
	require.NoError(t, err)

	items, err := svc.List(context.Background(), SupplierFilter{})

	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, supplier.ID, items[0].ID)
	require.Equal(t, adminplusdomain.SupplierCapabilityStatusPlanned, supplierCapabilityStatus(t, items[0], "rates"))
	require.Equal(t, adminplusdomain.SupplierCapabilityStatusPlanned, supplierCapabilityStatus(t, items[0], "announcements"))
	require.Equal(t, adminplusdomain.SupplierCapabilityStatusNeedsSession, supplierCapabilityStatus(t, items[0], "funding"))
	require.Equal(t, adminplusdomain.SupplierCapabilityStatusNeedsReadonlyDB, supplierCapabilityStatus(t, items[0], "local_runtime_observation"))
}

func TestServiceCreateSupplierBuildsIntegrationHint(t *testing.T) {
	svc := NewService(NewMemoryRepository())

	supplier, err := svc.Create(context.Background(), CreateSupplierInput{
		Name:       "DeepSeek Source",
		Kind:       adminplusdomain.SupplierKindSourceAccount,
		Type:       adminplusdomain.SupplierTypeOpenAI,
		APIBaseURL: "https://api.deepseek.com/v1",
	})

	require.NoError(t, err)
	require.NotNil(t, supplier.IntegrationHint)
	require.Equal(t, "deepseek-openai", supplier.IntegrationHint.ID)
	require.Equal(t, "DeepSeek", supplier.IntegrationHint.ProviderLabel)
	require.Equal(t, "openai", supplier.IntegrationHint.Protocol)
	require.True(t, supplier.IntegrationHint.RecommendedSkipModelFetch)
	require.Equal(t, []string{"deepseek-chat", "deepseek-reasoner"}, supplier.IntegrationHint.RecommendedModels)
	require.NotNil(t, supplier.PlatformHint)
	require.Equal(t, "DeepSeek", supplier.PlatformHint.Label)
	require.Equal(t, "api_provider", supplier.PlatformHint.Family)
}

func TestServiceCreateSupplierBuildsAPIEndpointCandidates(t *testing.T) {
	svc := NewService(NewMemoryRepository())

	supplier, err := svc.Create(context.Background(), CreateSupplierInput{
		Name:       "DeepSeek Source",
		Kind:       adminplusdomain.SupplierKindSourceAccount,
		Type:       adminplusdomain.SupplierTypeOpenAI,
		APIBaseURL: "https://api.deepseek.com/v1",
	})

	require.NoError(t, err)
	require.Equal(t, "https://api.deepseek.com/v1", supplierAPIEndpointCandidate(t, supplier, "configured_api_base").URL)
	require.Equal(t, "openai", supplierAPIEndpointCandidate(t, supplier, "configured_api_base").Protocol)
	require.Equal(t, "https://api.deepseek.com/anthropic", supplierAPIEndpointCandidate(t, supplier, "deepseek-claude:/anthropic").URL)
	require.Equal(t, "claude", supplierAPIEndpointCandidate(t, supplier, "deepseek-claude:/anthropic").Protocol)
	require.Equal(t, "多入口", supplierOperationHint(t, supplier, "api_endpoint_candidates").Label)
	require.Equal(t, "模型建议", supplierOperationHint(t, supplier, "recommended_model_review").Label)
}

func TestServiceCreateSupplierDerivesDashboardAPIEndpointCandidate(t *testing.T) {
	svc := NewService(NewMemoryRepository())

	supplier, err := svc.Create(context.Background(), CreateSupplierInput{
		Name:         "Relay",
		Kind:         adminplusdomain.SupplierKindRelay,
		Type:         adminplusdomain.SupplierTypeSub2API,
		DashboardURL: "https://relay.example.com/dashboard",
	})

	require.NoError(t, err)
	require.Equal(t, "https://relay.example.com/api/v1", supplierAPIEndpointCandidate(t, supplier, "dashboard_api_v1").URL)
	require.Equal(t, "同步后检测", supplierOperationHint(t, supplier, "post_sync_probe").Label)
}

func TestServiceCreateSupplierBuildsPlatformHintFromURLAndName(t *testing.T) {
	svc := NewService(NewMemoryRepository())

	donehub, err := svc.Create(context.Background(), CreateSupplierInput{
		Name:         "DoneHub Relay",
		Kind:         adminplusdomain.SupplierKindRelay,
		Type:         adminplusdomain.SupplierTypeNewAPI,
		DashboardURL: "https://donehub.example.com",
	})
	require.NoError(t, err)
	require.NotNil(t, donehub.PlatformHint)
	require.Equal(t, "done-hub", donehub.PlatformHint.ID)
	require.Equal(t, "new_api", donehub.PlatformHint.Family)
	require.Equal(t, "url_hint", donehub.PlatformHint.Source)

	sub2api, err := svc.Create(context.Background(), CreateSupplierInput{
		Name: "Sub2API Mirror",
		Kind: adminplusdomain.SupplierKindRelay,
		Type: adminplusdomain.SupplierTypeSub2API,
	})
	require.NoError(t, err)
	require.NotNil(t, sub2api.PlatformHint)
	require.Equal(t, "sub2api", sub2api.PlatformHint.ID)
	require.Equal(t, "name_hint", sub2api.PlatformHint.Source)
}

func TestServiceCreateSupplierBuildsOfficialPlatformHintFromURL(t *testing.T) {
	svc := NewService(NewMemoryRepository())

	supplier, err := svc.Create(context.Background(), CreateSupplierInput{
		Name:       "Official Gemini",
		Kind:       adminplusdomain.SupplierKindSourceAccount,
		Type:       adminplusdomain.SupplierTypeGemini,
		APIBaseURL: "https://generativelanguage.googleapis.com/v1beta",
	})

	require.NoError(t, err)
	require.NotNil(t, supplier.PlatformHint)
	require.Equal(t, "gemini", supplier.PlatformHint.ID)
	require.Equal(t, "source_account", supplier.PlatformHint.Family)
	require.Equal(t, "url_hint", supplier.PlatformHint.Source)
}

func TestServiceCreateSupplierBuildsGeminiPlatformHintFromGoogleapisOpenAIPath(t *testing.T) {
	svc := NewService(NewMemoryRepository())

	supplier, err := svc.Create(context.Background(), CreateSupplierInput{
		Name:       "Vertex OpenAI Compatible",
		Kind:       adminplusdomain.SupplierKindSourceAccount,
		Type:       adminplusdomain.SupplierTypeGemini,
		APIBaseURL: "https://aiplatform.googleapis.com/v1beta/openai",
	})

	require.NoError(t, err)
	require.NotNil(t, supplier.PlatformHint)
	require.Equal(t, "gemini", supplier.PlatformHint.ID)
	require.Equal(t, "source_account", supplier.PlatformHint.Family)
	require.Equal(t, "url_hint", supplier.PlatformHint.Source)
}

func TestServiceCreateSupplierBuildsURLHints(t *testing.T) {
	svc := NewService(NewMemoryRepository())

	supplier, err := svc.Create(context.Background(), CreateSupplierInput{
		Name:         "Relay",
		Kind:         adminplusdomain.SupplierKindRelay,
		Type:         adminplusdomain.SupplierTypeSub2API,
		DashboardURL: "https://relay.example.com/api/v1/user/self?from=browser",
		APIBaseURL:   "https://relay.example.com/v1/chat/completions?debug=1",
	})

	require.NoError(t, err)
	dashboardHint := supplierURLHintByKey(t, supplier, "dashboard_url")
	require.Equal(t, "api_path_in_dashboard", dashboardHint.Action)
	require.Equal(t, "warning", dashboardHint.Severity)
	require.Equal(t, "/api/v1/user/self", dashboardHint.MatchedPath)
	require.Equal(t, "https://relay.example.com", dashboardHint.SuggestedURL)

	apiHint := supplierURLHintByKey(t, supplier, "api_base_url")
	require.Equal(t, "endpoint_path", apiHint.Action)
	require.Equal(t, "warning", apiHint.Severity)
	require.Equal(t, "/v1/chat/completions", apiHint.MatchedPath)
	require.Equal(t, "https://relay.example.com/v1", apiHint.SuggestedURL)
}

func TestServiceCreateSupplierIncludesManualIntegrationCandidate(t *testing.T) {
	svc := NewService(NewMemoryRepository())

	supplier, err := svc.Create(context.Background(), CreateSupplierInput{
		Name:       "Zhipu Coding",
		Kind:       adminplusdomain.SupplierKindSourceAccount,
		Type:       adminplusdomain.SupplierTypeOpenAI,
		APIBaseURL: "https://open.bigmodel.cn/api/coding/paas/v4",
	})

	require.NoError(t, err)
	require.NotNil(t, supplier.IntegrationHint)
	require.Equal(t, "zhipu-coding-plan-openai", supplier.IntegrationHint.ID)
	manualCandidate := supplierAPIEndpointCandidate(t, supplier, "zhipu-coding-plan-claude:/api/anthropic")
	require.Equal(t, "https://open.bigmodel.cn/api/anthropic", manualCandidate.URL)
	require.Equal(t, "claude", manualCandidate.Protocol)
	require.Equal(t, "manual_integration_preset", manualCandidate.Source)
	require.False(t, manualCandidate.Recommended)
	require.Equal(t, "手动入口", supplierOperationHint(t, supplier, "manual_integration_candidate").Label)
}

func TestServiceCreateSupplierURLHintsPreserveSemanticAPIBase(t *testing.T) {
	svc := NewService(NewMemoryRepository())

	supplier, err := svc.Create(context.Background(), CreateSupplierInput{
		Name:       "Coding Plan",
		Kind:       adminplusdomain.SupplierKindSourceAccount,
		Type:       adminplusdomain.SupplierTypeOpenAI,
		APIBaseURL: "https://open.bigmodel.cn/api/coding/paas/v4?from=docs",
	})

	require.NoError(t, err)
	apiHint := supplierURLHintByKey(t, supplier, "api_base_url")
	require.Equal(t, "semantic_base", apiHint.Action)
	require.Equal(t, "success", apiHint.Severity)
	require.Empty(t, apiHint.SuggestedURL)
}

func TestServiceCreateSupplierDoesNotForceConflictingIntegrationProtocol(t *testing.T) {
	svc := NewService(NewMemoryRepository())

	supplier, err := svc.Create(context.Background(), CreateSupplierInput{
		Name:       "Claude Source",
		Kind:       adminplusdomain.SupplierKindSourceAccount,
		Type:       adminplusdomain.SupplierTypeAnthropic,
		APIBaseURL: "https://api.deepseek.com/v1",
	})

	require.NoError(t, err)
	require.Nil(t, supplier.IntegrationHint)
}

func TestServiceCreateSupplierReselectsPresetFromRootURLWhenProtocolKnown(t *testing.T) {
	svc := NewService(NewMemoryRepository())

	openai, err := svc.Create(context.Background(), CreateSupplierInput{
		Name:       "Aliyun Coding Root",
		Kind:       adminplusdomain.SupplierKindSourceAccount,
		Type:       adminplusdomain.SupplierTypeOpenAI,
		APIBaseURL: "https://coding.dashscope.aliyuncs.com",
	})
	require.NoError(t, err)
	require.NotNil(t, openai.IntegrationHint)
	require.Equal(t, "codingplan-openai", openai.IntegrationHint.ID)
	require.Equal(t, "https://coding.dashscope.aliyuncs.com/v1", openai.IntegrationHint.SourceURL)
	recommendedHint := supplierOperationHint(t, openai, "recommended_api_base")
	require.Equal(t, "推荐入口", recommendedHint.Label)
	require.Contains(t, recommendedHint.Description, "https://coding.dashscope.aliyuncs.com/v1")

	claude, err := svc.Create(context.Background(), CreateSupplierInput{
		Name:       "Aliyun Coding Claude Root",
		Kind:       adminplusdomain.SupplierKindSourceAccount,
		Type:       adminplusdomain.SupplierTypeAnthropic,
		APIBaseURL: "https://coding.dashscope.aliyuncs.com",
	})
	require.NoError(t, err)
	require.NotNil(t, claude.IntegrationHint)
	require.Equal(t, "codingplan-claude", claude.IntegrationHint.ID)
	require.Equal(t, "https://coding.dashscope.aliyuncs.com/apps/anthropic", claude.IntegrationHint.SourceURL)
}

func TestServiceCreateCandidateRequiresPositiveBalance(t *testing.T) {
	svc := NewService(NewMemoryRepository())

	_, err := svc.Create(context.Background(), CreateSupplierInput{
		Name:          "Cheap Relay",
		Kind:          adminplusdomain.SupplierKindRelay,
		Type:          adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus: adminplusdomain.SupplierRuntimeStatusCandidate,
	})

	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
	require.Equal(t, "SUPPLIER_BALANCE_REQUIRED_FOR_CANDIDATE", infraerrors.Reason(err))
}

func TestServiceCreateSupplierPersistsRechargeURLs(t *testing.T) {
	svc := NewService(NewMemoryRepository())

	supplier, err := svc.Create(context.Background(), CreateSupplierInput{
		Name:                  "Relay",
		Kind:                  adminplusdomain.SupplierKindRelay,
		Type:                  adminplusdomain.SupplierTypeSub2API,
		ThirdPartyRechargeURL: " https://relay.example.com/custom/topup ",
		LocalRechargeURL:      " https://sub2apiplus.example.com/custom/topup ",
	})

	require.NoError(t, err)
	require.Equal(t, "https://relay.example.com/custom/topup", supplier.ThirdPartyRechargeURL)
	require.Equal(t, "https://sub2apiplus.example.com/custom/topup", supplier.LocalRechargeURL)
}

func TestServiceAllowsMonitorOnlySupplierWithoutBalance(t *testing.T) {
	svc := NewService(NewMemoryRepository())

	supplier, err := svc.Create(context.Background(), CreateSupplierInput{
		Name:          "Discount Watch",
		Kind:          adminplusdomain.SupplierKindRelay,
		Type:          adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus: adminplusdomain.SupplierRuntimeStatusMonitorOnly,
	})

	require.NoError(t, err)
	require.Equal(t, adminplusdomain.SupplierRuntimeStatusMonitorOnly, supplier.RuntimeStatus)
	require.Zero(t, supplier.BalanceCents)
}
