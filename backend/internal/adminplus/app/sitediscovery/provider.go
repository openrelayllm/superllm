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

func UseRegistrationLogReader(opsService *service.OpsService) RegistrationLogReader {
	return opsService
}

func ProvideService(repo Repository, suppliers *suppliersapp.Service, extension *extensionapp.Service, directRegistration ports.DirectRegistrationAdapter, cipher CredentialCipher, client *http.Client, recorder *bizlogs.Recorder, logs RegistrationLogReader) *Service {
	return NewService(repo, suppliers, extension, cipher, client).WithDirectRegistration(directRegistration).WithDiagnostics(recorder).WithRegistrationLogs(logs)
}

func ProvideRegistrationProcessor(repo Repository, suppliers *suppliersapp.Service, cipher CredentialCipher, recorder *bizlogs.Recorder) *RegistrationProcessor {
	return NewRegistrationProcessor(repo, suppliers, cipher).WithDiagnostics(recorder)
}

var ProviderSet = wire.NewSet(
	UseCredentialCipher,
	UseRegistrationLogReader,
	NewSQLRepository,
	wire.Bind(new(Repository), new(*SQLRepository)),
	ProvideRegistrationProcessor,
	ProvideService,
)
