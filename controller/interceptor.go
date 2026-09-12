package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/interceptor"
	"github.com/gin-gonic/gin"
)

func GetAllInterceptors(c *gin.Context) {
	interceptors, err := model.GetAllInterceptors()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": interceptors})
}

func GetInterceptor(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "无效的 ID"})
		return
	}
	it, err := model.GetInterceptorById(id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": it})
}

func CreateInterceptor(c *gin.Context) {
	var it model.ChannelInterceptor
	if err := c.ShouldBindJSON(&it); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "参数错误: " + err.Error()})
		return
	}
	if err := model.CreateInterceptor(&it); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	_ = interceptor.ReloadCache()
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "创建成功"})
}

func UpdateInterceptor(c *gin.Context) {
	var it model.ChannelInterceptor
	if err := c.ShouldBindJSON(&it); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "参数错误: " + err.Error()})
		return
	}
	if err := model.UpdateInterceptor(&it); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	_ = interceptor.ReloadCache()
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "更新成功"})
}

func DeleteInterceptor(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "无效的 ID"})
		return
	}
	if err := model.DeleteInterceptor(id); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	_ = interceptor.ReloadCache()
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "删除成功"})
}

func GetInterceptorRuleTypes(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    interceptor.AllRuleDescriptions,
	})
}

func GetAllInterceptorLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	username := c.Query("username")
	channelId, _ := strconv.Atoi(c.Query("channel_id"))
	action := c.Query("action")
	modelName := c.Query("model_name")
	ruleType := c.Query("rule_type")
	start, _ := strconv.ParseInt(c.Query("start"), 10, 64)
	end, _ := strconv.ParseInt(c.Query("end"), 10, 64)

	logs, total, err := model.GetAllInterceptorLogs(page, pageSize, username, channelId,
		action, modelName, ruleType, start, end)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"items": logs,
			"total": total,
			"page":  page,
		},
	})
}

func GetInterceptorPresets(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "data": interceptor.Presets()})
}

func GetInterceptorLogStats(c *gin.Context) {
	start, _ := strconv.ParseInt(c.Query("start"), 10, 64)
	end, _ := strconv.ParseInt(c.Query("end"), 10, 64)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	stats, err := model.GetInterceptorLogStats(start, end, limit)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": stats})
}

func DeleteInterceptorLog(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "无效的 ID"})
		return
	}
	if err := model.DeleteInterceptorLog(id); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "删除成功"})
}

func ClearInterceptorLogs(c *gin.Context) {
	beforeStr := c.DefaultQuery("before", "0")
	before, _ := strconv.ParseInt(beforeStr, 10, 64)
	if before <= 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "无效的 before 参数"})
		return
	}
	affected, err := model.ClearInterceptorLogs(before)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "清理完成",
		"data":    gin.H{"deleted": affected},
	})
}
