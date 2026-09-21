package migration

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"app_version_manage/internal/storage"
)

// FileMapping 旧对象键到新对象键的映射。
type FileMapping struct {
	OldKey  string // 旧键 = 源目录中的相对路径（v1 为 "md5_原始文件名"）
	NewKey  string // 新键 = 分片后的 "<sha256前2位>/<次2位>/<sha256>_<安全文件名>"
	SHA256  string
	Size    int64
	Name    string // 原始文件名（去掉哈希前缀后的部分不可靠，这里保留源文件名）
	Copied  bool
	Skipped bool // 目标已存在，未重复上传
}

// FileStats 文件搬迁统计。
type FileStats struct {
	Scanned int
	Copied  int
	Skipped int
	Missing []string
	Mapping map[string]FileMapping
}

// copyFiles 遍历源目录并把文件搬到目标存储，返回 旧键 -> 新键 的映射。
//
// 幂等：目标已存在同名对象时跳过；dry-run 时只计算哈希与目标键，不写入。
func copyFiles(ctx context.Context, srcDir string, dst storage.Storage, dryRun bool, log *slog.Logger) (*FileStats, error) {
	stats := &FileStats{Mapping: map[string]FileMapping{}}

	info, err := os.Stat(srcDir)
	if err != nil {
		return nil, fmt.Errorf("访问源文件目录 %s 失败: %w", srcDir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("源文件路径 %s 不是目录", srcDir)
	}

	root := filepath.Clean(srcDir)

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		oldKey := filepath.ToSlash(rel)

		sha, size, err := hashFile(path)
		if err != nil {
			return fmt.Errorf("计算 %s 的文件哈希失败: %w", oldKey, err)
		}

		newKey := storage.BuildKey(sha, filepath.Base(oldKey))
		mapping := FileMapping{
			OldKey: oldKey,
			NewKey: newKey,
			SHA256: sha,
			Size:   size,
			Name:   filepath.Base(oldKey),
		}
		stats.Scanned++

		if dryRun {
			stats.Mapping[oldKey] = mapping
			return nil
		}

		// 目标已有该对象：跳过上传（可重复执行）
		if _, err := dst.Stat(ctx, newKey); err == nil {
			mapping.Skipped = true
			stats.Skipped++
			stats.Mapping[oldKey] = mapping
			return nil
		}

		src, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("打开 %s 失败: %w", oldKey, err)
		}
		defer src.Close()

		if _, err := dst.Put(ctx, newKey, src, size, contentTypeOf(oldKey), sha); err != nil {
			return fmt.Errorf("写入目标存储失败（%s -> %s）: %w", oldKey, newKey, err)
		}
		mapping.Copied = true
		stats.Copied++
		stats.Mapping[oldKey] = mapping

		if log != nil && stats.Copied%100 == 0 {
			log.Info("文件搬迁进行中", "copied", stats.Copied, "scanned", stats.Scanned)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return stats, nil
}

// hashFile 计算文件 sha256 与大小。
func hashFile(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	return storage.HashReader(f)
}

// contentTypeOf 依据扩展名推断内容类型（旧库没有可靠类型信息时使用）。
func contentTypeOf(key string) string {
	ext := strings.ToLower(filepath.Ext(key))
	switch ext {
	case ".apk":
		return "application/vnd.android.package-archive"
	case ".exe", ".msi":
		return "application/octet-stream"
	case ".dmg":
		return "application/x-apple-diskimage"
	case ".zip":
		return "application/zip"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".svg":
		return "image/svg+xml"
	case ".webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}
