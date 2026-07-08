package provider

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/adminplus/ports"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func (c *Client) ReadEntitlementTransactions(ctx context.Context, in ports.SessionProbeInput, request ports.ReadEntitlementTransactionsInput) (*ports.ReadEntitlementTransactionsResult, error) {
	if c == nil || c.httpClient == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_INTERNAL_ERROR", "new api provider adapter is not configured")
	}
	if request.SupplierID <= 0 {
		return nil, infraerrors.New(http.StatusBadRequest, "SUPPLIER_ID_INVALID", "invalid supplier id")
	}
	apiBaseURL, origin, err := normalizeBaseURLs(firstNonEmpty(in.APIBaseURL, stringValueAt(in.Bundle, "context", "api_base_url"), stringValue(in.Bundle, "api_base_url")), firstNonEmpty(in.Origin, stringValue(in.Bundle, "origin")))
	if err != nil {
		return nil, err
	}
	role, roleKnown := newAPIRoleFromBundle(in.Bundle)
	baseEndpoint, err := buildEndpointURL(apiBaseURL, "/api/redemption/")
	if err != nil {
		return nil, err
	}
	items := make([]ports.ProviderEntitlementTransaction, 0)
	for page := 1; page <= newAPIHistoryMaxPages; page++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		endpoint := appendNewAPIQueryValues(baseEndpoint, map[string]string{
			"p":         strconv.Itoa(page),
			"page_size": strconv.Itoa(newAPIPageSize),
		})
		raw, err := c.doSessionJSON(ctx, http.MethodGet, endpoint, in.Bundle)
		if err != nil {
			if isNewAPISessionPermissionError(err) {
				return nil, newAPISessionPermissionRequired(role, roleKnown, err)
			}
			return nil, err
		}
		envelope, err := decodeEnvelope(raw)
		if err != nil {
			return nil, infraerrors.New(http.StatusBadGateway, "SUPPLIER_ENTITLEMENT_RESPONSE_INVALID", "new api redemption response is invalid").WithCause(err)
		}
		if !envelope.Success {
			err := classifySessionBusinessFailure(envelope.Message)
			if isNewAPISessionPermissionError(err) {
				return nil, newAPISessionPermissionRequired(role, roleKnown, err)
			}
			return nil, err
		}
		pageItems := parseNewAPIEntitlementTransactions(envelope.Data)
		if len(pageItems) == 0 {
			break
		}
		items = append(items, pageItems...)
		total := int64FromAny(envelope.Data["total"])
		if total > 0 && len(items) >= int(total) {
			break
		}
		if total == 0 && len(pageItems) < newAPIPageSize {
			break
		}
		if page == newAPIHistoryMaxPages {
			return nil, newAPIPageLimitExceeded("entitlement_transactions", newAPIHistoryMaxPages, newAPIPageSize)
		}
		if err := waitForProviderPage(ctx, newAPIPageDelay); err != nil {
			return nil, err
		}
	}
	return &ports.ReadEntitlementTransactionsResult{
		SupplierID:   in.SupplierID,
		ProviderType: "new_api",
		SystemType:   "new_api",
		Origin:       origin,
		APIBaseURL:   apiBaseURL,
		Items:        items,
		CapturedAt:   c.now().UTC(),
	}, nil
}

func parseNewAPIEntitlementTransactions(data map[string]any) []ports.ProviderEntitlementTransaction {
	values, ok := data["items"].([]any)
	if !ok {
		return nil
	}
	items := make([]ports.ProviderEntitlementTransaction, 0, len(values))
	for _, value := range values {
		raw, ok := value.(map[string]any)
		if !ok {
			continue
		}
		item, ok := parseNewAPIEntitlementTransaction(raw)
		if ok {
			items = append(items, item)
		}
	}
	return items
}

func parseNewAPIEntitlementTransaction(raw map[string]any) (ports.ProviderEntitlementTransaction, bool) {
	code := stringFromAny(raw["key"])
	fingerprint := newAPICodeFingerprint(code)
	externalID := firstNonEmpty(newAPIIDString(raw["id"]), fingerprint)
	if externalID == "" {
		return ports.ProviderEntitlementTransaction{}, false
	}
	createdAt := unixTimePtr(raw["created_time"])
	usedAt := unixTimePtr(raw["redeemed_time"])
	rawQuota := float64FromAny(raw["quota"])
	item := ports.ProviderEntitlementTransaction{
		ExternalID:      externalID,
		CodeFingerprint: fingerprint,
		CodeLast4:       newAPILastN(code, 4),
		SourceFamily:    newAPIEntitlementSourceFamily(code),
		Type:            "balance",
		Status:          normalizeNewAPIRedemptionStatus(raw["status"]),
		Currency:        "USD",
		ValueCents:      newAPIQuotaToUSDCents(rawQuota),
		RawValue:        rawQuota,
		UsedAt:          usedAt,
		RawPayload:      sanitizeNewAPIPayload(raw),
	}
	if createdAt != nil {
		item.CreatedAtExternal = createdAt
	}
	return item, true
}

func normalizeNewAPIRedemptionStatus(value any) string {
	switch int64FromAny(value) {
	case 1:
		return "enabled"
	case 2:
		return "disabled"
	case 3:
		return "used"
	}
	switch strings.ToLower(strings.TrimSpace(stringFromAny(value))) {
	case "enabled", "available":
		return "enabled"
	case "disabled":
		return "disabled"
	case "used", "redeemed":
		return "used"
	default:
		return "unknown"
	}
}

func newAPICodeFingerprint(code string) string {
	code = strings.TrimSpace(code)
	if code == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(code))
	return fmt.Sprintf("%x", sum[:])
}

func newAPIEntitlementSourceFamily(code string) string {
	normalized := strings.ToUpper(strings.TrimSpace(code))
	if strings.HasPrefix(normalized, "PAY") {
		return "payment_auto_redeem"
	}
	return "manual_redeem"
}

func newAPILastN(value string, n int) string {
	value = strings.TrimSpace(value)
	if value == "" || n <= 0 {
		return ""
	}
	if len(value) <= n {
		return value
	}
	return value[len(value)-n:]
}
