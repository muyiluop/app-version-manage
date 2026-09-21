package service

import (
	"context"

	"app_version_manage/internal/storage"
)

// LogoKeyExists 判断对象键是否被某个应用用作图标。
//
// 图标需要在公开分享页展示，因此允许匿名读取；版本产物一律走签名下载。
func (s *AppService) LogoKeyExists(ctx context.Context, key string) (bool, error) {
	clean, err := storage.NormalizeKey(key)
	if err != nil {
		return false, nil
	}
	total, err := s.store.CountApplicationsByLogo(ctx, clean)
	if err != nil {
		return false, internal("查询图标引用失败", err)
	}
	return total > 0, nil
}
