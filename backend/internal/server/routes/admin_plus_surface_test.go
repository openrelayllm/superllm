package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	actionsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/actions"
	announcementsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/announcements"
	balancesapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/balances"
	costsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/costs"
	extensionapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/extension"
	healthapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/health"
	ratesapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/rates"
	schedulerapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/scheduler"
	sessionsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/sessions"
	sub2apiapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/sub2api"
	suppliergroupsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/suppliergroups"
	supplierkeysapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/supplierkeys"
	suppliersapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/suppliers"
	usagecostsapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/usagecosts"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/adminplus/ports"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	adminplushandler "github.com/Wei-Shaw/sub2api/internal/handler/adminplus"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newAdminPlusSurfaceRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	v1 := router.Group("/api/v1")
	supplierService := suppliersapp.NewService(suppliersapp.NewMemoryRepository())
	extensionService := extensionapp.NewService(extensionapp.NewMemoryRepository())
	sessionService := sessionsapp.NewServiceWithDependencies(
		sessionsapp.NewMemoryRepository(),
		routeSurfaceSessionCipher{},
		supplierService,
		&routeSurfaceProbeAdapter{},
		&routeSurfaceLoginAdapter{},
	)
	supplierGroupService := suppliergroupsapp.NewService(
		suppliergroupsapp.NewMemoryRepository(),
		sessionService,
		&routeSurfaceGroupReader{},
	)
	supplierKeyService := supplierkeysapp.NewService(
		supplierkeysapp.NewMemoryRepository(),
		sessionService,
		&routeSurfaceKeyAdapter{},
		&routeSurfaceLocalAccountCreator{},
	)
	handlers := &handler.Handlers{
		Auth:    &handler.AuthHandler{},
		Setting: &handler.SettingHandler{},
		Admin: &handler.AdminHandlers{
			Dashboard: &adminhandler.DashboardHandler{},
			Group:     &adminhandler.GroupHandler{},
			Setting:   &adminhandler.SettingHandler{},
			Ops:       &adminhandler.OpsHandler{},
			System:    &adminhandler.SystemHandler{},
		},
		AdminPlus: &handler.AdminPlusHandlers{
			Supplier:      adminplushandler.NewSupplierHandler(supplierService),
			SupplierGroup: adminplushandler.NewSupplierGroupHandler(supplierGroupService),
			SupplierKey:   adminplushandler.NewSupplierKeyHandler(supplierKeyService),
			Rate:          adminplushandler.NewRateHandler(ratesapp.NewServiceWithDependencies(newRouteSurfaceRateRepository(), nil, sessionService, &routeSurfaceRateReader{})),
			Balance:       adminplushandler.NewBalanceHandler(balancesapp.NewService(balancesapp.NewMemoryRepository())),
			Announcement:  adminplushandler.NewAnnouncementHandler(announcementsapp.NewService(announcementsapp.NewMemoryRepository())),
			Health:        adminplushandler.NewHealthHandler(healthapp.NewService(healthapp.NewMemoryRepository())),
			UsageCost:     adminplushandler.NewUsageCostHandler(usagecostsapp.NewServiceWithDependencies(usagecostsapp.NewMemoryRepository(), sessionService, &routeSurfaceUsageCostReader{})),
			Cost:          adminplushandler.NewCostHandler(costsapp.NewService(costsapp.NewMemoryRepository())),
			Extension:     adminplushandler.NewExtensionHandler(extensionService, nil),
			Session:       adminplushandler.NewSessionHandler(sessionService, nil),
			Scheduler:     adminplushandler.NewSchedulerHandler(schedulerapp.NewService(supplierService, extensionService)),
			Action:        adminplushandler.NewActionHandler(actionsapp.NewRuleService()),
			Sub2API:       adminplushandler.NewSub2APIHandler(sub2apiapp.NewService(newRouteSurfaceSub2APIRepository(), newRouteSurfaceSub2APIRuntimeReader())),
		},
	}

	RegisterAuthRoutes(
		v1,
		handlers,
		servermiddleware.JWTAuthMiddleware(func(c *gin.Context) { c.Next() }),
		nil,
		nil,
	)
	RegisterAdminRoutes(
		v1,
		handlers,
		servermiddleware.AdminAuthMiddleware(func(c *gin.Context) { c.Next() }),
	)
	RegisterAdminPlusRoutes(
		v1,
		handlers,
		servermiddleware.AdminAuthMiddleware(func(c *gin.Context) { c.Next() }),
	)

	return router
}

