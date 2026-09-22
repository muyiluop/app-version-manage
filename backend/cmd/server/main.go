// Command server 是版本发布系统的 HTTP 服务入口。
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"app_version_manage/internal/config"
	"app_version_manage/internal/database"
	"app_version_manage/internal/logger"
	"app_version_manage/internal/pkg/token"
	"app_version_manage/internal/repository"
	"app_version_manage/internal/router"
	"app_version_manage/internal/service"
	"app_version_manage/internal/storage"
)

func main() {
	configPath := flag.String("config", "config.yaml", "配置文件路径")
	migrateOnly := flag.Bool("migrate-only", false, "仅执行数据库迁移后退出")
	envFile := flag.String("env-file", "", "环境变量文件路径（默认依次尝试 APPV_ENV_FILE、./.env、../.env）")
	backupDir := flag.String("backup", "", "将数据库在线备份到指定目录后退出（仅 sqlite）")
	flag.Parse()

	// 先载入环境文件，再读配置：优先级 真实环境变量 > 环境文件 > YAML > 代码默认值。
	// 这样本地开发只需一份 .env（不入库），配置文件里不再出现任何明文密钥。
	envResult, envErr := config.LoadDotEnv(config.EnvFileCandidates(*envFile))
	if envErr != nil {
		slog.Error("加载环境文件失败", "error", envErr)
		os.Exit(1)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		// 日志尚未初始化，直接输出到标准错误。
		slog.Error("加载配置失败", "error", err)
		os.Exit(1)
	}

	log := logger.Init(cfg.Log.Level, cfg.Log.Format)
	if envResult.Path != "" {
		log.Info("已载入环境文件", "file", envResult.Path,
			"loaded", envResult.Loaded, "skipped", envResult.Skipped)
	}
	log.Info("配置加载完成", "config", cfg.Redacted())
	for _, name := range cfg.GeneratedSecrets() {
		log.Warn("开发模式检测到缺失密钥，已临时随机生成（重启后失效，生产环境请显式配置）", "key", name)
	}

	db, err := database.Open(cfg)
	if err != nil {
		log.Error("初始化数据库失败", "error", err)
		os.Exit(1)
	}

	// -migrate-only 是运维的显式动作，不受 autoMigrate 开关影响：
	// 它正是 database.autoMigrate=false 场景下「先建表」的那条路径。
	if *migrateOnly {
		if err := database.Migrate(db, log); err != nil {
			log.Error("执行数据库迁移失败", "error", err)
			os.Exit(1)
		}
		if err := database.Seed(db, cfg, log); err != nil {
			log.Error("初始化基础数据失败", "error", err)
			os.Exit(1)
		}
		log.Info("迁移完成，已按要求退出")
		return
	}

	if cfg.Database.AutoMigrateEnabled() {
		if err := database.Migrate(db, log); err != nil {
			log.Error("执行数据库迁移失败", "error", err)
			os.Exit(1)
		}
		if err := database.Seed(db, cfg, log); err != nil {
			log.Error("初始化基础数据失败", "error", err)
			os.Exit(1)
		}
	} else {
		// 关闭自动迁移适用于「DBA 预先建表」或「只读副本」：建表与初始管理员都不做。
		// 若目标库连表都没有，说明配置或部署顺序有问题，这里给出可直接执行的修复指引。
		if !db.Migrator().HasTable("users") {
			log.Error("已关闭自动迁移，但目标库缺少表结构；请先执行一次：" +
				"appv -config <配置文件> -migrate-only")
			os.Exit(1)
		}
		log.Warn("已关闭自动迁移（database.autoMigrate=false），跳过建表与初始管理员创建")
	}

	if *backupDir != "" {
		path, err := database.Backup(db, cfg, *backupDir)
		if err != nil {
			log.Error("数据库备份失败", "error", err)
			os.Exit(1)
		}
		log.Info("数据库备份完成", "path", path)
		return
	}

	store := repository.New(db)
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
		log.Error("初始化存储失败", "driver", cfg.Storage.Driver, "error", err)
		os.Exit(1)
	}

	signer, err := token.NewSigner(cfg.Security.DownloadTokenKey)
	if err != nil {
		log.Error("初始化下载签名器失败", "error", err)
		os.Exit(1)
	}

	svc := service.New(service.Deps{
		Store:   store,
		Storage: objStore,
		Signer:  signer,
		Config:  cfg,
		Log:     log,
	})

	engine := router.New(cfg, svc, log)

	srv := &http.Server{
		Addr:              address(cfg.Server.Port),
		Handler:           engine,
		ReadHeaderTimeout: 10 * time.Second,
		// 大文件上传需要较长的写超时，此处给到 1 小时。
		WriteTimeout: time.Hour,
		ReadTimeout:  time.Hour,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Info("服务已启动",
			"addr", srv.Addr,
			"mode", cfg.Server.Mode,
			"database", cfg.Database.Driver,
			"storage", cfg.Storage.Driver)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("服务启动失败", "error", err)
			os.Exit(1)
		}
	}()

	// 优雅退出
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("收到退出信号，正在关闭服务…")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error("服务关闭异常", "error", err)
	}
	log.Info("服务已退出")
}

func address(port int) string {
	return ":" + itoa(port)
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
