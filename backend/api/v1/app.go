package v1

import (
	"app_version_manage/model"
	"app_version_manage/repository"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// CreateAppRequest 创建应用请求
type CreateAppRequest struct {
	Name        string           `json:"name" binding:"required"`
	Identifier  string           `json:"identifier" binding:"required"`
	Logo        string           `json:"logo"`
	Description string           `json:"description"`
	Platforms   []model.Platform `json:"platforms" binding:"required"`
}

// UpdateAppRequest 更新应用请求
type UpdateAppRequest struct {
	Name        string           `json:"name" binding:"required"`
	Logo        string           `json:"logo"`
	Description string           `json:"description"`
	Platforms   []model.Platform `json:"platforms" binding:"required"`
}

// CreateApp 创建应用
func CreateApp(c *gin.Context) {
	var req CreateAppRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数"})
		return
	}

	app := model.Application{
		Name:        req.Name,
		Identifier:  req.Identifier,
		Logo:        req.Logo,
		Description: req.Description,
		Platforms:   req.Platforms,
	}

	if err := repository.DB.Create(&app).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建应用失败"})
		return
	}

	c.JSON(http.StatusOK, app)
}

// UpdateApp 更新应用
func UpdateApp(c *gin.Context) {
	var req UpdateAppRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数"})
		return
	}

	appID := c.Param("id")
	var app model.Application
	if err := repository.DB.First(&app, appID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "应用不存在"})
		return
	}

	// 检查是否可以移除平台
	if len(req.Platforms) > 0 {
		// 获取当前有版本的平台
		var usedPlatforms []model.Platform
		if err := repository.DB.Model(&model.Version{}).
			Where("app_id = ?", app.ID).
			Distinct().
			Pluck("platform", &usedPlatforms).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "验证平台信息失败"})
			return
		}

		// 检查是否有平台被移除
		for _, platform := range usedPlatforms {
			found := false
			for _, newPlatform := range req.Platforms {
				if platform == newPlatform {
					found = true
					break
				}
			}
			if !found {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": fmt.Sprintf("平台 %s 已存在版本，无法移除", platform),
				})
				return
			}
		}
		app.Platforms = req.Platforms
	}

	// 更新其他字段
	app.Name = req.Name
	if req.Logo != "" {
		app.Logo = req.Logo
	}
	if req.Description != "" {
		app.Description = req.Description
	}

	if err := repository.DB.Save(&app).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新应用失败"})
		return
	}

	c.JSON(http.StatusOK, app)
}

// GetApp 获取应用详情
func GetApp(c *gin.Context) {
	appID := c.Param("id")
	var app model.Application
	if err := repository.DB.First(&app, appID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "应用不存在"})
		return
	}

	c.JSON(http.StatusOK, app)
}

// ListApps 获取应用列表
func ListApps(c *gin.Context) {
	var apps []model.Application
	if err := repository.DB.Find(&apps).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取应用列表失败"})
		return
	}

	c.JSON(http.StatusOK, apps)
}
