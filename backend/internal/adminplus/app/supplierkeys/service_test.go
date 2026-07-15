package supplierkeys

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/adminplus/ports"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestServiceProvisionCreatesProviderKeyLocalAccountAndBinding(t *testing.T) {
	repo := NewMemoryRepository()
	repo.PutSupplier(&adminplusdomain.Supplier{
		ID:            7,
		Name:          "Relay",
		Type:          adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus: adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:  adminplusdomain.SupplierHealthStatusNormal,
	})
	repo.PutGroup(&adminplusdomain.SupplierGroup{
		ID:              10,
		SupplierID:      7,
		ExternalGroupID: "88",
		Name:            "Low Cost",
		ProviderFamily:  "openai",
		Status:          adminplusdomain.SupplierGroupStatusActive,
	})
	session := &stubSessionReader{
		input: ports.SessionProbeInput{
			SupplierID: 7,
			APIBaseURL: "https://relay.example.com/api/v1",
			Bundle:     map[string]any{"access_token": "browser-token"},
		},
	}
	keyAdapter := &stubKeyAdapter{
		result: &ports.ProviderKeyResult{
			SupplierID:      7,
			ExternalGroupID: "88",
			ExternalKeyID:   "99",
			Name:            "ops-key",
			Secret:          "sk-provider-secret",
			Status:          "active",
			RawPayload:      map[string]any{"id": 99},
			CreatedAt:       time.Date(2026, 6, 21, 10, 0, 0, 0, time.UTC),
		},
	}
	local := &stubLocalAccountCreator{}
	svc := NewService(repo, session, keyAdapter, local)
	rate := 0.7

	result, err := svc.Provision(context.Background(), ProvisionKeyInput{
		SupplierID:                 7,
		SupplierGroupID:            10,
		Name:                       "ops-key",
		QuotaUSD:                   25,
		LocalAccountPlatform:       service.PlatformOpenAI,
		LocalAccountName:           "local-upstream",
		LocalAccountBaseURL:        "https://relay.example.com/v1",
		LocalAccountConcurrency:    3,
		LocalAccountPriority:       40,
		LocalAccountRateMultiplier: &rate,
		RuntimeStatus:              adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:               adminplusdomain.SupplierHealthStatusNormal,
		BalanceCurrency:            "USD",
	})

	require.NoError(t, err)
	require.NotNil(t, result.Key)
	require.NotNil(t, result.Binding)
	require.Equal(t, adminplusdomain.SupplierKeyStatusBound, result.Key.Status)
	require.Equal(t, "99", result.Key.ExternalKeyID)
	require.Equal(t, "cret", result.Key.KeyLast4)
	require.NotEqual(t, "sk-provider-secret", result.Key.KeyFingerprint)
	require.Equal(t, int64(1001), result.Key.LocalSub2APIAccountID)
	require.Equal(t, int64(1001), result.Binding.LocalSub2APIAccountID)
	require.NotNil(t, local.input.Schedulable)
	require.False(t, *local.input.Schedulable)
	require.Equal(t, result.Key.ID, result.Binding.SupplierKeyID)
	require.Equal(t, "openai", local.input.Platform)
	require.Equal(t, service.AccountTypeAPIKey, local.input.Type)
	require.Equal(t, "sk-provider-secret", local.input.Credentials["api_key"])
	require.Equal(t, "https://relay.example.com/v1", local.input.Credentials["base_url"])
	require.Equal(t, true, local.input.Credentials["pool_mode"])
	require.Empty(t, local.input.GroupIDs)
	require.True(t, local.input.SkipDefaultGroupBind)
	require.True(t, local.input.SkipMixedChannelCheck)
	require.Equal(t, []ports.CreateProviderKeyInput{{
		SupplierID:      7,
		ExternalGroupID: "88",
		Name:            "ops-key",
		QuotaUSD:        25,
		ExpiresInDays:   nil,
		Metadata: map[string]any{
			"supplier_group_id": int64(10),
			"provider_family":   "openai",
		},
	}}, keyAdapter.calls)
}

func TestServiceProvisionRejectsGroupWithExistingBoundKeyBeforeProviderCall(t *testing.T) {
	repo := NewMemoryRepository()
	repo.PutSupplier(&adminplusdomain.Supplier{
		ID:            7,
		Name:          "Relay",
		Type:          adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus: adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:  adminplusdomain.SupplierHealthStatusNormal,
	})
	repo.PutGroup(&adminplusdomain.SupplierGroup{
		ID:              10,
		SupplierID:      7,
		ExternalGroupID: "88",
		Name:            "Low Cost",
		ProviderFamily:  "openai",
		Status:          adminplusdomain.SupplierGroupStatusActive,
	})
	_, err := repo.CreateKey(context.Background(), &adminplusdomain.SupplierKey{
		SupplierID:      7,
		SupplierGroupID: 10,
		ExternalGroupID: "88",
		ExternalKeyID:   "99",
		Name:            "existing",
		KeyFingerprint:  "fingerprint",
		KeyLast4:        "cret",
		Status:          adminplusdomain.SupplierKeyStatusBound,
		ProviderFamily:  "openai",
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	})
	require.NoError(t, err)

	keyAdapter := &stubKeyAdapter{
		result: &ports.ProviderKeyResult{
			SupplierID:      7,
			ExternalGroupID: "88",
			ExternalKeyID:   "100",
			Name:            "new-key",
			Secret:          "sk-provider-secret",
		},
	}
	svc := NewService(repo, &stubSessionReader{}, keyAdapter, &stubLocalAccountCreator{})

	result, err := svc.Provision(context.Background(), ProvisionKeyInput{
		SupplierID:              7,
		SupplierGroupID:         10,
		Name:                    "new-key",
		LocalAccountPlatform:    service.PlatformOpenAI,
		LocalAccountName:        "local-upstream",
		LocalAccountBaseURL:     "https://relay.example.com/v1",
		BalanceCurrency:         "USD",
		RuntimeStatus:           adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:            adminplusdomain.SupplierHealthStatusNormal,
		LocalAccountConcurrency: 1,
	})

	require.Nil(t, result)
	require.Error(t, err)
	require.Contains(t, err.Error(), "SUPPLIER_GROUP_KEY_ALREADY_BOUND")
	require.Empty(t, keyAdapter.calls)
}

func TestServicePlanEnsureAllUsesKnownProviderCapacityWhenSupplierPolicyUnknown(t *testing.T) {
	repo := NewMemoryRepository()
	repo.PutSupplier(&adminplusdomain.Supplier{
		ID:              7,
		Name:            "Relay",
		Type:            adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus:   adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:    adminplusdomain.SupplierHealthStatusNormal,
		KeyLimitPolicy:  adminplusdomain.SupplierKeyLimitPolicyUnknown,
		BalanceCurrency: "USD",
	})
	for _, group := range []*adminplusdomain.SupplierGroup{
		{ID: 10, SupplierID: 7, ExternalGroupID: "g10", Name: "First", ProviderFamily: "openai", Status: adminplusdomain.SupplierGroupStatusActive},
		{ID: 20, SupplierID: 7, ExternalGroupID: "g20", Name: "Second", ProviderFamily: "openai", Status: adminplusdomain.SupplierGroupStatusActive},
	} {
		repo.PutGroup(group)
	}
	keyAdapter := &stubKeyAdapter{capacityResult: &ports.ProviderKeyCapacityResult{
		SupplierID:        7,
		SystemType:        "sub2api",
		KeyLimitPolicy:    adminplusdomain.SupplierKeyLimitPolicyUnlimited,
		KeyCapacityStatus: adminplusdomain.SupplierKeyCapacityAvailable,
		LimitKnown:        true,
	}}
	svc := NewService(repo, &stubSessionReader{input: ports.SessionProbeInput{SupplierID: 7}}, keyAdapter, &stubLocalAccountCreator{})

	plan, err := svc.PlanEnsureAll(context.Background(), EnsureAllInput{
		SupplierID:          7,
		LocalAccountBaseURL: "https://relay.example.com/v1",
		RuntimeStatus:       adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:        adminplusdomain.SupplierHealthStatusNormal,
		BalanceCurrency:     "USD",
	})

	require.NoError(t, err)
	require.Equal(t, adminplusdomain.SupplierKeyLimitPolicyUnlimited, plan.KeyLimitPolicy)
	require.Equal(t, 2, plan.ToCreate)
	require.Zero(t, plan.Blocked)
}

func TestServiceEnsureAllCompletesProviderToSub2APIFlowForDuplicateNames(t *testing.T) {
	repo := NewMemoryRepository()
	repo.PutSupplier(&adminplusdomain.Supplier{
		ID: 7, Name: "Relay", Type: adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus:   adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:    adminplusdomain.SupplierHealthStatusNormal,
		KeyLimitPolicy:  adminplusdomain.SupplierKeyLimitPolicyUnknown,
		BalanceCurrency: "USD",
	})
	for _, group := range []*adminplusdomain.SupplierGroup{
		{ID: 10, SupplierID: 7, ExternalGroupID: "g10", Name: "OpenAI", StandardKeyName: "supplier-OpenAI-1x", ProviderFamily: "openai", Status: adminplusdomain.SupplierGroupStatusActive},
		{ID: 20, SupplierID: 7, ExternalGroupID: "g20", Name: "OpenAI", StandardKeyName: "supplier-OpenAI-1x", ProviderFamily: "openai", Status: adminplusdomain.SupplierGroupStatusActive},
	} {
		repo.PutGroup(group)
	}
	keyAdapter := &stubKeyAdapter{
		capacityResult: &ports.ProviderKeyCapacityResult{
			SupplierID: 7, SystemType: "sub2api", KeyLimitPolicy: adminplusdomain.SupplierKeyLimitPolicyUnlimited,
			KeyCapacityStatus: adminplusdomain.SupplierKeyCapacityAvailable, LimitKnown: true,
		},
		resultsByGroup: map[string]*ports.ProviderKeyResult{
			"g10": {SupplierID: 7, ExternalGroupID: "g10", ExternalKeyID: "provider-10", Name: "supplier-OpenAI-1x", Secret: "sk-provider-10", Status: "active"},
			"g20": {SupplierID: 7, ExternalGroupID: "g20", ExternalKeyID: "provider-20", Name: "supplier-OpenAI-1x", Secret: "sk-provider-20", Status: "active"},
		},
	}
	local := &stubLocalAccountCreator{}
	svc := NewService(repo, &stubSessionReader{input: ports.SessionProbeInput{SupplierID: 7}}, keyAdapter, local)

	result, err := svc.EnsureAll(context.Background(), EnsureAllInput{
		SupplierID: 7, LocalAccountBaseURL: "https://relay.example.com/v1",
		RuntimeStatus:   adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:    adminplusdomain.SupplierHealthStatusNormal,
		BalanceCurrency: "USD",
	})

	require.NoError(t, err)
	require.Equal(t, 2, result.Total)
	require.Equal(t, 2, result.Created)
	require.Zero(t, result.Failed)
	require.Len(t, keyAdapter.calls, 2)
	require.Len(t, local.accounts, 2)
	names := make(map[string]struct{}, len(local.accounts))
	for _, account := range local.accounts {
		names[account.Name] = struct{}{}
		require.Equal(t, int64(7), int64FromMap(account.Extra, "admin_plus_supplier_id"))
	}
	require.Contains(t, names, "Relay / OpenAI [g10]")
	require.Contains(t, names, "Relay / OpenAI [g20]")
}

