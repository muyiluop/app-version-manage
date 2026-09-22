package storage

import (
	"errors"
	"strings"
	"testing"
)

// 真实报错样本：自建 MinIO 配置了非默认 region 时的响应。
const regionMismatchMsg = "The authorization header is malformed; the region is wrong; expecting 'cn-east-hf'"

func TestExpectedRegion(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"解析出期望 region", errors.New(regionMismatchMsg), "cn-east-hf"},
		{"无关错误返回空", errors.New("connection refused"), ""},
		{"nil 错误返回空", nil, ""},
		{"带前缀包装后仍可解析", errors.New("检查 bucket 失败: " + regionMismatchMsg), "cn-east-hf"},
	}
	for _, c := range cases {
		if got := expectedRegion(c.err); got != c.want {
			t.Errorf("%s: expectedRegion = %q，期望 %q", c.name, got, c.want)
		}
	}
}

// TestRegionHint 提示必须给出可直接照做的配置项。
func TestRegionHint(t *testing.T) {
	err := errors.New(regionMismatchMsg)

	// 未配置 region：提示设置
	got := regionHint(err, "")
	if !strings.Contains(got, "cn-east-hf") || !strings.Contains(got, "APPV_STORAGE_S3_REGION") {
		t.Errorf("未配置时应提示要设置的值，实际: %s", got)
	}

	// 配错了 region：提示改成正确值
	got = regionHint(err, "us-east-1")
	if !strings.Contains(got, "us-east-1") || !strings.Contains(got, "cn-east-hf") {
		t.Errorf("配错时应同时指出当前值与正确值，实际: %s", got)
	}

	// 与 region 无关的错误不追加任何内容（避免噪声）
	if got := regionHint(errors.New("connection refused"), ""); got != "" {
		t.Errorf("无关错误不应追加提示，实际: %s", got)
	}
}
