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

func TestServiceCreateSupplierPreservesBrowserLoginSecrets(t *testing.T) {
	svc := NewService(NewMemoryRepository())

	supplier, err := svc.Create(context.Background(), CreateSupplierInput{
		Name:                 "Relay",
		Kind:                 adminplusdomain.SupplierKindRelay,
		Type:                 adminplusdomain.SupplierTypeSub2API,
		DashboardURL:         "https://relay.example.com",
		BrowserLoginEnabled:  true,
		BrowserLoginUsername: " ops@example.com ",
		BrowserLoginPassword: " secret-with-spaces ",
		BrowserLoginToken:    " token-with-spaces ",
	})

	require.NoError(t, err)
	require.Equal(t, "ops@example.com", supplier.BrowserLoginUsername)
	require.Equal(t, " secret-with-spaces ", supplier.BrowserLoginPassword)
	require.Equal(t, " token-with-spaces ", supplier.BrowserLoginToken)
	require.True(t, supplier.Credential.BrowserLoginPasswordConfigured)
	require.True(t, supplier.Credential.BrowserLoginTokenConfigured)

	credential, err := svc.GetBrowserCredential(context.Background(), supplier.ID)
	require.NoError(t, err)
	require.Equal(t, " secret-with-spaces ", credential.Password)
	require.Equal(t, " token-with-spaces ", credential.Token)
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

func TestServiceCreateSupplierAccountBindsLocalAccount(t *testing.T) {
	svc := NewService(NewMemoryRepository())
	supplier, err := svc.Create(context.Background(), CreateSupplierInput{
		Name:          "Relay",
		Kind:          adminplusdomain.SupplierKindRelay,
		Type:          adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus: adminplusdomain.SupplierRuntimeStatusCandidate,
		BalanceCents:  1000,
	})
	require.NoError(t, err)

	account, err := svc.CreateAccount(context.Background(), CreateSupplierAccountInput{
		SupplierID:                supplier.ID,
		LocalSub2APIAccountID:     1,
		SupplierAccountIdentifier: "supplier-user",
		SupplierAccountLabel:      "primary",
		BalanceCents:              1000,
		RuntimeStatus:             adminplusdomain.SupplierRuntimeStatusCandidate,
	})

	require.NoError(t, err)
	require.Equal(t, supplier.ID, account.SupplierID)
	require.Equal(t, int64(1), account.LocalSub2APIAccountID)
	require.Equal(t, "Local OpenAI", account.LocalAccountName)
	require.True(t, account.HasUsableBalance)
}

func TestServiceRejectsSwitchableAccountWhenParentIsMonitorOnly(t *testing.T) {
	svc := NewService(NewMemoryRepository())
	supplier, err := svc.Create(context.Background(), CreateSupplierInput{
		Name: "Relay",
		Kind: adminplusdomain.SupplierKindRelay,
		Type: adminplusdomain.SupplierTypeSub2API,
	})
	require.NoError(t, err)

	_, err = svc.CreateAccount(context.Background(), CreateSupplierAccountInput{
		SupplierID:            supplier.ID,
		LocalSub2APIAccountID: 1,
		BalanceCents:          1000,
		RuntimeStatus:         adminplusdomain.SupplierRuntimeStatusCandidate,
	})

	require.Error(t, err)
	require.Equal(t, "SUPPLIER_PARENT_NOT_SWITCHABLE", infraerrors.Reason(err))
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

func TestServiceUpdateStatusRejectsNoBalanceCandidate(t *testing.T) {
	svc := NewService(NewMemoryRepository())

	supplier, err := svc.Create(context.Background(), CreateSupplierInput{
		Name: "No Balance Supplier",
		Kind: adminplusdomain.SupplierKindRelay,
		Type: adminplusdomain.SupplierTypeSub2API,
	})
	require.NoError(t, err)

	_, err = svc.UpdateStatus(context.Background(), supplier.ID, UpdateSupplierStatusInput{
		RuntimeStatus: adminplusdomain.SupplierRuntimeStatusCandidate,
		HealthStatus:  adminplusdomain.SupplierHealthStatusNormal,
	})

	require.Error(t, err)
	require.Equal(t, "SUPPLIER_BALANCE_REQUIRED_FOR_CANDIDATE", infraerrors.Reason(err))
}

func TestServiceListFiltersSuppliers(t *testing.T) {
	svc := NewService(NewMemoryRepository())
	_, err := svc.Create(context.Background(), CreateSupplierInput{
		Name:          "Active Relay",
		Kind:          adminplusdomain.SupplierKindRelay,
		Type:          adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus: adminplusdomain.SupplierRuntimeStatusActive,
		BalanceCents:  1000,
	})
	require.NoError(t, err)
	_, err = svc.Create(context.Background(), CreateSupplierInput{
		Name: "Source Account",
		Kind: adminplusdomain.SupplierKindSourceAccount,
		Type: adminplusdomain.SupplierTypeOpenAI,
	})
	require.NoError(t, err)

	items, err := svc.List(context.Background(), SupplierFilter{
		RuntimeStatus: adminplusdomain.SupplierRuntimeStatusActive,
	})

	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "Active Relay", items[0].Name)
}

func TestServiceListConvertsLegacyNewAPIQuotaBalance(t *testing.T) {
	repo := NewMemoryRepository()
	_, err := repo.Create(context.Background(), &adminplusdomain.Supplier{
		Name:            "Codex APIs",
		Kind:            adminplusdomain.SupplierKindRelay,
		Type:            adminplusdomain.SupplierTypeNewAPI,
		RuntimeStatus:   adminplusdomain.SupplierRuntimeStatusCandidate,
		HealthStatus:    adminplusdomain.SupplierHealthStatusNormal,
		BalanceCents:    892305600,
		BalanceCurrency: "QTA",
	})
	require.NoError(t, err)
	svc := NewService(repo)

	items, err := svc.List(context.Background(), SupplierFilter{})

	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, int64(1785), items[0].BalanceCents)
	require.Equal(t, "USD", items[0].BalanceCurrency)
}

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