func TestAdminPlusCurrentRoutesAreMounted(t *testing.T) {
	router := newAdminPlusSurfaceRouter()
	routes := registeredRouteSet(router)

	currentRoutes := []string{
		"GET /api/v1/settings/public",
		"POST /api/v1/auth/login",
		"POST /api/v1/auth/login/2fa",
		"POST /api/v1/auth/refresh",
		"POST /api/v1/auth/logout",
		"GET /api/v1/auth/me",
		"GET /api/v1/admin/dashboard/snapshot-v2",
		"GET /api/v1/admin/groups/all",
		"GET /api/v1/admin/settings",
		"GET /api/v1/admin/ops/dashboard/snapshot-v2",
		"GET /api/v1/admin/system/version",
		"GET /api/v1/admin-plus/suppliers",
		"POST /api/v1/admin-plus/suppliers",
		"POST /api/v1/admin-plus/suppliers/site-match",
		"GET /api/v1/admin-plus/suppliers/:id",
		"PATCH /api/v1/admin-plus/suppliers/:id/status",
		"GET /api/v1/admin-plus/suppliers/:id/accounts",
		"POST /api/v1/admin-plus/suppliers/:id/accounts",
		"DELETE /api/v1/admin-plus/suppliers/:id/accounts/:accountID",
		"GET /api/v1/admin-plus/suppliers/:id/groups",
		"POST /api/v1/admin-plus/suppliers/:id/groups/sync",
		"GET /api/v1/admin-plus/suppliers/:id/keys",
		"POST /api/v1/admin-plus/suppliers/:id/keys/ensure-all",
		"POST /api/v1/admin-plus/suppliers/:id/keys/provision",
		"POST /api/v1/admin-plus/suppliers/:id/keys/:keyID/repair-binding",
		"POST /api/v1/admin-plus/suppliers/:id/rates/sync",
		"GET /api/v1/admin-plus/suppliers/:id/balance/current",
		"POST /api/v1/admin-plus/suppliers/:id/usage-costs/sync",
		"POST /api/v1/admin-plus/suppliers/:id/costs/sync",
		"GET /api/v1/admin-plus/suppliers/:id/costs/summary",
		"GET /api/v1/admin-plus/suppliers/:id/funding-transactions",
		"GET /api/v1/admin-plus/suppliers/:id/entitlement-transactions",
		"GET /api/v1/admin-plus/suppliers/:id/cost-ledger",
		"GET /api/v1/admin-plus/suppliers/:id/session",
		"POST /api/v1/admin-plus/suppliers/:id/session/login",
		"POST /api/v1/admin-plus/suppliers/:id/session/probe",
		"GET /api/v1/admin-plus/suppliers/:id/channel-monitors",
		"POST /api/v1/admin-plus/suppliers/:id/browser-sessions",
		"GET /api/v1/admin-plus/sub2api/accounts",
		"GET /api/v1/admin-plus/sub2api/accounts/:accountID/models",
		"POST /api/v1/admin-plus/sub2api/accounts/:accountID/test",
		"GET /api/v1/admin-plus/sub2api/account-runtime",
		"GET /api/v1/admin-plus/sub2api/account-usage-summary",
		"GET /api/v1/admin-plus/sub2api/usage-lines",
		"GET /api/v1/admin-plus/sub2api/usage-summary",
		"POST /api/v1/admin-plus/rates/snapshots",
		"GET /api/v1/admin-plus/rates/snapshots",
		"GET /api/v1/admin-plus/rates/events",
		"PATCH /api/v1/admin-plus/rates/events/:id/ack",
		"POST /api/v1/admin-plus/balances/snapshots",
		"GET /api/v1/admin-plus/balances/snapshots",
		"GET /api/v1/admin-plus/balances/events",
		"PATCH /api/v1/admin-plus/balances/events/:id/ack",
		"POST /api/v1/admin-plus/announcements",
		"GET /api/v1/admin-plus/announcements",
		"PATCH /api/v1/admin-plus/announcements/:id/ack",
		"POST /api/v1/admin-plus/health/samples",
		"GET /api/v1/admin-plus/health/samples",
		"GET /api/v1/admin-plus/health/events",
		"PATCH /api/v1/admin-plus/health/events/:id/ack",
		"POST /api/v1/admin-plus/usage-costs/lines/import",
		"GET /api/v1/admin-plus/usage-costs/lines",
		"GET /api/v1/admin-plus/costs/suppliers",
		"POST /api/v1/admin-plus/extension/tasks",
		"GET /api/v1/admin-plus/extension/tasks",
		"POST /api/v1/admin-plus/extension/tasks/claim",
		"POST /api/v1/admin-plus/extension/tasks/:id/heartbeat",
		"POST /api/v1/admin-plus/extension/tasks/:id/complete",
		"POST /api/v1/admin-plus/extension/tasks/:id/fail",
		"GET /api/v1/admin-plus/scheduler/status",
		"POST /api/v1/admin-plus/scheduler/run",
		"POST /api/v1/admin-plus/actions/generate",
		"GET /api/v1/admin-plus/actions/recommendations",
		"PATCH /api/v1/admin-plus/actions/recommendations/:id/status",
	}

	for _, route := range currentRoutes {
		require.Contains(t, routes, route, "current route should stay mounted")
	}
}

