// Package routes provides HTTP route registration and handlers.
package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterAdminRoutes 注册管理员路由
func RegisterAdminRoutes(
	v1 *gin.RouterGroup,
	h *handler.Handlers,
	adminAuth middleware.AdminAuthMiddleware,
) {
	admin := v1.Group("/admin")
	admin.Use(gin.HandlerFunc(adminAuth))
	{
		// 仪表盘
		registerDashboardRoutes(admin, h)

		// 运维监控筛选需要读取上游分组；MVP0 只保留只读列表。
		registerGroupRoutes(admin, h)

		// 系统设置
		registerSettingsRoutes(admin, h)

		// 运维监控（Ops）
		registerOpsRoutes(admin, h)

		// 系统管理
		registerSystemRoutes(admin, h)
	}
}

func registerOpsRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
	ops := admin.Group("/ops")
	{
		// Realtime ops signals
		ops.GET("/concurrency", h.Admin.Ops.GetConcurrencyStats)
		ops.GET("/user-concurrency", h.Admin.Ops.GetUserConcurrencyStats)
		ops.GET("/account-availability", h.Admin.Ops.GetAccountAvailability)
		ops.GET("/realtime-traffic", h.Admin.Ops.GetRealtimeTrafficSummary)

		// Alerts (rules + events)
		ops.GET("/alert-rules", h.Admin.Ops.ListAlertRules)
		ops.POST("/alert-rules", h.Admin.Ops.CreateAlertRule)
		ops.PUT("/alert-rules/:id", h.Admin.Ops.UpdateAlertRule)
		ops.DELETE("/alert-rules/:id", h.Admin.Ops.DeleteAlertRule)
		ops.GET("/alert-events", h.Admin.Ops.ListAlertEvents)
		ops.GET("/alert-events/:id", h.Admin.Ops.GetAlertEvent)
		ops.PUT("/alert-events/:id/status", h.Admin.Ops.UpdateAlertEventStatus)
		ops.POST("/alert-silences", h.Admin.Ops.CreateAlertSilence)

		// Email notification config (DB-backed)
		ops.GET("/email-notification/config", h.Admin.Ops.GetEmailNotificationConfig)
		ops.PUT("/email-notification/config", h.Admin.Ops.UpdateEmailNotificationConfig)

		// Runtime settings (DB-backed)
		runtime := ops.Group("/runtime")
		{
			runtime.GET("/alert", h.Admin.Ops.GetAlertRuntimeSettings)
			runtime.PUT("/alert", h.Admin.Ops.UpdateAlertRuntimeSettings)
			runtime.GET("/logging", h.Admin.Ops.GetRuntimeLogConfig)
			runtime.PUT("/logging", h.Admin.Ops.UpdateRuntimeLogConfig)
			runtime.POST("/logging/reset", h.Admin.Ops.ResetRuntimeLogConfig)
		}

		// Advanced settings (DB-backed)
		ops.GET("/advanced-settings", h.Admin.Ops.GetAdvancedSettings)
		ops.PUT("/advanced-settings", h.Admin.Ops.UpdateAdvancedSettings)

		// Settings group (DB-backed)
		settings := ops.Group("/settings")
		{
			settings.GET("/metric-thresholds", h.Admin.Ops.GetMetricThresholds)
			settings.PUT("/metric-thresholds", h.Admin.Ops.UpdateMetricThresholds)
		}

		// WebSocket realtime (QPS/TPS)
		ws := ops.Group("/ws")
		{
			ws.GET("/qps", h.Admin.Ops.QPSWSHandler)
		}

		// Error logs (legacy)
		ops.GET("/errors", h.Admin.Ops.GetErrorLogs)
		ops.GET("/errors/:id", h.Admin.Ops.GetErrorLogByID)
		ops.PUT("/errors/:id/resolve", h.Admin.Ops.UpdateErrorResolution)

		// Request errors (client-visible failures)
		ops.GET("/request-errors", h.Admin.Ops.ListRequestErrors)
		ops.GET("/request-errors/:id", h.Admin.Ops.GetRequestError)
		ops.GET("/request-errors/:id/upstream-errors", h.Admin.Ops.ListRequestErrorUpstreamErrors)
		ops.PUT("/request-errors/:id/resolve", h.Admin.Ops.ResolveRequestError)

		// Upstream errors (independent upstream failures)
		ops.GET("/upstream-errors", h.Admin.Ops.ListUpstreamErrors)
		ops.GET("/upstream-errors/:id", h.Admin.Ops.GetUpstreamError)
		ops.PUT("/upstream-errors/:id/resolve", h.Admin.Ops.ResolveUpstreamError)

		// Request drilldown (success + error)
		ops.GET("/requests", h.Admin.Ops.ListRequestDetails)

		// Indexed system logs
		ops.GET("/system-logs", h.Admin.Ops.ListSystemLogs)
		ops.POST("/system-logs/cleanup", h.Admin.Ops.CleanupSystemLogs)
		ops.GET("/system-logs/health", h.Admin.Ops.GetSystemLogIngestionHealth)

		// Dashboard (vNext - raw path for MVP)
		ops.GET("/dashboard/snapshot-v2", h.Admin.Ops.GetDashboardSnapshotV2)
		ops.GET("/dashboard/overview", h.Admin.Ops.GetDashboardOverview)
		ops.GET("/dashboard/throughput-trend", h.Admin.Ops.GetDashboardThroughputTrend)
		ops.GET("/dashboard/latency-histogram", h.Admin.Ops.GetDashboardLatencyHistogram)
		ops.GET("/dashboard/error-trend", h.Admin.Ops.GetDashboardErrorTrend)
		ops.GET("/dashboard/error-distribution", h.Admin.Ops.GetDashboardErrorDistribution)
		ops.GET("/dashboard/openai-token-stats", h.Admin.Ops.GetDashboardOpenAITokenStats)
	}
}

func registerDashboardRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
	dashboard := admin.Group("/dashboard")
	{
		dashboard.GET("/snapshot-v2", h.Admin.Dashboard.GetSnapshotV2)
		dashboard.GET("/stats", h.Admin.Dashboard.GetStats)
		dashboard.GET("/realtime", h.Admin.Dashboard.GetRealtimeMetrics)
		dashboard.GET("/trend", h.Admin.Dashboard.GetUsageTrend)
		dashboard.GET("/models", h.Admin.Dashboard.GetModelStats)
		dashboard.GET("/groups", h.Admin.Dashboard.GetGroupStats)
		dashboard.GET("/api-keys-trend", h.Admin.Dashboard.GetAPIKeyUsageTrend)
		dashboard.GET("/users-trend", h.Admin.Dashboard.GetUserUsageTrend)
		dashboard.GET("/users-ranking", h.Admin.Dashboard.GetUserSpendingRanking)
		dashboard.POST("/users-usage", h.Admin.Dashboard.GetBatchUsersUsage)
		dashboard.POST("/api-keys-usage", h.Admin.Dashboard.GetBatchAPIKeysUsage)
		dashboard.GET("/user-breakdown", h.Admin.Dashboard.GetUserBreakdown)
		dashboard.POST("/aggregation/backfill", h.Admin.Dashboard.BackfillAggregation)
	}
}

func registerGroupRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
	groups := admin.Group("/groups")
	{
		groups.GET("/all", h.Admin.Group.GetAll)
	}
}

func registerSettingsRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
	adminSettings := admin.Group("/settings")
	{
		adminSettings.GET("", h.Admin.Setting.GetSettings)
		adminSettings.PUT("", h.Admin.Setting.UpdateSettings)
	}
}

func registerSystemRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
	system := admin.Group("/system")
	{
		system.GET("/version", h.Admin.System.GetVersion)
		system.GET("/check-updates", h.Admin.System.CheckUpdates)
		system.POST("/update", h.Admin.System.PerformUpdate)
		system.POST("/rollback", h.Admin.System.Rollback)
		system.POST("/restart", h.Admin.System.RestartService)
	}
}
