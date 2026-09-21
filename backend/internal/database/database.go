// Package database 负责数据库连接（SQLite / PostgreSQL / MySQL）与迁移、初始化。
package database

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	"gorm.io/gorm/schema"

	"app_version_manage/internal/config"
)

// Open 按配置建立数据库连接并完成连接池设置与连通性检查。
func Open(cfg *config.Config) (*gorm.DB, error) {
	dialector, err := buildDialector(cfg)
	if err != nil {
		return nil, err
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: gormLogger(cfg),
		NamingStrategy: schema.NamingStrategy{
			SingularTable: false,
		},
		NowFunc: func() time.Time { return time.Now() },
	})
	if err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("获取数据库连接池失败: %w", err)
	}
	sqlDB.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(time.Hour)

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("数据库不可用: %w", err)
	}
	return db, nil
}

// buildDialector 依据 driver 构造对应的 GORM 方言。
func buildDialector(cfg *config.Config) (gorm.Dialector, error) {
	switch cfg.Database.Driver {
	case config.DriverSQLite:
		path := cfg.Database.Path
		if path != ":memory:" {
			if dir := filepath.Dir(path); dir != "" && dir != "." {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					return nil, fmt.Errorf("创建数据库目录 %s 失败: %w", dir, err)
				}
			}
		}
		return sqlite.Open(sqliteDSN(path)), nil

	case config.DriverPostgres:
		return postgres.Open(cfg.Database.DSN), nil

	case config.DriverMySQL:
		return mysql.Open(cfg.Database.DSN), nil

	default:
		return nil, fmt.Errorf("不支持的数据库驱动: %s", cfg.Database.Driver)
	}
}

// sqliteDSN 追加 SQLite 运行参数：WAL 提升并发、busy_timeout 避免锁冲突。
//
// Windows 路径中的反斜杠会被统一转换成正斜杠，否则拼接查询参数后
// 驱动可能无法正确解析文件路径。
func sqliteDSN(path string) string {
	if path == ":memory:" || path == "file::memory:" || strings.HasPrefix(path, "file:") {
		return path
	}
	normalized := strings.ReplaceAll(path, "\\", "/")
	return normalized + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
}

// gormLogger 依据配置选择 GORM 日志级别，避免生产环境打印全部 SQL。
func gormLogger(cfg *config.Config) gormlogger.Interface {
	level := gormlogger.Warn
	if cfg.Database.LogSQL {
		level = gormlogger.Info
	}
	return gormlogger.New(slogWriter{}, gormlogger.Config{
		SlowThreshold:             500 * time.Millisecond,
		LogLevel:                  level,
		IgnoreRecordNotFoundError: true,
		Colorful:                  false,
	})
}

// slogWriter 将 GORM 日志接入 slog。
type slogWriter struct{}

// Printf 实现 gormlogger.Writer。
func (slogWriter) Printf(format string, args ...any) {
	slog.Debug(fmt.Sprintf(format, args...), "component", "gorm")
}
