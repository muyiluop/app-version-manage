package repository

import (
	"context"

	"app_version_manage/internal/model"
)

// ReferencedVersionKeysIncludingDeleted 返回被版本引用的对象键，含已软删除版本。
// 清理孤儿文件时必须使用该口径，避免误删仍可恢复的版本产物。
func (s *Store) ReferencedVersionKeysIncludingDeleted(ctx context.Context) ([]string, error) {
	var keys []string
	err := s.db.WithContext(ctx).Unscoped().Model(&model.Version{}).
		Where("file_path <> ''").Distinct().Pluck("file_path", &keys).Error
	return keys, err
}

// CountSharesByApp 统计应用下的分享数量。
func (s *Store) CountSharesByApp(ctx context.Context, appID uint) (int64, error) {
	var total int64
	err := s.db.WithContext(ctx).Model(&model.Share{}).Where("app_id = ?", appID).Count(&total).Error
	return total, err
}
