package notifications

import (
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/wire"
)

func UseSecretCipher(encryptor service.SecretEncryptor) SecretCipher {
	return encryptor
}

var ProviderSet = wire.NewSet(
	NewSQLRepository,
	UseSecretCipher,
	NewServiceWithCipher,
	wire.Bind(new(Repository), new(*SQLRepository)),
)
