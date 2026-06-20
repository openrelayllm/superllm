package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

func RegisterAdminPlusRoutes(
	v1 *gin.RouterGroup,
	h *handler.Handlers,
	adminAuth middleware.AdminAuthMiddleware,
) {
	adminPlus := v1.Group("/admin-plus")
	adminPlus.Use(gin.HandlerFunc(adminAuth))
	{
		suppliers := adminPlus.Group("/suppliers")
		{
			suppliers.GET("", h.AdminPlus.Supplier.List)
			suppliers.POST("", h.AdminPlus.Supplier.Create)
			suppliers.POST("/site-match", h.AdminPlus.Supplier.MatchSite)
			suppliers.POST("/from-site-candidate", h.AdminPlus.Supplier.CreateFromSiteCandidate)
			suppliers.GET("/:id", h.AdminPlus.Supplier.Get)
			suppliers.PUT("/:id", h.AdminPlus.Supplier.Update)
			suppliers.DELETE("/:id", h.AdminPlus.Supplier.Delete)
			suppliers.PATCH("/:id/status", h.AdminPlus.Supplier.UpdateStatus)
			suppliers.GET("/:id/accounts", h.AdminPlus.Supplier.ListAccounts)
			suppliers.POST("/:id/accounts", h.AdminPlus.Supplier.CreateAccount)
			suppliers.PUT("/:id/accounts/:accountID", h.AdminPlus.Supplier.UpdateAccount)
			suppliers.DELETE("/:id/accounts/:accountID", h.AdminPlus.Supplier.DeleteAccount)
			suppliers.GET("/:id/groups", h.AdminPlus.SupplierGroup.List)
			suppliers.POST("/:id/groups/sync", h.AdminPlus.SupplierGroup.Sync)
			suppliers.GET("/:id/keys", h.AdminPlus.SupplierKey.List)
			suppliers.POST("/:id/keys/provision", h.AdminPlus.SupplierKey.Provision)
			suppliers.POST("/:id/keys/:keyID/repair-binding", h.AdminPlus.SupplierKey.RepairBinding)
			suppliers.POST("/:id/rates/sync", h.AdminPlus.Rate.SyncSupplierRates)
			suppliers.GET("/:id/session", h.AdminPlus.Session.Get)
			suppliers.POST("/:id/session/probe", h.AdminPlus.Session.Probe)
			suppliers.POST("/:id/browser-sessions", h.AdminPlus.Session.Upsert)
		}

		sub2api := adminPlus.Group("/sub2api")
		{
			sub2api.GET("/accounts", h.AdminPlus.Supplier.ListLocalAccounts)
			sub2api.GET("/account-runtime", h.AdminPlus.Sub2API.ListAccountRuntime)
			sub2api.GET("/usage-lines", h.AdminPlus.Sub2API.ListLocalUsageLines)
			sub2api.GET("/usage-summary", h.AdminPlus.Sub2API.ListLocalUsageSummaries)
			sub2api.GET("/account-usage-summary", h.AdminPlus.Sub2API.ListLocalAccountUsageSummaries)
		}

		rates := adminPlus.Group("/rates")
		{
			rates.POST("/snapshots", h.AdminPlus.Rate.RecordSnapshot)
			rates.GET("/snapshots", h.AdminPlus.Rate.ListSnapshots)
			rates.GET("/events", h.AdminPlus.Rate.ListEvents)
			rates.PATCH("/events/:id/ack", h.AdminPlus.Rate.AcknowledgeEvent)
		}

		balances := adminPlus.Group("/balances")
		{
			balances.POST("/snapshots", h.AdminPlus.Balance.RecordSnapshot)
			balances.GET("/snapshots", h.AdminPlus.Balance.ListSnapshots)
			balances.GET("/events", h.AdminPlus.Balance.ListEvents)
			balances.PATCH("/events/:id/ack", h.AdminPlus.Balance.AcknowledgeEvent)
		}

		promotions := adminPlus.Group("/promotions")
		{
			promotions.POST("", h.AdminPlus.Promotion.RecordPromotion)
			promotions.GET("", h.AdminPlus.Promotion.ListEvents)
			promotions.PATCH("/:id/ack", h.AdminPlus.Promotion.AcknowledgeEvent)
		}

		health := adminPlus.Group("/health")
		{
			health.POST("/probe", h.AdminPlus.Health.ProbeOpenAIResponses)
			health.POST("/samples", h.AdminPlus.Health.RecordSample)
			health.GET("/samples", h.AdminPlus.Health.ListSamples)
			health.GET("/events", h.AdminPlus.Health.ListEvents)
			health.PATCH("/events/:id/ack", h.AdminPlus.Health.AcknowledgeEvent)
		}

		notifications := adminPlus.Group("/notifications")
		{
			notifications.GET("/deliveries", h.AdminPlus.Notification.ListDeliveries)
		}

		billing := adminPlus.Group("/billing")
		{
			billing.POST("/lines/import", h.AdminPlus.Billing.ImportBillLines)
			billing.GET("/lines", h.AdminPlus.Billing.ListBillLines)
		}

		extension := adminPlus.Group("/extension")
		{
			extension.GET("/manifest", h.AdminPlus.Extension.Manifest)
			extension.GET("/package.zip", h.AdminPlus.Extension.DownloadPackage)
			extension.POST("/tasks", h.AdminPlus.Extension.CreateTask)
			extension.GET("/tasks", h.AdminPlus.Extension.ListTasks)
			extension.POST("/tasks/claim", h.AdminPlus.Extension.ClaimTask)
			extension.POST("/session/capture-task", h.AdminPlus.Extension.CreateCaptureSessionTask)
			extension.POST("/tasks/:id/heartbeat", h.AdminPlus.Extension.Heartbeat)
			extension.POST("/tasks/:id/browser-credential", h.AdminPlus.Extension.GetBrowserCredential)
			extension.POST("/tasks/:id/complete", h.AdminPlus.Extension.CompleteTask)
			extension.POST("/tasks/:id/fail", h.AdminPlus.Extension.FailTask)
		}

		scheduler := adminPlus.Group("/scheduler")
		{
			scheduler.GET("/status", h.AdminPlus.Scheduler.Status)
			scheduler.POST("/run", h.AdminPlus.Scheduler.Run)
		}

		reconciliation := adminPlus.Group("/reconciliation")
		{
			reconciliation.POST("/run", h.AdminPlus.Reconciliation.Run)
		}

		actions := adminPlus.Group("/actions")
		{
			actions.POST("/generate", h.AdminPlus.Action.Generate)
			actions.GET("/recommendations", h.AdminPlus.Action.ListRecommendations)
			actions.PATCH("/recommendations/:id/status", h.AdminPlus.Action.UpdateRecommendationStatus)
		}
	}
}
