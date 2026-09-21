package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"app_version_manage/internal/apierr"
	"app_version_manage/internal/model"
	"app_version_manage/internal/pkg/semver"
	"app_version_manage/internal/repository"
	"app_version_manage/internal/storage"
)

// DownloadTokenTTL 下载链接默认有效期，与 v1 行为保持一致。
const DownloadTokenTTL = 24 * time.Hour

// signDownload 为对象键签发下载令牌。
func (b base) signDownload(key string) string {
	return b.signer.Sign(key, DownloadTokenTTL)
}

// ---------- 应用 ----------

// AppService 应用与通道管理。
type AppService struct{ base }

// AppInput 应用创建/更新入参。
type AppInput struct {
	Name           string           `json:"name"`
	Identifier     string           `json:"identifier"`
	Logo           string           `json:"logo"`
	Description    string           `json:"description"`
	Platforms      []model.Platform `json:"platforms"`
	DefaultChannel string           `json:"defaultChannel"`
}

// List 分页查询应用。
func (s *AppService) List(ctx context.Context, keyword string, page model.PageQuery) ([]model.Application, int64, error) {
	page.Normalize()
	apps, total, err := s.store.ListApplications(ctx, keyword, page)
	if err != nil {
		return nil, 0, internal("查询应用列表失败", err)
	}
	return apps, total, nil
}

// Get 查询应用。
func (s *AppService) Get(ctx context.Context, id uint) (*model.Application, error) {
	app, err := s.store.GetApplication(ctx, id)
	if err != nil {
		return nil, notFoundOr(err, "应用不存在")
	}
	return app, nil
}

// GetByIdentifier 按标识查询应用。
func (s *AppService) GetByIdentifier(ctx context.Context, identifier string) (*model.Application, error) {
	app, err := s.store.GetApplicationByIdentifier(ctx, strings.TrimSpace(identifier))
	if err != nil {
		return nil, notFoundOr(err, "应用不存在")
	}
	return app, nil
}

