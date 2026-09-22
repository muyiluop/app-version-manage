package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// unsetForTest 保证待测变量在环境里不存在（loader 不会覆盖已存在的变量），
// 并在测试结束、或环境变量不存在时清理。
func unsetForTest(t *testing.T, keys ...string) {
	t.Helper()
	for _, key := range keys {
		old, had := os.LookupEnv(key)
		_ = os.Unsetenv(key)
		t.Cleanup(func() {
			if had {
				_ = os.Setenv(key, old)
			} else {
				_ = os.Unsetenv(key)
			}
		})
	}
}

func writeEnvFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("写入环境文件失败: %v", err)
	}
	return path
}

func TestLoadDotEnvParsing(t *testing.T) {
	keys := []string{
		"APPV_T_A", "APPV_T_B", "APPV_T_C", "APPV_T_D", "APPV_T_E", "APPV_T_F",
	}
	unsetForTest(t, keys...)

	content := strings.Join([]string{
		"# 注释行应被忽略",
		"",
		"   ",
		"APPV_T_A=1",
		"export APPV_T_B=hello",
		"APPV_T_C=\"line1\\nline2\"",
		"APPV_T_D='raw\\nvalue'",
		"APPV_T_E=quoted # 行尾注释",
		"APPV_T_F=\" spaced \"",
		"这一行不是键值对，应被忽略",
	}, "\n")

	path := writeEnvFile(t, ".env", content)
	res, err := LoadDotEnv([]string{path})
	if err != nil {
		t.Fatalf("载入失败: %v", err)
	}
	if res.Path != path {
		t.Errorf("应载入 %s，实际 %s", path, res.Path)
	}
	if res.Loaded != 6 {
		t.Errorf("应载入 6 个变量，实际 %d", res.Loaded)
	}

	expect := map[string]string{
		"APPV_T_A": "1",
		"APPV_T_B": "hello",
		"APPV_T_C": "line1\nline2", // 双引号内 \\n 转成真实换行
		"APPV_T_D": "raw\\nvalue",  // 单引号内原样保留
		"APPV_T_E": "quoted",       // 去掉行尾注释
		"APPV_T_F": " spaced ",
	}
	for key, want := range expect {
		got, ok := os.LookupEnv(key)
		if !ok {
			t.Errorf("%s 未被写入环境", key)
			continue
		}
		if got != want {
			t.Errorf("%s = %q，期望 %q", key, got, want)
		}
	}
}

// TestLoadDotEnvDoesNotOverride 真实环境变量优先级最高。
func TestLoadDotEnvDoesNotOverride(t *testing.T) {
	unsetForTest(t, "APPV_T_KEEP")
	t.Setenv("APPV_T_KEEP", "from-process")

	path := writeEnvFile(t, ".env", "APPV_T_KEEP=from-file\nAPPV_T_NEW=fresh")
	res, err := LoadDotEnv([]string{path})
	if err != nil {
		t.Fatalf("载入失败: %v", err)
	}
	if got := os.Getenv("APPV_T_KEEP"); got != "from-process" {
		t.Errorf("已存在的环境变量不应被覆盖，实际 %q", got)
	}
	if got := os.Getenv("APPV_T_NEW"); got != "fresh" {
		t.Errorf("新变量应被写入，实际 %q", got)
	}
	if res.Loaded != 1 || res.Skipped != 1 {
		t.Errorf("统计不符: loaded=%d skipped=%d", res.Loaded, res.Skipped)
	}
	_ = os.Unsetenv("APPV_T_NEW")
}

// TestLoadDotEnvCRLF Windows 上编辑出来的文件带 \\r，必须能正常解析。
func TestLoadDotEnvCRLF(t *testing.T) {
	unsetForTest(t, "APPV_T_CRLF")
	path := writeEnvFile(t, ".env", "APPV_T_CRLF=windows\r\n")
	if _, err := LoadDotEnv([]string{path}); err != nil {
		t.Fatalf("载入失败: %v", err)
	}
	if got := os.Getenv("APPV_T_CRLF"); got != "windows" {
		t.Errorf("CRLF 未处理干净，实际 %q", got)
	}
}

// TestLoadDotEnvPicksFirstExisting 按顺序取第一个存在的文件。
func TestLoadDotEnvPicksFirstExisting(t *testing.T) {
	unsetForTest(t, "APPV_T_ORDER")
	missing := filepath.Join(t.TempDir(), "not-exist.env")
	real := writeEnvFile(t, "real.env", "APPV_T_ORDER=second")

	res, err := LoadDotEnv([]string{missing, real})
	if err != nil {
		t.Fatalf("载入失败: %v", err)
	}
	if res.Path != real {
		t.Errorf("应跳过不存在的文件，实际载入 %s", res.Path)
	}
	_ = os.Unsetenv("APPV_T_ORDER")
}

// TestLoadDotEnvMissingFilesAreFine 没有 .env 也应能正常启动。
func TestLoadDotEnvMissingFilesAreFine(t *testing.T) {
	dir := t.TempDir()
	res, err := LoadDotEnv([]string{filepath.Join(dir, "a.env"), filepath.Join(dir, "b.env"), ""})
	if err != nil {
		t.Fatalf("不应报错: %v", err)
	}
	if res.Path != "" || res.Loaded != 0 {
		t.Errorf("缺文件时应返回空结果，实际 %+v", res)
	}
}

// TestLoadDotEnvRejectsBadKey 变量名不合法要明确报错，而不是塞进环境。
func TestLoadDotEnvRejectsBadKey(t *testing.T) {
	path := writeEnvFile(t, ".env", "1BAD=value")
	if _, err := LoadDotEnv([]string{path}); err == nil {
		t.Fatal("非法变量名应报错")
	}
}
