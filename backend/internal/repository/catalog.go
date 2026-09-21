package repository

import (
	"context"
	"strings"

	"gorm.io/gorm"

	"app_version_manage/internal/model"
)

// ---------- 应用 ----------

// CreateApplication 创建应用。
func (s *Store) CreateApplication(ctx context.Context, app *model.Application) error {
	return s.db.WithContext(ctx).Create(app).Error
}

// SaveApplication 保存应用全部字段。
func (s *Store) SaveApplication(ctx context.Context, app *model.Application) error {
	return s.db.WithContext(ctx).Save(app).Error
}

// GetApplication 按主键查询应用。
func (s *Store) GetApplication(ctx context.Context, id uint) (*model.Application, error) {
	var app model.Application
	if err := s.db.WithContext(ctx).First(&app, id).Error; err != nil {
		return nil, err
	}
	return &app, nil
}

// GetApplicationByIdentifier 按标识查询应用。
func (s *Store) GetApplicationByIdentifier(ctx context.Context, identifier string) (*model.Application, error) {
	var app model.Application
	if err := s.db.WithContext(ctx).Where("identifier = ?", identifier).First(&app).Error; err != nil {
		return nil, err
	}
	return &app, nil
}

// ListApplications 分页查询应用，支持名称/标识关键字。
func (s *Store) ListApplications(ctx context.Context, keyword string, query model.PageQuery) ([]model.Application, int64, error) {
	q := s.db.WithContext(ctx).Model(&model.Application{})
	if kw := strings.TrimSpace(keyword); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("name LIKE ? OR identifier LIKE ?", like, like)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var apps []model.Application
	if err := q.Order("id DESC").Offset(query.Offset()).Limit(query.PageSize).Find(&apps).Error; err != nil {
		return nil, 0, err
	}
	return apps, total, nil
}

// CountApplications 统计应用数量。
func (s *Store) CountApplications(ctx context.Context) (int64, error) {
	var total int64
	err := s.db.WithContext(ctx).Model(&model.Application{}).Count(&total).Error
	return total, err
}

// IdentifierTaken 判断应用标识是否已被占用。
func (s *Store) IdentifierTaken(ctx context.Context, identifier string, excludeID uint) (bool, error) {
	q := s.db.WithContext(ctx).Model(&model.Application{}).Where("identifier = ?", identifier)
	if excludeID > 0 {
		q = q.Where("id <> ?", excludeID)
	}
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// DeleteApplication 软删除应用。
func (s *Store) DeleteApplication(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Delete(&model.Application{}, id).Error
}

// UsedPlatforms 返回应用下已存在版本的平台集合。
func (s *Store) UsedPlatforms(ctx context.Context, appID uint) ([]model.Platform, error) {
	var platforms []model.Platform
	err := s.db.WithContext(ctx).Model(&model.Version{}).
		Where("app_id = ?", appID).
		Distinct().
		Pluck("platform", &platforms).Error
	return platforms, err
}

// ---------- 通道 ----------

// ListChannels 查询应用下全部通道。
func (s *Store) ListChannels(ctx context.Context, appID uint) ([]model.Channel, error) {
	var channels []model.Channel
	err := s.db.WithContext(ctx).Where("app_id = ?", appID).Order("sort ASC, id ASC").Find(&channels).Error
	return channels, err
}

// GetChannel 查询指定通道。
func (s *Store) GetChannel(ctx context.Context, appID uint, key string) (*model.Channel, error) {
	var channel model.Channel
	if err := s.db.WithContext(ctx).Where("app_id = ? AND key = ?", appID, key).First(&channel).Error; err != nil {
		return nil, err
	}
	return &channel, nil
}

// CreateChannel 创建通道。
func (s *Store) CreateChannel(ctx context.Context, channel *model.Channel) error {
	return s.db.WithContext(ctx).Create(channel).Error
}

// SaveChannel 保存通道。
func (s *Store) SaveChannel(ctx context.Context, channel *model.Channel) error {
	return s.db.WithContext(ctx).Save(channel).Error
}

// DeleteChannel 删除通道。
func (s *Store) DeleteChannel(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Delete(&model.Channel{}, id).Error
}

// CountVersionsInChannel 统计通道下的版本数量。
func (s *Store) CountVersionsInChannel(ctx context.Context, appID uint, channelKey string) (int64, error) {
	var total int64
	err := s.db.WithContext(ctx).Model(&model.Version{}).
		Where("app_id = ? AND channel = ?", appID, channelKey).
		Count(&total).Error
	return total, err
}

// SetDefaultChannel 将指定通道设为应用默认通道。
func (s *Store) SetDefaultChannel(ctx context.Context, appID uint, key string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Channel{}).Where("app_id = ?", appID).
			Update("is_default", false).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Channel{}).Where("app_id = ? AND key = ?", appID, key).
			Update("is_default", true).Error; err != nil {
			return err
		}
		return tx.Model(&model.Application{}).Where("id = ?", appID).
			Update("default_channel", key).Error
	})
}

