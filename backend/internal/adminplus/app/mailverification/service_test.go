package mailverification

import (
	"context"
	"testing"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestServiceReadVerificationCodeUsesSupplierTemplate(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	repo := NewMemoryRepository()
	credential := saveTestMailCredential(t, ctx, repo)
	provider := &fakeMailProvider{
		result: &ProviderSearchResult{
			TokenUpdate: &TokenUpdate{
				AccessToken: "new-access-token",
				Scopes:      []string{GmailReadonlyScope},
				TokenType:   "Bearer",
			},
			Messages: []MailMessage{
				{
					ID:           "reset-message",
					Subject:      "[Lime] Password Reset",
					Text:         "Reset password link token 999999",
					InternalDate: now.Add(-time.Minute),
				},
				{
					ID:           "verification-message",
					Subject:      "[Lime] Email Verification Code",
					Text:         "Your verification code is: 654321. It expires in 15 minutes.",
					InternalDate: now.Add(-30 * time.Second),
				},
			},
		},
	}
	service := newTestMailVerificationService(repo, provider, now)

	result, err := service.ReadVerificationCode(ctx, ReadVerificationCodeInput{
		CredentialID:    credential.ID,
		SupplierType:    adminplusdomain.SupplierTypeSub2API,
		SiteName:        "Lime",
		TriggeredAt:     ptrTime(now.Add(-5 * time.Minute)),
		TimeoutSeconds:  2,
		MaxResults:      10,
		ExpectedPurpose: PurposeEmailVerification,
	})

	require.NoError(t, err)
	require.Equal(t, "654321", result.Code)
	require.Equal(t, "verification-message", result.MessageID)
	require.Equal(t, "sub2api.auth_verify_code", result.TemplateFamily)
	require.Equal(t, PurposeEmailVerification, result.Purpose)
	require.Equal(t, adminplusdomain.SupplierTypeSub2API, result.SupplierType)
	require.Equal(t, 1, provider.searchCalls)
	require.Equal(t, adminplusdomain.SupplierTypeSub2API, provider.lastFilter.SupplierType)

	updated, err := repo.GetCredential(ctx, credential.ID)
	require.NoError(t, err)
	require.Equal(t, "new-access-token", updated.AccessToken)
}

func TestServiceReadVerificationCodeRejectsSiteMismatch(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	repo := NewMemoryRepository()
	credential := saveTestMailCredential(t, context.Background(), repo)
	provider := &fakeMailProvider{
		result: &ProviderSearchResult{
			Messages: []MailMessage{
				{
					ID:           "wrong-site-message",
					Subject:      "Other邮箱验证邮件",
					Text:         "您好，你正在进行Other邮箱验证。您的验证码为: 123456",
					InternalDate: now.Add(-time.Minute),
				},
			},
		},
	}
	service := newTestMailVerificationService(repo, provider, now)

	result, err := service.ReadVerificationCode(ctx, ReadVerificationCodeInput{
		CredentialID:        credential.ID,
		SupplierType:        adminplusdomain.SupplierTypeNewAPI,
		SiteName:            "Lime",
		TriggeredAt:         ptrTime(now.Add(-5 * time.Minute)),
		TimeoutSeconds:      120,
		PollIntervalSeconds: 2,
		MaxResults:          10,
		ExpectedPurpose:     PurposeEmailVerification,
	})

	require.Nil(t, result)
	require.Error(t, err)
	require.Equal(t, "MAIL_VERIFICATION_CODE_NOT_FOUND", infraerrors.Reason(err))
	require.GreaterOrEqual(t, provider.searchCalls, 1)
}

func TestServiceReadVerificationCodeFallsBackToGenericAlphanumericCode(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	repo := NewMemoryRepository()
	credential := saveTestMailCredential(t, ctx, repo)
	provider := &fakeMailProvider{
		result: &ProviderSearchResult{
			Messages: []MailMessage{
				{
					ID:           "generic-message",
					Subject:      "示例站点 邮箱验证邮件",
					Text:         "您好，你正在进行示例站点邮箱验证。\n您的验证码为: b39bbd\n验证码 10 分钟内有效。",
					InternalDate: now.Add(-30 * time.Second),
				},
			},
		},
	}
	service := newTestMailVerificationService(repo, provider, now)

	result, err := service.ReadVerificationCode(ctx, ReadVerificationCodeInput{
		CredentialID:    credential.ID,
		SupplierType:    adminplusdomain.SupplierTypeSub2API,
		TriggeredAt:     ptrTime(now.Add(-5 * time.Minute)),
		TimeoutSeconds:  2,
		MaxResults:      10,
		ExpectedPurpose: PurposeEmailVerification,
	})

	require.NoError(t, err)
	require.Equal(t, "b39bbd", result.Code)
	require.Equal(t, "generic.verification_code", result.TemplateFamily)
	require.Equal(t, adminplusdomain.SupplierTypeSub2API, result.SupplierType)
}

func TestServiceReadVerificationCodeClaimPreventsCrossTaskReuse(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	repo := NewMemoryRepository()
	credential := saveTestMailCredential(t, ctx, repo)
	provider := &fakeMailProvider{
		result: &ProviderSearchResult{
			Messages: []MailMessage{
				{
					ID:           "claimed-message",
					Subject:      "[Lime] Email Verification Code",
					Text:         "Your verification code is: 654321. It expires in 15 minutes.",
					InternalDate: now.Add(-30 * time.Second),
				},
			},
		},
	}
	service := newTestMailVerificationService(repo, provider, now)

	first, err := service.ReadVerificationCode(ctx, ReadVerificationCodeInput{
		CredentialID:        credential.ID,
		ClaimKey:            "registration_task:1",
		SupplierType:        adminplusdomain.SupplierTypeSub2API,
		SiteName:            "Lime",
		TriggeredAt:         ptrTime(now.Add(-5 * time.Minute)),
		TimeoutSeconds:      2,
		PollIntervalSeconds: 2,
		ExpectedPurpose:     PurposeEmailVerification,
	})
	require.NoError(t, err)
	require.Equal(t, "claimed-message", first.MessageID)

	second, err := service.ReadVerificationCode(ctx, ReadVerificationCodeInput{
		CredentialID:        credential.ID,
		ClaimKey:            "registration_task:1",
		SupplierType:        adminplusdomain.SupplierTypeSub2API,
		SiteName:            "Lime",
		TriggeredAt:         ptrTime(now.Add(-5 * time.Minute)),
		TimeoutSeconds:      2,
		PollIntervalSeconds: 2,
		ExpectedPurpose:     PurposeEmailVerification,
	})
	require.NoError(t, err)
	require.Equal(t, "claimed-message", second.MessageID)

	ctxThird, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
	defer cancel()
	third, err := service.ReadVerificationCode(ctxThird, ReadVerificationCodeInput{
		CredentialID:        credential.ID,
		ClaimKey:            "registration_task:2",
		SupplierType:        adminplusdomain.SupplierTypeSub2API,
		SiteName:            "Lime",
		TriggeredAt:         ptrTime(now.Add(-5 * time.Minute)),
		TimeoutSeconds:      120,
		PollIntervalSeconds: 2,
		ExpectedPurpose:     PurposeEmailVerification,
	})
	require.Nil(t, third)
	require.Error(t, err)
	require.Equal(t, "MAIL_VERIFICATION_CODE_NOT_FOUND", infraerrors.Reason(err))
}

func TestServiceReadVerificationCodeForEmailRequiresMatchingCredential(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	repo := NewMemoryRepository()
	saveTestMailCredential(t, ctx, repo)
	provider := &fakeMailProvider{
		result: &ProviderSearchResult{
			Messages: []MailMessage{
				{
					ID:           "verification-message",
					Subject:      "[Lime] Email Verification Code",
					Text:         "Your verification code is: 654321. It expires in 15 minutes.",
					InternalDate: now.Add(-30 * time.Second),
				},
			},
		},
	}
	service := newTestMailVerificationService(repo, provider, now)

	result, err := service.ReadVerificationCodeForEmail(ctx, ReadVerificationCodeForEmailInput{
		Email:          "other@example.com",
		TriggeredAt:    ptrTime(now.Add(-5 * time.Minute)),
		TimeoutSeconds: 2,
	})

	require.Nil(t, result)
	require.Error(t, err)
	require.Equal(t, "MAIL_CREDENTIAL_NOT_FOUND", infraerrors.Reason(err))
	require.Equal(t, 0, provider.searchCalls)
}

func TestServiceReadVerificationCodeForEmailMatchesGmailAlias(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	repo := NewMemoryRepository()
	credential, err := repo.SaveCredential(ctx, &Credential{
		Provider:    ProviderGmail,
		Email:       "operator+signup@gmail.com",
		AccessToken: "access-token",
		Scopes:      []string{GmailReadonlyScope},
		TokenType:   "Bearer",
	})
	require.NoError(t, err)
	provider := &fakeMailProvider{
		result: &ProviderSearchResult{
			Messages: []MailMessage{
				{
					ID:           "verification-message",
					Subject:      "[Lime] Email Verification Code",
					Text:         "Your verification code is: 654321. It expires in 15 minutes.",
					InternalDate: now.Add(-30 * time.Second),
				},
			},
		},
	}
	service := newTestMailVerificationService(repo, provider, now)

	result, err := service.ReadVerificationCodeForEmail(ctx, ReadVerificationCodeForEmailInput{
		Email:               "o.p.e.r.a.t.o.r@gmail.com",
		TriggeredAt:         ptrTime(now.Add(-5 * time.Minute)),
		TimeoutSeconds:      2,
		PollIntervalSeconds: 2,
		SupplierType:        adminplusdomain.SupplierTypeSub2API,
		SiteName:            "Lime",
		ExpectedPurpose:     PurposeEmailVerification,
	})

	require.NoError(t, err)
	require.Equal(t, "654321", result.Code)
	require.Equal(t, credential.ID, provider.lastCredentialID)
}

func TestServiceSendAndReadTestVerificationCodeSendsRealMailThenReadsGmail(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	repo := NewMemoryRepository()
	credential := saveTestMailCredential(t, ctx, repo)
	email := &fakeEmailSender{code: "654321"}
	provider := &fakeMailProvider{
		result: &ProviderSearchResult{
			Messages: []MailMessage{
				{
					ID:           "test-message",
					Subject:      "[Lime] Email Verification Code",
					Text:         "Your verification code is: 654321. It expires in 15 minutes.",
					InternalDate: now.Add(2 * time.Second),
				},
			},
		},
	}
	service := newTestMailVerificationService(repo, provider, now)
	service.email = email

	result, err := service.SendAndReadTestVerificationCode(ctx, SendTestVerificationCodeInput{
		CredentialID:    credential.ID,
		SupplierType:    adminplusdomain.SupplierTypeSub2API,
		ExpectedPurpose: PurposeEmailVerification,
		SiteName:        "Lime",
		TimeoutSeconds:  2,
	})

	require.NoError(t, err)
	require.Equal(t, "654321", result.Code)
	require.Equal(t, "operator@example.com", email.to)
	require.Contains(t, email.subject, "Email Verification Code")
	require.Contains(t, email.body, "654321")
	require.Equal(t, 1, provider.searchCalls)
	require.Contains(t, provider.lastFilter.Keywords, "654321")
}

func saveTestMailCredential(t *testing.T, ctx context.Context, repo *MemoryRepository) *Credential {
	t.Helper()
	credential, err := repo.SaveCredential(ctx, &Credential{
		Provider:    ProviderGmail,
		Email:       "operator@example.com",
		AccessToken: "access-token",
		Scopes:      []string{GmailReadonlyScope},
		TokenType:   "Bearer",
	})
	require.NoError(t, err)
	return credential
}

func newTestMailVerificationService(repo Repository, provider MailProvider, now time.Time) *Service {
	return &Service{
		repo: repo,
		providers: map[adminplusdomain.MailVerificationProvider]MailProvider{
			provider.Name(): provider,
		},
		classifier: NewDefaultTemplateClassifier(),
		now: func() time.Time {
			return now
		},
	}
}

func ptrTime(value time.Time) *time.Time {
	return &value
}

type fakeMailProvider struct {
	result           *ProviderSearchResult
	err              error
	searchCalls      int
	lastFilter       ProviderSearchFilter
	lastCredentialID int64
}

func (p *fakeMailProvider) Name() adminplusdomain.MailVerificationProvider {
	return ProviderGmail
}

func (p *fakeMailProvider) CheckCredential(context.Context, *Credential) (*TokenUpdate, error) {
	return nil, nil
}

func (p *fakeMailProvider) SearchMessages(_ context.Context, credential *Credential, filter ProviderSearchFilter) (*ProviderSearchResult, error) {
	p.searchCalls++
	p.lastFilter = filter
	if credential != nil {
		p.lastCredentialID = credential.ID
	}
	return p.result, p.err
}

func (p *fakeMailProvider) AuthorizeURL(context.Context, OAuthAuthorizeInput) (*OAuthAuthorizeResult, error) {
	return nil, nil
}

func (p *fakeMailProvider) ExchangeCode(context.Context, ExchangeCodeInput) (*SaveCredentialInput, error) {
	return nil, nil
}

type fakeEmailSender struct {
	code    string
	to      string
	subject string
	body    string
}

func (s *fakeEmailSender) GenerateVerifyCode() (string, error) {
	return s.code, nil
}

func (s *fakeEmailSender) SendEmail(_ context.Context, to, subject, body string) error {
	s.to = to
	s.subject = subject
	s.body = body
	return nil
}
