package adminplus

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	sub2apiprovider "github.com/Wei-Shaw/sub2api/internal/adminplus/adapters/sub2api/provider"
	actionsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/actions"
	balancesapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/balances"
	billingapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/billing"
	extensionapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/extension"
	notificationsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/notifications"
	ratesapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/rates"
	sessionsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/sessions"
	suppliergroupsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/suppliergroups"
	supplierkeysapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/supplierkeys"
	suppliersapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/suppliers"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestBillingHandlerImportBillLines(t *testing.T) {
	router := newOperationsHandlerTestRouter()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/billing/lines/import", bytes.NewBufferString(`{
		"lines": [
			{
				"supplier_id": 7,
				"source": "Chrome",
				"external_bill_id": "bill-1",
				"external_request_id": "req-1",
				"model": "gpt-4o-mini",
				"currency": "usd",
				"cost_cents": 123,
				"input_tokens": 1000,
				"output_tokens": 500,
				"started_at": "2026-06-20T10:00:00Z",
				"ended_at": "2026-06-20T10:00:02Z",
				"raw_payload": {"file":"daily.csv"}
			}
		]
	}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var body struct {
		Code int `json:"code"`
		Data struct {
			Total int `json:"total"`
			Items []struct {
				ID       int64  `json:"id"`
				Source   string `json:"source"`
				Currency string `json:"currency"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, 1, body.Data.Total)
	require.Equal(t, int64(1), body.Data.Items[0].ID)
	require.Equal(t, "chrome", body.Data.Items[0].Source)
	require.Equal(t, "USD", body.Data.Items[0].Currency)

	list := httptest.NewRecorder()
	listReq := httptest.NewRequest(http.MethodGet, "/billing/lines?supplier_id=7", nil)
	router.ServeHTTP(list, listReq)
	require.Equal(t, http.StatusOK, list.Code, list.Body.String())
	require.Contains(t, list.Body.String(), `"external_request_id":"req-1"`)
}

func TestExtensionHandlerTaskLifecycle(t *testing.T) {
	router := newOperationsHandlerTestRouter()

	created := performJSON(t, router, http.MethodPost, "/extension/tasks", `{
		"supplier_id": 7,
		"type": "export_bills",
		"priority": 10,
		"payload": {"page":"bills"}
	}`)
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())

	claimed := performJSON(t, router, http.MethodPost, "/extension/tasks/claim", `{
		"device_id": "chrome-1",
		"types": ["export_bills"],
		"lease_ttl_seconds": 60
	}`)
	require.Equal(t, http.StatusOK, claimed.Code, claimed.Body.String())
	var claimBody struct {
		Code int `json:"code"`
		Data struct {
			ID         int64  `json:"id"`
			Status     string `json:"status"`
			LeaseToken string `json:"lease_token"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(claimed.Body.Bytes(), &claimBody))
	require.Equal(t, int64(1), claimBody.Data.ID)
	require.Equal(t, "claimed", claimBody.Data.Status)
	require.NotEmpty(t, claimBody.Data.LeaseToken)

	heartbeat := performJSON(t, router, http.MethodPost, "/extension/tasks/1/heartbeat", `{
		"device_id": "chrome-1",
		"lease_token": "`+claimBody.Data.LeaseToken+`",
		"lease_ttl_seconds": 60
	}`)
	require.Equal(t, http.StatusOK, heartbeat.Code, heartbeat.Body.String())

	completed := performJSON(t, router, http.MethodPost, "/extension/tasks/1/complete", `{
		"device_id": "chrome-1",
		"lease_token": "`+claimBody.Data.LeaseToken+`",
		"result": {"file":"bill.csv"}
	}`)
	require.Equal(t, http.StatusOK, completed.Code, completed.Body.String())
	require.Contains(t, completed.Body.String(), `"status":"succeeded"`)
}

func TestExtensionHandlerBrowserCredentialRequiresLease(t *testing.T) {
	router := newOperationsHandlerTestRouter()

	created := performJSON(t, router, http.MethodPost, "/extension/tasks", `{
		"supplier_id": 1,
		"type": "fetch_rates"
	}`)
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())

	claimed := performJSON(t, router, http.MethodPost, "/extension/tasks/claim", `{
		"device_id": "chrome-1",
		"lease_ttl_seconds": 60
	}`)
	require.Equal(t, http.StatusOK, claimed.Code, claimed.Body.String())
	var claimBody struct {
		Data struct {
			LeaseToken string `json:"lease_token"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(claimed.Body.Bytes(), &claimBody))

	wrongLease := performJSON(t, router, http.MethodPost, "/extension/tasks/1/browser-credential", `{
		"device_id": "chrome-1",
		"lease_token": "bad-token"
	}`)
	require.Equal(t, http.StatusConflict, wrongLease.Code, wrongLease.Body.String())

	credential := performJSON(t, router, http.MethodPost, "/extension/tasks/1/browser-credential", `{
		"device_id": "chrome-1",
		"lease_token": "`+claimBody.Data.LeaseToken+`"
	}`)
	require.Equal(t, http.StatusOK, credential.Code, credential.Body.String())
	require.Contains(t, credential.Body.String(), `"username":"ops@example.com"`)
	require.Contains(t, credential.Body.String(), `"password":"secret"`)
}

func TestExtensionHandlerCreateCaptureSessionTask(t *testing.T) {
	router := newOperationsHandlerTestRouter()

	created := performJSON(t, router, http.MethodPost, "/extension/session/capture-task", `{
		"supplier_id": 1,
		"device_id": "chrome-1",
		"lease_ttl_seconds": 60,
		"payload": {
			"source_url": "https://relay.example.com/dashboard",
			"source_host": "relay.example.com"
		}
	}`)

	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())
	var body struct {
		Data struct {
			ID          int64  `json:"id"`
			Type        string `json:"type"`
			Status      string `json:"status"`
			DeviceID    string `json:"device_id"`
			LeaseToken  string `json:"lease_token"`
			MaxAttempts int    `json:"max_attempts"`
			Payload     struct {
				SourceHost string `json:"source_host"`
			} `json:"payload"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(created.Body.Bytes(), &body))
	require.Equal(t, int64(1), body.Data.ID)
	require.Equal(t, "capture_supplier_session", body.Data.Type)
	require.Equal(t, "claimed", body.Data.Status)
	require.Equal(t, "chrome-1", body.Data.DeviceID)
	require.NotEmpty(t, body.Data.LeaseToken)
	require.Equal(t, 1, body.Data.MaxAttempts)
	require.Equal(t, "relay.example.com", body.Data.Payload.SourceHost)
}

func TestSupplierHandlerMatchSite(t *testing.T) {
	router := newOperationsHandlerTestRouter()

	rec := performJSON(t, router, http.MethodPost, "/suppliers/site-match", `{
		"url": "https://relay.example.com/admin/accounts"
	}`)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"status":"matched"`)
	require.Contains(t, rec.Body.String(), `"name":"Relay"`)
}

func TestCaptureSessionStoresSanitizedSession(t *testing.T) {
	router := newOperationsHandlerTestRouter()

	created := performJSON(t, router, http.MethodPost, "/extension/session/capture-task", `{
		"supplier_id": 1,
		"device_id": "chrome-1",
		"lease_ttl_seconds": 60
	}`)
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())
	var body struct {
		Data struct {
			ID         int64  `json:"id"`
			LeaseToken string `json:"lease_token"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(created.Body.Bytes(), &body))

	completed := performJSON(t, router, http.MethodPost, "/extension/tasks/"+strconv.FormatInt(body.Data.ID, 10)+"/complete", `{
		"device_id": "chrome-1",
		"lease_token": "`+body.Data.LeaseToken+`",
		"result": {
			"session_bundle": {
				"origin": "https://relay.example.com",
				"access_token": "secret-token",
				"context": {"api_base_url": "https://relay.example.com"}
			}
		}
	}`)
	require.Equal(t, http.StatusOK, completed.Code, completed.Body.String())
	require.NotContains(t, completed.Body.String(), "secret-token")

	session := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/suppliers/1/session", nil)
	router.ServeHTTP(session, req)
	require.Equal(t, http.StatusOK, session.Code, session.Body.String())
	require.Contains(t, session.Body.String(), `"has_encrypted_bundle":true`)
	require.NotContains(t, session.Body.String(), "secret-token")
}