// UpdateApplicationField 更新应用单个字段。
func (s *Store) UpdateApplicationField(ctx context.Context, appID uint, field string, value any) error {
	return s.db.WithContext(ctx).Model(&model.Application{}).Where("id = ?", appID).
		Update(field, value).Error
}

// ---------- 版本 ----------

// CreateVersion 创建版本。
func (s *Store) CreateVersion(ctx context.Context, version *model.Version) error {
	return s.db.WithContext(ctx).Create(version).Error
}

// SaveVersion 保存版本。
func (s *Store) SaveVersion(ctx context.Context, version *model.Version) error {
	return s.db.WithContext(ctx).Save(version).Error
}

// UpdateVersionFields 局部更新版本字段。
func (s *Store) UpdateVersionFields(ctx context.Context, id uint, fields map[string]any) error {
	return s.db.WithContext(ctx).Model(&model.Version{}).Where("id = ?", id).Updates(fields).Error
}

// GetVersion 按主键查询版本（含已软删除过滤）。
func (s *Store) GetVersion(ctx context.Context, id uint) (*model.Version, error) {
	var version model.Version
	if err := s.db.WithContext(ctx).First(&version, id).Error; err != nil {
		return nil, err
	}
	return &version, nil
}

// VersionFilter 版本列表过滤条件。
type VersionFilter struct {
	AppID           uint
	Platform        model.Platform
	Channel         string
	Status          model.VersionStatus
	Keyword         string
	IncludeArchived bool
}

// ListVersions 分页查询版本。
func (s *Store) ListVersions(ctx context.Context, filter VersionFilter, query model.PageQuery) ([]model.Version, int64, error) {
	q := s.db.WithContext(ctx).Model(&model.Version{}).
		Where("app_id = ?", filter.AppID)
	if filter.Platform != "" {
		q = q.Where("platform = ?", filter.Platform)
	}
	if filter.Channel != "" {
		q = q.Where("channel = ?", filter.Channel)
	}
	if filter.Status != "" {
		q = q.Where("status = ?", filter.Status)
	} else if !filter.IncludeArchived {
		// 默认展示全部状态，交由调用方决定；此处不额外过滤。
		_ = filter
	}
	if kw := strings.TrimSpace(filter.Keyword); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("version LIKE ? OR changelog LIKE ?", like, like)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var versions []model.Version
	if err := q.Order(versionOrderClause).Offset(query.Offset()).Limit(query.PageSize).Find(&versions).Error; err != nil {
		return nil, 0, err
	}
	return versions, total, nil
}

// versionOrderClause 使用语义化版本数值列排序，避免按创建时间排序造成的错误。
const versionOrderClause = "version_major DESC, version_minor DESC, version_patch DESC, version_build DESC, " +
	"CASE WHEN prerelease = '' THEN 0 ELSE 1 END ASC, id DESC"

// LatestVersion 查询指定平台与通道下最新的已发布版本。
func (s *Store) LatestVersion(ctx context.Context, appID uint, platform model.Platform, channel string) (*model.Version, error) {
	q := s.db.WithContext(ctx).Model(&model.Version{}).
		Where("app_id = ? AND status = ?", appID, model.VersionPublished)
	if platform != "" {
		q = q.Where("platform = ?", platform)
	}
	if channel != "" {
		q = q.Where("channel = ?", channel)
	}

	var version model.Version
	if err := q.Order(versionOrderClause).First(&version).Error; err != nil {
		return nil, err
	}
	return &version, nil
}

