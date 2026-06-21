package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	adminplushandler "github.com/Wei-Shaw/sub2api/internal/handler/adminplus"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/google/wire"
)

// ProvideAdminHandlers creates the AdminHandlers struct
func ProvideAdminHandlers(
	dashboardHandler *admin.DashboardHandler,
	groupHandler *admin.GroupHandler,
	settingHandler *admin.SettingHandler,
	opsHandler *admin.OpsHandler,
	systemHandler *admin.SystemHandler,
) *AdminHandlers {
	return &AdminHandlers{
		Dashboard: dashboardHandler,
		Group:     groupHandler,
		Setting:   settingHandler,
		Ops:       opsHandler,
		System:    systemHandler,
	}
}

// ProvideAdminPlusHandlers creates the Admin Plus business handler collection.
func ProvideAdminPlusHandlers(
	supplierHandler *adminplushandler.SupplierHandler,
	supplierGroupHandler *adminplushandler.SupplierGroupHandler,
	supplierKeyHandler *adminplushandler.SupplierKeyHandler,
	provisionJobHandler *adminplushandler.ProvisionJobHandler,
	rateHandler *adminplushandler.RateHandler,
	balanceHandler *adminplushandler.BalanceHandler,
	announcementHandler *adminplushandler.AnnouncementHandler,
	healthHandler *adminplushandler.HealthHandler,
	notificationHandler *adminplushandler.NotificationHandler,
	usageCostHandler *adminplushandler.UsageCostHandler,
	costHandler *adminplushandler.CostHandler,
	extensionHandler *adminplushandler.ExtensionHandler,
	sessionHandler *adminplushandler.SessionHandler,
	schedulerHandler *adminplushandler.SchedulerHandler,
	actionHandler *adminplushandler.ActionHandler,
	sub2apiHandler *adminplushandler.Sub2APIHandler,
) *AdminPlusHandlers {
	return &AdminPlusHandlers{
		Supplier:      supplierHandler,
		SupplierGroup: supplierGroupHandler,
		SupplierKey:   supplierKeyHandler,
		ProvisionJob:  provisionJobHandler,
		Rate:          rateHandler,
		Balance:       balanceHandler,
		Announcement:  announcementHandler,
		Health:        healthHandler,
		Notification:  notificationHandler,
		UsageCost:     usageCostHandler,
		Cost:          costHandler,
		Extension:     extensionHandler,
		Session:       sessionHandler,
		Scheduler:     schedulerHandler,
		Action:        actionHandler,
		Sub2API:       sub2apiHandler,
	}
}

// ProvideSystemHandler creates admin.SystemHandler with UpdateService
func ProvideSystemHandler(updateService *service.UpdateService, lockService *service.SystemOperationLockService) *admin.SystemHandler {
	return admin.NewSystemHandler(updateService, lockService)
}

// ProvideSettingHandler creates SettingHandler with version from BuildInfo
func ProvideSettingHandler(settingService *service.SettingService, buildInfo BuildInfo, notificationEmailService *service.NotificationEmailService) *SettingHandler {
	h := NewSettingHandler(settingService, buildInfo.Version)
	h.SetNotificationEmailService(notificationEmailService)
	return h
}

// ProvideAdminSettingHandler creates the Admin Plus settings handler.
func ProvideAdminSettingHandler(settingService *service.SettingService, opsService *service.OpsService) *admin.SettingHandler {
	return admin.NewSettingHandler(settingService, opsService)
}

// ProvideAdminGroupHandler creates the Admin Plus read-only group handler.
func ProvideAdminGroupHandler(groupService *service.GroupService) *admin.GroupHandler {
	return admin.NewGroupHandler(groupService)
}

// ProvideHandlers creates the Handlers struct
func ProvideHandlers(
	authHandler *AuthHandler,
	adminHandlers *AdminHandlers,
	adminPlusHandlers *AdminPlusHandlers,
	settingHandler *SettingHandler,
) *Handlers {
	return &Handlers{
		Auth:      authHandler,
		Admin:     adminHandlers,
		AdminPlus: adminPlusHandlers,
		Setting:   settingHandler,
	}
}

// ProviderSet is the Wire provider set for all handlers
var ProviderSet = wire.NewSet(
	// Top-level handlers
	NewAuthHandler,
	ProvideSettingHandler,

	// Admin handlers
	admin.NewDashboardHandler,
	ProvideAdminGroupHandler,
	ProvideAdminSettingHandler,
	admin.NewOpsHandler,
	ProvideSystemHandler,
	adminplushandler.NewSupplierHandler,
	adminplushandler.NewSupplierGroupHandlerWithProvisionJobs,
	adminplushandler.NewSupplierKeyHandlerWithProvisionJobs,
	adminplushandler.NewProvisionJobHandler,
	adminplushandler.NewRateHandler,
	adminplushandler.NewBalanceHandler,
	adminplushandler.NewAnnouncementHandler,
	adminplushandler.NewHealthHandler,
	adminplushandler.NewNotificationHandler,
	adminplushandler.NewUsageCostHandler,
	adminplushandler.NewCostHandlerWithProvisionJobs,
	adminplushandler.NewExtensionHandler,
	adminplushandler.NewSessionHandler,
	adminplushandler.NewSchedulerHandler,
	adminplushandler.NewActionHandler,
	adminplushandler.NewSub2APIHandlerWithAccountTest,

	// AdminHandlers and Handlers constructors
	ProvideAdminHandlers,
	ProvideAdminPlusHandlers,
	ProvideHandlers,
)
