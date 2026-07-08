package provider

import (
	"context"
	"encoding/json"
	"io"
	"net"
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
			require.Equal(t, " secret ", payload["password"])
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
			require.Equal(t, "42", r.Header.Get("Veloera-User"))
			require.Equal(t, "42", r.Header.Get("X-User-Id"))
			require.Equal(t, "42", r.Header.Get("neo-api-user"))
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
		Password:   " secret ",
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
	headers, ok := result.SessionBundle["required_headers"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "42", headers["New-Api-User"])
	require.Equal(t, "42", headers["Veloera-User"])
	require.Equal(t, "42", headers["X-User-Id"])
	require.Equal(t, "42", headers["neo-api-user"])

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

func TestClientDirectLoginAcceptsRegisteredUserSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/user/login":
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "signed-session", Path: "/", HttpOnly: true})
			_, _ = w.Write([]byte(`{"success":true,"message":"","data":{"id":4111,"username":"wutongci","display_name":"wutongci","role":1,"status":1,"group":"default"}}`))
		case "/api/user/self":
			require.Equal(t, "4111", r.Header.Get("New-Api-User"))
			require.Equal(t, "session=signed-session", r.Header.Get("Cookie"))
			_, _ = w.Write([]byte(`{"success":true,"message":"","data":{"id":4111,"username":"wutongci","display_name":"wutongci","role":1,"status":1,"group":"default","quota":23288,"used_quota":59976712,"request_count":12775}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(server.Client())
	result, err := client.DirectLogin(context.Background(), ports.DirectLoginInput{
		SupplierID: 7,
		Origin:     server.URL,
		APIBaseURL: server.URL,
		Username:   "wutongci",
		Password:   "secret",
		LoginContext: map[string]any{
			"source": "supplier_manual_login",
		},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	contextValue, ok := result.SessionBundle["context"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "1", contextValue["role"])
	require.Equal(t, "4111", contextValue["user_id"])
}

func TestClientDirectLoginKeepsCookieSessionWhenProfileProbeDenied(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/user/login":
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "signed-session", Path: "/", HttpOnly: true})
			_, _ = w.Write([]byte(`{"success":true,"message":"","data":{"id":4111,"username":"wutongci","display_name":"wutongci","role":1,"status":1,"group":"default"}}`))
		case "/api/user/self":
			require.Equal(t, "4111", r.Header.Get("New-Api-User"))
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"success":false,"message":"unauthorized"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(server.Client())
	result, err := client.DirectLogin(context.Background(), ports.DirectLoginInput{
		SupplierID: 7,
		Origin:     server.URL,
		APIBaseURL: server.URL,
		Username:   "wutongci",
		Password:   "secret",
	})

	require.NoError(t, err)
	require.Equal(t, "new_api", result.SessionBundle["provider_type"])
	require.Equal(t, "4111", result.SessionBundle["auth_header_value"])
	require.Equal(t, "unverified", result.Diagnostics["profile_status"])
	require.Equal(t, true, result.Diagnostics["profile_probe_failed"])
	require.Equal(t, "SUPPLIER_SESSION_PERMISSION_DENIED", result.Diagnostics["profile_probe_reason"])
	contextValue, ok := result.SessionBundle["context"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "1", contextValue["role"])
	require.Equal(t, "1", contextValue["status"])
}

func TestClientDirectLoginWithAccessTokenStoresAuthorizationAndUserHeader(t *testing.T) {
	now := time.Date(2026, 6, 22, 10, 30, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/user/self", r.URL.Path)
		require.Equal(t, "42", r.Header.Get("New-Api-User"))
		require.Equal(t, "42", r.Header.Get("Veloera-User"))
		require.Equal(t, "42", r.Header.Get("X-User-Id"))
		require.Equal(t, "new-api-access-token", r.Header.Get("Authorization"))
		require.Empty(t, r.Header.Get("Cookie"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":{"id":42,"username":"root","display_name":"Root","role":10,"status":1,"group":"default","quota":2500000,"used_quota":0,"request_count":1}}`))
	}))
	defer server.Close()

	client := NewClient(server.Client())
	client.now = func() time.Time { return now }
	result, err := client.DirectLogin(context.Background(), ports.DirectLoginInput{
		SupplierID: 7,
		Origin:     server.URL,
		APIBaseURL: server.URL,
		Token:      `{"access_token":"new-api-access-token","user_id":42}`,
	})

	require.NoError(t, err)
	require.Equal(t, "new_api", result.SessionBundle["provider_type"])
	require.Equal(t, "direct_login", result.SessionBundle["session_source"])
	require.Equal(t, "new-api-access-token", result.SessionBundle["access_token"])
	require.Equal(t, "New-Api-User", result.SessionBundle["auth_header_name"])
	require.Equal(t, "42", result.SessionBundle["auth_header_value"])
	require.Equal(t, "access_token", result.Diagnostics["login_method"])

	headers, ok := result.SessionBundle["required_headers"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "42", headers["New-Api-User"])
	require.Equal(t, "42", headers["Veloera-User"])
	require.Equal(t, "42", headers["X-User-Id"])
	contextValue, ok := result.SessionBundle["context"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "access_token", contextValue["login_method"])
	require.Equal(t, "42", contextValue["user_id"])
	require.Equal(t, "10", contextValue["role"])
	require.Equal(t, "enabled", contextValue["status"])
}

