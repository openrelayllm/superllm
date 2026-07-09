package actions

import "github.com/google/wire"

func ProvideService(repo Repository, supplierUpdater SupplierStatusUpdater, notifier NotificationDispatcher) *Service {
	return NewServiceWithDependencies(repo, supplierUpdater, notifier)
}

var ProviderSet = wire.NewSet(
	NewSQLRepository,
	wire.Bind(new(Repository), new(*SQLRepository)),
	ProvideService,
)
