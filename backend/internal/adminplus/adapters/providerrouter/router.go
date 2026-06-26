package providerrouter

import (
	"context"
	"errors"
	"net/http"
	"strings"

	newapiprovider "github.com/Wei-Shaw/sub2api/internal/adminplus/adapters/newapi/provider"
	sub2apiprovider "github.com/Wei-Shaw/sub2api/internal/adminplus/adapters/sub2api/provider"
	"github.com/Wei-Shaw/sub2api/internal/adminplus/ports"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type Router struct {
	sub2api *sub2apiprovider.SessionProfileClient
	newapi  *newapiprovider.Client
}

func New(sub2api *sub2apiprovider.SessionProfileClient, newapi *newapiprovider.Client) *Router {
	return &Router{sub2api: sub2api, newapi: newapi}
}

func (r *Router) DirectLogin(ctx context.Context, in ports.DirectLoginInput) (*ports.DirectLoginResult, error) {
	switch providerTypeFromLogin(in) {
	case "new_api":
		if r == nil || r.newapi == nil {
			return nil, internalError()
		}
		return r.newapi.DirectLogin(ctx, in)
	default:
		if r == nil || r.sub2api == nil {
			return nil, internalError()
		}
		return r.sub2api.DirectLogin(ctx, in)
	}
}

func (r *Router) RegisterAccount(ctx context.Context, in ports.DirectRegistrationInput) (*ports.DirectRegistrationResult, error) {
	switch normalizeProviderType(string(in.ProviderType)) {
	case "new_api":
		if r == nil || r.newapi == nil {
			return nil, internalError()
		}
		return r.newapi.RegisterAccount(ctx, in)
	case "sub2api", "":
		if r == nil || r.sub2api == nil {
			return nil, internalError()
		}
		return r.sub2api.RegisterAccount(ctx, in)
	default:
		return nil, capabilityMissing("SUPPLIER_DIRECT_REGISTRATION_UNSUPPORTED", "supplier type does not support direct registration")
	}
}

func (r *Router) ProbeSub2APIUserProfile(ctx context.Context, in ports.SessionProbeInput) (*ports.SessionProbeResult, error) {
	if providerTypeFromBundle(in.Bundle) == "new_api" {
		if r == nil || r.newapi == nil {
			return nil, internalError()
		}
		return r.newapi.ProbeSub2APIUserProfile(ctx, in)
	}
	if r == nil || r.sub2api == nil {
		return nil, internalError()
	}
	probe, err := r.sub2api.ProbeSub2APIUserProfile(ctx, in)
	if err == nil {
		return probe, nil
	}
	if !shouldFallbackProfileProbeToNewAPI(err, in.Bundle) {
		return nil, err
	}
	if r.newapi == nil {
		return nil, err
	}
	fallbackProbe, fallbackErr := r.newapi.ProbeSub2APIUserProfile(ctx, in)
	if fallbackErr != nil {
		if hasNewAPISessionEvidence(in.Bundle) {
			return nil, withProfileFallbackErrorDiagnostics(fallbackErr, err)
		}
		return nil, err
	}
	addProfileFallbackDiagnostics(fallbackProbe, err)
	return fallbackProbe, nil
}

func (r *Router) ReadGroups(ctx context.Context, in ports.SessionProbeInput) (*ports.ReadGroupsResult, error) {
	if providerTypeFromBundle(in.Bundle) == "new_api" {
		if r == nil || r.newapi == nil {
			return nil, internalError()
		}
		return r.newapi.ReadGroups(ctx, in)
	}
	if r == nil || r.sub2api == nil {
		return nil, internalError()
	}
	return r.sub2api.ReadGroups(ctx, in)
}

func (r *Router) ReadRates(ctx context.Context, in ports.SessionProbeInput) (*ports.ReadRatesResult, error) {
	if providerTypeFromBundle(in.Bundle) == "new_api" {
		return nil, capabilityMissing("SUPPLIER_RATE_CAPABILITY_MISSING", "new api rate reading is not implemented")
	}
	if r == nil || r.sub2api == nil {
		return nil, internalError()
	}
	return r.sub2api.ReadRates(ctx, in)
}

func (r *Router) ReadAnnouncements(ctx context.Context, in ports.SessionProbeInput) (*ports.ReadAnnouncementsResult, error) {
	if providerTypeFromBundle(in.Bundle) == "new_api" {
		return nil, capabilityMissing("SUPPLIER_ANNOUNCEMENT_CAPABILITY_MISSING", "new api announcement reading is not implemented")
	}
	if r == nil || r.sub2api == nil {
		return nil, internalError()
	}
	return r.sub2api.ReadAnnouncements(ctx, in)
}

func (r *Router) ReadUsageCosts(ctx context.Context, in ports.SessionProbeInput, request ports.ReadUsageCostsInput) (*ports.ReadUsageCostsResult, error) {
	if providerTypeFromBundle(in.Bundle) == "new_api" {
		if r == nil || r.newapi == nil {
			return nil, internalError()
		}
		return r.newapi.ReadUsageCosts(ctx, in, request)
	}
	if r == nil || r.sub2api == nil {
		return nil, internalError()
	}
	return r.sub2api.ReadUsageCosts(ctx, in, request)
}

func (r *Router) ReadFundingTransactions(ctx context.Context, in ports.SessionProbeInput, request ports.ReadFundingTransactionsInput) (*ports.ReadFundingTransactionsResult, error) {
	if providerTypeFromBundle(in.Bundle) == "new_api" {
		if r == nil || r.newapi == nil {
			return nil, internalError()
		}
		return r.newapi.ReadFundingTransactions(ctx, in, request)
	}
	if r == nil || r.sub2api == nil {
		return nil, internalError()
	}
	return r.sub2api.ReadFundingTransactions(ctx, in, request)
}

func (r *Router) ReadEntitlementTransactions(ctx context.Context, in ports.SessionProbeInput, request ports.ReadEntitlementTransactionsInput) (*ports.ReadEntitlementTransactionsResult, error) {
	if providerTypeFromBundle(in.Bundle) == "new_api" {
		return nil, capabilityMissing("SUPPLIER_ENTITLEMENT_CAPABILITY_MISSING", "new api entitlement transaction reading is not implemented")
	}
	if r == nil || r.sub2api == nil {
		return nil, internalError()
	}
	return r.sub2api.ReadEntitlementTransactions(ctx, in, request)
}

func (r *Router) CreateKey(ctx context.Context, in ports.SessionProbeInput, request ports.CreateProviderKeyInput) (*ports.ProviderKeyResult, error) {
	if providerTypeFromBundle(in.Bundle) == "new_api" {
		if r == nil || r.newapi == nil {
			return nil, internalError()
		}
		return r.newapi.CreateKey(ctx, in, request)
	}
	if r == nil || r.sub2api == nil {
		return nil, internalError()
	}
	return r.sub2api.CreateKey(ctx, in, request)
}

func (r *Router) RenameKey(ctx context.Context, in ports.SessionProbeInput, request ports.RenameProviderKeyInput) (*ports.ProviderKeyResult, error) {
	if providerTypeFromBundle(in.Bundle) == "new_api" {
		if r == nil || r.newapi == nil {
			return nil, internalError()
		}
		return r.newapi.RenameKey(ctx, in, request)
	}
	if r == nil || r.sub2api == nil {
		return nil, internalError()
	}
	return r.sub2api.RenameKey(ctx, in, request)
}

func (r *Router) ReadChannelMonitors(ctx context.Context, in ports.SessionProbeInput) (*ports.ReadChannelMonitorsResult, error) {
	if providerTypeFromBundle(in.Bundle) == "new_api" {
		if r == nil || r.newapi == nil {
			return nil, internalError()
		}
		return r.newapi.ReadChannelMonitors(ctx, in)
	}
	if r == nil || r.sub2api == nil {
		return nil, internalError()
	}
	return r.sub2api.ReadChannelMonitors(ctx, in)
}

func providerTypeFromLogin(in ports.DirectLoginInput) string {
	return normalizeProviderType(firstNonEmpty(
		stringValue(in.LoginContext, "provider_type"),
		stringValue(in.LoginContext, "system_type"),
		stringValue(in.LoginContext, "supplier_type"),
	))
}

func providerTypeFromBundle(bundle map[string]any) string {
	explicit := normalizeProviderType(firstNonEmpty(
		stringValue(bundle, "provider_type"),
		stringValue(bundle, "system_type"),
		stringValueAt(bundle, "context", "provider_type"),
		stringValueAt(bundle, "context", "system_type"),
	))
	if explicit != "" {
		return explicit
	}
	if hasNewAPISessionEvidence(bundle) {
		return "new_api"
	}
	return ""
}

func normalizeProviderType(value string) string {
	v := strings.ToLower(strings.TrimSpace(value))
	switch v {
	case "newapi", "new-api":
		return "new_api"
	default:
		return v
	}
}

func internalError() error {
	return infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_INTERNAL_ERROR", "provider router is not configured")
}

func capabilityMissing(reason string, message string) error {
	return infraerrors.New(http.StatusConflict, reason, message)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func stringValue(in map[string]any, key string) string {
	if in == nil {
		return ""
	}
	if value, ok := in[key].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func stringValueAt(in map[string]any, path ...string) string {
	var current any = in
	for _, key := range path {
		obj, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current = obj[key]
	}
	if value, ok := current.(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func shouldFallbackProfileProbeToNewAPI(err error, bundle map[string]any) bool {
	reason := infraerrors.Reason(err)
	if hasNewAPISessionEvidence(bundle) {
		return reason != ""
	}
	switch reason {
	case "SUPPLIER_SESSION_PROBE_HTML", "SUPPLIER_SESSION_PROBE_BAD_STATUS", "SUPPLIER_SESSION_PROFILE_INVALID":
		return true
	default:
		return false
	}
}

func hasNewAPISessionEvidence(bundle map[string]any) bool {
	if bundle == nil {
		return false
	}
	requiredHeaders := mapValue(bundle, "required_headers")
	if firstNonEmpty(
		stringValue(requiredHeaders, "New-Api-User"),
		stringValue(requiredHeaders, "New-API-User"),
		stringValue(requiredHeaders, "new-api-user"),
	) != "" {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(stringValue(bundle, "auth_header_name")), "New-Api-User")
}

func mapValue(in map[string]any, key string) map[string]any {
	if in == nil {
		return nil
	}
	if value, ok := in[key].(map[string]any); ok {
		return value
	}
	return nil
}

func addProfileFallbackDiagnostics(probe *ports.SessionProbeResult, sub2apiErr error) {
	if probe == nil {
		return
	}
	if probe.Diagnostics == nil {
		probe.Diagnostics = map[string]any{}
	}
	sub2apiAppErr := infraerrors.FromError(sub2apiErr)
	probe.Diagnostics["fallback_from"] = "sub2api"
	probe.Diagnostics["fallback_reason"] = sub2apiAppErr.Reason
	probe.Diagnostics["fallback_message"] = sub2apiAppErr.Message
	if endpoint := sub2apiAppErr.Metadata["endpoint"]; endpoint != "" {
		probe.Diagnostics["fallback_endpoint"] = endpoint
	}
	if statusCode := sub2apiAppErr.Metadata["status_code"]; statusCode != "" {
		probe.Diagnostics["fallback_status_code"] = statusCode
	}
}

func withProfileFallbackErrorDiagnostics(err error, sub2apiErr error) error {
	if err == nil {
		return nil
	}
	var appErr *infraerrors.ApplicationError
	if !errors.As(err, &appErr) {
		return err
	}
	sub2apiAppErr := infraerrors.FromError(sub2apiErr)
	metadata := make(map[string]string, len(appErr.Metadata)+4)
	for key, value := range appErr.Metadata {
		metadata[key] = value
	}
	metadata["fallback_from"] = "sub2api"
	metadata["fallback_reason"] = sub2apiAppErr.Reason
	metadata["fallback_message"] = sub2apiAppErr.Message
	if endpoint := sub2apiAppErr.Metadata["endpoint"]; endpoint != "" {
		metadata["fallback_endpoint"] = endpoint
	}
	return appErr.WithMetadata(metadata)
}
