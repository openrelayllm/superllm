package mailverification

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/adminplus/app/bizlogs"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	defaultReadTimeout      = 90 * time.Second
	maxReadTimeout          = 120 * time.Second
	defaultPollInterval     = 5 * time.Second
	minPollInterval         = 2 * time.Second
	defaultRecentWindow     = 10 * time.Minute
	defaultSearchMaxResults = 10
)

type Service struct {
	repo       Repository
	providers  map[adminplusdomain.MailVerificationProvider]MailProvider
	classifier TemplateClassifier
	email      EmailSender
	claims     map[string]messageClaim
	claimsMu   sync.Mutex
	bizlog     *bizlogs.Recorder
	now        func() time.Time
}

type messageClaim struct {
	Key      string
	ExpireAt time.Time
}

func NewService(repo Repository, gmailProvider *GmailProvider, classifier TemplateClassifier, email EmailSender) *Service {
	providers := map[adminplusdomain.MailVerificationProvider]MailProvider{}
	if gmailProvider != nil {
		providers[gmailProvider.Name()] = gmailProvider
	}
	if classifier == nil {
		classifier = NewDefaultTemplateClassifier()
	}
	return &Service{
		repo:       repo,
		providers:  providers,
		classifier: classifier,
		email:      email,
		claims:     make(map[string]messageClaim),
		now:        time.Now,
	}
}

func (s *Service) WithDiagnostics(recorder *bizlogs.Recorder) *Service {
	if s != nil {
		s.bizlog = recorder
	}
	return s
}

func (s *Service) AuthorizeURL(ctx context.Context, in OAuthAuthorizeInput) (*OAuthAuthorizeResult, error) {
	provider, err := s.providerFor(in.Provider)
	if err != nil {
		return nil, err
	}
	in.Provider = provider.Name()
	return provider.AuthorizeURL(ctx, in)
}

func (s *Service) ExchangeCode(ctx context.Context, in ExchangeCodeInput) (*adminplusdomain.MailVerificationCredential, error) {
	provider, err := s.providerFor(in.Provider)
	if err != nil {
		return nil, err
	}
	in.Provider = provider.Name()
	saveInput, err := provider.ExchangeCode(ctx, in)
	if err != nil {
		return nil, err
	}
	credential, err := s.SaveCredential(ctx, *saveInput)
	if err != nil {
		return nil, err
	}
	return credential, nil
}

func (s *Service) OAuthSettings(ctx context.Context, provider adminplusdomain.MailVerificationProvider) (*OAuthSettings, error) {
	provider = normalizeProvider(provider)
	if provider == "" {
		provider = ProviderGmail
	}
	if provider != ProviderGmail {
		return nil, infraerrors.New(http.StatusNotImplemented, "MAIL_PROVIDER_NOT_SUPPORTED", "mail provider is not supported")
	}
	gmailProvider, ok := s.providers[ProviderGmail].(*GmailProvider)
	if !ok || gmailProvider == nil || gmailProvider.config == nil {
		return nil, infraerrors.InternalServer("MAIL_PROVIDER_NOT_CONFIGURED", "gmail provider is not configured")
	}
	settingsProvider, ok := gmailProvider.config.(*GoogleOAuthSettingsProvider)
	if !ok || settingsProvider == nil {
		return &OAuthSettings{Provider: ProviderGmail}, nil
	}
	return settingsProvider.OAuthSettings(ctx)
}

