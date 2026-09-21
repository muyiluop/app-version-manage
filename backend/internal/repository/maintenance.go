package repository

import (
	"context"

	"gorm.io/gorm"

	"app_version_manage/internal/model"
)

// SoftDeleteVersionsByApp 软删除应用下全部版本（用于应用级联删除）。
//
// GORM 不会为批量删除调用 BeforeDelete 钩子，因此这里显式把 deleted_flag
// 置为自身 id，避免这些行继续占用唯一索引中的“活跃位”。
func (s *Store) SoftDeleteVersionsByApp(ctx context.Context, appID uint) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Version{}).Where("app_id = ?", appID).
			UpdateColumn("deleted_flag", gorm.Expr("id")).Error; err != nil {
			return err
		}
		return tx.Where("app_id = ?", appID).Delete(&model.Version{}).Error
	})
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
