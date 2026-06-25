package mailverification

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const gmailAPIBase = "https://gmail.googleapis.com/gmail/v1"

type GmailProvider struct {
	client      *http.Client
	config      OAuthConfigProvider
	apiBase     string
	profileURL  string
	now         func() time.Time
	retryBase   time.Duration
	maxAPIRetry int
}

func NewGmailProvider(client *http.Client, config OAuthConfigProvider) *GmailProvider {
	if client == nil {
		client = http.DefaultClient
	}
	return &GmailProvider{
		client:      client,
		config:      config,
		apiBase:     gmailAPIBase,
		profileURL:  gmailAPIBase + "/users/me/profile",
		now:         time.Now,
		retryBase:   300 * time.Millisecond,
		maxAPIRetry: 2,
	}
}

func (p *GmailProvider) Name() adminplusdomain.MailVerificationProvider {
	return ProviderGmail
}

func (p *GmailProvider) AuthorizeURL(ctx context.Context, in OAuthAuthorizeInput) (*OAuthAuthorizeResult, error) {
	if p == nil || p.config == nil {
		return nil, infraerrors.InternalServer("MAIL_PROVIDER_NOT_CONFIGURED", "gmail provider is not configured")
	}
	cfg, err := p.config.GoogleOAuthClient(ctx)
	if err != nil {
		return nil, err
	}
	redirectURI := strings.TrimSpace(in.RedirectURI)
	if redirectURI == "" {
		return nil, infraerrors.BadRequest("MAIL_OAUTH_REDIRECT_URI_REQUIRED", "mail oauth redirect uri is required")
	}
	authorizeURL := firstNonEmpty(cfg.AuthorizeURL, defaultGoogleAuthorizeURL)
	values := url.Values{}
	values.Set("client_id", cfg.ClientID)
	values.Set("redirect_uri", redirectURI)
	values.Set("response_type", "code")
	values.Set("scope", GmailReadonlyScope)
	values.Set("access_type", "offline")
	values.Set("prompt", "consent")
	if strings.TrimSpace(in.State) != "" {
		values.Set("state", strings.TrimSpace(in.State))
	}
	if strings.TrimSpace(in.LoginHint) != "" {
		values.Set("login_hint", strings.TrimSpace(in.LoginHint))
	}
	if strings.Contains(authorizeURL, "?") {
		authorizeURL += "&" + values.Encode()
	} else {
		authorizeURL += "?" + values.Encode()
	}
	return &OAuthAuthorizeResult{
		Provider:     ProviderGmail,
		AuthorizeURL: authorizeURL,
		Scope:        GmailReadonlyScope,
	}, nil
}

func (p *GmailProvider) ExchangeCode(ctx context.Context, in ExchangeCodeInput) (*SaveCredentialInput, error) {
	if p == nil || p.config == nil {
		return nil, infraerrors.InternalServer("MAIL_PROVIDER_NOT_CONFIGURED", "gmail provider is not configured")
	}
	code := strings.TrimSpace(in.Code)
	if code == "" {
		return nil, infraerrors.BadRequest("MAIL_OAUTH_CODE_REQUIRED", "mail oauth code is required")
	}
	redirectURI := strings.TrimSpace(in.RedirectURI)
	if redirectURI == "" {
		return nil, infraerrors.BadRequest("MAIL_OAUTH_REDIRECT_URI_REQUIRED", "mail oauth redirect uri is required")
	}
	cfg, err := p.config.GoogleOAuthClient(ctx)
	if err != nil {
		return nil, err
	}
	token, err := p.tokenRequest(ctx, cfg, url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {code},
		"redirect_uri": {redirectURI},
	}, true)
	if err != nil {
		return nil, err
	}
	profile, err := p.getProfile(ctx, token.AccessToken)
	if err != nil {
		return nil, err
	}
	expiresAt := p.expiresAt(token.ExpiresIn)
	return &SaveCredentialInput{
		Provider:     ProviderGmail,
		Name:         strings.TrimSpace(in.Name),
		Email:        profile.EmailAddress,
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		Scopes:       normalizeScopes(nil, token.Scope),
		TokenType:    firstNonEmpty(token.TokenType, "Bearer"),
		ExpiresAt:    expiresAt,
		Metadata: map[string]string{
			"history_id": profile.HistoryID,
		},
	}, nil
}

