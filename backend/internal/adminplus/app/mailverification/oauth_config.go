package mailverification

import (
	"context"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

const (
	defaultGoogleAuthorizeURL = "https://accounts.google.com/o/oauth2/v2/auth"
	defaultGoogleTokenURL     = "https://oauth2.googleapis.com/token"
)

type GoogleOAuthSettingsProvider struct {
	settings *service.SettingService
}

func NewGoogleOAuthSettingsProvider(settings *service.SettingService) *GoogleOAuthSettingsProvider {
	return &GoogleOAuthSettingsProvider{settings: settings}
}

func (p *GoogleOAuthSettingsProvider) OAuthSettings(ctx context.Context) (*OAuthSettings, error) {
	if p == nil || p.settings == nil {
		return nil, infraerrors.InternalServer("MAIL_OAUTH_CONFIG_NOT_CONFIGURED", "mail oauth config is not configured")
	}
	settings, err := p.settings.GetAllSettings(ctx)
	if err != nil {
		return nil, err
	}
	return &OAuthSettings{
		Provider:               ProviderGmail,
		Enabled:                strings.TrimSpace(settings.GoogleOAuthClientID) != "" && settings.GoogleOAuthClientSecretConfigured,
		ClientID:               strings.TrimSpace(settings.GoogleOAuthClientID),
		ClientSecretConfigured: settings.GoogleOAuthClientSecretConfigured,
		RedirectURI:            strings.TrimSpace(settings.GoogleOAuthRedirectURL),
		FrontendRedirectURI:    strings.TrimSpace(settings.GoogleOAuthFrontendRedirectURL),
	}, nil
}

func (p *GoogleOAuthSettingsProvider) GoogleOAuthClient(ctx context.Context) (OAuthClientConfig, error) {
	if p == nil || p.settings == nil {
		return OAuthClientConfig{}, infraerrors.InternalServer("MAIL_OAUTH_CONFIG_NOT_CONFIGURED", "mail oauth config is not configured")
	}
	settings, err := p.settings.GetAllSettings(ctx)
	if err != nil {
		return OAuthClientConfig{}, err
	}
	cfg := OAuthClientConfig{
		ClientID:     strings.TrimSpace(settings.GoogleOAuthClientID),
		ClientSecret: strings.TrimSpace(settings.GoogleOAuthClientSecret),
		AuthorizeURL: defaultGoogleAuthorizeURL,
		TokenURL:     defaultGoogleTokenURL,
	}
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		return OAuthClientConfig{}, infraerrors.InternalServer("MAIL_OAUTH_CONFIG_INVALID", "google oauth client id or secret is not configured")
	}
	return cfg, nil
}