func TestCaptureSessionRejectsMismatchedHost(t *testing.T) {
	router := newOperationsHandlerTestRouter()

	created := performJSON(t, router, http.MethodPost, "/extension/session/capture-task", `{
		"supplier_id": 1,
		"device_id": "chrome-1",
		"lease_ttl_seconds": 60
	}`)
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())
	var body struct {
		Data struct {
			ID         int64  `json:"id"`
			LeaseToken string `json:"lease_token"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(created.Body.Bytes(), &body))

	completed := performJSON(t, router, http.MethodPost, "/extension/tasks/"+strconv.FormatInt(body.Data.ID, 10)+"/complete", `{
		"device_id": "chrome-1",
		"lease_token": "`+body.Data.LeaseToken+`",
		"result": {
			"session_bundle": {
				"origin": "https://evil.example.com",
				"access_token": "secret-token",
				"context": {"api_base_url": "https://evil.example.com"}
			}
		}
	}`)
	require.Equal(t, http.StatusBadRequest, completed.Code, completed.Body.String())
	require.Contains(t, completed.Body.String(), "SUPPLIER_SESSION_HOST_NOT_ALLOWED")
	require.NotContains(t, completed.Body.String(), "secret-token")
}

func TestSupplierSessionProbeReadsProfileAndRecordsBalance(t *testing.T) {
	var seenAuth string
	var seenCookie string
	var seenOrigin string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		require.Equal(t, "/api/v1/user/profile", req.URL.Path)
		seenAuth = req.Header.Get("Authorization")
		seenCookie = req.Header.Get("Cookie")
		seenOrigin = req.Header.Get("Origin")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": {
				"id": 9,
				"email": "ops@example.com",
				"status": "active",
				"balance": 12.34,
				"concurrency": 5,
				"allowed_groups": [1, 2]
			}
		}`))
	}))
	defer server.Close()

	router := newOperationsHandlerTestRouterWithSupplierURLs(server.URL, server.URL)

	created := performJSON(t, router, http.MethodPost, "/extension/session/capture-task", `{
		"supplier_id": 1,
		"device_id": "chrome-1",
		"lease_ttl_seconds": 60
	}`)
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())
	var body struct {
		Data struct {
			ID         int64  `json:"id"`
			LeaseToken string `json:"lease_token"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(created.Body.Bytes(), &body))

	completed := performJSON(t, router, http.MethodPost, "/extension/tasks/"+strconv.FormatInt(body.Data.ID, 10)+"/complete", `{
		"device_id": "chrome-1",
		"lease_token": "`+body.Data.LeaseToken+`",
		"result": {
			"session_bundle": {
				"origin": "`+server.URL+`",
				"tokens": {
					"access_token": "secret-token",
					"csrf_token": "csrf-token"
				},
				"required_headers": {
					"cookie": "sid=secret-cookie",
					"origin": "`+server.URL+`",
					"referer": "`+server.URL+`/dashboard"
				},
				"context": {"api_base_url": "`+server.URL+`"}
			}
		}
	}`)
	require.Equal(t, http.StatusOK, completed.Code, completed.Body.String())

	probed := performJSON(t, router, http.MethodPost, "/suppliers/1/session/probe", `{
		"low_balance_threshold_cents": 2000
	}`)
	require.Equal(t, http.StatusOK, probed.Code, probed.Body.String())
	require.Equal(t, "Bearer secret-token", seenAuth)
	require.Equal(t, "sid=secret-cookie", seenCookie)
	require.Equal(t, server.URL, seenOrigin)
	require.Contains(t, probed.Body.String(), `"system_type":"sub2api"`)
	require.Contains(t, probed.Body.String(), `"balance_cents":1234`)
	require.Contains(t, probed.Body.String(), `"balance_snapshot"`)
	require.NotContains(t, probed.Body.String(), "secret-token")
	require.NotContains(t, probed.Body.String(), "secret-cookie")
}

func TestSupplierBrowserSessionDirectUpsertSanitizesSession(t *testing.T) {
	router := newOperationsHandlerTestRouter()

	created := performJSON(t, router, http.MethodPost, "/suppliers/1/browser-sessions", `{
		"origin": "https://relay.example.com",
		"api_base_url": "https://relay.example.com/api/v1",
		"session_bundle": {
			"origin": "https://relay.example.com",
			"access_token": "direct-secret-token",
			"cookies": [
				{"name": "sid", "value": "direct-secret-cookie"}
			],
			"context": {"api_base_url": "https://relay.example.com/api/v1"}
		}
	}`)

	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())
	require.Contains(t, created.Body.String(), `"has_encrypted_bundle":true`)
	require.Contains(t, created.Body.String(), `"has_access_token":true`)
	require.Contains(t, created.Body.String(), `"cookie_count":1`)
	require.NotContains(t, created.Body.String(), "direct-secret-token")
	require.NotContains(t, created.Body.String(), "direct-secret-cookie")
}

func TestSupplierSessionProbeUsesCookieArray(t *testing.T) {
	var seenCookie string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		seenCookie = req.Header.Get("Cookie")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"id":9,"balance":1.23}}`))
	}))
	defer server.Close()

	router := newOperationsHandlerTestRouterWithSupplierURLs(server.URL, server.URL)

	created := performJSON(t, router, http.MethodPost, "/suppliers/1/browser-sessions", `{
		"origin": "`+server.URL+`",
		"session_bundle": {
			"origin": "`+server.URL+`",
			"cookies": [
				{"name": "sid", "value": "secret-cookie"},
				{"name": "theme", "value": "dark"}
			],
			"context": {"api_base_url": "`+server.URL+`"}
		}
	}`)
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())

	probed := performJSON(t, router, http.MethodPost, "/suppliers/1/session/probe", `{}`)
	require.Equal(t, http.StatusOK, probed.Code, probed.Body.String())
	require.Equal(t, "sid=secret-cookie; theme=dark", seenCookie)
	require.Contains(t, probed.Body.String(), `"balance_cents":123`)
	require.NotContains(t, probed.Body.String(), "secret-cookie")
}

