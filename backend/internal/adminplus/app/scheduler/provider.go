package scheduler

import (
	announcementsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/announcements"
	balancesapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/balances"
	healthapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/health"
	ratesapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/rates"
	suppliergroupsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/suppliergroups"
	usagecostsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/usagecosts"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	wire.Bind(new(GroupSyncer), new(*suppliergroupsapp.Service)),
	wire.Bind(new(RateSyncer), new(*ratesapp.Service)),
	wire.Bind(new(BalanceSyncer), new(*balancesapp.Service)),
	wire.Bind(new(AnnouncementSyncer), new(*announcementsapp.Service)),
	wire.Bind(new(HealthSyncer), new(*healthapp.Service)),
	wire.Bind(new(UsageCostSyncer), new(*usagecostsapp.Service)),
	NewServiceWithDependencies,
	ProvideWorker,
)
