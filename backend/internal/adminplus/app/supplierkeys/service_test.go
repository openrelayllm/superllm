package supplierkeys

import (
	"context"
	"net/http"
	"testing"
	"time"

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

func TestServiceEnsureAllFailsBeforeProviderKeyWhenSub2APIGatewayUnavailable(t *testing.T) {
	repo := NewMemoryRepository()
	repo.PutSupplier(&adminplusdomain.Supplier{
		ID:            7,
		Name:          "Relay",
		Type:          adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus: adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:  adminplusdomain.SupplierHealthStatusNormal,
		APIBaseURL:    "https://relay.example.com",
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
		ID:            7,
		Name:          "Relay",
		Type:          adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus: adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:  adminplusdomain.SupplierHealthStatusNormal,
		APIBaseURL:    "https://relay.example.com",
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

type stubSessionReader struct {
	input ports.SessionProbeInput
}

func (s *stubSessionReader) DecryptedProbeInput(_ context.Context, _ int64) (ports.SessionProbeInput, error) {
	return s.input, nil
}

type stubKeyAdapter struct {
	result      *ports.ProviderKeyResult
	calls       []ports.CreateProviderKeyInput
	renameCalls []ports.RenameProviderKeyInput
}

func (s *stubKeyAdapter) CreateKey(_ context.Context, _ ports.SessionProbeInput, request ports.CreateProviderKeyInput) (*ports.ProviderKeyResult, error) {
	s.calls = append(s.calls, request)
	return s.result, nil
}

func (s *stubKeyAdapter) RenameKey(_ context.Context, _ ports.SessionProbeInput, request ports.RenameProviderKeyInput) (*ports.ProviderKeyResult, error) {
	s.renameCalls = append(s.renameCalls, request)
	return &ports.ProviderKeyResult{
		SupplierID:      request.SupplierID,
		ExternalGroupID: request.ExternalGroupID,
		ExternalKeyID:   request.ExternalKeyID,
		Name:            request.Name,
		Status:          "active",
		RawPayload:      map[string]any{"renamed": true},
	}, nil
}

type stubLocalAccountCreator struct {
	input    service.CreateAccountInput
	accounts map[int64]*service.Account
	getCalls []int64
}

func (s *stubLocalAccountCreator) CreateAccount(_ context.Context, input *service.CreateAccountInput) (*service.Account, error) {
	s.input = *input
	account := &service.Account{
		ID:          1001,
		Name:        input.Name,
		Platform:    input.Platform,
		Type:        input.Type,
		Credentials: input.Credentials,
		GroupIDs:    append([]int64(nil), input.GroupIDs...),
	}
	if s.accounts == nil {
		s.accounts = make(map[int64]*service.Account)
	}
	s.accounts[account.ID] = account
	return account, nil
}

func (s *stubLocalAccountCreator) GetAccount(_ context.Context, id int64) (*service.Account, error) {
	s.getCalls = append(s.getCalls, id)
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