// Create 创建应用并初始化默认通道。
func (s *AppService) Create(ctx context.Context, in AppInput) (*model.Application, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Identifier = strings.TrimSpace(in.Identifier)
	if in.Name == "" {
		return nil, apierr.BadRequest("应用名称不能为空")
	}
	if in.Identifier == "" {
		return nil, apierr.BadRequest("应用标识不能为空")
	}
	if err := validatePlatforms(in.Platforms); err != nil {
		return nil, err
	}

	taken, err := s.store.IdentifierTaken(ctx, in.Identifier, 0)
	if err != nil {
		return nil, internal("校验应用标识失败", err)
	}
	if taken {
		return nil, apierr.Conflict("应用标识已存在")
	}

	channelKey := strings.TrimSpace(in.DefaultChannel)
	if channelKey == "" {
		channelKey = model.DefaultChannelKey
	}
	if !channelKeyValid(channelKey) {
		return nil, apierr.BadRequest("默认通道名称不合法")
	}

	app := &model.Application{
		Name:           in.Name,
		Identifier:     in.Identifier,
		Logo:           in.Logo,
		Description:    in.Description,
		Platforms:      model.PlatformList(in.Platforms),
		DefaultChannel: channelKey,
		CreatedBy:      ActorFrom(ctx).UserID,
	}

	err = s.store.WithTx(ctx, func(tx *repository.Store) error {
		if err := tx.CreateApplication(ctx, app); err != nil {
			return err
		}
		for _, def := range model.DefaultChannels {
			channel := &model.Channel{
				AppID:     app.ID,
				Key:       def.Key,
				Name:      def.Name,
				IsDefault: def.Key == channelKey,
				Sort:      def.Sort,
			}
			if err := tx.CreateChannel(ctx, channel); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, internal("创建应用失败", err)
	}

	s.audit(ctx, "app.create", "app", itoa(app.ID), "创建应用 "+app.Name,
		map[string]any{"identifier": app.Identifier}, true)
	return app, nil
}

// Update 更新应用，允许清空描述与图标。
func (s *AppService) Update(ctx context.Context, id uint, in AppInput) (*model.Application, error) {
	app, err := s.store.GetApplication(ctx, id)
	if err != nil {
		return nil, notFoundOr(err, "应用不存在")
	}

	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, apierr.BadRequest("应用名称不能为空")
	}
	if err := validatePlatforms(in.Platforms); err != nil {
		return nil, err
	}

	// 已有版本的平台不允许移除，否则历史版本将无法被查询。
	used, err := s.store.UsedPlatforms(ctx, id)
	if err != nil {
		return nil, internal("校验平台失败", err)
	}
	target := model.PlatformList(in.Platforms)
	for _, p := range used {
		if !target.Contains(p) {
			return nil, apierr.BadRequest("平台 " + string(p) + " 已存在版本，无法移除")
		}
	}

	if key := strings.TrimSpace(in.DefaultChannel); key != "" {
		if !channelKeyValid(key) {
			return nil, apierr.BadRequest("默认通道名称不合法")
		}
		if _, err := s.store.GetChannel(ctx, id, key); err != nil {
			return nil, notFoundOr(err, "默认通道不存在")
		}
		app.DefaultChannel = key
	}

	app.Name = name
	app.Logo = in.Logo
	app.Description = in.Description
	app.Platforms = target

	if err := s.store.SaveApplication(ctx, app); err != nil {
		return nil, internal("更新应用失败", err)
	}

	s.audit(ctx, "app.update", "app", itoa(id), "更新应用 "+app.Name, nil, true)
	return app, nil
}

// Delete 删除应用及其下版本、通道、分享、模板（软删除版本，文件交由清理任务处理）。
func (s *AppService) Delete(ctx context.Context, id uint) error {
	app, err := s.store.GetApplication(ctx, id)
	if err != nil {
		return notFoundOr(err, "应用不存在")
	}

	err = s.store.WithTx(ctx, func(tx *repository.Store) error {
		if err := tx.SoftDeleteVersionsByApp(ctx, id); err != nil {
			return err
		}
		if err := tx.DeleteChannelsByApp(ctx, id); err != nil {
			return err
		}
		if err := tx.DeleteSharesByApp(ctx, id); err != nil {
			return err
		}
		if err := tx.DeleteTemplatesByApp(ctx, id); err != nil {
			return err
		}
		return tx.DeleteApplication(ctx, id)
	})
	if err != nil {
		return internal("删除应用失败", err)
	}

	s.audit(ctx, "app.delete", "app", itoa(id), "删除应用 "+app.Name, nil, true)
	return nil
}

// Channels 查询应用通道列表。
func (s *AppService) Channels(ctx context.Context, appID uint) ([]model.Channel, error) {
	if _, err := s.Get(ctx, appID); err != nil {
		return nil, err
	}
	channels, err := s.store.ListChannels(ctx, appID)
	if err != nil {
		return nil, internal("查询通道失败", err)
	}
	return channels, nil
}

// ChannelInput 通道创建/更新入参。
type ChannelInput struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	Sort int    `json:"sort"`
}

// CreateChannel 新增通道。
func (s *AppService) CreateChannel(ctx context.Context, appID uint, in ChannelInput) (*model.Channel, error) {
	if _, err := s.Get(ctx, appID); err != nil {
		return nil, err
	}
	key := strings.ToLower(strings.TrimSpace(in.Key))
	if !channelKeyValid(key) {
		return nil, apierr.BadRequest("通道标识仅支持小写字母、数字、- 与 _")
	}
	if _, err := s.store.GetChannel(ctx, appID, key); err == nil {
		return nil, apierr.Conflict("通道已存在")
	}

	channel := &model.Channel{AppID: appID, Key: key, Name: strings.TrimSpace(in.Name), Sort: in.Sort}
	if channel.Name == "" {
		channel.Name = key
	}
	if err := s.store.CreateChannel(ctx, channel); err != nil {
		return nil, internal("创建通道失败", err)
	}

	s.audit(ctx, "channel.create", "channel", itoa(channel.ID), "创建通道 "+key, nil, true)
	return channel, nil
}

