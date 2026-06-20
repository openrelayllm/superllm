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
	rateHandler *adminplushandler.RateHandler,
	balanceHandler *adminplushandler.BalanceHandler,
	promotionHandler *adminplushandler.PromotionHandler,
	healthHandler *adminplushandler.HealthHandler,
	billingHandler *adminplushandler.BillingHandler,
	extensionHandler *adminplushandler.ExtensionHandler,
	actionHandler *adminplushandler.ActionHandler,
	reconciliationHandler *adminplushandler.ReconciliationHandler,
) *AdminPlusHandlers {
	return &AdminPlusHandlers{
		Supplier:       supplierHandler,
		Rate:           rateHandler,
		Balance:        balanceHandler,
		Promotion:      promotionHandler,
		Health:         healthHandler,
		Billing:        billingHandler,
		Extension:      extensionHandler,
		Action:         actionHandler,
		Reconciliation: reconciliationHandler,
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
	adminplushandler.NewRateHandler,
	adminplushandler.NewBalanceHandler,
	adminplushandler.NewPromotionHandler,
	adminplushandler.NewHealthHandler,
	adminplushandler.NewBillingHandler,
	adminplushandler.NewExtensionHandler,
	adminplushandler.NewActionHandler,
	adminplushandler.NewReconciliationHandler,

	// AdminHandlers and Handlers constructors
	ProvideAdminHandlers,
	ProvideAdminPlusHandlers,
	ProvideHandlers,
)
