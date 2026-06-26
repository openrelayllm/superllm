package adminplus

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	sub2apiprovider "github.com/Wei-Shaw/sub2api/internal/adminplus/adapters/sub2api/provider"
	actionsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/actions"
	balancesapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/balances"
	costsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/costs"
	extensionapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/extension"
	notificationsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/notifications"
	provisionjobsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/provisionjobs"
	ratesapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/rates"
	sessionsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/sessions"
	suppliergroupsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/suppliergroups"
	supplierkeysapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/supplierkeys"
	suppliersapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/suppliers"
	usagecostsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/usagecosts"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUsageCostHandlerImportUsageCostLines(t *testing.T) {
	router := newOperationsHandlerTestRouter()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/usage-costs/lines/import", bytes.NewBufferString(`{
		"lines": [
			{
				"supplier_id": 7,
				"source": "Chrome",
				"external_usage_cost_id": "bill-1",
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
	listReq := httptest.NewRequest(http.MethodGet, "/usage-costs/lines?supplier_id=7", nil)
	router.ServeHTTP(list, listReq)
	require.Equal(t, http.StatusOK, list.Code, list.Body.String())
	require.Contains(t, list.Body.String(), `"external_request_id":"req-1"`)
}

func TestExtensionHandlerTaskLifecycle(t *testing.T) {
	router := newOperationsHandlerTestRouter()

	created := performJSON(t, router, http.MethodPost, "/extension/tasks", `{
		"supplier_id": 7,
		"type": "fetch_usage_costs",
		"priority": 10,
		"payload": {"page":"bills"}
	}`)
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())

	claimed := performJSON(t, router, http.MethodPost, "/extension/tasks/claim", `{
		"device_id": "chrome-1",
		"types": ["fetch_usage_costs"],
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

	missingSupplier := performJSON(t, router, http.MethodPost, "/extension/session/capture-task", `{
		"device_id": "chrome-1",
		"lease_ttl_seconds": 60,
		"url": "https://relay.example.com/dashboard"
	}`)
	require.Equal(t, http.StatusBadRequest, missingSupplier.Code, missingSupplier.Body.String())
	require.Contains(t, missingSupplier.Body.String(), "SUPPLIER_ID_REQUIRED")
}

func TestExtensionHandlerReportSupplierCandidateCreatesSupplierWithCredential(t *testing.T) {
	router := newOperationsHandlerTestRouter()

	reported := performJSON(t, router, http.MethodPost, "/extension/suppliers/report-candidate", `{
		"device_id": "chrome-1",
		"name": "AI Pixel",
		"kind": "relay",
		"type": "new_api",
		"runtime_status": "monitor_only",
		"health_status": "normal",
		"dashboard_url": "https://pixel.example.com/dashboard",
		"api_base_url": "https://pixel.example.com",
		"source_url": "https://pixel.example.com/dashboard",
		"source_host": "pixel.example.com",
		"contact": "ops",
		"notes": "from extension",
		"balance_cents": 1234,
		"balance_currency": "USD",
		"recharge_multiplier": 2,
		"browser_login_enabled": true,
		"browser_login_username": "pixel@example.com",
		"browser_login_password": "pixel-secret",
		"browser_login_token": "pixel-token"
	}`)
	require.Equal(t, http.StatusOK, reported.Code, reported.Body.String())
	var reportBody struct {
		Data struct {
			SupplierID      int64  `json:"supplier_id"`
			SupplierName    string `json:"supplier_name"`
			Created         bool   `json:"created"`
			CredentialSaved bool   `json:"credential_saved"`
			MaskedUsername  string `json:"masked_username"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(reported.Body.Bytes(), &reportBody))
	require.Greater(t, reportBody.Data.SupplierID, int64(1))
	require.Equal(t, "AI Pixel", reportBody.Data.SupplierName)
	require.True(t, reportBody.Data.Created)
	require.True(t, reportBody.Data.CredentialSaved)
	require.Equal(t, "pi***@example.com", reportBody.Data.MaskedUsername)

	created := performJSON(t, router, http.MethodPost, "/extension/session/capture-task", `{
		"supplier_id": `+strconv.FormatInt(reportBody.Data.SupplierID, 10)+`,
		"device_id": "chrome-1",
		"lease_ttl_seconds": 60
	}`)
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())
	var createdBody struct {
		Data struct {
			ID         int64  `json:"id"`
			LeaseToken string `json:"lease_token"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(created.Body.Bytes(), &createdBody))

	credential := performJSON(t, router, http.MethodPost, "/extension/tasks/"+strconv.FormatInt(createdBody.Data.ID, 10)+"/browser-credential", `{
		"device_id": "chrome-1",
		"lease_token": "`+createdBody.Data.LeaseToken+`"
	}`)
	require.Equal(t, http.StatusOK, credential.Code, credential.Body.String())
	require.Contains(t, credential.Body.String(), `"username":"pixel@example.com"`)
	require.Contains(t, credential.Body.String(), `"password":"pixel-secret"`)
	require.Contains(t, credential.Body.String(), `"token":"pixel-token"`)
}

func TestExtensionHandlerReportSupplierCandidateRequiresRegisteredCredential(t *testing.T) {
	router := newOperationsHandlerTestRouter()

	reported := performJSON(t, router, http.MethodPost, "/extension/suppliers/report-candidate", `{
		"device_id": "chrome-1",
		"name": "Unregistered Relay",
		"type": "new_api",
		"dashboard_url": "https://unregistered.example.com/dashboard",
		"api_base_url": "https://unregistered.example.com",
		"source_url": "https://unregistered.example.com/dashboard",
		"source_host": "unregistered.example.com",
		"auto_create_supplier": true
	}`)

	require.Equal(t, http.StatusConflict, reported.Code, reported.Body.String())
	require.Contains(t, reported.Body.String(), "SUPPLIER_SITE_REGISTRATION_REQUIRED")
}

func TestExtensionHandlerReportSupplierCandidateIgnoresExistingSupplierWithoutUpdating(t *testing.T) {
	router := newOperationsHandlerTestRouter()

	reported := performJSON(t, router, http.MethodPost, "/extension/suppliers/report-candidate", `{
		"device_id": "chrome-1",
		"name": "Changed Relay",
		"type": "new_api",
		"dashboard_url": "https://relay.example.com/changed",
		"api_base_url": "https://relay.example.com/changed-api",
		"source_url": "https://relay.example.com/admin/accounts",
		"source_host": "relay.example.com",
		"balance_cents": 999999,
		"browser_login_enabled": true,
		"browser_login_username": "changed@example.com",
		"browser_login_password": "changed-secret",
		"browser_login_token": "changed-token"
	}`)
	require.Equal(t, http.StatusOK, reported.Code, reported.Body.String())
	require.Contains(t, reported.Body.String(), `"supplier_id":1`)
	require.Contains(t, reported.Body.String(), `"already_exists":true`)
	require.Contains(t, reported.Body.String(), `"ignored":true`)
	require.Contains(t, reported.Body.String(), `"credential_saved":false`)

	created := performJSON(t, router, http.MethodPost, "/extension/session/capture-task", `{
		"supplier_id": 1,
		"device_id": "chrome-1",
		"lease_ttl_seconds": 60
	}`)
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())
	var createdBody struct {
		Data struct {
			ID         int64  `json:"id"`
			LeaseToken string `json:"lease_token"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(created.Body.Bytes(), &createdBody))

	credential := performJSON(t, router, http.MethodPost, "/extension/tasks/"+strconv.FormatInt(createdBody.Data.ID, 10)+"/browser-credential", `{
		"device_id": "chrome-1",
		"lease_token": "`+createdBody.Data.LeaseToken+`"
	}`)
	require.Equal(t, http.StatusOK, credential.Code, credential.Body.String())
	require.Contains(t, credential.Body.String(), `"supplier_name":"Relay"`)
	require.Contains(t, credential.Body.String(), `"username":"ops@example.com"`)
	require.Contains(t, credential.Body.String(), `"password":"secret"`)
	require.NotContains(t, credential.Body.String(), "changed-secret")
	require.NotContains(t, credential.Body.String(), "changed-token")
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
	profileCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch req.URL.Path {
		case "/api/v1/user/profile":
			seenAuth = req.Header.Get("Authorization")
			seenCookie = req.Header.Get("Cookie")
			seenOrigin = req.Header.Get("Origin")
			profileCalls++
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
		default:
			writeOperationsSub2APICapabilityFixture(t, w, req)
		}
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
	require.Equal(t, 1, profileCalls)
	require.Equal(t, "Bearer secret-token", seenAuth)
	require.Equal(t, "sid=secret-cookie", seenCookie)
	require.Equal(t, server.URL, seenOrigin)
	require.Contains(t, probed.Body.String(), `"system_type":"sub2api"`)
	require.Contains(t, probed.Body.String(), `"balance_cents":1234`)
	require.Contains(t, probed.Body.String(), `"balance_snapshot"`)
	require.Contains(t, probed.Body.String(), `"can_read_groups":true`)
	require.Contains(t, probed.Body.String(), `"can_create_key":true`)
	require.NotContains(t, probed.Body.String(), "secret-token")
	require.NotContains(t, probed.Body.String(), "secret-cookie")
}

