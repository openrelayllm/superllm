package sessions

import (
	"github.com/Wei-Shaw/sub2api/internal/adminplus/app/bizlogs"
	"github.com/Wei-Shaw/sub2api/internal/adminplus/ports"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/wire"
)

func UseCipher(encryptor service.SecretEncryptor) Cipher {
	return encryptor
}

func ProvideService(repo Repository, cipher Cipher, suppliers SupplierLookup, prober ports.SessionProbeAdapter, monitors ports.SessionChannelMonitorAdapter, login ports.SessionLoginAdapter, recorder *bizlogs.Recorder) *Service {
	return NewServiceWithDependencies(repo, cipher, suppliers, prober, login).
		WithChannelMonitorReader(monitors).
		WithDiagnostics(recorder)
}

var ProviderSet = wire.NewSet(
	UseCipher,
	NewSQLRepository,
	wire.Bind(new(Repository), new(*SQLRepository)),
	ProvideService,
)
