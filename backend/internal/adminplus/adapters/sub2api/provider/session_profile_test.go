package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/adminplus/ports"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestSessionProfileClientProbeSub2APIUserProfile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer browser-access-token", r.Header.Get("Authorization"))
		require.Equal(t, "sid=abc; theme=dark", r.Header.Get("Cookie"))
		require.Equal(t, "https://relay.example.com", r.Header.Get("Origin"))
		require.Equal(t, "https://relay.example.com/dashboard", r.Header.Get("Referer"))
		require.Equal(t, "csrf-token", r.Header.Get("X-CSRF-Token"))
		require.Contains(t, r.Header.Get("Accept"), "application/json")
		require.Contains(t, r.Header.Get("User-Agent"), "Mozilla/5.0")
		require.NotContains(t, r.Header.Get("User-Agent"), "sub2api-admin-plus")

		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/user/profile":
			require.Equal(t, http.MethodGet, r.Method)
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
		case "/api/v1/groups/available":
			require.Equal(t, http.MethodGet, r.Method)
			_, _ = w.Write([]byte(`{"data":[{"id":1,"name":"GPT"}]}`))
		case "/api/v1/channels/available":
			require.Equal(t, http.MethodGet, r.Method)
			_, _ = w.Write([]byte(`{"data":[{"name":"OpenAI"}]}`))
		case "/api/v1/announcements":
			require.Equal(t, http.MethodGet, r.Method)
			_, _ = w.Write([]byte(`{"data":[{"id":1,"title":"公告","content":"通知"}]}`))
		case "/api/v1/usage":
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "1", r.URL.Query().Get("page"))
			require.Equal(t, "1", r.URL.Query().Get("page_size"))
			_, _ = w.Write([]byte(`{"data":[]}`))
		case "/api/v1/keys":
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "1", r.URL.Query().Get("page"))
			require.Equal(t, "1", r.URL.Query().Get("page_size"))
			_, _ = w.Write([]byte(`{"data":[]}`))
		default:
			http.NotFound(w, r)
		}
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
	require.True(t, result.Capabilities["can_read_rates"])
	require.True(t, result.Capabilities["can_read_announcements"])
	require.True(t, result.Capabilities["can_create_key"])
	require.True(t, result.Capabilities["can_read_usage_costs"])
	require.Equal(t, int64(42), result.Profile.ID)
	require.Equal(t, "ops@example.com", result.Profile.Email)
	require.Equal(t, []int64{1, 2}, result.Profile.AllowedGroups)
}

func TestBuildSub2APIUserEndpointURLNormalizesBasePath(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		wantPath string
	}{
		{
			name:     "origin",
			baseURL:  "https://relay.example.com",
			wantPath: "/api/v1/user/profile",
		},
		{
			name:     "api base",
			baseURL:  "https://relay.example.com/api",
			wantPath: "/api/v1/user/profile",
		},
		{
			name:     "api v1 base",
			baseURL:  "https://relay.example.com/api/v1?ignored=true",
			wantPath: "/api/v1/user/profile",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildSub2APIUserProfileURL(tt.baseURL)
			require.NoError(t, err)
			require.Equal(t, "https://relay.example.com"+tt.wantPath, got)
			require.NotContains(t, got, "?")
		})
	}
}

