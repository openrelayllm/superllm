package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	adminplushandler "github.com/Wei-Shaw/sub2api/internal/handler/adminplus"
)

// AdminHandlers contains all admin-related HTTP handlers
type AdminHandlers struct {
	Dashboard *admin.DashboardHandler
	Group     *admin.GroupHandler
	Setting   *admin.SettingHandler
	Ops       *admin.OpsHandler
	System    *admin.SystemHandler
}

// AdminPlusHandlers contains Admin Plus business HTTP handlers.
type AdminPlusHandlers struct {
	Supplier         *adminplushandler.SupplierHandler
	SupplierGroup    *adminplushandler.SupplierGroupHandler
	SupplierKey      *adminplushandler.SupplierKeyHandler
	ProvisionJob     *adminplushandler.ProvisionJobHandler
	Rate             *adminplushandler.RateHandler
	Balance          *adminplushandler.BalanceHandler
	Announcement     *adminplushandler.AnnouncementHandler
	Health           *adminplushandler.HealthHandler
	Notification     *adminplushandler.NotificationHandler
	UsageCost        *adminplushandler.UsageCostHandler
	Cost             *adminplushandler.CostHandler
	ChannelCheck     *adminplushandler.ChannelCheckHandler
	Extension        *adminplushandler.ExtensionHandler
	SiteDiscovery    *adminplushandler.SiteDiscoveryHandler
	SiteCatalog      *adminplushandler.SiteCatalogHandler
	MailVerification *adminplushandler.MailVerificationHandler
	Session          *adminplushandler.SessionHandler
	Scheduler        *adminplushandler.SchedulerHandler
	Action           *adminplushandler.ActionHandler
	Sub2API          *adminplushandler.Sub2APIHandler
	Proxy            *adminplushandler.ProxyHandler
}

// Handlers contains all HTTP handlers
type Handlers struct {
	Auth      *AuthHandler
	Admin     *AdminHandlers
	AdminPlus *AdminPlusHandlers
	Setting   *SettingHandler
}

// BuildInfo contains build-time information
type BuildInfo struct {
	Version   string
	BuildType string // "source" for manual builds, "release" for CI builds
}