func TestClientDirectLoginUsesAccessTokenFromLoginResponseWhenCookieMissing(t *testing.T) {
	now := time.Date(2026, 6, 22, 10, 45, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/user/login":
			require.Equal(t, http.MethodPost, r.Method)
			_, _ = w.Write([]byte(`{"success":true,"message":"","data":{"id":42,"username":"root","display_name":"Root","role":10,"status":1,"group":"default","access_token":"login-access-token"}}`))
		case "/api/user/self":
			require.Equal(t, "42", r.Header.Get("New-Api-User"))
			require.Equal(t, "login-access-token", r.Header.Get("Authorization"))
			require.Empty(t, r.Header.Get("Cookie"))
			_, _ = w.Write([]byte(`{"success":true,"data":{"id":42,"username":"root","display_name":"Root","role":10,"status":1,"group":"default","quota":2500000,"used_quota":0,"request_count":1}}`))
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
		Username:   "root",
		Password:   "secret",
	})

	require.NoError(t, err)
	require.Nil(t, result.ExpiresAt)
	require.Equal(t, "login-access-token", result.SessionBundle["access_token"])
	require.Equal(t, "access_token_from_login_response", result.Diagnostics["login_method"])
	require.Equal(t, "42", result.SessionBundle["auth_header_value"])
}

func TestClientDirectLoginWithAccessTokenInfersUserIDFromJWT(t *testing.T) {
	jwt := "eyJhbGciOiJub25lIn0.eyJzdWIiOiI0MiJ9.sig"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/user/self", r.URL.Path)
		require.Equal(t, "42", r.Header.Get("New-Api-User"))
		require.Equal(t, jwt, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":{"id":42,"username":"root","display_name":"Root","role":10,"status":1,"group":"default","quota":2500000,"used_quota":0,"request_count":1}}`))
	}))
	defer server.Close()

	client := NewClient(server.Client())
	result, err := client.DirectLogin(context.Background(), ports.DirectLoginInput{
		SupplierID: 7,
		Origin:     server.URL,
		APIBaseURL: server.URL,
		Token:      jwt,
	})

	require.NoError(t, err)
	require.Equal(t, "42", result.SessionBundle["auth_header_value"])
	require.Equal(t, jwt, result.SessionBundle["access_token"])
}

func TestClientDirectLoginWithAccessTokenRetriesBearerAuthorization(t *testing.T) {
	var attempts int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		require.Equal(t, "/api/user/self", r.URL.Path)
		require.Equal(t, "42", r.Header.Get("New-Api-User"))
		w.Header().Set("Content-Type", "application/json")
		if attempts == 1 {
			require.Equal(t, "new-api-access-token", r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"success":false,"message":"invalid token"}`))
			return
		}
		require.Equal(t, "Bearer new-api-access-token", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"success":true,"data":{"id":42,"username":"root","display_name":"Root","role":10,"status":1,"group":"default","quota":2500000,"used_quota":0,"request_count":1}}`))
	}))
	defer server.Close()

	client := NewClient(server.Client())
	result, err := client.DirectLogin(context.Background(), ports.DirectLoginInput{
		SupplierID: 7,
		Origin:     server.URL,
		APIBaseURL: server.URL,
		Token:      `{"access_token":"new-api-access-token","user_id":42}`,
	})

	require.NoError(t, err)
	require.Equal(t, 2, attempts)
	require.Equal(t, "42", result.SessionBundle["auth_header_value"])
}

func TestClientProbeSelfUsesCompatibleUserHeaderVariants(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/user/self", r.URL.Path)
		require.Equal(t, "9", r.Header.Get("New-Api-User"))
		require.Equal(t, "9", r.Header.Get("Veloera-User"))
		require.Equal(t, "9", r.Header.Get("X-User-Id"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":{"id":9,"username":"alice","role":1,"status":1,"quota":2500000,"used_quota":1000000,"request_count":3}}`))
	}))
	defer server.Close()

	client := NewClient(server.Client())
	result, err := client.ProbeSub2APIUserProfile(context.Background(), ports.SessionProbeInput{
		SupplierID: 7,
		Origin:     server.URL,
		APIBaseURL: server.URL,
		Bundle: map[string]any{
			"provider_type":    "new_api",
			"required_headers": map[string]any{"Veloera-User": "9"},
		},
	})

	require.NoError(t, err)
	require.Equal(t, "new_api", result.SystemType)
	require.Equal(t, int64(9), result.Profile.ID)
}

func TestClientDirectLoginWithAccessTokenRequiresUserID(t *testing.T) {
	client := NewClient(nil)
	_, err := client.DirectLogin(context.Background(), ports.DirectLoginInput{
		SupplierID: 7,
		Origin:     "https://newapi.example.com",
		APIBaseURL: "https://newapi.example.com",
		Token:      "new-api-access-token",
	})

	require.Error(t, err)
	require.Equal(t, "SUPPLIER_DIRECT_LOGIN_NEW_API_USER_REQUIRED", infraerrors.Reason(err))
}

