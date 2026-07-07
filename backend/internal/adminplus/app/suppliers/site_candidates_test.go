package suppliers

import (
	"context"
	"testing"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestServiceMatchSitePrefersExactHostPort(t *testing.T) {
	svc := NewService(NewMemoryRepository())
	first, err := svc.Create(context.Background(), CreateSupplierInput{
		Name:         "Local Relay A",
		Kind:         adminplusdomain.SupplierKindRelay,
		Type:         adminplusdomain.SupplierTypeSub2API,
		DashboardURL: "http://127.0.0.1:51001",
		APIBaseURL:   "http://127.0.0.1:51001",
	})
	require.NoError(t, err)
	second, err := svc.Create(context.Background(), CreateSupplierInput{
		Name:         "Local Relay B",
		Kind:         adminplusdomain.SupplierKindRelay,
		Type:         adminplusdomain.SupplierTypeSub2API,
		DashboardURL: "http://127.0.0.1:51002",
		APIBaseURL:   "http://127.0.0.1:51002",
	})
	require.NoError(t, err)

	matched, err := svc.MatchSite(context.Background(), SiteMatchInput{
		URL: "http://127.0.0.1:51002/dashboard",
	})

	require.NoError(t, err)
	require.Equal(t, "matched", matched.Status)
	require.Len(t, matched.Suppliers, 1)
	require.Equal(t, second.ID, matched.Suppliers[0].ID)
	require.NotEqual(t, first.ID, matched.Suppliers[0].ID)
}

func TestServiceEnsureFromSiteCandidateCreatesSub2APISupplier(t *testing.T) {
	svc := NewService(NewMemoryRepository())

	result, err := svc.EnsureFromSiteCandidateWithOptions(context.Background(), CreateFromSiteCandidateInput{
		Name:         "AI Pixel",
		DashboardURL: "https://ai-pixel.online/dashboard",
		APIBaseURL:   "https://ai-pixel.online",
		SourceHost:   "ai-pixel.online",
		SourceURL:    "https://ai-pixel.online/dashboard",
	}, EnsureFromSiteCandidateOptions{AllowCreate: true})

	require.NoError(t, err)
	require.True(t, result.Created)
	require.False(t, result.Matched)
	require.Equal(t, adminplusdomain.SupplierKindRelay, result.Supplier.Kind)
	require.Equal(t, adminplusdomain.SupplierTypeSub2API, result.Supplier.Type)
	require.Equal(t, adminplusdomain.SupplierRuntimeStatusMonitorOnly, result.Supplier.RuntimeStatus)
	require.Equal(t, "https://ai-pixel.online", result.Supplier.APIBaseURL)

	matched, err := svc.EnsureFromSiteCandidate(context.Background(), CreateFromSiteCandidateInput{
		SourceURL: "https://ai-pixel.online/settings",
	})
	require.NoError(t, err)
	require.True(t, matched.Matched)
	require.False(t, matched.Created)
	require.Equal(t, result.Supplier.ID, matched.Supplier.ID)
}

func TestServiceEnsureFromSiteCandidateInfersAPIBaseFromKnownEndpoint(t *testing.T) {
	svc := NewService(NewMemoryRepository())

	result, err := svc.EnsureFromSiteCandidateWithOptions(context.Background(), CreateFromSiteCandidateInput{
		Name:       "Relay",
		SourceHost: "relay.example.com",
		SourceURL:  "https://relay.example.com/api/v1/user/self?tab=profile",
	}, EnsureFromSiteCandidateOptions{AllowCreate: true})

	require.NoError(t, err)
	require.True(t, result.Created)
	require.Equal(t, "https://relay.example.com", result.Supplier.DashboardURL)
	require.Equal(t, "https://relay.example.com/api/v1", result.Supplier.APIBaseURL)
}

func TestServiceEnsureFromSiteCandidateInfersOpenAICompatibleAPIBase(t *testing.T) {
	svc := NewService(NewMemoryRepository())

	result, err := svc.EnsureFromSiteCandidateWithOptions(context.Background(), CreateFromSiteCandidateInput{
		Name:       "OpenAI Compatible",
		SourceHost: "relay.example.com",
		SourceURL:  "https://relay.example.com/v1/chat/completions",
	}, EnsureFromSiteCandidateOptions{AllowCreate: true})

	require.NoError(t, err)
	require.True(t, result.Created)
	require.Equal(t, "https://relay.example.com", result.Supplier.DashboardURL)
	require.Equal(t, "https://relay.example.com/v1", result.Supplier.APIBaseURL)
}

func TestServiceEnsureFromSiteCandidateInfersOfficialSourceAccountFromURLHint(t *testing.T) {
	svc := NewService(NewMemoryRepository())

	result, err := svc.EnsureFromSiteCandidateWithOptions(context.Background(), CreateFromSiteCandidateInput{
		Name:       "OpenAI Official",
		SourceHost: "api.openai.com",
		SourceURL:  "https://api.openai.com/v1/models?from=extension",
	}, EnsureFromSiteCandidateOptions{AllowCreate: true})

	require.NoError(t, err)
	require.True(t, result.Created)
	require.Equal(t, adminplusdomain.SupplierKindSourceAccount, result.Supplier.Kind)
	require.Equal(t, adminplusdomain.SupplierTypeOpenAI, result.Supplier.Type)
	require.Equal(t, "https://api.openai.com", result.Supplier.DashboardURL)
	require.Equal(t, "https://api.openai.com/v1", result.Supplier.APIBaseURL)
	require.NotNil(t, result.Supplier.IntegrationHint)
	require.Equal(t, "openai-official", result.Supplier.IntegrationHint.ID)
}

func TestServiceEnsureFromSiteCandidateKeepsExplicitSupplierTypeOverURLHint(t *testing.T) {
	svc := NewService(NewMemoryRepository())

	result, err := svc.EnsureFromSiteCandidateWithOptions(context.Background(), CreateFromSiteCandidateInput{
		Name:       "Explicit Custom",
		Type:       adminplusdomain.SupplierTypeCustom,
		SourceHost: "api.openai.com",
		SourceURL:  "https://api.openai.com/v1/models",
	}, EnsureFromSiteCandidateOptions{AllowCreate: true})

	require.NoError(t, err)
	require.True(t, result.Created)
	require.Equal(t, adminplusdomain.SupplierKindRelay, result.Supplier.Kind)
	require.Equal(t, adminplusdomain.SupplierTypeCustom, result.Supplier.Type)
	require.Equal(t, "https://api.openai.com/v1", result.Supplier.APIBaseURL)
}

func TestServiceEnsureFromSiteCandidateNormalizesExplicitSupplierTypeAlias(t *testing.T) {
	svc := NewService(NewMemoryRepository())

	result, err := svc.EnsureFromSiteCandidateWithOptions(context.Background(), CreateFromSiteCandidateInput{
		Name:       "Claude Compatible",
		Type:       adminplusdomain.SupplierType("claude"),
		SourceHost: "relay.example.com",
		SourceURL:  "https://relay.example.com/v1/messages",
	}, EnsureFromSiteCandidateOptions{AllowCreate: true})

	require.NoError(t, err)
	require.True(t, result.Created)
	require.Equal(t, adminplusdomain.SupplierKindSourceAccount, result.Supplier.Kind)
	require.Equal(t, adminplusdomain.SupplierTypeAnthropic, result.Supplier.Type)
}

func TestServiceEnsureFromSiteCandidateKeepsSemanticAPIBasePath(t *testing.T) {
	svc := NewService(NewMemoryRepository())

	result, err := svc.EnsureFromSiteCandidateWithOptions(context.Background(), CreateFromSiteCandidateInput{
		Name:       "Coding Plan",
		Type:       adminplusdomain.SupplierTypeCustom,
		SourceHost: "open.bigmodel.cn",
		SourceURL:  "https://open.bigmodel.cn/api/coding/paas/v4?from=docs",
	}, EnsureFromSiteCandidateOptions{AllowCreate: true})

	require.NoError(t, err)
	require.True(t, result.Created)
	require.Equal(t, "https://open.bigmodel.cn", result.Supplier.DashboardURL)
	require.Equal(t, "https://open.bigmodel.cn/api/coding/paas/v4", result.Supplier.APIBaseURL)
}

func TestServiceEnsureFromSiteCandidateKeepsDoubaoCodingAPIBasePath(t *testing.T) {
	svc := NewService(NewMemoryRepository())

	result, err := svc.EnsureFromSiteCandidateWithOptions(context.Background(), CreateFromSiteCandidateInput{
		Name:       "Doubao Coding",
		Type:       adminplusdomain.SupplierTypeCustom,
		SourceHost: "ark.cn-beijing.volces.com",
		SourceURL:  "https://ark.cn-beijing.volces.com/api/coding/v3?from=docs",
	}, EnsureFromSiteCandidateOptions{AllowCreate: true})

	require.NoError(t, err)
	require.True(t, result.Created)
	require.Equal(t, "https://ark.cn-beijing.volces.com", result.Supplier.DashboardURL)
	require.Equal(t, "https://ark.cn-beijing.volces.com/api/coding/v3", result.Supplier.APIBaseURL)
	require.NotNil(t, result.Supplier.IntegrationHint)
	require.Equal(t, "doubao-coding-openai", result.Supplier.IntegrationHint.ID)
	require.Contains(t, result.Supplier.IntegrationHint.RecommendedModels, "ark-code-latest")
}

func TestServiceEnsureFromSiteCandidateRequiresRegistrationBeforeCreate(t *testing.T) {
	svc := NewService(NewMemoryRepository())

	result, err := svc.EnsureFromSiteCandidate(context.Background(), CreateFromSiteCandidateInput{
		Name:         "AI Pixel",
		DashboardURL: "https://ai-pixel.online/dashboard",
		APIBaseURL:   "https://ai-pixel.online",
		SourceHost:   "ai-pixel.online",
		SourceURL:    "https://ai-pixel.online/dashboard",
	})

	require.Nil(t, result)
	require.Error(t, err)
	require.Equal(t, "SUPPLIER_SITE_REGISTRATION_REQUIRED", infraerrors.Reason(err))
}

func TestServiceEnsureFromSiteCandidateCreatesNewAPISupplier(t *testing.T) {
	svc := NewService(NewMemoryRepository())

	result, err := svc.EnsureFromSiteCandidateWithOptions(context.Background(), CreateFromSiteCandidateInput{
		Name:         "Codex APIs",
		Type:         adminplusdomain.SupplierTypeNewAPI,
		DashboardURL: "https://www.codexapis.com/console",
		APIBaseURL:   "https://www.codexapis.com",
		SourceHost:   "www.codexapis.com",
		SourceURL:    "https://www.codexapis.com/console",
	}, EnsureFromSiteCandidateOptions{AllowCreate: true})

	require.NoError(t, err)
	require.True(t, result.Created)
	require.False(t, result.Matched)
	require.Equal(t, adminplusdomain.SupplierKindRelay, result.Supplier.Kind)
	require.Equal(t, adminplusdomain.SupplierTypeNewAPI, result.Supplier.Type)
	require.Equal(t, adminplusdomain.SupplierRuntimeStatusMonitorOnly, result.Supplier.RuntimeStatus)
	require.Equal(t, "https://www.codexapis.com", result.Supplier.APIBaseURL)
	require.Equal(t, "USD", result.Supplier.BalanceCurrency)
}

func TestServiceEnsureFromSiteCandidateInfersNewAPIAliasFromURLHint(t *testing.T) {
	svc := NewService(NewMemoryRepository())

	result, err := svc.EnsureFromSiteCandidateWithOptions(context.Background(), CreateFromSiteCandidateInput{
		Name:       "VO API",
		SourceHost: "vo-api.example.com",
		SourceURL:  "https://vo-api.example.com/console",
	}, EnsureFromSiteCandidateOptions{AllowCreate: true})

	require.NoError(t, err)
	require.True(t, result.Created)
	require.Equal(t, adminplusdomain.SupplierKindRelay, result.Supplier.Kind)
	require.Equal(t, adminplusdomain.SupplierTypeNewAPI, result.Supplier.Type)
	require.NotNil(t, result.Supplier.PlatformHint)
	require.Equal(t, "new-api", result.Supplier.PlatformHint.ID)
}

func TestServiceEnsureFromSiteCandidateInfersGeminiCLIFromURLHint(t *testing.T) {
	svc := NewService(NewMemoryRepository())

	result, err := svc.EnsureFromSiteCandidateWithOptions(context.Background(), CreateFromSiteCandidateInput{
		Name:       "Gemini CLI",
		SourceHost: "cloudcode-pa.googleapis.com",
		SourceURL:  "https://cloudcode-pa.googleapis.com/v1internal",
	}, EnsureFromSiteCandidateOptions{AllowCreate: true})

	require.NoError(t, err)
	require.True(t, result.Created)
	require.Equal(t, adminplusdomain.SupplierKindSourceAccount, result.Supplier.Kind)
	require.Equal(t, adminplusdomain.SupplierTypeGemini, result.Supplier.Type)
	require.NotNil(t, result.Supplier.PlatformHint)
	require.Equal(t, "gemini-cli", result.Supplier.PlatformHint.ID)
}

func TestServiceEnsureFromSiteCandidateInfersThirdPartyRechargeURL(t *testing.T) {
	svc := NewService(NewMemoryRepository())

	result, err := svc.EnsureFromSiteCandidateWithOptions(context.Background(), CreateFromSiteCandidateInput{
		Name:       "AI Pixel",
		SourceHost: "ai-pixel.online",
		SourceURL:  "https://ai-pixel.online/custom/9acb40a98e688a94",
	}, EnsureFromSiteCandidateOptions{AllowCreate: true})

	require.NoError(t, err)
	require.True(t, result.Created)
	require.Equal(t, "https://ai-pixel.online/custom/9acb40a98e688a94", result.Supplier.ThirdPartyRechargeURL)
	require.Equal(t, "https://ai-pixel.online", result.Supplier.DashboardURL)
}

func TestServiceEnrichRechargeURLsOnlyFillsEmptyFields(t *testing.T) {
	svc := NewService(NewMemoryRepository())
	supplier, err := svc.Create(context.Background(), CreateSupplierInput{
		Name:         "Relay",
		Kind:         adminplusdomain.SupplierKindRelay,
		Type:         adminplusdomain.SupplierTypeSub2API,
		DashboardURL: "https://relay.example.com",
	})
	require.NoError(t, err)

	enriched, err := svc.EnrichRechargeURLs(context.Background(), supplier.ID, CreateFromSiteCandidateInput{
		SourceURL:        "https://relay.example.com/custom/first",
		LocalRechargeURL: "https://sub2apiplus.example.com/custom/first",
	})
	require.NoError(t, err)
	require.Equal(t, "https://relay.example.com/custom/first", enriched.ThirdPartyRechargeURL)
	require.Equal(t, "https://sub2apiplus.example.com/custom/first", enriched.LocalRechargeURL)

	unchanged, err := svc.EnrichRechargeURLs(context.Background(), supplier.ID, CreateFromSiteCandidateInput{
		ThirdPartyRechargeURL: "https://relay.example.com/custom/replaced",
		LocalRechargeURL:      "https://sub2apiplus.example.com/custom/replaced",
	})
	require.NoError(t, err)
	require.Equal(t, "https://relay.example.com/custom/first", unchanged.ThirdPartyRechargeURL)
	require.Equal(t, "https://sub2apiplus.example.com/custom/first", unchanged.LocalRechargeURL)
}
