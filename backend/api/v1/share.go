package v1

import (
	"app_version_manage/model"
	"app_version_manage/repository"
	"app_version_manage/utils"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// GenerateShareLink 生成应用分享链接
func GenerateShareLink(c *gin.Context) {
	appId := c.Param("id")

	// 获取应用信息
	var app model.Application
	if err := repository.DB.First(&app, appId).Error; err != nil {
		log.Printf("[Error] GenerateShareLink - 应用不存在: appId=%s, error=%v", appId, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "应用不存在"})
		return
	}

	// 生成分享token，有效期30天
	token, err := utils.GenerateSecureToken(appId, 30*24*time.Hour)
	if err != nil {
		log.Printf("[Error] GenerateShareLink - 生成token失败: appId=%s, error=%v", appId, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成分享链接失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"shareToken": token,
	})
}

// GetSharedApp 获取分享的应用信息
func GetSharedApp(c *gin.Context) {
	token := c.Param("token")

	// 验证token
	appId, err := utils.ValidateSecureToken(token)
	if err != nil {
		log.Printf("[Error] GetSharedApp - 无效的分享链接: token=%s, error=%v", token, err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "无效或已过期的分享链接"})
		return
	}

	// 获取应用信息
	var app model.Application
	if err := repository.DB.First(&app, appId).Error; err != nil {
		log.Printf("[Error] GetSharedApp - 应用不存在: appId=%s, error=%v", appId, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "应用不存在"})
		return
	}

	// 获取各平台最新版本
	var versions []model.Version
	if err := repository.DB.Where("app_id = ? AND is_active = ?", app.ID, true).
		Group("platform").
		Order("created_at DESC").
		Find(&versions).Error; err != nil {
		log.Printf("[Error] GetSharedApp - 获取版本信息失败: appId=%s, error=%v", appId, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取版本信息失败"})
		return
	}

	// 检测当前平台
	platform := c.Query("platform")
	if platform == "" {
		platform = c.GetHeader("X-Platform")
		if platform == "" {
			userAgent := c.GetHeader("User-Agent")
			platform = detectPlatform(userAgent)
		}
	} // 将versions中的文件地址修改为加密地址
	for i := range versions {
		if versions[i].FilePath != "" {
			downloadToken, err := utils.GenerateSecureToken(versions[i].FilePath, 24*time.Hour)
			if err != nil {
				log.Printf("[Error] GetSharedAppVersions - 生成下载token失败: versionId=%d, error=%v", versions[i].ID, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "获取文件下载链接失败"})
				return
			}
			versions[i].FilePath = downloadToken
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"app": gin.H{
			"name":        app.Name,
			"identifier":  app.Identifier,
			"logo":        app.Logo,
			"description": app.Description,
			"platforms":   app.Platforms,
		},
		"versions":        versions,
		"currentPlatform": platform,
	})
}

// GetSharedAppVersions 获取分享应用的版本历史
func GetSharedAppVersions(c *gin.Context) {
	token := c.Param("token")
	platform := c.Query("platform")

	// 验证token
	appId, err := utils.ValidateSecureToken(token)
	if err != nil {
		log.Printf("[Error] GetSharedAppVersions - 无效的分享链接: token=%s, error=%v", token, err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "无效或已过期的分享链接"})
		return
	}

	var pagination model.PaginationQuery
	if err := c.ShouldBindQuery(&pagination); err != nil {
		pagination = model.PaginationQuery{
			Page:     1,
			PageSize: 99,
		}
	}

	query := repository.DB.Model(&model.Version{})

	if appId != "" {
		query = query.Where("app_id = ?", appId)
	}
	if platform != "" {
		query = query.Where("platform = ?", platform)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取版本总数失败"})
		return
	}

	var versions []model.Version
	if err := query.Offset((pagination.Page - 1) * pagination.PageSize).
		Limit(pagination.PageSize).
		Order("created_at DESC").
		Find(&versions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取版本列表失败"})
		return
	}
	// 将versions中的文件地址修改为加密地址
	for i := range versions {
		if versions[i].FilePath != "" {
			downloadToken, err := utils.GenerateSecureToken(versions[i].FilePath, 24*time.Hour)
			if err != nil {
				log.Printf("[Error] GetSharedAppVersions - 生成下载token失败: versionId=%d, error=%v", versions[i].ID, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "获取文件下载链接失败"})
				return
			}
			versions[i].FilePath = downloadToken
		}
	}

	c.JSON(http.StatusOK, model.NewPaginationResponse(versions, total, pagination.Page, pagination.PageSize))
}
