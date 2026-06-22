package providerrouter

import (
	"context"
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
	return r.sub2api.ProbeSub2APIUserProfile(ctx, in)
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
		return nil, capabilityMissing("SUPPLIER_USAGE_COST_CAPABILITY_MISSING", "new api usage cost reading is not implemented")
	}
	if r == nil || r.sub2api == nil {
		return nil, internalError()
	}
	return r.sub2api.ReadUsageCosts(ctx, in, request)
}

func (r *Router) ReadFundingTransactions(ctx context.Context, in ports.SessionProbeInput, request ports.ReadFundingTransactionsInput) (*ports.ReadFundingTransactionsResult, error) {
	if providerTypeFromBundle(in.Bundle) == "new_api" {
		return nil, capabilityMissing("SUPPLIER_FUNDING_CAPABILITY_MISSING", "new api funding transaction reading is not implemented")
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
	return normalizeProviderType(firstNonEmpty(
		stringValue(bundle, "provider_type"),
		stringValue(bundle, "system_type"),
		stringValueAt(bundle, "context", "provider_type"),
		stringValueAt(bundle, "context", "system_type"),
	))
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
