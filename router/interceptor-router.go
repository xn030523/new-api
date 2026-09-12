package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-gonic/gin"
)

func registerInterceptorRoutes(apiRouter *gin.RouterGroup) {
	interceptorRoute := apiRouter.Group("/interceptor")
	interceptorRoute.Use(middleware.AdminAuth())
	{
		interceptorRoute.GET("/", controller.GetAllInterceptors)
		interceptorRoute.GET("/rule_types", controller.GetInterceptorRuleTypes)
		interceptorRoute.GET("/:id", controller.GetInterceptor)
		interceptorRoute.POST("/", controller.CreateInterceptor)
		interceptorRoute.PUT("/", controller.UpdateInterceptor)
		interceptorRoute.DELETE("/:id", controller.DeleteInterceptor)
		// 拦截日志
		interceptorRoute.GET("/logs/", controller.GetAllInterceptorLogs)
		interceptorRoute.GET("/logs/stats", controller.GetInterceptorLogStats)
		interceptorRoute.DELETE("/logs/:id", controller.DeleteInterceptorLog)
		interceptorRoute.DELETE("/logs/", controller.ClearInterceptorLogs)
	}
}