func (s *Service) UpdateOAuthSettings(ctx context.Context, in UpdateOAuthSettingsInput) (*OAuthSettings, error) {
	provider := normalizeProvider(in.Provider)
	if provider == "" {
		provider = ProviderGmail
	}
	if provider != ProviderGmail {
		return nil, infraerrors.New(http.StatusNotImplemented, "MAIL_PROVIDER_NOT_SUPPORTED", "mail provider is not supported")
	}
	gmailProvider, ok := s.providers[ProviderGmail].(*GmailProvider)
	if !ok || gmailProvider == nil || gmailProvider.config == nil {
		return nil, infraerrors.InternalServer("MAIL_PROVIDER_NOT_CONFIGURED", "gmail provider is not configured")
	}
	settingsProvider, ok := gmailProvider.config.(*GoogleOAuthSettingsProvider)
	if !ok || settingsProvider == nil || settingsProvider.settings == nil {
		return nil, infraerrors.InternalServer("MAIL_OAUTH_CONFIG_NOT_CONFIGURED", "mail oauth config is not configurable")
	}
	clientID := strings.TrimSpace(in.ClientID)
	clientSecret := strings.TrimSpace(in.ClientSecret)
	if clientID == "" {
		return nil, infraerrors.BadRequest("MAIL_OAUTH_CLIENT_ID_REQUIRED", "google oauth client id is required")
	}
	if clientSecret == "" {
		current, err := settingsProvider.OAuthSettings(ctx)
		if err != nil || current == nil || !current.ClientSecretConfigured {
			return nil, infraerrors.BadRequest("MAIL_OAUTH_CLIENT_SECRET_REQUIRED", "google oauth client secret is required")
		}
	}
	if err := settingsProvider.settings.SetGoogleOAuthClientCredentials(ctx, clientID, clientSecret); err != nil {
		return nil, err
	}
	return s.OAuthSettings(ctx, ProviderGmail)
}

func (s *Service) SaveCredential(ctx context.Context, in SaveCredentialInput) (*adminplusdomain.MailVerificationCredential, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.InternalServer("MAIL_VERIFICATION_SERVICE_NOT_CONFIGURED", "mail verification service is not configured")
	}
	provider := normalizeProvider(in.Provider)
	if !provider.Valid() {
		return nil, infraerrors.BadRequest("MAIL_PROVIDER_INVALID", "mail provider is invalid")
	}
	email := strings.ToLower(strings.TrimSpace(in.Email))
	if email == "" {
		return nil, infraerrors.BadRequest("MAIL_CREDENTIAL_EMAIL_REQUIRED", "mail credential email is required")
	}
	scopes := normalizeScopes(in.Scopes, in.Scope)
	if !hasGmailReadScope(scopes) {
		return nil, infraerrors.Forbidden("MAIL_GMAIL_SCOPE_REQUIRED", "gmail readonly scope is required")
	}
	if strings.TrimSpace(in.AccessToken) == "" {
		return nil, infraerrors.BadRequest("MAIL_CREDENTIAL_ACCESS_TOKEN_REQUIRED", "mail access token is required")
	}
	expiresAt := cloneTimePtr(in.ExpiresAt)
	if expiresAt == nil && in.ExpiresIn > 0 {
		t := s.now().UTC().Add(time.Duration(in.ExpiresIn) * time.Second)
		expiresAt = &t
	}
	now := s.now().UTC()
	credential := &Credential{
		Provider:     provider,
		Name:         trimLimit(in.Name, 120),
		Email:        email,
		EmailMasked:  maskEmail(email),
		AccessToken:  strings.TrimSpace(in.AccessToken),
		RefreshToken: strings.TrimSpace(in.RefreshToken),
		Scopes:       scopes,
		TokenType:    firstNonEmpty(in.TokenType, "Bearer"),
		ExpiresAt:    expiresAt,
		Metadata:     cloneStringMap(in.Metadata),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	created, err := s.repo.SaveCredential(ctx, credential)
	if err != nil {
		return nil, err
	}
	return created.Public(), nil
}

func (s *Service) ListCredentials(ctx context.Context, filter CredentialFilter) ([]*adminplusdomain.MailVerificationCredential, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.InternalServer("MAIL_VERIFICATION_SERVICE_NOT_CONFIGURED", "mail verification service is not configured")
	}
	filter.Provider = normalizeProvider(filter.Provider)
	if filter.Provider != "" && !filter.Provider.Valid() {
		return nil, infraerrors.BadRequest("MAIL_PROVIDER_INVALID", "mail provider is invalid")
	}
	return s.repo.ListCredentials(ctx, filter)
}

