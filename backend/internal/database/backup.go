package database

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gorm"

	"app_version_manage/internal/config"
)

// Backup 在不中断服务的前提下备份数据库，返回备份文件路径。
//
// SQLite 使用 VACUUM INTO 生成一致性快照（等价于在线备份）；
// PostgreSQL / MySQL 请使用 pg_dump / mysqldump，这里只做提示。
func Backup(db *gorm.DB, cfg *config.Config, dir string) (string, error) {
	if dir == "" {
		return "", fmt.Errorf("备份目录不能为空")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("创建备份目录失败: %w", err)
	}

	if cfg.Database.Driver != config.DriverSQLite {
		return "", fmt.Errorf("当前驱动 %s 不支持内置备份，请使用 pg_dump / mysqldump", cfg.Database.Driver)
	}

	stamp := time.Now().Format("20060102-150405")
	target := filepath.Join(dir, fmt.Sprintf("app_version-%s.db", stamp))
	// VACUUM INTO 的目标路径不能已存在。
	if _, err := os.Stat(target); err == nil {
		return "", fmt.Errorf("备份文件已存在: %s", target)
	}

	// SQLite 的字符串字面量使用单引号，路径中的单引号需转义。
	escaped := strings.ReplaceAll(target, "'", "''")
	if err := db.Exec("VACUUM INTO '" + escaped + "'").Error; err != nil {
		return "", fmt.Errorf("执行备份失败: %w", err)
	}
	return target, nil
}
