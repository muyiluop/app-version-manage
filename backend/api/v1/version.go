package v1

import (
	"app_version_manage/config"
	"app_version_manage/model"
	"app_version_manage/repository"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

// CreateVersionRequest 创建版本请求
type CreateVersionRequest struct {
	AppID       uint           `json:"appId" binding:"required"`
	Platform    model.Platform `json:"platform" binding:"required"`
	Version     string         `json:"version" binding:"required"`
	FilePath    string         `json:"filePath" binding:"required"`
	FileName    string         `json:"fileName" binding:"required"`
	FileSize    int64          `json:"fileSize" binding:"required"`
	Changelog   string         `json:"changelog"`
	Ext         string         `json:"ext"`
	ForceUpdate bool           `json:"forceUpdate"`
}

// CreateVersion 创建版本
func CreateVersion(c *gin.Context) {
	var req CreateVersionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数"})
		return
	}

	// 检查应用是否存在
	var app model.Application
	if err := repository.DB.First(&app, req.AppID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "应用不存在"})
		return
	}

	// 验证文件是否存在
	fullPath := filepath.Join(config.GlobalConfig.Storage.Path, req.FilePath)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "文件不存在"})
		return
	}

	version := model.Version{
		AppID:       req.AppID,
		Platform:    req.Platform,
		Version:     req.Version,
		FilePath:    req.FilePath,
		FileName:    req.FileName,
		FileSize:    req.FileSize,
		Changelog:   req.Changelog,
		Ext:         req.Ext,
		ForceUpdate: req.ForceUpdate,
		IsActive:    true,
	}

	if err := repository.DB.Create(&version).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建版本失败"})
		return
	}

	c.JSON(http.StatusOK, version)
}

// DeactivateVersion 下架版本
func DeactivateVersion(c *gin.Context) {
	versionID := c.Param("id")
	var version model.Version
	if err := repository.DB.First(&version, versionID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "版本不存在"})
		return
	}

	version.IsActive = false
	if err := repository.DB.Save(&version).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "下架版本失败"})
		return
	}

	c.JSON(http.StatusOK, version)
}

// DeleteVersion 删除版本
func DeleteVersion(c *gin.Context) {
	versionID := c.Param("id")
	if err := repository.DB.Delete(&model.Version{}, versionID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除版本失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "版本已删除"})
}

// ListVersions 获取版本列表
func ListVersions(c *gin.Context) {
	var pagination model.PaginationQuery
	if err := c.ShouldBindQuery(&pagination); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的分页参数"})
		return
	}

	appID := c.Query("appId")
	platform := c.Query("platform")

	query := repository.DB.Model(&model.Version{})

	if appID != "" {
		query = query.Where("app_id = ?", appID)
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
		Omit("ext").
		Order("created_at DESC").
		Find(&versions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取版本列表失败"})
		return
	}

	c.JSON(http.StatusOK, model.NewPaginationResponse(versions, total, pagination.Page, pagination.PageSize))
}