func TestCaptureSessionUsesReportedSupplierWithoutSyncingBalance(t *testing.T) {
	profileCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch req.URL.Path {
		case "/api/v1/user/profile":
			profileCalls++
			_, _ = w.Write([]byte(`{
				"code": 0,
				"message": "success",
				"data": {
					"id": 8850,
					"email": "wutongci@qq.com",
					"role": "user",
					"status": "active",
					"balance": 1.8347894,
					"concurrency": 100
				}
			}`))
		default:
			writeOperationsSub2APICapabilityFixture(t, w, req)
		}
	}))
	defer server.Close()

	router := newOperationsHandlerTestRouterWithAutoSessionBalanceSync("", "")

	reported := performJSON(t, router, http.MethodPost, "/extension/suppliers/report-candidate", `{
		"device_id": "chrome-1",
		"name": "AI Pixel",
		"type": "sub2api",
		"dashboard_url": "`+server.URL+`/dashboard",
		"api_base_url": "`+server.URL+`",
		"source_url": "`+server.URL+`/dashboard",
		"source_host": "`+strings.TrimPrefix(server.URL, "http://")+`",
		"auto_create_supplier": true,
		"browser_login_enabled": true,
		"browser_login_username": "registered@example.com",
		"browser_login_password": "registered-secret"
	}`)
	require.Equal(t, http.StatusOK, reported.Code, reported.Body.String())
	var reportBody struct {
		Data struct {
			SupplierID int64 `json:"supplier_id"`
			Created    bool  `json:"created"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(reported.Body.Bytes(), &reportBody))
	require.Greater(t, reportBody.Data.SupplierID, int64(1))
	require.True(t, reportBody.Data.Created)

	created := performJSON(t, router, http.MethodPost, "/extension/session/capture-task", `{
		"supplier_id": `+strconv.FormatInt(reportBody.Data.SupplierID, 10)+`,
		"device_id": "chrome-1",
		"lease_ttl_seconds": 60,
		"url": "`+server.URL+`/dashboard",
		"dashboard_url": "`+server.URL+`/dashboard",
		"api_base_url": "`+server.URL+`"
	}`)
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())
	var body struct {
		Data struct {
			ID         int64          `json:"id"`
			SupplierID int64          `json:"supplier_id"`
			LeaseToken string         `json:"lease_token"`
			Payload    map[string]any `json:"payload"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(created.Body.Bytes(), &body))
	require.Equal(t, reportBody.Data.SupplierID, body.Data.SupplierID)
	require.NotContains(t, body.Data.Payload, "supplier_auto_created")

	completed := performJSON(t, router, http.MethodPost, "/extension/tasks/"+strconv.FormatInt(body.Data.ID, 10)+"/complete", `{
		"device_id": "chrome-1",
		"lease_token": "`+body.Data.LeaseToken+`",
		"result": {
			"session_bundle": {
				"origin": "`+server.URL+`",
				"tokens": {"access_token": "real-provider-token"},
				"required_headers": {
					"origin": "`+server.URL+`",
					"referer": "`+server.URL+`/dashboard"
				},
				"context": {"api_base_url": "`+server.URL+`"}
			}
		}
	}`)
	require.Equal(t, http.StatusOK, completed.Code, completed.Body.String())
	require.Equal(t, 0, profileCalls)
	require.Contains(t, completed.Body.String(), `"session_captured":true`)
	require.NotContains(t, completed.Body.String(), `"balance_cents"`)
	require.NotContains(t, completed.Body.String(), `"balance_snapshot_id"`)
	require.NotContains(t, completed.Body.String(), "real-provider-token")
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

func TestSupplierSessionDirectLoginSyncsBalance(t *testing.T) {
	var seenLogin bool
	var seenProfileAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch req.URL.Path {
		case "/api/v1/settings/public":
			_, _ = w.Write([]byte(`{"data":{"login_agreement_revision":"rev-test"}}`))
		case "/api/v1/auth/login":
			seenLogin = true
			var payload map[string]any
			require.NoError(t, json.NewDecoder(req.Body).Decode(&payload))
			require.Equal(t, "ops@example.com", payload["email"])
			require.Equal(t, "secret", payload["password"])
			require.Equal(t, "rev-test", payload["login_agreement_revision"])
			require.Equal(t, "rev-test", payload["loginAgreementRevision"])
			require.Equal(t, true, payload["agreement_accepted"])
			consent, ok := payload["login_agreement_consent"].(map[string]any)
			require.True(t, ok)
			require.Equal(t, "rev-test", consent["revision"])
			_, _ = w.Write([]byte(`{"data":{"access_token":"direct-provider-token","expires_in":3600}}`))
		case "/api/v1/user/profile":
			seenProfileAuth = req.Header.Get("Authorization")
			_, _ = w.Write([]byte(`{
				"data": {
					"id": 42,
					"email": "ops@example.com",
					"status": "active",
					"balance": 8.88,
					"concurrency": 16,
					"allowed_groups": [10]
				}
			}`))
		default:
			writeOperationsSub2APICapabilityFixture(t, w, req)
		}
	}))
	defer server.Close()

	router := newOperationsHandlerTestRouterWithSupplierURLs(server.URL, server.URL)
	loggedIn := performJSON(t, router, http.MethodPost, "/suppliers/1/session/login", `{}`)
	require.Equal(t, http.StatusOK, loggedIn.Code, loggedIn.Body.String())
	require.True(t, seenLogin)
	require.Equal(t, "Bearer direct-provider-token", seenProfileAuth)
	require.Contains(t, loggedIn.Body.String(), `"session_source":"direct_login"`)
	require.Contains(t, loggedIn.Body.String(), `"balance_cents":888`)
	require.Contains(t, loggedIn.Body.String(), `"balance_snapshot"`)
	require.NotContains(t, loggedIn.Body.String(), "direct-provider-token")

	session := performJSON(t, router, http.MethodGet, "/suppliers/1/session", ``)
	require.Equal(t, http.StatusOK, session.Code, session.Body.String())
	require.Contains(t, session.Body.String(), `"session_source":"direct_login"`)
	require.Contains(t, session.Body.String(), `"has_access_token":true`)
	require.NotContains(t, session.Body.String(), "direct-provider-token")
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

