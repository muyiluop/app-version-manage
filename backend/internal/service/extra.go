package service

import (
	"context"

	"app_version_manage/internal/apierr"
	"app_version_manage/internal/model"
	"app_version_manage/internal/pkg/token"
	"app_version_manage/internal/repository"
)

// GetByID 查询文件记录。
func (s *FileService) GetByID(ctx context.Context, id uint) (*model.File, error) {
	file, err := s.store.GetFileByID(ctx, id)
	if err != nil {
		return nil, notFoundOr(err, "文件不存在")
	}
	return file, nil
}

// SignDownload 为对象键签发下载令牌。
func (s *FileService) SignDownload(key string) string { return s.signDownload(key) }

// Get 查询属于指定应用的模板。
func (s *TemplateService) Get(ctx context.Context, appID, templateID uint) (*model.Template, error) {
	tpl, err := s.store.GetTemplate(ctx, templateID)
	if err != nil || tpl.AppID != appID {
		return nil, apierr.NotFound("模板不存在")
	}
	return tpl, nil
}

// GetByName 按名称查询模板。
func (s *TemplateService) GetByName(ctx context.Context, appID uint, name string) (*model.Template, error) {
	tpl, err := s.store.GetTemplateByName(ctx, appID, name)
	if err != nil {
		return nil, notFoundOr(err, "未找到指定的输出模板")
	}
	return tpl, nil
}

// Signer 返回下载令牌签名器。
func (s *Services) Signer() *token.Signer { return s.Auth.signer }

// Store 暴露数据访问层（仅用于少量跨聚合查询）。
func (s *Services) Store() *repository.Store { return s.Auth.store }