func TestClientProbeSelfConvertsQuotaUnitsToUSD(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/user/self", r.URL.Path)
		require.Equal(t, "9", r.Header.Get("New-Api-User"))
		require.Equal(t, "session=abc", r.Header.Get("Cookie"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":{"id":9,"username":"alice","role":1,"status":1,"quota":2500000,"used_quota":1000000,"request_count":3}}`))
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
	require.Equal(t, "USD", result.BalanceCurrency)
	require.NotNil(t, result.BalanceCents)
	require.Equal(t, int64(500), *result.BalanceCents)
	require.Equal(t, float64(5), result.Profile.Balance)
	require.Equal(t, float64(2500000), result.Diagnostics["raw_quota"])
	require.Equal(t, float64(1000000), result.Diagnostics["raw_used_quota"])
	require.Equal(t, float64(500000), result.Diagnostics["quota_units_per_usd"])
	require.Equal(t, float64(5), result.Diagnostics["quota_balance_usd"])
	require.Equal(t, float64(2), result.Diagnostics["used_quota_usd"])
	require.Equal(t, int64(3), result.Diagnostics["request_count"])
}

func TestClientProbeSelfIncludesHTTPDiagnosticsOnBadStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/user/self", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"not found"}`))
	}))
	defer server.Close()

	client := NewClient(server.Client())
	_, err := client.ProbeSub2APIUserProfile(context.Background(), ports.SessionProbeInput{
		SupplierID: 7,
		Origin:     server.URL,
		APIBaseURL: server.URL,
		Bundle: map[string]any{
			"provider_type":    "new_api",
			"required_headers": map[string]any{"New-Api-User": "9"},
		},
	})

	require.Error(t, err)
	appErr := infraerrors.FromError(err)
	require.Equal(t, "SUPPLIER_SESSION_BAD_STATUS", appErr.Reason)
	require.Equal(t, server.URL+"/api/user/self", appErr.Metadata["endpoint"])
	require.Equal(t, "404", appErr.Metadata["status_code"])
	require.Equal(t, "json", appErr.Metadata["body_type"])
	require.Contains(t, appErr.Metadata["body_excerpt"], "not found")
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

func TestClientReadGroupsFallsBackToUserGroupMap(t *testing.T) {
	var sawFallback bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "9", r.Header.Get("New-Api-User"))
		switch r.URL.Path {
		case "/api/user/self/groups":
			http.NotFound(w, r)
			_, _ = w.Write([]byte(`{"message":"not found"}`))
		case "/api/user_group_map":
			sawFallback = true
			_, _ = w.Write([]byte(`{"success":true,"data":["default","vip"]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(server.Client())
	result, err := client.ReadGroups(context.Background(), ports.SessionProbeInput{
		SupplierID: 7,
		Origin:     server.URL,
		APIBaseURL: server.URL,
		Bundle: map[string]any{
			"provider_type":    "new_api",
			"required_headers": map[string]any{"New-Api-User": "9"},
		},
	})

	require.NoError(t, err)
	require.True(t, sawFallback)
	require.ElementsMatch(t, []string{"default", "vip"}, []string{result.Groups[0].ExternalGroupID, result.Groups[1].ExternalGroupID})
}

func TestClientCreateKeyCreatesTokenSearchesAndReadsSecret(t *testing.T) {
	var created bool
	var searchCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "9", r.Header.Get("New-Api-User"))
		require.Equal(t, "session=abc", r.Header.Get("Cookie"))
		switch r.URL.Path {
		case "/api/token/search":
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "AdminPlus-kiro", r.URL.Query().Get("keyword"))
			searchCount++
			if !created {
				_, _ = w.Write([]byte(`{"success":true,"data":{"page":1,"page_size":100,"total":0,"items":[]}}`))
				return
			}
			_, _ = w.Write([]byte(`{"success":true,"data":{"page":1,"page_size":100,"total":1,"items":[{"id":701,"name":"AdminPlus-kiro","group":"kiro","status":1,"key":"abcd**********wxyz"}]}}`))
		case "/api/token/":
			require.Equal(t, http.MethodPost, r.Method)
			var payload map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
			require.Equal(t, "AdminPlus-kiro", payload["name"])
			require.Equal(t, "kiro", payload["group"])
			require.Equal(t, true, payload["unlimited_quota"])
			require.Equal(t, false, payload["cross_group_retry"])
			require.Equal(t, float64(-1), payload["expired_time"])
			created = true
			_, _ = w.Write([]byte(`{"success":true,"message":""}`))
		case "/api/token/701/key":
			require.Equal(t, http.MethodPost, r.Method)
			_, _ = w.Write([]byte(`{"success":true,"data":{"key":"raw-new-api-secret"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(server.Client())
	result, err := client.CreateKey(context.Background(), ports.SessionProbeInput{
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
	}, ports.CreateProviderKeyInput{
		SupplierID:      7,
		ExternalGroupID: "kiro",
		Name:            "AdminPlus-kiro",
	})

	require.NoError(t, err)
	require.Equal(t, 2, searchCount)
	require.Equal(t, int64(7), result.SupplierID)
	require.Equal(t, "kiro", result.ExternalGroupID)
	require.Equal(t, "701", result.ExternalKeyID)
	require.Equal(t, "AdminPlus-kiro", result.Name)
	require.Equal(t, "sk-raw-new-api-secret", result.Secret)
	require.Equal(t, "active", result.Status)
	require.NotContains(t, result.RawPayload, "key")
}

func TestClientListKeysReadsNewAPITokenList(t *testing.T) {
	var sawList bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "/api/token/", r.URL.Path)
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "9", r.Header.Get("New-Api-User"))
		require.Equal(t, "session=abc", r.Header.Get("Cookie"))
		require.Equal(t, "1", r.URL.Query().Get("p"))
		require.Equal(t, "50", r.URL.Query().Get("page_size"))
		sawList = true
		_, _ = w.Write([]byte(`{
			"success": true,
			"data": {
				"total": 2,
				"items": [
					{"id":701,"name":"ops-low","group":"g10","status":1,"key":"sk-secret","token":"hidden-token"},
					{"id":702,"name":"ops-disabled","group":"g20","status":2,"key":"sk-disabled"}
				]
			}
		}`))
	}))
	defer server.Close()

	client := NewClient(server.Client())
	result, err := client.ListKeys(context.Background(), ports.SessionProbeInput{
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
	}, ports.ListProviderKeysInput{
		SupplierID: 7,
		Limit:      50,
	})

	require.NoError(t, err)
	require.True(t, sawList)
	require.Equal(t, int64(7), result.SupplierID)
	require.Equal(t, "new_api", result.SystemType)
	require.Equal(t, 2, result.Total)
	require.Len(t, result.Keys, 2)
	require.Equal(t, "701", result.Keys[0].ExternalKeyID)
	require.Equal(t, "g10", result.Keys[0].ExternalGroupID)
	require.Equal(t, "active", result.Keys[0].Status)
	require.NotContains(t, result.Keys[0].RawPayload, "key")
	require.NotContains(t, result.Keys[0].RawPayload, "token")
	require.Equal(t, "702", result.Keys[1].ExternalKeyID)
	require.Equal(t, "disabled", result.Keys[1].Status)
	require.NotContains(t, result.Keys[1].RawPayload, "key")
}