func (s *Service) CheckCredential(ctx context.Context, id int64) (*adminplusdomain.MailVerificationCredential, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.InternalServer("MAIL_VERIFICATION_SERVICE_NOT_CONFIGURED", "mail verification service is not configured")
	}
	credential, err := s.repo.GetCredential(ctx, id)
	if err != nil {
		return nil, err
	}
	provider, err := s.providerFor(credential.Provider)
	if err != nil {
		return nil, err
	}
	update, err := provider.CheckCredential(ctx, credential)
	now := s.now().UTC()
	if err != nil {
		_ = s.repo.RecordCredentialCheck(ctx, id, now, infraerrors.Reason(err))
		return nil, err
	}
	if update != nil {
		credential, err = s.repo.UpdateTokens(ctx, id, *update)
		if err != nil {
			return nil, err
		}
	}
	_ = s.repo.RecordCredentialCheck(ctx, id, now, "")
	if updated, getErr := s.repo.GetCredential(ctx, id); getErr == nil {
		credential = updated
	}
	return credential.Public(), nil
}

func (s *Service) ReadVerificationCode(ctx context.Context, in ReadVerificationCodeInput) (*ReadVerificationCodeResult, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.InternalServer("MAIL_VERIFICATION_SERVICE_NOT_CONFIGURED", "mail verification service is not configured")
	}
	if in.CredentialID <= 0 {
		return nil, infraerrors.BadRequest("MAIL_CREDENTIAL_ID_INVALID", "invalid mail credential id")
	}
	credential, err := s.repo.GetCredential(ctx, in.CredentialID)
	if err != nil {
		return nil, err
	}
	providerName := normalizeProvider(firstProvider(in.Provider, credential.Provider))
	if providerName != credential.Provider {
		return nil, infraerrors.BadRequest("MAIL_PROVIDER_MISMATCH", "mail provider does not match credential")
	}
	provider, err := s.providerFor(providerName)
	if err != nil {
		return nil, err
	}
	if !hasGmailReadScope(credential.Scopes) {
		return nil, infraerrors.Forbidden("MAIL_GMAIL_SCOPE_REQUIRED", "gmail readonly scope is required")
	}

	timeout := normalizeTimeout(in.TimeoutSeconds)
	pollInterval := normalizePollInterval(in.PollIntervalSeconds)
	deadlineCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	since := s.now().UTC().Add(-defaultRecentWindow)
	if in.TriggeredAt != nil && !in.TriggeredAt.IsZero() {
		since = in.TriggeredAt.UTC()
	}
	filter := ProviderSearchFilter{
		From:            in.From,
		To:              in.To,
		Keywords:        append([]string{}, in.Keywords...),
		MaxResults:      in.MaxResults,
		Since:           since,
		SupplierType:    adminplusdomain.NormalizeSupplierType(string(in.SupplierType)),
		ExpectedPurpose: normalizePurpose(in.ExpectedPurpose),
		SiteName:        strings.TrimSpace(in.SiteName),
	}
	claimKey := strings.TrimSpace(in.ClaimKey)
	if filter.MaxResults <= 0 {
		filter.MaxResults = defaultSearchMaxResults
	}

	for {
		result, err := provider.SearchMessages(deadlineCtx, credential, filter)
		if err != nil {
			s.recordReadFailure(ctx, in, credential, providerName, "search_messages", err)
			return nil, err
		}
		if result != nil && result.TokenUpdate != nil {
			credential, err = s.repo.UpdateTokens(deadlineCtx, credential.ID, *result.TokenUpdate)
			if err != nil {
				s.recordReadFailure(ctx, in, credential, providerName, "update_tokens", err)
				return nil, err
			}
		}
		if code := s.findCode(result, filter, claimKey); code != nil {
			s.recordReadSuccess(ctx, in, credential, code)
			return code, nil
		}

		select {
		case <-deadlineCtx.Done():
			if errors.Is(deadlineCtx.Err(), context.DeadlineExceeded) {
				err := infraerrors.NotFound("MAIL_VERIFICATION_CODE_NOT_FOUND", "mail verification code not found")
				s.recordReadFailure(ctx, in, credential, providerName, "poll_timeout", err)
				return nil, err
			}
			s.recordReadFailure(ctx, in, credential, providerName, "context_done", deadlineCtx.Err())
			return nil, deadlineCtx.Err()
		case <-time.After(pollInterval):
		}
	}
}

