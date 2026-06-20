package extension

import (
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/wire"
)

func UseSessionCipher(encryptor service.SecretEncryptor) SessionCipher {
	return encryptor
}

var ProviderSet = wire.NewSet(
	UseSessionCipher,
	NewSQLRepository,
	wire.Bind(new(Repository), new(*SQLRepository)),
	NewIngestProcessorWithCipher,
	wire.Bind(new(ResultProcessor), new(*IngestProcessor)),
	NewServiceWithDependencies,
)
