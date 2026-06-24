package rates

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
	NewSQLRepository,
	NewFeishuNotifier,
	wire.Bind(new(Repository), new(*SQLRepository)),
	wire.Bind(new(Notifier), new(*FeishuNotifier)),
	NewServiceWithDependencies,
)