// UpdateChannel 更新通道名称与排序。
func (s *AppService) UpdateChannel(ctx context.Context, appID, channelID uint, in ChannelInput) (*model.Channel, error) {
	channel, err := s.store.GetChannel(ctx, appID, strings.TrimSpace(in.Key))
	if err != nil || channel.ID != channelID {
		return nil, apierr.NotFound("通道不存在")
	}
	if name := strings.TrimSpace(in.Name); name != "" {
		channel.Name = name
	}
	channel.Sort = in.Sort
	if err := s.store.SaveChannel(ctx, channel); err != nil {
		return nil, internal("更新通道失败", err)
	}
	s.audit(ctx, "channel.update", "channel", itoa(channelID), "更新通道 "+channel.Key, nil, true)
	return channel, nil
}

// DeleteChannel 删除通道，默认通道或仍存版本的通道不允许删除。
func (s *AppService) DeleteChannel(ctx context.Context, appID, channelID uint) error {
	channels, err := s.store.ListChannels(ctx, appID)
	if err != nil {
		return internal("查询通道失败", err)
	}
	var target *model.Channel
	for i := range channels {
		if channels[i].ID == channelID {
			target = &channels[i]
			break
		}
	}
	if target == nil {
		return apierr.NotFound("通道不存在")
	}
	if target.IsDefault {
		return apierr.BadRequest("默认通道不能删除，请先设置其他默认通道")
	}
	total, err := s.store.CountVersionsInChannel(ctx, appID, target.Key)
	if err != nil {
		return internal("统计通道版本失败", err)
	}
	if total > 0 {
		return apierr.BadRequest("该通道下仍有版本，无法删除")
	}
	if err := s.store.DeleteChannel(ctx, channelID); err != nil {
		return internal("删除通道失败", err)
	}
	s.audit(ctx, "channel.delete", "channel", itoa(channelID), "删除通道 "+target.Key, nil, true)
	return nil
}

// SetDefaultChannel 设置应用默认通道。
func (s *AppService) SetDefaultChannel(ctx context.Context, appID uint, key string) error {
	if _, err := s.store.GetChannel(ctx, appID, key); err != nil {
		return notFoundOr(err, "通道不存在")
	}
	if err := s.store.SetDefaultChannel(ctx, appID, key); err != nil {
		return internal("设置默认通道失败", err)
	}
	s.audit(ctx, "channel.set_default", "app", itoa(appID), "设置默认通道 "+key, nil, true)
	return nil
}

// resolveChannel 解析通道参数，缺省使用应用默认通道。
func (s *AppService) resolveChannel(ctx context.Context, app *model.Application, key string) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		key = app.DefaultChannel
	}
	if key == "" {
		key = model.DefaultChannelKey
	}
	if _, err := s.store.GetChannel(ctx, app.ID, key); err != nil {
		return "", notFoundOr(err, "通道不存在")
	}
	return key, nil
}

// ResolveChannel 对外暴露的通道解析能力。
func (s *AppService) ResolveChannel(ctx context.Context, app *model.Application, key string) (string, error) {
	return s.resolveChannel(ctx, app, key)
}

// ---------- 版本 ----------

// VersionService 版本管理。
type VersionService struct{ base }

// PublishInput 发布版本入参。
type PublishInput struct {
	AppID               uint
	Platform            model.Platform
	Channel             string
	Version             string
	FileKey             string
	FileName            string
	FileSize            int64
	FileSHA256          string
	ContentType         string
	Changelog           string
	Ext                 string
	ForceUpdate         bool
	MinSupportedVersion string
	Status              model.VersionStatus
}

