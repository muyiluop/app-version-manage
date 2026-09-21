package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// localStorage 本地磁盘存储实现。
type localStorage struct {
	root string
}

// newLocal 构造本地存储并确保根目录存在。
func newLocal(root string) (*localStorage, error) {
	if strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("本地存储根目录不能为空")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("解析本地存储路径失败: %w", err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, fmt.Errorf("创建本地存储目录失败: %w", err)
	}
	return &localStorage{root: abs}, nil
}

// Driver 返回驱动名。
func (l *localStorage) Driver() string { return "local" }

// Root 返回存储根目录绝对路径。
func (l *localStorage) Root() string { return l.root }

// resolve 将对象键解析为根目录内的绝对路径，阻止路径穿越。
func (l *localStorage) resolve(key string) (string, error) {
	clean, err := NormalizeKey(key)
	if err != nil {
		return "", err
	}
	full, err := filepath.Abs(filepath.Join(l.root, filepath.FromSlash(clean)))
	if err != nil {
		return "", err
	}
	if full != l.root && !strings.HasPrefix(full, l.root+string(os.PathSeparator)) {
		return "", ErrInvalidKey
	}
	return full, nil
}

// Put 写入对象：先写临时文件再原子重命名，避免半截文件。
func (l *localStorage) Put(_ context.Context, key string, r io.Reader, _ int64, contentType, sha256Hex string) (*Object, error) {
	full, err := l.resolve(key)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return nil, fmt.Errorf("创建目录失败: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(full), ".upload-*")
	if err != nil {
		return nil, fmt.Errorf("创建临时文件失败: %w", err)
	}
	tmpName := tmp.Name()

	written, copyErr := io.Copy(tmp, r)
	closeErr := tmp.Close()
	if copyErr != nil {
		_ = os.Remove(tmpName)
		return nil, fmt.Errorf("写入文件失败: %w", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(tmpName)
		return nil, fmt.Errorf("关闭文件失败: %w", closeErr)
	}
	if err := os.Rename(tmpName, full); err != nil {
		_ = os.Remove(tmpName)
		return nil, fmt.Errorf("保存文件失败: %w", err)
	}
	if err := os.Chmod(full, 0o644); err != nil {
		return nil, fmt.Errorf("设置文件权限失败: %w", err)
	}

	clean, _ := NormalizeKey(key)
	return &Object{Key: clean, Size: written, ContentType: contentType, SHA256: sha256Hex}, nil
}

// Open 打开对象读取。
func (l *localStorage) Open(_ context.Context, key string) (io.ReadCloser, error) {
	full, err := l.resolve(key)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(full)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrObjectMiss, key)
		}
		return nil, err
	}
	return f, nil
}

// Delete 删除对象，不存在视为成功。
func (l *localStorage) Delete(_ context.Context, key string) error {
	full, err := l.resolve(key)
	if err != nil {
		return err
	}
	if err := os.Remove(full); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// Stat 返回对象元信息。
func (l *localStorage) Stat(_ context.Context, key string) (*Object, error) {
	full, err := l.resolve(key)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(full)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrObjectMiss, key)
		}
		return nil, err
	}
	clean, _ := NormalizeKey(key)
	return &Object{Key: clean, Size: info.Size()}, nil
}

// PresignGet 本地存储不支持直连地址，返回空字符串表示需经后端代理下载。
func (l *localStorage) PresignGet(context.Context, string, string, time.Duration) (string, error) {
	return "", nil
}

// randomSuffix 生成临时文件后缀。
func randomSuffix() string {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return "tmp"
	}
	return hex.EncodeToString(buf)
}
