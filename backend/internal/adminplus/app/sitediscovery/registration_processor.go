package sitediscovery

import (
	"context"
	"net/http"
	"strings"
	"time"

	suppliersapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/suppliers"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type RegistrationProcessor struct {
	repo      Repository
	suppliers *suppliersapp.Service
	cipher    CredentialCipher
	now       func() time.Time
}

func NewRegistrationProcessor(repo Repository, suppliers *suppliersapp.Service, cipher CredentialCipher) *RegistrationProcessor {
	return &RegistrationProcessor{
		repo:      repo,
		suppliers: suppliers,
		cipher:    cipher,
		now:       time.Now,
	}
}

func (p *RegistrationProcessor) ProcessRegistrationTaskResult(ctx context.Context, task *adminplusdomain.ExtensionTask, result map[string]any) (map[string]any, error) {
	if p == nil || p.repo == nil {
		return nil, internalError("site discovery registration dependencies are not configured")
	}
	if task == nil {
		return nil, badRequest("EXTENSION_TASK_REQUIRED", "extension task is required")
	}
	credential, item, err := p.repo.GetRegistrationCredentialByTaskID(ctx, task.ID)
	if err != nil {
		return nil, err
	}
	if credential == nil || item == nil {
		return nil, infraerrors.New(http.StatusNotFound, "SITE_DISCOVERY_REGISTRATION_CREDENTIAL_NOT_FOUND", "registration credential not found")
	}
	attemptedAt := p.now().UTC()
	if !boolValue(result, "registration_submitted") {
		updated, err := p.repo.CompleteRegistration(ctx, credential.ID, credential.SupplierID, adminplusdomain.SupplierRegistrationStatusFailed, "REGISTRATION_RESULT_INCOMPLETE", "registration result is incomplete", attemptedAt)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"registration_status": string(updated.Status),
			"registration_error":  updated.ErrorCode,
		}, nil
	}
	if p.suppliers == nil {
		return nil, internalError("supplier service is not configured")
	}
	password, err := p.decryptRegistrationPassword(credential)
	if err != nil {
		return nil, err
	}
	supplier, err := p.ensureRegisteredSupplier(ctx, item, credential.Email, password)
	if err != nil {
		return nil, err
	}
	if supplier == nil {
		return nil, internalError("failed to import registered supplier")
	}
	updated, err := p.repo.CompleteRegistration(ctx, credential.ID, supplier.ID, adminplusdomain.SupplierRegistrationStatusSucceeded, "", "", attemptedAt)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"registration_status": string(updated.Status),
		"supplier_id":         supplier.ID,
		"supplier_name":       supplier.Name,
		"supplier_imported":   true,
	}, nil
}

func (p *RegistrationProcessor) ProcessRegistrationTaskFailure(ctx context.Context, task *adminplusdomain.ExtensionTask, errorCode string, errorMessage string) (map[string]any, error) {
	if p == nil || p.repo == nil {
		return nil, internalError("site discovery registration dependencies are not configured")
	}
	if task == nil {
		return nil, badRequest("EXTENSION_TASK_REQUIRED", "extension task is required")
	}
	credential, _, err := p.repo.GetRegistrationCredentialByTaskID(ctx, task.ID)
	if err != nil {
		return nil, err
	}
	if credential == nil {
		return nil, infraerrors.New(http.StatusNotFound, "SITE_DISCOVERY_REGISTRATION_CREDENTIAL_NOT_FOUND", "registration credential not found")
	}
	status := adminplusdomain.SupplierRegistrationStatusFailed
	verificationStatus := strings.TrimSpace(errorCode)
	if verificationStatus == "REGISTRATION_VERIFICATION_REQUIRED" {
		status = adminplusdomain.SupplierRegistrationStatusWaitingManualVerification
	}
	updated, err := p.repo.CompleteRegistration(ctx, credential.ID, credential.SupplierID, status, verificationStatus, errorMessage, p.now().UTC())
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"registration_status": string(updated.Status),
		"manual_required":     updated.Status == adminplusdomain.SupplierRegistrationStatusWaitingManualVerification,
	}, nil
}

func (p *RegistrationProcessor) decryptRegistrationPassword(credential *adminplusdomain.SupplierRegistrationCredential) (string, error) {
	if credential == nil {
		return "", infraerrors.New(http.StatusNotFound, "SITE_DISCOVERY_REGISTRATION_CREDENTIAL_NOT_FOUND", "registration credential not found")
	}
	if p.cipher == nil {
		return "", internalError("registration credential cipher is not configured")
	}
	password, err := p.cipher.Decrypt(credential.PasswordCiphertext)
	if err != nil {
		return "", infraerrors.New(http.StatusInternalServerError, "SITE_DISCOVERY_PASSWORD_DECRYPT_FAILED", "failed to decrypt registration password")
	}
	return password, nil
}

func (p *RegistrationProcessor) ensureRegisteredSupplier(ctx context.Context, item *adminplusdomain.SiteDiscoveryItem, email string, password string) (*adminplusdomain.Supplier, error) {
	if item == nil {
		return nil, infraerrors.New(http.StatusNotFound, "SITE_DISCOVERY_ITEM_NOT_FOUND", "site discovery item not found")
	}
	ensured, err := p.suppliers.EnsureFromSiteCandidate(ctx, suppliersapp.CreateFromSiteCandidateInput{
		Name:         item.Name,
		Type:         item.ProviderType,
		DashboardURL: firstNonEmpty(item.DashboardURL, item.RegisterURL, item.APIBaseURL),
		APIBaseURL:   item.APIBaseURL,
		SourceHost:   item.Host,
		SourceURL:    item.RegisterURL,
		Title:        item.Name,
	})
	if err != nil {
		return nil, err
	}
	if ensured == nil || ensured.Supplier == nil {
		return nil, internalError("failed to import discovered supplier")
	}
	supplier := ensured.Supplier
	updated, err := p.suppliers.Update(ctx, supplier.ID, suppliersapp.UpdateSupplierInput{
		Name:                  supplier.Name,
		Kind:                  supplier.Kind,
		Type:                  supplier.Type,
		RuntimeStatus:         supplier.RuntimeStatus,
		HealthStatus:          supplier.HealthStatus,
		DashboardURL:          supplier.DashboardURL,
		APIBaseURL:            supplier.APIBaseURL,
		ThirdPartyRechargeURL: supplier.ThirdPartyRechargeURL,
		LocalRechargeURL:      supplier.LocalRechargeURL,
		Contact:               supplier.Contact,
		Notes:                 supplier.Notes,
		BrowserLoginEnabled:   true,
		BrowserLoginUsername:  email,
		BrowserLoginPassword:  password,
		BalanceCents:          supplier.BalanceCents,
		BalanceCurrency:       supplier.BalanceCurrency,
		RechargeMultiplier:    supplier.RechargeMultiplier,
	})
	if err != nil {
		return nil, err
	}
	if _, err := p.repo.LinkSupplier(ctx, item.ID, updated.ID); err != nil {
		return nil, err
	}
	return updated, nil
}