func TestSessionProfileClientDirectLogin(t *testing.T) {
	var sawSettings bool
	var sawLogin bool
	var serverURL string
	now := time.Date(2026, 6, 22, 0, 27, 8, 786000000, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/settings/public":
			require.Equal(t, http.MethodGet, r.Method)
			sawSettings = true
			_, _ = w.Write([]byte(`{"data":{"login_agreement_revision":"rev-1"}}`))
		case "/api/v1/auth/login":
			require.Equal(t, http.MethodPost, r.Method)
			require.Equal(t, "application/json", r.Header.Get("Content-Type"))
			require.Contains(t, r.Header.Get("User-Agent"), "Mozilla/5.0")
			require.NotContains(t, r.Header.Get("User-Agent"), "sub2api-admin-plus")
			require.Contains(t, r.Header.Get("Accept"), "application/json")
			require.Contains(t, r.Header.Get("Accept-Language"), "zh-CN")
			require.Equal(t, serverURL, r.Header.Get("Origin"))
			require.Equal(t, serverURL+"/", r.Header.Get("Referer"))
			var payload map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
			require.Equal(t, "ops@example.com", payload["email"])
			require.Equal(t, " secret ", payload["password"])
			require.Equal(t, "rev-1", payload["login_agreement_revision"])
			require.Equal(t, "rev-1", payload["loginAgreementRevision"])
			require.Equal(t, true, payload["agreement_accepted"])
			consent, ok := payload["login_agreement_consent"].(map[string]any)
			require.True(t, ok)
			require.Equal(t, "rev-1", consent["revision"])
			require.Equal(t, now.Format(time.RFC3339Nano), consent["accepted_at"])
			camelConsent, ok := payload["loginAgreementConsent"].(map[string]any)
			require.True(t, ok)
			require.Equal(t, consent, camelConsent)
			sawLogin = true
			_, _ = w.Write([]byte(`{"data":{"access_token":"direct-access-token","expires_in":3600}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	client := NewSessionProfileClient(server.Client())
	client.now = func() time.Time { return now }
	result, err := client.DirectLogin(context.Background(), ports.DirectLoginInput{
		SupplierID: 7,
		Origin:     server.URL,
		APIBaseURL: server.URL,
		Username:   "ops@example.com",
		Password:   " secret ",
	})

	require.NoError(t, err)
	require.True(t, sawSettings)
	require.True(t, sawLogin)
	require.Equal(t, int64(7), result.SupplierID)
	require.Equal(t, server.URL, result.Origin)
	require.Equal(t, server.URL, result.APIBaseURL)
	require.Equal(t, "direct-access-token", result.SessionBundle["access_token"])
	require.Equal(t, "direct_login", result.SessionBundle["session_source"])
	tokens, ok := result.SessionBundle["tokens"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "direct-access-token", tokens["access_token"])
	contextValue, ok := result.SessionBundle["context"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "direct_login", contextValue["login_method"])
	require.NotNil(t, result.ExpiresAt)
}

func TestSessionProfileClientRegisterAccountDirectSuccess(t *testing.T) {
	var sawSettings bool
	var sawRegister bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/settings/public":
			require.Equal(t, http.MethodGet, r.Method)
			sawSettings = true
			_, _ = w.Write([]byte(`{"data":{"registration_enabled":true,"email_verification_enabled":false,"turnstile_enabled":false}}`))
		case "/api/v1/auth/register":
			require.Equal(t, http.MethodPost, r.Method)
			var payload map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
			require.Equal(t, "ops@example.com", payload["email"])
			require.Equal(t, " secret ", payload["password"])
			require.Equal(t, "", payload["verify_code"])
			sawRegister = true
			_, _ = w.Write([]byte(`{"code":0,"data":{"access_token":"registered-access-token","refresh_token":"refresh-token","expires_in":3600}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewSessionProfileClient(server.Client())
	result, err := client.RegisterAccount(context.Background(), ports.DirectRegistrationInput{
		ProviderType: adminplusdomain.SupplierTypeSub2API,
		Origin:       server.URL + "/register",
		APIBaseURL:   server.URL,
		Email:        "ops@example.com",
		Password:     " secret ",
	})

	require.NoError(t, err)
	require.True(t, sawSettings)
	require.True(t, sawRegister)
	require.Equal(t, ports.DirectRegistrationStageCompleted, result.Stage)
	require.True(t, result.Submitted)
	require.Equal(t, "registered-access-token", result.SessionBundle["access_token"])
}

func TestSessionProfileClientRegisterAccountRequestsEmailVerificationCode(t *testing.T) {
	var sawVerifyCode bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/settings/public":
			_, _ = w.Write([]byte(`{"data":{"registration_enabled":true,"email_verification_enabled":true,"turnstile_enabled":false}}`))
		case "/api/v1/auth/send-verify-code":
			require.Equal(t, http.MethodPost, r.Method)
			var payload map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
			require.Equal(t, "ops@example.com", payload["email"])
			sawVerifyCode = true
			_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"countdown":60}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewSessionProfileClient(server.Client())
	result, err := client.RegisterAccount(context.Background(), ports.DirectRegistrationInput{
		ProviderType: adminplusdomain.SupplierTypeSub2API,
		Origin:       server.URL,
		APIBaseURL:   server.URL,
		Email:        "ops@example.com",
		Password:     "secret",
	})

	require.NoError(t, err)
	require.True(t, sawVerifyCode)
	require.Equal(t, ports.DirectRegistrationStageNeedEmailCode, result.Stage)
	require.True(t, result.EmailCodeRequired)
}

func TestSessionProfileClientDirectLoginUsesBrowserUAWhenLegacyAdapterUAIsRejected(t *testing.T) {
	var sawBrowserSettings bool
	var sawBrowserLogin bool
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userAgent := r.Header.Get("User-Agent")
		if strings.Contains(userAgent, "sub2api-admin-plus-provider-adapter") || !strings.Contains(userAgent, "Mozilla/5.0") {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(520)
			_, _ = w.Write([]byte(`<!doctype html><html><body>The origin web server returned an invalid or incomplete response to Cloudflare.</body></html>`))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/settings/public":
			require.Equal(t, http.MethodGet, r.Method)
			sawBrowserSettings = true
			_, _ = w.Write([]byte(`{"data":{"login_agreement_revision":"rev-ua"}}`))
		case "/api/v1/auth/login":
			require.Equal(t, http.MethodPost, r.Method)
			require.Equal(t, serverURL, r.Header.Get("Origin"))
			require.Equal(t, serverURL+"/", r.Header.Get("Referer"))
			var payload map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
			require.Equal(t, "rev-ua", payload["login_agreement_revision"])
			require.Equal(t, "rev-ua", payload["loginAgreementRevision"])
			require.Equal(t, true, payload["agreement_accepted"])
			consent, ok := payload["login_agreement_consent"].(map[string]any)
			require.True(t, ok)
			require.Equal(t, "rev-ua", consent["revision"])
			sawBrowserLogin = true
			_, _ = w.Write([]byte(`{"data":{"access_token":"browser-ua-access-token","expires_in":3600}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	legacyReq, err := http.NewRequest(http.MethodGet, server.URL+"/api/v1/settings/public", nil)
	require.NoError(t, err)
	legacyReq.Header.Set("User-Agent", "sub2api-admin-plus-provider-adapter/0.1")
	legacyResp, err := server.Client().Do(legacyReq)
	require.NoError(t, err)
	defer func() { _ = legacyResp.Body.Close() }()
	require.Equal(t, 520, legacyResp.StatusCode)

	client := NewSessionProfileClient(server.Client())
	result, err := client.DirectLogin(context.Background(), ports.DirectLoginInput{
		SupplierID: 7,
		Origin:     server.URL,
		APIBaseURL: server.URL,
		Username:   "ops@example.com",
		Password:   "secret",
	})

	require.NoError(t, err)
	require.True(t, sawBrowserSettings)
	require.True(t, sawBrowserLogin)
	require.Equal(t, "browser-ua-access-token", result.SessionBundle["access_token"])
}

func TestSessionProfileClientDirectLoginDoesNotPreflightGlobalTotpSetting(t *testing.T) {
	var sawLogin bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/settings/public":
			_, _ = w.Write([]byte(`{"data":{"totp_enabled":true,"login_agreement_revision":"rev-totp"}}`))
		case "/api/v1/auth/login":
			sawLogin = true
			var payload map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
			require.Equal(t, "rev-totp", payload["login_agreement_revision"])
			_, _ = w.Write([]byte(`{"data":{"access_token":"totp-site-access-token","refresh_token":"refresh-token","expires_in":86400}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewSessionProfileClient(server.Client())
	result, err := client.DirectLogin(context.Background(), ports.DirectLoginInput{
		SupplierID: 7,
		Origin:     server.URL,
		APIBaseURL: server.URL,
		Username:   "ops@example.com",
		Password:   "secret",
	})

	require.NoError(t, err)
	require.True(t, sawLogin)
	require.Equal(t, "totp-site-access-token", result.SessionBundle["access_token"])
	require.Equal(t, "refresh-token", result.SessionBundle["refresh_token"])
	require.Equal(t, true, result.Diagnostics["settings_totp_enabled"])
}

func TestSessionProfileClientReadChannelMonitors(t *testing.T) {
	var sawMonitors bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/channel-monitors", r.URL.Path)
		require.Equal(t, "Bearer browser-access-token", r.Header.Get("Authorization"))
		require.Contains(t, r.Header.Get("Accept"), "application/json")
		sawMonitors = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": {
				"items": [
					{
						"id": 11,
						"name": "openai池状态",
						"provider": "openai",
						"group_name": "vip站长",
						"primary_model": "gpt-5.5",
						"primary_status": "operational",
						"primary_latency_ms": 1329,
						"primary_ping_latency_ms": 1,
						"availability_7d": 85.64,
						"extra_models": [{"model":"gpt-5.1","status":"operational","latency_ms":1200}],
						"timeline": [{"status":"operational","latency_ms":1329,"ping_latency_ms":1,"checked_at":"2026-06-21T10:00:00Z"}]
					}
				]
			}
		}`))
	}))
	defer server.Close()

	client := NewSessionProfileClient(server.Client())
	result, err := client.ReadChannelMonitors(context.Background(), ports.SessionProbeInput{
		SupplierID: 7,
		Origin:     server.URL,
		APIBaseURL: server.URL,
		Bundle: map[string]any{
			"access_token": "browser-access-token",
		},
	})

	require.NoError(t, err)
	require.True(t, sawMonitors)
	require.Equal(t, int64(7), result.SupplierID)
	require.Equal(t, "sub2api", result.SystemType)
	require.Len(t, result.Items, 1)
	item := result.Items[0]
	require.Equal(t, int64(11), item.ID)
	require.Equal(t, "openai池状态", item.Name)
	require.Equal(t, "openai", item.Provider)
	require.Equal(t, "vip站长", item.GroupName)
	require.Equal(t, "gpt-5.5", item.PrimaryModel)
	require.Equal(t, "operational", item.PrimaryStatus)
	require.NotNil(t, item.PrimaryLatencyMS)
	require.Equal(t, int64(1329), *item.PrimaryLatencyMS)
	require.InDelta(t, 85.64, item.Availability7D, 0.001)
	require.Len(t, item.ExtraModels, 1)
	require.Len(t, item.Timeline, 1)
}

func TestSessionProfileClientDirectLoginClassifiesCaptcha(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/settings/public":
			_, _ = w.Write([]byte(`{"data":{}}`))
		case "/api/v1/auth/login":
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"message":"turnstile captcha required"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewSessionProfileClient(server.Client())
	_, err := client.DirectLogin(context.Background(), ports.DirectLoginInput{
		SupplierID: 7,
		Origin:     server.URL,
		APIBaseURL: server.URL,
		Username:   "ops@example.com",
		Password:   "secret",
	})

	require.Error(t, err)
	require.Equal(t, "LOGIN_CAPTCHA_REQUIRED", infraerrors.Reason(err))
}

func TestSessionProfileClientDirectLoginPreflightsTurnstile(t *testing.T) {
	var sawLogin bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/settings/public":
			_, _ = w.Write([]byte(`{"data":{"turnstile_enabled":true}}`))
		case "/api/v1/auth/login":
			sawLogin = true
			_, _ = w.Write([]byte(`{"data":{"access_token":"unexpected"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewSessionProfileClient(server.Client())
	_, err := client.DirectLogin(context.Background(), ports.DirectLoginInput{
		SupplierID: 7,
		Origin:     server.URL,
		APIBaseURL: server.URL,
		Username:   "ops@example.com",
		Password:   "secret",
	})

	require.Error(t, err)
	require.False(t, sawLogin)
	require.Equal(t, "LOGIN_CAPTCHA_REQUIRED", infraerrors.Reason(err))
}

func TestSessionProfileClientDirectLoginClassifiesCloudflareOriginError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/settings/public":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{}}`))
		case "/api/v1/auth/login":
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(520)
			_, _ = w.Write([]byte(`<!doctype html><html><body>The origin web server returned an invalid or incomplete response to Cloudflare.</body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewSessionProfileClient(server.Client())
	_, err := client.DirectLogin(context.Background(), ports.DirectLoginInput{
		SupplierID: 7,
		Origin:     server.URL,
		APIBaseURL: server.URL,
		Username:   "ops@example.com",
		Password:   "secret",
	})

	require.Error(t, err)
	require.Equal(t, "SUPPLIER_DIRECT_LOGIN_UPSTREAM_ORIGIN_ERROR", infraerrors.Reason(err))
	require.NotContains(t, infraerrors.Message(err), "Cloudflare.")
}

func TestSessionProfileClientDirectLoginClassifiesSettingsHTML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/settings/public":
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`<!doctype html><html><body>Bad gateway</body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewSessionProfileClient(server.Client())
	_, err := client.DirectLogin(context.Background(), ports.DirectLoginInput{
		SupplierID: 7,
		Origin:     server.URL,
		APIBaseURL: server.URL,
		Username:   "ops@example.com",
		Password:   "secret",
	})

	require.Error(t, err)
	require.Equal(t, "SUPPLIER_DIRECT_LOGIN_UPSTREAM_HTML", infraerrors.Reason(err))
}

func TestSessionProfileClientCreateKey(t *testing.T) {
	var listCalled bool
	var createCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer browser-access-token", r.Header.Get("Authorization"))
		require.Equal(t, "sid=abc", r.Header.Get("Cookie"))
		require.Equal(t, "/api/v1/keys", r.URL.Path)
		if r.Method == http.MethodGet {
			listCalled = true
			require.Equal(t, "1", r.URL.Query().Get("page"))
			require.Equal(t, "100", r.URL.Query().Get("page_size"))
			require.Equal(t, "ops-key", r.URL.Query().Get("search"))
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"items":[]}}`))
			return
		}

		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		createCalled = true

		var payload map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		require.Equal(t, "ops-key", payload["name"])
		require.Equal(t, float64(10), payload["group_id"])
		require.Equal(t, float64(25), payload["quota"])
		require.Equal(t, float64(7), payload["expires_in_days"])

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": {
				"id": 99,
				"name": "ops-key",
				"key": "sk-supplier-secret",
				"group_id": 10,
				"status": "active"
			}
		}`))
	}))
	defer server.Close()

	client := NewSessionProfileClient(server.Client())
	expires := 7
	result, err := client.CreateKey(context.Background(), ports.SessionProbeInput{
		SupplierID: 7,
		APIBaseURL: server.URL + "/api/v1",
		Bundle: map[string]any{
			"access_token": "browser-access-token",
			"required_headers": map[string]any{
				"cookie": "sid=abc",
			},
		},
	}, ports.CreateProviderKeyInput{
		SupplierID:      7,
		ExternalGroupID: "10",
		Name:            "ops-key",
		QuotaUSD:        25,
		ExpiresInDays:   &expires,
	})

	require.NoError(t, err)
	require.Equal(t, int64(7), result.SupplierID)
	require.Equal(t, "99", result.ExternalKeyID)
	require.Equal(t, "10", result.ExternalGroupID)
	require.Equal(t, "ops-key", result.Name)
	require.Equal(t, "sk-supplier-secret", result.Secret)
	require.Equal(t, "active", result.Status)
	require.NotContains(t, result.RawPayload, "key")
	require.True(t, listCalled)
	require.True(t, createCalled)
}

func TestSessionProfileClientListKeysReadsUserKeys(t *testing.T) {
	var sawList bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer browser-access-token", r.Header.Get("Authorization"))
		require.Equal(t, "sid=abc", r.Header.Get("Cookie"))
		require.Equal(t, "/api/v1/keys", r.URL.Path)
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "1", r.URL.Query().Get("page"))
		require.Equal(t, "1000", r.URL.Query().Get("page_size"))
		sawList = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": {
				"total": 2,
				"items": [
					{
						"id": 99,
						"name": "ops-low",
						"key": "sk-secret",
						"group_routes": [{"group_id": 10}],
						"status": "active"
					},
					{
						"id": 100,
						"name": "ops-disabled",
						"api_key": "sk-disabled",
						"group_id": 20,
						"status": "inactive"
					}
				]
			}
		}`))
	}))
	defer server.Close()

	client := NewSessionProfileClient(server.Client())
	result, err := client.ListKeys(context.Background(), ports.SessionProbeInput{
		SupplierID: 7,
		APIBaseURL: server.URL,
		Bundle: map[string]any{
			"access_token": "browser-access-token",
			"required_headers": map[string]any{
				"cookie": "sid=abc",
			},
		},
	}, ports.ListProviderKeysInput{
		SupplierID: 7,
	})

	require.NoError(t, err)
	require.True(t, sawList)
	require.Equal(t, int64(7), result.SupplierID)
	require.Equal(t, "sub2api", result.SystemType)
	require.Equal(t, 2, result.Total)
	require.Len(t, result.Keys, 2)
	require.Equal(t, "99", result.Keys[0].ExternalKeyID)
	require.Equal(t, "10", result.Keys[0].ExternalGroupID)
	require.Equal(t, "active", result.Keys[0].Status)
	require.NotContains(t, result.Keys[0].RawPayload, "key")
	require.Equal(t, "100", result.Keys[1].ExternalKeyID)
	require.Equal(t, "20", result.Keys[1].ExternalGroupID)
	require.Equal(t, "disabled", result.Keys[1].Status)
	require.NotContains(t, result.Keys[1].RawPayload, "api_key")
}

func TestSessionProfileClientReadKeyCapacityReadsPaginatedUserKeys(t *testing.T) {
	pages := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer browser-access-token", r.Header.Get("Authorization"))
		require.Equal(t, "sid=abc", r.Header.Get("Cookie"))
		require.Equal(t, "/api/v1/keys", r.URL.Path)
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "2", r.URL.Query().Get("page_size"))
		page := r.URL.Query().Get("page")
		pages = append(pages, page)
		w.Header().Set("Content-Type", "application/json")
		switch page {
		case "1":
			_, _ = w.Write([]byte(`{
				"data": {
					"total": 3,
					"items": [
						{
							"id": 99,
							"name": "ops-low",
							"key": "sk-secret",
							"group_routes": [{"group_id": 10}],
							"status": "active"
						},
						{
							"id": 100,
							"name": "ops-disabled",
							"api_key": "sk-disabled",
							"group_id": 20,
							"status": "inactive"
						}
					]
				}
			}`))
		case "2":
			_, _ = w.Write([]byte(`{
				"data": {
					"total": 3,
					"items": [
						{
							"id": 101,
							"name": "ops-mid",
							"key": "sk-mid",
							"group_routes": [{"group_id": 30}],
							"status": "active"
						}
					]
				}
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewSessionProfileClient(server.Client())
	result, err := client.ReadKeyCapacity(context.Background(), ports.SessionProbeInput{
		SupplierID: 7,
		APIBaseURL: server.URL,
		Bundle: map[string]any{
			"access_token": "browser-access-token",
			"required_headers": map[string]any{
				"cookie": "sid=abc",
			},
		},
	}, ports.ReadProviderKeyCapacityInput{
		SupplierID: 7,
		Limit:      2,
	})

	require.NoError(t, err)
	require.Equal(t, []string{"1", "2"}, pages)
	require.Equal(t, int64(7), result.SupplierID)
	require.Equal(t, "sub2api", result.SystemType)
	require.Equal(t, "unknown", result.KeyLimitPolicy)
	require.False(t, result.LimitKnown)
	require.Equal(t, 2, result.ActiveKeyCount)
	require.Equal(t, "unknown", result.KeyCapacityStatus)
	require.Len(t, result.Keys, 3)
	require.Equal(t, "30", result.Keys[2].ExternalGroupID)
	require.NotContains(t, result.Keys[0].RawPayload, "key")
	require.NotContains(t, result.Keys[1].RawPayload, "api_key")
	require.Equal(t, "list_keys", result.Diagnostics["capacity_source"])
	require.Equal(t, "not_exposed_by_provider", result.Diagnostics["limit_source"])
	require.Equal(t, 2, result.Diagnostics["pages_read"])
	require.Equal(t, 3, result.Diagnostics["provider_total"])
	require.NotEqual(t, true, result.Diagnostics["truncated"])
}

