package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"path"
	"strings"
	"text/template"
	"time"

	"app_version_manage/internal/apierr"
	"app_version_manage/internal/model"
	"app_version_manage/internal/storage"
)

// ---------- 文件 ----------

// FileService 文件存储与下载。
type FileService struct{ base }

// UploadResult 上传结果。
type UploadResult struct {
	File         *model.File `json:"file"`
	Deduplicated bool        `json:"deduplicated"`
}

// Upload 处理文件上传：校验 -> 哈希 -> 秒传/去重 -> 落存储 -> 记录。
func (s *FileService) Upload(ctx context.Context, header *multipart.FileHeader) (*UploadResult, error) {
	if header == nil {
		return nil, apierr.BadRequest("未接收到上传文件")
	}
	maxBytes := int64(s.cfg.Upload.MaxSizeMB) << 20
	if header.Size > maxBytes {
		return nil, apierr.PayloadTooLarge("文件超过大小限制")
	}
	if err := s.checkExtension(header.Filename); err != nil {
		return nil, err
	}

	sha256Hex, size, err := hashUpload(header)
	if err != nil {
		return nil, err
	}

	// 秒传：同内容文件已存在且对象仍在存储中。
	if existing, err := s.store.GetFileBySHA256(ctx, sha256Hex); err == nil {
		if _, statErr := s.storage.Stat(ctx, existing.Key); statErr == nil {
			return &UploadResult{File: existing, Deduplicated: true}, nil
		}
	} else if !errors.Is(err, errRecordNotFound()) {
		return nil, internal("查询文件失败", err)
	}

	key := storage.BuildKey(sha256Hex, header.Filename)
	contentType := header.Header.Get("Content-Type")

	src, err := header.Open()
	if err != nil {
		return nil, internal("读取上传文件失败", err)
	}
	defer src.Close()

	obj, err := s.storage.Put(ctx, key, src, size, contentType, sha256Hex)
	if err != nil {
		return nil, internal("保存文件失败", err)
	}

	file := &model.File{
		Name:        storage.SanitizeFileName(header.Filename),
		Key:         obj.Key,
		Size:        obj.Size,
		ContentType: contentType,
		SHA256:      sha256Hex,
		Storage:     s.storage.Driver(),
		CreatedBy:   ActorFrom(ctx).UserID,
	}
	if err := s.store.CreateFile(ctx, file); err != nil {
		// 记录失败时回滚已写入的对象，避免产生游离文件。
		_ = s.storage.Delete(ctx, obj.Key)
		return nil, internal("保存文件记录失败", err)
	}

	s.audit(ctx, "file.upload", "file", itoa(file.ID), "上传文件 "+file.Name,
		map[string]any{"sha256": sha256Hex, "size": file.Size}, true)
	return &UploadResult{File: file}, nil
}

// List 分页查询文件。
func (s *FileService) List(ctx context.Context, keyword string, page model.PageQuery) ([]model.File, int64, error) {
	page.Normalize()
	files, total, err := s.store.ListFiles(ctx, keyword, page)
	if err != nil {
		return nil, 0, internal("查询文件列表失败", err)
	}
	return files, total, nil
}

// Delete 删除文件记录与对象；仍被引用的文件不允许删除。
func (s *FileService) Delete(ctx context.Context, id uint) error {
	file, err := s.store.GetFileByID(ctx, id)
	if err != nil {
		return notFoundOr(err, "文件不存在")
	}
	inUse, err := s.isReferenced(ctx, file.Key)
	if err != nil {
		return err
	}
	if inUse {
		return apierr.Conflict("该文件仍被版本或应用图标引用，无法删除")
	}
	if err := s.storage.Delete(ctx, file.Key); err != nil {
		s.log.Warn("删除存储对象失败", "key", file.Key, "error", err)
	}
	if err := s.store.DeleteFile(ctx, id); err != nil {
		return internal("删除文件记录失败", err)
	}
	s.audit(ctx, "file.delete", "file", itoa(id), "删除文件 "+file.Name, nil, true)
	return nil
}

