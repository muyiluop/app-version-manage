// Package common 存放 v1/v2 处理器共用的 HTTP 辅助逻辑。
package common

import (
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"app_version_manage/internal/apierr"
	"app_version_manage/internal/pkg/token"
	"app_version_manage/internal/service"
)

// ServeDownload 校验下载令牌并输出文件。
//
// 本地存储：服务端流式代理；S3 存储：302 跳转到预签名直连地址。
// 返回的 error 仅在尚未写出响应体时可能出现，调用方可据此格式化错误。
func ServeDownload(c *gin.Context, files *service.FileService, signer *token.Signer, rawToken string) error {
	key, err := signer.Verify(strings.TrimSpace(rawToken))
	if err != nil {
		return apierr.Unauthorized("无效或已过期的下载链接")
	}

	target, err := files.OpenDownload(c.Request.Context(), key)
	if err != nil {
		return err
	}

	files.RecordDownload(c.Request.Context(), key)

	disposition := ContentDisposition(target.FileName)
	if target.RedirectURL != "" {
		c.Header("Content-Disposition", disposition)
		c.Redirect(http.StatusFound, target.RedirectURL)
		return nil
	}
	defer target.Reader.Close()

	c.Header("Content-Type", target.ContentType)
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", disposition)
	c.Header("X-Content-Type-Options", "nosniff")
	c.DataFromReader(http.StatusOK, target.Size, target.ContentType, target.Reader, nil)
	return nil
}

// ContentDisposition 按 RFC 5987 生成安全的下载响应头，避免中文文件名与注入问题。
func ContentDisposition(filename string) string {
	name := strings.TrimSpace(filename)
	if name == "" {
		name = "download"
	}
	ascii := asciiFallback(name)
	return fmt.Sprintf("attachment; filename=%q; filename*=UTF-8''%s", ascii, url.PathEscape(name))
}

// asciiFallback 生成仅含 ASCII 的降级文件名。
func asciiFallback(name string) string {
	base := filepath.Base(name)
	var b strings.Builder
	for _, r := range base {
		switch {
		case r < 32 || r == 127:
			continue
		case r > 126:
			b.WriteRune('_')
		default:
			b.WriteRune(r)
		}
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return "download"
	}
	return out
}

// ClientPlatform 解析客户端平台：显式参数 > X-Platform 头 > User-Agent。
func ClientPlatform(c *gin.Context) string {
	if p := strings.TrimSpace(c.Query("platform")); p != "" {
		return p
	}
	if p := strings.TrimSpace(c.GetHeader("X-Platform")); p != "" {
		return p
	}
	return DetectPlatform(c.GetHeader("User-Agent"))
}

// DetectPlatform 依据 User-Agent 粗略识别平台。
func DetectPlatform(userAgent string) string {
	ua := strings.ToLower(userAgent)
	switch {
	case strings.Contains(ua, "android"):
		return "android"
	case strings.Contains(ua, "iphone"), strings.Contains(ua, "ipad"):
		return "ios"
	case strings.Contains(ua, "windows"):
		return "windows"
	case strings.Contains(ua, "macintosh"), strings.Contains(ua, "mac os"):
		return "macos"
	case strings.Contains(ua, "harmony"):
		return "harmony"
	case strings.Contains(ua, "linux"):
		return "linux"
	default:
		return ""
	}
}
