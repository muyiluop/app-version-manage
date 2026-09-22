package storage

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestParsePublicEndpoint(t *testing.T) {
	cases := []struct {
		name       string
		raw        string
		wantHost   string
		wantSecure bool
		wantErr    bool
	}{
		{"留空表示不启用", "", "", false, false},
		{"仅空格同样视为未配置", "   ", "", false, false},
		{"https 地址", "https://files.example.com", "files.example.com", true, false},
		{"http 带端口", "http://192.168.10.6:9000", "192.168.10.6:9000", false, false},
		{"缺少 scheme 应报错", "files.example.com", "", false, true},
		{"不支持路径前缀", "https://files.example.com/s3", "", false, true},
		{"缺少主机名应报错", "https://", "", false, true},
	}
	for _, c := range cases {
		host, secure, err := parsePublicEndpoint(c.raw)
		if c.wantErr {
			if err == nil {
				t.Errorf("%s: 期望报错，实际 host=%q secure=%v", c.name, host, secure)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: 不应报错: %v", c.name, err)
			continue
		}
		if host != c.wantHost || secure != c.wantSecure {
			t.Errorf("%s: 得到 (%q, %v)，期望 (%q, %v)", c.name, host, secure, c.wantHost, c.wantSecure)
		}
	}
}

func TestParsePublicEndpointErrorMessage(t *testing.T) {
	_, _, err := parsePublicEndpoint("minio.internal:9000")
	if err == nil {
		t.Fatal("应报错")
	}
	if !strings.Contains(err.Error(), "publicEndpoint") {
		t.Errorf("报错应包含配置项名，实际: %v", err)
	}
}

// TestPresignDisabledWithoutPublicEndpoint 未配置对外地址时不得下发预签名地址，
// 否则浏览器会拿到内网/明文直连地址（HTTPS 页面下被按混合内容拦截）。
func TestPresignDisabledWithoutPublicEndpoint(t *testing.T) {
	s := &s3Storage{bucket: "app-version", prefix: "appv", urlTTL: time.Minute}

	got, err := s.PresignGet(context.Background(), "ab/cd/xyz_logo.png", "logo.png", time.Minute)
	if err != nil {
		t.Fatalf("未配置对外地址时不应报错: %v", err)
	}
	if got != "" {
		t.Errorf("未配置对外地址时应返回空串（改走后端代理），实际 %q", got)
	}
}

// TestPresignParams 文件名要原样带上，作为 URL 末尾的扩展名来源。
func TestPresignParams(t *testing.T) {
	const name = "移动告警恢复版.apk"
	params := presignParams(name)

	if got := params.Get("filename"); got != name {
		t.Errorf("filename 参数应保留原始文件名，实际 %q", got)
	}
	if got := params.Get("response-content-disposition"); !strings.Contains(got, "filename") {
		t.Errorf("应保留 response-content-disposition，实际 %q", got)
	}

	if got := presignParams(""); len(got) != 0 {
		t.Errorf("文件名为空时不应带任何参数，实际 %v", got)
	}
}

// TestMoveParamLast filename 必须排在查询串末尾，这样 URL 末段天然是扩展名；
// 同时其余参数与原始编码不能被动到（否则签名会失效）。
func TestMoveParamLast(t *testing.T) {
	u, err := url.Parse("https://files.example.com/b/obj.apk?X-Amz-Signature=abc&filename=a%2Fb.apk&response-content-disposition=x")
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	moveParamLast(u, "filename")

	if !strings.HasSuffix(u.RawQuery, "filename=a%2Fb.apk") {
		t.Errorf("filename 应位于查询串末尾，实际 %s", u.RawQuery)
	}
	if !strings.Contains(u.RawQuery, "X-Amz-Signature=abc") {
		t.Errorf("签名参数必须保留，实际 %s", u.RawQuery)
	}
	if !strings.Contains(u.RawQuery, "response-content-disposition=x") {
		t.Errorf("其余参数必须保留，实际 %s", u.RawQuery)
	}

	// 末尾就是扩展名：客户端「取最后一个点之后」应得到 apk
	tail := u.RawQuery[strings.LastIndex(u.RawQuery, ".")+1:]
	if tail != "apk" {
		t.Errorf("URL 末尾截取应得到 apk，实际 %q", tail)
	}

	// 没有该参数时不应改动
	other, _ := url.Parse("https://x/y?a=1&b=2")
	moveParamLast(other, "filename")
	if other.RawQuery != "a=1&b=2" {
		t.Errorf("缺少目标参数时不应改动查询串，实际 %s", other.RawQuery)
	}
}