func TestSupplierGroupSyncReadsProviderSessionAndListsGroups(t *testing.T) {
	var seenAvailableAuth string
	var seenRatesCookie string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch req.URL.Path {
		case "/api/v1/groups/available":
			seenAvailableAuth = req.Header.Get("Authorization")
			_, _ = w.Write([]byte(`{
				"data": [
					{
						"id": 10,
						"name": "GPT-5.5 Low Cost",
						"description": "low cost group",
						"platform": "openai",
						"rate_multiplier": 0.8,
						"rpm_limit": 120,
						"is_exclusive": false,
						"status": "active"
					}
				]
			}`))
		case "/api/v1/groups/rates":
			seenRatesCookie = req.Header.Get("Cookie")
			_, _ = w.Write([]byte(`{"data":{"10":0.65}}`))
		default:
			http.NotFound(w, req)
		}
	}))
	defer server.Close()

	router := newOperationsHandlerTestRouterWithSupplierURLs(server.URL, server.URL)
	created := performJSON(t, router, http.MethodPost, "/suppliers/1/browser-sessions", `{
		"origin": "`+server.URL+`",
		"session_bundle": {
			"origin": "`+server.URL+`",
			"tokens": {"access_token": "secret-token"},
			"required_headers": {"cookie": "sid=secret-cookie"},
			"context": {"api_base_url": "`+server.URL+`"}
		}
	}`)
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())

	synced := performJSON(t, router, http.MethodPost, "/suppliers/1/groups/sync", `{}`)
	require.Equal(t, http.StatusCreated, synced.Code, synced.Body.String())
	require.Equal(t, "Bearer secret-token", seenAvailableAuth)
	require.Equal(t, "sid=secret-cookie", seenRatesCookie)
	require.Contains(t, synced.Body.String(), `"external_group_id":"10"`)
	require.Contains(t, synced.Body.String(), `"effective_rate_multiplier":0.65`)
	require.NotContains(t, synced.Body.String(), "secret-token")
	require.NotContains(t, synced.Body.String(), "secret-cookie")

	listed := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/suppliers/1/groups?page=1&page_size=10", nil)
	router.ServeHTTP(listed, req)
	require.Equal(t, http.StatusOK, listed.Code, listed.Body.String())
	require.Contains(t, listed.Body.String(), `"total":1`)
	require.Contains(t, listed.Body.String(), `"name":"GPT-5.5 Low Cost"`)
}