func TestLocalAccountMatchesLookupRejectsSameNameWithDifferentAdminPlusIdentity(t *testing.T) {
	account := &service.Account{
		ID:       1001,
		Name:     "duplicate-name",
		Platform: service.PlatformOpenAI,
		Extra: map[string]any{
			"admin_plus_supplier_id":       int64(7),
			"admin_plus_supplier_group_id": int64(10),
			"admin_plus_supplier_key":      int64(100),
		},
	}

	require.False(t, localAccountMatchesLookup(account, Sub2APIAccountLookupInput{
		SupplierID:           7,
		SupplierGroupID:      20,
		SupplierKeyID:        200,
		LocalAccountName:     "duplicate-name",
		LocalAccountPlatform: service.PlatformOpenAI,
	}))
	require.True(t, localAccountMatchesLookup(account, Sub2APIAccountLookupInput{
		SupplierID:           7,
		SupplierGroupID:      10,
		SupplierKeyID:        100,
		LocalAccountName:     "duplicate-name",
		LocalAccountPlatform: service.PlatformOpenAI,
	}))
	require.False(t, localAccountMatchesLookup(account, Sub2APIAccountLookupInput{
		SupplierID:           7,
		SupplierGroupID:      10,
		SupplierKeyID:        200,
		LocalAccountName:     "duplicate-name",
		LocalAccountPlatform: service.PlatformOpenAI,
	}))
	require.True(t, localAccountMatchesLookup(&service.Account{
		ID: 1002, Name: "duplicate-name", Platform: service.PlatformOpenAI,
	}, Sub2APIAccountLookupInput{
		SupplierID: 7, SupplierGroupID: 20, LocalAccountName: "duplicate-name", LocalAccountPlatform: service.PlatformOpenAI,
	}))
}

func TestLocalAccountNameForGroupHandlesLongUnicodeValues(t *testing.T) {
	stablePrefix := strings.Repeat("分", 80)
	groupA := &adminplusdomain.SupplierGroup{ID: 10, ExternalGroupID: stablePrefix + "甲"}
	groupB := &adminplusdomain.SupplierGroup{ID: 20, ExternalGroupID: stablePrefix + "乙"}
	preferred := strings.Repeat("中文账号", 80)

	nameA := localAccountNameForGroup(nil, groupA, preferred)
	nameB := localAccountNameForGroup(nil, groupB, preferred)

	require.True(t, utf8.ValidString(nameA))
	require.LessOrEqual(t, utf8.RuneCountInString(nameA), 160)
	require.NotEqual(t, nameA, nameB)
	require.True(t, strings.HasSuffix(nameA, " ["+trimLimit(groupA.ExternalGroupID, 35)+"-"+fingerprintSecret(groupA.ExternalGroupID)[:12]+"]"))
}

func TestTrimLimitHandlesNonPositiveAndUnicodeLimits(t *testing.T) {
	require.Empty(t, trimLimit("value", -1))
	require.Equal(t, "中文", trimLimit("中文账号", 2))
}

func TestServiceProvisionReusesProviderKeyAfterFailedLocalLanding(t *testing.T) {
	repo := NewMemoryRepository()
	repo.PutSupplier(&adminplusdomain.Supplier{
		ID: 7, Name: "Relay", Type: adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus:  adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:   adminplusdomain.SupplierHealthStatusNormal,
		KeyLimitPolicy: adminplusdomain.SupplierKeyLimitPolicyUnlimited,
	})
	repo.PutGroup(&adminplusdomain.SupplierGroup{
		ID: 10, SupplierID: 7, ExternalGroupID: "88", Name: "Low Cost",
		ProviderFamily: "openai", Status: adminplusdomain.SupplierGroupStatusActive,
	})
	secret := "sk-provider-secret"
	failed, err := repo.CreateKey(context.Background(), &adminplusdomain.SupplierKey{
		SupplierID: 7, SupplierGroupID: 10, ExternalGroupID: "88", ExternalKeyID: "99",
		Name: "ops-key", KeyFingerprint: fingerprintSecret(secret), KeyLast4: "cret",
		Status: adminplusdomain.SupplierKeyStatusFailed, ProviderFamily: "openai",
		ErrorCode: "LOCAL_ACCOUNT_CREATE_FAILED", ErrorMessage: "previous local landing failed",
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	})
	require.NoError(t, err)
	keyAdapter := &stubKeyAdapter{capacityResult: &ports.ProviderKeyCapacityResult{
		SupplierID: 7, SystemType: "sub2api", KeyLimitPolicy: adminplusdomain.SupplierKeyLimitPolicyUnlimited,
		KeyCapacityStatus: adminplusdomain.SupplierKeyCapacityAvailable, LimitKnown: true,
		Keys: []ports.ProviderKeySnapshot{{
			SupplierID: 7, ExternalGroupID: "88", ExternalKeyID: "99", Name: "ops-key", Status: "active", Secret: secret,
		}},
	}}
	local := &stubLocalAccountCreator{}
	svc := NewService(repo, &stubSessionReader{input: ports.SessionProbeInput{SupplierID: 7}}, keyAdapter, local)

	result, err := svc.Provision(context.Background(), ProvisionKeyInput{
		SupplierID: 7, SupplierGroupID: 10, Name: "ops-key",
		LocalAccountPlatform: service.PlatformOpenAI,
		LocalAccountBaseURL:  "https://relay.example.com/v1",
		RuntimeStatus:        adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:         adminplusdomain.SupplierHealthStatusNormal,
		BalanceCurrency:      "USD",
	})

	require.NoError(t, err)
	require.Equal(t, failed.ID, result.Key.ID)
	require.Equal(t, adminplusdomain.SupplierKeyStatusBound, result.Key.Status)
	require.Equal(t, int64(1001), result.Key.LocalSub2APIAccountID)
	require.Empty(t, keyAdapter.calls)
	require.Equal(t, secret, local.input.Credentials["api_key"])
}

func TestServiceEnsureGroupRecreatesDeletedLocalAccountFromProviderSecret(t *testing.T) {
	repo := NewMemoryRepository()
	repo.PutSupplier(&adminplusdomain.Supplier{
		ID: 7, Name: "Relay", Type: adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus:  adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:   adminplusdomain.SupplierHealthStatusNormal,
		KeyLimitPolicy: adminplusdomain.SupplierKeyLimitPolicyUnlimited,
	})
	repo.PutGroup(&adminplusdomain.SupplierGroup{
		ID: 10, SupplierID: 7, ExternalGroupID: "88", Name: "Low Cost",
		ProviderFamily: "openai", Status: adminplusdomain.SupplierGroupStatusActive,
	})
	key, err := repo.CreateKey(context.Background(), &adminplusdomain.SupplierKey{
		SupplierID: 7, SupplierGroupID: 10, ExternalGroupID: "88", ExternalKeyID: "99",
		Name: "ops-key", KeyFingerprint: fingerprintSecret("sk-provider-secret"), KeyLast4: "cret",
		Status: adminplusdomain.SupplierKeyStatusBound, ProviderFamily: "openai",
		LocalSub2APIAccountID: 77, LocalAccountName: "deleted-local-account", LocalAccountPlatform: service.PlatformOpenAI,
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	})
	require.NoError(t, err)
	oldBinding, err := repo.CreateBinding(context.Background(), &adminplusdomain.SupplierAccount{
		SupplierID:                7,
		SupplierKeyID:             key.ID,
		LocalSub2APIAccountID:     77,
		SupplierAccountIdentifier: "99",
		SupplierAccountLabel:      "ops-key",
		RuntimeStatus:             adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:              adminplusdomain.SupplierHealthStatusNormal,
		BalanceCurrency:           "USD",
	})
	require.NoError(t, err)
	local := &stubLocalAccountCreator{missingAccountIDs: map[int64]bool{77: true}}
	keyAdapter := &stubKeyAdapter{readResult: &ports.ProviderKeyResult{
		SupplierID: 7, ExternalGroupID: "88", ExternalKeyID: "99", Name: "ops-key", Status: "active", Secret: "sk-provider-secret",
	}}
	svc := NewService(repo, &stubSessionReader{input: ports.SessionProbeInput{SupplierID: 7}}, keyAdapter, local)

	result, err := svc.EnsureGroup(context.Background(), EnsureGroupInput{
		SupplierGroupID: 10,
		EnsureAllInput: EnsureAllInput{
			SupplierID: 7, LocalAccountBaseURL: "https://relay.example.com/v1",
			RuntimeStatus:   adminplusdomain.SupplierRuntimeStatusMonitorOnly,
			HealthStatus:    adminplusdomain.SupplierHealthStatusNormal,
			BalanceCurrency: "USD",
		},
	})

	require.NoError(t, err)
	require.Equal(t, "skipped", result.Action)
	require.Equal(t, key.ID, result.Key.ID)
	require.Equal(t, int64(1001), result.Key.LocalSub2APIAccountID)
	require.NotNil(t, result.Binding)
	require.Equal(t, oldBinding.ID, result.Binding.ID)
	require.Equal(t, key.ID, result.Binding.SupplierKeyID)
	require.Equal(t, int64(1001), result.Binding.LocalSub2APIAccountID)
	require.Equal(t, []ports.ReadProviderKeyInput{{
		SupplierID: 7, ExternalKeyID: "99", ExternalGroupID: "88", Name: "ops-key",
	}}, keyAdapter.readCalls)
	require.Empty(t, keyAdapter.calls)
	require.Equal(t, "sk-provider-secret", local.input.Credentials["api_key"])
}

func TestServiceProvisionRejectsExhaustedSupplierKeyCapacityBeforeProviderCall(t *testing.T) {
	repo := NewMemoryRepository()
	repo.PutSupplier(&adminplusdomain.Supplier{
		ID:             7,
		Name:           "Relay",
		Type:           adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus:  adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:   adminplusdomain.SupplierHealthStatusNormal,
		KeyLimitPolicy: adminplusdomain.SupplierKeyLimitPolicyLimited,
		KeyLimitValue:  1,
	})
	repo.PutGroup(&adminplusdomain.SupplierGroup{
		ID:              10,
		SupplierID:      7,
		ExternalGroupID: "88",
		Name:            "Blocked",
		ProviderFamily:  "openai",
		Status:          adminplusdomain.SupplierGroupStatusActive,
	})
	repo.PutGroup(&adminplusdomain.SupplierGroup{
		ID:              20,
		SupplierID:      7,
		ExternalGroupID: "99",
		Name:            "Existing",
		ProviderFamily:  "openai",
		Status:          adminplusdomain.SupplierGroupStatusActive,
	})
	_, err := repo.CreateKey(context.Background(), &adminplusdomain.SupplierKey{
		SupplierID:      7,
		SupplierGroupID: 20,
		ExternalGroupID: "99",
		ExternalKeyID:   "existing",
		Name:            "existing",
		KeyFingerprint:  "fingerprint",
		KeyLast4:        "cret",
		Status:          adminplusdomain.SupplierKeyStatusBound,
		ProviderFamily:  "openai",
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	})
	require.NoError(t, err)
	keyAdapter := &stubKeyAdapter{
		result: &ports.ProviderKeyResult{
			SupplierID:      7,
			ExternalGroupID: "88",
			ExternalKeyID:   "new",
			Name:            "new-key",
			Secret:          "sk-provider-secret",
		},
	}
	svc := NewService(repo, &stubSessionReader{}, keyAdapter, &stubLocalAccountCreator{})

	result, err := svc.Provision(context.Background(), ProvisionKeyInput{
		SupplierID:              7,
		SupplierGroupID:         10,
		Name:                    "new-key",
		LocalAccountPlatform:    service.PlatformOpenAI,
		LocalAccountName:        "local-upstream",
		LocalAccountBaseURL:     "https://relay.example.com/v1",
		BalanceCurrency:         "USD",
		RuntimeStatus:           adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:            adminplusdomain.SupplierHealthStatusNormal,
		LocalAccountConcurrency: 1,
	})

	require.Nil(t, result)
	require.Error(t, err)
	require.Equal(t, "SUPPLIER_KEY_CAPACITY_EXHAUSTED", infraerrors.Reason(err))
	require.Empty(t, keyAdapter.calls)
}

