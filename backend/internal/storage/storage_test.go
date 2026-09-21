package storage

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeKeyRejectsTraversal(t *testing.T) {
	bad := []string{
		"", "../etc/passwd", "..", "a/../../b", "..\\windows\\system32",
		"../../../../etc/shadow",
	}
	for _, key := range bad {
		if _, err := NormalizeKey(key); err == nil {
			t.Errorf("会逃出存储根目录的对象键 %q 应被拒绝", key)
		}
	}
}

// TestLocalResolveStaysInsideRoot 验证本地存储最终解析出的路径始终位于根目录内。
func TestLocalResolveStaysInsideRoot(t *testing.T) {
	root := t.TempDir()
	store, err := newLocal(root)
	if err != nil {
		t.Fatalf("构造本地存储失败: %v", err)
	}

	// 领先斜杠与 a/./../b 会被规范化到根目录内部，属于安全行为。
	safe := map[string]string{
		"/absolute/path": "absolute/path",
		"a/./../b":       "b",
	}
	for key, want := range safe {
		full, err := store.resolve(key)
		if err != nil {
			t.Fatalf("对象键 %q 应可解析: %v", key, err)
		}
		expected := filepath.Join(root, filepath.FromSlash(want))
		if full != expected {
			t.Errorf("对象键 %q 应解析为 %q，实际 %q", key, expected, full)
		}
	}

	for _, key := range []string{"../outside.txt", "a/../../outside.txt"} {
		if _, err := store.resolve(key); err == nil {
			t.Errorf("逃逸对象键 %q 应被拒绝", key)
		}
	}
}

func TestNormalizeKeyAcceptsLegacyAndNewKeys(t *testing.T) {
	cases := map[string]string{
		"a56ce38779d869ee753e8b6fa9c838f3_logo_min.png": "a56ce38779d869ee753e8b6fa9c838f3_logo_min.png",
		"ab/cd/abcd1234_setup.exe":                      "ab/cd/abcd1234_setup.exe",
		"./ab/file.exe":                                 "ab/file.exe",
		"a//b/file.exe":                                 "a/b/file.exe",
	}
	for input, want := range cases {
		got, err := NormalizeKey(input)
		if err != nil {
			t.Fatalf("对象键 %q 应合法: %v", input, err)
		}
		if got != want {
			t.Errorf("对象键 %q 规范化结果应为 %q，实际 %q", input, want, got)
		}
	}
}

func TestSanitizeFileName(t *testing.T) {
	cases := map[string]string{
		"../../etc/passwd":    "passwd",
		"setup.exe":           "setup.exe",
		"a<script>.exe":       "a_script_.exe",
		"  spaced name.bin  ": "spaced name.bin",
		"..":                  "file",
		"":                    "file",
	}
	for input, want := range cases {
		if got := SanitizeFileName(input); got != want {
			t.Errorf("SanitizeFileName(%q) = %q，期望 %q", input, got, want)
		}
	}
}

func TestBuildKeyIsShardedAndSafe(t *testing.T) {
	sha := strings.Repeat("ab", 32)
	key := BuildKey(sha, "../../evil.exe")
	wantPrefix := sha[0:2] + "/" + sha[2:4] + "/" + sha + "_"
	if !strings.HasPrefix(key, wantPrefix) {
		t.Errorf("对象键应按哈希分片，实际 %q", key)
	}
	if strings.Contains(key, "..") {
		t.Errorf("对象键不应包含路径穿越片段: %q", key)
	}
	if _, err := NormalizeKey(key); err != nil {
		t.Errorf("生成的对象键应可通过校验: %v", err)
	}
}

func TestHashReader(t *testing.T) {
	// sha256("abc")
	sum, n, err := HashReader(strings.NewReader("abc"))
	if err != nil {
		t.Fatalf("计算哈希失败: %v", err)
	}
	if n != 3 {
		t.Errorf("字节数应为 3，实际 %d", n)
	}
	const want = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if sum != want {
		t.Errorf("sha256 结果错误: %s", sum)
	}
}
