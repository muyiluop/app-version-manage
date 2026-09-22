package storage

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// presignParams 组装预签名查询参数。
//
// 两个参数都带文件名，但用途不同：
//   - response-content-disposition：浏览器另存为时使用的文件名（S3 语义）；
//   - filename：把文件名原样再放一份，并排到查询串**最后**（见 moveParamLast）。
//
// 为什么要后者：不少客户端直接从 URL 文本截取扩展名判断文件类型，而 disposition 的
// 值里既有前缀 attachment; filename*=UTF-8” 又有百分号编码，最后一个点之后常常拖着一串
// 垃圾字符。让「以文件名为值的参数」收尾，URL 末尾天然就是 .apk 这样的完整后缀，
// 客户端取最后一个点之后即可，无需再解析任何前缀。
func presignParams(filename string) url.Values {
	params := url.Values{}
	if filename == "" {
		return params
	}
	params.Set("response-content-disposition",
		fmt.Sprintf("attachment; filename*=UTF-8''%s", url.PathEscape(filename)))
	params.Set("filename", filename)
	return params
}

// moveParamLast 把指定查询参数移动到查询串末尾。
//
// 为什么可以重排：SigV4 的校验基于「按参数名排序后的集合」，与参数在 URL 中的先后
// 顺序无关，因此重排不会破坏签名；但在下发之后再拼接**未签名**的参数会直接被拒（403）。
// 这里直接操作 RawQuery，保留原始编码，避免二次编码造成签名不一致。
func moveParamLast(u *url.URL, name string) {
	if u == nil || u.RawQuery == "" || name == "" {
		return
	}
	parts := strings.Split(u.RawQuery, "&")
	keep := make([]string, 0, len(parts))
	moved := make([]string, 0, 1)
	for _, p := range parts {
		if strings.HasPrefix(p, name+"=") {
			moved = append(moved, p)
			continue
		}
		keep = append(keep, p)
	}
	if len(moved) == 0 {
		return
	}
	u.RawQuery = strings.Join(append(keep, moved...), "&")
}

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