func (s *Service) ReadVerificationCodeForEmail(ctx context.Context, in ReadVerificationCodeForEmailInput) (*ReadVerificationCodeResult, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.InternalServer("MAIL_VERIFICATION_SERVICE_NOT_CONFIGURED", "mail verification service is not configured")
	}
	provider := normalizeProvider(in.Provider)
	if provider == "" {
		provider = ProviderGmail
	}
	email := strings.ToLower(strings.TrimSpace(in.Email))
	if email == "" {
		return nil, infraerrors.BadRequest("MAIL_TARGET_EMAIL_REQUIRED", "mail target email is required")
	}
	credential, err := s.selectCredentialForEmail(ctx, provider, email)
	if err != nil {
		s.recordReadForEmailFailure(ctx, in, provider, err)
		return nil, err
	}
	return s.ReadVerificationCode(ctx, ReadVerificationCodeInput{
		Provider:            credential.Provider,
		CredentialID:        credential.ID,
		ClaimKey:            in.ClaimKey,
		From:                in.From,
		To:                  email,
		Keywords:            in.Keywords,
		SupplierType:        in.SupplierType,
		ExpectedPurpose:     in.ExpectedPurpose,
		SiteName:            in.SiteName,
		TriggeredAt:         in.TriggeredAt,
		TimeoutSeconds:      in.TimeoutSeconds,
		PollIntervalSeconds: in.PollIntervalSeconds,
		MaxResults:          in.MaxResults,
	})
}

func (s *Service) recordReadSuccess(ctx context.Context, in ReadVerificationCodeInput, credential *Credential, result *ReadVerificationCodeResult) {
	if s == nil || s.bizlog == nil || result == nil {
		return
	}
	metadata := mailReadMetadata(in, credential)
	metadata["message_id"] = result.MessageID
	if !result.ReceivedAt.IsZero() {
		metadata["received_at"] = result.ReceivedAt.UTC().Format(time.RFC3339)
	}
	metadata["template_family"] = result.TemplateFamily
	metadata["confidence"] = result.Confidence
	metadata["supplier_type"] = string(result.SupplierType)
	metadata["purpose"] = result.Purpose
	s.bizlog.Record(ctx, bizlogs.Event{
		Level:        bizlogs.LevelInfo,
		Category:     bizlogs.CategoryMail,
		Action:       "read_verification_code",
		Outcome:      bizlogs.OutcomeSucceeded,
		Message:      "mail verification code read succeeded",
		ProviderType: string(result.Provider),
		Metadata:     metadata,
	})
}

func (s *Service) recordReadFailure(ctx context.Context, in ReadVerificationCodeInput, credential *Credential, provider adminplusdomain.MailVerificationProvider, action string, err error) {
	if s == nil || s.bizlog == nil {
		return
	}
	metadata := mailReadMetadata(in, credential)
	event := bizlogs.EventFromError(bizlogs.Event{
		Level:        bizlogs.LevelWarn,
		Category:     bizlogs.CategoryMail,
		Action:       action,
		Outcome:      bizlogs.OutcomeFailed,
		Message:      "mail verification code read failed",
		ProviderType: string(provider),
		Metadata:     metadata,
	}, err)
	s.bizlog.Record(ctx, event)
}

func (s *Service) recordReadForEmailFailure(ctx context.Context, in ReadVerificationCodeForEmailInput, provider adminplusdomain.MailVerificationProvider, err error) {
	if s == nil || s.bizlog == nil {
		return
	}
	metadata := map[string]any{
		"target_email":      maskEmail(in.Email),
		"supplier_type":     string(in.SupplierType),
		"expected_purpose":  in.ExpectedPurpose,
		"site_name":         in.SiteName,
		"timeout_seconds":   in.TimeoutSeconds,
		"poll_interval_sec": in.PollIntervalSeconds,
		"max_results":       in.MaxResults,
	}
	event := bizlogs.EventFromError(bizlogs.Event{
		Level:        bizlogs.LevelWarn,
		Category:     bizlogs.CategoryMail,
		Action:       "select_credential",
		Outcome:      bizlogs.OutcomeFailed,
		Message:      "mail credential selection failed",
		ProviderType: string(provider),
		Metadata:     metadata,
	}, err)
	s.bizlog.Record(ctx, event)
}

