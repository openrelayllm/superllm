package adminplus

import (
	newapiprovider "github.com/Wei-Shaw/sub2api/internal/adminplus/adapters/newapi/provider"
	providerrouter "github.com/Wei-Shaw/sub2api/internal/adminplus/adapters/providerrouter"
	sub2apiprovider "github.com/Wei-Shaw/sub2api/internal/adminplus/adapters/sub2api/provider"
	actionsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/actions"
	announcementsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/announcements"
	balancesapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/balances"
	costsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/costs"
	extensionapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/extension"
	healthapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/health"
	notificationsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/notifications"
	provisionjobsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/provisionjobs"
	ratesapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/rates"
	schedulerapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/scheduler"
	sessionsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/sessions"
	sub2apiapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/sub2api"
	suppliergroupsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/suppliergroups"
	supplierkeysapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/supplierkeys"
	suppliersapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/suppliers"
	usagecostsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/usagecosts"
	"github.com/Wei-Shaw/sub2api/internal/adminplus/ports"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	sub2apiprovider.ProvideHTTPClient,
	sub2apiprovider.NewSessionProfileClient,
	newapiprovider.NewClient,
	providerrouter.New,
	wire.Bind(new(ports.SessionProbeAdapter), new(*providerrouter.Router)),
	wire.Bind(new(ports.SessionLoginAdapter), new(*providerrouter.Router)),
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
	usagecostsapp.ProviderSet,
	costsapp.ProviderSet,
	extensionapp.ProviderSet,
	wire.Bind(new(extensionapp.BrowserCredentialProvider), new(*suppliersapp.Service)),
	wire.Bind(new(sessionsapp.SupplierLookup), new(*suppliersapp.Service)),
	wire.Bind(new(costsapp.SessionReader), new(*sessionsapp.Service)),
	wire.Bind(new(ratesapp.SessionReader), new(*sessionsapp.Service)),
	wire.Bind(new(suppliergroupsapp.SessionReader), new(*sessionsapp.Service)),
	wire.Bind(new(supplierkeysapp.SessionReader), new(*sessionsapp.Service)),
	healthapp.ProviderSet,
	notificationsapp.ProviderSet,
	announcementsapp.ProviderSet,
	ratesapp.ProviderSet,
	provisionjobsapp.ProviderSet,
	schedulerapp.ProviderSet,
	sessionsapp.ProviderSet,
	sub2apiapp.ProviderSet,
	suppliergroupsapp.ProviderSet,
	supplierkeysapp.ProviderSet,
	suppliersapp.ProviderSet,
)
