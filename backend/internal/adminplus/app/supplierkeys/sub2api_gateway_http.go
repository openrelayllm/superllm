package supplierkeys

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

const (
	sub2APIAdminBaseURLEnv = "ADMIN_PLUS_SUB2API_ADMIN_BASE_URL"
	sub2APIAdminAPIKeyEnv  = "ADMIN_PLUS_SUB2API_ADMIN_API_KEY"
)

type Sub2APIHTTPGateway struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func NewSub2APIHTTPGateway(baseURL string, apiKey string, client *http.Client) (*Sub2APIHTTPGateway, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	apiKey = strings.TrimSpace(apiKey)
	if baseURL == "" || apiKey == "" {
		return nil, infraerrors.New(http.StatusInternalServerError, "SUB2API_GATEWAY_CONFIG_REQUIRED", "sub2api gateway base url and admin api key are required")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, infraerrors.New(http.StatusInternalServerError, "SUB2API_GATEWAY_BASE_URL_INVALID", "sub2api gateway base url is invalid")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, infraerrors.New(http.StatusInternalServerError, "SUB2API_GATEWAY_BASE_URL_INVALID", "sub2api gateway base url must use http or https")
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &Sub2APIHTTPGateway{baseURL: baseURL, apiKey: apiKey, client: client}, nil
}

func NewSub2APIHTTPGatewayFromEnv(client *http.Client) (*Sub2APIHTTPGateway, error) {
	return NewSub2APIHTTPGateway(os.Getenv(sub2APIAdminBaseURLEnv), os.Getenv(sub2APIAdminAPIKeyEnv), client)
}

func ShouldUseSub2APIHTTPGatewayFromEnv() bool {
	return strings.TrimSpace(os.Getenv(sub2APIAdminBaseURLEnv)) != "" && strings.TrimSpace(os.Getenv(sub2APIAdminAPIKeyEnv)) != ""
}

func NewSub2APIHTTPGatewayFromConfig(cfg *config.Config, client *http.Client) (*Sub2APIHTTPGateway, error) {
	if cfg == nil {
		return NewSub2APIHTTPGatewayFromEnv(client)
	}
	return NewSub2APIHTTPGateway(cfg.AdminPlus.Sub2APIAdminBaseURL, cfg.AdminPlus.Sub2APIAdminAPIKey, client)
}

func ShouldUseSub2APIHTTPGatewayFromConfig(cfg *config.Config) bool {
	if cfg == nil {
		return ShouldUseSub2APIHTTPGatewayFromEnv()
	}
	return strings.TrimSpace(cfg.AdminPlus.Sub2APIAdminBaseURL) != "" && strings.TrimSpace(cfg.AdminPlus.Sub2APIAdminAPIKey) != ""
}

func (g *Sub2APIHTTPGateway) CreateAccount(ctx context.Context, input *service.CreateAccountInput) (*service.Account, error) {
	if input == nil {
		return nil, infraerrors.New(http.StatusBadRequest, "SUB2API_ACCOUNT_INPUT_REQUIRED", "sub2api account input is required")
	}
	payload := map[string]any{
		"name":                       input.Name,
		"notes":                      input.Notes,
		"platform":                   input.Platform,
		"type":                       input.Type,
		"credentials":                input.Credentials,
		"extra":                      input.Extra,
		"proxy_id":                   input.ProxyID,
		"concurrency":                input.Concurrency,
		"priority":                   input.Priority,
		"rate_multiplier":            input.RateMultiplier,
		"load_factor":                input.LoadFactor,
		"schedulable":                input.Schedulable,
		"group_ids":                  input.GroupIDs,
		"expires_at":                 input.ExpiresAt,
		"auto_pause_on_expired":      input.AutoPauseOnExpired,
		"confirm_mixed_channel_risk": input.SkipMixedChannelCheck,
	}
	var account sub2APIAccountDTO
	if err := g.doJSON(ctx, http.MethodPost, "/api/v1/admin/accounts", payload, &account); err != nil {
		return nil, err
	}
	return account.toService(), nil
}

func (g *Sub2APIHTTPGateway) GetAccount(ctx context.Context, id int64) (*service.Account, error) {
	if id <= 0 {
		return nil, infraerrors.New(http.StatusBadRequest, "SUB2API_ACCOUNT_ID_INVALID", "sub2api account id is invalid")
	}
	var account sub2APIAccountDTO
	if err := g.doJSON(ctx, http.MethodGet, "/api/v1/admin/accounts/"+strconv.FormatInt(id, 10), nil, &account); err != nil {
		return nil, err
	}
	return account.toService(), nil
}

func (g *Sub2APIHTTPGateway) UpdateAccount(ctx context.Context, id int64, input *service.UpdateAccountInput) (*service.Account, error) {
	if id <= 0 {
		return nil, infraerrors.New(http.StatusBadRequest, "SUB2API_ACCOUNT_ID_INVALID", "sub2api account id is invalid")
	}
	if input == nil {
		return g.GetAccount(ctx, id)
	}
	payload := map[string]any{
		"name":                       input.Name,
		"notes":                      input.Notes,
		"type":                       input.Type,
		"credentials":                input.Credentials,
		"extra":                      input.Extra,
		"proxy_id":                   input.ProxyID,
		"concurrency":                input.Concurrency,
		"priority":                   input.Priority,
		"rate_multiplier":            input.RateMultiplier,
		"load_factor":                input.LoadFactor,
		"status":                     input.Status,
		"group_ids":                  input.GroupIDs,
		"expires_at":                 input.ExpiresAt,
		"auto_pause_on_expired":      input.AutoPauseOnExpired,
		"confirm_mixed_channel_risk": input.SkipMixedChannelCheck,
	}
	var account sub2APIAccountDTO
	if err := g.doJSON(ctx, http.MethodPut, "/api/v1/admin/accounts/"+strconv.FormatInt(id, 10), payload, &account); err != nil {
		return nil, err
	}
	return account.toService(), nil
}

func (g *Sub2APIHTTPGateway) CreateGroup(ctx context.Context, input *service.CreateGroupInput) (*service.Group, error) {
	if input == nil {
		return nil, infraerrors.New(http.StatusBadRequest, "SUB2API_GROUP_INPUT_REQUIRED", "sub2api group input is required")
	}
	payload := map[string]any{
		"name":                                 input.Name,
		"description":                          input.Description,
		"platform":                             input.Platform,
		"rate_multiplier":                      input.RateMultiplier,
		"is_exclusive":                         input.IsExclusive,
		"subscription_type":                    input.SubscriptionType,
		"daily_limit_usd":                      input.DailyLimitUSD,
		"weekly_limit_usd":                     input.WeeklyLimitUSD,
		"monthly_limit_usd":                    input.MonthlyLimitUSD,
		"allow_image_generation":               input.AllowImageGeneration,
		"image_rate_independent":               input.ImageRateIndependent,
		"image_rate_multiplier":                input.ImageRateMultiplier,
		"image_price_1k":                       input.ImagePrice1K,
		"image_price_2k":                       input.ImagePrice2K,
		"image_price_4k":                       input.ImagePrice4K,
		"claude_code_only":                     input.ClaudeCodeOnly,
		"fallback_group_id":                    input.FallbackGroupID,
		"fallback_group_id_on_invalid_request": input.FallbackGroupIDOnInvalidRequest,
		"model_routing":                        input.ModelRouting,
		"model_routing_enabled":                input.ModelRoutingEnabled,
		"mcp_xml_inject":                       input.MCPXMLInject,
		"supported_model_scopes":               input.SupportedModelScopes,
		"allow_messages_dispatch":              input.AllowMessagesDispatch,
		"require_oauth_only":                   input.RequireOAuthOnly,
		"require_privacy_set":                  input.RequirePrivacySet,
		"default_mapped_model":                 input.DefaultMappedModel,
		"messages_dispatch_model_config":       input.MessagesDispatchModelConfig,
		"models_list_config":                   input.ModelsListConfig,
		"rpm_limit":                            input.RPMLimit,
		"copy_accounts_from_group_ids":         input.CopyAccountsFromGroupIDs,
	}
	var group sub2APIGroupDTO
	if err := g.doJSON(ctx, http.MethodPost, "/api/v1/admin/groups", payload, &group); err != nil {
		return nil, err
	}
	return group.toService(), nil
}

func (g *Sub2APIHTTPGateway) GetAllGroupsIncludingInactive(ctx context.Context) ([]service.Group, error) {
	var groups []sub2APIGroupDTO
	if err := g.doJSON(ctx, http.MethodGet, "/api/v1/admin/groups/all?include_inactive=true", nil, &groups); err != nil {
		return nil, err
	}
	out := make([]service.Group, 0, len(groups))
	for i := range groups {
		out = append(out, *groups[i].toService())
	}
	return out, nil
}

func (g *Sub2APIHTTPGateway) FindAccount(ctx context.Context, input Sub2APIAccountLookupInput) (*service.Account, error) {
	query := url.Values{}
	query.Set("page", "1")
	query.Set("page_size", "100")
	if strings.TrimSpace(input.LocalAccountPlatform) != "" {
		query.Set("platform", strings.TrimSpace(input.LocalAccountPlatform))
	}
	if strings.TrimSpace(input.LocalAccountName) != "" {
		query.Set("search", strings.TrimSpace(input.LocalAccountName))
	}
	path := "/api/v1/admin/accounts?" + query.Encode()
	var page sub2APIAccountPage
	if err := g.doJSON(ctx, http.MethodGet, path, nil, &page); err != nil {
		var accounts []sub2APIAccountDTO
		if secondErr := g.doJSON(ctx, http.MethodGet, path, nil, &accounts); secondErr != nil {
			return nil, err
		}
		page.Items = accounts
	}
	for i := range page.Items {
		account := page.Items[i].toService()
		if localAccountMatchesLookup(account, input) {
			return account, nil
		}
	}
	return nil, nil
}

func (g *Sub2APIHTTPGateway) doJSON(ctx context.Context, method string, path string, payload any, out any) error {
	if g == nil || g.client == nil {
		return infraerrors.New(http.StatusInternalServerError, "SUB2API_GATEWAY_NOT_CONFIGURED", "sub2api gateway is not configured")
	}
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, g.baseURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-api-key", g.apiKey)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := g.client.Do(req)
	if err != nil {
		return infraerrors.New(http.StatusBadGateway, "SUB2API_GATEWAY_REQUEST_FAILED", "failed to request sub2api admin api").WithCause(err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return sub2APIHTTPError(resp.StatusCode, data)
	}
	if out == nil {
		return nil
	}
	if err := decodeSub2APIResponse(data, out); err != nil {
		return infraerrors.New(http.StatusBadGateway, "SUB2API_GATEWAY_RESPONSE_INVALID", "sub2api admin api response is invalid").WithCause(err)
	}
	return nil
}

type sub2APIEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Reason  string          `json:"reason"`
	Data    json.RawMessage `json:"data"`
}

type sub2APIAccountPage struct {
	Items []sub2APIAccountDTO `json:"items"`
}

type sub2APIAccountDTO struct {
	ID                      int64          `json:"id"`
	Name                    string         `json:"name"`
	Notes                   *string        `json:"notes"`
	Platform                string         `json:"platform"`
	Type                    string         `json:"type"`
	Credentials             map[string]any `json:"credentials"`
	Extra                   map[string]any `json:"extra"`
	ProxyID                 *int64         `json:"proxy_id"`
	ProxyFallbackOriginID   *int64         `json:"proxy_fallback_origin_id"`
	ProxyFallbackOriginName *string        `json:"proxy_fallback_origin_name"`
	Concurrency             int            `json:"concurrency"`
	Priority                int            `json:"priority"`
	RateMultiplier          *float64       `json:"rate_multiplier"`
	LoadFactor              *int           `json:"load_factor"`
	Status                  string         `json:"status"`
	ErrorMessage            string         `json:"error_message"`
	GroupIDs                []int64        `json:"group_ids"`
	CreatedAt               time.Time      `json:"created_at"`
	UpdatedAt               time.Time      `json:"updated_at"`
	Schedulable             bool           `json:"schedulable"`
}

func (a sub2APIAccountDTO) toService() *service.Account {
	return &service.Account{
		ID:                      a.ID,
		Name:                    a.Name,
		Notes:                   a.Notes,
		Platform:                a.Platform,
		Type:                    a.Type,
		Credentials:             a.Credentials,
		Extra:                   a.Extra,
		ProxyID:                 a.ProxyID,
		ProxyFallbackOriginID:   a.ProxyFallbackOriginID,
		ProxyFallbackOriginName: a.ProxyFallbackOriginName,
		Concurrency:             a.Concurrency,
		Priority:                a.Priority,
		RateMultiplier:          a.RateMultiplier,
		LoadFactor:              a.LoadFactor,
		Status:                  a.Status,
		ErrorMessage:            a.ErrorMessage,
		GroupIDs:                append([]int64(nil), a.GroupIDs...),
		CreatedAt:               a.CreatedAt,
		UpdatedAt:               a.UpdatedAt,
		Schedulable:             a.Schedulable,
	}
}

type sub2APIGroupDTO struct {
	ID                              int64              `json:"id"`
	Name                            string             `json:"name"`
	Description                     string             `json:"description"`
	Platform                        string             `json:"platform"`
	RateMultiplier                  float64            `json:"rate_multiplier"`
	IsExclusive                     bool               `json:"is_exclusive"`
	Status                          string             `json:"status"`
	SubscriptionType                string             `json:"subscription_type"`
	DailyLimitUSD                   *float64           `json:"daily_limit_usd"`
	WeeklyLimitUSD                  *float64           `json:"weekly_limit_usd"`
	MonthlyLimitUSD                 *float64           `json:"monthly_limit_usd"`
	AllowImageGeneration            bool               `json:"allow_image_generation"`
	ImageRateIndependent            bool               `json:"image_rate_independent"`
	ImageRateMultiplier             float64            `json:"image_rate_multiplier"`
	ImagePrice1K                    *float64           `json:"image_price_1k"`
	ImagePrice2K                    *float64           `json:"image_price_2k"`
	ImagePrice4K                    *float64           `json:"image_price_4k"`
	ClaudeCodeOnly                  bool               `json:"claude_code_only"`
	FallbackGroupID                 *int64             `json:"fallback_group_id"`
	FallbackGroupIDOnInvalidRequest *int64             `json:"fallback_group_id_on_invalid_request"`
	AllowMessagesDispatch           bool               `json:"allow_messages_dispatch"`
	RequireOAuthOnly                bool               `json:"require_oauth_only"`
	RequirePrivacySet               bool               `json:"require_privacy_set"`
	RPMLimit                        int                `json:"rpm_limit"`
	SortOrder                       int                `json:"sort_order"`
	CreatedAt                       time.Time          `json:"created_at"`
	UpdatedAt                       time.Time          `json:"updated_at"`
	ModelRouting                    map[string][]int64 `json:"model_routing"`
	ModelRoutingEnabled             bool               `json:"model_routing_enabled"`
}

func (g sub2APIGroupDTO) toService() *service.Group {
	return &service.Group{
		ID:                              g.ID,
		Name:                            g.Name,
		Description:                     g.Description,
		Platform:                        g.Platform,
		RateMultiplier:                  g.RateMultiplier,
		IsExclusive:                     g.IsExclusive,
		Status:                          g.Status,
		Hydrated:                        true,
		SubscriptionType:                g.SubscriptionType,
		DailyLimitUSD:                   g.DailyLimitUSD,
		WeeklyLimitUSD:                  g.WeeklyLimitUSD,
		MonthlyLimitUSD:                 g.MonthlyLimitUSD,
		AllowImageGeneration:            g.AllowImageGeneration,
		ImageRateIndependent:            g.ImageRateIndependent,
		ImageRateMultiplier:             g.ImageRateMultiplier,
		ImagePrice1K:                    g.ImagePrice1K,
		ImagePrice2K:                    g.ImagePrice2K,
		ImagePrice4K:                    g.ImagePrice4K,
		ClaudeCodeOnly:                  g.ClaudeCodeOnly,
		FallbackGroupID:                 g.FallbackGroupID,
		FallbackGroupIDOnInvalidRequest: g.FallbackGroupIDOnInvalidRequest,
		AllowMessagesDispatch:           g.AllowMessagesDispatch,
		RequireOAuthOnly:                g.RequireOAuthOnly,
		RequirePrivacySet:               g.RequirePrivacySet,
		RPMLimit:                        g.RPMLimit,
		SortOrder:                       g.SortOrder,
		CreatedAt:                       g.CreatedAt,
		UpdatedAt:                       g.UpdatedAt,
		ModelRouting:                    g.ModelRouting,
		ModelRoutingEnabled:             g.ModelRoutingEnabled,
	}
}

func decodeSub2APIResponse(data []byte, out any) error {
	var envelope sub2APIEnvelope
	if err := json.Unmarshal(data, &envelope); err == nil && envelope.Data != nil {
		if envelope.Code != 0 {
			return fmt.Errorf("sub2api returned code %d: %s", envelope.Code, envelope.Message)
		}
		return json.Unmarshal(envelope.Data, out)
	}
	return json.Unmarshal(data, out)
}

func sub2APIHTTPError(statusCode int, data []byte) error {
	var envelope sub2APIEnvelope
	if err := json.Unmarshal(data, &envelope); err == nil {
		reason := strings.TrimSpace(envelope.Reason)
		if reason == "" {
			reason = "SUB2API_GATEWAY_BAD_STATUS"
		}
		message := strings.TrimSpace(envelope.Message)
		if message == "" {
			message = http.StatusText(statusCode)
		}
		return infraerrors.New(statusCode, reason, message)
	}
	message := strings.TrimSpace(string(data))
	if message == "" {
		message = http.StatusText(statusCode)
	}
	return infraerrors.New(statusCode, "SUB2API_GATEWAY_BAD_STATUS", message)
}

var _ Sub2APIGateway = (*Sub2APIHTTPGateway)(nil)
var _ Sub2APIAccountFinder = (*Sub2APIHTTPGateway)(nil)
