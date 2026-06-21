package sessions

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/adminplus/ports"
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
}

func (a *sessionTestLoginAdapter) DirectLogin(_ context.Context, _ ports.DirectLoginInput) (*ports.DirectLoginResult, error) {
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