// VersionUpdateInput 版本更新入参，nil 表示不修改。
type VersionUpdateInput struct {
	Platform            *model.Platform
	Channel             *string
	Version             *string
	Changelog           *string
	Ext                 *string
	ForceUpdate         *bool
	MinSupportedVersion *string
	Status              *model.VersionStatus
	FileKey             *string
	FileName            *string
	FileSize            *int64
	FileSHA256          *string
	ContentType         *string
}

// Publish 发布新版本。
func (s *VersionService) Publish(ctx context.Context, in PublishInput) (*model.Version, error) {
	app, err := s.store.GetApplication(ctx, in.AppID)
	if err != nil {
		return nil, notFoundOr(err, "应用不存在")
	}
	if !in.Platform.Valid() {
		return nil, apierr.BadRequest("不支持的平台")
	}
	if !app.Platforms.Contains(in.Platform) {
		return nil, apierr.BadRequest("应用未配置平台 " + string(in.Platform))
	}

	channelKey, err := (&AppService{base: s.base}).resolveChannel(ctx, app, in.Channel)
	if err != nil {
		return nil, err
	}

	rawVersion := strings.TrimSpace(in.Version)
	sv, err := semver.Parse(rawVersion)
	if err != nil {
		return nil, apierr.BadRequest(err.Error())
	}

	exists, err := s.store.VersionExists(ctx, app.ID, in.Platform, channelKey, rawVersion, 0)
	if err != nil {
		return nil, internal("校验版本号失败", err)
	}
	if exists {
		return nil, apierr.Conflict("该平台与通道下已存在相同版本号")
	}

	fileKey, err := storage.NormalizeKey(in.FileKey)
	if err != nil {
		return nil, apierr.BadRequest("版本文件路径不合法")
	}
	if _, err := s.storage.Stat(ctx, fileKey); err != nil {
		return nil, apierr.NotFound("版本文件不存在，请先上传文件")
	}

	if err := validateExt(in.Ext); err != nil {
		return nil, err
	}

	status := in.Status
	if status == "" {
		status = model.VersionPublished
	}
	if !status.Valid() {
		return nil, apierr.BadRequest("版本状态不合法")
	}

	// 从文件记录补全缺失的元信息，避免调用方必须回传 sha256 / 大小 / 类型。
	in = s.enrichFromFile(ctx, fileKey, in)

	version := &model.Version{
		AppID:               app.ID,
		Platform:            in.Platform,
		Channel:             channelKey,
		Version:             rawVersion,
		VersionMajor:        sv.Major,
		VersionMinor:        sv.Minor,
		VersionPatch:        sv.Patch,
		VersionBuild:        sv.Build,
		Prerelease:          sv.Prerelease,
		FileKey:             fileKey,
		FileName:            firstNonEmpty(in.FileName, storage.SanitizeFileName(fileKey)),
		FileSize:            in.FileSize,
		FileSHA256:          in.FileSHA256,
		ContentType:         in.ContentType,
		Changelog:           in.Changelog,
		Ext:                 in.Ext,
		ForceUpdate:         in.ForceUpdate,
		Status:              status,
		MinSupportedVersion: strings.TrimSpace(in.MinSupportedVersion),
		CreatedBy:           ActorFrom(ctx).UserID,
	}
	if version.FileSize == 0 {
		if obj, err := s.storage.Stat(ctx, fileKey); err == nil {
			version.FileSize = obj.Size
		}
	}
	if status == model.VersionPublished {
		now := time.Now()
		version.PublishedAt = &now
	}

	if err := s.store.CreateVersion(ctx, version); err != nil {
		return nil, internal("发布版本失败", err)
	}

	s.audit(ctx, "version.publish", "version", itoa(version.ID),
		"发布版本 "+app.Identifier+" "+string(version.Platform)+"/"+version.Channel+"/"+version.Version, nil, true)
	return version, nil
}