func TestClientReadKeyCapacityReadsPaginatedNewAPITokens(t *testing.T) {
	pages := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "/api/token/", r.URL.Path)
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "9", r.Header.Get("New-Api-User"))
		require.Equal(t, "session=abc", r.Header.Get("Cookie"))
		require.Equal(t, "2", r.URL.Query().Get("page_size"))
		page := r.URL.Query().Get("p")
		pages = append(pages, page)
		switch page {
		case "1":
			_, _ = w.Write([]byte(`{
				"success": true,
				"data": {
					"total": 3,
					"items": [
						{"id":701,"name":"ops-low","group":"g10","status":1,"key":"sk-secret","token":"hidden-token"},
						{"id":702,"name":"ops-disabled","group":"g20","status":2,"key":"sk-disabled"}
					]
				}
			}`))
		case "2":
			_, _ = w.Write([]byte(`{
				"success": true,
				"data": {
					"total": 3,
					"items": [
						{"id":703,"name":"ops-mid","group":"g30","status":1,"key":"sk-mid"}
					]
				}
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(server.Client())
	result, err := client.ReadKeyCapacity(context.Background(), ports.SessionProbeInput{
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
	}, ports.ReadProviderKeyCapacityInput{
		SupplierID: 7,
		Limit:      2,
	})

	require.NoError(t, err)
	require.Equal(t, []string{"1", "2"}, pages)
	require.Equal(t, int64(7), result.SupplierID)
	require.Equal(t, "new_api", result.SystemType)
	require.Equal(t, "unknown", result.KeyLimitPolicy)
	require.False(t, result.LimitKnown)
	require.Equal(t, 2, result.ActiveKeyCount)
	require.Equal(t, "unknown", result.KeyCapacityStatus)
	require.Len(t, result.Keys, 3)
	require.Equal(t, "g30", result.Keys[2].ExternalGroupID)
	require.NotContains(t, result.Keys[0].RawPayload, "key")
	require.NotContains(t, result.Keys[0].RawPayload, "token")
	require.Equal(t, "list_keys", result.Diagnostics["capacity_source"])
	require.Equal(t, "not_exposed_by_provider", result.Diagnostics["limit_source"])
	require.Equal(t, 2, result.Diagnostics["pages_read"])
	require.Equal(t, 3, result.Diagnostics["provider_total"])
	require.NotEqual(t, true, result.Diagnostics["truncated"])
}

func TestParseNewAPITokenListSupportsResponseVariants(t *testing.T) {
	tests := []map[string]any{
		{"data": map[string]any{"items": []any{map[string]any{"id": float64(701), "name": "AdminPlus", "group": "default"}}}},
		{"data": map[string]any{"list": []any{map[string]any{"id": float64(702), "name": "AdminPlus", "group": "default"}}}},
		{"data": []any{map[string]any{"id": float64(703), "name": "AdminPlus", "group": "default"}}},
		{"items": []any{map[string]any{"id": float64(704), "name": "AdminPlus", "group": "default"}}},
		{"list": []any{map[string]any{"id": float64(705), "name": "AdminPlus", "group": "default"}}},
	}
	for _, payload := range tests {
		tokens := parseNewAPITokenList(payload)
		require.Len(t, tokens, 1)
		require.Equal(t, "AdminPlus", tokens[0].Name)
	}
}

func TestClientDisableKeyUsesNewAPIStatusOnlyTokenUpdate(t *testing.T) {
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "/api/token/", r.URL.Path)
		require.Equal(t, "true", r.URL.Query().Get("status_only"))
		require.Equal(t, http.MethodPut, r.Method)
		require.Equal(t, "9", r.Header.Get("New-Api-User"))
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		_, _ = w.Write([]byte(`{"success":true,"data":{"id":701,"name":"AdminPlus-kiro","group":"kiro","status":2}}`))
	}))
	defer server.Close()

	client := NewClient(server.Client())
	result, err := client.DisableKey(context.Background(), ports.SessionProbeInput{
		SupplierID: 7,
		APIBaseURL: server.URL,
		Bundle: map[string]any{
			"provider_type":    "new_api",
			"required_headers": map[string]any{"New-Api-User": "9"},
		},
	}, ports.DisableProviderKeyInput{
		SupplierID:      7,
		ExternalGroupID: "kiro",
		ExternalKeyID:   "701",
		Name:            "AdminPlus-kiro",
	})

	require.NoError(t, err)
	require.Equal(t, float64(701), payload["id"])
	require.Equal(t, float64(2), payload["status"])
	require.Equal(t, "701", result.ExternalKeyID)
	require.Equal(t, "disabled", result.Status)
}

func TestClientDeleteKeyUsesNewAPITokenDelete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "/api/token/701", r.URL.Path)
		require.Equal(t, http.MethodDelete, r.Method)
		require.Equal(t, "9", r.Header.Get("New-Api-User"))
		_, _ = w.Write([]byte(`{"success":true,"message":""}`))
	}))
	defer server.Close()

	client := NewClient(server.Client())
	result, err := client.DeleteKey(context.Background(), ports.SessionProbeInput{
		SupplierID: 7,
		APIBaseURL: server.URL,
		Bundle: map[string]any{
			"provider_type":    "new_api",
			"required_headers": map[string]any{"New-Api-User": "9"},
		},
	}, ports.DeleteProviderKeyInput{
		SupplierID:      7,
		ExternalGroupID: "kiro",
		ExternalKeyID:   "701",
		Name:            "AdminPlus-kiro",
	})

	require.NoError(t, err)
	require.Equal(t, "701", result.ExternalKeyID)
	require.Equal(t, "deleted", result.Status)
}

func TestClientCreateKeyConvertsQuotaUSDToNewAPIUnits(t *testing.T) {
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "9", r.Header.Get("New-Api-User"))
		switch r.URL.Path {
		case "/api/token/search":
			_, _ = w.Write([]byte(`{"success":true,"data":{"items":[]}}`))
		case "/api/token/":
			require.Equal(t, http.MethodPost, r.Method)
			require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
			_, _ = w.Write([]byte(`{"success":true,"message":""}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(server.Client())
	_, err := client.CreateKey(context.Background(), ports.SessionProbeInput{
		SupplierID: 7,
		Origin:     server.URL,
		APIBaseURL: server.URL,
		Bundle: map[string]any{
			"provider_type":    "new_api",
			"required_headers": map[string]any{"New-Api-User": "9"},
		},
	}, ports.CreateProviderKeyInput{
		SupplierID:      7,
		ExternalGroupID: "kiro",
		Name:            "AdminPlus-kiro",
		QuotaUSD:        5,
	})

	require.Error(t, err)
	require.Equal(t, false, payload["unlimited_quota"])
	require.Equal(t, float64(2500000), payload["remain_quota"])
}

func TestClientReadFundingTransactionsReadsTopupOrders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/user/topup", r.URL.Path)
		require.Equal(t, "1", r.URL.Query().Get("p"))
		require.Equal(t, "100", r.URL.Query().Get("page_size"))
		require.Equal(t, "9", r.Header.Get("New-Api-User"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"success": true,
			"data": {
				"items": [
					{
						"id": 701,
						"amount": 100,
						"money": 10,
						"trade_no": "USR9NOabc",
						"payment_method": "redeem",
						"payment_provider": "redemption",
						"create_time": 1782122400,
						"complete_time": 1782122523,
						"status": "success"
					}
				]
			}
		}`))
	}))
	defer server.Close()

	client := NewClient(server.Client())
	result, err := client.ReadFundingTransactions(context.Background(), ports.SessionProbeInput{
		SupplierID: 7,
		Origin:     server.URL,
		APIBaseURL: server.URL,
		Bundle: map[string]any{
			"provider_type":    "new_api",
			"required_headers": map[string]any{"New-Api-User": "9"},
		},
	}, ports.ReadFundingTransactionsInput{SupplierID: 7})

	require.NoError(t, err)
	require.Equal(t, "new_api", result.ProviderType)
	require.Len(t, result.Items, 1)
	item := result.Items[0]
	require.Equal(t, "USR9NOabc", item.ExternalID)
	require.Equal(t, "USR9NOabc", item.OutTradeNo)
	require.Equal(t, "redeem", item.PaymentType)
	require.Equal(t, "redemption", item.OrderType)
	require.Equal(t, "COMPLETED", item.Status)
	require.Equal(t, "USD", item.Currency)
	require.Equal(t, int64(10000), item.AmountCents)
	require.Equal(t, int64(1000), item.CashAmountCents)
	require.NotNil(t, item.CompletedAt)
	require.Equal(t, time.Unix(1782122523, 0).UTC(), *item.CompletedAt)
}

func TestClientReadUsageCostsReadsConsumeLogsByPage(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/log", r.URL.Path)
		require.Equal(t, "100", r.URL.Query().Get("page_size"))
		require.Equal(t, "2", r.URL.Query().Get("type"))
		require.Equal(t, "1782122400", r.URL.Query().Get("start_timestamp"))
		require.Equal(t, "1782126000", r.URL.Query().Get("end_timestamp"))
		require.Equal(t, "9", r.Header.Get("New-Api-User"))
		requests++
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("p") {
		case "1":
			_, _ = w.Write([]byte(`{
				"success": true,
				"data": {
					"page": 1,
					"page_size": 100,
					"total": 2,
					"items": [
						{
							"id": 901,
							"created_at": 1782122500,
							"type": 2,
							"model_name": "gpt-5.4-mini",
							"quota": 250000,
							"prompt_tokens": 100,
							"completion_tokens": 40,
							"use_time": 2,
							"token_name": "ops-key",
							"group": "PRO",
							"request_id": "req-901",
							"upstream_request_id": "up-901"
						}
					]
				}
			}`))
		case "2":
			_, _ = w.Write([]byte(`{
				"success": true,
				"data": {
					"page": 2,
					"page_size": 100,
					"total": 2,
					"items": [
						{
							"id": 902,
							"created_at": 1782122600,
							"type": 2,
							"model_name": "claude-opus-4",
							"quota": 500000,
							"prompt_tokens": 200,
							"completion_tokens": 120,
							"use_time": 1500,
							"token_name": "ops-key",
							"group": "Claude",
							"request_id": "req-902"
						}
					]
				}
			}`))
		default:
			t.Fatalf("unexpected page %q", r.URL.Query().Get("p"))
		}
	}))
	defer server.Close()

	client := NewClient(server.Client())
	result, err := client.ReadUsageCosts(context.Background(), ports.SessionProbeInput{
		SupplierID: 7,
		Origin:     server.URL,
		APIBaseURL: server.URL,
		Bundle: map[string]any{
			"provider_type":    "new_api",
			"required_headers": map[string]any{"New-Api-User": "9"},
		},
	}, ports.ReadUsageCostsInput{
		SupplierID: 7,
		StartedAt:  time.Unix(1782122400, 0).UTC(),
		EndedAt:    time.Unix(1782126000, 0).UTC(),
	})

	require.NoError(t, err)
	require.Equal(t, 2, requests)
	require.Equal(t, "new_api", result.SystemType)
	require.Len(t, result.Lines, 2)
	require.Equal(t, "901", result.Lines[0].ExternalUsageCostID)
	require.Equal(t, "req-901", result.Lines[0].ExternalRequestID)
	require.Equal(t, "gpt-5.4-mini", result.Lines[0].Model)
	require.Equal(t, "PRO", result.Lines[0].ReasoningEffort)
	require.Equal(t, int64(50), result.Lines[0].CostCents)
	require.Equal(t, int64(100), result.Lines[0].InputTokens)
	require.Equal(t, int64(40), result.Lines[0].OutputTokens)
	require.Equal(t, int64(140), result.Lines[0].TotalTokens)
	require.Equal(t, int64(2000), result.Lines[0].DurationMS)
	require.NotNil(t, result.Lines[0].EndedAt)
	require.Equal(t, time.Unix(1782122502, 0).UTC(), *result.Lines[0].EndedAt)
	require.Equal(t, int64(100), result.Lines[1].CostCents)
	require.Equal(t, int64(1500), result.Lines[1].DurationMS)
}

func TestClientReadUsageCostsDoesNotPreflightFailForRegisteredUserRole(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		require.Equal(t, "/api/log", r.URL.Path)
		require.Equal(t, "2", r.URL.Query().Get("type"))
		require.Equal(t, "9", r.Header.Get("New-Api-User"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":{"page":1,"page_size":100,"total":0,"items":[]}}`))
	}))
	defer server.Close()

	client := NewClient(server.Client())
	result, err := client.ReadUsageCosts(context.Background(), ports.SessionProbeInput{
		SupplierID: 7,
		Origin:     server.URL,
		APIBaseURL: server.URL,
		Bundle: map[string]any{
			"provider_type":    "new_api",
			"required_headers": map[string]any{"New-Api-User": "9"},
			"context":          map[string]any{"role": "1"},
		},
	}, ports.ReadUsageCostsInput{
		SupplierID: 7,
		StartedAt:  time.Unix(1782122400, 0).UTC(),
		EndedAt:    time.Unix(1782126000, 0).UTC(),
	})

	require.NoError(t, err)
	require.Equal(t, 1, requests)
	require.Empty(t, result.Lines)
}