func (p *GmailProvider) CheckCredential(ctx context.Context, credential *Credential) (*TokenUpdate, error) {
	if credential == nil {
		return nil, infraerrors.BadRequest("MAIL_CREDENTIAL_INVALID", "mail credential is required")
	}
	tokenUpdate, accessToken, err := p.ensureAccessToken(ctx, credential)
	if err != nil {
		return nil, err
	}
	if _, err := p.getProfile(ctx, accessToken); err != nil {
		var apiErr *gmailAPIError
		if errors.As(err, &apiErr) && apiErr.statusCode == http.StatusUnauthorized {
			update, refreshErr := p.refreshAccessToken(ctx, credential)
			if refreshErr != nil {
				return nil, refreshErr
			}
			if _, retryErr := p.getProfile(ctx, update.AccessToken); retryErr != nil {
				return nil, retryErr
			}
			return update, nil
		}
		return tokenUpdate, err
	}
	return tokenUpdate, nil
}

func (p *GmailProvider) SearchMessages(ctx context.Context, credential *Credential, filter ProviderSearchFilter) (*ProviderSearchResult, error) {
	if credential == nil {
		return nil, infraerrors.BadRequest("MAIL_CREDENTIAL_INVALID", "mail credential is required")
	}
	tokenUpdate, accessToken, err := p.ensureAccessToken(ctx, credential)
	if err != nil {
		return nil, err
	}
	query := buildGmailQuery(filter)
	maxResults := filter.MaxResults
	if maxResults <= 0 || maxResults > 20 {
		maxResults = 10
	}
	params := url.Values{}
	params.Set("q", query)
	params.Set("maxResults", strconv.Itoa(maxResults))
	params.Set("includeSpamTrash", strconv.FormatBool(filter.IncludeSpam))
	listURL := p.apiBase + "/users/me/messages?" + params.Encode()

	list, err := p.listMessages(ctx, accessToken, listURL)
	if err != nil {
		var apiErr *gmailAPIError
		if errors.As(err, &apiErr) && apiErr.statusCode == http.StatusUnauthorized {
			tokenUpdate, err = p.refreshAccessToken(ctx, credential)
			if err != nil {
				return nil, err
			}
			accessToken = tokenUpdate.AccessToken
			list, err = p.listMessages(ctx, accessToken, listURL)
		}
		if err != nil {
			return nil, err
		}
	}

	messages := make([]MailMessage, 0, len(list.Messages))
	for _, item := range list.Messages {
		if strings.TrimSpace(item.ID) == "" {
			continue
		}
		message, err := p.getMessage(ctx, accessToken, item.ID)
		if err != nil {
			var apiErr *gmailAPIError
			if errors.As(err, &apiErr) && apiErr.statusCode == http.StatusNotFound {
				continue
			}
			if errors.As(err, &apiErr) && apiErr.statusCode == http.StatusUnauthorized {
				tokenUpdate, err = p.refreshAccessToken(ctx, credential)
				if err != nil {
					return nil, err
				}
				accessToken = tokenUpdate.AccessToken
				message, err = p.getMessage(ctx, accessToken, item.ID)
				if err != nil {
					return nil, err
				}
			} else {
				return nil, err
			}
		}
		messages = append(messages, formatGmailMessage(message))
	}

	return &ProviderSearchResult{Messages: messages, TokenUpdate: tokenUpdate}, nil
}

func (p *GmailProvider) ensureAccessToken(ctx context.Context, credential *Credential) (*TokenUpdate, string, error) {
	if !hasGmailReadScope(credential.Scopes) {
		return nil, "", infraerrors.Forbidden("MAIL_GMAIL_SCOPE_REQUIRED", "gmail readonly scope is required")
	}
	if strings.TrimSpace(credential.AccessToken) == "" {
		return nil, "", infraerrors.Unauthorized("MAIL_ACCESS_TOKEN_MISSING", "mail access token is missing")
	}
	if credential.ExpiresAt != nil && p.now().UTC().Add(time.Minute).After(credential.ExpiresAt.UTC()) {
		update, err := p.refreshAccessToken(ctx, credential)
		if err != nil {
			return nil, "", err
		}
		return update, update.AccessToken, nil
	}
	return nil, credential.AccessToken, nil
}