func TestAdminPlusDeadRoutesStayUnregistered(t *testing.T) {
	router := newAdminPlusSurfaceRouter()
	routes := registeredRouteSet(router)

	deadRoutes := []string{
		"POST /api/v1/auth/register",
		"POST /api/v1/auth/send-verify-code",
		"POST /api/v1/auth/forgot-password",
		"POST /api/v1/auth/reset-password",
		"POST /api/v1/auth/revoke-all-sessions",
		"GET /api/v1/admin/users",
		"GET /api/v1/admin/accounts",
		"GET /api/v1/admin/channels",
		"GET /api/v1/admin/groups",
		"GET /api/v1/admin/groups/:id",
		"GET /api/v1/admin/payment",
		"GET /api/v1/admin/subscriptions",
		"GET /api/v1/admin/redeem-codes",
		"POST /api/v1/admin-plus/promotions",
		"GET /api/v1/admin-plus/promotions",
		"PATCH /api/v1/admin-plus/promotions/:id/ack",
		"POST /api/v1/admin-plus/suppliers/:id/billing/sync",
		"POST /api/v1/admin-plus/billing/lines/import",
		"GET /api/v1/admin-plus/billing/lines",
		"POST /api/v1/admin-plus/reconciliation/run",
		"GET /v1/chat/completions",
		"POST /v1/chat/completions",
	}

	for _, route := range deadRoutes {
		require.NotContains(t, routes, route, "dead route must not be mounted")
	}
}

func TestAdminPlusDeadPathsReturn404(t *testing.T) {
	router := newAdminPlusSurfaceRouter()

	deadPaths := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/auth/register"},
		{http.MethodPost, "/api/v1/auth/send-verify-code"},
		{http.MethodPost, "/api/v1/auth/forgot-password"},
		{http.MethodPost, "/api/v1/auth/revoke-all-sessions"},
		{http.MethodGet, "/api/v1/admin/users"},
		{http.MethodGet, "/api/v1/admin/accounts"},
		{http.MethodGet, "/api/v1/admin/channels"},
		{http.MethodGet, "/api/v1/admin/groups"},
		{http.MethodGet, "/api/v1/admin/groups/1"},
		{http.MethodGet, "/api/v1/admin/payment"},
		{http.MethodGet, "/api/v1/admin/subscriptions"},
		{http.MethodPost, "/api/v1/admin-plus/promotions"},
		{http.MethodGet, "/api/v1/admin-plus/promotions"},
		{http.MethodPatch, "/api/v1/admin-plus/promotions/1/ack"},
		{http.MethodPost, "/v1/chat/completions"},
	}

	for _, deadPath := range deadPaths {
		req := httptest.NewRequest(deadPath.method, deadPath.path, nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusNotFound, w.Code, "%s %s", deadPath.method, deadPath.path)
	}
}

