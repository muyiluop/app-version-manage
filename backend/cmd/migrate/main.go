// Command migrate 把旧版（v1）SQLite 数据库与本地文件迁移到新的数据库与存储。
//
// 典型用法：
//
//	# 先看计划（不写任何数据）
//	go run ./cmd/migrate -src-db ../backend/data/app_version.db -src-files ../backend/static/uploads -dry-run
//
//	# 迁到 PostgreSQL + MinIO（目标由配置文件 / APPV_* 环境变量决定）
//	APPV_DATABASE_DRIVER=postgres APPV_DATABASE_DSN='...' APPV_STORAGE_DRIVER=s3 ... \
//	  go run ./cmd/migrate -src-db old.db -src-files old-uploads
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"app_version_manage/internal/config"
	"app_version_manage/internal/database"
	"app_version_manage/internal/logger"
	"app_version_manage/internal/migration"
	"app_version_manage/internal/model"
	"app_version_manage/internal/storage"
)

func main() {
	srcDB := flag.String("src-db", "", "旧 SQLite 数据库文件路径（必填）")
	srcFiles := flag.String("src-files", "", "旧上传目录（可选；不填则只迁数据不搬文件）")
	configPath := flag.String("config", "config.yaml", "目标配置：数据库与存储（可被 APPV_* 环境变量覆盖）")
	envFile := flag.String("env-file", "", "环境变量文件路径（默认依次尝试 APPV_ENV_FILE、./.env、../.env）")
	dryRun := flag.Bool("dry-run", false, "只输出迁移计划与统计，不写入任何数据")
	overwrite := flag.Bool("overwrite", false, "目标库非空时先清空目标库业务数据")
	keepOldKeys := flag.Bool("keep-old-keys", false, "保留旧对象键：只复制文件到新存储，不重写版本/图标引用")
	defaultRole := flag.String("default-role", "admin", "旧用户迁移后的角色：admin|releaser|viewer")
	batch := flag.Int("batch", 200, "批量写入大小")
	flag.Parse()

	if strings.TrimSpace(*srcDB) == "" || strings.HasPrefix(*srcDB, "-") {
		fmt.Fprintln(os.Stderr, "错误：必须用 -src-db 指定旧 SQLite 数据库文件")
		flag.Usage()
		os.Exit(2)
	}

	// 与 server 一致：先环境文件、后配置文件，避免把密钥写进 YAML
	envResult, envErr := config.LoadDotEnv(config.EnvFileCandidates(*envFile))
	if envErr != nil {
		fmt.Fprintf(os.Stderr, "加载环境文件失败: %v\n", envErr)
		os.Exit(1)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载目标配置失败: %v\n", err)
		os.Exit(1)
	}

	log := logger.Init(cfg.Log.Level, cfg.Log.Format)
	if envResult.Path != "" {
		log.Info("已载入环境文件", "file", envResult.Path,
			"loaded", envResult.Loaded, "skipped", envResult.Skipped)
	}
	log.Info("迁移目标已就绪",
		"database", cfg.Database.Driver,
		"storage", cfg.Storage.Driver,
		"dryRun", *dryRun,
		"rekey", !*keepOldKeys)

	// 目标数据库：先确保 schema 是最新的，再灌数据
	db, err := database.Open(cfg)
	if err != nil {
		log.Error("连接目标数据库失败", "error", err)
		os.Exit(1)
	}
	if err := database.Migrate(db, log); err != nil {
		log.Error("初始化目标库表结构失败", "error", err)
		os.Exit(1)
	}

	// 目标存储
	objStore, err := storage.New(storage.Options{
		Driver:    cfg.Storage.Driver,
		LocalRoot: cfg.Storage.Local.Root,
		S3: storage.S3Options{
			Endpoint:       cfg.Storage.S3.Endpoint,
			Region:         cfg.Storage.S3.Region,
			Bucket:         cfg.Storage.S3.Bucket,
			AccessKey:      cfg.Storage.S3.AccessKey,
			SecretKey:      cfg.Storage.S3.SecretKey,
			UseSSL:         cfg.Storage.S3.UseSSLEnabled(),
			ForcePathStyle: cfg.Storage.S3.ForcePathStyleEnabled(),
			Prefix:         cfg.Storage.S3.Prefix,
		},
		SignedURLTTL: time.Duration(cfg.Storage.SignedURLTTLMinutes) * time.Minute,
	})
	if err != nil {
		log.Error("初始化目标存储失败", "driver", cfg.Storage.Driver, "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Hour)
	defer cancel()

	report, err := migration.Run(ctx, migration.Options{
		SourceDB:    *srcDB,
		SourceFiles: *srcFiles,
		DryRun:      *dryRun,
		Rekey:       !*keepOldKeys,
		Overwrite:   *overwrite,
		BatchSize:   *batch,
		DefaultRole: model.Role(*defaultRole),
	}, db, objStore, log)

	printReport(log, report, *dryRun)
	if err != nil {
		log.Error("迁移失败", "error", err)
		os.Exit(1)
	}

	if *dryRun {
		log.Info("dry-run 结束：未写入任何数据；去掉 -dry-run 即可正式执行")
		return
	}

	// 旧库没有任何用户时，补一个默认管理员，避免迁移后无法登录
	var users int64
	if err := db.Model(&model.User{}).Count(&users).Error; err == nil && users == 0 {
		if err := database.Seed(db, cfg, log); err != nil {
			log.Error("创建默认管理员失败", "error", err)
			os.Exit(1)
		}
	}

	log.Info("迁移完成")
}

func printReport(log *slog.Logger, report *migration.Report, dryRun bool) {
	if report == nil {
		return
	}
	mode := "已迁移"
	if dryRun {
		mode = "计划迁移"
	}
	log.Info(mode,
		"applications", report.Applications,
		"channels", report.Channels,
		"versions", report.Versions,
		"templates", report.Templates,
		"users", report.Users,
		"shares", report.Shares,
		"files", report.Files,
	)
	if report.FilesScanned > 0 || report.FilesCopied > 0 {
		log.Info("文件",
			"scanned", report.FilesScanned,
			"copied", report.FilesCopied,
			"skipped(已存在)", report.FilesSkipped,
		)
	}
	if len(report.MissingFiles) > 0 {
		log.Warn("源目录中缺失的对象（引用已保留原键，下载会 404）", "count", len(report.MissingFiles))
		for _, key := range report.MissingFiles {
			log.Warn("  缺失对象", "key", key)
		}
	}
	for _, w := range report.Warnings {
		log.Warn(w)
	}
}