// CleanUnused 清理未被任何版本（含已删除版本）或应用图标引用的文件。
func (s *FileService) CleanUnused(ctx context.Context) (int, error) {
	versionKeys, err := s.store.ReferencedVersionKeysIncludingDeleted(ctx)
	if err != nil {
		return 0, internal("查询版本引用失败", err)
	}
	logoKeys, err := s.store.ReferencedLogoKeys(ctx)
	if err != nil {
		return 0, internal("查询图标引用失败", err)
	}

	inUse := map[string]bool{}
	for _, k := range append(versionKeys, logoKeys...) {
		if k != "" {
			inUse[k] = true
		}
	}

	files, err := s.store.AllFiles(ctx)
	if err != nil {
		return 0, internal("查询文件列表失败", err)
	}

	removed := 0
	var ids []uint
	for _, f := range files {
		if inUse[f.Key] {
			continue
		}
		if err := s.storage.Delete(ctx, f.Key); err != nil {
			s.log.Warn("删除孤儿对象失败", "key", f.Key, "error", err)
		}
		ids = append(ids, f.ID)
		removed++
	}
	if err := s.store.DeleteFilesByIDs(ctx, ids); err != nil {
		return removed, internal("删除文件记录失败", err)
	}

	s.audit(ctx, "file.clean", "file", "", "清理未使用文件", map[string]any{"removed": removed}, true)
	return removed, nil
}

// DownloadTarget 下载目标：重定向地址或本地流。
type DownloadTarget struct {
	RedirectURL string
	Reader      io.ReadCloser
	FileName    string
	ContentType string
	Size        int64
}

// OpenDownload 解析下载目标，S3 走预签名直连，本地存储走服务端流式代理。
func (s *FileService) OpenDownload(ctx context.Context, key string) (*DownloadTarget, error) {
	cleanKey, err := storage.NormalizeKey(key)
	if err != nil {
		return nil, apierr.BadRequest("文件路径不合法")
	}

	obj, err := s.storage.Stat(ctx, cleanKey)
	if err != nil {
		return nil, apierr.NotFound("文件不存在")
	}

	target := &DownloadTarget{
		FileName:    path.Base(cleanKey),
		ContentType: obj.ContentType,
		Size:        obj.Size,
	}
	if file, err := s.store.GetFileByKey(ctx, cleanKey); err == nil {
		target.FileName = file.Name
		if file.ContentType != "" {
			target.ContentType = file.ContentType
		}
	}
	if target.ContentType == "" {
		target.ContentType = "application/octet-stream"
	}

	ttl := time.Duration(s.cfg.Storage.SignedURLTTLMinutes) * time.Minute
	if url, err := s.storage.PresignGet(ctx, cleanKey, target.FileName, ttl); err == nil && url != "" {
		target.RedirectURL = url
		return target, nil
	}

	reader, err := s.storage.Open(ctx, cleanKey)
	if err != nil {
		return nil, apierr.NotFound("文件不存在")
	}
	target.Reader = reader
	return target, nil
}

// RecordDownload 记录版本下载次数。
func (s *FileService) RecordDownload(ctx context.Context, key string) {
	var version model.Version
	if err := s.store.DB().WithContext(ctx).Where("file_path = ?", key).First(&version).Error; err != nil {
		return
	}
	if err := s.store.IncrementDownloadCount(ctx, version.ID); err != nil {
		s.log.Warn("更新下载次数失败", "versionId", version.ID, "error", err)
	}
}

// checkExtension 校验扩展名白名单。
func (s *FileService) checkExtension(filename string) error {
	allowed := s.cfg.Upload.AllowedExtensions
	if len(allowed) == 0 {
		return nil
	}
	ext := strings.ToLower(path.Ext(filename))
	for _, item := range allowed {
		if ext == strings.ToLower(strings.TrimSpace(item)) {
			return nil
		}
	}
	return apierr.BadRequest("不支持的文件类型: " + ext)
}

// isReferenced 判断对象键是否仍被版本或应用图标引用。
func (s *FileService) isReferenced(ctx context.Context, key string) (bool, error) {
	versionKeys, err := s.store.ReferencedVersionKeysIncludingDeleted(ctx)
	if err != nil {
		return false, internal("查询版本引用失败", err)
	}
	logoKeys, err := s.store.ReferencedLogoKeys(ctx)
	if err != nil {
		return false, internal("查询图标引用失败", err)
	}
	for _, k := range append(versionKeys, logoKeys...) {
		if k == key {
			return true, nil
		}
	}
	return false, nil
}

func hashUpload(header *multipart.FileHeader) (string, int64, error) {
	src, err := header.Open()
	if err != nil {
		return "", 0, internal("读取上传文件失败", err)
	}
	defer src.Close()

	sha, size, err := storage.HashReader(src)
	if err != nil {
		return "", 0, internal("计算文件哈希失败", err)
	}
	return sha, size, nil
}

// ---------- 分享 ----------

