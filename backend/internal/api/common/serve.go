package common

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"app_version_manage/internal/service"
)

// ServeObject 直接输出指定对象，用于公开的图标等资源。
func ServeObject(c *gin.Context, files *service.FileService, key string, download bool) error {
	target, err := files.OpenDownload(c.Request.Context(), key)
	if err != nil {
		return err
	}
	if target.RedirectURL != "" {
		c.Redirect(http.StatusFound, target.RedirectURL)
		return nil
	}
	defer target.Reader.Close()

	disposition := "inline"
	if download {
		disposition = ContentDisposition(target.FileName)
	}
	c.Header("Content-Type", target.ContentType)
	c.Header("Content-Disposition", disposition)
	c.Header("Cache-Control", "public, max-age=86400")
	c.Header("X-Content-Type-Options", "nosniff")
	c.DataFromReader(http.StatusOK, target.Size, target.ContentType, target.Reader, nil)
	return nil
}