func mailReadMetadata(in ReadVerificationCodeInput, credential *Credential) map[string]any {
	out := map[string]any{
		"credential_id":     in.CredentialID,
		"provider":          string(in.Provider),
		"target_email":      maskEmail(in.To),
		"supplier_type":     string(in.SupplierType),
		"expected_purpose":  in.ExpectedPurpose,
		"site_name":         in.SiteName,
		"claim_key":         in.ClaimKey,
		"timeout_seconds":   in.TimeoutSeconds,
		"poll_interval_sec": in.PollIntervalSeconds,
		"max_results":       in.MaxResults,
	}
	if credential != nil {
		out["credential_id"] = credential.ID
		out["provider"] = string(credential.Provider)
		if out["target_email"] == "" {
			out["target_email"] = credential.EmailMasked
		}
	}
	if in.TriggeredAt != nil && !in.TriggeredAt.IsZero() {
		out["triggered_at"] = in.TriggeredAt.UTC().Format(time.RFC3339)
	}
	return out
}

func (s *Service) selectCredentialForEmail(ctx context.Context, provider adminplusdomain.MailVerificationProvider, email string) (*Credential, error) {
	if !provider.Valid() {
		return nil, infraerrors.BadRequest("MAIL_PROVIDER_INVALID", "mail provider is invalid")
	}
	items, err := s.repo.ListCredentialRecords(ctx, CredentialFilter{Provider: provider})
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if sameMailbox(strings.TrimSpace(item.Email), email) {
			return item, nil
		}
	}
	return nil, infraerrors.NotFound("MAIL_CREDENTIAL_NOT_FOUND", "mail credential for target email not found")
}

func (s *Service) SendAndReadTestVerificationCode(ctx context.Context, in SendTestVerificationCodeInput) (*ReadVerificationCodeResult, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.InternalServer("MAIL_VERIFICATION_SERVICE_NOT_CONFIGURED", "mail verification service is not configured")
	}
	if s.email == nil {
		return nil, infraerrors.ServiceUnavailable("MAIL_TEST_EMAIL_NOT_CONFIGURED", "email sender is not configured")
	}
	if in.CredentialID <= 0 {
		return nil, infraerrors.BadRequest("MAIL_CREDENTIAL_ID_INVALID", "invalid mail credential id")
	}
	credential, err := s.repo.GetCredential(ctx, in.CredentialID)
	if err != nil {
		return nil, err
	}
	recipient := strings.ToLower(strings.TrimSpace(credential.Email))
	if recipient == "" {
		return nil, infraerrors.BadRequest("MAIL_CREDENTIAL_EMAIL_UNAVAILABLE", "mail credential email is unavailable")
	}
	code, err := s.email.GenerateVerifyCode()
	if err != nil {
		return nil, err
	}
	siteName := firstNonEmpty(in.SiteName, "Lime")
	supplierType := adminplusdomain.NormalizeSupplierType(string(in.SupplierType))
	if supplierType == "" {
		supplierType = adminplusdomain.SupplierTypeSub2API
	}
	purpose := normalizePurpose(in.ExpectedPurpose)
	if purpose == "" {
		purpose = PurposeEmailVerification
	}
	subject := testMailSubject(siteName, supplierType, purpose)
	body := testMailBody(siteName, code, supplierType, purpose)
	triggeredAt := s.now().UTC()
	if err := s.email.SendEmail(ctx, recipient, subject, body); err != nil {
		return nil, err
	}
	result, err := s.ReadVerificationCode(ctx, ReadVerificationCodeInput{
		Provider:            credential.Provider,
		CredentialID:        credential.ID,
		SupplierType:        supplierType,
		ExpectedPurpose:     purpose,
		SiteName:            siteName,
		TriggeredAt:         &triggeredAt,
		TimeoutSeconds:      in.TimeoutSeconds,
		PollIntervalSeconds: in.PollIntervalSeconds,
		MaxResults:          defaultSearchMaxResults,
		Keywords:            []string{code, "verification code", "验证码"},
	})
	if err != nil {
		return nil, err
	}
	if result.Code != code {
		return nil, infraerrors.InternalServer("MAIL_TEST_CODE_MISMATCH", "read verification code does not match sent code")
	}
	return result, nil
}

