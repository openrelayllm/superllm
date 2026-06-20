package adminplus

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	actionsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/actions"
	billingapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/billing"
	extensionapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/extension"
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

func newOperationsHandlerTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)

	billingHandler := NewBillingHandler(billingapp.NewService(billingapp.NewMemoryRepository()))
	extensionHandler := NewExtensionHandler(extensionapp.NewService(extensionapp.NewMemoryRepository()))
	actionHandler := NewActionHandler(actionsapp.NewRuleService())

	router := gin.New()
	router.POST("/billing/lines/import", billingHandler.ImportBillLines)
	router.GET("/billing/lines", billingHandler.ListBillLines)
	router.POST("/extension/tasks", extensionHandler.CreateTask)
	router.POST("/extension/tasks/claim", extensionHandler.ClaimTask)
	router.POST("/extension/tasks/:id/heartbeat", extensionHandler.Heartbeat)
	router.POST("/extension/tasks/:id/complete", extensionHandler.CompleteTask)
	router.POST("/extension/tasks/:id/fail", extensionHandler.FailTask)
	router.GET("/extension/tasks", extensionHandler.ListTasks)
	router.POST("/actions/generate", actionHandler.Generate)
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
