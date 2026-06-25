package mailverification

import (
	"context"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
)

const (
	ProviderGmail = adminplusdomain.MailVerificationProviderGmail

	GmailReadonlyScope = "https://www.googleapis.com/auth/gmail.readonly"

	PurposeEmailVerification             = "email_verification"
	PurposeNotificationEmailVerification = "notification_email_verification"
)

type Credential struct {
	ID           int64
	Provider     adminplusdomain.MailVerificationProvider
	Name         string
	Email        string
	EmailMasked  string
	AccessToken  string
	RefreshToken string
	Scopes       []string
	TokenType    string
	ExpiresAt    *time.Time
	Metadata     map[string]string

	LastCheckedAt *time.Time
	LastErrorCode string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (c *Credential) Public() *adminplusdomain.MailVerificationCredential {
	if c == nil {
		return nil
	}
	return &adminplusdomain.MailVerificationCredential{
		ID:            c.ID,
		Provider:      c.Provider,
		Name:          c.Name,
		Email:         "",
		EmailMasked:   firstNonEmpty(c.EmailMasked, maskEmail(c.Email)),
		Scopes:        append([]string{}, c.Scopes...),
		TokenType:     c.TokenType,
		ExpiresAt:     cloneTimePtr(c.ExpiresAt),
		Metadata:      cloneStringMap(c.Metadata),
		LastCheckedAt: cloneTimePtr(c.LastCheckedAt),
		LastErrorCode: c.LastErrorCode,
		CreatedAt:     c.CreatedAt,
		UpdatedAt:     c.UpdatedAt,
	}
}

type SaveCredentialInput struct {
	Provider     adminplusdomain.MailVerificationProvider
	Name         string
	Email        string
	AccessToken  string
	RefreshToken string
	Scopes       []string
	Scope        string
	TokenType    string
	ExpiresAt    *time.Time
	ExpiresIn    int
	Metadata     map[string]string
}

type CredentialFilter struct {
	Provider adminplusdomain.MailVerificationProvider
}

type TokenUpdate struct {
	AccessToken  string
	RefreshToken string
	Scopes       []string
	TokenType    string
	ExpiresAt    *time.Time
}

type Repository interface {
	SaveCredential(ctx context.Context, credential *Credential) (*Credential, error)
	GetCredential(ctx context.Context, id int64) (*Credential, error)
	ListCredentialRecords(ctx context.Context, filter CredentialFilter) ([]*Credential, error)
	ListCredentials(ctx context.Context, filter CredentialFilter) ([]*adminplusdomain.MailVerificationCredential, error)
	UpdateTokens(ctx context.Context, id int64, update TokenUpdate) (*Credential, error)
	RecordCredentialCheck(ctx context.Context, id int64, checkedAt time.Time, errorCode string) error
}

type ProviderSearchFilter struct {
	Query           string
	From            string
	To              string
	Keywords        []string
	MaxResults      int
	IncludeSpam     bool
	Since           time.Time
	SupplierType    adminplusdomain.SupplierType
	ExpectedPurpose string
	SiteName        string
}

type MailMessage struct {
	ID           string
	ThreadID     string
	Subject      string
	From         string
	Snippet      string
	Text         string
	InternalDate time.Time
}

type ProviderSearchResult struct {
	Messages    []MailMessage
	TokenUpdate *TokenUpdate
}

type EmailSender interface {
	SendEmail(ctx context.Context, to, subject, body string) error
	GenerateVerifyCode() (string, error)
}

type OAuthClientConfig struct {
	ClientID     string
	ClientSecret string
	AuthorizeURL string
	TokenURL     string
}

type OAuthSettings struct {
	Provider               adminplusdomain.MailVerificationProvider `json:"provider"`
	Enabled                bool                                     `json:"enabled"`
	ClientID               string                                   `json:"client_id,omitempty"`
	ClientSecretConfigured bool                                     `json:"client_secret_configured"`
	RedirectURI            string                                   `json:"redirect_uri,omitempty"`
	FrontendRedirectURI    string                                   `json:"frontend_redirect_uri,omitempty"`
}

type UpdateOAuthSettingsInput struct {
	Provider            adminplusdomain.MailVerificationProvider
	ClientID            string
	ClientSecret        string
	RedirectURI         string
	FrontendRedirectURI string
}

type OAuthConfigProvider interface {
	GoogleOAuthClient(ctx context.Context) (OAuthClientConfig, error)
}

type OAuthAuthorizeInput struct {
	Provider    adminplusdomain.MailVerificationProvider
	RedirectURI string
	State       string
	LoginHint   string
}

type OAuthAuthorizeResult struct {
	Provider     adminplusdomain.MailVerificationProvider `json:"provider"`
	AuthorizeURL string                                   `json:"authorize_url"`
	Scope        string                                   `json:"scope"`
}

type ExchangeCodeInput struct {
	Provider    adminplusdomain.MailVerificationProvider
	Code        string
	RedirectURI string
	Name        string
}

type MailProvider interface {
	Name() adminplusdomain.MailVerificationProvider
	CheckCredential(ctx context.Context, credential *Credential) (*TokenUpdate, error)
	SearchMessages(ctx context.Context, credential *Credential, filter ProviderSearchFilter) (*ProviderSearchResult, error)
	AuthorizeURL(ctx context.Context, in OAuthAuthorizeInput) (*OAuthAuthorizeResult, error)
	ExchangeCode(ctx context.Context, in ExchangeCodeInput) (*SaveCredentialInput, error)
}

type ReadVerificationCodeInput struct {
	Provider            adminplusdomain.MailVerificationProvider
	CredentialID        int64
	ClaimKey            string
	From                string
	To                  string
	Keywords            []string
	SupplierType        adminplusdomain.SupplierType
	ExpectedPurpose     string
	SiteName            string
	TriggeredAt         *time.Time
	TimeoutSeconds      int
	PollIntervalSeconds int
	MaxResults          int
}

type ReadVerificationCodeForEmailInput struct {
	Provider            adminplusdomain.MailVerificationProvider
	Email               string
	ClaimKey            string
	From                string
	To                  string
	Keywords            []string
	SupplierType        adminplusdomain.SupplierType
	ExpectedPurpose     string
	SiteName            string
	TriggeredAt         *time.Time
	TimeoutSeconds      int
	PollIntervalSeconds int
	MaxResults          int
}

type ReadVerificationCodeResult struct {
	Provider       adminplusdomain.MailVerificationProvider `json:"provider"`
	Code           string                                   `json:"code"`
	MessageID      string                                   `json:"message_id"`
	ReceivedAt     time.Time                                `json:"received_at"`
	TemplateFamily string                                   `json:"template_family,omitempty"`
	Confidence     float64                                  `json:"confidence,omitempty"`
	SupplierType   adminplusdomain.SupplierType             `json:"supplier_type,omitempty"`
	Purpose        string                                   `json:"purpose,omitempty"`
}

type SendTestVerificationCodeInput struct {
	CredentialID        int64
	SupplierType        adminplusdomain.SupplierType
	ExpectedPurpose     string
	SiteName            string
	TimeoutSeconds      int
	PollIntervalSeconds int
}

type TemplateClassifyInput struct {
	SupplierType    adminplusdomain.SupplierType
	ExpectedPurpose string
	SiteName        string
	Subject         string
	From            string
	Snippet         string
	Text            string
}

type TemplateClassifier interface {
	Classify(in TemplateClassifyInput) adminplusdomain.MailTemplateMatch
}
