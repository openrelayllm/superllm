package adminplus

import (
	sub2apiprovider "github.com/Wei-Shaw/sub2api/internal/adminplus/adapters/sub2api/provider"
	actionsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/actions"
	balancesapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/balances"
	billingapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/billing"
	extensionapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/extension"
	healthapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/health"
	notificationsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/notifications"
	promotionsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/promotions"
	ratesapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/rates"
	reconciliationapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/reconciliation"
	schedulerapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/scheduler"
	sessionsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/sessions"
	sub2apiapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/sub2api"
	suppliergroupsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/suppliergroups"
	supplierkeysapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/supplierkeys"
	suppliersapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/suppliers"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	sub2apiprovider.ProviderSet,
	actionsapp.ProviderSet,
	balancesapp.ProviderSet,
	billingapp.ProviderSet,
	extensionapp.ProviderSet,
	wire.Bind(new(extensionapp.BrowserCredentialProvider), new(*suppliersapp.Service)),
	wire.Bind(new(sessionsapp.SupplierLookup), new(*suppliersapp.Service)),
	wire.Bind(new(ratesapp.SessionReader), new(*sessionsapp.Service)),
	wire.Bind(new(suppliergroupsapp.SessionReader), new(*sessionsapp.Service)),
	wire.Bind(new(supplierkeysapp.SessionReader), new(*sessionsapp.Service)),
	healthapp.ProviderSet,
	notificationsapp.ProviderSet,
	promotionsapp.ProviderSet,
	ratesapp.ProviderSet,
	reconciliationapp.ProviderSet,
	schedulerapp.ProviderSet,
	sessionsapp.ProviderSet,
	sub2apiapp.ProviderSet,
	suppliergroupsapp.ProviderSet,
	supplierkeysapp.ProviderSet,
	suppliersapp.ProviderSet,
)