func (p *GmailProvider) refreshAccessToken(ctx context.Context, credential *Credential) (*TokenUpdate, error) {
	if p == nil || p.config == nil {
		return nil, infraerrors.InternalServer("MAIL_PROVIDER_NOT_CONFIGURED", "gmail provider is not configured")
	}
	refreshToken := strings.TrimSpace(credential.RefreshToken)
	if refreshToken == "" {
		return nil, infraerrors.Unauthorized("MAIL_REFRESH_TOKEN_MISSING", "mail refresh token is missing")
	}
	cfg, err := p.config.GoogleOAuthClient(ctx)
	if err != nil {
		return nil, err
	}
	token, err := p.tokenRequest(ctx, cfg, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	}, false)
	if err != nil {
		return nil, err
	}
	scopes := normalizeScopes(credential.Scopes, token.Scope)
	if !hasGmailReadScope(scopes) {
		return nil, infraerrors.Forbidden("MAIL_GMAIL_SCOPE_REQUIRED", "gmail readonly scope is required")
	}
	return &TokenUpdate{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		Scopes:       scopes,
		TokenType:    firstNonEmpty(token.TokenType, credential.TokenType, "Bearer"),
		ExpiresAt:    p.expiresAt(token.ExpiresIn),
	}, nil
}

func (p *GmailProvider) listMessages(ctx context.Context, accessToken, listURL string) (*gmailListResponse, error) {
	var list gmailListResponse
	if err := p.gmailRequest(ctx, http.MethodGet, listURL, accessToken, nil, &list); err != nil {
		return nil, err
	}
	return &list, nil
}

func (p *GmailProvider) getMessage(ctx context.Context, accessToken, id string) (*gmailMessage, error) {
	u := p.apiBase + "/users/me/messages/" + url.PathEscape(id) + "?" + url.Values{"format": {"full"}}.Encode()
	var message gmailMessage
	if err := p.gmailRequest(ctx, http.MethodGet, u, accessToken, nil, &message); err != nil {
		var apiErr *gmailAPIError
		if errors.As(err, &apiErr) && apiErr.statusCode == http.StatusNotFound {
			return nil, err
		}
		return nil, err
	}
	return &message, nil
}

func (p *GmailProvider) getProfile(ctx context.Context, accessToken string) (*gmailProfile, error) {
	var profile gmailProfile
	if err := p.gmailRequest(ctx, http.MethodGet, p.profileURL, accessToken, nil, &profile); err != nil {
		return nil, err
	}
	if strings.TrimSpace(profile.EmailAddress) == "" {
		return nil, infraerrors.InternalServer("MAIL_GMAIL_PROFILE_INVALID", "gmail profile email address is missing")
	}
	return &profile, nil
}

func (p *GmailProvider) gmailRequest(ctx context.Context, method, rawURL, accessToken string, payload any, out any) error {
	var lastErr error
	var retryAfter time.Duration
	for attempt := 0; attempt <= p.maxAPIRetry; attempt++ {
		if attempt > 0 {
			if err := sleepContext(ctx, p.retryDelay(attempt, retryAfter)); err != nil {
				return err
			}
			retryAfter = 0
		}
		err := p.doJSONRequest(ctx, method, rawURL, accessToken, payload, out)
		if err == nil {
			return nil
		}
		lastErr = err
		var apiErr *gmailAPIError
		if !errors.As(err, &apiErr) || !apiErr.retryable() || attempt == p.maxAPIRetry {
			return err
		}
		retryAfter = apiErr.retryAfter
	}
	return lastErr
}

func (p *GmailProvider) doJSONRequest(ctx context.Context, method, rawURL, accessToken string, payload any, out any) error {
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(accessToken))
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return parseGmailAPIError(resp.StatusCode, resp.Header, raw)
	}
	if out == nil || len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("decode gmail response: %w", err)
	}
	return nil
}

