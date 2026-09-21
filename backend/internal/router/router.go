// Package router 负责路由注册与中间件装配。
package router

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"app_version_manage/internal/api/common"
	"app_version_manage/internal/api/v1compat"
	"app_version_manage/internal/api/v2"
	"app_version_manage/internal/apierr"
	"app_version_manage/internal/config"
	"app_version_manage/internal/middleware"
	"app_version_manage/internal/service"
)

// New 构造 gin 引擎。
func New(cfg *config.Config, svc *service.Services, log *slog.Logger) *gin.Engine {
	if cfg.Server.IsRelease() {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(
		middleware.RequestID(),
		middleware.AccessLog(log),
		middleware.Recovery(log),
		middleware.SecurityHeaders(),
		middleware.CORS(cfg.CORS.AllowedOrigins),
		middleware.MaxBodySize(int64(cfg.Upload.MaxSizeMB)<<20+64*1024*1024),
	)

	health := newHealthHandler(svc, log)
	r.GET("/healthz", health.Live)
	r.GET("/readyz", health.Ready)

	loginLimiter := middleware.NewRateLimiter(cfg.Security.RateLimitLoginPerMin)
	shareLimiter := middleware.NewRateLimiter(cfg.Security.RateLimitSharePerMin)
	openLimiter := middleware.NewRateLimiter(cfg.Security.RateLimitOpenPerMin)

	v1 := v1compat.New(svc, cfg, log)
	v2h := v2.New(svc, cfg, log)

	api := r.Group("/api")

	// ---------- v1 兼容：公开接口 ----------
	api.POST("/auth/login", middleware.RateLimit(loginLimiter, nil), v1.Login)

	open := api.Group("/open", middleware.RateLimit(openLimiter, nil))
	{
		open.GET("/latest", v1.GetLatestVersion)
		open.GET("/changelog", v1.GetChangelog)
		open.GET("/check", v1.CheckUpdate)
		open.GET("/download/:token", v1.SecureDownload)
	}

	share := api.Group("/share", middleware.RateLimit(shareLimiter, nil))
	{
		share.POST("/:token", v1.GetSharedApp)
		share.POST("/:token/versions", v1.GetSharedAppVersions)
	}

	// 公开的应用图标：仅当对象键确实被某个应用引用时才允许访问。
	api.GET("/static/logos/*key", func(c *gin.Context) {
		key := strings.TrimPrefix(c.Param("key"), "/")
		exists, err := svc.App.LogoKeyExists(c.Request.Context(), key)
		if err != nil || !exists {
			c.Status(http.StatusNotFound)
			return
		}
		if err := common.ServeObject(c, svc.File, key, false); err != nil {
			c.Status(http.StatusNotFound)
		}
	})

	// ---------- v2 ----------
	v2Group := api.Group("/v2")
	{
		// 公开
		v2Group.POST("/auth/login", middleware.RateLimit(loginLimiter, nil), v2h.Login)
		v2Group.POST("/auth/refresh", middleware.RateLimit(loginLimiter, nil), v2h.Refresh)
		v2Group.GET("/check", middleware.RateLimit(openLimiter, nil), v2h.Check)
		v2Group.GET("/download/:token", middleware.RateLimit(openLimiter, nil), v2h.Download)

		// 需要登录
		authed := v2Group.Group("", middleware.JWTAuth(svc.Auth))
		{
			authed.GET("/auth/profile", v2h.Profile)
			authed.POST("/auth/change-password", v2h.ChangePassword)

			admin := authed.Group("", middleware.RequireRole())
			{
				admin.GET("/users", v2h.ListUsers)
				admin.POST("/users", v2h.CreateUser)
				admin.PUT("/users/:id", v2h.UpdateUser)
				admin.DELETE("/users/:id", v2h.DeleteUser)
				admin.GET("/audit-logs", v2h.ListAuditLogs)
			}

			write := middleware.RequireWrite()

			authed.GET("/apps", v2h.ListApps)
			authed.POST("/apps", write, v2h.CreateApp)
			authed.GET("/apps/:id", v2h.GetApp)
			authed.PUT("/apps/:id", write, v2h.UpdateApp)
			authed.DELETE("/apps/:id", middleware.RequireRole(), v2h.DeleteApp)

			authed.GET("/apps/:id/channels", v2h.ListChannels)
			authed.POST("/apps/:id/channels", write, v2h.CreateChannel)
			authed.PUT("/apps/:id/channels/:channelId", write, v2h.UpdateChannel)
			authed.DELETE("/apps/:id/channels/:channelId", write, v2h.DeleteChannel)
			authed.PUT("/apps/:id/default-channel", write, v2h.SetDefaultChannel)

			authed.GET("/versions", v2h.ListVersions)
			authed.POST("/versions", write, v2h.PublishVersion)
			authed.GET("/versions/:id", v2h.GetVersion)
			authed.PUT("/versions/:id", write, v2h.UpdateVersion)
			authed.PUT("/versions/:id/status", write, v2h.SetVersionStatus)
			authed.DELETE("/versions/:id", write, v2h.DeleteVersion)
			authed.GET("/versions/:id/download", v2h.DownloadVersionByID)

			authed.GET("/files", v2h.ListFiles)
			authed.POST("/files/upload", write, v2h.UploadFile)
			authed.POST("/files/clean", write, v2h.CleanFiles)
			authed.GET("/files/:id/download", v2h.DownloadFileByID)
			authed.DELETE("/files/:id", write, v2h.DeleteFile)

			authed.GET("/apps/:id/shares", v2h.ListShares)
			authed.POST("/apps/:id/shares", write, v2h.CreateShare)
			authed.PUT("/apps/:id/shares/:shareId", write, v2h.UpdateShare)
			authed.PUT("/apps/:id/shares/:shareId/deactivate", write, v2h.DeactivateShare)
			authed.DELETE("/apps/:id/shares/:shareId", write, v2h.DeleteShare)

			authed.GET("/apps/:id/templates", v2h.ListTemplates)
			authed.POST("/apps/:id/templates", write, v2h.CreateTemplate)
			authed.PUT("/apps/:id/templates/:templateId", write, v2h.UpdateTemplate)
			authed.DELETE("/apps/:id/templates/:templateId", write, v2h.DeleteTemplate)
			authed.POST("/apps/:id/templates/:templateId/preview", v2h.PreviewTemplate)
		}
	}

	r.NoRoute(func(c *gin.Context) {
		apierr.FailRaw(c, apierr.CodeNotFound, http.StatusNotFound, "接口不存在")
	})
	return r
}
