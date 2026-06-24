package announcements

import (
	sessionsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/sessions"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	NewSQLRepository,
	NewFeishuNotifier,
	wire.Bind(new(Repository), new(*SQLRepository)),
	wire.Bind(new(Notifier), new(*FeishuNotifier)),
	wire.Bind(new(SessionReader), new(*sessionsapp.Service)),
	NewServiceWithDependencies,
)
