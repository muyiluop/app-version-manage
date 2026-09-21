package migration

import (
	"fmt"

	"gorm.io/gorm"
)

// businessTables 需要修正自增序列的表。
//
// 迁移时为了让旧的 ID（可能被外部系统引用）保持不变，会以显式 ID 插入，
// 而显式插入不会推进自增序列，之后新插入记录就会主键冲突，因此必须修一次。
var businessTables = []string{
	"applications", "channels", "versions", "templates", "files", "users", "shares",
}

// fixSequences 按方言修正自增序列；失败只记为告警，不阻断迁移。
func fixSequences(db *gorm.DB, report *Report) {
	switch db.Dialector.Name() {
	case "postgres":
		for _, table := range businessTables {
			sql := fmt.Sprintf(
				"SELECT setval(pg_get_serial_sequence('%s','id'), COALESCE((SELECT MAX(id) FROM %s), 0) + 1, false)",
				table, table)
			if err := db.Exec(sql).Error; err != nil {
				report.warn("修正 %s 自增序列失败: %v（新增记录若主键冲突，请手工执行 setval）", table, err)
			}
		}
	case "mysql":
		for _, table := range businessTables {
			var maxID int64
			if err := db.Raw("SELECT COALESCE(MAX(id), 0) FROM " + table).Scan(&maxID).Error; err != nil {
				report.warn("读取 %s 最大主键失败: %v", table, err)
				continue
			}
			if err := db.Exec(fmt.Sprintf("ALTER TABLE %s AUTO_INCREMENT = %d", table, maxID+1)).Error; err != nil {
				report.warn("修正 %s 自增起点失败: %v", table, err)
			}
		}
	default:
		// SQLite 依据当前最大 rowid 自增，无需处理。
	}
}
