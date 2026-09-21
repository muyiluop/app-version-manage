package database

import (
	"fmt"

	"gorm.io/gorm"

	"app_version_manage/internal/model"
)

// ensureSchema 以“只增不改”的方式补齐表结构（先列、后索引）。
//
// 这里刻意不使用 gorm 的 AutoMigrate：在 SQLite 上它为了修改列定义会
// DROP + CREATE 重建表，而 versions 对 applications 存在外键，重建会被拒绝。
// 迁移只允许新增表、新增列与新增索引，保证既有数据零风险。
func ensureSchema(db *gorm.DB) error {
	if err := ensureSchemaColumns(db); err != nil {
		return err
	}
	return ensureIndexes(db)
}

// ensureSchemaColumns 只补齐表与列，不创建索引。
//
// 每次启动都会先执行它，保证“模型新增了列”这件事对已迁移过的老库同样生效
// （否则新列永远不会被加上）；索引则交给迁移步骤，在数据清理之后再建。
func ensureSchemaColumns(db *gorm.DB) error {
	m := db.Migrator()

	for _, entity := range model.MigrateModels() {
		stmt := &gorm.Statement{DB: db}
		if err := stmt.Parse(entity); err != nil {
			return fmt.Errorf("解析模型失败: %w", err)
		}

		if !m.HasTable(entity) {
			if err := m.CreateTable(entity); err != nil {
				return fmt.Errorf("创建表 %s 失败: %w", stmt.Schema.Table, err)
			}
			continue
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
	return nil
}

// ensureIndexes 创建模型中定义但库里缺失的索引。
//
// 必须晚于数据清理：唯一索引在存在重复数据时会创建失败。
func ensureIndexes(db *gorm.DB) error {
	m := db.Migrator()

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
