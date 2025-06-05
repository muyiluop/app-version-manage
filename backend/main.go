package main

import (
	v1 "app_version_manage/api/v1"
	"app_version_manage/config"
	"app_version_manage/middleware"
	"app_version_manage/repository"
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
)

// @title 应用版本管理系统 API
// @version 1.0
// @description 这是一个应用版本管理系统的API服务
// @host localhost:8080
// @BasePath /api
func main() {
	// 初始化数据库
	if err := repository.InitDB(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// 创建Gin实例
	r := gin.Default()

	// 静态文件服务
	r.Static("/static", config.GlobalConfig.Storage.Path)
	// API路由
	apiGroup := r.Group("/api")
	{
		// 开放API
		apiGroup.GET("/open/latest", v1.GetLatestVersion)
		apiGroup.GET("/open/changelog", v1.GetChangelog)
		apiGroup.GET("/open/download/:token", v1.SecureDownload)
		apiGroup.GET("/share/:token", v1.GetSharedApp)
		apiGroup.GET("/share/:token/versions", v1.GetSharedAppVersions)

		// 认证API
		apiGroup.POST("/auth/login", v1.Login)

		// 需要认证的API组
		auth := apiGroup.Group("/", middleware.AuthMiddleware())
		{
			// 应用管理
			auth.POST("/apps", v1.CreateApp)
			auth.PUT("/apps/:id", v1.UpdateApp)
			auth.GET("/apps/:id", v1.GetApp)
			auth.GET("/apps", v1.ListApps)
			auth.POST("/apps/:id/share", v1.GenerateShareLink)

			// 版本管理
			auth.POST("/versions", v1.CreateVersion)
			auth.PUT("/versions/:id/deactivate", v1.DeactivateVersion)
			auth.DELETE("/versions/:id", v1.DeleteVersion)
			auth.GET("/versions", v1.ListVersions) // 模板管理
			auth.POST("/templates", v1.CreateTemplate)
			auth.PUT("/templates/:id", v1.UpdateTemplate)
			auth.DELETE("/templates/:id", v1.DeleteTemplate)
			auth.GET("/templates", v1.ListTemplates)

			// 文件管理
			auth.POST("/files/upload", v1.UploadFile)
			auth.GET("/files", v1.ListFiles)
			auth.POST("/files/clean", v1.CleanUnusedFiles)
			auth.GET("/files/download/*path", v1.DownloadFile) // 新增文件下载路由
		}
	}

	// 启动服务器
	addr := fmt.Sprintf(":%d", config.GlobalConfig.Server.Port)
	log.Printf("Server is running on http://localhost%s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