func TestSessionProfileClientCreateKeyReusesExistingProviderKey(t *testing.T) {
	var createCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/keys", r.URL.Path)
		require.Equal(t, "Bearer browser-access-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte(`{
				"data": {
					"items": [
						{
							"id": 99,
							"name": "ops-key",
							"key": "sk-existing-secret",
							"group_routes": [
								{"group_id": 10}
							],
							"status": "active"
						}
					]
				}
			}`))
		case http.MethodPost:
			createCalled = true
			http.Error(w, "must not create duplicate key", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewSessionProfileClient(server.Client())
	result, err := client.CreateKey(context.Background(), ports.SessionProbeInput{
		SupplierID: 7,
		APIBaseURL: server.URL,
		Bundle: map[string]any{
			"access_token": "browser-access-token",
		},
	}, ports.CreateProviderKeyInput{
		SupplierID:      7,
		ExternalGroupID: "10",
		Name:            "ops-key",
		QuotaUSD:        25,
	})

	require.NoError(t, err)
	require.False(t, createCalled)
	require.Equal(t, "99", result.ExternalKeyID)
	require.Equal(t, "10", result.ExternalGroupID)
	require.Equal(t, "ops-key", result.Name)
	require.Equal(t, "sk-existing-secret", result.Secret)
}

func TestSessionProfileClientDisableKeyPreservesKeyFields(t *testing.T) {
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer browser-access-token", r.Header.Get("Authorization"))
		require.Equal(t, "/api/v1/keys/99", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte(`{
				"data": {
					"id": 99,
					"name": "ops-key",
					"group_id": 10,
					"status": "active",
					"ip_whitelist": ["10.0.0.1"],
					"ip_blacklist": ["192.168.0.0/16"]
				}
			}`))
		case http.MethodPut:
			require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
			_, _ = w.Write([]byte(`{
				"data": {
					"id": 99,
					"name": "ops-key",
					"group_id": 10,
					"status": "inactive"
				}
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewSessionProfileClient(server.Client())
	result, err := client.DisableKey(context.Background(), ports.SessionProbeInput{
		SupplierID: 7,
		APIBaseURL: server.URL,
		Bundle: map[string]any{
			"access_token": "browser-access-token",
		},
	}, ports.DisableProviderKeyInput{
		SupplierID:      7,
		ExternalGroupID: "10",
		ExternalKeyID:   "99",
		Name:            "ops-key",
	})

	require.NoError(t, err)
	require.Equal(t, "ops-key", payload["name"])
	require.Equal(t, "inactive", payload["status"])
	require.Equal(t, []any{"10.0.0.1"}, payload["ip_whitelist"])
	require.Equal(t, []any{"192.168.0.0/16"}, payload["ip_blacklist"])
	require.Equal(t, "99", result.ExternalKeyID)
	require.Equal(t, "10", result.ExternalGroupID)
	require.Equal(t, "disabled", result.Status)
}

func TestSessionProfileClientDeleteKeyUsesUserKeyDelete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer browser-access-token", r.Header.Get("Authorization"))
		require.Equal(t, "/api/v1/keys/99", r.URL.Path)
		require.Equal(t, http.MethodDelete, r.Method)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"message":"API key deleted successfully"}}`))
	}))
	defer server.Close()

	client := NewSessionProfileClient(server.Client())
	result, err := client.DeleteKey(context.Background(), ports.SessionProbeInput{
		SupplierID: 7,
		APIBaseURL: server.URL,
		Bundle: map[string]any{
			"access_token": "browser-access-token",
		},
	}, ports.DeleteProviderKeyInput{
		SupplierID:      7,
		ExternalGroupID: "10",
		ExternalKeyID:   "99",
		Name:            "ops-key",
	})

	require.NoError(t, err)
	require.Equal(t, "99", result.ExternalKeyID)
	require.Equal(t, "10", result.ExternalGroupID)
	require.Equal(t, "deleted", result.Status)
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

func TestSessionProfileClientReadRates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer browser-access-token", r.Header.Get("Authorization"))
		require.Equal(t, "sid=abc", r.Header.Get("Cookie"))
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/rates/snapshots":
			_, _ = w.Write([]byte(`{"data":[]}`))
		case "/api/v1/channels/available":
			_, _ = w.Write([]byte(`{
				"data": [
					{
						"name": "OpenAI",
						"supported_models": [
							{
								"name": "gpt-5-mini",
								"platform": "openai",
								"pricing": {
									"billing_mode": "token",
									"input_price": 0.0000015,
									"output_price": 0.000006,
									"cache_read_price_micros": 250000
								}
							}
						]
					}
				]
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewSessionProfileClient(server.Client())
	result, err := client.ReadRates(context.Background(), ports.SessionProbeInput{
		SupplierID: 7,
		Origin:     server.URL,
		APIBaseURL: server.URL,
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
	require.Len(t, result.Entries, 3)

	input := findRateEntry(t, result.Entries, "input")
	require.Equal(t, "gpt-5-mini", input.Model)
	require.Equal(t, "1m_tokens", input.Unit)
	require.Equal(t, int64(1500000), input.PriceMicros)
	require.Equal(t, int64(6000000), findRateEntry(t, result.Entries, "output").PriceMicros)
	require.Equal(t, int64(250000), findRateEntry(t, result.Entries, "cache_read").PriceMicros)
}

func TestSessionProfileClientReadRatesParsesNestedAvailableChannels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer browser-access-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/rates/snapshots":
			http.NotFound(w, r)
		case "/api/v1/channels/available":
			_, _ = w.Write([]byte(`{
				"data": [
					{
						"name": "CODEX",
						"platforms": [
							{
								"platform": "openai",
								"groups": [
									{"id": 59, "name": "CODEX", "rate_multiplier": 0.12}
								],
								"supported_models": [
									{
										"name": "codex-auto-review",
										"platform": "openai",
										"pricing": {
											"billing_mode": "token",
											"input_price": 0.000001,
											"output_price": 0.000004,
											"cache_write_price": 0.0000005,
											"cache_read_price": 0.0000001,
											"image_input_price": 0.000002,
											"image_cache_read_price": 0.0000002,
											"image_output_price": 0.000008,
											"per_request_price": 0.01
										}
									}
								]
							}
						]
					}
				]
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewSessionProfileClient(server.Client())
	result, err := client.ReadRates(context.Background(), ports.SessionProbeInput{
		SupplierID: 7,
		Origin:     server.URL,
		APIBaseURL: server.URL,
		Bundle: map[string]any{
			"access_token": "browser-access-token",
		},
	})

	require.NoError(t, err)
	require.Equal(t, int64(7), result.SupplierID)
	require.Len(t, result.Entries, 8)
	require.Equal(t, int64(1000000), findRateEntry(t, result.Entries, "input").PriceMicros)
	require.Equal(t, int64(4000000), findRateEntry(t, result.Entries, "output").PriceMicros)
	require.Equal(t, int64(500000), findRateEntry(t, result.Entries, "cache_write").PriceMicros)
	require.Equal(t, int64(100000), findRateEntry(t, result.Entries, "cache_read").PriceMicros)
	require.Equal(t, int64(2000000), findRateEntry(t, result.Entries, "image_input").PriceMicros)
	require.Equal(t, int64(200000), findRateEntry(t, result.Entries, "image_cache_read").PriceMicros)
	require.Equal(t, int64(8000000), findRateEntry(t, result.Entries, "image_output").PriceMicros)
	require.Equal(t, int64(10000), findRateEntry(t, result.Entries, "per_request").PriceMicros)
}

func TestSessionProfileClientReadAnnouncements(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer browser-access-token", r.Header.Get("Authorization"))
		require.Equal(t, "sid=abc", r.Header.Get("Cookie"))
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/user/profile":
			_, _ = w.Write([]byte(`{"data":{"id":42,"email":"ops@example.com","balance":12.34}}`))
		case "/api/v1/payment/checkout-info":
			_, _ = w.Write([]byte(`{
				"data": {
					"currency": "usd",
					"balance_recharge_multiplier": 1.2,
					"global_min": 100
				}
			}`))
		case "/api/v1/announcements":
			_, _ = w.Write([]byte(`{
				"data": [
					{
						"id": "notice-1",
						"title": "Limited offer",
						"content": "Weekend limited sale",
						"type": "limited_offer",
						"discount_percent": "15%"
					},
					{
						"id": "notice-2",
						"title": "维护通知",
						"content": "今晚模型网关维护，部分请求可能中断"
					}
				]
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewSessionProfileClient(server.Client())
	result, err := client.ReadAnnouncements(context.Background(), ports.SessionProbeInput{
		SupplierID: 7,
		Origin:     server.URL,
		APIBaseURL: server.URL,
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
	require.Len(t, result.Announcements, 3)
	require.Equal(t, "充值倍率公告 20.00%", result.Announcements[0].Title)
	require.Equal(t, int64(10000), result.Announcements[0].MinRechargeCents)
	require.Equal(t, int64(1234), result.Announcements[0].BalanceCents)
	require.NotNil(t, result.Announcements[0].BonusPercent)
	require.InEpsilon(t, 20.0, *result.Announcements[0].BonusPercent, 0.0001)
	require.Equal(t, "Limited offer", result.Announcements[1].Title)
	require.Equal(t, adminplusdomain.AnnouncementTypeLimitedOffer, result.Announcements[1].Type)
	require.Equal(t, "维护通知", result.Announcements[2].Title)
	require.Equal(t, adminplusdomain.AnnouncementTypeMaintenance, result.Announcements[2].Type)
	require.Equal(t, "maintenance", result.Announcements[2].RawPayload["classification"])
}

func TestSessionProfileClientReadUsageCosts(t *testing.T) {
	startedAt := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)
	endedAt := startedAt.Add(24 * time.Hour)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer browser-access-token", r.Header.Get("Authorization"))
		require.Equal(t, "sid=abc", r.Header.Get("Cookie"))
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/admin/usage":
			require.Equal(t, "1", r.URL.Query().Get("page"))
			require.Equal(t, "1000", r.URL.Query().Get("page_size"))
			require.Equal(t, "created_at", r.URL.Query().Get("sort_by"))
			require.Equal(t, "desc", r.URL.Query().Get("sort_order"))
			require.Equal(t, "2026-06-20", r.URL.Query().Get("start_date"))
			require.Equal(t, "2026-06-20", r.URL.Query().Get("end_date"))
			require.Equal(t, startedAt.Format(time.RFC3339), r.URL.Query().Get("started_at"))
			require.Equal(t, endedAt.Format(time.RFC3339), r.URL.Query().Get("ended_at"))
			require.Equal(t, startedAt.Format(time.RFC3339), r.URL.Query().Get("from"))
			require.Equal(t, endedAt.Format(time.RFC3339), r.URL.Query().Get("to"))
			_, _ = w.Write([]byte(`{
				"data": {
					"items": [
						{
							"id": 91,
							"request_id": "req-1",
							"api_key_name": "ops-key",
							"model": "gpt-5-mini",
							"endpoint": "/v1/responses",
							"request_type": "responses",
							"billing_mode": "token",
							"currency": "usd",
							"cost": 1.23,
							"input_tokens": 1000,
							"output_tokens": 500,
							"cache_read_tokens": 200,
							"first_token_ms": 680,
							"duration_ms": 2200,
							"user_agent": "OpenAI/Python",
							"started_at": "2026-06-20T10:00:00Z",
							"ended_at": "2026-06-20T10:00:02Z",
							"access_token": "must-not-persist",
							"headers": {
								"cookie": "sid=secret",
								"x-safe": "kept"
							}
						}
					]
					,
					"total": 1,
					"page": 1,
					"page_size": 1000,
					"pages": 1
				}
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewSessionProfileClient(server.Client())
	result, err := client.ReadUsageCosts(context.Background(), ports.SessionProbeInput{
		SupplierID: 7,
		Origin:     server.URL,
		APIBaseURL: server.URL,
		Bundle: map[string]any{
			"access_token": "browser-access-token",
			"required_headers": map[string]any{
				"cookie": "sid=abc",
			},
		},
	}, ports.ReadUsageCostsInput{
		SupplierID: 7,
		StartedAt:  startedAt,
		EndedAt:    endedAt,
	})

	require.NoError(t, err)
	require.Equal(t, int64(7), result.SupplierID)
	require.Equal(t, "sub2api", result.SystemType)
	require.Len(t, result.Lines, 1)
	line := result.Lines[0]
	require.Equal(t, "91", line.ExternalUsageCostID)
	require.Equal(t, "req-1", line.ExternalRequestID)
	require.Equal(t, "ops-key", line.APIKeyName)
	require.Equal(t, "gpt-5-mini", line.Model)
	require.Equal(t, int64(123), line.CostCents)
	require.Equal(t, int64(1000), line.InputTokens)
	require.Equal(t, int64(500), line.OutputTokens)
	require.Equal(t, int64(200), line.CacheReadTokens)
	require.Equal(t, int64(680), line.FirstTokenMS)
	require.Equal(t, "usd", line.Currency)
	require.NotContains(t, line.RawPayload, "access_token")
	headers, ok := line.RawPayload["headers"].(map[string]any)
	require.True(t, ok)
	require.NotContains(t, headers, "cookie")
	require.Equal(t, "kept", headers["x-safe"])
}

func TestSessionProfileClientReadUsageCostsUsesUserEndpointForUserRole(t *testing.T) {
	startedAt := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)
	endedAt := startedAt.Add(24 * time.Hour)
	requestedPaths := make([]string, 0, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPaths = append(requestedPaths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/usage":
			require.Equal(t, "1", r.URL.Query().Get("page"))
			require.Equal(t, "1000", r.URL.Query().Get("page_size"))
			_, _ = w.Write([]byte(`{
				"data": {
					"items": [
						{
							"id": 92,
							"request_id": "req-user",
							"model": "gpt-5-mini",
							"total_cost": 0.42,
							"created_at": "2026-06-20T08:00:00Z"
						}
					],
					"total": 1,
					"page": 1,
					"page_size": 1000,
					"pages": 1
				}
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewSessionProfileClient(server.Client())
	result, err := client.ReadUsageCosts(context.Background(), ports.SessionProbeInput{
		SupplierID: 7,
		Origin:     server.URL,
		APIBaseURL: server.URL,
		Bundle: map[string]any{
			"access_token": "browser-access-token",
			"context": map[string]any{
				"role": "user",
			},
		},
	}, ports.ReadUsageCostsInput{
		SupplierID: 7,
		StartedAt:  startedAt,
		EndedAt:    endedAt,
	})

	require.NoError(t, err)
	require.Len(t, result.Lines, 1)
	require.Equal(t, []string{"/api/v1/usage"}, requestedPaths)
}

func TestParseSub2APIUsageCostLine_RealSub2APIShape(t *testing.T) {
	lines := parseSub2APIUsageCostLines([]byte(`{
		"code": 0,
		"data": {
			"items": [
				{
					"id": 45140367,
					"request_id": "generated:r1n61tur3bvbcl-3wwwf",
					"model": "gpt-5.5",
					"service_tier": "default",
					"reasoning_effort": "xhigh",
					"inbound_endpoint": "/v1/responses",
					"upstream_endpoint": "/v1/responses",
					"input_tokens": 17273,
					"output_tokens": 1635,
					"cache_read_tokens": 4544,
					"input_cost": 1.136365,
					"output_cost": 0.04905,
					"total_cost": 1.189959,
					"actual_cost": 0.2379918,
					"balance_deducted": 0.2379918,
					"billing_wallet_type": "balance",
					"request_type": "stream",
					"duration_ms": 35525,
					"first_token_ms": 996,
					"billing_mode": "token",
					"created_at": "2026-06-21T11:39:27.689924+08:00"
				}
			]
		}
	}`))

	require.Len(t, lines, 1)
	line := lines[0]
	require.Equal(t, "45140367", line.ExternalUsageCostID)
	require.Equal(t, "generated:r1n61tur3bvbcl-3wwwf", line.ExternalRequestID)
	require.Equal(t, "gpt-5.5", line.Model)
	require.Equal(t, "/v1/responses", line.Endpoint)
	require.Equal(t, "stream", line.RequestType)
	require.Equal(t, "token", line.BillingMode)
	require.Equal(t, "xhigh", line.ReasoningEffort)
	require.Equal(t, int64(24), line.CostCents)
	require.Equal(t, int64(17273), line.InputTokens)
	require.Equal(t, int64(1635), line.OutputTokens)
	require.Equal(t, int64(4544), line.CacheReadTokens)
	require.Equal(t, int64(996), line.FirstTokenMS)
	require.Equal(t, int64(35525), line.DurationMS)
}

func TestSessionProfileClientReadFundingTransactions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer browser-access-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/payment/orders/my":
			require.Equal(t, "1", r.URL.Query().Get("page"))
			require.Equal(t, "100", r.URL.Query().Get("page_size"))
			_, _ = w.Write([]byte(`{
				"data": {
					"items": [
						{
							"id": 91,
							"out_trade_no": "out-91",
							"payment_trade_no": "pay-secret-abcdef",
							"payment_type": "stripe",
							"order_type": "balance",
							"status": "paid",
							"currency": "usd",
							"amount": "12.34",
							"pay_amount_cents": 1200,
							"fee_rate": 0.03,
							"created_at": "2026-06-20T10:00:00Z",
							"paid_at": "2026-06-20T10:00:02Z"
						}
					]
				}
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewSessionProfileClient(server.Client())
	result, err := client.ReadFundingTransactions(context.Background(), ports.SessionProbeInput{
		SupplierID: 7,
		Origin:     server.URL,
		APIBaseURL: server.URL,
		Bundle:     map[string]any{"access_token": "browser-access-token"},
	}, ports.ReadFundingTransactionsInput{SupplierID: 7})

	require.NoError(t, err)
	require.Equal(t, "sub2api", result.ProviderType)
	require.Len(t, result.Items, 1)
	item := result.Items[0]
	require.Equal(t, "91", item.ExternalID)
	require.Equal(t, "out-91", item.OutTradeNo)
	require.Equal(t, "***********abcdef", item.PaymentTradeNo)
	require.Equal(t, "stripe", item.PaymentType)
	require.Equal(t, "PAID", item.Status)
	require.Equal(t, int64(1234), item.AmountCents)
	require.Equal(t, int64(1200), item.CashAmountCents)
	require.NotNil(t, item.FeeRate)
	require.InEpsilon(t, 0.03, *item.FeeRate, 0.0001)
	require.NotNil(t, item.PaidAt)
	require.Equal(t, time.Date(2026, 6, 20, 10, 0, 2, 0, time.UTC), *item.PaidAt)
}

func TestSessionProfileClientReadEntitlementTransactions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer browser-access-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/redeem/history":
			_, _ = w.Write([]byte(`{
				"data": [
					{
						"id": 10,
						"code": "PAY-ORDER-SECRET-0001",
						"type": "balance",
						"status": "used",
						"currency": "usd",
						"value": "5.50",
						"group_id": 3,
						"validity_days": 30,
						"used_at": "2026-06-20T11:00:00Z"
					},
					{
						"id": 11,
						"redeem_code": "MANUAL-CODE-9999",
						"amount_cents": 2500,
						"created_at": "2026-06-20T12:00:00Z"
					},
					{
						"id": 12,
						"redeem_code": "CONC-CODE-0002",
						"type": "concurrency",
						"status": "used",
						"value": 70,
						"used_at": "2026-06-21T21:55:32Z"
					}
				]
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewSessionProfileClient(server.Client())
	result, err := client.ReadEntitlementTransactions(context.Background(), ports.SessionProbeInput{
		SupplierID: 7,
		Origin:     server.URL,
		APIBaseURL: server.URL,
		Bundle:     map[string]any{"access_token": "browser-access-token"},
	}, ports.ReadEntitlementTransactionsInput{SupplierID: 7})

	require.NoError(t, err)
	require.Equal(t, "sub2api", result.ProviderType)
	require.Len(t, result.Items, 3)
	auto := result.Items[0]
	require.Equal(t, "10", auto.ExternalID)
	require.Equal(t, "payment_auto_redeem", auto.SourceFamily)
	require.Equal(t, "0001", auto.CodeLast4)
	require.NotEmpty(t, auto.CodeFingerprint)
	require.Equal(t, int64(550), auto.ValueCents)
	require.Equal(t, int64(3), auto.GroupID)
	require.Equal(t, 30, auto.ValidityDays)
	require.NotNil(t, auto.UsedAt)

	manual := result.Items[1]
	require.Equal(t, "11", manual.ExternalID)
	require.Equal(t, "manual_redeem", manual.SourceFamily)
	require.Equal(t, "9999", manual.CodeLast4)
	require.Equal(t, int64(2500), manual.ValueCents)
	require.NotEmpty(t, manual.CodeFingerprint)

	concurrency := result.Items[2]
	require.Equal(t, "12", concurrency.ExternalID)
	require.Equal(t, "manual_redeem", concurrency.SourceFamily)
	require.Equal(t, "concurrency", concurrency.Type)
	require.Equal(t, "0002", concurrency.CodeLast4)
	require.Equal(t, int64(0), concurrency.ValueCents)
	require.Equal(t, float64(70), concurrency.RawValue)
	require.NotEmpty(t, concurrency.CodeFingerprint)
}

func findRateEntry(t *testing.T, entries []ports.ProviderRateEntry, priceItem string) ports.ProviderRateEntry {
	t.Helper()
	for _, entry := range entries {
		if entry.PriceItem == priceItem {
			return entry
		}
	}
	require.Failf(t, "rate entry not found", "price_item=%s", priceItem)
	return ports.ProviderRateEntry{}
}
