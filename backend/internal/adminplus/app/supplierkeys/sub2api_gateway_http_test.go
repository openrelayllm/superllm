package supplierkeys

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestSub2APIHTTPGatewayCreatesAccountViaAdminAPI(t *testing.T) {
	var gotAPIKey string
	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/admin/accounts", r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)
		gotAPIKey = r.Header.Get("x-api-key")
		require.NoError(t, json.NewDecoder(r.Body).Decode(&gotPayload))
		writeSub2APISuccess(t, w, map[string]any{
			"id":         1001,
			"name":       "local-upstream",
			"platform":   "openai",
			"type":       "apikey",
			"group_ids":  []int64{2001},
			"extra":      map[string]any{"admin_plus_supplier_group_id": 10},
			"created_at": "2026-06-21T10:00:00Z",
			"updated_at": "2026-06-21T10:00:00Z",
		})
	}))
	defer server.Close()

	gateway, err := NewSub2APIHTTPGateway(server.URL, "admin-secret", server.Client())
	require.NoError(t, err)
	rate := 0.7
	account, err := gateway.CreateAccount(context.Background(), &service.CreateAccountInput{
		Name:                  "local-upstream",
		Platform:              service.PlatformOpenAI,
		Type:                  service.AccountTypeAPIKey,
		Credentials:           map[string]any{"api_key": "sk-provider-secret", "base_url": "https://supplier.example/v1"},
		Extra:                 map[string]any{"admin_plus_supplier_group_id": int64(10)},
		GroupIDs:              []int64{2001},
		RateMultiplier:        &rate,
		SkipMixedChannelCheck: true,
	})

	require.NoError(t, err)
	require.Equal(t, "admin-secret", gotAPIKey)
	require.Equal(t, "local-upstream", gotPayload["name"])
	require.Equal(t, nil, gotPayload["schedulable"])
	require.Equal(t, true, gotPayload["confirm_mixed_channel_risk"])
	require.Equal(t, int64(1001), account.ID)
	require.Equal(t, []int64{2001}, account.GroupIDs)
	require.Equal(t, int64(10), int64FromMap(account.Extra, "admin_plus_supplier_group_id"))
}

func TestSub2APIHTTPGatewayForwardsSchedulable(t *testing.T) {
	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&gotPayload))
		writeSub2APISuccess(t, w, map[string]any{
			"id":       1001,
			"name":     "local-upstream",
			"platform": "openai",
			"type":     "apikey",
		})
	}))
	defer server.Close()

	gateway, err := NewSub2APIHTTPGateway(server.URL, "admin-secret", server.Client())
	require.NoError(t, err)
	schedulable := false
	_, err = gateway.CreateAccount(context.Background(), &service.CreateAccountInput{
		Name:        "local-upstream",
		Platform:    service.PlatformOpenAI,
		Type:        service.AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "sk-provider-secret"},
		Schedulable: &schedulable,
	})

	require.NoError(t, err)
	require.Equal(t, false, gotPayload["schedulable"])
}

func TestSub2APIHTTPGatewayFindAccountUsesMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/admin/accounts", r.URL.Path)
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "openai", r.URL.Query().Get("platform"))
		writeSub2APISuccess(t, w, map[string]any{
			"items": []map[string]any{
				{
					"id":        1001,
					"name":      "unrelated",
					"platform":  "openai",
					"type":      "apikey",
					"group_ids": []int64{2001},
					"extra": map[string]any{
						"admin_plus_supplier_id":       7,
						"admin_plus_supplier_group_id": 10,
						"admin_plus_supplier_key":      99,
					},
				},
			},
		})
	}))
	defer server.Close()

	gateway, err := NewSub2APIHTTPGateway(server.URL, "admin-secret", server.Client())
	require.NoError(t, err)
	account, err := gateway.FindAccount(context.Background(), Sub2APIAccountLookupInput{
		SupplierID:           7,
		SupplierGroupID:      10,
		SupplierKeyID:        99,
		LocalAccountPlatform: service.PlatformOpenAI,
		LocalAccountName:     "expected-name",
	})

	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, int64(1001), account.ID)
	require.Equal(t, []int64{2001}, account.GroupIDs)
}

func TestShouldUseSub2APIHTTPGatewayFromEnvRequiresBothValues(t *testing.T) {
	t.Setenv(sub2APIAdminBaseURLEnv, "")
	t.Setenv(sub2APIAdminAPIKeyEnv, "")
	require.False(t, ShouldUseSub2APIHTTPGatewayFromEnv())
	t.Setenv(sub2APIAdminBaseURLEnv, "https://sub2api.example")
	require.False(t, ShouldUseSub2APIHTTPGatewayFromEnv())
	t.Setenv(sub2APIAdminAPIKeyEnv, "admin-secret")
	require.True(t, ShouldUseSub2APIHTTPGatewayFromEnv())
}

func TestShouldUseSub2APIHTTPGatewayFromConfigRequiresBothValues(t *testing.T) {
	require.False(t, ShouldUseSub2APIHTTPGatewayFromConfig(&config.Config{}))
	require.False(t, ShouldUseSub2APIHTTPGatewayFromConfig(&config.Config{
		AdminPlus: config.AdminPlusConfig{Sub2APIAdminBaseURL: "https://sub2api.example"},
	}))
	require.True(t, ShouldUseSub2APIHTTPGatewayFromConfig(&config.Config{
		AdminPlus: config.AdminPlusConfig{
			Sub2APIAdminBaseURL: "https://sub2api.example",
			Sub2APIAdminAPIKey:  "admin-secret",
		},
	}))
}

func TestNewSub2APIHTTPGatewayFromConfigUsesConfigValues(t *testing.T) {
	gateway, err := NewSub2APIHTTPGatewayFromConfig(&config.Config{
		AdminPlus: config.AdminPlusConfig{
			Sub2APIAdminBaseURL: "https://sub2api.example",
			Sub2APIAdminAPIKey:  "admin-secret",
		},
	}, nil)

	require.NoError(t, err)
	require.Equal(t, "https://sub2api.example", gateway.baseURL)
	require.Equal(t, "admin-secret", gateway.apiKey)
}

func writeSub2APISuccess(t *testing.T, w http.ResponseWriter, data any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
		"code":    0,
		"message": "success",
		"data":    data,
	}))
}
