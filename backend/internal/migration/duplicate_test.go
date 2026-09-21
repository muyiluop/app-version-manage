package migration

import (
	"context"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"app_version_manage/internal/model"
)

// TestMigrateHandlesLegacyDuplicateVersions 旧库允许重复版本号，迁移时必须先收敛，
// 否则会撞上 v2 的唯一索引导致整批迁移失败。
func TestMigrateHandlesLegacyDuplicateVersions(t *testing.T) {
	dbPath, filesDir := setupLegacy(t)

	// 追加一条与 id=10 完全重复、但 id 更大的记录
	legacy, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("打开旧库失败: %v", err)
	}
	if err := legacy.Exec(
		`INSERT INTO versions VALUES (99, 1, 'windows', '1.0.0', 'aaa_setup.exe', 'setup.exe', 11, 'dup', '', 0, 1, '2025-02-01 10:00:00', '2025-02-01 10:00:00')`,
	).Error; err != nil {
		t.Fatalf("写入重复版本失败: %v", err)
	}
	sqlDB, _ := legacy.DB()
	_ = sqlDB.Close()

	target, store, _ := setupTarget(t)

	report, err := Run(context.Background(), Options{
		SourceDB: dbPath, SourceFiles: filesDir, Rekey: true,
	}, target, store, silentLog())
	if err != nil {
		t.Fatalf("存在重复版本时迁移不应失败: %v", err)
	}

	var count int64
	target.Model(&model.Version{}).
		Where("app_id = ? AND platform = ? AND channel = ? AND version = ?", 1, "windows", "stable", "1.0.0").
		Count(&count)
	if count != 1 {
		t.Fatalf("重复版本应收敛为 1 条，实际 %d", count)
	}

	var kept model.Version
	if err := target.Where("app_id = ? AND version = ?", 1, "1.0.0").First(&kept).Error; err != nil {
		t.Fatalf("查询版本失败: %v", err)
	}
	if kept.ID != 99 {
		t.Errorf("应保留 id 最大的记录(99)，实际 %d", kept.ID)
	}

	warned := false
	for _, w := range report.Warnings {
		if strings.Contains(w, "重复版本记录") {
			warned = true
		}
	}
	if !warned {
		t.Errorf("应给出重复版本被跳过的告警: %v", report.Warnings)
	}
	if report.Versions != 3 {
		t.Errorf("迁移版本数应为 3（跳过重复的 1 条），实际 %d", report.Versions)
	}
}