func testMailSubject(siteName string, supplierType adminplusdomain.SupplierType, purpose string) string {
	switch supplierType {
	case adminplusdomain.SupplierTypeNewAPI:
		return siteName + "邮箱验证邮件"
	case adminplusdomain.SupplierTypeSub2API:
		if purpose == PurposeNotificationEmailVerification {
			return "[" + siteName + "] Notification email verification code"
		}
		return "[" + siteName + "] Email Verification Code"
	default:
		return "[" + siteName + "] Email Verification Code"
	}
}

func testMailBody(siteName, code string, supplierType adminplusdomain.SupplierType, purpose string) string {
	text := "Your verification code is: " + code + ". It expires in 15 minutes."
	if supplierType == adminplusdomain.SupplierTypeNewAPI {
		text = "您好，你正在进行" + siteName + "邮箱验证。您的验证码为: " + code
	}
	if supplierType == adminplusdomain.SupplierTypeSub2API && purpose == PurposeNotificationEmailVerification {
		text = "Your verification code is: " + code + ". This code verifies your notification email."
	}
	return `<!doctype html>
<html>
<head><meta charset="UTF-8"></head>
<body>
  <p>` + text + `</p>
  <p>This is a real mail verification test sent by Admin Plus.</p>
</body>
</html>`
}

func (s *Service) findCode(result *ProviderSearchResult, filter ProviderSearchFilter, claimKey string) *ReadVerificationCodeResult {
	if result == nil {
		return nil
	}
	fallbacks := make([]*ReadVerificationCodeResult, 0, 2)
	for _, message := range result.Messages {
		if message.ID == "" {
			continue
		}
		if !message.InternalDate.IsZero() && message.InternalDate.Before(filter.Since) {
			continue
		}
		combined := message.Subject + "\n" + message.Snippet + "\n" + message.Text
		match := s.classifier.Classify(TemplateClassifyInput{
			SupplierType:    filter.SupplierType,
			ExpectedPurpose: filter.ExpectedPurpose,
			SiteName:        filter.SiteName,
			Subject:         message.Subject,
			From:            message.From,
			Snippet:         message.Snippet,
			Text:            message.Text,
		})
		if match.Excluded {
			continue
		}
		isFallback := false
		if !match.Matched {
			fallbackMatch := fallbackVerificationMatch(filter, message, combined)
			if fallbackMatch == nil {
				continue
			}
			match = *fallbackMatch
			isFallback = true
		}
		code := ExtractVerificationCode(combined)
		if code == "" {
			continue
		}
		item := &ReadVerificationCodeResult{
			Provider:       ProviderGmail,
			Code:           code,
			MessageID:      message.ID,
			ReceivedAt:     message.InternalDate,
			TemplateFamily: match.Family,
			Confidence:     match.Confidence,
			SupplierType:   match.SupplierType,
			Purpose:        match.Purpose,
		}
		if isFallback {
			fallbacks = append(fallbacks, item)
			continue
		}
		if !s.claimMessage(message.ID, claimKey) {
			continue
		}
		return item
	}
	for _, item := range fallbacks {
		if s.claimMessage(item.MessageID, claimKey) {
			return item
		}
	}
	return nil
}