func (p *GmailProvider) tokenRequest(ctx context.Context, cfg OAuthClientConfig, values url.Values, requireGmailReadScope bool) (*oauthTokenResponse, error) {
	tokenURL := firstNonEmpty(cfg.TokenURL, defaultGoogleTokenURL)
	values.Set("client_id", cfg.ClientID)
	values.Set("client_secret", cfg.ClientSecret)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, parseOAuthError(resp.StatusCode, raw)
	}
	var token oauthTokenResponse
	if err := json.Unmarshal(raw, &token); err != nil {
		return nil, err
	}
	if strings.TrimSpace(token.AccessToken) == "" {
		return nil, infraerrors.InternalServer("MAIL_OAUTH_TOKEN_INVALID", "mail oauth token response is invalid")
	}
	if requireGmailReadScope && !hasGmailReadScope(normalizeScopes(nil, token.Scope)) {
		return nil, infraerrors.Forbidden("MAIL_GMAIL_SCOPE_REQUIRED", "gmail readonly scope is required")
	}
	return &token, nil
}

func (p *GmailProvider) expiresAt(expiresIn int) *time.Time {
	if expiresIn <= 0 {
		return nil
	}
	t := p.now().UTC().Add(time.Duration(expiresIn) * time.Second)
	return &t
}

func (p *GmailProvider) retryDelay(attempt int, retryAfter time.Duration) time.Duration {
	if retryAfter > 0 {
		if retryAfter > 10*time.Second {
			return 10 * time.Second
		}
		return retryAfter
	}
	delay := p.retryBase * time.Duration(math.Pow(2, float64(attempt-1)))
	if delay <= 0 {
		return p.retryBase
	}
	if delay > 3*time.Second {
		return 3 * time.Second
	}
	return delay
}

type oauthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

type gmailProfile struct {
	EmailAddress string `json:"emailAddress"`
	HistoryID    string `json:"historyId"`
}

type gmailListResponse struct {
	Messages           []gmailListMessage `json:"messages"`
	NextPageToken      string             `json:"nextPageToken"`
	ResultSizeEstimate int                `json:"resultSizeEstimate"`
}

type gmailListMessage struct {
	ID       string `json:"id"`
	ThreadID string `json:"threadId"`
}

type gmailMessage struct {
	ID           string       `json:"id"`
	ThreadID     string       `json:"threadId"`
	Snippet      string       `json:"snippet"`
	InternalDate string       `json:"internalDate"`
	Payload      gmailPayload `json:"payload"`
}

type gmailPayload struct {
	PartID   string         `json:"partId"`
	MimeType string         `json:"mimeType"`
	Filename string         `json:"filename"`
	Headers  []gmailHeader  `json:"headers"`
	Body     gmailBody      `json:"body"`
	Parts    []gmailPayload `json:"parts"`
}

type gmailHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type gmailBody struct {
	AttachmentID string `json:"attachmentId"`
	Size         int    `json:"size"`
	Data         string `json:"data"`
}

func formatGmailMessage(message *gmailMessage) MailMessage {
	if message == nil {
		return MailMessage{}
	}
	receivedAt := time.Time{}
	if millis, err := strconv.ParseInt(message.InternalDate, 10, 64); err == nil && millis > 0 {
		receivedAt = time.UnixMilli(millis).UTC()
	}
	text := extractTextFromGmailPayload(message.Payload)
	return MailMessage{
		ID:           message.ID,
		ThreadID:     message.ThreadID,
		Subject:      gmailHeaderValue(message.Payload.Headers, "subject"),
		From:         gmailHeaderValue(message.Payload.Headers, "from"),
		Snippet:      NormalizeMailBody(message.Snippet),
		Text:         NormalizeMailBody(text),
		InternalDate: receivedAt,
	}
}

func extractTextFromGmailPayload(payload gmailPayload) string {
	var plainParts []string
	var htmlParts []string
	var walk func(gmailPayload)
	walk = func(part gmailPayload) {
		if strings.TrimSpace(part.Body.Data) != "" && strings.TrimSpace(part.Body.AttachmentID) == "" {
			decoded := decodeGmailBody(part.Body.Data)
			switch strings.ToLower(strings.TrimSpace(part.MimeType)) {
			case "text/plain":
				plainParts = append(plainParts, decoded)
			case "text/html":
				htmlParts = append(htmlParts, stripHTML(decoded))
			}
		}
		for _, child := range part.Parts {
			walk(child)
		}
	}
	walk(payload)
	if len(plainParts) > 0 {
		return strings.Join(plainParts, "\n")
	}
	return strings.Join(htmlParts, "\n")
}