func TestSupplierRateSyncReadsProviderSession(t *testing.T) {
	var seenAuth string
	var seenCookie string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch req.URL.Path {
		case "/api/v1/rates/snapshots":
			_, _ = w.Write([]byte(`{"data":[]}`))
		case "/api/v1/channels/available":
			seenAuth = req.Header.Get("Authorization")
			seenCookie = req.Header.Get("Cookie")
			_, _ = w.Write([]byte(`{
				"data": [
					{
						"name": "OpenAI",
						"supported_models": [
							{
								"name": "gpt-5-mini",
								"pricing": {
									"billing_mode": "token",
									"input_price": 0.0000015,
									"output_price_micros": 6000000
								}
							}
						]
					}
				]
			}`))
		default:
			http.NotFound(w, req)
		}
	}))
	defer server.Close()

	router := newOperationsHandlerTestRouterWithSupplierURLs(server.URL, server.URL)
	created := performJSON(t, router, http.MethodPost, "/suppliers/1/browser-sessions", `{
		"origin": "`+server.URL+`",
		"session_bundle": {
			"origin": "`+server.URL+`",
			"tokens": {"access_token": "secret-token"},
			"required_headers": {"cookie": "sid=secret-cookie"},
			"context": {"api_base_url": "`+server.URL+`"}
		}
	}`)
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())

	synced := performJSON(t, router, http.MethodPost, "/suppliers/1/rates/sync", `{}`)
	require.Equal(t, http.StatusCreated, synced.Code, synced.Body.String())
	require.Equal(t, "Bearer secret-token", seenAuth)
	require.Equal(t, "sid=secret-cookie", seenCookie)
	require.Contains(t, synced.Body.String(), `"source":"provider_session"`)
	require.Contains(t, synced.Body.String(), `"model":"gpt-5-mini"`)
	require.Contains(t, synced.Body.String(), `"price_item":"input"`)
	require.Contains(t, synced.Body.String(), `"price_micros":1500000`)
	require.Contains(t, synced.Body.String(), `"price_item":"output"`)
	require.Contains(t, synced.Body.String(), `"price_micros":6000000`)
	require.NotContains(t, synced.Body.String(), "secret-token")
	require.NotContains(t, synced.Body.String(), "secret-cookie")
}

