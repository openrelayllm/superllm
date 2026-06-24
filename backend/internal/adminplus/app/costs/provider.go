package costs

import (
	balancesapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/balances"
	sessionsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/sessions"
	usagecostsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/usagecosts"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	NewSQLRepository,
	NewFeishuNotifier,
	wire.Bind(new(Repository), new(*SQLRepository)),
	wire.Bind(new(Notifier), new(*FeishuNotifier)),
	wire.Bind(new(SessionReader), new(*sessionsapp.Service)),
	wire.Bind(new(UsageCostSyncer), new(*usagecostsapp.Service)),
	wire.Bind(new(BalanceSyncer), new(*balancesapp.Service)),
	NewServiceWithDependenciesAndNotifier,
)
