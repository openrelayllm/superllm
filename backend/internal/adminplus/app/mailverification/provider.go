package mailverification

import (
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/wire"
)

func UseSecretCipher(encryptor service.SecretEncryptor) SecretCipher {
	return encryptor
}

var ProviderSet = wire.NewSet(
	UseSecretCipher,
	NewSQLRepositoryWithCipher,
	NewGoogleOAuthSettingsProvider,
	NewGmailProvider,
	NewDefaultTemplateClassifier,
	wire.Bind(new(Repository), new(*SQLRepository)),
	wire.Bind(new(OAuthConfigProvider), new(*GoogleOAuthSettingsProvider)),
	wire.Bind(new(TemplateClassifier), new(*DefaultTemplateClassifier)),
	NewService,
)
