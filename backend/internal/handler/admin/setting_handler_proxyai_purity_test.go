package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSettingHandler_UpdateSettings_ValidatesProxyAIPurityTurnstileWhenLoginTurnstileDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &settingHandlerRepoStub{values: map[string]string{}}
	settingService := service.NewSettingService(repo, &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}})
	handler := NewSettingHandler(settingService, nil, nil, nil, nil, nil, nil)

	body := map[string]any{
		"turnstile_enabled":                   false,
		"proxyai_purity_turnstile_enabled":    true,
		"proxyai_purity_turnstile_site_key":   "",
		"proxyai_purity_turnstile_secret_key": "",
	}
	rawBody, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(rawBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, "ProxyAI purity Turnstile Site Key is required when enabled", resp.Message)
	require.Nil(t, repo.lastUpdates)
}
