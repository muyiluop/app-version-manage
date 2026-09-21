// Package repository 封装数据访问，业务逻辑一律通过本层读写数据库。
package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"app_version_manage/internal/model"
)

// Store 数据访问入口。
type Store struct {
	db *gorm.DB
}

// New 构造 Store。
func New(db *gorm.DB) *Store { return &Store{db: db} }

// DB 暴露底层连接，仅供迁移与少量特殊查询使用。
func (s *Store) DB() *gorm.DB { return s.db }

// WithTx 在事务中执行 fn，fn 内所有操作使用同一连接。
func (s *Store) WithTx(ctx context.Context, fn func(tx *Store) error) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&Store{db: tx})
	})
}

// Ping 检查数据库连通性。
func (s *Store) Ping(ctx context.Context) error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return fmt.Errorf("获取数据库连接失败: %w", err)
	}
	return sqlDB.PingContext(ctx)
}

// CreateAuditLog 写入审计日志。
func (s *Store) CreateAuditLog(ctx context.Context, entry *model.AuditLog) error {
	return s.db.WithContext(ctx).Create(entry).Error
}

// ListAuditLogs 分页查询审计日志。
func (s *Store) ListAuditLogs(ctx context.Context, query model.PageQuery, action string) ([]model.AuditLog, int64, error) {
	q := s.db.WithContext(ctx).Model(&model.AuditLog{})
	if action != "" {
		q = q.Where("action = ?", action)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var logs []model.AuditLog
	if err := q.Order("id DESC").Offset(query.Offset()).Limit(query.PageSize).Find(&logs).Error; err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}
