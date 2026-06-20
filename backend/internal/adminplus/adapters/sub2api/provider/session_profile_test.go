package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/adminplus/ports"
	"github.com/stretchr/testify/require"
)

func TestSessionProfileClientProbeSub2APIUserProfile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/api/v1/user/profile", r.URL.Path)
		require.Equal(t, "Bearer browser-access-token", r.Header.Get("Authorization"))
		require.Equal(t, "sid=abc; theme=dark", r.Header.Get("Cookie"))
		require.Equal(t, "https://relay.example.com", r.Header.Get("Origin"))
		require.Equal(t, "https://relay.example.com/dashboard", r.Header.Get("Referer"))
		require.Equal(t, "csrf-token", r.Header.Get("X-CSRF-Token"))
		require.Equal(t, "application/json", r.Header.Get("Accept"))

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": {
				"id": 42,
				"email": "ops@example.com",
				"username": "ops",
				"role": "user",
				"status": "enabled",
				"balance": 12.34,
				"concurrency": 8,
				"allowed_groups": [1, 2]
			}
		}`))
	}))
	defer server.Close()

	client := NewSessionProfileClient(server.Client())
	result, err := client.ProbeSub2APIUserProfile(context.Background(), ports.SessionProbeInput{
		SupplierID: 7,
		Origin:     "https://relay.example.com",
		APIBaseURL: server.URL,
		Bundle: map[string]any{
			"access_token": "browser-access-token",
			"csrf_token":   "csrf-token",
			"required_headers": map[string]any{
				"cookie":  "sid=abc; theme=dark",
				"origin":  "https://relay.example.com",
				"referer": "https://relay.example.com/dashboard",
			},
		},
	})

	require.NoError(t, err)
	require.Equal(t, int64(7), result.SupplierID)
	require.Equal(t, "valid", result.Status)
	require.Equal(t, "sub2api", result.SystemType)
	require.Equal(t, server.URL, result.APIBaseURL)
	require.NotNil(t, result.BalanceCents)
	require.Equal(t, int64(1234), *result.BalanceCents)
	require.Equal(t, "USD", result.BalanceCurrency)
	require.True(t, result.Capabilities["can_read_profile"])
	require.True(t, result.Capabilities["can_read_balance"])
	require.True(t, result.Capabilities["can_read_groups"])
	require.False(t, result.Capabilities["can_create_key"])
	require.False(t, result.Capabilities["can_read_billing"])
	require.Equal(t, int64(42), result.Profile.ID)
	require.Equal(t, "ops@example.com", result.Profile.Email)
	require.Equal(t, []int64{1, 2}, result.Profile.AllowedGroups)
}

func TestSessionProfileClientReadGroups(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer browser-access-token", r.Header.Get("Authorization"))
		require.Equal(t, "sid=abc", r.Header.Get("Cookie"))
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/groups/available":
			_, _ = w.Write([]byte(`{
				"data": [
					{
						"id": 10,
						"name": "GPT-5.5 Low Cost",
						"description": "cheap upstream pool",
						"platform": "openai",
						"rate_multiplier": 0.8,
						"rpm_limit": 120,
						"is_exclusive": true,
						"status": "active",
						"daily_limit_usd": 100,
						"allow_image_generation": true
					},
					{
						"id": 11,
						"name": "Claude",
						"platform": "anthropic",
						"rate_multiplier": 1.2,
						"status": "disabled"
					}
				]
			}`))
		case "/api/v1/groups/rates":
			_, _ = w.Write([]byte(`{"data":{"10":0.7}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewSessionProfileClient(server.Client())
	result, err := client.ReadGroups(context.Background(), ports.SessionProbeInput{
		SupplierID: 7,
		Origin:     server.URL,
		APIBaseURL: server.URL + "/api/v1",
		Bundle: map[string]any{
			"access_token": "browser-access-token",
			"required_headers": map[string]any{
				"cookie": "sid=abc",
			},
		},
	})

	require.NoError(t, err)
	require.Equal(t, int64(7), result.SupplierID)
	require.Equal(t, "sub2api", result.SystemType)
	require.Len(t, result.Groups, 2)
	require.Equal(t, "10", result.Groups[0].ExternalGroupID)
	require.Equal(t, "openai", result.Groups[0].ProviderFamily)
	require.Equal(t, 0.8, result.Groups[0].RateMultiplier)
	require.NotNil(t, result.Groups[0].UserRateMultiplier)
	require.Equal(t, 0.7, *result.Groups[0].UserRateMultiplier)
	require.Equal(t, 0.7, result.Groups[0].EffectiveRateMultiplier)
	require.NotNil(t, result.Groups[0].RPMLimit)
	require.Equal(t, int64(120), *result.Groups[0].RPMLimit)
	require.True(t, result.Groups[0].IsPrivate)
	require.True(t, result.Groups[0].AllowImageGeneration)
	require.Equal(t, "disabled", result.Groups[1].Status)
}
