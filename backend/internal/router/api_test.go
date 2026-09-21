package router_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"app_version_manage/internal/config"
	"app_version_manage/internal/database"
	"app_version_manage/internal/logger"
	"app_version_manage/internal/pkg/token"
	"app_version_manage/internal/repository"
	"app_version_manage/internal/router"
	"app_version_manage/internal/service"
	"app_version_manage/internal/storage"
)

type testEnv struct {
	server *httptest.Server
	client *http.Client
	cfg    *config.Config
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	dir := t.TempDir()

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("加载默认配置失败: %v", err)
	}
	cfg.Server.Mode = config.ModeDebug
	cfg.Database.Driver = config.DriverSQLite
	cfg.Database.Path = filepath.Join(dir, "test.db")
	cfg.Storage.Driver = config.StorageLocal
	cfg.Storage.Local.Root = filepath.Join(dir, "uploads")
	cfg.JWT.Secret = "unit-test-jwt-secret-0123456789"
	cfg.Security.DownloadTokenKey = "unit-test-download-key-0123456789"
	cfg.Security.AdminInitialPassword = "admin123456"
	cfg.Log.Level = "error"

	log := logger.Init("error", "text")
	db, err := database.Open(cfg)
	if err != nil {
		t.Fatalf("打开数据库失败: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("获取连接池失败: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	if err := database.Migrate(db, log); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	if err := database.Seed(db, cfg, log); err != nil {
		t.Fatalf("初始化失败: %v", err)
	}

	objStore, err := storage.New(storage.Options{Driver: cfg.Storage.Driver, LocalRoot: cfg.Storage.Local.Root})
	if err != nil {
		t.Fatalf("初始化存储失败: %v", err)
	}
	signer, err := token.NewSigner(cfg.Security.DownloadTokenKey)
	if err != nil {
		t.Fatalf("初始化签名器失败: %v", err)
	}

	svc := service.New(service.Deps{
		Store:   repository.New(db),
		Storage: objStore,
		Signer:  signer,
		Config:  cfg,
		Log:     log,
	})

	ts := httptest.NewServer(router.New(cfg, svc, log))
	t.Cleanup(ts.Close)
	return &testEnv{server: ts, client: ts.Client(), cfg: cfg}
}

func (e *testEnv) do(t *testing.T, method, path, tok string, body any) (int, map[string]any) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("序列化请求体失败: %v", err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, e.server.URL+path, reader)
	if err != nil {
		t.Fatalf("构造请求失败: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := e.client.Do(req)
	if err != nil {
		t.Fatalf("请求 %s %s 失败: %v", method, path, err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	var parsed map[string]any
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &parsed)
	}
	return resp.StatusCode, parsed
}

// ok 断言请求成功并返回 data 字段。
func (e *testEnv) ok(t *testing.T, method, path, tok string, body any) any {
	t.Helper()
	status, parsed := e.do(t, method, path, tok, body)
	if status != http.StatusOK && status != http.StatusCreated {
		t.Fatalf("%s %s 期望成功，实际 %d: %v", method, path, status, parsed)
	}
	return parsed["data"]
}

func (e *testEnv) obj(t *testing.T, method, path, tok string, body any) map[string]any {
	t.Helper()
	data, _ := e.ok(t, method, path, tok, body).(map[string]any)
	if data == nil {
		t.Fatalf("%s %s 返回的 data 不是对象", method, path)
	}
	return data
}

// flat 用于 v1 兼容接口：响应结构为扁平 JSON，没有 data 信封。
func (e *testEnv) flat(t *testing.T, method, path, tok string, body any) map[string]any {
	t.Helper()
	status, parsed := e.do(t, method, path, tok, body)
	if status != http.StatusOK {
		t.Fatalf("%s %s 期望 200，实际 %d: %v", method, path, status, parsed)
	}
	return parsed
}

func (e *testEnv) login(t *testing.T, username, password string) string {
	t.Helper()
	return e.obj(t, http.MethodPost, "/api/v2/auth/login", "", map[string]any{
		"username": username, "password": password,
	})["accessToken"].(string)
}

func (e *testEnv) upload(t *testing.T, tok, filename string, payload []byte) map[string]any {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("构造表单失败: %v", err)
	}
	if _, err := part.Write(payload); err != nil {
		t.Fatalf("写入表单失败: %v", err)
	}
	_ = w.Close()

	req, err := http.NewRequest(http.MethodPost, e.server.URL+"/api/v2/files/upload", &buf)
	if err != nil {
		t.Fatalf("构造上传请求失败: %v", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+tok)

	resp, err := e.client.Do(req)
	if err != nil {
		t.Fatalf("上传失败: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("上传返回 %d: %s", resp.StatusCode, raw)
	}
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("解析上传响应失败: %v", err)
	}
	data, _ := parsed["data"].(map[string]any)
	return data
}

// TestEndToEndReleaseFlow 覆盖发布、检测更新、下载、分享与权限的完整链路。
func TestEndToEndReleaseFlow(t *testing.T) {
	e := newTestEnv(t)

	// ---- 健康检查 ----
	if status, _ := e.do(t, http.MethodGet, "/healthz", "", nil); status != http.StatusOK {
		t.Fatalf("healthz 返回 %d", status)
	}
	if status, _ := e.do(t, http.MethodGet, "/readyz", "", nil); status != http.StatusOK {
		t.Fatalf("readyz 返回 %d", status)
	}

	// ---- 未认证访问受保护接口 ----
	if status, _ := e.do(t, http.MethodGet, "/api/v2/apps", "", nil); status != http.StatusUnauthorized {
		t.Fatalf("未认证访问应为 401，实际 %d", status)
	}

	admin := e.login(t, "admin", "admin123456")
	// 默认管理员使用的是首次启动随机口令，这里仅验证登录成功
	if admin == "" {
		t.Fatal("登录未返回访问令牌")
	}

	// ---- 创建应用 ----
	app := e.obj(t, http.MethodPost, "/api/v2/apps", admin, map[string]any{
		"name":        "端到端测试应用",
		"identifier":  "com.example.e2e",
		"description": "集成测试",
		"platforms":   []string{"windows", "android"},
	})
	appID := uint(app["id"].(float64))

	// 重复标识应冲突
	if status, _ := e.do(t, http.MethodPost, "/api/v2/apps", admin, map[string]any{
		"name": "重复", "identifier": "com.example.e2e", "platforms": []string{"windows"},
	}); status != http.StatusConflict {
		t.Fatalf("重复标识应为 409，实际 %d", status)
	}

	// ---- 默认通道 ----
	channels := e.ok(t, http.MethodGet, fmt.Sprintf("/api/v2/apps/%d/channels", appID), admin, nil).([]any)
	if len(channels) != 2 {
		t.Fatalf("默认通道数量应为 2，实际 %d", len(channels))
	}

	// ---- 上传产物并验证秒传 ----
	payload := bytes.Repeat([]byte("release-artifact-payload"), 64)
	first := e.upload(t, admin, "setup.exe", payload)
	if first["deduplicated"].(bool) {
		t.Error("首次上传不应命中秒传")
	}
	fileKey := first["file"].(map[string]any)["key"].(string)
	second := e.upload(t, admin, "setup.exe", payload)
	if !second["deduplicated"].(bool) {
		t.Error("相同内容再次上传应命中秒传")
	}

	// ---- 发布版本 ----
	version := e.obj(t, http.MethodPost, "/api/v2/versions", admin, map[string]any{
		"appId": appID, "platform": "windows", "version": "1.2.3",
		"fileKey": fileKey, "fileName": "setup.exe", "fileSize": len(payload),
		"changelog": "首个版本", "forceUpdate": false, "status": "published",
	})
	versionID := uint(version["id"].(float64))
	if version["channel"] != "stable" {
		t.Errorf("未指定通道时应落到默认通道 stable，实际 %v", version["channel"])
	}
	if version["fileSha256"] == nil || version["fileSha256"].(string) == "" {
		t.Error("发布后应带内容校验值")
	}

	// 重复版本应冲突
	if status, _ := e.do(t, http.MethodPost, "/api/v2/versions", admin, map[string]any{
		"appId": appID, "platform": "windows", "version": "1.2.3", "fileKey": fileKey,
	}); status != http.StatusConflict {
		t.Fatalf("重复版本应为 409，实际 %d", status)
	}

	// ---- v1 开放接口结构兼容 ----
	status, latest := e.do(t, http.MethodGet, "/api/open/latest?identifier=com.example.e2e&platform=windows", "", nil)
	if status != http.StatusOK {
		t.Fatalf("v1 latest 返回 %d", status)
	}
	for _, key := range []string{
		"appName", "appId", "identifier", "version", "platform",
		"changelog", "isForce", "fileName", "filePath", "fileSize", "createdAt",
	} {
		if _, exists := latest[key]; !exists {
			t.Errorf("v1 latest 响应缺少字段 %s", key)
		}
	}
	if latest["version"] != "1.2.3" {
		t.Errorf("v1 latest 版本号错误: %v", latest["version"])
	}
	signedToken := latest["filePath"].(string)

	// ---- v1/v2 检测更新 ----
	v1Check := e.flat(t, http.MethodGet, "/api/open/check?identifier=com.example.e2e&platform=windows&currentVersion=1.0.0", "", nil)
	if v1Check["hasUpdate"] != true {
		t.Errorf("旧版本应提示更新: %v", v1Check)
	}
	v1Same := e.flat(t, http.MethodGet, "/api/open/check?identifier=com.example.e2e&platform=windows&currentVersion=1.2.3", "", nil)
	if v1Same["hasUpdate"] != false {
		t.Errorf("相同版本不应提示更新: %v", v1Same)
	}
	v2Check := e.obj(t, http.MethodGet, "/api/v2/check?identifier=com.example.e2e&platform=windows&currentVersion=1.0.0", "", nil)
	if v2Check["hasUpdate"] != true {
		t.Errorf("v2 检测更新结果错误: %v", v2Check)
	}

	// ---- 下载：签名令牌可用，伪造令牌被拒 ----
	resp, err := e.client.Get(e.server.URL + "/api/open/download/" + signedToken)
	if err != nil {
		t.Fatalf("下载失败: %v", err)
	}
	downloaded, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("下载返回 %d", resp.StatusCode)
	}
	if !bytes.Equal(downloaded, payload) {
		t.Errorf("下载内容与上传内容不一致: %d vs %d", len(downloaded), len(payload))
	}

	if resp, err := e.client.Get(e.server.URL + "/api/open/download/forged.token"); err == nil {
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("伪造令牌应返回 401，实际 %d", resp.StatusCode)
		}
	}

	// ---- 路径穿越防护 ----
	if _, parsed := e.do(t, http.MethodGet, "/api/v2/files", admin, nil); parsed == nil {
		t.Error("文件列表返回异常")
	}

	// ---- 分享（无密码 / 有密码）----
	openShare := e.obj(t, http.MethodPost, fmt.Sprintf("/api/v2/apps/%d/shares", appID), admin, map[string]any{})
	shareToken := openShare["token"].(string)
	shared := e.flat(t, http.MethodPost, "/api/share/"+shareToken, "", map[string]any{})
	if shared["app"] == nil {
		t.Fatal("分享接口未返回应用信息")
	}
	versions := shared["versions"].([]any)
	if len(versions) != 1 {
		t.Fatalf("分享应返回 1 个平台版本，实际 %d", len(versions))
	}
	if _, exists := versions[0].(map[string]any)["filePath"]; !exists {
		t.Error("分享版本缺少 filePath 字段（v1 兼容）")
	}

	locked := e.obj(t, http.MethodPost, fmt.Sprintf("/api/v2/apps/%d/shares", appID), admin, map[string]any{"password": "secret123"})
	lockedToken := locked["token"].(string)

	status, parsed := e.do(t, http.MethodPost, "/api/share/"+lockedToken, "", map[string]any{})
	if status != 209 || parsed["requirePassword"] != true {
		t.Fatalf("密码分享无密码访问应返回 209，实际 %d %v", status, parsed)
	}
	if status, _ := e.do(t, http.MethodPost, "/api/share/"+lockedToken, "", map[string]any{"password": "wrong"}); status != 209 {
		t.Fatalf("错误密码应返回 209，实际 %d", status)
	}
	if status, _ := e.do(t, http.MethodPost, "/api/share/"+lockedToken, "", map[string]any{"password": "secret123"}); status != http.StatusOK {
		t.Fatalf("正确密码应返回 200，实际 %d", status)
	}

	// ---- 通道删除保护 ----
	if status, _ := e.do(t, http.MethodGet, fmt.Sprintf("/api/v2/apps/%d/channels", appID), admin, nil); status != http.StatusOK {
		t.Fatal("通道列表异常")
	}
	stableID := uint(channels[0].(map[string]any)["id"].(float64))
	if status, _ := e.do(t, http.MethodDelete, fmt.Sprintf("/api/v2/apps/%d/channels/%d", appID, stableID), admin, nil); status != http.StatusBadRequest {
		t.Fatalf("删除默认通道应被拒绝，实际 %d", status)
	}

	// ---- 版本下架后开放接口不可见 ----
	e.obj(t, http.MethodPut, fmt.Sprintf("/api/v2/versions/%d/status", versionID), admin, map[string]any{"status": "archived"})
	if status, _ := e.do(t, http.MethodGet, "/api/open/latest?identifier=com.example.e2e&platform=windows", "", nil); status != http.StatusNotFound {
		t.Fatalf("下架后开放接口应返回 404，实际 %d", status)
	}

	// ---- 只读角色权限 ----
	e.obj(t, http.MethodPost, "/api/v2/users", admin, map[string]any{
		"username": "viewer1", "password": "viewer123456", "role": "viewer",
	})
	viewer := e.login(t, "viewer1", "viewer123456")
	if status, _ := e.do(t, http.MethodGet, "/api/v2/apps", viewer, nil); status != http.StatusOK {
		t.Fatalf("只读用户应可查看应用列表，实际 %d", status)
	}
	if status, _ := e.do(t, http.MethodPost, "/api/v2/apps", viewer, map[string]any{
		"name": "非法创建", "identifier": "com.example.denied", "platforms": []string{"windows"},
	}); status != http.StatusForbidden {
		t.Fatalf("只读用户创建应用应返回 403，实际 %d", status)
	}
	if status, _ := e.do(t, http.MethodGet, "/api/v2/users", viewer, nil); status != http.StatusForbidden {
		t.Fatalf("只读用户访问用户管理应返回 403，实际 %d", status)
	}

	// ---- 审计日志 ----
	logs := e.obj(t, http.MethodGet, "/api/v2/audit-logs?pageSize=100", admin, nil)
	list := logs["list"].([]any)
	if len(list) == 0 {
		t.Fatal("审计日志为空")
	}
	actions := map[string]bool{}
	for _, item := range list {
		entry := item.(map[string]any)
		actions[entry["action"].(string)] = true
	}
	for _, want := range []string{"auth.login", "app.create", "version.publish", "file.upload", "share.create"} {
		if !actions[want] {
			t.Errorf("审计日志缺少动作 %s", want)
		}
	}
	// 审计日志需要记录发起者
	firstEntry := list[0].(map[string]any)
	if firstEntry["actorName"] == "" {
		t.Error("审计日志未记录操作者")
	}
}
