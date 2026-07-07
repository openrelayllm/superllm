package adminplus

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	importexportapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/importexport"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestImportExportHandlerScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewImportExportHandler(importexportapp.NewService(nil))
	router.GET("/scope", handler.Scope)

	req := httptest.NewRequest(http.MethodGet, "/scope", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"product":"sub2api-admin-plus"`)
	require.Contains(t, rec.Body.String(), `"included_tables"`)
	require.Contains(t, rec.Body.String(), `"excluded_tables"`)
}

func TestImportExportHandlerPreviewRejectsWrongProduct(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewImportExportHandler(importexportapp.NewService(nil))
	router.POST("/preview", handler.Preview)

	req := httptest.NewRequest(http.MethodPost, "/preview", strings.NewReader(`{"version":1,"product":"metapi","tables":{}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "unsupported archive product")
}

func TestImportExportHandlerPreviewValidArchive(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewImportExportHandler(importexportapp.NewService(nil))
	router.POST("/preview", handler.Preview)

	body := `{
		"version":1,
		"product":"sub2api-admin-plus",
		"exported_at":"2026-07-07T00:00:00Z",
		"tables":{
			"settings":[{"key":"site_name","value":"Lime"}],
			"admin_plus_scheduler_runs":[{"id":"run-1"}]
		}
	}`
	req := httptest.NewRequest(http.MethodPost, "/preview", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"valid":true`)
	require.Contains(t, rec.Body.String(), `"settings"`)
	require.Contains(t, rec.Body.String(), `"admin_plus_scheduler_runs"`)
}
