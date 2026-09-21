package storage

import (
	"errors"
	"strings"
	"testing"
)

// TestNewRejectsUnknownDriver 未知驱动必须报错，避免配置写错时静默降级。
func TestNewRejectsUnknownDriver(t *testing.T) {
	if _, err := New(Options{Driver: "ftp"}); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("未知驱动应返回 ErrUnsupported，实际 %v", err)
	}
}

// TestNewLocalValidation 本地存储根目录不能为空。
func TestNewLocalValidation(t *testing.T) {
	if _, err := New(Options{Driver: "local", LocalRoot: "   "}); err == nil {
		t.Fatal("空的本地存储根目录应被拒绝")
	}
	store, err := New(Options{Driver: "local", LocalRoot: t.TempDir()})
	if err != nil {
		t.Fatalf("合法的本地存储应可构造: %v", err)
	}
	if store.Driver() != "local" {
		t.Errorf("驱动名错误: %s", store.Driver())
	}
}

// TestS3Validation 校验 S3/MinIO 必填项，避免运行期才暴露配置缺失。
func TestS3Validation(t *testing.T) {
	cases := []struct {
		name string
		opts S3Options
		want string
	}{
		{name: "缺少 endpoint", opts: S3Options{Bucket: "b", AccessKey: "k", SecretKey: "s"}, want: "endpoint"},
		{name: "缺少 bucket", opts: S3Options{Endpoint: "127.0.0.1:9000", AccessKey: "k", SecretKey: "s"}, want: "bucket"},
		{name: "缺少密钥", opts: S3Options{Endpoint: "127.0.0.1:9000", Bucket: "b"}, want: "bucket"},
	}
	for _, c := range cases {
		_, err := newS3(c.opts, 0)
		if err == nil {
			t.Errorf("%s 应被拒绝", c.name)
			continue
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s 的错误信息应包含 %q，实际 %q", c.name, c.want, err.Error())
		}
	}
}

// TestS3UnreachableEndpointFailsFast 指向不可达地址时应快速失败，而不是静默返回可用实例。
func TestS3UnreachableEndpointFailsFast(t *testing.T) {
	_, err := newS3(S3Options{
		Endpoint:  "127.0.0.1:1",
		Region:    "us-east-1",
		Bucket:    "appv",
		AccessKey: "ak",
		SecretKey: "sk",
		UseSSL:    false,
	}, 0)
	if err == nil {
		t.Fatal("不可达的 S3 端点应返回错误")
	}
}
