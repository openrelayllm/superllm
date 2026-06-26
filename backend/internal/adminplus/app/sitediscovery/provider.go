package sitediscovery

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/adminplus/app/bizlogs"
	extensionapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/extension"
	suppliersapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/suppliers"
	"github.com/Wei-Shaw/sub2api/internal/adminplus/ports"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/wire"
)

func UseCredentialCipher(encryptor service.SecretEncryptor) CredentialCipher {
	return encryptor
}

func ProvideService(repo Repository, suppliers *suppliersapp.Service, extension *extensionapp.Service, mail RegistrationMailReader, directRegistration ports.DirectRegistrationAdapter, cipher CredentialCipher, client *http.Client, recorder *bizlogs.Recorder, logs RegistrationLogReader, proxyManager ProxyManager) *Service {
	return NewService(repo, suppliers, extension, mail, cipher, client).WithDirectRegistration(directRegistration).WithDiagnostics(recorder).WithRegistrationLogs(logs).WithProxyManager(proxyManager)
}

func ProvideRegistrationProcessor(repo Repository, suppliers *suppliersapp.Service, cipher CredentialCipher, recorder *bizlogs.Recorder, proxyManager ProxyManager) *RegistrationProcessor {
	return NewRegistrationProcessor(repo, suppliers, cipher).WithDiagnostics(recorder).WithProxyManager(proxyManager)
}

var ProviderSet = wire.NewSet(
	UseCredentialCipher,
	NewSQLRepository,
	wire.Bind(new(Repository), new(*SQLRepository)),
	ProvideRegistrationProcessor,
	ProvideService,
)
