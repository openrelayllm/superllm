package suppliers

import (
	"context"
	"net/http"
	"strings"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func (s *Service) GetBrowserCredential(ctx context.Context, id int64) (*adminplusdomain.SupplierBrowserCredential, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("supplier service is not configured")
	}
	if id <= 0 {
		return nil, badRequest("SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	credential, err := s.repo.GetBrowserCredential(ctx, id)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(credential.DashboardURL) == "" {
		return nil, infraerrors.New(http.StatusConflict, "SUPPLIER_DASHBOARD_URL_REQUIRED", "supplier dashboard url is required for browser automation")
	}
	if strings.TrimSpace(credential.Username) == "" && !nonBlankSecret(credential.Password) && !nonBlankSecret(credential.Token) {
		return nil, infraerrors.New(http.StatusConflict, "SUPPLIER_BROWSER_CREDENTIAL_REQUIRED", "supplier browser credential is required")
	}
	return credential, nil
}

func supplierCredentialStatusFromCreateInput(in CreateSupplierInput) adminplusdomain.SupplierCredentialStatus {
	return adminplusdomain.SupplierCredentialStatus{
		PostgresConfigured:             strings.TrimSpace(in.PostgresReadDSN) != "",
		RedisConfigured:                strings.TrimSpace(in.RedisReadDSN) != "",
		BrowserLoginEnabled:            in.BrowserLoginEnabled,
		BrowserLoginUsernameConfigured: strings.TrimSpace(in.BrowserLoginUsername) != "",
		BrowserLoginPasswordConfigured: nonBlankSecret(in.BrowserLoginPassword),
		BrowserLoginTokenConfigured:    nonBlankSecret(in.BrowserLoginToken),
		MaskedBrowserLoginUsername:     maskUsername(in.BrowserLoginUsername),
	}
}

func applySupplierCredentialUpdate(updated *adminplusdomain.Supplier, existing *adminplusdomain.Supplier, in UpdateSupplierInput) {
	if updated == nil || existing == nil {
		return
	}
	updated.Credential.PostgresConfigured = strings.TrimSpace(in.PostgresReadDSN) != "" || existing.Credential.PostgresConfigured
	updated.Credential.RedisConfigured = strings.TrimSpace(in.RedisReadDSN) != "" || existing.Credential.RedisConfigured
	updated.Credential.BrowserLoginEnabled = in.BrowserLoginEnabled
	if updated.BrowserLoginUsername != "" {
		updated.Credential.BrowserLoginUsernameConfigured = true
		updated.Credential.MaskedBrowserLoginUsername = maskUsername(updated.BrowserLoginUsername)
	}
	if nonBlankSecret(updated.BrowserLoginPassword) {
		updated.Credential.BrowserLoginPasswordConfigured = true
	}
	if nonBlankSecret(updated.BrowserLoginToken) {
		updated.Credential.BrowserLoginTokenConfigured = true
	}
	if updated.Credential.MaskedBrowserLoginUsername == "" {
		updated.Credential.MaskedBrowserLoginUsername = existing.Credential.MaskedBrowserLoginUsername
	}
}