func (s *Service) claimMessage(messageID string, claimKey string) bool {
	messageID = strings.TrimSpace(messageID)
	claimKey = strings.TrimSpace(claimKey)
	if messageID == "" || claimKey == "" {
		return true
	}
	now := s.now().UTC()
	s.claimsMu.Lock()
	defer s.claimsMu.Unlock()
	if s.claims == nil {
		s.claims = make(map[string]messageClaim)
	}
	for id, claim := range s.claims {
		if !claim.ExpireAt.After(now) {
			delete(s.claims, id)
		}
	}
	if existing, ok := s.claims[messageID]; ok && existing.ExpireAt.After(now) && existing.Key != claimKey {
		return false
	}
	s.claims[messageID] = messageClaim{
		Key:      claimKey,
		ExpireAt: now.Add(defaultRecentWindow),
	}
	return true
}

func fallbackVerificationMatch(filter ProviderSearchFilter, message MailMessage, text string) *adminplusdomain.MailTemplateMatch {
	if isPasswordResetMail(strings.ToLower(text)) {
		return nil
	}
	if !subjectMatchesSite(message.Subject, filter.SiteName) {
		return nil
	}
	if ExtractVerificationCode(text) == "" {
		return nil
	}
	return &adminplusdomain.MailTemplateMatch{
		SupplierType: filter.SupplierType,
		Purpose:      firstNonEmpty(filter.ExpectedPurpose, PurposeEmailVerification),
		Family:       "generic.verification_code",
		Confidence:   0.6,
		Matched:      true,
	}
}

func (s *Service) providerFor(provider adminplusdomain.MailVerificationProvider) (MailProvider, error) {
	provider = normalizeProvider(provider)
	if provider == "" {
		provider = ProviderGmail
	}
	if !provider.Valid() {
		return nil, infraerrors.BadRequest("MAIL_PROVIDER_INVALID", "mail provider is invalid")
	}
	if s == nil || s.providers == nil {
		return nil, infraerrors.InternalServer("MAIL_PROVIDER_NOT_CONFIGURED", "mail provider is not configured")
	}
	mailProvider := s.providers[provider]
	if mailProvider == nil {
		return nil, infraerrors.New(http.StatusNotImplemented, "MAIL_PROVIDER_NOT_SUPPORTED", "mail provider is not supported")
	}
	return mailProvider, nil
}

func normalizeProvider(provider adminplusdomain.MailVerificationProvider) adminplusdomain.MailVerificationProvider {
	return adminplusdomain.MailVerificationProvider(strings.ToLower(strings.TrimSpace(string(provider))))
}

func firstProvider(values ...adminplusdomain.MailVerificationProvider) adminplusdomain.MailVerificationProvider {
	for _, value := range values {
		if normalized := normalizeProvider(value); normalized != "" {
			return normalized
		}
	}
	return ""
}

func sameMailbox(a string, b string) bool {
	a = strings.ToLower(strings.TrimSpace(a))
	b = strings.ToLower(strings.TrimSpace(b))
	if a == "" || b == "" {
		return false
	}
	if a == b {
		return true
	}
	return normalizeGmailAlias(a) == normalizeGmailAlias(b)
}

func normalizeGmailAlias(email string) string {
	local, domain, ok := strings.Cut(strings.ToLower(strings.TrimSpace(email)), "@")
	if !ok {
		return email
	}
	if domain != "gmail.com" && domain != "googlemail.com" {
		return email
	}
	if beforePlus, _, found := strings.Cut(local, "+"); found {
		local = beforePlus
	}
	local = strings.ReplaceAll(local, ".", "")
	return local + "@gmail.com"
}

func normalizeTimeout(seconds int) time.Duration {
	if seconds <= 0 {
		return defaultReadTimeout
	}
	timeout := time.Duration(seconds) * time.Second
	if timeout > maxReadTimeout {
		return maxReadTimeout
	}
	if timeout < minPollInterval {
		return minPollInterval
	}
	return timeout
}

func normalizePollInterval(seconds int) time.Duration {
	if seconds <= 0 {
		return defaultPollInterval
	}
	interval := time.Duration(seconds) * time.Second
	if interval < minPollInterval {
		return minPollInterval
	}
	if interval > 30*time.Second {
		return 30 * time.Second
	}
	return interval
}
