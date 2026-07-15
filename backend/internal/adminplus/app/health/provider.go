package health

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
	NewSQLRepositoryWithReadDB,
	NewFeishuNotifier,
	wire.Bind(new(Repository), new(*SQLRepository)),
	wire.Bind(new(Notifier), new(*FeishuNotifier)),
	NewServiceWithNotifier,
)
