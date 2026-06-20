package adminplus

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	balancesapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/balances"
	healthapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/health"
	promotionsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/promotions"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestBalanceHandlerRecordSnapshotCreatesLowBalanceEvent(t *testing.T) {
	router := newMonitoringHandlerTestRouter()

	createBalanceSnapshot(t, router, `{
		"supplier_id": 7,
		"runtime_status": "candidate",
		"balance_cents": 3000,
		"currency": "USD",
		"captured_at": "2026-06-20T10:00:00Z"
	}`)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/balances/snapshots", bytes.NewBufferString(`{
		"supplier_id": 7,
		"runtime_status": "candidate",
		"balance_cents": 500,
		"currency": "USD",
		"low_balance_threshold_cents": 1000,
		"captured_at": "2026-06-20T10:01:00Z"
	}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var body struct {
		Code int `json:"code"`
		Data struct {
			Snapshot struct {
				SwitchEligible bool `json:"switch_eligible"`
			} `json:"snapshot"`
			Event struct {
				Type string `json:"type"`
			} `json:"event"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.True(t, body.Data.Snapshot.SwitchEligible)
	require.Equal(t, "low_balance", body.Data.Event.Type)
}

func TestPromotionHandlerRecordPromotionRecommendsRecharge(t *testing.T) {
	router := newMonitoringHandlerTestRouter()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/promotions", bytes.NewBufferString(`{
		"supplier_id": 7,
		"type": "recharge_bonus",
		"title": "Recharge bonus",
		"runtime_status": "monitor_only",
		"balance_cents": 0,
		"bonus_percent": 20
	}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var body struct {
		Code int `json:"code"`
		Data struct {
			Recommendation string `json:"recommendation"`
			SwitchEligible bool   `json:"switch_eligible"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "recharge_to_unlock", body.Data.Recommendation)
	require.False(t, body.Data.SwitchEligible)
}

func TestHealthHandlerRecordSampleCreatesHealthEvents(t *testing.T) {
	router := newMonitoringHandlerTestRouter()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/health/samples", bytes.NewBufferString(`{
		"supplier_id": 7,
		"model": "gpt-4o-mini",
		"first_token_latency_ms": 5000,
		"total_latency_ms": 35000,
		"status_code": 502,
		"first_token_threshold_ms": 3000,
		"total_latency_threshold_ms": 30000
	}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var body struct {
		Code int `json:"code"`
		Data struct {
			Events []struct {
				Type string `json:"type"`
			} `json:"events"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Data.Events, 3)
	require.Equal(t, "slow_first_token", body.Data.Events[0].Type)
	require.Equal(t, "slow_total", body.Data.Events[1].Type)
	require.Equal(t, "request_error", body.Data.Events[2].Type)
}

func TestHealthHandlerProbeOpenAIResponses(t *testing.T) {
	router := newMonitoringHandlerTestRouter()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/health/probe", bytes.NewBufferString(`{
		"supplier_id": 7,
		"model": "gpt-5.5"
	}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var body struct {
		Code int `json:"code"`
		Data struct {
			Sample struct {
				Source string `json:"source"`
				Model  string `json:"model"`
			} `json:"sample"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "responses_probe", body.Data.Sample.Source)
	require.Equal(t, "gpt-5.5", body.Data.Sample.Model)
}

func newMonitoringHandlerTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)

	balanceHandler := NewBalanceHandler(balancesapp.NewService(balancesapp.NewMemoryRepository()))
	promotionHandler := NewPromotionHandler(promotionsapp.NewService(promotionsapp.NewMemoryRepository()))
	healthHandler := NewHealthHandler(healthapp.NewService(healthapp.NewMemoryRepository()))

	router := gin.New()
	router.POST("/balances/snapshots", balanceHandler.RecordSnapshot)
	router.GET("/balances/snapshots", balanceHandler.ListSnapshots)
	router.GET("/balances/events", balanceHandler.ListEvents)
	router.PATCH("/balances/events/:id/ack", balanceHandler.AcknowledgeEvent)
	router.POST("/promotions", promotionHandler.RecordPromotion)
	router.GET("/promotions", promotionHandler.ListEvents)
	router.PATCH("/promotions/:id/ack", promotionHandler.AcknowledgeEvent)
	router.POST("/health/probe", healthHandler.ProbeOpenAIResponses)
	router.POST("/health/samples", healthHandler.RecordSample)
	router.GET("/health/samples", healthHandler.ListSamples)
	router.GET("/health/events", healthHandler.ListEvents)
	router.PATCH("/health/events/:id/ack", healthHandler.AcknowledgeEvent)
	return router
}

func createBalanceSnapshot(t *testing.T, router *gin.Engine, payload string) {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/balances/snapshots", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
}
