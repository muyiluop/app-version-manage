// Package v1compat 实现与历史版本完全兼容的公开接口。
//
// 兼容范围（响应结构与字段名保持一字不变）：
//   - POST /api/auth/login
//   - GET  /api/open/latest
//   - GET  /api/open/changelog
//   - GET  /api/open/download/:token
//   - POST /api/share/:token
//   - POST /api/share/:token/versions
//
// 同时提供 v1 风格的检测更新接口 GET /api/open/check。
// 后台管理接口统一迁移到 /api/v2。
package v1compat

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"app_version_manage/internal/api/common"
	"app_version_manage/internal/apierr"
	"app_version_manage/internal/config"
	"app_version_manage/internal/model"
	"app_version_manage/internal/service"
)

// Handler v1 兼容处理器。
type Handler struct {
	svc *service.Services
	cfg *config.Config
	log *slog.Logger
}

// New 构造 v1 兼容处理器。
func New(svc *service.Services, cfg *config.Config, log *slog.Logger) *Handler {
	return &Handler{svc: svc, cfg: cfg, log: log}
}

// fail 以 v1 的 {"error": "..."} 结构输出错误。
func fail(c *gin.Context, err error) {
	e := apierr.AsError(err)
	if e.Code == apierr.CodePasswordRequired {
		// 历史行为：209 表示需要密码验证。
		c.JSON(209, gin.H{"error": "需要密码验证", "requirePassword": true})
		return
	}
	c.JSON(e.Status, gin.H{"error": e.Message})
}

