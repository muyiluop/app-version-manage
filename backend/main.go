package main

import (
	v1 "app_version_manage/api/v1"
	"app_version_manage/config"
	"app_version_manage/middleware"
	"app_version_manage/repository"
	"flag"
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
	configPath := flag.String("config", "config.yaml", "配置文件路径 (可选)")
	port := flag.Int("port", -1, "HTTP 服务端口，默认从配置文件读取")
	dbPath := flag.String("db", "", "数据库路径，覆盖配置文件")
	storagePath := flag.String("storage", "", "文件存储路径，覆盖配置文件")
	jwtSecret := flag.String("jwt-secret", "", "JWT 密钥，覆盖配置文件")
	jwtExpire := flag.Int("jwt-expire", -1, "JWT 过期时间（小时），覆盖配置文件")
	flag.Parse()

	overrides := config.Override{
		Port:      *port,
		DBPath:    *dbPath,
		Storage:   *storagePath,
		JWTSecret: *jwtSecret,
		JWTExpire: *jwtExpire,
	}

	if err := config.Load(*configPath, overrides); err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

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
		apiGroup.POST("/share/:token", v1.GetSharedApp)
		apiGroup.POST("/share/:token/versions", v1.GetSharedAppVersions)

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
			auth.GET("/apps/:id/shares", v1.ListShares)
			auth.PUT("/apps/:id/shares/:shareId", v1.UpdateShare)
			auth.PUT("/apps/:id/shares/:shareId/deactivate", v1.DeactivateShare)
			auth.DELETE("/apps/:id/shares/:shareId", v1.DeleteShare)

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
