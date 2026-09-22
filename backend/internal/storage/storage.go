// Package storage 定义对象存储抽象，并支持本地磁盘与 S3 兼容对象存储（MinIO / AWS S3）。
//
// 对象键（key）在两种驱动下语义一致：
//   - 历史键：md5_原始文件名（位于根目录，为兼容既有数据而保留）
//   - 新键：<sha256前2位>/<sha256次2位>/<sha256>_<安全文件名>
package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"time"
)

// 常见错误。
var (
	ErrInvalidKey  = errors.New("非法的对象键")
	ErrObjectMiss  = errors.New("对象不存在")
	ErrUnsupported = errors.New("不支持的存储驱动")
)

// Object 对象元信息。
type Object struct {
	Key         string
	Size        int64
	ContentType string
	SHA256      string
}

// Storage 对象存储接口。
type Storage interface {
	// Driver 返回驱动名称（local / s3）。
	Driver() string
	// Put 写入对象并返回元信息；sha256 由调用方计算后可传入，留空表示未知。
	Put(ctx context.Context, key string, r io.Reader, size int64, contentType, sha256Hex string) (*Object, error)
	// Open 读取对象。
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	// Delete 删除对象，对象不存在时不报错。
	Delete(ctx context.Context, key string) error
	// Stat 获取对象元信息。
	Stat(ctx context.Context, key string) (*Object, error)
	// PresignGet 返回直连下载地址；本地存储返回空字符串，表示需经后端代理下载。
	PresignGet(ctx context.Context, key, filename string, ttl time.Duration) (string, error)
}

// Options 存储构造参数。
type Options struct {
	Driver       string
	LocalRoot    string
	S3           S3Options
	SignedURLTTL time.Duration
}

// S3Options S3 兼容存储参数。
type S3Options struct {
	Endpoint       string
	Region         string
	Bucket         string
	AccessKey      string
	SecretKey      string
	UseSSL         bool
	ForcePathStyle bool
	Prefix         string
	// PublicEndpoint 为浏览器可达的对象存储地址（含 scheme）；留空表示不下发预签名地址。
	PublicEndpoint string
}

// New 依据配置构造存储实现。
func New(opts Options) (Storage, error) {
	switch opts.Driver {
	case "local", "":
		return newLocal(opts.LocalRoot)
	case "s3":
		return newS3(opts.S3, opts.SignedURLTTL)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupported, opts.Driver)
	}
}

// SanitizeFileName 清洗原始文件名，去除目录成分与控制字符。
func SanitizeFileName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "\\", "/")
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}
	name = strings.Map(func(r rune) rune {
		switch {
		case r < 32 || r == 127:
			return -1
		case r == '<' || r == '>' || r == ':' || r == '"' || r == '|' || r == '?' || r == '*':
			return '_'
		default:
			return r
		}
	}, name)
	name = strings.Trim(name, ". ")
	if name == "" || name == "." || name == ".." {
		name = "file"
	}
	if len(name) > 180 {
		ext := path.Ext(name)
		if len(ext) > 20 {
			ext = ""
		}
		name = name[:180-len(ext)] + ext
	}
	return name
}

// BuildKey 依据内容哈希与原始文件名生成新对象键（两级目录分片，避免单目录堆积）。
func BuildKey(sha256Hex, filename string) string {
	name := SanitizeFileName(filename)
	sha256Hex = strings.ToLower(strings.TrimSpace(sha256Hex))
	if len(sha256Hex) < 4 {
		return name
	}
	return fmt.Sprintf("%s/%s/%s_%s", sha256Hex[0:2], sha256Hex[2:4], sha256Hex, name)
}

// NormalizeKey 规范化并校验对象键，防止路径穿越与绝对路径。
func NormalizeKey(key string) (string, error) {
	k := strings.TrimSpace(key)
	k = strings.ReplaceAll(k, "\\", "/")
	k = strings.TrimPrefix(k, "/")
	if k == "" {
		return "", ErrInvalidKey
	}
	if strings.ContainsRune(k, rune(0)) {
		return "", ErrInvalidKey
	}
	clean := path.Clean(k)
	if clean == "." || clean == ".." || path.IsAbs(clean) || strings.HasPrefix(clean, "../") {
		return "", ErrInvalidKey
	}
	for _, seg := range strings.Split(clean, "/") {
		if seg == ".." {
			return "", ErrInvalidKey
		}
	}
	return clean, nil
}

// HashReader 计算 sha256，同时返回读取到的字节数。
func HashReader(r io.Reader) (string, int64, error) {
	h := sha256.New()
	n, err := io.Copy(h, r)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

// IsNotFound 判断错误是否表示对象不存在。
func IsNotFound(err error) bool {
	return errors.Is(err, ErrObjectMiss) || errors.Is(err, io.EOF)
}
