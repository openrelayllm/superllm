package reconciliation

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
	NewFeishuNotifierFromEnv,
	wire.Bind(new(Notifier), new(*FeishuNotifier)),
	NewServiceWithNotifier,
)
