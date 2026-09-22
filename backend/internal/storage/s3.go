package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
)

// s3Storage 基于 MinIO 客户端实现，兼容 MinIO、AWS S3 及其他 S3 协议对象存储。
type s3Storage struct {
	client  *minio.Client
	bucket  string
	prefix  string
	urlTTL  time.Duration
	usePath bool
}

// newS3 构造 S3 存储并确保 bucket 可用。
func newS3(opts S3Options, urlTTL time.Duration) (*s3Storage, error) {
	if opts.Endpoint == "" {
		return nil, fmt.Errorf("storage.s3.endpoint 不能为空")
	}
	if opts.Bucket == "" || opts.AccessKey == "" || opts.SecretKey == "" {
		return nil, fmt.Errorf("storage.s3 需要 bucket / accessKey / secretKey")
	}
	if urlTTL <= 0 {
		urlTTL = 30 * time.Minute
	}

	lookup := minio.BucketLookupAuto
	if opts.ForcePathStyle {
		lookup = minio.BucketLookupPath
	}

	client, err := newS3Client(opts, opts.Region, lookup)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	exists, err := client.BucketExists(ctx, opts.Bucket)
	if err != nil {
		return nil, fmt.Errorf("检查 bucket 失败: %w%s", err, regionHint(err, opts.Region))
	}
	if !exists {
		// 创建 bucket 时无法先探测「已存在 bucket 的位置」，所以跟随服务端在报错里
		// 给出的期望 region 重试一次（自建 MinIO 常配置了非默认 region）。
		mkErr := client.MakeBucket(ctx, opts.Bucket, minio.MakeBucketOptions{Region: opts.Region})
		if mkErr != nil {
			if expected := expectedRegion(mkErr); expected != "" && expected != opts.Region {
				if retry, rerr := newS3Client(opts, expected, lookup); rerr == nil {
					if rerr = retry.MakeBucket(ctx, opts.Bucket, minio.MakeBucketOptions{Region: expected}); rerr == nil {
						client = retry
						mkErr = nil
					}
				}
			}
		}
		if mkErr != nil {
			return nil, fmt.Errorf("创建 bucket %s 失败: %w%s", opts.Bucket, mkErr, regionHint(mkErr, opts.Region))
		}
	}

	return &s3Storage{
		client:  client,
		bucket:  opts.Bucket,
		prefix:  strings.Trim(opts.Prefix, "/"),
		urlTTL:  urlTTL,
		usePath: opts.ForcePathStyle,
	}, nil
}

// Driver 返回驱动名。
func (s *s3Storage) Driver() string { return "s3" }

// objectName 拼接对象前缀。
func (s *s3Storage) objectName(key string) (string, error) {
	clean, err := NormalizeKey(key)
	if err != nil {
		return "", err
	}
	if s.prefix == "" {
		return clean, nil
	}
	return s.prefix + "/" + clean, nil
}

// Put 上传对象。
func (s *s3Storage) Put(ctx context.Context, key string, r io.Reader, size int64, contentType, sha256Hex string) (*Object, error) {
	name, err := s.objectName(key)
	if err != nil {
		return nil, err
	}
	if size <= 0 {
		size = -1
	}

	info, err := s.client.PutObject(ctx, s.bucket, name, r, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return nil, fmt.Errorf("上传对象失败: %w", err)
	}

	clean, _ := NormalizeKey(key)
	return &Object{Key: clean, Size: info.Size, ContentType: contentType, SHA256: sha256Hex}, nil
}

// Open 读取对象。
func (s *s3Storage) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	name, err := s.objectName(key)
	if err != nil {
		return nil, err
	}
	obj, err := s.client.GetObject(ctx, s.bucket, name, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	// GetObject 为惰性读取，需 Stat 一次以尽早发现对象不存在。
	if _, err := obj.Stat(); err != nil {
		_ = obj.Close()
		return nil, wrapS3Error(err, key)
	}
	return obj, nil
}

// Delete 删除对象，不存在视为成功。
func (s *s3Storage) Delete(ctx context.Context, key string) error {
	name, err := s.objectName(key)
	if err != nil {
		return err
	}
	if err := s.client.RemoveObject(ctx, s.bucket, name, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("删除对象失败: %w", err)
	}
	return nil
}

// Stat 返回对象元信息。
func (s *s3Storage) Stat(ctx context.Context, key string) (*Object, error) {
	name, err := s.objectName(key)
	if err != nil {
		return nil, err
	}
	info, err := s.client.StatObject(ctx, s.bucket, name, minio.StatObjectOptions{})
	if err != nil {
		return nil, wrapS3Error(err, key)
	}
	clean, _ := NormalizeKey(key)
	return &Object{Key: clean, Size: info.Size, ContentType: info.ContentType}, nil
}

// PresignGet 生成带原始文件名的预签名下载地址。
func (s *s3Storage) PresignGet(ctx context.Context, key, filename string, ttl time.Duration) (string, error) {
	name, err := s.objectName(key)
	if err != nil {
		return "", err
	}
	if ttl <= 0 {
		ttl = s.urlTTL
	}

	params := url.Values{}
	if filename != "" {
		params.Set("response-content-disposition",
			fmt.Sprintf("attachment; filename*=UTF-8''%s", url.PathEscape(filename)))
	}

	u, err := s.client.PresignedGetObject(ctx, s.bucket, name, ttl, params)
	if err != nil {
		return "", fmt.Errorf("生成预签名地址失败: %w", err)
	}
	return u.String(), nil
}

// wrapS3Error 将 S3 的 NoSuchKey 归一化为 ErrObjectMiss。
func wrapS3Error(err error, key string) error {
	var resp minio.ErrorResponse
	if errors.As(err, &resp) {
		switch resp.Code {
		case "NoSuchKey", "NoSuchObject", "NotFound":
			return fmt.Errorf("%w: %s", ErrObjectMiss, key)
		}
	}
	return err
}
