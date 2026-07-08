package provider

import (
	"context"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/adminplus/ports"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	newAPIPageSize        = 100
	newAPIHistoryMaxPages = 10000
	newAPIPageDelay       = 150 * time.Millisecond
)

func (c *Client) ReadFundingTransactions(ctx context.Context, in ports.SessionProbeInput, request ports.ReadFundingTransactionsInput) (*ports.ReadFundingTransactionsResult, error) {
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
	baseEndpoint, err := buildEndpointURL(apiBaseURL, "/api/user/topup")
	if err != nil {
		return nil, err
	}
	items := make([]ports.ProviderFundingTransaction, 0)
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
			return nil, infraerrors.New(http.StatusBadGateway, "SUPPLIER_FUNDING_RESPONSE_INVALID", "new api topup response is invalid").WithCause(err)
		}
		if !envelope.Success {
			err := classifySessionBusinessFailure(envelope.Message)
			if isNewAPISessionPermissionError(err) {
				return nil, newAPISessionPermissionRequired(role, roleKnown, err)
			}
			return nil, err
		}
		pageItems := parseNewAPITopupTransactions(envelope.Data)
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
			return nil, newAPIPageLimitExceeded("funding_transactions", newAPIHistoryMaxPages, newAPIPageSize)
		}
		if err := waitForProviderPage(ctx, newAPIPageDelay); err != nil {
			return nil, err
		}
	}
	return &ports.ReadFundingTransactionsResult{
		SupplierID:   in.SupplierID,
		ProviderType: "new_api",
		SystemType:   "new_api",
		Origin:       origin,
		APIBaseURL:   apiBaseURL,
		Items:        items,
		CapturedAt:   c.now().UTC(),
	}, nil
}

func newAPIPageLimitExceeded(resource string, maxPages int, pageSize int) error {
	return infraerrors.New(
		http.StatusConflict,
		"SUPPLIER_HISTORY_PAGE_LIMIT_EXCEEDED",
		"supplier history pagination exceeded the safety limit; narrow the time range and retry",
	).WithMetadata(map[string]string{
		"resource":  resource,
		"max_pages": strconv.Itoa(maxPages),
		"page_size": strconv.Itoa(pageSize),
	})
}

func parseNewAPITopupTransactions(data map[string]any) []ports.ProviderFundingTransaction {
	values, ok := data["items"].([]any)
	if !ok {
		return nil
	}
	items := make([]ports.ProviderFundingTransaction, 0, len(values))
	for _, value := range values {
		raw, ok := value.(map[string]any)
		if !ok {
			continue
		}
		item, ok := parseNewAPITopupTransaction(raw)
		if ok {
			items = append(items, item)
		}
	}
	return items
}

func parseNewAPITopupTransaction(raw map[string]any) (ports.ProviderFundingTransaction, bool) {
	tradeNo := firstNonEmpty(stringFromAny(raw["trade_no"]), stringFromAny(raw["tradeNo"]))
	externalID := firstNonEmpty(tradeNo, newAPIIDString(raw["id"]))
	if externalID == "" {
		return ports.ProviderFundingTransaction{}, false
	}
	amountCents := newAPITopupAmountCents(raw)
	cashAmountCents := newAPITopupCashAmountCents(raw)
	if cashAmountCents == 0 {
		cashAmountCents = amountCents
	}
	createdAt := unixTimePtr(raw["create_time"])
	completedAt := unixTimePtr(raw["complete_time"])
	status := normalizeNewAPITopupStatus(stringFromAny(raw["status"]))
	item := ports.ProviderFundingTransaction{
		ExternalID:      externalID,
		OutTradeNo:      tradeNo,
		PaymentType:     firstNonEmpty(stringFromAny(raw["payment_method"]), stringFromAny(raw["paymentMethod"])),
		OrderType:       firstNonEmpty(stringFromAny(raw["payment_provider"]), stringFromAny(raw["paymentProvider"]), "topup"),
		Status:          status,
		Currency:        "USD",
		AmountCents:     amountCents,
		CashAmountCents: cashAmountCents,
		RawPayload:      sanitizeNewAPIPayload(raw),
	}
	if completedAt != nil {
		item.PaidAt = completedAt
		item.CompletedAt = completedAt
	}
	if createdAt != nil {
		item.CreatedAtExternal = createdAt
	}
	return item, true
}

func newAPITopupAmountCents(raw map[string]any) int64 {
	if amount := float64FromAny(raw["amount"]); amount > 0 {
		return int64(math.Round(amount * 100))
	}
	if quota := float64FromAny(raw["quota"]); quota > 0 {
		return newAPIQuotaToUSDCents(quota)
	}
	return 0
}

func newAPITopupCashAmountCents(raw map[string]any) int64 {
	for _, key := range []string{
		"money",
		"pay_money",
		"payMoney",
		"payment_amount",
		"paymentAmount",
		"cash_amount",
		"cashAmount",
		"actual_payment",
		"actualPayment",
	} {
		if amount := float64FromAny(raw[key]); amount > 0 {
			return int64(math.Round(amount * 100))
		}
	}
	return 0
}

func normalizeNewAPITopupStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "success", "succeeded", "paid", "completed":
		return "COMPLETED"
	case "pending":
		return "PENDING"
	case "failed", "fail":
		return "FAILED"
	case "expired":
		return "EXPIRED"
	default:
		return strings.ToUpper(strings.TrimSpace(value))
	}
}

func unixTimePtr(value any) *time.Time {
	seconds := int64FromAny(value)
	if seconds <= 0 {
		return nil
	}
	t := time.Unix(seconds, 0).UTC()
	return &t
}

func newAPIIDString(value any) string {
	n := int64FromAny(value)
	if n <= 0 {
		return ""
	}
	return strconv.FormatInt(n, 10)
}

func appendNewAPIQueryValues(endpoint string, pairs map[string]string) string {
	if len(pairs) == 0 {
		return endpoint
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return endpoint
	}
	values := u.Query()
	for key, value := range pairs {
		values.Set(key, value)
	}
	u.RawQuery = values.Encode()
	return u.String()
}

func sanitizeNewAPIPayload(raw map[string]any) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(raw))
	for key, value := range raw {
		if isNewAPISensitivePayloadKey(key) {
			continue
		}
		out[key] = sanitizeNewAPIPayloadValue(value)
	}
	return out
}

func sanitizeNewAPIPayloadValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		return sanitizeNewAPIPayload(v)
	case []any:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, sanitizeNewAPIPayloadValue(item))
		}
		return out
	default:
		return value
	}
}

func isNewAPISensitivePayloadKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	return strings.Contains(key, "key") ||
		strings.Contains(key, "token") ||
		strings.Contains(key, "secret") ||
		strings.Contains(key, "password") ||
		strings.Contains(key, "cookie")
}