// ShareService 分享链接管理。
type ShareService struct{ base }

// ShareInput 创建分享入参。
type ShareInput struct {
	Password      string
	ExpiresInDays int
}

// UpdateShareInput 更新分享入参，nil 表示不修改。
type UpdateShareInput struct {
	Password      *string
	ExpiresInDays *int
}

// ShareView 分享对外视图。
type ShareView struct {
	ID          uint       `json:"id"`
	Token       string     `json:"token"`
	HasPassword bool       `json:"hasPassword"`
	ExpiresAt   *time.Time `json:"expiresAt"`
	IsActive    bool       `json:"isActive"`
	AccessCount int64      `json:"accessCount"`
	CreatedAt   time.Time  `json:"createdAt"`
}

func shareView(share *model.Share) ShareView {
	return ShareView{
		ID:          share.ID,
		Token:       share.Token,
		HasPassword: share.PasswordHash != "",
		ExpiresAt:   share.ExpiresAt,
		IsActive:    share.IsActive,
		AccessCount: share.AccessCount,
		CreatedAt:   share.CreatedAt,
	}
}

// Create 生成分享链接。
func (s *ShareService) Create(ctx context.Context, appID uint, in ShareInput) (*ShareView, error) {
	if _, err := (&AppService{base: s.base}).Get(ctx, appID); err != nil {
		return nil, err
	}

	tok, err := randomToken(24)
	if err != nil {
		return nil, err
	}

	share := &model.Share{
		AppID:     appID,
		Token:     tok,
		IsActive:  true,
		CreatedBy: ActorFrom(ctx).UserID,
	}
	if strings.TrimSpace(in.Password) != "" {
		hash, err := s.hashSecret(in.Password)
		if err != nil {
			return nil, err
		}
		share.PasswordHash = hash
		share.PasswordAlgo = "bcrypt"
	}
	if in.ExpiresInDays > 0 {
		expiresAt := time.Now().AddDate(0, 0, in.ExpiresInDays)
		share.ExpiresAt = &expiresAt
	}

	if err := s.store.CreateShare(ctx, share); err != nil {
		return nil, internal("创建分享失败", err)
	}

	s.audit(ctx, "share.create", "share", itoa(share.ID), "创建分享链接",
		map[string]any{"appId": appID, "hasPassword": share.PasswordHash != ""}, true)
	view := shareView(share)
	return &view, nil
}

// List 查询应用下的分享。
func (s *ShareService) List(ctx context.Context, appID uint) ([]ShareView, error) {
	if _, err := (&AppService{base: s.base}).Get(ctx, appID); err != nil {
		return nil, err
	}
	shares, err := s.store.ListShares(ctx, appID)
	if err != nil {
		return nil, internal("查询分享列表失败", err)
	}
	out := make([]ShareView, 0, len(shares))
	for i := range shares {
		out = append(out, shareView(&shares[i]))
	}
	return out, nil
}

// Update 更新分享密码或有效期。
func (s *ShareService) Update(ctx context.Context, appID, shareID uint, in UpdateShareInput) (*ShareView, error) {
	share, err := s.store.GetAppShare(ctx, appID, shareID)
	if err != nil {
		return nil, notFoundOr(err, "分享不存在")
	}

	fields := map[string]any{}
	_ = share
	if in.Password != nil {
		if strings.TrimSpace(*in.Password) == "" {
			fields["password_hash"] = ""
			fields["password_algo"] = ""
			fields["password"] = ""
		} else {
			hash, err := s.hashSecret(*in.Password)
			if err != nil {
				return nil, err
			}
			fields["password_hash"] = hash
			fields["password_algo"] = "bcrypt"
			fields["password"] = ""
		}
	}
	if in.ExpiresInDays != nil {
		if *in.ExpiresInDays <= 0 {
			fields["expires_at"] = nil
		} else {
			fields["expires_at"] = time.Now().AddDate(0, 0, *in.ExpiresInDays)
		}
	}
	if len(fields) == 0 {
		return nil, apierr.BadRequest("没有需要更新的字段")
	}
	if err := s.store.UpdateShareFields(ctx, shareID, fields); err != nil {
		return nil, internal("更新分享失败", err)
	}

	s.audit(ctx, "share.update", "share", itoa(shareID), "更新分享链接", nil, true)

	updated, err := s.store.GetShare(ctx, shareID)
	if err != nil {
		return nil, notFoundOr(err, "分享不存在")
	}
	view := shareView(updated)
	return &view, nil
}