func TestSupplierKeyProvisionCreatesProviderKeyLocalAccountAndBinding(t *testing.T) {
	var seenAuth string
	var seenCookie string
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		require.Equal(t, http.MethodPost, req.Method)
		require.Equal(t, "/api/v1/api-keys", req.URL.Path)
		seenAuth = req.Header.Get("Authorization")
		seenCookie = req.Header.Get("Cookie")
		require.NoError(t, json.NewDecoder(req.Body).Decode(&payload))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": {
				"id": 99,
				"name": "ops-key",
				"key": "sk-provider-secret",
				"group_id": 10,
				"status": "active"
			}
		}`))
	}))
	defer server.Close()

	router := newOperationsHandlerTestRouterWithSupplierURLs(server.URL, server.URL)
	created := performJSON(t, router, http.MethodPost, "/suppliers/1/browser-sessions", `{
		"origin": "`+server.URL+`",
		"session_bundle": {
			"origin": "`+server.URL+`",
			"tokens": {"access_token": "secret-token"},
			"required_headers": {"cookie": "sid=secret-cookie"},
			"context": {"api_base_url": "`+server.URL+`"}
		}
	}`)
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())

	provisioned := performJSON(t, router, http.MethodPost, "/suppliers/1/keys/provision", `{
		"supplier_group_id": 10,
		"name": "ops-key",
		"quota_usd": 25,
		"local_account_platform": "openai",
		"local_account_name": "local-upstream",
		"local_account_base_url": "`+server.URL+`/v1",
		"local_account_concurrency": 3,
		"local_account_priority": 40,
		"balance_currency": "USD"
	}`)
	require.Equal(t, http.StatusCreated, provisioned.Code, provisioned.Body.String())
	require.Equal(t, "Bearer secret-token", seenAuth)
	require.Equal(t, "sid=secret-cookie", seenCookie)
	require.Equal(t, "ops-key", payload["name"])
	require.Equal(t, float64(10), payload["group_id"])
	require.Equal(t, float64(25), payload["quota"])
	require.Contains(t, provisioned.Body.String(), `"status":"bound"`)
	require.Contains(t, provisioned.Body.String(), `"external_key_id":"99"`)
	require.Contains(t, provisioned.Body.String(), `"key_last4":"cret"`)
	require.Contains(t, provisioned.Body.String(), `"supplier_key_id":1`)
	require.Contains(t, provisioned.Body.String(), `"local_sub2api_account_id":1001`)
	require.NotContains(t, provisioned.Body.String(), "sk-provider-secret")
	require.NotContains(t, provisioned.Body.String(), "secret-token")
	require.NotContains(t, provisioned.Body.String(), "secret-cookie")

	listed := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/suppliers/1/keys?page=1&page_size=10", nil)
	router.ServeHTTP(listed, req)
	require.Equal(t, http.StatusOK, listed.Code, listed.Body.String())
	require.Contains(t, listed.Body.String(), `"total":1`)
	require.Contains(t, listed.Body.String(), `"external_key_id":"99"`)
	require.NotContains(t, listed.Body.String(), "sk-provider-secret")
}

func TestExtensionHandlerManifestAndPackage(t *testing.T) {
	router := newOperationsHandlerTestRouter()

	manifest := httptest.NewRecorder()
	manifestReq := httptest.NewRequest(http.MethodGet, "/extension/manifest", nil)
	router.ServeHTTP(manifest, manifestReq)

	require.Equal(t, http.StatusOK, manifest.Code, manifest.Body.String())
	require.Contains(t, manifest.Body.String(), `"name":"Sub2API Plus Session Capture"`)
	require.Contains(t, manifest.Body.String(), `"permissions"`)

	archive := httptest.NewRecorder()
	archiveReq := httptest.NewRequest(http.MethodGet, "/extension/package.zip?admin_plus_origin=https%3A%2F%2Fsub2api-plus.example.com%2Fadmin%2Foperations%2Fscheduler", nil)
	router.ServeHTTP(archive, archiveReq)

	require.Equal(t, http.StatusOK, archive.Code, archive.Body.String())
	require.Equal(t, "application/zip", archive.Header().Get("Content-Type"))
	require.Contains(t, archive.Header().Get("Content-Disposition"), "sub2api-plus-session-capture-")
	zipReader, err := zip.NewReader(bytes.NewReader(archive.Body.Bytes()), int64(archive.Body.Len()))
	require.NoError(t, err)
	var manifestJSON string
	var defaultConfigJSON string
	for _, file := range zipReader.File {
		if file.Name != "manifest.json" && file.Name != extensionDefaultConfigPath {
			continue
		}
		reader, err := file.Open()
		require.NoError(t, err)
		data, err := io.ReadAll(reader)
		require.NoError(t, err)
		require.NoError(t, reader.Close())
		if file.Name == "manifest.json" {
			manifestJSON = string(data)
		}
		if file.Name == extensionDefaultConfigPath {
			defaultConfigJSON = string(data)
		}
	}
	require.Contains(t, manifestJSON, `"Sub2API Plus Session Capture"`)
	require.JSONEq(t, `{"baseURL":"https://sub2api-plus.example.com"}`, defaultConfigJSON)
}

func TestExtensionHandlerPackageRejectsInvalidOrigin(t *testing.T) {
	router := newOperationsHandlerTestRouter()

	archive := httptest.NewRecorder()
	archiveReq := httptest.NewRequest(http.MethodGet, "/extension/package.zip?admin_plus_origin=javascript%3Aalert(1)", nil)
	router.ServeHTTP(archive, archiveReq)

	require.Equal(t, http.StatusBadRequest, archive.Code, archive.Body.String())
}

func TestExtensionHandlerPackageInfersOriginFromRequest(t *testing.T) {
	router := newOperationsHandlerTestRouter()

	archive := httptest.NewRecorder()
	archiveReq := httptest.NewRequest(http.MethodGet, "/extension/package.zip", nil)
	archiveReq.Header.Set("X-Forwarded-Proto", "https")
	archiveReq.Header.Set("X-Forwarded-Host", "sub2api-plus.example.com")
	router.ServeHTTP(archive, archiveReq)

	require.Equal(t, http.StatusOK, archive.Code, archive.Body.String())
	zipReader, err := zip.NewReader(bytes.NewReader(archive.Body.Bytes()), int64(archive.Body.Len()))
	require.NoError(t, err)
	defaultConfigJSON := zipEntryText(t, zipReader, extensionDefaultConfigPath)
	require.JSONEq(t, `{"baseURL":"https://sub2api-plus.example.com"}`, defaultConfigJSON)
}

func TestActionHandlerGenerateDoesNotExecuteActions(t *testing.T) {
	router := newOperationsHandlerTestRouter()

	rec := performJSON(t, router, http.MethodPost, "/actions/generate", `{
		"suppliers": [
			{
				"supplier_id": 1,
				"runtime_status": "active",
				"health_status": "normal",
				"balance_cents": 0,
				"effective_cost_cents": 100
			},
			{
				"supplier_id": 2,
				"runtime_status": "candidate",
				"health_status": "normal",
				"balance_cents": 5000,
				"effective_cost_cents": 80
			}
		]
	}`)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body struct {
		Code int `json:"code"`
		Data struct {
			Total int `json:"total"`
			Items []struct {
				Type             string `json:"type"`
				ReasonCode       string `json:"reason_code"`
				RequiresApproval bool   `json:"requires_approval"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.GreaterOrEqual(t, body.Data.Total, 2)
	require.Equal(t, "pause_supplier", body.Data.Items[0].Type)
	require.True(t, body.Data.Items[0].RequiresApproval)
	require.Contains(t, rec.Body.String(), `"type":"switch_supplier"`)
}