func registeredRouteSet(router *gin.Engine) map[string]struct{} {
	routes := router.Routes()
	out := make(map[string]struct{}, len(routes))
	for _, route := range routes {
		out[route.Method+" "+route.Path] = struct{}{}
	}
	return out
}

type routeSurfaceRateRepository struct{}

type routeSurfaceSessionCipher struct{}

func (routeSurfaceSessionCipher) Encrypt(plaintext string) (string, error)  { return plaintext, nil }
func (routeSurfaceSessionCipher) Decrypt(ciphertext string) (string, error) { return ciphertext, nil }

type routeSurfaceLoginAdapter struct{}

func (r *routeSurfaceLoginAdapter) DirectLogin(_ context.Context, in ports.DirectLoginInput) (*ports.DirectLoginResult, error) {
	return &ports.DirectLoginResult{
		SupplierID: in.SupplierID,
		Origin:     in.Origin,
		APIBaseURL: in.APIBaseURL,
		SessionBundle: map[string]any{
			"origin":         in.Origin,
			"api_base_url":   in.APIBaseURL,
			"access_token":   "route-surface-token",
			"session_source": "direct_login",
			"context": map[string]any{
				"api_base_url":   in.APIBaseURL,
				"login_method":   "direct_login",
				"session_source": "direct_login",
			},
			"tokens": map[string]any{"access_token": "route-surface-token"},
		},
		CapturedAt: time.Now().UTC(),
	}, nil
}

type routeSurfaceProbeAdapter struct{}

func (r *routeSurfaceProbeAdapter) ProbeSub2APIUserProfile(_ context.Context, in ports.SessionProbeInput) (*ports.SessionProbeResult, error) {
	return &ports.SessionProbeResult{
		SupplierID:   in.SupplierID,
		Status:       "valid",
		SystemType:   "sub2api",
		Origin:       in.Origin,
		APIBaseURL:   in.APIBaseURL,
		Capabilities: map[string]bool{"can_read_profile": true},
		ProbedAt:     time.Now().UTC(),
	}, nil
}

type routeSurfaceGroupReader struct{}

func (r *routeSurfaceGroupReader) ReadGroups(_ context.Context, in ports.SessionProbeInput) (*ports.ReadGroupsResult, error) {
	return &ports.ReadGroupsResult{SupplierID: in.SupplierID, SystemType: "sub2api"}, nil
}

type routeSurfaceRateReader struct{}

func (r *routeSurfaceRateReader) ReadRates(_ context.Context, in ports.SessionProbeInput) (*ports.ReadRatesResult, error) {
	return &ports.ReadRatesResult{SupplierID: in.SupplierID, SystemType: "sub2api"}, nil
}

type routeSurfaceUsageCostReader struct{}

func (r *routeSurfaceUsageCostReader) ReadUsageCosts(_ context.Context, in ports.SessionProbeInput, _ ports.ReadUsageCostsInput) (*ports.ReadUsageCostsResult, error) {
	return &ports.ReadUsageCostsResult{SupplierID: in.SupplierID, SystemType: "sub2api"}, nil
}

type routeSurfaceKeyAdapter struct{}

func (r *routeSurfaceKeyAdapter) CreateKey(_ context.Context, in ports.SessionProbeInput, request ports.CreateProviderKeyInput) (*ports.ProviderKeyResult, error) {
	return &ports.ProviderKeyResult{
		SupplierID:      in.SupplierID,
		ExternalGroupID: request.ExternalGroupID,
		ExternalKeyID:   "route-surface-key",
		Name:            request.Name,
		Secret:          "sk-route-surface-secret",
		Status:          "active",
		RawPayload:      map[string]any{},
	}, nil
}

type routeSurfaceLocalAccountCreator struct {
	groups []service.Group
}

func (r *routeSurfaceLocalAccountCreator) CreateAccount(_ context.Context, input *service.CreateAccountInput) (*service.Account, error) {
	return &service.Account{
		ID:       1,
		Name:     input.Name,
		Platform: input.Platform,
		Type:     input.Type,
		GroupIDs: append([]int64(nil), input.GroupIDs...),
	}, nil
}

