package v2

import (
	"strings"

	"github.com/gin-gonic/gin"

	"app_version_manage/internal/api/common"
	"app_version_manage/internal/apierr"
	"app_version_manage/internal/model"
	"app_version_manage/internal/service"
)

// ---------- 文件 ----------

// UploadFile 上传文件。
func (h *Handler) UploadFile(c *gin.Context) {
	header, err := c.FormFile("file")
	if err != nil {
		apierr.Fail(c, apierr.BadRequest("未接收到上传文件"))
		return
	}
	result, err := h.svc.File.Upload(c.Request.Context(), header)
	if err != nil {
		apierr.Fail(c, err)
		return
	}
	apierr.OK(c, result)
}

// ListFiles 文件列表。
func (h *Handler) ListFiles(c *gin.Context) {
	page := h.pageQuery(c)
	files, total, err := h.svc.File.List(c.Request.Context(), c.Query("keyword"), page)
	if err != nil {
		apierr.Fail(c, err)
		return
	}
	apierr.OK(c, model.NewPaginated(files, total, page.Page, page.PageSize))
}

// DeleteFile 删除文件。
func (h *Handler) DeleteFile(c *gin.Context) {
	id, ok := h.pathUint(c, "id")
	if !ok {
		return
	}
	if err := h.svc.File.Delete(c.Request.Context(), id); err != nil {
		apierr.Fail(c, err)
		return
	}
	apierr.OK(c, gin.H{"message": "文件已删除"})
}

// CleanFiles 清理未使用文件。
func (h *Handler) CleanFiles(c *gin.Context) {
	removed, err := h.svc.File.CleanUnused(c.Request.Context())
	if err != nil {
		apierr.Fail(c, err)
		return
	}
	apierr.OK(c, gin.H{"removed": removed, "message": "清理完成"})
}

// DownloadFileByID 通过文件 ID 下载（需登录，内部生成签名链接）。
func (h *Handler) DownloadFileByID(c *gin.Context) {
	id, ok := h.pathUint(c, "id")
	if !ok {
		return
	}
	file, err := h.svc.File.GetByID(c.Request.Context(), id)
	if err != nil {
		apierr.Fail(c, err)
		return
	}
	token := h.svc.File.SignDownload(file.Key)
	if err := common.ServeDownload(c, h.svc.File, h.svc.Signer(), token); err != nil {
		apierr.Fail(c, err)
	}
}

// DownloadVersionByID 通过版本 ID 下载产物。
func (h *Handler) DownloadVersionByID(c *gin.Context) {
	id, ok := h.pathUint(c, "id")
	if !ok {
		return
	}
	version, err := h.svc.Version.Get(c.Request.Context(), id)
	if err != nil {
		apierr.Fail(c, err)
		return
	}
	token := h.svc.File.SignDownload(version.FileKey)
	if err := common.ServeDownload(c, h.svc.File, h.svc.Signer(), token); err != nil {
		apierr.Fail(c, err)
	}
}

// ---------- 分享 ----------

// ListShares 分享列表。
func (h *Handler) ListShares(c *gin.Context) {
	appID, ok := h.pathUint(c, "id")
	if !ok {
		return
	}
	shares, err := h.svc.Share.List(c.Request.Context(), appID)
	if err != nil {
		apierr.Fail(c, err)
		return
	}
	apierr.OK(c, shares)
}

type createShareRequest struct {
	Password      string `json:"password"`
	ExpiresInDays int    `json:"expiresInDays"`
}

// CreateShare 创建分享。
func (h *Handler) CreateShare(c *gin.Context) {
	appID, ok := h.pathUint(c, "id")
	if !ok {
		return
	}
	var req createShareRequest
	if !h.bindJSON(c, &req) {
		return
	}
	share, err := h.svc.Share.Create(c.Request.Context(), appID, service.ShareInput{
		Password:      req.Password,
		ExpiresInDays: req.ExpiresInDays,
	})
	if err != nil {
		apierr.Fail(c, err)
		return
	}
	apierr.Created(c, share)
}

type updateShareRequest struct {
	Password      *string `json:"password"`
	ExpiresInDays *int    `json:"expiresInDays"`
}

// UpdateShare 更新分享。
func (h *Handler) UpdateShare(c *gin.Context) {
	appID, ok := h.pathUint(c, "id")
	if !ok {
		return
	}
	shareID, ok := h.pathUint(c, "shareId")
	if !ok {
		return
	}
	var req updateShareRequest
	if !h.bindJSON(c, &req) {
		return
	}
	share, err := h.svc.Share.Update(c.Request.Context(), appID, shareID, service.UpdateShareInput{
		Password:      req.Password,
		ExpiresInDays: req.ExpiresInDays,
	})
	if err != nil {
		apierr.Fail(c, err)
		return
	}
	apierr.OK(c, share)
}

