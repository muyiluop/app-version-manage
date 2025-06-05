// Package v1 API文档
package v1

import (
	"app_version_manage/model"
	"app_version_manage/repository"
	"bytes"
	"log"
	"net/http"
	"strings"
	"text/template"
	"time"

	"app_version_manage/utils"

	"github.com/gin-gonic/gin"
)

// @Summary 获取应用最新版本
// @Description 根据应用标识和平台获取最新版本信息，支持自定义输出格式
// @Tags 开放接口
// @Accept json
// @Produce json
// @Param identifier query string true "应用标识"
// @Param platform query string false "平台 (android/ios/windows/macos/linux/harmony)"
// @Param format query string false "输出格式，对应模板名称"
// @Success 200 {object} map[string]interface{} "返回最新版本信息"
// @Failure 404 {object} map[string]string "应用或版本不存在"
// @Failure 500 {object} map[string]string "服务器内部错误"
// @Router /open/latest [get]
func GetLatestVersion(c *gin.Context) {
	identifier := c.Query("identifier")
	platform := c.Query("platform")
	format := c.Query("format")

	// 获取应用信息
	var app model.Application
	if err := repository.DB.Where("identifier = ?", identifier).First(&app).Error; err != nil {
		log.Printf("[Error] GetLatestVersion - 应用不存在: identifier=%s, error=%v", identifier, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "应用不存在"})
		return
	}

	// 如果未指定平台，尝试从请求头获取
	if platform == "" {
		platform = c.GetHeader("X-Platform")
		if platform == "" {
			userAgent := c.GetHeader("User-Agent")
			platform = detectPlatform(userAgent)
		}
	}

	// 构建查询
	query := repository.DB.Model(&model.Version{}).
		Where("app_id = ? AND is_active = ?", app.ID, true).
		Order("created_at DESC")

	if platform != "" {
		query = query.Where("platform = ?", platform)
	}

	var version model.Version
	if err := query.First(&version).Error; err != nil {
		log.Printf("[Error] GetLatestVersion - 未找到版本信息: identifier=%s, platform=%s, error=%v",
			identifier, platform, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到版本信息"})
		return
	} // 生成加密的下载URL，有效期24小时
	downloadToken, err := utils.GenerateSecureToken(version.FilePath, 24*time.Hour)
	if err != nil {
		log.Printf("[Error] renderTemplate - 生成下载token失败: error=%v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取文件下载链接失败"})
		return
	}
	version.FilePath = downloadToken
	// 如果指定了输出格式
	if format != "" {
		var template model.Template
		if err := repository.DB.Where("app_id = ? AND name = ?", app.ID, format).First(&template).Error; err != nil {
			log.Printf("[Error] GetLatestVersion - 未找到指定的输出模板: appId=%d, format=%s, error=%v",
				app.ID, format, err)
			c.JSON(http.StatusNotFound, gin.H{"error": "未找到指定的输出模板"})
			return
		}
		// 渲染模板
		output, err := renderTemplate(template.Content, app, version)
		if err != nil {
			log.Printf("[Error] GetLatestVersion - 渲染模板失败: templateId=%d, templateName=%s, error=%v",
				template.ID, template.Name, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "渲染模板失败"})
			return
		}

		// 根据模板名称后缀确定内容类型
		contentType := "text/plain"
		if strings.HasSuffix(template.Name, ".json") {
			contentType = "application/json"
		} else if strings.HasSuffix(template.Name, ".xml") {
			contentType = "application/xml"
		} else if strings.HasSuffix(template.Name, ".html") {
			contentType = "text/html"
		} else if strings.HasSuffix(template.Name, ".yaml") || strings.HasSuffix(template.Name, ".yml") {
			contentType = "application/x-yaml"
		}

		c.Header("Content-Type", contentType)
		c.String(http.StatusOK, output)
		return
	}

	// 默认输出格式
	c.JSON(http.StatusOK, gin.H{
		"appName":    app.Name,
		"appId":      app.ID,
		"identifier": app.Identifier,
		"version":    version.Version,
		"platform":   version.Platform,
		"changelog":  version.Changelog,
		"isForce":    version.ForceUpdate,
		"fileName":   version.FileName,
		"filePath":   version.FilePath,
		"fileSize":   version.FileSize,
		"createdAt":  version.CreatedAt.Format(time.RFC3339),
	})
}

