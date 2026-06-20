package rates

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
	NewSQLRepository,
	NewFeishuNotifierFromEnv,
	wire.Bind(new(Repository), new(*SQLRepository)),
	wire.Bind(new(Notifier), new(*FeishuNotifier)),
	NewServiceWithNotifier,
)
