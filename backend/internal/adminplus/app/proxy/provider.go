package proxy

import (
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/wire"
)

func UseSecretCipher(encryptor service.SecretEncryptor) SecretCipher {
	return encryptor
}

func ProvideRuntimeConfig(cfg *config.Config) RuntimeConfig {
	return RuntimeConfigFromConfig(cfg)
}

func ProvideRuntime(cfg RuntimeConfig) Runtime {
	return NewLocalMihomoRuntime(cfg)
}

var ProviderSet = wire.NewSet(
	NewSQLRepository,
	NewSubscriptionNormalizer,
	UseSecretCipher,
	ProvideRuntimeConfig,
	ProvideRuntime,
	NewService,
	wire.Bind(new(Repository), new(*SQLRepository)),
)
