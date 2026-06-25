package domain

import "time"

type MailVerificationProvider string

const (
	MailVerificationProviderGmail MailVerificationProvider = "gmail"
)

func (p MailVerificationProvider) Valid() bool {
	switch p {
	case MailVerificationProviderGmail:
		return true
	default:
		return false
	}
}

type MailVerificationCredential struct {
	ID            int64                    `json:"id"`
	Provider      MailVerificationProvider `json:"provider"`
	Name          string                   `json:"name,omitempty"`
	Email         string                   `json:"email,omitempty"`
	EmailMasked   string                   `json:"email_masked,omitempty"`
	Scopes        []string                 `json:"scopes,omitempty"`
	TokenType     string                   `json:"token_type,omitempty"`
	ExpiresAt     *time.Time               `json:"expires_at,omitempty"`
	Metadata      map[string]string        `json:"metadata,omitempty"`
	LastCheckedAt *time.Time               `json:"last_checked_at,omitempty"`
	LastErrorCode string                   `json:"last_error_code,omitempty"`
	CreatedAt     time.Time                `json:"created_at"`
	UpdatedAt     time.Time                `json:"updated_at"`
}

type MailTemplateMatch struct {
	SupplierType  SupplierType `json:"supplier_type,omitempty"`
	Purpose       string       `json:"purpose,omitempty"`
	Family        string       `json:"family,omitempty"`
	Confidence    float64      `json:"confidence,omitempty"`
	Matched       bool         `json:"matched"`
	Excluded      bool         `json:"excluded,omitempty"`
	ExcludeReason string       `json:"exclude_reason,omitempty"`
}
