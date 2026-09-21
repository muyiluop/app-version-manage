package repository

import (
	"context"

	"app_version_manage/internal/model"
)

// CountApplicationsByLogo 统计以该对象键作为图标的应用数量，用于公开图标访问校验。
func (s *Store) CountApplicationsByLogo(ctx context.Context, key string) (int64, error) {
	var total int64
	err := s.db.WithContext(ctx).Model(&model.Application{}).
		Where("logo = ?", key).Count(&total).Error
	return total, err
}
