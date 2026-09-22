package storage

import (
	"context"
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

// TestParsePublicEndpointErrorMessage 报错要指明是哪个配置项、当前值是什么。
func TestParsePublicEndpointErrorMessage(t *testing.T) {
	_, _, err := parsePublicEndpoint("minio.internal:9000")
	if err == nil {
		t.Fatal("应报错")
	}
	if !strings.Contains(err.Error(), "publicEndpoint") {
		t.Errorf("报错应包含配置项名，实际: %v", err)
	}
}
