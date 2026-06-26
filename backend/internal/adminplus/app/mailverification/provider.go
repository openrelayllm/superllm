package mailverification

import (
	"github.com/Wei-Shaw/sub2api/internal/adminplus/app/bizlogs"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/wire"
)

func UseSecretCipher(encryptor service.SecretEncryptor) SecretCipher {
	return encryptor
}

func ProvideService(repo Repository, gmailProvider *GmailProvider, classifier TemplateClassifier, email EmailSender, recorder *bizlogs.Recorder) *Service {
	return NewService(repo, gmailProvider, classifier, email).WithDiagnostics(recorder)
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
	ProvideService,
)
