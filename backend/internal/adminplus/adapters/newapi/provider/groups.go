package provider

import (
	"context"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/adminplus/ports"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func (c *Client) ReadGroups(ctx context.Context, in ports.SessionProbeInput) (*ports.ReadGroupsResult, error) {
	if c == nil || c.httpClient == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_INTERNAL_ERROR", "new api provider adapter is not configured")
	}
	apiBaseURL, origin, err := normalizeBaseURLs(firstNonEmpty(in.APIBaseURL, stringValueAt(in.Bundle, "context", "api_base_url"), stringValue(in.Bundle, "api_base_url")), firstNonEmpty(in.Origin, stringValue(in.Bundle, "origin")))
	if err != nil {
		return nil, err
	}
	endpoint, err := buildEndpointURL(apiBaseURL, "/api/user/self/groups")
	if err != nil {
		return nil, err
	}
	raw, err := c.doSessionJSON(ctx, http.MethodGet, endpoint, in.Bundle)
	if err != nil {
		return nil, err
	}
	envelope, err := decodeEnvelope(raw)
	if err != nil {
		return nil, infraerrors.New(http.StatusBadGateway, "SUPPLIER_GROUP_RESPONSE_INVALID", "new api groups response is invalid").WithCause(err)
	}
	if !envelope.Success {
		return nil, classifySessionBusinessFailure(envelope.Message)
	}
	groups := parseGroups(envelope.Data)
	return &ports.ReadGroupsResult{
		SupplierID: in.SupplierID,
		SystemType: "new_api",
		Origin:     origin,
		APIBaseURL: apiBaseURL,
		Groups:     groups,
		CapturedAt: c.now().UTC(),
	}, nil
}

func parseGroups(data map[string]any) []*ports.ProviderGroup {
	groups := make([]*ports.ProviderGroup, 0, len(data))
	for name, raw := range data {
		groupName := strings.TrimSpace(name)
		if groupName == "" {
			continue
		}
		rawMap, _ := raw.(map[string]any)
		ratio, ok := float64Value(firstExisting(rawMap, "ratio", "rate", "multiplier"))
		if !ok || ratio <= 0 {
			ratio = 1
		}
		userRatio := ratio
		description := firstNonEmpty(stringFromAny(firstExisting(rawMap, "desc", "description")), groupName)
		groups = append(groups, &ports.ProviderGroup{
			ExternalGroupID:         groupName,
			Name:                    groupName,
			Description:             description,
			ProviderFamily:          "new_api",
			RateMultiplier:          ratio,
			UserRateMultiplier:      &userRatio,
			EffectiveRateMultiplier: ratio,
			Status:                  "active",
			RawPayload:              cloneMap(rawMap),
		})
	}
	return groups
}