func decodeGmailBody(data string) string {
	data = strings.TrimSpace(data)
	if data == "" {
		return ""
	}
	if decoded, err := base64.RawURLEncoding.DecodeString(data); err == nil {
		return string(decoded)
	}
	if decoded, err := base64.URLEncoding.DecodeString(data); err == nil {
		return string(decoded)
	}
	return ""
}

func gmailHeaderValue(headers []gmailHeader, name string) string {
	for _, header := range headers {
		if strings.EqualFold(header.Name, name) {
			return strings.TrimSpace(header.Value)
		}
	}
	return ""
}

func buildGmailQuery(filter ProviderSearchFilter) string {
	if strings.TrimSpace(filter.Query) != "" {
		return strings.TrimSpace(filter.Query)
	}
	keywords := append([]string{}, filter.Keywords...)
	if len(keywords) == 0 {
		keywords = []string{"验证码", "verification code", "security code", "login code", "code"}
	}
	parts := []string{"newer_than:1d", "in:anywhere"}
	keywordQuery := make([]string, 0, len(keywords))
	for _, keyword := range keywords {
		keyword = strings.TrimSpace(keyword)
		if keyword == "" {
			continue
		}
		if strings.ContainsAny(keyword, " \t") {
			keyword = strconv.Quote(keyword)
		}
		keywordQuery = append(keywordQuery, keyword)
	}
	if len(keywordQuery) > 0 {
		parts = append(parts, "("+strings.Join(keywordQuery, " OR ")+")")
	}
	if strings.TrimSpace(filter.From) != "" {
		parts = append(parts, "from:"+strings.TrimSpace(filter.From))
	}
	if strings.TrimSpace(filter.To) != "" {
		parts = append(parts, "to:"+strings.TrimSpace(filter.To))
	}
	return strings.Join(parts, " ")
}

func hasGmailReadScope(scopes []string) bool {
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if scope == GmailReadonlyScope ||
			scope == "https://www.googleapis.com/auth/gmail.modify" ||
			scope == "https://mail.google.com/" {
			return true
		}
	}
	return false
}

type gmailAPIError struct {
	statusCode int
	reason     string
	message    string
	retryAfter time.Duration
}

func (e *gmailAPIError) Error() string {
	if e == nil {
		return ""
	}
	return e.message
}

func (e *gmailAPIError) retryable() bool {
	if e == nil {
		return false
	}
	return e.statusCode == http.StatusTooManyRequests || e.statusCode >= 500
}

func parseGmailAPIError(statusCode int, header http.Header, raw []byte) error {
	var payload struct {
		Error struct {
			Message string            `json:"message"`
			Status  string            `json:"status"`
			Details []json.RawMessage `json:"details"`
			Errors  []struct {
				Reason string `json:"reason"`
			} `json:"errors"`
		} `json:"error"`
	}
	_ = json.Unmarshal(raw, &payload)
	detail := firstGmailErrorDetail(payload.Error.Details)
	reason := firstNonEmpty(detail.reason, firstGmailErrorReason(payload.Error.Errors), payload.Error.Status, http.StatusText(statusCode))
	message := firstNonEmpty(payload.Error.Message, "gmail api request failed")
	apiErr := &gmailAPIError{
		statusCode: statusCode,
		reason:     reason,
		message:    message,
		retryAfter: parseRetryAfter(header.Get("Retry-After")),
	}
	metadata := gmailErrorMetadata(apiErr, payload.Error.Status, detail)
	switch statusCode {
	case http.StatusUnauthorized:
		return infraerrors.Unauthorized("MAIL_GMAIL_UNAUTHENTICATED", "gmail credential is unauthenticated").WithCause(apiErr).WithMetadata(metadata)
	case http.StatusForbidden:
		return infraerrors.Forbidden("MAIL_GMAIL_PERMISSION_DENIED", "gmail permission denied").WithCause(apiErr).WithMetadata(metadata)
	case http.StatusNotFound:
		return infraerrors.NotFound("MAIL_GMAIL_MESSAGE_NOT_FOUND", "gmail message not found").WithCause(apiErr).WithMetadata(metadata)
	case http.StatusTooManyRequests:
		return infraerrors.TooManyRequests("MAIL_GMAIL_RATE_LIMITED", "gmail api rate limited").WithCause(apiErr).WithMetadata(metadata)
	default:
		if statusCode >= 500 {
			return infraerrors.ServiceUnavailable("MAIL_GMAIL_UNAVAILABLE", "gmail api is unavailable").WithCause(apiErr).WithMetadata(metadata)
		}
		return infraerrors.BadRequest("MAIL_GMAIL_REQUEST_FAILED", "gmail api request failed").WithCause(apiErr).WithMetadata(metadata)
	}
}

