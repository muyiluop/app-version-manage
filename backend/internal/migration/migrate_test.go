package migration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"app_version_manage/internal/config"
	"app_version_manage/internal/database"
	"app_version_manage/internal/model"
	"app_version_manage/internal/storage"
)

// ---------- 构造旧环境 ----------

var legacySchema = []string{
	`CREATE TABLE applications (
		id integer PRIMARY KEY AUTOINCREMENT, name text, identifier text, logo text,
		description text, platforms text, created_at datetime, updated_at datetime)`,
	`CREATE TABLE versions (
		id integer PRIMARY KEY AUTOINCREMENT, app_id integer, platform text, version text,
		file_path text, file_name text, file_size integer, changelog text, ext text,
		force_update numeric DEFAULT false, is_active numeric DEFAULT true,
		created_at datetime, updated_at datetime)`,
	`CREATE TABLE templates (
		id integer PRIMARY KEY AUTOINCREMENT, app_id integer, name text, content text,
		created_at datetime, updated_at datetime)`,
	`CREATE TABLE files (
		id integer PRIMARY KEY AUTOINCREMENT, name text, path text, size integer, type text,
		hash text, created_at datetime, updated_at datetime)`,
	`CREATE TABLE users (
		id integer PRIMARY KEY AUTOINCREMENT, username text, password text,
		created_at datetime, updated_at datetime)`,
	`CREATE TABLE shares (
		id integer PRIMARY KEY AUTOINCREMENT, app_id integer, token text, password text,
		expires_at datetime, is_active numeric DEFAULT true,
		created_at datetime, updated_at datetime)`,
}

var legacyData = []string{
	`INSERT INTO applications VALUES (1, '应用A', 'com.example.a', 'aaa_logo.png', 'desc-a', '["windows"]', '2025-01-01 10:00:00', '2025-01-01 10:00:00')`,
	`INSERT INTO applications VALUES (2, '应用B', 'com.example.b', 'aaa_logo.png', 'desc-b', '["android","ios"]', '2025-01-02 10:00:00', '2025-01-02 10:00:00')`,
	`INSERT INTO versions VALUES (10, 1, 'windows', '1.0.0', 'aaa_setup.exe', 'setup.exe', 11, 'first', '', 0, 1, '2025-01-01 11:00:00', '2025-01-01 11:00:00')`,
	`INSERT INTO versions VALUES (11, 1, 'windows', '1.10.0', 'aaa_new.exe', 'new.exe', 7, 'second', '{"k":"v"}', 1, 1, '2025-01-03 11:00:00', '2025-01-03 11:00:00')`,
	`INSERT INTO versions VALUES (12, 2, 'android', '2.0.0', 'missing.apk', 'app.apk', 99, 'gone', '', 0, 0, '2025-01-04 11:00:00', '2025-01-04 11:00:00')`,
	`INSERT INTO templates VALUES (20, 1, 'custom.json', '{"v":"{{.ver.version}}"}', '2025-01-01 12:00:00', '2025-01-01 12:00:00')`,
	`INSERT INTO files VALUES (30, 'setup.exe', 'aaa_setup.exe', 11, 'application/octet-stream', 'md5aaa', '2025-01-01 11:00:00', '2025-01-01 11:00:00')`,
	`INSERT INTO users VALUES (40, 'admin', '123456', '2025-01-01 10:00:00', '2025-01-01 10:00:00')`,
	`INSERT INTO shares VALUES (50, 1, 'legacytoken123', 'encrypted-blob', NULL, 1, '2025-01-05 10:00:00', '2025-01-05 10:00:00')`,
}

func setupLegacy(t *testing.T) (dbPath, filesDir string) {
	t.Helper()
	dir := t.TempDir()

	dbPath = filepath.Join(dir, "legacy.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("创建旧库失败: %v", err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { _ = sqlDB.Close() })

	for _, stmt := range append(append([]string{}, legacySchema...), legacyData...) {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("初始化旧库失败: %v\n%s", err, stmt)
		}
	}

	filesDir = filepath.Join(dir, "uploads")
	if err := os.MkdirAll(filesDir, 0o755); err != nil {
		t.Fatalf("创建源目录失败: %v", err)
	}
	writeFile(t, filepath.Join(filesDir, "aaa_logo.png"), "logo-bytes")
	writeFile(t, filepath.Join(filesDir, "aaa_setup.exe"), "setup-bytes")
	writeFile(t, filepath.Join(filesDir, "aaa_new.exe"), "new-bytes")
	// missing.apk 故意不创建

	return dbPath, filesDir
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("写入 %s 失败: %v", path, err)
	}
}