// @Summary 获取应用更新日志
// @Description 获取应用的版本更新历史记录
// @Tags 开放接口
// @Accept json
// @Produce json
// @Param identifier query string true "应用标识"
// @Param platform query string false "平台筛选"
// @Success 200 {array} map[string]interface{} "版本更新历史列表"
// @Failure 404 {object} map[string]string "应用不存在"
// @Failure 500 {object} map[string]string "服务器内部错误"
// @Router /open/changelog [get]
func GetChangelog(c *gin.Context) {
	identifier := c.Query("identifier")
	platform := c.Query("platform")
	// 获取应用信息
	var app model.Application
	if err := repository.DB.Where("identifier = ?", identifier).First(&app).Error; err != nil {
		log.Printf("[Error] GetChangelog - 应用不存在: identifier=%s, error=%v", identifier, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "应用不存在"})
		return
	}

	// 构建查询
	query := repository.DB.Model(&model.Version{}).
		Where("app_id = ? AND is_active = ?", app.ID, true).
		Order("created_at DESC")

	if platform != "" {
		query = query.Where("platform = ?", platform)
	}

	var versions []struct {
		CreatedAt string         `json:"createdAt"`
		Version   string         `json:"version"`
		Platform  model.Platform `json:"platform"`
		Changelog string         `json:"changelog"`
	}
	if err := query.Find(&versions).Error; err != nil {
		log.Printf("[Error] GetChangelog - 获取更新日志失败: appId=%d, platform=%s, error=%v",
			app.ID, platform, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取更新日志失败"})
		return
	}

	c.JSON(http.StatusOK, versions)
}

// 根据 User-Agent 检测平台
func detectPlatform(userAgent string) string {
	userAgent = strings.ToLower(userAgent)
	switch {
	case strings.Contains(userAgent, "android"):
		return string(model.Android)
	case strings.Contains(userAgent, "iphone") || strings.Contains(userAgent, "ipad"):
		return string(model.IOS)
	case strings.Contains(userAgent, "windows"):
		return string(model.Windows)
	case strings.Contains(userAgent, "macintosh") || strings.Contains(userAgent, "mac os"):
		return string(model.MacOS)
	case strings.Contains(userAgent, "linux"):
		return string(model.Linux)
	case strings.Contains(userAgent, "harmony"):
		return string(model.Harmony)
	default:
		return ""
	}
}

// 渲染模板
func renderTemplate(templateContent string, app model.Application, version model.Version) (string, error) {

	// 准备模板变量
	data := map[string]any{
		"app": map[string]any{
			"id":          app.ID,
			"name":        app.Name,
			"identifier":  app.Identifier,
			"logo":        app.Logo,
			"description": app.Description,
		},
		"ver": map[string]interface{}{
			"version":   version.Version,
			"platform":  version.Platform,
			"changelog": version.Changelog,
			"isForce":   version.ForceUpdate,
			"fileName":  version.FileName,
			"filePath":  version.FilePath,
			"fileSize":  version.FileSize,
			"createdAt": version.CreatedAt.Format(time.RFC3339),
		},
	}

	// 将模板内容中的 ${xxx} 转换为 Go template 语法 {{.xxx}}
	templateContent = strings.ReplaceAll(templateContent, "${", "{{.")
	templateContent = strings.ReplaceAll(templateContent, "}", "}}")

	// 解析模板
	tmpl, err := template.New("output").Parse(templateContent)
	if err != nil {
		log.Printf("[Error] renderTemplate - 模板解析失败: error=%v", err)
		return "", err
	}

	// 执行模板
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		log.Printf("[Error] renderTemplate - 执行模板失败: error=%v", err)
		return "", err
	}

	return buf.String(), nil
}
