package service

import (
	"context"

	"app_version_manage/internal/model"
)

// AuditService 审计日志查询。
type AuditService struct{ base }

// List 分页查询审计日志。
func (s *AuditService) List(ctx context.Context, page model.PageQuery, action string) ([]model.AuditLog, int64, error) {
	page.Normalize()
	logs, total, err := s.store.ListAuditLogs(ctx, page, action)
	if err != nil {
		return nil, 0, internal("查询审计日志失败", err)
	}
	return logs, total, nil
}
