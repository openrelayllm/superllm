package sessions

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/adminplus/app/bizlogs"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/adminplus/ports"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestServiceLoginStoresDirectLoginSession(t *testing.T) {
	repo := NewMemoryRepository()
	supplier := &sessionTestSupplierLookup{supplier: &adminplusdomain.Supplier{
		ID:           7,
		Name:         "Relay",
		Type:         adminplusdomain.SupplierTypeSub2API,
		DashboardURL: "https://relay.example.com",
		APIBaseURL:   "https://relay.example.com/api/v1",
	}}
	capturedAt := time.Date(2026, 6, 21, 10, 0, 0, 0, time.UTC)
	login := &sessionTestLoginAdapter{result: &ports.DirectLoginResult{
		SupplierID: 7,
		Origin:     "https://relay.example.com",
		APIBaseURL: "https://relay.example.com/api/v1",
		SessionBundle: map[string]any{
			"origin":         "https://relay.example.com",
			"api_base_url":   "https://relay.example.com/api/v1",
			"access_token":   "direct-access-token",
			"session_source": "direct_login",
			"context": map[string]any{
				"api_base_url":   "https://relay.example.com/api/v1",
				"login_method":   "direct_login",
				"session_source": "direct_login",
			},
			"tokens": map[string]any{"access_token": "direct-access-token"},
		},
		CapturedAt:  capturedAt,
		Diagnostics: map[string]any{"login_endpoint": "redacted"},
	}}
	svc := NewServiceWithDependencies(repo, sessionTestCipher{}, supplier, nil, login)

	result, err := svc.Login(context.Background(), LoginInput{
		SupplierID: 7,
		Username:   "ops@example.com",
		Password:   "secret",
	})

	require.NoError(t, err)
	require.Equal(t, adminplusdomain.SupplierSessionSourceDirectLogin, result.Session.SessionSource)
	require.Equal(t, "https://relay.example.com", result.Session.Origin)
	require.Equal(t, "https://relay.example.com/api/v1", result.Session.APIBaseURL)
	require.Equal(t, capturedAt, result.Session.CapturedAt)
	require.Equal(t, true, result.Session.SessionSummary["has_access_token"])
	require.Equal(t, "direct_login", result.Session.SessionSummary["session_source"])
	require.NotContains(t, result.Session.SessionBundleCiphertext, "direct-access-token")

	input, err := svc.DecryptedProbeInput(context.Background(), 7)
	require.NoError(t, err)
	require.Equal(t, "direct-access-token", input.Bundle["access_token"])
	require.Equal(t, "https://relay.example.com/api/v1", input.APIBaseURL)
}

func TestServiceLoginSupportsNewAPI(t *testing.T) {
	repo := NewMemoryRepository()
	supplier := &sessionTestSupplierLookup{supplier: &adminplusdomain.Supplier{
		ID:           9,
		Name:         "New API",
		Type:         adminplusdomain.SupplierTypeNewAPI,
		DashboardURL: "https://newapi.example.com",
		APIBaseURL:   "https://newapi.example.com",
	}}
	capturedAt := time.Date(2026, 6, 22, 9, 0, 0, 0, time.UTC)
	login := &sessionTestLoginAdapter{result: &ports.DirectLoginResult{
		SupplierID: 9,
		Origin:     "https://newapi.example.com",
		APIBaseURL: "https://newapi.example.com",
		SessionBundle: map[string]any{
			"provider_type":  "new_api",
			"origin":         "https://newapi.example.com",
			"api_base_url":   "https://newapi.example.com",
			"session_source": "direct_login",
			"cookies": []any{
				map[string]any{"name": "session", "value": "signed-session"},
			},
			"required_headers": map[string]any{
				"New-Api-User": "42",
			},
			"context": map[string]any{
				"api_base_url":   "https://newapi.example.com",
				"login_method":   "direct_login",
				"session_source": "direct_login",
				"provider_type":  "new_api",
				"user_id":        "42",
			},
		},
		CapturedAt: capturedAt,
	}}
	svc := NewServiceWithDependencies(repo, sessionTestCipher{}, supplier, nil, login)

	result, err := svc.Login(context.Background(), LoginInput{
		SupplierID: 9,
		Username:   "ops@example.com",
		Password:   "secret",
	})

	require.NoError(t, err)
	require.Equal(t, adminplusdomain.SupplierSessionSourceDirectLogin, result.Session.SessionSource)
	require.Equal(t, "https://newapi.example.com", result.Session.Origin)
	require.Equal(t, "new_api", result.Session.SessionSummary["provider_type"])
	require.Equal(t, "new_api", result.Session.SessionSummary["system_type"])
	require.Equal(t, true, result.Session.SessionSummary["has_new_api_user_header"])
	require.Equal(t, 1, result.Session.SessionSummary["cookie_count"])
	require.Equal(t, "42", result.Session.SessionSummary["user_id"])
	require.Equal(t, "new_api", login.input.LoginContext["provider_type"])
	require.Equal(t, "new_api", login.input.LoginContext["supplier_type"])
	require.NotContains(t, result.Session.SessionBundleCiphertext, "signed-session")
}

