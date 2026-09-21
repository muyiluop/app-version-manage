package repository

import (
	"context"

	"app_version_manage/internal/model"
)

// SoftDeleteVersionsByApp 软删除应用下全部版本（用于应用级联删除）。
func (s *Store) SoftDeleteVersionsByApp(ctx context.Context, appID uint) error {
	return s.db.WithContext(ctx).Where("app_id = ?", appID).Delete(&model.Version{}).Error
}

// DeleteChannelsByApp 删除应用下全部通道。
func (s *Store) DeleteChannelsByApp(ctx context.Context, appID uint) error {
	return s.db.WithContext(ctx).Where("app_id = ?", appID).Delete(&model.Channel{}).Error
}

// DeleteSharesByApp 删除应用下全部分享。
func (s *Store) DeleteSharesByApp(ctx context.Context, appID uint) error {
	return s.db.WithContext(ctx).Where("app_id = ?", appID).Delete(&model.Share{}).Error
}

// DeleteTemplatesByApp 删除应用下全部模板。
func (s *Store) DeleteTemplatesByApp(ctx context.Context, appID uint) error {
	return s.db.WithContext(ctx).Where("app_id = ?", appID).Delete(&model.Template{}).Error
}