// Update 更新版本信息。
func (s *VersionService) Update(ctx context.Context, id uint, in VersionUpdateInput) (*model.Version, error) {
	version, err := s.store.GetVersion(ctx, id)
	if err != nil {
		return nil, notFoundOr(err, "版本不存在")
	}

	fields := map[string]any{}
	platform := version.Platform
	channelKey := version.Channel
	rawVersion := version.Version

	if in.Platform != nil {
		if !in.Platform.Valid() {
			return nil, apierr.BadRequest("不支持的平台")
		}
		app, err := s.store.GetApplication(ctx, version.AppID)
		if err != nil {
			return nil, notFoundOr(err, "应用不存在")
		}
		if !app.Platforms.Contains(*in.Platform) {
			return nil, apierr.BadRequest("应用未配置平台 " + string(*in.Platform))
		}
		platform = *in.Platform
	}
	if in.Channel != nil {
		app, err := s.store.GetApplication(ctx, version.AppID)
		if err != nil {
			return nil, notFoundOr(err, "应用不存在")
		}
		key, err := (&AppService{base: s.base}).resolveChannel(ctx, app, *in.Channel)
		if err != nil {
			return nil, err
		}
		channelKey = key
	}
	if in.Version != nil {
		rawVersion = strings.TrimSpace(*in.Version)
		sv, err := semver.Parse(rawVersion)
		if err != nil {
			return nil, apierr.BadRequest(err.Error())
		}
		fields["version_major"] = sv.Major
		fields["version_minor"] = sv.Minor
		fields["version_patch"] = sv.Patch
		fields["version_build"] = sv.Build
		fields["prerelease"] = sv.Prerelease
	}

	if platform != version.Platform || channelKey != version.Channel || rawVersion != version.Version {
		exists, err := s.store.VersionExists(ctx, version.AppID, platform, channelKey, rawVersion, version.ID)
		if err != nil {
			return nil, internal("校验版本号失败", err)
		}
		if exists {
			return nil, apierr.Conflict("该平台与通道下已存在相同版本号")
		}
		fields["platform"] = platform
		fields["channel"] = channelKey
		fields["version"] = rawVersion
	}

	if in.Changelog != nil {
		fields["changelog"] = *in.Changelog
	}
	if in.Ext != nil {
		if err := validateExt(*in.Ext); err != nil {
			return nil, err
		}
		fields["ext"] = *in.Ext
	}
	if in.ForceUpdate != nil {
		fields["force_update"] = *in.ForceUpdate
	}
	if in.MinSupportedVersion != nil {
		fields["min_supported_version"] = strings.TrimSpace(*in.MinSupportedVersion)
	}
	if in.FileKey != nil {
		key, err := storage.NormalizeKey(*in.FileKey)
		if err != nil {
			return nil, apierr.BadRequest("版本文件路径不合法")
		}
		if _, err := s.storage.Stat(ctx, key); err != nil {
			return nil, apierr.NotFound("版本文件不存在")
		}
		fields["file_path"] = key
	}
	if in.FileName != nil {
		fields["file_name"] = *in.FileName
	}
	if in.FileSize != nil {
		fields["file_size"] = *in.FileSize
	}
	if in.FileSHA256 != nil {
		fields["file_sha256"] = *in.FileSHA256
	}
	if in.ContentType != nil {
		fields["content_type"] = *in.ContentType
	}
	if in.Status != nil {
		if !in.Status.Valid() {
			return nil, apierr.BadRequest("版本状态不合法")
		}
		fields["status"] = *in.Status
		if *in.Status == model.VersionPublished && version.PublishedAt == nil {
			fields["published_at"] = time.Now()
		}
	}

	if len(fields) == 0 {
		return nil, apierr.BadRequest("没有需要更新的字段")
	}
	if err := s.store.UpdateVersionFields(ctx, id, fields); err != nil {
		return nil, internal("更新版本失败", err)
	}

	s.audit(ctx, "version.update", "version", itoa(id), "更新版本 "+rawVersion, fields, true)

	updated, err := s.store.GetVersion(ctx, id)
	if err != nil {
		return nil, notFoundOr(err, "版本不存在")
	}
	return updated, nil
}

