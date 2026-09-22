package storage

import (
	"fmt"
	"regexp"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// expectedRegionPattern 提取服务端在 region 不匹配时给出的期望值，例如：
//
//	The authorization header is malformed; the region is wrong; expecting 'cn-east-hf'
var expectedRegionPattern = regexp.MustCompile(`expecting '([^']+)'`)

// expectedRegion 从报错中解析服务端期望的 region；解析不到时返回空串。
func expectedRegion(err error) string {
	if err == nil {
		return ""
	}
	m := expectedRegionPattern.FindStringSubmatch(err.Error())
	if len(m) != 2 {
		return ""
	}
	return m[1]
}

// regionHint 在原始错误后追加可执行的修复提示；没有线索时返回空串。
func regionHint(err error, current string) string {
	expected := expectedRegion(err)
	if expected == "" {
		return ""
	}
	if current == "" {
		return fmt.Sprintf("；对象存储要求 region=%s，请设置 storage.s3.region（或环境变量 APPV_STORAGE_S3_REGION=%s）",
			expected, expected)
	}
	return fmt.Sprintf("；当前 region=%s 与服务端要求不符，请改为 storage.s3.region=%s（或环境变量 APPV_STORAGE_S3_REGION=%s）",
		current, expected, expected)
}

// newS3Client 构造 S3 客户端。
//
// region 留空时交给 minio-go 自行探测 bucket 所在区域（GetBucketLocation）——
// 自建 MinIO 常配置了非默认 region，写死默认值会导致签名被拒：
//
//	The authorization header is malformed; the region is wrong; expecting 'cn-east-hf'
//
// 因此默认不指定 region，需要时可显式配置。
func newS3Client(opts S3Options, region string, lookup minio.BucketLookupType) (*minio.Client, error) {
	client, err := minio.New(opts.Endpoint, &minio.Options{
		Creds:        credentials.NewStaticV4(opts.AccessKey, opts.SecretKey, ""),
		Secure:       opts.UseSSL,
		Region:       region,
		BucketLookup: lookup,
	})
	if err != nil {
		return nil, fmt.Errorf("初始化 S3 客户端失败: %w", err)
	}
	return client, nil
}
