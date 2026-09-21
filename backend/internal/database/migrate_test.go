package database

import (
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"app_version_manage/internal/model"
)

// legacyStatements 模拟 v1 时期的表结构与数据，用于验证迁移的安全性。
var legacyStatements = []string{
	`CREATE TABLE applications (
		id integer PRIMARY KEY AUTOINCREMENT,
		name text, identifier text, logo text, description text,
		platforms text, created_at datetime, updated_at datetime
	)`,
	`CREATE UNIQUE INDEX idx_legacy_app_identifier ON applications(identifier)`,
	`CREATE TABLE versions (
		id integer PRIMARY KEY AUTOINCREMENT,
		app_id integer, platform text, version text,
		file_path text, file_name text, file_size integer,
		changelog text, ext text,
		force_update numeric DEFAULT false,
		is_active numeric DEFAULT true,
		created_at datetime, updated_at datetime
	)`,
	`CREATE TABLE users (
		id integer PRIMARY KEY AUTOINCREMENT,
		username text, password text, created_at datetime, updated_at datetime
	)`,
}

var legacyRows = []string{
	`INSERT INTO applications (id, name, identifier, logo, description, platforms, created_at, updated_at)
		VALUES (1, '遗留应用', 'com.example.legacy', 'logo_old.png', '历史数据', '["windows","android"]', '2025-01-01 10:00:00', '2025-01-01 10:00:00')`,
	// 1.0.0 重复两条，迁移应保留 id 更大的一条
	`INSERT INTO versions (id, app_id, platform, version, file_path, file_name, file_size, changelog, force_update, is_active, created_at, updated_at)
		VALUES (1, 1, 'windows', '1.0.0', 'old_1.exe', 'old_1.exe', 100, 'first', 0, 1, '2025-01-01 10:00:00', '2025-01-01 10:00:00')`,
	`INSERT INTO versions (id, app_id, platform, version, file_path, file_name, file_size, changelog, force_update, is_active, created_at, updated_at)
		VALUES (2, 1, 'windows', '1.0.0', 'old_2.exe', 'old_2.exe', 200, 'dup', 0, 1, '2025-01-02 10:00:00', '2025-01-02 10:00:00')`,
	// 已下架版本：迁移后 status 应为 archived
	`INSERT INTO versions (id, app_id, platform, version, file_path, file_name, file_size, changelog, force_update, is_active, created_at, updated_at)
		VALUES (3, 1, 'android', '1.2.3', 'old_3.apk', 'old_3.apk', 300, 'offline', 0, 0, '2025-01-03 10:00:00', '2025-01-03 10:00:00')`,
	// 版本号数值排序：1.10.0 必须大于 1.2.3
	`INSERT INTO versions (id, app_id, platform, version, file_path, file_name, file_size, changelog, force_update, is_active, created_at, updated_at)
		VALUES (4, 1, 'windows', '1.10.0', 'old_4.exe', 'old_4.exe', 400, 'newer', 1, 1, '2025-01-04 10:00:00', '2025-01-04 10:00:00')`,
	`INSERT INTO users (id, username, password, created_at, updated_at)
		VALUES (1, 'admin', '123456', '2025-01-01 10:00:00', '2025-01-01 10:00:00')`,
}

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("获取连接池失败: %v", err)
	}
	// Windows 下必须显式关闭连接，否则临时目录无法删除。
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func execAll(t *testing.T, db *gorm.DB, statements []string) {
	t.Helper()
	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("执行 SQL 失败: %v\n%s", err, stmt)
		}
	}
}

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}

