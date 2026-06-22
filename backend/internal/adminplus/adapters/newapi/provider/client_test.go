package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/adminplus/ports"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestClientDirectLoginStoresCookieAndProbesSelfWithUserHeader(t *testing.T) {
	var sawLogin bool
	var sawSelf bool
	now := time.Date(2026, 6, 22, 9, 30, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/user/login":
			require.Equal(t, http.MethodPost, r.Method)
			require.Contains(t, r.Header.Get("User-Agent"), "Mozilla/5.0")
			var payload map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
			require.Equal(t, "ops@example.com", payload["username"])
			require.Equal(t, "secret", payload["password"])
			http.SetCookie(w, &http.Cookie{
				Name:     "session",
				Value:    "signed-session",
				Path:     "/",
				MaxAge:   2592000,
				HttpOnly: true,
			})
			sawLogin = true
			_, _ = w.Write([]byte(`{"success":true,"message":"","data":{"id":42,"username":"ops","display_name":"Ops","role":1,"status":1,"group":"default"}}`))
		case "/api/user/self":
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "42", r.Header.Get("New-Api-User"))
			require.Equal(t, "session=signed-session", r.Header.Get("Cookie"))
			sawSelf = true
			_, _ = w.Write([]byte(`{"success":true,"message":"","data":{"id":42,"username":"ops","display_name":"Ops","role":1,"status":1,"group":"default","quota":12345,"used_quota":67,"request_count":8}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(server.Client())
	client.now = func() time.Time { return now }
	result, err := client.DirectLogin(context.Background(), ports.DirectLoginInput{
		SupplierID: 7,
		Origin:     server.URL,
		APIBaseURL: server.URL,
		Username:   "ops@example.com",
		Password:   "secret",
	})

	require.NoError(t, err)
	require.True(t, sawLogin)
	require.True(t, sawSelf)
	require.Equal(t, int64(7), result.SupplierID)
	require.Equal(t, server.URL, result.Origin)
	require.Equal(t, server.URL, result.APIBaseURL)
	require.Equal(t, "new_api", result.SessionBundle["provider_type"])
	require.Equal(t, "direct_login", result.SessionBundle["session_source"])
	require.NotNil(t, result.ExpiresAt)
	require.Equal(t, "New-Api-User", result.SessionBundle["auth_header_name"])
	require.Equal(t, "42", result.SessionBundle["auth_header_value"])

	contextValue, ok := result.SessionBundle["context"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "new_api", contextValue["provider_type"])
	require.Equal(t, "42", contextValue["user_id"])

	cookies, ok := result.SessionBundle["cookies"].([]any)
	require.True(t, ok)
	require.Len(t, cookies, 1)
	cookie, ok := cookies[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "session", cookie["name"])
	require.Equal(t, "signed-session", cookie["value"])
}

func TestClientProbeSelfReturnsRawQuotaAsQTA(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/user/self", r.URL.Path)
		require.Equal(t, "9", r.Header.Get("New-Api-User"))
		require.Equal(t, "session=abc", r.Header.Get("Cookie"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":{"id":9,"username":"alice","role":1,"status":1,"quota":50,"used_quota":2,"request_count":3}}`))
	}))
	defer server.Close()

	client := NewClient(server.Client())
	result, err := client.ProbeSub2APIUserProfile(context.Background(), ports.SessionProbeInput{
		SupplierID: 7,
		Origin:     server.URL,
		APIBaseURL: server.URL,
		Bundle: map[string]any{
			"provider_type": "new_api",
			"cookies": []any{
				map[string]any{"name": "session", "value": "abc"},
			},
			"required_headers": map[string]any{"New-Api-User": "9"},
		},
	})

	require.NoError(t, err)
	require.Equal(t, "new_api", result.SystemType)
	require.Equal(t, "QTA", result.BalanceCurrency)
	require.NotNil(t, result.BalanceCents)
	require.Equal(t, int64(5000), *result.BalanceCents)
	require.Equal(t, float64(50), result.Profile.Balance)
	require.Equal(t, float64(2), result.Diagnostics["raw_used_quota"])
	require.Equal(t, int64(3), result.Diagnostics["request_count"])
}

