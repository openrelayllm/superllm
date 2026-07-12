package adminplus

import (
	newapiprovider "github.com/Wei-Shaw/sub2api/internal/adminplus/adapters/newapi/provider"
	providerrouter "github.com/Wei-Shaw/sub2api/internal/adminplus/adapters/providerrouter"
	sub2apiprovider "github.com/Wei-Shaw/sub2api/internal/adminplus/adapters/sub2api/provider"
	accountratesyncapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/accountratesync"
	actionsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/actions"
	balancesapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/balances"
	bizlogsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/bizlogs"
	channelchecksapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/channelchecks"
	costsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/costs"
	extensionapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/extension"
	healthapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/health"
	importexportapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/importexport"
	notificationsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/notifications"
	provisionjobsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/provisionjobs"
	purityapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/purity"
	ratesapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/rates"
	schedulerapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/scheduler"
	sessionsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/sessions"
	sitediscoveryapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/sitediscovery"
	sub2apiapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/sub2api"
	suppliergroupsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/suppliergroups"
	supplierkeysapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/supplierkeys"
	suppliersapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/suppliers"
	usagecostsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/usagecosts"
	"github.com/Wei-Shaw/sub2api/internal/adminplus/ports"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/wire"
)

func ProvideBusinessLogRecorder(repo service.OpsRepository) *bizlogsapp.Recorder {
	return bizlogsapp.NewRecorder(repo)
}

var ProviderSet = wire.NewSet(
	sub2apiprovider.ProvideHTTPClient,
	sub2apiprovider.NewSessionProfileClient,
	newapiprovider.NewClient,
	providerrouter.New,
	ProvideBusinessLogRecorder,
	wire.Bind(new(ports.SessionProbeAdapter), new(*providerrouter.Router)),
	wire.Bind(new(ports.SessionLoginAdapter), new(*providerrouter.Router)),
	wire.Bind(new(ports.DirectRegistrationAdapter), new(*providerrouter.Router)),
	wire.Bind(new(ports.SessionChannelMonitorAdapter), new(*providerrouter.Router)),
	wire.Bind(new(ports.SessionGroupAdapter), new(*providerrouter.Router)),
	wire.Bind(new(ports.SessionRateAdapter), new(*providerrouter.Router)),
	wire.Bind(new(ports.SessionUsageCostAdapter), new(*providerrouter.Router)),
	wire.Bind(new(ports.SessionFundingAdapter), new(*providerrouter.Router)),
	wire.Bind(new(ports.SessionEntitlementAdapter), new(*providerrouter.Router)),
	wire.Bind(new(ports.SessionKeyAdapter), new(*providerrouter.Router)),
	actionsapp.ProviderSet,
	accountratesyncapp.ProviderSet,
	balancesapp.ProviderSet,
	channelchecksapp.ProviderSet,
	usagecostsapp.ProviderSet,
	costsapp.ProviderSet,
	extensionapp.ProviderSet,
	wire.Bind(new(extensionapp.RegistrationResultProcessor), new(*sitediscoveryapp.RegistrationProcessor)),
	wire.Bind(new(extensionapp.BrowserCredentialProvider), new(*suppliersapp.Service)),
	wire.Bind(new(balancesapp.SessionReader), new(*sessionsapp.Service)),
	wire.Bind(new(balancesapp.RecentSupplierUsageReader), new(*sub2apiapp.SQLRepository)),
	wire.Bind(new(sessionsapp.SupplierLookup), new(*suppliersapp.Service)),
	wire.Bind(new(costsapp.SessionReader), new(*sessionsapp.Service)),
	wire.Bind(new(costsapp.UsageCostSyncer), new(*usagecostsapp.Service)),
	wire.Bind(new(costsapp.BalanceSyncer), new(*balancesapp.Service)),
	wire.Bind(new(costsapp.SupplierLookup), new(*suppliersapp.Service)),
	wire.Bind(new(actionsapp.SupplierStatusUpdater), new(*suppliersapp.Service)),
	wire.Bind(new(actionsapp.NotificationDispatcher), new(*notificationsapp.Service)),
	wire.Bind(new(ratesapp.SessionReader), new(*sessionsapp.Service)),
	wire.Bind(new(usagecostsapp.SessionReader), new(*sessionsapp.Service)),
	wire.Bind(new(provisionjobsapp.GroupSyncer), new(*suppliergroupsapp.Service)),
	wire.Bind(new(provisionjobsapp.KeyProvisioner), new(*supplierkeysapp.Service)),
	wire.Bind(new(provisionjobsapp.CostSyncer), new(*costsapp.Service)),
	wire.Bind(new(provisionjobsapp.ChannelChecker), new(*channelchecksapp.Service)),
	wire.Bind(new(schedulerapp.GroupSyncer), new(*suppliergroupsapp.Service)),
	wire.Bind(new(schedulerapp.RateSyncer), new(*ratesapp.Service)),
	wire.Bind(new(schedulerapp.BalanceSyncer), new(*balancesapp.Service)),
	wire.Bind(new(schedulerapp.HealthSyncer), new(*healthapp.Service)),
	wire.Bind(new(schedulerapp.UsageCostSyncer), new(*usagecostsapp.Service)),
	wire.Bind(new(schedulerapp.CostSyncer), new(*costsapp.Service)),
	wire.Bind(new(schedulerapp.ChannelChecker), new(*channelchecksapp.Service)),
	wire.Bind(new(schedulerapp.PurityChecker), new(*purityapp.Service)),
	wire.Bind(new(schedulerapp.SessionRefresher), new(*sessionsapp.Service)),
	wire.Bind(new(schedulerapp.CandidateSummaryReader), new(*sub2apiapp.Service)),
	wire.Bind(new(schedulerapp.RoutingRefiller), new(*sub2apiapp.Service)),
	wire.Bind(new(schedulerapp.ActionRecommendationSyncer), new(*actionsapp.Service)),
	wire.Bind(new(suppliergroupsapp.SessionReader), new(*sessionsapp.Service)),
	wire.Bind(new(supplierkeysapp.SessionReader), new(*sessionsapp.Service)),
	wire.Bind(new(channelchecksapp.LocalBindingEnsurer), new(*supplierkeysapp.Service)),
	healthapp.ProviderSet,
	importexportapp.ProviderSet,
	notificationsapp.ProviderSet,
	purityapp.ProviderSet,
	ratesapp.ProviderSet,
	provisionjobsapp.ProviderSet,
	schedulerapp.ProviderSet,
	sessionsapp.ProviderSet,
	sitediscoveryapp.ProviderSet,
	sub2apiapp.ProviderSet,
	suppliergroupsapp.ProviderSet,
	supplierkeysapp.ProviderSet,
	suppliersapp.ProviderSet,
)
