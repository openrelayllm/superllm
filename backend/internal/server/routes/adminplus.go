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
			suppliers.GET("/:id", h.AdminPlus.Supplier.Get)
			suppliers.PATCH("/:id/status", h.AdminPlus.Supplier.UpdateStatus)
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
			health.POST("/samples", h.AdminPlus.Health.RecordSample)
			health.GET("/samples", h.AdminPlus.Health.ListSamples)
			health.GET("/events", h.AdminPlus.Health.ListEvents)
			health.PATCH("/events/:id/ack", h.AdminPlus.Health.AcknowledgeEvent)
		}

		billing := adminPlus.Group("/billing")
		{
			billing.POST("/lines/import", h.AdminPlus.Billing.ImportBillLines)
			billing.GET("/lines", h.AdminPlus.Billing.ListBillLines)
		}

		extension := adminPlus.Group("/extension")
		{
			extension.POST("/tasks", h.AdminPlus.Extension.CreateTask)
			extension.GET("/tasks", h.AdminPlus.Extension.ListTasks)
			extension.POST("/tasks/claim", h.AdminPlus.Extension.ClaimTask)
			extension.POST("/tasks/:id/heartbeat", h.AdminPlus.Extension.Heartbeat)
			extension.POST("/tasks/:id/complete", h.AdminPlus.Extension.CompleteTask)
			extension.POST("/tasks/:id/fail", h.AdminPlus.Extension.FailTask)
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
