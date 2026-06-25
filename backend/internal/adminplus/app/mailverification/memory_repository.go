package mailverification

import (
	"context"
	"net/http"
	"strings"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type MemoryRepository struct {
	items []*Credential
	next  int64
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{next: 1}
}

func (r *MemoryRepository) SaveCredential(_ context.Context, credential *Credential) (*Credential, error) {
	if r == nil {
		return nil, infraerrors.InternalServer("MAIL_VERIFICATION_REPOSITORY_NOT_CONFIGURED", "mail verification repository is not configured")
	}
	if credential == nil {
		return nil, infraerrors.BadRequest("MAIL_CREDENTIAL_INVALID", "mail credential is required")
	}
	now := time.Now().UTC()
	item := cloneCredential(credential)
	item.Provider = adminplusdomain.MailVerificationProvider(strings.ToLower(strings.TrimSpace(string(item.Provider))))
	item.Email = strings.ToLower(strings.TrimSpace(item.Email))
	item.EmailMasked = firstNonEmpty(item.EmailMasked, maskEmail(item.Email))
	item.Scopes = normalizeScopes(item.Scopes, "")
	if item.TokenType == "" {
		item.TokenType = "Bearer"
	}
	for _, existing := range r.items {
		if existing.Provider == item.Provider && existing.Email == item.Email {
			item.ID = existing.ID
			item.CreatedAt = existing.CreatedAt
			item.UpdatedAt = now
			if item.RefreshToken == "" {
				item.RefreshToken = existing.RefreshToken
			}
			*existing = *item
			return cloneCredential(existing), nil
		}
	}
	item.ID = r.next
	r.next++
	item.CreatedAt = now
	item.UpdatedAt = now
	r.items = append(r.items, item)
	return cloneCredential(item), nil
}

func (r *MemoryRepository) GetCredential(_ context.Context, id int64) (*Credential, error) {
	if r == nil {
		return nil, infraerrors.InternalServer("MAIL_VERIFICATION_REPOSITORY_NOT_CONFIGURED", "mail verification repository is not configured")
	}
	for _, item := range r.items {
		if item.ID == id {
			return cloneCredential(item), nil
		}
	}
	return nil, infraerrors.NotFound("MAIL_CREDENTIAL_NOT_FOUND", "mail credential not found")
}

func (r *MemoryRepository) ListCredentials(_ context.Context, filter CredentialFilter) ([]*adminplusdomain.MailVerificationCredential, error) {
	if r == nil {
		return nil, infraerrors.InternalServer("MAIL_VERIFICATION_REPOSITORY_NOT_CONFIGURED", "mail verification repository is not configured")
	}
	items := make([]*adminplusdomain.MailVerificationCredential, 0, len(r.items))
	for i := len(r.items) - 1; i >= 0; i-- {
		item := r.items[i]
		if filter.Provider != "" && item.Provider != filter.Provider {
			continue
		}
		items = append(items, item.Public())
	}
	return items, nil
}

func (r *MemoryRepository) ListCredentialRecords(_ context.Context, filter CredentialFilter) ([]*Credential, error) {
	if r == nil {
		return nil, infraerrors.InternalServer("MAIL_VERIFICATION_REPOSITORY_NOT_CONFIGURED", "mail verification repository is not configured")
	}
	items := make([]*Credential, 0, len(r.items))
	for i := len(r.items) - 1; i >= 0; i-- {
		item := r.items[i]
		if filter.Provider != "" && item.Provider != filter.Provider {
			continue
		}
		items = append(items, cloneCredential(item))
	}
	return items, nil
}

func (r *MemoryRepository) UpdateTokens(_ context.Context, id int64, update TokenUpdate) (*Credential, error) {
	if r == nil {
		return nil, infraerrors.InternalServer("MAIL_VERIFICATION_REPOSITORY_NOT_CONFIGURED", "mail verification repository is not configured")
	}
	if strings.TrimSpace(update.AccessToken) == "" {
		return nil, infraerrors.New(http.StatusBadRequest, "MAIL_CREDENTIAL_ACCESS_TOKEN_REQUIRED", "mail access token is required")
	}
	for _, item := range r.items {
		if item.ID == id {
			item.AccessToken = strings.TrimSpace(update.AccessToken)
			if strings.TrimSpace(update.RefreshToken) != "" {
				item.RefreshToken = strings.TrimSpace(update.RefreshToken)
			}
			if len(update.Scopes) > 0 {
				item.Scopes = normalizeScopes(update.Scopes, "")
			}
			if strings.TrimSpace(update.TokenType) != "" {
				item.TokenType = strings.TrimSpace(update.TokenType)
			}
			item.ExpiresAt = cloneTimePtr(update.ExpiresAt)
			item.LastErrorCode = ""
			item.UpdatedAt = time.Now().UTC()
			return cloneCredential(item), nil
		}
	}
	return nil, infraerrors.NotFound("MAIL_CREDENTIAL_NOT_FOUND", "mail credential not found")
}

func (r *MemoryRepository) RecordCredentialCheck(_ context.Context, id int64, checkedAt time.Time, errorCode string) error {
	if r == nil {
		return infraerrors.InternalServer("MAIL_VERIFICATION_REPOSITORY_NOT_CONFIGURED", "mail verification repository is not configured")
	}
	for _, item := range r.items {
		if item.ID == id {
			t := checkedAt.UTC()
			item.LastCheckedAt = &t
			item.LastErrorCode = trimLimit(errorCode, 80)
			item.UpdatedAt = time.Now().UTC()
			return nil
		}
	}
	return infraerrors.NotFound("MAIL_CREDENTIAL_NOT_FOUND", "mail credential not found")
}

func cloneCredential(in *Credential) *Credential {
	if in == nil {
		return nil
	}
	out := *in
	out.Scopes = append([]string{}, in.Scopes...)
	out.Metadata = cloneStringMap(in.Metadata)
	out.ExpiresAt = cloneTimePtr(in.ExpiresAt)
	out.LastCheckedAt = cloneTimePtr(in.LastCheckedAt)
	return &out
}