// SetStatus 上架/下架版本。
func (s *VersionService) SetStatus(ctx context.Context, id uint, status model.VersionStatus) (*model.Version, error) {
	if !status.Valid() {
		return nil, apierr.BadRequest("版本状态不合法")
	}
	return s.Update(ctx, id, VersionUpdateInput{Status: &status})
}

// Delete 软删除版本。
func (s *VersionService) Delete(ctx context.Context, id uint) error {
	version, err := s.store.GetVersion(ctx, id)
	if err != nil {
		return notFoundOr(err, "版本不存在")
	}
	if err := s.store.SoftDeleteVersion(ctx, id); err != nil {
		return internal("删除版本失败", err)
	}
	s.audit(ctx, "version.delete", "version", itoa(id), "删除版本 "+version.Version, nil, true)
	return nil
}

// List 分页查询版本。
func (s *VersionService) List(ctx context.Context, filter repository.VersionFilter, page model.PageQuery) ([]model.Version, int64, error) {
	page.Normalize()
	versions, total, err := s.store.ListVersions(ctx, filter, page)
	if err != nil {
		return nil, 0, internal("查询版本列表失败", err)
	}
	return versions, total, nil
}

// Get 查询版本。
func (s *VersionService) Get(ctx context.Context, id uint) (*model.Version, error) {
	version, err := s.store.GetVersion(ctx, id)
	if err != nil {
		return nil, notFoundOr(err, "版本不存在")
	}
	return version, nil
}

// Latest 查询最新已发布版本。
func (s *VersionService) Latest(ctx context.Context, app *model.Application, platform model.Platform, channelKey string) (*model.Version, error) {
	key, err := (&AppService{base: s.base}).resolveChannel(ctx, app, channelKey)
	if err != nil {
		return nil, err
	}
	version, err := s.store.LatestVersion(ctx, app.ID, platform, key)
	if err != nil {
		return nil, notFoundOr(err, "未找到版本信息")
	}
	return version, nil
}

// Changelog 查询更新日志。
func (s *VersionService) Changelog(ctx context.Context, app *model.Application, platform model.Platform, channelKey string, limit int) ([]model.Version, error) {
	key, err := (&AppService{base: s.base}).resolveChannel(ctx, app, channelKey)
	if err != nil {
		return nil, err
	}
	versions, err := s.store.Changelog(ctx, app.ID, platform, key, limit)
	if err != nil {
		return nil, internal("查询更新日志失败", err)
	}
	return versions, nil
}

// CheckResult 检测更新结果。
type CheckResult struct {
	Identifier          string         `json:"identifier"`
	Platform            model.Platform `json:"platform"`
	Channel             string         `json:"channel"`
	CurrentVersion      string         `json:"currentVersion"`
	HasUpdate           bool           `json:"hasUpdate"`
	ForceUpdate         bool           `json:"forceUpdate"`
	BelowMinimumVersion bool           `json:"belowMinimumVersion"`
	MinSupportedVersion string         `json:"minSupportedVersion,omitempty"`
	Latest              *VersionInfo   `json:"latest"`
}

// VersionInfo 对外暴露的版本信息。
type VersionInfo struct {
	Version     string         `json:"version"`
	Channel     string         `json:"channel"`
	Platform    model.Platform `json:"platform"`
	Changelog   string         `json:"changelog"`
	ForceUpdate bool           `json:"forceUpdate"`
	PublishedAt *time.Time     `json:"publishedAt"`
	FileName    string         `json:"fileName"`
	FileSize    int64          `json:"fileSize"`
	SHA256      string         `json:"sha256,omitempty"`
	DownloadURL string         `json:"downloadUrl"`
}