func TestServiceProvisionRejectsUnsupportedKeyProvisioningPolicyBeforeProviderCall(t *testing.T) {
	repo := NewMemoryRepository()
	repo.PutSupplier(&adminplusdomain.Supplier{
		ID:             7,
		Name:           "Relay",
		Type:           adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus:  adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:   adminplusdomain.SupplierHealthStatusNormal,
		KeyLimitPolicy: adminplusdomain.SupplierKeyLimitPolicyUnsupported,
	})
	repo.PutGroup(&adminplusdomain.SupplierGroup{
		ID:              10,
		SupplierID:      7,
		ExternalGroupID: "88",
		Name:            "Manual",
		ProviderFamily:  "openai",
		Status:          adminplusdomain.SupplierGroupStatusActive,
	})
	keyAdapter := &stubKeyAdapter{
		result: &ports.ProviderKeyResult{
			SupplierID:      7,
			ExternalGroupID: "88",
			ExternalKeyID:   "new",
			Name:            "new-key",
			Secret:          "sk-provider-secret",
		},
	}
	svc := NewService(repo, &stubSessionReader{}, keyAdapter, &stubLocalAccountCreator{})

	result, err := svc.Provision(context.Background(), ProvisionKeyInput{
		SupplierID:              7,
		SupplierGroupID:         10,
		Name:                    "new-key",
		LocalAccountPlatform:    service.PlatformOpenAI,
		LocalAccountName:        "local-upstream",
		LocalAccountBaseURL:     "https://relay.example.com/v1",
		BalanceCurrency:         "USD",
		RuntimeStatus:           adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:            adminplusdomain.SupplierHealthStatusNormal,
		LocalAccountConcurrency: 1,
	})

	require.Nil(t, result)
	require.Error(t, err)
	require.Equal(t, "SUPPLIER_KEY_PROVISIONING_UNSUPPORTED", infraerrors.Reason(err))
	require.Empty(t, keyAdapter.calls)
}

