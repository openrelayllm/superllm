package routes

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

func RegisterPublicProxyAIRoutes(v1 *gin.RouterGroup, h *handler.Handlers, apiKeyAuth middleware.APIKeyAuthMiddleware) {
	if h == nil || h.AdminPlus == nil || (h.AdminPlus.PublicProxyAI == nil && h.AdminPlus.Purity == nil) {
		return
	}
	public := v1.Group("/public/proxyai", publicProxyAICORS())
	{
		public.OPTIONS("/*path", func(c *gin.Context) { c.Status(http.StatusNoContent) })
		if h.AdminPlus.PublicProxyAI != nil {
			public.GET("/summary", h.AdminPlus.PublicProxyAI.Summary)
			public.HEAD("/summary", h.AdminPlus.PublicProxyAI.Summary)
			public.GET("/runtime-config", h.AdminPlus.PublicProxyAI.RuntimeConfig)
			public.HEAD("/runtime-config", h.AdminPlus.PublicProxyAI.RuntimeConfig)
			public.GET("/sites", h.AdminPlus.PublicProxyAI.ListSites)
			public.HEAD("/sites", h.AdminPlus.PublicProxyAI.ListSites)
			public.GET("/sites/:slug", h.AdminPlus.PublicProxyAI.GetSite)
			public.HEAD("/sites/:slug", h.AdminPlus.PublicProxyAI.GetSite)
		}
		if h.AdminPlus.Purity != nil {
			public.POST("/web/purity/checks", h.AdminPlus.Purity.WebCheck)
			public.POST("/web/purity/checks/stream", h.AdminPlus.Purity.WebCheckStream)

			api := public.Group("/api")
			api.Use(requireProxyAIAPIKeyAuth(apiKeyAuth))
			api.POST("/purity/checks", h.AdminPlus.Purity.APICheck)
			api.POST("/purity/checks/stream", h.AdminPlus.Purity.APICheckStream)

			legacyAPI := public.Group("")
			legacyAPI.Use(requireProxyAIAPIKeyAuth(apiKeyAuth))
			legacyAPI.POST("/purity/checks", h.AdminPlus.Purity.APICheck)
			legacyAPI.POST("/purity/checks/stream", h.AdminPlus.Purity.APICheckStream)
		}
	}
}

func requireProxyAIAPIKeyAuth(apiKeyAuth middleware.APIKeyAuthMiddleware) gin.HandlerFunc {
	if apiKeyAuth != nil {
		return gin.HandlerFunc(apiKeyAuth)
	}
	return func(c *gin.Context) {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
			"error":   "proxyai_api_auth_not_configured",
			"message": "ProxyAI developer API authentication is not configured",
		})
	}
}

func publicProxyAICORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, HEAD, POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-API-Key, X-ProxyAI-Key")
		c.Header("Access-Control-Max-Age", "86400")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
