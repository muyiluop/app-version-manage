package storage

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// parsePublicEndpoint 解析「对外访问地址」，返回 minio 需要的 endpoint 与是否启用 TLS。
//
// 只接受形如 https://files.example.com[:port] 的地址：
//   - 必须带 http:// 或 https://，否则无法判断是否启用 TLS；
//   - 不接受路径前缀：minio 客户端的 endpoint 不支持路径，带路径会生成错误地址，
//     与其静默出错不如直接拒绝。
func parsePublicEndpoint(raw string) (endpoint string, secure bool, err error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", false, nil
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		return "", false, fmt.Errorf("storage.s3.publicEndpoint 解析失败: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", false, fmt.Errorf("storage.s3.publicEndpoint 需以 http:// 或 https:// 开头，当前为 %q", raw)
	}
	if u.Host == "" {
		return "", false, fmt.Errorf("storage.s3.publicEndpoint 缺少主机名: %q", raw)
	}
	if p := strings.Trim(u.Path, "/"); p != "" {
		return "", false, fmt.Errorf("storage.s3.publicEndpoint 不支持路径前缀（当前 %q），请改用独立域名或子域名", u.Path)
	}
	return u.Host, u.Scheme == "https", nil
}

// newPresignClient 构造「仅用于生成预签名地址」的客户端：
// 与业务客户端共用凭据与寻址方式，但用浏览器可达的地址来签名。
//
// 注意：SigV4 的签名覆盖 Host 头，所以必须用对外地址签名，
// 不能先按内网地址签名再改写 URL —— 那样签名校验会失败。
func newPresignClient(opts S3Options) (*minio.Client, error) {
	endpoint, secure, err := parsePublicEndpoint(opts.PublicEndpoint)
	if err != nil {
		return nil, err
	}
	lookup := minio.BucketLookupAuto
	if opts.ForcePathStyle {
		lookup = minio.BucketLookupPath
	}
	client, err := minio.New(endpoint, &minio.Options{
		Creds:        credentials.NewStaticV4(opts.AccessKey, opts.SecretKey, ""),
		Secure:       secure,
		Region:       opts.Region,
		BucketLookup: lookup,
	})
	if err != nil {
		return nil, fmt.Errorf("初始化对外地址 S3 客户端失败: %w", err)
	}
	return client, nil
}
