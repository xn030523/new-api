package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-gonic/gin"
)

func registerTgBotRoutes(apiRouter *gin.RouterGroup) {
	tgBotRoute := apiRouter.Group("/tgbot")
	tgBotRoute.Use(middleware.AdminAuth())
	{
		tgBotRoute.GET("/", controller.GetTgBotConfig)
		tgBotRoute.PUT("/", controller.UpdateTgBotConfig)
		tgBotRoute.PUT("/feature", controller.UpdateTgBotFeature)
		tgBotRoute.GET("/feature_types", controller.GetTgBotFeatureTypes)
	}
}
