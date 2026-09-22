package router_test

import (
	"net/http"
	"testing"
)

// TestV1OpenIncludesSHA256 v1 兼容的 latest / check 需要返回 sha256，
// 供老客户端在下载后校验产物完整性（v2 开放接口原本就有该字段）。
func TestV1OpenIncludesSHA256(t *testing.T) {
	e := newTestEnv(t)
	admin := e.login(t, "admin", "admin123456")

	app := e.obj(t, http.MethodPost, "/api/v2/apps", admin, map[string]any{
		"name": "校验测试", "identifier": "com.example.sha", "platforms": []string{"windows"},
	})
	appID := uint(app["id"].(float64))

	uploaded := e.upload(t, admin, "sha.exe", []byte("sha256-payload"))
	file, _ := uploaded["file"].(map[string]any)
	if file == nil {
		t.Fatalf("上传响应缺少 file 字段: %v", uploaded)
	}
	key, _ := file["key"].(string)
	want, _ := file["sha256"].(string)
	if want == "" {
		t.Fatalf("上传响应未返回 sha256，无法校验: %v", file)
	}

	e.obj(t, http.MethodPost, "/api/v2/versions", admin, map[string]any{
		"appId": appID, "platform": "windows", "version": "2.0.0",
		"fileKey": key, "changelog": "校验版本", "status": "published",
	})

	latest := e.flat(t, http.MethodGet, "/api/open/latest?identifier=com.example.sha&platform=windows", "", nil)
	if got, _ := latest["sha256"].(string); got != want {
		t.Errorf("/api/open/latest 的 sha256 = %q，期望 %q", got, want)
	}

	check := e.flat(t, http.MethodGet,
		"/api/open/check?identifier=com.example.sha&platform=windows&currentVersion=1.0.0", "", nil)
	if got, _ := check["sha256"].(string); got != want {
		t.Errorf("/api/open/check 的 sha256 = %q，期望 %q", got, want)
	}
}