// Deactivate 禁用分享。
func (s *ShareService) Deactivate(ctx context.Context, appID, shareID uint) error {
	if _, err := s.store.GetAppShare(ctx, appID, shareID); err != nil {
		return notFoundOr(err, "分享不存在")
	}
	if err := s.store.UpdateShareFields(ctx, shareID, map[string]any{"is_active": false}); err != nil {
		return internal("禁用分享失败", err)
	}
	s.audit(ctx, "share.deactivate", "share", itoa(shareID), "禁用分享链接", nil, true)
	return nil
}

// Delete 删除分享。
func (s *ShareService) Delete(ctx context.Context, appID, shareID uint) error {
	if _, err := s.store.GetAppShare(ctx, appID, shareID); err != nil {
		return notFoundOr(err, "分享不存在")
	}
	if err := s.store.DeleteShare(ctx, shareID); err != nil {
		return internal("删除分享失败", err)
	}
	s.audit(ctx, "share.delete", "share", itoa(shareID), "删除分享链接", nil, true)
	return nil
}

// Authorize 校验分享令牌与密码，返回分享与应用。
//
// 密码缺失或错误统一返回 CodePasswordRequired，由上层决定响应形态
// （v1 兼容接口返回 209，v2 返回 401）。
func (s *ShareService) Authorize(ctx context.Context, token, password string) (*model.Share, *model.Application, error) {
	share, err := s.store.GetShareByToken(ctx, strings.TrimSpace(token))
	if err != nil {
		return nil, nil, apierr.NotFound("分享链接无效或已过期")
	}
	if share.ExpiresAt != nil && time.Now().After(*share.ExpiresAt) {
		return nil, nil, apierr.NotFound("分享链接已过期")
	}

	switch {
	case share.PasswordHash != "":
		if strings.TrimSpace(password) == "" || !s.verifySecret(share.PasswordHash, password) {
			return nil, nil, apierr.New(apierr.CodePasswordRequired, 401, "需要密码验证")
		}
	case share.PasswordLegacy != "":
		// 旧版分享密码使用可逆加密，出于安全考虑不再解密，需管理员重置。
		return nil, nil, apierr.New(apierr.CodePasswordRequired, 401,
			"该分享使用了旧版密码保护，请管理员在后台重新设置访问密码")
	}

	app, err := s.store.GetApplication(ctx, share.AppID)
	if err != nil {
		return nil, nil, notFoundOr(err, "应用不存在")
	}

	if err := s.store.IncrementShareAccess(ctx, share.ID); err != nil {
		s.log.Warn("更新分享访问次数失败", "shareId", share.ID, "error", err)
	}
	return share, app, nil
}

// Versions 查询分享应用的版本列表。
func (s *ShareService) Versions(ctx context.Context, share *model.Share, platform model.Platform, page model.PageQuery) ([]model.Version, int64, error) {
	versions, total, err := s.VersionServiceForShare().List(ctx,
		repositoryFilter(share.AppID, platform, ""), page)
	if err != nil {
		return nil, 0, err
	}
	return versions, total, nil
}

// LatestPerPlatform 查询分享应用各平台的最新版本。
func (s *ShareService) LatestPerPlatform(ctx context.Context, app *model.Application, channelKey string) ([]model.Version, error) {
	key, err := (&AppService{base: s.base}).resolveChannel(ctx, app, channelKey)
	if err != nil {
		return nil, err
	}
	versions, err := s.store.LatestPerPlatform(ctx, app.ID, key)
	if err != nil {
		return nil, internal("查询版本信息失败", err)
	}
	return versions, nil
}

// VersionServiceForShare 返回版本服务，便于复用查询逻辑。
func (s *ShareService) VersionServiceForShare() *VersionService {
	return &VersionService{base: s.base}
}

// ---------- 模板 ----------

// TemplateService 输出模板管理。
type TemplateService struct{ base }

// TemplateInput 模板创建/更新入参。
type TemplateInput struct {
	Name        string
	Description string
	Content     string
}

// List 查询应用模板。
func (s *TemplateService) List(ctx context.Context, appID uint) ([]model.Template, error) {
	if _, err := (&AppService{base: s.base}).Get(ctx, appID); err != nil {
		return nil, err
	}
	templates, err := s.store.ListTemplates(ctx, appID)
	if err != nil {
		return nil, internal("查询模板列表失败", err)
	}
	return templates, nil
}