func TestClientReadEntitlementTransactionsReadsRedemptions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/redemption/", r.URL.Path)
		require.Equal(t, "1", r.URL.Query().Get("p"))
		require.Equal(t, "100", r.URL.Query().Get("page_size"))
		require.Equal(t, "9", r.Header.Get("New-Api-User"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"success": true,
			"data": {
				"page": 1,
				"page_size": 100,
				"total": 1,
				"items": [
					{
						"id": 801,
						"key": "PAYabc1234",
						"status": 3,
						"quota": 500000,
						"created_time": 1782122000,
						"redeemed_time": 1782122600,
						"used_user_id": 9
					}
				]
			}
		}`))
	}))
	defer server.Close()

	client := NewClient(server.Client())
	result, err := client.ReadEntitlementTransactions(context.Background(), ports.SessionProbeInput{
		SupplierID: 7,
		Origin:     server.URL,
		APIBaseURL: server.URL,
		Bundle: map[string]any{
			"provider_type":    "new_api",
			"required_headers": map[string]any{"New-Api-User": "9"},
			"context":          map[string]any{"role": "10"},
		},
	}, ports.ReadEntitlementTransactionsInput{SupplierID: 7})

	require.NoError(t, err)
	require.Equal(t, "new_api", result.ProviderType)
	require.Len(t, result.Items, 1)
	item := result.Items[0]
	require.Equal(t, "801", item.ExternalID)
	require.Equal(t, "1234", item.CodeLast4)
	require.Equal(t, "payment_auto_redeem", item.SourceFamily)
	require.Equal(t, "balance", item.Type)
	require.Equal(t, "used", item.Status)
	require.Equal(t, int64(100), item.ValueCents)
	require.NotNil(t, item.UsedAt)
	require.Equal(t, time.Unix(1782122600, 0).UTC(), *item.UsedAt)
	require.NotContains(t, item.RawPayload, "key")
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

func TestClientRegisterAccountDirectSuccess(t *testing.T) {
	var sawStatus bool
	var sawRegister bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/status":
			require.Equal(t, http.MethodGet, r.Method)
			sawStatus = true
			_, _ = w.Write([]byte(`{"success":true,"data":{"register_enabled":true,"password_register_enabled":true,"email_verification":false,"turnstile_check":false}}`))
		case "/api/user/register":
			require.Equal(t, http.MethodPost, r.Method)
			var payload map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
			require.Equal(t, "ops", payload["username"])
			require.Equal(t, "ops@example.com", payload["email"])
			require.Equal(t, " secret ", payload["password"])
			require.Equal(t, "", payload["verification_code"])
			sawRegister = true
			_, _ = w.Write([]byte(`{"success":true,"message":"","data":{"id":42,"username":"ops"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(server.Client())
	result, err := client.RegisterAccount(context.Background(), ports.DirectRegistrationInput{
		ProviderType: "new_api",
		Origin:       server.URL + "/login",
		APIBaseURL:   server.URL,
		Email:        "ops@example.com",
		Password:     " secret ",
		Username:     "ops",
	})

	require.NoError(t, err)
	require.True(t, sawStatus)
	require.True(t, sawRegister)
	require.Equal(t, ports.DirectRegistrationStageCompleted, result.Stage)
	require.True(t, result.Submitted)
	require.Equal(t, server.URL, result.Origin)
	require.Equal(t, server.URL, result.APIBaseURL)
}

func TestClientRegisterAccountRequestErrorIncludesDiagnostics(t *testing.T) {
	client := NewClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return nil, &net.DNSError{Err: "no such host", Name: req.URL.Host}
		}),
	})

	_, err := client.RegisterAccount(context.Background(), ports.DirectRegistrationInput{
		ProviderType: "new_api",
		Origin:       "https://missing.example/sign-up",
		APIBaseURL:   "https://missing.example",
		Email:        "ops@example.com",
		Password:     " secret ",
	})

	require.Error(t, err)
	require.Equal(t, "SUPPLIER_DIRECT_REGISTRATION_FAILED", infraerrors.Reason(err))
	appErr := infraerrors.FromError(err)
	require.Equal(t, "https://missing.example/api/user/register", appErr.Metadata["endpoint"])
	require.Equal(t, "dns", appErr.Metadata["error_kind"])
	require.Contains(t, appErr.Metadata["error_detail"], "no such host")
	require.NotContains(t, appErr.Metadata["error_detail"], "ops@example.com")
	require.NotContains(t, appErr.Metadata["error_detail"], " secret ")
}

