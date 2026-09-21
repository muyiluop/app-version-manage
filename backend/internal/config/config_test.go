package config

import (
	"strings"
	"testing"
)

// TestReleaseModeRequiresSecrets 生产模式缺少密钥必须拒绝启动。
func TestReleaseModeRequiresSecrets(t *testing.T) {
	t.Setenv("APPV_SERVER_MODE", ModeRelease)
	t.Setenv("APPV_JWT_SECRET", "")
	t.Setenv("APPV_DOWNLOAD_TOKEN_KEY", "")

	if _, err := Load(""); err == nil {
		t.Fatal("生产模式缺少密钥时应返回错误")
	}
}

// TestReleaseModeRejectsWeakSecrets 生产模式拒绝过短的密钥。
func TestReleaseModeRejectsWeakSecrets(t *testing.T) {
	t.Setenv("APPV_SERVER_MODE", ModeRelease)
	t.Setenv("APPV_JWT_SECRET", "short")
	t.Setenv("APPV_DOWNLOAD_TOKEN_KEY", strings.Repeat("k", 32))

	if _, err := Load(""); err == nil {
		t.Fatal("生产模式弱密钥应被拒绝")
	}
}

// TestDebugModeGeneratesSecrets 开发模式自动生成临时密钥并给出告警标记。
func TestDebugModeGeneratesSecrets(t *testing.T) {
	t.Setenv("APPV_SERVER_MODE", ModeDebug)
	t.Setenv("APPV_JWT_SECRET", "")
	t.Setenv("APPV_DOWNLOAD_TOKEN_KEY", "")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("开发模式应可启动: %v", err)
	}
	if len(cfg.GeneratedSecrets()) != 2 {
		t.Errorf("应记录两个临时生成的密钥，实际 %v", cfg.GeneratedSecrets())
	}
	if len(cfg.JWT.Secret) < 32 || len(cfg.Security.DownloadTokenKey) < 32 {
		t.Error("临时密钥长度不足")
	}
}

// TestEnvOverridesYAML 环境变量优先级必须高于默认值。
func TestEnvOverridesYAML(t *testing.T) {
	t.Setenv("APPV_SERVER_PORT", "19999")
	t.Setenv("APPV_DATABASE_DRIVER", DriverPostgres)
	t.Setenv("APPV_DATABASE_DSN", "host=127.0.0.1 user=appv dbname=appv")
	t.Setenv("APPV_STORAGE_DRIVER", StorageS3)
	t.Setenv("APPV_STORAGE_S3_BUCKET", "appv")
	t.Setenv("APPV_STORAGE_S3_ACCESS_KEY", "ak")
	t.Setenv("APPV_STORAGE_S3_SECRET_KEY", "sk")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}
	if cfg.Server.Port != 19999 {
		t.Errorf("端口应被环境变量覆盖，实际 %d", cfg.Server.Port)
	}
	if cfg.Database.Driver != DriverPostgres || cfg.Storage.Driver != StorageS3 {
		t.Errorf("驱动覆盖失败: %s / %s", cfg.Database.Driver, cfg.Storage.Driver)
	}
}

// TestInvalidCombinationsRejected 非法组合必须被拒绝。
func TestInvalidCombinationsRejected(t *testing.T) {
	cases := []map[string]string{
		{"APPV_DATABASE_DRIVER": "oracle"},
		{"APPV_DATABASE_DRIVER": DriverPostgres, "APPV_DATABASE_DSN": ""},
		{"APPV_STORAGE_DRIVER": "ftp"},
		{"APPV_STORAGE_DRIVER": StorageS3, "APPV_STORAGE_S3_BUCKET": ""},
		{"APPV_SERVER_PORT": "70000"},
		{"APPV_SERVER_MODE": "production"},
	}
	for _, env := range cases {
		for k, v := range env {
			t.Setenv(k, v)
		}
		if _, err := Load(""); err == nil {
			t.Errorf("配置 %v 应被拒绝", env)
		}
	}
}

// TestLegacyStoragePathAlias 兼容旧配置 storage.path。
func TestLegacyStoragePathAlias(t *testing.T) {
	t.Setenv("APPV_STORAGE_DRIVER", StorageLocal)
	t.Setenv("APPV_STORAGE_LOCAL_ROOT", "/legacy/uploads")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}
	if cfg.Storage.Local.Root != "/legacy/uploads" {
		t.Errorf("本地存储根目录错误: %s", cfg.Storage.Local.Root)
	}
}
