package router_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
)

// TestV1TemplateOutput 验证自定义模板输出（format=）与 v1 响应结构。
func TestV1TemplateOutput(t *testing.T) {
	e := newTestEnv(t)
	admin := e.login(t, "admin", "admin123456")

	app := e.obj(t, http.MethodPost, "/api/v2/apps", admin, map[string]any{
		"name": "模板测试应用", "identifier": "com.example.tpl", "platforms": []string{"windows"},
	})
	appID := uint(app["id"].(float64))

	uploaded := e.upload(t, admin, "tpl.exe", []byte("template-artifact-content"))
	fileKey := uploaded["file"].(map[string]any)["key"].(string)

	e.obj(t, http.MethodPost, "/api/v2/versions", admin, map[string]any{
		"appId": appID, "platform": "windows", "version": "1.0.0",
		"fileKey": fileKey, "changelog": "初始版本", "status": "published",
		"ext": `{"channel":"official"}`,
	})

	// 模板语法错误必须在保存前被拒绝
	if status, _ := e.do(t, http.MethodPost, fmt.Sprintf("/api/v2/apps/%d/templates", appID), admin, map[string]any{
		"name": "bad.json", "content": "{{.app.name",
	}); status != http.StatusBadRequest {
		t.Fatalf("模板语法错误应返回 400，实际 %d", status)
	}

	e.obj(t, http.MethodPost, fmt.Sprintf("/api/v2/apps/%d/templates", appID), admin, map[string]any{
		"name":    "custom.json",
		"content": `{"app":"{{.app.identifier}}","ver":"{{.ver.version}}","ext":"{{.ext.channel}}","file":"{{.ver.filePath}}"}`,
	})

	resp, err := e.client.Get(e.server.URL + "/api/open/latest?identifier=com.example.tpl&platform=windows&format=custom.json")
	if err != nil {
		t.Fatalf("请求模板输出失败: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("模板输出应返回 200，实际 %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("模板 Content-Type 应为 application/json，实际 %s", ct)
	}

	body, _ := io.ReadAll(resp.Body)
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("模板输出不是合法 JSON: %v\n%s", err, body)
	}
	if parsed["app"] != "com.example.tpl" {
		t.Errorf("模板变量 app.identifier 渲染错误: %v", parsed["app"])
	}
	if parsed["ver"] != "1.0.0" {
		t.Errorf("模板变量 ver.version 渲染错误: %v", parsed["ver"])
	}
	if parsed["ext"] != "official" {
		t.Errorf("模板变量 ext 渲染错误: %v", parsed["ext"])
	}
	if file, _ := parsed["file"].(string); file == "" || file == fileKey {
		t.Errorf("ver.filePath 应为下载令牌而非原始对象键: %v", parsed["file"])
	}

	// 未定义的模板应返回 404
	if status, _ := e.do(t, http.MethodGet, "/api/open/latest?identifier=com.example.tpl&platform=windows&format=missing.json", "", nil); status != http.StatusNotFound {
		t.Errorf("未定义的模板应返回 404，实际 %d", status)
	}
}

// TestV1ChangelogStructure 验证更新日志字段与排序。
func TestV1ChangelogStructure(t *testing.T) {
	e := newTestEnv(t)
	admin := e.login(t, "admin", "admin123456")

	app := e.obj(t, http.MethodPost, "/api/v2/apps", admin, map[string]any{
		"name": "日志测试", "identifier": "com.example.cl", "platforms": []string{"android"},
	})
	appID := uint(app["id"].(float64))

	for _, v := range []string{"1.0.0", "1.2.0", "1.10.0"} {
		uploaded := e.upload(t, admin, v+".apk", []byte("payload-"+v))
		key := uploaded["file"].(map[string]any)["key"].(string)
		e.obj(t, http.MethodPost, "/api/v2/versions", admin, map[string]any{
			"appId": appID, "platform": "android", "version": v,
			"fileKey": key, "changelog": "版本 " + v, "status": "published",
		})
	}

	status, _ := e.do(t, http.MethodGet, "/api/open/changelog?identifier=com.example.cl&platform=android", "", nil)
	if status != http.StatusOK {
		t.Fatalf("changelog 返回 %d", status)
	}

	// v1 返回数组，此处直接取原始响应体校验顺序
	resp, err := e.client.Get(e.server.URL + "/api/open/changelog?identifier=com.example.cl&platform=android")
	if err != nil {
		t.Fatalf("请求 changelog 失败: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var items []map[string]any
	if err := json.Unmarshal(body, &items); err != nil {
		t.Fatalf("changelog 不是数组: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("应有 3 条记录，实际 %d", len(items))
	}
	for _, key := range []string{"createdAt", "version", "platform", "changelog"} {
		if _, ok := items[0][key]; !ok {
			t.Errorf("changelog 条目缺少字段 %s", key)
		}
	}
	// 语义化排序：1.10.0 > 1.2.0 > 1.0.0
	if items[0]["version"] != "1.10.0" {
		t.Errorf("最新版本应为 1.10.0，实际 %v", items[0]["version"])
	}
}
