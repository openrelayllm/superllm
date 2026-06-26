package extension

import (
	"github.com/Wei-Shaw/sub2api/internal/adminplus/app/bizlogs"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/wire"
)

func UseSessionCipher(encryptor service.SecretEncryptor) SessionCipher {
	return encryptor
}

func ProvideService(repo Repository, processor ResultProcessor, credentials BrowserCredentialProvider, recorder *bizlogs.Recorder) *Service {
	return NewServiceWithDependencies(repo, processor, credentials).WithDiagnostics(recorder)
}

var ProviderSet = wire.NewSet(
	UseSessionCipher,
	NewSQLRepository,
	wire.Bind(new(Repository), new(*SQLRepository)),
	NewIngestProcessorWithCipher,
	wire.Bind(new(ResultProcessor), new(*IngestProcessor)),
	ProvideService,
)