// TestMigrateFromLegacySchema 验证从 v1 结构迁移到 v2 的完整性与安全性。
func TestMigrateFromLegacySchema(t *testing.T) {
	db := openTestDB(t)
	execAll(t, db, legacyStatements)
	execAll(t, db, legacyRows)

	if err := Migrate(db, silentLogger()); err != nil {
		t.Fatalf("执行迁移失败: %v", err)
	}

	// 1) 重复版本被合并，仅保留 id 更大的一条
	var versions []model.Version
	if err := db.Order("id ASC").Find(&versions).Error; err != nil {
		t.Fatalf("查询版本失败: %v", err)
	}
	if len(versions) != 3 {
		t.Fatalf("版本数量应为 3，实际 %d", len(versions))
	}
	if versions[0].FileKey != "old_2.exe" {
		t.Errorf("重复版本应保留 id 更大的一条，实际保留 %s", versions[0].FileKey)
	}

	// 2) 通道默认值、状态回填、语义化版本解析
	for _, v := range versions {
		if v.Channel != model.DefaultChannelKey {
			t.Errorf("版本 %d 的通道应为 %s，实际 %s", v.ID, model.DefaultChannelKey, v.Channel)
		}
		if v.PublishedAt == nil {
			t.Errorf("版本 %d 的发布时间未回填", v.ID)
		}
	}
	byID := map[uint]model.Version{}
	for _, v := range versions {
		byID[v.ID] = v
	}
	if byID[3].Status != model.VersionArchived {
		t.Errorf("is_active=0 的版本应回填为 archived，实际 %s", byID[3].Status)
	}
	if byID[4].Status != model.VersionPublished {
		t.Errorf("is_active=1 的版本应回填为 published，实际 %s", byID[4].Status)
	}
	if byID[4].VersionMajor != 1 || byID[4].VersionMinor != 10 {
		t.Errorf("1.10.0 解析结果错误: %d.%d", byID[4].VersionMajor, byID[4].VersionMinor)
	}
	if !byID[4].ForceUpdate {
		t.Error("force_update 未正确保留")
	}

	// 3) 明文密码升级为 bcrypt，且账号被提升为管理员
	var user model.User
	if err := db.First(&user).Error; err != nil {
		t.Fatalf("查询用户失败: %v", err)
	}
	if !strings.HasPrefix(user.Password, "$2") {
		t.Errorf("密码应升级为 bcrypt，实际 %q", user.Password)
	}
	if user.Role != model.RoleAdmin {
		t.Errorf("首个用户应被提升为 admin，实际 %s", user.Role)
	}
	if !user.MustChangePassword {
		t.Error("密码被升级的用户应标记为需要修改密码")
	}

	// 4) 默认通道已补齐
	var channels []model.Channel
	if err := db.Where("app_id = ?", 1).Find(&channels).Error; err != nil {
		t.Fatalf("查询通道失败: %v", err)
	}
	if len(channels) != len(model.DefaultChannels) {
		t.Fatalf("通道数量应为 %d，实际 %d", len(model.DefaultChannels), len(channels))
	}
	defaultCount := 0
	for _, c := range channels {
		if c.IsDefault {
			defaultCount++
		}
	}
	if defaultCount != 1 {
		t.Errorf("应仅有一个默认通道，实际 %d", defaultCount)
	}

	// 5) 唯一索引已建立
	if !db.Migrator().HasIndex(&model.Version{}, "idx_versions_unique") {
		t.Error("versions 唯一索引未创建")
	}

	// 6) 语义化排序：windows 平台最新版本应为 1.10.0
	var latest model.Version
	if err := db.Where("app_id = ? AND platform = ? AND status = ?", 1, model.PlatformWindows, model.VersionPublished).
		Order("version_major DESC, version_minor DESC, version_patch DESC, version_build DESC, id DESC").
		First(&latest).Error; err != nil {
		t.Fatalf("查询最新版本失败: %v", err)
	}
	if latest.Version != "1.10.0" {
		t.Errorf("最新版本应为 1.10.0，实际 %s", latest.Version)
	}
}

// TestMigrateIsIdempotent 重复执行迁移必须安全无副作用。
func TestMigrateIsIdempotent(t *testing.T) {
	db := openTestDB(t)
	execAll(t, db, legacyStatements)
	execAll(t, db, legacyRows)

	if err := Migrate(db, silentLogger()); err != nil {
		t.Fatalf("第一次迁移失败: %v", err)
	}
	if err := Migrate(db, silentLogger()); err != nil {
		t.Fatalf("第二次迁移失败: %v", err)
	}

	var applied int64
	if err := db.Model(&schemaMigration{}).Count(&applied).Error; err != nil {
		t.Fatalf("统计迁移记录失败: %v", err)
	}
	if applied != int64(len(migrations)) {
		t.Errorf("迁移记录数量应为 %d，实际 %d", len(migrations), applied)
	}

	var versionCount int64
	if err := db.Model(&model.Version{}).Count(&versionCount).Error; err != nil {
		t.Fatalf("统计版本失败: %v", err)
	}
	if versionCount != 3 {
		t.Errorf("重复迁移后版本数量应仍为 3，实际 %d", versionCount)
	}
}

// TestMigrateOnEmptyDatabase 全新数据库迁移后应可直接使用。
func TestMigrateOnEmptyDatabase(t *testing.T) {
	db := openTestDB(t)
	if err := Migrate(db, silentLogger()); err != nil {
		t.Fatalf("空库迁移失败: %v", err)
	}
	for _, entity := range model.MigrateModels() {
		if !db.Migrator().HasTable(entity) {
			t.Errorf("表未创建: %T", entity)
		}
	}
}
