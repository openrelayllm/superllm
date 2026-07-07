package provider

import (
	"context"
	"encoding/json"
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
	var firstErr error
	for _, endpointPath := range []string{"/api/user/self/groups", "/api/user_group_map"} {
		endpoint, err := buildEndpointURL(apiBaseURL, endpointPath)
		if err != nil {
			return nil, err
		}
		raw, err := c.doSessionJSON(ctx, http.MethodGet, endpoint, in.Bundle)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		groups, err := parseNewAPIGroupResponse(raw)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if len(groups) == 0 {
			continue
		}
		return &ports.ReadGroupsResult{
			SupplierID: in.SupplierID,
			SystemType: "new_api",
			Origin:     origin,
			APIBaseURL: apiBaseURL,
			Groups:     groups,
			CapturedAt: c.now().UTC(),
		}, nil
	}
	if firstErr != nil {
		return nil, firstErr
	}
	return &ports.ReadGroupsResult{
		SupplierID: in.SupplierID,
		SystemType: "new_api",
		Origin:     origin,
		APIBaseURL: apiBaseURL,
		Groups:     nil,
		CapturedAt: c.now().UTC(),
	}, nil
}

func parseNewAPIGroupResponse(raw []byte) ([]*ports.ProviderGroup, error) {
	var root map[string]any
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	if err := decoder.Decode(&root); err != nil {
		return nil, infraerrors.New(http.StatusBadGateway, "SUPPLIER_GROUP_RESPONSE_INVALID", "new api groups response is invalid").WithCause(err)
	}
	if success, ok := boolValue(root["success"]); ok && !success {
		return nil, classifySessionBusinessFailure(stringFromAny(root["message"]))
	}
	source := root["data"]
	if source == nil {
		source = root
	}
	return parseGroupsFromAny(source), nil
}

func parseGroupsFromAny(value any) []*ports.ProviderGroup {
	switch typed := value.(type) {
	case map[string]any:
		return parseGroups(typed)
	case []any:
		groups := make([]*ports.ProviderGroup, 0, len(typed))
		for _, raw := range typed {
			switch item := raw.(type) {
			case string:
				if group := buildNewAPIGroup(item, nil); group != nil {
					groups = append(groups, group)
				}
			case map[string]any:
				name := firstNonEmpty(stringFromAny(firstExisting(item, "id", "group", "group_id", "groupId", "name", "group_name", "groupName")))
				if group := buildNewAPIGroup(name, item); group != nil {
					groups = append(groups, group)
				}
			}
		}
		return groups
	default:
		return nil
	}
}

func parseGroups(data map[string]any) []*ports.ProviderGroup {
	groups := make([]*ports.ProviderGroup, 0, len(data))
	for name, raw := range data {
		groupName := strings.TrimSpace(name)
		if groupName == "" || isNewAPIEnvelopeKey(groupName) {
			continue
		}
		rawMap, _ := raw.(map[string]any)
		if group := buildNewAPIGroup(groupName, rawMap); group != nil {
			groups = append(groups, group)
		}
	}
	return groups
}

func buildNewAPIGroup(groupName string, rawMap map[string]any) *ports.ProviderGroup {
	groupName = strings.TrimSpace(groupName)
	if groupName == "" {
		return nil
	}
	ratio, ok := float64Value(firstExisting(rawMap,
		"ratio",
		"rate",
		"multiplier",
		"rate_multiplier",
		"group_ratio",
		"groupRate",
		"group_rate",
		"effective_rate_multiplier",
		"effectiveRateMultiplier",
	))
	if !ok || ratio <= 0 {
		ratio = 1
	}
	userRatio := ratio
	description := firstNonEmpty(stringFromAny(firstExisting(rawMap, "desc", "description")), groupName)
	return &ports.ProviderGroup{
		ExternalGroupID:         groupName,
		Name:                    groupName,
		Description:             description,
		ProviderFamily:          "new_api",
		RateMultiplier:          ratio,
		UserRateMultiplier:      &userRatio,
		EffectiveRateMultiplier: ratio,
		Status:                  "active",
		RawPayload:              cloneMap(rawMap),
	}
}

func isNewAPIEnvelopeKey(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "success", "message", "code", "data", "error":
		return true
	default:
		return false
	}
}
