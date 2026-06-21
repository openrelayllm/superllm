package provisionjobs

import (
	"github.com/Wei-Shaw/sub2api/internal/adminplus/app/costs"
	"github.com/Wei-Shaw/sub2api/internal/adminplus/app/suppliergroups"
	"github.com/Wei-Shaw/sub2api/internal/adminplus/app/supplierkeys"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	NewSQLRepository,
	NewStreamPublisher,
	wire.Bind(new(Repository), new(*SQLRepository)),
	wire.Bind(new(RedisPublisher), new(*StreamPublisher)),
	wire.Bind(new(GroupSyncer), new(*suppliergroups.Service)),
	wire.Bind(new(KeyProvisioner), new(*supplierkeys.Service)),
	wire.Bind(new(CostSyncer), new(*costs.Service)),
	NewServiceWithCostSyncer,
	ProvideWorker,
)
