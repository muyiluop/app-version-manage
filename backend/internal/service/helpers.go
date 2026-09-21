package service

import (
	"gorm.io/gorm"

	"app_version_manage/internal/model"
	"app_version_manage/internal/repository"
)

// errRecordNotFound 统一暴露 GORM 的未找到错误，避免上层直接依赖 gorm 包。
func errRecordNotFound() error { return gorm.ErrRecordNotFound }

// repositoryFilter 构造版本查询条件。
func repositoryFilter(appID uint, platform model.Platform, status model.VersionStatus) repository.VersionFilter {
	return repository.VersionFilter{AppID: appID, Platform: platform, Status: status}
}