// Create 创建模板。
func (s *TemplateService) Create(ctx context.Context, appID uint, in TemplateInput) (*model.Template, error) {
	if _, err := (&AppService{base: s.base}).Get(ctx, appID); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, apierr.BadRequest("模板名称不能为空")
	}
	if strings.TrimSpace(in.Content) == "" {
		return nil, apierr.BadRequest("模板内容不能为空")
	}
	if err := validateTemplate(in.Content); err != nil {
		return nil, err
	}
	if _, err := s.store.GetTemplateByName(ctx, appID, name); err == nil {
		return nil, apierr.Conflict("同名模板已存在")
	}

	tpl := &model.Template{AppID: appID, Name: name, Description: in.Description, Content: in.Content}
	if err := s.store.CreateTemplate(ctx, tpl); err != nil {
		return nil, internal("创建模板失败", err)
	}
	s.audit(ctx, "template.create", "template", itoa(tpl.ID), "创建模板 "+name, nil, true)
	return tpl, nil
}

// Update 更新模板。
func (s *TemplateService) Update(ctx context.Context, appID, templateID uint, in TemplateInput) (*model.Template, error) {
	tpl, err := s.store.GetTemplate(ctx, templateID)
	if err != nil || tpl.AppID != appID {
		return nil, apierr.NotFound("模板不存在")
	}
	if strings.TrimSpace(in.Content) != "" {
		if err := validateTemplate(in.Content); err != nil {
			return nil, err
		}
		tpl.Content = in.Content
	}
	if name := strings.TrimSpace(in.Name); name != "" {
		tpl.Name = name
	}
	if in.Description != "" {
		tpl.Description = in.Description
	}
	if err := s.store.SaveTemplate(ctx, tpl); err != nil {
		return nil, internal("更新模板失败", err)
	}
	s.audit(ctx, "template.update", "template", itoa(templateID), "更新模板 "+tpl.Name, nil, true)
	return tpl, nil
}

// Delete 删除模板。
func (s *TemplateService) Delete(ctx context.Context, appID, templateID uint) error {
	tpl, err := s.store.GetTemplate(ctx, templateID)
	if err != nil || tpl.AppID != appID {
		return apierr.NotFound("模板不存在")
	}
	if err := s.store.DeleteTemplate(ctx, templateID); err != nil {
		return internal("删除模板失败", err)
	}
	s.audit(ctx, "template.delete", "template", itoa(templateID), "删除模板 "+tpl.Name, nil, true)
	return nil
}

// Render 渲染模板，返回响应体与 Content-Type。
func (s *TemplateService) Render(name, content string, app *model.Application, version *model.Version) (string, string, error) {
	ext := map[string]any{}
	if strings.TrimSpace(version.Ext) != "" {
		if err := json.Unmarshal([]byte(version.Ext), &ext); err != nil {
			s.log.Warn("版本扩展信息解析失败", "versionId", version.ID, "error", err)
		}
	}

	data := map[string]any{
		"app": map[string]any{
			"id":          app.ID,
			"name":        app.Name,
			"identifier":  app.Identifier,
			"logo":        app.Logo,
			"description": app.Description,
			"platforms":   app.Platforms.Strings(),
		},
		"ver": map[string]any{
			"version":   version.Version,
			"platform":  version.Platform,
			"channel":   version.Channel,
			"changelog": version.Changelog,
			"isForce":   version.ForceUpdate,
			"fileName":  version.FileName,
			"filePath":  version.FileKey,
			"fileSize":  version.FileSize,
			"createdAt": version.CreatedAt.Format(time.RFC3339),
		},
		"ext": ext,
	}

	tmpl, err := template.New("output").Parse(content)
	if err != nil {
		return "", "", internal("模板解析失败", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", "", internal("模板渲染失败", err)
	}

	return buf.String(), contentTypeByName(name), nil
}

// validateTemplate 提前校验模板语法，避免运行时才发现错误。
func validateTemplate(content string) error {
	if _, err := template.New("probe").Parse(content); err != nil {
		return apierr.BadRequest("模板语法错误: " + err.Error())
	}
	return nil
}

// contentTypeByName 依据模板名称后缀推断响应类型，与 v1 行为一致。
func contentTypeByName(name string) string {
	switch {
	case strings.HasSuffix(name, ".json"):
		return "application/json"
	case strings.HasSuffix(name, ".xml"):
		return "application/xml"
	case strings.HasSuffix(name, ".html"):
		return "text/html"
	case strings.HasSuffix(name, ".yaml"), strings.HasSuffix(name, ".yml"):
		return "application/x-yaml"
	default:
		return "text/plain"
	}
}
