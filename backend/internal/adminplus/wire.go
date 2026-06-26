package adminplus

import (
	newapiprovider "github.com/Wei-Shaw/sub2api/internal/adminplus/adapters/newapi/provider"
	providerrouter "github.com/Wei-Shaw/sub2api/internal/adminplus/adapters/providerrouter"
	sub2apiprovider "github.com/Wei-Shaw/sub2api/internal/adminplus/adapters/sub2api/provider"
	actionsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/actions"
	announcementsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/announcements"
	balancesapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/balances"
	bizlogsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/bizlogs"
	channelchecksapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/channelchecks"
	costsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/costs"
	extensionapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/extension"
	healthapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/health"
	mailverificationapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/mailverification"
	notificationsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/notifications"
	provisionjobsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/provisionjobs"
	proxyapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/proxy"
	ratesapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/rates"
	schedulerapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/scheduler"
	sessionsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/sessions"
	sitecatalogapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/sitecatalog"
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
	wire.Bind(new(mailverificationapp.EmailSender), new(*service.EmailService)),
	wire.Bind(new(ports.SessionProbeAdapter), new(*providerrouter.Router)),
	wire.Bind(new(ports.SessionLoginAdapter), new(*providerrouter.Router)),
	wire.Bind(new(ports.DirectRegistrationAdapter), new(*providerrouter.Router)),
	wire.Bind(new(ports.SessionChannelMonitorAdapter), new(*providerrouter.Router)),
	wire.Bind(new(ports.SessionGroupAdapter), new(*providerrouter.Router)),
	wire.Bind(new(ports.SessionRateAdapter), new(*providerrouter.Router)),
	wire.Bind(new(ports.SessionAnnouncementAdapter), new(*providerrouter.Router)),
	wire.Bind(new(ports.SessionUsageCostAdapter), new(*providerrouter.Router)),
	wire.Bind(new(ports.SessionFundingAdapter), new(*providerrouter.Router)),
	wire.Bind(new(ports.SessionEntitlementAdapter), new(*providerrouter.Router)),
	wire.Bind(new(ports.SessionKeyAdapter), new(*providerrouter.Router)),
	actionsapp.ProviderSet,
	balancesapp.ProviderSet,
	channelchecksapp.ProviderSet,
	usagecostsapp.ProviderSet,
	costsapp.ProviderSet,
	extensionapp.ProviderSet,
	wire.Bind(new(extensionapp.RegistrationResultProcessor), new(*sitediscoveryapp.RegistrationProcessor)),
	wire.Bind(new(sitediscoveryapp.ProxyManager), new(*proxyapp.Service)),
	wire.Bind(new(extensionapp.BrowserCredentialProvider), new(*suppliersapp.Service)),
	wire.Bind(new(balancesapp.SessionReader), new(*sessionsapp.Service)),
	wire.Bind(new(sessionsapp.SupplierLookup), new(*suppliersapp.Service)),
	wire.Bind(new(costsapp.SessionReader), new(*sessionsapp.Service)),
	wire.Bind(new(costsapp.UsageCostSyncer), new(*usagecostsapp.Service)),
	wire.Bind(new(costsapp.BalanceSyncer), new(*balancesapp.Service)),
	wire.Bind(new(costsapp.SupplierLookup), new(*suppliersapp.Service)),
	wire.Bind(new(announcementsapp.SessionReader), new(*sessionsapp.Service)),
	wire.Bind(new(ratesapp.SessionReader), new(*sessionsapp.Service)),
	wire.Bind(new(usagecostsapp.SessionReader), new(*sessionsapp.Service)),
	wire.Bind(new(provisionjobsapp.GroupSyncer), new(*suppliergroupsapp.Service)),
	wire.Bind(new(provisionjobsapp.KeyProvisioner), new(*supplierkeysapp.Service)),
	wire.Bind(new(provisionjobsapp.CostSyncer), new(*costsapp.Service)),
	wire.Bind(new(provisionjobsapp.ChannelChecker), new(*channelchecksapp.Service)),
	wire.Bind(new(schedulerapp.GroupSyncer), new(*suppliergroupsapp.Service)),
	wire.Bind(new(schedulerapp.RateSyncer), new(*ratesapp.Service)),
	wire.Bind(new(schedulerapp.BalanceSyncer), new(*balancesapp.Service)),
	wire.Bind(new(schedulerapp.AnnouncementSyncer), new(*announcementsapp.Service)),
	wire.Bind(new(schedulerapp.HealthSyncer), new(*healthapp.Service)),
	wire.Bind(new(schedulerapp.UsageCostSyncer), new(*usagecostsapp.Service)),
	wire.Bind(new(schedulerapp.ChannelChecker), new(*channelchecksapp.Service)),
	wire.Bind(new(schedulerapp.SessionRefresher), new(*sessionsapp.Service)),
	wire.Bind(new(suppliergroupsapp.SessionReader), new(*sessionsapp.Service)),
	wire.Bind(new(supplierkeysapp.SessionReader), new(*sessionsapp.Service)),
	wire.Bind(new(channelchecksapp.LocalBindingEnsurer), new(*supplierkeysapp.Service)),
	healthapp.ProviderSet,
	mailverificationapp.ProviderSet,
	notificationsapp.ProviderSet,
	announcementsapp.ProviderSet,
	ratesapp.ProviderSet,
	provisionjobsapp.ProviderSet,
	proxyapp.ProviderSet,
	schedulerapp.ProviderSet,
	sessionsapp.ProviderSet,
	sitecatalogapp.ProviderSet,
	sitediscoveryapp.ProviderSet,
	sub2apiapp.ProviderSet,
	suppliergroupsapp.ProviderSet,
	supplierkeysapp.ProviderSet,
	suppliersapp.ProviderSet,
)