// Check 语义化检测更新：客户端上报当前版本，服务端判定是否需要更新。
func (s *VersionService) Check(ctx context.Context, identifier string, platform model.Platform, channelKey, currentVersion string) (*CheckResult, error) {
	if !platform.Valid() {
		return nil, apierr.BadRequest("不支持的平台")
	}

	app, err := (&AppService{base: s.base}).GetByIdentifier(ctx, identifier)
	if err != nil {
		return nil, err
	}
	if !app.Platforms.Contains(platform) {
		return nil, apierr.BadRequest("应用未配置平台 " + string(platform))
	}

	resolved, err := (&AppService{base: s.base}).resolveChannel(ctx, app, channelKey)
	if err != nil {
		return nil, err
	}

	latest, err := s.store.LatestVersion(ctx, app.ID, platform, resolved)
	if err != nil {
		return nil, notFoundOr(err, "未找到版本信息")
	}

	result := &CheckResult{
		Identifier:          app.Identifier,
		Platform:            platform,
		Channel:             resolved,
		CurrentVersion:      strings.TrimSpace(currentVersion),
		ForceUpdate:         latest.ForceUpdate,
		MinSupportedVersion: latest.MinSupportedVersion,
		Latest: &VersionInfo{
			Version:     latest.Version,
			Channel:     latest.Channel,
			Platform:    latest.Platform,
			Changelog:   latest.Changelog,
			ForceUpdate: latest.ForceUpdate,
			PublishedAt: latest.PublishedAt,
			FileName:    latest.FileName,
			FileSize:    latest.FileSize,
			SHA256:      latest.FileSHA256,
			DownloadURL: "/api/open/download/" + s.signDownload(latest.FileKey),
		},
	}

	current := strings.TrimSpace(currentVersion)
	if current == "" {
		result.HasUpdate = true
		return result, nil
	}

	cv, err := semver.Parse(current)
	if err != nil {
		return nil, apierr.BadRequest("当前版本号格式不正确: " + err.Error())
	}
	lv, err := semver.Parse(latest.Version)
	if err != nil {
		return nil, internal("服务端版本号解析失败", err)
	}

	result.HasUpdate = lv.GreaterThan(cv)

	if latest.MinSupportedVersion != "" {
		minV, err := semver.Parse(latest.MinSupportedVersion)
		if err == nil && minV.GreaterThan(cv) {
			result.BelowMinimumVersion = true
			result.ForceUpdate = true
			result.HasUpdate = true
		}
	}
	return result, nil
}

// LookupForOpen 供开放接口使用：解析应用、通道并返回最新版本。
func (s *VersionService) LookupForOpen(ctx context.Context, identifier string, platform model.Platform, channelKey string) (*model.Application, *model.Version, error) {
	app, err := (&AppService{base: s.base}).GetByIdentifier(ctx, identifier)
	if err != nil {
		return nil, nil, err
	}
	version, err := s.Latest(ctx, app, platform, channelKey)
	if err != nil {
		return nil, nil, err
	}
	return app, version, nil
}

// ---------- 公共校验 ----------

func validatePlatforms(platforms []model.Platform) error {
	if len(platforms) == 0 {
		return apierr.BadRequest("至少选择一个支持平台")
	}
	seen := map[model.Platform]bool{}
	for _, p := range platforms {
		if !p.Valid() {
			return apierr.BadRequest("不支持的平台: " + string(p))
		}
		if seen[p] {
			return apierr.BadRequest("平台重复: " + string(p))
		}
		seen[p] = true
	}
	return nil
}

func channelKeyValid(key string) bool {
	if key == "" || len(key) > 32 {
		return false
	}
	for _, r := range key {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}

func validateExt(ext string) error {
	trimmed := strings.TrimSpace(ext)
	if trimmed == "" {
		return nil
	}
	var probe map[string]any
	if err := json.Unmarshal([]byte(trimmed), &probe); err != nil {
		return apierr.BadRequest("扩展信息必须是合法的 JSON 对象")
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// 保持 errors 包被使用（notFoundOr 之外的错误判定）。
var _ = errors.Is
