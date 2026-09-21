package v2

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"app_version_manage/internal/apierr"
	"app_version_manage/internal/model"
	"app_version_manage/internal/repository"
	"app_version_manage/internal/service"
)

// ---------- 应用 ----------

// ListApps 应用列表。
func (h *Handler) ListApps(c *gin.Context) {
	page := h.pageQuery(c)
	apps, total, err := h.svc.App.List(c.Request.Context(), c.Query("keyword"), page)
	if err != nil {
		apierr.Fail(c, err)
		return
	}
	apierr.OK(c, model.NewPaginated(apps, total, page.Page, page.PageSize))
}

// CreateApp 创建应用。
func (h *Handler) CreateApp(c *gin.Context) {
	var req service.AppInput
	if !h.bindJSON(c, &req) {
		return
	}
	app, err := h.svc.App.Create(c.Request.Context(), req)
	if err != nil {
		apierr.Fail(c, err)
		return
	}
	apierr.Created(c, app)
}

// GetApp 应用详情。
func (h *Handler) GetApp(c *gin.Context) {
	id, ok := h.pathUint(c, "id")
	if !ok {
		return
	}
	app, err := h.svc.App.Get(c.Request.Context(), id)
	if err != nil {
		apierr.Fail(c, err)
		return
	}
	apierr.OK(c, app)
}

// UpdateApp 更新应用。
func (h *Handler) UpdateApp(c *gin.Context) {
	id, ok := h.pathUint(c, "id")
	if !ok {
		return
	}
	var req service.AppInput
	if !h.bindJSON(c, &req) {
		return
	}
	app, err := h.svc.App.Update(c.Request.Context(), id, req)
	if err != nil {
		apierr.Fail(c, err)
		return
	}
	apierr.OK(c, app)
}

// DeleteApp 删除应用。
func (h *Handler) DeleteApp(c *gin.Context) {
	id, ok := h.pathUint(c, "id")
	if !ok {
		return
	}
	if err := h.svc.App.Delete(c.Request.Context(), id); err != nil {
		apierr.Fail(c, err)
		return
	}
	apierr.OK(c, gin.H{"message": "应用已删除"})
}

// ---------- 通道 ----------

// ListChannels 通道列表。
func (h *Handler) ListChannels(c *gin.Context) {
	appID, ok := h.pathUint(c, "id")
	if !ok {
		return
	}
	channels, err := h.svc.App.Channels(c.Request.Context(), appID)
	if err != nil {
		apierr.Fail(c, err)
		return
	}
	apierr.OK(c, channels)
}

// CreateChannel 创建通道。
func (h *Handler) CreateChannel(c *gin.Context) {
	appID, ok := h.pathUint(c, "id")
	if !ok {
		return
	}
	var req service.ChannelInput
	if !h.bindJSON(c, &req) {
		return
	}
	channel, err := h.svc.App.CreateChannel(c.Request.Context(), appID, req)
	if err != nil {
		apierr.Fail(c, err)
		return
	}
	apierr.Created(c, channel)
}

// UpdateChannel 更新通道。
func (h *Handler) UpdateChannel(c *gin.Context) {
	appID, ok := h.pathUint(c, "id")
	if !ok {
		return
	}
	channelID, ok := h.pathUint(c, "channelId")
	if !ok {
		return
	}
	var req service.ChannelInput
	if !h.bindJSON(c, &req) {
		return
	}
	channel, err := h.svc.App.UpdateChannel(c.Request.Context(), appID, channelID, req)
	if err != nil {
		apierr.Fail(c, err)
		return
	}
	apierr.OK(c, channel)
}

// DeleteChannel 删除通道。
func (h *Handler) DeleteChannel(c *gin.Context) {
	appID, ok := h.pathUint(c, "id")
	if !ok {
		return
	}
	channelID, ok := h.pathUint(c, "channelId")
	if !ok {
		return
	}
	if err := h.svc.App.DeleteChannel(c.Request.Context(), appID, channelID); err != nil {
		apierr.Fail(c, err)
		return
	}
	apierr.OK(c, gin.H{"message": "通道已删除"})
}

type setDefaultChannelRequest struct {
	Key string `json:"key" binding:"required"`
}

// SetDefaultChannel 设置默认通道。
func (h *Handler) SetDefaultChannel(c *gin.Context) {
	appID, ok := h.pathUint(c, "id")
	if !ok {
		return
	}
	var req setDefaultChannelRequest
	if !h.bindJSON(c, &req) {
		return
	}
	if err := h.svc.App.SetDefaultChannel(c.Request.Context(), appID, req.Key); err != nil {
		apierr.Fail(c, err)
		return
	}
	apierr.OK(c, gin.H{"message": "默认通道已更新"})
}

// ---------- 版本 ----------

