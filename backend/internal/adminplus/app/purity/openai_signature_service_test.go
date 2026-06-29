package purity

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestServiceRunPublicCheck_OpenAIResponsesStoreIncludeAccepted(t *testing.T) {
	server := newOpenAIStoreIncludeServer(t, http.StatusOK, "", nil)
	defer server.Close()

	service := NewService(nil)
	service.httpClient = server.Client()
	service.allowPrivateHosts = true
	service.limiter = nil

	report, err := service.RunDeveloperAPICheck(context.Background(), PublicCheckInput{
		Provider:       ProviderOpenAI,
		APIBaseURL:     server.URL,
		APIKey:         "sk-store-include",
		ModelID:        "gpt-5.4",
		ClientIP:       "203.0.113.20",
		SkipTokenAudit: true,
	})
	require.NoError(t, err)
	require.Equal(t, CheckStatusPass, findCheck(t, report, "responses_store_include").Status)
	require.Equal(t, CheckStatusPass, findValidation(t, report, "signature").Status)
	require.Equal(t, VerdictOfficialOpenAI, report.Verdict)
	require.Equal(t, 100, report.OfficialScore)
}

func TestServiceRunPublicCheck_OpenAIResponsesStoreIncludeRejected(t *testing.T) {
	server := newOpenAIStoreIncludeServer(t, http.StatusBadRequest, "unsupported include: reasoning.encrypted_content", nil)
	defer server.Close()

	service := NewService(nil)
	service.httpClient = server.Client()
	service.allowPrivateHosts = true
	service.limiter = nil

	report, err := service.RunDeveloperAPICheck(context.Background(), PublicCheckInput{
		Provider:       ProviderOpenAI,
		APIBaseURL:     server.URL,
		APIKey:         "sk-store-include",
		ModelID:        "gpt-5.4",
		ClientIP:       "203.0.113.20",
		SkipTokenAudit: true,
	})
	require.NoError(t, err)
	require.Equal(t, CheckStatusFail, findCheck(t, report, "responses_store_include").Status)
	require.Equal(t, CheckStatusFail, findValidation(t, report, "signature").Status)
	require.Equal(t, VerdictOpenAICompatible, report.Verdict)
	require.Equal(t, 70, report.OfficialScore)
	require.Equal(t, 87, report.CompatibilityScore)
}

func TestServiceRunPublicCheck_OpenAIResponsesStoreIncludeBalanceWarn(t *testing.T) {
	server := newOpenAIStoreIncludeServer(t, http.StatusForbidden, "Insufficient account balance", nil)
	defer server.Close()

	service := NewService(nil)
	service.httpClient = server.Client()
	service.allowPrivateHosts = true
	service.limiter = nil

	report, err := service.RunDeveloperAPICheck(context.Background(), PublicCheckInput{
		Provider:       ProviderOpenAI,
		APIBaseURL:     server.URL,
		APIKey:         "sk-store-include",
		ModelID:        "gpt-5.4",
		ClientIP:       "203.0.113.20",
		SkipTokenAudit: true,
	})
	require.NoError(t, err)
	require.Equal(t, CheckStatusWarn, findCheck(t, report, "responses_store_include").Status)
	require.Equal(t, CheckStatusWarn, findValidation(t, report, "signature").Status)
	require.Equal(t, VerdictOfficialOpenAI, report.Verdict)
	require.Contains(t, findCheck(t, report, "responses_store_include").Message, "不能据此判断非官方")
}
