package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	ratesapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/rates"
	suppliersapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/suppliers"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	adminplushandler "github.com/Wei-Shaw/sub2api/internal/handler/adminplus"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newAdminPlusSurfaceRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	v1 := router.Group("/api/v1")
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
			Supplier: adminplushandler.NewSupplierHandler(suppliersapp.NewService(suppliersapp.NewMemoryRepository())),
			Rate:     adminplushandler.NewRateHandler(ratesapp.NewService(newRouteSurfaceRateRepository())),
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
		"GET /api/v1/admin-plus/suppliers/:id",
		"PATCH /api/v1/admin-plus/suppliers/:id/status",
		"POST /api/v1/admin-plus/rates/snapshots",
		"GET /api/v1/admin-plus/rates/snapshots",
		"GET /api/v1/admin-plus/rates/events",
		"PATCH /api/v1/admin-plus/rates/events/:id/ack",
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