func (r *routeSurfaceLocalAccountCreator) GetAccount(_ context.Context, id int64) (*service.Account, error) {
	return &service.Account{
		ID:       id,
		Name:     "route-surface-local",
		Platform: service.PlatformOpenAI,
		Type:     service.AccountTypeAPIKey,
	}, nil
}

func (r *routeSurfaceLocalAccountCreator) UpdateAccount(_ context.Context, id int64, input *service.UpdateAccountInput) (*service.Account, error) {
	account := &service.Account{
		ID:       id,
		Name:     "route-surface-local",
		Platform: service.PlatformOpenAI,
		Type:     service.AccountTypeAPIKey,
	}
	if input.GroupIDs != nil {
		account.GroupIDs = append([]int64(nil), (*input.GroupIDs)...)
	}
	return account, nil
}

func (r *routeSurfaceLocalAccountCreator) CreateGroup(_ context.Context, input *service.CreateGroupInput) (*service.Group, error) {
	group := service.Group{
		ID:             int64(1 + len(r.groups)),
		Name:           input.Name,
		Platform:       input.Platform,
		RateMultiplier: input.RateMultiplier,
		Status:         "active",
	}
	r.groups = append(r.groups, group)
	return &group, nil
}

func (r *routeSurfaceLocalAccountCreator) GetAllGroupsIncludingInactive(_ context.Context) ([]service.Group, error) {
	out := make([]service.Group, len(r.groups))
	copy(out, r.groups)
	return out, nil
}

func newRouteSurfaceRateRepository() *routeSurfaceRateRepository {
	return &routeSurfaceRateRepository{}
}

func (r *routeSurfaceRateRepository) CreateSnapshot(_ context.Context, snapshot *adminplusdomain.RateSnapshot) (*adminplusdomain.RateSnapshot, error) {
	return snapshot, nil
}

func (r *routeSurfaceRateRepository) FindLatestComparableSnapshot(_ context.Context, _ *adminplusdomain.RateSnapshot) (*adminplusdomain.RateSnapshot, error) {
	return nil, nil
}

func (r *routeSurfaceRateRepository) CreateChangeEvent(_ context.Context, event *adminplusdomain.RateChangeEvent) (*adminplusdomain.RateChangeEvent, error) {
	return event, nil
}

func (r *routeSurfaceRateRepository) ListSnapshots(_ context.Context, _ ratesapp.SnapshotFilter) ([]*adminplusdomain.RateSnapshot, error) {
	return nil, nil
}

func (r *routeSurfaceRateRepository) ListChangeEvents(_ context.Context, _ ratesapp.EventFilter) ([]*adminplusdomain.RateChangeEvent, error) {
	return nil, nil
}

func (r *routeSurfaceRateRepository) UpdateChangeEventStatus(_ context.Context, _ int64, _ adminplusdomain.RateChangeStatus) (*adminplusdomain.RateChangeEvent, error) {
	return nil, nil
}

type routeSurfaceSub2APIRepository struct{}

func newRouteSurfaceSub2APIRepository() *routeSurfaceSub2APIRepository {
	return &routeSurfaceSub2APIRepository{}
}

func (r *routeSurfaceSub2APIRepository) ListLocalUsageLines(_ context.Context, _ sub2apiapp.UsageFilter) ([]*adminplusdomain.LocalUsageLine, error) {
	return nil, nil
}

func (r *routeSurfaceSub2APIRepository) ListLocalUsageSummaries(_ context.Context, _ sub2apiapp.UsageFilter) ([]*adminplusdomain.LocalUsageSummary, error) {
	return nil, nil
}

func (r *routeSurfaceSub2APIRepository) ListLocalAccountUsageSummaries(_ context.Context, _ sub2apiapp.UsageFilter) ([]*adminplusdomain.LocalAccountUsageSummary, error) {
	return nil, nil
}

type routeSurfaceSub2APIRuntimeReader struct{}

func newRouteSurfaceSub2APIRuntimeReader() *routeSurfaceSub2APIRuntimeReader {
	return &routeSurfaceSub2APIRuntimeReader{}
}

func (r *routeSurfaceSub2APIRuntimeReader) ListAccountRuntime(_ context.Context, _ sub2apiapp.RuntimeFilter) ([]*adminplusdomain.LocalAccountRuntime, error) {
	return nil, nil
}
