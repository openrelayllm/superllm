package provisionjobs

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
	NewSQLRepository,
	NewStreamPublisher,
	wire.Bind(new(Repository), new(*SQLRepository)),
	wire.Bind(new(RedisPublisher), new(*StreamPublisher)),
	NewServiceWithDependencies,
	ProvideWorker,
)