func TestSupplierUsageCostSyncReadsProviderSession(t *testing.T) {
	var seenAuth string
	var seenCookie string
	var seenStartedAt string
	var seenEndedAt string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		require.Equal(t, http.MethodGet, req.Method)
		seenAuth = req.Header.Get("Authorization")
		seenCookie = req.Header.Get("Cookie")
		w.Header().Set("Content-Type", "application/json")
		switch req.URL.Path {
		case "/api/v1/billing/lines":
			seenStartedAt = req.URL.Query().Get("started_at")
			seenEndedAt = req.URL.Query().Get("ended_at")
			_, _ = w.Write([]byte(`{
				"data": [
					{
						"id": 91,
						"request_id": "req-billing-1",
						"api_key_name": "ops-key",
						"model": "gpt-5-mini",
						"endpoint": "/v1/responses",
						"request_type": "responses",
						"billing_mode": "token",
						"currency": "usd",
						"cost": 1.23,
						"input_tokens": 1000,
						"output_tokens": 500,
						"started_at": "2026-06-20T10:00:00Z",
						"access_token": "must-not-return",
						"headers": {"cookie": "must-not-return", "x-safe": "kept"}
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

	synced := performJSON(t, router, http.MethodPost, "/suppliers/1/usage-costs/sync", `{
		"started_at": "2026-06-20T00:00:00Z",
		"ended_at": "2026-06-21T00:00:00Z"
	}`)
	require.Equal(t, http.StatusCreated, synced.Code, synced.Body.String())
	require.Equal(t, "Bearer secret-token", seenAuth)
	require.Equal(t, "sid=secret-cookie", seenCookie)
	require.Equal(t, "2026-06-20T00:00:00Z", seenStartedAt)
	require.Equal(t, "2026-06-21T00:00:00Z", seenEndedAt)
	require.Contains(t, synced.Body.String(), `"source":"provider_session"`)
	require.Contains(t, synced.Body.String(), `"external_request_id":"req-billing-1"`)
	require.Contains(t, synced.Body.String(), `"model":"gpt-5-mini"`)
	require.Contains(t, synced.Body.String(), `"cost_cents":123`)
	require.NotContains(t, synced.Body.String(), "secret-token")
	require.NotContains(t, synced.Body.String(), "secret-cookie")
	require.NotContains(t, synced.Body.String(), "must-not-return")
	require.Contains(t, synced.Body.String(), `"x-safe":"kept"`)
}

func TestSupplierCostSyncSubmitsAsyncJob(t *testing.T) {
	router := newOperationsHandlerTestRouter()

	rec := performJSON(t, router, http.MethodPost, "/suppliers/1/costs/sync", `{
		"started_at": "2026-06-14T00:00:00Z",
		"ended_at": "2026-06-21T00:00:00Z",
		"include_funding_transactions": true,
		"include_entitlement_transactions": true,
		"include_usage_cost_lines": true,
		"include_balance_snapshot": true
	}`)

	require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"job_type":"sync_supplier_costs"`)
	require.Contains(t, rec.Body.String(), `"mode":"async_job"`)
	require.Contains(t, rec.Body.String(), `"poll_url":"/api/v1/admin-plus/supplier-provision-jobs/1"`)
}

func TestSupplierKeyProvisionCreatesProviderKeyLocalAccountAndBinding(t *testing.T) {
	var seenAuth string
	var seenCookie string
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if req.Method == http.MethodGet {
			writeOperationsSub2APICapabilityFixture(t, w, req)
			return
		}
		require.Equal(t, http.MethodPost, req.Method)
		require.Equal(t, "/api/v1/keys", req.URL.Path)
		seenAuth = req.Header.Get("Authorization")
		seenCookie = req.Header.Get("Cookie")
		require.NoError(t, json.NewDecoder(req.Body).Decode(&payload))
		_, _ = w.Write([]byte(`{"data":{"id":99,"name":"ops-key","key":"sk-provider-secret","group_id":10,"status":"active"}}`))
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

func TestSupplierKeyProvisionReplaysWithIdempotencyKey(t *testing.T) {
	repo := newOperationsIdempotencyRepository()
	cfg := service.DefaultIdempotencyConfig()
	service.SetDefaultIdempotencyCoordinator(service.NewIdempotencyCoordinator(repo, cfg))
	t.Cleanup(func() {
		service.SetDefaultIdempotencyCoordinator(nil)
	})

	var providerCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if req.Method == http.MethodGet {
			writeOperationsSub2APICapabilityFixture(t, w, req)
			return
		}
		require.Equal(t, http.MethodPost, req.Method)
		require.Equal(t, "/api/v1/keys", req.URL.Path)
		providerCalls++
		_, _ = w.Write([]byte(`{"data":{"id":99,"name":"ops-key","key":"sk-provider-secret","group_id":10,"status":"active"}}`))
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

	payload := `{
		"supplier_group_id": 10,
		"name": "ops-key",
		"quota_usd": 25,
		"local_account_platform": "openai",
		"local_account_name": "local-upstream",
		"local_account_base_url": "` + server.URL + `/v1",
		"local_account_concurrency": 3,
		"local_account_priority": 40,
		"balance_currency": "USD"
	}`
	first := performJSONWithHeaders(t, router, http.MethodPost, "/suppliers/1/keys/provision", payload, map[string]string{
		"Idempotency-Key": "supplier-1-group-10-ops-key",
	})
	require.Equal(t, http.StatusCreated, first.Code, first.Body.String())
	require.Empty(t, first.Header().Get("X-Idempotency-Replayed"))

	second := performJSONWithHeaders(t, router, http.MethodPost, "/suppliers/1/keys/provision", payload, map[string]string{
		"Idempotency-Key": "supplier-1-group-10-ops-key",
	})
	require.Equal(t, http.StatusCreated, second.Code, second.Body.String())
	require.Equal(t, "true", second.Header().Get("X-Idempotency-Replayed"))
	require.Equal(t, 1, providerCalls)
	require.Contains(t, second.Body.String(), `"external_key_id":"99"`)
	require.NotContains(t, second.Body.String(), "sk-provider-secret")
}

func TestSupplierKeyRepairBindingBindsFailedKeyWithoutProviderCall(t *testing.T) {
	var providerCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if req.Method == http.MethodGet {
			writeOperationsSub2APICapabilityFixture(t, w, req)
			return
		}
		require.Equal(t, http.MethodPost, req.Method)
		require.Equal(t, "/api/v1/keys", req.URL.Path)
		providerCalls++
		_, _ = w.Write([]byte(`{"data":{"id":99,"name":"ops-key","key":"sk-provider-secret","group_id":10,"status":"active"}}`))
	}))
	defer server.Close()

	localAccounts := &operationsLocalAccountCreator{createErr: errors.New("local account store unavailable")}
	router := newOperationsHandlerTestRouterWithDependencies(server.URL, server.URL, localAccounts, false)
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
		"balance_currency": "USD"
	}`)
	require.Equal(t, http.StatusBadGateway, provisioned.Code, provisioned.Body.String())
	require.Equal(t, 1, providerCalls)
	require.NotContains(t, provisioned.Body.String(), "sk-provider-secret")

	localAccounts.createErr = nil
	localAccounts.accounts = make(map[int64]*service.Account)
	localAccounts.accounts[2002] = &service.Account{
		ID:       2002,
		Name:     "manual-local",
		Platform: service.PlatformOpenAI,
		Type:     service.AccountTypeAPIKey,
	}
	repaired := performJSON(t, router, http.MethodPost, "/suppliers/1/keys/1/repair-binding", `{
		"local_sub2api_account_id": 2002,
		"runtime_status": "monitor_only",
		"health_status": "normal",
		"balance_currency": "USD"
	}`)
	require.Equal(t, http.StatusOK, repaired.Code, repaired.Body.String())
	require.Equal(t, 1, providerCalls)
	require.Contains(t, repaired.Body.String(), `"status":"bound"`)
	require.Contains(t, repaired.Body.String(), `"local_sub2api_account_id":2002`)
	require.Contains(t, repaired.Body.String(), `"supplier_key_id":1`)
	require.NotContains(t, repaired.Body.String(), "sk-provider-secret")
	require.NotContains(t, repaired.Body.String(), "secret-token")
	require.NotContains(t, repaired.Body.String(), "secret-cookie")
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

func TestExtensionRootFindsRuntimeExtensionDirectory(t *testing.T) {
	t.Setenv("ADMIN_PLUS_EXTENSION_DIR", "")
	root := t.TempDir()
	extensionDir := filepath.Join(root, "extension")
	require.NoError(t, os.MkdirAll(extensionDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(extensionDir, "manifest.json"), []byte(`{"name":"Runtime Extension"}`), 0o644))

	previousWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(root))
	t.Cleanup(func() {
		if err := os.Chdir(previousWD); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})

	resolved, err := extensionRoot()
	require.NoError(t, err)
	expected, err := filepath.Abs(extensionDir)
	require.NoError(t, err)
	expected, err = filepath.EvalSymlinks(expected)
	require.NoError(t, err)
	actual, err := filepath.EvalSymlinks(resolved)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
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

func writeOperationsSub2APICapabilityFixture(t *testing.T, w http.ResponseWriter, req *http.Request) {
	t.Helper()
	require.Equal(t, http.MethodGet, req.Method)
	switch req.URL.Path {
	case "/api/v1/groups/available":
		_, _ = w.Write([]byte(`{"data":[{"id":10,"name":"GPT-5.5 Low Cost"}]}`))
	case "/api/v1/channels/available":
		_, _ = w.Write([]byte(`{"data":[{"name":"OpenAI","supported_models":[]}]}`))
	case "/api/v1/announcements":
		_, _ = w.Write([]byte(`{"data":[{"id":1,"title":"公告","content":"通知"}]}`))
	case "/api/v1/payment/checkout-info":
		_, _ = w.Write([]byte(`{"data":{"currency":"usd"}}`))
	case "/api/v1/usage":
		require.Equal(t, "1", req.URL.Query().Get("page"))
		require.Equal(t, "1", req.URL.Query().Get("page_size"))
		_, _ = w.Write([]byte(`{"data":{"items":[]}}`))
	case "/api/v1/keys":
		require.Equal(t, "1", req.URL.Query().Get("page"))
		require.Contains(t, []string{"1", "100"}, req.URL.Query().Get("page_size"))
		_, _ = w.Write([]byte(`{"data":[]}`))
	default:
		http.NotFound(w, req)
	}
}

func newOperationsHandlerTestRouter() *gin.Engine {
	return newOperationsHandlerTestRouterWithSupplierURLs("https://relay.example.com", "https://relay.example.com")
}

func newOperationsHandlerTestRouterWithSupplierURLs(dashboardURL string, apiBaseURL string) *gin.Engine {
	return newOperationsHandlerTestRouterWithDependencies(dashboardURL, apiBaseURL, &operationsLocalAccountCreator{}, false)
}

func newOperationsHandlerTestRouterWithAutoSessionBalanceSync(dashboardURL string, apiBaseURL string) *gin.Engine {
	return newOperationsHandlerTestRouterWithDependencies(dashboardURL, apiBaseURL, &operationsLocalAccountCreator{}, true)
}

func newOperationsHandlerTestRouterWithDependencies(dashboardURL string, apiBaseURL string, localAccounts *operationsLocalAccountCreator, autoSessionBalanceSync bool) *gin.Engine {
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
	sessionSvc := sessionsapp.NewServiceWithDependencies(
		sessionsapp.NewMemoryRepository(),
		plainSessionCipher{},
		supplierSvc,
		sub2apiprovider.NewSessionProfileClient(nil),
		sub2apiprovider.NewSessionProfileClient(http.DefaultClient),
	)
	sessionGroupClient := sub2apiprovider.NewSessionProfileClient(http.DefaultClient)
	usageCostService := usagecostsapp.NewServiceWithDependencies(
		usagecostsapp.NewMemoryRepository(),
		sessionSvc,
		sessionGroupClient,
	)
	usageCostHandler := NewUsageCostHandler(usageCostService)
	costService := costsapp.NewService(costsapp.NewMemoryRepository())
	provisionJobService := provisionjobsapp.NewServiceWithCostSyncer(
		newOperationsProvisionRepository(),
		nil,
		nil,
		nil,
		costService,
	)
	costHandler := NewCostHandlerWithProvisionJobs(costService, provisionJobService)
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
		localAccounts,
	))
	balanceService := balancesapp.NewServiceWithDependencies(
		balancesapp.NewMemoryRepository(),
		nil,
		sessionSvc,
		sub2apiprovider.NewSessionProfileClient(http.DefaultClient),
	)
	var ingestBalanceService *balancesapp.Service
	if autoSessionBalanceSync {
		ingestBalanceService = balanceService
	}
	processor := extensionapp.NewIngestProcessorWithCipher(nil, ingestBalanceService, nil, nil, nil, sessionSvc, plainSessionCipher{}, nil)
	extensionHandler := NewExtensionHandler(extensionapp.NewServiceWithDependencies(extensionapp.NewMemoryRepository(), processor, supplierSvc), supplierSvc)
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
	notificationHandler := NewNotificationHandler(notificationsapp.NewService(notificationRepo))
	supplierHandler := NewSupplierHandler(supplierSvc)
	balanceHandler := NewBalanceHandler(balanceService)
	rateHandler := NewRateHandler(rateService)
	sessionHandler := NewSessionHandler(sessionSvc, balanceHandler.service)

	router := gin.New()
	router.POST("/suppliers/site-match", supplierHandler.MatchSite)
	router.GET("/suppliers/:id/session", sessionHandler.Get)
	router.POST("/suppliers/:id/session/login", sessionHandler.Login)
	router.POST("/suppliers/:id/session/probe", sessionHandler.Probe)
	router.POST("/suppliers/:id/browser-sessions", sessionHandler.Upsert)
	router.GET("/suppliers/:id/groups", supplierGroupHandler.List)
	router.POST("/suppliers/:id/groups/sync", supplierGroupHandler.Sync)
	router.GET("/suppliers/:id/keys", supplierKeyHandler.List)
	router.POST("/suppliers/:id/keys/ensure-all", supplierKeyHandler.EnsureAll)
	router.POST("/suppliers/:id/keys/provision", supplierKeyHandler.Provision)
	router.POST("/suppliers/:id/keys/:keyID/repair-binding", supplierKeyHandler.RepairBinding)
	router.POST("/suppliers/:id/rates/sync", rateHandler.SyncSupplierRates)
	router.POST("/suppliers/:id/usage-costs/sync", usageCostHandler.SyncSupplierUsageCosts)
	router.POST("/suppliers/:id/costs/sync", costHandler.SyncSupplierCosts)
	router.GET("/suppliers/:id/costs/summary", costHandler.GetSupplierSummary)
	router.GET("/suppliers/:id/funding-transactions", costHandler.ListFundingTransactions)
	router.GET("/suppliers/:id/entitlement-transactions", costHandler.ListEntitlementTransactions)
	router.GET("/suppliers/:id/cost-ledger", costHandler.ListLedgerEntries)
	router.POST("/usage-costs/lines/import", usageCostHandler.ImportUsageCostLines)
	router.GET("/usage-costs/lines", usageCostHandler.ListUsageCostLines)
	router.GET("/costs/suppliers", costHandler.ListSupplierSummaries)
	router.POST("/extension/tasks", extensionHandler.CreateTask)
	router.POST("/extension/tasks/claim", extensionHandler.ClaimTask)
	router.POST("/extension/suppliers/report-candidate", extensionHandler.ReportSupplierCandidate)
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
	return performJSONWithHeaders(t, router, method, path, payload, nil)
}

func performJSONWithHeaders(t *testing.T, router *gin.Engine, method string, path string, payload string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
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

type operationsIdempotencyRepository struct {
	mu     sync.Mutex
	nextID int64
	data   map[string]*service.IdempotencyRecord
}

type operationsProvisionRepository struct {
	nextID int64
}

func newOperationsProvisionRepository() *operationsProvisionRepository {
	return &operationsProvisionRepository{nextID: 1}
}

func (r *operationsProvisionRepository) CreateJobWithSteps(_ context.Context, job *adminplusdomain.SupplierProvisionJob, steps []*adminplusdomain.SupplierProvisionStep, _ string) (*adminplusdomain.SupplierProvisionJob, bool, error) {
	job.ID = r.nextID
	r.nextID++
	for index, step := range steps {
		if step == nil {
			continue
		}
		step.ID = job.ID + int64(index)
		step.JobID = job.ID
	}
	stored := *job
	stored.Steps = steps
	return &stored, false, nil
}

func (r *operationsProvisionRepository) GetJob(context.Context, int64) (*adminplusdomain.SupplierProvisionJob, error) {
	return nil, nil
}

func (r *operationsProvisionRepository) ListJobs(context.Context, provisionjobsapp.ListFilter) ([]*adminplusdomain.SupplierProvisionJob, error) {
	return nil, nil
}

func (r *operationsProvisionRepository) ClaimNextJob(context.Context, string, time.Time, time.Duration) (*adminplusdomain.SupplierProvisionJob, error) {
	return nil, nil
}

func (r *operationsProvisionRepository) MarkJobRunning(context.Context, int64, string, time.Time) error {
	return nil
}

func (r *operationsProvisionRepository) MarkJobSucceeded(context.Context, int64, map[string]any, time.Time) error {
	return nil
}

func (r *operationsProvisionRepository) MarkJobFailed(context.Context, int64, adminplusdomain.SupplierProvisionStatus, string, string, time.Time, time.Time) error {
	return nil
}

func (r *operationsProvisionRepository) ListRunnableSteps(context.Context, int64, time.Time) ([]*adminplusdomain.SupplierProvisionStep, error) {
	return nil, nil
}

func (r *operationsProvisionRepository) ListJobSteps(context.Context, int64) ([]*adminplusdomain.SupplierProvisionStep, error) {
	return nil, nil
}

func (r *operationsProvisionRepository) MarkStepRunning(context.Context, int64, string, time.Time) error {
	return nil
}

func (r *operationsProvisionRepository) MarkStepSucceeded(context.Context, int64, map[string]any, time.Time) error {
	return nil
}

func (r *operationsProvisionRepository) MarkStepFailed(context.Context, int64, adminplusdomain.SupplierProvisionStatus, string, string, time.Time, time.Time) error {
	return nil
}

func (r *operationsProvisionRepository) RecordAttempt(context.Context, provisionjobsapp.Attempt) error {
	return nil
}

func (r *operationsProvisionRepository) ListPendingOutboxEvents(context.Context, int, time.Time) ([]provisionjobsapp.OutboxEvent, error) {
	return nil, nil
}

func (r *operationsProvisionRepository) MarkOutboxPublished(context.Context, string, time.Time) error {
	return nil
}

func (r *operationsProvisionRepository) MarkOutboxFailed(context.Context, string, time.Time, error) error {
	return nil
}

func newOperationsIdempotencyRepository() *operationsIdempotencyRepository {
	return &operationsIdempotencyRepository{
		nextID: 1,
		data:   make(map[string]*service.IdempotencyRecord),
	}
}

func (r *operationsIdempotencyRepository) CreateProcessing(_ context.Context, record *service.IdempotencyRecord) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := r.key(record.Scope, record.IdempotencyKeyHash)
	if _, ok := r.data[key]; ok {
		return false, nil
	}
	cp := cloneOperationsIdempotencyRecord(record)
	cp.ID = r.nextID
	r.nextID++
	r.data[key] = cp
	record.ID = cp.ID
	return true, nil
}

func (r *operationsIdempotencyRepository) GetByScopeAndKeyHash(_ context.Context, scope string, keyHash string) (*service.IdempotencyRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return cloneOperationsIdempotencyRecord(r.data[r.key(scope, keyHash)]), nil
}

func (r *operationsIdempotencyRepository) TryReclaim(_ context.Context, id int64, fromStatus string, now time.Time, newLockedUntil time.Time, newExpiresAt time.Time) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, record := range r.data {
		if record.ID != id {
			continue
		}
		if record.Status != fromStatus {
			return false, nil
		}
		if record.LockedUntil != nil && record.LockedUntil.After(now) {
			return false, nil
		}
		record.Status = service.IdempotencyStatusProcessing
		record.LockedUntil = &newLockedUntil
		record.ExpiresAt = newExpiresAt
		record.ErrorReason = nil
		return true, nil
	}
	return false, nil
}

func (r *operationsIdempotencyRepository) ExtendProcessingLock(_ context.Context, id int64, requestFingerprint string, newLockedUntil time.Time, newExpiresAt time.Time) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, record := range r.data {
		if record.ID != id {
			continue
		}
		if record.Status != service.IdempotencyStatusProcessing || record.RequestFingerprint != requestFingerprint {
			return false, nil
		}
		record.LockedUntil = &newLockedUntil
		record.ExpiresAt = newExpiresAt
		return true, nil
	}
	return false, nil
}

func (r *operationsIdempotencyRepository) MarkSucceeded(_ context.Context, id int64, responseStatus int, responseBody string, expiresAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, record := range r.data {
		if record.ID != id {
			continue
		}
		record.Status = service.IdempotencyStatusSucceeded
		record.LockedUntil = nil
		record.ResponseStatus = &responseStatus
		record.ResponseBody = &responseBody
		record.ExpiresAt = expiresAt
		return nil
	}
	return errors.New("idempotency record not found")
}

func (r *operationsIdempotencyRepository) MarkFailedRetryable(_ context.Context, id int64, errorReason string, lockedUntil time.Time, expiresAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, record := range r.data {
		if record.ID != id {
			continue
		}
		record.Status = service.IdempotencyStatusFailedRetryable
		record.LockedUntil = &lockedUntil
		record.ExpiresAt = expiresAt
		record.ErrorReason = &errorReason
		return nil
	}
	return errors.New("idempotency record not found")
}

func (r *operationsIdempotencyRepository) DeleteExpired(_ context.Context, now time.Time, _ int) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var deleted int64
	for key, record := range r.data {
		if !record.ExpiresAt.After(now) {
			delete(r.data, key)
			deleted++
		}
	}
	return deleted, nil
}

func (r *operationsIdempotencyRepository) key(scope string, keyHash string) string {
	return scope + "|" + keyHash
}

func cloneOperationsIdempotencyRecord(in *service.IdempotencyRecord) *service.IdempotencyRecord {
	if in == nil {
		return nil
	}
	out := *in
	if in.ResponseStatus != nil {
		value := *in.ResponseStatus
		out.ResponseStatus = &value
	}
	if in.ResponseBody != nil {
		value := *in.ResponseBody
		out.ResponseBody = &value
	}
	if in.ErrorReason != nil {
		value := *in.ErrorReason
		out.ErrorReason = &value
	}
	if in.LockedUntil != nil {
		value := *in.LockedUntil
		out.LockedUntil = &value
	}
	return &out
}

type operationsLocalAccountCreator struct {
	createErr error
	accounts  map[int64]*service.Account
	groups    []service.Group
}

func (c *operationsLocalAccountCreator) CreateAccount(_ context.Context, input *service.CreateAccountInput) (*service.Account, error) {
	if c.createErr != nil {
		return nil, c.createErr
	}
	account := &service.Account{
		ID:          1001,
		Name:        input.Name,
		Platform:    input.Platform,
		Type:        input.Type,
		Credentials: input.Credentials,
		GroupIDs:    append([]int64(nil), input.GroupIDs...),
	}
	if c.accounts == nil {
		c.accounts = make(map[int64]*service.Account)
	}
	c.accounts[account.ID] = account
	return account, nil
}

func (c *operationsLocalAccountCreator) GetAccount(_ context.Context, id int64) (*service.Account, error) {
	if c.accounts != nil {
		if account, ok := c.accounts[id]; ok {
			cp := *account
			return &cp, nil
		}
	}
	return nil, errors.New("account not found")
}

func (c *operationsLocalAccountCreator) UpdateAccount(_ context.Context, id int64, input *service.UpdateAccountInput) (*service.Account, error) {
	if c.accounts == nil {
		return nil, errors.New("account not found")
	}
	account, ok := c.accounts[id]
	if !ok {
		return nil, errors.New("account not found")
	}
	if input.GroupIDs != nil {
		account.GroupIDs = append([]int64(nil), (*input.GroupIDs)...)
	}
	cp := *account
	return &cp, nil
}

func (c *operationsLocalAccountCreator) CreateGroup(_ context.Context, input *service.CreateGroupInput) (*service.Group, error) {
	group := service.Group{
		ID:             int64(2001 + len(c.groups)),
		Name:           input.Name,
		Platform:       input.Platform,
		RateMultiplier: input.RateMultiplier,
		Status:         "active",
	}
	c.groups = append(c.groups, group)
	return &group, nil
}

func (c *operationsLocalAccountCreator) GetAllGroupsIncludingInactive(_ context.Context) ([]service.Group, error) {
	out := make([]service.Group, len(c.groups))
	copy(out, c.groups)
	return out, nil
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