func TestServiceLoginPreservesSecretsAndNormalizesOrigin(t *testing.T) {
	repo := NewMemoryRepository()
	supplier := &sessionTestSupplierLookup{supplier: &adminplusdomain.Supplier{
		ID:           7,
		Name:         "Relay",
		Type:         adminplusdomain.SupplierTypeSub2API,
		DashboardURL: "https://relay.example.com/login",
		APIBaseURL:   "https://relay.example.com/api/v1",
		Credential: adminplusdomain.SupplierCredentialStatus{
			BrowserLoginEnabled:            true,
			BrowserLoginUsernameConfigured: true,
			BrowserLoginPasswordConfigured: true,
			BrowserLoginTokenConfigured:    true,
		},
	}, credential: &adminplusdomain.SupplierBrowserCredential{
		SupplierID:   7,
		SupplierName: "Relay",
		Type:         adminplusdomain.SupplierTypeSub2API,
		DashboardURL: "https://relay.example.com/login",
		APIBaseURL:   "https://relay.example.com/api/v1",
		Username:     "ops@example.com",
		Password:     " secret-with-spaces ",
		Token:        " token-with-spaces ",
	}}
	login := &sessionTestLoginAdapter{result: &ports.DirectLoginResult{
		SupplierID: 7,
		Origin:     "https://relay.example.com",
		APIBaseURL: "https://relay.example.com/api/v1",
		SessionBundle: map[string]any{
			"origin":         "https://relay.example.com",
			"api_base_url":   "https://relay.example.com/api/v1",
			"access_token":   "direct-access-token",
			"session_source": "direct_login",
			"context":        map[string]any{"login_method": "direct_login"},
			"tokens":         map[string]any{"access_token": "direct-access-token"},
		},
		CapturedAt: time.Date(2026, 6, 22, 9, 0, 0, 0, time.UTC),
	}}
	svc := NewServiceWithDependencies(repo, sessionTestCipher{}, supplier, nil, login)

	_, err := svc.Login(context.Background(), LoginInput{SupplierID: 7})

	require.NoError(t, err)
	require.Equal(t, "ops@example.com", login.input.Username)
	require.Equal(t, " secret-with-spaces ", login.input.Password)
	require.Equal(t, " token-with-spaces ", login.input.Token)
	require.Equal(t, "https://relay.example.com", login.input.Origin)
}