// ListVersions 版本列表。
func (h *Handler) ListVersions(c *gin.Context) {
	appID, err := strconv.ParseUint(c.Query("appId"), 10, 64)
	if err != nil {
		apierr.Fail(c, apierr.BadRequest("appId 不合法"))
		return
	}

	filter := repository.VersionFilter{
		AppID:    uint(appID),
		Platform: model.Platform(c.Query("platform")),
		Channel:  c.Query("channel"),
		Status:   model.VersionStatus(c.Query("status")),
		Keyword:  c.Query("keyword"),
	}
	page := h.pageQuery(c)
	versions, total, err := h.svc.Version.List(c.Request.Context(), filter, page)
	if err != nil {
		apierr.Fail(c, err)
		return
	}
	apierr.OK(c, model.NewPaginated(versions, total, page.Page, page.PageSize))
}

type publishVersionRequest struct {
	AppID               uint                `json:"appId" binding:"required"`
	Platform            model.Platform      `json:"platform" binding:"required"`
	Channel             string              `json:"channel"`
	Version             string              `json:"version" binding:"required"`
	FileKey             string              `json:"fileKey" binding:"required"`
	FileName            string              `json:"fileName"`
	FileSize            int64               `json:"fileSize"`
	FileSHA256          string              `json:"fileSha256"`
	ContentType         string              `json:"contentType"`
	Changelog           string              `json:"changelog"`
	Ext                 string              `json:"ext"`
	ForceUpdate         bool                `json:"forceUpdate"`
	MinSupportedVersion string              `json:"minSupportedVersion"`
	Status              model.VersionStatus `json:"status"`
}

// PublishVersion 发布版本。
func (h *Handler) PublishVersion(c *gin.Context) {
	var req publishVersionRequest
	if !h.bindJSON(c, &req) {
		return
	}
	version, err := h.svc.Version.Publish(c.Request.Context(), service.PublishInput{
		AppID:               req.AppID,
		Platform:            req.Platform,
		Channel:             req.Channel,
		Version:             req.Version,
		FileKey:             req.FileKey,
		FileName:            req.FileName,
		FileSize:            req.FileSize,
		FileSHA256:          req.FileSHA256,
		ContentType:         req.ContentType,
		Changelog:           req.Changelog,
		Ext:                 req.Ext,
		ForceUpdate:         req.ForceUpdate,
		MinSupportedVersion: req.MinSupportedVersion,
		Status:              req.Status,
	})
	if err != nil {
		apierr.Fail(c, err)
		return
	}
	apierr.Created(c, version)
}

type updateVersionRequest struct {
	Platform            *model.Platform      `json:"platform"`
	Channel             *string              `json:"channel"`
	Version             *string              `json:"version"`
	Changelog           *string              `json:"changelog"`
	Ext                 *string              `json:"ext"`
	ForceUpdate         *bool                `json:"forceUpdate"`
	MinSupportedVersion *string              `json:"minSupportedVersion"`
	Status              *model.VersionStatus `json:"status"`
	FileKey             *string              `json:"fileKey"`
	FileName            *string              `json:"fileName"`
	FileSize            *int64               `json:"fileSize"`
	FileSHA256          *string              `json:"fileSha256"`
	ContentType         *string              `json:"contentType"`
}

// UpdateVersion 更新版本。
func (h *Handler) UpdateVersion(c *gin.Context) {
	id, ok := h.pathUint(c, "id")
	if !ok {
		return
	}
	var req updateVersionRequest
	if !h.bindJSON(c, &req) {
		return
	}
	version, err := h.svc.Version.Update(c.Request.Context(), id, service.VersionUpdateInput{
		Platform:            req.Platform,
		Channel:             req.Channel,
		Version:             req.Version,
		Changelog:           req.Changelog,
		Ext:                 req.Ext,
		ForceUpdate:         req.ForceUpdate,
		MinSupportedVersion: req.MinSupportedVersion,
		Status:              req.Status,
		FileKey:             req.FileKey,
		FileName:            req.FileName,
		FileSize:            req.FileSize,
		FileSHA256:          req.FileSHA256,
		ContentType:         req.ContentType,
	})
	if err != nil {
		apierr.Fail(c, err)
		return
	}
	apierr.OK(c, version)
}

// GetVersion 版本详情。
func (h *Handler) GetVersion(c *gin.Context) {
	id, ok := h.pathUint(c, "id")
	if !ok {
		return
	}
	version, err := h.svc.Version.Get(c.Request.Context(), id)
	if err != nil {
		apierr.Fail(c, err)
		return
	}
	apierr.OK(c, version)
}

type versionStatusRequest struct {
	Status model.VersionStatus `json:"status" binding:"required"`
}

// SetVersionStatus 上架/下架版本。
func (h *Handler) SetVersionStatus(c *gin.Context) {
	id, ok := h.pathUint(c, "id")
	if !ok {
		return
	}
	var req versionStatusRequest
	if !h.bindJSON(c, &req) {
		return
	}
	version, err := h.svc.Version.SetStatus(c.Request.Context(), id, req.Status)
	if err != nil {
		apierr.Fail(c, err)
		return
	}
	apierr.OK(c, version)
}

// DeleteVersion 删除版本。
func (h *Handler) DeleteVersion(c *gin.Context) {
	id, ok := h.pathUint(c, "id")
	if !ok {
		return
	}
	if err := h.svc.Version.Delete(c.Request.Context(), id); err != nil {
		apierr.Fail(c, err)
		return
	}
	apierr.OK(c, gin.H{"message": "版本已删除"})
}