func TestNotificationHandlerListDeliveries(t *testing.T) {
	router := newOperationsHandlerTestRouter()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/notifications/deliveries?supplier_id=7&status=failed", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"event_type":"health.request_error"`)
	require.Contains(t, rec.Body.String(), `"last_error":"webhook failed"`)
}

func newOperationsHandlerTestRouter() *gin.Engine {
	return newOperationsHandlerTestRouterWithSupplierURLs("https://relay.example.com", "https://relay.example.com")
}

func newOperationsHandlerTestRouterWithSupplierURLs(dashboardURL string, apiBaseURL string) *gin.Engine {
	gin.SetMode(gin.TestMode)

	supplierRepo := suppliersapp.NewMemoryRepository()
	supplierSvc := suppliersapp.NewService(supplierRepo)
	_, _ = supplierSvc.Create(context.Background(), suppliersapp.CreateSupplierInput{
		Name:                 "Relay",
		Kind:                 adminplusdomain.SupplierKindRelay,
		Type:                 adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus:        adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:         adminplusdomain.SupplierHealthStatusNormal,
		DashboardURL:         dashboardURL,
		APIBaseURL:           apiBaseURL,
		BrowserLoginEnabled:  true,
		BrowserLoginUsername: "ops@example.com",
		BrowserLoginPassword: "secret",
	})
	billingHandler := NewBillingHandler(billingapp.NewService(billingapp.NewMemoryRepository()))
	sessionSvc := sessionsapp.NewServiceWithDependencies(
		sessionsapp.NewMemoryRepository(),
		plainSessionCipher{},
		supplierSvc,
		sub2apiprovider.NewSessionProfileClient(nil),
	)
	sessionGroupClient := sub2apiprovider.NewSessionProfileClient(http.DefaultClient)
	rateService := ratesapp.NewServiceWithDependencies(
		newOperationsRateRepository(),
		nil,
		sessionSvc,
		sessionGroupClient,
	)
	supplierGroupHandler := NewSupplierGroupHandler(suppliergroupsapp.NewService(
		suppliergroupsapp.NewMemoryRepository(),
		sessionSvc,
		sessionGroupClient,
	))
	keyRepo := supplierkeysapp.NewMemoryRepository()
	keyRepo.PutSupplier(&adminplusdomain.Supplier{
		ID:            1,
		Name:          "Relay",
		Type:          adminplusdomain.SupplierTypeSub2API,
		RuntimeStatus: adminplusdomain.SupplierRuntimeStatusMonitorOnly,
		HealthStatus:  adminplusdomain.SupplierHealthStatusNormal,
		DashboardURL:  dashboardURL,
		APIBaseURL:    apiBaseURL,
	})
	keyRepo.PutGroup(&adminplusdomain.SupplierGroup{
		ID:              10,
		SupplierID:      1,
		ExternalGroupID: "10",
		Name:            "GPT-5.5 Low Cost",
		ProviderFamily:  "openai",
		Status:          adminplusdomain.SupplierGroupStatusActive,
	})
	supplierKeyHandler := NewSupplierKeyHandler(supplierkeysapp.NewService(
		keyRepo,
		sessionSvc,
		sub2apiprovider.NewSessionProfileClient(http.DefaultClient),
		&operationsLocalAccountCreator{},
	))
	processor := extensionapp.NewIngestProcessorWithCipher(nil, nil, nil, nil, nil, sessionSvc, plainSessionCipher{})
	extensionHandler := NewExtensionHandler(extensionapp.NewServiceWithDependencies(extensionapp.NewMemoryRepository(), processor, supplierSvc))
	actionHandler := NewActionHandler(actionsapp.NewRuleService())
	notificationRepo := notificationsapp.NewMemoryRepository()
	_, _, _ = notificationRepo.CreateDelivery(context.Background(), &adminplusdomain.NotificationDelivery{
		Channel:    adminplusdomain.NotificationChannelFeishu,
		EventType:  "health.request_error",
		EventID:    31,
		SupplierID: 7,
		DedupeKey:  "feishu:health.request_error:31",
		Status:     adminplusdomain.NotificationStatusFailed,
		LastError:  "webhook failed",
		Payload:    map[string]any{"msg_type": "text"},
	})
	notificationHandler := NewNotificationHandler(notificationRepo)
	supplierHandler := NewSupplierHandler(supplierSvc)
	balanceHandler := NewBalanceHandler(balancesapp.NewService(balancesapp.NewMemoryRepository()))
	rateHandler := NewRateHandler(rateService)
	sessionHandler := NewSessionHandler(sessionSvc, balanceHandler.service)

	router := gin.New()
	router.POST("/suppliers/site-match", supplierHandler.MatchSite)
	router.GET("/suppliers/:id/session", sessionHandler.Get)
	router.POST("/suppliers/:id/session/probe", sessionHandler.Probe)
	router.POST("/suppliers/:id/browser-sessions", sessionHandler.Upsert)
	router.GET("/suppliers/:id/groups", supplierGroupHandler.List)
	router.POST("/suppliers/:id/groups/sync", supplierGroupHandler.Sync)
	router.GET("/suppliers/:id/keys", supplierKeyHandler.List)
	router.POST("/suppliers/:id/keys/provision", supplierKeyHandler.Provision)
	router.POST("/suppliers/:id/rates/sync", rateHandler.SyncSupplierRates)
	router.POST("/billing/lines/import", billingHandler.ImportBillLines)
	router.GET("/billing/lines", billingHandler.ListBillLines)
	router.POST("/extension/tasks", extensionHandler.CreateTask)
	router.POST("/extension/tasks/claim", extensionHandler.ClaimTask)
	router.POST("/extension/session/capture-task", extensionHandler.CreateCaptureSessionTask)
	router.GET("/extension/manifest", extensionHandler.Manifest)
	router.GET("/extension/package.zip", extensionHandler.DownloadPackage)
	router.POST("/extension/tasks/:id/heartbeat", extensionHandler.Heartbeat)
	router.POST("/extension/tasks/:id/browser-credential", extensionHandler.GetBrowserCredential)
	router.POST("/extension/tasks/:id/complete", extensionHandler.CompleteTask)
	router.POST("/extension/tasks/:id/fail", extensionHandler.FailTask)
	router.GET("/extension/tasks", extensionHandler.ListTasks)
	router.POST("/actions/generate", actionHandler.Generate)
	router.GET("/notifications/deliveries", notificationHandler.ListDeliveries)
	return router
}