func TestServiceDecryptedProbeInputReportsDecryptFailure(t *testing.T) {
	repo := NewMemoryRepository()
	supplier := &sessionTestSupplierLookup{supplier: &adminplusdomain.Supplier{
		ID:           7,
		Name:         "Relay",
		Type:         adminplusdomain.SupplierTypeSub2API,
		DashboardURL: "https://relay.example.com",
		APIBaseURL:   "https://relay.example.com/api/v1",
	}}
	svc := NewServiceWithDependencies(repo, failingSessionCipher{}, supplier, nil)

	_, err := repo.Upsert(context.Background(), &adminplusdomain.SupplierBrowserSession{
		SupplierID:              7,
		SessionSource:           adminplusdomain.SupplierSessionSourceDirectLogin,
		Origin:                  "https://relay.example.com",
		APIBaseURL:              "https://relay.example.com/api/v1",
		SessionBundleCiphertext: "stale-ciphertext",
		CapturedAt:              time.Date(2026, 6, 21, 10, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)

	_, err = svc.DecryptedProbeInput(context.Background(), 7)
	require.Error(t, err)
	require.Contains(t, err.Error(), "SUPPLIER_SESSION_DECRYPT_FAILED")
	require.NotContains(t, err.Error(), "stale-ciphertext")
}

func TestServiceLoginFailureRecordsDiagnostics(t *testing.T) {
	repo := NewMemoryRepository()
	supplier := &sessionTestSupplierLookup{supplier: &adminplusdomain.Supplier{
		ID:           7,
		Name:         "Relay",
		Type:         adminplusdomain.SupplierTypeSub2API,
		DashboardURL: "https://relay.example.com",
		APIBaseURL:   "https://relay.example.com/api/v1",
	}}
	loginErr := infraerrors.New(http.StatusUnauthorized, "LOGIN_CREDENTIAL_INVALID", "supplier direct login credential is invalid").
		WithMetadata(map[string]string{
			"endpoint":     "https://supplier.example/api/v1/auth/login",
			"status_code":  "401",
			"content_type": "application/json",
			"body_type":    "json",
			"body_excerpt": `{"code":401,"message":"invalid email or password","reason":"INVALID_CREDENTIALS"}`,
		})
	writer := &sessionLogWriter{}
	svc := NewServiceWithDependencies(repo, sessionTestCipher{}, supplier, nil, &sessionTestLoginAdapter{err: loginErr}).
		WithDiagnostics(bizlogs.NewRecorder(writer))

	result, err := svc.Login(context.Background(), LoginInput{
		SupplierID: 7,
		Username:   "ops@example.com",
		Password:   "secret",
	})

	require.Nil(t, result)
	require.Error(t, err)
	require.Len(t, writer.inputs, 1)
	input := writer.inputs[0]
	require.Equal(t, "warn", input.Level)
	require.Equal(t, "admin_plus.login", input.Component)
	require.NotContains(t, input.ExtraJSON, "secret")
	var extra map[string]any
	require.NoError(t, json.Unmarshal([]byte(input.ExtraJSON), &extra))
	require.Equal(t, "direct_login", extra["action"])
	require.Equal(t, "failed", extra["outcome"])
	require.Equal(t, "LOGIN_CREDENTIAL_INVALID", extra["reason"])
	require.Equal(t, "https://supplier.example/api/v1/auth/login", extra["endpoint"])
	require.Equal(t, float64(401), extra["status_code"])
	require.Equal(t, "Relay", extra["supplier_name"])
	require.Equal(t, "sub2api", extra["provider_type"])
	require.Contains(t, extra["body_excerpt"], "INVALID_CREDENTIALS")
}

type sessionTestSupplierLookup struct {
	supplier   *adminplusdomain.Supplier
	credential *adminplusdomain.SupplierBrowserCredential
}

func (s *sessionTestSupplierLookup) Get(_ context.Context, id int64) (*adminplusdomain.Supplier, error) {
	if s.supplier == nil || s.supplier.ID != id {
		return nil, nil
	}
	cp := *s.supplier
	return &cp, nil
}

func (s *sessionTestSupplierLookup) GetBrowserCredential(_ context.Context, id int64) (*adminplusdomain.SupplierBrowserCredential, error) {
	if s.credential != nil {
		cp := *s.credential
		return &cp, nil
	}
	return &adminplusdomain.SupplierBrowserCredential{
		SupplierID:   id,
		DashboardURL: s.supplier.DashboardURL,
		APIBaseURL:   s.supplier.APIBaseURL,
		Username:     "ops@example.com",
		Password:     "secret",
	}, nil
}

type sessionTestLoginAdapter struct {
	result *ports.DirectLoginResult
	err    error
	input  ports.DirectLoginInput
}

func (a *sessionTestLoginAdapter) DirectLogin(_ context.Context, in ports.DirectLoginInput) (*ports.DirectLoginResult, error) {
	a.input = in
	return a.result, a.err
}

type sessionTestCipher struct{}

func (sessionTestCipher) Encrypt(plaintext string) (string, error) {
	return base64.StdEncoding.EncodeToString([]byte(plaintext)), nil
}

func (sessionTestCipher) Decrypt(ciphertext string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

type failingSessionCipher struct{}

func (failingSessionCipher) Encrypt(plaintext string) (string, error) {
	return plaintext, nil
}

func (failingSessionCipher) Decrypt(string) (string, error) {
	return "", errors.New("decrypt: message authentication failed")
}

type sessionLogWriter struct {
	inputs []*service.OpsInsertSystemLogInput
}

func (w *sessionLogWriter) BatchInsertSystemLogs(_ context.Context, inputs []*service.OpsInsertSystemLogInput) (int64, error) {
	w.inputs = append(w.inputs, inputs...)
	return int64(len(inputs)), nil
}