func parseOAuthError(statusCode int, raw []byte) error {
	var payload struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	_ = json.Unmarshal(raw, &payload)
	switch statusCode {
	case http.StatusUnauthorized, http.StatusBadRequest:
		return infraerrors.Unauthorized("MAIL_OAUTH_TOKEN_REFRESH_FAILED", "mail oauth token refresh failed")
	default:
		if statusCode >= 500 {
			return infraerrors.ServiceUnavailable("MAIL_OAUTH_PROVIDER_UNAVAILABLE", "mail oauth provider is unavailable")
		}
		return infraerrors.BadRequest("MAIL_OAUTH_REQUEST_FAILED", "mail oauth request failed")
	}
}

func parseRetryAfter(value string) time.Duration {
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	if at, err := http.ParseTime(value); err == nil {
		return time.Until(at)
	}
	return 0
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func firstGmailErrorReason(errors []struct {
	Reason string `json:"reason"`
}) string {
	if len(errors) == 0 {
		return ""
	}
	return errors[0].Reason
}

type gmailErrorDetail struct {
	reason        string
	domain        string
	activationURL string
	helpURL       string
}

func firstGmailErrorDetail(details []json.RawMessage) gmailErrorDetail {
	var result gmailErrorDetail
	for _, raw := range details {
		var info struct {
			Reason   string            `json:"reason"`
			Domain   string            `json:"domain"`
			Metadata map[string]string `json:"metadata"`
		}
		if err := json.Unmarshal(raw, &info); err == nil {
			if result.reason == "" {
				result.reason = strings.TrimSpace(info.Reason)
			}
			if result.domain == "" {
				result.domain = strings.TrimSpace(info.Domain)
			}
			if result.activationURL == "" && info.Metadata != nil {
				result.activationURL = strings.TrimSpace(info.Metadata["activationUrl"])
			}
		}

		var help struct {
			Links []struct {
				URL string `json:"url"`
			} `json:"links"`
		}
		if err := json.Unmarshal(raw, &help); err == nil {
			for _, link := range help.Links {
				if result.helpURL = strings.TrimSpace(link.URL); result.helpURL != "" {
					break
				}
			}
		}
	}
	return result
}

func gmailErrorMetadata(apiErr *gmailAPIError, status string, detail gmailErrorDetail) map[string]string {
	if apiErr == nil {
		return nil
	}
	metadata := map[string]string{
		"gmail_status_code": strconv.Itoa(apiErr.statusCode),
	}
	if apiErr.reason != "" {
		metadata["gmail_reason"] = trimLimit(apiErr.reason, 160)
	}
	if apiErr.message != "" {
		metadata["gmail_message"] = trimLimit(apiErr.message, 500)
	}
	if status = strings.TrimSpace(status); status != "" {
		metadata["gmail_status"] = trimLimit(status, 160)
	}
	if detail.domain != "" {
		metadata["gmail_domain"] = trimLimit(detail.domain, 160)
	}
	if detail.activationURL != "" {
		metadata["gmail_activation_url"] = trimLimit(detail.activationURL, 500)
	}
	if detail.helpURL != "" {
		metadata["gmail_help_url"] = trimLimit(detail.helpURL, 500)
	}
	return metadata
}