// DeactivateShare 禁用分享。
func (h *Handler) DeactivateShare(c *gin.Context) {
	appID, ok := h.pathUint(c, "id")
	if !ok {
		return
	}
	shareID, ok := h.pathUint(c, "shareId")
	if !ok {
		return
	}
	if err := h.svc.Share.Deactivate(c.Request.Context(), appID, shareID); err != nil {
		apierr.Fail(c, err)
		return
	}
	apierr.OK(c, gin.H{"message": "分享已禁用"})
}

// DeleteShare 删除分享。
func (h *Handler) DeleteShare(c *gin.Context) {
	appID, ok := h.pathUint(c, "id")
	if !ok {
		return
	}
	shareID, ok := h.pathUint(c, "shareId")
	if !ok {
		return
	}
	if err := h.svc.Share.Delete(c.Request.Context(), appID, shareID); err != nil {
		apierr.Fail(c, err)
		return
	}
	apierr.OK(c, gin.H{"message": "分享已删除"})
}

// ---------- 模板 ----------

// ListTemplates 模板列表。
func (h *Handler) ListTemplates(c *gin.Context) {
	appID, ok := h.pathUint(c, "id")
	if !ok {
		return
	}
	templates, err := h.svc.Template.List(c.Request.Context(), appID)
	if err != nil {
		apierr.Fail(c, err)
		return
	}
	apierr.OK(c, templates)
}

type templateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Content     string `json:"content"`
}

// CreateTemplate 创建模板。
func (h *Handler) CreateTemplate(c *gin.Context) {
	appID, ok := h.pathUint(c, "id")
	if !ok {
		return
	}
	var req templateRequest
	if !h.bindJSON(c, &req) {
		return
	}
	tpl, err := h.svc.Template.Create(c.Request.Context(), appID, service.TemplateInput{
		Name: req.Name, Description: req.Description, Content: req.Content,
	})
	if err != nil {
		apierr.Fail(c, err)
		return
	}
	apierr.Created(c, tpl)
}

// UpdateTemplate 更新模板。
func (h *Handler) UpdateTemplate(c *gin.Context) {
	appID, ok := h.pathUint(c, "id")
	if !ok {
		return
	}
	templateID, ok := h.pathUint(c, "templateId")
	if !ok {
		return
	}
	var req templateRequest
	if !h.bindJSON(c, &req) {
		return
	}
	tpl, err := h.svc.Template.Update(c.Request.Context(), appID, templateID, service.TemplateInput{
		Name: req.Name, Description: req.Description, Content: req.Content,
	})
	if err != nil {
		apierr.Fail(c, err)
		return
	}
	apierr.OK(c, tpl)
}

// DeleteTemplate 删除模板。
func (h *Handler) DeleteTemplate(c *gin.Context) {
	appID, ok := h.pathUint(c, "id")
	if !ok {
		return
	}
	templateID, ok := h.pathUint(c, "templateId")
	if !ok {
		return
	}
	if err := h.svc.Template.Delete(c.Request.Context(), appID, templateID); err != nil {
		apierr.Fail(c, err)
		return
	}
	apierr.OK(c, gin.H{"message": "模板已删除"})
}

type previewTemplateRequest struct {
	Platform string `json:"platform"`
	Channel  string `json:"channel"`
}

// PreviewTemplate 预览模板渲染结果。
func (h *Handler) PreviewTemplate(c *gin.Context) {
	appID, ok := h.pathUint(c, "id")
	if !ok {
		return
	}
	templateID, ok := h.pathUint(c, "templateId")
	if !ok {
		return
	}
	var req previewTemplateRequest
	_ = c.ShouldBindJSON(&req)

	app, err := h.svc.App.Get(c.Request.Context(), appID)
	if err != nil {
		apierr.Fail(c, err)
		return
	}
	platform := model.Platform(strings.TrimSpace(req.Platform))
	if platform == "" && len(app.Platforms) > 0 {
		platform = app.Platforms[0]
	}
	if !platform.Valid() {
		apierr.Fail(c, apierr.BadRequest("预览平台不合法"))
		return
	}

	tpl, err := h.svc.Template.Get(c.Request.Context(), appID, templateID)
	if err != nil {
		apierr.Fail(c, err)
		return
	}
	version, err := h.svc.Version.Latest(c.Request.Context(), app, platform, req.Channel)
	if err != nil {
		apierr.Fail(c, err)
		return
	}

	// 预览时把文件路径替换为下载令牌，与真实开放接口输出保持一致。
	preview := *version
	preview.FileKey = h.svc.File.SignDownload(version.FileKey)

	content, contentType, err := h.svc.Template.Render(tpl.Name, tpl.Content, app, &preview)
	if err != nil {
		apierr.Fail(c, err)
		return
	}
	apierr.OK(c, gin.H{
		"contentType": contentType,
		"content":     content,
		"platform":    platform,
		"version":     version.Version,
	})
}