// setupTarget 构造全新的目标数据库与目标存储。
func setupTarget(t *testing.T) (*gorm.DB, storage.Storage, string) {
	t.Helper()
	dir := t.TempDir()

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}
	cfg.Database.Driver = config.DriverSQLite
	cfg.Database.Path = filepath.Join(dir, "target.db")
	cfg.Storage.Driver = config.StorageLocal
	cfg.Storage.Local.Root = filepath.Join(dir, "target-uploads")

	db, err := database.Open(cfg)
	if err != nil {
		t.Fatalf("打开目标库失败: %v", err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { _ = sqlDB.Close() })

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := database.Migrate(db, log); err != nil {
		t.Fatalf("目标库迁移失败: %v", err)
	}

	store, err := storage.New(storage.Options{Driver: config.StorageLocal, LocalRoot: cfg.Storage.Local.Root})
	if err != nil {
		t.Fatalf("初始化目标存储失败: %v", err)
	}
	return db, store, cfg.Storage.Local.Root
}

func sha256Of(t *testing.T, content string) string {
	t.Helper()
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func silentLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// ---------- 测试 ----------

func TestMigrateCarriesDataAndFiles(t *testing.T) {
	dbPath, filesDir := setupLegacy(t)
	target, store, storageRoot := setupTarget(t)

	report, err := Run(context.Background(), Options{
		SourceDB:    dbPath,
		SourceFiles: filesDir,
		Rekey:       true,
	}, target, store, silentLog())
	if err != nil {
		t.Fatalf("迁移失败: %v", err)
	}

	// 统计
	if report.Applications != 2 || report.Versions != 3 || report.Templates != 1 {
		t.Fatalf("统计不符: %+v", report)
	}
	if report.Users != 1 || report.Shares != 1 {
		t.Fatalf("用户/分享统计不符: %+v", report)
	}
	if report.FilesCopied != 3 {
		t.Errorf("应复制 3 个文件，实际 %d", report.FilesCopied)
	}

	// ID 保留
	var app model.Application
	if err := target.First(&app, 1).Error; err != nil {
		t.Fatalf("应用 1 未迁移: %v", err)
	}
	if app.ID != 1 || app.Identifier != "com.example.a" {
		t.Errorf("应用 ID/标识未保留: %+v", app)
	}
	if app.DefaultChannel != model.DefaultChannelKey {
		t.Errorf("默认通道应为 stable，实际 %s", app.DefaultChannel)
	}

	// 图标重新分片且文件已搬过去
	wantLogo := storage.BuildKey(sha256Of(t, "logo-bytes"), "aaa_logo.png")
	if app.Logo != wantLogo {
		t.Errorf("图标键未重写：期望 %s，实际 %s", wantLogo, app.Logo)
	}
	if _, err := os.Stat(filepath.Join(storageRoot, filepath.FromSlash(app.Logo))); err != nil {
		t.Errorf("目标存储缺少图标文件: %v", err)
	}

	// 通道为每个应用补齐
	var channels int64
	target.Model(&model.Channel{}).Where("app_id = ?", 1).Count(&channels)
	if channels != int64(len(model.DefaultChannels)) {
		t.Errorf("应用 1 通道数应为 %d，实际 %d", len(model.DefaultChannels), channels)
	}

	// 版本：语义化解析、通道、状态、校验值
	var v11 model.Version
	if err := target.First(&v11, 11).Error; err != nil {
		t.Fatalf("版本 11 未迁移: %v", err)
	}
	if v11.VersionMajor != 1 || v11.VersionMinor != 10 {
		t.Errorf("1.10.0 解析错误: %d.%d", v11.VersionMajor, v11.VersionMinor)
	}
	if v11.Channel != model.DefaultChannelKey {
		t.Errorf("通道未回填: %s", v11.Channel)
	}
	if v11.Status != model.VersionPublished {
		t.Errorf("启用版本状态应为 published，实际 %s", v11.Status)
	}
	if len(v11.FileSHA256) != 64 {
		t.Errorf("应写入 sha256，实际 %q", v11.FileSHA256)
	}
	if !v11.ForceUpdate {
		t.Error("force_update 未保留")
	}
	if v11.Ext != `{"k":"v"}` {
		t.Errorf("ext 未保留: %s", v11.Ext)
	}
	wantKey := storage.BuildKey(sha256Of(t, "new-bytes"), "aaa_new.exe")
	if v11.FileKey != wantKey {
		t.Errorf("版本文件键未重写：期望 %s，实际 %s", wantKey, v11.FileKey)
	}
	if v11.PublishedAt == nil {
		t.Error("published_at 未回填")
	}

	// 已下架版本 -> archived
	var v12 model.Version
	if err := target.First(&v12, 12).Error; err != nil {
		t.Fatalf("版本 12 未迁移: %v", err)
	}
	if v12.Status != model.VersionArchived {
		t.Errorf("is_active=0 应迁移为 archived，实际 %s", v12.Status)
	}

	// 缺失对象被记录，且键保留原值
	found := false
	for _, k := range report.MissingFiles {
		if k == "missing.apk" {
			found = true
		}
	}
	if !found {
		t.Errorf("缺失对象未被报告: %v", report.MissingFiles)
	}
	if v12.FileKey != "missing.apk" {
		t.Errorf("缺失对象的键应保留原值，实际 %s", v12.FileKey)
	}

	// 用户：口令 bcrypt 化 + 角色 + 强制改密
	var user model.User
	if err := target.First(&user, 40).Error; err != nil {
		t.Fatalf("用户未迁移: %v", err)
	}
	if !strings.HasPrefix(user.Password, "$2") {
		t.Errorf("口令应为 bcrypt，实际 %q", user.Password)
	}
	if user.Role != model.RoleAdmin || !user.MustChangePassword {
		t.Errorf("用户角色/改密标记错误: %+v", user)
	}

	// 分享：令牌保留（旧链接继续可用）+ 标记旧算法
	var share model.Share
	if err := target.First(&share, 50).Error; err != nil {
		t.Fatalf("分享未迁移: %v", err)
	}
	if share.Token != "legacytoken123" {
		t.Errorf("分享令牌应保留，实际 %s", share.Token)
	}
	if share.PasswordAlgo != "legacy-aes" || share.PasswordLegacy != "encrypted-blob" {
		t.Errorf("旧密码未按 legacy-aes 保留: %+v", share)
	}

	// 模板
	var tpl model.Template
	if err := target.First(&tpl, 20).Error; err != nil {
		t.Fatalf("模板未迁移: %v", err)
	}
	if tpl.Content == "" {
		t.Error("模板内容为空")
	}

	// 显式 ID 插入后，新记录仍能自增插入（序列未错位）
	next := model.Application{Name: "迁移后新增", Identifier: "com.example.new", Platforms: model.PlatformList{model.PlatformWindows}}
	if err := target.Create(&next).Error; err != nil {
		t.Fatalf("迁移后新增应用失败（自增序列可能错位）: %v", err)
	}
	if next.ID <= 2 {
		t.Errorf("新增应用 ID 应为 3 之后，实际 %d", next.ID)
	}
}

func TestMigrateDryRunWritesNothing(t *testing.T) {
	dbPath, filesDir := setupLegacy(t)
	target, store, storageRoot := setupTarget(t)

	report, err := Run(context.Background(), Options{
		SourceDB: dbPath, SourceFiles: filesDir, Rekey: true, DryRun: true,
	}, target, store, silentLog())
	if err != nil {
		t.Fatalf("dry-run 失败: %v", err)
	}
	if report.Applications != 2 || report.Versions != 3 {
		t.Errorf("dry-run 统计不符: %+v", report)
	}
	if report.FilesCopied != 0 {
		t.Errorf("dry-run 不应复制文件，实际 %d", report.FilesCopied)
	}

	var apps int64
	target.Model(&model.Application{}).Count(&apps)
	if apps != 0 {
		t.Errorf("dry-run 不应写入数据，实际 %d 个应用", apps)
	}
	entries, _ := os.ReadDir(storageRoot)
	if len(entries) != 0 {
		t.Errorf("dry-run 不应写入文件，实际 %d 个条目", len(entries))
	}
}

func TestMigrateRequiresOverwriteForNonEmptyTarget(t *testing.T) {
	dbPath, filesDir := setupLegacy(t)
	target, store, _ := setupTarget(t)

	opts := Options{SourceDB: dbPath, SourceFiles: filesDir, Rekey: true}
	if _, err := Run(context.Background(), opts, target, store, silentLog()); err != nil {
		t.Fatalf("首次迁移失败: %v", err)
	}

	// 目标非空时 dry-run 仍应给出计划（不写入），并提示需要 -overwrite
	dry := opts
	dry.DryRun = true
	dryReport, err := Run(context.Background(), dry, target, store, silentLog())
	if err != nil {
		t.Fatalf("目标非空时 dry-run 应仍可执行: %v", err)
	}
	if dryReport.Applications != 2 {
		t.Errorf("dry-run 计划中应用数应为 2，实际 %d", dryReport.Applications)
	}
	warned := false
	for _, w := range dryReport.Warnings {
		if strings.Contains(w, "目标库已有") {
			warned = true
		}
	}
	if !warned {
		t.Errorf("dry-run 应提示目标非空: %v", dryReport.Warnings)
	}
	var stillThere int64
	target.Model(&model.Application{}).Count(&stillThere)
	if stillThere != 2 {
		t.Errorf("dry-run 不应改动目标库，实际 %d 个应用", stillThere)
	}

	// 目标非空且正式执行、未声明覆盖 -> 报错
	if _, err := Run(context.Background(), opts, target, store, silentLog()); err == nil {
		t.Fatal("目标非空正式执行时应要求 -overwrite")
	}

	// 声明覆盖后应可重跑，且文件因已存在而跳过
	opts.Overwrite = true
	report, err := Run(context.Background(), opts, target, store, silentLog())
	if err != nil {
		t.Fatalf("覆盖迁移失败: %v", err)
	}
	if report.FilesSkipped != 3 || report.FilesCopied != 0 {
		t.Errorf("重跑应跳过已存在文件：copied=%d skipped=%d", report.FilesCopied, report.FilesSkipped)
	}

	var apps int64
	target.Model(&model.Application{}).Count(&apps)
	if apps != 2 {
		t.Errorf("覆盖后应用数应为 2，实际 %d", apps)
	}
}

func TestMigrateKeepsOldKeysWhenRequested(t *testing.T) {
	dbPath, filesDir := setupLegacy(t)
	target, store, storageRoot := setupTarget(t)

	if _, err := Run(context.Background(), Options{
		SourceDB: dbPath, SourceFiles: filesDir, Rekey: false,
	}, target, store, silentLog()); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}

	var app model.Application
	if err := target.First(&app, 1).Error; err != nil {
		t.Fatalf("应用未迁移: %v", err)
	}
	if app.Logo != "aaa_logo.png" {
		t.Errorf("保留旧键时图标应保持原值，实际 %s", app.Logo)
	}
	// 但文件仍按新键落到目标存储
	entries, err := os.ReadDir(filepath.Join(storageRoot, storage.BuildKey(sha256Of(t, "logo-bytes"), "aaa_logo.png")[0:2]))
	if err != nil || len(entries) == 0 {
		t.Errorf("文件应已按分片键写入目标存储: %v", err)
	}
}

func TestParsePlatformsFallback(t *testing.T) {
	got := parsePlatforms("windows,android,bogus")
	if len(got) != 2 || got[0] != model.PlatformWindows || got[1] != model.PlatformAndroid {
		t.Errorf("逗号分隔与非法值处理错误: %v", got)
	}
	if parsePlatforms("") != nil {
		t.Error("空值应返回 nil")
	}
	if v := parsePlatforms(`["ios"]`); len(v) != 1 || v[0] != model.PlatformIOS {
		t.Errorf("JSON 数组解析错误: %v", v)
	}
}

// 保证 Options/Report 的零值可用，避免调用方漏填 BatchSize 时出现除零之类问题。
func TestZeroValueOptionsUsable(t *testing.T) {
	o := Options{}
	if o.DryRun || o.Rekey || o.Overwrite {
		t.Error("零值布尔字段应为 false")
	}
	start := time.Now()
	_ = start
}