func TestClientRegisterAccountRequestsEmailVerificationCode(t *testing.T) {
	var sawVerification bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/status":
			_, _ = w.Write([]byte(`{"success":true,"data":{"register_enabled":true,"password_register_enabled":true,"email_verification":true,"turnstile_check":false,"system_name":"大模型云算力Token","site_name":"大模型云算力"}}`))
		case "/api/verification":
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "ops@example.com", r.URL.Query().Get("email"))
			sawVerification = true
			_, _ = w.Write([]byte(`{"success":true,"message":"","data":{}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(server.Client())
	result, err := client.RegisterAccount(context.Background(), ports.DirectRegistrationInput{
		ProviderType: "new_api",
		Origin:       server.URL,
		APIBaseURL:   server.URL,
		Email:        "ops@example.com",
		Password:     "secret",
	})

	require.NoError(t, err)
	require.True(t, sawVerification)
	require.Equal(t, ports.DirectRegistrationStageNeedEmailCode, result.Stage)
	require.True(t, result.EmailCodeRequired)
	require.Equal(t, "大模型云算力Token", result.Diagnostics["system_name"])
	require.Equal(t, "大模型云算力", result.Diagnostics["site_name"])
}

func TestClientRegisterAccountRetriesTransientStatusEOF(t *testing.T) {
	var statusAttempts int
	var sawVerification bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/status":
			statusAttempts++
			if statusAttempts < 3 {
				panic(http.ErrAbortHandler)
			}
			_, _ = w.Write([]byte(`{"success":true,"data":{"register_enabled":true,"password_register_enabled":true,"email_verification":true,"turnstile_check":false}}`))
		case "/api/verification":
			sawVerification = true
			_, _ = w.Write([]byte(`{"success":true,"message":"","data":{}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(server.Client())
	result, err := client.RegisterAccount(context.Background(), ports.DirectRegistrationInput{
		ProviderType: "new_api",
		Origin:       server.URL,
		APIBaseURL:   server.URL,
		Email:        "ops@example.com",
		Password:     "secret",
	})

	require.NoError(t, err)
	require.Equal(t, 3, statusAttempts)
	require.True(t, sawVerification)
	require.Equal(t, ports.DirectRegistrationStageNeedEmailCode, result.Stage)
}

func TestClientRegisterAccountUsesRegisterEndpointWhenStatusProbeTimesOut(t *testing.T) {
	statusStarted := make(chan struct{})
	releaseStatus := make(chan struct{})
	var sawRegister bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/status":
			_, _ = w.Write([]byte(`{"success":true,"data":{"email_verification":true,"logo":"`))
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			close(statusStarted)
			<-releaseStatus
		case "/api/user/register":
			sawRegister = true
			_, _ = w.Write([]byte(`{"success":false,"message":"管理员关闭了新用户注册"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	defer close(releaseStatus)

	httpClient := server.Client()
	httpClient.Timeout = 250 * time.Millisecond
	client := NewClient(httpClient)
	_, err := client.RegisterAccount(context.Background(), ports.DirectRegistrationInput{
		ProviderType: "new_api",
		Origin:       server.URL,
		APIBaseURL:   server.URL,
		Email:        "ops@example.com",
		Password:     "secret",
	})

	require.Error(t, err)
	require.True(t, sawRegister)
	require.Equal(t, "REGISTRATION_DISABLED", infraerrors.Reason(err))
}

func TestClientRegisterAccountRequestsCodeWhenRegisterEndpointRequiresVerification(t *testing.T) {
	var sawRegister bool
	var sawVerification bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/status":
			_, _ = w.Write([]byte(`{"success":true,"data":{"logo":"tiny"}}`))
		case "/api/user/register":
			sawRegister = true
			_, _ = w.Write([]byte(`{"success":false,"message":"管理员开启了邮箱验证，请输入邮箱地址和验证码"}`))
		case "/api/verification":
			sawVerification = true
			require.Equal(t, "ops@example.com", r.URL.Query().Get("email"))
			_, _ = w.Write([]byte(`{"success":true,"message":""}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(server.Client())
	result, err := client.RegisterAccount(context.Background(), ports.DirectRegistrationInput{
		ProviderType: "new_api",
		Origin:       server.URL,
		APIBaseURL:   server.URL,
		Email:        "ops@example.com",
		Password:     "secret",
	})

	require.NoError(t, err)
	require.True(t, sawRegister)
	require.True(t, sawVerification)
	require.Equal(t, ports.DirectRegistrationStageNeedEmailCode, result.Stage)
	require.True(t, result.EmailCodeRequired)
}

func TestClientRegisterAccountDecodesStatusBeforeConnectionClose(t *testing.T) {
	statusStarted := make(chan struct{})
	releaseStatus := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/status":
			_, _ = w.Write([]byte(`{"success":true,"data":{"register_enabled":false,"password_register_enabled":false,"email_verification":true,"turnstile_check":false,"system_name":"HTH"}}`))
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			close(statusStarted)
			<-releaseStatus
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	defer close(releaseStatus)

	httpClient := server.Client()
	httpClient.Timeout = 5 * time.Second
	client := NewClient(httpClient)
	errCh := make(chan error, 1)
	go func() {
		_, err := client.RegisterAccount(context.Background(), ports.DirectRegistrationInput{
			ProviderType: "new_api",
			Origin:       server.URL,
			APIBaseURL:   server.URL,
			Email:        "ops@example.com",
			Password:     "secret",
		})
		errCh <- err
	}()

	<-statusStarted
	select {
	case err := <-errCh:
		require.Error(t, err)
		require.Equal(t, "REGISTRATION_DISABLED", infraerrors.Reason(err))
	case <-time.After(time.Second):
		t.Fatal("expected status JSON to be decoded without waiting for connection close")
	}
}

func TestClientRegisterAccountTurnstileStatusDoesNotSkipEmailVerification(t *testing.T) {
	var sawVerification bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/status":
			_, _ = w.Write([]byte(`{"success":true,"data":{"register_enabled":true,"password_register_enabled":true,"email_verification":true,"turnstile_check":true}}`))
		case "/api/verification":
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "ops@example.com", r.URL.Query().Get("email"))
			sawVerification = true
			_, _ = w.Write([]byte(`{"success":true,"message":"","data":{}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(server.Client())
	result, err := client.RegisterAccount(context.Background(), ports.DirectRegistrationInput{
		ProviderType: "new_api",
		Origin:       server.URL + "/sign-up",
		APIBaseURL:   server.URL,
		Email:        "ops@example.com",
		Password:     "secret",
	})

	require.NoError(t, err)
	require.True(t, sawVerification)
	require.Equal(t, ports.DirectRegistrationStageNeedEmailCode, result.Stage)
	require.True(t, result.EmailCodeRequired)
	require.Equal(t, true, result.Diagnostics["turnstile_check"])
}

func TestClientRegisterAccountTurnstileStatusDoesNotSkipRegistrationSubmit(t *testing.T) {
	var sawRegister bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/status":
			_, _ = w.Write([]byte(`{"success":true,"data":{"register_enabled":true,"password_register_enabled":true,"email_verification":false,"turnstile_check":true}}`))
		case "/api/user/register":
			require.Equal(t, http.MethodPost, r.Method)
			sawRegister = true
			_, _ = w.Write([]byte(`{"success":true,"message":"","data":{"id":42,"username":"ops"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(server.Client())
	result, err := client.RegisterAccount(context.Background(), ports.DirectRegistrationInput{
		ProviderType: "new_api",
		Origin:       server.URL + "/sign-up",
		APIBaseURL:   server.URL,
		Email:        "ops@example.com",
		Password:     "secret",
	})

	require.NoError(t, err)
	require.True(t, sawRegister)
	require.Equal(t, ports.DirectRegistrationStageCompleted, result.Stage)
	require.True(t, result.Submitted)
	require.Equal(t, true, result.Diagnostics["turnstile_check"])
}

func TestClientRegisterAccountEmailVerificationMessageDoesNotRequireBrowserFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/status":
			_, _ = w.Write([]byte(`{"success":true,"data":{"register_enabled":true,"password_register_enabled":true,"email_verification":true,"turnstile_check":false}}`))
		case "/api/verification":
			_, _ = w.Write([]byte(`{"success":false,"message":"邮箱验证失败，请稍后重试"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(server.Client())
	_, err := client.RegisterAccount(context.Background(), ports.DirectRegistrationInput{
		ProviderType: "new_api",
		Origin:       server.URL,
		APIBaseURL:   server.URL,
		Email:        "ops@example.com",
		Password:     "secret",
	})

	require.Error(t, err)
	require.Equal(t, "SUPPLIER_VERIFICATION_CODE_FAILED", infraerrors.Reason(err))
}

func TestIsRetryableRegistrationStatusError(t *testing.T) {
	require.True(t, isRetryableRegistrationStatusError(io.EOF))
	require.False(t, isRetryableRegistrationStatusError(context.DeadlineExceeded))
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