func TestServiceUpdateSupplierKeepsBrowserCredentialWhenSecretsOmitted(t *testing.T) {
	svc := NewService(NewMemoryRepository())
	supplier, err := svc.Create(context.Background(), CreateSupplierInput{
		Name:                 "Relay",
		Kind:                 adminplusdomain.SupplierKindRelay,
		Type:                 adminplusdomain.SupplierTypeSub2API,
		DashboardURL:         "https://relay.example.com",
		BrowserLoginEnabled:  true,
		BrowserLoginUsername: "ops@example.com",
		BrowserLoginPassword: "secret",
		BrowserLoginToken:    "token",
	})
	require.NoError(t, err)

	updated, err := svc.Update(context.Background(), supplier.ID, UpdateSupplierInput{
		Name:                "Relay Updated",
		Kind:                adminplusdomain.SupplierKindRelay,
		Type:                adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus:       adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:        adminplusdomain.SupplierHealthStatusNormal,
		DashboardURL:        "https://relay.example.com",
		BrowserLoginEnabled: true,
	})
	require.NoError(t, err)
	require.Equal(t, "Relay Updated", updated.Name)
	require.True(t, updated.Credential.BrowserLoginUsernameConfigured)
	require.True(t, updated.Credential.BrowserLoginPasswordConfigured)
	require.True(t, updated.Credential.BrowserLoginTokenConfigured)
	require.Equal(t, "op***@example.com", updated.Credential.MaskedBrowserLoginUsername)

	credential, err := svc.GetBrowserCredential(context.Background(), supplier.ID)
	require.NoError(t, err)
	require.Equal(t, "ops@example.com", credential.Username)
	require.Equal(t, "secret", credential.Password)
	require.Equal(t, "token", credential.Token)
}

