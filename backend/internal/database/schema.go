package database

import (
	"fmt"

	"gorm.io/gorm"

	"app_version_manage/internal/model"
)

// ensureSchema 以“只增不改”的方式补齐 v2 表结构。
//
// 这里刻意不使用 gorm 的 AutoMigrate：在 SQLite 上它为了修改列定义会
// DROP + CREATE 重建表，而 versions 对 applications 存在外键，重建会被拒绝。
// 迁移只允许新增表、新增列与新增索引，保证既有数据零风险。
func ensureSchema(db *gorm.DB) error {
	m := db.Migrator()

	for _, entity := range model.MigrateModels() {
		if !m.HasTable(entity) {
			if err := m.CreateTable(entity); err != nil {
				return fmt.Errorf("创建表失败: %w", err)
			}
		}

		stmt := &gorm.Statement{DB: db}
		if err := stmt.Parse(entity); err != nil {
			return fmt.Errorf("解析模型失败: %w", err)
		}

		for _, field := range stmt.Schema.Fields {
			if field.DBName == "" || field.IgnoreMigration {
				continue
			}
			if m.HasColumn(entity, field.DBName) {
				continue
			}
			if err := m.AddColumn(entity, field.DBName); err != nil {
				return fmt.Errorf("新增列 %s.%s 失败: %w", stmt.Schema.Table, field.DBName, err)
			}
		}
	}

	// 索引在列补齐之后再创建，避免唯一索引因缺少列而失败。
	for _, entity := range model.MigrateModels() {
		stmt := &gorm.Statement{DB: db}
		if err := stmt.Parse(entity); err != nil {
			return fmt.Errorf("解析模型失败: %w", err)
		}
		for _, idx := range stmt.Schema.ParseIndexes() {
			if idx.Name == "" || m.HasIndex(entity, idx.Name) {
				continue
			}
			if err := m.CreateIndex(entity, idx.Name); err != nil {
				return fmt.Errorf("创建索引 %s.%s 失败: %w", stmt.Schema.Table, idx.Name, err)
			}
		}
	}

	return nil
}
