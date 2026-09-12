package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/service/tgbot"
	"github.com/gin-gonic/gin"
)

// botService is the global TG bot service instance.
var botService *tgbot.BotService

// InitTgBot initializes and starts the TG bot if enabled.
func InitTgBot() {
	config, err := tgbot.LoadBotConfig()
	if err != nil {
		return
	}
	if !config.Enabled || config.BotToken == "" {
		return
	}
	features, err := tgbot.LoadAllFeatures()
	if err != nil {
		return
	}
	botService = tgbot.NewBotService()
	if err := botService.Start(config, features); err != nil {
		botService = nil
	}
}

// RestartTgBot restarts the bot with current config.
func RestartTgBot() {
	if botService != nil {
		botService.Stop()
		botService = nil
	}
	InitTgBot()
}

func GetTgBotConfig(c *gin.Context) {
	config, err := tgbot.LoadBotConfig()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	features, err := tgbot.LoadAllFeatures()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"config":   config,
			"features": features,
			"running":  botService != nil,
			"descriptions": tgbot.FeatureDescriptions,
		},
	})
}

func UpdateTgBotConfig(c *gin.Context) {
	var config tgbot.BotConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "参数错误: " + err.Error()})
		return
	}
	config.Id = 1
	if err := tgbot.SaveBotConfig(&config); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	// Restart bot with new config
	go RestartTgBot()
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "保存成功"})
}

func UpdateTgBotFeature(c *gin.Context) {
	var feature tgbot.BotFeature
	if err := c.ShouldBindJSON(&feature); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "参数错误: " + err.Error()})
		return
	}
	if feature.Name == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "缺少 feature name"})
		return
	}
	// Load existing to get the ID
	existing, err := tgbot.LoadFeature(feature.Name)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "功能不存在: " + feature.Name})
		return
	}
	feature.Id = existing.Id
	if err := tgbot.SaveFeature(&feature); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	// Restart bot to pick up changes
	go RestartTgBot()
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "保存成功"})
}

func GetTgBotFeatureTypes(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    tgbot.FeatureDescriptions,
	})
}
