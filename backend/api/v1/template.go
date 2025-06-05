package v1

import (
	"app_version_manage/model"
	"app_version_manage/repository"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// CreateTemplateRequest 创建模板请求
type CreateTemplateRequest struct {
	AppID   uint   `json:"appId" binding:"required"`
	Name    string `json:"name" binding:"required"`
	Content string `json:"content" binding:"required"`
}

// CreateTemplate 创建模板
func CreateTemplate(c *gin.Context) {
	var req CreateTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("[Error] CreateTemplate - 无效的请求参数: error=%v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数"})
		return
	}

	// 检查应用是否存在
	var app model.Application
	if err := repository.DB.First(&app, req.AppID).Error; err != nil {
		log.Printf("[Error] CreateTemplate - 应用不存在: appId=%d, error=%v", req.AppID, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "应用不存在"})
		return
	}

	template := model.Template{
		AppID:   req.AppID,
		Name:    req.Name,
		Content: req.Content,
	}

	if err := repository.DB.Create(&template).Error; err != nil {
		log.Printf("[Error] CreateTemplate - 创建模板失败: appId=%d, name=%s, error=%v",
			req.AppID, req.Name, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建模板失败"})
		return
	}

	c.JSON(http.StatusOK, template)
}

// UpdateTemplate 更新模板
func UpdateTemplate(c *gin.Context) {
	var req CreateTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("[Error] UpdateTemplate - 无效的请求参数: error=%v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数"})
		return
	}

	templateID := c.Param("id")
	var template model.Template
	if err := repository.DB.First(&template, templateID).Error; err != nil {
		log.Printf("[Error] UpdateTemplate - 模板不存在: templateId=%s, error=%v", templateID, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "模板不存在"})
		return
	}

	template.Name = req.Name
	template.Content = req.Content

	if err := repository.DB.Save(&template).Error; err != nil {
		log.Printf("[Error] UpdateTemplate - 更新模板失败: templateId=%d, name=%s, error=%v",
			template.ID, template.Name, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新模板失败"})
		return
	}

	c.JSON(http.StatusOK, template)
}

// DeleteTemplate 删除模板
func DeleteTemplate(c *gin.Context) {
	templateID := c.Param("id")
	if err := repository.DB.Delete(&model.Template{}, templateID).Error; err != nil {
		log.Printf("[Error] DeleteTemplate - 删除模板失败: templateId=%s, error=%v", templateID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除模板失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "模板已删除"})
}

// ListTemplates 获取模板列表
func ListTemplates(c *gin.Context) {
	appID := c.Query("appId")

	query := repository.DB.Model(&model.Template{})
	if appID != "" {
		query = query.Where("app_id = ?", appID)
	}

	var templates []model.Template
	if err := query.Find(&templates).Error; err != nil {
		log.Printf("[Error] ListTemplates - 获取模板列表失败: appId=%s, error=%v", appID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取模板列表失败"})
		return
	}

	c.JSON(http.StatusOK, templates)
}