func performJSON(t *testing.T, router *gin.Engine, method string, path string, payload string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	return rec
}

type plainSessionCipher struct{}

type operationsRateRepository struct {
	nextSnapshotID int64
	nextEventID    int64
	snapshots      []*adminplusdomain.RateSnapshot
	events         []*adminplusdomain.RateChangeEvent
}

type operationsLocalAccountCreator struct{}

func (c *operationsLocalAccountCreator) CreateAccount(_ context.Context, input *service.CreateAccountInput) (*service.Account, error) {
	return &service.Account{
		ID:          1001,
		Name:        input.Name,
		Platform:    input.Platform,
		Type:        input.Type,
		Credentials: input.Credentials,
	}, nil
}

func newOperationsRateRepository() *operationsRateRepository {
	return &operationsRateRepository{nextSnapshotID: 1, nextEventID: 1}
}

func (r *operationsRateRepository) CreateSnapshot(_ context.Context, snapshot *adminplusdomain.RateSnapshot) (*adminplusdomain.RateSnapshot, error) {
	cp := cloneOperationsRateSnapshot(snapshot)
	cp.ID = r.nextSnapshotID
	r.nextSnapshotID++
	if cp.CreatedAt.IsZero() {
		cp.CreatedAt = cp.CapturedAt
	}
	r.snapshots = append(r.snapshots, cp)
	return cloneOperationsRateSnapshot(cp), nil
}

