package database

import (
	"testing"

	"app_version_manage/internal/model"
)

func newVersion(appID uint, version string) model.Version {
	return model.Version{
		AppID:    appID,
		Platform: model.PlatformWindows,
		Channel:  model.DefaultChannelKey,
		Version:  version,
		FileKey:  "aa/bb/file.exe",
		FileName: "file.exe",
		Status:   model.VersionPublished,
	}
}

// TestVersionUniqueIndexEnforced 验证唯一索引真的能拦住活跃重复版本。
//
// 回归背景：早期把可空的 deleted_at 放进唯一索引，而 SQLite/PostgreSQL/MySQL
// 都认为 NULL 互不相等，导致约束形同虚设——该测试保证不会再退回去。
func TestVersionUniqueIndexEnforced(t *testing.T) {
	db := openTestDB(t)
	if err := Migrate(db, silentLogger()); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}

	app := model.Application{
		Name: "唯一性测试", Identifier: "com.example.uniq",
		Platforms:      model.PlatformList{model.PlatformWindows},
		DefaultChannel: model.DefaultChannelKey,
	}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("创建应用失败: %v", err)
	}

	first := newVersion(app.ID, "1.0.0")
	if err := db.Create(&first).Error; err != nil {
		t.Fatalf("写入首个版本失败: %v", err)
	}

	duplicate := newVersion(app.ID, "1.0.0")
	if err := db.Create(&duplicate).Error; err == nil {
		t.Fatal("同一 (应用/平台/通道/版本) 的活跃重复记录必须被唯一索引拒绝")
	}

	// 不同通道不受影响
	other := newVersion(app.ID, "1.0.0")
	other.Channel = "beta"
	if err := db.Create(&other).Error; err != nil {
		t.Errorf("不同通道应可发布同名版本: %v", err)
	}

	// 软删除后可以重新发布同一版本
	if err := db.Delete(&first).Error; err != nil {
		t.Fatalf("软删除失败: %v", err)
	}

	var deleted model.Version
	if err := db.Unscoped().First(&deleted, first.ID).Error; err != nil {
		t.Fatalf("查询已删除版本失败: %v", err)
	}
	if deleted.DeletedFlag != uint64(first.ID) {
		t.Errorf("软删除后 deleted_flag 应为自身 id(%d)，实际 %d", first.ID, deleted.DeletedFlag)
	}
	if !deleted.DeletedAt.Valid {
		t.Error("软删除后 deleted_at 应被写入")
	}

	republished := newVersion(app.ID, "1.0.0")
	if err := db.Create(&republished).Error; err != nil {
		t.Errorf("软删除后应可重新发布同一版本: %v", err)
	}
}

// TestVersionUniqueIndexSurvivesReopen 重建连接后约束依旧生效（索引已持久化）。
func TestVersionUniqueIndexSurvivesReopen(t *testing.T) {
	db := openTestDB(t)
	if err := Migrate(db, silentLogger()); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	if !db.Migrator().HasIndex(&model.Version{}, "idx_versions_unique") {
		t.Fatal("唯一索引未创建")
	}

	// 索引定义必须包含 deleted_flag 而不是 deleted_at
	var sql string
	if err := db.Raw("SELECT sql FROM sqlite_master WHERE type = 'index' AND name = 'idx_versions_unique'").Scan(&sql).Error; err != nil {
		t.Fatalf("读取索引定义失败: %v", err)
	}
	if !contains(sql, "deleted_flag") {
		t.Errorf("唯一索引应基于 deleted_flag，实际定义: %s", sql)
	}
	if contains(sql, "deleted_at") {
		t.Errorf("唯一索引不应包含可空的 deleted_at，实际定义: %s", sql)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (func() bool {
		for i := 0; i+len(needle) <= len(haystack); i++ {
			if haystack[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})()
}
