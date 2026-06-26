package balances

import (
	"github.com/Wei-Shaw/sub2api/internal/adminplus/app/bizlogs"
	"github.com/Wei-Shaw/sub2api/internal/adminplus/ports"
	"github.com/google/wire"
)

func ProvideService(repo Repository, notifier Notifier, session SessionReader, reader ports.SessionProbeAdapter, cache BalanceCache, recorder *bizlogs.Recorder) *Service {
	return NewServiceWithCurrentCache(repo, notifier, session, reader, cache).WithDiagnostics(recorder)
}

var ProviderSet = wire.NewSet(
	NewSQLRepository,
	NewFeishuNotifier,
	wire.Bind(new(Repository), new(*SQLRepository)),
	wire.Bind(new(Notifier), new(*FeishuNotifier)),
	ProvideService,
)