// LatestPerPlatform 查询每个平台下最新的已发布版本。
func (s *Store) LatestPerPlatform(ctx context.Context, appID uint, channel string) ([]model.Version, error) {
	q := s.db.WithContext(ctx).Model(&model.Version{}).
		Where("app_id = ? AND status = ?", appID, model.VersionPublished)
	if channel != "" {
		q = q.Where("channel = ?", channel)
	}

	var versions []model.Version
	if err := q.Order(versionOrderClause).Find(&versions).Error; err != nil {
		return nil, err
	}

	// 结果已按版本从高到低排序，取每个平台首条即为最新。
	seen := map[model.Platform]bool{}
	out := make([]model.Version, 0, len(versions))
	for _, v := range versions {
		if seen[v.Platform] {
			continue
		}
		seen[v.Platform] = true
		out = append(out, v)
	}
	return out, nil
}

// Changelog 查询已发布版本列表，用于更新日志。
func (s *Store) Changelog(ctx context.Context, appID uint, platform model.Platform, channel string, limit int) ([]model.Version, error) {
	q := s.db.WithContext(ctx).Model(&model.Version{}).
		Where("app_id = ? AND status = ?", appID, model.VersionPublished)
	if platform != "" {
		q = q.Where("platform = ?", platform)
	}
	if channel != "" {
		q = q.Where("channel = ?", channel)
	}
	if limit > 0 {
		q = q.Limit(limit)
	}

	var versions []model.Version
	if err := q.Order(versionOrderClause).Find(&versions).Error; err != nil {
		return nil, err
	}
	return versions, nil
}

// VersionExists 判断同一应用/平台/通道下版本号是否已存在。
func (s *Store) VersionExists(ctx context.Context, appID uint, platform model.Platform, channel, version string, excludeID uint) (bool, error) {
	q := s.db.WithContext(ctx).Model(&model.Version{}).
		Where("app_id = ? AND platform = ? AND channel = ? AND version = ?", appID, platform, channel, version)
	if excludeID > 0 {
		q = q.Where("id <> ?", excludeID)
	}
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// SoftDeleteVersion 软删除版本。
func (s *Store) SoftDeleteVersion(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var version model.Version
		if err := tx.First(&version, id).Error; err != nil {
			return err
		}
		return tx.Delete(&version).Error
	})
}

// IncrementDownloadCount 下载计数 +1。
func (s *Store) IncrementDownloadCount(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Model(&model.Version{}).Where("id = ?", id).
		UpdateColumn("download_count", gorm.Expr("download_count + 1")).Error
}

// ---------- 模板 ----------

// ListTemplates 查询模板。
func (s *Store) ListTemplates(ctx context.Context, appID uint) ([]model.Template, error) {
	var templates []model.Template
	err := s.db.WithContext(ctx).Where("app_id = ?", appID).Order("id ASC").Find(&templates).Error
	return templates, err
}

// GetTemplate 按主键查询模板。
func (s *Store) GetTemplate(ctx context.Context, id uint) (*model.Template, error) {
	var template model.Template
	if err := s.db.WithContext(ctx).First(&template, id).Error; err != nil {
		return nil, err
	}
	return &template, nil
}

// GetTemplateByName 按应用与名称查询模板。
func (s *Store) GetTemplateByName(ctx context.Context, appID uint, name string) (*model.Template, error) {
	var template model.Template
	if err := s.db.WithContext(ctx).Where("app_id = ? AND name = ?", appID, name).First(&template).Error; err != nil {
		return nil, err
	}
	return &template, nil
}

// CreateTemplate 创建模板。
func (s *Store) CreateTemplate(ctx context.Context, template *model.Template) error {
	return s.db.WithContext(ctx).Create(template).Error
}

// SaveTemplate 保存模板。
func (s *Store) SaveTemplate(ctx context.Context, template *model.Template) error {
	return s.db.WithContext(ctx).Save(template).Error
}

// DeleteTemplate 删除模板。
func (s *Store) DeleteTemplate(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Delete(&model.Template{}, id).Error
}