func TestServiceUpdateSupplierPreservesBrowserLoginSecrets(t *testing.T) {
	svc := NewService(NewMemoryRepository())
	supplier, err := svc.Create(context.Background(), CreateSupplierInput{
		Name:                 "Relay",
		Kind:                 adminplusdomain.SupplierKindRelay,
		Type:                 adminplusdomain.SupplierTypeSub2API,
		DashboardURL:         "https://relay.example.com",
		BrowserLoginEnabled:  true,
		BrowserLoginUsername: "ops@example.com",
		BrowserLoginPassword: "secret",
		BrowserLoginToken:    "token",
	})
	require.NoError(t, err)

	updated, err := svc.Update(context.Background(), supplier.ID, UpdateSupplierInput{
		Name:                 "Relay Updated",
		Kind:                 adminplusdomain.SupplierKindRelay,
		Type:                 adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus:        adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:         adminplusdomain.SupplierHealthStatusNormal,
		DashboardURL:         "https://relay.example.com",
		BrowserLoginEnabled:  true,
		BrowserLoginUsername: " ops2@example.com ",
		BrowserLoginPassword: " changed-secret ",
		BrowserLoginToken:    " changed-token ",
	})

	require.NoError(t, err)
	require.Equal(t, "ops2@example.com", updated.BrowserLoginUsername)
	require.Equal(t, " changed-secret ", updated.BrowserLoginPassword)
	require.Equal(t, " changed-token ", updated.BrowserLoginToken)

	credential, err := svc.GetBrowserCredential(context.Background(), supplier.ID)
	require.NoError(t, err)
	require.Equal(t, "ops2@example.com", credential.Username)
	require.Equal(t, " changed-secret ", credential.Password)
	require.Equal(t, " changed-token ", credential.Token)
}

func TestServiceDeleteSupplierCascadesAccountsInRepository(t *testing.T) {
	svc := NewService(NewMemoryRepository())
	supplier, err := svc.Create(context.Background(), CreateSupplierInput{
		Name: "Relay",
		Kind: adminplusdomain.SupplierKindRelay,
		Type: adminplusdomain.SupplierTypeSub2API,
	})
	require.NoError(t, err)
	_, err = svc.CreateAccount(context.Background(), CreateSupplierAccountInput{
		SupplierID:            supplier.ID,
		LocalSub2APIAccountID: 1,
		RuntimeStatus:         adminplusdomain.SupplierRuntimeStatusMonitorOnly,
	})
	require.NoError(t, err)

	require.NoError(t, svc.Delete(context.Background(), supplier.ID))
	_, err = svc.Get(context.Background(), supplier.ID)
	require.Error(t, err)
	require.Equal(t, "SUPPLIER_NOT_FOUND", infraerrors.Reason(err))
}

func TestServiceUpdateSupplierAccount(t *testing.T) {
	svc := NewService(NewMemoryRepository())
	supplier, err := svc.Create(context.Background(), CreateSupplierInput{
		Name:          "Relay",
		Kind:          adminplusdomain.SupplierKindRelay,
		Type:          adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus: adminplusdomain.SupplierRuntimeStatusCandidate,
		BalanceCents:  1000,
	})
	require.NoError(t, err)
	account, err := svc.CreateAccount(context.Background(), CreateSupplierAccountInput{
		SupplierID:            supplier.ID,
		LocalSub2APIAccountID: 1,
		RuntimeStatus:         adminplusdomain.SupplierRuntimeStatusMonitorOnly,
	})
	require.NoError(t, err)

	updated, err := svc.UpdateAccount(context.Background(), UpdateSupplierAccountInput{
		SupplierID:                supplier.ID,
		AccountID:                 account.ID,
		SupplierAccountIdentifier: "supplier-key-1",
		SupplierAccountLabel:      "primary",
		RateProfile:               "discount-a",
		ConfiguredConcurrency:     8,
		ObservedMaxConcurrency:    6,
		BalanceThresholdCents:     500,
		BalanceCents:              3000,
		BalanceCurrency:           "CNY",
		RuntimeStatus:             adminplusdomain.SupplierRuntimeStatusCandidate,
		HealthStatus:              adminplusdomain.SupplierHealthStatusNormal,
	})

	require.NoError(t, err)
	require.Equal(t, account.ID, updated.ID)
	require.Equal(t, int64(1), updated.LocalSub2APIAccountID)
	require.Equal(t, "Local OpenAI", updated.LocalAccountName)
	require.Equal(t, "supplier-key-1", updated.SupplierAccountIdentifier)
	require.Equal(t, "discount-a", updated.RateProfile)
	require.Equal(t, 8, updated.ConfiguredConcurrency)
	require.Equal(t, 6, updated.ObservedMaxConcurrency)
	require.True(t, updated.HasUsableBalance)
	require.Equal(t, adminplusdomain.SupplierRuntimeStatusCandidate, updated.RuntimeStatus)
}