func TestClientReadGroups(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/user/self/groups", r.URL.Path)
		require.Equal(t, "9", r.Header.Get("New-Api-User"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":{"default":{"ratio":1,"desc":"Default"},"vip":{"ratio":0.5,"desc":"VIP"}}}`))
	}))
	defer server.Close()

	client := NewClient(server.Client())
	result, err := client.ReadGroups(context.Background(), ports.SessionProbeInput{
		SupplierID: 7,
		Origin:     server.URL,
		APIBaseURL: server.URL,
		Bundle: map[string]any{
			"provider_type":     "new_api",
			"required_headers":  map[string]any{"New-Api-User": "9"},
			"auth_header_value": "9",
		},
	})

	require.NoError(t, err)
	require.Equal(t, "new_api", result.SystemType)
	require.Len(t, result.Groups, 2)
	require.ElementsMatch(t, []string{"default", "vip"}, []string{result.Groups[0].ExternalGroupID, result.Groups[1].ExternalGroupID})
}

func TestClientReadChannelMonitorsFromPulse(t *testing.T) {
	generatedAt := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/pulse", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"generated_ms": 1782122400000,
			"window_seconds": 60,
			"models": [
				{
					"model": "gpt-5.4-mini",
					"avg_ttft_ms": 1014,
					"avg_resp_sec": 13.7,
					"success_rate": 99.7,
					"req_count": 356,
					"health": "good"
				},
				{
					"model": "claude-opus-4",
					"avg_ttft_ms": 2500,
					"success_rate": 94.5,
					"req_count": 3,
					"health": "warn"
				}
			]
		}`))
	}))
	defer server.Close()

	client := NewClient(server.Client())
	result, err := client.ReadChannelMonitors(context.Background(), ports.SessionProbeInput{
		SupplierID: 7,
		Origin:     "https://newapi.example.com",
		APIBaseURL: "https://newapi.example.com",
		Bundle: map[string]any{
			"provider_type": "new_api",
			"context": map[string]any{
				"pulse_url": server.URL,
			},
		},
	})

	require.NoError(t, err)
	require.Equal(t, "new_api", result.SystemType)
	require.Equal(t, server.URL+"/api/pulse", result.APIBaseURL)
	require.Equal(t, generatedAt, result.CapturedAt)
	require.Len(t, result.Items, 2)
	require.Equal(t, "gpt-5.4-mini", result.Items[0].Name)
	require.Equal(t, "openai", result.Items[0].Provider)
	require.Equal(t, "operational", result.Items[0].PrimaryStatus)
	require.NotNil(t, result.Items[0].PrimaryLatencyMS)
	require.Equal(t, int64(1014), *result.Items[0].PrimaryLatencyMS)
	require.NotNil(t, result.Items[0].PrimaryPingLatencyMS)
	require.Equal(t, int64(13700), *result.Items[0].PrimaryPingLatencyMS)
	require.Equal(t, 99.7, result.Items[0].Availability7D)
	require.Len(t, result.Items[0].Timeline, 1)
	require.Equal(t, "anthropic", result.Items[1].Provider)
	require.Equal(t, "degraded", result.Items[1].PrimaryStatus)
}

func TestClientDirectLoginRequiresBrowserFallbackFor2FAAndTurnstile(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		reason string
	}{
		{
			name:   "2fa",
			body:   `{"success":true,"message":"需要进行两步验证","data":{"require_2fa":true}}`,
			reason: "LOGIN_MFA_REQUIRED",
		},
		{
			name:   "turnstile",
			body:   `{"success":false,"message":"Turnstile token 为空"}`,
			reason: "BROWSER_CHALLENGE_REQUIRED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "/api/user/login", r.URL.Path)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := NewClient(server.Client())
			_, err := client.DirectLogin(context.Background(), ports.DirectLoginInput{
				SupplierID: 7,
				Origin:     server.URL,
				APIBaseURL: server.URL,
				Username:   "ops@example.com",
				Password:   "secret",
			})

			require.Error(t, err)
			require.Equal(t, tt.reason, infraerrors.Reason(err))
		})
	}
}
