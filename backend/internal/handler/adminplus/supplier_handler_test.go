package adminplus

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	suppliersapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/suppliers"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type supplierResponseEnvelope struct {
	Code int `json:"code"`
	Data struct {
		ID            int64  `json:"id"`
		Name          string `json:"name"`
		RuntimeStatus string `json:"runtime_status"`
		HealthStatus  string `json:"health_status"`
		Credential    struct {
			BrowserLoginEnabled            bool   `json:"browser_login_enabled"`
			BrowserLoginUsernameConfigured bool   `json:"browser_login_username_configured"`
			MaskedBrowserLoginUsername     string `json:"masked_browser_login_username"`
		} `json:"credential"`
	} `json:"data"`
}

type supplierListResponseEnvelope struct {
	Code int `json:"code"`
	Data struct {
		Items []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"items"`
		Total int `json:"total"`
	} `json:"data"`
}

func newSupplierHandlerTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewSupplierHandler(suppliersapp.NewService(suppliersapp.NewMemoryRepository()))
	router := gin.New()
	router.GET("/suppliers", h.List)
	router.POST("/suppliers", h.Create)
	router.GET("/suppliers/:id", h.Get)
	router.PUT("/suppliers/:id", h.Update)
	router.DELETE("/suppliers/:id", h.Delete)
	router.PATCH("/suppliers/:id/status", h.UpdateStatus)
	router.GET("/suppliers/:id/accounts", h.ListAccounts)
	router.POST("/suppliers/:id/accounts", h.CreateAccount)
	router.PUT("/suppliers/:id/accounts/:accountID", h.UpdateAccount)
	router.DELETE("/suppliers/:id/accounts/:accountID", h.DeleteAccount)
	return router
}

func TestSupplierHandlerCreateAndGet(t *testing.T) {
	router := newSupplierHandlerTestRouter()

	body := bytes.NewBufferString(`{
		"name": "Primary Sub2API Relay",
		"kind": "relay",
		"type": "sub2api",
		"runtime_status": "candidate",
		"balance_cents": 5000,
		"browser_login_enabled": true,
		"browser_login_username": "ops@example.com",
		"browser_login_password": "secret"
	}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/suppliers", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	var created supplierResponseEnvelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	require.Equal(t, int64(1), created.Data.ID)
	require.Equal(t, "Primary Sub2API Relay", created.Data.Name)
	require.Equal(t, "candidate", created.Data.RuntimeStatus)
	require.True(t, created.Data.Credential.BrowserLoginEnabled)
	require.True(t, created.Data.Credential.BrowserLoginUsernameConfigured)
	require.Equal(t, "op***@example.com", created.Data.Credential.MaskedBrowserLoginUsername)

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/suppliers/1", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var got supplierResponseEnvelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, created.Data.ID, got.Data.ID)
	require.Equal(t, created.Data.Name, got.Data.Name)
}

func TestSupplierHandlerList(t *testing.T) {
	router := newSupplierHandlerTestRouter()
	createSupplier(t, router, `{"name":"Relay A","kind":"relay","type":"sub2api"}`)
	createSupplier(t, router, `{"name":"OpenAI Source","kind":"source_account","type":"openai"}`)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/suppliers?kind=source_account", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body supplierListResponseEnvelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, 1, body.Data.Total)
	require.Equal(t, "OpenAI Source", body.Data.Items[0].Name)
}

func TestSupplierHandlerRejectsCandidateWithoutBalance(t *testing.T) {
	router := newSupplierHandlerTestRouter()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/suppliers", bytes.NewBufferString(`{
		"name": "No Balance Relay",
		"kind": "relay",
		"type": "sub2api",
		"runtime_status": "candidate"
	}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "candidate supplier must have positive balance")
}

func TestSupplierHandlerUpdateAndDelete(t *testing.T) {
	router := newSupplierHandlerTestRouter()
	createSupplier(t, router, `{"name":"Relay A","kind":"relay","type":"sub2api"}`)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/suppliers/1", bytes.NewBufferString(`{
		"name": "Relay A Updated",
		"kind": "relay",
		"type": "sub2api",
		"runtime_status": "monitor_only",
		"health_status": "normal",
		"browser_login_enabled": true
	}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var updated supplierResponseEnvelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &updated))
	require.Equal(t, "Relay A Updated", updated.Data.Name)

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/suppliers/1", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func TestSupplierHandlerAccountCRUD(t *testing.T) {
	router := newSupplierHandlerTestRouter()
	createSupplier(t, router, `{"name":"Relay A","kind":"relay","type":"sub2api","runtime_status":"candidate","balance_cents":5000}`)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/suppliers/1/accounts", bytes.NewBufferString(`{
		"local_sub2api_account_id": 1,
		"supplier_account_identifier": "supplier-user",
		"supplier_account_label": "primary",
		"runtime_status": "monitor_only"
	}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/suppliers/1/accounts/1", bytes.NewBufferString(`{
		"supplier_account_identifier": "supplier-key-1",
		"supplier_account_label": "primary",
		"rate_profile": "discount-a",
		"configured_concurrency": 8,
		"observed_max_concurrency": 6,
		"balance_cents": 3000,
		"balance_threshold_cents": 500,
		"balance_currency": "CNY",
		"runtime_status": "candidate",
		"health_status": "normal"
	}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "supplier-key-1")

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/suppliers/1/accounts/1", nil)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func createSupplier(t *testing.T, router *gin.Engine, payload string) {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/suppliers", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
}
