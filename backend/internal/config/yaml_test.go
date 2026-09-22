package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTempYAML(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("写入临时配置失败: %v", err)
	}
	return path
}

// TestYAMLBooleansAreHonored 回归测试：配置文件里的布尔项必须真的生效。
//
// 背景：早期 merge() 只拷贝「非零值」，而 bool 的零值就是 false，导致
// useSsl / forcePathStyle / autoMigrate 在 YAML 里怎么写都被忽略——
// 其中 forcePathStyle: false 是连接 AWS S3 的必需项，静默失效会直接连不上。
func TestYAMLBooleansAreHonored(t *testing.T) {
	path := writeTempYAML(t, `
server:
  port: 18080
  mode: debug

database:
  driver: sqlite
  path: ./data/test.db
  autoMigrate: false

storage:
  driver: s3
  s3:
    endpoint: minio.example.com:9000
    region: ap-east-1
    bucket: appv
    accessKey: ak
    secretKey: sk-secret
    useSsl: true
    forcePathStyle: false
    prefix: release
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	if cfg.Database.AutoMigrateEnabled() {
		t.Error("autoMigrate: false 未生效，启动时仍会自动建表")
	}
	if !cfg.Storage.S3.UseSSLEnabled() {
		t.Error("useSsl: true 未生效")
	}
	if cfg.Storage.S3.ForcePathStyleEnabled() {
		t.Error("forcePathStyle: false 未生效（连 AWS S3 会失败）")
	}
	if cfg.Storage.S3.Prefix != "release" || cfg.Storage.S3.Region != "ap-east-1" {
		t.Errorf("S3 普通字段未生效: prefix=%q region=%q", cfg.Storage.S3.Prefix, cfg.Storage.S3.Region)
	}
}

// TestYAMLBooleanDefaults 未配置时使用默认值。
func TestYAMLBooleanDefaults(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("加载默认配置失败: %v", err)
	}
	if !cfg.Database.AutoMigrateEnabled() {
		t.Error("默认应启用自动迁移")
	}
	if cfg.Storage.S3.UseSSLEnabled() {
		t.Error("默认不应启用 TLS")
	}
	if !cfg.Storage.S3.ForcePathStyleEnabled() {
		t.Error("默认应使用 path-style（自建 MinIO 常见配置）")
	}
}

// TestExampleConfigIsValid 随仓库提供的 config.example.yaml 必须是合法配置。
//
// 目的：示例文件是使用者的第一入口，写错键名或取值会让人一上手就启动失败；
// 纳入测试后，示例与代码结构不会再悄悄脱节。
func TestExampleConfigIsValid(t *testing.T) {
	cfg, err := Load("../../config.example.yaml")
	if err != nil {
		t.Fatalf("示例配置无法加载: %v", err)
	}
	if cfg.Database.Driver != DriverSQLite {
		t.Errorf("示例默认数据库应为 sqlite，实际 %s", cfg.Database.Driver)
	}
	if cfg.Storage.Driver != StorageLocal {
		t.Errorf("示例默认存储应为 local，实际 %s", cfg.Storage.Driver)
	}
	if !cfg.Database.AutoMigrateEnabled() {
		t.Error("示例应默认开启自动迁移")
	}
	if len(cfg.Server.TrustedProxies) == 0 {
		t.Error("示例应给出可信代理示例值")
	}
	if cfg.Server.Port != 9080 {
		t.Errorf("示例端口应为 9080，实际 %d", cfg.Server.Port)
	}
}

// TestEnvOverridesYAMLBooleans 环境变量优先级高于配置文件，且能表达 false。
func TestEnvOverridesYAMLBooleans(t *testing.T) {
	path := writeTempYAML(t, `
database:
  driver: sqlite
  path: ./data/test.db
  autoMigrate: true
storage:
  driver: s3
  s3:
    endpoint: minio.example.com:9000
    bucket: appv
    accessKey: ak
    secretKey: sk
    forcePathStyle: false
`)
	t.Setenv("APPV_DATABASE_AUTO_MIGRATE", "false")
	t.Setenv("APPV_STORAGE_S3_FORCE_PATH_STYLE", "true")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}
	if cfg.Database.AutoMigrateEnabled() {
		t.Error("环境变量应覆盖 YAML 的 autoMigrate")
	}
	if !cfg.Storage.S3.ForcePathStyleEnabled() {
		t.Error("环境变量应覆盖 YAML 的 forcePathStyle")
	}
}