func (r *operationsRateRepository) FindLatestComparableSnapshot(_ context.Context, snapshot *adminplusdomain.RateSnapshot) (*adminplusdomain.RateSnapshot, error) {
	var latest *adminplusdomain.RateSnapshot
	for _, item := range r.snapshots {
		if item.SupplierID != snapshot.SupplierID ||
			item.Model != snapshot.Model ||
			item.BillingMode != snapshot.BillingMode ||
			item.PriceItem != snapshot.PriceItem ||
			item.Unit != snapshot.Unit ||
			item.Currency != snapshot.Currency {
			continue
		}
		if latest == nil || item.CapturedAt.After(latest.CapturedAt) || (item.CapturedAt.Equal(latest.CapturedAt) && item.ID > latest.ID) {
			latest = item
		}
	}
	return cloneOperationsRateSnapshot(latest), nil
}

func (r *operationsRateRepository) CreateChangeEvent(_ context.Context, event *adminplusdomain.RateChangeEvent) (*adminplusdomain.RateChangeEvent, error) {
	cp := *event
	cp.ID = r.nextEventID
	r.nextEventID++
	r.events = append(r.events, &cp)
	return &cp, nil
}

func (r *operationsRateRepository) ListSnapshots(_ context.Context, _ ratesapp.SnapshotFilter) ([]*adminplusdomain.RateSnapshot, error) {
	items := make([]*adminplusdomain.RateSnapshot, 0, len(r.snapshots))
	for _, item := range r.snapshots {
		items = append(items, cloneOperationsRateSnapshot(item))
	}
	return items, nil
}

func (r *operationsRateRepository) ListChangeEvents(_ context.Context, _ ratesapp.EventFilter) ([]*adminplusdomain.RateChangeEvent, error) {
	items := make([]*adminplusdomain.RateChangeEvent, 0, len(r.events))
	for _, item := range r.events {
		cp := *item
		items = append(items, &cp)
	}
	return items, nil
}

func (r *operationsRateRepository) UpdateChangeEventStatus(_ context.Context, id int64, status adminplusdomain.RateChangeStatus) (*adminplusdomain.RateChangeEvent, error) {
	for _, event := range r.events {
		if event.ID == id {
			event.Status = status
			cp := *event
			return &cp, nil
		}
	}
	return nil, nil
}

func cloneOperationsRateSnapshot(in *adminplusdomain.RateSnapshot) *adminplusdomain.RateSnapshot {
	if in == nil {
		return nil
	}
	out := *in
	if in.RawPayload != nil {
		out.RawPayload = make(map[string]any, len(in.RawPayload))
		for key, value := range in.RawPayload {
			out.RawPayload[key] = value
		}
	}
	return &out
}

func (plainSessionCipher) Encrypt(plaintext string) (string, error) {
	return "encrypted:" + plaintext, nil
}

func (plainSessionCipher) Decrypt(ciphertext string) (string, error) {
	return strings.TrimPrefix(ciphertext, "encrypted:"), nil
}

func zipEntryText(t *testing.T, zipReader *zip.Reader, name string) string {
	t.Helper()
	for _, file := range zipReader.File {
		if file.Name != name {
			continue
		}
		reader, err := file.Open()
		require.NoError(t, err)
		data, err := io.ReadAll(reader)
		require.NoError(t, err)
		require.NoError(t, reader.Close())
		return string(data)
	}
	t.Fatalf("zip entry %q not found", name)
	return ""
}