// legacyVersion 历史版本的序列化结构，字段名必须与 v1 保持一致。
type legacyVersion struct {
	ID          uint           `json:"id"`
	AppID       uint           `json:"appId"`
	Platform    model.Platform `json:"platform"`
	Version     string         `json:"version"`
	FilePath    string         `json:"filePath"`
	FileName    string         `json:"fileName"`
	FileSize    int64          `json:"fileSize"`
	Changelog   string         `json:"changelog"`
	Ext         string         `json:"ext"`
	ForceUpdate bool           `json:"forceUpdate"`
	IsActive    bool           `json:"isActive"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
}

// toLegacy 将新模型转换为 v1 结构，文件路径替换为下载令牌。
func (h *Handler) toLegacy(v *model.Version, withToken bool) legacyVersion {
	filePath := v.FileKey
	if withToken && v.FileKey != "" {
		filePath = h.svc.File.SignDownload(v.FileKey)
	}
	return legacyVersion{
		ID:          v.ID,
		AppID:       v.AppID,
		Platform:    v.Platform,
		Version:     v.Version,
		FilePath:    filePath,
		FileName:    v.FileName,
		FileSize:    v.FileSize,
		Changelog:   v.Changelog,
		Ext:         v.Ext,
		ForceUpdate: v.ForceUpdate,
		IsActive:    v.Status == model.VersionPublished,
		CreatedAt:   v.CreatedAt,
		UpdatedAt:   v.UpdatedAt,
	}
}

// ---------- 认证 ----------

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Login 登录（v1 响应结构）。
func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数"})
		return
	}
	ctx := service.WithActor(c.Request.Context(), service.Actor{
		IP:        c.ClientIP(),
		UserAgent: c.GetHeader("User-Agent"),
	})
	result, err := h.svc.Auth.Login(ctx, req.Username, req.Password)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"token": result.AccessToken,
		"user": gin.H{
			"id":       result.User.ID,
			"username": result.User.Username,
			"role":     result.User.Role,
		},
	})
}

// ---------- 开放接口 ----------

// GetLatestVersion 获取最新版本（v1 响应结构，支持 format 模板输出）。
func (h *Handler) GetLatestVersion(c *gin.Context) {
	identifier := strings.TrimSpace(c.Query("identifier"))
	platform := model.Platform(common.ClientPlatform(c))

	app, version, err := h.svc.Version.LookupForOpen(c.Request.Context(), identifier, platform, c.Query("channel"))
	if err != nil {
		fail(c, err)
		return
	}

	// 模板输出：与历史行为一致，先把文件路径替换为下载令牌。
	format := strings.TrimSpace(c.Query("format"))
	if format != "" {
		tpl, err := h.svc.Template.GetByName(c.Request.Context(), app.ID, format)
		if err != nil {
			fail(c, err)
			return
		}
		preview := *version
		preview.FileKey = h.svc.File.SignDownload(version.FileKey)
		content, contentType, err := h.svc.Template.Render(tpl.Name, tpl.Content, app, &preview)
		if err != nil {
			fail(c, err)
			return
		}
		c.Header("Content-Type", contentType)
		c.String(http.StatusOK, content)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"appName":    app.Name,
		"appId":      app.ID,
		"identifier": app.Identifier,
		"version":    version.Version,
		"platform":   version.Platform,
		"channel":    version.Channel,
		"changelog":  version.Changelog,
		"isForce":    version.ForceUpdate,
		"fileName":   version.FileName,
		"filePath":   h.svc.File.SignDownload(version.FileKey),
		"fileSize":   version.FileSize,
		"createdAt":  version.CreatedAt.Format(time.RFC3339),
	})
}

// GetChangelog 更新日志（v1 响应结构）。
func (h *Handler) GetChangelog(c *gin.Context) {
	identifier := strings.TrimSpace(c.Query("identifier"))
	app, err := h.svc.App.GetByIdentifier(c.Request.Context(), identifier)
	if err != nil {
		fail(c, err)
		return
	}
	platform := model.Platform(strings.TrimSpace(c.Query("platform")))
	versions, err := h.svc.Version.Changelog(c.Request.Context(), app, platform, c.Query("channel"), 0)
	if err != nil {
		fail(c, err)
		return
	}

	type changelogItem struct {
		CreatedAt string         `json:"createdAt"`
		Version   string         `json:"version"`
		Platform  model.Platform `json:"platform"`
		Changelog string         `json:"changelog"`
	}
	items := make([]changelogItem, 0, len(versions))
	for i := range versions {
		items = append(items, changelogItem{
			CreatedAt: versions[i].CreatedAt.Format(time.RFC3339),
			Version:   versions[i].Version,
			Platform:  versions[i].Platform,
			Changelog: versions[i].Changelog,
		})
	}
	c.JSON(http.StatusOK, items)
}

// CheckUpdate 检测更新（v1 风格，字段与 /open/latest 保持一致的命名习惯）。
func (h *Handler) CheckUpdate(c *gin.Context) {
	identifier := strings.TrimSpace(c.Query("identifier"))
	platform := model.Platform(strings.TrimSpace(c.Query("platform")))
	if platform == "" {
		platform = model.Platform(common.ClientPlatform(c))
	}

	result, err := h.svc.Version.Check(c.Request.Context(), identifier, platform, c.Query("channel"), c.Query("currentVersion"))
	if err != nil {
		fail(c, err)
		return
	}

	latest := result.Latest
	c.JSON(http.StatusOK, gin.H{
		"identifier":          result.Identifier,
		"platform":            result.Platform,
		"channel":             result.Channel,
		"currentVersion":      result.CurrentVersion,
		"hasUpdate":           result.HasUpdate,
		"isForce":             result.ForceUpdate,
		"belowMinimumVersion": result.BelowMinimumVersion,
		"minSupportedVersion": result.MinSupportedVersion,
		"version":             latest.Version,
		"changelog":           latest.Changelog,
		"fileName":            latest.FileName,
		"filePath":            strings.TrimPrefix(latest.DownloadURL, "/api/open/download/"),
		"fileSize":            latest.FileSize,
		"createdAt":           formatTime(latest.PublishedAt),
	})
}

// SecureDownload 令牌下载（v1 行为）。
func (h *Handler) SecureDownload(c *gin.Context) {
	if err := common.ServeDownload(c, h.svc.File, h.svc.Signer(), c.Param("token")); err != nil {
		fail(c, err)
	}
}

// ---------- 分享 ----------

type accessShareRequest struct {
	Password string `json:"password"`
}

// GetSharedApp 获取分享的应用信息（v1 响应结构）。
func (h *Handler) GetSharedApp(c *gin.Context) {
	var req accessShareRequest
	_ = c.ShouldBindJSON(&req)

	_, app, err := h.svc.Share.Authorize(c.Request.Context(), c.Param("token"), req.Password)
	if err != nil {
		fail(c, err)
		return
	}

	versions, err := h.svc.Share.LatestPerPlatform(c.Request.Context(), app, c.Query("channel"))
	if err != nil {
		fail(c, err)
		return
	}

	list := make([]legacyVersion, 0, len(versions))
	for i := range versions {
		list = append(list, h.toLegacy(&versions[i], true))
	}

	c.JSON(http.StatusOK, gin.H{
		"app": gin.H{
			"name":        app.Name,
			"identifier":  app.Identifier,
			"logo":        app.Logo,
			"description": app.Description,
			"platforms":   app.Platforms.Strings(),
		},
		"versions":        list,
		"currentPlatform": common.ClientPlatform(c),
	})
}

// GetSharedAppVersions 获取分享应用的版本列表（v1 响应结构）。
func (h *Handler) GetSharedAppVersions(c *gin.Context) {
	var req accessShareRequest
	_ = c.ShouldBindJSON(&req)

	share, _, err := h.svc.Share.Authorize(c.Request.Context(), c.Param("token"), req.Password)
	if err != nil {
		fail(c, err)
		return
	}

	page := model.PageQuery{}
	if v := c.Query("page"); v != "" {
		if n, err := parseInt(v); err == nil {
			page.Page = n
		}
	}
	if v := c.Query("pageSize"); v != "" {
		if n, err := parseInt(v); err == nil {
			page.PageSize = n
		}
	}
	if page.Page == 0 && page.PageSize == 0 {
		page.Page, page.PageSize = 1, 99
	}
	page.Normalize()

	platform := model.Platform(strings.TrimSpace(c.Query("platform")))
	versions, total, err := h.svc.Share.Versions(c.Request.Context(), share, platform, page)
	if err != nil {
		fail(c, err)
		return
	}

	list := make([]legacyVersion, 0, len(versions))
	for i := range versions {
		list = append(list, h.toLegacy(&versions[i], true))
	}

	c.JSON(http.StatusOK, gin.H{
		"total":    total,
		"list":     list,
		"page":     page.Page,
		"pageSize": page.PageSize,
	})
}

// formatTime 输出 RFC3339 时间，nil 返回空字符串。
func formatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}
