package database

import (
	"log/slog"

	"gorm.io/gorm"

	"app_version_manage/internal/model"
)

// migrationRebuildVersionIndex 修正 versions 的唯一索引定义。
//
// 历史实现把可空的 deleted_at 放进了唯一索引，而 SQLite / PostgreSQL / MySQL
// 都认为 NULL 互不相等，导致同一 (app, platform, channel, version) 可以存在
// 多条活跃记录（唯一约束形同虚设）。这里改为使用非空的 deleted_flag：
// 活跃行为 0，软删除行置为自身 id。
func migrationRebuildVersionIndex(db *gorm.DB, log *slog.Logger) error {
	if err := backfillDeletedFlag(db); err != nil {
		return err
	}

	m := db.Migrator()
	if m.HasIndex(&model.Version{}, "idx_versions_unique") {
		if err := m.DropIndex(&model.Version{}, "idx_versions_unique"); err != nil {
			return err
		}
	}

	// 重建唯一索引前必须清除活跃重复记录，否则建索引会失败。
	if err := dedupeVersions(db, log); err != nil {
		return err
	}
	return m.CreateIndex(&model.Version{}, "idx_versions_unique")
}

// backfillDeletedFlag 把已软删除行的 deleted_flag 置为自身 id（幂等）。
func backfillDeletedFlag(db *gorm.DB) error {
	m := db.Migrator()
	if !m.HasTable(&model.Version{}) || !m.HasColumn(&model.Version{}, "deleted_flag") {
		return nil
	}
	return db.Exec("UPDATE versions SET deleted_flag = id WHERE deleted_at IS NOT NULL AND deleted_flag = 0").Error
}