func TestServiceEnsureAllFailsBeforeProviderKeyWhenSub2APIGatewayUnavailable(t *testing.T) {
	repo := NewMemoryRepository()
	repo.PutSupplier(&adminplusdomain.Supplier{
		ID:             7,
		Name:           "Relay",
		Type:           adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus:  adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:   adminplusdomain.SupplierHealthStatusNormal,
		APIBaseURL:     "https://relay.example.com",
		KeyLimitPolicy: adminplusdomain.SupplierKeyLimitPolicyUnlimited,
	})
	repo.PutGroup(&adminplusdomain.SupplierGroup{
		ID:              10,
		SupplierID:      7,
		ExternalGroupID: "88",
		Name:            "Low Cost",
		ProviderFamily:  "openai",
		Status:          adminplusdomain.SupplierGroupStatusActive,
	})
	keyAdapter := &stubKeyAdapter{
		result: &ports.ProviderKeyResult{
			SupplierID:      7,
			ExternalGroupID: "88",
			ExternalKeyID:   "99",
			Name:            "ops-key",
			Secret:          "sk-provider-secret",
		},
	}
	svc := NewService(repo, &stubSessionReader{}, keyAdapter, NewFailingSub2APIGateway(nil))

	result, err := svc.EnsureAll(context.Background(), EnsureAllInput{
		SupplierID:          7,
		LocalAccountBaseURL: "https://relay.example.com/v1",
		RuntimeStatus:       adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:        adminplusdomain.SupplierHealthStatusNormal,
		BalanceCurrency:     "USD",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 1, result.Total)
	require.Equal(t, 0, result.Created)
	require.Equal(t, 1, result.Failed)
	require.Len(t, result.Items, 1)
	require.Equal(t, "failed", result.Items[0].Action)
	require.Equal(t, "LOCAL_SUB2API_ACCOUNT_LOOKUP_FAILED", result.Items[0].ErrorCode)
	require.Contains(t, result.Items[0].ErrorMessage, "failed to lookup local Sub2API account")
	require.Contains(t, result.Items[0].ErrorMessage, "SUB2API_GATEWAY_CONFIG_REQUIRED")
	require.Empty(t, keyAdapter.calls)
	keys, listErr := repo.List(context.Background(), ListFilter{SupplierID: 7})
	require.NoError(t, listErr)
	require.Empty(t, keys)
}

func TestIsLocalAccountNotFoundAcceptsStatusAndLocalizedMessages(t *testing.T) {
	require.True(t, isLocalAccountNotFound(infraerrors.New(http.StatusNotFound, "SUB2API_GATEWAY_BAD_STATUS", "账号不存在")))
	require.True(t, isLocalAccountNotFound(infraerrors.New(http.StatusBadGateway, "LOCAL_ACCOUNT_DELETED", "本地账号已删除")))
	require.True(t, isLocalAccountNotFound(infraerrors.New(http.StatusBadGateway, "LOCAL_ACCOUNT_MISSING", "record does not exist")))
	require.False(t, isLocalAccountNotFound(infraerrors.New(http.StatusUnauthorized, "UNAUTHORIZED", "invalid admin key")))
}

func TestLocalGatewayErrorDetailRedactsSensitiveCause(t *testing.T) {
	err := localGatewayError(
		"LOCAL_SUB2API_GROUP_LIST_FAILED",
		"failed to list local Sub2API groups",
		infraerrors.New(http.StatusUnauthorized, "INVALID_ADMIN_KEY", "invalid api_key value"),
	)

	require.Contains(t, err.Error(), "error detail redacted because it contains sensitive fields")
	require.NotContains(t, err.Error(), "invalid api_key value")
}

func TestServiceEnsureAllDoesNotCreateOrBindLocalGroupForExistingKey(t *testing.T) {
	repo := NewMemoryRepository()
	repo.PutSupplier(&adminplusdomain.Supplier{
		ID:             7,
		Name:           "Relay",
		Type:           adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus:  adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:   adminplusdomain.SupplierHealthStatusNormal,
		APIBaseURL:     "https://relay.example.com",
		KeyLimitPolicy: adminplusdomain.SupplierKeyLimitPolicyUnlimited,
	})
	repo.PutGroup(&adminplusdomain.SupplierGroup{
		ID:                      10,
		SupplierID:              7,
		ExternalGroupID:         "88",
		Name:                    "Low Cost",
		ProviderFamily:          "openai",
		RateMultiplier:          0.7,
		EffectiveRateMultiplier: 0.7,
		Status:                  adminplusdomain.SupplierGroupStatusActive,
	})
	key, err := repo.CreateKey(context.Background(), &adminplusdomain.SupplierKey{
		SupplierID:      7,
		SupplierGroupID: 10,
		ExternalGroupID: "88",
		ExternalKeyID:   "99",
		Name:            "existing",
		KeyFingerprint:  "fingerprint",
		KeyLast4:        "cret",
		Status:          adminplusdomain.SupplierKeyStatusBound,
		ProviderFamily:  "openai",
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	})
	require.NoError(t, err)
	_, err = repo.UpdateKeyAfterLocalBind(context.Background(), key.ID, &service.Account{
		ID:       2002,
		Name:     "existing-local",
		Platform: service.PlatformOpenAI,
		Type:     service.AccountTypeAPIKey,
	}, adminplusdomain.SupplierKeyStatusBound, "", "")
	require.NoError(t, err)

	local := &stubLocalAccountCreator{
		accounts: map[int64]*service.Account{
			2002: {
				ID:       2002,
				Name:     "existing-local",
				Platform: service.PlatformOpenAI,
				Type:     service.AccountTypeAPIKey,
			},
		},
	}
	keyAdapter := &stubKeyAdapter{
		result: &ports.ProviderKeyResult{
			SupplierID:    7,
			ExternalKeyID: "should-not-be-created",
			Secret:        "sk-provider-secret",
		},
	}
	svc := NewService(repo, &stubSessionReader{}, keyAdapter, local)

	result, err := svc.EnsureAll(context.Background(), EnsureAllInput{
		SupplierID:          7,
		LocalAccountBaseURL: "https://relay.example.com/v1",
		RuntimeStatus:       adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:        adminplusdomain.SupplierHealthStatusNormal,
		BalanceCurrency:     "USD",
	})

	require.NoError(t, err)
	require.Equal(t, 1, result.Total)
	require.Equal(t, 0, result.Created)
	require.Equal(t, 1, result.Skipped)
	require.Equal(t, 0, result.LocalGroupsCreated)
	require.Equal(t, 0, result.LocalAccountsBound)
	require.Empty(t, keyAdapter.calls)
	require.Empty(t, local.accounts[2002].GroupIDs)
}

func TestServiceEnsureAllSupportsNewAPISupplier(t *testing.T) {
	repo := NewMemoryRepository()
	repo.PutSupplier(&adminplusdomain.Supplier{
		ID:              7,
		Name:            "Codex APIs",
		Type:            adminplusdomain.SupplierTypeNewAPI,
		RuntimeStatus:   adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:    adminplusdomain.SupplierHealthStatusNormal,
		APIBaseURL:      "https://www.codexapis.com",
		BalanceCurrency: "QTA",
		KeyLimitPolicy:  adminplusdomain.SupplierKeyLimitPolicyUnlimited,
	})
	repo.PutGroup(&adminplusdomain.SupplierGroup{
		ID:                      10,
		SupplierID:              7,
		ExternalGroupID:         "kiro",
		Name:                    "kiro",
		ProviderFamily:          "new_api",
		RateMultiplier:          0.3,
		EffectiveRateMultiplier: 0.3,
		Status:                  adminplusdomain.SupplierGroupStatusActive,
	})
	session := &stubSessionReader{
		input: ports.SessionProbeInput{
			SupplierID: 7,
			APIBaseURL: "https://www.codexapis.com",
			Bundle:     map[string]any{"provider_type": "new_api", "auth_header_value": "4111"},
		},
	}
	keyAdapter := &stubKeyAdapter{
		result: &ports.ProviderKeyResult{
			SupplierID:      7,
			ExternalGroupID: "kiro",
			ExternalKeyID:   "701",
			Name:            "AdminPlus-kiro-kiro",
			Secret:          "sk-new-api-secret",
			Status:          "active",
			RawPayload:      map[string]any{"id": 701, "group": "kiro"},
		},
	}
	local := &stubLocalAccountCreator{}
	svc := NewService(repo, session, keyAdapter, local)

	result, err := svc.EnsureAll(context.Background(), EnsureAllInput{
		SupplierID:              7,
		LocalAccountBaseURL:     "https://www.codexapis.com/v1",
		LocalAccountConcurrency: 2,
		LocalAccountPriority:    100,
		RuntimeStatus:           adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:            adminplusdomain.SupplierHealthStatusNormal,
		BalanceCurrency:         "QTA",
	})

	require.NoError(t, err)
	require.Equal(t, 1, result.Total)
	require.Equal(t, 1, result.Created)
	require.Equal(t, 0, result.Failed)
	require.Len(t, result.Items, 1)
	require.Equal(t, adminplusdomain.SupplierKeyStatusBound, result.Items[0].Key.Status)
	require.Equal(t, "701", result.Items[0].Key.ExternalKeyID)
	require.Equal(t, "new_api", result.Items[0].Key.ProviderFamily)
	require.Equal(t, "USD", result.Items[0].Binding.BalanceCurrency)
	require.Equal(t, "openai", local.input.Platform)
	require.Equal(t, "https://www.codexapis.com/v1", local.input.Credentials["base_url"])
	require.Equal(t, "sk-new-api-secret", local.input.Credentials["api_key"])
	require.Equal(t, []ports.CreateProviderKeyInput{{
		SupplierID:      7,
		ExternalGroupID: "kiro",
		Name:            "AdminPlus-kiro-kiro",
		QuotaUSD:        0,
		ExpiresInDays:   nil,
		Metadata: map[string]any{
			"supplier_group_id": int64(10),
			"provider_family":   "new_api",
		},
	}}, keyAdapter.calls)
}

func TestServicePlanEnsureAllLimitedCapacityPrioritizesLowestRate(t *testing.T) {
	repo := NewMemoryRepository()
	repo.PutSupplier(&adminplusdomain.Supplier{
		ID:              7,
		Name:            "Relay",
		Type:            adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus:   adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:    adminplusdomain.SupplierHealthStatusNormal,
		APIBaseURL:      "https://relay.example.com",
		KeyLimitPolicy:  adminplusdomain.SupplierKeyLimitPolicyLimited,
		KeyLimitValue:   2,
		BalanceCurrency: "USD",
	})
	for _, group := range []*adminplusdomain.SupplierGroup{
		{ID: 10, SupplierID: 7, ExternalGroupID: "g10", Name: "Existing", ProviderFamily: "openai", EffectiveRateMultiplier: 0.4, Status: adminplusdomain.SupplierGroupStatusActive},
		{ID: 20, SupplierID: 7, ExternalGroupID: "g20", Name: "Lowest", ProviderFamily: "openai", EffectiveRateMultiplier: 0.1, Status: adminplusdomain.SupplierGroupStatusActive},
		{ID: 30, SupplierID: 7, ExternalGroupID: "g30", Name: "Mid", ProviderFamily: "openai", EffectiveRateMultiplier: 0.2, Status: adminplusdomain.SupplierGroupStatusActive},
		{ID: 40, SupplierID: 7, ExternalGroupID: "g40", Name: "High", ProviderFamily: "openai", EffectiveRateMultiplier: 0.3, Status: adminplusdomain.SupplierGroupStatusActive},
	} {
		repo.PutGroup(group)
	}
	_, err := repo.CreateKey(context.Background(), &adminplusdomain.SupplierKey{
		SupplierID:      7,
		SupplierGroupID: 10,
		ExternalGroupID: "g10",
		Name:            "existing",
		Status:          adminplusdomain.SupplierKeyStatusBound,
		ProviderFamily:  "openai",
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	})
	require.NoError(t, err)
	svc := NewService(repo, &stubSessionReader{}, &stubKeyAdapter{}, &stubLocalAccountCreator{})

	plan, err := svc.PlanEnsureAll(context.Background(), EnsureAllInput{
		SupplierID:          7,
		LocalAccountBaseURL: "https://relay.example.com/v1",
		RuntimeStatus:       adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:        adminplusdomain.SupplierHealthStatusNormal,
		BalanceCurrency:     "USD",
	})

	require.NoError(t, err)
	require.Equal(t, adminplusdomain.SupplierKeyLimitPolicyLimited, plan.KeyLimitPolicy)
	require.Equal(t, 1, plan.ActiveKeyCount)
	require.Equal(t, 1, plan.RemainingKeySlots)
	require.Equal(t, 4, plan.Total)
	require.Equal(t, 1, plan.ToCreate)
	require.Equal(t, 1, plan.AlreadySatisfied)
	require.Equal(t, 2, plan.Blocked)
	require.Equal(t, "create", supplierPlanItem(plan, 20).Action)
	require.Equal(t, 1, supplierPlanItem(plan, 20).Priority)
	require.Equal(t, "skipped_existing", supplierPlanItem(plan, 10).Action)
	require.Equal(t, "blocked", supplierPlanItem(plan, 30).Action)
	require.Equal(t, "key_capacity_exhausted", supplierPlanItem(plan, 30).BlockedReason)
	require.Equal(t, "blocked", supplierPlanItem(plan, 40).Action)
}

func TestServicePlanEnsureAllLimitedCapacityHonorsPriorityOverride(t *testing.T) {
	repo := NewMemoryRepository()
	repo.PutSupplier(&adminplusdomain.Supplier{
		ID:              7,
		Name:            "Relay",
		Type:            adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus:   adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:    adminplusdomain.SupplierHealthStatusNormal,
		APIBaseURL:      "https://relay.example.com",
		KeyLimitPolicy:  adminplusdomain.SupplierKeyLimitPolicyLimited,
		KeyLimitValue:   2,
		BalanceCurrency: "USD",
	})
	for _, group := range []*adminplusdomain.SupplierGroup{
		{ID: 10, SupplierID: 7, ExternalGroupID: "g10", Name: "Existing", ProviderFamily: "openai", EffectiveRateMultiplier: 0.4, Status: adminplusdomain.SupplierGroupStatusActive},
		{ID: 20, SupplierID: 7, ExternalGroupID: "g20", Name: "Lowest", ProviderFamily: "openai", EffectiveRateMultiplier: 0.1, Status: adminplusdomain.SupplierGroupStatusActive},
		{ID: 30, SupplierID: 7, ExternalGroupID: "g30", Name: "Mid", ProviderFamily: "openai", EffectiveRateMultiplier: 0.2, Status: adminplusdomain.SupplierGroupStatusActive},
		{ID: 40, SupplierID: 7, ExternalGroupID: "g40", Name: "High Pinned", ProviderFamily: "openai", EffectiveRateMultiplier: 0.3, Status: adminplusdomain.SupplierGroupStatusActive},
	} {
		repo.PutGroup(group)
	}
	_, err := repo.CreateKey(context.Background(), &adminplusdomain.SupplierKey{
		SupplierID:      7,
		SupplierGroupID: 10,
		ExternalGroupID: "g10",
		Name:            "existing",
		Status:          adminplusdomain.SupplierKeyStatusBound,
		ProviderFamily:  "openai",
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	})
	require.NoError(t, err)
	svc := NewService(repo, &stubSessionReader{}, &stubKeyAdapter{}, &stubLocalAccountCreator{})

	plan, err := svc.PlanEnsureAll(context.Background(), EnsureAllInput{
		SupplierID:               7,
		SupplierGroupPriorityIDs: []int64{40, 20},
		LocalAccountBaseURL:      "https://relay.example.com/v1",
		RuntimeStatus:            adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:             adminplusdomain.SupplierHealthStatusNormal,
		BalanceCurrency:          "USD",
	})

	require.NoError(t, err)
	require.Equal(t, 1, plan.ToCreate)
	require.Equal(t, "create", supplierPlanItem(plan, 40).Action)
	require.Equal(t, 1, supplierPlanItem(plan, 40).Priority)
	require.Equal(t, "blocked", supplierPlanItem(plan, 20).Action)
	require.Equal(t, "key_capacity_exhausted", supplierPlanItem(plan, 20).BlockedReason)
	require.Equal(t, 2, supplierPlanItem(plan, 20).Priority)
	require.Equal(t, "blocked", supplierPlanItem(plan, 30).Action)
	require.Equal(t, "key_capacity_exhausted", supplierPlanItem(plan, 30).BlockedReason)
	require.Equal(t, 3, supplierPlanItem(plan, 30).Priority)
}

func TestServicePlanEnsureAllBlocksExplicitGroupCapacityRules(t *testing.T) {
	repo := NewMemoryRepository()
	repo.PutSupplier(&adminplusdomain.Supplier{
		ID:              7,
		Name:            "Relay",
		Type:            adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus:   adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:    adminplusdomain.SupplierHealthStatusNormal,
		APIBaseURL:      "https://relay.example.com",
		KeyLimitPolicy:  adminplusdomain.SupplierKeyLimitPolicyUnlimited,
		BalanceCurrency: "USD",
	})
	for _, group := range []*adminplusdomain.SupplierGroup{
		{
			ID:                      10,
			SupplierID:              7,
			ExternalGroupID:         "g10",
			Name:                    "Inherit",
			ProviderFamily:          "openai",
			EffectiveRateMultiplier: 0.1,
			KeyLimitPolicy:          adminplusdomain.SupplierGroupKeyLimitPolicyInherit,
			Status:                  adminplusdomain.SupplierGroupStatusActive,
		},
		{
			ID:                      20,
			SupplierID:              7,
			ExternalGroupID:         "g20",
			Name:                    "Unknown Group",
			ProviderFamily:          "openai",
			EffectiveRateMultiplier: 0.2,
			KeyLimitPolicy:          adminplusdomain.SupplierGroupKeyLimitPolicyUnknown,
			Status:                  adminplusdomain.SupplierGroupStatusActive,
		},
		{
			ID:                      30,
			SupplierID:              7,
			ExternalGroupID:         "g30",
			Name:                    "Unsupported Group",
			ProviderFamily:          "openai",
			EffectiveRateMultiplier: 0.3,
			KeyLimitPolicy:          adminplusdomain.SupplierGroupKeyLimitPolicyUnsupported,
			Status:                  adminplusdomain.SupplierGroupStatusActive,
		},
		{
			ID:                      40,
			SupplierID:              7,
			ExternalGroupID:         "g40",
			Name:                    "Full Group",
			ProviderFamily:          "openai",
			EffectiveRateMultiplier: 0.4,
			KeyLimitPolicy:          adminplusdomain.SupplierGroupKeyLimitPolicyLimited,
			KeyLimitValue:           0,
			Status:                  adminplusdomain.SupplierGroupStatusActive,
		},
	} {
		repo.PutGroup(group)
	}
	svc := NewService(repo, &stubSessionReader{}, &stubKeyAdapter{}, &stubLocalAccountCreator{})

	plan, err := svc.PlanEnsureAll(context.Background(), EnsureAllInput{
		SupplierID:          7,
		LocalAccountBaseURL: "https://relay.example.com/v1",
		RuntimeStatus:       adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:        adminplusdomain.SupplierHealthStatusNormal,
		BalanceCurrency:     "USD",
	})

	require.NoError(t, err)
	require.Equal(t, 4, plan.Total)
	require.Equal(t, 1, plan.ToCreate)
	require.Equal(t, 3, plan.Blocked)
	require.Equal(t, "create", supplierPlanItem(plan, 10).Action)
	require.Equal(t, "group_key_capacity_unknown", supplierPlanItem(plan, 20).BlockedReason)
	require.Equal(t, "group_key_provisioning_unsupported", supplierPlanItem(plan, 30).BlockedReason)
	require.Equal(t, "group_key_capacity_exhausted", supplierPlanItem(plan, 40).BlockedReason)
	require.Equal(t, 0, supplierPlanItem(plan, 40).GroupRemainingKeySlots)
}

func TestServiceDisableLocalProjectionReleasesCapacityForPlan(t *testing.T) {
	repo := NewMemoryRepository()
	repo.PutSupplier(&adminplusdomain.Supplier{
		ID:              7,
		Name:            "Relay",
		Type:            adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus:   adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:    adminplusdomain.SupplierHealthStatusNormal,
		APIBaseURL:      "https://relay.example.com",
		KeyLimitPolicy:  adminplusdomain.SupplierKeyLimitPolicyLimited,
		KeyLimitValue:   1,
		BalanceCurrency: "USD",
	})
	for _, group := range []*adminplusdomain.SupplierGroup{
		{ID: 10, SupplierID: 7, ExternalGroupID: "g10", Name: "Existing", ProviderFamily: "openai", EffectiveRateMultiplier: 0.4, Status: adminplusdomain.SupplierGroupStatusActive},
		{ID: 20, SupplierID: 7, ExternalGroupID: "g20", Name: "Lowest", ProviderFamily: "openai", EffectiveRateMultiplier: 0.1, Status: adminplusdomain.SupplierGroupStatusActive},
	} {
		repo.PutGroup(group)
	}
	key, err := repo.CreateKey(context.Background(), &adminplusdomain.SupplierKey{
		SupplierID:      7,
		SupplierGroupID: 10,
		ExternalGroupID: "g10",
		Name:            "existing",
		Status:          adminplusdomain.SupplierKeyStatusBound,
		ProviderFamily:  "openai",
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	})
	require.NoError(t, err)
	svc := NewService(repo, &stubSessionReader{}, &stubKeyAdapter{}, &stubLocalAccountCreator{})

	before, err := svc.PlanEnsureAll(context.Background(), EnsureAllInput{
		SupplierID:          7,
		LocalAccountBaseURL: "https://relay.example.com/v1",
		RuntimeStatus:       adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:        adminplusdomain.SupplierHealthStatusNormal,
		BalanceCurrency:     "USD",
	})
	require.NoError(t, err)
	require.Equal(t, 1, before.ActiveKeyCount)
	require.Equal(t, 1, before.Blocked)
	require.Equal(t, "blocked", supplierPlanItem(before, 20).Action)

	disabled, err := svc.DisableLocalProjection(context.Background(), DisableLocalProjectionInput{
		SupplierID: 7,
		KeyID:      key.ID,
		Reason:     "unused test key",
	})
	require.NoError(t, err)
	require.Equal(t, adminplusdomain.SupplierKeyStatusDisabled, disabled.Status)
	require.Equal(t, "LOCAL_PROJECTION_RELEASED", disabled.ErrorCode)
	require.Contains(t, disabled.ErrorMessage, "unused test key")

	after, err := svc.PlanEnsureAll(context.Background(), EnsureAllInput{
		SupplierID:          7,
		LocalAccountBaseURL: "https://relay.example.com/v1",
		RuntimeStatus:       adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:        adminplusdomain.SupplierHealthStatusNormal,
		BalanceCurrency:     "USD",
	})
	require.NoError(t, err)
	require.Equal(t, 0, after.ActiveKeyCount)
	require.Equal(t, 1, after.ToCreate)
	require.Equal(t, 1, after.Blocked)
	require.Equal(t, "create", supplierPlanItem(after, 20).Action)
	require.Equal(t, "blocked", supplierPlanItem(after, 10).Action)
}

func TestServicePlanEnsureAllUsesProviderActiveKeyCountAfterLocalProjectionRelease(t *testing.T) {
	repo := NewMemoryRepository()
	repo.PutSupplier(&adminplusdomain.Supplier{
		ID:              7,
		Name:            "Relay",
		Type:            adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus:   adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:    adminplusdomain.SupplierHealthStatusNormal,
		APIBaseURL:      "https://relay.example.com",
		KeyLimitPolicy:  adminplusdomain.SupplierKeyLimitPolicyLimited,
		KeyLimitValue:   1,
		BalanceCurrency: "USD",
	})
	for _, group := range []*adminplusdomain.SupplierGroup{
		{ID: 10, SupplierID: 7, ExternalGroupID: "g10", Name: "Existing", ProviderFamily: "openai", EffectiveRateMultiplier: 0.4, Status: adminplusdomain.SupplierGroupStatusActive},
		{ID: 20, SupplierID: 7, ExternalGroupID: "g20", Name: "Lowest", ProviderFamily: "openai", EffectiveRateMultiplier: 0.1, Status: adminplusdomain.SupplierGroupStatusActive},
	} {
		repo.PutGroup(group)
	}
	key, err := repo.CreateKey(context.Background(), &adminplusdomain.SupplierKey{
		SupplierID:      7,
		SupplierGroupID: 10,
		ExternalGroupID: "g10",
		ExternalKeyID:   "provider-g10",
		Name:            "existing",
		Status:          adminplusdomain.SupplierKeyStatusBound,
		ProviderFamily:  "openai",
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	})
	require.NoError(t, err)
	_, err = repo.DisableLocalProjection(context.Background(), 7, key.ID, "released locally")
	require.NoError(t, err)

	keyAdapter := &stubKeyAdapter{
		listResult: &ports.ListProviderKeysResult{
			SupplierID: 7,
			Keys: []ports.ProviderKeySnapshot{
				{
					SupplierID:      7,
					ExternalGroupID: "g10",
					ExternalKeyID:   "provider-g10",
					Name:            "provider still active",
					Status:          "active",
				},
			},
		},
	}
	svc := NewService(repo, &stubSessionReader{input: ports.SessionProbeInput{SupplierID: 7}}, keyAdapter, &stubLocalAccountCreator{})

	plan, err := svc.PlanEnsureAll(context.Background(), EnsureAllInput{
		SupplierID:          7,
		LocalAccountBaseURL: "https://relay.example.com/v1",
		RuntimeStatus:       adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:        adminplusdomain.SupplierHealthStatusNormal,
		BalanceCurrency:     "USD",
	})

	require.NoError(t, err)
	require.Equal(t, 1, plan.ActiveKeyCount)
	require.Equal(t, 0, plan.RemainingKeySlots)
	require.Equal(t, 0, plan.ToCreate)
	require.Equal(t, 2, plan.Blocked)
	providerBlocked := supplierPlanItem(plan, 10)
	require.Equal(t, "provider_key_exists_unbound", providerBlocked.BlockedReason)
	require.Equal(t, "provider-g10", providerBlocked.ProviderExternalKeyID)
	require.Equal(t, "provider still active", providerBlocked.ProviderKeyName)
	require.Equal(t, "key_capacity_exhausted", supplierPlanItem(plan, 20).BlockedReason)
	require.Len(t, keyAdapter.capacityCalls, 1)
	require.Equal(t, 5000, keyAdapter.capacityCalls[0].Limit)
}

func TestServiceEnsureAllReusesProviderKeyWithSecret(t *testing.T) {
	repo := NewMemoryRepository()
	repo.PutSupplier(&adminplusdomain.Supplier{
		ID:              7,
		Name:            "Relay",
		Type:            adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus:   adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:    adminplusdomain.SupplierHealthStatusNormal,
		APIBaseURL:      "https://relay.example.com",
		KeyLimitPolicy:  adminplusdomain.SupplierKeyLimitPolicyLimited,
		KeyLimitValue:   1,
		BalanceCurrency: "USD",
	})
	repo.PutGroup(&adminplusdomain.SupplierGroup{
		ID: 10, SupplierID: 7, ExternalGroupID: "g10", Name: "Existing",
		ProviderFamily: "openai", EffectiveRateMultiplier: 0.4,
		Status: adminplusdomain.SupplierGroupStatusActive,
	})
	keyAdapter := &stubKeyAdapter{
		listResult: &ports.ListProviderKeysResult{
			SupplierID: 7,
			Keys: []ports.ProviderKeySnapshot{{
				SupplierID: 7, ExternalGroupID: "g10", ExternalKeyID: "provider-g10",
				Name: "provider existing", Status: "active",
			}},
		},
		readResult: &ports.ProviderKeyResult{
			SupplierID: 7, ExternalGroupID: "g10", ExternalKeyID: "provider-g10",
			Name: "provider existing", Status: "active", Secret: "sk-provider-existing",
		},
	}
	local := &stubLocalAccountCreator{}
	svc := NewService(repo, &stubSessionReader{input: ports.SessionProbeInput{SupplierID: 7}}, keyAdapter, local)

	result, err := svc.EnsureAll(context.Background(), EnsureAllInput{
		SupplierID: 7, LocalAccountBaseURL: "https://relay.example.com/v1",
		RuntimeStatus: adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:  adminplusdomain.SupplierHealthStatusNormal, BalanceCurrency: "USD",
	})

	require.NoError(t, err)
	require.Equal(t, 1, result.Reused)
	require.Len(t, result.Items, 1)
	require.Equal(t, "reused", result.Items[0].Action)
	require.Equal(t, "provider-g10", result.Items[0].Key.ExternalKeyID)
	require.Equal(t, "sk-provider-existing", local.input.Credentials["api_key"])
	require.Empty(t, keyAdapter.calls)
	require.NotEmpty(t, keyAdapter.readCalls)
}

func TestServiceImportProviderProjectionCreatesManualSecretRequiredKey(t *testing.T) {
	repo := NewMemoryRepository()
	repo.PutSupplier(&adminplusdomain.Supplier{
		ID:              7,
		Name:            "Relay",
		Type:            adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus:   adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:    adminplusdomain.SupplierHealthStatusNormal,
		APIBaseURL:      "https://relay.example.com",
		KeyLimitPolicy:  adminplusdomain.SupplierKeyLimitPolicyLimited,
		KeyLimitValue:   10,
		BalanceCurrency: "USD",
	})
	repo.PutGroup(&adminplusdomain.SupplierGroup{
		ID:                      10,
		SupplierID:              7,
		ExternalGroupID:         "g10",
		Name:                    "Existing",
		ProviderFamily:          "openai",
		EffectiveRateMultiplier: 0.4,
		Status:                  adminplusdomain.SupplierGroupStatusActive,
	})
	keyAdapter := &stubKeyAdapter{
		listResult: &ports.ListProviderKeysResult{
			SupplierID: 7,
			Keys: []ports.ProviderKeySnapshot{
				{
					SupplierID:      7,
					ExternalGroupID: "g10",
					ExternalKeyID:   "provider-g10",
					Name:            "provider imported",
					Status:          "active",
					RawPayload:      map[string]any{"id": "provider-g10", "name": "provider imported", "key": "****cret"},
				},
			},
		},
	}
	svc := NewService(repo, &stubSessionReader{input: ports.SessionProbeInput{SupplierID: 7}}, keyAdapter, &stubLocalAccountCreator{})

	imported, err := svc.ImportProviderProjection(context.Background(), ImportProviderProjectionInput{
		SupplierID:      7,
		SupplierGroupID: 10,
		ExternalKeyID:   "provider-g10",
	})

	require.NoError(t, err)
	require.NotNil(t, imported)
	require.Equal(t, adminplusdomain.SupplierKeyStatusManualSecretRequired, imported.Status)
	require.Equal(t, "provider-g10", imported.ExternalKeyID)
	require.Equal(t, "provider imported", imported.Name)
	require.Empty(t, imported.KeyFingerprint)
	require.Empty(t, imported.KeyLast4)
	require.Equal(t, "SUPPLIER_KEY_SECRET_REQUIRED", imported.ErrorCode)
	require.Equal(t, "provider_key_list_import", imported.ProvisionRequest["source"])
	require.Equal(t, "[redacted]", imported.ProvisionResponse["key"])

	replayed, err := svc.ImportProviderProjection(context.Background(), ImportProviderProjectionInput{
		SupplierID:      7,
		SupplierGroupID: 10,
		ExternalKeyID:   "provider-g10",
	})
	require.NoError(t, err)
	require.Equal(t, imported.ID, replayed.ID)
	require.Len(t, keyAdapter.capacityCalls, 1)
}

func TestServiceImportProviderProjectionsCreatesBatchManualSecretRequiredKeys(t *testing.T) {
	repo := NewMemoryRepository()
	repo.PutSupplier(&adminplusdomain.Supplier{
		ID:              7,
		Name:            "Relay",
		Type:            adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus:   adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:    adminplusdomain.SupplierHealthStatusNormal,
		APIBaseURL:      "https://relay.example.com",
		KeyLimitPolicy:  adminplusdomain.SupplierKeyLimitPolicyLimited,
		KeyLimitValue:   10,
		BalanceCurrency: "USD",
	})
	for _, group := range []*adminplusdomain.SupplierGroup{
		{
			ID:                      10,
			SupplierID:              7,
			ExternalGroupID:         "g10",
			Name:                    "Existing 10",
			ProviderFamily:          "openai",
			EffectiveRateMultiplier: 0.4,
			Status:                  adminplusdomain.SupplierGroupStatusActive,
		},
		{
			ID:                      20,
			SupplierID:              7,
			ExternalGroupID:         "g20",
			Name:                    "Existing 20",
			ProviderFamily:          "openai",
			EffectiveRateMultiplier: 0.3,
			Status:                  adminplusdomain.SupplierGroupStatusActive,
		},
		{
			ID:                      30,
			SupplierID:              7,
			ExternalGroupID:         "g30",
			Name:                    "Missing Provider Key",
			ProviderFamily:          "openai",
			EffectiveRateMultiplier: 0.2,
			Status:                  adminplusdomain.SupplierGroupStatusActive,
		},
	} {
		repo.PutGroup(group)
	}
	keyAdapter := &stubKeyAdapter{
		listResult: &ports.ListProviderKeysResult{
			SupplierID: 7,
			Keys: []ports.ProviderKeySnapshot{
				{
					SupplierID:      7,
					ExternalGroupID: "g10",
					ExternalKeyID:   "provider-g10",
					Name:            "provider imported 10",
					Status:          "active",
					RawPayload:      map[string]any{"id": "provider-g10", "key": "secret-10"},
				},
				{
					SupplierID:      7,
					ExternalGroupID: "g20",
					ExternalKeyID:   "provider-g20",
					Name:            "provider imported 20",
					Status:          "active",
					RawPayload:      map[string]any{"id": "provider-g20", "api_key": "secret-20"},
				},
			},
		},
	}
	svc := NewService(repo, &stubSessionReader{input: ports.SessionProbeInput{SupplierID: 7}}, keyAdapter, &stubLocalAccountCreator{})

	result, err := svc.ImportProviderProjections(context.Background(), ImportProviderProjectionsInput{
		SupplierID: 7,
		Items: []ImportProviderProjectionInput{
			{SupplierGroupID: 10, ExternalKeyID: "provider-g10"},
			{SupplierGroupID: 20, ExternalKeyID: "provider-g20"},
			{SupplierGroupID: 30, ExternalKeyID: "provider-g30"},
		},
	})

	require.NoError(t, err)
	require.Equal(t, int64(7), result.SupplierID)
	require.Equal(t, 3, result.Total)
	require.Equal(t, 2, result.Imported)
	require.Equal(t, 0, result.Skipped)
	require.Equal(t, 1, result.Failed)
	require.Len(t, result.Items, 3)
	require.Equal(t, "imported", result.Items[0].Action)
	require.Equal(t, adminplusdomain.SupplierKeyStatusManualSecretRequired, result.Items[0].Key.Status)
	require.Equal(t, "[redacted]", result.Items[0].Key.ProvisionResponse["key"])
	require.Equal(t, "imported", result.Items[1].Action)
	require.Equal(t, "[redacted]", result.Items[1].Key.ProvisionResponse["api_key"])
	require.Equal(t, "failed", result.Items[2].Action)
	require.Equal(t, "SUPPLIER_PROVIDER_KEY_NOT_FOUND", result.Items[2].ErrorCode)
	require.Len(t, keyAdapter.capacityCalls, 1)

	replayed, err := svc.ImportProviderProjections(context.Background(), ImportProviderProjectionsInput{
		SupplierID: 7,
		Items: []ImportProviderProjectionInput{
			{SupplierGroupID: 10, ExternalKeyID: "provider-g10"},
			{SupplierGroupID: 20, ExternalKeyID: "provider-g20"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, 0, replayed.Imported)
	require.Equal(t, 2, replayed.Skipped)
	require.Equal(t, "skipped_existing", replayed.Items[0].Action)
	require.Equal(t, result.Items[0].Key.ID, replayed.Items[0].Key.ID)
	require.Len(t, keyAdapter.capacityCalls, 1)
}

func TestServicePlanEnsureAllBlocksWhenProviderKeyCapacityIncomplete(t *testing.T) {
	repo := NewMemoryRepository()
	repo.PutSupplier(&adminplusdomain.Supplier{
		ID:              7,
		Name:            "Relay",
		Type:            adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus:   adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:    adminplusdomain.SupplierHealthStatusNormal,
		APIBaseURL:      "https://relay.example.com",
		KeyLimitPolicy:  adminplusdomain.SupplierKeyLimitPolicyUnlimited,
		BalanceCurrency: "USD",
	})
	for _, group := range []*adminplusdomain.SupplierGroup{
		{ID: 10, SupplierID: 7, ExternalGroupID: "g10", Name: "Low", ProviderFamily: "openai", EffectiveRateMultiplier: 0.1, Status: adminplusdomain.SupplierGroupStatusActive},
		{ID: 20, SupplierID: 7, ExternalGroupID: "g20", Name: "Mid", ProviderFamily: "openai", EffectiveRateMultiplier: 0.2, Status: adminplusdomain.SupplierGroupStatusActive},
	} {
		repo.PutGroup(group)
	}
	keyAdapter := &stubKeyAdapter{
		capacityResult: &ports.ProviderKeyCapacityResult{
			SupplierID:     7,
			SystemType:     "sub2api",
			KeyLimitPolicy: "unknown",
			LimitKnown:     false,
			Diagnostics:    map[string]any{"truncated": true, "truncated_reason": "empty_page"},
		},
	}
	svc := NewService(repo, &stubSessionReader{input: ports.SessionProbeInput{SupplierID: 7}}, keyAdapter, &stubLocalAccountCreator{})
	input := EnsureAllInput{
		SupplierID:          7,
		LocalAccountBaseURL: "https://relay.example.com/v1",
		RuntimeStatus:       adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:        adminplusdomain.SupplierHealthStatusNormal,
		BalanceCurrency:     "USD",
	}

	plan, err := svc.PlanEnsureAll(context.Background(), input)

	require.NoError(t, err)
	require.Equal(t, 2, plan.Blocked)
	require.Equal(t, 0, plan.ToCreate)
	require.Equal(t, "provider_key_capacity_incomplete", supplierPlanItem(plan, 10).BlockedReason)
	require.Equal(t, "provider_key_capacity_incomplete", supplierPlanItem(plan, 20).BlockedReason)
	err = ensureAllPlanCanApply(plan, true)
	require.Error(t, err)
	require.Equal(t, "SUPPLIER_PROVIDER_KEY_CAPACITY_INCOMPLETE", infraerrors.Reason(err))

	result, err := svc.EnsureAll(context.Background(), input)
	require.Nil(t, result)
	require.Error(t, err)
	require.Equal(t, "SUPPLIER_PROVIDER_KEY_CAPACITY_INCOMPLETE", infraerrors.Reason(err))
	require.Len(t, keyAdapter.capacityCalls, 2)
	require.Empty(t, keyAdapter.calls)
}

func TestServiceProvisionRequiresSecretForProviderActiveKeyWithoutLocalBinding(t *testing.T) {
	repo := NewMemoryRepository()
	repo.PutSupplier(&adminplusdomain.Supplier{
		ID:              7,
		Name:            "Relay",
		Type:            adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus:   adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:    adminplusdomain.SupplierHealthStatusNormal,
		APIBaseURL:      "https://relay.example.com",
		KeyLimitPolicy:  adminplusdomain.SupplierKeyLimitPolicyUnlimited,
		BalanceCurrency: "USD",
	})
	repo.PutGroup(&adminplusdomain.SupplierGroup{
		ID:              10,
		SupplierID:      7,
		ExternalGroupID: "g10",
		Name:            "Low Cost",
		ProviderFamily:  "openai",
		Status:          adminplusdomain.SupplierGroupStatusActive,
	})
	keyAdapter := &stubKeyAdapter{
		result: &ports.ProviderKeyResult{
			SupplierID:      7,
			ExternalGroupID: "g10",
			ExternalKeyID:   "created-duplicate",
			Name:            "duplicate",
			Secret:          "sk-provider-secret",
		},
		listResult: &ports.ListProviderKeysResult{
			SupplierID: 7,
			Keys: []ports.ProviderKeySnapshot{
				{
					SupplierID:      7,
					ExternalGroupID: "g10",
					ExternalKeyID:   "provider-existing",
					Name:            "provider existing",
					Status:          "active",
				},
			},
		},
	}
	svc := NewService(repo, &stubSessionReader{input: ports.SessionProbeInput{SupplierID: 7}}, keyAdapter, &stubLocalAccountCreator{})

	result, err := svc.Provision(context.Background(), ProvisionKeyInput{
		SupplierID:              7,
		SupplierGroupID:         10,
		Name:                    "duplicate",
		LocalAccountPlatform:    service.PlatformOpenAI,
		LocalAccountName:        "local-upstream",
		LocalAccountBaseURL:     "https://relay.example.com/v1",
		LocalAccountConcurrency: 1,
		RuntimeStatus:           adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:            adminplusdomain.SupplierHealthStatusNormal,
		BalanceCurrency:         "USD",
	})

	require.Nil(t, result)
	require.Error(t, err)
	require.Equal(t, "SUPPLIER_KEY_SECRET_REQUIRED", infraerrors.Reason(err))
	require.Len(t, keyAdapter.capacityCalls, 1)
	require.Empty(t, keyAdapter.calls)
}

func TestServiceProvisionRejectsIncompleteProviderKeyCapacityBeforeCreate(t *testing.T) {
	repo := NewMemoryRepository()
	repo.PutSupplier(&adminplusdomain.Supplier{
		ID:              7,
		Name:            "Relay",
		Type:            adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus:   adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:    adminplusdomain.SupplierHealthStatusNormal,
		APIBaseURL:      "https://relay.example.com",
		KeyLimitPolicy:  adminplusdomain.SupplierKeyLimitPolicyUnlimited,
		BalanceCurrency: "USD",
	})
	repo.PutGroup(&adminplusdomain.SupplierGroup{
		ID:              10,
		SupplierID:      7,
		ExternalGroupID: "g10",
		Name:            "Low Cost",
		ProviderFamily:  "openai",
		Status:          adminplusdomain.SupplierGroupStatusActive,
	})
	keyAdapter := &stubKeyAdapter{
		result: &ports.ProviderKeyResult{
			SupplierID:      7,
			ExternalGroupID: "g10",
			ExternalKeyID:   "created-duplicate",
			Name:            "duplicate",
			Secret:          "sk-provider-secret",
		},
		capacityResult: &ports.ProviderKeyCapacityResult{
			SupplierID:     7,
			SystemType:     "sub2api",
			KeyLimitPolicy: "unknown",
			LimitKnown:     false,
			Diagnostics:    map[string]any{"truncated": true, "truncated_reason": "empty_page"},
		},
	}
	svc := NewService(repo, &stubSessionReader{input: ports.SessionProbeInput{SupplierID: 7}}, keyAdapter, &stubLocalAccountCreator{})

	result, err := svc.Provision(context.Background(), ProvisionKeyInput{
		SupplierID:              7,
		SupplierGroupID:         10,
		Name:                    "duplicate",
		LocalAccountPlatform:    service.PlatformOpenAI,
		LocalAccountName:        "local-upstream",
		LocalAccountBaseURL:     "https://relay.example.com/v1",
		LocalAccountConcurrency: 1,
		RuntimeStatus:           adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:            adminplusdomain.SupplierHealthStatusNormal,
		BalanceCurrency:         "USD",
	})

	require.Nil(t, result)
	require.Error(t, err)
	require.Equal(t, "SUPPLIER_PROVIDER_KEY_CAPACITY_INCOMPLETE", infraerrors.Reason(err))
	require.Len(t, keyAdapter.capacityCalls, 1)
	require.Empty(t, keyAdapter.calls)
}

func TestServiceDisableProviderKeyCallsProviderBeforeLocalProjection(t *testing.T) {
	repo := NewMemoryRepository()
	repo.PutSupplier(&adminplusdomain.Supplier{
		ID:              7,
		Name:            "Relay",
		Type:            adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus:   adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:    adminplusdomain.SupplierHealthStatusNormal,
		BalanceCurrency: "USD",
	})
	key, err := repo.CreateKey(context.Background(), &adminplusdomain.SupplierKey{
		SupplierID:            7,
		SupplierGroupID:       10,
		ExternalGroupID:       "g10",
		ExternalKeyID:         "99",
		Name:                  "provider-key",
		Status:                adminplusdomain.SupplierKeyStatusBound,
		ProviderFamily:        "openai",
		LocalSub2APIAccountID: 1001,
		LocalAccountName:      "local-account",
		LocalAccountPlatform:  service.PlatformOpenAI,
		LocalAccountType:      service.AccountTypeAPIKey,
		CreatedAt:             time.Now().UTC(),
		UpdatedAt:             time.Now().UTC(),
	})
	require.NoError(t, err)
	keyAdapter := &stubKeyAdapter{}
	svc := NewService(repo, &stubSessionReader{input: ports.SessionProbeInput{SupplierID: 7}}, keyAdapter, &stubLocalAccountCreator{})

	disabled, err := svc.DisableProviderKey(context.Background(), DisableProviderKeyInput{
		SupplierID: 7,
		KeyID:      key.ID,
		Reason:     "provider quota exhausted",
	})

	require.NoError(t, err)
	require.Equal(t, adminplusdomain.SupplierKeyStatusDisabled, disabled.Status)
	require.Equal(t, "PROVIDER_KEY_DISABLED", disabled.ErrorCode)
	require.Equal(t, "provider quota exhausted", disabled.ErrorMessage)
	require.Equal(t, int64(1001), disabled.LocalSub2APIAccountID)
	require.Equal(t, "local-account", disabled.LocalAccountName)
	require.Len(t, keyAdapter.disableCalls, 1)
	require.Equal(t, "99", keyAdapter.disableCalls[0].ExternalKeyID)
	require.Equal(t, "g10", keyAdapter.disableCalls[0].ExternalGroupID)
	require.Equal(t, key.ID, keyAdapter.disableCalls[0].Metadata["supplier_key_id"])
}

func TestServiceDeleteProviderKeyFailureDoesNotReleaseLocalProjection(t *testing.T) {
	repo := NewMemoryRepository()
	repo.PutSupplier(&adminplusdomain.Supplier{
		ID:              7,
		Name:            "Relay",
		Type:            adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus:   adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:    adminplusdomain.SupplierHealthStatusNormal,
		BalanceCurrency: "USD",
	})
	key, err := repo.CreateKey(context.Background(), &adminplusdomain.SupplierKey{
		SupplierID:      7,
		SupplierGroupID: 10,
		ExternalGroupID: "g10",
		ExternalKeyID:   "99",
		Name:            "provider-key",
		Status:          adminplusdomain.SupplierKeyStatusBound,
		ProviderFamily:  "openai",
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	})
	require.NoError(t, err)
	keyAdapter := &stubKeyAdapter{err: infraerrors.New(http.StatusBadGateway, "PROVIDER_DELETE_FAILED", "provider delete failed")}
	svc := NewService(repo, &stubSessionReader{input: ports.SessionProbeInput{SupplierID: 7}}, keyAdapter, &stubLocalAccountCreator{})

	disabled, err := svc.DeleteProviderKey(context.Background(), DeleteProviderKeyInput{
		SupplierID: 7,
		KeyID:      key.ID,
		Reason:     "delete upstream",
	})

	require.Nil(t, disabled)
	require.Error(t, err)
	require.Equal(t, "PROVIDER_DELETE_FAILED", infraerrors.Reason(err))
	require.Len(t, keyAdapter.deleteCalls, 1)
	current, err := repo.GetKey(context.Background(), 7, key.ID)
	require.NoError(t, err)
	require.Equal(t, adminplusdomain.SupplierKeyStatusBound, current.Status)
	require.Empty(t, current.ErrorCode)
}

func TestServiceEnsureAllRejectsUnknownCapacityForMissingGroups(t *testing.T) {
	repo := NewMemoryRepository()
	repo.PutSupplier(&adminplusdomain.Supplier{
		ID:              7,
		Name:            "Relay",
		Type:            adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus:   adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:    adminplusdomain.SupplierHealthStatusNormal,
		APIBaseURL:      "https://relay.example.com",
		KeyLimitPolicy:  adminplusdomain.SupplierKeyLimitPolicyUnknown,
		BalanceCurrency: "USD",
	})
	repo.PutGroup(&adminplusdomain.SupplierGroup{
		ID:              20,
		SupplierID:      7,
		ExternalGroupID: "g20",
		Name:            "Lowest",
		ProviderFamily:  "openai",
		Status:          adminplusdomain.SupplierGroupStatusActive,
	})
	keyAdapter := &stubKeyAdapter{
		result: &ports.ProviderKeyResult{
			SupplierID:    7,
			ExternalKeyID: "should-not-be-created",
			Secret:        "sk-provider-secret",
		},
	}
	svc := NewService(repo, &stubSessionReader{}, keyAdapter, &stubLocalAccountCreator{})

	result, err := svc.EnsureAll(context.Background(), EnsureAllInput{
		SupplierID:          7,
		LocalAccountBaseURL: "https://relay.example.com/v1",
		RuntimeStatus:       adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:        adminplusdomain.SupplierHealthStatusNormal,
		BalanceCurrency:     "USD",
	})

	require.Nil(t, result)
	require.Error(t, err)
	require.Contains(t, err.Error(), "SUPPLIER_KEY_CAPACITY_UNKNOWN")
	require.Empty(t, keyAdapter.calls)
}

func TestServiceEnsureAllAllowsExplicitPartialCapacityPlan(t *testing.T) {
	repo := NewMemoryRepository()
	repo.PutSupplier(&adminplusdomain.Supplier{
		ID:              7,
		Name:            "Relay",
		Type:            adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus:   adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:    adminplusdomain.SupplierHealthStatusNormal,
		APIBaseURL:      "https://relay.example.com",
		KeyLimitPolicy:  adminplusdomain.SupplierKeyLimitPolicyLimited,
		KeyLimitValue:   1,
		BalanceCurrency: "USD",
	})
	repo.PutGroup(&adminplusdomain.SupplierGroup{
		ID:                      20,
		SupplierID:              7,
		ExternalGroupID:         "g20",
		Name:                    "Lowest",
		ProviderFamily:          "openai",
		EffectiveRateMultiplier: 0.1,
		Status:                  adminplusdomain.SupplierGroupStatusActive,
	})
	repo.PutGroup(&adminplusdomain.SupplierGroup{
		ID:                      30,
		SupplierID:              7,
		ExternalGroupID:         "g30",
		Name:                    "High",
		ProviderFamily:          "openai",
		EffectiveRateMultiplier: 0.3,
		Status:                  adminplusdomain.SupplierGroupStatusActive,
	})
	keyAdapter := &stubKeyAdapter{
		result: &ports.ProviderKeyResult{
			SupplierID:      7,
			ExternalGroupID: "g20",
			ExternalKeyID:   "created-lowest",
			Name:            "created-lowest",
			Secret:          "sk-provider-secret",
		},
	}
	svc := NewService(repo, &stubSessionReader{}, keyAdapter, &stubLocalAccountCreator{})

	result, err := svc.EnsureAll(context.Background(), EnsureAllInput{
		SupplierID:          7,
		AllowPartial:        true,
		LocalAccountBaseURL: "https://relay.example.com/v1",
		RuntimeStatus:       adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:        adminplusdomain.SupplierHealthStatusNormal,
		BalanceCurrency:     "USD",
	})

	require.NoError(t, err)
	require.Equal(t, 1, result.Total)
	require.Equal(t, 1, result.Created)
	require.Len(t, result.Items, 1)
	require.Equal(t, int64(20), result.Items[0].SupplierGroupID)
	require.Len(t, keyAdapter.calls, 1)
	require.Equal(t, "g20", keyAdapter.calls[0].ExternalGroupID)
}

func TestServiceRepairBindingBindsFailedKeyToExistingLocalAccount(t *testing.T) {
	repo := NewMemoryRepository()
	repo.PutSupplier(&adminplusdomain.Supplier{
		ID:            7,
		Name:          "Relay",
		Type:          adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus: adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:  adminplusdomain.SupplierHealthStatusNormal,
	})
	repo.PutGroup(&adminplusdomain.SupplierGroup{
		ID:              10,
		SupplierID:      7,
		ExternalGroupID: "88",
		Name:            "Low Cost",
		ProviderFamily:  "openai",
		Status:          adminplusdomain.SupplierGroupStatusActive,
	})
	key, err := repo.CreateKey(context.Background(), &adminplusdomain.SupplierKey{
		SupplierID:      7,
		SupplierGroupID: 10,
		ExternalGroupID: "88",
		ExternalKeyID:   "99",
		Name:            "failed-key",
		KeyFingerprint:  "fingerprint",
		KeyLast4:        "cret",
		Status:          adminplusdomain.SupplierKeyStatusProvisioning,
		ProviderFamily:  "openai",
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	})
	require.NoError(t, err)
	_, err = repo.UpdateKeyAfterLocalBind(context.Background(), key.ID, nil, adminplusdomain.SupplierKeyStatusFailed, "LOCAL_ACCOUNT_CREATE_FAILED", "local account unavailable")
	require.NoError(t, err)

	local := &stubLocalAccountCreator{
		accounts: map[int64]*service.Account{
			2002: {
				ID:       2002,
				Name:     "manual-local",
				Platform: service.PlatformOpenAI,
				Type:     service.AccountTypeAPIKey,
			},
		},
	}
	keyAdapter := &stubKeyAdapter{
		result: &ports.ProviderKeyResult{
			SupplierID:    7,
			ExternalKeyID: "should-not-be-called",
			Secret:        "sk-provider-secret",
		},
	}
	svc := NewService(repo, &stubSessionReader{}, keyAdapter, local)

	result, err := svc.RepairBinding(context.Background(), RepairBindingInput{
		SupplierID:            7,
		KeyID:                 key.ID,
		LocalSub2APIAccountID: 2002,
		RuntimeStatus:         adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:          adminplusdomain.SupplierHealthStatusNormal,
		BalanceCurrency:       "USD",
	})

	require.NoError(t, err)
	require.NotNil(t, result.Key)
	require.NotNil(t, result.Binding)
	require.Equal(t, adminplusdomain.SupplierKeyStatusBound, result.Key.Status)
	require.Equal(t, int64(2002), result.Key.LocalSub2APIAccountID)
	require.Equal(t, "manual-local", result.Key.LocalAccountName)
	require.Equal(t, int64(2002), result.Binding.LocalSub2APIAccountID)
	require.Equal(t, "99", result.Binding.SupplierAccountIdentifier)
	require.Equal(t, "failed-key", result.Binding.SupplierAccountLabel)
	require.Empty(t, result.Key.ErrorCode)
	require.Empty(t, keyAdapter.calls)
	require.Equal(t, []int64{2002}, local.getCalls)
	require.Empty(t, local.accounts[2002].GroupIDs)
}

func TestServiceRepairBindingCompletesManualSecretRequiredKey(t *testing.T) {
	repo := NewMemoryRepository()
	repo.PutSupplier(&adminplusdomain.Supplier{
		ID:            7,
		Name:          "Relay",
		Type:          adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus: adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:  adminplusdomain.SupplierHealthStatusNormal,
	})
	repo.PutGroup(&adminplusdomain.SupplierGroup{
		ID:                      10,
		SupplierID:              7,
		ExternalGroupID:         "88",
		Name:                    "Low Cost",
		ProviderFamily:          "openai",
		EffectiveRateMultiplier: 0.5,
		Status:                  adminplusdomain.SupplierGroupStatusActive,
	})
	key, err := repo.CreateKey(context.Background(), &adminplusdomain.SupplierKey{
		SupplierID:      7,
		SupplierGroupID: 10,
		ExternalGroupID: "88",
		ExternalKeyID:   "manual-99",
		Name:            "manual-key",
		Status:          adminplusdomain.SupplierKeyStatusManualSecretRequired,
		ProviderFamily:  "openai",
		ErrorCode:       "SUPPLIER_KEY_SECRET_REQUIRED",
		ErrorMessage:    "provider key list does not expose secret",
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	})
	require.NoError(t, err)
	local := &stubLocalAccountCreator{}
	keyAdapter := &stubKeyAdapter{
		result: &ports.ProviderKeyResult{
			SupplierID:    7,
			ExternalKeyID: "should-not-be-called",
			Secret:        "sk-provider-secret",
		},
	}
	svc := NewService(repo, &stubSessionReader{}, keyAdapter, local)
	rate := 0.5

	result, err := svc.RepairBinding(context.Background(), RepairBindingInput{
		SupplierID:                 7,
		KeyID:                      key.ID,
		ManualSecret:               "sk-manual-secret",
		LocalAccountPlatform:       service.PlatformOpenAI,
		LocalAccountName:           "relay-low-cost",
		LocalAccountBaseURL:        "https://relay.example.com/v1",
		LocalAccountPriority:       80,
		LocalAccountRateMultiplier: &rate,
		RuntimeStatus:              adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:               adminplusdomain.SupplierHealthStatusNormal,
		ConfiguredConcurrency:      2,
		BalanceCurrency:            "USD",
	})

	require.NoError(t, err)
	require.NotNil(t, result.Key)
	require.NotNil(t, result.Binding)
	require.Equal(t, adminplusdomain.SupplierKeyStatusBound, result.Key.Status)
	require.Equal(t, int64(1001), result.Key.LocalSub2APIAccountID)
	require.Equal(t, "relay-low-cost [88]", result.Key.LocalAccountName)
	require.Equal(t, "cret", result.Key.KeyLast4)
	require.NotEmpty(t, result.Key.KeyFingerprint)
	require.NotEqual(t, "sk-manual-secret", result.Key.KeyFingerprint)
	require.Empty(t, result.Key.ErrorCode)
	require.Equal(t, int64(1001), result.Binding.LocalSub2APIAccountID)
	require.Equal(t, "manual-99", result.Binding.SupplierAccountIdentifier)
	require.Equal(t, "manual-key", result.Binding.SupplierAccountLabel)
	require.Equal(t, service.PlatformOpenAI, local.input.Platform)
	require.Equal(t, service.AccountTypeAPIKey, local.input.Type)
	require.Equal(t, "sk-manual-secret", local.input.Credentials["api_key"])
	require.Equal(t, "https://relay.example.com/v1", local.input.Credentials["base_url"])
	require.Empty(t, keyAdapter.calls)
	require.Empty(t, local.getCalls)
}

type stubSessionReader struct {
	input ports.SessionProbeInput
}

func (s *stubSessionReader) DecryptedProbeInput(_ context.Context, _ int64) (ports.SessionProbeInput, error) {
	return s.input, nil
}

type stubKeyAdapter struct {
	result         *ports.ProviderKeyResult
	resultsByGroup map[string]*ports.ProviderKeyResult
	readResult     *ports.ProviderKeyResult
	listResult     *ports.ListProviderKeysResult
	capacityResult *ports.ProviderKeyCapacityResult
	err            error
	readErr        error
	listErr        error
	capacityErr    error
	calls          []ports.CreateProviderKeyInput
	readCalls      []ports.ReadProviderKeyInput
	listCalls      []ports.ListProviderKeysInput
	capacityCalls  []ports.ReadProviderKeyCapacityInput
	renameCalls    []ports.RenameProviderKeyInput
	disableCalls   []ports.DisableProviderKeyInput
	deleteCalls    []ports.DeleteProviderKeyInput
}

func (s *stubKeyAdapter) ListKeys(_ context.Context, in ports.SessionProbeInput, request ports.ListProviderKeysInput) (*ports.ListProviderKeysResult, error) {
	s.listCalls = append(s.listCalls, request)
	if s.listErr != nil {
		return nil, s.listErr
	}
	if s.listResult != nil {
		return s.listResult, nil
	}
	return nil, nil
}

func (s *stubKeyAdapter) ReadKey(_ context.Context, _ ports.SessionProbeInput, request ports.ReadProviderKeyInput) (*ports.ProviderKeyResult, error) {
	s.readCalls = append(s.readCalls, request)
	if s.readErr != nil {
		return nil, s.readErr
	}
	return s.readResult, nil
}

func (s *stubKeyAdapter) ReadKeyCapacity(_ context.Context, in ports.SessionProbeInput, request ports.ReadProviderKeyCapacityInput) (*ports.ProviderKeyCapacityResult, error) {
	s.capacityCalls = append(s.capacityCalls, request)
	if s.capacityErr != nil {
		return nil, s.capacityErr
	}
	if s.capacityResult != nil {
		return s.capacityResult, nil
	}
	if s.listResult != nil {
		snapshot := providerKeyCapacitySnapshotFromList(s.listResult.Keys)
		return &ports.ProviderKeyCapacityResult{
			SupplierID:        in.SupplierID,
			SystemType:        s.listResult.SystemType,
			Origin:            s.listResult.Origin,
			APIBaseURL:        s.listResult.APIBaseURL,
			KeyLimitPolicy:    "unknown",
			KeyLimitValue:     0,
			ActiveKeyCount:    snapshot.ActiveKeyCount,
			RemainingKeySlots: 0,
			KeyCapacityStatus: "unknown",
			LimitKnown:        false,
			Keys:              append([]ports.ProviderKeySnapshot(nil), s.listResult.Keys...),
			Diagnostics:       map[string]any{"capacity_source": "test_list_result"},
			CapturedAt:        time.Now().UTC(),
		}, nil
	}
	return nil, nil
}

func (s *stubKeyAdapter) CreateKey(_ context.Context, _ ports.SessionProbeInput, request ports.CreateProviderKeyInput) (*ports.ProviderKeyResult, error) {
	s.calls = append(s.calls, request)
	if s.err != nil {
		return nil, s.err
	}
	if result := s.resultsByGroup[request.ExternalGroupID]; result != nil {
		copy := *result
		return &copy, nil
	}
	return s.result, nil
}

func (s *stubKeyAdapter) RenameKey(_ context.Context, _ ports.SessionProbeInput, request ports.RenameProviderKeyInput) (*ports.ProviderKeyResult, error) {
	s.renameCalls = append(s.renameCalls, request)
	if s.err != nil {
		return nil, s.err
	}
	return &ports.ProviderKeyResult{
		SupplierID:      request.SupplierID,
		ExternalGroupID: request.ExternalGroupID,
		ExternalKeyID:   request.ExternalKeyID,
		Name:            request.Name,
		Status:          "active",
		RawPayload:      map[string]any{"renamed": true},
	}, nil
}

func (s *stubKeyAdapter) DisableKey(_ context.Context, _ ports.SessionProbeInput, request ports.DisableProviderKeyInput) (*ports.ProviderKeyResult, error) {
	s.disableCalls = append(s.disableCalls, request)
	if s.err != nil {
		return nil, s.err
	}
	return &ports.ProviderKeyResult{
		SupplierID:      request.SupplierID,
		ExternalGroupID: request.ExternalGroupID,
		ExternalKeyID:   request.ExternalKeyID,
		Name:            request.Name,
		Status:          "disabled",
		RawPayload:      map[string]any{"disabled": true},
	}, nil
}

func (s *stubKeyAdapter) DeleteKey(_ context.Context, _ ports.SessionProbeInput, request ports.DeleteProviderKeyInput) (*ports.ProviderKeyResult, error) {
	s.deleteCalls = append(s.deleteCalls, request)
	if s.err != nil {
		return nil, s.err
	}
	return &ports.ProviderKeyResult{
		SupplierID:      request.SupplierID,
		ExternalGroupID: request.ExternalGroupID,
		ExternalKeyID:   request.ExternalKeyID,
		Name:            request.Name,
		Status:          "deleted",
		RawPayload:      map[string]any{"deleted": true},
	}, nil
}

type stubLocalAccountCreator struct {
	input             service.CreateAccountInput
	accounts          map[int64]*service.Account
	missingAccountIDs map[int64]bool
	getCalls          []int64
}

func (s *stubLocalAccountCreator) CreateAccount(_ context.Context, input *service.CreateAccountInput) (*service.Account, error) {
	s.input = *input
	if s.accounts == nil {
		s.accounts = make(map[int64]*service.Account)
	}
	accountID := int64(1001)
	for {
		if _, exists := s.accounts[accountID]; !exists {
			break
		}
		accountID++
	}
	account := &service.Account{
		ID:          accountID,
		Name:        input.Name,
		Platform:    input.Platform,
		Type:        input.Type,
		Credentials: input.Credentials,
		Extra:       input.Extra,
		GroupIDs:    append([]int64(nil), input.GroupIDs...),
	}
	s.accounts[account.ID] = account
	return account, nil
}

func (s *stubLocalAccountCreator) GetAccount(_ context.Context, id int64) (*service.Account, error) {
	s.getCalls = append(s.getCalls, id)
	if s.missingAccountIDs[id] {
		return nil, infraerrors.New(http.StatusNotFound, "LOCAL_SUB2API_ACCOUNT_NOT_FOUND", "local Sub2API account not found")
	}
	if account, ok := s.accounts[id]; ok {
		cp := *account
		return &cp, nil
	}
	return &service.Account{
		ID:       id,
		Name:     "local-upstream",
		Platform: service.PlatformOpenAI,
		Type:     service.AccountTypeAPIKey,
	}, nil
}

func (s *stubLocalAccountCreator) FindAccount(_ context.Context, input LocalAccountLookupInput) (*service.Account, error) {
	for _, account := range s.accounts {
		if localAccountMatchesLookup(account, input) {
			cp := *account
			return &cp, nil
		}
	}
	return nil, nil
}

func (s *stubLocalAccountCreator) UpdateAccount(_ context.Context, id int64, input *service.UpdateAccountInput) (*service.Account, error) {
	if s.accounts == nil {
		s.accounts = make(map[int64]*service.Account)
	}
	account, ok := s.accounts[id]
	if !ok {
		account = &service.Account{
			ID:       id,
			Name:     "local-upstream",
			Platform: service.PlatformOpenAI,
			Type:     service.AccountTypeAPIKey,
		}
		s.accounts[id] = account
	}
	if input.GroupIDs != nil {
		account.GroupIDs = append([]int64(nil), (*input.GroupIDs)...)
	}
	cp := *account
	return &cp, nil
}

func supplierPlanItem(plan *EnsureAllPlan, groupID int64) ProvisionGroupPlan {
	if plan == nil {
		return ProvisionGroupPlan{}
	}
	for _, item := range plan.Items {
		if item.SupplierGroupID == groupID {
			return item
		}
	}
	return ProvisionGroupPlan{}
}
